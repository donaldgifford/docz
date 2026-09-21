package repo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/doctemplate"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/docwrite"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/index"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/toc"
)

// ErrMalformedOutput reports that a document's marked-up form did not read
// back as the regions the pass wrote into it.
//
// The pass runs docparse.Regions over its own output and compares the result
// against what it inserted (DESIGN-0015 §6). It cannot happen for a document
// whose spans inference located cleanly, which is the point: the check exists
// so that the one case nobody thought of — a span boundary landing inside an
// unterminated code fence, where a marker is text rather than a marker —
// aborts instead of writing a file whose regions say something the author did
// not.
var ErrMalformedOutput = errors.New("repo: marked-up document does not read back as the regions inserted")

// InsertRegionsOptions configures the migration pass.
type InsertRegionsOptions struct {
	// DryRun populates the same report and writes nothing.
	//
	// The CLI has no flag for it — plain `docz validate` is the preview, and
	// its region.inferred warning lists exactly the kinds --fix would mark
	// (DESIGN-0015 §6). The option is here for a library caller that wants
	// the report without the side effect, which is not something a consumer
	// should have to get by copying the pass.
	DryRun bool
}

// InsertRegionsResult is what the pass did to one document.
type InsertRegionsResult struct {
	// Path is the document's path relative to Repo.Root.
	Path string

	// Inserted names the kinds the pass marked, in document order. Nil for a
	// document that only had its marker spellings corrected.
	Inserted []string

	// Fixed counts the non-canonical marker lines rewritten in place.
	Fixed int
}

// InsertRegionsReport is the pass over one run.
//
// Every scanned document appears in exactly one of the two fields, so a
// caller can report "n of m documents marked" without counting twice.
type InsertRegionsReport struct {
	// Changed holds one result per document the pass would write.
	Changed []InsertRegionsResult

	// Unchanged holds the repo-relative path of every document the pass left
	// alone: one that already carries canonical docz regions, and one where
	// no heading in the type's spec was found.
	Unchanged []string
}

// InsertRegions marks the regions inference finds, for every document of
// every named type.
//
// This is `docz validate --fix`, and it is the only writer of region markers
// outside the templates (DESIGN-0015 §6). A nil or empty types slice means
// every enabled type, in Cfg.EnabledTypes order.
//
// The pass is idempotent by construction: a document that already carries any
// docz region is never given more, because markers once present are
// authoritative and re-inferring over them would mark the sections the author
// deliberately left unmarked. Such a document is only touched to canonicalize
// a marker docz read leniently, which is what makes a second run a no-op even
// for a repo whose markers were hand-typed.
//
// Nothing is invented. A heading the document lacks is skipped, and
// validate.Document then reports region.missing for the author to fix by
// hand — a migration that added the heading would be writing the document
// rather than marking it.
//
// The context is checked between types. A cancelled run returns the report
// completed so far together with ctx.Err(), and what was already written
// stays written.
func (r *Repo) InsertRegions(
	ctx context.Context, types []string, opts InsertRegionsOptions,
) (InsertRegionsReport, error) {
	var report InsertRegionsReport

	names, err := r.typesOrEnabled(types)
	if err != nil {
		return report, err
	}

	for _, typeName := range names {
		if err := ctx.Err(); err != nil {
			return report, err
		}

		if err := r.insertRegionsInType(ctx, typeName, opts, &report); err != nil {
			return report, err
		}
	}

	return report, nil
}

// insertRegionsInType runs the pass over one type's directory.
//
// The spec is resolved once per type rather than once per document: it comes
// from the type's template, which does not change mid-run, and resolving it
// per document would read the same file off disk for every plan in the repo.
func (r *Repo) insertRegionsInType(
	ctx context.Context, typeName string, opts InsertRegionsOptions, report *InsertRegionsReport,
) error {
	docs, err := r.Scan(ctx, typeName)
	if err != nil {
		return err
	}

	if len(docs) == 0 {
		return nil
	}

	spec, err := r.headingSpec(typeName)
	if err != nil {
		return err
	}

	dir := r.TypeDir(typeName)

	for i := range docs {
		path := filepath.Join(dir, docs[i].Filename)

		if err := r.insertRegionsInDoc(ctx, path, docs[i].Content, spec, opts, report); err != nil {
			return err
		}
	}

	return nil
}

