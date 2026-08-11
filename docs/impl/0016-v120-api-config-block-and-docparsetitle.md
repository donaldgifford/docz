---
id: IMPL-0016
title: "v1.2.0 — api config block and docparse.Title"
status: Draft
author: Donald Gifford
created: 2026-08-11
---

<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL-0016: v1.2.0 — api config block and docparse.Title

<!--toc:start-->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: shared path validator + api: config block](#phase-1-shared-path-validator--api-config-block)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2: docparse.Title](#phase-2-docparsetitle)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 3: consumer proof + living docs](#phase-3-consumer-proof--living-docs)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 4: the v1.2.0 release](#phase-4-the-v120-release)
    - [Tasks](#tasks-3)
    - [Success Criteria](#success-criteria-3)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Dependencies](#dependencies)
- [Open Questions](#open-questions)
  - [1. What rules apply to api.exclude entries?](#1-what-rules-apply-to-apiexclude-entries)
  - [2. What should the shared path validator's signature be?](#2-what-should-the-shared-path-validators-signature-be)
  - [3. Should internal/wiki.firstH1 collapse onto docparse.Title?](#3-should-internalwikifirsth1-collapse-onto-docparsetitle)
  - [4. How should Title treat a setext H1?](#4-how-should-title-treat-a-setext-h1)
  - [5. Should Title skip YAML frontmatter?](#5-should-title-skip-yaml-frontmatter)
  - [6. Where does the consumer proof for the new surface live?](#6-where-does-the-consumer-proof-for-the-new-surface-live)
  - [7. PR and release strategy for this branch?](#7-pr-and-release-strategy-for-this-branch)
  - [8. Should Phase 2 extend the existing fixtures or add a new one?](#8-should-phase-2-extend-the-existing-fixtures-or-add-a-new-one)
- [Decisions](#decisions)
  - [Note: Decisions 1 and 2 interact](#note-decisions-1-and-2-interact)
- [References](#references)
<!--toc:end-->

## Objective

Implement DESIGN-0011: the opt-in `api:` block in `.docz.yaml` plus the one
public addition it requires, `docparse.Title`. Ships as an additive
**`v1.2.0`**, dormant by default, and unblocks docz-api's R10 pin.

**Implements:** DESIGN-0011 (grounded by INV-0007)

Two things make this smaller than it looks and one makes it larger:

- The `changelog:` block (DESIGN-0010 / IMPL-0015) is a working precedent for
  every config-side step — struct, defaults, normalize, enabled-only validate,
  `docz_yaml.tmpl` block, consumer proof.
- The path-hardening rules already exist and are already correct; this is an
  extraction, not a rewrite.
- But `docparse.Title` adds a function to a **frozen** package, and
  `internal/wiki` already contains a second, subtly different H1 implementation
  that should collapse into it. That is the phase with real design in it.

## Scope

### In Scope

- `APIConfig` struct, `Config.API`, `DefaultConfig()` entry, `normalizeAPI`,
  `validateAPI`, `ErrInvalidAPIPath`
- Extracting the repo-relative path rules into one unexported helper shared by
  `validateChangelog` and `validateAPI` (INV-0007 Decision 2)
- `docparse.Title(content []byte) string`
- Collapsing `internal/wiki.firstH1` onto `docparse.Title` (Decision 3)
- `api:` block in `internal/template/templates/docz_yaml.tmpl`, dormant
- Consumer proof in `test/consumer/`
- README + godoc + `.docz.example.yaml` alignment, and the `v1.2.0` release

### Out of Scope

- **Any consumption behavior.** docz does not fetch, walk, glob, or route. The
  consumption rule in DESIGN-0011 is a contract for docz-api, not code here.
- **Any `cmd/` change.** `docz config` picks the block up for free from the
  struct field.
- **Assets/images** (DESIGN-0011 Decision 10).
- **docz-api and docz-site work.** Separate repos, separate plans.
- **A `Links` extractor.** Only relevant to the asset design we deferred.

## Implementation Phases

Four phases, sequential. Phases 1 and 2 are independent of each other and could
be swapped; Phase 3 depends on both.

---

### Phase 1: shared path validator + `api:` config block

The config side in `pkg/doczcore/config`. Starts with the extraction so
`validateAPI` is written against the shared helper rather than retrofitted onto
it.

#### Tasks

- [x] Extract the repo-relative path rules out of `validateChangelog` into an
      unexported `validateRepoRelativePath(field, value string, allowDir bool)
      error` (Decisions 1 + 2). It returns a **bare** error the caller wraps
      with its own sentinel, so `errors.Is(err, ErrInvalidChangelogFile)`
      keeps working for existing consumers. It keeps the existing rule set
      verbatim — empty,
      control characters, backslash, `filepath.IsAbs`/leading `/`/volume name,
      leading `~`, trailing `/`, `..` segment, `.` or empty segment — and
      returns a bare error whose message names `field`. Each caller wraps it
      with its own sentinel.
- [x] Rewire `validateChangelog` onto the helper, wrapping with
      `ErrInvalidChangelogFile`. **Behavior must not change**, with one
      deliberate exception: the current `.`/empty-segment branch stutters —
      `ErrInvalidChangelogFile` already renders "invalid changelog.file" and
      the message repeats it. Fix the wording as part of the move.
- [x] Confirm `hasVolumeName` moves with the helper and keeps its comment about
      not depending on the validating host. That reasoning *is* the function
      and is easy to lose in a move.
- [ ] Add `APIConfig{Enabled bool, LandingPage string, Exclude []string,
      AdditionalDocs []string}` with yaml tags only (`enabled`,
      `landing_page`, `exclude`, `additional_docs`), and `Config.API` with yaml
      tag `api`. No `mapstructure` tags — vestigial post-viper, per
      IMPL-0015's `ChangelogConfig` precedent.
- [ ] Add the `DefaultConfig()` entry: `{Enabled: false, LandingPage: "",
      Exclude: nil, AdditionalDocs: nil}`. Dormant, and `DefaultConfig()` stays
      the sole source of defaults.
- [ ] Add the `api:` block to `internal/template/templates/docz_yaml.tmpl`,
      rendered from `DefaultConfig()` beside the `changelog:` block, with
      comments covering what it does **and** the blast-radius caution
      (enabling publishes every `.md` under `docs_dir`). Empty `exclude` /
      `additional_docs` need `{{- range }}` handling that emits a valid empty
      list. **Do this in Phase 1, not later:**
      `TestDoczYAMLTemplate_RoundTripsToDefaultConfig` renders the template
      with `DefaultConfig()` and parses it back, so a new default field with no
      template line fails immediately — this is part of adding the field, as
      IMPL-0015 Phase 1 discovered.
- [ ] Add `normalizeAPI(cfg *Config)` and call it in `Load` immediately after
      `normalizeChangelog` at **both** call sites (`config.go:216` and `:659`).
      It strips repeated leading `./` from `LandingPage` and every `Exclude` /
      `AdditionalDocs` entry, and backfills `LandingPage` to
      `<DocsDir>/index.md` when empty **and** `Enabled` is true. Never
      `filepath.Clean` — `..` must survive for `Validate` to reject; mirror the
      comment `normalizeChangelog` already carries.
- [ ] Add `ErrInvalidAPIPath` and `validateAPI()`, called from `Validate()`.
      Returns immediately unless `api.Enabled` — the dormancy guarantee. When
      enabled, run `LandingPage` and every `Exclude` / `AdditionalDocs` entry
      through the shared helper, then apply the block-specific rules: duplicate
      `AdditionalDocs` entry; `AdditionalDocs` entry under `DocsDir`;
      `AdditionalDocs` entry whose first path segment matches an **enabled**
      type's `Dir` or canonical name.
- [ ] Pass `allowDir: true` for `Exclude` entries (Decision 1): they are
      `docs_dir`-relative directory prefixes, so `templates` and `templates/`
      must mean the same thing. Every other rule — traversal, absolute, volume
      name, `~`, backslash, control characters, `.`/empty segments — applies
      unchanged. `LandingPage` and `AdditionalDocs` pass `false`.
- [ ] Config test table per DESIGN-0011's Testing Strategy: decode (full /
      partial / absent / `enabled` alone); defaults; normalization (`./`
      stripping on all three fields, backfill under a non-default `docs_dir`,
      **no** backfill when disabled, `..` survives); validation enabled (every
      rejection class, each asserted with `errors.Is(err, ErrInvalidAPIPath)`);
      validation disabled (**every** rejection class passes — the same table,
      inverted); first-segment collision allowed for a *disabled* type; merge
      semantics where `exclude`/`additional_docs` replace rather than append.
- [ ] Add a `parity_baseline_test.go` case pinning that a config carrying a
      dormant `api:` block now decodes rather than being ignored — the same
      rollout handshake IMPL-0015 added for `changelog:`.
- [ ] Godoc on every new exported symbol; `golangci-lint fmt ./...` +
      `make lint` green.

#### Success Criteria

- `validateChangelog`'s existing test table passes **unchanged** except for the
  one corrected message — the extraction is provably behavior-preserving.
- Full API config table green, including the inverted disabled-block table.
- `TestDoczYAMLTemplate_RoundTripsToDefaultConfig` and
  `TestDoczYAMLTemplate_RetainsCommentHeader` green.
- Unknown-key leniency and case-sensitivity pins still green.
- `docz config` prints the resolved `api:` block with no `cmd/` change.
- `make ci` green.

---

### Phase 2: `docparse.Title`

The one public addition, plus collapsing the duplicate H1 implementation
INV-0007 F2 found.

#### Tasks

- [ ] Add `Title(content []byte) string` to `pkg/doczcore/docparse` — new file
      `title.go`, beside `headings.go`. Returns the first H1's text with inline
      markdown stripped, or `""` when there is none. No error return: `""` is a
      normal outcome and the consumer supplies the fallback (INV-0007
      Decision 1 amendment).
- [ ] Reuse the package's existing primitives rather than re-deriving them:
      `isFenceToggle` for fence awareness, `stripInlineMarkdown` for the text.
      A fenced `# not a title` must not match.
- [ ] Support setext H1 — a non-blank line followed by a line of only `=`
      (Decision 4). `Title` reads markdown docz did not write, where setext is
      common; a `CONTRIBUTING.md` with a setext title must not fall back to the
      filename. **`Headings` is not touched** — this rule lives in `Title`
      alone. Needs an explicit test and a doc-comment sentence.
- [ ] Skip a leading YAML frontmatter block, **only when it starts at byte
      0** (Decision 5), so a `---` horizontal rule mid-document is never
      mistaken for a frontmatter opener. This preserves `wiki.firstH1`'s
      behavior exactly, which is what makes Decision 3's collapse safe.
- [ ] Verify `Headings` is byte-for-byte unchanged — its H1 exclusion is frozen
      behavior and `toc` plus consumer ToCs depend on it. The existing golden
      fixtures are the proof; they must not move.
- [ ] Extend `renderFacts` in `docparse/golden_test.go` with a `# title` line
      so `impl_plan.md` and `messy.md` cover `Title` too, regenerate with
      `-update`, and **read the diff** rather than trusting it.
- [ ] Add a dedicated fixture for the cases those two do not reach: no H1; H1
      inside a fence; multiple H1s (first wins); H1 not on line 1; H1 with
      inline markdown; H1 after frontmatter; setext; empty input.
- [ ] Collapse `internal/wiki.firstH1` onto `docparse.Title` (Decision 3)
      so there is one H1 definition module-wide — the rule
      `docwrite.CheckTask`→`docparse.TaskItems` already follows. Note the
      behavior delta: `firstH1` is not fence-aware and does not strip inline
      markdown, so `# **Bold** Title` currently yields `**Bold** Title` in the
      MkDocs nav and would become `Bold Title`.
- [ ] Update `internal/wiki` tests for the delta and regenerate the wiki
      goldens if nav titles move. Read that diff carefully — it is the only
      user-visible behavior change in this release.
- [ ] Update `doc.go` to list `Title` beside `Headings` and `TaskItems`,
      keeping the facts-not-interpretation framing.

#### Success Criteria

- `docparse` golden fixtures regenerate to a diff that is **additive only** —
  the `# headings` and `# tasks` sections byte-identical.
- The new Title fixture covers all eight cases, each with an asserted
  expectation.
- `internal/wiki` has no second H1 scanner; `firstH1` is gone or is a one-line
  delegation.
- `go doc pkg/doczcore/docparse` shows `Title` with a contract stating the `""`
  case, the fence rule, the setext decision, and the frontmatter decision.
- `make ci` green.

---

### Phase 3: consumer proof + living docs

Prove the surface is importable exactly as docz-api will import it, then make
the repo's own documentation tell the truth.

#### Tasks

- [ ] Add `test/consumer/consumer_v12_test.go` (Decision 6 — one file per
      release, the existing precedent) reading `cfg.API` fields by their public paths and call `docparse.Title`. Mirror
      the R10 clause literally: decode a block, prove `./` normalization, prove
      a traversal rejection via `errors.Is(err, config.ErrInvalidAPIPath)`,
      prove a dormant block does **not** reject, and prove `Title` returns `""`
      for a frontmatter-only document.
- [ ] Note in the phase record that `make test-consumer` is what catches a
      regression here — `test/consumer/` is a separate module outside root
      `./...`, so `go test ./...` passes while it is broken. This exact trap
      bit during the PR that preceded this branch.
- [ ] Update `.docz.example.yaml`: drop the "PROPOSED — not yet implemented"
      header from the `api:` block now that it is real.
- [ ] README: document the `api:` block — fields, the dormancy guarantee, the
      `docs_dir`-mirrors-URL rule, and the blast-radius caution about enabling
      it. Add `docparse.Title` wherever the public packages are summarized.
- [ ] Update `CLAUDE.md`'s `pkg/doczcore/config` and `pkg/doczcore/docparse`
      paragraphs — that file is the architecture map and is load-bearing for
      future work.
- [ ] Record **R10** in DESIGN-0008's requirements list (R1–R9 today), in the
      same form as its siblings, so docz-api has an explicit acceptance
      criterion rather than an inference (INV-0007 Recommendation 5).
- [ ] Godoc sweep: read `go doc -all` over `config` and `docparse` as a
      consumer would.

#### Success Criteria

- `make test-consumer` green, and demonstrably red if `Config.API` is renamed.
- `go doc -all ./pkg/doczcore/config ./pkg/doczcore/docparse` diffed against
  `v1.1.1` shows **additions only** — no signature or doc-comment regressions
  on existing symbols.
- README, `CLAUDE.md`, `.docz.example.yaml`, and DESIGN-0008 all describe the
  same block.
- `make ci` green.

---

### Phase 4: the `v1.2.0` release

#### Tasks

- [ ] Open the release PR with the `minor` label. `pr-semver-bump` +
      goreleaser cut and publish `v1.2.0`.
- [ ] Verify the published module from a scratch module outside this repo:
      `go get github.com/donaldgifford/docz@v1.2.0`, then exercise the full R10
      surface — block decode, `./` normalization, `errors.Is` on a traversal
      rejection, a dormant block loading clean, and `docparse.Title` on a
      document with and without an H1.
- [ ] Post-tag `dont-release` PR flipping DESIGN-0011 → Implemented and
      IMPL-0016 → Completed (Decision 7 — the `#67`/`#68`/`#79` pattern, since
      the release PR's own merge is what cuts the tag).
- [ ] Note in the release body that this is the release docz-api pins for R10,
      and that `v1.1.1` already changed `DefaultConfig()`/`EnabledTypes()` for
      the PLAN default — two consecutive releases touching defaults deserves one
      sentence so anyone bumping from `v1.1.0` sees both.

#### Success Criteria

- `v1.2.0` tagged and published; `go get` from the proxy works.
- The scratch-module R10 exercise passes against the **published** module, not
  a local `replace`.
- DESIGN-0011 Implemented, IMPL-0016 Completed.

## File Changes

| File | Action | Description |
| ---- | ------ | ----------- |
| `pkg/doczcore/config/config.go` | Modify | `APIConfig`, `Config.API`, `normalizeAPI`, `validateAPI`, `ErrInvalidAPIPath`, extracted `validateRepoRelativePath` |
| `pkg/doczcore/config/config_test.go` | Modify | API block table, inverted disabled table, merge semantics |
| `pkg/doczcore/config/parity_baseline_test.go` | Modify | Dormant-`api:`-block decode pin |
| `pkg/doczcore/docparse/title.go` | Create | `Title(content []byte) string` |
| `pkg/doczcore/docparse/title_test.go` | Create | The eight H1 cases |
| `pkg/doczcore/docparse/golden_test.go` | Modify | `renderFacts` gains a `# title` section |
| `pkg/doczcore/docparse/testdata/` | Create | New H1-edge-case fixture + regenerated goldens |
| `pkg/doczcore/docparse/doc.go` | Modify | Package doc lists `Title` |
| `internal/wiki/titles.go` | Modify | `firstH1` collapses onto `docparse.Title` |
| `internal/template/templates/docz_yaml.tmpl` | Modify | Dormant `api:` block |
| `test/consumer/` | Modify | R10 surface proof |
| `README.md` | Modify | `api:` block docs + blast-radius caution |
| `CLAUDE.md` | Modify | Architecture map for config + docparse |
| `.docz.example.yaml` | Modify | Drop the "proposed" header |
| `docs/design/0008-*.md` | Modify | Add contract clause R10 |

## Testing Plan

- [ ] Config: decode / defaults / normalize / validate-enabled /
      validate-disabled tables, all `errors.Is`-asserted
- [ ] Config: merge semantics for the two slice fields
- [ ] Parity: dormant-block decode pin; unknown-key leniency unchanged
- [ ] `docparse.Title`: eight-case table + golden fixtures
- [ ] `docparse.Headings`: golden fixtures **unchanged** (the freeze proof)
- [ ] `internal/wiki`: nav titles after the `firstH1` collapse
- [ ] `test/consumer`: R10 surface via public import paths
- [ ] Scratch module against the published `v1.2.0`
- [ ] `make ci` green at the end of every phase

## Dependencies

- **DESIGN-0011** — ten decisions recorded, zero open questions
- **INV-0007** — Concluded; supplies Decisions 1 and 2
- **`v1.1.1`** — merged and tagged (PR #83); this branch is cut from it
- **ADR-0001** — the additive-only rule that makes this a `minor`
- No external dependencies; no new modules

## Open Questions

**None.** All eight were resolved **(a)** on 2026-08-11; see **Decisions**
below. They are kept here for the alternatives, which record what was weighed.

### 1. What rules apply to `api.exclude` entries?

`exclude` entries are `docs_dir`-relative **directory prefixes**, not file
paths, so the shared validator's file-oriented rules do not all fit — notably
trailing `/`.

- **a. (Recommendation) Share the validator, but tolerate a trailing `/`.**
  Every rule that matters for safety — traversal, absolute, volume name, `~`,
  backslash, control characters, `.`/empty segments — applies identically to a
  directory prefix. Only "must be a file path, not a directory" is wrong here,
  so split that one branch out behind a parameter. `templates` and `templates/`
  should mean the same thing; rejecting the latter is the kind of papercut that
  makes a config feel brittle.
- **b. Apply the file rules verbatim**, rejecting a trailing `/`. Simplest
  code, one shared function with no parameters, at the cost of rejecting the
  natural way to write a directory.
- **c. Strip a trailing `/` in `normalizeAPI`**, then apply the file rules
  verbatim. Same user-visible result as (a) with no validator change — but it
  puts a semantic decision in the normalizer, and "Load normalizes, Validate
  judges" has so far kept normalization to purely lexical `./` stripping.
- **d. Give `exclude` its own validator.** Most precise, and reintroduces
  exactly the duplication INV-0007 Decision 2 exists to prevent.

### 2. What should the shared path validator's signature be?

INV-0007 Decision 2 settled *that* it is shared and unexported, not its shape.

- **a. (Recommendation) `validateRepoRelativePath(field, value string) error`,
  returning a bare error the caller wraps.** The caller owns its sentinel
  (`ErrInvalidChangelogFile` / `ErrInvalidAPIPath`), so `errors.Is` stays
  precise per-block, and `field` keeps messages specific — "changelog.file",
  "api.additional_docs[1]". Matches how the current code already builds them.
- **b. Return a pre-wrapped error with one new shared sentinel**
  (`ErrInvalidPath`). Simpler call sites, but it collapses two sentinels into
  one and breaks `errors.Is(err, ErrInvalidChangelogFile)` for existing
  consumers — a real break on a frozen package.
- **c. Take the sentinel as a parameter.** Keeps both sentinels and centralizes
  wrapping, at the cost of an odd signature.
- **d. Make it a method on `Config`** so it can consult `DocsDir` and `Types`.
  Tempting, since the `additional_docs`-specific rules need both — but those
  rules are *not* shared, so the helper should stay a free function and the
  block-specific checks stay in `validateAPI`.

### 3. Should `internal/wiki.firstH1` collapse onto `docparse.Title`?

The duplication INV-0007 F2 found. Collapsing is the module's stated rule, but
`firstH1` and a fence-aware, markdown-stripping `Title` are not identical, so
this moves some MkDocs nav titles.

- **a. (Recommendation) Yes — delete `firstH1`, call `docparse.Title`, accept
  the delta.** One H1 definition module-wide is what ADR-0001 states and what
  `docwrite.CheckTask` already does. The delta is strictly an improvement:
  `# **Bold** Title` becoming `Bold Title` is the correct rendering, and a
  fenced `# example` no longer being mistaken for a title is a bug fix.
  `DocTitle` prefers frontmatter anyway, so only frontmatter-less files are
  affected — which in this repo means none of the docz documents.
- **b. Yes, but preserve current behavior** by having `Title` not strip inline
  markdown. Avoids the nav delta and makes `Title` worse for its actual
  consumer: docz-site would render `**Bold** Title` in a breadcrumb.
- **c. Leave `firstH1` alone.** Zero risk to the wiki goldens, and it leaves
  two H1 scanners in one module — the exact state this work exists to fix.
- **d. Defer to a follow-up PR** so `v1.2.0` carries no `internal/wiki` change.
  Cleanest release diff; in practice the follow-up competes with docz-api work
  and does not happen.

### 4. How should `Title` treat a setext H1?

```markdown
My Title
========
```

`Headings` is ATX-only and `firstH1` matches only `# `, so docz has never
recognized setext. But `Title`'s inputs are arbitrary repo files — a
`CONTRIBUTING.md` written by anyone — where setext is more common than in
docz-generated documents.

- **a. (Recommendation) Support it.** `Title`'s whole job is reading markdown
  docz did not write, and a `CONTRIBUTING.md` with a setext title returning
  `""` would fall back to the filename for no good reason. The rule is small: a
  non-blank line followed by a line of only `=`. It does **not** touch
  `Headings`, so nothing frozen changes.
- **b. ATX only.** Consistent with `Headings` and `firstH1`, trivially simple,
  documented as a known limitation — but it optimizes for internal consistency
  over the actual use case.
- **c. ATX only for now, revisit if a repo hits it.** Adding it later is
  additive and safe, so this is defensible; it just means the first repo to hit
  it gets a wrong title.

### 5. Should `Title` skip YAML frontmatter?

`wiki.firstH1` toggles on `---` lines to avoid matching inside frontmatter.
`Headings` does not, because H2–H6 do not appear there.

- **a. (Recommendation) Yes, skip a leading `---` block.** A docz document's
  frontmatter contains no `# ` line today, so this is defensive rather than
  load-bearing — but `Title` targets arbitrary files, and frontmatter carrying
  a `#`-prefixed string in a multiline block scalar would otherwise yield
  nonsense. It also preserves `firstH1`'s behavior exactly, which makes Open
  Question 3's collapse safer.
- **b. No — walk the whole input like `Headings`.** Simpler, consistent with
  its sibling, and relies on frontmatter never containing an H1-shaped line.
- **c. Skip frontmatter only when it starts at byte 0.** What (a) means in
  practice; worth stating explicitly so a `---` horizontal rule mid-document is
  never mistaken for a frontmatter opener.

### 6. Where does the consumer proof for the new surface live?

- **a. (Recommendation) A new `test/consumer/consumer_v12_test.go`.** Follows
  the existing precedent exactly — `consumer_test.go` covers the original
  surface and `consumer_v1_test.go` was added for what `v1.0.0` introduced, so
  a per-release file is already the pattern. It also makes "what did `v1.2.0`
  add to the contract" answerable by reading one file.
- **b. Extend `consumer_v1_test.go`.** Fewer files; muddies a file whose header
  explicitly says it is the v1.0.0 surface proof.
- **c. Extend `consumer_test.go`**, the original config+document file. Wrong
  home — `docparse.Title` is neither.

### 7. PR and release strategy for this branch?

- **a. (Recommendation) One `minor` PR carrying all four phases, then a
  post-tag `dont-release` PR for the status flips.** Exactly the IMPL-0015
  shape (PR #78 → `v1.1.0` → PR #79), which has now worked cleanly twice.
  Phases are commit boundaries inside the one PR, so review still reads phase
  by phase.
- **b. Sequential per-phase PRs**, each `dont-release`, with a final `minor`.
  More review checkpoints; four times the CI and merge overhead for work that
  is not risky enough to warrant it. IMPL-0014 planned this and abandoned it
  mid-flight.
- **c. Split config and docparse into two `minor` releases.** Would ship the
  config block without the title extractor — precisely the split INV-0007's
  Recommendation 1 warns against, because docz-api fills the gap with its own
  H1 parser.

### 8. Should Phase 2 extend the existing fixtures or add a new one?

- **a. (Recommendation) Both — extend `renderFacts` so the two existing
  fixtures also report `Title`, and add one new fixture for the edge cases.**
  Extending `renderFacts` gets `Title` coverage over realistic documents for
  free and makes any future change to it visible in an existing golden; the new
  fixture carries the pathological cases that would make `impl_plan.md`
  unrealistic.
- **b. New fixture only.** Keeps the existing goldens untouched — but then
  `Title` is only ever tested against contrived input.
- **c. Extend the existing fixtures only.** No new file, at the cost of
  stuffing fenced-H1 and setext oddities into fixtures meant to read like real
  documents.

## Decisions

All eight open questions resolved **(a)** on 2026-08-11.

| # | Question | Resolution |
| - | -------- | ---------- |
| 1 | `api.exclude` entry rules | Share the validator, tolerate a trailing `/` — exclude entries are directory prefixes, so `templates` and `templates/` mean the same thing. Every safety rule still applies |
| 2 | Shared validator signature | `validateRepoRelativePath(field, value string, ...) error` returning a **bare** error the caller wraps with its own sentinel — `errors.Is(err, ErrInvalidChangelogFile)` must keep working |
| 3 | Collapse `wiki.firstH1`? | Yes — delete it, call `docparse.Title`, accept the nav delta. One H1 definition module-wide, per ADR-0001 |
| 4 | Setext H1 | Support it in `Title` (a non-blank line followed by a line of only `=`). `Headings` is untouched |
| 5 | Frontmatter | Skip a leading `---` block, only when it starts at byte 0. Preserves `firstH1` behavior, making Decision 3's collapse safe |
| 6 | Consumer proof home | New `test/consumer/consumer_v12_test.go` — one file per release, matching the existing pattern |
| 7 | PR / release strategy | One `minor` PR carrying all four phases → `v1.2.0`, then a post-tag `dont-release` PR for the status flips |
| 8 | Fixture strategy | Both — extend `renderFacts` so the existing fixtures report `Title`, and add one new fixture for the edge cases |

### Note: Decisions 1 and 2 interact

Taken together they determine the helper's signature. Decision 2 asks for
`(field, value string)`; Decision 1 needs one rule — "must be a file path, not
a directory" — to be skippable for `exclude` entries. So the helper takes a
third argument:

```go
func validateRepoRelativePath(field, value string, allowDir bool) error
```

`LandingPage`, `AdditionalDocs`, and `changelog.file` pass `false`; `Exclude`
passes `true`. The alternative — a separate `validateRepoRelativeDir` wrapper —
buys a cleaner call site at the cost of two functions that must be kept in sync,
which is the duplication INV-0007 Decision 2 exists to prevent. Flagged here
because neither open question could settle it alone.

## References

- DESIGN-0011 — the design this implements; ten decisions, zero open questions
- INV-0007 — the internals audit; Decision 1 (`docparse.Title`), Decision 2
  (shared path validator), Decision 3 (docz does not construct routes)
- DESIGN-0010 / IMPL-0015 — the `changelog:` precedent this mirrors phase for
  phase, including the `docz_yaml.tmpl` round-trip guard that forces the
  template edit into Phase 1
- DESIGN-0008 — docz-api ingestion; gains contract clause R10 in Phase 3
- ADR-0001 — additive-only, roll-forward semver, one definition module-wide
- IMPL-0014 — the v1.0.0 plan whose per-phase-PR strategy was abandoned
  mid-flight, informing Decision 7
