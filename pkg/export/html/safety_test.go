package html

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/theme"
)

const evil = `<script>alert(1)</script>`

func TestRenderEscapesEveryField(t *testing.T) {
	t.Parallel()

	src := "---\n" +
		"id: \"DESIGN-0001<script>alert(1)</script>\"\n" +
		"title: \"T <script>alert(1)</script>\"\n" +
		"status: \"Draft<script>alert(1)</script>\"\n" +
		"author: \"A <script>alert(1)</script>\"\n" +
		"created: \"2026-10-31<script>alert(1)</script>\"\n" +
		"---\n" +
		"# Title " + evil + "\n\n" +
		"Lead " + evil + " paragraph.\n\n" +
		"## Heading " + evil + "\n\n" +
		"Prose " + evil + " and `" + evil + "`.\n\n" +
		"<div onclick=\"x()\">" + evil + "</div>\n\n" +
		"| a |\n| --- |\n| " + evil + " |\n\n" +
		"- [ ] task " + evil + "\n\n" +
		"```go\n" + evil + "\n```\n\n" +
		"```mermaid\n" + evil + "\n```\n"

	opts := RenderOptions{
		Type: "design" + evil, Site: "site" + evil, Source: "docs/" + evil + ".md", Version: "v" + evil,
	}

	for _, mode := range []MermaidMode{MermaidEmbed, MermaidSource, MermaidCDN} {
		opts.Mermaid = mode

		p, err := Render([]byte(src), opts)
		if err != nil {
			t.Fatalf("Render: %v", err)
		}

		body := string(p.Body)
		if n := strings.Count(body, "<script>alert"); n != 0 {
			t.Errorf("mode %d: %d unescaped scripts reach the page", mode, n)
		}

		if regexp.MustCompile(`<[a-z][^>]*onclick`).MatchString(body) {
			t.Errorf("mode %d: an attribute on raw HTML reaches the page", mode)
		}
	}
}

func TestRenderRawHTMLAllowList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		src, want, notWant string
	}{
		{"Press <kbd>Ctrl</kbd>.", "<kbd>Ctrl</kbd>", ""},
		{"H<sub>2</sub>O and x<sup>2</sup>", "<sub>2</sub>", ""},
		{"a<br>b", "<br>", ""},
		{`<span style="color:red" class="x">s</span>`, "<span>s</span>", "style="},
		{"<details>\n<summary>More</summary>\n\nBody.\n\n</details>", "<summary>More</summary>", ""},
		{`<iframe src="https://example.com"></iframe>`, "&lt;iframe", "<iframe"},
		{`<img src=x onerror=alert(1)>`, "&lt;img", "<img"},
		{`<a href="javascript:alert(1)">x</a>`, "", `href="javascript`},
		{"<style>body{}</style>", "", "<style>body"},
		{"Open <kbd>tag", "<kbd>tag</kbd></p>", ""},
		{"- an <kbd>open\n- next", "<kbd>open</kbd></li>", ""},
	}

	for _, tt := range tests {
		body, _ := renderBody(t, "## S\n\n"+tt.src+"\n", &RenderOptions{})

		if tt.want != "" && !strings.Contains(body, tt.want) {
			t.Errorf("%q: body lacks %q:\n%s", tt.src, tt.want, body)
		}

		if tt.notWant != "" && strings.Contains(body, tt.notWant) {
			t.Errorf("%q: body has %q:\n%s", tt.src, tt.notWant, body)
		}
	}
}

func TestRenderLinkSchemes(t *testing.T) {
	t.Parallel()

	body, _ := renderBody(t, "## S\n\n[a](javascript:alert(1)) [b](data:text/html,x) [c](vbscript:x) "+
		"[d](https://example.com) [e](mailto:a@b.c) ![f](http://example.com/i.png)\n", &RenderOptions{})

	for _, bad := range []string{"javascript:", "data:text/html", "vbscript:", `src="http://`} {
		if strings.Contains(body, bad) {
			t.Errorf("body carries %q:\n%s", bad, body)
		}
	}

	for _, good := range []string{`href="https://example.com"`, `href="mailto:a@b.c"`} {
		if !strings.Contains(body, good) {
			t.Errorf("body lacks %q:\n%s", good, body)
		}
	}
}

// loads is every URL a page asks the browser to fetch.
var loads = regexp.MustCompile(`(?i)\b(?:src|href)\s*=\s*"?(https?:[^"\s>]*)`)

func TestRenderNoNetworkOutsideCDN(t *testing.T) {
	t.Parallel()

	const src = "## S\n\n```mermaid\ngraph LR\n```\n\n[a link](https://example.com) is not a load.\n"

	for _, mode := range []MermaidMode{MermaidEmbed, MermaidSource} {
		body := string(page(t, src, &RenderOptions{Mermaid: mode}).Body)

		for _, m := range loads.FindAllStringSubmatch(body, -1) {
			if m[1] != "https://example.com" {
				t.Errorf("mode %d loads %s", mode, m[1])
			}
		}

		if !strings.Contains(body, `http-equiv="Content-Security-Policy"`) {
			t.Errorf("mode %d: no CSP", mode)
		}

		if strings.Contains(body, "connect-src") || strings.Contains(body, "cdn.jsdelivr") {
			t.Errorf("mode %d: the CSP allows a fetch", mode)
		}
	}
}

func TestThemeCannotCloseItsElement(t *testing.T) {
	t.Parallel()

	for _, name := range theme.Names() {
		css, err := theme.CSS(name)
		if err != nil {
			t.Fatal(err)
		}

		if strings.Contains(strings.ToLower(css), "</style") {
			t.Errorf("theme %s closes its <style>", name)
		}
	}

	for _, js := range []string{theme.Runtime(), mermaidInit, mermaidScript()} {
		if strings.Contains(strings.ToLower(js), "</script") {
			t.Errorf("a script closes its <script>")
		}
	}
}

func TestRenderRejects(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		src  string
		want error
	}{
		{"crlf", "---\r\nid: X-0001\r\n---\r\nBody\r\n", ErrLineEndings},
		{"cr", "---\nid: X-0001\n---\nBody\rmore\n", ErrLineEndings},
		{"no frontmatter", "# Just markdown\n", ErrNoFrontmatter},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, err := Render([]byte(tt.src), RenderOptions{}); !errors.Is(err, tt.want) {
				t.Errorf("Render(%q) = %v, want %v", tt.src, err, tt.want)
			}
		})
	}
}
