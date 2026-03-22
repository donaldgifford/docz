package toc

import (
	"strings"
	"testing"
)

func TestSlugify(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{name: "basic text", text: "Problem Statement", want: "problem-statement"},
		{name: "with colon", text: "Phase 1: Setup", want: "phase-1-setup"},
		{name: "with slash", text: "API / Interface Changes", want: "api--interface-changes"},
		{name: "special chars", text: "What's New?", want: "whats-new"},
		{name: "numbers", text: "Step 2 Details", want: "step-2-details"},
		{name: "leading trailing hyphens", text: "- test -", want: "test"},
		{name: "multiple spaces", text: "too  many   spaces", want: "too--many---spaces"},
		{name: "empty string", text: "", want: ""},
		{name: "unicode stripped", text: "Héllo Wörld", want: "hllo-wrld"},
		{name: "all special", text: "!@#$%", want: ""},
		{name: "parentheses", text: "Config (default)", want: "config-default"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Slugify(tt.text)
			if got != tt.want {
				t.Errorf("Slugify(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}

func TestStripInlineMarkdown(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{name: "bold asterisks", text: "**Bold** text", want: "Bold text"},
		{name: "bold underscores", text: "__Bold__ text", want: "Bold text"},
		{name: "italic asterisk", text: "*italic* text", want: "italic text"},
		{name: "italic underscore", text: "_italic_ text", want: "italic text"},
		{name: "inline code", text: "`code` text", want: "code text"},
		{name: "link", text: "[link text](http://example.com)", want: "link text"},
		{name: "mixed", text: "**Bold** and `code`", want: "Bold and code"},
		{name: "no formatting", text: "plain text", want: "plain text"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripInlineMarkdown(tt.text)
			if got != tt.want {
				t.Errorf("stripInlineMarkdown(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}

func TestParseHeadings(t *testing.T) {
	t.Run("H2 through H6 levels", func(t *testing.T) {
		content := EndMarker + "\n" +
			"## Level 2\n" +
			"### Level 3\n" +
			"#### Level 4\n" +
			"##### Level 5\n" +
			"###### Level 6\n"

		headings := ParseHeadings(content)
		if len(headings) != 5 {
			t.Fatalf("got %d headings, want 5", len(headings))
		}

		for i, want := range []int{2, 3, 4, 5, 6} {
			if headings[i].Level != want {
				t.Errorf("headings[%d].Level = %d, want %d", i, headings[i].Level, want)
			}
		}
	})

	t.Run("skips H1", func(t *testing.T) {
		content := EndMarker + "\n" +
			"# Title\n" +
			"## Section\n"

		headings := ParseHeadings(content)
		if len(headings) != 1 {
			t.Fatalf("got %d headings, want 1", len(headings))
		}
		if headings[0].Text != "Section" {
			t.Errorf("heading text = %q, want %q", headings[0].Text, "Section")
		}
	})

	t.Run("skips headings before end marker", func(t *testing.T) {
		content := "## Before Marker\n" +
			BeginMarker + "\n" +
			EndMarker + "\n" +
			"## After Marker\n"

		headings := ParseHeadings(content)
		if len(headings) != 1 {
			t.Fatalf("got %d headings, want 1", len(headings))
		}
		if headings[0].Text != "After Marker" {
			t.Errorf("heading text = %q, want %q", headings[0].Text, "After Marker")
		}
	})

	t.Run("skips headings inside fenced code blocks", func(t *testing.T) {
		content := EndMarker + "\n" +
			"## Real Heading\n" +
			"```\n" +
			"## Fake Heading In Code\n" +
			"```\n" +
			"## Another Real Heading\n"

		headings := ParseHeadings(content)
		if len(headings) != 2 {
			t.Fatalf("got %d headings, want 2", len(headings))
		}
		if headings[0].Text != "Real Heading" {
			t.Errorf("headings[0].Text = %q, want %q", headings[0].Text, "Real Heading")
		}
		if headings[1].Text != "Another Real Heading" {
			t.Errorf("headings[1].Text = %q, want %q", headings[1].Text, "Another Real Heading")
		}
	})

	t.Run("skips headings in code blocks with language", func(t *testing.T) {
		content := EndMarker + "\n" +
			"## Before\n" +
			"```go\n" +
			"## Not A Heading\n" +
			"```\n" +
			"## After\n"

		headings := ParseHeadings(content)
		if len(headings) != 2 {
			t.Fatalf("got %d headings, want 2", len(headings))
		}
	})

	t.Run("strips inline markdown", func(t *testing.T) {
		content := EndMarker + "\n" +
			"## **Bold** Heading\n" +
			"## `Code` Heading\n" +
			"## [Link](http://example.com) Heading\n"

		headings := ParseHeadings(content)
		if len(headings) != 3 {
			t.Fatalf("got %d headings, want 3", len(headings))
		}
		if headings[0].Text != "Bold Heading" {
			t.Errorf("headings[0].Text = %q, want %q", headings[0].Text, "Bold Heading")
		}
		if headings[1].Text != "Code Heading" {
			t.Errorf("headings[1].Text = %q, want %q", headings[1].Text, "Code Heading")
		}
		if headings[2].Text != "Link Heading" {
			t.Errorf("headings[2].Text = %q, want %q", headings[2].Text, "Link Heading")
		}
	})

	t.Run("duplicate slug suffixes", func(t *testing.T) {
		content := EndMarker + "\n" +
			"## Overview\n" +
			"## Details\n" +
			"## Overview\n" +
			"## Overview\n"

		headings := ParseHeadings(content)
		if len(headings) != 4 {
			t.Fatalf("got %d headings, want 4", len(headings))
		}
		if headings[0].Slug != "overview" {
			t.Errorf("headings[0].Slug = %q, want %q", headings[0].Slug, "overview")
		}
		if headings[1].Slug != "details" {
			t.Errorf("headings[1].Slug = %q, want %q", headings[1].Slug, "details")
		}
		if headings[2].Slug != "overview-1" {
			t.Errorf("headings[2].Slug = %q, want %q", headings[2].Slug, "overview-1")
		}
		if headings[3].Slug != "overview-2" {
			t.Errorf("headings[3].Slug = %q, want %q", headings[3].Slug, "overview-2")
		}
	})

	t.Run("no end marker parses from start", func(t *testing.T) {
		content := "## First\n## Second\n"
		headings := ParseHeadings(content)
		if len(headings) != 2 {
			t.Fatalf("got %d headings, want 2", len(headings))
		}
	})

	t.Run("empty content", func(t *testing.T) {
		headings := ParseHeadings("")
		if len(headings) != 0 {
			t.Fatalf("got %d headings, want 0", len(headings))
		}
	})
}

func TestGenerateToC(t *testing.T) {
	t.Run("single level", func(t *testing.T) {
		headings := []Heading{
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
		headings := []Heading{
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
		headings := []Heading{
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
		headings := []Heading{
			{Level: 2, Text: "Only One", Slug: "only-one"},
		}
		got := GenerateToC(headings, 3)
		if got != "" {
			t.Errorf("GenerateToC() = %q, want empty string", got)
		}
	})

	t.Run("empty input returns empty", func(t *testing.T) {
		got := GenerateToC(nil, 1)
		if got != "" {
			t.Errorf("GenerateToC(nil) = %q, want empty string", got)
		}
	})

	t.Run("zero min_headings generates toc", func(t *testing.T) {
		headings := []Heading{
			{Level: 2, Text: "One", Slug: "one"},
		}
		got := GenerateToC(headings, 0)
		if got == "" {
			t.Error("GenerateToC() returned empty, want non-empty")
		}
	})
}

func TestUpdateToC(t *testing.T) {
	t.Run("markers present with headings", func(t *testing.T) {
		content := "# Title\n\n" +
			BeginMarker + "\n" +
			EndMarker + "\n\n" +
			"## First\n\n" +
			"## Second\n"

		got, found := UpdateToC(content, 1)
		if !found {
			t.Fatal("UpdateToC() found = false, want true")
		}
		if !containsAll(got, "- [First](#first)", "- [Second](#second)") {
			t.Errorf("UpdateToC() missing expected ToC entries:\n%s", got)
		}
		// Verify markers are preserved.
		if !containsAll(got, BeginMarker, EndMarker) {
			t.Error("markers not preserved")
		}
	})

	t.Run("markers present but below threshold", func(t *testing.T) {
		content := "# Title\n\n" +
			BeginMarker + "\n" +
			EndMarker + "\n\n" +
			"## Only One\n"

		got, found := UpdateToC(content, 3)
		if !found {
			t.Fatal("UpdateToC() found = false, want true")
		}
		// ToC should be empty between markers.
		expected := BeginMarker + "\n" + EndMarker
		if !strings.Contains(got, expected) {
			t.Errorf("expected empty ToC between markers, got:\n%s", got)
		}
	})

	t.Run("no markers returns original", func(t *testing.T) {
		content := "# Title\n\n## Section\n"
		got, found := UpdateToC(content, 1)
		if found {
			t.Error("UpdateToC() found = true, want false")
		}
		if got != content {
			t.Errorf("content was modified when no markers present")
		}
	})

	t.Run("existing ToC content gets replaced", func(t *testing.T) {
		content := "# Title\n\n" +
			BeginMarker + "\n" +
			"- [Old Entry](#old-entry)\n" +
			EndMarker + "\n\n" +
			"## New Entry\n"

		got, found := UpdateToC(content, 1)
		if !found {
			t.Fatal("UpdateToC() found = false, want true")
		}
		if strings.Contains(got, "Old Entry") {
			t.Error("old ToC entry was not replaced")
		}
		if !strings.Contains(got, "- [New Entry](#new-entry)") {
			t.Error("new ToC entry not found")
		}
	})

	t.Run("only begin marker no end", func(t *testing.T) {
		content := "# Title\n\n" + BeginMarker + "\n## Section\n"
		got, found := UpdateToC(content, 1)
		if found {
			t.Error("UpdateToC() found = true, want false (missing end marker)")
		}
		if got != content {
			t.Error("content was modified with missing end marker")
		}
	})
}

func TestItoa(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{1, "1"},
		{9, "9"},
		{10, "10"},
		{123, "123"},
	}

	for _, tt := range tests {
		got := itoa(tt.n)
		if got != tt.want {
			t.Errorf("itoa(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
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
