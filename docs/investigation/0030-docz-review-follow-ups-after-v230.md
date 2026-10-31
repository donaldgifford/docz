---
id: INV-0030
title: "docz-review follow-ups after v2.3.0"
status: Open
author: Donald Gifford
created: 2026-10-31
---

<!-- markdownlint-disable-file MD025 MD041 -->

# INV-0030: docz-review follow-ups after v2.3.0

<!--toc:start-->
<!-- markdownlint-capture -->
<!-- markdownlint-disable MD013 -->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
  - [Observation 1: the description is stale in five places](#observation-1-the-description-is-stale-in-five-places)
  - [Observation 2: a round's diff is by line only](#observation-2-a-rounds-diff-is-by-line-only)
  - [Observation 3: tasks are shown, and nothing can act on them](#observation-3-tasks-are-shown-and-nothing-can-act-on-them)
  - [Observation 4: the two anchor resolvers measure context differently](#observation-4-the-two-anchor-resolvers-measure-context-differently)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
- [Open Questions](#open-questions)
  - [1. How does the description fix ship?](#1-how-does-the-description-fix-ship)
  - [2. Does the review page get a word-level diff?](#2-does-the-review-page-get-a-word-level-diff)
  - [3. How are tasks ticked from the page?](#3-how-are-tasks-ticked-from-the-page)
  - [4. How do the two anchor resolvers stay in agreement?](#4-how-do-the-two-anchor-resolvers-stay-in-agreement)
- [References](#references)
<!-- markdownlint-restore -->
<!--toc:end-->

<!--docz:question:start-->
## Question

Which of the docz-review items IMPL-0027 deferred, and the ones its trial
and releases left behind, are worth doing next, and in what shape?

<!--docz:question:end-->

<!--docz:hypothesis:start-->
## Hypothesis

Four items remain and each is small: a stale plugin description, a
word-level diff toggle, ticking tasks from the review page, and the
anchor shape the review page shares with the Confluence export. None
needs a new design; each fits one PR, and only the last could hide a
correctness problem rather than a missing feature.

<!--docz:hypothesis:end-->

<!--docz:context:start-->
## Context

**Triggered by:** IMPL-0027's last open task, "open follow-up issues:
the word-level diff toggle, task toggles from the page, INV-0021's
Confluence side of the anchor shape, and anything Phase 5's trial
surfaced", and v2.3.0 (#206), which made `docz create` arm a review only
with `DOCZ_REVIEW_AUTO_ARM` on.

Where docz-review stands on 2026-10-31:

| Released | Version | What it carried |
| --- | --- | --- |
| 2026-10-31 | v2.3.0 (#206), plugin 0.2.0, charts/docz 1.3.0 | Auto-arming opt-in; README "Auto-arming" and "Without the plugin" |
| 2026-10-31 | v2.2.2 (#205), charts/docz 1.2.1 | Submit never approves an empty draft; an unadded comment holds Submit and Approve; install-scope note |
| 2026-10-31 | v2.2.1 (#204) | go-jose CVE fix |
| 2026-10-28 | v2.2.0 (#200) | First release |

Verified: the hook drill (#203), the trial on this repository, both
private-marketplace install paths (claude-skills #156, resynced in
claude-skills #157), and v2.2.0 on macOS. Not yet verified: the install on Linux, and
whether `DOCZ_REVIEW_AUTO_ARM` set in a settings `env` block reaches the
hook.

<!--docz:context:end-->

<!--docz:approach:start-->
## Approach

1. Search both repositories for the plugin's description and list every
   copy.
2. Read `ui/src/review/diff.tsx` for what a round's diff shows today and
   which bounds apply.
3. Read how the page renders tasks, whether the review record or the
   CLI has any task operation, and what `docwrite` offers.
4. Compare `ui/src/review/anchor.ts` with
   `pkg/export/confluence/comments.go`: the fields, the context length,
   and the scoring.

<!--docz:approach:end-->

<!--docz:environment:start-->
## Environment

| Component | Version / Value |
| --------- | --------------- |
| docz | v2.3.0 (`main` at the merge of #206) |
| docz-review plugin | 0.2.0 |
| claude-skills | metadata 3.3.0 |

<!--docz:environment:end-->

<!--docz:findings:start-->
## Findings

### Observation 1: the description is stale in five places

The plugin is described as reviewing "every docz document the agent
created", true since v2.3.0 only with auto-arming on:

| Repository | File | Field |
| --- | --- | --- |
| docz | `plugins/docz-review/.claude-plugin/plugin.json` | `description` |
| docz | `.claude-plugin/marketplace.json` | the docz-review entry |
| docz | `plugins/docz-review/README.md` | the example entry under "Copy the directory" |
| claude-skills | `.claude-plugin/marketplace.json` | the docz-review entry |
| claude-skills | `plugins/docz-review/CARD.md` | generated from `plugin.json` by `just cards` |

It changes no behaviour, so the docz side needs no release and the
claude-skills side is a patch bump of the plugin and the metadata.

### Observation 2: a round's diff is by line only

From round two the page diffs the whole source with `diffLines`, with
three lines of context and unchanged runs collapsed, and marks each
changed region. A one-word fix in a long paragraph shows as the
paragraph removed and added again, and most annotations ask for exactly
that kind of change. The `diff` package already installed has
`diffWordsWithSpace`, so a word view needs no dependency. Two bounds
already apply and would apply to it too: `DIFF_CHAR_LIMIT` (512 KiB)
and `DIFF_EDIT_LIMIT` (4,000 edits), past which only the marks show.

### Observation 3: tasks are shown, and nothing can act on them

The page renders an IMPL's tasks with their state; the review record
(`api/cli/review.schema.json`) has no task field; `docz` has no command
that ticks a task. The library does, `docwrite.SetTaskState(path, line,
checked)`, but only Go callers reach it. A reviewer who sees a task done
says so in a comment, and the agent has to work out which checkbox the
comment means.

A status change already works the way a toggle would have to: the page
records it, the feedback says `Status: set to Accepted`, and the agent
makes the change. Writing the document from the page stays a non-goal
(DESIGN-0024).

### Observation 4: the two anchor resolvers measure context differently

INV-0021 resolved that a Confluence comment and a review annotation
share one anchor shape, and DESIGN-0024 section 7 says the two tools
"agree on what an anchor means". The fields and the rules do agree: a
quote, the text either side, the heading above; one hit wins; a strictly
best context score wins; a tie is lost; a matching heading adds twice
the context length. The unit does not:

| Resolver | Context kept | Scored over |
| --- | --- | --- |
| `pkg/export/confluence` (`contextLen`) | 32 bytes, cut to a character boundary | bytes |
| `ui/src/review/anchor.ts` (`CONTEXT`) | 32 code points | code points |

For ASCII the two are identical. For text outside it they are not: a
CJK character is three bytes, so the export keeps about ten characters
either side where the page keeps 32, and the heading bonus is worth a
different amount of text in each. The two tools never read each
other's anchors today, so nothing breaks; but nothing would catch the
drift if they started to, because the review page's tests were ported
from `carryMarkers`' by hand and no test runs the same cases through
both. The field names differ too (`quote` and `exact`), which is
cosmetic.

<!--docz:findings:end-->

<!--docz:conclusion:start-->
## Conclusion

**Answer:** Yes, all four are worth doing, each as one PR. Three are
features or a fix as expected. The fourth is not the open design
question IMPL-0027 named, since INV-0021 settled the shape and #186
built the Confluence side, but Observation 4 found a real mismatch in
its unit, and that is what is left to do.

<!--docz:conclusion:end-->

<!--docz:recommendation:start-->
## Recommendation

Open one issue per item once the questions below are answered, and
mark IMPL-0027 Completed in the PR that fixes the description, after
the Linux install and the settings `env` check are done.

<!--docz:open-questions:start-->
## Open Questions

### 1. How does the description fix ship?

- **a. One `dont-release` docz PR that fixes the three docz copies,
  bumps the plugin to 0.2.1, and marks IMPL-0027 Completed; then a
  claude-skills resync (plugin 0.2.1, metadata 3.3.1, card
  regenerated).** *(recommendation)* The text: "Blocks the end of a
  turn on a browser review of the docz documents you queue, or every
  one the agent creates with auto-arming on, and feeds the reviewer's
  answers back as the next instruction."
- b. The same, without IMPL-0027, which closes separately.
- c. Other.

### 2. Does the review page get a word-level diff?

- **a. A toggle beside "collapse unchanged" that shows changed lines
  as a word diff (`diffWordsWithSpace`), off by default, remembered for
  the review in `sessionStorage`, under the existing bounds, with diff
  text still not annotatable.** *(recommendation)* Acceptance: a
  one-word change in a 200-word paragraph shows as that word; vitest
  over the hunk-to-words split; one Playwright case on round two.
- b. Word diff always on, no toggle.
- c. Not now.
- d. Other.

### 3. How are tasks ticked from the page?

- **a. As a decision sent to the agent, like a status change: a flip is
  an actionable draft item `{line, text, checked}`, the feedback says
  `Task: mark "<text>" (line N) done` or `not done`, and the review
  record gains an optional `tasks` array (an additive change, so
  `$version` moves and `schema_version` stays 1). IMPL documents only
  at first.** *(recommendation)* A moved task resolves by its text, as
  anchors do.
- b. The same, for any document with `- [ ]` items.
- c. First add `docz task check|uncheck <id> <line>` over
  `docwrite.SetTaskState`, and have the feedback name that command.
- d. Not now.
- e. Other.

### 4. How do the two anchor resolvers stay in agreement?

- **a. Count context in code points in Go as the page does, and add
  shared conformance cases (JSON under `testdata/`) that both
  `pkg/export/confluence` and `ui/src/review` run, so a change to
  either rule fails the other's tests.** *(recommendation)* Anchors
  already stored in page properties keep working, since scoring
  compares the stored strings; new captures are just cut by characters.
- b. Count bytes in the page instead, to match Go.
- c. Shared cases only, and document the unit difference.
- d. Leave it: the tools never exchange anchors.
- e. Other.

<!--docz:open-questions:end-->

<!--docz:recommendation:end-->

<!--docz:references:start-->
## References

- [IMPL-0027](../impl/0027-docz-review-the-cli-contract-the-review-page-the-binary-and-the.md):
  the plan whose last task this investigation answers
- [DESIGN-0024](../design/0024-docz-review-a-blocking-review-page-for-agent-written-documents.md):
  sections 5 and 7, the non-goals, rollout step 7, open question 9
- [INV-0021](0021-confluence-comments-as-a-view-layer-kept-across-source-changes.md):
  the shared anchor shape, question 2
- [docz #186](https://github.com/donaldgifford/docz/pull/186): anchors
  in the Confluence page property
- [docz #205](https://github.com/donaldgifford/docz/pull/205) and
  [#206](https://github.com/donaldgifford/docz/pull/206): the v2.2.2 and
  v2.3.0 fixes
- [claude-skills #156](https://github.com/donaldgifford/claude-skills/pull/156)
  and [#157](https://github.com/donaldgifford/claude-skills/pull/157): the
  plugin's copy and its resync

<!--docz:references:end-->
