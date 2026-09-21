package docwrite_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/document"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/docwrite"
)

// The byte cores are what a consumer holding bytes it fetched calls, with no
// filesystem anywhere (DESIGN-0014 §2.7). The path-taking wrappers are read,
// core, write — and the existing golden files already cover those, running
// through the core unchanged. What is left to pin here is the core's own
// contract: the splice, the sentinels, and that the input is not modified.

const statusDoc = `---
id: RFC-0001
title: A document
status: Draft
author: Test
created: 2026-09-20
---

# RFC-0001: A document
`

func TestSetStatusBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		doc    string
		status string
		want   string
		old    string
		err    error
	}{
		{
			name:   "bare scalar",
			doc:    "---\nstatus: Draft\n---\n",
			status: "Accepted",
			want:   "---\nstatus: Accepted\n---\n",
			old:    "Draft",
		},
		{
			name:   "double-quoted value keeps its quotes",
			doc:    "---\nstatus: \"Draft\"\n---\n",
			status: "Accepted",
			want:   "---\nstatus: \"Accepted\"\n---\n",
			old:    "Draft",
		},
		{
			name:   "single-quoted value keeps its quotes",
			doc:    "---\nstatus: 'Draft'\n---\n",
			status: "Accepted",
			want:   "---\nstatus: 'Accepted'\n---\n",
			old:    "Draft",
		},
		{
			name:   "a trailing comment survives",
			doc:    "---\nstatus: Draft  # why\n---\n",
			status: "Accepted",
			want:   "---\nstatus: Accepted  # why\n---\n",
			old:    "Draft",
		},
		{
			name:   "unusual spacing survives",
			doc:    "---\nstatus:\t\tDraft\n---\n",
			status: "Accepted",
			want:   "---\nstatus:\t\tAccepted\n---\n",
			old:    "Draft",
		},
		{
			name:   "a status with spaces in it",
			doc:    "---\nstatus: In Review\n---\n",
			status: "Approved",
			want:   "---\nstatus: Approved\n---\n",
			old:    "In Review",
		},
		{
			name:   "an empty value is a value",
			doc:    "---\nstatus:\n---\n",
			status: "Draft",
			want:   "---\nstatus:Draft\n---\n",
			old:    "",
		},
		{
			name:   "the same value writes the same bytes",
			doc:    statusDoc,
			status: "Draft",
			want:   statusDoc,
			old:    "Draft",
		},
		{
			name:   "no frontmatter",
			doc:    "# A document\n\nstatus: Draft\n",
			status: "Accepted",
			err:    document.ErrNoFrontmatter,
		},
		{
			name:   "no status key",
			doc:    "---\nid: RFC-0001\n---\n",
			status: "Accepted",
			err:    docwrite.ErrStatusFieldMissing,
		},
		{
			name:   "a block scalar is refused",
			doc:    "---\nstatus: |\n  Draft\n---\n",
			status: "Accepted",
			err:    docwrite.ErrStatusFieldMissing,
		},
		{
			name:   "CRLF is refused",
			doc:    "---\r\nstatus: Draft\r\n---\r\n",
			status: "Accepted",
			err:    docwrite.ErrUnsupportedLineEndings,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, old, err := docwrite.SetStatusBytes([]byte(tt.doc), tt.status)

			if tt.err != nil {
				if !errors.Is(err, tt.err) {
					t.Fatalf("error = %v, want %v", err, tt.err)
				}

				if got != nil {
					t.Errorf("output = %q on error, want nil", got)
				}

				return
			}

			if err != nil {
				t.Fatalf("SetStatusBytes: %v", err)
			}

			if string(got) != tt.want {
				t.Errorf("output =\n%q\nwant\n%q", got, tt.want)
			}

			if old != tt.old {
				t.Errorf("old = %q, want %q", old, tt.old)
			}
		})
	}
}

// TestSetStatusBytes_DoesNotModifyItsInput is the contract that makes the core
// safe for a consumer that keeps the bytes it fetched.
func TestSetStatusBytes_DoesNotModifyItsInput(t *testing.T) {
	t.Parallel()

	in := []byte(statusDoc)

	if _, _, err := docwrite.SetStatusBytes(in, "Accepted"); err != nil {
		t.Fatalf("SetStatusBytes: %v", err)
	}

	if string(in) != statusDoc {
		t.Errorf("the input was modified:\n%q", in)
	}
}

