package investigation_test

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
	"github.com/donaldgifford/docz/v2/pkg/investigation"
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
// verbatim snapshot of a real investigation, and <name>.md is the same document
// with canonical region markers added and nothing else changed.
//
// Snapshots rather than reads from docs/: a fixture that followed the repo's
// own documents would change its own expectations every time someone edited an
// investigation, and these exist to pin the grammar against documents as they
// were actually written.
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

			parsed, err := investigation.Parse(marked)
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
		t.Errorf("%s is stale; run go test ./pkg/investigation/... -update\n%s",
			path, firstDifference(string(got), string(want)))
	}
}

// firstDifference reports the first line the two differ on, which is enough to
// see what moved without printing a 900-line document.
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
// blank line is inserted: INV-0001 writes its decisions as a numbered list
// where the template asks for a table, and the migration leaves it that way, so
// the migrated copy reads exactly as the original does — no decisions either
// way. A migration that "fixed" the document would make the pair test
// meaningless.
func insertRegions(doc []byte) ([]byte, error) {
	regions := kinds.InferRegions(doc, investigation.Headings())
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

// describe renders a Doc as the fact file a golden pins.
//
// Facts, not a Go dump: a reader of the golden is checking whether the parser
// read the document the way a person does, and %+v of a nineteen-field struct
// is not something anyone reads.
func describe(d *investigation.Doc) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "id=%q title=%q status=%q inferred=%v\n", d.ID, d.Title, d.Status, d.Inferred)
	fmt.Fprintf(&sb, "author=%q created=%q\n", d.Author, d.Created)
	fmt.Fprintf(&sb, "question=%q\n", excerpt(d.Question))
	fmt.Fprintf(&sb, "hypothesis=%q\n", excerpt(d.Hypothesis))
	fmt.Fprintf(&sb, "context=%q\n", excerpt(d.Context))
	fmt.Fprintf(&sb, "triggered-by=%q\n", excerpt(d.TriggeredBy))
	fmt.Fprintf(&sb, "conclusion=%q\n", excerpt(d.Conclusion))
	fmt.Fprintf(&sb, "answer=%q\n", excerpt(d.Answer))
	fmt.Fprintf(&sb, "verdict=%s\n", d.Verdict)
	fmt.Fprintf(&sb, "recommendation=%q\n", excerpt(d.Recommendation))
	fmt.Fprintf(&sb, "approach=%d environment=%d findings=%d\n",
		len(d.Approach), len(d.Environment), len(d.Findings))
	fmt.Fprintf(&sb, "open-questions=%d decisions=%d references=%d\n",
		len(d.OpenQuestions), len(d.Decisions), len(d.References))

	for _, step := range d.Approach {
		fmt.Fprintf(&sb, "\nstep line=%d text=%q\n", step.Line, excerpt(step.Text))
	}

	for _, c := range d.Environment {
		fmt.Fprintf(&sb, "\ncomponent line=%d component=%q value=%q\n",
			c.Line, c.Component, c.Value)
	}

	for _, f := range d.Findings {
		fmt.Fprintf(&sb, "\nfinding line=%d body=%d bytes title=%q\n  %q\n",
			f.Line, len(f.Body), f.Title, excerpt(f.Body))
	}

	for _, q := range d.OpenQuestions {
		fmt.Fprintf(&sb, "\nquestion %d line=%d options=%d title=%q\n",
			q.Number, q.Line, len(q.Options), excerpt(q.Title))
		fmt.Fprintf(&sb, "  resolved=%v%s\n", q.Resolved != nil, choiceOf(q.Resolved))
	}

	for _, dec := range d.Decisions {
		fmt.Fprintf(&sb, "\ndecision %d line=%d question=%q resolution=%q\n",
			dec.Number, dec.Line, excerpt(dec.Question), excerpt(dec.Resolution))
	}

	for _, ref := range d.References {
		fmt.Fprintf(&sb, "\nreference line=%d url=%q text=%q\n",
			ref.Line, ref.URL, excerpt(ref.Text))
	}

	return sb.String()
}

// choiceOf renders a resolution's date, letter, and note, or nothing for a
// question still open. Written out rather than left to %+v, which prints a nil
// pointer as an address and a resolution as one unreadable brace pair.
func choiceOf(r *kinds.Resolution) string {
	if r == nil {
		return ""
	}

	return fmt.Sprintf(" date=%q choice=%q note=%q", r.Date, r.Choice, excerpt(r.Note))
}

