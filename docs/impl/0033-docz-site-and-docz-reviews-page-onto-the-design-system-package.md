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
- [x] Grant the docz repository read access to the package in the
  package's "Manage Actions access" settings.
  Seen working on 2026-11-23: the `ui`, `ui-e2e`, and `plugin` jobs
  installed on pull request #238, and so did `prerelease.yml` for
  `v2.7.0-beta.1`, each with the workflow's own token.
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
- [x] `badges.tsx`: the status pill and dot as `Badge` and `Dot` by
  variant, and the type dot and chip by `tone` from `typeColor()`.
- [x] Point the eight hash colours at `--category-1` to `--category-8`
  and confirm a custom type still gets a stable colour.
- [x] `query-states.tsx`: `EmptyState`, `ErrorState`, `Skeleton`, and
  `InlineLoading`, with `SessionRequiredRedirect` unchanged.
- [x] `route-error.tsx` and `not-found.tsx` on the same parts.
- [x] Repository cards in `repos.tsx` as `Card`.
- [x] The page hero on `directory`, `repos`, `repo-home`, and
  `repo-type` as `PageHeader`.
- [x] Every "show more" as `LoadMore`.
- [x] `login.tsx` and `consent.tsx` on `Card` and `Button`, with each
  provider button still a real `<a href="/auth/login?...">` through
  `Button`'s `render` prop. Keep the test that asserts the anchor.
- [x] Rewrite `badges.test.tsx` to assert the variant and tone a status
  and a type produce.
- [x] Move the classes in the files touched here from docz's names to
  the package's.
- [x] Record the eager JavaScript size in the pull request.

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

Measured on 2026-11-23. The eager JavaScript is 140.6 KB gzipped, against
124.1 KB after Phase 2, with ten parts imported: `Badge`, `Button`,
`Card`, `Skeleton`, `EmptyState`, `ErrorState`, `InlineLoading`,
`ListRow` (for `Dot`), `LoadMore`, and `PageHeader`. The unit suite
(1,531 tests) and the e2e suite (84, with full-rule axe in both modes)
pass.

Three things differ from the list above:

- A sign-in provider button is an anchor wearing `buttonVariants`, not
  `Button` with `render`. Base UI's `Button` gives a rendered anchor the
  button role, the package's README says a link styled as a button stays
  a link, and the test that asserts the anchor asserts its link role.
  The route error's "go home" and the 404's link are drawn the same way.
- The route error boundary uses `ErrorState`, not the package's
  `RouteErrorLayout`, whose one action is Retry. The boundary must not
  offer a retry, so its link home sits under the panel.
- `@tabler/icons-react` is now a direct dependency, pinned to the
  version the package pins (3.49.0), because the package's blocks take
  an icon as a Tabler component.

The sign-in flow against a real docz-api (`just ui local-up`) is
**deferred - human required**: it needs the local docz-api stack and a
person to sign in through a provider. The hand check against
`ui/design/mockup.html` is owed for the pull request, as in Phase 2.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 5: Controls and overlays

The palette, the menus, and the directory's filters are the package's.

<!--docz:tasks:start-->
#### Tasks

- [x] `command-palette.tsx` on `CommandDialog`, with `CommandFilters`
  for the type filter, `CommandFooter` for the key hints, and `preview`
  for the hit preview. Keep recent documents and the query behaviour.
- [x] A palette hit as a `ListRow` with `render` (a router link) and
  `onClick` (close the palette), with a test that following the link
  closes it.
  Decided 2026-11-23 by the author: the hit stays as built, with no
  link. It is a `ListRow` selected through cmdk, and a click or Enter
  closes the palette and navigates. A link inside cmdk's
  `role="option"` item fails axe's `nested-interactive` rule.
- [x] Keep the palette's body out of the eager graph where
  `CommandDialog` allows, and note in the pull request what could not be
  made lazy.
- [x] `session-menu.tsx` as a `DropdownMenu`, still in the top bar until
  Phase 7. Sign-out navigates to `/login` before it clears the query
  client.
