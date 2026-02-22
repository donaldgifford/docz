package cmd

import (
	"fmt"
	"os"
	"path/filepath"

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
		typeDir := appCfg.TypeDir(typeName)
		if err := os.MkdirAll(typeDir, 0o755); err != nil {
			return fmt.Errorf("creating directory %s: %w", typeDir, err)
		}

		readmePath := filepath.Join(typeDir, "README.md")
		if err := writeIndexReadme(readmePath, typeName); err != nil {
			return err
		}
	}

	fmt.Println("Initialized docz successfully.")
	return nil
}

func writeDefaultConfig() error {
	const configPath = ".docz.yaml"

	if _, err := os.Stat(configPath); err == nil {
		if verbose {
			fmt.Printf("Config file %s already exists, skipping.\n", configPath)
		}
		return nil
	}

	content := `docs_dir: docs

types:
  rfc:
    enabled: true
    dir: rfc
    template: ""
    id_prefix: "RFC"
    id_width: 4
    statuses:
      - Draft
      - Proposed
      - Accepted
      - Rejected
      - Superseded
    status_field: "status"

  adr:
    enabled: true
    dir: adr
    template: ""
    id_prefix: "ADR"
    id_width: 4
    statuses:
      - Proposed
      - Accepted
      - Deprecated
      - Superseded
    status_field: "status"

  design:
    enabled: true
    dir: design
    template: ""
    id_prefix: "DESIGN"
    id_width: 4
    statuses:
      - Draft
      - In Review
      - Approved
      - Implemented
      - Abandoned
    status_field: "status"

  impl:
    enabled: true
    dir: impl
    template: ""
    id_prefix: "IMPL"
    id_width: 4
    statuses:
      - Draft
      - In Progress
      - Completed
      - Paused
      - Cancelled
    status_field: "status"

index:
  auto_update: true
  preserve_header: true

author:
  from_git: true
  default: ""
`

	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	fmt.Printf("Created %s\n", configPath)
	return nil
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

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	fmt.Printf("Created %s\n", path)
	return nil
}
