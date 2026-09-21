package rfc_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/rfc"
)

// FuzzParse pins the total contract: Parse returns a Doc or an error and never
// panics, whatever markdown it is handed.
//
// It also pins the invariant a consumer relies on without being able to check
// it — that every Line is inside the document it was parsed from — because
// that is what makes a Doc safe to act on. A Line past the end of the file
// would send an editor, a splice, or a reviewer nowhere, and a risks row's
// Line is derived by arithmetic over a table offset rather than reported by a
// reader, which is exactly the shape that goes wrong off the happy path.
func FuzzParse(f *testing.F) {
	seeds, err := filepath.Glob(filepath.Join("testdata", "*.md"))
	if err != nil {
		f.Fatalf("globbing seeds: %v", err)
	}

	for _, name := range seeds {
		content, err := os.ReadFile(name)
		if err != nil {
			f.Fatalf("reading seed: %v", err)
		}

		f.Add(content)
		f.Add(unmark(content))
	}

	// The shapes that are all grammar and no prose, where the interactions
	// live.
	f.Add([]byte("---\nid: RFC-1\n---\n\n## Risks and Mitigations\n\n" +
		"| a | b | c | d |\n| - | - | - | - |\n| x |  |  |  |\n"))
	f.Add([]byte("---\n---\n## Alternatives Considered\n\n- **A. x.** y\n- z\n"))
	f.Add([]byte("---\n---\n## Open Questions\n\n### 1. a\n\n- a. b\n\n" +
		"> **Resolved 2026-09-20: (a)** ok\n"))
	f.Add([]byte("---\n---\n## Success Criteria\n\n- `make ci` passes\n- [x] and this\n"))
	f.Add([]byte("---\n---\n## References\n\n- [a](b.md)\n- <https://x/>\n"))

	f.Fuzz(func(t *testing.T, content []byte) {
		got, err := rfc.Parse(content)
		if err != nil {
			return
		}

		total := strings.Count(string(content), "\n") + 1

		inside := func(what string, line int) {
			if line < 1 || line > total {
				t.Fatalf("%s line %d outside a %d-line document", what, line, total)
			}
		}

		for _, alt := range got.Alternatives {
			inside("alternative", alt.Line)
		}

		for _, risk := range got.Risks {
			inside("risk", risk.Line)
		}

		for _, c := range got.Criteria {
			inside("criterion", c.Line)
		}

		for _, ref := range got.References {
			inside("reference", ref.Line)
		}

		for _, q := range got.OpenQuestions {
			inside("question", q.Line)

			for _, o := range q.Options {
				inside("option", o.Line)
			}

			if q.Resolved != nil {
				inside("resolution", q.Resolved.Line)
			}
		}

		// Validate reads the same document through the same parser, so a
		// finding's Line is under the same contract as a field's.
		for _, finding := range rfc.Validate(content) {
			if finding.Line != 0 {
				inside("finding "+finding.Code, finding.Line)
			}
		}
	})
}
