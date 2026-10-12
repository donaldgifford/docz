package docparse_test

import (
	"slices"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
)

func headingTexts(content string) []string {
	hs := docparse.Headings([]byte(content))
	out := make([]string, 0, len(hs))
	for _, h := range hs {
		out = append(out, h.Text)
	}
	return out
}

func TestHeadingsCommentBlocks(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name:    "heading inside a comment block is skipped",
			content: "## Before\n<!--\n## Hidden\n-->\n## After\n",
			want:    []string{"Before", "After"},
		},
		{
			name:    "comment block closing mid-line",
			content: "<!-- guidance\n## Hidden --> trailing\n## After\n",
			want:    []string{"After"},
		},
		{
			name:    "single-line comment opens nothing",
			content: "<!-- note -->\n## Kept\n",
			want:    []string{"Kept"},
		},
		{
			name:    "docz marker opens nothing",
			content: "<!--docz:phase:start-->\n### Phase 1: <!-- Foundation -->\n<!--docz:phase:end-->\n",
			want:    []string{"Phase 1: <!-- Foundation -->"},
		},
		{
			name:    "heading carrying a comment is kept",
			content: "## Foo <!-- c --> bar\n",
			want:    []string{"Foo <!-- c --> bar"},
		},
		{
			name:    "unclosed comment block runs to EOF",
			content: "## Kept\n<!--\n## Hidden\n## Also hidden\n",
			want:    []string{"Kept"},
		},
		{
			name:    "comment opener inside a fence opens nothing",
			content: "```\n<!--\n```\n## Kept\n",
			want:    []string{"Kept"},
		},
		{
			name:    "fence inside a comment block toggles nothing",
			content: "<!--\n```\n-->\n## Kept\n",
			want:    []string{"Kept"},
		},
		{
			name:    "indented comment opener",
			content: "  <!--\n## Hidden\n-->\n## Kept\n",
			want:    []string{"Kept"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := headingTexts(tt.content); !slices.Equal(got, tt.want) {
				t.Errorf("Headings texts = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTitleCommentBlocks(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "H1 in a comment block is skipped", content: "<!--\n# Hidden\n-->\n# Real\n", want: "Real"},
		{name: "only a commented H1", content: "<!--\n# Hidden\n-->\n", want: ""},
		{name: "setext title in a comment block", content: "<!--\nHidden\n===\n-->\nReal\n===\n", want: "Real"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := docparse.Title([]byte(tt.content)); got != tt.want {
				t.Errorf("Title = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTaskItemsCommentBlocks(t *testing.T) {
	t.Parallel()
	content := "- [ ] one\n<!--\n- [ ] hidden\n- [x] hidden too\n-->\n- [x] two\n<!-- - [ ] single-line comment -->\n"

	items := docparse.TaskItems([]byte(content))
	got := make([]string, 0, len(items))
	lines := make([]int, 0, len(items))
	for _, item := range items {
		got = append(got, item.Text)
		lines = append(lines, item.Line)
	}

	if want := []string{"one", "two"}; !slices.Equal(got, want) {
		t.Errorf("TaskItems texts = %q, want %q", got, want)
	}
	if want := []int{1, 6}; !slices.Equal(lines, want) {
		t.Errorf("TaskItems lines = %v, want %v", lines, want)
	}
}

func TestHeadingsSlugSource(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		heading  string
		wantText string
		wantSlug string
	}{
		{"trailing comment", "### Phase 1: <!-- Foundation / Setup -->", "Phase 1: <!-- Foundation / Setup -->", "phase-1-"},
		{"comment-only heading", "## <!-- only a comment -->", "<!-- only a comment -->", ""},
		{"inline tag", "## An <code>inline</code> tag", "An <code>inline</code> tag", "an-inline-tag"},
		{"closing sequence", "## Closing sequence ##", "Closing sequence", "closing-sequence"},
		{"hash without space is text", "## C#", "C#", "c"},
		{"intraword underscores", "## Use a_b_c names", "Use a_b_c names", "use-a_b_c-names"},
		{"underscore emphasis", "## _under_ and _more_", "under and more", "under-and-more"},
		{"code span underscore", "## The `code_span` heading", "The code_span heading", "the-code_span-heading"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			hs := docparse.Headings([]byte(tt.heading + "\n"))
			if len(hs) != 1 {
				t.Fatalf("Headings = %d headings, want 1", len(hs))
			}
			if hs[0].Text != tt.wantText || hs[0].Slug != tt.wantSlug {
				t.Errorf("Heading = {%q, %q}, want {%q, %q}", hs[0].Text, hs[0].Slug, tt.wantText, tt.wantSlug)
			}
		})
	}
}

// TestHeadingsDuplicateCollision pins github-slugger's duplicate rule,
// which docz-site's rehype-slug uses: a suffix already taken is skipped.
// markdownlint's MD051 does not skip, so anchors.md leaves this case out.
func TestHeadingsDuplicateCollision(t *testing.T) {
	t.Parallel()
	content := "## Dup\n## Dup-1\n## Dup\n## Dup\n## Dup 1\n"

	hs := docparse.Headings([]byte(content))
	got := make([]string, 0, len(hs))
	for _, h := range hs {
		got = append(got, h.Slug)
	}

	want := []string{"dup", "dup-1", "dup-2", "dup-3", "dup-1-1"}
	if !slices.Equal(got, want) {
		t.Errorf("slugs = %q, want %q", got, want)
	}
}
