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

// newTemplateTestRunner builds a Runner that captures Out into the
// supplied buffer and resolves cwd-relative paths under dir. Tests
// can mutate `runner.Cfg` to override the supplied cfg after setup.
func newTemplateTestRunner(t *testing.T, dir string, cfg *config.Config, out io.Writer) {
	t.Helper()
	appCfg = *cfg
	runner = &Runner{
		Cfg:      *cfg,
		Out:      out,
		Err:      io.Discard,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now:      time.Now,
		Git:      staticGit{},
		RepoRoot: dir,
	}
	t.Cleanup(func() { runner = nil })
}

func TestRunTemplateShow(t *testing.T) {
	var out bytes.Buffer
	cfg := config.DefaultConfig()
	newTemplateTestRunner(t, t.TempDir(), &cfg, &out)

	if err := runTemplateShow(nil, []string{"rfc"}); err != nil {
		t.Fatalf("runTemplateShow() error: %v", err)
	}

	if !strings.Contains(out.String(), "{{ .Title }}") {
		t.Error("template output should contain {{ .Title }} placeholder")
	}
}

func TestRunTemplateShow_InvalidType(t *testing.T) {
	cfg := config.DefaultConfig()
	newTemplateTestRunner(t, t.TempDir(), &cfg, io.Discard)
	err := runTemplateShow(nil, []string{"badtype"})
	if err == nil {
		t.Error("expected error for invalid type, got nil")
	}
}

func TestRunTemplateExport(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "exported-rfc.md")
	cfg := config.DefaultConfig()
	newTemplateTestRunner(t, dir, &cfg, io.Discard)

	if err := runTemplateExport(nil, []string{"rfc", outPath}); err != nil {
		t.Fatalf("runTemplateExport() error: %v", err)
	}

	content, readErr := os.ReadFile(outPath)
	if readErr != nil {
		t.Fatalf("reading exported file: %v", readErr)
	}

	if !strings.Contains(string(content), "{{ .Title }}") {
		t.Error("exported template should contain {{ .Title }} placeholder")
	}
}

// TestRunTemplateExport_DefaultPath confirms that with no explicit
// output path the handler writes the template alongside the caller's
// repo root (Runner.RepoRoot), not the process cwd — so the test
// doesn't need to os.Chdir.
func TestRunTemplateExport_DefaultPath(t *testing.T) {
	dir := t.TempDir()
	cfg := config.DefaultConfig()
	newTemplateTestRunner(t, dir, &cfg, io.Discard)

	if err := runTemplateExport(nil, []string{"adr"}); err != nil {
		t.Fatalf("runTemplateExport() error: %v", err)
	}

	if _, statErr := os.Stat(filepath.Join(dir, "adr.md")); statErr != nil {
		t.Errorf("expected adr.md to be created under RepoRoot: %v", statErr)
	}
}

func TestRunTemplateOverride(t *testing.T) {
	dir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.DocsDir = filepath.Join(dir, "docs")
	newTemplateTestRunner(t, dir, &cfg, io.Discard)

	if err := runTemplateOverride(nil, []string{"rfc"}); err != nil {
		t.Fatalf("runTemplateOverride() error: %v", err)
	}

	overridePath := filepath.Join(dir, "docs", "templates", "rfc.md")
	content, readErr := os.ReadFile(overridePath)
	if readErr != nil {
		t.Fatalf("reading override file: %v", readErr)
	}

	if !strings.Contains(string(content), "{{ .Title }}") {
		t.Error("override template should contain {{ .Title }} placeholder")
	}
}

func TestRunTemplateOverride_AlreadyExists(t *testing.T) {
	dir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.DocsDir = filepath.Join(dir, "docs")
	newTemplateTestRunner(t, dir, &cfg, io.Discard)

	overrideDir := filepath.Join(dir, "docs", "templates")
	if err := os.MkdirAll(overrideDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(overrideDir, "rfc.md"), []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := runTemplateOverride(nil, []string{"rfc"})
	if err == nil {
		t.Error("expected error when override file already exists, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error should mention 'already exists', got: %v", err)
	}
}
