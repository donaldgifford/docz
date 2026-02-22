package template

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "update golden files")

func TestGoldenTemplates(t *testing.T) {
	data := TemplateData{
		Number:   "0001",
		Title:    "Test Document",
		Date:     "2026-02-22",
		Author:   "Test Author",
		Status:   "Draft",
		Type:     "rfc",
		Prefix:   "RFC",
		Slug:     "test-document",
		Filename: "0001-test-document.md",
	}

	types := map[string]TemplateData{
		"rfc":    data,
		"adr":    withOverrides(data, "adr", "ADR", "Proposed"),
		"design": withOverrides(data, "design", "DESIGN", "Draft"),
		"impl":   withOverrides(data, "impl", "IMPL", "Draft"),
	}

	for typeName, td := range types {
		t.Run(typeName, func(t *testing.T) {
			tmpl, err := EmbeddedDocumentTemplate(typeName)
			if err != nil {
				t.Fatalf("EmbeddedDocumentTemplate(%q): %v", typeName, err)
			}

			got, err := Render(tmpl, td)
			if err != nil {
				t.Fatalf("Render(): %v", err)
			}

			goldenPath := filepath.Join("..", "..", "testdata", "golden", typeName+".md")

			if *update {
				if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}

			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("reading golden file %s: %v\nRun with -update to create it", goldenPath, err)
			}

			if got != string(want) {
				t.Errorf("template output differs from golden file %s\nRun with -update to update", goldenPath)
			}
		})
	}
}

func withOverrides(base TemplateData, typeName, prefix, status string) TemplateData {
	base.Type = typeName
	base.Prefix = prefix
	base.Status = status
	return base
}
