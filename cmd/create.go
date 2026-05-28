package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/donaldgifford/docz/internal/config"
	"github.com/donaldgifford/docz/internal/document"
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

Examples:
  docz create rfc "API Rate Limiting Strategy"
  docz create adr "Use PostgreSQL for Primary Storage"
  docz create design "User Authentication Flow"
  docz create impl "Migrate to gRPC"
  docz create implementation "Migrate to gRPC"
  docz create plan "Telemetry Pipeline Approach"
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
	ctx := context.Background()
	if cmd != nil {
		ctx = cmd.Context()
	}
	opts := createOpts{
		status:   createStatus,
		author:   createAuthor,
		noUpdate: createNoUpdate,
	}
	return getRunner().Create(ctx, opts, args)
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

	createOpts := document.CreateOptions{
		Type:         docType,
		Title:        title,
		Author:       author,
		Status:       status,
		Prefix:       tc.IDPrefix,
		IDWidth:      tc.IDWidth,
		DocsDir:      r.Cfg.DocsDir,
		TypeDir:      tc.Dir,
		TemplatePath: tc.Template,
		CreatedAt:    r.Now(),
	}

	result, err := document.Create(&createOpts)
	if err != nil {
		return fmt.Errorf("creating %s document: %w", docType, err)
	}

	if _, err := fmt.Fprintf(r.Out, "Created %s: %s\n",
		strings.ToUpper(docType), result.FilePath); err != nil {
		return err
	}

	if !opts.noUpdate && r.Cfg.Index.AutoUpdate {
		if err := r.updateType(docType, false); err != nil {
			return fmt.Errorf("auto-updating index: %w", err)
		}
	}

	if !opts.noUpdate && r.Cfg.Wiki.AutoUpdate {
		if _, err := os.Stat(r.Cfg.Wiki.MkDocsPath); err == nil {
			if err := runWikiUpdateNav(r.Cfg.Wiki.MkDocsPath); err != nil {
				return fmt.Errorf("auto-updating wiki nav: %w", err)
			}
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
