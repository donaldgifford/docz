package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/adr"
	"github.com/donaldgifford/docz/v2/pkg/design"
	"github.com/donaldgifford/docz/v2/pkg/impl"
	"github.com/donaldgifford/docz/v2/pkg/investigation"
	"github.com/donaldgifford/docz/v2/pkg/rfc"
	"github.com/donaldgifford/docz/v2/pkg/runbook"
)

// parsers are the type packages' Parse functions as any, so the test can
// marshal each result the way Fields does.
var parsers = map[string]func([]byte) (any, error){
	KindRFC:           func(b []byte) (any, error) { return rfc.Parse(b) },
	KindADR:           func(b []byte) (any, error) { return adr.Parse(b) },
	KindDesign:        func(b []byte) (any, error) { return design.Parse(b) },
	KindImpl:          func(b []byte) (any, error) { return impl.Parse(b) },
	KindInvestigation: func(b []byte) (any, error) { return investigation.Parse(b) },
	KindRunbook:       func(b []byte) (any, error) { return runbook.Parse(b) },
}

// TestFieldsMatchCorpora pins get_doc.fields to each type package's Parse
// over every .md under pkg/<type>/testdata: the fields are the Doc with a
// kind beside it and nothing else, and a document the package rejects is
// generic with the error.
func TestFieldsMatchCorpora(t *testing.T) {
	t.Parallel()
	for _, typeName := range builtinTypes {
		files, err := filepath.Glob(filepath.Join("..", "..", "pkg", typeName, "testdata", "*.md"))
		if err != nil || len(files) == 0 {
			t.Fatalf("%s corpus: %v (%d files)", typeName, err, len(files))
		}
		for _, file := range files {
			t.Run(typeName+"/"+filepath.Base(file), func(t *testing.T) {
				t.Parallel()
				content, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				got, err := json.Marshal(Fields(typeName, content))
				if err != nil {
					t.Fatal(err)
				}
				var fields map[string]json.RawMessage
				if err := json.Unmarshal(got, &fields); err != nil {
					t.Fatal(err)
				}

				doc, parseErr := parsers[typeName](content)
				if parseErr != nil {
					if string(fields["kind"]) != `"generic"` || !strings.Contains(string(fields["parse_error"]), parseErr.Error()) {
						t.Fatalf("rejected document: kind=%s parse_error=%s, want generic with %q", fields["kind"], fields["parse_error"], parseErr)
					}

					return
				}
				if string(fields["kind"]) != `"`+typeName+`"` {
					t.Fatalf("kind = %s, want %q", fields["kind"], typeName)
				}
				delete(fields, "kind")
				want, err := json.Marshal(doc)
				if err != nil {
					t.Fatal(err)
				}
				var wantFields map[string]json.RawMessage
				if err := json.Unmarshal(want, &wantFields); err != nil {
					t.Fatal(err)
				}
				if len(fields) != len(wantFields) {
					t.Fatalf("fields has %d keys, Parse has %d", len(fields), len(wantFields))
				}
				for key, raw := range wantFields {
					if string(fields[key]) != string(raw) {
						t.Errorf("fields.%s = %s, Parse = %s", key, fields[key], raw)
					}
				}
			})
		}
	}
}

func TestFieldsGeneric(t *testing.T) {
	t.Parallel()

	custom := []byte("---\nid: FW-0001\n---\n\n# Intro\n\n## Why\n\n<!--docz:why:start-->\nBecause.\n<!--docz:why:end-->\n")
	got, ok := Fields("frameworks", custom).(GenericFields)
	if !ok || got.Kind != KindGeneric || got.ParseError != "" {
		t.Fatalf("custom type = %+v", got)
	}
	if len(got.Headings) != 1 || got.Headings[0].Text != "Why" || len(got.Regions) != 1 || got.Regions[0].Kind != "why" {
		t.Errorf("custom type read %+v", got)
	}

	noFrontmatter := []byte("# Just a heading\n\n## Phase 1: Go\n\n- [ ] do it\n")
	got, ok = Fields(KindImpl, noFrontmatter).(GenericFields)
	if !ok || got.Kind != KindGeneric || got.ParseError == "" {
		t.Fatalf("rejected impl = %+v, want generic with parse_error", got)
	}
	if len(got.Headings) != 1 {
		t.Errorf("headings = %+v", got.Headings)
	}

	empty, _ := Fields("frameworks", nil).(GenericFields)
	if empty.Headings == nil || empty.Regions == nil {
		t.Errorf("empty document must marshal [] not null: %+v", empty)
	}
}
