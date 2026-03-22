---
id: DESIGN-0003
title: "Table of Contents Generation"
status: Draft
author: Donald Gifford
created: 2026-03-22
---

<!-- markdownlint-disable-file MD025 MD041 -->

# DESIGN 0003: Table of Contents Generation

**Status:** Draft **Author:** Donald Gifford **Date:** 2026-03-22

## Overview

Add automatic table of contents (ToC) generation to `docz update`. When
`docz update` runs, it should scan each document for ToC markers and regenerate
the table of contents between them based on the document's heading structure.
This complements the existing README index table generation and provides
navigable ToC within longer documents like RFCs and design docs.

## Goals and Non-Goals

### Goals

- Generate a markdown table of contents from document headings
- Use HTML comment markers to delimit the ToC region (compatible with existing
  editor plugins)
- Run ToC generation as part of `docz update` alongside README index updates
- Support configuring ToC behavior per-repo via `.docz.yaml`
- Respect the `--dry-run` flag for previewing ToC changes
- Work with all six document types

### Non-Goals

- Replacing editor-based ToC plugins (should coexist with them)
- Generating ToC for non-docz markdown files outside the docs directory
- Adding ToC to README index files (those have the auto-generated table)
- Nested/collapsible ToC (keep it simple markdown links)

## Background

Longer docz documents (RFCs, design docs, implementation plans) benefit from a
table of contents for navigation. Currently this is handled by editor plugins
like the lazyvim markdown ToC plugin, which generates a ToC between
`<!--toc:start-->` and `<!--toc:end-->` markers.

The limitation is that these ToC entries only update when the file is open in
the editor. When documents are created or modified programmatically (e.g., via
Claude Code skills or `docz create`), the ToC becomes stale or is missing
entirely.

By integrating ToC generation into `docz update`, we ensure ToCs stay current
regardless of how the document was edited.

### Prior Art

- **lazyvim markdown-toc plugin**: Uses `<!--toc:start-->` / `<!--toc:end-->`
  markers, generates linked headings from `##` through `######`
- **doctoc**: Node.js CLI that generates ToC with similar marker-based approach
- **markdown-toc (Go)**: Various Go implementations that parse headings and
  generate linked lists

## Detailed Design

### Marker Format

Use the same markers as the lazyvim plugin for maximum compatibility:

```markdown
<!--toc:start-->

- [Section One](#section-one)
- [Section Two](#section-two)
  - [Subsection](#subsection)
  <!--toc:end-->
```

This means:

- Documents edited in lazyvim will have their ToC updated by either the plugin
  or `docz update` — both produce the same output
- `docz create` templates can include the markers, so new documents get ToC
  support out of the box
- The markers are HTML comments, so they're invisible in rendered markdown

### ToC Generation Algorithm

1. Read the document content
2. Find `<!--toc:start-->` and `<!--toc:end-->` markers
3. If no markers are found, skip the document (no error)
4. Parse all markdown headings (`##` through `######`) that appear **after** the
   ToC end marker — headings before the markers and the document's H1 (`#`) are
   excluded
5. For each heading:
   - Extract the heading text (strip any inline markdown like bold/code)
   - Generate a GitHub-compatible anchor slug
   - Determine indent level based on heading depth relative to the minimum
     heading level found
6. Build the ToC as an indented markdown list with links
7. Replace the content between the markers with the new ToC

### Anchor Slug Generation

Follow GitHub's heading anchor algorithm:

1. Lowercase the heading text
2. Strip any characters that aren't letters, numbers, spaces, or hyphens
3. Replace spaces with hyphens
4. Collapse multiple hyphens

Examples:

- `## Problem Statement` → `#problem-statement`
- `## API / Interface Changes` → `#api--interface-changes`
- `### Phase 1: Setup` → `#phase-1-setup`

### Heading Depth and Indentation

The ToC uses relative indentation based on the shallowest heading level found.
If a document only uses `##` and `###`, the `##` entries are top-level (no
indent) and `###` entries are indented one level.

```markdown
<!--toc:start-->

- [Problem Statement](#problem-statement)
- [Proposed Solution](#proposed-solution)
- [Design](#design)
  - [Phase 1: Setup](#phase-1-setup)
  - [Phase 2: Migration](#phase-2-migration)
- [References](#references)
<!--toc:end-->
```

Indent is 2 spaces per level (matching common markdown list conventions).

### Integration with `docz update`

The `updateType()` function in `cmd/update.go` currently:

1. Scans the type directory for documents
2. Generates the README index table
3. Updates the README between DOCZ markers

We add a step between 1 and 2:

1. Scans the type directory for documents
2. **For each document, update its ToC if markers are present**
3. Generates the README index table
4. Updates the README between DOCZ markers

This ensures ToC is always current before the index is regenerated. The ToC
update step is independent per document and can fail gracefully (log a warning,
continue with next document).

### Integration with `docz create`

Update the embedded templates to include ToC markers after the metadata header
block. Placement should be between the status/author/date block and the first
`##` section heading:

