---
id: DESIGN-0011
title: "api config block: index page and additional_docs for docz-api and docz-site"
status: Draft
author: Donald Gifford
created: 2026-08-10
---

<!-- markdownlint-disable-file MD025 MD041 -->

# DESIGN-0011: api config block: index page and additional_docs for docz-api and docz-site

<!--toc:start-->
- [Overview](#overview)
- [Goals and Non-Goals](#goals-and-non-goals)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Background](#background)
- [Detailed Design](#detailed-design)
  - [The api block](#the-api-block)
  - [The consumption rule](#the-consumption-rule)
  - [Config types](#config-types)
  - [Normalization](#normalization)
  - [Validation](#validation)
  - [Titles for frontmatter-less documents](#titles-for-frontmatter-less-documents)
  - [Route mapping (non-normative)](#route-mapping-non-normative)
- [API / Interface Changes](#api--interface-changes)
  - [Downstream contract (what docz-api pins as R10)](#downstream-contract-what-docz-api-pins-as-r10)
- [Data Model](#data-model)
- [Testing Strategy](#testing-strategy)
- [Migration / Rollout Plan](#migration--rollout-plan)
- [Open Questions](#open-questions)
  - [10. Should images and other assets be consumed?](#10-should-images-and-other-assets-be-consumed)
- [Decisions](#decisions)
  - [Revision history](#revision-history)
- [References](#references)
<!--toc:end-->

## Overview

Add an opt-in `api:` block to `.docz.yaml` that declares what docz-api ingests
and docz-site renders beyond the docz documents themselves: a **landing page**
for the repo, an **exclusion list**, and **`additional_docs`** — markdown that
lives outside `docs_dir` entirely, like `CONTRIBUTING.md`.

The governing rule is that **the URL path mirrors the `docs_dir` path**. Every
`.md` under `docs_dir` is consumable: a directory's `README.md` is that
directory's page, documents inside a type directory keep their id-addressed
route, and everything else is addressed at its `docs_dir`-relative path.
`additional_docs` is the escape hatch for the files that convention places
outside `docs_dir`.

The block is inert to the docz CLI: it changes no command's behavior. Its job
is to be a **validated, semver-governed declaration** that consumers read.
INV-0007 audited what this costs docz internally: the config half is routine,
but the feature also needs one genuine addition to the frozen public surface —
a bytes-in title extractor for documents that have no frontmatter.

## Goals and Non-Goals

### Goals

- Make everything a repo puts under `docs_dir` reachable on docz-site, without
  the repo enumerating it.
- Let a repo surface markdown that lives outside `docs_dir` by convention
  (`CONTRIBUTING.md`, `DEVELOPMENT.md`).
- Let a repo declare its landing page and exclude what should not be published.
- Validate the declared paths at load time with the same rigor as
  `changelog.file` — these are paths a consumer fetches from a git tree.
- Preserve the **dormancy guarantee** (DESIGN-0010 Decision 7): a repo can add
  the block before any consumer understands it, and nothing breaks.
- Give consumers a supported way to derive a display title for a document with
  no frontmatter, so the H1 rule is defined once in docz rather than
  reimplemented per consumer.

### Non-Goals

- **No CLI behavior change.** `docz create`/`update`/`list`/`wiki` ignore the
  block entirely. It is declaration, not instruction.
- **No route construction in docz.** docz stores and validates configuration;
  building `https://docz.fartlab.dev/...` is docz-site's concern (ADR-0001:
  facts, not interpretation).
- **No fetching, globbing, or filesystem walking** in `pkg/doczcore`. Consumers
  resolve paths against whatever tree they have.
- **No new document model.** Path-addressed documents are not docz documents:
  no id, no type, no status, no ToC injection, no index-table row.
- **No asset pipeline.** Images and other binaries are out of scope for this
  design (Decision 10).
- **Not a replacement for `wiki:`.** MkDocs nav generation stays as it is.

## Background

Three things converge here.

**docz-api ingests without a checkout** (DESIGN-0008 Decision 1). It calls the
GitHub Git Trees API with `recursive=1`, filters to `.docz.yaml` plus blobs
under `docs_dir/<type.dir>/`, fetches those blobs as bytes, and parses them
with `doczcfg.Load` + `doczdoc.ParseFrontmatter`. Every file it knows about
today is inside a type directory and has frontmatter. `docs/examples/README.md`
is outside that filter — `examples` is not a type dir — and `CONTRIBUTING.md` is
not even under `docs_dir` (INV-0007 F5).

**docz-site already mirrors the docz tree** (DESIGN-0009):

```text
/:owner/:repo                   Repo detail
/:owner/:repo/:type             List of documents of one type
/:owner/:repo/:type/:docId      Single document reader
```

That is the same shape as `docs/`, `docs/rfc/`, `docs/rfc/0001-x.md`. This
design makes that correspondence the explicit rule rather than a coincidence,
and extends it to directories under `docs_dir` that are not types.

**`changelog:` is the precedent.** DESIGN-0010 / IMPL-0015 added an opt-in
block carrying a repo-relative path, normalized it on both `Load` paths,
validated it only when enabled, and emitted it default-off from `docz init`.
`additional_docs` is that same thing pluralized. The precedent also supplies
the path-hardening rules — control characters, backslashes, absolute paths,
Windows volume names, `~`, trailing `/`, `..` and non-canonical segments —
which were expensive to get right and must not be reinvented (INV-0007 F3).

## Detailed Design

### The api block

```yaml
api:
  enabled: true

  # The repo's landing page. Optional; defaults to <docs_dir>/index.md.
  landing_page: "docs/index.md"

  # Path prefixes under docs_dir that are never published.
  # docs_dir/templates/ is always excluded and need not be listed.
  exclude:
    - scratch

  # Markdown OUTSIDE docs_dir. Anything inside docs_dir is already consumed.
  additional_docs:
    - "CONTRIBUTING.md"
    - "DEVELOPMENT.md"
```

Note what is *absent*: `docs/examples/README.md` no longer needs listing. It is
under `docs_dir`, so it is consumed automatically and serves as the page for
`examples/`.

Leading `./` is accepted and stripped during normalization, matching
`normalizeChangelog`, so the `./CONTRIBUTING.md` form stays valid.

### The consumption rule

When `api.enabled` is true:

1. **Markdown only.** `*.md` under `docs_dir`. Nothing else — this repo's
   `docs/examples/` holds one `.md` and twelve `.sh` files, and shell scripts
   are not documents. Assets are Decision 10.
2. **A directory's index file is that directory's page.** `docs/index.md` is
   the repo root; `<dir>/README.md` is the page for `<dir>`. This holds
   uniformly, inside type directories and outside them:
   `docs/impl/README.md` → `/:owner/:repo/impl`,
   `docs/examples/README.md` → `/:owner/:repo/examples`. The docz-generated
   index table therefore *is* the type page's body, which is how docz-site
   already serves it.
3. **Type directories are otherwise reserved for docz documents.** A file under
   an enabled type's `dir` that is not its `README.md` must have frontmatter
   and match `IsDoczFile`; it is addressed `/:owner/:repo/:type/:docId` as
   today. A stray non-conforming `.md` there is skipped and reported rather
   than path-addressed — `/impl/notes.md` would be indistinguishable from
   `/impl/:docId`, and a type directory is a namespace docz itself manages, so
   a non-conforming file there is far more likely a mistake than an intent.
4. **Everything else under `docs_dir`** — minus exclusions, minus the index
   files consumed by (2) — is **path-addressed at its path relative to
   `docs_dir`**. `docs/examples/example1.md` → `examples/example1.md`.
5. **`docs_dir/templates/` is always excluded.** It holds
   `<docs_dir>/templates/<type>.md` and `index_<type>.md` override files —
   docz's own machinery, never documents. Excluding it unconditionally means
   `api.exclude` defaults to empty and a user who sets it does not silently
   lose the `templates` protection (the footgun `wiki.exclude` has, where
   setting the key replaces the default list wholesale).
6. **`additional_docs` are files outside `docs_dir`**, each path-addressed at
   its repo-relative path. An entry under `docs_dir` is a validation error —
   it is already consumed by (1)–(4), and listing it would produce two records
   for one file.

### Config types

```go
// APIConfig declares what docz-api and docz-site surface for this repo.
type APIConfig struct {
    Enabled        bool     `yaml:"enabled"`
    LandingPage    string   `yaml:"landing_page"`
    Exclude        []string `yaml:"exclude"`
    AdditionalDocs []string `yaml:"additional_docs"`
}
```

`yaml` tags only, no `mapstructure` — the siblings' `mapstructure` tags are
vestigial post-viper, and IMPL-0015 set this precedent for `ChangelogConfig`.

There is deliberately **no nested index struct**. Naming a landing page is
enabling it, so a separate boolean is redundant, and a nested `api.index` would
collide with the top-level `index:` block that governs README table generation.

`Config` gains one field:

```go
type Config struct {
    // ...
    Changelog ChangelogConfig `yaml:"changelog"`
    API       APIConfig       `yaml:"api"`
}
```

`DefaultConfig()` remains the sole source of defaults: `Enabled: false`,
`LandingPage: ""`, `Exclude: nil`, `AdditionalDocs: nil`. The block is dormant
by default.

### Normalization

`normalizeAPI(&cfg)` runs on both `Load` paths, immediately after
`normalizeChangelog` (which follows `fillTypeFieldDefaults`). It:

1. Backfills `api.landing_page` when empty and `api.enabled` is true —
   resolved as `<docs_dir>/index.md`, so it tracks a non-default `docs_dir`.
2. Strips leading `./` from `landing_page`, every `exclude` entry, and every
   `additional_docs` entry.
3. Leaves everything else alone. As with `changelog.file`, it deliberately does
   **not** call `filepath.Clean` — that would silently resolve the `..` that
   validation must still be able to see and reject.

Normalization never rejects. *Load normalizes, Validate judges.*

### Validation

`validateAPI()` is called from `Validate()` and returns immediately unless
`api.enabled` is true. This is the dormancy guarantee: a repo can commit the
block today, before docz-api or docz-site understand it, and `docz` keeps
loading the config without complaint.

When enabled, every path — `landing_page`, each `exclude` entry, each
`additional_docs` entry — goes through the shared repo-relative path rules
extracted from `validateChangelog` (INV-0007 Decision 2), wrapping:

```go
var ErrInvalidAPIPath = errors.New("invalid api path")
```

Rules specific to this block:

- An empty `exclude` or `additional_docs` entry is an error. An empty string in
  a list is always a mistake, unlike an omitted scalar which has a default.
- Duplicate `additional_docs` entries are an error — after `./` stripping, two
  entries naming the same file would produce two nav items for one document.
- An `additional_docs` entry under `docs_dir` is an error (consumption rule 6).
- An `additional_docs` entry whose **first path segment matches an enabled
  type's `dir` or canonical name** is an error. This is the one route
  ambiguity that dropping the namespace segment leaves open: a repo-root
  `design/notes.md` would address as `/:owner/:repo/design/notes.md`, which
  the router cannot distinguish from `/:owner/:repo/:type/:docId`. Catching it
  at load time is the same move `validateResolution` already makes for
  colliding type tokens.

Validation is **path-shape only**. It never checks that a file exists — it
cannot, since docz-api validates a config it fetched from a git tree with no
checkout. Consumers skip missing files and report them.

### Titles for frontmatter-less documents

This is the part that is not just config, and the reason INV-0007 concluded the
feature is not purely additive.

Every document docz describes today has frontmatter, so a display title is
always `Frontmatter.Title`. `CONTRIBUTING.md` and `docs/examples/README.md`
have none. Something must derive a title for the site's nav, breadcrumbs, and
search results, and the only signal in the file is its H1.

docz already implements exactly this logic — in `internal/wiki/titles.go`, via
an unexported `firstH1` behind a path-based `DocTitle`. Both facts disqualify
it: `internal/` is unreachable from another module (the `test/consumer/`
proof), and docz-api has bytes, not paths. Meanwhile `docparse.Headings`
**deliberately excludes H1**, because in a docz document the H1 is the title
and would pollute a ToC.

So docz adds one function (INV-0007 Decision 1):

```go
// Title returns the text of the first H1 heading in content, with inline
// markdown stripped, or "" if there is none.
func Title(content []byte) string
```

`""` is a normal outcome, not an error — the consumer supplies its own
fallback, typically the filename, exactly as docz-api already defaults a
missing changelog title. That keeps `docparse`'s no-error contract intact and
keeps presentation decisions consumer-side, per ADR-0001. `Headings` is
untouched, so its frozen H2–H6 behavior does not change.

### Route mapping (non-normative)

docz does not construct routes. This section records the intended mapping so
docz, docz-api, and docz-site agree on what the config means.

| Source | Route |
| ------ | ----- |
| `docs/index.md` (landing page) | `/:owner/:repo` |
| `docs/impl/README.md` (generated index) | `/:owner/:repo/impl` |
| `docs/impl/0015-foo.md` | `/:owner/:repo/impl/IMPL-0015` |
| `docs/examples/README.md` | `/:owner/:repo/examples` |
| `docs/examples/example1.md` | `/:owner/:repo/examples/example1` |
| `CONTRIBUTING.md` (additional) | `/:owner/:repo/CONTRIBUTING` |

The path is `docs_dir`-relative with no namespace segment, so the URL space is
a direct mirror of the `docs_dir` tree. This is coherent with DESIGN-0009's
existing map rather than an addition to it: `/:owner/:repo` is already "repo
detail" (now backed by `docs/index.md`) and `/:owner/:repo/:type` is already
"list of documents of one type" (now backed by `docs/<type>/README.md`, which
is what docz-site serves there today).

The index-file rule in consumption rule 2 is what makes the mirror uniform: a
directory's page is its `README.md`, whether or not the directory is a type.
Note this revises the mapping sketched in `.docz.example.yaml`, which had
`docs/examples/README.md → .../examples/README.md`; under the uniform rule it
is `.../examples`, matching how `docs/impl/README.md → .../impl` already works.

One thing left to docz-site: **whether `.md` appears in the URL**
(`/examples/example1` vs `/examples/example1.md`). docz supplies the path with
its extension; stripping it is presentation.

## API / Interface Changes

| Change | Surface | Kind |
| ------ | ------- | ---- |
| `APIConfig` struct | `pkg/doczcore/config` | Additive |
| `Config.API` field | `pkg/doczcore/config` | Additive |
| `ErrInvalidAPIPath` sentinel | `pkg/doczcore/config` | Additive |
| `DefaultConfig()` gains a dormant `API` value | `pkg/doczcore/config` | Additive |
| `Validate()` rejects bad paths **when enabled** | `pkg/doczcore/config` | Additive (dormant block unaffected) |
| `docparse.Title(content []byte) string` | `pkg/doczcore/docparse` | Additive |
| Shared repo-relative path validator | `pkg/doczcore/config`, unexported | Internal refactor |
| Dormant `api:` block in generated config | `internal/template/templates/docz_yaml.tmpl` | Golden regen |

Nothing is removed or changed in meaning, so this is a **minor** release under
ADR-0001's roll-forward semver: `v1.2.0`.

No CLI changes. No `cmd/` changes.

### Downstream contract (what docz-api pins as R10)

DESIGN-0008 lists requirements R1–R9 that the docz repo must satisfy for
docz-api. This design adds **R10**, in the same form:

> **R10.** `pkg/doczcore/config` exposes `Config.API` with `Enabled`,
> `LandingPage`, `Exclude []string`, and `AdditionalDocs []string`. Paths are
> repo-relative, `./`-stripped, forward-slash separated, and guaranteed by
> `Validate()` — when `api.enabled` is true — to contain no absolute prefix,
> volume name, `~`, `..`, `.`, empty segment, backslash, control character, or
> trailing `/`. `AdditionalDocs` entries are unique, lie outside `docs_dir`,
> and never begin with an enabled type's directory name.
> `pkg/doczcore/docparse` exposes `Title(content []byte) string`, returning `""`
> when a document has no H1, so consumers derive titles for frontmatter-less
> documents without reimplementing the rule. Available from docz `v1.2.0`.

Three consequences docz-api should treat as explicit, because each is easy to
rediscover as a bug:

- **The tree filter widens from `docs_dir/<type.dir>/` to `docs_dir/`.** The
  push-diff intersection in DESIGN-0008 step 4 widens the same way. This is a
  relaxation of an existing filter rather than a new fetch path — the recursive
  listing is already in hand.
- **`additional_docs` still needs its own lookup**, because those files are
  outside `docs_dir` by definition.
- **Path-addressed documents fail `document.IsDoczFile`.** The pattern is
  `^(\d+)-.*\.md$`; `CONTRIBUTING.md` and `examples/README.md` never match. An
  ingestion loop shaped as "scan → keep `IsDoczFile`" silently drops all of
  them. That predicate is now the *discriminator* between the two record types
  inside a type directory, not a global filter — and the `README.md` in each
  type directory must be kept despite failing it, because that file is the
  type page's body (consumption rule 2). Dropping it is the most likely way to
  get this wrong, since it is the one file that both fails `IsDoczFile` and
  lives inside a type directory.

## Data Model

docz itself stores nothing — the config *is* the model. For consumers, the
shape implied is a second record type alongside `documents`:

| Field | Source |
| ----- | ------ |
| `repo_id` | ingestion context |
| `path` | `docs_dir`-relative path, or repo-relative for `additional_docs` |
| `title` | `docparse.Title`; fall back to the filename when it returns `""` |
| `content` / `content_hash` | fetched blob, same gating as documents |

Keying is `(repo_id, path)` rather than `(repo_id, doc_id)` — path-addressed
documents have no id, which is precisely why DESIGN-0009's document route does
not fit them unchanged.

Ordering is by path. The enumerate-everything form of this design would have
given presentation order from list position; auto-consumption does not, and
nobody has asked for a curated `docs/` nav.

The landing page is a nullable pointer on the repo record rather than a row in
this table; it is one per repo and addressed as the repo root.

## Testing Strategy

Mirrors IMPL-0015's approach for `changelog:`.

**Unit — `pkg/doczcore/config`:**

- Decode: full block, partial block, absent block, `enabled` alone.
- Defaults: `DefaultConfig()` yields a dormant block; a config with no `api:`
  key round-trips identically to today.
- Normalization: `./` stripping on all three path-bearing fields;
  `landing_page` backfill when empty and enabled, including under a non-default
  `docs_dir`; **no** backfill when disabled; `..` survives normalization so
  validation can see it.
- Validation, enabled: every rejection class — absolute, volume name, `~`,
  backslash, control char, trailing `/`, `..`, `.`, empty segment, empty
  entry, duplicate entry, entry under `docs_dir`, entry whose first segment is
  an enabled type dir. Each asserted with `errors.Is(err, ErrInvalidAPIPath)`,
  not string matching.
- Validation, disabled: **every** rejection class above passes when
  `api.enabled` is false. This is the dormancy guarantee, and it deserves the
  same table-driven exhaustiveness as the positive cases.
- Type-collision validation interacts with enablement: a first segment matching
  a *disabled* type's dir is allowed.
- Merge: global + repo config, repo wins, `exclude` and `additional_docs`
  **replace** rather than append under the existing deep merge — worth an
  explicit test, since a reader may assume otherwise.

**Unit — `docparse.Title`:** golden fixtures under `docparse/testdata/`, per
the package's existing pattern. Cases: plain H1; H1 with inline markdown; H1
inside a fence (not a title); setext H1 (`===` underline — decide and pin the
behavior); no H1 at all; H1 not on the first line; multiple H1s (first wins);
H1 after frontmatter; empty input.

**Consumer proof — `test/consumer/`:** extend the existing external-module test
to read `cfg.API` and call `docparse.Title` by their public paths, proving the
surface is importable exactly as docz-api will import it.

**Parity:** `parity_baseline_test.go` continues to pin lenient unknown-key
decoding and case-sensitivity; the new block must not disturb either.

## Migration / Rollout Plan

1. **Ship dormant.** `v1.2.0` adds the schema, normalization, validation,
   `docparse.Title`, and the default-off block in `docz init` output. Repos
   without an `api:` key are byte-for-byte unaffected.
2. **Repos opt in.** A repo adds the block and turns it on whenever it likes.
   Because validation is enabled-gated, a repo can commit it `enabled: false`
   first and flip later.
3. **docz-api bumps its pin** to `v1.2.0`, widens the tree filter, adds the
   `additional_docs` lookup and the path-addressed record type, and pins R10.
4. **docz-site** backs `/:owner/:repo` with the landing page and adds the
   path-addressed catch-all route.

Rollback is trivial: the block is opt-in and inert to the CLI, so downgrading
docz only means the config carries a key nothing reads — the same lenient
unknown-key path that already lets an older docz load a newer config.

There is no data migration, because docz stores nothing.

The one operational caution: **flipping `api.enabled` publishes every `.md`
under `docs_dir`.** A repo with `docs/scratch/` or a stray `docs/TEMP-design.md`
(this repo had exactly that during IMPL-0015) publishes it unasked. That is the
cost of not enumerating, and `api.exclude` is the mitigation. Worth a sentence
in the README and in the generated config's comments.

## Open Questions

**None.** All ten were resolved on 2026-08-10; see **Decisions** below. The
question that produced Decision 10 is kept here for its alternatives, since a
future asset design will want them.

### 10. Should images and other assets be consumed?

Markdown under `docs_dir` routinely references relative images
(`![diagram](./images/arch.png)`). If only `.md` is ingested, those render
broken. DESIGN-0008 does not mention assets at all — there is no existing
answer to inherit, and whether docz-api serves blob content for non-markdown
today is a question for that repo.

- **a. (Recommendation) Out of scope for docz; no config, no schema change.**
  docz's rule stays `.md`-only for the *document* set. Assets are not
  documents — they have no title, no nav entry, no search body — so modeling
  them alongside documents is a category error. docz-api can fetch a fixed set
  of image extensions under `docs_dir` and serve them at their
  `docs_dir`-relative path with no docz involvement, because this design
  already defines that path mapping. Revisit only if it needs to be
  configurable per repo.
- **b. Add `api.assets` now** — either an extension list or a path list. Makes
  it explicit and repo-controlled, but it is speculative config for a need
  nobody has hit yet, and every extension list is wrong for someone.
- **c. Resolve links from ingested markdown** and fetch only referenced assets.
  Precise, no orphan blobs, and self-maintaining. But it needs relative-link
  parsing, which `docparse` does not provide (`Headings` + `TaskItems` only),
  so docz-api would parse markdown itself — the duplication INV-0007 F2 argues
  against, unless docz also grows a `Links` extractor.
- **d. No asset support.** Images 404. Honest and cheap, but a design doc with
  a broken architecture diagram is a bad first impression of the site.

## Decisions

All ten open questions resolved **(a)** on 2026-08-10, across three rounds of
review the same day. The body above reflects the final state; this table
records how it got there.

| # | Question | Resolution |
| - | -------- | ---------- |
| 1 | Entry shape for `additional_docs` | Bare strings — path only, title from the H1. Upgradeable to a scalar-or-mapping union later without a break, but only if we start from strings |
| 2 | `api.index` naming | **Revised.** Renamed to `api.landing_page`, a single optional string; `APIIndexConfig` dropped entirely. Removes the collision with the top-level `index:` block and a redundant boolean |
| 3 | Landing page default | `<docs_dir>/index.md`, resolved during normalization so it tracks a non-default `docs_dir`. This repo's existing `docs/index.md` confirms the shape |
| 4 | Meaning of `enabled: false` | Gates the api surface only. Disabled or absent → docz-api ingests exactly what it does today. The only backward-compatible reading |
| 5 | Globs | No — literal paths only. Moot under the consumption rule: nothing needs enumerating inside `docs_dir`, and docz cannot expand patterns in the no-checkout model anyway |
| 6 | Overlap with type dirs | **Revised and strengthened.** "An `additional_docs` entry must not be under `docs_dir`" replaces the per-type-dir check — one prefix test, no type enumeration, stable when a type is disabled |
| 7 | Missing files | Path-*shape* validation only; docz never checks existence. Consumers skip and report. Anything else makes validity host-dependent |
| 8 | Route shape | **Revised.** No namespace segment: `/:owner/:repo/*path`, mirroring `docs_dir` directly. Safe for `docs_dir` content because type dirs are reserved; the residual repo-root ambiguity is closed by validation instead |
| 9 | `docz init` emission | Yes, `enabled: false` with comments — the DESIGN-0010 Decision 6 precedent |
| 10 | Images and assets | Out of scope for docz — no config, no schema change. Assets are not documents; docz-api can serve them at their `docs_dir`-relative path using the mapping this design already defines |

### Revision history

**2026-08-10, after the first round of answers.** Three changes, all adopted:

1. **Consume the whole `docs_dir`** rather than enumerating extra files. This
   shrinks the docz-api change to widening an existing filter, makes globs
   moot, reduces the overlap rule to one prefix check, and turns route mapping
   from a policy into a coordinate system. `additional_docs` narrows to mean
   "outside `docs_dir`," which makes it structurally identical to
   `changelog.file`. Restricted to `*.md` after finding twelve `.sh` files
   under this repo's `docs/examples/`.
2. **`api.exclude` added**, with `docs_dir/templates/` always excluded
   implicitly so the default can be empty. Deliberately *not* mirroring
   `wiki.exclude`, which also excludes `examples` — docz-site is a different
   product and should publish `docs/examples/`. This resolves INV-0007 F7
   rather than leaving it as an apparent inconsistency.
3. **Decision 8 reversed** from a `docs/` namespace segment to a bare path,
   plus the first-segment validation rule that makes it unambiguous.

**2026-08-10, third round.** Two corrections:

4. **A directory's `README.md` is that directory's page** — consumption rule 2.
   The previous draft reserved type directories entirely, which would have made
   docz-api skip `docs/impl/README.md`; but that file is exactly what
   docz-site already serves at `/donaldgifford/docz/impl`. Correcting it turned
   out to *generalize* the model rather than special-case it: the same rule
   covers `docs/index.md` at the root and `docs/examples/README.md` in a
   non-type directory, so the URL space mirrors `docs_dir` uniformly. It also
   revises the mapping sketched in `.docz.example.yaml`, where
   `docs/examples/README.md` was drawn as `.../examples/README.md` rather than
   `.../examples`.
5. **Decision 10 (assets) closed as out of scope**, keeping this design to the
   config block and `docparse.Title`.

## References

- INV-0007 — the internals audit this design is built on; its three decisions
  cover `docparse.Title`, the shared path validator, and docz's non-role in
  route construction
- DESIGN-0008 — docz-api ingestion pipeline, no-checkout model, requirements
  R1–R9 (this design proposes R10)
- DESIGN-0009 — docz-site information architecture and route map
- DESIGN-0010 / IMPL-0015 — the `changelog:` block: opt-in schema,
  normalize-then-validate split, dormancy guarantee, and the path-hardening
  history this design reuses
- ADR-0001 — five-package public core, whole-package promotion, roll-forward
  semver, facts-not-interpretation
- `.docz.example.yaml` — the drafted `api:` block this design formalizes
