package html

import (
	"html"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/util"
)

// linkAttr records on a link whether its <a> was written, for the exit.
const linkAttr = "docz-link"

// link writes a link whose destination is safe and placed as <a>, and
// anything else as its text alone, recorded in Page.Links when it was a
// relative link or an anchor with no heading.
func (nr *nodeRenderer) link(w util.BufWriter, src []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	l, ok := n.(*ast.Link)
	if !ok {
		return ast.WalkContinue, nil
	}

	if !entering {
		if v, _ := n.AttributeString(linkAttr); v == true {
			put(w, "</a>")
		}

		return ast.WalkContinue, nil
	}

	href := string(l.Destination)

	url := nr.target(href)
	if url == "" {
		nr.unresolved(n, href, src)
		n.SetAttributeString(linkAttr, false)

		return ast.WalkContinue, nil
	}

	put(w, `<a href="`+html.EscapeString(url)+`"`)
	if len(l.Title) > 0 {
		put(w, ` title="`+html.EscapeString(string(l.Title))+`"`)
	}

	put(w, ">")
	n.SetAttributeString(linkAttr, true)

	return ast.WalkContinue, nil
}

// autoLink writes an autolink when its scheme is safe, else its text.
func (*nodeRenderer) autoLink(w util.BufWriter, src []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	a, ok := n.(*ast.AutoLink)
	if !entering || !ok {
		return ast.WalkSkipChildren, nil
	}

	label := string(a.Label(src))
	url := string(a.URL(src))

	if a.AutoLinkType == ast.AutoLinkEmail && !strings.HasPrefix(strings.ToLower(url), "mailto:") {
		url = "mailto:" + url
	}

	if !safeScheme(url) {
		put(w, html.EscapeString(label))

		return ast.WalkSkipChildren, nil
	}

	put(w, `<a href="`+html.EscapeString(url)+`">`+html.EscapeString(label)+`</a>`)

	return ast.WalkSkipChildren, nil
}

// image writes an image whose source the page's policy can load (https
// or a data: URL) as <img>, any other placed source as a link to it, and
// an unplaced one as its alt text, recorded.
func (nr *nodeRenderer) image(w util.BufWriter, src []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	img, ok := n.(*ast.Image)
	if !entering || !ok {
		return ast.WalkSkipChildren, nil
	}

	dest := string(img.Destination)
	alt := html.EscapeString(plainText(n, src))
	url := nr.target(dest)

	switch {
	case strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "data:image/"):
		put(w, `<img src="`+html.EscapeString(url)+`" alt="`+alt+`">`)
	case url != "":
		put(w, `<a href="`+html.EscapeString(url)+`">`+alt+`</a>`)
	default:
		nr.unresolved(n, dest, src)
		put(w, "<em>"+alt+"</em>")
	}

	return ast.WalkSkipChildren, nil
}

// target places a destination: an in-page anchor when a heading has it, an
// absolute URL with a safe scheme as itself, a relative one through the
// resolver. "" means it could not be placed.
func (nr *nodeRenderer) target(href string) string {
	if slug, ok := strings.CutPrefix(href, "#"); ok {
		if _, found := nr.taken[slug]; found {
			return href
		}

		return ""
	}

	if hasScheme(href) {
		if safeScheme(href) {
			return href
		}

		return ""
	}

	if nr.opts.Resolve == nil {
		return ""
	}

	url := nr.opts.Resolve(nr.opts.Source, href)
	if hasScheme(url) && !safeScheme(url) && !strings.HasPrefix(url, "data:image/") {
		return ""
	}

	return url
}

// hasScheme reports a destination that names a scheme.
func hasScheme(href string) bool {
	i := strings.IndexByte(href, ':')
	if i <= 0 {
		return false
	}

	return !strings.ContainsAny(href[:i], "/?#")
}

// safeScheme reports an absolute URL a page may link to.
func safeScheme(href string) bool {
	h := strings.ToLower(href)

	return strings.HasPrefix(h, "https://") || strings.HasPrefix(h, "http://") ||
		strings.HasPrefix(h, "mailto:")
}

// unresolved records a link the page could not place, unless it was an
// absolute URL with an unsafe scheme, which is not a link to fix.
func (nr *nodeRenderer) unresolved(n ast.Node, href string, src []byte) {
	nr.links = append(nr.links, Link{Href: href, Text: plainText(n, src), Line: nr.nodeLine(n)})
}

// nodeLine is the line a node starts on: its first text's, else the
// nearest enclosing block's first line.
func (nr *nodeRenderer) nodeLine(n ast.Node) int {
	start := -1

	walk(n, func(c ast.Node) bool {
		if t, ok := c.(*ast.Text); ok {
			start = t.Segment.Start

			return false
		}

		return true
	})

	for p := n; start < 0 && p != nil; p = p.Parent() {
		if p.Type() == ast.TypeBlock && p.Lines().Len() > 0 {
			start = p.Lines().At(0).Start
		}
	}

	if start < 0 {
		return 0
	}

	return nr.line(start)
}
