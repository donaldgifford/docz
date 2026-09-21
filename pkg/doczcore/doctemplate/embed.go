package doctemplate

import (
	"bytes"
	"embed"
	"fmt"
	"regexp"
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

// schemaDir is the subdirectory of templates/ that holds the marker
// skeletons, on disk and in the embedded tree alike. One spelling, because a
// repo-local override has to land where the resolver looks.
const schemaDir = "schema"

// schemaName is the schema-name grammar: lower-case alphanumerics, hyphens,
// and underscores, starting with an alphanumeric.
//
// Narrow on purpose. A name is a filename stem, and it arrives from a
// document's own frontmatter, which may say anything at all. Anything with a
// separator, a dot segment, or an upper-case letter in it would resolve
// differently on a case-insensitive filesystem than on a case-sensitive one,
// or escape the directory, or not resolve at all — so the grammar is checked
// before either lookup rather than left to whichever filesystem answers.
var schemaName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// EmbeddedSchema returns the embedded marker skeleton for the given schema
// name — one per built-in type, plus DefaultTemplateName for a custom type
// that has not scaffolded its own (DESIGN-0015 §3).
//
// A skeleton is a markdown body of nothing but region markers, read by the
// same walker as a document, so there is no schema language and nothing a
// skeleton can require that a document cannot show.
//
// Baked-in only, which is the point: a consumer with no checkout has the
// binary's own skeletons and nothing else (DESIGN-0014 §2.7). A repo that
// overrides one is served by ResolveSchema.
//
// A name outside the grammar is ErrBadSchemaName, which wraps ErrNoSchema, so
// a caller that only wants "there is no schema" tests one sentinel and
// validate can still tell the two apart to emit schema.name.
func EmbeddedSchema(name string) ([]byte, error) {
	if !schemaName.MatchString(name) {
		return nil, badSchemaName(name)
	}

	data, err := templateFS.ReadFile("templates/" + schemaDir + "/" + name + ".md")
	if err != nil {
		return nil, fmt.Errorf("%w: %q is not baked in", ErrNoSchema, name)
	}

	return data, nil
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
