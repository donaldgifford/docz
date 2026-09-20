package repo_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/docwrite"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/repo"
)

// statusDoc is a design document with an LF-terminated frontmatter block, the
// shape docwrite.SetStatus is willing to rewrite.
const statusDoc = `---
id: DESIGN-0001
title: A Design
status: Draft
created: 2026-01-01
author: Tester
---

# DESIGN-0001: A Design
`

// statusCRLFDoc is the same document with CRLF endings, which docwrite
// refuses: rewriting one line of a file whose endings docz does not
// understand would leave the rest of it inconsistent.
var statusCRLFDoc = strings.ReplaceAll(statusDoc, "\n", "\r\n")

// statusTestRepo builds a repository containing the two design documents the
// status table needs, and returns it with its root.
//
// The default config is used unmodified so the statuses under test are the
// real ones a repo gets, and so the disabled `plan` type is available to prove
// the disabled-type path without inventing a config.
func statusTestRepo(t *testing.T) (*repo.Repo, string) {
	t.Helper()

	root := t.TempDir()
	dir := filepath.Join(root, "docs", "design")

	if err := os.MkdirAll(dir, config.DirMode); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}

	statusTestWrite(t, filepath.Join(dir, "0001-a-design.md"), statusDoc)
	statusTestWrite(t, filepath.Join(dir, "0002-crlf.md"),
		strings.ReplaceAll(statusCRLFDoc, "DESIGN-0001", "DESIGN-0002"))

	cfg := config.DefaultConfig()

	return &repo.Repo{Root: root, Cfg: &cfg}, root
}

