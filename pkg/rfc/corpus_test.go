package rfc_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
	"github.com/donaldgifford/docz/v2/pkg/rfc"
)

// The corpus properties run over both copies of every document. A golden says
// what one document parsed to; these say what any document must — and running
// them over the snapshot as well as the migrated sibling is what keeps a
// property from holding only for the path markers take.

// copies names both copies of every fixture: the snapshot and the migrated
// sibling. Each property loops over it and opens its own subtest, rather than
// taking a callback, so a failure's line number lands on the assertion that
// failed instead of on a shared walker.
func copies(t *testing.T) []string {
	t.Helper()

	names := corpus(t)

	out := make([]string, 0, 2*len(names))
	for _, name := range names {
		out = append(out, name+origSuffix, name+docSuffix)
	}

	return out
}

// regionOf returns the first closed region of a kind.
func regionOf(regions []docparse.Region, kind string) (docparse.Region, bool) {
	for _, r := range regions {
		if r.Kind == kind && r.Closed {
			return r, true
		}
	}

	return docparse.Region{}, false
}

// TestCorpusEveryLineIsALineDocparseReports is the invariant that makes a Doc
// safe to act on: every Line is an address the facts layer agrees points at
// the thing the field describes.
//
// A Line that merely fell inside the document would still send an editor, a
// splice, or a reviewer to the wrong place, so each kind is checked against
// the docparse reader that owns its shape — list items for the bullet-shaped
// kinds, headings for a question, the risks table for a row.
func TestCorpusEveryLineIsALineDocparseReports(t *testing.T) {
	t.Parallel()

	for _, name := range copies(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			doc := readFixture(t, name)
			parsed := parse(t, doc)

			items := make(map[int]bool)
			for _, it := range docparse.ListItems(doc) {
				items[it.Line] = true
			}

			heads := make(map[int]int)
			for _, h := range docparse.Headings(doc) {
				heads[h.Line] = h.Level
			}

			for _, c := range parsed.Criteria {
				if !items[c.Line] {
					t.Errorf("criterion %q at line %d is not a list item docparse reports", c.Text, c.Line)
				}
			}

			for _, ref := range parsed.References {
				if !items[ref.Line] {
					t.Errorf("reference %q at line %d is not a list item docparse reports",
						ref.Text, ref.Line)
				}
			}

			// An alternative is written either as a bullet or as a level-3
			// heading, and kinds.Alternatives reads whichever the document
			// uses, so either reader may be the one that owns the line.
			for _, alt := range parsed.Alternatives {
				if !items[alt.Line] && heads[alt.Line] == 0 {
					t.Errorf("alternative %q at line %d is neither a list item nor a heading "+
						"docparse reports", alt.Title, alt.Line)
				}
			}

			for _, q := range parsed.OpenQuestions {
				if heads[q.Line] != 3 {
					t.Errorf("open question %d at line %d is not a level-3 heading docparse "+
						"reports (level %d)", q.Number, q.Line, heads[q.Line])
				}

				for _, o := range q.Options {
					if !items[o.Line] {
						t.Errorf("option %q of question %d at line %d is not a list item "+
							"docparse reports", o.Letter, q.Number, o.Line)
					}
				}
			}

			checkRiskLines(t, doc, &parsed)
		})
	}
}

// checkRiskLines pins a risks row's Line to the first table of the risks
// region, which is the only table risks() reads.
//
// Both halves matter. Inside the table's body-row span is what makes the row
// index meaningful; a line that starts with a pipe is what makes it a row
// somebody can edit. A Line that satisfied only the first would still be the
// delimiter row or the line past the table's end.
func checkRiskLines(t *testing.T, doc []byte, parsed *rfc.Doc) {
	t.Helper()

	if len(parsed.Risks) == 0 {
		return
	}

	regions, _ := kinds.ResolveRegions(doc, rfc.Headings())

	region, ok := regionOf(regions, "risks")
	if !ok {
		t.Fatal("the document reports risks but has no risks region")
	}

	tables := docparse.Tables(kinds.RegionBytes(doc, region))
	if len(tables) == 0 {
		t.Fatal("the document reports risks but its risks region holds no table")
	}

	// The header row, then the delimiter row, then one line per body row.
	from := region.Start + tables[0].Line + 2
	through := from + len(tables[0].Rows) - 1

	lines := strings.Split(string(doc), "\n")

	for _, risk := range parsed.Risks {
		if risk.Line < from || risk.Line > through {
			t.Errorf("risk %q at line %d is outside the first table's body rows (%d-%d)",
				risk.Risk, risk.Line, from, through)

			continue
		}

		if got := strings.TrimSpace(lines[risk.Line-1]); !strings.HasPrefix(got, "|") {
			t.Errorf("risk %q at line %d is not a table row: %q", risk.Risk, risk.Line, got)
		}
	}
}

