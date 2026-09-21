package design_test

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

	"github.com/donaldgifford/docz/v2/pkg/design"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/validate"
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
// verbatim snapshot of a real design, and <name>.md is the same document with
// canonical region markers added and nothing else changed.
//
// Snapshots rather than reads from docs/: a fixture that followed the repo's
// own documents would change its own expectations every time someone edited a
// design, and these exist to pin the grammar against documents as they were
// actually written. docz-design-0014 is a snapshot of the document that
// specifies this package, which is deliberate — it is also the longest design
// in the fleet and the only one that resolves questions in a second
// blockquote paragraph.
const (
	origSuffix = ".orig.md"
	docSuffix  = ".md"
)

// The region kinds this test names. Spelled here rather than read from the
// package, because design's own constants are unexported and an external test
// is the outside view: a kind is part of the marker vocabulary, so a test that
// could not name one is not testing what a consumer sees.
const (
	kindGoals         = "goals"
	kindNonGoals      = "non-goals"
	kindOpenQuestions = "open-questions"
	kindDecisions     = "decisions"
	kindReferences    = "references"
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
// disagreement with 5,000 lines somebody typed.
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

			parsed, err := design.Parse(marked)
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
		t.Errorf("%s is stale; run go test ./pkg/design/... -update\n%s",
			path, firstDifference(string(got), string(want)))
	}
}

// firstDifference reports the first line the two differ on, which is enough to
// see what moved without printing a 2,000-line document.
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
// blank line is inserted: docz-design-0002 has no Detailed Design section and
// files its decisions as a numbered list rather than a table, and the
// migration leaves both alone, so the migrated copy reads exactly as the
// original does. A migration that "fixed" the document would make the pair
// test meaningless.
func insertRegions(doc []byte) ([]byte, error) {
	regions := kinds.InferRegions(doc, design.Headings())
	if len(regions) == 0 {
		return nil, errors.New("no regions inferred: the fixture would migrate to itself")
	}

	lines := strings.Split(strings.TrimSuffix(string(doc), "\n"), "\n")

	// before[n] is emitted above line n, after[n] below it. Closers of nested
	// regions come first and openers of outer ones come first, so a stack of
	// markers reads outside-in on the way down and inside-out on the way up.
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

// bodyField is one of design's string fields, named by the region kind that
// fills it. The two spellings are the same word on purpose: the table doubles
// as the map from kind to field that the zero-field invariant walks.
type bodyField struct {
	name  string
	value string
}

// bodyFields returns every string field a region fills, in template order.
func bodyFields(d *design.Doc) []bodyField {
	return []bodyField{
		{"overview", d.Overview},
		{"background", d.Background},
		{"detailed-design", d.DetailedDesign},
		{"api-changes", d.APIChanges},
		{"data-model", d.DataModel},
		{"testing", d.Testing},
		{"rollout", d.Rollout},
	}
}

// describe renders a Doc as the fact file a golden pins.
//
// Facts, not a Go dump: a reader of the golden is checking whether the parser
// read the document the way a person does, and %+v of a seventeen-field struct
// is not something anyone reads.
//
// Every string field records its length beside its excerpt. Detailed Design
// runs to 40,000 bytes in this corpus, so the excerpt alone could not tell a
// section that grew by a paragraph from one that lost half of itself to a
// misplaced span end — the length can, and it is one number.
func describe(d *design.Doc) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "id=%q title=%q status=%q\n", d.ID, d.Title, d.Status)
	fmt.Fprintf(&sb, "author=%q created=%q inferred=%v\n", d.Author, d.Created, d.Inferred)
	fmt.Fprintf(&sb, "goals=%d non-goals=%d open-questions=%d decisions=%d references=%d\n",
		len(d.Goals), len(d.NonGoals), len(d.OpenQuestions), len(d.Decisions), len(d.References))

	for _, f := range bodyFields(d) {
		fmt.Fprintf(&sb, "%s len=%d excerpt=%q\n", f.name, len(f.value), excerpt(f.value))
	}

	for _, item := range d.Goals {
		fmt.Fprintf(&sb, "\ngoal line=%d text=%q", item.Line, item.Text)
	}

	for _, item := range d.NonGoals {
		fmt.Fprintf(&sb, "\nnon-goal line=%d text=%q", item.Line, item.Text)
	}

	for _, q := range d.OpenQuestions {
		fmt.Fprintf(&sb, "\n\nquestion %d line=%d options=%d title=%q\n",
			q.Number, q.Line, len(q.Options), q.Title)
		fmt.Fprintf(&sb, "  %s", resolutionOf(q.Resolved))
	}

	for _, row := range d.Decisions {
		fmt.Fprintf(&sb, "\n\ndecision %d line=%d\n  question=%q\n  resolution=%q",
			row.Number, row.Line, excerpt(row.Question), excerpt(row.Resolution))
	}

	for _, ref := range d.References {
		fmt.Fprintf(&sb, "\n\nreference line=%d url=%q\n  text=%q",
			ref.Line, ref.URL, excerpt(ref.Text))
	}

	return strings.TrimRight(sb.String(), "\n") + "\n"
}

