package doctemplate

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
)

//go:embed templates/*.md templates/*.tmpl templates/schema/*.md
var templateFS embed.FS

// DefaultTemplateName is the type name whose template and schema scaffold a
// custom type: a body of a ToC pair, a summary, and references, and a schema
// that requires only the ToC and references (DESIGN-0015 §3).
const DefaultTemplateName = "default"

// EmbeddedDocumentTemplate returns the embedded default template for the given
// document type. Valid types: rfc, adr, design, impl, plan, investigation.
//
// docType is the typed config.DocType (DESIGN-0004 §F) so a stray
// status or path string fails to compile here rather than producing a
// confusing "no embedded template for type %q" miss at runtime.
//
// A type with no embedded template is ErrNoTemplate. The embedded FS's own
// error is not wrapped: it says only that a file does not exist, which a
// caller already knows, and the sentinel is what a caller can act on.
func EmbeddedDocumentTemplate(docType config.DocType) (string, error) {
	data, err := templateFS.ReadFile("templates/" + string(docType) + ".md")
	if err != nil {
		return "", fmt.Errorf("%w: %q has no embedded template", ErrNoTemplate, docType)
	}

	return string(data), nil
}

// GenericTemplate returns the embedded default body template — the one that
// scaffolds a custom type (DESIGN-0015 §3).
//
// Its own name rather than EmbeddedDocumentTemplate(DefaultTemplateName),
// because "default" is not a document type and asking for it through the
// type-shaped accessor reads as though it were one. A consumer scaffolding a
// repo writes this next to the skeleton EmbeddedSchema returns for the same
// name, and the pair is what makes a custom type's documents validate.
func GenericTemplate() (string, error) {
	return EmbeddedDocumentTemplate(DefaultTemplateName)
}

// EmbeddedSchema returns the embedded marker skeleton for the given schema
// name — one per built-in type, plus DefaultTemplateName for a custom type
// that has not scaffolded its own (DESIGN-0015 §3).
//
// A skeleton is a markdown body of nothing but region markers, read by the
// same walker as a document, so there is no schema language and nothing a
// skeleton can require that a document cannot show.
//
// The name is taken as a plain filename, so a caller must not pass a path: a
// name with a separator or a dot segment misses rather than escaping the
// embedded tree, because embed.FS rejects it.
func EmbeddedSchema(name string) (string, error) {
	data, err := templateFS.ReadFile("templates/schema/" + name + ".md")
	if err != nil {
		return "", fmt.Errorf("no embedded schema %q: %w", name, err)
	}

	return string(data), nil
}

// EmbeddedWikiIndex returns the embedded default wiki index template
// used to generate docs/index.md.
func EmbeddedWikiIndex() (string, error) {
	data, err := templateFS.ReadFile("templates/wiki_index.md")
	if err != nil {
		return "", fmt.Errorf("reading embedded wiki index template: %w", err)
	}
	return string(data), nil
}

// DefaultConfigYAML returns the `.docz.yaml` that `docz init` writes: the
// embedded template rendered over config.DefaultConfig().
//
// It absorbs the rendering every caller was doing for itself, so `docz init`
// and any consumer that scaffolds a repo produce the same file from the same
// source of defaults (DESIGN-0014 §2.7). The template source is deliberately
// not exported: rendering it over some *other* config would produce a file
// whose comments describe defaults it does not hold, and nothing in the fleet
// wants that.
func DefaultConfigYAML() (string, error) {
	src, err := templateFS.ReadFile("templates/docz_yaml.tmpl")
	if err != nil {
		return "", fmt.Errorf("reading embedded docz yaml template: %w", err)
	}

	tmpl, err := template.New("docz_yaml").Parse(string(src))
	if err != nil {
		return "", fmt.Errorf("parsing docz yaml template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, config.DefaultConfig()); err != nil {
		return "", fmt.Errorf("rendering docz yaml template: %w", err)
	}

	return buf.String(), nil
}
