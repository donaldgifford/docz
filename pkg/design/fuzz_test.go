package design_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/design"
)

// FuzzParse pins the total contract: Parse returns a Doc or an error and
// never panics, whatever markdown it is handed.
//
// It also pins the invariants a consumer relies on without being able to
// check them — that every Line a Doc carries is inside the document it was
// parsed from, and that a letter is a letter. Those are what make a Doc safe
// to act on: a Line past the end of the file would send an editor, or a
// splice, into nothing.
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
	f.Add([]byte("---\nid: D-1\n---\n\n## Goals and Non-Goals\n\n### Goals\n\n- a\n"))
	f.Add([]byte("---\n---\n## Open Questions\n### 1. q\n- a. x *(recommendation)*\n"))
	f.Add([]byte("---\n---\n## Open Questions\n### 0) q\n> **Resolved 2026-01-01: (A)** y\n>\n> z\n"))
	f.Add([]byte("---\n---\n## Decisions\n| # | Question | Choice |\n| - | - | - |\n| — | q | c |\n"))
	f.Add([]byte("---\n---\n## Overview\n<!-- unterminated\n## References\n- <http://x>\n"))

	f.Fuzz(func(t *testing.T, content []byte) {
		got, err := design.Parse(content)
		if err != nil {
			return
		}

		total := strings.Count(string(content), "\n") + 1

		inside := func(label string, line int) {
			if line < 1 || line > total {
				t.Fatalf("%s line %d outside a %d-line document", label, line, total)
			}
		}

		for _, item := range got.Goals {
			inside("goal", item.Line)
		}

		for _, item := range got.NonGoals {
			inside("non-goal", item.Line)
		}

		for _, ref := range got.References {
			inside("reference", ref.Line)
		}

		for _, row := range got.Decisions {
			inside("decision", row.Line)
		}

		for _, q := range got.OpenQuestions {
			inside("question", q.Line)

			if q.Number < 0 {
				t.Fatalf("question at line %d is numbered %d", q.Line, q.Number)
			}

			for _, o := range q.Options {
				inside("option", o.Line)

				// A letter is one lower-case letter. A consumer matches a
				// resolution's Choice against it, so the two have to be
				// comparable without either side folding the case.
				if len(o.Letter) != 1 || o.Letter != strings.ToLower(o.Letter) {
					t.Fatalf("option at line %d has letter %q", o.Line, o.Letter)
				}
			}

			if q.Resolved == nil {
				continue
			}

			inside("resolution", q.Resolved.Line)

			if q.Resolved.Choice != strings.ToLower(q.Resolved.Choice) {
				t.Fatalf("resolution at line %d chose %q", q.Resolved.Line, q.Resolved.Choice)
			}
		}
	})
}
