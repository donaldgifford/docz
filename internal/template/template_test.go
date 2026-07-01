package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/pkg/doczcore/config"
)

func TestFilenameSlug(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "simple title", input: "API Rate Limiting", want: "api-rate-limiting"},
		{name: "already kebab", input: "already-kebab", want: "already-kebab"},
		{name: "special characters", input: "What's the Plan?", want: "whats-the-plan"},
		{name: "multiple spaces", input: "lots   of   spaces", want: "lots-of-spaces"},
		{name: "leading/trailing spaces", input: "  trim me  ", want: "trim-me"},
		{name: "uppercase", input: "ALL UPPERCASE", want: "all-uppercase"},
		{name: "numbers", input: "Phase 2 Design", want: "phase-2-design"},
		{name: "empty string", input: "", want: ""},
		{name: "only special chars", input: "!@#$%", want: ""},
		{name: "mixed special", input: "go-based CLI (v2)", want: "go-based-cli-v2"},
		{name: "leading hyphens after strip", input: "---leading", want: "leading"},
		{name: "trailing hyphens after strip", input: "trailing---", want: "trailing"},
		{name: "unicode chars", input: "über design für API", want: "ber-design-fr-api"},
		{
			name:  "very long title",
			input: "This Is A Very Long Title That Should Be Truncated Because It Exceeds The Maximum Slug Length Limit",
			want:  "this-is-a-very-long-title-that-should-be-truncated-because-it",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FilenameSlug(tt.input)
			if got != tt.want {
				t.Errorf("FilenameSlug(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestEmbeddedDocumentTemplate(t *testing.T) {
	t.Parallel()
	for _, docType := range []config.DocType{"rfc", "adr", "design", "impl"} {
		t.Run(string(docType), func(t *testing.T) {
			t.Parallel()
			content, err := EmbeddedDocumentTemplate(docType)
			if err != nil {
				t.Fatalf("EmbeddedDocumentTemplate(%q) error: %v", docType, err)
			}
			if content == "" {
				t.Errorf("EmbeddedDocumentTemplate(%q) returned empty content", docType)
			}
			// Verify template contains expected placeholders.
			if !containsAll(content, "{{ .Title }}", "{{ .Status }}", "{{ .Author }}") {
				t.Errorf("template for %q missing expected placeholders", docType)
			}
		})
	}
}

func TestEmbeddedDocumentTemplate_InvalidType(t *testing.T) {
	t.Parallel()
	_, err := EmbeddedDocumentTemplate(config.DocType("nonexistent"))
	if err == nil {
		t.Error("expected error for nonexistent type, got nil")
	}
}

func TestResolve_EmbeddedDefault(t *testing.T) {
	t.Parallel()
	content, err := Resolve("rfc", "", "/nonexistent/path")
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if content == "" {
		t.Error("Resolve() returned empty content for embedded default")
	}
}

// TestResolveIndexHeader_LocalOverride proves tier 1 wins and is returned
// byte-for-byte, including a literal "{{" that must NOT be rendered.
func TestResolveIndexHeader_LocalOverride(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tmplDir := filepath.Join(dir, config.TemplatesDir)
	if err := os.MkdirAll(tmplDir, 0o755); err != nil {
		t.Fatalf("creating templates dir: %v", err)
	}
	override := "# Custom Header {{ raw }}\n\nVerbatim override.\n\n"
	if err := os.WriteFile(filepath.Join(tmplDir, "index_frameworks.md"), []byte(override), 0o644); err != nil {
		t.Fatalf("writing override: %v", err)
	}

	got, err := ResolveIndexHeader("frameworks", dir, IndexHeaderData{TypeName: "frameworks", PluralLabel: "Frameworks"})
	if err != nil {
		t.Fatalf("ResolveIndexHeader() error: %v", err)
	}
	if got != override {
		t.Errorf("ResolveIndexHeader() = %q, want verbatim override %q", got, override)
	}
}

// TestResolveIndexHeader_EmbeddedBuiltin is the golden-stability guard and
// the registry-coupling invariant: for every doc type in the registry, a
// matching embedded index_<TemplateName>.md must exist and ResolveIndexHeader
// must return it byte-for-byte (tier 2 is verbatim, so a built-in's curated
// header is never replaced by the generic fallback and README output never
// churns). Adding a registry entry without its embedded header fails here.
func TestResolveIndexHeader_EmbeddedBuiltin(t *testing.T) {
	t.Parallel()
	for _, dt := range config.AllDocTypes() {
		t.Run(dt.Name, func(t *testing.T) {
			t.Parallel()
			want, err := templateFS.ReadFile("templates/index_" + dt.TemplateName + ".md")
			if err != nil {
				t.Fatalf("doc type %q has no embedded index_%s.md: %v", dt.Name, dt.TemplateName, err)
			}
			got, err := ResolveIndexHeader(dt.Name, "/nonexistent/path", IndexHeaderData{TypeName: dt.Name})
			if err != nil {
				t.Fatalf("ResolveIndexHeader(%q) error: %v", dt.Name, err)
			}
			if got != string(want) {
				t.Errorf("ResolveIndexHeader(%q) not byte-identical to embedded header", dt.Name)
			}
		})
	}
}

// TestResolveIndexHeader_GenericFallback covers tier 3 — a custom type with
// no override and no embedded header renders index_default.md with its label.
func TestResolveIndexHeader_GenericFallback(t *testing.T) {
	t.Parallel()
	got, err := ResolveIndexHeader(
		"frameworks", "/nonexistent/path",
		IndexHeaderData{TypeName: "frameworks", PluralLabel: "Frameworks"},
	)
	if err != nil {
		t.Fatalf("ResolveIndexHeader() error: %v", err)
	}
	for _, want := range []string{"Frameworks", "docz create frameworks"} {
		if !strings.Contains(got, want) {
			t.Errorf("ResolveIndexHeader() generic fallback = %q, want it to contain %q", got, want)
		}
	}
}

// TestResolveIndexHeader_GenericFallbackEmptyLabel proves an empty
// PluralLabel still yields a non-empty, well-formed header.
func TestResolveIndexHeader_GenericFallbackEmptyLabel(t *testing.T) {
	t.Parallel()
	got, err := ResolveIndexHeader(
		"frameworks", "/nonexistent/path",
		IndexHeaderData{TypeName: "frameworks"},
	)
	if err != nil {
		t.Fatalf("ResolveIndexHeader() error: %v", err)
	}
	if got == "" {
		t.Fatal("ResolveIndexHeader() with empty label returned empty content")
	}
	if !strings.Contains(got, "docz create frameworks") {
		t.Errorf("ResolveIndexHeader() = %q, want the create example", got)
	}
}

func TestResolve_LocalOverride(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	templatesDir := filepath.Join(dir, "templates")
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	overrideContent := "# Custom RFC Template\n{{ .Title }}"
	if err := os.WriteFile(filepath.Join(templatesDir, "rfc.md"), []byte(overrideContent), 0o644); err != nil {
		t.Fatal(err)
	}

	content, err := Resolve("rfc", "", dir)
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if content != overrideContent {
		t.Errorf("Resolve() = %q, want %q", content, overrideContent)
	}
}

func TestResolve_ConfigPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	customPath := filepath.Join(dir, "my-rfc.md")
	customContent := "# My Custom Template"
	if err := os.WriteFile(customPath, []byte(customContent), 0o644); err != nil {
		t.Fatal(err)
	}

	content, err := Resolve("rfc", customPath, "/nonexistent")
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if content != customContent {
		t.Errorf("Resolve() = %q, want %q", content, customContent)
	}
}

func TestResolve_ConfigPathNotFound(t *testing.T) {
	t.Parallel()
	_, err := Resolve("rfc", "/nonexistent/template.md", "/nonexistent")
	if err == nil {
		t.Error("expected error for missing config path, got nil")
	}
}

func TestRender(t *testing.T) {
	t.Parallel()
	tmpl := "# {{ .Prefix }}-{{ .Number }}: {{ .Title }}\nBy {{ .Author }} on {{ .Date }}"
	data := Data{
		Number:   "0001",
		Title:    "Test Document",
		Date:     "2026-02-22",
		Author:   "Test Author",
		Status:   "Draft",
		Type:     "RFC",
		Prefix:   "RFC",
		Slug:     "test-document",
		Filename: "0001-test-document.md",
	}

	got, err := Render(tmpl, &data)
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}

	want := "# RFC-0001: Test Document\nBy Test Author on 2026-02-22"
	if got != want {
		t.Errorf("Render() = %q, want %q", got, want)
	}
}

