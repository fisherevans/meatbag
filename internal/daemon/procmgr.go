package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/fisherevans/meatbag/internal/store"
)

// State is the JSON we write to ~/.meatbag/state.json so other processes
// (CLI url, status) can discover the running daemon.
type State struct {
	Port      int    `json:"port"`
	PID       int    `json:"pid"`
	StartedAt string `json:"started_at"`
	Version   string `json:"version"`
}

// ReadState loads ~/.meatbag/state.json. Returns os.ErrNotExist if absent.
func ReadState(s *store.Store) (*State, error) {
	data, err := os.ReadFile(s.StatePath())
	if err != nil {
		return nil, err
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

func writeState(s *store.Store, st State) error {
	data, _ := json.MarshalIndent(st, "", "  ")
	return os.WriteFile(s.StatePath(), data, 0o600)
}

func removeState(s *store.Store) {
	_ = os.Remove(s.StatePath())
}

// IsRunning reports whether the recorded PID is alive.
func IsRunning(st *State) bool {
	if st == nil || st.PID <= 0 {
		return false
	}
	p, err := os.FindProcess(st.PID)
	if err != nil {
		return false
	}
	// Signal 0: existence probe. Returns no error if alive and we have permission.
	err = p.Signal(syscall.Signal(0))
	return err == nil
}

// RunForeground is the daemon body: bind, listen, serve until the context is
// canceled (Ctrl-C or SIGTERM). It owns state.json.
func RunForeground(ctx context.Context, s *store.Store, port int) error {
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	// Reserve the port up-front so we can write state.json with the final
	// port (in case port was 0). Then hand the listener to the server.
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	addr = ln.Addr().String()
	_, portStr, _ := net.SplitHostPort(addr)
	port, _ = strconv.Atoi(portStr)

	srv, err := New(s)
	if err != nil {
		ln.Close()
		return err
	}
	st := State{
		Port:      port,
		PID:       os.Getpid(),
		StartedAt: srv.StartedAt.Format(time.RFC3339),
		Version:   srv.Version,
	}
	if err := writeState(s, st); err != nil {
		ln.Close()
		return fmt.Errorf("write state: %w", err)
	}
	defer removeState(s)

	// Wire signals.
	sigCtx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Pass our pre-bound listener to the HTTP server by switching to manual
	// serve. Easier path: close our listener and let Run bind. Race window
	// exists, but the state file already has the chosen port. Acceptable for
	// a local-only daemon.
	ln.Close()
	fmt.Printf("meatbag daemon listening on http://%s\n", addr)
	return srv.Run(sigCtx, addr)
}

// StartBackground forks the current binary as a detached child running in
// foreground mode. Returns the child's State once state.json appears, or an
// error after startTimeout.
func StartBackground(s *store.Store, port int) (*State, error) {
	// Refuse if already running.
	if existing, err := ReadState(s); err == nil && IsRunning(existing) {
		return existing, errors.New("daemon already running")
	}
	// Always clear stale state.
	removeState(s)

	self, err := os.Executable()
	if err != nil {
		return nil, err
	}
	logFile, err := os.OpenFile(s.LogPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open log: %w", err)
	}

	args := []string{"web", "start", "--foreground"}
	if port > 0 {
		args = append(args, "--port", strconv.Itoa(port))
	}
	if s.Home != store.DefaultHome() {
		args = append(args, "--home", s.Home)
	}
	cmd := exec.Command(self, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil
	cmd.Env = os.Environ()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return nil, err
	}
	// Don't wait; let it detach. But reap exit code in goroutine to avoid zombie
	// (Setsid + parent exit handles this on macOS; this is defensive).
	go func() { _ = cmd.Process.Release() }()
	// Close our handle (the child inherited a dup).
	logFile.Close()

	// Poll for state.json.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if st, err := ReadState(s); err == nil && st.PID > 0 {
			return st, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil, errors.New("daemon failed to start within 5s; check daemon.log")
}

// Stop signals the running daemon and waits up to stopTimeout for it to exit.
func Stop(s *store.Store) error {
	st, err := ReadState(s)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errors.New("daemon not running")
		}
		return err
	}
	if !IsRunning(st) {
		removeState(s)
		return errors.New("daemon not running (stale state cleaned)")
	}
	p, err := os.FindProcess(st.PID)
	if err != nil {
		return err
	}
	if err := p.Signal(syscall.SIGTERM); err != nil {
		return err
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !IsRunning(st) {
			removeState(s)
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return errors.New("daemon did not exit within 5s")
}

// TailLog streams lines from the log file. If follow is true, blocks and emits
// new lines as they arrive (until ctx is canceled or the file is removed).
func TailLog(ctx context.Context, s *store.Store, lastN int, follow bool, w io.Writer) error {
	f, err := os.Open(s.LogPath())
	if err != nil {
		return err
	}
	defer f.Close()

	if lastN > 0 {
		// Naive tail: read from end, count newlines back.
		if err := writeLastN(f, lastN, w); err != nil {
			return err
		}
	} else {
		if _, err := io.Copy(w, f); err != nil {
			return err
		}
	}
	if !follow {
		return nil
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		buf := make([]byte, 4096)
		n, err := f.Read(buf)
		if n > 0 {
			_, _ = w.Write(buf[:n])
			continue
		}
		if err == io.EOF {
			time.Sleep(250 * time.Millisecond)
			continue
		}
		if err != nil {
			return err
		}
	}
}

func writeLastN(f *os.File, n int, w io.Writer) error {
	const chunk = 8192
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	size := fi.Size()
	if size == 0 {
		return nil
	}
	var off int64 = size
	lines := 0
	buf := make([]byte, chunk)
	var collected []byte
	for off > 0 && lines <= n {
		read := int64(chunk)
		if off < read {
			read = off
		}
		off -= read
		if _, err := f.Seek(off, io.SeekStart); err != nil {
			return err
		}
		got, err := f.Read(buf[:read])
		if err != nil {
			return err
		}
		collected = append(buf[:got:got], collected...)
		lines = 0
		for _, b := range collected {
			if b == '\n' {
				lines++
			}
		}
	}
	// Trim to last n+1 newlines.
	if lines > n {
		count := 0
		for i := 0; i < len(collected); i++ {
			if collected[i] == '\n' {
				count++
				if count == lines-n {
					collected = collected[i+1:]
					break
				}
			}
		}
	}
	_, err = w.Write(collected)
	return err
}