// resolutionOf renders a question's resolution, or says there is none. An
// unresolved question is the state design.status.open-question reads, so the
// golden says so in as many words rather than leaving a field out.
func resolutionOf(r *kinds.Resolution) string {
	if r == nil {
		return "unresolved"
	}

	return fmt.Sprintf("resolved line=%d date=%q choice=%q note=%q",
		r.Line, r.Date, r.Choice, excerpt(r.Note))
}

// excerpt keeps a golden readable. A fact file that reproduced a 2,000-line
// design's whole detailed design would not be one anybody reviews.
func excerpt(s string) string {
	const limit = 90

	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= limit {
		return s
	}

	return s[:limit] + "…"
}

// TestCorpusMigrationChangesNothingButMarkers is the load-bearing proof of
// DESIGN-0015 §6 over the real corpus: a document read by its headings and the
// same document read by its markers are the same document.
//
// Line numbers are excluded, because markers are lines and adding them moves
// everything below. Everything else — every field, item, question, option,
// resolution, row, and reference — has to match exactly.
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
//
// Inferred is dropped too, and only that: it is the one field that is
// supposed to differ between the two copies, and the caller has already
// asserted which way round.
func withoutLines(d *design.Doc) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "id=%q title=%q status=%q\n", d.ID, d.Title, d.Status)
	fmt.Fprintf(&sb, "author=%q created=%q\n", d.Author, d.Created)

	for _, f := range bodyFields(d) {
		fmt.Fprintf(&sb, "%s=%q\n", f.name, f.value)
	}

	for _, item := range d.Goals {
		fmt.Fprintf(&sb, "goal %q\n", item.Text)
	}

	for _, item := range d.NonGoals {
		fmt.Fprintf(&sb, "non-goal %q\n", item.Text)
	}

	for _, q := range d.OpenQuestions {
		fmt.Fprintf(&sb, "question %d %q\n", q.Number, q.Title)

		for _, o := range q.Options {
			fmt.Fprintf(&sb, "  option %q recommended=%v %q\n", o.Letter, o.Recommended, o.Text)
		}

		if q.Resolved != nil {
			fmt.Fprintf(&sb, "  resolved %q %q %q\n",
				q.Resolved.Date, q.Resolved.Choice, q.Resolved.Note)
		}
	}

	for _, row := range d.Decisions {
		fmt.Fprintf(&sb, "decision %d %q %q\n", row.Number, row.Question, row.Resolution)
	}

	for _, ref := range d.References {
		fmt.Fprintf(&sb, "reference %q %q\n", ref.URL, ref.Text)
	}

	return sb.String()
}

// TestCorpusLinesAreFacts is the first invariant: every Line a Doc carries is
// a line the facts layer agrees is that kind of line.
//
// This is what makes a Doc safe to act on. A consumer jumps an editor to a
// goal's line, or points a reviewer at a decisions row; a Line that named the
// blank line above the bullet would send them somewhere plausible and wrong,
// which is worse than no line at all.
func TestCorpusLinesAreFacts(t *testing.T) {
	t.Parallel()

	for _, name := range corpus(t) {
		for _, suffix := range []string{origSuffix, docSuffix} {
			t.Run(name+suffix, func(t *testing.T) {
				t.Parallel()

				doc := readFixture(t, name+suffix)
				parsed := parse(t, doc)

				checkLines(t, doc, &parsed)
			})
		}
	}
}

