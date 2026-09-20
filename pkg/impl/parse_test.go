package impl_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/document"
	"github.com/donaldgifford/docz/v2/pkg/impl"
)

// grammar is the marked fixture every grammar assertion reads. One document
// rather than a literal per case: the rules interact — a wrapped task ends
// where the next bullet begins, a verify line is a continuation that is not
// text — and a fixture a person can read is the only way to see that.
func grammar(t *testing.T) []byte {
	t.Helper()

	return readFixture(t, "grammar.md")
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()

	content, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	return content
}

// lineOf returns the 1-based line holding the only occurrence of want.
//
// Assertions name the line by its text rather than by number, so inserting a
// paragraph into the fixture does not renumber thirty expectations. It fails
// on a second match, because an ambiguous anchor would silently assert
// against the wrong line.
func lineOf(t *testing.T, doc []byte, want string) int {
	t.Helper()

	found := 0
	at := 0

	for i, line := range strings.Split(string(doc), "\n") {
		if strings.Contains(line, want) {
			found++
			at = i + 1
		}
	}

	switch found {
	case 1:
		return at
	case 0:
		t.Fatalf("fixture has no line containing %q", want)
	default:
		t.Fatalf("fixture has %d lines containing %q, want exactly one", found, want)
	}

	return 0
}

func parse(t *testing.T, doc []byte) impl.Doc {
	t.Helper()

	got, err := impl.Parse(doc)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	return got
}

func TestParse_Frontmatter(t *testing.T) {
	t.Parallel()

	got := parse(t, grammar(t))

	for _, tt := range []struct{ field, got, want string }{
		{"ID", got.ID, "IMPL-0001"},
		{"Title", got.Title, "Grammar fixture"},
		{"Status", string(got.Status), "In Progress"},
		{"Author", got.Author, "Test Author"},
		{"Created", got.Created, "2026-09-20"},
	} {
		if tt.got != tt.want {
			t.Errorf("%s = %q, want %q", tt.field, tt.got, tt.want)
		}
	}

	if got.Inferred {
		t.Error("Inferred = true for a fully marked document")
	}
}

func TestParse_Fields(t *testing.T) {
	t.Parallel()

	got := parse(t, grammar(t))

	// The Implements line stays in Objective. A string field is its region's
	// body (DESIGN-0014 §2.9) and Implements is the same bytes read a second
	// way, not a line cut out of the prose.
	if !strings.HasPrefix(got.Objective, "**Implements:** DESIGN-0014") {
		t.Errorf("Objective = %q, want it to open with the Implements field", got.Objective)
	}

	if !strings.HasSuffix(got.Objective, "read.") {
		t.Errorf("Objective = %q, want it to end with the last prose line", got.Objective)
	}

	if want := []string{"DESIGN-0014", "DESIGN-0015"}; !equal(got.Implements, want) {
		t.Errorf("Implements = %v, want %v", got.Implements, want)
	}

	if len(got.InScope) != 2 {
		t.Errorf("InScope has %d items, want 2", len(got.InScope))
	}

	if len(got.OutOfScope) != 1 {
		t.Errorf("OutOfScope has %d items, want 1", len(got.OutOfScope))
	}

	if got.Dependencies != "None." {
		t.Errorf("Dependencies = %q, want %q", got.Dependencies, "None.")
	}

	if len(got.References) != 1 || got.References[0].URL == "" {
		t.Errorf("References = %+v, want one entry with a link", got.References)
	}
}

// TestParse_LinesAreDocumentLines pins the conversion every field goes
// through: the kinds readers number from the start of the region they were
// handed, and a Doc's Line is an address in the file the caller read.
func TestParse_LinesAreDocumentLines(t *testing.T) {
	t.Parallel()

	doc := grammar(t)
	got := parse(t, doc)

	tests := []struct {
		field  string
		got    int
		anchor string
	}{
		{"InScope[0].Line", got.InScope[0].Line, "- The phase, task, and criteria grammar"},
		{"OutOfScope[0].Line", got.OutOfScope[0].Line, "- The CLI"},
		{"References[0].Line", got.References[0].Line, "- [DESIGN-0014]"},
		{"Criteria[0].Line", got.Phases[0].Criteria[0].Line, "- `go test ./...` passes"},
		{"Tasks[0].Line", got.Phases[0].Tasks[0].Line, "- [x] Write the parser"},
		{"Testing[0].Line", got.Testing[0].Line, "- [ ] Unit tests for the grammar"},
		{"FileChanges[0].Line", got.FileChanges[0].Line, "| `pkg/impl/parse.go` |"},
		{"Phases[0].Line", got.Phases[0].Line, "### Phase 1: Foundations"},
	}

	for _, tt := range tests {
		if want := lineOf(t, doc, tt.anchor); tt.got != want {
			t.Errorf("%s = %d, want %d (%q)", tt.field, tt.got, want, tt.anchor)
		}
	}
}

