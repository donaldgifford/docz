package html

import (
	"bytes"
	"sort"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// markdown is one document parsed once, with what is needed to render it a
// span at a time: the AST, a line index, and the node renderer whose state
// (heading ids, open raw tags, unresolved links) is the page's.
type markdown struct {
	src   []byte
	root  ast.Node
	md    goldmark.Markdown
	nodes *nodeRenderer
	// starts[i] is the byte offset of line i+1.
	starts []int
}

// parseMarkdown parses body, the document with its frontmatter blanked.
func parseMarkdown(body []byte, nr *nodeRenderer) *markdown {
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM, extension.Footnote),
		goldmark.WithRendererOptions(
			renderer.WithNodeRenderers(util.Prioritized(nr, 100)),
		),
	)

	m := &markdown{
		src:   body,
		root:  md.Parser().Parse(text.NewReader(body), parser.WithContext(parser.NewContext())),
		md:    md,
		nodes: nr,
	}

	m.starts = append(m.starts, 0)
	for i, c := range body {
		if c == '\n' {
			m.starts = append(m.starts, i+1)
		}
	}

	nr.src = body
	nr.line = m.lineOf

	return m
}

// lineOf is the 1-based line holding byte offset off.
func (m *markdown) lineOf(off int) int {
	return sort.Search(len(m.starts), func(i int) bool { return m.starts[i] > off })
}

// startLine is the line a node starts on: its own first line, else its
// first descendant's that has one. A node with neither is 0.
func (m *markdown) startLine(n ast.Node) int {
	off := -1

	walk(n, func(c ast.Node) bool {
		if c.Type() == ast.TypeBlock && c.Lines().Len() > 0 {
			off = c.Lines().At(0).Start

			return false
		}

		if t, ok := c.(*ast.Text); ok {
			off = t.Segment.Start

			return false
		}

		return true
	})

	if off < 0 {
		return 0
	}

	return m.lineOf(off)
}

// span renders the top-level nodes that start on lines from through
// through, skipping the heading on line skip (0 for none), and closes any
// raw tag left open inside the span.
func (m *markdown) span(from, through, skip int) ([]byte, error) {
	var out bytes.Buffer

	r := m.md.Renderer()

	for n := m.root.FirstChild(); n != nil; n = n.NextSibling() {
		if _, ok := n.(*east.FootnoteList); ok {
			continue
		}

		line := m.startLine(n)
		if line < from || line > through {
			continue
		}

		if h, ok := n.(*ast.Heading); ok && line == skip && h.Level > 1 {
			continue
		}

		if err := r.Render(&out, m.src, n); err != nil {
			return nil, err
		}
	}

	m.nodes.closeOpen(&out)

	return out.Bytes(), nil
}

// footnotes renders the footnote list, empty when the document has none.
func (m *markdown) footnotes() ([]byte, error) {
	var out bytes.Buffer

	for n := m.root.FirstChild(); n != nil; n = n.NextSibling() {
		if _, ok := n.(*east.FootnoteList); !ok {
			continue
		}

		if err := m.md.Renderer().Render(&out, m.src, n); err != nil {
			return nil, err
		}
	}

	return out.Bytes(), nil
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
