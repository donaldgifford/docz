package repo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/doctemplate"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/document"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/index"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/toc"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/validate"
)

// The codes this tier emits itself. Every other finding in a report comes
// from validate.Document, which owns its own families (DESIGN-0015 §4);
// these are the ones only a tier holding a filesystem can see.
const (
	// CodeSchemaUnresolved reports a document whose schema name resolves to
	// no skeleton. It belongs here rather than in validate because
	// validate.Document only ever sees a Schema that is already resolved —
	// it never touches a filesystem, so it cannot know the name pointed at
	// nothing (DESIGN-0015 §3).
	CodeSchemaUnresolved = "schema.unresolved"

	// CodeTemplateUnresolved reports a type whose template resolves nowhere:
	// no path in its config, no file under docs/templates, and no embedded
	// default (issue #92). It is a finding rather than an error return
	// because one broken custom type must not stop a repository-wide run.
	CodeTemplateUnresolved = "template.unresolved"

	// CodeTemplateRender reports a template that is not valid text/template,
	// or whose actions ask for data docz does not supply. The template check
	// has to render before it can read, so this is where that failure lands.
	CodeTemplateRender = "template.render"

	// CodeToCStale is validate's own code, re-emitted by this tier for the
	// drift only a regeneration can see. Same code deliberately: an author
	// fixes both the same way, and a consumer filtering on codes should not
	// have to learn a second spelling for one problem.
	CodeToCStale = "toc.stale"
)

// The metadata the template check renders a template with. Fixed values
// rather than today's date and the caller's name, so two runs over an
// unchanged repository produce the same report.
const (
	// placeholderTitle is a title that is neither empty nor a template
	// placeholder, so frontmatter.title passes for a sound template.
	placeholderTitle = "Placeholder title"

	// placeholderAuthor fills the author line; no check reads it.
	placeholderAuthor = "docz"

	// placeholderDate is ISO, which is all frontmatter.created asks.
	placeholderDate = "2000-01-01"

	// placeholderStatus is used only by a type that configures no statuses.
	// A type that configures some gets its first, since that is what `docz
	// create` writes.
	placeholderStatus = "Draft"
)

// ValidateOptions are the knobs on a validation run.
type ValidateOptions struct {
	// Strict is the caller's failure threshold, not this package's. A run
	// records the same findings either way; Strict says only that the caller
	// intends to fail on warnings as well as errors, which is why
	// ValidateReport counts the two separately and nothing here branches on
	// this field. cmd/validate.go turns it into an exit code (DESIGN-0015
	// §4) and docz-api applies its own policy to the same report.
	Strict bool
}

// DocFindings is what one document — or one type's rendered template — had to
// say about itself.
type DocFindings struct {
	// Type is the canonical type name. Path is repo-relative: the document's
	// path, or for a Templates entry the template file a person would edit,
	// empty when the template is docz's own embedded one and there is no
	// file to name.
	Type, Path string

	// Schema is the name the document's schema resolved under: its
	// frontmatter schema field, or its type name when that field is empty.
	// Empty means the name resolved nowhere, CodeSchemaUnresolved is in
	// Findings, and validation ran against an empty schema — which is
	// well-formedness only, and deliberately still useful.
	Schema string

	// Findings are the generic tier's, in the order it returned them, with
	// this tier's own appended after. A consumer that wants them strictly by
	// line sorts them, because two tiers cannot merge-sort without agreeing
	// on a tie-break neither of them owns.
	Findings []validate.Finding
}

// IndexDrift reports a type whose README index table is not what a fresh
// render would produce.
//
// Not a Finding, because it is not about a document: the table is generated,
// so the fix is `docz update` rather than an edit, and a consumer that means
// to run update wants the list of types to run it for rather than a line
// number in a file nobody wrote by hand.
type IndexDrift struct {
	// Type is the canonical type name; Path is the repo-relative README.
	Type, Path string
}

