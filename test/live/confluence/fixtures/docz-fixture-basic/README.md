# docz-fixture-basic

A live fixture for docz-api's Confluence export (IMPL-0024). Its contents
are generated from
[docz's `test/live/confluence/fixtures/docz-fixture-basic`](https://github.com/donaldgifford/docz/tree/main/test/live/confluence/fixtures/docz-fixture-basic)
and force-pushed by `just confluence-fixtures-push`, so anything changed
here is reset on the next push.

## Expected

- `GET /api/v1/repos/donaldgifford/docz-fixture-basic/confluence` is
  `succeeded`, folder `docz-fixture-basic` at the top of `DOCZ`.
- Pages: `docz-fixture-basic` (home), `docz-fixture-basic: RFCs`,
  `docz-fixture-basic: ADRs`, and one per document, e.g.
  `docz-fixture-basic: RFC-0002: Render everything`.
- RFC-0002's ADR link is a page link, its script link a GitHub blob URL,
  and `../missing.md` is reported unresolved. The mermaid diagram renders
  through the Mermaid diagrams viewer.
- Onboard this repository **before** `docz-fixture-folder-clash`.
- The hand-driven scenarios (rename, remove, edit in Confluence, inline
  comment, disable) run against this repository; see docz's
  `test/live/confluence/README.md`.
