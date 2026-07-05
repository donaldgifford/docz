package toc

import (
	"strconv"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/pkg/doczcore/docparse"
)

func TestGenerateToC(t *testing.T) {
	t.Parallel()
	t.Run("single level", func(t *testing.T) {
		t.Parallel()
		headings := []docparse.Heading{
			{Level: 2, Text: "First", Slug: "first"},
			{Level: 2, Text: "Second", Slug: "second"},
		}
		got := GenerateToC(headings, 1)
		want := "- [First](#first)\n- [Second](#second)\n"
		if got != want {
			t.Errorf("GenerateToC() =\n%q\nwant:\n%q", got, want)
		}
	})

	t.Run("mixed levels with relative indentation", func(t *testing.T) {
		t.Parallel()
		headings := []docparse.Heading{
			{Level: 2, Text: "Section", Slug: "section"},
			{Level: 3, Text: "Subsection", Slug: "subsection"},
			{Level: 4, Text: "Detail", Slug: "detail"},
			{Level: 2, Text: "Another", Slug: "another"},
		}
		got := GenerateToC(headings, 1)
		want := "- [Section](#section)\n" +
			"  - [Subsection](#subsection)\n" +
			"    - [Detail](#detail)\n" +
			"- [Another](#another)\n"
		if got != want {
			t.Errorf("GenerateToC() =\n%q\nwant:\n%q", got, want)
		}
	})

	t.Run("relative indentation starts at min level", func(t *testing.T) {
		t.Parallel()
		headings := []docparse.Heading{
			{Level: 3, Text: "First", Slug: "first"},
			{Level: 4, Text: "Second", Slug: "second"},
		}
		got := GenerateToC(headings, 1)
		want := "- [First](#first)\n  - [Second](#second)\n"
		if got != want {
			t.Errorf("GenerateToC() =\n%q\nwant:\n%q", got, want)
		}
	})

	t.Run("below min_headings returns empty", func(t *testing.T) {
		t.Parallel()
		headings := []docparse.Heading{
			{Level: 2, Text: "Only One", Slug: "only-one"},
		}
		got := GenerateToC(headings, 3)
		if got != "" {
			t.Errorf("GenerateToC() = %q, want empty string", got)
		}
	})

	t.Run("empty input returns empty", func(t *testing.T) {
		t.Parallel()
		got := GenerateToC(nil, 1)
		if got != "" {
			t.Errorf("GenerateToC(nil) = %q, want empty string", got)
		}
	})

	t.Run("empty input with zero min_headings returns empty", func(t *testing.T) {
		t.Parallel()
		// min_headings: 0 in config plus a doc whose post-marker
		// content has no headings must not panic (frozen public API).
		got := GenerateToC(nil, 0)
		if got != "" {
			t.Errorf("GenerateToC(nil, 0) = %q, want empty string", got)
		}
	})

	t.Run("zero min_headings generates toc", func(t *testing.T) {
		t.Parallel()
		headings := []docparse.Heading{
			{Level: 2, Text: "One", Slug: "one"},
		}
		got := GenerateToC(headings, 0)
		if got == "" {
			t.Error("GenerateToC() returned empty, want non-empty")
		}
	})
}

