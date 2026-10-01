package validate_test

import (
	"slices"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/validate"
)

// marked wraps a body in a kind's markers, heading first.
func marked(kind, heading, body string) string {
	return "<!--docz:" + kind + ":start-->\n" + heading + "\n\n" + body +
		"\n<!--docz:" + kind + ":end-->\n"
}

// TestRunbookKinds covers the nine kinds DESIGN-0019 §5 adds: one well-formed
// and one malformed case for each kind with a content rule, and a clean case
// for each kind that has none.
func TestRunbookKinds(t *testing.T) {
	t.Parallel()

	const lastVerified = "## Last Verified"

	steps := func(body string) string { return marked("steps", "#### Steps", body) }

	tests := []struct {
		name     string
		body     string
		wantCode string // "" means no finding at all
		wantSev  validate.Severity
	}{
		// last-verified.
		{
			name: "last-verified: the template's table is clean",
			body: marked("last-verified", lastVerified,
				"| Date | PR | Commit | Verified by |\n| ---- | -- | ------ | ----------- |\n|  |  |  |  |"),
		},
		{
			name: "last-verified: columns in another order are clean",
			body: marked("last-verified", lastVerified,
				"| Verified by | Commit | PR | Date |\n| --- | --- | --- | --- |\n| a | b | c | d |"),
		},
		{
			name: "last-verified: a missing column",
			body: marked("last-verified", lastVerified,
				"| Date | PR | Commit |\n| ---- | -- | ------ |\n| a | b | c |"),
			wantCode: "content.table-columns",
			wantSev:  validate.Warning,
		},
		{
			name:     "last-verified: prose where the table belongs",
			body:     marked("last-verified", lastVerified, "Ran it last week."),
			wantCode: "content.no-table",
			wantSev:  validate.Warning,
		},

		// escalation.
		{
			name: "escalation: the three columns are clean",
			body: marked("escalation", "## Escalation",
				"| Who | When | How |\n| --- | ---- | --- |\n| a | b | c |"),
		},
		{
			name: "escalation: a missing column",
			body: marked("escalation", "## Escalation",
				"| Who | When |\n| --- | ---- |\n| a | b |"),
			wantCode: "content.table-columns",
			wantSev:  validate.Warning,
		},

		// The list kinds.
		{
			name: "when: bullets are clean",
			body: marked("when", "## When to Use", "- the DoczAPIDown alert fires"),
		},
		{
			name:     "when: prose where the bullets belong",
			body:     marked("when", "## When to Use", "Whenever something breaks."),
			wantCode: "content.not-bullets",
			wantSev:  validate.Warning,
		},
		{
			name:     "prerequisites: prose where the bullets belong",
			body:     marked("prerequisites", "## Prerequisites", "You need access."),
			wantCode: "content.not-bullets",
			wantSev:  validate.Warning,
		},
		{
			name:     "verification: prose where the bullets belong",
			body:     marked("verification", "#### Verification", "It works."),
			wantCode: "content.not-bullets",
			wantSev:  validate.Warning,
		},

		// The kinds with no content rule.
		{
			name: "procedure: prose is clean",
			body: marked("procedure", "### Procedure 1: Rotate", "Rotate the key."),
		},
		{
			name: "scenario: prose is clean",
			body: marked("scenario", "### Scenario: 401s", "**Alert:** none"),
		},
		{
			name: "rollback: prose is an answer",
			body: marked("rollback", "#### Rollback", "Not applicable."),
		},

		// steps.
		{
			name: "steps: a numbered list is clean",
			body: steps("1. one\n2. two"),
		},
		{
			name:     "steps: a bullet list",
			body:     steps("- one\n- two"),
			wantCode: "steps.not-ordered",
			wantSev:  validate.Error,
		},
		{
			name: "steps: bullets beside numbered steps are notes",
			body: steps("1. one\n   - a note\n2. two"),
		},
		{
			name: "steps: nested numbered sub-steps are clean",
			body: steps("1. one\n   1. sub\n   2. sub\n2. two"),
		},
		{
			name: "steps: an empty region is incomplete, not malformed",
			body: steps(""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := validate.Document([]byte(goodFrontmatter+tt.body), validate.Options{})

			if tt.wantCode == "" {
				if len(got) > 0 {
					t.Errorf("expected no findings, got %v", got)
				}

				return
			}

			i := slices.IndexFunc(got, func(f validate.Finding) bool { return f.Code == tt.wantCode })
			if i < 0 {
				t.Fatalf("no %s finding in %v", tt.wantCode, got)
			}

			if got[i].Severity != tt.wantSev {
				t.Errorf("%s severity = %s, want %s", tt.wantCode, got[i].Severity, tt.wantSev)
			}
		})
	}
}

// One steps region per procedure and one per scenario is the template's
// shape; two in one procedure is ambiguous about which list is the
// procedure.
func TestRunbookKinds_StepsAreSingletonPerParent(t *testing.T) {
	t.Parallel()

	steps := marked("steps", "#### Steps", "1. one")

	procedure := func(n, body string) string {
		return marked("procedure", "### Procedure "+n+": x", body)
	}

	scenario := marked("scenario", "### Scenario: x", steps)

	clean := procedure("1", steps) + procedure("2", steps) + scenario
	if got := codesOf(validate.Document([]byte(goodFrontmatter+clean), validate.Options{})); len(got) > 0 {
		t.Errorf("one steps region per parent reported %v", got)
	}

	doubled := procedure("1", steps+steps)

	got := codesOf(validate.Document([]byte(goodFrontmatter+doubled), validate.Options{}))
	if !slices.Contains(got, "region.duplicate-singleton") {
		t.Errorf("two steps regions in one procedure reported %v, want region.duplicate-singleton", got)
	}
}

func codesOf(findings []validate.Finding) []string {
	out := make([]string, 0, len(findings))
	for _, f := range findings {
		out = append(out, f.Code)
	}

	return out
}
