package wiki

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
)

// testConfig returns a default config with repo-relative paths, so the
// tests exercise the root joining Init and UpdateNav do rather than
// pre-resolving every path the way cmd's own tests have to.
func testConfig() *config.Config {
	cfg := config.DefaultConfig()

	return &cfg
}

// tempRoot returns a repo root whose directory name is known, so a test
// can assert on the site name derived from it (t.TempDir()'s own base
// name is a counter).
func tempRoot(t *testing.T, name string) string {
	t.Helper()

	root := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(root, 0o750); err != nil {
		t.Fatal(err)
	}

	return root
}

// navDocsTree builds docs/ with an index, an RFC directory holding a
// README and one document, and an ADR directory holding a README: three
// top-level nav entries and four pages.
func navDocsTree(t *testing.T, root string) {
	t.Helper()

	mkdirAll(t, root, filepath.Join("docs", "rfc"))
	mkdirAll(t, root, filepath.Join("docs", "adr"))
	writeFile(t, root, filepath.Join("docs", "index.md"), "# Home\n")
	writeFile(t, root, filepath.Join("docs", "rfc", "README.md"), "# RFCs\n")
	writeFile(t, root, filepath.Join("docs", "adr", "README.md"), "# ADRs\n")
	writeFile(
		t, root, filepath.Join("docs", "rfc", "0001-test.md"),
		"---\nid: RFC-0001\ntitle: \"Test RFC\"\nstatus: Draft\n"+
			"author: Test\ncreated: 2026-01-01\n---\n",
	)
}

func readFileString(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return string(data)
}

// entryTitles flattens the top-level nav titles, which is the order a
// user hand-edits and therefore the order the tests assert on.
func entryTitles(entries []NavEntry) []string {
	titles := make([]string, 0, len(entries))
	for i := range entries {
		titles = append(titles, entries[i].Title)
	}

	return titles
}

func TestAction_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		action Action
		want   string
	}{
		{Created, "created"},
		{Skipped, "skipped"},
		{Overwritten, "overwritten"},
		{Action(0), "unknown"},
		{Action(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()

			if got := tt.action.String(); got != tt.want {
				t.Errorf("Action(%d).String() = %q, want %q", tt.action, got, tt.want)
			}
		})
	}
}

func TestInit_EmptyRepo(t *testing.T) {
	t.Parallel()

	root := tempRoot(t, "empty-repo")

	report, err := Init(t.Context(), root, testConfig(), InitOptions{})
	if err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	wantMkDocs := filepath.Join(root, "mkdocs.yml")
	if report.MkDocsPath != wantMkDocs {
		t.Errorf("MkDocsPath = %q, want %q", report.MkDocsPath, wantMkDocs)
	}

	wantIndex := filepath.Join(root, "docs", "index.md")
	if report.IndexPath != wantIndex {
		t.Errorf("IndexPath = %q, want %q", report.IndexPath, wantIndex)
	}

	if report.MkDocs != Created {
		t.Errorf("MkDocs = %v, want created", report.MkDocs)
	}

	if report.Index != Created {
		t.Errorf("Index = %v, want created", report.Index)
	}

	mkdocs := readFileString(t, wantMkDocs)
	for _, want := range []string{"site_name: empty-repo", "techdocs-core", "nav:"} {
		if !strings.Contains(mkdocs, want) {
			t.Errorf("mkdocs.yml should contain %q, got:\n%s", want, mkdocs)
		}
	}

	index := readFileString(t, wantIndex)
	for _, want := range []string{"RFCs", "ADRs", "Design"} {
		if !strings.Contains(index, want) {
			t.Errorf("index.md should contain %q, got:\n%s", want, index)
		}
	}
}

func TestInit_SkipsExistingWithoutForce(t *testing.T) {
	t.Parallel()

	root := tempRoot(t, "skip-repo")
	cfg := testConfig()

	if _, err := Init(t.Context(), root, cfg, InitOptions{}); err != nil {
		t.Fatalf("first Init() error: %v", err)
	}

	// Sentinel content, so the assertion is that the second run left the
	// files alone and not merely that it rewrote them identically.
	mkdocsPath := filepath.Join(root, "mkdocs.yml")
	indexPath := filepath.Join(root, "docs", "index.md")
	const (
		mkdocsSentinel = "site_name: hand written\n"
		indexSentinel  = "# Hand written\n"
	)

	writeFile(t, root, "mkdocs.yml", mkdocsSentinel)
	writeFile(t, root, filepath.Join("docs", "index.md"), indexSentinel)

	report, err := Init(t.Context(), root, cfg, InitOptions{})
	if err != nil {
		t.Fatalf("second Init() error: %v", err)
	}

	if report.MkDocs != Skipped {
		t.Errorf("MkDocs = %v, want skipped", report.MkDocs)
	}

	if report.Index != Skipped {
		t.Errorf("Index = %v, want skipped", report.Index)
	}

	if got := readFileString(t, mkdocsPath); got != mkdocsSentinel {
		t.Errorf("mkdocs.yml was modified: got %q, want %q", got, mkdocsSentinel)
	}

	if got := readFileString(t, indexPath); got != indexSentinel {
		t.Errorf("index.md was modified: got %q, want %q", got, indexSentinel)
	}
}

