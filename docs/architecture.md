# Architecture

`meatbag` is a single Go binary that contains both a CLI and a small web
daemon. The two communicate only through the filesystem - they never need to
talk to each other directly.

```
        agent --- meatbag list create / item add / state ...
                          |
                          v
   ~/.meatbag/lists/<slug>-<ulid>.yaml  <-- flock-guarded
                          ^
                          |
                  fsnotify (per file)
                          |
                          v
   meatbag web daemon ----+--- broadcasts SSE to UI tabs
                          |
                          v
                React + Vite UI (embedded)
                          |
                          v
                       human :)
```

## Data plane

- Lists are one YAML file per list under `~/.meatbag/lists/`.
- Archived lists move to `~/.meatbag/archive/`.
- Secrets (`password`-type inputs) live in the macOS Keychain under service
  `meatbag`, account `<list-id>:<item-id>:<field>`. The YAML stores only a
  `secret_ref`.
- File uploads live in `~/.meatbag/blobs/<sha256>`. The YAML stores a
  `blob_ref`. Content-addressing gives free dedupe across lists.

## Concurrency

Every write to a list YAML is wrapped in a `flock` on that file. Both the CLI
and the daemon take the same lock, so cross-process races are bounded. Writes
use `tempfile + fsync + rename` for crash safety.

## Daemon

- Listens on `127.0.0.1:7421` by default. Stores PID in `~/.meatbag/daemon.pid`
  and the live port in `~/.meatbag/state.json`.
- `fsnotify` watches `lists/` and `archive/`. File events are debounced ~150ms
  and dispatched to a tiny pub/sub broker.
- Browser tabs subscribe over SSE at `/api/events`. The daemon emits
  `{type: "list_updated" | "list_deleted", slug: "<slug>"}` so the UI knows
  which list to re-fetch.
- The UI is embedded via `//go:embed`. The daemon serves a SPA fallback so
  client-side routes work on hard refresh.

## CLI

Built on cobra. Every mutating command takes the list's flock for the duration
of its read-modify-write. The CLI does not depend on the daemon being up -
agents can do work in headless contexts and the human picks it up later in the
UI.

A global `--json` flag switches every command to machine-readable output for
agent consumption.

## Cleanup

Delete operations (`list delete`, `item delete`, `input clear`) walk the
deleted subtree and remove any referenced Keychain entries and blobs before
unlinking the YAML node. `meatbag gc` is the reconciliation pass: it scans
every list (active + archived), collects referenced refs, then deletes any
Keychain entry under service `meatbag` or any blob in `~/.meatbag/blobs/` that
isn't referenced.
