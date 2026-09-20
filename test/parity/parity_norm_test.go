package parity

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRootNormalizer(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	n := RootNormalizer(root)

	if n.Name != "root" {
		t.Errorf("Name = %q, want root", n.Name)
	}

	in := "wrote " + filepath.Join(root, "docs", "rfc", "README.md")
	want := "wrote $ROOT/docs/rfc/README.md"

	if got := n.Apply(in); got != want {
		t.Errorf("Apply(%q) = %q, want %q", in, got, want)
	}
}

// On macOS a temporary directory is handed out under /var/folders while a
// process that resolves its working directory reports /private/var/folders.
// Output that mixes the two must still normalise to one token.
func TestRootNormalizer_ResolvedForm(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Skipf("EvalSymlinks: %v", err)
	}

	if resolved == root {
		t.Skip("no symlinked temp dir on this platform")
	}

	n := RootNormalizer(root)

	for _, form := range []string{root, resolved} {
		if got := n.Apply(form + "/docs"); got != "$ROOT/docs" {
			t.Errorf("Apply(%q) = %q, want $ROOT/docs", form+"/docs", got)
		}
	}
}

func TestDateNormalizer(t *testing.T) {
	t.Parallel()

	n := DateNormalizer("2026-03-04")

	in := "created: 2026-03-04\nother: 2026-03-05\n"
	want := "created: $DATE\nother: 2026-03-05\n"

	if got := n.Apply(in); got != want {
		t.Errorf("Apply() = %q, want %q", got, want)
	}
}

func TestMarkerNormalizer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "drops a canonical pair",
			in:   "## Summary\n<!--docz:summary:start-->\nbody\n<!--docz:summary:end-->\n",
			want: "## Summary\nbody\n",
		},
		{
			name: "drops an indented marker",
			in:   "  <!--docz:tasks:start-->\n- [ ] a\n",
			want: "- [ ] a\n",
		},
		{
			name: "keeps a marker with text beside it",
			in:   "<!--docz:tasks:start--> trailing\n",
			want: "<!--docz:tasks:start--> trailing\n",
		},
		{
			name: "keeps the legacy toc pair",
			in:   "<!--toc:start-->\n- [A](#a)\n<!--toc:end-->\n",
			want: "<!--toc:start-->\n- [A](#a)\n<!--toc:end-->\n",
		},
		{
			name: "keeps a hyphenated kind out of other comments",
			in:   "<!-- markdownlint-disable-file MD025 -->\n<!--docz:file-changes:end-->\n",
			want: "<!-- markdownlint-disable-file MD025 -->\n",
		},
		{
			name: "leaves marker-free input untouched",
			in:   "plain\nbody\n",
			want: "plain\nbody\n",
		},
		// The template layout: sections separated by one blank line, every
		// marker on a line of its own. Dropping a marker between two blanks
		// has to take one of them, or v2 shows a double blank where v1.2.2
		// has one and every section break is a false difference.
		{
			name: "a marker between two blanks takes one with it",
			in:   "text\n\n<!--docz:summary:end-->\n\n## Next\n",
			want: "text\n\n## Next\n",
		},
		{
			name: "stacked closers collapse to one blank",
			in:   "-\n\n<!--docz:out-of-scope:end-->\n<!--docz:scope:end-->\n\n## Next\n",
			want: "-\n\n## Next\n",
		},
		{
			name: "an opener above a heading keeps the blank above it",
			in:   "text\n\n<!--docz:summary:start-->\n## Summary\n",
			want: "text\n\n## Summary\n",
		},
		{
			name: "a blank not next to a marker is untouched",
			in:   "a\n\n\nb\n<!--docz:x:start-->\n",
			want: "a\n\n\nb\n",
		},
	}

	n := MarkerNormalizer()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := n.Apply(tt.in); got != tt.want {
				t.Errorf("Apply(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestNormalize_AppliesInOrder(t *testing.T) {
	t.Parallel()

	first := Normalizer{Name: "a", Apply: func(s string) string { return s + "1" }}
	second := Normalizer{Name: "b", Apply: func(s string) string { return s + "2" }}

	if got := Normalize("x", first, second); got != "x12" {
		t.Errorf("Normalize() = %q, want x12", got)
	}
}

func TestTree(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "b.md"), "bbb\n")
	writeFile(t, filepath.Join(dir, "docs", "a.md"), "a\n")
	writeFile(t, filepath.Join(dir, ".git", "HEAD"), "ref\n")

	files, err := Tree(dir)
	if err != nil {
		t.Fatalf("Tree: %v", err)
	}

	paths := make([]string, 0, len(files))
	for _, f := range files {
		paths = append(paths, f.Path)
	}

	want := []string{"b.md", "docs/a.md"}
	if strings.Join(paths, ",") != strings.Join(want, ",") {
		t.Errorf("paths = %v, want %v (.git must be skipped, order sorted)", paths, want)
	}

	if files[0].Size != 4 {
		t.Errorf("b.md size = %d, want 4", files[0].Size)
	}

	if files[0].Sum == "" || len(files[0].Sum) != 12 {
		t.Errorf("Sum = %q, want a 12-character digest", files[0].Sum)
	}
}

