package rfc_test

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

	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
	"github.com/donaldgifford/docz/v2/pkg/rfc"
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
// verbatim snapshot of a real RFC, and <name>.md is the same document with
// canonical region markers added and nothing else changed.
//
// Snapshots rather than reads from docs/: a fixture that followed the repo's
// own documents would change its own expectations every time someone edited a
// proposal, and these exist to pin the grammar against documents as they were
// actually written.
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

			parsed, err := rfc.Parse(marked)
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
		t.Errorf("%s is stale; run go test ./pkg/rfc/... -update\n%s",
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
// blank line is inserted: all three booty RFCs write their alternatives as
// bold-lead-in paragraphs rather than bullets, and the migration leaves them
// that way, so the migrated copy reports the same zero alternatives the
// original does. A migration that "fixed" the document would make the pair
// test meaningless.
func insertRegions(doc []byte) ([]byte, error) {
	regions := kinds.InferRegions(doc, rfc.Headings())
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
// read the document the way a person does, and %+v of a 14-field struct is not
// something anyone reads.
//
// The four risks cells and the criteria, reference, and alternative titles are
// printed verbatim rather than excerpted. Those are the values a reviewer has
// to check character by character — a mitigation truncated in the golden would
// hide a mitigation truncated by the parser — while the three prose fields run
// to thousands of characters and are excerpted.
func describe(d *rfc.Doc) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "id=%q title=%q status=%q inferred=%v\n", d.ID, d.Title, d.Status, d.Inferred)
	fmt.Fprintf(&sb, "author=%q created=%q\n", d.Author, d.Created)
	fmt.Fprintf(&sb, "summary=%q\n", excerpt(d.Summary))
	fmt.Fprintf(&sb, "problem=%q\n", excerpt(d.Problem))
	fmt.Fprintf(&sb, "proposal=%q\n", excerpt(d.Proposal))
	fmt.Fprintf(&sb, "alternatives=%d risks=%d criteria=%d open-questions=%d references=%d\n",
		len(d.Alternatives), len(d.Risks), len(d.Criteria),
		len(d.OpenQuestions), len(d.References))

	for _, alt := range d.Alternatives {
		fmt.Fprintf(&sb, "\nalternative line=%d label=%q title=%q\n  text=%q\n",
			alt.Line, alt.Label, alt.Title, excerpt(alt.Text))
	}

	for _, risk := range d.Risks {
		fmt.Fprintf(&sb, "\nrisk line=%d\n  risk=%q\n  impact=%q\n  likelihood=%q\n  mitigation=%q\n",
			risk.Line, risk.Risk, risk.Impact, risk.Likelihood, risk.Mitigation)
	}

	if len(d.Criteria) > 0 {
		sb.WriteString("\n")
	}

	for _, c := range d.Criteria {
		fmt.Fprintf(&sb, "criterion line=%d executable=%v command=%q\n  text=%q\n",
			c.Line, c.Executable, c.Command, c.Text)
	}

	for _, q := range d.OpenQuestions {
		fmt.Fprintf(&sb, "\nquestion number=%d line=%d options=%d title=%q\n",
			q.Number, q.Line, len(q.Options), q.Title)
		fmt.Fprintf(&sb, "  resolved=%s\n", resolutionOf(q.Resolved))
	}

	if len(d.References) > 0 {
		sb.WriteString("\n")
	}

	for _, ref := range d.References {
		fmt.Fprintf(&sb, "reference line=%d url=%q\n  text=%q\n", ref.Line, ref.URL, ref.Text)
	}

	return sb.String()
}

// resolutionOf renders an open question's resolution, or the fact that it has
// none. A question with no resolution is what rfc.status.open-question reads,
// so the golden has to distinguish "open" from "resolved with no choice".
func resolutionOf(r *kinds.Resolution) string {
	if r == nil {
		return "no"
	}

	return fmt.Sprintf("{line=%d date=%q choice=%q note=%q}", r.Line, r.Date, r.Choice, excerpt(r.Note))
}

// excerpt keeps a golden readable. A fact file that reproduced a 900-line
// RFC's whole problem statement would not be one anybody reviews.
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
// everything below. Everything else — every alternative, risks cell,
// criterion, question, option, resolution, and reference — has to match
// exactly.
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
func withoutLines(d *rfc.Doc) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "id=%q title=%q status=%q author=%q created=%q\n",
		d.ID, d.Title, d.Status, d.Author, d.Created)
	fmt.Fprintf(&sb, "summary=%q\n", d.Summary)
	fmt.Fprintf(&sb, "problem=%q\n", d.Problem)
	fmt.Fprintf(&sb, "proposal=%q\n", d.Proposal)

	for _, alt := range d.Alternatives {
		fmt.Fprintf(&sb, "alternative %q %q %q\n", alt.Label, alt.Title, alt.Text)
	}

	for _, risk := range d.Risks {
		fmt.Fprintf(&sb, "risk %q %q %q %q\n",
			risk.Risk, risk.Impact, risk.Likelihood, risk.Mitigation)
	}

	for _, c := range d.Criteria {
		fmt.Fprintf(&sb, "criterion executable=%v command=%q %q\n", c.Executable, c.Command, c.Text)
	}

	for _, q := range d.OpenQuestions {
		fmt.Fprintf(&sb, "question %d %q\n", q.Number, q.Title)

		for _, o := range q.Options {
			fmt.Fprintf(&sb, "  option %q recommended=%v %q\n", o.Letter, o.Recommended, o.Text)
		}

		if q.Resolved != nil {
			fmt.Fprintf(&sb, "  resolution %q %q %q\n",
				q.Resolved.Date, q.Resolved.Choice, q.Resolved.Note)
		}
	}

	for _, ref := range d.References {
		fmt.Fprintf(&sb, "reference %q %q\n", ref.Text, ref.URL)
	}

	return sb.String()
}
