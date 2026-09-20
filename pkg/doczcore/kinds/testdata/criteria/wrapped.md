#### Success Criteria

- `git diff --stat pkg/doczcore/docwrite/testdata/golden/status/` is empty
  after the refactor — the locator generalization is provably
  byte-preserving.
- `TestSetStatus_*` pass unchanged; `TestSetUpdated_Golden`, the error
  table, and `FuzzSetUpdated` (≥ 1M execs locally, seed corpus committed)
  green.
- A document written by `SetUpdated` round-trips through
  `document.ParseFrontmatter` with `Updated` set and no other field changed.
- `make ci` green.
