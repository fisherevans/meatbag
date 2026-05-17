package store

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"gopkg.in/yaml.v3"

	"github.com/fisherevans/meatbag/internal/schema"
)

// Store handles list YAML files under a home directory layout:
//
//	<home>/lists/<slug>-<ulid>.yaml      active
//	<home>/archive/<slug>-<ulid>.yaml    archived
//	<home>/blobs/<sha256>                file uploads
//	<home>/daemon.pid                    web daemon pid
//	<home>/daemon.log                    web daemon log
//	<home>/state.json                    daemon port + meta
type Store struct {
	Home string
}

// ErrNotFound is returned when a list lookup fails.
var ErrNotFound = errors.New("list not found")

// ErrSlugTaken is returned when an explicit slug collides.
var ErrSlugTaken = errors.New("slug already in use")

// New ensures the home layout exists and returns a Store.
func New(home string) (*Store, error) {
	if home == "" {
		home = DefaultHome()
	}
	for _, sub := range []string{"", "lists", "archive", "blobs"} {
		if err := os.MkdirAll(filepath.Join(home, sub), 0o755); err != nil {
			return nil, err
		}
	}
	return &Store{Home: home}, nil
}

// DefaultHome returns the default data root, honoring MEATBAG_HOME.
func DefaultHome() string {
	if h := os.Getenv("MEATBAG_HOME"); h != "" {
		return h
	}
	uh, err := os.UserHomeDir()
	if err != nil {
		return ".meatbag"
	}
	return filepath.Join(uh, ".meatbag")
}

func (s *Store) listsDir() string   { return filepath.Join(s.Home, "lists") }
func (s *Store) archiveDir() string { return filepath.Join(s.Home, "archive") }
func (s *Store) BlobsDir() string   { return filepath.Join(s.Home, "blobs") }
func (s *Store) StatePath() string  { return filepath.Join(s.Home, "state.json") }
func (s *Store) PidPath() string    { return filepath.Join(s.Home, "daemon.pid") }
func (s *Store) LogPath() string    { return filepath.Join(s.Home, "daemon.log") }

// listAllPaths returns every list file path. status is "active", "archived", or "all".
func (s *Store) listAllPaths(status string) ([]string, error) {
	var dirs []string
	switch status {
	case "active":
		dirs = []string{s.listsDir()}
	case "archived":
		dirs = []string{s.archiveDir()}
	case "", "all":
		dirs = []string{s.listsDir(), s.archiveDir()}
	default:
		return nil, fmt.Errorf("unknown status %q", status)
	}
	var out []string
	for _, d := range dirs {
		entries, err := os.ReadDir(d)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
				continue
			}
			out = append(out, filepath.Join(d, e.Name()))
		}
	}
	return out, nil
}

// findPath returns the path of a list matched by slug or ULID. archived is set
// when the file lives under archive/.
func (s *Store) findPath(idOrSlug string) (path string, archived bool, err error) {
	if idOrSlug == "" {
		return "", false, ErrNotFound
	}
	lower := strings.ToLower(idOrSlug)
	idMatch := IsListID(lower)
	for _, d := range []string{s.listsDir(), s.archiveDir()} {
		entries, err := os.ReadDir(d)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", false, err
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
				continue
			}
			name := strings.TrimSuffix(e.Name(), ".yaml")
			// filename is "<slug>-<ulid>"
			cut := strings.LastIndex(name, "-")
			if cut < 0 {
				continue
			}
			slug := name[:cut]
			id := name[cut+1:]
			if (idMatch && id == lower) || slug == idOrSlug {
				return filepath.Join(d, e.Name()), d == s.archiveDir(), nil
			}
		}
	}
	return "", false, ErrNotFound
}

// FindPath is exported for callers that need the resolved path.
func (s *Store) FindPath(idOrSlug string) (string, bool, error) {
	return s.findPath(idOrSlug)
}

// LoadList reads a list YAML file without locking. Older schema versions are
// transparently migrated to the latest in-memory shape; future versions
// surface schema.ErrUnsupportedVersion.
func (s *Store) LoadList(path string) (*List, error) {
	l, err := schema.LoadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", filepath.Base(path), err)
	}
	return l, nil
}

// ListAll returns every list at the given status, loaded.
func (s *Store) ListAll(status string) ([]*List, error) {
	paths, err := s.listAllPaths(status)
	if err != nil {
		return nil, err
	}
	out := make([]*List, 0, len(paths))
	for _, p := range paths {
		l, err := s.LoadList(p)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, nil
}

// existingSlugs collects all slugs across active + archived for collision checks.
func (s *Store) existingSlugs() (map[string]bool, error) {
	paths, err := s.listAllPaths("all")
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(paths))
	for _, p := range paths {
		name := strings.TrimSuffix(filepath.Base(p), ".yaml")
		cut := strings.LastIndex(name, "-")
		if cut > 0 {
			out[name[:cut]] = true
		}
	}
	return out, nil
}

