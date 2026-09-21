package adr_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/adr"
)

// FuzzParse pins the total contract: Parse returns a Doc or an error and never
// panics, whatever markdown it is handed.
//
// It also pins the invariants a consumer relies on without being able to check
// them — that every Line a Doc reports is inside the document it was parsed
// from, and that an option or a resolution never points above its own question
// — because those are what make a Doc safe to act on. A parser that returned a
// line past the end of the file would send an editor nowhere.
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

	// The shapes that are all grammar and no prose, where the interactions live.
	f.Add([]byte("---\nid: ADR-1\n---\n\n## Consequences\n\n### Positive\n\n- a\n"))
	f.Add([]byte("---\n---\n## Consequences\n### Neutral\n- a\n### Positive\n- b\n"))
	f.Add([]byte("---\n---\n## Open Questions\n### 1. a\n- a. b\n> **Resolved 2026-01-01: (a).**\n"))
	f.Add([]byte("---\n---\n## Alternatives Considered\n### a) One\n- **b. Two.** no\n"))
	f.Add([]byte("---\n---\n## Positive\n- a\n## References\n- [x](y)\n"))

	f.Fuzz(func(t *testing.T, content []byte) {
		got, err := adr.Parse(content)
		if err != nil {
			return
		}

		total := strings.Count(string(content), "\n") + 1

		inside := func(what string, line int) {
			if line < 1 || line > total {
				t.Fatalf("%s line %d outside a %d-line document", what, line, total)
			}
		}

		for _, list := range consequenceLists(&got) {
			for _, item := range list.items {
				inside(list.label+" consequence", item.Line)
			}
		}

		for _, alt := range got.Alternatives {
			inside("alternative", alt.Line)
		}

		for _, ref := range got.References {
			inside("reference", ref.Line)
		}

		for _, q := range got.OpenQuestions {
			inside("question", q.Line)

			// An option and a resolution both belong to the question above
			// them, so a line at or above that heading means the region
			// arithmetic lost track of which question it was reading.
			for _, o := range q.Options {
				inside("option", o.Line)

				if o.Line <= q.Line {
					t.Fatalf("question %d option %q at line %d is not below its heading at %d",
						q.Number, o.Letter, o.Line, q.Line)
				}
			}

			if q.Resolved == nil {
				continue
			}

			inside("resolution", q.Resolved.Line)

			if q.Resolved.Line <= q.Line {
				t.Fatalf("question %d resolution at line %d is not below its heading at %d",
					q.Number, q.Resolved.Line, q.Line)
			}
		}
	})
}
