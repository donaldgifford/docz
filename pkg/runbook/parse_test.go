package runbook_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/document"
	"github.com/donaldgifford/docz/v2/pkg/runbook"
)

const frontmatter = "---\nid: RUNBOOK-0009\ntitle: \"t\"\nstatus: Draft\nauthor: a\ncreated: 2026-01-02\n---\n\n"

// marked wraps a body in a kind's markers, heading first.
func marked(kind, heading, body string) string {
	return "<!--docz:" + kind + ":start-->\n" + heading + "\n\n" + body +
		"\n<!--docz:" + kind + ":end-->\n"
}

// procedure builds a marked procedure with the given steps body, and a
// rollback when one is given.
func procedure(token, steps, rollback string) string {
	body := marked("steps", "#### Steps", steps)
	if rollback != "" {
		body += "\n" + marked("rollback", "#### Rollback", rollback)
	}

	return marked("procedure", "### Procedure "+token+": Title "+token, body)
}

func scenario(symptom, steps string) string {
	return marked("scenario", "### Scenario: "+symptom,
		"**Alert:** none\n\n"+marked("steps", "#### Steps", steps))
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()

	content, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	return content
}

func parse(t *testing.T, doc []byte) runbook.Doc {
	t.Helper()

	got, err := runbook.Parse(doc)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	return got
}

// lineOf returns the 1-based line holding the only occurrence of want, so an
// assertion names a line by its text rather than by a number a fixture edit
// would move.
func lineOf(t *testing.T, doc, want string) int {
	t.Helper()

	at := 0

	for i, line := range strings.Split(doc, "\n") {
		if strings.Contains(line, want) {
			if at != 0 {
				t.Fatalf("more than one line contains %q", want)
			}

			at = i + 1
		}
	}

	if at == 0 {
		t.Fatalf("no line contains %q", want)
	}

	return at
}

func ids(steps []runbook.Step) []string {
	out := make([]string, 0, len(steps))
	for _, s := range steps {
		out = append(out, s.ID)
	}

	return out
}

