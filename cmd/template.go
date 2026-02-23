package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/donaldgifford/docz/internal/config"
	doctemplate "github.com/donaldgifford/docz/internal/template"
)

var templateCmd = &cobra.Command{
	Use:   "template",
	Short: "Manage document templates",
	Long: `View, export, and override document templates.

Subcommands:
  show      Print the resolved template for a type
  export    Write the resolved template to a file
  override  Copy the resolved template into the local overrides directory`,
}

var templateShowCmd = &cobra.Command{
	Use:   "show <type>",
	Short: "Print the resolved template for a document type",
	Long: `Print the resolved template for the given document type to stdout.

The template is resolved in order: config path > local override > embedded default.

Types: ` + strings.Join(config.ValidTypes(), ", "),
	Args: cobra.ExactArgs(1),
	RunE: runTemplateShow,
}

var templateExportCmd = &cobra.Command{
	Use:   "export <type> [path]",
	Short: "Export the resolved template to a file",
	Long: `Write the resolved template for the given type to a file.

If no path is specified, the file is written to ./<type>.md in the current directory.

Types: ` + strings.Join(config.ValidTypes(), ", "),
	Args: cobra.RangeArgs(1, 2),
	RunE: runTemplateExport,
}

var templateOverrideCmd = &cobra.Command{
	Use:   "override <type>",
	Short: "Copy the resolved template into the local overrides directory",
	Long: `Copy the resolved template into <docs_dir>/templates/<type>.md so you can
edit it locally. Future document creation will use this override.

Fails if the override file already exists.

Types: ` + strings.Join(config.ValidTypes(), ", "),
	Args: cobra.ExactArgs(1),
	RunE: runTemplateOverride,
}

func init() {
	templateCmd.AddCommand(templateShowCmd)
	templateCmd.AddCommand(templateExportCmd)
	templateCmd.AddCommand(templateOverrideCmd)
	rootCmd.AddCommand(templateCmd)
}

func runTemplateShow(_ *cobra.Command, args []string) error {
	docType := strings.ToLower(args[0])
	if err := validateType(docType); err != nil {
		return err
	}

	content, err := resolveTemplate(docType)
	if err != nil {
		return err
	}

	fmt.Print(content)
	return nil
}

func runTemplateExport(_ *cobra.Command, args []string) error {
	docType := strings.ToLower(args[0])
	if err := validateType(docType); err != nil {
		return err
	}

	outPath := docType + ".md"
	if len(args) > 1 {
		outPath = args[1]
	}

	content, err := resolveTemplate(docType)
	if err != nil {
		return err
	}

	if err := os.WriteFile(outPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing template to %s: %w", outPath, err)
	}

	fmt.Printf("Exported %s template to %s\n", docType, outPath)
	return nil
}

func runTemplateOverride(_ *cobra.Command, args []string) error {
	docType := strings.ToLower(args[0])
	if err := validateType(docType); err != nil {
		return err
	}

	overrideDir := filepath.Join(appCfg.DocsDir, "templates")
	overridePath := filepath.Join(overrideDir, docType+".md")

	if _, err := os.Stat(overridePath); err == nil {
		return fmt.Errorf("override file already exists: %s", overridePath)
	}

	content, err := resolveTemplate(docType)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(overrideDir, 0o750); err != nil {
		return fmt.Errorf("creating templates directory: %w", err)
	}

	if err := os.WriteFile(overridePath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing override template: %w", err)
	}

	fmt.Printf("Created override template: %s\n", overridePath)
	return nil
}

func validateType(docType string) error {
	if _, ok := appCfg.Types[docType]; !ok {
		return fmt.Errorf("unknown document type %q (valid types: %s)",
			docType, strings.Join(config.ValidTypes(), ", "))
	}
	return nil
}

func resolveTemplate(docType string) (string, error) {
	tc := appCfg.Types[docType]
	return doctemplate.Resolve(docType, tc.Template, appCfg.DocsDir)
}