func checkLines(t *testing.T, doc []byte, parsed *design.Doc) {
	t.Helper()

	// The whole document, walked once. Deliberately not the region's bytes:
	// the point is that a Line means the same thing to a consumer holding the
	// file as it does to the reader that produced it.
	items := make(map[int]bool)
	for _, item := range docparse.ListItems(doc) {
		items[item.Line] = true
	}

	headings := make(map[int]bool)

	for _, h := range docparse.Headings(doc) {
		if h.Level == 3 {
			headings[h.Line] = true
		}
	}

	for i, item := range parsed.Goals {
		checkListLine(t, items, fmt.Sprintf("Goals[%d]", i), item.Line, item.Text)
	}

	for i, item := range parsed.NonGoals {
		checkListLine(t, items, fmt.Sprintf("NonGoals[%d]", i), item.Line, item.Text)
	}

	for i, ref := range parsed.References {
		checkListLine(t, items, fmt.Sprintf("References[%d]", i), ref.Line, ref.Text)
	}

	for _, q := range parsed.OpenQuestions {
		if !headings[q.Line] {
			t.Errorf("question %d is at line %d, which is not a level-3 heading docparse reports: %q",
				q.Number, q.Line, q.Title)
		}

		for _, o := range q.Options {
			checkListLine(t, items,
				fmt.Sprintf("question %d option %q", q.Number, o.Letter), o.Line, o.Text)
		}
	}

	// A decisions row is a table row, and docparse reports a table's start
	// line rather than each row's, so the fact to check is the line itself.
	lines := strings.Split(string(doc), "\n")

	for _, row := range parsed.Decisions {
		if row.Line < 1 || row.Line > len(lines) {
			t.Errorf("decision %d is at line %d, outside a %d-line document",
				row.Number, row.Line, len(lines))

			continue
		}

		if got := strings.TrimSpace(lines[row.Line-1]); !strings.HasPrefix(got, "|") {
			t.Errorf("decision %d is at line %d, which is not a table row: %q",
				row.Number, row.Line, got)
		}
	}
}

func checkListLine(t *testing.T, items map[int]bool, label string, line int, text string) {
	t.Helper()

	if !items[line] {
		t.Errorf("%s is at line %d, which is not a list item docparse reports: %q",
			label, line, text)
	}
}

// TestCorpusFieldIsZeroOnlyWhenItsRegionIs is the second invariant: a zero
// field means the document has no such section, or has an empty one, and never
// means the reader lost it.
//
// The fixtures were picked so that both Open Questions and Decisions are
// absent from some documents and present in others, which is the case this
// exists to exercise: an absent region and an empty one both leave a field
// zero, and only one of the two is a document's fault.
func TestCorpusFieldIsZeroOnlyWhenItsRegionIs(t *testing.T) {
	t.Parallel()

	for _, name := range corpus(t) {
		for _, suffix := range []string{origSuffix, docSuffix} {
			t.Run(name+suffix, func(t *testing.T) {
				t.Parallel()

				doc := readFixture(t, name+suffix)
				parsed := parse(t, doc)

				checkZeroFields(t, doc, &parsed)
			})
		}
	}
}

func checkZeroFields(t *testing.T, doc []byte, parsed *design.Doc) {
	t.Helper()

	regions, _ := kinds.ResolveRegions(doc, design.Headings())

	present := make(map[string][][]byte)

	for _, r := range regions {
		if !r.Closed {
			continue
		}

		present[r.Kind] = append(present[r.Kind], kinds.RegionBytes(doc, r))
	}

	zero := zeroFields(parsed)

	kindNames := make([]string, 0, len(zero))
	for kind := range zero {
		kindNames = append(kindNames, kind)
	}

	sort.Strings(kindNames)

	for _, kind := range kindNames {
		bodies, ok := present[kind]

		switch {
		case !ok:
			if !zero[kind] {
				t.Errorf("the %s field is set but the document has no %s region", kind, kind)
			}
		case zero[kind] && anyContent(kind, bodies):
			t.Errorf("the %s field is zero but its region holds content a reader should find", kind)
		case zero[kind]:
			// The legitimate case, and the reason this is an "only when"
			// rather than a plain "if and only if": docz-design-0002 heads a
			// numbered list "## Decisions", so the region is present and the
			// rows are not there to be read.
			t.Logf("the %s region is present but empty, so a zero field is right", kind)
		case !anyContent(kind, bodies):
			t.Errorf("the %s field is set but its region holds nothing docparse reports", kind)
		}
	}
}

