package wiki

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateMkDocs(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		cfg  MkDocsConfig
		want string
	}{
		{
			name: "minimal_only_site_name_and_description",
			cfg: MkDocsConfig{
				SiteName:        "My Service",
				SiteDescription: "Documentation for My Service",
			},
			want: "site_name: My Service\n" +
				"site_description: Documentation for My Service\n" +
				"\nnav:\n    - Home: index.md\n",
		},
		{
			name: "full_config_all_optional_fields",
			cfg: MkDocsConfig{
				SiteName:        "My Service",
				SiteDescription: "Documentation for My Service",
				DocsDir:         "docs",
				RepoURL:         "https://github.com/example/my-service",
				SiteURL:         "https://docs.example.com",
				Theme:           "material",
				Plugins: []string{
					"techdocs-core",
					"search",
				},
				MarkdownExtensions: []string{
					"admonition",
					"toc",
				},
			},
			want: "site_name: My Service\n" +
				"site_description: Documentation for My Service\n" +
				"docs_dir: docs\n" +
				"repo_url: https://github.com/example/my-service\n" +
				"site_url: https://docs.example.com\n" +
				"theme: material\n" +
				"\nplugins:\n" +
				"    - techdocs-core\n" +
				"    - search\n" +
				"\nmarkdown_extensions:\n" +
				"    - admonition\n" +
				"    - toc\n" +
				"\nnav:\n    - Home: index.md\n",
		},
		{
			name: "plugins_preserve_input_order",
			cfg: MkDocsConfig{
				SiteName:        "X",
				SiteDescription: "Y",
				Plugins:         []string{"zeta", "alpha", "mike"},
			},
			want: "site_name: X\n" +
				"site_description: Y\n" +
				"\nplugins:\n" +
				"    - zeta\n" +
				"    - alpha\n" +
				"    - mike\n" +
				"\nnav:\n    - Home: index.md\n",
		},
		{
			name: "markdown_extensions_emitted_alone",
			cfg: MkDocsConfig{
				SiteName:           "X",
				SiteDescription:    "Y",
				MarkdownExtensions: []string{"admonition"},
			},
			want: "site_name: X\n" +
				"site_description: Y\n" +
				"\nmarkdown_extensions:\n" +
				"    - admonition\n" +
				"\nnav:\n    - Home: index.md\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "mkdocs.yml")
			if err := CreateMkDocs(path, &tc.cfg); err != nil {
				t.Fatalf("CreateMkDocs() error: %v", err)
			}

			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading written file: %v", err)
			}

			if string(got) != tc.want {
				t.Errorf("output mismatch\nwant:\n%s\n\ngot:\n%s", tc.want, string(got))
			}
		})
	}
}

func TestCreateMkDocs_WriteError(t *testing.T) {
	t.Parallel()
	// Writing to a path whose parent directory does not exist should fail.
	missingDir := filepath.Join(t.TempDir(), "nonexistent", "mkdocs.yml")
	err := CreateMkDocs(missingDir, &MkDocsConfig{SiteName: "X", SiteDescription: "Y"})
	if err == nil {
		t.Fatal("expected error for missing parent directory, got nil")
	}
	if !strings.Contains(err.Error(), "writing") {
		t.Errorf("error %q does not mention 'writing'", err.Error())
	}
}

func TestReadWriteMkDocs_RoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "mkdocs.yml")

	original := `site_name: My Service
site_description: Documentation for My Service
plugins:
    - techdocs-core
theme:
    name: material
nav:
    - Home: index.md
    - RFCs:
        - Overview: rfc/README.md
`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	data, err := ReadMkDocs(path)
	if err != nil {
		t.Fatalf("ReadMkDocs() error: %v", err)
	}

	// Verify non-nav fields are preserved.
	if data["site_name"] != "My Service" {
		t.Errorf("site_name = %v, want My Service", data["site_name"])
	}
	if data["site_description"] != "Documentation for My Service" {
		t.Errorf(
			"site_description = %v, want Documentation for My Service",
			data["site_description"],
		)
	}

	// Write it back.
	if err := WriteMkDocs(path, data); err != nil {
		t.Fatalf("WriteMkDocs() error: %v", err)
	}

	// Re-read and verify fields still present.
	data2, err := ReadMkDocs(path)
	if err != nil {
		t.Fatalf("second ReadMkDocs() error: %v", err)
	}
	if data2["site_name"] != "My Service" {
		t.Errorf("after round-trip, site_name = %v", data2["site_name"])
	}
	if data2["theme"] == nil {
		t.Error("theme was lost during round-trip")
	}
}

func TestReadMkDocs_FileNotFound(t *testing.T) {
	t.Parallel()
	_, err := ReadMkDocs("/nonexistent/mkdocs.yml")
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
}

