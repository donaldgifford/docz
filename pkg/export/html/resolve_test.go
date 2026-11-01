package html

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRelativeTo(t *testing.T) {
	t.Parallel()

	tests := []struct{ from, to, want string }{
		{"rfc/a.html", "rfc/b.html", "b.html"},
		{"rfc/a.html", "adr/b.html", "../adr/b.html"},
		{"index.html", "rfc/b.html", "rfc/b.html"},
		{"rfc/x/a.html", "b.html", "../../b.html"},
	}

	for _, tt := range tests {
		if got := relativeTo(tt.from, tt.to); got != tt.want {
			t.Errorf("relativeTo(%q, %q) = %q, want %q", tt.from, tt.to, got, tt.want)
		}
	}
}

func TestExportLinksAndImages(t *testing.T) {
	t.Parallel()

	rp := fixtureRepo(t, "rfc")
	entries, err := rp.List(context.Background(), nil)
	if err != nil || len(entries) != 2 {
		t.Fatalf("List: %d, %v", len(entries), err)
	}

	png := []byte("\x89PNG\r\n\x1a\nsmall")
	if err := os.MkdirAll(rp.Path("docs/img"), 0o750); err != nil {
		t.Fatal(err)
	}

	for name, body := range map[string][]byte{
		"docs/img/small.png": png,
		"docs/img/big.png":   make([]byte, maxInlineImage+1),
	} {
		if err := os.WriteFile(rp.Path(name), body, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.WriteFile(filepath.Join(filepath.Dir(rp.Root), "outside.png"), png, 0o600); err != nil {
		t.Fatal(err)
	}

	second := filepath.Base(entries[1].Path)
	extra := "\n## Links\n\n[next](" + second + "#motivation) [readme](../../README.md) [gone](nothing.md)\n\n" +
		"![small](../img/small.png) ![big](../img/big.png) ![out](../../../outside.png)\n"

	if err := os.WriteFile(rp.Path(entries[0].Path), append(bytes.Clone(entries[0].Content), extra...), 0o600); err != nil {
		t.Fatal(err)
	}

	resolve := func(_, href string) string {
		if strings.HasSuffix(href, "README.md") || strings.HasSuffix(href, "big.png") {
			return "https://example.test/" + strings.TrimLeft(href, "./")
		}

		return ""
	}

	rep, err := Export(context.Background(), rp, ExportOptions{DryRun: true, Resolve: resolve})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	body := string(rep.Pages[0].Body)

	for _, want := range []string{
		`<a href="` + strings.TrimSuffix(second, ".md") + `.html#motivation">next</a>`,
		`<a href="https://example.test/README.md">readme</a>`,
		`<img src="data:image/png;base64,iVBORw0KGgpzbWFsbA==" alt="small">`,
		`<img src="https://example.test/img/big.png" alt="big">`,
		`<em>out</em>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the page lacks %s", want)
		}
	}

	unresolved := make([]string, 0, len(rep.Pages[0].Links))
	for _, l := range rep.Pages[0].Links {
		unresolved = append(unresolved, l.Href)
	}

	if got := strings.Join(unresolved, " "); !strings.Contains(got, "nothing.md") || !strings.Contains(got, "outside.png") {
		t.Errorf("unresolved links = %q, want nothing.md and the outside image", got)
	}
}
