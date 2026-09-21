---
id: INV-0006
title: "Per-package core requirements: docz CLI vs docz-api, docz-site, sdk-booty-sh"
status: Open
author: Donald Gifford
created: 2026-07-03
---
<!-- markdownlint-disable-file MD025 MD041 -->

# INV 0006: Per-package core requirements: docz CLI vs docz-api, docz-site, sdk-booty-sh

**Status:** Open
**Author:** Donald Gifford
**Date:** 2026-07-03

<!--toc:start-->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
  - [Observation 1 — the consumer demand set, from primary sources](#observation-1--the-consumer-demand-set-from-primary-sources)
  - [Observation 2 — per-package matrix: cmd/ requires vs consumers require](#observation-2--per-package-matrix-cmd-requires-vs-consumers-require)
  - [Observation 3 — the packages nobody outside asks for are also the CLI-shaped ones](#observation-3--the-packages-nobody-outside-asks-for-are-also-the-cli-shaped-ones)
  - [Observation 4 — Create is the only coupling between the write side and the template engine](#observation-4--create-is-the-only-coupling-between-the-write-side-and-the-template-engine)
  - [Observation 5 — toc vs docparse: one concern splits cleanly in half](#observation-5--toc-vs-docparse-one-concern-splits-cleanly-in-half)
  - [Observation 6 — dependency and embed audit: the four-package core is clean](#observation-6--dependency-and-embed-audit-the-four-package-core-is-clean)
  - [Observation 7 — the wholesale-move failure mode has already happened once, in miniature](#observation-7--the-wholesale-move-failure-mode-has-already-happened-once-in-miniature)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
- [Open Questions](#open-questions)
  - [1. Scope of the v1.0 public core?](#1-scope-of-the-v10-public-core)
  - [2. Where does Create live at v1.0?](#2-where-does-create-live-at-v10)
  - [3. One parser — does internal/toc delegate to docparse?](#3-one-parser--does-internaltoc-delegate-to-docparse)
  - [4. The already-public CLI-shaped config helpers at v1.0?](#4-the-already-public-cli-shaped-config-helpers-at-v10)
- [Decisions](#decisions)
- [References](#references)
<!--toc:end-->

## Question

For every package the docz CLI is built from — `pkg/doczcore/{config,document}`
(public since `v0.5.0`), `internal/{docwrite,template,index,toc,wiki}`, and the
planned `pkg/doczcore/docparse` (`v0.6.0`) — exactly which exported surface does
(a) `cmd/` require for the CLI to keep working as it does today, and (b) each
external consumer (**docz-api**, **docz-site**, **sdk-booty-sh**) require?

Concretely: does any external consumer need the embedded templates, README
index generation, ToC splicing, or MkDocs wiki machinery — or is the
demand-complete public core exactly the four parse/config/write-primitive
packages (`config`, `document`, `docparse`, `docwrite`)?

## Hypothesis

External consumers need only the parsing/config core plus the two byte-level
write primitives (`SetStatus`, `CheckTask`). Embedded templates, README index
tables, ToC splicing, and MkDocs nav are **CLI presentation concerns** with
zero external demand. If confirmed, ADR-0001's "move everything public" scope
is wider than demand, and the idiomatic Go resolution is a smaller frozen
`pkg/doczcore` core with the presentation packages remaining in `internal/` —
the CLI goal ("works exactly as today") does not require promoting them.

## Context

ADR-0001 (Proposed) would end the per-consumer promotion drip by moving all
five remaining `internal/` packages into `pkg/doczcore/*` and cutting `v1.0.0`
as a semver freeze, with `cmd/` reduced to a thin shell. Its own "Demand
mismatch" consequence flags that the known consumers never asked for
`template`, `index`, or `wiki` — but that claim was stated from memory, not
audited. This investigation grounds it symbol-by-symbol against the code and
the three consumer requirement sources, so the ADR's scope (and its Open
Question 3) can be resolved on evidence.

**Triggered by:** ADR-0001; DESIGN-0008 (docz-api), DESIGN-0009 (docz-site),
the sdk-booty-sh `v0.6.0` requirements handoff (`docz-v0.6.0-requirements.md`).

## Approach

1. Enumerate the exported surface of each package with `go doc -all` (the two
   public packages plus all five `internal/` packages).
2. Map `cmd/`'s actual per-symbol usage:
   `grep -oE '(doctemplate|index|toc|wiki|docwrite)\.[A-Z]\w*' cmd/*.go`
   (non-test files only), plus the import graph of `internal/*`.
3. Extract each consumer's requirements from its primary source: DESIGN-0008
   Requirements R1–R9 (including its exact-symbol table), DESIGN-0009's
   decisions and non-goals, and `docz-v0.6.0-requirements.md` R1–R3.
4. Diff the requirement sets per package; audit third-party dependencies and
   `//go:embed` usage to check what each promotion would drag into a
   consumer's build.

## Environment

| Component | Version / Value |
|-----------|----------------|
| docz tree | post-`v0.5.0`, commit `eb5c027` (branch `docs/adr-doczcore-v1-single-core`) |
| Go | 1.26.4 |
| Consumer sources | DESIGN-0008 (docz-api), DESIGN-0009 (docz-site, Draft), `docz-v0.6.0-requirements.md` (sdk-booty-sh handoff) |

## Findings

### Observation 1 — the consumer demand set, from primary sources

**docz-api** (DESIGN-0008, R1–R9) is read-only and names its exact surface in
the R2 table:

- `doczcore/config`: `Load`, `(*Config).Validate`, `EnabledTypes`, `TypeDir`,
  `ValidateType`; types `Config`, `TypeConfig` (fields `Enabled`, `Dir`,
  `IDPrefix`, `IDWidth`, `Statuses`, `PluralLabel`, `Aliases`), `DocType`,
  `Status`; sentinel `ErrUnknownType`.
- `doczcore/document`: `ParseFrontmatter` (bytes-in is load-bearing — R3, the
  no-checkout ingest path), `IsDoczFile`; types `Frontmatter`, `DocEntry`
  (with `Content []byte` — R5); sentinel `ErrNoFrontmatter`. `ScanDocuments` /
  `LoadFrontmatter` are nice-to-have only.
- Explicitly **not** required: `docz export --json` (R8), any write API
  (ingest is read-only, Decisions 4/5).

**sdk-booty-sh** (`docz-v0.6.0-requirements.md`, R1–R3):

- R1: `config.Load` with the same signature, **viper-free** (yaml.v3-only), so
  importing `pkg/doczcore/...` compiles no viper tree.
- R2: a new public `pkg/doczcore/docparse` — `Headings` (with 1-based `Line`,
  which `internal/toc.Heading` lacks) and `ParsePlan` (phases + checkbox tasks
  with `Checked` and `Line`).
- R3: promote `docwrite` with `SetStatus` as-is and a new `CheckTask(path,
  line)` — byte-preserving, LF-only. The write surface is deliberately
  "status + checkbox only"; a general markdown editor is a stated non-goal.
  **`Create` is not requested.**

**docz-site** (DESIGN-0009) imports **no Go code at all**. It is a
TypeScript thin client of docz-api: markdown is rendered client-side
(markdown-it/remark + DOMPurify, Decision 3), the reader builds its own ToC
from rendered headings and *strips* docz's `<!--toc:start-->` markers (OQ8a),
and per-repo MkDocs wiki output is explicitly complementary, not consumed.

Across all three consumers there is **zero demand** for `template`, `index`,
`wiki`, or the ToC-splice half of `toc`.

### Observation 2 — per-package matrix: `cmd/` requires vs consumers require

| Package | `cmd/` requires (non-test call sites) | docz-api | sdk-booty-sh | docz-site | Verdict |
|---|---|---|---|---|---|
| `pkg/doczcore/config` | every command file: `Load`, `DefaultConfig`, `Validate`, `ValidateType`, `EnabledTypes`, `TypeDir`, `DocTypeNames`, `TypesHelp`, `DefaultNavTitles`, registry lookups, constants | R2 symbol list, R4 type resolution, R7 stable schema | R1 viper-free `Load` | — (via API) | **public** (already); `v0.6.0` removes viper |
| `pkg/doczcore/document` | `list`/`status`/`update`: `ScanDocuments`, `LoadFrontmatter`, `IsDoczFile`, types | R2/R3/R5 (`ParseFrontmatter` bytes-in, `DocEntry.Content`) | baseline reads | — | **public** (already); stays read-only |
| `internal/docwrite` | `create.go`: `Create`/`CreateOptions`; `status.go`: `SetStatus` + `ErrUnsupportedLineEndings` | none (read-only) | R3: `SetStatus` + `CheckTask` — **not `Create`** | — | **split**: `SetStatus`/`CheckTask` public (`v0.6.0`); `Create` has no external demand |
| `internal/template` | `init` (`EmbeddedDoczYAML`, `ResolveIndexHeader`), `template` (`Resolve`), `update` (`ResolveIndexHeader`), `wiki` (`ResolveWikiIndex`, `RenderWikiIndex`, `WikiIndex*`); via `docwrite.Create` (`Resolve`, `Render`, `FilenameSlug`) | none | none | renders client-side | **internal** |
| `internal/index` | `update.go`: `GenerateTable`, `UpdateReadme`, `DryRunReadme`, `UpdateOutcome` + `Action` enum | none | none | none | **internal** |
| `internal/toc` | `update.go`: `FileInput`, `UpdateFiles`, markers; `wiki.go`: `BeginMarker`/`EndMarker` | none | heading **walk** only, generalized as `docparse` with `Line` | builds own ToC; strips markers | walk → **`docparse` (public)**; splice stays **internal** |
| `internal/wiki` | `wiki.go` only: `ScanDocs`, `BuildNav`, `MergeNavOrder`, `NavToYAML`, `ReadMkDocs`/`WriteMkDocs`, `CreateMkDocs`, titles | none | none | complementary non-goal | **internal** |
| `pkg/doczcore/docparse` (planned) | none today (CLI adoption is an explicit `v0.6.0` non-goal) | — | R2 (the driver) | — | **public** (new, `v0.6.0`) |

The CLI's requirement set and the consumers' requirement set overlap **only**
on `config`, `document`, `docwrite`'s status primitive, and (indirectly, once
`toc`'s walker generalizes) `docparse`. Everything else `cmd/` needs is
presentation: rendering templates, splicing marker blocks, formatting index
tables, writing MkDocs nav.

### Observation 3 — the packages nobody outside asks for are also the CLI-shaped ones

The three unpromoted-by-demand packages are exactly the ones whose APIs encode
CLI decisions, confirming ADR-0001's "CLI-shaped" concern:

- `index.UpdateOutcome`'s `Action` enum (`ActionCreated`, `ActionNoMarkers`,
  `ActionDryRunUpdated`, …) exists so `cmd/update.go` can choose user-facing
  wording — six of its consts appear in `cmd/` conditionals.
- `wiki`'s entire surface is MkDocs-specific (`CreateMkDocs`, `ReadMkDocs`,
  `WriteMkDocs`, `NavToYAML`) and exists for `cmd/wiki.go` alone.
- `template`'s embed accessors are barely used even in-repo:
  `EmbeddedDocumentTemplate` and `EmbeddedWikiIndex` are called **only inside
  the package itself** (tier-3 fallbacks of `Resolve`/`ResolveWikiIndex`);
  the single direct `cmd/` embed call is `EmbeddedDoczYAML` in `init.go`.
  The embedded FS is already an implementation detail — keeping it internal
  costs nothing and avoids freezing template names/contents (ADR-0001 Corner
  Case 1 dissolves rather than needing mitigation).

Freezing these under semver would buy no consumer anything and would convert
every future CLI UX change (new outcome wording, a non-MkDocs wiki backend, a
template layout change) into a public-API question.

### Observation 4 — `Create` is the only coupling between the write side and the template engine

The single cross-`internal` edge in the tree is `docwrite → template`
(`internal/docwrite/create.go` imports `internal/template` for `Resolve`,
`Render`, `FilenameSlug`). Consequences:

- The `v0.6.0` public `docwrite` (`SetStatus` + `CheckTask`) is
  **stdlib-only** and byte-splice-shaped — it never touches the template
  engine or the embed.
- Promoting `Create` does **not** force `template` public: a public package
  may import its own module's `internal/` packages (standard Go; the stdlib
  does this pervasively). A public `docwrite.Create` keeps rendering via
  `internal/template`; consumers compile the embed into their binaries but
  cannot import it, and template names/contents stay out of the contract.
  _(An earlier draft of this observation claimed the promotion "forces
  `template` public" — that was wrong and is corrected here.)_
- `CreateOptions`/`CreateResult` expose only `config` types, primitives, and
  `time.Time` (verified against `create.go`); `internal/template` types
  appear only in the function body, so the promoted signature leaks no
  internal type.
- What promoting `Create` **does** freeze is its observable behavior: the
  `NNNN-slug.md` filename convention and the auto-increment scan — both
  already de facto public via `document.DoczFilePattern`.

### Observation 5 — `toc` vs `docparse`: one concern splits cleanly in half

`internal/toc` is two concerns in one package:

1. a **heading walk** (`ParseHeadings`, `Heading`, `AnchorSlug`) — this is
   what sdk-booty-sh needs generalized, but with line numbers `toc.Heading`
   doesn't carry, so `v0.6.0` R2 already specifies a richer `docparse.Heading`
   rather than reusing it;
2. a **ToC splice** (`GenerateToC`, `UpdateToC`, `UpdateFiles`, the
   `BeginMarker`/`EndMarker` consts) — used by `cmd/update.go` and
   `cmd/wiki.go`, and by nothing external. docz-site deliberately ignores the
   marker blocks (DESIGN-0009 OQ8a strips them).

So the public parser is `docparse`; the splice is CLI presentation. The only
open point is whether `internal/toc` keeps its own walker or delegates to
`docparse` (Open Question 3) — a private refactor either way, invisible to
consumers and to CLI behavior.

### Observation 6 — dependency and embed audit: the four-package core is clean

- After `v0.6.0` R1 (viper removal), the demand-complete core
  `pkg/doczcore/{config,document,docparse,docwrite}` depends on
  `go.yaml.in/yaml/v3` (config, document) and stdlib **only**; `docparse` and
  `docwrite` are stdlib-only by their specs.
- The heavyweight bits stay behind: the one `//go:embed` lives in
  `internal/template`; `internal/wiki` carries the only other `yaml.v3`
  import; `index` and `toc` are stdlib-only but CLI-shaped.
- A consumer importing `config`/`document`/`docparse` pulls no template
  engine, no embed, no MkDocs machinery — the acceptance criterion
  sdk-booty-sh already states for R1. (`docwrite`, once it includes `Create`,
  compiles `internal/template` + the embed into the consumer's binary via
  the legal public→internal import, but exposes none of it — see
  Observation 4.)

### Observation 7 — the wholesale-move failure mode has already happened once, in miniature

The `v0.5.0` wholesale move of `config` carried some CLI-display helpers into
the public, semver-governed surface: `TypesHelp()` (literally the `docz
--help` body), `DefaultNavTitles()` (the wiki nav-title map), and
`ResolveTypeAlias`. No consumer uses them; they are now frozen anyway. This is
the exact failure mode ADR-0001's full promotion would repeat at 10× scale
with `template`/`index`/`wiki` — evidence that "promote wholesale, review
lightly" (ADR-0001 OQ3a) leaks CLI shapes into the contract. What to do with
these three already-public symbols at v1.0 is Open Question 4.

## Conclusion

**Answer: Yes — the hypothesis is confirmed.**

The demand-complete public core is exactly
`pkg/doczcore/{config,document,docparse,docwrite}` — which is precisely where
the tree stands after `v0.6.0` ships. Symbol-by-symbol, no external consumer
needs the embedded templates, README index generation, ToC splicing, or MkDocs
wiki machinery: docz-api's R2 table names only `config` + `document` symbols,
sdk-booty-sh's slate adds only `docparse` + `docwrite`'s two byte-splice
primitives, and docz-site imports no Go at all.

The two goals in play — **the CLI works exactly as today** and **external
services build on a shared core** — do not require promoting
`template`/`index`/`toc`/`wiki`. The CLI is satisfied by `cmd/` + `internal/`
presentation layered over the public core; the consumers are satisfied by the
four-package core alone. ADR-0001's full-promotion scope is therefore wider
than either goal demands, and its cost (freezing CLI-shaped APIs under semver)
buys consistency, not capability. The idiomatic Go shape for this repo is the
standard library-with-CLI layout: a deliberate, frozen `pkg/` surface plus an
`internal/` that exists precisely to keep product-private code out of the
contract.

## Recommendation

1. **Revise ADR-0001 before acceptance**: scope v1.0.0 as the freeze of the
   four-package core; `template`, `index`, `toc` (splice), and `wiki` remain
   `internal/`, and `docwrite` promotes **whole** (incl. `Create`,
   implemented over `internal/template` — Observation 4). This resolves
   ADR-0001 OQ3 (presentation stays internal; cohesion for `docwrite`),
   makes OQ6 (embed publicity) moot, and reframes "thin `cmd/`" as "no *core
   logic* in `cmd/`" rather than "no `internal/` at all.
   _(Done 2026-07-03 — ADR-0001 revised; see its Decisions table.)_
2. **Ship `v0.6.0` first, unchanged** (ADR-0001 OQ2a) — it already builds the
   entire demand-complete core and unblocks sdk-booty-sh.
3. **v1.0.0 becomes mostly a contract release**: the freeze declaration, API
   doc hygiene and package docs over the four packages, expanding
   `test/consumer/` to exercise the full v1.0 surface (incl. `docparse` +
   `docwrite`), a stance on the already-public CLI-shaped config helpers
   (Open Question 4), and the CLI-stability note.
4. Resolve the Open Questions below, then fold the outcome into ADR-0001's
   Decision section and cut the implementation plan.

## Open Questions

> Each question is numbered; option `a` is my recommendation, later letters
> are alternatives, and the final letter is a free-form "other."

### 1. Scope of the v1.0 public core?

> **Resolved 2026-07-03: (a)** — via the promotion rule adopted in ADR-0001
> Decision 1 ("public by default where demand exists, whole packages, named
> exceptions"): `template`/`index`/`toc` splice/`wiki` have nothing external
> demand requires, so they stay `internal/`.

- **a. (Recommended)** **Four-package core**
  `pkg/doczcore/{config,document,docparse,docwrite}`; `template`/`index`/
  `toc`/`wiki` stay `internal/`. Demand-complete, zero unwanted semver
  surface, CLI unchanged, idiomatic `internal/` use.
- b. Full promotion per the ADR-0001 draft (all packages public, `cmd/`
  imports only `pkg/doczcore/*`).
- c. Full promotion with `template`/`index`/`wiki` documented as
  experimental/exempt from the compatibility guarantee (ADR-0001 OQ3c).
- d. Other.

### 2. Where does `Create` live at v1.0?

> **Resolved 2026-07-03: (b)** — package cohesion beats surface minimalism:
> splitting `docwrite` to hide one function leaves a confusing twin-package
> layout (public `docwrite` + an internal rump), and `Create` is plausibly
> wanted by future consumers anyway. The cost cited against (b) in the
> original draft was overstated — see Observation 4: the template engine
> stays private behind the public→internal import.

- a. **Stays internal.** When `SetStatus`/`CheckTask` move out in `v0.6.0`,
  what's left of `internal/docwrite` is `Create` alone — keep it (optionally
  renamed, e.g. `internal/doccreate`). Minimal surface, but a twin-package
  layout every contributor must re-learn.
- **b. (Chosen)** Promote `Create` into public `docwrite` at v1.0 — the whole
  package promotes; the embed rides along in consumer *builds* via the legal
  public→internal import, while `template`'s API, names, and contents stay
  private (Observation 4).
- c. Defer: promote `Create` only when a consumer asks (a normal additive
  minor later, possibly with a template-injection seam instead of the embed).
- d. Other.

### 3. One parser — does `internal/toc` delegate to `docparse`?

> **Resolved 2026-07-03: (a)** — delegate; and the export-lean ADR-0001
> revision goes further: the splice itself promotes to `pkg/doczcore/toc`
> (superseding this INV's demand-only "stays internal" verdict), with
> `docparse` as the single public parser and `toc`'s own walker retired.

- **a. (Recommended)** **Delegate.** Refactor `internal/toc` to consume
  `docparse.Headings` for its walk (keeping the splice + markers internal),
  during or immediately after the `v0.6.0` work so the two walkers never
  drift on fence/slug behavior. Private refactor; invisible to consumers and
  CLI output.
- b. Keep two walkers (`toc` as-is) — allowed by the `v0.6.0` handoff
  ("internal/toc can stay as-is or delegate"), but risks slug/fence
  divergence between what the CLI splices and what consumers parse.
- c. Other.

### 4. The already-public CLI-shaped `config` helpers at v1.0?

`TypesHelp()`, `DefaultNavTitles()`, and `ResolveTypeAlias` shipped with the
`v0.5.0` wholesale move; no consumer uses them.

> **Resolved 2026-07-03: (a)** — keep, consistent with the export-lean rule;
> document them as CLI-support helpers in the package doc.

- **a. (Recommended)** **Keep them** — they're small, already frozen, and
  harmless; document them as CLI-support helpers in the package doc so the
  core's intent stays legible.
- b. Deprecate at v1.0 (`Deprecated:` markers) and remove at v2 — cleaner
  long-term core, bookkeeping now.
- c. Remove **before** v1.0 while still in `v0.x` (a `v0.7.0` with removals;
  breaking-in-0.x is legal and no consumer is affected, but it contradicts
  the additive-minor posture of `v0.6.0` and needs its own release).
- d. Other.

## Decisions

Resolved by user review on 2026-07-03 and folded into ADR-0001's revised
Decision section; all four questions are closed. Note: this INV's matrix
records the **demand-only** verdicts; the adopted ADR-0001 scope goes further
under the export-lean rule — `Create` and the `toc` splice promote too, and
the entire former `v0.6.0` slate (viper-free `Load`, `docparse`, `CheckTask`)
ships inside the single `v1.0.0` release with no intermediate tag.

| #   | Question                        | Choice                          | Notes                                                                                              |
| --- | ------------------------------- | ------------------------------- | -------------------------------------------------------------------------------------------------- |
| 1   | v1.0 core scope                 | (a) as demand floor             | adopted scope adds `toc`: `pkg/doczcore/{config,document,docparse,docwrite,toc}`; `template`/`index`/`wiki` stay `internal/` |
| 2   | `Create` at v1.0                | (b) promote `docwrite` whole    | cohesion over splitting; implemented over `internal/template`, embed stays private (Observation 4) |
| 3   | `toc` → `docparse` delegation   | (a) delegate                    | and the splice promotes to public `pkg/doczcore/toc` (ADR-0001 export-lean); `toc`'s walker retires |
| 4   | CLI-shaped `config` helpers     | (a) keep                        | documented as CLI-support helpers in the package doc                                               |

## References

- **ADR-0001** — pkg/doczcore as the single public core (the trigger; this
  INV grounds its "Demand mismatch" consequence and OQ3/OQ4/OQ5/OQ6; revised
  2026-07-03 with the promotion rule and Decisions this INV informed).
- **DESIGN-0008** — docz-api: Requirements R1–R9, incl. the exact-symbol R2
  table and R8 ("no `docz export --json`").
- **DESIGN-0009** — docz-site: thin client of docz-api; client-side rendering
  (Decision 3), own-ToC-from-rendered-headings (OQ8a), MkDocs wiki as
  complementary non-goal.
- **`docz-v0.6.0-requirements.md`** — sdk-booty-sh handoff: R1 viper-free
  `config.Load`, R2 `docparse`, R3 `docwrite` promotion + `CheckTask`;
  non-goals (no general editor, no new CLI commands).
- **DESIGN-0007 / IMPL-0013** — the `v0.5.0` promotion (read-only stance,
  minimal-surface goal, `internal/docwrite` split precedent).
- **INV-0005** — Decision 7 (shared library over re-implemented parser).
- Audit method: `go doc -all` per package;
  `grep -oE '(doctemplate|index|toc|wiki|docwrite)\.[A-Z]\w*' cmd/*.go`
  (non-test); `grep -rn 'docz/internal' internal/` (cross-internal edges);
  `grep -rn 'yaml' internal/ pkg/` (dependency audit).
