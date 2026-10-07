# docz-fixture-folder-clash

A live fixture for docz-api's Confluence export (IMPL-0024). Its contents
are generated from
[docz's `test/live/confluence/fixtures/docz-fixture-folder-clash`](https://github.com/donaldgifford/docz/tree/main/test/live/confluence/fixtures/docz-fixture-folder-clash)
and force-pushed by `just confluence-fixtures-push`, so anything changed
here is reset on the next push.

## Expected

- Names `docz-fixture-basic`'s folder. After that repository has exported,
  this one is `failed` with a configuration error naming
  `sync.confluence.folder` (the folder belongs to another repository), and
  is not retried. `docz-fixture-basic`'s pages are untouched.
- Onboarded **first**, it would create the folder itself and
  `docz-fixture-basic` would be the one to fail, so keep the order.
