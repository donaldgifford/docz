package ingest

import (
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"path"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/donaldgifford/docz-api/internal/store"
	doczcfg "github.com/donaldgifford/docz/pkg/doczcore/config"
	doczparse "github.com/donaldgifford/docz/pkg/doczcore/docparse"
	doczdoc "github.com/donaldgifford/docz/pkg/doczcore/document"
)

// buildPages maps fetched blobs to the page rows an enabled api: block
// publishes (DESIGN-0004's six-rule classifier over DESIGN-0011's consumption
// rule). It runs beside buildDocuments over the same widened blob set; a
// dormant block returns nil, which makes the reconcile delete every existing
// page row (desired state).
//
// cfg is the authoritative post-Load config: normalized, validated, landing
// page backfilled. All path comparisons are exact-byte — git is
// case-sensitive, and serving anything but exact bytes reopens the aliasing
// problems docz's validator closed.
func buildPages(cfg *doczcfg.Config, blobs []BlobEntry) []store.PageInput {
	if !cfg.API.Enabled {
		return nil
	}
	c := newPageClassifier(cfg)
	c.claimReadmeDirs(blobs)

	// The docs_dir namespace claims its published paths first: OQ-1a makes
	// the docs_dir-derived page the deterministic winner of a cross-namespace
	// collision, so additional_docs is classified second against `taken`.
	pages := make([]store.PageInput, 0, len(blobs))
	taken := make(map[string]string, len(blobs)) // published path -> winning repo path
	blobByPath := make(map[string]*BlobEntry, len(blobs))
	for i := range blobs {
		blob := &blobs[i]
		blobByPath[blob.Path] = blob
		published, ok := c.classifyDocsDir(blob.Path)
		if !ok {
			continue
		}
		// Blob paths come straight from the git tree, which docz's config
		// validation never sees — a crafted tree can carry dot segments or
		// control bytes. Ingest is the single writer, so reject hostile keys
		// here rather than trusting every future reader of the table.
		if !validPublishedPath(published) {
			slog.Warn("skipping blob with an unsafe tree path", "path", blob.Path)
			continue
		}
		pages = append(pages, pageInput(blob, published))
		taken[published] = blob.Path
	}

	// The additional_docs pass iterates the config (validator-clean paths),
	// not the blobs, so an entry whose file is absent at HEAD is skipped AND
	// reported — R10: docz never checked existence; that duty is the
	// consumer's.
	for _, entry := range cfg.API.AdditionalDocs {
		if !strings.HasSuffix(entry, ".md") {
			// DESIGN-0011 clause 1: markdown only — skip and report. Checked
			// before absence: the fetch skips non-markdown entries too.
			slog.Warn("skipping non-markdown additional_docs entry", "path", entry)
			continue
		}
		blob, ok := blobByPath[entry]
		if !ok {
			slog.Warn("additional_docs entry absent at HEAD", "path", entry)
			continue
		}
		if winner, collides := taken[entry]; collides {
			slog.Warn("additional_docs entry collides with a docs_dir page; the docs_dir page wins",
				"published_path", entry, "docs_dir_file", winner)
			continue
		}
		pages = append(pages, pageInput(blob, entry))
	}
	return pages
}

// validPublishedPath reports whether a published path derived from a git
// tree is a lookup-safe key: relative, forward-slash separated, no "." /
// ".." / empty segments, no backslashes, no control bytes — the same rules
// the serve layer re-applies to decoded URL paths on read.
func validPublishedPath(p string) bool {
	if p == "" || strings.HasPrefix(p, "/") || strings.ContainsRune(p, '\\') {
		return false
	}
	for _, r := range p {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return false
		}
	}
	return true
}

// pageClassifier holds the enabled api: block's resolved path sets, all in
// repo-relative terms.
type pageClassifier struct {
	docsDir    string          // cleaned docs_dir ("docs")
	landing    string          // resolved landing page (repo-relative)
	exclude    []string        // repo-relative excluded prefixes, templates included
	typeDirs   []string        // enabled types' dirs (repo-relative)
	readmeDirs map[string]bool // docs_dir-relative dirs claimed by a README.md
}

func newPageClassifier(cfg *doczcfg.Config) *pageClassifier {
	docsDir := cleanRepoDir(cfg.DocsDir)

	c := &pageClassifier{
		docsDir:    docsDir,
		landing:    cfg.API.LandingPage,
		exclude:    []string{path.Join(docsDir, doczcfg.TemplatesDir)},
		readmeDirs: make(map[string]bool),
	}
	for _, entry := range cfg.API.Exclude {
		c.exclude = append(c.exclude, path.Join(docsDir, entry))
	}
	for _, name := range cfg.EnabledTypes() {
		c.typeDirs = append(c.typeDirs, filepath.ToSlash(cfg.TypeDir(name)))
	}
	return c
}

// claimReadmeDirs records which docs_dir-relative directories own a README.md
// before classification, so the README-wins-over-index precedence (INV-0008
// 5a) can be judged per blob without ordering effects. Excluded READMEs claim
// nothing — exclusion is judged first, so an excluded README leaves a lone
// index.md as that directory's page.
func (c *pageClassifier) claimReadmeDirs(blobs []BlobEntry) {
	for i := range blobs {
		p := blobs[i].Path
		if !underDir(p, c.docsDir) || c.excluded(p) || c.typeDirOf(p) != "" {
			continue
		}
		rel := strings.TrimPrefix(p, c.docsDir+"/")
		if dir := path.Dir(rel); dir != "." && path.Base(rel) == readmeName {
			c.readmeDirs[dir] = true
		}
	}
}

