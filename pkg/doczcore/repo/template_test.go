package repo_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/doctemplate"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/repo"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/validate"
)

// templateTestCustomType is a type with nothing embedded for it, which is what
// makes ExportTemplate scaffold rather than copy.
const templateTestCustomType = "frameworks"

// templateTestRepo returns a Repo over an empty directory whose config carries
// the custom type beside the built-ins.
func templateTestRepo(t *testing.T) (*repo.Repo, string) {
	t.Helper()

	root := t.TempDir()
	cfg := config.DefaultConfig()

	cfg.Types[templateTestCustomType] = config.TypeConfig{
		Enabled:  true,
		Dir:      templateTestCustomType,
		IDPrefix: "FW",
		IDWidth:  4,
		Statuses: []string{"Draft", "Active"},
	}

	return &repo.Repo{Root: root, Cfg: &cfg}, root
}

// templateTestWrite writes body at path under root, creating the parents.
func templateTestWrite(t *testing.T, path, body string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), config.DirMode); err != nil {
		t.Fatalf("mkdir for %s: %v", path, err)
	}

	if err := os.WriteFile(path, []byte(body), config.FileMode); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestTemplate_ResolvesTheEmbeddedDefault(t *testing.T) {
	t.Parallel()

	r, _ := templateTestRepo(t)

	body, err := r.Template(t.Context(), "adr")
	if err != nil {
		t.Fatalf("Template: %v", err)
	}

	want, err := doctemplate.EmbeddedDocumentTemplate("adr")
	if err != nil {
		t.Fatalf("EmbeddedDocumentTemplate: %v", err)
	}

	if body != want {
		t.Error("Template did not return the embedded ADR template")
	}
}

func TestTemplate_LocalOverrideWins(t *testing.T) {
	t.Parallel()

	r, root := templateTestRepo(t)

	const override = "# mine\n"

	// Under Root rather than the process working directory: that is the whole
	// reason repo joins every config-relative path itself.
	templateTestWrite(t, filepath.Join(root, "docs", config.TemplatesDir, "adr.md"), override)

	body, err := r.Template(t.Context(), "adr")
	if err != nil {
		t.Fatalf("Template: %v", err)
	}

	if body != override {
		t.Errorf("body = %q, want the repo-local override", body)
	}
}

func TestTemplate_AliasAndErrors(t *testing.T) {
	t.Parallel()

	r, _ := templateTestRepo(t)

	if _, err := r.Template(t.Context(), "inv"); err != nil {
		t.Errorf("Template(inv): %v, want the investigation template", err)
	}

	_, err := r.Template(t.Context(), "nonsense")

	var unknown *repo.UnknownTypeError
	if !errors.As(err, &unknown) {
		t.Errorf("want *UnknownTypeError, got %T: %v", err, err)
	}

	// A custom type with nothing embedded, no config path, and no file is
	// ErrNoTemplate — issue #92's clear error, and what ExportTemplate branches
	// on to scaffold.
	if _, err := r.Template(t.Context(), templateTestCustomType); !errors.Is(err, doctemplate.ErrNoTemplate) {
		t.Errorf("err = %v, want doctemplate.ErrNoTemplate", err)
	}
}

func TestExportTemplate_ExplicitDest(t *testing.T) {
	t.Parallel()

	r, root := templateTestRepo(t)

	res, err := r.ExportTemplate(t.Context(), "adr", "out/adr.md", repo.ExportOptions{})
	if err != nil {
		t.Fatalf("ExportTemplate: %v", err)
	}

	if res.Path != filepath.Join("out", "adr.md") {
		t.Errorf("Path = %q, want the repo-relative destination", res.Path)
	}

	if res.Overwritten || res.Scaffolded || res.SchemaPath != "" {
		t.Errorf("res = %+v, want a plain first write", res)
	}

	body, err := os.ReadFile(filepath.Join(root, res.Path))
	if err != nil {
		t.Fatalf("read export: %v", err)
	}

	want, err := doctemplate.EmbeddedDocumentTemplate("adr")
	if err != nil {
		t.Fatalf("EmbeddedDocumentTemplate: %v", err)
	}

	if string(body) != want {
		t.Error("the exported file is not the resolved template")
	}
}

// An empty destination is `docz template override`: the file the resolver picks
// up next time, so the export round-trips through Template.
func TestExportTemplate_EmptyDestIsTheOverride(t *testing.T) {
	t.Parallel()

	r, root := templateTestRepo(t)

	res, err := r.ExportTemplate(t.Context(), "adr", "", repo.ExportOptions{})
	if err != nil {
		t.Fatalf("ExportTemplate: %v", err)
	}

	want := filepath.Join("docs", config.TemplatesDir, "adr.md")
	if res.Path != want {
		t.Fatalf("Path = %q, want %q", res.Path, want)
	}

	if _, err := os.Stat(filepath.Join(root, want)); err != nil {
		t.Fatalf("override not written: %v", err)
	}

	body, err := r.Template(t.Context(), "adr")
	if err != nil {
		t.Fatalf("Template after export: %v", err)
	}

	if !strings.Contains(body, "<!--docz:") {
		t.Error("the re-resolved template does not carry region markers")
	}
}

