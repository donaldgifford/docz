package html

import (
	"bytes"
	"regexp"

	"github.com/yuin/goldmark/ast"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
)

// resolutionLine opens a blockquote that records a question's resolution
// or a revision of it.
var resolutionLine = regexp.MustCompile(`^\s*>\s*\*{0,2}(Resolved|Revised)\b`)

// questions writes one expandable per open question kinds.OpenQuestions
// finds, carrying its heading's anchor: the options as a list with the
// recommended one marked, and each resolution or revision paragraph as a
// p.resolution. A resolved question is closed, an open one open. Prose
// above the first question stays text.
func (d *doc) questions(w *bytes.Buffer, s section) (bool, error) {
	r, ok := d.region(s)
	if !ok {
		return false, nil
	}

	qs := kinds.ShiftQuestions(kinds.OpenQuestions(kinds.RegionBytes(d.body, r)), r)
	if len(qs) == 0 {
		return false, nil
	}

	if err := d.text(w, s.Body, qs[0].Line-1); err != nil {
		return false, err
	}

	for i := range qs {
		end := s.Through
		if i+1 < len(qs) {
			end = qs[i+1].Line - 1
		}

		if err := d.question(w, &qs[i], end); err != nil {
			return false, err
		}
	}

	return true, nil
}

// question writes one question, from its heading through end.
func (d *doc) question(w *bytes.Buffer, q *kinds.Question, end int) error {
	h := d.headingAt(q.Line)
	if h == nil {
		return d.text(w, q.Line, end)
	}

	title, err := d.headingText(h)
	if err != nil {
		return err
	}

	open := ""
	if q.Resolved == nil {
		open = " open"
	}

	put(w, `<details class="expand"`+idAttr(h)+open+">\n<summary>")
	w.Write(title)
	put(w, "</summary>\n"+`<div class="body">`+"\n")

	recommended := make(map[int]bool, len(q.Options))
	for _, o := range q.Options {
		recommended[o.Line] = o.Recommended
	}

	replace := make(map[int]replacer)

	for n := d.md.root.FirstChild(); n != nil; n = n.NextSibling() {
		line := d.md.startLine(n)
		if line <= q.Line || line > end {
			continue
		}

		switch n.(type) {
		case *ast.List:
			replace[line] = func(w *bytes.Buffer, n ast.Node) error {
				return d.options(w, n, recommended)
			}
		case *ast.Blockquote:
			if line <= len(d.lines) && resolutionLine.MatchString(d.lines[line-1]) {
				replace[line] = d.resolution
			}
		}
	}

	if err := d.textWith(w, q.Line+1, end, replace); err != nil {
		return err
	}

	put(w, "</div>\n</details>\n")

	return nil
}

// options writes a question's list with each lettered option as li.option,
// the recommended one li.option.recommended, and any other item as it is.
func (d *doc) options(w *bytes.Buffer, list ast.Node, recommended map[int]bool) error {
	put(w, "<ul>\n")

	for it := list.FirstChild(); it != nil; it = it.NextSibling() {
		rec, isOption := recommended[d.md.startLine(it)]

		switch {
		case rec:
			put(w, `<li class="option recommended">`)
		case isOption:
			put(w, `<li class="option">`)
		default:
			put(w, "<li>")
		}

		if err := d.itemContent(w, it); err != nil {
			return err
		}

		put(w, "</li>\n")
	}

	put(w, "</ul>\n")

	return nil
}

// resolution writes a resolution blockquote's paragraphs, each its own
// p.resolution, in document order.
func (d *doc) resolution(w *bytes.Buffer, quote ast.Node) error {
	for c := quote.FirstChild(); c != nil; c = c.NextSibling() {
		if _, ok := c.(*ast.Paragraph); !ok {
			if err := d.md.render(w, c); err != nil {
				return err
			}

			continue
		}

		put(w, `<p class="resolution">`)

		var buf bytes.Buffer
		if err := d.md.children(&buf, c); err != nil {
			return err
		}

		w.Write(bytes.TrimSpace(buf.Bytes()))
		put(w, "</p>\n")
	}

	return nil
}
