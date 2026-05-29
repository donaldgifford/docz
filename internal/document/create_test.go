package document

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fixedTime is the deterministic timestamp shared by every test that
// asserts on the rendered `created:` frontmatter field.
var fixedTime = time.Date(2026, 2, 22, 0, 0, 0, 0, time.UTC)

func TestCreate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	docsDir := filepath.Join(dir, "docs")
	if err := os.MkdirAll(filepath.Join(docsDir, "rfc"), 0o755); err != nil {
		t.Fatal(err)
	}

	opts := CreateOptions{
		Type:      "rfc",
		Title:     "API Rate Limiting",
		Author:    "Test Author",
		Status:    "Draft",
		Prefix:    "RFC",
		IDWidth:   4,
		DocsDir:   docsDir,
		TypeDir:   "rfc",
		CreatedAt: fixedTime,
	}

	result, err := Create(&opts)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if result.Number != "0001" {
		t.Errorf("Number = %q, want %q", result.Number, "0001")
	}
	if result.Filename != "0001-api-rate-limiting.md" {
		t.Errorf("Filename = %q, want %q", result.Filename, "0001-api-rate-limiting.md")
	}

	content, err := os.ReadFile(result.FilePath)
	if err != nil {
		t.Fatalf("reading created file: %v", err)
	}

	contentStr := string(content)
	// Verify frontmatter fields.
	if !strings.Contains(contentStr, "id: RFC-0001") {
		t.Error("missing expected frontmatter id")
	}
	if !strings.Contains(contentStr, `title: "API Rate Limiting"`) {
		t.Error("missing expected frontmatter title")
	}
	if !strings.Contains(contentStr, "status: Draft") {
		t.Error("missing expected frontmatter status")
	}
	if !strings.Contains(contentStr, "author: Test Author") {
		t.Error("missing expected frontmatter author")
	}
	if !strings.Contains(contentStr, "created: 2026-02-22") {
		t.Error("missing expected frontmatter date")
	}
}

func TestCreate_AutoIncrement(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	docsDir := filepath.Join(dir, "docs")
	adrDir := filepath.Join(docsDir, "adr")
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a dummy existing file to simulate existing document.
	if err := os.WriteFile(filepath.Join(adrDir, "0001-first.md"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := CreateOptions{
		Type:      "adr",
		Title:     "Second Decision",
		Author:    "Author",
		Status:    "Proposed",
		Prefix:    "ADR",
		IDWidth:   4,
		DocsDir:   docsDir,
		TypeDir:   "adr",
		CreatedAt: fixedTime,
	}

	result, err := Create(&opts)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if result.Number != "0002" {
		t.Errorf("Number = %q, want %q", result.Number, "0002")
	}
}

func TestCreate_DuplicateFilename(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	docsDir := filepath.Join(dir, "docs")

	opts := CreateOptions{
		Type:      "rfc",
		Title:     "Duplicate Test",
		Author:    "Author",
		Status:    "Draft",
		Prefix:    "RFC",
		IDWidth:   4,
		DocsDir:   docsDir,
		TypeDir:   "rfc",
		CreatedAt: fixedTime,
	}

	// First create should succeed.
	if _, err := Create(&opts); err != nil {
		t.Fatalf("first Create() error: %v", err)
	}

	// Second create should get next ID (not duplicate).
	result, err := Create(&opts)
	if err != nil {
		t.Fatalf("second Create() error: %v", err)
	}
	if result.Number != "0002" {
		t.Errorf("second doc Number = %q, want %q", result.Number, "0002")
	}
}

func TestCreate_CreatesDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Don't pre-create the docs/design dir - Create should make it.
	opts := CreateOptions{
		Type:      "design",
		Title:     "New Design",
		Author:    "Author",
		Status:    "Draft",
		Prefix:    "DESIGN",
		IDWidth:   4,
		DocsDir:   filepath.Join(dir, "docs"),
		TypeDir:   "design",
		CreatedAt: fixedTime,
	}

	result, err := Create(&opts)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if result.Filename != "0001-new-design.md" {
		t.Errorf("Filename = %q, want %q", result.Filename, "0001-new-design.md")
	}
}

// TestCreate_ZeroCreatedAtFallsBackToNow asserts that a CreateOptions
// with CreatedAt left zero still produces a valid Date — Create
// substitutes time.Now() so callers without a time source keep
// working.
func TestCreate_ZeroCreatedAtFallsBackToNow(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	docsDir := filepath.Join(dir, "docs")
	if err := os.MkdirAll(filepath.Join(docsDir, "rfc"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Compare in local time because Create formats with time.Now() (the
	// system local clock), not UTC. A previous version of this test used
	// time.Now().UTC().Truncate(24h) which races at the local-vs-UTC
	// day boundary — when the local date and UTC date differ, the file
	// shows the local date but the test expects the UTC one.
	wantDate := time.Now().Format(time.DateOnly)
	opts := CreateOptions{
		Type:    "rfc",
		Title:   "Zero Time",
		Author:  "Author",
		Status:  "Draft",
		Prefix:  "RFC",
		IDWidth: 4,
		DocsDir: docsDir,
		TypeDir: "rfc",
		// CreatedAt is intentionally the zero Time.
	}

	result, err := Create(&opts)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	content, err := os.ReadFile(result.FilePath)
	if err != nil {
		t.Fatal(err)
	}
	contentStr := string(content)
	if !strings.Contains(contentStr, "created: "+wantDate) {
		t.Errorf("frontmatter date should be %q (today), got content:\n%s", wantDate, contentStr)
	}
}

func TestNextID_NonexistentDir(t *testing.T) {
	t.Parallel()
	id := nextID("/nonexistent/path")
	if id != 1 {
		t.Errorf("nextID() = %d, want 1 for nonexistent dir", id)
	}
}

func TestNextID_EmptyDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	id := nextID(dir)
	if id != 1 {
		t.Errorf("nextID() = %d, want 1", id)
	}
}

func TestNextID_NonSequential(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Create files with non-sequential IDs.
	for _, name := range []string{"0001-first.md", "0005-fifth.md", "0003-third.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("test"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	id := nextID(dir)
	if id != 6 {
		t.Errorf("nextID() = %d, want 6", id)
	}
}
