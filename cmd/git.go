// Package cmd implements the docz CLI commands.
package cmd

import (
	"context"
	"os/exec"
	"strings"
)

// GitResolver looks up git-derived author identity. The interface
// exists so tests can substitute a staticGit value instead of shelling
// out to `git`. See docs/design/0004-runner-pattern-and-doctype-registry.md
// §A and §H; IMPL-0009 Phase 6 wires the realGit/staticGit fixtures
// into the create handler.
type GitResolver interface {
	// UserName returns the configured git user.name, or "" if it cannot
	// be resolved (git missing, no global config, lookup cancelled).
	// The context allows the caller to bound the lookup so Ctrl+C
	// during `docz create` cancels the shellout.
	UserName(ctx context.Context) string
}

// realGit implements GitResolver by shelling out to `git config user.name`.
type realGit struct{}

// UserName runs `git config user.name` under the supplied context and
// returns the trimmed output, or "" on any error.
func (realGit) UserName(ctx context.Context) string {
	out, err := exec.CommandContext(ctx, "git", "config", "user.name").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// staticGit is the test double for GitResolver. Tests set Name to the
// desired user.name value and pass it in via Runner.Git.
type staticGit struct {
	Name string
}

// UserName returns the configured static name, ignoring ctx.
func (s staticGit) UserName(_ context.Context) string {
	return s.Name
}
