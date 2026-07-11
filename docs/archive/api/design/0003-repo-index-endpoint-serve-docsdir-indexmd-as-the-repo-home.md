---
id: DESIGN-0003
title: "Repo index endpoint: serve docs_dir index.md as the repo home"
status: Draft
author: Donald Gifford
created: 2026-07-10
---
<!-- markdownlint-disable-file MD025 MD041 -->

# DESIGN 0003: Repo index endpoint: serve docs_dir index.md as the repo home

**Status:** Draft
**Author:** Donald Gifford
**Date:** 2026-07-10

<!--toc:start-->
- [Overview](#overview)
- [Goals and Non-Goals](#goals-and-non-goals)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Background](#background)
- [Detailed Design](#detailed-design)
  - [Fetch: one more targeted blob in githubapp](#fetch-one-more-targeted-blob-in-githubapp)
  - [Snapshot and ingest: two new RepoSnapshot fields](#snapshot-and-ingest-two-new-reposnapshot-fields)
  - [Store: two nullable repo columns](#store-two-nullable-repo-columns)
  - [API: GET /api/v1/repos/{owner}/{name}/index](#api-get-apiv1reposownernameindex)
  - [OpenAPI contract: new path, new schema, minor bump](#openapi-contract-new-path-new-schema-minor-bump)
  - [Refresh: already covered by push detection](#refresh-already-covered-by-push-detection)
- [API / Interface Changes](#api--interface-changes)
- [Data Model](#data-model)
- [Testing Strategy](#testing-strategy)
- [Migration / Rollout Plan](#migration--rollout-plan)
- [Open Questions](#open-questions)
  - [1. Response shape for the index endpoint?](#1-response-shape-for-the-index-endpoint)
  - [2. Where is docs_dir resolved for the fetch?](#2-where-is-docs_dir-resolved-for-the-fetch)
  - [3. Semantics when the repo has no index.md?](#3-semantics-when-the-repo-has-no-indexmd)
  - [4. Backfill for repos onboarded before this ships?](#4-backfill-for-repos-onboarded-before-this-ships)
  - [5. Endpoint path naming?](#5-endpoint-path-naming)
  - [6. Include index_sha in the response?](#6-include-index_sha-in-the-response)
- [Follow-ups](#follow-ups)
- [References](#references)
<!--toc:end-->

## Overview

docz-api will fetch, cache, and serve the repo's **`docs_dir/index.md`** — the
exact file docz's mkdocs wiki renders as its landing page — so the docz-site
can replace its generated repo-home fallback with the real rendered document.
The pipeline change is a near-mechanical copy of the existing `CHANGELOG.md`
precedent (one targeted blob fetch → two cached repo columns), plus one new
read endpoint, **`GET /api/v1/repos/{owner}/{name}/index`**, returning the raw
markdown in a JSON envelope (404 when the repo has no `index.md`). Everything
is additive: no existing wire shape changes, and the OpenAPI contract bumps
`1.0.0 → 1.1.0` (minor) so the site can pin against the new surface.

This implements **INV-0003 finding F1** — the highest-priority item in the
docz-site deferred-features disposition.

## Goals and Non-Goals

### Goals

- Serve the repo home document (`docs_dir/index.md`) through the read API so
  the docz-site's `/:owner/:repo` page renders it via the existing reader
  pipeline ("when the API grows an index endpoint, the rendered `index.md`
  slots into the same frame").
- **Match the docz wiki convention exactly** — the served file is the one docz
  itself renders as the wiki landing page (`doczcfg.WikiIndexName` joined to
  the config's `docs_dir`), so the site page and the wiki page can never
  disagree about what "the repo home" is.
- **Strictly additive**: no change to any existing endpoint, DTO, or wire
  shape; the contract test proves it (a leaked field fails
  `additionalProperties: false`).
- Reuse the established precedents end to end: the `CHANGELOG.md` targeted
  fetch + cached repo columns, the `resolveRepo` existence-hiding 404, the
  DESIGN-0002 spec/contract-test/SemVer discipline.
- Keep refresh free: default-branch pushes touching `docs_dir/` already
  trigger a re-ingest; `index.md` lives under `docs_dir/`.

### Non-Goals

- **No HTML rendering** — the endpoint returns raw markdown; rendering is the
  site's reader pipeline (same division as `getDoc`'s `raw_md`).
- **No changelog endpoint** — `repos.changelog_md` stays cached-but-unserved
  until the versions feature (DESIGN-0001 OQ-12) lands; this endpoint's shape
  is deliberately copyable for it (INV-0003).
- **No arbitrary-file serving or ingest** — `index.md` is cached on the repo
  row, not ingested into `documents` (INV-0003 F1 option c is rejected: it
  would break the "documents are docz docs" invariant that list/search rely
  on).
- **No type-dir index/README serving** — the site synthesizes type pages
  client-side (docz-site Decision 8); unchanged.
- **No link-graph work** — that is INV-0003 #2, a separate design.

## Background

- The **docz-site** (its DESIGN-0001) ships v1 with a client-generated repo
  home and a "No index.md configured" fallback, because docz-api serves no
  repo-level page body. The mockup's intended page is the markdown render of
  the repo's `index.md`.
- **docz defines the file**: the wiki landing page is
  `filepath.Join(cfg.DocsDir, config.WikiIndexName)` — `docs_dir/index.md` —
  in docz v1.0.0 (`cmd/wiki.go`; `WikiIndexName = "index.md"` is an exported
  constant in `pkg/doczcore/config`). Serving this file introduces no new
  convention.
- **The pipeline already has the exact precedent.** `githubapp.Client.Fetch`
  targeted-fetches `.docz.yaml` and an optional root `CHANGELOG.md`
  (`classifyTree` → `fetchBlob`), carries them on `ingest.RepoSnapshot`
  (`ChangelogMD []byte` / `ChangelogSHA string`, nil/"" when absent), and
  `store.ReconcileRepo` upserts them onto `repos.changelog_md` /
  `changelog_sha` (nullable TEXT) on every reconcile.
- **`index.md` is invisible to the current pipeline**: `doczdoc.IsDoczFile`
  requires leading digits + hyphen (`0001-….md`), so `index.md` is never
  swept up as a doc blob — there is no collision with document ingest, and
  nothing fetches it today.
- **INV-0003** dispositioned the docz-site's deferred features and recommended
  exactly this design as item #1 (small, fully precedented, additive), with
  the dedicated-endpoint shape chosen over a `RepoDetail` field (payload
  bloat) or a pseudo-document (invariant break).

## Detailed Design

### Fetch: one more targeted blob in githubapp

`githubapp.Client.Fetch` today resolves HEAD, pulls the recursive tree,
classifies entries (`.docz.yaml` sha, root `CHANGELOG.md` sha, doc blobs), and
fetches each. The wrinkle for `index.md`: its path is `docs_dir/index.md`, and
**`docs_dir` is not known until `.docz.yaml` is parsed** — which today happens
in ingest (`loadConfig`), not githubapp.

Resolution (OQ-2a): Fetch already retrieves the config blob **first**. Add a
minimal, fetch-scoped parse of just the `docs_dir` key from those bytes —
a one-field `yaml.Unmarshal` (`struct{ DocsDir string }`), defaulting to
`doczcfg.DefaultConfig().DocsDir` (`"docs"`) when unset — then look up the
exact path `docsDir + "/" + doczcfg.WikiIndexName` in the already-fetched tree
entries and `fetchBlob` it. Sketch:

```go
// after snap.ConfigYAML is fetched:
docsDir := docsDirHint(snap.ConfigYAML) // minimal parse; default "docs"
if sha := findEntry(tree, docsDir+"/"+doczcfg.WikiIndexName); sha != "" {
    if snap.IndexMD, err = c.fetchBlob(ctx, owner, name, sha); err != nil {
        return nil, fmt.Errorf("fetch %s/%s: %w", docsDir, doczcfg.WikiIndexName, err)
    }
    snap.IndexSHA = sha
}
```

Properties:

- **At most one extra blob request** per fetch (zero when the file is absent —
  the tree is already in memory).
- The minimal parse is a _path hint only_; the authoritative config parse +
  validation stays in ingest's `loadConfig` (the HOME-suppressed `doczcfg.Load`
  bridge). A repo whose `.docz.yaml` fails the real validation still fails
  ingest exactly as today — the hint parse cannot mask errors.
- No hardcoded defaults: both the filename (`WikiIndexName`) and the fallback
  docs dir (`DefaultConfig().DocsDir`) come from the pinned docz library, so a
  docz convention change surfaces as a pin bump, guarded by `doczcontract`.
- Absent file ⇒ `IndexMD` nil / `IndexSHA` "" — the `CHANGELOG.md` absence
  contract, verbatim.

### Snapshot and ingest: two new RepoSnapshot fields

`ingest.RepoSnapshot` grows two fields mirroring the changelog pair:

```go
// IndexMD is the raw bytes of docs_dir/index.md, or nil if absent.
IndexMD []byte
// IndexSHA is the git blob sha of docs_dir/index.md, or "" if absent.
IndexSHA string
```

`ingest.Service.Run`'s `RepoInput` mapping adds the corresponding two lines
next to the changelog pair (`IndexMD: string(snap.IndexMD), IndexSHA:
snap.IndexSHA`). Nothing else in ingest changes — `index.md` never enters the
blob/frontmatter path.

### Store: two nullable repo columns

- **Migration** (second goose migration, embedded like the first):

  ```sql
  -- +goose Up
  ALTER TABLE repos
      ADD COLUMN index_md  TEXT,   -- cached raw docs_dir/index.md (NOT parsed)
      ADD COLUMN index_sha TEXT;   -- blob sha of the cached index.md

  -- +goose Down
  ALTER TABLE repos
      DROP COLUMN index_md,
      DROP COLUMN index_sha;
  ```

  Nullable, no backfill — additive and instant; `main()` auto-migrates on
  startup and `-migrate` stays the CI/ops pre-step.

- **`store.RepoInput`** grows `IndexMD` / `IndexSHA string`; `reconcile.go`
  maps them with the existing `textOrNull` helper, exactly beside
  `ChangelogMd`/`ChangelogSha`. The upsert is unconditional per reconcile
  (one row per repo — no content gate needed, same as the changelog).
- **`UpsertRepo`** (`queries/repos.sql`) adds the two columns to the INSERT
  and `DO UPDATE SET`; `sqlc generate` refreshes the typed query. **No new
  read query is needed**: `GetRepoByOwnerName` / `ListRepos` are `SELECT *`,
  so `store.Repo` picks the columns up on regeneration.
- Wire-safety note: `store.Repo` gaining fields does **not** leak to the API —
  the DTOs are hand-mapped and the contract test's
  `additionalProperties: false` would fail if a field leaked.

### API: GET /api/v1/repos/{owner}/{name}/index

New route in `internal/httpapi`, registered inside the same gated `/api/v1`
group (`Handler.Mount`), directly under the repo subtree:

```go
r.Route("/repos/{owner}/{name}", func(r chi.Router) {
    r.Get("/", h.getRepo)
    r.Get("/index", h.getRepoIndex)   // new
    ...
})
```

Handler behavior (`getRepoIndex`):

1. `resolveRepo` — the shared helper: unknown repo **and** repo outside the
   caller's allowed set both 404 (existence hiding), server errors 500. No new
   store method: the resolved `store.Repo` already carries `IndexMd`/`IndexSha`.
2. If `repo.IndexMd` is not valid (no `index.md` at last ingest) → **404**
   with the standard error envelope (`{"error": "index not found"}`) — OQ-3a.
   The site's existing "No index.md configured" fallback maps onto this
   directly.
3. Otherwise → 200 with the index DTO:

```go
type repoIndexDTO struct {
    Repo     string `json:"repo"`      // "owner/name"
    IndexMD  string `json:"index_md"`  // raw markdown, never rendered
    IndexSHA string `json:"index_sha"` // blob sha (client cache key)
}
```

The envelope shape (OQ-1a) keeps the all-JSON `/api/v1` convention, reuses the
error envelope on the 404 path, and gives the response room to grow without a
breaking change. `index_sha` (OQ-6a) mirrors `git_sha` on documents and gives
the site a cheap cache key. Large bodies stay off the list/detail endpoints —
the same altitude rule as `raw_md`-only-on-`getDoc`.

### OpenAPI contract: new path, new schema, minor bump

Per the DESIGN-0002 regime, the wire change ships spec-first:

- **New path** `/api/v1/repos/{owner}/{name}/index` (`get`, tag `repos`,
  `operationId: getRepoIndex`), reusing the shared `Owner`/`Name` parameters
  and `Unauthorized`/`NotFound` responses.
- **New schema** `RepoIndex` — `repo` / `index_md` / `index_sha`, all
  `required`, `additionalProperties: false` (the drift detector).
- **`info.version: 1.0.0 → 1.1.0`** (additive = minor). The docz-site pins
  `>= 1.1.0` for the repo-home feature.
- **Contract test**: `seededStore`'s fixture repo gains an index body so the
  table adds a `getRepoIndex` happy-path case; a repo without an index covers
  the 404 envelope. `vacuum` + `yamlfmt` gates apply unchanged.

### Refresh: already covered by push detection

`webhook.shouldIngest` re-ingests on any default-branch push whose changed
paths include `.docz.yaml` or anything under `docs_dir/` — which includes
`docs_dir/index.md`. **No webhook change.** Creating, editing, or deleting
`index.md` lands on the repo row at the next reconcile (deletion nulls the
columns, because the upsert writes the absent state unconditionally).

## API / Interface Changes

| Surface | Change |
| --- | --- |
| `GET /api/v1/repos/{owner}/{name}/index` | **New** (session-gated, existence-hiding 404s; 404 when no `index.md`) |
| `api/openapi.yaml` | New path + `RepoIndex` schema; `info.version` → `1.1.0` |
| `ingest.RepoSnapshot` | + `IndexMD []byte`, `IndexSHA string` |
| `ingest.RepoFetcher` implementations | `githubapp.Client.Fetch` populates the new fields; the e2e fake follows |
| `store.RepoInput` / `UpsertRepo` | + `IndexMD`/`IndexSHA` → `repos.index_md`/`index_sha` |
| Existing endpoints / DTOs | **Unchanged** (contract test enforces) |

## Data Model

One additive migration (nullable columns, no data movement):

| Column | Type | Meaning |
| --- | --- | --- |
| `repos.index_md` | `TEXT NULL` | cached raw `docs_dir/index.md` at last ingest; NULL = absent |
| `repos.index_sha` | `TEXT NULL` | git blob sha of the cached file; NULL = absent |

The pair is intentionally symmetric with `changelog_md`/`changelog_sha` — the
repo row is the home for "one optional cached file per repo" state.

## Testing Strategy

- **githubapp unit tests** (stub `http.RoundTripper` + `testdata/` fixtures,
  the existing pattern): tree containing `docs/index.md` → fetched into
  `IndexMD`/`IndexSHA`; tree without it → nil/"" and **no extra blob call**;
  non-default `docs_dir` in `.docz.yaml` → the hint parse targets the right
  path; config missing `docs_dir` → default `"docs"` used.
- **ingest unit test**: `RepoInput` carries the snapshot's index pair (beside
  the existing changelog mapping assertion).
- **store integration test** (`//go:build integration`, testcontainers):
  reconcile writes the columns; a reconcile from a snapshot without the file
  nulls them (the delete-index.md path).
- **httpapi unit tests**: 200 envelope for a repo with an index; 404 for a
  repo without one; 404 for an unknown repo and for a repo outside the
  allowed set (existence hiding).
- **OpenAPI contract test**: new `getRepoIndex` table case (schema-validated
  request + response) and the 404 envelope case; spec self-validation and the
  `1.1.0` bump ride the existing harness.
- **e2e** (`internal/e2e`): extend the fake `RepoFetcher` snapshot with an
  index body and assert the endpoint serves it after onboarding — proving the
  fetch→reconcile→serve slice.

## Migration / Rollout Plan

1. Land the migration + pipeline + endpoint + spec in one PR (a single
   vertical slice; each layer is independently testable but shipping them
   together avoids a specced-but-unserved window).
2. Startup auto-migrate applies the columns; `-migrate` remains the ops
   pre-step. Down migration drops the columns cleanly.
3. **Already-onboarded repos serve 404 until their next re-ingest** (the
   columns are NULL until a reconcile runs). Per OQ-4a this is left to natural
   refresh — the next docs push re-ingests, and a manual
   `-onboard owner/name@installation` forces one immediately. At homelab
   scale (a handful of repos) no backfill machinery is warranted; the site's
   fallback renders exactly as today until then.
4. The docz-site swaps its generated fallback for the rendered `index.md`
   when it sees spec `1.1.0`; on 404 it keeps the current fallback — so the
   rollout has no coordination deadline in either direction.

## Open Questions

**Resolved 2026-07-10: all option `a`.** Decisions are recorded inline
(**→ Decision**) and were already reflected in the Detailed Design above (which
cites the OQ-`n`a choices at each decision point); the original options are
kept for context. Each question is numbered; option `a` was the
recommendation, later letters alternatives, **Other** free-form.

### 1. Response shape for the index endpoint?

- **a (recommended):** **JSON envelope** `{"repo", "index_md", "index_sha"}`
  (`RepoIndex` schema, `additionalProperties: false`). Consistent with every
  other `/api/v1` response, reuses the JSON error envelope on 404, is directly
  contract-testable, and leaves room to grow (e.g. a future `updated_at`)
  without a breaking change.
- **b:** **Raw markdown body** (`Content-Type: text/markdown`). Simplest
  possible client read, but it breaks the all-JSON convention, needs a
  different error-envelope story, special-cases the contract test, and cannot
  carry the sha.
- **c:** **Content negotiation** — JSON by default, `text/markdown` via
  `Accept`. Serves both, but doubles the spec/response surface and the test
  matrix for no consumer that asked for it.
- Other.

**→ Decision: 1a.** JSON envelope `{"repo", "index_md", "index_sha"}` as the
`RepoIndex` schema, `additionalProperties: false`.

### 2. Where is docs_dir resolved for the fetch?

`docs_dir` comes from `.docz.yaml`, which githubapp fetches but ingest parses —
and the index blob's path needs it at fetch time.

- **a (recommended):** **Minimal path-hint parse in githubapp** — unmarshal
  just `docs_dir` from the already-fetched config bytes, defaulting to
  `doczcfg.DefaultConfig().DocsDir`; the authoritative parse/validation stays
  in ingest's `loadConfig`. One extra blob call max; no interface changes; no
  hardcoded conventions (filename + default both come from the pinned docz
  library).
- **b:** **Collect candidates, decide in ingest** — classifyTree gathers every
  `index.md` tree entry; ingest picks `docs_dir/index.md` after the real
  config parse. But blobs are fetched inside `Fetch`, so this either
  over-fetches every `index.md` in the repo (wiki setups can have per-section
  ones) or requires a second fetch round-trip after ingest decides — both
  worse than the hint parse.
- **c:** **Refactor the fetch boundary** — split `Fetch` into
  tree/config-first and blobs-second phases so ingest owns all path decisions.
  Cleanest layering in the abstract, but a real interface refactor
  (`RepoFetcher` is one method) for a one-field hint; not worth it at this
  scale.
- Other.

**→ Decision: 2a.** Minimal path-hint parse of `docs_dir` in githubapp
(default `doczcfg.DefaultConfig().DocsDir`); authoritative parse/validation
stays in ingest's `loadConfig`.

### 3. Semantics when the repo has no index.md?

- **a (recommended):** **404** with the standard error envelope
  (`{"error": "index not found"}`), reusing the spec's `NotFound` response.
  "This repo has no home document" is a missing resource; the site's existing
  fallback path is its 404 handler; and absent stays distinguishable from an
  empty-but-present file (which returns 200 + `""`).
- **b:** **200 with `index_md: ""`** (or `null`). Saves the site an error
  branch but conflates absent with empty, forces emptiness-testing on every
  consumer, and (if `null`) breaks the "nullable columns are empty strings,
  never null" serialization invariant.
- **c:** **204 No Content.** Signals absence without an error, but a bodiless
  success is unlike every other endpoint and awkward to schema-contract.
- Other.

**→ Decision: 3a.** 404 with the standard error envelope
(`{"error": "index not found"}`), reusing the spec's `NotFound` response;
absent stays distinguishable from empty-but-present (200 + `""`).

### 4. Backfill for repos onboarded before this ships?

Their `index_md` is NULL until a reconcile runs.

- **a (recommended):** **Natural refresh** — the next default-branch docs push
  re-ingests, and a manual `-onboard owner/name@id` forces one per repo.
  Document it in the rollout notes. At homelab scale this is minutes of
  one-time toil, and the site's 404 fallback covers the gap gracefully.
- **b:** **One-shot re-ingest-all trigger** — a `-reingest-all` flag (or admin
  endpoint) that enqueues an ingest per stored repo. A generic ops tool,
  useful beyond this feature, but new surface + auth questions for a one-time
  need.
- **c:** **Auto-enqueue on startup** when the binary detects repos with NULL
  `index_sha` post-migration. Zero toil, but hidden magic on deploy and a
  thundering herd against the GitHub API if the fleet ever grows.
- Other.

**→ Decision: 4a.** Natural refresh — the next docs push re-ingests, or a
manual `-onboard owner/name@id` forces one; documented in the rollout notes.
No backfill machinery.

### 5. Endpoint path naming?

- **a (recommended):** **`/repos/{owner}/{name}/index`** — a resource noun
  beside `/types` and `/docs`, matching the surface's naming style; the
  response self-describes as markdown.
- **b:** **`/repos/{owner}/{name}/index.md`** — self-documenting file flavor,
  but no other route carries an extension and it reads as a raw-file promise
  (the response is a JSON envelope per OQ-1a).
- **c:** **`/repos/{owner}/{name}/home`** — matches the site's page name, but
  invents a docz-api concept; docz's own name for this file is the wiki
  _index_.
- Other.

**→ Decision: 5a.** `/repos/{owner}/{name}/index` — a resource noun beside
`/types` and `/docs`.

### 6. Include index_sha in the response?

- **a (recommended):** **Yes.** It mirrors `git_sha` on the document DTO,
  costs nothing (already stored), and gives the site a stable cache key
  (skip re-render when unchanged) without inventing ETag plumbing.
- **b:** **No — trim to `{repo, index_md}`.** Strictly YAGNI-minimal; adding
  the sha later is a compatible minor bump, but dropping a field later would
  be a major one, and the sha's cost is zero now.
- Other.

**→ Decision: 6a.** Include `index_sha` — mirrors `git_sha` on documents and
serves as the site's cache key.

## Follow-ups

- **IMPL doc** for the build once the open questions are resolved (single
  vertical slice: migration → fetch → reconcile → endpoint → spec `1.1.0`).
- **Changelog endpoint** (versions feature, DESIGN-0001 OQ-12): copy this
  endpoint's resolved shape verbatim for `repos.changelog_md` when that
  feature lands.
- **Link-graph investigation** (INV-0003 #2): the next deferred-feature
  keystone; independent of this design.
- **docz-site**: swap the generated repo home for the rendered `index.md` at
  spec `1.1.0`; keep the fallback for 404.

## References

- [INV-0003](../investigation/0003-docz-site-deferred-features-and-the-docz-api-surface-to-unblock.md)
  — the disposition that prioritized this design (finding F1, option a)
- [DESIGN-0001](0001-docz-api-cross-repo-docz-registry-and-ingestion-service.md)
  — the service design (ingest pipeline, repos schema, changelog precedent,
  deferred versions feature OQ-12)
- [DESIGN-0002](0002-openapi-contract-for-docz-api-and-the-docz-site.md)
  — the wire-contract regime this change ships under (spec, contract test,
  SemVer)
- docz-site DESIGN-0001 — "Repo home (`/:owner/:repo`)": the consumer slot
  this endpoint fills
- docz v1.0.0 — `config.WikiIndexName` (`"index.md"`),
  `DefaultConfig().DocsDir` (`"docs"`), wiki landing page join in
  `cmd/wiki.go`; `document.IsDoczFile` (digits-hyphen pattern; excludes
  `index.md`)
- `internal/githubapp/client.go` — `Fetch`/`classifyTree`/`fetchBlob` (the
  `CHANGELOG.md` targeted-fetch precedent)
- `internal/ingest/fetcher.go` — `RepoSnapshot` (the changelog field pair)
- `internal/store/queries/repos.sql` + `internal/store/reconcile.go` — the
  upsert the new columns join
- `internal/httpapi/handler.go` — `Mount`, `resolveRepo` (existence hiding)
- `internal/webhook/events.go` — `shouldIngest` (push refresh already covers
  `docs_dir/index.md`)