func TestReadMkDocs_InvalidYAML(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "mkdocs.yml")
	if err := os.WriteFile(path, []byte(":::invalid yaml"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := ReadMkDocs(path)
	if err == nil {
		t.Error("expected error for invalid YAML, got nil")
	}
}

func TestReadMkDocs_EmptyFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "mkdocs.yml")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	data, err := ReadMkDocs(path)
	if err != nil {
		t.Fatalf("ReadMkDocs() error: %v", err)
	}
	if data == nil {
		t.Error("expected non-nil map for empty file")
	}
}

func TestNavToYAML_Leaves(t *testing.T) {
	t.Parallel()
	entries := []NavEntry{
		{Title: "Home", Path: "index.md"},
		{Title: "About", Path: "about.md"},
	}

	result := NavToYAML(entries)
	if len(result) != 2 {
		t.Fatalf("got %d entries, want 2", len(result))
	}

	first, ok := result[0].(map[string]any)
	if !ok {
		t.Fatal("first entry is not a map")
	}
	if first["Home"] != "index.md" {
		t.Errorf("first entry = %v, want Home: index.md", first)
	}
}

func TestNavToYAML_Groups(t *testing.T) {
	t.Parallel()
	entries := []NavEntry{
		{Title: "Home", Path: "index.md"},
		{Title: "RFCs", Children: []NavEntry{
			{Title: "Overview", Path: "rfc/README.md"},
			{Title: "RFC-0001: Test", Path: "rfc/0001-test.md"},
		}},
	}

	result := NavToYAML(entries)
	if len(result) != 2 {
		t.Fatalf("got %d entries, want 2", len(result))
	}

	group, ok := result[1].(map[string]any)
	if !ok {
		t.Fatal("second entry is not a map")
	}
	children, ok := group["RFCs"].([]any)
	if !ok {
		t.Fatal("RFCs value is not a slice")
	}
	if len(children) != 2 {
		t.Fatalf("RFCs has %d children, want 2", len(children))
	}
}

func TestExistingNavOrder(t *testing.T) {
	t.Parallel()
	data := map[string]any{
		"site_name": "Test",
		"nav": []any{
			map[string]any{"Home": "index.md"},
			map[string]any{"RFCs": []any{}},
			map[string]any{"ADRs": []any{}},
		},
	}

	order := ExistingNavOrder(data)
	want := []string{"Home", "RFCs", "ADRs"}
	if len(order) != len(want) {
		t.Fatalf("got %d titles, want %d", len(order), len(want))
	}
	for i, got := range order {
		if got != want[i] {
			t.Errorf("order[%d] = %q, want %q", i, got, want[i])
		}
	}
}

func TestExistingNavOrder_NoNav(t *testing.T) {
	t.Parallel()
	data := map[string]any{"site_name": "Test"}
	order := ExistingNavOrder(data)
	if order != nil {
		t.Errorf("expected nil, got %v", order)
	}
}

func TestMergeNavOrder_PreservesExisting(t *testing.T) {
	t.Parallel()
	existing := []string{"Home", "RFCs", "ADRs"}
	newEntries := []NavEntry{
		{Title: "ADRs"},
		{Title: "Home", Path: "index.md"},
		{Title: "Design"},
		{Title: "RFCs"},
	}

	result := MergeNavOrder(existing, newEntries)
	if len(result) != 4 {
		t.Fatalf("got %d entries, want 4", len(result))
	}
	if result[0].Title != "Home" {
		t.Errorf("result[0] = %q, want Home", result[0].Title)
	}
	if result[1].Title != "RFCs" {
		t.Errorf("result[1] = %q, want RFCs", result[1].Title)
	}
	if result[2].Title != "ADRs" {
		t.Errorf("result[2] = %q, want ADRs", result[2].Title)
	}
	if result[3].Title != "Design" {
		t.Errorf("result[3] = %q, want Design (new, appended)", result[3].Title)
	}
}

func TestMergeNavOrder_EmptyExisting(t *testing.T) {
	t.Parallel()
	entries := []NavEntry{{Title: "B"}, {Title: "A"}}
	result := MergeNavOrder(nil, entries)
	// Should return entries unchanged (no reordering by MergeNavOrder itself).
	if len(result) != 2 {
		t.Fatalf("got %d entries, want 2", len(result))
	}
}

func TestMergeNavOrder_NewSectionsSorted(t *testing.T) {
	t.Parallel()
	existing := []string{"Home"}
	newEntries := []NavEntry{
		{Title: "Home", Path: "index.md"},
		{Title: "Zebra"},
		{Title: "Alpha"},
		{Title: "Middle"},
	}

	result := MergeNavOrder(existing, newEntries)
	if len(result) != 4 {
		t.Fatalf("got %d entries, want 4", len(result))
	}
	if result[0].Title != "Home" {
		t.Errorf("result[0] = %q, want Home", result[0].Title)
	}
	if result[1].Title != "Alpha" {
		t.Errorf("result[1] = %q, want Alpha", result[1].Title)
	}
	if result[2].Title != "Middle" {
		t.Errorf("result[2] = %q, want Middle", result[2].Title)
	}
	if result[3].Title != "Zebra" {
		t.Errorf("result[3] = %q, want Zebra", result[3].Title)
	}
}
