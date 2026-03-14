package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/donaldgifford/docz/internal/config"
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
		return err
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

	if err := writeMkDocsYAML(mkdocsPath, siteName, siteDesc); err != nil {
		return err
	}

	fmt.Printf("Created %s\n", mkdocsPath)

	if err := ensureDocsIndex(siteName); err != nil {
		return err
	}

	// Populate the nav from existing docs.
	return runWikiUpdateNav(mkdocsPath)
}

func runWikiUpdate(_ *cobra.Command, _ []string) error {
	mkdocsPath := appCfg.Wiki.MkDocsPath

	if _, err := os.Stat(mkdocsPath); os.IsNotExist(err) {
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
	entries, err := wiki.ScanDocs(
		appCfg.DocsDir,
		appCfg.Wiki.Exclude,
		appCfg.Wiki.NavTitles,
	)
	if err != nil {
		return fmt.Errorf("scanning docs: %w", err)
	}

	data, err := wiki.ReadMkDocs(mkdocsPath)
	if err != nil {
		return err
	}

	existingOrder := wiki.ExistingNavOrder(data)
	if len(existingOrder) > 0 {
		entries = wiki.MergeNavOrder(existingOrder, entries)
	} else {
		entries = wiki.SortEntries(entries)
	}

	if verbose {
		printNavDebug(entries, "")
	}

	data["nav"] = wiki.NavToYAML(entries)

	if err := wiki.WriteMkDocs(mkdocsPath, data); err != nil {
		return err
	}

	pageCount := wiki.CountPages(entries)
	fmt.Printf("Updated nav in %s (%d pages)\n", mkdocsPath, pageCount)
	return nil
}

func runWikiUpdateDryRun(mkdocsPath string) error {
	entries, err := wiki.ScanDocs(
		appCfg.DocsDir,
		appCfg.Wiki.Exclude,
		appCfg.Wiki.NavTitles,
	)
	if err != nil {
		return fmt.Errorf("scanning docs: %w", err)
	}

	data, err := wiki.ReadMkDocs(mkdocsPath)
	if err != nil {
		return err
	}

	existingOrder := wiki.ExistingNavOrder(data)
	if len(existingOrder) > 0 {
		entries = wiki.MergeNavOrder(existingOrder, entries)
	} else {
		entries = wiki.SortEntries(entries)
	}

	printNav(entries, "")
	return nil
}

// ensureDoczInit checks if docz has been initialized and runs init if not.
func ensureDoczInit() error {
	configExists := false
	if _, err := os.Stat(".docz.yaml"); err == nil {
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

func writeMkDocsYAML(path, siteName, siteDesc string) error {
	content := fmt.Sprintf(`site_name: %s
site_description: %s

plugins:
    - techdocs-core

nav:
    - Home: index.md
`, siteName, siteDesc)

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	return nil
}

func ensureDocsIndex(siteName string) error {
	indexPath := filepath.Join(appCfg.DocsDir, "index.md")

	if _, err := os.Stat(indexPath); err == nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "%s already exists, skipping\n", indexPath)
		}
		return nil
	}

	var b strings.Builder
	b.WriteString("# " + siteName + "\n\n")
	b.WriteString("Welcome to the documentation for " + siteName + ".\n\n")
	b.WriteString("## Document Types\n\n")

	for _, typeName := range config.ValidTypes() {
		tc := appCfg.Types[typeName]
		navTitle := appCfg.Wiki.NavTitles[typeName]
		if navTitle == "" {
			navTitle = strings.ToUpper(typeName)
		}
		b.WriteString(
			fmt.Sprintf("- [%s](%s/README.md)\n", navTitle, tc.Dir),
		)
	}

	if err := os.WriteFile(indexPath, []byte(b.String()), 0o644); err != nil {
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
