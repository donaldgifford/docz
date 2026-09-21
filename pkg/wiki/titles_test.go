package wiki

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirTitle(t *testing.T) {
	t.Parallel()
	navTitles := map[string]string{
		"rfc":           "RFCs",
		"adr":           "ADRs",
		"impl":          "Implementation Plans",
		"investigation": "Investigations",
	}

	tests := []struct {
		dir  string
		want string
	}{
		{"rfc", "RFCs"},
		{"adr", "ADRs"},
		{"impl", "Implementation Plans"},
		{"investigation", "Investigations"},
		{"architecture", "Architecture"},
		{"getting-started", "Getting Started"},
		{"my_guides", "My_guides"},
		{"unknown", "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.dir, func(t *testing.T) {
			t.Parallel()
			got := DirTitle(tt.dir, navTitles)
			if got != tt.want {
				t.Errorf("DirTitle(%q) = %q, want %q", tt.dir, got, tt.want)
			}
		})
	}
}

func TestDirTitle_EmptyNavTitles(t *testing.T) {
	t.Parallel()
	got := DirTitle("getting-started", nil)
	if got != "Getting Started" {
		t.Errorf("DirTitle with nil navTitles = %q, want %q", got, "Getting Started")
	}
}

func TestDocTitle_Frontmatter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content := `---
id: RFC-0001
title: "API Rate Limiting"
status: Draft
author: Test
created: 2026-01-01
---
# RFC 0001: API Rate Limiting
`
	path := filepath.Join(dir, "0001-api-rate-limiting.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := DocTitle(path)
	if err != nil {
		t.Fatalf("DocTitle() error: %v", err)
	}
	if got != "RFC-0001: API Rate Limiting" {
		t.Errorf("DocTitle() = %q, want %q", got, "RFC-0001: API Rate Limiting")
	}
}

func TestDocTitle_H1Fallback(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content := `# System Overview

Some content here.
`
	path := filepath.Join(dir, "system-overview.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := DocTitle(path)
	if err != nil {
		t.Fatalf("DocTitle() error: %v", err)
	}
	if got != "System Overview" {
		t.Errorf("DocTitle() = %q, want %q", got, "System Overview")
	}
}

func TestDocTitle_FilenameFallback(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content := "Just some text without a heading.\n"
	path := filepath.Join(dir, "deployment-guide.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := DocTitle(path)
	if err != nil {
		t.Fatalf("DocTitle() error: %v", err)
	}
	if got != "Deployment Guide" {
		t.Errorf("DocTitle() = %q, want %q", got, "Deployment Guide")
	}
}

func TestDocTitle_NonexistentFile(t *testing.T) {
	t.Parallel()
	// Per Decisions §3, DocTitle now follows the standard "value OR
	// error" contract: a read failure returns "" alongside the error,
	// not a filename-derived fallback. Callers that want a fallback
	// must call FilenameTitle themselves.
	got, err := DocTitle("/nonexistent/file.md")
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
	if got != "" {
		t.Errorf("DocTitle() = %q on error, want empty string per the strict contract", got)
	}
}

func TestDocTitle_FrontmatterWithMarkdownDisable(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content := `---
id: DESIGN-0002
title: "Wiki Command"
status: Draft
author: Test
created: 2026-03-11
---
<!-- markdownlint-disable-file MD025 MD041 -->

# DESIGN-0002: Wiki Command
`
	path := filepath.Join(dir, "0002-wiki-command.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := DocTitle(path)
	if err != nil {
		t.Fatalf("DocTitle() error: %v", err)
	}
	if got != "DESIGN-0002: Wiki Command" {
		t.Errorf("DocTitle() = %q, want %q", got, "DESIGN-0002: Wiki Command")
	}
}

func TestFilenameTitle(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"system-overview.md", "System Overview"},
		{"deployment.md", "Deployment"},
		{"getting_started.md", "Getting Started"},
		{"README.md", "README"},
		{"0001-api-rate-limiting.md", "0001 Api Rate Limiting"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := FilenameTitle(tt.input)
			if got != tt.want {
				t.Errorf("FilenameTitle(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestDocTitle_H1Delegation pins DocTitle's H1 fallback now that the
// scan is docparse.Title rather than a second implementation local to
// this package (IMPL-0016 Phase 2, Decision 3). The first three cases
// are what the old firstH1 table covered; the rest are the delta —
// behavior the local scanner did not have, and the reason collapsing
// them is an improvement rather than a lateral move.
func TestDocTitle_H1Delegation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "simple heading", content: "# Hello World\n", want: "Hello World"},
		{
			name:    "heading after frontmatter",
			content: "---\ntitle: test\n---\n# Real Heading\n",
			want:    "Real Heading",
		},
		{name: "h2 only falls back to the filename", content: "## Not H1\n", want: "Nav Doc"},
		{
			name:    "heading with extra spaces",
			content: "#  Spaced  Title \n",
			want:    "Spaced  Title",
		},
		{name: "crlf heading", content: "# CRLF Heading\r\n", want: "CRLF Heading"},

		// The delta. firstH1 was not fence-aware and did not strip inline
		// markdown, so these two produced "# Not A Title"'s neighbor and a
		// nav entry with literal asterisks in it.
		{
			name:    "inline markdown is stripped",
			content: "# **Bold** Title\n",
			want:    "Bold Title",
		},
		{
			name:    "heading inside a fence is skipped",
			content: "```md\n# Not A Title\n```\n\n# Real Title\n",
			want:    "Real Title",
		},
		{
			// firstH1 toggled on every "---", so a mid-document rule put it
			// into frontmatter mode and hid the heading that followed.
			name:    "heading after a horizontal rule is found",
			content: "Intro.\n\n---\n\n# Real Title\n",
			want:    "Real Title",
		},
		{
			name:    "setext heading is found",
			content: "Setext Title\n============\n",
			want:    "Setext Title",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "nav-doc.md")
			if err := os.WriteFile(path, []byte(tt.content), 0o644); err != nil {
				t.Fatal(err)
			}

			got, err := DocTitle(path)
			if err != nil {
				t.Fatalf("DocTitle() error: %v", err)
			}
			if got != tt.want {
				t.Errorf("DocTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTitleCase(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"hello world", "Hello World"},
		{"getting started", "Getting Started"},
		{"ALREADY CAPS", "ALREADY CAPS"},
		{"", ""},
		{"single", "Single"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := titleCase(tt.input)
			if got != tt.want {
				t.Errorf("titleCase(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
