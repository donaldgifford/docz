package repo

import (
	"context"
	"fmt"
	"path/filepath"
	"time"
	"unicode"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/doctemplate"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/document"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/index"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/toc"
)

// UpdateOptions is the per-invocation input to Update.
//
// A struct with one field rather than a bare bool, because the next flag
// this operation grows should not change the signature of every caller.
type UpdateOptions struct {
	// DryRun computes everything and writes nothing. The reports are
	// filled in either way, so a caller can show exactly what a real run
	// would do.
	DryRun bool
}

// UpdateReport is the result of an Update: one TypeReport per type, in the
// order they were processed.
//
// A cancelled or failed run returns the entries completed so far, so the
// slice is always as long as the work that actually finished.
type UpdateReport struct {
	Types []TypeReport
}

// TypeReport is what happened to one document type.
//
// The two nested reports are the ones the layers below produced, verbatim:
// they name their files by the absolute path the write used, because that
// is the path that was written. A consumer that wants the short form calls
// Repo.RelPath on them. Dir is the exception and is repo-relative, because
// it is the one path this package computes itself rather than passes
// through.
type TypeReport struct {
	// Type is the canonical type name.
	Type string
	// Dir is the type's directory relative to Repo.Root.
	Dir string
	// Docs is the number of documents the scan found.
	Docs int
	// ToC is the table-of-contents pass, or nil when Cfg.TOC.Enabled is
	// false. A non-nil report with nothing in it means the pass ran over a
	// type with no documents, which is a different fact from not running.
	ToC *toc.UpdateReport
	// Index is the README splice outcome.
	Index index.UpdateOutcome
	// Elapsed is the wall time this type took. Cheap profiling data for a
	// consumer that updates a repository of a few thousand documents and
	// wants to know which type is slow, with no telemetry dependency.
	Elapsed time.Duration
}

// Update regenerates the README index table, and optionally each
// document's table of contents, for one or more types.
//
// A nil or empty types slice means every enabled type in Cfg.EnabledTypes
// order: built-ins first in registry order, then custom types sorted. An
// explicit token that names a disabled type is a TypeDisabledError rather
// than a silent skip, which is the one place this diverges from
// cmd/update.go — the user named the type, so saying nothing about it
// would be a lie. Every token is resolved before any work starts, so a
// bad argument cannot leave half the repository updated.
//
// The context is checked before each type. A cancelled run returns the
// report completed so far together with ctx.Err(), and what was already
// written stays written. A type that fails outright is reported through
// the error rather than as a report entry, so an entry never claims work
// that did not happen.
func (r *Repo) Update(ctx context.Context, types []string, opts UpdateOptions) (UpdateReport, error) {
	names, err := r.typesOrEnabled(types)
	if err != nil {
		return UpdateReport{}, err
	}

	report := UpdateReport{Types: make([]TypeReport, 0, len(names))}

	for _, typeName := range names {
		if err := ctx.Err(); err != nil {
			return report, err
		}

		typeReport, err := r.updateType(ctx, typeName, opts.DryRun)
		if err != nil {
			return report, fmt.Errorf("updating %s: %w", typeName, err)
		}

		report.Types = append(report.Types, typeReport)
	}

	return report, nil
}

