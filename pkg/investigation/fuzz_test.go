package investigation_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/investigation"
)

// FuzzParse pins the total contract: Parse returns a Doc or an error and never
// panics, whatever markdown it is handed.
//
// It also pins the invariants a consumer relies on without being able to check
// them — that every Line is inside the document it came from, and that Verdict
// is Answer's first word and nothing else. A parser that returned a Line past
// the end of the file would send an editor nowhere, and a Verdict that
// disagreed with the Answer rendered beside it would be worse than none.
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
	f.Add([]byte("---\nid: INV-1\n---\n\n## Conclusion\n\n**Answer:** Yes\n"))
	f.Add([]byte("---\n---\n## Context\n\n**Triggered by:**\n## Conclusion\n**Answer**: no.\n"))
	f.Add([]byte("---\n---\n## Environment\n\n| a | b |\n| - | - |\n|  |  |\n"))
	f.Add([]byte("---\n---\n## Findings\n\n### 1\n\n### 2\n## Open Questions\n\n### 1. x\n\n- a. y\n"))
	f.Add([]byte("<!--docz:conclusion:start-->\n---\n---\n**Answer:** **Inconclusive** —\n"))

	f.Fuzz(func(t *testing.T, content []byte) {
		got, err := investigation.Parse(content)
		if err != nil {
			return
		}

		total := strings.Count(string(content), "\n") + 1

		inside := func(what string, line int) {
			if line < 1 || line > total {
				t.Fatalf("%s line %d outside a %d-line document", what, line, total)
			}
		}

		for _, step := range got.Approach {
			inside("approach step", step.Line)
		}

		for _, c := range got.Environment {
			inside("environment row", c.Line)
		}

		for _, finding := range got.Findings {
			inside("finding", finding.Line)
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

		for _, dec := range got.Decisions {
			inside("decision", dec.Line)
		}

		for _, ref := range got.References {
			inside("reference", ref.Line)
		}

		// The verdict is derived, so it cannot say anything the answer does not.
		if want := verdictFromAnswer(got.Answer); got.Verdict.String() != want {
			t.Fatalf("Verdict = %s, want %s for answer %q", got.Verdict, want, got.Answer)
		}

		if got.Answer == "" && got.Verdict != investigation.VerdictUnknown {
			t.Fatalf("Verdict = %s with no answer", got.Verdict)
		}
	})
}
