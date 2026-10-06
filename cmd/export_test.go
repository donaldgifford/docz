package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/repo"
	"github.com/donaldgifford/docz/v2/pkg/export/confluence"
)

var updateExportGoldens = flag.Bool("update", false, "update golden files")

// The Confluence export through the CLI (IMPL-0023). A file of its own
// because the existing cmd/*_test.go files are frozen (ADR-0001 Decision 7).

// exportRunner builds a Runner over a temp repository with the default
// config.
func exportRunner(t *testing.T) (*Runner, *bytes.Buffer, string) {
	t.Helper()

	root := t.TempDir()

	cfg := config.DefaultConfig()
	cfg.DocsDir = filepath.Join(root, "docs")

	var out bytes.Buffer

	return &Runner{
		Cfg:      cfg,
		Out:      &out,
		Err:      io.Discard,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now:      time.Now,
		Git:      staticGit{},
		RepoRoot: root,
	}, &out, root
}

func TestExport_InitWritesTheSyncBlockDisabled(t *testing.T) {
	r, _, root := exportRunner(t)

	if err := r.Init(false); err != nil {
		t.Fatalf("init: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(root, config.ConfigFileName))
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := config.ParseBytes(raw)
	if err != nil {
		t.Fatalf("parsing the generated .docz.yaml: %v", err)
	}

	if !strings.Contains(string(raw), "\nsync:\n  confluence:\n    enabled: false\n") {
		t.Errorf("generated .docz.yaml has no disabled sync.confluence block:\n%s", raw)
	}

	if cfg.Sync.Confluence.Enabled || cfg.Sync.Confluence.Mermaid.Viewer != config.MermaidViewerAuto {
		t.Errorf("Sync = %+v, want disabled with viewer auto", cfg.Sync)
	}
}

func TestExport_ConfigPrintsTheSyncBlock(t *testing.T) {
	r, out, _ := exportRunner(t)
	r.Cfg.Sync.Confluence.Site = "https://example.atlassian.net"

	if err := r.Config(); err != nil {
		t.Fatalf("config: %v", err)
	}

	cfg, err := config.ParseBytes(out.Bytes())
	if err != nil {
		t.Fatalf("parsing docz config output: %v", err)
	}

	if cfg.Sync.Confluence.Site != "https://example.atlassian.net" {
		t.Errorf("round-tripped site = %q, want it printed and read back", cfg.Sync.Confluence.Site)
	}
}

// cmdFake is a minimal in-memory Confluence for the command's tests: pages
// by title, a call count, and one switch each for a failing update and a
// rejected credential.
type cmdFake struct {
	pages     map[string]*confluence.Page
	props     map[string]*confluence.Property
	bodies    map[string][]byte
	calls     int
	failTitle string
	authFail  bool
}

func newCmdFake() *cmdFake {
	return &cmdFake{
		pages:  make(map[string]*confluence.Page),
		props:  make(map[string]*confluence.Property),
		bodies: make(map[string][]byte),
	}
}

func (f *cmdFake) SpaceID(context.Context, string) (string, error) {
	f.calls++
	if f.authFail {
		return "", &confluence.AuthError{Op: "get space", Status: 401, Body: "unauthorized"}
	}

	return "S", nil
}

func (f *cmdFake) FindPage(_ context.Context, _, title string) (*confluence.Page, error) {
	f.calls++
	if p, ok := f.pages[title]; ok {
		cp := *p

		return &cp, nil
	}

	return nil, nil
}

func (f *cmdFake) byID(id string) *confluence.Page {
	for _, p := range f.pages {
		if p.ID == id {
			return p
		}
	}

	return nil
}

func (f *cmdFake) CreatePage(_ context.Context, p *confluence.NewPage) (*confluence.Page, error) {
	f.calls++
	id := fmt.Sprint(len(f.pages) + 1)
	f.pages[p.Title] = &confluence.Page{
		ID: id, Title: p.Title, ParentID: p.ParentID, SpaceID: p.SpaceID, Version: 1,
		WebURL: "https://example.atlassian.net/wiki/pages/" + id,
	}
	f.bodies[id] = p.Body
	cp := *f.pages[p.Title]

	return &cp, nil
}

func (f *cmdFake) UpdatePage(_ context.Context, id string, u *confluence.PageUpdate) (*confluence.Page, error) {
	f.calls++
	p := f.byID(id)

	if p.Title == f.failTitle {
		return nil, &confluence.RequestError{Op: "update page", Status: 500, Body: "boom"}
	}

	p.Version, p.ParentID = u.Version, u.ParentID
	if u.Body != nil {
		f.bodies[id] = u.Body
	}

	cp := *p

	return &cp, nil
}

func (f *cmdFake) Property(_ context.Context, pageID, _ string) (*confluence.Property, error) {
	f.calls++
	if p, ok := f.props[pageID]; ok {
		cp := *p

		return &cp, nil
	}

	return nil, nil
}

func (f *cmdFake) SetProperty(_ context.Context, pageID string, p *confluence.Property) error {
	f.calls++
	f.props[pageID] = &confluence.Property{ID: "p" + pageID, Key: p.Key, Value: p.Value, Version: p.Version + 1}

	return nil
}

func (f *cmdFake) Children(_ context.Context, parentID string) ([]confluence.Page, error) {
	f.calls++

	var out []confluence.Page

	for _, p := range f.pages {
		if p.ParentID == parentID {
			out = append(out, *p)
		}
	}

	return out, nil
}

// exportFixture is a Runner over a repository with sync enabled, two
// documents, credentials in the environment, and fake injected as the
// client.
func exportFixture(t *testing.T, fake *cmdFake) (*Runner, *bytes.Buffer) {
	t.Helper()

	r, out, root := exportRunner(t)
	r.Cfg.Sync.Confluence = config.ConfluenceSyncConfig{
		Enabled: true, Site: "https://example.atlassian.net", Space: "DOCZ", Parent: "docz",
		Types:   []string{"rfc", "adr"},
		Mermaid: config.MermaidSyncConfig{Viewer: config.MermaidViewerAuto},
	}
	r.Git = staticGit{Remote: "https://github.com/o/r"}

	rp := r.repoOrOpen()
	ctx := t.Context()

	if _, err := rp.Init(ctx, repo.InitOptions{}); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 3, 4, 0, 0, 0, 0, time.UTC)

	for _, d := range []struct{ typ, title, body string }{
		{"rfc", "First proposal", ""},
		{"adr", "A decision", ""},
	} {
		if _, err := rp.Create(ctx, repo.CreateOptions{Type: d.typ, Title: d.title, Author: "T", Now: now, Update: true}); err != nil {
			t.Fatal(err)
		}
	}

	// The ADR links to a file nobody exports and to a sibling that does not
	// exist, so the report has one placed link and one unresolved.
	if err := os.WriteFile(filepath.Join(root, "justfile"), []byte("default:\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	adr := filepath.Join(root, "docs", "adr", "0001-a-decision.md")
	appendFile(t, adr, "\nSee [the justfile](../../justfile) and [nothing](0009-missing.md).\n")

	t.Setenv(envAtlassianEmail, "me@example.com")
	t.Setenv(envAtlassianToken, "tok")

	saved := newConfluenceClient
	newConfluenceClient = func(string, string, string) confluence.Client { return fake }

	t.Cleanup(func() { newConfluenceClient = saved })

	return r, out
}

func appendFile(t *testing.T, path, extra string) {
	t.Helper()

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, append(b, extra...), 0o600); err != nil {
		t.Fatal(err)
	}
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}

	return exitCodeFor(err)
}

func checkExportGolden(t *testing.T, name string, got []byte) {
	t.Helper()

	path := filepath.Join("testdata", "golden", "export", name)

	if *updateExportGoldens {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, got, 0o600); err != nil {
			t.Fatal(err)
		}

		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden (run with -update): %v", err)
	}

	if !bytes.Equal(got, want) {
		t.Errorf("%s mismatch:\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

func TestExportConfluence_TextReport(t *testing.T) {
	fake := newCmdFake()
	r, out := exportFixture(t, fake)

	if err := r.exportConfluence(t.Context(), exportOpts{format: formatText}, nil); err != nil {
		t.Fatalf("export: %v", err)
	}

	checkExportGolden(t, "text.golden", out.Bytes())

	out.Reset()

	if err := r.exportConfluence(t.Context(), exportOpts{format: formatText}, nil); err != nil {
		t.Fatalf("second export: %v", err)
	}

	checkExportGolden(t, "text_unchanged.golden", out.Bytes())

	body := string(fake.bodies[fake.pages["ADR-0001: A decision"].ID])
	if !strings.Contains(body, `href="https://github.com/o/r/blob/main/justfile"`) {
		t.Errorf("the unexported link is not a blob URL:\n%s", body)
	}
}

func TestExportConfluence_JSONReport(t *testing.T) {
	r, out := exportFixture(t, newCmdFake())

	if err := r.exportConfluence(t.Context(), exportOpts{format: formatJSON}, nil); err != nil {
		t.Fatalf("export: %v", err)
	}

	var report struct {
		Pages []map[string]any `json:"pages"`
	}

	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}

	checkExportGolden(t, "json.golden", out.Bytes())
}

func TestExportConfluence_ExitCodes(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, r *Runner, f *cmdFake)
		opts  exportOpts
		args  []string
		want  int
	}{
		{name: "ok", want: 0},
		{name: "failed_page", want: 1, setup: func(t *testing.T, r *Runner, f *cmdFake) {
			t.Helper()
			exportOnce(t, r)
			appendFile(t, filepath.Join(r.RepoRoot, "docs", "rfc", "0001-first-proposal.md"), "\nChanged.\n")
			f.failTitle = "RFC-0001: First proposal"
		}},
		{name: "strict_skip", opts: exportOpts{strict: true}, want: 1, setup: func(t *testing.T, _ *Runner, f *cmdFake) {
			t.Helper()
			f.pages["ADR-0001: A decision"] = &confluence.Page{ID: "99", Title: "ADR-0001: A decision", Version: 1}
		}},
		{name: "skip_without_strict", want: 0, setup: func(t *testing.T, _ *Runner, f *cmdFake) {
			t.Helper()
			f.pages["ADR-0001: A decision"] = &confluence.Page{ID: "99", Title: "ADR-0001: A decision", Version: 1}
		}},
		{name: "disabled", want: 2, setup: func(t *testing.T, r *Runner, _ *cmdFake) {
			t.Helper()
			r.Cfg.Sync.Confluence.Enabled = false
		}},
		{name: "no_credentials", want: 2, setup: func(t *testing.T, _ *Runner, _ *cmdFake) {
			t.Helper()
			t.Setenv(envAtlassianToken, "")
		}},
		{name: "rejected_credentials", want: 2, setup: func(t *testing.T, _ *Runner, f *cmdFake) {
			t.Helper()
			f.authFail = true
		}},
		{name: "unknown_type", opts: exportOpts{types: []string{"nope"}}, want: 2},
		{name: "unknown_id", args: []string{"RFC-0099"}, want: 2},
		{name: "bad_format", opts: exportOpts{format: "yaml"}, want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newCmdFake()
			r, _ := exportFixture(t, fake)

			if tt.setup != nil {
				tt.setup(t, r, fake)
			}

			if tt.opts.format == "" {
				tt.opts.format = formatText
			}

			err := r.exportConfluence(t.Context(), tt.opts, tt.args)
			if got := exitCode(err); got != tt.want {
				t.Errorf("exit %d (%v), want %d", got, err, tt.want)
			}

			if tt.name == "no_credentials" && fake.calls != 0 {
				t.Errorf("missing credentials still made %d calls", fake.calls)
			}
		})
	}
}

