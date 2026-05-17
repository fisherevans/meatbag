## meatbag

`meatbag` is installed on this machine. It is a local CLI + web UI for shared
to-do lists between you (the agent) and the human. Use it when a request needs
back-and-forth that won't fit cleanly in chat:

- Multi-step workflows where you'd otherwise lose state across turns.
- Human-in-the-loop steps: fetching credentials, signing in, generating API
  keys, uploading files.
- Work you're doing in parallel that the human should be able to watch.

Don't use it for one-shot questions. Just ask.

Quick shape: create a list, add items (each owned by `human` or `agent`),
optionally attach an `inputs` schema so the human fills structured fields in
the web UI. Mark items `in_progress`, `blocked`, or `done` as you go. Share
the list with the human by printing `meatbag url <slug>`.

Always pass `--json` so output is parseable:

```
meatbag --json list create --title "..." --project .
meatbag --json item add <slug> --title "..." --owner human --inputs inputs.yaml
meatbag --json input get <slug> <item> <field> [--reveal]
meatbag --json url <slug>
```

For the full usage guide (list/item/input/url commands, input types, etiquette,
nesting rules), run:

```
meatbag agent help
```
