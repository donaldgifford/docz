---
name: docz:update
description: >
  Regenerate docz README index tables after edits.
when_to_use: >
  "update the doc indexes", "regenerate the README tables", "sync the doc
  index", "update docz", "the README tables are stale", "refresh the index".
---

# Docz Update

Regenerate README index tables with `docz update [type]`.

## Process

1. **Determine scope** from the user's request — all types or a specific one:
   ```bash
   docz update              # regenerate all indexes
   docz update <type>       # regenerate one type (rfc, adr, design, impl, inv, runbook)
   docz update --dry-run    # preview changes without writing
   ```

2. **Report what changed** — list which README files were updated and summarize the changes (rows added, updated, or unchanged).

3. **Regenerates tables of contents too** — every document with a ToC region gets it refreshed.

4. **For CI, prefer `docz validate --strict`** — it fails on README index drift and stale ToCs alongside the schema checks; `--dry-run` previews without writing.

## Common Triggers

- **After `docz status set`** — it rewrites the document's frontmatter but not
  the index table, so the README shows the old status until `docz update`
  runs. See the `docz:status` skill, which pairs the two commands.
- After directly editing document frontmatter (title changes)
- After bulk-creating documents with `--no-update`
- When README tables look out of date
- After resolving a merge conflict in a doc directory
