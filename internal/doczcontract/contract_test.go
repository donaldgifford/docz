package doczcontract_test

import (
	"errors"
	"path/filepath"
	"slices"
	"testing"

	doczcfg "github.com/donaldgifford/docz/pkg/doczcore/config"
	doczdoc "github.com/donaldgifford/docz/pkg/doczcore/document"
)

// repoDir is the testdata fixture repo: a .docz.yaml declaring the built-in
// rfc/investigation types plus a custom "frameworks" type (id_prefix FW,
// alias fw), and one document under docs/frameworks.
const repoDir = "testdata/repo"

// loadConfig loads the fixture manifest hermetically. Load merges
// $HOME/.docz.yaml before the repo config, so HOME is pointed at an empty
// temp dir to keep the fixture the only input.
func loadConfig(t *testing.T) doczcfg.Config {
	t.Helper()
	t.Setenv("HOME", t.TempDir())

	cfg, err := doczcfg.Load("", repoDir)
	if err != nil {
		t.Fatalf("config.Load(%q): %v", repoDir, err)
	}
	if warnings, err := cfg.Validate(); err != nil {
		t.Fatalf("config.Validate: %v (warnings: %v)", err, warnings)
	}
	return cfg
}

func TestConfigLoadsFixtureManifest(t *testing.T) {
	cfg := loadConfig(t)

	if cfg.DocsDir != "docs" {
		t.Errorf("DocsDir = %q, want %q", cfg.DocsDir, "docs")
	}
	if got, want := cfg.TypeDir("frameworks"), filepath.Join("docs", "frameworks"); got != want {
		t.Errorf("TypeDir(frameworks) = %q, want %q", got, want)
	}
	if enabled := cfg.EnabledTypes(); !slices.Contains(enabled, "frameworks") {
		t.Errorf("EnabledTypes() = %v, want it to include %q", enabled, "frameworks")
	}
}

func TestValidateTypeResolvesNameAliasPrefix(t *testing.T) {
	cfg := loadConfig(t)

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"canonical name", "rfc", "rfc"},
		{"builtin alias", "inv", "investigation"},
		{"custom name", "frameworks", "frameworks"},
		{"custom alias", "fw", "frameworks"},
		{"id_prefix", "FW", "frameworks"},
		{"case-insensitive prefix", "fW", "frameworks"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := cfg.ValidateType(tt.input)
			if err != nil {
				t.Fatalf("ValidateType(%q): unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("ValidateType(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateTypeRejectsUnknown(t *testing.T) {
	cfg := loadConfig(t)

	if _, err := cfg.ValidateType("nonexistent"); !errors.Is(err, doczcfg.ErrUnknownType) {
		t.Fatalf("ValidateType(nonexistent) error = %v, want errors.Is ErrUnknownType", err)
	}
}

func TestParseFrontmatter(t *testing.T) {
	const doc = "---\n" +
		"id: FW-0001\n" +
		"title: Example Framework\n" +
		"status: Draft\n" +
		"author: Test Author\n" +
		"created: 2026-07-01\n" +
		"---\n\n# FW 0001\n\nBody.\n"

	fm, err := doczdoc.ParseFrontmatter([]byte(doc))
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if fm.ID != "FW-0001" {
		t.Errorf("ID = %q, want %q", fm.ID, "FW-0001")
	}
	if fm.Title != "Example Framework" {
		t.Errorf("Title = %q, want %q", fm.Title, "Example Framework")
	}
	if string(fm.Status) != "Draft" {
		t.Errorf("Status = %q, want %q", fm.Status, "Draft")
	}

	if _, err := doczdoc.ParseFrontmatter([]byte("no frontmatter here")); !errors.Is(err, doczdoc.ErrNoFrontmatter) {
		t.Errorf("ParseFrontmatter(no delimiters) error = %v, want errors.Is ErrNoFrontmatter", err)
	}
}

func TestScanDocumentsPopulatesContent(t *testing.T) {
	dir := filepath.Join(repoDir, "docs", "frameworks")

	docs, err := doczdoc.ScanDocuments(dir)
	if err != nil {
		t.Fatalf("ScanDocuments(%q): %v", dir, err)
	}
	if len(docs) != 1 {
		t.Fatalf("ScanDocuments returned %d docs, want 1", len(docs))
	}

	entry := docs[0]
	if entry.ID != "FW-0001" {
		t.Errorf("DocEntry.ID = %q, want %q", entry.ID, "FW-0001")
	}
	if len(entry.Content) == 0 {
		t.Error("DocEntry.Content is empty, want the raw file bytes")
	}
}

func TestIsDoczFile(t *testing.T) {
	tests := []struct {
		filename string
		want     bool
	}{
		{"0001-intro.md", true},
		{"0042-some-slug.md", true},
		{"README.md", false},
		{"notes.md", false},
		{"0001-intro.txt", false},
	}
	for _, tt := range tests {
		if got := doczdoc.IsDoczFile(tt.filename); got != tt.want {
			t.Errorf("IsDoczFile(%q) = %v, want %v", tt.filename, got, tt.want)
		}
	}
}