func TestInit_ForceOverwrites(t *testing.T) {
	t.Parallel()

	root := tempRoot(t, "force-repo")
	cfg := testConfig()

	if _, err := Init(t.Context(), root, cfg, InitOptions{}); err != nil {
		t.Fatalf("first Init() error: %v", err)
	}

	writeFile(t, root, "mkdocs.yml", "site_name: hand written\n")
	writeFile(t, root, filepath.Join("docs", "index.md"), "# Hand written\n")

	report, err := Init(t.Context(), root, cfg, InitOptions{
		SiteName: "Forced Name",
		Force:    true,
	})
	if err != nil {
		t.Fatalf("forced Init() error: %v", err)
	}

	if report.MkDocs != Overwritten {
		t.Errorf("MkDocs = %v, want overwritten", report.MkDocs)
	}

	if report.Index != Overwritten {
		t.Errorf("Index = %v, want overwritten", report.Index)
	}

	mkdocs := readFileString(t, report.MkDocsPath)
	if !strings.Contains(mkdocs, "site_name: Forced Name") {
		t.Errorf("mkdocs.yml was not overwritten, got:\n%s", mkdocs)
	}

	index := readFileString(t, report.IndexPath)
	if strings.Contains(index, "Hand written") {
		t.Errorf("index.md was not overwritten, got:\n%s", index)
	}
}

func TestInit_SiteNameSources(t *testing.T) {
	t.Parallel()

	// Two tiers, not three: config.WikiConfig carries no site name, so
	// InitOptions and the root directory name are the whole chain. The
	// placeholder third tier (an empty root) is covered by
	// TestResolveSiteName, which needs no files.
	tests := []struct {
		name     string
		rootName string
		opts     InitOptions
		want     string
	}{
		{
			name:     "from options",
			rootName: "ignored-dir-name",
			opts:     InitOptions{SiteName: "My Service"},
			want:     "My Service",
		},
		{
			name:     "from root directory name",
			rootName: "my-repo",
			opts:     InitOptions{},
			want:     "my-repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := tempRoot(t, tt.rootName)

			report, err := Init(t.Context(), root, testConfig(), tt.opts)
			if err != nil {
				t.Fatalf("Init() error: %v", err)
			}

			mkdocs := readFileString(t, report.MkDocsPath)
			if !strings.Contains(mkdocs, "site_name: "+tt.want+"\n") {
				t.Errorf("mkdocs.yml should name site %q, got:\n%s", tt.want, mkdocs)
			}

			// The description follows the resolved name unless the
			// caller sets one, which is what cmd/wiki.go does today.
			wantDesc := "site_description: Documentation for " + tt.want + "\n"
			if !strings.Contains(mkdocs, wantDesc) {
				t.Errorf("mkdocs.yml should describe site as %q, got:\n%s", wantDesc, mkdocs)
			}

			index := readFileString(t, report.IndexPath)
			if !strings.Contains(index, tt.want) {
				t.Errorf("index.md should name site %q, got:\n%s", tt.want, index)
			}
		})
	}
}

func TestInit_SiteDescriptionFromOptions(t *testing.T) {
	t.Parallel()

	root := tempRoot(t, "desc-repo")

	report, err := Init(t.Context(), root, testConfig(), InitOptions{
		SiteDescription: "Custom description",
	})
	if err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	mkdocs := readFileString(t, report.MkDocsPath)
	if !strings.Contains(mkdocs, "site_description: Custom description\n") {
		t.Errorf("mkdocs.yml should carry the caller's description, got:\n%s", mkdocs)
	}
}

