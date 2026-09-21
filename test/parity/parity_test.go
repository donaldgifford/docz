//go:build parity

// The parity driver. Behind a build tag because it needs a built binary,
// named by DOCZ_PARITY_BIN, and because `go test ./...` should stay a
// source-only run.
//
//	make parity                  # build build/bin/docz, replay the goldens
//	make parity BIN=/path/to/bin # replay against another binary
//	make parity-capture          # capture from v1.2.2 (the only source)
package parity

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var update = flag.Bool(
	"update",
	false,
	"capture goldens from DOCZ_PARITY_BIN instead of comparing against them",
)

// binEnv names the binary under test. There is no default on purpose: a
// golden captured from whatever happened to be on PATH would be worthless as
// evidence, so the caller says which binary every time.
const binEnv = "DOCZ_PARITY_BIN"

// A caseSpec is one recorded invocation.
type caseSpec struct {
	name string
	args []string
	// setup runs before the recorded command and is not itself recorded. It
	// exists for cases that need a precondition the fixture does not carry,
	// such as `wiki update` needing an mkdocs.yml.
	setup [][]string
	// emptyDir runs the case in an empty directory instead of a fixture copy,
	// which is the only way to record `init` scaffolding from nothing.
	emptyDir bool
}

// A fixtureSpec describes one fixture repository and the type tokens its
// cases use. Everything else about the fixture lives on disk.
type fixtureSpec struct {
	name string

	// docType is the fixture's primary type, altType a second enabled type
	// (empty when the fixture has one). Both are spelled as a user would.
	docType string
	altType string

	// docID is a document that exists in the fixture, current its status, and
	// next a different valid status for that type.
	docID   string
	current string
	next    string

	// prefix is the type's id_prefix, used to build an id that cannot exist.
	prefix string

	// extra are cases specific to this fixture.
	extra []caseSpec
}

func fixtures() []fixtureSpec {
	return []fixtureSpec{
		{
			name:    "rfc",
			docType: "rfc",
			docID:   "RFC-0001",
			current: "Draft",
			next:    "Accepted",
			prefix:  "RFC",
		},
		{
			name:    "adr",
			docType: "adr",
			docID:   "ADR-0001",
			current: "Accepted",
			next:    "Superseded",
			prefix:  "ADR",
		},
		{
			name:    "design",
			docType: "design",
			docID:   "DESIGN-0001",
			current: "Approved",
			next:    "Implemented",
			prefix:  "DESIGN",
		},
		{
			name:    "impl",
			docType: "impl",
			docID:   "IMPL-0001",
			current: "In Progress",
			next:    "Completed",
			prefix:  "IMPL",
			extra: []caseSpec{
				// The registry alias, which resolves to the same type.
				{name: "list-alias", args: []string{"list", "implementation"}},
			},
		},
		{
			name:    "investigation",
			docType: "investigation",
			docID:   "INV-0001",
			current: "Concluded",
			next:    "Abandoned",
			prefix:  "INV",
			extra: []caseSpec{
				{name: "list-alias", args: []string{"list", "inv"}},
			},
		},
		{
			name:    "custom",
			docType: "frameworks",
			altType: "adr",
			docID:   "FW-0001",
			current: "Active",
			next:    "Deprecated",
			prefix:  "FW",
			extra: []caseSpec{
				// A custom type resolves by name, by alias, and by id_prefix
				// (IMPL-0012). All three are user-visible behaviour.
				{name: "create-alias", args: []string{"create", "fw", "Parity alias create"}},
				{name: "create-prefix", args: []string{"create", "FW", "Parity prefix create"}},
				{name: "list-alias", args: []string{"list", "fw"}},
				{name: "list-prefix", args: []string{"list", "FW"}},
				{name: "template-show-custom", args: []string{"template", "show", "frameworks"}},
				{name: "template-export-custom", args: []string{"template", "export", "frameworks"}},
				// The fixture already has docs/templates/frameworks.md, so this
				// records the refuse-to-clobber path.
				{
					name: "template-override-custom",
					args: []string{"template", "override", "frameworks"},
				},
			},
		},
		{
			name:    "legacy",
			docType: "rfc",
			altType: "adr",
			docID:   "RFC-0001",
			current: "Accepted",
			next:    "Superseded",
			prefix:  "RFC",
			// No plan cases. The fixture's plan: block is dormant, which is the
			// point of the fixture: the block must keep loading and keep showing
			// up in `docz config`. `create plan` and `template show|export plan`
			// are deliberately not recorded, because ADR-0003 removes the
			// built-in on the v2 line and those three would be the only cases
			// whose change is expected rather than a regression.
		},
	}
}

