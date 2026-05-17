package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fisherevans/meatbag/internal/store"
)

// openStore returns a Store rooted at gFlags.Home or the default.
func openStore() (*store.Store, error) {
	return store.New(gFlags.Home)
}

// emitJSON prints v as indented JSON to stdout.
func emitJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// emit writes either JSON or a text-formatted version of v.
// `text` is the human-readable fallback string (already formatted, no trailing
// newline required).
func emit(v any, text string) error {
	if gFlags.JSON {
		return emitJSON(v)
	}
	if gFlags.Quiet {
		return nil
	}
	if text != "" {
		fmt.Println(text)
	}
	return nil
}

// readContentArg interprets a content flag value: leading "@" means file path
// (or "@-" for stdin), otherwise the literal string is used.
func readContentArg(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	if !strings.HasPrefix(s, "@") {
		return s, nil
	}
	path := s[1:]
	var data []byte
	var err error
	if path == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// listProgress summarizes counts of items by state, walking the whole tree.
type listProgress struct {
	Todo          int `json:"todo"`
	InProgress    int `json:"in_progress"`
	Blocked       int `json:"blocked"`
	Done          int `json:"done"`
	Skipped       int `json:"skipped"`
	AwaitingInput int `json:"awaiting_input"`
}

func progressOf(list *store.List) listProgress {
	var p listProgress
	var walk func(items []*store.Item)
	walk = func(items []*store.Item) {
		for _, it := range items {
			switch it.State {
			case store.StateTodo:
				p.Todo++
			case store.StateInProgress:
				p.InProgress++
			case store.StateBlocked:
				p.Blocked++
			case store.StateDone:
				p.Done++
			case store.StateSkipped:
				p.Skipped++
			}
			for _, in := range it.Inputs {
				v, ok := it.InputValues[in.Name]
				if in.Required && (!ok || !v.HasValue) && it.State != store.StateDone && it.State != store.StateSkipped {
					p.AwaitingInput++
					break
				}
			}
			walk(it.Children)
		}
	}
	walk(list.Items)
	return p
}

// fail returns a cobra-friendly error after printing context.
func fail(msg string, err error) error {
	if errors.Is(err, store.ErrNotFound) {
		return fmt.Errorf("%s: not found", msg)
	}
	return fmt.Errorf("%s: %w", msg, err)
}