// ValidateReport is everything one Validate call found.
type ValidateReport struct {
	// Docs is one entry per document, in type order and then id order — the
	// order Scan returns them in, so a report reads like the tree.
	Docs []DocFindings

	// Templates is one entry per type validated, in the same type order.
	// Always one per type, even for a type whose template resolves nowhere:
	// a missing entry would be indistinguishable from a type that was never
	// reached.
	Templates []DocFindings

	// Index is one entry per type whose README drifted. Empty is the healthy
	// case.
	Index []IndexDrift

	// Errors and Warnings are the totals over every finding in Docs and
	// Templates, by severity. ValidateOptions.Strict does not change them:
	// which total a caller fails on is the caller's policy, and a library
	// that folded the two would make the other policy unimplementable.
	Errors, Warnings int
}

// Validate runs the repository tier of DESIGN-0015 §4 over one or more types.
//
// Per type, in this order: the type's template rendered and checked against
// the schema its type name resolves to, then every document checked against
// the schema it names, then the type's README index compared with a fresh
// render. A nil or empty types slice means every enabled type, in
// Cfg.EnabledTypes order.
//
// The options are accepted and not read. Strict is the caller's failure
// threshold rather than this package's behaviour, and the parameter is here
// because the design publishes it — adding it later would break every caller.
//
// This is two of the three tiers. The per-type tier lives in pkg/impl and its
// siblings, which repo may not import (DESIGN-0014 §6 rule R2), so the caller
// composes it: switch each DocFindings on Schema with Type as the fallback,
// call that package's Validate on the document's bytes, and append. What
// arrives here is therefore complete about markers, regions, frontmatter, and
// drift, and silent about anything that needs the typed model.
//
// Cancellation is checked between types. A cancelled run returns the report
// completed so far — totals included — together with ctx.Err(), so a caller
// can report partial progress instead of discarding the types that finished.
// A filesystem failure returns the partial report the same way.
//
// One gap worth naming: a file with no frontmatter is not a document to
// document.ScanDocuments, which skips it silently, so it never reaches the
// validator and validate's frontmatter.missing cannot fire from here. That
// finding is reachable through a consumer that hands bytes straight to
// validate.Document — docz-api's no-checkout path — and through the template
// check, which validates bytes it rendered rather than bytes it scanned.
func (r *Repo) Validate(ctx context.Context, types []string, _ ValidateOptions) (ValidateReport, error) {
	names, err := r.typesOrEnabled(types)
	if err != nil {
		return ValidateReport{}, err
	}

	run := &validateRun{repo: r, schemas: make(map[string]cachedSchema)}

	var report ValidateReport

	for _, typeName := range names {
		if err := ctx.Err(); err != nil {
			tally(&report)

			return report, err
		}

		if err := run.checkType(ctx, typeName, &report); err != nil {
			tally(&report)

			return report, err
		}
	}

	tally(&report)

	return report, nil
}

// validateRun is one Validate call's state: the repository it walks, and the
// schemas it has resolved so far.
//
// A per-call struct rather than fields on Repo, so two concurrent runs over
// the same Repo share nothing and a cached schema cannot outlive the
// filesystem it was read from.
type validateRun struct {
	repo    *Repo
	schemas map[string]cachedSchema
}

// cachedSchema is one schema name's resolution outcome.
//
// A non-nil err is cached as readily as a schema. An unresolvable name is a
// fact about the repository rather than a transient failure, and a repo where
// every document names the same missing skeleton should pay for one lookup,
// not one per document.
type cachedSchema struct {
	schema validate.Schema
	err    error
}

// typeContext is what every check for one type needs, gathered once: the
// type's configuration, its resolved template, and the two things derived
// from that template.
type typeContext struct {
	// name is the canonical type name and cfg its configuration.
	name string
	cfg  config.TypeConfig

	// tmpl is the resolved template, and tmplErr why it did not resolve.
	// Exactly one of the two is meaningful.
	tmpl    string
	tmplErr error

	// tmplPath is the repo-relative template file a person would edit, or ""
	// when the template is the embedded one.
	tmplPath string

	// spec is the inference grammar derived from tmpl (see inferenceSpec).
	spec kinds.HeadingSpec

	// fromTemplate is the schema the template's own markers describe: the
	// third resolution tier of DESIGN-0015 §3, for a custom type that has
	// not scaffolded a schema file. Derived once here so schemaFor's fallback
	// costs nothing per document.
	fromTemplate validate.Schema
}

