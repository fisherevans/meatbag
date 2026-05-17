# meatbag

A local CLI + web UI for to-do lists that LLM agents create for humans and
both sides update collaboratively.

- Agent drives `meatbag` CLI: create lists, add nested items, request
  structured input (text, files, secrets), mark items done.
- Human opens the web UI (`meatbag web start`) to work through the list,
  fill inputs, and check things off.
- State lives in YAML files under `~/.meatbag/lists/`. Secrets go in the
  macOS Keychain. Uploaded files are content-addressed under
  `~/.meatbag/blobs/`.

See:

- `docs/agent-guide.md` (also printed by `meatbag agent help`)
- `docs/architecture.md`
- `docs/data-model.md`

## Quick start

```
make build
./bin/meatbag list create --title "Set up new laptop"
./bin/meatbag web start
./bin/meatbag url set-up-new-laptop
```

## Build

```
make build       # builds UI + binary -> bin/meatbag
make ui          # ui only
make test        # go test ./...
```
