package docparse_test

import (
	"testing"

	"github.com/donaldgifford/docz/pkg/doczcore/docparse"
)

func TestAnchorSlug(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
			got := docparse.AnchorSlug(tt.text)
			if got != tt.want {
				t.Errorf("AnchorSlug(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}

func TestHeadings(t *testing.T) {
	t.Parallel()
	t.Run("H2 through H6 levels with line numbers", func(t *testing.T) {
		t.Parallel()
		content := []byte("# Title\n" +
			"## Level 2\n" +
			"### Level 3\n" +
			"#### Level 4\n" +
			"##### Level 5\n" +
			"###### Level 6\n")

		headings := docparse.Headings(content)
		if len(headings) != 5 {
			t.Fatalf("got %d headings, want 5", len(headings))
		}

		for i, want := range []struct {
			level int
			line  int
		}{
			{2, 2}, {3, 3}, {4, 4}, {5, 5}, {6, 6},
		} {
			if headings[i].Level != want.level {
				t.Errorf("headings[%d].Level = %d, want %d", i, headings[i].Level, want.level)
			}
			if headings[i].Line != want.line {
				t.Errorf("headings[%d].Line = %d, want %d", i, headings[i].Line, want.line)
			}
		}
	})

	t.Run("skips H1", func(t *testing.T) {
		t.Parallel()
		content := []byte("# Title\n## Section\n")

		headings := docparse.Headings(content)
		if len(headings) != 1 {
			t.Fatalf("got %d headings, want 1", len(headings))
		}
		if headings[0].Text != "Section" {
			t.Errorf("heading text = %q, want %q", headings[0].Text, "Section")
		}
	})

	t.Run("walks the whole input including before toc markers", func(t *testing.T) {
		t.Parallel()
		// Unlike the internal ToC walker, Headings has no marker-based
		// skipping: a heading before <!--toc:end--> is a fact too.
		content := []byte("## Before Marker\n" +
			"<!--toc:start-->\n" +
			"<!--toc:end-->\n" +
			"## After Marker\n")

		headings := docparse.Headings(content)
		if len(headings) != 2 {
			t.Fatalf("got %d headings, want 2", len(headings))
		}
		if headings[0].Text != "Before Marker" || headings[0].Line != 1 {
			t.Errorf(
				"headings[0] = {Text: %q, Line: %d}, want {Text: \"Before Marker\", Line: 1}",
				headings[0].Text, headings[0].Line,
			)
		}
		if headings[1].Text != "After Marker" || headings[1].Line != 4 {
			t.Errorf(
				"headings[1] = {Text: %q, Line: %d}, want {Text: \"After Marker\", Line: 4}",
				headings[1].Text, headings[1].Line,
			)
		}
	})

	t.Run("skips headings inside fenced code blocks", func(t *testing.T) {
		t.Parallel()
		content := []byte("## Real Heading\n" +
			"```\n" +
			"## Fake Heading In Code\n" +
			"```\n" +
			"## Another Real Heading\n")

		headings := docparse.Headings(content)
		if len(headings) != 2 {
			t.Fatalf("got %d headings, want 2", len(headings))
		}
		if headings[0].Text != "Real Heading" {
			t.Errorf("headings[0].Text = %q, want %q", headings[0].Text, "Real Heading")
		}
		if headings[1].Text != "Another Real Heading" {
			t.Errorf(
				"headings[1].Text = %q, want %q",
				headings[1].Text, "Another Real Heading",
			)
		}
		if headings[1].Line != 5 {
			t.Errorf("headings[1].Line = %d, want 5", headings[1].Line)
		}
	})

	t.Run("skips headings in code blocks with language", func(t *testing.T) {
		t.Parallel()
		content := []byte("## Before\n" +
			"```go\n" +
			"## Not A Heading\n" +
			"```\n" +
			"## After\n")

		headings := docparse.Headings(content)
		if len(headings) != 2 {
			t.Fatalf("got %d headings, want 2", len(headings))
		}
	})

	t.Run("indented fence still toggles", func(t *testing.T) {
		t.Parallel()
		content := []byte("## Real\n" +
			"  ```\n" +
			"## Hidden\n" +
			"  ```\n")

		headings := docparse.Headings(content)
		if len(headings) != 1 {
			t.Fatalf("got %d headings, want 1", len(headings))
		}
	})

	t.Run("strips inline markdown", func(t *testing.T) {
		t.Parallel()
		content := []byte("## **Bold** Heading\n" +
			"## `Code` Heading\n" +
			"## [Link](http://example.com) Heading\n")

		headings := docparse.Headings(content)
		if len(headings) != 3 {
			t.Fatalf("got %d headings, want 3", len(headings))
		}
		for i, want := range []string{"Bold Heading", "Code Heading", "Link Heading"} {
			if headings[i].Text != want {
				t.Errorf("headings[%d].Text = %q, want %q", i, headings[i].Text, want)
			}
		}
	})

	t.Run("duplicate slug suffixes", func(t *testing.T) {
		t.Parallel()
		content := []byte("## Overview\n" +
			"## Details\n" +
			"## Overview\n" +
			"## Overview\n")

		headings := docparse.Headings(content)
		if len(headings) != 4 {
			t.Fatalf("got %d headings, want 4", len(headings))
		}
		for i, want := range []string{"overview", "details", "overview-1", "overview-2"} {
			if headings[i].Slug != want {
				t.Errorf("headings[%d].Slug = %q, want %q", i, headings[i].Slug, want)
			}
		}
	})

	t.Run("no trailing newline", func(t *testing.T) {
		t.Parallel()
		headings := docparse.Headings([]byte("## Last Line"))
		if len(headings) != 1 {
			t.Fatalf("got %d headings, want 1", len(headings))
		}
		if headings[0].Line != 1 {
			t.Errorf("Line = %d, want 1", headings[0].Line)
		}
	})

	t.Run("empty content", func(t *testing.T) {
		t.Parallel()
		if headings := docparse.Headings(nil); len(headings) != 0 {
			t.Fatalf("got %d headings, want 0", len(headings))
		}
	})
}
