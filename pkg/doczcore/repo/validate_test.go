package repo

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/doctemplate"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/document"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/index"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/toc"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/validate"
)

// Every helper here is prefixed vtest*, because three packages' worth of
// tests share this directory and a generically named helper would collide
// with a sibling file's.

// The fixture document every test writes: one title, and the filename `docz
// create` would give it.
const (
	vtestTitle   = "A placeholder title"
	vtestDocName = "0001-a-placeholder-title.md"
)

// vtestRepo returns a Repo over an empty temp root with the default config.
func vtestRepo(t *testing.T) *Repo {
	t.Helper()

	cfg := config.DefaultConfig()

	return &Repo{Root: t.TempDir(), Cfg: &cfg}
}

// vtestDocsDir is the repository's docs directory.
func vtestDocsDir(r *Repo) string {
	return r.Path(r.Cfg.DocsDir)
}

// vtestRender renders a type's resolved template the way `docz create` does,
// so a fixture document is the document a user would have.
func vtestRender(t *testing.T, r *Repo, typeName, number, title string) []byte {
	t.Helper()

	tmpl, err := doctemplate.Resolve(typeName, "", vtestDocsDir(r))
	if err != nil {
		t.Fatalf("Resolve(%q): %v", typeName, err)
	}

	typeCfg := r.Cfg.Types[typeName]
	slug := doctemplate.FilenameSlug(title)

	body, err := doctemplate.Render(tmpl, &doctemplate.Data{
		Number:   number,
		Title:    title,
		Date:     "2026-09-20",
		Author:   "Test Author",
		Status:   config.Status(typeCfg.Statuses[0]),
		Type:     config.DocType(typeName),
		Prefix:   typeCfg.IDPrefix,
		Slug:     slug,
		Filename: number + "-" + slug + ".md",
	})
	if err != nil {
		t.Fatalf("Render(%q): %v", typeName, err)
	}

	return []byte(body)
}

// vtestWrite writes content to a repo-relative path, creating its directory.
func vtestWrite(t *testing.T, r *Repo, rel string, content []byte) {
	t.Helper()

	path := r.Path(rel)

	if err := os.MkdirAll(filepath.Dir(path), config.DirMode); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	if err := os.WriteFile(path, content, config.FileMode); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}

// vtestCreate renders and writes one document of a type, returning its
// repo-relative path.
func vtestCreate(t *testing.T, r *Repo, typeName, number, title string) string {
	t.Helper()

	filename := number + "-" + doctemplate.FilenameSlug(title) + ".md"
	rel := filepath.Join(r.Cfg.TypeDir(typeName), filename)

	vtestWrite(t, r, rel, vtestRender(t, r, typeName, number, title))

	return rel
}

// vtestUpdate does to a repository what `docz update` does: it fills every
// document's ToC and rewrites every type's README index.
//
// Built from the same packages Validate checks against rather than from
// Validate's own helpers, so "clean" means what the CLI would produce.
func vtestUpdate(t *testing.T, r *Repo, types ...string) {
	t.Helper()

	for _, typeName := range types {
		dir := r.TypeDir(typeName)

		docs, err := document.ScanDocuments(dir)
		if err != nil {
			t.Fatalf("ScanDocuments(%s): %v", dir, err)
		}

		for i := range docs {
			result := toc.UpdateToC(string(docs[i].Content), r.Cfg.TOC.MinHeadings)
			if !result.Found {
				continue
			}

			path := filepath.Join(dir, docs[i].Filename)
			if err := os.WriteFile(path, []byte(result.Updated), config.FileMode); err != nil {
				t.Fatalf("WriteFile %s: %v", path, err)
			}
		}

		vtestWriteIndex(t, r, typeName)
	}
}

