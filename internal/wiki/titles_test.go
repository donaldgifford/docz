package wiki

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirTitle(t *testing.T) {
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
			got := DirTitle(tt.dir, navTitles)
			if got != tt.want {
				t.Errorf("DirTitle(%q) = %q, want %q", tt.dir, got, tt.want)
			}
		})
	}
}

func TestDirTitle_EmptyNavTitles(t *testing.T) {
	got := DirTitle("getting-started", nil)
	if got != "Getting Started" {
		t.Errorf("DirTitle with nil navTitles = %q, want %q", got, "Getting Started")
	}
}

func TestDocTitle_Frontmatter(t *testing.T) {
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
	_, err := DocTitle("/nonexistent/file.md")
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
}

func TestDocTitle_FrontmatterWithMarkdownDisable(t *testing.T) {
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
			got := FilenameTitle(tt.input)
			if got != tt.want {
				t.Errorf("FilenameTitle(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFirstH1(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"simple heading", "# Hello World\n", "Hello World"},
		{"heading after frontmatter", "---\ntitle: test\n---\n# Real Heading\n", "Real Heading"},
		{"no heading", "Just text\n", ""},
		{"h2 only", "## Not H1\n", ""},
		{"heading with extra spaces", "#  Spaced  Title \n", "Spaced  Title"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := firstH1([]byte(tt.content))
			if got != tt.want {
				t.Errorf("firstH1() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTitleCase(t *testing.T) {
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
			got := titleCase(tt.input)
			if got != tt.want {
				t.Errorf("titleCase(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
