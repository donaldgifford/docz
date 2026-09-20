package cmd

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/wiki"
)

var (
	wikiForce           bool
	wikiSiteName        string
	wikiSiteDescription string
	wikiDryRun          bool
)

// wikiInitOpts captures the per-invocation flag values for
// `docz wiki init`, populated by the RunE wrapper from the package
// globals above.
type wikiInitOpts struct {
	force           bool
	siteName        string
	siteDescription string
}

var wikiCmd = &cobra.Command{
	Use:   "wiki",
	Short: "Manage MkDocs/TechDocs integration",
	Long: `Generate and maintain a mkdocs.yml file compatible with Backstage TechDocs.

Subcommands:
  init      Create mkdocs.yml with TechDocs defaults
  update    Rebuild the nav section from docs/ contents`,
}

var wikiInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create mkdocs.yml with TechDocs defaults",
	Long: `Create a mkdocs.yml at the repo root with sensible TechDocs defaults.
If the project hasn't been initialized with docz init, it runs docz init
automatically.

If mkdocs.yml already exists and --force is not passed, the command fails
with a clear error message.`,
	RunE: runWikiInit,
}

var wikiUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Rebuild the nav section from docs/ contents",
	Long: `Scan the docs/ directory and regenerate the nav section of mkdocs.yml.
All other fields (site_name, plugins, theme, etc.) are preserved.`,
	RunE: runWikiUpdate,
}

func init() {
	wikiInitCmd.Flags().BoolVar(
		&wikiForce, "force", false,
		"overwrite existing mkdocs.yml",
	)
	wikiInitCmd.Flags().StringVar(
		&wikiSiteName, "site-name", "",
		"set site_name (default: repo directory name)",
	)
	wikiInitCmd.Flags().StringVar(
		&wikiSiteDescription, "site-description", "",
		"set site_description",
	)
	wikiUpdateCmd.Flags().BoolVar(
		&wikiDryRun, "dry-run", false,
		"print the generated nav without modifying mkdocs.yml",
	)

	wikiCmd.AddCommand(wikiInitCmd)
	wikiCmd.AddCommand(wikiUpdateCmd)
	rootCmd.AddCommand(wikiCmd)
}

func runWikiInit(cmd *cobra.Command, _ []string) error {
	return getRunner().WikiInit(cmdContext(cmd), wikiInitOpts{
		force:           wikiForce,
		siteName:        wikiSiteName,
		siteDescription: wikiSiteDescription,
	})
}

func runWikiUpdate(cmd *cobra.Command, _ []string) error {
	return getRunner().WikiUpdate(cmdContext(cmd), wikiDryRun)
}

// runWikiUpdateNav is the package-level shim cmd/create.go's
// auto-wiki-update path calls, so the nav update has one implementation in
// this package instead of a second copy there.
//
// It takes the creating command's context, so a `docz create` interrupted
// while rebuilding the nav stops there rather than finishing a write nobody
// is waiting for.
func runWikiUpdateNav(ctx context.Context, mkdocsPath string) error {
	return getRunner().wikiUpdateNav(ctx, mkdocsPath)
}