// TestParse_StepGrammar covers DESIGN-0019 §4's rules 1–6 one shape at a
// time.
func TestParse_StepGrammar(t *testing.T) {
	t.Parallel()

	t.Run("nesting three deep", func(t *testing.T) {
		t.Parallel()

		doc := parse(t, []byte(frontmatter+procedure("2",
			"1. a\n   1. b\n      1. c\n   2. d\n2. e", "")))

		got := ids(doc.Steps())
		want := []string{"2.1", "2.1.1", "2.1.1.1", "2.1.2", "2.2"}

		if !slices.Equal(got, want) {
			t.Errorf("IDs = %v, want %v", got, want)
		}

		deep, ok := doc.Step("2.1.1.1")
		if !ok || deep.Text != "c" {
			t.Errorf("Step(2.1.1.1) = %+v, %v", deep, ok)
		}
	})

	t.Run("a parent's span covers its children", func(t *testing.T) {
		t.Parallel()

		src := frontmatter + procedure("1", "1. a\n   1. b\n   2. c\n2. d", "")
		doc := parse(t, []byte(src))

		parent, _ := doc.Step("1.1")
		if want := lineOf(t, src, "2. c"); parent.EndLine != want {
			t.Errorf("1.1 EndLine = %d, want %d", parent.EndLine, want)
		}
	})

	t.Run("two commands under one step", func(t *testing.T) {
		t.Parallel()

		src := frontmatter + procedure("1",
			"1. Run both.\n\n   ```sh\n   just build\n   ```\n\n   ```json\n   {\"a\": 1}\n   ```\n\n2. Next.", "")
		doc := parse(t, []byte(src))

		step, _ := doc.Step("1.1")
		if len(step.Commands) != 2 {
			t.Fatalf("Commands = %+v, want 2", step.Commands)
		}

		if step.Commands[0].Lang != "sh" || step.Commands[0].Body != "just build" {
			t.Errorf("first command = %+v", step.Commands[0])
		}

		if step.Commands[1].Lang != "json" || step.Commands[1].Body != `{"a": 1}` {
			t.Errorf("second command = %+v", step.Commands[1])
		}

		if want := lineOf(t, src, "```json"); step.Commands[1].Line != want {
			t.Errorf("second command Line = %d, want %d", step.Commands[1].Line, want)
		}

		if step.Text != "Run both." {
			t.Errorf("Text = %q, want the commands out of it", step.Text)
		}
	})

	t.Run("a numbered line inside a command is not a step", func(t *testing.T) {
		t.Parallel()

		doc := parse(t, []byte(frontmatter+procedure("1",
			"1. Run it.\n\n   ```text\n   1. not a step\n   ```", "")))

		if got := ids(doc.Steps()); !slices.Equal(got, []string{"1.1"}) {
			t.Errorf("IDs = %v, want [1.1]", got)
		}
	})

	t.Run("expected, wrapped, out of the text", func(t *testing.T) {
		t.Parallel()

		doc := parse(t, []byte(frontmatter+procedure("1",
			"1. Check it.\n\n   **Expected:** two lines of\n   output\n\n2. Next.", "")))

		step, _ := doc.Step("1.1")
		if step.Expected != "two lines of output" {
			t.Errorf("Expected = %q", step.Expected)
		}

		if step.Text != "Check it." {
			t.Errorf("Text = %q", step.Text)
		}
	})

	t.Run("bullets under a step are prose", func(t *testing.T) {
		t.Parallel()

		doc := parse(t, []byte(frontmatter+procedure("1",
			"1. Do it.\n   - a note\n2. Next.", "")))

		if got := ids(doc.Steps()); !slices.Equal(got, []string{"1.1", "1.2"}) {
			t.Errorf("IDs = %v", got)
		}

		step, _ := doc.Step("1.1")
		if step.Text != "Do it. - a note" {
			t.Errorf("Text = %q", step.Text)
		}
	})

	t.Run("continuation lines fold, comments drop", func(t *testing.T) {
		t.Parallel()

		doc := parse(t, []byte(frontmatter+procedure("1",
			"1. Diagnose: <!-- what to check -->\n   the second line\n   and a third", "")))

		step, _ := doc.Step("1.1")
		if step.Text != "Diagnose: the second line and a third" {
			t.Errorf("Text = %q", step.Text)
		}
	})

	t.Run("rollback IDs", func(t *testing.T) {
		t.Parallel()

		doc := parse(t, []byte(frontmatter+procedure("B", "1. a", "1. undo\n2. undo more")))

		p, ok := doc.Procedure("b")
		if !ok {
			t.Fatal("Procedure(b) not found")
		}

		if got := ids(p.Rollback); !slices.Equal(got, []string{"B.R1", "B.R2"}) {
			t.Errorf("rollback IDs = %v", got)
		}
	})

	t.Run("scenario IDs", func(t *testing.T) {
		t.Parallel()

		doc := parse(t, []byte(frontmatter+scenario("one", "1. a\n2. b")+scenario("two", "1. c")))

		if got := ids(doc.Steps()); !slices.Equal(got, []string{"S1.1", "S1.2", "S2.1"}) {
			t.Errorf("IDs = %v", got)
		}

		if doc.Scenarios[1].Symptom != "two" || doc.Scenarios[1].Alert != "none" {
			t.Errorf("scenario 2 = %+v", doc.Scenarios[1])
		}
	})

	t.Run("a procedure with no token takes its index", func(t *testing.T) {
		t.Parallel()

		doc := parse(t, []byte(frontmatter+marked("procedure", "### Procedure: Rotate",
			marked("steps", "#### Steps", "1. a"))))

		if got := ids(doc.Steps()); !slices.Equal(got, []string{"1.1"}) {
			t.Errorf("IDs = %v", got)
		}

		if doc.Procedures[0].Title != "Rotate" {
			t.Errorf("Title = %q", doc.Procedures[0].Title)
		}
	})
}

func lastVerifiedDoc(rows ...string) string {
	return frontmatter + marked("last-verified", "## Last Verified",
		"| Date | PR | Commit | Verified by |\n| --- | --- | --- | --- |\n"+
			strings.Join(rows, "\n")+"\n\n**Notes:** ran it\non staging.")
}

