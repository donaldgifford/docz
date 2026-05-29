package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/donaldgifford/docz/internal/config"
	doctemplate "github.com/donaldgifford/docz/internal/template"
	"github.com/donaldgifford/docz/internal/wiki"
)

var (
	wikiForce           bool
	wikiSiteName        string
	wikiSiteDescription string
	wikiDryRun          bool
)

// wikiInitOpts captures the per-invocation flag values for
// `docz wiki init`. (Phase 3 transition: populated by the wrapper
// from the package globals above.)
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

func runWikiInit(_ *cobra.Command, _ []string) error {
	return getRunner().WikiInit(wikiInitOpts{
		force:           wikiForce,
		siteName:        wikiSiteName,
		siteDescription: wikiSiteDescription,
	})
}

func runWikiUpdate(_ *cobra.Command, _ []string) error {
	return getRunner().WikiUpdate(wikiDryRun)
}

// runWikiUpdateNav is the package-level shim retained during the
// Phase 3 transition so cmd/create.go's auto-wiki-update path keeps
// compiling without depending on a Runner method symbol.
func runWikiUpdateNav(mkdocsPath string) error {
	return getRunner().updateWikiNav(mkdocsPath)
}

// WikiInit auto-runs docz init if needed, writes mkdocs.yml with
// TechDocs defaults, scaffolds docs/index.md, and populates the nav
// from the existing docs tree.
func (r *Runner) WikiInit(opts wikiInitOpts) error {
	if err := r.ensureDoczInit(); err != nil {
		return fmt.Errorf("ensuring docz init: %w", err)
	}

	mkdocsPath := r.Cfg.Wiki.MkDocsPath

	if !opts.force {
		if _, err := os.Stat(mkdocsPath); err == nil {
			return fmt.Errorf(
				"%s already exists (use --force to overwrite)",
				mkdocsPath,
			)
		}
	}

	siteName := opts.siteName
	if siteName == "" {
		siteName = r.repoName()
	}

	siteDesc := opts.siteDescription
	if siteDesc == "" {
		siteDesc = "Documentation for " + siteName
	}

	mkdocsCfg := &wiki.MkDocsConfig{
		SiteName:           siteName,
		SiteDescription:    siteDesc,
		DocsDir:            r.Cfg.Wiki.DocsDir,
		RepoURL:            r.Cfg.Wiki.RepoURL,
		SiteURL:            r.Cfg.Wiki.SiteURL,
		Theme:              r.Cfg.Wiki.Theme,
		Plugins:            r.Cfg.Wiki.Plugins,
		MarkdownExtensions: r.Cfg.Wiki.MarkdownExtensions,
	}
	if err := wiki.CreateMkDocs(mkdocsPath, mkdocsCfg); err != nil {
		return fmt.Errorf("writing %s: %w", mkdocsPath, err)
	}

	if _, err := fmt.Fprintf(r.Out, "Created %s\n", mkdocsPath); err != nil {
		return err
	}

	if err := r.ensureDocsIndex(siteName); err != nil {
		return fmt.Errorf("ensuring docs index: %w", err)
	}

	return r.updateWikiNav(mkdocsPath)
}

// WikiUpdate refreshes the nav in an existing mkdocs.yml (or prints
// what it would generate when dryRun is true).
func (r *Runner) WikiUpdate(dryRun bool) error {
	mkdocsPath := r.Cfg.Wiki.MkDocsPath

	if _, err := os.Stat(mkdocsPath); errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf(
			"%s not found (run `docz wiki init` first)",
			mkdocsPath,
		)
	}

	if dryRun {
		return r.updateWikiNavDryRun(mkdocsPath)
	}

	return r.updateWikiNav(mkdocsPath)
}

// updateWikiNav scans the docs directory and updates the nav in
// mkdocs.yml.
func (r *Runner) updateWikiNav(mkdocsPath string) error {
	data, err := wiki.ReadMkDocs(mkdocsPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", mkdocsPath, err)
	}
	existingOrder := wiki.ExistingNavOrder(data)
	r.logScan(existingOrder)

	entries, err := wiki.BuildNav(
		r.Cfg.DocsDir,
		r.Cfg.Wiki.Exclude,
		r.Cfg.Wiki.NavTitles,
		existingOrder,
	)
	if err != nil {
		return fmt.Errorf("scanning docs: %w", err)
	}
	r.logScanResult(entries)

	data["nav"] = wiki.NavToYAML(entries)
	if err := wiki.WriteMkDocs(mkdocsPath, data); err != nil {
		return fmt.Errorf("writing %s: %w", mkdocsPath, err)
	}

	_, err = fmt.Fprintf(r.Out, "Updated nav in %s (%d pages)\n",
		mkdocsPath, wiki.CountPages(entries))
	return err
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

func (r *Runner) updateWikiNavDryRun(mkdocsPath string) error {
	data, err := wiki.ReadMkDocs(mkdocsPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", mkdocsPath, err)
	}

	entries, err := wiki.BuildNav(
		r.Cfg.DocsDir,
		r.Cfg.Wiki.Exclude,
		r.Cfg.Wiki.NavTitles,
		wiki.ExistingNavOrder(data),
	)
	if err != nil {
		return fmt.Errorf("scanning docs: %w", err)
	}

	return r.printNav(entries, "")
}

// ensureDoczInit checks if docz has been initialized and runs init if not.
func (r *Runner) ensureDoczInit() error {
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
	return r.Init(forceInit)
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

func (r *Runner) ensureDocsIndex(siteName string) error {
	indexPath := filepath.Join(r.Cfg.DocsDir, config.WikiIndexName)

	if _, err := os.Stat(indexPath); err == nil {
		r.Logger.Debug("docs index exists, skipping", "path", indexPath)
		return nil
	}

	tmplContent, err := doctemplate.ResolveWikiIndex(r.Cfg.DocsDir)
	if err != nil {
		return fmt.Errorf("resolving wiki index template: %w", err)
	}

	enabled := r.Cfg.EnabledTypes()
	types := make([]doctemplate.WikiIndexType, 0, len(enabled))
	for _, typeName := range enabled {
		tc := r.Cfg.Types[typeName]
		// WikiConfig.NavTitles wins over PluralLabel (Decisions §4
		// back-compat); PluralLabel is the fallback. Capitalized
		// typeName is only used as a last-resort label if neither is
		// configured.
		navTitle := r.Cfg.Wiki.NavTitles[typeName]
		if navTitle == "" {
			navTitle = tc.PluralLabel
		}
		if navTitle == "" {
			navTitle = strings.ToUpper(typeName)
		}
		types = append(types, doctemplate.WikiIndexType{
			Name:     typeName,
			NavTitle: navTitle,
			Dir:      tc.Dir,
		})
	}

	data := &doctemplate.WikiIndexData{
		SiteName: siteName,
		Types:    types,
	}

	content, err := doctemplate.RenderWikiIndex(tmplContent, data)
	if err != nil {
		return fmt.Errorf("rendering wiki index: %w", err)
	}

	if err := os.WriteFile(indexPath, []byte(content), config.FileMode); err != nil {
		return fmt.Errorf("writing %s: %w", indexPath, err)
	}

	_, err = fmt.Fprintf(r.Out, "Created %s\n", indexPath)
	return err
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
