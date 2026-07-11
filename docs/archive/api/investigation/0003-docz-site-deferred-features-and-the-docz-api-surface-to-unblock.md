---
id: INV-0003
title: "docz-site deferred features and the docz-api surface to unblock them"
status: Open
author: Donald Gifford
created: 2026-07-10
---
<!-- markdownlint-disable-file MD025 MD041 -->

# INV 0003: docz-site deferred features and the docz-api surface to unblock them

**Status:** Open
**Author:** Donald Gifford
**Date:** 2026-07-10

<!--toc:start-->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
  - [The deferred set clusters into five root causes](#the-deferred-set-clusters-into-five-root-causes)
  - [F1: Repo home from index.md — the priority, and it is small](#f1-repo-home-from-indexmd--the-priority-and-it-is-small)
  - [F2: The cross-doc link graph is the keystone dependency](#f2-the-cross-doc-link-graph-is-the-keystone-dependency)
  - [F3: Lifecycle rail — commit dates and links](#f3-lifecycle-rail--commit-dates-and-links)
  - [F4: Labels are blocked upstream in docz itself](#f4-labels-are-blocked-upstream-in-docz-itself)
  - [F5: Not docz-api work — pdf, raw files, MCP](#f5-not-docz-api-work--pdf-raw-files-mcp)
  - [Every addition rides the OpenAPI contract discipline](#every-addition-rides-the-openapi-contract-discipline)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
- [References](#references)
<!--toc:end-->

## Question

The docz-site's DESIGN-0001 mockup coverage map defers a set of elements (the
◐ and ⏸ rows) because docz-api lacks the surface to power them: the repo home
rendered from `index.md`, cross-doc references/backlinks and relationship
banners, lifecycle dates/commit links, labels, "not ingested" raw files, and a
pdf format. For each: **what should docz-api (or docz upstream) build, what is
the recommended approach, and in what order** — so the site's per-feature
fallbacks can be swapped for real data without breaking anything already
shipped?

## Hypothesis

Three expectations going in:

1. **`index.md` repo homes are the cheapest win** — the ingest pipeline already
   fetches-and-caches exactly one optional extra file per repo (`CHANGELOG.md`
   → `repos.changelog_md`/`changelog_sha`), so a `docs_dir/index.md` sibling
   should be a near-mechanical copy of that precedent plus one new read
   endpoint. This is the stated priority.
2. **Most of the remaining deferred rows collapse onto one keystone** — a
   cross-doc **link graph**. References/Referenced-by, relationship banners,
   and real xref hover previews are all the same missing data structure, so it
   deserves its own design rather than being nibbled at per-feature.
3. **Labels are not docz-api's to start** — docz v1.0.0's `Frontmatter` has no
   labels field, so the chain begins with a docz library feature, not an API
   change.

## Context

- The **docz-site** (DESIGN-0001 in that repo) ships v1 against docz-api's
  existing read+search surface, with deliberate client-side fallbacks where the
  API has no data: the repo home is generated from repo metadata ("No index.md
  configured" fallback), xref links are client-side doc-id matching, the
  lifecycle rail is position-only, and References/Referenced-by, relationship
  banners, labels, raw files, and the MCP/API pages are parked (⏸).
- The site's design explicitly leaves a slot for each: e.g. "when the API grows
  an index endpoint, the rendered `index.md` slots into the same frame through
  the reader pipeline." So the API side can land additively — **nothing new may
  break the existing consumers or wire shapes** (the OpenAPI contract now
  enforces this).
- The docz **wiki** convention already defines what a repo home is: docz's
  mkdocs integration renders `docs_dir/index.md` as the landing page
  (`config.WikiIndexName = "index.md"`, joined to `Cfg.DocsDir` in docz's
  `cmd/wiki.go`). The site rendering the same file means repo pages match the
  wiki without inventing anything new.
- docz-api just froze its wire contract (DESIGN-0002 / IMPL-0002, spec
  `1.0.0`): additions are cheap and safe now — spec + contract-test + minor
  version bump per endpoint.

**Triggered by:** docz-site DESIGN-0001 ("Repos and repo pages" + the mockup
coverage map), user prioritization of `index.md` repo homes.

## Approach

1. Re-read the docz-site coverage map and cluster the deferred rows by the
   **root cause** (missing API data vs missing docz feature vs not-our-work).
2. For the priority (`index.md`): trace the existing single-file fetch/cache
   precedent (`CHANGELOG.md`) through `githubapp.Client.Fetch` →
   `ingest.RepoSnapshot` → `store.ReconcileRepo` → the `repos` schema, and
   confirm push-triggered refresh already covers `docs_dir/index.md`.
3. For the link-graph cluster: inventory what data exists today
   (`documents.raw_md`, `doc_types.id_prefix`) and sketch extraction, storage,
   and API options; identify the scope questions that need a design.
4. For lifecycle/labels/raw/pdf/MCP: identify the blocking dependency (GitHub
   API cost, docz frontmatter surface, or out-of-repo ownership) and assign
   each a disposition.
5. Sequence everything and name the follow-up docs.

## Environment

| Component | Version / Value |
| --- | --- |
| docz library pin | `github.com/donaldgifford/docz v1.0.0` (`doczcfg` / `doczdoc`, guarded by `internal/doczcontract`) |
| docz wiki index | `config.WikiIndexName = "index.md"` at `docs_dir/index.md` (docz `cmd/wiki.go`) |
| docz `Frontmatter` | `ID`, `Title`, `Status`, `Author`, `Created` — **no labels, no relationships** |
| Single-file fetch precedent | `CHANGELOG.md` → `RepoSnapshot.ChangelogMD/ChangelogSHA` → `repos.changelog_md/changelog_sha` (cached, **not** exposed via API yet) |
| `documents` schema | `doc_id`, `title`, `status`, `author`, `created`, `path`, `git_sha` (blob), `content_hash`, `raw_md`, `updated_at` (row time, not commit time) |
| Push refresh filter | `webhook.shouldIngest`: default branch AND (`.docz.yaml` OR `docs_dir/` prefix) — `docs_dir/index.md` already matches |
| Wire contract | `api/openapi.yaml` `1.0.0` + kin-openapi contract test (DESIGN-0002); additive = minor bump |
| docz-site | DESIGN-0001 (v1 ships on the current API; per-feature fallback slots) |

## Findings

### The deferred set clusters into five root causes

The coverage map's ◐/⏸ rows are not eight independent features — they reduce
to five root causes, two of which are not docz-api work at all:

| Mockup row | Root cause | Cluster |
| --- | --- | --- |
| Repo home from `index.md` (◐) | no repo-index endpoint | **F1** |
| References / Referenced-by (⏸) | no link graph | **F2** |
| Relationship banners (⏸) | no link graph (+ docz semantics) | **F2** |
| Xref hover previews (◐) | no link graph (client matching works, no preview/graph data) | **F2** |
| Lifecycle rail dates/commit links (◐) | no per-doc commit metadata | **F3** |
| Labels + label search (⏸) | docz `Frontmatter` has no labels | **F4** |
| Formats: pdf (◐) | consumer-side render, not API data | **F5** |
| "Not ingested" raw files (⏸) | arbitrary-file ingest — future docz/docz-api scope | **F5** |
| MCP page / API reference (⏸) | separate deliverables (docz-mcp; spec already served) | **F5** |

### F1: Repo home from index.md — the priority, and it is small

Everything needed already has a worked precedent in the pipeline:

- **What file:** `docs_dir/index.md` — the exact file docz's wiki renders as
  its landing page (`WikiIndexName` joined to `DocsDir`). Serving this file
  means the site's repo home *is* the wiki home; no new convention, no docz
  change, nothing breaks if the file is absent.
- **Fetch:** `githubapp.Client.Fetch` already does targeted single-blob
  fetches for `.docz.yaml` and an optional root `CHANGELOG.md`. Add one more
  for `docs_dir/index.md`; `RepoSnapshot` grows `IndexMD []byte` /
  `IndexSHA string` mirroring `ChangelogMD`/`ChangelogSHA` (nil/"" when
  absent).
- **Store:** new goose migration adding `repos.index_md TEXT` /
  `repos.index_sha TEXT` (nullable), written by `ReconcileRepo` exactly as the
  changelog columns are — the blob-sha comparison is the cheap no-change gate.
- **Refresh is free:** `shouldIngest` already re-ingests on any default-branch
  push touching `docs_dir/`, which includes `docs_dir/index.md`. No webhook
  change.
- **API options:**

| Option | What | Pros | Cons |
| --- | --- | --- | --- |
| **a. dedicated endpoint** `GET /api/v1/repos/{owner}/{name}/index` | JSON envelope `{repo, index_md, index_sha}`; **404** when the repo has no `index.md` | mirrors the `raw_md`-only-on-`getDoc` discipline (bodies stay out of list/detail payloads); the 404 maps 1:1 onto the site's existing "No index.md configured" fallback; additive minor bump | one more endpoint to spec + test |
| b. field on `RepoDetail` | add `index_md` to `getRepo` | no new route | bloats every repo-detail fetch with a potentially large body; the site fetches repo detail for nav, not reading — wrong altitude |
| c. pseudo-document | ingest `index.md` into `documents` | reuses `getDoc` | pollutes the doc list/search with a non-docz file (no frontmatter/id/type); breaks the "documents are docz docs" invariant everywhere |

**Option a is the recommendation.** It is the same shape the site design
anticipates ("when the API grows an index endpoint, the rendered `index.md`
slots into the same frame through the reader pipeline"), and the whole change
is: one fetch call, two columns, one store write, one endpoint, one spec path +
contract-test case, minor bump to `1.1.0`.

Symmetry note: this makes `index_md` the second cached-but-unserved repo file
after `changelog_md`. If/when the versions feature lands (DESIGN-0001 OQ-12
deferred), a `/changelog` endpoint can copy this endpoint's shape verbatim.

### F2: The cross-doc link graph is the keystone dependency

Three deferred rows (References/Referenced-by, relationship banners, hover
previews) and the upgrade path for a fourth (in-body xref links, currently
client-side doc-id matching) are all views over one missing structure: **which
documents reference which**. Building any one of them piecemeal would create
the graph anyway; it should be designed once.

What exists to build on:

- `documents.raw_md` is already cached per doc — extraction needs no extra
  GitHub fetches.
- `doc_types.id_prefix` per repo gives the exact token set to scan for
  (`RFC-\d{4}`, `FW-\d{4}`, …) — the same matching the site does client-side
  today, moved to ingest where it can be stored and reversed.
- Relative markdown links (`../design/0002-….md`) resolve against
  `documents.path` for the same repo.

Approach sketch (the design to write, not decided here):

- **Extraction at ingest**, per upserted doc (the content-hash gate already
  scopes this to changed docs): scan `raw_md` for known id-prefix tokens +
  resolvable relative links → edge list.
- **Storage:** a `doc_links` table (`repo_id`, `source_doc_id`,
  `target_doc_id`, `kind`, maybe surrounding-text context for previews),
  reconciled delete-and-insert per changed source doc inside the existing
  `ReconcileRepo` transaction.
- **API:** either `getDoc` grows `links` / `referenced_by` arrays, or a
  dedicated `GET …/docs/{doc_id}/links` keeps the doc payload stable —
  same altitude question as F1, to be settled in the design.
- **Two tiers, only tier 1 is docz-api-only:**
  - *Tier 1 — untyped body references* ("this doc mentions RFC-0001"): powers
    References/Referenced-by and hover previews. No docz change needed.
  - *Tier 2 — typed relationships* (the mockup's `instantiates →` banner):
    docz `Frontmatter` has no relationship field, so semantic edges need an
    upstream docz feature (e.g. frontmatter `relates_to:` / `implements:`)
    before docz-api can ingest them. Tier 1 should not wait for tier 2.
- **Scope questions for the design:** repo-local only vs cross-repo (doc ids
  are repo-scoped — `RFC-0001` exists in many repos, so cross-repo needs
  qualified references); false-positive tolerance (a prefix token in a code
  fence); whether search should index "referenced by N docs" for ranking.

This is a real design (schema + extraction semantics + API shape), not a task:
recommend a dedicated **INV/DESIGN** following this one.

### F3: Lifecycle rail — commit dates and links

The rail needs per-doc git history the pipeline never fetches: authoritative
created/last-modified **commit** dates and commit URLs. Today `documents` has
`git_sha` (blob sha, not a commit), `created` (frontmatter date, author-typed),
and `updated_at` (ingest row time — wrong for display).

| Option | What | Pros | Cons |
| --- | --- | --- | --- |
| a. GitHub `ListCommits(path=…)` per doc at ingest | authoritative first/last commit per file | correct + complete | N+1 API calls per repo; rate-limit exposure on onboard of large repos |
| b. harvest from push payloads | `commits[].added/modified/removed` + timestamps arrive free in every push | zero extra API calls | no backfill (onboarded history is empty); misses force-push rewrites |
| **c. hybrid** | (b) incrementally + (a) once per doc at onboard, cached by blob sha, refreshed only for changed docs (the content-hash gate already identifies them) | complete and cheap in steady state | the most moving parts; still N calls at onboard |
| d. cheap subset now | derive blob/file URLs (`https://github.com/{owner}/{name}/blob/{branch}/{path}`) from data already stored; skip dates | zero ingest change; ships a useful link | dates/commit links stay empty — rail remains ◐ |

Recommendation: **(d) is worth taking opportunistically** (it is pure
serialization), but the full rail should be **(c)**, and it should be scoped
*after* F2 — the link-graph design touches the same ingest/store/API layers, so
sequencing it second avoids reworking fresh code.

### F4: Labels are blocked upstream in docz itself

Verified against the pinned library: docz v1.0.0's `Frontmatter` is exactly
`ID / Title / Status / Author / Created` — there is no labels field, so
docz-api has nothing to ingest. The dependency chain is:

1. **docz feature:** frontmatter `labels: []` (+ whatever `docz create` /
   validation support it needs) — a docz repo feature request, not docz-api
   work.
2. docz release + `doczcontract` pin bump here (the contract tests exist
   precisely to make this bump safe).
3. Mechanical downstream: ingest mapper (`labels` → JSONB, mirroring how
   `statuses`/`aliases` are handled), a `documents.labels` column, a Meili
   filterable facet (mirroring `status`/`author`), a `labels` query param +
   facet in the search API, spec minor bump.

Every downstream step mirrors an existing pattern; the only genuinely new work
is step 1, which lives in the docz repo. Recommendation: open the docz feature
request now so it can ride any planned docz release; do nothing in docz-api
until the pin bumps.

### F5: Not docz-api work — pdf, raw files, MCP

- **pdf format:** the formats list needs `md` (already served via `raw_md`)
  and `json` (the API response *is* the json). A pdf is a render of the
  markdown — consumer-side (docz-site or a shared render service), not an API
  data gap. No docz-api work; recommend the site marks it explicitly
  client-deferred.
- **"Not ingested" raw files:** the mockup itself marks arbitrary-file ingest
  as future docz/docz-api work. Ingesting non-docz files would break the
  current invariant that `documents` rows are docz docs (frontmatter, id,
  type) — which F1's option-c analysis shows is worth protecting. Recommend an
  explicit **non-goal** until a concrete need exists; if it comes, it is its
  own design (likely a separate `repo_files` surface, not `documents`).
- **MCP page / API reference:** docz-mcp is its own deliverable. docz-api's
  contribution already shipped — the served `/openapi.yaml` (DESIGN-0002) is
  exactly the artifact an MCP server or API-reference page consumes. Nothing
  further here.

### Every addition rides the OpenAPI contract discipline

All of F1–F4's API surface lands under the DESIGN-0002 regime: each new
endpoint/field is specced in `api/openapi.yaml` (`additionalProperties:
false`), exercised by the kin-openapi contract test, linted by vacuum, and
signalled by a SemVer `info.version` bump (all of the above are additive →
**minor**). The site can pin the spec version per feature: `1.1.0` = index
endpoint exists, `1.2.0` = links, and so on. This is what makes "add features
without breaking anything previously" checkable rather than aspirational.

## Conclusion

**Answer: confirmed on all three hypotheses.**

1. The `index.md` repo home is a **small, fully-precedented, additive change**
   — one extra targeted blob fetch (the `CHANGELOG.md` pattern), two repo
   columns, one new endpoint, one spec path, minor bump. Push-driven refresh
   already works. It is correctly the first thing to build.
2. The largest deferred cluster (References/Referenced-by, relationship
   banners, hover previews, better xrefs) is **one keystone: the cross-doc
   link graph**, extractable from data docz-api already stores (`raw_md` +
   `id_prefix`) with no extra GitHub calls. It has real scope questions
   (typed vs untyped edges, cross-repo identity, API shape) and warrants its
   own design. Typed relationship banners additionally need an upstream docz
   frontmatter feature — untyped references should ship first and not wait.
3. **Labels are blocked in docz**, not docz-api (no frontmatter field in
   v1.0.0); pdf, raw-file ingest, and the MCP page are respectively
   consumer-side, an explicit non-goal for now, and a separate deliverable.

## Recommendation

Sequenced dispositions — each API-surface item is additive and rides the
OpenAPI contract (spec + contract test + minor bump):

| # | Feature | Disposition | Size |
| --- | --- | --- | --- |
| 1 | Repo home `index.md` endpoint (**F1**, option a) | short **DESIGN + IMPL** here: fetch `docs_dir/index.md`, cache on `repos`, serve `GET /api/v1/repos/{owner}/{name}/index`, 404-when-absent; spec → `1.1.0` | **S** |
| 2 | Cross-doc link graph, untyped tier (**F2**) | dedicated **INV → DESIGN** (extraction semantics, `doc_links` schema, API shape, cross-repo scope); unblocks three ⏸/◐ rows at once | **L** |
| 3 | Lifecycle commit metadata (**F3**, option c) | design after the link graph (same layers); take option d (blob URLs from existing data) opportunistically any time | **M** |
| 4 | Labels (**F4**) | **feature request in the docz repo** (frontmatter `labels: []`); docz-api work starts only at the pin bump and is mechanical | M (mostly upstream) |
| 5 | Typed relationship banners (**F2 tier 2**) | fold into the same docz feature request (relationship frontmatter); graph design should leave a `kind` column ready | — |
| — | pdf format | consumer-side; no docz-api work | — |
| — | Raw-file ingest | explicit **non-goal** until a concrete need; own design if it comes | — |
| — | MCP / API reference page | separate deliverable; the served `/openapi.yaml` is docz-api's contribution, already shipped | — |

Concrete next steps:

1. Write the short **DESIGN** for the index endpoint (#1) and implement it —
   the docz-site swaps its generated fallback for the rendered `index.md` at
   spec `1.1.0`.
2. Open the **link-graph INV** (#2) — it is the gate for the biggest cluster
   of deferred site features.
3. File the **docz feature request** covering labels + relationship
   frontmatter (#4/#5) so the upstream chain starts moving in parallel.
4. Record the non-goals (pdf, raw files, MCP) in the docz-site design so the
   coverage map's ⏸ rows point at owned dispositions instead of open ends.

## References

- docz-site DESIGN-0001 — "Repos and repo pages" + the mockup coverage map
  (the deferred ◐/⏸ rows this investigation dispositions)
- [DESIGN-0001](../design/0001-docz-api-cross-repo-docz-registry-and-ingestion-service.md)
  — docz-api service design (ingest pipeline, `documents`/`repos` schema,
  deferred versions feature OQ-12)
- [DESIGN-0002](../design/0002-openapi-contract-for-docz-api-and-the-docz-site.md)
  — the wire contract every addition here rides (spec, contract test, SemVer)
- [INV-0001](0001-migrate-docz-api-to-docz-v100.md) — the docz `v1.0.0` pin
  and `doczcontract` guard (the mechanism a labels-era pin bump rides)
- docz `v1.0.0` `pkg/doczcore/config` — `WikiIndexName = "index.md"` +
  `Cfg.DocsDir` join (`cmd/wiki.go`); `Frontmatter` field set (no labels)
- `internal/githubapp/client.go` — the `.docz.yaml` / `CHANGELOG.md` targeted
  single-blob fetch precedent for `index.md`
- `internal/store/migrations/` — `repos.changelog_md`/`changelog_sha` (the
  cached-but-unserved repo-file precedent) and the `documents` schema
- `internal/webhook/events.go` `shouldIngest` — push refresh already covers
  `docs_dir/index.md`
