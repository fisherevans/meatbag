package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/fisherevans/meatbag/internal/blobs"
	"github.com/fisherevans/meatbag/internal/secrets"
	"github.com/fisherevans/meatbag/internal/store"
	"github.com/fisherevans/meatbag/internal/tree"
)

func newItemCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "item",
		Aliases: []string{"i"},
		Short:   "Manage items within a list",
	}
	cmd.AddCommand(
		itemAddCmd(),
		itemShowCmd(),
		itemUpdateCmd(),
		itemMoveCmd(),
		itemStateCmd(),
		itemDeleteCmd(),
	)
	return cmd
}

// resolveItemRef returns the item's stable ID given either an ID or a label.
// Used so flags like --parent / --after / --before can accept either form.
func resolveItemRef(list *store.List, idOrLabel string) (string, error) {
	if idOrLabel == "" {
		return "", nil
	}
	it, ok := tree.Resolve(list, idOrLabel)
	if !ok {
		return "", fmt.Errorf("item not found: %s", idOrLabel)
	}
	return it.ID, nil
}

func itemAddCmd() *cobra.Command {
	var (
		title, owner, content, inputsPath, parent, after, before string
	)
	cmd := &cobra.Command{
		Use:   "add <list>",
		Short: "Add a new item",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if title == "" {
				return fmt.Errorf("--title is required")
			}
			s, err := openStore()
			if err != nil {
				return err
			}
			ownerVal := store.Owner(owner)
			if owner == "" {
				ownerVal = store.OwnerHuman
			}
			if !store.ValidOwner(ownerVal) {
				return fmt.Errorf("--owner must be human or agent")
			}
			body, err := readContentArg(content)
			if err != nil {
				return err
			}
			var inputs []store.Input
			if inputsPath != "" {
				data, err := os.ReadFile(inputsPath)
				if err != nil {
					return fmt.Errorf("read --inputs: %w", err)
				}
				if err := yaml.Unmarshal(data, &inputs); err != nil {
					return fmt.Errorf("parse --inputs: %w", err)
				}
				for _, in := range inputs {
					if !validInputType(in.Type) {
						return fmt.Errorf("invalid input type %q on field %q", in.Type, in.Name)
					}
				}
			}
			it := &store.Item{
				ID:      store.NewItemID(),
				Title:   title,
				Owner:   ownerVal,
				State:   store.StateTodo,
				Content: body,
				Inputs:  inputs,
			}
			err = s.Update(args[0], func(l *store.List) error {
				parentID, err := resolveItemRef(l, parent)
				if err != nil {
					return err
				}
				afterID, err := resolveItemRef(l, after)
				if err != nil {
					return err
				}
				beforeID, err := resolveItemRef(l, before)
				if err != nil {
					return err
				}
				return tree.InsertAt(l, it, tree.MoveOptions{
					ParentID: parentID,
					AfterID:  afterID,
					BeforeID: beforeID,
				})
			})
			if err != nil {
				return fail("add", err)
			}
			return emit(map[string]string{"id": it.ID, "title": it.Title}, fmt.Sprintf("added %s (%s)", it.ID, it.Title))
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "item title (required)")
	cmd.Flags().StringVar(&owner, "owner", "human", "human | agent")
	cmd.Flags().StringVar(&content, "content", "", "markdown body (use @file or @-)")
	cmd.Flags().StringVar(&inputsPath, "inputs", "", "path to a YAML file defining input fields")
	cmd.Flags().StringVar(&parent, "parent", "", "parent item id or label")
	cmd.Flags().StringVar(&after, "after", "", "place after this sibling")
	cmd.Flags().StringVar(&before, "before", "", "place before this sibling")
	return cmd
}

func validInputType(t string) bool {
	switch t {
	case "text", "textarea", "password", "number", "url",
		"select", "multiselect", "radio", "checkbox", "file", "markdown":
		return true
	}
	return false
}

func itemShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <list> <item>",
		Short: "Show a single item",
		Args:  cobra.ExactArgs(2),
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
			it, ok := tree.Resolve(l, args[1])
			if !ok {
				return fmt.Errorf("item not found: %s", args[1])
			}
			label := tree.Labels(l)[it.ID]
			if gFlags.JSON {
				return emitJSON(map[string]any{
					"id": it.ID, "label": label, "title": it.Title,
					"owner": it.Owner, "state": it.State, "content": it.Content,
					"inputs": it.Inputs, "input_values": it.InputValues,
					"note": it.Note, "children": it.Children,
				})
			}
			fmt.Printf("%s %s %s\n", stateMarker(it.State), label, it.Title)
			fmt.Printf("id: %s   owner: %s\n", it.ID, it.Owner)
			if it.Note != "" {
				fmt.Printf("note: %s\n", it.Note)
			}
			if it.Content != "" {
				fmt.Println()
				fmt.Println(it.Content)
			}
			if len(it.Inputs) > 0 {
				fmt.Println("\ninputs:")
				for _, in := range it.Inputs {
					flag := ""
					if v, ok := it.InputValues[in.Name]; ok && v.HasValue {
						flag = " [set]"
					} else if in.Required {
						flag = " [required]"
					}
					fmt.Printf("  %s: %s%s\n", in.Name, in.Type, flag)
				}
			}
			return nil
		},
	}
	return cmd
}

