---
id: ADR-0001
title: "pkg/doczcore as the single public core; cmd as a thin CLI shell (v1.0.0)"
status: Accepted
author: Donald Gifford
created: 2026-07-03
---
<!-- markdownlint-disable-file MD025 MD041 -->

# 0001. pkg/doczcore as the single public core; cmd as a thin CLI shell (v1.0.0)

<!--toc:start-->
- [Status](#status)
- [Context](#context)
  - [What the code looks like today (the facts that make this feasible)](#what-the-code-looks-like-today-the-facts-that-make-this-feasible)
  - [The benchmark](#the-benchmark)
- [Decision](#decision)
  - [Scope — what moves](#scope--what-moves)
  - [The compatibility contract (what v1.0 promises)](#the-compatibility-contract-what-v10-promises)
- [Consequences](#consequences)
  - [Positive](#positive)
  - [Negative](#negative)
  - [Neutral](#neutral)
- [Corner Cases](#corner-cases)
- [Alternatives Considered](#alternatives-considered)
- [Open Questions](#open-questions)
  - [1. Package granularity — subpackage family or one flat package?](#1-package-granularity--subpackage-family-or-one-flat-package)
  - [2. Sequencing against the in-flight v0.6.0?](#2-sequencing-against-the-in-flight-v060)
  - [3. How do we treat the CLI-shaped packages (template, index, wiki) at v1.0?](#3-how-do-we-treat-the-cli-shaped-packages-template-index-wiki-at-v10)
  - [4. toc vs docparse overlap in the public API?](#4-toc-vs-docparse-overlap-in-the-public-api)
  - [5. What remains in internal/ after the move?](#5-what-remains-in-internal-after-the-move)
  - [6. Embedded templates — how public?](#6-embedded-templates--how-public)
  - [7. Post-1.0 compatibility & deprecation policy?](#7-post-10-compatibility--deprecation-policy)
- [Decisions](#decisions)
- [References](#references)
<!--toc:end-->

## Status

Accepted (2026-07-03 — all seven open questions resolved; implemented by
IMPL-0014)

## Context

docz began as a single CLI binary with all logic under `internal/`. As external
consumers appeared — **docz-api** (DESIGN-0008, cross-repo registry/ingestion),
**docz-site** (DESIGN-0009, viewer/search), and **sdk-booty-sh** (a doc-driven
agent-loop harness) — we have been **promoting packages from `internal/` to
`pkg/doczcore/` one at a time, on demand**:

- `v0.5.0` (DESIGN-0007 / IMPL-0013) promoted `config` + `document` (read side).
- A `v0.6.0` slate (the sdk-booty-sh requirements handoff,
  `docz-v0.6.0-requirements.md`) specified a viper-free `config.Load`, a new
  `pkg/doczcore/docparse`, and the `docwrite` promotion + `CheckTask` — but
  **none of it exists in code** (no IMPL doc, no `docparse` package, no
  `CheckTask`, viper still in `Load`; verified 2026-07-03). Per Open
  Question 2's resolution, no separate `v0.6.0` ships: the slate is part of
  the `v1.0.0` work.

Each promotion is a separate PR with its own API review and import sweep. That
per-consumer drip is repetitive, and it means the public surface is defined by
"whatever a consumer happened to need last," not by a coherent design.

This ADR proposes ending that drip: **establish `pkg/doczcore` as the single
public core that both the docz CLI and every external consumer build on, and cut
`v1.0.0` as the first stable, semver-governed release of that surface.**

### What the code looks like today (the facts that make this feasible)

An audit of the current tree (`cmd/` + five `internal/` packages) shows the move
is mechanically shallow:

- **`cmd/` already imports only `internal/*`, `pkg/doczcore/*`, and stdlib/cobra.**
  It calls into `internal/{docwrite,template,toc,index,wiki}` and
  `pkg/doczcore/{config,document}`. There is no third home for logic to hide in.
- **The internal dependency graph is shallow and one-directional.** Every
  `internal/*` package depends only on `pkg/doczcore/{config,document}` plus
  stdlib. The single cross-internal edge is `docwrite → template` (used by
  `Create`).
- **The third-party dependency surface is already tiny.** Of the five internal
  packages, only `wiki` pulls a non-stdlib dependency (`go.yaml.in/yaml/v3`,
  which is already a core dependency). `docwrite`, `index`, `template`, and `toc`
  are **stdlib-only**. With viper removed as part of this work, promoting these
  adds no new heavy dependency to a consumer's build.
- **One embed.** `//go:embed` appears only in `internal/template`
  (`templates/*.md`, `templates/*.tmpl`); the embedded files travel with the
  package wherever it moves.

### The benchmark

The benchmark is twofold: (1) the docz CLI keeps working **exactly as it does
today** — every command (`init`, `create`, `update`, `list`, `status`,
`template`, `config`, `wiki`, `version`), output, and exit code unchanged —
and (2) the public core is **sufficient for every known external consumer**
(docz-api, docz-site via docz-api, sdk-booty-sh) to build on with zero parser
drift. The CLI is *not* required to build on `pkg/doczcore` alone: `cmd/` +
`internal/` presentation code layered over the public core satisfies (1), and
the INV-0006 demand audit shows the four-package demand core (`config`,
`document`, `docparse`, `docwrite`) satisfies (2); the adopted scope adds the
`toc` splice on top (export-lean, Decision 2). This ADR does not remove or
change any capability; it fixes where the public boundary sits and freezes
the API behind it.

## Decision

> Revised 2026-07-03 after the INV-0006 per-package demand audit and design
> review. The original draft moved *all* `internal/` packages public; the
> revision replaces that with a promotion rule and a smaller frozen core.

1. **Promotion rule — public by default where demand exists, whole packages,
   named exceptions.** Any package containing a symbol an external consumer
   requires is promoted **whole** into `pkg/doczcore/*` — package cohesion
   beats surface minimalism; we do not split a package to hide one function —
   UNLESS a specific, documented reason keeps machinery private. The embedded
   template engine is the canonical exception. Packages nothing external
   needs stay in `internal/`.
2. **Applied to today's tree** (per the INV-0006 audit plus the export-lean
   review of 2026-07-03): the public core is
   `pkg/doczcore/{config,document,docparse,docwrite,toc}`, all landing in
   the **single `v1.0.0` release** — the planned `v0.6.0` never started, so
   its slate folds in here:
   - `config` goes **viper-free** (same `Load` signature, yaml-only —
     handoff R1);
   - `docparse` is **created** as the canonical markdown **fact extractor**
     — `Headings` + `TaskItems` checkbox items, both with byte-accurate
     1-based line numbers (handoff R2, revised by IMPL-0014 Q3: the
     `ParsePlan` plan/phase *interpretation* stays consumer-side in
     sdk-booty-sh — docz parses facts, consumers own meaning);
   - `docwrite` promotes **whole**: `SetStatus` as-is, the new `CheckTask`
     (handoff R3), and `Create`/`CreateOptions`/`CreateResult`;
   - `toc` promotes as the **ToC-splice** package (`GenerateToC`/
     `UpdateToC`/`UpdateFiles`, the markers), its heading walk delegating to
     `docparse` — one public parser (Open Question 4a); `toc`'s own
     `ParseHeadings`/`Heading`/`AnchorSlug` walker retires rather than going
     public.

   `template`, `index`, and `wiki` remain `internal/`: zero consumer demand,
   CLI-shaped APIs, and (for `template`) the embed.
3. **Public packages may depend on `internal/` packages.** This is standard
   Go — the stdlib does it pervasively — and it is how `docwrite.Create`
   keeps rendering via `internal/template` without the embed, template
   names, or render API entering the public contract.
   `CreateOptions`/`CreateResult` expose only `config` types, primitives,
   and `time.Time` (verified), so nothing internal leaks through the
   promoted signature.
4. **`cmd/` + `internal/` are the CLI product; `pkg/doczcore` is the library
   product.** "Thin `cmd/`" means no *core logic* in `cmd/` — cobra wiring,
   the `Runner`, flags, output formatting, exit codes only — not "no
   `internal/`". `internal/` is the CLI's private half, not a third tier.
5. **API design principles for the public core:**
   - **Bytes-in / values-out where possible.** Parsing takes `[]byte`
     (`ParseFrontmatter`, `docparse.Headings`/`ParsePlan`), not paths, so
     consumers with no checkout (docz-api) stay first-class.
   - **No library-defined interfaces.** The core exports concrete functions
     and structs; consumers define their own small interfaces at the call
     site. Exported interfaces are the most expensive surface to freeze —
     adding a method breaks every external implementer.
   - **Options/result structs over positional parameters** for compound
     operations (`CreateOptions`/`CreateResult`) — fields evolve additively
     within the frozen contract.
   - **Path-based write helpers only where a consumer requires in-place,
     byte-preserving mutation** (`SetStatus`, `CheckTask`). Any future
     creation-style API for non-filesystem callers returns
     `(name, content)` rather than performing I/O.
6. **`v1.0.0` is the first stable release** of this surface, cut via the
   `major` release label (0.5.x → 1.0.0; no intermediate `v0.6.0`). Exported
   identifiers under `pkg/doczcore/*` become a semver-governed compatibility
   contract. sdk-booty-sh pins `v1.0.0` instead of the formerly planned
   `v0.6.0` tag.
7. **The CLI feature set is the completeness benchmark.** v1.0 ships only
   with **no CLI behavior change**: commands, flags, outputs, and exit codes
   identical; only import paths and package homes move.

### Scope — what moves

All of this ships in the single `v1.0.0` release:

| Package (today) | v1.0 home | Surface | Notes |
|---|---|---|---|
| `pkg/doczcore/config` | unchanged | — | already public (v0.5.0); goes viper-free in this work (handoff R1) |
| `pkg/doczcore/document` | unchanged | — | already public (v0.5.0); stays read-only |
| — (new) | `pkg/doczcore/docparse` | new | fact extractor: `Headings` + `TaskItems` with line numbers (handoff R2 as revised by IMPL-0014 Q3 — no plan model) |
| `internal/docwrite` | `pkg/doczcore/docwrite` | small | **whole package**: `SetStatus`, new `CheckTask` (handoff R3), and `Create` implemented over `internal/template` |
| `internal/toc` | `pkg/doczcore/toc` | small | the ToC-splice (`GenerateToC`/`UpdateToC`/`UpdateFiles`, markers); walk delegates to `docparse`; its private walker retires |
| `internal/template` | **stays `internal/`** | — | the named exception: embed + render machinery stay out of the contract |
| `internal/index` | **stays `internal/`** | — | zero external demand; `UpdateOutcome` is CLI wording (INV-0006 Obs 3) |
| `internal/wiki` | **stays `internal/`** | — | MkDocs/site-specific; zero external demand |
| `cmd/*` | unchanged path | — | repoint the `docwrite`/`toc` imports; keep CLI/UX only |

### The compatibility contract (what v1.0 promises)

- **Covered by semver:** every exported identifier under `pkg/doczcore/*`.
  Breaking changes require a major bump with a deprecation window.
- **NOT covered:** `cmd/`, any code that remains in `internal/`, `test/`, the
  CLI's stdout/stderr **text** and exit codes (these get a separate, looser
  CLI-stability note), and the **contents/names of embedded templates**.

## Consequences

### Positive

- **One coherent public surface**, designed as a whole rather than accreted
  per-consumer. Consumers import exactly the subpackage they need.
- **Ends the promotion drip.** No more one-off promotion PRs; the next consumer
  need is already satisfied (or is a normal additive minor).
- **`cmd/` becomes a clean, thin shell** — a working reference implementation of
  the library and a forcing function that keeps logic out of the CLI layer.
- **Cheap moment to do it.** The dependency surface is already stdlib + `yaml.v3`
  (viper leaves in the same release); the import graph is shallow; the move is
  mostly `git mv` + repoint. Pre-1.0 is the last window where the API can be
  reshaped without a major bump.
- **An explicit stability guarantee** external consumers can pin — the point of a
  `v1.0.0`.

### Negative

- **`Create` and the `toc` splice become contract without a consumer.**
  Promoting `docwrite` whole (cohesion) freezes `Create`'s observable
  conventions — the `NNNN-slug.md` filename shape, the auto-increment scan,
  the template-resolution tiers — and promoting `toc` freezes the splice API
  (`UpdateFiles` and its report categories included) before any external
  caller exists. Mitigated: options/result/report structs evolve additively,
  and the filename and ID conventions are already de facto frozen via the
  public `document.DoczFilePattern`.
- **Consumers of `docwrite` compile the embed.** The `internal/template`
  dependency rides along in builds (not in the API). The cost is a handful of
  embedded markdown templates plus stdlib `text/template` — negligible, but
  nonzero for a consumer that only wants `SetStatus`.
- **Support burden.** Public APIs invite bug reports and feature requests
  against code that was previously ours to change freely — now scoped to the
  five core packages rather than everything.
- **A softer "single core" story.** The CLI is *not* a pure `pkg/doczcore`
  consumer; contributors must learn the rule ("presentation stays internal")
  rather than a flat "everything is public."

_The original draft's negatives — freezing CLI-shaped `template`/`index`/
`wiki` APIs under semver, and the demand mismatch of promoting surface no
consumer asked for — are resolved by the revised scope rather than accepted:
those packages stay `internal/` (INV-0006)._

### Neutral

- **Dependency graph barely changes.** With viper removed in this release the
  public core depends on `yaml.v3` + stdlib only; `docparse`, `toc`, and the
  `docwrite` splice primitives are stdlib-only, and per-subpackage imports
  mean a consumer of `document` compiles none of the CLI's internal
  machinery.
- **Version jump is bookkeeping.** The `major` label drives `v1.0.0` through the
  existing `pr-semver-bump` + goreleaser automation, same mechanism as prior tags.
- **No CLI behavior change.** This is a relocation + freeze, not a feature change.

## Corner Cases

1. **Embedded templates as public contract.** Dissolved by the revised scope:
   `internal/template` does not move, so the `embed.FS`, template names, and
   template contents can never enter the contract through the public API —
   no mitigation needed (Open Question 6, now moot).
2. **`toc` vs `docparse` overlap.** Resolved (Open Question 4a): `docparse`
   is the canonical — and only — public parser. `toc` promotes as the
   ToC-splice package (`GenerateToC`/`UpdateToC`/`UpdateFiles`, the markers
   `cmd/wiki.go` also uses) with its heading walk delegating to `docparse`
   (public `GenerateToC` takes `[]docparse.Heading`); `toc`'s own
   `ParseHeadings`/`Heading`/`AnchorSlug` walker retires.
3. **CLI-shaped return types.** `index.UpdateOutcome` (with an `Action` enum tuned
   to `cmd`'s user-facing wording) and `wiki`'s nav/mkdocs helpers encode CLI
   decisions. Freezing them as-is exports those decisions; redesigning them is the
   real cost (Open Question 3/5).
4. **`Create` needs `template` — and that's fine.** `internal/docwrite.Create`
   is the only cross-internal edge (`docwrite → template`). A public package
   may legally import its own module's `internal/` packages (the stdlib does
   this pervasively), so the promoted `pkg/doczcore/docwrite` keeps rendering
   via `internal/template`. Consumers importing `docwrite` compile the embed
   and `text/template` into their binaries (both stdlib-backed — no new
   module dependencies), but none of it is importable or part of the
   contract. What *does* freeze with `Create` is observable behavior: the
   `NNNN-slug.md` filename convention and the auto-increment scan, both
   already de facto contract via the public `document.DoczFilePattern`.
5. **Sequencing against the planned v0.6.0.** Resolved (Open Question 2): the
   v0.6.0 slate was never started — no IMPL doc, no `docparse`, no
   `CheckTask`, viper still in `config.Load` (verified 2026-07-03) — so there
   is nothing to "ship first." The slate is part of the `v1.0.0` work, and
   sdk-booty-sh pins `v1.0.0` instead of `v0.6.0`. Accepted trade-off: its
   wait extends by the width of the v1.0-only additions (`Create`, `toc`,
   the freeze), which is small relative to the slate itself.
6. **Empty `internal/`?** Moot under the revised scope — `internal/` remains
   populated (`template`, `index`, `toc` splice, `wiki`) as the CLI's private
   half (Open Question 5, resolved a).
7. **`test/consumer/` grows.** The external-module smoke test should exercise the
   full v1.0 surface (import each subpackage, assert it compiles + runs from
   outside the module) so accidental surface narrowing is caught.
8. **Naming / stutter.** A flat `pkg/doczcore` package yields `doczcore.Config`,
   `doczcore.ParsePlan`, etc.; the subpackage family keeps `config.Config`,
   `docparse.ParsePlan`. This interacts with Open Question 1.
9. **No regression to existing pins.** `config` and `document` keep their existing
   import paths, so v0.5.0/v0.6.0 consumers are unaffected — the consolidation is
   additive for them.

## Alternatives Considered

- **A. Status quo — promote on demand.** Keep `internal/` as the default and
  promote a package only when a consumer needs it and its API passes review.
  *Pros:* minimal semver surface; maximum refactor freedom; each promotion gets a
  design checkpoint. *Cons:* the recurring per-consumer PR churn this ADR is
  reacting to; the public surface stays accretive rather than designed.
- **B. Separate `doczcore` module** (`github.com/donaldgifford/docz-core`) with its
  own version line. *Pros:* fully decouples library semver from CLI releases.
  *Cons:* two modules, two release trains, `replace`/pin friction for the CLI
  itself; heavier than the problem warrants.
- **C. Flat single `pkg/doczcore` package.** Collapse everything into one package.
  *Pros:* one import. *Cons:* no per-subpackage tree-shaking; a huge single API;
  massive rename churn. (Captured as Open Question 1.)
- **D. This ADR — subpackage family under `pkg/doczcore`, v1.0 freeze.** Promote
  the remaining logic packages into the existing `pkg/doczcore/*` namespace and
  commit to semver at v1.0. Chosen as the base for discussion; the open questions
  below refine it.

## Open Questions

> Each question is numbered; option `a` is my recommendation, later letters are
> alternatives, and the final letter is a free-form "other" for you to fill in.
> Answer inline (e.g. `1a`, `3c`) and I'll lock the choices into a Decision before
> we build the implementation plan.
>
> **Update 2026-07-03:** all seven questions are resolved — see the
> [Decisions](#decisions) table. Implementation is planned in IMPL-0014.

### 1. Package granularity — subpackage family or one flat package?

> **Resolved 2026-07-03: (a)** — implied by the per-package promotion rule
> (Decision 1) and the INV-0006 per-package audit.

- **a. (Recommended)** Keep the **subpackage family**
  `pkg/doczcore/{config,document,docparse,docwrite,template,index,toc,wiki}`.
  Preserves the current split and the typed boundaries, keeps imports
  tree-shakeable (a consumer of `document` doesn't compile `wiki`), and is minimal
  churn (mostly `git mv` + repoint).
- b. Collapse to a **single flat** `pkg/doczcore` package — one import, but no
  tree-shaking, a very large single surface, and pervasive rename churn.
- c. Other.

### 2. Sequencing against the in-flight `v0.6.0`?

> **Resolved 2026-07-03: (b), amended by fact-check** — the premise of (a)
> was wrong: nothing of the v0.6.0 slate exists in code (no IMPL doc, no
> `docparse`, no `CheckTask`, viper still in `Load`), so there is nothing to
> ship first. The slate folds into the single `v1.0.0` release;
> sdk-booty-sh pins `v1.0.0`.

- **a. (Recommended)** **Ship `v0.6.0` (IMPL-0014) first** to unblock sdk-booty-sh
  (it is waiting on the tag), then do this consolidation as `v1.0.0`. Two clean
  releases; the blocked consumer isn't held hostage to the big refactor.
- b. **Fold everything into one `v1.0.0`** and drop the separate `v0.6.0` — fewer
  releases, but delays the waiting consumer behind the larger, riskier change.
- c. Other (e.g. ship `v0.6.0`, then a `v0.7.0` staging release, then `v1.0.0`).

### 3. How do we treat the CLI-shaped packages (`template`, `index`, `wiki`) at v1.0?

> **Resolved 2026-07-03: none of the drafted options** — they stay
> `internal/` (zero demand, INV-0006), while `docwrite` promotes **whole**
> (incl. `Create`, implemented over `internal/template`). See Decisions.

- **a. (Recommended)** **Promote with a light naming/doc review** and accept the
  current shapes as v1.0 API (the benchmark is "cmd works unchanged"). Fastest path
  to v1.0; we can deprecate/refine later within semver.
- b. **Redesign each for general public use** before freezing (e.g. decouple
  `index.UpdateOutcome` from CLI wording, generalize `wiki` beyond MkDocs). Cleaner
  long-term API, materially more work, delays v1.0.
- c. **Promote but mark `template`/`index`/`wiki` as `experimental`/`unstable`**
  (documented as *exempt* from the v1.0 compatibility guarantee) so the *needed*
  core (`config`/`document`/`docparse`/`docwrite`) is stable while the CLI-shaped
  packages keep refactor freedom. A middle path that limits the semver cost.
- d. Other.

### 4. `toc` vs `docparse` overlap in the public API?

> **Resolved 2026-07-03: (a)** — export-lean: the splice is document
> machinery (bytes-in content transforms), not CLI presentation, so it
> promotes to `pkg/doczcore/toc` delegating its walk to `docparse`. One
> public parser; `toc`'s own walker retires.

- **a. (Recommended)** `docparse` is the **canonical parser**; keep a
  `pkg/doczcore/toc` for the **ToC-splice** concern
  (`GenerateToC`/`UpdateToC`/`UpdateFiles`) that **delegates** to `docparse` for
  the heading walk — one parser, ToC generation layered on top.
- b. **Fold the ToC-splice into `docparse`** (single package owns both parse and
  ToC rendering) — fewer packages, but mixes "parse" and "render" concerns.
- c. Keep `toc` independent with its own walker (accept two parsers in public).
- d. Other.

### 5. What remains in `internal/` after the move?

> **Resolved 2026-07-03: (a)**, strengthened — `internal/` stays *populated*
> (`template`, `index`, `toc` splice, `wiki`), not merely available.

- **a. (Recommended)** **Keep `internal/` available** as the sanctioned home for
  genuinely CLI-private helpers (none today, but the door stays open), while moving
  all current logic out. `cmd/` holds UX/wiring; `internal/` holds CLI-only glue if
  any arises.
- b. **Delete `internal/` entirely**; anything CLI-private lives directly in
  `cmd/`. Strongest "single core" statement, least flexibility.
- c. Other.

### 6. Embedded templates — how public?

> **Resolved 2026-07-03: moot** — `template` never promotes; the embed stays
> private behind the `internal/` boundary.

- **a. (Recommended)** Move `templates/` with `pkg/doczcore/template`, keep the
  `embed.FS` **package-private**, and expose only accessor functions
  (`EmbeddedDocumentTemplate`, etc.). **Exclude template file names/contents from
  the compatibility contract** so we can edit built-in templates freely.
- b. **Export the `embed.FS`** for full consumer access to raw templates — maximal
  flexibility for consumers, but freezes template names/paths as public contract.
- c. Other.

### 7. Post-1.0 compatibility & deprecation policy?

> **Resolved 2026-07-03: (a), simplified — roll forward.** Standard semver
> coverage as drafted; breaking changes ship in a major bump. No mandatory
> deprecation window — `Deprecated:` markers when practical, then roll
> forward.

- **a. (Recommended)** Standard semver: exported identifiers under
  `pkg/doczcore/*` are covered; `cmd/`, `internal/`, `test/`, CLI output **text**,
  and embedded template contents are **not**. Breaking changes ship in a major bump
  after a documented deprecation window (one minor with `Deprecated:` markers).
- b. A stricter/looser variant (specify).
- c. Other.

## Decisions

Resolved by review on 2026-07-03, grounded in the INV-0006 per-package demand
audit and the export-lean review. All seven questions are closed; IMPL-0014
carries the implementation plan.

| #   | Question                          | Choice                                        | Notes                                                                                                                       |
| --- | --------------------------------- | --------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------- |
| 1   | Package granularity               | (a) subpackage family                         | implied by the per-package promotion rule; INV-0006 audits package-by-package                                               |
| 2   | Sequencing vs `v0.6.0`            | fold into one `v1.0.0`                        | nothing of the slate exists in code (verified 2026-07-03); no separate `v0.6.0` ships; sdk-booty-sh pins `v1.0.0`           |
| 3   | CLI-shaped packages at v1.0       | `template`/`index`/`wiki` stay `internal/`; `docwrite` promotes whole | zero external demand for the three (INV-0006); `Create` joins public `docwrite` at v1.0 over `internal/template` |
| 4   | `toc` vs `docparse`               | (a) one parser, both public                   | `docparse` canonical; `toc` promotes as the splice package delegating to it; `toc`'s private walker retires                 |
| 5   | What remains in `internal/`       | (a) keep `internal/`, populated               | `template`, `index`, `wiki` — the CLI's private half                                                                        |
| 6   | Embedded templates                | moot                                          | `template` never promotes; embed stays private                                                                              |
| —   | CLI-shaped `config` helpers       | keep                                          | `TypesHelp`/`DefaultNavTitles`/`ResolveTypeAlias` stay public (export-lean); documented as CLI-support (INV-0006 Q4a)       |
| 7   | Post-1.0 compat policy            | (a) simplified — roll forward                 | standard semver coverage; breaking changes = major bump; no mandatory deprecation window (`Deprecated:` markers when practical) |
| —   | API design principles             | adopted (Decision 5)                          | bytes-in/values-out; no library-defined interfaces; options/result structs; path-writers only on consumer demand            |

## References

- **INV-0006** — per-package core requirements audit (docz CLI vs docz-api,
  docz-site, sdk-booty-sh): the demand evidence behind the revised scope.
- **IMPL-0014** — the implementation plan for this ADR (all six phases of
  the single `v1.0.0` release). Note: the IMPL-0014 id was once informally
  reserved for a standalone-`v0.6.0` plan that was never written; the id now
  belongs to the v1.0.0 plan.
- **ADR context / prior art** — DESIGN-0007 (`pkg/doczcore` public surface,
  read-only stance), IMPL-0013 (`v0.5.0` promotion), and the
  `docz-v0.6.0-requirements.md` handoff (R1–R3), whose slate ships inside
  `v1.0.0`.
- **Consumers** — DESIGN-0008 (docz-api), DESIGN-0009 (docz-site), the
  `sdk-booty-sh` v0.6.0 requirements handoff (loop harness).
- **Release mechanism** — `pr-semver-bump` (`major` label → `v1.0.0`) + goreleaser,
  same path as prior tags.
