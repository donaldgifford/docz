package index

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/pkg/doczcore/config"
	"github.com/donaldgifford/docz/pkg/doczcore/document"
)

// indexHeader is a stand-in for the caller-resolved header string that
// UpdateReadme/DryRunReadme now splice above the table (IMPL-0012 Phase 2).
// The index package no longer resolves headers itself, so any string works.
const indexHeader = "# Test Index Header\n\n"

func TestGenerateTable_Empty(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	dir := t.TempDir()
	readmePath := filepath.Join(dir, "README.md")

	existing := "# My Custom Header\n\nSome custom content.\n\n" +
		"<!-- BEGIN DOCZ AUTO-GENERATED -->\nold content\n<!-- END DOCZ AUTO-GENERATED -->\n"
	if err := os.WriteFile(readmePath, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	table := "| ID | new data |\n"
	outcome, err := UpdateReadme(readmePath, indexHeader, table)
	if err != nil {
		t.Fatalf("UpdateReadme() error: %v", err)
	}
	if outcome.Action != ActionUpdated {
		t.Errorf("Action = %v, want ActionUpdated", outcome.Action)
	}
	if outcome.Path != readmePath {
		t.Errorf("Path = %q, want %q", outcome.Path, readmePath)
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
	t.Parallel()
	dir := t.TempDir()
	readmePath := filepath.Join(dir, "README.md")

	existing := "# Manual README\n\nNo markers here.\n"
	if err := os.WriteFile(readmePath, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	outcome, err := UpdateReadme(readmePath, indexHeader, "table content")
	if err != nil {
		t.Fatalf("UpdateReadme() error: %v", err)
	}
	if outcome.Action != ActionNoMarkers {
		t.Errorf("Action = %v, want ActionNoMarkers", outcome.Action)
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
	t.Parallel()
	dir := t.TempDir()
	readmePath := filepath.Join(dir, "subdir", "README.md")

	outcome, err := UpdateReadme(readmePath, indexHeader, "| table |\n")
	if err != nil {
		t.Fatalf("UpdateReadme() error: %v", err)
	}
	if outcome.Action != ActionCreated {
		t.Errorf("Action = %v, want ActionCreated", outcome.Action)
	}

	content, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatal(err)
	}
	contentStr := string(content)

	// The caller-provided header is spliced verbatim above the markers.
	if !strings.HasPrefix(contentStr, indexHeader) {
		t.Errorf("content does not start with the provided header verbatim:\n%q", contentStr)
	}
	if !strings.Contains(contentStr, "<!-- BEGIN DOCZ AUTO-GENERATED -->") {
		t.Error("missing begin marker")
	}
	if !strings.Contains(contentStr, "| table |") {
		t.Error("missing table content")
	}
}

func TestDryRunReadme_WithMarkers(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	readmePath := filepath.Join(dir, "README.md")

	existing := "# Header\n\n" +
		"<!-- BEGIN DOCZ AUTO-GENERATED -->\nold content\n<!-- END DOCZ AUTO-GENERATED -->\n"
	if err := os.WriteFile(readmePath, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	outcome, err := DryRunReadme(readmePath, indexHeader, "| new data |\n")
	if err != nil {
		t.Fatalf("DryRunReadme() error: %v", err)
	}
	if outcome.Action != ActionDryRunUpdated {
		t.Errorf("Action = %v, want ActionDryRunUpdated", outcome.Action)
	}
	if !strings.Contains(outcome.Body, "| new data |") {
		t.Error("Body should contain new data")
	}
	if strings.Contains(outcome.Body, "old content") {
		t.Error("Body should not contain old content")
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
	t.Parallel()
	dir := t.TempDir()
	readmePath := filepath.Join(dir, "nonexistent", "README.md")

	outcome, err := DryRunReadme(readmePath, indexHeader, "| table |\n")
	if err != nil {
		t.Fatalf("DryRunReadme() error: %v", err)
	}
	if outcome.Action != ActionDryRunCreated {
		t.Errorf("Action = %v, want ActionDryRunCreated", outcome.Action)
	}
	if !strings.HasPrefix(outcome.Body, indexHeader) {
		t.Errorf("Body does not start with the provided header verbatim:\n%q", outcome.Body)
	}
	if !strings.Contains(outcome.Body, "| table |") {
		t.Error("Body should contain table content")
	}
	if !strings.Contains(outcome.Body, "<!-- BEGIN DOCZ AUTO-GENERATED -->") {
		t.Error("Body should contain markers")
	}
}

func TestDryRunReadme_NoMarkers(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	readmePath := filepath.Join(dir, "README.md")

	existing := "# Manual README\n"
	if err := os.WriteFile(readmePath, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	outcome, err := DryRunReadme(readmePath, indexHeader, "table")
	if err != nil {
		t.Fatalf("DryRunReadme() error: %v", err)
	}
	if outcome.Action != ActionNoMarkers {
		t.Errorf("Action = %v, want ActionNoMarkers", outcome.Action)
	}
}

func makeFM(id, title, status, author, created string) document.Frontmatter {
	return document.Frontmatter{
		ID:      id,
		Title:   title,
		Status:  config.Status(status),
		Author:  author,
		Created: created,
	}
}
