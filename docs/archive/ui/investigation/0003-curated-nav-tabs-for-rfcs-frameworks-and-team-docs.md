---
id: INV-0003
title: "Curated nav tabs for RFCs, frameworks, and team docs"
status: Concluded
author: Donald Gifford
created: 2026-07-25
---
<!-- markdownlint-disable-file MD025 MD041 -->

# INV 0003: Curated nav tabs for RFCs, frameworks, and team docs

**Status:** Concluded
**Author:** Donald Gifford
**Date:** 2026-07-25

<!--toc:start-->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
  - [Observation 1: two of the three tabs are links, not features](#observation-1-two-of-the-three-tabs-are-links-not-features)
  - [Observation 2: the production model needs zero special-casing](#observation-2-the-production-model-needs-zero-special-casing)
  - [Observation 3: how the tabs get configured — extend the runtime config](#observation-3-how-the-tabs-get-configured--extend-the-runtime-config)
  - [Observation 4: the Docs tab is the real project](#observation-4-the-docs-tab-is-the-real-project)
  - [Observation 5: search coherence is the deciding argument](#observation-5-search-coherence-is-the-deciding-argument)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
- [References](#references)
<!--toc:end-->

## Question

We want curated top-level navigation tabs alongside Directory and
Repos:

1. **RFCs** — a quick-find entry point to a dedicated `rfcs` repo that
   holds only RFC docz docs.
2. **Docs** — a `docs` repo of *standard* markdown (non-docz: no IDs,
   no frontmatter contract, nested folders), rendered mkdocs-style as
   a team documentation site.
3. **Frameworks** — like RFCs, but pointing at a specific repo's docz
   docs of a custom `FW-XXXX` type.

And the production model to validate: the `rfcs` repo is RFC-only;
frameworks live as an FW type inside their owning repo; **every other
repo keeps its normal adr/design/impl surfaces exactly as today**.
What would this look like, what already works, and what genuinely
needs building?

## Hypothesis

Going in: all three tabs probably need real features — new routes, new
rendering, maybe API work. Expected the mkdocs-style Docs tab to be
the heaviest, since DESIGN-0001 explicitly deferred "team docs repos
(Backstage-TechDocs-style, non-docz-format content)" as future state
with a separate design.

## Context

**Triggered by:** planning the production docs estate: a `rfcs` repo
(org-wide RFCs), a `docs` repo (team documentation, plain markdown),
and framework definitions (FW type) in an owning repo — while
ordinary repos keep publishing adr/design/impl/inv through the normal
repo pages.

Relevant current state:

- The topbar (`src/app/AppShell.tsx`) renders exactly two `NavLink`s —
  Directory (`/`) and Repos (`/repos`) — plus the session menu.
- Routes are `/:owner/:repo`, `/:owner/:repo/:type`,
  `/:owner/:repo/:type/:docId`; type URLs resolve by
  name/id_prefix/alias via `lib/docTypes.resolveDocType`.
- The directory URL is the single source of filter truth
  (`src/lib/searchParams.ts`): `/?type=rfc` is a working,
  deep-linkable, cross-repo RFC listing today.
- v0.1.2 shipped the runtime-config mechanism: `server/serve.ts`
  injects whitelist-validated values into `index.html` as
  `window.__DOCZ_CONFIG__` (currently `{ authProviders }`), fed by
  env / Helm `config.*` values — one image, per-deployment behavior.
- DESIGN-0001 explicitly scoped out non-docz content twice: "raw /
  not-ingested files" (v1 omits the section) and "team docs repos …
  future state, separate design."

## Approach

Desk investigation against both codebases — no code experiments:

1. Inventory what the three tabs need vs what the existing routes,
   type resolution, and directory filters already provide.
2. Check whether the custom FW type works through the stack today
   (docz-site colors/docTypes, docz-api contract handling).
3. Enumerate configuration mechanisms for the tabs themselves
   (hardcode / runtime config / API-driven) against the v0.1.2
   runtime-config precedent.
4. Lay out the option space for the non-docz `docs` repo (force into
   docz as `guide` or a dedicated DOCS type / a config-declared
   "extras" section in the docz contract / separate static site /
   first-class pages ingest) with pros, cons, and required upstream
   asks.

## Environment

| Component | Version / Value                                   |
| --------- | ------------------------------------------------- |
| docz-site | main @ 3304248 (v0.1.2) + this branch             |
| docz-api  | main @ 2a5a43f                                    |
| Reference | DESIGN-0001 §Out of scope, §Repo pages, §API asks |

## Findings

### Observation 1: two of the three tabs are links, not features

**RFCs.** Once the `rfcs` repo is onboarded into docz-api it is just a
repo: it appears in `/repos`, its RFC docs land in Meilisearch, the
palette finds them, and two curated views of it exist *today* with
zero code:

- `/:owner/rfcs/rfc` — the README-style type page (ID / title / status
  badge / date table, synthesized from listTypes + listDocs);
- `/?type=rfc` — the cross-repo directory pre-filtered to RFCs, which
  also aggregates RFCs from any *other* repo that ever grows one.

The tab is an anchor to one of those two URLs. The only real decision
is which target: the type page is the tidier "registry" view; the
filtered directory is the future-proof aggregator. Since the tab is
just an href, this can even differ per deployment.

**Frameworks.** The FW type already works through the entire stack:

- docz-api's contract tests exercise exactly this shape — a custom
  `frameworks` type with `id_prefix: FW`, alias `fw`, living under
  `docs/frameworks/` (`internal/doczcontract/contract_test.go`).
- docz-site curates `framework` in `src/lib/colors.ts` and gives it a
  blurb in `src/lib/docTypes.ts`; unknown/custom types get
  deterministic fallback colors anyway.
- Type URL resolution accepts name, id_prefix, or alias — so
  `/:owner/:repo/frameworks`, `/fw`, and `/FW` all land correctly.

The Frameworks tab is an anchor to `/:owner/:repo/frameworks` (or
`/?type=frameworks` if FW docs ever spread across repos). Nothing to
build beyond the tab itself.

### Observation 2: the production model needs zero special-casing

The proposed estate — `rfcs` repo RFC-only, FW type in one owning
repo, all other repos publishing adr/design/impl normally — falls out
of behavior that already exists:

- Repo pages derive their nav from per-repo facet counts and **omit
  zero-hit types**, so the `rfcs` repo's pages show only the RFC
  section; a repo carrying FW *and* adr/design shows all of them.
  Nothing anywhere assumes a repo has multiple types.
- The `rfcs` and `docs`-adjacent repos still appear in `/repos` and
  the directory like any other repo — the tabs are *additive* curated
  entry points over the same data, not a partition. A framework
  repo's own ADRs keep rendering "the normal way" because the tab
  never touches repo-page behavior; it is a deep link into it.
- Single-type repos make for a sparse repo home nav. That is
  cosmetic, and arguably correct for a registry repo.

The model is sound as stated. No route, component, or API change is
required to *support* it — only to *surface* it (the tabs).

### Observation 3: how the tabs get configured — extend the runtime config

Three mechanisms considered for where the tab set comes from:

1. **Hardcoded tabs.** Rejected — repo names/owners differ per
   deployment, and v0.1.2 just established the principle that
   per-deployment presentation ships without a rebuild.
2. **Runtime config (recommended).** Extend `window.__DOCZ_CONFIG__`
   with a `nav` array injected by `server/serve.ts` from a
   `DOCZ_NAV_LINKS` env (Helm `config.navLinks`), rendered as
   `NavLink`s after Directory/Repos:

   ```jsonc
   window.__DOCZ_CONFIG__ = {
     authProviders: ["github"],
     nav: [
       { label: "RFCs", href: "/donaldgifford/rfcs/rfc" },
       { label: "Frameworks", href: "/donaldgifford/platform/frameworks" },
       { label: "Docs", href: "/docs" }, // once pages exist (Obs 4)
     ],
   };
   ```

   Validation must keep the injected `<script>` breakout-proof and the
   nav abuse-proof, following the two patterns already in the
   codebase: hrefs restricted to same-origin app paths — `/`-prefixed,
   not `//` (the `authReturn.ts` open-redirect rule) — and labels
   charset-restricted and length-capped like the codeblock
   `metastring` (the injected script currently carries *only*
   closed-whitelist values; labels are the first free text, so they
   get the same treatment: validate at the server AND at the reader).
   Same-origin-only also keeps tabs as router `NavLink`s with proper
   active state, and external URLs stay out of the topbar by policy.
3. **API-driven "pinned collections" from docz-api.** The cleanest
   long-term shape (curation lives with the data, all clients agree),
   but it is an upstream feature for what is presentation config
   today. Not justified yet; the runtime-config shape ports to it
   later without UI changes.

Scope of (2): serve.ts + its Bun tests, `authProviders.ts`-style
reader module, AppShell render, chart value + schema + unittest,
axe/e2e additions. A small feature PR, entirely site-side.

### Observation 4: the Docs tab is the real project

A `docs` repo of standard markdown breaks every assumption docz-api's
ingest makes: no `.docz.yaml`, no IDs or frontmatter contract, nested
directories where *hierarchy and order are the nav*, relative links
between pages, and embedded images. docz-api ingests through
`internal/doczcontract` (the docz Go library) — a non-docz repo
cannot be onboarded today, which is exactly why DESIGN-0001 deferred
this. Five options:

**(a) Force the repo into docz as `guide`** — rejected: wrong mental
model (guides are a curated docz type, not a dumping ground), and it
inherits every cost of (a′) with none of the honesty.

**(a′) A dedicated `docs` type — DOCS-XXXX.** Make the `docs` repo a
docz repo whose only enabled type is a custom `docs` (id_prefix
`DOCS`). This works **end-to-end today with zero code**: custom types
already ingest, render, facet, and search (the FW type in Obs 1 is
the proof), so every page gets the reader, the palette, and the
directory — the one-front-door principle (Obs 5) is satisfied on day
one, which neither (a″) nor (b) manages. Optionally curate a color
and blurb (two-line PR). The honest costs: a flat namespace (no
nested tree; "nav" is the type-page table plus search), docz ceremony
per page (frontmatter + IDs — softened by `docz create docs …`),
status/date columns that mean little for a how-to, and no embedded
images — though that last is a limitation of *all* docz docs today,
not of this option. Best read: a validation vehicle — if team docs
thrive as flat, searchable DOCS pages, the heavier options may never
be needed.

**(a″) An "extras" section in the docz contract.** Extend
`.docz.yaml` so a repo can declare non-docz markdown under its
`docs_dir` (named files or globs) as renderable "extras": no IDs, no
frontmatter — the config entry itself is the contract. This has
direct mockup precedent: the mockup's "not ingested" nav section
listing files like `RECOMMENDATIONS.md` is exactly this, made real.
It is *not* minimum-change, though: it touches the docz Go library
(config schema), docz-api (ingest + at least a listExtras/getExtra
surface), and docz-site (nav section + rendering routes) — a
mini-(c) spanning three repos. And as floated ("rendered but not
searchable"), it walks straight into the Obs 5 trap: pages the
palette cannot find. Its real value is different — **it is the right
contract shape for (c)**: nav/inclusion declared in `.docz.yaml`
keeps the docz Go library the single contract owner (the exact
anchor INV-0002 identified) instead of teaching docz-api to parse
`mkdocs.yml`, and search can join from the start via a Meilisearch
`source` facet.

**(b) Separate static site** — build the `docs` repo with real
MkDocs (Material) and either host it externally or serve the built
site behind `server/serve.ts` (proxied under `/docs/`, which keeps it
same-origin and lets the server gate it behind the `docz_session`
cookie). Pros: fastest to ship; full mkdocs feature set (nav from
`mkdocs.yml`, search-in-page, admonitions) with zero docz-site
rendering work. Cons: a second look-and-feel (mkdocs themes vs the
tokens.css terminal aesthetic — restyling MkDocs to match is real,
ongoing work), a second markdown dialect, and — decisively (Obs 5) —
its content is invisible to the palette and directory search.

**(c) First-class "pages" ingest** — the "separate design"
DESIGN-0001 promised. docz-api learns a second content mode: ingest a
markdown tree + nav — declared in `.docz.yaml` per (a″), rather than
parsed from `mkdocs.yml` — expose roughly `getPageTree` / `getPage` /
a raw asset endpoint (embedded images — genuinely new surface; docz
docs never needed it), and index pages in Meilisearch with a
`source`/kind facet. docz-site adds a `/docs` route family: tree nav
in the left rail (RepoNav's collapsible-drawer pattern), content
through the existing markdown pipeline (already handles GFM, Shiki,
mermaid, GitHub-alert admonitions), no metadata table/lifecycle. New
work beyond the tree: **relative-link and image resolution** — docz
docs are flat and xref-linked by ID; pages link by path, so the
renderer needs a base-path-aware anchor/img resolver (same
sanitize-first rules; img sources restricted to the API's asset
endpoint). One deliberate content convention: admonitions stay
GitHub-style (`> [!NOTE]`) — the pipeline already renders them —
rather than teaching it mkdocs' `!!! note` dialect; revisit only if
imported content demands it.

(c) keeps one renderer, one search, one session, one aesthetic — at
the cost of a coordinated DESIGN pair (docz-api ingest/API +
docz-site routes/rendering) plus the docz-library contract change
from (a″). The cheap path and the right path are not in conflict:
(a′) now, (a″)-as-contract inside (c) later, (b) only under
deadline pressure and explicitly labeled temporary.

### Observation 5: search coherence is the deciding argument

DESIGN-0001's core bet is the directory + ⌘K palette as the one front
door: every indexed doc findable from one search box. RFC and
Frameworks tabs inherit that for free — their content is docz content.
Team docs under option (b) never join it: the palette would find every
RFC and framework but silently miss the page that documents the
on-call rotation. That asymmetry ("search sees some tabs but not
others") would read as broken search, not as a hosting detail. Any
long-term Docs answer therefore must end at (c) — pages in the same
Meilisearch index behind the same facets — even if (b) bridges the
gap short-term.

## Conclusion

**Answer:** the model is sound and cheaper than hypothesized — but
unevenly.

1. **RFCs and Frameworks tabs are configuration, not features.** The
   surfaces they point at (repo type pages, filtered directory) exist
   and work today, including the custom FW-XXXX type, which the whole
   stack already handles. The only build is the tab mechanism itself:
   a `nav` extension to the v0.1.2 runtime config — a small,
   site-only feature PR.
2. **The production estate needs zero special-casing.** RFC-only
   `rfcs` repo, FW type in its owning repo, every other repo
   rendering adr/design/impl the normal way — all of that falls out
   of existing behavior (zero-hit types omitted, tabs as additive
   deep links). Repo pages never learn the tabs exist.
3. **The Docs tab is the deferred team-docs project — but it now has
   both a cheap v1 and a destination.** Non-docz content cannot be
   ingested today. A dedicated DOCS-XXXX type (a′) ships with zero
   code and full search — flat and slightly ceremonious, but a real
   product to learn from. The destination remains a first-class pages
   ingest (c), with the `.docz.yaml` "extras" idea (a″) as its
   contract mechanism — keeping the docz Go library the single
   contract owner rather than parsing `mkdocs.yml`. A proxied MkDocs
   build (b) stays the deadline escape hatch only. The hypothesis
   that this tab is the heavy one held; the other two being nearly
   free did not.

## Recommendation

1. **Adopt the estate model as stated** — `rfcs` repo (RFC-only), FW
   type in its owning repo, all other repos unchanged. Verified
   against current behavior; no special-casing to build or maintain.
2. **Build the nav-pins feature**: `DOCZ_NAV_LINKS` →
   `window.__DOCZ_CONFIG__.nav` → topbar `NavLink`s, with
   same-origin-path validation on hrefs (authReturn rule) and
   charset-capped labels (metastring rule) at both the injector and
   the reader. Chart value `config.navLinks`. Per-link targets are
   just hrefs — start with the type pages (`/:owner/rfcs/rfc`,
   `/:owner/:repo/frameworks`) and switch any tab to a filtered
   directory (`/?type=rfc`) if content ever spreads across repos.
3. **Start the docs repo as DOCS-XXXX (a′)** — a docz repo whose only
   enabled type is `docs` (id_prefix `DOCS`). Zero code today, fully
   searchable, and it validates whether flat + one-front-door is
   actually enough before any upstream work is committed. Curate a
   `docs` color/blurb in docz-site as a tiny follow-up.
4. **When (if) hierarchy, nav order, or embedded images become real
   needs, author the team-docs DESIGN pair** for option (c) with the
   (a″) contract: a docz/docz-api design (`.docz.yaml`-declared pages
   ("extras") so the docz Go library stays the single contract owner,
   getPageTree/getPage/asset endpoints, Meilisearch `source` facet —
   searchable from day one, per Obs 5) and a docz-site design
   (`/docs` routes, tree nav via the RepoNav drawer pattern,
   base-path-aware link/image resolution through the sanitize-first
   pipeline). Keep admonitions GitHub-style as the content
   convention. Migration from (a′) is mechanical: strip
   frontmatter, move files, declare them in config.
5. **Only under deadline pressure**: stand up MkDocs behind the
   `serve.ts` proxy under `/docs/` (same-origin, session-gated),
   explicitly labeled interim, tab pointing at `/docs` so the native
   replacement is a drop-in.
6. Whatever ships, the acceptance test for calling the Docs tab done
   is Obs 5: every page findable from the ⌘K palette and directory.

## References

- INV-0002 — Effect/consolidation evaluation on this branch (sibling
  investigation; its "contract anchor" finding is unaffected by this
  one)
- DESIGN-0001 §Out of scope ("team docs repos … future state,
  separate design"; "raw / not-ingested files"), §Repo pages, §API
  asks
  (`docs/design/0001-docz-site-cross-repo-docz-reader-and-search-ui.md`)
- `src/app/AppShell.tsx` — current two-tab topbar nav
- `src/lib/searchParams.ts` — `/?type=rfc` deep-linkable filter state
- `src/lib/docTypes.ts`, `src/lib/colors.ts` — framework type already
  curated
- `src/lib/authReturn.ts` — the same-origin path validation rule to
  reuse for nav hrefs
- `server/serve.ts` — the v0.1.2 runtime-config injection this
  extends
- docz-api `internal/doczcontract/contract_test.go` — custom FW type
  (id_prefix FW, alias fw) exercised end-to-end
- [MkDocs](https://www.mkdocs.org/) /
  [Material for MkDocs](https://squidfunk.github.io/mkdocs-material/)
  — the interim option's tooling
