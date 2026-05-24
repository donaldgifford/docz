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
	if err := writeDefaultConfig(); err != nil {
		return err
	}

	for _, typeName := range config.ValidTypes() {
		tc, ok := appCfg.Types[typeName]
		if !ok || !tc.Enabled {
			if verbose {
				fmt.Fprintf(os.Stderr, "Type %s is disabled, skipping.\n", typeName)
			}
			continue
		}

		typeDir := appCfg.TypeDir(typeName)
		if err := os.MkdirAll(typeDir, config.DirMode); err != nil {
			return fmt.Errorf("creating directory %s: %w", typeDir, err)
		}

		readmePath := filepath.Join(typeDir, config.IndexFileName)
		if err := writeIndexReadme(readmePath, typeName); err != nil {
			return err
		}
	}

	fmt.Println("Initialized docz successfully.")
	return nil
}

func writeDefaultConfig() error {
	configPath := config.ConfigFileName

	if _, err := os.Stat(configPath); err == nil {
		if verbose {
			fmt.Printf("Config file %s already exists, skipping.\n", configPath)
		}
		return nil
	}

	content, err := renderDefaultConfig()
	if err != nil {
		return err
	}

	if err := os.WriteFile(configPath, []byte(content), config.FileMode); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	fmt.Printf("Created %s\n", configPath)
	return nil
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

func writeIndexReadme(path, typeName string) error {
	if !forceInit {
		if _, err := os.Stat(path); err == nil {
			if verbose {
				fmt.Printf("README %s already exists, skipping (use --force to overwrite).\n", path)
			}
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

	fmt.Printf("Created %s\n", path)
	return nil
}
