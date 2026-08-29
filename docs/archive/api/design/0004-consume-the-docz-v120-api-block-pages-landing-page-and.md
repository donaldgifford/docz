---
id: DESIGN-0004
title: "Consume the docz v1.2.0 api block: pages, landing page, and additional docs"
status: Implemented
author: Donald Gifford
created: 2026-08-28
---
<!-- markdownlint-disable-file MD025 MD041 -->

# DESIGN 0004: Consume the docz v1.2.0 api block: pages, landing page, and additional docs

**Status:** Implemented
**Author:** Donald Gifford
**Date:** 2026-08-28

<!--toc:start-->
- [Overview](#overview)
- [Goals and Non-Goals](#goals-and-non-goals)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Background](#background)
- [Detailed Design](#detailed-design)
  - [Pin bump and contract clause R10](#pin-bump-and-contract-clause-r10)
  - [The published-path namespace](#the-published-path-namespace)
  - [Fetch: apiHint and the widened tree filter](#fetch-apihint-and-the-widened-tree-filter)
  - [Ingest: the page classifier](#ingest-the-page-classifier)
  - [Store: repo_pages and the reconcile](#store-repo_pages-and-the-reconcile)
  - [Serve: list and get-by-path](#serve-list-and-get-by-path)
  - [Webhook: matching files outside docs_dir](#webhook-matching-files-outside-docs_dir)
  - [Search: pages join the palette](#search-pages-join-the-palette)
- [API / Interface Changes](#api--interface-changes)
- [Data Model](#data-model)
- [Testing Strategy](#testing-strategy)
- [Migration / Rollout Plan](#migration--rollout-plan)
- [Open Questions](#open-questions)
  - [1. Who wins a published-path collision between the two namespaces?](#1-who-wins-a-published-path-collision-between-the-two-namespaces)
  - [2. Gate blob fetches on the stored git_sha?](#2-gate-blob-fetches-on-the-stored-git_sha)
  - [3. Search PK scheme: hash pages only, or unify both kinds?](#3-search-pk-scheme-hash-pages-only-or-unify-both-kinds)
- [Follow-ups](#follow-ups)
- [References](#references)
<!--toc:end-->

## Overview

docz `v1.2.0` shipped the `api:` config block — a repo's declaration of what
docz-api ingests and docz-site renders beyond docz documents — and
`docparse.Title`, under contract clause R10 (docz DESIGN-0008). This design
consumes it: docz-api bumps its pin, freezes the new surface in
`internal/doczcontract`, and grows a second record type — **pages** — beside
documents: every `.md` under `docs_dir` (minus exclusions), each
`additional_docs` file outside it, and a configurable landing page, served
raw at `GET /api/v1/repos/{owner}/{name}/pages[/{path}]` and joined into
search under a `source` facet.

All six INV-0008 open questions are answered (1a 2a 3a 4a 5a-amended 6a);
this design turns those decisions into architecture. The consumption rule
itself is docz DESIGN-0011's: *"docz declares; it does not fetch, walk, or
route"* — the fetch, walk, store, and route are specified here.

## Goals and Non-Goals

### Goals

- Pin docz `v1.2.0` and freeze the R10 surface (`Config.API`,
  `ErrInvalidAPIPath`, `docparse.Title`) in `internal/doczcontract`, before
  any runtime caller exists — the R6 pattern.
- Implement DESIGN-0011's six-clause consumption rule, with INV-0008's
  amended directory-page precedence (README wins, lone index serves, root is
  always the landing page).
- A dormant or absent `api:` block produces **today's behavior
  byte-for-byte** — fetch, store, serve, and wire all unchanged (docz
  Decision 4: no repo goes dark, none lights up unasked).
- Pages are desired state, not a cache: disabling the block deletes every
  page row and nulls the persisted config columns on the next ingest, the
  changelog-triple precedent.
- Pages reach the docz-site ⌘K palette (its INV-0003 Obs 5 acceptance bar)
  via the existing Meilisearch index and a `source` facet — in a late phase
  of the same IMPL, not a separate design.
- Additive wire contract only: spec `1.2.1 → 1.3.0`, no existing schema or
  route changes shape.

### Non-Goals

- **Assets and images** (docz Decision 10). docz's rule is `.md`-only; the
  raw-asset endpoint is genuinely new surface on both API and site, deferred
  to its own design. The path mapping defined here is what it would ride.
- **The link graph, lifecycle dates, labels** — INV-0003 F2/F3/F4, untouched
  by `v1.2.0`.
- **Rendering.** Pages serve raw markdown; sanitize/render stays entirely in
  docz-site's one pipeline, exactly as `raw_md`, `index_md`, and
  `changelog_md` do today.
- **docz-site's page tree UI and routes** — the site's own "future DESIGN
  pair" (its DESIGN-0002 non-goal). This design gives it the list shape its
  `byPath` link-resolver map wants; the rest is site work.
- **Narrowed per-push blob fetching** — unchanged from DESIGN-0001's
  deferral, but see OQ-2 for the sha-gated variant this feature makes
  tempting.

## Background

The `api:` block generalizes two slices this repo built one file at a time —
the cached `docs_dir/index.md` repo home (DESIGN-0003/IMPL-0003) and the
opt-in changelog (INV-0005/IMPL-0005) — into a declaration covering any
markdown a repo wants published. INV-0008 records the full investigation:
the upstream chain (docz #81 → INV-0007 → DESIGN-0011 → DESIGN-0008 R10 →
IMPL-0016 → PR #84 → `v1.2.0`), the prior-art trail here (INV-0003 F5's
deliberate "non-goal until a concrete need exists"; IMPL-0005's
"generic consumed-files pattern later" seam), and the six answered
decisions this design builds on.

The R10 contract docz-api may rely on, compressed (full text in INV-0008
Observation 1): **dormancy** (a disabled/absent block never fails `Load` or
`Validate`), **normalization at load** (backfilled landing page, one
spelling arrives), **validation when enabled** (repo-relative, clean, no
traversal/Win32/reserved-segment tricks, wrapping `ErrInvalidAPIPath`),
**path-shape only** (docz never checks existence — consumers skip missing
files and report them), and **`Title` is total** (first H1's stripped text
or `""`; consumer supplies the filename fallback; frontmatter `title:` is
never read).

Two constraints inherited from the bump itself (INV-0008 Observation 3):
crossing `v1.1.1` flips the built-in PLAN type default to disabled, which
changes ingest's desired state for repos without an explicit `types:` block
— a **data-deletion risk** requiring a fleet check before the bump ships;
and `v1.2.0` narrows `changelog.file` validation (trailing-period
segments), which the R1–R6 contract suite must confirm harmless.

## Detailed Design

### Pin bump and contract clause R10

`go get github.com/donaldgifford/docz@v1.2.0` (never a bare `go mod tidy`),
then the existing R1–R6 suite must pass unchanged — that is the whole point
of `internal/doczcontract`. A new `api_test.go` in the R6 mold (own file,
own doc-comment header, the temp-dir + `HOME`-override loader) freezes
clause **R10**:

- **Dormancy:** an absent block and `enabled: false` with hostile paths both
  load and validate clean; `Config.API` zero values (`Enabled=false`,
  `LandingPage=""`, nil slices) for the absent case.
- **Normalization:** `./docs/index.md` → `docs/index.md`; `scratch/` →
  `scratch`; enabled + empty landing page backfills to
  `<docs_dir>/index.md`, tracking a non-default `docs_dir`.
- **Enabled validation:** traversal, absolute, percent-encoded-separator,
  Win32 trailing-period, under-`docs_dir` `additional_docs`, and
  reserved-first-segment cases all fail wrapping `ErrInvalidAPIPath`
  (`errors.Is`); the same values validate clean when dormant.
- **`Title`:** ATX with inline markdown stripped, setext, frontmatter
  skipped, frontmatter-`title:`-ignored (`""`), prose-only `""` — the cases
  docz's own `consumer_v12_test.go` pins, mirrored here so *our* pin fails
  if they drift.

`internal/doczcontract/doc.go` updates R1–R6 → R1–R6+R10 (our clause
numbering follows docz DESIGN-0008's, as R6 already does).

The PLAN-flip pre-flight (INV-0008 Recommendation 3) runs before the bump
deploys, against our own data:

```sql
SELECT r.owner || '/' || r.name
FROM repos r
JOIN doc_types t ON t.repo_id = r.id AND t.name = 'plan'
WHERE NOT (r.config_snapshot ? 'types');
```

Any hit either gets `types.plan.enabled: true` committed upstream first, or
the deletion is accepted knowingly. The check and its outcome are recorded
in the IMPL.

### The published-path namespace

Every page is addressed by one string, its **published path** — the lookup
key for `GET .../pages/{path}`, the `UNIQUE (repo_id, path)` key in
Postgres, and the path docz-site's `byPath` resolver map keys on. The
mapping implements DESIGN-0011's rule plus INV-0008's 5a amendment:

| Source file | Published path | Why |
| ----------- | -------------- | --- |
| `<landing_page>` (default `docs/index.md`) | — (repo row, served by `/index`) | The repo root is always the landing page (5a rule 1); it is a pointer on the repo record, not a page row (DESIGN-0011 data model) |
| `docs/impl/README.md` | `impl` | A directory's `README.md` is that directory's page — including type dirs, where the docz-generated table *is* the type page |
| `docs/guides/index.md` (no README) | `guides` | Lone `index.md` serves as the directory page (5a amendment; docz README's "index.md or README.md") |
| `docs/guides/index.md` (README present) | `guides/index.md` | Both present → `README.md` wins the directory; the index is path-addressed, nothing dropped |
| `docs/examples/example1.md` | `examples/example1.md` | Path-addressed at its `docs_dir`-relative path, extension kept (6a) |
| `CONTRIBUTING.md` (additional_docs) | `CONTRIBUTING.md` | Repo-relative, extension kept |

Directory pages publish at the **directory** path (no extension — the
address is the directory, per DESIGN-0011's route table); file pages keep
their extension (6a). The two never collide: a directory `examples` and a
file `examples.md` publish as `examples` and `examples.md`.

What *can* collide is the two namespaces: `docs_dir`-relative paths and
repo-relative `additional_docs` paths share one address space by design
(that is what makes the URL mirror the tree), so `docs/examples/a.md`
(publishes `examples/a.md`) collides with an `additional_docs` entry
`examples/a.md` at the repo root — which docz's validation cannot see,
because each namespace is internally valid. Resolution is **OQ-1**; the
recommendation makes the `docs_dir`-derived page win deterministically and
skips the additional_doc with a Warn.

Titles: `docparse.Title(content)`, falling back to a title-cased basename
for file pages and the directory name for directory pages (docz-api's own
helper — docz's `FilenameTitle` is `internal/`, and INV-0007's amendment
blesses an independent implementation as presentation, not grammar).

### Fetch: apiHint and the widened tree filter

`internal/githubapp` gains `apiHint(configYAML []byte)` — the third
fetch-scoped hint beside `docsDirHint` and `changelogHint`: a one-block
`yaml.Unmarshal` of `api:` returning `(enabled, landingPage, exclude,
additionalDocs)`, docz defaults on malformed yaml, `./` and trailing-`/`
trimmed to match docz's normalization. As with the other hints, the
**authoritative** parse stays in ingest's `loadConfig`; the hint only
decides what to fetch, so a malformed config still fails ingest in exactly
one place.

`classifyTree` changes only when the hint reports enabled:

- **Dormant/absent (the default):** today's keep-set byte-for-byte —
  `.docz.yaml` plus `IsDoczFile` matches under type dirs, plus the
  unconditional `docs_dir/index.md` and conditional changelog lookups.
- **Enabled:** the keep-set widens to **every `.md` under `docs_dir/`**
  (the exclusion filtering happens in ingest, where the authoritative
  config lives — the fetch over-fetches excluded files rather than
  duplicating deny-list logic; see OQ-2 for the cost lever). The landing
  page is resolved by `findBlobSHA` at the hint's `landingPage` (falling
  back to `docs_dir/index.md`) instead of the hard-wired index path — same
  snapshot fields, different source path. Each `additionalDocs` entry gets
  its own `findBlobSHA` lookup against the already-listed recursive tree:
  one blob request per present file, **zero for absent ones** (R10 clause
  d: docz never checked existence; we skip and report).

`RepoSnapshot` is unchanged structurally: page candidates ride the existing
`Blobs []BlobEntry` alongside doc blobs (ingest discriminates), and the
landing page stays `IndexMD`/`IndexSHA`.

### Ingest: the page classifier

`loadConfig` → `cfg.Validate()` is untouched: an invalid **enabled** `api:`
block fails the whole ingest, one error path, exactly like a bad
`changelog.file` (IMPL-0005 OQ-2a). A dormant block with garbage paths
passes, per R10 dormancy — and produces no pages.

A new `buildPages(cfg, blobs)` runs beside `buildDocuments`, implementing
the consumption rule over the widened blob set. Per blob, in order:

1. Path equals the resolved landing page → **skip** (repo-row cache).
2. Outside `docs_dir` and not in `additional_docs` → skip (over-fetch
   guard; today's `buildDocuments` has the same shape via its `typeByDir`
   miss).
3. Under `<docs_dir>/templates/` or an `api.exclude` prefix → **skip**
   (always-excluded beats everything; the deny-list is the authoritative
   post-`Load` config, exclusive of the fetch hint).
4. Inside an enabled type's dir: `README.md` → **directory page** for that
   type dir; `IsDoczFile` + frontmatter → **document** (the existing
   pipeline, unchanged — `IsDoczFile` is now the discriminator DESIGN-0011
   says it becomes, not a keep-filter); anything else → **skip + Warn**
   (DESIGN-0011 rule 3: a stray file in a docz-managed namespace is more
   likely a mistake than an intent; the Warn is the "report" half of
   skip-and-report).
5. Anywhere else under `docs_dir`: `README.md` (or lone `index.md`) →
   directory page at the directory's published path; everything else → file
   page at its `docs_dir`-relative path.
6. In `additional_docs` → file page at its repo-relative path.

`buildDocuments` itself is untouched except for its input: the widened blob
set means its `typeByDir` miss branch now sees page candidates and must
stay silent about them (they are `buildPages`'s business) — the existing
"skip quietly on dir miss" behavior already does this.

The type-dir `README.md` is called out in the IMPL as its own test case: it
is DESIGN-0011's predicted most-likely bug (fails `IsDoczFile`, lives in a
type dir, must be **kept**).

### Store: repo_pages and the reconcile

One migration adds the pages table and two repo columns:

```sql
CREATE TABLE repo_pages (
    id           BIGSERIAL PRIMARY KEY,
    repo_id      BIGINT NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
    path         TEXT   NOT NULL,  -- published path (the addressing key)
    repo_path    TEXT   NOT NULL,  -- actual file path in the repo
    title        TEXT   NOT NULL,  -- docparse.Title, fallback applied at ingest
    git_sha      TEXT   NOT NULL,  -- blob sha (cache key, like index_sha)
    content_hash TEXT   NOT NULL,  -- sha256(raw_md) — the re-ingest gate
    raw_md       TEXT   NOT NULL,  -- cached markdown (NOT html)
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (repo_id, path)
);
CREATE INDEX repo_pages_repo_idx ON repo_pages (repo_id);

ALTER TABLE repos
    ADD COLUMN api_landing_page    TEXT,   -- resolved landing_page; NULL = block disabled
    ADD COLUMN api_additional_docs JSONB;  -- normalized additional_docs; NULL = block disabled
```

`repo_pages` deliberately mirrors `documents` minus the docz-only columns
(no `doc_id`, `type`, `status`, `author`, `created`) — pages are the
"second record type keyed `(repo_id, path)`" both DESIGN-0011's data model
and INV-0003's `repo_files` instinct describe, and `documents`' NOT NULL
contract stays intact.

The two repo columns are the webhook's exact-match surface (4a — the
`changelog_file` precedent, pluralized), written by the reconcile from the
**post-`Load`** config, so the webhook always matches what the last ingest
actually resolved. `api_landing_page` exists because an enabled block may
point the landing page **outside** `docs_dir` (e.g. a root `README.md`),
where `shouldIngest`'s prefix check would never see a push to it — the
precise gap `changelog_file` closed for the changelog. Both are NULL when
the block is disabled: desired state, not a cache.

`ReconcileInput` grows `Pages []PageInput` and the repo input the two
fields; `reconcileRepoPages` mirrors `reconcileDocuments` exactly — hash
map of current rows, content-hash gate (unchanged pages are no-ops), upsert
changed, delete absent, all inside the one existing transaction —
surfacing `UpsertedPagePaths`/`DeletedPagePaths` on `ReconcileResult` for
the search sync. A dormant block reconciles an **empty** page set: every
row deleted, both columns nulled, the landing-page cache reverting to plain
`docs_dir/index.md` semantics. Nothing served regresses, because nothing
was served.

### Serve: list and get-by-path

Two routes join the repo-scoped group (behind the same gate, same
existence-hiding `resolveRepo`):

- **`GET /api/v1/repos/{owner}/{name}/pages`** → `{"pages": [{"path",
  "title", "git_sha"}]}`, ordered by path. The flat list is deliberate
  (2a): docz-site builds its own tree and its link-resolver `byPath` map
  from exactly this shape. A repo with no pages — including every
  non-opted-in repo — returns `200 {"pages": []}`, not 404: the repo
  exists; its page set is empty. (Existence hiding stays at the repo
  level.) The site distinguishes "doesn't do pages" via `config_snapshot`,
  the `changelogConfig` precedent.
- **`GET /api/v1/repos/{owner}/{name}/pages/*`** (chi wildcard) →
  `{"repo", "path", "title", "raw_md", "git_sha"}`, 404 `{"error":"page
  not found"}` otherwise. The handler takes chi's decoded wildcard value
  and **re-validates before lookup**: non-empty, no `..` or `.` segments,
  no leading `/`, no `\`, no control characters — rejecting with 404 (not
  400: an invalid path is definitionally not a page, and existence hiding
  argues for one indistinguishable miss). docz's validation explicitly
  does not survive URL decoding (*"a consumer that decodes a
  config-sourced path before resolving it has voided this check"*), so
  this check is load-bearing, not belt-and-braces. Lookup is **exact-byte**
  on the stored published path — no case folding, no Unicode
  normalization; git is case-sensitive and serving anything but exact
  bytes reopens the aliasing problems docz's validator closed.

`pages` becomes a reserved literal under the repo route, beside `index`,
`changelog`, and `types` — no collision on the API side (`{type}` only
appears under `/types/`), and the site-side reserved-word budget is the
site's design to spend.

### Webhook: matching files outside docs_dir

`shouldIngest` grows two exact-path checks, fed from the repo row it
already holds:

```text
p == doczConfigFile                          (existing)
strings.HasPrefix(p, docsDir+"/")            (existing — covers ALL under-docs_dir pages)
p == changelogFile        when non-empty     (existing)
p == apiLandingPage       when non-empty     (new — landing page outside docs_dir)
p ∈ apiAdditionalDocs     when non-empty     (new — set membership, one row read)
```

Everything else is unchanged: default-branch check, full re-ingest through
the queue, debounce, content-hash gate. Pages under `docs_dir` need no
webhook change at all — the prefix check has covered them since Phase 5.

### Search: pages join the palette

Landed as the IMPL's final feature phase (1a): the site's acceptance bar is
that every rendered page is findable from the ⌘K palette, and a
rendered-but-unfindable window is acceptable only while the IMPL is still
open.

- **Both record kinds gain `source`** (`"doc"` / `"page"`) in the index
  document and as a filterable facet. Existing doc entries pick it up on
  their next content-hash-gated reindex; a search filtered `source = doc`
  behaves identically to today's unfiltered search in the interim (docs
  without the field simply don't match `source = page`).
- **Doc PKs are unchanged** (`<repo_id>_<doc_id>`); pages get
  `<repo_id>_p_<hex(sha256(published_path))[:16]>` — paths violate the PK
  charset (`[a-zA-Z0-9-_]`), and hashing beats escaping because it is
  fixed-length and cannot collide with any doc_id spelling. The PK never
  appears on the wire (the Phase 3 precedent). See OQ-3 for the
  one-scheme-for-both alternative and why it loses.
- **Page index docs** carry `source`, `repo`, `repo_id`, `path`, `title`,
  `body`, `updated_at` — no `type`/`status`/`author`/`created` (they have
  none; empty strings, the wire's existing "unset" convention).
- **`SearchHit`** gains `source` and `path` (both required; docs fill
  `path` with their repo path, which is useful in its own right). The
  page-hit `doc_id`/`type`/`status`/`author` are `""`. Facet counts for
  `type`/`status`/`author` simply never include pages; the new `source`
  facet splits the two populations.
- **Deletion** rides the same sync: `DeletedPagePaths` → PK hashes →
  `DeleteDocuments`, alongside the existing doc flow; offboarding's
  `repo_id` filter purge already covers pages (same index, same
  `repo_id` field).

## API / Interface Changes

Spec `1.2.1 → 1.3.0` (additive):

- `GET /api/v1/repos/{owner}/{name}/pages` — `PageList` (`{"pages":
  [PageSummary]}`), `PageSummary{path, title, git_sha}`,
  `additionalProperties: false`.
- `GET /api/v1/repos/{owner}/{name}/pages/{path}` — `Page{repo, path,
  title, raw_md, git_sha}`; 404 via the standard error envelope. `{path}`
  is spec'd `style: simple` with a description stating it is a
  slash-containing repo path and that clients percent-encode it as one
  segment; the kin-openapi contract test exercises the percent-encoded
  spelling, which chi routes identically to the literal one (2a).
- `SearchHit` gains `source` (`"doc" | "page"`) and `path` — additive,
  required, with the existing empty-string-means-unset convention for the
  doc-only fields on page hits.
- Descriptions only: `getRepoIndex` documents that an enabled `api:` block
  may point the served landing page at a configured path (default
  unchanged).

Config, flags, CLI: **no changes**. The feature is driven entirely by each
repo's `.docz.yaml`; docz-api has no server-side toggle (a repo that opts
in is served — the operator surface is unchanged).

Go interface deltas, all consumer-side and narrow: `ingest.Service`'s
store seam gains the pages reconcile fields; the `Indexer` seam is
unchanged in shape (pages ride `IndexDocuments`/`DeleteDocuments` as
`IndexDoc`s); `httpapi.storeReader` gains `ListRepoPages` and
`GetRepoPageByPath`.

## Data Model

The migration in [Store](#store-repo_pages-and-the-reconcile), summarized:

| Object | Change | Notes |
| ------ | ------ | ----- |
| `repo_pages` (new) | `(repo_id, path)` unique; `repo_path`, `title`, `git_sha`, `content_hash`, `raw_md` | The second record type; CASCADE on repo delete; no FK into `doc_types` |
| `repos.api_landing_page` | nullable TEXT | Resolved post-`Load` path; NULL = disabled; webhook exact-match |
| `repos.api_additional_docs` | nullable JSONB | Normalized string array; NULL = disabled; webhook set-match |
| `repos.index_md` / `index_sha` | **no change** | Cache the landing page; source path follows `api_landing_page` when enabled (3a) |
| `documents` | **no change** | The "documents are docz docs" invariant holds |

Presence semantics repeat the house gotcha deliberately: an
empty-but-present page stores `raw_md = ''` with a valid `git_sha`
(`textOrNull` does not apply — `raw_md` is NOT NULL on `repo_pages`, and
row existence is the presence signal, so pages avoid the sha-gating
subtlety the repo-row caches need).

## Testing Strategy

- **Contract (doczcontract):** the R10 suite above; R1–R6 re-run green on
  the bumped pin — this is the gate everything else waits on.
- **Unit — classifier:** table-driven `buildPages` cases: both-index-files
  precedence (README wins, index path-addressed), lone index as directory
  page, type-dir README kept (the predicted bug), type-dir stray skipped
  with Warn, templates/ and exclude pruning, landing page skipped,
  additional_docs mapping, the OQ-1 collision (deterministic winner +
  Warn), title fallbacks (no-H1 file → filename; directory page →
  directory name).
- **Unit — fetch:** `apiHint` malformed/absent/partial yaml (docz
  defaults); stub-roundtripper `Fetch` cases: dormant block fetches
  today's set byte-for-byte (assert by withheld blobs, the
  `TestFetchRepoChangelog` technique), enabled block widens, absent
  additional_docs cost zero requests, landing override fetches the
  configured path.
- **Unit — serve:** decoded-path re-validation (traversal, `.`, absolute,
  backslash, control bytes → 404), exact-byte lookup (case miss → 404).
- **Store integration:** migration up/down; reconcile round-trip; hash
  gate no-op; delete-absent; disable-at-HEAD wipes rows and nulls both
  columns.
- **Contract test (httpapi):** list happy + empty, page happy + 404, the
  percent-encoded wildcard spelling, `SearchHit` with `source`/`path` —
  all against spec `1.3.0`, `additionalProperties: false` doing the drift
  detection.
- **e2e (`internal/e2e`):** onboard a fixture repo with an enabled block →
  pages listed and served → push disabling the block → rows gone, 404s —
  the `TestE2ERepoChangelogServeAndDisable` shape.
- **Search integration:** page indexing, `source` facet counts, page-hit
  shape, deletion, repo-scope filter still applied.
- **Webhook unit:** `shouldIngest` matches the persisted landing page and
  additional_docs paths; NULL columns match nothing.

## Migration / Rollout Plan

1. **Pre-flight:** the PLAN-flip fleet query; fix or accept each hit
   (recorded in the IMPL). This gates the pin bump, not the feature.
2. The migration is additive and instant (new empty table, two nullable
   columns); auto-migrate on deploy as always.
3. **Natural refresh only** (the DESIGN-0003 precedent): no backfill. A
   repo serves pages after its first ingest with an enabled block — which
   is the push that enables the block, or an operator `-onboard`. Every
   repo without the block is untouched at every layer.
4. Docs: `deploy/README.md` and `api/README.md` gain the consumer note
   DESIGN-0011's rollout caution demands — **enabling publishes every
   `.md` under `docs_dir`**, scratch files included; `exclude` is the
   guard rail. Dogfood by enabling the block on this repo and docz first.
5. Upstream follow-ups filed against docz (from INV-0008 Recommendation 5
   plus this design): close #81 against `v1.2.0`; report the IMPL-0016
   Phase-4 leftovers; ask for the directory-page precedence ratification
   (5a-amended) and the cross-namespace uniqueness check (OQ-1c's
   hardening half).
6. docz-site coordination: an issue describing the new surface once the
   spec lands — the list shape for `byPath`, the `source` facet, the
   reserved-word note for its router. Site work is its own design there.

## Open Questions

Answer each with a letter — **a is the recommendation**, b onward are
alternatives; write in your own option if none fits.

**All answered `a` (2026-08-28).** OQ-1a comes with a review note: in
practice most repos will put their non-docz markdown under `docs_dir` too,
so the two namespaces rarely both carry entries and collisions should stay
rare — 1a is the safety net for the rare case, not a hot path. (That same
observation is why the deterministic-winner rule costs nothing: it almost
never fires, but when it does, it fires the same way every time.)

### 1. Who wins a published-path collision between the two namespaces?

`docs_dir`-relative page paths and repo-relative `additional_docs` paths
share one address space by design — that is what makes the URL mirror the
tree. So `docs/examples/a.md` (publishes `examples/a.md`) collides with an
`additional_docs` entry `examples/a.md` at the repo root, and `docs/README.md`
(publishes `README.md` when not the landing page) collides with an
`additional_docs` root `README.md`. docz's validator cannot see this — each
namespace is internally valid — and `UNIQUE (repo_id, path)` makes the
second write fail, so the resolution must be deliberate, not incidental.

- **a. (Recommendation) The `docs_dir`-derived page wins, deterministically;
  the `additional_docs` entry is skipped with a Warn naming both files.**
  The docs_dir page is convention (it exists because the tree exists); the
  additional_docs entry is configuration (one line the repo owner can
  change), so the fixable side loses. Deterministic regardless of blob
  order, covered by a classifier test, and the Warn satisfies
  skip-and-report. File the upstream ask for a cross-namespace validation
  rule so future docz versions reject it at `Validate` time.
- b. Publish everything at repo-relative paths — collisions become
  impossible. But it breaks DESIGN-0011's governing rule (URLs would grow a
  `docs/` segment), the docz README's published URL table, and the site's
  route expectations, for an edge case.
- c. Fail the whole ingest on collision (treat it like an invalid config).
  Consistent with the one-error-path rule — but docz's `Validate` passes
  this config, so ingest would be rejecting configs docz calls valid, and a
  repo could be knocked fully dark by one ambiguous entry.

### 2. Gate blob fetches on the stored git_sha?

The widened filter fetches every kept `.md`'s blob on every ingest, then
the content-hash gate discards unchanged ones at reconcile — the same
fetch-everything shape documents have today, but pages can multiply the
blob count. The recursive tree listing already carries each blob's sha, and
we store `git_sha` per row, so skipping the fetch when they match is
correct by construction (a git sha identifies content).

- **a. (Recommendation) Defer, unchanged from DESIGN-0001's deferral — but
  record it as the known cost lever.** At homelab scale the extra blob
  GETs are noise (rate limit 5,000/hr/installation; the debounce already
  coalesces bursts), the fetch-everything pipeline is uniform and easy to
  reason about, and the optimization is a drop-in later (compare tree sha
  → stored sha → skip fetch) precisely because the data is already
  persisted. Doing it now would also gate *documents* or leave the two
  pipelines asymmetric mid-feature.
- b. Implement sha-gated fetching for pages only, now. Halves the new cost
  where it appears, but forks the fetch shape between record kinds.
- c. Implement it for all blobs (docs + pages) in this work. The right end
  state, but it grows this feature's blast radius into the proven document
  path for a cost nobody is paying yet.

### 3. Search PK scheme: hash pages only, or unify both kinds?

INV-0008 OQ-1's answer said the PK scheme "changes once for both record
kinds"; detailed design says otherwise — changing doc PKs orphans every
existing index entry (delete-by-PK no longer matches), forcing an explicit
full purge + reindex on deploy.

- **a. (Recommendation) Docs keep `<repo_id>_<doc_id>`; pages get
  `<repo_id>_p_<hex(sha256(published_path))[:16]>`.** No migration, no
  orphaned entries, no deploy-time reindex; doc_ids already satisfy the PK
  charset so they never needed encoding. The `p` discriminator makes the
  namespaces disjoint by construction (`sha` output is hex; a doc_id would
  have to *be* a 16-char hex string prefixed `p_` to collide — and even
  then the `repo_id_` prefix differs). Amends the INV's sketch, which this
  OQ records explicitly.
- b. One hashed scheme for both kinds, with a one-time index purge + full
  reindex in the deploy (the reconcile re-indexes on next ingest per repo,
  so the search index is partially empty until the fleet re-ingests).
  Uniform code, ugly rollout window.
- c. Encode paths into the PK charset (e.g. `-`/`_` escaping) instead of
  hashing. Human-readable PKs, but variable-length, collision-prone under
  escaping mistakes, and the PK is never user-visible anyway.

## Follow-ups

- The asset/raw-file endpoint (docz Decision 10's retained option) — its
  path mapping is this design's, when a concrete need arrives.
- Sha-gated blob fetching (OQ-2a's deferral) as its own small change once
  fetch volume warrants it.
- docz-site: the page-tree route family and nav — the site's "future
  DESIGN pair", unblocked by this surface.

## References

- [INV-0008] — the grounding investigation; all six OQs answered
  (1a 2a 3a 4a 5a-amended 6a)
- docz DESIGN-0011 — the consumption rule (upstream)
- docz DESIGN-0008 clause **R10** — the consumer contract this design pins
- docz PR [#84] / `v1.2.0` — the release
- [INV-0003] — the deferred-features investigation whose F5 this supersedes
- DESIGN-0003 / IMPL-0003 — the repo-index slice (cache + serve precedent)
- [INV-0005] / IMPL-0005 — the changelog slice (opt-in fetch, persisted
  path, desired-state nulling precedent)
- docz-site INV-0003 Obs 5 — the search acceptance bar; DESIGN-0002 — the
  `byPath` link resolver this feeds

[INV-0008]: ../investigation/0008-adopt-the-docz-v120-api-block-additional-docs-landing-page-and.md
[INV-0003]: ../investigation/0003-docz-site-deferred-features-and-the-docz-api-surface-to-unblock.md
[INV-0005]: ../investigation/0005-changelog-as-a-first-class-docz-artifact.md
[#84]: https://github.com/donaldgifford/docz/pull/84
