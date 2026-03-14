package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/internal/config"
)

func setupWikiTestDir(t *testing.T) string {
	t.Helper()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	appCfg = config.DefaultConfig()
	return dir
}

func TestWikiInit_EmptyDirectory(t *testing.T) {
	_ = setupWikiTestDir(t)

	// Capture stdout.
	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	err := runWikiInit(nil, nil)

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("runWikiInit() error: %v", err)
	}

	// Should have auto-run docz init → .docz.yaml exists.
	if _, err := os.Stat(".docz.yaml"); err != nil {
		t.Error("expected .docz.yaml to exist after wiki init")
	}

	// mkdocs.yml should exist.
	if _, err := os.Stat("mkdocs.yml"); err != nil {
		t.Error("expected mkdocs.yml to exist")
	}

	// docs/index.md should exist.
	if _, err := os.Stat(filepath.Join("docs", "index.md")); err != nil {
		t.Error("expected docs/index.md to exist")
	}

	// Check mkdocs.yml content.
	data, err := os.ReadFile("mkdocs.yml")
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "techdocs-core") {
		t.Error("mkdocs.yml should contain techdocs-core plugin")
	}
}

func TestWikiInit_AlreadyInitialized(t *testing.T) {
	dir := setupWikiTestDir(t)

	// Pre-create docs structure.
	docsDir := filepath.Join(dir, "docs")
	if err := os.MkdirAll(docsDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(".docz.yaml", []byte("docs_dir: docs\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	err := runWikiInit(nil, nil)

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("runWikiInit() error: %v", err)
	}

	if _, err := os.Stat("mkdocs.yml"); err != nil {
		t.Error("expected mkdocs.yml to exist")
	}
}

func TestWikiInit_SiteName(t *testing.T) {
	_ = setupWikiTestDir(t)
	wikiSiteName = "My Service"
	t.Cleanup(func() { wikiSiteName = "" })

	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	err := runWikiInit(nil, nil)

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("runWikiInit() error: %v", err)
	}

	data, err := os.ReadFile("mkdocs.yml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "site_name: My Service") {
		t.Error("mkdocs.yml should contain site_name: My Service")
	}
}

func TestWikiInit_SiteDescription(t *testing.T) {
	_ = setupWikiTestDir(t)
	wikiSiteDescription = "Custom description"
	t.Cleanup(func() { wikiSiteDescription = "" })

	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	err := runWikiInit(nil, nil)

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("runWikiInit() error: %v", err)
	}

	data, err := os.ReadFile("mkdocs.yml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "site_description: Custom description") {
		t.Error("expected custom site_description in mkdocs.yml")
	}
}