// vtestWriteIndex rewrites one type's README index from what is on disk now.
func vtestWriteIndex(t *testing.T, r *Repo, typeName string) {
	t.Helper()

	docs, err := document.ScanDocuments(r.TypeDir(typeName))
	if err != nil {
		t.Fatalf("ScanDocuments: %v", err)
	}

	label := r.Cfg.Types[typeName].PluralLabel

	header, err := doctemplate.ResolveIndexHeader(typeName, vtestDocsDir(r), doctemplate.IndexHeaderData{
		TypeName:    typeName,
		PluralLabel: label,
	})
	if err != nil {
		t.Fatalf("ResolveIndexHeader(%q): %v", typeName, err)
	}

	if _, err := index.UpdateReadme(r.ReadmePath(typeName), header, index.GenerateTable(docs, "All "+label)); err != nil {
		t.Fatalf("UpdateReadme(%q): %v", typeName, err)
	}
}

// vtestUnmark strips every docz region marker line, leaving a pre-v2
// document. The legacy ToC pair stays: it is not a region marker, every v1
// document had one, and inference exists for exactly those documents.
func vtestUnmark(content []byte) []byte {
	lines := strings.Split(string(content), "\n")
	kept := make([]string, 0, len(lines))

	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "<!--docz:") {
			continue
		}

		kept = append(kept, line)
	}

	return []byte(strings.Join(kept, "\n"))
}

// vtestEntry returns the report entry for the fixture document.
func vtestEntry(t *testing.T, entries []DocFindings) DocFindings {
	t.Helper()

	for i := range entries {
		if strings.HasSuffix(entries[i].Path, vtestDocName) {
			return entries[i]
		}
	}

	t.Fatalf("no entry for %s among %d entries", vtestDocName, len(entries))

	return DocFindings{}
}

// vtestHasCode reports whether findings hold one with this code.
func vtestHasCode(findings []validate.Finding, code string) bool {
	for _, f := range findings {
		if f.Code == code {
			return true
		}
	}

	return false
}

// vtestCodes lists the codes in findings, for a failure message that says
// what was there instead.
func vtestCodes(findings []validate.Finding) string {
	codes := make([]string, 0, len(findings))
	for _, f := range findings {
		codes = append(codes, f.String())
	}

	return strings.Join(codes, "\n  ")
}

// vtestTypeWithTemplate declares a custom type and gives it an on-disk
// template, which is the only way a custom type has one.
func vtestTypeWithTemplate(t *testing.T, r *Repo, typeName, prefix string, tmpl []byte) {
	t.Helper()

	r.Cfg.Types[typeName] = config.TypeConfig{
		Enabled:  true,
		Dir:      typeName,
		IDPrefix: prefix,
		IDWidth:  4,
		Statuses: []string{"Draft", "Done"},
	}

	if tmpl != nil {
		vtestWrite(t, r, filepath.Join(r.Cfg.DocsDir, config.TemplatesDir, typeName+".md"), tmpl)
	}
}

// TestValidate_CleanRepoReportsNoErrors is the golden this tier is built
// against: a repository whose documents came from docz's own templates and
// whose indexes are up to date has nothing wrong with it.
//
// Errors only, deliberately. A warning is a judgement about prose — a
// reference with no link in a template's placeholder section — and a fixture
// built from templates cannot avoid all of them without inventing content.
func TestValidate_CleanRepoReportsNoErrors(t *testing.T) {
	t.Parallel()

	r := vtestRepo(t)

	types := r.Cfg.EnabledTypes()
	for _, typeName := range types {
		vtestCreate(t, r, typeName, "0001", vtestTitle)
	}

	vtestUpdate(t, r, types...)

	report, err := r.Validate(t.Context(), nil, ValidateOptions{})
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	for i := range report.Docs {
		for _, f := range report.Docs[i].Findings {
			if f.Severity == validate.Error {
				t.Errorf("%s reports %s", report.Docs[i].Path, f)
			}
		}
	}

	for i := range report.Templates {
		for _, f := range report.Templates[i].Findings {
			if f.Severity == validate.Error {
				t.Errorf("the %s template reports %s", report.Templates[i].Type, f)
			}
		}
	}

	if report.Errors != 0 {
		t.Errorf("Errors = %d, want 0", report.Errors)
	}

	if len(report.Index) != 0 {
		t.Errorf("Index = %v, want none", report.Index)
	}
}

