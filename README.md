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

## Telling your agent about meatbag

LLM agents won't know meatbag is available unless you tell them. The easiest
way is to paste a small markdown snippet into the agent config file your tool
already reads (CLAUDE.md, AGENTS.md, `.cursorrules`, etc.) so the agent picks
it up automatically each session.

Print the snippet:

```
meatbag agent snippet
```

Or append it directly to a project-level config file:

```
meatbag agent snippet >> CLAUDE.md
```

With `--json` the snippet is wrapped as `{"snippet": "..."}` so it can be
post-processed:

```
meatbag --json agent snippet | jq -r .snippet >> ~/.config/AGENTS.md
```

The snippet is intentionally short. It points the agent at `meatbag agent help`
for the full usage guide once it decides meatbag is the right tool for the job.

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
