---
id: INV-0017
title: "docz-api: serve runbook metadata as structured fields"
status: Open
author: Donald Gifford
created: 2026-10-02
---

<!-- markdownlint-disable-file MD025 MD041 -->

# INV-0017: docz-api: serve runbook metadata as structured fields

<!--toc:start-->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
  - [Observation 1: one DTO and one schema serve both endpoints](#observation-1-one-dto-and-one-schema-serve-both-endpoints)
  - [Observation 2: the store has no place for it yet](#observation-2-the-store-has-no-place-for-it-yet)
  - [Observation 3: the content-hash gate blocks the backfill](#observation-3-the-content-hash-gate-blocks-the-backfill)
  - [Observation 4: there is no per-type dispatch in the server](#observation-4-there-is-no-per-type-dispatch-in-the-server)
  - [Observation 5: a search attribute has four drop sites, not one](#observation-5-a-search-attribute-has-four-drop-sites-not-one)
  - [Observation 6: what the API would serve](#observation-6-what-the-api-would-serve)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
  - [1. Where do the fields live?](#1-where-do-the-fields-live)
  - [2. When does the server run runbook.Parse?](#2-when-does-the-server-run-runbookparse)
  - [3. What is the wire shape?](#3-what-is-the-wire-shape)
  - [4. How do existing rows get the fields?](#4-how-do-existing-rows-get-the-fields)
  - [5. What reaches search?](#5-what-reaches-search)
- [References](#references)
<!--toc:end-->

<!--docz:question:start-->
## Question

Should docz-api read a runbook with `runbook.Parse` at ingest and serve its
owner, service, and last verification as structured fields beside the body,
and what does that need in the store, the OpenAPI contract, and the search
index?

<!--docz:question:end-->

<!--docz:hypothesis:start-->
## Hypothesis

Yes, and it is the first type-specific read the server makes: today nothing
under `internal/` imports a type package, and ingest reads frontmatter only.
The parse is the small part. The decisions are where the fields live (a
per-type JSON column that impl and investigation can use next, or columns
for runbook alone), how the server decides which documents to parse, and
how existing rows get the fields when the reconcile's content-hash gate
only rewrites a row whose bytes changed. The contract change is additive, a
minor bump from 1.5.0, because `Document` has `additionalProperties: false`
and the fields are optional.

<!--docz:hypothesis:end-->

<!--docz:context:start-->
## Context

**Triggered by:** issue
[#145](https://github.com/donaldgifford/docz/issues/145), from DESIGN-0019
and IMPL-0022.

docz-api serves a runbook as it serves every document: frontmatter fields
and `raw_md`. A runbook's owner and service are bold lines in its overview,
and its last verification is the first row of a table. docz-site shows them
only by rendering the markdown, and search cannot filter on them. INV-0018
(the step-aware site view) wants the Last Verified row as a badge and lists
this as one of its two ways to get it.

<!--docz:context:end-->

<!--docz:approach:start-->
## Approach

1. Map the document path through the server: `mapDocument` in
   `internal/ingest`, `DocumentInput` and the `documents` table in
   `internal/store`, `documentDTO` in `internal/httpapi`, `Document` in
   `api/openapi.yaml`, and `IndexDoc` in `internal/search`.
2. List what `pkg/runbook` returns that the API would serve.
3. Check how the content-hash gate affects a field added to existing rows.
4. Enumerate the drop sites a new search attribute needs.
5. Resolve the storage, dispatch, wire-shape, backfill, and search
   questions, then write a DESIGN against them.

<!--docz:approach:end-->

<!--docz:environment:start-->
## Environment

| Component | Version / Value |
| --------- | --------------- |
| Spec | `api/openapi.yaml` `info.version: 1.5.0`; `Document` and `SearchHit` both `additionalProperties: false` |
| Store | `documents` columns: id, repo_id, type, doc_id, title, status, author, created, path, git_sha, content_hash, raw_md, updated_at; no JSONB on documents |
| Ingest | `mapDocument(typeName, blob, fm)` from `document.ParseFrontmatter`; no type package imported under `internal/` |
| Search | `retrieveAttributes` of eleven names; filterable `repo, repo_id, type, status, author, source`; facets `repo, type, status, author, source` |
| `pkg/runbook` | `Doc{Service, Owner, LastVerified *Verification{Date, PR, Commit, VerifiedBy []string, Notes}, Procedures, Scenarios, Escalation, …}` |

<!--docz:environment:end-->

<!--docz:findings:start-->
## Findings

### Observation 1: one DTO and one schema serve both endpoints

`documentDTO` (`internal/httpapi/dto.go`) is used for the list and the
single document alike; `toDocumentSummary` leaves `raw_md` out and
`toDocument` fills it. The spec mirrors that with one `Document` schema,
`additionalProperties: false`, and a required list of the eleven columns.
Adding a field therefore means adding an optional property to `Document`,
bumping `info.version` to 1.6.0, and giving the contract test's fixture a
runbook document so the new property is exercised. docz-site's `Document`
type is generated from the spec by orval, so the field reaches the site in
the same PR through `gen-api`, and `gen-api-check` fails until it does.

### Observation 2: the store has no place for it yet

`documents` has no JSONB or metadata column; JSONB appears only on `repos`
(`config_snapshot`, `api_additional_docs`) and `doc_types` (`statuses`,
`aliases`). The nullable JSONB precedent is `api_additional_docs`, which
needed its own sqlc override (`nullable: true` → `json.RawMessage`). A new
column is a fifth migration and a change to `DocumentInput`, the sqlc
queries, and the reconcile's write.

### Observation 3: the content-hash gate blocks the backfill

`reconcileDocuments` rewrites a row only when `ContentHash`, the sha256 of
the blob, changes. A runbook's owner and Last Verified row are in the body,
so a change to them always changes the hash and the gate stays correct for
new writes. But an existing runbook whose body does not change is never
rewritten, so a column added by migration stays NULL for it until someone
edits the file. The index.md rollout (docz-api DESIGN-0003 OQ-4a) accepted
"natural refresh only", but that field is on the repo row, which every
reconcile upserts; documents are gated. Either the hash input includes a
mapper version, so bumping it rewrites every row once, or there is a
one-off forced re-ingest.

### Observation 4: there is no per-type dispatch in the server

No file under `internal/` imports `pkg/rfc`, `pkg/impl`, or any sibling, and
the only type switch is the webhook's switch over GitHub event types. Type
handling is name-based: `typeresolver.go` resolves `{type}` from the
`doc_types` row, and `mapDocType` maps a `TypeConfig`. The CLI's own
precedent is `cmd/validate.go`'s `typeValidator`, the one type-name switch
in the repository, which picks a type package by canonical name. ADR-0002 R7
says a type package reads region kinds and never the type name, which is
true of `runbook.Parse`; it says nothing against a *caller* choosing when to
run it by name, and that is what the CLI does.

### Observation 5: a search attribute has four drop sites, not one

The gotcha docz-api's INV-0009 F2 recorded (issue #34 there) still holds: a new `SearchHit` field must
be added to `IndexDoc`, `rawHit`, `decodeHits`, and `retrieveAttributes`, or
Meilisearch omits it and every faked-searcher test still passes. Filtering
on `owner` or `service` also needs `FilterableAttributes` in `EnsureIndex`
(idempotent at startup) and `facetNames` if it is faceted. Both have bounded
cardinality, a team per runbook and a service per runbook, so faceting them
is cheap. `verified_by` and the date are better left to the document than to
the index for now.

### Observation 6: what the API would serve

From `runbook.Doc`: `Service` and `Owner` as strings, and `LastVerified`
as `{date, pr, commit, verified_by[]}` or absent, since the field is nil
while the template's empty row stands — "a runbook nobody has run has no
verification". `Notes` is prose and stays in the body. Procedures,
scenarios, and steps are richer than the issue asks for; INV-0018 wants step
ids and lines, and the storage choice in question 1 should leave room for
them without a second column.

<!--docz:findings:end-->

<!--docz:conclusion:start-->
## Conclusion

**Answer:** Yes, provisionally. The parse is one import and one call; the
cost is a migration, a backfill the content-hash gate would otherwise block
(Observation 3), an additive 1.6.0 spec change (Observation 1), and five
drop sites in search (Observation 5). The questions below pick the shape;
the DESIGN that follows them is where the investigation concludes.

<!--docz:conclusion:end-->

<!--docz:recommendation:start-->
## Recommendation

Add a nullable per-type JSON column to `documents`, fill it for runbooks at
ingest by canonical type name, serve it as a nested optional `runbook`
object on `Document` at spec 1.6.0, and rewrite every row once by
versioning the hash input. Facet `owner` and `service` in search.

### 1. Where do the fields live?

- a. **One nullable JSONB `metadata` column on `documents`**, holding a
  per-type object (`{"runbook": {…}}`), with the `api_additional_docs`
  sqlc override. impl phases and investigation verdicts go in the same
  column later with no migration. *(recommendation)*
- b. Dedicated columns `owner`, `service`, `last_verified_date`, `…_pr`,
  `…_commit`, `…_by`. Typed and indexable, but runbook-only, and the next
  type adds another migration.
- c. A `document_metadata` side table keyed by document id.
- d. Other.

### 2. When does the server run `runbook.Parse`?

- a. **When the document's canonical type is `runbook`**, in `mapDocument`,
  mirroring `cmd/validate.go`'s single type-name switch. *(recommendation)*
- b. On every document, keeping a non-zero result: faithful to ADR-0002 R7,
  so a custom type carrying runbook regions is served too, at a parse per
  document.
- c. When the document's `schema` resolves to the runbook skeleton.
- d. Other.

### 3. What is the wire shape?

- a. **A nested optional `runbook` object on `Document`**:
  `{owner, service, last_verified: {date, pr, commit, verified_by[]}}`,
  present only for runbooks, so the other five types' documents are
  unchanged and the next type adds its own key. *(recommendation)*
- b. Top-level optional `owner`, `service`, and `last_verified` on
  `Document`, as the issue lists them.
- c. A separate endpoint, `GET …/docs/{doc_id}/runbook`.
- d. Other.

### 4. How do existing rows get the fields?

- a. **Include a mapper version in the content-hash input**, so bumping it
  rewrites every row on each repository's next ingest. One full rewrite,
  no new command. *(recommendation)*
- b. A one-off forced re-ingest after deploy, per repository.
- c. Natural refresh only, as DESIGN-0003 OQ-4a chose for index.md: a
  runbook gets its fields when its body next changes.
- d. Other.

### 5. What reaches search?

- a. **`owner` and `service` filterable and faceted, not searchable**; the
  verification stays off the index. *(recommendation)*
- b. Searchable as well, so a team name matches in the query box.
- c. Nothing in search for now.
- d. Other.

<!--docz:recommendation:end-->

<!--docz:references:start-->
## References

- [#145](https://github.com/donaldgifford/docz/issues/145): this issue;
  [#144](https://github.com/donaldgifford/docz/issues/144) and
  [INV-0018](0018-docz-site-a-step-aware-runbook-view.md): the site view
  that consumes the fields
- [DESIGN-0019](../design/0019-runbook-a-sixth-built-in-document-type-disabled-by-default.md)
  and [IMPL-0022](../impl/0022-runbook-the-sixth-built-in-type-v200-beta6.md)
- [ADR-0002](../adr/0002-docz-is-an-api-package-whose-first-consumer-is-the-cli.md)
  R7: a type package reads region kinds
- docz-api INV-0009 (archived) F2, the `retrieveAttributes` gotcha:
  [`docs/archive/api/investigation/0009-expose-the-indexed-updated-timestamp-on-search-hits.md`](../archive/api/investigation/0009-expose-the-indexed-updated-timestamp-on-search-hits.md)
- [`pkg/runbook/doc.go`](../../pkg/runbook/doc.go),
  [`internal/httpapi/dto.go`](../../internal/httpapi/dto.go),
  [`internal/ingest/mapper.go`](../../internal/ingest/mapper.go),
  [`internal/search/search.go`](../../internal/search/search.go),
  [`api/openapi.yaml`](../../api/openapi.yaml)

<!--docz:references:end-->