func TestChangedAndDeleted(t *testing.T) {
	t.Parallel()

	before := []File{
		{Path: "keep.md", Size: 2, Body: "a\n", Sum: "aaa"},
		{Path: "gone.md", Size: 2, Body: "b\n", Sum: "bbb"},
	}
	after := []File{
		{Path: "keep.md", Size: 2, Body: "a\n", Sum: "aaa"},
		{Path: "new.md", Size: 2, Body: "c\n", Sum: "ccc"},
		{Path: "edited.md", Size: 2, Body: "d\n", Sum: "ddd"},
	}

	got := Changed(before, after)

	if len(got) != 3 {
		t.Fatalf("Changed returned %d entries, want 3", len(got))
	}

	if got[0].Body != "" {
		t.Errorf("unchanged file kept its body: %q", got[0].Body)
	}

	if got[1].Body != "c\n" || got[2].Body != "d\n" {
		t.Errorf("written files lost their bodies: %+v", got[1:])
	}

	gone := Deleted(before, after)
	if len(gone) != 1 || gone[0] != "gone.md" {
		t.Errorf("Deleted = %v, want [gone.md]", gone)
	}
}

func TestFormat(t *testing.T) {
	t.Parallel()

	r := Result{
		Args:     []string{"create", "rfc", "A title"},
		ExitCode: 0,
		Stdout:   "Created $ROOT/docs/rfc/0003-a-title.md\n",
		Files: []File{
			{Path: "docs/rfc/0001-x.md", Size: 10, Sum: "aaaaaaaaaaaa"},
			{Path: "docs/rfc/0003-a-title.md", Size: 3, Sum: "bbbbbbbbbbbb", Body: "hi\n"},
		},
	}

	got := Format(&r, []string{"docs/rfc/old.md"})

	for _, want := range []string{
		"$ docz create rfc A title\n",
		"exit 0\n",
		"=== stdout\nCreated $ROOT/docs/rfc/0003-a-title.md\n",
		"=== stderr\n(empty)\n",
		"docs/rfc/0001-x.md (10 bytes, aaaaaaaaaaaa)\n",
		"docs/rfc/old.md (deleted)\n",
		"=== written docs/rfc/0003-a-title.md\nhi\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Format() missing %q\ngot:\n%s", want, got)
		}
	}

	if strings.Contains(got, "=== written docs/rfc/0001-x.md") {
		t.Error("Format() wrote a body for an unchanged file")
	}
}

func TestFormat_FlagsMissingTrailingNewline(t *testing.T) {
	t.Parallel()

	got := Format(&Result{Args: []string{"config"}, Stdout: "no newline"}, nil)

	if !strings.Contains(got, "\\ no trailing newline\n") {
		t.Errorf("Format() did not flag a missing trailing newline:\n%s", got)
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}

	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// A gitlink is a file named .git holding an absolute host path that no
// normaliser would catch, so the name is skipped whether it is a file or a
// directory.
func TestTree_SkipsGitlinkFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".git"), "gitdir: /Users/someone/code/x/.git/modules/y\n")
	writeFile(t, filepath.Join(dir, "keep.md"), "a\n")

	files, err := Tree(dir)
	if err != nil {
		t.Fatalf("Tree: %v", err)
	}

	if len(files) != 1 || files[0].Path != "keep.md" {
		t.Errorf("Tree recorded %+v, want only keep.md", files)
	}
}

// A recorded size and digest describe the normalised body, so the number
// beside a body in a golden is one a reviewer can check against what they
// are reading.
func TestNormalizeFiles_RecomputesSizeAndSum(t *testing.T) {
	t.Parallel()

	raw := "## Summary\n\n<!--docz:summary:end-->\n\ntext\n"
	want := "## Summary\n\ntext\n"

	got := NormalizeFiles([]File{{Path: "a.md", Body: raw, Size: 99, Sum: "stale"}},
		MarkerNormalizer())

	if len(got) != 1 {
		t.Fatalf("NormalizeFiles returned %d files, want 1", len(got))
	}

	if got[0].Body != want {
		t.Errorf("Body = %q, want %q", got[0].Body, want)
	}

	if got[0].Size != int64(len(want)) {
		t.Errorf("Size = %d, want %d", got[0].Size, len(want))
	}

	sum := sha256.Sum256([]byte(want))
	if wantSum := hex.EncodeToString(sum[:])[:12]; got[0].Sum != wantSum {
		t.Errorf("Sum = %q, want %q", got[0].Sum, wantSum)
	}
}

// Both snapshots are normalised before Changed compares them, so a file the
// command did not touch is still recognised as unchanged and its body stays
// out of the golden.
func TestNormalizeFiles_KeepsChangedComparable(t *testing.T) {
	t.Parallel()

	norms := []Normalizer{MarkerNormalizer()}
	untouched := File{Path: "a.md", Body: "<!--docz:x:start-->\nsame\n", Size: 25, Sum: "raw"}

	before := NormalizeFiles([]File{untouched}, norms...)
	after := NormalizeFiles([]File{untouched}, norms...)

	changed := Changed(before, after)
	if len(changed) != 1 {
		t.Fatalf("Changed returned %d files, want 1", len(changed))
	}

	if changed[0].Body != "" {
		t.Errorf("an untouched file recorded a body: %q", changed[0].Body)
	}
}
