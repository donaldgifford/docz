---
id: IMPL-0013
title: "Promote parsing core to pkg/doczcore"
status: Draft
author: Donald Gifford
created: 2026-06-30
---
<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL 0013: Promote parsing core to pkg/doczcore

**Status:** Draft
**Author:** Donald Gifford
**Date:** 2026-06-30

<!--toc:start-->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Affected code (verified inventory)](#affected-code-verified-inventory)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: Move the packages & repoint imports (behavior-preserving move)](#phase-1-move-the-packages--repoint-imports-behavior-preserving-move)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2: Public-surface hygiene & minimality](#phase-2-public-surface-hygiene--minimality)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 3: Consumer import smoke test (external-caller proof)](#phase-3-consumer-import-smoke-test-external-caller-proof)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 4: Release, docs, and the v0.5.0 tag](#phase-4-release-docs-and-the-v050-tag)
    - [Tasks](#tasks-3)
    - [Success Criteria](#success-criteria-3)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Dependencies](#dependencies)
- [Open Questions](#open-questions)
  - [1. How do the CLI-only write-side symbols relate to the public surface?](#1-how-do-the-cli-only-write-side-symbols-relate-to-the-public-surface)
  - [2. Where does the consumer import smoke test live? (DESIGN-0007 OQ8)](#2-where-does-the-consumer-import-smoke-test-live-design-0007-oq8)
  - [3. Confirm docz export --json stays deferred? (DESIGN-0007 OQ3)](#3-confirm-docz-export---json-stays-deferred-design-0007-oq3)
  - [4. When is v0.5.0 cut relative to the PR?](#4-when-is-v050-cut-relative-to-the-pr)
  - [5. Single atomic PR or a staged move?](#5-single-atomic-pr-or-a-staged-move)
- [Decisions](#decisions)
- [References](#references)
<!--toc:end-->

## Objective

Promote docz's config and frontmatter parsing out of `internal/` into an
importable public package so an external module (docz-api, DESIGN-0008) consumes
the *same* code the CLI runs, with zero parser drift. Concretely: move
`internal/config` → `pkg/doczcore/config` and `internal/document` →
`pkg/doczcore/document`, repoint every in-repo import, prove an external caller
can import the surface, and cut the `v0.5.0` tag that makes it pinnable.

The change is **behavior-preserving for the CLI**: no command, flag, output, or
`.docz.yaml`/frontmatter schema change; golden files must not move. It is a code
relocation plus a new, semver-governed public import surface.

**Implements:** DESIGN-0007 (docz changes to support docz-api and docz-site).
Downstream consumer: **DESIGN-0008** (docz-api, via Requirements R1–R7).
Upstream: **INV-0005** Decision 7 (shared library, not a re-implemented parser).

**Locked decisions taken as given** (DESIGN-0007 Decisions table, 2026-06-30):
- **OQ1a** — two subpackages `pkg/doczcore/config` + `pkg/doczcore/document`
  (preserve package names `config`/`document` and the typed `Status`/`DocType`
  boundary).
- **OQ2a** — move wholesale + update all imports; **no permanent re-export
  shim** survives the release (a transitional shim is allowed only inside a
  single PR if the sweep can't land atomically).
- **OQ4a** — same module, new path, tag a normal minor **`v0.5.0`** (latest tag
  today is `v0.4.1`); stay in `v0.x`.

This plan's own implementation questions — write-side surface, consumer-test
location, `docz export`, release mechanics, and PR strategy — were resolved on
2026-06-30; see the [Decisions](#decisions) table. The
[Open Questions](#open-questions) menu is kept for the record.

## Scope

### In Scope

- Physically moving `internal/config` and `internal/document` to
  `pkg/doczcore/config` and `pkg/doczcore/document` (files, tests, and
  `internal/document/testdata/golden/status/`).
- Repointing every import: `cmd/*`, `internal/template`, `internal/index`,
  `internal/toc`, `internal/wiki`, plus the `document → config` cross-package
  import and all in-package `_test.go` imports.
- Updating the `.golangci.yml` goconst exclusion path pinned to
  `internal/config/doctype.go`.
- Keeping the CLI-only **write-side** symbols (`SetStatus` + sentinels;
  `Create`/`CreateOptions`/`CreateResult`) out of the public surface by
  relocating them to a new `internal/docwrite` package (Decision 1).
- A consumer import smoke test proving an external module can import and use the
  public surface (incl. a custom type).
- Updating living docs (CLAUDE.md, CONTRIBUTING.md, DEVELOPMENT.md, README if
  needed) that reference the old internal paths.
- Cutting and releasing `v0.5.0`.

### Out of Scope

- **`docz export --json`** — deferred (DESIGN-0007 OQ3a); Open Question 3 confirms
  it is not in this plan.
- **Any `.docz.yaml` / frontmatter schema change** — visibility change only.
- **Reshaping signatures or structs** — the move is import-path-only; the
  promoted symbols keep their exact current shapes (that is what freezes as API).
- **Exposing `internal/template` / `index` / `toc` / `wiki`** — CLI output
  concerns stay internal.
- **docz-api itself** (DESIGN-0008) and **docz-site** (DESIGN-0009).

## Affected code (verified inventory)

Moved packages (all files ride along in a wholesale `git mv`):

- `internal/config/` → `pkg/doczcore/config/`: `config.go`, `doctype.go`,
  `constants.go` + `config_test.go`, `doctype_test.go`,
  `parity_baseline_test.go`.
- `internal/document/` → `pkg/doczcore/document/`: `document.go`, `scan.go`,
  `status.go`, `create.go` + their `_test.go` files + `testdata/` (golden status
  fixtures). **Note:** `status.go` (`SetStatus`) and `create.go` (`Create`) are
  CLI-only write-side code and are relocated to `internal/docwrite`, **not**
  promoted (Decision 1); only the read-side (`document.go`, `scan.go`) goes
  public.

Import sites to repoint (~37 references across 37 files, all compiler-verified):

- `cmd/`: `create.go`, `init.go`, `list.go`, `root.go`, `runner.go`, `status.go`,
  `template.go`, `update.go`, `wiki.go` (+ every matching `_test.go`, plus
  `config_test.go`, `inv0003_test.go`, `update_custom_test.go`).
- `internal/`: `template/{embed,template}.go` (+ tests), `index/index.go`
  (+ test), `toc/update.go`, `wiki/{mkdocs,titles,wiki}.go`.
- In-package cross-refs: `document/{create,document,status}.go` and the
  `document`/`config` test files import `.../config`.

Non-Go references to update:

- `.golangci.yml:266` — `path: internal/config/doctype\.go` (goconst exclusion)
  → `pkg/doczcore/config/doctype\.go`. **If missed, goconst re-fires on the moved
  file and CI fails.**
- Living docs: `CLAUDE.md` (Architecture section), `CONTRIBUTING.md`,
  `DEVELOPMENT.md`, `README.md` if it names the paths. **Do not** rewrite
  historical `docs/impl/*` or `docs/design/*` — those are point-in-time records.

## Implementation Phases

Each phase builds on the previous one. A phase is complete when all its tasks are
checked off and its success criteria are met.

---

### Phase 1: Move the packages & repoint imports (behavior-preserving move)

The mechanical core: relocate both packages and make the repo build again. No
logic changes — only on-disk location and import strings. This is the phase that
must prove "byte-for-byte identical behavior."

#### Tasks

- [x] `git mv internal/config pkg/doczcore/config` (preserve package name
      `config`).
- [x] `git mv internal/document pkg/doczcore/document` (preserve package name
      `document`, including `testdata/`), then relocate the write-side —
      `status.go` (`SetStatus` + `ErrStatusFieldMissing` /
      `ErrUnsupportedLineEndings`) and `create.go`
      (`Create`/`CreateOptions`/`CreateResult`) — into a new `internal/docwrite`
      package that imports the public read-side (Decision 1).
- [x] Repoint the `document → config` cross-package import to
      `github.com/donaldgifford/docz/pkg/doczcore/config`.
- [x] Repoint all `cmd/*` and `internal/{template,index,toc,wiki}` imports to the
      new `pkg/doczcore/...` paths (goimports/gci ordering preserved).
- [x] Repoint `cmd/status.go` and `cmd/create.go` to `internal/docwrite` for
      `SetStatus` / `Create` (they still import `pkg/doczcore/{config,document}`
      for the read-side types).
- [x] Repoint every in-package `_test.go` import (config/document external test
      packages, `parity_baseline_test.go`).
- [x] Update `.golangci.yml` goconst exclusion path from
      `internal/config/doctype\.go` to `pkg/doczcore/config/doctype\.go`.
- [x] `make fmt` + `go build ./...` until the tree compiles clean.

#### Success Criteria

- `go build ./...` succeeds; `grep -rn 'internal/\(config\|document\)'
  --include='*.go'` returns **zero** matches.
- `make test` is green and `go test ./... -update` produces **zero** golden diff
  (proves behavior unchanged).
- `make lint` is green — in particular goconst is still excluded on the moved
  `doctype.go` (confirms the `.golangci.yml` path was updated).
- No `cmd/` test changed except its import line; the CLI is behavior-identical.

---

### Phase 2: Public-surface hygiene & minimality

Make the newly public packages read like a deliberate API, and resolve what the
CLI-only write-side symbols do (Open Question 1). Every exported symbol here is a
future semver constraint (DESIGN-0007), so the surface is trimmed on purpose.

#### Tasks

- [x] Confirm the write-side (`SetStatus`, `Create`) lives only in
      `internal/docwrite` and is absent from the public `pkg/doczcore/document`
      surface (Decision 1).
- [x] Add/verify package doc comments on `pkg/doczcore/config` and
      `pkg/doczcore/document` so `go doc` presents the intended surface.
- [x] Confirm the intended read-side surface is exported and unchanged: `Load`,
      `DefaultConfig`, `Validate`, `EnabledTypes`, `TypeDir`, `ValidateType`,
      `DocTypeNames`, `AllDocTypes`, `LookupDocType`, `ErrUnknownType`; `Config`,
      `TypeConfig`, `DocType`, `Status`; `ScanDocuments`, `LoadFrontmatter`,
      `ParseFrontmatter`, `IsDoczFile`, `Frontmatter`, `DocEntry`,
      `ErrNoFrontmatter`.
- [x] Verify the `goheader` license header is present on every moved (and any
      relocated) file.

#### Success Criteria

- `go doc ./pkg/doczcore/config` and `./pkg/doczcore/document` list exactly the
  intended public surface (write-side handled per Open Question 1).
- No unintended CLI-private symbol is exported from the public packages.
- `make ci` (lint + test + build + license-check) is green.

---

### Phase 3: Consumer import smoke test (external-caller proof)

Prove an external module can import the surface by its public path and run the
real ingest sequence — the test that catches an accidental visibility narrowing
or a broken `document → config` typed-field link.

#### Tasks

- [x] Add the consumer test per Open Question 2 (separate module under
      `test/consumer/` with its own `go.mod`, recommended).
- [x] Import `pkg/doczcore/config` + `pkg/doczcore/document` by their public
      paths; write a `.docz.yaml` + a couple of docs into `t.TempDir()`.
- [x] Run `Load → Validate → EnabledTypes → ScanDocuments`; assert the
      `DocEntry` set, including a **custom type** (locks in the type-agnostic
      contract, DESIGN-0006 / INV-0005 Obs 1).
- [x] Call `ParseFrontmatter` on raw bytes (the no-checkout path) and assert the
      parsed `Frontmatter`.
- [x] Wire the consumer module into `make ci` (a dedicated `go test` invocation,
      since a separate module is outside root `./...`) and CI; add the license
      header to its files.

#### Success Criteria

- The consumer test passes as an external importer (separate module), asserting
  the custom-type `DocEntry` set and the bytes-in `ParseFrontmatter` path.
- `make ci` runs the consumer module and is green.
- Temporarily narrowing a promoted symbol's visibility makes this test fail to
  compile (spot-checked once).

---

### Phase 4: Release, docs, and the v0.5.0 tag

Land the move as a public, pinnable surface and bring the living docs in line.

#### Tasks

- [ ] Update `CLAUDE.md` Architecture bullets: `internal/config` →
      `pkg/doczcore/config`, `internal/document` → `pkg/doczcore/document`, and
      the `internal/index` "scanning lives in `internal/document`" note →
      `pkg/doczcore/document`.
- [ ] Update `CONTRIBUTING.md` and `DEVELOPMENT.md` references to the moved
      paths (`doctype.go`, `doctype_test.go`, `document.CreateOptions.Type`,
      package sections). **Leave historical `docs/impl/*` and `docs/design/*`
      untouched.**
- [ ] Add release notes: `pkg/doczcore` is now a supported, semver-governed
      public surface; `internal/config` / `internal/document` import paths are
      gone (invisible outside the repo).
- [ ] Open the promotion PR with the **`minor`** release label (satisfies the
      required-label gate); on merge, the release automation cuts and pushes
      **`v0.5.0`** — no manual `git tag`. Confirm the goreleaser release runs.
- [ ] Flip DESIGN-0007 `status: Draft → Implemented` (frontmatter + body) and
      record OQ3/OQ5/OQ8 resolutions in its Decisions table once tagged.

#### Success Criteria

- The merged PR's `minor` label produces the `v0.5.0` tag and a successful
  goreleaser release build (no manual tagging).
- No living doc (CLAUDE.md, CONTRIBUTING.md, DEVELOPMENT.md, README) references
  `internal/config` or `internal/document`; historical docs are unchanged.
- A scratch module can `require github.com/donaldgifford/docz v0.5.0` and import
  `pkg/doczcore/config` + `document` (validated by the Phase 3 consumer test
  against the tag, or a manual `go get` spot-check).
- DESIGN-0007 is marked Implemented with its remaining OQs resolved.

---

## File Changes

| File / path | Action | Description |
|------|--------|-------------|
| `pkg/doczcore/config/*` | Move | `git mv` from `internal/config` (name `config` preserved) |
| `pkg/doczcore/document/*` | Move | `git mv` from `internal/document` (name `document`; incl. `testdata/`) |
| `cmd/*.go` (+ tests) | Modify | Repoint imports to `pkg/doczcore/...` (and `internal/docwrite` in `status.go`/`create.go`) |
| `internal/{template,index,toc,wiki}/*.go` | Modify | Repoint imports to `pkg/doczcore/...` |
| `.golangci.yml` | Modify | goconst exclusion path → `pkg/doczcore/config/doctype\.go` |
| `internal/docwrite/*` | Create | Relocated write-side (`SetStatus`, `Create`) — kept internal (Decision 1) |
| `test/consumer/{go.mod,consumer_test.go}` | Create | External-import smoke test (Open Question 2) |
| `Makefile` | Modify | Add the consumer-module test invocation to `ci` |
| `CLAUDE.md`, `CONTRIBUTING.md`, `DEVELOPMENT.md`, `README.md` | Modify | Repoint living-doc references |

## Testing Plan

- [x] **Moved tests pass unchanged except import paths** — `config`/`document`
      table-driven tests (`t.Parallel()`, golden `status/` fixtures) prove
      behavior is preserved; `go test ./... -update` yields zero churn.
- [x] **CLI regression suite stays green** — `cmd/` tests (serial, `Runner` +
      `bytes.Buffer`, `RepoRoot: t.TempDir()`) pass untouched except imports.
- [x] **Consumer import smoke test** (Phase 3) — external module runs
      `Load→Validate→EnabledTypes→ScanDocuments` + `ParseFrontmatter`, asserts a
      custom type.
- [ ] **`make ci` gates the move** — lint (`golangci-lint` + `golines`),
      license-check, build, full suite, and the consumer module.

## Dependencies

- **None external.** This is a self-contained, single-repo relocation; all
  callers (`cmd/` + four `internal/` packages) are in-repo and compiler-verified.
- **Enables (does not depend on):** DESIGN-0008 docz-api, which pins the
  `v0.5.0` tag this plan cuts. docz-api can prototype against a local `replace`
  before the tag exists (DESIGN-0008 R6 / OQ2), so this plan is a prerequisite
  for docz-api's *release*, not for starting it.
- **Tooling:** `goreleaser` (existing `.goreleaser.yml`) for the `v0.5.0`
  release build.

## Open Questions

> **Resolved 2026-06-30** — see the [Decisions](#decisions) table for the chosen
> option per question; the menu below is kept for the record.

Each question is numbered; option `a` is my recommendation, later letters are
alternatives, and **Other** is free-form. These are implementation choices; the
three locked DESIGN-0007 decisions (OQ1a/OQ2a/OQ4a) are not reopened.

### 1. How do the CLI-only write-side symbols relate to the public surface?

A pure wholesale `git mv` of `internal/document` also promotes `SetStatus`
(`status.go`) **and** `Create`/`CreateOptions`/`CreateResult` (`create.go`) into
the public package — but both are CLI-only *writers* (the API ingests read-only,
DESIGN-0008 Decision 4/5). DESIGN-0007's minimal-surface Goal and its OQ5
recommend keeping `SetStatus` internal; `Create` is the same case and the design
sketch did not call it out. Every public symbol is a permanent semver constraint,
so this matters.

- **a. (Recommended)** Keep the write-side **internal**. Move `config` wholesale;
  move `document`'s read-side (`document.go`, `scan.go`, their tests, `testdata/`)
  to `pkg/doczcore/document`; relocate `status.go` + `create.go` into a new
  `internal/docwrite` package (distinct name to avoid a double-`document` import
  alias) that imports the public read-side. `cmd/status.go` / `cmd/create.go`
  repoint to `internal/docwrite`. Honors the minimal-surface Goal + OQ5; small,
  compiler-verified extra churn.
- b. **Pure wholesale** `git mv` of both packages, `status.go`/`create.go`
  included; accept `SetStatus` + `Create` as public, semver-governed API. Lowest
  churn and matches the migration sketch literally; fine while in `v0.x`, revisit
  before `v1.0.0`. (Resolves OQ5 as "expose it.")
- c. **Split the difference** — keep only `SetStatus` internal (per OQ5's letter)
  but let `Create` be public, since document creation is arguably closer to core
  doc handling than a status byte-patch.
- Other.

### 2. Where does the consumer import smoke test live? (DESIGN-0007 OQ8)

- **a. (Recommended)** A separate module under `test/consumer/` with its own
  `go.mod`, importing `pkg/doczcore/...` by public path — the truest proof an
  external module can import and scan. Wired into `make ci` as its own `go test`
  run.
- b. An in-repo peer-package test — simpler CI wiring, slightly weaker "external"
  guarantee (it lives in the same module).
- c. Both — in-repo test for fast local feedback plus the separate module in CI.
- Other.

### 3. Confirm `docz export --json` stays deferred? (DESIGN-0007 OQ3)

- **a. (Recommended)** Defer entirely — not in this IMPL. Ship the promotion +
  tag; add `docz export` only when a concrete non-Go consumer appears (it is a
  new golden JSON surface with its own compatibility cost). docz-api does **not**
  need it (DESIGN-0008 R8).
- b. Include a minimal `docz export --json` in this plan as a bonus increment
  (adds a Phase 5, a new CLI command, and a golden manifest fixture).
- Other.

### 4. When is `v0.5.0` cut relative to the PR?

- **a. (Recommended)** Land the whole change (move + smoke test + doc updates) in
  one PR to `main`, then cut `v0.5.0` immediately after merge so the public
  surface is real and pinnable and DESIGN-0007 can flip to Implemented.
- b. Merge the move but **delay the tag** until docz-api actually needs a
  published version, to avoid advertising a public surface before its first
  consumer exists (docz-api prototypes against a local `replace` until then).
- c. Batch `v0.5.0` with other pending `main` changes into one release.
- Other.

### 5. Single atomic PR or a staged move?

- **a. (Recommended)** One atomic PR — `git mv` + the import sweep +
  `.golangci.yml` + docs + consumer test, **no shim**. The sweep is mechanical
  and compiler-verified, and the PR reverts cleanly as a unit (DESIGN-0007's
  rollback story).
- b. Staged with a **transitional** shim inside a single PR (re-export at the old
  `internal/` paths, migrate callers, delete the shim before the tag) — only if
  the sweep proves too large to land at once. The shim must not survive the
  release (OQ2a).
- Other.

## Decisions

Resolved by user review on 2026-06-30.

| #   | Question                   | Choice                                                     | Notes                                                                                                                     |
| --- | -------------------------- | --------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------ |
| 1   | Write-side surface         | (a) keep internal via a new `internal/docwrite` package   | Honors DESIGN-0007's minimal-surface Goal + OQ5; only the read-side (`document.go`, `scan.go`) is promoted to `pkg/doczcore/document` |
| 2   | Consumer test location     | (a) separate `test/consumer/` module                      | Truest external-import proof; wired into `make ci` as its own `go test` run                                              |
| 3   | `docz export --json`       | (a) deferred                                              | Not in this plan; docz-api does not need it (DESIGN-0008 R8). Revisit only when a concrete non-Go consumer appears        |
| 4   | `v0.5.0` release mechanism | Label-driven — no manual tag                              | The promotion PR carries the **`minor`** label; on merge the release automation cuts `v0.5.0` and runs goreleaser         |
| 5   | PR strategy                | (a) single atomic PR, no shim                             | Mechanical, compiler-verified sweep; reverts cleanly as one unit                                                         |

## References

- **DESIGN-0007** — docz changes to support docz-api and docz-site (this plan's
  parent; the promotion design, locked OQ1a/OQ2a/OQ4a, the semver obligation, and
  the exported-shape contract).
- **DESIGN-0008** — docz-api (the downstream consumer; Requirements R1–R7 and the
  `v0.5.0` tag this plan cuts).
- **INV-0005** — Decision 7 (shared library over a re-implemented parser) that
  motivates the promotion.
- **DESIGN-0006** — Custom Document Type Support (the type-agnostic resolution the
  consumer smoke test asserts).
- **IMPL-0008** — the earlier `internal/index` → `internal/document` move; prior
  art for a compiler-verified, behavior-preserving package relocation in this
  repo.