// typeContext resolves one type's template and everything derived from it.
//
// A template that does not resolve is not a failure of this function: the
// error is carried in the context and reported by the template check, and the
// type's documents are still validated — with no inference grammar, so a
// marked document is read from its markers and an unmarked one is read as
// having no regions at all.
func (v *validateRun) typeContext(typeName string) *typeContext {
	tc := &typeContext{name: typeName, cfg: v.repo.Cfg.Types[typeName]}

	tc.tmpl, tc.tmplErr = doctemplate.Resolve(typeName, v.repo.Path(tc.cfg.Template), v.docsDir())
	if tc.tmplErr != nil {
		return tc
	}

	tc.tmplPath = v.templatePath(tc)
	tc.spec = tc.inferenceSpec()
	tc.fromTemplate = validate.SchemaFromMarkers([]byte(tc.tmpl))

	return tc
}

// checkType runs the three per-type checks and appends their results.
//
// A filesystem failure stops the run and is returned: a directory that cannot
// be read or an index header that cannot be resolved is not a finding about a
// document, and recording it as one would bury it among findings a person is
// meant to skim. A type whose template does not resolve is the exception —
// that is a finding, because saying so is what the template check is for.
func (v *validateRun) checkType(ctx context.Context, typeName string, report *ValidateReport) error {
	tc := v.typeContext(typeName)

	report.Templates = append(report.Templates, v.checkTemplate(tc))

	docs, err := v.repo.Scan(ctx, typeName)
	if err != nil {
		return err
	}

	dir := v.repo.TypeDir(typeName)

	for i := range docs {
		report.Docs = append(report.Docs, v.checkDoc(tc, dir, &docs[i]))
	}

	return v.checkIndex(tc, docs, report)
}

// checkTemplate validates one type's rendered template against the schema its
// type name resolves to.
//
// Templates are golden tests, not schemas (DESIGN-0015 §3): the schema is its
// own artifact, so this check is the one that says a repo's template and its
// schema have drifted apart — a template override that dropped a region would
// otherwise loosen the contract silently, which is the opposite of what an
// override should be able to do.
//
// The template is rendered with placeholder metadata first, so its
// frontmatter passes by construction and what remains is structural.
// Inference is deliberately off: a template that lost its markers is exactly
// the drift this exists to report, and inferring the regions back would hide
// it.
//
// ToC drift is not checked. A template's pair is written empty and filled at
// create time, so there is nothing for drift to mean; the generic tier reads
// an empty pair as ungenerated rather than stale, which makes that a property
// of the check rather than a hope about the input.
func (v *validateRun) checkTemplate(tc *typeContext) DocFindings {
	entry := DocFindings{Type: tc.name, Path: tc.tmplPath}

	if tc.tmplErr != nil {
		entry.Findings = []validate.Finding{{
			Code:     CodeTemplateUnresolved,
			Severity: validate.Error,
			Detail:   tc.tmplErr.Error(),
		}}

		return entry
	}

	entry.Schema = tc.name

	schema, err := v.schemaFor(tc.name, tc)
	if err != nil {
		entry.Schema = ""
		entry.Findings = append(entry.Findings, schemaUnresolvedFinding(tc.name, err))
	}

	rendered, filename, err := tc.render()
	if err != nil {
		entry.Findings = append(entry.Findings, validate.Finding{
			Code:     CodeTemplateRender,
			Severity: validate.Error,
			Detail:   err.Error(),
		})

		return entry
	}

	entry.Findings = append(entry.Findings, validate.Document(rendered, validate.Options{
		Schema:      schema,
		Type:        tc.cfg,
		Filename:    filename,
		MinHeadings: v.repo.Cfg.TOC.MinHeadings,
	})...)

	return entry
}

