package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/fisherevans/meatbag/internal/daemon"
)

func newWebCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "web",
		Short: "Run the web daemon (start/stop/status/logs/restart)",
	}
	cmd.AddCommand(
		webStartCmd(),
		webStopCmd(),
		webStatusCmd(),
		webLogsCmd(),
		webRestartCmd(),
	)
	return cmd
}

func webStartCmd() *cobra.Command {
	var port int
	var foreground bool
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the web daemon (default: detached)",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore()
			if err != nil {
				return err
			}
			if foreground {
				ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
				defer cancel()
				return daemon.RunForeground(ctx, s, port)
			}
			st, err := daemon.StartBackground(s, port)
			if err != nil {
				return err
			}
			url := fmt.Sprintf("http://127.0.0.1:%d", st.Port)
			return emit(map[string]any{"pid": st.PID, "port": st.Port, "url": url},
				fmt.Sprintf("daemon started (pid=%d port=%d) -> %s", st.PID, st.Port, url))
		},
	}
	cmd.Flags().IntVar(&port, "port", 7421, "TCP port (default 7421)")
	cmd.Flags().BoolVar(&foreground, "foreground", false, "run in the current shell (no detach)")
	return cmd
}

func webStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the running web daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore()
			if err != nil {
				return err
			}
			if err := daemon.Stop(s); err != nil {
				return err
			}
			return emit(map[string]bool{"stopped": true}, "stopped")
		},
	}
}

func webStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Report whether the daemon is running, on what port, etc.",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore()
			if err != nil {
				return err
			}
			st, err := daemon.ReadState(s)
			if err != nil {
				if os.IsNotExist(err) {
					return emit(map[string]bool{"running": false}, "not running")
				}
				return err
			}
			alive := daemon.IsRunning(st)
			out := map[string]any{
				"running":    alive,
				"pid":        st.PID,
				"port":       st.Port,
				"started_at": st.StartedAt,
				"version":    st.Version,
				"url":        fmt.Sprintf("http://127.0.0.1:%d", st.Port),
				"log":        s.LogPath(),
			}
			text := fmt.Sprintf("running pid=%d port=%d started=%s\n  url: %s\n  log: %s",
				st.PID, st.Port, st.StartedAt,
				out["url"], out["log"])
			if !alive {
				text = "state.json present but pid " + fmt.Sprint(st.PID) + " not alive"
			}
			return emit(out, text)
		},
	}
}

func webLogsCmd() *cobra.Command {
	var follow bool
	var tail int
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Print daemon logs (--follow to stream)",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore()
			if err != nil {
				return err
			}
			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer cancel()
			return daemon.TailLog(ctx, s, tail, follow, os.Stdout)
		},
	}
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "stream new lines as they arrive")
	cmd.Flags().IntVarP(&tail, "tail", "n", 0, "print last N lines (0 = all)")
	return cmd
}

func webRestartCmd() *cobra.Command {
	var port int
	cmd := &cobra.Command{
		Use:   "restart",
		Short: "Stop then start the daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore()
			if err != nil {
				return err
			}
			if st, err := daemon.ReadState(s); err == nil && daemon.IsRunning(st) {
				if err := daemon.Stop(s); err != nil {
					return err
				}
				// Brief settle.
				time.Sleep(200 * time.Millisecond)
			}
			st, err := daemon.StartBackground(s, port)
			if err != nil {
				return err
			}
			return emit(map[string]any{"pid": st.PID, "port": st.Port},
				fmt.Sprintf("restarted (pid=%d port=%d)", st.PID, st.Port))
		},
	}
	cmd.Flags().IntVar(&port, "port", 7421, "TCP port")
	return cmd
}
