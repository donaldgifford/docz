package cmd

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
)

// The runbook type through the CLI (IMPL-0022 Phase 5). A file of its own
// because the existing cmd/*_test.go files are frozen (ADR-0001 Decision 7).

// rbRunner builds a Runner over a temp repo whose only enabled type is
// runbook when enabled is set, and with the default config otherwise.
func rbRunner(t *testing.T, enabled bool) (*Runner, *bytes.Buffer, string) {
	t.Helper()

	root := t.TempDir()

	cfg := config.DefaultConfig()
	cfg.DocsDir = filepath.Join(root, "docs")

	if enabled {
		for name := range cfg.Types {
			tc := cfg.Types[name]
			tc.Enabled = name == "runbook"
			cfg.Types[name] = tc
		}
	}

	var out bytes.Buffer

	return &Runner{
		Cfg:      cfg,
		Out:      &out,
		Err:      io.Discard,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now:      time.Now,
		Git:      staticGit{},
		RepoRoot: root,
	}, &out, root
}

func TestRunbook_CreateOnTheDefaultConfigNamesTheKey(t *testing.T) {
	r, _, _ := rbRunner(t, false)

	err := r.Create(t.Context(), createOpts{}, []string{"rb", "First"})
	if err == nil {
		t.Fatal("create on a disabled type succeeded")
	}

	if !strings.Contains(err.Error(), "set types.runbook.enabled: true in .docz.yaml") {
		t.Errorf("error = %q, want it to name types.runbook.enabled", err)
	}

	if code := exitCodeFor(err); code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestRunbook_CreateWritesTheDocumentAndIndex(t *testing.T) {
	r, _, root := rbRunner(t, true)

	for _, token := range []string{"runbook", "rb", "RUNBOOK"} {
		if err := r.Create(t.Context(), createOpts{}, []string{token, "Via " + token}); err != nil {
			t.Fatalf("create %s: %v", token, err)
		}
	}

	dir := filepath.Join(root, "docs", "runbook")

	for _, name := range []string{"0001-via-runbook.md", "0002-via-rb.md", "0003-via-runbook.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}

	readme, err := os.ReadFile(filepath.Join(dir, config.IndexFileName))
	if err != nil {
		t.Fatalf("reading the index: %v", err)
	}

	for _, id := range []string{"RUNBOOK-0001", "RUNBOOK-0002", "RUNBOOK-0003"} {
		if !bytes.Contains(readme, []byte(id)) {
			t.Errorf("the index does not list %s:\n%s", id, readme)
		}
	}
}

func TestRunbook_ValidateRunsTheTypedTier(t *testing.T) {
	r, out, root := rbRunner(t, true)

	if err := r.Create(t.Context(), createOpts{}, []string{"runbook", "Broken"}); err != nil {
		t.Fatalf("create: %v", err)
	}

	path := filepath.Join(root, "docs", "runbook", "0001-broken.md")

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	// A filled Last Verified row with a commit that is not a SHA is a finding
	// only pkg/runbook makes.
	broken := strings.Replace(string(body),
		"|      |    |        |             |",
		"| 2026-09-25 | #1 | main | @a |", 1)
	vtestWrite(t, path, broken)

	out.Reset()

	// The error is the failing exit; what matters is that the code is there.
	_ = r.validate(t.Context(), validateOpts{format: formatText}, nil)

	if !vtestCodes(out.String())["runbook.last-verified.bad-commit"] {
		t.Errorf("want runbook.last-verified.bad-commit from pkg/runbook:\n%s", out.String())
	}
}

func TestRunbook_ListListsIt(t *testing.T) {
	r, out, _ := rbRunner(t, true)

	if err := r.Create(t.Context(), createOpts{}, []string{"runbook", "Listed"}); err != nil {
		t.Fatalf("create: %v", err)
	}

	out.Reset()

	if err := r.List(t.Context(), listOpts{format: "table"}, []string{"rb"}); err != nil {
		t.Fatalf("list: %v", err)
	}

	if !strings.Contains(out.String(), "RUNBOOK-0001") {
		t.Errorf("list does not show RUNBOOK-0001:\n%s", out.String())
	}
}
