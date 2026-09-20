---
id: INV-0010
title: "IMPL plan parse and write-back API for doczcore (issue 100)"
status: Concluded
author: Donald Gifford
created: 2026-09-13
---

<!-- markdownlint-disable-file MD025 MD041 -->

# INV-0010: IMPL plan parse and write-back API for doczcore (issue 100)

<!--toc:start-->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
  - [Observation 1: the request reverses a recorded decision, and the reversal is justified](#observation-1-the-request-reverses-a-recorded-decision-and-the-reversal-is-justified)
  - [Observation 2: what already exists, and what doczwork already solved](#observation-2-what-already-exists-and-what-doczwork-already-solved)
  - [Observation 3: what the corpus says the grammar must tolerate](#observation-3-what-the-corpus-says-the-grammar-must-tolerate)
  - [Observation 4: the hand-written markers that "must parse" do not match the proposed spelling](#observation-4-the-hand-written-markers-that-must-parse-do-not-match-the-proposed-spelling)
  - [Observation 5: "starts with a backticked command" misclassifies about a tenth of real criteria](#observation-5-starts-with-a-backticked-command-misclassifies-about-a-tenth-of-real-criteria)
  - [Observation 6: write-back needs more than four splices](#observation-6-write-back-needs-more-than-four-splices)
  - [Observation 7: DiffPlans needs an identity for tasks, and positional IDs are not one](#observation-7-diffplans-needs-an-identity-for-tasks-and-positional-ids-are-not-one)
  - [Observation 8: there is no CLI task surface to agree with](#observation-8-there-is-no-cli-task-surface-to-agree-with)
  - [Observation 9: sizing](#observation-9-sizing)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
- [Open Questions](#open-questions)
  - [1. Where does the model live, and what are the names?](#1-where-does-the-model-live-and-what-are-the-names)
  - [2. How is the reversal of IMPL-0014 Decision 3(d) recorded?](#2-how-is-the-reversal-of-impl-0014-decision-3d-recorded)
  - [3. What is a phase, and what is a task?](#3-what-is-a-phase-and-what-is-a-task)
  - [4. How are continuation lines and verify: handled?](#4-how-are-continuation-lines-and-verify-handled)
  - [5. Marker grammar, idempotency, and clearing a deferral](#5-marker-grammar-idempotency-and-clearing-a-deferral)
  - [6. Criteria classification](#6-criteria-classification)
  - [7. DiffPlans matching and shape](#7-diffplans-matching-and-shape)
  - [8. CLI surface in the first release](#8-cli-surface-in-the-first-release)
  - [9. Release sequencing against IMPL-0017](#9-release-sequencing-against-impl-0017)
- [References](#references)
<!--toc:end-->

## Question

Issue #100 asks `pkg/doczcore` for an IMPL plan model — `ParseImpl`,
`SetTaskState`, `SetDeferred`, `SetSkipped`, a byte-level `SetStatus`, and
`DiffPlans` — all pure over a `[]byte` document. What does that require of
docz, does it fit the frozen v1 surface and ADR-0001's facts-versus-
interpretation rule, which parts already exist, what does the real IMPL
corpus say the grammar has to tolerate, and how big is the work?

## Hypothesis

Feasible as one additive `minor` release, with three costs the issue does not
mention: it reverses IMPL-0014 Decision 3(d), which deliberately kept the
plan/phase model *out* of docz; the grammar has enough unstated edge cases
(wrapped task text, checkboxes outside phases, hand-written markers that do
not match the proposed spelling, a criteria heuristic that misfires on symbol
names) that the design has to pin each one; and the acceptance bullet that
compares CLI task IDs with the library's has no CLI to compare against.
Roughly half of the read side already exists in `docparse` and in
sdk-booty-sh's `doczwork`, and the write side is an extension of `docwrite`'s
byte-preserving splices.

## Context

Issue #100 (opened 2026-09-13) is the docz half of **tempy** — a Temporal-
orchestrated worker that executes docz IMPL docs task by task
(`~/code/tempy`, DESIGN-0001 "Temporal-orchestrated IMPL loop execution",
IMPL-0001). Its design says `doczcore` is "the only code that reads or writes
IMPL structure", it fetches the IMPL file at a SHA through the GitHub API with
no checkout, re-parses after every agent iteration, and rejects any commit
whose `DiffPlans` shows a change other than the assigned task's checkbox.
tempy IMPL-0001 has the task "In the docz repository, donaldgifford/docz#100
… add `ParseImpl` …" followed by "Pin that docz release in `go.mod`", so this
work is on tempy's critical path. It outranks IMPL-0017 (the `updated:`
field, not started).

There is already a second consumer with the same problem: sdk-booty-sh's
`pkg/loop/doczwork` (pinned to docz v1.0.0) carries its own 260-line
phase/task model over `docparse` — exactly the "each worker carries its own
parser" duplication the issue wants to end.

The history matters. ADR-0001 Decision 2 and IMPL-0014 Decision 3(d) dropped
`ParsePlan` from `docparse` on 2026-07-03: "docz parses *facts* … the
plan/phase *interpretation* is loop-harness policy and moves to sdk-booty-sh's
`doczwork`." The memory of that review adds the principle "a public API docz
itself never calls, encoding policy docz has no opinion on, is a smell."

**Triggered by:** issue #100; tempy DESIGN-0001 / IMPL-0001; ADR-0001;
IMPL-0014 Decision 3.

## Approach

1. Read the issue's API and conventions against tempy DESIGN-0001 (the
   activities that call each function, the phase gate, the `verify:` /
   deferred / skipped conventions, tempy OQ-6) and tempy IMPL-0001.
2. Inventory what docz already has: `document.ParseFrontmatter`,
   `docparse.Headings` / `TaskItems` / `Title`, `docwrite.SetStatus` /
   `CheckTask` and their unexported byte-level cores, the golden and
   write-through tests.
3. Read sdk-booty-sh `doczwork` (`model.go`, `source.go`, fixtures) as the
   existing consumer-side implementation of the same semantics.
4. Survey the real corpus the acceptance tests must run on: the 16 IMPL docs
   in this repo, docz-api's IMPL docs, sdk-booty-sh's — phase heading shapes,
   task nesting and wrapping, checkboxes outside phases, criteria shapes, and
   every hand-written `verify:` / deferred / skipped marker.
5. Check the CLI for any task surface the acceptance bullet could mean.
6. Size the work against IMPL-0015 / IMPL-0016 (four phases each).

## Environment

| Component | Version / Value |
| --------- | --------------- |
| docz | `main` at `dd6d853`, latest release v1.2.2; public core `pkg/doczcore/{config,document,docparse,docwrite,toc}` frozen since v1.0.0 |
| sdk-booty-sh `doczwork` | `~/code/sdk-booty-sh/pkg/loop/doczwork`, pins docz **v1.0.0** |
| tempy | `~/code/tempy`, DESIGN-0001 and IMPL-0001 dated 2026-09-13, no Go code against docz yet |
| corpus | 16 docz IMPL docs (`docs/impl/0001`–`0017`, no 0010), 10 docz-api and 3 sdk-booty-sh IMPL docs, `doczwork` clean/messy fixtures |

## Findings

### Observation 1: the request reverses a recorded decision, and the reversal is justified

IMPL-0014 Decision 3(d) dropped `ParsePlan` because every option was "docz
picking a workflow semantic it has no use for itself — the ambiguity was the
signal that the semantics belong downstream." Two things changed:

- **Demand is now plural.** `doczwork` (sdk-booty-sh) and tempy both need the
  same phase/task/criteria model and the same write-backs. ADR-0001
  Decision 1's promotion rule is "public where demand exists"; two consumers
  re-implementing one grammar is the promotion drip the ADR exists to end.
- **docz owns the grammar.** The IMPL template (`internal/template/templates/
  impl.md`) defines `### Phase N:`, `#### Tasks`, `#### Success Criteria`,
  and the `## Testing Plan` checkboxes. A consumer can only guess at the
  template's intent; docz can state it.

What does *not* change is the split inside docz: `docparse` stays the fact
extractor (headings, checkbox items, byte-accurate lines) and is frozen.
The plan model is interpretation and belongs in a new package layered on
top, not in `docparse`. The "API docz never calls" smell is answered by
giving the CLI a task surface (Observation 8) rather than by leaving the
model consumer-only.

### Observation 2: what already exists, and what `doczwork` already solved

| Need in #100 | Exists today | Gap |
| ------------ | ------------ | --- |
| frontmatter id/title/status from bytes | `document.ParseFrontmatter([]byte)` | none; note `Frontmatter.Status` is typed `config.Status`, the issue's `ImplPlan.Status` is `string` |
| headings with levels and lines | `docparse.Headings` (H2–H6, inline markdown stripped, fence-aware) | none |
| checkbox items with lines | `docparse.TaskItems` (`Text`, `Checked`, `Indent`, byte-accurate `Line`) | **`Text` is the first line only** — continuation lines are not folded (Observation 3) |
| phase spans, `#### Tasks` / `#### Success Criteria` sub-spans, top-level task scoping, wrapped-criteria folding, phase token with ordinal fallback, duplicate-phase refusal | `doczwork/model.go` `buildModel` + clean/messy fixtures | consumer-side; lift and re-fixture |
| status write-back from bytes | `docwrite.SetStatus(path, …)` — its core `findStatusValue(content []byte)` is already byte-level | the exported function reads and writes a path |
| checkbox flip from bytes | `docwrite.CheckTask(path, line)` single-byte splice validated by `docparse.TaskItems` | path-based; **check only** — refuses an already-checked item and cannot uncheck |
| deferred / skipped markers, `verify:` lines, criteria classification, `DiffPlans` | nothing | new |

`doczwork` also shows the semantics a real harness needed beyond the issue:
`MarkDone` relocates a task by exact text when lines shifted under it
(`TaskMovedError` when the text is gone or ambiguous), and `DuplicatePhaseError`
refuses a doc whose phase tokens collide. Both belong in the docz version.

### Observation 3: what the corpus says the grammar must tolerate

Surveyed with `grep`/`awk` over the 16 IMPL docs in this repo:

| Fact | Evidence | Consequence for the parser |
| ---- | -------- | -------------------------- |
| Every phase is `### Phase N: Title`, N numeric and contiguous (1–11) | 118 phase headings, all `Phase <digits>:` | positional phase segment and heading token coincide today; `doczwork` also tolerates `Phase A:` / `Phase SH:` / `Phase 2B:` and bold headings |
| `### Phase N` **without** a colon appears under `## File Changes` | 8 headings in IMPL-0001 / IMPL-0002 | the colon is the discriminator; "every `###` is a phase" (IMPL-0014 option a, `doczwork`'s rule) would also make `### In Scope` / `### Out of Scope` phases 1–2 and shift every positional ID |
| `#### Success Criteria` is optional | IMPL-0007 phases 2 and 5 have tasks but no criteria | `Criteria` may be empty |
| Nested checkboxes | 0 in this repo; 1 in `doczwork`'s messy fixture | `Indent == 0` scoping (as `doczwork`) is right; nested items are never tasks |
| **Task text wraps onto continuation lines** | IMPL-0009: 68 tasks, IMPL-0014: 35, IMPL-0016: 41, IMPL-0017: 48 of 56 | `TaskItem.Text` alone truncates most tasks in recent docs; the parser must fold continuation lines, and the `verify:` / marker continuation rules interact with that folding |
| Checkboxes outside any phase | template `## Testing Plan` has 3; IMPL-0013 lists OQs as checkboxes | a level-2 heading must end the phase span so these never get IDs (`doczwork`'s `spanEnd` does this) |
| Criteria that start with a backtick | ~200 lines; leading tokens `docz` 59, `make` 42, `go` 25, `grep` 12 … | see Observation 5 |

### Observation 4: the hand-written markers that "must parse" do not match the proposed spelling

The issue says hand-written markers in existing docs must parse. The corpus
has exactly these:

- **`verify:`** — none in docz; docz-api IMPL-0004 has four, all spelled
  **`Verify:`**, not necessarily the first continuation line, and followed
  by prose:

  ```markdown
  - [x] **2.1 Unify helpers under `docz-api.*`.** In `charts/docz-api/`, rename
        every `repo-guardian.` template define/include to `docz-api.`:
        `grep -rl 'repo-guardian\.' charts/docz-api/templates | xargs sed -i 's/repo-guardian\./docz-api./g'`.
        Verify: `grep -r 'repo-guardian\.' charts/docz-api/templates` prints
        nothing.
  ```

  Note the task body itself contains a backticked command before the
  `Verify:` line, so "the command is the backtick span on the verify line"
  is the only workable rule — not "the first command in the task".
- **deferred** — none on a task line in docz. docz-api IMPL-0006 has one,
  and it is a *prefix*, bold, ASCII hyphen, different phrase:

  ```markdown
  - [ ] **deferred - blocked, no cluster available** — redeploy the new image +
  ```

  with prose lower down saying "marked `deferred - human required` above".
  The issue's canonical form is a trailing `deferred – human required:
  <reason>` (en dash, colon). Either the parser is lenient about position,
  emphasis, dash, and phrase, or that doc is hand-normalised before tempy
  runs it.
- **skipped** (`- [ ] ~~text~~ — skipped: <note>`) — zero occurrences
  anywhere; a new convention with no legacy to honour. Because the line stays
  a `- [ ]` checkbox item, a skipped task keeps its positional ID — a
  property worth keeping.

### Observation 5: "starts with a backticked command" misclassifies about a tenth of real criteria

tempy's phase gate runs every `Criterion.Executable` command and treats exit 0
as pass. In this repo's IMPL docs the backtick-leading criteria are mostly
commands (`docz …`, `make …`, `go …`, `grep …`), but roughly one in ten is a
symbol or path used as the sentence's subject:

```markdown
- `WikiConfig` has fields `MkDocsPath`, `Exclude`, `NavTitles`
- `internal/index/index.go` no longer imports `internal/template`
- `--verbose` shows per-file ToC output
- `BenchmarkCmdUpdate/100` allocations drop by ≥30%
```

Under the issue's rule these become `Executable: true, Command: "WikiConfig"`
and fail at the gate with "command not found" — a visible, recoverable
failure, but noise on every legacy doc. tempy OQ-6 already chose the backtick
rule for phase criteria ("keep it simple for now"), so docz should implement
it as specified and document the caveat; docz itself never executes anything.

### Observation 6: write-back needs more than four splices

- **Bytes in, bytes out is consistent with ADR-0001.** Decision 5 keeps
  path-based helpers "only where a consumer requires in-place mutation" and
  says non-filesystem callers get content back rather than I/O. tempy is
  exactly that caller (GitHub API, "zero data through activity payloads that
  isn't a reference"). The existing path-based `SetStatus` / `CheckTask` stay;
  their byte-level cores become exported.
- **Unchecking is new.** `CheckTask` refuses `[x]`; `SetTaskState(…, Unchecked)`
  needs the reverse splice.
- **The issue's API cannot undo a deferral.** tempy's signal table has
  `unblock(task_id, note)`: "clears the marker, resets the task iteration
  count." Nothing in #100 clears a marker. A `ClearDeferred` (or an
  `Unchecked`-style state) is required for tempy to work at all.
- **Skipping is a multi-line rewrite.** With wrapped tasks the norm
  (Observation 3), `~~text~~` has to span the first line and its continuation
  lines, and the `— skipped: <note>` suffix lands on the last text line,
  before any `verify:` line. "Only the target line(s)" is right, but it is *lines*.
- **Idempotency has two readings.** Same marker, same reason → unchanged
  bytes is uncontroversial. Same marker, different reason → replace or error
  is a decision (Open Question 5).
- **Line-ending policy carries over.** `ErrUnsupportedLineEndings` (LF only,
  DESIGN-0005 Decision 7) applies to every new writer.

### Observation 7: `DiffPlans` needs an identity for tasks, and positional IDs are not one

"Tasks added, removed, reordered" cannot be computed from positional IDs
alone: after an insertion at 2.3, every later task's ID shifts and a
position-only diff reports the tail as "changed". `doczwork` solved the
same problem for relocation by matching on exact task text. The diff should
match phases by token, then tasks within a phase by text (longest common
subsequence), reporting a reworded task as removed + added — which is what
tempy's check 2 wants to reject anyway. Checkbox, deferred, and skipped
changes are then reported per matched task ID; criteria diff by text per
phase. tempy needs one predicate — "the only change is this task's
`Checked`" — so `PlanDiff` should make that cheap to ask.

### Observation 8: there is no CLI task surface to agree with

The acceptance bullet "`docz` (CLI) and `ParseImpl` report the same task IDs
for the same doc" has nothing on the CLI side today: no `cmd/` file imports
`docparse.TaskItems` or `docwrite.CheckTask` — the checkbox primitive shipped
in v1.0.0 for sdk-booty-sh and the CLI never calls it. That is the exact
"public API docz itself never calls" smell from the ADR-0001 review. ADR-0001
Decision 7 makes the CLI feature set the completeness benchmark; a thin
`docz task` command family (list / check / uncheck / defer / undefer / skip)
over the new package makes the bullet testable and gives humans the same
tool the worker uses — the `status set` pattern (IMPL-0011) already shows
the shape: resolve the doc by ID, call the library, stable exit codes.

### Observation 9: sizing

Comparable past work: IMPL-0015 (changelog block + parser, 4 phases, ~26
tasks) and IMPL-0016 (api block + `Title`, 4 phases, ~42 tasks). This is
larger — a parser with a real grammar, five writers, a diff, a CLI family —
and closer to IMPL-0014 Phase 2–3 in shape. A reasonable plan is five
phases: parse (lift `doczwork`'s model, add continuation folding, markers,
criteria) → byte-level writers in `docwrite` + the IMPL-specific writers →
`DiffPlans` → `docz task` CLI → docs, consumer proof, release. Fixtures are
snapshots of real docs (this repo's IMPL-0009/0014/0017 for wrapping,
docz-api IMPL-0004 for `Verify:`, IMPL-0006 for the prefix deferred marker,
`doczwork`'s messy fixture), copied under `testdata/` rather than read from
`docs/` so later edits to living docs cannot break the suite.

## Conclusion

**Answer:** Yes — feasible as one additive `minor` release, with the design
having to settle nine things the issue leaves open. Nothing in the frozen
surface changes: `docparse` stays facts-only, `docwrite` gains byte-level
variants of what it already does, and the plan model lands in a new
subpackage that composes them. The reversal of IMPL-0014 Decision 3(d) is
real but sound under ADR-0001's own promotion rule now that two consumers
need the grammar docz's template defines, and it is answered properly by
giving the CLI a task surface so docz consumes the API it publishes.

The parts the issue underestimates: task text wraps in most recent docs so
the parser must fold continuation lines and the writers must handle
multi-line targets; the only hand-written markers in the fleet do not match
the proposed spelling; the criteria heuristic misfires on about a tenth of
legacy criteria; `DiffPlans` needs text-based matching; tempy's `unblock`
needs a marker remover the issue does not list; and the CLI-agreement
acceptance bullet needs a CLI to exist.

## Recommendation

1. **Write DESIGN-0013 from this investigation** with the decisions below
   locked, including a dated amendment to ADR-0001 Decision 2 recording why
   the plan model now lives in docz.
2. **New subpackage `pkg/doczcore/impl`** composing `document`, `docparse`,
   and `docwrite`; `docparse` untouched. Lift `doczwork`'s span logic as the
   starting point.
3. **Export byte-level cores in `docwrite`** (`SetStatusBytes`,
   `SetTaskStateBytes`) so the path-based helpers and the new package share
   one splice each; the IMPL-specific writers (`SetDeferred`,
   `ClearDeferred`, `SetSkipped`) live in `impl`.
4. **Lenient parse, canonical write** for every marker: accept `Verify:` /
   `verify:`, any dash, prefix or suffix deferred markers; always write the
   issue's canonical trailing forms.
5. **`docz task` CLI family in the same release** so the acceptance bullet
   is a real test and the API has a first-party caller.
6. **Ship as v1.3.0 ahead of IMPL-0017**, one `minor` PR from a `feat/`
   branch, post-tag `dont-release` flips; renumber IMPL-0017's target to
   v1.4.0.
7. **File the consumer issues at design time**: tempy already has its
   IMPL-0001 task; file one in sdk-booty-sh to migrate `doczwork` onto
   `impl` and drop its private model (the duplication the issue is about).

## Open Questions

### 1. Where does the model live, and what are the names?

- a. **New subpackage `pkg/doczcore/impl`, identifiers without the `Impl`
  prefix** — `impl.Parse`, `impl.Plan`, `impl.Phase`, `impl.Task`,
  `impl.Criterion`, `impl.TaskState`, `impl.Diff`, `impl.PlanDiff`. Matches the
  doc type's name (`docz create impl`), avoids `impl.ImplPlan` /
  `impl.ParseImpl` stutter, and `Plan.Status` is typed `config.Status` like
  `document.Frontmatter.Status` (DESIGN-0004 §F). *(recommendation)*
- b. Same package, the issue's identifiers verbatim (`ParseImpl`, `ImplPlan`,
  `DiffPlans`, `Status string`) — matches tempy's design prose today.
- c. Package `implplan` (no stutter, clearer in prose, longer import).
- d. Split across `docparse` (types + `Parse`) and `docwrite` (writers) —
  puts interpretation into the frozen fact package.
- e. Other.

### 2. How is the reversal of IMPL-0014 Decision 3(d) recorded?

- a. **Dated amendment to ADR-0001 Decision 2** (the DESIGN-0007 amendment
  pattern), plus a note in IMPL-0014's Decisions table and in `CLAUDE.md` /
  the API-philosophy memory: facts stay in `docparse`; interpretation that
  docz's own template defines may ship in a sibling package when more than
  one consumer needs it. *(recommendation)*
- b. A new ADR-0002 superseding that clause of ADR-0001.
- c. Other.

### 3. What is a phase, and what is a task?

- a. **A phase is a level-3 heading matching `^Phase\s+([^\s/:]+):`** (inline
  markdown already stripped, so bold headings match), anywhere in the doc;
  its span ends at the next heading of level ≤ 3. Tasks are `Indent == 0`
  checkbox items inside the phase's `#### Tasks` sub-span, or the whole
  phase span when there is no such subheading; criteria come from
  `#### Success Criteria` and may be absent. The phase segment of a task ID
  is the heading token (`1`, `A`, `2B`); a duplicate token is an error.
  `### Phase 1` without a colon (IMPL-0001/0002 File Changes) and
  `### In Scope` are not phases. *(recommendation)*
- b. `doczwork`-literal: every level-3 heading is a phase, token with ordinal
  fallback — hand-written docs with odd headings still parse, but `In Scope`
  becomes phase 1 in every template-generated doc.
- c. Only under `## Implementation Phases`; two-mode contract keyed to an
  English heading.
- d. Other.

### 4. How are continuation lines and `verify:` handled?

- a. **Fold continuations, extract `verify:` case-insensitively.** `Task.Text`
  = first line plus every following line that is more indented than the
  bullet, non-blank, not a nested bullet, joined with single spaces — minus
  the `verify:` line and any marker. A `verify:` / `Verify:` continuation
  line anywhere in the task yields `Task.Verify` = the contents of its first
  backtick span; prose after the span is ignored. docz-api's four hand-written
  lines parse unchanged. *(recommendation)*
- b. Exact lowercase `verify:` as the only recognised form, whole remainder
  must be one backtick span — strict and simple; the docz-api lines do not
  parse.
- c. `Text` stays first-line-only (`docparse` literal) and `verify:` must be
  the first continuation line — cheapest, but truncates most tasks written
  since IMPL-0009.
- d. Other.

### 5. Marker grammar, idempotency, and clearing a deferral

- a. **Lenient parse, canonical write, explicit clear.** Parse `deferred`
  followed by `-` / `–` / `—`, optional emphasis, as a prefix or suffix of the
  task text (folding a wrapped reason); parse `~~…~~` plus `skipped:` after any
  dash. Write only the canonical trailing `deferred – human required:
  <reason>` and `~~text~~ — skipped: <note>`. Same marker with the same
  reason → bytes unchanged; different reason → replaced. Add
  `ClearDeferred(doc, taskID)` for tempy's `unblock`. *(recommendation)*
- b. Canonical spellings only, in both directions; docz-api IMPL-0006 is
  hand-fixed; `unblock` is modelled as `SetDeferred` with an empty reason.
- c. Lenient parse, but a different reason on an already-marked task is an
  error rather than a replacement.
- d. Other.

### 6. Criteria classification

- a. **As specified**: `Executable` when the bullet text starts with a
  backtick span, `Command` = the span; everything else assertive. Document
  the caveat that symbol-subject criteria (about a tenth in this repo)
  classify as executable, and that docz never runs them — the consumer
  gate reports the failure. *(recommendation)*
- b. Stricter: executable only when the span's first token contains no `/`,
  `.`, uppercase, or leading `--` — fewer false positives, heuristics on
  heuristics.
- c. Executable only when the criterion is *entirely* a backtick span (no
  trailing prose) — clean, but "`make ci` passes" stops being executable.
- d. Other.

### 7. `DiffPlans` matching and shape

- a. **Phases by token, tasks by text.** Within a matched phase, tasks are
  aligned by normalised `Text` (LCS); a reworded task is removed + added;
  `PlanDiff` reports `TasksAdded` / `TasksRemoved` / `TasksReordered` as
  task refs, per-matched-ID `Checked` / `Deferred` / `Skipped` changes,
  per-phase criteria changes, plus `Empty()` and `OnlyChecked(taskID)` for
  tempy's check 2. *(recommendation)*
- b. Positional IDs only — cheaper, cannot distinguish an insertion from a
  reword of every later task.
- c. Other.

### 8. CLI surface in the first release

- a. **`docz task list|check|uncheck|defer|undefer|skip <impl-id> [task-id]`**
  as thin wrappers over `impl` with `--format text|json` and the `status set`
  exit-code pattern — makes the CLI/library agreement a test and gives the
  API a first-party caller. *(recommendation)*
- b. `docz task list` only; writers stay library-only until a human needs
  them.
- c. No CLI; drop the acceptance bullet.
- d. Other.

### 9. Release sequencing against IMPL-0017

- a. **#100 ships first as v1.3.0**; IMPL-0017 (`updated:` field) retargets
  to v1.4.0, its title and DESIGN-0012 rollout text edited in the same docs
  PR that lands DESIGN-0013. *(recommendation)*
- b. Both in one v1.3.0 — couples an unstarted feature to tempy's critical
  path.
- c. Other.

## References

- [Issue #100](https://github.com/donaldgifford/docz/issues/100) — the request
- tempy `docs/design/0001-temporal-orchestrated-impl-loop-execution.md`
  (activities table, phase gate, "API / Interface Changes → doczcore", OQ-6)
  and `docs/impl/0001-temporal-orchestrated-impl-loop-worker-mvp.md`
- sdk-booty-sh `pkg/loop/doczwork/{doc.go,model.go,source.go,model_test.go}`
- [ADR-0001](../adr/0001-pkgdoczcore-as-the-single-public-core-cmd-as-a-thin-cli-shell.md)
  Decisions 1, 2, 5, 7
- [IMPL-0014](../impl/0014-v100-the-five-package-pkgdoczcore-public-core.md)
  Decision 3 (`ParsePlan` dropped)
- [DESIGN-0005](../design/0005-status-set-cli-primitive.md) — byte-preservation
  contract; [IMPL-0011](../impl/0011-status-set-cli-primitive.md) — the
  `status set` command pattern
- `pkg/doczcore/docparse/taskitems.go`, `pkg/doczcore/docwrite/{status,checktask}.go`,
  `pkg/doczcore/document/document.go`, `internal/template/templates/impl.md`
- docz-api `docs/impl/0004-…md` (`Verify:` lines), `docs/impl/0006-…md`
  (prefix deferred marker)
