package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/fisherevans/meatbag/internal/daemon"
	"github.com/fisherevans/meatbag/internal/store"
)

// installVersion is the string surfaced in the success line. Kept in sync with
// daemon.Server.Version; not wired through a build flag yet.
const installVersion = "0.1.0"

func newInstallCmd() *cobra.Command {
	var target string
	var noRestart bool
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install or upgrade the meatbag binary (stops/restarts the daemon)",
		Long: "Atomically swap the meatbag binary at the target path. If the " +
			"daemon is running, it is stopped (SIGTERM) before the swap and " +
			"restarted afterwards so the new version is live immediately.\n\n" +
			"Default target is $HOME/.local/bin/meatbag (or " +
			"$XDG_BIN_HOME/meatbag if XDG_BIN_HOME is set). Pass --target to " +
			"override; a directory is treated as <dir>/meatbag.\n\n" +
			"Schema migrations: list files migrate forward lazily on next " +
			"read/save, so restarting the daemon is enough - no explicit " +
			"migration step.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstall(target, noRestart)
		},
	}
	cmd.Flags().StringVar(&target, "target", "", "install path (file or directory; default $HOME/.local/bin/meatbag)")
	cmd.Flags().BoolVar(&noRestart, "no-restart", false, "skip daemon restart even if it was running before install")
	return cmd
}

func runInstall(target string, noRestart bool) error {
	src, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve current binary: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(src); err == nil {
		src = resolved
	}

	targetPath, err := resolveTarget(target)
	if err != nil {
		return err
	}

	srcAbs, _ := filepath.Abs(src)
	tgtAbs, _ := filepath.Abs(targetPath)
	if tgtResolved, err := filepath.EvalSymlinks(tgtAbs); err == nil {
		tgtAbs = tgtResolved
	}
	if srcAbs == tgtAbs {
		return fmt.Errorf("source and target are the same path (%s); nothing to install", srcAbs)
	}

	targetDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("create target directory: %w", err)
	}

	// Check daemon status. Store errors are fatal; missing state is fine.
	s, err := openStore()
	if err != nil {
		return err
	}
	daemonWasRunning := false
	if st, err := daemon.ReadState(s); err == nil && daemon.IsRunning(st) {
		daemonWasRunning = true
		if !gFlags.Quiet && !gFlags.JSON {
			fmt.Printf("stopping daemon (pid %d)...\n", st.PID)
		}
		if err := daemon.Stop(s); err != nil {
			return fmt.Errorf("stop daemon: %w", err)
		}
	}

	if err := atomicInstall(src, targetPath); err != nil {
		return err
	}

	var restartedState *daemon.State
	if daemonWasRunning && !noRestart {
		st, err := spawnNewBinary(targetPath, s)
		if err != nil {
			return fmt.Errorf("restart daemon with new binary: %w", err)
		}
		restartedState = st
	}

	// PATH check.
	pathWarning := pathHint(targetDir)

	// Emit results.
	out := map[string]any{
		"installed_to":     targetPath,
		"version":          installVersion,
		"restarted_daemon": restartedState != nil,
	}
	if restartedState != nil {
		out["port"] = restartedState.Port
		out["url"] = fmt.Sprintf("http://127.0.0.1:%d", restartedState.Port)
	}
	if pathWarning != "" {
		out["path_warning"] = pathWarning
	}

	var b strings.Builder
	fmt.Fprintf(&b, "installed meatbag %s to %s", installVersion, targetPath)
	if restartedState != nil {
		fmt.Fprintf(&b, "\ndaemon restarted: http://127.0.0.1:%d", restartedState.Port)
	} else if daemonWasRunning && noRestart {
		fmt.Fprint(&b, "\ndaemon was stopped; --no-restart set, leaving it stopped")
	}
	if pathWarning != "" {
		fmt.Fprintf(&b, "\n%s", pathWarning)
	}
	return emit(out, b.String())
}

// resolveTarget normalises the --target flag value into an absolute file path
// ending in /meatbag. Empty input picks the default location.
func resolveTarget(target string) (string, error) {
	if target == "" {
		base, err := defaultBinDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(base, "meatbag"), nil
	}
	// Expand ~/ manually since we're not going through a shell.
	if strings.HasPrefix(target, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		target = filepath.Join(home, target[2:])
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	// If target exists and is a directory, append "meatbag". If it doesn't
	// exist and ends with a separator, treat as a directory.
	if info, err := os.Stat(abs); err == nil && info.IsDir() {
		return filepath.Join(abs, "meatbag"), nil
	}
	if strings.HasSuffix(target, string(os.PathSeparator)) {
		return filepath.Join(abs, "meatbag"), nil
	}
	return abs, nil
}

func defaultBinDir() (string, error) {
	if xdg := os.Getenv("XDG_BIN_HOME"); xdg != "" {
		return xdg, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "bin"), nil
}

// atomicInstall copies src into a sibling temp file of dst then renames over
// dst. Rename is atomic within a filesystem, so concurrent reads of dst see
// either the old or new binary - never a truncated one.
func atomicInstall(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source binary: %w", err)
	}
	defer in.Close()

	tmp, err := os.CreateTemp(filepath.Dir(dst), filepath.Base(dst)+".new-*")
	if err != nil {
		return fmt.Errorf("create temp file in target dir: %w", err)
	}
	tmpPath := tmp.Name()
	// Best-effort cleanup if anything below fails.
	defer func() {
		if _, err := os.Stat(tmpPath); err == nil {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		return fmt.Errorf("copy binary: %w", err)
	}
	if err := tmp.Chmod(0o755); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod temp binary: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp binary: %w", err)
	}
	if err := os.Rename(tmpPath, dst); err != nil {
		return fmt.Errorf("rename into place: %w", err)
	}
	return nil
}

// spawnNewBinary execs `<target> web start` as a detached child, then polls
// state.json until the new daemon registers. Mirrors daemon.StartBackground
// but uses the freshly-installed binary instead of os.Executable() so the new
// code is the one serving requests.
func spawnNewBinary(target string, s *store.Store) (*daemon.State, error) {
	// Defensive: refuse if something started up before us.
	if st, err := daemon.ReadState(s); err == nil && daemon.IsRunning(st) {
		return nil, errors.New("daemon already running after stop; refusing to spawn another")
	}

	logFile, err := os.OpenFile(s.LogPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open log: %w", err)
	}
	defer logFile.Close()

	args := []string{"web", "start", "--foreground"}
	// Propagate --home so the child writes state to the matching store.
	if gFlags.Home != "" {
		args = append(args, "--home", gFlags.Home)
	}
	cmd := exec.Command(target, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil
	cmd.Env = os.Environ()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	go func() { _ = cmd.Process.Release() }()

	// Poll for state.json. The child is `web start` (default, not --foreground)
	// which itself calls daemon.StartBackground and waits on state.json before
	// exiting, but we still poll here to surface the final state to the caller.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if st, err := daemon.ReadState(s); err == nil && daemon.IsRunning(st) {
			return st, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil, errors.New("daemon failed to start within 10s; check daemon.log")
}

// pathHint returns a friendly warning if dir isn't on $PATH, or empty if it is.
func pathHint(dir string) string {
	absDir, _ := filepath.Abs(dir)
	for _, entry := range filepath.SplitList(os.Getenv("PATH")) {
		if entry == "" {
			continue
		}
		if abs, err := filepath.Abs(entry); err == nil && abs == absDir {
			return ""
		}
	}
	return fmt.Sprintf("warning: %s is not on $PATH. Add it with:\n  export PATH=%q:$PATH\nin ~/.zshrc or ~/.bashrc.", dir, dir)
}
