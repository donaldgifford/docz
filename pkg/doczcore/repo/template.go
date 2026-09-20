package repo

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/doctemplate"
)

// schemaDirName is the directory under <DocsDir>/templates that holds marker
// skeletons. It mirrors doctemplate's own unexported spelling; a scaffolded
// schema has to land where ResolveSchema will look for it.
const schemaDirName = "schema"

// ExportOptions is what a caller can vary about writing a template out.
type ExportOptions struct {
	// Overwrite replaces a file that is already there. Without it an existing
	// destination is an ExistsError, because a template is something a repo
	// edits and clobbering an edited one silently is the failure mode worth
	// refusing.
	Overwrite bool
}

// ExportResult is the outcome of an export.
//
// Scaffolded and SchemaPath are set together and only on the scaffolding
// path: a caller reading Scaffolded knows the type had no template, and
// SchemaPath is the second file it now has (DESIGN-0015 §3).
type ExportResult struct {
	// Path is the written template, relative to Repo.Root.
	Path string
	// Overwritten reports that a file was already there and Overwrite
	// replaced it.
	Overwritten bool
	// Scaffolded reports that no template resolved for the type and the
	// generic pair was written instead.
	Scaffolded bool
	// SchemaPath is the written marker skeleton, relative to Repo.Root, and
	// empty unless Scaffolded is set.
	SchemaPath string
}

// Template returns the resolved body template for a type.
//
// Resolution is doctemplate's: the path in the type's config, then
// <DocsDir>/templates/<type>.md, then the embedded default. Both
// config-relative paths are joined under Root first, so a consumer operating
// on a checkout somewhere other than its own working directory resolves that
// checkout's overrides rather than its own.
//
// A type with none of the three is doctemplate.ErrNoTemplate, wrapped and
// still reachable with errors.Is. That is the clear error issue #92 asked for,
// and ExportTemplate branches on it to scaffold.
func (r *Repo) Template(ctx context.Context, typeName string) (string, error) {
	canonical, err := r.resolveType(typeName)
	if err != nil {
		return "", err
	}

	if err := ctx.Err(); err != nil {
		return "", err
	}

	body, err := doctemplate.Resolve(
		canonical,
		r.Path(r.Cfg.Types[canonical].Template),
		r.Path(r.Cfg.DocsDir),
	)
	if err != nil {
		return "", fmt.Errorf("resolving %s template: %w", canonical, err)
	}

	return body, nil
}

// ExportTemplate writes a type's resolved template to dest.
//
// An empty dest means the override location, <DocsDir>/templates/<type>.md,
// which is `docz template override`: the file the resolver picks up next time.
// A non-empty dest is taken as given, joined under Root when it is relative,
// so a caller can hand the template to somebody outside the docs tree.
//
// A type whose template does not resolve is scaffolded instead when dest is
// empty (DESIGN-0015 §3): the embedded generic template becomes
// templates/<type>.md and the generic marker skeleton becomes
// templates/schema/<type>.md, so a custom type ends up with the same two
// artifacts a built-in has and the same check over them. With an explicit dest
// there is nothing to scaffold — the pair is only meaningful at the override
// location, and a schema written next to an arbitrary destination would
// resolve for nobody — so the ErrNoTemplate error is returned as it is.
func (r *Repo) ExportTemplate(
	ctx context.Context,
	typeName, dest string,
	opts ExportOptions,
) (ExportResult, error) {
	canonical, err := r.resolveType(typeName)
	if err != nil {
		return ExportResult{}, err
	}

	body, err := r.Template(ctx, canonical)
	if err != nil {
		if dest == "" && errors.Is(err, doctemplate.ErrNoTemplate) {
			return r.scaffoldTemplate(ctx, canonical, opts.Overwrite)
		}

		return ExportResult{}, err
	}

	path := r.templateOverridePath(canonical)
	if dest != "" {
		path = r.Path(dest)
	}

	rel := r.RelPath(path)

	overwritten, err := r.claimDest(ctx, path, opts.Overwrite)
	if err != nil {
		return ExportResult{}, err
	}

	if err := r.writeAndFire(ctx, path, []byte(body), FileDocument); err != nil {
		return ExportResult{}, err
	}

	return ExportResult{Path: rel, Overwritten: overwritten}, nil
}

// scaffoldTemplate writes the generic template and the generic marker
// skeleton under the type's own names.
//
// Both destinations are claimed before either is written, so an export that
// cannot finish does not leave a template with no schema beside it — the one
// state that would make the pair's agreement check fail on a file the caller
// never asked to write.
func (r *Repo) scaffoldTemplate(ctx context.Context, typeName string, overwrite bool) (ExportResult, error) {
	body, err := doctemplate.GenericTemplate()
	if err != nil {
		return ExportResult{}, fmt.Errorf("reading the generic template: %w", err)
	}

	skeleton, err := doctemplate.EmbeddedSchema(doctemplate.DefaultTemplateName)
	if err != nil {
		return ExportResult{}, fmt.Errorf("reading the generic schema: %w", err)
	}

	tmplPath := r.templateOverridePath(typeName)
	schemaPath := r.schemaOverridePath(typeName)

	tmplOverwritten, err := r.claimDest(ctx, tmplPath, overwrite)
	if err != nil {
		return ExportResult{}, err
	}

	schemaOverwritten, err := r.claimDest(ctx, schemaPath, overwrite)
	if err != nil {
		return ExportResult{}, err
	}

	if err := r.writeAndFire(ctx, tmplPath, []byte(body), FileDocument); err != nil {
		return ExportResult{}, err
	}

	if err := r.writeAndFire(ctx, schemaPath, skeleton, FileDocument); err != nil {
		return ExportResult{}, err
	}

	return ExportResult{
		Path:        r.RelPath(tmplPath),
		Overwritten: tmplOverwritten || schemaOverwritten,
		Scaffolded:  true,
		SchemaPath:  r.RelPath(schemaPath),
	}, nil
}

// claimDest reports whether path is about to be overwritten, refusing when it
// exists and overwrite is not set.
//
// The decision is initAction's, so "this file is already there" means the same
// thing to an export as it does to an init. Only the consequence differs: init
// records a skip and carries on, while an export the caller asked for has
// nothing left to do and says so.
func (r *Repo) claimDest(ctx context.Context, path string, overwrite bool) (bool, error) {
	action, write := initAction(path, overwrite)
	if !write {
		rel := r.RelPath(path)

		fireFileSkipped(ctx, rel, SkipExists)

		return false, &ExistsError{Path: rel}
	}

	return action == InitOverwritten, nil
}

// templateOverridePath is the body template a type resolves next time:
// <DocsDir>/templates/<type>.md, absolute.
func (r *Repo) templateOverridePath(typeName string) string {
	return filepath.Join(r.Path(r.Cfg.DocsDir), config.TemplatesDir, typeName+".md")
}

// schemaOverridePath is the marker skeleton a type validates against:
// <DocsDir>/templates/schema/<type>.md, absolute.
func (r *Repo) schemaOverridePath(typeName string) string {
	return filepath.Join(r.Path(r.Cfg.DocsDir), config.TemplatesDir, schemaDirName, typeName+".md")
}
