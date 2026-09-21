package wiki

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/doctemplate"
)

// defaultSiteName is the site name of last resort, used only when the
// caller supplies neither a name nor a root to take one from. It is the
// same value cmd/wiki.go falls back to.
const defaultSiteName = "my-project"

// Action names what Init did to one of the two files it owns.
//
// A file rather than a run: Init reports mkdocs.yml and the docs index
// separately, because they are independently present or absent and a
// caller wants to say which one it just wrote.
type Action int

const (
	// Created means the file was not there and Init wrote it.
	Created Action = iota + 1
	// Skipped means the file was already there and Force was not set, so
	// Init left it exactly as it found it.
	Skipped
	// Overwritten means the file was already there and Force was set, so
	// Init replaced it.
	Overwritten
)

// String returns the lower-case action name — "created", "skipped",
// "overwritten", or "unknown" for a zero or out-of-range value.
//
// One spelling of each action, so two consumers reporting the same run
// cannot word it differently. Everything else a caller prints around it
// is the caller's.
func (a Action) String() string {
	switch a {
	case Created:
		return "created"
	case Skipped:
		return "skipped"
	case Overwritten:
		return "overwritten"
	default:
		return "unknown"
	}
}

// InitOptions is the per-invocation input to Init. Every field is
// optional; the zero value initialises a wiki entirely from cfg and root.
//
// SiteName arrives already resolved. Deriving it from the git remote is
// an L4 dependency — the same call the author name is — and stays in the
// caller; Init's own fallbacks reach no further than cfg and root.
// RepoURL, SiteURL and Theme override their cfg.Wiki counterparts when
// set, which is how a flag beats a config file without Init knowing what
// a flag is.
type InitOptions struct {
	SiteName        string
	SiteDescription string
	RepoURL         string
	SiteURL         string
	Theme           string
	Force           bool
}

// InitReport is the outcome of Init: both paths it owns and what it did
// to each.
//
// Both, always — the paths are filled in before any work happens, so a
// report returned alongside an error still says which files were at
// stake, and an Action left zero says that file was never reached.
type InitReport struct {
	MkDocsPath string
	MkDocs     Action
	IndexPath  string
	Index      Action
}

// Init writes the two files a MkDocs wiki starts from: mkdocs.yml at
// cfg.Wiki.MkDocsPath, and the docs landing page at
// <cfg.DocsDir>/index.md rendered from doctemplate.ResolveWikiIndex.
//
// Both paths resolve under root, so the caller's repo root decides where
// they land and nothing here consults the process working directory.
//
// Each file is created when absent, left alone when present unless
// opts.Force is set, and rewritten when it is; the returned report says
// which of the three happened to each. A file already there is not an
// error at this layer — the caller decides whether a Skipped mkdocs.yml
// is worth refusing over, which is what `docz wiki init` does.
//
// Init does not touch the nav. CreateMkDocs leaves the placeholder
// `nav: - Home: index.md` behind, and a caller that wants the real tree
// calls UpdateNav next; that keeps the nav result in NavReport, where a
// caller can read the page count, instead of discarded inside this one.
//
// The context is checked before each file. Cancellation stops Init where
// it stands and returns the report so far with ctx.Err() — what was
// already written stays written.
//
// opts is taken by value because DESIGN-0014 §2.10 fixes the signature
// that way; at 88 bytes it is over gocritic's hugeParam threshold, so
// the helpers below take it by pointer.
//
//nolint:gocritic // hugeParam: the by-value signature is fixed by DESIGN-0014 §2.10.
func Init(
	ctx context.Context,
	root string,
	cfg *config.Config,
	opts InitOptions,
) (InitReport, error) {
	docsDir := underRoot(root, cfg.DocsDir)

	report := InitReport{
		MkDocsPath: underRoot(root, cfg.Wiki.MkDocsPath),
		IndexPath:  filepath.Join(docsDir, config.WikiIndexName),
	}

	if err := ctx.Err(); err != nil {
		return report, err
	}

	siteName := resolveSiteName(root, &opts)

	action, err := writeMkDocs(report.MkDocsPath, cfg, &opts, siteName)
	if err != nil {
		return report, err
	}

	report.MkDocs = action

	if err := ctx.Err(); err != nil {
		return report, err
	}

	action, err = writeIndex(report.IndexPath, docsDir, cfg, siteName, opts.Force)
	if err != nil {
		return report, err
	}

	report.Index = action

	return report, nil
}