// checkDoc validates one document against the schema it names.
func (v *validateRun) checkDoc(tc *typeContext, dir string, doc *document.DocEntry) DocFindings {
	path := v.repo.RelPath(filepath.Join(dir, doc.Filename))

	name := doc.Schema
	if name == "" {
		// An absent or empty schema field means the document's own type,
		// which is the common case and what every built-in template ships,
		// so a document validates against the baked-in schema without
		// carrying a line for it (DESIGN-0015 §3).
		name = tc.name
	}

	entry := DocFindings{Type: tc.name, Path: path, Schema: name}

	schema, err := v.schemaFor(name, tc)
	if err != nil {
		// Validation continues against the zero Schema. A name nobody can
		// resolve costs the document its region checks, not its whole run:
		// its markers, frontmatter, and content rules are still worth
		// reporting, and refusing to look would punish the document for a
		// missing file it may not own.
		entry.Schema = ""
		entry.Findings = append(entry.Findings, schemaUnresolvedFinding(name, err))
	}

	entry.Findings = append(entry.Findings, validate.Document(doc.Content, validate.Options{
		Schema:      schema,
		Type:        tc.cfg,
		Filename:    path,
		Headings:    tc.spec,
		MinHeadings: v.repo.Cfg.TOC.MinHeadings,
	})...)

	drift := v.tocDrift(doc.Content, entry.Findings)
	entry.Findings = append(entry.Findings, drift...)

	return entry
}

// tocDrift reports a document whose table of contents the splice would
// rewrite — one of the two drift checks that need more than a document's own
// bytes (DESIGN-0015 §4).
//
// Regenerated through the toc package rather than compared by eye, so a
// finding and `docz update` cannot disagree about whether a document needs
// one. That is the same rule the generic tier follows, which is why this takes
// the findings so far: it says nothing when validate.Document already reported
// staleness, and that tier reports it for every document whose ToC region has
// content. What is left for this check is the pair that is present and still
// empty — a document created but never updated, where `docz update` has work
// to do and the generic tier deliberately stays quiet because reporting drift
// there would fail every document the moment it was created.
//
// It says nothing at all when the repo has turned the ToC off, because then
// `docz update` will not fill the pair and "run docz update" would be advice
// that does not work.
func (v *validateRun) tocDrift(content []byte, reported []validate.Finding) []validate.Finding {
	if !v.repo.Cfg.TOC.Enabled {
		return nil
	}

	for _, f := range reported {
		if f.Code == CodeToCStale {
			return nil
		}
	}

	src := string(content)

	result := toc.UpdateToC(src, v.repo.Cfg.TOC.MinHeadings)
	if !result.Found || result.Updated == src {
		return nil
	}

	return []validate.Finding{{
		Code:     CodeToCStale,
		Severity: validate.Warning,
		Line:     tocStartLine(content),
		Kind:     docparse.TocKind,
		Detail:   "table of contents is out of date; run docz update to regenerate it",
	}}
}

// tocStartLine is the 1-based line of a document's ToC start marker, or 0 when
// it has none.
//
// Through the region walker rather than a string search, so a leniently
// spelled marker is found — the same marker the splice would rewrite (INV-0009
// Finding 4).
func tocStartLine(content []byte) int {
	for _, region := range docparse.Regions(content) {
		if region.Kind == docparse.TocKind {
			return region.Start
		}
	}

	return 0
}

// checkIndex compares a type's README index with a fresh render and records
// an IndexDrift when the two differ.
//
// Through index.DryRunReadme, so what it compares against is exactly what
// `docz update` would write, down to the header resolution. The heading and
// label are built the same way update builds them for the same reason: a
// heading that differed by a word would make every README in the repository
// look drifted.
func (v *validateRun) checkIndex(tc *typeContext, docs []document.DocEntry, report *ValidateReport) error {
	heading, label := tc.indexHeading()

	header, err := doctemplate.ResolveIndexHeader(tc.name, v.docsDir(), doctemplate.IndexHeaderData{
		TypeName:    tc.name,
		PluralLabel: label,
	})
	if err != nil {
		return fmt.Errorf("resolving index header for %s: %w", tc.name, err)
	}

	readme := v.repo.ReadmePath(tc.name)

	outcome, err := index.DryRunReadme(readme, header, index.GenerateTable(docs, heading))
	if err != nil {
		return fmt.Errorf("checking index %s: %w", v.repo.RelPath(readme), err)
	}

	drifted, err := indexDrifted(readme, &outcome)
	if err != nil {
		return err
	}

	if drifted {
		report.Index = append(report.Index, IndexDrift{Type: tc.name, Path: v.repo.RelPath(readme)})
	}

	return nil
}