// excerpt keeps a golden readable. A fact file that reproduced INV-0002's whole
// context would not be one anybody reviews.
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
// everything below. Everything else — every field, step, row, finding,
// question, decision, and reference — has to match exactly.
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
func withoutLines(d *investigation.Doc) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "id=%q title=%q status=%q\n", d.ID, d.Title, d.Status)
	fmt.Fprintf(&sb, "author=%q created=%q\n", d.Author, d.Created)
	fmt.Fprintf(&sb, "question=%q\n", d.Question)
	fmt.Fprintf(&sb, "hypothesis=%q\n", d.Hypothesis)
	fmt.Fprintf(&sb, "context=%q\n", d.Context)
	fmt.Fprintf(&sb, "triggered-by=%q\n", d.TriggeredBy)
	fmt.Fprintf(&sb, "conclusion=%q\n", d.Conclusion)
	fmt.Fprintf(&sb, "answer=%q verdict=%s\n", d.Answer, d.Verdict)
	fmt.Fprintf(&sb, "recommendation=%q\n", d.Recommendation)

	for _, step := range d.Approach {
		fmt.Fprintf(&sb, "step %q\n", step.Text)
	}

	for _, c := range d.Environment {
		fmt.Fprintf(&sb, "component %q %q\n", c.Component, c.Value)
	}

	for _, f := range d.Findings {
		fmt.Fprintf(&sb, "finding %q\n  %q\n", f.Title, f.Body)
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

	for _, dec := range d.Decisions {
		fmt.Fprintf(&sb, "decision %d %q %q\n", dec.Number, dec.Question, dec.Resolution)
	}

	for _, ref := range d.References {
		fmt.Fprintf(&sb, "reference %q %q\n", ref.Text, ref.URL)
	}

	return sb.String()
}

// TestCorpusLinesAreDocumentLines is invariant 1: every Line a Doc publishes is
// a line the facts layer agrees carries the thing it names.
//
// This is what makes a Doc an address a consumer can act on — the line an
// editor jumps to, the line a review comment hangs off. A Line one row above
// the row it means still looks like a line number, so nothing short of running
// docparse over the whole document can check it.
func TestCorpusLinesAreDocumentLines(t *testing.T) {
	t.Parallel()

	forEachCopy(t, checkLines)
}

func checkLines(t *testing.T, doc []byte, parsed *investigation.Doc) {
	t.Helper()

	bullets := make(map[int]bool)
	for _, item := range docparse.ListItems(doc) {
		bullets[item.Line] = true
	}

	level3 := make(map[int]bool)

	for _, h := range docparse.Headings(doc) {
		if h.Level == 3 {
			level3[h.Line] = true
		}
	}

	lines := strings.Split(string(doc), "\n")

	for i, step := range parsed.Approach {
		if !bullets[step.Line] {
			t.Errorf("Approach[%d].Line = %d, not a list item docparse reports: %q",
				i, step.Line, lineAt(lines, step.Line))
		}
	}

	for i, ref := range parsed.References {
		if !bullets[ref.Line] {
			t.Errorf("References[%d].Line = %d, not a list item docparse reports: %q",
				i, ref.Line, lineAt(lines, ref.Line))
		}
	}

	for i, f := range parsed.Findings {
		if !level3[f.Line] {
			t.Errorf("Findings[%d].Line = %d, not a level-3 heading: %q",
				i, f.Line, lineAt(lines, f.Line))
		}
	}

	for i, q := range parsed.OpenQuestions {
		if !level3[q.Line] {
			t.Errorf("OpenQuestions[%d].Line = %d, not a level-3 heading: %q",
				i, q.Line, lineAt(lines, q.Line))
		}
	}

	// A component and a decision are table rows, and docparse.Tables reports
	// the header line rather than each row's, so the row itself is the
	// assertion: the line the Doc names has to be one.
	for i, c := range parsed.Environment {
		if got := strings.TrimSpace(lineAt(lines, c.Line)); !strings.HasPrefix(got, "|") {
			t.Errorf("Environment[%d].Line = %d, not a table row: %q", i, c.Line, got)
		}
	}

	for i, dec := range parsed.Decisions {
		if got := strings.TrimSpace(lineAt(lines, dec.Line)); !strings.HasPrefix(got, "|") {
			t.Errorf("Decisions[%d].Line = %d, not a table row: %q", i, dec.Line, got)
		}
	}
}

// lineAt returns the 1-based line, or a marker when the number is outside the
// document — which is itself a failure worth seeing in the message.
func lineAt(lines []string, n int) string {
	if n < 1 || n > len(lines) {
		return fmt.Sprintf("<line %d of %d>", n, len(lines))
	}

	return lines[n-1]
}

