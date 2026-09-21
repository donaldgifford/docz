package validate_test

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/doctemplate"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/validate"
)

// TestSchemaFromMarkers_DerivesTheSameSchemaAsItsTemplate is why the schema
// can be its own artifact.
//
// DESIGN-0015 §3 rejected deriving a schema from the resolved template, on
// the grounds that a template is a rendering artifact an override can
// silently loosen. What replaced that argument is this test: the pair must
// say the same thing, so the two files can only be edited together, and a
// template that grows a section without its schema growing one fails here
// rather than shipping a document that passes a schema it has outgrown.
//
// The IMPL template's three placeholder phases collapse to the one phase
// entry its skeleton lists, because a schema is a set of requirements rather
// than a sequence: it says which regions must exist, never how many.
func TestSchemaFromMarkers_DerivesTheSameSchemaAsItsTemplate(t *testing.T) {
	t.Parallel()

	for _, name := range config.DocTypeNames() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			body, err := doctemplate.EmbeddedDocumentTemplate(config.DocType(name))
			if err != nil {
				t.Fatal(err)
			}

			skeleton, err := doctemplate.EmbeddedSchema(name)
			if err != nil {
				t.Fatal(err)
			}

			fromTemplate := formatSchema(validate.SchemaFromMarkers([]byte(body)))
			fromSkeleton := formatSchema(validate.SchemaFromMarkers(skeleton))

			if fromTemplate != fromSkeleton {
				t.Errorf("the %s template and its schema disagree\n--- template ---\n%s\n--- schema ---\n%s",
					name, fromTemplate, fromSkeleton)
			}
		})
	}
}

func formatSchema(s validate.Schema) string {
	lines := make([]string, 0, len(s.Regions))
	for _, r := range s.Regions {
		lines = append(lines, fmt.Sprintf("%s under %q", r.Kind, r.Parent))
	}

	sort.Strings(lines)

	return strings.Join(lines, "\n")
}

// The kinds and parents each baked-in skeleton requires, from DESIGN-0015 §3.
// Spelled out rather than derived, so a skeleton edited by accident fails
// against the design rather than against itself.
func TestSchemaFromMarkers_BakedInSkeletons(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want []string
	}{
		{
			name: "rfc",
			want: []string{
				`alternatives under ""`, `criteria under ""`, `problem under ""`,
				`proposal under ""`, `references under ""`, `risks under ""`,
				`summary under ""`, `toc under ""`,
			},
		},
		{
			name: "adr",
			want: []string{
				`alternatives under ""`, `consequences under ""`, `context under ""`,
				`decision under ""`, `negative under "consequences"`,
				`neutral under "consequences"`, `positive under "consequences"`,
				`references under ""`, `summary under ""`, `toc under ""`,
			},
		},
		{
			name: "design",
			want: []string{
				`api-changes under ""`, `background under ""`, `data-model under ""`,
				`detailed-design under ""`, `goals under ""`, `non-goals under ""`,
				`open-questions under ""`, `overview under ""`, `references under ""`,
				`rollout under ""`, `testing under ""`, `toc under ""`,
			},
		},
		{
			name: "impl",
			want: []string{
				`criteria under "phase"`, `dependencies under ""`, `file-changes under ""`,
				`in-scope under "scope"`, `objective under ""`,
				`out-of-scope under "scope"`, `phase under ""`, `references under ""`,
				`scope under ""`, `tasks under "phase"`, `testing under ""`, `toc under ""`,
			},
		},
		{
			name: "investigation",
			want: []string{
				`approach under ""`, `conclusion under ""`, `context under ""`,
				`environment under ""`, `findings under ""`, `hypothesis under ""`,
				`question under ""`, `recommendation under ""`, `references under ""`,
				`toc under ""`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			skeleton, err := doctemplate.EmbeddedSchema(tt.name)
			if err != nil {
				t.Fatal(err)
			}

			sort.Strings(tt.want)

			if got := formatSchema(validate.SchemaFromMarkers(skeleton)); got !=
				strings.Join(tt.want, "\n") {
				t.Errorf("schema/%s.md\n--- got ---\n%s\n--- want ---\n%s",
					tt.name, got, strings.Join(tt.want, "\n"))
			}
		})
	}
}

// The generic skeleton requires only what a custom type is guaranteed to
// have. Anything more would mean a scaffolded type failed validation before
// its author had written a line.
func TestSchemaFromMarkers_GenericSkeleton(t *testing.T) {
	t.Parallel()

	skeleton, err := doctemplate.EmbeddedSchema("default")
	if err != nil {
		t.Fatal(err)
	}

	want := "references under \"\"\ntoc under \"\""
	if got := formatSchema(validate.SchemaFromMarkers(skeleton)); got != want {
		t.Errorf("schema/default.md = %q, want %q", got, want)
	}
}

// A schema is a set, so two skeletons requiring the same regions in a
// different order are the same schema. Without that the derivation test
// would depend on the order a template happens to list its sections in.
func TestSchemaFromMarkers_OrderIndependent(t *testing.T) {
	t.Parallel()

	first := validate.SchemaFromMarkers([]byte(
		"<!--docz:summary:start-->\n<!--docz:summary:end-->\n" +
			"<!--docz:references:start-->\n<!--docz:references:end-->\n"))
	second := validate.SchemaFromMarkers([]byte(
		"<!--docz:references:start-->\n<!--docz:references:end-->\n" +
			"<!--docz:summary:start-->\n<!--docz:summary:end-->\n"))

	if formatSchema(first) != formatSchema(second) {
		t.Errorf("order changed the schema:\n%v\n%v", first.Regions, second.Regions)
	}
}

// The zero Schema requires nothing, which is what a document whose schema
// name resolves nowhere gets. The run is then well-formedness only, and that
// is deliberately still useful.
func TestDocument_EmptySchemaIsWellFormednessOnly(t *testing.T) {
	t.Parallel()

	doc := []byte("---\nid: RFC-0001\ntitle: T\nstatus: Draft\ncreated: 2026-09-20\n---\n\n" +
		"# T\n\n<!--docz:summary:start-->\n## Summary\n\ntext\n")

	got := validate.Document(doc, validate.Options{})

	codes := make([]string, 0, len(got))
	for _, f := range got {
		codes = append(codes, f.Code)
	}

	// The unclosed marker is still reported; no region is required.
	if strings.Join(codes, " ") != "marker.unclosed" {
		t.Errorf("findings = %v, want marker.unclosed alone", codes)
	}
}
