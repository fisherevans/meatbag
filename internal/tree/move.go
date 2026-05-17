package tree

import (
	"errors"
	"fmt"

	"github.com/fisherevans/meatbag/internal/store"
)

var (
	ErrCycle = errors.New("move would create a cycle")
)

// MoveOptions specifies where an item should land. Pass Parent to nest under
// a specific item (empty = list root). Pass After or Before to position
// relative to a sibling. After/Before are mutually exclusive; if either is
// set, Parent is inferred (and validated if also supplied).
type MoveOptions struct {
	ParentID string
	AfterID  string
	BeforeID string
}

// Move relocates an item within the list per opts.
func Move(list *store.List, itemID string, opts MoveOptions) error {
	if itemID == "" {
		return fmt.Errorf("itemID required")
	}
	if opts.AfterID != "" && opts.BeforeID != "" {
		return fmt.Errorf("--after and --before are mutually exclusive")
	}

	item, srcParent, srcChildren := findWithParent(list, itemID)
	if item == nil {
		return store.ErrNotFound
	}

	// Cycle check first: ensure dest parent is not item itself or in its subtree.
	destParentID, err := resolveDestParent(list, opts)
	if err != nil {
		return err
	}
	if destParentID == item.ID || isDescendant(item, destParentID) {
		return ErrCycle
	}

	// Compute pre-detach destination index, using the live tree.
	destIdx, err := resolveDestIndex(list, opts)
	if err != nil {
		return err
	}

	// Detach from source.
	newSrcChildren := remove(srcChildren, item)
	writeChildren(list, srcParent, newSrcChildren)

	// If src and dest share the same parent and we removed from before
	// the dest position, adjust.
	srcParentID := idOf(srcParent)
	if srcParentID == destParentID {
		oldIdx := indexOf(srcChildren, item.ID)
		if oldIdx >= 0 && oldIdx < destIdx {
			destIdx--
		}
	}

	// Re-fetch the dest children slice (may have been mutated above).
	destChildren := childrenByID(list, destParentID)
	if destIdx < 0 {
		destIdx = 0
	}
	if destIdx > len(destChildren) {
		destIdx = len(destChildren)
	}

	// Insert.
	newDest := make([]*store.Item, 0, len(destChildren)+1)
	newDest = append(newDest, destChildren[:destIdx]...)
	newDest = append(newDest, item)
	newDest = append(newDest, destChildren[destIdx:]...)
	writeChildren(list, parentByID(list, destParentID), newDest)
	return nil
}

// resolveDestParent returns the ID of the item that should own the moved item
// after the operation, or "" for list root.
func resolveDestParent(list *store.List, opts MoveOptions) (string, error) {
	if opts.AfterID != "" {
		_, parent, _ := findWithParent(list, opts.AfterID)
		pid := idOf(parent)
		if opts.ParentID != "" && opts.ParentID != pid {
			return "", fmt.Errorf("--after sibling is not under --parent")
		}
		return pid, nil
	}
	if opts.BeforeID != "" {
		_, parent, _ := findWithParent(list, opts.BeforeID)
		pid := idOf(parent)
		if opts.ParentID != "" && opts.ParentID != pid {
			return "", fmt.Errorf("--before sibling is not under --parent")
		}
		return pid, nil
	}
	return opts.ParentID, nil
}

// resolveDestIndex returns the desired insertion index in the dest parent's
// children, assuming the tree is unmodified.
func resolveDestIndex(list *store.List, opts MoveOptions) (int, error) {
	if opts.AfterID != "" {
		_, parent, children := findWithParent(list, opts.AfterID)
		if parent == nil && children == nil {
			return 0, fmt.Errorf("after: %w", store.ErrNotFound)
		}
		i := indexOf(children, opts.AfterID)
		if i < 0 {
			return 0, fmt.Errorf("after: %w", store.ErrNotFound)
		}
		return i + 1, nil
	}
	if opts.BeforeID != "" {
		_, parent, children := findWithParent(list, opts.BeforeID)
		if parent == nil && children == nil {
			return 0, fmt.Errorf("before: %w", store.ErrNotFound)
		}
		i := indexOf(children, opts.BeforeID)
		if i < 0 {
			return 0, fmt.Errorf("before: %w", store.ErrNotFound)
		}
		return i, nil
	}
	// Append at end of dest parent
	if opts.ParentID == "" {
		return len(list.Items), nil
	}
	parent, _, _ := findWithParent(list, opts.ParentID)
	if parent == nil {
		return 0, fmt.Errorf("parent: %w", store.ErrNotFound)
	}
	return len(parent.Children), nil
}

