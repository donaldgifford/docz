package ingest

import (
	"slices"
	"testing"

	doczcfg "github.com/donaldgifford/docz/pkg/doczcore/config"
)

// loadPagesConfig loads + validates a .docz.yaml body hermetically, the
// post-Load shape buildPages consumes (landing backfilled, paths normalized).
func loadPagesConfig(t *testing.T, yaml string) doczcfg.Config {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	cfg, err := loadConfig([]byte(yaml))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if _, err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	return cfg
}

// pblob builds a BlobEntry for classifier tests.
func pblob(path, content string) BlobEntry {
	return BlobEntry{Path: path, GitSHA: "sha-" + path, Content: []byte(content)}
}

// pagesConfig enables one docz type (rfc), an exclusion, and an additional
// doc — the fixture the classifier table runs against. The landing page
// backfills to docs/index.md.
const pagesConfig = `docs_dir: docs
types:
  rfc:
    enabled: true
    dir: rfc
    id_prefix: RFC
    id_width: 4
    statuses: [Draft]
api:
  enabled: true
  exclude: [drafts]
  additional_docs: [CONTRIBUTING.md]
`

// TestBuildPagesClassifier drives every DESIGN-0004 classifier rule through
// one widened blob set and asserts on the published-path namespace.
func TestBuildPagesClassifier(t *testing.T) {
	cfg := loadPagesConfig(t, pagesConfig)

	blobs := []BlobEntry{
		pblob("docs/index.md", "# Home"),                          // rule 1: landing → repo row, not a page
		pblob("docs/rfc/README.md", "# RFC Index"),                // rule 4: type-dir README kept (the predicted bug)
		pblob("docs/rfc/0001-intro.md", "---\nid: RFC-0001\n---"), // rule 4: docz doc → document pipeline
		pblob("docs/rfc/notes.md", "# Stray"),                     // rule 4: stray in a type dir → skip + Warn
		pblob("docs/impl/README.md", "# Impl Notes"),              // rule 5: README wins the directory…
		pblob("docs/impl/index.md", "# Impl Index"),               // …and the loser index is path-addressed
		pblob("docs/guides/index.md", "just prose\n"),             // rule 5: lone index serves the directory
		pblob("docs/examples/example1.md", "# Example One"),       // rule 5: file page, extension kept
		pblob("docs/getting-started.md", "no heading here\n"),     // rule 5: docs_dir-root file page
		pblob("docs/templates/rfc.md", "template"),                // rule 3: templates always excluded
		pblob("docs/drafts/wip.md", "# WIP"),                      // rule 3: api.exclude prefix
		pblob("CONTRIBUTING.md", "# Contributing"),                // rule 6: additional doc, repo-relative
		pblob("README.md", "# Root"),                              // rule 2: outside docs_dir, not additional → skip
	}

	pages := buildPages(&cfg, blobs)

	byPath := make(map[string]int, len(pages))
	got := make([]string, 0, len(pages))
	for i := range pages {
		byPath[pages[i].Path] = i
		got = append(got, pages[i].Path)
	}
	want := []string{
		"rfc", "impl", "impl/index.md", "guides",
		"examples/example1.md", "getting-started.md", "CONTRIBUTING.md",
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("published paths = %v, want %v", got, want)
	}

	// Directory pages carry the directory address and their source file.
	rfc := pages[byPath["rfc"]]
	if rfc.RepoPath != "docs/rfc/README.md" || rfc.Title != "RFC Index" {
		t.Errorf("rfc page = {%q %q}, want the type-dir README", rfc.RepoPath, rfc.Title)
	}
	impl := pages[byPath["impl"]]
	if impl.RepoPath != "docs/impl/README.md" {
		t.Errorf("impl directory page from %q, want docs/impl/README.md (README wins)", impl.RepoPath)
	}

	// Title fallbacks: a no-H1 directory page takes the directory name; a
	// no-H1 file page takes its title-cased filename.
	if guides := pages[byPath["guides"]]; guides.Title != "Guides" {
		t.Errorf("lone-index directory title = %q, want Guides (fallback)", guides.Title)
	}
	if gs := pages[byPath["getting-started.md"]]; gs.Title != "Getting Started" {
		t.Errorf("file fallback title = %q, want Getting Started", gs.Title)
	}

	// Content plumbing: hash + raw markdown round-trip.
	ex := pages[byPath["examples/example1.md"]]
	if ex.Title != "Example One" || ex.RawMD != "# Example One" || ex.GitSHA != "sha-docs/examples/example1.md" {
		t.Errorf("example page = %+v, want title/raw/sha from the blob", ex)
	}
	if ex.ContentHash == "" || ex.ContentHash == pages[byPath["rfc"]].ContentHash {
		t.Error("ContentHash missing or not content-derived")
	}
}

func TestBuildPagesDormantBlockIsNil(t *testing.T) {
	cfg := loadPagesConfig(t, "docs_dir: docs\napi:\n  enabled: false\n")
	if pages := buildPages(&cfg, []BlobEntry{pblob("docs/guides/setup.md", "# Setup")}); pages != nil {
		t.Errorf("buildPages(dormant) = %v, want nil (reconcile deletes all rows)", pages)
	}
}