func TestParse_LastVerified(t *testing.T) {
	t.Parallel()

	t.Run("the template's empty row is nil", func(t *testing.T) {
		t.Parallel()

		if got := parse(t, []byte(lastVerifiedDoc("|  |  |  |  |"))).LastVerified; got != nil {
			t.Errorf("LastVerified = %+v, want nil", got)
		}
	})

	t.Run("a filled row", func(t *testing.T) {
		t.Parallel()

		src := lastVerifiedDoc("| 2026-09-25 | #137 | e41203e | @a |")
		got := parse(t, []byte(src)).LastVerified

		if got == nil {
			t.Fatal("LastVerified = nil")
		}

		want := runbook.Verification{
			Date: "2026-09-25", PR: "#137", Commit: "e41203e",
			VerifiedBy: []string{"@a"}, Notes: "ran it on staging.",
			Line: lineOf(t, src, "2026-09-25"),
		}

		if got.Date != want.Date || got.PR != want.PR || got.Commit != want.Commit ||
			!slices.Equal(got.VerifiedBy, want.VerifiedBy) || got.Notes != want.Notes ||
			got.Line != want.Line {
			t.Errorf("LastVerified = %+v, want %+v", *got, want)
		}
	})

	t.Run("several verifiers", func(t *testing.T) {
		t.Parallel()

		got := parse(t, []byte(lastVerifiedDoc("| 2026-09-25 | #1 | abcdef0 | @a, @b ,, @c |"))).LastVerified
		if got == nil || !slices.Equal(got.VerifiedBy, []string{"@a", "@b", "@c"}) {
			t.Errorf("VerifiedBy = %+v", got)
		}
	})

	t.Run("columns are read by name", func(t *testing.T) {
		t.Parallel()

		src := frontmatter + marked("last-verified", "## Last Verified",
			"| Verified by | Commit | PR | Date |\n| --- | --- | --- | --- |\n| @a | abcdef0 | #2 | 2026-01-01 |")

		got := parse(t, []byte(src)).LastVerified
		if got == nil || got.Date != "2026-01-01" || got.PR != "#2" || got.Commit != "abcdef0" {
			t.Errorf("LastVerified = %+v", got)
		}
	})
}

// TestParse_RunbookZeroOne is Phase 4's success criterion over the real
// document: the procedures, scenarios, commands, and verification of the
// runbook this repository releases with.
func TestParse_RunbookZeroOne(t *testing.T) {
	t.Parallel()

	doc := parse(t, readFixture(t, "runbook-0001.md"))

	v := doc.LastVerified
	if v == nil || v.Date != "2026-09-25" || v.PR != "#137" || v.Commit != "e41203e" {
		t.Fatalf("LastVerified = %+v, want 2026-09-25 / #137 / e41203e", v)
	}

	if len(doc.Procedures) != 3 || len(doc.Scenarios) != 2 {
		t.Errorf("procedures=%d scenarios=%d, want 3 and 2", len(doc.Procedures), len(doc.Scenarios))
	}

	commands := 0
	for _, s := range doc.Steps() {
		commands += len(s.Commands)
	}

	if commands == 0 {
		t.Error("no step carries a command")
	}
}

func TestParse_Failures(t *testing.T) {
	t.Parallel()

	t.Run("no frontmatter", func(t *testing.T) {
		t.Parallel()

		if _, err := runbook.Parse([]byte("# no frontmatter\n")); !errors.Is(err, document.ErrNoFrontmatter) {
			t.Errorf("err = %v, want ErrNoFrontmatter", err)
		}
	})

	t.Run("CR line endings", func(t *testing.T) {
		t.Parallel()

		_, err := runbook.Parse([]byte("---\r\nid: X\r\n---\r\n"))
		if !errors.Is(err, document.ErrUnsupportedLineEndings) {
			t.Errorf("err = %v, want ErrUnsupportedLineEndings", err)
		}
	})

	t.Run("two procedures claim one token", func(t *testing.T) {
		t.Parallel()

		_, err := runbook.Parse([]byte(frontmatter + procedure("1", "1. a", "") + procedure("1", "1. b", "")))

		var dup *runbook.DuplicateProcedureError
		if !errors.As(err, &dup) || dup.Token != "1" || len(dup.Lines) != 2 {
			t.Errorf("err = %v, want a DuplicateProcedureError for 1", err)
		}
	})

	t.Run("a runbook with nothing filled in parses", func(t *testing.T) {
		t.Parallel()

		doc := parse(t, []byte(frontmatter+"# t\n"))
		if len(doc.Procedures) != 0 || doc.LastVerified != nil {
			t.Errorf("doc = %+v", doc)
		}
	})
}