// regionField is one kind the package reads, the field it fills, and an
// independent reading of whether the region holds anything that field takes.
type regionField struct {
	kind  string
	field string

	// zero reports whether the parsed field is at its zero value.
	zero func(*investigation.Doc) bool

	// content reports whether the region's bytes hold what the field reads,
	// derived from docparse rather than from the field, so the two can
	// disagree and the test can say which.
	content func(region []byte) bool
}

// regionFields is invariant 2's table: every kind readRegion switches on,
// paired with the field it fills.
//
// Written out rather than derived, so a kind added to readRegion without a
// field — or a field filled from the wrong kind — fails here.
func regionFields() []regionField {
	prose := func(region []byte) bool { return strings.TrimSpace(uncomment(bodyOf(region))) != "" }

	return []regionField{
		{
			kind:    "question",
			field:   "Question",
			zero:    func(d *investigation.Doc) bool { return d.Question == "" },
			content: prose,
		},
		{
			kind:    "hypothesis",
			field:   "Hypothesis",
			zero:    func(d *investigation.Doc) bool { return d.Hypothesis == "" },
			content: prose,
		},
		{
			kind:    kindContext,
			field:   "Context",
			zero:    func(d *investigation.Doc) bool { return d.Context == "" },
			content: prose,
		},
		{
			kind:    "conclusion",
			field:   "Conclusion",
			zero:    func(d *investigation.Doc) bool { return d.Conclusion == "" },
			content: prose,
		},
		{
			kind:    "recommendation",
			field:   "Recommendation",
			zero:    func(d *investigation.Doc) bool { return d.Recommendation == "" },
			content: prose,
		},
		{
			kind:    "approach",
			field:   "Approach",
			zero:    func(d *investigation.Doc) bool { return len(d.Approach) == 0 },
			content: hasTopLevelItem,
		},
		{
			kind:    "environment",
			field:   "Environment",
			zero:    func(d *investigation.Doc) bool { return len(d.Environment) == 0 },
			content: hasFilledTableRow,
		},
		{
			kind:    "findings",
			field:   "Findings",
			zero:    func(d *investigation.Doc) bool { return len(d.Findings) == 0 },
			content: hasLevel3Heading,
		},
		{
			kind:    "open-questions",
			field:   "OpenQuestions",
			zero:    func(d *investigation.Doc) bool { return len(d.OpenQuestions) == 0 },
			content: hasNumberedLevel3Heading,
		},
		{
			kind:    "decisions",
			field:   "Decisions",
			zero:    func(d *investigation.Doc) bool { return len(d.Decisions) == 0 },
			content: hasDecisionTable,
		},
		{
			kind:    "references",
			field:   "References",
			zero:    func(d *investigation.Doc) bool { return len(d.References) == 0 },
			content: hasTopLevelItem,
		},
	}
}

// kindContext is the one kind an invariant names as well as the table does. A
// constant so the rule's re-derivation and the table cannot drift into asking
// about different regions.
const kindContext = "context"

// TestCorpusFieldIsZeroOnlyWithoutItsRegion is invariant 2, both ways: a field
// holds something exactly when its own region does.
//
// The forward direction catches a field fed from the wrong place — the whole
// document, or the region above the one it meant. The reverse catches the
// quieter bug: a region resolved, read, and thrown away, which from the outside
// looks like a document that never had the section.
//
// The third branch is the one the corpus needs. A present region that holds
// nothing the field reads is legitimate and common — INV-0001 writes its
// decisions as a numbered list, INV-0005 heads its decisions table "Topic"
// rather than "Question" — so the case is asserted with the reason in the
// message rather than skipped.
func TestCorpusFieldIsZeroOnlyWithoutItsRegion(t *testing.T) {
	t.Parallel()

	forEachCopy(t, checkFieldsAgainstRegions)
}

func checkFieldsAgainstRegions(t *testing.T, doc []byte, parsed *investigation.Doc) {
	t.Helper()

	regions, _ := kinds.ResolveRegions(doc, investigation.Headings())

	for _, rf := range regionFields() {
		at, ok := closedRegion(regions, rf.kind)

		switch {
		case !ok:
			if !rf.zero(parsed) {
				t.Errorf("%s is set but the document has no %s region", rf.field, rf.kind)
			}
		case rf.content(kinds.RegionBytes(doc, at)):
			if rf.zero(parsed) {
				t.Errorf("%s is zero though its %s region at line %d holds content it reads",
					rf.field, rf.kind, at.Start)
			}
		default:
			if !rf.zero(parsed) {
				t.Errorf("%s is set though its %s region at line %d holds nothing it reads",
					rf.field, rf.kind, at.Start)
			}
		}
	}
}

