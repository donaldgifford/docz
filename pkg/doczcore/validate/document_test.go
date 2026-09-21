package validate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	doctemplate "github.com/donaldgifford/docz/v2/internal/template"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/validate"
)

// optionsFor builds the Options a caller assembles for a built-in type: the
// schema from the baked-in skeleton, the type's own config, and the heading
// spec from its template.
func optionsFor(t *testing.T, name, filename string) validate.Options {
	t.Helper()

	marked, err := doctemplate.EmbeddedDocumentTemplate(config.DocType(name))
	if err != nil {
		t.Fatalf("EmbeddedDocumentTemplate(%q): %v", name, err)
	}

	skeleton, err := doctemplate.EmbeddedSchema(name)
	if err != nil {
		t.Fatalf("EmbeddedSchema(%q): %v", name, err)
	}

	return validate.Options{
		Schema:      validate.SchemaFromMarkers([]byte(skeleton)),
		Type:        config.DefaultConfig().Types[name],
		Filename:    filename,
		Headings:    kinds.SpecFromTemplate([]byte(marked)),
		MinHeadings: 3,
	}
}

// renderTemplate renders a type's embedded template the way `docz create`
// does, so the bytes under test are the bytes a user gets.
func renderTemplate(t *testing.T, name string) []byte {
	t.Helper()

	body, err := doctemplate.EmbeddedDocumentTemplate(config.DocType(name))
	if err != nil {
		t.Fatal(err)
	}

	tmpl, err := template.New(name).Parse(body)
	if err != nil {
		t.Fatalf("parsing the %s template: %v", name, err)
	}

	typ := config.DefaultConfig().Types[name]

	var sb strings.Builder

	err = tmpl.Execute(&sb, map[string]any{
		"Prefix": typ.IDPrefix,
		"Number": "0001",
		"Title":  "A placeholder title",
		"Status": typ.Statuses[0],
		"Author": "Test Author",
		"Date":   "2026-09-20",
	})
	if err != nil {
		t.Fatalf("rendering the %s template: %v", name, err)
	}

	return []byte(sb.String())
}

// TestDocument_TemplatesValidateClean is the golden this package is built
// against: what `docz create` writes must pass `docz validate` with nothing
// to say about it.
//
// A validator that failed the document it had just written would be worse
// than none — the first thing every user did would be to turn it off. It has
// already caught two design mistakes: content rules that reported an
// unfilled section as malformed, and a singleton check scoped to the parent
// kind rather than the parent region, which called the second and third
// phases' task lists duplicates of the first's.
func TestDocument_TemplatesValidateClean(t *testing.T) {
	t.Parallel()

	for _, name := range config.DocTypeNames() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := validate.Document(renderTemplate(t, name),
				optionsFor(t, name, "0001-a-placeholder-title.md"))

			for _, f := range got {
				t.Errorf("a freshly created %s reports %s", name, f)
			}
		})
	}
}

// The same documents with their markers still in place must also validate
// clean when inference is off. A repo that has migrated is not relying on
// the heuristic, and the two paths must agree.
func TestDocument_TemplatesValidateCleanWithoutInference(t *testing.T) {
	t.Parallel()

	for _, name := range config.DocTypeNames() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			opts := optionsFor(t, name, "0001-a-placeholder-title.md")
			opts.Headings = nil

			for _, f := range validate.Document(renderTemplate(t, name), opts) {
				t.Errorf("a marked %s reports %s with inference off", name, f)
			}
		})
	}
}

// TestDocument_OverTheCorpus runs the validator over this repo's own
// documents. None carries markers, so every one exercises inference, and
// what it reports has to be proportionate: a validator whose output a
// maintainer learns to ignore is not a validator.
func TestDocument_OverTheCorpus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		typ  string
		glob string
	}{
		{"adr", "adr/0002-*.md"},
		{"design", "design/0015-*.md"},
		{"impl", "impl/0017-*.md"},
		{"investigation", "investigation/0010-*.md"},
	}

	for _, tt := range tests {
		t.Run(tt.typ, func(t *testing.T) {
			t.Parallel()

			paths, err := filepath.Glob(filepath.Join("..", "..", "..", "docs", tt.glob))
			if err != nil || len(paths) == 0 {
				t.Skipf("no %s document matching %s in this checkout", tt.typ, tt.glob)
			}

			body, err := os.ReadFile(paths[0])
			if err != nil {
				t.Fatal(err)
			}

			got := validate.Document(body, optionsFor(t, tt.typ, filepath.Base(paths[0])))

			codes := make(map[string]int, len(got))
			for _, f := range got {
				codes[f.Code]++

				if f.Severity == validate.Error {
					t.Errorf("%s reports an error: %s", filepath.Base(paths[0]), f)
				}
			}

			// Inference must have run, and said so exactly once.
			if codes["region.inferred"] != 1 {
				t.Errorf("region.inferred appeared %d times, want 1: %v",
					codes["region.inferred"], codes)
			}

			// No section of a real document may be unrecognisable. A
			// region.missing here means inference cannot read the corpus,
			// which is the whole claim DESIGN-0015 §6 rests on.
			if codes["region.missing"] > 0 {
				for _, f := range got {
					if f.Code == "region.missing" {
						t.Errorf("inference could not find a section: %s", f)
					}
				}
			}

			// A document showing markers in fenced blocks gets one finding,
			// not one per line. DESIGN-0015 has twenty-eight such lines.
			if codes["marker.in-fence"] > 1 {
				t.Errorf("marker.in-fence appeared %d times, want at most 1",
					codes["marker.in-fence"])
			}
		})
	}
}
