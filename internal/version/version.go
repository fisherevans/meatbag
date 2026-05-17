// Package version exposes the build-time version metadata. Values are
// overridden via -ldflags at build time; defaults below apply to `go run` /
// `go install` without explicit -ldflags so the binary still works.
package version

import "strings"

var (
	Version = "dev"
	Commit  = ""
	Date    = ""
)

// String returns a single-line summary like
// "v0.1.0 (commit 72067e7, built 2026-05-17)". Falls back gracefully when
// Commit/Date are empty.
func String() string {
	s := Version
	extras := []string{}
	if Commit != "" {
		extras = append(extras, "commit "+Commit)
	}
	if Date != "" {
		extras = append(extras, "built "+Date)
	}
	if len(extras) > 0 {
		s += " (" + strings.Join(extras, ", ") + ")"
	}
	return s
}
