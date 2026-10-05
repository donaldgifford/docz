package confluence

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/document"
)

// DefaultViewerKey is the production extension key of Atlassian Labs'
// Mermaid diagrams viewer (DESIGN-0020 §4). A Forge app has one production
// environment every installation shares, so the key is the app's, not the
// site's.
const DefaultViewerKey = "23392b90-4271-4239-98ca-a3e96c663cbb/" +
	"63d4d207-ac2f-4273-865c-0240d37f044a/static/mermaid-diagram"

// MermaidMode chooses how a mermaid fence renders.
type MermaidMode int

const (
	// MermaidViewer draws the diagram with the viewer macro and folds the
	// source into an expand beneath it. The zero value.
	MermaidViewer MermaidMode = iota
	// MermaidCode leaves the source as an open code panel, for a site
	// without the viewer installed.
	MermaidCode
)

// LinkTarget is where a relative link goes. At most one of PageTitle or URL
// is set; the zero value means the link could not be placed.
type LinkTarget struct {
	// PageTitle is another exported document, by its page title.
	PageTitle string
	// URL is anything else, as an absolute URL.
	URL string
	// Anchor is the fragment to carry across to PageTitle, already in
	// Confluence's anchor form.
	Anchor string
}

// LinkResolver maps a link as written in the markdown at from (the
// document's repository-relative path) to its target. A zero LinkTarget
// means unresolved: the renderer writes the link text alone and records it
// in Rendered.Links.
type LinkResolver func(from, href string) LinkTarget

// RenderOptions configures one Render. The zero value renders a page with
// no banner, mermaid through the default viewer, and every relative link
// unresolved.
type RenderOptions struct {
	// Resolve places relative links. Nil leaves every one unresolved.
	Resolve LinkResolver
	// Mermaid chooses how mermaid fences render.
	Mermaid MermaidMode
	// ViewerKey is the viewer's extension key. Empty means DefaultViewerKey.
	ViewerKey string
	// Source is the repository-relative path the banner names and the from
	// argument Resolve receives. Empty writes no banner.
	Source string
	// SourceURL is where Source is browsable. Empty makes the banner name
	// the path without a link.
	SourceURL string
	// Title overrides the page title. Empty derives it from the document:
	// "ID: Title" from its frontmatter, then its first H1.
	Title string
}

// Link is a relative link or image the resolver could not place.
type Link struct {
	// Href is the destination as written.
	Href string
	// Text is the link text, or the image's alt text.
	Text string
	// Line is the 1-based line in the document.
	Line int
}

// Rendered is one page's storage-format body and what is known about it.
type Rendered struct {
	// ID is the frontmatter id, empty for a document without one.
	ID string
	// Title is the page title.
	Title string
	// Body is the storage-format page body, well-formed XML.
	Body []byte
	// Hash is "sha256:<hex>" of Body with every viewer macro's local id
	// replaced by a fixed placeholder, so it is stable across runs.
	Hash string
	// Links lists the relative links and in-page anchors the renderer could
	// not place, in document order.
	Links []Link
}

// Render converts one docz document to a Confluence storage-format page.
//
// It never modifies src, touches no filesystem, and keeps no state between
// calls. It fails for CR line endings, for a document with neither
// frontmatter nor opts.Title, for frontmatter that does not parse, and for
// output that is not well-formed XML (*MalformedError), which is how raw
// HTML a renderer cannot place shows itself.
//
//nolint:gocritic // RenderOptions by value is the API (DESIGN-0020); copied once per page.
func Render(src []byte, opts RenderOptions) (Rendered, error) {
	if bytes.IndexByte(src, '\r') >= 0 {
		return Rendered{}, fmt.Errorf("confluence: %w", document.ErrUnsupportedLineEndings)
	}

	body, fm, err := cutFrontmatter(bytes.Clone(src))
	if err != nil && (!errors.Is(err, document.ErrNoFrontmatter) || opts.Title == "") {
		return Rendered{}, fmt.Errorf("confluence: %w", err)
	}

	title := pageTitle(&fm, body, opts.Title)
	body = blankRegion(body, docparse.TocKind)

	st := &storage{
		opts:      opts,
		pageTitle: title,
		headings:  headingText(body),
		src:       body,
	}

	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM, extension.Footnote),
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
			html.WithXHTML(),
			renderer.WithNodeRenderers(util.Prioritized(st, 100)),
		),
	)

	var out bytes.Buffer

	writeBanner(&out, opts.Source, opts.SourceURL)

	if err := md.Convert(body, &out); err != nil {
		return Rendered{}, fmt.Errorf("confluence: rendering: %w", err)
	}

	if err := wellFormed(out.Bytes()); err != nil {
		return Rendered{}, err
	}

	sum := sha256.Sum256(out.Bytes())

	return Rendered{
		ID:    fm.ID,
		Title: title,
		Body:  mintLocalIDs(out.Bytes(), st.viewers),
		Hash:  "sha256:" + hex.EncodeToString(sum[:]),
		Links: st.links,
	}, nil
}

// cutFrontmatter parses the frontmatter and returns the document with the
// frontmatter's lines blanked rather than removed, so every line number the
// renderer reports is the document's own.
func cutFrontmatter(src []byte) ([]byte, document.Frontmatter, error) {
	fm, err := document.ParseFrontmatter(src)
	if err != nil {
		return src, fm, err
	}

	open := bytes.Index(src, []byte("---"))
	closeAt := bytes.Index(src[open+3:], []byte("\n---"))
	end := open + 3 + closeAt + len("\n---")

	if nl := bytes.IndexByte(src[end:], '\n'); nl >= 0 {
		end += nl
	} else {
		end = len(src)
	}

	return blankLines(src, 0, end), fm, nil
}

// blankLines replaces every byte in src[from:to] except newlines with
// nothing, keeping the line count.
func blankLines(src []byte, from, to int) []byte {
	out := make([]byte, 0, len(src))
	out = append(out, src[:from]...)
	out = append(out, bytes.Repeat([]byte("\n"), bytes.Count(src[from:to], []byte("\n")))...)

	return append(out, src[to:]...)
}

// pageTitle is the override, else "ID: Title", else the frontmatter title,
// else the first H1.
func pageTitle(fm *document.Frontmatter, body []byte, override string) string {
	switch {
	case override != "":
		return override
	case fm.ID != "" && fm.Title != "":
		return fm.ID + ": " + fm.Title
	case fm.Title != "":
		return fm.Title
	default:
		return docparse.Title(body)
	}
}

// headingText maps each heading's anchor slug to its text, for in-page
// links.
func headingText(body []byte) map[string]string {
	out := make(map[string]string)
	for _, h := range docparse.Headings(body) {
		out[h.Slug] = h.Text
	}

	return out
}

// anchorID is Confluence Cloud's id for a heading: the page title and the
// heading text with spaces removed, joined by a hyphen. A colon reads as a
// URL scheme to Confluence's sanitizer, which then drops the href, so it is
// percent-encoded.
func anchorID(title, heading string) string {
	id := strings.ReplaceAll(title, " ", "") + "-" + strings.ReplaceAll(heading, " ", "")

	return strings.ReplaceAll(id, ":", "%3A")
}
