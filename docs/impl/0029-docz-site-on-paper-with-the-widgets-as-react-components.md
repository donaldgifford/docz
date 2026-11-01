---
id: IMPL-0029
title: "docz-site on paper, with the widgets as React components"
status: Cancelled
author: Donald Gifford
created: 2026-10-31
---

<!-- markdownlint-disable-file MD024 MD025 MD041 -->

# IMPL-0029: docz-site on paper, with the widgets as React components

<!--toc:start-->
<!-- markdownlint-capture -->
<!-- markdownlint-disable MD013 -->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: The site on paper](#phase-1-the-site-on-paper)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2: The widgets as React components](#phase-2-the-widgets-as-react-components)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 3: Close-out](#phase-3-close-out)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Dependencies](#dependencies)
- [References](#references)
<!-- markdownlint-restore -->
<!--toc:end-->

<!--docz:objective:start-->
## Objective

> **Cancelled 2026-11-01.** No phase was started. IMPL-0030 took its
> scope (Open Question 10): docz-site on `paper` with `classic` and
> `charcoal` selectable, the theme from `html.theme`, and the widgets as
> React components held to the export's validator by a shared corpus.
> The plan is kept as written.

Move docz-site to the `paper` theme with `classic` selectable, and draw
the three docz widgets (`docz-flow`, `docz-stepper`, `docz-scorecard`) in
the reader as React components from the same fenced YAML blocks the
export draws. This is rollout step 5 of DESIGN-0025 and the follow-up its
open question 10 settled.

**Implements:** [DESIGN-0025](../design/0025-docz-html-standalone-html-pages-from-docz-documents-and-one.md)

This is a **draft**: the phases are a first cut from what IMPL-0028 left
in place, to be reviewed before any of it starts.

<!--docz:objective:end-->

<!--docz:scope:start-->
## Scope

<!--docz:in-scope:start-->
### In Scope

- `ui/index.html` on `paper`, the theme selector and the light/dark
  toggle in the site's topbar, sharing `docz:theme` and `docz:mode` with
  the review page and exported pages.
- The review page's `src/review/appearance.ts` moved to `src/lib/` and
  shared by both entries; mermaid redrawn on a change through
  `refreshMermaidTheme`, as the review page does.
- Every site surface checked under paper light, paper dark, and classic:
  the contrast test already covers the tokens, so this is the e2e and axe
  sweep and anything that hardcodes a dark-only colour.
- The three widgets as React components, reading the closed YAML
  `pkg/export/html` validates (`plugins/docz-html/skills/docz-html/references/widgets.md`),
  drawn from tokens with `textContent`-equivalent rendering only, and an
  invalid block shown as its source with the error, as the export does.

<!--docz:in-scope:end-->

<!--docz:out-of-scope:start-->
### Out of Scope

- The paper radii (`--radius-card`, `--radius-control`) on site chrome:
  docz-site's wiped radius scale stays until a design decision says
  otherwise.
- New widgets, or widget fields the export does not read.
- docz-api: the widgets are markdown, so the API serves them already.

<!--docz:out-of-scope:end-->
<!--docz:scope:end-->

## Implementation Phases

Each phase builds on the previous one. A phase is complete when all its tasks
are checked off and its success criteria are met.

---

<!--docz:phase:start-->
### Phase 1: The site on paper

<!--docz:tasks:start-->
#### Tasks

- [ ] Move `src/review/appearance.ts` and `AppearanceControls` to
  `src/lib/` and `src/components/`; the review page imports them.
- [ ] Set `data-theme="paper"` on `ui/index.html` and apply the stored
  appearance in `src/main.tsx` before the first render.
- [ ] Add the selector and toggle to `AppShell`'s topbar, and redraw
  mermaid on a change.
- [ ] Find and replace any colour the site hardcodes outside the tokens
  (Shiki's contrast nudge reads `--color-code-bg`, which every theme keeps
  dark; check it still holds).
- [ ] Run `e2e/a11y.spec.ts` and the rendering specs under paper light,
  paper dark, and classic.
- [ ] Update the specimen page review and the screenshots, if any.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- The site defaults to `paper`, follows the system mode with no stored
  choice, and keeps the reader's choice across the site, the review page,
  and exported pages.
- Full-rule axe passes on every core view under all three settings.
- `just ui ci` passes; the PR is labelled `minor`.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 2: The widgets as React components

<!--docz:tasks:start-->
#### Tasks

- [ ] A remark step that marks `docz-flow`, `docz-stepper`, and
  `docz-scorecard` fences before Shiki, as `mermaid-marker` does for
  mermaid, so sanitize never sees component markup.
- [ ] Parse and validate each block against the same closed schema as
  `pkg/export/html/widgets.go`; share fixtures with its goldens so the two
  cannot disagree.
- [ ] `Flow`, `Stepper`, and `Scorecard` components drawn from tokens,
  redrawn on a mode change, with an invalid block shown as its source and
  the error.
- [ ] Extend the XSS suite to the widget path, and add a section per
  widget to the markdown specimen.
- [ ] Render the widgets in the review page too, since it shares the
  pipeline.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- Each widget draws in the reader, the review page, and an exported page
  from the same block, and each invalid fixture shows its source and
  error in all three.
- The XSS suite and the bundle budget pass; the widgets load lazily.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 3: Close-out

<!--docz:tasks:start-->
#### Tasks

- [ ] Update `ui/CLAUDE.md`, the root `CLAUDE.md`, and DESIGN-0025's
  rollout.
- [ ] Regenerate the changelog as the last commit of each PR.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- DESIGN-0025's rollout is complete.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:file-changes:start-->
## File Changes

| File | Action | Description |
| ---- | ------ | ----------- |
| `ui/index.html` | Modify | `data-theme="paper"` |
| `ui/src/lib/appearance.ts` | Create | Moved from `src/review/` |
| `ui/src/app/AppShell.tsx` | Modify | The selector and toggle |
| `ui/src/markdown/*` | Modify | The widget marker and components |
| `ui/e2e/*.spec.ts` | Modify | Three settings |

<!--docz:file-changes:end-->

<!--docz:testing:start-->
## Testing Plan

- [ ] Unit tests for the widget parser over `pkg/export/html`'s widget
  fixtures, valid and invalid.
- [ ] Component tests for each widget and its fallback.
- [ ] e2e and axe under paper light, paper dark, and classic.

<!--docz:testing:end-->

<!--docz:dependencies:start-->
## Dependencies

- IMPL-0028 merged and released: `pkg/theme`, the paper tokens that pass
  the contrast test, and the review page's appearance code.

<!--docz:dependencies:end-->

<!--docz:references:start-->
## References

- [DESIGN-0025](../design/0025-docz-html-standalone-html-pages-from-docz-documents-and-one.md),
  open question 10 and rollout step 5
- [IMPL-0028](0028-docz-html-pkgtheme-the-skill-docz-export-html-and-the-review.md)

<!--docz:references:end-->