func TestInit_OptionsOverrideConfig(t *testing.T) {
	t.Parallel()

	root := tempRoot(t, "override-repo")
	cfg := testConfig()
	cfg.Wiki.RepoURL = "https://example.com/from-config"
	cfg.Wiki.SiteURL = "https://example.com/config-site"
	cfg.Wiki.Theme = "readthedocs"

	report, err := Init(t.Context(), root, cfg, InitOptions{
		RepoURL: "https://example.com/from-options",
		Theme:   "material",
	})
	if err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	mkdocs := readFileString(t, report.MkDocsPath)
	for _, want := range []string{
		"repo_url: https://example.com/from-options",
		"site_url: https://example.com/config-site",
		"theme: material",
	} {
		if !strings.Contains(mkdocs, want) {
			t.Errorf("mkdocs.yml should contain %q, got:\n%s", want, mkdocs)
		}
	}
}

func TestInit_ContextCancelled(t *testing.T) {
	t.Parallel()

	root := tempRoot(t, "cancelled-repo")

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := Init(ctx, root, testConfig(), InitOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Init() error = %v, want context.Canceled", err)
	}

	for _, path := range []string{
		filepath.Join(root, "mkdocs.yml"),
		filepath.Join(root, "docs", "index.md"),
	} {
		if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("%s should not have been written", path)
		}
	}
}

func TestInitThenUpdateNav_Composes(t *testing.T) {
	t.Parallel()

	root := tempRoot(t, "compose-repo")
	cfg := testConfig()

	if _, err := Init(t.Context(), root, cfg, InitOptions{}); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	report, err := UpdateNav(t.Context(), root, cfg, NavOptions{})
	if err != nil {
		t.Fatalf("UpdateNav() error: %v", err)
	}

	// Init writes docs/index.md and nothing else, so the only page is Home.
	if report.Pages != 1 {
		t.Errorf("Pages = %d, want 1", report.Pages)
	}

	if got := entryTitles(report.Entries); !reflect.DeepEqual(got, []string{"Home"}) {
		t.Errorf("titles = %v, want [Home]", got)
	}

	if !report.Written {
		t.Error("Written = false, want true")
	}
}

func TestUpdateNav_WritesNav(t *testing.T) {
	t.Parallel()

	root := tempRoot(t, "nav-repo")
	navDocsTree(t, root)
	writeFile(
		t, root, "mkdocs.yml",
		"site_name: test\nplugins:\n    - techdocs-core\nnav:\n    - Home: index.md\n",
	)

	report, err := UpdateNav(t.Context(), root, testConfig(), NavOptions{})
	if err != nil {
		t.Fatalf("UpdateNav() error: %v", err)
	}

	if report.Path != filepath.Join(root, "mkdocs.yml") {
		t.Errorf("Path = %q, want %q", report.Path, filepath.Join(root, "mkdocs.yml"))
	}

	if !report.Written {
		t.Error("Written = false, want true")
	}

	// Home + adr/README.md + rfc/README.md + rfc/0001-test.md.
	if report.Pages != 4 {
		t.Errorf("Pages = %d, want 4", report.Pages)
	}

	wantTitles := []string{"Home", "ADRs", "RFCs"}
	if got := entryTitles(report.Entries); !reflect.DeepEqual(got, wantTitles) {
		t.Errorf("titles = %v, want %v", got, wantTitles)
	}

	mkdocs := readFileString(t, report.Path)
	for _, want := range []string{
		"site_name: test",
		"techdocs-core",
		"rfc/0001-test.md",
		"RFC-0001: Test RFC",
	} {
		if !strings.Contains(mkdocs, want) {
			t.Errorf("mkdocs.yml should contain %q, got:\n%s", want, mkdocs)
		}
	}
}

func TestUpdateNav_DryRunLeavesFileAlone(t *testing.T) {
	t.Parallel()

	root := tempRoot(t, "dryrun-repo")
	navDocsTree(t, root)

	const mkdocsContent = "site_name: test\nnav:\n    - Home: index.md\n"

	writeFile(t, root, "mkdocs.yml", mkdocsContent)

	cfg := testConfig()

	dry, err := UpdateNav(t.Context(), root, cfg, NavOptions{DryRun: true})
	if err != nil {
		t.Fatalf("UpdateNav(dry-run) error: %v", err)
	}

	if dry.Written {
		t.Error("Written = true on a dry run, want false")
	}

	if dry.Pages != 4 {
		t.Errorf("Pages = %d, want 4", dry.Pages)
	}

	if got := readFileString(t, dry.Path); got != mkdocsContent {
		t.Errorf("mkdocs.yml was modified by a dry run:\n%s", got)
	}

	// The same nav a live run would write, entry for entry.
	live, err := UpdateNav(t.Context(), root, cfg, NavOptions{})
	if err != nil {
		t.Fatalf("UpdateNav() error: %v", err)
	}

	if !reflect.DeepEqual(dry.Entries, live.Entries) {
		t.Errorf("dry-run entries = %v, live entries = %v", dry.Entries, live.Entries)
	}

	if dry.Pages != live.Pages {
		t.Errorf("dry-run pages = %d, live pages = %d", dry.Pages, live.Pages)
	}
}

