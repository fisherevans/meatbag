package daemon

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/fisherevans/meatbag/internal/store"
)

// watchLists fires Event{"list_updated", <slug>} (or "list_deleted") on the
// broker whenever a file under lists/ or archive/ changes. It debounces
// events so rapid renames or save-temp-then-rename sequences collapse.
func watchLists(ctx context.Context, s *store.Store, b *broker) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer w.Close()

	for _, d := range []string{filepath.Join(s.Home, "lists"), filepath.Join(s.Home, "archive")} {
		if err := w.Add(d); err != nil {
			return err
		}
	}

	// Debounce: per-path timer that fires the broker event N ms after the
	// last fs event for that path.
	const debounce = 150 * time.Millisecond
	timers := map[string]*time.Timer{}
	publish := func(path string, deleted bool) {
		slug := slugFromPath(path)
		if slug == "" {
			return
		}
		ev := Event{Type: "list_updated", Slug: slug}
		if deleted {
			ev.Type = "list_deleted"
		}
		b.publish(ev)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-w.Events:
			if !ok {
				return nil
			}
			if !strings.HasSuffix(ev.Name, ".yaml") {
				continue
			}
			// Skip the atomic-write temp files; they don't end in .yaml because
			// we pattern "*.meatbag-*.yaml" - actually CreateTemp does end in
			// .yaml. Filter out hidden temp names.
			base := filepath.Base(ev.Name)
			if strings.HasPrefix(base, ".meatbag-") {
				continue
			}
			deleted := ev.Op&fsnotify.Remove != 0 || ev.Op&fsnotify.Rename != 0
			if t, ok := timers[ev.Name]; ok {
				t.Stop()
			}
			path := ev.Name
			timers[ev.Name] = time.AfterFunc(debounce, func() { publish(path, deleted) })
		case err, ok := <-w.Errors:
			if !ok {
				return nil
			}
			_ = err
		}
	}
}

// slugFromPath extracts "<slug>" from ".../lists/<slug>-<ulid>.yaml".
func slugFromPath(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), ".yaml")
	cut := strings.LastIndex(base, "-")
	if cut < 0 {
		return ""
	}
	return base[:cut]
}
