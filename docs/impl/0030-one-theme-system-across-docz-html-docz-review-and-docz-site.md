---
id: IMPL-0030
title: "One theme system across docz-html, docz-review, and docz-site, with the widgets in the reader"
status: Draft
author: Donald Gifford
created: 2026-11-01
---

<!-- markdownlint-disable-file MD024 MD025 MD041 -->

# IMPL-0030: One theme system across docz-html, docz-review, and docz-site, with the widgets in the reader

<!--toc:start-->
<!-- markdownlint-capture -->
<!-- markdownlint-disable MD013 -->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: The theme contract and the .docz.yaml key](#phase-1-the-theme-contract-and-the-doczyaml-key)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2: docz export html and docz-review read it](#phase-2-docz-export-html-and-docz-review-read-it)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 3: docz-site on the theme system](#phase-3-docz-site-on-the-theme-system)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 4: charcoal, a third theme in repo-guardian's design](#phase-4-charcoal-a-third-theme-in-repo-guardians-design)
    - [Tasks](#tasks-3)
    - [Success Criteria](#success-criteria-3)
  - [Phase 5: The widgets in the reader and the review page](#phase-5-the-widgets-in-the-reader-and-the-review-page)
    - [Tasks](#tasks-4)
    - [Success Criteria](#success-criteria-4)
  - [Phase 6: Close-out](#phase-6-close-out)
    - [Tasks](#tasks-5)
    - [Success Criteria](#success-criteria-5)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Dependencies](#dependencies)
- [Open Questions](#open-questions)
  - [1. Where does the key live in .docz.yaml?](#1-where-does-the-key-live-in-doczyaml)
  - [2. Which package checks the value against pkg/theme.Names()?](#2-which-package-checks-the-value-against-pkgthemenames)
  - [3. What does an unknown theme name do?](#3-what-does-an-unknown-theme-name-do)
  - [4. What does the review page do with a docz too old to report the theme?](#4-what-does-the-review-page-do-with-a-docz-too-old-to-report-the-theme)
  - [5. How does docz-site learn the list of themes?](#5-how-does-docz-site-learn-the-list-of-themes)
  - [6. What is the third theme called, and how does it get its fonts?](#6-what-is-the-third-theme-called-and-how-does-it-get-its-fonts)
  - [7. Does docz-site adopt the themes' radii?](#7-does-docz-site-adopt-the-themes-radii)
  - [8. How do the phases release?](#8-how-do-the-phases-release)
  - [9. How are the TypeScript widget rules kept in step with widgets.go?](#9-how-are-the-typescript-widget-rules-kept-in-step-with-widgetsgo)
  - [10. What happens to IMPL-0029?](#10-what-happens-to-impl-0029)
- [References](#references)
<!-- markdownlint-restore -->
<!--toc:end-->

<!--docz:objective:start-->
## Objective

Make `pkg/theme` the one theme system for docz's three HTML surfaces, so a
theme added there works on all of them and the theme a surface shows is
set where it belongs. That place is the repository's `.docz.yaml` for
`docz export html` and docz-review, and the deployment for docz-site.
Then add a third theme in repo-guardian's design, and draw the three docz
widgets in docz-site's reader and the review page as React components.

**Implements:** [DESIGN-0025](../design/0025-docz-html-standalone-html-pages-from-docz-documents-and-one.md)
§3 and rollout step 5, amended in Phase 1. Tracks
[#214](https://github.com/donaldgifford/docz/issues/214) (Phases 1 to 3),
[#213](https://github.com/donaldgifford/docz/issues/213) (Phase 4), and
[#209](https://github.com/donaldgifford/docz/issues/209) (Phase 5). It
takes over the draft
[IMPL-0029](0029-docz-site-on-paper-with-the-widgets-as-react-components.md):
its Phase 1 is this document's Phase 3, and its Phase 2 is Phase 5.

**What already holds, and is not rebuilt here:**

- `pkg/theme` is the one source of tokens. `docz export html` inlines a
  theme, the review page imports both token files, and docz-site's
  `src/theme/tokens.css` imports them too, with its `@theme` block pinned
  to `classic` by `TestClassicMatchesUITokens`.
- Every colour token docz-site uses is defined by `paper` as well
  (checked against `tokens.css` while writing this plan). Putting the
  site on paper is a matter of the attribute and the surfaces that
  hardcode a colour, not a new token set.
- `ui/src/theme/contrast.test.ts` already checks every text token in
  every theme and mode at 4.5:1 on the three surfaces.
- `ui/src/review/appearance.ts` and `AppearanceControls` hold a theme and
  a mode, and `refreshMermaidTheme` redraws mermaid on a change.

**What changes:**

- An `html.theme` key in `.docz.yaml` sets the theme for `docz export html` and
  docz-review. `--theme` overrides it for one export. Neither surface
  offers a reader a theme selector, since the repository has made the
  choice. Both keep the light/dark toggle, because that is the reader's
  preference.
- docz-site never reads a repository's `.docz.yaml` for its theme,
  because one site shows many repositories. Its default comes from the
  deployment (`DOCZ_THEME`, the chart's `site.config.theme`), and a
  reader may change it with a selector in the topbar.
- `pkg/theme.Names()` is the one list every theme check and every
  selector uses. A theme added to `pkg/theme` needs no change on any
  surface to be usable.
- A choice stored in `localStorage` is per origin, so it never travels
  between surfaces. That is accepted, not worked around: docz-site is one
  origin, each review session is `127.0.0.1` on a random port, and an
  exported page is `file://`.

<!--docz:objective:end-->

<!--docz:scope:start-->
## Scope

<!--docz:in-scope:start-->
### In Scope

- Amending DESIGN-0025 §3 and DESIGN-0024 (the review page) with the
  theme contract.
- The `html:` block and its `theme` key in `pkg/doczcore/config`, written by `docz init`,
  printed by `docz config`, served in docz-api's `config_snapshot` by its
  json tag, and dropped from the parity comparison by a normaliser.
- `docz export html` taking its default theme from the key.
- `docz inspect --format json` reporting the repository's theme, and the
  review page rendering in it with its theme selector removed.
- docz-site on `paper` by default with `classic` selectable: the runtime
  setting, the chart value, the topbar controls, and mermaid and Shiki on
  every theme.
- A third theme in repo-guardian's design (repo-guardian DESIGN-0036),
  usable on all three surfaces.
- `docz-flow`, `docz-stepper`, and `docz-scorecard` drawn in docz-site's
  reader and the review page as React components, from the same fenced
  YAML that `pkg/export/html/widgets.go` validates.

<!--docz:in-scope:end-->

<!--docz:out-of-scope:start-->
### Out of Scope

- The review page's layout (#212). It is DESIGN-0024's, still has design
  questions open, and builds on Phase 5's components, so it comes after
  this plan.
- A theme selector on an exported page. Settled on #214: an exported page
  carries the one theme it was exported with.
- Carrying a reader's choice between surfaces. `localStorage` is per
  origin, and nothing here works around that.
- docz-site's wiped radius scale. `--radius-card` and `--radius-control`
  apply to exported pages and the review page. Whether the site adopts
  them was Open Question 7, settled no.
- New widgets, or widget fields the export does not read.
- docz-api behaviour. The `html:` block reaches `config_snapshot` through
  its json tag and nothing on the server reads it. The widgets are
  markdown, so the API serves them already.

<!--docz:out-of-scope:end-->
<!--docz:scope:end-->

## Implementation Phases

Each phase builds on the previous one. A phase is complete when all its tasks
are checked off and its success criteria are met. Each phase is one PR
labelled `dont-release`, and the last is labelled `minor` (Open Question 8).
The open questions are settled; each answer is recorded under its question.

---

<!--docz:phase:start-->
### Phase 1: The theme contract and the `.docz.yaml` key

The design amendment, then the library half: a `theme` value every later
phase reads, validated against `pkg/theme`, and written into every new
repository.

<!--docz:tasks:start-->
#### Tasks

- [x] Amend DESIGN-0025 §3 with the contract above: defaults by surface,
  no theme selector where the repository decides, the light/dark toggle
  kept, `pkg/theme.Names()` as the one list, and per-origin storage
  accepted. Record the answers to this plan's open questions there.
- [x] Amend DESIGN-0024 so the review page renders in the repository's
  theme, and drop the theme selector IMPL-0028 Phase 6 added.
- [x] `pkg/doczcore/config`: a new `HTMLConfig{Theme string}` in `html.go`,
  on `Config` as `HTML` (yaml and json tags `html` and `theme`, so
  `TestJSONTags_MirrorYAML` passes), the place left for later page
  settings (Open Question 1). `DefaultConfig()` sets `HTML.Theme` to
  `paper`.
- [x] `normalizeHTML`, run on both `Load` paths and on `ParseBytes`
  after `normalizeAccess`: trim, lower-case, and backfill an empty value
  to `paper`.
- [x] `validateHTML`, called from `Validate()`, checks the value against
  `pkg/theme.Names()`. `config` imports `pkg/theme` (stdlib only), and
  `layer_test.go` allows that one edge by name, with a comment saying why
  (Open Question 2). An unknown name is a validation **warning**, and the
  value falls back to `paper`, so docz-api never refuses a repository's
  ingest over it (Open Question 3). `cmd/` prints the warning as it
  prints the others.
- [x] `docz_yaml.tmpl`: write `html:` with `theme: paper` and a comment that names
  the themes and says it sets `docz export html` and docz-review, not
  docz-site. `DefaultConfigYAML()` and `docz init` pick it up.
- [x] `test/parity`: an `HTMLNormalizer` that drops the `html:` block and
  its comment from a generated `.docz.yaml` and from `docz config`
  (`dropTopLevelBlock`, as `AccessNormalizer` does), in `runCase`'s
  both-sides list, with a unit test in `parity_norm_test.go`. v1.2.2
  never wrote the block, so it is a permitted additive delta.
- [x] Tests: the default, normalisation (`" Classic "` becomes
  `classic`), an unknown value (a warning and `paper`), `ParseBytes` equal to
  `Load` (`parsebytes_test.go`), and `TestConfigJSON_MarshaledShape`
  updated for the new field.
- [x] `test/consumer`: assert `DefaultConfig().Theme`, and that
  `ParseBytes` reads the key, through the public path.
- [x] This repository's own `.docz.yaml` gains `html: {theme: paper}`, and
  `test/archive` still passes.
- [x] Update `CLAUDE.md` (the config bullet and the parity list).

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- A fresh `docz init` writes `html:` with `theme: paper`, `docz config` prints it, and
  `docz validate --strict` on a fresh repository is still clean.
- A repository with no `html:` block loads as `paper`, and a key naming
  no theme loads as `paper` with one validation warning.
- `just parity`, `just test-consumer`, and `just lint-generated` pass,
  and so does `just ci`.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 2: `docz export html` and docz-review read it

The two surfaces the repository decides for. After this phase the
repository's key changes the theme of every exported page and every
review session, and neither surface offers a theme selector.

<!--docz:tasks:start-->
#### Tasks

- [x] `cmd/export_html.go`: `--theme` defaults to empty, which means
  `Cfg.HTML.Theme`. An explicit `--theme` still wins and is still checked
  against `theme.Names()` (exit 2). The `--help` text says where the
  default comes from.
- [x] `pkg/export/html`: `ExportOptions.Theme` empty means the
  repository's `Cfg.HTML.Theme`, so a consumer calling `Export` without the
  CLI gets the same default. A test covers both paths.
- [x] The `export-html` JSON report gains `theme` (the theme the run
  used). `api/cli/export-html.schema.json` takes an additive `$version`
  bump, and the contract fixtures are regenerated with `-update`.
- [x] `docz inspect --format json` gains a top-level `theme`, the
  repository's resolved theme. `api/cli/inspect.schema.json` takes an
  additive `$version` bump; `SchemaVersionInspect` stays 1. The review
  fixture is regenerated (`just build && just ui gen-review-fixture`) so
  `cmd/review_fixture_test.go` passes.
- [x] docz-review server (`ui/review/`): pass the inspected theme to the
  page in the session payload. A docz too old to report one means
  `paper`, with no refusal and no `SUPPORTED.minDocz` bump (Open
  Question 4).
- [x] Review page: `main.tsx` sets `data-theme` from the session before
  the first render. Delete the theme half of `appearance.ts`
  (`THEMES`, `storedTheme`, `setTheme`, the `docz:theme` key) and the
  selector in `AppearanceControls`. The light/dark toggle stays, shown
  on every theme that has a dark block, which is `paper` today.
- [x] `ui/e2e/review.spec.ts`: open a session per theme by giving the
  fixture repository a different `theme` (replacing the `docz:theme`
  init scripts), with full-rule axe on each. The e2e that stored a theme
  is removed.
- [x] `ui/e2e/export-html.spec.ts`: a fixture with `html: {theme: classic}`
  exports classic pages with no `--theme`.
- [x] Plugin `docz-html`: `SKILL.md` says the theme comes from
  `.docz.yaml`, with `classic` (or `--theme`) for a one-off, and the
  `argument-hint` follows. Version to 0.3.0. The eval case
  `docz-html-exp-002` still passes, and a case where the fixture's key
  picks the theme is added.
- [x] Update `ui/CLAUDE.md` (Appearance) and `CLAUDE.md` (docz-html,
  docz-review).

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- With `html: {theme: classic}` in `.docz.yaml`, `docz export html` writes
  classic pages with no flag, `--theme paper` overrides it, and the JSON
  report names the theme used.
- A review session in that repository renders in classic, with no theme
  selector, and the light/dark toggle works on paper.
- `just ui ci`, `just ui e2e`, `just plugin-eval docz-html`, and
  `just ci` pass.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 3: docz-site on the theme system

The surface whose default comes from its deployment. `paper` is the
default, the reader may pick any theme in `pkg/theme`, and every
surface is checked under each.

<!--docz:tasks:start-->
#### Tasks

- [x] Move the mode half of `src/review/appearance.ts` to
  `src/lib/appearance.ts`, shared by the review page and the site. The
  site adds a theme half that the review page does not import: stored
  `docz:theme`, then the injected default, then `paper`.
- [x] `scripts/gen-themes.ts` writes `src/theme/__generated__/themes.ts`
  (gitignored, like the CLI types) from the file names under
  `pkg/theme/tokens/`, run before `typecheck`, `test`, and `build` and
  checked by a `gen-themes-check` in `just ui ci`. `Dockerfile.ui`
  already has `pkg/theme` as the `theme` named context (Open Question 5).
- [x] `server/serve.ts`: `resolveTheme(DOCZ_THEME)` validated against that
  generated list, into `__DOCZ_CONFIG__.theme`. An unknown
  value falls back to `paper` and is reported by `invalidConfigVars`, so
  readiness fails on a typo as it does for the other whitelisted
  variables. `src/lib/theme.ts` re-validates on read (both ends, as the
  nav pins do), then `VITE_THEME`, then `paper`.
- [x] `index.html` keeps a `data-theme` for the no-JavaScript first
  paint, and `src/main.tsx` applies the resolved theme and stored mode
  before the first render.
- [x] `AppShell` topbar: a labelled theme selector over the theme list
  and the light/dark toggle, between the nav pins and `SessionMenu`. The
  toggle shows only on a theme with a dark block. A change redraws
  mermaid through `refreshMermaidTheme`.
- [x] Shiki's contrast nudge: `theme-contrast.ts` pins `CODE_BG` to
  classic's `--color-code-bg`, and paper's is darker (`#0f1419`). Check
  every theme's `--color-code-bg` in `theme-contrast.test.tsx` (read from
  `pkg/theme/tokens`), so a theme whose code surface is lighter fails
  there.
- [x] Find and replace colours the site hardcodes outside the tokens.
  `mermaid-block.tsx` falls back to classic hex when a token is
  unreadable, which is fine as a fallback, but no rendered surface may
  depend on it.
- [x] Chart: `site.config.theme` (enum in `values.schema.json` from the
  theme list, empty by default) renders `DOCZ_THEME` only when set;
  helm-unittest for both, and `just chart docs`.
- [x] `e2e/a11y.spec.ts` and the rendering specs run under every theme
  and, on paper, both modes. Full-rule axe passes on every core view.
- [ ] The markdown specimen is checked under each theme, and any
  screenshot in a spec is regenerated.
- [ ] Update `ui/CLAUDE.md` (themes, runtime config, Toolchain notes) and
  the chart README.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- A deployment with no `DOCZ_THEME` serves the site on paper and follows
  the system mode. `DOCZ_THEME=classic` serves it on classic. A reader's
  pick holds across reloads and overrides the default.
- Full-rule axe passes on every core view under every theme (and both
  paper modes), and the contrast and Shiki tests pass for every theme.
- `just ui ci`, `just ui e2e`, `just chart lint`, and
  `just chart unittest` pass.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 4: `charcoal`, a third theme in repo-guardian's design

The first theme added under the contract. If the earlier phases are
right, this is token values and fonts, and no surface needs a change to
offer it.

<!--docz:tasks:start-->
#### Tasks

- [ ] `pkg/theme/tokens/charcoal.css` (Open Question 6: named for the look,
  not the project it came from), one block for every
  mode like `classic`, mapping repo-guardian DESIGN-0036's values onto
  docz's token names. Background `#17191d`, panel `#1f2227`, panel-2
  `#262a30`, and border `#31363d` go to the `bg-*` and border tokens.
  Foreground `#e8eaed`, muted `#9aa1ab`, accent `#5fa8f5`, ok `#3fb950`,
  warn `#d9a020`, bad `#f2554d`, and info `#7aa7ff` go to `fg-*`,
  `accent`/`link`, and the state and status tokens. Radii are 3px.
- [ ] Derive the tokens DESIGN-0036 has no value for from its set: the
  type colours `--color-t-*`, the status pills `--color-st-*`, the
  admonition tints `--color-adm-*-bg` (precomputed hex, as the contrast
  test needs), `--color-hash-*`, and `--color-code-bg`/`--color-code-fg`.
  Each choice is recorded in a comment in the file.
- [ ] Fonts: IBM Plex Sans and JetBrains Mono named first in the theme's
  font stacks, the system fonts after. Nothing is embedded, so a page
  stays network-free and its size does not change (Open Question 6).
- [ ] `pkg/theme`: register the theme in `Names()` and `CSS()`;
  `theme_test.go`'s every-token check covers it with no new test.
- [ ] `ui/src/theme/contrast.test.ts` passes for it at 4.5:1 on the three
  surfaces. DESIGN-0036 already moved its bad tone once for AA, so muted
  and warn may need the same, and each move is noted in the file.
- [ ] `pkg/export/html`: a full-page golden per theme, the theme cut to
  its markers as today.
- [ ] `ui/src/theme/tokens.css` imports the new file, and docz-site's
  selector offers it with no change to the selector.
- [ ] The e2e and axe runs of Phases 2 and 3 cover it through the theme
  list, with no new spec.
- [ ] Mention the theme in the plugin's `references/` and in
  `docz_yaml.tmpl`'s comment.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `html: {theme: charcoal}` in `.docz.yaml` exports pages and opens review
  sessions in the new theme, and docz-site's selector offers it, all
  without changing any surface's code.
- Every token is defined, every text colour passes 4.5:1, and full-rule
  axe passes on all three surfaces in the new theme.
- `just ci` and `just ui ci` pass.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 5: The widgets in the reader and the review page

The three widgets, drawn in React from the YAML `docz export html`
validates, so a document reads the same on an exported page, in
docz-site, and in a review.

<!--docz:tasks:start-->
#### Tasks

- [ ] A remark step, `widget-marker.ts`, beside `mermaid-marker.ts`, that
  marks `docz-flow`, `docz-stepper`, and `docz-scorecard` fences after
  sanitize and before Shiki. It moves the source onto a data attribute
  as mermaid's does, so sanitize never sees component markup and Shiki
  never replaces the block.
- [ ] Validate each block in TypeScript with the rules of
  `pkg/export/html/widgets.go`: the same closed fields, required fields,
  and cross-references, and the same error text at the same document
  lines. The rules are ported by hand and kept in step by a shared
  corpus (Open Question 9): `pkg/export/html/testdata/widgets/` holds each
  valid and invalid block with an `.errors.golden` listing the expected
  errors and lines, written by the Go tests with `-update` and read by
  the UI tests, so either side drifting fails a test.
- [ ] `Flow`, `Stepper`, and `Scorecard` components drawn from tokens
  only, with `textContent`-equivalent rendering (no
  `dangerouslySetInnerHTML`, no `innerHTML`). They redraw on a theme or
  mode change, and are lazy-loaded so the bundle budget holds.
- [ ] An invalid block renders as a caution callout naming each error at
  its line, followed by the YAML, as the export does.
- [ ] `MarkdownPre` routes marked blocks to the components, and the
  review render mode keeps `data-line` on the widget root, as
  `MermaidBlock` does, so annotations anchor to it.
- [ ] Extend the XSS suite (`processor.xss.test.tsx`) to the widget path:
  script, an event handler, and a `javascript:` URL in every string
  field, and a forged marker attribute in raw HTML.
- [ ] The markdown specimen gains a section per widget, and the axe
  sweep (`src/a11y/axe.test.tsx`) covers a valid and an invalid one.
- [ ] e2e: each widget draws in the reader and the review page, and each
  invalid fixture shows its source and errors.
- [ ] Update `ui/CLAUDE.md` (the markdown pipeline section) and the
  plugin's `references/widgets.md` to say where each widget is drawn.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- Each widget draws from the same block on an exported page, in
  docz-site's reader, and in a review, and each invalid fixture shows the
  same errors in all three.
- The XSS suite, the axe sweep, and the bundle budget pass, and the
  widget chunk loads only on a document that has a widget.
- `just ui ci` and `just ui e2e` pass.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 6: Close-out

<!--docz:tasks:start-->
#### Tasks

- [ ] Set IMPL-0029 to `Cancelled`, with a line at its top pointing here
  (Open Question 10). It stays in the index as the record.
- [ ] Mark DESIGN-0025's rollout complete, with this plan as step 5.
- [ ] Close #214, #213, and #209 from the final PR, and comment on #212
  that Phase 5's components are in place.
- [ ] Check `CLAUDE.md`, `ui/CLAUDE.md`, the chart README, and the
  plugin README against what shipped.
- [ ] Regenerate the changelog as the last commit of every PR.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- Every task above is checked, the three issues are closed, and the
  release that carries this plan is cut by its `minor` label.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:file-changes:start-->
## File Changes

| File | Action | Description |
| --- | --- | --- |
| `docs/design/0025-*.md`, `docs/design/0024-*.md` | Modify | The theme contract |
| `pkg/doczcore/config/config.go`, `html.go` | Modify, Create | The `html:` block, normalisation, validation |
| `pkg/doczcore/doctemplate/templates/docz_yaml.tmpl` | Modify | `html:` with `theme: paper` |
| `pkg/doczcore/layer_test.go` | Modify | Allow `config` → `pkg/theme` |
| `test/parity/parity.go` | Modify | `HTMLNormalizer` |
| `cmd/export_html.go`, `cmd/inspect.go` | Modify | Default from the config; `theme` in the JSON |
| `pkg/export/html/export.go` | Modify | `Theme` empty means the config's |
| `api/cli/inspect.schema.json`, `export-html.schema.json` | Modify | Additive `theme` |
| `ui/review/*`, `ui/src/review/*` | Modify | The repository's theme; selector removed |
| `ui/src/lib/appearance.ts`, `theme.ts`, `ui/scripts/gen-themes.ts` | Create | Shared mode; the site's theme; the generated theme list |
| `ui/server/serve.ts` | Modify | `DOCZ_THEME` |
| `ui/src/app/AppShell.tsx` | Modify | Selector and toggle |
| `charts/docz/values.yaml`, `templates/site-*.yaml` | Modify | `site.config.theme` |
| `pkg/theme/tokens/charcoal.css`, `pkg/theme/theme.go` | Create, Modify | The third theme |
| `ui/src/markdown/widget-marker.ts`, `widgets/*` | Create | Marker, validation, components |
| `plugins/docz-html/*` | Modify | The key; 0.3.0 |

<!--docz:file-changes:end-->

<!--docz:testing:start-->
## Testing Plan

- [ ] Config: the default, normalisation, an unknown value, `ParseBytes`
  equal to `Load`, the JSON shape, and the consumer module.
- [ ] Parity: `HTMLNormalizer` unit-tested and `just parity` green.
- [ ] CLI contract: `inspect` and `export-html` fixtures validated against
  their closed schemas with the new field.
- [ ] Export: the key picks the theme, `--theme` overrides it, and there
  is a golden per theme.
- [ ] Review: a session per theme from the repository's key, axe on each,
  and no theme selector.
- [ ] Site: `resolveTheme` both ends, readiness on a bad `DOCZ_THEME`,
  the selector, stored over injected, mermaid and Shiki per theme, and
  axe per theme and mode.
- [ ] Chart: `site.config.theme` set and unset.
- [ ] Contrast: every theme and mode, including the new one.
- [ ] Widgets: shared fixtures agree with the Go validator, the XSS
  suite, the axe sweep, the bundle budget, and e2e in the reader and the
  review.
- [ ] Plugin: `just plugin-eval docz-html` with the new case.

<!--docz:testing:end-->

<!--docz:dependencies:start-->
## Dependencies

- Phases 2 and 3 need Phase 1's key. Phase 4 needs Phases 2 and 3, so
  that a new theme reaches every surface through the list. Phase 5
  depends only on Phase 3's shared `appearance.ts` (the components
  redraw on a mode change), so it can run beside Phase 4.
- #212 (the review page's layout) starts after Phase 5.
- No new Go module. Phase 5 needs a TypeScript YAML parser: `yaml` is
  already in `ui/node_modules` through orval, and becomes a direct
  dependency.

<!--docz:dependencies:end-->

<!--docz:open-questions:start-->
## Open Questions

### 1. Where does the key live in `.docz.yaml`?

- a. A top-level `theme: paper`. It sets two surfaces (export and review),
  so it is not the export's alone, and it is one short line next to
  `docs_dir`.
- b. Under a new `html:` block (`html: {theme: paper}`), leaving room for
  later page settings such as the mermaid mode.
- c. Under `wiki:`, beside the MkDocs settings.

> **Resolved 2026-11-01: (b).**

### 2. Which package checks the value against `pkg/theme.Names()`?

- a. `pkg/doczcore/config` imports `pkg/theme`, which is stdlib-only, with
  the edge allowed by name in `layer_test.go`. `Validate()` then catches
  a typo at load in every consumer, docz-api included. The cost is that
  every consumer of `config` links `pkg/theme`'s embedded CSS, tens of
  kilobytes.
- b. `config` only normalises, and the value is checked where it is used
  (`docz export html`, `docz inspect`, and docz-review through it). A
  typo goes unnoticed until one of those runs, and `docz validate`
  does not report it.
- c. `pkg/theme` gains a tiny `names` subpackage with no embeds, which
  `config` imports. This is like a, without the CSS.

> **Resolved 2026-11-01: (a).**

### 3. What does an unknown theme name do?

- a. It is a validation warning, and the value falls back to `paper`.
  docz-api ingests every repository's `.docz.yaml`, so a hard error
  would stop a repository ingesting over a typo, or over a theme name
  newer than the deployed server.
- b. It is a hard error from `Validate()`, as an unknown type is. This is
  strict and simple, but it fails ingest in the case above.
- c. It is a hard error for the CLI and a warning for docz-api, through a
  flag on `Validate`.

> **Resolved 2026-11-01: (a).**

### 4. What does the review page do with a docz too old to report the theme?

- a. Render in `paper` and go on. The field is additive and the page has
  a safe default.
- b. Raise `SUPPORTED.minDocz` to the release that adds it, so an older
  docz is refused with both versions named, as for a schema mismatch.

> **Resolved 2026-11-01: (a).**

### 5. How does docz-site learn the list of themes?

- a. At build time from `pkg/theme`. A generated `src/theme/__generated__/themes.ts`
  is written from `pkg/theme/tokens/*.css` (the tokens are already read
  from there), and both `serve.ts` and the SPA validate against it.
- b. A hand-kept list in `ui/` with a test asserting it equals the files
  under `pkg/theme/tokens/`.
- c. `serve.ts` accepts any value for which a token file exists in the
  built CSS.

> **Resolved 2026-11-01: (a).**

### 6. What is the third theme called, and how does it get its fonts?

- a. `guardian`, with IBM Plex Sans and JetBrains Mono named first in its
  font stacks and the system fonts after. Pages stay network-free and
  small, and the design's fonts appear wherever they are installed.
- b. `guardian`, with subset `woff2` embedded as base64. The page always
  looks as designed, at a measured cost per page (to be recorded in the
  phase).
- c. Another name, for example `charcoal`, which describes the look
  rather than the project it came from. Fonts as in a.

> **Resolved 2026-11-01: (c).** The theme is `charcoal`; fonts as in a.

### 7. Does docz-site adopt the themes' radii?

- a. No. The site's wiped radius scale stays, and the radius tokens apply
  to exported pages and the review page only, as now. This is the
  smallest change and matches IMPL-0029's scope.
- b. Yes. Cards and controls take `--radius-card`/`--radius-control`, so a
  theme's corners are the same on all three surfaces. This needs a design
  note in docz-site's DESIGN-0005 line.

> **Resolved 2026-11-01: (a).**

### 8. How do the phases release?

- a. Each phase is a `dont-release` PR, and Phase 6's is labelled `minor`.
  This is the IMPL-0026 pattern: one minor carries the whole feature.
- b. Phases 1 and 2 ship together as a `minor`, because they are usable on
  their own (the key works for export and review), then each later
  phase is its own `minor`.
- c. Every phase is a `minor`.

> **Resolved 2026-11-01: (a).**

### 9. How are the TypeScript widget rules kept in step with `widgets.go`?

- a. Port the rules by hand and share the fixtures. A
  `pkg/export/html/testdata/widgets/` corpus holds each valid and invalid
  block with an `.errors.golden` file listing the expected errors and
  lines. The Go tests write it (`-update`) and the UI tests read the
  same files, so either side drifting fails a test.
- b. One JSON Schema per widget under `api/`. The UI validates with ajv,
  and Go keeps its hand-written checks, with a test that each fixture's
  verdict matches the schema's. Field rules are stated once, but the
  cross-reference rules (a stepper step's `next`, a scorecard result's
  check id) do not fit a schema and stay duplicated.
- c. docz-site validates nothing and draws what parses, showing YAML
  errors only. This is simplest, but an invalid block draws in the
  reader and fails on an export.

> **Resolved 2026-11-01: (a).**

### 10. What happens to IMPL-0029?

- a. Set it to `Cancelled`, with a line at its top pointing to this plan,
  which carries its two phases. It stays in the index as the record.
- b. Delete it, since it was never reviewed and this plan replaces it.
- c. Keep it as the plan for Phase 5 alone, and drop Phase 5 from this
  plan.

> **Resolved 2026-11-01: (a).**

<!--docz:open-questions:end-->

<!--docz:references:start-->
## References

- [DESIGN-0025: docz-html](../design/0025-docz-html-standalone-html-pages-from-docz-documents-and-one.md)
  §3 (themes), §4 (layout), and rollout step 5
- [DESIGN-0024: docz-review](../design/0024-docz-review-a-blocking-review-page-for-agent-written-documents.md)
- [IMPL-0028: docz-html](0028-docz-html-pkgtheme-the-skill-docz-export-html-and-the-review.md),
  Phase 6 (the review page on paper)
- [IMPL-0029](0029-docz-site-on-paper-with-the-widgets-as-react-components.md),
  the draft this plan takes over
- [#214](https://github.com/donaldgifford/docz/issues/214),
  [#213](https://github.com/donaldgifford/docz/issues/213),
  [#209](https://github.com/donaldgifford/docz/issues/209),
  [#212](https://github.com/donaldgifford/docz/issues/212)
- [repo-guardian DESIGN-0036: Frontend design cleanup and refresh for the v2 UI](https://github.com/donaldgifford/repo-guardian/blob/main/docs/design/0036-frontend-design-cleanup-and-refresh-for-the-v2-ui.md),
  and `ui/design/repo-guardian-mockup.html` there

<!--docz:references:end-->