func itemUpdateCmd() *cobra.Command {
	var title, content, owner, inputsPath string
	cmd := &cobra.Command{
		Use:   "update <list> <item>",
		Short: "Update an item's title, content, owner, or input schema",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore()
			if err != nil {
				return err
			}
			err = s.Update(args[0], func(l *store.List) error {
				it, ok := tree.Resolve(l, args[1])
				if !ok {
					return fmt.Errorf("item not found: %s", args[1])
				}
				if cmd.Flags().Changed("title") {
					it.Title = title
				}
				if cmd.Flags().Changed("content") {
					body, err := readContentArg(content)
					if err != nil {
						return err
					}
					it.Content = body
				}
				if cmd.Flags().Changed("owner") {
					ownerVal := store.Owner(owner)
					if !store.ValidOwner(ownerVal) {
						return fmt.Errorf("--owner must be human or agent")
					}
					it.Owner = ownerVal
				}
				if cmd.Flags().Changed("inputs") {
					data, err := os.ReadFile(inputsPath)
					if err != nil {
						return err
					}
					var inputs []store.Input
					if err := yaml.Unmarshal(data, &inputs); err != nil {
						return err
					}
					for _, in := range inputs {
						if !validInputType(in.Type) {
							return fmt.Errorf("invalid input type %q on field %q", in.Type, in.Name)
						}
					}
					it.Inputs = inputs
				}
				return nil
			})
			if err != nil {
				return fail("update", err)
			}
			return emit(map[string]string{"item": args[1]}, "updated "+args[1])
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "new title")
	cmd.Flags().StringVar(&content, "content", "", "new markdown body (@file or @-)")
	cmd.Flags().StringVar(&owner, "owner", "", "human | agent")
	cmd.Flags().StringVar(&inputsPath, "inputs", "", "replace inputs from YAML file")
	return cmd
}

func itemMoveCmd() *cobra.Command {
	var parent, after, before string
	cmd := &cobra.Command{
		Use:   "move <list> <item>",
		Short: "Re-parent or reorder an item",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore()
			if err != nil {
				return err
			}
			err = s.Update(args[0], func(l *store.List) error {
				itemID, err := resolveItemRef(l, args[1])
				if err != nil {
					return err
				}
				parentID, err := resolveItemRef(l, parent)
				if err != nil {
					return err
				}
				afterID, err := resolveItemRef(l, after)
				if err != nil {
					return err
				}
				beforeID, err := resolveItemRef(l, before)
				if err != nil {
					return err
				}
				return tree.Move(l, itemID, tree.MoveOptions{
					ParentID: parentID,
					AfterID:  afterID,
					BeforeID: beforeID,
				})
			})
			if err != nil {
				return fail("move", err)
			}
			return emit(map[string]string{"item": args[1]}, "moved "+args[1])
		},
	}
	cmd.Flags().StringVar(&parent, "parent", "", "place under this parent (empty = root)")
	cmd.Flags().StringVar(&after, "after", "", "place after this sibling")
	cmd.Flags().StringVar(&before, "before", "", "place before this sibling")
	return cmd
}

func itemStateCmd() *cobra.Command {
	var note string
	cmd := &cobra.Command{
		Use:   "state <list> <item> <state>",
		Short: "Set state: todo | in_progress | blocked | done | skipped",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			state := store.State(args[2])
			if !store.ValidState(state) {
				return fmt.Errorf("invalid state %q", args[2])
			}
			s, err := openStore()
			if err != nil {
				return err
			}
			err = s.Update(args[0], func(l *store.List) error {
				it, ok := tree.Resolve(l, args[1])
				if !ok {
					return fmt.Errorf("item not found: %s", args[1])
				}
				it.State = state
				if cmd.Flags().Changed("note") {
					it.Note = note
				}
				return nil
			})
			if err != nil {
				return fail("state", err)
			}
			return emit(map[string]string{"item": args[1], "state": args[2]}, fmt.Sprintf("%s -> %s", args[1], args[2]))
		},
	}
	cmd.Flags().StringVar(&note, "note", "", "optional status note")
	return cmd
}

func itemDeleteCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <list> <item>",
		Short: "Delete an item (recursive); purges its secrets and blobs",
		Long: "Delete an item and its entire subtree. This is irreversible: the " +
			"item, all descendant items, and every input value attached to " +
			"any of them are removed. Every referenced Keychain entry " +
			"(service=meatbag) is deleted and every content-addressed blob " +
			"in ~/.meatbag/blobs/ referenced by the subtree is purged. " +
			"Requires --yes.",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return fmt.Errorf("refusing to delete without --yes")
			}
			s, err := openStore()
			if err != nil {
				return err
			}
			var purgedSecrets, purgedBlobs int
			err = s.Update(args[0], func(l *store.List) error {
				itemID, err := resolveItemRef(l, args[1])
				if err != nil {
					return err
				}
				removed, err := tree.Remove(l, itemID)
				if err != nil {
					return err
				}
				bs, _ := blobs.New(s.BlobsDir())
				c := purgeRefs([]*store.Item{removed}, bs)
				purgedSecrets, purgedBlobs = c.secrets, c.blobs
				_ = secrets.Service // keep import in case
				return nil
			})
			if err != nil {
				return fail("delete", err)
			}
			return emit(map[string]any{
				"item":           args[1],
				"secrets_purged": purgedSecrets,
				"blobs_purged":   purgedBlobs,
			}, fmt.Sprintf("deleted %s (purged %d secrets, %d blobs)", args[1], purgedSecrets, purgedBlobs))
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm deletion (required)")
	return cmd
}