// closedRegion returns the first closed region of a kind. An unclosed region is
// skipped because Parse skips it, so a check that read one would assert against
// a field Parse never filled.
func closedRegion(regions []docparse.Region, kind string) (docparse.Region, bool) {
	for _, r := range regions {
		if r.Kind == kind && r.Closed {
			return r, true
		}
	}

	return docparse.Region{}, false
}

// bodyOf drops a region's first line, which is its heading or its marker. It is
// kinds.bodyLines re-derived, because a predicate that read the heading would
// call every region non-empty.
func bodyOf(region []byte) string {
	lines := strings.Split(strings.TrimSuffix(string(region), "\n"), "\n")
	if len(lines) <= 1 {
		return ""
	}

	return strings.Join(lines[1:], "\n")
}

// uncomment removes HTML comments, including the multi-line ones the templates
// put their guidance to the author in. An unterminated comment swallows the
// rest, as a renderer does with it.
func uncomment(s string) string {
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

func hasTopLevelItem(region []byte) bool {
	for _, item := range docparse.ListItems([]byte(bodyOf(region))) {
		if item.Indent == 0 && item.Text != "" {
			return true
		}
	}

	return false
}

// hasFilledTableRow reads the first table only, which is the one Parse reads,
// and ignores a wholly empty row.
func hasFilledTableRow(region []byte) bool {
	tables := docparse.Tables([]byte(bodyOf(region)))
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

func hasLevel3Heading(region []byte) bool {
	for _, h := range docparse.Headings([]byte(bodyOf(region))) {
		if h.Level == 3 {
			return true
		}
	}

	return false
}

func hasNumberedLevel3Heading(region []byte) bool {
	for _, h := range docparse.Headings([]byte(bodyOf(region))) {
		first, _ := utf8.DecodeRuneInString(h.Text)
		if h.Level == 3 && unicode.IsDigit(first) {
			return true
		}
	}

	return false
}

// hasDecisionTable re-derives the decisions grammar: a table with a question
// column, a column naming the decision, and at least one row.
func hasDecisionTable(region []byte) bool {
	for _, table := range docparse.Tables([]byte(bodyOf(region))) {
		question, resolution := false, false

		// Column names, not region kinds: the two happen to share a word.
		for _, cell := range table.Header {
			switch strings.ToLower(strings.TrimSpace(cell)) {
			case "question", "open question":
				question = true
			case "decision", "resolution", "answer", "choice":
				resolution = true
			}
		}

		if question && resolution && len(table.Rows) > 0 {
			return true
		}
	}

	return false
}

// TestCorpusVerdictAgreesWithAnswer is invariant 3: the verdict is the answer's
// first word and nothing else.
//
// Re-derived here rather than read off a golden, because a golden records what
// the parser did and this records what the document says. The two agreeing is
// the point: a consumer branches on Verdict and renders Answer beside it, and a
// verdict that disagreed with the sentence next to it would be worse than none.
func TestCorpusVerdictAgreesWithAnswer(t *testing.T) {
	t.Parallel()

	forEachCopy(t, checkVerdict)
}

func checkVerdict(t *testing.T, _ []byte, parsed *investigation.Doc) {
	t.Helper()

	if want := verdictFromAnswer(parsed.Answer); parsed.Verdict.String() != want {
		t.Errorf("Verdict = %s, want %s for answer %q",
			parsed.Verdict, want, excerpt(parsed.Answer))
	}
}

// verdictFromAnswer reads a verdict from an answer the way a person does: the
// first run of letters, whatever emphasis or punctuation is wrapped round it.
func verdictFromAnswer(answer string) string {
	var word strings.Builder

	for _, r := range answer {
		if unicode.IsLetter(r) {
			word.WriteRune(unicode.ToLower(r))

			continue
		}

		if word.Len() > 0 {
			break
		}
	}

	switch word.String() {
	case "yes", "no", "inconclusive":
		return word.String()
	default:
		return "unknown"
	}
}

// TestCorpusFindingsHoldTheirConditions is invariant 4: every finding Validate
// reports over the corpus is re-derived from the Doc, independently of the rule
// that produced it.
//
// A finding whose condition cannot be re-derived is a bug in the rule, not a
// reason to loosen the assertion: a code a consumer filters on has to mean the
// same thing to the consumer as it does to the emitter.
func TestCorpusFindingsHoldTheirConditions(t *testing.T) {
	t.Parallel()

	forEachCopy(t, checkFindingConditions)
}

func checkFindingConditions(t *testing.T, doc []byte, parsed *investigation.Doc) {
	t.Helper()

	regions, _ := kinds.ResolveRegions(doc, investigation.Headings())

	for _, f := range investigation.Validate(doc) {
		switch f.Code {
		case investigation.CodeContextNoTrigger:
			_, ok := closedRegion(regions, kindContext)
			if !ok || parsed.TriggeredBy != "" {
				t.Errorf("%s reported with a context region=%v and TriggeredBy=%q",
					f.Code, ok, parsed.TriggeredBy)
			}

		case investigation.CodeConclusionNoAnswer:
			if !concludedStatus(string(parsed.Status)) || parsed.Answer != "" {
				t.Errorf("%s reported with status %q and answer %q",
					f.Code, parsed.Status, excerpt(parsed.Answer))
			}

		case investigation.CodeConclusionVerdict:
			if parsed.Answer == "" || parsed.Verdict != investigation.VerdictUnknown {
				t.Errorf("%s reported with answer %q and verdict %s",
					f.Code, excerpt(parsed.Answer), parsed.Verdict)
			}

		case investigation.CodeParse:
			t.Errorf("%s reported for a document Parse read: %s", f.Code, f.Detail)

		default:
			t.Errorf("unknown code %q: %s", f.Code, f.Detail)
		}
	}
}

// concludedStatus re-derives validate's status gate: the two words that claim
// the investigation is over, folded.
func concludedStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "concluded", "inconclusive":
		return true
	default:
		return false
	}
}

