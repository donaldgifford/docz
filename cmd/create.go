package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	clischema "github.com/donaldgifford/docz/v2/api/cli"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/index"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/repo"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/toc"
	"github.com/donaldgifford/docz/v2/pkg/wiki"
)

var (
	createStatus   string
	createAuthor   string
	createNoUpdate bool
	createFormat   string
)

// createOpts captures the per-invocation flag values that
// (*Runner).Create needs. During the Phase 3 transition the package
// globals above still hold the bound flag values; the wrapper packs
// them into a createOpts so the method itself never touches a global.
type createOpts struct {
	status   string
	author   string
	noUpdate bool
	format   string
}

// createOutput is the whole of stdout under `docz create --format json`, the
// contract api/cli/create.schema.json describes. The prose the text form
// prints for the index refresh and the nav rebuild is replaced by the update
// and nav structures; the generic-template note stays on stderr.
type createOutput struct {
	SchemaVersion   int           `json:"schema_version"`
	ID              string        `json:"id"`
	Type            string        `json:"type"`
	Path            string        `json:"path"`
	Title           string        `json:"title"`
	GenericTemplate bool          `json:"generic_template"`
	Update          *createUpdate `json:"update"`
	Nav             *createNav    `json:"nav"`
}

// createUpdate is the type's index and ToC refresh.
type createUpdate struct {
	Type  string      `json:"type"`
	Dir   string      `json:"dir"`
	Docs  int         `json:"docs"`
	Index createIndex `json:"index"`
	ToC   *createToC  `json:"toc"`
}

// createIndex is the README index outcome.
type createIndex struct {
	Action string `json:"action"`
	Path   string `json:"path"`
}

// createToC is the ToC pass over the type's documents.
type createToC struct {
	Updated     []createToCFile  `json:"updated"`
	Unchanged   []createToCFile  `json:"unchanged"`
	WouldUpdate []createToCFile  `json:"would_update"`
	Skipped     []string         `json:"skipped"`
	WriteErrors []createToCError `json:"write_errors"`
}

// createToCFile is one document the ToC pass looked at.
type createToCFile struct {
	Path     string `json:"path"`
	Headings int    `json:"headings"`
}

// createToCError is one document the ToC pass could not write.
type createToCError struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

// createNav is the MkDocs nav rebuild.
type createNav struct {
	Path    string `json:"path"`
	Pages   int    `json:"pages"`
	Written bool   `json:"written"`
}

var createCmd = &cobra.Command{
	Use:   "create <type> <title>",
	Short: "Create a new document from a template",
	Long: `Create a new document of the specified type with an auto-incremented ID.

` + config.TypesHelp() + `

Custom types defined in .docz.yaml can be created the same way — by their
canonical name, their id_prefix, or any configured alias.

Examples:
  docz create rfc "API Rate Limiting Strategy"
  docz create adr "Use PostgreSQL for Primary Storage"
  docz create design "User Authentication Flow"
  docz create impl "Migrate to gRPC"
  docz create implementation "Migrate to gRPC"
  docz create investigation "Can pgvector Handle Concurrent Writes"
  docz create inv "Can pgvector Handle Concurrent Writes"

With --format json, stdout is one JSON object (api/cli/create.schema.json):
the new document's id, type, path, and title, and the index refresh and nav
rebuild as structures. A failed creation prints an error and no JSON.`,
	Args: cobra.ExactArgs(2),
	RunE: runCreate,
}

func init() {
	createCmd.Flags().StringVar(&createStatus, "status", "", "initial status (default varies by type)")
	createCmd.Flags().StringVar(&createAuthor, "author", "", "document author (default: git user.name)")
	createCmd.Flags().BoolVar(&createNoUpdate, "no-update", false, "skip automatic index update after creation")
	createCmd.Flags().StringVar(&createFormat, "format", formatText, "output format: text or json")
	rootCmd.AddCommand(createCmd)
}

func runCreate(cmd *cobra.Command, args []string) error {
	opts := createOpts{
		status:   createStatus,
		author:   createAuthor,
		noUpdate: createNoUpdate,
		format:   createFormat,
	}
	return getRunner().Create(cmdContext(cmd), opts, args)
}