// Create writes a brand new list. The list's ID/Slug/Status/CreatedAt/UpdatedAt
// are filled in if zero. If list.Slug collides with an existing list, ErrSlugTaken
// is returned (caller should choose a different slug or pass empty to auto-allocate).
func (s *Store) Create(list *List) (string, error) {
	now := time.Now().UTC()
	if list.ID == "" {
		list.ID = NewListID()
	}
	if list.Status == "" {
		list.Status = ListActive
	}
	if list.CreatedAt.IsZero() {
		list.CreatedAt = now
	}
	list.UpdatedAt = now

	existing, err := s.existingSlugs()
	if err != nil {
		return "", err
	}
	if list.Slug == "" {
		base := SlugifyTitle(list.Title)
		list.Slug = AllocateSlug(base, existing)
	} else {
		if !ValidSlug(list.Slug) {
			return "", fmt.Errorf("invalid slug %q", list.Slug)
		}
		if existing[list.Slug] {
			return "", ErrSlugTaken
		}
	}

	path := filepath.Join(s.listsDir(), list.Slug+"-"+list.ID+".yaml")
	if err := writeYAMLAtomic(path, list); err != nil {
		return "", err
	}
	return path, nil
}

// Update opens the list, takes an exclusive flock on a sidecar .lock file,
// reloads, calls fn, writes back atomically. fn may mutate the list in place;
// returning an error aborts the write. The lock is on a sidecar (not the
// YAML) because atomic-rename replaces the inode and would invalidate a lock
// taken on the YAML itself.
func (s *Store) Update(idOrSlug string, fn func(*List) error) error {
	path, _, err := s.findPath(idOrSlug)
	if err != nil {
		return err
	}
	fl, err := acquireLock(path)
	if err != nil {
		return err
	}
	defer releaseLock(fl)

	list, err := s.LoadList(path)
	if err != nil {
		return err
	}
	if err := fn(list); err != nil {
		return err
	}
	list.UpdatedAt = time.Now().UTC()
	return writeYAMLAtomic(path, list)
}

func acquireLock(path string) (*flock.Flock, error) {
	fl := flock.New(path + ".lock")
	if err := fl.Lock(); err != nil {
		return nil, fmt.Errorf("lock %s: %w", path, err)
	}
	return fl, nil
}

func releaseLock(fl *flock.Flock) {
	_ = fl.Unlock()
}

// Delete removes the list file. Caller is responsible for purging secrets/blobs
// referenced by the list before calling Delete (load the list, walk it, then
// delete external resources, then call Delete). Also removes the sidecar .lock.
func (s *Store) Delete(idOrSlug string) error {
	path, _, err := s.findPath(idOrSlug)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		return err
	}
	_ = os.Remove(path + ".lock")
	return nil
}

// Archive moves the list YAML from lists/ to archive/ and flips Status.
func (s *Store) Archive(idOrSlug string) error {
	return s.moveStatus(idOrSlug, ListArchived, s.archiveDir())
}

// Unarchive moves it back.
func (s *Store) Unarchive(idOrSlug string) error {
	return s.moveStatus(idOrSlug, ListActive, s.listsDir())
}

func (s *Store) moveStatus(idOrSlug string, status ListStatus, destDir string) error {
	srcPath, _, err := s.findPath(idOrSlug)
	if err != nil {
		return err
	}
	fl, err := acquireLock(srcPath)
	if err != nil {
		return err
	}
	defer releaseLock(fl)

	list, err := s.LoadList(srcPath)
	if err != nil {
		return err
	}
	list.Status = status
	list.UpdatedAt = time.Now().UTC()

	destPath := filepath.Join(destDir, filepath.Base(srcPath))
	if err := writeYAMLAtomic(destPath, list); err != nil {
		return err
	}
	if destPath != srcPath {
		if err := os.Remove(srcPath); err != nil {
			return err
		}
		// Move the .lock alongside so future locks are on the right path.
		_ = os.Remove(srcPath + ".lock")
	}
	return nil
}

// writeYAMLAtomic writes a list to disk via tempfile + fsync + rename. The
// schema package stamps the current schema version into the YAML.
func writeYAMLAtomic(path string, list *List) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".meatbag-*.yaml")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if err := schema.Encode(tmp, list); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

// WriteYAML serializes v as YAML to w (helper for callers like tests / CLI dump).
func WriteYAML(w io.Writer, v any) error {
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		return err
	}
	return enc.Close()
}