- [x] `directory-controls.tsx`: `FilterBar`, the type chips as
  `FilterChip` with `pressed`, and the repository picker as `OptionRows`
  in a `Popover`. One click still changes a type filter.
- [x] The directory's hit rows as `ListRow` with `eyebrow`, `detail`,
  and `render`, the snippet through `blocks/marked`, and the time as
  `Time` with `style="stacked"`. Retire `snippet.tsx` if nothing else
  uses it.
- [x] Confirm `blocks/marked` escapes what it highlights: a hit whose
  snippet holds markup renders as text. Add the payloads from
  `mocks/content/xss-payloads.json` as a test.
- [x] `DocMetaTable`'s format buttons as `SegmentedControl`.
- [x] Rewrite `command-palette.test.tsx` and `session-menu.test.tsx`
  against the new parts, asserting docz's behaviour and not the
  package's.
- [x] Check keyboard use by hand in both modes: open, move, select, and
  close the palette; the menu; the popover.
  Checked by the author on 2026-11-24, on `v2.7.0-beta.1` on the
  homelab: all three work.
- [x] Move the classes in the files touched here to the package's
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

Measured on 2026-11-23. The eager JavaScript is 125.7 KB gzipped, against
140.6 KB after Phase 4, with 21 parts imported. The unit suite (1,562
tests) and the e2e suite (84, with full-rule axe in both modes) pass,
and `mvp.spec.ts` needed no change at all.

What is lazy, and what could not be:

- The palette is a chunk of its own (`command-palette.tsx`: cmdk, the
  package's dialog and command parts, the search queries). What stays
  eager is `command-palette-lazy.tsx`: the `⌘K` and `/` listener and the
  dynamic import. The chunk is fetched 1.5 seconds after the shell
  mounts and on the search button's hover or focus, so the first `⌘K`
  finds it loaded.
- The account menu is a chunk of its own too (`account-menu.tsx`).
  `DropdownMenu` brings Base UI's menu and positioning, about 30 KB
  gzipped, and with it in the top bar the eager total was 171.7 KB, over
  the budget. Only a signed-in session draws it, so `session-menu.tsx`
  keeps the session's states eager and loads the menu behind the
  placeholder.
- Nothing asked for could not be made lazy. `CommandDialog` itself is
  inside the lazy chunk.

Four things differ from the list above:

- A palette hit is a `ListRow` inside cmdk's item with no link of its
  own, so the second task is open. cmdk's item is a `role="option"`, and
  a link inside one fails axe's `nested-interactive` rule (serious) in
  the jsdom sweep, and would in the browser's. A click and Enter both
  select the item, which closes the palette and navigates once, as on
  `main`; a test holds that. What is lost against the design is a real
  link on a hit (open in a new tab). This needs a decision: a change in
  the package (a `CommandItem` that renders as the link, say), or
  leaving it as it is.
- `lib/updatedAt.ts` is deleted with `snippet.tsx`: `Time` replaced its
  only caller. `snippet.test.tsx` became `snippet.xss.test.tsx`, which
  runs `blocks/marked` over every payload in
  `mocks/content/xss-payloads.json`, outside the match and inside it.
- A directory row now shows the hit's snippet under its title, as the
  task asks; on `main` only the palette showed one. The status badge and
  the time sit in `ListRow`'s `trailing`, which is above the stretched
  link, so a click on them does not follow the row. That is the
  package's behaviour and is noted for its Phase 6 report.
- The filter bar's reset reads "Clear", the package's wording, and the
  repository picker's trigger reads `repo = all`.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 6: Tables and rails

The reader's side columns and the type index are the package's.

<!--docz:tasks:start-->
#### Tasks

- [x] `repo-type.tsx`: the type's document table as `DataTable`, with
  the sort it has today.
- [x] `TocList` as `blocks/outline` with `hooks/use-active-heading`,
  following the headings the document pane and `DocSurface` render.
- [x] Confirm the outline tracks headings inside a fragment after the
  runtime switches a tab, since the pane's HTML is not React's.
  Decided 2026-11-23 by the author: a fragment gets no outline, so
  there is none to track. `doc.tsx` gives the document pane no rail,
  because a fragment has its own tabs, which is `main`'s behaviour.
- [x] `LifecycleRail` as `blocks/disclosure` holding a `Stepper` with
  `orientation="vertical"` and `size="sm"`.
- [x] `DocMetaTable` as `DetailsGrid`.
- [x] Keep the right rail a column of the content at the same
  breakpoints, so the document's measure does not change.
- [x] Add tests for the outline's active heading and the lifecycle's
  steps for a document with and without a status history.
- [x] Move the classes in the files touched here to the package's
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

Measured on 2026-11-23. The eager JavaScript is 125.9 KB gzipped, against
125.7 KB after Phase 5, with 26 parts imported. The unit suite (1,570
tests) and the e2e suite (84, with full-rule axe in both modes) pass.
`reader-pane.spec.ts` and `pages.spec.ts` needed no change, and no file
of the document pane was touched.

Four things differ from the list above:

- The outline follows the headings a `DocSurface` renders: a page, a
  repository's home, its changelog, the type index, and a document in
  the site's own rendering. A fragment in the document pane has no
  outline on `main`, so the third task has nothing to confirm and is
  left for a decision.
- The type index's table sits under the document surface, not in it:
  the surface's prose rules would restyle a table inside. The "docz
  auto-generated" markers around it are gone with that, and the empty
  type is the package's `EmptyState`. The table has no sort, as on
  `main`: `sortable={false}`.
- The lifecycle's steps are not in the DOM while its `Disclosure` is
  closed, where the `<details>` it replaces held them hidden. Closed, it
  shows the status in force.
- docz's own `useActiveHeading` hook is deleted; the package's takes
  the band docz used as its `rootMargin`.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 7: The frame

The top bar and rail are replaced by the package's sidebar and top bar.
This is the phase a reader sees.

<!--docz:tasks:start-->
#### Tasks

- [x] Read the package's `SidebarProvider` and `use-tree-open` at the
  pinned version and record where each keeps its state (a cookie or
  `localStorage`) and under what key. If either uses a cookie, confirm
  `ui/server` neither forwards it to docz-api nor logs it.
- [x] `AppShell.tsx`: `SidebarProvider`, `ShellSidebar`, and
  `TopBarLayout` around the `Outlet`, with `RestoreAfterLogin` kept.
- [x] The sidebar's header (the logo mark and name), the `Browse` group
  (Directory, Repositories), and the `Pinned` group from
  `enabledNavLinks()`, absent when there are none.
- [x] The repository's groups on a repository's routes: Home and
  Changelog, the pages tree, and the documents tree, replacing
  `repo-nav.tsx` and `repo-nav-pages.tsx`.
- [x] The documents tree as `ShellSidebarTree`, one row per type with
  its count, fetching a type's documents when its row first opens,
  including through the icon rail's popover.
- [x] Loading inside a tree as `InlineLoading` in a
  `ShellSidebarTreeNote`.
- [x] The type's colour by DESIGN-0028 question 3: on the type row's
  dot, on `Dot` and `Badge`, and inside the pane. Remove
  `--type-accent` from the logo mark, the breadcrumb leaf, and the
  active row.
- [x] `RepoBreadcrumbs` as the package's `Breadcrumb` in the top bar.
- [x] `shell/search-trigger` and `shell/mode-toggle` in the top bar,
  the toggle driven by `useAppearance`. Retire
  `appearance-controls.tsx`.
- [x] Move the account menu to the sidebar's footer.
- [x] Render `/login` and `/consent` outside the frame.
- [x] Check the narrow layout: the sidebar is the package's sheet, and
  the document is readable at 360 px.
- [x] Site-rendered code: Shiki with `github-light` and `github-dark`
  (or the package's own pair, by DESIGN-0028 question 6) and
  `defaultColor: false`, with CSS choosing the set from `.dark`.
- [x] `theme-contrast.ts`: `CODE_BG` as a light and dark pair pinned to
  the package's `--code-bg`, and `liftThemeContrast` run once per theme.
  Its test checks both.
- [x] Remove `code-bg` and `st-superseded` from the chrome's use of
  `docz-tokens.css`; both stay in the file for `charcoal`.
- [x] Rewrite `repo-nav.test.tsx` and `nav-pins.test.tsx` for the
  sidebar, and update the e2e specs that drive navigation
  (`mvp.spec.ts`, `pages.spec.ts`, `a11y.spec.ts`).
- [x] Add a specimen section for a code block in each mode, and check
  the specimen page first.
- [ ] Compare each screen with `ui/design/mockup.html` in both modes and
  attach screenshots to the pull request.
  **Deferred** - human required: the pull request is #238, opened
  2026-11-23. Screenshots of every screen in both modes were taken and
  looked at while building; the comparison against the mockup, and what
  to accept where they differ, is a person's. The author does it on the
  beta.
- [ ] Cut a beta from this branch and check it on the homelab (question
  5 below), since the release workflows' token wiring runs nowhere else.
  **Deferred** - human required: `v2.7.0-beta.1` was cut from this
  branch on 2026-11-23, on the author's word. `prerelease.yml` passed,
  and chart `1.5.0-beta.1` and both images are published. Deploying it
  to the homelab and checking it there is the author's.
- [x] `ui/CLAUDE.md`: rewrite the routing, shell, and per-type accent
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

Measured on 2026-11-23. The unit suite (1,575 tests) and the e2e suite
(92, with full-rule axe in both modes over the frame, the icon rail and
its popover, the sheet at 360 px, and `/login`) pass, with 33 parts
imported. The chunk assertions for mermaid, ELK, and the widgets hold.

**One criterion is not met: the eager JavaScript is 171.6 KB gzipped,
6.6 KB over the 165 KB ceiling.** It was 125.9 KB after Phase 6. The
budget in `bundle-budget.ts` is unchanged, so `just ui bundle-budget`
and with it `just ui ci` fail on this phase's commits. This needs a
decision, **deferred - human required**:

- What is left in the eager set is the package's `components/sidebar`
  module, which imports its `Sheet` and `Tooltip` statically, about
  45 KB of Base UI's dialog, focus, and positioning code. DESIGN-0028
  §12 says the frame cannot be lazy, and the package offers no way to
  take the sidebar without them.
- Two things were moved out to get from 190.7 KB to 171.6 KB: the
  repository's groups are a lazy chunk (`repo-sidebar.tsx`), and the
  app's own rows are the plain `SidebarMenuButton`, so the tree's
  popover, collapsible, and scroll area load on a repository's routes
  only.
- The choices: accept the measured total, which Phase 9's rule (the
  measured total plus 6 KB) would set as the budget anyway; or ask the
  package to put its sheet behind a lazy boundary and hold 165 KB.

Where the two stores are: `SidebarProvider` and `use-tree-open` keep
their state in React state only, with no cookie and no `localStorage`
key, so `ui/server` has nothing to forward or log. A reload opens the
sidebar again and resets a tree to the branch the route is in.

What differs from the list above:

- `repo-nav.test.tsx` is replaced by `repo-sidebar.test.tsx`, which
  mounts the whole app: which repository, type, and page is current
  comes from AppShell reading the route, and that is half of what it
  tests.
- The repository's group is named `owner/name` and has no line for the
  default branch, which the old rail's header carried.
- `appearance-controls.tsx` is deleted, so the review page's toggle is
  the package's `ModeToggle` already, ahead of Phase 8.
- `src/lib/mermaid-theme.ts` is renamed `diagram-theme.ts`. The bundler
  now emits it as a preloaded chunk named for the module, and
  `rendering.spec.ts` fails a diagram-free page on any request whose
  URL says "mermaid". The assertion is unchanged.
- Site-rendered code sits on the package's `--code-bg` inside every
  document theme, including a theme with one dark look. Its rules are
  `!important`, because the document's own stylesheet paints its
  theme's dark code surface and is not in a layer. GitHub's themes set
  comments upright, where `tokyo-night` set them in italics.
- `code-bg` and `st-superseded` are no longer registered in
  `tokens.css`'s `@theme` block, so `bg-code-bg` is the package's
  utility. Both remain in `docz-tokens.css`.
- Only the last breadcrumb is `aria-current="page"`; the old ones
  marked every crumb with no link. Below 768 px the last is the only
  one shown.
- The sign-in link's test id is `session-sign-in`.
- The specimen's new section is in the fixture copy under
  `ui/src/mocks/content/`, which is the page the site and both axe
  sweeps render. The archived copy under `docs/archive/ui/` is not
  rewritten.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 8: docz-review's page

The review page's controls are the package's, and its review logic is
untouched.

<!--docz:tasks:start-->
#### Tasks

- [x] `controls.tsx`, `status.tsx`, and `app.tsx`: `Button`, `Tabs`,
  `Badge`, `SegmentedControl`, and `InlineLoading`.
- [x] Any confirmation or submit prompt as the package's `Dialog`.
- [x] `questions.tsx`, `findings.tsx`, `regions.tsx`, and
  `annotations.tsx`: their buttons and badges on the package's parts,
  with the option cards' behaviour unchanged.
- [x] Keep the page's own single-column frame and header, with
  `shell/mode-toggle` for the mode. Do not import `shell/sidebar`.
- [x] Leave `diff.tsx`'s rendering alone and move only its colours'
  names. Check the diff in light mode against the fix in v2.6.1.
- [x] Confirm the document view still renders in the document's theme
  inside `.docz-page`, across every theme in `e2e/appearances.ts`.
- [x] Confirm no sidebar code is in the review bundle by searching
  `dist-review` for it.
- [x] Run `just ui build-review` and open a compiled binary's page with
  the network off.
- [x] Rewrite the affected tests in `src/review` and update
  `review.spec.ts` and `review-server.spec.ts` selectors.
- [ ] Walk one real review end to end with the plugin: comment, answer
  a question, submit, and confirm the agent receives the feedback.
  **Deferred** - human required: a review through the plugin needs a
  person in a Claude Code session with the released binary and a
  browser. The same path is covered unattended by
  `review-server.spec.ts` (the real server and a built docz) and by the
  plugin's hook tests.
- [x] `plugins/CLAUDE.md` and `ui/CLAUDE.md`: the review page's
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

Measured on 2026-11-23. The unit suite (1,581 tests), the review
server's suite (98), the plugin's hook tests (13), and the review e2e
specs (45: every document theme in both shell modes, with full-rule
axe over the page and over an open dialog) pass, with 35 parts
imported. The feedback goldens under `src/review/testdata/feedback/`
are unchanged. `just ui build-review` builds the four binaries and its
smoke test passes.

How three of the tasks were checked:

- **The binary with no network.** The compiled `darwin_arm64` binary
  was driven through a whole review (open, answer, the discard dialog,
  submit, session ended) in a browser that aborted every request to a
  host other than `127.0.0.1`. The page attempted none, and its
  stylesheet and fonts came from the binary. That is not the same as
  unplugging the machine, and no person has opened the page by hand.
- **The diff in light mode.** `contrast.test.ts` holds the three line
  pairs to 4.5:1 in both modes in the package's names, which is the
  test v2.6.1 added. The diff was not looked at in a browser: the fake
  server has no hook for a second round.
- **Sidebar code.** No JavaScript file in `dist-review` mentions the
  sidebar. The stylesheet does (84 times): both pages share
  `tokens.css`, which imports every part's rules.

What differs from the list above:

- **The dev server was broken, and is fixed here.** Base UI's store
  imports a CommonJS shim that Vite does not pre-bundle for a file
  under `node_modules`, so any page with a dialog, tabs, a popover, or
  a menu was blank under `vite` in dev: the review page with this
  phase's parts, and the site since the frame mounted in Phase 7. A
  build was never affected, which is why the preview-based e2e passed.
  `vite.config.ts` now names the shim's two entries in
  `optimizeDeps.include`. This is the package's to document for every
  consumer on Vite, and goes in the report on its IMPL-0001 Phase 6.
- The documents are the package's `Tabs` and the status is the
  `SegmentedControl`. The status was a native `<select>`; its segments
  wrap, because a type can have more statuses than the sidebar's width
  holds.
- The two prompts are one `ConfirmDialog`. "Leave without an outcome
  for the open documents?" and "Discard this draft and approve?" keep
  their wording as the dialogs' titles, and each gains a sentence
  saying what is lost.
- `regions.tsx` has no control and no colour, and is unchanged.
- Text boxes stay native `textarea` and `input` elements in the
  package's colours; the design's list for this page has no input
  part.
- A removed diff line is `foreground`, where it was one step dimmer
  than an added one: the package has no text step between `foreground`
  and `muted-foreground`.
- A disabled button keeps its pointer, so the `title` that says why it
  is disabled still shows; the package's `Button` turns the pointer
  off.
- The heading locators in both review specs gain `exact: true`. The
  document's own h1 contains the title, and the real-server spec now
  sees both at once.

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 9: Removal and close-out

Nothing of the old shell is left, the budget is final, and the records
are flipped.

<!--docz:tasks:start-->
#### Tasks

- [x] Delete `shell.css` and `shell.test.ts`, and the `:root` aliases
  Phase 2 added.
- [x] Reduce `tokens.css` to the imports, docz's fonts, the
  document-scoped utility names, and the prose and chip rules.
- [x] Add the vocabulary test: a document-scoped utility name outside
  `src/markdown`, `doc-pane.tsx`, `doc-surface.tsx`, and the review
  page's document view fails.
- [x] Delete the components the package replaced and anything left
  unused: confirm with a search for each file's exports.
- [x] Remove `cmdk` from `ui/package.json` if only the package imports
  it now, and any other dependency left without an importer.
- [x] Measure the eager JavaScript and set the budget to that total
  plus 6 KB, with the comment in `bundle-budget.ts` stating both
  numbers.
- [x] Record the final stylesheet and JavaScript sizes beside
  INV-0036's in this document.
- [x] `ui/CLAUDE.md`: remove what describes the old shell, including
  `mockup.html` as "the visual source of truth", and point at the
  package's Storybook instead. `mockup.html` and `ui/design/` stay.
- [x] Root `CLAUDE.md`: one line, only if the map changed.
- [ ] Report back on the package's IMPL-0001 Phase 6: which tasks are
  done, and that the export keeps `data-mode` by INV-0036 question 2.
  **Deferred** - human required: a report on another repository's plan
  is the author's to post. What it should carry is listed under the
  measurements below.
- [x] Open an issue for the app switcher fast follow, with the mockup's
  section and the runtime setting it needs.
  Opened 2026-11-23 as #239.
- [ ] `docz status set`: DESIGN-0028 to Implemented and this document
  to Completed.
  **Deferred** - human required: the statuses flip once the branch is
  merged and the checks deferred to a person in Phases 5, 7, 8, and 9
  are done. Setting them on the branch would say so before it is true.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- A search of `ui/src` finds no `shell.css`, no `:root[data-mode]`
  token block, and no docz-named utility in chrome.
- `just ci` and `just ui ci` pass with the final budget.
- The released site and docz-review binaries are checked once on the
  homelab.
- `docz validate` passes, and both documents show their final status.

Measured on 2026-11-23. `just ci` and `just ui ci` pass: 1,545 unit
tests, 98 e2e, the review server's 98, and the four generated-file
checks, with 35 parts imported. A search of `ui/src` finds no
`shell.css`, no `:root[data-mode]` block, and no docz-named utility in
chrome, which `vocabulary.test.ts` now holds.

The sizes, beside INV-0036's:

<!-- markdownlint-capture -->
<!-- markdownlint-disable MD013 -->

| Artifact | `main` before | INV-0036's spike | Final |
| --- | --- | --- | --- |
| Stylesheet | 68.0 KB (16.6 KB gzipped) | 107.9 KB (23.6 KB) with `core.css` | 120.2 KB (25.2 KB) |
| Eager JavaScript, gzipped | 124.0 KB | 156.6 KB with the frame beside the old shell | 171.6 KB |
| Bundle budget | 130 KB | | 177.6 KB |

<!-- markdownlint-restore -->

**The budget is 177.6 KB: the measured 171.6 KB plus 6 KB, as this
phase's rule and DESIGN-0028 question 1 say.** That is 12.6 KB above
the 165 KB the design expected the frame to fit. About 45 KB of the
total is the package's `components/sidebar`, which imports its sheet
and tooltip statically. If the package puts the sheet behind a lazy
boundary the total falls, and the number should be measured again and
lowered. This is the decision Phase 7's note left open; taking the
phase's rule closes it unless the author prefers to wait for the
package.

What the report on the package's IMPL-0001 Phase 6 should carry:

- docz imports 35 parts by subpath with `core.css` under
  `source(none)`, and the export keeps `data-mode` by INV-0036
  question 2.
- `components/sidebar` imports `Sheet` and `Tooltip` statically, about
  45 KB gzipped in a consumer's first paint.
- Under Vite's dev server a consumer needs
  `use-sync-external-store/shim` and its `with-selector` entry in
  `optimizeDeps.include`, or any page with a dialog, tabs, a popover,
  or a menu is blank. The package ships source, so Vite does not
  pre-bundle Base UI's CommonJS dependency for it.
- `ListRow` draws `trailing` above its stretched link, so a badge or a
  time there is not part of the link.
- A `ListRow` with a link cannot sit inside a `Command` item: the item
  is an option, and axe fails a link inside one.
- There is no text step between `foreground` and `muted-foreground`,
  which docz's own `fg-secondary` and `fg-tertiary` were.

What differs from the list above:

- **The statuses are not flipped.** DESIGN-0028 and this document stay
  Draft until the branch is merged and a person has done the deferred
  checks: the keyboard pass (Phase 5), the mockup
  comparison and the beta on the homelab (Phase 7), a review through
  the plugin (Phase 8), and the released site and binaries on the
  homelab (this phase's third criterion).
- The document-scoped names are registered `@theme inline`, each as
  its own value, so no stylesheet maps a docz name onto a package
  token. Tailwind still emits such a name at `:root` as itself where a
  rule reads it, which is a cycle and so no value.
- `border-strong` joins `code-bg` and `st-superseded` as a name docz
  does not register: it is the package's utility too, and the
  directory's and the repository list's hover borders use it.
- Three things read a colour outside a utility and were moved by hand.
  A custom type's colour is `var(--color-hash-N, var(--category-N+1))`,
  the document's inside a `.docz-page` and the package's in chrome.
  mermaid's theme reads the package's `--card`, `--secondary`,
  `--border-strong`, and `--foreground` from `<html>`. The page's
  surface, text, selection, focus ring, and body size are `core.css`'s,
  so text with no size of its own in chrome is 14px, where docz's body
  rule set 16px.
- `tokens.css` keeps three behaviours the task's list does not name:
  smooth scrolling, the scrollbar gutter with thin scrollbars, and the
  review page's highlight and changed-region badge.
- `shell.test.ts`'s checks that still mean something moved:
  `vocabulary.test.ts` holds which names are not registered, and
  `contrast.test.ts` still checks the shell under `pkg/theme`'s 58
  names in both modes, read through `charcoal.ts`'s derivation table.
- No component file was left to delete: each was removed in the phase
  that replaced it, and a search finds no module without an importer.
  The one unused thing was the `.mono-chip-y` class, which is gone.
- Root `CLAUDE.md` is unchanged: its map did not change.
- Phase 8 missed one selector. `widgets.spec.ts` looked for the review
  page's old document buttons and is fixed here; an axe check over the
  icon rail's popover now waits for the popover to finish fading in.

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
