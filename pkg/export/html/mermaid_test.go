package html

import (
	stdhtml "html"
	"strings"
	"testing"
)

func TestMermaidModes(t *testing.T) {
	t.Parallel()

	const diagram = "## Flow\n\n```mermaid\ngraph LR\n  A --> B<script>\n```\n"

	tests := []struct {
		name      string
		src       string
		mode      MermaidMode
		want      []string
		notWant   []string
		cdnPolicy bool
	}{
		{
			name:    "embed",
			src:     diagram,
			mode:    MermaidEmbed,
			want:    []string{`<pre class="mermaid">graph LR`, "A --&gt; B&lt;script&gt;", "vendored in IMPL-0028 Phase 5", "securityLevel"},
			notWant: []string{"cdn.jsdelivr.net"},
		},
		{
			name:      "cdn",
			src:       diagram,
			mode:      MermaidCDN,
			want:      []string{`<pre class="mermaid">`, "securityLevel"},
			notWant:   []string{"vendored in IMPL-0028 Phase 5"},
			cdnPolicy: true,
		},
		{
			name:    "source",
			src:     diagram,
			mode:    MermaidSource,
			want:    []string{`Diagram (mermaid source)`, `<code class="language-mermaid">graph LR`},
			notWant: []string{`<pre class="mermaid">`, "securityLevel", "cdn.jsdelivr.net"},
		},
		{
			name:    "no diagram",
			src:     "## Flow\n\nProse.\n",
			mode:    MermaidCDN,
			notWant: []string{"securityLevel", "cdn.jsdelivr.net"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body := string(page(t, tt.src, &RenderOptions{Mermaid: tt.mode}).Body)
			// Attribute values are entity-escaped; read them decoded.
			attrs := stdhtml.UnescapeString(body)

			for _, w := range tt.want {
				if !strings.Contains(body, w) {
					t.Errorf("page lacks %q", w)
				}
			}

			for _, w := range tt.notWant {
				if strings.Contains(body, w) {
					t.Errorf("page has %q", w)
				}
			}

			script := `<script src="` + mermaidCDN + `" integrity="` + mermaidIntegrity + `" crossorigin="anonymous">`
			if got := strings.Contains(attrs, script); got != tt.cdnPolicy {
				t.Errorf("page loads mermaid from the CDN = %v, want %v", got, tt.cdnPolicy)
			}

			cdn := strings.Contains(attrs, "script-src 'unsafe-inline' "+mermaidCDNOrigin)
			if cdn != tt.cdnPolicy {
				t.Errorf("CSP allows the CDN = %v, want %v", cdn, tt.cdnPolicy)
			}

			if !strings.Contains(attrs, "default-src 'none'") {
				t.Errorf("page has no CSP")
			}
		})
	}
}
