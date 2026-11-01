package html

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/repo"
)

// fixtureRepo copies a parity fixture into a temporary directory and opens
// it, so a test may write beside it.
func fixtureRepo(t *testing.T, name string) *repo.Repo {
	t.Helper()

	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS(filepath.Join("..", "..", "..", "test", "parity", "fixtures", name))); err != nil {
		t.Fatal(err)
	}

	rp, err := repo.Open(context.Background(), dir, "")
	if err != nil {
		t.Fatal(err)
	}

	return rp
}

func TestExportWritesThenReportsUnchanged(t *testing.T) {
	t.Parallel()

	rp := fixtureRepo(t, "rfc")

	var done []string

	ctx := WithHooks(context.Background(), &Hooks{PageDone: func(r PageResult) { done = append(done, r.ID) }})

	first, err := Export(ctx, rp, ExportOptions{Version: "test"})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	if first.Out != filepath.Join(rp.Root, DefaultOut) {
		t.Errorf("Out = %q", first.Out)
	}

	if len(first.Pages) != 3 || len(done) != 3 {
		t.Fatalf("%d pages, %d hook calls, want two documents and the index", len(first.Pages), len(done))
	}

	for _, p := range first.Pages[:2] {
		if p.Action != ActionWrote || !strings.HasPrefix(p.Path, "rfc/") || !strings.HasSuffix(p.Path, ".html") {
			t.Errorf("page %s %s at %s", p.ID, p.Action, p.Path)
		}

		body, err := os.ReadFile(filepath.Join(first.Out, filepath.FromSlash(p.Path)))
		if err != nil || !bytes.Equal(body, p.Body) {
			t.Errorf("%s on disk differs from the report: %v", p.Path, err)
		}
	}

	second, err := Export(context.Background(), rp, ExportOptions{Version: "test"})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	for _, p := range second.Pages {
		if p.Action != ActionUnchanged {
			t.Errorf("second run: %s %s, want unchanged", p.Path, p.Action)
		}
	}
}

func TestExportAbsoluteDocsDir(t *testing.T) {
	t.Parallel()

	rp := fixtureRepo(t, "rfc")
	rp.Cfg.DocsDir = filepath.Join(rp.Root, rp.Cfg.DocsDir)

	rep, err := Export(context.Background(), rp, ExportOptions{DryRun: true})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	if !strings.HasPrefix(rep.Pages[0].Path, "rfc/") {
		t.Errorf("Path = %q with an absolute docs_dir, want it below the docs directory", rep.Pages[0].Path)
	}
}

func TestExportDryRunWritesNothing(t *testing.T) {
	t.Parallel()

	rp := fixtureRepo(t, "adr")

	rep, err := Export(context.Background(), rp, ExportOptions{DryRun: true, Out: "site"})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	if !rep.DryRun || len(rep.Pages) == 0 || rep.Failed() != 0 {
		t.Fatalf("dry run %v, %d pages, %d failed", rep.DryRun, len(rep.Pages), rep.Failed())
	}

	if _, err := os.Stat(filepath.Join(rp.Root, "site")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a dry run created the output directory: %v", err)
	}
}

