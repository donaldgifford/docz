package impl_test

import (
	"reflect"
	"strings"
	"testing"
	"text/template"

	doctemplate "github.com/donaldgifford/docz/v2/internal/template"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
	"github.com/donaldgifford/docz/v2/pkg/impl"
)

// typeName is the built-in this package reads.
const typeName = "impl"

// TestHeadings_MatchTheEmbeddedTemplate binds the package's heading table to
// the template it was derived from.
//
// The table is written out as data rather than derived at run time, because
// deriving it would mean this package importing internal/template, and rule R2
// stops a type package's production imports at the core. So the coupling lives
// here instead: rename a section in the template, or move a marker, and this
// test fails rather than a field silently going zero for every document the
// inference path reads.
//
// The comparison includes order, because kinds.matchRule tries text rules
// before prefix rules in slice order and a reordering could change which rule
// a heading matches.
func TestHeadings_MatchTheEmbeddedTemplate(t *testing.T) {
	t.Parallel()

	want := kinds.SpecFromTemplate(embeddedTemplate(t))
	got := impl.Headings()

	if !reflect.DeepEqual(got, want) {
		t.Errorf("Headings() and the template disagree:\ngot  %+v\nwant %+v", got, want)
	}
}

// TestHeadings_AreACopy pins that a caller cannot reshape what Parse does.
func TestHeadings_AreACopy(t *testing.T) {
	t.Parallel()

	first := impl.Headings()
	if len(first) == 0 {
		t.Fatal("Headings() is empty")
	}

	first[0].Kind = "clobbered"

	if second := impl.Headings(); second[0].Kind == "clobbered" {
		t.Error("Headings() handed out the package's own slice")
	}
}

// TestHeadings_RenderingChangesNothing pins that the spec is the same whether
// it is derived from the template source or from a rendered document.
//
// A heading that carried a template action would make the two differ, and the
// table has to match the rendered documents, since those are what Parse reads.
func TestHeadings_RenderingChangesNothing(t *testing.T) {
	t.Parallel()

	source := kinds.SpecFromTemplate(embeddedTemplate(t))
	rendered := kinds.SpecFromTemplate(renderedTemplate(t))

	if !reflect.DeepEqual(source, rendered) {
		t.Errorf("the spec changes when the template is rendered:\nsource   %+v\nrendered %+v",
			source, rendered)
	}
}

// embeddedTemplate returns the type's template as it ships.
//
// internal/template is reachable from a test inside the module, and only from
// a test: an external test package's imports are not in the package's own
// dependency graph, so the layer test does not see this one.
func embeddedTemplate(t *testing.T) []byte {
	t.Helper()

	body, err := doctemplate.EmbeddedDocumentTemplate(config.DocType(typeName))
	if err != nil {
		t.Fatal(err)
	}

	return []byte(body)
}

// renderedTemplate returns the type's template rendered the way docz create
// renders it, which is the bytes a user's first document holds.
func renderedTemplate(t *testing.T) []byte {
	t.Helper()

	tmpl, err := template.New(typeName).Parse(string(embeddedTemplate(t)))
	if err != nil {
		t.Fatalf("parsing the %s template: %v", typeName, err)
	}

	typ := config.DefaultConfig().Types[typeName]

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
		t.Fatalf("rendering the %s template: %v", typeName, err)
	}

	return []byte(sb.String())
}
