// Package repo turns the docz primitives into the operations a consumer
// would otherwise copy out of cmd/.
//
// EXPERIMENTAL until v2.0.0: the surface may change between betas
// (ADR-0002 Decision 7). The five packages frozen at v1.0.0 are not
// affected; this one is not among them.
//
// A Repo is a root directory and a loaded config. Every method is the
// orchestration one cmd/ handler performs today with the printing removed:
// Create, Update, SetStatus, Init, Validate, InsertRegions, Template, and
// the reads. Every method takes a context as its first argument, returns a
// typed report, and prints nothing (DESIGN-0014 §2.8, rules R4 and R8).
//
// This is L3, the only layer that touches the filesystem as a whole
// repository rather than a file at a time. The layers below it are
// bytes-in/values-out and take no context: there is nothing to cancel in a
// function that neither blocks nor opens anything.
//
// repo never imports a type package (rule R2). The grammar it validates
// against is a grammar over region kinds, not over document type names, so
// a repo's own custom type gets the same treatment a built-in does. The
// per-type interpretation — that an IMPL's phases are numbered, that an
// investigation must answer its question — is the caller's to compose from
// pkg/impl and its siblings, and cmd/validate.go does exactly that.
//
// Cancellation is checked between per-type iterations, not mid-file. A
// cancelled run returns the report completed so far together with
// ctx.Err(), so a caller can report partial progress rather than throwing
// away the types that finished. What was already written stays written.
package repo

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
)

// Repo is a docz repository: a root directory and the configuration that
// applies to it.
//
// Both fields are exported and a struct literal is as valid as Open. A
// consumer that already holds a validated config — docz-api, which loads
// one per request from a cache — should not have to re-read it from disk
// to get the operations.
type Repo struct {
	// Root is the repository root. Every config-relative path resolves
	// under it, so nothing in this package consults the process working
	// directory and a consumer can operate on a checkout somewhere other
	// than its own cwd.
	Root string
	// Cfg is the loaded and validated configuration.
	Cfg *config.Config
}

// Open loads and validates the configuration for the repository at root
// and returns the Repo for it.
//
// configFile is the explicit --config path, or "" to let config.Load find
// .docz.yaml under root and merge it over the global one. The validation
// warnings config.Load surfaces are deliberately dropped here: they are a
// presentation concern, and a consumer that wants them calls config.Load
// itself and builds the struct literal.
//
// The context is accepted rather than used. Loading a config is two file
// reads with no cancellation point worth the complexity of aborting
// halfway, and the parameter is here because R8 says every L3 function
// takes one — adding it later would break every caller.
func Open(_ context.Context, root, configFile string) (*Repo, error) {
	cfg, err := config.Load(configFile, root)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	if _, err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &Repo{Root: root, Cfg: &cfg}, nil
}

// Path resolves a repo-relative path under Root.
//
// An absolute path passes through unchanged, and an empty Root leaves the
// path relative — the same rule as cmd's Runner.inRepo, so a test that
// builds a Repo with absolute paths and no root gets the paths it passed.
func (r *Repo) Path(rel string) string {
	if rel == "" || r.Root == "" || filepath.IsAbs(rel) {
		return rel
	}

	return filepath.Join(r.Root, rel)
}

// TypeDir returns the absolute directory for a canonical type name.
//
// The name is not resolved or validated here: this is a path helper, pure
// and total, and config.TypeDir already falls back to <DocsDir>/<name> for
// a name it does not know. A caller that needs the token resolved calls a
// read or write method, which does it and reports the typed error.
func (r *Repo) TypeDir(typeName string) string {
	return r.Path(r.Cfg.TypeDir(typeName))
}

// ReadmePath returns the index README for a canonical type name.
func (r *Repo) ReadmePath(typeName string) string {
	return filepath.Join(r.TypeDir(typeName), config.IndexFileName)
}

// RelPath returns path relative to Root, or path unchanged when it is not
// under Root.
//
// Exported because every consumer that reports a path wants the short
// form, and computing it means knowing Root — which the consumer would
// otherwise have to thread alongside every report. Failure is not an
// error: a path outside the repository is a legitimate thing to report,
// just not shortenable.
//
// A result that starts with ".." counts as failure even though
// filepath.Rel succeeded. "../../etc/passwd" is a worse thing to show
// somebody than the absolute path it came from: it reads as a path inside
// the repo until you count the segments, and it means nothing at all to a
// reader who does not know what Root was.
func (r *Repo) RelPath(path string) string {
	if r.Root == "" {
		return path
	}

	rel, err := filepath.Rel(r.Root, path)
	if err != nil {
		return path
	}

	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return path
	}

	return rel
}

// typesOrEnabled resolves an explicit type list, or returns every enabled
// type when the list is nil or empty.
//
// The shared front half of List, Update, Validate, and InsertRegions. Nil
// meaning "all enabled" is what keeps a custom type from being silently
// skipped by a no-argument command, since EnabledTypes includes custom
// keys (built-ins first in registry order, then custom sorted).
//
// An explicit token that names a disabled type is an error rather than a
// skip: the user asked for it by name, so saying nothing would be a lie.
// A type only reached by way of "all enabled" cannot be disabled, so the
// two paths cannot disagree.
func (r *Repo) typesOrEnabled(types []string) ([]string, error) {
	if len(types) == 0 {
		return r.Cfg.EnabledTypes(), nil
	}

	resolved := make([]string, 0, len(types))

	for _, token := range types {
		typeName, err := r.resolveType(token)
		if err != nil {
			return nil, err
		}

		resolved = append(resolved, typeName)
	}

	return resolved, nil
}
