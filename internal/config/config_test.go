package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.DocsDir != "docs" {
		t.Errorf("DocsDir = %q, want %q", cfg.DocsDir, "docs")
	}

	for _, typeName := range ValidTypes() {
		tc, ok := cfg.Types[typeName]
		if !ok {
			t.Errorf("missing type config for %q", typeName)
			continue
		}
		if !tc.Enabled {
			t.Errorf("type %q should be enabled by default", typeName)
		}
		if tc.IDWidth != 4 {
			t.Errorf("type %q IDWidth = %d, want 4", typeName, tc.IDWidth)
		}
		if len(tc.Statuses) == 0 {
			t.Errorf("type %q has no statuses", typeName)
		}
		if tc.StatusField != "status" {
			t.Errorf("type %q StatusField = %q, want %q", typeName, tc.StatusField, "status")
		}
	}

	if !cfg.Index.AutoUpdate {
		t.Error("Index.AutoUpdate should be true by default")
	}
	if !cfg.Author.FromGit {
		t.Error("Author.FromGit should be true by default")
	}

	// Wiki defaults.
	if !cfg.Wiki.AutoUpdate {
		t.Error("Wiki.AutoUpdate should be true by default")
	}
	if cfg.Wiki.MkDocsPath != "mkdocs.yml" {
		t.Errorf("Wiki.MkDocsPath = %q, want %q", cfg.Wiki.MkDocsPath, "mkdocs.yml")
	}
	wantExclude := []string{"templates", "examples"}
	if len(cfg.Wiki.Exclude) != len(wantExclude) {
		t.Fatalf("Wiki.Exclude has %d elements, want %d", len(cfg.Wiki.Exclude), len(wantExclude))
	}
	for i, got := range cfg.Wiki.Exclude {
		if got != wantExclude[i] {
			t.Errorf("Wiki.Exclude[%d] = %q, want %q", i, got, wantExclude[i])
		}
	}
	wantTitles := map[string]string{
		"rfc": "RFCs", "adr": "ADRs", "design": "Design",
		"impl": "Implementation Plans", "plan": "Plans",
		"investigation": "Investigations",
	}
	for k, want := range wantTitles {
		if got := cfg.Wiki.NavTitles[k]; got != want {
			t.Errorf("Wiki.NavTitles[%q] = %q, want %q", k, got, want)
		}
	}
}

func TestValidTypes(t *testing.T) {
	types := ValidTypes()
	want := []string{"rfc", "adr", "design", "impl", "plan", "investigation"}
	if len(types) != len(want) {
		t.Fatalf("ValidTypes() has %d elements, want %d", len(types), len(want))
	}
	for i, got := range types {
		if got != want[i] {
			t.Errorf("ValidTypes()[%d] = %q, want %q", i, got, want[i])
		}
	}
}

func TestTypeDir(t *testing.T) {
	cfg := DefaultConfig()

	tests := []struct {
		docType string
		want    string
	}{
		{"rfc", filepath.Join("docs", "rfc")},
		{"adr", filepath.Join("docs", "adr")},
		{"design", filepath.Join("docs", "design")},
		{"impl", filepath.Join("docs", "impl")},
		{"unknown", filepath.Join("docs", "unknown")},
	}

	for _, tt := range tests {
		t.Run(tt.docType, func(t *testing.T) {
			got := cfg.TypeDir(tt.docType)
			if got != tt.want {
				t.Errorf("TypeDir(%q) = %q, want %q", tt.docType, got, tt.want)
			}
		})
	}
}

func TestLoad_NoConfigFiles(t *testing.T) {
	// Run in a temp dir with no config files.
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Should return defaults.
	if cfg.DocsDir != "docs" {
		t.Errorf("DocsDir = %q, want %q", cfg.DocsDir, "docs")
	}
	if len(cfg.Types) != 6 {
		t.Errorf("expected 6 types, got %d", len(cfg.Types))
	}
}

