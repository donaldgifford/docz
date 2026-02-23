package index

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/internal/document"
)

func TestScanDocuments_Empty(t *testing.T) {
	dir := t.TempDir()
	docs, err := ScanDocuments(dir)
	if err != nil {
		t.Fatalf("ScanDocuments() error: %v", err)
	}
	if len(docs) != 0 {
		t.Errorf("expected 0 documents, got %d", len(docs))
	}
}

func TestScanDocuments_NonexistentDir(t *testing.T) {
	docs, err := ScanDocuments("/nonexistent/path")
	if err != nil {
		t.Fatalf("ScanDocuments() error: %v", err)
	}
	if docs != nil {
		t.Errorf("expected nil, got %v", docs)
	}
}

func TestScanDocuments_WithDocuments(t *testing.T) {
	dir := t.TempDir()

	writeDoc(t, dir, "0001-first.md", "RFC-0001", "First Doc", "Draft", "2026-01-01")
	writeDoc(t, dir, "0003-third.md", "RFC-0003", "Third Doc", "Accepted", "2026-03-01")
	writeDoc(t, dir, "0002-second.md", "RFC-0002", "Second Doc", "Proposed", "2026-02-01")

	docs, err := ScanDocuments(dir)
	if err != nil {
		t.Fatalf("ScanDocuments() error: %v", err)
	}
	if len(docs) != 3 {
		t.Fatalf("expected 3 documents, got %d", len(docs))
	}

	// Should be sorted by ID.
	if docs[0].ID != "RFC-0001" {
		t.Errorf("docs[0].ID = %q, want %q", docs[0].ID, "RFC-0001")
	}
	if docs[1].ID != "RFC-0002" {
		t.Errorf("docs[1].ID = %q, want %q", docs[1].ID, "RFC-0002")
	}
	if docs[2].ID != "RFC-0003" {
		t.Errorf("docs[2].ID = %q, want %q", docs[2].ID, "RFC-0003")
	}
}