func TestUpdateNav_PreservesExistingOrder(t *testing.T) {
	t.Parallel()

	root := tempRoot(t, "order-repo")

	for _, name := range []string{"rfc", "adr", "design"} {
		mkdirAll(t, root, filepath.Join("docs", name))
		writeFile(t, root, filepath.Join("docs", name, "README.md"), "# "+name+"\n")
	}

	writeFile(t, root, filepath.Join("docs", "index.md"), "# Home\n")

	// A hand-ordered nav with ADRs before RFCs, and no Design section yet.
	writeFile(
		t, root, "mkdocs.yml",
		"site_name: test\nnav:\n"+
			"    - Home: index.md\n"+
			"    - ADRs:\n        - Overview: adr/README.md\n"+
			"    - RFCs:\n        - Overview: rfc/README.md\n",
	)

	report, err := UpdateNav(t.Context(), root, testConfig(), NavOptions{})
	if err != nil {
		t.Fatalf("UpdateNav() error: %v", err)
	}

	// Existing sections keep their order; Design is new and appended.
	wantTitles := []string{"Home", "ADRs", "RFCs", "Design"}
	if got := entryTitles(report.Entries); !reflect.DeepEqual(got, wantTitles) {
		t.Errorf("titles = %v, want %v", got, wantTitles)
	}

	mkdocs := readFileString(t, report.Path)

	adrs := strings.Index(mkdocs, "ADRs")
	rfcs := strings.Index(mkdocs, "RFCs")
	design := strings.Index(mkdocs, "Design")

	if adrs == -1 || rfcs == -1 || design == -1 {
		t.Fatalf("mkdocs.yml missing a section:\n%s", mkdocs)
	}

	if adrs >= rfcs || rfcs >= design {
		t.Errorf("nav order should be ADRs, RFCs, Design, got:\n%s", mkdocs)
	}
}

func TestUpdateNav_MissingMkDocs(t *testing.T) {
	t.Parallel()

	root := tempRoot(t, "missing-repo")

	_, err := UpdateNav(t.Context(), root, testConfig(), NavOptions{})
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("UpdateNav() error = %v, want one wrapping fs.ErrNotExist", err)
	}
}

func TestUpdateNav_ContextCancelled(t *testing.T) {
	t.Parallel()

	root := tempRoot(t, "nav-cancelled-repo")
	navDocsTree(t, root)

	const mkdocsContent = "site_name: test\nnav:\n    - Home: index.md\n"

	writeFile(t, root, "mkdocs.yml", mkdocsContent)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	report, err := UpdateNav(ctx, root, testConfig(), NavOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("UpdateNav() error = %v, want context.Canceled", err)
	}

	if report.Written {
		t.Error("Written = true after cancellation, want false")
	}

	if got := readFileString(t, filepath.Join(root, "mkdocs.yml")); got != mkdocsContent {
		t.Errorf("mkdocs.yml was modified after cancellation:\n%s", got)
	}
}

func TestResolveSiteName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		root string
		opts InitOptions
		want string
	}{
		{
			name: "options win over root",
			root: filepath.Join("tmp", "some-repo"),
			opts: InitOptions{SiteName: "Chosen"},
			want: "Chosen",
		},
		{
			name: "root directory name",
			root: filepath.Join("tmp", "some-repo"),
			want: "some-repo",
		},
		{
			name: "placeholder when neither is set",
			want: defaultSiteName,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts := tt.opts
			if got := resolveSiteName(tt.root, &opts); got != tt.want {
				t.Errorf("resolveSiteName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUnderRoot(t *testing.T) {
	t.Parallel()

	root := filepath.Join(string(filepath.Separator), "repo")

	tests := []struct {
		name string
		root string
		path string
		want string
	}{
		{
			name: "relative path joins under root",
			root: root,
			path: "docs",
			want: filepath.Join(root, "docs"),
		},
		{
			name: "absolute path is left alone",
			root: root,
			path: filepath.Join(string(filepath.Separator), "elsewhere", "docs"),
			want: filepath.Join(string(filepath.Separator), "elsewhere", "docs"),
		},
		{
			name: "empty root leaves the path alone",
			path: "docs",
			want: "docs",
		},
		{
			name: "empty path stays empty",
			root: root,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := underRoot(tt.root, tt.path); got != tt.want {
				t.Errorf("underRoot(%q, %q) = %q, want %q", tt.root, tt.path, got, tt.want)
			}
		})
	}
}