// cases builds the invocation table for one fixture.
func (f fixtureSpec) cases() []caseSpec {
	t := f.docType

	cs := []caseSpec{
		{name: "init-empty", args: []string{"init"}, emptyDir: true},
		{name: "init", args: []string{"init"}},
		{name: "init-force", args: []string{"init", "--force"}},

		{name: "create", args: []string{"create", t, "Parity created document"}},
		{name: "create-flags", args: []string{
			"create", t, "Parity flagged document",
			"--author", "Pinned Author", "--status", f.next, "--no-update",
		}},

		{name: "update", args: []string{"update"}},
		{name: "update-type", args: []string{"update", t}},
		{name: "update-dry-run", args: []string{"update", "--dry-run"}},
		{name: "update-type-dry-run", args: []string{"update", t, "--dry-run"}},

		{name: "list", args: []string{"list"}},
		{name: "list-type", args: []string{"list", t}},
		{name: "list-json", args: []string{"list", "--format", "json"}},
		{name: "list-csv", args: []string{"list", "--format", "csv"}},
		{name: "list-status", args: []string{"list", "--status", f.current}},

		{name: "status-set", args: []string{"status", "set", t, f.docID, f.next}},
		{name: "status-set-unchanged", args: []string{"status", "set", t, f.docID, f.current}},
		{name: "status-set-dry-run", args: []string{
			"status", "set", t, f.docID, f.next, "--dry-run",
		}},
		{name: "status-set-json", args: []string{
			"status", "set", t, f.docID, f.next, "--format", "json",
		}},
		{name: "status-set-missing-id", args: []string{
			"status", "set", t, f.prefix + "-9999", f.next,
		}},
		{name: "status-set-unknown-type", args: []string{
			"status", "set", "nosuchtype", f.docID, f.next,
		}},
		{name: "status-set-invalid-status", args: []string{
			"status", "set", t, f.docID, "Nonsense",
		}},

		{name: "template-show", args: []string{"template", "show", t}},
		{name: "template-export", args: []string{"template", "export", t}},
		{name: "template-override", args: []string{"template", "override", t}},

		{name: "config", args: []string{"config"}},

		{name: "wiki-init", args: []string{"wiki", "init"}},
		{name: "wiki-update", args: []string{"wiki", "update"}, setup: [][]string{
			{"wiki", "init"},
		}},
		{name: "wiki-update-dry-run", args: []string{"wiki", "update", "--dry-run"},
			setup: [][]string{{"wiki", "init"}}},
	}

	if f.altType != "" {
		cs = append(cs,
			caseSpec{name: "create-alt", args: []string{
				"create", f.altType, "Parity second type",
			}},
			caseSpec{name: "list-alt", args: []string{"list", f.altType}},
			caseSpec{name: "update-alt", args: []string{"update", f.altType}},
			caseSpec{name: "template-show-alt", args: []string{
				"template", "show", f.altType,
			}},
		)
	}

	return append(cs, f.extra...)
}

func TestParity(t *testing.T) {
	bin := os.Getenv(binEnv)
	if bin == "" {
		t.Fatalf("%s is unset: name the docz binary to drive", binEnv)
	}

	abs, err := filepath.Abs(bin)
	if err != nil {
		t.Fatalf("resolve %s: %v", bin, err)
	}

	if _, err := os.Stat(abs); err != nil {
		t.Fatalf("binary %s: %v", abs, err)
	}

	today := time.Now().Format("2006-01-02")

	for _, f := range fixtures() {
		t.Run(f.name, func(t *testing.T) {
			for _, c := range f.cases() {
				t.Run(c.name, func(t *testing.T) {
					runCase(t, abs, f, c, today)
				})
			}
		})
	}
}

