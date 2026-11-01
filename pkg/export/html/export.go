package html

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/repo"
)

// DefaultOut is where Export writes when ExportOptions.Out is empty,
// relative to the repository root.
const DefaultOut = "build/html"

// Action is what Export did with one page.
type Action string

const (
	// ActionWrote is a page written because it was new or changed.
	ActionWrote Action = "wrote"
	// ActionUnchanged is a page whose bytes on disk already match.
	ActionUnchanged Action = "unchanged"
	// ActionFailed is a document that did not render or write.
	ActionFailed Action = "failed"
)

// ExportOptions configures one Export. The zero value exports every
// enabled type to <root>/build/html in the repository's html.theme with
// mermaid embedded.
type ExportOptions struct {
	// Types limits the run to these type tokens; empty means every enabled
	// type. IDs, when set, wins.
	Types []string
	// IDs exports exactly these documents, in this order.
	IDs []string
	// Out is the directory the pages go to, relative to the repository
	// root when not absolute. Empty means DefaultOut.
	Out string
	// Theme is the pkg/theme name every page renders in. Empty means the
	// repository's html.theme (DESIGN-0025 §3), so a consumer calling
	// Export without the CLI gets the default the CLI gets.
	Theme string
	// Mermaid, Site, and Version are passed to every Render.
	Mermaid MermaidMode
	Site    string
	Version string
	// Resolve places a relative link that is not to a document this run
	// exports or an image Export inlines. Nil leaves it unresolved.
	Resolve LinkResolver
	// DryRun renders every page and reports what a run would do, writing
	// nothing.
	DryRun bool
}

// PageResult is one page of a run.
type PageResult struct {
	// ID is the document's id; empty for the index.
	ID string `json:"id"`
	// Type is the document's canonical type name; empty for the index.
	Type string `json:"type"`
	// Source is the document's repository-relative path.
	Source string `json:"source"`
	// Path is the page's path relative to the output directory.
	Path string `json:"path"`
	// Action is what the run did with the page.
	Action Action `json:"action"`
	// Hash is the page's [Page.Hash]; empty when it failed to render.
	Hash string `json:"hash"`
	// Links are the links and images the page could not place.
	Links []Link `json:"unresolved_links"`
	// Problems are what fell back to plain rendering, as text.
	Problems []string `json:"problems"`
	// Error is why the page failed; empty otherwise.
	Error string `json:"error,omitempty"`
	// Body is the rendered page, for a caller that prints it.
	Body []byte `json:"-"`
}

// Report is one run.
type Report struct {
	// Out is the output directory as the run resolved it.
	Out string `json:"out"`
	// Theme is the theme the run rendered in.
	Theme string `json:"theme"`
	// DryRun says nothing was written.
	DryRun bool `json:"dry_run"`
	// Pages are the documents' pages in docz list order, then the index
	// when the run wrote one.
	Pages []PageResult `json:"pages"`
}

// Failed counts the pages that failed.
func (r *Report) Failed() int {
	n := 0

	for i := range r.Pages {
		if r.Pages[i].Action == ActionFailed {
			n++
		}
	}

	return n
}

// Export renders documents of rp as standalone pages under the output
// directory, mirroring each type's directory below the docs directory. A
// page whose bytes on disk already match is not rewritten. A document that
// fails is reported and the run continues; ctx is checked between
// documents, and a cancelled run returns the report so far with ctx.Err().
// The error is for the run itself: a type or id that does not resolve, or
// an output directory that cannot be opened.
func Export(ctx context.Context, rp *repo.Repo, opts ExportOptions) (Report, error) { //nolint:gocritic // options by value, as Render.
	out := opts.Out
	if out == "" {
		out = DefaultOut
	}

	if !filepath.IsAbs(out) {
		out = filepath.Join(rp.Root, out)
	}

	if opts.Theme == "" && rp.Cfg != nil {
		opts.Theme = rp.Cfg.HTML.Theme
	}

	if opts.Theme == "" {
		opts.Theme = config.DefaultTheme
	}

	rep := Report{Out: out, Theme: opts.Theme, DryRun: opts.DryRun}

	entries, err := entriesFor(ctx, rp, &opts)
	if err != nil {
		return rep, err
	}

	var root *os.Root

	if !opts.DryRun {
		if err := os.MkdirAll(out, config.DirMode); err != nil {
			return rep, fmt.Errorf("html: output directory: %w", err)
		}

		root, err = os.OpenRoot(out)
		if err != nil {
			return rep, fmt.Errorf("html: output directory: %w", err)
		}
		defer root.Close() //nolint:errcheck // read and write errors are reported per page.
	}

	src, err := os.OpenRoot(rp.Root)
	if err != nil {
		return rep, fmt.Errorf("html: repository root: %w", err)
	}
	defer src.Close() //nolint:errcheck // only read from.

	x := &exporter{
		rp: rp, opts: &opts, root: root, src: src,
		specs: make(map[string]kinds.HeadingSpec), pages: pagePaths(rp, entries),
	}

	for i := range entries {
		if err := ctx.Err(); err != nil {
			return rep, err
		}

		res := x.page(ctx, &entries[i])
		rep.Pages = append(rep.Pages, res)
		firePageDone(ctx, &res)
	}

	if len(entries) > 1 {
		res := x.index(entries, rep.Pages)
		rep.Pages = append(rep.Pages, res)
		firePageDone(ctx, &res)
	}

	return rep, nil
}

