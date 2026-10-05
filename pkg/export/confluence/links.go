package confluence

import (
	"bytes"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/util"
)

// linkForm is how one link was written, so its closing tag matches.
type linkForm string

const (
	formAnchor linkForm = "a"       // <a href>
	formPage   linkForm = "ac:link" // <ac:link><ri:page/>
	formText   linkForm = ""        // unresolved: the text alone
	linkAttr            = "docz-link"
)

// link writes an in-page link as Confluence's anchor id, a link to another
// exported document as ri:page, anything else the resolver places as a
// plain href, and an unresolved link as its text alone.
func (s *storage) link(w util.BufWriter, src []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	l, ok := n.(*ast.Link)
	if !ok {
		return ast.WalkContinue, nil
	}

	if !entering {
		switch linkFormOf(n) {
		case formAnchor:
			put(w, "</a>")
		case formPage:
			put(w, "</ac:link-body></ac:link>")
		}

		return ast.WalkContinue, nil
	}

	href := string(l.Destination)
	target := s.target(href)

	var form linkForm

	switch {
	case target.PageTitle != "":
		form = formPage
		put(w, "<ac:link")

		if target.Anchor != "" {
			putf(w, ` ac:anchor="%s"`, escapeAttr(target.Anchor))
		}

		putf(w, `><ri:page ri:content-title="%s"/><ac:link-body>`, escapeAttr(target.PageTitle))
	case target.URL != "":
		form = formAnchor
		putf(w, `<a href="%s">`, escapeAttr(target.URL))
	default:
		s.unresolved(n, href, src)
	}

	n.SetAttributeString(linkAttr, form)

	return ast.WalkContinue, nil
}

// linkFormOf is the form link recorded on entry.
func linkFormOf(n ast.Node) linkForm {
	v, _ := n.AttributeString(linkAttr)
	form, ok := v.(linkForm)

	if !ok {
		return formText
	}

	return form
}

// target resolves a link destination.
func (s *storage) target(href string) LinkTarget {
	if slug, ok := strings.CutPrefix(href, "#"); ok {
		if text, found := s.headings[slug]; found {
			return LinkTarget{URL: "#" + anchorID(s.pageTitle, text)}
		}

		return LinkTarget{}
	}

	if isAbsolute(href) {
		return LinkTarget{URL: href}
	}

	if s.opts.Resolve == nil {
		return LinkTarget{}
	}

	return s.opts.Resolve(s.opts.Source, href)
}

// isAbsolute reports whether a destination names its own scheme.
func isAbsolute(href string) bool {
	return strings.Contains(href, "://") || strings.HasPrefix(href, "mailto:")
}

// image writes a remote image inline, a local one as a link to wherever the
// resolver places it, and an unplaced one as its alt text.
func (s *storage) image(w util.BufWriter, src []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkSkipChildren, nil
	}

	img, ok := n.(*ast.Image)
	if !ok {
		return ast.WalkSkipChildren, nil
	}

	dest := string(img.Destination)
	alt := escapeText(plainText(n, src))

	switch target := s.target(dest); {
	case isAbsolute(dest):
		putf(w, `<ac:image><ri:url ri:value="%s"/></ac:image>`, escapeAttr(dest))
	case target.URL != "":
		putf(w, `<a href="%s">%s</a>`, escapeAttr(target.URL), alt)
	default:
		s.unresolved(n, dest, src)
		putf(w, "<em>%s</em>", alt)
	}

	return ast.WalkSkipChildren, nil
}

// unresolved records a link the renderer could not place.
func (s *storage) unresolved(n ast.Node, href string, src []byte) {
	s.links = append(s.links, Link{Href: href, Text: plainText(n, src), Line: nodeLine(n, src)})
}

// plainText joins the text under n.
func plainText(n ast.Node, src []byte) string {
	var b strings.Builder

	walk(n, func(c ast.Node) bool {
		if t, ok := c.(*ast.Text); ok {
			b.Write(t.Segment.Value(src))
		}

		return true
	})

	return b.String()
}

// nodeLine is the 1-based line n starts on: its first text descendant's, or
// else the nearest enclosing block's first line.
func nodeLine(n ast.Node, src []byte) int {
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

	return bytes.Count(src[:start], []byte("\n")) + 1
}
