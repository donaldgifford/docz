# CLAUDE.md: `internal/httpapi`

Loaded when a session works under this directory. The repository's map is the root `CLAUDE.md`.

## Repo index endpoint (DESIGN-0003 / IMPL-0003)

The docz-site repo home: `docs_dir/index.md` (docz's wiki landing page,
`doczcfg.WikiIndexName`) is fetched at ingest, cached on the repo row, and
served at **`GET /api/v1/repos/{owner}/{name}/index`** as
`{repo, index_md, index_sha}` (spec `1.1.0`, additive). Built per
`docs/design/0003-*.md` (all six design OQs = `a`) and `docs/impl/0003-*.md`
(all five impl OQs = `a`) — **all four phases COMPLETE ✅**.

- **Persistence:** `repos.index_md` / `repos.index_sha` (nullable TEXT,
  migration `20260710000000_add_repo_index.sql`, mirrors the
  `changelog_md`/`changelog_sha` precedent); `UpsertRepo` writes both;
  `RepoInput.IndexMD/IndexSHA` map through `textOrNull`.
  **GOTCHA — presence keys off `index_sha`:** `textOrNull("")` is NULL, so an
  empty-but-present `index.md` stores a NULL body with a **valid sha** — the
  handler gates on `IndexSha.Valid` and `nullText` yields the `""` body, which
  is exactly DESIGN OQ-3a's "empty file ⇒ 200 + empty string; absent ⇒ 404".
- **Fetch (githubapp):** `docsDirHint(configYAML)` does a fetch-scoped
  one-field `yaml.Unmarshal` of `docs_dir` (trailing-`/` trimmed; default
  `doczcfg.DefaultConfig().DocsDir`; the authoritative parse stays in ingest's
  `loadConfig`, so a malformed config still fails there). `findBlobSHA` looks
  up `docsDirHint(...)+"/"+doczcfg.WikiIndexName` in the already-listed
  recursive tree (exact path, blob type) — **at most one extra blob request,
  zero when absent**. `gopkg.in/yaml.v3` is a **direct** require (same dialect
  docz uses — no drift; promoted via the require block, never a bare tidy).
  `IsDoczFile` requires leading digits so `index.md` never collides with doc
  ingest; the webhook `shouldIngest` already re-ingests on `docs_dir/` pushes,
  so index refresh needed **no webhook change**.
- **Serve (httpapi):** `getRepoIndex` = `resolveRepo` (existence hiding) →
  `IndexSha.Valid` check → 404 `{"error":"index not found"}` or 200
  `repoIndexDTO`. Contract-tested happy + 404 via the second bare fixture repo
  `acme/bare` (OQ-2a; the fixture growth shifted the `list repos` count and
  the search allowed-set assertions).
- **Rollout:** natural refresh only (DESIGN OQ-4a) — pre-existing repos 404
  until their next ingest; note in `deploy/README.md`. No size cap on the
  cached body (capless changelog precedent, OQ-4a).
- **Proof:** `TestE2ERepoIndexServeAndRemoval` (real Postgres) covers ingest →
  serve → removal-at-HEAD → 404; store round-trip + migration up/down live in
  the store integration tests.

## Changelog endpoint (INV-0005 / IMPL-0005)

The repo changelog became **opt-in and served**: `.docz.yaml`'s `changelog:`
block (docz **v1.1.0**, upstream DESIGN-0010) drives the fetch, and the cached
raw markdown is served at **`GET /api/v1/repos/{owner}/{name}/changelog`**
(spec `1.2.0`, additive). Built per `docs/investigation/0005-*.md` (Concluded,
all OQs `a`) and `docs/impl/0005-*.md` — **all five phases COMPLETE ✅**. It
deliberately mirrors the `index.md` slice (DESIGN-0003 / IMPL-0003); only the
deltas are noted here.

- **docz pin is `v1.1.0`** and `internal/doczcontract` gained **clause R6**
  (`internal/doczcontract/changelog_test.go` + the frozen
  `testdata/changelog_fleet.md`): `ChangelogConfig` defaults/partial-merge/
  `./`-normalization, **enabled-only validation** (a dormant block with a bad
  path must NOT fail `Validate`), unknown-sibling-key tolerance, and
  `ParseChangelog`'s parse shape + `ErrNoVersions`. `ParseChangelog` has **no
  runtime caller** — it is pinned now so feature 2 (per-doc backlinks) starts
  on a frozen surface.
