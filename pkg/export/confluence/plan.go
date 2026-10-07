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
	"github.com/donaldgifford/docz/v2/pkg/doczcore/document"
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
	// folder is the repository folder's title in the folder layout, and
	// empty in the page layout. Every title but the home page's starts with
	// it and a colon.
	folder string
	// archive is the title of the page orphans move under.
	archive string
	// parentTitle is sync.confluence.parent: the parent page's title in the
	// page layout, the page the folder sits under in the folder layout.
	parentTitle string
	// space is sync.confluence.space, for messages.
	space string
	// top is the parent index of a type page or an additional doc: the
	// parent page (0) in the page layout, the folder (-1) in the folder
	// layout.
	top int
}

// parentKey is the property id of the parent page, which is the folder's
// home page in the folder layout.
const parentKey = "docz:parent"

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
// reads opts.FS, never the disk, and makes no request.
func buildPlan(ctx context.Context, rp *repo.Repo, opts *ExportOptions) (*plan, error) {
	sync := &rp.Cfg.Sync.Confluence
	src := &source{rp: rp, fsys: opts.FS}
	if src.fsys == nil {
		src.fsys = os.DirFS(rp.Root)
	}

	p := &plan{
		full:        len(opts.Types) == 0 && len(opts.IDs) == 0,
		keys:        make(map[string]bool),
		titles:      make(map[string]bool),
		targets:     make(map[string]target),
		render:      renderOptions(sync),
		archive:     archiveTitle,
		parentTitle: sync.Parent,
		space:       sync.Space,
	}

	if sync.Layout != config.LayoutPage {
		p.folder = folderTitle(rp, sync, opts.Repository)
		p.archive = p.folder + ": " + archiveTitle
		p.top = -1
	}

	allTypes, err := canonicalTypes(rp.Cfg, sync.Types)
	if err != nil {
		return nil, err
	}

	allTypes = slices.DeleteFunc(allTypes, func(t string) bool { return excluded(rp, src.readme(t)) })

	all, err := src.listDocs(ctx, allTypes)
	if err != nil {
		return nil, err
	}

	selected, types, err := selectDocs(ctx, src, opts, all, allTypes)
	if err != nil {
		return nil, err
	}

	parent, err := parentItem(src, sync, opts.Repository)
	if err != nil {
		return nil, err
	}

	p.add(parent)

	for _, typeName := range allTypes {
		p.addTarget(typePage(src, typeName, p.top))
	}

	for i := range all {
		p.addTarget(docItem(&all[i], 0))
	}

	for _, typeName := range types {
		tp := len(p.items)
		p.add(typePage(src, typeName, p.top))

		for i := range selected {
			if selected[i].Type == typeName {
				p.add(docItem(&selected[i], tp))
			}
		}
	}

	if err := p.addPages(src, sync); err != nil {
		return nil, err
	}

	return p, nil
}

// folderTitle is the repository folder's title: sync.confluence.folder,
// else the repository's name, else the base name of the root.
func folderTitle(rp *repo.Repo, sync *config.ConfluenceSyncConfig, repository string) string {
	switch {
	case sync.Folder != "":
		return sync.Folder
	case repository != "":
		return path.Base(repository)
	default:
		return filepath.Base(rp.Root)
	}
}

// add appends an item to the write order and marks it exported.
func (p *plan) add(it item) { //nolint:gocritic // the item is stored by value
	p.retitle(&it)
	p.items = append(p.items, it)
	p.record(&it)
}

// addTarget records an item as a link target and a known key without
// writing it, so a narrowed run still links to pages a full run wrote.
func (p *plan) addTarget(it item) { //nolint:gocritic // the caller's copy, retitled here
	p.retitle(&it)
	p.record(&it)
}

// retitle prefixes an item's title with the folder's in the folder layout:
// the home page takes the folder's title alone, everything else the folder
// name and a colon, and the title is forced on render so a document's own
// "ID: Title" carries the prefix too.
func (p *plan) retitle(it *item) {
	if p.folder == "" {
		return
	}

	if it.key == parentKey {
		it.title = p.folder
	} else {
		it.title = p.folder + ": " + it.title
	}

	it.override = it.title
}

