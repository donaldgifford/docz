package validate

import (
	"fmt"
	"strings"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/toc"
)

// Options is what a caller tells Document about the document it is
// validating. Every field is optional, and an empty Options still yields a
// useful run: marker well-formedness, frontmatter shape, and the content
// rules of whatever kinds the document happens to carry.
type Options struct {
	// Schema is the set of regions the document must have, resolved by the
	// caller from the name in its frontmatter (DESIGN-0015 §3). The zero
	// Schema requires nothing, which makes the run well-formedness only —
	// what a document whose schema name resolves nowhere gets.
	Schema Schema

	// Type carries the statuses, id prefix, and id width to check
	// frontmatter against. A zero TypeConfig skips each check it would
	// have driven rather than inventing a default: docz cannot know what a
	// repo's statuses are.
	Type config.TypeConfig

	// Filename is the document's path, for the filename checks. Empty skips
	// them, which is what docz-api's no-checkout path needs: it has the
	// bytes and no file.
	Filename string

	// Headings enables inference for a document that carries no markers
	// (DESIGN-0015 §6). Empty disables it, and every unmarked section is
	// then region.missing.
	Headings kinds.HeadingSpec

	// MinHeadings is the ToC threshold, matching the repo's toc.min_headings.
	// Zero means the config default is not known, and toc.stale is then
	// checked only for a document whose ToC region already has entries.
	MinHeadings int
}

// Document runs every generic check and returns the findings in document
// order, with the ones about the document as a whole first.
//
// It never fails. A document with no frontmatter is a document with a
// finding, not an error return: a validator that refused to look at a broken
// document would be useless on exactly the documents that need it most.
//
// Regions come from the document's markers when it has any. When it has none
// and Options.Headings is set, they are inferred from its headings, one
// region.inferred warning is emitted, and every other check then runs over
// the inferred regions as if they were marked — a document without markers
// is not an error (DESIGN-0015 §4 as amended). A schema kind whose heading is
// absent is still region.missing, because that section is genuinely
// unrecognisable and the author has to act.
//
// §4); the internal helpers below take a pointer.
//
//nolint:gocritic // Options by value is the published signature (DESIGN-0015
func Document(content []byte, opts Options) []Finding {
	findings := checkFile(content)
	findings = append(findings, checkFrontmatter(content, &opts)...)
	findings = append(findings, checkMarkers(content)...)

	regions, inferred := resolveRegions(content, &opts)
	if inferred {
		findings = append(findings, inferredFinding(regions, opts.Schema, opts.Headings))
	}

	findings = append(findings, checkRegions(regions, opts.Schema)...)
	findings = append(findings, checkContent(content, regions)...)
	findings = append(findings, checkToC(content, &opts)...)

	sortFindings(findings)

	return findings
}

// resolveRegions picks between the document's markers and its headings.
//
// With no heading spec there is nothing to infer from, so an unmarked
// document has no regions and the schema reports each one missing. That is
// the pre-inference behaviour, kept reachable because docz-api validating
// raw bytes may not know the type's headings.
func resolveRegions(content []byte, opts *Options) (regions []docparse.Region, inferred bool) {
	if len(opts.Headings) == 0 {
		return docparse.Regions(content), false
	}

	return kinds.ResolveRegions(content, opts.Headings)
}

// checkToC reports a document whose ToC region is absent or out of date.
//
// Both are warnings: a stale ToC is a broken link in a rendered page, not a
// wrong document. The freshness check regenerates through the toc package
// rather than comparing by eye, so a finding and `docz update` can never
// disagree about whether a document needs one — the same reason the content
// rules call the kinds readers.
//
// Only a document that already has a ToC region is checked for staleness. A
// repo that does not use them is not doing anything wrong, which is why
// toc.missing fires only when the schema asks for one.
func checkToC(content []byte, opts *Options) []Finding {
	var region docparse.Region

	// The document's own markers, not the region list the caller's other
	// checks run over. A ToC region is a literal marker pair and can never
	// be inferred, so when inference replaced the list — an unmarked
	// document — a real, filled ToC pair had vanished from it and this
	// reported toc.missing on a document whose ToC was perfectly good.
	for _, r := range docparse.Regions(content) {
		if r.Kind == docparse.TocKind {
			region = r

			break
		}
	}

	if region.Kind == "" {
		if !schemaWants(opts.Schema, docparse.TocKind) {
			return nil
		}

		return []Finding{{
			Code:     "toc.missing",
			Severity: Warning,
			Kind:     docparse.TocKind,
			Detail:   "the schema requires a table of contents and the document has no marker pair",
		}}
	}

	current := strings.TrimSpace(string(kinds.RegionBytes(content, region)))

	// An empty region is not yet generated, which is different from stale.
	// `docz create` writes the pair empty and `docz update` fills it, so
	// reporting drift here would fail every document the moment it was
	// created — caught by the zero-findings golden over the templates.
	if current == "" {
		return nil
	}

	result := toc.UpdateToC(string(content), opts.MinHeadings)
	if !result.Found || freshToC(result.Updated) == current {
		return nil
	}

	return []Finding{{
		Code:     "toc.stale",
		Severity: Warning,
		Line:     region.Start,
		Kind:     docparse.TocKind,
		Detail: fmt.Sprintf("table of contents is out of date; %s",
			"run docz update to regenerate it"),
	}}
}

// freshToC extracts what a regenerated document holds between the markers,
// so it compares against the region's current content on equal terms.
func freshToC(updated string) string {
	_, after, found := strings.Cut(updated, toc.BeginMarker)
	if !found {
		return ""
	}

	body, _, found := strings.Cut(after, toc.EndMarker)
	if !found {
		return ""
	}

	return strings.TrimSpace(body)
}

// schemaWants reports whether the schema requires a kind anywhere.
func schemaWants(schema Schema, kind string) bool {
	for _, want := range schema.Regions {
		if want.Kind == kind {
			return true
		}
	}

	return false
}
