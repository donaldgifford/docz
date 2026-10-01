package runbook_test

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
	"github.com/donaldgifford/docz/v2/pkg/runbook"
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
// verbatim snapshot of a real runbook, and <name>.md is the same document with
// canonical region markers added and nothing else changed.
//
// Snapshots rather than reads from docs/: a fixture that followed the repo's
// own documents would change its own expectations every time someone edited a
// plan, and these exist to pin the grammar against documents as they were
// actually written. The two docz runbooks are snapshotted with their markers
// taken out, so the pair proves inference over documents the template shaped.
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
// disagreement with lines somebody typed.
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

			parsed, err := runbook.Parse(marked)
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
		t.Errorf("%s is stale; run go test ./pkg/runbook/... -update\n%s",
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
// blank line is inserted: legacy-onboarding writes one procedure's steps as
// headings, and the migration leaves them that way. A migration that "fixed"
// the document would make the pair test meaningless.
func insertRegions(doc []byte) ([]byte, error) {
	regions := kinds.InferRegions(doc, runbook.Headings())
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
func describe(d *runbook.Doc) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "id=%q title=%q status=%q inferred=%v\n", d.ID, d.Title, d.Status, d.Inferred)
	fmt.Fprintf(&sb, "overview=%q\n", excerpt(d.Overview))
	fmt.Fprintf(&sb, "service=%q owner=%q\n", d.Service, d.Owner)

	if v := d.LastVerified; v != nil {
		fmt.Fprintf(&sb, "last-verified line=%d date=%q pr=%q commit=%q by=%q\n  notes=%q\n",
			v.Line, v.Date, v.PR, v.Commit, v.VerifiedBy, excerpt(v.Notes))
	} else {
		fmt.Fprintln(&sb, "last-verified=nil")
	}

	fmt.Fprintf(&sb, "when=%d prerequisites=%d references=%d\n",
		len(d.When), len(d.Prerequisites), len(d.References))

	for _, c := range d.Escalation {
		fmt.Fprintf(&sb, "contact line=%d who=%q when=%q how=%q\n", c.Line, c.Who, c.When, c.How)
	}

	for i := range d.Procedures {
		p := &d.Procedures[i]

		fmt.Fprintf(&sb, "\nprocedure index=%d token=%q line=%d title=%q\n",
			p.Index, p.Token, p.Line, p.Title)
		fmt.Fprintf(&sb, "  description=%q verification=%d\n", excerpt(p.Description), len(p.Verification))
		describeSteps(&sb, p.Steps, "  ")
		describeSteps(&sb, p.Rollback, "  ")
	}

	for _, s := range d.Scenarios {
		fmt.Fprintf(&sb, "\nscenario index=%d line=%d symptom=%q\n", s.Index, s.Line, s.Symptom)
		fmt.Fprintf(&sb, "  alert=%q\n  likely-cause=%q\n", s.Alert, excerpt(s.LikelyCause))
		describeSteps(&sb, s.Steps, "  ")
	}

	return sb.String()
}

func describeSteps(sb *strings.Builder, steps []runbook.Step, indent string) {
	for _, s := range steps {
		fmt.Fprintf(sb, "%sstep %s line=%d end=%d\n%s  text=%q\n",
			indent, s.ID, s.Line, s.EndLine, indent, excerpt(s.Text))

		if s.Expected != "" {
			fmt.Fprintf(sb, "%s  expected=%q\n", indent, excerpt(s.Expected))
		}

		for _, c := range s.Commands {
			fmt.Fprintf(sb, "%s  command line=%d lang=%q body=%q\n",
				indent, c.Line, c.Lang, excerpt(c.Body))
		}

		describeSteps(sb, s.Children, indent+"  ")
	}
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

// TestCorpusInvariants runs the properties that hold for every document,
// original and migrated alike.
func TestCorpusInvariants(t *testing.T) {
	t.Parallel()

	for _, name := range corpus(t) {
		for _, suffix := range []string{origSuffix, docSuffix} {
			t.Run(name+suffix, func(t *testing.T) {
				t.Parallel()

				doc := readFixture(t, name+suffix)

				parsed, err := runbook.Parse(doc)
				if err != nil {
					t.Fatalf("Parse: %v", err)
				}

				checkInvariants(t, doc, &parsed)
			})
		}
	}
}

// checkInvariants pins what a consumer relies on: every step is a numbered
// item the facts layer agrees is one, its span is the right way round, and
// its ID is unique in the document.
func checkInvariants(t *testing.T, doc []byte, parsed *runbook.Doc) {
	t.Helper()

	ordered := make(map[int]bool)

	for _, item := range docparse.ListItems(doc) {
		if item.Ordered {
			ordered[item.Line] = true
		}
	}

	seen := make(map[string]bool)

	for _, s := range parsed.Steps() {
		if !ordered[s.Line] {
			t.Errorf("step %s at line %d is not a numbered item docparse reports", s.ID, s.Line)
		}

		if s.EndLine < s.Line {
			t.Errorf("step %s EndLine %d before Line %d", s.ID, s.EndLine, s.Line)
		}

		if seen[s.ID] {
			t.Errorf("step ID %s appears twice", s.ID)
		}

		seen[s.ID] = true

		if strings.Contains(s.Text, "<!--docz:") || strings.Contains(s.Text, "**Expected") {
			t.Errorf("step %s Text keeps what has a field of its own: %q", s.ID, s.Text)
		}
	}
}

// TestCorpusMigrationChangesNothingButMarkers is DESIGN-0015 §6 over the
// corpus: a runbook read by its headings and the same runbook read by its
// markers are the same runbook, line numbers aside.
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

// withoutLines renders a Doc's facts with every line number dropped, and
// without Inferred.
func withoutLines(d *runbook.Doc) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "id=%q title=%q status=%q\n", d.ID, d.Title, d.Status)
	fmt.Fprintf(&sb, "overview=%q service=%q owner=%q\n", d.Overview, d.Service, d.Owner)

	if v := d.LastVerified; v != nil {
		fmt.Fprintf(&sb, "verified %q %q %q %q %q\n", v.Date, v.PR, v.Commit, v.VerifiedBy, v.Notes)
	}

	for _, item := range d.When {
		fmt.Fprintf(&sb, "when=%q\n", item.Text)
	}

	for _, item := range d.Prerequisites {
		fmt.Fprintf(&sb, "prerequisite=%q\n", item.Text)
	}

	for _, c := range d.Escalation {
		fmt.Fprintf(&sb, "contact %q %q %q\n", c.Who, c.When, c.How)
	}

	for _, ref := range d.References {
		fmt.Fprintf(&sb, "reference %q %q\n", ref.Text, ref.URL)
	}

	for i := range d.Procedures {
		p := &d.Procedures[i]

		fmt.Fprintf(&sb, "procedure %d %q %q\n  %q\n", p.Index, p.Token, p.Title, p.Description)

		for _, item := range p.Verification {
			fmt.Fprintf(&sb, "  verification=%q\n", item.Text)
		}
	}

	for _, s := range d.Scenarios {
		fmt.Fprintf(&sb, "scenario %d %q %q %q\n", s.Index, s.Symptom, s.Alert, s.LikelyCause)
	}

	for _, s := range d.Steps() {
		fmt.Fprintf(&sb, "step %s %q expected=%q children=%d\n", s.ID, s.Text, s.Expected, len(s.Children))

		for _, c := range s.Commands {
			fmt.Fprintf(&sb, "  command %q %q\n", c.Lang, c.Body)
		}
	}

	return sb.String()
}
