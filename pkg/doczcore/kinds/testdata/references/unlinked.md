## References

- INV-0001: Wiki Init Template and Init Enabled Fix — earlier dogfooding pass
  that surfaced related issues
- PR #30 (`fix/version-and-mkdocs-extensions`) — instance of three-way defaults
  drift
- PR #31 (`fix/update-skips-disabled-types`) — instance of `ValidTypes()`
  iteration vs config-map disconnect
- Uber Go Style Guide — naming, error handling, initialism, defer, named returns
- Effective Go — package layout, interface design, error wrapping
- `cmd/` source — all command files reviewed
- `internal/config/config.go` — config struct, defaults, validation
- `internal/document/{document,create,time}.go` — frontmatter, create, time
- `internal/index/index.go` — scanning, table generation, README splicing
- `internal/template/{template,embed}.go` — template resolution and rendering
- `internal/toc/toc.go` — table-of-contents generation
- `internal/wiki/{wiki,titles,mkdocs}.go` — MkDocs nav generation
- Review agents used: `go-development:go-architect`, `go-development:go-style`,
  `go-development:go-performance`, and a general-purpose idiomatic Go review
