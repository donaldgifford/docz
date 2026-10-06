package confluence

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/repo"
)

// plan is the page tree one Export writes, in write order, and the set of
// files a link can resolve to.
type plan struct {
	items []item
	// full reports a run with no Types and no IDs, the only kind that
	// archives orphans.
	full bool
	// keys are the property ids every page of a full export carries; a
	// docz page under the tree whose id is not here is an orphan.
	keys map[string]bool
	// titles are the page titles of everything a full export writes. A
	// page holding one is never an orphan, whatever its property says.
	titles map[string]bool
	// targets maps a repository-relative slash path to the page it is
	// exported as, over everything a full export would write.
	targets map[string]target
	// render is the per-page render configuration minus Source and Title.
	render RenderOptions
}

// typeKeyPrefix starts a type page's property id.
const typeKeyPrefix = "docz:type:"

// item is one page to reconcile.
type item struct {
	// key is the docz property id: the document id, or docz:parent,
	// docz:type:<name>, docz:page:<path> for the pages docz composes.
	key string
	// docID is the document id, empty for a composed page.
	docID string
	// title is the page title.
	title string
	// source is the repository-relative slash path the page is rendered
	// from; empty for a parent page with no landing page.
	source string
	// src is the markdown to render.
	src []byte
	// override is RenderOptions.Title: set for every composed page.
	override string
	// parent indexes the item this page sits under; -1 for the parent page,
	// which sits wherever it already is or at the space root.
	parent int
}

// target is an exported file: its page title and its headings' anchors.
type target struct {
	title    string
	headings map[string]string
}

// buildPlan lists, filters, and orders everything Export will write. It
// reads the repository and makes no request.
func buildPlan(ctx context.Context, rp *repo.Repo, opts *ExportOptions) (*plan, error) {
	sync := &rp.Cfg.Sync.Confluence
	p := &plan{
		full:    len(opts.Types) == 0 && len(opts.IDs) == 0,
		keys:    make(map[string]bool),
		titles:  make(map[string]bool),
		targets: make(map[string]target),
		render:  renderOptions(sync),
	}

	allTypes, err := canonicalTypes(rp.Cfg, sync.Types)
	if err != nil {
		return nil, err
	}

	allTypes = slices.DeleteFunc(allTypes, func(t string) bool { return excluded(rp, rp.RelPath(rp.ReadmePath(t))) })

	all, err := listDocs(ctx, rp, allTypes)
	if err != nil {
		return nil, err
	}

	selected, types, err := selectDocs(ctx, rp, opts, all, allTypes)
	if err != nil {
		return nil, err
	}

	parent, err := parentItem(rp, sync)
	if err != nil {
		return nil, err
	}

	p.add(parent)

	for _, typeName := range allTypes {
		p.addTarget(typePage(rp, typeName, 0))
	}

	for i := range all {
		p.addTarget(docItem(&all[i], 0))
	}

	for _, typeName := range types {
		tp := len(p.items)
		p.add(typePage(rp, typeName, 0))

		for i := range selected {
			if selected[i].Type == typeName {
				p.add(docItem(&selected[i], tp))
			}
		}
	}

	if err := p.addPages(rp, sync); err != nil {
		return nil, err
	}

	return p, nil
}

// add appends an item to the write order and marks it exported.
func (p *plan) add(it item) { //nolint:gocritic // the item is stored by value
	p.items = append(p.items, it)
	p.addTarget(it)
}

// addTarget records an item as a link target and a known key without
// writing it, so a narrowed run still links to pages a full run wrote.
func (p *plan) addTarget(it item) { //nolint:gocritic // read once, by value like add
	p.keys[it.key] = true
	p.titles[it.title] = true

	if it.source != "" {
		p.targets[it.source] = target{title: it.title, headings: headingText(it.src)}
	}
}