// exportOnce runs one successful export to seed the fake.
func exportOnce(t *testing.T, r *Runner) {
	t.Helper()

	saved := r.Out
	r.Out = io.Discard

	defer func() { r.Out = saved }()

	if err := r.exportConfluence(t.Context(), exportOpts{format: formatText}, nil); err != nil {
		t.Fatalf("seed export: %v", err)
	}
}

func TestExportConfluence_DryRunAndOut(t *testing.T) {
	fake := newCmdFake()
	r, out := exportFixture(t, fake)
	dir := filepath.Join(t.TempDir(), "out")

	if err := r.exportConfluence(t.Context(), exportOpts{format: formatText, dryRun: true, out: dir}, nil); err != nil {
		t.Fatalf("export: %v", err)
	}

	if len(fake.pages) != 0 {
		t.Errorf("dry run created %d pages", len(fake.pages))
	}

	if !strings.Contains(out.String(), "(dry run: nothing written)") {
		t.Errorf("report does not say dry run:\n%s", out)
	}

	for _, name := range []string{"docz", "RFCs", "RFC-0001", "ADRs", "ADR-0001"} {
		if _, err := os.Stat(filepath.Join(dir, name+".xhtml")); err != nil {
			t.Errorf("--out did not write %s.xhtml: %v", name, err)
		}
	}
}