// indexDrifted reports whether the body DryRunReadme built differs from what
// is on disk.
//
// DryRunReadme says what it would write, not whether writing it would change
// anything, so the comparison belongs here. A README that is not there at all
// is drift — the table it should hold is missing entirely. A README with no
// marker pair is not, because it is somebody's own file and docz does not
// rewrite it, which is the rule `docz update` follows when it reports "no
// markers" and leaves the file alone.
func indexDrifted(readme string, outcome *index.UpdateOutcome) (bool, error) {
	switch outcome.Action {
	case index.ActionDryRunCreated:
		return true, nil
	case index.ActionDryRunUpdated:
		current, err := os.ReadFile(readme)
		if err != nil {
			return false, fmt.Errorf("reading %s: %w", readme, err)
		}

		return string(current) != outcome.Body, nil
	case index.ActionNoMarkers, index.ActionCreated, index.ActionUpdated:
		// NoMarkers is somebody's own README. The other two are not values
		// DryRunReadme returns; naming them means a new action cannot be
		// added to index without this switch failing to compile.
	}

	return false, nil
}

// schemaFor resolves a schema name to the regions it requires, resolving each
// distinct name at most once for the run.
//
// The cache is the point. A repository's documents overwhelmingly name the
// same handful of schemas — most of them by saying nothing and getting their
// type's — so resolving per document would stat and read one file hundreds of
// times for an answer that cannot change mid-run.
//
// The third tier of DESIGN-0015 §3 is applied after the cache rather than
// through it: a schema derived from a type's own template belongs to that
// type, and caching it under the bare name would hand it to a different type
// that happened to name it, making the answer depend on which type was
// validated first.
func (v *validateRun) schemaFor(name string, tc *typeContext) (validate.Schema, error) {
	hit, ok := v.schemas[name]
	if !ok {
		hit = v.resolveByName(name)
		v.schemas[name] = hit
	}

	if hit.err == nil {
		return hit.schema, nil
	}

	if name == tc.name && tc.tmplErr == nil {
		// A custom type that has not scaffolded a schema: its template's own
		// markers are its contract until it does (DESIGN-0015 §3, tier 3).
		return tc.fromTemplate, nil
	}

	return validate.Schema{}, hit.err
}

// resolveByName is the file-then-embedded half of schema resolution — the half
// that touches the filesystem, and so the half worth caching.
func (v *validateRun) resolveByName(name string) cachedSchema {
	skeleton, err := doctemplate.ResolveSchema(name, v.docsDir())
	if err != nil {
		return cachedSchema{err: err}
	}

	return cachedSchema{schema: validate.SchemaFromMarkers(skeleton)}
}

// schemaUnresolvedFinding is the finding for a schema name that resolves
// nowhere. Line 1 puts it on the frontmatter block the name came from, where
// the other frontmatter findings point.
func schemaUnresolvedFinding(name string, err error) validate.Finding {
	return validate.Finding{
		Code:     CodeSchemaUnresolved,
		Severity: validate.Error,
		Line:     1,
		Detail:   fmt.Sprintf("schema %q resolves to no skeleton: %v", name, err),
	}
}

// docsDir is the docs directory resolved under the repository root.
//
// Every doctemplate lookup needs it in this form: the package joins its own
// templates/... suffixes onto what it is given, and a config-relative path
// would send it to the process working directory instead of the repository a
// consumer asked about.
func (v *validateRun) docsDir() string {
	return v.repo.Path(v.repo.Cfg.DocsDir)
}