// Create runs the `docz create` workflow: validates the document type,
// resolves the author (config default → git → fallback), instantiates
// the template, and (unless opts.noUpdate is set) refreshes the type's
// README index and wiki nav.
func (r *Runner) Create(ctx context.Context, opts createOpts, args []string) error {
	format, err := resolveStatusFormat(opts.format)
	if err != nil {
		return err
	}

	docType, err := r.Cfg.ValidateType(args[0])
	if err != nil {
		return err
	}
	title := args[1]

	tc := r.Cfg.Types[docType]
	if !tc.Enabled {
		return fmt.Errorf(
			"document type %q is disabled in configuration; set types.%s.enabled: true in .docz.yaml",
			docType, docType)
	}

	author := r.resolveAuthor(ctx, opts.author)
	r.Logger.Debug("resolved author", "author", author)

	status := opts.status
	if status == "" && len(tc.Statuses) > 0 {
		status = tc.Statuses[0]
	}
	r.Logger.Debug("create plan",
		"status", status,
		"template", tc.Template,
	)

	// The index refresh is asked for here and performed inside Create, so
	// a created document and `docz update` cannot produce different
	// READMEs for the same directory — they run the same routine.
	result, err := r.repoOrOpen().Create(ctx, repo.CreateOptions{
		Type:   docType,
		Title:  title,
		Author: author,
		Status: status,
		Now:    r.Now(),
		Update: !opts.noUpdate && r.Cfg.Index.AutoUpdate,
	})

	// A failure with no path means nothing was written. A failure with one
	// means the document exists and the refresh after it did not, which is
	// worth saying in that order: the user needs to know the file is there
	// before hearing that its index is stale.
	if err != nil && result.FilePath == "" {
		return err
	}

	if format == formatJSON {
		return r.printCreatedJSON(ctx, docType, title, &result, err, opts.noUpdate)
	}

	if err := r.printCreated(docType, &result, err); err != nil {
		return err
	}

	return r.autoUpdateNav(ctx, opts.noUpdate)
}

// printCreated reports a creation: the path, the generic-template note when
// the type had no template, and the index refresh. It returns createErr, or
// the first print failure when the creation itself succeeded.
func (r *Runner) printCreated(docType string, result *repo.CreateResult, createErr error) error {
	err := createErr

	// Printing runs to the end even if it fails partway, and its error is
	// kept only when the creation itself succeeded. A write to r.Out that
	// fails is worth reporting on its own and never worth reporting instead
	// of the index failure below, which is the one that tells the user their
	// new document is not in the README yet.
	//
	// repo.Create has already wrapped err as "updating <type>: …", so it is
	// returned as it stands rather than wrapped a second time in words that
	// say the same thing.
	if _, perr := fmt.Fprintf(r.Out, "Created %s: %s\n",
		strings.ToUpper(docType), result.FilePath); perr != nil && err == nil {
		err = perr
	}

	if result.GenericTemplate {
		if perr := r.genericTemplateNote(docType); perr != nil && err == nil {
			err = perr
		}
	}

	if result.Update != nil {
		if perr := r.printTypeReport(result.Update); perr != nil && err == nil {
			err = perr
		}
	}

	return err
}

// printCreatedJSON is the --format json form of a creation: the envelope on
// stdout and the generic-template note on stderr. A refresh that failed after
// the document was written is returned as the error with no JSON, because a
// consumer reading the envelope would take a half-done creation for a whole
// one.
func (r *Runner) printCreatedJSON(
	ctx context.Context, docType, title string, result *repo.CreateResult, createErr error, noUpdate bool,
) error {
	if createErr != nil {
		return createErr
	}

	if result.GenericTemplate {
		if err := r.genericTemplateNote(docType); err != nil {
			return err
		}
	}

	rp := r.repoOrOpen()

	out := createOutput{
		SchemaVersion:   clischema.SchemaVersionCreate,
		ID:              r.Cfg.Types[docType].IDPrefix + "-" + result.Number,
		Type:            docType,
		Path:            filepath.ToSlash(rp.RelPath(result.FilePath)),
		Title:           title,
		GenericTemplate: result.GenericTemplate,
		Update:          toCreateUpdate(rp, result.Update),
	}

	nav, err := r.createNavJSON(ctx, rp, noUpdate)
	if err != nil {
		return err
	}

	out.Nav = nav

	enc := json.NewEncoder(r.Out)
	enc.SetIndent("", "  ")

	return enc.Encode(out)
}

