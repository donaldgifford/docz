package html

import (
	"bytes"
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
				switch n.Data {
				case "script":
					scripts++
				case "style":
					styles++
				case "iframe", "object", "embed", "form", "base":
					t.Fatalf("the page carries a <%s>", n.Data)
				}

				for _, a := range n.Attr {
					if strings.HasPrefix(a.Key, "on") {
						t.Fatalf("<%s> carries %s", n.Data, a.Key)
					}

					if (a.Key == "href" || a.Key == "src") && strings.HasPrefix(strings.ToLower(strings.TrimSpace(a.Val)), "javascript:") {
						t.Fatalf("<%s %s=%q>", n.Data, a.Key, a.Val)
					}
				}
			}

			for c := n.FirstChild; c != nil; c = c.NextSibling {
				visit(c)
			}
		}

		visit(doc)

		// The runtime is the one script and the theme the one style a page
		// without mermaid has; anything more came from the document.
		if scripts != 1 || styles != 1 {
			t.Fatalf("the page has %d scripts and %d styles, want 1 and 1", scripts, styles)
		}
	})
}
