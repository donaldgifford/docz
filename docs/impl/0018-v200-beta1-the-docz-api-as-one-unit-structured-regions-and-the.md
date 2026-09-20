---
id: IMPL-0018
title: "v2.0.0-beta.1 — the docz API as one unit, structured regions, and the cmd swap"
status: In Progress
author: Donald Gifford
created: 2026-09-19
---

<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL-0018: v2.0.0-beta.1 — the docz API as one unit, structured regions, and the cmd swap

<!--toc:start-->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 0: Module path, parity goldens, and release mechanics](#phase-0-module-path-parity-goldens-and-release-mechanics)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 1: The type layer — regions, validate, kinds, five type packages, docwrite byte cores, marked templates](#phase-1-the-type-layer--regions-validate-kinds-five-type-packages-docwrite-byte-cores-marked-templates)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 2: Promotions — doctemplate, index, pkg/wiki; schema resolution; internal/ emptied](#phase-2-promotions--doctemplate-index-pkgwiki-schema-resolution-internal-emptied)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 3: The repository core — pkg/doczcore/repo with context, Hooks, Validate, InsertRegions](#phase-3-the-repository-core--pkgdoczcorerepo-with-context-hooks-validate-insertregions)
    - [Tasks](#tasks-3)
    - [Success Criteria](#success-criteria-3)
  - [Phase 4: Catalogue — ADR-0003 plan removal](#phase-4-catalogue--adr-0003-plan-removal)
    - [Tasks](#tasks-4)
    - [Success Criteria](#success-criteria-4)
  - [Phase 5: The cmd swap, docz validate, corpus migration, and v2.0.0-beta.1](#phase-5-the-cmd-swap-docz-validate-corpus-migration-and-v200-beta1)
    - [Tasks](#tasks-5)
    - [Success Criteria](#success-criteria-5)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Dependencies](#dependencies)
- [Open Questions](#open-questions)
  - [1. PR granularity](#1-pr-granularity)
  - [2. Where the v1.2.2 binary for the golden capture comes from](#2-where-the-v122-binary-for-the-golden-capture-comes-from)
  - [3. Shape of the parity driver](#3-shape-of-the-parity-driver)
  - [4. Pre-release build trigger](#4-pre-release-build-trigger)
  - [5. docz task list <impl-id>](#5-docz-task-list-impl-id)
  - [6. Placement of docz's own corpus migration](#6-placement-of-doczs-own-corpus-migration)
  - [7. docz validate in this repo's CI](#7-docz-validate-in-this-repos-ci)
  - [8. The plan delta in the parity suite](#8-the-plan-delta-in-the-parity-suite)
  - [9. Marking this document by hand now](#9-marking-this-document-by-hand-now)
  - [10. Which documents get the per-type tier in docz validate](#10-which-documents-get-the-per-type-tier-in-docz-validate)
- [Decisions](#decisions)
- [References](#references)
<!--toc:end-->

## Objective

Build the whole docz API under the `github.com/donaldgifford/docz/v2`
module path, then move the CLI onto it and prove the move by byte-for-byte
parity with v1.2.2. The API is step one; the CLI's migration is step two
and the validation of step one. Six phases, one per rollout step 0–5 of
DESIGN-0014, each a `dont-release` PR; the last is tagged `v2.0.0-beta.1`
by hand. Nothing in this document cuts v2.0.0 (ADR-0002 Decision 6): that
milestone is the docz-api and UI consolidation, each its own design, after
the CLI.

Every built-in type is a structured type (ADR-0002 Decision 4 as amended
2026-09-19): five type packages, one per built-in, each reading every
section of its template into a typed `Doc` over the shared `kinds`
readers, so a consumer reads any docz document as a typed value with no
ambiguity about what it holds. Unstructured markdown is not a type; it
reaches a consumer through the `api:` block's additional docs
(DESIGN-0011), which is the escape hatch that lets the types stay rigid.
A document without markers still parses, by inference from its headings,
with `Doc.Inferred` set and one `region.inferred` warning; `docz validate
--fix` writes the markers inference found (amended 2026-09-20).

**Implements:** DESIGN-0014 (all 12 open questions resolved 2026-09-19) and
DESIGN-0015 (all 9 resolved 2026-09-19) as one unit (DESIGN-0014 Open
Question 11), under ADR-0002 (Decisions 1–7, R1–R8) and ADR-0003 (plan
removal on the v2 line).

## Scope

### In Scope

- Phase 0: module path `/v2`, the parity suite with goldens captured from
  the v1.2.2 binary, and the pre-release tag mechanics.
- Phase 1: `docparse.Markers`/`Regions`/`ListItems`/`Tables`; the
  `validate` package with the forty-one-kind catalogue; `kinds`; the five
  type packages `impl`, `rfc`, `adr`, `design`, and `investigation`, each
  with `Parse` and `Validate`; `docwrite` byte cores, `SetTaskState`,
  `NextNumber`, `Render`; markers around every section and embedded schema
  skeletons for every built-in template; `document.Frontmatter.Schema`.
- Phase 2: `internal/template` → `pkg/doczcore/doctemplate` (with schema
  resolution and `DefaultConfigYAML`), `internal/index` → `pkg/doczcore/index`
  (with `Splice` and `Scaffold`), `internal/wiki` → `pkg/wiki` (with `Init`
  and `UpdateNav`); `internal/` emptied.
- Phase 3: `pkg/doczcore/repo` with context, `Hooks`, typed errors,
  `Validate`, `InsertRegions`, and `ExportTemplate` scaffolding for custom
  types.
- Phase 4: ADR-0003 — `plan` removed from the built-in registry, templates,
  goldens, and docs.
- Phase 5: `cmd/` re-pointed at the API with its tests unchanged;
  `docz validate` with `--fix`; docz's own `docs/` migrated;
  parity in `make ci`; ADR-0001 amendment; living docs; docz issue #97
  closed; the `v2.0.0-beta.1` tag.
- The external-consumer proof (`test/consumer`) extended to every `pkg/`
  package as each lands.

### Out of Scope

- Cutting v2.0.0. Every PR here is `dont-release`; `major` is reserved for
  the consolidated-repo milestone (ADR-0002 Decision 6). Further betas after
  `beta.1` follow the same hand-tagged path as the API settles.
- docz-api, docz-site, tempy, and sdk-booty-sh. They pin v1 and are
  unaffected; what they rely on is recorded in DESIGN-0014 §5 for awareness
  only. No downstream issues.
- The claude-skills docz plugin (amended 2026-09-20). It serves v1 users,
  and bundling marked templates, five types, or a `docz validate` step
  before v2 ships would break them, so the plugin is not touched and no
  claude-skills issue is filed here; it updates as a follow-up once
  `v2.0.0-beta.1` is out. Until then its `docz:create` fallback keeps
  writing unmarked v1 documents, which the inference path reads (DESIGN-0014
  §2.9).
- Moving docz-api into this repo (`cmd/docz-api`) and the UI; one chart.
- IMPL-0017 (`updated:` field). It retargets from v1.3.0 to the v2 line
  after this unit; no work here.
- A `v1` maintenance branch. It is cut from `v1.2.2` only on demand.
- Any change to the frozen v1.0 surface of `config`, `document`, `docparse`,
  `docwrite`, and `toc` beyond additions (ADR-0001 Decision 7 still holds
  through the betas; the freeze is lifted for shape changes only at v2.0.0).
- `docz task list <impl-id>` (Open Question 5: deferred to a later beta).
- A type package for custom types. A custom type on its own schema gets
  the generic tier and the `kinds` readers; one that declares
  `schema: <built-in>` gets that built-in's package (ADR-0002 R7).
- Generics in the type layer (ADR-0002 Open Question 5: no).
- A CommonMark AST (DESIGN-0015 Open Question 7: hand-rolled walker).
- Hooks for `pkg/wiki` (DESIGN-0014 Open Question 12: none).

## Implementation Phases

Each phase builds on the previous one. A phase is complete when all its
tasks are checked off and its success criteria are met. One phase is one
`dont-release` PR merged in order, Phase 1's landing as per-package commits
(Open Question 1); Phases 0–4 leave the
CLI on its current code paths, so a `main` build at any point behaves like
v1.2.2 for CLI users while carrying the new packages for library consumers.
Every phase ends with `make fmt`, `make lint`, and `make ci` green, a
go-review pass over the phase's diff, and — from Phase 1 on — `make parity`
green against `build/bin/docz` under only the permitted deltas. A
go-architect pass precedes Phase 1 (`validate`, `kinds`, and the five type
packages) and Phase 3 (`repo`), the two phases that create packages with
no v1 ancestor. Every
new package's doc comment opens with the sentence "EXPERIMENTAL until
v2.0.0: the surface may change between betas", removed at v2.0.0.

---

### Phase 0: Module path, parity goldens, and release mechanics

The three things every later phase depends on: the `/v2` import path that
all new code is written against, the v1.2.2 goldens that must be captured
before any behaviour-adjacent change lands, and the tag trigger that
Phase 5 needs to publish a beta. Nothing in this phase changes what the
CLI does.

#### Tasks

- [x] Change the `go.mod` module line to `github.com/donaldgifford/docz/v2`
      and rewrite every import under `cmd/`, `internal/`, and `pkg/`
      (including tests) to the `/v2` path; run `make fmt`.
      verify: `go build ./... && go vet ./...`
- [x] Update the `-X` ldflags in the `Makefile` `build-core` target and in
      `.goreleaser.yml` to `github.com/donaldgifford/docz/v2/cmd.Version`
      and `.Commit`; confirm `make build` still injects the version.
      verify: `make build && build/bin/docz version`
- [x] Update `test/consumer/go.mod` (`replace github.com/donaldgifford/docz/v2
      => ../..`) and the import paths in `test/consumer/doc.go`; the
      existing assertions do not change.
      verify: `make test-consumer`
- [x] Update every non-Go spelling of the module path: the README install
      and library sections, `DEVELOPMENT.md`, and the CLAUDE.md
      `test/consumer` bullet; state that v1.x tags keep the old path.
      verify: `grep -rnE 'donaldgifford/docz/(cmd|pkg|internal)' --include='*.go' .`
      prints nothing
- [x] Build the parity fixture repos under `test/parity/fixtures/`: one
      per built-in type (`rfc`, `adr`, `design`, `impl`, `investigation`)
      with two or three documents each — one straight from the type's
      template and one hand-written in the fleet's messier style — one
      repo with a custom type (`frameworks`, alias `fw`, `id_prefix: FW`,
      `docs/templates/frameworks.md`), and one legacy repo whose
      `.docz.yaml` still carries a `plan:` block. Fixture documents carry
      canonical region markers from the start (DESIGN-0015 rollout note)
      so no later migration touches them, and `author:` is pinned in each
      fixture config.
- [x] Write the parity driver as a Go test in `test/parity/` behind
      `//go:build parity`, driving the binary with `os/exec` (Open
      Question 3), with one subtest per fixture × command: `init` on an
      empty directory
      and on the fixture; `create <type> "<title>"` with `--author`,
      `--status`, and `--no-update`; `update` and `update <type>` with and
      without `--dry-run`; `list` in text, json, and csv with `--status`;
      `status set` for the happy path, an unchanged value, `--dry-run`,
      `--format json`, id-not-found, unknown type, and invalid status;
      `template show`, `export`, and `override` for a built-in and the
      custom type; `config`; `wiki init` and `wiki update` with
      `--dry-run`. Each case records stdout, stderr, the exit code, and
      the resulting file tree (path and bytes) into
      `test/parity/testdata/<fixture>/<case>.golden`. Normalisers, each
      named and unit-tested: the fixture's absolute root → `$ROOT`,
      today's date → `$DATE`, and `<!--docz:…-->` lines dropped from
      `create` and `template` output (DESIGN-0014 §4 permitted delta).
      The legacy fixture skips `create plan` and `template show|export
      plan`. A `-update` flag captures goldens only when `DOCZ_PARITY_BIN`
      names the binary to capture from.
      verify: `go vet -tags parity ./test/parity/...`
- [x] Add the `parity-capture` Makefile target: `go install
      github.com/donaldgifford/docz/cmd/docz@v1.2.2` into a temporary
      `GOBIN` (Open Question 2), then the `-update` run with
      `DOCZ_PARITY_BIN` pointing at it; the variable can be preset to
      capture from another binary. Capture the goldens once and commit
      them with `test/parity/README.md` recording the tag, the module
      checksum, `go version`, and platform. Goldens are never regenerated
      from a v2 build; a golden is only edited by hand under a permitted
      delta, with the reason in the PR.
      verify: `make parity-capture` leaves a populated
      `test/parity/testdata/`
- [x] Add the `parity` Makefile target (build `build/bin/docz`, then
      `go test -tags parity ./test/parity/...` against it, overridable with
      `BIN=`). Not yet in `make ci` (Phase 5 wires it). As a determinism
      self-check, replay the goldens against the same v1.2.2 binary they
      came from.
      verify: `make parity BIN=<v1.2.2 binary>`
- [x] Add `.github/workflows/prerelease.yml` on `push: tags: ['v*-beta.*']`
      running only the goreleaser job (checkout with full history,
      setup-go, GPG import, `goreleaser release --clean`) so a pushed beta
      tag builds binaries with the `/v2` ldflags; `prerelease: auto`
      already marks the GitHub release (Open Question 4). `release.yml` is
      untouched, so the label-driven `bump-version` job never sees a tag
      event.
      verify: `actionlint .github/workflows/*.yml` (or the schema check the
      repo's editor runs) and `make release-local`
- [x] Check `pr-semver-bump`'s pre-release-base behaviour (ADR-0002 Open
      Question 3): read how v1.7.4 discovers the latest tag and whether a
      pre-release counts, and what `major` from `v2.0.0-beta.N` yields.
      npm `semver.inc("2.0.0-beta.1", "major")` is `2.0.0`, so the open
      part is base-tag discovery. Record the finding and the fallback
      (tag v2.0.0 by hand with `make release TAG=`) in the release section
      of `DEVELOPMENT.md`, together with the beta procedure and the
      on-demand `v1` branch.
- [x] CLAUDE.md: module path in the `test/consumer` bullet; a "Parity
      suite" bullet describing `test/parity/`, the normalisers, the
      permitted deltas, and the rule that goldens come from v1.2.2 only.

#### Success Criteria

- `make ci` passes on the `/v2` module path with the `cmd/` test files
  byte-identical to v1.2.2's.
- `grep -rnE 'donaldgifford/docz/(cmd|pkg|internal)' --include='*.go' .`
  prints nothing.
- `make parity BIN=<v1.2.2 binary>` is green: the goldens replay exactly
  against the binary they were captured from.
- `make release-local` builds a snapshot with the `/v2` ldflags and
  `build/bin/docz version` prints an injected version.
- `test/parity/README.md` records the capture provenance;
  `.github/workflows/prerelease.yml` exists and lints clean; the
  `pr-semver-bump` finding is written into `DEVELOPMENT.md`.
- The PR merges with `dont-release` and `main` cuts nothing.

---

### Phase 1: The type layer — regions, validate, kinds, five type packages, docwrite byte cores, marked templates

Everything here is additive to the frozen packages or an entirely new
package, so ADR-0001's freeze holds. Order inside the phase follows the
dependency graph, one commit per package: `docparse` first (everything
reads regions, list items, and tables through it), then the templates and
skeletons (the golden-pair tests need them), then `kinds`, `validate`,
`pkg/impl`, the four other type packages, the `docwrite` cores, and the
consumer proof. The CLI's only visible change is the marker lines in
`create` and `template` output, which the parity driver drops.

#### Tasks

- [x] Add `pkg/doczcore/docparse/regions.go` with `Role`, `Marker{Kind, Role,
      Line, Canonical}`, `Region{Kind, Start, End, Depth, Closed}`,
      `Markers([]byte) []Marker`, and `Regions([]byte) []Region` per
      DESIGN-0015 §1: trimmed-line match, lenient spellings on the read
      side with `Canonical` reporting the canonical form, fence-aware via
      the package's fence rule, stack nesting with `Depth`, `Closed ==
      false` for a region open at end of file, a stray end reported as a
      marker but never a region, the legacy `<!--toc:start-->` pair
      reported as kind `toc`, and the README `DOCZ AUTO-GENERATED` pair as
      kind `index` (Open Question 9 of DESIGN-0015). Bytes in, values out,
      no error return.
      verify: `go test ./pkg/doczcore/docparse/...`
- [x] Walker goldens under `pkg/doczcore/docparse/testdata/regions/`: the
      canonical IMPL shape, nested and repeated kinds, stray end, unclosed
      at end of file, markers inside a fence, every lenient spelling, a
      legacy-ToC-only document, and a README with the index pair; each
      with a `.golden.txt` fact file regenerated by `-update`. `FuzzRegions`
      pins never-panic, `Start < End`, depth consistency, and `Closed`
      semantics.
- [x] Add `docparse.ListItems` (`ListItem{Text, Ordered, Indent, Line}`:
      every bullet or numbered item, `TaskItems` being the checkbox
      subset) and `docparse.Tables` (`Table{Header, Rows, Line}`: GFM pipe
      tables, cells trimmed, inline markdown kept, the delimiter row
      dropped) under the same bytes-in/values-out contract, fence-aware,
      with goldens under `testdata/{listitems,tables}/` and a fuzz target
      each (DESIGN-0014 §2.3).
      verify: `go test ./pkg/doczcore/docparse/...`
- [x] Re-point `toc.UpdateToC` and the `parseHeadings` skip past
      `<!--toc:end-->` onto `docparse.Regions` kind `toc` (DESIGN-0014
      §2.5); the exported surface does not change and the existing golden
      pins the output byte-identical.
      verify: `go test ./pkg/doczcore/toc/...`
- [x] Add `Schema string` with the yaml tag `schema,omitempty` to
      `document.Frontmatter` (DESIGN-0015 §3); `ParseFrontmatter` and
      `LoadFrontmatter` round-trip it; a `docwrite.SetStatus` golden with a
      `schema:` line proves the status locator ignores it.
      verify: `go test ./pkg/doczcore/document/... ./pkg/doczcore/docwrite/...`
- [x] Add canonical region markers around every section of every embedded
      template (`rfc.md`, `adr.md`, `design.md`, `impl.md`,
      `investigation.md`, and `plan.md` until Phase 4 deletes it) per the
      DESIGN-0015 §3 skeletons: one region per heading the type's `Doc`
      exposes, nested where the skeleton nests (`consequences` over
      positive, negative, neutral; `scope` over in-scope and out-of-scope;
      `phase` over tasks and criteria), the `## Goals and Non-Goals` and
      `## Implementation Phases` headings and the `---` breaks left
      outside, Supporting Data left inside its parent. No template gains
      or loses a section. The legacy ToC pair stays as it is. Built-in
      templates ship without a `schema:` line. Regenerate the template
      goldens.
      verify: `go test ./internal/template/... -update && go test ./internal/template/...`
- [x] Add the embedded skeletons `internal/template/templates/schema/<type>.md`
      for each built-in type, plus the generic pair `default.md` and
      `schema/default.md` used to scaffold custom types in Phase 3. A
      skeleton is a body of only region markers listing every section of
      its template in order with the same nesting (DESIGN-0015 §3: the
      full IMPL skeleton and the table for the other four); `default.md`
      carries the ToC pair and `references` only. Extend the embed
      directive to cover `templates/schema/*.md`.
- [x] Create `pkg/doczcore/kinds` (`item.go`, `question.go`,
      `reference.go`, `decision.go`, `criteria.go`, `alternative.go`,
      `field.go`): `Item`, `Section`, `Criterion`, `Alternative`,
      `Reference`, `Option`, `Resolution`, `Question`, `Decision`, and the
      readers `Body`, `Items`, `Sections`, `Criteria`, `Alternatives`,
      `References`, `OpenQuestions`, `Decisions`, `Field` (DESIGN-0014
      §2.12), each taking one region's bytes, heading included, no error,
      no filesystem, no type name. The open-question grammar is the
      fleet's (`### N.` headings, lettered `- a.` options, the
      `> **Resolved <date>: (x)**` blockquote); `Field` accepts both bold
      spellings the investigations use for `**Answer:**`. Plus the
      inference trio (`infer.go`): `HeadingRule{Kind, Level, Text, Prefix,
      Parent}`, `HeadingSpec`, `SpecFromTemplate(tmpl []byte)` (first
      heading inside each region of a marked template, nesting as parent,
      a placeholder-comment heading such as `### Phase N: <!-- … -->`
      generalised to the prefix rule `Phase <token>:`, the shared kinds'
      default headings always included), and `InferRegions(doc, spec)
      []docparse.Region` (nil when the document has any `docz:` marker;
      headings compared after trimming, case-folding, and stripping inline
      markdown and comments; spans to the next same-or-shallower heading
      minus trailing blanks and a trailing `---`; a parent's span to the
      end of its last child).
      verify: `go test ./pkg/doczcore/kinds/...`
- [x] `kinds` tests: a table per reader over fixtures cut from the corpus
      (every open-questions section of ADR-0002, ADR-0003, DESIGN-0014,
      and DESIGN-0015; references with and without links; the Decisions
      tables; criteria in the RFC and IMPL positions; alternatives as
      bullets and as headings) and a fuzz target per reader pinning
      never-panic and byte-accurate lines. For inference: `SpecFromTemplate`
      over every embedded template asserting the expected rules including
      the phase prefix rule; `InferRegions` over each type package's
      `.orig.md` fixtures asserting the regions equal `docparse.Regions`
      over the migrated sibling in kind, depth, and parent; a mixed
      document (one marker) infers nothing; `FuzzInferRegions`.

      > **Done 2026-09-20, with two amendments.** Alternatives as headings
      > has no corpus fixture: all three real `## Alternatives Considered`
      > sections in this repo are lettered bullet lists, so that shape is
      > covered by a unit test and the gap is recorded in
      > `pkg/doczcore/kinds/testdata/README.md` rather than papered over with
      > an invented fixture. Criteria in the RFC position comes from the
      > parity suite's RFC fixture, since no RFC has ever been committed
      > under `docs/`. The `.orig.md` half of the inference proof moves to
      > the task that creates those fixtures (Phase 1, the type-package
      > fixtures task); its equivalent runs here already as
      > `TestInferenceEqualsMarkers` over all six embedded templates plus
      > `TestInferRegions_OverARealDocument` over the real ADR-0002.
- [ ] Create `pkg/doczcore/validate` (`finding.go`, `schema.go`,
      `document.go`, one file per code family): `Severity` (`Error`,
      `Warning`), `Finding{Code, Severity, Line, Kind, Detail}`,
      `SchemaRegion{Kind, Parent}`, `Schema{Regions}`, `Options{Schema,
      Type, Filename, Headings}`, `SchemaFromMarkers([]byte) Schema`, and
      `Document([]byte, Options) []Finding` with the DESIGN-0015 §4 code
      families and severities (`marker.*`, `region.*`, `frontmatter.*`,
      `references.*`, `open-questions.*`, `tasks.*`, `toc.*`, `file.*`,
      `schema.name`) and the forty-one-kind catalogue of DESIGN-0015 §2 as
      a `map[string]KindRule`, whose `references` and `open-questions`
      rules call the `kinds` readers. Every schema kind is required at
      least once under the same parent; unlisted kinds are optional and
      still checked by their rule when present; an empty `Schema` is
      well-formedness only. `Document` never fails. `Options.Headings
      kinds.HeadingSpec`: when set and the document carries no `docz:`
      marker, regions come from `kinds.InferRegions`, one `region.inferred`
      warning names the kinds inferred and, per schema kind it could not
      infer, the heading it looked for, and every other check runs over
      the inferred regions; an absent heading is still `region.missing`
      (DESIGN-0015 §4 as amended).
      verify: `go test ./pkg/doczcore/validate/...`
- [ ] Validator tables per code family with a passing and a failing
      document each, `schema.name` included. Golden pairs: `Document` over
      every embedded template rendered with placeholder data, against its
      skeleton, asserts zero findings; `SchemaFromMarkers` over each
      skeleton asserts the expected kinds and parents; and the derivation
      test asserts `SchemaFromMarkers(template) == SchemaFromMarkers(skeleton)`
      for each built-in, so the pair can only be edited together.
- [ ] Add the layer-rule tests in `pkg/doczcore` (DESIGN-0014 §6, R2): a
      `go list -deps` walk per core package fails if `pkg/impl` or
      `pkg/wiki` appears; a sibling test fails if any `go.opentelemetry.io`
      or logging module appears under `pkg/`.
      verify: `go test ./pkg/doczcore/ -run 'TestLayer'`
- [ ] Create `pkg/impl` (`doc.go`, `parse.go`, `walk.go`, `validate.go`):
      `Parse([]byte) (Doc, error)`, `Doc{ID, Title, Status, Author,
      Created, Objective, Implements, InScope, OutOfScope, Phases,
      FileChanges, Testing, Dependencies, OpenQuestions, Decisions,
      References}`, `Phase{Index, Token, Title, Description, Tasks,
      Criteria []kinds.Criterion, Line}`, `Task{ID, Text, Checked, Verify,
      Deferred, Skipped *Marker, Line, EndLine}`, `Marker{Note, Line}`,
      `FileChange{File, Action, Description, Line}`, `Doc.Task(id)`,
      `Doc.Phase(token)`, `Doc.Tasks()`, `Doc.Progress()`, `ErrNoPhases`,
      `DuplicatePhaseError{Token, Lines}`. Spans come from
      `docparse.Regions` (`phase` at depth 0, `tasks` and `criteria` at
      depth 1; every other field from its own region per the DESIGN-0014
      §2.9 field rules, with `kinds.Field` for `**Implements:**` and
      `docparse.Tables` for the file-changes table); the type name is
      never read (R7). A document with no `docz:` marker has its regions
      inferred by `kinds.InferRegions` from the package's heading table
      and `Doc.Inferred` set; markers, once present, are authoritative
      (DESIGN-0014 §2.9 as amended). The DESIGN-0014 §3 grammar table
      verbatim: first H3 inside the phase
      region with inline markdown and HTML comments stripped, matched by
      `^Phase\s+([^\s/:]+):\s*(.*)$`; description between the heading and
      the first depth-1 region; tasks are `TaskItem`s with `Indent == 0`;
      continuation folding of deeper-indented non-list lines; `verify:`
      case-insensitive with the first backtick span as the command;
      deferred and skipped markers; criteria `Executable` iff the bullet
      starts with a backtick span; LF only; every string copied.
      verify: `go test ./pkg/impl/...`
- [ ] `impl.Validate(doc []byte) []validate.Finding` with the codes
      `impl.phase.duplicate-token`, `impl.phase.no-heading`,
      `impl.phase.no-tasks`, `impl.phase.no-title`, `impl.task.empty`,
      `impl.task.verify-no-command`, `impl.task.skipped-no-note`; a
      document `Parse` rejects yields a single finding for the error.
- [ ] Golden fixtures under `pkg/impl/testdata/`, snapshotted (never read
      from `docs/`) and kept in two copies each: `<name>.orig.md` exactly
      as written and `<name>.md` hand-migrated with canonical markers (the
      `.orig.md` copies become Phase 3's `InsertRegions` inputs): docz
      IMPL-0009 and IMPL-0017 (wrapping), IMPL-0001 (non-phase `### Phase
      N` headings under File Changes), IMPL-0007 (phases without
      criteria), docz-api IMPL-0004 (verify lines), docz-api IMPL-0006
      (prefix deferred marker), sdk-booty-sh's clean and messy fixtures, a
      synthetic document with a skipped task and one with a task inserted
      mid-run. `.golden.txt` fact files regenerated by `-update`.
      Invariant tests: every `Task.Line` is a line `docparse.TaskItems`
      reports; IDs are unique; no `Text` contains a verify prefix or a
      marker; `EndLine >= Line`; a skipped task keeps its ID; `Parse` over
      each `.orig.md` equals `Parse` over its migrated sibling in every
      field but `Inferred`. `FuzzParse` pins never-panic.
- [ ] Create `pkg/rfc`, `pkg/adr`, `pkg/design`, and `pkg/investigation`,
      one commit each, with the `Doc` shapes and `Validate` codes of
      DESIGN-0014 §2.9: `rfc.Doc{Summary, Problem, Proposal, Alternatives,
      Risks, Criteria, OpenQuestions, References}` with `Risk` from the
      risks table; `adr.Doc{Summary, Context, Decision, Consequences
      {Positive, Negative, Neutral}, Alternatives, OpenQuestions,
      References}`; `design.Doc{Overview, Goals, NonGoals, Background,
      DetailedDesign, APIChanges, DataModel, Testing, Rollout,
      OpenQuestions, Decisions, References}`; `investigation.Doc{Question,
      Hypothesis, Context, TriggeredBy, Approach, Environment, Findings,
      Conclusion, Answer, Verdict, Recommendation, OpenQuestions,
      Decisions, References}` with `Component` from the environment table
      and `Verdict` from the answer's first word. Each `Parse` reads every
      field from its region through `kinds` or `docparse.Tables`, infers
      regions from its heading table when the document has no `docz:`
      marker (setting `Doc.Inferred`), fails only on `ErrNoFrontmatter` or
      CR, and copies every string; each `Validate` runs `Parse` and
      reports its three codes.
      verify: `go test ./pkg/rfc/... ./pkg/adr/... ./pkg/design/... ./pkg/investigation/...`
- [ ] Heading tables: each of the five packages carries `var headings
      kinds.HeadingSpec` as data, and a test in the package asserts it
      equals `kinds.SpecFromTemplate` over the embedded template of its
      type (`doctemplate` is a test-only import there), so a template
      edit that moves a heading fails the package's tests and production
      imports stay at L0, `kinds`, and `validate`.
- [ ] Golden fixtures for the four packages under each `testdata/`, as
      `.orig.md` plus hand-migrated `.md` pairs like `impl`'s: sdk-booty-sh's
      three RFCs and one rendered from the template; docz ADR-0001–0003
      with sdk-booty-sh's and tempy's ADRs; docz designs sampled for shape
      variety (one with a Decisions table, one without Open Questions)
      plus docz-api's; docz INV-0001–0010 (free-form Findings headings,
      both `**Answer:**` spellings, one without a trigger line).
      `.golden.txt` fact files regenerated by `-update`; a `FuzzParse`
      each; the invariants: every `Line` is a line `docparse` reports, a
      field is zero iff its region is absent, each `Validate` code appears
      only when its condition holds, with one passing and one failing
      document per code, and `Parse` over each `.orig.md` equals `Parse`
      over its migrated sibling in every field but `Inferred`.
- [ ] `docwrite.SetStatusBytes(doc []byte, status string) (out []byte, old
      string, err error)` as the byte core; `SetStatus(path, …)` becomes
      read → core → write. The existing status goldens pass through the
      wrapper unchanged; a bytes-only table covers the core.
      verify: `go test ./pkg/doczcore/docwrite/...`
- [ ] `docwrite.SetTaskStateBytes(doc []byte, line int, checked bool)
      ([]byte, error)`, `SetTaskState(path, line, checked)`, and
      `ErrTaskAlreadyUnchecked`; `CheckTask` becomes `SetTaskState(path,
      line, true)`. The checktask goldens pass unchanged; a bytes-only
      table covers the uncheck direction.
- [ ] `docwrite.Rendered{Filename, Content}`, `NextNumber(dir string,
      width int) (string, error)`, and `Render(opts *CreateOptions, number
      string) (Rendered, error)`; `Create` becomes `NextNumber` → `Render`
      → write. A test asserts `Render`'s output equals what `Create`
      writes for the same inputs.
- [ ] Extend `test/consumer/doc.go`: one `Parse` call per type package
      over an inline fixture each, `kinds.OpenQuestions`,
      `validate.Document` with a schema from `SchemaFromMarkers`,
      `docparse.Regions`, and `docwrite.SetStatusBytes`, each asserting a
      known value from outside the module.
      verify: `make test-consumer`
- [ ] CLAUDE.md: architecture bullets for `docparse` regions, list items,
      and tables, `validate`, `kinds`, the five type packages, the
      `docwrite` byte cores, `Frontmatter.Schema`, and the embedded
      skeletons; `DEVELOPMENT.md`'s "add a type" walkthrough gains the
      markers-and-skeleton step and the note that a built-in also needs a
      package.

#### Success Criteria

- `make ci` is green and the `test/consumer` calls that existed at v1.2.2
  compile unchanged — nothing in the five frozen packages changed shape.
- `go test ./pkg/doczcore/docparse/... ./pkg/doczcore/kinds/... ./pkg/doczcore/validate/... ./pkg/impl/... ./pkg/rfc/... ./pkg/adr/... ./pkg/design/... ./pkg/investigation/...`
  passes, fuzz seed corpora included.
- Every embedded template validates clean against its skeleton, the
  derivation test binds each template to its skeleton, and each type
  package parses its own rendered template with every field present.
- Every `.orig.md` fixture parses through inference to the same facts as
  its migrated sibling with `Inferred` set, and each package's heading
  table equals `SpecFromTemplate` over its embedded template.
- `go test ./pkg/doczcore/toc/...` passes with the golden byte-identical
  after the `Regions` re-point.
- The `docwrite` status and checktask goldens pass unchanged through the
  path wrappers.
- `go test ./pkg/doczcore/ -run 'TestLayer'` passes.
- `make parity` is green against `build/bin/docz` with marker lines as the
  only delta.
- `make test-consumer` exercises `validate`, `kinds`, and all five type
  packages from outside the module.

---

### Phase 2: Promotions — doctemplate, index, pkg/wiki; schema resolution; internal/ emptied

Three `git mv` promotions that carry their tests with them, plus the
additions each package needs to stand on its own. `cmd/` only re-points
its imports; its logic and its tests are untouched until Phase 5. At the
end of this phase `internal/` no longer exists.

#### Tasks

- [ ] `git mv internal/template pkg/doczcore/doctemplate` (package
      `doctemplate`, embedded `templates/` and `docz_yaml.tmpl` included)
      and settle the exported surface per DESIGN-0014 §2.7: `ErrNoTemplate`,
      `ErrNoSchema`, `Data`, `IndexHeaderData`, `WikiIndexData`, `Resolve`,
      `ResolveIndexHeader`, `ResolveWikiIndex`, `Render`, `RenderWikiIndex`,
      `FilenameSlug`, `EmbeddedDocumentTemplate`, `EmbeddedWikiIndex`,
      `GenericTemplate() (string, error)`, `DefaultConfigYAML() (string,
      error)`. `cmd/init` calls `DefaultConfigYAML` in place of its inline
      rendering — the one `cmd/` edit this phase makes beyond imports.
      verify: `go test ./pkg/doczcore/doctemplate/... ./cmd/...`
- [ ] Schema resolution: `EmbeddedSchema(name string) ([]byte, error)`
      (baked-in only) and `ResolveSchema(name, docsDir string) ([]byte,
      error)` — `<docsDir>/templates/schema/<name>.md`, then the embedded
      `schema/<name>.md`, else `ErrNoSchema`. The name grammar
      `[a-z0-9][a-z0-9_-]*` is enforced here (a bad name is `ErrNoSchema`
      with a distinguishable wrapped sentinel so `validate` can emit
      `schema.name`). The third tier — the type's own resolved template's
      markers when the name is the type name — and the `schema.unresolved`
      finding belong to the resolving tier in `repo.Validate` (Phase 3).
- [ ] Resolution tests: a repo-local `templates/schema/impl.md` beats the
      baked-in one; an unknown name is `ErrNoSchema`; `EmbeddedSchema`
      returns a skeleton for every built-in and for `default`.
- [ ] `git mv internal/index pkg/doczcore/index` and add `BeginMarker`,
      `EndMarker`, the action enum exported as `UpdateAction`,
      `Splice(existing []byte, header, table string) ([]byte, UpdateAction)`
      locating the pair via `docparse.Regions` kind `index`, and
      `Scaffold(header string) []byte`; `UpdateReadme` and `DryRunReadme`
      become wrappers over `Splice`. The package now imports `docparse`.
      verify: `go test ./pkg/doczcore/index/...`
- [ ] `Splice` table pinning every `UpdateReadme` outcome unchanged
      (created, updated, no markers, both dry-run forms) and the `Scaffold`
      regression from issue #99: exactly one marker pair for every type,
      built-in and custom.
- [ ] `git mv internal/wiki pkg/wiki` and add `Action` (`Created`,
      `Skipped`, `Overwritten`), `InitOptions{SiteName, SiteDescription,
      RepoURL, SiteURL, Theme, Force}`, `InitReport{MkDocsPath, MkDocs,
      IndexPath, Index}`, `Init(ctx, root, cfg, opts)`, `NavOptions{DryRun}`,
      `NavReport{Path, Entries, Pages, Written}`, and `UpdateNav(ctx, root,
      cfg, opts)` (DESIGN-0014 §2.10, Open Question 8). The existing
      primitives stay exported; `cmd/wiki.go` keeps its own helpers until
      Phase 5.
      verify: `go test ./pkg/wiki/...`
- [ ] Temp-dir tests for `wiki.Init` and `UpdateNav` mirroring today's
      `cmd/wiki` tests (create, skip, force, dry-run, the nav page count);
      existing goldens carry over with the move.
- [ ] Re-point every `cmd/` import from `internal/{template,index,wiki}`
      to the promoted packages with no logic change; remove the now-empty
      `internal/` directory.
      verify: `test ! -d internal && go test ./cmd/...`
- [ ] Extend `test/consumer/doc.go`: `doctemplate.EmbeddedDocumentTemplate`,
      `doctemplate.EmbeddedSchema`, `index.Scaffold`, and
      `wiki.FilenameTitle`, one call each.
      verify: `make test-consumer`
- [ ] CLAUDE.md: bullets for the three promoted packages, the schema
      resolution tiers, and the removal of `internal/`; drop the sentence
      that `internal/template` is not importable from outside the module.

#### Success Criteria

- `make ci` is green; `internal/` does not exist; the `cmd/` test files
  differ from Phase 1 only in import paths.
- The moved goldens (`doctemplate`, `index`, `wiki`) pass byte-identical;
  the `Splice` table and the `Scaffold` #99 regression pass.
- Resolution tests pass and `EmbeddedSchema` returns a skeleton for every
  built-in.
- `make parity` is green with marker lines as the only delta.
- `make test-consumer` covers fifteen `pkg/` packages.
- `go test ./pkg/doczcore/ -run 'TestLayer'` still passes with the three
  new packages in the graph.

---

### Phase 3: The repository core — pkg/doczcore/repo with context, Hooks, Validate, InsertRegions

The package that turns the primitives into the operations a consumer
would otherwise copy out of `cmd/`. Every method takes a context, returns
a typed report, and prints nothing; `cmd/` does not adopt any of it until
Phase 5, so this phase is invisible to CLI users. `InsertRegions` writes
what `kinds.InferRegions` finds; no heuristic lives here.

#### Tasks

- [ ] Create `pkg/doczcore/repo` (`repo.go`, `scan.go`, `create.go`,
      `update.go`, `status.go`, `init.go`, `template.go`, `validate.go`,
      `regions.go`, `hooks.go`, `errors.go`) with `Repo{Root, Cfg}`,
      `Open(ctx, root, configFile)` (config load and validate), and the
      path helpers `Path`, `TypeDir`, `ReadmePath` (DESIGN-0014 §2.8).
      verify: `go test ./pkg/doczcore/repo/...`
- [ ] Typed errors in `errors.go`: `NotFoundError{Type, ID}`,
      `TypeDisabledError{Type}`, `ExistsError{Path}`,
      `InvalidStatusError{Type, Status, Allowed}`, `UnknownTypeError{Token,
      Valid}` unwrapping to `config.ErrUnknownType`, and `WriteError{Path,
      Err}` (Open Question 9 of DESIGN-0014); an error test per shape.
- [ ] Read side: `Scan(ctx, types...)`, `List`, `Find(ctx, id)`,
      `FindIn(ctx, typeName, id)` returning `Entry{DocEntry, Type, Path}`;
      nil types means `EnabledTypes()`; a disabled type is
      `TypeDisabledError`; an unresolved id is `NotFoundError`.
- [ ] `Create(ctx, CreateOptions{Type, Title, Author, Status, Now, Update})
      (CreateResult, error)` over `docwrite.NextNumber` and `Render`, with
      `Update == true` running the type's index and ToC pass and reporting
      it in `CreateResult.Update`; `ExistsError` on a filename collision.
      `Create` never calls `wiki.UpdateNav` — that stays in `cmd/` behind
      `Wiki.AutoUpdate`.
- [ ] `Update(ctx, UpdateOptions{DryRun}, types...) (UpdateReport, error)`:
      per type `Scan`, `toc.UpdateFiles`, and `index.UpdateReadme` or
      `DryRunReadme` with the header from `doctemplate.ResolveIndexHeader`,
      producing `TypeReport{Type, Dir, Docs, ToC, Index, Elapsed}`. This
      absorbs `cmd/update.go`'s `updateType`, `runToCUpdate`, and
      `indexLabel`, which stay in `cmd/` until Phase 5.
- [ ] `SetStatus(ctx, typeName, id, status, StatusOptions{DryRun})
      (StatusResult, error)`: an unchanged value returns `Changed: false`
      without writing (DESIGN-0014 Open Question 7); `InvalidStatusError`
      for a status outside `TypeConfig.Statuses`; `WriteError` wraps the
      `docwrite` failure.
- [ ] `Init(ctx, InitOptions{Force}) (InitReport, error)`: `.docz.yaml`
      from `doctemplate.DefaultConfigYAML`, the enabled type directories,
      and `index.Scaffold(header)` READMEs; `InitFile{Path, Action}` with
      `InitCreated`, `InitSkipped`, `InitOverwritten`.
- [ ] `Template(ctx, typeName) ([]byte, error)` and `ExportTemplate(ctx,
      typeName, dest, ExportOptions{Overwrite}) (ExportResult, error)`;
      `dest == ""` means override at `<docsDir>/templates/<type>.md`. For a
      custom type with no resolvable template it scaffolds
      `GenericTemplate()` and `EmbeddedSchema("default")` as
      `templates/<type>.md` and `templates/schema/<type>.md`, returning
      `Scaffolded: true` and `SchemaPath` (DESIGN-0015 §3). A test asserts
      the written pair validates clean.
- [ ] `Validate(ctx, types, ValidateOptions{Strict}) (ValidateReport,
      error)` per DESIGN-0015 §4: per enabled type, the template check
      (render the resolved template with placeholder data, validate
      against the schema the type name resolves to, skip ToC drift); per
      document, resolve the schema by the frontmatter `schema:` name or
      the type name — `ResolveSchema`, then for the type's own name the
      resolved template's markers, else `schema.unresolved` with
      well-formedness only — cached by name for the run, then
      `validate.Document` with `Type` and `Filename`, then `toc.UpdateToC`
      for `toc.stale`; per type `index.DryRunReadme` for `IndexDrift`.
      `DocFindings{Type, Path, Findings}` (plus the field Open Question 10
      settles), `ValidateReport{Docs, Templates, Index, Errors, Warnings}`.
      `repo` never imports `pkg/impl` (R2).
- [ ] `InsertRegions(ctx, types, InsertRegionsOptions{DryRun})
      (InsertRegionsReport, error)` per DESIGN-0015 §6 as amended: a
      document with any `docz:` region is never given more (non-canonical
      spellings are rewritten and counted in `Fixed`); otherwise regions
      come from `kinds.InferRegions` with `kinds.SpecFromTemplate` over
      the type's resolved template, so custom types with a marked template
      migrate too, and the pass writes what inference found: markers on
      their own lines with a blank line on each side, parents inserted
      before children; a heading the document lacks is skipped; the pass
      runs `Regions` on its output and refuses to write a malformed
      result. `DryRun` stays on the library option for consumers; the CLI
      has no flag for it because plain `validate` is the preview.
- [ ] Migration tests: `InsertRegions` over each `.orig.md` fixture in the
      five type packages' `testdata/` equals its hand-migrated sibling
      byte-for-byte, and a second run changes nothing; snapshots of docz's
      own `docs/{adr,design,impl,investigation}` trees under `t.TempDir()`
      yield the expected `Regions` kinds and are idempotent; a custom type
      with a marked template migrates by the same map. The heuristics
      parity proof: a test-only heading-based locator over each IMPL
      `.orig.md` yields the same task IDs and task-line text as
      `impl.Parse` over the migrated output, which is what licenses
      deleting the heuristics with the pass later.
- [ ] `Hooks{ScanStart, ScanDone, TypeSkipped, FileWritten, FileSkipped}`,
      `WithHooks(ctx, Hooks) context.Context`, `HooksFrom(ctx) Hooks`,
      `FileKind`, and `SkipReason` (DESIGN-0014 §7, R8); nil hooks are
      no-ops; the context is checked between per-type iterations and a
      cancelled run returns the partial report with `ctx.Err()`.
- [ ] Context and hooks tests: a cancelled context makes `Update` return
      after the current type with the partial report intact; a hooks test
      asserts the exact event sequence for a two-type update.
- [ ] Table tests over `t.TempDir()` repos for every method, each
      asserting the typed report and the on-disk result, including the
      dry-run forms.
- [ ] Extend `test/consumer/doc.go`: `repo.Open` on a temp repo, `Scan`,
      `SetStatus` with `DryRun`, and `Validate`, asserting typed results.
      verify: `make test-consumer`
- [ ] CLAUDE.md: the `repo` bullet (methods, typed errors, hooks, the
      no-op status rule, the never-imports-`impl` rule).

#### Success Criteria

- `make ci` is green and `cmd/` is untouched (no diff under `cmd/`).
- `go test ./pkg/doczcore/repo/...` passes: every method's table, every
  typed error, the cancelled-context test, and the hook-sequence test.
- `go test ./pkg/doczcore/ -run 'TestLayer'` passes with `repo` in the
  graph and `pkg/impl` absent from its dependencies.
- `InsertRegions` reproduces every hand-migrated fixture across the five
  type packages byte-for-byte and is idempotent; the heuristics parity
  proof passes.
- `ExportTemplate` on a template-less custom type writes both files and
  the pair validates clean.
- `make parity` is green with marker lines as the only delta.
- `make test-consumer` covers `repo`.

---

### Phase 4: Catalogue — ADR-0003 plan removal

The one breaking change to the frozen catalogue, riding the v2 line as
ADR-0003 decided. It lands as its own PR ahead of the swap so the registry
is final before `cmd/` is re-pointed, and it is the first phase whose CLI
output differs from v1.2.2 in more than marker lines, which is why it adds
the fourth permitted delta to the parity suite (Open Question 8).

#### Tasks

- [ ] Remove the `plan` entry from `allDocTypes` in
      `pkg/doczcore/config/doctype.go`; `DocTypeNames`, `TypesHelp`,
      `DefaultConfig().Types`, and `DefaultNavTitles` follow from the
      registry.
      verify: `go test ./pkg/doczcore/config/...`
- [ ] Delete the embedded `plan.md`, `index_plan.md`, and
      `schema/plan.md`, their goldens, and the `types.plan.enabled: true`
      comment in `docz_yaml.tmpl`; trim the PLAN hints in the `impl.md`
      and `investigation.md` "Implements / Triggered by" comments.
- [ ] Regenerate goldens with `go test ./... -update` and fix the
      remaining test references (`--help` output, `init` creating five
      directories, config and wiki nav-title tests).
      verify: `make test`
- [ ] Legacy-block tests: a `.docz.yaml` carrying `types.plan` loads as a
      custom type; `list` and `update` work over `docs/plan`;
      `docz create plan` fails with the message naming
      `docs/templates/plan.md` as the fix; `docz template override plan`
      now scaffolds the generic pair (Phase 3), which the message can
      point at.
- [ ] Add the `types.plan` normaliser to the parity driver — the block is
      dropped from `docz config` output and from a generated `.docz.yaml`
      on both sides before comparing — with its own unit test, list it as
      the fourth permitted delta in `test/parity/README.md`, and confirm
      the legacy fixture's skips (`create plan`, `template show|export
      plan`) hold (Open Question 8).
      verify: `make parity`
- [ ] This repo's own remnants: drop the dormant `plan:` block from
      `.docz.yaml`, delete `docs/plan/README.md`, and run
      `docz wiki update` so `mkdocs.yml` loses the Plans nav entry.
- [ ] Docs: README types table and the PLAN section; CLAUDE.md "Six
      built-in doc types" becomes five and the alias line loses nothing;
      `DEVELOPMENT.md`'s worked example if it names plan; a release-notes
      paragraph (kept in the PR body under `### RELEASE NOTES` for the
      Phase 5 tag's notes) spelling out the custom-type fallback.

#### Success Criteria

- `make ci` is green; `config.DocTypeNames()` returns five names and
  `config.LookupDocType("plan")` reports false.
- No `plan.md`, `index_plan.md`, or `schema/plan.md` exists under the
  embedded templates, and `docz --help` lists five types.
- The legacy-block tests pass; `make parity` is green for every fixture
  with the `types.plan` normaliser as the fourth permitted delta.
- README, CLAUDE.md, and `DEVELOPMENT.md` no longer describe plan as a
  built-in; `mkdocs.yml` has no Plans entry.

---

### Phase 5: The cmd swap, docz validate, corpus migration, and v2.0.0-beta.1

The validation of everything above: `cmd/` moves onto the API with its
test files unchanged, the parity suite runs green against the swapped
binary and joins `make ci`, docz's own documents get their regions, and
the merge commit is tagged `v2.0.0-beta.1` by hand. Any `cmd/` test that
has to change is a behaviour change and blocks the PR (ADR-0001
Decision 7).

#### Tasks

- [ ] `Runner` gains `Repo *repo.Repo`; `loadAndValidateConfig` resolves the
      repo root as today, then calls `repo.Open` and sets `Runner.Cfg`
      from `Repo.Cfg` so the print, emit, and format helpers are untouched.
      verify: `go test ./cmd/...`
- [ ] Wire the context and hooks (DESIGN-0014 §7): `signal.NotifyContext`
      at the top of `Execute`, `repo.WithHooks` mapping `ScanStart`,
      `ScanDone`, `TypeSkipped`, `FileWritten`, and `FileSkipped` onto the
      `r.Logger.Debug` lines `cmd/` emits today, so the existing debug-log
      test pins the mapping from the other side.
- [ ] Re-point each command per the DESIGN-0014 §4 table: `init` →
      `repo.Init` and, when the wiki block is enabled, `wiki.Init`;
      `create` → `repo.Create` then `wiki.UpdateNav` behind `Wiki.AutoUpdate`;
      `update` → `repo.Update`; `list` → `repo.Scan`; `status set` →
      `repo.FindIn` and `repo.SetStatus`, printing from `StatusResult`;
      `template show|export|override` → `repo.Template` and
      `repo.ExportTemplate`; `wiki init|update` → `wiki.Init` and
      `wiki.UpdateNav`; `config` unchanged. Delete `updateType`,
      `runToCUpdate`, `indexLabel`, `writeDefaultConfig`, `writeIndexReadme`,
      `findByID`, `updateWikiNav`, `updateWikiNavDryRun`, and
      `ensureDocsIndex`; keep `Runner`, `GitResolver`, `buildLogger`,
      `exitCodeFor`, the print/emit/format functions, and every flag.
      verify: `git diff --stat origin/main -- 'cmd/*_test.go'` prints
      nothing
- [ ] `docz validate [type] [--strict] [--format text|json]` in
      `cmd/validate.go`: `repo.Validate`, then the per-type tier composed
      in `cmd/` as an explicit five-arm switch on `DocFindings.Schema`
      with the type name as fallback — `impl`, `rfc`, `adr`, `design`,
      `investigation` (Open Question 10) — one line per finding as
      `path:line code detail`, JSON as
      the report verbatim, exit 0 clean, 1 on errors (or warnings under
      `--strict`), 2 for a usage error; command tests pin both formats and
      every exit code.
- [ ] `docz validate --fix`: after the first report, `repo.InsertRegions`
      over the documents that reported `region.inferred` or
      `marker.spelling`, print each file written and the kinds inserted,
      re-run `repo.Validate` and the type tier, print the remaining
      findings, and exit on the second report; command tests pin the
      files written, the second report, and that a second `--fix` writes
      nothing.
- [ ] Migrate docz's own `docs/` in this PR as separate commits (Open
      Question 6): run `build/bin/docz validate --fix` and commit the
      mechanical diff on its own, then fix by hand whatever the second
      report lists (sections whose heading was renamed, stale ToCs after
      `docz update`), committing the hand edits separately.
      verify: `build/bin/docz validate` exits 0
- [ ] Wire `make parity` into `make ci` and the `test-go` CI job after the
      consumer smoke test; the swapped binary must be green under only the
      permitted deltas. Then add a `validate` target running
      `build/bin/docz validate` non-strict over this repo's `docs/` and
      append it to `make ci` after `parity` (Open Question 7), so warnings
      print and errors fail.
      verify: `make ci`
- [ ] Extend `test/consumer/doc.go` so it imports every `pkg/` package
      (`config`, `document`, `docparse`, `docwrite`, `toc`, `validate`,
      `kinds`, `doctemplate`, `index`, `repo`, `impl`, `rfc`, `adr`,
      `design`, `investigation`, `wiki`) with one call each.
      verify: `make test-consumer`
- [ ] ADR-0001 dated amendment (ADR-0002 Open Question 2): the frozen five
      keep their v1 shapes through the betas, `internal/template` is
      importable as `doctemplate`, and the new packages are experimental
      until v2.0.0; IMPL-0014 Decision 3 note; DESIGN-0013 → Abandoned.
- [ ] Living docs: CLAUDE.md (architecture bullets for the eleven new
      packages, the `cmd/` swap, `internal/` gone, the parity suite, the
      beta release procedure), README (library section with the `/v2`
      import path and the experimental note, `docz validate` and
      `--fix`, five types), `DEVELOPMENT.md` (add-a-type walkthrough
      with markers and skeleton, release section), and `mkdocs.yml`
      (`pymdownx.superfences` so the design diagrams render).
- [ ] Close docz issue #97 (retargeted 2026-09-20 from `update --check` to
      `docz validate`: `toc.stale` and `IndexDrift` are the CI gate) with a
      comment naming the beta tag. The claude-skills plugin is not touched
      in this unit (Out of Scope).
- [ ] Merge as `dont-release`, then from the merge commit run
      `make release TAG=v2.0.0-beta.1`; confirm the pre-release workflow
      built the binaries and marked the release a pre-release; write the
      release notes (the library changelog for Phases 0–5, the `/v2`
      path with the v1 tags keeping the old one, the plan fallback) into
      the GitHub release body with `gh release edit`.
      verify: `gh release view v2.0.0-beta.1 --json isPrerelease,assets`
- [ ] Post-tag proof: from a scratch module,
      `go get github.com/donaldgifford/docz/v2@v2.0.0-beta.1` resolves
      through the module proxy and `test/consumer`'s calls compile against
      it without the local replace.
- [ ] Status flips with `docz status set`: DESIGN-0014 and DESIGN-0015 →
      Implemented and this document → Completed (ADR-0002 and ADR-0003
      were Accepted and both designs Approved with the docs PR on
      2026-09-20); note in IMPL-0017's Objective that it now targets the
      v2 line.

#### Success Criteria

- `make ci`, now including `make parity`, is green on the swap PR and the
  `cmd/` test files are byte-identical to the Phase 4 baseline.
- `make parity` is green against the swapped binary with only the four
  permitted deltas (marker lines; no goldens for `validate` with or
  without `--fix`; the legacy plan skips; the `types.plan` normaliser).
- `build/bin/docz validate` exits 0 over docz's own `docs/` and a second
  `docz validate --fix` writes nothing.
- `make test-consumer` imports all sixteen `pkg/` packages from outside
  the module.
- The `v2.0.0-beta.1` tag exists, its GitHub release is a pre-release with
  binaries, and `go get …/v2@v2.0.0-beta.1` resolves from a scratch module.
- ADR-0001 carries the amendment; DESIGN-0014 and DESIGN-0015 read
  Implemented; ADR-0002 and ADR-0003 read Accepted; CLAUDE.md, README, and
  `DEVELOPMENT.md` describe the v2 layout.
- `main` builds a CLI that behaves like v1.2.2 apart from the two new
  commands and the plan removal.

---

## File Changes

| File | Action | Description |
| ---- | ------ | ----------- |
| `go.mod`, `**/*.go`, `Makefile`, `.goreleaser.yml`, `test/consumer/go.mod` | Modify | Module path `/v2` and ldflags (Phase 0) |
| `test/parity/` (driver, `fixtures/`, `testdata/`, `README.md`) | Create | Parity suite with goldens captured from v1.2.2 (Phase 0) |
| `.github/workflows/prerelease.yml` | Create | `v*-beta.*` tag trigger running goreleaser only (Phase 0, Open Question 4) |
| `pkg/doczcore/docparse/regions.go` + `testdata/regions/` | Create | `Markers`, `Regions`, goldens, `FuzzRegions` (Phase 1) |
| `pkg/doczcore/docparse/{listitems,tables}.go` + testdata | Create | `ListItems`, `Tables` facts for the type packages (Phase 1) |
| `pkg/doczcore/kinds/` | Create | Readers for the shared kinds: open questions, references, decisions, criteria, alternatives, fields, items, sections (Phase 1) |
| `pkg/doczcore/toc/toc.go` | Modify | Span located via `docparse.Regions` kind `toc` (Phase 1) |
| `pkg/doczcore/document/document.go` | Modify | `Frontmatter.Schema` (Phase 1) |
| `internal/template/templates/*.md`, `templates/schema/*.md`, `default.md` | Modify/Create | Region markers around every section, embedded skeletons listing them, generic pair (Phase 1) |
| `pkg/doczcore/validate/` | Create | `Finding`, `Schema`, `SchemaFromMarkers`, `Document` (Phase 1) |
| `pkg/doczcore/layer_test.go` | Create | R2 dependency and no-telemetry tests (Phase 1) |
| `pkg/impl/` + `testdata/` | Create | `Parse`, `Doc`, `Validate`, goldens in `.orig.md`/`.md` pairs, `FuzzParse` (Phase 1) |
| `pkg/{rfc,adr,design,investigation}/` + `testdata/` | Create | `Parse`, `Doc`, `Validate`, goldens, `FuzzParse`, one package per built-in (Phase 1) |
| `pkg/doczcore/docwrite/{status,checktask,create}.go` | Modify | `SetStatusBytes`, `SetTaskStateBytes`/`SetTaskState`, `NextNumber`/`Render` (Phase 1) |
| `pkg/doczcore/doctemplate/` | Create (git mv) | Promotion + `ResolveSchema`, `EmbeddedSchema`, `GenericTemplate`, `DefaultConfigYAML` (Phase 2) |
| `pkg/doczcore/index/` | Create (git mv) | Promotion + `Splice`, `Scaffold`, `UpdateAction` (Phase 2) |
| `pkg/wiki/` | Create (git mv) | Promotion + `Init`, `UpdateNav`, reports (Phase 2) |
| `internal/` | Delete | Emptied by the promotions (Phase 2) |
| `pkg/doczcore/repo/` | Create | `Repo`, operations, `Validate`, `InsertRegions`, `Hooks`, typed errors (Phase 3) |
| `pkg/doczcore/config/doctype.go` | Modify | `plan` registry entry removed (Phase 4) |
| `pkg/doczcore/doctemplate/templates/{plan,index_plan,schema/plan}.md` | Delete | ADR-0003 (Phase 4) |
| `.docz.yaml`, `docs/plan/README.md`, `mkdocs.yml` | Modify/Delete | This repo's plan remnants (Phase 4) |
| `cmd/runner.go`, `cmd/root.go` | Modify | `Runner.Repo`, `repo.Open`, context and hooks wiring (Phase 5) |
| `cmd/{init,create,update,list,status,template,wiki}.go` | Modify | Re-pointed at `repo` and `wiki`; helpers deleted (Phase 5) |
| `cmd/validate.go`, `cmd/validate_test.go` | Create | `docz validate` and `--fix` (Phase 5) |
| `docs/**/*.md` | Modify | Corpus migrated with `validate --fix` and hand fixes (Phase 5) |
| `Makefile`, `.github/workflows/ci.yml` | Modify | `parity` and `validate` in `make ci` and the `test-go` job (Phase 5) |
| `test/consumer/doc.go` | Modify | Grows with each phase to cover every `pkg/` package |
| `docs/adr/0001-*.md`, `docs/impl/0014-*.md`, `docs/design/0013-*.md` | Modify | Amendment, note, Abandoned (Phase 5) |
| `CLAUDE.md`, `README.md`, `DEVELOPMENT.md` | Modify | Per phase; consolidated in Phase 5 |

## Testing Plan

- [ ] Parity: `test/parity/` goldens from v1.2.2 replayed against
      `build/bin/docz` from Phase 1 on; in `make ci` from Phase 5
- [ ] `docparse`: region, list-item, and table goldens with a fuzz target
      each; `toc` golden byte-identical after the re-point
- [ ] `kinds`: a table per reader over corpus cuts; a fuzz target per
      reader
- [ ] `validate`: per-family tables, the catalogue's per-kind rules, golden
      pairs over every embedded template and skeleton, the
      template-to-skeleton derivation test
- [ ] `pkg/impl`, `pkg/rfc`, `pkg/adr`, `pkg/design`, `pkg/investigation`:
      golden fixtures with invariants, `FuzzParse`, `Validate` cases for
      every code, each package parsing its own rendered template and each
      `.orig.md` fixture through inference; heading tables pinned to the
      embedded templates
- [ ] `docwrite`: existing goldens through the path wrappers; bytes-only
      tables for `SetStatusBytes`, the uncheck direction, and `Render`
- [ ] `doctemplate`, `index`, `wiki`: moved tests unchanged; `Splice`
      table, `Scaffold` #99 regression, resolution tests, `Init` and
      `UpdateNav` temp-dir tests
- [ ] `repo`: table tests over temp repos for every method, an error test
      per typed error, cancelled-context and hook-sequence tests,
      `InsertRegions` equality with hand-migrated fixtures and idempotence,
      the heuristics parity proof, `ExportTemplate` scaffold validates clean
- [ ] Layer rules: `go list -deps` walk and the no-telemetry test in
      `pkg/doczcore`
- [ ] `cmd/`: test files unchanged through the swap; new tests only for
      `validate` (formats, exit codes) and `validate --fix` (files
      written, second report, idempotence)
- [ ] Consumer proof: `test/consumer` imports every `pkg/` package by the
      end of Phase 5 and compiles against the published beta
- [ ] Corpus: `docz validate` exits 0 over docz's own `docs/` after
      migration

## Dependencies

- ADR-0002 and ADR-0003 accepted, DESIGN-0014 and DESIGN-0015 approved,
  and the open questions below answered before Phase 0 starts.
- The `v1.2.2` tag and a Go 1.26.4 toolchain for the golden capture.
- goreleaser v2 with `prerelease: auto`, and the `pr-semver-bump` v1.7.4
  behaviour finding from Phase 0 before Phase 5 tags.
- `gh` for the release-body edit and closing docz #97.
- The go-development skills: go-architect before Phases 1 and 3, go-review
  after every phase, go-test for the new test packages.
- Nothing external: docz-api, docz-site, tempy, and sdk-booty-sh are not
  waited on and do not wait on this.

## Open Questions

> Each question is numbered; option `a` is my recommendation, later letters
> are alternatives, and the last is a free-form "other".

### 1. PR granularity

DESIGN-0014's rollout shows one merge per step. Phase 1 is by far the
largest step (two new packages, four frozen packages touched additively,
every template, and the fixture corpus), and its review cost is the
question.

> **Resolved 2026-09-19: (a).**

- a. **One `dont-release` PR per phase, merged in order**, with Phase 1's
  work landing as a sequence of per-package commits so a reviewer can walk
  it commit by commit. Matches the design's gitGraph, and each phase's
  success criteria are the PR's acceptance. *(recommendation)*
- b. Split Phase 1 into two PRs: `docparse` regions, templates, skeletons,
  and `validate` first; `pkg/impl` and the `docwrite` cores second. Smaller
  reviews at the cost of an intermediate `main` where the skeletons exist
  and nothing reads them.
- c. One long-lived branch for all six phases and a single PR at the end.
- d. Other.

### 2. Where the v1.2.2 binary for the golden capture comes from

The goldens are captured once and committed, so this only has to be
reproducible and documented.

> **Resolved 2026-09-19: (a).**

- a. **`go install github.com/donaldgifford/docz/cmd/docz@v1.2.2` into a
  temporary `GOBIN`** from a `make parity-capture` target: checksum-verified
  by the module proxy, no checkout, and the tag plus `go version` are
  stamped into `test/parity/README.md`. `docz version` prints `dev`, which
  no golden records. *(recommendation)*
- b. Build from the tag in a `git worktree` with `make build`, so the
  ldflags version is injected. More steps for a value nothing compares.
- c. Download the goreleaser asset for the host platform from the v1.2.2
  release. Reproducible only for that platform's build.
- d. Other.

### 3. Shape of the parity driver

> **Resolved 2026-09-19: (a).**

- a. **A Go test in `test/parity/` inside the root module behind
  `//go:build parity`**, driving the binary with `os/exec`, one subtest per
  fixture × command, goldens as files, `-update` gated on `DOCZ_PARITY_BIN`,
  and `make parity` as build-then-`go test -tags parity`. Excluded from
  `make test` by the tag, no new dependency, and the normalisers are plain
  functions with their own unit tests. *(recommendation)*
- b. `rogpeppe/go-internal/testscript` with `.txtar` scripts. Expressive,
  but a new test-only dependency through `license-check`, and the
  normalisers become script-level conventions.
- c. A shell script and `diff`. No subtests, no `-update`, and the
  normalisers live in `sed`.
- d. Other.

### 4. Pre-release build trigger

> **Resolved 2026-09-19: (a).**

- a. **A separate `.github/workflows/prerelease.yml`** on
  `push: tags: ['v*-beta.*']` that runs only the goreleaser job (checkout
  with full history, setup-go, GPG import, `goreleaser release --clean`).
  `release.yml` stays untouched, so the label-driven path cannot regress
  and there are no `if:` guards to get wrong. The goreleaser steps are
  duplicated in two small files. *(recommendation)*
- b. Extend `release.yml` with the `tags` trigger and guard both jobs with
  `if:` on the event. One file, two code paths.
- c. No workflow: `make release-local` and upload the artifacts by hand.
- d. Other.

### 5. `docz task list <impl-id>`

DESIGN-0014 §4 lists it as optional; ADR-0002 Open Question 4 says the CLI
needs no first-party caller of `pkg/impl`.

> **Resolved 2026-09-19: (a).** Removed from Phase 5; listed under Out of
> Scope.

- a. **Defer past this document.** Nothing consumes it, the parity suite
  has no golden for it, and it can ship in any later beta without a design
  change. *(recommendation)*
- b. Include it in Phase 5 as a text and JSON listing over `impl.Parse`
  (ID, checked, verify, markers per task).
- c. Other.

### 6. Placement of docz's own corpus migration

DESIGN-0015's rollout says "in the same PR" as the swap. The mechanical
diff over every document in `docs/` is large.

> **Resolved 2026-09-19: (a).**

- a. **Same PR as the swap, as separate commits**: the `docz validate
  --fix` output as one commit, hand fixes as another, so the reviewer
  can skip the mechanical one. Keeps the design's statement true and lets
  the swap PR itself prove `docz validate` over a real corpus.
  *(recommendation)*
- b. A separate `dont-release` PR merged immediately after the swap and
  before the tag. Smaller swap PR; two PRs to sequence before tagging.
- c. Other.

### 7. `docz validate` in this repo's CI

> **Resolved 2026-09-19: (a).**

- a. **Yes, non-strict**: `build/bin/docz validate` joins `make ci` in
  Phase 5 after the corpus migration, so the corpus cannot drift from its
  schemas. Warnings print but do not fail. *(recommendation)*
- b. `--strict`, so warnings fail too. Cleaner, but the hand-written older
  documents will need more fixing before the swap PR can merge.
- c. Not in CI; run on demand.
- d. Other.

### 8. The plan delta in the parity suite

ADR-0003 changes two outputs the parity suite records against v1.2.2:
`docz config` prints `types.plan` from `DefaultConfig()` in v1.2.2 and
not after Phase 4, and `docz init`'s generated `.docz.yaml` carries the
plan comment in v1.2.2 and not after. DESIGN-0014 §4's permitted deltas do
not cover this. (`docz --help` also changes, but it is not in the suite.)

> **Resolved 2026-09-19: (a).** The fourth permitted delta; applied in
> Phase 4.

- a. **A named normaliser in the driver** that drops the `types.plan` block
  from `docz config` output and from a generated `.docz.yaml` on both
  sides before comparing, listed as the fourth permitted delta in
  `test/parity/README.md` and applied from Phase 4. The legacy fixture
  keeps its skips. Everything else stays captured from v1.2.2.
  *(recommendation)*
- b. Recapture the affected `config` and `init` goldens from the Phase 4
  build. Simpler, but breaks the rule that goldens come only from v1.2.2.
- c. Drop `config` and `init`'s `.docz.yaml` from the suite.
- d. Other.

### 9. Marking this document by hand now

> **Resolved 2026-09-19: (a).**

- a. **No.** Leave IMPL-0018 unmarked and migrate it with the corpus in
  Phase 5, so it is one more real `InsertRegions` case. Nothing reads its
  regions before then. *(recommendation)*
- b. Hand-mark it now as the first real document in the new shape.
- c. Other.

### 10. Which documents get the per-type tier in `docz validate`

DESIGN-0015 §4's sequence runs `impl.Validate` for `DocFindings` with
`Type == impl`. DESIGN-0015 §3 also says a custom type that sets
`schema: impl` "gets `impl.Parse`" (ADR-0002 R7). `DocFindings{Type, Path,
Findings}` does not carry the resolved schema name, so `cmd/` cannot make
that second case true.

> **Resolved 2026-09-19: (a), generalised to every built-in.** Every
> built-in is a structured type with its own package (DESIGN-0014 §2.9 as
> amended), so the switch in `cmd/validate.go` has five arms — `impl`,
> `rfc`, `adr`, `design`, `investigation` — keyed on `DocFindings.Schema`
> with the type name as fallback. Applied to DESIGN-0015 §4 and
> DESIGN-0014 §4; the escape hatch for unstructured markdown is the
> `api:` block's additional docs, not a looser type.

- a. **Add `Schema string` (the resolved schema name) to
  `repo.DocFindings`** and have `cmd/validate.go` run `impl.Validate` for
  every document whose `Schema == "impl"` or whose `Type == "impl"`. The
  first clause covers a custom type on the impl contract; the second keeps
  a built-in IMPL document on the impl check even when it names a stricter
  repo-local skeleton (`schema: strict-impl`), since it is still an IMPL.
  A custom type whose own skeleton merely copies the phase, tasks, and
  criteria kinds is not on the impl contract and gets the generic tier
  only — the name is the declaration. A one-field, dated amendment to
  DESIGN-0015 §4. *(recommendation)*
- b. Dispatch on `Type == "impl"` only, as the diagram reads; a custom type
  with `schema: impl` gets the generic tier only until a later beta.
- c. Other.

## Decisions

All ten open questions resolved **(a)** on 2026-09-19; 10 generalised
to every built-in type, which amended DESIGN-0014 §2.9/§2.12 and
DESIGN-0015 §2/§3/§4/§6 the same day.

| # | Question | Resolution |
| - | -------- | ---------- |
| 1 | PR granularity | One `dont-release` PR per phase, merged in order; Phase 1 lands as per-package commits |
| 2 | v1.2.2 binary for the golden capture | `go install …/cmd/docz@v1.2.2` into a temporary `GOBIN` from `make parity-capture`; provenance in `test/parity/README.md` |
| 3 | Parity driver shape | A Go test in `test/parity/` behind `//go:build parity`, `os/exec` over the binary, file goldens, `-update` gated on `DOCZ_PARITY_BIN` |
| 4 | Pre-release build trigger | A separate `prerelease.yml` on `v*-beta.*` tags running only goreleaser; `release.yml` untouched |
| 5 | `docz task list` | Deferred past this document; removed from Phase 5 |
| 6 | Corpus migration placement | In the swap PR as separate commits: the mechanical `validate --fix` diff, then the hand fixes |
| 7 | `docz validate` in CI | Yes, non-strict, appended to `make ci` after `parity` in Phase 5 |
| 8 | Plan delta in the parity suite | A named `types.plan` normaliser, the fourth permitted delta, applied from Phase 4 |
| 9 | Hand-marking this document | No; it migrates with the corpus in Phase 5 |
| 10 | Per-type tier dispatch in `docz validate` | An explicit five-arm switch in `cmd/validate.go` on the new `DocFindings.Schema` with the type name as fallback; every built-in has a package (`impl`, `rfc`, `adr`, `design`, `investigation`), a custom type on `schema: <built-in>` gets that arm, one on its own schema gets the generic tier only |
| — | **Amendment 2026-09-20: documents without markers** | A document with no `docz:` marker parses by inference from its headings (`kinds.InferRegions`; `Doc.Inferred`; one `region.inferred` warning, `region.missing` still an error for an absent heading; markers once present are authoritative); `docz validate --fix` writes what inference found and re-validates, replacing `update --regions` (DESIGN-0015 OQ 4 → c). Phase 1, 3, and 5 tasks updated |
| — | **Amendment 2026-09-20: claude-skills deferred, #97 retargeted** | The docz skills plugin is not touched and no claude-skills issue is filed in this unit: it serves v1 users and marked templates or five types there would break them before v2 ships; its update is a follow-up after `v2.0.0-beta.1` (Out of Scope; ADR-0003 Decision 5 amendment). docz issue #97 (`update --check`) was retargeted the same day to `docz validate` (`toc.stale`, `IndexDrift`) and is closed by Phase 5 |

## References

- [DESIGN-0014](../design/0014-the-docz-api-as-one-unit-packages-types-functions-and-the-cmd.md)
  — the API as one unit: packages, types, functions, the cmd swap, parity
  suite, context and hooks, rollout steps 0–5
- [DESIGN-0015](../design/0015-structured-regions-and-docz-validate.md)
  — region markers, the schema skeleton model, `validate`, corpus migration
- [ADR-0002](../adr/0002-docz-is-an-api-package-whose-first-consumer-is-the-cli.md)
  — the layers, R1–R8, the `/v2` module path, betas only, v2.0.0 reserved
- [ADR-0003](../adr/0003-remove-plan-from-the-built-in-document-types.md)
  — plan removal on the v2 line
- [ADR-0001](../adr/0001-pkgdoczcore-as-the-single-public-core-cmd-as-a-thin-cli-shell.md)
  — the freeze this work amends at the swap
- [IMPL-0017](0017-v130-updated-frontmatter-field-and-the-docz-update-stamp-pass.md)
  — retargets to the v2 line after this unit
- [Issue #99](https://github.com/donaldgifford/docz/issues/99) — the
  `Scaffold` marker-pair regression
- [Issue #103](https://github.com/donaldgifford/docz/issues/103) — plan
  removal facts
- `test/parity/README.md` — golden provenance and permitted deltas (created
  in Phase 0)
