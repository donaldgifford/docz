package docparse_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
)

// regionFixtures are the shapes the walker has to survive. Each is a real
// document rather than a snippet, because the rules that break are the
// ones that interact: a fence next to a marker, a parent closing over an
// open child, a legacy pair in a document that has nothing else.
var regionFixtures = []string{
	"impl_canonical",    // every nesting the IMPL skeleton uses
	"nested_repeated",   // three levels, and a kind repeated at two depths
	"stray_end",         // ends with nothing open, before and after a good pair
	"unclosed_eof",      // a child cut off by its parent, a region open at EOF
	"fenced_markers",    // markers as example text, and the tilde non-toggle
	"lenient_spellings", // every accepted spelling, and the near misses
	"legacy_toc_only",   // a v1 document: one ToC pair, no docz: marker
	"readme_index",      // the README index pair
}

// renderRegionFacts serializes every marker and region, one fact per line.
func renderRegionFacts(content []byte) string {
	var sb strings.Builder

	sb.WriteString("# markers\n")

	for _, m := range docparse.Markers(content) {
		fmt.Fprintf(
			&sb, "line=%d kind=%q role=%s canonical=%t\n",
			m.Line, m.Kind, m.Role, m.Canonical,
		)
	}

	sb.WriteString("# regions\n")

	for _, r := range docparse.Regions(content) {
		fmt.Fprintf(
			&sb, "kind=%q start=%d end=%d depth=%d closed=%t\n",
			r.Kind, r.Start, r.End, r.Depth, r.Closed,
		)
	}

	return sb.String()
}

func TestGoldenRegions(t *testing.T) {
	t.Parallel()

	for _, name := range regionFixtures {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join("testdata", "regions", name+".md")

			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading fixture: %v", err)
			}

			got := renderRegionFacts(content)
			goldenPath := filepath.Join("testdata", "regions", "golden", name+".golden.txt")

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
				t.Errorf("facts differ from %s\n--- got ---\n%s\n--- want ---\n%s",
					goldenPath, got, want)
			}
		})
	}
}

// Invariants the goldens cannot state, checked over every fixture.
func TestRegions_Invariants(t *testing.T) {
	t.Parallel()

	for _, name := range regionFixtures {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			content, err := os.ReadFile(filepath.Join("testdata", "regions", name+".md"))
			if err != nil {
				t.Fatalf("reading fixture: %v", err)
			}

			checkRegionInvariants(t, content)
		})
	}
}

// checkRegionInvariants asserts what must hold for any input at all. It is
// shared with the fuzz target, so a shape the corpus never thought of is
// held to the same contract.
func checkRegionInvariants(t *testing.T, content []byte) {
	t.Helper()

	markerLines := make(map[int]bool)
	for _, m := range docparse.Markers(content) {
		if m.Line < 1 {
			t.Errorf("marker line %d is not 1-based: %+v", m.Line, m)
		}

		if m.Role != docparse.Start && m.Role != docparse.End {
			t.Errorf("marker has no valid role: %+v", m)
		}

		if m.Kind == "" {
			t.Errorf("marker has an empty kind: %+v", m)
		}

		markerLines[m.Line] = true
	}

	regions := docparse.Regions(content)

	// A stack replay: each region must nest inside the one before it at
	// depth-1, which is what makes Depth meaningful to a consumer.
	var open []docparse.Region

	for _, r := range regions {
		// A closed region's two marker lines are always distinct, so it
		// spans. An unclosed one may not: a start marker on the last line
		// opens a region that contains nothing, and reporting End past the
		// last line would name a line that does not exist.
		switch {
		case r.Closed && r.Start >= r.End:
			t.Errorf("closed region does not span: %+v", r)
		case !r.Closed && r.Start > r.End:
			t.Errorf("unclosed region ends before it starts: %+v", r)
		}

		if !markerLines[r.Start] {
			t.Errorf("region starts off a marker line: %+v", r)
		}

		if r.Closed && !markerLines[r.End] {
			t.Errorf("closed region ends off a marker line: %+v", r)
		}

		if r.Depth < 0 {
			t.Errorf("negative depth: %+v", r)
		}

		for len(open) > 0 && open[len(open)-1].End < r.Start {
			open = open[:len(open)-1]
		}

		if r.Depth != len(open) {
			t.Errorf("region depth %d does not match its nesting %d: %+v", r.Depth, len(open), r)
		}

		if len(open) > 0 {
			parent := open[len(open)-1]
			if r.Start < parent.Start || r.End > parent.End {
				t.Errorf("region %+v is not contained in its parent %+v", r, parent)
			}
		}

		open = append(open, r)
	}
}

// FuzzRegions pins the contract for inputs no fixture covers: never panic,
// every span real, depth consistent with nesting, and the marker lines a
// writer would splice at actually being marker lines.
func FuzzRegions(f *testing.F) {
	for _, name := range regionFixtures {
		content, err := os.ReadFile(filepath.Join("testdata", "regions", name+".md"))
		if err != nil {
			f.Fatalf("seeding from %s: %v", name, err)
		}

		f.Add(content)
	}

	f.Add([]byte(""))
	f.Add([]byte("<!--docz:a:start-->"))
	f.Add([]byte("<!--docz:a:end-->"))
	f.Add([]byte("<!--docz:a:start-->\n<!--docz:a:start-->\n<!--docz:a:end-->"))
	f.Add([]byte("```\n<!--docz:a:start-->\n"))
	f.Add([]byte("<!--toc:start-->\n<!-- BEGIN DOCZ AUTO-GENERATED -->\n"))

	f.Fuzz(func(t *testing.T, content []byte) {
		checkRegionInvariants(t, content)
	})
}
