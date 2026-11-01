package html

import (
	"crypto/sha512"
	"encoding/base64"
	stdhtml "html"
	"os"
	"path/filepath"
	"regexp"
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
			want:    []string{`<pre class="mermaid">graph LR`, "A --&gt; B&lt;script&gt;", "__esbuild_esm_mermaid_nm", "securityLevel"},
			notWant: []string{"cdn.jsdelivr.net"},
		},
		{
			name:      "cdn",
			src:       diagram,
			mode:      MermaidCDN,
			want:      []string{`<pre class="mermaid">`, "securityLevel"},
			notWant:   []string{"__esbuild_esm_mermaid_nm"},
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

// TestVendoredMermaid holds the embedded mermaid to the version ui/bun.lock
// pins and to the CDN mode's integrity hash, so the two modes draw with the
// same bytes and a lockfile bump without just vendor-mermaid fails here.
func TestVendoredMermaid(t *testing.T) {
	t.Parallel()

	lock, err := os.ReadFile(filepath.Join("..", "..", "..", "ui", "bun.lock"))
	if err != nil {
		t.Fatal(err)
	}

	m := regexp.MustCompile(`"mermaid": \["mermaid@([^"]+)"`).FindSubmatch(lock)
	if m == nil {
		t.Fatal("ui/bun.lock pins no mermaid")
	}

	if string(m[1]) != mermaidVersion {
		t.Errorf("mermaidVersion = %s, ui/bun.lock pins %s; run just vendor-mermaid and update the version and hash", mermaidVersion, m[1])
	}

	script := mermaidScript()
	if !strings.Contains(script, `"`+mermaidVersion+`"`) {
		t.Errorf("the embedded mermaid does not carry version %s", mermaidVersion)
	}

	sum := sha512.Sum384([]byte(script))
	if got := "sha384-" + base64.StdEncoding.EncodeToString(sum[:]); got != mermaidIntegrity {
		t.Errorf("the embedded mermaid hashes to %s, mermaidIntegrity is %s", got, mermaidIntegrity)
	}
}
