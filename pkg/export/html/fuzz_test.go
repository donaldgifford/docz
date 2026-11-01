package html

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func FuzzRender(f *testing.F) {
	for _, seed := range []string{
		"---\nid: X-0001\ntitle: T\n---\n## A\n\nText.\n",
		"---\nid: X-0001\n---\n<!--docz:summary:start-->\n## S\n<kbd>open\n<!--docz:summary:end-->\n",
		"---\ntitle: \"<script>x</script>\"\n---\n# <script>y</script>\n\n<details><summary>s\n\n- [ ] t <sub>\n",
		"---\nid: X\n---\n```mermaid\n</pre><script>z</script>\n```\n[a](javascript:x) ![i](data:image/png;base64,AA==)\n",
		"---\nid: X\n---\n```docz-flow\nstages:\n  - {name: \"</script><script>x\", does: d}\n  - {name: b, does: d}\n```\n",
		"---\nid: X\n---\n| a | b |\n| --- | --- |\n| <span onclick=x> | `c` |\n\n> [!WARNING]\n> w[^1]\n\n[^1]: n\n",
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, src []byte) {
		p, err := Render(src, RenderOptions{Mermaid: MermaidSource, Site: "s", Type: "design"})
		if err != nil {
			// Bad frontmatter or line endings is an error, never a panic.
			return
		}

		doc, err := html.Parse(bytes.NewReader(p.Body))
		if err != nil {
			t.Fatalf("the page does not parse: %v", err)
		}

		var scripts, styles int

		var visit func(*html.Node)

		visit = func(n *html.Node) {
			if n.Type == html.ElementNode {
				if checkElement(t, n) {
					scripts++
				}

				if n.Data == "style" {
					styles++
				}
			}

			for c := n.FirstChild; c != nil; c = c.NextSibling {
				visit(c)
			}
		}

		visit(doc)

		// The runtime is the one script, beside any widget islands, and the theme the one style a page
		// without mermaid has; anything more came from the document.
		if scripts != 1 || styles != 1 {
			t.Fatalf("the page has %d scripts and %d styles, want 1 and 1", scripts, styles)
		}
	})
}

// isIsland reports a widget's JSON island: a script the browser never runs.
func isIsland(n *html.Node) bool {
	var typ, widget bool

	for _, a := range n.Attr {
		typ = typ || a.Key == "type" && a.Val == "application/json"
		widget = widget || a.Key == "data-widget"
	}

	return typ && widget
}

// checkElement fails on an element a page must never carry and reports
// whether n is a script the browser runs: a widget island must be JSON and
// is not one.
func checkElement(t *testing.T, n *html.Node) bool {
	t.Helper()

	switch n.Data {
	case "iframe", "object", "embed", "form", "base":
		t.Fatalf("the page carries a <%s>", n.Data)
	}

	for _, a := range n.Attr {
		if strings.HasPrefix(a.Key, "on") {
			t.Fatalf("<%s> carries %s", n.Data, a.Key)
		}

		if (a.Key == "href" || a.Key == "src") &&
			strings.HasPrefix(strings.ToLower(strings.TrimSpace(a.Val)), "javascript:") {
			t.Fatalf("<%s %s=%q>", n.Data, a.Key, a.Val)
		}
	}

	if n.Data != "script" {
		return false
	}

	if isIsland(n) {
		if n.FirstChild == nil || !json.Valid([]byte(n.FirstChild.Data)) {
			t.Fatalf("a widget island is not JSON")
		}

		return false
	}

	return true
}
