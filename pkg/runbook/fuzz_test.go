package runbook_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/runbook"
)

// markerLine matches a whole region-marker line, for seeding the fuzzer
// with each fixture's inferred form as well as its marked one.
var markerLine = regexp.MustCompile(`(?m)^[ \t]*<!--docz:[a-z0-9-]+:(?:start|end)-->\n`)

// FuzzParse pins the total contract: Parse returns a Doc or an error and
// never panics, every step's span lies inside the document the right way
// round, and no step ID is used twice.
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
		f.Add(markerLine.ReplaceAll(content, nil))
	}

	// The shapes that are all grammar and no prose.
	f.Add([]byte("---\n---\n### Procedure 1: a\n#### Steps\n1. a\n   ```sh\n   x\n"))
	f.Add([]byte("---\n---\n### Procedure: a\n#### Steps\n1. a\n      1. b\n   1. c\n"))
	f.Add([]byte("---\n---\n### Scenario:\n#### Steps\n- a\n1.\n   **Expected:**\n"))
	f.Add([]byte("---\n---\n## Last Verified\n| Date |\n| - |\n| x | y | z |\n"))

	f.Fuzz(func(t *testing.T, content []byte) {
		got, err := runbook.Parse(content)
		if err != nil {
			return
		}

		total := strings.Count(string(content), "\n") + 1
		seen := make(map[string]bool)

		for _, s := range got.Steps() {
			if s.Line < 1 || s.Line > total {
				t.Fatalf("step %s line %d outside a %d-line document", s.ID, s.Line, total)
			}

			if s.EndLine < s.Line || s.EndLine > total {
				t.Fatalf("step %s spans %d–%d in a %d-line document", s.ID, s.Line, s.EndLine, total)
			}

			if seen[s.ID] {
				t.Fatalf("step ID %s is used twice", s.ID)
			}

			seen[s.ID] = true
		}

		// Validate reads the same Doc and must not panic on it either.
		_ = runbook.Validate(content)
	})
}
