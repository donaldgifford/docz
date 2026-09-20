package repo

import (
	"context"
	"slices"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/docwrite"
)

// StatusOptions is what a caller can vary about a status mutation.
//
// One field today, and a struct anyway: the alternative is a bool parameter
// that every future option has to be appended after, and adding a field to a
// struct breaks nobody.
type StatusOptions struct {
	// DryRun computes the result without writing the file. The result is
	// otherwise the same, so a caller can show what would happen and then do
	// it with the identical call.
	DryRun bool
}

// StatusResult is the outcome of a status mutation.
//
// Old and New are always filled, whether or not anything was written, because
// "already at Approved" and "Draft -> Approved" are the two things a caller
// wants to say and both need the pair. Changed is the discriminator between
// them.
type StatusResult struct {
	// ID is the document's frontmatter id, echoed back so a caller
	// assembling a report does not have to carry the argument alongside the
	// result.
	ID string
	// Type is the canonical type name, not the token the caller passed.
	Type string
	// Path is the document's path relative to Repo.Root.
	Path string

	// Old is the status the document carried before the call.
	Old string
	// New is the status that was requested, which is also the status the
	// document carries afterwards.
	New string

	// Changed reports whether the file was written. It is false when Old and
	// New are equal and false on a dry run — the two reasons nothing was
	// written — so a caller that only wants to know whether the bytes moved
	// reads this one field.
	Changed bool
}

// SetStatus sets a document's frontmatter status, validated against the
// type's configured lifecycle.
//
// The order is fixed and matters (DESIGN-0014 §2.8): the document is located
// first, then the status is validated. That is what gives the CLI its two
// exit codes — a missing id is a lookup failure and an unconfigured status is
// a validation failure — and swapping the two would report "Abandoned is not
// a valid status" for a document that does not exist, which tells the user to
// fix the wrong thing.
//
// Nothing is written when the document already carries the requested status.
// The no-op short-circuit lives here rather than only in the caller
// (DESIGN-0014 Open Question 7) so that every consumer gets it: a CI job that
// sets a status on every run should not produce a commit per run. DryRun
// reaches the same result by the same path and stops at the same place.
//
// A write failure is a WriteError naming the document. The underlying error
// is wrapped, so errors.Is still finds docwrite.ErrUnsupportedLineEndings or
// docwrite.ErrStatusFieldMissing through it and a caller can keep mapping
// those to their own exit codes.
func (r *Repo) SetStatus(
	ctx context.Context,
	typeName, id, status string,
	opts StatusOptions,
) (StatusResult, error) {
	entry, err := r.FindIn(ctx, typeName, id)
	if err != nil {
		return StatusResult{}, err
	}

	// Statuses are read off the resolved type rather than the caller's token,
	// so an alias and a canonical name validate against the same list.
	allowed := r.Cfg.Types[entry.Type].Statuses
	if !slices.Contains(allowed, status) {
		return StatusResult{}, &InvalidStatusError{
			Type:    entry.Type,
			Status:  status,
			Allowed: allowed,
		}
	}

	res := StatusResult{
		ID:   entry.ID,
		Type: entry.Type,
		Path: entry.Path,
		Old:  string(entry.Status),
		New:  status,
	}

	if res.Old == res.New || opts.DryRun {
		return res, nil
	}

	// FindIn already refused on a cancelled context, so there is no check
	// here: a run cancelled before the call never reaches the write, and one
	// cancelled during a single-file mutation has nothing to stop between.
	if _, err := docwrite.SetStatus(r.Path(res.Path), res.New); err != nil {
		return res, &WriteError{Path: res.Path, Err: err}
	}

	fireFileWritten(ctx, res.Path, FileDocument)

	res.Changed = true

	return res, nil
}
