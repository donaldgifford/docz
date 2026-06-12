package document

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "update golden files")

// writeTemp writes content to a fresh file under t.TempDir() and returns
// its path. SetStatus mutates files in place, so every test operates on a
// throwaway copy.
func writeTemp(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "doc.md")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	return p
}

// TestSetStatus_Golden round-trips each built-in template's create output
// through a status mutation and byte-compares the result against a
// committed fixture. Regenerate with `go test -run TestSetStatus_Golden
// -update ./internal/document/...`.
func TestSetStatus_Golden(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string // fixture stem == built-in type
		newStatus string
		wantOld   string
	}{
		{name: "rfc", newStatus: "Accepted", wantOld: "Draft"},
		{name: "adr", newStatus: "Accepted", wantOld: "Proposed"},
		{name: "design", newStatus: "Implemented", wantOld: "Draft"},
		{name: "impl", newStatus: "Completed", wantOld: "Draft"},
		{name: "plan", newStatus: "Completed", wantOld: "Draft"},
		{name: "investigation", newStatus: "Concluded", wantOld: "Open"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			inputPath := filepath.Join("testdata", "golden", "status", tc.name+".input.md")
			data, err := os.ReadFile(inputPath)
			if err != nil {
				t.Fatalf("reading input fixture %s: %v", inputPath, err)
			}
			tmp := writeTemp(t, string(data))

			gotOld, err := SetStatus(tmp, tc.newStatus)
			if err != nil {
				t.Fatalf("SetStatus(%s, %q) error: %v", inputPath, tc.newStatus, err)
			}
			if gotOld != tc.wantOld {
				t.Errorf("SetStatus(%s, %q) old = %q, want %q",
					inputPath, tc.newStatus, gotOld, tc.wantOld)
			}

			got, err := os.ReadFile(tmp)
			if err != nil {
				t.Fatalf("reading mutated file: %v", err)
			}

			goldenPath := filepath.Join("testdata", "golden", "status", tc.name+".output.md")
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
				t.Errorf("output differs from golden %s\n got: %q\nwant: %q\nRun with -update to update",
					goldenPath, got, want)
			}
		})
	}
}

