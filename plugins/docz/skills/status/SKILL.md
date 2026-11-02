---
name: docz:status
description: >
  Set a docz document's status with `docz status set`, validated against the
  type's configured lifecycle.
when_to_use: >
  "mark ADR-0003 accepted", "set RFC-0002 to Proposed", "this design is
  implemented now", "supersede ADR-0005", "close out the investigation",
  "change the status of", "move the IMPL to completed".
allowed-tools: Bash(docz *)
argument-hint: "<type> <id> <status>"
---

# Docz Status

Set a document's status with `docz status set <type> <id> <new-status>`.
This is the canonical mutator — never hand-edit the `status:` frontmatter key.

Requires the `docz` CLI ≥ v1.2.0 on `PATH` (`status set` landed in v1.1.1).

## Process

1. **Resolve the type and id** from the request. The type accepts a canonical
   name, an alias, or an `id_prefix` (`adr`, `investigation`, `inv`, `impl`,
   `implementation`, or a custom type). The id is the frontmatter `id`, matched
   **case-sensitively** — `ADR-0003`, never `adr-0003`.

2. **Confirm the target status** if the request is vague ("close this out").
   Run `docz config` and read the type's `statuses` list. Status values are
   **case-sensitive** too: `Accepted`, not `accepted`.

3. **Preview** when the change is non-obvious or the user asked what would
   happen:
   ```bash
   docz status set adr ADR-0003 Accepted --dry-run
   # [dry-run] docs/adr/0003-….md: status Proposed -> Accepted
   ```

4. **Apply it**:
   ```bash
   docz status set adr ADR-0003 Accepted
   # docs/adr/0003-….md: status Proposed -> Accepted
   ```
   Only the status value bytes change — key order, quoting, and trailing
   comments in the frontmatter are preserved.

5. **Always regenerate the index** — `docz status set` does **not** touch the
   README table, so the index still shows the old status until you run:
   ```bash
   docz update adr
   ```
   Skipping this is the most common mistake with this command.

6. **Report** the file path, the old status, and the new one.

## Exit Codes

Branch on these in scripts; `--quiet` suppresses stdout but leaves them unchanged.

| Code | Meaning |
|------|---------|
| `0` | Applied, no-op (`already at X`), or `--dry-run` |
| `1` | Id not found in the type's directory, or the write failed |
| `2` | Unknown type, invalid status, or unsupported line endings |

A wrong-case id returns `1` (it simply is not found); a wrong-case status
returns `2` with the valid list printed. Both are easy to misdiagnose.

Machine-readable output:

```bash
docz status set adr ADR-0003 Accepted --format json
# {"path":"docs/adr/0003-….md","from":"Proposed","to":"Accepted","dry_run":false,"changed":true}
```

## Superseding a Document

Two steps — the command owns the status, you own the prose:

```bash
docz status set adr ADR-0005 Superseded
docz update adr
```

Then edit the **new** document's body to add a `Supersedes ADR-0005.` line,
and optionally link forward from the old one. `docz status set` only writes
the status field; body edits are ordinary file edits.

## Disabled and Missing Types

- A disabled type (PLAN is off by default since docz v1.1.1) has no documents
  to find, so `status set` returns `1` with `no plan document with id … found
  in docs/plan`. Check `docz config` before assuming the id is wrong.
- An unrecognized type word may be a **custom type** defined in `.docz.yaml`.
  Run `docz config` and check `types:` before treating it as a typo.

## Reference

Read `../docz/references/workflow.md` for the full lifecycle flows — proposal,
decision, review, superseding, and archiving.
