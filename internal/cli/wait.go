package cli

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"

	"github.com/fisherevans/meatbag/internal/store"
	"github.com/fisherevans/meatbag/internal/tree"
)

// exitErr lets RunE bubble a custom exit code through cobra. cmd/meatbag's
// root handler turns errors returned from RunE into exit code 1; for `wait`
// we need finer control (0 satisfied, 2 item gone, 124 timeout, 130 SIGINT),
// so we just os.Exit ourselves and return nil to cobra.
type waitExit struct {
	code int
	msg  string
}

func (e *waitExit) Error() string { return e.msg }

// newWaitCmd implements `meatbag wait`. The command blocks until the named
// item satisfies the requested conditions, then exits 0. On timeout it exits
// 124 (Unix `timeout(1)` convention); if the item or list vanishes, exit 2.
//
// fsnotify drives wakeups so the command is cheap to leave running for hours.
func newWaitCmd() *cobra.Command {
	var (
		stateFlag   string
		inputFlags  []string
		timeoutFlag time.Duration
	)
	cmd := &cobra.Command{
		Use:   "wait <list> <item>",
		Short: "Block until an item reaches a state or has inputs filled",
		Long: "Block until the named item satisfies the given conditions, then " +
			"exit 0. Exit 124 on timeout, 2 if the item or list disappears. " +
			"Wakeups are fsnotify-driven, not polled.\n\n" +
			"Examples:\n" +
			"  meatbag wait my-list 2 --state=done\n" +
			"  meatbag wait my-list 2 --input=api_key --timeout=30m\n" +
			"  meatbag wait my-list 2 --state=done,skipped --input=cert",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			wantStates, err := parseWantStates(stateFlag)
			if err != nil {
				return err
			}
			wantInputs := parseWantInputs(inputFlags)
			if len(wantStates) == 0 && len(wantInputs) == 0 {
				return fmt.Errorf("no wait condition specified - pass --state or --input")
			}

			s, err := openStore()
			if err != nil {
				return err
			}
			path, _, err := s.FindPath(args[0])
			if err != nil {
				return fail("wait", err)
			}

			eval := func() (*evalResult, error) {
				l, err := s.LoadList(path)
				if err != nil {
					return nil, err
				}
				it, ok := tree.Resolve(l, args[1])
				if !ok {
					return &evalResult{itemMissing: true}, nil
				}
				return evaluate(it, wantStates, wantInputs), nil
			}

			// Initial check before setting up any watcher.
			res, err := eval()
			if err != nil {
				return fail("wait", err)
			}
			if res.itemMissing {
				exitWith(2, fmt.Sprintf("item disappeared: %s", args[1]))
			}
			if res.satisfied {
				emitWaitSuccess(res)
				return nil
			}

			// Set up fsnotify on the parent directory. Watching the file
			// directly breaks across atomic-rename writes.
			dir := filepath.Dir(path)
			base := filepath.Base(path)
			w, err := fsnotify.NewWatcher()
			if err != nil {
				return err
			}
			defer w.Close()
			if err := w.Add(dir); err != nil {
				return err
			}

			// SIGINT/SIGTERM handling for clean exit 130.
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			defer signal.Stop(sigCh)

			// Debounce so save-temp-then-rename collapses into one re-eval.
			const debounce = 150 * time.Millisecond
			var debounceT *time.Timer
			debounceC := make(chan struct{}, 1)
			fire := func() {
				select {
				case debounceC <- struct{}{}:
				default:
				}
			}

			var timeoutC <-chan time.Time
			if timeoutFlag > 0 {
				t := time.NewTimer(timeoutFlag)
				defer t.Stop()
				timeoutC = t.C
			}

			for {
				select {
				case <-timeoutC:
					msg := fmt.Sprintf("timeout waiting for %s", describeUnsatisfied(res, wantStates, wantInputs))
					exitWith(124, msg)
				case sig := <-sigCh:
					if sig == syscall.SIGINT {
						exitWith(130, "")
					}
					exitWith(143, "")
				case err, ok := <-w.Errors:
					if !ok {
						return nil
					}
					_ = err
				case ev, ok := <-w.Events:
					if !ok {
						return nil
					}
					evBase := filepath.Base(ev.Name)
					if evBase != base {
						continue
					}
					// On delete/rename, re-check immediately; the underlying
					// file may be gone. Also re-add the watch in case fsnotify
					// dropped it (only directory watches survive rename, but
					// be defensive).
					if debounceT != nil {
						debounceT.Stop()
					}
					debounceT = time.AfterFunc(debounce, fire)
				case <-debounceC:
					// Stat first so we can report a vanished file distinctly
					// from a transient one.
					if _, statErr := os.Stat(path); statErr != nil {
						if errors.Is(statErr, os.ErrNotExist) {
							exitWith(2, fmt.Sprintf("list file disappeared: %s", args[0]))
						}
					}
					res, err = eval()
					if err != nil {
						// Transient read errors (mid-rename) shouldn't kill
						// the wait; another event will arrive.
						continue
					}
					if res.itemMissing {
						exitWith(2, fmt.Sprintf("item disappeared: %s", args[1]))
					}
					if res.satisfied {
						emitWaitSuccess(res)
						return nil
					}
				}
			}
		},
	}
	cmd.Flags().StringVar(&stateFlag, "state", "", "comma-separated states to wait for (todo,in_progress,blocked,done,skipped)")
	cmd.Flags().StringArrayVar(&inputFlags, "input", nil, "input field that must be set; repeatable")
	cmd.Flags().DurationVar(&timeoutFlag, "timeout", time.Hour, "max time to wait; 0 = forever")
	return cmd
}