// findWithParent returns the item, its parent *Item (nil if at list root), and
// the children slice it lives in.
func findWithParent(list *store.List, id string) (*store.Item, *store.Item, []*store.Item) {
	var found, foundParent *store.Item
	var foundChildren []*store.Item
	var walk func(parent *store.Item, items []*store.Item)
	walk = func(parent *store.Item, items []*store.Item) {
		if found != nil {
			return
		}
		for _, it := range items {
			if it.ID == id {
				found = it
				foundParent = parent
				foundChildren = items
				return
			}
			walk(it, it.Children)
		}
	}
	walk(nil, list.Items)
	return found, foundParent, foundChildren
}

func childrenByID(list *store.List, id string) []*store.Item {
	if id == "" {
		return list.Items
	}
	if p, _, _ := findWithParent(list, id); p != nil {
		return p.Children
	}
	return nil
}

func parentByID(list *store.List, id string) *store.Item {
	if id == "" {
		return nil
	}
	p, _, _ := findWithParent(list, id)
	return p
}

func writeChildren(list *store.List, parent *store.Item, children []*store.Item) {
	if parent == nil {
		list.Items = children
	} else {
		parent.Children = children
	}
}

func idOf(parent *store.Item) string {
	if parent == nil {
		return ""
	}
	return parent.ID
}

func remove(slice []*store.Item, item *store.Item) []*store.Item {
	out := make([]*store.Item, 0, len(slice))
	for _, it := range slice {
		if it != item {
			out = append(out, it)
		}
	}
	return out
}

func indexOf(slice []*store.Item, id string) int {
	for i, it := range slice {
		if it.ID == id {
			return i
		}
	}
	return -1
}

func isDescendant(root *store.Item, id string) bool {
	if id == "" {
		return false
	}
	for _, c := range root.Children {
		if c.ID == id {
			return true
		}
		if isDescendant(c, id) {
			return true
		}
	}
	return false
}

// AppendChild adds a new item under parent (empty = root). Convenience for the
// CLI `item add` command.
func AppendChild(list *store.List, parentID string, item *store.Item) error {
	if parentID == "" {
		list.Items = append(list.Items, item)
		return nil
	}
	p, _, _ := findWithParent(list, parentID)
	if p == nil {
		return store.ErrNotFound
	}
	p.Children = append(p.Children, item)
	return nil
}

// InsertAt inserts an item into the list at the position implied by opts.
// Unlike Move, this is for new items that are not yet in the tree.
func InsertAt(list *store.List, item *store.Item, opts MoveOptions) error {
	if opts.AfterID != "" && opts.BeforeID != "" {
		return fmt.Errorf("--after and --before are mutually exclusive")
	}
	destParentID, err := resolveDestParent(list, opts)
	if err != nil {
		return err
	}
	destIdx, err := resolveDestIndex(list, opts)
	if err != nil {
		return err
	}
	destChildren := childrenByID(list, destParentID)
	if destIdx < 0 {
		destIdx = 0
	}
	if destIdx > len(destChildren) {
		destIdx = len(destChildren)
	}
	newDest := make([]*store.Item, 0, len(destChildren)+1)
	newDest = append(newDest, destChildren[:destIdx]...)
	newDest = append(newDest, item)
	newDest = append(newDest, destChildren[destIdx:]...)
	writeChildren(list, parentByID(list, destParentID), newDest)
	return nil
}

// Remove detaches the item from the tree and returns it. Children come along.
func Remove(list *store.List, itemID string) (*store.Item, error) {
	item, parent, children := findWithParent(list, itemID)
	if item == nil {
		return nil, store.ErrNotFound
	}
	writeChildren(list, parent, remove(children, item))
	return item, nil
}