// headingSpec derives the heading-to-kind map for a type from its resolved
// template.
//
// Derived rather than hand-written, and from the *resolved* template rather
// than the embedded one, so a repo's own override and a custom type's
// template both migrate their documents the way they describe them
// (DESIGN-0015 §6). This is also why repo needs no knowledge of any document
// type: the grammar arrives as data.
//
// A type whose template resolves to nothing is an error rather than a skip.
// Silence here would mean every document of that type came back Unchanged
// with no reason, which is the failure mode issue #92 was filed about.
func (r *Repo) headingSpec(typeName string) (kinds.HeadingSpec, error) {
	body, err := doctemplate.Resolve(
		typeName,
		r.Path(r.Cfg.Types[typeName].Template),
		r.Path(r.Cfg.DocsDir),
	)
	if err != nil {
		return nil, fmt.Errorf("resolving %s template: %w", typeName, err)
	}

	return kinds.SpecFromTemplate([]byte(body)), nil
}

// insertRegionsInDoc marks one document and records the outcome.
//
// The order is canonicalize, then splice, then verify, then write. Rewriting
// a marker spelling replaces one line with one line, so it cannot move a
// heading and the spans computed from the original still hold — which is what
// lets both steps happen in a single write rather than two.
func (r *Repo) insertRegionsInDoc(
	ctx context.Context,
	path string,
	content []byte,
	spec kinds.HeadingSpec,
	opts InsertRegionsOptions,
	report *InsertRegionsReport,
) error {
	rel := r.RelPath(path)

	// A CR anywhere is refused rather than spliced into. Every writer in the
	// module rejects CR endings, and a pass that inserted LF-only marker
	// lines into a CRLF document would leave behind a file with two kinds of
	// line ending and no way to tell which was original.
	if bytes.IndexByte(content, '\r') >= 0 {
		return &WriteError{Path: rel, Err: docwrite.ErrUnsupportedLineEndings}
	}

	regions, inferred := kinds.ResolveRegions(content, spec)

	out, fixed := canonicalizeMarkers(content)

	var inserted []string

	if inferred {
		out, inserted = spliceMarkers(out, regions)
	}

	if len(inserted) == 0 && fixed == 0 {
		report.Unchanged = append(report.Unchanged, rel)

		return nil
	}

	if !spansReadBack(out, regions, inferred) {
		// Reported rather than skipped, and with the document recorded as
		// unchanged, so a caller that logs the error still has a report whose
		// two fields account for every document it scanned.
		report.Unchanged = append(report.Unchanged, rel)

		return fmt.Errorf("%s: %w", rel, ErrMalformedOutput)
	}

	if !opts.DryRun {
		if err := os.WriteFile(path, out, config.FileMode); err != nil {
			return &WriteError{Path: rel, Err: err}
		}

		fireFileWritten(ctx, path, FileDocument)
	}

	report.Changed = append(report.Changed, InsertRegionsResult{
		Path:     rel,
		Inserted: inserted,
		Fixed:    fixed,
	})

	return nil
}

// canonicalizeMarkers rewrites every leniently-spelled marker line to the
// canonical spelling and returns the new bytes and the number of lines
// changed.
//
// A marker docz read but would not write is a marker somebody will edit again
// and spell differently again, so the fixer settles it (INV-0009 Finding 4).
// The whole line is replaced, indentation included, because the canonical
// form is the whole trimmed line and an indented marker is a marker docz was
// being generous about.
//
// The document is returned unchanged, byte for byte and slice for slice, when
// every marker is already canonical. Nothing here mutates the input.
func canonicalizeMarkers(doc []byte) ([]byte, int) {
	markers := docparse.Markers(doc)

	fixed := 0

	for i := range markers {
		if !markers[i].Canonical {
			fixed++
		}
	}

	if fixed == 0 {
		return doc, 0
	}

	text := string(doc)
	trailing := strings.HasSuffix(text, "\n")
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")

	for i := range markers {
		m := markers[i]
		if m.Canonical || m.Line < 1 || m.Line > len(lines) {
			continue
		}

		lines[m.Line-1] = markerLine(m.Kind, m.Role)
	}

	return joinLines(lines, trailing), fixed
}

