package toc

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "update golden files")

func TestGoldenToCOutput(t *testing.T) {
	// A representative document with mixed heading levels, code blocks,
	// inline formatting, and duplicate headings.
	input := `---
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

	got, found := UpdateToC(input, 1)
	if !found {
		t.Fatal("UpdateToC() found = false, want true")
	}

	goldenPath := filepath.Join("..", "..", "testdata", "golden", "toc", "basic.md")

	if *update {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Log("Updated golden file:", goldenPath)
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf(
			"reading golden file %s: %v\nRun with -update to create it",
			goldenPath, err,
		)
	}

	if got != string(want) {
		t.Errorf(
			"ToC output differs from golden file %s\nGot:\n%s\nRun with -update to update",
			goldenPath, got,
		)
	}
}
