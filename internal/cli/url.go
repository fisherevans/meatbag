package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/spf13/cobra"

	"github.com/fisherevans/meatbag/internal/tree"
)

const defaultDaemonPort = 7421

// daemonState mirrors the JSON written by the daemon at start time.
type daemonState struct {
	Port      int    `json:"port"`
	PID       int    `json:"pid"`
	StartedAt string `json:"started_at"`
	Version   string `json:"version"`
}

func readDaemonState() (*daemonState, error) {
	s, err := openStore()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(s.StatePath())
	if err != nil {
		return nil, err
	}
	var st daemonState
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

// daemonPort returns the port from state.json, or the default if state isn't readable.
func daemonPort() int {
	st, err := readDaemonState()
	if err != nil || st == nil || st.Port == 0 {
		return defaultDaemonPort
	}
	return st.Port
}

func newURLCmd() *cobra.Command {
	var field string
	cmd := &cobra.Command{
		Use:   "url <list> [<item>]",
		Short: "Print a deep-link URL to the web UI",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore()
			if err != nil {
				return err
			}
			path, _, err := s.FindPath(args[0])
			if err != nil {
				return fail("url", err)
			}
			l, err := s.LoadList(path)
			if err != nil {
				return err
			}

			port := daemonPort()
			u := &url.URL{
				Scheme: "http",
				Host:   fmt.Sprintf("127.0.0.1:%d", port),
				Path:   "/lists/" + l.Slug,
			}
			if len(args) == 2 {
				it, ok := tree.Resolve(l, args[1])
				if !ok {
					return fmt.Errorf("item not found: %s", args[1])
				}
				frag := "item-" + it.ID
				if field != "" {
					frag = "input-" + it.ID + "-" + field
				}
				u.Fragment = frag
			}
			if gFlags.JSON {
				return emitJSON(map[string]string{"url": u.String(), "slug": l.Slug})
			}
			fmt.Println(u.String())
			// Hint if daemon not running.
			if _, err := readDaemonState(); err != nil {
				fmt.Fprintln(os.Stderr, "(daemon not running; start it with `meatbag web start`)")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&field, "field", "", "focus a specific input field on the item")
	return cmd
}
