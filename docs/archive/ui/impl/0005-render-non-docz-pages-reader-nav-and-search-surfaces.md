---
id: IMPL-0005
title: "Render non-docz pages: reader, nav, and search surfaces"
status: In Progress
author: Donald Gifford
created: 2026-08-30
---

<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL-0005: Render non-docz pages: reader, nav, and search surfaces

**Status:** In Progress
**Author:** Donald Gifford
**Date:** 2026-08-30

<!--toc:start-->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: contract and gates](#phase-1-contract-and-gates)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2: page reader and link resolution](#phase-2-page-reader-and-link-resolution)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 3: discovery surfaces](#phase-3-discovery-surfaces)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 4: ship — docs, chart, release](#phase-4-ship--docs-chart-release)
    - [Tasks](#tasks-3)
    - [Success Criteria](#success-criteria-3)
- [Standing gates (every phase)](#standing-gates-every-phase)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Dependencies](#dependencies)
- [Open Questions](#open-questions)
- [References](#references)
<!--toc:end-->

## Objective

Execute DESIGN-0004 (Approved 2026-08-30; decisions 1-other/shipped,
2a 3a 4a 5a 6a): consume docz-api `v0.8.1` (spec `1.4.1`) — the docz
`v1.2.0` `api:` block surface — so the site renders non-docz pages: a
reader under the reserved `/pages/*` route family, a Pages tree in
the repo nav, page hits in the palette and directory, and page paths
joined into the relative-link resolver both directions. One `minor`
release (v0.6.0).

**Implements:** DESIGN-0004 (from issue
[#21](https://github.com/donaldgifford/docz-site/issues/21))

## Scope

### In Scope

- Spec re-vendor `1.2.1` → `1.4.1` + client regen — a breaking-fixture
  sweep (`SearchHit` gains required `source`/`path`).
- `src/lib/apiConfig.ts` — the defensive `api:` block gate over the
  now-normalized `config_snapshot` (docz v1.2.2 spellings).
- Page fixtures in the demo org (list + get resolvers, api-block
  snapshots).
- `useRepoDocIndex`: pages join `byPath` via published-path →
  source-path reconstruction; both-direction link resolution.
- Router: `:owner/:repo/pages/*` reserved splat; `src/routes/page.tsx`
  reader (h1 kept, ToC rail, four states, 503-never-logout).
- RepoNav Pages tree (design OQ-2a), palette page hits +
  `kind`-aware recents (5a), directory page rows + source counts (3a).
- CLAUDE.md/README, chart bump (appVersion `0.6.0`, chart `0.1.7`),
  the additive upstream ask for a `source` filter param.

### Out of Scope

- A directory source-filter **control** — blocked on the upstream
  `searchDocs` param (the ask is filed in Phase 4; the control is a
  small follow-up when it lands).
- Assets/images, page-tree persistence, per-page metadata beyond
  `PageSummary` (design non-goals).
- Any chart/env/`__DOCZ_CONFIG__` configuration — the feature is
  repo-driven; the site detects it from API data alone.
- Dual-casing snapshot compatibility — the upstream chain shipped
  (docz v1.2.2 / docz-api v0.8.1); a stale capitalized snapshot reads
  as "no pages surface", which is the quiet gate working.

## Implementation Phases

Each phase builds on the previous one. A phase is complete when all
its tasks are checked off and its success criteria are met. Per OQ-1,
the phases land as commits on ONE branch/PR (`feat/impl-0005`).

---

### Phase 1: contract and gates

The contract flip and everything that must absorb it, plus the
config gate — no rendered-surface changes yet.

#### Tasks

- [x] Re-vendor `api/openapi.yaml` at `1.4.1` from docz-api `v0.8.1`
      and run `bun run gen-api`; confirm the new surface (pages
      endpoints, `SearchHit.source`/`path`, `config_snapshot`
      spelling contract) and `gen-api-check` current
- [x] The forced `SearchHit` sweep: every fixture/test hit gains
      `source: "doc"` and its real repo `path`; the fixture search
      handler emits `source` facet counts; typecheck is the
      completeness proof (required fields)
- [x] `src/lib/apiConfig.ts` + tests: `apiConfig(snapshot)` →
      `{landingPage, additionalDocs} | undefined` per DESIGN-0004
      Component 2 — lowercase spellings only, `enabled !== true` →
      undefined, `arr()` semantics for null lists, wrong shapes and
      stale capitalized snapshots read as "no pages surface"
- [x] Demo-org page fixtures: `listRepoPages`/`getRepoPage` resolvers
      layered before faker (fall-through outside the demo org, the
      existing pattern); api-block `config_snapshot` on the opted
      fixture repo; content is this repo's REAL markdown (OQ-2a):
      `docs/design/README.md` + `docs/impl/README.md` as directory
      pages via `?raw`, one nested file page, one root
      `additional_docs` entry (snapshot if not in-repo); non-opted
      fixture repo returns `{"pages": []}` and no `api:` block
- [x] Fixture realism pins: one repo carries `exclude`/
      `additional_docs: null` (the `arr()` path); fixture published
      paths cover a directory page (extensionless), a nested `.md`
      file page, and an `additional_docs` root file

#### Success Criteria

- `gen-api-check` green; `just ci` green with zero rendered-surface
  changes (the sweep is type-level and fixture-level only)
- `apiConfig` table suite green, including the stale-capitalized
  degradation row
- Fixture pages listable/fetchable in tests via the generated hooks

---

### Phase 2: page reader and link resolution

The route family and the resolver work — a page deep link renders
and cross-links resolve before any discovery UI exists.

#### Tasks

- [x] Router: `:owner/:repo/pages/*` registered above `:type`
      (reserved-word comment mirroring the changelog entry); empty
      splat (`/pages`, `/pages/`) redirects to the repo home
      (design OQ-2a: the landing page IS the repo home)
- [x] `src/routes/page.tsx`: mounts in `RepoFrame` (breadcrumbs home
      · pages · path segments), four states (skeleton / content /
      NotFoundPanel / ErrorPanel-with-retry; 503 stays the retryable
      panel per DESIGN-0003), `useRenderedSource` with **h1 kept**,
      ToC rail, metadata footer line (`git_sha` short + source path)
      instead of the doc meta table
- [x] Splat → API call: pass the splat through
      `encodeURIComponent` before the generated hook — orval's URL
      builder interpolates **raw** (verified), and the spec blesses
      the one-segment percent-encoded spelling; never string-build
      the URL by hand
- [x] `useRepoDocIndex`: gated `listRepoPages` query (skipped
      entirely without an `apiConfig` hit) joins the all-resolve
      barrier; `byPath` gains page entries per the reconstruction
      table (additional_docs member → itself; `.md` → docs_dir join;
      extensionless → both `README.md` and `index.md` keys); hrefs
      `/{owner}/{name}/pages/{path}`; page bodies drop their own
      source path (the reader's own-id drop generalized)
- [x] Repo home relative-link base follows
      `apiConfig(snapshot)?.landingPage ?? docs_dir/index.md`
- [x] Recents schema (design OQ-5a): `RecentDoc` gains
      `kind: "doc" | "page"`; page entries carry the published path
      in the coordinates slot with per-segment validation (`/` split,
      each segment `SEGMENT`-checked); missing `kind` on stored
      entries is malformed → store resets; the page reader records on
      successful load
- [x] Tests: reconstruction unit table; reader four states + splat
      shapes (nested, percent-encodable, empty-splat redirect);
      reserved `pages` outranks `:type`; doc→page and page→doc link
      resolution end-to-end; own-path drop; XSS resolver-active
      section gains page-target cases (hostile hrefs, traversal —
      fail closed); recents kind round-trip + reset-on-old-shape
- [x] Axe sweep entry: page reader

#### Success Criteria

- Cold deep link into a fixture page renders through the one
  pipeline; 404/503/skeleton pinned; no login navigation on 503
- Cross-links resolve both directions in tests; misses and hostile
  links stay inert; XSS suite green
- Non-opted repos: zero pages requests fired (gate test)

---

### Phase 3: discovery surfaces

Pages become findable: nav tree, palette, directory.

#### Tasks

- [x] RepoNav Pages tree (design OQ-2a): section between the type
      drawers and the changelog row, rendered only when `apiConfig`
      hits AND the list is non-empty; tree built from the flat list
      (path-split); directory nodes collapse with the type-drawer
      caret behavior and are links when a directory page exists;
      leaves link with `PageSummary` titles; the active route's
      branch auto-expands; hover/focus prefetches `getRepoPage`
- [x] Palette: page hits render with a neutral mono "page" marker
      (no type badge), title + repo + published path, navigate to
      the page route, highlighted-hit prefetch extends to
      `getRepoPage`; recents list renders `kind`-aware hrefs and the
      `recent:` value-prefix convention holds for page entries
- [x] Directory: page hits render inline — title links to the page
      route, `source` marker, "—" for type/status/author (the
      empty-string wire convention, like `updated_at`); the
      results-count line extends to "N results · X docs · Y pages"
      from the `source` facet counts, page term omitted when Y = 0
      so non-opted deployments stay byte-identical (OQ-3a); row
      hover/focus prefetch
- [x] Tests: nav gating (absent without block — zero requests;
      present with; collapse/expand; caret vs navigate), palette
      page-hit rendering + navigation + recents, directory page rows
      + counts, prefetch wiring (landed with each surface's commit)
- [x] Axe sweep entries: repo page with the Pages tree; directory
      with mixed doc/page hits
- [x] e2e journey (one spec): repo nav → Pages tree → open a page →
      rendered markdown; assert the mermaid chunk stays off a
      diagram-free page

#### Success Criteria

- A none-opted repo's nav/directory/palette are byte-identical to
  today (unit-proven, zero pages requests)
- The opted fixture repo: every fixture page reachable from nav,
  palette, and directory; recents survive a doc+page mix
- Axe green including new entries; e2e green; bundle budget
  unchanged (the reader reuses the lazy pipeline)

---

### Phase 4: ship — docs, chart, release

#### Tasks

- [x] Update CLAUDE.md (pages surface: reserved route, apiConfig
      gate, reconstruction rule, recents kind, orval raw-interpolation
      gotcha) and README (Pages section under the feature list)
- [x] File the additive upstream ask on docz-api: a `source` filter
      param on `searchDocs` (unblocks the directory filter control
      follow-up) — filed as donaldgifford/docz-api#27
- [x] Chart bump per convention: appVersion `0.5.0` → `0.6.0`
      (label `minor`, design OQ-6a), chart `0.1.6` → `0.1.7`,
      deployment unittest image-tag assert, helm-docs regenerated
- [x] Full local gate + changelog sync (`git fetch --tags`, regen,
      cliff-skipped `chore(changelog):` commit **as the last branch
      commit** — the IMPL-0004 lesson)
- [ ] PR `feat/impl-0005` labeled `minor`; merge on green;
      sync main
- [ ] Mark this IMPL Completed and DESIGN-0004 Implemented; close
      issue #21 (the merge references it)

#### Success Criteria

- `just ci` chain green (unit, test-server, lint, `tsc -b --force`,
  build, format, bundle-budget, e2e, gen-api-check)
- helm lint + unittest green with the `0.6.0` image assert
- PR merged; v0.6.0 train cut; issue #21 closed; upstream `source`
  ask filed

---

## Standing gates (every phase)

- `just ci` semantics: test, test-server, lint, `tsc -b --force`,
  build, format:check, bundle-budget, e2e, gen-api-check
- No credential-shaped strings; no `schema.ts` widening; hrefs from
  API data only; the only storage change is the recents `kind`
  (coordinates+title, validated both directions)
- Check tasks off here as completed; update CLAUDE.md when guidance
  changes; conventional commits per task

## File Changes

| File | Action | Description |
| ---- | ------ | ----------- |
| `api/openapi.yaml` | Modify | re-vendor at 1.4.1 |
| `src/lib/apiConfig.ts` (+ test) | Create | `api:` block gate |
| `src/lib/recentDocs.ts` (+ test) | Modify | `kind`, page-path validation |
| `src/hooks/useRepoDocIndex.ts` | Modify | pages in `byPath`; reconstruction |
| `src/app/router.tsx` | Modify | `pages/*` reserved splat |
| `src/routes/page.tsx` (+ test) | Create | page reader |
| `src/routes/repo-home.tsx` | Modify | base follows `landingPage` |
| `src/components/repo-nav.tsx` (+ test) | Modify | Pages tree |
| `src/components/command-palette.tsx` (+ test) | Modify | page hits, recents |
| `src/routes/directory.tsx` (+ test) | Modify | page rows, source counts |
| `src/markdown/xss.test.*` | Modify | page-target resolver cases |
| `src/a11y/axe.test.tsx` | Modify | page reader, nav tree, mixed directory |
| `e2e/*` | Modify | pages journey |
| `src/mocks/fixtures.ts` | Modify | pages resolvers, api-block snapshot, SearchHit sweep |
| `charts/docz-site/*` | Modify | appVersion 0.6.0, chart 0.1.7, assert |
| `CLAUDE.md`, `README.md` | Modify | guidance + Pages section |

## Testing Plan

- [ ] Unit: `apiConfig` table; reconstruction table; recents kind;
      reader four states; nav gating/tree; palette + directory page
      hits
- [ ] Integration (route-level, MSW): deep-link page render;
      empty-splat redirect; doc↔page link resolution; 503 → retryable
      panel, never `/login`
- [ ] XSS: resolver-active page-target cases; sanitize order
      untouched
- [ ] Axe: three new entries; existing sweep stays green
- [ ] e2e: one pages journey; mermaid-chunk absence on pages

## Dependencies

- docz-api ≥ `v0.8.1` (spec `1.4.1`) — shipped 2026-08-30, includes
  the normalized `config_snapshot` spellings (docz v1.2.2).
- The design-doc branch (PR #22) merges first, so this IMPL's
  checkbox updates land on main-based branches (the docs-first
  precedent).
- Deployment note (not code): repos serve stale capitalized
  snapshots until their next ingest after docz-api `v0.8.1` rolls
  out — the pages gate (and the changelog row) light up per-repo as
  the fleet re-ingests.

## Open Questions

Answer each with a letter — **a is the recommendation**, b onward are
alternatives; write in your own option if none fits.

**Reviewed 2026-08-30 — decided: 1a, 2a, 3a.** Options preserved
below for the record. Consequences are folded into the phase tasks:
one branch/PR (`feat/impl-0005`) carries all four phases (1a);
the demo fixtures serve this repo's real markdown mirroring the
dogfood state (2a); the directory presents source counts on the
results-count line, page term omitted at zero (3a).

**OQ-1 — PR granularity?**

- **a (recommended).** One branch, one `minor` PR carrying all four
  phases (per-task conventional commits inside it) — the IMPL-0004
  precedent and the design's single-release decision (6a). Phases 1–2
  ship nothing user-visible alone; a stacked review adds two release
  trains for no independent value.
- b. Two PRs: contract + reader (Phases 1–2) landing dark, then
  discovery + ship (Phases 3–4). Smaller reviews, but the first PR
  ships dead routes and a dead resolver behind no UI.
- other: —

**OQ-2 — What page content do the demo fixtures carry?**

- **a (recommended).** Real markdown from this repo, mirroring the
  dogfood state the design's rollout names: the docz-site fixture
  repo enables the block with its actual `docs/design/README.md` and
  `docs/impl/README.md` (directory pages via `?raw`), one nested
  file page, and a root `additional_docs` entry (snapshot if the
  file doesn't exist in-repo). Real content exercises the pipeline
  the way fixtures always have (the `?raw`/snapshot precedent), and
  the fixture tree matches what dogfooding will actually serve.
- b. Synthetic minimal pages (three tiny authored files). Fastest,
  but demo content stops matching reality and the palette/nav demos
  read as lorem ipsum.
- other: —

**OQ-3 — How do source counts appear in the directory (no filter param yet)?**

- **a (recommended).** Extend the results-count line: "N results · X
  docs · Y pages" (from the `source` facet counts; the page term
  omitted when Y = 0, which keeps non-opted deployments
  byte-identical). Honest, zero dead controls, and trivially
  replaced by a real facet control when the upstream `source` param
  lands.
- b. A non-interactive chip group styled like the existing facet
  chips. Visually consistent, but a control-shaped thing that does
  nothing invites clicks and axe scrutiny.
- c. Row badges only — no aggregate display until the filter lands.
  Cleanest, but hides information the API already returns and the
  design's 3a asked to show it.
- other: —

## References

- DESIGN-0004 — Render non-docz pages from the docz v1.2.0 api block
  (Approved; decisions 1-other/shipped, 2a 3a 4a 5a 6a)
- Issue [#21](https://github.com/donaldgifford/docz-site/issues/21) —
  the upstream coordination issue this closes
- docz-api DESIGN-0004 / `v0.8.1` (spec `1.4.1`); docz `v1.2.2` —
  the shipped upstream chain
- IMPL-0004 — PR/branch/chart-bump conventions and the
  changelog-sync-last lesson this doc follows
- IMPL-0003 — reserved-segment (`changelog`) and fixture precedents