// TestParse_TestingIsNotTasks pins the one field whose type says what it is
// not: the testing checkboxes are docparse.TaskItem, so a consumer cannot
// hand them to anything that counts progress.
func TestParse_TestingIsNotTasks(t *testing.T) {
	t.Parallel()

	doc := grammar(t)
	got := parse(t, doc)

	if len(got.Testing) != 2 {
		t.Fatalf("Testing has %d items, want 2", len(got.Testing))
	}

	if got.Testing[0].Checked || !got.Testing[1].Checked {
		t.Errorf("Testing checked states = %v/%v, want false/true",
			got.Testing[0].Checked, got.Testing[1].Checked)
	}

	if want := lineOf(t, doc, "Unit tests for the grammar"); got.Testing[0].Line != want {
		t.Errorf("Testing[0].Line = %d, want %d", got.Testing[0].Line, want)
	}

	if _, total := got.Progress(); total != 4 {
		t.Errorf("Progress total = %d, want 4: the testing checkboxes are not tasks", total)
	}
}

func TestParse_FileChanges(t *testing.T) {
	t.Parallel()

	doc := grammar(t)
	got := parse(t, doc)

	// Three rows, one of them the template's empty placeholder.
	if len(got.FileChanges) != 2 {
		t.Fatalf("FileChanges has %d rows, want 2: the empty row is dropped", len(got.FileChanges))
	}

	first := got.FileChanges[0]

	if first.File != "`pkg/impl/parse.go`" {
		t.Errorf("File = %q, want the cell verbatim with its code span", first.File)
	}

	if first.Action != "Add" || first.Description != "The parser" {
		t.Errorf("row = %+v, want Action Add and Description \"The parser\"", first)
	}

	if want := lineOf(t, doc, "| `pkg/impl/parse.go` |"); first.Line != want {
		t.Errorf("Line = %d, want %d", first.Line, want)
	}
}

func TestParse_Phases(t *testing.T) {
	t.Parallel()

	doc := grammar(t)
	got := parse(t, doc)

	if len(got.Phases) != 2 {
		t.Fatalf("got %d phases, want 2", len(got.Phases))
	}

	first, second := got.Phases[0], got.Phases[1]

	if first.Index != 1 || first.Token != "1" || first.Title != "Foundations" {
		t.Errorf("phase 1 = {Index:%d Token:%q Title:%q}, want {1 \"1\" \"Foundations\"}",
			first.Index, first.Token, first.Title)
	}

	// "2B": the token is what the document calls the phase, not its position.
	if second.Index != 2 || second.Token != "2B" || second.Title != "Cleanup" {
		t.Errorf("phase 2 = {Index:%d Token:%q Title:%q}, want {2 \"2B\" \"Cleanup\"}",
			second.Index, second.Token, second.Title)
	}

	if want := "Sets up the packages the later phases build on."; first.Description != want {
		t.Errorf("Description = %q, want %q", first.Description, want)
	}

	if second.Description != "" {
		t.Errorf("Description = %q, want empty for a phase with no prose", second.Description)
	}

	if want := lineOf(t, doc, "### Phase 1: Foundations"); first.Line != want {
		t.Errorf("Line = %d, want %d", first.Line, want)
	}

	if second.Criteria != nil {
		t.Errorf("Criteria = %+v, want nil for a phase with no criteria region", second.Criteria)
	}
}

