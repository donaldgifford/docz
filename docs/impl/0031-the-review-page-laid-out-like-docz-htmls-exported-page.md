---
id: IMPL-0031
title: "The review page laid out like docz-html's exported page"
status: Draft
author: Donald Gifford
created: 2026-11-01
---

<!-- markdownlint-disable-file MD024 MD025 MD041 -->

# IMPL-0031: The review page laid out like docz-html's exported page

<!--toc:start-->
<!-- markdownlint-capture -->
<!-- markdownlint-disable MD013 -->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: The design amendment and the outline in docz inspect](#phase-1-the-design-amendment-and-the-outline-in-docz-inspect)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2: Tabs on the review page](#phase-2-tabs-on-the-review-page)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 3: The components that are layout only](#phase-3-the-components-that-are-layout-only)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 4: Phases, procedures, scenarios, and questions](#phase-4-phases-procedures-scenarios-and-questions)
    - [Tasks](#tasks-3)
    - [Success Criteria](#success-criteria-3)
  - [Phase 5: Close-out](#phase-5-close-out)
    - [Tasks](#tasks-4)
    - [Success Criteria](#success-criteria-4)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Dependencies](#dependencies)
- [Open Questions](#open-questions)
  - [1. Where does the review page get the outline from?](#1-where-does-the-review-page-get-the-outline-from)
  - [2. Do hidden panels stay mounted?](#2-do-hidden-panels-stay-mounted)
  - [3. Which tab opens first?](#3-which-tab-opens-first)
  - [4. Is there a whole-document view?](#4-is-there-a-whole-document-view)
  - [5. Where do the layout components live?](#5-where-do-the-layout-components-live)
  - [6. How is IMPL phase progress counted?](#6-how-is-impl-phase-progress-counted)
  - [7. What does the Questions tab hold?](#7-what-does-the-questions-tab-hold)
  - [8. How are findings shown?](#8-how-are-findings-shown)
  - [9. How do the phases release?](#9-how-do-the-phases-release)
  - [10. Does docz-site's reader adopt tabs too?](#10-does-docz-sites-reader-adopt-tabs-too)
- [References](#references)
<!-- markdownlint-restore -->
<!--toc:end-->

<!--docz:objective:start-->
## Objective

Lay out docz-review's page the way `docz export html` lays out a page:
a tab per section and the components of DESIGN-0025 §4, chosen by region
kind (cards, callouts, phase cards, expandables). A reviewer should see a
document as its readers outside the repository will, and review it
without losing anything the review page does today.

**Implements:** [DESIGN-0024](../design/0024-docz-review-a-blocking-review-page-for-agent-written-documents.md)
§5 and §7, amended in Phase 1 to adopt
[DESIGN-0025](../design/0025-docz-html-standalone-html-pages-from-docz-documents-and-one.md)
§4. Tracks [#212](https://github.com/donaldgifford/docz/issues/212).

**What already holds, and is not rebuilt here:**

- The review render mode keeps document lines: frontmatter and the ToC
  are blanked rather than cut, and `rehype-data-line.ts` stamps
  `data-line` on every block after sanitize.
- `regions.tsx` wraps blocks in `<section data-region data-ordinal>` from
  `docz inspect`'s regions, the deepest region winning.
- Anchors (`anchor.ts`) resolve against the **source**: quote, context,
  region, ordinal, heading, and a line hint, by the rules of
  `carryMarkers`. Nothing about resolution depends on where a block sits
  on the page.
- Annotations are painted with the CSS Custom Highlight API over DOM
  ranges, and a changed region is marked by a `data-changed` attribute.
  Neither inserts DOM into the rendered markdown.
- `pkg/export/html` already computes the outline (`outline.go`: a tab per
  depth-0 region, a tab per H2 outside regions, the ToC dropped, gaps
  folded) and the component per kind (`layout.go`, DESIGN-0025 §4's
  table).

**What this plan depends on:** [IMPL-0030](0030-one-theme-system-across-docz-html-docz-review-and-docz-site.md)
Phase 2 (the review page's theme comes from `.docz.yaml`, and its
`inspect` schema bump) and Phase 5 (the widgets as React components,
which go inside the panels this plan builds).

<!--docz:objective:end-->

<!--docz:scope:start-->
## Scope

<!--docz:in-scope:start-->
### In Scope

- Amending DESIGN-0024 §5 and §7, and DESIGN-0025 §11, for a tabbed
  review page.
- `docz inspect` reporting the page outline: the tabs, which regions and
  lines each holds, and each region's component.
- The review page's tab bar and panels, with every jump (finding,
  annotation, question, changed region, outline link) opening the right
  tab first.
- React components for DESIGN-0025 §4's table, built over the review
  render so `data-line`, region wrappers, highlights, and the selection
  popover keep working inside them.
- Per-tab badges for findings, annotations, unanswered questions, and
  changes.
- e2e, full-rule axe, and the unit suites for all of it.

<!--docz:in-scope:end-->

<!--docz:out-of-scope:start-->
### Out of Scope

- docz-site's reader. Whether it adopts tabs is Open Question 10.
- Toggling IMPL tasks from the page. That stays a DESIGN-0024 non-goal.
- Changing what an export lays out. This plan reads the export's outline
  and does not alter it.
- Theme work. That belongs to IMPL-0030.

<!--docz:out-of-scope:end-->
<!--docz:scope:end-->

## Implementation Phases

Each phase builds on the previous one. A phase is complete when all its tasks
are checked off and its success criteria are met. Each phase is one PR
labelled `dont-release`, and the last is labelled `minor` (Open Question 9).

---

<!--docz:phase:start-->
### Phase 1: The design amendment and the outline in `docz inspect`

The contract the page builds on: one definition of the layout, in Go,
reported by `docz inspect` so the review page never re-derives it.

<!--docz:tasks:start-->
#### Tasks

- [ ] Amend DESIGN-0024 §5 so the page is laid out by DESIGN-0025 §4: tabs,
  components, and per-tab badges. Settle this plan's open questions there.
- [ ] Amend DESIGN-0024 §7 to say anchors are unchanged, and that a jump
  opens the tab holding the anchor's block before scrolling.
- [ ] Amend DESIGN-0025 §11 to point at DESIGN-0024's new §5.
- [ ] `pkg/export/html`: export the outline as a value, for example
  `Outline(src []byte, spec kinds.HeadingSpec) ([]Tab, error)` with
  `Tab{Key, Label, Sections []Section}` and
  `Section{Kind, Component, Start, End}`, built from the same
  `outline()` and `layout` the renderer uses, so the two cannot differ.
  `Component` is a stable string (`text`, `goals`, `scope`, `checklist`,
  `last-verified`, `decision`, `consequences`, `alternatives`, `risks`,
  `criteria`, `phases`, `findings`, `verdict`, `procedures`,
  `scenarios`, `questions`). A golden over every corpus document pins it
  equal to the tabs the page renders.
- [ ] `docz inspect --format json` gains `outline` (Open Question 1): the
  tabs, each with its key, label, and sections. `api/cli/inspect.schema.json`
  takes an additive `$version` bump, coordinated with IMPL-0030 Phase 2's
  `theme` bump if both are open at once. `SchemaVersionInspect` stays 1.
- [ ] `docz inspect` text output prints the tab list under the regions.
- [ ] If Open Question 6 settles on Go-computed progress, `inspect` also
  reports `phases[]` (token, title, line, done, total, deferred, skipped)
  from `impl.Parse`, in `cmd/` beside the per-type validator switch, so
  `repo` still imports no type package.
- [ ] Regenerate the CLI contract fixtures and the review fixture
  (`just build && just ui gen-review-fixture`). `just ui gen-cli`
  regenerates the types.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- For every document in the six type corpora and the parity fixtures,
  `docz inspect`'s outline names the same tabs, in the same order, with
  the same sections as the page `docz export html` writes.
- `TestCLIJSONContractFixtures`, `cmd/review_fixture_test.go`, and
  `just ci` pass.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 2: Tabs on the review page

The structure, before any component: the panels hold today's rendering
of their regions, and everything the page did before still works through
the tabs.

<!--docz:tasks:start-->
#### Tasks

- [ ] `src/review/tabs.tsx`: a tab bar and panels from `inspect.outline`,
  with `role="tablist"`/`tab`/`tabpanel`, roving tabindex, arrow keys,
  Home and End, and `aria-controls`. Panels stay mounted and hidden
  with the `hidden` attribute (Open Question 2), so ranges, `data-line`
  lookups, and the highlight registry see every block.
- [ ] Split the review render into panels by the outline's line spans,
  after `regions.tsx` has wrapped them, so a region is never cut across
  two panels. A block the outline does not place goes to the tab of the
  nearest section above it, and a test proves no block is dropped.
- [ ] One `openBlock(line)` helper that opens the tab holding a line and
  scrolls to the block. Every jump goes through it: the findings list,
  the annotation list, question cards, changed-region badges, and in-page
  `#slug` links. A finding with no line opens Overview.
- [ ] Selections: the popover works in any open panel. A selection
  cannot span panels, and the UI says so rather than truncating, as it
  does for a cross-block replace.
- [ ] Badges on each tab: the number of findings, of annotations in the
  draft, of unanswered questions, and a "changed" mark from round two.
  The counts come from the same draft and diff state the sidebar reads.
- [ ] The tab that opens first, as Open Question 3 settles. The open tab
  per document is kept in the page's state, not in storage.
- [ ] The whole-document view, as Open Question 4 settles.
- [ ] Unit tests: the panel split (every block placed once), `openBlock`
  for each jump source, and the badges. Extend `src/a11y/axe.test.tsx`
  to the tabbed page.
- [ ] `ui/e2e/review.spec.ts`: annotate in a hidden tab via the
  annotation list's jump, then resolve and paint after a round, for a
  finding in a non-first tab, a question in its own tab, and a changed
  region's badge. Full-rule axe on every theme, as IMPL-0030 sets up.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- Every review action that worked on the one-column page works on the
  tabbed page: annotate, replace, delete, comment, answer, change
  status, mark a finding to fix, and approve. Each e2e passes with its
  target in a tab that was not open.
- Every anchor resolves and paints exactly as before, which the existing
  anchor suite and the round-two e2e both show.
- `just ui ci` and `just ui e2e` pass.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 3: The components that are layout only

The components that need no typed facts: each arranges the region's
rendered blocks into DESIGN-0025 §4's cards and callouts, with the
blocks and their `data-line` untouched inside.

<!--docz:tasks:start-->
#### Tasks

- [ ] `src/layout/` (Open Question 5): a component per §4 kind that
  takes a region's rendered children and their nested regions and
  returns the arrangement. It never re-renders markdown and never drops
  a block. A section whose component cannot find its shape falls back to
  `text`, as the export does.
- [ ] `goals` and `scope`: two cards, ok and neutral (or ok and bad).
- [ ] `consequences`: three cards, positive, negative, and neutral, from
  the nested regions.
- [ ] `alternatives`, `findings`, and `criteria` outside a phase: one card
  per subsection, by the headings inside the region, matching
  `kinds.Alternatives`/`kinds.Sections`/`kinds.Criteria` on the corpus.
- [ ] `decision`: its first block in a callout, the rest below. `verdict`:
  the `**Answer:**` line as the verdict.
- [ ] `risks`: the table with its likelihood and impact cells tinted.
  `checklist`: the list as open tasks. `last-verified`: a card.
- [ ] The CSS is the export's `components.css` classes, made available to
  the review bundle through the `theme` named context or a port into
  `tokens.css` (decided in the PR). Either way it uses tokens only, with
  no literal colour.
- [ ] Each component keeps `<section data-region>` and every block's
  `data-line`, so `regions.tsx`, anchors, and highlights are unaffected.
  A test per component asserts the multiset of `data-line` values in is
  the multiset out.
- [ ] e2e: annotate inside a card of each kind, then resolve and paint
  after a round.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- Each corpus document's review page shows the same cards and callouts
  as its exported page, tab for tab, checked by a test that reads the
  outline's components against what the page renders.
- No block is lost or duplicated by a component, and annotations anchor
  inside every component.
- `just ui ci` and `just ui e2e` pass.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 4: Phases, procedures, scenarios, and questions

The components built on facts: IMPL phase cards with progress, runbook
procedures and scenarios, and the questions tab, which is where the
review page and the export differ on purpose.

<!--docz:tasks:start-->
#### Tasks

- [ ] `phases`: a card per phase with its tasks and their state, a
  progress bar, deferred and skipped pills with their notes, the success
  criteria in an ok callout, and the all/open/done filter. Progress is
  counted as Open Question 6 settles. Tasks stay read-only.
- [ ] `procedures`: numbered steps, with commands in code blocks and
  rollbacks shown under their procedure. `scenarios`: one expandable per
  scenario.
- [ ] `questions`: the review page's answerable `QuestionCards` are the
  Questions tab's content, in place of the export's read-only
  expandables (Open Question 7). The sidebar keeps its list and jumps
  into the tab.
- [ ] Widgets from IMPL-0030 Phase 5 render inside whichever panel holds
  them, with no change here.
- [ ] Unit tests per component over the IMPL and runbook corpora: phase
  progress equal to the chosen rule, every step and command present,
  and every question answerable from its tab.
- [ ] e2e: filter phases, annotate a task's text, answer a question in
  its tab, and annotate inside an open scenario.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- An IMPL's review page shows phase cards whose progress matches the
  chosen rule, and a runbook's shows its procedures and scenarios as the
  export does.
- Every open question is answerable from the Questions tab, and the
  feedback a round writes is unchanged (the `feedback.ts` goldens pass
  untouched).
- `just ui ci` and `just ui e2e` pass.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 5: Close-out

<!--docz:tasks:start-->
#### Tasks

- [ ] Walk the review binary through an RFC, an ADR, a design, an IMPL,
  an investigation, and a runbook under every theme, and record
  screenshots in the PR.
- [ ] Update `ui/CLAUDE.md` (docz-review), `CLAUDE.md` (docz-review,
  `docz inspect`), and the plugin's README if the page it opens looks
  different.
- [ ] Mark DESIGN-0024's amendment implemented, and close #212 from the
  final PR.
- [ ] Regenerate the changelog as the last commit of every PR.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- Every task above is checked, #212 is closed, and the release that
  carries this plan is cut by its `minor` label.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:file-changes:start-->
## File Changes

| File | Action | Description |
| --- | --- | --- |
| `docs/design/0024-*.md`, `docs/design/0025-*.md` | Modify | The tabbed review page |
| `pkg/export/html/outline.go`, `layout.go` | Modify | `Outline`, `Tab`, `Section`, component names |
| `cmd/inspect.go` | Modify | `outline` (and `phases`, per Open Question 6) |
| `api/cli/inspect.schema.json` | Modify | Additive `outline` |
| `ui/src/review/tabs.tsx`, `document-review.tsx` | Create, Modify | Tab bar, panels, `openBlock` |
| `ui/src/review/{findings,annotations,questions,diff}.tsx` | Modify | Jumps through `openBlock`; badges |
| `ui/src/layout/*` | Create | The §4 components |
| `ui/src/theme/tokens.css` | Modify | Component classes, if ported |
| `ui/e2e/review.spec.ts` | Modify | Actions across tabs |

<!--docz:file-changes:end-->

<!--docz:testing:start-->
## Testing Plan

- [ ] Go: the `Outline` golden over every corpus document, equal to the
  tabs the renderer writes, and the CLI contract fixtures.
- [ ] Panel split: every block placed exactly once, for every corpus
  document.
- [ ] Components: `data-line` multisets preserved, shape per kind, and the
  fallback to `text`.
- [ ] Jumps: every jump source opens the right tab.
- [ ] Anchors: the existing suite is unchanged and passes, and the
  round-two e2e covers each component.
- [ ] Accessibility: the tabs pattern in jsdom axe, and full-rule axe in
  e2e on every theme.
- [ ] Feedback: the goldens are unchanged.

<!--docz:testing:end-->

<!--docz:dependencies:start-->
## Dependencies

- IMPL-0030 Phase 2 (theme from `.docz.yaml`, selector removed, and its
  `inspect` schema bump) and Phase 5 (widgets as React components).
- No new Go or npm dependency.

<!--docz:dependencies:end-->

<!--docz:open-questions:start-->
## Open Questions

### 1. Where does the review page get the outline from?

- a. `docz inspect` reports it, computed by an exported `Outline` in
  `pkg/export/html` from the same code the renderer uses. One layout
  definition, in Go, and the page cannot disagree with the export.
- b. Port `outline.go` and DESIGN-0025 §4's table to TypeScript, kept in
  step by a shared golden corpus, as IMPL-0030 Phase 5 does for the
  widgets.
- c. Embed the HTML `docz export html --stdout` writes. This is one
  renderer, but it loses the review render mode (`data-line`, region
  wrappers), the sanitizer, and the highlights, so anchoring breaks.

### 2. Do hidden panels stay mounted?

- a. Yes. Every panel is in the DOM with `hidden`, so `Range`s, the
  highlight registry, `data-line` lookups, and mermaid's measured layout
  all work, and a jump only flips an attribute. The cost is that a long
  document mounts everything, as it does today.
- b. Only the open panel is mounted. The first paint is faster, but every
  jump and every repaint has to remount, and highlights in other tabs
  would need recomputing.

### 3. Which tab opens first?

- a. Overview in round one. From round two, the first tab with a change,
  so a reviewer lands on what moved, with Overview as the fallback.
- b. Always the first tab.
- c. The tab holding the first finding, else Overview.

### 4. Is there a whole-document view?

- a. Yes. An "All sections" toggle shows every panel stacked, as an
  exported page does without JavaScript, for reading straight through or
  reading the round's diff in context. It is off by default.
- b. No. Tabs only.

### 5. Where do the layout components live?

- a. `ui/src/layout/`, outside `src/review`, so docz-site can adopt them
  later (Open Question 10) without moving them. The review page is
  their only user for now.
- b. `ui/src/review/layout/`, private to the review page until docz-site
  needs them.

### 6. How is IMPL phase progress counted?

- a. As the export counts it: `docz inspect` reports `phases[]` from
  `impl.Parse` (skipped tasks excluded from the count, deferred ones
  included), so the page and the exported page show the same number.
- b. In TypeScript from `inspect.tasks`, counting every checkbox, which
  is `docz inspect`'s existing count. This is simpler, but the number
  differs from the export's whenever a task is skipped.

### 7. What does the Questions tab hold?

- a. The review page's answerable question cards. The export's read-only
  expandables would hide the one thing a reviewer does with a question,
  so the two surfaces differ on purpose there.
- b. The export's expandables, with the answer controls inside each.
- c. Both: the expandables in the tab, and the cards in the sidebar only.

### 8. How are findings shown?

- a. As now: a findings panel above the tabs. Each finding jumps into its
  tab, and each tab's badge counts its findings.
- b. A Findings tab of its own.
- c. Inline in each panel, at the finding's line, with no list.

### 9. How do the phases release?

- a. Each phase is a `dont-release` PR, and Phase 5's is labelled `minor`,
  as in IMPL-0030.
- b. Phases 1 and 2 ship as a `minor` (tabs without components), then
  Phases 3 to 5 as another.

### 10. Does docz-site's reader adopt tabs too?

- a. Not in this plan. File a follow-up once the review page has shipped
  and the components in `src/layout/` have been used in anger.
- b. Yes, as a sixth phase, reusing the same components.

<!--docz:open-questions:end-->

<!--docz:references:start-->
## References

- [DESIGN-0024: docz-review](../design/0024-docz-review-a-blocking-review-page-for-agent-written-documents.md)
  §5 (the review page) and §7 (anchors)
- [DESIGN-0025: docz-html](../design/0025-docz-html-standalone-html-pages-from-docz-documents-and-one.md)
  §4 (layout by region kind) and §11 (the review page)
- [IMPL-0030](0030-one-theme-system-across-docz-html-docz-review-and-docz-site.md),
  Phases 2 and 5
- [IMPL-0027: docz-review](0027-docz-review-the-cli-contract-the-review-page-the-binary-and-the.md)
- [#212](https://github.com/donaldgifford/docz/issues/212)

<!--docz:references:end-->
