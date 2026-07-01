// Package cmd implements the docz CLI commands.
package cmd

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/donaldgifford/docz/pkg/doczcore/config"
)

// LogFormat values for the --log-format flag.
const (
	logFormatText = "text"
	logFormatJSON = "json"
)

// runner is the process-wide Runner constructed in PersistentPreRunE.
// Command handlers reach it through method-receiver conversion during
// IMPL-0009 Phase 3 onward. The getRunner accessor below covers the
// transitional period when handler wrappers (`runCreate`, `runConfig`,
// etc.) may be invoked from tests that bypass PersistentPreRunE.
var runner *Runner

// getRunner returns the package-level Runner if it has been constructed
// (the normal production path via PersistentPreRunE) or, as a transitional
// safety net for tests that call handler wrappers directly without
// invoking Cobra, builds an ad-hoc Runner from the current appCfg.
// The ad-hoc Runner captures os.Stdout/os.Stderr at call time, which
// preserves the existing test pattern that redirects os.Stdout via
// os.Pipe to capture output. Once Phase 3 conversion is complete and
// tests construct Runners directly, this fallback will be removed.
func getRunner() *Runner {
	if runner != nil {
		return runner
	}
	return NewRunner(&appCfg)
}

// Runner bundles resolved config with the injectable dependencies that
// command handlers need: writers for output, a slog logger, a time
// source, and a git resolver. Tests construct a Runner directly with
// bytes.Buffer writers and stub Now / Git implementations to avoid
// process-wide side effects.
//
// See docs/design/0004-runner-pattern-and-doctype-registry.md §A for
// the full rationale and field-by-field justification.
type Runner struct {
	Cfg    config.Config
	Out    io.Writer
	Err    io.Writer
	Logger *slog.Logger
	Now    func() time.Time
	Git    GitResolver

	// RepoRoot is the directory cwd-relative path lookups resolve
	// against. Production wires it to os.Getwd() in
	// loadAndValidateConfig; tests set it to t.TempDir() so they can
	// run without an os.Chdir. An empty RepoRoot preserves the
	// pre-refactor "cwd" behavior for any caller that constructs a
	// Runner without setting it.
	RepoRoot string
}

// NewRunner returns a Runner wired with default real-world
// implementations: stdout/stderr writers, a slog.TextHandler at
// LevelInfo writing to stderr, time.Now, and a realGit resolver that
// shells out to `git config user.name`. Callers that need a non-default
// logger (e.g. Cobra's loadAndValidateConfig wiring --verbose,
// --log-level, --log-format) overwrite `r.Logger` after construction.
//
// cfg is taken by pointer per gocritic hugeParam (Config is ~240B);
// the Runner stores a value copy so it owns an immutable snapshot.
func NewRunner(cfg *config.Config) *Runner {
	return &Runner{
		Cfg: *cfg,
		Out: os.Stdout,
		Err: os.Stderr,
		Logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})),
		Now: time.Now,
		Git: realGit{},
	}
}

// buildLogger returns a slog.Logger configured per the CLI flags.
//
// Resolution order for the level: if `level` is non-empty it wins; else
// `verbose` selects debug; else the default is info. This lets users
// pin a specific level with --log-level while keeping --verbose as the
// familiar shorthand for "more output".
//
// `format` selects between the text and JSON slog handlers. Anything
// other than the constants logFormatText / logFormatJSON returns an
// error so a typo at the CLI surfaces immediately instead of silently
// defaulting.
func buildLogger(w io.Writer, verbose bool, level, format string) (*slog.Logger, error) {
	lvl, err := resolveLogLevel(verbose, level)
	if err != nil {
		return nil, err
	}
	opts := &slog.HandlerOptions{Level: lvl}
	switch strings.ToLower(format) {
	case "", logFormatText:
		return slog.New(slog.NewTextHandler(w, opts)), nil
	case logFormatJSON:
		return slog.New(slog.NewJSONHandler(w, opts)), nil
	default:
		return nil, fmt.Errorf(
			"invalid --log-format %q (want %q or %q)",
			format, logFormatText, logFormatJSON,
		)
	}
}

// inRepo resolves name against r.RepoRoot, returning name unchanged
// if it is already absolute or if RepoRoot is empty (preserving the
// pre-refactor cwd-relative behavior for callers that have not opted
// in). When RepoRoot is set, this gives handlers a consistent way to
// reach a docz config file or wiki path without depending on the
// process working directory — which is what lets tests skip os.Chdir.
func (r *Runner) inRepo(name string) string {
	if name == "" || filepath.IsAbs(name) || r.RepoRoot == "" {
		return name
	}
	return filepath.Join(r.RepoRoot, name)
}

// resolveLogLevel picks the slog.Level per the --log-level / --verbose
// flag precedence documented on buildLogger.
func resolveLogLevel(verbose bool, level string) (slog.Level, error) {
	switch strings.ToLower(level) {
	case "":
		if verbose {
			return slog.LevelDebug, nil
		}
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf(
			"invalid --log-level %q (want debug, info, warn, or error)",
			level,
		)
	}
}
