package docwrite

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// checkTaskDoc is a small task list exercising bullets, indentation, and
// checked states. Line numbers are load-bearing in the tests below.
const checkTaskDoc = `# Doc

- [ ] first task
- [x] already done
  - [ ] nested two spaces
* [ ] star bullet
- [-] tri-state, not a task
- plain list item
prose line
- [X] shouted done
`

func TestCheckTask(t *testing.T) {
	t.Parallel()

	t.Run("flips exactly one byte", func(t *testing.T) {
		t.Parallel()
		p := writeTemp(t, checkTaskDoc)

		if err := CheckTask(p, 3); err != nil {
			t.Fatalf("CheckTask() error = %v", err)
		}

		got, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		want := strings.Replace(checkTaskDoc, "- [ ] first task", "- [x] first task", 1)
		if string(got) != want {
			t.Errorf("file after CheckTask:\n%s\nwant:\n%s", got, want)
		}

		// Byte-diff accounting: exactly one byte differs from the input.
		diffs := 0
		for i := range got {
			if got[i] != checkTaskDoc[i] {
				diffs++
			}
		}
		if len(got) != len(checkTaskDoc) || diffs != 1 {
			t.Errorf("diff = %d bytes (len %d vs %d), want exactly 1 byte",
				diffs, len(got), len(checkTaskDoc))
		}
	})

	t.Run("indented and star bullets accepted", func(t *testing.T) {
		t.Parallel()
		for _, line := range []int{5, 6} {
			p := writeTemp(t, checkTaskDoc)
			if err := CheckTask(p, line); err != nil {
				t.Errorf("CheckTask(line %d) error = %v", line, err)
			}
		}
	})

	t.Run("already checked", func(t *testing.T) {
		t.Parallel()
		for _, line := range []int{4, 10} { // [x] and [X]
			p := writeTemp(t, checkTaskDoc)
			err := CheckTask(p, line)
			if !errors.Is(err, ErrTaskAlreadyChecked) {
				t.Errorf("CheckTask(line %d) error = %v, want ErrTaskAlreadyChecked", line, err)
			}
		}
	})

	t.Run("not a task item", func(t *testing.T) {
		t.Parallel()
		// Blank line (2), tri-state (7), plain bullet (8), prose (9),
		// and the trailing empty line after the final newline (11).
		for _, line := range []int{2, 7, 8, 9, 11} {
			p := writeTemp(t, checkTaskDoc)
			err := CheckTask(p, line)
			if !errors.Is(err, ErrNotTaskItem) {
				t.Errorf("CheckTask(line %d) error = %v, want ErrNotTaskItem", line, err)
			}
		}
	})

	t.Run("line out of range", func(t *testing.T) {
		t.Parallel()
		for _, line := range []int{0, -3, 12, 100} {
			p := writeTemp(t, checkTaskDoc)
			err := CheckTask(p, line)
			if !errors.Is(err, ErrLineOutOfRange) {
				t.Errorf("CheckTask(line %d) error = %v, want ErrLineOutOfRange", line, err)
			}
		}
	})

	t.Run("errors leave the file untouched", func(t *testing.T) {
		t.Parallel()
		p := writeTemp(t, checkTaskDoc)
		for _, line := range []int{0, 2, 4, 100} {
			if err := CheckTask(p, line); err == nil {
				t.Fatalf("CheckTask(line %d) error = nil, want error", line)
			}
		}
		got, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != checkTaskDoc {
			t.Error("file changed despite CheckTask errors")
		}
	})

	t.Run("rejects CRLF", func(t *testing.T) {
		t.Parallel()
		p := writeTemp(t, "- [ ] task\r\n")
		err := CheckTask(p, 1)
		if !errors.Is(err, ErrUnsupportedLineEndings) {
			t.Errorf("error = %v, want ErrUnsupportedLineEndings", err)
		}
	})

	t.Run("missing file wraps path", func(t *testing.T) {
		t.Parallel()
		err := CheckTask("/nonexistent/doc.md", 1)
		if err == nil {
			t.Fatal("error = nil, want read error")
		}
		if !strings.Contains(err.Error(), "/nonexistent/doc.md") {
			t.Errorf("error %q does not mention the path", err)
		}
	})

	t.Run("sentinels are distinguishable", func(t *testing.T) {
		t.Parallel()
		p := writeTemp(t, checkTaskDoc)
		err := CheckTask(p, 4)
		if errors.Is(err, ErrNotTaskItem) || errors.Is(err, ErrLineOutOfRange) {
			t.Errorf("ErrTaskAlreadyChecked matched a sibling sentinel: %v", err)
		}
	})

	t.Run("fixture line inside a code fence is not a task item", func(t *testing.T) {
		t.Parallel()
		// Line 74 of the golden input fixture is "// - [ ] this
		// checkbox is inside a fence" — a comment inside a Go fence.
		// docparse never reports it, and CheckTask's line-shape
		// validation rejects it too.
		data, err := os.ReadFile(
			filepath.Join("testdata", "golden", "checktask", "impl.input.md"),
		)
		if err != nil {
			t.Fatal(err)
		}
		p := writeTemp(t, string(data))
		if err := CheckTask(p, 74); !errors.Is(err, ErrNotTaskItem) {
			t.Errorf("CheckTask(fenced line) error = %v, want ErrNotTaskItem", err)
		}
	})

	t.Run("no trailing newline final line", func(t *testing.T) {
		t.Parallel()
		p := writeTemp(t, "# Doc\n- [ ] last")
		if err := CheckTask(p, 2); err != nil {
			t.Fatalf("CheckTask() error = %v", err)
		}
		got, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "# Doc\n- [x] last" {
			t.Errorf("file = %q, want %q", got, "# Doc\n- [x] last")
		}
	})
}

// TestCheckTask_Golden flips one task in a template-conformant IMPL
// fixture and byte-compares the on-disk result against a committed
// golden file, asserting the diff touches exactly one line and is
// byte-identical elsewhere. Regenerate with `go test -run
// TestCheckTask_Golden -update ./pkg/doczcore/docwrite/...`.
func TestCheckTask_Golden(t *testing.T) {
	t.Parallel()

	const targetLine = 51 // "- [ ] Write unit tests for `Widget`"

	inputPath := filepath.Join("testdata", "golden", "checktask", "impl.input.md")
	input, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatalf("reading input fixture %s: %v", inputPath, err)
	}
	tmp := writeTemp(t, string(input))

	if err := CheckTask(tmp, targetLine); err != nil {
		t.Fatalf("CheckTask(%s, %d) error: %v", inputPath, targetLine, err)
	}

	got, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatalf("reading mutated file: %v", err)
	}

	goldenPath := filepath.Join("testdata", "golden", "checktask", "impl.output.md")
	if *update {
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatalf("writing golden file %s: %v", goldenPath, err)
		}
		t.Log("updated golden file:", goldenPath)
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("reading golden file %s: %v\nRun with -update to create it",
			goldenPath, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("output differs from golden %s\nRun with -update to update", goldenPath)
	}

	// Line-level diff accounting against the input: exactly one line
	// changed, and it is the target line.
	inLines := strings.Split(string(input), "\n")
	outLines := strings.Split(string(got), "\n")
	if len(inLines) != len(outLines) {
		t.Fatalf("line count changed: %d -> %d", len(inLines), len(outLines))
	}
	var changed []int
	for i := range inLines {
		if inLines[i] != outLines[i] {
			changed = append(changed, i+1)
		}
	}
	if len(changed) != 1 || changed[0] != targetLine {
		t.Errorf("changed lines = %v, want exactly [%d]", changed, targetLine)
	}
}
