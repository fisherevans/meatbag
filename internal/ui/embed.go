// Package ui embeds the built Vite UI for serving from the daemon. The dist
// directory is populated by `make ui`; if it's missing (e.g. you ran `go build`
// directly), FS returns an empty filesystem and the daemon serves a fallback.
package ui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// FS returns the rooted built UI filesystem (so paths like "index.html" work),
// or nil if the dist directory hasn't been built.
func FS() fs.FS {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return nil
	}
	// Check whether the embedded subtree actually has files.
	entries, err := fs.ReadDir(sub, ".")
	if err != nil || len(entries) == 0 {
		return nil
	}
	return sub
}
