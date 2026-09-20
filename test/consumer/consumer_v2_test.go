package consumer

// The v2.0.0 surface proof (IMPL-0018 Phase 1): the region walker, the
// generic validator, the kinds readers, all five type packages, and the
// docwrite byte cores, each exercised from outside the module.
//
// Everything here is bytes in and values out. Not one test writes a file or
// names a path, because that is the whole point of the layer: docz-api reads
// a document through the GitHub API and never has a checkout to point at
// (DESIGN-0014 §2.7). The fixtures are inline for the same reason — a
// consumer that had to ship testdata to call Parse would be a consumer with
// a filesystem dependency.

import (
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/adr"
	"github.com/donaldgifford/docz/v2/pkg/design"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/docwrite"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/validate"
	"github.com/donaldgifford/docz/v2/pkg/impl"
	"github.com/donaldgifford/docz/v2/pkg/investigation"
	"github.com/donaldgifford/docz/v2/pkg/rfc"
)

// The fixtures. Each is the smallest document that still carries the shape
// its package is being asked about, and each names a value below that could
// only come from reading it: a table cell, a bullet's line, a resolved
// option's letter.
//
// Two of the five are deliberately unmarked (adr, design), because inference
// is a permanent path and not a migration aid (DESIGN-0015 §6) — a consumer
// pointed at a repo that never ran the fixer has to get the same facts.

const rfcFixture = `---
id: RFC-0007
title: Structured Regions
status: Accepted
author: Consumer Test
created: 2026-09-20
---

# RFC-0007: Structured Regions

<!--docz:summary:start-->
## Summary

A marker makes a section addressable by kind rather than by heading text.
<!--docz:summary:end-->

<!--docz:risks:start-->
## Risks and Mitigations

| Risk | Impact | Likelihood | Mitigation |
| --- | --- | --- | --- |
| Markers drift from headings | High | Low | The validator reports the drift |
| Comment noise in every file | Low | Medium | |
<!--docz:risks:end-->

<!--docz:open-questions:start-->
## Open Questions

### 1. Is a marker required for every section?

- a. Yes, every section in the skeleton *(recommendation)*
- b. Only the sections a consumer reads

> **Resolved 2026-09-20 (a):** the skeleton is the schema.
<!--docz:open-questions:end-->

<!--docz:references:start-->
## References

- [DESIGN-0015](../design/0015-structured-regions.md)
- A bullet with no link at all
<!--docz:references:end-->
`

const adrFixture = `---
id: ADR-0004
title: Markers Are Authoritative
status: Accepted
author: Consumer Test
created: 2026-09-20
---

# ADR-0004: Markers Are Authoritative

## Context

A heading is prose and an author may rewrite it at will.

## Decision

Where a document carries markers, the markers define its regions and the
headings are ignored.

## Consequences

### Positive

- A consumer addresses a section by kind.
- Renaming a heading no longer moves a region.

### Negative

- Every document carries comment noise.

## References

- [DESIGN-0015](../design/0015-structured-regions.md)
`

const designFixture = `---
id: DESIGN-0016
title: The Region Grammar
status: Approved
author: Consumer Test
created: 2026-09-20
---

# DESIGN-0016: The Region Grammar

## Overview

The grammar is over region kinds, never over type names.

### Goals

- Address every section by a stable kind.
- Let a custom type reuse a built-in's reader.

### Non-Goals

- Rewriting anybody's prose.

## Decisions

| # | Question | Resolution | Date |
| --- | --- | --- | --- |
| 1 | Is the grammar over kinds? | Yes, never over type names | 2026-09-20 |
`

const implFixture = `---
id: IMPL-0019
title: Wire The Region Reader
status: In Progress
author: Consumer Test
created: 2026-09-20
---

# IMPL-0019: Wire The Region Reader

<!--docz:objective:start-->
## Objective

**Implements:** DESIGN-0015

Read a document's regions from its markers.
<!--docz:objective:end-->

<!--docz:phase:start-->
### Phase 1: The walker

The marker scan and the region stack.

<!--docz:tasks:start-->
#### Tasks

- [x] Add the marker scanner
      verify: ` + "`go test ./pkg/doczcore/docparse/`" + `
- [ ] Add the region stack
- [ ] Wire the fixer
      deferred - human required: the corpus migration needs review
<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- Every marked document round-trips unchanged.
<!--docz:criteria:end-->
<!--docz:phase:end-->
`

