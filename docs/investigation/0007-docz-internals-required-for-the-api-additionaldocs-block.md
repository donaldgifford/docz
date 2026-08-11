---
id: INV-0007
title: "docz internals required for the api additional_docs block"
status: Concluded
author: Donald Gifford
created: 2026-08-10
---

<!-- markdownlint-disable-file MD025 MD041 -->

# INV-0007: docz internals required for the api additional_docs block

<!--toc:start-->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
  - [F1. The config block itself is genuinely small](#f1-the-config-block-itself-is-genuinely-small)
  - [F2. There is no public way to get a title out of a frontmatter-less markdown file](#f2-there-is-no-public-way-to-get-a-title-out-of-a-frontmatter-less-markdown-file)
  - [F3. The path-validation rules exist but are not reusable](#f3-the-path-validation-rules-exist-but-are-not-reusable)
  - [F4. Additional docs have no id, and the whole downstream contract is keyed on id](#f4-additional-docs-have-no-id-and-the-whole-downstream-contract-is-keyed-on-id)
  - [F5. The ingestion fetcher filters the tree by type dir, so additional docs are invisible to it](#f5-the-ingestion-fetcher-filters-the-tree-by-type-dir-so-additional-docs-are-invisible-to-it)
  - [F6. Globs cannot be expanded by docz in the consumer's model](#f6-globs-cannot-be-expanded-by-docz-in-the-consumers-model)
  - [F7. wiki exclude and api additional_docs disagree about examples](#f7-wiki-exclude-and-api-additionaldocs-disagree-about-examples)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
- [Open Questions](#open-questions)
  - [1. Where should the H1/title extractor live?](#1-where-should-the-h1title-extractor-live)
  - [2. Should the path-validation rules be shared between changelog.file and additional_docs?](#2-should-the-path-validation-rules-be-shared-between-changelogfile-and-additionaldocs)
  - [3. Does docz need to expose the URL/route mapping?](#3-does-docz-need-to-expose-the-urlroute-mapping)
- [Decisions](#decisions)
  - [Amendment to Decision 1: the empty-string contract](#amendment-to-decision-1-the-empty-string-contract)
  - [How the design resolved F5, F6, and F7](#how-the-design-resolved-f5-f6-and-f7)
- [References](#references)
<!--toc:end-->

## Question

DESIGN-0011 proposes an `api:` block in `.docz.yaml` carrying an index page
and an `additional_docs` list of non-docz markdown files for docz-api and
docz-site to ingest and render. **Is this purely additive config, or does it
require changes to docz's internals and public surface?**

The working assumption going in was "there is not much we have to do." This
investigation is a concrete audit of that assumption.

## Hypothesis

Mostly additive. `ChangelogConfig` (DESIGN-0010 / IMPL-0015) is the exact
precedent — a new struct, a `DefaultConfig()` entry, a normalization pass, an
enabled-only validator, a `docz_yaml.tmpl` block — and nothing outside
`pkg/doczcore/config` changed for it. The expectation was that `api:` would
follow the same shape and be similarly self-contained.

**The hypothesis is partly wrong.** The config half is as small as expected;
the *parsing* half is not, because additional docs are the first documents docz
would describe that have neither frontmatter nor a docz filename.

## Context

**Triggered by:** DESIGN-0011 (`api:` block), which in turn serves DESIGN-0008
(docz-api ingestion) and DESIGN-0009 (docz-site route map). The immediate
prompt was the `api:` section drafted in `.docz.example.yaml`.

This matters *now* rather than later because `pkg/doczcore` is frozen under
semver (ADR-0001): additive changes are a minor bump, but a helper that turns
out to be needed after the fact is either a second minor release or a
consumer-side reimplementation that drifts from docz's own behavior.

## Approach

1. Read the drafted `api:` block in `.docz.example.yaml` and enumerate every
   field it implies.
2. Trace the `ChangelogConfig` precedent end to end to establish what "purely
   additive config" actually costs.
3. For each field, ask what a consumer must *do* with it, and check whether
   the public surface supports that.
4. Check DESIGN-0008's ingestion pipeline and DESIGN-0009's route map for
   assumptions that additional docs would violate.
5. Grep the public packages for the specific capabilities implied (title
   extraction, path validation, filename classification).

## Environment

| Component | Version / Value |
| --------- | --------------- |
| docz | `v1.1.0` (frozen `pkg/doczcore`, ADR-0001) |
| Branch | `docs/design-api-additional-docs` |
| Precedent | DESIGN-0010 / IMPL-0015 (`changelog:` block) |
| Consumers | docz-api (DESIGN-0008), docz-site (DESIGN-0009) |

## Findings

### F1. The config block itself is genuinely small

The `changelog:` precedent is a fair estimate for the `api:` block. Everything
`ChangelogConfig` needed lives in one package:

- `pkg/doczcore/config/config.go` — the struct, the `Config` field, the
  `DefaultConfig()` entry, `normalizeChangelog`, `validateChangelog`
- `pkg/doczcore/config/constants.go` — `DefaultChangelogFile`
- `internal/template/templates/docz_yaml.tmpl` — the dormant block

`Load` already has the hook points: `fillTypeFieldDefaults(&cfg)` followed by
`normalizeChangelog(&cfg)` at both `Load` call sites (`config.go:211` and
`:651`). An `normalizeAPI(&cfg)` slots in beside it with no restructuring.

`api:` is a deeper shape (a nested `index:` object and a string slice rather
than two scalars), but nothing about it is structurally new. **This half of the
hypothesis holds.**

### F2. There is no public way to get a title out of a frontmatter-less markdown file

This is the finding that breaks the assumption.

Every document docz currently describes has YAML frontmatter, so a display
title is always `Frontmatter.Title`. `additional_docs` points at
`CONTRIBUTING.md`, `DEVELOPMENT.md`, `docs/examples/README.md` — ordinary
markdown with **no frontmatter at all**. Something has to produce a human
title for the site's nav, breadcrumbs, and search results.

docz already solves this, in the wrong place and with the wrong signature:

```go
// internal/wiki/titles.go
func DocTitle(filePath string) (string, error)   // ← internal/, and path-based
func firstH1(data []byte) string                 // ← unexported
```

Two independent blockers:

1. **`internal/wiki` is unreachable from another module.** This is not a
   guess — `test/consumer/` exists precisely to prove it, and IMPL-0014
   recorded the spot-check that importing `internal/template` from outside
   fails with `use of internal package ... not allowed`.
2. **`DocTitle` reads from disk.** docz-api never has a checkout (DESIGN-0008
   Decision 1 — Git Trees API, blobs fetched as bytes). Even promoted verbatim
   it would be unusable; the consumer needs bytes-in, exactly the argument
   R3 already made for `ParseFrontmatter` over `ScanDocuments`.

And the obvious public candidate does not cover it. `docparse.Headings`
**deliberately excludes H1**:

```go
// pkg/doczcore/docparse/headings.go:11
// Level is the heading depth, 2 through 6. H1 headings are
// excluded: in docz documents the H1 is the document title, which
```

That exclusion is correct for docz documents and exactly wrong for additional
docs, where the H1 *is* the only title available.

**Net:** with today's surface, docz-api must reimplement H1 extraction. That
means duplicating fence-awareness, ATX parsing, and inline-markdown stripping —
all of which `docparse` already implements and pins with goldens — in a second
codebase, where it will drift. This is the precise failure mode ADR-0001's
"one definition module-wide" rule exists to prevent (`docwrite.CheckTask`
validating through `docparse.TaskItems` is the same rule applied earlier).

### F3. The path-validation rules exist but are not reusable

`additional_docs` entries are repo-relative paths fetched out of a git tree —
byte for byte the same trust problem as `changelog.file`. IMPL-0015 hardened
that path against traversal, host-dependent separator semantics, `~`
expansion, control characters, and non-canonical segments, and those rules
were expensive to get right (a real bypass shipped in review and was caught
pre-tag).

The rules currently live in an unexported method:

```go
func (c *Config) validateChangelog() error   // config.go:450
func hasVolumeName(p string) bool            // config.go, unexported
```

So `additional_docs` cannot reuse them without either extracting a shared
helper or copy-pasting the switch. Copy-pasting is how the two validators
drift apart the next time one is hardened.

Note this is *internal* reuse — no new export is required, since validation
runs inside `Validate()` and the consumer only sees the error. That keeps the
fix cheap.

### F4. Additional docs have no id, and the whole downstream contract is keyed on id

DESIGN-0009's route map addresses a document as:

```
/:owner/:repo/:type/:docId      e.g. .../design/DESIGN-0009
```

and notes "`:docId` is the stable frontmatter id ... unique within a repo, so
`(owner, repo, docId)` is a stable permalink." DESIGN-0008's data model keys
`documents` on `(repo_id, doc_id)` and computes `content_hash` per doc.

An additional doc has **no type and no id**. It cannot be addressed by that
route shape at all. The example URLs in `.docz.example.yaml` are a different
shape entirely — path-based, and rooted at `/repos/${repo-name}/`:

```
./docs/examples/README.md -> ${docz-site-url}/repos/${repo-name}/examples/README.md
./CONTRIBUTING.md         -> ${docz-site-url}/repos/${repo-name}/CONTRIBUTING.md
```

Two mismatches with the existing design, both worth resolving in DESIGN-0011
rather than discovering during implementation:

- **Prefix.** The drafted URLs carry `/repos/`; DESIGN-0009's map has
  `/:owner/:repo` at the root with no `/repos/` prefix (`/repos` is only the
  *list* route).
- **Owner.** The drafted URLs use a single `${repo-name}`; DESIGN-0009 uses
  `:owner/:repo` because owner+repo together identify a repo.

This is a docz-site/docz-api concern, not a docz-CLI one — but it determines
whether `additional_docs` needs to carry anything beyond a path (a slug? a
title override? a nav position?), which *is* a docz config-schema question.

### F5. The ingestion fetcher filters the tree by type dir, so additional docs are invisible to it

DESIGN-0008 step 1 is explicit:

> filter to `.docz.yaml` and blobs under `docs_dir/<type.dir>/`

`CONTRIBUTING.md` at the repo root is outside that filter, and so is
`docs/examples/README.md` (`examples` is not a type dir). The push-diff
optimization in step 4 has the same shape — it intersects changed paths with
`docs_dir`.

Consequently docz-api needs a second, config-driven fetch set. That is
entirely docz-api's work, but it confirms `additional_docs` is not something a
consumer picks up for free by reading the config — it changes the pipeline.

Related: `document.IsDoczFile` is `^(\d+)-.*\.md$`, so every additional doc
fails it by construction. Any consumer loop written as "scan dir → keep
`IsDoczFile`" silently drops all of them. Worth stating in the contract so it
is not rediscovered as a bug.

### F6. Globs cannot be expanded by docz in the consumer's model

The natural next request after `additional_docs` is `docs/examples/*.md`. docz
cannot expand that for docz-api: pattern expansion needs a listing, docz-api
has a git tree rather than a filesystem, and `pkg/doczcore` is
stdlib-and-bytes only by ADR-0001.

Either the consumer expands patterns against its own tree listing (in which
case docz's role is to define the syntax and validate it, not to resolve it),
or v1 ships literal paths only. The CLI could expand globs locally via
`filepath.Glob` and diverge from the consumer — which would be worse than not
supporting them.

### F7. wiki exclude and api additional_docs disagree about examples

`DefaultConfig()` sets `Wiki.Exclude: []string{"templates", "examples"}`, and
the drafted `api:` block lists `./docs/examples/README.md` as an additional
doc. Both are defensible — the MkDocs nav and the docz-site surface are
different products — but a reader will notice one config excluding what the
other includes. Worth an explicit sentence rather than leaving it as apparent
inconsistency.

## Conclusion

**Answer: No — this is not purely additive config.** The config block is as
small as hypothesized (F1), but the feature as drafted needs one genuine
addition to the frozen public surface and one internal refactor:

| Need | Where | Cost |
| ---- | ----- | ---- |
| `APIConfig` struct, defaults, normalize, validate | `pkg/doczcore/config` | Additive, minor bump |
| Dormant `api:` block | `internal/template/templates/docz_yaml.tmpl` | Trivial (+ golden regen) |
| **Public H1 / title extractor, bytes-in** | `pkg/doczcore/docparse` or `document` | **Additive, minor bump — the real finding** |
| Shared repo-relative path validator | `pkg/doczcore/config`, unexported | Internal refactor, no API change |

None of it is breaking, so the whole thing is one **minor** release. But
shipping the config without the title extractor would push H1 parsing into
docz-api, which is the duplication ADR-0001 exists to prevent — so they should
ship together.

## Recommendation

1. **Ship the config block and the title extractor in the same minor release.**
   Splitting them guarantees docz-api writes its own H1 parser in the gap.
2. **Extract the repo-relative path rules into one unexported helper** and
   have both `validateChangelog` and the new `validateAPI` call it, before
   there are two hardening histories to keep in sync.
3. **Resolve the route shape in DESIGN-0011** (F4) before implementing, since
   it determines whether `additional_docs` entries are bare strings or objects.
   Changing that after the config ships is a breaking schema change.
4. **Defer globs** (F6). Literal paths only for v1; revisit once docz-api has
   a working tree-listing path.
5. **Add the contract clause** (call it R10) to DESIGN-0008's requirements
   list, in the same form as R1–R9, so docz-api has an explicit acceptance
   criterion rather than an inference from this INV.

## Open Questions

### 1. Where should the H1/title extractor live?

The one new public symbol this feature actually requires (F2). It must be
bytes-in, must be fence-aware, and must not disturb `docparse.Headings`'
frozen H2–H6 contract.

- **a. (Recommendation) `docparse.Title(content []byte) string`** — a new
  function in `docparse` returning the first H1's stripped text, or `""` when
  there is none. `docparse` is already the module's markdown fact extractor,
  is already fence-aware, already has `AnchorSlug` and inline-markdown
  stripping to reuse, and its no-error / facts-not-interpretation contract fits
  a "" return exactly. `Headings` keeps excluding H1, so nothing existing
  changes.
- **b. `docparse.Headings` starts including H1** with a `Level: 1` entry, and
  consumers filter. Truthful to the name, but it is a **breaking behavior
  change** to a frozen function — every existing caller (`toc`, and any
  consumer ToC) would suddenly emit the document title as a ToC entry.
- **c. `document.Title(content []byte) string`** — put it beside
  `ParseFrontmatter`, arguing that "title" is a document concern rather than a
  markdown fact. Keeps docz-api's imports to one package, but splits markdown
  parsing across two packages and duplicates fence handling (the same tension
  DESIGN-0010 Decision 1 already navigated for `ParseChangelog`).
- **d. `document.DocTitle(content []byte, filename string) string`** — the
  full `internal/wiki` fallback chain promoted: frontmatter `ID: Title`, then
  H1, then title-cased filename. Most useful to the consumer, but it bakes a
  presentation policy ("`ID: Title`") into the frozen core, which ADR-0001
  says belongs consumer-side.
- **e.** Ship nothing; let docz-api extract the H1 itself.

### 2. Should the path-validation rules be shared between `changelog.file` and `additional_docs`?

Both are repo-relative paths a consumer fetches from a git tree, and F3 shows
the hardened rules are currently locked inside `validateChangelog`.

- **a. (Recommendation) Extract one unexported
  `validateRepoRelativePath(field, value string) error`** and call it from
  both, each wrapping its own sentinel. One hardening history, no new public
  API, and the error text stays field-specific.
- **b. Duplicate the switch** into `validateAPI`. Zero refactor risk today,
  guaranteed drift the next time either is hardened.
- **c. Export it as `config.ValidateRepoPath`** so docz-api can pre-validate
  paths itself. Adds public surface to the frozen package for a case the
  consumer gets for free via `Validate()`.
- **d.** Relax the rules for `additional_docs` (allow `..`), on the grounds
  that these are explicitly-listed files rather than a computed path. Not
  recommended — it reintroduces exactly the traversal the changelog validator
  rejects, for the same fetch-from-tree consumer.

### 3. Does docz need to expose the URL/route mapping?

F4 shows the drafted URL shape does not match DESIGN-0009's route map. Whoever
owns that mapping owns a compatibility surface.

- **a. (Recommendation) No — docz stores and validates config only.** Route
  construction is docz-site's concern; docz-api serves paths and titles.
  Consistent with ADR-0001 (facts, not interpretation) and keeps docz out of
  the site's URL-versioning problem. DESIGN-0011 still documents the
  *intended* mapping as non-normative prose so the three repos agree.
- **b. docz exposes a helper** (e.g. `config.APIDocRoute(path) string`)
  implementing the "strip `docs/`, keep folders" rule. One definition, but it
  freezes a site URL policy into a Go library that ships on a different
  release cadence than the site.
- **c. `additional_docs` entries become objects** carrying an explicit
  `route:`/`slug:` per file, so no mapping rule is needed at all. Most
  flexible and most verbose; also the only option that must be decided
  *before* the schema ships, since string→object is a breaking change.

## Decisions

All three open questions resolved **(a)** on 2026-08-10.

| # | Question | Resolution |
| - | -------- | ---------- |
| 1 | H1 extractor home | `docparse.Title(content []byte) string` — new function in `docparse`, returns the first H1's stripped text or `""`. `Headings` keeps excluding H1, so nothing frozen changes |
| 2 | Share the path validator | Extract one unexported `validateRepoRelativePath`; `validateChangelog` and `validateAPI` both call it and wrap their own sentinel. No new public API |
| 3 | Expose route mapping | No — docz stores and validates config only. Route construction is docz-site's; DESIGN-0011 documents the intended mapping as non-normative prose |

### Amendment to Decision 1: the empty-string contract

Resolving (1a) added an explicit consumer-side rule: **`Title` returning `""` is
a normal outcome, not an error**, and the consumer supplies its own fallback —
the filename, exactly as docz-api already defaults a missing changelog title.

This keeps the split clean and is the same division ADR-0001 draws everywhere
else: docz reports the fact (there is no H1), the consumer decides the
presentation (call it `CONTRIBUTING`). It also means `Title` needs no error
return and no sentinel, preserving `docparse`'s no-error contract exactly.

One note for the consumer: `wiki.FilenameTitle` — docz's own
basename-to-title-case helper — is in `internal/wiki` and therefore
unreachable, like `DocTitle` itself (F2). docz-api writes its own. That is
acceptable here in a way the H1 rule was not: title-casing a basename is a
presentation choice with no markdown grammar behind it, so two implementations
diverging is a cosmetic difference rather than a correctness bug.

### How the design resolved F5, F6, and F7

DESIGN-0011 settled on consuming every `.md` under `docs_dir` rather than
enumerating extra files, which changes what two of these findings cost:

- **F5 softens.** docz-api widens its existing `docs_dir/<type.dir>/` tree
  filter to `docs_dir/` — a relaxation of a filter it already applies to a
  listing it already has, not a new fetch path. Only `additional_docs`, now
  meaning "outside `docs_dir`," needs a separate lookup.
- **F6 becomes moot.** Nothing inside `docs_dir` needs enumerating, so the
  pressure for glob support largely disappears. The finding still stands as the
  reason docz should not add globs later.
- **F7 is resolved rather than deferred.** `api.exclude` defaults to empty with
  `docs_dir/templates/` always excluded, deliberately *not* mirroring
  `wiki.exclude`'s `examples` entry — docz-site should publish
  `docs/examples/`, MkDocs nav should not. Different products, different
  defaults, now stated.

**F2 is unchanged and remains the finding that mattered**: auto-consumption
increases the number of frontmatter-less documents rather than reducing it, so
`docparse.Title` is more necessary under the final design, not less.

## References

- DESIGN-0011 — the `api:` block this investigation supports
- DESIGN-0008 §Ingestion pipeline, §Requirements for the docz repo (R1–R9)
- DESIGN-0009 §Information architecture and route map
- DESIGN-0010 / IMPL-0015 — the `changelog:` block precedent and its
  path-hardening history
- ADR-0001 — the five-package public core, whole-package promotion, and the
  one-definition-module-wide rule
- `internal/wiki/titles.go` — the existing, unreachable title logic
- `pkg/doczcore/docparse/headings.go:11` — the H1 exclusion