// TestSetStatus_Shapes covers the quoting, spacing, comment, and key-order
// variants the byte-level mutator must preserve.
func TestSetStatus_Shapes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		newStatus string
		wantOld   string
		want      string
	}{
		{
			name:      "bare value",
			input:     "---\nstatus: Draft\n---\nbody\n",
			newStatus: "Accepted",
			wantOld:   "Draft",
			want:      "---\nstatus: Accepted\n---\nbody\n",
		},
		{
			name:      "double quoted",
			input:     "---\nstatus: \"Draft\"\n---\nbody\n",
			newStatus: "Accepted",
			wantOld:   "Draft",
			want:      "---\nstatus: \"Accepted\"\n---\nbody\n",
		},
		{
			name:      "single quoted",
			input:     "---\nstatus: 'Draft'\n---\nbody\n",
			newStatus: "Accepted",
			wantOld:   "Draft",
			want:      "---\nstatus: 'Accepted'\n---\nbody\n",
		},
		{
			name:      "no space after colon",
			input:     "---\nstatus:Draft\n---\nbody\n",
			newStatus: "Accepted",
			wantOld:   "Draft",
			want:      "---\nstatus:Accepted\n---\nbody\n",
		},
		{
			name:      "extra spaces after colon",
			input:     "---\nstatus:   Draft\n---\nbody\n",
			newStatus: "Accepted",
			wantOld:   "Draft",
			want:      "---\nstatus:   Accepted\n---\nbody\n",
		},
		{
			name:      "trailing comment preserved",
			input:     "---\nstatus: Draft  # current\n---\nbody\n",
			newStatus: "Accepted",
			wantOld:   "Draft",
			want:      "---\nstatus: Accepted  # current\n---\nbody\n",
		},
		{
			name:      "multi-word existing value",
			input:     "---\nstatus: In Review\n---\nbody\n",
			newStatus: "Approved",
			wantOld:   "In Review",
			want:      "---\nstatus: Approved\n---\nbody\n",
		},
		{
			name:      "multi-word new value stays bare",
			input:     "---\nstatus: Draft\n---\nbody\n",
			newStatus: "In Review",
			wantOld:   "Draft",
			want:      "---\nstatus: In Review\n---\nbody\n",
		},
		{
			name:      "leading blank line",
			input:     "\n---\nstatus: Draft\n---\nbody\n",
			newStatus: "Accepted",
			wantOld:   "Draft",
			want:      "\n---\nstatus: Accepted\n---\nbody\n",
		},
		{
			name:      "status not first key",
			input:     "---\nid: RFC-0001\ntitle: \"X\"\nstatus: Draft\nauthor: A\n---\nbody\n",
			newStatus: "Accepted",
			wantOld:   "Draft",
			want:      "---\nid: RFC-0001\ntitle: \"X\"\nstatus: Accepted\nauthor: A\n---\nbody\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := writeTemp(t, tt.input)

			gotOld, err := SetStatus(p, tt.newStatus)
			if err != nil {
				t.Fatalf("SetStatus(%q) error: %v", tt.input, err)
			}
			if gotOld != tt.wantOld {
				t.Errorf("SetStatus(%q) old = %q, want %q", tt.input, gotOld, tt.wantOld)
			}

			got, err := os.ReadFile(p)
			if err != nil {
				t.Fatalf("reading mutated file: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("SetStatus(%q) wrote %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestSetStatus_Idempotent proves that setting the value a file already
// holds produces byte-identical output, since the helper writes
// unconditionally (DESIGN-0005 Decision 8).
func TestSetStatus_Idempotent(t *testing.T) {
	t.Parallel()

	input := "---\nstatus: Draft\n---\nbody\n"
	p := writeTemp(t, input)

	gotOld, err := SetStatus(p, "Draft")
	if err != nil {
		t.Fatalf("SetStatus error: %v", err)
	}
	if gotOld != "Draft" {
		t.Errorf("old status = %q, want %q", gotOld, "Draft")
	}

	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != input {
		t.Errorf("idempotent write changed bytes: got %q, want %q", got, input)
	}
}

// TestSetStatus_Errors covers the sentinel error paths. None of them
// should write to the file.
func TestSetStatus_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		{
			name:    "crlf endings",
			input:   "---\r\nstatus: Draft\r\n---\r\nbody\r\n",
			wantErr: ErrUnsupportedLineEndings,
		},
		{
			name:    "lone cr endings",
			input:   "---\rstatus: Draft\r---\rbody\r",
			wantErr: ErrUnsupportedLineEndings,
		},
		{
			name:    "missing status key",
			input:   "---\nid: RFC-0001\ntitle: \"X\"\n---\nbody\n",
			wantErr: ErrStatusFieldMissing,
		},
		{
			name:    "block scalar value",
			input:   "---\nstatus: |\n  Draft\n---\nbody\n",
			wantErr: ErrStatusFieldMissing,
		},
		{
			name:    "flow mapping value",
			input:   "---\nstatus: {state: draft}\n---\nbody\n",
			wantErr: ErrStatusFieldMissing,
		},
		{
			name:    "unterminated quote",
			input:   "---\nstatus: \"Draft\n---\nbody\n",
			wantErr: ErrStatusFieldMissing,
		},
		{
			name:    "no frontmatter",
			input:   "just a plain markdown file\nwith no frontmatter\n",
			wantErr: ErrNoFrontmatter,
		},
		{
			name:    "false closing delimiter",
			input:   "---\nstatus: Draft\n---more text here\n",
			wantErr: ErrNoFrontmatter,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := writeTemp(t, tt.input)

			gotOld, err := SetStatus(p, "Accepted")
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("SetStatus(%q) error = %v, want %v", tt.input, err, tt.wantErr)
			}
			if gotOld != "" {
				t.Errorf("SetStatus(%q) old = %q, want empty on error", tt.input, gotOld)
			}

			got, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.input {
				t.Errorf("file mutated on error path: got %q, want %q", got, tt.input)
			}
		})
	}
}

// TestSetStatus_ReadError surfaces a wrapped, path-qualified IO error for a
// missing file (Decision 5).
func TestSetStatus_ReadError(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "does-not-exist.md")
	_, err := SetStatus(missing, "Accepted")
	if err == nil {
		t.Fatal("SetStatus on missing file = nil error, want non-nil")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("error = %v, want wrapped os.ErrNotExist", err)
	}
}
