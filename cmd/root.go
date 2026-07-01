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
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/donaldgifford/docz/pkg/doczcore/config"
)

var (
	cfgFile   string
	repoRoot  string
	docsDir   string
	verbose   bool
	logLevel  string
	logFormat string
	appCfg    config.Config
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
	if err := rootCmd.Execute(); err != nil {
		os.Exit(exitCodeFor(err))
	}
}

// exitCodeFor maps a command error to a process exit code. Only the
// errExitCode2 validation marker is special-cased; errExitCode1 and every
// other failure exit 1, preserving the pre-existing behavior.
func exitCodeFor(err error) int {
	if errors.Is(err, errExitCode2) {
		return 2
	}
	return 1
}

func init() {
	rootCmd.PersistentFlags().StringVar(
		&cfgFile,
		"config",
		"",
		"config file (default is .docz.yaml in repo root)",
	)
	rootCmd.PersistentFlags().StringVar(
		&repoRoot,
		"repo-root",
		"",
		"repository root to scan for .docz.yaml and to scaffold against (default: working directory)",
	)
	rootCmd.PersistentFlags().StringVar(
		&docsDir,
		"docs-dir",
		"",
		"base documentation directory (default: docs)",
	)
	rootCmd.PersistentFlags().BoolVar(
		&verbose,
		"verbose",
		false,
		"shorthand for --log-level=debug",
	)
	rootCmd.PersistentFlags().StringVar(
		&logLevel,
		"log-level",
		"",
		"log level: debug, info, warn, error (overrides --verbose)",
	)
	rootCmd.PersistentFlags().StringVar(
		&logFormat,
		"log-format",
		logFormatText,
		"log handler format: text or json",
	)
}

// loadAndValidateConfig is wired as the rootCmd PersistentPreRunE so a
// broken .docz.yaml causes a hard, non-zero exit at startup instead of
// silently printing warnings and continuing with a half-defaulted config.
// Cobra short-circuits PersistentPreRunE when --help/-h is set or no
// runnable subcommand was given, so help still works with a broken config.
func loadAndValidateConfig(_ *cobra.Command, _ []string) error {
	// Precedence for the repo root: explicit --repo-root flag, else
	// directory of --config when that's set, else process cwd. The
	// repo-root knob lets tests drive PersistentPreRunE without
	// os.Chdir and lets users scaffold a different tree than the
	// directory they invoked from.
	root, err := resolveRepoRoot()
	if err != nil {
		return err
	}

	cfg, err := config.Load(cfgFile, root)
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

	// Absolutize cwd-relative config paths against root so handlers
	// don't carry an implicit dependency on the process cwd.
	if !filepath.IsAbs(cfg.DocsDir) {
		cfg.DocsDir = filepath.Join(root, cfg.DocsDir)
	}
	if cfg.Wiki.MkDocsPath != "" && !filepath.IsAbs(cfg.Wiki.MkDocsPath) {
		cfg.Wiki.MkDocsPath = filepath.Join(root, cfg.Wiki.MkDocsPath)
	}

	appCfg = cfg
	r := NewRunner(&cfg)
	r.RepoRoot = root
	logger, err := buildLogger(r.Err, verbose, logLevel, logFormat)
	if err != nil {
		return err
	}
	r.Logger = logger
	runner = r
	return nil
}

// resolveRepoRoot picks the directory PersistentPreRunE should treat as
// the repo root. Precedence: explicit --repo-root flag, else directory
// of --config when set, else process cwd.
func resolveRepoRoot() (string, error) {
	if repoRoot != "" {
		return repoRoot, nil
	}
	if cfgFile != "" {
		return filepath.Dir(cfgFile), nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolving working directory: %w", err)
	}
	return wd, nil
}