// TestCorpusExercisesTheCodesItCan records which rules the corpus tests and
// which it does not, so a gap is a failing line in a test rather than something
// a reader has to notice.
//
// Exercised:
//
//   - inv.context.no-trigger — no-trigger, the one fixture written for this
//     package. All ten real investigations carry a "**Triggered by:**" line,
//     which is itself worth knowing: the field gets written when the template
//     asks for it.
//   - inv.conclusion.verdict — docz-inv-0001, whose answer opens "Three
//     targeted changes are needed".
//   - inv.conclusion.no-answer — docz-inv-0005 and docz-inv-0007, both
//     Concluded, both of which write "**Answer: Yes — …**" with the bold
//     closing later in the sentence. kinds.Field matches "**Answer:**" and
//     "**Answer**:" and neither of those, so the answer is not found at all.
//     Nobody wrote these documents wrong on purpose, which is exactly why they
//     are the corpus instance of this rule.
//
// Not exercised, and deliberately not fixed by inventing a fixture:
//
//   - inv.parse — every fixture parses. parse_test.go and validate_test.go
//     cover the two rejections (no frontmatter, CR line endings) directly, and
//     a snapshot of a real document that could not be parsed would not be a
//     snapshot of a real document.
func TestCorpusExercisesTheCodesItCan(t *testing.T) {
	t.Parallel()

	seen := make(map[string]bool)

	for _, name := range corpus(t) {
		for _, suffix := range []string{origSuffix, docSuffix} {
			for _, f := range investigation.Validate(readFixture(t, name+suffix)) {
				seen[f.Code] = true
			}
		}
	}

	exercised := []string{
		investigation.CodeContextNoTrigger,
		investigation.CodeConclusionNoAnswer,
		investigation.CodeConclusionVerdict,
	}

	absent := []string{investigation.CodeParse}

	for _, code := range exercised {
		if !seen[code] {
			t.Errorf("%s is recorded as exercised by the corpus, but no fixture reports it", code)
		}
	}

	for _, code := range absent {
		if seen[code] {
			t.Errorf("%s is recorded as absent from the corpus, but a fixture reports it: "+
				"move it to the exercised list and name the fixture", code)
		}
	}

	if got, want := len(seen), len(exercised); got != want {
		t.Errorf("the corpus reports %d codes, want %d: %v", got, want, sortedCodes(seen))
	}
}

func sortedCodes(seen map[string]bool) []string {
	out := make([]string, 0, len(seen))
	for code := range seen {
		out = append(out, code)
	}

	sort.Strings(out)

	return out
}

// forEachCopy runs an invariant over both copies of every corpus document.
//
// Both copies, because the two paths into a Doc are different code: the
// original is read by inference over its headings and the migrated copy by its
// markers, and an invariant that held for one and not the other would be a bug
// a consumer hits on exactly half the fleet.
func forEachCopy(t *testing.T, check func(*testing.T, []byte, *investigation.Doc)) {
	t.Helper()

	for _, name := range corpus(t) {
		for _, suffix := range []string{origSuffix, docSuffix} {
			t.Run(name+suffix, func(t *testing.T) {
				t.Parallel()

				doc := readFixture(t, name+suffix)
				parsed := parse(t, doc)

				check(t, doc, &parsed)
			})
		}
	}
}
