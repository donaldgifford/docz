package adr_test

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/adr"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
)

var update = flag.Bool("update", false, "regenerate the migrated fixtures and golden fact files")

// TestMain writes the migrated fixtures before any test runs.
//
// Not inside TestGoldenCorpus, which is parallel: the invariant and pair tests
// read the same files, and a -update run would race them against the writer.
// Regenerating up front means one place produces them and every test sees them
// finished.
func TestMain(m *testing.M) {
	flag.Parse()

	if *update {
		if err := regenerate(); err != nil {
			fmt.Fprintln(os.Stderr, "regenerating fixtures:", err)
			os.Exit(1)
		}
	}

	os.Exit(m.Run())
}

// regenerate writes a migrated sibling for every snapshot.
func regenerate() error {
	paths, err := filepath.Glob(filepath.Join("testdata", "*"+origSuffix))
	if err != nil {
		return err
	}

	for _, path := range paths {
		orig, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		marked, err := insertRegions(orig)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}

		target := strings.TrimSuffix(path, origSuffix) + docSuffix
		if err := os.WriteFile(target, marked, 0o644); err != nil {
			return err
		}
	}

	return nil
}

// The corpus under testdata/ is two files per document: <name>.orig.md is a
// verbatim snapshot of a real ADR, and <name>.md is the same document with
// canonical region markers added and nothing else changed.
//
// Snapshots rather than reads from docs/: a fixture that followed the repo's
// own documents would change its own expectations every time someone edited an
// ADR, and these exist to pin the grammar against documents as they were
// actually written — three repos' worth, by three sets of habits.
const (
	origSuffix = ".orig.md"
	docSuffix  = ".md"
)

// corpus returns the fixture base names, sorted.
func corpus(t *testing.T) []string {
	t.Helper()

	paths, err := filepath.Glob(filepath.Join("testdata", "*"+origSuffix))
	if err != nil {
		t.Fatalf("globbing fixtures: %v", err)
	}

	if len(paths) == 0 {
		t.Fatal("no fixtures under testdata/")
	}

	out := make([]string, 0, len(paths))
	for _, path := range paths {
		out = append(out, strings.TrimSuffix(filepath.Base(path), origSuffix))
	}

	sort.Strings(out)

	return out
}

// TestGoldenCorpus pins two things per fixture: the markers a migration adds,
// and the facts Parse reads out of the migrated document.
//
// The migrated copies are generated rather than hand-edited. They are the
// spans inference already found, written out as markers and nothing else —
// which makes them the expected output of Phase 3's InsertRegions, and means a
// change to inference shows up here as a diff instead of as a silent
// disagreement with 2,000 lines somebody typed.
func TestGoldenCorpus(t *testing.T) {
	t.Parallel()

	for _, name := range corpus(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			marked, err := insertRegions(readFixture(t, name+origSuffix))
			if err != nil {
				t.Fatalf("migrating: %v", err)
			}

			compare(t, filepath.Join("testdata", name+docSuffix), marked)

			parsed, err := adr.Parse(marked)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}

			compare(t, filepath.Join("testdata", name+".golden.txt"),
				[]byte(describe(&parsed)))
		})
	}
}

// compare checks a generated artifact against the file on disk, writing it
// instead when -update is set.
func compare(t *testing.T, path string, got []byte) {
	t.Helper()

	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("writing %s: %v", path, err)
		}

		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s (run go test -update): %v", path, err)
	}

	if !bytes.Equal(got, want) {
		t.Errorf("%s is stale; run go test ./pkg/adr/... -update\n%s",
			path, firstDifference(string(got), string(want)))
	}
}

// firstDifference reports the first line the two differ on, which is enough to
// see what moved without printing a 500-line document.
func firstDifference(got, want string) string {
	g := strings.Split(got, "\n")
	w := strings.Split(want, "\n")

	for i := range max(len(g), len(w)) {
		var gl, wl string

		if i < len(g) {
			gl = g[i]
		}

		if i < len(w) {
			wl = w[i]
		}

		if gl != wl {
			return fmt.Sprintf("line %d:\n  got  %q\n  want %q", i+1, gl, wl)
		}
	}

	return "(files differ only in length)"
}

