# Workflow Reference

End-to-end workflows for common docz documentation scenarios.

## Initial Repository Setup

```bash
# 1. Initialize docz (creates .docz.yaml and docs/ directory structure)
docz init

# 2. Review and customize the generated .docz.yaml
docz config   # verify resolved settings

# 3. Create the first document to test setup
docz create adr "Initial architecture decisions"

# 4. View what was created, and check it
docz list
docz validate --strict   # a fresh repository is clean
```

`docz init` creates:
- `.docz.yaml` with default settings
- `docs/` with subdirectories for each enabled type
- `docs/<type>/README.md` files with index markers

`runbook` is not scaffolded until `types.runbook.enabled: true`; run `docz init` again after enabling it.

## Proposing a Significant Change (RFC → ADR → DESIGN → IMPL)

This is the full lifecycle for large, planned changes:

```bash
# 1. Research (optional) — investigate before proposing
docz create inv "Evaluate message queue options"
# ... fill in findings ...
# Status: Open → In Progress → Concluded

# 2. Propose — write the RFC with problem + proposed solution
docz create rfc "Migrate to event-driven architecture"
# ... circulate for review, update status when ready ...
# Status: Draft → Proposed → Accepted

# 3. Record the decision — create ADR(s) for specific choices made
docz create adr "Use Kafka for event streaming"
docz create adr "Use Avro for event schemas"
# Status: Proposed → Accepted

# 4. Design — work out implementation details
docz create design "Event streaming infrastructure setup"
# ... document interfaces, data flow, error handling ...
# Status: Draft → In Review → Approved → Implemented

# 5. Implement — track phased execution
docz create impl "Phase 1: Kafka cluster provisioning"
docz create impl "Phase 2: Producer migration"
docz create impl "Phase 3: Consumer migration"
# Status: Draft → In Progress → Completed
```

## Writing and Checking a Document

Every section a template writes sits between region markers. Fill in the content between them and leave the markers alone:

```markdown
<!--docz:context:start-->
## Context

The service writes 40k events a second and the current queue drops ...
<!--docz:context:end-->
```

Then check the document before handing it off:

```bash
docz validate ADR-0004            # one document, read once
docz inspect ADR-0004             # its regions, headings, tasks, and open questions
```

A finding prints as `path:line code detail`. Fix errors; warnings are a judgement call unless CI runs with `--strict`. For a document written by hand with headings but no markers, `docz validate --fix ADR-0004` adds the markers docz can infer from the headings and reports what is left.

## Quick Decision Recording (ADR only)

For smaller decisions that don't need an RFC:

```bash
docz create adr "Use structured logging with zerolog"
# Fill in context, decision, consequences
# Status: Proposed → Accepted
```

## Updating Document Status

Use `docz status set` — it validates the new status against the type's
configured lifecycle and rewrites only the status value, leaving key order,
quoting, and comments untouched. Never hand-edit the `status:` key.

```bash
docz status set adr ADR-0003 Accepted --dry-run   # preview
docz status set adr ADR-0003 Accepted             # apply
docz update adr                                   # refresh the index table
```

The second command is not optional: `status set` changes the document's
frontmatter but **not** the README index, so the table shows the old status
until `docz update` runs.

Ids and status values are both case-sensitive (`ADR-0003` and `Accepted`).
Exit codes: `0` success, no-op, or dry-run; `1` id not found or write failed;
`2` unknown type or invalid status. `--format json` emits
`{"path":…,"from":…,"to":…,"dry_run":…,"changed":…}`; `--quiet` suppresses
stdout without changing the exit code.

## Superseding an ADR

When a decision changes, create a new ADR and mark the old one as superseded:

```bash
# Create the new ADR
docz create adr "Switch from Redis to Valkey for caching"

# Mark the old one superseded and refresh the index
docz status set adr ADR-0005 Superseded
docz update adr
```

Then edit the new ADR's body to reference the old one — `status set` owns the
status field, you own the prose:

```markdown
Supersedes ADR-0005.
```

## Bulk Operations and CI

```bash
# CI gate: schemas, content rules, stale ToCs, and README index drift
docz validate --strict
docz validate --strict --format json   # index drift is its own "index" array

# Create multiple docs in a script without updating indexes each time
docz create inv "Evaluate queue A" --no-update
docz create inv "Evaluate queue B" --no-update
docz update   # single update pass at the end

# Read a document's facts instead of parsing its markdown
docz inspect IMPL-0007 --format json | jq '.tasks | map(select(.checked | not)) | length'

# List docs in machine-readable format for scripts
docz list --format json | jq '.[] | select(.status == "Draft")'
docz list --format csv > docs-export.csv
```

## Template Customization

```bash
# Preview the default template for a type
docz template show adr

# Export and customize a template
docz template override adr
# Edits docs/templates/adr.md — now used instead of built-in

# View any template variable
# Available: .Number .Title .Slug .Filename .Date .Author .Status .Type .Prefix
```

## Exporting

The repository stays the source of truth: an export writes outward and reads nothing back.

```bash
# Standalone HTML pages in the repository's html.theme, under build/html/
docz export html                      # every enabled type, plus an index.html
docz export html DESIGN-0003 --stdout # one page to stdout
docz export html DESIGN-0003 --theme charcoal   # a one-off theme

# Confluence Cloud, into the space sync.confluence names
export ATLASSIAN_EMAIL=you@example.com ATLASSIAN_API_TOKEN=...   # never in .docz.yaml
docz export confluence --dry-run      # every read, no write
docz export confluence                # pages created or updated; a page edited in Confluence is skipped
```

A Confluence page someone edited there is skipped with a `WARNING:` line unless `--force`. Documents removed from the repository move under an Archive page on a full run; nothing is ever deleted.

## Runbooks

Runbooks ship disabled. Enable the type, scaffold its directory, then write one:

```bash
# in .docz.yaml, inside the existing types: block:  runbook: {enabled: true}
docz init
docz create runbook "Rotate the signing key"
```

Write each procedure's steps as a numbered list, a command in a fenced block under its step. After someone runs it end to end, replace the Last Verified row with the date, PR, commit, and who verified it, and run `docz validate RUNBOOK-0001`.

## Archiving and Maintenance

Each of these is `docz status set <type> <id> <status>` followed by
`docz update <type>`:

- **Completed IMPLs** — leave them as-is; they serve as historical record
- **Superseded RFCs** — set `Superseded` and link to the replacement in the body; do not delete the file
- **Superseded ADRs** — set `Superseded`; link to the new ADR
- **Inconclusive INVs** — set `Inconclusive` and note what made the question unanswerable; follow up with a new, narrower INV
- **Abandoned INVs** — set `Abandoned` with a brief note on why
- **Stale runbooks** — set `Needs Review` when the system changed under them, `Deprecated` when retired

Run `docz list --status Draft` periodically to find stale drafts that need attention.
