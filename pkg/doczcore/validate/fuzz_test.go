package validate_test

import (
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/validate"
)

// FuzzDocument pins the contract that makes Document usable in a CI gate: it
// is total. Arbitrary bytes are a document with findings, never a panic and
// never an error, because the documents that most need validating are the
// broken ones.
func FuzzDocument(f *testing.F) {
	for _, seed := range []string{
		goodFrontmatter,
		goodFrontmatter + "<!--docz:summary:start-->\n## Summary\n\ntext\n<!--docz:summary:end-->\n",
		goodFrontmatter + "<!--docz:summary:end-->\n",
		goodFrontmatter + "<!--toc:start-->\n- [A](#a)\n<!--toc:end-->\n\n## A\n",
		"---\n",
		"---\nnot: yaml: at: all\n---\n",
		"",
		"\r\n",
		"# no frontmatter\n",
	} {
		f.Add([]byte(seed))
	}

	opts := validate.Options{
		Schema:      schemaOf("toc", "summary", "references"),
		Type:        rfcType(),
		Filename:    "0001-a-title.md",
		Headings:    kinds.HeadingSpec{{Kind: "summary", Level: 2, Text: "summary"}},
		MinHeadings: 2,
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, finding := range validate.Document(content, opts) {
			if finding.Code == "" {
				t.Fatal("a finding with no code: the code is the contract")
			}

			if finding.Detail == "" {
				t.Fatalf("%s has no detail", finding.Code)
			}

			if finding.Severity != validate.Error && finding.Severity != validate.Warning {
				t.Fatalf("%s has severity %d", finding.Code, finding.Severity)
			}

			// A line a consumer cannot point at is worse than no line: 0
			// means the whole document, and anything past the end is a bug.
			if finding.Line < 0 {
				t.Fatalf("%s reports line %d", finding.Code, finding.Line)
			}
		}
	})
}
