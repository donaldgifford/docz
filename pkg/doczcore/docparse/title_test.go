package docparse_test

import (
	"strings"
	"testing"

	"github.com/donaldgifford/docz/pkg/doczcore/docparse"
)

// TestTitle covers the cases the corpus fixtures do not reach. The
// realistic documents are covered by the golden facts; this table is the
// pathological half.
func TestTitle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "empty input", content: "", want: ""},
		{name: "no h1", content: "## Section\n\nBody text.\n", want: ""},
		{name: "h1 on line 1", content: "# The Title\n\nBody.\n", want: "The Title"},
		{
			name:    "h1 further down",
			content: "Some preamble.\n\n# The Title\n\nBody.\n",
			want:    "The Title",
		},
		{
			// A ToC or a nav list can repeat the title; the first wins.
			name:    "multiple h1s take the first",
			content: "# First\n\n# Second\n",
			want:    "First",
		},
		{
			name:    "inline markdown stripped",
			content: "# **Bold** and `code` and [link](http://x)\n",
			want:    "Bold and code and link",
		},
		{
			name:    "h1 after frontmatter",
			content: "---\nid: RFC-0001\ntitle: \"Frontmatter Title\"\n---\n\n# Body Title\n",
			want:    "Body Title",
		},
		{
			// The frontmatter block is skipped, not scanned: a "# " line
			// inside it is a YAML comment, not the document's title.
			name:    "hash inside frontmatter is not a title",
			content: "---\n# a yaml comment\nid: RFC-0001\n---\n\n# Real Title\n",
			want:    "Real Title",
		},
		{
			// Only a document that opens with "---" has frontmatter.
			// Elsewhere it is a horizontal rule, and treating it as an
			// opener would hide every heading until the next rule.
			name:    "mid-document rule is not frontmatter",
			content: "Intro paragraph.\n\n---\n\n# The Title\n",
			want:    "The Title",
		},
		{
			name:    "unterminated frontmatter falls back to scanning",
			content: "---\nid: RFC-0001\n\n# The Title\n",
			want:    "The Title",
		},
		{
			// Both delimiters sit at column 0, so an indented "---" inside
			// a YAML block scalar does not end the block. Without that
			// rule the rest of the frontmatter is scanned as body and the
			// title becomes whatever string a key happens to hold — with
			// untrusted markdown, a value the repo author chose.
			name:    "indented dashes inside a block scalar do not close it",
			content: "---\nnotes: |\n  ---\n  # Injected\ntitle: Real\n---\n\n# The Title\n",
			want:    "The Title",
		},
		{
			// Matches document.ParseFrontmatter, which trims leading
			// newlines before looking for its delimiter. The two must
			// agree on what "opens with frontmatter" means, since
			// internal/wiki calls one and falls through to the other.
			name:    "leading blank line before frontmatter",
			content: "\n---\nid: X\n# owner: platform-team\n---\n\n# The Title\n",
			want:    "The Title",
		},
		{
			// A document that opens with a rule and closes with another
			// would otherwise have everything between them — including its
			// title — read as frontmatter and skipped. Real frontmatter
			// opens with a key, not a blank line.
			name:    "opening rule followed by a blank line is not frontmatter",
			content: "---\n\n# The Title\n\n---\n\nMore.\n",
			want:    "The Title",
		},

		// The fence rule, shared with Headings.
		{
			name:    "h1 inside a fence is skipped",
			content: "```md\n# Not A Title\n```\n\n# Real Title\n",
			want:    "Real Title",
		},
		{
			name:    "h1 only inside a fence yields nothing",
			content: "Prose.\n\n```md\n# Not A Title\n```\n",
			want:    "",
		},

		// Setext, which Headings deliberately does not support.
		{name: "setext h1", content: "The Title\n=========\n\nBody.\n", want: "The Title"},
		{name: "setext single equals", content: "The Title\n=\n", want: "The Title"},
		{
			name:    "setext underline is trimmed",
			content: "The Title\n   ===   \n",
			want:    "The Title",
		},
		{
			name:    "setext with inline markdown",
			content: "**Bold** Title\n====\n",
			want:    "Bold Title",
		},
		{
			// "---" underlines an H2, and an H2 is not a title.
			name:    "setext h2 is not a title",
			content: "Not The Title\n-------------\n\n# The Title\n",
			want:    "The Title",
		},
		{
			// A stray "===" after a deeper ATX heading is a paragraph, not
			// an underline.
			name:    "equals after an atx heading is not setext",
			content: "## Section\n===\n\n# The Title\n",
			want:    "The Title",
		},
		{
			name:    "blank line before equals is not setext",
			content: "Not a title\n\n===\n\n# The Title\n",
			want:    "The Title",
		},

		// The setext branch is the only one that can return arbitrary
		// line content, so a line that is not a paragraph must not
		// qualify — otherwise the title arrives with a bullet marker, a
		// blockquote arrow, or a tag in it.
		{
			name:    "list item is not setext text",
			content: "- alpha\n===\n\n# The Title\n",
			want:    "The Title",
		},
		{
			name:    "ordered list item is not setext text",
			content: "1. alpha\n===\n\n# The Title\n",
			want:    "The Title",
		},
		{
			name:    "blockquote is not setext text",
			content: "> quoted\n===\n\n# The Title\n",
			want:    "The Title",
		},
		{
			name:    "html is not setext text",
			content: "<div>\n====\n\n# The Title\n",
			want:    "The Title",
		},
		{
			name:    "table row is not setext text",
			content: "| a | b |\n=======\n\n# The Title\n",
			want:    "The Title",
		},
		{
			// A run of punctuation is a thematic break or an underline,
			// not text.
			name:    "thematic break is not setext text",
			content: "***\n===\n",
			want:    "",
		},
		{
			name:    "equals under equals is not setext text",
			content: "=\n=\n",
			want:    "",
		},
		{
			// Documented rather than fixed: a multi-line setext heading
			// yields only its last line.
			name:    "multiline setext yields the last line",
			content: "Line one\nLine two\n========\n",
			want:    "Line two",
		},
		{
			name:    "atx wins when it comes first",
			content: "# ATX Title\n\nSetext Title\n============\n",
			want:    "ATX Title",
		},
		{
			name:    "setext wins when it comes first",
			content: "Setext Title\n============\n\n# ATX Title\n",
			want:    "Setext Title",
		},

		// Shape rules shared with Headings.
		{name: "h2 is not a title", content: "## Section\n", want: ""},
		{name: "hash without a space is not a title", content: "#NotATitle\n", want: ""},
		{name: "hash alone is not a title", content: "#\n", want: ""},
		{name: "indented h1 still counts", content: "  # The Title\n", want: "The Title"},
		{name: "trailing whitespace trimmed", content: "#   The Title   \n", want: "The Title"},
		{
			name:    "crlf input keeps no carriage return",
			content: "# The Title\r\n",
			want:    "The Title",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := docparse.Title([]byte(tt.content)); got != tt.want {
				t.Errorf("Title() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestTitleAndHeadingsDoNotOverlap pins the division doc.go promises:
// the two walkers never report the same line. A document may legitimately
// carry both "# X" and "## X", so the invariant is per line, not per
// text — which is why this compares against the raw fixture lines rather
// than against Title's return value.
func TestTitleAndHeadingsDoNotOverlap(t *testing.T) {
	t.Parallel()

	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			content := readFixture(t, name)
			title := docparse.Title(content)
			if title == "" {
				t.Fatalf("Title() = %q; the corpus fixtures all have one", title)
			}

			lines := strings.Split(string(content), "\n")
			for _, h := range docparse.Headings(content) {
				if h.Level < 2 {
					t.Errorf("Headings() reported level %d; H1 belongs to Title", h.Level)
				}
				raw := strings.TrimSpace(lines[h.Line-1])
				if strings.HasPrefix(raw, "# ") {
					t.Errorf("Headings() reported line %d (%q), which is an H1", h.Line, raw)
				}
			}
		})
	}
}

// FuzzTitle pins the contract the package makes for every fact
// extractor: arbitrary bytes in, a value out, never a panic. The two
// shape assertions are what a consumer relies on when it puts the result
// in a nav entry or a page heading — one line, already trimmed.
//
// The precedent is document.FuzzParseChangelog. Title is the more
// exposed surface of the two, since it runs over markdown fetched from
// repos docz does not control.
func FuzzTitle(f *testing.F) {
	seeds := []string{
		"",
		"# The Title\n",
		"---\nid: X\n---\n\n# The Title\n",
		"---\nnotes: |\n  ---\n  # Injected\n---\n\n# The Title\n",
		"Setext\n======\n",
		"```md\n# Fenced\n```\n",
		"- item\n===\n",
		"#\n#\n#\n",
		"---\n",
		"\r\n# CR\r\n",
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		got := docparse.Title(content)

		if strings.Contains(got, "\n") {
			t.Errorf("Title() = %q, want a single line", got)
		}
		if got != strings.TrimSpace(got) {
			t.Errorf("Title() = %q, want it already trimmed", got)
		}
	})
}
