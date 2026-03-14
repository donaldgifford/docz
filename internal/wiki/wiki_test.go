package wiki

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanDocs_BasicStructure(t *testing.T) {
	dir := t.TempDir()
	navTitles := map[string]string{"rfc": "RFCs", "adr": "ADRs"}

	// Create docs/index.md
	writeFile(t, dir, "index.md", "# Home\n")

	// Create docs/rfc/ with README and two docs
	mkdirAll(t, dir, "rfc")
	writeFile(t, dir, "rfc/README.md", "# RFCs\n")
	writeFile(
		t,
		dir,
		"rfc/0001-first.md",
		"---\nid: RFC-0001\ntitle: \"First RFC\"\nstatus: Draft\nauthor: Test\ncreated: 2026-01-01\n---\n# RFC 0001\n",
	)
	writeFile(
		t,
		dir,
		"rfc/0002-second.md",
		"---\nid: RFC-0002\ntitle: \"Second RFC\"\nstatus: Draft\nauthor: Test\ncreated: 2026-01-02\n---\n# RFC 0002\n",
	)

	entries, err := ScanDocs(dir, nil, navTitles)
	if err != nil {
		t.Fatalf("ScanDocs() error: %v", err)
	}

	// Should have: index.md and rfc/ group
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}

	// First should be index.md (overview at root level uses extracted title)
	if entries[0].Path != "index.md" {
		t.Errorf("entries[0].Path = %q, want %q", entries[0].Path, "index.md")
	}

	// Second should be RFCs group
	rfcGroup := entries[1]
	if rfcGroup.Title != "RFCs" {
		t.Errorf("rfcGroup.Title = %q, want %q", rfcGroup.Title, "RFCs")
	}
	if len(rfcGroup.Children) != 3 {
		t.Fatalf("RFCs has %d children, want 3", len(rfcGroup.Children))
	}
	if rfcGroup.Children[0].Title != "Overview" {
		t.Errorf("first child title = %q, want %q", rfcGroup.Children[0].Title, "Overview")
	}
	if rfcGroup.Children[1].Title != "RFC-0001: First RFC" {
		t.Errorf(
			"second child title = %q, want %q",
			rfcGroup.Children[1].Title,
			"RFC-0001: First RFC",
		)
	}
	if rfcGroup.Children[2].Title != "RFC-0002: Second RFC" {
		t.Errorf(
			"third child title = %q, want %q",
			rfcGroup.Children[2].Title,
			"RFC-0002: Second RFC",
		)
	}
}

func TestScanDocs_ExcludesDirectories(t *testing.T) {
	dir := t.TempDir()
	exclude := []string{"templates", "examples"}

	mkdirAll(t, dir, "rfc")
	writeFile(t, dir, "rfc/0001-test.md", "# Test\n")
	mkdirAll(t, dir, "templates")
	writeFile(t, dir, "templates/rfc.md", "template content\n")
	mkdirAll(t, dir, "examples")
	writeFile(t, dir, "examples/sample.md", "example content\n")

	entries, err := ScanDocs(dir, exclude, nil)
	if err != nil {
		t.Fatalf("ScanDocs() error: %v", err)
	}

	// Should only have rfc/ group, not templates/ or examples/
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].Title != "Rfc" {
		t.Errorf("entry title = %q, want %q", entries[0].Title, "Rfc")
	}
}