func TestRender_InvalidTemplate(t *testing.T) {
	t.Parallel()
	_, err := Render("{{ .Invalid", &Data{})
	if err == nil {
		t.Error("expected error for invalid template, got nil")
	}
}

func TestEmbeddedWikiIndex(t *testing.T) {
	t.Parallel()
	content, err := EmbeddedWikiIndex()
	if err != nil {
		t.Fatalf("EmbeddedWikiIndex() error: %v", err)
	}
	if content == "" {
		t.Error("EmbeddedWikiIndex() returned empty content")
	}
	if !containsAll(content, "{{ .SiteName }}", ".Types") {
		t.Error("wiki index template missing expected placeholders")
	}
}

func TestResolveWikiIndex_Embedded(t *testing.T) {
	t.Parallel()
	content, err := ResolveWikiIndex("/nonexistent/path")
	if err != nil {
		t.Fatalf("ResolveWikiIndex() error: %v", err)
	}
	if content == "" {
		t.Error("ResolveWikiIndex() returned empty content")
	}
}

func TestResolveWikiIndex_LocalOverride(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	templatesDir := filepath.Join(dir, "templates")
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	override := "# Custom Home\n{{ .SiteName }}\n"
	if err := os.WriteFile(
		filepath.Join(templatesDir, "wiki_index.md"),
		[]byte(override),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	content, err := ResolveWikiIndex(dir)
	if err != nil {
		t.Fatalf("ResolveWikiIndex() error: %v", err)
	}
	if content != override {
		t.Errorf("ResolveWikiIndex() = %q, want %q", content, override)
	}
}

func TestRenderWikiIndex(t *testing.T) {
	t.Parallel()
	tmpl := "# {{ .SiteName }}\n{{ range .Types }}- {{ .NavTitle }}\n{{ end }}"
	data := &WikiIndexData{
		SiteName: "My Project",
		Types: []WikiIndexType{
			{Name: "rfc", NavTitle: "RFCs", Dir: "rfc"},
			{Name: "adr", NavTitle: "ADRs", Dir: "adr"},
		},
	}

	got, err := RenderWikiIndex(tmpl, data)
	if err != nil {
		t.Fatalf("RenderWikiIndex() error: %v", err)
	}

	if !containsAll(got, "# My Project", "- RFCs", "- ADRs") {
		t.Errorf("RenderWikiIndex() missing expected content:\n%s", got)
	}
}

func containsAll(s string, substrings ...string) bool {
	for _, sub := range substrings {
		found := false
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
