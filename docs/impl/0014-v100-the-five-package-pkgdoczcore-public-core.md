---
id: IMPL-0014
title: "v1.0.0: the five-package pkg/doczcore public core"
status: In Progress
author: Donald Gifford
created: 2026-07-03
---

<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL 0014: v1.0.0: the five-package pkg/doczcore public core

**Status:** In Progress **Author:** Donald Gifford **Date:** 2026-07-03

<!--toc:start-->

- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Affected code (verified inventory)](#affected-code-verified-inventory)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: viper-free config.Load (handoff R1)](#phase-1-viper-free-configload-handoff-r1)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2: create pkg/doczcore/docparse (handoff R2, revised)](#phase-2-create-pkgdoczcoredocparse-handoff-r2-revised)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 3: promote docwrite whole + add CheckTask (handoff R3)](#phase-3-promote-docwrite-whole--add-checktask-handoff-r3)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 4: promote toc as the splice package over docparse (ADR-0001 OQ4a)](#phase-4-promote-toc-as-the-splice-package-over-docparse-adr-0001-oq4a)
    - [Tasks](#tasks-3)
    - [Success Criteria](#success-criteria-3)
  - [Phase 5: consumer proof + public-surface hygiene](#phase-5-consumer-proof--public-surface-hygiene)
    - [Tasks](#tasks-4)
    - [Success Criteria](#success-criteria-4)
  - [Phase 6: living docs, release, and the v1.0.0 tag](#phase-6-living-docs-release-and-the-v100-tag)
    - [Tasks](#tasks-5)
    - [Success Criteria](#success-criteria-5)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Dependencies](#dependencies)
- [Open Questions](#open-questions)
  - [1. PR strategy — per-phase PRs or one atomic PR?](#1-pr-strategy--per-phase-prs-or-one-atomic-pr)
  - [2. Unknown-key handling in the yaml-only Load?](#2-unknown-key-handling-in-the-yaml-only-load)
  - [3. ParsePlan phase semantics — which ### headings are phases?](#3-parseplan-phase-semantics--which--headings-are-phases)
  - [4. Does the UpdateFiles family go public with toc?](#4-does-the-updatefiles-family-go-public-with-toc)
- [Decisions](#decisions)
- [References](#references)
<!--toc:end-->

## Objective

Implement **ADR-0001** as a single `v1.0.0` release: make `config.Load`
viper-free, create `pkg/doczcore/docparse` (heading + checkbox-task fact
extraction with byte-accurate line numbers — no plan interpretation, see
Decisions Q3), promote `internal/docwrite` **whole** (`SetStatus`, `Create`,
plus the new `CheckTask`), promote `internal/toc` as the ToC-splice package
delegating its walk to `docparse`, and freeze the resulting five-package surface
— `pkg/doczcore/{config,document,docparse,docwrite,toc}` — under semver via the
`major` release label. `template`, `index`, and `wiki` remain `internal/`.

The change is **behavior-preserving for the CLI**: no command, flag, output, or
`.docz.yaml`/frontmatter schema change; golden files must not move. There is
**no intermediate `v0.6.0`** — the sdk-booty-sh slate
(`docz-v0.6.0-requirements.md` R1–R3) ships inside this release, and
sdk-booty-sh pins the `v1.0.0` tag (ADR-0001 Decision / OQ2).

**Implements:** ADR-0001 (grounded in the INV-0006 per-package audit; absorbs
the `docz-v0.6.0-requirements.md` handoff R1–R3).

> **Numbering note:** earlier docs (ADR-0001's first draft) referred to a
> planned "IMPL-0014" covering a standalone v0.6.0. That plan was never written;
> this document takes the IMPL-0014 id and covers the full v1.0.0.

## Scope

### In Scope

- **Viper-free `config.Load`** (handoff R1): same exported signature, yaml-only
  implementation; `github.com/spf13/viper` (and its `mapstructure` indirect)
  leave `go.mod` entirely — `cmd/` has no direct viper import (verified
  2026-07-03).
- **New `pkg/doczcore/docparse`** (handoff R2, **revised** — Decisions Q3): two
  fact-extraction primitives with 1-based `Line` — `Headings` and `TaskItems`
  (every checkbox list item: fence-aware, `-`/`*`-bullet tolerant, `Indent`
  exposed) — absorbing the `internal/toc` walker and its `AnchorSlug` slug
  logic. The `Plan`/`Phase` **interpretation** moves to sdk-booty-sh's
  `doczwork`, built on these primitives.
- **`internal/docwrite` → `pkg/doczcore/docwrite`, whole** (handoff R3 +
  ADR-0001 Decision 2): `SetStatus` as-is, `Create` as-is (keeping its
  `internal/template` import — public→internal is legal, ADR-0001 Decision 3),
  plus the new `CheckTask` byte-splice.
- **`internal/toc` → `pkg/doczcore/toc`** (ADR-0001 OQ4a): the splice surface
  (`GenerateToC`/`UpdateToC`/`UpdateFiles`, `BeginMarker`/ `EndMarker`); its
  private walker (`ParseHeadings`/`Heading`/`AnchorSlug`) retires in favor of
  `docparse`.
- **Consumer-proof + hygiene**: `test/consumer/` exercises all five subpackages;
  package doc comments; `TypesHelp`/`DefaultNavTitles`/ `ResolveTypeAlias`
  documented as CLI-support helpers.
- **Docs + release**: living-doc updates, dated DESIGN-0007 amendment, absorb +
  delete the root handoff file, `major`-labeled PR → `v1.0.0`.

### Out of Scope

- **Promoting `template`, `index`, or `wiki`** — they stay `internal/` (ADR-0001
  Decisions; zero external demand per INV-0006).
- **Any CLI behavior change** — no new commands, flags, outputs, or exit codes;
  the CLI may only change import paths.
- **Any `.docz.yaml` / frontmatter schema change.**
- **The plan/phase model** (`ParsePlan`, phase grouping, `#### Tasks` scoping,
  nesting policy) — consumer-side interpretation, owned by sdk-booty-sh's
  `doczwork` on top of the `docparse` primitives (Decisions Q3: docz parses
  _facts_; consumers own _meaning_).
- **A general markdown AST/editing API** — the write surface stays status +
  checkbox + create, by design.
- **Frontmatter editing beyond status; CRLF support** (LF-only stays the
  contract).
- **Reshaping `Create`** (e.g. a pure `PlanCreate` seam) — deferred until a
  consumer asks; it promotes with its current shape.
- **`docz export --json`** (still deferred, DESIGN-0007 OQ3a).
- **Module-path change** — `v1.x` requires no `/vN` suffix (that starts at v2).

## Affected code (verified inventory)

- `pkg/doczcore/config/config.go` — viper's only home: `Load` →
  `mergeConfigFile` / `loadFromFile` / `viper.New` call sites. All structs
  already carry dual `mapstructure` + `yaml` tags. `cmd/` has **no** direct
  viper import, so `go mod tidy` drops viper + the `mapstructure` indirect after
  Phase 1.
- `internal/docwrite/` — `create.go` (imports `internal/template` for
  `Resolve`/`Render`/`FilenameSlug` — the only cross-internal edge), `status.go`
  (+ sentinels), tests, and `testdata/golden/status/<type>.{input,output}.md` —
  all move wholesale. `CreateOptions`/`CreateResult` expose only `config` types,
  primitives, and `time.Time` (verified) — no internal type leaks through
  promotion.
- `internal/toc/` — `toc.go` (walker + `GenerateToC`/`UpdateToC` + markers),
  `update.go` (`UpdateFiles`, `FileInput{Path, Content}`, `FileResult`,
  `FileError`, `UpdateReport`), `toc_test.go`, `update_test.go`,
  `golden_test.go`.
- `cmd/` repoints: `status.go` + `create.go` (docwrite), `update.go` (the only
  non-test `toc` importer — verified).
- `.golangci.yml` — **no** `internal/toc`/`internal/docwrite` path pins exist
  (verified); nothing to update, re-check after the moves.
- `test/consumer/` — separate module; extend to the full surface.
- Living docs: `CLAUDE.md` (incl. the "Cobra/Viper for CLI" conventions line),
  `CONTRIBUTING.md`, `DEVELOPMENT.md`, `README.md` if it names paths.
- `docs/design/0007-…` — dated amendment (write surface). Root
  `docz-v0.6.0-requirements.md` — deleted once absorbed here.

## Implementation Phases

Each phase builds on the previous one. A phase is complete when all its tasks
are checked off and its success criteria are met. Phases 1–4 are independent
except **Phase 4 depends on Phase 2** (the `docparse` delegation).

---

### Phase 1: viper-free `config.Load` (handoff R1)

Swap the config decode engine without moving a single symbol: yaml.v3 decoding
onto a pre-populated `DefaultConfig()`, preserving every observable behavior the
tests pin.

#### Tasks

- [ ] Replace the viper path (`mergeConfigFile`, `loadFromFile`, the `viper.New`
      call sites in `Load`) with `go.yaml.in/yaml/v3` decoding onto a
      pre-populated `DefaultConfig()` (the `yaml` tags are already on every
      struct), preserving: defaults fill, `.docz.yaml` discovery from
      `repoRoot`, explicit `configFile` override, types-replace-on-presence, and
      validation order.
- [ ] Keep the exported signature byte-identical:
      `Load(configFile, repoRoot string) (Config, error)`.
- [ ] Apply Open Question 2's resolution for unknown-key handling (default
      posture: lenient, viper-parity).
- [ ] Audit error-path wording: malformed-YAML messages may differ from viper's
      — exit codes and success paths must not; note any wording delta for the
      release notes.
- [ ] `go mod tidy` — confirm `github.com/spf13/viper` and the
      `go-viper/mapstructure` indirect leave `go.mod`/`go.sum`.
- [ ] Extend the parity baseline test to pin the **case-sensitivity delta**:
      lowercase keys behave identically; a MixedCase key is no longer matched
      (documented, not honored — viper was case-insensitive).
- [ ] Draft the release-notes lines: viper removal, case-sensitivity delta, any
      error-wording changes.

#### Success Criteria

- A scratch module importing only `pkg/doczcore/{config,document}` shows **no**
  `github.com/spf13/viper` packages in `go list -deps` (the handoff R1
  acceptance test).
- `make test` green including the parity baseline; `go test ./... -update`
  produces **zero** golden churn.
- `cmd/` tests untouched except (at most) pinned error-text updates, each
  flagged in the PR description.

---

### Phase 2: create `pkg/doczcore/docparse` (handoff R2, revised)

The canonical markdown **fact extractor**: one heading walker and one checkbox
walker for the whole module — byte-accurate lines, zero interpretation. The
plan/phase model is deliberately absent (Decisions Q3): sdk-booty-sh's
`doczwork` builds it on these primitives.

#### Tasks

- [x] Create the package — bytes-in/values-out (ADR-0001 Decision 5):
      `Heading{Level, Text, Slug, Line}`, `Headings(content []byte) []Heading`.
- [x] Port the `internal/toc` walker semantics — fence-aware, H2–H6 (H1
      excluded), inline-markdown stripping, GitHub-compatible slugs — adding
      1-based `Line`; move the `AnchorSlug` slug logic here (exported), with
      slug output **byte-identical** to `internal/toc`'s on its existing test
      corpus.
- [x] Add the checkbox primitive: `TaskItem{Text, Checked, Indent, Line}`,
      `TaskItems(content []byte) []TaskItem` — every `- [ ]` / `- [x]` list item
      (`-` or `*` bullets, marker stripped from `Text`), fence-aware (checkboxes
      inside code fences ignored), LF-only line accounting matching `docwrite`'s
      contract so `TaskItem.Line` feeds `CheckTask` directly; `Indent`
      (leading-whitespace width) exposed so consumers apply their own
      top-level/nesting policy. **No** phase grouping, no `#### Tasks` scoping —
      facts only.
- [x] Golden fixtures: template-conformant IMPL output **and** a messy
      hand-written corpus; add a write-through test that byte-splices at
      reported `TaskItem.Line` values against the raw file bytes.
- [ ] Package doc comment (stating the facts-not-interpretation contract) +
      goheader license headers.

#### Success Criteria

- Slug parity: `docparse` slugs byte-identical to `internal/toc`'s across toc's
  existing corpus (pre-work for Phase 4's zero-churn guarantee).
- `TaskItem.Line` values verified against raw bytes (a consumer will write
  through them); golden tests green; the package is **stdlib-only**.

---

### Phase 3: promote `docwrite` whole + add `CheckTask` (handoff R3)

The write side goes public as one cohesive package: status splice, checkbox
splice, and document creation — with the template machinery staying private
behind it.

#### Tasks

- [ ] `git mv internal/docwrite pkg/doczcore/docwrite` (package name `docwrite`
      preserved, `testdata/golden/status/` rides along); `create.go` keeps its
      `internal/template` import (public→internal, legal — ADR-0001 Decision 3).
- [ ] Add `CheckTask(path string, line int) error`: flips the unchecked
      top-level checkbox item on the given 1-based line to checked (`[ ]` →
      `[x]`), changing **exactly the three marker bytes**; typed,
      distinguishable sentinels for line-out-of-range / not-a-task-item /
      task-already-checked; reject CR/CRLF via the existing
      `ErrUnsupportedLineEndings`; same read-whole-file / single-splice /
      write-back approach as `SetStatus`.
- [ ] Repoint `cmd/status.go` and `cmd/create.go` to `pkg/doczcore/docwrite`.
- [ ] Golden byte-diff tests for `CheckTask`: the on-disk diff touches exactly
      one line, byte-identical elsewhere; existing `SetStatus`/`Create` tests
      pass unchanged from the new import path.
- [ ] Package doc comment stating the deliberate write surface — status +
      checkbox + create; **not** a general editor.
- [ ] Add the dated amendment to DESIGN-0007: the read-only-public stance is
      revised; the public write surface is status + checkbox + create, by design
      (per ADR-0001).

#### Success Criteria

- `grep -rn 'docz/internal/docwrite' --include='*.go'` returns zero.
- `CheckTask` golden diffs are single-line; `make test` green with zero churn on
  the existing status fixtures; CLI behavior identical.

---

### Phase 4: promote `toc` as the splice package over `docparse` (ADR-0001 OQ4a)

One public parser: `toc` keeps the marker-splice concern and delegates every
heading walk to `docparse`. Depends on Phase 2.

#### Tasks

- [ ] `git mv internal/toc pkg/doczcore/toc`; retire `ParseHeadings`, `Heading`,
      and `AnchorSlug` from the surface — all walking goes through
      `docparse.Headings`.
- [ ] Reshape the delegating surface:
      `GenerateToC(headings []docparse.Heading, minHeadings int) string`;
      `UpdateToC` delegates internally and `UpdateResult.Headings` becomes
      `[]docparse.Heading`; keep `BeginMarker`/`EndMarker` exported; the
      `UpdateFiles` family per Open Question 4's resolution.
- [ ] Repoint `cmd/update.go` (the only non-test `toc` importer).
- [ ] Move `toc_test.go`/`update_test.go`/`golden_test.go` and fixtures; adapt
      to the `docparse` types; assert generated ToC output is **byte-identical**
      on the existing corpus.
- [ ] Package doc comment + license headers.

#### Success Criteria

- Exactly one heading walker in the public API — `go doc ./pkg/doczcore/toc`
  lists no `ParseHeadings`/`Heading`/`AnchorSlug`.
- `grep -rn 'docz/internal/toc' --include='*.go'` returns zero; zero golden
  churn (ToC output byte-identical); `make test` green.

---

### Phase 5: consumer proof + public-surface hygiene

Prove the whole v1.0 surface from outside the module and make it read like a
deliberate API before it freezes.

#### Tasks

- [ ] Extend `test/consumer/` to import **all five** subpackages by public path
      and exercise: viper-free `Load → Validate → EnabledTypes → ScanDocuments`
      (incl. a custom type); `docparse.Headings` + `TaskItems` on fixture bytes;
      `SetStatus` + `CheckTask` round-trips in `t.TempDir()`; `Create` into
      `t.TempDir()` (exercising the embedded template through the private
      engine); a `toc.UpdateToC` splice.
- [ ] Spot-check and record in the PR: temporarily narrowing a promoted symbol
      breaks the consumer build; `internal/template` is **not** importable from
      the consumer module.
- [ ] `go doc` review of all five packages: doc comments present, intended
      surface only; document `TypesHelp`/`DefaultNavTitles`/ `ResolveTypeAlias`
      as CLI-support helpers in the `config` package doc (ADR-0001 Decisions).
- [ ] Re-verify `.golangci.yml` needs no path updates after the moves;
      `make lint` and the goheader license check green.

#### Success Criteria

- `make ci` green including the consumer module; every public subpackage is
  exercised by an external importer.
- The narrowing spot-check fails compilation as expected (recorded once).

---

### Phase 6: living docs, release, and the v1.0.0 tag

Land the freeze as a pinnable contract and close the loop with the waiting
consumer.

#### Tasks

- [ ] Update `CLAUDE.md` (architecture bullets: `docwrite`/`toc` new homes,
      `docparse` addition, viper gone — including the "Cobra/Viper for CLI"
      conventions line), `CONTRIBUTING.md`, `DEVELOPMENT.md`, and `README.md` as
      needed. **Historical `docs/*` records stay untouched.**
- [ ] Delete the root `docz-v0.6.0-requirements.md` handoff — its R1–R3 slate is
      fully absorbed by Phases 1–3 of this plan.
- [ ] Write the PR's `### RELEASE NOTES`: the v1.0.0 stability contract
      (five-package surface; semver covers exported identifiers under
      `pkg/doczcore/*`; `cmd/`, `internal/`, `test/`, CLI output text, and
      embedded template contents excluded; roll-forward posture — breaking
      changes ship in a major bump, ADR-0001 OQ7); viper removal + the
      case-sensitivity delta; note that v1 requires no module-path change.
- [ ] The release PR carries the **`major`** label; on merge, `pr-semver-bump` +
      goreleaser cut and publish **`v1.0.0`** (merge to `main` is human-gated).
- [ ] Post-tag: flip this IMPL → Completed; notify sdk-booty-sh — its IMPL-0002
      Phase 0/W gate now pins `v1.0.0` (not `v0.6.0`).

#### Success Criteria

- The `v1.0.0` tag exists with a successful goreleaser release; a scratch module
  can `go get github.com/donaldgifford/docz@v1.0.0` and import all five
  subpackages.
- No living doc references `internal/docwrite`/`internal/toc` or claims the
  module uses viper; the full golden suite shows zero churn (CLI
  byte-identical).

---

## File Changes

| File / path                                                   | Action | Description                                                                                           |
| ------------------------------------------------------------- | ------ | ----------------------------------------------------------------------------------------------------- |
| `pkg/doczcore/config/config.go`                               | Modify | viper → yaml.v3 decode onto `DefaultConfig()`; signature unchanged                                    |
| `go.mod` / `go.sum`                                           | Modify | drop `spf13/viper` + `mapstructure` indirect (`go mod tidy`)                                          |
| `pkg/doczcore/docparse/*`                                     | Create | `Headings`/`TaskItems` + types (facts only); golden fixtures under `testdata/`                        |
| `pkg/doczcore/docwrite/*`                                     | Move   | `git mv` from `internal/docwrite` (incl. `testdata/golden/status/`); keeps `internal/template` import |
| `pkg/doczcore/docwrite/checktask.go`                          | Create | `CheckTask` + typed sentinels + byte-diff goldens                                                     |
| `pkg/doczcore/toc/*`                                          | Move   | `git mv` from `internal/toc`; walker retired, delegates to `docparse`                                 |
| `cmd/status.go`, `cmd/create.go`, `cmd/update.go`             | Modify | repoint imports to `pkg/doczcore/{docwrite,toc}`                                                      |
| `test/consumer/*`                                             | Modify | exercise all five public subpackages                                                                  |
| `docs/design/0007-…`                                          | Modify | dated amendment: write surface = status + checkbox + create                                           |
| `CLAUDE.md`, `CONTRIBUTING.md`, `DEVELOPMENT.md`, `README.md` | Modify | repoint living-doc references; drop viper mentions                                                    |
| `docz-v0.6.0-requirements.md` (repo root)                     | Delete | absorbed by Phases 1–3                                                                                |

## Testing Plan

- [ ] **Moved tests pass unchanged except import paths** — `docwrite` status
      goldens and `toc` splice goldens prove behavior preserved;
      `go test ./... -update` yields zero churn at every phase boundary.
- [ ] **Parity baseline pins the config decode** — including the new
      case-sensitivity case (Phase 1).
- [ ] **`docparse` golden corpus** — template-conformant + messy fixtures; slug
      parity vs the old walker; `TaskItem.Line` write-through accuracy.
- [ ] **`CheckTask` byte-diff goldens** — exactly one line changes.
- [ ] **CLI regression suite stays green** — `cmd/` tests (serial `Runner` +
      `bytes.Buffer`) untouched except imports and any flagged error-text pins.
- [ ] **Consumer module exercises all five subpackages** as an external importer
      (Phase 5).
- [ ] **`make ci` gates every phase** — lint, license-check, build, full suite,
      consumer module.

## Dependencies

- **None external.** Self-contained single-repo work; all callers are in-repo
  and compiler-verified. Phase 4 depends on Phase 2 (`docparse`).
- **Enables (does not depend on):** sdk-booty-sh IMPL-0002 Phase 0/W — its
  work-source harness waits on the `v1.0.0` tag (its policy: no `replace`
  directives). docz-api continues on `v0.5.0` unaffected (its surface is
  untouched; the consolidation is additive for it).
- **Tooling:** existing `pr-semver-bump` (label `major`) + goreleaser.

## Open Questions

> Each question is numbered; option `a` is my recommendation, later letters are
> alternatives, and the final letter is a free-form "other" for you to fill in.
> Answer inline (e.g. `1a`, `3c`) and I'll lock them into a Decisions table
> before implementation starts.
>
> **Update 2026-07-03:** all four questions are resolved (`1a 2a 3d 4a`) — see
> the [Decisions](#decisions) table. The menus below are kept for the record.

### 1. PR strategy — per-phase PRs or one atomic PR?

> **Resolved: (a).**

- **a. (Recommended)** **Sequential per-phase PRs to `main`** — P1 (viper), P2
  (docparse), P3 (docwrite), P4 (toc), P5+P6 (consumer/hygiene/release) — each
  labeled `dont-release`, with the final PR carrying `major`. Every merge leaves
  `main` green and untagged; diffs stay reviewable; each phase reverts
  independently. Sequential (each merges before the next opens), not stacked —
  avoiding the stacked-PR auto-close gotcha.
- b. **One atomic PR** with the `major` label (IMPL-0013 style) — a single
  revert unit, but a very large diff: dependency removal + a new package + two
  package moves + a reshape in one review.
- c. **Two PRs**: the core slate (Phases 1–4, `dont-release`) + the
  hygiene/release PR (Phases 5–6, `major`).
- d. Other.

### 2. Unknown-key handling in the yaml-only `Load`?

> **Resolved: (a).**

- **a. (Recommended)** **Lenient** — ignore unknown keys (yaml.v3's default),
  matching viper's behavior exactly: a typo'd key silently falls back to
  defaults, as today. Zero behavior change; keeps "the CLI works exactly as it
  does today" literal.
- b. **Strict** (`KnownFields(true)`) — fail fast on unknown keys. Catches
  `.docz.yaml` typos early, but it is a behavior change (configs that load today
  could start erroring) and needs its own release-note callout and a test sweep.
- c. Other.

### 3. `ParsePlan` phase semantics — which `###` headings are phases?

> **Resolved: (d) — the question dissolves: `ParsePlan` is dropped from
> `docparse`.** docz parses _facts_ (headings, checkbox items — with
> byte-accurate lines that feed `CheckTask`); the plan/phase _interpretation_ is
> loop-harness policy and moves to sdk-booty-sh's `doczwork`, built on the
> primitives. Every option below was docz picking a workflow semantic it has no
> use for itself — the ambiguity was the signal that the semantics belong
> downstream. Handoff R2 is revised accordingly: `docparse` ships `Headings` +
> `TaskItems` instead of `ParsePlan`.

- **a. (Recommended)** **Handoff-literal**: every `###` heading opens a phase
  (bold names tolerated); phases with no checkbox tasks are included with an
  empty `Tasks` slice; consumers filter. Simplest contract, no magic English
  heading names, and it matches the messy-corpus leniency intent — a
  hand-written doc with tasks under an unconventional heading still parses.
- b. **Scope to the `## Implementation Phases` section** when that heading
  exists, else fall back to (a). Cleaner output for template-conformant docs (no
  "Observation 1"-style pseudo-phases), but a two-mode contract keyed to an
  English heading name that custom templates may not use.
- c. **Task-bearing headings only** — a `###` heading becomes a phase only if it
  contains at least one checkbox. Smallest output, but silently drops genuinely
  empty phases (e.g. a planned phase whose tasks aren't written yet), which a
  harness may want to see.
- d. Other.

### 4. Does the `UpdateFiles` family go public with `toc`?

> **Resolved: (a).**

- **a. (Recommended)** **Yes — promote the package whole**: `UpdateFiles`,
  `FileInput{Path, Content}`, `FileResult`, `FileError`, `UpdateReport` all go
  public. Consistent with the ADR-0001 whole-package rule; the write-back
  mirrors `docwrite`'s path-writer precedent; the report structs evolve
  additively.
- b. **No — keep the file-walking layer CLI-side** (a small helper in `cmd/` or
  `internal/`): public `toc` is the pure `GenerateToC`/`UpdateToC` + markers. A
  purer bytes-in surface, but it re-introduces exactly the split-package layout
  ADR-0001 Decision 1 rejects.
- c. Other.

## Decisions

Resolved by user review on 2026-07-03.

| #   | Question                    | Choice                            | Notes                                                                                                                 |
| --- | --------------------------- | --------------------------------- | --------------------------------------------------------------------------------------------------------------------- |
| 1   | PR strategy                 | (a) sequential per-phase PRs      | each `dont-release`; the final release PR carries `major`; sequential, not stacked                                    |
| 2   | Unknown-key handling        | (a) lenient                       | viper parity — zero behavior change                                                                                   |
| 3   | `ParsePlan` phase semantics | (d) drop `ParsePlan` — facts only | `docparse` ships `Headings` + `TaskItems`; the Plan/Phase model moves to sdk-booty-sh `doczwork` (handoff R2 revised) |
| 4   | `UpdateFiles` publicity     | (a) whole package                 | consistent with the ADR-0001 whole-package rule                                                                       |

## References

- **ADR-0001** — the decision this plan implements (promotion rule, five
  packages, API design principles, single-release sequencing, roll-forward
  compat posture; all seven OQs resolved 2026-07-03).
- **INV-0006** — the per-package demand audit (cmd vs docz-api / docz-site /
  sdk-booty-sh) grounding the scope.
- **`docz-v0.6.0-requirements.md`** — the sdk-booty-sh handoff (R1 viper-free
  Load, R2 docparse, R3 docwrite + CheckTask), absorbed by Phases 1–3 and
  deleted in Phase 6. **R2 is revised by Decisions Q3**: docz ships the
  `Headings`/`TaskItems` fact primitives; the `ParsePlan` plan model moves to
  the consumer (the handoff itself notes its shapes are "a consumer's sketch,
  not a contract").
- **DESIGN-0007** — receives the dated write-surface amendment (Phase 3).
- **DESIGN-0008** — docz-api (unaffected consumer; its R2 surface is frozen, not
  changed).
- **IMPL-0013** — prior art: compiler-verified, behavior-preserving package
  relocation + the `test/consumer/` external-module proof.
- **sdk-booty-sh IMPL-0002** — Phase 0/W downstream gate; flips against the
  `v1.0.0` tag. Its `doczwork` package additionally takes ownership of the
  Plan/Phase model over the `docparse` primitives (Decisions Q3) — its Phase W
  scope grows by that grouping code.
