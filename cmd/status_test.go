package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/donaldgifford/docz/pkg/doczcore/config"
	"github.com/donaldgifford/docz/pkg/doczcore/document"
)

// newStatusRunner builds a Runner rooted at dir with docs under dir/docs,
// writing success output to out. It mirrors the IMPL-0009 pattern: a
// directly-constructed Runner with bytes.Buffer output and RepoRoot set to
// a temp dir, so no os.Chdir or os.Pipe is needed.
func newStatusRunner(t *testing.T, dir string, out io.Writer) *Runner {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.DocsDir = filepath.Join(dir, "docs")
	return &Runner{
		Cfg:      cfg,
		Out:      out,
		Err:      io.Discard,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now:      time.Now,
		Git:      staticGit{},
		RepoRoot: dir,
	}
}

// setupStatusRepo creates a repo with one RFC at Draft and returns the
// repo root and the RFC directory.
func setupStatusRepo(t *testing.T) (dir, rfcDir string) {
	t.Helper()
	dir = t.TempDir()
	rfcDir = filepath.Join(dir, "docs", "rfc")
	if err := os.MkdirAll(rfcDir, config.DirMode); err != nil {
		t.Fatal(err)
	}
	writeTestDoc(t, rfcDir, "0001-first.md", "RFC-0001", "First RFC", "Draft", "Author A", "2026-01-01")
	return dir, rfcDir
}

func readDoc(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(data)
}

func TestStatusSet_Happy(t *testing.T) {
	dir, rfcDir := setupStatusRepo(t)
	var out bytes.Buffer
	r := newStatusRunner(t, dir, &out)

	err := r.statusSet(statusSetOpts{format: formatText}, []string{"rfc", "RFC-0001", "Accepted"})
	if err != nil {
		t.Fatalf("statusSet() error: %v", err)
	}

	want := "docs/rfc/0001-first.md: status Draft -> Accepted\n"
	if out.String() != want {
		t.Errorf("output = %q, want %q", out.String(), want)
	}
	if got := readDoc(t, filepath.Join(rfcDir, "0001-first.md")); !strings.Contains(got, "status: Accepted") {
		t.Errorf("file not mutated to Accepted:\n%s", got)
	}
}

func TestStatusSet_NoOp(t *testing.T) {
	dir, rfcDir := setupStatusRepo(t)
	var out bytes.Buffer
	r := newStatusRunner(t, dir, &out)

	before := readDoc(t, filepath.Join(rfcDir, "0001-first.md"))
	err := r.statusSet(statusSetOpts{format: formatText}, []string{"rfc", "RFC-0001", "Draft"})
	if err != nil {
		t.Fatalf("statusSet() error: %v", err)
	}

	want := "docs/rfc/0001-first.md: already at Draft\n"
	if out.String() != want {
		t.Errorf("output = %q, want %q", out.String(), want)
	}
	if after := readDoc(t, filepath.Join(rfcDir, "0001-first.md")); after != before {
		t.Errorf("no-op mutated file:\nbefore: %q\nafter:  %q", before, after)
	}
}

func TestStatusSet_DryRun(t *testing.T) {
	dir, rfcDir := setupStatusRepo(t)
	var out bytes.Buffer
	r := newStatusRunner(t, dir, &out)

	before := readDoc(t, filepath.Join(rfcDir, "0001-first.md"))
	err := r.statusSet(statusSetOpts{format: formatText, dryRun: true}, []string{"rfc", "RFC-0001", "Accepted"})
	if err != nil {
		t.Fatalf("statusSet() error: %v", err)
	}

	want := "[dry-run] docs/rfc/0001-first.md: status Draft -> Accepted\n"
	if out.String() != want {
		t.Errorf("output = %q, want %q", out.String(), want)
	}
	if after := readDoc(t, filepath.Join(rfcDir, "0001-first.md")); after != before {
		t.Errorf("--dry-run mutated file:\nbefore: %q\nafter:  %q", before, after)
	}
}

