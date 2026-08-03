package document_test

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/donaldgifford/docz/pkg/doczcore/document"
)

var updateChangelog = flag.Bool("update", false, "update golden files")

// changelogFixtures are the parseable corpus documents under
// testdata/changelog. The fleet fixture is a verbatim snapshot of a real
// git-cliff changelog (docz-api's), so the golden pins the shape
// git-cliff actually emits rather than an idealized one.
var changelogFixtures = []string{"fleet", "chart", "edge", "nopreamble"}

func readChangelogFixture(t *testing.T, name string) []byte {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("testdata", "changelog", name+".md"))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	return content
}

// renderChangelog serializes a parsed changelog for golden comparison.
func renderChangelog(cl *document.Changelog) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "preamble=%q\n", cl.Preamble)
	for _, v := range cl.Versions {
		fmt.Fprintf(&sb, "version=%q unreleased=%t date=%q\n",
			v.Version, v.Unreleased, v.Date)
		for _, g := range v.Groups {
			fmt.Fprintf(&sb, "  group=%q\n", g.Title)
			for _, it := range g.Items {
				fmt.Fprintf(&sb, "    item=%q\n", it)
			}
		}
	}
	return sb.String()
}

func TestParseChangelog_Golden(t *testing.T) {
	t.Parallel()
	for _, name := range changelogFixtures {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cl, err := document.ParseChangelog(readChangelogFixture(t, name))
			if err != nil {
				t.Fatalf("ParseChangelog(%s) = %v, want nil", name, err)
			}
			got := renderChangelog(cl)

			goldenPath := filepath.Join("testdata", "changelog", name+".golden.txt")
			if *updateChangelog {
				if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				t.Log("updated golden file:", goldenPath)
				return
			}

			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("reading golden %s: %v\nRun with -update to create it",
					goldenPath, err)
			}
			if got != string(want) {
				t.Errorf("parsed facts differ from golden %s\nGot:\n%s\nRun with -update to update",
					goldenPath, got)
			}
		})
	}
}

// TestParseChangelog_Invariants asserts the contract properties across
// every fixture. Golden files alone cannot pin these: a golden
// regenerated with -update happily bakes in a regression.
func TestParseChangelog_Invariants(t *testing.T) {
	t.Parallel()
	for _, name := range changelogFixtures {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			raw := readChangelogFixture(t, name)
			cl, err := document.ParseChangelog(raw)
			if err != nil {
				t.Fatalf("ParseChangelog(%s) = %v, want nil", name, err)
			}

			if !strings.HasPrefix(string(raw), cl.Preamble) {
				t.Error("Preamble is not a verbatim prefix of the input")
			}
			if len(cl.Versions) == 0 {
				t.Error("Versions is empty on a nil error")
			}

			for i, v := range cl.Versions {
				if v.Unreleased != (v.Version == "unreleased") {
					t.Errorf("versions[%d]: Unreleased=%t but Version=%q",
						i, v.Unreleased, v.Version)
				}
				if v.Unreleased && v.Date != "" {
					t.Errorf("versions[%d]: unreleased section carries Date=%q", i, v.Date)
				}
				for _, g := range v.Groups {
					for j, it := range g.Items {
						first, _, _ := strings.Cut(it, "\n")
						if strings.HasPrefix(first, "- ") || strings.HasPrefix(first, "* ") {
							t.Errorf("versions[%d] group %q item %d keeps its bullet marker: %q",
								i, g.Title, j, first)
						}
					}
				}
			}

			// The preamble alone is never a changelog.
			if cl.Preamble != "" {
				if _, err := document.ParseChangelog([]byte(cl.Preamble)); !errors.Is(err, document.ErrNoVersions) {
					t.Errorf("parsing the preamble alone = %v, want ErrNoVersions", err)
				}
			}
		})
	}
}

func TestParseChangelog_NoVersions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		content []byte
		fixture string // read from testdata when content is nil
	}{
		{name: "nil", content: nil},
		{name: "empty", content: []byte{}},
		{name: "prose only", fixture: "noversions"},
		{name: "no heading", content: []byte("just some text\n\nmore text\n")},
		{name: "wrong heading level", content: []byte("### [1.0.0] - 2026-01-01\n")},
		{name: "no brackets", content: []byte("## 1.0.0 - 2026-01-01\n")},
		{name: "no space after hashes", content: []byte("##[1.0.0]\n")},
		{
			name:    "version only inside a fence",
			content: []byte("# Doc\n\n```\n## [1.0.0] - 2026-01-01\n```\n"),
		},
		{
			name:    "unterminated fence swallows the version",
			content: []byte("# Doc\n\n```\n\n## [1.0.0] - 2026-01-01\n"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			content := tt.content
			if tt.fixture != "" {
				content = readChangelogFixture(t, tt.fixture)
			}
			cl, err := document.ParseChangelog(content)
			if !errors.Is(err, document.ErrNoVersions) {
				t.Errorf("ParseChangelog() error = %v, want ErrNoVersions", err)
			}
			if cl != nil {
				t.Errorf("ParseChangelog() = %+v, want nil result alongside the error", cl)
			}
		})
	}
}

