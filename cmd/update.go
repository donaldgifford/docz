package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/donaldgifford/docz/internal/config"
	"github.com/donaldgifford/docz/internal/index"
	"github.com/donaldgifford/docz/internal/toc"
)

var updateDryRun bool

var updateCmd = &cobra.Command{
	Use:   "update [type]",
	Short: "Update index/README for a document type (or all types)",
	Long: `Regenerate the auto-generated table in the README.md for the specified
document type directory. If no type is given, all types are updated.

` + config.TypesHelp(),
	Args: cobra.MaximumNArgs(1),
	RunE: runUpdate,
}

func init() {
	updateCmd.Flags().BoolVar(&updateDryRun, "dry-run", false, "show what would change without writing")
	rootCmd.AddCommand(updateCmd)
}

func runUpdate(_ *cobra.Command, args []string) error {
	var types []string
	if len(args) > 0 {
		typeName, err := appCfg.ValidateType(args[0])
		if err != nil {
			return err
		}
		types = []string{typeName}
	} else {
		types = appCfg.EnabledTypes()
	}

	for _, typeName := range types {
		tc, ok := appCfg.Types[typeName]
		if !ok || !tc.Enabled {
			if verbose {
				fmt.Fprintf(os.Stderr, "Type %s is disabled, skipping.\n", typeName)
			}
			continue
		}

		if err := updateType(typeName); err != nil {
			return fmt.Errorf("updating %s: %w", typeName, err)
		}
	}

	return nil
}

func updateType(typeName string) error {
	tc := appCfg.Types[typeName]
	typeDir := appCfg.TypeDir(typeName)
	readmePath := filepath.Join(typeDir, config.IndexFileName)

	if verbose {
		fmt.Fprintf(os.Stderr, "Scanning %s for documents...\n", typeDir)
	}

	docs, err := index.ScanDocuments(typeDir)
	if err != nil {
		return fmt.Errorf("scanning %s: %w", typeDir, err)
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "  Found %d documents\n", len(docs))
	}

	// Update ToC in each document before regenerating the README index.
	if appCfg.ToC.Enabled {
		updateToCs(typeDir, docs)
	}

	heading := "All " + tc.PluralLabel
	tableContent := index.GenerateTable(docs, heading)

	if updateDryRun {
		result, err := index.DryRunReadme(readmePath, typeName, tableContent)
		if err != nil {
			return fmt.Errorf("dry-run readme %s: %w", readmePath, err)
		}
		fmt.Println(result)
		return nil
	}

	msg, err := index.UpdateReadme(readmePath, typeName, tableContent)
	if err != nil {
		return fmt.Errorf("updating readme %s: %w", readmePath, err)
	}

	fmt.Println(msg)
	return nil
}

// updateToCs updates the table of contents in each document file that has
// ToC markers. Errors are logged as warnings but do not stop processing.
//
// IMPL-0007 Phase 3: operates on the bytes cached on DocEntry.Content
// (populated during ScanDocuments) so each document is read once per
// `docz update`, not twice.
func updateToCs(typeDir string, docs []index.DocEntry) {
	for _, doc := range docs {
		docPath := filepath.Join(typeDir, doc.Filename)
		original := string(doc.Content)

		updated, found := toc.UpdateToC(original, appCfg.ToC.MinHeadings)
		if !found {
			continue
		}

		if updated == original {
			if verbose {
				fmt.Fprintf(os.Stderr, "  ToC unchanged in %s\n", docPath)
			}
			continue
		}

		if updateDryRun {
			headings := toc.ParseHeadings(original)
			fmt.Printf(
				"Would update ToC in %s (%d headings)\n",
				docPath,
				len(headings),
			)
			continue
		}

		if err := os.WriteFile(docPath, []byte(updated), config.FileMode); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: writing ToC to %s: %v\n", docPath, err)
			continue
		}

		if verbose {
			fmt.Fprintf(os.Stderr, "  Updated ToC in %s\n", docPath)
		}
	}
}