// NavOptions is the per-invocation input to UpdateNav.
type NavOptions struct {
	// DryRun builds the nav and reports it without writing mkdocs.yml.
	DryRun bool
}

// NavReport is the outcome of UpdateNav: the nav it built, and whether
// that nav reached the file.
//
// Entries is the whole tree rather than a rendered string, so a caller
// can print it however it likes — the indented tree `docz wiki update
// --dry-run` shows is one rendering of this field, not something this
// package produces.
type NavReport struct {
	Path    string
	Entries []NavEntry
	Pages   int
	Written bool
}

// UpdateNav rebuilds the nav section of mkdocs.yml from the documents
// under cfg.DocsDir, preserving every other key in the file.
//
// The existing top-level section order is read back out of the file
// first and handed to BuildNav, so a hand-ordered nav survives the
// regeneration and only new sections are appended. That is the one
// behaviour a user notices, which is why the read happens even on a dry
// run.
//
// A missing mkdocs.yml comes back as the read error wrapping
// fs.ErrNotExist; telling the user to run `docz wiki init` first is the
// caller's wording, not this package's.
//
// With opts.DryRun the report carries the same Entries and Pages and
// Written stays false, so a caller can show exactly what a live run
// would write.
//
// The context is checked before the read, before the scan, and before
// the write; cancellation returns the report so far with ctx.Err() and
// leaves the file alone.
func UpdateNav(
	ctx context.Context,
	root string,
	cfg *config.Config,
	opts NavOptions,
) (NavReport, error) {
	report := NavReport{Path: underRoot(root, cfg.Wiki.MkDocsPath)}

	if err := ctx.Err(); err != nil {
		return report, err
	}

	data, err := ReadMkDocs(report.Path)
	if err != nil {
		return report, err
	}

	if err := ctx.Err(); err != nil {
		return report, err
	}

	docsDir := underRoot(root, cfg.DocsDir)

	entries, err := BuildNav(
		docsDir,
		cfg.Wiki.Exclude,
		cfg.Wiki.NavTitles,
		ExistingNavOrder(data),
	)
	if err != nil {
		return report, fmt.Errorf("scanning %s: %w", docsDir, err)
	}

	report.Entries = entries
	report.Pages = CountPages(entries)

	if opts.DryRun {
		return report, nil
	}

	if err := ctx.Err(); err != nil {
		return report, err
	}

	data["nav"] = NavToYAML(entries)
	if err := WriteMkDocs(report.Path, data); err != nil {
		return report, err
	}

	report.Written = true

	return report, nil
}

// writeMkDocs writes mkdocs.yml unless it is there and force is not set,
// and reports which of the three things it did.
func writeMkDocs(
	mkdocsPath string,
	cfg *config.Config,
	opts *InitOptions,
	siteName string,
) (Action, error) {
	action := planWrite(mkdocsPath, opts.Force)
	if action == Skipped {
		return Skipped, nil
	}

	siteDesc := opts.SiteDescription
	if siteDesc == "" {
		siteDesc = "Documentation for " + siteName
	}

	if err := ensureParent(mkdocsPath); err != nil {
		return 0, err
	}

	err := CreateMkDocs(mkdocsPath, &MkDocsConfig{
		SiteName:        siteName,
		SiteDescription: siteDesc,
		// cfg.Wiki.DocsDir is mkdocs' own docs_dir key, which is not
		// cfg.DocsDir: one tells MkDocs where to read, the other tells
		// docz where to write.
		DocsDir:            cfg.Wiki.DocsDir,
		RepoURL:            firstNonEmpty(opts.RepoURL, cfg.Wiki.RepoURL),
		SiteURL:            firstNonEmpty(opts.SiteURL, cfg.Wiki.SiteURL),
		Theme:              firstNonEmpty(opts.Theme, cfg.Wiki.Theme),
		Plugins:            cfg.Wiki.Plugins,
		MarkdownExtensions: cfg.Wiki.MarkdownExtensions,
	})
	if err != nil {
		return 0, err
	}

	return action, nil
}

