# meatbag agent guide

You are an LLM agent and `meatbag` is your way to give a human a structured
checklist that you can both update. Lists live as YAML files under
`~/.meatbag/`. The web UI at `http://127.0.0.1:7421` is what the human looks
at; the CLI is what you drive.

This guide assumes you'll always pass `--json` so output is parseable. Examples
below omit it for readability.

## When to use meatbag

Use it whenever a multi-step workflow needs back-and-forth with the human:

- You need them to fetch credentials, sign in somewhere, generate API keys, or
  upload files.
- You're going to do work in parallel (provisioning, running migrations) and
  you want them to see what you're doing.
- A request can't be completed in one turn and you'd otherwise lose state when
  chat scrollback gets long.

Don't use it for one-shot questions; just ask.

## The shape of a list

A list is a tree of items. Each item has:

- A **title** (one line).
- A **state**: `todo`, `in_progress`, `blocked`, `done`, `skipped`.
- An **owner**: `human` (the human is expected to do this) or `agent` (you are
  doing this; it's visible to the human as context).
- Optional **markdown content** (instructions, links, code blocks, tables).
- Optional **inputs** schema (text, password, file, etc.) that the human fills
  in via the UI.
- Optional children, which nest arbitrarily. Labels are derived from position:
  `1`, `1.1`, `1.1a`, `1.1a.i`.

## Creating a list

```
meatbag list create --title "Set up Postgres replica" --project .
```

`--project .` tags the list with the current working directory so it groups
nicely on the UI home page. The slug is derived from the title; pass `--slug`
to override.

Then tell the human:

```
meatbag url set-up-postgres-replica
```

Paste that URL into chat. The human clicks it.

## Adding items

```
meatbag item add set-up-postgres-replica \
  --title "Provision the replica VM" --owner agent

meatbag item add set-up-postgres-replica \
  --title "Get the read-replica password from 1Password" --owner human \
  --content @notes.md \
  --inputs @inputs.yaml
```

`inputs.yaml`:

```yaml
- name: replica_password
  type: password
  label: Read-replica password
  required: true
- name: cidr
  type: text
  label: Allowed source CIDR
  required: true
- name: cert
  type: file
  label: TLS client cert
  accept: [".pem"]
```

Input types: `text`, `textarea`, `password`, `number`, `url`, `select`,
`multiselect`, `radio`, `checkbox`, `file`, `markdown`.

Guidelines:

- Ask for the minimum needed.
- Use `password` for secrets - it goes to the macOS Keychain, not the YAML.
- Use `file` for uploads - they go to `~/.meatbag/blobs/`, content-addressed.
- Use `select` with `options` when answers are bounded; reduces typos.
- Use `required: true` so the UI nags the human.

## Nesting

```
meatbag item add set-up-postgres-replica --title "DB checks" --owner agent
meatbag item add set-up-postgres-replica --title "Verify lag" \
  --parent 2 --owner agent
```

You can reference items by their derived label (`2`, `2.1a`) or by their stable
ID (`it_abcd1234`). Stable IDs survive reordering; labels don't.

## Updating state

```
meatbag item state set-up-postgres-replica 2 in_progress
meatbag item state set-up-postgres-replica 2.1 blocked --note "waiting on CIDR"
meatbag item state set-up-postgres-replica 2 done
```

Use `blocked` + `--note` when you're waiting on the human (especially for
inputs they haven't filled yet). Use `in_progress` on your own items so the
human can see you're working.

## Polling for human-supplied input

```
meatbag input get set-up-postgres-replica 1 replica_password
```

By default, the value is redacted (just `has_value: true` or `false`). Pass
`--reveal` when you actually need to use the secret. Do that as late as
possible - print to logs or commands, not back into chat.

For non-secret values, just read them:

```
meatbag --json input get set-up-postgres-replica 1 cidr
```

## Waiting for the human

Polling `meatbag input get` in a loop works but burns tokens. For long-running
agents (especially Claude Code bash monitors that watch for completion), use
`meatbag wait` instead. It blocks on the list file via fsnotify and wakes up
the moment the human changes anything relevant.

```
meatbag wait set-up-postgres-replica 2 --state=done
meatbag wait set-up-postgres-replica 1 --input=replica_password --timeout=30m
meatbag wait set-up-postgres-replica 2 --state=done,skipped --input=cert
```

- `--state=<s1,s2,...>`: exit 0 when the item enters any listed state.
- `--input=<field>`: repeatable. All listed inputs must have a value.
- `--timeout=<dur>`: Go duration (`30m`, `1h`, `5s`). Default `1h`. `0` blocks
  forever.

Exit codes follow `timeout(1)` conventions:

- `0` - conditions satisfied.
- `124` - timed out before satisfied.
- `2` - the item or list disappeared while waiting.
- `130` - interrupted (SIGINT).

State and input conditions AND together; multiple states inside `--state`
OR. It's cheap to leave running for an hour - fsnotify, not polling - so
prefer this over `sleep N && meatbag input get ...` loops.

## Marking work complete and tidying up

- Mark the human's items `done` when they confirm.
- Mark your own items `done` as you finish them.
- Archive the list when the workflow is over: `meatbag list archive <slug>`.
- Only delete with the human's explicit say-so: `meatbag list delete <slug>
  --yes`. Delete **purges** keychain entries and blob files for the list.

## Etiquette

- Don't spawn ten micro-items where two would do; the human is reading them.
- Don't delete items the human has already worked on without telling them.
- Don't reveal secrets you don't need to. Read them just-in-time.
- When you stall, leave a `--note` on the relevant item and mark it `blocked`.
  Don't go quiet.
- When you start something the human didn't ask for, add it as an agent-owned
  item first so they see it.

## Reference (selected)

```
meatbag list create --title "..." [--slug X] [--project .] [--description @file]
meatbag list ls [--project .] [--status active|archived|all]
meatbag list show <list>
meatbag list archive <list>
meatbag list delete <list> --yes

meatbag item add <list> --title "..." [--owner human|agent]
    [--parent <ref>] [--after <ref>] [--before <ref>]
    [--content @file] [--inputs @schema.yaml]
meatbag item show <list> <item>
meatbag item state <list> <item> <state> [--note "..."]
meatbag item update <list> <item> [--title ...] [--content @file]
meatbag item move <list> <item> [--parent <ref>] [--after <ref>] [--before <ref>]
meatbag item delete <list> <item> --yes

meatbag input get <list> <item> <field> [--reveal]
meatbag input set <list> <item> <field> (--value "..." | --file <path> | --stdin)
meatbag input clear <list> <item> <field>

meatbag wait <list> <item> [--state s1,s2] [--input field ...] [--timeout 1h]

meatbag url <list> [<item>] [--field <name>]
meatbag web start | stop | status | logs | restart
meatbag gc [--dry-run]
```

Run `meatbag help <command>` for full flag details.