// classifyDocsDir maps one blob to its published path within the docs_dir
// namespace, or reports false for a blob that publishes nothing there
// (skipped, excluded, a docz document, or outside docs_dir entirely).
func (c *pageClassifier) classifyDocsDir(p string) (string, bool) {
	// Rule 1: the landing page is the repo row's cache, never a page row.
	if p == c.landing {
		return "", false
	}
	// Rule 2 (over-fetch guard): outside docs_dir only additional_docs
	// publishes, and that namespace is classified separately.
	if !underDir(p, c.docsDir) || !strings.HasSuffix(p, ".md") {
		return "", false
	}
	// Rule 3: templates/ and api.exclude prefixes are never published.
	if c.excluded(p) {
		return "", false
	}
	// Rule 4: type dirs publish nothing (IMPL-0009, amending DESIGN-0004 and
	// docz DESIGN-0011 clause 2). The consumer owns the type surface — it
	// synthesizes /:owner/:repo/:type from the live document list — so
	// publishing the dir's docz-generated README.md as a page only
	// duplicated that surface at a second URL. IsDoczFile matches stay silent
	// (buildDocuments's business), and so does the README: docz writes it
	// there on every `docz update`, so warning about it would fire on correct
	// configuration. Anything else in a docz-managed namespace is more likely
	// a mistake than an intent: skip + Warn (DESIGN-0011 rule 3).
	if td := c.typeDirOf(p); td != "" {
		// The silence is for the dir's OWN README.md — the file `docz update`
		// regenerates. A README nested deeper (docs/rfc/sub/README.md) is a
		// human's misplaced directory, so it stays a reported stray like the
		// index.md beside it.
		if p != td+"/"+readmeName && !doczdoc.IsDoczFile(path.Base(p)) {
			slog.Warn("skipping stray file in a docz type directory", "path", p)
		}
		return "", false
	}
	// Rules 5: a directory's README.md is that directory's page; a lone
	// index.md serves the same way; when both exist README wins and the
	// index stays path-addressed. Everything else is a file page at its
	// docs_dir-relative path. The docs_dir root is the repo home (rule 1's
	// territory), so it never forms a directory page.
	rel := strings.TrimPrefix(p, c.docsDir+"/")
	dir, base := path.Dir(rel), path.Base(rel)
	if dir != "." {
		if base == readmeName {
			return dir, true
		}
		if base == doczcfg.APILandingFileName && !c.readmeDirs[dir] {
			return dir, true
		}
	}
	return rel, true
}

// excluded reports whether p falls under any excluded prefix (or names one
// exactly).
func (c *pageClassifier) excluded(p string) bool {
	for _, prefix := range c.exclude {
		if p == prefix || underDir(p, prefix) {
			return true
		}
	}
	return false
}

// typeDirOf returns the enabled type directory p lives under, or "" when p is
// outside every type dir.
func (c *pageClassifier) typeDirOf(p string) string {
	for _, td := range c.typeDirs {
		if underDir(p, td) {
			return td
		}
	}
	return ""
}

// readmeName is the directory-page filename (DESIGN-0011: a directory's
// README.md is that directory's page).
const readmeName = "README.md"

// underDir reports whether p is inside dir (exact-byte, slash-separated).
func underDir(p, dir string) bool {
	return strings.HasPrefix(p, dir+"/")
}

// cleanRepoDir settles a directory path into the form the classifier compares
// against: whitespace and leading "./" stripped (docz's normalizeRepoPath),
// then cleaned so "docs/" and "docs" compare equal.
func cleanRepoDir(dir string) string {
	dir = strings.TrimSpace(dir)
	for strings.HasPrefix(dir, "./") {
		dir = dir[len("./"):]
	}
	return path.Clean(dir)
}

// pageInput builds the store row for one classified page. The title is
// docparse.Title's first-H1 read, with docz-api's own fallback (INV-0007's
// amendment blesses an independent implementation as presentation): the
// title-cased filename for a file page, the title-cased directory name for a
// directory page — a directory page's published path has no .md suffix, so
// one fallback rule covers both.
func pageInput(blob *BlobEntry, published string) store.PageInput {
	sum := sha256.Sum256(blob.Content)
	title := doczparse.Title(blob.Content)
	if title == "" {
		title = fallbackTitle(path.Base(published))
	}
	return store.PageInput{
		Path:        published,
		RepoPath:    blob.Path,
		Title:       title,
		GitSHA:      blob.GitSHA,
		ContentHash: hex.EncodeToString(sum[:]),
		RawMD:       string(blob.Content),
	}
}

// fallbackTitle turns a file or directory basename into a display title:
// extension stripped, dashes and underscores read as spaces, each word
// capitalized ("getting-started.md" → "Getting Started").
func fallbackTitle(base string) string {
	name := strings.TrimSuffix(base, path.Ext(base))
	words := strings.FieldsFunc(name, func(r rune) bool {
		return r == '-' || r == '_' || r == ' '
	})
	for i, w := range words {
		runes := []rune(w)
		runes[0] = unicode.ToUpper(runes[0])
		words[i] = string(runes)
	}
	if len(words) == 0 {
		return base
	}
	return strings.Join(words, " ")
}

// apiFields returns the RepoInput api pair from the authoritative post-Load
// config: the resolved landing page and normalized additional_docs when the
// block is enabled, empty otherwise — which makes the reconcile null both
// columns (desired state, the changelogFile rule).
func apiFields(cfg *doczcfg.Config) (landing string, additional []string) {
	if !cfg.API.Enabled {
		return "", nil
	}
	return cfg.API.LandingPage, cfg.API.AdditionalDocs
}
