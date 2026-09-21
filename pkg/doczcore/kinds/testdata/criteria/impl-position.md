#### Success Criteria

- `make ci` is green and the `test/consumer` calls that existed at v1.2.2
  compile unchanged — nothing in the five frozen packages changed shape.
- `go test ./pkg/doczcore/docparse/... ./pkg/doczcore/kinds/... ./pkg/doczcore/validate/... ./pkg/impl/... ./pkg/rfc/... ./pkg/adr/... ./pkg/design/... ./pkg/investigation/...`
  passes, fuzz seed corpora included.
- Every embedded template validates clean against its skeleton, the
  derivation test binds each template to its skeleton, and each type
  package parses its own rendered template with every field present.
- Every `.orig.md` fixture parses through inference to the same facts as
  its migrated sibling with `Inferred` set, and each package's heading
  table equals `SpecFromTemplate` over its embedded template.
- `go test ./pkg/doczcore/toc/...` passes with the golden byte-identical
  after the `Regions` re-point.
- The `docwrite` status and checktask goldens pass unchanged through the
  path wrappers.
- `go test ./pkg/doczcore/ -run 'TestLayer'` passes.
- `make parity` is green against `build/bin/docz` with marker lines as the
  only delta.
- `make test-consumer` exercises `validate`, `kinds`, and all five type
  packages from outside the module.