func TestParse_Criteria(t *testing.T) {
	t.Parallel()

	doc := grammar(t)
	got := parse(t, doc)

	criteria := got.Phases[0].Criteria
	if len(criteria) != 2 {
		t.Fatalf("got %d criteria, want 2", len(criteria))
	}

	// Executable iff the bullet opens with a code span: the criterion is a
	// command someone can run, not one that merely mentions a filename.
	if !criteria[0].Executable || criteria[0].Command != "go test ./..." {
		t.Errorf("criteria[0] = {Executable:%v Command:%q}, want {true \"go test ./...\"}",
			criteria[0].Executable, criteria[0].Command)
	}

	if criteria[1].Executable || criteria[1].Command != "" {
		t.Errorf("criteria[1] = {Executable:%v Command:%q}, want a prose criterion",
			criteria[1].Executable, criteria[1].Command)
	}

	if want := lineOf(t, doc, "- `go test ./...` passes"); criteria[0].Line != want {
		t.Errorf("Line = %d, want %d: criteria lines are the document's", criteria[0].Line, want)
	}
}

func TestParse_Tasks(t *testing.T) {
	t.Parallel()

	doc := grammar(t)
	got := parse(t, doc)

	tasks := got.Phases[0].Tasks
	if len(tasks) != 3 {
		t.Fatalf("got %d tasks, want 3: the nested checkbox is not a task", len(tasks))
	}

	checked := tasks[0]

	if checked.ID != "1.1" || !checked.Checked || checked.Text != "Write the parser" {
		t.Errorf("tasks[0] = {ID:%q Checked:%v Text:%q}, want {\"1.1\" true \"Write the parser\"}",
			checked.ID, checked.Checked, checked.Text)
	}

	if want := "go test ./pkg/impl/..."; checked.Verify != want {
		t.Errorf("Verify = %q, want %q", checked.Verify, want)
	}

	if want := lineOf(t, doc, "- [x] Write the parser"); checked.Line != want {
		t.Errorf("Line = %d, want %d", checked.Line, want)
	}

	// EndLine runs through the verify line, so a caller rewriting the task
	// knows how far it reaches even though Text does not include it.
	if want := lineOf(t, doc, "verify: `go test ./pkg/impl/...`"); checked.EndLine != want {
		t.Errorf("EndLine = %d, want the verify line %d", checked.EndLine, want)
	}
}

func TestParse_TaskContinuation(t *testing.T) {
	t.Parallel()

	doc := grammar(t)
	got := parse(t, doc)

	wrapped := got.Phases[0].Tasks[1]

	want := "Write the walker with a task description that wraps onto a second line"
	if wrapped.Text != want {
		t.Errorf("Text = %q, want the wrapped lines folded with one space:\n  %q",
			wrapped.Text, want)
	}

	// The nested checkbox under it ends the task. It is a list item, and a
	// list item is never a continuation.
	if end := lineOf(t, doc, "wraps onto a second line"); wrapped.EndLine != end {
		t.Errorf("EndLine = %d, want %d: the nested bullet is not a continuation",
			wrapped.EndLine, end)
	}

	if wrapped.Verify != "" {
		t.Errorf("Verify = %q, want empty", wrapped.Verify)
	}
}

func TestParse_Deferred(t *testing.T) {
	t.Parallel()

	doc := grammar(t)
	got := parse(t, doc)

	task := got.Phases[0].Tasks[2]

	if task.Deferred == nil {
		t.Fatal("Deferred = nil, want the marker the task carries")
	}

	if want := "needs a release owner"; task.Deferred.Note != want {
		t.Errorf("Note = %q, want %q: the human-required qualification is not the reason",
			task.Deferred.Note, want)
	}

	// The marker text leaves Text: Text is what the task asks for, and its
	// state lives in the typed fields.
	if want := "Bump the toolchain"; task.Text != want {
		t.Errorf("Text = %q, want %q", task.Text, want)
	}

	if want := lineOf(t, doc, "Bump the toolchain"); task.Deferred.Line != want {
		t.Errorf("Line = %d, want %d", task.Deferred.Line, want)
	}
}