func TestWikiInit_FailsIfExists(t *testing.T) {
	_ = setupWikiTestDir(t)

	// Create mkdocs.yml first.
	if err := os.WriteFile("mkdocs.yml", []byte("site_name: test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Also create .docz.yaml and docs/ to skip auto-init.
	if err := os.WriteFile(".docz.yaml", []byte("docs_dir: docs\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll("docs", 0o750); err != nil {
		t.Fatal(err)
	}

	wikiForce = false
	err := runWikiInit(nil, nil)
	if err == nil {
		t.Error("expected error when mkdocs.yml already exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want 'already exists'", err.Error())
	}
}

func TestWikiInit_ForceOverwrites(t *testing.T) {
	_ = setupWikiTestDir(t)

	// Create existing mkdocs.yml.
	if err := os.WriteFile("mkdocs.yml", []byte("site_name: old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(".docz.yaml", []byte("docs_dir: docs\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll("docs", 0o750); err != nil {
		t.Fatal(err)
	}

	wikiForce = true
	wikiSiteName = "New Name"
	t.Cleanup(func() {
		wikiForce = false
		wikiSiteName = ""
	})

	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	err := runWikiInit(nil, nil)

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("runWikiInit() error: %v", err)
	}

	data, err := os.ReadFile("mkdocs.yml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "New Name") {
		t.Error("mkdocs.yml should have been overwritten with new name")
	}
}

func TestWikiUpdate_BasicNav(t *testing.T) {
	dir := setupWikiTestDir(t)

	// Create docs structure.
	rfcDir := filepath.Join(dir, "docs", "rfc")
	if err := os.MkdirAll(rfcDir, 0o750); err != nil {
		t.Fatal(err)
	}
	writeTestFile(
		t, filepath.Join(dir, "docs", "index.md"),
		"# Home\n",
	)
	writeTestFile(
		t, filepath.Join(rfcDir, "README.md"),
		"# RFCs\n",
	)
	writeTestFile(
		t,
		filepath.Join(rfcDir, "0001-test.md"),
		"---\nid: RFC-0001\ntitle: \"Test RFC\"\nstatus: Draft\nauthor: Test\ncreated: 2026-01-01\n---\n",
	)

	// Create mkdocs.yml.
	writeTestFile(
		t, filepath.Join(dir, "mkdocs.yml"),
		"site_name: test\nplugins:\n    - techdocs-core\nnav:\n    - Home: index.md\n",
	)

	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	err := runWikiUpdate(nil, nil)

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("runWikiUpdate() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "mkdocs.yml"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	if !strings.Contains(content, "site_name: test") {
		t.Error("site_name should be preserved")
	}
	if !strings.Contains(content, "techdocs-core") {
		t.Error("plugins should be preserved")
	}
}

func TestWikiUpdate_MissingMkDocs(t *testing.T) {
	_ = setupWikiTestDir(t)

	err := runWikiUpdate(nil, nil)
	if err == nil {
		t.Error("expected error when mkdocs.yml doesn't exist")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want 'not found'", err.Error())
	}
}

func TestWikiUpdate_DryRun(t *testing.T) {
	dir := setupWikiTestDir(t)

	// Create minimal docs.
	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o750); err != nil {
		t.Fatal(err)
	}
	writeTestFile(
		t, filepath.Join(dir, "docs", "index.md"),
		"# Home\n",
	)
	mkdocsContent := "site_name: test\nnav:\n    - Home: index.md\n"
	writeTestFile(
		t, filepath.Join(dir, "mkdocs.yml"),
		mkdocsContent,
	)

	wikiDryRun = true
	t.Cleanup(func() { wikiDryRun = false })

	// Capture stdout to verify output.
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w

	err := runWikiUpdate(nil, nil)

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("runWikiUpdate() error: %v", err)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if !strings.Contains(output, "Home") {
		t.Error("dry-run output should contain 'Home'")
	}

	// Verify mkdocs.yml was NOT modified.
	data, err := os.ReadFile(filepath.Join(dir, "mkdocs.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != mkdocsContent {
		t.Error("mkdocs.yml should not have been modified during dry-run")
	}
}

func TestWikiUpdate_PreservesExistingOrder(t *testing.T) {
	dir := setupWikiTestDir(t)

	// Create docs with multiple types.
	for _, d := range []string{"rfc", "adr", "design"} {
		typeDir := filepath.Join(dir, "docs", d)
		if err := os.MkdirAll(typeDir, 0o750); err != nil {
			t.Fatal(err)
		}
		writeTestFile(
			t, filepath.Join(typeDir, "README.md"),
			"# "+d+"\n",
		)
	}
	writeTestFile(
		t, filepath.Join(dir, "docs", "index.md"),
		"# Home\n",
	)

	// Create mkdocs.yml with custom order: ADRs before RFCs.
	writeTestFile(
		t,
		filepath.Join(dir, "mkdocs.yml"),
		"site_name: test\nnav:\n    - Home: index.md\n    - ADRs:\n        - Overview: adr/README.md\n    - RFCs:\n        - Overview: rfc/README.md\n",
	)

	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	err := runWikiUpdate(nil, nil)

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("runWikiUpdate() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "mkdocs.yml"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	// ADRs should appear before RFCs (preserved order).
	adrsIdx := strings.Index(content, "ADRs")
	rfcsIdx := strings.Index(content, "RFCs")
	if adrsIdx > rfcsIdx {
		t.Error("ADRs should appear before RFCs (preserved order)")
	}
}

func TestCreateAutoUpdatesWikiNav(t *testing.T) {
	dir := setupWikiTestDir(t)

	// Set up docs structure with mkdocs.yml.
	rfcDir := filepath.Join(dir, "docs", "rfc")
	if err := os.MkdirAll(rfcDir, 0o750); err != nil {
		t.Fatal(err)
	}
	writeTestFile(
		t, filepath.Join(dir, "docs", "index.md"),
		"# Home\n",
	)
	writeTestFile(
		t, filepath.Join(rfcDir, "README.md"),
		"# RFCs\n\n<!-- BEGIN DOCZ AUTO-GENERATED -->\n<!-- END DOCZ AUTO-GENERATED -->\n",
	)
	writeTestFile(
		t, filepath.Join(dir, "mkdocs.yml"),
		"site_name: test\nnav:\n    - Home: index.md\n",
	)

	// Ensure wiki auto-update is enabled.
	appCfg.Wiki.AutoUpdate = true

	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	err := runCreate(nil, []string{"rfc", "Test RFC"})

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("runCreate() error: %v", err)
	}

	// Check that mkdocs.yml was updated with the new RFC.
	data, err := os.ReadFile(filepath.Join(dir, "mkdocs.yml"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "RFC") {
		t.Error("mkdocs.yml nav should contain RFC section after create")
	}
}

func TestCreateNoWikiUpdateWhenMissing(t *testing.T) {
	dir := setupWikiTestDir(t)

	// Set up docs structure WITHOUT mkdocs.yml.
	rfcDir := filepath.Join(dir, "docs", "rfc")
	if err := os.MkdirAll(rfcDir, 0o750); err != nil {
		t.Fatal(err)
	}
	writeTestFile(
		t, filepath.Join(rfcDir, "README.md"),
		"# RFCs\n\n<!-- BEGIN DOCZ AUTO-GENERATED -->\n<!-- END DOCZ AUTO-GENERATED -->\n",
	)

	appCfg.Wiki.AutoUpdate = true

	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	err := runCreate(nil, []string{"rfc", "Test RFC"})

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("runCreate() should succeed without mkdocs.yml: %v", err)
	}

	// mkdocs.yml should NOT have been created.
	if _, err := os.Stat(filepath.Join(dir, "mkdocs.yml")); err == nil {
		t.Error("mkdocs.yml should not be created by docz create")
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