// numberedHeading matches the "1." or "1)" an open question's heading opens
// with. Written out here rather than imported, because the probe below has to
// be able to disagree with the reader it is checking.
var numberedHeading = regexp.MustCompile(`^\d{1,3}[.)]`)

// fieldProbe pairs a Doc field with an independent answer to "does its region
// hold anything?", derived from docparse rather than from the kinds reader
// under test.
type fieldProbe struct {
	// kind is the region kind, and field the name the failure message uses.
	kind  string
	field string

	// zero reports whether the field is at its zero value.
	zero func(*rfc.Doc) bool

	// content reports whether the region's bytes hold something the grammar
	// could read, judged from docparse facts.
	content func([]byte) bool

	// legitimate names the reason a present region may still hold nothing, so
	// the third case is an assertion with an explanation rather than a skip.
	legitimate string
}

// probes covers every region kind Parse has a field for. Eight, not the seven
// the RFC template ships: open-questions has no section in the template and is
// read anyway, because an RFC accumulates questions during review.
//
// The decisions kind is in rfc.Headings() and deliberately absent here: it has
// no field on Doc, and is resolved only so a Decisions section is not mistaken
// for part of the section above it.
func probes() []fieldProbe {
	return []fieldProbe{
		{
			kind: "summary", field: "Summary",
			zero:    func(d *rfc.Doc) bool { return d.Summary == "" },
			content: prose,
			legitimate: "a freshly created document's summary region holds only " +
				"the template's guidance comment",
		},
		{
			kind: "problem", field: "Problem",
			zero:    func(d *rfc.Doc) bool { return d.Problem == "" },
			content: prose,
			legitimate: "a freshly created document's problem region holds only " +
				"comments and the Supporting Data heading",
		},
		{
			kind: "proposal", field: "Proposal",
			zero:       func(d *rfc.Doc) bool { return d.Proposal == "" },
			content:    prose,
			legitimate: "a freshly created document's proposal region holds only a comment",
		},
		{
			kind: "alternatives", field: "Alternatives",
			zero:    func(d *rfc.Doc) bool { return d.Alternatives == nil },
			content: func(r []byte) bool { return hasItem(r) || hasLevel3(r) },
			legitimate: "the kind is bullets or level-3 headings, and all three booty " +
				"RFCs write their alternatives as bold-lead-in paragraphs instead",
		},
		{
			kind: "risks", field: "Risks",
			zero:    func(d *rfc.Doc) bool { return d.Risks == nil },
			content: hasFilledRow,
			legitimate: "the template's table ships one wholly empty row, which the " +
				"parser drops rather than reporting as a blank risk",
		},
		{
			kind: "criteria", field: "Criteria",
			zero:       func(d *rfc.Doc) bool { return d.Criteria == nil },
			content:    hasBullet,
			legitimate: "a freshly created document's criteria region holds only a comment",
		},
		{
			kind: "open-questions", field: "OpenQuestions",
			zero:       func(d *rfc.Doc) bool { return d.OpenQuestions == nil },
			content:    hasNumberedLevel3,
			legitimate: "a section whose level-3 headings carry no number holds no questions",
		},
		{
			kind: "references", field: "References",
			zero:       func(d *rfc.Doc) bool { return d.References == nil },
			content:    hasItem,
			legitimate: "a freshly created document's references region holds only a comment",
		},
	}
}