func TestParseChangelog_VersionIdentity(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		heading        string
		wantVersion    string
		wantUnreleased bool
		wantDate       string
	}{
		{name: "plain", heading: "## [0.4.2] - 2026-07-23", wantVersion: "0.4.2", wantDate: "2026-07-23"},
		{name: "v prefix", heading: "## [v0.4.2] - 2026-07-23", wantVersion: "0.4.2", wantDate: "2026-07-23"},
		{name: "uppercase V", heading: "## [V1.2.3]", wantVersion: "1.2.3"},
		{name: "v not before digit", heading: "## [vnext]", wantVersion: "vnext"},
		{name: "prerelease", heading: "## [1.0.0-rc.1] - 2026-01-01", wantVersion: "1.0.0-rc.1", wantDate: "2026-01-01"},
		{name: "build metadata", heading: "## [1.0.0+build-2]", wantVersion: "1.0.0+build-2"},
		{name: "unreleased lower", heading: "## [unreleased]", wantVersion: "unreleased", wantUnreleased: true},
		{name: "unreleased mixed case", heading: "## [Unreleased]", wantVersion: "unreleased", wantUnreleased: true},
		{name: "unreleased with date discarded", heading: "## [Unreleased] - 2026-01-01", wantVersion: "unreleased", wantUnreleased: true},
		{name: "inner whitespace", heading: "## [ 1.0.0 ]", wantVersion: "1.0.0"},
		{name: "no separator dash", heading: "## [1.0.0] 2026-01-01", wantVersion: "1.0.0", wantDate: "2026-01-01"},
		{name: "trailing junk kept", heading: "## [1.0.0] - 2026-01-01 [YANKED]", wantVersion: "1.0.0", wantDate: "2026-01-01 [YANKED]"},
		{name: "tab after hashes", heading: "##\t[1.0.0]", wantVersion: "1.0.0"},
		{name: "crlf line ending", heading: "## [1.0.0] - 2026-01-01\r", wantVersion: "1.0.0", wantDate: "2026-01-01"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cl, err := document.ParseChangelog([]byte(tt.heading + "\n"))
			if err != nil {
				t.Fatalf("ParseChangelog(%q) = %v, want nil", tt.heading, err)
			}
			if len(cl.Versions) != 1 {
				t.Fatalf("got %d versions, want 1", len(cl.Versions))
			}
			v := cl.Versions[0]
			if v.Version != tt.wantVersion {
				t.Errorf("Version = %q, want %q", v.Version, tt.wantVersion)
			}
			if v.Unreleased != tt.wantUnreleased {
				t.Errorf("Unreleased = %t, want %t", v.Unreleased, tt.wantUnreleased)
			}
			if v.Date != tt.wantDate {
				t.Errorf("Date = %q, want %q", v.Date, tt.wantDate)
			}
		})
	}
}

func TestParseChangelog_PreambleBoundary(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "version at byte zero", content: "## [1.0.0]\n", want: ""},
		{name: "trailing newline included", content: "# T\n## [1.0.0]\n", want: "# T\n"},
		{name: "blank line included", content: "# T\n\n## [1.0.0]\n", want: "# T\n\n"},
		{
			name:    "fenced decoy does not cut the preamble",
			content: "# T\n\n```\n## [9.9.9]\n```\n\n## [1.0.0]\n",
			want:    "# T\n\n```\n## [9.9.9]\n```\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cl, err := document.ParseChangelog([]byte(tt.content))
			if err != nil {
				t.Fatalf("ParseChangelog() = %v, want nil", err)
			}
			if cl.Preamble != tt.want {
				t.Errorf("Preamble = %q, want %q", cl.Preamble, tt.want)
			}
		})
	}
}

// TestParseChangelog_CRLF pins the documented promise that CRLF input
// yields the same field values as its LF twin. Every line shape that can
// reach an item is represented: the bullet itself, wrapped prose, a
// nested sub-bullet, and a fenced block — a continuation line is
// appended to the item raw, so each is a place a stray \r can survive
// into the frozen output. (Preamble is exempt: it is byte-verbatim by
// contract, so it keeps the carriage returns the input had.)
func TestParseChangelog_CRLF(t *testing.T) {
	t.Parallel()
	lf := "# T\n\n## [1.0.0] - 2026-01-01\n\n### Bug Fixes\n\n" +
		"- one fix\n  wrapped prose\n  - nested sub-bullet\n\n  ```\n  code line\n  ```\n" +
		"- second fix\n\n## [0.9.0] - 2025-12-01\n\n- loose bullet\n"
	crlf := strings.ReplaceAll(lf, "\n", "\r\n")

	lfCL, err := document.ParseChangelog([]byte(lf))
	if err != nil {
		t.Fatalf("LF parse = %v", err)
	}
	crlfCL, err := document.ParseChangelog([]byte(crlf))
	if err != nil {
		t.Fatalf("CRLF parse = %v", err)
	}

	if !reflect.DeepEqual(crlfCL.Versions, lfCL.Versions) {
		t.Errorf("CRLF and LF parses differ.\nCRLF:\n%s\nLF:\n%s",
			renderChangelog(crlfCL), renderChangelog(lfCL))
	}
	// Scan the versions only — Preamble is byte-verbatim, so it keeps the
	// carriage returns the input had.
	rendered := renderChangelog(&document.Changelog{Versions: crlfCL.Versions})
	if strings.Contains(rendered, `\r`) {
		t.Errorf("a carriage return survived into the parsed values:\n%s", rendered)
	}
}

