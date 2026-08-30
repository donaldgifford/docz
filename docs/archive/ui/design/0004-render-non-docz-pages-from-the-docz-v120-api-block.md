---
id: DESIGN-0004
title: "Render non-docz pages from the docz v1.2.0 api block"
status: Implemented
author: Donald Gifford
created: 2026-08-30
---

<!-- markdownlint-disable-file MD025 MD041 -->

# DESIGN-0004: Render non-docz pages from the docz v1.2.0 api block

**Status:** Approved
**Author:** Donald Gifford
**Date:** 2026-08-30

<!--toc:start-->
- [Overview](#overview)
- [Goals and Non-Goals](#goals-and-non-goals)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Background](#background)
  - [The upstream surface, verified](#the-upstream-surface-verified)
  - [Found bug: the config_snapshot key-casing mismatch](#found-bug-the-configsnapshot-key-casing-mismatch)
  - [What the site already has](#what-the-site-already-has)
- [Detailed Design](#detailed-design)
  - [Component 1: spec re-vendor at 1.4.1](#component-1-spec-re-vendor-at-141)
  - [Component 2: the api-block snapshot reader](#component-2-the-api-block-snapshot-reader)
  - [Component 3: the page index and link resolution](#component-3-the-page-index-and-link-resolution)
  - [Component 4: routes, reader, and nav](#component-4-routes-reader-and-nav)
  - [Component 5: search surfaces](#component-5-search-surfaces)
  - [Cross-cutting: security invariants](#cross-cutting-security-invariants)
- [API / Interface Changes](#api--interface-changes)
- [Data Model](#data-model)
- [Testing Strategy](#testing-strategy)
- [Migration / Rollout Plan](#migration--rollout-plan)
- [Open Questions](#open-questions)
- [References](#references)
<!--toc:end-->

## Overview

docz-api `v0.8.0` (spec `1.4.0`, its DESIGN-0004/IMPL-0007) consumes the
docz `v1.2.0` `api:` config block and now serves **pages** — every
markdown file a repo publishes beyond its docz documents — plus a
`source` facet that joins pages into search. This design makes the site
render that surface: a page reader under a reserved `/pages/*` route
family, a Pages section in the repo nav, page hits in the palette and
directory, and page paths joined into the relative-link resolver. The
research also surfaced a real bug — `config_snapshot`'s wire keys were
Go field names, so the site's lowercase readers never matched — fixed
**upstream first** (docz#89 → docz v1.2.2 → docz-api#25 → v0.8.1,
spec `1.4.1`) so no compatibility layer ships here (OQ-1, answered;
chain shipped and verified 2026-08-30).

**Implements:** issue
[#21](https://github.com/donaldgifford/docz-site/issues/21) (the
upstream coordination issue).

## Goals and Non-Goals

### Goals

- Re-vendor the spec at `1.4.1` (docz-api `v0.8.1`) and regenerate the
  client — one hop picks up the pages endpoints (`1.3.0`), the search
  `source`/`path` fields (`1.4.0`), and the now-contractual
  `config_snapshot` key spellings (`1.4.1`).
- A page reader at `/:owner/:repo/pages/*` rendering `raw_md` through
  the one existing markdown pipeline — pages are untrusted input
  exactly like doc bodies, `index_md`, and `changelog_md`.
- A repo-nav Pages section gated on the repo's `config_snapshot` (the
  `changelogConfig` precedent: zero extra requests for repos that
  don't opt in).
- Page hits rendered in the ⌘K palette and the directory, with the
  `source` facet distinguishing the two populations.
- Pages join the link resolver: author-written relative links between
  docs and pages resolve to SPA routes through the same
  exact-match-whitelist transform docs use today.
- ~~Get the `config_snapshot` key-casing bug fixed **upstream**~~
  **Done** (docz#89 → v1.2.2; docz-api#25 → v0.8.1): both the existing
  `changelog:` gate and the new `api:` gate read one normalized shape
  and no compatibility layer ships here (OQ-1, answered).
- One `minor` release; no new chart values, env vars, or
  `__DOCZ_CONFIG__` keys — the feature is driven entirely by each
  repo's `.docz.yaml`, so the site detects it from API data alone.

### Non-Goals

- **Assets and images.** docz's rule is `.md`-only; the raw-asset
  endpoint doesn't exist upstream (docz-api DESIGN-0004 non-goal).
- **A source filter control in the directory.** Spec `1.4.0` has no
  `source` query param on `searchDocs` (verified in
  `internal/httpapi/search.go` — the facet is counts-only on the
  wire). Filing the additive upstream ask is in scope; shipping a
  filter that can't paginate honestly is not (OQ-3).
- **Page-aware repo home redesign.** `getRepoIndex` already serves the
  (possibly relocated) landing page; the repo home route is unchanged
  apart from its relative-link base (Component 3).
- **Server/chart changes.** None — no config surface exists for this
  feature by design.
- **Editing, tree persistence, or per-page metadata** beyond what
  `PageSummary` carries (`path`, `title`, `git_sha`).

## Background

### The upstream surface, verified

Verified against docz-api `origin/main` (v0.8.0, spec `1.4.0`) and the
docz `v1.2.0` source:

**`GET /api/v1/repos/{owner}/{name}/pages`** (`listRepoPages`) →
`{"pages": [{path, title, git_sha}]}`, ordered by path. The flat list
is deliberate — consumers build their own tree, and it is exactly the
shape the site's `byPath` resolver map wants. A repo with no pages —
including every repo without an enabled `api:` block — returns
`200 {"pages": []}`, **not 404**. "Doesn't do pages" is distinguished
via `config_snapshot`, the `changelogConfig` precedent.

**`GET /api/v1/repos/{owner}/{name}/pages/{path}`** (`getRepoPage`) →
`{repo, path, title, raw_md, git_sha}`. Published paths contain `/`;
clients percent-encode the whole path as one segment
(`pages/guides%2Fsetup.md`), though the literal-slash spelling routes
identically. Lookup is exact-byte (git is case-sensitive); traversal
or otherwise invalid decoded paths are indistinguishable 404s. Note
the wire deliberately does **not** carry the source file's repo path —
the site reconstructs it (Component 3).

**The published-path mapping** (docz-api DESIGN-0004, condensed):

| Source file | Published path |
| ----------- | -------------- |
| the landing page (default `docs/index.md`, relocatable via `api.landing_page`) | — (repo row; served by `getRepoIndex`) |
| `docs/impl/README.md` | `impl` (directory page; README wins a directory) |
| `docs/guides/index.md`, no README | `guides` (lone index serves the directory) |
| `docs/examples/example1.md` | `examples/example1.md` (extension kept) |
| `CONTRIBUTING.md` via `additional_docs` | `CONTRIBUTING.md` (repo-relative) |

Directory pages publish extensionless at the directory path; file
pages keep `.md`. The two namespaces (docs_dir-relative and
repo-relative `additional_docs`) share one address space; upstream
resolves collisions deterministically (docs_dir wins).

**Search (`1.4.0`).** `SearchHit` gains two **required** fields:
`source` (`"doc" | "page"`) and `path` (repo-relative file path on doc
hits, published page path on page hits). Doc-only fields (`doc_id`,
`type`, `status`, `author`) are `""` on page hits. `source` joins the
facet counts. There is **no** `source` filter param on `searchDocs`.

**`getRepoIndex`** now documents that an enabled block may relocate
the served landing page via `api.landing_page`; the endpoint's shape
is unchanged.

**Reserved word.** `pages` is a reserved literal under the repo route
on the API side, beside `index`, `changelog`, and `types`; the
site-side reserved-word spend is this design's (Component 4).

### Found bug: the config_snapshot key-casing mismatch

`config_snapshot` is docz-api's `json.Marshal` of docz's post-`Load`
`Config` struct — which has **no json tags** (verified in docz
`v1.2.0` `pkg/doczcore/config/config.go`; also true of `v1.1.0`), so
the real wire shape uses Go field names, verified empirically:

```json
{"DocsDir": "docs", "Changelog": {"Enabled": true, "File": "…"},
 "API": {"Enabled": true, "LandingPage": "docs/index.md",
         "Exclude": null, "AdditionalDocs": null}}
```

The site's shipped `changelogConfig` reads lowercase
`snapshot.changelog.enabled` / `.file` — so against a real docz-api
the defensive reader returns `undefined` and the changelog nav row
**never renders**. The site's own fixtures were hand-authored
lowercase, which is why every suite passes. The bug is real but quiet
by design: the reader failing means a missing nav row, not an error.

Two more real-shape facts the readers must respect: nil Go slices
marshal as `null` (`Exclude`, `AdditionalDocs` — the existing `arr()`
gotcha), and an enabled block's `LandingPage` arrives **already
resolved** (docz normalization backfills `<docs_dir>/index.md` at
load, so the snapshot never carries an empty landing page while
enabled).

Resolution: **OQ-1, answered — fixed upstream first. Shipped and
verified 2026-08-30.**
[docz#89](https://github.com/donaldgifford/docz/issues/89) (closed →
docz `v1.2.2`) added json tags mirroring the yaml spellings;
[docz-api#25](https://github.com/donaldgifford/docz-api/issues/25)
(closed → docz-api `v0.8.1`, spec `1.4.1`) bumped the pin,
contract-pinned the marshaled shape (`doczcontract/snapshot_test.go`),
and made the spellings spec contract — `config_snapshot`'s
description now documents the `.docz.yaml` keys, `null` list
semantics, and the pre-`v1.2.2` capitalized history. Verified
empirically against the new pin: `{"api": {"enabled": true,
"landing_page": "docs/index.md", "exclude": null, "additional_docs":
null}, "changelog": {"enabled": true, "file": "CHANGELOG.md"}}`.
`changelogConfig` already reads this shape, so the shipped gate
starts working with **zero site changes** (repos refresh their
snapshots on next ingest), and this design's new reader targets the
normalized shape only. No dual-casing compatibility layer here.

### What the site already has

- One sanitize-first markdown pipeline with per-surface options (h1
  kept for repo home/changelog, stripped for the reader), a
  caller-supplied xref map, and relative-link resolution against a
  per-surface `base` path (`useRenderedSource`).
- `useRepoDocIndex` → `{byId, byPath}`: hrefs built from API data
  only; `byPath` keys ingested repo-relative doc paths.
- `RepoFrame` (three-column repo chrome), RepoNav with collapsible
  per-type drawers, `useRepoFacts` for counts.
- The `changelog` reserved-segment precedent in the router, and
  `changelogConfig` as the snapshot-gate precedent.
- Palette + directory rendering `SearchHit`s, with facet pills/chips
  excluding their own dimension via limit-0 queries; snippets render
  only through `snippet.tsx`.

## Detailed Design

### Component 1: spec re-vendor at 1.4.1

Re-vendor `api/openapi.yaml` from docz-api main (`v0.8.1`) and
`bun run gen-api`.
This is **not** editorial: the generated client gains `listRepoPages`
and `getRepoPage` hooks and `PageList`/`PageSummary`/`Page` types, and
`SearchHit` gains two required fields — every fixture and test that
builds a `SearchHit` fails to typecheck until it carries
`source`/`path`. That forced sweep is the point: fixture doc hits gain
`source: "doc"` plus their real repo paths, and the fixture search
handler starts emitting the `source` facet counts so palette/directory
suites exercise the real shape.

### Component 2: the api-block snapshot reader

Depends on the upstream normalization chain (docz#89 → docz-api#25;
see the found-bug section): the snapshot's key spellings become the
`.docz.yaml` ones, so `changelogConfig` needs **no change** (it
already reads that shape) and the new reader targets it directly:

```ts
export interface ApiBlockConfig {
  landingPage: string;        // resolved; "" only on wrong shapes
  additionalDocs: string[];   // normalized repo-relative paths
}
/** undefined unless the api: block is present AND enabled === true;
 *  any wrong shape — including a pre-normalization capitalized
 *  snapshot from a not-yet-re-ingested repo — reads as "no pages
 *  surface". */
export function apiConfig(snapshot: unknown): ApiBlockConfig | undefined;
```

`additionalDocs` runs through `arr()` semantics (null → `[]`). The
reader is the **only** gate the nav and route need — a repo without an
enabled block never fires a pages request. A stale capitalized
snapshot (a repo that hasn't re-ingested since the docz-api bump)
degrades to exactly today's behavior — no pages row, no requests —
which is the quiet-gate semantics working as designed.

Fixtures stay lowercase, which is now the verified real shape; one
fixture repo keeps `Exclude`/`AdditionalDocs`-style `null` slices so
the `arr()` path stays exercised (the `aliases: null` realism
precedent).

### Component 3: the page index and link resolution

`useRepoDocIndex` grows into the repo's one link index:

- A `useListRepoPages(owner, name)` query joins the existing
  `getRepo + listDocs` set, **gated on `apiConfig`** (skipped —
  `enabled: false` query — when the block is absent, so non-opted
  repos are untouched).
- `byPath` gains page entries keyed by each page's **reconstructed
  source path**, mapping to `/:owner/:repo/pages/<published-path>`.
  Reconstruction inverts the published-path mapping using only data
  the site already holds (`docs_dir` from `getRepo`, `additionalDocs`
  from `apiConfig`):

| Published path | Source path used as the `byPath` key |
| -------------- | ------------------------------------ |
| ∈ `additionalDocs` | itself (repo-relative already) |
| ends `.md` | `<docs_dir>/<path>` |
| extensionless (directory page) | `<docs_dir>/<path>/README.md` **and** `…/index.md` (two keys, one href — either file may be the source, and resolution only needs the right directory) |

- The reader's own-path drop generalizes: a page body drops its own
  source path from the map exactly as a doc body drops its own
  `doc_id`.
- Because `byPath` is an exact-match whitelist and hrefs are built
  from API data only, the security posture is unchanged — a page
  body's hostile relative link either misses the map (stays inert
  text) or lands on a same-repo SPA route the API itself published.

Relative-link **bases** (the other half of resolution):

- Page reader: the page's reconstructed source path (table above).
- Repo home: today's hard-wired `docs_dir/index.md` becomes
  `apiConfig(snapshot)?.landingPage ?? docs_dir/index.md` — the
  landing page may be relocated (even outside `docs_dir`), and the
  base must follow or every relative link on a relocated home
  resolves from the wrong directory.
- Doc reader and changelog: unchanged.

The render-cache fingerprint already covers link-map changes (fnv1a
over both key sets); page entries ride it with no new mechanism.

### Component 4: routes, reader, and nav

**Routes.** One new reserved segment, mirroring `changelog`:

```
:owner/:repo/pages/*     → src/routes/page.tsx   (splat = published path)
```

Registered above `:type` — static outranks dynamic, so `pages` joins
`changelog` as a reserved word: a doc type literally named "pages" is
reachable only via its id_prefix/alias URL (the DESIGN-0002 budget,
spent the same way). An empty splat (`/pages/`, `/pages`) renders the
repo's page list — or 404s — per OQ-2's outcome.

**Page reader** (`src/routes/page.tsx`): mounts inside `RepoFrame`
(breadcrumbs: home · pages · path segments), fetches `getRepoPage`
with the splat **percent-encoded as one segment** (the generated
client path-encodes its params; the route passes the raw splat
string), renders through `useRenderedSource` with the **h1 kept** (the
repo-home/changelog precedent — pages are arbitrary markdown that own
their heading), ToC rail from the collector, metadata footer line
(`git_sha`, source path) rather than the doc meta table (pages carry
no docz metadata). 404 renders the neutral not-found panel — an
invalid or traversal path is indistinguishable from a miss upstream,
and the site treats it identically. A `503` is the retryable error
panel per DESIGN-0003's invariant. Doc-link prefetch wiring
(`usePrefetchDoc` pattern) extends to page links.

**Nav.** RepoNav gains a Pages section between the type drawers and
the (gated) changelog row, rendered only when `apiConfig` returns a
config AND `listRepoPages` returns a non-empty list. Presentation is
OQ-2; the recommendation is a collapsible tree built from the flat
list (directories as branch nodes — which are themselves links when a
directory page exists — files as leaves, titles from `PageSummary`,
ordered as the API returns them). Hover/focus prefetches `getRepoPage`
like the changelog row prefetches its content.

### Component 5: search surfaces

- **Palette**: page hits render with the `source` distinction — no
  type badge (there is no type), a neutral "page" marker instead,
  title + repo + published path, navigating to the page route. The
  highlighted-hit prefetch extends to `getRepoPage`. Recents handling
  is OQ-5.
- **Directory**: page hits appear inline; the type/status/author
  columns render "—" (the empty-string-means-unset wire convention,
  same as the missing `updated_at`), and the title cell links to the
  page route. The `source` facet appears as counts alongside the
  existing facets, but no filter control ships until the upstream
  `source` query param exists (OQ-3; the additive ask is part of this
  design's rollout, the `sort=`/`updated_at` precedent).
- **Snippets** are unchanged: page-hit snippets flow through
  `snippet.tsx` exactly like doc snippets — untrusted, `<em>`-split,
  everything else text.

### Cross-cutting: security invariants

- Pages are untrusted markdown; they enter the **same** pipeline
  (sanitize after raw, highlight after sanitize). No `schema.ts`
  changes; nothing new renders outside the pipeline.
- All page hrefs are built from API data (`PageSummary.path`,
  published paths) — never from document text. The `byPath` map stays
  an exact-match whitelist.
- The route splat is passed to the generated client as data (the
  client percent-encodes it); it is never string-built into a URL by
  hand and never rendered as HTML.
- No new storage. Recents (if OQ-5a) stay coordinates+title only,
  segment-validated, same key.
- 404-on-invalid-path upstream means the site needs no path
  validation of its own for fetching — but the recents/deep-link
  validation that exists keeps rejecting non-path shapes on read.

## API / Interface Changes

Site-internal only; consumed API changes are spec `1.4.0`'s
(documented above). New/changed site modules:

| Module | Change |
| ------ | ------ |
| `src/lib/apiConfig.ts` | new — defensive `api:` block reader |
| `src/lib/changelogConfig.ts` | unchanged — becomes correct when docz#89/docz-api#25 ship |
| `src/hooks/useRepoDocIndex.ts` | pages join `byPath`; page-source reconstruction |
| `src/routes/page.tsx` | new — page reader (splat route) |
| `src/app/router.tsx` | `:owner/:repo/pages/*` reserved segment |
| `src/components/repo-nav.tsx` | Pages section (tree per OQ-2) |
| `src/components/command-palette.tsx` | page-hit rendering + prefetch |
| `src/routes/directory.tsx` | page-hit rows; source facet counts |
| `src/routes/repo-home.tsx` | relative-link base follows `landingPage` |
| `src/mocks/fixtures.ts` | pages fixtures; api-block snapshots; SearchHit `source`/`path` |

No chart, env, or `__DOCZ_CONFIG__` changes.

## Data Model

None owned by the site. Generated OpenAPI types are the data model:
`PageList`/`PageSummary`/`Page` arrive from the spec; `SearchHit`
gains required `source`/`path`. The only site-side derived structure
is the page tree built in-memory from the flat list, and the extended
`byPath` map.

## Testing Strategy

- **Unit — readers:** `apiConfig` table tests over wrong shapes,
  `null` slices, disabled blocks, and a stale capitalized snapshot
  (reads as "no pages surface" — the pre-re-ingest degradation).
- **Unit — reconstruction:** published-path → source-path table
  (additional_docs member, `.md` file, extensionless directory page
  with both README/index keys).
- **Route:** page reader four states (skeleton/content/404/error);
  splat paths with slashes and percent-encodable characters; reserved
  `pages` outranks `:type`; empty-splat behavior per OQ-2.
- **Link resolution:** doc body → page link resolves; page body → doc
  link resolves; page's own source path dropped; misses stay text.
  The XSS suite's resolver-active section gains page-target cases
  (hostile relative hrefs, traversal past root — fail closed).
- **Nav:** Pages section absent without the block (and zero page
  requests fired — the changelog-gate test shape), present with it,
  tree collapse/expand, prefetch.
- **Search:** palette and directory page-hit rendering ("—" columns,
  navigation); source facet counts; fixture SearchHits carry
  `source`/`path`.
- **Axe:** page reader + nav-with-pages entries join the sweep.
- **e2e:** one journey — repo nav → page tree → open page → rendered
  markdown (and the mermaid chunk stays off page routes without
  diagrams). Bundle budget unchanged: the page reader reuses the lazy
  pipeline.

## Migration / Rollout Plan

1. **Upstream first (blocking): SHIPPED 2026-08-30.** docz#89 → docz
   `v1.2.2` (json tags); docz-api#25 → `v0.8.1` (pin bump,
   contract-pinned snapshot shape, spec `1.4.1` documents the
   spellings). Existing repos pick up normalized snapshots on their
   next ingest — a fleet nudge clears stragglers at current scale,
   and the shipped changelog row starts working with zero site
   changes.
2. Land the site work as one `minor` release (OQ-6): spec re-vendor
   first (fixtures sweep), reader, then routes/nav/search —
   sequencing is the IMPL's business.
3. Remaining upstream ask, additive and non-blocking: a `source`
   filter param on `searchDocs` (unblocks the directory filter
   control as a follow-up).
4. Dogfood: docz-site's and docz-api's own repos enable the `api:`
   block upstream once deployed; the demo fixtures mirror that state.
5. Close issue #21 on merge.

## Open Questions

Answer each with a letter — **a is the recommendation**, b onward are
alternatives; write in your own option if none fits.

**Reviewed 2026-08-30 — decided: 1 other (upstream-first; shipped as
docz v1.2.2 / docz-api v0.8.1), 2a, 3a, 4a, 5a, 6a.** The detailed
design above is written to the decided options: Pages nav tree with
empty-splat redirect home (2a), inline directory page hits with facet
counts and no filter control until the upstream `source` param lands
(3a), pages in the link resolver both directions (4a), recents grow a
`kind` with page-path validation (5a), one `minor` release → v0.6.0
(6a).

**OQ-1 — The config_snapshot casing bug: how does the site read it?**

**Answered 2026-08-30 — other (upstream-first):** fix the root cause
before any site work, so no compatibility layer gets baked in here.
Filed as [docz#89](https://github.com/donaldgifford/docz/issues/89)
(json tags mirroring the yaml spellings; purely additive — docz never
JSON-marshals `Config` itself) and
[docz-api#25](https://github.com/donaldgifford/docz-api/issues/25)
(pin bump + contract-pin the marshaled shape; snapshots refresh on
each repo's next ingest). The site then reads the normalized
lowercase shape only; `changelogConfig` needs no change at all.
Original options kept for the record:

- a. Read **both** casings via one shared helper, split fixtures
  across the two spellings, file the upstream ask as non-blocking.
  Works against every docz-api version, but ships a compatibility
  layer the site then carries (and later cleans up) forever.
- b. Read the capitalized spelling only (match today's wire exactly).
  Honest about reality but breaks the day upstream normalizes.
- c. Block this design on upstream normalizing first — what was
  chosen, minus a's helper: the casing chain is quick to ship
  upstream and this design is already gated on answering its OQs.

**OQ-2 — How does the repo nav present pages?**

- **a (recommended).** A collapsible **tree** section ("Pages") built
  from the flat list: directory nodes collapse (and are links when a
  directory page exists), files are leaf links with `PageSummary`
  titles, drawer behavior copied from the type drawers (caret peeks,
  route auto-expands the active branch). `/pages` with an empty splat
  redirects to the repo home (the landing page **is** the repo home;
  there is no separate index to render). Matches the "URL mirrors the
  tree" rule and scales past a handful of pages.
- b. A flat list capped at N with a "view all" page at `/pages`.
  Simpler, but a 30-page repo turns the nav into a wall, and it
  spends the empty-splat route on a page that duplicates the nav.
- c. No nav section — pages reachable via search/palette and in-body
  links only. Cheapest, but rendered-but-undiscoverable pages fail
  the feature's point.

**OQ-3 — Page hits in the directory before a source filter param exists?**

- **a (recommended).** Render page hits inline now (source badge, "—"
  doc columns), show the `source` facet as counts, ship **no filter
  control** until the upstream param lands (ask filed in rollout).
  Pages become findable everywhere immediately; the directory stays
  honest (no client-side filtering that breaks pagination); the
  control is a small follow-up when the param exists — the exact
  `updated_at`/`sort=` playbook.
- b. Suppress page hits in the directory (drop client-side),
  palette-only until the param lands. Keeps the directory pure-docs,
  but silently hides results the API returned and makes total counts
  lie.
- c. Block the search surfaces on the upstream param. Couples this
  release to an upstream one for a filter control nobody has asked
  for yet.

**OQ-4 — Do pages join the relative-link resolver maps?**

- **a (recommended).** Yes, both directions: page source paths join
  `byPath` (doc bodies can link pages), and page bodies resolve
  through the same map (pages can link docs and each other). One map,
  one transform, the whitelist posture unchanged — and cross-linking
  is precisely what "the URL mirrors the tree" is for.
- b. Pages resolve links *to* docs, but doc bodies don't link *to*
  pages (byPath stays docs-only). Halves the map work, breaks the
  common "see CONTRIBUTING.md" direction.
- c. Defer all page link resolution. Links in pages render as inert
  text or dead relative hrefs — a worse reading experience than the
  changelog shipped with.

**OQ-5 — Do pages enter the palette's recent-docs store?**

- **a (recommended).** Yes: the stored entry gains a `kind`
  (`"doc" | "page"`) and page entries store the published path in the
  existing coordinates slot, path-segment validation extended to
  allow `/` in that one field for page entries only. Recents stay
  coordinates+title, capped, malformed-resets — the palette treats
  both kinds uniformly.
- b. Docs only — pages never enter recents. Simpler store, but the
  palette's empty state stops reflecting where you actually were.

**OQ-6 — Release label?**

- **a (recommended).** `minor` — additive surface (new routes, new
  consumed endpoints), v0.6.0, appVersion/chart bump per the
  IMPL-0003 convention.
- b. `patch` — undersells new user-facing routes and a consumed spec
  minor.

## References

- Issue [#21](https://github.com/donaldgifford/docz-site/issues/21) —
  upstream coordination notes (this design implements them)
- docz-api DESIGN-0004 — "Consume the docz v1.2.0 api block" (the
  serving side: published-path mapping, flat list rationale,
  exact-byte lookup, reserved words)
- docz-api spec `1.4.1` (`api/openapi.yaml` at `v0.8.1`) — the
  vendored contract this consumes, `config_snapshot` spellings
  included; docz `v1.2.2` — the json-tags release (docz#89)
- docz `v1.2.0` `pkg/doczcore/config` — `APIConfig` (yaml-only tags;
  the casing evidence), load-time landing-page normalization
- DESIGN-0002 — relative-link resolution, `byPath`, the
  reserved-word budget; DESIGN-0003 — 503-never-logout (the page
  reader's error states inherit it)
- `src/lib/changelogConfig.ts` — the snapshot-gate precedent (and
  half of the casing bug)
- INV-0005 / IMPL-0003 — changelog page precedents (h1 kept, quiet
  404, prefetch row)
