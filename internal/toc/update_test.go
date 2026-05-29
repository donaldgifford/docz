package toc

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

const docWithMarkers = `# Title

<!--toc:start-->
<!--toc:end-->

## Section One

Body.

## Section Two

Body.

## Section Three

Body.
`

const docWithoutMarkers = `# Title

Body without markers.

## Section
`

func writeDocFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestUpdateFiles_DryRun(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := writeDocFile(t, dir, "doc.md", docWithMarkers)
	originalBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	report, err := UpdateFiles(
		[]FileInput{{Path: path, Content: originalBytes}},
		1, true,
	)
	if err != nil {
		t.Fatalf("UpdateFiles() error: %v", err)
	}

	if len(report.WouldUpdate) != 1 {
		t.Fatalf("WouldUpdate len = %d, want 1", len(report.WouldUpdate))
	}
	if got := report.WouldUpdate[0].Path; got != path {
		t.Errorf("WouldUpdate[0].Path = %q, want %q", got, path)
	}
	if report.WouldUpdate[0].Headings != 3 {
		t.Errorf("WouldUpdate[0].Headings = %d, want 3", report.WouldUpdate[0].Headings)
	}
	if len(report.Updated)+len(report.Unchanged)+len(report.Skipped)+len(report.WriteErrors) != 0 {
		t.Errorf("expected all other buckets empty, got report=%+v", report)
	}

	// File on disk must be untouched.
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != docWithMarkers {
		t.Error("dry-run modified the file on disk")
	}
}

func TestUpdateFiles_RealUpdate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := writeDocFile(t, dir, "doc.md", docWithMarkers)

	report, err := UpdateFiles(
		[]FileInput{{Path: path, Content: []byte(docWithMarkers)}},
		1, false,
	)
	if err != nil {
		t.Fatalf("UpdateFiles() error: %v", err)
	}
	if len(report.Updated) != 1 {
		t.Fatalf("Updated len = %d, want 1", len(report.Updated))
	}
	if report.Updated[0].Path != path {
		t.Errorf("Updated[0].Path = %q, want %q", report.Updated[0].Path, path)
	}
	if report.Updated[0].Headings != 3 {
		t.Errorf("Updated[0].Headings = %d, want 3", report.Updated[0].Headings)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) == docWithMarkers {
		t.Error("real update did not modify the file on disk")
	}
}

func TestUpdateFiles_NoMarkersSkipped(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := writeDocFile(t, dir, "doc.md", docWithoutMarkers)

	report, err := UpdateFiles(
		[]FileInput{{Path: path, Content: []byte(docWithoutMarkers)}},
		1, false,
	)
	if err != nil {
		t.Fatalf("UpdateFiles() error: %v", err)
	}
	if len(report.Skipped) != 1 {
		t.Fatalf("Skipped len = %d, want 1", len(report.Skipped))
	}
	if report.Skipped[0] != path {
		t.Errorf("Skipped[0] = %q, want %q", report.Skipped[0], path)
	}
	if len(report.Updated)+len(report.WouldUpdate) != 0 {
		t.Error("expected no Updated/WouldUpdate when markers are absent")
	}

	// File untouched.
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != docWithoutMarkers {
		t.Error("file content changed even though markers were absent")
	}
}

func TestUpdateFiles_IdempotentRerun(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := writeDocFile(t, dir, "doc.md", docWithMarkers)

	// First pass: real update.
	first, err := UpdateFiles(
		[]FileInput{{Path: path, Content: []byte(docWithMarkers)}},
		1, false,
	)
	if err != nil {
		t.Fatalf("first UpdateFiles() error: %v", err)
	}
	if len(first.Updated) != 1 {
		t.Fatalf("first.Updated len = %d, want 1", len(first.Updated))
	}

	// Re-read the file and run again.
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := UpdateFiles(
		[]FileInput{{Path: path, Content: after}},
		1, false,
	)
	if err != nil {
		t.Fatalf("second UpdateFiles() error: %v", err)
	}
	if len(second.Unchanged) != 1 {
		t.Fatalf("second.Unchanged len = %d, want 1; report=%+v", len(second.Unchanged), second)
	}
	if len(second.Updated) != 0 {
		t.Errorf("second.Updated len = %d, want 0", len(second.Updated))
	}
}

func TestUpdateFiles_WriteErrorIsNonFatal(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	goodPath := writeDocFile(t, dir, "good.md", docWithMarkers)

	// Point one input at a path under a non-existent directory so
	// os.WriteFile fails. The other input should still be processed.
	badPath := filepath.Join(dir, "nope", "doc.md")

	report, err := UpdateFiles(
		[]FileInput{
			{Path: badPath, Content: []byte(docWithMarkers)},
			{Path: goodPath, Content: []byte(docWithMarkers)},
		},
		1, false,
	)
	if err != nil {
		t.Fatalf("UpdateFiles() returned err: %v (write errors are non-fatal)", err)
	}
	if len(report.WriteErrors) != 1 {
		t.Fatalf("WriteErrors len = %d, want 1; report=%+v", len(report.WriteErrors), report)
	}
	if report.WriteErrors[0].Path != badPath {
		t.Errorf("WriteErrors[0].Path = %q, want %q", report.WriteErrors[0].Path, badPath)
	}
	if report.WriteErrors[0].Err == nil {
		t.Error("WriteErrors[0].Err is nil")
	}
	if !errors.Is(report.WriteErrors[0].Err, os.ErrNotExist) {
		t.Errorf("WriteErrors[0].Err = %v, want wrapping os.ErrNotExist", report.WriteErrors[0].Err)
	}
	if len(report.Updated) != 1 || report.Updated[0].Path != goodPath {
		t.Errorf("good file not processed after bad file failure: %+v", report)
	}
}

func TestUpdateFiles_EmptyInput(t *testing.T) {
	t.Parallel()
	report, err := UpdateFiles(nil, 1, false)
	if err != nil {
		t.Fatalf("UpdateFiles(nil) error: %v", err)
	}
	if report.Updated != nil || report.Unchanged != nil ||
		report.WouldUpdate != nil || report.Skipped != nil ||
		report.WriteErrors != nil {
		t.Errorf("expected zero-value report, got %+v", report)
	}
}
