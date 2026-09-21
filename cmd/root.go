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
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/repo"
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
//
// The context it runs under is cancelled on SIGINT, which is what makes
// Ctrl-C stop a long `docz update` between types instead of mid-file
// (DESIGN-0014 §7). Cancellation is cooperative: the repo API checks it
// between per-type iterations and returns the report completed so far, so
// what was already written stays written.
//
// stop is called before any os.Exit rather than deferred, because
// os.Exit does not run deferred functions and a defer here would only
// look like it did.
func Execute() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)

	err := rootCmd.ExecuteContext(ctx)

	stop()

	if err != nil {
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
func loadAndValidateConfig(cmd *cobra.Command, _ []string) error {
	// Precedence for the repo root: explicit --repo-root flag, else
	// directory of --config when that's set, else process cwd. The
	// repo-root knob lets tests drive PersistentPreRunE without
	// os.Chdir and lets users scaffold a different tree than the
	// directory they invoked from.
	root, err := resolveRepoRoot()
	if err != nil {
		return err
	}

	// repo.Open is the library's own load: config.Load then Validate,
	// wrapped with the same two messages this function used to build by
	// hand. Everything below adjusts the config it returned, so there is
	// one config in the process and the Repo owns it.
	rp, err := repo.Open(cmd.Context(), root, cfgFile)
	if err != nil {
		return err
	}

	if docsDir != "" {
		rp.Cfg.DocsDir = docsDir
	}

	// Validated a second time, deliberately. Open discards the warnings
	// because they are a presentation concern it has no business
	// printing, and this is where they get printed; re-running it also
	// puts a --docs-dir override back inside validation, where it was
	// before the swap. Validate is pure and in-memory, so the only cost
	// is the call.
	warnings, validErr := rp.Cfg.Validate()
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", w)
	}
	if validErr != nil {
		return fmt.Errorf("invalid config: %w", validErr)
	}

	// Absolutize cwd-relative config paths against root so handlers
	// don't carry an implicit dependency on the process cwd. The repo
	// package is indifferent — Repo.Path passes an absolute path through
	// — but cmd's own helpers join DocsDir directly.
	if !filepath.IsAbs(rp.Cfg.DocsDir) {
		rp.Cfg.DocsDir = filepath.Join(root, rp.Cfg.DocsDir)
	}
	if rp.Cfg.Wiki.MkDocsPath != "" && !filepath.IsAbs(rp.Cfg.Wiki.MkDocsPath) {
		rp.Cfg.Wiki.MkDocsPath = filepath.Join(root, rp.Cfg.Wiki.MkDocsPath)
	}

	appCfg = *rp.Cfg
	r := NewRunner(rp.Cfg)
	r.RepoRoot = root

	// Point the Repo back at the Runner's copy so there is one config
	// rather than two that merely start out equal. A handler that adjusts
	// r.Cfg and the Repo it orchestrates through cannot then disagree.
	rp.Cfg = &r.Cfg
	r.Repo = rp
	logger, err := buildLogger(r.Err, verbose, logLevel, logFormat)
	if err != nil {
		return err
	}
	r.Logger = logger
	runner = r

	// Hooks ride the context from here down, so every handler's
	// r.Repo.<Op>(cmd.Context(), …) narrates onto this logger without any
	// handler knowing that is what happens. SetContext is on the leaf
	// command Cobra hands PersistentPreRunE, which is the same command
	// whose RunE reads cmd.Context() next.
	cmd.SetContext(repo.WithHooks(cmd.Context(), r.hooks()))

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