const taskDoc = `# A plan

- [ ] first
- [x] second
- [ ] third
`

func TestSetTaskStateBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		doc     string
		line    int
		checked bool
		want    string
		err     error
	}{
		{
			name:    "check an unchecked task",
			doc:     taskDoc,
			line:    3,
			checked: true,
			want:    "# A plan\n\n- [x] first\n- [x] second\n- [ ] third\n",
		},
		{
			name:    "uncheck a checked task",
			doc:     taskDoc,
			line:    4,
			checked: false,
			want:    "# A plan\n\n- [ ] first\n- [ ] second\n- [ ] third\n",
		},
		{
			name:    "an indented nested task",
			doc:     "- [ ] parent\n  - [x] child\n",
			line:    2,
			checked: false,
			want:    "- [ ] parent\n  - [ ] child\n",
		},
		{
			name:    "a star bullet",
			doc:     "* [ ] starred\n",
			line:    1,
			checked: true,
			want:    "* [x] starred\n",
		},
		{
			name:    "an upper-case X is checked",
			doc:     "- [X] loud\n",
			line:    1,
			checked: false,
			want:    "- [ ] loud\n",
		},
		{
			// Written lower-case whatever it was, because that is what every
			// template and every document in the corpus writes.
			name:    "checking writes a lower-case x",
			doc:     "- [ ] quiet\n",
			line:    1,
			checked: true,
			want:    "- [x] quiet\n",
		},
		{
			name:    "already checked",
			doc:     taskDoc,
			line:    4,
			checked: true,
			err:     docwrite.ErrTaskAlreadyChecked,
		},
		{
			name:    "already unchecked",
			doc:     taskDoc,
			line:    3,
			checked: false,
			err:     docwrite.ErrTaskAlreadyUnchecked,
		},
		{
			name:    "not a task item",
			doc:     taskDoc,
			line:    1,
			checked: true,
			err:     docwrite.ErrNotTaskItem,
		},
		{
			name:    "line zero",
			doc:     taskDoc,
			line:    0,
			checked: true,
			err:     docwrite.ErrLineOutOfRange,
		},
		{
			name:    "past the end",
			doc:     taskDoc,
			line:    99,
			checked: true,
			err:     docwrite.ErrLineOutOfRange,
		},
		{
			name:    "CRLF is refused",
			doc:     "- [ ] first\r\n",
			line:    1,
			checked: true,
			err:     docwrite.ErrUnsupportedLineEndings,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := docwrite.SetTaskStateBytes([]byte(tt.doc), tt.line, tt.checked)

			if tt.err != nil {
				if !errors.Is(err, tt.err) {
					t.Fatalf("error = %v, want %v", err, tt.err)
				}

				if got != nil {
					t.Errorf("output = %q on error, want nil", got)
				}

				return
			}

			if err != nil {
				t.Fatalf("SetTaskStateBytes: %v", err)
			}

			if string(got) != tt.want {
				t.Errorf("output =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

// TestSetTaskStateBytes_DoesNotModifyItsInput matters more here than for
// status: the splice is a single byte into an existing slice, so the obvious
// implementation would write straight through the caller's bytes.
func TestSetTaskStateBytes_DoesNotModifyItsInput(t *testing.T) {
	t.Parallel()

	in := []byte(taskDoc)

	if _, err := docwrite.SetTaskStateBytes(in, 3, true); err != nil {
		t.Fatalf("SetTaskStateBytes: %v", err)
	}

	if string(in) != taskDoc {
		t.Errorf("the input was modified:\n%q", in)
	}
}

// TestSetTaskState_RoundTrip pins that the two directions are inverses at the
// byte level, which is what makes a consumer able to undo a check without
// remembering what the line looked like.
func TestSetTaskState_RoundTrip(t *testing.T) {
	t.Parallel()

	checked, err := docwrite.SetTaskStateBytes([]byte(taskDoc), 3, true)
	if err != nil {
		t.Fatalf("checking: %v", err)
	}

	back, err := docwrite.SetTaskStateBytes(checked, 3, false)
	if err != nil {
		t.Fatalf("unchecking: %v", err)
	}

	if string(back) != taskDoc {
		t.Errorf("round trip gave\n%q\nwant\n%q", back, taskDoc)
	}
}

// TestCheckTaskIsSetTaskState pins that the older name still means the same
// thing, since IMPL-0011's callers use it.
func TestCheckTaskIsSetTaskState(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "plan.md")

	if err := os.WriteFile(path, []byte(taskDoc), config.FileMode); err != nil {
		t.Fatal(err)
	}

	if err := docwrite.CheckTask(path, 3); err != nil {
		t.Fatalf("CheckTask: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	want, err := docwrite.SetTaskStateBytes([]byte(taskDoc), 3, true)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(got, want) {
		t.Errorf("CheckTask wrote\n%q\nwant\n%q", got, want)
	}
}

// TestRenderEqualsWhatCreateWrites is the proof that the split changed
// nothing: the bytes a consumer renders and the bytes docz writes are the same
// bytes, or the two paths have drifted.
func TestRenderEqualsWhatCreateWrites(t *testing.T) {
	t.Parallel()

	for _, name := range config.DocTypeNames() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			typ := config.DefaultConfig().Types[name]

			opts := &docwrite.CreateOptions{
				Type:      config.DocType(name),
				Title:     "A placeholder title",
				Author:    "Test Author",
				Status:    typ.Statuses[0],
				Prefix:    typ.IDPrefix,
				IDWidth:   4,
				DocsDir:   dir,
				TypeDir:   typ.Dir,
				CreatedAt: time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC),
			}

			result, err := docwrite.Create(opts)
			if err != nil {
				t.Fatalf("Create: %v", err)
			}

			written, err := os.ReadFile(result.FilePath)
			if err != nil {
				t.Fatal(err)
			}

			rendered, err := docwrite.Render(opts, result.Number)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}

			if rendered.Filename != result.Filename {
				t.Errorf("Render filename = %q, Create wrote %q",
					rendered.Filename, result.Filename)
			}

			if !bytes.Equal(rendered.Content, written) {
				t.Errorf("Render and Create disagree; first difference:\n%s",
					firstDiff(string(rendered.Content), string(written)))
			}
		})
	}
}

