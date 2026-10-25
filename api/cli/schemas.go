// Package cli holds the JSON Schemas for the docz CLI's machine-readable
// outputs (DESIGN-0024 §3).
//
// Five documents are governed: `docz inspect --format json`,
// `docz validate --format json` (with and without --fix), `docz create
// --format json`, `docz list --format json`, and the review record
// docz-review writes. Each schema is draft 2020-12 with
// additionalProperties false on every object, which is the producer's view:
// the contract test in cmd/ validates the CLI's real output against these
// bytes, so an added, renamed, or removed field fails it. A consumer derives
// an open variant by relaxing additionalProperties and ignores fields it does
// not know.
//
// A schema's $id names its major version and never changes. An additive
// field bumps the file's $version; a removed, renamed, or retyped one bumps
// schema_version, and the previous file is kept under its own $id.
package cli

import (
	"embed"
	"fmt"
)

// The schema_version each governed output carries. The list output is the
// exception: it is a bare array frozen by the parity suite, so it has
// nowhere to put one.
const (
	SchemaVersionInspect  = 1
	SchemaVersionValidate = 1
	SchemaVersionCreate   = 1
	SchemaVersionReview   = 1
)

// Names lists the governed schemas, in the order the design tabulates them.
var Names = []string{"inspect", "validate", "create", "list", "review"}

//go:embed *.schema.json
var schemas embed.FS

// Schema returns the embedded bytes of the named schema, such as "inspect".
//
// The bytes are a fresh copy each call, so a caller may keep or modify them.
func Schema(name string) ([]byte, error) {
	b, err := schemas.ReadFile(name + ".schema.json")
	if err != nil {
		return nil, fmt.Errorf("cli schema %q: %w", name, err)
	}

	return b, nil
}