// createNavJSON rebuilds the nav under the same conditions as autoUpdateNav
// and returns its report instead of printing it; nil when nothing ran.
func (r *Runner) createNavJSON(ctx context.Context, rp *repo.Repo, noUpdate bool) (*createNav, error) {
	if noUpdate || !r.Cfg.Wiki.AutoUpdate {
		return nil, nil //nolint:nilnil // no nav rebuild is the null nav field
	}

	// Read the positive way round, as autoUpdateNav does: there is an
	// mkdocs.yml, therefore rebuild it. Absent is the common case.
	if _, statErr := os.Stat(rp.Path(r.Cfg.Wiki.MkDocsPath)); statErr == nil {
		report, err := wiki.UpdateNav(ctx, r.RepoRoot, &r.Cfg, wiki.NavOptions{})
		if err != nil {
			return nil, fmt.Errorf("auto-updating wiki nav: %w", wikiNavError(report.Path, err))
		}

		return &createNav{
			Path:    filepath.ToSlash(rp.RelPath(report.Path)),
			Pages:   report.Pages,
			Written: report.Written,
		}, nil
	}

	return nil, nil //nolint:nilnil // no mkdocs.yml is the null nav field
}

// toCreateUpdate converts the type report to its JSON form; nil when no
// refresh ran.
func toCreateUpdate(rp *repo.Repo, tr *repo.TypeReport) *createUpdate {
	if tr == nil {
		return nil
	}

	out := &createUpdate{
		Type: tr.Type,
		Dir:  filepath.ToSlash(rp.RelPath(tr.Dir)),
		Docs: tr.Docs,
		Index: createIndex{
			Action: indexActionName(tr.Index.Action),
			Path:   filepath.ToSlash(rp.RelPath(tr.Index.Path)),
		},
	}

	if tr.ToC != nil {
		out.ToC = toCreateToC(rp, tr.ToC)
	}

	return out
}

func toCreateToC(rp *repo.Repo, report *toc.UpdateReport) *createToC {
	files := func(in []toc.FileResult) []createToCFile {
		out := make([]createToCFile, 0, len(in))
		for _, f := range in {
			out = append(out, createToCFile{Path: filepath.ToSlash(rp.RelPath(f.Path)), Headings: f.Headings})
		}

		return out
	}

	out := &createToC{
		Updated:     files(report.Updated),
		Unchanged:   files(report.Unchanged),
		WouldUpdate: files(report.WouldUpdate),
		Skipped:     make([]string, 0, len(report.Skipped)),
		WriteErrors: make([]createToCError, 0, len(report.WriteErrors)),
	}

	for _, p := range report.Skipped {
		out.Skipped = append(out.Skipped, filepath.ToSlash(rp.RelPath(p)))
	}

	for _, e := range report.WriteErrors {
		out.WriteErrors = append(out.WriteErrors, createToCError{
			Path: filepath.ToSlash(rp.RelPath(e.Path)), Error: e.Err.Error(),
		})
	}

	return out
}

// indexActionName is the schema's spelling of an index outcome.
func indexActionName(a index.UpdateAction) string {
	switch a {
	case index.ActionCreated:
		return "created"
	case index.ActionUpdated:
		return "updated"
	case index.ActionNoMarkers:
		return "no_markers"
	case index.ActionDryRunCreated:
		return "dry_run_created"
	case index.ActionDryRunUpdated:
		return "dry_run_updated"
	default:
		return "none"
	}
}

// autoUpdateNav rebuilds the MkDocs nav after a creation, when the config
// asks for it and there is an mkdocs.yml to rebuild.
//
// The stat is the condition: a repository that has never run `docz wiki init`
// has no nav, and creating one as a side effect of `docz create` would be
// docz deciding on the user's behalf that they wanted a wiki.
func (r *Runner) autoUpdateNav(ctx context.Context, noUpdate bool) error {
	if noUpdate || !r.Cfg.Wiki.AutoUpdate {
		return nil
	}

	// Absent is the common case and not a failure, so the stat is read the
	// positive way round: there is an mkdocs.yml, therefore rebuild it.
	if _, err := os.Stat(r.Cfg.Wiki.MkDocsPath); err == nil {
		if err := runWikiUpdateNav(ctx, r.Cfg.Wiki.MkDocsPath); err != nil {
			return fmt.Errorf("auto-updating wiki nav: %w", err)
		}
	}

	return nil
}

// resolveAuthor determines the document author from (in order):
// flag, config default, git config user.name, fallback.
func (r *Runner) resolveAuthor(ctx context.Context, flagAuthor string) string {
	if flagAuthor != "" {
		return flagAuthor
	}

	if r.Cfg.Author.Default != "" {
		return r.Cfg.Author.Default
	}

	if r.Cfg.Author.FromGit {
		if name := r.Git.UserName(ctx); name != "" {
			return name
		}
	}

	return "Unknown"
}