```markdown
# RFC 0001: Title

**Status:** Draft **Author:** Name **Date:** 2026-01-01

<!--toc:start-->
<!--toc:end-->

## Summary
```

On initial creation, the markers will be empty (no ToC content). The first
`docz update` or editor-based ToC generation will populate them.

### Package Structure

Add a new file `internal/toc/toc.go` with the core logic:

```go
package toc

// Markers used to delimit the ToC region.
const (
    BeginMarker = "<!--toc:start-->"
    EndMarker   = "<!--toc:end-->"
)

// Heading represents a parsed markdown heading.
type Heading struct {
    Level int    // 2-6 (H1 is excluded)
    Text  string // heading text with inline markdown stripped
    Slug  string // GitHub-compatible anchor
}

// ParseHeadings extracts headings from markdown content.
// Only headings after the ToC end marker are included.
// H1 headings are excluded.
func ParseHeadings(content string) []Heading

// GenerateToC builds a markdown table of contents from headings.
func GenerateToC(headings []Heading) string

// Slugify converts heading text to a GitHub-compatible anchor.
func Slugify(text string) string

// UpdateToC replaces the content between ToC markers in a document.
// Returns the updated content and true if markers were found.
// If no markers are found, returns the original content and false.
func UpdateToC(content string) (string, bool)
```

### Configuration

Add a `toc` section to `.docz.yaml`:

```yaml
toc:
  enabled: true # generate ToC during docz update (default: true)
  min_headings: 3 # minimum headings to generate a ToC (default: 3)
```

The `min_headings` setting prevents generating a ToC for very short documents
where a table of contents adds no value. Documents with fewer headings than this
threshold will have their ToC markers left empty.

Add to the Config struct:

```go
type ToCConfig struct {
    Enabled     bool `mapstructure:"enabled"     yaml:"enabled"`
    MinHeadings int  `mapstructure:"min_headings" yaml:"min_headings"`
}
```

## API / Interface Changes

### CLI Changes

No new commands or flags. The ToC generation is integrated into existing
commands:

- `docz update [type]` — now also updates ToC in documents
- `docz update --dry-run` — shows ToC changes that would be made
- `docz create <type> <title>` — new documents include ToC markers in templates

### Config Changes

New `toc` section in `.docz.yaml`:

```yaml
toc:
  enabled: true
  min_headings: 3
```

### Template Changes

All six document templates gain `<!--toc:start-->` / `<!--toc:end-->` markers
placed after the metadata header block. Existing documents without markers are
unaffected — ToC generation is opt-in via the markers.

## Data Model

No data model changes. ToC is a pure text transformation on existing document
files.

## Testing Strategy

- **Unit tests for `internal/toc/`**:
  - `ParseHeadings`: various heading levels, inline markdown, headings
    before/after markers
  - `Slugify`: special characters, unicode, duplicate anchors
  - `GenerateToC`: indentation levels, empty headings, min_headings threshold
  - `UpdateToC`: marker splicing, missing markers, empty documents
- **Golden file tests**: representative documents with expected ToC output
- **Integration tests in `cmd/`**: `docz update` updates ToC + index together,
  `docz create` produces documents with markers
- **Edge cases**: document with no headings, document with only H1, markers but
  empty content between them, markers in code blocks (should be ignored)

## Migration / Rollout Plan

1. **Existing documents** are unaffected — no ToC markers means no ToC
   generation. Users opt-in by adding markers to existing documents manually or
   re-creating from updated templates.
2. **New documents** created after this change will include ToC markers by
   default via updated templates.
3. **`docz init --force`** will not retroactively add markers to existing
   documents (it only updates README index files).
4. Users can disable ToC generation entirely via `toc.enabled: false` in
   `.docz.yaml`.

## Decisions

1. **ToC scoping follows `docz update` scoping.** `docz update rfc` only
   updates ToC in `docs/rfc/` documents. Same directory-scoped behavior as
   README index updates.

2. **Headings inside fenced code blocks are excluded.** The parser must track
   whether it is inside a ` ``` ` block and skip any headings found there.
   This prevents false positives from markdown examples in code blocks.

3. **Duplicate heading slugs get `-1`, `-2` suffixes.** Matches GitHub's
   anchor behavior so ToC links work correctly in GitHub rendered views.

4. **No standalone `docz toc` command.** ToC generation runs as part of
   `docz update`. A standalone command can be added later if needed.

5. **Markers are placed after the metadata block, before the first `##`.**
   The `<!--toc:start-->` / `<!--toc:end-->` markers go after the
   `**Status:**`/`**Author:**`/`**Date:**` lines and before the first section
   heading.

## References

- [lazyvim markdown-toc plugin](https://github.com/hedyhli/markdown-toc.nvim) —
  editor ToC generation using same markers
- DESIGN-0001: docz CLI Tool — original CLI design
- `internal/index/index.go` — existing marker-based splicing pattern
  (`spliceMarkers`)
