package runbook_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/validate"
	"github.com/donaldgifford/docz/v2/pkg/runbook"
)

const activeFrontmatter = "---\nid: RUNBOOK-0009\ntitle: \"t\"\nstatus: Active\nauthor: a\ncreated: 2026-01-02\n---\n\n"

func overview(owner string) string {
	return marked("overview", "## Overview", "Prose.\n\n**Service:** svc\n\n**Owner:** "+owner)
}

func verified(row string) string {
	return marked("last-verified", "## Last Verified",
		"| Date | PR | Commit | Verified by |\n| --- | --- | --- | --- |\n"+row)
}

// clean is a runbook with nothing for the typed tier to say.
func clean() string {
	return activeFrontmatter + verified("| 2026-09-25 | #137 | e41203e | @a |") + overview("@a") +
		procedure("1", "1. a", "1. undo") + scenario("s", "1. b")
}

// TestValidate_EveryCode has one case per code, each the clean document
// with one thing wrong, and asserts the code appears with its severity.
func TestValidate_EveryCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		doc  string
		code string
		sev  validate.Severity
	}{
		{"parse", "no frontmatter\n", runbook.CodeParse, validate.Error},
		{
			"duplicate token", frontmatter + procedure("1", "1. a", "") + procedure("1", "1. b", ""),
			runbook.CodeDuplicateToken, validate.Error,
		},
		{
			"no title",
			frontmatter + marked("procedure", "### Procedure 1: <!-- x -->", marked("steps", "#### Steps", "1. a")),
			runbook.CodeNoTitle, validate.Warning,
		},
		{
			"no steps", frontmatter + procedure("1", "Nothing numbered.", ""),
			runbook.CodeNoSteps, validate.Error,
		},
		{"step empty", frontmatter + procedure("1", "1.\n2. b", ""), runbook.CodeStepEmpty, validate.Error},
		{
			"step unordered", frontmatter + procedure("1", "1. a\n- b", ""),
			runbook.CodeStepUnordered, validate.Warning,
		},
		{
			"no symptom", frontmatter + scenario("<!-- the symptom -->", "1. a"),
			runbook.CodeNoSymptom, validate.Warning,
		},
		{"scenario no steps", frontmatter + scenario("s", "Prose."), runbook.CodeScenarioNoSteps, validate.Error},
		{"no owner", frontmatter + overview("<!-- team -->"), runbook.CodeNoOwner, validate.Warning},
		{
			"bad date", frontmatter + verified("| 25/09/2026 | #1 | abcdef0 | @a |"),
			runbook.CodeBadDate, validate.Error,
		},
		{"bad pr", frontmatter + verified("| 2026-09-25 | 137 | abcdef0 | @a |"), runbook.CodeBadPR, validate.Warning},
		{
			"bad commit", frontmatter + verified("| 2026-09-25 | #1 | main | @a |"),
			runbook.CodeBadCommit, validate.Error,
		},
		{
			"no verifier", frontmatter + verified("| 2026-09-25 | #1 | abcdef0 |  |"),
			runbook.CodeNoVerifier, validate.Error,
		},
		{
			"extra rows",
			frontmatter + verified("| 2026-09-25 | #1 | abcdef0 | @a |\n| 2026-08-01 | #2 | abcdef1 | @b |"),
			runbook.CodeExtraRows, validate.Warning,
		},
		{
			"active and never verified", activeFrontmatter + verified("|  |  |  |  |") + procedure("1", "1. a", ""),
			runbook.CodeNotVerified, validate.Warning,
		},
		{
			"active with nothing to run", activeFrontmatter + verified("| 2026-09-25 | #1 | abcdef0 | @a |"),
			runbook.CodeStatusNoProcedure, validate.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := runbook.Validate([]byte(tt.doc))

			i := slices.IndexFunc(got, func(f validate.Finding) bool { return f.Code == tt.code })
			if i < 0 {
				t.Fatalf("no %s in %v", tt.code, got)
			}

			if got[i].Severity != tt.sev {
				t.Errorf("%s severity = %s, want %s", tt.code, got[i].Severity, tt.sev)
			}
		})
	}
}

func TestValidate_CleanDocumentHasNoFindings(t *testing.T) {
	t.Parallel()

	if got := runbook.Validate([]byte(clean())); len(got) > 0 {
		t.Errorf("findings = %v", got)
	}
}

// The shapes each cell accepts, so a well-formed variant is never a finding.
func TestValidate_AcceptedCellShapes(t *testing.T) {
	t.Parallel()

	for _, row := range []string{
		"| 2026-09-25 | https://github.com/o/r/pull/1 | `e41203e` | @a |",
		"| 2026-09-25 | [#1](https://github.com/o/r/pull/1) | " + strings.Repeat("a", 40) + " | @a, @b |",
		"| 2026-09-25 |  |  | @a |",
	} {
		got := runbook.Validate([]byte(frontmatter + verified(row)))
		if len(got) > 0 {
			t.Errorf("%s: findings = %v", row, got)
		}
	}
}

// A rejected document gets exactly one finding: every other check reads a
// parsed Doc.
func TestValidate_RejectedDocumentGetsOneFinding(t *testing.T) {
	t.Parallel()

	if got := runbook.Validate([]byte("---\r\nid: X\r\n---\r\n")); len(got) != 1 {
		t.Errorf("findings = %v, want one", got)
	}
}

// The template's own placeholders are warnings at most: a runbook somebody
// has just created is unfinished, not broken.
func TestValidate_TheRenderedTemplateHasNoErrors(t *testing.T) {
	t.Parallel()

	for _, f := range runbook.Validate(renderedTemplate(t)) {
		if f.Severity == validate.Error {
			t.Errorf("the template reports an error: %+v", f)
		}
	}
}