// insertRegions writes the markers for every span inference finds, and
// nothing else.
//
// Nothing else is the whole rule. No heading is added, no prose is moved, no
// blank line is inserted: booty-adr-0002 writes its alternatives as bold
// paragraphs with no bullet in sight, and the migration leaves it that way, so
// the migrated copy reads exactly as the original does. A migration that
// "fixed" the document would make the pair test meaningless.
func insertRegions(doc []byte) ([]byte, error) {
	regions := kinds.InferRegions(doc, adr.Headings())
	if len(regions) == 0 {
		return nil, errors.New("no regions inferred: the fixture would migrate to itself")
	}

	lines := strings.Split(strings.TrimSuffix(string(doc), "\n"), "\n")

	// before[n] is emitted above line n, after[n] below it. Closers of nested
	// regions come first and openers of outer ones come first, so a stack of
	// markers reads outside-in on the way down and inside-out on the way up.
	// An ADR needs that: its consequences region ends on the same line its
	// neutral list does, so the two closers land together and the inner one
	// has to be written first.
	before := make(map[int][]string, len(regions))
	after := make(map[int][]string, len(regions))

	sorted := make([]docparse.Region, len(regions))
	copy(sorted, regions)

	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Depth < sorted[j].Depth })

	for _, r := range sorted {
		open := fmt.Sprintf("<!--docz:%s:start-->", r.Kind)
		shut := fmt.Sprintf("<!--docz:%s:end-->", r.Kind)

		before[r.Start+1] = append(before[r.Start+1], open)
		after[r.End-1] = append([]string{shut}, after[r.End-1]...)
	}

	out := make([]string, 0, len(lines)+2*len(regions))

	for n := 1; n <= len(lines); n++ {
		out = append(out, before[n]...)
		out = append(out, lines[n-1])
		out = append(out, after[n]...)
	}

	return []byte(strings.Join(out, "\n") + "\n"), nil
}

// describe renders a Doc as the fact file a golden pins.
//
// Facts, not a Go dump: a reader of the golden is checking whether the parser
// read the document the way a person does, and %+v of a nested struct with
// three consequence lists in it is not something anyone reads.
//
// The three prose fields are excerpted because a fact file that reproduced
// docz ADR-0002's whole decision — 190 lines of numbered rules, tables, and
// mermaid — would not be one anybody reviews. Every list item is verbatim,
// because folding a wrapped bullet is exactly what these documents test.
func describe(d *adr.Doc) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "id=%q title=%q status=%q inferred=%v\n", d.ID, d.Title, d.Status, d.Inferred)
	fmt.Fprintf(&sb, "author=%q created=%q\n", d.Author, d.Created)
	fmt.Fprintf(&sb, "summary=%q\n", excerpt(d.Summary))
	fmt.Fprintf(&sb, "context=%q\n", excerpt(d.Context))
	fmt.Fprintf(&sb, "decision=%q\n", excerpt(d.Decision))
	fmt.Fprintf(&sb, "consequences positive=%d negative=%d neutral=%d\n",
		len(d.Consequences.Positive), len(d.Consequences.Negative), len(d.Consequences.Neutral))
	fmt.Fprintf(&sb, "alternatives=%d open-questions=%d references=%d\n",
		len(d.Alternatives), len(d.OpenQuestions), len(d.References))

	for _, list := range consequenceLists(d) {
		for _, item := range list.items {
			fmt.Fprintf(&sb, "\n%s line=%d\n  text=%q\n", list.label, item.Line, item.Text)
		}
	}

	for _, alt := range d.Alternatives {
		fmt.Fprintf(&sb, "\nalternative line=%d label=%q title=%q\n  text=%q\n",
			alt.Line, alt.Label, alt.Title, excerpt(alt.Text))
	}

	for _, q := range d.OpenQuestions {
		fmt.Fprintf(&sb, "\nquestion %d line=%d title=%q\n  options=%d %v resolved=%s\n",
			q.Number, q.Line, q.Title, len(q.Options), letters(q.Options), resolutionOf(q.Resolved))
	}

	for _, ref := range d.References {
		fmt.Fprintf(&sb, "\nreference line=%d url=%q\n  text=%q\n", ref.Line, ref.URL, ref.Text)
	}

	return sb.String()
}

// consequenceList is one of the three lists with the name a reader knows it
// by, so describe and the invariants walk them the same way.
type consequenceList struct {
	label string
	items []kinds.Item
}

func consequenceLists(d *adr.Doc) []consequenceList {
	return []consequenceList{
		{labelPositive, d.Consequences.Positive},
		{labelNegative, d.Consequences.Negative},
		{labelNeutral, d.Consequences.Neutral},
	}
}