// zeroFields reports, per region kind design reads, whether the field that
// kind fills is zero.
func zeroFields(d *design.Doc) map[string]bool {
	out := map[string]bool{
		kindGoals:         len(d.Goals) == 0,
		kindNonGoals:      len(d.NonGoals) == 0,
		kindOpenQuestions: len(d.OpenQuestions) == 0,
		kindDecisions:     len(d.Decisions) == 0,
		kindReferences:    len(d.References) == 0,
	}

	for _, f := range bodyFields(d) {
		out[f.name] = f.value == ""
	}

	return out
}

// anyContent reports whether any of a kind's regions holds something its
// reader should have reported.
func anyContent(kind string, bodies [][]byte) bool {
	for _, body := range bodies {
		if hasContent(kind, body) {
			return true
		}
	}

	return false
}

// numberedQuestion matches the heading shape that makes a level-3 heading a
// question. Spelled here rather than borrowed, so the invariant is derived
// from the document rather than from the code under test.
var numberedQuestion = regexp.MustCompile(`^\d{1,3}[.)]`)

// hasContent judges a region's bytes from docparse alone: whether the kind's
// reader had anything to find.
//
// Derived independently of the field under test, which is the point — an
// assertion that asked design what it read would pass for a reader that read
// nothing.
func hasContent(kind string, body []byte) bool {
	switch kind {
	case kindGoals, kindNonGoals, kindReferences:
		return hasTopLevelItem(body)
	case kindOpenQuestions:
		return hasNumberedQuestion(body)
	case kindDecisions:
		return hasDecisionRow(body)
	default:
		return hasProse(body)
	}
}

// hasTopLevelItem reports whether the region has a bullet of its own, past its
// heading. A nested bullet is part of what its parent says, which is the same
// rule kinds applies and the same reason a region of nothing but sub-bullets
// reports nothing.
func hasTopLevelItem(body []byte) bool {
	for _, item := range docparse.ListItems(pastHeading(body)) {
		if item.Indent == 0 && item.Text != "" {
			return true
		}
	}

	return false
}

// hasNumberedQuestion reports whether the region has a numbered level-3
// heading in it. A section that numbers its questions any other way — as an
// ordered list, as docz-design-0004 does — has no questions to find.
func hasNumberedQuestion(body []byte) bool {
	for _, h := range docparse.Headings(pastHeading(body)) {
		if h.Level == 3 && numberedQuestion.MatchString(h.Text) {
			return true
		}
	}

	return false
}

// hasDecisionRow reports whether the region has a table with a question column,
// a decision column, and a row under them.
//
// All three conditions, because all three are what a row means: api-design-0001
// heads its columns "# | Topic | Choice | Rationale", and a table with no
// column named for the question is a table this reader cannot line up against
// the questions.
func hasDecisionRow(body []byte) bool {
	for _, table := range docparse.Tables(pastHeading(body)) {
		if len(table.Rows) == 0 {
			continue
		}

		question, resolution := false, false

		for _, cell := range table.Header {
			switch strings.ToLower(strings.TrimSpace(cell)) {
			case "question", "open question":
				question = true
			case "decision", "resolution", "answer", "choice":
				resolution = true
			}
		}

		if question && resolution {
			return true
		}
	}

	return false
}

// hasProse reports whether anything is left of the region once its heading and
// its HTML comments are gone. The comments go because the templates put their
// guidance to the author in them, so a section nobody filled in holds nothing.
func hasProse(body []byte) bool {
	return strings.TrimSpace(stripComments(string(pastHeading(body)))) != ""
}

