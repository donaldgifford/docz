package doctemplate

import (
	"sort"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
)

// pairs walks a marked document and returns one "kind under parent" string per
// region, deduplicated and sorted. A schema requires each kind at least once
// under the same parent, so this is the comparable form of both a skeleton and
// the template it describes.
func pairs(t *testing.T, body string) []string {
	t.Helper()

	regions := docparse.Regions([]byte(body))
	seen := make(map[string]bool, len(regions))

	for i, r := range regions {
		if !r.Closed {
			t.Errorf("unclosed %q region at line %d", r.Kind, r.Start)
		}

		parent := "-"

		// The innermost earlier region that still contains this one. Regions
		// come back sorted by start line, so scanning backwards finds it.
		for j := i - 1; j >= 0; j-- {
			if regions[j].Start < r.Start && regions[j].End >= r.End {
				parent = regions[j].Kind

				break
			}
		}

		seen[r.Kind+" under "+parent] = true
	}

	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}

	sort.Strings(out)

	return out
}

// TestSchemasAgreeWithTemplates is the check DESIGN-0015 §3 asks for once the
// schema is its own artifact rather than something derived from the template:
// the two must say the same thing, or `docz create` writes a document that
// fails the validator it shipped with.
//
// Both directions hold for a built-in. A skeleton lists every kind its
// template carries, and the template carries every kind its skeleton requires.
func TestSchemasAgreeWithTemplates(t *testing.T) {
	t.Parallel()

	for _, name := range config.DocTypeNames() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			body, err := EmbeddedDocumentTemplate(config.DocType(name))
			if err != nil {
				t.Fatalf("EmbeddedDocumentTemplate(%q): %v", name, err)
			}

			skeleton, err := EmbeddedSchema(name)
			if err != nil {
				t.Fatalf("EmbeddedSchema(%q): %v", name, err)
			}

			want := strings.Join(pairs(t, skeleton), "\n")
			if got := strings.Join(pairs(t, body), "\n"); got != want {
				t.Errorf("template and schema disagree\n--- template ---\n%s\n--- schema ---\n%s",
					got, want)
			}
		})
	}
}

// The generic pair scaffolds a custom type. Its schema requires only the ToC
// and references, so unlike a built-in its template may carry more — a kind a
// schema does not list is optional (DESIGN-0015 §3). What must hold is that
// every kind the schema requires is in the template.
func TestDefaultSchemaIsSatisfiedByDefaultTemplate(t *testing.T) {
	t.Parallel()

	body, err := templateFS.ReadFile("templates/" + DefaultTemplateName + ".md")
	if err != nil {
		t.Fatalf("reading the generic template: %v", err)
	}

	skeleton, err := EmbeddedSchema(DefaultTemplateName)
	if err != nil {
		t.Fatalf("EmbeddedSchema(%q): %v", DefaultTemplateName, err)
	}

	have := make(map[string]bool)
	for _, p := range pairs(t, string(body)) {
		have[p] = true
	}

	for _, required := range pairs(t, skeleton) {
		if !have[required] {
			t.Errorf("the generic template is missing %q", required)
		}
	}
}

// A skeleton is markers and nothing else: no heading, no prose, no
// frontmatter. Anything else in the file would be content the walker ignores
// and a reader might not, which is how a schema language sneaks in.
func TestSchemasAreMarkersOnly(t *testing.T) {
	t.Parallel()

	names := append(config.DocTypeNames(), DefaultTemplateName)

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			skeleton, err := EmbeddedSchema(name)
			if err != nil {
				t.Fatalf("EmbeddedSchema(%q): %v", name, err)
			}

			if !strings.HasSuffix(skeleton, "\n") {
				t.Error("skeleton does not end in a newline")
			}

			for i, line := range strings.Split(strings.TrimSuffix(skeleton, "\n"), "\n") {
				if !strings.HasPrefix(line, "<!--") || !strings.HasSuffix(line, "-->") {
					t.Errorf("line %d is not a marker: %q", i+1, line)
				}
			}
		})
	}
}

// A name with a path separator must miss rather than reach outside the
// embedded schema directory.
func TestEmbeddedSchema_RejectsAPath(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"../impl", "nested/impl", ".."} {
		if _, err := EmbeddedSchema(name); err == nil {
			t.Errorf("EmbeddedSchema(%q) succeeded, want an error", name)
		}
	}
}