// record marks an item's key, title, and source known.
func (p *plan) record(it *item) {
	p.keys[it.key] = true
	p.titles[it.title] = true

	if it.source != "" {
		p.targets[it.source] = target{title: it.title, headings: headingText(it.src)}
	}
}

// addPages appends the api: additional docs, on a full run with api_pages
// on.
func (p *plan) addPages(src *source, sync *config.ConfluenceSyncConfig) error {
	if !sync.APIPages {
		return nil
	}

	for _, rel := range src.rp.Cfg.API.AdditionalDocs {
		it, err := pageItem(src, rel, p.top)
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

// source reads the repository through an fs.FS: rp supplies the
// configuration and the root's name, fsys every byte.
type source struct {
	rp   *repo.Repo
	fsys fs.FS
}

// rel is a configured path as the slash path fsys addresses: repository
// relative and cleaned, whether the configuration spelled it relative or
// absolute under the root.
func (s *source) rel(p string) string {
	return path.Clean(filepath.ToSlash(s.rp.RelPath(s.rp.Path(p))))
}

// typeDir is a type's directory as a slash path.
func (s *source) typeDir(typeName string) string {
	return s.rel(s.rp.Cfg.TypeDir(typeName))
}

// readme is a type's README as a slash path.
func (s *source) readme(typeName string) string {
	return path.Join(s.typeDir(typeName), config.IndexFileName)
}

// scan lists one type's documents as repo.Scan does: names
// document.IsDoczFile accepts, files without frontmatter or that will not
// read skipped, a missing directory empty, sorted by id.
func (s *source) scan(typeName string) ([]repo.Entry, error) {
	dir := s.typeDir(typeName)

	files, err := fs.ReadDir(s.fsys, dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}

		return nil, fmt.Errorf("scanning %s: %w", dir, err)
	}

	var entries []repo.Entry

	for _, f := range files {
		if f.IsDir() || !document.IsDoczFile(f.Name()) {
			continue
		}

		rel := path.Join(dir, f.Name())

		content, err := fs.ReadFile(s.fsys, rel)
		if err != nil {
			continue
		}

		fm, err := document.ParseFrontmatter(content)
		if err != nil {
			continue
		}

		entries = append(entries, repo.Entry{
			DocEntry: document.DocEntry{Frontmatter: fm, Filename: f.Name(), Content: content},
			Type:     typeName,
			Path:     rel,
		})
	}

	slices.SortFunc(entries, func(a, b repo.Entry) int { return strings.Compare(a.ID, b.ID) })

	return entries, nil
}

// listDocs lists the documents of types in their order, minus
// sync.confluence.exclude. The context is checked between types.
func (s *source) listDocs(ctx context.Context, types []string) ([]repo.Entry, error) {
	var all []repo.Entry

	for _, typeName := range types {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		entries, err := s.scan(typeName)
		if err != nil {
			return nil, err
		}

		all = append(all, entries...)
	}

	return slices.DeleteFunc(all, func(e repo.Entry) bool { return excluded(s.rp, e.Path) }), nil
}

// find resolves an id as repo.Find does: the prefix before the first "-"
// names the type, an id with none is a *repo.UnknownTypeError, and a miss
// is a *repo.NotFoundError. listed holds the entries already read; a type
// the export does not cover is scanned on demand.
func (s *source) find(id string, listed []repo.Entry) (repo.Entry, error) {
	cfg := s.rp.Cfg

	prefix, _, ok := strings.Cut(id, "-")
	if !ok || prefix == "" {
		return repo.Entry{}, &repo.UnknownTypeError{Token: id, Valid: cfg.EnabledTypes()}
	}

	typeName, err := cfg.ValidateType(prefix)
	if err != nil {
		return repo.Entry{}, &repo.UnknownTypeError{Token: prefix, Valid: cfg.EnabledTypes()}
	}

	if !cfg.Types[typeName].Enabled {
		return repo.Entry{}, &repo.TypeDisabledError{Type: typeName}
	}

	if i := slices.IndexFunc(listed, func(e repo.Entry) bool { return e.Type == typeName && e.ID == id }); i >= 0 {
		return listed[i], nil
	}

	entries, err := s.scan(typeName)
	if err != nil {
		return repo.Entry{}, err
	}

	if i := slices.IndexFunc(entries, func(e repo.Entry) bool { return e.ID == id }); i >= 0 {
		return entries[i], nil
	}

	return repo.Entry{}, &repo.NotFoundError{Type: typeName, ID: id}
}

// selectDocs narrows the full set by opts.Types and opts.IDs, and returns
// the documents to write with the types whose pages they need.
func selectDocs(
	ctx context.Context,
	src *source,
	opts *ExportOptions,
	all []repo.Entry,
	allTypes []string,
) ([]repo.Entry, []string, error) {
	if len(opts.IDs) > 0 {
		return selectIDs(ctx, src, opts.IDs, all, allTypes)
	}

	if len(opts.Types) == 0 {
		return all, allTypes, nil
	}

	types, err := canonicalTypes(src.rp.Cfg, opts.Types)
	if err != nil {
		return nil, nil, err
	}

	docs := slices.DeleteFunc(slices.Clone(all), func(e repo.Entry) bool { return !slices.Contains(types, e.Type) })

	return docs, orderTypes(types, allTypes), nil
}

// selectIDs resolves each id and returns the documents with the types
// whose pages they need; an excluded document is dropped silently.
func selectIDs(
	ctx context.Context,
	src *source,
	ids []string,
	all []repo.Entry,
	allTypes []string,
) ([]repo.Entry, []string, error) {
	var (
		docs  []repo.Entry
		types []string
	)

	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}

		e, err := src.find(id, all)
		if err != nil {
			return nil, nil, err
		}

		if excluded(src.rp, e.Path) {
			continue
		}

		docs = append(docs, e)

		if !slices.Contains(types, e.Type) {
			types = append(types, e.Type)
		}
	}

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

