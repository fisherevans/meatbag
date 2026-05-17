// Package tree handles the derived numeric labels (1, 1.1, 1.1a, 1.1a.i) and
// position-shaped operations (move, find-by-label) that live on top of the
// flat parent/child item structure in internal/store.
package tree

import (
	"strings"

	"github.com/fisherevans/meatbag/internal/store"
)

// LabelForIndex returns the label segment for a given depth + zero-based position.
//
// Depth scheme (alternates by depth, matches "2.1a.i" style):
//
//	0: "1", "2", ...
//	1: ".1", ".2", ...
//	2: "a", "b", ..., "z", "aa", "ab", ...
//	3: ".i", ".ii", ".iii", ...
//	4+: repeats from depth 0
func LabelForIndex(depth, index int) string {
	switch depth % 4 {
	case 0:
		return itoa(index + 1)
	case 1:
		return "." + itoa(index+1)
	case 2:
		return alpha(index + 1)
	case 3:
		return "." + roman(index+1)
	}
	return ""
}

// LabelForItem walks `path` (each entry is the zero-based index at its depth)
// and joins the segments. Empty path -> "".
func LabelForPath(path []int) string {
	var b strings.Builder
	for d, i := range path {
		b.WriteString(LabelForIndex(d, i))
	}
	return b.String()
}

// Labels returns a map of item ID -> rendered label for every item in the list.
func Labels(list *store.List) map[string]string {
	out := make(map[string]string)
	var walk func(items []*store.Item, prefix string, depth int)
	walk = func(items []*store.Item, prefix string, depth int) {
		for i, it := range items {
			label := prefix + LabelForIndex(depth, i)
			out[it.ID] = label
			if len(it.Children) > 0 {
				walk(it.Children, label, depth+1)
			}
		}
	}
	walk(list.Items, "", 0)
	return out
}

// FindByLabel walks the tree following the label's segments. Returns the item,
// its parent slice, and its index in that slice.
func FindByLabel(list *store.List, label string) (*store.Item, []*store.Item, int, bool) {
	segs := parseLabel(label)
	if len(segs) == 0 {
		return nil, nil, 0, false
	}
	items := list.Items
	var item *store.Item
	idx := 0
	for d, seg := range segs {
		want := indexForSegment(d, seg)
		if want < 0 || want >= len(items) {
			return nil, nil, 0, false
		}
		item = items[want]
		idx = want
		if d < len(segs)-1 {
			items = item.Children
		}
	}
	return item, items, idx, true
}

// FindByID searches every item recursively.
func FindByID(list *store.List, id string) (*store.Item, []*store.Item, int, bool) {
	var found *store.Item
	var foundParent []*store.Item
	var foundIdx int
	var walk func(items []*store.Item) bool
	walk = func(items []*store.Item) bool {
		for i, it := range items {
			if it.ID == id {
				found = it
				foundParent = items
				foundIdx = i
				return true
			}
			if walk(it.Children) {
				return true
			}
		}
		return false
	}
	if walk(list.Items) {
		return found, foundParent, foundIdx, true
	}
	return nil, nil, 0, false
}

// Resolve returns the item matching either a stable ID (starting "it_") or a
// rendered label. The boolean reports whether it was found.
func Resolve(list *store.List, idOrLabel string) (*store.Item, bool) {
	if strings.HasPrefix(idOrLabel, "it_") {
		it, _, _, ok := FindByID(list, idOrLabel)
		return it, ok
	}
	it, _, _, ok := FindByLabel(list, idOrLabel)
	return it, ok
}

// ----- segment encoding -----

func itoa(n int) string {
	if n <= 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// alpha: 1 -> a, 26 -> z, 27 -> aa, 28 -> ab ... (bijective base 26)
func alpha(n int) string {
	if n <= 0 {
		return ""
	}
	var out []byte
	for n > 0 {
		n--
		out = append([]byte{byte('a' + n%26)}, out...)
		n /= 26
	}
	return string(out)
}

func alphaToIndex(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < 'a' || c > 'z' {
			return -1
		}
		n = n*26 + int(c-'a') + 1
	}
	return n - 1
}

// roman: 1 -> i, 4 -> iv, 9 -> ix, etc. Lowercase.
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

func romanToInt(s string) int {
	if s == "" {
		return -1
	}
	pairs := map[byte]int{'i': 1, 'v': 5, 'x': 10, 'l': 50, 'c': 100, 'd': 500, 'm': 1000}
	total := 0
	for i := 0; i < len(s); i++ {
		v, ok := pairs[s[i]]
		if !ok {
			return -1
		}
		if i+1 < len(s) {
			nv := pairs[s[i+1]]
			if nv > v {
				total -= v
				continue
			}
		}
		total += v
	}
	return total - 1 // zero-based index
}

// parseLabel splits a label like "2.1a.ii" into its segments preserving the
// alternation: ["2", "1", "a", "ii"].
func parseLabel(s string) []string {
	if s == "" {
		return nil
	}
	var segs []string
	i := 0
	depth := 0
	for i < len(s) {
		// skip optional leading dot for depths that use one
		if s[i] == '.' {
			i++
		}
		start := i
		switch depth % 4 {
		case 0, 1:
			for i < len(s) && s[i] >= '0' && s[i] <= '9' {
				i++
			}
		case 2:
			for i < len(s) && s[i] >= 'a' && s[i] <= 'z' {
				// Stop if next char is a dot (boundary)
				i++
			}
		case 3:
			for i < len(s) && (s[i] == 'i' || s[i] == 'v' || s[i] == 'x' || s[i] == 'l' || s[i] == 'c' || s[i] == 'd' || s[i] == 'm') {
				i++
			}
		}
		if start == i {
			return nil
		}
		segs = append(segs, s[start:i])
		depth++
	}
	return segs
}

func indexForSegment(depth int, seg string) int {
	switch depth % 4 {
	case 0, 1:
		n := 0
		for _, c := range seg {
			if c < '0' || c > '9' {
				return -1
			}
			n = n*10 + int(c-'0')
		}
		return n - 1
	case 2:
		return alphaToIndex(seg)
	case 3:
		return romanToInt(seg)
	}
	return -1
}
