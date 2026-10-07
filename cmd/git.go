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
	// RemoteURL returns the origin remote as https://github.com/<o>/<r>,
	// from either the SSH or the HTTPS spelling, or "" when there is no
	// origin or it is not GitHub-shaped (IMPL-0023 Open Question 8).
	RemoteURL(ctx context.Context) string
	// DefaultBranch returns origin's default branch, or "main" when it
	// cannot be read.
	DefaultBranch(ctx context.Context) string
}

// realGit implements GitResolver by shelling out to git. Dir is the
// repository the remote lookups run in; empty means the process cwd.
type realGit struct {
	Dir string
}

// UserName runs `git config user.name` under the supplied context and
// returns the trimmed output, or "" on any error.
func (realGit) UserName(ctx context.Context) string {
	out, err := exec.CommandContext(ctx, "git", "config", "user.name").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// RemoteURL runs `git remote get-url origin` and normalises the result.
func (g realGit) RemoteURL(ctx context.Context) string {
	return githubURL(g.output(ctx, "remote", "get-url", "origin"))
}

// DefaultBranch reads origin's HEAD symref, falling back to "main".
func (g realGit) DefaultBranch(ctx context.Context) string {
	ref := g.output(ctx, "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
	if branch, ok := strings.CutPrefix(ref, "origin/"); ok && branch != "" {
		return branch
	}

	return defaultBranch
}

// output runs git in g.Dir and returns trimmed stdout, or "" on any error.
func (g realGit) output(ctx context.Context, args ...string) string {
	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec // fixed git subcommands from this file

	cmd.Dir = g.Dir

	out, err := cmd.Output()
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(out))
}

// defaultBranch is the branch assumed when origin's HEAD cannot be read.
const defaultBranch = "main"

// githubPrefix starts every remote githubURL returns.
const githubPrefix = "https://github.com/"

// githubURL normalises a GitHub remote to https://github.com/<o>/<r>:
// git@github.com:o/r.git, ssh://git@github.com/o/r, and
// https://github.com/o/r.git all give the same answer. Anything else is "".
func githubURL(remote string) string {
	rest := ""

	for _, prefix := range []string{"git@github.com:", "ssh://git@github.com/", "https://github.com/", "http://github.com/"} {
		if r, ok := strings.CutPrefix(remote, prefix); ok {
			rest = r

			break
		}
	}

	rest = strings.TrimSuffix(strings.TrimSuffix(rest, "/"), ".git")

	owner, name, ok := strings.Cut(rest, "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return ""
	}

	return githubPrefix + owner + "/" + name
}

// staticGit is the test double for GitResolver. Tests set Name to the
// desired user.name value and pass it in via Runner.Git.
type staticGit struct {
	Name   string
	Remote string
	Branch string
}

// UserName returns the configured static name, ignoring ctx.
func (s staticGit) UserName(_ context.Context) string {
	return s.Name
}

// RemoteURL returns the configured static remote.
func (s staticGit) RemoteURL(_ context.Context) string {
	return s.Remote
}

// DefaultBranch returns the configured static branch, or "main".
func (s staticGit) DefaultBranch(_ context.Context) string {
	if s.Branch == "" {
		return defaultBranch
	}

	return s.Branch
}