func TestExportConfluence_OfflineDryRun(t *testing.T) {
	fake := newCmdFake()
	r, out := exportFixture(t, fake)
	t.Setenv(envAtlassianEmail, "")

	dir := t.TempDir()
	opts := exportOpts{format: formatText, dryRun: true, out: dir}
	if err := r.exportConfluence(t.Context(), opts, []string{"docs/rfc/0001-first-proposal.md"}); err != nil {
		t.Fatalf("offline export: %v", err)
	}

	if fake.calls != 0 {
		t.Errorf("an offline run called the client %d times", fake.calls)
	}

	if _, err := os.Stat(filepath.Join(dir, "RFC-0001.xhtml")); err != nil {
		t.Errorf("offline render wrote no page: %v\n%s", err, out)
	}
}

func TestExportConfluence_VerboseReachesTheLogger(t *testing.T) {
	r, _ := exportFixture(t, newCmdFake())

	var logs bytes.Buffer
	r.Logger = slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))

	ctx := confluence.WithHooks(t.Context(), r.exportHooks())
	if err := r.exportConfluence(ctx, exportOpts{format: formatText}, nil); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(logs.String(), `msg="page done" title="RFC-0001: First proposal" action=created`) {
		t.Errorf("no page line in the debug log:\n%s", logs.String())
	}
}

