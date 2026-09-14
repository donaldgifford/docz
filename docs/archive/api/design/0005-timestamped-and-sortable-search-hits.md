---
id: DESIGN-0005
title: "Timestamped and sortable search hits"
status: Implemented
author: Donald Gifford
created: 2026-09-12
---
<!-- markdownlint-disable-file MD025 MD041 -->

# DESIGN 0005: Timestamped and sortable search hits

**Status:** Implemented
**Author:** Donald Gifford
**Date:** 2026-09-12
**Landed:** 2026-09-13 — implemented by IMPL-0010 in one PR (spec
`1.5.0`). One prediction was corrected against a real Meilisearch: an
empty sort value places a record last in **both** directions, not first
ascending. See the correction block under "The sort parameter".

<!--toc:start-->
- [Overview](#overview)
- [Goals and Non-Goals](#goals-and-non-goals)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Background](#background)
  - [What INV-0009 established](#what-inv-0009-established)
  - [New finding: one reconcile, one timestamp](#new-finding-one-reconcile-one-timestamp)
- [Detailed Design](#detailed-design)
  - [Read path: retrieve, decode, format](#read-path-retrieve-decode-format)
  - [Wire struct and the zone pin](#wire-struct-and-the-zone-pin)
  - [The sort parameter](#the-sort-parameter)
  - [Ranking rules: sort becomes a total order](#ranking-rules-sort-becomes-a-total-order)
  - [Tie-breaking with an implicit secondary key](#tie-breaking-with-an-implicit-secondary-key)
  - [The source filter](#the-source-filter)
  - [OpenAPI contract: schema, parameter, a first 400](#openapi-contract-schema-parameter-a-first-400)
- [API / Interface Changes](#api--interface-changes)
- [Data Model](#data-model)
- [Testing Strategy](#testing-strategy)
- [Migration / Rollout Plan](#migration--rollout-plan)
- [Open Questions](#open-questions)
  - [1. Expose the created date on hits as well?](#1-expose-the-created-date-on-hits-as-well)
  - [2. Sort token shape on the wire?](#2-sort-token-shape-on-the-wire)
  - [3. Implicit secondary sort key for ties?](#3-implicit-secondary-sort-key-for-ties)
  - [4. Server-side default order when no sort is given?](#4-server-side-default-order-when-no-sort-is-given)
  - [5. Add a source filter parameter in the same bump?](#5-add-a-source-filter-parameter-in-the-same-bump)
- [Follow-ups](#follow-ups)
- [References](#references)
<!--toc:end-->

## Overview

`GET /api/v1/search` hits gain the timestamp the index has carried since
Phase 3 — `updated_at`, as an RFC3339 UTC string matching `Document.updated_at`
— and the endpoint gains a `sort` query parameter over the index's two
sortable attributes (`updated_at`, `created`), with the `sort` ranking rule
moved to the front so a requested sort is a total order over the matches.
Both are read-side: no ingest change, no migration, no reindex. The OpenAPI
contract bumps `1.4.2 → 1.5.0` (additive), and docz-site's directory column
lights up with no site change while its default "newest first" order becomes
one query parameter.

This implements **INV-0009** with its five decisions (OQ 1–4 as recommended,
OQ 5 amended to ship the sort in the same PR), which in turn closes
**issue #34**.

## Goals and Non-Goals

### Goals

- **Date every hit** with the same logical field `Document.updated_at`
  already serves — same type (RFC3339), same empty convention (`""`), same
  semantics (when docz-api last ingested a content change), pinned to UTC on
  both endpoints so the suffix is `Z` on every host.
- **Make "newest first" a query, not a client sort**: `sort=updated_at:desc`
  (and `created`, both directions) orders the whole result set server-side,
  so pagination via `offset`/`limit` stays correct.
- **A requested sort is a total order.** With a query present, relevance
  must not silently override the order the caller asked for.
- **Strictly additive** under the DESIGN-0002 regime: new property, new
  parameter, new documented `400`; `additionalProperties: false` and the
  kin-openapi contract test prove nothing else moved.
- **Honest wording in the contract** for what the stamp means (INV-0009
  OQ-2a), so a later commit-dated field can take its own name.

### Non-Goals

- **No commit-dated history.** The stamp is the ingest-observed change time;
  git commit dates remain INV-0003 F3 (per-path `ListCommits`, push-payload
  harvest, or the hybrid), untouched here.
- **No timestamp on the pages endpoints.** `Page`/`PageSummary` stay as they
  are; adding `updated_at` there is a recorded follow-up (INV-0009 OQ-1a).
- **No change to unsorted ranking.** A request without `sort` ranks exactly
  as today; the ranking-rule move is inert without the parameter.
- **No free-form sort expressions.** The parameter accepts an enumerated set
  of tokens over the two sortable attributes; adding a sortable attribute is
  a settings change plus a token, not a parser.
- **No new store queries, columns, or ingest paths.**

## Background

### What INV-0009 established

- The index record has carried `updated_at` (Unix seconds) since Phase 3, on
  documents and pages alike, and `EnsureIndex` declares it sortable beside
  `created` — every record in a live index already has the value (INV-0009
  F1, F3).
- The read path drops it at **three** sites: the explicit
  `AttributesToRetrieve` list, the `rawHit` decode target, and the
  `decodeHits` copy (F2). The first is the one the issue missed and the one
  the unit tests cannot see, because they fake the searcher.
- The value is written by Postgres (`updated_at = now()` on insert and on
  conflict-update) and only through the content-hash gate, so it moves when
  bytes change and never otherwise (F4). It is not the commit time; the same
  value has served on `Document.updated_at` since Phase 2.
- RFC3339 string is the right wire type — docz-site's `hitUpdatedAt` accepts
  only a string — and pgx scans `timestamptz` in the process zone, so the
  formatter must pin UTC (F5).
- The `sort` ranking rule acts only on requests that pass `sort`; moving it
  to the front changes sorted requests only (F8, as corrected).
- The contract precedent for a new required response property is a minor
  bump (`1.4.0`), and `api/README.md`'s current-version line is a release
  stale (F6).

### New finding: one reconcile, one timestamp

Postgres `now()` is `transaction_timestamp()` — the start of the current
transaction — and `store.ReconcileRepo` runs the whole repo in one
transaction. So every document and page upserted by one reconcile carries
the **identical** `updated_at`, to the microsecond. Consequences for a sort:

- After a repo's first onboard, all of its documents tie on `updated_at`.
- Across repos, a newer onboard's documents sort ahead of an older one's
  **as a block**, regardless of when anything was written.
- The index stores whole seconds, so ties are exact, not approximate.

A `sort=updated_at:desc` over a freshly onboarded registry is therefore
"repos in onboard order, documents within a repo in Meilisearch's internal
order". That is correct but not useful, which is why the design carries a
secondary key (OQ-3) and why `created` — the author's frontmatter date, which
does vary within a repo — is the other sortable attribute worth exposing.

## Detailed Design

### Read path: retrieve, decode, format

All three drop sites are in `internal/search/search.go`:

```go
// Search: the retrieve list names every attribute a hit needs; an attribute
// missing here never reaches the decoder, whatever rawHit declares.
AttributesToRetrieve: []string{
    "source", "repo", "doc_id", "type", "title", "path",
    "status", "author", "created", "updated_at", "body",
},
```

```go
type rawHit struct {
    // ...existing fields...
    Created   string       `json:"created"`    // OQ-1
    UpdatedAt int64        `json:"updated_at"` // Unix seconds, the index schema
    Formatted rawFormatted `json:"_formatted"`
}
```

```go
// decodeHits
hits[i] = SearchHit{
    // ...existing fields...
    Created:   r.Created,                // OQ-1
    UpdatedAt: formatUnix(r.UpdatedAt),
    Snippet:   r.Formatted.Body,
}

// formatUnix renders Unix seconds as RFC3339 in UTC, or "" for zero — the
// wire's not-applicable convention, the same shape Document.updated_at uses.
func formatUnix(sec int64) string {
    if sec == 0 {
        return ""
    }
    return time.Unix(sec, 0).UTC().Format(time.RFC3339)
}
```

Precision is preserved end to end: the index holds whole seconds and
`time.RFC3339` renders whole seconds, so a document's stamp reads the same
on both endpoints. `created` needs no conversion — the index already stores
the `YYYY-MM-DD` string `Document.created` serves, or `""`.

### Wire struct and the zone pin

`internal/search/types.go`:

```go
// SearchHit is one result row with a highlighted body snippet. ...
// UpdatedAt is when docz-api last ingested a content change for the record
// (RFC3339, UTC) — the same value Document.updated_at serves, not the git
// commit time; "" when unset. Created is the document's frontmatter date
// (YYYY-MM-DD) and "" on page hits.
type SearchHit struct {
    // ...existing fields...
    Created   string `json:"created"`    // OQ-1
    UpdatedAt string `json:"updated_at"`
    Snippet   string `json:"snippet"`
}
```

Page hits emit their real stamp (INV-0009 OQ-1a): `toIndexPage` already
copies `repo_pages.updated_at`, so nothing blanks it.

`internal/httpapi/dto.go` pins the other endpoint (OQ-3a):

```go
func nullTimestamp(t pgtype.Timestamptz) string {
    if t.Valid {
        return t.Time.UTC().Format(time.RFC3339)
    }
    return ""
}
```

Same instant, `Z` suffix on every host — an editorial change for `Document`
consumers, covered by the `1.5.0` bump's description update rather than a
type change.

### The sort parameter

**Ownership.** The `search` package owns the index's sortable attributes
(`EnsureIndex`), so it owns the accepted tokens; `httpapi` only maps the
parse failure to a status. The two lists are one variable, so they cannot
drift:

```go
// internal/search/client.go
// sortableAttributes are the index attributes a Search may order by; the
// sort tokens below are derived from this list, so a new sortable attribute
// is one edit here plus its tokens.
var sortableAttributes = []string{"created", "updated_at"}
```

```go
// internal/search/types.go
// Sort tokens accepted by Search, "<attribute>:<direction>" over the index's
// sortable attributes. The set is the contract's enum for the sort query
// parameter; anything else is ErrInvalidSort.
const (
    SortUpdatedDesc = "updated_at:desc"
    SortUpdatedAsc  = "updated_at:asc"
    SortCreatedDesc = "created:desc"
    SortCreatedAsc  = "created:asc"
)

// ErrInvalidSort reports a sort token outside the accepted set.
var ErrInvalidSort = errors.New("invalid sort")

// ParseSort validates a sort token. "" is the unsorted default and parses to
// ""; an accepted token parses to itself; anything else is ErrInvalidSort.
func ParseSort(s string) (string, error)
```

`SearchParams` gains `Sort string` (a validated token or `""`). `Search`
passes it through:

```go
if p.Sort != "" {
    req.Sort = sortKeys(p.Sort) // requested key + implicit secondary (OQ-3)
}
```

**Handler** (`internal/httpapi/search.go`): parse before searching, reject
with the existing envelope:

```go
sort, err := search.ParseSort(q.Get("sort"))
if err != nil {
    writeError(w, http.StatusBadRequest, "invalid sort")
    return
}
```

Rejecting rather than ignoring is deliberate (INV-0009, recorded with OQ-5):
a silently ignored sort is exactly the "newest-first that visibly is not"
confusion the ranking-rule move exists to prevent. The lenient
`parseNonNegInt` precedent for `offset`/`limit` is about *defaults* (empty
→ 0, and the search layer applies its own limit), not about swallowing
typos; a sort has no meaningful default to fall back to.

**Shape.** One parameter, one token, `<attribute>:<direction>` — the
Meilisearch-native spelling, enumerated in the spec so the generated client
gets a union type (OQ-2a). Direction is required in the token; there is no
bare `sort=updated_at`, because a default direction would be one more
convention to document and the enum makes every accepted value explicit.

### Ranking rules: sort becomes a total order

`EnsureIndex` today applies `words, typo, proximity, attribute, sort,
exactness`. At that position Meilisearch orders by relevance first and uses
the requested sort only to break ties among equally relevant hits — so
`q=logging&sort=updated_at:desc` would return the *most relevant* match
first, not the newest. The new order:

```go
RankingRules: []string{
    "sort", "words", "typo", "proximity", "attribute", "exactness",
},
```

Two facts make this safe:

- The `sort` rule is **inert without a `sort` parameter**. An unsorted
  request ranks exactly as today; only sorted requests change, and today no
  caller can send one.
- With `sort` first, relevance still applies **within** ties — two documents
  with the same stamp still order by words/typo/proximity — which is what the
  secondary key (OQ-3) then makes deterministic.

The change is a settings update through the idempotent `EnsureIndex` at
startup; `waitTask` blocks until Meilisearch has applied it, which at homelab
index sizes is sub-second. No reindex.

### Tie-breaking with an implicit secondary key

Per the new finding, a primary sort on `updated_at` ties across every
document a reconcile touched. `sortKeys` appends a fixed secondary key the
caller never sees (OQ-3a):

| Requested | Sent to Meilisearch |
| --------- | ------------------- |
| `updated_at:desc` | `["updated_at:desc", "created:desc"]` |
| `updated_at:asc` | `["updated_at:asc", "created:asc"]` |
| `created:desc` | `["created:desc", "updated_at:desc"]` |
| `created:asc` | `["created:asc", "updated_at:asc"]` |

So "most recently updated" resolves an onboard-time tie toward the newest
frontmatter date, and "newest created" resolves a same-day tie toward the
most recently changed. Remaining ties (same second, same day) fall through
to relevance and then Meilisearch's internal order, which is stable for a
given index state — good enough for pagination at this scale and not worth a
third sortable attribute.

`created` sorts as its stored string: `YYYY-MM-DD` orders lexicographically
as chronologically. Page records carry `""`, and a caller that wants
documents only filters by source (OQ-5).

> **Corrected during implementation (2026-09-13, IMPL-0010 Phase 4).** This
> section originally predicted that the empty `created` on page records
> would sort **first ascending and last descending**, as a plain
> lexicographic order implies. Meilisearch does not do that: it treats an
> empty value as absent for sorting and places such records **last in both
> directions**. The integration test found it, and the parameter
> description in the spec states the real behavior. The observed behavior
> is the more useful one — undated records never crowd the top of a
> "newest first" listing — so only the documentation changed.

### The source filter

While designing the sort a gap surfaced: `source` is filterable and faceted
(since `1.4.0`), but `SearchParams` has no `Source` and the spec lists no
`source` query parameter — a caller cannot ask for documents only, which a
`created` sort makes visible (pages have no `created`). The fix is one
`appendEq(parts, "source", p.Source)` in `buildFilter`, one handler line, one
spec parameter with the `[doc, page]` enum; additive under the same bump.
Whether it rides along is OQ-5.

### OpenAPI contract: schema, parameter, a first 400

`api/openapi.yaml`, `info.version: 1.5.0`:

```yaml
    SearchHit:
      required: [source, repo, doc_id, type, title, path, status, author, created, updated_at, snippet]
      properties:
        # ...existing...
        created:
          type: string
          description: 'Frontmatter date, YYYY-MM-DD; "" on page hits and when unset.'
        updated_at:
          type: string
          description: >
            When docz-api last ingested a content change for this record
            (RFC3339, UTC). The first ingest stamps every record at onboard
            time; the value moves only when the content changes. Not the git
            commit time. "" when unset.
```

The same `updated_at` description replaces `Document.updated_at`'s terse
`'RFC3339, or "" when unset.'` (OQ-2a), so the two spellings of one field
carry one explanation.

```yaml
  /api/v1/search:
    get:
      parameters:
        # ...existing q/repo/type/status/author/offset/limit...
        - name: sort
          in: query
          description: >
            Order the matches; a total order over the result set, with
            relevance breaking ties. Absent, results are ranked by relevance.
            `created` sorts as YYYY-MM-DD, so page hits (which have none)
            sort first ascending and last descending.
          schema:
            type: string
            enum: [updated_at:desc, updated_at:asc, created:desc, created:asc]
      responses:
        "200": ...
        "400":
          $ref: "#/components/responses/BadRequest"
        "401": ...
```

```yaml
  responses:
    BadRequest:
      description: A query parameter is malformed (an unrecognized sort token).
      content:
        application/json:
          schema:
            $ref: "#/components/schemas/Error"
```

This is the first documented `400` on the read surface; the `Error` envelope
already exists, so it is a new `components.responses` entry only. `just
lint-openapi` (vacuum + yamlfmt) must stay at 100/100, which the enum and the
folded descriptions satisfy.

`api/README.md`'s current-version paragraph moves to `1.5.0` and picks up
the `1.4.2` entry it skipped (the `groups` description, PR #32). Its
versioning rule "a newly required field or header" under **major** is
clarified as request-side; a new required *response* property is minor, per
the `1.4.0` precedent (INV-0009 OQ-4a).

## API / Interface Changes

| Surface | Change |
| --- | --- |
| `GET /api/v1/search` response | `SearchHit` + `updated_at` (RFC3339 UTC or `""`), + `created` (`YYYY-MM-DD` or `""`, OQ-1) |
| `GET /api/v1/search` request | + `sort` (enum of four tokens; unknown → `400 {"error":"invalid sort"}`); + `source` filter (OQ-5) |
| `GET /api/v1/repos/.../types/{type}/docs[/{doc_id}]` | `updated_at` now always `Z`-suffixed (same instant; editorial) |
| `api/openapi.yaml` | `SearchHit` properties + `required`; `sort` parameter; `BadRequest` response; `Document.updated_at` wording; `info.version` → `1.5.0` |
| `search.SearchParams` | + `Sort string` (validated token or `""`), + `Source string` (OQ-5) |
| `search` package | + `ParseSort`, `ErrInvalidSort`, the four `Sort*` tokens, `sortableAttributes` shared with `EnsureIndex` |
| Meilisearch index settings | ranking rules reordered to `sort, words, typo, proximity, attribute, exactness` (applied by `EnsureIndex` at startup) |
| `api/README.md`, `CLAUDE.md` | current version + versioning clarification; Phase 3 gotchas for the retrieve list and the `sort` rule placement |
| Existing endpoints / DTOs otherwise | **Unchanged** (contract test enforces) |

## Data Model

None. No migration, no new column, no new query. The Meilisearch record
schema (`IndexDoc`) is unchanged; only the index's **settings** change
(ranking-rule order), applied idempotently at boot. Every record already
carries `updated_at` and `created`, so no reindex.

## Testing Strategy

- **search unit tests** (`internal/search`): `formatUnix` table (zero →
  `""`; a value → `...Z`); `ParseSort` table (`""` → `""`; each token →
  itself; `updated_at`, `UPDATED_AT:desc`, `updated_at:down`, `body:desc` →
  `ErrInvalidSort`); `sortKeys` table (each token → its pair); a guard test
  that every token's attribute is in `sortableAttributes`; `buildFilter`
  gains the `source` case (OQ-5).
- **httpapi unit tests** (`fakeSearcher` captures params): `sort=updated_at:desc`
  reaches the searcher as `Sort`; `sort=bogus` → `400 {"error":"invalid
  sort"}` and the searcher is never called; the wire `updated_at` /
  `created` round-trip from a canned hit.
- **OpenAPI contract test**: `contractSearcher` returns a real RFC3339
  stamp (and `created`) so the schema's string type is exercised; a new
  table case `GET /api/v1/search?q=intro&sort=updated_at:desc` validates the
  request (enum) and the `200`. The `400` is **not** a contract case — an
  out-of-enum `sort` fails kin-openapi's *request* validation before the
  handler runs, which is the spec doing its job; the response envelope is
  covered by the httpapi unit test and the `BadRequest` component by
  `doc.Validate`.
- **search integration test** (real Meilisearch, `//go:build integration`):
  the seeded corpus already has strictly increasing `UpdatedAt` across all
  six records (`search_integration_test.go:93-125`), so:
  - empty query + `updated_at:desc` → the exact reverse seed order (proves
    the retrieve list and the sort plumbing);
  - `q=logging` + `updated_at:desc` → `2_p_99…` (456), `2_ADR-0001` (453),
    `1_RFC-0001` (451): a total order in which the most relevant hit is
    **last** (proves the ranking-rule placement);
  - a hit's `UpdatedAt` is a non-empty RFC3339 string;
  - `created:desc` puts the three page records last (the `""` behavior the
    description promises).
- **e2e** (`internal/e2e`, real Postgres + Meilisearch): after onboarding,
  the search wire struct gains `updated_at` and the test asserts it parses
  as RFC3339 with a `Z` suffix and that `Document.updated_at` for the same
  doc is byte-equal. Ordering by `updated_at` cannot be proven here (one
  reconcile, one stamp — the new finding), so the e2e sort case uses
  `created:desc` over the fixture's distinct frontmatter dates.
- `just lint-openapi` stays 100/100; `just helm-unittest` unaffected (no
  config change).

## Migration / Rollout Plan

1. One PR, one bump: read path + wire struct + zone pin + sort + ranking
   rules + spec `1.5.0` + README/CLAUDE notes, per INV-0009 OQ-5.
2. On deploy, `EnsureIndex` applies the new ranking rules before the server
   accepts traffic (it already blocks on `waitTask`). During a rolling
   update the index briefly serves old and new pods with the **new** rules;
   old pods never send `sort`, and the rule is inert without it, so their
   results are unchanged.
3. `Document.updated_at` switches to a `Z` suffix on hosts whose process
   zone was not UTC — in the distroless image it already was, so production
   sees no byte change there.
4. Pre-existing index records need nothing: they carry both attributes.
5. docz-site, at its own pace: the directory column lights up on deploy
   (the defensive `hitUpdatedAt` read); re-vendoring `1.5.0` gives it the
   typed property, the `sort` enum, and lets it pass `sort=updated_at:desc`
   as the directory default and drop the cast. Its fixtures stop sending
   `""` on page hits.
6. Rollback is a redeploy of the previous image: its `EnsureIndex` restores
   the old ranking order, and the extra response fields simply stop being
   sent (the site's read is defensive).

## Open Questions

Each question is numbered; option **a** is the recommendation, later letters
are alternatives, and **Other** is free-form. INV-0009's five decisions are
taken as given (page stamps emitted; honest wording; UTC on both endpoints;
`1.5.0`; sort ships in this PR with `sort` first in the ranking rules and
unknown tokens rejected with `400`) and are not re-asked here.

**Decisions (2026-09-12): all five, option a.** The Detailed Design cites
`OQ-1` at every `created`-on-hit line, marking the decision that put it
there. Terminology, since it caused confusion in review: a **hit** is one
row of the `hits` array in the `searchDocs` response — one search result,
the `SearchHit` schema. It is Meilisearch's word and the wire field's name;
nothing to do with telemetry. "`updated_at` on hits" is exactly "every
search result carries its last-updated time".

### 1. Expose the created date on hits as well?

- **a (recommended): yes — add `created` beside `updated_at`.** It is one of
  the two sort keys, and a list sorted by a value it does not show is a UX
  trap ("why is this one first?"). The mechanics are identical (retrieve
  list, `rawHit`, `decodeHits`, `SearchHit`, schema), the value is already a
  `YYYY-MM-DD` string in the index — no conversion — and it mirrors
  `Document.created` exactly. Same `1.5.0` bump; docz-site's generated type
  gains one more optional-to-read field.
- **b: `updated_at` only**, the issue's literal scope. Smallest diff; the
  `created` sort still works, the caller just cannot display its key
  without a second request per row.
- Other: \_\_\_\_\_

**→ Decision: 1a.** `created` rides on every hit beside `updated_at`. The
question was only whether the *second* date — the frontmatter `created`,
the author-typed date on the doc — also rides on each search result;
`updated_at` was never in question. The "trap" in **a** is narrow: a list
sorted by `created:desc` whose rows show only `updated_at` gives the
reader no visible reason for the order. Kept deliberately simple; features
under consideration for later may refactor this surface, and a second
string field is the cheapest thing to carry until then.

### 2. Sort token shape on the wire?

- **a (recommended): one parameter, one enumerated token —
  `sort=updated_at:desc`.** Meilisearch-native spelling, four values, an
  OpenAPI `enum` so orval/openapi-typescript emit a union type and the site
  cannot misspell it; `ParseSort` is a set lookup.
- **b: two parameters — `sort=updated_at&order=desc`.** Reads naturally,
  but doubles the validation surface (an `order` without a `sort`, an
  unknown `order`) and the spec cannot express the pairing.
- **c: sign prefix — `sort=-updated_at` / `sort=updated_at`.** JSON:API
  style; compact, but a bare token needs a documented default direction and
  the enum becomes `[-updated_at, updated_at, -created, created]`, which
  generated clients render less readably.
- Other: \_\_\_\_\_

**→ Decision: 2a.** One enumerated `<attribute>:<direction>` token.

### 3. Implicit secondary sort key for ties?

- **a (recommended): yes, fixed and server-side** — `updated_at` sorts fall
  through to `created` in the same direction, and vice versa (table in the
  design). Every document a reconcile touched ties on `updated_at` (the new
  finding), so without this a fresh registry's "newest first" is Meilisearch
  internal order within each repo. The caller sees one token; the pairing is
  an implementation detail the tests pin.
- **b: exactly the requested key, no secondary.** Simplest contract; ties
  resolve by relevance then internal order, which is stable but arbitrary
  and makes the first-onboard listing look unsorted.
- **c: client-supplied multi-key — `sort=updated_at:desc,created:desc`.**
  Most expressive, but the enum goes away (a comma list cannot be
  enumerated), validation becomes a parser, and no consumer has asked for
  it.
- Other: \_\_\_\_\_

**→ Decision: 3a.** Fixed server-side secondary key per the table in the
Detailed Design.

### 4. Server-side default order when no sort is given?

- **a (recommended): none — absent `sort` keeps today's relevance ranking;
  the site passes `sort=updated_at:desc` explicitly.** The contract stays
  "you get what you ask for", the unsorted path is byte-for-byte unchanged
  (the ranking-rule move is inert), and the palette (query-driven) and the
  directory (listing-driven) can differ without server knowledge of which
  is which.
- **b: default to `updated_at:desc` when `q` is empty.** Makes a bare
  listing newest-first with no client change, but introduces a hidden
  branch on query emptiness that the spec has to describe and that changes
  the empty-query facet-count requests the site already sends with
  `limit=0` (harmless, but no longer "unchanged").
- **c: default to `updated_at:desc` always.** Turns every search into a
  recency list unless the caller opts out; wrong for the palette.
- Other: \_\_\_\_\_

**→ Decision: 4a.** No server default; the site passes the sort.

### 5. Add a source filter parameter in the same bump?

- **a (recommended): yes — `source` query parameter, enum `[doc, page]`.**
  One line in `buildFilter`, one in the handler, one spec parameter; the
  attribute is already filterable and faceted. It is the natural companion
  to a `created` sort (pages have no `created`) and to a "newest doc"
  listing, and it is additive under the bump already being taken.
- **b: leave it out**; file it as its own follow-up. Keeps this PR to
  #34's topic plus the sort; callers wanting documents only filter
  client-side and lose accurate `estimated_total_hits` and offsets.
- Other: \_\_\_\_\_

**→ Decision: 5a.** The `source` filter parameter ships in the same bump.

## Follow-ups

- **IMPL doc** for the single-PR build once the open questions are resolved:
  read path → wire struct → sort + ranking rules → spec `1.5.0` → tests →
  README/CLAUDE notes.
- **Pages endpoints**: `updated_at` on `PageSummary`/`Page` (INV-0009
  OQ-1a's deferred half) so a page's detail endpoint is not poorer than its
  search hit; its own minor bump.
- **Commit-dated history** (INV-0003 F3): the spec wording here leaves
  `last_commit_at` (or similar) its own name when that lands.
- **docz-site**: re-vendor `1.5.0`; drop the `hitUpdatedAt` cast; directory
  default `sort=updated_at:desc`; render `created` if OQ-1a; fixtures emit
  real page stamps.
- **Adjacent noise, not blocking**: the Dependabot alert on
  `google.golang.org/grpc` (fixed in `1.83.2`) will trip the Trivy gate on
  the fix PR's CI unless bumped first or alongside.

## References

- Issue #34 — `searchDocs: expose the already-indexed updated_at on SearchHit`
- [INV-0009](../investigation/0009-expose-the-indexed-updated-timestamp-on-search-hits.md)
  — the investigation this design implements (F1–F8, OQ 1–5 decisions)
- [INV-0003](../investigation/0003-docz-site-deferred-features-and-the-docz-api-surface-to-unblock.md)
  — F3, commit-dated history (explicitly out of scope here)
- [DESIGN-0001](0001-docz-api-cross-repo-docz-registry-and-ingestion-service.md)
  — the index record (`updated_at` Unix seconds), sortable attributes, the
  search wire example
- [DESIGN-0002](0002-openapi-contract-for-docz-api-and-the-docz-site.md)
  — the wire-contract regime (spec, contract test, SemVer)
- [DESIGN-0004](0004-consume-the-docz-v120-api-block-pages-landing-page-and.md)
  — the `source` facet and page records in the index (`1.4.0`)
- `internal/search/search.go` — `AttributesToRetrieve`, `rawHit`,
  `decodeHits`, `buildFilter`/`appendEq`
- `internal/search/client.go` — `EnsureIndex` settings (sortable attributes,
  ranking rules)
- `internal/search/types.go` — `IndexDoc`, `SearchParams`, `SearchHit`
- `internal/httpapi/search.go` — `searchDocs`, `parseNonNegInt`;
  `internal/httpapi/handler.go` — `writeError`
- `internal/httpapi/dto.go` — `nullTimestamp`, `documentDTO`
- `internal/store/reconcile.go` — one transaction per reconcile (the
  shared-timestamp finding); `internal/store/queries/documents.sql`,
  `pages.sql` — `updated_at = now()`
- `internal/httpapi/openapi_contract_test.go` — `contractSearcher`;
  `internal/search/search_integration_test.go` — the increasing-`UpdatedAt`
  corpus
- `api/openapi.yaml` — `SearchHit`, `searchDocs` parameters,
  `components.responses`; `api/README.md` — versioning rules
- Meilisearch — ranking rules: the `sort` rule applies only when the search
  carries a `sort` parameter, and its position decides whether sort or
  relevance leads; `sort` accepts multiple `attribute:direction` keys;
  string attributes sort lexicographically
- PostgreSQL — `now()` is `transaction_timestamp()`: one value per
  transaction
- docz-site `v0.7.1` — `src/lib/updatedAt.ts` (`hitUpdatedAt`,
  `formatUpdatedStamp`), `src/routes/directory.tsx` (`UpdatedCell`, the
  searchDocs-driven directory), DESIGN-0005 amendments six and eight
