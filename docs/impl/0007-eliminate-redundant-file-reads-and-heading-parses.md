---
id: IMPL-0007
title: "Eliminate Redundant File Reads and Heading Parses"
status: Draft
author: Donald Gifford
created: 2026-05-15
---
<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL 0007: Eliminate Redundant File Reads and Heading Parses

**Status:** Draft
**Author:** Donald Gifford
**Date:** 2026-05-15

<!--toc:start-->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: Baseline benchmarks](#phase-1-baseline-benchmarks)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2: Cache bytes on DocEntry](#phase-2-cache-bytes-on-docentry)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 3: Refactor updateToCs to use cached bytes](#phase-3-refactor-updatetocs-to-use-cached-bytes)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 4: Change UpdateToC API to return []Heading](#phase-4-change-updatetoc-api-to-return-heading)
    - [Tasks](#tasks-3)
    - [Success Criteria](#success-criteria-3)
  - [Phase 5: Verify and ship](#phase-5-verify-and-ship)
    - [Tasks](#tasks-4)
    - [Success Criteria](#success-criteria-4)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Decisions](#decisions)
- [Dependencies](#dependencies)
- [References](#references)
<!--toc:end-->

## Objective

Fix the three "must-fix" performance findings from INV-0002 Wave 3:

1. During `docz update`, each document file is read twice (once for
   frontmatter scan, once for ToC update).
2. `toc.UpdateToC` parses headings internally then discards them, forcing
   the dry-run path to call `ParseHeadings` a second time.
3. The `UpdateToC` API forces callers to re-parse to get heading metadata.

The fix is architectural at the API boundary, not a micro-optimization.
Effect: halves the file-read count and parse work on the `update` hot path.

**Implements:** INV-0002 (Wave 3 — Performance worth fixing)

## Scope

### In Scope

- Cache document bytes on `index.DocEntry` so callers don't re-read (F41)
- Change `toc.UpdateToC` to return `[]Heading` (F42)
- Update `cmd/update.go` to use the new APIs and stop re-reading files
- Add baseline + post-change benchmarks

### Out of Scope

- Parallelizing the update loop (rejected as premature in INV-0002)
- Optimizing `Slugify`, `strings.Split`, or other "do-not-touch" items
- Moving `updateToCs` into `internal/toc` (that's IMPL-0008)

## Implementation Phases

---

### Phase 1: Baseline benchmarks

Before changing anything, measure the current cost so we can prove the
change is a win and prevent regressions.

#### Tasks

- [x] Add `BenchmarkScanDocuments` in `internal/index/index_test.go`
      (100 / 500 / 1000 generated docs with ~2KB bodies)
- [x] Add `BenchmarkUpdateToC` in `internal/toc/toc_test.go`
      (10 / 50 / 200 headings with realistic body text between them)
- [x] Add `BenchmarkCmdUpdate` in `cmd/update_test.go` that runs
      `updateType("rfc")` against a synthesized repo of 100 docs each
      carrying ToC markers and three H2 sections
- [x] Record baseline numbers in this doc (committed inline below)
- [x] Confirm the benchmarks are deterministic — ran each 3× and
      confirmed variance is under 5% for the index and toc benchmarks;
      `BenchmarkCmdUpdate/100` was 7% (end-to-end disk + render +
      write churn, accepted as the headline number)

Baseline numbers (Apple M5 Max, Go 1.25.7, darwin/arm64, medians of 3 runs):

```
BenchmarkScanDocuments/100-18   1539236 ns/op   1324656 B/op   10724 allocs/op
BenchmarkScanDocuments/500-18   7865092 ns/op   6591693 B/op   53537 allocs/op
BenchmarkScanDocuments/1000-18 16143180 ns/op  13181361 B/op  107053 allocs/op
BenchmarkUpdateToC/10-18           7950 ns/op      8764 B/op     200 allocs/op
BenchmarkUpdateToC/50-18          40365 ns/op     43254 B/op     929 allocs/op
BenchmarkUpdateToC/200-18        164569 ns/op    177823 B/op    3639 allocs/op
BenchmarkCmdUpdate/100-18       6575343 ns/op   1697682 B/op   18861 allocs/op
```

#### Success Criteria

- All three benchmarks compile and run
- Baseline numbers recorded in the doc for future comparison
- Benchmarks ignored by `go test ./...` (default — no `-bench` flag)

---

### Phase 2: Cache bytes on `DocEntry`

Capture the file bytes during `ScanDocuments` and expose them so callers
that need the file content don't have to re-read.

#### Tasks

- [ ] Add `Content []byte` to `index.DocEntry` (after the existing
      `Filename` field)
- [ ] In `index.ScanDocuments`, populate `Content` from the bytes already
      read for `ParseFrontmatter` (no extra read)
- [ ] Decide whether to keep `Content` always-populated, or add a
      `ScanOptions` parameter (see Decisions §1)
- [ ] Update `index.GenerateTable` — it doesn't need `Content`, so verify
      no caller relies on `DocEntry` being lightweight
- [ ] Document the memory implication in `DocEntry`'s doc comment

#### Success Criteria

- `DocEntry.Content` holds the raw file bytes after `ScanDocuments`
- No existing test breaks; golden files unchanged
- Memory increase per `DocEntry`: roughly the document's size in bytes
  (acceptable at CLI scale — a repo with 1000×10KB docs uses ~10MB)

---

### Phase 3: Refactor `updateToCs` to use cached bytes

Stop re-reading files in `cmd/update.go:updateToCs`. The bytes are already
on `DocEntry`.

#### Tasks

- [ ] In `cmd/update.go:updateToCs`, replace `os.ReadFile(docPath)` with
      `doc.Content`
- [ ] Verify the warning-on-read-error path is now dead code; remove it
- [ ] Confirm the `os.WriteFile` path still functions (only the read is
      eliminated, not the write)
- [ ] Update tests in `cmd/update_test.go` to confirm files are no longer
      stat'd twice (use a counting `fs.FS` shim or just verify behavior
      unchanged)

#### Success Criteria

- A repo with 1000 docs runs `docz update` with 1000 file reads, not 2000
- `BenchmarkCmdUpdate/100` shows measurable improvement (target: ≥30% faster
  wall clock, ≥50% fewer reads)
- All existing `cmd/update_test.go` tests pass

---

### Phase 4: Change `UpdateToC` API to return `[]Heading`

Make the heading metadata available to callers without forcing a second
parse.

#### Tasks

- [ ] Change `toc.UpdateToC(content string, minHeadings int) (string, bool)`
      to `(content string, minHeadings int) (updated string, headings []Heading, found bool)`
- [ ] Update `cmd/update.go:updateToCs` to use the returned `headings`
      slice instead of calling `toc.ParseHeadings` a second time in the
      dry-run branch
- [ ] Update any other callers (audit with `grep -rn 'UpdateToC' .`)
- [ ] Update test cases in `internal/toc/toc_test.go` for the new signature
- [ ] Verify `BenchmarkUpdateToC` numbers do not regress (the function
      now allocates a `[]Heading` for the caller; should be cheap)

#### Success Criteria

- `grep -rn 'ParseHeadings' .` returns exactly two call sites in
  production code: inside `UpdateToC` itself and any caller that
  explicitly wants headings without updating
- Dry-run `docz update --dry-run` no longer double-parses
- `BenchmarkUpdateToC` not slower than baseline

---

### Phase 5: Verify and ship

#### Tasks

- [ ] Re-run all three benchmarks; record post-change numbers in this doc
- [ ] Confirm improvement targets met
- [ ] Run `make ci`
- [ ] Smoke test: `docz update --dry-run` against this repo
- [ ] Smoke test: `docz update` against this repo; verify generated files
      byte-identical to pre-change
- [ ] Open PR with `dont-release` label
- [ ] Update INV-0002 status to reflect Wave 3 completion

Post-change numbers (fill in after Phase 5):

```
BenchmarkScanDocuments/100   <ns/op>  (delta: ...)
BenchmarkScanDocuments/500   ...
BenchmarkScanDocuments/1000  ...
BenchmarkUpdateToC/10        ...
BenchmarkUpdateToC/50        ...
BenchmarkUpdateToC/200       ...
BenchmarkCmdUpdate/100       ...
```

#### Success Criteria

- `BenchmarkCmdUpdate/100` ≥30% faster than baseline
- No golden-file regression
- No memory leak under repeated invocation (sanity check)

---

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/index/index.go` | Modify | Add `DocEntry.Content`; populate in `ScanDocuments` |
| `internal/index/index_test.go` | Modify | Add benchmark; assert `Content` populated |
| `internal/toc/toc.go` | Modify | Change `UpdateToC` signature to return headings |
| `internal/toc/toc_test.go` | Modify | Update test cases; add benchmark |
| `cmd/update.go` | Modify | Use cached `Content`; consume returned `headings` |
| `cmd/update_test.go` | Modify | Add `BenchmarkCmdUpdate`; verify no double-read |

## Testing Plan

- [ ] Benchmarks for `ScanDocuments`, `UpdateToC`, `runUpdate`
- [ ] Correctness regression: golden files unchanged
- [ ] Edge cases: empty file, file with frontmatter only and no ToC
      markers, file with ToC markers but no headings
- [ ] Memory check: scan 1000 large files, verify reasonable allocation
      ceiling

## Decisions

Resolved during INV-0002 planning review.

1. **`DocEntry.Content`:** always populated. ~10MB ceiling at realistic
   CLI scale (1000 docs × ~10KB) is negligible.
2. **`fs.FS` parameter:** defer to IMPL-0008, which restructures the
   `index` / `document` packages and is the natural point to introduce
   the abstraction.
3. **Benchmark scale:** 100 / 500 / 1000 sweep. Report the 100-doc
   number as the headline; the larger sizes catch regressions in the
   curve.
4. **Memory implication on `DocEntry.Content`:** no action needed for
   a one-shot CLI. Document the assumption in `DocEntry`'s doc comment
   so a future library consumer understands it.
5. **`UpdateToC` return shape:** struct
   `UpdateResult{Updated string; Headings []Heading; Found bool}`.
   Easier to extend without breaking signature.

## Dependencies

- Builds on IMPL-0006 (assumes Wave 2 helpers exist; specifically
  `EnabledTypes` simplifies the test setup)
- Independent of IMPL-0008; can ship before or after

## References

- INV-0002 — Wave 3, findings F41, F42
- Performance review notes — `cmd/update.go:131` (double parse),
  `cmd/update.go:112` (double read)