// TestCorpusFieldIsZeroExactlyWhenItsRegionIs is the second invariant: a field
// is zero if and only if its region is absent or holds nothing the grammar
// reads.
//
// The "holds nothing" half is judged from docparse — list items, tables,
// headings, and prose left after the HTML comments are removed — so the check
// is a re-derivation rather than a restatement of the reader it is checking. A
// field set for an absent region would mean a span leaked in from elsewhere; a
// field zero for a region full of bullets would mean the reader missed them.
func TestCorpusFieldIsZeroExactlyWhenItsRegionIs(t *testing.T) {
	t.Parallel()

	for _, name := range copies(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			doc := readFixture(t, name)
			parsed := parse(t, doc)

			regions, _ := kinds.ResolveRegions(doc, rfc.Headings())

			for _, p := range probes() {
				region, present := regionOf(regions, p.kind)
				zero := p.zero(&parsed)

				switch {
				case !present:
					if !zero {
						t.Errorf("%s is set although the document has no %s region",
							p.field, p.kind)
					}
				case p.content(kinds.RegionBytes(doc, region)):
					if zero {
						t.Errorf("%s is zero although its %s region holds content docparse reports",
							p.field, p.kind)
					}
				default:
					if !zero {
						t.Errorf("%s is set although its %s region holds nothing the grammar "+
							"reads; zero is the right answer here because %s",
							p.field, p.kind, p.legitimate)
					}
				}
			}
		})
	}
}

// prose reports whether a region holds anything but its heading, blank lines,
// and HTML comments.
//
// The comment walk is this test's own rather than kinds' — the point of a
// probe is that it can disagree with the reader it checks — and it is a walk
// rather than a pattern because a comment may span lines, as the templates'
// guidance comments do.
func prose(region []byte) bool {
	lines := strings.Split(string(region), "\n")
	if len(lines) <= 1 {
		return false
	}

	body := strings.Join(lines[1:], "\n")

	for {
		open := strings.Index(body, "<!--")
		if open < 0 {
			break
		}

		shut := strings.Index(body[open:], "-->")
		if shut < 0 {
			body = body[:open]

			break
		}

		body = body[:open] + body[open+shut+3:]
	}

	return strings.TrimSpace(body) != ""
}

// hasItem reports a top-level list item with text, bulleted or numbered. The
// readers for alternatives and references take both: an author who numbers
// their alternatives has still listed them.
func hasItem(region []byte) bool {
	for _, it := range docparse.ListItems(region) {
		if it.Indent == 0 && it.Text != "" {
			return true
		}
	}

	return false
}

// hasBullet is hasItem for a kind that reads dash bullets only. An ordered
// list in the criteria position is a procedure, not a checklist.
func hasBullet(region []byte) bool {
	for _, it := range docparse.ListItems(region) {
		if it.Indent == 0 && !it.Ordered && it.Text != "" {
			return true
		}
	}

	return false
}

// hasLevel3 reports a level-3 heading inside the region, the other shape an
// alternatives section is written in.
func hasLevel3(region []byte) bool {
	for _, h := range docparse.Headings(region) {
		if h.Level == 3 {
			return true
		}
	}

	return false
}

// hasNumberedLevel3 reports a numbered level-3 heading: an open question.
func hasNumberedLevel3(region []byte) bool {
	for _, h := range docparse.Headings(region) {
		if h.Level == 3 && numberedHeading.MatchString(h.Text) {
			return true
		}
	}

	return false
}

// hasFilledRow reports a first table with at least one cell that says
// something.
func hasFilledRow(region []byte) bool {
	tables := docparse.Tables(region)
	if len(tables) == 0 {
		return false
	}

	for _, row := range tables[0].Rows {
		for _, cell := range row {
			if strings.TrimSpace(cell) != "" {
				return true
			}
		}
	}

	return false
}