// TestParseChangelog_InVersionContent pins what the empty-title group
// does and does not hold. Bullets before the first "###" are kept there
// (DESIGN-0010: the parser must not lose them); column-0 prose in the
// same position is discarded like column-0 prose anywhere else, so a
// release note can never be mistaken for a commit item.
func TestParseChangelog_InVersionContent(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		body       string
		wantGroups []string // titles, in order
		wantItems  []string // items of the first group
	}{
		{
			name:       "bullets before any heading",
			body:       "- loose bullet\n\n### Features\n\n- a\n",
			wantGroups: []string{"", "Features"},
			wantItems:  []string{"loose bullet"},
		},
		{
			name:       "prose before any heading is dropped",
			body:       "Important release note prose.\n\n### Features\n\n- a\n",
			wantGroups: []string{"Features"},
			wantItems:  []string{"a"},
		},
		{
			name:       "prose between bullets is dropped",
			body:       "### Features\n\n- a\n\nloose prose\n\n- b\n",
			wantGroups: []string{"Features"},
			wantItems:  []string{"a", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cl, err := document.ParseChangelog(
				[]byte("## [1.0.0] - 2026-01-01\n\n" + tt.body))
			if err != nil {
				t.Fatalf("ParseChangelog() = %v, want nil", err)
			}

			var titles []string
			for _, g := range cl.Versions[0].Groups {
				titles = append(titles, g.Title)
			}
			if !reflect.DeepEqual(titles, tt.wantGroups) {
				t.Fatalf("group titles = %q, want %q", titles, tt.wantGroups)
			}
			if got := cl.Versions[0].Groups[0].Items; !reflect.DeepEqual(got, tt.wantItems) {
				t.Errorf("first group items = %q, want %q", got, tt.wantItems)
			}
		})
	}
}

func FuzzParseChangelog(f *testing.F) {
	// Seed from the real fixtures: the panic surface is the byte-offset
	// arithmetic around the preamble cut, which fixture-shaped inputs
	// exercise far better than hand-written strings.
	for _, name := range append(changelogFixtures, "noversions") {
		content, err := os.ReadFile(filepath.Join("testdata", "changelog", name+".md"))
		if err != nil {
			f.Fatalf("seeding from %s: %v", name, err)
		}
		f.Add(content)
	}
	f.Add([]byte("## [1.0.0]"))
	f.Add([]byte("```\n## [1.0.0]"))
	f.Add([]byte("-"))

	f.Fuzz(func(t *testing.T, content []byte) {
		cl, err := document.ParseChangelog(content)
		switch {
		case err == nil && cl == nil:
			t.Fatal("nil result with a nil error")
		case err != nil && !errors.Is(err, document.ErrNoVersions):
			t.Fatalf("unexpected error kind: %v", err)
		case err != nil && cl != nil:
			t.Fatal("non-nil result alongside an error")
		case err == nil && !strings.HasPrefix(string(content), cl.Preamble):
			t.Fatal("Preamble is not a prefix of the input")
		}
	})
}

// TestParseChangelog_LongItemIsLinear pins that one very long item does
// not cost quadratic time. Appending each continuation line to a string
// copies the whole item every line, so a single multi-megabyte item took
// tens of seconds — a real hazard for a parser that runs over whatever
// CHANGELOG.md a repo happens to hold. The accumulator is a Builder now.
//
// The budget is deliberately loose: the linear parse is milliseconds, so
// even a heavily loaded runner stays orders of magnitude under it, while
// the quadratic version blew past it by more than 2x.
func TestParseChangelog_LongItemIsLinear(t *testing.T) {
	t.Parallel()

	const (
		lines  = 200_000
		budget = 10 * time.Second
	)

	var sb strings.Builder
	sb.WriteString("## [1.0.0] - 2026-01-01\n\n### Bug Fixes\n\n- the one item\n")
	for range lines {
		sb.WriteString("  a continuation line belonging to that item\n")
	}
	content := []byte(sb.String())

	start := time.Now()
	cl, err := document.ParseChangelog(content)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("ParseChangelog() = %v, want nil", err)
	}
	if elapsed > budget {
		t.Errorf("parsing %d bytes took %v, over the %v budget: the item "+
			"accumulator is likely copying per line again", len(content), elapsed, budget)
	}

	// The fix must not have changed what is parsed.
	items := cl.Versions[0].Groups[0].Items
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if got := strings.Count(items[0], "\n"); got != lines {
		t.Errorf("item holds %d newlines, want %d: continuation lines were dropped", got, lines)
	}
	if !strings.HasPrefix(items[0], "the one item\n") {
		t.Errorf("item text starts %q, want the bullet body first", items[0][:min(40, len(items[0]))])
	}
}
