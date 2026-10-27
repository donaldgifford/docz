package repo

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/validate"
)

// documentUnmarked is a design document with real sections and no markers,
// which inference reads and InsertRegions would mark.
func documentUnmarked(id string) string {
	return regionsDoc(id,
		"# "+id+": Fixture",
		"",
		"## Overview",
		"",
		"Text.",
		"",
		"## Goals and Non-Goals",
		"",
		"### Goals",
		"",
		"- One.",
	)
}

// documentTreeDigest maps every file under root to the SHA-256 of its bytes.
func documentTreeDigest(t *testing.T, root string) map[string]string {
	t.Helper()

	out := make(map[string]string)

	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}

		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		sum := sha256.Sum256(body)
		out[path] = hex.EncodeToString(sum[:])

		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}

	return out
}

func TestValidateDocument(t *testing.T) {
	t.Parallel()

	t.Run("a broken sibling never reaches the report", func(t *testing.T) {
		t.Parallel()

		r := regionsFixtureRepo(t)
		body := documentUnmarked("DESIGN-0001")
		path := regionsPutDoc(t, r, "design", "0001-fixture.md", body)
		regionsPutDoc(t, r, "design", "0002-broken.md", "no frontmatter at all\n")

		got, err := r.ValidateDocument(t.Context(), path)
		if err != nil {
			t.Fatalf("ValidateDocument(%s) error = %v", path, err)
		}

		if want := filepath.Join("docs", "design", "0001-fixture.md"); got.Path != want {
			t.Errorf("ValidateDocument().Path = %q, want %q", got.Path, want)
		}

		if got.Type != "design" || got.ID != "DESIGN-0001" || got.Schema != "design" {
			t.Errorf("ValidateDocument() = type %q id %q schema %q, want design DESIGN-0001 design",
				got.Type, got.ID, got.Schema)
		}

		if string(got.Content) != body {
			t.Errorf("ValidateDocument().Content differs from the file")
		}

		sum := sha256.Sum256([]byte(body))
		if want := hex.EncodeToString(sum[:]); got.ContentSHA256 != want {
			t.Errorf("ValidateDocument().ContentSHA256 = %s, want %s", got.ContentSHA256, want)
		}

		for _, f := range got.Findings {
			if f.Code == "frontmatter.missing" {
				t.Errorf("ValidateDocument() reported the sibling's finding %v", f)
			}
		}

		if !documentHasCode(got.Findings, validate.CodeRegionInferred) {
			t.Errorf("ValidateDocument().Findings = %v, want %s", got.Findings, validate.CodeRegionInferred)
		}
	})

	t.Run("a repo-relative path resolves", func(t *testing.T) {
		t.Parallel()

		r := regionsFixtureRepo(t)
		regionsPutDoc(t, r, "design", "0001-fixture.md", documentUnmarked("DESIGN-0001"))

		rel := filepath.Join("docs", "design", "0001-fixture.md")

		got, err := r.ValidateDocument(t.Context(), rel)
		if err != nil {
			t.Fatalf("ValidateDocument(%s) error = %v", rel, err)
		}

		if got.Path != rel {
			t.Errorf("ValidateDocument().Path = %q, want %q", got.Path, rel)
		}
	})

	t.Run("no frontmatter is findings, not an error", func(t *testing.T) {
		t.Parallel()

		r := regionsFixtureRepo(t)
		body := "# Notes\n\n## Overview\n\nText.\n"
		path := regionsPutDoc(t, r, "design", "draft.md", body)

		got, err := r.ValidateDocument(t.Context(), path)
		if err != nil {
			t.Fatalf("ValidateDocument(%s) error = %v", path, err)
		}

		if !documentHasCode(got.Findings, "frontmatter.missing") {
			t.Errorf("ValidateDocument().Findings = %v, want frontmatter.missing", got.Findings)
		}

		if string(got.Content) != body {
			t.Errorf("ValidateDocument().Content differs from the file")
		}
	})

	refused := []struct {
		name  string
		setup func(t *testing.T, r *Repo) string
	}{
		{"a symlink escaping the root", func(t *testing.T, r *Repo) string {
			t.Helper()

			outside := filepath.Join(t.TempDir(), "secret.md")
			if err := os.WriteFile(outside, []byte(documentUnmarked("DESIGN-0009")), config.FileMode); err != nil {
				t.Fatal(err)
			}

			link := filepath.Join(r.TypeDir("design"), "0009-link.md")
			if err := os.MkdirAll(filepath.Dir(link), config.DirMode); err != nil {
				t.Fatal(err)
			}

			if err := os.Symlink(outside, link); err != nil {
				t.Fatal(err)
			}

			return link
		}},
		{"a path outside the root", func(t *testing.T, _ *Repo) string {
			t.Helper()

			outside := filepath.Join(t.TempDir(), "0001-x.md")
			if err := os.WriteFile(outside, []byte("x\n"), config.FileMode); err != nil {
				t.Fatal(err)
			}

			return outside
		}},
		{"a path under a disabled type", func(t *testing.T, r *Repo) string {
			t.Helper()

			path := regionsPutDoc(t, r, "rfc", "0001-fixture.md", regionsDoc("RFC-0001"))

			tc := r.Cfg.Types["rfc"]
			tc.Enabled = false
			r.Cfg.Types["rfc"] = tc

			return path
		}},
		{"a file outside every type directory", func(t *testing.T, r *Repo) string {
			t.Helper()

			path := filepath.Join(r.Root, "README.md")
			if err := os.WriteFile(path, []byte("x\n"), config.FileMode); err != nil {
				t.Fatal(err)
			}

			return path
		}},
	}

	for _, tt := range refused {
		t.Run(tt.name+" is a PathError", func(t *testing.T) {
			t.Parallel()

			r := regionsFixtureRepo(t)
			path := tt.setup(t, r)

			_, err := r.ValidateDocument(t.Context(), path)

			var pe *PathError
			if !errors.As(err, &pe) {
				t.Fatalf("ValidateDocument(%s) error = %v, want *PathError", path, err)
			}

			if _, err := r.InsertRegionsIn(t.Context(), []string{path}); !errors.As(err, &pe) {
				t.Errorf("InsertRegionsIn(%s) error = %v, want *PathError", path, err)
			}
		})
	}
}