func TestLoad_RepoConfig(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	configContent := `docs_dir: documentation
author:
  default: "Test Author"
  from_git: false
`
	if err := os.WriteFile(".docz.yaml", []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.DocsDir != "documentation" {
		t.Errorf("DocsDir = %q, want %q", cfg.DocsDir, "documentation")
	}
	if cfg.Author.Default != "Test Author" {
		t.Errorf("Author.Default = %q, want %q", cfg.Author.Default, "Test Author")
	}
	if cfg.Author.FromGit {
		t.Error("Author.FromGit should be false")
	}
	// Types should still have defaults.
	if len(cfg.Types) != 6 {
		t.Errorf("expected 6 types, got %d", len(cfg.Types))
	}
}

func TestLoad_ExplicitConfigFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "custom.yaml")
	configContent := `docs_dir: custom-docs
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.DocsDir != "custom-docs" {
		t.Errorf("DocsDir = %q, want %q", cfg.DocsDir, "custom-docs")
	}
}

func TestLoad_WikiConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "custom.yaml")
	configContent := `wiki:
  auto_update: false
  mkdocs_path: docs/mkdocs.yml
  exclude:
    - drafts
  nav_titles:
    rfc: "Request for Comments"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Wiki.AutoUpdate {
		t.Error("Wiki.AutoUpdate should be false")
	}
	if cfg.Wiki.MkDocsPath != "docs/mkdocs.yml" {
		t.Errorf("Wiki.MkDocsPath = %q, want %q", cfg.Wiki.MkDocsPath, "docs/mkdocs.yml")
	}
	if len(cfg.Wiki.Exclude) != 1 || cfg.Wiki.Exclude[0] != "drafts" {
		t.Errorf("Wiki.Exclude = %v, want [drafts]", cfg.Wiki.Exclude)
	}
	if got := cfg.Wiki.NavTitles["rfc"]; got != "Request for Comments" {
		t.Errorf("Wiki.NavTitles[rfc] = %q, want %q", got, "Request for Comments")
	}
}

func TestResolveTypeAlias(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"impl", "impl"},
		{"implementation", "impl"},
		{"investigation", "investigation"},
		{"inv", "investigation"},
		{"rfc", "rfc"},
		{"adr", "adr"},
		{"design", "design"},
		{"plan", "plan"},
		{"unknown", "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ResolveTypeAlias(tt.input)
			if got != tt.want {
				t.Errorf("ResolveTypeAlias(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTypesHelp(t *testing.T) {
	help := TypesHelp()
	for _, typeName := range []string{"rfc", "adr", "design", "impl", "plan", "investigation"} {
		if !strings.Contains(help, typeName) {
			t.Errorf("TypesHelp() missing type %q", typeName)
		}
	}
	for _, alias := range []string{"implementation", "inv"} {
		if !strings.Contains(help, alias) {
			t.Errorf("TypesHelp() missing alias %q", alias)
		}
	}
}

func TestValidate_DefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	warnings, err := cfg.Validate()
	if err != nil {
		t.Errorf("Validate() error: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
}

func TestValidate_EmptyDocsDir(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DocsDir = ""
	_, err := cfg.Validate()
	if err == nil {
		t.Error("expected error for empty docs_dir, got nil")
	}
}

func TestValidate_EmptyStatuses(t *testing.T) {
	cfg := DefaultConfig()
	tc := cfg.Types["rfc"]
	tc.Statuses = nil
	cfg.Types["rfc"] = tc

	_, err := cfg.Validate()
	if err == nil {
		t.Error("expected error for empty statuses, got nil")
	}
}

func TestValidate_UnknownType(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Types["custom"] = TypeConfig{Enabled: true, Statuses: []string{"Draft"}}

	warnings, err := cfg.Validate()
	if err != nil {
		t.Errorf("Validate() error: %v", err)
	}
	if len(warnings) == 0 {
		t.Error("expected warning for unknown type, got none")
	}
}
