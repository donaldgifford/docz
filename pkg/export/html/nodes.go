package html

import (
	"bytes"
	"fmt"
	"html"
	"io"
	"regexp"
	"strings"

	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
)

// nodeRenderer overrides the nodes a docz page treats differently from
// goldmark's HTML: headings get docz's anchors and drop a level, so the
// panel title is the page's only h2; raw HTML is rebuilt from an
// allow-list; alerts are callouts; task lists are read-only; tables
// scroll; links and images go through the resolver. Every other node falls
// through to goldmark's renderer, which escapes text. One nodeRenderer
// renders one page.
type nodeRenderer struct {
	opts *RenderOptions
	src  []byte
	line func(off int) int

	// ids maps a heading's line to its anchor, as docparse.Headings
	// assigns them; taken holds every anchor in use.
	ids   map[int]string
	taken map[string]int

	// open is the stack of allowed raw tags opened and not yet closed.
	open []string
	// marks holds len(open) at the start of each open inline container,
	// so a raw tag opened inside a paragraph closes inside it.
	marks []int

	links    []Link
	problems []error

	// mermaid reports that a diagram was written for mermaid to draw.
	mermaid bool
}

// newNodeRenderer seeds the heading anchors from docparse, so an id on the
// page and a #slug link in the text agree with every other docz consumer.
func newNodeRenderer(opts *RenderOptions, headings []docparse.Heading) *nodeRenderer {
	nr := &nodeRenderer{opts: opts, ids: make(map[int]string), taken: make(map[string]int)}

	for _, h := range headings {
		nr.ids[h.Line] = h.Slug
		nr.taken[h.Slug] = 0
	}

	return nr
}

// RegisterFuncs implements renderer.NodeRenderer.
func (nr *nodeRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindHeading, nr.heading)
	reg.Register(ast.KindParagraph, nr.paragraph)
	reg.Register(ast.KindTextBlock, nr.textBlock)
	reg.Register(ast.KindFencedCodeBlock, nr.fenced)
	reg.Register(ast.KindCodeBlock, nr.codeBlock)
	reg.Register(ast.KindHTMLBlock, nr.htmlBlock)
	reg.Register(ast.KindRawHTML, nr.rawHTML)
	reg.Register(ast.KindLink, nr.link)
	reg.Register(ast.KindAutoLink, nr.autoLink)
	reg.Register(ast.KindImage, nr.image)
	reg.Register(ast.KindBlockquote, nr.blockquote)
	reg.Register(ast.KindList, nr.list)
	reg.Register(ast.KindListItem, nr.listItem)
	reg.Register(east.KindTaskCheckBox, nr.checkbox)
	reg.Register(east.KindTable, nr.table)
}

// heading writes H2 to H6 one level down, with docz's anchor. The H1 is
// the page title and is dropped.
func (nr *nodeRenderer) heading(w util.BufWriter, src []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	h, ok := n.(*ast.Heading)
	if !ok || h.Level == 1 {
		return ast.WalkSkipChildren, nil
	}

	level := min(h.Level+1, 6)

	if !entering {
		nr.closeInline(w)
		putf(w, "</h%d>\n", level)

		return ast.WalkContinue, nil
	}

	putf(w, `<h%d id="%s">`, level, html.EscapeString(nr.anchor(h, src)))
	nr.openInline()

	return ast.WalkContinue, nil
}

// paragraph writes a paragraph, closing any raw tag opened in it.
func (nr *nodeRenderer) paragraph(w util.BufWriter, _ []byte, _ ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		put(w, "<p>")
		nr.openInline()

		return ast.WalkContinue, nil
	}

	nr.closeInline(w)
	put(w, "</p>\n")

	return ast.WalkContinue, nil
}

// textBlock writes a tight list item's text as goldmark does, closing any
// raw tag opened in it.
func (nr *nodeRenderer) textBlock(w util.BufWriter, _ []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		nr.openInline()

		return ast.WalkContinue, nil
	}

	nr.closeInline(w)

	if n.NextSibling() != nil && n.FirstChild() != nil {
		put(w, "\n")
	}

	return ast.WalkContinue, nil
}

// openInline marks the start of an inline container.
func (nr *nodeRenderer) openInline() {
	nr.marks = append(nr.marks, len(nr.open))
}

// closeInline closes the raw tags opened since the matching openInline.
func (nr *nodeRenderer) closeInline(w util.BufWriter) {
	if len(nr.marks) == 0 {
		return
	}

	m := nr.marks[len(nr.marks)-1]
	nr.marks = nr.marks[:len(nr.marks)-1]

	for i := len(nr.open) - 1; i >= m && i < len(nr.open); i-- {
		put(w, "</"+nr.open[i]+">")
	}

	if m < len(nr.open) {
		nr.open = nr.open[:m]
	}
}

