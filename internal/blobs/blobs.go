// Package blobs is a content-addressed file store under ~/.meatbag/blobs/.
// Uploaded inputs of type "file" land here keyed by sha256(content). The
// blob_ref string persisted in list YAML is "blobs/<hex-sha256>".
package blobs

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const RefPrefix = "blobs/"

// Store wraps the blobs directory.
type Store struct {
	Dir string
}

func New(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Store{Dir: dir}, nil
}

// BuildRef returns the persisted reference for a sha.
func BuildRef(sha string) string { return RefPrefix + sha }

// ParseRef returns the sha portion of a blob_ref.
func ParseRef(ref string) (string, error) {
	rest, ok := strings.CutPrefix(ref, RefPrefix)
	if !ok {
		return "", fmt.Errorf("not a blob ref: %q", ref)
	}
	if len(rest) != 64 {
		return "", fmt.Errorf("blob ref sha length != 64: %q", ref)
	}
	return rest, nil
}

// Path returns the on-disk path for a sha.
func (s *Store) Path(sha string) string { return filepath.Join(s.Dir, sha) }

// Write streams r to a temp file, then renames to the sha-named final path.
// Returns sha, bytes-written. If the blob already exists, the temp is removed
// and the existing path is reused.
func (s *Store) Write(r io.Reader) (sha string, size int64, err error) {
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return "", 0, err
	}
	tmp, err := os.CreateTemp(s.Dir, ".blob-*")
	if err != nil {
		return "", 0, err
	}
	tmpPath := tmp.Name()
	defer func() {
		// Best-effort cleanup if we never renamed it.
		os.Remove(tmpPath)
	}()
	h := sha256.New()
	mw := io.MultiWriter(tmp, h)
	n, err := io.Copy(mw, r)
	if err != nil {
		tmp.Close()
		return "", 0, err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return "", 0, err
	}
	if err := tmp.Close(); err != nil {
		return "", 0, err
	}
	sha = hex.EncodeToString(h.Sum(nil))
	final := s.Path(sha)
	if _, err := os.Stat(final); err == nil {
		// Already present; dedupe wins.
		return sha, n, nil
	}
	if err := os.Rename(tmpPath, final); err != nil {
		return "", 0, err
	}
	return sha, n, nil
}

// Read returns a reader for the blob. Caller must Close.
func (s *Store) Read(sha string) (io.ReadCloser, error) {
	return os.Open(s.Path(sha))
}

// Delete removes a blob. Missing blobs return nil.
func (s *Store) Delete(sha string) error {
	err := os.Remove(s.Path(sha))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// ListAll enumerates the shas of every blob present on disk.
func (s *Store) ListAll() ([]string, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") || len(name) != 64 {
			continue
		}
		out = append(out, name)
	}
	return out, nil
}

// Size returns the on-disk size of a blob.
func (s *Store) Size(sha string) (int64, error) {
	fi, err := os.Stat(s.Path(sha))
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}
