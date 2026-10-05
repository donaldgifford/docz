package confluence

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// goldenInputs returns every golden input's bytes.
func goldenInputs(tb testing.TB) [][]byte {
	tb.Helper()

	paths, err := filepath.Glob(filepath.Join("testdata", "render", "*.md"))
	if err != nil || len(paths) == 0 {
		tb.Fatalf("no golden inputs: %v", err)
	}

	out := make([][]byte, 0, len(paths))

	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			tb.Fatal(err)
		}

		out = append(out, b)
	}

	return out
}

// TestRender_Invariants pins what holds for every input: the input is not
// modified, two renders agree byte for byte once the minted local ids are
// restored, and the hash does not depend on the ids.
func TestRender_Invariants(t *testing.T) {
	t.Parallel()

	for _, src := range goldenInputs(t) {
		orig := bytes.Clone(src)
		opts := RenderOptions{Resolve: testResolver, Source: "docs/a/0001-a.md", SourceURL: "https://x/y"}

		first, err := Render(src, opts)
		if err != nil {
			t.Fatal(err)
		}

		second, err := Render(src, opts)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(src, orig) {
			t.Error("Render modified its input")
		}

		if !bytes.Equal(restoreLocalIDs(first.Body), restoreLocalIDs(second.Body)) {
			t.Errorf("two renders of %q differ beyond their local ids", first.Title)
		}

		if first.Hash != second.Hash {
			t.Errorf("hash of %q changed between renders", first.Title)
		}
	}
}

func TestRender_LocalIDsAreUniquePerMacro(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile(filepath.Join("testdata", "render", "mermaid-viewer.md"))
	if err != nil {
		t.Fatal(err)
	}

	got, err := Render(src, RenderOptions{})
	if err != nil {
		t.Fatal(err)
	}

	ids := localIDValue.FindAllSubmatch(got.Body, -1)
	if len(ids) != 4 {
		t.Fatalf("found %d local-id values, want 4 (two per macro)", len(ids))
	}

	all := localIDValue.FindAll(got.Body, -1)
	if bytes.Equal(all[0], all[2]) {
		t.Error("two viewer macros share a local id")
	}

	if !bytes.Equal(all[0], all[1]) {
		t.Error("a macro's parameter and attribute local ids differ")
	}
}

// FuzzRender pins the contract: Render never panics, and anything it returns
// without an error is well-formed.
func FuzzRender(f *testing.F) {
	for _, src := range goldenInputs(f) {
		f.Add(src)
	}

	f.Fuzz(func(t *testing.T, src []byte) {
		got, err := Render(src, RenderOptions{Title: "T: fuzz", Resolve: testResolver})
		if err != nil {
			var malformed *MalformedError
			if errors.As(err, &malformed) && malformed.Line < 0 {
				t.Fatalf("negative line in %v", err)
			}

			return
		}

		if werr := wellFormed(restoreLocalIDs(got.Body)); werr != nil {
			t.Fatalf("returned body is not well-formed: %v", werr)
		}
	})
}
