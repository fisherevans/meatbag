package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/fisherevans/meatbag/internal/blobs"
	"github.com/fisherevans/meatbag/internal/secrets"
	"github.com/fisherevans/meatbag/internal/store"
)

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"l"},
		Short:   "Manage to-do lists",
	}
	cmd.AddCommand(
		listCreateCmd(),
		listLsCmd(),
		listShowCmd(),
		listUpdateCmd(),
		listArchiveCmd(),
		listUnarchiveCmd(),
		listDeleteCmd(),
	)
	return cmd
}

func listCreateCmd() *cobra.Command {
	var (
		title, slug, project, description string
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new list",
		RunE: func(cmd *cobra.Command, args []string) error {
			if title == "" {
				return fmt.Errorf("--title is required")
			}
			s, err := openStore()
			if err != nil {
				return err
			}
			desc, err := readContentArg(description)
			if err != nil {
				return err
			}
			proj := project
			if proj == "" {
				cwd, _ := os.Getwd()
				proj = cwd
			} else if proj == "." {
				cwd, _ := os.Getwd()
				proj = cwd
			} else {
				if abs, err := filepath.Abs(proj); err == nil {
					proj = abs
				}
			}
			l := &store.List{
				Title:       title,
				Slug:        slug,
				Description: desc,
				ProjectPath: proj,
			}
			if _, err := s.Create(l); err != nil {
				return fail("create", err)
			}
			return emit(l, fmt.Sprintf("created list %s (id=%s)", l.Slug, l.ID))
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "list title (required)")
	cmd.Flags().StringVar(&slug, "slug", "", "explicit slug (default: derived from title)")
	cmd.Flags().StringVar(&project, "project", "", "project path (default: cwd; '.' resolves cwd)")
	cmd.Flags().StringVar(&description, "description", "", "markdown description (use @file for path, @- for stdin)")
	return cmd
}

func listLsCmd() *cobra.Command {
	var status, project string
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List all to-do lists",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore()
			if err != nil {
				return err
			}
			st := status
			if st == "" {
				st = "active"
			}
			all, err := s.ListAll(st)
			if err != nil {
				return err
			}
			if project != "" {
				if project == "." {
					project, _ = os.Getwd()
				} else if abs, err := filepath.Abs(project); err == nil {
					project = abs
				}
				kept := all[:0]
				for _, l := range all {
					if l.ProjectPath == project {
						kept = append(kept, l)
					}
				}
				all = kept
			}
			sort.Slice(all, func(i, j int) bool { return all[i].UpdatedAt.After(all[j].UpdatedAt) })
			if gFlags.JSON {
				type row struct {
					ID          string `json:"id"`
					Slug        string `json:"slug"`
					Title       string `json:"title"`
					ProjectPath string `json:"project_path,omitempty"`
					Status      string `json:"status"`
					Progress    listProgress `json:"progress"`
				}
				rows := make([]row, 0, len(all))
				for _, l := range all {
					rows = append(rows, row{l.ID, l.Slug, l.Title, l.ProjectPath, string(l.Status), progressOf(l)})
				}
				return emitJSON(rows)
			}
			if len(all) == 0 {
				fmt.Println("(no lists)")
				return nil
			}
			for _, l := range all {
				p := progressOf(l)
				done := p.Done + p.Skipped
				total := p.Todo + p.InProgress + p.Blocked + done
				fmt.Printf("%-25s %-12s %d/%d done", l.Slug, l.Status, done, total)
				if p.AwaitingInput > 0 {
					fmt.Printf(" (%d awaiting input)", p.AwaitingInput)
				}
				if l.ProjectPath != "" {
					fmt.Printf("  [%s]", l.ProjectPath)
				}
				fmt.Printf("  -- %s\n", l.Title)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&status, "status", "active", "active | archived | all")
	cmd.Flags().StringVar(&project, "project", "", "filter to lists for this project path ('.' resolves cwd)")
	return cmd
}

func listShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <list>",
		Short: "Show a list and its items",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore()
			if err != nil {
				return err
			}
			path, _, err := s.FindPath(args[0])
			if err != nil {
				return fail("show", err)
			}
			l, err := s.LoadList(path)
			if err != nil {
				return err
			}
			if gFlags.JSON {
				return emitJSON(l)
			}
			fmt.Printf("# %s [%s]\n", l.Title, l.Slug)
			fmt.Printf("id: %s   status: %s   updated: %s\n", l.ID, l.Status, l.UpdatedAt.Format("2006-01-02 15:04"))
			if l.ProjectPath != "" {
				fmt.Printf("project: %s\n", l.ProjectPath)
			}
			if l.Description != "" {
				fmt.Println()
				fmt.Println(l.Description)
			}
			fmt.Println()
			renderTree(l.Items, "", 0)
			return nil
		},
	}
	return cmd
}

func renderTree(items []*store.Item, prefix string, depth int) {
	for i, it := range items {
		label := prefix + labelSegment(depth, i)
		marker := stateMarker(it.State)
		owner := ""
		if it.Owner == store.OwnerAgent {
			owner = " (agent)"
		}
		fmt.Printf("  %s %s %s%s\n", marker, label, it.Title, owner)
		if it.Note != "" {
			fmt.Printf("      note: %s\n", it.Note)
		}
		if len(it.Inputs) > 0 {
			for _, in := range it.Inputs {
				flag := ""
				if v, ok := it.InputValues[in.Name]; ok && v.HasValue {
					flag = " [set]"
				} else if in.Required {
					flag = " [required]"
				}
				fmt.Printf("      input %s: %s%s\n", in.Name, in.Type, flag)
			}
		}
		if len(it.Children) > 0 {
			renderTree(it.Children, label, depth+1)
		}
	}
}

