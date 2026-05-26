package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/donaldgifford/docz/internal/config"
	"github.com/donaldgifford/docz/internal/document"
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

	docs, err := document.ScanDocuments(typeDir)
	if err != nil {
		return fmt.Errorf("scanning %s: %w", typeDir, err)
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "  Found %d documents\n", len(docs))
	}

	// Update ToC in each document before regenerating the README index.
	if appCfg.ToC.Enabled {
		runToCUpdate(typeDir, docs)
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

// runToCUpdate builds the toc.FileInput list from cached scan results,
// delegates to toc.UpdateFiles, and formats user-facing messages so the
// internal/toc package stays free of I/O-shaped strings.
func runToCUpdate(typeDir string, docs []document.DocEntry) {
	if len(docs) == 0 {
		return
	}

	files := make([]toc.FileInput, len(docs))
	for i, doc := range docs {
		files[i] = toc.FileInput{
			Path:    filepath.Join(typeDir, doc.Filename),
			Content: doc.Content,
		}
	}

	report, err := toc.UpdateFiles(files, appCfg.ToC.MinHeadings, updateDryRun)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: ToC update failed: %v\n", err)
		return
	}

	for _, r := range report.WouldUpdate {
		fmt.Printf("Would update ToC in %s (%d headings)\n", r.Path, r.Headings)
	}

	if verbose {
		for _, r := range report.Updated {
			fmt.Fprintf(os.Stderr, "  Updated ToC in %s\n", r.Path)
		}
		for _, r := range report.Unchanged {
			fmt.Fprintf(os.Stderr, "  ToC unchanged in %s\n", r.Path)
		}
	}

	for _, fe := range report.WriteErrors {
		fmt.Fprintf(os.Stderr, "Warning: writing ToC to %s: %v\n", fe.Path, fe.Err)
	}
}
