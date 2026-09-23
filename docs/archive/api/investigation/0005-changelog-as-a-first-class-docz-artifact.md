---
id: INV-0005
title: "Changelog as a first-class docz artifact"
status: Concluded
author: Donald Gifford
created: 2026-08-02
---

<!-- markdownlint-disable-file MD025 MD041 -->

# INV 0005: Changelog as a first-class docz artifact

**Status:** Concluded **Author:** Donald Gifford **Date:** 2026-08-02

<!--toc:start-->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
  - [F1: docz-api already fetches and caches the changelog — it just never serves it](#f1-docz-api-already-fetches-and-caches-the-changelog--it-just-never-serves-it)
  - [F2: docz v1.0.0 silently ignores an unknown changelog: key — rollout is additive-safe](#f2-docz-v100-silently-ignores-an-unknown-changelog-key--rollout-is-additive-safe)
  - [F3: A changelog is not doc-type-shaped — three modeling options](#f3-a-changelog-is-not-doc-type-shaped--three-modeling-options)
  - [F4: The git-cliff-everywhere convention makes structured parsing tractable — upstream](#f4-the-git-cliff-everywhere-convention-makes-structured-parsing-tractable--upstream)
  - [F5: Feature 2 (per-doc backlinks) cannot be built from the markdown alone](#f5-feature-2-per-doc-backlinks-cannot-be-built-from-the-markdown-alone)
  - [F6: docz v1.1.0 ships the full surface — verified](#f6-docz-v110-ships-the-full-surface--verified)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
  - [Review decisions (2026-08-02)](#review-decisions-2026-08-02)
  - [Proposed phasing](#proposed-phasing)
  - [Open questions (answer inline)](#open-questions-answer-inline)
- [References](#references)
<!--toc:end-->

## Question

docz is gaining an opt-in `changelog:` config block:

```yaml
changelog:
  enabled: true # enable changelog mapping of docz files.
  file: CHANGELOG.md # name of the changelog file. Defaults to CHANGELOG.md
```

Two features want to ride on it:

1. **Feature 1 (this INV's focus):** the changelog is mapped in `.docz.yaml` and
   consumed by docz-api like the other docz artifacts — config-driven fetch,
   cache, and a served read surface for the docz-site.
2. **Feature 2 (scoped here, designed later):** on a docz-site document page,
   show the changelog sections in which that specific file was added/modified —
   changelog-to-document backlinks.

For feature 1: **what is the right modeling and the cheapest correct
implementation**, given the fleet-wide conventions (git-cliff with a shared
config, conventional commits, SemVer)? And what must feature 1 get right so
feature 2 doesn't require rework?

## Hypothesis

Three expectations going in:

1. **Feature 1 is mostly plumbing, not new machinery.** The ingest pipeline
   already fetches an optional root `CHANGELOG.md` and caches it on the repo row
   (`repos.changelog_md`/`changelog_sha` — the exact precedent DESIGN-0003
   copied for `index.md`). The delta is: honor the config instead of hardcoding,
   and add one read endpoint mirroring `getRepoIndex`.
2. **"Like the other docz types" should not mean a `doc_types` row.** A
   changelog has no frontmatter, no ID, no status lifecycle, and is one
   generated file rather than a directory of authored files — every assumption
   the doc-type pipeline makes. It is repo-level metadata, like `index.md`.
3. **Feature 2 is a separate design with a data dependency feature 1 cannot
   satisfy.** Mapping changelog sections to files needs commit→file data that
   the rendered markdown does not contain.

## Context

- The user sketched the `changelog:` block into this repo's own `.docz.yaml`
  (committed on this branch as the worked example). The feature is **disabled by
  default** upstream.
- Fleet conventions make the changelog format uniform: **git-cliff** everywhere
  with essentially the same `cliff.toml`, **conventional commits**, **SemVer**
  tags. Every changelog docz-api will ever consume looks like this repo's own
  `CHANGELOG.md`.
- INV-0003 (docz-site deferred features) already surveyed the adjacent surface:
  F1 there (repo home from `index.md`) shipped as DESIGN-0003 / IMPL-0003 and
  established the repo-level-artifact pattern end to end (fetch → cache on repo
  row → `GET /api/v1/repos/{owner}/{name}/index` → spec minor bump). INV-0003's
  F3 (lifecycle rail) is a cousin of feature 2 — both want per-document git
  history docz-api does not currently hold.

**Triggered by:** upstream docz `changelog:` config proposal; docz-site
changelog page + per-doc backlink wishlist.

## Approach

1. Recon the current changelog handling in docz-api (fetch, store, serve).
2. Probe whether docz v1.0.0 tolerates the unknown `changelog:` config key —
   this decides rollout ordering (can repos add the block before the
   libraries/API understand it?).
3. Inspect the fleet's actual git-cliff output shape to assess structured
   parsing and what feature 2 can (and cannot) extract from it.
4. Enumerate modeling options for feature 1 and pick one; scope feature 2's real
   dependency.

## Environment

| Component                         | Version / Value                               |
| --------------------------------- | --------------------------------------------- |
| docz (library + CLI)              | v1.0.0 (pinned, plain require)                |
| docz-api                          | `main` @ this branch's parent                 |
| OpenAPI spec (`api/openapi.yaml`) | 1.1.0                                         |
| Chart                             | 0.2.2                                         |
| git-cliff config                  | repo-root `cliff.toml` (fleet-standard shape) |

## Findings

### F1: docz-api already fetches and caches the changelog — it just never serves it

Verified current state:

- `internal/githubapp/client.go:31` hardcodes `changelogFile = "CHANGELOG.md"`
  (repo root). `classifyTree` picks its blob sha out of the recursive tree and
  `Fetch` downloads it opportunistically — absent file is fine, no config
  involvement.
- `store.ReconcileRepo` writes it through to `repos.changelog_md` /
  `repos.changelog_sha`. The initial schema comment is explicit: "caches the raw
  root CHANGELOG.md (OQ 10), **not parsed**".
- `internal/httpapi` has **zero** changelog references. No endpoint, not in the
  OpenAPI spec, invisible to the docz-site.

So feature 1's true delta is small: (a) make the fetch config-driven
(`enabled` + `file`), (b) serve what is already cached. The `index.md` feature
(DESIGN-0003/IMPL-0003) is a completed dress rehearsal of exactly this shape —
including the presence-gate-on-sha trick for empty-but-present files.

### F2: docz v1.0.0 silently ignores an unknown `changelog:` key — rollout is additive-safe

Probe: with the `changelog:` block present in this repo's `.docz.yaml`,
`docz config` (CLI v1.0.0, same library ingest pins) resolves the full config
and exits 0 — the unknown key is silently dropped (yaml.v3 without
`KnownFields`).

Consequences:

- **Repos can add the block today** without breaking `docz` CLI usage or
  docz-api ingest. No flag-day ordering between docz, docz-api, and repo
  configs. The block is simply dormant until the libraries understand it.
- The inverse also holds: docz-api **cannot detect** the block through
  `doczcfg.Load` until docz ships the field. Feature 1 therefore starts with an
  upstream docz release, and docz-api's pin bump rides the established INV-0001
  procedure (bump + re-run `internal/doczcontract`; the new field should get a
  contract clause — see OQ-7).

### F3: A changelog is not doc-type-shaped — three modeling options

"Consume just like the other docz types" cannot be literal. Every doc-type
assumption fails for a changelog: `ParseFrontmatter` (none), `fm.ID` (none),
status lifecycle (none), one-directory-many-authored-files (one generated file),
`IsDoczFile` naming (deliberately excludes it). Three options:

- **Option A — repo-level artifact, raw (the `index.md` precedent).**
  Config-driven fetch → existing `changelog_md`/`changelog_sha` columns → new
  `GET /api/v1/repos/{owner}/{name}/changelog` returning
  `{repo, changelog_md, changelog_sha}`, 404 when absent. The site renders the
  markdown. Cost: small — the storage and fetch already exist; the endpoint is
  `getRepoIndex` with the nouns changed. No migration.
- **Option B — structured sections.** Parse the uniform git-cliff shape
  (`## [x.y.z] - date` headers, `### Group` subsections, bullet items) into
  version records — either a `changelog_entries` table or parse-on-serve — and
  return JSON like `{versions: [{version, date, groups: [{title, items_md}]}]}`.
  Unlocks per-version anchors/deep-links (which feature 2 wants) and lets the
  site build version pickers without client-side parsing. Cost: a parser (see
  F4), a schema/DTO decision, spec work.
- **Option C — model as a pseudo doc-type** (one `documents` row per version,
  `doc_id` = version). Rejected: fights the frontmatter pipeline end to end,
  pollutes search facets with meaningless type/status values, and turns
  desired-state reconcile into a special case. Nothing wants this shape.

A is strictly a subset of B's plumbing (same fetch, same config), so shipping A
first wastes nothing if B follows.

### F4: The git-cliff-everywhere convention makes structured parsing tractable — upstream

This repo's `CHANGELOG.md` (representative of the fleet by convention):

```markdown
## [0.4.2] - 2026-07-23

### Bug Fixes

- _(ci)_ Drop stale goreleaser GPG signing of archives ([#10](…))
```

Uniform `## [semver] - YYYY-MM-DD` version headers (plus `## [unreleased]`),
`### Group` subsections from the shared `cliff.toml` groups, one bullet per
commit with optional `*(scope)*` and PR links. A parser over this shape is a
~100-line, fixture-testable job — **and it belongs in the docz library, not
docz-api**: the config block lives there, the docz-site could reuse it, and
`internal/doczcontract` would pin its behavior exactly like `ParseFrontmatter`'s
(byte-based, no disk). Precedent: doc parsing already lives upstream
(`doczdoc`); docz-api only maps results to store inputs.

### F5: Feature 2 (per-doc backlinks) cannot be built from the markdown alone

The rendered bullets carry scope + message + PR number — **no commit SHAs and no
file paths**. "Which sections touched `docs/design/0001-*.md`" therefore needs
commit→file data from outside the changelog:

- **(a) GitHub compare API at ingest** — for each release tag pair (`vN-1..vN`),
  list commits + files, invert to `path → [versions]`. Append-only and cacheable
  (released sections never change), but a per-release API cost on first ingest
  and a new store surface.
- **(b) `git-cliff --context` artifact** — repos' release CI emits the context
  JSON (which has commit ids + can carry file lists) as a committed or attached
  artifact docz-api fetches. Zero GitHub API cost at ingest, but every repo's
  release workflow must change — a fleet-wide rollout. **Rejected in review**
  (see Review decisions).
- **(c) PR-number joins** — resolve the `(#N)` links to PRs → commits → files.
  Same API cost as (a) with more moving parts; only works for squash-merge PR
  flows (which the fleet does use).

All three are real designs with store schema, backfill, and rate-limit questions
— confirming feature 2 is **its own DESIGN doc**, not a rider on feature 1. What
feature 1 must do so feature 2 slots in without rework: keep the version
identity stable (SemVer string as the section key), which Option B's parsed
shape provides and Option A at minimum must not obscure.

### F6: docz v1.1.0 ships the full surface — verified

_Addendum 2026-08-03._ docz **v1.1.0** (released 2026-08-03) implements the
handoff design — upstream it is **DESIGN-0010** (the ported
`TEMP-DESIGN-docz-changelog.md`, now deleted here per its lifecycle note).
Verified two ways:

**Source review (tag `v1.1.0`):**

- `pkg/doczcore/config`: `ChangelogConfig{Enabled bool, File string}` with yaml
  tags `enabled`/`file`; `Config.Changelog` under yaml key `changelog`;
  `DefaultConfig()` returns `{false, DefaultChangelogFile}` with
  `DefaultChangelogFile = "CHANGELOG.md"`. Partial-block merge falls out of
  decode-in-place over `DefaultConfig()`; `normalizeChangelog` backfills an
  explicit empty `file:` and strips `./` prefixes. Validation runs **only when
  enabled** (dormant block stays unvalidated) and is stricter than specced:
  rejects absolute paths, `..` traversal, trailing slash, backslashes, Windows
  drive prefixes, control characters, `~` prefixes, and unclean segments.
- `pkg/doczcore/document`: `ParseChangelog([]byte) (*Changelog, error)` with
  `Changelog{Preamble, Versions}` /
  `ChangelogVersion{Version, Unreleased, Date, Groups}` /
  `ChangelogGroup{Title, Items}` and the `ErrNoVersions` sentinel — the spec
  exactly, plus beyond-spec hardening: fenced-code-block awareness, CRLF
  tolerance, `v`-trim only when a digit follows (`vnext` untouched), em/en-dash
  date separators, duplicate headings preserved in document order. Items strip
  the bullet marker (re-prefix with `-` and a space to render); column-0 prose
  between bullets is discarded (documented as not-lossless). Upstream testdata
  fixtures (`fleet`/`chart`/`edge`/`nopreamble`/`noversions`) mirror this INV's
  testing strategy, including the unknown-sibling-key tolerance regression.
- All three implementer OQs resolved as this INV leaned: **OQ-A** parser in the
  `document` package (docz-api's existing `doczdoc` import — one import, no path
  changes); **OQ-B** `Date` stays a string; **OQ-C** preamble verbatim.

**Empirical probe** (scratch module pinning `v1.1.0`, run 2026-08-03):

```text
1. DefaultConfig().Changelog = {Enabled:false File:CHANGELOG.md}
2. Load(partial block + unknown sibling) err=<nil>
   Changelog={Enabled:true File:CHANGELOG.md}
3. ParseChangelog(docz-api CHANGELOG.md): preamble=222B versions=7
   [0] ver="unreleased" unreleased=true date="" groups=3
   [1] ver="0.4.2" date="2026-07-23" groups=2 (items keep *(scope)* + PR links)
4. errors.Is(err, ErrNoVersions) = true
```

Every R6 contract point holds against the released module. **v1.1.0 is
sufficient — feature 1 is unblocked** (pin bump v1.0.0 → v1.1.0 via the INV-0001
procedure, add the R6 clause, then the fetch + endpoint work).

## Conclusion

**Answer: Yes — feature 1 is small and the shape is clear.**

- Model the changelog as a **repo-level artifact** (Option A → B), not a doc
  type (Option C rejected). The `index.md` feature already proved the whole
  pattern.
- The config block ships **upstream in docz first**; v1.0.0's tolerant parsing
  means repos can adopt the block immediately with zero breakage, and docz-api
  activates it on its next pin bump.
- The structured parser (Option B), when wanted, belongs **in the docz library**
  next to `ParseFrontmatter`, contract-guarded.
- Feature 2 is confirmed as a **separate design** — its blocker is commit→file
  data, not changelog parsing.

## Recommendation

### Review decisions (2026-08-02)

Reviewed; decisions recorded:

- **Direction confirmed (F1/F2):** the docz-repo work happens first — the point
  of investigating here was to derive the requirements docz-api hands to the
  docz repo.
- **Modeling confirmed (F3):** the changelog is not a doc type. It is an
  `index.md`-like repo-level artifact and stays that way.
- **The parser lives in the docz library (OQ-2 = a).** The alternative — serve
  the raw git-cliff file rendered like `index.md`, plus an optional config key
  pointing at a committed `git-cliff --context` artifact for parsing — was
  weighed and rejected: the context artifact is a half measure, and rolling CI
  changes across every fleet repo is strictly more expensive than shipping a
  docz version and bumping the pin.
- **Feature 2's data source narrows to F5 (a) or (c)** (compare API or PR-number
  joins); (b) falls with the context artifact.
- **All open questions answered `a` (2026-08-02):** OQ-1a raw markdown serve,
  OQ-2a parser in docz, OQ-3a strict `enabled` semantics, OQ-4a
  repo-root-relative subpaths (see the convention note on OQ-4), OQ-5a no Meili
  indexing, OQ-6a dedicated endpoint + spec 1.2.0, OQ-7a full contract clause.
  The INV is **Concluded**; the docz-repo work is handed off via the temporary
  design doc at `docs/TEMP-DESIGN-docz-changelog.md` (to be re-created with
  `docz create design` in the docz repo and deleted here).
- **Upstream shipped and verified (2026-08-03):** docz v1.1.0 implements the
  handoff as its DESIGN-0010 — source-reviewed and probe-verified in F6; the
  temporary design doc is deleted. Feature 1 is unblocked and planned as
  **IMPL-0005** (five phases: pin+R6, persistence, fetch+webhook, endpoint at
  spec 1.2.0, e2e + close-out).

### Proposed phasing

1. **docz (upstream):** add `Changelog{Enabled bool, File string}` to the config
   (default disabled, `File` default `CHANGELOG.md`), release. Optionally in the
   same or a later release: `ParseChangelog([]byte)` returning version sections
   (Option B's parser).
2. **docz-api (feature 1, one small DESIGN or direct IMPL):** bump the docz pin
   (INV-0001 procedure, contract clause for the new field), make
   `githubapp.Fetch` honor `enabled`/`file` (a `changelogHint` twin of
   `docsDirHint` keeps it fetch-scoped), add
   `GET /api/v1/repos/{owner}/{name}/changelog` mirroring `getRepoIndex`, spec
   1.1.0 → 1.2.0, natural-refresh rollout (DESIGN-0003 OQ-4a precedent).
3. **docz-site:** render the changelog page from the raw markdown.
4. **Feature 2:** new INV/DESIGN once feature 1 is serving — pick the
   commit→file data source (F5 a or c; b rejected in review) with real
   rate-limit numbers.

### Open questions (answer inline)

- **OQ-1 — serve shape for feature 1:** **(a)** raw markdown only
  (`{repo, changelog_md, changelog_sha}`, the `index.md` twin) — _recommended_;
  **(b)** raw + parsed sections in one response; **(c)** parsed only.
  **Answered: (a).**
- **OQ-2 — parser home (whenever structured parsing lands):** **(a)** docz
  library (`doczdoc`-style, contract-guarded) — _recommended_; **(b)** docz-api
  internal package; **(c)** docz-site client-side. **Answered: (a)** — parser in
  the docz library; the git-cliff context-artifact alternative is rejected
  (fleet-wide CI rollout costs more than a docz release + pin bump; see Review
  decisions).
- **OQ-3 — `enabled: false` (and absent block) semantics in docz-api:** **(a)**
  honor strictly: skip the fetch and null the cached `changelog_md`/`sha` on
  next reconcile (desired-state; serving 404s) — _recommended, but note_: repos
  that never add the block stop caching the changelog. Nothing serves it today,
  so there is no user-visible regression — but it makes the endpoint opt-in per
  repo from day one; **(b)** skip the fetch but keep the last cached value
  (stale forever); **(c)** keep today's unconditional fetch; config only gates
  whether the endpoint serves it. **Answered: (a).**
- **OQ-4 — `changelog.file` path semantics:** **(a)** repo-root-relative,
  subpaths allowed (`docs/CHANGELOG.md` works) — _recommended, matches
  git-cliff's repo-root convention_; **(b)** bare filename at repo root only;
  **(c)** `docs_dir`-relative. **Answered: (a)**, with the fleet convention
  noted: the value is either `CHANGELOG.md` (repo root) or
  `charts/<chart-name>/CHANGELOG.md` (helm-chart changelogs). Subpath support
  exists precisely for the chart shape; no other layouts are in use.
- **OQ-5 — Meilisearch:** index the changelog body? **(a)** no — repo-level
  artifact, not a searchable doc — _recommended_; **(b)** yes, as a
  pseudo-document per repo. **Answered: (a).**
- **OQ-6 — wire shape:** **(a)** new
  `GET /api/v1/repos/{owner}/{name}/changelog` endpoint, spec minor bump to
  1.2.0 — _recommended_; **(b)** fold `changelog_md` into the existing repo
  detail/list DTOs (bloats list payloads; rejected by the
  `raw_md`-is-detail-only precedent). **Answered: (a).**
- **OQ-7 — contract surface:** when the docz pin bumps, add a doczcontract
  clause (R6) freezing `Config.Changelog` defaults + `ParseChangelog` behavior
  (if shipped)? **(a)** yes, both — _recommended_; **(b)** config field only.
  **Answered: (a).**

## References

- INV-0001 — docz v1.0.0 pin bump procedure (contract-guarded surface)
- INV-0003 — docz-site deferred features (F3 lifecycle rail is feature 2's
  cousin; F1 index.md is feature 1's template)
- DESIGN-0003 / IMPL-0003 — repo index endpoint (the repo-level-artifact
  precedent this INV recommends copying)
- `internal/githubapp/client.go` — hardcoded `changelogFile` fetch
- `internal/store/migrations/20260702000000_initial_schema.sql` —
  `changelog_md`/`changelog_sha` columns ("NOT parsed")
- `cliff.toml` + `CHANGELOG.md` — the fleet-standard git-cliff shape
- `.docz.yaml` (this branch) — the proposed `changelog:` block, live example
- docz v1.1.0 (released 2026-08-03) — ships the surface as docz DESIGN-0010:
  `pkg/doczcore/config` `ChangelogConfig` + `pkg/doczcore/document`
  `ParseChangelog`/`ErrNoVersions`
  (<https://github.com/donaldgifford/docz/releases/tag/v1.1.0>)