// TestCorpusValidateFindingsRederive is the third invariant: every finding the
// corpus earns has its condition independently re-derived from the Doc.
//
// A rule that fired for the wrong reason would still produce a plausible
// finding, and a reviewer reading the golden could not tell. So each code is
// checked against the state it claims — not against the code path that emitted
// it — and an unrecognised code is a failure rather than a pass, so a new rule
// cannot ship without being re-derived here.
func TestCorpusValidateFindingsRederive(t *testing.T) {
	t.Parallel()

	for _, name := range copies(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			doc := readFixture(t, name)
			parsed := parse(t, doc)

			regions, _ := kinds.ResolveRegions(doc, rfc.Headings())

			for _, f := range rfc.Validate(doc) {
				switch f.Code {
				case rfc.CodeAlternativesEmpty:
					if len(parsed.Alternatives) != 0 {
						t.Errorf("%s with %d alternatives parsed", f.Code, len(parsed.Alternatives))
					}

					if _, ok := regionOf(regions, "alternatives"); !ok {
						t.Errorf("%s for a document with no alternatives region: "+
							"an absent section is the generic tier's region.missing", f.Code)
					}

				case rfc.CodeRisksNoMitigation:
					if !unmitigatedAt(&parsed, f.Line) {
						t.Errorf("%s at line %d, where no risk has an empty mitigation",
							f.Code, f.Line)
					}

				case rfc.CodeStatusOpenQuestion:
					if !strings.EqualFold(string(parsed.Status), "accepted") {
						t.Errorf("%s for a document whose status is %q", f.Code, parsed.Status)
					}

					if !unresolvedAt(&parsed, f.Line) {
						t.Errorf("%s at line %d, where no unresolved question sits", f.Code, f.Line)
					}

				case rfc.CodeParse:
					t.Errorf("%s for a fixture Parse accepted: %s", f.Code, f.Detail)

				default:
					t.Errorf("Validate reported %q, which this test cannot re-derive", f.Code)
				}
			}
		})
	}
}

// unmitigatedAt reports a risk at line with no mitigation.
func unmitigatedAt(parsed *rfc.Doc, line int) bool {
	for _, risk := range parsed.Risks {
		if risk.Line == line && risk.Mitigation == "" {
			return true
		}
	}

	return false
}

// unresolvedAt reports an open question at line that is still open.
func unresolvedAt(parsed *rfc.Doc, line int) bool {
	for _, q := range parsed.OpenQuestions {
		if q.Line == line && q.Resolved == nil {
			return true
		}
	}

	return false
}

// codesAbsentFromCorpus records the rules no real RFC in the corpus trips, and
// why.
//
// Recorded rather than manufactured. The task of a corpus is to say what real
// documents do; inventing a fixture to reach a code would make the corpus
// agree with the rules by construction, which is the one thing it is here to
// avoid. Each of these is covered by validate_test.go against the synthetic
// grammar fixture, where an edit to a known document is the honest way to
// reach a rule.
var codesAbsentFromCorpus = map[string]string{
	rfc.CodeParse: "every fixture is a real markdown document with frontmatter and LF " +
		"endings, and Parse rejects only those two things",
	rfc.CodeRisksNoMitigation: "all 28 risks rows in the three booty RFCs fill every " +
		"column, and the rendered template's one row is wholly empty, which the parser drops",
	rfc.CodeStatusOpenQuestion: "every fixture is status: Draft, and none of them has " +
		"an Open Questions section at all",
}

// TestCorpusCodeCoverage is the fourth invariant: each code this package can
// report is either exercised by the corpus or recorded as absent from it.
//
// Both directions are checked. A code that is neither exercised nor recorded
// is a rule nobody has seen fire over a real document; a code recorded as
// absent that the corpus now trips is a stale note, and a stale note about
// coverage is worse than none.
func TestCorpusCodeCoverage(t *testing.T) {
	t.Parallel()

	seen := make(map[string]bool)

	for _, name := range copies(t) {
		for _, f := range rfc.Validate(readFixture(t, name)) {
			seen[f.Code] = true
		}
	}

	all := []string{
		rfc.CodeParse,
		rfc.CodeAlternativesEmpty,
		rfc.CodeRisksNoMitigation,
		rfc.CodeStatusOpenQuestion,
	}

	for _, code := range all {
		reason, recorded := codesAbsentFromCorpus[code]

		switch {
		case seen[code] && recorded:
			t.Errorf("%s is recorded as absent from the corpus (%s) but the corpus reports it",
				code, reason)
		case !seen[code] && !recorded:
			t.Errorf("%s is neither exercised by the corpus nor recorded in "+
				"codesAbsentFromCorpus with the reason why", code)
		}
	}
}
