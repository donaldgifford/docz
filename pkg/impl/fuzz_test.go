package impl_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/impl"
)

// FuzzParse pins the total contract: Parse returns a Doc or an error and
// never panics, whatever markdown it is handed.
//
// It also pins the invariants a consumer relies on without being able to
// check them — that a task's Line is inside its document, that EndLine is
// never before Line, and that a task ID is unique — because those are what
// make a Doc safe to hand to docwrite. A parser that returned a Line past the
// end of the file would make a splice write into nothing.
func FuzzParse(f *testing.F) {
	seeds, err := filepath.Glob(filepath.Join("testdata", "*.md"))
	if err != nil {
		f.Fatalf("globbing seeds: %v", err)
	}

	for _, name := range seeds {
		content, err := os.ReadFile(name)
		if err != nil {
			f.Fatalf("reading seed: %v", err)
		}

		f.Add(content)
		f.Add(unmark(content))
	}

	// The shapes that are all grammar and no prose, where the interactions
	// live.
	f.Add([]byte("---\nid: I-1\n---\n\n### Phase 1: A\n\n#### Tasks\n\n- [ ] a\n"))
	f.Add([]byte("---\n---\n### Phase :\n#### Tasks\n- [x] ~~a~~ — skipped:\n"))
	f.Add([]byte("---\n---\n### Phase 1:\n#### Tasks\n- [ ] a\n      verify:\n"))
	f.Add([]byte("---\n---\n### Phase 1: a\n#### Tasks\n- [ ] deferred\n"))

	f.Fuzz(func(t *testing.T, content []byte) {
		got, err := impl.Parse(content)
		if err != nil {
			return
		}

		total := strings.Count(string(content), "\n") + 1
		seen := make(map[string]bool)

		for _, phase := range got.Phases {
			if phase.Line < 1 || phase.Line > total {
				t.Fatalf("phase %q line %d outside a %d-line document",
					phase.Token, phase.Line, total)
			}

			for _, task := range phase.Tasks {
				if task.Line < 1 || task.Line > total {
					t.Fatalf("task %s line %d outside a %d-line document",
						task.ID, task.Line, total)
				}

				if task.EndLine < task.Line {
					t.Fatalf("task %s EndLine %d before Line %d",
						task.ID, task.EndLine, task.Line)
				}

				if seen[task.ID] {
					t.Fatalf("task ID %s is used twice", task.ID)
				}

				seen[task.ID] = true

				// Text carries the task, not its state: a marker's own words
				// are in the marker.
				if task.Skipped != nil && strings.Contains(task.Text, "~~") {
					t.Fatalf("task %s keeps its strikethrough: %q", task.ID, task.Text)
				}
			}
		}

		// Progress can only count tasks that exist, and only the ones that
		// could be checked.
		done, progress := got.Progress()
		if done > progress || progress > len(got.Tasks()) {
			t.Fatalf("Progress() = %d/%d over %d tasks", done, progress, len(got.Tasks()))
		}
	})
}
