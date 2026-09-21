package kinds_test

import (
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
)

// Every reader is total: arbitrary markdown is never invalid to it, the worst
// case is an empty result (ADR-0001). These targets pin that, and pin the one
// promise a consumer cannot check for itself — that a Line is a line of the
// input it was given, since a caller splices at it.

// lineBound is the number of lines a region has, which is the largest Line any
// reader may report. Counted the way every reader counts, by LF.
func lineBound(region []byte) int {
	return len(strings.Split(strings.TrimSuffix(string(region), "\n"), "\n"))
}

func checkLine(t *testing.T, what string, line, bound int) {
	t.Helper()

	if line < 1 || line > bound {
		t.Fatalf("%s reported line %d, outside the region's %d lines", what, line, bound)
	}
}

func checkText(t *testing.T, what, text string) {
	t.Helper()

	if text != strings.TrimSpace(text) {
		t.Fatalf("%s text is not trimmed: %q", what, text)
	}

	if strings.Contains(text, "\n") {
		t.Fatalf("%s text spans lines: %q", what, text)
	}
}

func seedRegions(f *testing.F) {
	f.Helper()

	for _, seed := range []string{
		"## Summary\n\ntext\n",
		"### Goals\n\n- a\n- b that wraps\n  onto a line\n",
		"## References\n\n- [A](a.md)\n- no link\n- <https://x.invalid>\n",
		"#### Success Criteria\n\n- `make ci` passes\n- [ ] coverage\n",
		"## Decisions\n\n| # | Question | Decision |\n| - | - | - |\n| 1 | q | d |\n",
		"## Open Questions\n\n### 1. Why?\n\n> **Resolved 2026-09-20: (a).** because\n\n- a. **Yes.** *(recommendation)*\n- b. No.\n",
		"## Alternatives Considered\n\n- a. **Do it.** reason\n\n### B. Or not\n\nreason\n",
		"## Objective\n\n**Implements:** DESIGN-0011\n",
		"## Findings\n\n### Observation 1\n\nevidence\n",
		"",
		"\n",
		"## X",
		"<!-- only a comment -->",
	} {
		f.Add([]byte(seed))
	}
}

func FuzzBody(f *testing.F) {
	seedRegions(f)

	f.Fuzz(func(t *testing.T, region []byte) {
		got := kinds.Body(region)

		if got != strings.TrimSpace(got) {
			t.Fatalf("Body() is not trimmed: %q", got)
		}

		if strings.Contains(got, "<!--") {
			t.Fatalf("Body() kept a comment opener: %q", got)
		}
	})
}

func FuzzItems(f *testing.F) {
	seedRegions(f)

	f.Fuzz(func(t *testing.T, region []byte) {
		bound := lineBound(region)

		for _, it := range kinds.Items(region) {
			checkLine(t, "Item", it.Line, bound)
			checkText(t, "Item", it.Text)

			if it.Text == "" {
				t.Fatal("Items() reported an empty item")
			}
		}
	})
}

func FuzzSections(f *testing.F) {
	seedRegions(f)

	f.Fuzz(func(t *testing.T, region []byte) {
		bound := lineBound(region)

		last := 0

		for _, s := range kinds.Sections(region) {
			checkLine(t, "Section", s.Line, bound)
			checkText(t, "Section title", s.Title)

			if s.Line <= last {
				t.Fatalf("sections out of document order: %d after %d", s.Line, last)
			}

			last = s.Line
		}
	})
}

func FuzzCriteria(f *testing.F) {
	seedRegions(f)

	f.Fuzz(func(t *testing.T, region []byte) {
		bound := lineBound(region)

		for _, c := range kinds.Criteria(region) {
			checkLine(t, "Criterion", c.Line, bound)
			checkText(t, "Criterion", c.Text)

			if c.Text == "" {
				t.Fatal("Criteria() reported an empty criterion")
			}

			if strings.Contains(c.Command, "`") {
				t.Fatalf("Command kept its backticks: %q", c.Command)
			}

			// Executable means the text opens with the span, so the command
			// must be non-empty whenever it is set.
			if c.Executable && c.Command == "" {
				t.Fatalf("executable criterion with no command: %+v", c)
			}
		}
	})
}

func FuzzReferences(f *testing.F) {
	seedRegions(f)

	f.Fuzz(func(t *testing.T, region []byte) {
		bound := lineBound(region)

		for _, r := range kinds.References(region) {
			checkLine(t, "Reference", r.Line, bound)
			checkText(t, "Reference", r.Text)

			if strings.ContainsAny(r.URL, " \t\n") {
				t.Fatalf("URL contains whitespace: %q", r.URL)
			}
		}
	})
}

func FuzzAlternatives(f *testing.F) {
	seedRegions(f)

	f.Fuzz(func(t *testing.T, region []byte) {
		bound := lineBound(region)

		for _, a := range kinds.Alternatives(region) {
			checkLine(t, "Alternative", a.Line, bound)
			checkText(t, "Alternative title", a.Title)

			if a.Label != strings.ToLower(a.Label) {
				t.Fatalf("label is not folded: %q", a.Label)
			}

			if len(a.Label) > 2 {
				t.Fatalf("label is not a label: %q", a.Label)
			}
		}
	})
}

