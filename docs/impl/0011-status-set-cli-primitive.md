---
id: IMPL-0011
title: "Status Set CLI Primitive"
status: Completed
author: Donald Gifford
created: 2026-06-01
---
<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL 0011: Status Set CLI Primitive

**Status:** Completed
**Author:** Donald Gifford
**Date:** 2026-06-01

<!--toc:start-->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: Foundation — internal/document.SetStatus helper](#phase-1-foundation--internaldocumentsetstatus-helper)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2: cmd/status.go Cobra wiring + text output](#phase-2-cmdstatusgo-cobra-wiring--text-output)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 3: JSON output (--format=json)](#phase-3-json-output---formatjson)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 4: Verify and ship](#phase-4-verify-and-ship)
    - [Tasks](#tasks-3)
    - [Success Criteria](#success-criteria-3)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Decisions](#decisions)
- [Dependencies](#dependencies)
- [References](#references)
<!--toc:end-->

## Objective

Implement DESIGN-0005 — the `docz status set <type> <id> <new-status>`
CLI primitive that mutates a docz document's YAML frontmatter
`status:` field with byte-level precision, lifecycle validation
against `.docz.yaml`, and stable exit codes. The command's primary
consumer is a GitHub Action in rfc-api that syncs PR labels to
frontmatter; secondary consumers are release tooling and shell
scripts.

The implementation is intentionally small: one new `internal/document`
helper, one new `cmd/status.go` parent + subcommand, and two output
formats (text + JSON) selected by `--format`. Everything else reuses
existing infrastructure (`Config.ValidateType`, `document.ScanDocuments`,
`document.LoadFrontmatter`, the IMPL-0009 Runner pattern).

**Implements:** DESIGN-0005 — Status Set CLI Primitive.

## Scope

### In Scope

- New `internal/document/status.go` with `SetStatus(path, newStatus
  string) (oldStatus string, err error)` — byte-level mutator that
  preserves key order, quoting style, and surrounding whitespace
- New error sentinels in `internal/document`: `ErrStatusFieldMissing`
  and `ErrUnsupportedLineEndings`. `ErrNoFrontmatter` already exists
  and is reused
- New `cmd/status.go` with `statusCmd` parent and `statusSetCmd`
  subcommand, registered in `cmd/root.go`
- `(*Runner).statusSet(ctx context.Context, args []string) error`
  handler following the IMPL-0009 method pattern
- Three flags on `status set`: `--dry-run`, `--quiet`,
  `--format=text|json`
- Text and JSON output paths matching DESIGN-0005 §Output format
- Exit codes 0/1/2 matching DESIGN-0005 §Exit codes
- Cmd-level no-op short-circuit per DESIGN-0005 Decision 8 (helper
  always writes when called)
- Table-driven unit tests for the helper using golden fixtures
  under `internal/document/testdata/golden/status/`
- Cmd tests for every flag combination and error path via a
  constructed Runner with `Out: &bytes.Buffer{}` and
  `RepoRoot: t.TempDir()`
- `docz --help` text update is automatic via Cobra

### Out of Scope

- `status get` and `status list-allowed` companion subcommands —
  Decision 10 defers these to a follow-up design
- ID prefix matching, case-insensitive matching — Decision 3 locks
  exact match only
- Order-aware transition validation — Decision 4 locks list
  membership only
- Multi-document batch mode — Decision 5 defers
- `--field` flag overriding the `status:` field name — Decision 9
  hard-codes `status:`
- Windows / CRLF support — Decision 7 rejects non-LF endings
- Any change to `.docz.yaml` schema or `internal/config`

## Implementation Phases

Each phase produces a self-contained, lint-clean commit. Phase
boundaries follow buildable units: a helper, a CLI surface, an
output format, a ship gate.

---

### Phase 1: Foundation — `internal/document.SetStatus` helper

Build the byte-level mutator and its golden-test suite first. The
helper is independent of Cobra and `Runner`, so Phase 2 can rely on
it without re-testing the byte-level invariants.

#### Tasks

- [x] Create `internal/document/status.go` with the documented
      `SetStatus(path, newStatus string) (oldStatus string, err error)`
      signature
- [x] Define `ErrStatusFieldMissing` and `ErrUnsupportedLineEndings`
      as exported sentinels next to the existing `ErrNoFrontmatter`
      in `internal/document/document.go`
- [x] Implement the frontmatter delimiter scan: locate `---\n`
      opening on line 0 or 1, locate the closing `---\n`. Reject
      CRLF / mixed endings with `ErrUnsupportedLineEndings`
- [x] Implement the status-line finder using a tight regex that
      captures the value across three quoting shapes (bare,
      double-quoted, single-quoted). Block scalars and flow
      mappings return `ErrStatusFieldMissing` with a clear message
- [x] Implement the byte-level value replacement: preserve the key
      token, the colon, the spacing, the quoting glyphs, and any
      trailing comment. Only the value bytes change
- [x] Wrap all `os.ReadFile` / `os.WriteFile` errors with the file
      path: `fmt.Errorf("%s: %w", path, err)` (Decision 5)
- [x] Write `os.WriteFile` at the `config.FileMode` constant (0o644)
      to match every other writer in docz
- [x] Create golden fixtures under
      `internal/document/testdata/golden/status/` — one input file
      per built-in template (rfc/adr/design/impl/plan/investigation)
      plus six matching outputs after a status mutation. Use the
      `-update` flag pattern already established for other golden
      tests in this repo (Decision 4)
- [x] Write `internal/document/status_test.go` with `t.Parallel()`
      on every top-level test and subtest:
  - Six-template golden round-trip table
  - Quoting shape table: bare, `"Draft"`, `'Draft'`
  - No-space variant: `status:Draft` accepted, preserved
  - Trailing comment: `status: Draft  # current` — value replaced,
    comment preserved
  - CRLF input → `ErrUnsupportedLineEndings`
  - Missing `status:` line in valid block → `ErrStatusFieldMissing`
  - File with no frontmatter → `ErrNoFrontmatter`
  - Idempotency proof: calling `SetStatus(p, "Draft")` against a
    file already at `Draft` produces byte-identical output

#### Success Criteria

- `go build ./...` and `go vet ./...` clean
- `make lint` returns zero issues
- `go test -race -shuffle=on -count=3 ./internal/document/...` green
- Six golden fixtures present and byte-validated
- The helper has no dependency on `cmd/`, `Runner`, or
  `internal/config` (it can be reused outside docz if needed)

---

### Phase 2: `cmd/status.go` Cobra wiring + text output

Build the user-visible surface in its minimal form: parent command,
subcommand, type/id/status resolution, cmd-level no-op short-circuit,
text output, and full cmd-test coverage of the text path. JSON
output is layered on in Phase 3 — the `--format` flag exists in this
phase but only `text` is allowed; `json` returns "not yet implemented"
to keep this phase's surface area small.

Per Decision 1 the work is split: Phase 2 lands the parent + subcommand,
`--format` flag wiring, and the text path; Phase 3 layers JSON onto the
same `runStatusSet`. The flag parsing and validation are written once in
Phase 2.

#### Tasks

- [x] Create `cmd/status.go` with `statusCmd` (parent, no `RunE`,
      relies on Cobra's default help-when-no-subcommand behavior)
- [x] Add `statusSetCmd` with `Args: cobra.ExactArgs(3)` and
      `RunE: runStatusSet`
- [x] Implement `func runStatusSet(cmd *cobra.Command, args []string)
      error` that calls `(*Runner).statusSet(cmd.Context(), args)`
- [x] Implement `(r *Runner) statusSet(ctx context.Context, args
      []string) error` with the DESIGN-0005 §Resolution algorithm:
  1. `args[0]` → `Config.ValidateType` (error → exit 2)
  2. Resolve type dir via `r.inRepo(typeConfig.Dir)`
  3. `document.ScanDocuments(typeDir)` → walk entries, match
     `args[1]` against `entry.Frontmatter.ID` with case-sensitive
     equality. No match → exit 1 with the type dir scanned
  4. Validate `args[2]` against `typeConfig.Statuses` via a
     case-sensitive `slices.Contains` check. Invalid → exit 2,
     list valid values in registry-declared order
  5. If `string(entry.Frontmatter.Status) == args[2]`: print the
     "already at" line, exit 0 without calling `SetStatus`
  6. Otherwise, if not `--dry-run`: call
     `document.SetStatus(entry.Path, args[2])`. On error, exit 1
- [x] Add `--dry-run`, `--quiet`, `--format` flag declarations on
      `statusSetCmd`. `--format` defaults to `text`; validate
      `text|json` membership at the top of `statusSet` (Decision 2)
- [x] Register `statusCmd` in `cmd/root.go`'s `init()` next to the
      other top-level command registrations
- [x] Build the text output line via a small `formatStatusText(...)`
      helper inside `cmd/status.go`. Format:
      `<relpath>: status <old> -> <new>` (success) or
      `<relpath>: already at <status>` (no-op), prefixed with
      `[dry-run] ` when `--dry-run` is set
- [x] Relative path printed is **relative to `r.RepoRoot`** (Decision 3)
- [x] Errors print to `r.Err` in the existing `Error: <message>`
      format — never JSON
- [x] `--quiet` suppresses success/no-op stdout but never stderr
- [x] Define `errExitCode1` and `errExitCode2` as sentinel errors in
      `cmd/status.go` (Decision 6); `runStatusSet` returns them and
      `Execute()` in `cmd/root.go` translates them to `os.Exit(1)` and
      `os.Exit(2)`. Cobra's `SilenceUsage: true` (already set on
      rootCmd) prevents the usage dump on a non-zero exit
- [x] Write `cmd/status_test.go` covering the text path:
  - Happy: type=rfc, id=RFC-0001, new=Accepted → exit 0, file
    mutated, buffer matches expected line
  - No-op: current == new → exit 0, file unchanged, buffer
    matches "already at" line
  - `--dry-run` → exit 0, file unchanged, buffer matches
    `[dry-run]` line
  - `--quiet` happy → exit 0, file mutated, empty buffer
  - Unknown type → exit 2, error mentions valid types
  - Unknown id → exit 1, error mentions the type dir scanned
  - Invalid status → exit 2, error lists `statuses:` in registry
    order
  - CRLF file → exit 2, error mentions unsupported line endings
  - `--format=bogus` → exit 2, error lists `text` and `json`
  - Path arg with `--repo-root` set: id resolved under the
    repo-root, not cwd
- [x] Every cmd test constructs a `Runner` directly with
      `Out: &bytes.Buffer{}` and `RepoRoot: t.TempDir()` per the
      IMPL-0009 pattern (no `os.Pipe`, no `os.Chdir`)

> **Implementation notes (intentional deviations from the literal task
> text above):**
>
> - **Resolution step 2** uses `r.inRepo(r.Cfg.TypeDir(typeName))`, not
>   `r.inRepo(typeConfig.Dir)`. `typeConfig.Dir` is just the leaf (e.g.
>   `rfc`) and omits the configured `docs_dir`; `TypeDir` joins both
>   (`docs/rfc`) the same way `cmd/list.go` does, and `inRepo` roots it
>   under `RepoRoot`.
> - **Resolution step 6** uses `filepath.Join(typeDir, entry.Filename)`
>   because `document.DocEntry` exposes `Filename`, not a `Path` field.
> - **Handler signature** is `statusSet(opts statusSetOpts, args []string)`
>   — no `context.Context`. The handler does no context-aware work, and
>   `unparam`/`revive` flag an unused `ctx`; this matches
>   `cmd/list.go`'s `List(opts, args)`. Flags are packed into
>   `statusSetOpts` (like `createOpts`/`listOpts`) so the method never
>   reads a package global.
> - **Exit-code plumbing** wraps `errExitCode1`/`errExitCode2` in an
>   `exitCodeError{msg, marker}` so the user-facing message rides on the
>   returned error (Cobra prints `Error: <message>`) while
>   `exitCodeFor` in `root.go` selects the code via `errors.Is`. The
>   handler returns errors; it never writes to `r.Err` itself.

#### Success Criteria

- `docz status set rfc RFC-0001 Accepted` works against this repo
  end-to-end and prints the expected text line
- `docz status set --dry-run rfc RFC-0001 Accepted` prints
  `[dry-run] …` and does not mutate the file
- `docz status set rfc bogus Accepted` exits 1 with a clear error
- `docz status set rfc RFC-0001 Approved` exits 2 listing the
  valid statuses
- `docz --help` shows `status` in the command list and
  `docz status set --help` shows all three flags
- `go test -race -shuffle=on -count=3 ./cmd/... -run TestStatusSet`
  green
- `make lint` clean

---

### Phase 3: JSON output (`--format=json`)

Layer the JSON path onto the Phase 2 cmd. Phase 2's text helper
stays untouched; this phase adds a sibling `formatStatusJSON(...)`
emitter and the test cases that exercise it.

#### Tasks

- [x] Add `formatStatusJSON(...)` helper returning a single-line
      JSON object via `encoding/json` (`Marshal` + a trailing
      `\n`). Fields: `path`, `from`, `to`, `dry_run`, `changed`
- [x] On no-op: `from` and `to` are equal, `changed: false` (Decision 7)
- [x] On `--dry-run`: `dry_run: true`, file unchanged, `changed`
      still reports what *would* have happened
- [x] `--quiet` suppresses the JSON object completely (DESIGN-0005
      Decision 6's "quiet wins" clause)
- [x] Add JSON cmd tests parallel to the text ones:
  - `--format=json` happy → exit 0, single JSON object on stdout
    with all five fields populated correctly
  - `--format=json` no-op → `changed:false`, `from == to`
  - `--format=json --dry-run` → `dry_run:true`, file unchanged
  - `--format=json --quiet` → exit 0, empty buffer
  - `--format=json` on error → JSON not emitted, stderr has plain
    `Error: …` text
- [x] JSON tests use `json.Unmarshal` into a struct rather than
      string-matching, to avoid spurious failures from field
      ordering (Go's `encoding/json` happens to emit struct field
      order deterministically, but the test shouldn't depend on
      that)

> **Implementation note:** the emitter is the `(*Runner).emitStatusJSON`
> method (with a `statusJSON` struct), not a free `formatStatusJSON`
> function — it writes through `r.Out` like `emitStatusText`. The Phase 2
> `emitStatus` json branch (previously a "not yet implemented" stub) now
> dispatches to it.

#### Success Criteria

- `docz status set --format=json rfc RFC-0001 Accepted` emits a
  single JSON object terminated by a newline, parseable by
  `jq '.changed'`
- `docz status set --format=json --dry-run rfc RFC-0001 Accepted`
  has `dry_run: true` and leaves the file unchanged
- `docz status set --format=json --quiet rfc RFC-0001 Accepted`
  exits 0 with empty stdout
- `docz status set --format=json rfc BAD-0001 Accepted` exits 1
  with plain-text stderr and no JSON on stdout
- `go test -race ./cmd/... -run TestStatusSet` green for the
  expanded JSON test set
- `make lint` clean

---

### Phase 4: Verify and ship

#### Tasks

- [x] Full `make ci` green (lint + test + build + license-check)
- [x] `go test -race -shuffle=on -count=3 ./...` green
- [x] Manual smoke against this repo:
      `docz status set design DESIGN-0005 Implemented` then
      `git diff docs/design/0005-status-set-cli-primitive.md` shows
      exactly one line changed (the `status:` value); the
      `**Status:**` body line is *not* touched (Decision 8). Verified
      (numstat `1 1`, body `**Status:** Approved` untouched), then the
      smoke mutation was reverted — the real flip is task below.
- [x] Manual smoke with `--format=json` against this repo;
      pipe through `jq` to confirm shape (ran as `--dry-run` so the
      repo stays clean; `jq '.changed'` → `true`)
- [x] Update CLAUDE.md architecture section to add a one-line
      entry under `cmd/` describing `status set` and pointing at
      `internal/document.SetStatus` as the byte-level seam (landed
      in the Phase 2 commit)
- [x] Run `docz update` to refresh the impl/design index READMEs
      and the mkdocs nav (no diff — indexes already current)
- [ ] Flip DESIGN-0005 frontmatter status from `Approved` to
      `Implemented` after the PR merges to main *(post-merge: not done
      while unmerged)*
- [ ] Flip this doc's frontmatter status from `Draft` to
      `Completed` after the PR merges to main *(post-merge: not done
      while unmerged)*
- [ ] Open the PR with the `dont-release` label so the rfc-api
      Action consuming this can be coordinated with the next docz
      release *(left to the user; not auto-opening a PR)*

#### Success Criteria

- `make ci` green on the final commit
- All three smoke runs (text, JSON, dry-run) produce the expected
  outputs against this repo's own docs
- CLAUDE.md mentions `status set` in the architecture section
- Branch is ready to squash-merge with the standard
  `gh pr merge --squash --delete-branch` flow

---

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/document/status.go` | Create | `SetStatus` helper + new error sentinels |
| `internal/document/document.go` | Modify | Export `ErrStatusFieldMissing`, `ErrUnsupportedLineEndings` next to `ErrNoFrontmatter` |
| `internal/document/status_test.go` | Create | Golden round-trip + edge-case table tests |
| `internal/document/testdata/golden/status/*.md` | Create | Six input + six output fixtures (one pair per built-in type) |
| `cmd/status.go` | Create | `statusCmd` parent + `statusSetCmd` subcommand + `(*Runner).statusSet` |
| `cmd/status_test.go` | Create | Text + JSON path coverage via constructed Runner |
| `cmd/root.go` | Modify | Register `statusCmd` |
| `CLAUDE.md` | Modify | One-line entry in the cmd/ architecture line for `status set` |
| `docs/design/0005-status-set-cli-primitive.md` | Modify | Flip status `Approved` → `Implemented` on merge |
| `docs/impl/0011-status-set-cli-primitive.md` | Modify | Flip status `Draft` → `Completed` on merge |
| `docs/impl/README.md` | Modify | Auto-regenerated by `docz update` |

## Testing Plan

- [x] `internal/document/status_test.go` covers every quoting
      shape and edge case from Phase 1's task list
- [x] Golden fixtures live under
      `internal/document/testdata/golden/status/` and are
      regenerable with `go test -run TestSetStatus -update ./internal/document/...`
- [x] `cmd/status_test.go` covers the full happy/no-op/error
      matrix for both text and JSON output and every flag
      combination
- [x] `go test -race -shuffle=on -count=3 ./...` green at the end
      of every phase
- [x] One end-to-end smoke against this repo's own docs is captured
      in Phase 4's task list

## Decisions

Resolved 2026-06-01. All recommendations accepted.

| # | Topic | Choice | Rationale |
|---|-------|--------|-----------|
| 1 | Phase split | (a) Two phases — text in Phase 2, JSON in Phase 3 | Clearer commit history; each phase a small, reviewable diff; matches DESIGN-0005's output-format section ordering |
| 2 | `--format` validation site | (a) Inside `runStatusSet` at the top | Single function owns the contract; matches `listFormat` validation in `cmd/list.go` |
| 3 | Path format in output | (a) Relative to `r.RepoRoot` | Stable across cwd's; matches DESIGN-0005's example (`docs/rfc/0042-…md`); what `RepoRoot` is for |
| 4 | Golden fixture layout | (a) `internal/document/testdata/golden/status/<type>.{input,output}.md` | Standard golden-file convention; pairs per type are easy to point at on failure |
| 5 | Helper error wrapping | (a) Wrap every IO error with the file path | `fmt.Errorf("%s: %w", path, err)`; cmd layer can re-wrap with user-facing context |
| 6 | Exit code mechanism | (a) Sentinel error types (`errExitCode1`, `errExitCode2`) | Returned by `(*Runner).statusSet`, translated by `Execute()` in `cmd/root.go`; keeps handlers testable |
| 7 | JSON no-op shape | (a) `from == to`, `changed: false` | Preserves field positions; consumers branch on `changed`; matches DESIGN-0005's example |
| 8 | Body `**Status:**` line | (a) Don't touch it | Canonical status is frontmatter; mutating prose would force the helper to understand templating conventions |
| 9 | CLAUDE.md scope | (a) One-line addition pointing at `internal/document.SetStatus` | Matches the existing terse style of the cmd/ architecture bullet |
| 10 | PR shape | (a) Single PR for all four phases | <500 LOC total; follows IMPL-0009's Wave 5 single-PR precedent |

## Dependencies

- **Blocking:** none on the docz side. DESIGN-0005 is `Approved`;
  IMPL-0009 (Runner pattern, `inRepo`, `RepoRoot`) is merged to
  main as of 2026-05-29 and is the foundation every cmd file in
  this PR will build on
- **Coordinated with:** rfc-api **DESIGN-0004** §"docz status
  primitive" — the rfc-api PR (<https://github.com/donaldgifford/rfc-api/pull/34>)
  is the primary consumer; ship this with the `dont-release` label
  until rfc-api is ready to bump its docz pin
- **Followed by:** A separate design + impl pair for `status get`
  and `status list-allowed` if demand materializes (Decision 10
  defers these; the `status` parent verb is established here so
  the follow-ups are one-file additions)

## References

- [DESIGN-0005](../design/0005-status-set-cli-primitive.md) — the
  design this implements; every decision in §Decisions is the
  authoritative answer for the corresponding implementation choice
- [Issue #52](https://github.com/donaldgifford/docz/issues/52) —
  the original feature request
- rfc-api **DESIGN-0004** §"docz status primitive" —
  <https://github.com/donaldgifford/rfc-api/pull/34>
- [DESIGN-0004](../design/0004-runner-pattern-and-doctype-registry.md)
  — Runner method pattern that `(*Runner).statusSet` follows
- [IMPL-0009](0009-runner-pattern-and-doctype-registry-refactor.md)
  — the implementation shipping the Runner + `RepoRoot` + `inRepo`
  helpers this PR builds on
- `internal/document/scan.go` — `ScanDocuments` reused for id lookup
- `internal/document/document.go` — `LoadFrontmatter` and the
  existing `ErrNoFrontmatter` sentinel
- `internal/config/doctype.go` — `TypeConfig.Statuses` is the
  lifecycle source of truth
