package wiki

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadWriteMkDocs_RoundTrip(t *testing.T) {
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
	_, err := ReadMkDocs("/nonexistent/mkdocs.yml")
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
}

func TestReadMkDocs_InvalidYAML(t *testing.T) {
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
	data := map[string]any{"site_name": "Test"}
	order := ExistingNavOrder(data)
	if order != nil {
		t.Errorf("expected nil, got %v", order)
	}
}

func TestMergeNavOrder_PreservesExisting(t *testing.T) {
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
	entries := []NavEntry{{Title: "B"}, {Title: "A"}}
	result := MergeNavOrder(nil, entries)
	// Should return entries unchanged (no reordering by MergeNavOrder itself).
	if len(result) != 2 {
		t.Fatalf("got %d entries, want 2", len(result))
	}
}

func TestMergeNavOrder_NewSectionsSorted(t *testing.T) {
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
