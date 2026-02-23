package template

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSlugify(t *testing.T) {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Slugify(tt.input)
			if got != tt.want {
				t.Errorf("Slugify(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestEmbeddedDocumentTemplate(t *testing.T) {
	for _, docType := range []string{"rfc", "adr", "design", "impl"} {
		t.Run(docType, func(t *testing.T) {
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
	_, err := EmbeddedDocumentTemplate("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent type, got nil")
	}
}

func TestEmbeddedIndexHeader(t *testing.T) {
	for _, docType := range []string{"rfc", "adr", "design", "impl"} {
		t.Run(docType, func(t *testing.T) {
			content, err := EmbeddedIndexHeader(docType)
			if err != nil {
				t.Fatalf("EmbeddedIndexHeader(%q) error: %v", docType, err)
			}
			if content == "" {
				t.Errorf("EmbeddedIndexHeader(%q) returned empty content", docType)
			}
		})
	}
}

func TestResolve_EmbeddedDefault(t *testing.T) {
	content, err := Resolve("rfc", "", "/nonexistent/path")
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if content == "" {
		t.Error("Resolve() returned empty content for embedded default")
	}
}

func TestResolve_LocalOverride(t *testing.T) {
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
	_, err := Resolve("rfc", "/nonexistent/template.md", "/nonexistent")
	if err == nil {
		t.Error("expected error for missing config path, got nil")
	}
}

func TestRender(t *testing.T) {
	tmpl := "# {{ .Prefix }}-{{ .Number }}: {{ .Title }}\nBy {{ .Author }} on {{ .Date }}"
	data := TemplateData{
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
	_, err := Render("{{ .Invalid", &TemplateData{})
	if err == nil {
		t.Error("expected error for invalid template, got nil")
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