// TestValidate_TemplatesOnePerEnabledType pins the shape a caller iterates:
// one entry per type, present even for a type with no documents, because a
// missing entry cannot be told apart from a type that was never reached.
func TestValidate_TemplatesOnePerEnabledType(t *testing.T) {
	t.Parallel()

	r := vtestRepo(t)

	report, err := r.Validate(t.Context(), nil, ValidateOptions{})
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	want := r.Cfg.EnabledTypes()
	if len(report.Templates) != len(want) {
		t.Fatalf("Templates has %d entries, want %d", len(report.Templates), len(want))
	}

	for i, typeName := range want {
		if report.Templates[i].Type != typeName {
			t.Errorf("Templates[%d].Type = %q, want %q", i, report.Templates[i].Type, typeName)
		}

		if report.Templates[i].Schema != typeName {
			t.Errorf("Templates[%d].Schema = %q, want %q", i, report.Templates[i].Schema, typeName)
		}
	}
}

// TestValidate_BrokenStatus is the generic tier arriving through this one: a
// status no configuration allows is the frontmatter finding, reported against
// the document that carries it.
func TestValidate_BrokenStatus(t *testing.T) {
	t.Parallel()

	r := vtestRepo(t)

	doc := vtestRender(t, r, "adr", "0001", vtestTitle)
	broken := strings.Replace(string(doc), "status: Proposed", "status: Bogus", 1)

	if broken == string(doc) {
		t.Fatal("fixture did not change; the adr template no longer writes status: Proposed")
	}

	rel := vtestCreate(t, r, "adr", "0001", vtestTitle)
	vtestWrite(t, r, rel, []byte(broken))

	report, err := r.Validate(t.Context(), []string{"adr"}, ValidateOptions{})
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	entry := vtestEntry(t, report.Docs)
	if !vtestHasCode(entry.Findings, "frontmatter.status") {
		t.Errorf("no frontmatter.status finding; got:\n  %s", vtestCodes(entry.Findings))
	}

	if report.Errors == 0 {
		t.Error("Errors = 0, want the status finding counted")
	}
}

// TestValidate_UnmarkedDocumentIsInferred is the amended rule of DESIGN-0015
// §4: a document with no markers is not an error. A v1 document under a v2
// template has its regions inferred from its headings, reported once as a
// warning, and nothing about it is missing.
func TestValidate_UnmarkedDocumentIsInferred(t *testing.T) {
	t.Parallel()

	r := vtestRepo(t)

	rel := vtestCreate(t, r, "adr", "0001", vtestTitle)
	vtestWrite(t, r, rel, vtestUnmark(vtestRender(t, r, "adr", "0001", vtestTitle)))
	vtestUpdate(t, r, "adr")

	report, err := r.Validate(t.Context(), []string{"adr"}, ValidateOptions{})
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	entry := vtestEntry(t, report.Docs)

	if !vtestHasCode(entry.Findings, "region.inferred") {
		t.Errorf("no region.inferred warning; got:\n  %s", vtestCodes(entry.Findings))
	}

	if vtestHasCode(entry.Findings, "region.missing") {
		t.Errorf("inference left a region missing; got:\n  %s", vtestCodes(entry.Findings))
	}

	for _, f := range entry.Findings {
		if f.Severity == validate.Error {
			t.Errorf("an unmarked document reports the error %s", f)
		}
	}
}

