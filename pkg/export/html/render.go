package html

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/document"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
)

// MermaidMode chooses how a mermaid fence renders.
type MermaidMode int

const (
	// MermaidEmbed draws diagrams with mermaid inlined in the page, on
	// pages that have one. The zero value.
	MermaidEmbed MermaidMode = iota
	// MermaidSource leaves each diagram as a labelled code block.
	MermaidSource
	// MermaidCDN draws diagrams with mermaid loaded from a pinned URL.
	MermaidCDN
)

// LinkResolver maps a relative link as written in the markdown at from
// (the document's repository-relative path) to a URL: a relative path to
// another page, or an absolute URL. An empty result means unresolved: the
// page shows the link's text alone and records it in [Page.Links].
type LinkResolver func(from, href string) string

// RenderOptions configures one Render. The zero value renders a paper page
// with every relative link unresolved and no eyebrow site.
type RenderOptions struct {
	// Theme is a pkg/theme name. Empty means "paper".
	Theme string
	// Mermaid chooses how mermaid fences render.
	Mermaid MermaidMode
	// Spec is the type's heading spec, for an unmarked document's regions.
	Spec kinds.HeadingSpec
	// Type is the document's canonical type name; it sets [Page.Type] and
	// the type's label in the eyebrow.
	Type string
	// Source is the repository-relative path the footer names and the from
	// argument Resolve receives.
	Source string
	// Resolve places relative links. Nil leaves every one unresolved.
	Resolve LinkResolver
	// Site is the eyebrow's first part, the repository's name. Empty
	// leaves the type's label alone.
	Site string
	// Version is the docz version the footer names.
	Version string
}

// Link is a relative link or image the resolver could not place.
type Link struct {
	// Href is the destination as written.
	Href string `json:"href"`
	// Text is the link text, or the image's alt text.
	Text string `json:"text"`
	// Line is the 1-based line in the document.
	Line int `json:"line"`
}

// Page is one rendered document.
type Page struct {
	// ID is the frontmatter id, Title its title, Type the type rendered as.
	ID, Title, Type string
	// Body is the whole HTML page.
	Body []byte
	// Hash is "sha256:<hex>" of Body.
	Hash string
	// Links lists the relative links and in-page anchors the page could not
	// place, in document order.
	Links []Link
	// Inferred reports that the regions were inferred from headings.
	Inferred bool
	// Problems lists what fell back to plain rendering, such as a widget
	// whose YAML did not validate.
	Problems []error
}

// ErrNoFrontmatter reports a document with no frontmatter block, which
// gives a page no id and no title. It wraps document.ErrNoFrontmatter.
var ErrNoFrontmatter = fmt.Errorf("html: %w", document.ErrNoFrontmatter)

// ErrLineEndings reports a document with CR or CRLF line endings. It wraps
// document.ErrUnsupportedLineEndings.
var ErrLineEndings = fmt.Errorf("html: %w", document.ErrUnsupportedLineEndings)

// Render converts one docz document to a standalone HTML page.
//
// It never modifies src, touches no filesystem, and keeps no state between
// calls. It fails for CR line endings ([ErrLineEndings]), for a document
// with no frontmatter ([ErrNoFrontmatter]), for frontmatter that does not
// parse, and for an unknown theme.
//
//nolint:gocritic // RenderOptions by value is the API (DESIGN-0025); copied once per page.
func Render(src []byte, opts RenderOptions) (Page, error) {
	if bytes.IndexByte(src, '\r') >= 0 {
		return Page{}, ErrLineEndings
	}

	fm, err := document.ParseFrontmatter(src)
	if errors.Is(err, document.ErrNoFrontmatter) {
		return Page{}, ErrNoFrontmatter
	}
	if err != nil {
		return Page{}, fmt.Errorf("html: %w", err)
	}

	return Page{ID: fm.ID, Title: fm.Title, Type: opts.Type}, nil
}
