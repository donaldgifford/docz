package impl_test

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
	"github.com/donaldgifford/docz/v2/pkg/impl"
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
// verbatim snapshot of a real plan, and <name>.md is the same document with
// canonical region markers added and nothing else changed.
//
// Snapshots rather than reads from docs/: a fixture that followed the repo's
// own documents would change its own expectations every time someone edited a
// plan, and these exist to pin the grammar against documents as they were
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
// disagreement with 3,900 lines somebody typed.
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

			parsed, err := impl.Parse(marked)
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
		t.Errorf("%s is stale; run go test ./pkg/impl/... -update\n%s",
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
// blank line is inserted: booty-messy has checkboxes under a phase heading
// with no Tasks heading above them, and the migration leaves it that way, so
// the migrated copy reads exactly as the original does. A migration that
// "fixed" the document would make the pair test meaningless.
func insertRegions(doc []byte) ([]byte, error) {
	regions := kinds.InferRegions(doc, impl.Headings())
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
// read the document the way a person does, and %+v of a 17-field struct is not
// something anyone reads.
func describe(d *impl.Doc) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "id=%q title=%q status=%q inferred=%v\n", d.ID, d.Title, d.Status, d.Inferred)
	fmt.Fprintf(&sb, "implements=%v\n", d.Implements)
	fmt.Fprintf(&sb, "objective=%q\n", excerpt(d.Objective))
	fmt.Fprintf(&sb, "in-scope=%d out-of-scope=%d\n", len(d.InScope), len(d.OutOfScope))
	fmt.Fprintf(&sb, "dependencies=%q\n", excerpt(d.Dependencies))
	fmt.Fprintf(&sb, "testing=%d references=%d open-questions=%d decisions=%d\n",
		len(d.Testing), len(d.References), len(d.OpenQuestions), len(d.Decisions))

	done, total := d.Progress()
	fmt.Fprintf(&sb, "progress=%d/%d\n", done, total)

	for _, change := range d.FileChanges {
		fmt.Fprintf(&sb, "file-change line=%d file=%q action=%q description=%q\n",
			change.Line, change.File, change.Action, excerpt(change.Description))
	}

	for _, phase := range d.Phases {
		fmt.Fprintf(&sb, "\nphase index=%d token=%q line=%d title=%q\n",
			phase.Index, phase.Token, phase.Line, phase.Title)
		fmt.Fprintf(&sb, "  description=%q\n", excerpt(phase.Description))

		for _, task := range phase.Tasks {
			fmt.Fprintf(&sb, "  task %s line=%d end=%d checked=%v%s%s%s\n    text=%q\n",
				task.ID, task.Line, task.EndLine, task.Checked,
				marker(" verify", task.Verify),
				markerOf(" deferred", task.Deferred),
				markerOf(" skipped", task.Skipped),
				task.Text)
		}

		for _, c := range phase.Criteria {
			fmt.Fprintf(&sb, "  criterion line=%d executable=%v command=%q text=%q\n",
				c.Line, c.Executable, c.Command, excerpt(c.Text))
		}
	}

	return sb.String()
}

func marker(label, value string) string {
	if value == "" {
		return ""
	}

	return fmt.Sprintf("%s=%q", label, value)
}

func markerOf(label string, m *impl.Marker) string {
	if m == nil {
		return ""
	}

	return fmt.Sprintf("%s={line=%d note=%q}", label, m.Line, m.Note)
}

// excerpt keeps a golden readable. A fact file that reproduced a 900-line
// plan's whole objective would not be one anybody reviews.
func excerpt(s string) string {
	const limit = 90

	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= limit {
		return s
	}

	return s[:limit] + "…"
}

// TestCorpusInvariants runs the properties that hold for every document,
// original and migrated alike. A golden says what one document parsed to; these
// say what any document must.
func TestCorpusInvariants(t *testing.T) {
	t.Parallel()

	for _, name := range corpus(t) {
		for _, suffix := range []string{origSuffix, docSuffix} {
			t.Run(name+suffix, func(t *testing.T) {
				t.Parallel()

				doc := readFixture(t, name+suffix)

				parsed, err := impl.Parse(doc)
				if err != nil {
					t.Fatalf("Parse: %v", err)
				}

				checkInvariants(t, doc, &parsed)
			})
		}
	}
}

