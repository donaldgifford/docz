package html

import (
	"strings"
	"testing"
)

// renderBody renders src's panels and joins their bodies.
func renderBody(t *testing.T, src string, opts *RenderOptions) (string, *doc) {
	t.Helper()

	body := blankFrontmatter([]byte("---\nid: X-0001\ntitle: T\n---\n" + src))
	d := prepare(body, opts)

	panels, err := d.panels()
	if err != nil {
		t.Fatalf("panels: %v", err)
	}

	var b strings.Builder
	for _, p := range panels {
		b.Write(p.Body)
	}

	return b.String(), d
}

func TestMarkdownNodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		src  string
		want []string
		not  []string
	}{
		{
			name: "headings drop a level and keep docparse's anchors",
			src:  "## Intro\n\n### Same\n\n### Same\n",
			want: []string{`<h4 id="same">Same</h4>`, `<h4 id="same-1">Same</h4>`},
			not:  []string{"<h2", "<h3"},
		},
		{
			name: "an alert is a callout",
			src:  "## A\n\n> [!WARNING]\n> Careful.\n",
			want: []string{`<div class="callout warning"><p class="callout-title">Warning</p>`, "Careful."},
			not:  []string{"[!WARNING]", "<blockquote"},
		},
		{
			name: "task lists are read-only",
			src:  "## A\n\n- [x] done\n- [ ] open\n",
			want: []string{
				`<ul class="tasks">`,
				`<li data-state="done"><input type="checkbox" checked disabled><div class="text">done`,
				`<li data-state="open"><input type="checkbox" disabled><div class="text">open`,
			},
		},
		{
			name: "tables scroll",
			src:  "## A\n\n| a | b |\n| --- | --- |\n| 1 | 2 |\n",
			want: []string{"<div class=\"table-wrap\">\n<table>"},
		},
		{
			name: "allowed raw HTML loses its attributes",
			src:  "## A\n\nPress <kbd onclick=\"x()\">K</kbd> and <span style=\"a\">s</span>.\n",
			want: []string{"<kbd>K</kbd>", "<span>s</span>"},
			not:  []string{"onclick", "style="},
		},
		{
			name: "other raw HTML is text",
			src:  "## A\n\n<script>alert(1)</script>\n\nA <img src=x onerror=y> here.\n",
			want: []string{"&lt;script&gt;alert(1)&lt;/script&gt;", "&lt;img src=x onerror=y&gt;"},
			not:  []string{"<script", "<img"},
		},
		{
			name: "an unclosed details is closed at the end of its panel",
			src:  "## A\n\n<details>\n\nopen\n\n## B\n\nafter\n",
			want: []string{"<details>", "</details>"},
		},
		{
			name: "comments and region markers are dropped",
			src:  "<!--docz:summary:start-->\n## A\n\n<!-- a note -->\n\nx\n<!--docz:summary:end-->\n",
			not:  []string{"<!--", "docz:summary", "a note"},
		},
		{
			name: "unsafe link schemes are text",
			src:  "## A\n\n[x](javascript:alert(1)) [y](https://example.com) <mailto:a@b.c>\n",
			want: []string{`<a href="https://example.com">y</a>`, `<a href="mailto:a@b.c">`},
			not:  []string{"javascript:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, _ := renderBody(t, tt.src, &RenderOptions{})
			for _, w := range tt.want {
				if !strings.Contains(got, w) {
					t.Errorf("body lacks %q:\n%s", w, got)
				}
			}

			for _, n := range tt.not {
				if strings.Contains(got, n) {
					t.Errorf("body has %q:\n%s", n, got)
				}
			}
		})
	}
}

func TestMarkdownLinks(t *testing.T) {
	t.Parallel()

	resolve := func(_, href string) string {
		if href == "../adr/0001-x.md" {
			return "ADR-0001.html"
		}

		return ""
	}

	got, d := renderBody(t, "## A\n\n[ok](../adr/0001-x.md) [gone](missing.md) [in](#a) [nowhere](#zzz)\n",
		&RenderOptions{Resolve: resolve})

	for _, w := range []string{`<a href="ADR-0001.html">ok</a>`, `<a href="#a">in</a>`, "gone", "nowhere"} {
		if !strings.Contains(got, w) {
			t.Errorf("body lacks %q:\n%s", w, got)
		}
	}

	links := d.md.nodes.links
	if len(links) != 2 || links[0].Href != "missing.md" || links[1].Href != "#zzz" || links[0].Line != 7 {
		t.Errorf("unresolved = %+v, want missing.md and #zzz on line 7", links)
	}
}