const investigationFixture = `---
id: INV-0011
title: Can A Consumer Parse Without A Checkout
status: Concluded
author: Consumer Test
created: 2026-09-20
---

# INV-0011: Can A Consumer Parse Without A Checkout

<!--docz:context:start-->
## Context

**Triggered by:** docz-api issue #42

The service reads documents through the GitHub API.
<!--docz:context:end-->

<!--docz:environment:start-->
## Environment

| Component | Version / Value |
| --- | --- |
| docz | v2.0.0-beta.1 |
| Go | 1.26.4 |
<!--docz:environment:end-->

<!--docz:findings:start-->
## Findings

### F1: Every reader takes bytes

Not one of them opens a file.

### F2: Line numbers are document lines

A consumer splices at the line a reader reports.
<!--docz:findings:end-->

<!--docz:conclusion:start-->
## Conclusion

**Answer:** Yes — the whole read path is bytes in and values out.
<!--docz:conclusion:end-->
`

// The skeleton a consumer resolves the schema from. In this repo it is the
// embedded templates/schema/rfc.md; a consumer fetches the repo's own copy,
// which is why SchemaFromMarkers takes bytes rather than a type name.
const rfcSkeleton = `<!--toc:start-->
<!--toc:end-->
<!--docz:summary:start-->
<!--docz:summary:end-->
<!--docz:problem:start-->
<!--docz:problem:end-->
<!--docz:proposal:start-->
<!--docz:proposal:end-->
<!--docz:alternatives:start-->
<!--docz:alternatives:end-->
<!--docz:risks:start-->
<!--docz:risks:end-->
<!--docz:criteria:start-->
<!--docz:criteria:end-->
<!--docz:references:start-->
<!--docz:references:end-->
`

// TestExternalConsumerParsesRFC reads the two shapes only rfc has: the
// four-column risks table and the criteria/alternatives split.
func TestExternalConsumerParsesRFC(t *testing.T) {
	doc, err := rfc.Parse([]byte(rfcFixture))
	if err != nil {
		t.Fatalf("rfc.Parse() = %v, want nil", err)
	}

	if doc.ID != "RFC-0007" || doc.Status != config.Status("Accepted") {
		t.Errorf("ID/Status = %q/%q, want RFC-0007/Accepted", doc.ID, doc.Status)
	}
	if doc.Inferred {
		t.Error("Inferred = true for a marked document, want false")
	}
	if !strings.HasPrefix(doc.Summary, "A marker makes") {
		t.Errorf("Summary = %q, want it to start with the section's prose", doc.Summary)
	}

	if len(doc.Risks) != 2 {
		t.Fatalf("len(Risks) = %d, want 2", len(doc.Risks))
	}
	if want := "Markers drift from headings"; doc.Risks[0].Risk != want {
		t.Errorf("Risks[0].Risk = %q, want %q", doc.Risks[0].Risk, want)
	}
	if got, want := doc.Risks[0].Line, lineOf(t, rfcFixture, "| Markers drift"); got != want {
		t.Errorf("Risks[0].Line = %d, want %d (the document's line, not the region's)", got, want)
	}
	// The row is kept with an empty Mitigation rather than dropped: that is
	// the row rfc.risks.no-mitigation is about, and a parser that threw it
	// away could not report it.
	if doc.Risks[1].Mitigation != "" {
		t.Errorf("Risks[1].Mitigation = %q, want empty", doc.Risks[1].Mitigation)
	}

	if len(doc.References) != 2 {
		t.Fatalf("len(References) = %d, want 2", len(doc.References))
	}
	if want := "../design/0015-structured-regions.md"; doc.References[0].URL != want {
		t.Errorf("References[0].URL = %q, want %q", doc.References[0].URL, want)
	}
	if doc.References[1].URL != "" {
		t.Errorf("References[1].URL = %q, want empty for a bullet with no link", doc.References[1].URL)
	}
}