// updateType is the whole of an update for a single type: scan, then the
// optional table-of-contents pass, then the README index.
//
// Both Update and Create call this. Create's post-creation refresh is the
// same operation as a one-type update, and the two drifting apart is how
// `docz create` and `docz update` would come to produce different READMEs
// for the same directory.
func (r *Repo) updateType(ctx context.Context, typeName string, dryRun bool) (TypeReport, error) {
	start := time.Now()

	docs, err := r.Scan(ctx, typeName)
	if err != nil {
		return TypeReport{}, err
	}

	dir := r.TypeDir(typeName)

	report := TypeReport{
		Type: typeName,
		Dir:  r.RelPath(dir),
		Docs: len(docs),
	}

	if r.Cfg.TOC.Enabled {
		tocReport, tocErr := r.updateToC(ctx, dir, docs, dryRun)
		if tocErr != nil {
			return report, tocErr
		}

		report.ToC = tocReport
	}

	// The label feeds both the table's heading and the generic index
	// header's render data, so it is resolved once. The heading is "All "
	// plus the label, which is the wording every existing README carries.
	label := r.indexLabelFor(typeName)
	table := index.GenerateTable(docs, "All "+label)

	header, err := doctemplate.ResolveIndexHeader(typeName, r.Path(r.Cfg.DocsDir), doctemplate.IndexHeaderData{
		TypeName:    typeName,
		PluralLabel: label,
	})
	if err != nil {
		return report, fmt.Errorf("resolving index header for %s: %w", typeName, err)
	}

	outcome, err := r.writeIndex(ctx, typeName, header, table, dryRun)
	if err != nil {
		return report, err
	}

	report.Index = outcome
	report.Elapsed = time.Since(start)

	return report, nil
}

// updateToC runs the table-of-contents splice over every document the scan
// returned.
//
// The bytes come from the scan rather than a second read: DocEntry.Content
// is cached for exactly this pass. Per-file write failures are not fatal
// and live in the returned report, because one unwritable document is no
// reason to leave the rest of a type stale.
func (r *Repo) updateToC(
	ctx context.Context,
	dir string,
	docs []document.DocEntry,
	dryRun bool,
) (*toc.UpdateReport, error) {
	files := make([]toc.FileInput, len(docs))

	for i := range docs {
		files[i] = toc.FileInput{
			Path:    filepath.Join(dir, docs[i].Filename),
			Content: docs[i].Content,
		}
	}

	report, err := toc.UpdateFiles(files, r.Cfg.TOC.MinHeadings, dryRun)
	if err != nil {
		return nil, fmt.Errorf("updating table of contents under %s: %w", r.RelPath(dir), err)
	}

	for i := range report.Updated {
		fireFileWritten(ctx, report.Updated[i].Path, FileToC)
	}

	return &report, nil
}

// writeIndex splices the table into the type's README, or computes what
// the splice would do when dryRun is set.
//
// A README with no marker pair is left alone and reported as
// ActionNoMarkers: it is somebody's own file, not docz's to rewrite. That
// is a FileSkipped event on both the real and the dry-run path, since the
// reason is a fact about the file rather than about the write.
//
// A failure is a WriteError naming the path. A dry run can only fail on
// the read, which is the same class of problem to a caller deciding
// whether the repository is usable at all.
func (r *Repo) writeIndex(
	ctx context.Context,
	typeName, header, table string,
	dryRun bool,
) (index.UpdateOutcome, error) {
	path := r.ReadmePath(typeName)

	splice := index.UpdateReadme
	if dryRun {
		splice = index.DryRunReadme
	}

	outcome, err := splice(path, header, table)
	if err != nil {
		return index.UpdateOutcome{}, &WriteError{Path: path, Err: err}
	}

	switch outcome.Action {
	case index.ActionNoMarkers:
		fireFileSkipped(ctx, outcome.Path, SkipNoMarkers)
	case index.ActionCreated, index.ActionUpdated:
		fireFileWritten(ctx, outcome.Path, FileIndex)
	case index.ActionDryRunCreated, index.ActionDryRunUpdated:
		// Nothing was written, so nothing is reported as written. The
		// would-be body is in the outcome for a caller to show.
	}

	return outcome, nil
}

// indexLabelFor returns the display label for a type's index header and
// table heading: the configured plural_label, or the type name with its
// first rune upper-cased when the type declares none (DESIGN-0006
// Decision 3).
//
// Upper-casing rather than leaving a custom type's name bare, because the
// heading reads as prose — "All Frameworks" — and a lower-case word there
// looks like a bug rather than a choice.
func (r *Repo) indexLabelFor(typeName string) string {
	if label := r.Cfg.Types[typeName].PluralLabel; label != "" {
		return label
	}

	if typeName == "" {
		return typeName
	}

	runes := []rune(typeName)
	runes[0] = unicode.ToUpper(runes[0])

	return string(runes)
}
