package consumer

// The Phase 2 promotions (IMPL-0018): doctemplate, index, and wiki exercised
// from outside the module.
//
// These three used to be under internal/, so no consumer could reach them at
// all — a consumer that wanted to scaffold a repo had to reimplement the
// templates, the marker pair, and the nav titles, and then drift from docz.
// That every call below compiles is the whole point of the file.

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/doctemplate"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/index"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/validate"
	"github.com/donaldgifford/docz/v2/pkg/wiki"
)

// TestExternalConsumerReadsAnEmbeddedTemplate covers what docz-api needs to
// render a "create this document" affordance for a repo it has no checkout of:
// the template the binary ships, and the fact that it is marked from birth.
func TestExternalConsumerReadsAnEmbeddedTemplate(t *testing.T) {
	body, err := doctemplate.EmbeddedDocumentTemplate(config.DocType("rfc"))
	if err != nil {
		t.Fatalf("EmbeddedDocumentTemplate(rfc) = %v, want nil", err)
	}

	// Markers, not headings: a document created from this template is
	// addressable by region without anybody running a migration.
	for _, marker := range []string{"<!--docz:summary:start-->", "<!--docz:summary:end-->"} {
		if !bytes.Contains([]byte(body), []byte(marker)) {
			t.Errorf("the rfc template is missing %s", marker)
		}
	}

	// The contents are observable but not semver-governed, so nothing here
	// asserts their prose. An unknown type is the sentinel, and that is.
	if _, err := doctemplate.EmbeddedDocumentTemplate("nosuchtype"); !errors.Is(err, doctemplate.ErrNoTemplate) {
		t.Errorf("EmbeddedDocumentTemplate(nosuchtype) = %v, want ErrNoTemplate", err)
	}
}

// TestExternalConsumerResolvesASchema is the pair of calls a consumer makes to
// validate a document it fetched: get the skeleton, turn it into a schema.
func TestExternalConsumerResolvesASchema(t *testing.T) {
	skeleton, err := doctemplate.EmbeddedSchema("rfc")
	if err != nil {
		t.Fatalf("EmbeddedSchema(rfc) = %v, want nil", err)
	}

	schema := validate.SchemaFromMarkers(skeleton)

	var found bool

	for _, region := range schema.Regions {
		if region.Kind == "summary" {
			found = true

			break
		}
	}

	if !found {
		t.Errorf("the rfc schema requires no summary region:\n%s", skeleton)
	}

	// A repo's own override beats the baked-in skeleton, which is how a repo
	// tightens its schema without waiting for a docz release.
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "templates", "schema")

	if err := os.MkdirAll(dir, config.DirMode); err != nil {
		t.Fatal(err)
	}

	override := "<!--docz:overview:start-->\n<!--docz:overview:end-->\n"
	if err := os.WriteFile(filepath.Join(dir, "rfc.md"), []byte(override), config.FileMode); err != nil {
		t.Fatal(err)
	}

	got, err := doctemplate.ResolveSchema("rfc", filepath.Join(root, "docs"))
	if err != nil {
		t.Fatalf("ResolveSchema(rfc) = %v, want nil", err)
	}

	if !bytes.Equal(got, []byte(override)) {
		t.Error("ResolveSchema returned the baked-in skeleton, not the repo's override")
	}

	// And a name a document made up is the sentinel, not a panic and not a
	// path traversal.
	if _, err := doctemplate.ResolveSchema("../../etc/passwd", root); !errors.Is(err, doctemplate.ErrNoSchema) {
		t.Errorf("ResolveSchema with a path = %v, want ErrNoSchema", err)
	}
}

// TestExternalConsumerScaffoldsAndSplicesAnIndex is the README half: a consumer
// that publishes a type directory builds the same table docz does, in the same
// place, without reimplementing the marker pair.
func TestExternalConsumerScaffoldsAndSplicesAnIndex(t *testing.T) {
	header := "# RFCs\n\nProse a repo wrote.\n\n"

	body := index.Scaffold(header)

	// Exactly one pair, which is issue #99 seen from outside: a consumer that
	// appended its own would get a second, permanently empty one.
	if got := bytes.Count(body, []byte(index.BeginMarker)); got != 1 {
		t.Errorf("%d begin markers in the scaffold, want 1:\n%s", got, body)
	}

	table := "| ID | Title |\n|----|-------|\n| RFC-0001 | A proposal |\n"

	spliced, action := index.Splice(body, header, table)
	if action != index.ActionUpdated {
		t.Fatalf("Splice action = %v, want ActionUpdated", action)
	}

	if !bytes.Contains(spliced, []byte("RFC-0001")) {
		t.Errorf("the spliced README has no table:\n%s", spliced)
	}

	// A README somebody wrote by hand is left alone, and the consumer is told
	// so rather than having its content replaced.
	if got, action := index.Splice([]byte("# Mine\n"), header, table); action != index.ActionNoMarkers || got != nil {
		t.Errorf("Splice over an unmarked README = (%q, %v), want (nil, ActionNoMarkers)", got, action)
	}
}

// TestExternalConsumerDerivesANavTitle is the last of the three: the title a
// document gets in a rendered nav when it has no frontmatter and no heading to
// take one from.
func TestExternalConsumerDerivesANavTitle(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{"0001-api-rate-limiting.md", "0001 Api Rate Limiting"},
		{"contributing.md", "Contributing"},
		{"some_snake_case.md", "Some Snake Case"},
	}

	for _, tt := range tests {
		if got := wiki.FilenameTitle(tt.filename); got != tt.want {
			t.Errorf("FilenameTitle(%q) = %q, want %q", tt.filename, got, tt.want)
		}
	}
}