// TestExternalConsumerParsesADR covers the nested consequences, which is
// the only depth-1 shape in the five packages and so the only place a
// region-relative line number could survive unnoticed.
func TestExternalConsumerParsesADR(t *testing.T) {
	doc, err := adr.Parse([]byte(adrFixture))
	if err != nil {
		t.Fatalf("adr.Parse() = %v, want nil", err)
	}

	// Unmarked on purpose: inference is a permanent path, so a consumer
	// pointed at a repo that never ran the fixer gets the same facts and one
	// flag saying where they came from.
	if !doc.Inferred {
		t.Error("Inferred = false for an unmarked document, want true")
	}
	if !strings.HasPrefix(doc.Decision, "Where a document carries markers") {
		t.Errorf("Decision = %q, want the section's prose", doc.Decision)
	}

	if got := len(doc.Consequences.Positive); got != 2 {
		t.Fatalf("len(Consequences.Positive) = %d, want 2", got)
	}
	if want := "A consumer addresses a section by kind."; doc.Consequences.Positive[0].Text != want {
		t.Errorf("Consequences.Positive[0].Text = %q, want %q",
			doc.Consequences.Positive[0].Text, want)
	}
	got := doc.Consequences.Positive[0].Line
	if want := lineOf(t, adrFixture, "- A consumer addresses"); got != want {
		t.Errorf("Consequences.Positive[0].Line = %d, want %d", got, want)
	}
	if n := len(doc.Consequences.Negative); n != 1 {
		t.Errorf("len(Consequences.Negative) = %d, want 1", n)
	}
	if n := len(doc.Consequences.Neutral); n != 0 {
		t.Errorf("len(Consequences.Neutral) = %d, want 0 for an absent subsection", n)
	}
}

// TestExternalConsumerParsesDesign covers the goals/non-goals items and the
// decisions table's by-name column mapping.
func TestExternalConsumerParsesDesign(t *testing.T) {
	doc, err := design.Parse([]byte(designFixture))
	if err != nil {
		t.Fatalf("design.Parse() = %v, want nil", err)
	}

	if !doc.Inferred {
		t.Error("Inferred = false for an unmarked document, want true")
	}
	if len(doc.Goals) != 2 || len(doc.NonGoals) != 1 {
		t.Fatalf("len(Goals)/len(NonGoals) = %d/%d, want 2/1", len(doc.Goals), len(doc.NonGoals))
	}
	if want := "Address every section by a stable kind."; doc.Goals[0].Text != want {
		t.Errorf("Goals[0].Text = %q, want %q", doc.Goals[0].Text, want)
	}

	if len(doc.Decisions) != 1 {
		t.Fatalf("len(Decisions) = %d, want 1", len(doc.Decisions))
	}
	// Columns are matched by header name, not position, so a fourth Date
	// column changes nothing.
	dec := doc.Decisions[0]
	if dec.Number != 1 || dec.Resolution != "Yes, never over type names" {
		t.Errorf("Decisions[0] = {%d, %q}, want {1, %q}",
			dec.Number, dec.Resolution, "Yes, never over type names")
	}
	if want := lineOf(t, designFixture, "| 1 | Is the grammar"); dec.Line != want {
		t.Errorf("Decisions[0].Line = %d, want %d", dec.Line, want)
	}
}

// TestExternalConsumerParsesImpl covers the phase/task tree, which is the
// deepest model in the five packages and the one issue #100 asked for.
func TestExternalConsumerParsesImpl(t *testing.T) {
	doc, err := impl.Parse([]byte(implFixture))
	if err != nil {
		t.Fatalf("impl.Parse() = %v, want nil", err)
	}

	if want := []string{"DESIGN-0015"}; len(doc.Implements) != 1 || doc.Implements[0] != want[0] {
		t.Errorf("Implements = %q, want %q", doc.Implements, want)
	}
	if len(doc.Phases) != 1 {
		t.Fatalf("len(Phases) = %d, want 1", len(doc.Phases))
	}

	phase := doc.Phases[0]
	if phase.Token != "1" || phase.Title != "The walker" {
		t.Errorf("Phase{Token, Title} = {%q, %q}, want {1, The walker}", phase.Token, phase.Title)
	}
	if len(phase.Tasks) != 3 {
		t.Fatalf("len(Tasks) = %d, want 3", len(phase.Tasks))
	}

	if !phase.Tasks[0].Checked {
		t.Error("Tasks[0].Checked = false, want true")
	}
	// Verify is the command out of the verify line's backticks, not the line:
	// a bot that runs it needs the command and nothing around it.
	if want := "go test ./pkg/doczcore/docparse/"; phase.Tasks[0].Verify != want {
		t.Errorf("Tasks[0].Verify = %q, want %q", phase.Tasks[0].Verify, want)
	}
	if want := lineOf(t, implFixture, "- [x] Add the marker scanner"); phase.Tasks[0].Line != want {
		t.Errorf("Tasks[0].Line = %d, want %d", phase.Tasks[0].Line, want)
	}

	// The deferral is a marker on the task, not prose inside its text: a
	// consumer that reports blocked work filters on it.
	if phase.Tasks[2].Deferred == nil {
		t.Fatal("Tasks[2].Deferred = nil, want the deferral marker")
	}
	if !strings.Contains(phase.Tasks[2].Deferred.Note, "corpus migration") {
		t.Errorf("Tasks[2].Deferred.Note = %q, want the note after the dash",
			phase.Tasks[2].Deferred.Note)
	}
	if n := len(phase.Criteria); n != 1 {
		t.Errorf("len(Criteria) = %d, want 1", n)
	}
}