func TestParse_Skipped(t *testing.T) {
	t.Parallel()

	doc := grammar(t)
	got := parse(t, doc)

	tasks := got.Phases[1].Tasks
	if len(tasks) != 2 {
		t.Fatalf("got %d tasks, want 2", len(tasks))
	}

	skipped := tasks[0]

	if skipped.Skipped == nil {
		t.Fatal("Skipped = nil, want the marker the struck-through task carries")
	}

	if want := "the shim shipped"; skipped.Skipped.Note != want {
		t.Errorf("Note = %q, want %q", skipped.Skipped.Note, want)
	}

	// Text is the task as it read before the strike, so a consumer can show
	// what was abandoned.
	if want := "Delete the compatibility shim"; skipped.Text != want {
		t.Errorf("Text = %q, want %q", skipped.Text, want)
	}

	// A skipped task keeps its ID: renumbering would move every address in
	// the phase the moment one task was abandoned.
	if skipped.ID != "2B.1" || tasks[1].ID != "2B.2" {
		t.Errorf("IDs = %q, %q, want 2B.1 and 2B.2", skipped.ID, tasks[1].ID)
	}
}

// TestParse_VerifyWithoutCommand pins the parse half of the finding
// impl.task.verify-no-command reports: a verify line naming no command
// yields no command rather than the prose.
func TestParse_VerifyWithoutCommand(t *testing.T) {
	t.Parallel()

	got := parse(t, grammar(t))

	task := got.Phases[1].Tasks[1]

	if task.Verify != "" {
		t.Errorf("Verify = %q, want empty: the verify line has no code span", task.Verify)
	}

	if want := "Update the docs"; task.Text != want {
		t.Errorf("Text = %q, want %q: the bold verify line is still a verify line",
			task.Text, want)
	}
}

func TestDoc_Lookups(t *testing.T) {
	t.Parallel()

	got := parse(t, grammar(t))

	if task, ok := got.Task("2B.2"); !ok || task.Text != "Update the docs" {
		t.Errorf("Task(2B.2) = %+v, %v, want the second task of phase 2B", task, ok)
	}

	if _, ok := got.Task("9.1"); ok {
		t.Error("Task(9.1) = ok, want not found")
	}

	// Folded, so a document that writes "Phase 2b:" is reachable either way.
	if phase, ok := got.Phase("2b"); !ok || phase.Token != "2B" {
		t.Errorf("Phase(2b) = %+v, %v, want the 2B phase", phase, ok)
	}

	if len(got.Tasks()) != 5 {
		t.Errorf("Tasks() has %d, want 5 across both phases", len(got.Tasks()))
	}
}

// TestDoc_Progress pins the one counting decision in the package: a skipped
// task is in neither the numerator nor the denominator.
func TestDoc_Progress(t *testing.T) {
	t.Parallel()

	got := parse(t, grammar(t))

	done, total := got.Progress()
	if done != 1 || total != 4 {
		t.Errorf("Progress() = %d/%d, want 1/4: 5 tasks less the skipped one", done, total)
	}
}

// TestParse_Inferred is the compatibility path: the same document with every
// marker removed parses to the same phases, tasks, and criteria.
//
// It compares against the marked parse rather than against a literal, so the
// two can never be updated apart. Only Inferred differs, which is the flag
// that exists to say so.
func TestParse_Inferred(t *testing.T) {
	t.Parallel()

	doc := grammar(t)
	marked := parse(t, doc)
	bare := parse(t, unmark(doc))

	if !bare.Inferred {
		t.Error("Inferred = false for a document with no markers")
	}

	if len(bare.Phases) != len(marked.Phases) {
		t.Fatalf("inferred %d phases, marked %d", len(bare.Phases), len(marked.Phases))
	}

	for i := range bare.Phases {
		got, want := bare.Phases[i], marked.Phases[i]

		if got.Token != want.Token || got.Title != want.Title {
			t.Errorf("phase %d = {%q %q}, marked {%q %q}",
				i, got.Token, got.Title, want.Token, want.Title)
		}

		if got.Description != want.Description {
			t.Errorf("phase %d description = %q, marked %q", i, got.Description, want.Description)
		}

		if len(got.Tasks) != len(want.Tasks) {
			t.Fatalf("phase %d inferred %d tasks, marked %d", i, len(got.Tasks), len(want.Tasks))
		}

		for k := range got.Tasks {
			if got.Tasks[k].ID != want.Tasks[k].ID || got.Tasks[k].Text != want.Tasks[k].Text {
				t.Errorf("task %s = %q, marked %s = %q",
					got.Tasks[k].ID, got.Tasks[k].Text,
					want.Tasks[k].ID, want.Tasks[k].Text)
			}

			if got.Tasks[k].Verify != want.Tasks[k].Verify {
				t.Errorf("task %s verify = %q, marked %q",
					got.Tasks[k].ID, got.Tasks[k].Verify, want.Tasks[k].Verify)
			}
		}

		if len(got.Criteria) != len(want.Criteria) {
			t.Errorf("phase %d inferred %d criteria, marked %d",
				i, len(got.Criteria), len(want.Criteria))
		}
	}

	if bare.Objective != marked.Objective {
		t.Errorf("Objective = %q, marked %q", bare.Objective, marked.Objective)
	}

	if len(bare.FileChanges) != len(marked.FileChanges) {
		t.Errorf("inferred %d file changes, marked %d",
			len(bare.FileChanges), len(marked.FileChanges))
	}
}

