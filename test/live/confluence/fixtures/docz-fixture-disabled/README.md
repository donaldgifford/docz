# docz-fixture-disabled

A live fixture for docz-api's Confluence export (IMPL-0024). Its contents
are generated from
[docz's `test/live/confluence/fixtures/docz-fixture-disabled`](https://github.com/donaldgifford/docz/tree/main/test/live/confluence/fixtures/docz-fixture-disabled)
and force-pushed by `just confluence-fixtures-push`, so anything changed
here is reset on the next push.

## Expected

- An ingest enqueues no export, so the status stays `never`.
- `docz-api -export donaldgifford/docz-fixture-disabled` records
  `disabled`, writing nothing.
