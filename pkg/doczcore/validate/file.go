package validate

import (
	"bytes"
	"fmt"
)

// frontmatterLine is where a frontmatter finding points. Naming the line
// the block opens on rather than the key's own line is deliberate: the
// parsed value has no line, and pointing at the block is honest about that
// while still taking a reader to the right place.
const frontmatterLine = 1

// checkFile reports the byte-level findings, the ones that are about the
// file rather than the document in it.
//
// A carriage return is an error because docz will not write the file back:
// docwrite.SetStatus rejects CR outright (DESIGN-0005 Decision 7), so a
// document with one cannot have its status set, its tasks checked, or its
// ToC spliced. Reporting it as a finding is how an author learns that
// before a write fails.
func checkFile(content []byte) []Finding {
	i := bytes.IndexByte(content, '\r')
	if i < 0 {
		return nil
	}

	return []Finding{{
		Code:     "file.crlf",
		Severity: Error,
		Line:     bytes.Count(content[:i], []byte("\n")) + 1,
		Detail: fmt.Sprintf(
			"file contains a carriage return; docz writes LF only, so %s",
			"status, task, and ToC updates would be refused"),
	}}
}