// excluded reports whether a repository-relative slash path falls under
// one of sync.confluence.exclude's prefixes, which are relative to
// docs_dir.
func excluded(rp *repo.Repo, rel string) bool {
	docsDir := (&source{rp: rp}).rel(rp.Cfg.DocsDir)

	under, ok := strings.CutPrefix(rel, docsDir+"/")
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
func typePage(s *source, typeName string, parent int) item {
	title := navTitle(s.rp.Cfg, typeName)
	readme := s.readme(typeName)

	src, err := fs.ReadFile(s.fsys, readme)
	if err != nil {
		src = []byte("No " + title + " yet.\n")
	}

	return item{
		key:      typeKeyPrefix + typeName,
		title:    title,
		source:   readme,
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
func parentItem(s *source, sync *config.ConfluenceSyncConfig, repository string) (item, error) {
	rp := s.rp
	if sync.APIPages && rp.Cfg.API.LandingPage != "" {
		it, err := pageItem(s, rp.Cfg.API.LandingPage, -1)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return item{}, err
		}

		if err == nil {
			it.key, it.title, it.override = parentKey, sync.Parent, sync.Parent

			return it, nil
		}
	}

	name := repository
	if name == "" {
		name = filepath.Base(rp.Root)
	}

	if rp.Root == "" && repository == "" {
		name = sync.Parent
	}

	return item{
		key:      parentKey,
		title:    sync.Parent,
		src:      fmt.Appendf(nil, "Documentation exported from `%s` by docz.\n", name),
		override: sync.Parent,
		parent:   -1,
	}, nil
}

// pageItem is an api: page: a file with no docz frontmatter, titled by its
// first H1 or its path.
func pageItem(s *source, rel string, parent int) (item, error) {
	src, err := fs.ReadFile(s.fsys, s.rel(rel))
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