// anchor is the heading's id: docparse's for a heading it knows, else
// github-slugger's rule over the text, unique among the page's ids.
// goldmark sees headings docparse does not (setext, inside a blockquote).
func (nr *nodeRenderer) anchor(h *ast.Heading, src []byte) string {
	if h.Lines().Len() > 0 {
		if id, ok := nr.ids[nr.line(h.Lines().At(0).Start)]; ok {
			return id
		}
	}

	return uniqueSlug(nr.taken, docparse.AnchorSlug(plainText(h, src)))
}

// uniqueSlug copies docparse's duplicate rule: the first use of a slug is
// bare, later ones take -1, -2, and so on, skipping any already taken.
func uniqueSlug(taken map[string]int, slug string) string {
	result := slug

	for {
		if _, ok := taken[result]; !ok {
			taken[result] = 0

			return result
		}

		taken[slug]++
		result = fmt.Sprintf("%s-%d", slug, taken[slug])
	}
}

// fenced writes a code block with its language as a class; a mermaid
// fence is drawn by mermaid unless the mode leaves it as source.
func (nr *nodeRenderer) fenced(w util.BufWriter, src []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	f, ok := n.(*ast.FencedCodeBlock)
	if !entering || !ok {
		return ast.WalkSkipChildren, nil
	}

	lang := strings.ToLower(string(f.Language(src)))
	body := blockLines(n, src)

	if lang == "mermaid" {
		nr.writeMermaid(w, body)

		return ast.WalkSkipChildren, nil
	}

	writeCode(w, lang, body)

	return ast.WalkSkipChildren, nil
}

// codeBlock writes an indented code block with no language.
func (*nodeRenderer) codeBlock(w util.BufWriter, src []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		writeCode(w, "", blockLines(n, src))
	}

	return ast.WalkSkipChildren, nil
}

// writeCode writes a pre/code pair, the body escaped.
func writeCode(w io.Writer, lang string, body []byte) {
	if lang != "" {
		putf(w, `<pre><code class="language-%s">`, html.EscapeString(lang))
	} else {
		put(w, "<pre><code>")
	}

	put(w, html.EscapeString(string(body)))
	put(w, "</code></pre>\n")
}

// htmlBlock drops comments (region markers included) and rebuilds the
// rest from the allow-list.
func (nr *nodeRenderer) htmlBlock(w util.BufWriter, src []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkSkipChildren, nil
	}

	raw := strings.TrimSpace(string(blockLines(n, src)))
	if raw == "" || isComment(raw) {
		return ast.WalkSkipChildren, nil
	}

	out, passed := nr.rebuild(raw)
	if !passed {
		// Escaped, the block is text, and text belongs in a paragraph.
		out = "<p>" + out + "</p>"
	}

	put(w, out+"\n")

	return ast.WalkSkipChildren, nil
}

// rawHTML applies the block rule to inline HTML.
func (nr *nodeRenderer) rawHTML(w util.BufWriter, src []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	r, ok := n.(*ast.RawHTML)
	if !entering || !ok {
		return ast.WalkSkipChildren, nil
	}

	var b strings.Builder
	for i := range r.Segments.Len() {
		seg := r.Segments.At(i)
		b.Write(seg.Value(src))
	}

	if raw := b.String(); !isComment(raw) {
		out, _ := nr.rebuild(raw)
		put(w, out)
	}

	return ast.WalkSkipChildren, nil
}

// isComment reports a fragment that is one or more comments and nothing
// else.
func isComment(raw string) bool {
	return strings.HasPrefix(raw, "<!--") && strings.TrimSpace(stripComments(raw)) == ""
}

// alertMarker matches a GitHub alert's first line.
var alertMarker = regexp.MustCompile(`^\[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION)\]$`)

// alertAttr records an alert's kind on its blockquote for the exit.
const alertAttr = "docz-alert"

// blockquote writes an alert as a callout titled with its kind, and any
// other blockquote as itself.
func (*nodeRenderer) blockquote(w util.BufWriter, src []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		kind := alertKind(n, src)
		n.SetAttributeString(alertAttr, kind)

		if kind == "" {
			put(w, "<blockquote>\n")

			return ast.WalkContinue, nil
		}

		stripAlertMarker(n, src)

		lower := strings.ToLower(kind)
		putf(w, `<div class="callout %s"><p class="callout-title">%s</p>`+"\n", lower,
			strings.ToUpper(lower[:1])+lower[1:])

		return ast.WalkContinue, nil
	}

	if v, _ := n.AttributeString(alertAttr); v != "" {
		put(w, "</div>\n")
	} else {
		put(w, "</blockquote>\n")
	}

	return ast.WalkContinue, nil
}

// alertKind is an alert blockquote's kind, NOTE through CAUTION, or "".
func alertKind(n ast.Node, src []byte) string {
	p, ok := n.FirstChild().(*ast.Paragraph)
	if !ok {
		return ""
	}

	line, _ := firstLine(p, src)
	if m := alertMarker.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
		return m[1]
	}

	return ""
}

