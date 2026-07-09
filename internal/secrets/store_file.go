//go:build !darwin

// Non-macOS backend: one 0600 file per secret under <home>/secrets/, where
// <home> is $MEATBAG_HOME or ~/.meatbag (the same root as list YAML and blobs).
//
// Headless Linux boxes have no login keychain and, on a bare LXC, no D-Bus
// Secret Service daemon either, so a file store is the portable equivalent. On
// a single-tenant box file permissions are the boundary - the same posture the
// rest of meatbag's on-disk state (YAML, blobs) already relies on.
//
// The filename is base64url(account) so ListAll can recover the
// list/item/field triple; the file content is the raw secret value.
package secrets

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// home mirrors store.DefaultHome (duplicated to avoid an import cycle: store
// depends on secrets, not the other way around).
func home() string {
	if h := os.Getenv("MEATBAG_HOME"); h != "" {
		return h
	}
	uh, err := os.UserHomeDir()
	if err != nil {
		return ".meatbag"
	}
	return filepath.Join(uh, ".meatbag")
}

func secretsDir() string {
	return filepath.Join(home(), "secrets")
}

func fileFor(ref Ref) string {
	name := base64.RawURLEncoding.EncodeToString([]byte(ref.Account()))
	return filepath.Join(secretsDir(), name)
}

// Set writes (or replaces) a secret file atomically with 0600 perms. Returns
// the canonical secret_ref.
func Set(listID, itemID, field, value string) (string, error) {
	ref := Ref{ListID: listID, ItemID: itemID, Field: field}
	dir := secretsDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(dir, ".meatbag-secret-*")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after a successful rename
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return "", err
	}
	if _, err := tmp.WriteString(value); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tmpName, fileFor(ref)); err != nil {
		return "", err
	}
	return ref.String(), nil
}

// Get fetches the secret value.
func Get(ref Ref) (string, error) {
	b, err := os.ReadFile(fileFor(ref))
	if errors.Is(err, os.ErrNotExist) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Has reports whether a secret exists without reading its value.
func Has(ref Ref) (bool, error) {
	_, err := os.Stat(fileFor(ref))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// Delete removes the secret. Returns nil if it was already gone.
func Delete(ref Ref) error {
	err := os.Remove(fileFor(ref))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// ListAll enumerates every stored secret by decoding each filename back into an
// account triple. Used by `meatbag gc`.
func ListAll() ([]Ref, error) {
	entries, err := os.ReadDir(secretsDir())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]Ref, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".meatbag-secret-") {
			continue // skip stray temp files from an interrupted Set
		}
		acctBytes, derr := base64.RawURLEncoding.DecodeString(e.Name())
		if derr != nil {
			continue
		}
		parts := strings.SplitN(string(acctBytes), ":", 3)
		if len(parts) != 3 {
			continue
		}
		out = append(out, Ref{ListID: parts[0], ItemID: parts[1], Field: parts[2]})
	}
	return out, nil
}
