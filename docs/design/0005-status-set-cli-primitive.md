---
id: DESIGN-0005
title: "Status Set CLI Primitive"
status: Draft
author: Donald Gifford
created: 2026-05-30
---
<!-- markdownlint-disable-file MD025 MD041 -->

# DESIGN 0005: Status Set CLI Primitive

**Status:** Draft
**Author:** Donald Gifford
**Date:** 2026-05-30

<!--toc:start-->
- [Overview](#overview)
- [Goals and Non-Goals](#goals-and-non-goals)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Background](#background)
- [Detailed Design](#detailed-design)
  - [Command shape](#command-shape)
  - [Resolution algorithm](#resolution-algorithm)
  - [Frontmatter mutation](#frontmatter-mutation)
  - [Exit codes](#exit-codes)
  - [Output format](#output-format)
- [API / Interface Changes](#api--interface-changes)
- [Data Model](#data-model)
- [Testing Strategy](#testing-strategy)
- [Migration / Rollout Plan](#migration--rollout-plan)
- [Open Questions](#open-questions)
  - [1. Command shape](#1-command-shape)
  - [2. Status value case sensitivity](#2-status-value-case-sensitivity)
  - [3. ID prefix matching](#3-id-prefix-matching)
  - [4. Transition validation depth](#4-transition-validation-depth)
  - [5. Multi-document batch mode](#5-multi-document-batch-mode)
  - [6. Output format](#6-output-format)
  - [7. CRLF / Windows line endings](#7-crlf--windows-line-endings)
  - [8. Idempotency placement](#8-idempotency-placement)
  - [9. Status field name](#9-status-field-name)
  - [10. status get / status list-allowed companion verbs](#10-status-get--status-list-allowed-companion-verbs)
- [References](#references)
<!--toc:end-->

## Overview

Add a CLI primitive `docz status set <type> <id> <new-status>` that
mutates the `status:` field in a docz document's YAML frontmatter,
with lifecycle validation against `.docz.yaml`. The command is the
canonical mutator that external automation (GitHub Actions syncing
RFC PR labels to frontmatter, release tooling, scripts) calls
instead of hand-rolling YAML edits with `yq` or `sed`.

This design centers on making `.docz.yaml`'s `statuses:` list the
single source of truth for what statuses are valid, and on
preserving the exact byte shape of the rest of the frontmatter so
the mutation is a minimal, reviewable diff.

## Goals and Non-Goals

### Goals

- Programmatic, idempotent status mutation safe for use inside CI
- Lifecycle validation: reject any value not in the type's
  `.docz.yaml` `statuses:` list
- Look up documents by frontmatter `id:` (not filename), since users
  rename files
- Preserve frontmatter shape: key order, surrounding whitespace,
  trailing newline, quoting style
- Stable exit codes so callers (workflows, scripts) can branch on
  outcome without parsing stdout
- `--dry-run` and `--quiet` flags for the typical "check then apply"
  CI pattern
- Implementation reuses the existing `internal/document` scan +
  frontmatter parse helpers — no new YAML round-trip path

### Non-Goals

- Reverse direction (frontmatter → label sync). Excluded to avoid
  label↔frontmatter loops (see Issue §"Out of scope")
- Multi-document batch mode. Callers loop; one id per invocation
- Updating computed indexes (`README.md` status tables). If
  `docz update` needs to fire after mutation, the caller chains it
- Transition validation beyond list membership (e.g.
  `Draft → Accepted` requires going through `Proposed`). See Open
  Question 4
- Writing or reading any field other than `status:` in the
  frontmatter

## Background

The triggering use case is a GitHub Action in the **rfc-api** repo
that reflects PR label changes (`status/draft`, `status/proposed`,
`status/accepted`, …) into the corresponding docz document's
frontmatter. The action runs on every label event and needs a
programmatic mutator that:

1. Doesn't require the workflow to know about `.docz.yaml`
2. Rejects typos against the actual allowed-statuses list for the
   type
3. Doesn't re-flow the frontmatter (a re-flow would produce a
   diff covering every key, defeating PR review)

Today the alternatives are:

- `yq -i '.status = "Accepted"' <file>` — works, but loses docz
  lifecycle awareness, and a typo silently writes an invalid status
- `sed -i 's/^status:.*/status: Accepted/' <file>` — brittle (breaks
  on `status: "Draft"`, multi-doc frontmatter, etc.)
- A custom Go binary per consumer — duplicates the docz
  config-loading + lifecycle code that docz already owns

Making docz the canonical mutator removes the duplication and pins
the contract.

**Reference design:** rfc-api **DESIGN-0004** §"docz status
primitive" (PR <https://github.com/donaldgifford/rfc-api/pull/34>).

## Detailed Design

### Command shape

```text
docz status set <type> <id> <new-status>
  --dry-run    print the change, do not write
  --quiet      suppress success messages on stdout (exit code unchanged)
```

`status` is introduced as a parent command so future verbs
(`status get`, `status list-allowed`) compose naturally. The `set`
subcommand takes three positional args; flag wiring follows the
existing Cobra conventions in `cmd/`.

Examples:

```bash
# Successful transition (writes file, exits 0)
$ docz status set rfc RFC-0042 Accepted
docs/rfc/0042-payment-rate-limits.md: status Draft -> Accepted

# Already at target (no write, exits 0)
$ docz status set rfc RFC-0042 Accepted
docs/rfc/0042-payment-rate-limits.md: already at Accepted

# Invalid status (no write, exits 2)
$ docz status set rfc RFC-0042 Approved
Error: "Approved" is not a valid status for rfc.
Valid statuses: Draft, Proposed, Accepted, Rejected, Superseded.

# Dry run (no write, exits 0, prints the same line a real run would)
$ docz status set --dry-run rfc RFC-0042 Accepted
[dry-run] docs/rfc/0042-payment-rate-limits.md: status Draft -> Accepted
```

### Resolution algorithm

1. Validate `<type>` via `Config.ValidateType(name)` — single
   source of "unknown document type" errors. On error, exit 2 with
   the canonical "valid types" message
2. Resolve type dir via `r.inRepo(typeConfig.Dir)` so the command
   works with `--repo-root` and without `os.Chdir` (same pattern
   IMPL-0009 established for every other handler)
3. Scan documents via the existing `document.ScanDocuments(typeDir)`
   — returns `[]DocEntry` with cached frontmatter. The id lookup
   walks this slice; no new filesystem code
4. Match `<id>` against `entry.Frontmatter.ID` (Open Question 2
   covers case sensitivity, 3 covers prefix matching). On no
   match, exit 1
5. Validate `<new-status>` against `typeConfig.Statuses` — list
   membership only, no transition graph. On invalid, exit 2 and
   list the valid values
6. No-op short-circuit: if `entry.Frontmatter.Status == new-status`,
   print "already at <status>" and exit 0 without touching the file
7. Otherwise invoke `document.SetStatus(path, newStatus)` (new
   helper — see §Data Model), print the transition line, exit 0

### Frontmatter mutation

The mutation is intentionally byte-level, not a YAML round-trip:

- Read the file via `os.ReadFile`
- Locate the leading `---\n` and the closing `---\n` (or `---\r\n`
  — see Open Question 7 for Windows line endings)
- Within that range, find the line whose trimmed left side matches
  the pattern `^status:\s*(?:"([^"]*)"|'([^']*)'|(\S.*?))\s*$`,
  capturing the existing value and the surrounding quoting
- Replace only the captured value, preserving the key, the colon,
  the spacing, and the quoting style
- Write the file with `os.WriteFile` at `0o644` (matches
  `config.FileMode`)

A YAML round-trip via `yaml.Marshal` would normalize key order,
strip blank lines, and re-quote values, producing a diff far larger
than the user asked for. The byte-level approach is unconventional
but necessary for the use case.

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success: wrote the change, or no-op (already at target), or `--dry-run` printed the planned change |
| 1 | Lookup failure: id not found in the type's doc list, or file IO error |
| 2 | Validation failure: unknown type, invalid status (not in `statuses:`), ambiguous id (if Open Question 3 enables prefix matching) |

These match the conventions Issue #52 calls out (typo → exit 2,
not-found → exit 1).

### Output format

Default text shape (Open Question 6 covers JSON):

```text
<relative-path>: status <old> -> <new>
```

Prefixed with `[dry-run] ` when `--dry-run` is set. Errors go to
stderr in the existing `Error: <message>` format used by every other
command.

`--quiet` suppresses stdout on success and no-op (exit code is the
contract; the line is for humans). Errors still print to stderr
under `--quiet`.

## API / Interface Changes

New Cobra commands (`cmd/status.go`):

- `statusCmd` parent command, no `RunE`
- `statusSetCmd` subcommand with `RunE: runStatusSet` → calls
  `(*Runner).statusSet(ctx context.Context, args []string) error`
- Flags: `--dry-run bool`, `--quiet bool` (each bound to package-level
  vars per the existing pattern; the broader cmd-globals cleanup is
  deferred to a follow-up RFC per IMPL-0009 Phase 7's "blocked on
  per-command opts structs" note)

No `.docz.yaml` schema changes. `TypeConfig.Statuses` and
`TypeConfig.StatusField` already exist; this design only reads them.

No `internal/config` changes.

`internal/document` additions — see §Data Model.

`docz --help` text is auto-derived from Cobra; no manual edit
needed.

## Data Model

No schema changes. One new helper in `internal/document/status.go`:

```go
// SetStatus rewrites the `status:` field in path's YAML frontmatter
// to newStatus. Returns the old status that was replaced. Preserves
// key order, quoting style, and trailing whitespace; the only bytes
// that change are the status value itself.
//
// Returns ErrNoFrontmatter if path has no leading --- block,
// ErrStatusFieldMissing if the block exists but has no status: key.
// Errors from os.ReadFile / os.WriteFile are returned wrapped.
func SetStatus(path, newStatus string) (oldStatus string, err error)
```

Implementation notes:

- The line-finder is a tight regex (or simple line scan) that
  recognizes the three common YAML scalar shapes — bare, `"..."`,
  `'...'` — and bails on anything else (block scalars, flow
  mappings) with a clear error. The existing six templates all use
  the bare shape, so the common path is unsurprising
- The helper is independent of Cobra and `Runner`, so it gets its
  own table-driven unit tests under `internal/document/status_test.go`
- The cmd handler in `cmd/status.go` wraps it with type/id
  resolution and the runner's `Out`/`Err` writers

## Testing Strategy

`internal/document/status_test.go` (unit, `t.Parallel()`):

- Table-driven golden tests: each of the six built-in templates'
  `docz create` output mutated through `SetStatus(..., "<other>")`
  and byte-compared against a fixture. Add a `-update` flag (matches
  existing golden-file convention)
- Edge cases:
  - `status: "Draft"` double-quoted
  - `status: 'Draft'` single-quoted
  - `status:Draft` (no space) — accept; preserve the no-space form
  - Trailing comment: `status: Draft  # current`
  - Multi-line block scalar: reject with a clear "unsupported
    frontmatter shape" error
  - Missing `status:` line in an otherwise-valid block:
    `ErrStatusFieldMissing`
  - File with no frontmatter: `ErrNoFrontmatter`
- Idempotency: `SetStatus(p, "Draft")` on a doc already at `Draft`
  is a byte-perfect no-op (or — equivalent — returns the same old
  status without rewriting). See Open Question 8 — should the
  helper short-circuit or let the cmd layer decide? Recommendation
  is to keep `SetStatus` always-write and let `cmd/status.go`
  short-circuit; the unit tests cover the byte-perfect property
  separately

`cmd/status_test.go` (cmd, table-driven, constructed Runner per the
IMPL-0009 pattern with `Out: &bytes.Buffer{}` and
`RepoRoot: t.TempDir()`):

- Happy path: type=rfc, id=RFC-0001, new=Accepted → exit 0, file
  mutated, buffer matches expected line
- No-op: current == new → exit 0, file unchanged, buffer matches
  "already at" line
- Unknown type → exit 2, error mentions valid types
- Unknown id → exit 1, error mentions the type dir scanned
- Invalid status → exit 2, error lists the type's `statuses:`
- `--dry-run` → exit 0, file unchanged, buffer matches `[dry-run]`
  line
- `--quiet` happy path → exit 0, file mutated, empty buffer
- `--quiet` error path → exit 2, error still on stderr

Smoke test: against this repo, run `docz status set design
DESIGN-0005 "In Review"` and watch the diff (one line changed).

## Migration / Rollout Plan

There is no migration. The command is purely additive.

A follow-up IMPL doc will translate this design into the ordered
task list. Suggested phases there:

1. `internal/document/status.go` + golden tests
2. `cmd/status.go` parent + `set` subcommand, wired to `Runner`,
   cmd tests
3. Manual smoke + CLAUDE.md update + ship

Ship behind the existing `dont-release` PR label so the rfc-api
Action can be coordinated with the next docz release.

## Open Questions

Each option is lettered. **(a) is the recommendation**; (b+) are
alternatives kept in scope. Type `other: <freeform>` to override.

### 1. Command shape

- **(a) `docz status set <type> <id> <new-status>`** — parent +
  subcommand; leaves room for `status get` / `status list-allowed`
  without growing the top-level verb list
- (b) `docz set-status <type> <id> <new-status>` — flat verb; one
  fewer namespace level; matches `docz wiki update` flat style
- (c) `docz status <type> <id> <new-status>` — flattest; default-set
  semantics, but ambiguous for future readers
- (d) other: `<your value>`

### 2. Status value case sensitivity

- **(a) Case-sensitive** — `Accepted` must match exactly; reject
  `accepted`. Matches the convention DESIGN-0004 §F locked in for
  the `Status` typed string (canonical display values)
- (b) Case-insensitive accept, canonical write — accept `accepted`
  from CLI, but write `Accepted` to the file from the
  `statuses:` list
- (c) Case-insensitive both ways — accept any case, store whatever
  the user typed; diverges from canonical display values
- (d) other: `<your value>`

### 3. ID prefix matching

- **(a) Exact match only** (`RFC-0042` required) — predictable, easy
  to script, mirrors `docz list` which accepts full IDs only
- (b) Case-insensitive exact (`rfc-0042` matches `RFC-0042`) —
  small friction reduction; no ambiguity risk
- (c) Case-insensitive prefix (`RFC-42` matches `RFC-0042` if
  unambiguous; ambiguous case is exit 2 with a list) — power-user
  shortcut; introduces ambiguity logic
- (d) other: `<your value>`

### 4. Transition validation depth

- **(a) Membership only** — new status must be in `statuses:`; any
  → any allowed. Matches Issue #52's spec and keeps docz unaware
  of business workflow
- (b) Order-aware — `Draft → Accepted` rejected unless `Proposed`
  comes between; would require encoding allowed transitions per
  type in `.docz.yaml`; significant scope creep
- (c) Optional `--force` to bypass membership — pointless since
  membership is the only check
- (d) other: `<your value>`

### 5. Multi-document batch mode

- **(a) Out of scope** (matches issue) — one id per invocation;
  callers loop. Keeps the contract simple
- (b) Accept multiple ids — `docz status set rfc RFC-0042 RFC-0043
  Accepted`; surface ambiguity: is the last arg the status or
  another id?
- (c) Accept `--all` to set every doc of a type to one status —
  dangerous, unclear use case
- (d) other: `<your value>`

### 6. Output format

- **(a) Plain text** — `<path>: status <old> -> <new>`; human-
  readable, grep/awk-friendly. `--quiet` suppresses
- (b) JSON output behind `--json` — first-class scripting;
  `{"path":"...","from":"Draft","to":"Accepted","dry_run":false}`
- (c) Both: `--format=text|json` — covers both cases; small surface
  increase; matches `docz list` which already has `--format`
- (d) other: `<your value>`

### 7. CRLF / Windows line endings

- **(a) Reject non-LF endings** with a clear "unsupported line
  endings" error — keeps the byte mutator simple; matches docz's
  Unix-only stance per INV-0004 §Decisions 6
- (b) Detect and preserve — mutate in whatever line ending the
  file uses; ~10 lines of extra logic
- (c) Force-normalize to LF on write — fixes mixed endings but
  changes more than the user asked for; surprises CRLF users
- (d) other: `<your value>`

### 8. Idempotency placement

- **(a) Cmd-level short-circuit** — `cmd/status.go` checks current
  == new before calling `SetStatus`; `SetStatus` always writes when
  called. Keeps the helper's contract simple; cmd layer owns the
  "already at" message
- (b) Helper-level short-circuit — `SetStatus` reads, compares,
  and returns the old status without writing when current ==
  new; cmd layer just reports
- (c) Both — defensive double-check; redundant
- (d) other: `<your value>`

### 9. Status field name

- **(a) Hard-coded `status:`** — matches every built-in template
  and every existing doc; `TypeConfig.StatusField` is `"status"`
  for every default type
- (b) Read `TypeConfig.StatusField` — supports user-renamed
  fields; mostly hypothetical but cheap
- (c) Accept `--field <name>` override — flexibility for users
  who diverge from the default; broadens the CLI for an edge case
- (d) other: `<your value>`

### 10. `status get` / `status list-allowed` companion verbs

- **(a) Not in this design** — DESIGN-0005 is `set` only; leave
  `get` / `list-allowed` for a follow-up design if demand
  materializes. The `status` parent verb is created here so that
  followup is a one-file addition
- (b) Include `status get <type> <id>` in this design — easy
  scope add; reuses the same resolution algorithm
- (c) Include `status list-allowed <type>` — exposes
  `TypeConfig.Statuses` to scripts; trivial to implement
- (d) other: `<your value>`

## References

- [Issue #52](https://github.com/donaldgifford/docz/issues/52) — the
  feature request
- rfc-api **DESIGN-0004** §"docz status primitive" —
  <https://github.com/donaldgifford/rfc-api/pull/34>
- DESIGN-0001 — original docz CLI design (`docs/design/0001-docz-cli-design.md`)
- DESIGN-0004 — Runner pattern + DocType registry (this design
  builds the new `cmd/status.go` on top of `(*Runner)` method
  pattern)
- `internal/document/scan.go` — `ScanDocuments` reused for id lookup
- `internal/document/document.go` — `LoadFrontmatter` reused for
  parse
- `internal/config/doctype.go` — `TypeConfig.Statuses` is the
  lifecycle source of truth this command enforces
