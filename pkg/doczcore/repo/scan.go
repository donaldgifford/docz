package repo

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/document"
)

// Entry is a scanned document with the two facts a DocEntry alone does not
// carry: which type it belongs to, and where it is.
//
// Path is repo-relative, because that is the form every consumer reports —
// a CLI line, an API response, a Temporal activity result — and computing
// it means knowing the root. A caller that needs the absolute path joins
// it back under Repo.Root, which is the direction that cannot be wrong.
type Entry struct {
	document.DocEntry

	// Type is the canonical type name, not the token the caller passed.
	Type string
	// Path is the document's path relative to Repo.Root.
	Path string
}

// Scan reads one type's directory and returns its documents in ID order.
//
// The token is resolved first, so an alias or id_prefix works here exactly
// as it does on the command line. A disabled type is a TypeDisabledError
// rather than an empty slice: the two mean different things, and returning
// the same value for both is what made "did that type have no documents,
// or is it switched off?" unanswerable.
//
// A missing directory is not an error — document.ScanDocuments reports it
// as no documents, which is what a caller wants for a type nobody has
// written to yet. Files without frontmatter are skipped silently by the
// same call.
func (r *Repo) Scan(ctx context.Context, typeName string) ([]document.DocEntry, error) {
	canonical, err := r.resolveType(typeName)
	if err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	dir := r.TypeDir(canonical)

	fireScanStart(ctx, canonical, dir)

	docs, err := document.ScanDocuments(dir)
	if err != nil {
		return nil, fmt.Errorf("scanning %s: %w", r.RelPath(dir), err)
	}

	fireScanDone(ctx, canonical, len(docs))

	return docs, nil
}

// List scans several types and returns every document as an Entry.
//
// A nil or empty types slice means every enabled type, in
// Cfg.EnabledTypes order — built-ins first in registry order, then custom
// types sorted. That ordering is what makes no-argument output stable, and
// including custom keys is what keeps them from being silently dropped.
//
// The context is checked between types, and a cancelled run returns the
// entries gathered so far together with ctx.Err(). A caller that wants all
// or nothing discards the slice; a caller reporting progress does not have
// to.
func (r *Repo) List(ctx context.Context, types []string) ([]Entry, error) {
	names, err := r.typesOrEnabled(types)
	if err != nil {
		return nil, err
	}

	var entries []Entry

	for _, typeName := range names {
		if err := ctx.Err(); err != nil {
			return entries, err
		}

		docs, err := r.Scan(ctx, typeName)
		if err != nil {
			return entries, err
		}

		dir := r.TypeDir(typeName)

		for i := range docs {
			entries = append(entries, Entry{
				DocEntry: docs[i],
				Type:     typeName,
				Path:     r.RelPath(filepath.Join(dir, docs[i].Filename)),
			})
		}
	}

	return entries, nil
}

// Find resolves a document by its frontmatter id alone, deriving the type
// from the id's prefix.
//
// "IMPL-0018" names its own type, so a consumer holding an id out of a
// commit message or an issue body should not have to say which directory
// to look in (DESIGN-0014 Open Question 6). The prefix is everything
// before the first "-"; it goes through the same resolution the CLI uses,
// so a custom type's prefix works too.
//
// An id with no "-" is an UnknownTypeError naming the whole id as the
// token, because there is no prefix to resolve and guessing which type to
// search would give a wrong answer rather than no answer.
func (r *Repo) Find(ctx context.Context, id string) (Entry, error) {
	prefix, _, ok := strings.Cut(id, "-")
	if !ok || prefix == "" {
		return Entry{}, r.unknownType(id)
	}

	typeName, err := r.resolveType(prefix)
	if err != nil {
		return Entry{}, err
	}

	return r.FindIn(ctx, typeName, id)
}

// FindIn resolves a document by frontmatter id within one type.
//
// The comparison is exact and case-sensitive (DESIGN-0005 Decision 3): an
// id is a key, and a lenient match would let two documents answer to the
// same name. No match is a NotFoundError carrying the type and the id, so
// a caller can word the failure itself — the CLI names the directory it
// searched, which only the CLI knows how to shorten.
func (r *Repo) FindIn(ctx context.Context, typeName, id string) (Entry, error) {
	canonical, err := r.resolveType(typeName)
	if err != nil {
		return Entry{}, err
	}

	docs, err := r.Scan(ctx, canonical)
	if err != nil {
		return Entry{}, err
	}

	dir := r.TypeDir(canonical)

	for i := range docs {
		if docs[i].ID != id {
			continue
		}

		return Entry{
			DocEntry: docs[i],
			Type:     canonical,
			Path:     r.RelPath(filepath.Join(dir, docs[i].Filename)),
		}, nil
	}

	return Entry{}, &NotFoundError{Type: canonical, ID: id}
}