// The three consequence kinds, which are also their region kinds: the
// heading table names the region "positive" and the field holds the positive
// consequences, so one spelling serves both.
const (
	labelPositive = "positive"
	labelNegative = "negative"
	labelNeutral  = "neutral"
)

// letters is an open question's option letters, which is what makes a dropped
// option visible in a golden: a count alone cannot show that the recommended
// "a" is the one missing.
func letters(options []kinds.Option) []string {
	out := make([]string, 0, len(options))
	for _, o := range options {
		letter := o.Letter
		if o.Recommended {
			letter += "*"
		}

		out = append(out, letter)
	}

	return out
}

// resolutionOf renders a resolution compactly, or "-" for an open question.
func resolutionOf(r *kinds.Resolution) string {
	if r == nil {
		return "-"
	}

	return fmt.Sprintf("{line=%d date=%q choice=%q note=%q}",
		r.Line, r.Date, r.Choice, excerpt(r.Note))
}

// excerpt keeps a golden readable.
func excerpt(s string) string {
	const limit = 90

	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= limit {
		return s
	}

	return s[:limit] + "…"
}

// eachCopy runs fn over both copies of every fixture.
//
// Both, because the two ways into a Doc are different code — spans inferred
// from headings, and spans read from markers — and an invariant that held for
// one and not the other is a bug no golden can see.
func eachCopy(t *testing.T, fn func(t *testing.T, doc []byte, parsed *adr.Doc)) {
	t.Helper()

	for _, name := range corpus(t) {
		for _, suffix := range []string{origSuffix, docSuffix} {
			t.Run(name+suffix, func(t *testing.T) {
				t.Parallel()

				doc := readFixture(t, name+suffix)
				parsed := parse(t, doc)

				fn(t, doc, &parsed)
			})
		}
	}
}

// TestCorpusLinesAreDocumentLines is the first invariant: every Line a Doc
// reports is a line docparse reports the same thing on.
//
// This is what makes a Doc safe to act on — the line an editor jumps to, the
// line a finding names, the line a future writer splices at. The three
// consequence lists are the load-bearing rows: they come from regions nested
// one level inside consequences, which is where a depth-1 offset error would
// hide, and a line one off still looks like a line.
func TestCorpusLinesAreDocumentLines(t *testing.T) {
	t.Parallel()

	eachCopy(t, checkLines)
}

func checkLines(t *testing.T, doc []byte, parsed *adr.Doc) {
	t.Helper()

	bullets := make(map[int]bool)
	for _, item := range docparse.ListItems(doc) {
		bullets[item.Line] = true
	}

	levels := make(map[int]int)
	for _, h := range docparse.Headings(doc) {
		levels[h.Line] = h.Level
	}

	for _, list := range consequenceLists(parsed) {
		for _, item := range list.items {
			if !bullets[item.Line] {
				t.Errorf("%s consequence at line %d is not a bullet docparse reports: %q",
					list.label, item.Line, item.Text)
			}
		}
	}

	for _, ref := range parsed.References {
		if !bullets[ref.Line] {
			t.Errorf("reference at line %d is not a bullet docparse reports: %q", ref.Line, ref.Text)
		}
	}

	// An alternative is written either way, and kinds.Alternatives reads
	// whichever shape the document uses: this repo's ADRs use lettered
	// bullets, and a document that gives each alternative a level-3 heading
	// reports the heading's line instead.
	for _, alt := range parsed.Alternatives {
		if !bullets[alt.Line] && levels[alt.Line] != 3 {
			t.Errorf("alternative %q at line %d is neither a bullet nor a level-3 heading",
				alt.Title, alt.Line)
		}
	}

	for _, q := range parsed.OpenQuestions {
		if levels[q.Line] != 3 {
			t.Errorf("question %d at line %d is not a level-3 heading (level %d)",
				q.Number, q.Line, levels[q.Line])
		}

		// Not asked for, but the same class of error: an option's line is what
		// a consumer that offers a choice has to point at.
		for _, o := range q.Options {
			if !bullets[o.Line] {
				t.Errorf("question %d option %q at line %d is not a bullet docparse reports",
					q.Number, o.Letter, o.Line)
			}
		}
	}
}

// regionField is one field adr reads out of one region kind, with a probe that
// says — independently of the package under test — whether the region holds
// content of the shape the field reads.
//
// The probe is deliberately written over docparse rather than over the kinds
// readers Parse itself calls. A probe that asked kinds.Items whether there
// were items would agree with Parse by construction and prove nothing.
type regionField struct {
	kind  string
	name  string
	zero  func(*adr.Doc) bool
	holds func(body []byte) bool
}

