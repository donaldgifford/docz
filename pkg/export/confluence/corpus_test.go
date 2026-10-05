package confluence

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/doctemplate"
)

// TestRender_Corpus renders every snapshotted fleet document the type
// packages carry and every embedded template, and requires each to come back
// well-formed with no in-page anchor left unresolved (DESIGN-0020 Testing).
// It reads the snapshots, never docs/, so a document edit cannot move it.
func TestRender_Corpus(t *testing.T) {
	t.Parallel()

	fixtures, err := filepath.Glob("../../*/testdata/*.orig.md")
	if err != nil || len(fixtures) < 38 {
		t.Fatalf("found %d corpus fixtures, want at least 38: %v", len(fixtures), err)
	}

	for _, path := range fixtures {
		t.Run(strings.TrimPrefix(path, "../../"), func(t *testing.T) {
			t.Parallel()

			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}

			checkCorpusPage(t, src)
		})
	}
}

func TestRender_Templates(t *testing.T) {
	t.Parallel()

	types := append(config.DocTypeNames(), "default")

	for _, name := range types {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var (
				tmpl string
				err  error
			)

			if name == "default" {
				tmpl, err = doctemplate.GenericTemplate()
			} else {
				tmpl, err = doctemplate.Resolve(name, "", t.TempDir())
			}

			if err != nil {
				t.Fatal(err)
			}

			doc, err := doctemplate.Render(tmpl, &doctemplate.Data{
				Number: "0001", Title: "A title", Date: "2026-01-01", Author: "A",
				Status: "Draft", Type: config.DocType(name), Prefix: strings.ToUpper(name),
			})
			if err != nil {
				t.Fatal(err)
			}

			checkCorpusPage(t, []byte(doc))
		})
	}
}

// checkCorpusPage renders src and fails on an error or an in-page anchor
// that names no heading.
func checkCorpusPage(t *testing.T, src []byte) {
	t.Helper()

	got, err := Render(src, RenderOptions{Source: "docs/x/0001-x.md"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	for _, l := range got.Links {
		if strings.HasPrefix(l.Href, "#") {
			t.Errorf("unresolved in-page anchor %q on line %d", l.Href, l.Line)
		}
	}
}