func TestScanDocuments_SkipsNoFrontmatter(t *testing.T) {
	dir := t.TempDir()

	writeDoc(t, dir, "0001-valid.md", "RFC-0001", "Valid", "Draft", "2026-01-01")

	// Write a file without frontmatter.
	if err := os.WriteFile(filepath.Join(dir, "0002-no-fm.md"), []byte("# No frontmatter\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	docs, err := ScanDocuments(dir)
	if err != nil {
		t.Fatalf("ScanDocuments() error: %v", err)
	}
	if len(docs) != 1 {
		t.Errorf("expected 1 document (skip no frontmatter), got %d", len(docs))
	}
}

func TestScanDocuments_SkipsNonMatchingFiles(t *testing.T) {
	dir := t.TempDir()

	writeDoc(t, dir, "0001-valid.md", "RFC-0001", "Valid", "Draft", "2026-01-01")

	// Non-matching files.
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# README"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "template.md"), []byte("# Template"), 0o644); err != nil {
		t.Fatal(err)
	}

	docs, err := ScanDocuments(dir)
	if err != nil {
		t.Fatalf("ScanDocuments() error: %v", err)
	}
	if len(docs) != 1 {
		t.Errorf("expected 1 document, got %d", len(docs))
	}
}

func TestGenerateTable_Empty(t *testing.T) {
	result := GenerateTable(nil, "All RFCs")
	if !strings.Contains(result, "## All RFCs") {
		t.Error("missing heading")
	}
	if !strings.Contains(result, "| ID |") {
		t.Error("missing table header")
	}
	// Should only have heading + header + separator, no data rows.
	lines := strings.Split(strings.TrimSpace(result), "\n")
	if len(lines) != 4 { // heading, blank, header, separator
		t.Errorf("expected 4 lines, got %d: %v", len(lines), lines)
	}
}

func TestGenerateTable_WithDocs(t *testing.T) {
	docs := []DocEntry{
		{Frontmatter: makeFM("RFC-0001", "First", "Draft", "Author", "2026-01-01"), Filename: "0001-first.md"},
		{Frontmatter: makeFM("RFC-0002", "Second", "Accepted", "Author", "2026-02-01"), Filename: "0002-second.md"},
	}

	result := GenerateTable(docs, "All RFCs")
	if !strings.Contains(result, "| RFC-0001 |") {
		t.Error("missing RFC-0001 row")
	}
	if !strings.Contains(result, "[0002-second.md](0002-second.md)") {
		t.Error("missing link for second doc")
	}
}

func TestUpdateReadme_WithMarkers(t *testing.T) {
	dir := t.TempDir()
	readmePath := filepath.Join(dir, "README.md")

	existing := "# My Custom Header\n\nSome custom content.\n\n" +
		"<!-- BEGIN DOCZ AUTO-GENERATED -->\nold content\n<!-- END DOCZ AUTO-GENERATED -->\n"
	if err := os.WriteFile(readmePath, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	table := "| ID | new data |\n"
	msg, err := UpdateReadme(readmePath, "rfc", table)
	if err != nil {
		t.Fatalf("UpdateReadme() error: %v", err)
	}
	if !strings.Contains(msg, "Updated") {
		t.Errorf("expected 'Updated' message, got %q", msg)
	}

	content, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatal(err)
	}
	contentStr := string(content)

	// Custom header preserved.
	if !strings.Contains(contentStr, "# My Custom Header") {
		t.Error("custom header was not preserved")
	}
	if !strings.Contains(contentStr, "Some custom content.") {
		t.Error("custom content was not preserved")
	}
	// New table present.
	if !strings.Contains(contentStr, "| ID | new data |") {
		t.Error("new table content missing")
	}
	// Old content gone.
	if strings.Contains(contentStr, "old content") {
		t.Error("old content was not replaced")
	}
}

func TestUpdateReadme_NoMarkers(t *testing.T) {
	dir := t.TempDir()
	readmePath := filepath.Join(dir, "README.md")

	existing := "# Manual README\n\nNo markers here.\n"
	if err := os.WriteFile(readmePath, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	msg, err := UpdateReadme(readmePath, "rfc", "table content")
	if err != nil {
		t.Fatalf("UpdateReadme() error: %v", err)
	}
	if !strings.Contains(msg, "Warning") {
		t.Errorf("expected warning message, got %q", msg)
	}

	// File should not be modified.
	content, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != existing {
		t.Error("file was modified despite having no markers")
	}
}

func TestUpdateReadme_NewFile(t *testing.T) {
	dir := t.TempDir()
	readmePath := filepath.Join(dir, "subdir", "README.md")

	msg, err := UpdateReadme(readmePath, "rfc", "| table |\n")
	if err != nil {
		t.Fatalf("UpdateReadme() error: %v", err)
	}
	if !strings.Contains(msg, "Created") {
		t.Errorf("expected 'Created' message, got %q", msg)
	}

	content, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatal(err)
	}
	contentStr := string(content)

	if !strings.Contains(contentStr, "<!-- BEGIN DOCZ AUTO-GENERATED -->") {
		t.Error("missing begin marker")
	}
	if !strings.Contains(contentStr, "| table |") {
		t.Error("missing table content")
	}
}

func writeDoc(t *testing.T, dir, filename, id, title, status, created string) {
	t.Helper()
	content := "---\n" +
		"id: " + id + "\n" +
		"title: \"" + title + "\"\n" +
		"status: " + status + "\n" +
		"author: Author\n" +
		"created: " + created + "\n" +
		"---\n\n# Body\n"
	if err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func makeFM(id, title, status, author, created string) document.Frontmatter {
	return document.Frontmatter{
		ID:      id,
		Title:   title,
		Status:  status,
		Author:  author,
		Created: created,
	}
}