// TestExternalConsumerParsesInvestigation covers the two derived values only
// this package has: the bold Answer field and the verdict read off it.
func TestExternalConsumerParsesInvestigation(t *testing.T) {
	doc, err := investigation.Parse([]byte(investigationFixture))
	if err != nil {
		t.Fatalf("investigation.Parse() = %v, want nil", err)
	}

	if want := "docz-api issue #42"; doc.TriggeredBy != want {
		t.Errorf("TriggeredBy = %q, want %q", doc.TriggeredBy, want)
	}
	if !strings.HasPrefix(doc.Answer, "Yes —") {
		t.Errorf("Answer = %q, want it to start with the verdict word", doc.Answer)
	}
	if doc.Verdict != investigation.VerdictYes {
		t.Errorf("Verdict = %v, want VerdictYes", doc.Verdict)
	}
	if got := doc.Verdict.String(); got != "yes" {
		t.Errorf("Verdict.String() = %q, want %q", got, "yes")
	}

	// Findings headings are the author's own: the package reports them as
	// sections rather than matching them against a grammar.
	if len(doc.Findings) != 2 {
		t.Fatalf("len(Findings) = %d, want 2", len(doc.Findings))
	}
	if want := "F1: Every reader takes bytes"; doc.Findings[0].Title != want {
		t.Errorf("Findings[0].Title = %q, want %q", doc.Findings[0].Title, want)
	}
	if want := lineOf(t, investigationFixture, "### F1:"); doc.Findings[0].Line != want {
		t.Errorf("Findings[0].Line = %d, want %d", doc.Findings[0].Line, want)
	}

	if len(doc.Environment) != 2 {
		t.Fatalf("len(Environment) = %d, want 2", len(doc.Environment))
	}
	if doc.Environment[0].Component != "docz" || doc.Environment[0].Value != "v2.0.0-beta.1" {
		t.Errorf("Environment[0] = {%q, %q}, want {docz, v2.0.0-beta.1}",
			doc.Environment[0].Component, doc.Environment[0].Value)
	}
}

// TestExternalConsumerReadsRegions is the walker on its own, for a consumer
// that wants the spans without a typed model — docz-api slices a section out
// of a document it is serving and never needs to know what an RFC is.
func TestExternalConsumerReadsRegions(t *testing.T) {
	regions := docparse.Regions([]byte(rfcFixture))
	if len(regions) != 4 {
		t.Fatalf("len(Regions) = %d, want 4", len(regions))
	}

	want := []string{"summary", "risks", "open-questions", "references"}
	for i, kind := range want {
		if regions[i].Kind != kind {
			t.Errorf("Regions[%d].Kind = %q, want %q", i, regions[i].Kind, kind)
		}
		if regions[i].Depth != 0 {
			t.Errorf("Regions[%d].Depth = %d, want 0", i, regions[i].Depth)
		}
		if !regions[i].Closed {
			t.Errorf("Regions[%d].Closed = false, want true", i)
		}
	}

	if got := regions[0].Start; got != lineOf(t, rfcFixture, "<!--docz:summary:start-->") {
		t.Errorf("Regions[0].Start = %d, want the start marker's own line", got)
	}

	// RegionBytes is how a consumer gets from a span to the bytes, without
	// doing its own line arithmetic.
	body := kinds.RegionBytes([]byte(rfcFixture), regions[0])
	if !strings.Contains(string(body), "addressable by kind") {
		t.Errorf("RegionBytes(summary) = %q, want the section's prose", body)
	}
}