func TestStatusSet_QuietHappy(t *testing.T) {
	dir, rfcDir := setupStatusRepo(t)
	var out bytes.Buffer
	r := newStatusRunner(t, dir, &out)

	err := r.statusSet(statusSetOpts{format: formatText, quiet: true}, []string{"rfc", "RFC-0001", "Accepted"})
	if err != nil {
		t.Fatalf("statusSet() error: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("--quiet wrote to stdout: %q", out.String())
	}
	if got := readDoc(t, filepath.Join(rfcDir, "0001-first.md")); !strings.Contains(got, "status: Accepted") {
		t.Errorf("--quiet did not mutate the file:\n%s", got)
	}
}

func TestStatusSet_UnknownType(t *testing.T) {
	dir, _ := setupStatusRepo(t)
	r := newStatusRunner(t, dir, io.Discard)

	err := r.statusSet(statusSetOpts{format: formatText}, []string{"bogus", "RFC-0001", "Accepted"})
	if !errors.Is(err, errExitCode2) {
		t.Errorf("error = %v, want errExitCode2", err)
	}
	if err == nil || !strings.Contains(err.Error(), "valid types") {
		t.Errorf("error %v, want mention of valid types", err)
	}
}

func TestStatusSet_UnknownID(t *testing.T) {
	dir, _ := setupStatusRepo(t)
	r := newStatusRunner(t, dir, io.Discard)

	err := r.statusSet(statusSetOpts{format: formatText}, []string{"rfc", "RFC-9999", "Accepted"})
	if !errors.Is(err, errExitCode1) {
		t.Errorf("error = %v, want errExitCode1", err)
	}
	if err == nil || !strings.Contains(err.Error(), filepath.Join("docs", "rfc")) {
		t.Errorf("error %v, want mention of the scanned type dir", err)
	}
}

func TestStatusSet_InvalidStatus(t *testing.T) {
	dir, _ := setupStatusRepo(t)
	r := newStatusRunner(t, dir, io.Discard)

	err := r.statusSet(statusSetOpts{format: formatText}, []string{"rfc", "RFC-0001", "Approved"})
	if !errors.Is(err, errExitCode2) {
		t.Errorf("error = %v, want errExitCode2", err)
	}
	// Registry order for rfc: Draft, Proposed, Accepted, Rejected, Superseded.
	if err == nil || !strings.Contains(err.Error(), "Draft, Proposed, Accepted, Rejected, Superseded") {
		t.Errorf("error %v, want statuses listed in registry order", err)
	}
}

func TestStatusSet_CRLF(t *testing.T) {
	dir := t.TempDir()
	rfcDir := filepath.Join(dir, "docs", "rfc")
	if err := os.MkdirAll(rfcDir, config.DirMode); err != nil {
		t.Fatal(err)
	}
	crlf := "---\r\nid: RFC-0001\r\ntitle: \"X\"\r\nstatus: Draft\r\n---\r\n\r\n# Body\r\n"
	if err := os.WriteFile(filepath.Join(rfcDir, "0001-first.md"), []byte(crlf), config.FileMode); err != nil {
		t.Fatal(err)
	}
	r := newStatusRunner(t, dir, io.Discard)

	err := r.statusSet(statusSetOpts{format: formatText}, []string{"rfc", "RFC-0001", "Accepted"})
	if !errors.Is(err, errExitCode2) {
		t.Errorf("error = %v, want errExitCode2", err)
	}
	if err == nil || !strings.Contains(err.Error(), "unsupported line endings") {
		t.Errorf("error %v, want mention of unsupported line endings", err)
	}
}

func TestStatusSet_BogusFormat(t *testing.T) {
	dir, _ := setupStatusRepo(t)
	r := newStatusRunner(t, dir, io.Discard)

	err := r.statusSet(statusSetOpts{format: "bogus"}, []string{"rfc", "RFC-0001", "Accepted"})
	if !errors.Is(err, errExitCode2) {
		t.Errorf("error = %v, want errExitCode2", err)
	}
	if err == nil || !strings.Contains(err.Error(), "text") || !strings.Contains(err.Error(), "json") {
		t.Errorf("error %v, want mention of text and json", err)
	}
}

// TestStatusSet_ResolvesUnderRepoRoot confirms the id is resolved against
// RepoRoot, not the process cwd, and that the printed path is repo-root
// relative (Decision 3). The test never calls os.Chdir.
func TestStatusSet_ResolvesUnderRepoRoot(t *testing.T) {
	dir, _ := setupStatusRepo(t)
	var out bytes.Buffer
	r := newStatusRunner(t, dir, &out)

	err := r.statusSet(statusSetOpts{format: formatText}, []string{"rfc", "RFC-0001", "Accepted"})
	if err != nil {
		t.Fatalf("statusSet() error: %v", err)
	}
	if !strings.HasPrefix(out.String(), "docs/rfc/0001-first.md:") {
		t.Errorf("path not repo-root relative: %q", out.String())
	}
}

// parseStatusJSON unmarshals the JSON status output into a struct so the
// assertions do not depend on field ordering (DESIGN-0005 Phase 3).
func parseStatusJSON(t *testing.T, b []byte) statusJSON {
	t.Helper()
	var got statusJSON
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("invalid JSON output %q: %v", b, err)
	}
	return got
}

