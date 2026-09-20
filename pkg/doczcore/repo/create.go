package repo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/docwrite"
)

// CreateOptions is the input to Create.
//
// Author arrives resolved. Reading git config is an L4 dependency (§2.8),
// so the fallback chain cmd/create.go runs — flag, then config default,
// then git user.name, then "Unknown" — stays in cmd/ and this package
// writes whatever it is handed, including nothing.
type CreateOptions struct {
	// Type is any token ValidateType accepts: a canonical name, a
	// registry or per-type alias, or an id_prefix.
	Type string
	// Title is the document title, which also becomes the filename slug.
	Title string
	// Author is written to the document's frontmatter verbatim.
	Author string
	// Status is the initial status. Empty takes the type's first
	// configured status, which is what every template's default is.
	Status string
	// Now is the creation timestamp written as the document's date. The
	// zero value means time.Now(), applied by docwrite.Render so the rule
	// has one home.
	Now time.Time
	// Update runs the type's index refresh afterwards and reports it in
	// CreateResult.Update. Whether to ask for that is the caller's
	// decision: cmd/create.go consults --no-update and Cfg.Index.AutoUpdate
	// and this package obeys the answer.
	Update bool
}

// CreateResult is the created document plus the refresh that followed it.
//
// The embedded docwrite.CreateResult is verbatim from the writer, so
// FilePath is absolute under Repo.Root; RelPath shortens it for a consumer
// that reports paths.
type CreateResult struct {
	// Type is the canonical type name the token resolved to.
	Type string

	docwrite.CreateResult

	// Update is the index refresh, or nil when CreateOptions.Update was
	// false. A non-nil pointer means the refresh ran; its own fields say
	// what it did.
	Update *TypeReport
}

// Create writes a new document of the given type with the next available
// number, and optionally refreshes the type's index afterwards.
//
// The type token is resolved first, so an unknown token is an
// UnknownTypeError and a switched-off type is a TypeDisabledError before
// anything touches the filesystem. A document already at the path this
// would write is an ExistsError, checked before the write rather than
// discovered by it, so a collision never leaves a half-created document
// and a caller can branch on the error instead of reading its message.
//
// Create does not touch the wiki. Rebuilding the MkDocs nav after a
// creation is a second operation over a second file, and pkg/wiki is a
// sibling of this package rather than something under it (rule R2): a
// consumer that wants both calls wiki.UpdateNav next, which is what
// cmd/create.go does.
//
// A failure in the refresh returns the result and the error together. The
// document was written by then, and a caller told only about the failure
// would not know a file now exists.
//
// opts is taken by value because DESIGN-0014 §2.8 fixes the signature that
// way; at 96 bytes it is over gocritic's hugeParam threshold, so the
// helper below takes the derived options by pointer.
//
//nolint:gocritic // hugeParam: the by-value signature is fixed by DESIGN-0014 §2.8.
func (r *Repo) Create(ctx context.Context, opts CreateOptions) (CreateResult, error) {
	typeName, err := r.resolveType(opts.Type)
	if err != nil {
		return CreateResult{}, err
	}

	if err := ctx.Err(); err != nil {
		return CreateResult{}, err
	}

	docOpts := r.createOptions(typeName, &opts)

	path, err := createPlannedPath(&docOpts)
	if err != nil {
		return CreateResult{}, fmt.Errorf("preparing %s document: %w", typeName, err)
	}

	if _, err := os.Stat(path); err == nil {
		return CreateResult{}, &ExistsError{Path: path}
	}

	written, err := docwrite.Create(&docOpts)
	if err != nil {
		return CreateResult{}, fmt.Errorf("creating %s document: %w", typeName, err)
	}

	fireFileWritten(ctx, written.FilePath, FileDocument)

	result := CreateResult{Type: typeName, CreateResult: written}

	if !opts.Update {
		return result, nil
	}

	// The same per-type routine Update runs, never a copy of it: a
	// creation that refreshed the README differently from `docz update`
	// would make the index depend on which command last touched it.
	report, err := r.updateType(ctx, typeName, false)
	if err != nil {
		return result, fmt.Errorf("updating %s: %w", typeName, err)
	}

	result.Update = &report

	return result, nil
}

// createOptions builds the writer's options for a resolved type.
//
// Every path is resolved under Root here so the writer never consults the
// process working directory: DocsDir becomes absolute, and so does the
// type's explicit template path when it has one. An empty template path
// stays empty, which is how docwrite asks doctemplate for the override
// tier and then the embedded one.
func (r *Repo) createOptions(typeName string, opts *CreateOptions) docwrite.CreateOptions {
	typeCfg := r.Cfg.Types[typeName]

	status := opts.Status
	if status == "" && len(typeCfg.Statuses) > 0 {
		status = typeCfg.Statuses[0]
	}

	return docwrite.CreateOptions{
		Type:         config.DocType(typeName),
		Title:        opts.Title,
		Author:       opts.Author,
		Status:       status,
		Prefix:       typeCfg.IDPrefix,
		IDWidth:      typeCfg.IDWidth,
		DocsDir:      r.Path(r.Cfg.DocsDir),
		TypeDir:      typeCfg.Dir,
		TemplatePath: r.Path(typeCfg.Template),
		CreatedAt:    opts.Now,
	}
}

// createPlannedPath returns the path docwrite.Create will write for opts.
//
// Asking the writer's own two halves — NextNumber for the number, Render
// for the filename — rather than reassembling the naming rule here, so
// there is no second definition of what a document is called to drift out
// of step with the first. Render reads the template and is therefore the
// step that fails when a custom type has none, which is worth failing at:
// better before the directory is created than after.
func createPlannedPath(opts *docwrite.CreateOptions) (string, error) {
	dir := filepath.Join(opts.DocsDir, opts.TypeDir)

	number, err := docwrite.NextNumber(dir, opts.IDWidth)
	if err != nil {
		return "", fmt.Errorf("numbering the next document in %s: %w", dir, err)
	}

	rendered, err := docwrite.Render(opts, number)
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, rendered.Filename), nil
}
