package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/donaldgifford/docz/pkg/doczcore/config"
)

// installListRunner wires up the package-level runner for the list
// tests. Each test gets its own bytes.Buffer Out so output capture is
// race-safe — no os.Pipe redirection.
func installListRunner(t *testing.T, dir string, out io.Writer) {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.DocsDir = filepath.Join(dir, "docs")
	appCfg = cfg
	runner = &Runner{
		Cfg:      cfg,
		Out:      out,
		Err:      io.Discard,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now:      time.Now,
		Git:      staticGit{},
		RepoRoot: dir,
	}
	t.Cleanup(func() { runner = nil })
}

func setupListTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	rfcDir := filepath.Join(dir, "docs", "rfc")
	adrDir := filepath.Join(dir, "docs", "adr")
	if err := os.MkdirAll(rfcDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(adrDir, 0o750); err != nil {
		t.Fatal(err)
	}

	writeTestDoc(t, rfcDir, "0001-first-rfc.md", "RFC-0001", "First RFC", "Draft", "Author A", "2026-01-01")
	writeTestDoc(t, rfcDir, "0002-second-rfc.md", "RFC-0002", "Second RFC", "Accepted", "Author B", "2026-01-15")
	writeTestDoc(t, adrDir, "0001-first-adr.md", "ADR-0001", "First ADR", "Proposed", "Author A", "2026-02-01")

	return dir
}

func writeTestDoc(t *testing.T, dir, filename, id, title, status, author, created string) {
	t.Helper()
	content := "---\n" +
		"id: " + id + "\n" +
		"title: \"" + title + "\"\n" +
		"status: " + status + "\n" +
		"author: " + author + "\n" +
		"created: " + created + "\n" +
		"---\n\n# Body\n"
	if err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFilterByStatus(t *testing.T) {
	t.Parallel()
	entries := []listEntry{
		{Status: "Draft"},
		{Status: "Accepted"},
		{Status: "draft"},
		{Status: "Rejected"},
	}

	filtered := filterByStatus(entries, "draft")
	if len(filtered) != 2 {
		t.Errorf("expected 2 entries with status 'draft', got %d", len(filtered))
	}

	filtered = filterByStatus(entries, "ACCEPTED")
	if len(filtered) != 1 {
		t.Errorf("expected 1 entry with status 'ACCEPTED', got %d", len(filtered))
	}

	filtered = filterByStatus(entries, "nonexistent")
	if len(filtered) != 0 {
		t.Errorf("expected 0 entries with status 'nonexistent', got %d", len(filtered))
	}
}

func TestOutputTable(t *testing.T) {
	t.Parallel()
	entries := []listEntry{
		{ID: "RFC-0001", Title: "First", Status: "Draft", Date: "2026-01-01", Author: "Author", Type: "RFC"},
		{ID: "RFC-0002", Title: "Second", Status: "Accepted", Date: "2026-02-01", Author: "Author", Type: "RFC"},
	}

	var buf bytes.Buffer
	if err := outputTable(&buf, entries); err != nil {
		t.Fatalf("outputTable() error: %v", err)
	}
	output := buf.String()

	if !strings.Contains(output, "RFC-0001") {
		t.Error("missing RFC-0001 in table output")
	}
	if !strings.Contains(output, "RFC-0002") {
		t.Error("missing RFC-0002 in table output")
	}
	if !strings.Contains(output, "ID") {
		t.Error("missing header in table output")
	}
}

func TestOutputJSON(t *testing.T) {
	t.Parallel()
	entries := []listEntry{
		{ID: "RFC-0001", Title: "First", Status: "Draft", Date: "2026-01-01", Author: "Author", Type: "RFC", File: "0001-first.md"},
	}

	var buf bytes.Buffer
	if err := outputJSON(&buf, entries); err != nil {
		t.Fatalf("outputJSON() error: %v", err)
	}

	var result []listEntry
	if jsonErr := json.Unmarshal(buf.Bytes(), &result); jsonErr != nil {
		t.Fatalf("invalid JSON output: %v", jsonErr)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 entry, got %d", len(result))
	}
	if result[0].ID != "RFC-0001" {
		t.Errorf("ID = %q, want %q", result[0].ID, "RFC-0001")
	}
}

func TestOutputCSV(t *testing.T) {
	t.Parallel()
	entries := []listEntry{
		{ID: "RFC-0001", Title: "First", Status: "Draft", Date: "2026-01-01", Author: "Author", Type: "RFC", File: "0001-first.md"},
	}

	var buf bytes.Buffer
	if err := outputCSV(&buf, entries); err != nil {
		t.Fatalf("outputCSV() error: %v", err)
	}
	output := buf.String()

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines (header + 1 row), got %d", len(lines))
	}
	if !strings.HasPrefix(lines[0], "ID,") {
		t.Errorf("CSV header = %q, want prefix 'ID,'", lines[0])
	}
	if !strings.Contains(lines[1], "RFC-0001") {
		t.Error("missing RFC-0001 in CSV output")
	}
}

func TestRunList_AllTypes(t *testing.T) {
	dir := setupListTestDir(t)
	installListRunner(t, dir, io.Discard)

	listStatus = ""
	listFormat = "table"
	if err := runList(nil, nil); err != nil {
		t.Fatalf("runList() error: %v", err)
	}
}

func TestRunList_FilterByType(t *testing.T) {
	dir := setupListTestDir(t)
	var out bytes.Buffer
	installListRunner(t, dir, &out)

	listStatus = ""
	listFormat = formatJSON
	if err := runList(nil, []string{"rfc"}); err != nil {
		t.Fatalf("runList() error: %v", err)
	}

	var result []listEntry
	if jsonErr := json.Unmarshal(out.Bytes(), &result); jsonErr != nil {
		t.Fatalf("invalid JSON: %v", jsonErr)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 RFC entries, got %d", len(result))
	}
}

func TestRunList_FilterByStatus(t *testing.T) {
	dir := setupListTestDir(t)
	var out bytes.Buffer
	installListRunner(t, dir, &out)

	listStatus = "draft"
	listFormat = formatJSON
	if err := runList(nil, nil); err != nil {
		t.Fatalf("runList() error: %v", err)
	}

	var result []listEntry
	if jsonErr := json.Unmarshal(out.Bytes(), &result); jsonErr != nil {
		t.Fatalf("invalid JSON: %v", jsonErr)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 draft entry, got %d", len(result))
	}
}

func TestRunList_InvalidType(t *testing.T) {
	installListRunner(t, t.TempDir(), io.Discard)
	listStatus = ""
	listFormat = "table"
	err := runList(nil, []string{"badtype"})
	if err == nil {
		t.Error("expected error for invalid type, got nil")
	}
}
