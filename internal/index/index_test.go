package index

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/internal/document"
)

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
	docs := []document.DocEntry{
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

func TestDryRunReadme_WithMarkers(t *testing.T) {
	dir := t.TempDir()
	readmePath := filepath.Join(dir, "README.md")

	existing := "# Header\n\n" +
		"<!-- BEGIN DOCZ AUTO-GENERATED -->\nold content\n<!-- END DOCZ AUTO-GENERATED -->\n"
	if err := os.WriteFile(readmePath, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := DryRunReadme(readmePath, "rfc", "| new data |\n")
	if err != nil {
		t.Fatalf("DryRunReadme() error: %v", err)
	}

	if !strings.Contains(result, "| new data |") {
		t.Error("dry run result should contain new data")
	}
	if strings.Contains(result, "old content") {
		t.Error("dry run result should not contain old content")
	}

	// Verify file was NOT modified.
	content, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "old content") {
		t.Error("file should not be modified during dry run")
	}
}

func TestDryRunReadme_NoFile(t *testing.T) {
	dir := t.TempDir()
	readmePath := filepath.Join(dir, "nonexistent", "README.md")

	result, err := DryRunReadme(readmePath, "rfc", "| table |\n")
	if err != nil {
		t.Fatalf("DryRunReadme() error: %v", err)
	}

	if !strings.Contains(result, "| table |") {
		t.Error("dry run result should contain table content")
	}
	if !strings.Contains(result, "<!-- BEGIN DOCZ AUTO-GENERATED -->") {
		t.Error("dry run result should contain markers")
	}
}

func TestDryRunReadme_NoMarkers(t *testing.T) {
	dir := t.TempDir()
	readmePath := filepath.Join(dir, "README.md")

	existing := "# Manual README\n"
	if err := os.WriteFile(readmePath, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := DryRunReadme(readmePath, "rfc", "table")
	if err != nil {
		t.Fatalf("DryRunReadme() error: %v", err)
	}

	if !strings.Contains(result, "Warning") {
		t.Errorf("expected warning for no markers, got %q", result)
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
