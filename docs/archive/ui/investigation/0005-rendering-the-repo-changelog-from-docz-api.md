---
id: INV-0005
title: "Rendering the repo changelog from docz-api"
status: Concluded
author: Donald Gifford
created: 2026-08-10
---
<!-- markdownlint-disable-file MD025 MD041 -->

# INV 0005: Rendering the repo changelog from docz-api

**Status:** Concluded
**Author:** Donald Gifford
**Date:** 2026-08-10

<!--toc:start-->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
  - [Observation 1: the API contract, as shipped](#observation-1-the-api-contract-as-shipped)
  - [Observation 2: the site already owns the pattern — this is repo home, again](#observation-2-the-site-already-owns-the-pattern--this-is-repo-home-again)
  - [Observation 3: the one real design fork is the existence signal](#observation-3-the-one-real-design-fork-is-the-existence-signal)
  - [Observation 4: content and interplay notes](#observation-4-content-and-interplay-notes)
- [Open questions](#open-questions)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
- [References](#references)
<!--toc:end-->

## Question

docz-api now serves an opt-in repository changelog (its INV-0005/
IMPL-0005, merged as `d4399ea`, spec **1.2.0**): a repo whose
`.docz.yaml` enables the `changelog:` block gets its configured file
(root `CHANGELOG.md` by default, or a subpath like
`charts/<name>/CHANGELOG.md`) fetched at ingest and served via
`getRepoChangelog`. What does docz-site need to show it — with the
proposed UX of a row that "fits under the Home / index.md section" of
the repo nav?

## Hypothesis

Going in: this should be a small, pattern-following addition — the
repo home (`getRepoIndex` → rendered markdown inside `RepoFrame`)
already established the "cached repo file as a page" shape, so the
expectation is: re-vendor the spec, add one lazy route, one nav row,
fixtures, and tests. The open design work should concentrate in two
places: how the nav knows a changelog exists without wasted requests,
and presentation details (header, rail, hint text).

## Context

**Triggered by:** docz-api `d4399ea` — "feat: serve the repo
changelog (INV-0005/IMPL-0005)" — plus the nav mock idea: a
`Changelog` row directly under `Home · index.md` in the RepoNav
identity section, shown only for repos that serve one.

Current site state:

- Vendored spec is **1.1.0** — `getRepoChangelog` does not exist in
  `src/api/__generated__/` yet; re-vendor + `just gen-api` is step
  zero (the spec-drift workflow will flag 1.2.0 independently).
- `src/components/repo-nav.tsx` renders the Home row
  (`Home · index.md`, `repo-nav.tsx:164`) followed by the `doc types`
  section — the proposed row slots between them.
- `src/routes/repo-home.tsx` is the pattern to copy: `getRepoIndex` →
  `useRenderedSource` → `RepoFrame` with a `TocList` rail; 404 →
  generated fallback; `SessionRequired`/error panels shared from
  `query-states.tsx`.
- `RepoFrame`'s `rail` prop is optional, and repo routes are lazy
  chunks — a changelog route adds zero eager-bundle cost.

## Approach

Desk investigation, no code:

1. Read the shipped upstream contract (spec 1.2.0 endpoint semantics,
   `RepoChangelog` schema, ingest/webhook behavior, config block).
2. Diff against the vendored 1.1.0 spec and the generated client.
3. Map the repo-home pattern onto the changelog page; enumerate the
   full deliverable list.
4. Isolate the genuine decision forks and write them up as lettered
   open questions for review.

## Environment

| Component     | Version / Value                                    |
| ------------- | -------------------------------------------------- |
| docz-api      | main @ d4399ea, spec 1.2.0 (`getRepoChangelog`)    |
| docz-site     | main @ 3304248 (v0.1.2); vendored spec 1.1.0       |
| Config gate   | `.docz.yaml` `changelog: { enabled, file }`        |
| Nav reference | `src/components/repo-nav.tsx:164` (Home row)       |

## Findings

### Observation 1: the API contract, as shipped

`GET /api/v1/repos/{owner}/{name}/changelog` → `RepoChangelog`:

```yaml
RepoChangelog:
  required: [repo, changelog_md, changelog_sha]
  # changelog_md: raw markdown at the last ingest; "" for an
  #   empty-but-present file (absence is a 404, never an empty body)
  # changelog_sha: git blob SHA; usable as a cache key
```

Semantics that matter to the site:

- **Opt-in.** No `changelog:` block (or `enabled: false`) → 404. The
  configured file absent at HEAD → 404. Empty-but-present → 200 with
  `changelog_md: ""` — so the page needs a real empty state, and 404
  is *never* an error condition worth an error panel; it means "this
  repo doesn't serve one."
- **The file can live outside `docs_dir`** (root `CHANGELOG.md`, or a
  subpath like `charts/<name>/CHANGELOG.md`). docz-api's webhook
  matches the configured path explicitly so a release's
  changelog-sync push re-ingests even when nothing under `docs_dir`
  changed — the served changelog does not lag its own release.
- **Raw markdown only.** The docz library has a changelog parser
  (`ParseChangelog`) but the API exposes no parsed form — rendering
  is entirely the site's job, same as `index_md`.
- `RepoDetail` gained **no typed changelog field** — but its
  `config_snapshot` (the repo's `.docz.yaml` at HEAD as JSON,
  `additionalProperties: true`) necessarily contains the
  `changelog:` block that gates the whole feature (see Obs 3).

### Observation 2: the site already owns the pattern — this is repo home, again

Every piece of the changelog page has a shipped precedent:

| Need                  | Precedent                                                        |
| --------------------- | ---------------------------------------------------------------- |
| Fetch cached repo file| `getRepoIndex` query in `repo-home.tsx`                          |
| Render raw markdown   | `useRenderedSource` (h1 kept or stripped by wrapper choice)      |
| Page frame + crumbs   | `RepoFrame` (breadcrumb `home · changelog`)                      |
| Right-rail ToC        | repo home already passes `TocList` into `rail`                   |
| 401 / error states    | shared panels in `query-states.tsx`                              |
| Render memoization    | per `(doc_id, content_hash)` in the reader → per `(repo, changelog_sha)` here |
| Hover prefetch        | `usePrefetchDoc` convention for doc links → same for the nav row |
| Lazy chunk            | every route module (`lazy:` import in `router.tsx`)              |

Deliverable list for the feature PR, in order: re-vendor spec 1.2.0 +
`just gen-api`; MSW fixtures (demo-org repo with a realistic
git-cliff changelog, one repo with `changelog_md: ""`, faker
fallthrough for the rest); route module
`/:owner/:repo/changelog` (a static segment — React Router ranks it
above `/:owner/:repo/:type`, see OQ-2); RepoNav row under Home gated
on the existence signal (OQ-1); breadcrumbs; empty state for `""`;
axe-sweep entry for the new view; unit tests for row
visibility/404/empty/render. No new API asks under the recommended
open-question answers.

### Observation 3: the one real design fork is the existence signal

The nav row must show only for repos that serve a changelog, and
RepoNav renders on *every* repo-scoped page. Three ways to know:

1. **Read `config_snapshot.changelog.enabled`** from the getRepo
   response RepoNav's data already includes — zero extra requests.
   The snapshot is untyped JSON (`additionalProperties: true`), so
   the read is defensive (`enabled === true`, `file` as string for
   the hint). Edge: config enabled but file absent at HEAD → the row
   shows and the page renders the "no changelog at HEAD" state — an
   honest signal to the repo owner who opted in, not a bug.
2. **Probe `getRepoChangelog` per repo visit** — precise (row hides
   on 404) and doubles as a prefetch, but downloads the full
   changelog body on every repo page for every visitor, clicked or
   not.
3. **Upstream ask: typed `has_changelog` / `changelog_file` on
   `RepoDetail`** — cleanest contract, but a docz-api release +
   re-vendor for something the config snapshot already answers.

This is OQ-1; the rest of the forks are presentation-level.

### Observation 4: content and interplay notes

- **Changelog markdown renders clean through the existing pipeline.**
  keep-a-changelog / git-cliff shape is h2 version sections, list
  items, absolute compare/PR links, and reference-style link
  definitions — all plain CommonMark/GFM. The ToC collector turns the
  h2 versions into a jump list for free (OQ-4).
- **Xrefs apply.** Changelog prose that mentions doc ids ("see
  INV-0002") linkifies if the render passes the repo's xref resolver
  — worth doing since the resolver data is already on hand for
  RepoNav's pages.
- **INV-0004 interplay.** If the relative-link resolver lands,
  the changelog's base path for resolution is the *configured file's
  directory* (repo root by default — not `docs_dir`). INV-0004's
  design already requires a per-source base path, so this is a
  parameter, not a change.
- **Search:** the changelog is repo metadata, not a document — it is
  not in Meilisearch and the palette will not find it. That is
  consistent with `index.md` today and out of scope here.

## Open questions

**Reviewed 2026-08-10: all five resolved to option (a)** — the
recommendation in each. Options preserved below for the record.

**OQ-1 — How does the nav know a repo serves a changelog?**

- **a (recommended).** Read `config_snapshot.changelog.enabled`
  (defensively) from the getRepo data RepoNav already has: zero extra
  requests, no upstream work. Accept the enabled-but-absent edge
  rendering an honest "no changelog at HEAD" page state.
- **b.** Probe `getRepoChangelog` on repo pages (limit nothing —
  full body): row visibility is exact and the click is pre-warmed,
  at the cost of fetching the changelog for every visitor.
- **c.** Ask docz-api for a typed `has_changelog`/`changelog_file`
  field on `RepoDetail` first; gate the row on it (a: as the interim
  until it ships).
- other: —

**OQ-2 — Route shape?**

- **a (recommended).** `/:owner/:repo/changelog` — static segment,
  ranked above `/:owner/:repo/:type` by the router. Caveat to accept:
  a custom doc type literally *named* `changelog` becomes unreachable
  by its name URL (still reachable via id_prefix/alias, e.g.
  `/repo/CL`); vanishingly unlikely and worth the clean URL.
- **b.** A reserved-prefix path like `/:owner/:repo/-/changelog`
  (GitLab-style): collision-proof, uglier, and introduces a URL
  convention no other page uses.
- other: —

**OQ-3 — Page header: keep the file's own h1, or synthesize?**

- **a (recommended).** Keep the changelog's `# Changelog` h1, exactly
  like repo home keeps index.md's h1 (`useRenderedSource` with h1
  kept; the `.doc-prose` h1 style already exists for this). File-as-
  page surfaces render their own title; zero special-casing.
- **b.** Strip the h1 and synthesize a header like the type pages
  (label + file path + blurb): more uniform chrome, but a second
  h1-stripping wrapper and bespoke header code for one page.
- other: —

**OQ-4 — Right rail?**

- **a (recommended).** `TocList` of the version headings — a
  jump-to-version list, free from the existing ToC collector, and
  the rail repo home already uses.
- **b.** Empty rail (RepoFrame's two-column collapse look).
- other: —

**OQ-5 — Nav row hint text (the right-aligned filename)?**

- **a (recommended).** Basename of the configured file
  (`CHANGELOG.md`), with the full subpath as the `title` tooltip when
  it differs (`charts/<name>/CHANGELOG.md`) — matches the
  `index.md` hint's width discipline in a 250px rail.
- **b.** Always the full configured path: more precise, likely to
  truncate for chart-style subpaths.
- **c.** No hint — just `Changelog`.
- other: —

## Conclusion

**Answer:** this is a small, pattern-following addition — the
hypothesis held. The API side is done and shipped (spec 1.2.0); the
site side is one lazy route + one gated nav row + a spec re-vendor,
with every rendering/state/test ingredient already precedented by
repo home. Under the decided answers — all (a): config-snapshot
existence signal, `/changelog` route, h1-kept rendering, ToC rail,
basename hint — it requires **no new docz-api asks** and no
eager-bundle cost. The only genuine design fork was the existence
signal (OQ-1); the rest was presentation preference.

## Recommendation

1. Open questions reviewed 2026-08-10 — all five resolved to (a);
   this INV is Concluded.
2. Implement as one feature PR in the deliverable order of Obs 2
   (re-vendor → fixtures → route → nav row → states → tests), per
   the decided answers.
3. Pass the repo xref resolver into the changelog render so doc-id
   mentions linkify (Obs 4).
4. No upstream ask needed under OQ-1(a); note a typed
   `has_changelog` field alongside DESIGN-0001's additive asks only
   if the config-snapshot read ever proves brittle.
5. When INV-0004's relative-link resolver is built, include the
   changelog page with base path = configured file's directory.

## References

- docz-api `d4399ea` — "feat: serve the repo changelog
  (INV-0005/IMPL-0005) + per-provider chart login wiring (#14)";
  spec 1.2.0 `getRepoChangelog` + `RepoChangelog`
  (`api/openapi.yaml:91,541` upstream)
- docz-api `internal/ingest/service.go` (`changelogFile`, config
  gate), `internal/webhook/events.go` (`shouldIngest` matching the
  changelog path outside `docs_dir`)
- `src/components/repo-nav.tsx:164` — the Home row the new row
  follows
- `src/routes/repo-home.tsx` — the pattern being copied
  (`getRepoIndex`, `useRenderedSource`, `TocList` rail, 404
  fallback)
- `src/components/repo-frame.tsx` — optional `rail`, breadcrumbs
- INV-0004 — relative-link resolver; per-source base path covers the
  changelog
- Nav mock: CleanShot 2026-08-10 (Changelog row under
  `Home · index.md`)