// TestParse_PartlyMarkedIsNotInferred pins the amended rule: markers, once
// present, are authoritative. A document that names one region is read as
// naming one, and the rest are validate's region.missing findings.
func TestParse_PartlyMarkedIsNotInferred(t *testing.T) {
	t.Parallel()

	doc := grammar(t)

	// Keep only the phase and tasks markers, so the objective has a heading
	// and no marker.
	kept := keepMarkers(doc, "phase", "tasks")

	got := parse(t, kept)

	if got.Inferred {
		t.Error("Inferred = true for a partly marked document")
	}

	if got.Objective != "" {
		t.Errorf("Objective = %q, want empty: its region is not marked", got.Objective)
	}

	if len(got.Phases) != 2 {
		t.Fatalf("got %d phases, want 2", len(got.Phases))
	}

	if len(got.Phases[0].Tasks) != 3 {
		t.Errorf("phase 1 has %d tasks, want 3", len(got.Phases[0].Tasks))
	}

	if got.Phases[0].Criteria != nil {
		t.Error("Criteria is set, want nil: the criteria region is not marked")
	}
}

func TestParse_Errors(t *testing.T) {
	t.Parallel()

	doc := grammar(t)

	tests := []struct {
		name string
		doc  []byte
		want error
	}{
		{
			name: "no frontmatter",
			doc:  []byte("# IMPL-0001\n\n### Phase 1: One\n\n#### Tasks\n\n- [ ] a\n"),
			want: document.ErrNoFrontmatter,
		},
		{
			name: "CR line endings",
			doc:  []byte(strings.ReplaceAll(string(doc), "\n", "\r\n")),
			want: document.ErrUnsupportedLineEndings,
		},
		{
			name: "no phases",
			doc:  dropPhases(doc),
			want: ErrNoPhasesSentinel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, err := impl.Parse(tt.doc); !errors.Is(err, tt.want) {
				t.Errorf("Parse error = %v, want %v", err, tt.want)
			}
		})
	}
}

// ErrNoPhasesSentinel names the exported sentinel so the table above reads
// as three errors of the same shape.
var ErrNoPhasesSentinel = impl.ErrNoPhases

// TestParse_DuplicatePhaseToken pins the other returned error. A token is an
// address, so two phases claiming one makes every task ID under them
// ambiguous.
func TestParse_DuplicatePhaseToken(t *testing.T) {
	t.Parallel()

	doc := []byte(strings.ReplaceAll(string(grammar(t)),
		"### Phase 2B: Cleanup", "### Phase 1: Cleanup"))

	_, err := impl.Parse(doc)

	var dup *impl.DuplicatePhaseError
	if !errors.As(err, &dup) {
		t.Fatalf("Parse error = %v, want a *DuplicatePhaseError", err)
	}

	if dup.Token != "1" {
		t.Errorf("Token = %q, want %q", dup.Token, "1")
	}

	if len(dup.Lines) != 2 {
		t.Errorf("Lines = %v, want both phase headings", dup.Lines)
	}

	if !strings.Contains(dup.Error(), "and") {
		t.Errorf("Error() = %q, want both lines named", dup.Error())
	}
}

// TestParse_CopiesEveryString pins the copy contract: a caller may reuse the
// bytes it passed in.
func TestParse_CopiesEveryString(t *testing.T) {
	t.Parallel()

	doc := grammar(t)
	got := parse(t, doc)

	for i := range doc {
		doc[i] = 'x'
	}

	if got.Title != "Grammar fixture" {
		t.Errorf("Title = %q after overwriting the input, want the parsed value", got.Title)
	}

	if len(got.Phases) == 0 || got.Phases[0].Tasks[0].Text != "Write the parser" {
		t.Error("a task's Text aliased the input")
	}
}

