---
id: INV-0032
title: "docz-review's page after Plannotator: adopt its layout or fork it"
status: Open
author: Donald Gifford
created: 2026-11-02
---

<!-- markdownlint-disable-file MD025 MD041 -->

# INV-0032: docz-review's page after Plannotator: adopt its layout or fork it

<!--toc:start-->
<!-- markdownlint-capture -->
<!-- markdownlint-disable MD013 -->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
  - [Observation 1: the round-two diff is raw source in a box apart from the document](#observation-1-the-round-two-diff-is-raw-source-in-a-box-apart-from-the-document)
  - [Observation 2: Plannotator's page is three columns, a toolstrip, and labels](#observation-2-plannotators-page-is-three-columns-a-toolstrip-and-labels)
  - [Observation 3: Plannotator is fifty times docz-review's size and ships daily](#observation-3-plannotator-is-fifty-times-docz-reviews-size-and-ships-daily)
  - [Observation 4: what docz-review knows that Plannotator does not](#observation-4-what-docz-review-knows-that-plannotator-does-not)
  - [Observation 5: the pieces worth taking are small and map onto existing files](#observation-5-the-pieces-worth-taking-are-small-and-map-onto-existing-files)
  - [Observation 6: the share page is a future docz-site surface](#observation-6-the-share-page-is-a-future-docz-site-surface)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
- [Open Questions](#open-questions)
  - [1. Port, fork, or use Plannotator as it is?](#1-port-fork-or-use-plannotator-as-it-is)
  - [2. How does the page show what changed since the previous round?](#2-how-does-the-page-show-what-changed-since-the-previous-round)
  - [3. Which quick labels, and where do they live?](#3-which-quick-labels-and-where-do-they-live)
  - [4. What is the page's layout?](#4-what-is-the-pages-layout)
  - [5. Does the future share page change this work?](#5-does-the-future-share-page-change-this-work)
  - [6. How are a document's open questions answered?](#6-how-are-a-documents-open-questions-answered)
- [Adversarial Review](#adversarial-review)
  - [AR1 — Medium: the cost comparison omits package reuse](#ar1--medium-the-cost-comparison-omits-package-reuse)
  - [AR2 — High: a rendered diff is not supplied by the existing diff dependency](#ar2--high-a-rendered-diff-is-not-supplied-by-the-existing-diff-dependency)
  - [AR3 — High: historical text needs a different annotation contract](#ar3--high-historical-text-needs-a-different-annotation-contract)
  - [AR4 — High: stored snapshots are not yet a history API or change provenance](#ar4--high-stored-snapshots-are-not-yet-a-history-api-or-change-provenance)
  - [AR5 — High: quick labels cross the contract and outcome boundaries](#ar5--high-quick-labels-cross-the-contract-and-outcome-boundaries)
  - [AR6 — Medium: the usability conclusion precedes its experiment](#ar6--medium-the-usability-conclusion-precedes-its-experiment)
  - [AR7 — Medium: visual parity does not establish accessible interaction](#ar7--medium-visual-parity-does-not-establish-accessible-interaction)
  - [AR8 — Medium: some claimed new work already exists, and some claimed existing work does not](#ar8--medium-some-claimed-new-work-already-exists-and-some-claimed-existing-work-does-not)
  - [AR9 — Medium: copying code needs an explicit licence and distribution check](#ar9--medium-copying-code-needs-an-explicit-licence-and-distribution-check)
  - [Review disposition and design gates](#review-disposition-and-design-gates)
- [References](#references)
<!-- markdownlint-restore -->
<!--toc:end-->

<!--docz:question:start-->
## Question

Should docz-review's page take on Plannotator's reading layout (a
contents rail, the rendered document, an annotations rail, quick
labels, and a diff shown in the document) by porting those pieces into
`ui/src/review`, or is it less work to fork Plannotator and strip out
what docz does not need?

<!--docz:question:end-->

<!--docz:hypothesis:start-->
## Hypothesis

Port, do not fork. What we want from Plannotator is a layout and about
five components. A fork's cost is everything else it carries: a
monorepo about fifty times docz-review's size that releases almost
every day. docz-review's value is the docz knowledge Plannotator lacks
(regions, findings, questions, statuses, the review record), and a fork
would have to rebuild that inside a model that was not made for it.

<!--docz:hypothesis:end-->

<!--docz:context:start-->
## Context

The second round of a review of INV-0006 in another repository showed
the diff since the previous round. The content was right, but the view
was hard to follow (Observation 1). Plannotator's annotate view of a
plan, with its toolstrip and quick labels, reads better and needs far
less scrolling (Observation 2).

INV-0027 compared Plannotator's mechanics before docz-review existed
and took its concept (a local page that holds a hook until a person
decides). DESIGN-0024 then left out much of what Plannotator has:

- its non-goals exclude share links, history, and "ask AI";
- question 8 chose the CSS Custom Highlight API over Plannotator's
  `web-highlighter`;
- question 9 chose a whole-source line diff with a word diff to follow.

Those were decided before either tool had been used on real documents.
This investigation revisits the page, not the mechanics: the hooks,
the local server, and the credential are not in question, and the JSON
contract changes only additively (question 3).

**Triggered by:** DESIGN-0024, IMPL-0027, INV-0027, INV-0030 question 2,
and a review of INV-0006 on 2026-11-02

<!--docz:context:end-->

<!--docz:approach:start-->
## Approach

1. Read Plannotator's source at the commit in Environment: the
   annotation model, quick labels, the plan diff views, the layout
   components, and how feedback is written for the agent.
2. Measure both code bases: Plannotator's packages against
   `ui/review` and `ui/src/review`.
3. List what docz-review does that Plannotator does not, and what a
   fork would therefore have to add.
4. Run `plannotator annotate` over a docz document (INV-0006 or a
   parity fixture) beside `docz-review` on the same document, two
   rounds each, and note what each makes easy or hard.
5. For each piece worth taking, name the docz-review file it would
   change, and whether it needs a new dependency.
6. Answer the open questions; the answers become a DESIGN-0024
   amendment or a new design for the page.

<!--docz:approach:end-->

<!--docz:environment:start-->
## Environment

| Component | Version / Value |
| --------- | --------------- |
| docz, docz-review | v2.6.1 |
| Plannotator | `backnotprop/plannotator` at `9d9e701` (2026-11-02), version 0.28.45 |
| Plannotator licence | MIT OR Apache-2.0 (`LICENSE-MIT`, `LICENSE-APACHE`) |
| docz-review page | `ui/src/review`, 26 files, about 5,100 lines |
| docz-review server | `ui/review`, 23 files, about 4,700 lines |

<!--docz:environment:end-->

<!--docz:findings:start-->
## Findings

### Observation 1: the round-two diff is raw source in a box apart from the document

The page (docz-review v2.6.1) reported "4 regions changed since the
previous round" and showed, above the document:

- a summary line for the status change, then a whole-source line diff
  in a scrolling box that starts at the frontmatter;
- the markdownlint comments and the regenerated ToC lines that `docz
  update` writes, counted among the changes;
- runs of unchanged lines around each hunk. The code keeps three a
  side (`CONTEXT_LINES` in `diff.tsx`), but with the ToC and
  markdownlint lines counted as changes the hunks run into one another
  and most of the document is shown.

Reading it means reading markdown source, finding each `+` and `-` in
a long box, and then scrolling to the rendered document to comment,
because diff text cannot be annotated. Two fixes are already
named elsewhere: INV-0030 question 2 adds a word diff toggle, and
DESIGN-0024 question 9 collapses unchanged regions. Neither puts the
change where the reader is already reading.

### Observation 2: Plannotator's page is three columns, a toolstrip, and labels

Plannotator's annotate view of a document has:

- **A contents rail** on the left, built from the headings, with a
  count of annotations beside each section.
- **The rendered document** in the middle, with a toolstrip above it:
  Pinpoint and Markup modes; Comment, Delete, and Label actions; and
  Wide, Focus, and Edit views. There is also a global comment.
- **An annotations rail** on the right: one card per annotation,
  showing the quoted text, the comment or label, the author, and the
  time.
- **Quick labels** (`packages/ui/utils/quickLabels.ts`): select text
  and pick one of ten labels, or press Alt with a digit. A label is
  `{id, emoji, text, color, tip}`. The `tip` is an instruction added
  to the feedback for the agent, so "Verify this" tells the agent to
  read the code before going on. The selection then stays tinted in
  the document, and the label appears as a card in the rail.
- **A plan diff** (`packages/ui/components/plan-diff/`) with a
  rendered "clean" mode and a raw mode, behind a mode switcher.
  Annotations made in the diff record whether the text was added,
  removed, or modified (`Annotation.diffContext`).
- **A version history per annotated file** (`packages/server/annotate.ts`,
  off with `PLANNOTATOR_ANNOTATE_HISTORY=0`), with a diff between two
  versions of the file. So `plannotator annotate` has a round diff of
  its own, and Approach step 4 compares two of them.

Some of the ten default labels carry tips written for code plans:
"Consider alternatives" names Plannotator's own plans directory,
"Verify this" says to read the actual code, "Match existing patterns"
says to search the codebase, and "Missing overview" places the overview
before the implementation details. The set itself is not portable as
it stands. The shape is.

### Observation 3: Plannotator is fifty times docz-review's size and ships daily

TypeScript lines (`*.ts` and `*.tsx` under each directory, `node_modules`
excluded, tests and generated files included):

| Package | Files | Lines |
| ------- | ----: | ----: |
| `packages/ui` | 764 | 165,288 |
| `packages/server` | 369 | 130,297 |
| `packages/shared` | 322 | 99,040 |
| `apps/hook` | 135 | 45,593 |
| `packages/editor` | 126 | 33,237 |
| `packages/core` | 119 | 30,801 |
| **docz-review, both halves** | 49 | about 9,800 |

Beyond plan and document review, the monorepo also carries:

- code review (local git, GitHub and Bitbucket pull requests);
- an inbox and an AI chat;
- a share portal, a paste service, and a relay;
- a VS Code extension;
- adapters for Claude Code, Codex, OpenCode, Pi, Copilot, and DeepSeek
  Harness;
- image, video, and HTML annotation.

Every surface checks GitHub for a newer release (from the machine, at
most once every ten minutes, off with `PLANNOTATOR_UPDATE_CHECK=0` or a
settings toggle), and the binary can update itself when
`PLANNOTATOR_AUTO_UPDATE` is on. `packages/ui` alone depends on CodeMirror,
`@pierre/diffs`, `@base-ui/react`, KaTeX, `@viz-js/viz`, `motion`, and
three of Plannotator's own published packages.

The repository was pushed the day this was read, its version is
0.28.45, and it made twenty releases in the fourteen days to that day
(v0.28.24 on 2026-10-20 to v0.28.45 on 2026-11-02). A fork stripped to document review would diverge on its
first day, and every upstream fix after that is a merge into code we
had cut down.

### Observation 4: what docz-review knows that Plannotator does not

Plannotator's `Annotation` anchors to selected text. It has no idea of
a document's type, its regions, or its lifecycle. docz-review is built
on those, and a fork would have to add each of these to Plannotator's
model:

- **Anchors** of region, heading, and quote (DESIGN-0024 §7), which
  find their text again after the agent edits the document.
- **`docz validate` findings** for the one document, shown with the
  document and recomputed every round.
- **Open-question cards**: choose an option, or write another, from
  the document's open-questions region.
- **Status decisions**, checked against the type's lifecycle.
- **The review record**, governed by `api/cli/review.schema.json` and
  produced to a schema `cmd/` tests. This is the agent-neutral contract
  DESIGN-0024 kept independent of the page.
- **The local server's rules**: a bearer credential, `127.0.0.1` only,
  `Host` and `Origin` checks, and state under the XDG state directory.
- **The document's theme** from `html.theme`, inside the shell
  (DESIGN-0026).

Plannotator's `annotate <file.md>` will review a docz document as
plain markdown today, but it does so without any of the above.

### Observation 5: the pieces worth taking are small and map onto existing files

| Piece | Where it lands | New dependency |
| ----- | -------------- | -------------- |
| Three-column layout, contents rail with counts per section | `document-review.tsx`, `app.tsx` | none |
| Annotation cards in a rail, the selection tinted in the text | `annotations.tsx`, `anchor.ts` (Custom Highlight API, DESIGN-0024 question 8) | none |
| Quick labels with an agent tip and Alt-digit keys | `annotations.tsx`, `feedback.ts`, `review.schema.json` (additive) | none |
| The diff shown in the rendered document, raw on request | `diff.tsx` | none (`diff` is already used) |
| Toolstrip modes and a global comment | `controls.tsx` | none |
| Questions panel and walk-through mode | `questions.tsx`, `draft.ts` | none |
| Label semantics in the record and the feedback | `review.schema.json` (`$version`), `draft.ts`, `feedback.ts` | none |
| Every kept snapshot served to the page, not only the previous one (AR4) | `ui/review/round.ts`, `ui/review/state.ts`, `api.ts` | none |

Plannotator's tinted highlight is achievable with the Custom Highlight
API that DESIGN-0024 already chose, so taking the look does not
reopen question 8. The page files above total about 2,600 lines today,
of which `document-review.tsx`, `annotations.tsx`, `diff.tsx`, and
`feedback.ts`, where most of the work lands, are about 1,250; the two
server files are the history's side. The intent is to write each piece
over docz-review's own model, not to copy. `packages/ui`'s manifest
says Apache-2.0 alone (the dual licence is the repository root's), so a
copied file is recorded with its commit and licence, and Apache-2.0's
licence text, attribution, notice of modification, and any NOTICE ship
with the compiled binary and the page bundle, not only beside the source.

### Observation 6: the share page is a future docz-site surface

The longer-term idea is a docz-site page where a person reads a
document in the web UI and annotations travel between them and an
agent. That is out of scope here, but two things follow from it:

- **Build the page so docz-site can mount it.** The review page is
  already docz-site's second Vite entry and reuses its renderer.
  Keeping the annotation components free of the local server's API
  keeps that path open.
- **Annotations sent to an agent from docz-site are a write path.**
  docz-api is read-only by decision: the scope settled in INV-0023,
  INV-0025, and INV-0026 keeps agents and write access out of it, and
  DESIGN-0023's scope grammar reserves `write`. A share page would
  need that decision revisited first, in its own document.

<!--docz:findings:end-->

<!--docz:conclusion:start-->
## Conclusion

**Answer:** Yes: port the pieces rather than fork (question 1).
Observations 3 to 5 support the hypothesis: the wanted pieces are
small, they need no new dependency, and they land in existing files on
both halves of docz-review, with one server change for the per-round
history. A fork's cost is in what it carries and how fast upstream
moves. The questions below fix the shape, provisionally until step 4
and the Adversarial Review's gates are met:

- a rendered diff with a rail section per round;
- Plannotator's ten labels, two reworded for docz;
- three columns;
- a questions panel at the top with a walk-through mode.

Approach step 4, the side-by-side run of both tools, has not been
done. Do it before the design, to check the layout against a real
second round.

<!--docz:conclusion:end-->

<!--docz:recommendation:start-->
## Recommendation

The questions below are answered provisionally. Do step 4 of the
Approach and meet the five gates in the Adversarial Review, then record
the answers that survive as an amendment to DESIGN-0024's §5, or in a
new design if the layout change is large enough to need its own phases. Fold INV-0030
question 2 (the word diff) into question 2 here rather than building
it separately.

<!--docz:open-questions:start-->
## Open Questions

### 1. Port, fork, or use Plannotator as it is?

- **a. Port the pieces in Observation 5 into `ui/src/review`, written
  in docz-review's own code over its anchors and record.**
  *(recommendation)* No new dependency, and docz-review keeps its
  contract. Any copied code keeps its notice.
- b. Fork Plannotator, strip it to document review, and add docz's
  regions, findings, questions, statuses, and record to it.
- c. Drop docz-review's page and use `plannotator annotate` on docz
  documents, keeping docz-review's hooks only.
- d. Other.

> **Resolved 2026-11-02: (a).** Porting is less
> work: the pieces are small, and a fork means cutting down ~500,000
> lines and then merging every upstream release into what is left.

### 2. How does the page show what changed since the previous round?

- **a. In the rendered document: changed blocks marked in the margin,
  added text tinted, removed text shown struck through in place, and
  each change listed in the contents rail. Frontmatter changes become
  one line each (such as "status: Concluded → In Progress"). Changes
  only to the generated ToC or to markdownlint comments are counted
  but not shown. A raw source diff stays one toggle away. This
  replaces INV-0030 question 2's word diff.** *(recommendation)*
- b. Plannotator's clean and raw switcher as it is.
- c. Keep the raw diff, and add INV-0030 question 2's word toggle and
  generated-line filtering.
- d. Other.

> **Resolved 2026-11-02: (a), revised.** Changes show in the rendered
> document as (a) says, and the right rail also gets one collapsible
> section per round, listing that round's changes. This is where
> docz-review differs from Plannotator: when the agent hands the turn
> back, its edits arrive tagged with their round (such as "Round 2").
> Clicking a round's tag, in the rail or beside a change in the text,
> brings that round's changes forward in the document. The round of
> each change comes from the per-round snapshots docz-review already
> keeps (DESIGN-0024 question 9).

### 3. Which quick labels, and where do they live?

- **a. A docz set, each with a tip for the agent: Clarify this, Verify
  this, Give an example, Missing context, Out of scope, Needs a
  decision, Wrong status, Split this, and Looks good. Alt-digit picks
  one, and the label and tip go into the feedback and the review record
  (an additive `$version` change). Built in at first.**
  *(recommendation)*
- b. Plannotator's ten as they are.
- c. The docz set, plus overrides in `.docz.yaml`.
- d. Other.

> **Resolved 2026-11-02: (b), revised.** Start with Plannotator's
> ten, in its order, so Alt+1 to Alt+0 cover the whole set and the
> menu needs no scrolling. The tips that only make sense for
> Plannotator are reworded for docz:
>
> - **Consider alternatives:** "Propose 2-3 alternatives with
>   trade-offs. Check this repository's RFCs, ADRs, and investigations
>   for approaches already considered or rejected."
> - **Needs tests:** gains a tip, "Say how this will be tested or
>   verified, and add it to the document."
>
> Any other label waits until the ten have been used for a while.
> Changing the list later is the (c) override, not part of this work.

### 4. What is the page's layout?

- **a. Three columns. On the left, a contents rail from the document's
  headings, with counts per section and the round's changes. In the
  middle, the document. On the right, a rail holding the annotation
  cards, the findings, and the question cards, each linked to its
  place in the text. Below a set width, it falls back to one column.**
  *(recommendation)*
- b. Keep one column and add only the annotations rail.
- c. Other.

> **Resolved 2026-11-02: (a), revised.** The left contents rail is part of
> this work. The per-round list of changes lives in the right rail,
> one collapsible section per round, as question 2 resolves it; the
> left rail shows a count of annotations per section and marks the
> sections the latest round changed, and lists no changes itself.

### 5. Does the future share page change this work?

- **a. No. Build the page so docz-site could mount it, and record the
  share page as a later idea. That idea first needs docz-api's
  read-only scope revisited in its own document.** *(recommendation)*
- b. Design for the share page now.
- c. Other.

> **Resolved 2026-11-02: (a).** Not now. Keep the components free of
> the local server's API.

### 6. How are a document's open questions answered?

Raised in review on 2026-11-02. A document with an open-questions
region gets a questions panel at the top of the page, above the
document. The question here is what the panel does.

- **a. List every unresolved question with its options as quick
  picks, and the recommended option preselected. Add an "Accept all
  recommendations" action, and a walk-through mode that shows one
  question at a time, with a key per option and keys for the next and
  previous question. Answers go into the feedback and the review
  record exactly as the in-place question cards' answers do today.
  The cards in the text stay, linked to the panel.**
  *(recommendation)*
- b. The panel without the walk-through mode.
- c. Other.

> **Resolved 2026-11-02: (a), revised.** Nothing is preselected.
> DESIGN-0024 §5 says the recommended option is badged, never
> preselected, and `questions.tsx` holds to that; this keeps it, so an
> answer is always a choice the reviewer made. "Accept all
> recommendations" already exists on the question cards and moves to the
> panel rather than being added. §5 places the cards in a sidebar and
> inline at each question, but the page mounts them in its aside only
> (AR8), so the panel is where the aside's cards go, and the inline
> cards stay §5's unbuilt promise, linked from the panel. The
> walk-through mode is the new piece.

<!--docz:open-questions:end-->

<!--docz:recommendation:end-->

<!--docz:adversarial-review:start-->
## Adversarial Review

Reviewed 2026-11-02 against the current working-tree implementation,
DESIGN-0024, the review schema, and Plannotator's pinned `9d9e701`
quick-label source and UI package manifest. This was a source and
design review, **not** Approach step 4's interactive comparison. The
findings below challenge the rationale and implementation assumptions;
they do not silently replace the six recorded resolutions.

**Verdict:** keeping docz-review's model and porting selected interactions
is defensible. The stronger claims—less work, seven files, no new
dependency, and a settled page shape—are not yet demonstrated. The
rendered diff and historical annotations are design work, not merely
layout work.

### AR1 — Medium: the cost comparison omits package reuse

Question 1 compares a rewrite with a full fork or replacing the page.
At the pinned commit, however, [`@plannotator/ui`'s manifest][ar-ui]
exports individual components, `components/plan-diff/*`, hooks, utilities,
and contents-rail styles. Evaluating a narrow package import is a fourth
option; it does not require adopting Plannotator's server or record.
The dependency list is substantial, so this is not a recommendation to
import it without measurement.

The whole-repository line count does not measure the dependency closure
of the document-review surface, and the document gives no counting
command or exclusions for tests, declarations, or generated code.
Likewise, a push on one day and version `0.28.45` do not establish a
daily release cadence. A fork may pin a baseline and selectively take
fixes; it need not merge every release. Ported code also needs fixes.

**Needed:** compare a docz-native implementation, narrow package reuse,
and a pinned stripped fork by integration effort, required dependencies,
shipped bundle, test coverage, and ownership of future fixes. Keep the
port decision if it wins that comparison, not solely the line-count ratio.

**Response (2026-11-02):** Measured. `@plannotator/ui` 0.58.0 is on
npm under Apache-2.0 with 37 dependencies and `react`, `react-dom`,
and `tailwindcss` as peers, and its exports are component globs, so a
narrow import is possible in principle. What step 4 measures is whether
a rail or toolbar can be used without its `Annotation` type and
contexts, since adopting those is the cost Observation 4 names. The
counting method is now in Observation 3 (tests and generated files
included), and the cadence is data there: twenty releases in fourteen
days. A pinned fork avoids the merges by forgoing the fixes, which is
the trade question 1 weighed. Adopted: the comparison in step 4 covers
the port, a narrow import, and a pinned fork by integration effort,
dependency closure, bundle size, and who owns later fixes.

### AR2 — High: a rendered diff is not supplied by the existing diff dependency

`ui/src/review/diff.tsx` uses `diffLines` to compare source strings and
marks regions by kind and heading. It does not align Markdown trees or
map word edits into rendered nodes. Having `diff` installed establishes
neither a rendered-diff algorithm nor its implementation cost.

Text can move between headings, lists, tables, links, code blocks, and
raw HTML. A changed link destination or code language may leave the
visible words unchanged. Combining removed and current source before
parsing can also change Markdown structure rather than merely decorate
it. Heading renames currently pair as deletion plus addition, not moves.

**Needed:** prototype a block-alignment and rendering strategy, with
fixtures for those cases and generated-only changes. Preserve
`ui/src/markdown/processor.ts`'s raw-HTML-before-sanitize invariant and
trusted post-sanitize line stamps. Retain raw inspection and explicit
size/time fallbacks; the current diff already has character and edit
bounds. Treat “no new dependency” as a target until this spike succeeds.

**Response (2026-11-02):** Agreed that `diff` gives line hunks and
nothing else. What the page already has is `data-line` on every
rendered block, so hunks map to blocks by line range without aligning
trees: a block whose lines a hunk touches is changed, a block only in
the previous source is removed. The first slice is at block level:
changed blocks marked in the margin, removed blocks rendered from the
previous source and struck through, and inline word tinting only inside
a block whose change is text-only (same block type on both sides), with
the source shown otherwise. A link destination or code language changes
the block's source, so the block is marked even when its words are not.
The bounds stay (`DIFF_CHAR_LIMIT`, `DIFF_EDIT_LIMIT`); past them the
page shows badges only, as today. The spike with those fixtures is the
gate before the design, and "no new dependency" is its target.

### AR3 — High: historical text needs a different annotation contract

Question 2 puts struck-through removed text into the reading surface,
but does not say whether it is selectable. Today `anchorFromRange` and
`resolveAnchor` in `ui/src/review/anchor.ts` assume one document's text
and line coordinates. `DocumentReview` resolves submissions against the
current `inspect.content`; `review.schema.json` has no source-version or
diff-side field on an annotation.

A comment on deleted text will be lost, or could match identical text
elsewhere in the current document. A selection crossing removed and
current text has no single-source meaning. Merely recording `lost` does
not tell the agent that the reviewer intentionally selected old text.

**Needed:** either make historical/deleted text explicitly non-annotatable
in the first version, or design version-qualified anchors containing the
source round/digest and side, with matching feedback. Mixed-version
replace/delete actions must be refused. Viewing an old round must never
allow approval of those bytes as if they were the current snapshot.

**Response (2026-11-02):** Adopted. Removed text is not annotatable
in the first version: it renders with `data-no-annotate`, as diff text
does today, and the popover refuses a selection that touches it, so no
anchor carries a version and the record is unchanged by the diff.
Version-qualified anchors wait until step 4 shows a need. Viewing an
earlier round is reading only; the draft, Submit, and Approve stay
bound to the current round's bytes (`round.ts`'s 409).

### AR4 — High: stored snapshots are not yet a history API or change provenance

`ui/review/state.ts` persists per-round snapshots, but `Review.open()` in
`ui/review/round.ts` reads only round N−1. `Review.view()` and
`ui/src/review/api.ts` expose a single `previous: string | null`, not a
round list. The resolved right-rail history therefore needs server and
API changes outside Observation 5's component list.

Snapshots can show that text differed between two round openings. They
cannot prove the agent authored an edit, or assign one durable origin to
text edited in round 2, moved in round 3, and reverted in round 4. The
phrase “brings that round's changes forward” could mean viewing N−1 → N
or projecting old edits onto today's document; those are different jobs.

**Needed:** define pairwise history versus current-document provenance,
round numbering, missing snapshots, retention, and bounded/lazy loading.
Call edits “observed between rounds,” not agent-authored, unless there is
additional evidence. A first slice showing only previous → current
avoids promising a provenance system accidentally.

**Response (2026-11-02):** Correct, and Observation 5's table now
carries it: the page gets `previous` only, so the per-round rail needs
`round.ts` to serve every kept snapshot, or each pair's hunks, and
`api.ts` to carry them. The words are "changes observed between
rounds", never "the agent's edits". "Brings forward" means showing that
round's N−1 → N hunks over today's document where their blocks still
exist, with the rest listed as no longer present; it is not provenance.
The first slice is previous → current only; the round list follows
step 4, with retention and lazy loading as design items.

### AR5 — High: quick labels cross the contract and outcome boundaries

Context says the JSON contract is not in question, but Observation 5 and
question 3 explicitly extend it. The current annotation schema rejects
unknown properties and has a closed `kind` enum. Adding optional label
metadata may follow DESIGN-0024's minor `$version` policy, but an old
strict validator still rejects records containing it; adding a new kind
is not equivalent to adding an optional field.

There is also a behavioral decision hidden inside the ten-label set.
Plannotator's tenth default is **Nice approach**, while **Looks good**
is a separate positive action in its [quick-label source][ar-labels].
docz's `draft.ts` treats `looks-good` as non-actionable and `feedback.ts`
omits it from change instructions. Encoding every label as a comment
would make praise request another agent turn.

**Needed:** specify label IDs, saved display text/tips, actionable versus
positive semantics, and whether any label is destructive. Carry reviewer-
selected tips as review instructions, not elevated system authority.
Test the full draft → record → schema validation → feedback path,
including regeneration of CLI types, schema fixtures, and Go contract
tests. Define how mixed-version readers handle the optional metadata.

**Response (2026-11-02):** Adopted. A label is an optional `label`
object on an annotation (`id`, `text`, `tip`), an additive field under
the schema's `$version` policy; no new `kind`. The kind carries the
semantics: "Nice approach" and "Looks good" are `looks-good`
(non-actionable, no turn), and the other eight are `comment` with the
tip appended to the text the agent reads, so `draft.ts` and
`feedback.ts` need no new branch and a tip travels as reviewer text,
not as an instruction above the reviewer's. The hand-written review
record fixture gains a labelled annotation, the CLI types regenerate,
and the Go contract test revalidates. Context now says the contract
changes only additively.

### AR6 — Medium: the usability conclusion precedes its experiment

The document says the comparison has not run, yet all six questions are
resolved and the conclusion fixes the layout. A real complaint about
round-two source diffs supports improving that experience; it does not
establish that a toolstrip, ten labels, three columns, historical rails,
and a question walk-through together improve it.

**Needed:** run the same review tasks on the same two-round fixture in
both tools: find a substantive change, comment on it, answer a question,
and submit without losing a draft. Record task time, scrolling/navigation,
missed changes, and mistakes at wide and narrow widths. Compare against
a smaller docz change, such as its existing collapsed diff plus generated-
line filtering. If keeping the resolutions, mark their usability claims
as awaiting validation rather than treating the experiment as a formality.

**Response (2026-11-02):** Agreed; this is why the INV is Open and
why the Recommendation now calls the resolutions provisional. Step 4
runs the tasks this finding names on the same two-round fixture in both
tools, with the smaller docz change (the collapsed diff plus
generated-line filtering) as the control, and records time, scrolling,
missed changes, and mistakes at wide and narrow widths.

### AR7 — Medium: visual parity does not establish accessible interaction

`paintAnnotations` currently paints every annotation under one highlight
name and paints nothing without the Custom Highlight API. This supports
tinting, not label-specific colors, overlap rules, card selection, or
keyboard navigation. Color alone cannot communicate change kind or label.

Alt-digit shortcuts can conflict with browser/OS behavior and keyboard
layouts. The question walk-through adds another key map, and moving
questions above the document can itself increase scrolling. “Below a set
width” leaves the navigation, focus order, and access to actions undefined.

**Needed:** test keyboard-only selection and card navigation, focus return,
text-input shortcut suppression, zoom/reflow, supported browsers without
highlight painting, and narrow-screen rail access. Supply text indicators
and ordinary buttons for every shortcut action, and define highlight
priority when annotations and diff marks overlap.

**Response (2026-11-02):** Adopted as acceptance tests for the
design. Plannotator reads `e.code` (`Digit1` to `Digit0`) with Alt, so
the shortcut works on a macOS layout where Option and a digit type a
symbol; docz does the same, suppresses it inside text inputs, and gives
every shortcut a button. Highlights: one `Highlight` per label colour
and one for diff marks, with a defined priority where they overlap, and
the change kind and label always also in text (the margin mark and the
rail), so a browser without the API loses only the tint. Below the
width threshold the rails become a tabbed drawer with the same actions.

### AR8 — Medium: some claimed new work already exists, and some claimed existing work does not

Observation 1's “every unchanged line” describes the reported session,
not necessarily today's code: `diff.tsx` already collapses long unchanged
runs with three context lines. `document-review.tsx` currently renders
that diff **above**, not below, the document. `annotations.tsx` already
provides an annotation list, selection painting, and a document comment;
`processor.ts` already returns a rendered ToC.

Conversely, DESIGN-0024 specifies inline question cards, but the current
`DocumentReview` mounts `QuestionCards` only in its aside; the review
renderer does not insert those cards into the text. Question 6's
resolution must not count the design promise as implemented behavior.

**Needed:** record the binary/build used in Observation 1 and separate
observed behavior, implemented features, and design-only features. Replace
the seven-file estimate with a delta that includes `questions.tsx`,
`draft.ts`, the renderer, styling, generated types, server/API work, and
tests where required. Update Recommendation's “answer the questions”
wording now that the decisions are recorded.

**Response (2026-11-02):** Corrected in Observation 1 (the diff is
above the document, unchanged runs collapse to three lines a side, the
build was v2.6.1), in Observation 5 (the table now carries the
questions, draft, schema, server, and API work), in question 6 (inline
cards are §5's promise, not the page's behaviour), and in the
Recommendation's wording.

### AR9 — Medium: copying code needs an explicit licence and distribution check

The environment records dual licensing at repository level, while the
pinned UI package declares `Apache-2.0`. That discrepancy needs checking
for the exact files copied; “keep a notice beside it” is not a complete
distribution plan. Under Apache-2.0, relevant obligations include a copy
of the licence, retained attribution, notices of modifications, and any
applicable upstream NOTICE attribution.

**Needed:** record each copied file's commit and applicable licence,
including third-party portions. Check that required notices ship with the
compiled review binary and browser assets, not just beside source files.
A layout independently implemented over docz's model need not involve
copying upstream code at all.

**Response (2026-11-02):** Confirmed: `@plannotator/ui`'s manifest
and its npm record say Apache-2.0 alone; the dual licence is the
repository root's. Observation 5 now says the intent is independent
implementation over docz's model, and what a copied file would require.

### Review disposition and design gates

Keep **port, not fork** as the working direction, but do not use this
investigation as evidence that the full resolved scope is a small UI-only
change. Before design approval:

1. Complete the interactive comparison and narrow-package evaluation.
2. Prove a bounded rendered-diff approach through the existing sanitizer.
3. Decide historical annotation semantics and history/API scope.
4. Specify label compatibility and no-action semantics end to end.
5. Define keyboard, narrow-screen, and non-highlight acceptance tests.

Prefer a first slice of contents/annotation navigation and latest-round
changes. Historical projection and question walk-through can follow if
the experiment demonstrates their value. The local credential and hook
security model can remain unchanged; server payloads and the record
contract cannot both be declared untouched under the current resolutions.

**Response (2026-11-02):** The five gates are adopted as the entry
conditions for the design, and the first slice is as proposed: the
contents and annotation rails and the latest round's changes, with the
round history and the walk-through after step 4.

[ar-ui]: https://github.com/backnotprop/plannotator/blob/9d9e701/packages/ui/package.json
[ar-labels]: https://github.com/backnotprop/plannotator/blob/9d9e701/packages/ui/utils/quickLabels.ts

<!--docz:adversarial-review:end-->

<!--docz:references:start-->
## References

- [DESIGN-0024](../design/0024-docz-review-a-blocking-review-page-for-agent-written-documents.md): docz-review, a blocking review page for agent-written
  documents (§5 the page, §7 anchors, questions 8 and 9)
- [IMPL-0027](../impl/0027-docz-review-the-cli-contract-the-review-page-the-binary-and-the.md): docz-review's implementation
- [INV-0027](0027-interactive-mode.md): interactive mode, the first Plannotator comparison
- [INV-0030](0030-docz-review-follow-ups-after-v230.md): docz-review follow-ups after v2.3.0 (question 2, word diff)
- [INV-0031](0031-trial-docz-review-on-its-own-repository.md): trial of docz-review on its own repository
- [DESIGN-0026](../design/0026-docz-sites-shell-in-repo-guardians-look-and-the-readers.md): the shell and the document look
- [DESIGN-0023](../design/0023-a-read-only-mcp-server-in-docz-api-oauth-21-tokens-tools-and.md): scoped access and the MCP server (the read-only scope)
- [Plannotator source](https://github.com/backnotprop/plannotator)
- [Plannotator documentation](https://docs.plannotator.ai/)

<!--docz:references:end-->
