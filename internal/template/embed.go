package template

import (
	"embed"
	"fmt"
)

//go:embed templates/*.md
var templateFS embed.FS

// EmbeddedDocumentTemplate returns the embedded default template for the given
// document type. Valid types: rfc, adr, design, impl.
func EmbeddedDocumentTemplate(docType string) (string, error) {
	data, err := templateFS.ReadFile("templates/" + docType + ".md")
	if err != nil {
		return "", fmt.Errorf("no embedded template for type %q: %w", docType, err)
	}
	return string(data), nil
}

// EmbeddedIndexHeader returns the embedded default index header template for
// the given document type. Valid types: rfc, adr, design, impl.
func EmbeddedIndexHeader(docType string) (string, error) {
	data, err := templateFS.ReadFile("templates/index_" + docType + ".md")
	if err != nil {
		return "", fmt.Errorf("no embedded index header for type %q: %w", docType, err)
	}
	return string(data), nil
}
