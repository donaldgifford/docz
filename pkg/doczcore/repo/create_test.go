package repo

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/document"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/index"
)

// createTestRepo returns a Repo over an empty temp root with the default
// config, which is the state `docz init` leaves behind: every built-in type
// enabled, docs under docs/, ToC on.
func createTestRepo(t *testing.T) *Repo {
	t.Helper()

	cfg := config.DefaultConfig()

	return &Repo{Root: t.TempDir(), Cfg: &cfg}
}

// createTestRepoWithDisabled returns the same Repo with one built-in switched
// off, which is the only way to reach TypeDisabledError now: every built-in is
// enabled by default since ADR-0003 dropped plan, the one that used to ship
// disabled.
func createTestRepoWithDisabled(t *testing.T, typeName string) *Repo {
	t.Helper()

	cfg := config.DefaultConfig()

	tc := cfg.Types[typeName]
	tc.Enabled = false
	cfg.Types[typeName] = tc

	return &Repo{Root: t.TempDir(), Cfg: &cfg}
}

// createTestNow is the pinned creation timestamp. A fixed date is what
// makes the frontmatter assertion exact rather than a format check.
var createTestNow = time.Date(2026, time.March, 4, 10, 30, 0, 0, time.UTC)

// createTestFrontmatter parses the frontmatter of a created document,
// failing the test if it cannot be read.
func createTestFrontmatter(t *testing.T, path string) document.Frontmatter {
	t.Helper()

	fm, _, err := document.LoadFrontmatter(path)
	if err != nil {
		t.Fatalf("reading frontmatter from %s: %v", path, err)
	}

	return fm
}

func TestCreate_WritesTheDocument(t *testing.T) {
	t.Parallel()

	r := createTestRepo(t)

	result, err := r.Create(t.Context(), CreateOptions{
		Type:   "adr",
		Title:  "Use PostgreSQL",
		Author: "Ada Lovelace",
		Now:    createTestNow,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if result.Type != "adr" {
		t.Errorf("Type = %q, want %q", result.Type, "adr")
	}

	if result.Number != "0001" {
		t.Errorf("Number = %q, want %q", result.Number, "0001")
	}

	if result.Filename != "0001-use-postgresql.md" {
		t.Errorf("Filename = %q, want %q", result.Filename, "0001-use-postgresql.md")
	}

	want := filepath.Join(r.Root, "docs", "adr", "0001-use-postgresql.md")
	if result.FilePath != want {
		t.Errorf("FilePath = %q, want %q", result.FilePath, want)
	}

	fm := createTestFrontmatter(t, want)

	if fm.ID != "ADR-0001" {
		t.Errorf("id = %q, want %q", fm.ID, "ADR-0001")
	}

	if fm.Title != "Use PostgreSQL" {
		t.Errorf("title = %q, want %q", fm.Title, "Use PostgreSQL")
	}

	if fm.Author != "Ada Lovelace" {
		t.Errorf("author = %q, want %q", fm.Author, "Ada Lovelace")
	}

	if fm.Created != "2026-03-04" {
		t.Errorf("created = %q, want %q", fm.Created, "2026-03-04")
	}
}

// TestCreate_ResolvesTheTypeToken pins that Create accepts anything
// ValidateType does: the id_prefix and a registry alias both land in the
// canonical type's directory.
func TestCreate_ResolvesTheTypeToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		token string
		want  string
	}{
		{name: "canonical name", token: "investigation", want: "investigation"},
		{name: "registry alias", token: "inv", want: "investigation"},
		{name: "id prefix", token: "INV", want: "investigation"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := createTestRepo(t)

			result, err := r.Create(t.Context(), CreateOptions{
				Type:  tt.token,
				Title: "Does It Resolve",
				Now:   createTestNow,
			})
			if err != nil {
				t.Fatalf("Create: %v", err)
			}

			if result.Type != tt.want {
				t.Errorf("Type = %q, want %q", result.Type, tt.want)
			}

			if _, err := os.Stat(result.FilePath); err != nil {
				t.Errorf("created document: %v", err)
			}
		})
	}
}

