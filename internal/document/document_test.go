package document

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestParseFrontmatter(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    Frontmatter
		wantErr bool
		errIs   error
	}{
		{
			name: "valid frontmatter",
			content: `---
id: RFC-0001
title: "Test RFC"
status: Draft
author: Test Author
created: "2026-02-22"
---

# Body content here
`,
			want: Frontmatter{
				ID:      "RFC-0001",
				Title:   "Test RFC",
				Status:  "Draft",
				Author:  "Test Author",
				Created: "2026-02-22",
			},
		},
		{
			name: "minimal frontmatter",
			content: `---
id: ADR-0001
title: "Minimal"
---

Body
`,
			want: Frontmatter{
				ID:    "ADR-0001",
				Title: "Minimal",
			},
		},
		{
			name:    "no frontmatter",
			content: "# Just a heading\n\nSome content.",
			wantErr: true,
			errIs:   ErrNoFrontmatter,
		},
		{
			name:    "empty content",
			content: "",
			wantErr: true,
			errIs:   ErrNoFrontmatter,
		},
		{
			name: "unclosed frontmatter",
			content: `---
id: RFC-0001
title: "Unclosed"
`,
			wantErr: true,
		},
		{
			name: "frontmatter with leading newlines",
			content: `

---
id: RFC-0001
title: "Leading newlines"
---
`,
			want: Frontmatter{
				ID:    "RFC-0001",
				Title: "Leading newlines",
			},
		},
		{
			// IMPL-0006 Phase 9: a docs file authored on Windows uses
			// CRLF line endings; ParseFrontmatter previously rejected
			// the opening `---\r\n`.
			name:    "frontmatter with CRLF line endings",
			content: "---\r\nid: RFC-0001\r\ntitle: \"CRLF\"\r\n---\r\n\r\nBody\r\n",
			want: Frontmatter{
				ID:    "RFC-0001",
				Title: "CRLF",
			},
		},
		{
			// Mixed line endings (LF for header, CRLF for body) must
			// still parse. The opening `---\n` already worked; pin it
			// alongside the CRLF case to lock the contract.
			name:    "frontmatter with mixed line endings",
			content: "---\nid: RFC-0001\ntitle: \"Mixed\"\n---\r\n\r\nBody\r\n",
			want: Frontmatter{
				ID:    "RFC-0001",
				Title: "Mixed",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseFrontmatter([]byte(tt.content))
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errIs != nil && !errors.Is(err, tt.errIs) {
					t.Errorf("error = %v, want %v", err, tt.errIs)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.ID != tt.want.ID {
				t.Errorf("ID = %q, want %q", got.ID, tt.want.ID)
			}
			if got.Title != tt.want.Title {
				t.Errorf("Title = %q, want %q", got.Title, tt.want.Title)
			}
			if got.Status != tt.want.Status {
				t.Errorf("Status = %q, want %q", got.Status, tt.want.Status)
			}
			if got.Author != tt.want.Author {
				t.Errorf("Author = %q, want %q", got.Author, tt.want.Author)
			}
			if got.Created != tt.want.Created {
				t.Errorf("Created = %q, want %q", got.Created, tt.want.Created)
			}
		})
	}
}

func TestLoadFrontmatter_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.md")
	content := []byte("---\nid: RFC-0001\ntitle: \"Hello\"\nstatus: Draft\nauthor: T\ncreated: 2026-01-01\n---\n# Body\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	fm, got, err := LoadFrontmatter(path)
	if err != nil {
		t.Fatalf("LoadFrontmatter() error: %v", err)
	}
	if fm.ID != "RFC-0001" || fm.Title != "Hello" {
		t.Errorf("frontmatter = %+v", fm)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("returned bytes do not equal file content")
	}
}

func TestLoadFrontmatter_NoFrontmatterReturnsSentinel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.md")
	content := []byte("# Just a heading, no frontmatter\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	fm, got, err := LoadFrontmatter(path)
	if !errors.Is(err, ErrNoFrontmatter) {
		t.Fatalf("err = %v, want ErrNoFrontmatter wrapped", err)
	}
	if fm != (Frontmatter{}) {
		t.Errorf("expected zero Frontmatter, got %+v", fm)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("bytes should still be returned on no-frontmatter so callers can fall back")
	}
}

func TestLoadFrontmatter_ReadError(t *testing.T) {
	_, _, err := LoadFrontmatter("/definitely/does/not/exist/load-fm-test.md")
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
	if errors.Is(err, ErrNoFrontmatter) {
		t.Errorf("read error should not be classified as ErrNoFrontmatter: %v", err)
	}
}

func TestLoadFrontmatter_ParseError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.md")
	// Frontmatter opens but never closes.
	content := []byte("---\nid: RFC-0001\nno closing delimiter\n# Body\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, err := LoadFrontmatter(path)
	if err == nil {
		t.Fatal("expected parse error for unterminated frontmatter, got nil")
	}
	if errors.Is(err, ErrNoFrontmatter) {
		t.Errorf("parse error should not be classified as ErrNoFrontmatter: %v", err)
	}
}