// templatePath names the repo-relative template file a person would edit to
// answer a Templates finding, or "" when the template came from the embedded
// set and there is no file to name.
//
// doctemplate.Resolve does not report which of its three tiers won, so this
// walks the first two in the same order: the explicit path in the type's
// config, then the conventional override. A stat rather than a read, because
// the content is already in hand and all that is wanted is its origin.
func (v *validateRun) templatePath(tc *typeContext) string {
	if tc.cfg.Template != "" {
		return v.repo.RelPath(v.repo.Path(tc.cfg.Template))
	}

	override := filepath.Join(v.docsDir(), config.TemplatesDir, tc.name+".md")
	if _, err := os.Stat(override); err == nil {
		return v.repo.RelPath(override)
	}

	return ""
}

// inferenceSpec derives the heading grammar that lets an unmarked document be
// read (DESIGN-0015 §6) from the type's resolved template.
//
// Known hazard, and a consequence of template resolution rather than of
// anything here: the grammar comes from the template's region markers, so a
// repository whose docs/templates/<type>.md is a pre-v2 copy without markers
// gets only the shared defaults. Inference then finds almost nothing, and
// every region the schema requires is reported region.missing on documents a
// person would call fine.
//
// The report says so in the same breath — that type's Templates entry reports
// the same regions missing from the template itself, which is the actual
// cause — and the fix is to re-export the override or to mark the documents.
// Falling back to the embedded template instead would be the wrong repair: it
// would validate a repository's documents against a grammar the repository
// does not use, which is precisely the drift the template check exists to
// find.
func (tc *typeContext) inferenceSpec() kinds.HeadingSpec {
	return kinds.SpecFromTemplate([]byte(tc.tmpl))
}

// render renders the type's template the way `docz create` does and returns
// the bytes with the filename a document made from them would take.
//
// The placeholders are chosen so that a sound template reports nothing: an id
// that is a prefix and a number of the configured width, a status the type
// allows, an ISO date, a non-empty title, and a filename whose number matches
// the id — that last pair is what the filename check compares, so passing the
// filename is safer than skipping the check.
func (tc *typeContext) render() ([]byte, string, error) {
	number := fmt.Sprintf("%0*d", tc.cfg.IDWidth, 1)
	slug := doctemplate.FilenameSlug(placeholderTitle)
	filename := number + "-" + slug + ".md"

	prefix := tc.cfg.IDPrefix
	if prefix == "" {
		// A type that configures no prefix would render an id of "-0001",
		// which is not a prefix and a number. That is the placeholder's
		// fault rather than the template's, so it gets one that parses.
		prefix = strings.ToUpper(tc.name)
	}

	status := placeholderStatus
	if len(tc.cfg.Statuses) > 0 {
		status = tc.cfg.Statuses[0]
	}

	body, err := doctemplate.Render(tc.tmpl, &doctemplate.Data{
		Number:   number,
		Title:    placeholderTitle,
		Date:     placeholderDate,
		Author:   placeholderAuthor,
		Status:   config.Status(status),
		Type:     config.DocType(tc.name),
		Prefix:   prefix,
		Slug:     slug,
		Filename: filename,
	})
	if err != nil {
		return nil, "", err
	}

	return []byte(body), filename, nil
}

// indexHeading is the README table heading and the display label behind it.
//
// The pair `docz update` builds: the configured plural_label, or the type name
// with its first rune upper-cased for a type that declares none (DESIGN-0006
// Decision 3). Built the same way here because a heading that differed by a
// word would report every README in the repository as drifted.
func (tc *typeContext) indexHeading() (heading, label string) {
	label = tc.cfg.PluralLabel
	if label == "" && tc.name != "" {
		runes := []rune(tc.name)
		runes[0] = unicode.ToUpper(runes[0])
		label = string(runes)
	}

	return "All " + label, label
}

// tally recounts a report's severity totals from its findings.
//
// Recounted rather than incremented as findings arrive, so the cancellation
// path returns totals that match the partial report it carries rather than
// whatever a pair of counters had reached.
func tally(report *ValidateReport) {
	report.Errors, report.Warnings = 0, 0

	for _, group := range [][]DocFindings{report.Docs, report.Templates} {
		for i := range group {
			for _, f := range group[i].Findings {
				switch f.Severity {
				case validate.Error:
					report.Errors++
				case validate.Warning:
					report.Warnings++
				}
			}
		}
	}
}