func TestStatusSet_JSONHappy(t *testing.T) {
	dir, rfcDir := setupStatusRepo(t)
	var out bytes.Buffer
	r := newStatusRunner(t, dir, &out)

	err := r.statusSet(statusSetOpts{format: formatJSON}, []string{"rfc", "RFC-0001", "Accepted"})
	if err != nil {
		t.Fatalf("statusSet() error: %v", err)
	}

	got := parseStatusJSON(t, out.Bytes())
	want := statusJSON{
		Path:    "docs/rfc/0001-first.md",
		From:    "Draft",
		To:      "Accepted",
		DryRun:  false,
		Changed: true,
	}
	if got != want {
		t.Errorf("json = %+v, want %+v", got, want)
	}
	if !strings.HasSuffix(out.String(), "}\n") {
		t.Errorf("json output not newline-terminated: %q", out.String())
	}
	if doc := readDoc(t, filepath.Join(rfcDir, "0001-first.md")); !strings.Contains(doc, "status: Accepted") {
		t.Errorf("file not mutated:\n%s", doc)
	}
}

func TestStatusSet_JSONNoOp(t *testing.T) {
	dir, _ := setupStatusRepo(t)
	var out bytes.Buffer
	r := newStatusRunner(t, dir, &out)

	err := r.statusSet(statusSetOpts{format: formatJSON}, []string{"rfc", "RFC-0001", "Draft"})
	if err != nil {
		t.Fatalf("statusSet() error: %v", err)
	}

	got := parseStatusJSON(t, out.Bytes())
	if got.Changed {
		t.Errorf("no-op changed = true, want false")
	}
	if got.From != got.To {
		t.Errorf("no-op from %q != to %q, want equal", got.From, got.To)
	}
}

func TestStatusSet_JSONDryRun(t *testing.T) {
	dir, rfcDir := setupStatusRepo(t)
	var out bytes.Buffer
	r := newStatusRunner(t, dir, &out)

	before := readDoc(t, filepath.Join(rfcDir, "0001-first.md"))
	err := r.statusSet(statusSetOpts{format: formatJSON, dryRun: true}, []string{"rfc", "RFC-0001", "Accepted"})
	if err != nil {
		t.Fatalf("statusSet() error: %v", err)
	}

	got := parseStatusJSON(t, out.Bytes())
	if !got.DryRun {
		t.Errorf("dry_run = false, want true")
	}
	if !got.Changed {
		t.Errorf("changed = false, want true (reports what would happen)")
	}
	if after := readDoc(t, filepath.Join(rfcDir, "0001-first.md")); after != before {
		t.Errorf("--dry-run mutated file")
	}
}

func TestStatusSet_JSONQuiet(t *testing.T) {
	dir, _ := setupStatusRepo(t)
	var out bytes.Buffer
	r := newStatusRunner(t, dir, &out)

	err := r.statusSet(statusSetOpts{format: formatJSON, quiet: true}, []string{"rfc", "RFC-0001", "Accepted"})
	if err != nil {
		t.Fatalf("statusSet() error: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("--quiet --format=json wrote to stdout: %q", out.String())
	}
}

func TestStatusSet_JSONError(t *testing.T) {
	dir, _ := setupStatusRepo(t)
	var out bytes.Buffer
	r := newStatusRunner(t, dir, &out)

	err := r.statusSet(statusSetOpts{format: formatJSON}, []string{"rfc", "RFC-9999", "Accepted"})
	if !errors.Is(err, errExitCode1) {
		t.Errorf("error = %v, want errExitCode1", err)
	}
	if out.Len() != 0 {
		t.Errorf("error path emitted JSON to stdout: %q", out.String())
	}
	// The error message is plain text, not JSON.
	if err == nil || strings.HasPrefix(strings.TrimSpace(err.Error()), "{") {
		t.Errorf("error message looks like JSON: %v", err)
	}
}

func TestExitCodeFor(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"nil-ish plain error", errors.New("boom"), 1},
		{"exit code 1 marker", exitErrorf(errExitCode1, "lookup"), 1},
		{"exit code 2 marker", exitErrorf(errExitCode2, "validation"), 2},
		{"wrapped exit code 2", document.ErrNoFrontmatter, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := exitCodeFor(tt.err); got != tt.want {
				t.Errorf("exitCodeFor(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}

func TestFormatStatusText(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		res  statusResult
		want string
	}{
		{
			name: "changed",
			res:  statusResult{path: "docs/rfc/0001.md", from: "Draft", to: "Accepted", changed: true},
			want: "docs/rfc/0001.md: status Draft -> Accepted",
		},
		{
			name: "no-op",
			res:  statusResult{path: "docs/rfc/0001.md", to: "Accepted", changed: false},
			want: "docs/rfc/0001.md: already at Accepted",
		},
		{
			name: "dry-run changed",
			res:  statusResult{path: "docs/rfc/0001.md", from: "Draft", to: "Accepted", changed: true, dryRun: true},
			want: "[dry-run] docs/rfc/0001.md: status Draft -> Accepted",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := formatStatusText(tt.res); got != tt.want {
				t.Errorf("formatStatusText() = %q, want %q", got, tt.want)
			}
		})
	}
}
