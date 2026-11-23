---
id: IMPL-0033
title: "docz-site and docz-review's page onto the design-system package, in nine phases"
status: Draft
author: Donald Gifford
created: 2026-11-23
---

<!-- markdownlint-disable-file MD024 MD025 MD041 -->

# IMPL-0033: docz-site and docz-review's page onto the design-system package, in nine phases

<!--toc:start-->
<!-- markdownlint-capture -->
<!-- markdownlint-disable MD013 -->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: Install and auth](#phase-1-install-and-auth)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2: Stylesheet and tokens](#phase-2-stylesheet-and-tokens)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 3: The Go side](#phase-3-the-go-side)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 4: Leaves](#phase-4-leaves)
    - [Tasks](#tasks-3)
    - [Success Criteria](#success-criteria-3)
  - [Phase 5: Controls and overlays](#phase-5-controls-and-overlays)
    - [Tasks](#tasks-4)
    - [Success Criteria](#success-criteria-4)
  - [Phase 6: Tables and rails](#phase-6-tables-and-rails)
    - [Tasks](#tasks-5)
    - [Success Criteria](#success-criteria-5)
  - [Phase 7: The frame](#phase-7-the-frame)
    - [Tasks](#tasks-6)
    - [Success Criteria](#success-criteria-6)
  - [Phase 8: docz-review's page](#phase-8-docz-reviews-page)
    - [Tasks](#tasks-7)
    - [Success Criteria](#success-criteria-7)
  - [Phase 9: Removal and close-out](#phase-9-removal-and-close-out)
    - [Tasks](#tasks-8)
    - [Success Criteria](#success-criteria-8)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Dependencies](#dependencies)
- [Open Questions](#open-questions)
  - [1. Which semver label does each phase's pull request carry?](#1-which-semver-label-does-each-phases-pull-request-carry)
  - [2. Which version of the package is pinned?](#2-which-version-of-the-package-is-pinned)
  - [3. What happens if a fork's pull request cannot install the package?](#3-what-happens-if-a-forks-pull-request-cannot-install-the-package)
  - [4. What is the budget between Phase 4 and Phase 7?](#4-what-is-the-budget-between-phase-4-and-phase-7)
  - [5. Is a beta cut before the frame merges?](#5-is-a-beta-cut-before-the-frame-merges)
  - [6. What does a phase do when the package lacks something it needs?](#6-what-does-a-phase-do-when-the-package-lacks-something-it-needs)
  - [7. How is the look checked at each phase?](#7-how-is-the-look-checked-at-each-phase)
- [References](#references)
<!-- markdownlint-restore -->
<!--toc:end-->

<!--docz:objective:start-->
## Objective

Move docz-site's chrome and docz-review's page onto
`@donaldgifford/design-system`, and generate `pkg/theme`'s `charcoal`
theme from the same package, in the order DESIGN-0028 sets out: tokens
first, components after, the frame last.

**Implements:** DESIGN-0028

<!--docz:objective:end-->

<!--docz:scope:start-->
## Scope

<!--docz:in-scope:start-->
### In Scope

- The package pinned in `ui/`, and the registry token on every path that
  installs `ui/`'s dependencies.
- `core.css` and the per-part sheets in place of `shell.css`, docz's own
  tokens in `docz-tokens.css`, the `accent` utility renamed to `brand`,
  and `.dark` set beside `data-mode`.
- `charcoal.css` generated from the package's `tokens.json`, dark only,
  with the version test and the drift check.
- Every part of the shell in DESIGN-0028 section 7 redrawn with the
  package's parts, in the package's frame.
- Site-rendered code on `github-light` and `github-dark`.
- docz-review's page on the package's controls.
- A bundle budget of 165 KB from Phase 4, the first package import,
  then the measured total plus 6 KB.
- `ui/CLAUDE.md`, `ui/README.md`, and `ui/CONTRIBUTING.md` kept true in
  the phase that changes what each describes.

<!--docz:in-scope:end-->

<!--docz:out-of-scope:start-->
### Out of Scope

- The app switcher in the sidebar's header. It is a fast follow after
  Phase 9, as its own change (DESIGN-0028 question 9).
- Deleting `ui/mockup.html` or `ui/design/`. Both stay for now
  (DESIGN-0028 question 10).
- A light mode for `charcoal` (DESIGN-0028 question 4).
- Any part built locally that the package lacks. A gap is an issue on
  the package.
- The document pane, the document themes, the markdown pipeline beyond
  its code themes, the widgets, `ui/server`, and the `ui/review` binary's
  server.
- docz-api, the CLI, and the chart's values.

<!--docz:out-of-scope:end-->
<!--docz:scope:end-->

## Implementation Phases

Each phase builds on the previous one. A phase is complete when all its tasks
are checked off and its success criteria are met.

Each phase is one pull request into `main` (DESIGN-0028 question 8) and
leaves the site shippable. Every pull request runs `just ci` and
`just ui ci` green before it merges, carries one semver label (question
1 below), and ends with the regenerated `CHANGELOG.md` commit. Those
three are not repeated in each phase's list.

Phase 3 depends on Phase 1 only and may be done at any point after it.

---

<!--docz:phase:start-->
### Phase 1: Install and auth

The package is a dependency that every install path can fetch, and
nothing imports it yet.

<!--docz:tasks:start-->
#### Tasks

- [x] Confirm the version to pin (question 2 below) and read its release
  notes since 0.6.0 for anything that changes DESIGN-0028.
- [x] Add `@donaldgifford/design-system` to `ui/package.json`, pinned
  exactly with no caret, and commit `ui/bun.lock`.
- [x] Commit `ui/.npmrc` with the `@donaldgifford` registry line and
  `_authToken=${NPM_TOKEN}`. Confirm trufflehog passes on it.
- [x] `just ui install`: fail before running Bun when `NPM_TOKEN` is
  unset, printing the `NPM_TOKEN=$(gh auth token)` line and the
  `read:packages` scope it needs.
- [ ] Grant the docz repository read access to the package in the
  package's "Manage Actions access" settings.
  **Deferred** - human required: it is a setting on the package's page
  with no API, and every CI install fails with a 403 until it is granted.
- [x] `ci.yml`: add `packages: read` and
  `NPM_TOKEN: ${{ secrets.GITHUB_TOKEN }}` to the install step of the
  `ui`, `ui-e2e`, and `plugin` jobs.
- [x] `release.yml` and `prerelease.yml`: the same on the "Install ui
  dependencies" step of the docz-review build.
- [x] `Dockerfile.ui`: copy `.npmrc` beside `package.json` and
  `bun.lock`, and run the install with
  `--mount=type=secret,id=NPM_TOKEN`. No `ARG` and no `ENV` carries the
  token.
- [x] `docker-bake.hcl`: `_common_ui` gains
  `secret = ["id=NPM_TOKEN,env=NPM_TOKEN"]`, so `dev-ui`, `ci-ui`, and
  `release-ui` all pass it.
- [x] Give `NPM_TOKEN` to every workflow step that bakes a ui target:
  `ci.yml`'s `docker-build` job, and each workflow that bakes
  `release-ui` (find them with a search for `bake` under
  `.github/workflows`).
- [x] The other image builds: the `docker build` recipe in `ui.just`
  (`--secret id=NPM_TOKEN,env=NPM_TOKEN`), and
  `deploy/ui/compose.yaml` and `deploy/ui/compose.local.yaml` (a
  `secrets:` entry sourced from the environment on the `build:` block).
- [x] Prove the cache trap is closed: build the image once with the
  secret, then again without it and with `--no-cache`, and confirm the
  second fails.
- [ ] `renovate.json5`: a `hostRules` entry for `npm.pkg.github.com`
  with an encrypted token, and a rule that keeps the package pinned
  exactly and out of any grouped update.
  **Deferred** - human required: the rule is in; the host rule waits for
  a token encrypted for this repository, which only a person can make.
  The file says what to add.
- [ ] Open a pull request from a fork that touches `ui/` and record
  whether the `ui` job can install (DESIGN-0028 question 5). Act on the
  result by question 3 below.
  **Deferred** - human required: it needs a second account's fork and
  the Actions access grant above. `ui/CONTRIBUTING.md` already describes
  question 3's answer (a), so a failing install needs no further change.
- [x] `ui/README.md` and `ui/CONTRIBUTING.md`: installing needs a GitHub
  token with `read:packages`, with the one-line way to get it.
- [x] `ui/CLAUDE.md`: the token comes from the environment and is never
  written to a file.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `bun install --frozen-lockfile` succeeds in the `ui`, `ui-e2e`, and
  `plugin` jobs and in the docz-review build, and fails with a clear
  message when the token is absent.
- `just ui local-up` builds the image with the token passed as a secret.
- `docker history` on the built image shows no token, and the runtime
  stage holds `dist` and `server` only.
- No file under `ui/src` imports the package, and the built site is
  byte-for-byte what `main` builds apart from the lockfile.
- The fork result is written into this document. **Not yet run**: see
  the deferred task above.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 2: Stylesheet and tokens

The site compiles on the package's stylesheet and reads the package's
values, and looks exactly as it did.

<!--docz:tasks:start-->
#### Tasks

- [x] `ui/src/theme/oklch.ts`: resolve a `{name}` reference and convert
  oklch to sRGB hex, with unit tests for a reference chain, a cycle, an
  unknown name, and the eleven values INV-0036's spike confirmed.
- [x] `tokens.css`: replace `@import "tailwindcss"` with the package's
  `core.css` under `source(none)`, and add the three `@source` lines for
  `src/`, `index.html`, and `review.html`.
- [x] Confirm the stylesheet's size against INV-0036's numbers, so a
  missing `source(none)` is caught here.
- [x] `ui/src/theme/docz-tokens.css`: docz's 26 own tokens, a light
  block at `:root` and a dark block under `.dark`, moved out of
  `shell.css` unchanged in value.
- [x] Point the shell's 32 shared names at the package's tokens in
  `shell.css` and the `@theme` block (`bg-base` at `--background`, and
  the rest of DESIGN-0028's section 4 table), so chrome that has not
  moved yet keeps working. These aliases are removed in Phase 9.
- [x] Rename the `accent` utility to `brand` across `ui/src`: `accent`,
  `accent-dim`, `accent-bg`, `accent-border`, and `on-accent`, in the 47
  utilities and the 17 `var()` reads. Register
  `--color-brand: var(--color-accent)` so `pkg/theme`'s variable keeps
  its name.
- [x] Add a test that fails on `bg-accent`, `text-accent`, or
  `border-accent` anywhere under `ui/src`, since each now compiles to the
  package's hover surface.
- [x] Remove the radius wipe (`--radius-*: initial`) and confirm docz's
  own controls still render at `rounded-sm`.
- [x] `appearance.ts`: `writeMode()` toggles `dark` on `<html>` from the
  stored mode or the system, and a media-query listener updates the
  class when nothing is stored and the system changes.
- [x] Test `applyStoredMode()` and the toggle: `.dark` and `data-mode`
  agree for stored light, stored dark, and system-follow in both
  directions.
- [x] `contrast.test.ts`: follow `var()` into the package's `tokens.css`
  through `oklch.ts`, and fail when a token resolves to nothing, so a
  lost mapping can never pass silently.
- [x] `src/test/setup.ts`: add the `window.matchMedia` stub.
- [x] Keep the fonts as they are: the three `@fontsource-variable`
  imports in `main.tsx`, and no import of the package's `fonts.css`.
- [x] `ui/CLAUDE.md`: rewrite the `tokens.css`, radius, and shell
  paragraphs for the new head of the stylesheet.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `contrast.test.ts` passes every pair in both modes, with no token
  skipped.
- The unit suite, `tsc -b`, eslint, and both axe sweeps pass.
- The built stylesheet emits no static font file from the package.
- Each screen in `ui/design/mockup.html`'s list looks the same as on
  `main` in both modes, checked by hand and noted in the pull request.
- The eager JavaScript is within the 130 KB budget, since no package
  component is imported yet.

Measured on the branch: the stylesheet is 75.1 KB (18.7 KB gzipped)
against 68.3 KB on `main`, with no part imported yet, and ships the same
34 font files. The eager JavaScript is 124.1 KB against 124.0 KB. Ten
routes (the directory with and without a query, repositories, a
repository's home, a type's index, a document, a missing document, the
changelog, the specimen page, and sign-in) were screenshotted from
`main`'s build and from the branch's in both modes and compared pixel
for pixel: all twenty are identical. The check by hand against
`ui/design/mockup.html` is still owed for the pull request.

One thing the phase's list did not have: mermaid read the shell's tokens
from computed style, where a shared token is now `oklch(...)`, which its
colour library cannot parse, so every diagram failed to render. The read
converts through `oklch.ts`, and the e2e suite is what caught it.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 3: The Go side

`charcoal.css` is generated from the package, and two checks keep it
there.

<!--docz:tasks:start-->
#### Tasks

- [x] `ui/scripts/gen-charcoal.ts`: read `tokens.json` from
  `ui/node_modules`, take the dark set, resolve references, and convert
  through `oklch.ts`.
- [x] The derivation table in the script: the 32 names taken from the
  package, the nine that follow by rule (`link` is the accent,
  `st-draft` is `warn`, `st-deprecated` is `fg-muted`, and the rest), and
  the two accent tints at 10% and 30%.
- [x] Read docz's own tokens from `docz-tokens.css`'s dark block, and
  fail on any of the 58 names left without a value or given two.
- [x] Write the token block into a template that keeps the hand-written
  font block and header prose, with a "generated, do not edit" line and
  the package version in the header.
- [x] `just gen-charcoal` at the root, failing with the
  `just ui install` hint when `node_modules` is missing, as
  `just vendor-mermaid` does.
- [x] `gen-charcoal:check` in `ui/package.json` and `ui.just`,
  regenerating to a temporary file and failing on a difference; add it
  to `just ui ci` and to the `ui` job in `ci.yml`.
- [x] A Go test in `pkg/theme` that reads the version from
  `charcoal.css`'s header and holds it to the one `ui/bun.lock` pins,
  after `TestVendoredMermaid`.
- [x] `ci.yml`: add `ui/bun.lock` to the `go` path filter, and put the
  new Go test in the job `.github/CLAUDE.md` names for tests that read
  the real tree.
- [x] Run the recipe, commit the generated `charcoal.css`, and
  regenerate the export's goldens with `go test ./... -update`.
- [x] Read the golden diff: every changed line is a colour value, and
  the changes match DESIGN-0028 section 11 (surfaces by a step or two,
  `bad` to `#f7756e`, type and hash colours to the shell's).
- [x] Confirm `charcoal` still has one token block, so it stays dark
  under any `data-mode`.
- [x] Unit tests for the generator: the derivation table covers every
  name once, and a missing token in `tokens.json` is an error that names
  it.
- [x] `pkg/CLAUDE.md` and `docs/development/build-notes.md`: `charcoal`
  is generated, the recipe that writes it, and that a package bump is
  the lockfile, the recipe, and the goldens in one pull request.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `just gen-charcoal` run twice writes the same bytes.
- `pkg/theme`'s tests pass, `pkg/theme` still imports the standard
  library only, and `components.css` holds no literal colour.
- Changing the pinned version in `ui/bun.lock` without running the
  recipe fails the Go test, and editing `charcoal.css` by hand fails
  `gen-charcoal:check`. Both are tried once and reverted.
- An exported `charcoal` page before and after differs only in the
  colours listed above.
- `just ci` passes, including the parity suite and the consumer module.

Measured on 2026-11-23, against 0.6.0. The recipe run twice wrote the
same bytes. Both checks were made to fail once and reverted: the
lockfile set to 0.6.1 failed `TestCharcoalMatchesPinnedDesignSystem`
naming both versions, and a hand edit of `--color-bad` failed
`gen-charcoal:check` with the diff. 55 of the 58 colours moved.

Two things read differently from the list above. The table's split is 21
names taken from the package directly, nine by rule, and the two tints
(32 shared in all), with docz's own 26 beside them. And the golden diff
is the colour values plus the file's header comment, which a page
inlines with its theme; the page hashes in the CLI contract's
`export-html-*.json` fixtures moved for the same reason.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 4: Leaves

The smallest parts are the package's: badges, dots, buttons, cards,
states, the page header, and load more.

<!--docz:tasks:start-->
#### Tasks

- [x] The `parts/` line test: read every package import under `ui/src`
  and fail when a part has no matching `parts/*.css` import in
  `tokens.css`. Add it before the first import, so every later phase is
  held to it.
- [x] `bundle-budget.ts`: 165 KB, with the comment rewritten for the new
  eager set (question 4 below).
- [x] `statusVariant()` in `lib/colors.ts` over the existing
  `STATUS_GROUPS`, by DESIGN-0028's section 8 table, with a table test
  over every status and an unknown one.
- [ ] `badges.tsx`: the status pill and dot as `Badge` and `Dot` by
  variant, and the type dot and chip by `tone` from `typeColor()`.
- [ ] Point the eight hash colours at `--category-1` to `--category-8`
  and confirm a custom type still gets a stable colour.
- [ ] `query-states.tsx`: `EmptyState`, `ErrorState`, `Skeleton`, and
  `InlineLoading`, with `SessionRequiredRedirect` unchanged.
- [ ] `route-error.tsx` and `not-found.tsx` on the same parts.
- [ ] Repository cards in `repos.tsx` as `Card`.
- [ ] The page hero on `directory`, `repos`, `repo-home`, and
  `repo-type` as `PageHeader`.
- [ ] Every "show more" as `LoadMore`.
- [ ] `login.tsx` and `consent.tsx` on `Card` and `Button`, with each
  provider button still a real `<a href="/auth/login?...">` through
  `Button`'s `render` prop. Keep the test that asserts the anchor.
- [ ] Rewrite `badges.test.tsx` to assert the variant and tone a status
  and a type produce.
- [ ] Move the classes in the files touched here from docz's names to
  the package's.
- [ ] Record the eager JavaScript size in the pull request.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- No file touched in this phase draws a badge, a button, a card, or an
  empty or error panel by hand.
- `Superseded` renders in the package's `ignored` colour, and every
  other status keeps its colour.
- The sign-in flow works against a real docz-api, checked with
  `just ui local-up`.
- The unit suite and both axe sweeps pass in both modes.
- The eager JavaScript is under 165 KB.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 5: Controls and overlays

The palette, the menus, and the directory's filters are the package's.

<!--docz:tasks:start-->
#### Tasks

- [ ] `command-palette.tsx` on `CommandDialog`, with `CommandFilters`
  for the type filter, `CommandFooter` for the key hints, and `preview`
  for the hit preview. Keep recent documents and the query behaviour.
- [ ] A palette hit as a `ListRow` with `render` (a router link) and
  `onClick` (close the palette), with a test that following the link
  closes it.
- [ ] Keep the palette's body out of the eager graph where
  `CommandDialog` allows, and note in the pull request what could not be
  made lazy.
- [ ] `session-menu.tsx` as a `DropdownMenu`, still in the top bar until
  Phase 7. Sign-out navigates to `/login` before it clears the query
  client.
- [ ] `directory-controls.tsx`: `FilterBar`, the type chips as
  `FilterChip` with `pressed`, and the repository picker as `OptionRows`
  in a `Popover`. One click still changes a type filter.
- [ ] The directory's hit rows as `ListRow` with `eyebrow`, `detail`,
  and `render`, the snippet through `blocks/marked`, and the time as
  `Time` with `style="stacked"`. Retire `snippet.tsx` if nothing else
  uses it.
- [ ] Confirm `blocks/marked` escapes what it highlights: a hit whose
  snippet holds markup renders as text. Add the payloads from
  `mocks/content/xss-payloads.json` as a test.
- [ ] `DocMetaTable`'s format buttons as `SegmentedControl`.
- [ ] Rewrite `command-palette.test.tsx` and `session-menu.test.tsx`
  against the new parts, asserting docz's behaviour and not the
  package's.
- [ ] Check keyboard use by hand in both modes: open, move, select, and
  close the palette; the menu; the popover.
- [ ] Move the classes in the files touched here to the package's
  names, and record the eager JavaScript size.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- Search, filter, and sort on the directory return what they return on
  `main`, and the URL's parameters are unchanged.
- The palette opens on the same shortcut, lists the same results, and
  closes when a result is followed.
- The XSS suites pass, with the snippet payloads added.
- `mvp.spec.ts` passes with only selector changes.
- The eager JavaScript is under 165 KB.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 6: Tables and rails

The reader's side columns and the type index are the package's.

<!--docz:tasks:start-->
#### Tasks

- [ ] `repo-type.tsx`: the type's document table as `DataTable`, with
  the sort it has today.
- [ ] `TocList` as `blocks/outline` with `hooks/use-active-heading`,
  following the headings the document pane and `DocSurface` render.
- [ ] Confirm the outline tracks headings inside a fragment after the
  runtime switches a tab, since the pane's HTML is not React's.
- [ ] `LifecycleRail` as `blocks/disclosure` holding a `Stepper` with
  `orientation="vertical"` and `size="sm"`.
- [ ] `DocMetaTable` as `DetailsGrid`.
- [ ] Keep the right rail a column of the content at the same
  breakpoints, so the document's measure does not change.
- [ ] Add tests for the outline's active heading and the lifecycle's
  steps for a document with and without a status history.
- [ ] Move the classes in the files touched here to the package's
  names, and record the eager JavaScript size.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- A document's outline, lifecycle, and details show the same content as
  on `main`.
- `reader-pane.spec.ts` and `pages.spec.ts` pass with only selector
  changes.
- The document pane's files are untouched, and
  `doc-pane.xss.test.tsx` passes unchanged.
- The eager JavaScript is under 165 KB.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 7: The frame

The top bar and rail are replaced by the package's sidebar and top bar.
This is the phase a reader sees.

<!--docz:tasks:start-->
#### Tasks

- [ ] Read the package's `SidebarProvider` and `use-tree-open` at the
  pinned version and record where each keeps its state (a cookie or
  `localStorage`) and under what key. If either uses a cookie, confirm
  `ui/server` neither forwards it to docz-api nor logs it.
- [ ] `AppShell.tsx`: `SidebarProvider`, `ShellSidebar`, and
  `TopBarLayout` around the `Outlet`, with `RestoreAfterLogin` kept.
- [ ] The sidebar's header (the logo mark and name), the `Browse` group
  (Directory, Repositories), and the `Pinned` group from
  `enabledNavLinks()`, absent when there are none.
- [ ] The repository's groups on a repository's routes: Home and
  Changelog, the pages tree, and the documents tree, replacing
  `repo-nav.tsx` and `repo-nav-pages.tsx`.
- [ ] The documents tree as `ShellSidebarTree`, one row per type with
  its count, fetching a type's documents when its row first opens,
  including through the icon rail's popover.
- [ ] Loading inside a tree as `InlineLoading` in a
  `ShellSidebarTreeNote`.
- [ ] The type's colour by DESIGN-0028 question 3: on the type row's
  dot, on `Dot` and `Badge`, and inside the pane. Remove
  `--type-accent` from the logo mark, the breadcrumb leaf, and the
  active row.
- [ ] `RepoBreadcrumbs` as the package's `Breadcrumb` in the top bar.
- [ ] `shell/search-trigger` and `shell/mode-toggle` in the top bar,
  the toggle driven by `useAppearance`. Retire
  `appearance-controls.tsx`.
- [ ] Move the account menu to the sidebar's footer.
- [ ] Render `/login` and `/consent` outside the frame.
- [ ] Check the narrow layout: the sidebar is the package's sheet, and
  the document is readable at 360 px.
- [ ] Site-rendered code: Shiki with `github-light` and `github-dark`
  (or the package's own pair, by DESIGN-0028 question 6) and
  `defaultColor: false`, with CSS choosing the set from `.dark`.
- [ ] `theme-contrast.ts`: `CODE_BG` as a light and dark pair pinned to
  the package's `--code-bg`, and `liftThemeContrast` run once per theme.
  Its test checks both.
- [ ] Remove `code-bg` and `st-superseded` from the chrome's use of
  `docz-tokens.css`; both stay in the file for `charcoal`.
- [ ] Rewrite `repo-nav.test.tsx` and `nav-pins.test.tsx` for the
  sidebar, and update the e2e specs that drive navigation
  (`mvp.spec.ts`, `pages.spec.ts`, `a11y.spec.ts`).
- [ ] Add a specimen section for a code block in each mode, and check
  the specimen page first.
- [ ] Compare each screen with `ui/design/mockup.html` in both modes and
  attach screenshots to the pull request.
- [ ] Cut a beta from this branch and check it on the homelab (question
  5 below), since the release workflows' token wiring runs nowhere else.
- [ ] `ui/CLAUDE.md`: rewrite the routing, shell, and per-type accent
  paragraphs.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- Every route renders in the package's frame except `/login` and
  `/consent`, at the same URLs.
- A deep link to a document opens with its repository's groups in the
  sidebar and its type's row open.
- The mode toggle changes the frame and the document together, survives
  a reload, and follows the system when nothing is stored.
- Code in a page, the changelog, and the fallback render is readable at
  4.5:1 in both modes.
- Both axe sweeps pass over every shell mode, and the chunk assertions
  for mermaid, ELK, and the widgets still hold.
- The eager JavaScript is under 165 KB.
- The beta's image and docz-review binaries build in the release
  workflows, and the site works on the homelab.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 8: docz-review's page

The review page's controls are the package's, and its review logic is
untouched.

<!--docz:tasks:start-->
#### Tasks

- [ ] `controls.tsx`, `status.tsx`, and `app.tsx`: `Button`, `Tabs`,
  `Badge`, `SegmentedControl`, and `InlineLoading`.
- [ ] Any confirmation or submit prompt as the package's `Dialog`.
- [ ] `questions.tsx`, `findings.tsx`, `regions.tsx`, and
  `annotations.tsx`: their buttons and badges on the package's parts,
  with the option cards' behaviour unchanged.
- [ ] Keep the page's own single-column frame and header, with
  `shell/mode-toggle` for the mode. Do not import `shell/sidebar`.
- [ ] Leave `diff.tsx`'s rendering alone and move only its colours'
  names. Check the diff in light mode against the fix in v2.6.1.
- [ ] Confirm the document view still renders in the document's theme
  inside `.docz-page`, across every theme in `e2e/appearances.ts`.
- [ ] Confirm no sidebar code is in the review bundle by searching
  `dist-review` for it.
- [ ] Run `just ui build-review` and open a compiled binary's page with
  the network off.
- [ ] Rewrite the affected tests in `src/review` and update
  `review.spec.ts` and `review-server.spec.ts` selectors.
- [ ] Walk one real review end to end with the plugin: comment, answer
  a question, submit, and confirm the agent receives the feedback.
- [ ] `plugins/CLAUDE.md` and `ui/CLAUDE.md`: the review page's
  paragraphs.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `just ui test-review` and the review e2e specs pass across every
  shell mode and document theme.
- A submitted review's feedback payload is byte-for-byte what `main`
  produces for the same actions, held by the existing feedback tests.
- The four binaries build, and a binary serves its page with no network.
- The plugin job passes.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 9: Removal and close-out

Nothing of the old shell is left, the budget is final, and the records
are flipped.

<!--docz:tasks:start-->
#### Tasks

- [ ] Delete `shell.css` and `shell.test.ts`, and the `:root` aliases
  Phase 2 added.
- [ ] Reduce `tokens.css` to the imports, docz's fonts, the
  document-scoped utility names, and the prose and chip rules.
- [ ] Add the vocabulary test: a document-scoped utility name outside
  `src/markdown`, `doc-pane.tsx`, `doc-surface.tsx`, and the review
  page's document view fails.
- [ ] Delete the components the package replaced and anything left
  unused: confirm with a search for each file's exports.
- [ ] Remove `cmdk` from `ui/package.json` if only the package imports
  it now, and any other dependency left without an importer.
- [ ] Measure the eager JavaScript and set the budget to that total
  plus 6 KB, with the comment in `bundle-budget.ts` stating both
  numbers.
- [ ] Record the final stylesheet and JavaScript sizes beside
  INV-0036's in this document.
- [ ] `ui/CLAUDE.md`: remove what describes the old shell, including
  `mockup.html` as "the visual source of truth", and point at the
  package's Storybook instead. `mockup.html` and `ui/design/` stay.
- [ ] Root `CLAUDE.md`: one line, only if the map changed.
- [ ] Report back on the package's IMPL-0001 Phase 6: which tasks are
  done, and that the export keeps `data-mode` by INV-0036 question 2.
- [ ] Open an issue for the app switcher fast follow, with the mockup's
  section and the runtime setting it needs.
- [ ] `docz status set`: DESIGN-0028 to Implemented and this document
  to Completed.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- A search of `ui/src` finds no `shell.css`, no `:root[data-mode]`
  token block, and no docz-named utility in chrome.
- `just ci` and `just ui ci` pass with the final budget.
- The released site and docz-review binaries are checked once on the
  homelab.
- `docz validate` passes, and both documents show their final status.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:file-changes:start-->
## File Changes

<!-- markdownlint-capture -->
<!-- markdownlint-disable MD013 -->

| File | Action | Description |
| ---- | ------ | ----------- |
| `ui/package.json`, `ui/bun.lock` | Modify | The package, pinned exactly; `gen-charcoal:check` |
| `ui/.npmrc` | Create | The registry and the token variable |
| `Dockerfile.ui`, `docker-bake.hcl` | Modify | The build secret |
| `deploy/ui/compose.yaml`, `deploy/ui/compose.local.yaml` | Modify | The build secret |
| `.github/workflows/ci.yml` | Modify | The token on three installs and the bake; `ui/bun.lock` in the `go` filter; the drift check |
| `.github/workflows/release.yml`, `prerelease.yml`, and the image workflows | Modify | The token |
| `renovate.json5` | Modify | The registry's host rule |
| `ui.just`, `docz.just` | Modify | The install check; `gen-charcoal` and its check |
| `ui/src/theme/tokens.css` | Modify | `core.css`, the `parts/` lines, `brand` |
| `ui/src/theme/docz-tokens.css` | Create | docz's own tokens |
| `ui/src/theme/oklch.ts` | Create | Reference resolution and oklch to hex |
| `ui/src/theme/shell.css`, `shell.test.ts` | Delete | Phase 9 |
| `ui/src/theme/contrast.test.ts` | Modify | The resolver |
| `ui/src/lib/appearance.ts` | Modify | `.dark` |
| `ui/src/lib/colors.ts` | Modify | `statusVariant()` |
| `ui/src/test/setup.ts` | Modify | The `matchMedia` stub |
| `ui/scripts/gen-charcoal.ts`, `gen-charcoal-check.sh` | Create | The generator and its drift check |
| `ui/scripts/bundle-budget.ts` | Modify | 165 KB, then the final number |
| `ui/src/app/AppShell.tsx` | Modify | The frame |
| `ui/src/components/*` | Modify or delete | Each part, by phase |
| `ui/src/routes/*` | Modify | Each route's chrome |
| `ui/src/markdown/processor.ts`, `theme-contrast.ts` | Modify | The Shiki pair |
| `ui/src/review/*.tsx` | Modify | The review page's controls |
| `ui/e2e/*.spec.ts` | Modify | Selectors |
| `pkg/theme/tokens/charcoal.css` | Modify | Generated |
| `pkg/theme/theme_test.go` | Modify | The version test |
| `pkg/export/html/testdata/golden/*` | Modify | Regenerated with `-update` |
| `ui/CLAUDE.md`, `ui/README.md`, `ui/CONTRIBUTING.md`, `pkg/CLAUDE.md`, `plugins/CLAUDE.md` | Modify | What each area's notes say |

<!-- markdownlint-restore -->

<!--docz:file-changes:end-->

<!--docz:testing:start-->
## Testing Plan

- [ ] `oklch.ts`: references, cycles, unknown names, and known values.
- [ ] The generator: the derivation table covers all 58 names once.
- [ ] The Go version test, and `gen-charcoal:check`, each seen to fail
  once on purpose.
- [ ] `contrast.test.ts` in both modes through the resolver, failing on
  an unresolved token.
- [ ] The mode test: `.dark` and `data-mode` together.
- [ ] The `bg-accent` test, the `parts/` line test, and the vocabulary
  test.
- [ ] `statusVariant()` over every status.
- [ ] Rewritten component tests for the palette, the account menu, the
  badges, and the sidebar.
- [ ] The XSS suites, with search snippet payloads added.
- [ ] Both axe sweeps over every shell mode, at every phase.
- [ ] The e2e chunk assertions, unchanged.
- [ ] The review page's tests and e2e across every theme, and one real
  review walked by hand.
- [ ] By hand: every mockup screen in both modes at Phases 2 and 7, an
  exported `charcoal` page at Phase 3, keyboard use at Phase 5, and the
  narrow layout at Phase 7.

<!--docz:testing:end-->

<!--docz:dependencies:start-->
## Dependencies

- `@donaldgifford/design-system` at 0.6.0 or later. 0.6.0 is the first
  version with the per-part sheets and with `ListRow` running `onClick`
  beside `render`.
- A GitHub token with `read:packages` for every developer, and the docz
  repository's Actions access on the package.
- An encrypted registry token for Renovate.
- The homelab deployment, for the checks in Phases 7 and 9.
- Nothing in docz-api. No open docz pull request blocks this.
- A gap found in the package blocks the phase that needs it until the
  package ships a fix (question 6 below).

<!--docz:dependencies:end-->

<!--docz:open-questions:start-->
## Open Questions

### 1. Which semver label does each phase's pull request carry?

- **a. `dont-release` on Phases 1 to 6, `minor` on Phase 7, and `patch`
  on Phases 8 and 9.** *(recommendation)* The frame is the change a
  reader sees, so it is the release. Phase 3's small shift in exported
  `charcoal` pages and Phase 4's `Superseded` colour ride along with
  whatever releases next.
- b. `patch` on Phase 3 as well, so the `charcoal` change gets its own
  release and changelog entry, since it reaches CLI users.
- c. `dont-release` on everything through Phase 8 and one `minor` on
  Phase 9.
- d. Other.

> **Resolved 2026-11-23: (a).**

### 2. Which version of the package is pinned?

- **a. The latest release when Phase 1 starts, 0.6.0 or later, then held
  for the whole migration except for a fix docz asked for.**
  *(recommendation)* The package is moving quickly, and starting on its
  newest avoids a bump in the middle.
- b. 0.6.0 exactly, the version the spike ran on, with a bump after
  Phase 9.
- c. Follow every release as Renovate proposes it.
- d. Other.

> **Resolved 2026-11-23: (a).**

### 3. What happens if a fork's pull request cannot install the package?

- **a. The UI jobs fail on that pull request, and a maintainer pushes
  the branch to the repository to run them. `ui/CONTRIBUTING.md` says
  so.** *(recommendation)* No gate is skipped, and a secret would not
  reach a fork either.
- b. Skip the UI jobs on a fork's pull request with a notice, and rely
  on the maintainer's local `just ui ci`.
- c. Other.

> **Resolved 2026-11-23: (a).**

### 4. What is the budget between Phase 4 and Phase 7?

- **a. Raise it to 165 KB in Phase 4, with the first package import.**
  *(recommendation)* The spike measured 137.3 KB with one button and one
  badge, so Phase 4 cannot merge under 130 KB. DESIGN-0028 question 1
  set 165 KB "from the phase that mounts the frame"; this moves that
  three phases earlier, and the final number is still set in Phase 9.
- b. Step it: 145 KB for Phases 4 to 6, then 165 KB at Phase 7.
- c. Other.

> **Resolved 2026-11-23: (a).** 165 KB from Phase 4. This amends the
> timing in DESIGN-0028 question 1, which carries a note saying so.

### 5. Is a beta cut before the frame merges?

- **a. Yes: one beta tagged from Phase 7's branch, checked on the
  homelab, before the pull request merges as a `minor`.**
  *(recommendation)* The release workflows build the image and the
  docz-review binaries with the token, and nothing before a tag runs
  them. The beta pull request bumps `Chart.yaml` by the usual rule.
- b. Two betas: one after Phase 1, to prove the release workflows'
  token wiring early, and one at Phase 7.
- c. No beta; check the release itself.
- d. Other.

> **Resolved 2026-11-23: (a).**

### 6. What does a phase do when the package lacks something it needs?

- **a. File an issue on the package and hold that task; the rest of the
  phase merges, and the task moves to the phase after the fix ships.**
  *(recommendation)* DESIGN-0028's goal is that docz builds none of the
  package's parts, and design-system#32 was answered in a day.
- b. Build a local stand-in marked for removal, and delete it in Phase
  9.
- c. Hold the whole phase until the fix ships.
- d. Other.

> **Resolved 2026-11-23: (a).**

### 7. How is the look checked at each phase?

- **a. By hand against `ui/design/mockup.html` in both modes, with
  screenshots in the pull request at Phases 2 and 7.**
  *(recommendation)* The gates that matter (contrast, axe, e2e) are
  automated already, and screenshot tests would be rewritten at every
  phase of a restyle.
- b. Add Playwright screenshot tests in Phase 2 as a baseline, and
  update them per phase.
- c. Other.

> **Resolved 2026-11-23: (a).**

<!--docz:open-questions:end-->

<!--docz:references:start-->
## References

- [DESIGN-0028](../design/0028-docz-site-and-docz-reviews-page-on-the-shared-design-system.md):
  the design this implements
- [INV-0036](../investigation/0036-docz-site-on-the-shared-design-system-or-docz-as-a-backend-of.md):
  the investigation and the spike's numbers
- [DESIGN-0026](../design/0026-docz-sites-shell-in-repo-guardians-look-and-the-readers.md)
  and
  [IMPL-0032](0032-the-repo-guardian-look-shell-the-readers-document-pane.md):
  the shell this replaces
- [`ui/design/mockup.html`](../../ui/design/mockup.html): the screens
  each phase is checked against
- [donaldgifford/design-system](https://github.com/donaldgifford/design-system):
  IMPL-0001 Phase 6 and
  [issue 32](https://github.com/donaldgifford/design-system/issues/32)
- [donaldgifford/repo-guardian](https://github.com/donaldgifford/repo-guardian):
  IMPL-0037, the same migration in eight phases

<!--docz:references:end-->
