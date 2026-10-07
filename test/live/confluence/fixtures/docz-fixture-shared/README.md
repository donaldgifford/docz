# docz-fixture-shared

A live fixture for docz-api's Confluence export (IMPL-0024). Its contents
are generated from
[docz's `test/live/confluence/fixtures/docz-fixture-shared`](https://github.com/donaldgifford/docz/tree/main/test/live/confluence/fixtures/docz-fixture-shared)
and force-pushed by `just confluence-fixtures-push`, so anything changed
here is reset on the next push.

## Expected

- `succeeded`, folder `docz-fixture-shared` in `DOCZ` beside
  `docz-fixture-basic`'s: two repositories, one space, no title collision
  (both have an `RFC-0001`).
- The home page's body is `docs/index.md`; `docz-fixture-shared:
  Contributing` sits in the folder (`api_pages: true`).
