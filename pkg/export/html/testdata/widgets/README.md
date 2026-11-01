# Widget validation corpus

Each `<widget>-<case>.yaml` is the body of a `docz-<widget>` fence: a
`flow`, `stepper`, or `scorecard` block. `widgets_corpus_test.go`
decodes each one with `decodeWidget` as if its fence opened on line 1,
so the block's first line is document line 2, and writes what it finds
(regenerate with `go test ./pkg/export/html/ -update`):

- `<case>.json.golden`, for a valid block, is the JSON island the
  exported page carries.
- `<case>.errors.golden`, for an invalid one, is each error at its
  document line, one per line, as the caution callout lists them.

docz-site's port of the validator (`ui/src/markdown/widgets/`) reads the
same files, so the two cannot drift without a test failing
(IMPL-0030 Open Question 9). A `<widget>-parse-<case>.yaml` is a block
the YAML parser itself refuses; its message is yaml.v3's own wording,
which no JavaScript parser reproduces, so docz-site is held to the same
single error at the same line and not to the text.
