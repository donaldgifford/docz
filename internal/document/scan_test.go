package document

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

// TestScanDocuments_PopulatesContent guards the IMPL-0007 Phase 2
// contract: every returned DocEntry carries the raw file bytes so
// downstream callers (cmd/update.go ToC pass) can skip re-reading.
func TestScanDocuments_PopulatesContent(t *testing.T) {
	dir := t.TempDir()
	writeDoc(t, dir, "0001-first.md", "RFC-0001", "First", "Draft", "2026-01-01")

	docs, err := ScanDocuments(dir)
	if err != nil {
		t.Fatalf("ScanDocuments: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("got %d docs, want 1", len(docs))
	}
	if len(docs[0].Content) == 0 {
		t.Fatal("DocEntry.Content is empty; should hold file bytes")
	}

	want, err := os.ReadFile(filepath.Join(dir, "0001-first.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(docs[0].Content, want) {
		t.Errorf("DocEntry.Content diverges from on-disk bytes")
	}
}

func TestScanDocuments_SkipsNoFrontmatter(t *testing.T) {
	dir := t.TempDir()

	writeDoc(t, dir, "0001-valid.md", "RFC-0001", "Valid", "Draft", "2026-01-01")

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

// generateBenchDocs writes n synthetic docz documents into dir. Bodies
// are deliberately non-trivial (~2KB of markdown each) so the scan cost
// reflects a realistic large-repo profile, not a degenerate
// frontmatter-only case.
func generateBenchDocs(b *testing.B, dir string, n int) {
	b.Helper()
	body := strings.Repeat(
		"## Section\n\nLorem ipsum dolor sit amet, consectetur adipiscing elit. "+
			"Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.\n\n"+
			"### Subsection\n\nUt enim ad minim veniam, quis nostrud exercitation "+
			"ullamco laboris nisi ut aliquip ex ea commodo consequat.\n\n",
		8,
	)
	for i := 1; i <= n; i++ {
		name := fmt.Sprintf("%04d-bench-doc-%d.md", i, i)
		content := fmt.Sprintf(
			"---\nid: RFC-%04d\ntitle: \"Bench Doc %d\"\nstatus: Draft\n"+
				"author: Bench\ncreated: 2026-01-01\n---\n\n# Bench Doc %d\n\n%s",
			i, i, i, body,
		)
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkScanDocuments measures ScanDocuments cost across 100/500/1000
// realistic docs. Recorded as the Phase 1 baseline for IMPL-0007 so
// later phases can prove they did not regress the scan path.
func BenchmarkScanDocuments(b *testing.B) {
	for _, n := range []int{100, 500, 1000} {
		b.Run(fmt.Sprintf("%d", n), func(b *testing.B) {
			dir := b.TempDir()
			generateBenchDocs(b, dir, n)
			b.ResetTimer()
			b.ReportAllocs()
			for b.Loop() {
				docs, err := ScanDocuments(dir)
				if err != nil {
					b.Fatal(err)
				}
				if len(docs) != n {
					b.Fatalf("got %d docs, want %d", len(docs), n)
				}
			}
		})
	}
}