// WikiInit auto-runs docz init if needed, writes mkdocs.yml with TechDocs
// defaults and the docs landing page, and populates the nav from the
// existing docs tree.
//
// The writing is wiki.Init's. What stays here is what belongs to the CLI
// rather than the library:
//
//   - Refusing over an mkdocs.yml it was not told to replace. pkg/wiki
//     reports an existing file as wiki.Skipped and carries on to the
//     landing page, so the refusal happens before Init runs and not merely
//     before its report is printed — otherwise a failing `docz wiki init`
//     would start leaving a docs/index.md behind it.
//   - Spending --force on mkdocs.yml alone. InitOptions.Force covers both
//     files Init writes, and docz has never replaced an existing
//     docs/index.md: that is the half of the pair a person hand-edits.
//   - Resolving the site name from the git remote (an L4 dependency, the
//     same way the author name is) and every printed line.
//
// wiki.Init deliberately leaves the nav placeholder alone, so the nav pass
// runs after it and its page count is reported rather than discarded.
func (r *Runner) WikiInit(ctx context.Context, opts wikiInitOpts) error {
	if err := r.ensureDoczInit(ctx); err != nil {
		return fmt.Errorf("ensuring docz init: %w", err)
	}

	mkdocsPath := r.wikiMkDocsPath()
	if err := wikiPrepareMkDocs(mkdocsPath, opts.force); err != nil {
		return err
	}

	siteName := opts.siteName
	if siteName == "" {
		siteName = r.repoName()
	}

	report, err := wiki.Init(ctx, r.RepoRoot, &r.Cfg, wiki.InitOptions{
		SiteName:        siteName,
		SiteDescription: opts.siteDescription,
	})
	if err != nil {
		// Which file was at stake: an Action left zero is one Init never
		// reached, so a failure before mkdocs.yml keeps the "writing
		// <path>" wrapping this command has always used and one after it
		// keeps "ensuring docs index".
		if report.MkDocs == 0 {
			return fmt.Errorf("writing %s: %w", report.MkDocsPath, err)
		}

		return fmt.Errorf("ensuring docs index: %w", err)
	}

	// Unreachable via wikiPrepareMkDocs, and kept for the file that appears
	// between the two calls: Init reports that Skipped rather than failing,
	// and this command refuses either way.
	if report.MkDocs == wiki.Skipped {
		return wikiExistsError(report.MkDocsPath)
	}

	if _, err := fmt.Fprintf(r.Out, "Created %s\n", report.MkDocsPath); err != nil {
		return err
	}

	if report.Index == wiki.Created {
		if _, err := fmt.Fprintf(r.Out, "Created %s\n", report.IndexPath); err != nil {
			return err
		}
	} else {
		r.Logger.Debug("docs index exists, skipping", "path", report.IndexPath)
	}

	return r.wikiUpdateNav(ctx, report.MkDocsPath)
}

// WikiUpdate refreshes the nav in an existing mkdocs.yml, or prints the nav
// it would write when dryRun is set.
func (r *Runner) WikiUpdate(ctx context.Context, dryRun bool) error {
	if !dryRun {
		return r.wikiUpdateNav(ctx, r.wikiMkDocsPath())
	}

	report, err := wiki.UpdateNav(ctx, r.RepoRoot, &r.Cfg, wiki.NavOptions{DryRun: true})
	if err != nil {
		return wikiNavError(report.Path, err)
	}

	return r.printNav(report.Entries, "")
}

// wikiUpdateNav rebuilds the nav in mkdocs.yml through wiki.UpdateNav and
// prints the single line this command has always printed.
//
// mkdocsPath is where the caller believes the file is, which is where
// UpdateNav resolves it from too; it is passed for the debug narration,
// while what gets printed is the path the report came back with.
func (r *Runner) wikiUpdateNav(ctx context.Context, mkdocsPath string) error {
	r.wikiLogScan(ctx, mkdocsPath)

	report, err := wiki.UpdateNav(ctx, r.RepoRoot, &r.Cfg, wiki.NavOptions{})
	if err != nil {
		return wikiNavError(report.Path, err)
	}

	r.logScanResult(report.Entries)

	_, err = fmt.Fprintf(r.Out, "Updated nav in %s (%d pages)\n",
		report.Path, report.Pages)

	return err
}

// wikiPrepareMkDocs applies this command's policy to an mkdocs.yml already
// on disk: refuse it when --force was not passed, remove it when it was.
//
// Removing the file rather than handing wiki.Init the force is what keeps
// --force off the landing page. Init's single Force covers both files it
// writes, and an existing docs/index.md has always been left alone, so the
// force is spent here: the file the user asked to replace goes, Init's own
// absent-then-create path writes it back, and Init runs with Force off so
// an existing landing page is skipped exactly as before.
func wikiPrepareMkDocs(mkdocsPath string, force bool) error {
	if !force {
		if _, err := os.Stat(mkdocsPath); err == nil {
			return wikiExistsError(mkdocsPath)
		}

		return nil
	}

	if err := os.Remove(mkdocsPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("removing %s: %w", mkdocsPath, err)
	}

	return nil
}

// wikiExistsError is the refusal `docz wiki init` prints for an mkdocs.yml
// it was not told to replace.
func wikiExistsError(path string) error {
	return fmt.Errorf("%s already exists (use --force to overwrite)", path)
}

