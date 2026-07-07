// Package doczcontract guards docz-api against silent drift in the pinned
// docz parsing library (github.com/donaldgifford/docz, pinned in go.mod).
//
// It intentionally contains no runtime code. Its tests compile against the
// public pkg/doczcore/config and pkg/doczcore/document surface (DESIGN-0007,
// requirements R1–R5) and assert the behavior the ingest pipeline relies on:
// manifest loading, type resolution by name/alias/id_prefix, frontmatter
// parsing, the docz filename convention, and DocEntry.Content population.
//
// If a future docz bump removes or changes that surface, these tests fail
// here — cheaply and unambiguously — rather than deep inside internal/ingest.
package doczcontract