// TestExternalConsumerReadsOpenQuestions exercises a kinds reader directly:
// a consumer that renders a decision log walks the questions and their
// resolutions without wanting any of the rest of the document.
func TestExternalConsumerReadsOpenQuestions(t *testing.T) {
	var region docparse.Region

	for _, r := range docparse.Regions([]byte(rfcFixture)) {
		if r.Kind == "open-questions" {
			region = r

			break
		}
	}

	if region.Kind == "" {
		t.Fatal("no open-questions region in the fixture")
	}

	questions := kinds.OpenQuestions(kinds.RegionBytes([]byte(rfcFixture), region))
	if len(questions) != 1 {
		t.Fatalf("len(OpenQuestions) = %d, want 1", len(questions))
	}

	q := questions[0]
	if q.Number != 1 || q.Title != "Is a marker required for every section?" {
		t.Errorf("Question{Number, Title} = {%d, %q}, want {1, …}", q.Number, q.Title)
	}
	if len(q.Options) != 2 {
		t.Fatalf("len(Options) = %d, want 2", len(q.Options))
	}
	if q.Options[0].Letter != "a" || !q.Options[0].Recommended {
		t.Errorf("Options[0] = {%q, recommended=%v}, want {a, true}",
			q.Options[0].Letter, q.Options[0].Recommended)
	}
	if q.Options[1].Recommended {
		t.Error("Options[1].Recommended = true, want false")
	}
	if q.Resolved == nil {
		t.Fatal("Resolved = nil, want the resolution blockquote")
	}
	if q.Resolved.Choice != "a" || q.Resolved.Date != "2026-09-20" {
		t.Errorf("Resolved{Choice, Date} = {%q, %q}, want {a, 2026-09-20}",
			q.Resolved.Choice, q.Resolved.Date)
	}

	// Lines are the region's here, because the reader was handed the region.
	// The type packages shift them to the document; a consumer calling a
	// reader itself adds region.Start the same way.
	if q.Line+region.Start != lineOf(t, rfcFixture, "### 1. Is a marker required") {
		t.Errorf("region.Start + Question.Line = %d, want the heading's document line",
			region.Start+q.Line)
	}
}

// TestExternalConsumerValidatesAgainstSkeletonSchema is the generic tier:
// the schema comes from a skeleton's markers, so a repo that adds a section
// to its own skeleton gets it required without docz shipping a new release.
func TestExternalConsumerValidatesAgainstSkeletonSchema(t *testing.T) {
	schema := validate.SchemaFromMarkers([]byte(rfcSkeleton))
	if len(schema.Regions) != 8 {
		t.Fatalf("len(schema.Regions) = %d, want 8", len(schema.Regions))
	}

	cfg := config.DefaultConfig()

	findings := validate.Document([]byte(rfcFixture), validate.Options{
		Schema:   schema,
		Type:     cfg.Types["rfc"],
		Filename: "docs/rfc/0007-structured-regions.md",
	})

	codes := codeSet(findings)

	// The fixture carries three of the skeleton's seven content regions, so
	// the other four are each reported once. This is the finding docz validate
	// prints and docz validate --fix does not fix: a missing section is the
	// author's to write.
	for _, kind := range []string{"problem", "proposal", "alternatives", "criteria"} {
		if !hasFinding(findings, "region.missing", kind) {
			t.Errorf("no region.missing finding for %q; got %v", kind, codes)
		}
	}
	if hasFinding(findings, "region.missing", "summary") {
		t.Error("region.missing reported for summary, which the fixture carries")
	}

	// The skeleton's toc region is the one kind that is not reported as a
	// missing region: the ToC is generated rather than written, so an absent
	// one is toc.missing and a consumer that offers to run docz update keys
	// on that code instead.
	if _, ok := codes["toc.missing"]; !ok {
		t.Errorf("no toc.missing finding; got %v", codes)
	}
	if hasFinding(findings, "region.missing", "toc") {
		t.Error("region.missing reported for toc, which has its own code")
	}
	if _, ok := codes["region.inferred"]; ok {
		t.Error("region.inferred reported for a marked document")
	}

	// Every finding carries a code, and a consumer filters on the code
	// rather than on the wording.
	for _, f := range findings {
		if f.Code == "" {
			t.Errorf("finding with no code: %+v", f)
		}
		if f.Severity != validate.Error && f.Severity != validate.Warning {
			t.Errorf("finding %q has severity %v, want Error or Warning", f.Code, f.Severity)
		}
	}
}

