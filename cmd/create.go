package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/repo"
)

var (
	createStatus   string
	createAuthor   string
	createNoUpdate bool
)

// createOpts captures the per-invocation flag values that
// (*Runner).Create needs. During the Phase 3 transition the package
// globals above still hold the bound flag values; the wrapper packs
// them into a createOpts so the method itself never touches a global.
type createOpts struct {
	status   string
	author   string
	noUpdate bool
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
  docz create inv "Can pgvector Handle Concurrent Writes"`,
	Args: cobra.ExactArgs(2),
	RunE: runCreate,
}

func init() {
	createCmd.Flags().StringVar(&createStatus, "status", "", "initial status (default varies by type)")
	createCmd.Flags().StringVar(&createAuthor, "author", "", "document author (default: git user.name)")
	createCmd.Flags().BoolVar(&createNoUpdate, "no-update", false, "skip automatic index update after creation")
	rootCmd.AddCommand(createCmd)
}

func runCreate(cmd *cobra.Command, args []string) error {
	opts := createOpts{
		status:   createStatus,
		author:   createAuthor,
		noUpdate: createNoUpdate,
	}
	return getRunner().Create(cmdContext(cmd), opts, args)
}

// Create runs the `docz create` workflow: validates the document type,
// resolves the author (config default → git → fallback), instantiates
// the template, and (unless opts.noUpdate is set) refreshes the type's
// README index and wiki nav.
func (r *Runner) Create(ctx context.Context, opts createOpts, args []string) error {
	docType, err := r.Cfg.ValidateType(args[0])
	if err != nil {
		return err
	}
	title := args[1]

	tc := r.Cfg.Types[docType]
	if !tc.Enabled {
		return fmt.Errorf("document type %q is disabled in configuration", docType)
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

	if result.Update != nil {
		if perr := r.printTypeReport(result.Update); perr != nil && err == nil {
			err = perr
		}
	}

	if err != nil {
		return err
	}

	return r.autoUpdateNav(ctx, opts.noUpdate)
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