// pastHeading drops a region's first line, which is its heading or its marker
// and never content.
func pastHeading(body []byte) []byte {
	_, rest, found := strings.Cut(string(body), "\n")
	if !found {
		return nil
	}

	return []byte(rest)
}

// stripComments removes HTML comments, including the multi-line ones the
// templates ship. An unterminated comment swallows the rest, which is what a
// renderer does with it too.
//
// The same arithmetic kinds uses, on purpose: the independence this invariant
// needs is that the answer comes from the document's bytes rather than from the
// field under test, not that a comment means something different here.
func stripComments(s string) string {
	var sb strings.Builder

	for {
		open := strings.Index(s, "<!--")
		if open < 0 {
			sb.WriteString(s)

			return sb.String()
		}

		sb.WriteString(s[:open])

		rest := s[open+4:]

		shut := strings.Index(rest, "-->")
		if shut < 0 {
			return sb.String()
		}

		s = rest[shut+3:]
	}
}

// TestCorpusFindingsAreEarned is the third invariant: every finding Validate
// reports on a real document has its condition re-derived from the Doc and
// confirmed.
//
// A finding whose condition does not hold is not a strict finding, it is
// noise, and noise is what makes a person turn a validator off. So the
// assertion is per finding rather than per document: a run that reported the
// right number of findings for the wrong reasons would pass a count.
func TestCorpusFindingsAreEarned(t *testing.T) {
	t.Parallel()

	for _, name := range corpus(t) {
		for _, suffix := range []string{origSuffix, docSuffix} {
			t.Run(name+suffix, func(t *testing.T) {
				t.Parallel()

				doc := readFixture(t, name+suffix)
				parsed := parse(t, doc)

				for _, f := range design.Validate(doc) {
					checkFinding(t, doc, &parsed, f)
				}
			})
		}
	}
}

func checkFinding(t *testing.T, doc []byte, parsed *design.Doc, f validate.Finding) {
	t.Helper()

	switch f.Code {
	case design.CodeGoalsEmpty:
		if len(parsed.Goals) != 0 {
			t.Errorf("%s at line %d, but the document has %d goals",
				f.Code, f.Line, len(parsed.Goals))
		}

		if !hasRegion(doc, kindGoals) {
			t.Errorf("%s at line %d, but the document has no goals region", f.Code, f.Line)
		}
	case design.CodeStatusOpenQuestion:
		checkStatusOpenQuestion(t, parsed, f)
	case design.CodeDecisionsMismatch:
		checkDecisionsMismatch(t, doc, parsed, f)
	case design.CodeParse:
		// Unreachable for a corpus document: parse(t, doc) above would have
		// failed the test first. Named so a new fixture that Parse rejects
		// says which code it earned rather than falling into the default.
		t.Errorf("%s: the fixture parsed, so this finding cannot be right: %s", f.Code, f.Detail)
	default:
		t.Errorf("finding %q has no re-derivation here: %+v", f.Code, f)
	}
}

// checkStatusOpenQuestion re-derives the status rule: the status means the
// thinking is over, and an unresolved question sits at the line the finding
// names.
func checkStatusOpenQuestion(t *testing.T, parsed *design.Doc, f validate.Finding) {
	t.Helper()

	status := strings.ToLower(string(parsed.Status))
	if status != "approved" && status != "implemented" {
		t.Errorf("%s at line %d, but the status is %q", f.Code, f.Line, parsed.Status)
	}

	for _, q := range parsed.OpenQuestions {
		if q.Line == f.Line {
			if q.Resolved != nil {
				t.Errorf("%s at line %d, but question %d is resolved",
					f.Code, f.Line, q.Number)
			}

			return
		}
	}

	t.Errorf("%s at line %d, but no question is at that line", f.Code, f.Line)
}