func FuzzOpenQuestions(f *testing.F) {
	seedRegions(f)

	f.Fuzz(func(t *testing.T, region []byte) {
		bound := lineBound(region)

		for _, q := range kinds.OpenQuestions(region) {
			checkLine(t, "Question", q.Line, bound)
			checkText(t, "Question title", q.Title)

			// Zero is possible, from a heading numbered "0.": the grammar
			// numbers from 1, and reporting it is what lets validate say so.
			if q.Number < 0 {
				t.Fatalf("negative question number: %d", q.Number)
			}

			for _, o := range q.Options {
				checkLine(t, "Option", o.Line, bound)
				checkText(t, "Option", o.Text)

				if len(o.Letter) != 1 || o.Letter != strings.ToLower(o.Letter) {
					t.Fatalf("option letter is not one folded letter: %q", o.Letter)
				}

				// An option belongs to its question, so it cannot precede it.
				if o.Line <= q.Line {
					t.Fatalf("option at line %d precedes its question at %d", o.Line, q.Line)
				}
			}

			if q.Resolved == nil {
				continue
			}

			checkLine(t, "Resolution", q.Resolved.Line, bound)
			checkText(t, "Resolution note", q.Resolved.Note)

			if q.Resolved.Choice != strings.ToLower(q.Resolved.Choice) {
				t.Fatalf("resolution choice is not folded: %q", q.Resolved.Choice)
			}
		}
	})
}

func FuzzDecisions(f *testing.F) {
	seedRegions(f)

	f.Fuzz(func(t *testing.T, region []byte) {
		bound := lineBound(region)

		for _, d := range kinds.Decisions(region) {
			checkLine(t, "Decision", d.Line, bound)
			checkText(t, "Decision question", d.Question)
			checkText(t, "Decision resolution", d.Resolution)

			if d.Number < 0 {
				t.Fatalf("negative decision number: %d", d.Number)
			}
		}
	})
}

func FuzzField(f *testing.F) {
	seedRegions(f)

	f.Fuzz(func(t *testing.T, region []byte) {
		for _, label := range []string{"Implements", "Answer", "Triggered by", "", "**"} {
			value, ok := kinds.Field(region, label)
			if !ok && value != "" {
				t.Fatalf("Field(%q) returned %q with ok false", label, value)
			}

			checkText(t, "Field value", value)
		}
	})
}

// FuzzInferRegions pins the span arithmetic every reader depends on. A region
// whose End is not past its Start, or whose Start is outside the document,
// would make RegionBytes return the wrong text or panic.
func FuzzInferRegions(f *testing.F) {
	spec := kinds.HeadingSpec{
		{Kind: "summary", Level: 2, Text: "summary"},
		{Kind: "scope", Level: 2, Text: "scope"},
		{Kind: "in-scope", Level: 3, Text: "in scope", Parent: "scope"},
		{Kind: "phase", Level: 3, Prefix: "phase"},
		{Kind: "tasks", Level: 4, Text: "tasks", Parent: "phase"},
	}

	for _, seed := range []string{
		"## Summary\n\ntext\n",
		"## Scope\n\n### In Scope\n\n- a\n",
		"### Phase 1: Setup\n\n#### Tasks\n\n- [ ] a\n\n---\n\n### Phase 2: Core\n",
		"#### Tasks\n\n- [ ] orphaned\n",
		"<!--docz:summary:start-->\n## Summary\n",
		"<!--toc:start-->\n<!--toc:end-->\n## Summary\n",
		"## Summary",
		"",
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, doc []byte) {
		regions := kinds.InferRegions(doc, spec)
		lines := lineBound(doc)

		last := -1

		for _, r := range regions {
			if !r.Closed {
				t.Fatalf("inferred an unclosed region: %+v", r)
			}

			if r.Start < 0 || r.Start >= r.End {
				t.Fatalf("region span is not a span: %+v", r)
			}

			if r.End > lines+1 {
				t.Fatalf("region ends at %d, past the document's %d lines", r.End, lines)
			}

			if r.Depth < 0 {
				t.Fatalf("negative depth: %+v", r)
			}

			if r.Start < last {
				t.Fatalf("regions out of start order: %d after %d", r.Start, last)
			}

			last = r.Start

			// What the readers are handed must be a slice of the document.
			if body := kinds.RegionBytes(doc, r); !strings.Contains(
				string(doc), strings.TrimSuffix(string(body), "\n")) {
				t.Fatalf("region bytes are not from the document: %q", body)
			}
		}

		// Inference and the walker must not both claim the same document.
		if len(regions) > 0 {
			for _, m := range docparse.Markers(doc) {
				if m.Kind != docparse.TocKind && m.Kind != docparse.IndexKind {
					t.Fatalf("inferred %d regions for a marked document", len(regions))
				}
			}
		}
	})
}
