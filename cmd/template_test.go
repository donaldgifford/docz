package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/internal/config"
)

func TestRunTemplateShow(t *testing.T) {
	appCfg = config.DefaultConfig()

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runTemplateShow(nil, []string{"rfc"})

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("runTemplateShow() error: %v", err)
	}

	var buf bytes.Buffer
	if _, cpErr := buf.ReadFrom(r); cpErr != nil {
		t.Fatal(cpErr)
	}
	output := buf.String()

	if !strings.Contains(output, "{{ .Title }}") {
		t.Error("template output should contain {{ .Title }} placeholder")
	}
}

func TestRunTemplateShow_InvalidType(t *testing.T) {
	appCfg = config.DefaultConfig()
	err := runTemplateShow(nil, []string{"badtype"})
	if err == nil {
		t.Error("expected error for invalid type, got nil")
	}
}

func TestRunTemplateExport(t *testing.T) {
	dir := t.TempDir()
	appCfg = config.DefaultConfig()

	outPath := filepath.Join(dir, "exported-rfc.md")

	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	err := runTemplateExport(nil, []string{"rfc", outPath})

	w.Close()
	os.Stdout = old

	if err != nil {
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

func TestRunTemplateExport_DefaultPath(t *testing.T) {
	appCfg = config.DefaultConfig()

	// Change to temp dir to avoid polluting the working directory.
	origDir, _ := os.Getwd()
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	err := runTemplateExport(nil, []string{"adr"})

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("runTemplateExport() error: %v", err)
	}

	if _, statErr := os.Stat(filepath.Join(dir, "adr.md")); statErr != nil {
		t.Errorf("expected adr.md to be created: %v", statErr)
	}
}

func TestRunTemplateOverride(t *testing.T) {
	dir := t.TempDir()
	appCfg = config.DefaultConfig()
	appCfg.DocsDir = filepath.Join(dir, "docs")

	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	err := runTemplateOverride(nil, []string{"rfc"})

	w.Close()
	os.Stdout = old

	if err != nil {
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
	appCfg = config.DefaultConfig()
	appCfg.DocsDir = filepath.Join(dir, "docs")

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

func TestValidateType(t *testing.T) {
	appCfg = config.DefaultConfig()

	if err := validateType("rfc"); err != nil {
		t.Errorf("validateType(rfc) error: %v", err)
	}
	if err := validateType("badtype"); err == nil {
		t.Error("validateType(badtype) should return error")
	}
}
