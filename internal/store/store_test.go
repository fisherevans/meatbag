package store

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}

func TestCreateAndFind(t *testing.T) {
	s := newTestStore(t)
	list := &List{Title: "My List", ProjectPath: "/tmp/x"}
	path, err := s.Create(list)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if list.Slug != "my-list" {
		t.Fatalf("slug: got %q", list.Slug)
	}
	if !filepath.IsAbs(path) {
		t.Fatalf("path not absolute: %s", path)
	}

	// Find by slug
	p, archived, err := s.findPath("my-list")
	if err != nil || archived || p != path {
		t.Fatalf("findPath slug: %v %v %s", err, archived, p)
	}
	// Find by ULID
	p, _, err = s.findPath(list.ID)
	if err != nil || p != path {
		t.Fatalf("findPath id: %v %s", err, p)
	}

	// Find unknown
	if _, _, err := s.findPath("nope"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestSlugCollision(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.Create(&List{Title: "Foo Bar"}); err != nil {
		t.Fatal(err)
	}
	l2 := &List{Title: "Foo Bar"}
	if _, err := s.Create(l2); err != nil {
		t.Fatal(err)
	}
	if l2.Slug != "foo-bar-2" {
		t.Fatalf("expected foo-bar-2, got %q", l2.Slug)
	}
	if _, err := s.Create(&List{Title: "Foo", Slug: "foo-bar"}); err != ErrSlugTaken {
		t.Fatalf("expected ErrSlugTaken, got %v", err)
	}
}

func TestArchiveUnarchive(t *testing.T) {
	s := newTestStore(t)
	l := &List{Title: "Box"}
	if _, err := s.Create(l); err != nil {
		t.Fatal(err)
	}
	if err := s.Archive(l.Slug); err != nil {
		t.Fatalf("Archive: %v", err)
	}
	p, archived, err := s.findPath(l.Slug)
	if err != nil || !archived {
		t.Fatalf("after archive: %v %v", err, archived)
	}
	if filepath.Dir(p) != filepath.Join(s.Home, "archive") {
		t.Fatalf("not in archive dir: %s", p)
	}
	if err := s.Unarchive(l.Slug); err != nil {
		t.Fatalf("Unarchive: %v", err)
	}
	_, archived, _ = s.findPath(l.Slug)
	if archived {
		t.Fatal("still archived")
	}
}

func TestUpdateConcurrent(t *testing.T) {
	s := newTestStore(t)
	l := &List{Title: "Counter"}
	if _, err := s.Create(l); err != nil {
		t.Fatal(err)
	}
	const N = 50
	var wg sync.WaitGroup
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := s.Update(l.Slug, func(ll *List) error {
				ll.Items = append(ll.Items, &Item{ID: NewItemID(), Title: "x", State: StateTodo, Owner: OwnerHuman})
				return nil
			})
			if err != nil {
				t.Errorf("Update: %v", err)
			}
		}()
	}
	wg.Wait()
	got, err := s.LoadList(filepath.Join(s.Home, "lists", l.Slug+"-"+l.ID+".yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != N {
		t.Fatalf("expected %d items, got %d", N, len(got.Items))
	}
}

func TestDelete(t *testing.T) {
	s := newTestStore(t)
	l := &List{Title: "Trash"}
	p, _ := s.Create(l)
	if _, err := os.Stat(p); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(l.Slug); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("expected file gone, got %v", err)
	}
}
