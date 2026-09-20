package docparse_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
)

func readGoldenFixture(t *testing.T, dir, name string) []byte {
	t.Helper()

	content, err := os.ReadFile(filepath.Join("testdata", dir, name+".md"))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	return content
}

func compareGolden(t *testing.T, dir, name, got string) {
	t.Helper()

	goldenPath := filepath.Join("testdata", dir, "golden", name+".golden.txt")

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
		t.Fatalf("reading golden file %s: %v\nRun with -update to create it", goldenPath, err)
	}

	if got != string(want) {
		t.Errorf("facts differ from %s\n--- got ---\n%s\n--- want ---\n%s", goldenPath, got, want)
	}
}

func TestGoldenListItems(t *testing.T) {
	t.Parallel()

	content := readGoldenFixture(t, "listitems", "lists")

	var sb strings.Builder
	for _, it := range docparse.ListItems(content) {
		fmt.Fprintf(
			&sb, "line=%d ordered=%t indent=%d text=%q\n",
			it.Line, it.Ordered, it.Indent, it.Text,
		)
	}

	compareGolden(t, "listitems", "lists", sb.String())
}

func TestGoldenTables(t *testing.T) {
	t.Parallel()

	content := readGoldenFixture(t, "tables", "tables")

	var sb strings.Builder
	for i, tbl := range docparse.Tables(content) {
		fmt.Fprintf(&sb, "table=%d line=%d header=%q\n", i, tbl.Line, tbl.Header)

		for j, row := range tbl.Rows {
			fmt.Fprintf(&sb, "  row=%d %q\n", j, row)
		}
	}

	compareGolden(t, "tables", "tables", sb.String())
}

// TaskItems is documented as the checkbox subset of ListItems' lines.
// If that stops being true, a consumer reading "the bullets in this
// region" and one reading "the tasks" disagree about what is there.
func TestListItems_TaskItemsAreASubset(t *testing.T) {
	t.Parallel()

	for _, fixture := range []struct{ dir, name string }{
		{"listitems", "lists"},
		{"regions", "impl_canonical"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()

			content := readGoldenFixture(t, fixture.dir, fixture.name)

			listLines := make(map[int]bool)
			for _, it := range docparse.ListItems(content) {
				listLines[it.Line] = true
			}

			tasks := docparse.TaskItems(content)
			if len(tasks) == 0 {
				t.Fatal("fixture has no task items, so the subset claim is untested")
			}

			for _, task := range tasks {
				if !listLines[task.Line] {
					t.Errorf("TaskItem at line %d is not a ListItem: %q", task.Line, task.Text)
				}
			}
		})
	}
}

