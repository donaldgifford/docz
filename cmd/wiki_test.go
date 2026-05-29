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

// setupWikiTestDir builds an isolated repo root in t.TempDir() and
// installs a fresh package-level Runner that writes its output to
// io.Discard and resolves cwd-relative paths under that temp dir
// (via Runner.RepoRoot). Tests do not need to os.Chdir, capture
// stdout via os.Pipe, or share state with neighbor tests — all paths
// they assert on live under the returned dir.
func setupWikiTestDir(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.DocsDir = filepath.Join(dir, "docs")
	cfg.Wiki.MkDocsPath = filepath.Join(dir, "mkdocs.yml")

	appCfg = cfg
	runner = &Runner{
		Cfg:      cfg,
		Out:      io.Discard,
		Err:      io.Discard,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now:      time.Now,
		Git:      staticGit{},
		RepoRoot: dir,
	}
	t.Cleanup(func() { runner = nil })
	return dir
}

func TestWikiInit_EmptyDirectory(t *testing.T) {
	dir := setupWikiTestDir(t)

	// Capture stdout.
	err := runWikiInit(nil, nil)
	if err != nil {
		t.Fatalf("runWikiInit() error: %v", err)
	}

	// Should have auto-run docz init → .docz.yaml exists.
	if _, err := os.Stat(filepath.Join(dir, ".docz.yaml")); err != nil {
		t.Error("expected .docz.yaml to exist after wiki init")
	}

	// mkdocs.yml should exist.
	if _, err := os.Stat(filepath.Join(dir, "mkdocs.yml")); err != nil {
		t.Error("expected mkdocs.yml to exist")
	}

	// docs/index.md should exist.
	if _, err := os.Stat(filepath.Join(dir, "docs", "index.md")); err != nil {
		t.Error("expected docs/index.md to exist")
	}

	// Check mkdocs.yml content.
	data, err := os.ReadFile(filepath.Join(dir, "mkdocs.yml"))
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
	if err := os.WriteFile(filepath.Join(dir, ".docz.yaml"), []byte("docs_dir: docs\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := runWikiInit(nil, nil)
	if err != nil {
		t.Fatalf("runWikiInit() error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "mkdocs.yml")); err != nil {
		t.Error("expected mkdocs.yml to exist")
	}
}

func TestWikiInit_SiteName(t *testing.T) {
	dir := setupWikiTestDir(t)
	wikiSiteName = "My Service"
	t.Cleanup(func() { wikiSiteName = "" })
	err := runWikiInit(nil, nil)
	if err != nil {
		t.Fatalf("runWikiInit() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "mkdocs.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "site_name: My Service") {
		t.Error("mkdocs.yml should contain site_name: My Service")
	}
}

func TestWikiInit_SiteDescription(t *testing.T) {
	dir := setupWikiTestDir(t)
	wikiSiteDescription = "Custom description"
	t.Cleanup(func() { wikiSiteDescription = "" })
	err := runWikiInit(nil, nil)
	if err != nil {
		t.Fatalf("runWikiInit() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "mkdocs.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "site_description: Custom description") {
		t.Error("expected custom site_description in mkdocs.yml")
	}
}

func TestWikiInit_FailsIfExists(t *testing.T) {
	dir := setupWikiTestDir(t)

	// Create mkdocs.yml first.
	if err := os.WriteFile(filepath.Join(dir, "mkdocs.yml"), []byte("site_name: test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Also create .docz.yaml and docs/ to skip auto-init.
	if err := os.WriteFile(filepath.Join(dir, ".docz.yaml"), []byte("docs_dir: docs\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o750); err != nil {
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
	dir := setupWikiTestDir(t)

	// Create existing mkdocs.yml.
	if err := os.WriteFile(filepath.Join(dir, "mkdocs.yml"), []byte("site_name: old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".docz.yaml"), []byte("docs_dir: docs\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o750); err != nil {
		t.Fatal(err)
	}

	wikiForce = true
	wikiSiteName = "New Name"
	t.Cleanup(func() {
		wikiForce = false
		wikiSiteName = ""
	})
	err := runWikiInit(nil, nil)
	if err != nil {
		t.Fatalf("runWikiInit() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "mkdocs.yml"))
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
	err := runWikiUpdate(nil, nil)
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

	// Capture handler output via the Runner's Out writer — no
	// os.Stdout redirection or pipe.
	var capture bytes.Buffer
	runner.Out = &capture

	if err := runWikiUpdate(nil, nil); err != nil {
		t.Fatalf("runWikiUpdate() error: %v", err)
	}

	if !strings.Contains(capture.String(), "Home") {
		t.Errorf("dry-run output should contain 'Home', got %q", capture.String())
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
	err := runWikiUpdate(nil, nil)
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
	runner.Cfg.Wiki.AutoUpdate = true
	err := runCreate(nil, []string{"rfc", "Test RFC"})
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

	runner.Cfg.Wiki.AutoUpdate = true
	err := runCreate(nil, []string{"rfc", "Test RFC"})
	if err != nil {
		t.Fatalf("runCreate() should succeed without mkdocs.yml: %v", err)
	}

	// mkdocs.yml should NOT have been created.
	if _, err := os.Stat(filepath.Join(dir, "mkdocs.yml")); err == nil {
		t.Error("mkdocs.yml should not be created by docz create")
	}
}

func TestWikiInit_Plugins(t *testing.T) {
	dir := setupWikiTestDir(t)
	err := runWikiInit(nil, nil)
	if err != nil {
		t.Fatalf("runWikiInit() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "mkdocs.yml"))
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	if !strings.Contains(content, "- techdocs-core") {
		t.Error("mkdocs.yml should contain techdocs-core plugin")
	}
}

func TestWikiInit_MultiplePlugins(t *testing.T) {
	dir := setupWikiTestDir(t)
	runner.Cfg.Wiki.Plugins = []string{"techdocs-core", "search", "mermaid"}
	err := runWikiInit(nil, nil)
	if err != nil {
		t.Fatalf("runWikiInit() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "mkdocs.yml"))
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	for _, plugin := range []string{"techdocs-core", "search", "mermaid"} {
		if !strings.Contains(content, "- "+plugin) {
			t.Errorf("mkdocs.yml should contain plugin %q", plugin)
		}
	}
}

func TestWikiInit_MarkdownExtensions(t *testing.T) {
	dir := setupWikiTestDir(t)
	runner.Cfg.Wiki.MarkdownExtensions = []string{"admonition", "tables", "pymdownx.tasklist"}
	err := runWikiInit(nil, nil)
	if err != nil {
		t.Fatalf("runWikiInit() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "mkdocs.yml"))
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	if !strings.Contains(content, "markdown_extensions:") {
		t.Error("mkdocs.yml should contain markdown_extensions section")
	}
	for _, ext := range []string{"admonition", "tables", "pymdownx.tasklist"} {
		if !strings.Contains(content, "- "+ext) {
			t.Errorf("mkdocs.yml should contain extension %q", ext)
		}
	}
}

func TestWikiInit_AllOptionalFields(t *testing.T) {
	dir := setupWikiTestDir(t)
	runner.Cfg.Wiki.DocsDir = "documentation"
	runner.Cfg.Wiki.RepoURL = "https://github.com/example/repo"
	runner.Cfg.Wiki.SiteURL = "https://example.com/docs"
	runner.Cfg.Wiki.Theme = "readthedocs"
	err := runWikiInit(nil, nil)
	if err != nil {
		t.Fatalf("runWikiInit() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "mkdocs.yml"))
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	for _, want := range []string{
		"docs_dir: documentation",
		"repo_url: https://github.com/example/repo",
		"site_url: https://example.com/docs",
		"theme: readthedocs",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("mkdocs.yml should contain %q", want)
		}
	}
}

func TestWikiInit_OmitsEmptyOptionalFields(t *testing.T) {
	dir := setupWikiTestDir(t)
	// All optional fields are zero values by default.
	err := runWikiInit(nil, nil)
	if err != nil {
		t.Fatalf("runWikiInit() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "mkdocs.yml"))
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	for _, absent := range []string{"docs_dir:", "repo_url:", "site_url:", "theme:", "markdown_extensions:"} {
		if strings.Contains(content, absent) {
			t.Errorf("mkdocs.yml should not contain %q when not configured", absent)
		}
	}
}

func TestWikiInit_IndexTemplate(t *testing.T) {
	dir := setupWikiTestDir(t)
	err := runWikiInit(nil, nil)
	if err != nil {
		t.Fatalf("runWikiInit() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "docs", "index.md"))
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	// Should contain enabled types.
	for _, navTitle := range []string{"RFCs", "ADRs", "Design"} {
		if !strings.Contains(content, navTitle) {
			t.Errorf("index.md should contain %q", navTitle)
		}
	}
}

func TestWikiInit_IndexSkipsDisabledTypes(t *testing.T) {
	dir := setupWikiTestDir(t)

	// Pre-create .docz.yaml and docs/ so ensureDoczInit skips runInit.
	writeTestFile(t, filepath.Join(dir, ".docz.yaml"), "docs_dir: docs\n")
	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o750); err != nil {
		t.Fatal(err)
	}

	// Disable plan and investigation.
	tc := runner.Cfg.Types["plan"]
	tc.Enabled = false
	runner.Cfg.Types["plan"] = tc

	tc = runner.Cfg.Types["investigation"]
	tc.Enabled = false
	runner.Cfg.Types["investigation"] = tc
	err := runWikiInit(nil, nil)
	if err != nil {
		t.Fatalf("runWikiInit() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "docs", "index.md"))
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	if strings.Contains(content, "plan/README.md") {
		t.Error("index.md should not contain disabled type plan")
	}
	if strings.Contains(content, "investigation/README.md") {
		t.Error("index.md should not contain disabled type investigation")
	}
	// Enabled types should still be present.
	if !strings.Contains(content, "RFCs") {
		t.Error("index.md should contain enabled type RFCs")
	}
}

func TestWikiInit_IndexTemplateOverride(t *testing.T) {
	dir := setupWikiTestDir(t)

	// Create a local override template.
	templatesDir := filepath.Join(dir, "docs", "templates")
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	override := "# {{ .SiteName }} Custom\n\nCustom homepage.\n"
	writeTestFile(t, filepath.Join(templatesDir, "wiki_index.md"), override)
	err := runWikiInit(nil, nil)
	if err != nil {
		t.Fatalf("runWikiInit() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "docs", "index.md"))
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	if !strings.Contains(content, "Custom homepage.") {
		t.Error("index.md should use the local override template")
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
