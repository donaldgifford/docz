---
id: IMPL-0003
title: "Nav pins, changelog page, and doc link resolution"
status: Draft
author: Donald Gifford
created: 2026-08-11
---
<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL 0003: Nav pins, changelog page, and doc link resolution

**Status:** Draft
**Author:** Donald Gifford
**Date:** 2026-08-11

<!--toc:start-->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: repo changelog page](#phase-1-repo-changelog-page)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2: doc link resolution](#phase-2-doc-link-resolution)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 3: nav pins and docs-type curation](#phase-3-nav-pins-and-docs-type-curation)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
- [Standing gates (every phase)](#standing-gates-every-phase)
- [Open Questions](#open-questions)
- [References](#references)
<!--toc:end-->

## Objective

Execute DESIGN-0002 (Approved 2026-08-11; decisions 1a 2a 3a 4a 5a
6b): per-deployment nav pins, the repo changelog page, and relative
doc-link resolution — three additive phases, each independently
shippable as its own PR, no docz-api changes.

**Implements:** DESIGN-0002 (from INV-0003 / INV-0004 / INV-0005)

## Scope

### In Scope

- Vendored spec re-vendor 1.1.0 → 1.2.0 (`getRepoChangelog`) and the
  changelog route/page/nav row (INV-0005 decisions, all "a").
- `useRepoDocIndex` widening (`byId` + `byPath`), the post-sanitize
  relative-link transform (exact-path matching), per-surface base
  paths, and the filename-fallback redirect in the doc route.
- `DOCZ_NAV_LINKS` (JSON array format) → `__DOCZ_CONFIG__.nav` →
  AppShell pins, with `VITE_NAV_LINKS` build-time fallback; Helm
  `config.navLinks`.
- `docs` type curation (OQ-6b): `CURATED_TYPES` entry, contrast-
  checked `--color-t-docs` token, `docTypes.ts` blurb.

### Out of Scope

- Team-docs pages ingest (future DESIGN pair; DOCS-XXXX v1 needs no
  site work beyond the Phase 3 curation).
- Referenced-by backlinks / cross-doc link graph (DESIGN-0001 API
  ask).
- Changelog search indexing; external URLs in nav pins.

## Implementation Phases

Each phase builds on the previous one. A phase is complete when all
its tasks are checked off and its success criteria are met. One PR
per phase, labeled `minor` (OQ-2a); conventional commits. Every
phase PR bumps the chart — appVersion to the incoming release's
bare semver plus a chart patch bump, image-tag unittest assert
updated to match — so defaults-only installs pull the released
image (the metadata-action strips the `v`; appVersion stays bare).

---

### Phase 1: repo changelog page

The "cached repo file as page" pattern (repo home) applied to
`getRepoChangelog`, plus the gated nav row.

#### Tasks

- [ ] Re-vendor `api/openapi.yaml` at 1.2.0 from docz-api; run
      `just gen-api`; confirm `gen-api-check` is clean
- [ ] MSW fixtures: demo-org docz-site repo serves its real
      `CHANGELOG.md` via `?raw`; one fixture repo returns
      `changelog_md: ""`; all other repos fall through to the 404
      handler
- [ ] Route module `src/routes/repo-changelog.tsx` registered as
      `:owner/:repo/changelog` (static segment above `:type`); lazy
      chunk
- [ ] Page states per DESIGN-0002 Component 3: shared 401 redirect,
      repo 404 panel, quiet "no changelog" panel, empty state for
      `""`, rendered content with h1 kept (`useRenderedSource`),
      `RepoFrame` crumbs `home · changelog`, `TocList` rail
- [ ] Render memoized per `(repo, changelog_sha)`; xref resolver
      passed so doc-id mentions linkify
- [ ] RepoNav row under Home: gate on defensive
      `config_snapshot.changelog.enabled === true` read from the
      existing `useGetRepo` data; hint = basename of the configured
      file with full subpath as `title` when it differs; hover/focus
      prefetch of `getRepoChangelog`
- [ ] Unit tests: row gating (enabled / disabled / absent block /
      malformed snapshot), page states (404 / empty / content), memo
      keying; changelog view added to the axe sweep
- [ ] Chart bump per OQ-2a: appVersion to the incoming bare semver,
      chart version patch, image-tag unittest assert updated
- [ ] Update CLAUDE.md (reserved `changelog` segment, snapshot-gated
      row) and check off this phase

#### Success Criteria

- A changelog-enabled fixture repo shows the row and renders its
  changelog with a version jump list; disabled repos show no row
- Direct URL to a changelog-less repo's `/changelog` renders the
  quiet panel, not an error
- `just ci` semantics green; axe sweep passes with the new view

---

### Phase 2: doc link resolution

The INV-0004 resolver: relative doc links become router links; shared
filename URLs redirect.

#### Tasks

- [ ] Widen `useRepoDocIndex` to return `{ byId, byPath }` from the
      already-fetched queries; extend the fnv1a render-cache
      fingerprint over both key sets; update existing call sites
- [ ] New `src/markdown/relative-links.ts` post-sanitize transform:
      skip absolute/`//`/root/`#`-only hrefs; posix-resolve against
      the base directory; exact normalized-path lookup (3a); on hit
      rewrite href from the map + `dataXref`, preserving `#fragment`;
      on miss leave the anchor untouched
- [ ] Thread the `base` parameter through
      `renderMarkdown`/`useRenderedSource`: reader = doc `path`,
      repo home = `docs_dir`/index.md, changelog = configured file's
      directory
- [ ] Doc-route filename fallback: on `NotFoundError` with a
      `/\.md$/i` `:docId`, redirect (`replace`, hash preserved) when
      exactly one doc's `path` basename matches; ambiguous/none →
      existing panel
- [ ] Unit fixtures: `../type/file.md`, bare `file.md`,
      `./file.md`, fragments, misses, traversal (`../../..`),
      percent-encoded traversal; a demo-org fixture doc with a
      relative References footer rendered end-to-end
- [ ] XSS suite: hostile relative hrefs never resolve to emitted
      links; no `schema.ts` changes
- [ ] Chart bump per OQ-2a: appVersion to the incoming bare semver,
      chart version patch, image-tag unittest assert updated
- [ ] Update CLAUDE.md (resolver, base-path parameter, fallback) and
      check off this phase

#### Success Criteria

- The INV-0004 motivating case works against fixtures: a References
  link `../adr/0013-….md` navigates in-app to `ADR-0013`
- A pasted filename URL redirects to the canonical doc route
- XSS suite green with the new payloads; no sanitize-schema diff

---

### Phase 3: nav pins and docs-type curation

The runtime-config extension (1a JSON format, 2a build-time
fallback) and the OQ-6b pre-styling.

#### Tasks

- [ ] `server/serve.ts`: `resolveNavLinks` — parse the JSON array,
      validate label (`/^[\w .&+-]{1,24}$/`) and href (authReturn
      rule: leading `/`, not `//`, no whitespace/control, ≤ 200),
      cap at 6, drop invalid entries/payloads; inject as
      `__DOCZ_CONFIG__.nav`
- [ ] `server/serve.test.ts`: parse/cap/drop cases + no-breakout
      assertions for hostile `DOCZ_NAV_LINKS`
- [ ] `src/lib/navLinks.ts`: runtime read with the same validation,
      `VITE_NAV_LINKS` fallback, else `[]`; unit tests for
      precedence and hostile input
- [ ] AppShell: pins as `NavLink`s between Repos and `SessionMenu`;
      small-viewport parity; AppShell axe sweep still green
- [ ] e2e: pins render + navigate in the MSW build via
      `VITE_NAV_LINKS` (2a)
- [ ] Helm chart: `config.navLinks` (list of `{label, href}`) →
      `DOCZ_NAV_LINKS` env via `toJson`; `values.schema.json`;
      helm-unittest default + override; appVersion + chart patch
      bump with image-tag assert (OQ-2a); README.md.gotmpl +
      helm-docs
- [ ] `docs` type curation (6b): `CURATED_TYPES` entry,
      `--color-t-docs` token passing `contrast.test.ts`, blurb in
      `docTypes.ts`
- [ ] Update CLAUDE.md + root README (nav pins, env, chart value);
      check off this phase; mark this IMPL Completed and DESIGN-0002
      Implemented

#### Success Criteria

- One image serves any pin set per deployment; invalid config
  degrades to fewer/no pins, never a broken script
- `just bundle-budget` green (eager delta ≲1 KB gz)
- Full `just ci` chain + helm lint/unittest green

## Standing gates (every phase)

- `just ci` semantics: test, test-server, lint, `tsc -b --force`,
  build, format:check, bundle-budget, e2e, gen-api-check
- No credential-shaped strings; no `schema.ts` widening without XSS
  suite extension; hrefs only from API data or validated config
- Check tasks off here as completed; update CLAUDE.md when guidance
  changes; conventional commits per task

## Open Questions

**Reviewed 2026-08-11 — decided: 1a, 2a, 3a, 4b.** Options preserved
below for the record. Consequences are folded into the phase tasks:
the documentation PR merges first as `dont-release` (1a); every phase
PR is labeled `minor` and bumps the chart appVersion alongside (2a);
no docz-api rollout ordering (3a); execution runs as a donald-loop
over this doc (4b) once the documentation PR is merged.

**OQ-1 — Merge the documentation branch before starting Phase 1?**

- **a (recommended).** Yes: push `docs/inv-effect-migration` (four
  INVs + DESIGN-0002 + this IMPL), PR labeled `dont-release`, merge
  when green. Phase branches then cut cleanly from a main that
  already carries the plan, and per-task checkbox updates land on
  main-based branches instead of a long-lived docs branch.
- **b.** Keep the docs local and fold them into the Phase 1 PR —
  one fewer PR, but it buries a large docs diff inside a feature
  review and delays the paper trail landing on main.
- other: —

**OQ-2 — Release labeling per phase PR?**

- **a (recommended).** `minor` on each phase PR — each phase is a
  complete user-visible feature (IMPL-0002 set the minor precedent),
  shipping value as it lands (Phase 2 fixes live broken links).
  Consequence to accept: every phase PR must also bump the chart
  (appVersion to the new bare semver + chart patch) so defaults-only
  installs pull the released image — including Phases 1–2, which
  otherwise don't touch the chart.
- **b.** `dont-release` on Phases 1–2, one `minor` on Phase 3 — a
  single release train and only one chart bump, but the changelog
  page and the link fix sit unreleased until the end.
- **c.** `patch` per phase — smallest version churn, but these are
  features, not fixes, and it muddies the version history.
- other: —

**OQ-3 — Coordination with the production docz-api?**

- **a (recommended).** No ordering constraint. The site degrades
  gracefully by design: the nav row gates on `config_snapshot` (absent
  block → no row) and a missing endpoint just renders the quiet
  panel, so site phases merge and deploy freely while docz-api 1.2.0
  rolls out on its own schedule. Courtesy check of the deployed
  docz-api version at Phase 1 deploy time.
- **b.** Gate the Phase 1 merge on production docz-api serving spec
  1.2.0 — strict ordering, no window where the row can't appear, at
  the cost of coupling this repo's pipeline to the API's rollout.
- other: —

**OQ-4 — Execution mode?**

- **a (recommended).** Direct, phase by phase in-session: implement a
  phase, open its PR, you review/merge per the established "if green,
  merge it" flow, then the next phase starts. Keeps you at the two
  decision points that matter (merge and release).
- **b.** donald-loop autonomous run over this IMPL (the IMPL-0001
  style) with per-phase completion gates — faster wall-clock, less
  per-phase review.
- other: —

## References

- DESIGN-0002 — the approved design this executes
  (`docs/design/0002-nav-pins-changelog-page-and-doc-link-resolution.md`)
- INV-0003 / INV-0004 / INV-0005 — source investigations
- docz-api `d4399ea` — spec 1.2.0 (`getRepoChangelog`)
- IMPL-0002 — prior phase-tracking precedent