func TestGitHubURL(t *testing.T) {
	for in, want := range map[string]string{
		"git@github.com:o/r.git":       "https://github.com/o/r",
		"ssh://git@github.com/o/r":     "https://github.com/o/r",
		"https://github.com/o/r.git":   "https://github.com/o/r",
		"https://github.com/o/r/":      "https://github.com/o/r",
		"https://gitlab.com/o/r.git":   "",
		"":                             "",
		"https://github.com/o/r/extra": "",
	} {
		if got := githubURL(in); got != want {
			t.Errorf("githubURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBlobResolver(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "justfile"), nil, 0o600); err != nil {
		t.Fatal(err)
	}

	res := blobResolver("https://github.com/o/r", "main", root)

	if got := res("docs/adr/0001-x.md", "../../justfile#L3").URL; got != "https://github.com/o/r/blob/main/justfile#L3" {
		t.Errorf("got %q", got)
	}

	if got := res("docs/adr/0001-x.md", "../../../outside"); got != (confluence.LinkTarget{}) {
		t.Errorf("a link out of the repository resolved to %+v", got)
	}

	if got := res("docs/adr/0001-x.md", "0009-missing.md"); got != (confluence.LinkTarget{}) {
		t.Errorf("a link to a missing file resolved to %+v", got)
	}

	if blobResolver("", "main", root) != nil {
		t.Error("no remote should mean no resolver")
	}
}