func TestExportTemplate_ExistingDest(t *testing.T) {
	t.Parallel()

	t.Run("refuses without overwrite", func(t *testing.T) {
		t.Parallel()

		r, root := templateTestRepo(t)

		dest := filepath.Join(root, "out", "adr.md")
		templateTestWrite(t, dest, "mine\n")

		_, err := r.ExportTemplate(t.Context(), "adr", "out/adr.md", repo.ExportOptions{})

		var exists *repo.ExistsError
		if !errors.As(err, &exists) {
			t.Fatalf("want *ExistsError, got %T: %v", err, err)
		}

		if exists.Path != filepath.Join("out", "adr.md") {
			t.Errorf("Path = %q, want the repo-relative destination", exists.Path)
		}

		body, readErr := os.ReadFile(dest)
		if readErr != nil {
			t.Fatalf("read %s: %v", dest, readErr)
		}

		if string(body) != "mine\n" {
			t.Error("the refused export wrote the file anyway")
		}
	})

	t.Run("overwrites when asked", func(t *testing.T) {
		t.Parallel()

		r, root := templateTestRepo(t)

		dest := filepath.Join(root, "out", "adr.md")
		templateTestWrite(t, dest, "mine\n")

		res, err := r.ExportTemplate(t.Context(), "adr", "out/adr.md", repo.ExportOptions{Overwrite: true})
		if err != nil {
			t.Fatalf("ExportTemplate: %v", err)
		}

		if !res.Overwritten {
			t.Error("Overwritten = false, want true for a destination that was there")
		}

		body, err := os.ReadFile(dest)
		if err != nil {
			t.Fatalf("read %s: %v", dest, err)
		}

		if string(body) == "mine\n" {
			t.Error("the file was reported overwritten but still holds its old content")
		}
	})
}

// A custom type with no template gets the generic pair instead of an error, so
// it ends up with the same two artifacts a built-in has (DESIGN-0015 §3).
func TestExportTemplate_ScaffoldsACustomType(t *testing.T) {
	t.Parallel()

	r, root := templateTestRepo(t)

	res, err := r.ExportTemplate(t.Context(), templateTestCustomType, "", repo.ExportOptions{})
	if err != nil {
		t.Fatalf("ExportTemplate: %v", err)
	}

	if !res.Scaffolded {
		t.Error("Scaffolded = false, want true for a type with no template")
	}

	wantTmpl := filepath.Join("docs", config.TemplatesDir, templateTestCustomType+".md")
	wantSchema := filepath.Join("docs", config.TemplatesDir, "schema", templateTestCustomType+".md")

	if res.Path != wantTmpl {
		t.Errorf("Path = %q, want %q", res.Path, wantTmpl)
	}

	if res.SchemaPath != wantSchema {
		t.Errorf("SchemaPath = %q, want %q", res.SchemaPath, wantSchema)
	}

	tmpl, err := os.ReadFile(filepath.Join(root, res.Path))
	if err != nil {
		t.Fatalf("read the scaffolded template: %v", err)
	}

	skeleton, err := os.ReadFile(filepath.Join(root, res.SchemaPath))
	if err != nil {
		t.Fatalf("read the scaffolded schema: %v", err)
	}

	// Both files resolve by name from here on, which is the point of writing
	// them where the resolvers look.
	if _, err := r.Template(t.Context(), templateTestCustomType); err != nil {
		t.Errorf("Template after scaffolding: %v", err)
	}

	resolved, err := doctemplate.ResolveSchema(templateTestCustomType, filepath.Join(root, "docs"))
	if err != nil {
		t.Fatalf("ResolveSchema after scaffolding: %v", err)
	}

	if !bytes.Equal(resolved, skeleton) {
		t.Error("ResolveSchema does not return the scaffolded skeleton")
	}

	templateTestAssertPairValidates(t, r, tmpl, skeleton)
}

// templateTestAssertPairValidates renders the scaffolded template the way
// `docz create` would and validates the result against the scaffolded schema.
//
// The template is rendered first because a raw one is not a document: its
// frontmatter holds {{ .Prefix }}-{{ .Number }} where an id belongs, and
// validating that would only prove text/template has not run yet. What the pair
// has to guarantee is that the document it produces satisfies the schema it
// ships with.
func templateTestAssertPairValidates(t *testing.T, r *repo.Repo, tmpl, skeleton []byte) {
	t.Helper()

	typ := r.Cfg.Types[templateTestCustomType]

	body, err := doctemplate.Render(string(tmpl), &doctemplate.Data{
		Number: "0001",
		Title:  "A Framework",
		Date:   "2026-01-01",
		Author: "Tester",
		Status: config.Status(typ.Statuses[0]),
		Type:   config.DocType(templateTestCustomType),
		Prefix: typ.IDPrefix,
	})
	if err != nil {
		t.Fatalf("rendering the scaffolded template: %v", err)
	}

	findings := validate.Document([]byte(body), validate.Options{
		Schema:      validate.SchemaFromMarkers(skeleton),
		Type:        typ,
		MinHeadings: r.Cfg.TOC.MinHeadings,
	})

	for i := range findings {
		if findings[i].Severity == validate.Error {
			t.Errorf("scaffolded pair produces an error finding: %s", findings[i])
		}
	}
}
