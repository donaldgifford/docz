package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/donaldgifford/docz/internal/config"
	"github.com/donaldgifford/docz/internal/index"
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
	types := config.ValidTypes()
	if len(args) > 0 {
		typeName := config.ResolveTypeAlias(strings.ToLower(args[0]))
		if _, ok := appCfg.Types[typeName]; !ok {
			return fmt.Errorf("unknown document type %q (valid types: %s)",
				typeName, strings.Join(config.ValidTypes(), ", "))
		}
		types = []string{typeName}
	}

	for _, typeName := range types {
		if err := updateType(typeName); err != nil {
			return err
		}
	}

	return nil
}

func updateType(typeName string) error {
	typeDir := appCfg.TypeDir(typeName)
	readmePath := filepath.Join(typeDir, "README.md")

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

	heading := "All " + strings.ToUpper(typeName) + "s"
	if typeName == "adr" {
		heading = "All ADRs"
	}
	tableContent := index.GenerateTable(docs, heading)

	if updateDryRun {
		result, err := index.DryRunReadme(readmePath, typeName, tableContent)
		if err != nil {
			return err
		}
		fmt.Println(result)
		return nil
	}

	msg, err := index.UpdateReadme(readmePath, typeName, tableContent)
	if err != nil {
		return err
	}

	fmt.Println(msg)
	return nil
}
