# Archive

Planning records of the repositories that moved into this one. Each tree keeps
its own ID namespace: inside a tree, an ID means that repository's document, not
docz's.

| Tree | Repository | Moved in by | Lives on as |
| --- | --- | --- | --- |
| [`api/`](api/README.md) | docz-api | IMPL-0019 (v2.0.0-beta.3) | `cmd/docz-api/`, `internal/`, `api/`, `charts/docz-api/` |
| [`ui/`](ui/README.md) | docz-site | IMPL-0020 (v2.0.0-beta.4) | `ui/`, `charts/docz-site/` |

Both trees are excluded from the wiki (`wiki.exclude`) and the `api:` listing
(`api.exclude`) by the one `archive` entry each list carries, and `docz update`
and `docz validate` never see them because they sit outside every type
directory.