func checkInvariants(t *testing.T, doc []byte, parsed *impl.Doc) {
	t.Helper()

	// Every task line is a line docparse reports a checkbox on. This is what
	// makes a Doc safe to hand to docwrite.CheckTask: the splice target is a
	// line the facts layer agrees is a task item.
	checkboxes := make(map[int]bool)
	for _, item := range docparse.TaskItems(doc) {
		checkboxes[item.Line] = true
	}

	seen := make(map[string]bool)

	for _, phase := range parsed.Phases {
		for i, task := range phase.Tasks {
			if !checkboxes[task.Line] {
				t.Errorf("task %s at line %d is not a checkbox docparse reports",
					task.ID, task.Line)
			}

			if task.EndLine < task.Line {
				t.Errorf("task %s EndLine %d before Line %d", task.ID, task.EndLine, task.Line)
			}

			if seen[task.ID] {
				t.Errorf("task ID %s appears twice", task.ID)
			}

			seen[task.ID] = true

			// IDs are positional, so a skipped task keeps the ID it had: the
			// nth task of a phase is always <token>.n, skipped or not.
			if want := fmt.Sprintf("%s.%d", phase.Token, i+1); task.ID != want {
				t.Errorf("task %d of phase %s has ID %s, want %s", i+1, phase.Token, task.ID, want)
			}

			checkTaskText(t, &phase.Tasks[i])
		}
	}
}

// checkTaskText is the contract a consumer matches on: Text is what the task
// asks for, with nothing about its state left in it.
//
// A verify prefix is checked at the start of the folded text, not anywhere in
// it. docz-api IMPL-0004 writes most of its verify steps inline — "…get their
// modelines then. Verify: `yamllint …`" — and the grammar reads a verify step
// only from a line that starts with one (DESIGN-0014 §3), so that prose stays
// in Text and belongs there. What must never happen is a verify line surviving
// as the head of a task, which would mean the line was not recognised.
func checkTaskText(t *testing.T, task *impl.Task) {
	t.Helper()

	lowered := strings.ToLower(task.Text)

	for _, banned := range []string{"verify:", "skipped:", "**verify"} {
		if strings.HasPrefix(lowered, banned) {
			t.Errorf("task %s Text opens with %q: %q", task.ID, banned, task.Text)
		}
	}

	if strings.Contains(task.Text, "<!--docz:") {
		t.Errorf("task %s Text keeps a region marker: %q", task.ID, task.Text)
	}

	if task.Skipped != nil && strings.Contains(task.Text, "~~") {
		t.Errorf("task %s is skipped but Text keeps its strikethrough: %q", task.ID, task.Text)
	}
}

// TestCorpusMigrationChangesNothingButMarkers is the load-bearing proof of
// DESIGN-0015 §6 over the real corpus: a document read by its headings and the
// same document read by its markers are the same document.
//
// Line numbers are excluded, because markers are lines and adding them moves
// everything below. Everything else — every phase, task, criterion, field, and
// marker note — has to match exactly.
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
// without Inferred: markers are lines, so adding them moves everything below,
// and Inferred is the one field the two copies are meant to differ on.
func withoutLines(d *impl.Doc) string {
	stripped := *d

	var sb strings.Builder

	fmt.Fprintf(&sb, "id=%q title=%q status=%q\n", stripped.ID, stripped.Title, stripped.Status)
	fmt.Fprintf(&sb, "implements=%v\n", stripped.Implements)
	fmt.Fprintf(&sb, "objective=%q\n", stripped.Objective)
	fmt.Fprintf(&sb, "dependencies=%q\n", stripped.Dependencies)

	for _, item := range stripped.InScope {
		fmt.Fprintf(&sb, "in-scope=%q\n", item.Text)
	}

	for _, item := range stripped.OutOfScope {
		fmt.Fprintf(&sb, "out-of-scope=%q\n", item.Text)
	}

	for _, change := range stripped.FileChanges {
		fmt.Fprintf(&sb, "file-change %q %q %q\n", change.File, change.Action, change.Description)
	}

	for _, item := range stripped.Testing {
		fmt.Fprintf(&sb, "testing checked=%v %q\n", item.Checked, item.Text)
	}

	for _, ref := range stripped.References {
		fmt.Fprintf(&sb, "reference %q %q\n", ref.Text, ref.URL)
	}

	for _, q := range stripped.OpenQuestions {
		fmt.Fprintf(&sb, "question %d %q options=%d resolved=%v\n",
			q.Number, q.Title, len(q.Options), q.Resolved != nil)
	}

	for _, d := range stripped.Decisions {
		fmt.Fprintf(&sb, "decision %d %q %q\n", d.Number, d.Question, d.Resolution)
	}

	for _, phase := range stripped.Phases {
		fmt.Fprintf(&sb, "phase %d %q %q\n  %q\n",
			phase.Index, phase.Token, phase.Title, phase.Description)

		for _, task := range phase.Tasks {
			fmt.Fprintf(&sb, "  task %s checked=%v verify=%q deferred=%s skipped=%s\n    %q\n",
				task.ID, task.Checked, task.Verify,
				noteOf(task.Deferred), noteOf(task.Skipped), task.Text)
		}

		for _, c := range phase.Criteria {
			fmt.Fprintf(&sb, "  criterion executable=%v command=%q %q\n",
				c.Executable, c.Command, c.Text)
		}
	}

	return sb.String()
}

func noteOf(m *impl.Marker) string {
	if m == nil {
		return "-"
	}

	return fmt.Sprintf("%q", m.Note)
}