// spliceMarkers writes a marker pair around every region and returns the
// kinds it marked, in document order.
//
// Nothing else changes. No heading is added, no prose is moved, no blank line
// is inserted or removed: the markers land immediately above the span's first
// line and immediately below its last, so whatever blank line separated the
// section from its neighbours still separates it, now from the marker. That
// is the rule the type packages' fixture pairs are built on, and a migration
// that tidied the document would make them prove nothing.
//
// Within one gap between two lines, openers are emitted outermost-first and
// closers innermost-first, so a nested pair reads as a stack: a phase opens
// before its tasks and closes after its criteria.
func spliceMarkers(doc []byte, regions []docparse.Region) ([]byte, []string) {
	if len(regions) == 0 {
		return doc, nil
	}

	text := string(doc)
	trailing := strings.HasSuffix(text, "\n")
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")

	// before[n] is emitted above line n and after[n] below it. An inferred
	// region's Start is the line above its heading and its End the line below
	// its last content line, which is exactly where the marker goes.
	before := make(map[int][]string, len(regions))
	after := make(map[int][]string, len(regions))

	byDepth := make([]docparse.Region, len(regions))
	copy(byDepth, regions)
	slices.SortStableFunc(byDepth, func(a, b docparse.Region) int { return a.Depth - b.Depth })

	for i := range byDepth {
		r := byDepth[i]

		before[r.Start+1] = append(before[r.Start+1], markerLine(r.Kind, docparse.Start))
		after[r.End-1] = append([]string{markerLine(r.Kind, docparse.End)}, after[r.End-1]...)
	}

	out := make([]string, 0, len(lines)+2*len(regions))

	for n := 1; n <= len(lines); n++ {
		out = append(out, before[n]...)
		out = append(out, lines[n-1])
		out = append(out, after[n]...)
	}

	marked := make([]string, 0, len(regions))
	for i := range regions {
		marked = append(marked, regions[i].Kind)
	}

	return joinLines(out, trailing), marked
}

// joinLines rebuilds a document from its lines, restoring the trailing
// newline only when the input had one.
//
// Preserved rather than normalized because the pass promises to change
// nothing but markers, and whether a file ends in a newline is something a
// linter somewhere already has an opinion about. A document ending in two
// newlines keeps both: the split leaves the extra one as a final empty line,
// which rejoins as itself.
func joinLines(lines []string, trailing bool) []byte {
	joined := strings.Join(lines, "\n")
	if trailing {
		joined += "\n"
	}

	return []byte(joined)
}

// markerLine returns the canonical spelling of one marker.
//
// The two legacy pairs keep their own spellings — the ToC pair for Marksman
// compatibility, the README index pair because index.Splice finds it by that
// name — and both constants are taken from the packages that own them rather
// than restated here, so a spelling cannot drift between the writer and the
// reader.
func markerLine(kind string, role docparse.Role) string {
	switch kind {
	case docparse.TocKind:
		if role == docparse.End {
			return toc.EndMarker
		}

		return toc.BeginMarker
	case docparse.IndexKind:
		if role == docparse.End {
			return index.EndMarker
		}

		return index.BeginMarker
	default:
		return "<!--docz:" + kind + ":" + role.String() + "-->"
	}
}

// spansReadBack reports whether the pass's own output reads back as the
// regions it meant to write (DESIGN-0015 §6).
//
// The two branches check different things because they promise different
// things. A spliced document must come back carrying exactly the regions
// inference found, each closed and at the depth it was inferred at. A
// document that was only canonicalized must come back with its spans
// untouched, line numbers included — a rewrite that replaces one line with
// one line cannot legitimately change a single span, so any difference is a
// bug and not a judgement call.
func spansReadBack(out []byte, want []docparse.Region, inferred bool) bool {
	if !inferred {
		return slices.Equal(docparse.Regions(out), want)
	}

	got := doczRegions(out)
	if len(got) != len(want) {
		return false
	}

	for i := range got {
		if got[i].Kind != want[i].Kind || got[i].Depth != want[i].Depth || !got[i].Closed {
			return false
		}
	}

	return true
}

// doczRegions returns the regions docz's own markers delimit, dropping the
// two legacy pairs.
//
// The ToC and index pairs are excluded because the pass never inserts them
// and a v1 document carries a ToC pair already, so counting it would make
// every migrated document look like it gained a region nobody asked for.
func doczRegions(doc []byte) []docparse.Region {
	all := docparse.Regions(doc)

	out := make([]docparse.Region, 0, len(all))

	for i := range all {
		if all[i].Kind == docparse.TocKind || all[i].Kind == docparse.IndexKind {
			continue
		}

		out = append(out, all[i])
	}

	return out
}