// writeIndex renders and writes the docs landing page unless it is there
// and force is not set, and reports which of the three things it did.
func writeIndex(
	indexPath, docsDir string,
	cfg *config.Config,
	siteName string,
	force bool,
) (Action, error) {
	action := planWrite(indexPath, force)
	if action == Skipped {
		return Skipped, nil
	}

	tmpl, err := doctemplate.ResolveWikiIndex(docsDir)
	if err != nil {
		return 0, fmt.Errorf("resolving wiki index template: %w", err)
	}

	content, err := doctemplate.RenderWikiIndex(tmpl, &doctemplate.WikiIndexData{
		SiteName: siteName,
		Types:    indexTypes(cfg),
	})
	if err != nil {
		return 0, fmt.Errorf("rendering wiki index: %w", err)
	}

	if err := ensureParent(indexPath); err != nil {
		return 0, err
	}

	if err := os.WriteFile(indexPath, []byte(content), config.FileMode); err != nil {
		return 0, fmt.Errorf("writing %s: %w", indexPath, err)
	}

	return action, nil
}

// indexTypes lists the enabled document types for the landing-page
// template, in cfg.EnabledTypes order.
//
// The label precedence is WikiConfig.NavTitles, then the type's
// PluralLabel, then the upper-cased type name. NavTitles wins for
// back-compat (DESIGN-0006 Decisions §4) — a repo that named its
// sections before PluralLabel existed keeps the names it chose.
func indexTypes(cfg *config.Config) []doctemplate.WikiIndexType {
	enabled := cfg.EnabledTypes()
	types := make([]doctemplate.WikiIndexType, 0, len(enabled))

	for _, typeName := range enabled {
		tc := cfg.Types[typeName]

		navTitle := cfg.Wiki.NavTitles[typeName]
		if navTitle == "" {
			navTitle = tc.PluralLabel
		}

		if navTitle == "" {
			navTitle = strings.ToUpper(typeName)
		}

		types = append(types, doctemplate.WikiIndexType{
			Name:     typeName,
			NavTitle: navTitle,
			Dir:      tc.Dir,
		})
	}

	return types
}

// planWrite decides what writing path amounts to: Created when the file
// is not there, Overwritten when it is and force is set, Skipped when it
// is and force is not.
//
// Present means "stat succeeded", which is cmd/wiki.go's rule: a path
// that cannot be stat'ed for any other reason is one docz tries to write,
// and the write reports the real error rather than this function guessing
// at it.
func planWrite(path string, force bool) Action {
	if _, err := os.Stat(path); err != nil {
		return Created
	}

	if force {
		return Overwritten
	}

	return Skipped
}

// ensureParent creates the directory a file is about to be written into.
//
// cmd/wiki.go gets docs/ for free because `docz init` runs first; a
// library caller has promised no such thing, and MkdirAll over a
// directory that already exists costs nothing — so this adds a guarantee
// without changing what the CLI does.
func ensureParent(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, config.DirMode); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	return nil
}

// resolveSiteName picks the site name: the caller's, else the directory
// name of root, else a placeholder.
//
// There is no config tier between them because config.WikiConfig carries
// no site name — the CLI resolves the git remote into opts.SiteName, and
// a library caller that wants something else passes it.
func resolveSiteName(root string, opts *InitOptions) string {
	if opts.SiteName != "" {
		return opts.SiteName
	}

	if root != "" {
		return filepath.Base(root)
	}

	return defaultSiteName
}

// underRoot resolves a config-relative path under root, leaving an
// absolute path — or an empty root — alone.
//
// The same rule as cmd's Runner.inRepo, and the reason Init and UpdateNav
// never call os.Getwd: the repo root is a parameter, so a caller that
// operates on a checkout somewhere other than its own working directory
// gets the paths it meant.
func underRoot(root, path string) string {
	if path == "" || root == "" || filepath.IsAbs(path) {
		return path
	}

	return filepath.Join(root, path)
}

// firstNonEmpty returns a if it is set, else b.
func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}

	return b
}