// TestValidate_UnresolvedSchema covers the finding this tier owns: a name
// only a filesystem can fail to resolve, reported with the document's Schema
// left empty so a consumer knows the regions were not checked.
func TestValidate_UnresolvedSchema(t *testing.T) {
	t.Parallel()

	r := vtestRepo(t)

	doc := vtestRender(t, r, "adr", "0001", vtestTitle)
	named := strings.Replace(string(doc), "status: Proposed", "status: Proposed\nschema: nowhere", 1)

	rel := vtestCreate(t, r, "adr", "0001", vtestTitle)
	vtestWrite(t, r, rel, []byte(named))

	report, err := r.Validate(t.Context(), []string{"adr"}, ValidateOptions{})
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	entry := vtestEntry(t, report.Docs)

	if !vtestHasCode(entry.Findings, CodeSchemaUnresolved) {
		t.Errorf("no %s finding; got:\n  %s", CodeSchemaUnresolved, vtestCodes(entry.Findings))
	}

	if entry.Schema != "" {
		t.Errorf("Schema = %q, want empty for an unresolved name", entry.Schema)
	}

	// Well-formedness only, so the regions the adr schema requires are not
	// reported against a document that never got that schema.
	if vtestHasCode(entry.Findings, "region.missing") {
		t.Errorf("regions were checked against a schema that did not resolve:\n  %s", vtestCodes(entry.Findings))
	}
}

// TestValidate_SchemaCacheResolvesEachNameOnce proves the cache short-circuits
// the filesystem rather than merely agreeing with it.
//
// The observable consequence is all a test can reach through Validate itself —
// two documents naming one schema cannot reveal how many times it was read —
// so the first subtest seeds the cache with a schema no file could have
// produced and shows it comes back anyway.
func TestValidate_SchemaCacheResolvesEachNameOnce(t *testing.T) {
	t.Parallel()

	t.Run("a cached name is not resolved again", func(t *testing.T) {
		t.Parallel()

		r := vtestRepo(t)
		run := &validateRun{repo: r, schemas: make(map[string]cachedSchema)}
		sentinel := validate.Schema{Regions: []validate.SchemaRegion{{Kind: "cached-only"}}}

		// Nothing on disk or embedded is named this, so a second lookup
		// would fail. Getting the sentinel back is the cache answering.
		run.schemas["ghost"] = cachedSchema{schema: sentinel}

		got, err := run.schemaFor("ghost", &typeContext{name: "adr"})
		if err != nil {
			t.Fatalf("schemaFor: %v", err)
		}

		if len(got.Regions) != 1 || got.Regions[0].Kind != "cached-only" {
			t.Errorf("schemaFor returned %+v, want the cached sentinel", got)
		}
	})

	t.Run("one entry per distinct name", func(t *testing.T) {
		t.Parallel()

		r := vtestRepo(t)
		run := &validateRun{repo: r, schemas: make(map[string]cachedSchema)}
		tc := run.typeContext("adr")

		for range 3 {
			if _, err := run.schemaFor("adr", tc); err != nil {
				t.Fatalf("schemaFor: %v", err)
			}
		}

		if len(run.schemas) != 1 {
			t.Errorf("cache holds %d entries after three lookups of one name, want 1", len(run.schemas))
		}
	})

	t.Run("two documents naming one schema agree", func(t *testing.T) {
		t.Parallel()

		r := vtestRepo(t)

		skeleton, err := doctemplate.EmbeddedSchema("adr")
		if err != nil {
			t.Fatalf("EmbeddedSchema: %v", err)
		}

		vtestWrite(t, r, filepath.Join(r.Cfg.DocsDir, config.TemplatesDir, "schema", "house.md"), skeleton)

		for _, number := range []string{"0001", "0002"} {
			rel := vtestCreate(t, r, "adr", number, vtestTitle)
			doc := vtestRender(t, r, "adr", number, vtestTitle)
			vtestWrite(t, r, rel,
				[]byte(strings.Replace(string(doc), "status: Proposed", "status: Proposed\nschema: house", 1)))
		}

		vtestUpdate(t, r, "adr")

		report, err := r.Validate(t.Context(), []string{"adr"}, ValidateOptions{})
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}

		if len(report.Docs) != 2 {
			t.Fatalf("Docs has %d entries, want 2", len(report.Docs))
		}

		for i := range report.Docs {
			if report.Docs[i].Schema != "house" {
				t.Errorf("Docs[%d].Schema = %q, want %q", i, report.Docs[i].Schema, "house")
			}

			if vtestHasCode(report.Docs[i].Findings, CodeSchemaUnresolved) {
				t.Errorf("Docs[%d] reports the repo's own schema unresolved:\n  %s",
					i, vtestCodes(report.Docs[i].Findings))
			}
		}
	})
}

