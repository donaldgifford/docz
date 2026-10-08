# docz-fixture-layout-page

A live fixture for docz-api's Confluence export (IMPL-0024). Its contents
are generated from
[docz's `test/live/confluence/fixtures/docz-fixture-layout-page`](https://github.com/donaldgifford/docz/tree/main/test/live/confluence/fixtures/docz-fixture-layout-page)
and force-pushed by `just confluence-fixtures-push`, so anything changed
here is reset on the next push.

## Expected

- `refused`, reason `layout: page is for the CLI; the server writes the
  folder layout`. Nothing is written.