// unmark removes every docz marker line, which is how a v1 document reads.
func unmark(doc []byte) []byte {
	var out []string

	for _, line := range strings.Split(string(doc), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "<!--docz:") {
			continue
		}

		out = append(out, line)
	}

	return []byte(strings.Join(out, "\n"))
}

// keepMarkers removes every docz marker except the named kinds.
func keepMarkers(doc []byte, kinds ...string) []byte {
	var out []string

	for _, line := range strings.Split(string(doc), "\n") {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "<!--docz:") {
			keep := false

			for _, kind := range kinds {
				if strings.HasPrefix(trimmed, "<!--docz:"+kind+":") {
					keep = true
				}
			}

			if !keep {
				continue
			}
		}

		out = append(out, line)
	}

	return []byte(strings.Join(out, "\n"))
}

// dropPhases removes the phase headings and their markers, leaving a
// document with every other region intact.
func dropPhases(doc []byte) []byte {
	var out []string

	for _, line := range strings.Split(string(doc), "\n") {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "<!--docz:phase:") || strings.HasPrefix(trimmed, "### Phase ") {
			continue
		}

		out = append(out, line)
	}

	return []byte(strings.Join(out, "\n"))
}

func equal(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}

	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}

	return true
}

// TestParse_RenderedTemplate is the criterion that the package and the
// template it ships agree: the document `docz create impl` writes parses, and
// every field is either filled or zero for a reason.
//
// The zeroes are the point. The template's placeholders are HTML comments, a
// bare "-" bullet, and a table of empty cells, and all three are deliberately
// read as nothing — which is what makes a freshly created document validate
// clean instead of reporting its own placeholders as content. Pinning the whole
// shape means a template edit that renames a heading, moves a marker, or fills
// a placeholder in shows up here rather than silently changing what every
// document created afterwards parses to.
func TestParse_RenderedTemplate(t *testing.T) {
	t.Parallel()

	got := parse(t, renderedTemplate(t))

	if got.Inferred {
		t.Error("Inferred = true: the template carries markers")
	}

	// What the template fills in.
	if got.Objective == "" {
		t.Error("Objective is empty")
	}

	if len(got.Phases) != 3 {
		t.Fatalf("got %d phases, want the template's 3", len(got.Phases))
	}

	if len(got.Testing) != 3 {
		t.Errorf("Testing has %d checkboxes, want the template's 3", len(got.Testing))
	}

	for _, phase := range got.Phases {
		if len(phase.Tasks) == 0 {
			t.Errorf("phase %s has no tasks", phase.Token)
		}

		if len(phase.Criteria) == 0 {
			t.Errorf("phase %s has no criteria", phase.Token)
		}

		// The title is a placeholder comment, which impl.phase.no-title
		// reports — see TestValidate_Template. The description is guidance in a
		// comment, so it reads as nothing.
		if phase.Title != "" {
			t.Errorf("phase %s title = %q, want empty: the template's is a comment",
				phase.Token, phase.Title)
		}

		if phase.Description != "" {
			t.Errorf("phase %s description = %q, want empty: the template's "+
				"guidance is a comment", phase.Token, phase.Description)
		}
	}

	// What the template leaves for the author, each zero for a stated reason.
	for _, tt := range []struct {
		field, why string
		empty      bool
	}{
		{"InScope", "the placeholder is a bare \"-\" bullet", len(got.InScope) == 0},
		{"OutOfScope", "the placeholder is a bare \"-\" bullet", len(got.OutOfScope) == 0},
		{"FileChanges", "its rows have no file and no description", len(got.FileChanges) == 0},
		{"Dependencies", "the placeholder is an HTML comment", got.Dependencies == ""},
		{"References", "the placeholder is an HTML comment", len(got.References) == 0},
		{"Implements", "the placeholder is an HTML comment", len(got.Implements) == 0},
		{"OpenQuestions", "the template ships no such section", got.OpenQuestions == nil},
		{"Decisions", "the template ships no such section", got.Decisions == nil},
	} {
		if !tt.empty {
			t.Errorf("%s is filled, want empty: %s", tt.field, tt.why)
		}
	}
}
