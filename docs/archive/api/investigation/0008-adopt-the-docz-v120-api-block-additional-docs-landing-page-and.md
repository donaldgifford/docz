---
id: INV-0008
title: "Adopt the docz v1.2.0 api block: additional docs, landing page, and path-addressed pages"
status: Concluded
author: Donald Gifford
created: 2026-08-27
---
<!-- markdownlint-disable-file MD025 MD041 -->

# INV 0008: Adopt the docz v1.2.0 api block: additional docs, landing page, and path-addressed pages

**Status:** Concluded
**Author:** Donald Gifford
**Date:** 2026-08-27

<!--toc:start-->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
  - [Observation 1 — what v1.2.0 ships, and what R10 obligates docz-api to](#observation-1--what-v120-ships-and-what-r10-obligates-docz-api-to)
  - [Observation 2 — prior docz-api work: predicted, deliberately deferred, never designed](#observation-2--prior-docz-api-work-predicted-deliberately-deferred-never-designed)
  - [Observation 3 — the pin bump crosses two default-changing releases](#observation-3--the-pin-bump-crosses-two-default-changing-releases)
  - [Observation 4 — the documents table cannot carry pages; a second record type is required](#observation-4--the-documents-table-cannot-carry-pages-a-second-record-type-is-required)
  - [Observation 5 — pipeline deltas are small and mostly relaxations](#observation-5--pipeline-deltas-are-small-and-mostly-relaxations)
  - [Observation 6 — the serve surface is new, and the path-shaped route has sharp edges](#observation-6--the-serve-surface-is-new-and-the-path-shaped-route-has-sharp-edges)
  - [Observation 7 — docz-site is further along than expected](#observation-7--docz-site-is-further-along-than-expected)
  - [Observation 8 — inherited security posture and upstream loose ends](#observation-8--inherited-security-posture-and-upstream-loose-ends)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
- [Open Questions](#open-questions)
  - [1. Do pages join search, and when?](#1-do-pages-join-search-and-when)
  - [2. Serve shape for path-addressed pages?](#2-serve-shape-for-path-addressed-pages)
  - [3. Landing page: generalize the index columns or add new ones?](#3-landing-page-generalize-the-index-columns-or-add-new-ones)
  - [4. How does the webhook match additional_docs pushes?](#4-how-does-the-webhook-match-additional_docs-pushes)
  - [5. index.md vs README.md when a directory has both?](#5-indexmd-vs-readmemd-when-a-directory-has-both)
  - [6. Does the published path keep the .md extension?](#6-does-the-published-path-keep-the-md-extension)
- [References](#references)
<!--toc:end-->

## Question

docz `v1.2.0` (docz PR [#84], released 2026-08-26) shipped the `api:` config
block — a repo-level declaration of what docz-api ingests and docz-site
renders beyond docz documents — plus `docparse.Title`. Its release notes name
this repo directly: *"This is the release docz-api pins for R10."*

Three questions:

1. What work has docz-api already done, designed, or recorded around this
   feature set?
2. What does consuming `v1.2.0` actually require of docz-api — contract,
   pipeline, storage, serve surface?
3. Is the existing material current enough to go straight to an IMPL, or does
   this need a DESIGN first?

## Hypothesis

We discussed this feature before docz could support it, so *something* exists
here — but probably as scattered deferrals rather than a design, because the
docz-side code had to land first. Expect the changelog slice (INV-0005 /
IMPL-0005) to be the closest structural precedent, and expect the consumption
work to be real but bounded: docz deliberately kept fetch/walk/route out of
scope, so the interesting decisions live on our side.

## Context

The `.docz.yaml` `api:` block generalizes what this repo has so far built
one file at a time: the cached `docs_dir/index.md` repo home (DESIGN-0003 /
IMPL-0003) and the opt-in changelog (INV-0005 / IMPL-0005). With it, a repo
declares a landing page, exclusions, and `additional_docs` — markdown outside
`docs_dir` — and, when enabled, every `.md` under `docs_dir` becomes
consumable at its `docs_dir`-relative path.

Upstream, the ask originated as docz issue [#81] ("Docz config"), was
investigated as docz INV-0007, designed as docz DESIGN-0011, contracted as
docz DESIGN-0008 clause **R10**, and implemented via docz IMPL-0016 → PR
[#84] → `v1.2.0`. docz-api pins `v1.1.0` today and references none of it.

**Triggered by:** docz PR [#84] / docz `v1.2.0` release; docz DESIGN-0008
clause R10; this repo's [INV-0003] (Open) and its deferred raw-files cluster.

## Approach

1. Read docz PR #84, DESIGN-0011, DESIGN-0008 R10, IMPL-0016, and the
   docz-side INV-0007 from the docz repo at `origin/main`; extract the
   consumer contract and the `v1.1.0 → v1.2.0` behavioral delta from the
   actual code and tag diffs.
2. Sweep this repo (docs, code, git log, issues/PRs) for prior art on
   additional docs / arbitrary files / landing pages, and map the existing
   index + changelog slices' machinery from code.
3. Sweep docz-site (issues, docs, code at `origin/main`) for the site-side
   asks, current API consumption, and rendering/routing readiness.

## Environment

| Component | Version / Value |
| --------- | --------------- |
| docz-api docz pin (`go.mod`) | `v1.1.0` |
| docz latest release | `v1.2.0` (tag `d81b8ad`, 2026-08-26) |
| Releases crossed by the bump | `v1.1.1` (PLAN default flip), `v1.2.0` |
| docz-api spec (`api/openapi.yaml`) | `1.2.1` |
| docz-site vendored spec | `1.2.0` (re-vendor tracked in docz-site #17) |
| docz-api HEAD at investigation | `ccd6e43` (main) |

## Findings

### Observation 1 — what v1.2.0 ships, and what R10 obligates docz-api to

**Surface.** `pkg/doczcore/config` gains `Config.API` of type
`APIConfig{Enabled, LandingPage, Exclude, AdditionalDocs}`, the sentinel
`ErrInvalidAPIPath`, and `APILandingFileName` (`"index.md"`, deliberately a
separate constant from `WikiIndexName`). `pkg/doczcore/docparse` gains
`Title(content []byte) string`. Nothing else in the public surface moved —
docz's own godoc diff against `v1.1.1` is additions-only, and
`pkg/doczcore/document` is byte-identical to `v1.1.0` (`IsDoczFile` still
requires leading digits).

**The R10 contract** (docz DESIGN-0008, "Requirements for the docz repo"
section) is written as acceptance criteria docz-api may rely on:

- **a. Dormancy** — `enabled: false` or an absent block never fails `Load` or
  `Validate`, whatever paths it holds. Same additive-rollout property R6
  pinned for the changelog block.
- **b. Normalization at load** — `LandingPage` backfilled to
  `<docs_dir>/index.md` when empty *and enabled*; leading `./` stripped from
  all three fields; trailing `/` stripped from `Exclude` entries. One
  spelling arrives; consumers don't canonicalize.
- **c. Validation when enabled** — repo-relative, clean, slash-separated; no
  `..`, no absolute paths, no volume names, no control/format characters, no
  segment ending in a space or period (the Win32 `...`-resolves-as-`..`
  rule). `AdditionalDocs` additionally: case-insensitively unique, outside
  `docs_dir`, not duplicating `LandingPage`, first segment not a token an
  enabled type resolves from (name, alias, `id_prefix`, dir — checked in raw
  *and* percent-decoded spelling). All failures wrap `ErrInvalidAPIPath`.
- **d. Validation is path-shape only** — docz never checks existence.
  Consumers skip missing files and report them.
- **e. `Title` is total** — first H1's text with inline markdown stripped, or
  `""`; `""` is normal, consumer supplies the fallback (filename). It reads
  setext H1s and skips YAML frontmatter — but **never reads a frontmatter
  `title:` key**; a frontmatter-only file returns `""`.

**The consumption rule is explicitly ours to implement.** R10 closes with:
*"markdown only, a directory's index file is that directory's page, type
directories reserved for docz documents, everything else path-addressed at
its `docs_dir`-relative path, `<docs_dir>/templates/` always excluded — is
specified in DESIGN-0011 and is docz-api's to implement. docz declares; it
does not fetch, walk, or route."* DESIGN-0011's six clauses, compressed:

1. Markdown only (`*.md`); assets are out of scope (docz Decision 10 — but
   see its retained note: docz-api *may* serve images at their
   `docs_dir`-relative path with no docz involvement, since the path mapping
   is already defined).
2. A directory's index file is that directory's page — `docs/index.md` is
   the repo root; `<dir>/README.md` is `<dir>`'s page, uniformly, including
   type directories (the docz-generated index table *is* the type page).
3. Type directories are otherwise reserved: a non-`README.md` file there
   must match `IsDoczFile` + have frontmatter; a stray non-conforming `.md`
   is **skipped and reported**, not path-addressed.
4. Everything else under `docs_dir` — minus excludes, minus consumed index
   files — is path-addressed at its `docs_dir`-relative path.
5. `<docs_dir>/templates/` always excluded, independent of `api.exclude`.
6. `additional_docs` are outside `docs_dir`, path-addressed repo-relative;
   an entry under `docs_dir` is a validation error.

**DESIGN-0011 pre-names our three most likely bugs:** the tree filter must
widen from `docs_dir/<type.dir>/` to `docs_dir/` (and the push-diff
intersection with it); `additional_docs` still needs its own lookup outside
that widened filter; and `IsDoczFile` becomes the *discriminator* between
record types inside a type directory rather than a global keep-filter — with
each type dir's `README.md` being the file most likely to be wrongly dropped,
since it both fails `IsDoczFile` and lives inside a type directory.

**Two under-specified spots found while reading:**

- **`index.md` vs `README.md` precedence is not defined.** DESIGN-0011 rule 2
  names `index.md` only at the repo root and `README.md` for directories; the
  docz README widens it to "a directory's `index.md` or `README.md`" with no
  tiebreak for a directory containing both. docz-api has to pick a rule (and
  should push it upstream as a clarification).
- **Whether `.md` appears in the published path is left to the consumer.**
  DESIGN-0011's route table drops the extension; the docz README's table
  keeps it. docz supplies paths with extensions; stripping is presentation.

### Observation 2 — prior docz-api work: predicted, deliberately deferred, never designed

**No design exists.** Zero hits for `APIConfig` / `api:` block / R10 /
DESIGN-0011 / `v1.2.0` across this repo's docs, code, git log, and (empty)
issue tracker. The feature was, however, *predicted twice and deferred on
purpose*, which is exactly the trail the question asked about:

- **[INV-0003]** (Open, 2026-07-10) — "docz-site deferred features and the
  docz-api surface to unblock them" — is the discussion we half-remembered.
  Its F5 cluster covers "not ingested" raw files and concludes: *"Recommend
  an explicit non-goal until a concrete need exists; if it comes, it is its
  own design (likely a separate `repo_files` surface, not `documents`)."*
  The concrete need has now arrived. Its F1 (repo-home endpoint) shipped as
  DESIGN-0003; F2 (link graph), F3 (lifecycle dates), F4 (labels) remain
  open and are *not* addressed by `v1.2.0`.
- **IMPL-0005's OQ-1a design note** left the seam this feature lands in:
  *"the persisted-path-on-the-repo-row shape is expected to become a more
  generic consumed-files pattern later... one plain column now, no bespoke
  abstractions that would fight a future tracked-files generalization — and
  no premature generalization either."*
- **DESIGN-0003's Non-Goals** ("no arbitrary-file serving or ingest", "no
  type-dir index/README serving") are the two lines the `api:` block
  supersedes *for opted-in repos*. Both cite INV-0003's option-c rejection,
  which still stands (Observation 4) — the non-goal falls, the modeling
  argument behind it does not.

Upstream, docz issue [#81] (still Open) is the origin ask; it should be
closed against docz `v1.2.0` or against this work when it ships.

### Observation 3 — the pin bump crosses two default-changing releases

`v1.1.0 → v1.2.0` is not a plain additive bump; the release notes flag both.

**v1.1.1 flipped the built-in PLAN type to `enabled: false`.** This changes
`DefaultConfig()` and `EnabledTypes()` for any repo whose `.docz.yaml` omits
an explicit `types:` block. In docz-api that is not cosmetic: ingest's
desired state is built from `cfg.EnabledTypes()`, and the reconcile
**deletes** doc types absent from the desired set and treats their documents
as removed — so a fleet repo relying on the default-enabled PLAN type with
docs under `docs/plan/` would have those rows (and their search entries)
**deleted on its first ingest after the bump**. The docz repo itself declares
`types.plan.enabled: true` explicitly and is safe; the fleet needs a sweep
before the bump ships (checkable from our own DB: any repo with a `plan` doc
type whose `config_snapshot` lacks `types:`).

**v1.2.0 narrows `changelog.file` validation** — a path component ending in a
period (`.../CHANGELOG.md`) now fails `Validate` on an enabled block, which
in our pipeline fails that repo's whole ingest (IMPL-0005 OQ-2a: one error
path). No fleet repo plausibly uses such a path, but it is a
behavior change on an already-pinned surface, which is precisely what
`internal/doczcontract` exists to catch — the R1–R6 suite must run green on
the bumped pin before anything else is built.

Also inherited (template-level, not API): v1.1.1 changed generated H1s to the
prefix-qualified `# DESIGN-0011: title` form and dropped the bold
Status/Author/Date block from templates. Only relevant where we parse H1s —
which after this feature we will, via `docparse.Title`.

### Observation 4 — the documents table cannot carry pages; a second record type is required

Confirming INV-0003's instinct with current schema and code:

- `documents.doc_id` is `NOT NULL` and `UNIQUE (repo_id, doc_id)`; pages
  have no id. `title` is `NOT NULL`; pages have no frontmatter. `type` is
  `NOT NULL` and every read route resolves `{type}` against `doc_types`
  rows, which `reconcileDocTypes` actively deletes when absent from desired
  state — a synthetic type would fight the reconciler.
- Every `documents` row flows to Meilisearch unconditionally via
  `toIndexDoc`; mixing pages in changes list/search semantics everywhere at
  once rather than opt-in.
- docz DESIGN-0011's own data-model sketch agrees: a second record type
  keyed `(repo_id, path)` — fields `path`, `title` (`docparse.Title`,
  filename fallback), `content`, `content_hash` — ordered by path. Its
  Non-Goals say it outright: *"Path-addressed documents are not docz
  documents: no id, no type, no status, no ToC injection, no index-table
  row."*

So the shape is a new table (working name `repo_pages`; INV-0003 called it
`repo_files`) with the same content-hash gate discipline `documents` has,
plus the landing page staying on the repo row. The existing
`repos.index_md`/`index_sha` columns *are* the landing-page cache — today
they are hard-wired to `docs_dir/index.md`, which is exactly `v1.2.0`'s
default `landing_page`; under an enabled block the fetch follows
`cfg.API.LandingPage` instead. `GET .../index` keeps working unchanged for
non-opted-in repos by construction.

### Observation 5 — pipeline deltas are small and mostly relaxations

Mapped against the current code:

- **Fetch (`internal/githubapp`)**: `classifyTree` widens its keep-set from
  "docz files under type dirs" to "`*.md` under `docs_dir/`" when — and only
  when — the block is enabled. The hint pattern already exists twice
  (`docsDirHint`, `changelogHint`); an `apiHint` reading
  `api.enabled`/`landing_page`/`exclude`/`additional_docs` fetch-scoped is
  the same move, with the authoritative parse staying in ingest's
  `loadConfig`. `additional_docs` resolve via `findBlobSHA` against the
  already-listed recursive tree — one blob request per present file, zero
  network cost for absent ones, subpaths free by construction (the changelog
  precedent exactly). A dormant block must produce today's fetch
  byte-for-byte (docz Decision 4: no repo goes dark).
- **Ingest (`internal/ingest`)**: `buildDocuments`'s two hard filters
  (`IsDoczFile`, `ParseFrontmatter`) stay for docz documents; a sibling
  `buildPages` classifies the widened blob set per the six-clause rule —
  excludes, `templates/`, index-file consumption, the skip-and-report rule
  for stray files in type dirs. Titles: `docparse.Title(content)`, filename
  fallback (docz-api writes its own title-case helper; docz's is
  `internal/` — settled in docz INV-0007's Decision 1 amendment).
- **Reconcile (`internal/store`)**: a `reconcilePages` mirroring
  `reconcileDocuments` — content-hash gate, delete-absent, IDs surfaced for
  the search sync. The "opt-in is desired state, not a cache" rule carries
  over: a disabled/absent block reconciles an empty page set, wiping
  previously cached pages, same as the changelog triple nulls.
- **Webhook (`internal/webhook`)**: pushes touching `docs_dir/` already
  trigger re-ingest — pages under `docs_dir` need **no change**.
  `additional_docs` live outside it and need exact-path matches, the
  `changelog_file` problem again; `handlePush` already holds the repo row,
  so the question is only where the paths live at webhook time
  (`config_snapshot` is already on the row — OQ-4).
- **Search (`internal/search`)**: nothing indexes pages until we decide they
  join (OQ-3). Note the Meilisearch PK charset gotcha in advance: PKs allow
  only `[a-zA-Z0-9-_]`, and page paths contain `/` and `.` — the
  `<repo_id>_<doc_id>` scheme does not transfer; a page PK needs a hash or
  encoding.

### Observation 6 — the serve surface is new, and the path-shaped route has sharp edges

Nothing served today addresses by path. The natural additive shape, riding
the existing repo-scoped precedent (`.../index`, `.../changelog`):

- `GET /api/v1/repos/{owner}/{name}/pages` — the page list/tree (paths +
  titles; the site builds nav and its link-resolver whitelist from this).
- `GET /api/v1/repos/{owner}/{name}/pages/{path...}` — one page, raw
  markdown + sha, 404 for absent/excluded/not-opted-in.

Sharp edges to design for, not discover:

- **chi wildcard routing + URL decoding.** The `{path...}` param arrives
  URL-decoded. docz's validation explicitly judges raw bytes and warns that
  *"a consumer that decodes a config-sourced path before resolving it has
  voided this check and must re-validate afterwards."* The handler must
  re-validate the decoded path (clean, no `..`, no absolute) before any
  lookup — DB-keyed lookup by exact stored path makes this cheap, but the
  check still has to exist for defense in depth.
- **Spec shape.** OAS 3.1 has no native wildcard path params; the spec and
  the kin-openapi contract test need a workable encoding (single
  `{path}` segment with slashes percent-encoded, or a documented deviation).
  Spec bump is minor (`1.2.1 → 1.3.0`).
- **Route-space collisions.** `pages` becomes a reserved segment under the
  repo route the same way `changelog` is on the site. docz's validator
  already protects the *page* namespace from *type* collisions (reserved
  first-segment rule); the API's own static segments are our problem.

### Observation 7 — docz-site is further along than expected

The site is not waiting on a design of ours — it has already built the
consumption patterns this feature slots into:

- `getRepoIndex` and `getRepoChangelog` are **live** (`repo-home.tsx`,
  `repo-changelog.tsx`), rendered through the one sanitize-invariant
  markdown pipeline.
- The relative-link resolver (site DESIGN-0002, shipped) resolves
  author-written relative hrefs against a per-surface base path using a
  `byPath` whitelist built from API data — *"the emitted href always comes
  from API data"*. Pages join by adding their paths to that map; links to
  not-yet-served files already fail closed (stay byte-identical).
- Its route table reserves static segments over `:type` (the `changelog`
  precedent) and currently 404s anything deeper than
  `/:owner/:repo/:type/:docId` — the page-tree route family is site
  DESIGN-0002's named "future DESIGN pair with the `.docz.yaml` 'extras'
  contract".
- Site INV-0003 option (a″) — *"an 'extras' section in the docz contract...
  the config entry itself is the contract"* — is the direct ancestor of the
  `api:` block, and it comes with the site's acceptance bar (its Obs 5):
  pages that render but don't surface in the ⌘K palette *"would read as
  broken search, not as a hosting detail"*. That bar is the strongest
  argument that search inclusion is part of this feature's definition of
  done, even if phased (OQ-3).
- **Images are the known gap on both sides**: the site's resolver handles
  `<a>` only, its sanitize schema is deliberately narrow, and its INV-0003
  names "a raw asset endpoint" as genuinely new surface. docz Decision 10
  leaves assets to us if ever. Keep it out of scope here, stated.

### Observation 8 — inherited security posture and upstream loose ends

- **docz IMPL-0016 OQ-9 shipped unresolved: `docs_dir` itself is never
  path-validated.** `docs_dir: ../../etc` loads and validates clean in
  `v1.2.0`. docz contains it for the api-block comparisons via `path.Clean`;
  our exposure is bounded (git tree paths never start with `..`, so a
  traversal `docs_dir` simply matches nothing) but the design should state
  the assumption and clean/normalize `docs_dir` before prefix use, not
  inherit safety by accident.
- **Case and Unicode**: docz's dedupe folds ASCII case only (stdlib-only, no
  NFC/NFD normalization) and its uniqueness guarantees are case-*insensitive*
  while git is case-sensitive; serving by exact stored path keeps us
  honest, but the design should say whether path lookup is exact-byte
  (recommended) or folded.
- **Enabling publishes everything.** DESIGN-0011's rollout caution — a repo
  flipping `api.enabled` publishes every `.md` under `docs_dir`, scratch
  files included. Ours to surface in `deploy/README.md`/consumer docs, since
  our API is where it becomes visible.
- **Upstream Phase 4 leftovers** (docz IMPL-0016): the post-tag status flips
  (DESIGN-0011/IMPL-0016 still say Draft), the scratch-module verification
  of the published `v1.2.0`, and the release-notes extraction (the GitHub
  release shows goreleaser boilerplate; the real notes live only in PR #84's
  body). None blocks us — our `doczcontract` R10 clause *is* the scratch-
  module exercise, done properly — but worth reporting upstream.
- **Local drift noticed while investigating** (fix-alongside candidates):
  `CLAUDE.md` says the spec is "currently 1.1.0" (it is `1.2.1`);
  `api/README.md` says `1.2.0`; docz-site's vendored spec is `1.2.0`
  (tracked by docz-site #17).

## Conclusion

**Answer:** Prior work exists but is deliberately incomplete — and nothing
needs *re*doing.

1. **What we did before:** [INV-0003] predicted this exact feature and
   parked it as a non-goal pending "a concrete need"; IMPL-0005 left the
   generalization seam unbuilt on purpose; DESIGN-0003 wrote the non-goals
   that `v1.2.0` now lifts. There is no docz-api design for consuming the
   `api:` block, and no code, doc, or issue here references `v1.2.0` — the
   trail we remembered is real, and it all points forward, none of it stale.
2. **What consuming it requires:** a pin bump crossing two default-changing
   releases (with one real fleet risk: default-enabled PLAN repos losing
   plan docs on first post-bump ingest); a `doczcontract` clause R10 in the
   R6 mold; a fetch-hint + widened tree filter; a `buildPages` classifier
   implementing DESIGN-0011's six-clause rule; a new `(repo_id, path)`-keyed
   table with the standard content-hash/delete-absent reconcile; two new
   endpoints (list + get-by-path) with a re-validation rule at the decoded-
   path boundary; a minor spec bump; and a decision on search inclusion.
3. **DESIGN or straight to IMPL:** DESIGN. docz settled the *contract* but
   explicitly left fetch/walk/route/store to us, and the six decisions below
   are genuinely open with real trade-offs — the same test that sent the
   (smaller) index and changelog slices through design first.

## Recommendation

1. **Write DESIGN-0004** ("Consume the docz v1.2.0 api block: pages,
   landing page, and additional docs") from the answered decisions below,
   then an IMPL with the phasing we've converged on across IMPL-0003/0005:
   pin bump + contract clause first, vertical slice per surface after.
2. **Phase 1 of the eventual IMPL is mechanical and low-risk**: bump to
   `v1.2.0` via `go get` (never bare `go mod tidy`), run R1–R6, add clause
   **R10** (`internal/doczcontract/api_test.go` in the R6 mold — dormancy,
   normalization/backfill, enabled-only validation + `ErrInvalidAPIPath`,
   the `Title` cases including setext, frontmatter-skip, and
   frontmatter-`title:`-is-ignored), pinned before any runtime caller
   exists, exactly as R6 pinned `ParseChangelog`.
3. **Run the PLAN-flip fleet check before the bump ships** (Observation 3):
   query for repos carrying a `plan` doc type whose `config_snapshot` has no
   explicit `types:` block; fix those repos' `.docz.yaml` first or accept
   the deletion knowingly.
4. **Record the supersession in [INV-0003]**: its F5 non-goal is lifted by
   this INV; F2/F3/F4 (link graph, lifecycle dates, labels) remain its open
   clusters, untouched by `v1.2.0`.
5. **Upstream follow-ups to docz**: close issue [#81] against `v1.2.0`;
   report the Phase-4 leftovers (status flips, release-notes extraction);
   ask for the `index.md` vs `README.md` precedence ruling (Observation 1).
6. **Out of scope, stated now so the design says so too**: assets/images
   (docz Decision 10; genuinely new surface both here and on the site), the
   link graph, lifecycle dates, labels (INV-0003 F2–F4), and any PDF/export
   surface.

## Open Questions

Answer each with a letter — **a is the recommendation**, b onward are
alternatives; write in your own option if none fits. Decisions recorded here
feed DESIGN-0004 directly.

**All answered (2026-08-28): 1a, 2a, 3a, 4a, 5a (amended in review — see
its resolution), 6a.** Review raised one check on 5: the docz CLI backfills
`landing_page` to `<docs_dir>/index.md` when the block is enabled, and the
rule must match that expectation. It does at the repo root by construction —
the landing page *is* the root page (3a) — and the 5a amendment extends the
same courtesy to lone `index.md` files in directories.

### 1. Do pages join search, and when?

docz-site's acceptance bar (its INV-0003 Obs 5) is that every rendered page
is findable from the ⌘K palette — pages that render but don't search *"would
read as broken search, not as a hosting detail."* But pages break two search
assumptions: `SearchHit` requires `doc_id`/`type`/`status`/`author` (pages
have none), and the index PK scheme `<repo_id>_<doc_id>` cannot encode paths
(the PK charset is `[a-zA-Z0-9-_]`).

- **a. (Recommendation) Same `documents` index with a `source` facet
  (`doc`/`page`), landed as a late phase of the same IMPL.** The first
  vertical slice ships fetch→store→serve without search, but the IMPL is not
  done until pages search. One palette, one front door (the site's bar); the
  facet keeps doc-only filtering intact; the PK scheme changes once (hashed
  or encoded) for both record kinds. `SearchHit` grows a `source` field with
  relaxed requireds — an additive spec bump.
- b. Search in the very first slice. Cleanest end state, but it front-loads
  the wire-contract change and doubles the first PR's surface.
- c. A second Meilisearch index for pages. Isolated schemas, but two queries
  per palette search, split facet counts, and the multi-search coordination
  lands on the site.
- d. Defer search to its own design after pages ship. Violates the site's
  acceptance bar; the rendered-but-unfindable window becomes open-ended.

### 2. Serve shape for path-addressed pages?

Page paths contain `/` and `.`; OpenAPI 3.1 has no native multi-segment path
parameter, and the kin-openapi/gorillamux contract test must be able to
exercise whatever shape we pick.

- **a. (Recommendation) `GET .../pages` (flat list: path, title, sha) plus
  `GET .../pages/{path}` with `{path}` matched by a chi wildcard.** The flat
  list is exactly what the site's `byPath` resolver map wants (the site
  builds its own tree); the page route follows GitHub's own contents-API
  precedent (`/repos/{owner}/{repo}/contents/{path}`). Spec the param
  `style: simple` with a description stating slashes are allowed; the
  contract test exercises the percent-encoded spelling, which routes
  identically (chi matches the decoded path). The handler re-validates the
  decoded path (clean, no `..`) before lookup, per docz's warning that
  decoding a path voids its config-time validation.
- b. Query parameter: `GET .../page?path=…`. Trivially spec-able, no routing
  subtlety — at the cost of an asymmetric surface (`/index`, `/changelog`,
  but `?path=` for pages) and a second, singular route name.
- c. One tree endpoint returning every page with its body. One round trip,
  but unbounded response size and no per-page caching off the sha.

### 3. Landing page: generalize the index columns or add new ones?

`repos.index_md`/`index_sha` already cache `docs_dir/index.md` — which is
exactly `v1.2.0`'s *default* `landing_page`. An enabled block can point it
anywhere repo-relative (outside templates/excludes).

- **a. (Recommendation) Reuse the columns; the fetch follows
  `cfg.API.LandingPage` when the block is enabled** and stays hard-wired to
  `docs_dir/index.md` otherwise. `GET .../index` keeps serving "the repo
  home" — which is what it always meant — and non-opted-in repos are
  byte-for-byte unchanged. The spec change is a description edit, not a
  shape change.
- b. New `landing_md`/`landing_sha` columns and a new endpoint, deprecating
  `/index`. Cleaner naming; pointless migration and a breaking journey for
  the one consumer.
- c. Model the landing page as a row in the pages table flagged as landing.
  One storage shape, but it breaks the "pages live under `docs_dir` or
  `additional_docs`" invariant and complicates the repo-row read the site
  performs first.

### 4. How does the webhook match additional_docs pushes?

`additional_docs` live outside `docs_dir`, so `shouldIngest`'s prefix check
never sees them — the changelog problem, pluralized. `handlePush` already
holds the full repo row.

- **a. (Recommendation) Persist the normalized list on the repo row (JSONB
  `api_additional_docs`, written by the reconcile from the post-`Load`
  config).** The plural of the `changelog_file` column precedent, chosen
  then for the same reason: the webhook matches against what the last ingest
  actually resolved — normalized and validated — not raw YAML it would have
  to re-parse. One exact-set membership check per push.
- b. Parse `config_snapshot` in `handlePush`. Zero migration, but it
  re-derives desired state from raw config on the webhook path and
  duplicates normalization assumptions the reconcile already owns.
- c. Enqueue on any default-branch push when `api.enabled`. The simplest
  rule, and the debounce plus content-hash gate absorb the redundancy — but
  every push then costs a full GitHub tree fetch.

### 5. index.md vs README.md when a directory has both?

docz left this unspecified: DESIGN-0011's normative rule names `README.md`
as a directory's page (`index.md` appears only as the repo-root landing
default), while the docz README's paraphrase says "`index.md` or
`README.md`" with no tiebreak.

- **a. (Recommendation) Follow the normative text: `README.md` is the
  directory page; a non-root `index.md` is an ordinary path-addressed
  page.** Self-consistent with DESIGN-0011 rule 2 (the docz-generated
  type-dir `README.md` tables *are* the type pages), no information is ever
  dropped, and we ask upstream to align the README wording rather than
  invent a local precedence.
- b. `index.md` wins where both exist (the MkDocs instinct, matching the
  root convention); `README.md` becomes path-addressed. Defensible, but it
  contradicts the one normative sentence that exists.
- c. Block the feature on an upstream ruling. Safest and slowest; the
  ambiguity only bites directories carrying both, which we can log when
  seen.

**Resolution (a, amended 2026-08-28):** review pointed at the CLI's
`<docs_dir>/index.md` landing default as the expectation to match, which
exposed a gap in a's strict reading — docz's public README promises *"a
directory's `index.md` or `README.md` is that directory's page"*, so a lone
`index.md` must serve as the directory page, not be path-addressed. The full
precedence:

1. The repo root is always the landing page (`cfg.API.LandingPage`,
   defaulting to `<docs_dir>/index.md` via the CLI backfill) — Q3a's
   territory, untouched here.
2. A non-root directory's page is its `README.md`; when there is no
   `README.md`, its `index.md`.
3. When a directory has **both**, `README.md` wins (DESIGN-0011's normative
   rule 2 — in type dirs the docz-generated table *is* the type page) and
   the `index.md` is served path-addressed, so nothing is dropped.

Upstream ask (folds into Recommendation 5): ratify this precedence and align
the docz README / DESIGN-0011 wording, which currently disagree.

### 6. Does the published path keep the .md extension?

- **a. (Recommendation) Yes — pages are addressed by their exact repo
  paths, extension included.** Matches the site's `byPath` map (keyed on
  real paths), the docz README's URL table, and exact-byte serving with no
  canonicalization surface. DESIGN-0011 explicitly leaves
  extension-stripping to the site as presentation.
- b. Strip `.md` in the API. Prettier URLs at the cost of a collision rule
  (`a.md` vs a directory `a`) and a second path spelling to validate
  everywhere.

## References

- docz PR [#84] — `feat: v1.2.0 — api config block and docparse.Title
  (IMPL-0016)` (merged 2026-08-26; the release notes live in its body)
- docz `v1.2.0` release —
  <https://github.com/donaldgifford/docz/releases/tag/v1.2.0>
- docz DESIGN-0011 — api config block (the consumption rule; Draft upstream,
  pending status flip)
- docz DESIGN-0008 — clause **R10**, the consumer contract this repo pins
- docz IMPL-0016 — phases 1–3 shipped; Phase 4 leftovers per Observation 8;
  OQ-9 (`docs_dir` unvalidated) open upstream
- docz INV-0007 — "docz internals required for the api additional-docs
  block" (the grounding investigation; F2/F4/F5 are the consumer findings)
- docz issue [#81] — the origin feature ask (Open)
- [INV-0003] — docz-site deferred features and the docz-api surface to
  unblock them (Open; F5 superseded by this INV)
- [INV-0005] / IMPL-0005 — the changelog slice (structural precedent:
  opt-in fetch hint, persisted path, desired-state nulling)
- DESIGN-0003 / IMPL-0003 — the repo-index slice (structural precedent:
  cached repo-row file, presence keyed off sha)
- docz-site INV-0003 — option (a″) "extras section" (the feature's site-side
  ancestor) and Obs 5 (the search acceptance bar)
- docz-site DESIGN-0002 — relative-link resolution (the `byPath` whitelist
  pages will join)

[#84]: https://github.com/donaldgifford/docz/pull/84
[#81]: https://github.com/donaldgifford/docz/issues/81
[INV-0003]: 0003-docz-site-deferred-features-and-the-docz-api-surface-to-unblock.md
[INV-0005]: 0005-changelog-as-a-first-class-docz-artifact.md