// The nine fields Parse fills, each from one region kind.
//
// Two kinds adr resolves are absent from this table on purpose. "consequences"
// is a container: it carries no field of its own, which is exactly why an ADR
// whose prose sits directly under "## Consequences" earns
// adr.consequences.empty. "decisions" has no field at all — the heading table
// names the kind so a document that grew the section is not mis-nested, but
// Doc has nowhere to put a decisions table, so docz ADR-0001's resolution
// table is read by nothing.
func regionFields() []regionField {
	return []regionField{
		{"summary", "Summary", func(d *adr.Doc) bool { return d.Summary == "" }, hasProse},
		{"context", "Context", func(d *adr.Doc) bool { return d.Context == "" }, hasProse},
		{"decision", "Decision", func(d *adr.Doc) bool { return d.Decision == "" }, hasProse},
		{
			labelPositive, "Consequences.Positive",
			func(d *adr.Doc) bool { return len(d.Consequences.Positive) == 0 }, hasBullets,
		},
		{
			labelNegative, "Consequences.Negative",
			func(d *adr.Doc) bool { return len(d.Consequences.Negative) == 0 }, hasBullets,
		},
		{
			labelNeutral, "Consequences.Neutral",
			func(d *adr.Doc) bool { return len(d.Consequences.Neutral) == 0 }, hasBullets,
		},
		{
			"alternatives", "Alternatives",
			func(d *adr.Doc) bool { return len(d.Alternatives) == 0 },
			func(body []byte) bool { return hasBullets(body) || countSubheadings(body) > 0 },
		},
		{
			"open-questions", "OpenQuestions",
			func(d *adr.Doc) bool { return len(d.OpenQuestions) == 0 }, hasNumberedSubheadings,
		},
		{
			"references", "References",
			func(d *adr.Doc) bool { return len(d.References) == 0 }, hasBullets,
		},
	}
}

// TestCorpusFieldsFollowTheirRegions is the second invariant: a field is zero
// if and only if its region is absent or holds nothing the field reads.
//
// It is the assertion that catches a mis-wired switch arm — a field reading the
// wrong region's bytes, or a kind the parser silently ignores — which a golden
// would happily pin.
func TestCorpusFieldsFollowTheirRegions(t *testing.T) {
	t.Parallel()

	eachCopy(t, checkFields)
}

func checkFields(t *testing.T, doc []byte, parsed *adr.Doc) {
	t.Helper()

	regions, _ := kinds.ResolveRegions(doc, adr.Headings())

	for _, f := range regionFields() {
		r, ok := regionOf(regions, f.kind)

		switch {
		case !ok:
			if !f.zero(parsed) {
				t.Errorf("%s is set, but the document carries no %s region at all", f.name, f.kind)
			}
		case f.holds(kinds.RegionBytes(doc, r)):
			if f.zero(parsed) {
				t.Errorf("%s is zero, but its %s region at line %d holds content the field reads",
					f.name, f.kind, r.Start)
			}
		default:
			// A present-but-empty region legitimately gives a zero field: the
			// section exists and says nothing the field can hold.
			// booty-adr-0001 and booty-adr-0002 are the real case — their
			// alternatives are bold paragraphs with no bullet and no heading,
			// so Alternatives is nil and the four alternatives each names are
			// not in the model.
			if !f.zero(parsed) {
				t.Errorf("%s is set, but its %s region at line %d holds nothing of that shape",
					f.name, f.kind, r.Start)
			}

			t.Logf("%s is zero: the %s region at line %d exists but holds nothing the field reads",
				f.name, f.kind, r.Start)
		}
	}
}

func regionOf(regions []docparse.Region, kind string) (docparse.Region, bool) {
	for _, r := range regions {
		if r.Kind == kind && r.Closed {
			return r, true
		}
	}

	return docparse.Region{}, false
}

// hasProse reports whether a region holds anything but its heading and the
// template's guidance comments.
//
// The comment walk is a local copy of the rule kinds applies, for the same
// reason the probes are written over docparse: a test that asked the code under
// test what it stripped could not disagree with it.
func hasProse(body []byte) bool {
	lines := strings.Split(string(body), "\n")
	if len(lines) <= 1 {
		return false
	}

	rest := strings.Join(lines[1:], "\n")

	for {
		open := strings.Index(rest, "<!--")
		if open < 0 {
			break
		}

		shut := strings.Index(rest[open:], "-->")
		if shut < 0 {
			rest = rest[:open]

			break
		}

		rest = rest[:open] + rest[open+shut+len("-->"):]
	}

	return strings.TrimSpace(rest) != ""
}