func TestScanDocs_SkipsEmptyDirectories(t *testing.T) {
	dir := t.TempDir()

	mkdirAll(t, dir, "empty-dir")
	mkdirAll(t, dir, "has-content")
	writeFile(t, dir, "has-content/doc.md", "# Content\n")

	entries, err := ScanDocs(dir, nil, nil)
	if err != nil {
		t.Fatalf("ScanDocs() error: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].Title != "Has Content" {
		t.Errorf("entry title = %q, want %q", entries[0].Title, "Has Content")
	}
}

func TestScanDocs_ArbitraryNesting(t *testing.T) {
	dir := t.TempDir()

	mkdirAll(t, dir, "guides/getting-started/setup")
	writeFile(t, dir, "guides/getting-started/setup/install.md", "# Install\n")
	writeFile(t, dir, "guides/getting-started/intro.md", "# Introduction\n")

	entries, err := ScanDocs(dir, nil, nil)
	if err != nil {
		t.Fatalf("ScanDocs() error: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}

	guides := entries[0]
	if guides.Title != "Guides" {
		t.Errorf("guides.Title = %q, want %q", guides.Title, "Guides")
	}
	if len(guides.Children) != 1 {
		t.Fatalf("guides has %d children, want 1", len(guides.Children))
	}

	gs := guides.Children[0]
	if gs.Title != "Getting Started" {
		t.Errorf("gs.Title = %q, want %q", gs.Title, "Getting Started")
	}
	// Should have intro.md and setup/ group
	if len(gs.Children) != 2 {
		t.Fatalf("getting-started has %d children, want 2", len(gs.Children))
	}
}

func TestScanDocs_SkipsNonMarkdown(t *testing.T) {
	dir := t.TempDir()

	mkdirAll(t, dir, "assets")
	writeFile(t, dir, "assets/diagram.png", "fake png data")
	writeFile(t, dir, "assets/notes.md", "# Notes\n")

	entries, err := ScanDocs(dir, nil, nil)
	if err != nil {
		t.Fatalf("ScanDocs() error: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if len(entries[0].Children) != 1 {
		t.Fatalf("assets has %d children, want 1", len(entries[0].Children))
	}
}

func TestScanDocs_DoczDocsSortedByID(t *testing.T) {
	dir := t.TempDir()

	mkdirAll(t, dir, "rfc")
	writeFile(t, dir, "rfc/0003-third.md", "# Third\n")
	writeFile(t, dir, "rfc/0001-first.md", "# First\n")
	writeFile(t, dir, "rfc/0002-second.md", "# Second\n")

	entries, err := ScanDocs(dir, nil, nil)
	if err != nil {
		t.Fatalf("ScanDocs() error: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}

	children := entries[0].Children
	if len(children) != 3 {
		t.Fatalf("rfc has %d children, want 3", len(children))
	}
	if children[0].Path != "rfc/0001-first.md" {
		t.Errorf("first = %q, want rfc/0001-first.md", children[0].Path)
	}
	if children[1].Path != "rfc/0002-second.md" {
		t.Errorf("second = %q, want rfc/0002-second.md", children[1].Path)
	}
	if children[2].Path != "rfc/0003-third.md" {
		t.Errorf("third = %q, want rfc/0003-third.md", children[2].Path)
	}
}

func TestSortEntries(t *testing.T) {
	entries := []NavEntry{
		{Title: "RFCs", Children: []NavEntry{{Title: "Overview"}}},
		{Title: "Home", Path: "index.md"},
		{Title: "ADRs", Children: []NavEntry{{Title: "Overview"}}},
		{Title: "Design", Children: []NavEntry{{Title: "Overview"}}},
	}

	sorted := SortEntries(entries)

	if len(sorted) != 4 {
		t.Fatalf("got %d entries, want 4", len(sorted))
	}
	if sorted[0].Title != "Home" {
		t.Errorf("sorted[0] = %q, want Home", sorted[0].Title)
	}
	if sorted[1].Title != "ADRs" {
		t.Errorf("sorted[1] = %q, want ADRs", sorted[1].Title)
	}
	if sorted[2].Title != "Design" {
		t.Errorf("sorted[2] = %q, want Design", sorted[2].Title)
	}
	if sorted[3].Title != "RFCs" {
		t.Errorf("sorted[3] = %q, want RFCs", sorted[3].Title)
	}
}

func TestSortEntries_NoHome(t *testing.T) {
	entries := []NavEntry{
		{Title: "RFCs", Children: []NavEntry{}},
		{Title: "ADRs", Children: []NavEntry{}},
	}

	sorted := SortEntries(entries)
	if len(sorted) != 2 {
		t.Fatalf("got %d entries, want 2", len(sorted))
	}
	if sorted[0].Title != "ADRs" {
		t.Errorf("sorted[0] = %q, want ADRs", sorted[0].Title)
	}
}

func TestSortEntries_Empty(t *testing.T) {
	sorted := SortEntries(nil)
	if sorted != nil {
		t.Errorf("expected nil, got %v", sorted)
	}
}

func TestCountPages(t *testing.T) {
	entries := []NavEntry{
		{Title: "Home", Path: "index.md"},
		{Title: "RFCs", Children: []NavEntry{
			{Title: "Overview", Path: "rfc/README.md"},
			{Title: "RFC-0001", Path: "rfc/0001.md"},
		}},
		{Title: "ADRs", Children: []NavEntry{
			{Title: "Overview", Path: "adr/README.md"},
		}},
	}

	got := CountPages(entries)
	if got != 4 {
		t.Errorf("CountPages() = %d, want 4", got)
	}
}

// Test helpers

func writeFile(t *testing.T, base, relPath, content string) {
	t.Helper()
	absPath := filepath.Join(base, relPath)
	if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mkdirAll(t *testing.T, base, relPath string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(base, relPath), 0o750); err != nil {
		t.Fatal(err)
	}
}