// entriesFor lists the run's documents: the ids in order, else the types
// in docz list order.
func entriesFor(ctx context.Context, rp *repo.Repo, opts *ExportOptions) ([]repo.Entry, error) {
	if len(opts.IDs) == 0 {
		entries, err := rp.List(ctx, opts.Types)
		if err != nil {
			return nil, fmt.Errorf("html: %w", err)
		}

		return entries, nil
	}

	entries := make([]repo.Entry, 0, len(opts.IDs))

	for _, id := range opts.IDs {
		e, err := rp.Find(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("html: %w", err)
		}

		entries = append(entries, e)
	}

	return entries, nil
}

// pagePaths maps each document's repository-relative path to its page's
// path under the output directory: the path below the docs directory with
// .html for .md. The docs directory may be absolute, as cmd/ makes it.
func pagePaths(rp *repo.Repo, entries []repo.Entry) map[string]string {
	docs := rp.Cfg.DocsDir
	if rel, err := filepath.Rel(rp.Root, docs); err == nil && filepath.IsAbs(docs) {
		docs = rel
	}

	docs = path.Clean(filepath.ToSlash(docs))
	out := make(map[string]string, len(entries))

	for i := range entries {
		src := filepath.ToSlash(entries[i].Path)

		rel := src
		if r, ok := strings.CutPrefix(src, docs+"/"); ok {
			rel = r
		}

		out[src] = strings.TrimSuffix(rel, ".md") + ".html"
	}

	return out
}

// exporter is one run's state.
type exporter struct {
	rp   *repo.Repo
	opts *ExportOptions
	root *os.Root
	// src is the repository root, the only place an image is read from.
	src   *os.Root
	specs map[string]kinds.HeadingSpec
	// pages maps an exported document's repository-relative path to its
	// page's path under the output directory.
	pages map[string]string
}

// spec is a type's heading spec, read from its template once per run.
func (x *exporter) spec(ctx context.Context, typeName string) kinds.HeadingSpec {
	if s, ok := x.specs[typeName]; ok {
		return s
	}

	var s kinds.HeadingSpec

	if body, _, err := x.rp.Template(ctx, typeName); err == nil {
		s = kinds.SpecFromTemplate([]byte(body))
	}

	x.specs[typeName] = s

	return s
}

// page renders and writes one document.
func (x *exporter) page(ctx context.Context, e *repo.Entry) PageResult {
	src := filepath.ToSlash(e.Path)
	res := PageResult{ID: e.ID, Type: e.Type, Source: src, Path: x.pages[src], Links: []Link{}, Problems: []string{}}

	content := e.Content
	if content == nil {
		b, err := os.ReadFile(x.rp.Path(e.Path))
		if err != nil {
			return failed(&res, err)
		}

		content = b
	}

	p, err := Render(content, RenderOptions{
		Theme: x.opts.Theme, Mermaid: x.opts.Mermaid, Spec: x.spec(ctx, e.Type), Type: e.Type,
		Source: src, Resolve: x.resolve, Site: x.opts.Site, Version: x.opts.Version,
	})
	if err != nil {
		return failed(&res, err)
	}

	res.Hash, res.Body = p.Hash, p.Body
	res.Links = append(res.Links, p.Links...)

	for _, pr := range p.Problems {
		res.Problems = append(res.Problems, pr.Error())
	}

	action, err := x.write(res.Path, p.Body)
	if err != nil {
		return failed(&res, err)
	}

	res.Action = action

	return res
}

// write writes body at rel under the output directory unless the bytes
// there already match, and reports which it did. A dry run reads nothing
// and writes nothing, and reports every page as written.
func (x *exporter) write(rel string, body []byte) (Action, error) {
	if x.root == nil {
		return ActionWrote, nil
	}

	rel = filepath.FromSlash(rel)

	if old, err := x.root.ReadFile(rel); err == nil && bytes.Equal(old, body) {
		return ActionUnchanged, nil
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("html: %w", err)
	}

	if dir := filepath.Dir(rel); dir != "." {
		if err := x.root.MkdirAll(dir, config.DirMode); err != nil {
			return "", fmt.Errorf("html: %w", err)
		}
	}

	if err := x.root.WriteFile(rel, body, config.FileMode); err != nil {
		return "", fmt.Errorf("html: %w", err)
	}

	return ActionWrote, nil
}

// failed marks res failed with err.
func failed(res *PageResult, err error) PageResult {
	res.Action = ActionFailed
	res.Error = err.Error()
	res.Body = nil

	return *res
}
