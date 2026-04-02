package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/internal/config"
	"github.com/donaldgifford/docz/internal/toc"
)

func setupUpdateTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	rfcDir := filepath.Join(dir, "docs", "rfc")
	if err := os.MkdirAll(rfcDir, 0o750); err != nil {
		t.Fatal(err)
	}

	// Write a README with markers.
	readme := `# RFCs

<!-- BEGIN DOCZ AUTO-GENERATED -->
<!-- END DOCZ AUTO-GENERATED -->
`
	if err := os.WriteFile(filepath.Join(rfcDir, "README.md"), []byte(readme), 0o644); err != nil {
		t.Fatal(err)
	}

	return dir
}

func writeDocWithToC(t *testing.T, dir, filename, id, title string) {
	t.Helper()
	content := "---\n" +
		"id: " + id + "\n" +
		"title: \"" + title + "\"\n" +
		"status: Draft\n" +
		"author: Test\n" +
		"created: 2026-01-01\n" +
		"---\n\n" +
		"# " + title + "\n\n" +
		toc.BeginMarker + "\n" +
		toc.EndMarker + "\n\n" +
		"## Summary\n\n" +
		"Text here.\n\n" +
		"## Problem Statement\n\n" +
		"More text.\n\n" +
		"## Design\n\n" +
		"Design details.\n"
	if err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateGeneratesToC(t *testing.T) {
	dir := setupUpdateTestDir(t)
	rfcDir := filepath.Join(dir, "docs", "rfc")
	writeDocWithToC(t, rfcDir, "0001-test.md", "RFC-0001", "Test RFC")

	appCfg = config.DefaultConfig()
	appCfg.DocsDir = filepath.Join(dir, "docs")
	updateDryRun = false
	verbose = false

	if err := updateType("rfc"); err != nil {
		t.Fatalf("updateType() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(rfcDir, "0001-test.md"))
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	if !strings.Contains(content, "- [Summary](#summary)") {
		t.Error("ToC missing Summary entry")
	}
	if !strings.Contains(content, "- [Problem Statement](#problem-statement)") {
		t.Error("ToC missing Problem Statement entry")
	}
	if !strings.Contains(content, "- [Design](#design)") {
		t.Error("ToC missing Design entry")
	}
}

func TestUpdateToCDisabled(t *testing.T) {
	dir := setupUpdateTestDir(t)
	rfcDir := filepath.Join(dir, "docs", "rfc")
	writeDocWithToC(t, rfcDir, "0001-test.md", "RFC-0001", "Test RFC")

	appCfg = config.DefaultConfig()
	appCfg.DocsDir = filepath.Join(dir, "docs")
	appCfg.ToC.Enabled = false
	updateDryRun = false
	verbose = false

	if err := updateType("rfc"); err != nil {
		t.Fatalf("updateType() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(rfcDir, "0001-test.md"))
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	// ToC should remain empty between markers.
	if strings.Contains(content, "- [Summary]") {
		t.Error("ToC should not be generated when disabled")
	}
}

func TestUpdateToCDryRun(t *testing.T) {
	dir := setupUpdateTestDir(t)
	rfcDir := filepath.Join(dir, "docs", "rfc")
	writeDocWithToC(t, rfcDir, "0001-test.md", "RFC-0001", "Test RFC")

	appCfg = config.DefaultConfig()
	appCfg.DocsDir = filepath.Join(dir, "docs")
	updateDryRun = true
	verbose = false
	t.Cleanup(func() { updateDryRun = false })

	// Read original content.
	originalData, err := os.ReadFile(filepath.Join(rfcDir, "0001-test.md"))
	if err != nil {
		t.Fatal(err)
	}

	if err := updateType("rfc"); err != nil {
		t.Fatalf("updateType() error: %v", err)
	}

	// File should not have been modified.
	afterData, err := os.ReadFile(filepath.Join(rfcDir, "0001-test.md"))
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(afterData, originalData) {
		t.Error("dry-run should not modify files")
	}
}

func TestUpdateToCNoMarkers(t *testing.T) {
	dir := setupUpdateTestDir(t)
	rfcDir := filepath.Join(dir, "docs", "rfc")

	// Write a doc without ToC markers.
	writeTestDoc(t, rfcDir, "0001-test.md", "RFC-0001", "Test", "Draft", "Test", "2026-01-01")

	appCfg = config.DefaultConfig()
	appCfg.DocsDir = filepath.Join(dir, "docs")
	updateDryRun = false
	verbose = false

	originalData, err := os.ReadFile(filepath.Join(rfcDir, "0001-test.md"))
	if err != nil {
		t.Fatal(err)
	}

	if err := updateType("rfc"); err != nil {
		t.Fatalf("updateType() error: %v", err)
	}

	afterData, err := os.ReadFile(filepath.Join(rfcDir, "0001-test.md"))
	if err != nil {
		t.Fatal(err)
	}

	// Doc without markers should not have ToC added.
	if strings.Contains(string(afterData), toc.BeginMarker) {
		t.Error("markers should not be added to docs without them")
	}
	// Original content should be unchanged.
	if !bytes.Equal(afterData, originalData) {
		t.Error("doc without markers should not be modified")
	}
}

func TestUpdateSkipsDisabledTypes(t *testing.T) {
	dir := t.TempDir()
	docsDir := filepath.Join(dir, "docs")

	// Only create rfc dir — plan dir should NOT be created by update.
	rfcDir := filepath.Join(docsDir, "rfc")
	if err := os.MkdirAll(rfcDir, 0o750); err != nil {
		t.Fatal(err)
	}

	readme := "# RFCs\n\n<!-- BEGIN DOCZ AUTO-GENERATED -->\n<!-- END DOCZ AUTO-GENERATED -->\n"
	if err := os.WriteFile(filepath.Join(rfcDir, "README.md"), []byte(readme), 0o644); err != nil {
		t.Fatal(err)
	}

	appCfg = config.DefaultConfig()
	appCfg.DocsDir = docsDir

	// Disable plan type.
	tc := appCfg.Types["plan"]
	tc.Enabled = false
	appCfg.Types["plan"] = tc

	updateDryRun = false
	verbose = false

	if err := runUpdate(nil, nil); err != nil {
		t.Fatalf("runUpdate() error: %v", err)
	}

	// Plan directory should NOT have been created.
	planDir := filepath.Join(docsDir, "plan")
	if _, err := os.Stat(planDir); err == nil {
		t.Error("plan directory should not be created for disabled type")
	}

	// RFC README should have been updated (it existed).
	if _, err := os.Stat(filepath.Join(rfcDir, "README.md")); err != nil {
		t.Error("rfc README should still exist")
	}
}

func TestCreateIncludesToCMarkers(t *testing.T) {
	dir := t.TempDir()
	docsDir := filepath.Join(dir, "docs")
	rfcDir := filepath.Join(docsDir, "rfc")
	if err := os.MkdirAll(rfcDir, 0o750); err != nil {
		t.Fatal(err)
	}

	// Write a README with markers for the index update.
	readme := `# RFCs

<!-- BEGIN DOCZ AUTO-GENERATED -->
<!-- END DOCZ AUTO-GENERATED -->
`
	if err := os.WriteFile(filepath.Join(rfcDir, "README.md"), []byte(readme), 0o644); err != nil {
		t.Fatal(err)
	}

	appCfg = config.DefaultConfig()
	appCfg.DocsDir = docsDir

	if err := runCreate(nil, []string{"rfc", "Test ToC Markers"}); err != nil {
		t.Fatalf("runCreate() error: %v", err)
	}

	// Find the created file.
	entries, err := os.ReadDir(rfcDir)
	if err != nil {
		t.Fatal(err)
	}

	var docFile string
	for _, e := range entries {
		if e.Name() != "README.md" && strings.HasSuffix(e.Name(), ".md") {
			docFile = e.Name()
			break
		}
	}
	if docFile == "" {
		t.Fatal("no document file created")
	}

	data, err := os.ReadFile(filepath.Join(rfcDir, docFile))
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	if !strings.Contains(content, toc.BeginMarker) {
		t.Error("created document missing ToC begin marker")
	}
	if !strings.Contains(content, toc.EndMarker) {
		t.Error("created document missing ToC end marker")
	}
}
