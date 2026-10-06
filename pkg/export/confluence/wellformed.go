package confluence

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
)

// MalformedError reports a rendered body that is not well-formed XML.
// Confluence rejects such a page, so Render returns this instead of it.
type MalformedError struct {
	// Line is the 1-based line of the rendered body the parser stopped at.
	Line int
	// Err is the parser's error.
	Err error
}

func (e *MalformedError) Error() string {
	return fmt.Sprintf("confluence: rendered page is not well-formed XML at line %d: %v", e.Line, e.Err)
}

func (e *MalformedError) Unwrap() error { return e.Err }

// wellFormed decodes body inside a root declaring the ac and ri prefixes,
// in strict mode with HTML's named entities.
func wellFormed(body []byte) error {
	doc := `<root xmlns:ac="http://atlassian.com/content" xmlns:ri="http://atlassian.com/resource/identifier">` +
		string(body) + `</root>`

	d := xml.NewDecoder(strings.NewReader(doc))
	d.Strict = true
	d.Entity = xml.HTMLEntity

	for {
		_, err := d.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}

		if err != nil {
			line := 0

			var syn *xml.SyntaxError
			if errors.As(err, &syn) {
				line = syn.Line
			}

			return &MalformedError{Line: line, Err: err}
		}
	}
}
