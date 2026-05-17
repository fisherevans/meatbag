# Stretch goals

Things deferred from v1 that are worth doing later but didn't make the cut.

## Mermaid diagrams in markdown

Render fenced code blocks with `mermaid` info-strings as interactive Mermaid
diagrams instead of plain code blocks. Two viable approaches:

- **Client-side**: ship `mermaid` as an npm dep, post-process `pre code.language-mermaid`
  in the UI and call `mermaid.run()` after each render. Adds ~600KB gzipped to
  the bundle; only loads when a diagram is on the page if we lazy-import.
- **Server-side**: pre-render to inline SVG via a Go wrapper or the
  [mermaid-cli](https://github.com/mermaid-js/mermaid-cli) Node binary. Keeps
  the UI bundle slim but introduces a build-time dep.

Lean toward client-side with dynamic `import()` so users who never have a
diagram in a list pay nothing.

## Other deferred items

- Audit log per list (append-only `.log` sibling next to each YAML)
- Linux + Windows secret backends (`libsecret`, `wincred`) behind a build-tag interface
- Multi-machine sync (likely just "use Syncthing on `~/.meatbag/`" with a note)
- Auth on the web UI for shared machines
- Collaborative editing (CRDT or operational transform for `input_values`)
- `meatbag agent install` to ship a Claude Code skill with usage examples
- Image uploads through `file` inputs as inline previews in the UI