// TestValidate_ToCDrift covers the first of the two drift checks: stale,
// fresh, and the case the generic tier leaves to this one — a pair that is
// there and has never been filled.
func TestValidate_ToCDrift(t *testing.T) {
	t.Parallel()

	t.Run("stale", func(t *testing.T) {
		t.Parallel()

		r := vtestRepo(t)
		rel := vtestCreate(t, r, "adr", "0001", vtestTitle)
		vtestUpdate(t, r, "adr")

		current, err := os.ReadFile(r.Path(rel))
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}

		vtestWrite(t, r, rel, append(current, []byte("\n## Late Addition\n\nText.\n")...))

		report, err := r.Validate(t.Context(), []string{"adr"}, ValidateOptions{})
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}

		entry := vtestEntry(t, report.Docs)
		if !vtestHasCode(entry.Findings, CodeToCStale) {
			t.Errorf("no %s finding; got:\n  %s", CodeToCStale, vtestCodes(entry.Findings))
		}

		// Once, not once per tier.
		count := 0

		for _, f := range entry.Findings {
			if f.Code == CodeToCStale {
				count++
			}
		}

		if count != 1 {
			t.Errorf("%s reported %d times, want 1", CodeToCStale, count)
		}
	})

	t.Run("fresh", func(t *testing.T) {
		t.Parallel()

		r := vtestRepo(t)
		vtestCreate(t, r, "adr", "0001", vtestTitle)
		vtestUpdate(t, r, "adr")

		report, err := r.Validate(t.Context(), []string{"adr"}, ValidateOptions{})
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}

		entry := vtestEntry(t, report.Docs)
		if vtestHasCode(entry.Findings, CodeToCStale) {
			t.Errorf("a freshly updated document reports %s:\n  %s", CodeToCStale, vtestCodes(entry.Findings))
		}
	})

	t.Run("never generated", func(t *testing.T) {
		t.Parallel()

		r := vtestRepo(t)
		vtestCreate(t, r, "adr", "0001", vtestTitle)

		report, err := r.Validate(t.Context(), []string{"adr"}, ValidateOptions{})
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}

		entry := vtestEntry(t, report.Docs)
		if !vtestHasCode(entry.Findings, CodeToCStale) {
			t.Errorf("an unfilled ToC pair reports nothing; got:\n  %s", vtestCodes(entry.Findings))
		}
	})

	t.Run("nothing when the toc is off", func(t *testing.T) {
		t.Parallel()

		r := vtestRepo(t)
		r.Cfg.TOC.Enabled = false
		vtestCreate(t, r, "adr", "0001", vtestTitle)

		report, err := r.Validate(t.Context(), []string{"adr"}, ValidateOptions{})
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}

		entry := vtestEntry(t, report.Docs)
		if vtestHasCode(entry.Findings, CodeToCStale) {
			t.Errorf("a repo with the ToC off is told to run docz update:\n  %s", vtestCodes(entry.Findings))
		}
	})
}

