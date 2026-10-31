package html

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

// page renders src with frontmatter prepended.
func page(t *testing.T, src string, opts *RenderOptions) Page {
	t.Helper()

	p, err := Render([]byte("---\nid: DESIGN-0001\ntitle: T\nstatus: Draft\n---\n"+src), *opts)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	return p
}

func TestRenderEmptyRegion(t *testing.T) {
	t.Parallel()

	p := page(t, "<!--docz:background:start-->\n## Background\n\n<!-- Why now? -->\n\n<!--docz:background:end-->\n\n"+
		"<!--docz:testing:start-->\n## Testing\n\nUnit tests.\n<!--docz:testing:end-->\n", &RenderOptions{})
	body := string(p.Body)

	bg := body[strings.Index(body, `id="p-background"`):strings.Index(body, `id="p-testing"`)]
	if !strings.Contains(bg, `<p class="empty">Not written yet.</p>`) {
		t.Errorf("an empty region is not marked as not written yet:\n%s", bg)
	}

	if strings.Count(body, "Not written yet.") != 1 {
		t.Errorf("a written region is marked as not written yet")
	}
}

func TestRenderDeterministic(t *testing.T) {
	t.Parallel()

	const src = "## Summary\n\nA page.\n\n```mermaid\ngraph LR\n  A --> B\n```\n"

	first := page(t, src, &RenderOptions{Version: "v1"})
	second := page(t, src, &RenderOptions{Version: "v1"})

	if !bytes.Equal(first.Body, second.Body) || first.Hash != second.Hash {
		t.Fatalf("two renders of one document differ")
	}

	sum := sha256.Sum256(first.Body)
	if want := "sha256:" + hex.EncodeToString(sum[:]); first.Hash != want {
		t.Errorf("Hash = %q, want %q", first.Hash, want)
	}

	if other := page(t, src, &RenderOptions{Version: "v2"}); other.Hash == first.Hash {
		t.Errorf("a different body hashes the same")
	}
}
