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
package cmd

import (
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
	Long: `docz generates and manages standardized documentation files (RFC, ADR,
DESIGN, IMPL) from templates. It creates documents with auto-incremented IDs,
YAML frontmatter, and auto-generated index pages.

Document types:
  rfc      Request for Comments — high-level proposals
  adr      Architecture Decision Records — technical decisions
  design   Design documents — detailed feature designs
  impl     Implementation plans — concrete tasks and milestones`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is .docz.yaml in repo root)")
	rootCmd.PersistentFlags().StringVar(&docsDir, "docs-dir", "", "base documentation directory (default: docs)")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "enable verbose output")
}

func initConfig() {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		cfg = config.DefaultConfig()
	}

	if docsDir != "" {
		cfg.DocsDir = docsDir
	}

	appCfg = cfg
}
