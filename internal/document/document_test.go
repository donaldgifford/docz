package document

import (
	"errors"
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