// addPages appends the api: additional docs, on a full run with api_pages
// on.
func (p *plan) addPages(rp *repo.Repo, sync *config.ConfluenceSyncConfig) error {
	if !sync.APIPages {
		return nil
	}

	for _, rel := range rp.Cfg.API.AdditionalDocs {
		it, err := pageItem(rp, rel, 0)
		if err != nil {
			return err
		}

		if p.full {
			p.add(it)
		} else {
			p.addTarget(it)
		}
	}

	return nil
}

// renderOptions maps the sync block's mermaid setting.
func renderOptions(sync *config.ConfluenceSyncConfig) RenderOptions {
	switch v := sync.Mermaid.Viewer; v {
	case config.MermaidViewerOff:
		return RenderOptions{Mermaid: MermaidCode}
	case config.MermaidViewerAuto, "":
		return RenderOptions{}
	default:
		return RenderOptions{ViewerKey: v}
	}
}

// canonicalTypes resolves type tokens, or returns every enabled type for
// none.
func canonicalTypes(cfg *config.Config, tokens []string) ([]string, error) {
	if len(tokens) == 0 {
		return cfg.EnabledTypes(), nil
	}

	out := make([]string, 0, len(tokens))

	for _, tok := range tokens {
		name, err := cfg.ValidateType(tok)
		if err != nil {
			return nil, err
		}

		if !cfg.Types[name].Enabled {
			return nil, &repo.TypeDisabledError{Type: name}
		}

		if !slices.Contains(out, name) {
			out = append(out, name)
		}
	}

	return out, nil
}

// listDocs lists the documents of types, minus sync.confluence.exclude.
func listDocs(ctx context.Context, rp *repo.Repo, types []string) ([]repo.Entry, error) {
	entries, err := rp.List(ctx, types)
	if err != nil {
		return nil, err
	}

	return slices.DeleteFunc(entries, func(e repo.Entry) bool { return excluded(rp, e.Path) }), nil
}

// selectDocs narrows the full set by opts.Types and opts.IDs, and returns
// the documents to write with the types whose pages they need.
func selectDocs(
	ctx context.Context,
	rp *repo.Repo,
	opts *ExportOptions,
	all []repo.Entry,
	allTypes []string,
) ([]repo.Entry, []string, error) {
	if len(opts.IDs) > 0 {
		var (
			docs  []repo.Entry
			types []string
		)

		for _, id := range opts.IDs {
			e, err := rp.Find(ctx, id)
			if err != nil {
				return nil, nil, err
			}

			if excluded(rp, e.Path) {
				continue
			}

			docs = append(docs, e)

			if !slices.Contains(types, e.Type) {
				types = append(types, e.Type)
			}
		}

		return docs, orderTypes(types, allTypes), nil
	}

	if len(opts.Types) == 0 {
		return all, allTypes, nil
	}

	types, err := canonicalTypes(rp.Cfg, opts.Types)
	if err != nil {
		return nil, nil, err
	}

	docs := slices.DeleteFunc(slices.Clone(all), func(e repo.Entry) bool { return !slices.Contains(types, e.Type) })

	return docs, orderTypes(types, allTypes), nil
}

// orderTypes puts types in the full run's order, dropping any a full run
// would not export, so a narrowed run writes in the same order and to the
// same pages.
func orderTypes(types, allTypes []string) []string {
	out := make([]string, 0, len(types))

	for _, name := range allTypes {
		if slices.Contains(types, name) {
			out = append(out, name)
		}
	}

	return out
}

// excluded reports whether a repository-relative path falls under one of
// sync.confluence.exclude's prefixes, which are relative to docs_dir.
func excluded(rp *repo.Repo, rel string) bool {
	docsDir := filepath.ToSlash(filepath.Clean(rp.Cfg.DocsDir))
	if filepath.IsAbs(rp.Cfg.DocsDir) && rp.Root != "" {
		docsDir = filepath.ToSlash(rp.RelPath(rp.Cfg.DocsDir))
	}

	under, ok := strings.CutPrefix(filepath.ToSlash(rel), docsDir+"/")
	if !ok {
		return false
	}

	for _, prefix := range rp.Cfg.Sync.Confluence.Exclude {
		if under == prefix || strings.HasPrefix(under, prefix+"/") {
			return true
		}
	}

	return false
}

