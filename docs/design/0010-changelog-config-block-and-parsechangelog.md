---
id: DESIGN-0010
title: "Changelog config block and ParseChangelog"
status: Draft
author: Donald Gifford
created: 2026-08-02
---
<!-- markdownlint-disable-file MD025 MD041 -->

# DESIGN 0010: Changelog config block and ParseChangelog

**Status:** Draft
**Author:** Donald Gifford
**Date:** 2026-08-02

<!--toc:start-->
- [Overview](#overview)
- [Goals and Non-Goals](#goals-and-non-goals)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Background](#background)
- [Detailed Design](#detailed-design)
  - [Config](#config)
  - [Parser](#parser)
- [API / Interface Changes](#api--interface-changes)
  - [Downstream contract (what docz-api pins as R6)](#downstream-contract-what-docz-api-pins-as-r6)
- [Data Model](#data-model)
- [Testing Strategy](#testing-strategy)
- [Migration / Rollout Plan](#migration--rollout-plan)
- [Open Questions](#open-questions)
  - [1. Parser package home?](#1-parser-package-home)
  - [2. Date field type?](#2-date-field-type)
  - [3. Preamble fidelity?](#3-preamble-fidelity)
  - [4. Should heading detection be fence-aware?](#4-should-heading-detection-be-fence-aware)
  - [5. Where does file validation live?](#5-where-does-file-validation-live)
  - [6. Should docz init emit the block in generated .docz.yaml?](#6-should-docz-init-emit-the-block-in-generated-doczyaml)
  - [7. Does file validation apply when the block is disabled?](#7-does-file-validation-apply-when-the-block-is-disabled)
  - [8. Are nested sub-bullets part of the parent item or their own item?](#8-are-nested-sub-bullets-part-of-the-parent-item-or-their-own-item)
  - [9. What happens on duplicate version headings?](#9-what-happens-on-duplicate-version-headings)
- [Decisions](#decisions)
- [References](#references)
<!--toc:end-->

## Overview

docz gains first-class awareness of a repo's changelog: an opt-in `changelog:`
block in `.docz.yaml` naming the file, and a byte-based `ParseChangelog` that
parses the fleet-standard git-cliff / Keep-a-Changelog shape into structured
version sections. Consumers are docz-api (fetches, caches, and serves the
changelog per repo; later builds per-document backlinks on the parsed sections)
and, through it, the docz-site.

Requirements were derived in docz-api's INV-0005 ("Changelog as a first-class
docz artifact"); all of its open questions are resolved and this design encodes
the answers.

## Goals and Non-Goals

### Goals

- Add a `changelog:` config block: `enabled` (default **false**) and `file`
  (default **`CHANGELOG.md`**), with defaults surfaced via `DefaultConfig()` and
  per-key merge behavior identical to the rest of the config.
- Add `ParseChangelog([]byte)` — byte-based, no disk I/O (the `ParseFrontmatter`
  precedent) — parsing the fleet-standard shape into versions → groups → items.
- Keep the surface contract-pinnable: docz-api will freeze the config defaults
  and parser behavior in its `internal/doczcontract` (clause R6) on the pin
  bump.
- Ship both in **one additive minor release** — a second release costs a second
  fleet-wide pin dance for no benefit.

### Non-Goals

- No CLI subcommand (`docz changelog …`) — nothing in the CLI consumes the block
  yet; `docz config` printing the resolved block falls out for free.
- No changelog _generation_ — git-cliff owns that. docz only locates and parses.
- No wiki/mkdocs nav integration (possible follow-up).
- No commit→file mapping — that is docz-api "feature 2" (per-document backlinks)
  and depends on data outside the changelog file (GitHub compare API or PR
  joins), designed separately in docz-api.

## Background

- The fleet generates every changelog with **git-cliff** from a shared
  `cliff.toml`, **conventional commits**, and **SemVer** tags, so the input
  shape is uniform:

  ```markdown
  # Changelog

  <preamble prose>

  ## [unreleased]

  ### Bug Fixes

  - _(chart)_ Scope the main Service selector … ([#12](…))

  ## [0.4.2] - 2026-07-23

  ### Bug Fixes

  - _(ci)_ Drop stale goreleaser GPG signing … ([#10](…))
  ```

  Version headers are `## [semver] - YYYY-MM-DD` (plus `## [unreleased]`),
  groups are `### Title` from the shared cliff groups, and each bullet is one
  commit with optional `*(scope)*` and PR links. **No commit SHAs or file paths
  appear in the body** (that fact scoped feature 2 out).

- **Fleet file locations** (drives the `file` semantics): either `CHANGELOG.md`
  at the repo root, or `charts/<chart-name>/CHANGELOG.md` for helm-chart
  changelogs. Those are the only shapes in use; subpath support exists precisely
  for the chart case.

- **Rollout tolerance (verified in INV-0005 F2):** docz v1.0.0's yaml.v3-based
  `Load` silently ignores the unknown `changelog:` key, so repos can add the
  block before this ships with zero breakage. Preserve that property (see
  Testing).

- docz-api today already fetches a hardcoded root `CHANGELOG.md` and caches it
  raw; this design is what lets it become config-driven and eventually
  structured.

## Detailed Design

### Config

```go
// ChangelogConfig maps the changelog: block of .docz.yaml.
type ChangelogConfig struct {
    // Enabled opts the repo into changelog mapping. Default false.
    Enabled bool `yaml:"enabled"`
    // File is the changelog path relative to the repo root. Subpaths
    // are allowed (charts/<name>/CHANGELOG.md). Default "CHANGELOG.md".
    File string `yaml:"file"`
}
```

- `Config` gains `Changelog ChangelogConfig \`yaml:"changelog"\``.
- `DefaultConfig()` returns `{Enabled: false, File: "CHANGELOG.md"}`.
- **Partial-block merge:** `changelog: {enabled: true}` with no `file` must
  resolve `File` to the default. The omitted-key case is free — `Load` decodes
  the merged map onto a pre-populated `DefaultConfig()`, like every other
  block. An **explicitly empty** `file: ""` must also resolve to the default,
  and that needs a small new post-decode normalization: no existing top-level
  block does empty→default backfill (`fillTypeFieldDefaults` exists only
  because `Types` is a map whose values decode from zero). An empty `file:`
  value is the default, never "".
- **Validation** (Decisions 5/7): a hard error in `Config.Validate()`,
  applied **only when `enabled: true`** — a dormant block never fails load,
  preserving the rollout guarantee. `file` must be a clean relative path —
  reject absolute paths, `..` traversal, and trailing `/`; normalize a leading
  `./`. Consumers fetch this path out of a git tree, so cleanliness is the
  contract.

### Parser

Home (Decision 1): `pkg/doczcore/document`, alongside `ParseFrontmatter` —
the same typed-struct / bytes-in / sentinel-error shape as the precedent, and
docz-api gets both parsers through the one `doczdoc` import it already has.

```go
// Changelog is a parsed git-cliff / Keep-a-Changelog document.
type Changelog struct {
    // Preamble is the markdown before the first version heading
    // (title + prose), verbatim.
    Preamble string
    // Versions in document order (git-cliff emits newest first).
    Versions []ChangelogVersion
}

type ChangelogVersion struct {
    // Version is the bare version string: brackets stripped, a single
    // leading "v" trimmed ("0.4.2"), or "unreleased".
    Version    string
    Unreleased bool
    // Date is the raw "YYYY-MM-DD" from the heading; empty for
    // unreleased. Kept as a string — consumers parse if they need to.
    Date   string
    Groups []ChangelogGroup
}

type ChangelogGroup struct {
    Title string   // e.g. "Bug Fixes"
    Items []string // raw markdown bullet bodies, one per commit
}

// ErrNoVersions is returned when the input has no version headings —
// the file is not a changelog (or is empty). Callers decide skip vs
// fail, mirroring ErrNoFrontmatter.
var ErrNoVersions = errors.New(...)

func ParseChangelog(raw []byte) (*Changelog, error)
```

Parsing rules:

- A version heading is a line matching `## [<ver>]` or `## [<ver>] - <date>`; a
  `<ver>` of `unreleased` (case-insensitive) sets `Unreleased: true`.
- Heading detection is **fence-aware** (Decision 4): lines inside fenced code
  blocks never open a version or group, using the module's existing fence rule
  (trimmed-line ` ``` ` toggle) as an internal helper — no `docparse`
  dependency required.
- `### <title>` inside a version opens a group; bullet lines (`- …`) append to
  the open group's `Items` with the leading `-` marker and its following space
  stripped and the raw markdown preserved (scope markers, PR links untouched).
  Everything indented under a top-level bullet — wrapped continuation prose
  *and* nested sub-bullets — folds into that item's string verbatim, their own
  `-` markers preserved (Decision 8), keeping one `Items` entry per top-level
  commit bullet.
- Duplicate version headings are emitted as separate `ChangelogVersion`
  entries verbatim, in document order (Decision 9) — the parser never
  editorializes; docz-api's contract pins uniqueness on the fleet shape and
  its join logic treats the first occurrence as canonical. Stated in the
  parser's doc comment.
- Content inside a version but before any `###` heading is collected into a
  group with `Title: ""` (does not occur in the fleet shape, but the parser must
  not lose it).
- **Never panic on arbitrary input**; error only via `ErrNoVersions` (wrapped).
  Non-conforming markdown parses best-effort.
- **Stable version identity is the load-bearing contract**: the bare semver
  string is the key docz-api's feature 2 (backlinks) will join on. Normalization
  (bracket strip + `v` trim) must never change meaning.

## API / Interface Changes

All additive (one minor release, e.g. `v1.1.0`):

- `config`: `ChangelogConfig`, `Config.Changelog`, default values, validation.
- `document`: `Changelog`, `ChangelogVersion`, `ChangelogGroup`,
  `ErrNoVersions`, `ParseChangelog` (Decision 1).
- `docz init`: the generated `.docz.yaml` gains the block (`enabled: false` +
  a short comment) via the embedded `docz_yaml.tmpl` (Decision 6), so the
  feature is discoverable; `docz config` prints the resolved block for free.
- No changes to existing symbols; no breaking changes.

### Downstream contract (what docz-api pins as R6)

On its pin bump docz-api will add contract tests asserting:

- yaml keys `changelog.enabled` / `changelog.file`; defaults `false` /
  `"CHANGELOG.md"`; partial-block merge keeps the file default.
- Unknown-key tolerance of `Load` (the INV-0005 F2 rollout guarantee) stays
  intact.
- `ParseChangelog` over the fleet fixture: version order and identity, group
  titles, item counts, preamble capture; `ErrNoVersions` identity via
  `errors.Is` on a no-heading input.

Treat those as frozen once released — changes to any of them are breaking for
consumers regardless of Go-API compatibility.

## Data Model

None — docz holds no state. (docz-api maps the parsed shape onto its own store
when it builds structured serving; not this repo's concern.)

## Testing Strategy

Table-driven with golden fixtures:

- **Fleet-standard fixture** — a real git-cliff output (docz-api's
  `CHANGELOG.md` is representative): preamble + unreleased + several released
  versions, scopes, PR links.
- **Chart changelog fixture** — the `charts/<name>/CHANGELOG.md` shape (same
  format; exercises nothing special in the parser but pins the convention).
- **Edge fixtures** — empty input, prose-only input (both `ErrNoVersions`);
  unreleased-only; a version with no groups; bullets before any group heading;
  multi-line bullet continuation.
- **Config table** — absent block (defaults), partial block (`enabled` only),
  explicit empty `file: ""` (backfills to default), full block, invalid `file`
  values (absolute, `..`, trailing slash) rejected, leading-`./` normalized,
  unknown sibling keys still tolerated.

## Migration / Rollout Plan

1. Ship config + parser in one additive minor release.
2. Repos may add the `changelog:` block **before or after** the release (v1.0.0
   already tolerates it; the block is dormant until consumers act on it).
3. docz-api bumps its pin per its INV-0001 procedure, adds contract clause R6,
   then builds its feature 1 (config-driven fetch + raw serve endpoint) and
   later feature 2 (backlinks over parsed sections).
4. No migration inside docz — the block is opt-in and default-off.

## Open Questions

### 1. Parser package home?

**Resolved: (a)** — locked 2026-08-02; see [Decisions](#decisions).


The handoff doc leans toward the doc package; docz's actual v1.0.0 layout has
three candidate homes with different contract implications.

- **a. (Recommended)** `pkg/doczcore/document`, next to `ParseFrontmatter`.
  Same shape as the precedent: typed domain struct, bytes-in, sentinel error
  (`ErrNoVersions` mirrors `ErrNoFrontmatter`), and docz-api gets both parsers
  through the one `doczdoc` import it already has.
- b. A new sibling package `pkg/doczcore/changelog`. Cleanest cohesion (the
  changelog types share nothing with frontmatter), but adds a sixth public
  semver-governed package and a second import for every consumer.
- c. `pkg/doczcore/docparse`. It is markdown parsing, but it would break
  docparse's frozen contract — no error returns, facts-not-interpretation —
  since `Changelog` is a domain model with a sentinel error, not a raw fact.
- d. Other.

### 2. `Date` field type?

**Resolved: (a)** — locked 2026-08-02; see [Decisions](#decisions).


- **a. (Recommended)** Raw string, as specced — no timezone/format opinions;
  the contract pins the raw `YYYY-MM-DD` text and consumers parse if they need
  `time.Time`. Revisit only if a second consumer needs parsed dates.
- b. Parse to `time.Time` at parse time — convenient for consumers but forces
  a timezone decision, and a malformed date would either error (breaking
  best-effort parsing) or zero out (losing the raw text).
- c. Other.

### 3. Preamble fidelity?

**Resolved: (a)** — locked 2026-08-02; see [Decisions](#decisions).


- **a. (Recommended)** Verbatim capture, as specced — consumers render it, so
  the parser must not editorialize (no trimming, no normalization).
- b. Trimmed (leading/trailing whitespace stripped) — marginally tidier for
  API payloads, but loses byte fidelity and the consumer can trim on render
  anyway.
- c. Other.

### 4. Should heading detection be fence-aware?

**Resolved: (a)** — locked 2026-08-02; see [Decisions](#decisions).


The spec detects version/group headings by line shape. A fenced code block in
the preamble or in a bullet that happens to contain a `## [1.0.0]` line (e.g. a
changelog-about-changelogs, or example output in release notes) would open a
phantom version. docz already has a frozen fence rule (`docparse`'s
trimmed-line ` ``` ` toggle).

- **a. (Recommended)** Yes — skip heading detection inside fenced blocks using
  the same trimmed-``` toggle rule the rest of the module uses (an internal
  helper; no dependency on `docparse` required). Cheap, consistent
  module-wide, and removes the only realistic false-positive class.
- b. No — pure line matching. The fleet's git-cliff output never fences
  heading-shaped lines, and best-effort parsing tolerates weird output; keeps
  the parser a few lines simpler.
- c. Other.

### 5. Where does `file` validation live?

**Resolved: (a)** — locked 2026-08-02; see [Decisions](#decisions).


The handoff says "`Load`'s existing validation pass", but in docz v1.0.0
`Load` only merges/decodes — validation lives in `Config.Validate()` (which
returns warnings and a hard error, called by the CLI after load, and by
docz-api the same way).

- **a. (Recommended)** Hard error in `Config.Validate()` — consistent with how
  resolution collisions and other config mistakes fail fast; an absolute or
  traversing `file:` is a config bug the user must fix, and consumers fetching
  from a git tree must be able to trust the path.
- b. Warning in `Validate()` + treat the block as disabled — softer rollout,
  but silently ignoring an explicitly-enabled block hides the mistake.
- c. Validate inside `Load` itself — matches the handoff wording but breaks
  docz's load-then-validate separation and would be the only validation done
  there.
- d. Other.

### 6. Should `docz init` emit the block in generated `.docz.yaml`?

**Resolved: (a)** — locked 2026-08-02; see [Decisions](#decisions).


`docz config` prints the resolved block for free once the field exists. But
`docz init` renders `.docz.yaml` from the embedded `docz_yaml.tmpl` +
`DefaultConfig()`, so whether new repos *see* the block is a template choice.

- **a. (Recommended)** Yes — include the block in the generated `.docz.yaml`
  with `enabled: false` and a short comment (mirrors how the template surfaces
  other optional blocks), so the feature is discoverable without reading docs.
- b. No — leave `docz init` output unchanged; document the block in the README
  configuration section only. Smaller template churn, less discoverable.
- c. Other.

### 7. Does `file` validation apply when the block is disabled?

**Resolved: (a)** — locked 2026-08-02; see [Decisions](#decisions).


The design leans twice on a dormancy guarantee ("repos can add the block …
with zero breakage"; "the block is dormant until consumers act on it"), but
the validation rule as written is unconditional — a repo carrying
`changelog: {enabled: false, file: "../x"}` (mid-edit leftovers, testing
debris) would hard-fail config load even though the feature is off.

- **a. (Recommended)** Validate only when `enabled: true`. Matches docz's
  existing posture — resolution validation runs over the *enabled* type set
  only — and preserves the dormancy guarantee; the malformed path surfaces at
  the moment the user flips the block on, which is when they're editing it
  anyway.
- b. Always validate the path shape, even disabled — catches the config bug
  earliest, but a dormant block can now break `docz` entirely, contradicting
  the rollout story.
- c. Always validate but only as a `Validate()` *warning* while disabled,
  hard error when enabled — most informative, slightly more code.
- d. Other.

### 8. Are nested sub-bullets part of the parent item or their own item?

**Resolved: (a)** — locked 2026-08-02; see [Decisions](#decisions).


The continuation rule ("including continuation lines indented under the
bullet") is ambiguous for an indented `- sub-item` line: it is indented under
the bullet, but it's a nested list item, not wrapped prose. The answer decides
whether `Items` stays one-entry-per-commit (the shape the stable-identity
contract protects) when git-cliff or a human emits a nested list.

- **a. (Recommended)** Fold everything indented under a top-level bullet —
  wrapped prose *and* nested sub-bullets — into that item's string verbatim
  (their own `-` markers preserved). Keeps one `Items` entry per top-level
  commit bullet, loses nothing, and consumers render markdown anyway.
- b. Every `-` line at any indent is its own item — simpler scanner, but a
  single commit with a nested list explodes into multiple `Items` entries and
  breaks the one-item-per-commit shape.
- c. Other.

### 9. What happens on duplicate version headings?

**Resolved: (a)** — locked 2026-08-02; see [Decisions](#decisions).


Version identity is "the load-bearing contract" (docz-api's backlinks join on
the bare version string), but the design never says what `ParseChangelog` does
if the same normalized version appears twice (bad manual edit, re-tag).

- **a. (Recommended)** Emit both `ChangelogVersion` entries verbatim in
  document order — the parser reports what the file says and never
  editorializes (consistent with best-effort parsing); docz-api's contract
  fixture pins uniqueness on the fleet shape, and its join logic treats the
  first occurrence as canonical. Document this in the parser's doc comment.
- b. Dedupe in the parser (first or last occurrence wins) — protects the join
  key but silently discards content the file actually contains.
- c. Return an error — treats a malformed-but-parseable file as fatal,
  breaking the "never fail on non-conforming markdown, best-effort" rule.
- d. Other.

## Decisions

All nine open questions resolved **(a)** on 2026-08-02.

| #   | Question                        | Resolution                                                                                                                                  |
| --- | ------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | Parser package home             | `pkg/doczcore/document`, next to `ParseFrontmatter` — one import for consumers; keeps `docparse`'s no-error facts contract intact            |
| 2   | `Date` type                     | Raw `YYYY-MM-DD` string; consumers parse if they need `time.Time`                                                                            |
| 3   | Preamble fidelity               | Verbatim capture — the parser never editorializes what consumers render                                                                      |
| 4   | Fence-aware heading detection   | Yes — the module's trimmed-``` toggle rule as an internal helper; fenced heading-lookalikes never open a version/group                       |
| 5   | `file` validation home          | Hard error in `Config.Validate()` (not `Load`, which only merges/decodes)                                                                    |
| 6   | `docz init` template            | Generated `.docz.yaml` gains the block, `enabled: false` + comment, via `docz_yaml.tmpl`                                                     |
| 7   | Validation when disabled        | Validate only when `enabled: true` — a dormant block never fails load, preserving the rollout guarantee                                      |
| 8   | Nested sub-bullets              | Fold everything indented under a top-level bullet into that item verbatim — one `Items` entry per commit bullet                              |
| 9   | Duplicate version headings      | Emit both entries verbatim in document order; docz-api's contract pins uniqueness and treats first occurrence as canonical                   |

## References

- docz-api `INV-0005` — Changelog as a first-class docz artifact (the
  requirements source; all OQs answered `a`)
- docz-api `DESIGN-0003` / `IMPL-0003` — repo index endpoint (the
  repo-level-artifact consumption pattern feature 1 copies)
- docz-api `internal/doczcontract` — the contract-test harness that will pin
  this surface (clause R6)
- Keep a Changelog 1.1.0; git-cliff + the fleet-shared `cliff.toml`
