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

	"github.com/donaldgifford/docz/internal/config"
)

// newCustomTypeRunner builds a Runner rooted at dir whose config enables a
// custom "frameworks" type (id_prefix FW). It exercises IMPL-0012's
// custom-type index-header resolution end-to-end through updateType. It
// reuses staticGit and readDoc from status_test.go (same package).
func newCustomTypeRunner(t *testing.T, dir string, out io.Writer) *Runner {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.DocsDir = filepath.Join(dir, "docs")
	cfg.Types["frameworks"] = config.TypeConfig{
		Enabled:     true,
		Dir:         "frameworks",
		IDPrefix:    "FW",
		IDWidth:     4,
		Statuses:    []string{"Draft", "Active"},
		StatusField: "status",
		PluralLabel: "Frameworks",
	}
	return &Runner{
		Cfg:      cfg,
		Out:      out,
		Err:      io.Discard,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now:      time.Now,
		Git:      staticGit{},
		RepoRoot: dir,
	}
}

// TestUpdateType_CustomTypeGeneratedHeader is the IMPL-0012 Phase 2
// regression test: a custom type with no embedded and no on-disk header gets
// a generated fallback header instead of failing "no embedded index header
// for type". This is the original reported bug.
func TestUpdateType_CustomTypeGeneratedHeader(t *testing.T) {
	dir := t.TempDir()
	fwDir := filepath.Join(dir, "docs", "frameworks")
	if err := os.MkdirAll(fwDir, config.DirMode); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	r := newCustomTypeRunner(t, dir, &out)

	if err := r.updateType("frameworks", false); err != nil {
		t.Fatalf("updateType(frameworks) error: %v", err)
	}

	content := readDoc(t, filepath.Join(fwDir, "README.md"))
	for _, want := range []string{
		"# Frameworks",
		"docz create frameworks",
		"<!-- BEGIN DOCZ AUTO-GENERATED -->",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("generated README missing %q:\n%s", want, content)
		}
	}
}

// TestUpdateType_CustomTypeOverrideWins proves a
// docs/templates/index_<type>.md override is used verbatim instead of the
// generated fallback (tier 1 beats tier 3).
func TestUpdateType_CustomTypeOverrideWins(t *testing.T) {
	dir := t.TempDir()
	fwDir := filepath.Join(dir, "docs", "frameworks")
	if err := os.MkdirAll(fwDir, config.DirMode); err != nil {
		t.Fatal(err)
	}
	tmplDir := filepath.Join(dir, "docs", config.TemplatesDir)
	if err := os.MkdirAll(tmplDir, config.DirMode); err != nil {
		t.Fatal(err)
	}
	override := "# Bespoke Frameworks Index\n\nHand-written.\n\n"
	if err := os.WriteFile(filepath.Join(tmplDir, "index_frameworks.md"), []byte(override), config.FileMode); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	r := newCustomTypeRunner(t, dir, &out)

	if err := r.updateType("frameworks", false); err != nil {
		t.Fatalf("updateType(frameworks) error: %v", err)
	}

	content := readDoc(t, filepath.Join(fwDir, "README.md"))
	if !strings.HasPrefix(content, override) {
		t.Errorf("override header not used verbatim:\n%s", content)
	}
	if strings.Contains(content, "docz create frameworks") {
		t.Error("generated fallback leaked despite override present")
	}
}
