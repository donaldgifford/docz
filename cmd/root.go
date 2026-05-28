/*
Copyright © 2026 Donald Gifford

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package cmd implements the docz CLI commands.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/donaldgifford/docz/internal/config"
)

var (
	cfgFile string
	docsDir string
	verbose bool
	appCfg  config.Config
)

var rootCmd = &cobra.Command{
	Use:   "docz",
	Short: "A CLI tool for managing standardized repository documentation",
	Long: `docz generates and manages standardized documentation files from templates.
It creates documents with auto-incremented IDs, YAML frontmatter, and
auto-generated index pages.

` + config.TypesHelp(),
	SilenceUsage:      true,
	PersistentPreRunE: loadAndValidateConfig,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is .docz.yaml in repo root)")
	rootCmd.PersistentFlags().StringVar(&docsDir, "docs-dir", "", "base documentation directory (default: docs)")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "enable verbose output")
}

// loadAndValidateConfig is wired as the rootCmd PersistentPreRunE so a
// broken .docz.yaml causes a hard, non-zero exit at startup instead of
// silently printing warnings and continuing with a half-defaulted config.
// Cobra short-circuits PersistentPreRunE when --help/-h is set or no
// runnable subcommand was given, so help still works with a broken config.
func loadAndValidateConfig(_ *cobra.Command, _ []string) error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolving working directory: %w", err)
	}

	cfg, err := config.Load(cfgFile, repoRoot)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if docsDir != "" {
		cfg.DocsDir = docsDir
	}

	warnings, validErr := cfg.Validate()
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", w)
	}
	if validErr != nil {
		return fmt.Errorf("invalid config: %w", validErr)
	}

	appCfg = cfg
	runner = NewRunner(&cfg)
	return nil
}
