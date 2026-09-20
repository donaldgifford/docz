#### Success Criteria

- [x] `make ci` green
- [x] A new contributor can add a doc type by editing one file plus two
      template files (CONTRIBUTING.md "Adding a New Built-In Document
      Type" and DEVELOPMENT.md walkthrough both pin this)
- [x] Tests run in parallel — all internal/* tests, top-level and
      subtests, call `t.Parallel()`. `go test -race -shuffle=on
      -count=3 ./...` is green. cmd/ tests stay serial intentionally
      until the package-level globals are removed (out of scope here)
- [x] No `cmd/` package-level globals remain except the threaded
      `*Runner` and the bound Cobra flag values
      (`cfgFile`, `docsDir`, `verbose`, `logLevel`, `logFormat`, and
      per-command flag vars like `createStatus`); the bound globals
      are CLI-flag plumbing rather than runtime state