// hasBullets reports whether a region holds a top-level list item. Nested
// bullets do not count: every reader that means "the bullets in this region"
// means the top-level ones.
func hasBullets(body []byte) bool {
	for _, item := range docparse.ListItems(body) {
		if item.Indent == 0 && item.Text != "" {
			return true
		}
	}

	return false
}

// countSubheadings counts the level-3 headings in a region, which is the shape
// an alternative takes when it is written as a section rather than a bullet.
func countSubheadings(body []byte) int {
	n := 0

	for _, h := range docparse.Headings(body) {
		if h.Level == 3 {
			n++
		}
	}

	return n
}

// numberedHeading matches the numbering an open question's heading carries.
var numberedHeading = regexp.MustCompile(`^\d{1,3}[.)]`)

// hasNumberedSubheadings reports whether a region holds a numbered level-3
// heading: an unnumbered one is not a question, which is why an Open Questions
// section of plain prose reads as no questions rather than as one.
func hasNumberedSubheadings(body []byte) bool {
	for _, h := range docparse.Headings(body) {
		if h.Level == 3 && numberedHeading.MatchString(h.Text) {
			return true
		}
	}

	return false
}

// TestCorpusValidateFindingsAreDerivable is the third invariant: every finding
// Validate reports on a real document has a condition this test can re-derive
// from the parsed Doc alone.
//
// A finding whose condition cannot be re-derived is a bug in the rule, not a
// gap in the test: the point of a code is that a consumer can act on it, and a
// consumer only can if the code means what it says.
func TestCorpusValidateFindingsAreDerivable(t *testing.T) {
	t.Parallel()

	eachCopy(t, checkFindings)
}

func checkFindings(t *testing.T, doc []byte, parsed *adr.Doc) {
	t.Helper()

	for _, f := range adr.Validate(doc) {
		switch f.Code {
		case adr.CodeParse:
			// Unreachable here by construction: eachCopy parsed the same bytes.
			t.Errorf("%s on a document that parsed: %q", f.Code, f.Detail)
		case adr.CodeDecisionEmpty:
			if !strings.EqualFold(string(parsed.Status), "accepted") || parsed.Decision != "" {
				t.Errorf("%s on a document with status %q and a %d-byte decision",
					f.Code, parsed.Status, len(parsed.Decision))
			}
		case adr.CodeConsequencesEmpty:
			for _, list := range consequenceLists(parsed) {
				if len(list.items) > 0 {
					t.Errorf("%s on a document with %d %s consequences",
						f.Code, len(list.items), list.label)
				}
			}
		case adr.CodeSupersededNoReference:
			if !strings.EqualFold(string(parsed.Status), "superseded") {
				t.Errorf("%s on a document with status %q", f.Code, parsed.Status)
			}

			if where := forwardPointerIn(parsed); where != "" {
				t.Errorf("%s on a document whose %s names another ADR", f.Code, where)
			}
		default:
			t.Errorf("finding %q has no condition this test can re-derive: %+v", f.Code, f)
		}
	}
}

// adrID matches a reference to an ADR by id, the way the rule does.
var adrID = regexp.MustCompile(`(?i)\bADR-\d+\b`)

// forwardPointerIn names where the document points at an ADR other than
// itself, or "" when it points nowhere.
//
// Re-derived rather than borrowed: the rule counts the summary, the context,
// and the reference list, because the corpus records a supersession in prose at
// least as often as in the list.
func forwardPointerIn(d *adr.Doc) string {
	if namesOther(d.Summary, d.ID) {
		return "summary"
	}

	if namesOther(d.Context, d.ID) {
		return "context"
	}

	for _, ref := range d.References {
		if namesOther(ref.Text, d.ID) || namesOther(ref.URL, d.ID) {
			return "reference list"
		}
	}

	return ""
}

func namesOther(s, own string) bool {
	for _, id := range adrID.FindAllString(s, -1) {
		if !strings.EqualFold(id, own) {
			return true
		}
	}

	return false
}

