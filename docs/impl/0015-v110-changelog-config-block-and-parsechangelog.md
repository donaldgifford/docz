---
id: IMPL-0015
title: "v1.1.0 — changelog config block and ParseChangelog"
status: Draft
author: Donald Gifford
created: 2026-08-02
---
<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL 0015: v1.1.0 — changelog config block and ParseChangelog

**Status:** Draft
**Author:** Donald Gifford
**Date:** 2026-08-02

<!--toc:start-->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: changelog: config block](#phase-1-changelog-config-block)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2: document.ParseChangelog](#phase-2-documentparsechangelog)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 3: CLI surfacing + consumer proof](#phase-3-cli-surfacing--consumer-proof)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 4: living docs + the v1.1.0 release](#phase-4-living-docs--the-v110-release)
    - [Tasks](#tasks-3)
    - [Success Criteria](#success-criteria-3)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Dependencies](#dependencies)
- [Open Questions](#open-questions)
  - [1. PR / release strategy for this branch?](#1-pr--release-strategy-for-this-branch)
  - [2. Source of the fleet-standard fixture?](#2-source-of-the-fleet-standard-fixture)
  - [3. ParseChangelog return shape?](#3-parsechangelog-return-shape)
- [Decisions](#decisions)
- [References](#references)
<!--toc:end-->

## Objective

Implement DESIGN-0010: an opt-in `changelog:` block in `.docz.yaml`
(`enabled`/`file`) and a byte-based `document.ParseChangelog` that parses the
fleet-standard git-cliff / Keep-a-Changelog shape into versions → groups →
items — shipped together as **one additive minor release (`v1.1.0`)** so
docz-api can bump its pin once and freeze the surface as contract clause R6.

**Implements:** DESIGN-0010 (all nine open questions resolved **(a)**,
2026-08-02 — see its Decisions table).

## Scope

### In Scope

- `config`: `ChangelogConfig`, `Config.Changelog`, defaults, empty-`file`
  backfill, enabled-only validation (DESIGN-0010 Decisions 5/7).
- `document`: `Changelog` / `ChangelogVersion` / `ChangelogGroup`,
  `ErrNoVersions`, `ParseChangelog` (Decision 1) with fence-aware,
  best-effort, never-panic parsing (Decisions 4/8/9).
- `docz init` template surfacing (Decision 6) + `docz config` verification.
- Consumer-module proof, living docs, and the `v1.1.0` release.

### Out of Scope

- Any `docz changelog` CLI subcommand; changelog generation (git-cliff owns
  it); wiki/mkdocs nav integration; commit→file backlinks (docz-api feature 2).
- docz-api's own work: the pin bump, contract clause R6, fetch/serve endpoints.

## Implementation Phases

Each phase builds on the previous one. A phase is complete when all its tasks
are checked off and its success criteria are met.

---

### Phase 1: `changelog:` config block

The config side in `pkg/doczcore/config`: struct, defaults, merge/backfill
semantics, and enabled-only validation.

#### Tasks

- [x] Add `ChangelogConfig{Enabled bool, File string}` (yaml tags
      `enabled`/`file`), `Config.Changelog` with yaml tag `changelog`, and the
      `DefaultConfig()` entry `{Enabled: false, File: DefaultChangelogFile}`.
- [x] Add the `changelog:` block (`enabled: false`, default `file`, one-line
      comment) to `internal/template/templates/docz_yaml.tmpl`, rendered from
      `DefaultConfig()` like the `wiki:`/`toc:` blocks (Decision 6). **Moved
      here from Phase 3 during implementation:** the pre-existing
      `TestDoczYAMLTemplate_RoundTripsToDefaultConfig` guard renders the
      template with `DefaultConfig()` and parses it back, so a new default
      field with no template line fails immediately — the template edit is
      part of adding the field, not a later polish step.
- [ ] Add the post-decode normalization step in `Load` (runs after
      `decodeSettings`, beside the `fillTypeFieldDefaults` call): backfill an
      explicitly empty `file: ""` to the default, and normalize a leading
      `./` — `Load` mutates, `Validate` only judges. Note in a comment why
      this is new code: no existing top-level block backfills empty values
      (`fillTypeFieldDefaults` exists only for the `Types` map case).
- [ ] Add `validateChangelog` to `Config.Validate()`: **only when
      `Enabled: true`** (Decision 7 — a dormant block never fails load),
      hard-error on absolute paths, `..` traversal, and trailing `/`
      (Decision 5). Error wording follows the existing `Validate()` style.
- [ ] Config test table: absent block (defaults), partial block
      (`enabled` only → `File` keeps default), explicit `file: ""`
      (backfilled), full block, leading `./` normalized, each invalid shape
      rejected **only when enabled** (same values accepted while disabled),
      unknown sibling keys still tolerated; extend `parity_baseline_test.go`
      to pin that a pre-existing `changelog:` block that older configs carried
      dormant now decodes instead of being ignored (the INV-0005 F2 rollout
      handshake).
- [ ] Field/doc comments on the new symbols; `make fmt` + `make lint` green.

#### Success Criteria

- Config table green; `docz config` prints the resolved block (no cmd/ code
  change needed — falls out of the struct field).
- Unknown-key leniency and case-sensitivity pins in `parity_baseline_test.go`
  still green (the rollout guarantee is intact).
- `make ci` green.

---

### Phase 2: `document.ParseChangelog`

The parser in `pkg/doczcore/document` (Decision 1), next to
`ParseFrontmatter`: same bytes-in / typed-struct / sentinel-error shape.

#### Tasks

- [ ] Add `changelog.go`: `Changelog{Preamble, Versions}`,
      `ChangelogVersion{Version, Unreleased, Date, Groups}`,
      `ChangelogGroup{Title, Items}`, `ErrNoVersions`, and
      `ParseChangelog(raw []byte) (*Changelog, error)` — the DESIGN-0010
      signature as handed off (Decision 3; deliberate asymmetry with
      `ParseFrontmatter`'s value return, noted in the doc comment).
- [ ] Version-heading recognition: line matching `## [<ver>]` or
      `## [<ver>] - <date>`; normalization strips the brackets and a single
      leading lowercase `v` ("0.4.2"); `unreleased` matched
      case-insensitively sets `Unreleased: true` and leaves `Date` empty;
      `Date` is the raw `YYYY-MM-DD` text, unvalidated (Decision 2).
- [ ] Fence-aware line walk (Decision 4): an internal helper mirroring the
      module's fence rule (trimmed-line ` ``` ` toggle; tildes do not toggle)
      — duplicated locally with a comment naming the docparse rule it
      mirrors, **not** a new export or a docparse import.
- [ ] Preamble + grouping rules: `Preamble` is everything before the first
      version-heading line, byte-verbatim (Decision 3 — including the
      no-blank-line-before-first-version shape the real fleet fixture has);
      `###` opens a group; content inside a version before any `###` goes to
      a `Title: ""` group; bullet lines strip only the `- ` marker; everything
      indented under a top-level bullet (wrapped prose *and* nested
      sub-bullets, their `-` markers kept) folds into that item verbatim
      (Decision 8); duplicate version headings emit separate entries in
      document order (Decision 9, stated in the doc comment).
- [ ] Fixtures + golden fact tests under
      `pkg/doczcore/document/testdata/changelog/` (repo `-update`
      convention): the fleet-standard fixture — a verbatim snapshot of
      docz-api's real `CHANGELOG.md`, trimmed to a few versions
      (Decision 2) — a
      `charts/<name>/CHANGELOG.md`-shaped fixture, and edge fixtures — empty
      input and prose-only input (both `ErrNoVersions` via `errors.Is`),
      unreleased-only, version with no groups, bullets before any group
      heading, multi-line bullet continuation, nested sub-bullets, duplicate
      versions, fenced `## [1.0.0]` decoy.
- [ ] `FuzzParseChangelog` fuzz test pinning the never-panic contract
      (Decision: "never panic on arbitrary input" is a contract line, so it
      gets an executable check, not just review).
- [ ] Doc comments: the best-effort/never-panic contract, duplicate-version
      emission, and the `ErrNoVersions`-mirrors-`ErrNoFrontmatter` skip-vs-fail
      guidance; `make fmt` + `make lint` green.

#### Success Criteria

- Golden tests green over all fixtures; `ErrNoVersions` identity via
  `errors.Is` pinned; a short fuzz run (`go test -fuzz=FuzzParseChangelog
  -fuzztime=30s ./pkg/doczcore/document/`) finds no panics.
- Version identity spot-pins: `[0.4.2]` → `0.4.2`, `[v0.4.2]` → `0.4.2`,
  `[Unreleased]` → `unreleased` + `Unreleased: true`.
- The package stays free of new third-party deps; `make ci` green.

---

### Phase 3: CLI surfacing + consumer proof

Make the block discoverable (Decision 6) and prove the new surface from
outside the module before it freezes.

#### Tasks

- [ ] cmd/ test: `docz config` output includes the resolved `changelog:`
      block (and honors a repo override), following the existing serial
      Runner test pattern.
- [ ] Extend `test/consumer/`: decode a `.docz.yaml` carrying the block
      (defaults + partial-block backfill assertions) and run `ParseChangelog`
      over fixture bytes, asserting version identity/order, group titles,
      item counts, preamble, and the `ErrNoVersions` sentinel — the R6
      contract shape docz-api will pin, proven from an external module first.
- [ ] README: add the `changelog:` block to the Configuration section and a
      `ParseChangelog` mention to the "Using docz as a Go Library" table's
      `document` row.

#### Success Criteria

- `make ci` green including `make test-consumer`; the consumer module
  exercises both the config block and the parser.
- `docz init` output carries the dormant block; `docz config` prints it
  resolved.

---

### Phase 4: living docs + the `v1.1.0` release

#### Tasks

- [ ] Update `CLAUDE.md` (config + document architecture bullets gain the
      changelog block/parser) and `DEVELOPMENT.md` (package responsibility
      sections); historical `docs/*` records stay untouched.
- [ ] Write the PR's `### RELEASE NOTES`: the additive `v1.1.0` surface
      (config block dormant by default; `ParseChangelog` best-effort parser),
      the rollout note (older docz versions ignore the block; v1.1.0 begins
      validating it **only when enabled**), and the docz-api R6 pin pointer.
- [ ] Open **one** release PR from `feat/docz-changelog-support` carrying
      the design doc, this IMPL, and all four phases, labeled **`minor`**
      (Decision 1); on merge, `pr-semver-bump` + goreleaser cut `v1.1.0`
      (merge to `main` is human-gated).
- [ ] Post-tag: scratch-module `go get github.com/donaldgifford/docz@v1.1.0`
      exercising `Config.Changelog` + `ParseChangelog`; flip DESIGN-0010 →
      Implemented and this IMPL → Completed (`dont-release` PR, the #67/#68
      pattern); notify docz-api that the pin bump + contract clause R6 are
      unblocked.

#### Success Criteria

- The `v1.1.0` tag exists with a successful goreleaser release; the scratch
  module imports and runs the new surface.
- No living doc is stale (CLAUDE.md/DEVELOPMENT.md/README describe the
  block + parser); DESIGN-0010 = Implemented, IMPL-0015 = Completed.
- docz-api notified; no breaking change to any existing v1.0.0 symbol
  (`gorelease`-style sanity: additive only).

---

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `pkg/doczcore/config/config.go` | Modify | `ChangelogConfig`, `Config.Changelog`, backfill + `./` normalization in `Load`, `validateChangelog` in `Validate()` |
| `pkg/doczcore/config/config_test.go` | Modify | Config table for the block |
| `pkg/doczcore/config/parity_baseline_test.go` | Modify | Dormant-block rollout pin |
| `pkg/doczcore/document/changelog.go` | Create | Types, `ErrNoVersions`, `ParseChangelog` |
| `pkg/doczcore/document/changelog_test.go` | Create | Golden + edge + fuzz tests |
| `pkg/doczcore/document/testdata/changelog/` | Create | Fleet, chart, and edge fixtures + golden facts |
| `internal/template/templates/docz_yaml.tmpl` | Modify | Dormant `changelog:` block (Decision 6) |
| `cmd/config_*_test.go` | Modify | Resolved-block output pin |
| `test/consumer/consumer_v1_test.go` (or sibling) | Modify | External proof of block + parser |
| `README.md`, `CLAUDE.md`, `DEVELOPMENT.md` | Modify | Living docs |

## Testing Plan

- [ ] Config table + parity pins (Phase 1) — merge, backfill, enabled-only
      validation, rollout tolerance.
- [ ] Parser goldens over fleet/chart/edge fixtures + fuzz never-panic
      (Phase 2).
- [ ] cmd/ `docz config`/`docz init` output pins (Phase 3).
- [ ] Consumer-module external proof of the full new surface (Phase 3).
- [ ] `make ci` gates every phase; zero churn outside intended goldens.

## Dependencies

- DESIGN-0010 (Accepted decisions; this plan encodes them).
- docz-api INV-0005 (requirements source) and its pin-bump procedure
  (INV-0001) — consumer-side, not blocking.
- No new Go module dependencies (stdlib-only parser).

## Open Questions

### 1. PR / release strategy for this branch?

**Resolved: (a)** — locked 2026-08-02; see [Decisions](#decisions).

The design doc (uncommitted) and the implementation both live on
`feat/docz-changelog-support`. IMPL-0014 shipped as one branch → one `major`
PR; docs-only changes historically go in separate `dont-release` PRs.

- **a. (Recommended)** One PR from this branch carrying the design doc,
  IMPL-0015, and all four phases, labeled **`minor`** → merge cuts `v1.1.0`;
  a small post-tag `dont-release` PR flips DESIGN-0010/IMPL-0015 statuses
  (the #67/#68 pattern). Matches "one additive minor release" and keeps the
  fleet pin dance to a single bump.
- b. Split: `dont-release` docs PR first (DESIGN-0010 + IMPL-0015), then the
  implementation PR with `minor`. Cleaner review separation, one more PR
  round-trip.
- c. Per-phase PRs (`dont-release` until the last, which carries `minor`).
  Most granular; most ceremony for a two-package additive feature.
- d. Other.

### 2. Source of the fleet-standard fixture?

**Resolved: (a)** — locked 2026-08-02; see [Decisions](#decisions).

DESIGN-0010 calls docz-api's `CHANGELOG.md` "representative". It exists
locally (preamble + unreleased + released versions, `*(scope)*` markers, PR
links — and a realistic quirk: no blank line between the preamble and
`## [unreleased]`). docz itself has no `CHANGELOG.md`.

- **a. (Recommended)** Snapshot docz-api's real `CHANGELOG.md` into
  `testdata/changelog/fleet.md` verbatim (trimmed to a few versions if long).
  A real git-cliff artifact pins the actual fleet shape, quirks included, and
  the fixture is frozen — it won't drift when docz-api's file changes.
- b. Hand-craft a synthetic fleet fixture from the design's example block.
  Fully controlled, but risks encoding the design's idealized shape (e.g.
  `_(scope)_` vs the real `*(scope)*`, blank-line placement) instead of what
  git-cliff actually emits.
- c. Other.

### 3. `ParseChangelog` return shape?

**Resolved: (a)** — locked 2026-08-02; see [Decisions](#decisions).

DESIGN-0010's signature is `(*Changelog, error)`, but its stated precedent —
`ParseFrontmatter` in the same package — returns a value
`(Frontmatter, error)`. Whichever ships is frozen public API.

- **a. (Recommended)** Keep `(*Changelog, error)` as designed — nil on error
  is unambiguous ("no partial results"), the struct carries slices so the
  pointer avoids copying, and it's the exact signature the docz-api handoff
  encodes and will pin as R6.
- b. Return `(Changelog, error)` for intra-package consistency with
  `ParseFrontmatter`. Symmetry, but deviates from the handed-off signature
  docz-api's contract work is already written against.
- c. Other.

## Decisions

All three open questions resolved **(a)** on 2026-08-02.

| #   | Question                 | Resolution                                                                                                                       |
| --- | ------------------------ | --------------------------------------------------------------------------------------------------------------------------------- |
| 1   | PR / release strategy    | One `minor` PR from `feat/docz-changelog-support` (design + IMPL + implementation) → `v1.1.0`; post-tag `dont-release` status flip |
| 2   | Fleet fixture source     | Verbatim snapshot of docz-api's real `CHANGELOG.md` (git-cliff quirks included), frozen in testdata                                |
| 3   | `ParseChangelog` return  | `(*Changelog, error)` as handed off — nil on error; the R6 contract signature docz-api encodes                                     |

## References

- DESIGN-0010 — Changelog config block and ParseChangelog (all decisions)
- docz-api INV-0005 (requirements), INV-0001 (pin-bump procedure),
  `internal/doczcontract` clause R6 (the pin target)
- IMPL-0014 / ADR-0001 — the v1.0.0 freeze this release adds to
- Keep a Changelog 1.1.0; git-cliff + the fleet-shared `cliff.toml`