// TestBuildPagesCollision pins design OQ-1a: when a docs_dir-derived
// published path and an additional_docs entry collide, the docs_dir page
// wins deterministically and the additional doc is skipped.
func TestBuildPagesCollision(t *testing.T) {
	cfg := loadPagesConfig(t, `docs_dir: docs
api:
  enabled: true
  additional_docs: [examples/a.md]
`)
	blobs := []BlobEntry{
		pblob("docs/examples/a.md", "# From docs_dir"),
		pblob("examples/a.md", "# From repo root"),
	}
	pages := buildPages(&cfg, blobs)
	if len(pages) != 1 {
		t.Fatalf("pages = %d, want 1 (collision drops the additional doc)", len(pages))
	}
	if pages[0].Path != "examples/a.md" || pages[0].RepoPath != "docs/examples/a.md" {
		t.Errorf("winner = {%q %q}, want the docs_dir file at the shared path",
			pages[0].Path, pages[0].RepoPath)
	}
}

// TestBuildPagesExcludedReadmeYieldsIndex proves exclusion is judged before
// the README claim: an excluded README leaves a lone index.md as that
// directory's page.
func TestBuildPagesExcludedReadmeYieldsIndex(t *testing.T) {
	cfg := loadPagesConfig(t, `docs_dir: docs
api:
  enabled: true
  exclude: [wiki/README.md]
`)
	blobs := []BlobEntry{
		pblob("docs/wiki/README.md", "# Excluded"),
		pblob("docs/wiki/index.md", "# Wiki Home"),
	}
	pages := buildPages(&cfg, blobs)
	if len(pages) != 1 || pages[0].Path != "wiki" || pages[0].RepoPath != "docs/wiki/index.md" {
		t.Fatalf("pages = %+v, want the lone index serving directory page wiki", pages)
	}
}

func TestBuildPagesSkipsNonMarkdownAdditionalDoc(t *testing.T) {
	cfg := loadPagesConfig(t, `docs_dir: docs
api:
  enabled: true
  additional_docs: [LICENSE]
`)
	if pages := buildPages(&cfg, []BlobEntry{pblob("LICENSE", "MIT")}); len(pages) != 0 {
		t.Errorf("pages = %+v, want none (markdown only, DESIGN-0011 clause 1)", pages)
	}
}

// TestBuildPagesSkipsAbsentAdditionalDoc pins the R10 skip-and-report duty:
// an entry whose file is absent at HEAD produces no page (and a Warn) rather
// than an error.
func TestBuildPagesSkipsAbsentAdditionalDoc(t *testing.T) {
	cfg := loadPagesConfig(t, `docs_dir: docs
api:
  enabled: true
  additional_docs: [MISSING.md]
`)
	if pages := buildPages(&cfg, nil); len(pages) != 0 {
		t.Errorf("pages = %+v, want none for an absent entry", pages)
	}
}

// TestBuildPagesRejectsHostileTreePaths proves the single-writer guard: blob
// paths come from the git tree, which docz's config validation never sees, so
// a crafted tree's dot segments and control bytes must never become store
// keys.
func TestBuildPagesRejectsHostileTreePaths(t *testing.T) {
	cfg := loadPagesConfig(t, "docs_dir: docs\napi:\n  enabled: true\n")
	blobs := []BlobEntry{
		pblob("docs/../secret.md", "# escape"),      // publishes "../secret.md" unguarded
		pblob("docs/./sneaky.md", "# dot"),          // "." segment
		pblob("docs/a\nb.md", "# control"),          // control byte
		pblob("docs/back\\slash.md", "# backslash"), // backslash
		pblob("docs/fine.md", "# Fine"),
	}
	pages := buildPages(&cfg, blobs)
	if len(pages) != 1 || pages[0].Path != "fine.md" {
		t.Fatalf("pages = %+v, want only fine.md (hostile tree paths rejected)", pages)
	}
}

func TestFallbackTitle(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"getting-started.md", "Getting Started"},
		{"api_reference.md", "Api Reference"},
		{"guides", "Guides"},
		{"README.md", "README"},
		{"a.md", "A"},
	}
	for _, tt := range tests {
		if got := fallbackTitle(tt.in); got != tt.want {
			t.Errorf("fallbackTitle(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestAPIFields(t *testing.T) {
	cfg := loadPagesConfig(t, `docs_dir: docs
api:
  enabled: true
  additional_docs: [CONTRIBUTING.md]
`)
	landing, additional := apiFields(&cfg)
	if landing != "docs/index.md" || !slices.Equal(additional, []string{"CONTRIBUTING.md"}) {
		t.Errorf("apiFields = (%q, %v), want the backfilled landing + entries", landing, additional)
	}

	cfg = loadPagesConfig(t, "docs_dir: docs\napi:\n  enabled: false\n  additional_docs: [X.md]\n")
	if landing, additional = apiFields(&cfg); landing != "" || additional != nil {
		t.Errorf("apiFields(dormant) = (%q, %v), want empty (reconcile nulls both)", landing, additional)
	}
}