// docItem is one document's page.
func docItem(e *repo.Entry, parent int) item {
	return item{
		key:    e.ID,
		docID:  e.ID,
		title:  pageTitle(&e.Frontmatter, e.Content, ""),
		source: filepath.ToSlash(e.Path),
		src:    e.Content,
		parent: parent,
	}
}

// typePage is a type's page: its nav title, its README index as the body.
func typePage(rp *repo.Repo, typeName string, parent int) item {
	title := navTitle(rp.Cfg, typeName)
	readme := rp.ReadmePath(typeName)

	src, err := os.ReadFile(readme)
	if err != nil {
		src = []byte("No " + title + " yet.\n")
	}

	return item{
		key:      typeKeyPrefix + typeName,
		title:    title,
		source:   filepath.ToSlash(rp.RelPath(readme)),
		src:      src,
		override: title,
		parent:   parent,
	}
}

// navTitle is a type's section title, by the wiki's precedence.
func navTitle(cfg *config.Config, typeName string) string {
	if t := cfg.Wiki.NavTitles[typeName]; t != "" {
		return t
	}

	if t := cfg.Types[typeName].PluralLabel; t != "" {
		return t
	}

	return strings.ToUpper(typeName)
}

// parentItem is the parent page: the api landing page when api_pages is
// on, else a line naming the repository.
func parentItem(rp *repo.Repo, sync *config.ConfluenceSyncConfig) (item, error) {
	if sync.APIPages && rp.Cfg.API.LandingPage != "" {
		it, err := pageItem(rp, rp.Cfg.API.LandingPage, -1)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return item{}, err
		}

		if err == nil {
			it.key, it.title, it.override = "docz:parent", sync.Parent, sync.Parent

			return it, nil
		}
	}

	name := filepath.Base(rp.Root)
	if rp.Root == "" {
		name = sync.Parent
	}

	return item{
		key:      "docz:parent",
		title:    sync.Parent,
		src:      fmt.Appendf(nil, "Documentation exported from `%s` by docz.\n", name),
		override: sync.Parent,
		parent:   -1,
	}, nil
}

// pageItem is an api: page: a file with no docz frontmatter, titled by its
// first H1 or its path.
func pageItem(rp *repo.Repo, rel string, parent int) (item, error) {
	src, err := os.ReadFile(rp.Path(rel))
	if err != nil {
		return item{}, fmt.Errorf("confluence: reading %s: %w", rel, err)
	}

	title := docparse.Title(src)
	if title == "" {
		title = path.Base(rel)
	}

	return item{
		key:      "docz:page:" + rel,
		title:    title,
		source:   rel,
		src:      bytes.Clone(src),
		override: title,
		parent:   parent,
	}, nil
}

// resolver places a relative link: on its exported page when the target is
// in the plan, else through the caller's resolver.
func (p *plan) resolver(fallback LinkResolver) LinkResolver {
	return func(from, href string) LinkTarget {
		file, frag, _ := strings.Cut(href, "#")

		rel := path.Clean(path.Join(path.Dir(from), file))
		if file == "" {
			rel = from
		}

		t, ok := p.targets[rel]
		if !ok {
			t, ok = p.targets[path.Join(rel, config.IndexFileName)]
		}

		if ok {
			out := LinkTarget{PageTitle: t.title}
			if text, found := t.headings[frag]; found {
				out.Anchor = anchorID(t.title, text)
			}

			return out
		}

		if fallback == nil {
			return LinkTarget{}
		}

		return fallback(from, href)
	}
}
