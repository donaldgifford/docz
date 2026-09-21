package repo

import (
	"fmt"
	"strings"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
)

// The typed errors repo returns. Every failure a caller may branch on
// carries its facts as fields rather than only inside a message, so a
// consumer decides what to do with errors.As instead of matching on
// English (DESIGN-0014 Open Question 9, resolved b).
//
// The messages are the library's, not the CLI's. cmd/ formats its own
// wording from the fields — the type directory in a "not found" line, for
// instance, is a repo-relative path only cmd/ knows how to render — so a
// message changing here is not a CLI behaviour change.

// NotFoundError reports that no document in a type directory carries the
// requested frontmatter id.
//
// Returned by Find, FindIn, and SetStatus. The id comparison is
// case-sensitive (DESIGN-0005 Decision 3), so a caller that wants a
// lenient lookup does its own scan over Scan's result.
type NotFoundError struct {
	Type string
	ID   string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("no %s document with id %q", e.Type, e.ID)
}

// TypeDisabledError reports that a type resolved but is turned off in the
// configuration.
//
// Distinct from UnknownTypeError on purpose: the type exists and the user
// spelled it correctly, so the fix is a config edit rather than a
// different argument. Scan returns this rather than an empty slice,
// which is what cmd/update.go's "type disabled, skipping" branch needs to
// tell the two apart.
type TypeDisabledError struct {
	Type string
}

func (e *TypeDisabledError) Error() string {
	return fmt.Sprintf("document type %q is disabled", e.Type)
}

// ExistsError reports that a path a write would have created is already
// there and no overwrite was requested.
//
// Returned by ExportTemplate without Overwrite. Init does not use it: a
// file already present there is reported as InitSkipped, because
// initialising a repo twice is a normal thing to do.
type ExistsError struct {
	Path string
}

func (e *ExistsError) Error() string {
	return fmt.Sprintf("%s already exists", e.Path)
}

// InvalidStatusError reports a status outside the type's configured set.
//
// Allowed carries the set so a caller can list the valid values without
// reaching back into the config, which is what makes the CLI's two-line
// error reproducible from the error alone.
type InvalidStatusError struct {
	Type    string
	Status  string
	Allowed []string
}

func (e *InvalidStatusError) Error() string {
	return fmt.Sprintf("%q is not a valid status for %s (valid: %s)",
		e.Status, e.Type, strings.Join(e.Allowed, ", "))
}

// UnknownTypeError reports a token that resolves to no document type.
//
// Unwrap returns config.ErrUnknownType, so the frozen sentinel still
// answers errors.Is for every caller written against v1 while a v2
// caller reads Token and Valid off the struct.
type UnknownTypeError struct {
	Token string
	Valid []string
}

func (e *UnknownTypeError) Error() string {
	return fmt.Sprintf("%s %q (valid types: %s)",
		config.ErrUnknownType, e.Token, strings.Join(e.Valid, ", "))
}

// Unwrap returns config.ErrUnknownType so errors.Is keeps working.
func (*UnknownTypeError) Unwrap() error { return config.ErrUnknownType }

// WriteError reports a filesystem failure, naming the path it happened to.
//
// The wrapped error is whatever the writer returned, so a caller can still
// reach fs.ErrPermission or docwrite.ErrUnsupportedLineEndings through
// errors.Is while getting the path from the struct.
type WriteError struct {
	Path string
	Err  error
}

func (e *WriteError) Error() string {
	return fmt.Sprintf("writing %s: %v", e.Path, e.Err)
}

// Unwrap returns the underlying failure.
func (e *WriteError) Unwrap() error { return e.Err }

// unknownType turns config.ValidateType's wrapped-sentinel error into the
// typed shape, so every repo method reports an unresolvable token the same
// way.
//
// The token is the caller's verbatim spelling rather than the normalised
// one, because that is what the user typed and what they need to see.
func (r *Repo) unknownType(token string) error {
	return &UnknownTypeError{Token: token, Valid: r.Cfg.EnabledTypes()}
}

// resolveType canonicalises a user-supplied type token and reports whether
// the type is enabled.
//
// One place for the two checks every method makes, in the order the CLI
// makes them: an unresolvable token is UnknownTypeError, and a resolved but
// switched-off type is TypeDisabledError. Splitting them is the whole point
// — "you typed something wrong" and "you turned this off" have different
// fixes, and the old code returned the same error for both.
func (r *Repo) resolveType(token string) (string, error) {
	typeName, err := r.Cfg.ValidateType(token)
	if err != nil {
		return "", r.unknownType(token)
	}

	if !r.Cfg.Types[typeName].Enabled {
		return "", &TypeDisabledError{Type: typeName}
	}

	return typeName, nil
}
