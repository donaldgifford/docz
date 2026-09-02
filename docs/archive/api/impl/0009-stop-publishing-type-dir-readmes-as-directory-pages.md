---
id: IMPL-0009
title: "Stop publishing type-dir READMEs as directory pages"
status: Completed
author: Donald Gifford
created: 2026-09-01
---
<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL 0009: Stop publishing type-dir READMEs as directory pages

**Status:** Completed
**Author:** Donald Gifford
**Date:** 2026-09-01

<!--toc:start-->
- [Objective](#objective)
- [Background](#background)
  - [What changes, concretely](#what-changes-concretely)
  - [What deliberately does not change](#what-deliberately-does-not-change)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: Classifier change + unit proof](#phase-1-classifier-change--unit-proof)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2: End-to-end proof](#phase-2-end-to-end-proof)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 3: Docs, rollout note, close-out](#phase-3-docs-rollout-note-close-out)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Rollout](#rollout)
- [Follow-ups](#follow-ups)
- [Open Questions](#open-questions)
  - [1. Where does the type-dir README land after the carve-out is gone?](#1-where-does-the-type-dir-readme-land-after-the-carve-out-is-gone)
  - [2. Narrow the fetch to stop downloading type-dir READMEs?](#2-narrow-the-fetch-to-stop-downloading-type-dir-readmes)
  - [3. Spec surface: bump for the behavior change?](#3-spec-surface-bump-for-the-behavior-change)
  - [4. Operator-facing rollout note?](#4-operator-facing-rollout-note)
- [Dependencies](#dependencies)
- [References](#references)
<!--toc:end-->

## Objective

Delete the six-rule page classifier's rule-4 carve-out so enabled type
directories publish **nothing**: no more publishing a type dir's own
`README.md` as the extensionless `<dir>` directory page.

**Implements:** [issue #28] (amends this repo's [DESIGN-0004] rule 4).

## Background

Rule 4 of the classifier (`internal/ingest/pages.go`, `classifyDocsDir`)
reserves enabled type dirs for docz documents but carves out one page: the
dir's own `README.md`, published at the extensionless `<dir>` path. That
implemented docz DESIGN-0011 clause 2, whose premise was "the docz-generated
index table *is* the type page's body, which is how docz-site already serves
it."

The premise doesn't survive contact with the consumer. docz-site synthesizes
its own type page at `/:owner/:repo/:type` (live listDocs, counts, curated
blurbs) and reserves that URL for the type route — so the published README
materializes at a **second** URL, `/:owner/:repo/pages/<dir>`, duplicating
the type surface:

- the repo nav shows the type under **doc types** AND its README under
  **pages**
- search returns a `source: "page"` hit ("Design Documents") beside the
  type's own docs
- the page body is the docz-generated index table — a stale, less capable
  copy of the type page

### What changes, concretely

Research findings that bound the change (all verified against HEAD):

- **One branch in one function.** `classifyDocsDir`'s rule-4 block has three
  arms: the `README.md` carve-out (delete), the `IsDoczFile` silent skip
  (keep — `buildDocuments`'s business), and the stray-file skip+Warn (keep).
  The only decision is which arm catches `README.md` afterwards (OQ-1).
- **Only the classifier unit test pins the carve-out.**
  `TestBuildPagesClassifier` publishes `docs/rfc/README.md` as page `rfc`
  and asserts its RepoPath/Title; its `docs/impl/README.md` and
  `docs/guides/README.md` fixtures are **non-type** dirs (rule 5) and stay
  published. The e2e fixture (`internal/e2e/pages_integration_test.go`), the
  httpapi handler tests, and the indexmap test all use only the rule-5
  `docs/guides/README.md` — none pin rule 4, none change shape.
- **No spec text states the rule.** `api/openapi.yaml` and `api/README.md`
  never mention type-dir READMEs (grep-verified), so the wire contract's
  prose stays true as written; the page rows simply stop appearing (OQ-3).
- **The fetch still downloads the blobs.** `classifyTree` widens to every
  `.md` under `docs_dir` when `apiHint` reports enabled; the hint layer
  can't see enabled types. Post-change the blob is fetched then classified
  to nothing (OQ-2).
- **The webhook needs no change.** Type-dir READMEs live under `docs_dir/`,
  so `shouldIngest`'s prefix check already re-ingests pushes touching them —
  which is exactly what drives the desired-state row deletion.

### What deliberately does not change

- `buildDocuments`, the store reconcile, `syncIndex`, the serve layer, and
  the wire DTOs are untouched. Reconcile is desired-state: a page absent
  from `buildPages`' output is deleted from `repo_pages` and purged from
  Meilisearch by the machinery IMPL-0007 already proved.
- The rule-5 directory-page mapping outside type dirs (README wins, lone
  `index.md` serves) is untouched.

## Scope

### In Scope

- `internal/ingest/pages.go` rule-4 carve-out removal (+ comment rewrite).
- Unit + e2e test updates proving type dirs publish nothing.
- DESIGN-0004 amendment note, CLAUDE.md classifier text, rollout note
  (per OQ-3/OQ-4).

### Out of Scope

- **docz** (upstream): amending DESIGN-0011 clause 2 to exempt enabled type
  dirs from the uniform directory-page mapping — companion change in the
  docz repo, referenced from the PR.
- **docz-site** (after this ships): dropping the `design`/`impl` entries
  from `DEMO_PAGES` fixtures + the nav-tree test pin; optional redirect of
  stale `/pages/<type-dir>` deep links to the type page. Rides docz-site's
  own tracking.
- Backfill/migration tooling — none needed (see [Rollout](#rollout)).
- Fetch narrowing beyond OQ-2's answer.

## Implementation Phases

Each phase builds on the previous one. A phase is complete when all its tasks
are checked off and its success criteria are met.

---

### Phase 1: Classifier change + unit proof

Remove the carve-out and repin the classifier's behavior at the unit seam.

#### Tasks

- [x] Delete the `p == td+"/"+readmeName` branch from `classifyDocsDir`
      rule 4 and route `README.md` per OQ-1's answer; rewrite the rule-4
      comment (type dirs publish nothing; docz docs stay
      `buildDocuments`'s business; strays keep skip+Warn).
- [x] Update `TestBuildPagesClassifier`: keep the `docs/rfc/README.md` blob
      in the fixture but drop `"rfc"` from the wanted published paths (the
      blob now proves **absence**); replace the rfc-page assertions with an
      explicit not-published check; fix the ContentHash comparator that
      referenced the rfc page; update the rule-4 fixture comments.
- [x] Per OQ-1's answer, assert the log behavior for a type-dir README
      (no Warn if 1a; Warn if 1b) alongside the existing stray-file case.
- [x] `just fmt` + `just lint` clean; commit
      (`fix(ingest): stop publishing type-dir READMEs as pages`).

#### Success Criteria

- `go test ./internal/ingest/` green: a type-dir `README.md` publishes
  nothing, docz docs still flow to `buildDocuments`, genuinely stray files
  still skip+Warn, and rule-5 directory pages (non-type `README.md`, lone
  `index.md`) are untouched.
- `just lint` reports 0 issues.

**Status: COMPLETE ✅** (2026-09-01) — the carve-out is gone: rule 4 now
returns `"", false` for everything in an enabled type dir, warning only
when the file is neither the dir's `README.md` nor an `IsDoczFile` match.
`TestBuildPagesClassifier` keeps the `docs/rfc/README.md` blob as an
**absence** proof, and the new `TestBuildPagesTypeDirWarnsOnlyForStrays`
captures slog records to pin OQ-1a directly: three type-dir blobs
(README, document, stray) publish nothing and exactly one — the stray —
is reported. `just lint` 0 issues; full unit suite green.

A post-phase review pass (style + adversarial correctness) caught one
real regression in the first cut: gating the silence on `path.Base(p)`
matched `README.md` at **any** depth under a type dir, so a nested
`docs/<type>/sub/README.md` — a human's misplaced directory, not a docz
artifact — went from reported stray to silent, and inconsistently (the
`index.md` beside it still warned). The check is now the exact path
(`p != td+"/"+readmeName`), scoping the silence to the file `docz update`
actually regenerates, with a fixture pinning the distinction
(revert-drilled: the `path.Base` form fails the test by name).

---

### Phase 2: End-to-end proof

Prove the full pipeline — ingest, serve, search — through the real stack.

#### Tasks

- [x] Add a `docs/frameworks/README.md` blob (the fixture's enabled type
      dir) to `TestE2ERepoPagesServeAndDisable`'s blob set.
- [x] Assert the pages list still has exactly its 3 pages (additional doc,
      file page, rule-5 directory page), `GET
      /api/v1/repos/acme/paged/pages/frameworks` 404s, and search returns
      no `source: "page"` hit for the type-dir README's content.
- [x] Run the integration suites (`just test-integration`) against real
      Postgres + Meilisearch.
- [x] Commit (`test(e2e): prove type dirs publish no pages`).

#### Success Criteria

- The e2e run proves a repo with an enabled type dir and a README in it
  serves no page row for it and no search hit from it, while every other
  page kind is unaffected.
- Full `just test-integration` green (no other integration test regressed).

**Status: COMPLETE ✅** (2026-09-01) — `TestE2ERepoPagesServeAndDisable`
gained the enabled type dir's own `docs/frameworks/README.md` and a
`type dir publishes no page` subtest: the pages list still holds exactly
its three rows, `GET /pages/frameworks` 404s, and the README's distinctive
content (`pangolin`) returns no `source: "page"` hit. Green against real
Postgres + Meilisearch, and the whole `just test-integration` suite
(including store, queue, search, webhook) passed with nothing else
regressed. The run also surfaced the `buildDocuments` warn recorded under
[Follow-ups](#follow-ups).

---

### Phase 3: Docs, rollout note, close-out

Record the amendment where the old rule is written down, then ship.

#### Tasks

- [x] Amend [DESIGN-0004]: rule-4 text, the mapping-table row for
      `docs/impl/README.md`, and the test-plan mention — an amendment note
      referencing [issue #28] + this IMPL (the doc already carries INV-0008
      5a-amendment precedent).
- [x] Update CLAUDE.md's IMPL-0007 classifier bullet ("type-dir `README.md`
      is the type's one page" → type dirs publish nothing, per IMPL-0009).
- [x] Apply OQ-3's answer (spec bump or none) and OQ-4's answer
      (deploy/README rollout note or none).
- [x] `docz update` (restore any TOC underscore-anchor damage in other
      docs); flip this doc → Completed with per-phase status blocks.
- [x] Final gates: `just ci` green; changelog sync commit
      (`mise exec -- git-cliff -o CHANGELOG.md`).
- [x] Open the PR (`patch` label — a behavior fix, no schema or spec-surface
      change) with `Closes #28`; note the docz DESIGN-0011 amendment and
      docz-site follow-ups as companion changes in their own repos.

#### Success Criteria

- `just ci` green; the PR closes [issue #28] on merge.
- No doc in the repo still states that type dirs publish their README —
  grep for the old rule text comes back empty outside historical
  IMPL-0007/INV-0008 records.

**Status: COMPLETE ✅** (2026-09-01) — DESIGN-0004 carries an amendment
block (rule 4, the mapping-table row, and the test-plan line) with the
consumer rationale; CLAUDE.md's classifier bullet is corrected and the
pages section leads with the amendment plus the known adjacent
`buildDocuments` warn; `deploy/README.md` records the natural-refresh
rollout (OQ-4a). Per OQ-3a no spec change was made — grep confirms
`api/openapi.yaml` never described the rule, so the contract stays true
as written at `1.4.1`. `just ci` green end to end; `docz update` run with
the TOC underscore anchors restored in DESIGN-0004 and INV-0008.

---

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/ingest/pages.go` | Modify | Delete rule-4 README carve-out; rewrite comment |
| `internal/ingest/pages_test.go` | Modify | Repin classifier: type-dir README publishes nothing |
| `internal/e2e/pages_integration_test.go` | Modify | Type-dir README blob → absent from list/serve/search |
| `docs/design/0004-consume-the-docz-v120-api-block-pages-landing-page-and.md` | Modify | Rule-4 amendment note (issue #28) |
| `CLAUDE.md` | Modify | IMPL-0007 classifier bullet updated |
| `deploy/README.md` | Modify | Rollout note (per OQ-4) |
| `api/openapi.yaml` / `api/README.md` | Modify | Only if OQ-3 answers `b` |

## Testing Plan

- [x] Unit: `TestBuildPagesClassifier` proves the type-dir README publishes
      nothing while every other rule's fixture is byte-identical in outcome.
- [x] Unit: log-behavior assertion per OQ-1 (no-Warn vs Warn).
- [x] Integration: e2e onboard proves no page row, 404 on the path, no
      search hit; the existing disable-at-HEAD case continues to prove the
      desired-state deletion machinery this rollout leans on.
- [x] Full `just ci` + `just test-integration` as regression gates.

## Rollout

No migration: reconcile is desired-state, so each repo's next ingest deletes
its existing type-README page rows and `syncIndex` purges them from
Meilisearch. A re-ingest requires a fresh push per repo (webhook
delivery-GUID dedup makes GitHub redelivery a no-op) or a manual `-onboard`
nudge — the same natural-refresh story as IMPL-0003/0005/0008. Until then,
stale rows keep serving; nothing breaks, they're just the duplication this
change removes.

## Follow-ups

- **`buildDocuments` still warns on the type-dir README.** Surfaced by the
  Phase 2 e2e run: `buildDocuments` selects blobs by `path.Dir` matching a
  type dir (not by `IsDoczFile`), so `docs/<type>/README.md` reaches
  `ParseFrontmatter`, returns `ErrNoFrontmatter`, and logs `skipping doc
  without frontmatter` on **every ingest of every docz-managed repo**. This
  is pre-existing — unchanged by IMPL-0009, which only silenced the pages
  side — but it is the same "Warn fires on correct configuration" problem
  OQ-1a rejected, so silencing the pages half alone leaves the operator's
  log unchanged. It also means a genuine stray in a type dir is now
  reported **twice** (once by each pipeline) for the same file.
  Deliberately out of scope here: the doc fenced `buildDocuments` off, and
  touching the documents pipeline carries more blast radius than a page
  classifier fix. Worth its own small change.

  **RESOLVED (2026-09-02)**, in the follow-up branch
  `fix/ingest-type-dir-readme-log-noise`: the `ErrNoFrontmatter` warn is
  now gated on `doczdoc.IsDoczFile(path.Base(blob.Path))`, so only a file
  whose name follows the docz convention (`^\d+-.*\.md$`) is reported. The
  README and other page candidates go quiet, and the double-report is gone
  because `buildPages` remains the single reporter of genuine strays. The
  change is **logging only** — the `ParseFrontmatter` result still decides
  what becomes a document, so a non-convention file with valid frontmatter
  ingests exactly as before.

## Open Questions

### 1. Where does the type-dir README land after the carve-out is gone?

**Answered `1a` (2026-09-01).** The README is still fetched at ingest and
is not an error — it just never becomes a page row. The type surface at
`/:owner/:repo/<type>` belongs to the doc-types side (the site builds it
from the docs/types data); the duplication under `/pages/<dir>` is exactly
what goes away, so no Warn: it's expected content, not a stray.

With the carve-out deleted, a type dir's `README.md` falls through to
rule 4's remaining arms: it fails `IsDoczFile`, so untouched code would hit
the stray-file **skip + Warn**. But this README is not a stray — `docz
update` generates an index-table `README.md` in every type dir, so
virtually every docz-managed repo would log a Warn per type dir per ingest,
forever.

- **a. Silently skip `README.md` in type dirs; keep the Warn for everything
  else.** (Recommendation.) One extra `base == readmeName` check in the
  rule-4 block. The Warn keeps meaning "likely a mistake" (DESIGN-0011
  rule 3); a docz-generated artifact sitting exactly where docz puts it is
  the opposite of a mistake. Log noise that fires on correct configuration
  trains operators to ignore the Warn that matters.
- **b. Let it fall into the stray skip+Warn unmodified.** Smallest diff
  (pure deletion), and arguably "not published" deserves a trace. Cost: a
  permanent per-type-dir Warn on every ingest of every well-formed repo.
- **Other:** _____

### 2. Narrow the fetch to stop downloading type-dir READMEs?

**Answered `2a` (2026-09-01).** One authoritative parse, one place that
decides: the fetch keeps its deliberately dumb "every `.md` under
`docs_dir`" over-fetch, and the classifier — which holds the fully parsed
config and already knows the type dirs — is the single point of exclusion.
Teaching the download step about `types:` would replicate classifier logic
in a second place that can drift, to save one tiny README per type dir per
ingest.

`classifyTree` fetches every `.md` under `docs_dir` when the api block is
enabled, so type-dir READMEs are still downloaded and then classified to
nothing.

- **a. Leave the fetch alone.** (Recommendation.) The fetch hint layer
  (`apiHint`) is deliberately one-field and cannot see enabled types; the
  cost is one small blob per type dir per ingest; and this repo has an
  explicit precedent — "narrow blob fetches are deferred (a fetch-cost
  optimization only)" (Phase 5 push handling). Widening the hint to parse
  `types:` re-creates the authoritative-parse-in-two-places risk the hint
  design avoids.
- **b. Teach the fetch hint the enabled type dirs and skip their READMEs.**
  Saves one blob request per type dir; costs a second partial config parse
  in `githubapp` and a new drift surface between hint and authoritative
  classification.
- **Other:** _____

### 3. Spec surface: bump for the behavior change?

**Answered `3a` (2026-09-01).** The schema doesn't change — the rows just
stop showing up for the site to render under pages; the type surface stays
the doc-types side's business. No spec change, no version bump.

Neither `api/openapi.yaml` nor `api/README.md` ever stated the type-dir
README rule; the schemas are untouched and every remaining page kind serves
identically.

- **a. No spec change, no version bump.** (Recommendation.) The contract
  describes shapes and route semantics, both unchanged; which rows exist is
  repo content. Issue #28 itself scopes this as "no schema or spec-surface
  change". The PR body + DESIGN-0004 amendment carry the behavioral note.
- **b. Editorial patch bump (1.4.1 → 1.4.2)** adding a sentence to the
  pages descriptions ("enabled type directories publish no pages").
  Consumers get the note through the spec they vendor, at the cost of a
  version bump with zero wire delta.
- **Other:** _____

### 4. Operator-facing rollout note?

**Answered `4a` (2026-09-01).**

Existing deployments keep serving stale type-README page rows until each
repo's next ingest.

- **a. One short note in `deploy/README.md`.** (Recommendation.) Mirrors
  the IMPL-0003/0008 natural-refresh precedent: says rows disappear on each
  repo's next push (or `-onboard` nudge), no backfill job exists. Cheap,
  and it's the doc operators actually read when output looks stale.
- **b. No note.** The stale rows are harmless duplication and age out on
  their own; the precedent notes have accumulated three times already.
- **Other:** _____

## Dependencies

- None on docz — the classifier change is entirely in this repo; the docz
  DESIGN-0011 clause-2 amendment is documentation-only and can land before
  or after.
- docz-site's fixture/nav/redirect follow-ups depend on this shipping, not
  the reverse.

## References

- [issue #28] — the ask, with the docz-site duplication story
- [DESIGN-0004] — the six-rule classifier this amends (rule 4)
- [IMPL-0007] — the build-out that introduced the carve-out (Phases 2/4)
- docz DESIGN-0011 clause 2 — the upstream premise being amended
- donaldgifford/docz-site DESIGN-0004 / IMPL-0005 — the consumer surfaces
  that make the published README a duplicate

[issue #28]: https://github.com/donaldgifford/docz-api/issues/28
[DESIGN-0004]: ../design/0004-consume-the-docz-v120-api-block-pages-landing-page-and.md
[IMPL-0007]: 0007-consume-the-docz-v120-api-block-pages-landing-page-and.md
