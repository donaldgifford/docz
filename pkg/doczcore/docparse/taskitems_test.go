package docparse_test

import (
	"testing"

	"github.com/donaldgifford/docz/pkg/doczcore/docparse"
)

func TestTaskItems(t *testing.T) {
	t.Parallel()
	t.Run("basic checked and unchecked with line numbers", func(t *testing.T) {
		t.Parallel()
		content := []byte("# Title\n" +
			"\n" +
			"- [ ] first task\n" +
			"- [x] second task\n")

		items := docparse.TaskItems(content)
		if len(items) != 2 {
			t.Fatalf("got %d items, want 2", len(items))
		}
		want := []docparse.TaskItem{
			{Text: "first task", Checked: false, Indent: 0, Line: 3},
			{Text: "second task", Checked: true, Indent: 0, Line: 4},
		}
		for i := range want {
			if items[i] != want[i] {
				t.Errorf("items[%d] = %+v, want %+v", i, items[i], want[i])
			}
		}
	})

	t.Run("uppercase X is checked", func(t *testing.T) {
		t.Parallel()
		items := docparse.TaskItems([]byte("- [X] shouted done\n"))
		if len(items) != 1 {
			t.Fatalf("got %d items, want 1", len(items))
		}
		if !items[0].Checked {
			t.Error("Checked = false, want true for [X]")
		}
	})

	t.Run("star bullets", func(t *testing.T) {
		t.Parallel()
		items := docparse.TaskItems([]byte("* [ ] star task\n"))
		if len(items) != 1 {
			t.Fatalf("got %d items, want 1", len(items))
		}
		if items[0].Text != "star task" {
			t.Errorf("Text = %q, want %q", items[0].Text, "star task")
		}
	})

	t.Run("indent counts leading whitespace characters", func(t *testing.T) {
		t.Parallel()
		content := []byte("- [ ] top\n" +
			"  - [ ] two spaces\n" +
			"    - [ ] four spaces\n" +
			"\t- [ ] one tab\n")

		items := docparse.TaskItems(content)
		if len(items) != 4 {
			t.Fatalf("got %d items, want 4", len(items))
		}
		for i, want := range []int{0, 2, 4, 1} {
			if items[i].Indent != want {
				t.Errorf(
					"items[%d].Indent = %d, want %d (tabs count as one, no expansion)",
					i, items[i].Indent, want,
				)
			}
		}
	})

	t.Run("inline markdown preserved in text", func(t *testing.T) {
		t.Parallel()
		items := docparse.TaskItems([]byte("- [ ] add `CheckTask` to **docwrite**\n"))
		if len(items) != 1 {
			t.Fatalf("got %d items, want 1", len(items))
		}
		if want := "add `CheckTask` to **docwrite**"; items[0].Text != want {
			t.Errorf("Text = %q, want %q (inline markdown must survive)", items[0].Text, want)
		}
	})

	t.Run("empty task text", func(t *testing.T) {
		t.Parallel()
		content := []byte("- [ ]\n- [ ] \n")
		items := docparse.TaskItems(content)
		if len(items) != 2 {
			t.Fatalf("got %d items, want 2", len(items))
		}
		for i := range items {
			if items[i].Text != "" {
				t.Errorf("items[%d].Text = %q, want empty", i, items[i].Text)
			}
		}
	})

	t.Run("non-task lines skipped", func(t *testing.T) {
		t.Parallel()
		content := []byte("- plain list item\n" +
			"- [-] tri-state marker\n" +
			"- [] empty brackets\n" +
			"- [x]no space after checkbox\n" +
			"-[ ] no space after bullet\n" +
			"+ [ ] plus bullet\n" +
			"[ ] no bullet at all\n" +
			"- [xx] two chars\n")

		if items := docparse.TaskItems(content); len(items) != 0 {
			t.Fatalf("got %d items, want 0: %+v", len(items), items)
		}
	})

	t.Run("checkboxes inside code fences skipped", func(t *testing.T) {
		t.Parallel()
		content := []byte("- [ ] real\n" +
			"```markdown\n" +
			"- [ ] example inside fence\n" +
			"```\n" +
			"- [x] also real\n")

		items := docparse.TaskItems(content)
		if len(items) != 2 {
			t.Fatalf("got %d items, want 2", len(items))
		}
		if items[0].Line != 1 || items[1].Line != 5 {
			t.Errorf(
				"Lines = %d, %d, want 1, 5",
				items[0].Line, items[1].Line,
			)
		}
	})

	t.Run("multiple spaces between bullet and checkbox", func(t *testing.T) {
		t.Parallel()
		items := docparse.TaskItems([]byte("-   [ ] roomy\n"))
		if len(items) != 1 {
			t.Fatalf("got %d items, want 1", len(items))
		}
	})

	t.Run("trailing whitespace trimmed from text", func(t *testing.T) {
		t.Parallel()
		items := docparse.TaskItems([]byte("- [x] done  \n"))
		if len(items) != 1 {
			t.Fatalf("got %d items, want 1", len(items))
		}
		if items[0].Text != "done" {
			t.Errorf("Text = %q, want %q", items[0].Text, "done")
		}
	})

	t.Run("no trailing newline", func(t *testing.T) {
		t.Parallel()
		items := docparse.TaskItems([]byte("- [ ] last line"))
		if len(items) != 1 {
			t.Fatalf("got %d items, want 1", len(items))
		}
		if items[0].Line != 1 {
			t.Errorf("Line = %d, want 1", items[0].Line)
		}
	})

	t.Run("empty content", func(t *testing.T) {
		t.Parallel()
		if items := docparse.TaskItems(nil); len(items) != 0 {
			t.Fatalf("got %d items, want 0", len(items))
		}
	})
}