// checkDecisionsMismatch re-derives the mismatch rule in both its directions:
// a resolved question the table has no row for, or a row the questions do not
// ask. Either way the region has to be present, because a design with no table
// is written correctly and must report nothing.
func checkDecisionsMismatch(t *testing.T, doc []byte, parsed *design.Doc, f validate.Finding) {
	t.Helper()

	if !hasRegion(doc, kindDecisions) {
		t.Errorf("%s at line %d, but the document has no decisions region", f.Code, f.Line)
	}

	decided := make(map[int]bool, len(parsed.Decisions))

	for _, row := range parsed.Decisions {
		if row.Number != 0 {
			decided[row.Number] = true
		}
	}

	asked := make(map[int]bool, len(parsed.OpenQuestions))
	for _, q := range parsed.OpenQuestions {
		asked[q.Number] = true
	}

	for _, q := range parsed.OpenQuestions {
		if q.Line != f.Line {
			continue
		}

		if q.Resolved == nil {
			t.Errorf("%s at question %d (line %d), but the question is still open",
				f.Code, q.Number, f.Line)
		}

		if decided[q.Number] {
			t.Errorf("%s at question %d (line %d), but the table has a row for it",
				f.Code, q.Number, f.Line)
		}

		return
	}

	for _, row := range parsed.Decisions {
		if row.Line != f.Line {
			continue
		}

		if row.Number != 0 && asked[row.Number] {
			t.Errorf("%s at row %d (line %d), but the document asks question %d",
				f.Code, row.Number, f.Line, row.Number)
		}

		return
	}

	t.Errorf("%s at line %d, which is neither a question nor a decisions row", f.Code, f.Line)
}

// hasRegion reports whether the document has a closed region of the kind.
func hasRegion(doc []byte, kind string) bool {
	regions, _ := kinds.ResolveRegions(doc, design.Headings())

	for _, r := range regions {
		if r.Kind == kind && r.Closed {
			return true
		}
	}

	return false
}

// corpusCodes records which of design's codes the real corpus reaches.
//
// Every code is named, and the false ones carry the reason: the alternative is
// a corpus that quietly stops exercising a rule, which looks exactly like a
// rule that works. Nothing is invented to fill a gap — a fixture written to
// trip a code would be a synthetic document wearing a real one's clothes, and
// validate_test.go already covers every code against the grammar fixture.
var corpusCodes = map[string]bool{
	// A snapshot of a real design has frontmatter and LF endings, so the only
	// way into this code is bytes docz would not have written in the first
	// place. Covered by TestValidate_ParseErrorIsOneFinding instead.
	design.CodeParse: false,

	// Every design in the corpus states its goals as a bulleted list under
	// "### Goals", which is what the template asks for. Covered by
	// TestValidate_Rules instead.
	design.CodeGoalsEmpty: false,

	design.CodeStatusOpenQuestion: true,
	design.CodeDecisionsMismatch:  true,
}

// TestCorpusCodeCoverage pins which rules the real documents exercise.
//
// The task asks for a passing and a failing document per code, and the corpus
// cannot supply both for every code — so this records what it does supply. A
// code that starts firing, or stops, changes this table, which is a decision
// somebody makes rather than a fact that drifts.
func TestCorpusCodeCoverage(t *testing.T) {
	t.Parallel()

	seen := make(map[string]bool, len(corpusCodes))

	for _, name := range corpus(t) {
		for _, suffix := range []string{origSuffix, docSuffix} {
			counts := make(map[string]int, len(corpusCodes))

			for _, f := range design.Validate(readFixture(t, name+suffix)) {
				seen[f.Code] = true
				counts[f.Code]++
			}

			// Logged rather than asserted: which document earns which code is
			// the corpus's business and changes when a fixture is added, but a
			// reader of a failure wants to know where the codes came from.
			t.Logf("%s%s: %s", name, suffix, tally(counts))
		}
	}

	for code, want := range corpusCodes {
		if got := seen[code]; got != want {
			t.Errorf("%s is exercised by the corpus = %v, want %v (update corpusCodes and say why)",
				code, got, want)
		}
	}

	for code := range seen {
		if _, ok := corpusCodes[code]; !ok {
			t.Errorf("the corpus reports %q, which corpusCodes does not name", code)
		}
	}
}

// tally renders a code-count map in a stable order.
func tally(counts map[string]int) string {
	if len(counts) == 0 {
		return "no findings"
	}

	out := make([]string, 0, len(counts))
	for code, n := range counts {
		out = append(out, fmt.Sprintf("%s×%d", code, n))
	}

	sort.Strings(out)

	return strings.Join(out, " ")
}
