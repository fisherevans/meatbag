package tree

import (
	"testing"

	"github.com/fisherevans/meatbag/internal/store"
)

func mkItem(id string, children ...*store.Item) *store.Item {
	return &store.Item{ID: id, Title: id, State: store.StateTodo, Owner: store.OwnerHuman, Children: children}
}

func TestLabelForIndex(t *testing.T) {
	tests := []struct {
		depth, idx int
		want       string
	}{
		{0, 0, "1"},
		{0, 9, "10"},
		{1, 0, ".1"},
		{1, 4, ".5"},
		{2, 0, "a"},
		{2, 25, "z"},
		{2, 26, "aa"},
		{2, 27, "ab"},
		{3, 0, ".i"},
		{3, 3, ".iv"},
		{3, 8, ".ix"},
	}
	for _, tt := range tests {
		if got := LabelForIndex(tt.depth, tt.idx); got != tt.want {
			t.Errorf("LabelForIndex(%d,%d) = %q, want %q", tt.depth, tt.idx, got, tt.want)
		}
	}
}

func TestLabels(t *testing.T) {
	list := &store.List{Items: []*store.Item{
		mkItem("A"),
		mkItem("B",
			mkItem("B1"),
			mkItem("B2",
				mkItem("B2a"),
				mkItem("B2b",
					mkItem("B2bi"),
				),
			),
		),
		mkItem("C"),
	}}
	labels := Labels(list)
	cases := map[string]string{
		"A": "1", "B": "2", "C": "3",
		"B1": "2.1", "B2": "2.2",
		"B2a": "2.2a", "B2b": "2.2b",
		"B2bi": "2.2b.i",
	}
	for id, want := range cases {
		if labels[id] != want {
			t.Errorf("label %s = %q, want %q", id, labels[id], want)
		}
	}
}

func TestFindByLabel(t *testing.T) {
	list := &store.List{Items: []*store.Item{
		mkItem("A"),
		mkItem("B",
			mkItem("B1"),
			mkItem("B2",
				mkItem("B2a"),
			),
		),
	}}
	for _, c := range []struct {
		label, wantID string
	}{
		{"1", "A"},
		{"2", "B"},
		{"2.1", "B1"},
		{"2.2", "B2"},
		{"2.2a", "B2a"},
	} {
		it, _, _, ok := FindByLabel(list, c.label)
		if !ok || it.ID != c.wantID {
			t.Errorf("FindByLabel(%q) = %v ok=%v want %s", c.label, it, ok, c.wantID)
		}
	}
	if _, _, _, ok := FindByLabel(list, "9.9"); ok {
		t.Error("expected miss for 9.9")
	}
}

func TestMoveBasic(t *testing.T) {
	list := &store.List{Items: []*store.Item{
		mkItem("A"),
		mkItem("B"),
		mkItem("C"),
	}}
	// Move A after C -> [B, C, A]
	if err := Move(list, "A", MoveOptions{AfterID: "C"}); err != nil {
		t.Fatal(err)
	}
	ids := rootIDs(list)
	if got := stringSlice(ids); got != "B,C,A" {
		t.Fatalf("after move: %s", got)
	}
}

func TestMoveUnderParent(t *testing.T) {
	list := &store.List{Items: []*store.Item{
		mkItem("A"),
		mkItem("B"),
		mkItem("C"),
	}}
	if err := Move(list, "A", MoveOptions{ParentID: "B"}); err != nil {
		t.Fatal(err)
	}
	if stringSlice(rootIDs(list)) != "B,C" {
		t.Fatalf("root: %v", rootIDs(list))
	}
	b, _, _, _ := FindByID(list, "B")
	if len(b.Children) != 1 || b.Children[0].ID != "A" {
		t.Fatalf("B children: %v", b.Children)
	}
}

func TestMoveCycle(t *testing.T) {
	list := &store.List{Items: []*store.Item{
		mkItem("A",
			mkItem("A1"),
		),
	}}
	if err := Move(list, "A", MoveOptions{ParentID: "A1"}); err != ErrCycle {
		t.Fatalf("expected ErrCycle, got %v", err)
	}
}

func TestInsertAt(t *testing.T) {
	list := &store.List{Items: []*store.Item{
		mkItem("A"),
		mkItem("C"),
	}}
	if err := InsertAt(list, mkItem("B"), MoveOptions{BeforeID: "C"}); err != nil {
		t.Fatal(err)
	}
	if stringSlice(rootIDs(list)) != "A,B,C" {
		t.Fatalf("got %v", rootIDs(list))
	}
}

func TestRemove(t *testing.T) {
	list := &store.List{Items: []*store.Item{
		mkItem("A"),
		mkItem("B"),
	}}
	it, err := Remove(list, "A")
	if err != nil || it.ID != "A" {
		t.Fatalf("remove: %v %v", err, it)
	}
	if stringSlice(rootIDs(list)) != "B" {
		t.Fatalf("got %v", rootIDs(list))
	}
}

func rootIDs(list *store.List) []string {
	out := make([]string, 0, len(list.Items))
	for _, it := range list.Items {
		out = append(out, it.ID)
	}
	return out
}

func stringSlice(s []string) string {
	out := ""
	for i, x := range s {
		if i > 0 {
			out += ","
		}
		out += x
	}
	return out
}
