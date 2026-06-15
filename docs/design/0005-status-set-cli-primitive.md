---
id: DESIGN-0005
title: "Status Set CLI Primitive"
status: Implemented
author: Donald Gifford
created: 2026-05-30
---
<!-- markdownlint-disable-file MD025 MD041 -->

# DESIGN 0005: Status Set CLI Primitive

**Status:** Implemented
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
- [Decisions](#decisions)
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
  `Draft → Accepted` requires going through `Proposed`). List
  membership is the only check — Decision 4
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
  --dry-run         print the change, do not write
  --quiet           suppress success messages on stdout (exit code unchanged)
  --format text|json  output format (default: text)
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
4. Match `<id>` against `entry.Frontmatter.ID` with case-sensitive
   exact equality (Decisions 2 and 3 — no fuzzy/prefix matching).
   On no match, exit 1
5. Validate `<new-status>` against `typeConfig.Statuses` with
   case-sensitive exact equality (Decision 2); list membership
   only, no transition graph (Decision 4). On invalid, exit 2 and
   list the valid values
6. No-op short-circuit: if `entry.Frontmatter.Status == new-status`,
   print "already at <status>" and exit 0 without touching the file
7. Otherwise invoke `document.SetStatus(path, newStatus)` (new
   helper — see §Data Model), print the transition line, exit 0

### Frontmatter mutation

The mutation is intentionally byte-level, not a YAML round-trip:

- Read the file via `os.ReadFile`
- Locate the leading `---\n` and the closing `---\n`. Non-LF
  endings are rejected with a clear "unsupported line endings"
  error per Decision 7
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
| 2 | Validation failure: unknown type, invalid status (not in `statuses:`), unsupported line endings (Decision 7) |

These match the conventions Issue #52 calls out (typo → exit 2,
not-found → exit 1).

### Output format

`--format` selects the output shape; default is `text`. The flag
mirrors `docz list` which already uses `--format=table|json|csv`,
keeping the cmd's flag surface consistent (Decision 6).

**Text** (`--format=text`):

```text
<relative-path>: status <old> -> <new>
```

The no-op message is `<relative-path>: already at <status>`. Both
are prefixed with `[dry-run] ` when `--dry-run` is set.

**JSON** (`--format=json`):

```json
{
  "path": "docs/rfc/0042-payment-rate-limits.md",
  "from": "Draft",
  "to": "Accepted",
  "dry_run": false,
  "changed": true
}
```

The `changed` field is `false` on a no-op (current == new); `from`
and `to` are equal in that case. JSON output is always a single
object on stdout terminated by a trailing newline; errors still go
to stderr as plain text so CI logs aren't confused by partial JSON
on a failure path.

Errors always go to stderr in the existing `Error: <message>` plain
format used by every other command, regardless of `--format`.

`--quiet` suppresses stdout on success and no-op (exit code is the
contract; the line is for humans). Errors still print to stderr
under `--quiet`. When `--quiet` is combined with `--format=json` the
JSON object is also suppressed — `--quiet` always wins.

## API / Interface Changes

New Cobra commands (`cmd/status.go`):

- `statusCmd` parent command, no `RunE`
- `statusSetCmd` subcommand with `RunE: runStatusSet` → calls
  `(*Runner).statusSet(ctx context.Context, args []string) error`
- Flags: `--dry-run bool`, `--quiet bool`, `--format string`
  (validated against `{text, json}` at startup, default `text`).
  Each bound to package-level vars per the existing pattern; the
  broader cmd-globals cleanup is deferred to a follow-up RFC per
  IMPL-0009 Phase 7's "blocked on per-command opts structs" note

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
- Idempotency: `cmd/status.go` does the current-vs-new check and
  short-circuits before calling `SetStatus`, so the helper itself
  always writes when called (Decision 8). Unit tests still cover
  the byte-perfect property by passing the helper a status it
  already has and comparing the bytes against the input file

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
- `--format=json` happy path → exit 0, single JSON object on stdout
  with `from`, `to`, `path`, `dry_run`, `changed` fields
- `--format=json` no-op → `changed:false`, `from == to`
- `--format=json` with `--dry-run` → `dry_run:true`, file unchanged
- `--format=json --quiet` → exit 0, empty buffer (quiet wins)
- `--format=bogus` → exit 2, error lists `text` and `json`

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

## Decisions

Resolved by user review on 2026-06-01. Every recommendation
accepted except #6, where the user picked (c) to give scripts a
first-class JSON output.

| # | Topic | Decision |
|---|-------|----------|
| 1 | Command shape | (a) `docz status set <type> <id> <new-status>` — parent + subcommand; leaves room for `status get` / `status list-allowed` without growing the top-level verb list |
| 2 | Status value case sensitivity | (a) Case-sensitive exact match — `Accepted` matches, `accepted` does not. Aligns with DESIGN-0004 §F's canonical `Status` display values |
| 3 | ID prefix matching | (a) Exact match only — `RFC-0042` required; mirrors `docz list` |
| 4 | Transition validation depth | (a) Membership only — new status must be in `statuses:`; any → any allowed |
| 5 | Multi-document batch mode | (a) Out of scope — one id per invocation, callers loop |
| 6 | Output format | **(c) Both — `--format=text|json`** with default `text`. Matches `docz list`'s existing flag style |
| 7 | CRLF / Windows line endings | (a) Reject non-LF endings with a clear error — matches docz's Unix-only stance per INV-0004 §Decisions 6 |
| 8 | Idempotency placement | (a) Cmd-level short-circuit — `cmd/status.go` checks current == new before calling `SetStatus`; the helper always writes when invoked |
| 9 | Status field name | (a) Hard-coded `status:` — `TypeConfig.StatusField` is `"status"` for every default type |
| 10 | `status get` / `status list-allowed` companion verbs | (a) Not in this design — `set` only; the `status` parent verb is established here so follow-up additions are one-file |

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
