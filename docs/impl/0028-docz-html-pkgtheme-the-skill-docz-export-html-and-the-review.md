---
id: IMPL-0028
title: "docz-html: pkg/theme, the skill, docz export html, and the review page on paper"
status: Completed
author: Donald Gifford
created: 2026-10-31
---

<!-- markdownlint-disable-file MD024 MD025 MD041 -->

# IMPL-0028: docz-html: pkg/theme, the skill, docz export html, and the review page on paper

<!--toc:start-->
<!-- markdownlint-capture -->
<!-- markdownlint-disable MD013 -->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: pkg/theme and the ui plumbing](#phase-1-pkgtheme-and-the-ui-plumbing)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2: The skill, stage 1, and the widgets](#phase-2-the-skill-stage-1-and-the-widgets)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 3: Rendering one document](#phase-3-rendering-one-document)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 4: Layouts by region kind, typed facts, and widgets](#phase-4-layouts-by-region-kind-typed-facts-and-widgets)
    - [Tasks](#tasks-3)
    - [Success Criteria](#success-criteria-3)
  - [Phase 5: docz export html](#phase-5-docz-export-html)
    - [Tasks](#tasks-4)
    - [Success Criteria](#success-criteria-4)
  - [Phase 6: The review page on paper](#phase-6-the-review-page-on-paper)
    - [Tasks](#tasks-5)
    - [Success Criteria](#success-criteria-5)
  - [Phase 7: Close-out](#phase-7-close-out)
    - [Tasks](#tasks-6)
    - [Success Criteria](#success-criteria-6)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Dependencies](#dependencies)
- [Open Questions](#open-questions)
  - [1. How does the docz binary get mermaid's bytes?](#1-how-does-the-docz-binary-get-mermaids-bytes)
  - [2. How is a widget's YAML validated?](#2-how-is-a-widgets-yaml-validated)
  - [3. How does the stage-1 skill read the theme?](#3-how-does-the-stage-1-skill-read-the-theme)
  - [4. Where do Phase 2's design changes get recorded?](#4-where-do-phase-2s-design-changes-get-recorded)
  - [5. When is the docz-html/ source directory removed?](#5-when-is-the-docz-html-source-directory-removed)
  - [6. How large are the goldens?](#6-how-large-are-the-goldens)
  - [7. How does the fuzz test check that output is HTML?](#7-how-does-the-fuzz-test-check-that-output-is-html)
  - [8. How is the work split into pull requests?](#8-how-is-the-work-split-into-pull-requests)
  - [9. What does the eyebrow say?](#9-what-does-the-eyebrow-say)
- [References](#references)
<!-- markdownlint-restore -->
<!--toc:end-->

<!--docz:objective:start-->
## Objective

Build DESIGN-0025 up to and including the review page: one themeable
design for docz's HTML in `pkg/theme`, a Claude Code skill that iterates
on that design by hand before any renderer exists, `pkg/export/html` and
`docz export html` encoding what the skill settled, widgets authored in
markdown, and the docz-review page moved to the `paper` theme with a
light/dark toggle.

**Implements:** DESIGN-0025

The order is DESIGN-0025's rollout, which its question 10 fixed: the
theme first, then the skill as the fast loop for the design, then the Go
renderer, then the review page. docz-site follows in its own IMPL.

<!--docz:objective:end-->

<!--docz:scope:start-->
## Scope

<!--docz:in-scope:start-->
### In Scope

- `pkg/theme`: the `paper` and `classic` token files with light and dark
  values, `components.css`, `runtime.js`, and the Go accessors `Names`,
  `CSS`, and `Runtime`.
- `ui/` reading `pkg/theme`'s token files at build time, through Vite and
  through a `theme` named context in `Dockerfile.ui`.
- `plugins/docz-html/`: the skill, stage 1 (hand-assembled pages against
  the theme) and then stage 2 (instructions over `docz export html`),
  published in this repository's marketplace.
- The three widgets, `docz-flow`, `docz-stepper`, and `docz-scorecard`:
  fenced blocks with YAML bodies, drawn by the runtime from JSON islands.
- `pkg/export/html`: `Render` (one document, pure) and `Export` (a
  repository, through `repo`), the region-kind layout table, typed facts
  from the six type packages, links, images, the index page, and mermaid
  in three modes.
- `docz export html` with its JSON report in the CLI contract
  (`api/cli/export-html.schema.json`).
- The Confluence export showing a widget block's YAML in an expand
  (DESIGN-0025 question 15).
- The docz-review page on `paper` by default, `classic` selectable, the
  toggle, and contrast tests over both themes and modes.

<!--docz:in-scope:end-->

<!--docz:out-of-scope:start-->
### Out of Scope

- docz-site on `paper` and the widgets as React components (DESIGN-0025
  rollout step 5): a follow-up IMPL once this one is Completed (question
  6).
- Code highlighting in exported pages (DESIGN-0025 question 8).
- A `.docz.yaml` block for the export (DESIGN-0025 question 12).
- `docz validate` checking a widget's YAML.
- Ticking an IMPL task from a page (DESIGN-0025 question 5).
- docz-api serving exported pages.

<!--docz:out-of-scope:end-->
<!--docz:scope:end-->

## Implementation Phases

Each phase builds on the previous one. A phase is complete when all its tasks
are checked off and its success criteria are met. Every phase ends with
`just ci` green.

---

<!--docz:phase:start-->
### Phase 1: pkg/theme and the ui plumbing

The design lives once, as CSS and JS that Go embeds and `ui/` imports.
Both `ui/` pages select `classic`, so nothing anyone sees changes: this
phase proves the plumbing before the look moves.

<!--docz:tasks:start-->
#### Tasks

- [x] Add `pkg/theme/doc.go` with the package comment: what a theme is
  (token values only), the two themes, the `data-theme` and `data-mode`
  selection, and that the files are part of the semver contract by name
  and token, not by byte.
- [x] Port the source design's palette into `pkg/theme/tokens/paper.css`
  under `:root[data-theme="paper"]`, with the light values on the plain
  selector, the dark values under `[data-mode="dark"]`, and a
  `@media (prefers-color-scheme: dark)` block for no `data-mode`. Use
  `ui/`'s token names per DESIGN-0025 §3's mapping table; add
  `--color-ok`, `--color-link`, `--color-warn`, and `--color-bad`. Drop
  every "SSE" string and the framework catalogue's colours.
- [x] Move today's values out of `ui/src/theme/tokens.css` into
  `pkg/theme/tokens/classic.css` unchanged, dark only (its light block
  repeats the dark values so a forced `data-mode="light"` is still
  readable), and add the four new tokens with values that match the
  existing status colours.
- [x] Give both themes values for every status (`--color-st-*`), type
  (`--color-t-*`), admonition (`--color-adm-*`), and hash-palette token
  `tokens.css` declares today, and for the two review rules
  (`::highlight(docz-review)`, the changed-region mark).
- [x] Write `pkg/theme/components.css`: the header, pills, sticky tab row,
  panels, cards, callouts (note, tip, important, warning, caution), tables
  with horizontal scroll, phase cards with the progress bar, step lists,
  expandables, the "not written yet" line, code blocks (dark in both
  modes), and the print rules (every panel shown, tabs hidden). Colour
  only through tokens; no literal colour anywhere.
- [x] Write `pkg/theme/runtime.js` without the widgets: the mode toggle
  (`docz:mode`), tabs (click, `#p-<tab>` and `data-tab` links,
  ←/→/Home/End, overflow fade, last tab as `docz:<id>:tab`), copy buttons
  on code, the IMPL task filter (`docz:<id>:filter`), opening the
  expandable an in-page link points into, and a `docz:mode` event other
  code (mermaid, widgets) listens to. Every storage access in a `try`.
  Plain ES2020, no build step, no globals beyond one `docz` object.
- [x] Add `pkg/theme/theme.go`: `//go:embed` of the three kinds of file,
  `Names()` (`paper`, `classic`), `CSS(name)` (tokens then components;
  `ErrUnknownTheme` wrapped with the name and the valid names), and
  `Runtime()`.
- [x] Write `pkg/theme/theme_test.go`: every `var(--…)` in
  `components.css` and every `--…` `runtime.js` reads is defined by every
  theme in both modes; neither file contains a hex, `rgb(`, `hsl(`, or
  named colour; `CSS` for each name starts with that theme's selector;
  an unknown name is `ErrUnknownTheme`. `t.Parallel()` throughout.
- [x] Import the token files from `ui/src/theme/tokens.css` after its
  `@theme static` block (`@import "../../../pkg/theme/tokens/…"`), keep
  the `@theme` declarations so Tailwind still generates every utility,
  and set `data-theme="classic"` on `<html>` in `ui/index.html` and
  `ui/review.html`.
- [x] Add the repository root's `pkg/theme` to `server.fs.allow` in
  `ui/vite.config.ts`, beside Vite's default.
- [x] Add `theme = "pkg/theme"` to the `-ui` bake targets' `contexts` in
  `docker-bake.hcl`, and `COPY --from=theme . /pkg/theme` in
  `Dockerfile.ui`'s build stage, laid out so the relative import resolves
  as it does on disk.
- [x] Add `pkg/theme/**` to the `ui` path filter in
  `.github/workflows/ci.yml`. The `go` filter already covers it through
  `pkg/**`.
- [x] Run `just ui ci` and the review e2e; confirm the bundle budget and
  that the site and the review page render as before.
- [x] Add `pkg/theme` to `test/consumer`: `Names`, `CSS` for each, and
  `Runtime` non-empty, from an external module.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `go test ./pkg/theme/...` passes, including the completeness test, and
  removing one token from `paper.css` fails it naming the token.
- `just ui ci` passes, and a screenshot of the site home and the review
  page shows no change from `main`.
- `docker buildx bake` builds the `-ui` image with the theme context.
- `TestLayerRules_*` passes with `pkg/theme` importing only the standard
  library.
- `just ci` passes.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 2: The skill, stage 1, and the widgets

The quick loop for the design. The agent assembles pages by hand against
`pkg/theme`, a person looks at them, and the theme, the widgets, and the
layout table change in minutes. What this phase settles is what Phases 3
and 4 encode.

<!--docz:tasks:start-->
#### Tasks

- [x] Create `plugins/docz-html/` beside `plugins/docz-review/`:
  `.claude-plugin/plugin.json` (its own version, 0.1.0), `README.md`, and
  `skills/docz-html/SKILL.md`.
- [x] Add the plugin to `.claude-plugin/marketplace.json`.
- [x] Give the skill a copy of the theme it can read after a marketplace
  install, `skills/docz-html/theme/`, written by a new `just
  plugin-theme-sync` recipe, with a drift check in the `plugin` CI job
  (question 3).
- [x] Write `skills/docz-html/references/components.md`: the page
  skeleton (header, tab row, panels, footer), each component's markup and
  classes, and the JSON island contract for widgets.
- [x] Write `skills/docz-html/references/layout.md`: DESIGN-0025 §4's
  region-kind table and per-type tabs, as the instructions the agent
  follows to place each region.
- [x] Write the stage-1 `SKILL.md`: when to make a page; read the document
  with `docz inspect <id> --format json` for its regions, headings, tasks,
  and open questions rather than re-parsing the markdown; assemble the
  page from the references with the theme inlined; write it under
  `build/html/`; never edit the document to suit the page except to
  improve the markdown itself.
- [x] Define the three widgets' YAML in
  `skills/docz-html/references/widgets.md`, starting from DESIGN-0025
  §6's `docz-flow` example and the source skill's `SSE.initFlow`,
  `initStepper`, and `initScorecard` options, every field named,
  required or optional, and typed.
- [x] Add the widget drawers to `pkg/theme/runtime.js`: each finds its
  `<script type="application/json" data-widget="…">` island, draws into
  the sibling mount point in tokens, redraws on the `docz:mode` event,
  and leaves the no-JS fallback (a definition list or table of the same
  data) in place when the island fails to parse.
- [x] Add widget styles to `components.css` and re-run the Phase 1
  completeness test.
- [x] Iterate over this repository's own documents, one per built-in type
  (DESIGN-0025, IMPL-0027, ADR-0005, INV-0030, an RFC, RUNBOOK-0004),
  plus one unmarked document and one with each widget added on a scratch
  copy. Each change to the theme lands in `pkg/theme`; each change to the
  layout or a widget's YAML lands in the references.
- [x] Amend DESIGN-0025 §3, §4, and §6 with what the loop changed, each
  amendment dated (question 4).
- [x] Delete the `docz-html/` source directory from the working tree; it
  was never committed (question 5).
- [ ] Review the hand-assembled pages in a browser in both modes and
  both themes, and sign off the layout table and the widgets' YAML.
  **deferred** - human required

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `/plugin install docz-html@docz` installs the skill, and the skill
  produces a page for each built-in type that opens from `file://` with no
  network request and no console error.
- The three widgets draw in both themes and modes, redraw on a toggle, and
  fall back to readable data with JavaScript off.
- The layout table and the widgets' YAML are signed off and recorded in
  DESIGN-0025.
- The plugin CI job passes, including the theme drift check.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 3: Rendering one document

`pkg/export/html.Render`: generic rendering for any document, every tab a
region's markdown. Phase 4 adds the typed layouts on top.

<!--docz:tasks:start-->
#### Tasks

- [x] Add `pkg/export/html/doc.go` and `render.go` with the DESIGN-0025
  API: `MermaidMode`, `RenderOptions`, `Page`, `Link`, and
  `Render(src, opts)`. Reject CR line endings and a missing frontmatter
  block with sentinels, as `confluence.Render` does.
- [x] Resolve regions with `kinds.ResolveRegions(src, opts.Spec)`; set
  `Page.Inferred`, and say so in the header. A document with no regions
  gets one tab per H2.
- [x] Build the goldmark instance: GFM, footnotes, and a node renderer at
  priority 100 for headings (ids from `docparse.AnchorSlug` with
  github-slugger's duplicate rule, as `docparse.Headings` assigns them),
  GitHub alerts as callouts, task lists as disabled checkboxes, tables in
  a scroll wrapper, raw HTML escaped except `br`, `kbd`, `sub`, `sup`,
  `span`, `details`, `summary`, and comments and region markers dropped.
  Reuse nothing from `pkg/export/confluence` by import; copy the
  allow-list with a comment naming its twin.
- [x] Drop the ToC region; render the tab row and an in-panel heading
  list instead.
- [x] Write the page skeleton as an embedded `html/template`: doctype,
  `lang`, charset, viewport, the CSP `<meta>` of DESIGN-0025 §8,
  `data-theme` and no `data-mode`, the header (eyebrow from `Site` and the
  type's label, title, the summary region's first paragraph, pills for
  id, status, author, created), the tab row, the panels, and the footer
  (source path, content SHA-256, `Version`). Inline `theme.CSS` and
  `theme.Runtime` between `<!-- docz:theme -->` markers.
- [x] Render an empty region (only comments or whitespace) as the "not
  written yet" line.
- [x] Mermaid fences: `MermaidSource` writes a labelled code block;
  `MermaidEmbed` and `MermaidCDN` write a `<pre class="mermaid">` and set
  a flag the template reads to inline or link the script. The vendored
  file arrives in Phase 5; until then the embed mode is tested with a
  stub.
- [x] Compute `Page.Hash` over the body; write nothing time-dependent.
- [x] Goldens under `testdata/golden/`: `Render` over every `.md` in the
  six type packages' corpora and the parity fixtures, the theme block
  replaced by its markers in the golden (question 6), regenerated with
  `-update`; one full-page golden with the theme inlined.
- [x] Unit tests: escaping of every interpolated field, the raw-HTML
  allow-list, the CSP present, no `http` URL in a page outside CDN mode,
  determinism (two renders, same bytes), CR and no-frontmatter
  rejections.
- [x] `FuzzRender`: never panics, and the output parses with
  `golang.org/x/net/html` in the test only (question 7).
- [x] Add `pkg/export/html` to `test/consumer` with one inline document.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- Every corpus document renders, inferred or marked, and its golden
  parses as HTML.
- A document carrying `<script>` in prose, a heading, a title, or a
  frontmatter field renders it as text.
- `TestLayerRules_*` passes with no new third-party module in production
  code.
- `just ci` passes.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 4: Layouts by region kind, typed facts, and widgets

The layout DESIGN-0025 §4 tabulates and Phase 2 signed off, read from the
typed packages, plus the widgets from fenced blocks.

<!--docz:tasks:start-->
#### Tasks

- [x] Add `layout.go`: the region-kind table as data (kind → tab name,
  order, widget), tabs in the order their first region appears, an
  unknown kind as a tab named by its heading.
- [x] Pin the table: a test that every kind in
  `pkg/doczcore/validate`'s catalogue and in every embedded template has
  a row or is listed as deliberately generic, and that every built-in
  template yields DESIGN-0025's tab list for its type.
- [x] Typed facts, switching on region kind and never on type name: phases
  through `impl.Parse` (tasks with state, progress as
  `impl.Doc.Progress` counts it, deferred and skipped pills with their
  notes, criteria in an ok callout, the filter); procedures and scenarios
  through `runbook.Parse` (numbered steps with children, commands with
  copy buttons, `Expected`, verification, rollback in a caution callout,
  the Last Verified card); consequences through `adr.Parse`; the verdict
  through `investigation.Parse`; alternatives, risks, criteria, open
  questions, and decisions through `kinds`.
- [x] Open questions: one expandable per question, the recommended option
  marked, a resolved question closed with its resolution shown.
- [x] A typed parse error falls back to the region's markdown and is
  appended to `Page.Problems`; it never fails `Render`.
- [x] Widgets: a `docz-flow`, `docz-stepper`, or `docz-scorecard` fence
  is decoded with yaml.v3 `KnownFields(true)` into the Go type for that
  widget (question 2), validated, and written as the JSON island, the
  mount point, and the no-JS fallback; an invalid block renders as its
  source with the error and is recorded with its line in `Page.Problems`.
- [x] Goldens for the typed layouts: one per type from the corpora, one
  custom type carrying IMPL regions (it gets phases), one per widget, one
  per widget error.
- [x] Confluence: a widget fence in `pkg/export/confluence/nodes.go`
  renders as a collapsed expand titled with the widget's name (`Flow`,
  `Stepper`, `Scorecard`) holding the YAML in a code macro, with a
  golden.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- Every built-in type's golden shows the tabs DESIGN-0025 §4 lists.
- Adding a region kind to a template without a table row fails the pin
  test.
- An IMPL page's progress equals `docz inspect`'s task counts for the
  same document.
- Each widget golden carries its island and fallback; each error golden
  carries the source and the line.
- `just ci` passes.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 5: docz export html

The repository operation, the command, its contract, mermaid's bytes,
and the browser test. The skill moves to stage 2. This is the `minor`.

<!--docz:tasks:start-->
#### Tasks

- [x] `export.go` in the package: `ExportOptions`, `Report` (per page:
  id, path, action `wrote`/`unchanged`/`failed`, hash, unresolved links,
  problems), and `Export(ctx, rp, opts)`, listing through `repo.List` and
  `repo.Find`, resolving each document's spec through `repo.Template`,
  checking `ctx` between documents, and writing only under `Out`
  (`build/html` relative to the root when empty), mirroring each type's
  directory.
- [x] "Unchanged": compare the render with the bytes on disk and skip the
  write when equal.
- [x] Hooks `{PageDone}` in the `WithHooks`/`HooksFrom` shape.
- [x] Links: `#slug` within the page; a relative link to a document this
  run exports becomes a relative link to its page; anything else goes
  through `RenderOptions.Resolve`, and an unresolved link is reported.
- [x] Images: a relative image under the root of 1 MB or less becomes a
  `data:` URL with its MIME type from the extension; a larger one goes
  through the resolver. A path outside the root is never read.
- [x] `index.html` when a run writes more than one page: one tab per type,
  a table of id, title, status, and date in `docz list` order.
- [x] Mermaid: `pkg/export/html/assets/mermaid.min.js.gz`, written by a
  new `just vendor-mermaid` recipe from `ui/node_modules` at the version
  `ui/bun.lock` pins, with a test that the embedded version string equals
  the lockfile's (question 1). `MermaidEmbed` inlines it only in a page
  with a diagram; `MermaidCDN` links jsDelivr at that version with an
  SRI hash and adds the origin to the CSP. The runtime re-renders
  diagrams on the `docz:mode` event and shows the source and error for a
  diagram that fails to parse.
- [x] `cmd/export.go`: `exportHTMLCmd` beside `exportConfluenceCmd`, with
  `--out`, `--stdout`, `--theme`, `--mermaid`, `--format`, `--dry-run`;
  arguments resolved as `export confluence` resolves them; `blobResolver`
  as the resolver; `Site` from the GitHub remote's repository name;
  `Version` from the build. Exit 0, 1 when a page failed, 2 for a usage
  or config error (unknown or disabled type, unknown theme or mermaid
  mode, no such document, `--stdout` with more than one page).
- [x] The text report (one line per page and a total) and the JSON
  report; `api/cli/export-html.schema.json`, closed, `schema_version` 1,
  added to `Names` with `SchemaVersionExportHTML`, and the cmd contract
  test running it over the parity fixtures.
- [x] `cmd/export_html_test.go`: each exit code, `--stdout`, `--dry-run`
  writing nothing, a second run reporting every page unchanged.
- [x] `ui/e2e/export-html.spec.ts` in the `ui-e2e` job: export the parity
  fixtures with the docz the job builds, open one page per type from
  `file://`, click every tab, toggle the mode, filter IMPL tasks, expand
  an open question, draw each widget, and assert no console error and no
  network request.
- [x] Rewrite `SKILL.md` for stage 2 per DESIGN-0025 §10 and delete the
  skill's theme copy, its sync recipe, and the drift check; bump the
  plugin to 0.2.0.
- [x] Document the command in `README.md`, `docs/` usage, `CLAUDE.md`
  (the package, the command, the plugin), and the package's godoc.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `docz export html` over this repository writes a page per document and
  an index that open offline with no console error.
- A second run reports every page unchanged and writes nothing.
- The JSON report validates against `export-html.schema.json`.
- The Playwright spec passes in CI.
- `docz` grows by no more than about 2 MB with mermaid embedded.
- `just ci` passes, and the PR is labelled `minor`.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 6: The review page on paper

The docz-review page moves to `paper` with the toggle; `classic` stays
selectable. Its own `minor`.

<!--docz:tasks:start-->
#### Tasks

- [x] Set `data-theme="paper"` on `ui/review.html`; docz-site stays on
  `classic`.
- [x] Add the light/dark toggle to the review page's header, sharing
  `docz:mode` with exported pages, and a theme selector storing
  `docz:theme`.
- [x] Re-render the review page's mermaid diagrams on a mode change
  through `mermaidThemeFromTokens`.
- [x] Extend `ui/src/theme/contrast.test.ts` to every theme and mode, and
  fix any pair below its threshold in the token files.
- [x] Run the review e2e under `paper` light, `paper` dark, and `classic`.
- [x] Rebuild the review binary (`just ui build-review`) and walk through
  a review in each.
- [x] Bump the docz-review plugin version if its README shows the look.
  It does not (no screenshot or description of the page), so 0.2.0 stays.
- [ ] Run a review on the new theme in a real session before the
  release. **deferred** - human required

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- The review page defaults to `paper`, follows the system mode with no
  stored choice, and keeps the reader's choice across sessions.
- The contrast test passes for every theme and mode.
- The review e2e passes under all three settings.
- `just ci` passes, and the PR is labelled `minor`.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 7: Close-out

<!--docz:tasks:start-->
#### Tasks

- [x] Set DESIGN-0025 to Implemented and this IMPL to Completed.
- [ ] Open the follow-up for docz-site on `paper` with the widgets as
  React components, as an issue and a draft IMPL. The draft is IMPL-0029;
  the issue is outward-facing and waits for the owner. **deferred** -
  human required
- [ ] Regenerate the changelog as the last commit of each PR.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- Both documents carry their final status, and the site follow-up is
  filed.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:file-changes:start-->
## File Changes

| File | Action | Description |
| ---- | ------ | ----------- |
| `pkg/theme/{doc,theme,theme_test}.go` | Create | Embeds and accessors, completeness tests |
| `pkg/theme/tokens/{paper,classic}.css` | Create | The two themes' token values |
| `pkg/theme/{components.css,runtime.js}` | Create | Components and runtime, widgets included |
| `ui/src/theme/tokens.css` | Modify | Values move to `classic.css`; imports both themes |
| `ui/{index,review}.html` | Modify | `data-theme` |
| `ui/vite.config.ts` | Modify | `server.fs.allow` gains `pkg/theme` |
| `Dockerfile.ui`, `docker-bake.hcl` | Modify | The `theme` named context |
| `.github/workflows/ci.yml` | Modify | `pkg/theme/**` in the `ui` filter |
| `plugins/docz-html/**` | Create | The plugin and skill |
| `.claude-plugin/marketplace.json` | Modify | The second plugin |
| `pkg/export/html/*.go`, `testdata/**`, `assets/mermaid.min.js.gz` | Create | The renderer and export |
| `pkg/export/confluence/nodes.go` | Modify | Widget fences as expands |
| `cmd/export.go`, `cmd/export_html_test.go` | Modify, Create | The command |
| `api/cli/export-html.schema.json`, `api/cli/schemas.go` | Create, Modify | The report contract |
| `ui/e2e/export-html.spec.ts` | Create | The browser test |
| `ui/src/theme/contrast.test.ts`, the review header | Modify | Themes, modes, the toggle |
| `test/consumer/consumer_v2_html_test.go` | Create | `pkg/theme` and `pkg/export/html` from outside |
| `docz.just` | Modify | `vendor-mermaid`, then `plugin-theme-sync` for Phase 2 only |
| `docs/design/0025-*.md` | Modify | Amendments from Phase 2, final status |

<!--docz:file-changes:end-->

<!--docz:testing:start-->
## Testing Plan

- [ ] `pkg/theme`: token completeness across themes and modes, no literal
  colours, accessors.
- [ ] `pkg/export/html`: goldens over every corpus document and the
  parity fixtures, typed-layout and widget goldens, the kind-table pin,
  escaping and CSP units, `FuzzRender` with an HTML parse in the test.
- [ ] `cmd`: exit codes, `--stdout`, `--dry-run`, unchanged on rerun, the
  JSON report against its schema.
- [ ] `pkg/export/confluence`: the widget expand golden.
- [ ] `test/consumer`: both new packages from an external module.
- [ ] `ui`: contrast over every theme and mode; the review e2e under three
  settings; the export Playwright spec.
- [ ] Plugin: the theme drift check while stage 1 lasts.

<!--docz:testing:end-->

<!--docz:dependencies:start-->
## Dependencies

- DESIGN-0025, all fifteen questions resolved on 2026-10-31.
- goldmark, already a dependency of `pkg/export/`; no new third-party
  module in production code. `golang.org/x/net/html` in tests only
  (question 7).
- `ui/`'s pinned mermaid 12.x for the vendored file.
- docz-review's e2e harness and the `ui-e2e` job, which already builds
  docz.

<!--docz:dependencies:end-->

<!--docz:open-questions:start-->
## Open Questions

### 1. How does the docz binary get mermaid's bytes?

- **a. Committed, gzipped, at `pkg/export/html/assets/`, written by `just
  vendor-mermaid` from `ui/node_modules` at the lockfile's version, with a
  test that the two versions agree.** *(recommendation)* `go install`
  keeps working, and Renovate's bump of `ui/` fails the test until the
  file is re-vendored.
- b. The recipe downloads the tarball from the npm registry at the
  lockfile's version instead of reading `node_modules`.
- c. Not committed: generated at build time by `go generate`, which
  breaks `go install` from a tag.
- d. Other.

> **Resolved 2026-10-31: (a).**

### 2. How is a widget's YAML validated?

- **a. A Go struct per widget, decoded with yaml.v3 `KnownFields(true)`
  plus field checks, in `pkg/export/html`.** *(recommendation)* No new
  dependency; the site IMPL writes TypeScript types from the same
  reference.
- b. JSON Schema files in `pkg/theme/widgets/` as the source of truth,
  validated in Go with `santhosh-tekuri/jsonschema`, which the
  third-party allow-list would then permit under `pkg/export/`, and read
  by the site later.
- c. Both: the structs validate, and a test generates the schemas the
  site will read.
- d. Other.

> **Resolved 2026-10-31: (a).**

### 3. How does the stage-1 skill read the theme?

- **a. A copy inside the plugin, synced by `just plugin-theme-sync` with
  a drift check in the plugin CI job, removed at stage 2.**
  *(recommendation)* A marketplace install copies only the plugin's
  directory, so a relative path to `pkg/theme` would not resolve.
- b. A hidden `docz theme css|runtime` command shipped in Phase 1, so the
  skill never carries a copy; it needs a docz release before the skill
  works anywhere.
- c. Stage 1 only works inside this repository and reads `pkg/theme` by
  path.
- d. Other.

> **Resolved 2026-10-31: (a).**

### 4. Where do Phase 2's design changes get recorded?

- **a. Amend DESIGN-0025 in place, each change dated, as DESIGN-0024 was
  amended.** *(recommendation)*
- b. A section in this IMPL, leaving DESIGN-0025 as approved.
- c. Other.

> **Resolved 2026-10-31: (a).**

### 5. When is the `docz-html/` source directory removed?

- **a. At the end of Phase 2, once the skill's references carry
  everything the loop needs; never committed.** *(recommendation)*
- b. At the end of Phase 1, once the palette is ported.
- c. Other.

> **Resolved 2026-10-31: (a).** The directory is never committed; it stays untracked
> until the end of Phase 2 and is deleted then.

### 6. How large are the goldens?

- **a. The inlined theme is cut to its `<!-- docz:theme -->` markers in
  every golden but one full-page golden.** *(recommendation)* A theme
  change then touches one golden, not a hundred.
- b. Every golden is a full page.
- c. Goldens hold only the `<main>` element.
- d. Other.

> **Resolved 2026-10-31: (a).**

### 7. How does the fuzz test check that output is HTML?

- **a. `golang.org/x/net/html` in the test only, outside the production
  import graph.** *(recommendation)*
- b. A hand-written tag-balance check, no new module.
- c. Other.

> **Resolved 2026-10-31: (a).**

### 8. How is the work split into pull requests?

- **a. One PR per phase. Phases 1 to 4 are `dont-release`, Phase 5 is
  `minor`, Phase 6 is `minor`, and Phase 7 rides on Phase 6.**
  *(recommendation)* Each phase ends green.
- b. Phases 1 and 2 together, then 3 to 5, then 6.
- c. One PR for Phases 1 to 5, then 6.
- d. Other.

> **Resolved 2026-10-31: (a).**

### 9. What does the eyebrow say?

- **a. The repository's name from the GitHub remote, then the type's
  label: `docz · Design`.** *(recommendation)* Falls back to the root's
  base name.
- b. The type's label only.
- c. A `--site` flag, defaulting to (a).
- d. Other.

> **Resolved 2026-10-31: (a).**

<!--docz:open-questions:end-->

<!--docz:references:start-->
## References

- [DESIGN-0025](../design/0025-docz-html-standalone-html-pages-from-docz-documents-and-one.md),
  the design this implements
- [IMPL-0027](0027-docz-review-the-cli-contract-the-review-page-the-binary-and-the.md),
  docz-review, whose page Phase 6 moves and whose CLI contract Phase 5
  extends
- [IMPL-0023](0023-confluence-export-the-library-the-cli-and-this-repositorys-scratch.md),
  the Confluence export whose shape `pkg/export/html` follows

<!--docz:references:end-->