// TestNextNumber counts from what a directory already holds.
func TestNextNumber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		files []string
		width int
		want  string
	}{
		{name: "an empty directory starts at one", width: 4, want: "0001"},
		{
			name:  "one past the highest, not one past the count",
			files: []string{"0001-a.md", "0007-b.md"},
			width: 4,
			want:  "0008",
		},
		{
			name:  "a gap is not filled",
			files: []string{"0001-a.md", "0003-c.md"},
			width: 4,
			want:  "0004",
		},
		{
			name:  "files that are not documents are ignored",
			files: []string{"README.md", "notes.txt", "0002-b.md"},
			width: 4,
			want:  "0003",
		},
		{
			name:  "the width is the configured one",
			files: []string{"0001-a.md"},
			width: 2,
			want:  "02",
		},
		{
			// A number wider than the pad is not truncated: an ID is an
			// address, and cutting it would collide with another document.
			name:  "a number wider than the pad keeps its digits",
			files: []string{"12345-a.md"},
			width: 4,
			want:  "12346",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()

			for _, name := range tt.files {
				if err := os.WriteFile(filepath.Join(dir, name),
					[]byte("---\n---\n"), config.FileMode); err != nil {
					t.Fatal(err)
				}
			}

			got, err := docwrite.NextNumber(dir, tt.width)
			if err != nil {
				t.Fatalf("NextNumber: %v", err)
			}

			if got != tt.want {
				t.Errorf("NextNumber = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestNextNumber_MissingDirectory starts at one rather than failing: a repo's
// first document of a type is created before the type's directory exists.
func TestNextNumber_MissingDirectory(t *testing.T) {
	t.Parallel()

	got, err := docwrite.NextNumber(filepath.Join(t.TempDir(), "nope"), 4)
	if err != nil {
		t.Fatalf("NextNumber: %v", err)
	}

	if got != "0001" {
		t.Errorf("NextNumber = %q, want 0001", got)
	}
}

// firstDiff names the first line two documents differ on, which is enough to
// see what moved without printing a whole rendered template.
func firstDiff(got, want string) string {
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

	return "(they differ only in length)"
}
