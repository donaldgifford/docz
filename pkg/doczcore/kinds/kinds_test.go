package kinds_test

import (
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
)

// Every reader is handed a region: its heading on the first line, the
// content under it. region builds one from a section written the way a
// document writes it.
func region(heading string, body ...string) []byte {
	return []byte(heading + "\n\n" + strings.Join(body, "\n") + "\n")
}

func TestBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []byte
		want string
	}{
		{
			name: "prose without the heading",
			in:   region("## Decision", "We will use NATS.", "", "It is already deployed."),
			want: "We will use NATS.\n\nIt is already deployed.",
		},
		{
			name: "an unfilled section is empty, not its guidance",
			in:   region("## Decision", "<!-- What is the change we're proposing? -->"),
			want: "",
		},
		{
			name: "a multi-line comment goes whole",
			in:   region("## Context", "<!-- why this", "     is needed -->", "Real prose."),
			want: "Real prose.",
		},
		{
			name: "an unterminated comment swallows the rest, as a renderer does",
			in:   region("## Context", "kept <!-- from here on it is a comment", "and this too"),
			want: "kept",
		},
		{
			name: "a heading with no body",
			in:   []byte("## Dependencies\n"),
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := kinds.Body(tt.in); got != tt.want {
				t.Errorf("Body() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestItems(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []byte
		want []kinds.Item
	}{
		{
			name: "flat bullets",
			in:   region("### Goals", "- One thing.", "- Another."),
			want: []kinds.Item{{Text: "One thing.", Line: 3}, {Text: "Another.", Line: 4}},
		},
		{
			name: "a wrapped bullet folds to one line",
			in:   region("### Goals", "- A goal that runs", "  onto a second line."),
			want: []kinds.Item{{Text: "A goal that runs onto a second line.", Line: 3}},
		},
		{
			// A nested bullet is neither a top-level item nor a continuation
			// line, and Item has nowhere to put it. A consumer that needs the
			// nesting reads docparse.ListItems over the same bytes.
			name: "a nested bullet is not reported",
			in:   region("### Goals", "- Parent.", "  - Child."),
			want: []kinds.Item{{Text: "Parent.", Line: 3}},
		},
		{
			name: "numbered items count",
			in:   region("## Approach", "1. First.", "2. Second."),
			want: []kinds.Item{{Text: "First.", Line: 3}, {Text: "Second.", Line: 4}},
		},
		{
			name: "the template's empty bullet is not an item",
			in:   region("### Goals", "-"),
			want: nil,
		},
		{
			name: "inline markdown is kept verbatim",
			in:   region("### Goals", "- **Bold** and `code` and [a link](x.md)."),
			want: []kinds.Item{{Text: "**Bold** and `code` and [a link](x.md).", Line: 3}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := kinds.Items(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("Items() = %+v, want %+v", got, tt.want)
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("Items()[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestSections(t *testing.T) {
	t.Parallel()

	in := region("## Findings",
		"<!-- fill this in as you go -->",
		"",
		"### Observation 1",
		"",
		"The queue drops messages under load.",
		"",
		"#### Evidence",
		"",
		"Log output here.",
		"",
		"### Observation 2",
		"",
		"Nothing else reproduces.",
	)

	got := kinds.Sections(in)
	if len(got) != 2 {
		t.Fatalf("Sections() returned %d sections, want 2: %+v", len(got), got)
	}

	if got[0].Title != "Observation 1" || got[1].Title != "Observation 2" {
		t.Errorf("titles = %q, %q", got[0].Title, got[1].Title)
	}

	// A level-4 heading is part of its section's body, so a finding that
	// breaks its evidence into sub-points stays one observation.
	if !strings.Contains(got[0].Body, "#### Evidence") {
		t.Errorf("a deeper heading was not kept in the body: %q", got[0].Body)
	}

	if got[1].Body != "Nothing else reproduces." {
		t.Errorf("second body = %q", got[1].Body)
	}
}

func TestCriteria(t *testing.T) {
	t.Parallel()

	in := region("#### Success Criteria",
		"- `make ci` passes with zero errors",
		"- [ ] Test coverage above 80%",
		"- Errors mention the file, per `docz status`",
		"1. Not a criterion, this is a procedure",
	)

	got := kinds.Criteria(in)
	if len(got) != 3 {
		t.Fatalf("Criteria() returned %d, want 3: %+v", len(got), got)
	}

	if !got[0].Executable || got[0].Command != "make ci" {
		t.Errorf("first = %+v, want executable with command 'make ci'", got[0])
	}

	if got[1].Text != "Test coverage above 80%" {
		t.Errorf("a leading checkbox was not stripped: %q", got[1].Text)
	}

	// A backtick span in the middle of a criterion names something other
	// than a command, so neither Executable nor Command is set from it.
	if got[2].Executable {
		t.Errorf("third is executable, want not: %+v", got[2])
	}

	if got[2].Command != "" {
		t.Errorf("third command = %q, want empty", got[2].Command)
	}
}

func TestReferences(t *testing.T) {
	t.Parallel()

	in := region("## References",
		"- [ADR-0002](../adr/0002-x.md) — the decision",
		"- A bullet with no link at all",
		"- <https://example.invalid/spec>",
		"- [nested [brackets]](../x.md)",
		`- [titled](../y.md "A title")`,
	)

	got := kinds.References(in)
	if len(got) != 5 {
		t.Fatalf("References() returned %d, want 5: %+v", len(got), got)
	}

	want := []string{
		"../adr/0002-x.md",
		"",
		"https://example.invalid/spec",
		"../x.md",
		"../y.md",
	}

	for i, url := range want {
		if got[i].URL != url {
			t.Errorf("References()[%d].URL = %q, want %q (%q)", i, got[i].URL, url, got[i].Text)
		}
	}
}

func TestField(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		in        []byte
		label     string
		wantValue string
		wantOK    bool
	}{
		{
			name:      "the colon inside the bold",
			in:        region("## Objective", "**Implements:** DESIGN-0011"),
			label:     "Implements",
			wantValue: "DESIGN-0011",
			wantOK:    true,
		},
		{
			name:      "the colon outside the bold",
			in:        region("## Conclusion", "**Answer**: Yes"),
			label:     "Answer",
			wantValue: "Yes",
			wantOK:    true,
		},
		{
			name:      "an unfilled field is found with an empty value",
			in:        region("## Conclusion", "**Answer:** <!-- Yes / No / Inconclusive -->"),
			label:     "Answer",
			wantValue: "",
			wantOK:    true,
		},
		{
			name:   "an absent field is not found",
			in:     region("## Conclusion", "Some prose."),
			label:  "Answer",
			wantOK: false,
		},
		{
			name:      "a field inside a blockquote",
			in:        region("## Context", "> **Triggered by:** issue #100"),
			label:     "Triggered by",
			wantValue: "issue #100",
			wantOK:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			value, ok := kinds.Field(tt.in, tt.label)
			if ok != tt.wantOK {
				t.Fatalf("Field(%q) ok = %t, want %t", tt.label, ok, tt.wantOK)
			}

			if value != tt.wantValue {
				t.Errorf("Field(%q) = %q, want %q", tt.label, value, tt.wantValue)
			}
		})
	}
}

func TestDecisions(t *testing.T) {
	t.Parallel()

	in := region("## Decisions",
		"| # | Question | Decision |",
		"| - | -------- | -------- |",
		"| 1 | Where does the schema come from? | **(d)** a marker skeleton |",
		"| — | **Amendment 2026-09-20** | inference is permanent |",
	)

	got := kinds.Decisions(in)
	if len(got) != 2 {
		t.Fatalf("Decisions() returned %d, want 2: %+v", len(got), got)
	}

	if got[0].Number != 1 || got[0].Question != "Where does the schema come from?" {
		t.Errorf("first = %+v", got[0])
	}

	// An em dash in the number column is an amendment row, and it is still a
	// decision worth reporting.
	if got[1].Number != 0 || !strings.Contains(got[1].Question, "Amendment") {
		t.Errorf("second = %+v", got[1])
	}

	if got[0].Line != 5 {
		t.Errorf("first row line = %d, want 5", got[0].Line)
	}
}

func TestDecisions_SkipsATableWithoutBothColumns(t *testing.T) {
	t.Parallel()

	in := region("## Decisions",
		"| Component | Value |",
		"| --------- | ----- |",
		"| Go | 1.26.4 |",
		"",
		"| Open Question | Resolution |",
		"| ------------- | ---------- |",
		"| Which walker? | hand-rolled |",
	)

	got := kinds.Decisions(in)
	if len(got) != 1 {
		t.Fatalf("Decisions() returned %d, want 1: %+v", len(got), got)
	}

	if got[0].Question != "Which walker?" || got[0].Resolution != "hand-rolled" {
		t.Errorf("row = %+v", got[0])
	}
}

func TestAlternatives(t *testing.T) {
	t.Parallel()

	t.Run("lettered bullets with bold lead-ins", func(t *testing.T) {
		t.Parallel()

		in := region("## Alternatives Considered",
			"- a. **Read the template.** Zero config, and an override edits the schema.",
			"- b. **A config block.** Explicit, but duplicates the template.",
		)

		got := kinds.Alternatives(in)
		if len(got) != 2 {
			t.Fatalf("Alternatives() returned %d, want 2: %+v", len(got), got)
		}

		if got[0].Label != "a" || got[0].Title != "Read the template." {
			t.Errorf("first = %+v", got[0])
		}

		if !strings.HasPrefix(got[0].Text, "Zero config") {
			t.Errorf("first text = %q", got[0].Text)
		}
	})

	t.Run("an unlabelled one-line bullet is all text", func(t *testing.T) {
		t.Parallel()

		got := kinds.Alternatives(region("## Alternatives Considered",
			"- Use goldmark v1.7, since it is already a dependency."))

		if len(got) != 1 {
			t.Fatalf("Alternatives() returned %d, want 1", len(got))
		}

		if got[0].Label != "" || got[0].Title != "" {
			t.Errorf("invented a label or title: %+v", got[0])
		}

		if !strings.HasPrefix(got[0].Text, "Use goldmark") {
			t.Errorf("text = %q", got[0].Text)
		}
	})

	t.Run("level-3 headings when there are no bullets", func(t *testing.T) {
		t.Parallel()

		in := region("## Alternatives Considered",
			"### A. Vendor the parser",
			"",
			"Too much code to own.",
			"",
			"### B. Depend on goldmark",
			"",
			"A CommonMark AST we do not need.",
		)

		got := kinds.Alternatives(in)
		if len(got) != 2 {
			t.Fatalf("Alternatives() returned %d, want 2: %+v", len(got), got)
		}

		if got[0].Label != "a" || got[0].Title != "Vendor the parser" {
			t.Errorf("first = %+v", got[0])
		}

		if got[1].Text != "A CommonMark AST we do not need." {
			t.Errorf("second text = %q", got[1].Text)
		}
	})
}

func TestOpenQuestions(t *testing.T) {
	t.Parallel()

	in := region("## Open Questions",
		"> Option `a` is my recommendation.",
		"",
		"### 1. Where does the schema come from?",
		"",
		"> **Resolved 2026-09-19: (d) — a marker skeleton.**",
		"> Neither the template nor a config block.",
		"",
		"- a. **The resolved template.** Zero config. *(recommendation)*",
		"- b. **A config block.** Explicit but duplicated.",
		"- d. Other.",
		"",
		"### 2. Marker spelling on the read side",
		"",
		"- a. **Lenient read, canonical write.** *(recommendation)*",
		"- b. Canonical only.",
	)

	got := kinds.OpenQuestions(in)
	if len(got) != 2 {
		t.Fatalf("OpenQuestions() returned %d, want 2: %+v", len(got), got)
	}

	if got[0].Number != 1 || got[0].Title != "Where does the schema come from?" {
		t.Errorf("first = number %d title %q", got[0].Number, got[0].Title)
	}

	if len(got[0].Options) != 3 {
		t.Fatalf("first has %d options, want 3: %+v", len(got[0].Options), got[0].Options)
	}

	if !got[0].Options[0].Recommended || got[0].Options[1].Recommended {
		t.Errorf("recommendation read from the wrong option: %+v", got[0].Options)
	}

	if got[0].Options[2].Letter != "d" {
		t.Errorf("third option letter = %q, want d", got[0].Options[2].Letter)
	}

	if got[0].Resolved == nil {
		t.Fatal("first question has no resolution")
	}

	if got[0].Resolved.Date != "2026-09-19" || got[0].Resolved.Choice != "d" {
		t.Errorf("resolution = %+v", *got[0].Resolved)
	}

	if !strings.Contains(got[0].Resolved.Note, "Neither the template") {
		t.Errorf("the resolution note dropped its continuation: %q", got[0].Resolved.Note)
	}

	// The second question's options must not be swept into the first, and it
	// is still open.
	if got[1].Resolved != nil {
		t.Errorf("second question was read as resolved: %+v", *got[1].Resolved)
	}

	if len(got[1].Options) != 2 {
		t.Errorf("second has %d options, want 2", len(got[1].Options))
	}
}

// The fleet writes the recommendation as option "a" by habit, but an author
// who recommends "c" says so with the marker. Reading the marker is what
// lets them.
func TestOpenQuestions_RecommendationIsTheMarkerNotTheLetter(t *testing.T) {
	t.Parallel()

	got := kinds.OpenQuestions(region("## Open Questions",
		"### 1. Which?",
		"",
		"- a. Plain.",
		"- c. **The good one.** *(recommendation)*",
	))

	if len(got) != 1 || len(got[0].Options) != 2 {
		t.Fatalf("got %+v", got)
	}

	if got[0].Options[0].Recommended {
		t.Error("option a was marked recommended without the marker")
	}

	if !got[0].Options[1].Recommended {
		t.Error("option c carries the marker and was not marked recommended")
	}
}