// firstLine joins the text nodes of a paragraph's first line. goldmark
// splits "[!NOTE]" into "[" and "!NOTE]", since the bracket opens a link
// candidate, so the marker is never one node. pkg/export/confluence has the
// same helper.
func firstLine(p ast.Node, src []byte) (string, []ast.Node) {
	var (
		b     strings.Builder
		nodes []ast.Node
	)

	for c := p.FirstChild(); c != nil; c = c.NextSibling() {
		t, ok := c.(*ast.Text)
		if !ok {
			break
		}

		b.Write(t.Segment.Value(src))
		nodes = append(nodes, c)

		if t.SoftLineBreak() || t.HardLineBreak() {
			break
		}
	}

	return b.String(), nodes
}

// stripAlertMarker removes the marker line from the alert's first
// paragraph, and the paragraph when nothing else was in it.
func stripAlertMarker(n ast.Node, src []byte) {
	p := n.FirstChild()

	_, nodes := firstLine(p, src)
	for _, c := range nodes {
		p.RemoveChild(p, c)
	}

	if p.ChildCount() == 0 {
		n.RemoveChild(n, p)
	}
}

// isTaskList reports whether a list's first item opens with a checkbox.
func isTaskList(n ast.Node) bool {
	li := n.FirstChild()
	if li == nil || li.FirstChild() == nil {
		return false
	}

	_, ok := li.FirstChild().FirstChild().(*east.TaskCheckBox)

	return ok
}

// list writes a task list as ul.tasks and any other list as itself.
func (*nodeRenderer) list(w util.BufWriter, _ []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	l, ok := n.(*ast.List)
	if !ok {
		return ast.WalkContinue, nil
	}

	tag := "ul"
	if l.IsOrdered() {
		tag = "ol"
	}

	switch {
	case !entering:
		putf(w, "</%s>\n", tag)
	case isTaskList(n):
		put(w, `<ul class="tasks">`+"\n")
	case l.IsOrdered() && l.Start > 1:
		putf(w, "<ol start=\"%d\">\n", l.Start)
	default:
		putf(w, "<%s>\n", tag)
	}

	return ast.WalkContinue, nil
}

// listItem writes a task item as a disabled checkbox and its text, with
// its state for the filter.
func (*nodeRenderer) listItem(w util.BufWriter, _ []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	cb := itemCheckBox(n)

	switch {
	case !entering && cb != nil:
		put(w, "</div></li>\n")
	case !entering:
		put(w, "</li>\n")
	case cb != nil && cb.IsChecked:
		put(w, `<li data-state="done"><input type="checkbox" checked disabled><div class="text">`)
	case cb != nil:
		put(w, `<li data-state="open"><input type="checkbox" disabled><div class="text">`)
	default:
		put(w, "<li>")
	}

	return ast.WalkContinue, nil
}

// itemCheckBox is a list item's checkbox, nil when it has none.
func itemCheckBox(li ast.Node) *east.TaskCheckBox {
	if li.FirstChild() == nil {
		return nil
	}

	if cb, ok := li.FirstChild().FirstChild().(*east.TaskCheckBox); ok {
		return cb
	}

	return nil
}

// checkbox writes nothing; listItem writes it.
func (*nodeRenderer) checkbox(util.BufWriter, []byte, ast.Node, bool) (ast.WalkStatus, error) {
	return ast.WalkSkipChildren, nil
}

// table wraps a table so it scrolls sideways on a narrow screen.
func (*nodeRenderer) table(w util.BufWriter, _ []byte, _ ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		put(w, "<div class=\"table-wrap\">\n<table>\n")
	} else {
		put(w, "</table>\n</div>\n")
	}

	return ast.WalkContinue, nil
}

// blockLines joins a block node's lines.
func blockLines(n ast.Node, src []byte) []byte {
	var b bytes.Buffer

	lines := n.Lines()
	for i := range lines.Len() {
		seg := lines.At(i)
		b.Write(seg.Value(src))
	}

	return b.Bytes()
}

// plainText joins the text under n.
func plainText(n ast.Node, src []byte) string {
	var b strings.Builder

	walk(n, func(c ast.Node) bool {
		switch t := c.(type) {
		case *ast.Text:
			b.Write(t.Segment.Value(src))
		case *ast.String:
			b.Write(t.Value)
		}

		return true
	})

	return b.String()
}

func put(w io.Writer, s string) {
	_, _ = io.WriteString(w, s) //nolint:errcheck // goldmark's BufWriter reports errors on Flush.
}

func putf(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...) //nolint:errcheck // as put.
}

// writeMermaid writes a mermaid fence: a labelled code block in
// MermaidSource mode, else a pre.mermaid holding the escaped source for the
// page's mermaid to draw.
func (nr *nodeRenderer) writeMermaid(w util.BufWriter, body []byte) {
	if nr.opts.Mermaid == MermaidSource {
		put(w, `<p class="card-label">Diagram (mermaid source)</p>`+"\n")
		writeCode(w, "mermaid", body)

		return
	}

	nr.mermaid = true

	put(w, `<pre class="mermaid">`+html.EscapeString(string(body))+"</pre>\n")
}
