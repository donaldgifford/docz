package doctemplate

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
)

// The schema resolution tiers (IMPL-0018 Phase 2, DESIGN-0015 §3):
// <docsDir>/templates/schema/<name>.md, then the embedded skeleton, else
// ErrNoSchema. A name outside the grammar reaches neither tier.

// writeSchema puts a skeleton in a temp repo's override directory and returns
// the docsDir to resolve against.
func writeSchema(t *testing.T, name, body string) string {
	t.Helper()

	docsDir := t.TempDir()
	dir := filepath.Join(docsDir, config.TemplatesDir, schemaDir)

	if err := os.MkdirAll(dir, config.DirMode); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, name+".md")
	if err := os.WriteFile(path, []byte(body), config.FileMode); err != nil {
		t.Fatal(err)
	}

	return docsDir
}

// TestResolveSchema_LocalOverrideBeatsTheBakedInOne is the reason the schema is
// a file of markers rather than a table in the binary: a repo that adds a
// section to its own copy gets it required without waiting for a docz release.
func TestResolveSchema_LocalOverrideBeatsTheBakedInOne(t *testing.T) {
	t.Parallel()

	override := "<!--docz:objective:start-->\n<!--docz:objective:end-->\n"
	docsDir := writeSchema(t, "impl", override)

	got, err := ResolveSchema("impl", docsDir)
	if err != nil {
		t.Fatalf("ResolveSchema(impl) = %v, want nil", err)
	}

	if !bytes.Equal(got, []byte(override)) {
		t.Errorf("ResolveSchema returned the baked-in skeleton, not the override:\n%s", got)
	}

	// The override is returned verbatim, not rendered: a literal "{{" in
	// somebody's file must not be given a meaning it does not have.
	baked, err := EmbeddedSchema("impl")
	if err != nil {
		t.Fatalf("EmbeddedSchema(impl) = %v, want nil", err)
	}

	if bytes.Equal(got, baked) {
		t.Error("the override and the baked-in skeleton are identical: the test proves nothing")
	}
}

// TestResolveSchema_FallsBackToTheBakedInOne covers the other tier: a repo with
// no override of its own resolves the skeleton the binary ships, which is what
// every repo in the fleet does today.
func TestResolveSchema_FallsBackToTheBakedInOne(t *testing.T) {
	t.Parallel()

	// An override for some *other* name must not be picked up for this one.
	docsDir := writeSchema(t, "rfc", "<!--docz:summary:start-->\n<!--docz:summary:end-->\n")

	got, err := ResolveSchema("impl", docsDir)
	if err != nil {
		t.Fatalf("ResolveSchema(impl) = %v, want nil", err)
	}

	want, err := EmbeddedSchema("impl")
	if err != nil {
		t.Fatalf("EmbeddedSchema(impl) = %v, want nil", err)
	}

	if !bytes.Equal(got, want) {
		t.Error("ResolveSchema did not fall back to the baked-in skeleton")
	}
}

// TestResolveSchema_UnknownNameIsErrNoSchema pins the sentinel and that the
// message names the path a repo would create, since the usual cause is a
// document naming a schema the repo meant to write and has not.
func TestResolveSchema_UnknownNameIsErrNoSchema(t *testing.T) {
	t.Parallel()

	docsDir := t.TempDir()

	_, err := ResolveSchema("frameworks", docsDir)
	if !errors.Is(err, ErrNoSchema) {
		t.Fatalf("ResolveSchema(frameworks) = %v, want it to wrap ErrNoSchema", err)
	}

	if errors.Is(err, ErrBadSchemaName) {
		t.Error("a legal name that resolves nowhere must not read as a bad name")
	}

	want := filepath.Join(docsDir, config.TemplatesDir, schemaDir, "frameworks.md")
	if !bytes.Contains([]byte(err.Error()), []byte(want)) {
		t.Errorf("error %q does not name the path %s", err, want)
	}
}

// TestSchemaName_RejectsWhatCannotBeAFilename is the grammar. Every case here
// arrives from a document's own frontmatter, which may say anything, so the
// check runs before either lookup rather than being left to whichever
// filesystem answers — a name that differs only in case resolves on one
// machine and not on another.
func TestSchemaName_RejectsWhatCannotBeAFilename(t *testing.T) {
	t.Parallel()

	for _, name := range []string{
		"", "..", ".", "../impl", "nested/impl", "impl/", `impl\x`,
		"Impl", "IMPL", "-impl", "_impl", "impl.md", "impl doc",
		"../../etc/passwd", "impl\x00", "im\npl",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, embedErr := EmbeddedSchema(name)
			if !errors.Is(embedErr, ErrBadSchemaName) {
				t.Errorf("EmbeddedSchema(%q) = %v, want ErrBadSchemaName", name, embedErr)
			}

			// ErrBadSchemaName wraps ErrNoSchema, so a caller that only wants
			// "there is no schema" tests one sentinel.
			if !errors.Is(embedErr, ErrNoSchema) {
				t.Errorf("EmbeddedSchema(%q) error does not wrap ErrNoSchema", name)
			}

			_, resolveErr := ResolveSchema(name, t.TempDir())
			if !errors.Is(resolveErr, ErrBadSchemaName) {
				t.Errorf("ResolveSchema(%q) = %v, want ErrBadSchemaName", name, resolveErr)
			}
		})
	}
}

// TestSchemaName_AcceptsTheShapesARepoWouldUse is the other half: the grammar
// has to admit every name a real type could carry, or a repo's own type cannot
// have a schema at all.
func TestSchemaName_AcceptsTheShapesARepoWouldUse(t *testing.T) {
	t.Parallel()

	for _, name := range []string{
		"impl", "investigation", "default", "frameworks",
		"runbook-v2", "runbook_v2", "adr2", "0day",
	} {
		if !schemaName.MatchString(name) {
			t.Errorf("schemaName rejects %q, which a repo could legitimately use", name)
		}
	}
}

// TestEmbeddedSchema_EveryBuiltInAndTheGenericOne is the coverage guarantee: a
// built-in with no skeleton would validate against nothing and report no
// missing region, so the gap would be silent.
func TestEmbeddedSchema_EveryBuiltInAndTheGenericOne(t *testing.T) {
	t.Parallel()

	for _, name := range append(config.DocTypeNames(), DefaultTemplateName) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := EmbeddedSchema(name)
			if err != nil {
				t.Fatalf("EmbeddedSchema(%q) = %v, want a skeleton", name, err)
			}

			if len(got) == 0 {
				t.Errorf("EmbeddedSchema(%q) is empty", name)
			}

			if !bytes.Contains(got, []byte(":start-->")) {
				t.Errorf("EmbeddedSchema(%q) carries no start marker:\n%s", name, got)
			}
		})
	}
}
