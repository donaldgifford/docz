package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	doctemplate "github.com/donaldgifford/docz/internal/template"
	"github.com/donaldgifford/docz/pkg/doczcore/config"
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

Types: ` + strings.Join(config.DocTypeNames(), ", "),
	Args: cobra.ExactArgs(1),
	RunE: runTemplateShow,
}

var templateExportCmd = &cobra.Command{
	Use:   "export <type> [path]",
	Short: "Export the resolved template to a file",
	Long: `Write the resolved template for the given type to a file.

If no path is specified, the file is written to ./<type>.md in the current directory.

Types: ` + strings.Join(config.DocTypeNames(), ", "),
	Args: cobra.RangeArgs(1, 2),
	RunE: runTemplateExport,
}

var templateOverrideCmd = &cobra.Command{
	Use:   "override <type>",
	Short: "Copy the resolved template into the local overrides directory",
	Long: `Copy the resolved template into <docs_dir>/templates/<type>.md so you can
edit it locally. Future document creation will use this override.

Fails if the override file already exists.

Types: ` + strings.Join(config.DocTypeNames(), ", "),
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
	return getRunner().TemplateShow(args)
}

func runTemplateExport(_ *cobra.Command, args []string) error {
	return getRunner().TemplateExport(args)
}

func runTemplateOverride(_ *cobra.Command, args []string) error {
	return getRunner().TemplateOverride(args)
}

// TemplateShow prints the resolved template for the given document
// type to r.Out.
func (r *Runner) TemplateShow(args []string) error {
	docType, err := r.Cfg.ValidateType(args[0])
	if err != nil {
		return err
	}

	content, err := r.resolveTemplate(docType)
	if err != nil {
		return fmt.Errorf("resolving %s template: %w", docType, err)
	}

	_, err = fmt.Fprint(r.Out, content)
	return err
}

// TemplateExport writes the resolved template to a file (default
// ./<type>.md) and reports the path on r.Out.
func (r *Runner) TemplateExport(args []string) error {
	docType, err := r.Cfg.ValidateType(args[0])
	if err != nil {
		return err
	}

	outPath := r.inRepo(docType + ".md")
	if len(args) > 1 {
		outPath = args[1]
	}

	content, err := r.resolveTemplate(docType)
	if err != nil {
		return fmt.Errorf("resolving %s template: %w", docType, err)
	}

	if err := os.WriteFile(outPath, []byte(content), config.FileMode); err != nil {
		return fmt.Errorf("writing template to %s: %w", outPath, err)
	}

	_, err = fmt.Fprintf(r.Out, "Exported %s template to %s\n", docType, outPath)
	return err
}

// TemplateOverride copies the resolved template into
// <docs_dir>/templates/<type>.md, failing if the override file already
// exists.
func (r *Runner) TemplateOverride(args []string) error {
	docType, err := r.Cfg.ValidateType(args[0])
	if err != nil {
		return err
	}

	overrideDir := filepath.Join(r.Cfg.DocsDir, config.TemplatesDir)
	overridePath := filepath.Join(overrideDir, docType+".md")

	if _, err := os.Stat(overridePath); err == nil {
		return fmt.Errorf("override file already exists: %s", overridePath)
	}

	content, err := r.resolveTemplate(docType)
	if err != nil {
		return fmt.Errorf("resolving %s template: %w", docType, err)
	}

	if err := os.MkdirAll(overrideDir, config.DirMode); err != nil {
		return fmt.Errorf("creating templates directory: %w", err)
	}

	if err := os.WriteFile(overridePath, []byte(content), config.FileMode); err != nil {
		return fmt.Errorf("writing override template: %w", err)
	}

	_, err = fmt.Fprintf(r.Out, "Created override template: %s\n", overridePath)
	return err
}

func (r *Runner) resolveTemplate(docType string) (string, error) {
	tc := r.Cfg.Types[docType]
	r.Logger.Debug("resolving template",
		"type", docType,
		"config_template_path", tc.Template,
		"docs_dir", r.Cfg.DocsDir,
	)
	return doctemplate.Resolve(docType, tc.Template, r.Cfg.DocsDir)
}