func TestListItems_Table(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want []docparse.ListItem
	}{
		{
			name: "dash star and plus bullets",
			in:   "- a\n* b\n+ c\n",
			want: []docparse.ListItem{
				{Text: "a", Line: 1},
				{Text: "b", Line: 2},
				{Text: "c", Line: 3},
			},
		},
		{
			name: "ordered with dot and paren",
			in:   "1. a\n2) b\n",
			want: []docparse.ListItem{
				{Text: "a", Ordered: true, Line: 1},
				{Text: "b", Ordered: true, Line: 2},
			},
		},
		{
			name: "a checkbox item keeps its marker in Text",
			in:   "- [ ] a\n- [x] b\n",
			want: []docparse.ListItem{
				{Text: "[ ] a", Line: 1},
				{Text: "[x] b", Line: 2},
			},
		},
		{
			name: "indent counts tabs as one",
			in:   "  - a\n\t- b\n",
			want: []docparse.ListItem{
				{Text: "a", Indent: 2, Line: 1},
				{Text: "b", Indent: 1, Line: 2},
			},
		},
		{
			name: "thematic breaks are not items",
			in:   "---\n***\n___\n- - -\n",
			want: nil,
		},
		{
			name: "no whitespace after the marker is not an item",
			in:   "-a\n1.b\n",
			want: nil,
		},
		{
			name: "an empty item has empty text",
			in:   "-\n- \n",
			want: []docparse.ListItem{
				{Text: "", Line: 1},
				{Text: "", Line: 2},
			},
		},
		{
			name: "items in a fence are skipped",
			in:   "```\n- a\n```\n- b\n",
			want: []docparse.ListItem{{Text: "b", Line: 4}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := docparse.ListItems([]byte(tt.in))

			if len(got) != len(tt.want) {
				t.Fatalf("ListItems() = %+v, want %+v", got, tt.want)
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ListItems()[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestTables_Table(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		in         string
		wantCount  int
		wantHeader []string
		wantRows   [][]string
		wantLine   int
	}{
		{
			name:       "outer pipes",
			in:         "| A | B |\n| - | - |\n| 1 | 2 |\n",
			wantCount:  1,
			wantHeader: []string{"A", "B"},
			wantRows:   [][]string{{"1", "2"}},
			wantLine:   1,
		},
		{
			name:       "without outer pipes",
			in:         "A | B\n--- | ---\n1 | 2\n",
			wantCount:  1,
			wantHeader: []string{"A", "B"},
			wantRows:   [][]string{{"1", "2"}},
			wantLine:   1,
		},
		{
			name:       "alignment colons",
			in:         "| A | B | C |\n|:--|:-:|--:|\n| 1 | 2 | 3 |\n",
			wantCount:  1,
			wantHeader: []string{"A", "B", "C"},
			wantRows:   [][]string{{"1", "2", "3"}},
			wantLine:   1,
		},
		{
			name:       "inline markdown is kept and an escaped pipe stays in its cell",
			in:         "| A |\n| - |\n| **b** \\| c |\n",
			wantCount:  1,
			wantHeader: []string{"A"},
			wantRows:   [][]string{{`**b** \| c`}},
			wantLine:   1,
		},
		{
			name:       "ragged rows are reported as written",
			in:         "| A | B |\n| - | - |\n| one |\n| 1 | 2 | 3 |\n",
			wantCount:  1,
			wantHeader: []string{"A", "B"},
			wantRows:   [][]string{{"one"}, {"1", "2", "3"}},
			wantLine:   1,
		},
		{
			name:      "no delimiter row is no table",
			in:        "| A | B |\n| x | y |\n",
			wantCount: 0,
		},
		{
			name:      "prose with a pipe is no table",
			in:        "a | b in a sentence\n\nmore prose\n",
			wantCount: 0,
		},
		{
			name:      "a table inside a fence is skipped",
			in:        "```\n| A |\n| - |\n| 1 |\n```\n",
			wantCount: 0,
		},
		{
			name:       "header with no body rows",
			in:         "| A |\n| - |\n\nprose\n",
			wantCount:  1,
			wantHeader: []string{"A"},
			wantLine:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := docparse.Tables([]byte(tt.in))

			if len(got) != tt.wantCount {
				t.Fatalf("Tables() returned %d tables, want %d: %+v", len(got), tt.wantCount, got)
			}

			if tt.wantCount == 0 {
				return
			}

			if strings.Join(got[0].Header, "|") != strings.Join(tt.wantHeader, "|") {
				t.Errorf("Header = %q, want %q", got[0].Header, tt.wantHeader)
			}

			if got[0].Line != tt.wantLine {
				t.Errorf("Line = %d, want %d", got[0].Line, tt.wantLine)
			}

			if len(got[0].Rows) != len(tt.wantRows) {
				t.Fatalf("Rows = %q, want %q", got[0].Rows, tt.wantRows)
			}

			for i := range got[0].Rows {
				if strings.Join(got[0].Rows[i], "|") != strings.Join(tt.wantRows[i], "|") {
					t.Errorf("Rows[%d] = %q, want %q", i, got[0].Rows[i], tt.wantRows[i])
				}
			}
		})
	}
}

func FuzzListItems(f *testing.F) {
	f.Add([]byte("- a\n"))
	f.Add([]byte("1. a\n  - b\n"))
	f.Add([]byte("---\n"))
	f.Add([]byte("```\n- a\n"))
	f.Add(readSeed(f, "listitems", "lists"))

	f.Fuzz(func(t *testing.T, content []byte) {
		lines := strings.Split(string(content), "\n")

		for _, it := range docparse.ListItems(content) {
			if it.Line < 1 || it.Line > len(lines) {
				t.Fatalf("line %d is outside the input's %d lines: %+v", it.Line, len(lines), it)
			}

			if it.Indent < 0 {
				t.Fatalf("negative indent: %+v", it)
			}

			if it.Text != strings.TrimSpace(it.Text) {
				t.Fatalf("text is not trimmed: %q", it.Text)
			}

			if strings.Contains(it.Text, "\n") {
				t.Fatalf("text spans lines: %q", it.Text)
			}
		}
	})
}

func FuzzTables(f *testing.F) {
	f.Add([]byte("| A |\n| - |\n| 1 |\n"))
	f.Add([]byte("A | B\n--|--\n"))
	f.Add([]byte("|\n|\n"))
	f.Add([]byte("| a \\| b |\n| --- |\n"))
	f.Add(readSeed(f, "tables", "tables"))

	f.Fuzz(func(t *testing.T, content []byte) {
		lines := strings.Split(string(content), "\n")

		for _, tbl := range docparse.Tables(content) {
			if tbl.Line < 1 || tbl.Line > len(lines) {
				t.Fatalf("line %d is outside the input's %d lines", tbl.Line, len(lines))
			}

			if len(tbl.Header) == 0 {
				t.Fatalf("table at line %d has no header cells", tbl.Line)
			}

			for _, cell := range tbl.Header {
				if cell != strings.TrimSpace(cell) {
					t.Fatalf("header cell is not trimmed: %q", cell)
				}
			}

			for _, row := range tbl.Rows {
				for _, cell := range row {
					if cell != strings.TrimSpace(cell) {
						t.Fatalf("row cell is not trimmed: %q", cell)
					}
				}
			}
		}
	})
}

func readSeed(f *testing.F, dir, name string) []byte {
	f.Helper()

	content, err := os.ReadFile(filepath.Join("testdata", dir, name+".md"))
	if err != nil {
		f.Fatalf("seeding from %s/%s: %v", dir, name, err)
	}

	return content
}