func TestCreate_UpdateRunsTheTypeRefresh(t *testing.T) {
	t.Parallel()

	r := createTestRepo(t)

	result, err := r.Create(t.Context(), CreateOptions{
		Type:   "adr",
		Title:  "Use PostgreSQL",
		Now:    createTestNow,
		Update: true,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if result.Update == nil {
		t.Fatal("Update report is nil, want the refresh that ran")
	}

	if result.Update.Type != "adr" {
		t.Errorf("Update.Type = %q, want %q", result.Update.Type, "adr")
	}

	if result.Update.Docs != 1 {
		t.Errorf("Update.Docs = %d, want 1", result.Update.Docs)
	}

	if result.Update.Index.Action != index.ActionCreated {
		t.Errorf("Update.Index.Action = %v, want ActionCreated", result.Update.Index.Action)
	}

	// The README is the point of the refresh, and the new document has to
	// be in its table.
	readme := createTestReadFile(t, filepath.Join(r.Root, "docs", "adr", config.IndexFileName))
	if !strings.Contains(readme, "ADR-0001") {
		t.Errorf("README does not list the new document:\n%s", readme)
	}

	// Every built-in template ships a ToC marker pair, so the created
	// document is rewritten with its own table of contents.
	if result.Update.ToC == nil {
		t.Fatal("ToC report is nil with toc enabled")
	}

	if got := len(result.Update.ToC.Updated); got != 1 {
		t.Errorf("ToC.Updated = %d files, want 1 (report: %+v)", got, *result.Update.ToC)
	}
}

func TestCreate_NoUpdateLeavesTheReportNil(t *testing.T) {
	t.Parallel()

	r := createTestRepo(t)

	result, err := r.Create(t.Context(), CreateOptions{
		Type:  "adr",
		Title: "Use PostgreSQL",
		Now:   createTestNow,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if result.Update != nil {
		t.Errorf("Update = %+v, want nil without CreateOptions.Update", *result.Update)
	}

	readme := filepath.Join(r.Root, "docs", "adr", config.IndexFileName)
	if _, err := os.Stat(readme); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("stat %s = %v, want the README not to exist", readme, err)
	}
}

// TestCreate_SecondDocumentTakesTheNextNumber is the shape a repeated
// title actually has: the number increments, so two documents with the
// same slug coexist and neither collides.
func TestCreate_SecondDocumentTakesTheNextNumber(t *testing.T) {
	t.Parallel()

	r := createTestRepo(t)

	opts := CreateOptions{Type: "adr", Title: "Use PostgreSQL", Now: createTestNow}

	first, err := r.Create(t.Context(), opts)
	if err != nil {
		t.Fatalf("first Create: %v", err)
	}

	second, err := r.Create(t.Context(), opts)
	if err != nil {
		t.Fatalf("second Create: %v", err)
	}

	if first.Number != "0001" || second.Number != "0002" {
		t.Errorf("numbers = %q, %q, want 0001, 0002", first.Number, second.Number)
	}

	if second.Filename != "0002-use-postgresql.md" {
		t.Errorf("second Filename = %q, want %q", second.Filename, "0002-use-postgresql.md")
	}
}

// TestCreate_ExistingPathIsAnExistsError uses a directory in the way,
// which is the collision the numbering cannot avoid: the ID scan skips
// directories, so the next number is one the path is already taken by.
func TestCreate_ExistingPathIsAnExistsError(t *testing.T) {
	t.Parallel()

	r := createTestRepo(t)

	inTheWay := filepath.Join(r.Root, "docs", "adr", "0001-use-postgresql.md")
	if err := os.MkdirAll(inTheWay, 0o750); err != nil {
		t.Fatal(err)
	}

	_, err := r.Create(t.Context(), CreateOptions{
		Type:  "adr",
		Title: "Use PostgreSQL",
		Now:   createTestNow,
	})

	var exists *ExistsError
	if !errors.As(err, &exists) {
		t.Fatalf("Create error = %v, want *ExistsError", err)
	}

	if exists.Path != inTheWay {
		t.Errorf("ExistsError.Path = %q, want %q", exists.Path, inTheWay)
	}
}

func TestCreate_DisabledTypeIsATypeDisabledError(t *testing.T) {
	t.Parallel()

	r := createTestRepoWithDisabled(t, "design")

	// A disabled type still resolves, and is still refused. That it is a
	// different error from an unknown token is the whole point: one is a
	// config edit, the other is a typo.
	_, err := r.Create(t.Context(), CreateOptions{Type: "design", Title: "Anything"})

	var disabled *TypeDisabledError
	if !errors.As(err, &disabled) {
		t.Fatalf("Create error = %v, want *TypeDisabledError", err)
	}

	if disabled.Type != "design" {
		t.Errorf("TypeDisabledError.Type = %q, want %q", disabled.Type, "design")
	}

	if _, err := os.Stat(filepath.Join(r.Root, "docs")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("stat docs/ = %v, want nothing written", err)
	}
}

func TestCreate_UnknownTypeIsAnUnknownTypeError(t *testing.T) {
	t.Parallel()

	r := createTestRepo(t)

	_, err := r.Create(t.Context(), CreateOptions{Type: "frameworks", Title: "Anything"})

	var unknown *UnknownTypeError
	if !errors.As(err, &unknown) {
		t.Fatalf("Create error = %v, want *UnknownTypeError", err)
	}

	if unknown.Token != "frameworks" {
		t.Errorf("UnknownTypeError.Token = %q, want %q", unknown.Token, "frameworks")
	}

	// The frozen sentinel still answers, which is what a v1 caller tests.
	if !errors.Is(err, config.ErrUnknownType) {
		t.Errorf("errors.Is(err, config.ErrUnknownType) = false, want true")
	}
}

// TestCreate_ZeroNowDatesTheDocument pins the documented fallback: no time
// source still produces a date, rather than an empty field or a zero year.
func TestCreate_ZeroNowDatesTheDocument(t *testing.T) {
	t.Parallel()

	r := createTestRepo(t)

	result, err := r.Create(t.Context(), CreateOptions{Type: "rfc", Title: "Undated"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	fm := createTestFrontmatter(t, result.FilePath)

	if want := time.Now().Format(time.DateOnly); fm.Created != want {
		t.Errorf("created = %q, want today (%q)", fm.Created, want)
	}
}

func TestCreate_Status(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		docType  string
		status   string
		wantStat string
	}{
		{
			name:     "explicit status is honoured",
			docType:  "rfc",
			status:   "Accepted",
			wantStat: "Accepted",
		},
		{
			name:     "empty status takes the type's first",
			docType:  "rfc",
			status:   "",
			wantStat: "Draft",
		},
		{
			name:     "a type whose first status is not Draft",
			docType:  "adr",
			status:   "",
			wantStat: "Proposed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := createTestRepo(t)

			result, err := r.Create(t.Context(), CreateOptions{
				Type:   tt.docType,
				Title:  "Status Check",
				Status: tt.status,
				Now:    createTestNow,
			})
			if err != nil {
				t.Fatalf("Create: %v", err)
			}

			fm := createTestFrontmatter(t, result.FilePath)

			if string(fm.Status) != tt.wantStat {
				t.Errorf("status = %q, want %q", fm.Status, tt.wantStat)
			}
		})
	}
}

// TestCreate_FiresFileWritten pins the document event, including that it
// carries FileDocument rather than a path the hook has to classify.
func TestCreate_FiresFileWritten(t *testing.T) {
	t.Parallel()

	r := createTestRepo(t)

	var written []string

	ctx := WithHooks(t.Context(), &Hooks{
		FileWritten: func(path string, kind FileKind) {
			written = append(written, kind.String()+" "+filepath.Base(path))
		},
	})

	if _, err := r.Create(ctx, CreateOptions{Type: "adr", Title: "Hooked", Now: createTestNow}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if len(written) != 1 || written[0] != "document 0001-hooked.md" {
		t.Errorf("FileWritten events = %v, want [document 0001-hooked.md]", written)
	}
}

// createTestReadFile reads a file the test expects to exist.
func createTestReadFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	return string(data)
}
