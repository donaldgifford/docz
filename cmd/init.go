package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/spf13/cobra"

	"github.com/donaldgifford/docz/internal/config"
	doctemplate "github.com/donaldgifford/docz/internal/template"
)

var forceInit bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize docz in the current repository",
	Long: `Create a .docz.yaml configuration file and set up the documentation
directory structure with default README index files for each document type.

If .docz.yaml already exists and declares a top-level "types:" block,
only the types listed there are scaffolded. Omit the "types:" block (or
delete .docz.yaml entirely and let init regenerate it) to scaffold all
six built-in types.

Existing README files are not overwritten unless --force is passed.`,
	RunE: runInit,
}

func init() {
	initCmd.Flags().BoolVar(&forceInit, "force", false, "overwrite existing README index files")
	rootCmd.AddCommand(initCmd)
}

func runInit(_ *cobra.Command, _ []string) error {
	return getRunner().Init(forceInit)
}

// Init scaffolds .docz.yaml plus a README index per enabled doc type.
// Existing files are skipped unless force is true.
func (r *Runner) Init(force bool) error {
	if err := r.writeDefaultConfig(); err != nil {
		return fmt.Errorf("writing default config: %w", err)
	}

	for _, typeName := range r.Cfg.EnabledTypes() {
		typeDir := r.Cfg.TypeDir(typeName)
		if err := os.MkdirAll(typeDir, config.DirMode); err != nil {
			return fmt.Errorf("creating directory %s: %w", typeDir, err)
		}

		readmePath := filepath.Join(typeDir, config.IndexFileName)
		if err := r.writeIndexReadme(readmePath, typeName, force); err != nil {
			return fmt.Errorf("writing index readme for %s: %w", typeName, err)
		}
	}

	_, err := fmt.Fprintln(r.Out, "Initialized docz successfully.")
	return err
}

func (r *Runner) writeDefaultConfig() error {
	configPath := config.ConfigFileName

	if _, err := os.Stat(configPath); err == nil {
		r.Logger.Debug("config file exists, skipping", "path", configPath)
		return nil
	}

	content, err := renderDefaultConfig()
	if err != nil {
		return fmt.Errorf("rendering default config: %w", err)
	}

	if err := os.WriteFile(configPath, []byte(content), config.FileMode); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	_, err = fmt.Fprintf(r.Out, "Created %s\n", configPath)
	return err
}

// renderDefaultConfig renders the embedded .docz.yaml template using
// config.DefaultConfig() as the template data, so the generated file is
// derived from the same source the binary uses at runtime.
func renderDefaultConfig() (string, error) {
	tmplSrc, err := doctemplate.EmbeddedDoczYAML()
	if err != nil {
		return "", fmt.Errorf("loading docz yaml template: %w", err)
	}

	tmpl, err := template.New("docz_yaml").Parse(tmplSrc)
	if err != nil {
		return "", fmt.Errorf("parsing docz yaml template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, config.DefaultConfig()); err != nil {
		return "", fmt.Errorf("rendering docz yaml template: %w", err)
	}

	return buf.String(), nil
}

func (r *Runner) writeIndexReadme(path, typeName string, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			r.Logger.Debug("readme exists, skipping",
				"path", path, "hint", "use --force to overwrite")
			return nil
		}
	}

	header, err := doctemplate.EmbeddedIndexHeader(typeName)
	if err != nil {
		return fmt.Errorf("loading index header for %s: %w", typeName, err)
	}

	content := header +
		"<!-- BEGIN DOCZ AUTO-GENERATED -->\n" +
		"<!-- END DOCZ AUTO-GENERATED -->\n"

	if err := os.WriteFile(path, []byte(content), config.FileMode); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	_, err = fmt.Fprintf(r.Out, "Created %s\n", path)
	return err
}