// TestCorpusValidateCodeCoverage records which of Validate's codes the real
// corpus exercises and which it does not.
//
// validate_test.go already has a passing and a failing document per code,
// against the synthetic grammar fixture. This is the other half: it says what
// eight real ADRs, written by three sets of habits, actually trip. The answer
// is nothing — every fixture validates clean — and each gap below is a
// condition no ADR in the fleet has ever been in. No fixture is invented to
// close one: a fixture written to trip a rule would only prove the rule fires
// on a fixture written to trip it, which validate_test.go already proves.
func TestCorpusValidateCodeCoverage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		code      string
		exercised bool
		gap       string
	}{
		{
			code: adr.CodeParse,
			gap:  "every fixture has frontmatter and LF endings; a rejected document is not an ADR corpus",
		},
		{
			code: adr.CodeDecisionEmpty,
			gap:  "the four accepted fixtures all record their decision; the proposed ones are exempt by the rule",
		},
		{
			code: adr.CodeConsequencesEmpty,
			gap:  "all eight fill positive, negative, and neutral — the template's three headings are load-bearing habit",
		},
		{
			code: adr.CodeSupersededNoReference,
			gap:  "no ADR in the fleet carries the Superseded status; docz ADR-0001 is superseded in part and stays Accepted",
		},
	}

	seen := make(map[string]bool, len(tests))

	for _, name := range corpus(t) {
		for _, suffix := range []string{origSuffix, docSuffix} {
			for _, f := range adr.Validate(readFixture(t, name+suffix)) {
				seen[f.Code] = true

				t.Logf("%s%s: %s at line %d — %s", name, suffix, f.Code, f.Line, f.Detail)
			}
		}
	}

	known := make(map[string]bool, len(tests))

	for _, tt := range tests {
		known[tt.code] = true

		if seen[tt.code] == tt.exercised {
			continue
		}

		if tt.exercised {
			t.Errorf("%s is no longer exercised by the corpus", tt.code)

			continue
		}

		t.Errorf("%s now fires on the corpus, which recorded it as absent: %s", tt.code, tt.gap)
	}

	for code := range seen {
		if !known[code] {
			t.Errorf("Validate reports %q, which this test does not account for", code)
		}
	}
}

// TestCorpusMigrationChangesNothingButMarkers is the load-bearing proof of
// DESIGN-0015 §6 over the real corpus: a document read by its headings and the
// same document read by its markers are the same document.
//
// Line numbers are excluded, because markers are lines and adding them moves
// everything below. Everything else — every consequence, alternative, option,
// resolution, reference, and prose field — has to match exactly.
func TestCorpusMigrationChangesNothingButMarkers(t *testing.T) {
	t.Parallel()

	for _, name := range corpus(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			orig := parse(t, readFixture(t, name+origSuffix))
			marked := parse(t, readFixture(t, name+docSuffix))

			if !orig.Inferred {
				t.Error("the original is not inferred: does it already carry markers?")
			}

			if marked.Inferred {
				t.Error("the migrated copy is inferred: are its markers being read?")
			}

			if got, want := withoutLines(&orig), withoutLines(&marked); got != want {
				t.Errorf("the two read differently\n%s", firstDifference(got, want))
			}
		})
	}
}

// withoutLines renders a Doc's facts with every line number dropped.
func withoutLines(d *adr.Doc) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "id=%q title=%q status=%q\n", d.ID, d.Title, d.Status)
	fmt.Fprintf(&sb, "author=%q created=%q\n", d.Author, d.Created)
	fmt.Fprintf(&sb, "summary=%q\n", d.Summary)
	fmt.Fprintf(&sb, "context=%q\n", d.Context)
	fmt.Fprintf(&sb, "decision=%q\n", d.Decision)

	for _, list := range consequenceLists(d) {
		for _, item := range list.items {
			fmt.Fprintf(&sb, "%s %q\n", list.label, item.Text)
		}
	}

	for _, alt := range d.Alternatives {
		fmt.Fprintf(&sb, "alternative %q %q\n  %q\n", alt.Label, alt.Title, alt.Text)
	}

	for _, q := range d.OpenQuestions {
		fmt.Fprintf(&sb, "question %d %q resolved=%s\n", q.Number, q.Title, noteOf(q.Resolved))

		for _, o := range q.Options {
			fmt.Fprintf(&sb, "  option %q recommended=%v %q\n", o.Letter, o.Recommended, o.Text)
		}
	}

	for _, ref := range d.References {
		fmt.Fprintf(&sb, "reference %q %q\n", ref.Text, ref.URL)
	}

	return sb.String()
}

func noteOf(r *kinds.Resolution) string {
	if r == nil {
		return "-"
	}

	return fmt.Sprintf("{%q %q %q}", r.Date, r.Choice, r.Note)
}