// statusTestWrite writes one fixture file, failing the test rather than
// returning an error a caller would have to check.
func statusTestWrite(t *testing.T, path, body string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(body), config.FileMode); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestSetStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		typeArg string
		id      string
		status  string
		opts    repo.StatusOptions
		wantOld string
		wantNew string
		changed bool
		// wantErr is checked with a per-case assertion rather than a
		// comparison, because every failure here is a typed error whose
		// fields are the thing worth asserting.
		wantErr func(t *testing.T, err error)
	}{
		{
			name:    "changed",
			typeArg: "design",
			id:      "DESIGN-0001",
			status:  "Approved",
			wantOld: "Draft",
			wantNew: "Approved",
			changed: true,
		},
		{
			name:    "alias resolves to the same type",
			typeArg: "DESIGN",
			id:      "DESIGN-0001",
			status:  "Approved",
			wantOld: "Draft",
			wantNew: "Approved",
			changed: true,
		},
		{
			name:    "unchanged does not write",
			typeArg: "design",
			id:      "DESIGN-0001",
			status:  "Draft",
			wantOld: "Draft",
			wantNew: "Draft",
			changed: false,
		},
		{
			name:    "dry run does not write",
			typeArg: "design",
			id:      "DESIGN-0001",
			status:  "Approved",
			opts:    repo.StatusOptions{DryRun: true},
			wantOld: "Draft",
			wantNew: "Approved",
			changed: false,
		},
		{
			name:    "id not found",
			typeArg: "design",
			id:      "DESIGN-9999",
			status:  "Approved",
			wantErr: func(t *testing.T, err error) {
				t.Helper()

				var notFound *repo.NotFoundError
				if !errors.As(err, &notFound) {
					t.Fatalf("want *NotFoundError, got %T: %v", err, err)
				}

				if notFound.Type != "design" || notFound.ID != "DESIGN-9999" {
					t.Errorf("got %+v, want type design id DESIGN-9999", notFound)
				}
			},
		},
		{
			name:    "id is case sensitive",
			typeArg: "design",
			id:      "design-0001",
			status:  "Approved",
			wantErr: func(t *testing.T, err error) {
				t.Helper()

				var notFound *repo.NotFoundError
				if !errors.As(err, &notFound) {
					t.Fatalf("want *NotFoundError, got %T: %v", err, err)
				}
			},
		},
		{
			name:    "invalid status",
			typeArg: "design",
			id:      "DESIGN-0001",
			status:  "Shipped",
			wantErr: func(t *testing.T, err error) {
				t.Helper()

				var invalid *repo.InvalidStatusError
				if !errors.As(err, &invalid) {
					t.Fatalf("want *InvalidStatusError, got %T: %v", err, err)
				}

				if invalid.Status != "Shipped" || invalid.Type != "design" {
					t.Errorf("got %+v, want type design status Shipped", invalid)
				}

				// Allowed is the reason this error is a struct: a caller can
				// list the valid values without reaching back into the config.
				want := config.DefaultConfig().Types["design"].Statuses
				if len(invalid.Allowed) != len(want) {
					t.Fatalf("Allowed = %v, want %v", invalid.Allowed, want)
				}

				for i := range want {
					if invalid.Allowed[i] != want[i] {
						t.Errorf("Allowed = %v, want %v", invalid.Allowed, want)

						break
					}
				}
			},
		},
		{
			name:    "unknown type",
			typeArg: "nonsense",
			id:      "DESIGN-0001",
			status:  "Approved",
			wantErr: func(t *testing.T, err error) {
				t.Helper()

				var unknown *repo.UnknownTypeError
				if !errors.As(err, &unknown) {
					t.Fatalf("want *UnknownTypeError, got %T: %v", err, err)
				}

				if !errors.Is(err, config.ErrUnknownType) {
					t.Error("want the frozen config.ErrUnknownType sentinel to answer errors.Is")
				}
			},
		},
		{
			name:    "disabled type",
			typeArg: "plan",
			id:      "PLAN-0001",
			status:  "Draft",
			wantErr: func(t *testing.T, err error) {
				t.Helper()

				var disabled *repo.TypeDisabledError
				if !errors.As(err, &disabled) {
					t.Fatalf("want *TypeDisabledError, got %T: %v", err, err)
				}

				if disabled.Type != "plan" {
					t.Errorf("Type = %q, want plan", disabled.Type)
				}
			},
		},
		{
			name:    "crlf document",
			typeArg: "design",
			id:      "DESIGN-0002",
			status:  "Approved",
			wantErr: func(t *testing.T, err error) {
				t.Helper()

				var write *repo.WriteError
				if !errors.As(err, &write) {
					t.Fatalf("want *WriteError, got %T: %v", err, err)
				}

				if write.Path != filepath.Join("docs", "design", "0002-crlf.md") {
					t.Errorf("Path = %q, want the repo-relative document path", write.Path)
				}

				// The wrapping has to stay transparent: cmd maps this
				// particular failure to its own exit code with errors.Is.
				if !errors.Is(err, docwrite.ErrUnsupportedLineEndings) {
					t.Error("want docwrite.ErrUnsupportedLineEndings to answer errors.Is through WriteError")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r, root := statusTestRepo(t)

			docPath := filepath.Join(root, "docs", "design", "0001-a-design.md")

			before, err := os.ReadFile(docPath)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}

			res, err := r.SetStatus(t.Context(), tt.typeArg, tt.id, tt.status, tt.opts)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("SetStatus succeeded, want an error (result %+v)", res)
				}

				tt.wantErr(t, err)

				return
			}

			if err != nil {
				t.Fatalf("SetStatus: %v", err)
			}

			if res.ID != tt.id || res.Type != "design" {
				t.Errorf("got ID %q type %q, want %q design", res.ID, res.Type, tt.id)
			}

			if res.Path != filepath.Join("docs", "design", "0001-a-design.md") {
				t.Errorf("Path = %q, want the repo-relative document path", res.Path)
			}

			if res.Old != tt.wantOld || res.New != tt.wantNew {
				t.Errorf("got %q -> %q, want %q -> %q", res.Old, res.New, tt.wantOld, tt.wantNew)
			}

			if res.Changed != tt.changed {
				t.Errorf("Changed = %v, want %v", res.Changed, tt.changed)
			}

			after, err := os.ReadFile(docPath)
			if err != nil {
				t.Fatalf("read document: %v", err)
			}

			// The file is the real assertion for the two no-write cases: a
			// result saying Changed:false while the bytes moved would be the
			// bug worth catching.
			switch {
			case tt.changed && bytes.Equal(after, before):
				t.Error("document unchanged, want the new status written")
			case !tt.changed && !bytes.Equal(after, before):
				t.Errorf("document rewritten, want it byte-identical:\n%s", after)
			}

			if tt.changed && !strings.Contains(string(after), "status: "+tt.wantNew) {
				t.Errorf("document does not carry the new status:\n%s", after)
			}
		})
	}
}

// TestSetStatus_LookupBeforeValidation pins the order the CLI's exit codes
// depend on: a document that does not exist is a lookup failure even when the
// status is also invalid, so the user is told to fix the id rather than the
// status.
func TestSetStatus_LookupBeforeValidation(t *testing.T) {
	t.Parallel()

	r, _ := statusTestRepo(t)

	_, err := r.SetStatus(t.Context(), "design", "DESIGN-9999", "Shipped", repo.StatusOptions{})

	var notFound *repo.NotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("want *NotFoundError for a missing id with a bad status, got %T: %v", err, err)
	}
}