- **Opt-in is desired state, not a cache.** A disabled or absent block means
  ingest maps an empty `ChangelogFile`/`MD`/`SHA`, so the reconcile **nulls all
  three columns** and the endpoint 404s. Pre-config-era cached changelogs are
  cleared on the next ingest of a repo that never opts in (nothing served them,
  so nothing regresses).
- **`repos.changelog_file`** (migration `20260803000000_add_repo_changelog_file`)
  stores the resolved path. **Why a column and not a re-parse:** the changelog
  lives **outside `docs_dir`**, so `shouldIngest`'s prefix check would never
  match it and a release's changelog-sync push (which touches nothing else)
  would leave the served copy a release behind. `handlePush` already holds the
  full repo row, so the webhook match is one exact-path comparison.
  Deliberately one plain column — the shape is expected to generalize to other
  API-consumed files later, so no bespoke abstraction was introduced.
- **`changelogHint`** (`internal/githubapp`) is the `docsDirHint` twin:
  fetch-scoped one-field unmarshal, docz defaults on malformed yaml, `./`
  trimmed. `classifyTree` **no longer recognizes** `CHANGELOG.md` at all —
  the path is resolved by `findBlobSHA` like `index.md`, so a repo that never
  opts in costs **zero** extra requests. Subpaths
  (`charts/<name>/CHANGELOG.md`) work by construction.
- **`ingest.changelogFile(cfg)`** maps the **authoritative** post-`Load` value
  (normalized + validated), never the hint. An invalid `changelog.file` on an
  enabled block fails `Validate` → fails the whole ingest, like any other
  malformed `.docz.yaml` (IMPL OQ-2a — one error path, no partial-ingest mode).
- **Serve:** `getRepoChangelog` gates on `ChangelogSha.Valid`, so an
  empty-but-present file is 200 with `""` and absence is 404 — the same
  `textOrNull` presence-keys-off-the-sha gotcha as the index pair.
- **Proof:** `TestE2ERepoChangelogServeAndDisable` (real Postgres) covers
  ingest → serve → disable-at-HEAD → 404; `TestReconcileRepoChangelogTriple`
  round-trips the columns; `TestFetchRepoChangelog` proves the no-fetch cases
  by **withholding the blob** (the stub 404s on an unfetched sha).

## Pages endpoints + search source facet (DESIGN-0004 / IMPL-0007)