func TestExportByIDAndErrors(t *testing.T) {
	t.Parallel()

	rp := fixtureRepo(t, "impl")

	rep, err := Export(context.Background(), rp, ExportOptions{IDs: []string{"IMPL-0001"}, DryRun: true})
	if err != nil || len(rep.Pages) != 1 || rep.Pages[0].ID != "IMPL-0001" {
		t.Fatalf("Export(IMPL-0001) = %d pages, %v", len(rep.Pages), err)
	}

	var nf *repo.NotFoundError
	if _, err := Export(context.Background(), rp, ExportOptions{IDs: []string{"IMPL-0099"}}); !errors.As(err, &nf) {
		t.Errorf("a missing id: %v, want a NotFoundError", err)
	}

	var ut *repo.UnknownTypeError
	if _, err := Export(context.Background(), rp, ExportOptions{Types: []string{"nope"}}); !errors.As(err, &ut) {
		t.Errorf("an unknown type: %v, want an UnknownTypeError", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := Export(ctx, rp, ExportOptions{DryRun: true}); !errors.Is(err, context.Canceled) {
		t.Errorf("a cancelled run: %v", err)
	}
}

func TestExportReportsAFailedPage(t *testing.T) {
	t.Parallel()

	rp := fixtureRepo(t, "design")
	entries, err := rp.List(context.Background(), []string{"design"})
	if err != nil || len(entries) == 0 {
		t.Fatal(err)
	}

	crlf := strings.ReplaceAll(string(entries[0].Content), "\n", "\r\n")
	if err := os.WriteFile(rp.Path(entries[0].Path), []byte(crlf), 0o600); err != nil {
		t.Fatal(err)
	}

	rep, err := Export(context.Background(), rp, ExportOptions{DryRun: true})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	if rep.Failed() != 1 || rep.Pages[0].Action != ActionFailed || !strings.Contains(rep.Pages[0].Error, "line endings") {
		t.Errorf("first page %s %q, want it failed for its line endings; %d failed", rep.Pages[0].Action, rep.Pages[0].Error, rep.Failed())
	}
}

func TestExportIndex(t *testing.T) {
	t.Parallel()

	rp := fixtureRepo(t, "rfc")

	rep, err := Export(context.Background(), rp, ExportOptions{DryRun: true, Site: "fixture"})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	index := rep.Pages[len(rep.Pages)-1]
	if index.Path != IndexPath || index.ID != "" || index.Action != ActionWrote {
		t.Fatalf("last page %q %q %s, want the index", index.Path, index.ID, index.Action)
	}

	body := string(index.Body)

	for _, want := range []string{
		`<title>fixture documents</title>`,
		`data-tab="rfcs"`,
		"<th>ID</th>\n<th>Title</th>\n<th>Status</th>\n<th>Date</th>",
		`<a href="` + rep.Pages[0].Path + `">` + rep.Pages[0].ID + `</a>`,
		`<a href="` + rep.Pages[1].Path + `">` + rep.Pages[1].ID + `</a>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the index lacks %s", want)
		}
	}

	if strings.Contains(body, "inferred from headings") {
		t.Errorf("the index says its sections were inferred")
	}

	if len(index.Links) != 0 {
		t.Errorf("the index has unresolved links: %v", index.Links)
	}

	one, err := Export(context.Background(), rp, ExportOptions{DryRun: true, IDs: []string{rep.Pages[0].ID}})
	if err != nil || len(one.Pages) != 1 {
		t.Errorf("a one-page run has %d pages (%v), want no index", len(one.Pages), err)
	}
}

// TestExportThemeFromConfig pins DESIGN-0025 §3: an empty Theme is the
// repository's html.theme, and an explicit one wins.
func TestExportThemeFromConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, cfg, opt, want string
	}{
		{name: "default config is paper", want: "paper"},
		{name: "the key picks the theme", cfg: "classic", want: "classic"},
		{name: "an explicit theme wins", cfg: "classic", opt: "paper", want: "paper"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rp := fixtureRepo(t, "adr")
			if tt.cfg != "" {
				rp.Cfg.HTML.Theme = tt.cfg
			}

			rep, err := Export(context.Background(), rp, ExportOptions{DryRun: true, Theme: tt.opt})
			if err != nil {
				t.Fatalf("Export: %v", err)
			}

			if rep.Theme != tt.want {
				t.Errorf("Report.Theme = %q, want %q", rep.Theme, tt.want)
			}

			marker := `data-theme="` + tt.want + `"`
			if !bytes.Contains(rep.Pages[0].Body, []byte(marker)) {
				t.Errorf("page has no %s", marker)
			}
		})
	}
}