func runCase(t *testing.T, bin string, f fixtureSpec, c caseSpec, today string) {
	t.Helper()

	root := t.TempDir()

	if !c.emptyDir {
		src := filepath.Join("fixtures", f.name)
		if err := copyTree(src, root); err != nil {
			t.Fatalf("copy fixture %s: %v", src, err)
		}
	}

	for _, args := range c.setup {
		if _, _, _, err := run(t.Context(), bin, root, args); err != nil {
			t.Fatalf("setup %v: %v", args, err)
		}
	}

	before, err := Tree(root)
	if err != nil {
		t.Fatalf("snapshot before: %v", err)
	}

	stdout, stderr, code, err := run(t.Context(), bin, root, c.args)
	if err != nil {
		t.Fatalf("run %v: %v", c.args, err)
	}

	after, err := Tree(root)
	if err != nil {
		t.Fatalf("snapshot after: %v", err)
	}

	norms := []Normalizer{RootNormalizer(root), DateNormalizer(today), MarkerNormalizer()}

	// Normalised on both sides before the comparison, so a size and digest
	// recorded beside a body describe the body the golden shows.
	before = NormalizeFiles(before, norms...)
	after = NormalizeFiles(after, norms...)

	result := Result{
		Args:     c.args,
		ExitCode: code,
		Stdout:   Normalize(stdout, norms...),
		Stderr:   Normalize(stderr, norms...),
		Files:    Changed(before, after),
	}

	got := Format(&result, Deleted(before, after))
	path := filepath.Join("testdata", f.name, c.name+".golden")

	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir golden dir: %v", err)
		}

		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}

		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden (run `make parity-capture` first): %v", err)
	}

	if got != string(want) {
		t.Errorf("parity mismatch for %s/%s\n%s", f.name, c.name, firstDiff(string(want), got))
	}
}

// caseTimeout bounds one invocation. Without it a binary that hangs blocks
// until the package-level `go test` deadline, which then panics the whole run
// without naming the case that wedged it. With 200-odd cases that deadline is
// a budget, not a safety net.
const caseTimeout = 30 * time.Second

// run executes the binary in dir and returns its output and exit code. A
// non-zero exit is data, not an error; the returned error is for a failure to
// run the binary at all.
func run(
	ctx context.Context,
	bin, dir string,
	args []string,
) (stdout, stderr string, code int, err error) {
	ctx, cancel := context.WithTimeout(ctx, caseTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir

	// A child holding the output pipes open after the deadline must not wedge
	// Wait as well.
	cmd.WaitDelay = time.Second

	var out, errb bytes.Buffer

	cmd.Stdout = &out
	cmd.Stderr = &errb

	// The environment is an allowlist, not os.Environ() plus overrides. The
	// suite's whole value is that a golden means the same thing on every
	// machine, and docz reads a global ~/.docz.yaml and deep-merges it under
	// the repo's (config.Load). Inheriting HOME would merge whoever ran the
	// capture into all 213 cases and commit their author name, paths, and
	// custom types into testdata. HOME points at the fixture copy, where no
	// global config exists.
	//
	// TZ is pinned because the parent computes today's date for the $DATE
	// normaliser while the child stamps its own; a run spanning midnight in a
	// different zone would otherwise miss.
	//
	// Anything CI puts in the environment, GITHUB_TOKEN included, is simply
	// not handed to the binary under test.
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + dir,
		"USERPROFILE=" + dir,
		"TZ=UTC",
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
		"NO_COLOR=1",
	}

	runErr := cmd.Run()

	var exitErr *exec.ExitError
	switch {
	case runErr == nil:
		code = 0
	case errors.As(runErr, &exitErr):
		code = exitErr.ExitCode()
	default:
		return "", "", 0, runErr
	}

	return out.String(), errb.String(), code, nil
}

// copyTree copies src's contents into dst, which must exist.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		target := filepath.Join(dst, rel)

		if d.IsDir() {
			if rel == "." {
				return nil
			}

			return os.MkdirAll(target, 0o755)
		}

		// Refuse anything that is not a regular file rather than skipping it.
		// os.ReadFile follows a symlink, so a fixture entry pointing at
		// ~/.ssh/id_rsa would be copied in as a regular file, read back by
		// Tree, and embedded verbatim in a committed golden; a FIFO with no
		// writer would block the read outright. Symlink mode is invisible in a
		// diff, so this has to fail loudly instead of relying on review.
		if !d.Type().IsRegular() {
			return fmt.Errorf("fixture %s: not a regular file (%s)", path, d.Type())
		}

		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}

		return os.WriteFile(target, body, 0o644)
	})
}

// firstDiff reports the first differing line with a little context, which is
// what a reviewer needs; the whole golden is on disk for the rest.
func firstDiff(want, got string) string {
	wl := strings.Split(want, "\n")
	gl := strings.Split(got, "\n")

	for i := 0; i < len(wl) || i < len(gl); i++ {
		w, g := "", ""
		if i < len(wl) {
			w = wl[i]
		}

		if i < len(gl) {
			g = gl[i]
		}

		if w != g {
			return fmt.Sprintf("first difference at line %d:\n  want: %q\n  got:  %q", i+1, w, g)
		}
	}

	return "no line differs (trailing bytes only)"
}