// wikiNavError adds the one thing pkg/wiki leaves to the CLI: the hint that
// a missing mkdocs.yml is what `docz wiki init` is for. Every other failure
// UpdateNav returns already names the file it was reading or writing, so it
// passes through unwrapped.
func wikiNavError(path string, err error) error {
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%s not found (run `docz wiki init` first)", path)
	}

	return err
}

// wikiMkDocsPath is the mkdocs.yml this run operates on, resolved the way
// pkg/wiki resolves it: an absolute path (what PersistentPreRunE produces)
// is used as it stands, a relative one lands under the repo root.
func (r *Runner) wikiMkDocsPath() string {
	return r.inRepo(r.Cfg.Wiki.MkDocsPath)
}

// wikiLogScan narrates the scan wiki.UpdateNav is about to perform.
//
// The existing nav order no longer passes through cmd — UpdateNav reads it
// out of mkdocs.yml itself — so it is read back here only when someone is
// listening: the whole call is gated on debug logging being enabled, which
// keeps the --verbose narration as it was and costs a normal run nothing.
func (r *Runner) wikiLogScan(ctx context.Context, mkdocsPath string) {
	if !r.Logger.Enabled(ctx, slog.LevelDebug) {
		return
	}

	data, err := wiki.ReadMkDocs(mkdocsPath)
	if err != nil {
		// Not this function's failure to report: the read that matters is
		// UpdateNav's, and it reports it.
		return
	}

	r.logScan(wiki.ExistingNavOrder(data))
}

func (r *Runner) logScan(existingOrder []string) {
	r.Logger.Debug("scanning docs",
		"dir", r.Cfg.DocsDir,
		"exclude", r.Cfg.Wiki.Exclude,
	)
	if len(existingOrder) > 0 {
		r.Logger.Debug("preserving existing nav order", "order", existingOrder)
	} else {
		r.Logger.Debug("no existing nav order, sorting alphabetically")
	}
}

func (r *Runner) logScanResult(entries []wiki.NavEntry) {
	r.Logger.Debug("scan complete", "top_level_entries", len(entries))
	r.debugNav(entries, "")
}

// ensureDoczInit checks if docz has been initialized and runs init if not.
//
// The scaffolding goes through initRepo rather than Init so the context
// this command was given reaches it: an auto-init is the longest thing
// `docz wiki init` does, and it is the one a Ctrl-C should be able to stop.
func (r *Runner) ensureDoczInit(ctx context.Context) error {
	configExists := false
	if _, err := os.Stat(r.inRepo(config.ConfigFileName)); err == nil {
		configExists = true
	}

	docsExists := false
	if info, err := os.Stat(r.Cfg.DocsDir); err == nil && info.IsDir() {
		docsExists = true
	}

	if configExists && docsExists {
		r.Logger.Debug("docz already initialized, skipping init")
		return nil
	}

	r.Logger.Debug("running docz init")
	return r.initRepo(ctx, forceInit)
}

func (r *Runner) repoName() string {
	if r.RepoRoot != "" {
		return filepath.Base(r.RepoRoot)
	}
	dir, err := os.Getwd()
	if err != nil {
		return "my-project"
	}
	return filepath.Base(dir)
}

func (r *Runner) printNav(entries []wiki.NavEntry, indent string) error {
	for i := range entries {
		e := &entries[i]
		if len(e.Children) > 0 {
			if _, err := fmt.Fprintf(r.Out, "%s- %s:\n", indent, e.Title); err != nil {
				return err
			}
			if err := r.printNav(e.Children, indent+"    "); err != nil {
				return err
			}
			continue
		}
		if _, err := fmt.Fprintf(r.Out, "%s- %s: %s\n", indent, e.Title, e.Path); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runner) debugNav(entries []wiki.NavEntry, indent string) {
	for i := range entries {
		e := &entries[i]
		if len(e.Children) > 0 {
			r.Logger.Debug("nav dir",
				"indent", indent,
				"title", e.Title,
				"children", len(e.Children),
			)
			r.debugNav(e.Children, indent+"  ")
			continue
		}
		r.Logger.Debug("nav page",
			"indent", indent,
			"title", e.Title,
			"path", e.Path,
		)
	}
}
