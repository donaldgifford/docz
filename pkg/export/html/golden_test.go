package html

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"golang.org/x/net/html"

	"github.com/donaldgifford/docz/v2/pkg/adr"
	"github.com/donaldgifford/docz/v2/pkg/design"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
	"github.com/donaldgifford/docz/v2/pkg/impl"
	"github.com/donaldgifford/docz/v2/pkg/investigation"
	"github.com/donaldgifford/docz/v2/pkg/rfc"
	"github.com/donaldgifford/docz/v2/pkg/runbook"
)

var update = flag.Bool("update", false, "regenerate the golden pages")

// specs is each built-in's heading spec, by type name.
var specs = map[string]func() kinds.HeadingSpec{
	"rfc":           rfc.Headings,
	"adr":           adr.Headings,
	"design":        design.Headings,
	"impl":          impl.Headings,
	"investigation": investigation.Headings,
	"runbook":       runbook.Headings,
}

// themeBlock is each inlined copy of the theme, which a golden replaces by
// its markers (IMPL-0028 question 6) so a theme change moves one golden.
var themeBlock = regexp.MustCompile(`(?s)` + regexp.QuoteMeta(string(themeOpen)) + `.*?` + regexp.QuoteMeta(string(themeClose)))

// corpus is every document the goldens cover: each type package's corpus
// and each parity fixture's documents, with the type each renders as.
func corpus(t *testing.T) map[string]string {
	t.Helper()

	out := make(map[string]string)

	for name := range specs {
		matches, err := filepath.Glob(filepath.Join("..", "..", name, "testdata", "*.md"))
		if err != nil {
			t.Fatal(err)
		}

		for _, m := range matches {
			if filepath.Base(m) != "README.md" {
				out[m] = name
			}
		}
	}

	matches, err := filepath.Glob(filepath.Join("..", "..", "..", "test", "parity", "fixtures", "*", "docs", "*", "*.md"))
	if err != nil {
		t.Fatal(err)
	}

	for _, m := range matches {
		typ := filepath.Base(filepath.Dir(m))
		if filepath.Base(m) != "README.md" && typ != "templates" {
			out[m] = typ
		}
	}

	return out
}

// goldenName is a corpus path's golden file: the type package or fixture
// and the file's base name.
func goldenName(path, typ string) string {
	base := strings.TrimSuffix(filepath.Base(path), ".md")

	if strings.Contains(path, filepath.Join("test", "parity")) {
		fixture := filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(path))))

		return filepath.Join("testdata", "golden", "parity", fixture+"-"+typ+"-"+base+".html")
	}

	return filepath.Join("testdata", "golden", typ, base+".html")
}

func renderFile(t *testing.T, path, typ string) []byte {
	t.Helper()

	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var spec kinds.HeadingSpec
	if f, ok := specs[typ]; ok {
		spec = f()
	}

	p, err := Render(src, RenderOptions{
		Spec: spec, Type: typ, Source: filepath.ToSlash(path), Site: "docz", Version: "test",
	})
	if err != nil {
		return []byte("error: " + err.Error() + "\n")
	}

	return p.Body
}

func compare(t *testing.T, golden string, got []byte) {
	t.Helper()

	if *update {
		if err := os.MkdirAll(filepath.Dir(golden), 0o750); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(golden, got, 0o600); err != nil {
			t.Fatal(err)
		}

		return
	}

	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("%v (run go test -update)", err)
	}

	if !bytes.Equal(got, want) {
		t.Errorf("%s differs from its golden; run go test -update and review the diff", golden)
	}
}

func TestGoldenPages(t *testing.T) {
	t.Parallel()

	for path, typ := range corpus(t) {
		golden := goldenName(path, typ)

		t.Run(golden, func(t *testing.T) {
			t.Parallel()

			page := renderFile(t, path, typ)
			if _, err := html.ParseWithOptions(bytes.NewReader(page), html.ParseOptionEnableScripting(true)); err != nil {
				t.Errorf("%s does not parse: %v", path, err)
			}

			if bytes.HasPrefix(page, []byte("error:")) {
				t.Errorf("%s did not render: %s", path, page)
			}

			got := themeBlock.ReplaceAll(page, []byte(string(themeOpen)+string(themeClose)))
			compare(t, golden, got)
		})
	}
}

// TestGoldenFullPage keeps one page whole, theme and all, so the inlining
// itself is pinned.
func TestGoldenFullPage(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", "design", "testdata", "docz-design-0009.md")
	compare(t, filepath.Join("testdata", "golden", "full-page.html"), renderFile(t, path, "design"))
}
