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

// The plan normaliser is the fourth permitted delta (ADR-0003, IMPL-0018 Open
// Question 8) and the only one that runs on both sides of the comparison, so
// every rule needs a case for what it removes and a case for the near-miss it
// has to leave alone.
func TestPlanNormalizer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "drops the plan block under types",
			in: "types:\n  adr:\n    dir: adr\n" +
				"  plan:\n    dir: plan\n    id_prefix: PLAN\n" +
				"  rfc:\n    dir: rfc\n",
			want: "types:\n  adr:\n    dir: adr\n  rfc:\n    dir: rfc\n",
		},
		{
			name: "a blank line inside the block goes with it",
			// The rendered config separates type blocks with one blank line, so
			// keeping it would leave a double blank where v1.2.2 has one.
			in:   "types:\n  plan:\n    dir: plan\n\n  rfc:\n    dir: rfc\n",
			want: "types:\n  rfc:\n    dir: rfc\n",
		},
		{
			name: "drops the nav title entry",
			in: "  nav_titles:\n    impl: Implementation Plans\n" +
				"    plan: Plans\n    rfc: RFCs\n",
			want: "  nav_titles:\n    impl: Implementation Plans\n    rfc: RFCs\n",
		},
		{
			name: "keeps a label that merely contains the word",
			// impl's plural label is "Implementation Plans". A rule that matched
			// the word rather than the entry would delete a type still shipping.
			in:   "    impl: Implementation Plans\n",
			want: "    impl: Implementation Plans\n",
		},
		{
			name: "drops the non-built-in warning",
			in:   "=== stderr\nWarning: config declares non-built-in type \"plan\" (typo?)\n",
			want: "=== stderr\n(empty)\n",
		},
		{
			name: "keeps a warning about another type",
			in:   "Warning: config declares non-built-in type \"frameworks\" (typo?)\n",
			want: "Warning: config declares non-built-in type \"frameworks\" (typo?)\n",
		},
		{
			name: "drops the id-prefix hint from a field line",
			in:   "**Implements:** <!-- RFC-XXXX / DESIGN-XXXX / PLAN-XXXX -->\n",
			want: "**Implements:** <!-- RFC-XXXX / DESIGN-XXXX -->\n",
		},
		{
			name: "drops the hint from the middle of a list",
			in:   "**Triggered by:** <!-- RFC-XXXX / DESIGN-XXXX / PLAN-XXXX / issue #XXX -->\n",
			want: "**Triggered by:** <!-- RFC-XXXX / DESIGN-XXXX / issue #XXX -->\n",
		},
		{
			name: "drops the slash form",
			in:   "<!-- Link to the RFC/DESIGN/PLAN it implements. -->\n",
			want: "<!-- Link to the RFC/DESIGN it implements. -->\n",
		},
		{
			name: "keeps a heading that contains the word",
			// `## Testing Plan` is the reason the hint rules are substring
			// deletions of the id-prefix spellings and not a word match.
			in:   "## Testing Plan\n\n<!-- Links to related RFCs, ADRs, designs, plans, issues -->\n",
			want: "## Testing Plan\n\n<!-- Links to related RFCs, ADRs, designs, plans, issues -->\n",
		},
		{
			name: "drops the generated config preamble",
			in: "=== written .docz.yaml\n" +
				"# .docz.yaml -- configuration for the docz CLI.\n" +
				"#\n" +
				"#   entirely to keep all six built-in types (rfc, adr, design, impl, plan,\n" +
				"#   investigation).\n" +
				"\n" +
				"docs_dir: docs\n",
			want: "=== written .docz.yaml\ndocs_dir: docs\n",
		},
		{
			name: "keeps a comment that is not the preamble",
			in:   "docs_dir: docs\n# a comment somebody added\n",
			want: "docs_dir: docs\n# a comment somebody added\n",
		},
		{
			name: "leaves plan-free input untouched",
			in:   "=== stdout\ndocs_dir: docs\n\n=== files\ndocs/rfc/README.md (10 bytes, abc123)\n",
			want: "=== stdout\ndocs_dir: docs\n\n=== files\ndocs/rfc/README.md (10 bytes, abc123)\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := Normalize(tt.in, PlanNormalizer()); got != tt.want {
				t.Errorf("PlanNormalizer()\ngot:  %q\nwant: %q", got, tt.want)
			}
		})
	}
}