// TestValidate_IndexDrift covers the second drift check, including the README
// docz did not write and will not rewrite.
func TestValidate_IndexDrift(t *testing.T) {
	t.Parallel()

	t.Run("fresh", func(t *testing.T) {
		t.Parallel()

		r := vtestRepo(t)
		vtestCreate(t, r, "adr", "0001", vtestTitle)
		vtestUpdate(t, r, "adr")

		report, err := r.Validate(t.Context(), []string{"adr"}, ValidateOptions{})
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}

		if len(report.Index) != 0 {
			t.Errorf("Index = %v, want none", report.Index)
		}
	})

	t.Run("a document the table does not list", func(t *testing.T) {
		t.Parallel()

		r := vtestRepo(t)
		vtestCreate(t, r, "adr", "0001", vtestTitle)
		vtestUpdate(t, r, "adr")
		vtestCreate(t, r, "adr", "0002", "Another placeholder title")

		report, err := r.Validate(t.Context(), []string{"adr"}, ValidateOptions{})
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}

		if len(report.Index) != 1 {
			t.Fatalf("Index = %v, want one entry", report.Index)
		}

		if report.Index[0].Type != "adr" {
			t.Errorf("Index[0].Type = %q, want %q", report.Index[0].Type, "adr")
		}

		want := filepath.Join("docs", "adr", config.IndexFileName)
		if report.Index[0].Path != want {
			t.Errorf("Index[0].Path = %q, want %q", report.Index[0].Path, want)
		}
	})

	t.Run("no readme at all", func(t *testing.T) {
		t.Parallel()

		r := vtestRepo(t)
		vtestCreate(t, r, "adr", "0001", vtestTitle)

		report, err := r.Validate(t.Context(), []string{"adr"}, ValidateOptions{})
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}

		if len(report.Index) != 1 {
			t.Errorf("Index = %v, want one entry for the missing README", report.Index)
		}
	})

	t.Run("a readme with no markers is left alone", func(t *testing.T) {
		t.Parallel()

		r := vtestRepo(t)
		vtestCreate(t, r, "adr", "0001", vtestTitle)
		vtestWrite(t, r, filepath.Join("docs", "adr", config.IndexFileName), []byte("# Ours\n\nHand written.\n"))

		report, err := r.Validate(t.Context(), []string{"adr"}, ValidateOptions{})
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}

		if len(report.Index) != 0 {
			t.Errorf("Index = %v, want none for a README docz does not own", report.Index)
		}
	})
}

