---
id: INV-0009
title: "Expose the indexed updated timestamp on search hits"
status: Concluded
author: Donald Gifford
created: 2026-09-12
---
<!-- markdownlint-disable-file MD025 MD041 -->

# INV 0009: Expose the indexed updated timestamp on search hits

**Status:** Concluded
**Author:** Donald Gifford
**Date:** 2026-09-12

<!--toc:start-->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
  - [F1 — The value has been indexed since Phase 3; no reindex is needed](#f1--the-value-has-been-indexed-since-phase-3-no-reindex-is-needed)
  - [F2 — Three read-path sites drop it, not the two the issue names](#f2--three-read-path-sites-drop-it-not-the-two-the-issue-names)
  - [F3 — Page hits do carry a timestamp; the issue assumes they do not](#f3--page-hits-do-carry-a-timestamp-the-issue-assumes-they-do-not)
  - [F4 — What the stamp means: ingest-observed change time, not commit time](#f4--what-the-stamp-means-ingest-observed-change-time-not-commit-time)
  - [F5 — RFC3339 string is the right wire type; the zone needs pinning](#f5--rfc3339-string-is-the-right-wire-type-the-zone-needs-pinning)
  - [F6 — Contract mechanics: required property, minor bump, one stale README line](#f6--contract-mechanics-required-property-minor-bump-one-stale-readme-line)
  - [F7 — docz-site lights up with no change, on the string form only](#f7--docz-site-lights-up-with-no-change-on-the-string-form-only)
  - [F8 — The sort follow-on is cheap but has a ranking-rule catch](#f8--the-sort-follow-on-is-cheap-but-has-a-ranking-rule-catch)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
  - [Change list](#change-list)
  - [Test evidence](#test-evidence)
  - [Follow-ups (not part of this change)](#follow-ups-not-part-of-this-change)
- [Open questions](#open-questions)
  - [1. Should page hits emit their real timestamp or an empty string?](#1-should-page-hits-emit-their-real-timestamp-or-an-empty-string)
  - [2. How should the spec describe what the stamp means?](#2-how-should-the-spec-describe-what-the-stamp-means)
  - [3. Should the timezone be pinned to UTC on both endpoints?](#3-should-the-timezone-be-pinned-to-utc-on-both-endpoints)
  - [4. Which spec version does a new required response property take?](#4-which-spec-version-does-a-new-required-response-property-take)
  - [5. Does the sort parameter ride along or stay a separate ask?](#5-does-the-sort-parameter-ride-along-or-stay-a-separate-ask)
- [References](#references)
<!--toc:end-->

## Question

Issue #34 asks for `searchDocs` hits to carry the `updated_at` the search
index already stores, as an RFC3339 string matching `Document.updated_at`.
Three concrete questions to settle before the change is made:

1. Is the issue's plan (decode the field, add it to `SearchHit`, add an
   additive schema property, no reindex) complete and correct as written?
2. What does the value actually mean once it is on the wire — is it a
   timestamp a directory listing can honestly label "Updated"?
3. What is the exact wire shape (type, zone, empty convention, page hits)
   that keeps `SearchHit.updated_at` and `Document.updated_at` the same
   logical field?

## Hypothesis

The issue is right in shape: the value is already in Meilisearch, so this is
a serialization change with no ingest or migration work. Expected wrinkles:
the read path may drop the field at more than the two sites the issue names,
the "pages have no timestamp" claim needs checking against `repo_pages`, and
the value is a Postgres row time rather than a git commit time (INV-0003 F3
already flagged that for the lifecycle rail), so the spec wording has to say
what the number is.

## Context

**Triggered by:** issue #34 (`searchDocs: expose the already-indexed
updated_at on SearchHit`), filed from the docz-site side.

docz-site's directory page (DESIGN-0005 there, sixth and eighth amendments)
followed the RFD-index shape and wanted a dated column. Its sixth amendment
records that `SearchHit` has no date field, so the repo took the slot; the
eighth amendment moved the repo beside the doc id and gave the right-hand
column to an updated stamp — rendered as an em dash today, because the
generated `SearchHit` type has no timestamp property. The site shipped
`src/lib/updatedAt.ts` in v0.7.0: `hitUpdatedAt` reads the property
defensively so the column lights up the day docz-api sends it.

On the docz-api side, DESIGN-0001's index record has carried `updated_at`
(Unix seconds) since Phase 3 and declared it sortable, but its search wire
example never listed it on a hit. INV-0003 F3 noted the column is "ingest
row time — wrong for display" in the context of a commit-dated lifecycle
rail. This investigation is the bridge: it checks the issue's plan against
the code as it stands at v0.9.0 and pins the wire semantics.

## Approach

1. Read the index schema, the indexer mapping, and the index settings
   (`internal/search/types.go`, `internal/ingest/indexmap.go`,
   `internal/search/client.go`) to confirm what is stored today.
2. Trace the read path (`internal/search/search.go`) from the Meilisearch
   request through `rawHit` and `decodeHits` to `SearchHit`, listing every
   site that would have to change.
3. Check the page side: `repo_pages` schema, `toIndexPage`, and the pages
   DTOs, against the issue's "pages have no timestamp" claim.
4. Establish the value's semantics from the SQL: when `updated_at` is
   written (`UpsertDocument`, `UpsertRepoPage`) and what the content-hash
   gate in `store.reconcileDocuments` does to it.
5. Compare with the existing `Document.updated_at` serialization
   (`internal/httpapi/dto.go`) and the spec (`api/openapi.yaml`), including
   pgx's timestamptz scan zone.
6. Read the consumer: docz-site `src/lib/updatedAt.ts`,
   `src/routes/directory.tsx`, its `CLAUDE.md` note, and DESIGN-0005's
   amendments, to confirm what shape it accepts.
7. Size the sort follow-on the issue explicitly defers.

## Environment

| Component | Version / Value |
| --------- | --------------- |
| docz-api | `main` @ `63d326e` (v0.9.0) |
| OpenAPI contract | `api/openapi.yaml` `info.version: 1.4.2` |
| meilisearch-go | `v0.36.3` (`SearchRequest.AttributesToRetrieve`, `.Sort`) |
| Meilisearch (integration tests) | `getmeili/meilisearch:v1.12` |
| pgx | `v5.10.0` (`pgtype.Timestamptz`, `ScanLocation` unset) |
| kin-openapi (contract test) | `v0.144.0` |
| docz-site | `v0.7.1`; `src/lib/updatedAt.ts` since v0.7.0 |
| index settings | searchable `title,body`; filterable `repo,repo_id,type,status,author,source`; sortable `created,updated_at`; ranking `words,typo,proximity,attribute,sort,exactness` |

## Findings

### F1 — The value has been indexed since Phase 3; no reindex is needed

The index record has always carried the stamp. `search.IndexDoc` declares
`UpdatedAt int64` with the `updated_at` JSON tag (`internal/search/types.go:33`),
and both indexer mappings populate it from the Postgres row:
`toIndexDoc` (`internal/ingest/indexmap.go:47-49,64`) and `toIndexPage`
(`:73-76,85`). `EnsureIndex` lists it as a sortable attribute
(`internal/search/client.go:53`), and the search integration corpus seeds
every record with a value (`search_integration_test.go:93-125`).

Consequence: every record already in a live index has the field. The change
is read-side only — the issue's "no indexing change, no reindex, no
migration" holds.

### F2 — Three read-path sites drop it, not the two the issue names

The issue points at `rawHit` (no field) and `decodeHits` (no copy). There is a
third site upstream of both, and it is the one that matters most:

```go
// internal/search/search.go:44
AttributesToRetrieve: []string{"source", "repo", "doc_id", "type", "title", "path", "status", "author", "body"},
```

`Search` sets an explicit retrieve list, and `updated_at` is not on it, so
Meilisearch never returns the attribute. Adding the field to `rawHit` alone
would decode to zero on every hit and the new wire field would render as `""`
everywhere — a silent no-op that the unit tests (which fake the searcher)
would not catch. The full site list is:

| Site | File | What changes |
| ---- | ---- | ------------ |
| retrieve list | `internal/search/search.go:44` | add `"updated_at"` |
| decode target | `internal/search/search.go:80-90` (`rawHit`) | add `UpdatedAt int64 \`json:"updated_at"\`` |
| copy + convert | `internal/search/search.go:100-121` (`decodeHits`) | `UpdatedAt: formatUnix(r.UpdatedAt)` |
| wire struct | `internal/search/types.go:55-65` (`SearchHit`) | add `UpdatedAt string \`json:"updated_at"\`` |
| contract | `api/openapi.yaml:773-810` (`SearchHit`) | add the property + `required` entry, bump `info.version` |

Only the integration tests hit a real Meilisearch, so the proof that the
retrieve list is right belongs in `search_integration_test.go` (assert a
non-empty stamp on a hit) and in the e2e search test.

### F3 — Page hits do carry a timestamp; the issue assumes they do not

The issue says page records "have no timestamp, so they emit `""`", and
docz-site's demo fixtures follow suit ("page hits send `""` — nothing in the
contract dates a published page"). The index disagrees:

- `repo_pages.updated_at TIMESTAMPTZ NOT NULL DEFAULT now()`
  (`internal/store/migrations/20260828000000_add_repo_pages.sql:18`);
- `UpsertRepoPage` writes `now()` on insert and on conflict-update
  (`internal/store/queries/pages.sql:10,18`);
- `toIndexPage` copies it onto the record (`internal/ingest/indexmap.go:73-76`).

So once the read path stops dropping the field, page hits will carry a real
stamp unless the decoder blanks it deliberately. The "nothing in the
contract dates a page" half is true of the **pages endpoints**: `pageDTO` and
`pageSummaryDTO` (`internal/httpapi/dto.go:53-65`) expose `path`, `title`,
`git_sha`, and `raw_md` only, so a page hit would carry a timestamp its own
detail endpoint does not. That asymmetry is the decision in OQ-1. Either
choice is safe for the site: `formatUpdatedStamp` renders any RFC3339 string
and `""` yields the em dash.

### F4 — What the stamp means: ingest-observed change time, not commit time

`updated_at` is set by the database, never by the fetcher:

```sql
-- internal/store/queries/documents.sql:4-21
INSERT INTO documents (..., updated_at) VALUES (..., now())
ON CONFLICT (repo_id, doc_id) DO UPDATE SET ..., updated_at = now();
```

and the reconcile only reaches that statement through the content-hash gate
(`internal/store/reconcile.go:133-136`): a document whose bytes are unchanged
is skipped, so its row and its stamp stay put. The semantics that fall out:

- **First onboard** stamps every document with the onboard time, regardless
  of when it was last edited in git.
- **Thereafter** a document's stamp moves only when its content changes, to
  the moment the worker ingested that change — push time plus the debounce
  window, in practice.
- **A fresh database** (new deployment, offboard + re-onboard) restamps
  everything at the new onboard time. The Meilisearch record is rebuilt from
  the row, so the index follows.
- Frontmatter edits count as content (the hash is over the raw bytes);
  metadata-only reconcile changes do not exist separately.
- **Addendum (2026-09-12, found while designing the sort):** Postgres
  `now()` is `transaction_timestamp()`, and `ReconcileRepo` is one
  transaction, so every document and page a reconcile touches gets the
  **identical** stamp — a first onboard makes a whole repo tie on
  `updated_at`. DESIGN-0005 carries the consequence (an implicit secondary
  sort key).

This is exactly the value `Document.updated_at` has served since Phase 2
(`internal/httpapi/dto.go:166,183`), so exposing it on hits introduces no new
meaning — it surfaces an existing one on a second endpoint. It is not the git
commit time; INV-0003 F3 already scoped commit-dated history as separate,
larger work (per-path `ListCommits` at onboard, push-payload harvest, or the
hybrid). For a directory "Updated" column the honest label is "last changed
as seen by docz-api", and the spec description should say so rather than
inherit the current terse `'RFC3339, or "" when unset.'` (OQ-2).

### F5 — RFC3339 string is the right wire type; the zone needs pinning

The issue's argument stands: `Document.updated_at` is already an RFC3339
string (`dto.go:78`, `nullTimestamp` at `:214-219`; spec `:767-769`), and
docz-site's `hitUpdatedAt` accepts **only** a string — `typeof value ===
"string" ? value : ""` — so a raw integer would be ignored and the column
would stay dashed. Two details the issue does not cover:

- **Precision matches.** The index holds Unix seconds; `time.RFC3339` (not
  `RFC3339Nano`) also renders whole seconds. Converting with
  `time.Unix(sec, 0).UTC().Format(time.RFC3339)` loses nothing relative to
  what `Document.updated_at` shows for the same row.
- **Zone does not match by default.** pgx v5 scans a `timestamptz` via
  `time.Unix(...)` and applies `ScanLocation` only when one is configured
  (`pgtype/timestamptz.go:270-275`); we set none, so `Document.updated_at`
  renders in the **process's local zone**. In the distroless image `TZ` is
  unset, so production prints `Z`, but a `just run` on a laptop prints
  `-04:00`. The new search formatter should call `.UTC()` explicitly so the
  suffix is `Z` regardless of host, and `nullTimestamp` can be normalized
  the same way in the same pass — same instant, so it is editorial, not a
  wire change (OQ-3).
- **Empty convention.** `formatUnix(0)` must return `""` to match the wire's
  "not applicable" convention; `Valid` is always true on a `NOT NULL`
  column, so `0` only arises from a hand-built record, but the contract must
  still say what it means.

### F6 — Contract mechanics: required property, minor bump, one stale README line

`SearchHit` is `additionalProperties: false` with an explicit `required`
list (`api/openapi.yaml:780-781`), so the kin-openapi contract test fails
the moment the Go struct emits a field the spec lacks — the drift detector
working as designed. The property therefore lands in both `properties` and
`required` (the Go struct has no `omitempty`, so the field is always
present).

Precedent for the version: `1.4.0` added `source` and `path` to `SearchHit`
**and** to its `required` list as a **minor** bump (`api/README.md:48-51`).
`api/README.md:37-38` lists "a newly required field or header" under
**major**, which reads as a request-side rule (a client that omits it
breaks); a new required *response* property gives clients more than they
asked for and breaks nothing. `1.4.2 → 1.5.0` follows the precedent (OQ-4).

Adjacent drift found while reading: `api/README.md:43` says `Current:
1.4.1` while the spec has been `1.4.2` since PR #32 (the `groups`
description). The same PR that bumps to `1.5.0` should correct the README's
current-version paragraph.

The contract fixture (`internal/httpapi/openapi_contract_test.go:57-73`,
`contractSearcher`) should set a real RFC3339 value so the schema's string
type is exercised rather than satisfied by the zero value.

### F7 — docz-site lights up with no change, on the string form only

Verified against `donaldgifford/docz-site` at `v0.7.1`:

- `src/routes/directory.tsx:125-127` renders `UpdatedCell` from
  `hitUpdatedAt(hit)` → `formatUpdatedStamp(iso)`, an absolute two-line
  stamp in the reader's zone (locale pinned `en-US`); `formatRelativeTime`
  exists for surfaces that want relative.
- `hitUpdatedAt` is the defensive read described above; the generated
  `SearchHit` type will gain the property only when the site re-vendors
  `1.5.0`, at which point the cast can go (a site follow-up, not a blocker).
- Its `CLAUDE.md` and DESIGN-0005 both cite the docz-api file/line for the
  ask, so the two repos agree on the mechanism.
- The site's fixtures assume `""` on page hits (F3). If OQ-1 emits real page
  stamps, page rows in the directory simply gain a date; nothing in the
  site branches on source for this column.

### F8 — The sort follow-on is cheap but has a ranking-rule catch

The issue defers a `sort=` parameter. Sizing it so the follow-on starts
informed:

- meilisearch-go's `SearchRequest` has `Sort []string` (`types.go:650`), and
  `updated_at` / `created` are already sortable (F1). Plumbing is a
  `SearchParams.Sort` field, an allowlist validator in `searchDocs`
  (`internal/httpapi/search.go`), a spec query parameter, and a contract
  test case.
- **The catch:** our ranking rules are `words, typo, proximity, attribute,
  sort, exactness` (`client.go:54-56`). Meilisearch applies `sort` at its
  position in that list, so with a query present relevance dominates and the
  sort only breaks ties among equally relevant hits. A "newest first"
  directory view (empty query, filters only) sorts cleanly because every hit
  ties on relevance; a "newest first" *search* would not behave as a user
  expects unless `sort` is moved ahead of the relevance rules.
- **The placement is cheaper than it looks.** The `sort` ranking rule is
  inert unless the request carries a `sort` parameter, so moving it to the
  front of the list changes the order of **sorted requests only** — an
  unsorted search keeps pure relevance either way. The choice is therefore
  about what a requested sort *means*: a tie-break within relevance (current
  position) or a total order over the matches (first position, the
  GitHub-issues convention). Applying it is a settings update through the
  idempotent `EnsureIndex`, not a reindex. Decided in OQ-5.
- **`created` sorts as a string.** The index stores it as `YYYY-MM-DD`, which
  orders lexicographically as chronologically; page records carry `""`, so
  they sort first ascending and last descending. Acceptable for a
  doc-oriented "newest" view, worth a line in the spec description.

## Conclusion

**Answer:** Yes — the change is feasible exactly as a read-side serialization
change, with three corrections to the issue's plan.

1. The plan is incomplete by one site: `AttributesToRetrieve` must include
   `updated_at` or the decode is a silent no-op (F2). It needs no reindex
   (F1).
2. The stamp is the time docz-api last ingested a content change for the
   record, not the git commit time; it is the same value `Document.updated_at`
   already serves, and the spec should describe it as such (F4).
3. Page hits have a real timestamp in the index; whether to emit it is a
   decision, not a given (F3). RFC3339 string is correct and should be pinned
   to UTC (F5); the version is a minor bump by precedent, and the README's
   current-version line is already one release stale (F6).

## Recommendation

Ship the issue as a single small fix PR, amended per the findings.

### Change list

1. `internal/search/search.go` — add `"updated_at"` to `AttributesToRetrieve`;
   add `UpdatedAt int64` to `rawHit`; copy via a new `formatUnix` in
   `decodeHits`:

   ```go
   // formatUnix renders Unix seconds as RFC3339 in UTC, or "" for zero — the
   // wire's not-applicable convention, matching Document.updated_at.
   func formatUnix(sec int64) string {
       if sec == 0 {
           return ""
       }
       return time.Unix(sec, 0).UTC().Format(time.RFC3339)
   }
   ```

2. `internal/search/types.go` — `UpdatedAt string \`json:"updated_at"\`` on
   `SearchHit`, with the doc comment stating the ingest-observed semantics.
3. `internal/httpapi/dto.go` — per OQ-3, `.UTC()` in `nullTimestamp` so both
   endpoints print `Z`.
4. `api/openapi.yaml` — `SearchHit.properties.updated_at` + `required`, with
   the OQ-2 wording mirrored onto `Document.updated_at`; `info.version`
   `1.5.0`.
5. `api/README.md` — current-version paragraph: `1.5.0` (and note that
   `1.4.2` was the `groups` editorial bump).
6. `CLAUDE.md` — one line under the Phase 3 search bullets recording the
   retrieve-list gotcha, so a future attribute does not repeat F2, and one
   for the `sort` ranking-rule placement (F8).
7. **Sort parameter** (OQ-5): `SearchParams.Sort string`; `searchDocs` reads
   `sort=` and accepts exactly `updated_at:desc`, `updated_at:asc`,
   `created:desc`, `created:asc` (the two sortable attributes), rejecting
   anything else with `400 {"error":"invalid sort"}` — a silently ignored
   sort is F8's confusion by another route, and the lenient `offset`/`limit`
   precedent is about defaults, not typos; `Search` passes it as
   `SearchRequest.Sort`; `EnsureIndex` moves `sort` to the **front** of the
   ranking rules so a requested sort is a total order over the matches
   (unsorted searches are unaffected, F8); spec query parameter `sort` with
   that enum, still under the `1.5.0` bump (additive).

### Test evidence

- Unit: `formatUnix` table (zero → `""`; a value → `...Z`); `decodeHits`
  copies the field.
- Contract: `contractSearcher` returns a real stamp; the kin-openapi test
  validates it against the new schema.
- Integration: `search_integration_test.go` asserts a non-empty RFC3339
  `UpdatedAt` on a hit (proves the retrieve list); the e2e search test's
  wire struct gains `updated_at` and asserts it parses as RFC3339 after a
  real onboard.
- Sort: the integration corpus already has strictly increasing `UpdatedAt`
  values (`search_integration_test.go:93-125`), so `sort=updated_at:desc`
  with an empty query asserts the exact reverse order, and the same sort
  with a query asserts a total order (proves the ranking-rule placement);
  a unit test pins the allowlist + `400`.
- `just lint-openapi` stays 100/100.

### Follow-ups (not part of this change)

- **Pages endpoints** — add `updated_at` to `PageSummary`/`Page` so a page's
  detail endpoint is not poorer than its search hit (OQ-1 emits it on hits).
- **docz-site** — re-vendor `1.5.0`, drop the defensive cast in
  `hitUpdatedAt`, update the fixtures per OQ-1, and pass
  `sort=updated_at:desc` as the directory's default order.
- **Commit-dated history** — remains INV-0003 F3; this change does not
  preempt it, and its spec wording (OQ-2) leaves room for a later
  `last_commit_at`.

## Open questions

Each question lists lettered options; **a** is the recommendation. Enter a
different choice under "Other". **All five were decided on 2026-09-12** —
1–4 as recommended, 5 against the recommendation; the decision line under
each question is authoritative and the Recommendation above reflects it.

### 1. Should page hits emit their real timestamp or an empty string?

- **a. Emit the real page stamp (recommended).** The data is indexed and has
  the same semantics as a document's; blanking it would be a deliberate
  omission the contract then has to explain. The site renders it with no
  change; only its demo fixtures assume `""`.
- **b. Emit `""` on page hits**, as the issue proposes. Keeps parity with the
  pages endpoints, which have no timestamp today, and matches docz-site's
  fixtures.
- **c. Emit it on hits and add `updated_at` to the `Page`/`PageSummary` DTOs
  in the same bump.** Fully uniform, but grows a one-file fix into a
  two-surface change.
- Other: \_\_\_\_\_

**Decision (2026-09-12): a.** Page hits emit their real stamp; the pages
DTOs stay a follow-up.

### 2. How should the spec describe what the stamp means?

- **a. Say what it is (recommended).** Describe `updated_at` on both
  `SearchHit` and `Document` as "when docz-api last ingested a content change
  for this record (RFC3339, UTC); the first ingest stamps every record at
  onboard time; not the git commit time; `""` when unset." Honest for a
  directory column, and leaves a later commit-dated field its own name.
- **b. Keep the current terse wording** (`'RFC3339, or "" when unset.'`)
  verbatim on the new property, as the issue drafts it. Smallest diff; the
  semantics stay undocumented on both endpoints.
- **c. Hold the field until commit-dated history exists** (INV-0003 F3c), so
  the column never shows an ingest artifact. Blocks the site's column on
  work that is not scheduled.
- Other: \_\_\_\_\_

**Decision (2026-09-12): a.** Both `SearchHit.updated_at` and
`Document.updated_at` describe the ingest-observed semantics.

### 3. Should the timezone be pinned to UTC on both endpoints?

- **a. `.UTC()` in the new search formatter and in `nullTimestamp`
  (recommended).** Both endpoints then print a `Z` suffix on every host;
  same instant, so `Document` consumers see an editorial change only.
- **b. Only the search formatter.** Leaves `Document.updated_at` following
  the process zone (already `Z` in the container).
- **c. Leave both to the process zone.** No code beyond the issue's; local
  runs print an offset.
- Other: \_\_\_\_\_

**Decision (2026-09-12): a.** UTC on both endpoints.

### 4. Which spec version does a new required response property take?

- **a. `1.5.0`, minor (recommended).** Matches the `1.4.0` precedent
  (`source`/`path` were added to `required` as a minor bump); fix the stale
  `Current: 1.4.1` line in `api/README.md` in the same PR and clarify that
  the README's "newly required field" major rule is about request fields.
- **b. `2.0.0`, major**, reading `api/README.md:37-38` literally. Signals a
  re-vendor to the site, but no client is broken by an extra response field.
- Other: \_\_\_\_\_

**Decision (2026-09-12): a.** `1.5.0`; the sort parameter (OQ-5) is
additive and rides under the same bump.

### 5. Does the sort parameter ride along or stay a separate ask?

- **a. Separate ask (recommended).** As #34 says. The ranking-rule placement
  (F8) is a design decision with query-wide effects and deserves its own
  issue; this INV records the finding so that issue starts informed.
- **b. Ship `sort=updated_at:desc|asc` in the same PR** with the current
  ranking rules (tie-break only). Cheap, but a "newest first" search that
  visibly is not newest-first is a support question waiting to happen.
- Other: \_\_\_\_\_

**Decision (2026-09-12): same PR (b, amended).** The docz-site discovery
list is meant to default to newest-first — most recently updated, or newest
created — so the timestamp without the sort leaves that page half-built.
The amendment is the F8 placement finding: since the `sort` ranking rule
only acts on requests that pass `sort`, it moves to the **front** of the
ranking rules so a requested sort is a total order (b's tie-break caveat
goes away) while unsorted searches keep pure relevance. Allowlist is the
four `field:direction` tokens over `updated_at` and `created`; unknown
values `400`. Details in the Recommendation change list, item 7.

## References

- Issue #34 — `searchDocs: expose the already-indexed updated_at on SearchHit`
- `internal/search/search.go` — `AttributesToRetrieve` (:44), `rawHit`
  (:80-90), `decodeHits` (:100-121)
- `internal/search/types.go` — `IndexDoc.UpdatedAt` (:33), `SearchHit`
  (:55-65)
- `internal/search/client.go` — sortable attributes + ranking rules (:51-57)
- `internal/ingest/indexmap.go` — `toIndexDoc` (:42-66), `toIndexPage`
  (:72-87)
- `internal/store/queries/documents.sql` (:4-21), `pages.sql` (:6-18) —
  `updated_at = now()`; `internal/store/reconcile.go` — content-hash gate
  (:133-136, :190-193)
- `internal/store/migrations/20260828000000_add_repo_pages.sql:18` —
  `repo_pages.updated_at NOT NULL`
- `internal/httpapi/dto.go` — `documentDTO.UpdatedAt` (:78), `nullTimestamp`
  (:214-219), page DTOs (:53-65)
- `api/openapi.yaml` — `Document.updated_at` (:767-769), `SearchHit`
  (:773-810); `api/README.md` — versioning rules (:29-59), stale current
  line (:43)
- `internal/httpapi/openapi_contract_test.go` — `contractSearcher` (:52-73);
  `internal/search/search_integration_test.go` — seeded `UpdatedAt` corpus
  (:86-128); `internal/e2e/search_integration_test.go` — wire struct
  (:118-128)
- pgx `v5.10.0` `pgtype/timestamptz.go:270-275` — scan zone follows
  `ScanLocation` (unset → process local)
- meilisearch-go `v0.36.3` `types.go:650` — `SearchRequest.Sort`
- docz-site `v0.7.1` — `src/lib/updatedAt.ts` (`hitUpdatedAt`,
  `formatUpdatedStamp`, `formatRelativeTime`), `src/routes/directory.tsx`
  (`UpdatedCell`, :125-127), `CLAUDE.md` (the updated column note),
  DESIGN-0005 sixth (2026-09-10) and eighth (2026-09-11) amendments
- INV-0003 F3 — `updated_at` as "ingest row time — wrong for display" and
  the commit-dated history options
- DESIGN-0001 — index record with `updated_at` (Unix seconds) + sortable
  attributes; search wire example without a hit timestamp
