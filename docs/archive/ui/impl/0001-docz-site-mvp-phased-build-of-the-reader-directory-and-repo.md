---
id: IMPL-0001
title: "docz-site MVP: phased build of the reader, directory, and repo pages"
status: Draft
author: Donald Gifford
created: 2026-07-10
---
<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL 0001: docz-site MVP: phased build of the reader, directory, and repo pages

**Status:** Draft
**Author:** Donald Gifford
**Date:** 2026-07-10

<!--toc:start-->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 0: Sweep and scaffold](#phase-0-sweep-and-scaffold)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 1: The reader, end-to-end](#phase-1-the-reader-end-to-end)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 2: Directory + command palette](#phase-2-directory--command-palette)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 3: Repo pages](#phase-3-repo-pages)
    - [Tasks](#tasks-3)
    - [Success Criteria](#success-criteria-3)
  - [Phase 4: Polish — MVP complete](#phase-4-polish--mvp-complete)
    - [Tasks](#tasks-4)
    - [Success Criteria](#success-criteria-4)
  - [Phase 5: Auth UX (post-MVP)](#phase-5-auth-ux-post-mvp)
    - [Tasks](#tasks-5)
    - [Success Criteria](#success-criteria-5)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Dependencies](#dependencies)
- [Open Questions](#open-questions)
- [References](#references)
<!--toc:end-->

## Objective

Build the docz-site MVP — the cross-repo docz reader, search directory, and
TechDocs-style repo pages — exactly as specified by DESIGN-0001, in six
phases that mirror its rollout slices (0–4 = MVP, 5 = post-MVP auth UX).
Each phase is independently shippable; a phase is complete when every task
is checked and its success criteria hold.

**Implements:** DESIGN-0001 (`docs/design/0001-docz-site-cross-repo-docz-reader-and-search-ui.md`)

Locked context from the design: Vite + React 19 SPA (TS strict), react-router
library mode, TanStack Query + orval-generated client from the vendored
docz-api OpenAPI 3.1 spec (v1.0.0), Tailwind v4 with the mockup tokens, cmdk
palette, unified/rehype-sanitize reader pipeline with Shiki, Bun + mise +
justfile, same-origin deployment, `mockup.html` as visual source of truth.

## Scope

### In Scope

- Phases 0–4: template sweep, scaffold, reader pipeline + XSS suite,
  search-backed directory, ⌘K palette, repos grid, repo home + type pages,
  three-column reader portal, responsive/a11y/perf polish, Playwright e2e,
  same-origin deploy container.
- Phase 5 (post-MVP): `/login` page, session/avatar/logout, 401 redirect
  with destination restore.
- The MVP-era 401 behavior: a bare "session required" panel linking to
  `/auth/login?provider=github`.

### Out of Scope

- Any docz-api change. The additive asks in DESIGN-0001 (`sort=` on search,
  `index.md`/`README.md` bodies, lifecycle dates, link graph, labels) are
  separate docz-api proposals; every consumer here has a graceful fallback.
- Mockup features gated on those asks: relationship banners,
  References/Referenced-by footer, labels, lifecycle dates, rendered repo
  `index.md`, "not ingested" raw files, PDF export.
- MCP and API marketing pages; docz-mcp; team docs repos; SSR; per-user
  authorization UI.

## Implementation Phases

Each phase builds on the previous one. A phase is complete when all its
tasks are checked off and its success criteria are met. Phase numbers match
DESIGN-0001's rollout slices.

---

### Phase 0: Sweep and scaffold

Remove the leftover `rfc-site` template and stand up the decided toolchain:
Bun, Vite, React 19, TypeScript strict, Tailwind v4 with the mockup tokens,
the vendored spec + orval codegen, test scaffolding, and CI. At the end the
repo is a clean, green shell app.

#### Tasks

- [x] Sweep template leftovers: delete `react-router.config.ts`, the stale
      `orval.config.ts`, `vite.config.ts`, `vitest.config.ts`,
      `eslint.config.js`, `charts/temp/`, `Dockerfile`,
      `docker-compose.yml`, `docker-bake.hcl`, `justfile.docker`, duplicate
      prettier config (`.prettierrc.json` vs `.prettierrc.yaml` — keep one),
      and `package.json` script references to missing files
      (`scripts/gen-api-check.sh`)
- [x] Remove superseded `docz-site-mockup3.html` (`mockup.html` is the
      source of truth)
- [x] Rewrite `package.json`: name `docz-site`, accurate description,
      license aligned with the README (Apache-2.0), scripts for dev,
      dev:msw, build, preview, typecheck, lint, format, test, gen-api,
      gen-api:check
- [x] Scaffold Vite + React 19 + TS strict: new `vite.config.ts`
      (`@vitejs/plugin-react`, `@tailwindcss/vite`, dev `server.proxy` for
      `/api`, `/auth`, `/openapi.yaml` → local docz-api), strict
      `tsconfig.json` with `@/` path alias, `index.html`, `src/main.tsx`
- [x] Port `mockup.html` `:root` tokens into Tailwind v4 `@theme`
      (`src/theme/tokens.css`): bg/fg/border scales, accent, `--st-*`
      status colors, `--t-*` type colors, font stacks, zero radius,
      focus-visible style
- [x] Wire fonts via self-hosted `@fontsource` packages (`ibm-plex-mono`,
      `ibm-plex-sans`, `source-serif-4`) — no third-party font requests
- [x] App shell: topbar (brand mark, search affordance, nav, avatar
      placeholder) + `createBrowserRouter` route table — `/`, `/repos`,
      `/:owner/:repo`, `/:owner/:repo/:type`, `/:owner/:repo/:type/:docId`,
      `*` — placeholder elements, route-level `lazy()`
- [x] TanStack Query provider with sane defaults; `src/api/fetcher.ts`
      mutator (same-origin base, JSON handling, error mapping: 401 →
      `SessionRequiredError`, 404 → `NotFoundError`, other → `ApiError`)
- [x] Vendor `api/openapi.yaml` from docz-api at `info.version` 1.0.0
- [x] New `orval.config.ts`: `client: react-query`, `httpClient: fetch`,
      custom mutator, `mock: true`, output `src/api/__generated__/`
      (gitignored, `clean: true`)
- [x] `scripts/gen-api-check.sh`: regenerate + diff on the generated dir
      (PR gate; the dir is gitignored so the gate is snapshot →
      regenerate → `diff -r`, not `git diff`)
- [x] Spec drift workflow (scheduled + on-PR): fetch docz-api main's
      `api/openapi.yaml` and compare `info.version` to the vendored copy —
      the same regenerate-and-diff pattern as the git-cliff CHANGELOG
      drift check; drift opens/updates a tracking issue, never fails PRs
      (implemented as a full-content diff with `info.version` reported,
      since upstream has shipped spec changes without a version bump)
- [x] Test scaffolding: Vitest + Testing Library + MSW
      (`src/test/setup.ts`, server from generated handlers) in jsdom; one
      smoke test (shell renders, a generated hook resolves against MSW)
- [x] Lint/format: ESLint (typescript-eslint, react-hooks, jsx-a11y) +
      Prettier; keep markdownlint/yamllint configs
- [x] Rewrite `mise.toml` (pin bun; drop unused tools) and `justfile`
      (dev, dev-msw, gen-api, gen-api-check, lint, fmt, test, build,
      preview, ci)
- [x] GitHub Actions `ci.yml`: bun setup → install → lint → typecheck →
      test → build → gen-api drift check
- [x] Prune inherited `.github/` automation: the template ships ~10
      workflows (codeql, security, trufflehog, changelog, release,
      license-check, pr-labels, dependabot-severity-label, …) written for
      the template's stack, and their *scheduled* triggers don't honor
      `[skip ci]` — rewrite `ci.yml` (above), add the spec-drift workflow,
      keep security scanners only where reconfigured for the TS/Bun stack,
      delete the rest
- [x] Remove `.github/dependabot.yml` — Renovate (`renovate.json5`) owns
      dependency automation; audit its config for bun/npm coverage of the
      new stack
- [x] Rewrite `README.md` quickstart (mise install → bun install →
      just dev / just dev-msw)

#### Success Criteria

- `bun install && just dev` serves the shell app proxied to a local
  docz-api; `just dev-msw` works with no backend running
- `just ci` (lint + typecheck + test + build + gen-api-check) passes
  locally and in GitHub Actions
- The generated client compiles under strict TS and exposes hooks for all
  nine SPA-facing spec operations (the vendored 1.0.0 spec also documents
  `authCallback` and `githubWebhook` — server-side surfaces the SPA never
  calls; generated code for them is inert)
- `grep -ri "rfc-site"` across the repo returns nothing outside
  `docs/input.md`, `mockup.html` demo copy, and the design/impl docs'
  own history of the sweep; no template leftovers remain at the root
- The Actions tab shows only intended workflows (ci, spec-drift, and any
  deliberately kept, reconfigured scanners); `dependabot.yml` is gone and
  Renovate covers the bun/npm stack

---

### Phase 1: The reader, end-to-end

The thin vertical slice and the security-critical path: fetch `raw_md`,
run the sanitizing pipeline, render the document with its ToC, metadata,
and lifecycle rails. The XSS suite lands here first and gates CI from this
phase onward.

#### Tasks

- [x] `src/markdown/preprocess.ts`: strip the YAML frontmatter block and
      the `<!--toc:start-->…<!--toc:end-->` marker block from `raw_md`
- [x] `src/markdown/schema.ts`: rehype-sanitize schema — GitHub default
      extended with only the `id`/`class` attributes the slugger and Shiki
      need; unit tests pin the schema's shape
- [x] `src/markdown/processor.ts`: unified pipeline in the design's fixed
      order — `remark-parse` + `remark-gfm` → `remark-rehype`
      (`allowDangerousHtml`) → `rehype-raw` → `rehype-sanitize` →
      `rehype-slug` + ToC collector (h2–h4 → `{depth, text, id}`) →
      `@shikijs/rehype` (slim grammar set: yaml, go, ts/js, bash, json,
      hcl, sql, python; lazy-loaded) → hast-to-React (no
      `dangerouslySetInnerHTML`)
- [x] XSS suite (`processor.xss.test.ts`): payload table — `<script>`,
      `<img onerror>`, `javascript:` URLs (markdown links and raw HTML),
      event-handler attributes, `<iframe>`/`<object>`/`<embed>`, hostile
      inline SVG/MathML, `data:` URIs — every payload neutralized;
      companion benign suite (GFM tables, fenced code, footnotes, images,
      blockquotes) survives intact; slug stability asserted
- [x] `src/lib/colors.ts`: curated type→color map (rfc, adr, design, impl,
      investigation/inv, mandate, guide, principle, policy, framework —
      values from the mockup `--t-*` tokens); deterministic hash into a
      fixed 8-color palette for unknown types; case-insensitive status
      convention (draft/open → amber; proposed/in review/in progress →
      blue; accepted/active/approved/adopted/completed/implemented/
      concluded → green; rejected/cancelled/abandoned → red; superseded/
      deprecated/archived/paused → purple/grey; unknown → neutral); tests
      cover determinism and fallbacks
- [x] Type badge / status badge / status pill components per mockup styles
- [x] Reader route `/:owner/:repo/:type/:docId`: `useGetDoc`, skeleton
      article, inline error + retry, neutral not-found panel ("Not found —
      or not visible to you"), bare 401 session-required panel linking
      `/auth/login?provider=github`
- [x] Center column: file-path breadcrumb (from `path`), id line
      (`DESIGN / 0009`), title, status pill · author · updated
- [x] Right rail: "On this page" ToC (sticky wide / disclosure narrow),
      trimmed metadata card (omit `""` fields; "all fields · json →" link
      to the document endpoint), formats list ("md · source" view of the
      already-fetched `raw_md`; "json" endpoint link)
- [x] Lifecycle rail: type `statuses` via `listTypes` (cached per repo),
      current status as the active stop, position-only
- [x] Memoize pipeline output per `(doc_id, content_hash)`
- [x] Fixtures: a curated "demo org" (2–3 repos of real docz markdown —
      this repo's DESIGN-0001, docz-api docs) layered over orval's faker
      handlers, so rendering truth is exercised alongside shape coverage
- [x] Component tests: reader four-state matrix, metadata `""` omission,
      lifecycle positioning, ToC anchor navigation

#### Success Criteria

- A real document renders identically (modulo data) from a local docz-api
  and from MSW fixtures at `/:owner/:repo/:type/:docId`
- The XSS suite is green and marked required in CI; every payload is
  neutralized and every benign construct survives
- This repo's own DESIGN-0001 renders correctly as a fixture — frontmatter
  and ToC markers stripped, tables and fenced code intact, ToC anchors
  navigate
- An unknown custom type and unknown status render with deterministic
  fallback/neutral colors (no hardcoded type list anywhere)

> Phase 1 verification note (2026-07-11): criteria 2–4 are pinned by the
> test suite (XSS gate, DESIGN-0001 fixture matrix, color fallbacks).
> Criterion 1's "from a local docz-api" leg was verified by construction
> only — no local docz-api stack was running; the app has no
> fixture-specific rendering path (MSW serves the same wire contract the
> generated client pins), so re-checking against a live API when one is
> up is a formality, not a code change.

---

### Phase 2: Directory + command palette

The cross-repo search surfaces: the URL-bound directory table at `/` and
the ⌘K palette overlay. Both ride `searchDocs`; snippets are treated as
untrusted.

#### Tasks

- [x] `src/lib/searchParams.ts`: typed URL param helper (`q`, `repo`,
      `type[]`, `status[]`, `author[]`, `offset`) with parse/serialize
      round-trip tests
- [x] Directory route `/`: `searchDocs` bound to URL params, debounced
      `q` (~200 ms), relative-time updated column
      (2026-07-11: the planned interim client-side `updated_at` sort is
      impossible — `SearchHit` carries no timestamp, verified in the
      vendored spec and upstream main. Hits render in API order and the
      updated column renders "—"; the additive ask in DESIGN-0001 now
      covers both `sort=` and `updated_at` on `SearchHit`)
- [x] Repo picker dropdown fed by the `repo` facet (counts per repo);
      type chips from the union of `type` facet values, colored by the
      color system; "showing X of Y"; clear-filters action
      (facet sources exclude their own dimension — separate limit-0
      facet queries — so every repo/type stays offered while one is
      selected; chips are single-select while the API takes one `type`)
- [x] "Load more" pagination via offset/limit against
      `estimated_total_hits`
      (URL `offset` means "rows 0..offset+PAGE_SIZE shown" and the query
      fetches the whole window from 0 — a deep-linked URL renders the
      same rows, keeping the URL the only source of truth; each click
      pushes offset so back shrinks the window)
- [x] Directory four states: skeleton rows, contextual empty ("No
      documents yet — onboard a repo with the docz GitHub App" / "No
      matches — clear filters"), inline error + retry, 401 panel
- [x] Snippet renderer: escape snippet text, re-insert only `<em>` match
      markers as `<mark>`; XSS tests with hostile snippet fixtures
- [x] cmdk palette: opened by ⌘K and `/`, closed by Esc; debounced
      `searchDocs`; results grouped by repo; preview pane (title, status,
      snippet — no extra fetch); filter pills (all / per-repo /
      per-type); ↑/↓/↵/Tab keyboard flow; Enter navigates to the reader
      (Tab steps the highlight/preview; pill sources come from a
      pill-free facet query so every repo/type stays offered; palette
      state is local — the page URL is untouched until Enter)
- [x] Topbar search affordance opens the palette
- [ ] Component tests: URL round-tripping (params ↔ UI state), facet count
      rendering, palette keyboard flow, snippet sanitization

#### Success Criteria

- Filter state survives reload and sharing — the URL is the only source of
  filter truth; back/forward walks filter history
- The palette opens on every route, finds documents by title and body, and
  navigates on Enter
- Hostile snippet fixtures render inert; the snippet suite is green in CI
- An empty query shows all visible docs within the fetched page (API index
  order — `SearchHit` carries no `updated_at` to sort by; see the directory
  task note)

---

### Phase 3: Repo pages

The browse-by-repo complement: the `/repos` grid, the shared repo nav, the
generated repo home and type pages, and the reader integrated into the
full three-column portal.

#### Tasks

- [ ] Repo-facts hook: per-repo totals and per-type doc counts from a
      repo-filtered `searchDocs` facet query, cached per repo
- [ ] `/repos` grid: one card per `listRepos` entry — name, default
      branch, per-type counts, doc total, last-updated — linking to the
      repo home
- [ ] Shared repo-nav component: identity header (letter mark, name,
      `branch · docz.yaml`), Home item, per-type items with counts, docs
      nested beneath each type, active item highlighting
- [ ] Repo home `/:owner/:repo` (Decision 8): generated client-side from
      `getRepo` + `listTypes` — type sections with counts and the
      mockup's "No index.md configured" note; frame ready to render
      `index.md` through the reader pipeline when the API serves it
- [ ] Type page `/:owner/:repo/:type`: synthesized README-style page —
      `plural_label`, blurb (curated for standard types, generic
      fallback), `docz create …` hint, doc table (ID, title, status
      badge, date, filename from `path`)
- [ ] Wire the reader into the three-column portal with the repo nav;
      collapse behavior for narrow screens
- [ ] File-path breadcrumbs consistent across `/repos` → repo → type →
      doc
- [ ] Type-in-URL resolution: generate links from the canonical type name;
      accept `id_prefix`/alias URLs (the API resolves them)
- [ ] Four-state coverage + component tests for repos grid, repo home, and
      type pages

#### Success Criteria

- Full drill-down works with deep links at every level: `/repos`,
  `/:owner/:repo`, `/:owner/:repo/:type`, and the reader all load directly
  from a cold URL
- Counts agree across repo cards, repo nav, and directory facets for the
  same data set
- A type with zero documents shows the docz-create empty state; a repo
  with no `index.md` shows the generated home
- Sibling-doc navigation in the reader swaps documents without a full
  reload (query cache hit)

---

### Phase 4: Polish — MVP complete

Responsive behavior, accessibility, performance, e2e coverage, and the
same-origin deploy artifact. When this phase closes, the MVP is deployable
and demoable end-to-end.

#### Tasks

- [ ] Responsive passes at the mockup's breakpoints (~680/760/860 px):
      rails collapse, ToC becomes a disclosure, palette goes full-screen
      on small viewports, repo nav becomes a drawer
- [ ] Xref linking: doc-id-shaped tokens (built from the repo's
      `id_prefix` set) in rendered bodies that match a sibling doc become
      router links
- [ ] Accessibility: axe checks in component tests (zero serious/critical
      on core views), keyboard paths (palette, chips, picker, ToC, repo
      nav), visible focus states, landmark/heading structure, badge color
      contrast
- [ ] Performance: verify route-level code-splitting and lazy Shiki
      grammars; add a CI bundle-size budget for the initial chunk;
      prefetch doc data on link hover
- [ ] Playwright e2e against a `vite preview` of an MSW-enabled build
      (browser worker, same fixtures as unit tests): directory → filter →
      open doc; palette search → open doc; cold deep-link into the
      reader; 404 route; MVP 401 panel
- [ ] Deploy artifact: multi-stage Dockerfile — `bun run build` → a small
      `Bun.serve` static server (`server/serve.ts`: assets with cache
      headers, precompressed where it pays, SPA fallback to `index.html`,
      `/healthz`) on a slim Bun base; compose file routes `/api`, `/auth`,
      `/webhooks`, `/openapi.yaml` to docz-api behind one origin
- [ ] Replace or remove `charts/temp` (real chart only when a deploy
      target exists)
- [ ] Sweep TODO/FIXME; refresh README (dev, test, build, deploy
      sections)

#### Success Criteria

- e2e suite green in CI; axe reports zero serious/critical violations on
  directory, reader, repos, and type views
- `docker compose up` serves site + API same-origin; after one visit to
  `/auth/login?provider=github`, full browse/search/read works
- A keyboard-only user can search, filter, navigate repos, and read a
  document
- CI enforces the full gate: lint, typecheck, unit/component (including
  the XSS suites), e2e, codegen drift, bundle budget

---

### Phase 5: Auth UX (post-MVP)

The deliberate post-MVP slice: a real login surface and session-aware
chrome. docz-api already owns the flow and the cookie; this phase is UX
only.

#### Tasks

- [ ] `/login` provider page: GitHub primary; Okta/Keycloak buttons shown
      when enabled by config
- [ ] 401 handling upgraded from panel to redirect: stash the intended
      path in sessionStorage → `/login`; restore the destination on first
      authenticated load (the API callback always lands on `/`)
- [ ] `getSession`-backed avatar menu (GitHub `login` or OIDC `email`);
      logout: `POST /api/v1/auth/logout`, clear the TanStack Query cache,
      return to `/login`
- [ ] Remember last-used provider in localStorage
- [ ] Tests: 401 → redirect → destination restore; logout clears cache;
      e2e login loop with a mocked provider callback

#### Success Criteria

- Full sign-in/sign-out loop works against a real docz-api with GitHub;
  a deep link visited signed-out lands back on that document after login
- No token or session id is ever readable by JS — cookie only; the only
  storage writes are UI preferences

---

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `api/openapi.yaml` | Create | Vendored docz-api contract, pinned at 1.0.0 |
| `orval.config.ts` | Replace | react-query + fetch mutator + MSW mocks → `src/api/__generated__/` |
| `vite.config.ts` | Replace | React + Tailwind v4 plugins, dev proxy, build config |
| `package.json`, `mise.toml`, `justfile` | Rewrite | docz-site naming, Bun pin, task surface |
| `src/main.tsx`, `src/router.tsx`, `src/app.tsx` | Create | Entry, route table, shell (topbar + outlet) |
| `src/theme/tokens.css` | Create | Mockup `:root` tokens as Tailwind `@theme` |
| `src/api/fetcher.ts` | Create | Orval mutator: base URL, JSON, 401/404 error mapping |
| `src/markdown/{preprocess,schema,processor,toc}.ts` | Create | The reader pipeline (Phase 1) |
| `src/lib/{colors,searchParams,relativeTime,xref}.ts` | Create | Color system, URL state, time formatting, doc-id linking |
| `src/components/` | Create | Badges, pills, skeletons, state panels, breadcrumbs |
| `src/features/{reader,directory,palette,repos}/` | Create | Route-level features (Phases 1–3) |
| `src/test/{setup.ts,fixtures/}` | Create | MSW server, curated docz fixtures, XSS payload table |
| `e2e/` | Create | Playwright specs + config (Phase 4) |
| `.github/workflows/ci.yml` | Rewrite | bun → lint → typecheck → test → build → drift check |
| `Dockerfile`, `compose.yaml`, `server/serve.ts` | Rewrite/Create | Bun.serve static image + same-origin site/API composition (Phase 4) |
| `react-router.config.ts`, `charts/temp/`, `docz-site-mockup3.html`, stale configs | Delete | rfc-site template sweep (Phase 0) |

## Testing Plan

- [ ] **XSS suites** (Phase 1 reader pipeline; Phase 2 snippet renderer) —
      payload tables asserted neutralized; benign-markdown survival; slug
      stability. Required CI checks from the moment they land.
- [ ] **Unit**: preprocess (frontmatter/ToC-marker stripping), sanitize
      schema shape, ToC collector, color system determinism, searchParams
      round-trip, xref matcher.
- [ ] **Component** (Vitest + Testing Library + MSW): four-state matrix
      per data-bound view; URL ↔ facet round-trip; palette keyboard flow;
      metadata `""` omission; lifecycle positioning; count consistency.
- [ ] **Contract**: `gen-api-check` drift gate in CI; the scheduled
      spec-version drift workflow (git-cliff-style compare) against
      docz-api main.
- [ ] **E2E** (Playwright, Phase 4): the five core journeys listed in
      Phase 4, against the MSW-enabled preview build.
- [ ] **Accessibility**: axe assertions in component tests; keyboard-path
      e2e checks.

## Dependencies

- **docz-api** running locally (its `compose.yaml`) for real-API dev; MSW
  covers everything else. The MVP needs a session cookie once —
  `/auth/login?provider=github` — no site auth code before Phase 5.
- **Vendored spec v1.0.0**; re-vendor is a reviewed bump keyed to
  `info.version` (majors reconciled before upgrading).
- **Additive docz-api asks** (DESIGN-0001): `sort=`, `index.md`/README
  bodies, lifecycle dates, link graph, labels. None block any phase; each
  consumer has a specified fallback.
- **Orval + OpenAPI 3.1**: the same pairing the prior rfc-site template
  used; if generation misbehaves on 3.1 constructs, the fallback is
  `openapi-typescript` for types + thin hand-written hooks (noted here as
  a risk, not an open question — orval is proven for this shape).

## Open Questions

None open. All seven were resolved 2026-07-10 — questions 1–4, 6, and 7 as
recommended, question 5 with a better option:

1. **Fonts** → self-hosted `@fontsource` packages (`ibm-plex-mono`,
   `ibm-plex-sans`, `source-serif-4`); no third-party requests, versioned
   by Renovate.
2. **Spec-version pin** → scheduled + on-PR workflow fetches docz-api
   main's `api/openapi.yaml` and compares `info.version` to the vendored
   copy — the same regenerate-and-diff pattern as the git-cliff
   CHANGELOG drift check; drift opens/updates a tracking issue, never
   fails PRs.
3. **MSW fixtures** → curated "demo org" of real docz markdown (this
   repo's DESIGN-0001, docz-api docs) layered over orval's
   faker-generated handlers.
4. **Component-test environment** → jsdom.
5. **Prod static server** → a small **`Bun.serve` server on the Bun base
   image** ("since it's Bun, just run the binary") — one runtime across
   dev/CI/prod, ~30 lines for assets + cache headers + SPA fallback +
   `/healthz`. Accepted trade-off vs nginx: larger base image and
   hand-rolled compression (precompressed assets at build time cover
   it).
6. **Lint/format** → ESLint (typescript-eslint, react-hooks, jsx-a11y) +
   Prettier.
7. **Playwright e2e backend** → `vite preview` of an MSW-enabled build
   (browser worker, same fixtures as unit tests); the compose-stack
   variant remains a candidate nightly job, not the PR gate.

## References

- DESIGN-0001 —
  `docs/design/0001-docz-site-cross-repo-docz-reader-and-search-ui.md`
  (the design this plan implements; decisions 1–11, coverage map, rollout
  slices).
- `mockup.html` — visual source of truth for every surface built here.
- docz-api `api/openapi.yaml` (v1.0.0) + `api/README.md` — the vendored
  contract and its consumption model.
- docz-api `compose.yaml` — local real-API development.
- Tooling: orval, TanStack Query, react-router, Tailwind v4, cmdk,
  unified/remark/rehype, rehype-sanitize, Shiki, MSW, Vitest, Testing
  Library, Playwright.
