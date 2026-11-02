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
  - [Observation 1: the round-two diff is raw source in a box below the document](#observation-1-the-round-two-diff-is-raw-source-in-a-box-below-the-document)
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
the local server, the credential, and the JSON contract are not in
question.

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

### Observation 1: the round-two diff is raw source in a box below the document

The page reported "4 regions changed since the previous round" and
showed:

- a summary line for the status change, then a whole-source line diff
  in a scrolling box that starts at the frontmatter;
- the markdownlint comments and the regenerated ToC lines that `docz
  update` writes, counted among the changes;
- every unchanged line of the document between hunks.

Reading it means reading markdown source, finding each `+` and `-` in
a long box, and then scrolling back up to the rendered document to
comment, because diff text cannot be annotated. Two fixes are already
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

TypeScript lines, excluding `node_modules`:

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

The repository was pushed the day this was read, and its version is
0.28.45. A fork stripped to document review would diverge on its
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

Plannotator's tinted highlight is achievable with the Custom Highlight
API that DESIGN-0024 already chose, so taking the look does not
reopen question 8. The seven files above total about 1,900 lines
today, of which `document-review.tsx`, `annotations.tsx`, `diff.tsx`,
and `feedback.ts`, where most of the work lands, are about 1,250.
If any code is copied rather than rewritten, the licence permits it
with the copyright and licence notice kept beside it.

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
small, they need no new dependency, and they land in seven existing
files. A fork's cost is in what it carries and how fast upstream
moves. The questions below fix the shape:

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

Do step 4 of the Approach, then answer the questions below. Record the
answers as an amendment to DESIGN-0024's §5, or in a new design if the
layout change is large enough to need its own phases. Fold INV-0030
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
> panel rather than being added. §5 also already places the cards in a
> sidebar and inline at each question, so the panel is where the
> sidebar's cards go, not a third copy. The walk-through mode is the new
> piece.

<!--docz:open-questions:end-->

<!--docz:recommendation:end-->

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
