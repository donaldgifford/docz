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
	// Auto-run docz init if not already initialized.
	if err := ensureDoczInit(); err != nil {
		return fmt.Errorf("ensuring docz init: %w", err)
	}

	mkdocsPath := appCfg.Wiki.MkDocsPath

	if !wikiForce {
		if _, err := os.Stat(mkdocsPath); err == nil {
			return fmt.Errorf(
				"%s already exists (use --force to overwrite)",
				mkdocsPath,
			)
		}
	}

	siteName := wikiSiteName
	if siteName == "" {
		siteName = repoName()
	}

	siteDesc := wikiSiteDescription
	if siteDesc == "" {
		siteDesc = "Documentation for " + siteName
	}

	mkdocsCfg := &wiki.MkDocsConfig{
		SiteName:           siteName,
		SiteDescription:    siteDesc,
		DocsDir:            appCfg.Wiki.DocsDir,
		RepoURL:            appCfg.Wiki.RepoURL,
		SiteURL:            appCfg.Wiki.SiteURL,
		Theme:              appCfg.Wiki.Theme,
		Plugins:            appCfg.Wiki.Plugins,
		MarkdownExtensions: appCfg.Wiki.MarkdownExtensions,
	}
	if err := wiki.CreateMkDocs(mkdocsPath, mkdocsCfg); err != nil {
		return fmt.Errorf("writing %s: %w", mkdocsPath, err)
	}

	fmt.Printf("Created %s\n", mkdocsPath)

	if err := ensureDocsIndex(siteName); err != nil {
		return fmt.Errorf("ensuring docs index: %w", err)
	}

	// Populate the nav from existing docs.
	return runWikiUpdateNav(mkdocsPath)
}

func runWikiUpdate(_ *cobra.Command, _ []string) error {
	mkdocsPath := appCfg.Wiki.MkDocsPath

	if _, err := os.Stat(mkdocsPath); errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf(
			"%s not found (run `docz wiki init` first)",
			mkdocsPath,
		)
	}

	if wikiDryRun {
		return runWikiUpdateDryRun(mkdocsPath)
	}

	return runWikiUpdateNav(mkdocsPath)
}

// runWikiUpdateNav scans the docs directory and updates the nav in mkdocs.yml.
func runWikiUpdateNav(mkdocsPath string) error {
	data, err := wiki.ReadMkDocs(mkdocsPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", mkdocsPath, err)
	}
	existingOrder := wiki.ExistingNavOrder(data)
	logScan(existingOrder)

	entries, err := wiki.BuildNav(
		appCfg.DocsDir,
		appCfg.Wiki.Exclude,
		appCfg.Wiki.NavTitles,
		existingOrder,
	)
	if err != nil {
		return fmt.Errorf("scanning docs: %w", err)
	}
	logScanResult(entries)

	data["nav"] = wiki.NavToYAML(entries)
	if err := wiki.WriteMkDocs(mkdocsPath, data); err != nil {
		return fmt.Errorf("writing %s: %w", mkdocsPath, err)
	}

	fmt.Printf("Updated nav in %s (%d pages)\n", mkdocsPath, wiki.CountPages(entries))
	return nil
}

func logScan(existingOrder []string) {
	if !verbose {
		return
	}
	fmt.Fprintf(os.Stderr, "Scanning %s for documents...\n", appCfg.DocsDir)
	fmt.Fprintf(os.Stderr, "  Excluding: %v\n", appCfg.Wiki.Exclude)
	if len(existingOrder) > 0 {
		fmt.Fprintf(os.Stderr, "Preserving existing nav order: %v\n", existingOrder)
	} else {
		fmt.Fprintln(os.Stderr, "No existing nav order, sorting alphabetically")
	}
}

func logScanResult(entries []wiki.NavEntry) {
	if !verbose {
		return
	}
	fmt.Fprintf(os.Stderr, "  Found %d top-level entries\n", len(entries))
	printNavDebug(entries, "")
}

func runWikiUpdateDryRun(mkdocsPath string) error {
	data, err := wiki.ReadMkDocs(mkdocsPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", mkdocsPath, err)
	}

	entries, err := wiki.BuildNav(
		appCfg.DocsDir,
		appCfg.Wiki.Exclude,
		appCfg.Wiki.NavTitles,
		wiki.ExistingNavOrder(data),
	)
	if err != nil {
		return fmt.Errorf("scanning docs: %w", err)
	}

	printNav(entries, "")
	return nil
}

// ensureDoczInit checks if docz has been initialized and runs init if not.
func ensureDoczInit() error {
	configExists := false
	if _, err := os.Stat(config.ConfigFileName); err == nil {
		configExists = true
	}

	docsExists := false
	if info, err := os.Stat(appCfg.DocsDir); err == nil && info.IsDir() {
		docsExists = true
	}

	if configExists && docsExists {
		if verbose {
			fmt.Fprintln(os.Stderr, "docz already initialized, skipping init")
		}
		return nil
	}

	if verbose {
		fmt.Fprintln(os.Stderr, "Running docz init...")
	}

	return runInit(nil, nil)
}

func repoName() string {
	dir, err := os.Getwd()
	if err != nil {
		return "my-project"
	}
	return filepath.Base(dir)
}

func ensureDocsIndex(siteName string) error {
	indexPath := filepath.Join(appCfg.DocsDir, config.WikiIndexName)

	if _, err := os.Stat(indexPath); err == nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "%s already exists, skipping\n", indexPath)
		}
		return nil
	}

	tmplContent, err := doctemplate.ResolveWikiIndex(appCfg.DocsDir)
	if err != nil {
		return fmt.Errorf("resolving wiki index template: %w", err)
	}

	enabled := appCfg.EnabledTypes()
	types := make([]doctemplate.WikiIndexType, 0, len(enabled))
	for _, typeName := range enabled {
		tc := appCfg.Types[typeName]
		// WikiConfig.NavTitles wins over PluralLabel (Decisions §4
		// back-compat); PluralLabel is the fallback. Capitalized
		// typeName is only used as a last-resort label if neither is
		// configured.
		navTitle := appCfg.Wiki.NavTitles[typeName]
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

	fmt.Printf("Created %s\n", indexPath)
	return nil
}

func printNav(entries []wiki.NavEntry, indent string) {
	for i := range entries {
		e := &entries[i]
		if len(e.Children) > 0 {
			fmt.Printf("%s- %s:\n", indent, e.Title)
			printNav(e.Children, indent+"    ")
		} else {
			fmt.Printf("%s- %s: %s\n", indent, e.Title, e.Path)
		}
	}
}

func printNavDebug(entries []wiki.NavEntry, indent string) {
	for i := range entries {
		e := &entries[i]
		if len(e.Children) > 0 {
			fmt.Fprintf(os.Stderr, "%s[dir] %s (%d children)\n", indent, e.Title, len(e.Children))
			printNavDebug(e.Children, indent+"  ")
		} else {
			fmt.Fprintf(os.Stderr, "%s[page] %s → %s\n", indent, e.Title, e.Path)
		}
	}
}