// evalResult captures the most recent evaluation of the item against the
// requested conditions. It's used both to decide whether to exit 0 and to
// describe what's still missing on timeout.
type evalResult struct {
	satisfied   bool
	itemMissing bool

	state        store.State
	inputs       map[string]store.InputValue
	missingState bool
	missingInput []string
}

func evaluate(it *store.Item, wantStates map[store.State]struct{}, wantInputs []string) *evalResult {
	res := &evalResult{state: it.State, inputs: it.InputValues}
	if len(wantStates) > 0 {
		if _, ok := wantStates[it.State]; !ok {
			res.missingState = true
		}
	}
	for _, name := range wantInputs {
		v, ok := it.InputValues[name]
		if !ok || !v.HasValue {
			res.missingInput = append(res.missingInput, name)
		}
	}
	res.satisfied = !res.missingState && len(res.missingInput) == 0
	return res
}

func parseWantStates(raw string) (map[store.State]struct{}, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	out := map[store.State]struct{}{}
	for _, part := range strings.Split(raw, ",") {
		p := strings.TrimSpace(part)
		if p == "" {
			continue
		}
		st := store.State(p)
		if !store.ValidState(st) {
			return nil, fmt.Errorf("unknown state %q (valid: todo,in_progress,blocked,done,skipped)", p)
		}
		out[st] = struct{}{}
	}
	return out, nil
}

func parseWantInputs(raw []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, r := range raw {
		for _, part := range strings.Split(r, ",") {
			p := strings.TrimSpace(part)
			if p == "" || seen[p] {
				continue
			}
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

// emitWaitSuccess prints the success line (or JSON) honoring --quiet and --json.
func emitWaitSuccess(res *evalResult) {
	if gFlags.Quiet {
		return
	}
	if gFlags.JSON {
		out := map[string]any{
			"satisfied": true,
			"state":     string(res.state),
		}
		if res.inputs != nil {
			out["inputs"] = redactInputsForWait(res.inputs)
		}
		_ = emitJSON(out)
		return
	}
	var parts []string
	parts = append(parts, "state="+string(res.state))
	names := inputNames(res.inputs)
	if len(names) > 0 {
		parts = append(parts, "inputs="+strings.Join(names, ","))
	}
	fmt.Println("ok " + strings.Join(parts, " "))
}

// describeUnsatisfied formats the unsatisfied conditions for the timeout error.
func describeUnsatisfied(res *evalResult, wantStates map[store.State]struct{}, wantInputs []string) string {
	var parts []string
	if res.missingState {
		parts = append(parts, "state="+stateList(wantStates))
	}
	for _, name := range res.missingInput {
		parts = append(parts, "input "+name)
	}
	if len(parts) == 0 {
		// Shouldn't happen, but guard against an empty timeout message.
		return "conditions"
	}
	return strings.Join(parts, ", ")
}

func stateList(want map[store.State]struct{}) string {
	out := make([]string, 0, len(want))
	for s := range want {
		out = append(out, string(s))
	}
	sort.Strings(out)
	return strings.Join(out, "|")
}

func inputNames(m map[string]store.InputValue) []string {
	out := make([]string, 0, len(m))
	for k, v := range m {
		if v.HasValue {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// redactInputsForWait mirrors daemon.redactInputs - the CLI shouldn't dump
// raw secrets when emitting JSON. For non-secret values we keep HasValue +
// type-derived metadata but drop the literal `value` to stay conservative.
func redactInputsForWait(in map[string]store.InputValue) map[string]store.InputValue {
	if in == nil {
		return nil
	}
	out := make(map[string]store.InputValue, len(in))
	for k, v := range in {
		// Strip the raw scalar; agents that need the value should use
		// `meatbag input get --reveal`. Preserve enough metadata for the
		// caller to know what's there.
		out[k] = store.InputValue{
			HasValue:  v.HasValue,
			SecretRef: v.SecretRef,
			BlobRef:   v.BlobRef,
			Filename:  v.Filename,
			Size:      v.Size,
		}
	}
	return out
}

// exitWith prints msg to stderr (unless --quiet) and exits with code. Used
// for non-zero exits where we want a specific shell convention code rather
// than the cobra default of 1.
func exitWith(code int, msg string) {
	if msg != "" && !gFlags.Quiet {
		fmt.Fprintln(os.Stderr, msg)
	}
	os.Exit(code)
}