> **Amended by IMPL-0009 (issue #28):** enabled **type dirs publish
> nothing** — rule 4's type-dir `README.md` carve-out is gone. docz-site
> synthesizes its own type page at `/:owner/:repo/:type` from the live
> document list, so publishing the docz-generated index table duplicated
> that surface at `/pages/<dir>` (repo nav, search hit, stale body). The
> README skips **silently** — `docz update` writes it there on every run,
> so a Warn would fire on correct configuration. No schema or spec change;
> reconcile's desired-state delete retires existing rows on each repo's
> next ingest. **Adjacent noise, since fixed:** `buildDocuments` matches by
> type dir rather than `IsDoczFile`, so an enabled `api:` block (which
> widens the fetch to every `.md` under `docs_dir`) sent that same README
> through `ParseFrontmatter` and logged `skipping doc without frontmatter`
> on every ingest. Its `ErrNoFrontmatter` warn is now gated on
> `doczdoc.IsDoczFile(path.Base(...))`: an unparseable **document** is a
> real mistake worth surfacing, while a README or page candidate is not —
> and it kills the double-report, since `buildPages` already names genuine
> strays. **Logging only** — which blobs become documents is unchanged, so
> a non-convention file carrying valid frontmatter still ingests exactly
> as before.

docz v1.2.0's `api:` block publishes a repo's **non-docz markdown** as pages:
fetched at ingest, reconciled into `repo_pages`, served at
`GET /api/v1/repos/{owner}/{name}/pages[/{path}]` (spec `1.3.0`), and
searchable under a `source` facet (spec `1.4.0`). Built per
`docs/design/0004-*.md` (grounded by INV-0008) and `docs/impl/0007-*.md`.
Per IMPL OQ-1a it shipped as **two PRs**: PR-1 = Phases 1–5 (pages surface),
PR-2 = Phases 6–7 (search facet + close-out). Deltas from the
index/changelog precedents:

- **Contract clause R10** (`internal/doczcontract/api_test.go`):
  `Config.API{Enabled, LandingPage, Exclude, AdditionalDocs}`,
  `ErrInvalidAPIPath`, `APILandingFileName`, normalization-at-Load (landing
  backfill `docs_dir/index.md` **only when enabled**; dormant blocks are
  never validated or backfilled), and `docparse.Title` (first H1, ATX +
  setext, frontmatter skipped — frontmatter `title:` deliberately unread).
  Import alias **`doczparse`** joins `doczcfg`/`doczdoc`.
- **Dormancy is byte-for-byte.** A repo without an enabled block fetches
  exactly what v1.1.0 fetched (proven by withheld-blob stubs: the test stub
  404s any blob request the dormant path shouldn't make). `classifyTree`
  widens to every `.md` under `docs_dir` **only when** `apiHint` reports
  enabled; `additional_docs` fetches are per-entry, skipped for non-`.md`
  entries without a request.
- **`ingest.buildPages`** is the six-rule classifier (DESIGN-0004): landing
  skip → over-fetch guard → templates/`api.exclude` pruning → type-dir
  discrimination (**enabled type dirs publish nothing** per IMPL-0009 —
  `IsDoczFile` matches and the dir's own `README.md` stay silent; strays warn
  + skip) → README-wins-over-index directory
  precedence (lone `index.md` serves the directory; `docs_dir` root never
  forms a directory page) → repo-relative `additional_docs`, classified
  second so a cross-namespace collision deterministically goes to the
  docs_dir page (design OQ-1a). **`validPublishedPath` guards the single
  writer**: git-tree paths (dot segments, control bytes, backslashes) never
  become store keys; the serve layer re-applies the same rules to decoded URL
  paths (`validPagePath`) and 404s — indistinguishable from a miss.
- **Pages are desired state.** `reconcileRepoPages` mirrors the documents
  reconcile (content-hash gate, delete-absent) keyed on the published path;
  a dormant block maps `Pages: nil` + empty api fields, so the reconcile
  wipes every row and nulls `repos.api_landing_page`/`api_additional_docs`
  (migration `20260828000000_add_repo_pages`). The nullable JSONB column
  needed its **own sqlc override entry** (`nullable: true` →
  `json.RawMessage`).
- **Webhook:** `shouldIngest`'s changelog exact-path check generalized to a
  `watched []string` built by `watchedFiles(repo)` (changelog + api landing
  page + decoded `api_additional_docs`; NULL columns contribute nothing) —
  a push touching only a root additional doc re-ingests.
- **Search (PR-2):** `IndexDoc`/`SearchHit` carry `Source` (`doc`/`page`) +
  `Path` (repo-relative on docs, published on pages); `source` is filterable
  + faceted. Page PK is `<repo_id>_p_<hex(sha256(published_path))[:16]>` —
  published paths carry chars Meilisearch ids reject, so the path is hashed;
  the `p` marker keeps it out of the doc-id namespace. `syncIndex` folds page
  deletes/upserts into its one delete + one index call via
  `GetRepoPagesByPaths`; the repo-id purge covers pages by construction.
- **Serve:** `GET .../pages` returns an **empty list, not 404**, for a repo
  without the block (the repo exists; its page set is empty); `GET
  .../pages/*` is a chi wildcard + `url.PathUnescape`, so both the literal
  and percent-encoded spellings resolve. Exact-byte lookup, no case folding.
- **Proof:** `TestE2ERepoPagesServeAndDisable` (real Postgres + Meilisearch)
  covers onboard → list/serve (directory page, file page, additional doc) →
  search with `source: page` → disable-at-HEAD → empty list, 404s, index
  purged. This repo **dogfoods** the block (`.docz.yaml`: `api.enabled` +
  `additional_docs: [DEVELOPMENT.md]`).