// TestExternalConsumerValidatesByInference pins the other half: a consumer
// hands the type package's heading table to the validator and an unmarked
// document validates against the same schema, with one warning saying the
// regions were inferred.
func TestExternalConsumerValidatesByInference(t *testing.T) {
	findings := validate.Document([]byte(adrFixture), validate.Options{
		Schema:   validate.SchemaFromMarkers([]byte(rfcSkeleton)),
		Headings: adr.Headings(),
	})

	codes := codeSet(findings)
	if _, ok := codes["region.inferred"]; !ok {
		t.Errorf("no region.inferred finding for an unmarked document; got %v", codes)
	}

	// The document's own sections were found, so they are not reported
	// missing — inference feeds every other check as if the markers were
	// there.
	if hasFinding(findings, "region.missing", "references") {
		t.Error("region.missing reported for references, which inference finds")
	}
}

// TestExternalConsumerSetsStatusInBytes is the write side without a
// filesystem: docz-api reads a document through the GitHub API and commits
// the result back through it, so the byte core is the only entry point it
// can use.
func TestExternalConsumerSetsStatusInBytes(t *testing.T) {
	in := []byte(rfcFixture)

	out, old, err := docwrite.SetStatusBytes(in, "Superseded")
	if err != nil {
		t.Fatalf("SetStatusBytes() = %v, want nil", err)
	}
	if old != "Accepted" {
		t.Errorf("old status = %q, want Accepted", old)
	}

	// One line changes and nothing else, which is what makes the commit
	// reviewable.
	if got := diffLines(string(in), string(out)); got != 1 {
		t.Errorf("%d lines differ, want 1", got)
	}

	// The input is untouched, so a caller may keep or reuse what it passed.
	if string(in) != rfcFixture {
		t.Error("SetStatusBytes modified its input")
	}

	// And the rewritten document parses back with the new status.
	doc, err := rfc.Parse(out)
	if err != nil {
		t.Fatalf("rfc.Parse(out) = %v, want nil", err)
	}
	if doc.Status != config.Status("Superseded") {
		t.Errorf("reparsed Status = %q, want Superseded", doc.Status)
	}
}

// TestExternalConsumerSetsTaskState is the checkbox half of the byte core:
// a bot that closes out a phase flips one task at the line impl.Parse
// reported, with no line arithmetic of its own.
func TestExternalConsumerSetsTaskState(t *testing.T) {
	doc, err := impl.Parse([]byte(implFixture))
	if err != nil {
		t.Fatalf("impl.Parse() = %v, want nil", err)
	}

	task := doc.Phases[0].Tasks[1]
	if task.Checked {
		t.Fatalf("fixture task %q is already checked", task.Text)
	}

	out, err := docwrite.SetTaskStateBytes([]byte(implFixture), task.Line, true)
	if err != nil {
		t.Fatalf("SetTaskStateBytes(line %d) = %v, want nil", task.Line, err)
	}
	if got := diffLines(implFixture, string(out)); got != 1 {
		t.Errorf("%d lines differ, want 1", got)
	}

	after, err := impl.Parse(out)
	if err != nil {
		t.Fatalf("impl.Parse(out) = %v, want nil", err)
	}
	if !after.Phases[0].Tasks[1].Checked {
		t.Error("task is still unchecked after SetTaskStateBytes")
	}
}

// lineOf returns the 1-based line of fixture that contains want, failing the
// test when no line does.
//
// Expected line numbers are located rather than written down: a fixture edit
// then moves them with it, and the assertion still says what it means —
// "Parse reports the line this text is actually on".
func lineOf(t *testing.T, fixture, want string) int {
	t.Helper()

	for i, line := range strings.Split(fixture, "\n") {
		if strings.Contains(line, want) {
			return i + 1
		}
	}

	t.Fatalf("no line of the fixture contains %q", want)

	return 0
}

// diffLines counts the lines that differ between two documents of the same
// shape.
func diffLines(before, after string) int {
	a, b := strings.Split(before, "\n"), strings.Split(after, "\n")
	if len(a) != len(b) {
		return -1
	}

	n := 0

	for i := range a {
		if a[i] != b[i] {
			n++
		}
	}

	return n
}

// codeSet collapses findings to the set of codes they carry, for an
// assertion about what was and was not reported.
func codeSet(findings []validate.Finding) map[string]int {
	out := make(map[string]int, len(findings))
	for _, f := range findings {
		out[f.Code]++
	}

	return out
}

// hasFinding reports whether findings holds one with this code about this
// kind.
func hasFinding(findings []validate.Finding, code, kind string) bool {
	for _, f := range findings {
		if f.Code == code && f.Kind == kind {
			return true
		}
	}

	return false
}
