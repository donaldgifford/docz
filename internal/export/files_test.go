package export

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/donaldgifford/docz/v2/internal/store"
	doczcfg "github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/repo"
	"github.com/donaldgifford/docz/v2/pkg/export/confluence"
)

// TestFiles_ReadmesMatchDoczUpdate: the README files builds for each
// enabled type is the one docz update writes for the same documents.
func TestFiles_ReadmesMatchDoczUpdate(t *testing.T) {
	t.Parallel()

	cfg := fixtureConfig()
	in := fixtureInputs(t, &cfg)

	rp := &repo.Repo{Root: t.TempDir(), Cfg: &cfg}
	if _, err := rp.Init(t.Context(), repo.InitOptions{}); err != nil {
		t.Fatal(err)
	}

	for _, d := range in.Documents {
		if err := os.WriteFile(filepath.Join(rp.Root, filepath.FromSlash(d.Path)), []byte(d.RawMd), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := rp.Update(t.Context(), nil, repo.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}

	fsys, err := files(&in, &cfg)
	if err != nil {
		t.Fatal(err)
	}

	for _, typeName := range cfg.EnabledTypes() {
		readme := filepath.ToSlash(filepath.Join(cfg.TypeDir(typeName), "README.md"))

		want, err := os.ReadFile(filepath.Join(rp.Root, filepath.FromSlash(readme)))
		if err != nil {
			t.Fatal(err)
		}

		got, ok := fsys[readme]
		if !ok {
			t.Fatalf("no %s", readme)
		}

		if !bytes.Equal(got.Data, want) {
			t.Errorf("%s:\n%s\nwant (docz update):\n%s", readme, got.Data, want)
		}
	}
}

func TestFiles_LandingAndPages(t *testing.T) {
	t.Parallel()

	cfg := fixtureConfig()
	cfg.API.Enabled, cfg.API.LandingPage = true, "docs/index.md"

	in := fixtureInputs(t, &cfg)
	in.Repo.IndexMd = pgtype.Text{String: "# Home\n", Valid: true}
	in.Repo.IndexSha = pgtype.Text{String: "abc", Valid: true}
	in.Pages = []store.RepoPage{{Path: "DEVELOPMENT.md", RepoPath: "DEVELOPMENT.md", RawMd: "# Dev\n"}}

	fsys, err := files(&in, &cfg)
	if err != nil {
		t.Fatal(err)
	}

	for p, want := range map[string]string{
		"docs/index.md":          "# Home\n",
		"DEVELOPMENT.md":         "# Dev\n",
		"docs/rfc/0001-first.md": rfcMD,
	} {
		if f, ok := fsys[p]; !ok || string(f.Data) != want {
			t.Errorf("%s = %v", p, f)
		}
	}

	// With no cached index the landing page is absent, not empty.
	in.Repo.IndexSha = pgtype.Text{}

	fsys, err = files(&in, &cfg)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := fsys["docs/index.md"]; ok {
		t.Error("landing page present with no cached index")
	}
}

func TestBlobResolver(t *testing.T) {
	t.Parallel()

	resolve := blobResolver("o", "r", "main", func(string) bool { return false })
	base := "https://github.com/o/r/blob/main/"

	tests := []struct{ from, href, want string }{
		{"docs/rfc/0001-a.md", "../../justfile", base + "justfile"},
		{"docs/rfc/0001-a.md", "../../cmd/x.go#L3", base + "cmd/x.go#L3"},
		{"docs/rfc/0001-a.md", "0001-a.md", base + "docs/rfc/0001-a.md"},
		{"docs/rfc/0001-a.md", "../../../outside.md", ""},
		{"docs/rfc/0001-a.md", "/abs.md", ""},
		{"docs/rfc/0001-a.md", "#section", ""},
	}

	for _, tt := range tests {
		if got := resolve(tt.from, tt.href).URL; got != tt.want {
			t.Errorf("resolve(%q, %q) = %q, want %q", tt.from, tt.href, got, tt.want)
		}
	}
}

func TestBlobResolver_RemovedDocument(t *testing.T) {
	t.Parallel()

	cfg := doczcfg.DefaultConfig()
	fsys := fstest.MapFS{"docs/adr/0001-kept.md": &fstest.MapFile{Data: []byte("x")}}
	resolve := blobResolver("o", "r", "main", missingDocument(fsys, &cfg))
	base := "https://github.com/o/r/blob/main/"

	tests := []struct{ name, href, want string }{
		{"removed document", "../adr/0002-removed.md", ""},
		{"removed document with a fragment", "../adr/0002-removed.md#context", ""},
		{"ingested document", "../adr/0001-kept.md", base + "docs/adr/0001-kept.md"},
		{"non-document under a type dir", "../adr/notes.md", base + "docs/adr/notes.md"},
		{"document outside a type dir", "../../other/0001-x.md", base + "other/0001-x.md"},
		{"other file", "../../scripts/check.sh", base + "scripts/check.sh"},
	}

	for _, tt := range tests {
		if got := resolve("docs/rfc/0001-a.md", tt.href).URL; got != tt.want {
			t.Errorf("%s: resolve(%q) = %q, want %q", tt.name, tt.href, got, tt.want)
		}
	}
}

func TestClassify(t *testing.T) {
	t.Parallel()

	page := func(err error) error { return &confluence.PageError{Title: "p", Err: err} }

	tests := []struct {
		name   string
		err    error
		status Status
		skip   bool
	}{
		{"nil", nil, StatusSucceeded, false},
		{"config", &confluence.ConfigError{Reason: "no folder"}, StatusFailed, true},
		{"auth", &confluence.AuthError{Status: http.StatusUnauthorized}, StatusFailed, true},
		{"auth on a page", page(&confluence.AuthError{Status: http.StatusForbidden}), StatusFailed, true},
		{"page 429", page(&confluence.RequestError{Status: http.StatusTooManyRequests}), StatusPartial, false},
		{"page 5xx", page(&confluence.RequestError{Status: http.StatusBadGateway}), StatusPartial, false},
		{"page network", page(&confluence.RequestError{Err: os.ErrDeadlineExceeded}), StatusPartial, false},
		{"page malformed", page(&confluence.MalformedError{Line: 3}), StatusPartial, true},
		{"page title", page(&confluence.TitleError{Title: "t"}), StatusPartial, true},
		{"page 400", page(&confluence.RequestError{Status: http.StatusBadRequest}), StatusPartial, true},
		{"cancelled", context.Canceled, StatusFailed, false},
		{"space 5xx", &confluence.RequestError{Status: http.StatusInternalServerError}, StatusFailed, false},
		{"space 404", &confluence.RequestError{Status: http.StatusNotFound}, StatusFailed, true},
	}

	for _, tt := range tests {
		status, skip := classify(tt.err)
		if status != tt.status || skip != tt.skip {
			t.Errorf("%s: classify = %s, %v; want %s, %v", tt.name, status, skip, tt.status, tt.skip)
		}
	}
}
