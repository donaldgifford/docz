package doctemplate

import "errors"

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
	// A name that is not a legal schema name yields this too, wrapped with
	// ErrBadSchemaName, so a caller that wants to tell "you asked for a
	// skeleton nobody shipped" from "that is not a name" can, while a caller
	// that only wants "no schema" tests one sentinel.
	ErrNoSchema = errors.New("no schema of that name")

	// ErrBadSchemaName means the name does not match the schema-name grammar
	// [a-z0-9][a-z0-9_-]*.
	//
	// The grammar is enforced here rather than at the filesystem, because the
	// name comes from a document's own frontmatter and may name anything at
	// all. It is narrow on purpose: a name is a filename stem under
	// templates/schema/, so a separator, a dot segment, or an upper-case
	// letter would resolve differently on two machines or not at all. It
	// wraps ErrNoSchema, because a name that cannot resolve has no schema —
	// that is the finding validate reports as schema.name.
	ErrBadSchemaName = errors.New("not a valid schema name")
)
