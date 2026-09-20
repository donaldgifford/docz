package doctemplate

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
)

// The surface this package gained on promotion (IMPL-0018 Phase 2,
// DESIGN-0014 §2.7): the two error sentinels, GenericTemplate, and
// DefaultConfigYAML.

// TestResolve_NoTemplateNamesTheTypeAndThePath is issue #92. The old error was
// the embedded FS's "file does not exist", which told a person neither which
// type was misconfigured nor where to put the file.
func TestResolve_NoTemplateNamesTheTypeAndThePath(t *testing.T) {
	t.Parallel()

	docsDir := t.TempDir()

	_, err := Resolve("frameworks", "", docsDir)
	if !errors.Is(err, ErrNoTemplate) {
		t.Fatalf("Resolve for an unknown type = %v, want it to wrap ErrNoTemplate", err)
	}

	msg := err.Error()

	if !strings.Contains(msg, `"frameworks"`) {
		t.Errorf("error %q does not name the type", msg)
	}

	want := filepath.Join(docsDir, config.TemplatesDir, "frameworks.md")
	if !strings.Contains(msg, want) {
		t.Errorf("error %q does not name the path %s a person would create", msg, want)
	}
}

// TestResolve_LocalOverrideBeatsTheMissError pins that the sentinel is only
// reached when every tier misses: a custom type with a file on disk resolves,
// which is how a repo adds a type without a docz release.
func TestResolve_LocalOverrideBeatsTheMissError(t *testing.T) {
	t.Parallel()

	docsDir := t.TempDir()
	dir := filepath.Join(docsDir, config.TemplatesDir)

	if err := os.MkdirAll(dir, config.DirMode); err != nil {
		t.Fatal(err)
	}

	body := "# {{ .Prefix }}-{{ .Number }}: {{ .Title }}\n"
	if err := os.WriteFile(filepath.Join(dir, "frameworks.md"), []byte(body), config.FileMode); err != nil {
		t.Fatal(err)
	}

	got, err := Resolve("frameworks", "", docsDir)
	if err != nil {
		t.Fatalf("Resolve with an on-disk override = %v, want nil", err)
	}

	if got != body {
		t.Errorf("Resolve = %q, want the file verbatim", got)
	}
}

// TestEmbeddedDocumentTemplate_UnknownTypeIsErrNoTemplate covers the tier the
// resolver falls through to, since a consumer with no checkout calls it
// directly.
func TestEmbeddedDocumentTemplate_UnknownTypeIsErrNoTemplate(t *testing.T) {
	t.Parallel()

	if _, err := EmbeddedDocumentTemplate("nosuchtype"); !errors.Is(err, ErrNoTemplate) {
		t.Errorf("EmbeddedDocumentTemplate(nosuchtype) = %v, want ErrNoTemplate", err)
	}
}

// TestGenericTemplate_IsTheDefaultTemplate pins that the named accessor and
// the type-shaped one agree, and that what comes back is the scaffold a custom
// type gets: markers, so a document created from it validates against the
// generic skeleton (DESIGN-0015 §3).
func TestGenericTemplate_IsTheDefaultTemplate(t *testing.T) {
	t.Parallel()

	got, err := GenericTemplate()
	if err != nil {
		t.Fatalf("GenericTemplate() = %v, want nil", err)
	}

	want, err := EmbeddedDocumentTemplate(DefaultTemplateName)
	if err != nil {
		t.Fatalf("EmbeddedDocumentTemplate(default) = %v, want nil", err)
	}

	if got != want {
		t.Error("GenericTemplate() and EmbeddedDocumentTemplate(default) differ")
	}

	if !strings.Contains(got, "<!--docz:") {
		t.Error("the generic template carries no region markers")
	}
}

// TestDefaultConfigYAML_RendersOverTheDefaults is the one assertion the
// function exists for: `docz init` and a consumer that scaffolds a repo get the
// same file, so neither renders the template itself.
func TestDefaultConfigYAML_RendersOverTheDefaults(t *testing.T) {
	t.Parallel()

	got, err := DefaultConfigYAML()
	if err != nil {
		t.Fatalf("DefaultConfigYAML() = %v, want nil", err)
	}

	// Nothing unrendered is left in it. A stray action would otherwise reach
	// a user's .docz.yaml as literal text.
	if strings.Contains(got, "{{") {
		t.Errorf("rendered config still holds a template action:\n%s", got)
	}

	// And it round-trips: this is the same guard config's parity baseline
	// keeps, asserted here too because the rendering now lives in this
	// package and a change here is what would break it.
	var cfg config.Config
	if err := yaml.Unmarshal([]byte(got), &cfg); err != nil {
		t.Fatalf("the rendered config does not parse: %v\n%s", err, got)
	}

	if cfg.DocsDir != config.DefaultConfig().DocsDir {
		t.Errorf("docs_dir = %q, want the default %q",
			cfg.DocsDir, config.DefaultConfig().DocsDir)
	}
}