func TestInsertRegionsIn(t *testing.T) {
	t.Parallel()

	r := regionsFixtureRepo(t)
	target := regionsPutDoc(t, r, "design", "0001-fixture.md", documentUnmarked("DESIGN-0001"))
	sibling := regionsPutDoc(t, r, "design", "0002-sibling.md", documentUnmarked("DESIGN-0002"))
	regionsPutDoc(t, r, "adr", "0001-other.md", regionsDoc("ADR-0001", "## Context", "", "Text."))

	before := documentTreeDigest(t, r.Root)

	report, err := r.InsertRegionsIn(t.Context(), []string{target})
	if err != nil {
		t.Fatalf("InsertRegionsIn(%s) error = %v", target, err)
	}

	change := regionsOnlyChange(t, &report)
	if want := filepath.Join("docs", "design", "0001-fixture.md"); change.Path != want {
		t.Errorf("InsertRegionsIn() changed %q, want %q", change.Path, want)
	}

	after := documentTreeDigest(t, r.Root)

	for path, sum := range before {
		changed := after[path] != sum
		if changed != (path == target) {
			t.Errorf("after InsertRegionsIn, %s changed = %v, want %v", path, changed, path == target)
		}
	}

	if after[sibling] != before[sibling] {
		t.Errorf("InsertRegionsIn touched the sibling %s", sibling)
	}

	again, err := r.InsertRegionsIn(t.Context(), []string{target})
	if err != nil || len(again.Changed) != 0 {
		t.Errorf("second InsertRegionsIn() = %v, %v; want no change", again.Changed, err)
	}
}

func documentHasCode(findings []validate.Finding, code string) bool {
	for _, f := range findings {
		if f.Code == code {
			return true
		}
	}

	return false
}