// TestValidate_CancelledContextReturnsPartialReport pins the cancellation rule
// (DESIGN-0014 §2.8): the report completed so far, its totals included, comes
// back with ctx.Err().
//
// Cancelled from a hook during the first type's scan, so the report really is
// partial rather than empty — the whole point of returning it.
func TestValidate_CancelledContextReturnsPartialReport(t *testing.T) {
	t.Parallel()

	r := vtestRepo(t)

	for _, typeName := range []string{"adr", "rfc"} {
		vtestCreate(t, r, typeName, "0001", vtestTitle)
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	ctx = WithHooks(ctx, &Hooks{ScanDone: func(string, int) { cancel() }})

	report, err := r.Validate(ctx, []string{"adr", "rfc"}, ValidateOptions{})
	if !errorsIsCancelled(err) {
		t.Fatalf("Validate error = %v, want context.Canceled", err)
	}

	if len(report.Templates) != 1 {
		t.Errorf("Templates has %d entries, want the one type that finished", len(report.Templates))
	}

	if len(report.Docs) != 1 {
		t.Errorf("Docs has %d entries, want the one document that was reached", len(report.Docs))
	}

	if report.Errors+report.Warnings == 0 && len(report.Docs) > 0 {
		// The counts are recomputed on the cancellation path, so a partial
		// report's totals describe the partial report.
		t.Log("partial report carries no findings, which is possible but worth noticing")
	}
}

// errorsIsCancelled keeps the cancellation assertion readable.
func errorsIsCancelled(err error) bool {
	return err != nil && strings.Contains(err.Error(), context.Canceled.Error())
}

// TestValidate_TemplateThatResolvesNowhere covers issue #92 through this tier:
// a custom type with no template is a finding against that type, and the run
// carries on to the next one.
func TestValidate_TemplateThatResolvesNowhere(t *testing.T) {
	t.Parallel()

	r := vtestRepo(t)
	vtestTypeWithTemplate(t, r, "frameworks", "FW", nil)

	report, err := r.Validate(t.Context(), []string{"frameworks", "adr"}, ValidateOptions{})
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	if len(report.Templates) != 2 {
		t.Fatalf("Templates has %d entries, want 2 — the run stopped at the broken type", len(report.Templates))
	}

	if !vtestHasCode(report.Templates[0].Findings, CodeTemplateUnresolved) {
		t.Errorf("no %s finding; got:\n  %s", CodeTemplateUnresolved, vtestCodes(report.Templates[0].Findings))
	}

	if vtestHasCode(report.Templates[1].Findings, CodeTemplateUnresolved) {
		t.Errorf("the adr template did not resolve either:\n  %s", vtestCodes(report.Templates[1].Findings))
	}
}

// TestValidate_CustomTypeSchemaFromItsTemplate is the third resolution tier of
// DESIGN-0015 §3: a custom type that has not scaffolded a schema is held to
// the markers its own template carries, not to nothing.
func TestValidate_CustomTypeSchemaFromItsTemplate(t *testing.T) {
	t.Parallel()

	r := vtestRepo(t)

	generic, err := doctemplate.GenericTemplate()
	if err != nil {
		t.Fatalf("GenericTemplate: %v", err)
	}

	vtestTypeWithTemplate(t, r, "frameworks", "FW", []byte(generic))
	vtestCreate(t, r, "frameworks", "0001", vtestTitle)
	vtestUpdate(t, r, "frameworks")

	report, err := r.Validate(t.Context(), []string{"frameworks"}, ValidateOptions{})
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	entry := vtestEntry(t, report.Docs)

	if entry.Schema != "frameworks" {
		t.Errorf("Schema = %q, want the type's own name", entry.Schema)
	}

	if vtestHasCode(entry.Findings, CodeSchemaUnresolved) {
		t.Errorf("a custom type with a marked template has no schema:\n  %s", vtestCodes(entry.Findings))
	}
}

// TestValidate_V1TemplateOverrideHazard pins a known consequence of template
// resolution rather than desired behaviour (see inferenceSpec).
//
// A repository whose docs/templates/<type>.md predates v2 carries no region
// markers, so the inference grammar derived from it is empty and an unmarked
// document under it reports every region its schema requires as missing. The
// test exists so the day that changes is a test failure and not a surprise,
// and it also pins the signal that explains it: the same report says the
// template itself is missing those regions.
func TestValidate_V1TemplateOverrideHazard(t *testing.T) {
	t.Parallel()

	r := vtestRepo(t)

	embedded, err := doctemplate.EmbeddedDocumentTemplate(config.DocType("adr"))
	if err != nil {
		t.Fatalf("EmbeddedDocumentTemplate: %v", err)
	}

	// The v1-era override: the same template, before it grew markers.
	vtestWrite(t, r, filepath.Join(r.Cfg.DocsDir, config.TemplatesDir, "adr.md"), vtestUnmark([]byte(embedded)))

	rel := vtestCreate(t, r, "adr", "0001", vtestTitle)
	vtestWrite(t, r, rel, vtestUnmark(vtestRender(t, r, "adr", "0001", vtestTitle)))
	vtestUpdate(t, r, "adr")

	report, err := r.Validate(t.Context(), []string{"adr"}, ValidateOptions{})
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	entry := vtestEntry(t, report.Docs)
	if !vtestHasCode(entry.Findings, "region.missing") {
		t.Errorf("the hazard is gone: an unmarked document under an unmarked override now validates:\n  %s",
			vtestCodes(entry.Findings))
	}

	if !vtestHasCode(report.Templates[0].Findings, "region.missing") {
		t.Errorf("the template check does not report the cause:\n  %s", vtestCodes(report.Templates[0].Findings))
	}

	if report.Templates[0].Path != filepath.Join("docs", config.TemplatesDir, "adr.md") {
		t.Errorf("Templates[0].Path = %q, want the override a person would edit", report.Templates[0].Path)
	}
}

// TestValidate_UnknownTypeIsAnError keeps the resolution contract: a token
// nobody can resolve fails the call rather than producing an empty report,
// because the caller asked for it by name.
func TestValidate_UnknownTypeIsAnError(t *testing.T) {
	t.Parallel()

	r := vtestRepo(t)

	if _, err := r.Validate(t.Context(), []string{"nope"}, ValidateOptions{}); err == nil {
		t.Fatal("Validate accepted an unknown type")
	}
}
