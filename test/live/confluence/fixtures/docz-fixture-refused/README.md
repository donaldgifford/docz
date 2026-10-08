# docz-fixture-refused

A live fixture for docz-api's Confluence export (IMPL-0024). Its contents
are generated from
[docz's `test/live/confluence/fixtures/docz-fixture-refused`](https://github.com/donaldgifford/docz/tree/main/test/live/confluence/fixtures/docz-fixture-refused)
and force-pushed by `just confluence-fixtures-push`, so anything changed
here is reset on the next push.

## Expected

- `refused`, reason `space NOTALLOWED on https://dgifford06.atlassian.net
  is not allowed on this server`. Nothing is written to Confluence, and the
  job is not retried.