func TestUpdateToC(t *testing.T) {
	t.Parallel()
	t.Run("markers present with headings", func(t *testing.T) {
		t.Parallel()
		content := "# Title\n\n" +
			BeginMarker + "\n" +
			EndMarker + "\n\n" +
			"## First\n\n" +
			"## Second\n"

		res := UpdateToC(content, 1)
		if !res.Found {
			t.Fatal("UpdateToC() Found = false, want true")
		}
		if !containsAll(res.Updated, "- [First](#first)", "- [Second](#second)") {
			t.Errorf("UpdateToC() missing expected ToC entries:\n%s", res.Updated)
		}
		// Verify markers are preserved.
		if !containsAll(res.Updated, BeginMarker, EndMarker) {
			t.Error("markers not preserved")
		}
		// Headings should be surfaced for callers that need them.
		if len(res.Headings) != 2 {
			t.Errorf("Headings len = %d, want 2", len(res.Headings))
		}
	})

	t.Run("markers present but below threshold", func(t *testing.T) {
		t.Parallel()
		content := "# Title\n\n" +
			BeginMarker + "\n" +
			EndMarker + "\n\n" +
			"## Only One\n"

		res := UpdateToC(content, 3)
		if !res.Found {
			t.Fatal("UpdateToC() Found = false, want true")
		}
		// ToC should be empty between markers.
		expected := BeginMarker + "\n" + EndMarker
		if !strings.Contains(res.Updated, expected) {
			t.Errorf("expected empty ToC between markers, got:\n%s", res.Updated)
		}
		// Headings still returned even when below threshold so the
		// dry-run path can report what would have been generated.
		if len(res.Headings) != 1 {
			t.Errorf("Headings len = %d, want 1", len(res.Headings))
		}
	})

	t.Run("no headings after markers with zero threshold", func(t *testing.T) {
		t.Parallel()
		// All post-marker content fenced -> zero headings; with
		// minHeadings 0 this used to panic in GenerateToC.
		content := "# Title\n\n" +
			BeginMarker + "\n" +
			EndMarker + "\n\n" +
			"```\n## Fenced Only\n```\n"

		res := UpdateToC(content, 0)
		if !res.Found {
			t.Fatal("UpdateToC() Found = false, want true")
		}
		if len(res.Headings) != 0 {
			t.Errorf("Headings len = %d, want 0", len(res.Headings))
		}
		if !strings.Contains(res.Updated, BeginMarker+"\n"+EndMarker) {
			t.Errorf("expected empty ToC between markers, got:\n%s", res.Updated)
		}
	})

	t.Run("headings before the end marker are excluded", func(t *testing.T) {
		t.Parallel()
		// The slice-past-EndMarker policy lives here (parseHeadings),
		// not in docparse.Headings, which walks its whole input.
		content := "## Preamble Section\n\n" +
			BeginMarker + "\n" +
			"- [Old Entry](#old-entry)\n" +
			EndMarker + "\n\n" +
			"## Real Section\n"

		res := UpdateToC(content, 1)
		if !res.Found {
			t.Fatal("UpdateToC() Found = false, want true")
		}
		if strings.Contains(res.Updated, "- [Preamble Section](#preamble-section)") {
			t.Error("pre-marker heading leaked into the generated ToC")
		}
		if len(res.Headings) != 1 || res.Headings[0].Text != "Real Section" {
			t.Errorf("Headings = %+v, want only the post-marker heading", res.Headings)
		}
	})

	t.Run("no markers returns original", func(t *testing.T) {
		t.Parallel()
		content := "# Title\n\n## Section\n"
		res := UpdateToC(content, 1)
		if res.Found {
			t.Error("UpdateToC() Found = true, want false")
		}
		if res.Updated != content {
			t.Errorf("content was modified when no markers present")
		}
		if res.Headings != nil {
			t.Errorf("Headings = %v, want nil when no markers", res.Headings)
		}
	})

	t.Run("existing ToC content gets replaced", func(t *testing.T) {
		t.Parallel()
		content := "# Title\n\n" +
			BeginMarker + "\n" +
			"- [Old Entry](#old-entry)\n" +
			EndMarker + "\n\n" +
			"## New Entry\n"

		res := UpdateToC(content, 1)
		if !res.Found {
			t.Fatal("UpdateToC() Found = false, want true")
		}
		if strings.Contains(res.Updated, "Old Entry") {
			t.Error("old ToC entry was not replaced")
		}
		if !strings.Contains(res.Updated, "- [New Entry](#new-entry)") {
			t.Error("new ToC entry not found")
		}
	})

	t.Run("only begin marker no end", func(t *testing.T) {
		t.Parallel()
		content := "# Title\n\n" + BeginMarker + "\n## Section\n"
		res := UpdateToC(content, 1)
		if res.Found {
			t.Error("UpdateToC() Found = true, want false (missing end marker)")
		}
		if res.Updated != content {
			t.Error("content was modified with missing end marker")
		}
	})
}

// containsAll checks that s contains all of the given substrings.
func containsAll(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}

// buildBenchDoc synthesizes a markdown document with n H2/H3 headings,
// realistic-looking body text between them, and the ToC marker pair.
// Used by BenchmarkUpdateToC to feed UpdateToC a representative input
// across 10/50/200 heading sizes.
func buildBenchDoc(numHeadings int) string {
	var sb strings.Builder
	sb.WriteString("# Bench Doc\n\n")
	sb.WriteString(BeginMarker)
	sb.WriteByte('\n')
	sb.WriteString(EndMarker)
	sb.WriteString("\n\n")
	for i := 1; i <= numHeadings; i++ {
		level := "## "
		if i%3 == 0 {
			level = "### "
		}
		sb.WriteString(level)
		sb.WriteString("Section ")
		sb.WriteString(strconv.Itoa(i))
		sb.WriteString("\n\nLorem ipsum dolor sit amet, consectetur adipiscing elit.\n\n")
	}
	return sb.String()
}

// BenchmarkUpdateToC measures UpdateToC cost on a document with
// 10 / 50 / 200 headings. Phase 1 baseline for IMPL-0007; IMPL-0014
// Phase 4 rewired the heading walk through docparse and this benchmark
// guards against regression on the hot parse + GenerateToC path.
func BenchmarkUpdateToC(b *testing.B) {
	for _, n := range []int{10, 50, 200} {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			content := buildBenchDoc(n)
			b.ResetTimer()
			b.ReportAllocs()
			for b.Loop() {
				res := UpdateToC(content, 3)
				if !res.Found {
					b.Fatal("markers not found in synthesized doc")
				}
			}
		})
	}
}
