# Data model

## List YAML

```yaml
id: 01HQXXXXXXXXXXXXXXXXXXXXXX     # ULID, immutable
slug: onboard-aws                  # in URL; unique across active + archive
title: AWS Onboarding
description: |
  Markdown description (rendered server-side to safe HTML).
project_path: /Users/me/code/foo   # auto-set from CWD on create
status: active                     # active | archived
created_at: 2026-05-16T18:00:00Z
updated_at: 2026-05-16T18:30:00Z
items:
  - id: it_abcd1234
    title: Sign up for AWS account
    owner: human                   # human | agent
    state: todo                    # todo | in_progress | blocked | done | skipped
    content: |
      Markdown body; can include code blocks, tables, links, etc.
    inputs:
      - name: account_id
        type: text
        label: AWS Account ID
        required: true
    input_values:
      account_id:
        value: "123456789012"
        has_value: true
    note: ""
    created_at: ...
    updated_at: ...
    children:
      - id: it_efgh5678
        title: Enable MFA
        owner: human
        state: todo
        children: []
```

## Numeric labels

Labels like `1`, `1.1`, `1.1a`, `1.1a.i` are **derived from tree position**, not
stored. The scheme alternates by depth:

| depth | shape    | example  |
|-------|----------|----------|
| 0     | `N`      | `1`, `2` |
| 1     | `.N`     | `.1`, `.2` |
| 2     | `letter` | `a`, `b`, ..., `aa` |
| 3     | `.roman` | `.i`, `.ii`, `.iv` |
| 4+    | repeats from depth 0 |

Item IDs (`it_abcd1234`) are stable, so URLs / saved references survive
reordering even though labels change.

## Input field types

| type          | UI control            | storage         |
|---------------|-----------------------|-----------------|
| text          | single-line input     | inline `value`  |
| textarea      | multi-line input      | inline `value`  |
| password      | hidden input          | macOS Keychain via `secret_ref` |
| number        | number input          | inline `value` (float64) |
| url           | url input             | inline `value`  |
| select        | dropdown              | inline `value`  |
| multiselect   | checkbox group        | inline `value` (array) |
| radio         | dropdown (same as select) | inline `value` |
| checkbox      | toggle                | inline `value` (bool) |
| file          | file upload           | content-addressed blob via `blob_ref` |
| markdown      | textarea              | inline `value`  |

## External resources

- **Secrets** in Keychain. Service: `meatbag`. Account:
  `<list-id>:<item-id>:<field>`. The label is `meatbag: <field> (<item-id>)`.
- **Blobs** at `~/.meatbag/blobs/<sha256>`. Hex-encoded sha256 of the
  payload. Dedupe is automatic.

## State and projects

- `state.json` at the data root stores the current daemon port and PID.
  Written by the daemon at start, removed on clean exit.
- `project_path` on a list is the CWD where it was created. The UI groups by
  this; the CLI `--project .` flag filters by it.
