// Package cmd implements the docz CLI commands.
package cmd

import (
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/donaldgifford/docz/internal/config"
)

// runner is the process-wide Runner constructed in PersistentPreRunE.
// Command handlers will reach it through method-receiver conversion
// during IMPL-0009 Phase 3 onward.
var runner *Runner

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
}

// NewRunner returns a Runner wired with default real-world
// implementations: stdout/stderr writers, a slog.TextHandler at
// LevelInfo writing to stderr, time.Now, and a realGit resolver that
// shells out to `git config user.name`. IMPL-0009 Phase 4 will wire the
// logger level to the --verbose flag and add --log-level / --log-format
// flags.
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
