package docparse_test

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/pkg/doczcore/docparse"
)

var update = flag.Bool("update", false, "update golden files")

// fixtures are the corpus documents under testdata: a
// template-conformant IMPL doc and a messy hand-written one.
var fixtures = []string{"impl_plan", "messy"}

// renderFacts serializes every fact both walkers extract from content,
// one line per fact, for golden comparison.
func renderFacts(content []byte) string {
	var sb strings.Builder
	sb.WriteString("# headings\n")
	for _, h := range docparse.Headings(content) {
		fmt.Fprintf(&sb, "level=%d line=%d slug=%q text=%q\n", h.Level, h.Line, h.Slug, h.Text)
	}
	sb.WriteString("# tasks\n")
	for _, it := range docparse.TaskItems(content) {
		fmt.Fprintf(
			&sb, "line=%d checked=%t indent=%d text=%q\n",
			it.Line, it.Checked, it.Indent, it.Text,
		)
	}
	return sb.String()
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("testdata", name+".md"))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	return content
}

func TestGoldenFacts(t *testing.T) {
	t.Parallel()
	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := renderFacts(readFixture(t, name))

			goldenPath := filepath.Join("testdata", "golden", name+".facts.txt")

			if *update {
				if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				t.Log("Updated golden file:", goldenPath)
				return
			}

			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf(
					"reading golden file %s: %v\nRun with -update to create it",
					goldenPath, err,
				)
			}

			if got != string(want) {
				t.Errorf(
					"facts differ from golden file %s\nGot:\n%s\nRun with -update to update",
					goldenPath, got,
				)
			}
		})
	}
}

// TestFactLineByteAccuracy checks every reported Line against the raw
// fixture bytes: the heading line must carry exactly Level hash marks,
// and the task line must carry the checkbox state the fact reports.
func TestFactLineByteAccuracy(t *testing.T) {
	t.Parallel()
	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			content := readFixture(t, name)
			lines := strings.Split(string(content), "\n")

			for _, h := range docparse.Headings(content) {
				raw := strings.TrimSpace(lines[h.Line-1])
				hashes := strings.Repeat("#", h.Level)
				if !strings.HasPrefix(raw, hashes) ||
					strings.HasPrefix(raw, hashes+"#") {
					t.Errorf(
						"heading %q reports line %d, but raw line is %q",
						h.Text, h.Line, raw,
					)
				}
			}

			for _, it := range docparse.TaskItems(content) {
				raw := lines[it.Line-1]
				marker := "[ ]"
				if it.Checked {
					if !strings.Contains(raw, "[x]") && !strings.Contains(raw, "[X]") {
						t.Errorf(
							"checked task %q reports line %d, but raw line is %q",
							it.Text, it.Line, raw,
						)
					}
					continue
				}
				if !strings.Contains(raw, marker) {
					t.Errorf(
						"unchecked task %q reports line %d, but raw line is %q",
						it.Text, it.Line, raw,
					)
				}
			}
		})
	}
}

// TestTaskItemLineWriteThrough simulates the docwrite.CheckTask
// consumer (IMPL-0014 Phase 3): byte-splice "[ ]" to "[x]" at each
// reported TaskItem.Line and verify exactly that item flips while
// every other fact is untouched.
func TestTaskItemLineWriteThrough(t *testing.T) {
	t.Parallel()
	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			content := readFixture(t, name)
			items := docparse.TaskItems(content)

			for idx, target := range items {
				if target.Checked {
					continue
				}

				lines := strings.Split(string(content), "\n")
				lines[target.Line-1] = strings.Replace(
					lines[target.Line-1], "[ ]", "[x]", 1,
				)
				after := docparse.TaskItems([]byte(strings.Join(lines, "\n")))

				if len(after) != len(items) {
					t.Fatalf(
						"splicing line %d changed item count: %d != %d",
						target.Line, len(after), len(items),
					)
				}
				for i := range items {
					wantChecked := items[i].Checked || i == idx
					if after[i].Checked != wantChecked {
						t.Errorf(
							"after splicing line %d: item %d Checked = %t, want %t",
							target.Line, i, after[i].Checked, wantChecked,
						)
					}
					if after[i].Text != items[i].Text ||
						after[i].Line != items[i].Line ||
						after[i].Indent != items[i].Indent {
						t.Errorf(
							"after splicing line %d: item %d = %+v, want %+v (Checked aside)",
							target.Line, i, after[i], items[i],
						)
					}
				}
			}
		})
	}
}
