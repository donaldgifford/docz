// Package doczcontract guards docz-api against silent drift in the pinned
// docz parsing library (github.com/donaldgifford/docz, pinned in go.mod).
//
// It intentionally contains no runtime code. Its tests compile against the
// public pkg/doczcore/config, pkg/doczcore/document, and
// pkg/doczcore/docparse surface (DESIGN-0007, requirements R1–R5, plus R6
// from IMPL-0005, R10 from IMPL-0007, and R11 from IMPL-0008) and assert
// the behavior the ingest pipeline relies on: manifest loading, type
// resolution by name/alias/id_prefix, frontmatter parsing, the docz
// filename convention, DocEntry.Content population; — R6 — the changelog:
// config block (defaults, merge, normalization, enabled-only validation)
// and ParseChangelog's parse shape and ErrNoVersions sentinel; — R10 —
// the api: config block (dormancy, load-time normalization, enabled-only
// validation behind ErrInvalidAPIPath) and docparse.Title; and — R11 —
// the marshaled config shape (json tags mirror yaml tags in name and
// omitempty, upstream DESIGN-0008 R11), which config_snapshot serves to
// every snapshot reader.
//
// If a future docz bump removes or changes that surface, these tests fail
// here — cheaply and unambiguously — rather than deep inside internal/ingest.
package doczcontract