// TestPlanNormalizer_NeutralisesRecordedBodies pins the rule that makes the
// two sides comparable at all: a file whose body the golden records loses its
// size and digest, and a file whose body it does not keeps both.
//
// The rule cannot depend on which side carried a trace, because the side that
// no longer carries one has no way to know a trace was there.
func TestPlanNormalizer_NeutralisesRecordedBodies(t *testing.T) {
	t.Parallel()

	in := "=== files\n" +
		"docs/impl/0001-a.md (2959 bytes, 4b2296e7028b)\n" +
		"docs/impl/README.md (1531 bytes, 7d206b655c6d)\n" +
		"\n=== written docs/impl/0001-a.md\n" +
		"**Implements:** <!-- RFC-XXXX / DESIGN-XXXX / PLAN-XXXX -->\n"

	want := "=== files\n" +
		"docs/impl/0001-a.md ($SIZE bytes, $SUM)\n" +
		"docs/impl/README.md (1531 bytes, 7d206b655c6d)\n" +
		"\n=== written docs/impl/0001-a.md\n" +
		"**Implements:** <!-- RFC-XXXX / DESIGN-XXXX -->\n"

	if got := Normalize(in, PlanNormalizer()); got != want {
		t.Errorf("PlanNormalizer()\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// TestPlanNormalizer_Idempotent pins that a second pass is a no-op, since the
// suite applies it to a golden that may already have been through it in a
// previous run's diff output.
func TestPlanNormalizer_Idempotent(t *testing.T) {
	t.Parallel()

	in := "types:\n  plan:\n    dir: plan\n\n  rfc:\n    dir: rfc\n" +
		"=== stderr\nWarning: config declares non-built-in type \"plan\" (typo?)\n"

	once := Normalize(in, PlanNormalizer())
	if twice := Normalize(once, PlanNormalizer()); twice != once {
		t.Errorf("a second pass changed the result\nonce:  %q\ntwice: %q", once, twice)
	}
}

// TestIndexPairNormalizer covers the fifth permitted delta: a v1 `init` golden
// carries the duplicate index marker pair of issue #99 and the v2 binary does
// not, so both sides are collapsed to one pair before comparison.
func TestIndexPairNormalizer(t *testing.T) {
	t.Parallel()

	const (
		begin = "<!-- BEGIN DOCZ AUTO-GENERATED -->"
		end   = "<!-- END DOCZ AUTO-GENERATED -->"
	)

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "collapses a duplicate empty pair",
			in:   "# Investigations\n\n" + begin + "\n" + end + "\n" + begin + "\n" + end + "\n",
			want: "# Investigations\n\n" + begin + "\n" + end + "\n",
		},
		{
			name: "collapses three pairs down to one",
			in:   begin + "\n" + end + "\n" + begin + "\n" + end + "\n" + begin + "\n" + end + "\n",
			want: begin + "\n" + end + "\n",
		},
		{
			name: "leaves a single pair alone",
			in:   "# RFCs\n\n" + begin + "\n" + end + "\n",
			want: "# RFCs\n\n" + begin + "\n" + end + "\n",
		},
		{
			// The whole point of requiring the pair to be empty: a spliced
			// table must still be compared line by line.
			name: "never collapses a pair with a table in it",
			in:   begin + "\n| ID |\n| -- |\n" + end + "\n" + begin + "\n" + end + "\n",
			want: begin + "\n| ID |\n| -- |\n" + end + "\n",
		},
		{
			name: "a blank line between pairs stops the collapse",
			in:   begin + "\n" + end + "\n\n" + begin + "\n" + end + "\n",
			want: begin + "\n" + end + "\n\n" + begin + "\n" + end + "\n",
		},
		{
			name: "input with no markers is untouched",
			in:   "$ docz list\nexit 0\n",
			want: "$ docz list\nexit 0\n",
		},
		{
			name: "a lone begin with no end is untouched",
			in:   begin + "\n" + end + "\n" + begin + "\n",
			want: begin + "\n" + end + "\n" + begin + "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := Normalize(tt.in, IndexPairNormalizer()); got != tt.want {
				t.Errorf("IndexPairNormalizer()\ngot:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

// TestIndexPairNormalizer_Idempotent pins that a second pass is a no-op, the
// same contract the plan normaliser keeps.
func TestIndexPairNormalizer_Idempotent(t *testing.T) {
	t.Parallel()

	in := "# Investigations\n\n<!-- BEGIN DOCZ AUTO-GENERATED -->\n" +
		"<!-- END DOCZ AUTO-GENERATED -->\n<!-- BEGIN DOCZ AUTO-GENERATED -->\n" +
		"<!-- END DOCZ AUTO-GENERATED -->\n"

	once := Normalize(in, IndexPairNormalizer())
	if twice := Normalize(once, IndexPairNormalizer()); twice != once {
		t.Errorf("a second pass changed the result\nonce:  %q\ntwice: %q", once, twice)
	}
}
