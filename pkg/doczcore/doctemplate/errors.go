package doctemplate

import (
	"errors"
	"fmt"
)

// The sentinels this package returns, distinguishable with errors.Is.
//
// Both exist because the caller's next move differs: a missing template is a
// repo-configuration problem a person fixes by writing one, and a missing
// schema is a document claiming a skeleton nobody shipped. Matching on the
// sentinel rather than on the message is what lets cmd/ pick an exit code and
// validate emit a finding without either of them parsing prose.
var (
	// ErrNoTemplate means no configured, on-disk, or embedded body template
	// exists for a document type.
	//
	// This is issue #92: a custom type with no template used to fail with the
	// embedded FS's "file does not exist", which named neither the type nor
	// the path a person would have to create. The wrapped message names both.
	ErrNoTemplate = errors.New("no template for document type")

	// ErrNoSchema means no on-disk or embedded marker skeleton exists under
	// the given name (DESIGN-0015 §3).
	//
	// An illegal name yields this as well as ErrBadSchemaName, so a caller
	// that only wants "there is no schema" tests one sentinel while validate
	// can still tell the two apart.
	ErrNoSchema = errors.New("no schema")

	// ErrBadSchemaName means the name does not match the schema-name grammar
	// [a-z0-9][a-z0-9_-]*.
	//
	// The grammar is enforced before the filesystem, because the name comes
	// from a document's own frontmatter and may say anything at all. It is
	// narrow on purpose: a name is a filename stem under templates/schema/, so
	// a separator, a dot segment, or an upper-case letter would escape the
	// directory, resolve differently on two machines, or not resolve at all.
	// This is the finding validate reports as schema.name.
	ErrBadSchemaName = errors.New("not a valid schema name")
)

// badSchemaName is the error both lookups return for a name outside the
// grammar. One helper, so the two cannot drift: a caller matching on either
// sentinel gets the same answer from EmbeddedSchema and ResolveSchema.
func badSchemaName(name string) error {
	return fmt.Errorf("%w: %w: %q", ErrNoSchema, ErrBadSchemaName, name)
}