func stateMarker(s store.State) string {
	switch s {
	case store.StateDone:
		return "[x]"
	case store.StateSkipped:
		return "[-]"
	case store.StateInProgress:
		return "[~]"
	case store.StateBlocked:
		return "[!]"
	default:
		return "[ ]"
	}
}

// labelSegment mirrors tree.LabelForIndex but is inlined here so the CLI
// renderer doesn't pull in tree (it would create a cycle later).
func labelSegment(depth, idx int) string {
	// Keep in sync with tree.LabelForIndex.
	switch depth % 4 {
	case 0:
		return fmt.Sprintf("%d", idx+1)
	case 1:
		return fmt.Sprintf(".%d", idx+1)
	case 2:
		var out []byte
		n := idx + 1
		for n > 0 {
			n--
			out = append([]byte{byte('a' + n%26)}, out...)
			n /= 26
		}
		return string(out)
	case 3:
		return "." + roman(idx+1)
	}
	return ""
}

func roman(n int) string {
	if n <= 0 || n >= 4000 {
		return ""
	}
	vals := []int{1000, 900, 500, 400, 100, 90, 50, 40, 10, 9, 5, 4, 1}
	syms := []string{"m", "cm", "d", "cd", "c", "xc", "l", "xl", "x", "ix", "v", "iv", "i"}
	var b strings.Builder
	for i, v := range vals {
		for n >= v {
			b.WriteString(syms[i])
			n -= v
		}
	}
	return b.String()
}

func listUpdateCmd() *cobra.Command {
	var title, description string
	cmd := &cobra.Command{
		Use:   "update <list>",
		Short: "Update list title or description",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore()
			if err != nil {
				return err
			}
			err = s.Update(args[0], func(l *store.List) error {
				if cmd.Flags().Changed("title") {
					l.Title = title
				}
				if cmd.Flags().Changed("description") {
					d, err := readContentArg(description)
					if err != nil {
						return err
					}
					l.Description = d
				}
				return nil
			})
			if err != nil {
				return fail("update", err)
			}
			return emit(map[string]string{"slug": args[0]}, "updated "+args[0])
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "new title")
	cmd.Flags().StringVar(&description, "description", "", "new markdown description (@file or @-)")
	return cmd
}

func listArchiveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "archive <list>",
		Short: "Archive a list (hidden from default ls)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore()
			if err != nil {
				return err
			}
			if err := s.Archive(args[0]); err != nil {
				return fail("archive", err)
			}
			return emit(map[string]string{"slug": args[0], "status": "archived"}, "archived "+args[0])
		},
	}
}

func listUnarchiveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unarchive <list>",
		Short: "Restore an archived list",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore()
			if err != nil {
				return err
			}
			if err := s.Unarchive(args[0]); err != nil {
				return fail("unarchive", err)
			}
			return emit(map[string]string{"slug": args[0], "status": "active"}, "restored "+args[0])
		},
	}
}

func listDeleteCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <list>",
		Short: "Delete a list and purge its secrets + blobs",
		Long: "Delete a list and everything under it. This is irreversible: the " +
			"list YAML is removed, every Keychain entry (service=meatbag) " +
			"referenced by any input value is deleted, and every " +
			"content-addressed blob in ~/.meatbag/blobs/ referenced by the " +
			"list's input values is purged. Requires --yes.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return fmt.Errorf("refusing to delete without --yes")
			}
			s, err := openStore()
			if err != nil {
				return err
			}
			path, _, err := s.FindPath(args[0])
			if err != nil {
				return fail("delete", err)
			}
			l, err := s.LoadList(path)
			if err != nil {
				return err
			}
			// Walk and purge external resources first.
			bs, _ := blobs.New(s.BlobsDir())
			purged := purgeRefs(l.Items, bs)
			if err := s.Delete(args[0]); err != nil {
				return err
			}
			return emit(map[string]any{
				"slug":           l.Slug,
				"id":             l.ID,
				"secrets_purged": purged.secrets,
				"blobs_purged":   purged.blobs,
			}, fmt.Sprintf("deleted %s (purged %d secrets, %d blobs)", l.Slug, purged.secrets, purged.blobs))
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm deletion (required)")
	return cmd
}

type purgeCounts struct {
	secrets, blobs int
}

// purgeRefs walks every item in `items` (recursively), deleting any referenced
// keychain secret or blob. Errors are swallowed but counted as not-purged.
func purgeRefs(items []*store.Item, bs *blobs.Store) purgeCounts {
	var c purgeCounts
	var walk func(its []*store.Item)
	walk = func(its []*store.Item) {
		for _, it := range its {
			for _, v := range it.InputValues {
				if v.SecretRef != "" {
					if ref, err := secrets.ParseRef(v.SecretRef); err == nil {
						if secrets.Delete(ref) == nil {
							c.secrets++
						}
					}
				}
				if v.BlobRef != "" {
					if sha, err := blobs.ParseRef(v.BlobRef); err == nil && bs != nil {
						// Only delete if no other list references this blob.
						// Caller (gc) handles cross-list refs; here we delete
						// eagerly since the list is being removed entirely.
						_ = bs.Delete(sha)
						c.blobs++
					}
				}
			}
			walk(it.Children)
		}
	}
	walk(items)
	return c
}
