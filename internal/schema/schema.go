// Package schema owns the on-disk versioning for list files. The current
// version's types are re-exported as `Latest` (which today aliases v1). When
// a breaking shape change ships, add a sibling package vN, a MigrateFromVN-1
// function on it, and re-point `Latest`. Loaders dispatch by reading the
// `schema_version` field, then chain migrations forward to Latest.
//
// External callers (CLI, daemon, tests) should never reach into a versioned
// sub-package; they go through `internal/store`, which re-exports Latest as
// `store.List`, `store.Item`, etc.
package schema

import (
	"errors"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"

	v1 "github.com/fisherevans/meatbag/internal/schema/v1"
)

// CurrentVersion is the on-disk version that new files are written at.
const CurrentVersion = v1.SchemaVersion

// ErrUnsupportedVersion is returned when a list file declares a schema
// version newer than this binary knows how to read. The user should upgrade.
var ErrUnsupportedVersion = errors.New("unsupported schema version (upgrade meatbag)")

// versionEnvelope is the minimum we need to decide which versioned type to
// unmarshal into. Files written before versioning existed have no field, so
// SchemaVersion comes back as zero - treated as v1 below.
type versionEnvelope struct {
	SchemaVersion int `yaml:"schema_version"`
}

// LoadFile reads a list file from disk and returns it as a *Latest. Older
// versions are migrated forward; future versions return ErrUnsupportedVersion.
func LoadFile(path string) (*Latest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return LoadBytes(data)
}

// LoadBytes is LoadFile for in-memory data (useful for tests and the API).
func LoadBytes(data []byte) (*Latest, error) {
	var env versionEnvelope
	if err := yaml.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("parse schema envelope: %w", err)
	}
	switch env.SchemaVersion {
	case 0, 1:
		// 0 = unstamped legacy file = identical shape to v1.
		var l v1.List
		if err := yaml.Unmarshal(data, &l); err != nil {
			return nil, fmt.Errorf("parse v1 list: %w", err)
		}
		l.SchemaVersion = CurrentVersion // normalize in memory
		return &l, nil
	default:
		return nil, fmt.Errorf("%w: file is v%d, binary supports up to v%d",
			ErrUnsupportedVersion, env.SchemaVersion, CurrentVersion)
	}
}

// Encode writes a list as YAML to w, stamping the current schema version
// first. The list value is mutated to set SchemaVersion = CurrentVersion so
// callers that re-read the in-memory struct see the canonical value.
func Encode(w io.Writer, list *Latest) error {
	list.SchemaVersion = CurrentVersion
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	if err := enc.Encode(list); err != nil {
		return err
	}
	return enc.Close()
}
