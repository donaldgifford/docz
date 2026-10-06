package confluence

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

// storage is the goldmark node renderer that overrides exactly the nodes
// storage format treats differently from XHTML (DESIGN-0020 §2). Every other
// node falls through to goldmark's own HTML renderer. One storage renders one
// page and is then discarded.
type storage struct {
	opts      RenderOptions
	pageTitle string
	headings  map[string]string // anchor slug -> heading text
	src       []byte            // the source with frontmatter blanked
	links     []Link            // unresolved links, in document order
	viewers   int               // viewer macros written, for local ids
}

// RegisterFuncs implements renderer.NodeRenderer.
func (s *storage) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindHeading, s.heading)
	reg.Register(ast.KindFencedCodeBlock, s.fenced)
	reg.Register(ast.KindCodeBlock, s.codeBlock)
	reg.Register(ast.KindHTMLBlock, s.htmlBlock)
	reg.Register(ast.KindRawHTML, s.rawHTML)
	reg.Register(ast.KindLink, s.link)
	reg.Register(ast.KindImage, s.image)
	reg.Register(ast.KindBlockquote, s.blockquote)
	reg.Register(ast.KindList, s.list)
	reg.Register(ast.KindListItem, s.listItem)
	reg.Register(east.KindTaskCheckBox, s.checkbox)
	reg.Register(east.KindTable, s.table)
}

// walk visits n and its descendants in document order until visit returns
// false.
func walk(n ast.Node, visit func(ast.Node) bool) {
	// The visitor never returns an error, so neither does the walk.
	if err := ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering && !visit(c) {
			return ast.WalkStop, nil
		}

		return ast.WalkContinue, nil
	}); err != nil {
		return
	}
}

// heading drops the H1, which the page title carries, and writes H2 to H6
// with no id: Confluence assigns its own.
func (*storage) heading(w util.BufWriter, _ []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	h, ok := n.(*ast.Heading)
	if !ok || h.Level == 1 {
		return ast.WalkSkipChildren, nil
	}

	if entering {
		putf(w, "<h%d>", h.Level)
	} else {
		putf(w, "</h%d>\n", h.Level)
	}

	return ast.WalkContinue, nil
}

// fenced writes a fenced block as a code macro, or a mermaid fence as the
// viewer macro with its source folded beneath it.
func (s *storage) fenced(w util.BufWriter, src []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkSkipChildren, nil
	}

	f, ok := n.(*ast.FencedCodeBlock)
	if !ok {
		return ast.WalkSkipChildren, nil
	}

	lang := strings.ToLower(string(f.Language(src)))
	body := blockLines(n, src)

	if lang != "mermaid" {
		writeCode(w, codeLanguage(lang), body)

		return ast.WalkSkipChildren, nil
	}

	if s.opts.Mermaid == MermaidCode {
		writeCode(w, "mermaid", body)

		return ast.WalkSkipChildren, nil
	}

	key := s.opts.ViewerKey
	if key == "" {
		key = DefaultViewerKey
	}

	writeViewer(w, key, localIDPlaceholder(s.viewers))
	s.viewers++
	writeExpand(w, "Diagram source", func() { writeCode(w, "mermaid", body) })

	return ast.WalkSkipChildren, nil
}

// codeBlock writes an indented code block as a code macro with no language.
func (*storage) codeBlock(w util.BufWriter, src []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		writeCode(w, "none", blockLines(n, src))
	}

	return ast.WalkSkipChildren, nil
}

// htmlBlock turns the ToC placeholder into the toc macro, drops every
// comment (region markers included), and passes or escapes the rest.
func (*storage) htmlBlock(w util.BufWriter, src []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkSkipChildren, nil
	}

	raw := strings.TrimSpace(string(blockLines(n, src)))

	switch {
	case raw == tocPlaceholder:
		writeTOC(w)
	case strings.HasPrefix(raw, "<!--"):
	default:
		// An escaped block is text, and text at the top level of a page
		// belongs in a paragraph.
		if out := passOrEscape(raw); out == escapeText(raw) {
			put(w, "<p>"+out+"</p>\n")
		} else {
			put(w, out+"\n")
		}
	}

	return ast.WalkSkipChildren, nil
}

// rawHTML applies the block rule to inline HTML, node by node.
func (*storage) rawHTML(w util.BufWriter, src []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkSkipChildren, nil
	}

	r, ok := n.(*ast.RawHTML)
	if !ok {
		return ast.WalkSkipChildren, nil
	}

	var b bytes.Buffer
	for i := range r.Segments.Len() {
		seg := r.Segments.At(i)
		b.Write(seg.Value(src))
	}

	if raw := b.String(); !strings.HasPrefix(raw, "<!--") {
		put(w, passOrEscape(raw))
	}

	return ast.WalkSkipChildren, nil
}

// allowedTags is the raw HTML that passes through (DESIGN-0020 §2).
var allowedTags = map[string]bool{
	"br": true, "kbd": true, "sub": true, "sup": true,
	"span": true, "details": true, "summary": true,
}

