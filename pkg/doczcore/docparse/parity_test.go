package docparse_test

// Parity tests pinning docparse's walker output to internal/toc's
// (IMPL-0014 Phase 2 success criteria): slugs and heading sets must be
// byte-identical across toc's existing corpus so Phase 4 can retire
// toc's walker with a zero-churn delegation. sliceAfterEndMarker below
// is that delegation recipe: toc.ParseHeadings skips everything up to
// and including the <!--toc:end--> line, so toc will slice first and
// then call docparse.Headings.
//
// This file retires with internal/toc's ParseHeadings/AnchorSlug in
// Phase 4 — the delegation itself then guarantees parity.

import (
	"strings"
	"testing"

	"github.com/donaldgifford/docz/internal/toc"
	"github.com/donaldgifford/docz/pkg/doczcore/docparse"
)

// sliceAfterEndMarker returns the lines of content after the first
// <!--toc:end--> marker line, or all of content when no marker exists —
// mirroring toc.ParseHeadings' start-position rule.
func sliceAfterEndMarker(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == toc.EndMarker {
			return strings.Join(lines[i+1:], "\n")
		}
	}
	return content
}

func TestAnchorSlugParity(t *testing.T) {
	t.Parallel()
	// toc's AnchorSlug corpus plus edge cases; every input must slug
	// identically through both implementations.
	inputs := []string{
		"Problem Statement",
		"Phase 1: Setup",
		"API / Interface Changes",
		"What's New?",
		"Step 2 Details",
		"- test -",
		"too  many   spaces",
		"",
		"Héllo Wörld",
		"!@#$%",
		"Config (default)",
		"UPPER case MiXeD",
		"trailing space ",
		"tab\there",
		"emoji 🎉 heading",
		"--already-hyphenated--",
	}

	for _, in := range inputs {
		if got, want := docparse.AnchorSlug(in), toc.AnchorSlug(in); got != want {
			t.Errorf("AnchorSlug(%q): docparse %q != toc %q", in, got, want)
		}
	}
}

func TestHeadingsParityWithToC(t *testing.T) {
	t.Parallel()

	// The representative document from toc's golden test: mixed levels,
	// inline formatting, a fenced block, and duplicate headings.
	golden := `---
id: RFC-0001
title: "API Rate Limiting"
status: Draft
author: Test
created: 2026-01-01
---
<!-- markdownlint-disable-file MD025 MD041 -->

# RFC 0001: API Rate Limiting

**Status:** Draft
**Author:** Test
**Date:** 2026-01-01

<!--toc:start-->
<!--toc:end-->

## Summary

Brief summary of the proposal.

## Problem Statement

What problem does this address?

## **Proposed** Solution

High-level description with **bold** in heading.

### Phase 1: Setup

First phase details.

### Phase 2: ` + "`Migration`" + `

Second phase with inline code in heading.

## Design

### [API](http://example.com) Endpoints

Link in heading.

### Error Handling

How errors are handled.

` + "```go" + `
## This Is Not A Heading
func main() {}
` + "```" + `

## Alternatives Considered

### Overview

First overview section.

## References

### Overview

Second overview section (duplicate heading).
`

	corpus := map[string]string{
		"golden representative doc": golden,
		"levels H2-H6": toc.EndMarker + "\n" +
			"## Level 2\n### Level 3\n#### Level 4\n##### Level 5\n###### Level 6\n",
		"H1 skipped":         toc.EndMarker + "\n# Title\n## Section\n",
		"pre-marker heading": "## Before Marker\n<!--toc:start-->\n" + toc.EndMarker + "\n## After Marker\n",
		"fenced code": toc.EndMarker + "\n" +
			"## Real Heading\n```\n## Fake Heading In Code\n```\n## Another Real Heading\n",
		"fenced with language": toc.EndMarker + "\n## Before\n```go\n## Not A Heading\n```\n## After\n",
		"inline markdown": toc.EndMarker + "\n" +
			"## **Bold** Heading\n## `Code` Heading\n## [Link](http://example.com) Heading\n",
		"duplicate slugs": toc.EndMarker + "\n## Overview\n## Details\n## Overview\n## Overview\n",
		"no end marker":   "## First\n## Second\n",
		"empty":           "",
	}

	for name, content := range corpus {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			want := toc.ParseHeadings(content)
			got := docparse.Headings([]byte(sliceAfterEndMarker(content)))

			if len(got) != len(want) {
				t.Fatalf("heading count: docparse %d != toc %d", len(got), len(want))
			}
			for i := range want {
				if got[i].Level != want[i].Level {
					t.Errorf(
						"[%d].Level: docparse %d != toc %d",
						i, got[i].Level, want[i].Level,
					)
				}
				if got[i].Text != want[i].Text {
					t.Errorf(
						"[%d].Text: docparse %q != toc %q",
						i, got[i].Text, want[i].Text,
					)
				}
				if got[i].Slug != want[i].Slug {
					t.Errorf(
						"[%d].Slug: docparse %q != toc %q",
						i, got[i].Slug, want[i].Slug,
					)
				}
			}
		})
	}
}