var (
	tagPattern = regexp.MustCompile(`</?([a-zA-Z][a-zA-Z0-9-]*)\b[^>]*>`)
	brPattern  = regexp.MustCompile(`(?i)<br\s*/?>`)
)

// passOrEscape keeps a fragment whose every tag is allowed, with <br> made
// self-closing, and escapes anything else whole, so a <status> placeholder
// in prose stays text.
func passOrEscape(raw string) string {
	tags := tagPattern.FindAllStringSubmatch(raw, -1)
	ok := len(tags) > 0

	for _, m := range tags {
		if !allowedTags[strings.ToLower(m[1])] {
			ok = false

			break
		}
	}

	if ok {
		return brPattern.ReplaceAllString(raw, "<br/>")
	}

	return escapeText(raw)
}

// table writes the table with the layout attribute that widens it alone,
// leaving the page centred (INV-0019 Observation 19).
func (*storage) table(w util.BufWriter, _ []byte, _ ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		put(w, "<table data-layout=\"full-width\">\n")
	} else {
		put(w, "</table>\n")
	}

	return ast.WalkContinue, nil
}

// alertMarker matches a GitHub alert's first line.
var alertMarker = regexp.MustCompile(`^\[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION)\]$`)

// alertPanel maps an alert to its panel macro.
var alertPanel = map[string]string{
	"NOTE": "info", "TIP": "tip", "IMPORTANT": "note", "WARNING": "warning", "CAUTION": "warning",
}

const alertAttr = "docz-alert"

// blockquote writes an alert as its panel macro, with the marker cut from
// the AST before the walk descends, and any other blockquote as itself.
func (*storage) blockquote(w util.BufWriter, src []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		panel := alertKind(n, src)
		n.SetAttributeString(alertAttr, panel)

		if panel == "" {
			put(w, "<blockquote>\n")

			return ast.WalkContinue, nil
		}

		stripAlertMarker(n, src)
		putf(w, `<ac:structured-macro ac:name="%s"><ac:rich-text-body>`, panel)

		return ast.WalkContinue, nil
	}

	if alertPanelOf(n) != "" {
		put(w, "</ac:rich-text-body></ac:structured-macro>\n")
	} else {
		put(w, "</blockquote>\n")
	}

	return ast.WalkContinue, nil
}

// firstLine joins the text nodes of a paragraph's first line. goldmark
// splits "[!NOTE]" into "[" and "!NOTE]", since the bracket opens a link
// candidate, so the marker is never one node.
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

// alertKind returns the panel macro for an alert blockquote, or "".
func alertKind(n ast.Node, src []byte) string {
	p, ok := n.FirstChild().(*ast.Paragraph)
	if !ok {
		return ""
	}

	line, _ := firstLine(p, src)

	m := alertMarker.FindStringSubmatch(strings.TrimSpace(line))
	if m == nil {
		return ""
	}

	return alertPanel[m[1]]
}

// alertPanelOf is the panel blockquote recorded on entry, or "".
func alertPanelOf(n ast.Node) string {
	v, _ := n.AttributeString(alertAttr)
	panel, ok := v.(string)

	if !ok {
		return ""
	}

	return panel
}

// stripAlertMarker removes the marker line from the alert's first
// paragraph.
func stripAlertMarker(n ast.Node, src []byte) {
	p := n.FirstChild()

	_, nodes := firstLine(p, src)
	for _, c := range nodes {
		p.RemoveChild(p, c)
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

// list writes a task list as ac:task-list and any other list as itself.
func (*storage) list(w util.BufWriter, _ []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	l, ok := n.(*ast.List)
	if !ok {
		return ast.WalkContinue, nil
	}

	tag := "ul"

	switch {
	case isTaskList(n):
		tag = "ac:task-list"
	case l.IsOrdered():
		tag = "ol"
	}

	switch {
	case !entering:
		putf(w, "</%s>\n", tag)
	case tag == "ol" && l.Start > 1:
		putf(w, "<ol start=\"%d\">\n", l.Start)
	default:
		putf(w, "<%s>\n", tag)
	}

	return ast.WalkContinue, nil
}

// listItem writes a task list's items as ac:task with their status.
func (*storage) listItem(w util.BufWriter, _ []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	task := isTaskList(n.Parent())

	switch {
	case !entering && task:
		put(w, "</ac:task-body></ac:task>\n")
	case !entering:
		put(w, "</li>\n")
	case task:
		status := "incomplete"

		if tb := n.FirstChild(); tb != nil {
			if cb, ok := tb.FirstChild().(*east.TaskCheckBox); ok && cb.IsChecked {
				status = "complete"
			}
		}

		putf(w, "<ac:task><ac:task-status>%s</ac:task-status><ac:task-body>", status)
	default:
		put(w, "<li>")
	}

	return ast.WalkContinue, nil
}

// checkbox writes nothing; listItem carries the status.
func (*storage) checkbox(util.BufWriter, []byte, ast.Node, bool) (ast.WalkStatus, error) {
	return ast.WalkSkipChildren, nil
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
