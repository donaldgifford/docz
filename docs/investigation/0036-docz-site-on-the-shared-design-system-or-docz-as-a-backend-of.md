---
id: INV-0036
title: "docz-site on the shared design system, or docz as a backend of repo-guardian's UI"
status: Concluded
author: Donald Gifford
created: 2026-11-19
---

<!-- markdownlint-disable-file MD025 MD041 -->

# INV-0036: docz-site on the shared design system, or docz as a backend of repo-guardian's UI

<!--toc:start-->
<!-- markdownlint-capture -->
<!-- markdownlint-disable MD013 -->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
  - [Observation 1: the package already plans for docz](#observation-1-the-package-already-plans-for-docz)
  - [Observation 2: what the package is](#observation-2-what-the-package-is)
  - [Observation 3: docz-site by area](#observation-3-docz-site-by-area)
  - [Observation 4: docz's token names are not the package's](#observation-4-doczs-token-names-are-not-the-packages)
  - [Observation 5: what the same move cost repo-guardian](#observation-5-what-the-same-move-cost-repo-guardian)
  - [Observation 6: A has a Go half](#observation-6-a-has-a-go-half)
  - [Observation 7: what B needs from docz-api](#observation-7-what-b-needs-from-docz-api)
  - [Observation 8: what B moves, and what it leaves behind](#observation-8-what-b-moves-and-what-it-leaves-behind)
  - [Observation 9: what a sketch of A shows](#observation-9-what-a-sketch-of-a-shows)
  - [Observation 10: the spike](#observation-10-the-spike)
  - [Observation 11: the Go side](#observation-11-the-go-side)
  - [Observation 12: the package's answer, and 0.5.0](#observation-12-the-packages-answer-and-050)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
- [Open Questions](#open-questions)
  - [1. Does docz-site take the package's frame?](#1-does-docz-site-take-the-packages-frame)
  - [2. How does the mode switch change?](#2-how-does-the-mode-switch-change)
  - [3. How does pkg/theme get the package's tokens?](#3-how-does-pkgtheme-get-the-packages-tokens)
  - [4. Where do docz's extra colours live?](#4-where-do-doczs-extra-colours-live)
  - [5. Does docz-review's page adopt the package too?](#5-does-docz-reviews-page-adopt-the-package-too)
  - [6. Do the widgets move onto the package's flow and stepper blocks?](#6-do-the-widgets-move-onto-the-packages-flow-and-stepper-blocks)
  - [7. When does docz adopt, given the package is 0.x?](#7-when-does-docz-adopt-given-the-package-is-0x)
- [References](#references)
<!-- markdownlint-restore -->
<!--toc:end-->

<!--docz:question:start-->
## Question

docz-site (`ui/`) wears repo-guardian's look through tokens it copied in
(DESIGN-0026). Two ways to stop keeping that copy are on the table:

- **A. Adopt the package.** docz-site installs
  `@donaldgifford/design-system` and takes its tokens, primitives, and
  shell, as repo-guardian did in its IMPL-0037.
- **B. Move into repo-guardian's UI.** docz-api becomes a second backend
  of repo-guardian's UI, and the reader, the directory, and search are
  rebuilt there as docz features. docz-site is retired or reduced.

Which of the two should docz do, and what does the one it picks cost?

<!--docz:question:end-->

<!--docz:hypothesis:start-->
## Hypothesis

A is the answer and B is not.

- A is already the plan on the package's side: its RFC-0001 names docz as
  the second consumer and its IMPL-0001 has a "docz adoption" phase with
  five open tasks. The work is a restyle of the shell, about a third of
  `ui/src`, and leaves the markdown pipeline, the document pane, and
  docz-review alone.
- B makes every docz deployment a repo-guardian deployment, which needs
  Temporal, Postgres, and two GitHub Apps, and it does not remove docz's
  frontend: docz-review's page is built from `ui/` and stays here.

What a merged UI would give a reader, one place to go from a repository's
compliance to its documents, can be had with links between two apps that
look the same.

<!--docz:hypothesis:end-->

<!--docz:context:start-->
## Context

repo-guardian finished its move onto the package on 2026-11-18
(repo-guardian IMPL-0037, PR #255, package 0.3.2). docz is the package's
other named consumer and has not started. DESIGN-0026 listed "importing a
design-system package" as a non-goal because no package existed; one does
now, so that non-goal is due a second look. The idea of putting docz
inside repo-guardian's UI came up as the alternative to doing the same
migration twice.

**Triggered by:** design-system RFC-0001 and IMPL-0001 Phase 6, DESIGN-0026

<!--docz:context:end-->

<!--docz:approach:start-->
## Approach

1. Read the package at 0.3.2: its exports, token names, layers, and what
   its own documents say about docz. Done; Observations 1 and 2.
2. Size docz-site by area and count what is written against docz's own
   token names. Done; Observations 3 and 4.
3. Read repo-guardian's adoption (IMPL-0037, PR #255, design-system#11)
   for what it cost and what went wrong. Done; Observation 5.
4. Read repo-guardian's UI and BFF, and docz-api's gate, for what B needs
   from each side. Done; Observations 6 to 8.
5. Spike A on a branch: install the package, import `globals.css`, map
   the shell's tokens, and run the contrast test, the unit suite, the
   bundle budget, and `Dockerfile.ui` with the registry token. Done;
   Observation 10.
6. Design the Go side of A: how `pkg/theme`'s `charcoal` is derived from
   the package's `tokens.css` and held to it. Done; Observation 11.
7. Sketch A as static HTML on the package's tokens: docz-site's screens
   with the package's parts, and a page of the parts docz has that the
   package lacks. Done; Observation 9.

Steps 1 to 4 were done by reading source on 2026-11-19, and step 7 by
hand on 2026-11-21. The package was not installed or built for either,
so step 7 is not the spike in step 5. Step 5 was run on 2026-11-21 on a
local branch, `chore/spike-design-system`, which is not for merge.

<!--docz:approach:end-->

<!--docz:environment:start-->
## Environment

| Component | Version / Value |
| --------- | --------------- |
| `@donaldgifford/design-system` | 0.3.2 (`064bb78`), GitHub Packages; 0.5.0 and 0.6.0 for Observation 12 |
| repo-guardian | `main` at `cc97e3f`, UI on the package at 0.3.2 |
| docz | `main` at `0aa53f1` |
| docz-site | React 19, react-router 8, Tailwind 4.3, Vite 8, vitest in jsdom |
| repo-guardian UI | React 19, TanStack Router, Tailwind 4.3, Vite 8, `bun test` in happy-dom, a Bun and Hono BFF |

<!--docz:environment:end-->

<!--docz:findings:start-->
## Findings

### Observation 1: the package already plans for docz

The package's RFC-0001 is titled for both apps, and its success criteria
include "docz's shell stylesheet and its duplicated dark token block are
gone, and its `charcoal` theme is derived from `tokens.css`". Its
IMPL-0001 Phase 6, "docz adoption", is unstarted:

- decide how the Go export gets `tokens.css` (RFC-0001 open question 1)
- replace the shell stylesheet and the duplicated dark block
- derive the `charcoal` document theme from `tokens.css`
- switch the mode toggle from `data-mode` to `.dark`, in the app and the
  export
- replace the dropdowns, command palette, and breadcrumbs

The RFC also says what stays in docz: the review UI and the document
themes (`paper`, `classic`), which "control how a document reads, not how
the app looks". So A is the package's stated plan, and this investigation
is deciding whether docz still agrees with it.

### Observation 2: what the package is

- No build step. A consumer's Vite and Tailwind v4 compile its TypeScript
  and CSS. Imports are by subpath (`components/<name>`, `blocks/<name>`,
  `shell/<name>`, `lib/<name>`), never a barrel.
- `tokens.css` imports nothing and works on a plain HTML page, which the
  RFC added for docz's export.
- One look, repo-guardian's, in light and dark. Dark is a `.dark` class on
  `<html>`. A second look needs a new RFC.
- Primitives sit on Base UI, not Radix. docz-site has neither today; its
  one overlay library is cmdk, which the package also uses.
- Published to GitHub Packages, which wants a token even to read a public
  package.
- 0.x, with breaking changes expected between minors until 1.0. Its open
  question 3 proposes that 1.0 means both apps have adopted it.
- It has blocks docz-site draws by hand today: `flow`, `stepper`,
  `diff-view`, `code-block`, `timeline`, `empty-state`, `error-state`,
  `page-skeleton`, `filter-chip`, `data-table`.

### Observation 3: docz-site by area

Lines of TypeScript and CSS, tests and generated files left out:

| Area | Lines | Touched by A |
| ---- | ----- | ------------ |
| `src/markdown` (pipeline, widgets, mermaid) | 3,191 | colours only |
| `src/review` (docz-review's page) | 3,269 | colours, and its controls |
| `src/components` | 2,199 | yes |
| `src/routes` | 2,106 | yes |
| `src/lib` | 1,222 | `appearance.ts`, `theme.ts`, `colors.ts` |
| `src/theme` (`shell.css`, `tokens.css`, `scope-css.ts`) | 1,015 | mostly deleted |
| `src/app` | 321 | yes |
| `server` (the Bun server) | 1,342 | no |
| `review` (the docz-review binary) | 2,487 | no |

The shell proper, `app`, `components`, `routes`, and `theme`, is about
5,600 lines. repo-guardian's whole `ui/web/src` is 5,487.

### Observation 4: docz's token names are not the package's

docz-site's utilities are written against docz's names: 498 uses across
33 files (`bg-bg-raised`, `text-fg-muted`, `border-border-default`), and
268 `var(--color-*)` references. The package uses shadcn's names
(`bg-card`, `text-muted-foreground`, `border-border`). `shell.css`
already records the mapping, since it was written by copying
repo-guardian's values onto docz's names.

Three things in the mapping are not one-for-one:

- **`accent` means two things.** docz's `--color-accent` is the brand
  colour. The package's Tailwind theme sets `--color-accent` to its
  `--accent`, a hover surface, and the brand colour is `primary`. Any
  `bg-accent` left behind compiles and paints the wrong colour.
  repo-guardian's IMPL-0037 carried a table for the same clash.
- **docz has steps the package lacks**: `bg-hover`, `bg-active`,
  `border-hairline`, `border-strong`, `fg-secondary`, `fg-tertiary`,
  `fg-prose`. Each is dropped, mapped to a neighbour, or proposed
  upstream.
- **docz has domain colours**: `t-*` per document type, `hash-*`, `st-*`
  per status, `adm-*` per alert. These are the "mapping from an app's
  states" the RFC leaves in the app. The statuses could ride the
  package's badge variants (`pending`, `approved`, `ignored`, `running`),
  but docz has more statuses than the package has states.

The document pane is safe from all of this. A document is styled inside
`.docz-page` by `page.css`, generated from `pkg/theme`, and the package
styles nothing by a selector outside its own components. The one name
both define is `--color-accent`, and `page.css` sets it inside its own
scope.

### Observation 5: what the same move cost repo-guardian

- PR #255 touched 129 files under `ui/`, +2,538 and -4,553, in eight
  phases: install and auth, stylesheet and tokens, leaves, primitives and
  overlays, tables, shell, removal, release.
- Before designing it, repo-guardian compared the package with its own
  UI and filed design-system#11: 29 gaps and 19 differences. The fixes
  went upstream as 0.2 and 0.3. docz will find its own list; the reader's
  directory and palette are not shaped like repo-guardian's tables.
- The frame changed. The package's shell is a full-height collapsible
  sidebar with the top bar beside it. docz-site's frame is a top bar and
  a per-repository rail, so A is where docz decides whether it takes the
  package's frame or keeps its own on the package's primitives.
- JavaScript grew from 989 kB to 1,187 kB and CSS from 31 kB to 103 kB.
  `just ui ci` has a `bundle-budget` gate, so A moves that number.
- The registry token reached every install path. docz has seven:
  `just ui install`, `Dockerfile.ui`, three jobs in `ci.yml`, and the
  docz-review build in `release.yml` and `prerelease.yml`. The image
  build needs it as a build secret, and Renovate needs its own.
- Base UI and cmdk decide at import whether a document exists, which
  cost repo-guardian a preload file and an order-dependent test failure.
  docz's suite is vitest in jsdom, which the package's README treats as
  the harder case (cmdk needs a `ResizeObserver` jsdom lacks; docz
  already stubs one).

### Observation 6: A has a Go half

`pkg/theme` is a public, semver-governed package with no dependencies,
and its `charcoal.css` is repo-guardian's look written by hand. Deriving
it from the package's `tokens.css` means a file from an npm package that
needs a token has to reach a Go `//go:embed`. `just vendor-mermaid` is
the precedent: a recipe copies from `ui/node_modules`, and a test holds
the copy to the version `ui/bun.lock` pins.

Two details make this more than a copy:

- The package's tokens are oklch. `pkg/theme`'s tests and
  `contrast.test.ts` read hex, and `components.css` may hold no literal
  colour.
- The mode switch. Exported pages, the reader, and docz-review share
  `data-mode` on `<html>` and the `docz:mode` storage key, read by
  `runtime.js`, three token files, `layout.go`, `scope-css.ts`, and
  `appearance.ts`. The package wants `.dark`. Either the site sets both,
  or the export changes, and "exported HTML pages look the same as
  before" is the package's own criterion for this phase.

### Observation 7: what B needs from docz-api

repo-guardian's BFF holds the session. The browser has one sealed
cookie, and the BFF proxies `GET` and `HEAD` under `/api/*` to one
`API_UPSTREAM` with the user's OIDC access token as a bearer. docz-api
would be a second upstream under a second prefix, and its gate does not
fit that as built:

- **A bearer is read only while MCP is on.** With `MCP_ENABLED` off the
  verifier is nil and the header is ignored, so B makes the MCP stack a
  requirement of the UI.
- **The audience is `<OAUTH_ISSUER>/mcp`.** The token the BFF holds is
  minted for repo-guardian's API. docz-api would need a second accepted
  audience, or the BFF a second token.
- **Token callers are rate limited.** `/api/v1` requests with
  `Via == token` draw on `MCP_RATE_LIMIT`, sized for agents, not for a
  person paging through a directory.
- **The two authorize differently.** repo-guardian maps groups to orgs.
  docz maps scopes to repositories through `access.scopes` and
  `AUTHZ_SCOPE_MAP`. One sign-in would be checked two ways.
- **The consent page is docz-site's.** `DOCZ_SITE_URL` is where the
  OAuth server sends a person to approve an MCP client; that route would
  have to move too.

### Observation 8: what B moves, and what it leaves behind

To read a document inside repo-guardian's UI, that UI needs:

- the document pane, which injects docz-api's HTML fragment, mounts
  `pkg/theme/runtime.js` on it, and styles it with `page.css`. Both are
  generated from `pkg/theme` in this repository at build time, so
  repo-guardian's build would read docz's Go tree or a published copy of
  it;
- the markdown pipeline for pages, the changelog, and the fallback
  render, with shiki and mermaid, which are most of docz-site's bundle;
- the directory, search, the palette's document results, and the routes,
  rewritten from react-router and orval onto TanStack Router and
  openapi-fetch;
- a second generated client, pinned to docz-api's spec, in a repository
  that releases on its own schedule.

What B does not remove:

- **docz-review.** Its page is `ui/`'s second Vite entry and reuses the
  reader's markdown pipeline. It ships inside a local binary and cannot
  move. docz keeps a React project either way, and under B that project
  and repo-guardian's would each hold a copy of the pipeline.
- **A way to read docz without repo-guardian.** repo-guardian needs a
  Temporal cluster, Postgres, and two GitHub Apps. `charts/docz` installs
  the API, the site, and its backends in one release. Under B a docz-only
  install has no reader unless docz-site stays, and then there are two.

### Observation 9: what a sketch of A shows

`ui/design/` holds two static pages drawn by hand on a copy of the
package's `tokens.css` at 0.3.2, after boop's `ui/design/candidates.html`.
They were rendered in Chromium in both themes. They are something to look
at, not the package's output.

`mockup.html` draws the directory, a document, a type's index, the
repositories page, the menus, and the states. It draws the document twice,
in the package's frame and in docz's own, which is open question 1 side by
side. Its first table maps the shell's twenty parts to what would draw each:
eleven the package has at 0.3.2, one that waits on its working tree,
seven that need something the package lacks, and the document pane,
which stays docz's.

`candidates.html` is those gaps, nine proposals in all:

1. A sidebar tree: collapsible rows with a count, nested to any depth.
   The package's `SidebarMenuSub` is one level and always open. This is
   the largest of the nine.
2. An outline, "On this page", with a scroll spy.
3. A search result row: an eyebrow and a snippet on `ListRow`, and a
   two-line time.
4. Filter, preview, and footer slots on `CommandDialog`.
5. A search trigger and a mode toggle for the top bar.
6. A vertical `Stepper`, for a document's lifecycle.
7. A disclosure. The package has no collapsible primitive.
8. A tone the app supplies to `Badge` and `Dot`, for docz's type
   colours, and a categorical palette. `Superseded` is the one docz
   status with no package variant to take.
9. Two tokens, a stronger border and a pressed surface, and one
   decision: the package's `--code-bg` follows the page, and docz's code
   stays dark in both modes.

Two things the sketch found that the reading did not:

- Four of boop's candidates (`SegmentedControl`, `OptionRows`,
  `FilterBar`, and the inline loading and empty text) are in the
  package's working tree, unreleased. docz would use all four: the format
  switch, the repository and type pickers, and the directory's filter row.
- The package's filter is a `key = value` chip that opens an editor.
  docz's type filter is a row of toggle chips, one click to change. The
  mockup draws both; keeping docz's means keeping it local or proposing
  a tenth part.

### Observation 10: the spike

The spike installed the package at 0.3.2 in `ui/`, replaced
`@import "tailwindcss"` in `tokens.css` with the package's `globals.css`,
pointed the shell's eleven shared tokens at the package's (`bg-base` at
`--background`, `accent` at `--primary`, and so on, in `shell.css` and
the `@theme` block), set the `.dark` class beside `data-mode`, and
mounted a hidden probe in `AppShell` holding package components.

<!-- markdownlint-capture -->
<!-- markdownlint-disable MD013 -->

| Check | Result | What it took |
| --- | --- | --- |
| Contrast test | Passes, 428 of 428 | The test reads hex only, so it lost every mapped token (179 failures as written). With a resolver that follows `var()` into the package's `tokens.css` and converts oklch, it passes. |
| Unit suite | Passes, 1439 of 1439 | No vitest change for a button and a badge. The package's sidebar reads `window.matchMedia`, which jsdom lacks (134 failures), so the test setup needs a stub. |
| Bundle budget | Fails | Eager JS is 124.0 KB gzipped before, 137.3 KB with a button and a badge, and 156.6 KB with the sidebar frame and top bar, against a 130 KB budget. |
| `Dockerfile.ui` | Builds | `.npmrc` copied beside `package.json`, and the install run with `--mount=type=secret,id=NPM_TOKEN`. The image's history holds no token and the runtime stage holds `dist` and `server` only. |

<!-- markdownlint-restore -->

Also run: `tsc -b` passes with the package's source compiled under
docz's strict settings, and eslint has nothing to say about the spike's
code.

What the spike found beyond the four checks:

- **The values already agree.** Each of the eleven package tokens
  converts to the hex the shell carries today, in both modes, so the
  mapping changes no colour.
- **The budget is the one gate that does not hold.** The first two
  components cost 13.3 KB (Base UI's button, `class-variance-authority`,
  `tailwind-merge`), and the frame 19.3 KB more. docz's own shell and
  palette leave as the package's arrive, so the net is smaller than the
  sum, but the frame cannot be lazy. The DESIGN has to set a new number.
- **The stylesheet more than doubles**, 68.0 KB to 149.7 KB (16.6 KB to
  29.4 KB gzipped), because `globals.css` scans the whole package, not
  the components docz imports. It also emits five static font files
  beside docz's variable ones.
- **`accent` means two things.** In the package `bg-accent` is a hover
  surface; in docz `--color-accent` is the sky accent, used by 47
  utilities and 17 `var()` reads, and a type's routes override it. docz's
  `@theme` block comes second and wins, so a package component's hover
  surface would be the accent colour. docz's name has to change.
- **docz wipes the radius scale** (`--radius-*: initial`), which removes
  `rounded-md`, `rounded-lg`, and `rounded-xs` from the build, so package
  components that use them render square. The wipe has to go.
- **Dark needs script.** The package has no `prefers-color-scheme` block,
  so following the system means setting `.dark` before the first paint,
  which `applyStoredMode` already runs early enough to do.
- **A cached image build hides a missing secret.** A second build
  without the secret succeeded from cache, so CI has to pass the secret
  on every build, not only the first.

### Observation 11: the Go side

Open question 3 chose a recipe that generates `charcoal.css` from the
package. Read against the package at 0.3.2 and docz at `0744925`, that
recipe is smaller than Observation 6 feared.

- **The source is `tokens.json`, not `tokens.css`.** The package ships
  one, generated from its stylesheet, with a `light` and a `dark` set
  whose values are oklch or a `{name}` reference to another token. A
  generator resolves the references and needs no CSS parser.
- **oklch stops at the generator.** It converts each value to sRGB hex
  and writes hex, so `pkg/theme`'s tests, `contrast.test.ts`, and the
  export's goldens keep reading what they read today, and `pkg/theme`
  stays standard library only.
- **The shell already is the generated file.** `shell.css`'s dark block
  has the same 58 token names as `charcoal.css`. Eleven come straight
  from the package and convert to the shell's hex exactly, in both
  modes. Nine more follow from those by rule in the dark set (`link` is
  the accent, `st-draft` is `warn`, `st-deprecated` is `fg-muted`, and
  so on), and
  the two accent tints are the accent at 10% and 30%. That is 22 of 58
  from the package. The other 36 are docz's own: four surface and border
  steps, three text steps, `accent-dim`, `st-superseded`, five alert
  backgrounds, thirteen type colours, eight hash colours, and `code-bg`,
  all already fitted to 4.5:1 on the package's surfaces.
- **`charcoal` moves a little.** It was written from repo-guardian's
  DESIGN-0036 by hand, and only three of its 58 values equal the
  shell's. Surfaces move by one or two steps in a channel (`#17191d` to
  `#17191c`); `bad` moves from `#f4645c` to `#f7756e`; the type and hash
  colours become the shell's. Exported charcoal pages change by that
  much, and the per-theme page golden with them. `pkg/theme` is governed
  by name and token, not by byte, so this is not a breaking change.
- **A light mode comes free.** The package has a light set and the
  shell's light block has all 58 names, so a generated `charcoal` could
  carry both modes as `paper` does. Whether it should is the DESIGN's.

The mechanism, after `just vendor-mermaid`:

1. A recipe, run after `just ui install`, reads the package's
   `tokens.json` from `ui/node_modules` and writes `charcoal.css`'s token
   block from a table of the 22 package-derived tokens and a
   hand-written file of docz's 36. The font block stays hand-written.
2. The generated file records the package version it came from. A Go
   test holds that version to the one `ui/bun.lock` pins, as
   `TestVendoredMermaid` does, so a lockfile bump without the recipe
   fails.
3. The frontend's CI job, which has `node_modules`, regenerates and
   fails on a difference, as `gen-themes-check` does. The Go job cannot,
   so the version is its half and the bytes are the frontend's.

Two things the DESIGN has to carry:

- `ci.yml` names `bun.lock` in no path filter, so a pull request that
  only bumps the package would not run the Go test in step 2. The `go`
  filter needs `ui/bun.lock`. The same gap exists for mermaid today.
- The mode switch does not change here. Open question 2 keeps
  `data-mode` in `pkg/theme` and exported pages, and has the site set
  `.dark` beside it.

### Observation 12: the package's answer, and 0.5.0

design-system#32 was answered the day it was filed, on #11's rule: a gap
in a component is fixed in the package, and a design difference is the
package's design and docz adopts it. The four parts that were waiting in
the working tree shipped in 0.4.0, and 0.5.0, released on 2026-11-22,
carries the rest.

<!-- markdownlint-capture -->
<!-- markdownlint-disable MD013 -->

| Item in #32 | Decision | In 0.5.0 |
| --- | --- | --- |
| A1 sidebar tree | Package, after RFC-0003 | `shell/sidebar-tree`, `hooks/use-tree-open` |
| A2 outline | Package | `blocks/outline`, `hooks/use-active-heading` |
| A3 search result row | Package | `eyebrow`, `detail`, `render` on `ListRow`; `Time` `style="stacked"`; `blocks/marked` |
| A4 palette slots | Package | `CommandFilters`, `CommandFooter`, `preview` on `CommandDialog` |
| A5 top-bar controls | Package | `shell/search-trigger`, `shell/mode-toggle` |
| A6 vertical stepper | Package | `orientation="vertical"` and `size="sm"` on `Stepper` |
| A7 disclosure | Package | `components/collapsible`, `blocks/disclosure` |
| A8 a tone the app supplies | Package; `Superseded` maps to `ignored` | `tone` on `Badge` and `Dot`; `--category-1` to `--category-8` |
| A9 two tokens | Package | `--border-strong`, `--surface-pressed` |
| B1 `--code-bg` follows the page | Intended; docz adopts | docz moves to a light and dark Shiki theme pair |
| B2 toggle filter chip | Package | `pressed` on `FilterChip` |
| B3 `accent` | docz renames its token | A README note |
| C1, C2 stylesheet scans everything, static fonts | Package | `core.css` and `fonts.css`; `globals.css` unchanged |
| C3 no `prefers-color-scheme` | Package | A README note with the script |

<!-- markdownlint-restore -->

So none of the nine parts is docz's to build, and two decisions fall to
docz: its site-rendered code follows the page's mode, and `Superseded`
takes the `ignored` variant.

The release note on the issue adds three things for the DESIGN:

- `core.css` has to be imported with `source(none)`. Without it,
  Tailwind 4.3.3 takes the whole directory of any file named under
  `node_modules`, and the stylesheet is no smaller.
- A loading line in the sidebar tree is an `InlineLoading` inside a
  `ShellSidebarTreeNote`; a bare paragraph in the nested list is not
  valid markup.
- On the icon rail, opening a row's popover calls `onOpenChange(true)`
  on the top-level row only. docz fetches a type's documents when its
  row first opens, which that covers; a nested branch that loaded on
  open would show nothing there, and docz's pages tree has none, since
  it comes from one request.

0.6.0 followed the same day with two more. Each part has a stylesheet
under `parts/` that names the part's file and everything it imports, so
an app imports `parts/shell/sidebar.css` and writes no `@source` line
into `node_modules`. And a `ListRow` with both `render` and `onClick`
runs `onClick` when its link is followed, which 0.5.0 dropped and which
a search hit needs to close the palette. One limit stays on purpose: a
`Badge` with `tone` is tinted only, never filled. docz fills nothing
with a type colour, so that costs nothing.

The spike was moved to 0.6.0 and the per-part sheets. Four imports
replace ten `@source` lines and compile to the same stylesheet, byte
for byte in size, and the tests and `tsc -b` pass.

The spike branch was moved to 0.5.0 and its stylesheet to `core.css`
with `source(none)` and one `@source` line per part the probe uses. The
contrast test and the unit suite pass as before (1439 of 1439), and
`tsc -b` passes. The image build was not run again; nothing about the
install changed.

<!-- markdownlint-capture -->
<!-- markdownlint-disable MD013 -->

| Measure | Before | 0.3.2, `globals.css` | 0.5.0, `core.css` |
| --- | --- | --- | --- |
| Stylesheet | 68.0 KB (16.6 KB gzipped) | 149.7 KB (29.4 KB) | 107.9 KB (23.6 KB) |
| Static font files emitted | 0 | 5 | 0 |
| Eager JS, button and badge | 124.0 KB gzipped | 137.3 KB | 137.3 KB |
| Eager JS, with the frame | 124.0 KB gzipped | 156.6 KB | 156.6 KB |

<!-- markdownlint-restore -->

The stylesheet numbers on 0.5.0 include the classes of the frame's ten
parts, which the stylesheet names whether or not the probe mounts them.
The JavaScript is unchanged, so the bundle budget is still the one gate
that does not hold.

0.5.0's new tokens are docz's own values. `--border-strong` and
`--surface-pressed` convert to the shell's `border-strong` and
`bg-active`, and `--category-1` to `--category-8` to the shell's eight
hash colours, exactly, in both modes. That moves Observation 11's count
from 22 of 58 tokens to 32, and leaves 26 that are docz's own.

<!--docz:findings:end-->

<!--docz:conclusion:start-->
## Conclusion

**Answer:** Yes to A, no to B: adopt the package and do not merge the
UIs. The spike (Observation 10) passed three of docz's four gates with
small edits; the bundle budget does not hold and needs a new number.

- **B is ruled out by reading alone.** It couples installs (Observation
  8), needs changes to docz-api's gate that exist only to serve it
  (Observation 7), and does not retire docz's frontend, because
  docz-review stays.
- **A is plausible and already planned** (Observation 1), and it is
  about the size of repo-guardian's migration on the TypeScript side
  (Observations 3 and 5). It fits three of docz's gates: the 4.5:1
  contrast test, vitest in jsdom, and the image build with a registry
  token (Observation 10). It does not fit the 130 KB bundle budget, and
  two of docz's token choices, the name `accent` and the wiped radius
  scale, have to change before a package component renders correctly.
- **A's Go half is a generator and two checks** (Observation 11):
  `charcoal.css` written from the package's `tokens.json`, a Go test on
  the version, and a drift check in the frontend's CI. 22 of its 58
  tokens come from the package and exported charcoal pages shift by a
  few steps of colour.
- **A's gaps are filed** (Observation 9) as
  [design-system#32](https://github.com/donaldgifford/design-system/issues/32):
  nine parts the package lacks, three differences to decide, and three
  things the spike found in the package's stylesheet. All were answered,
  and 0.5.0 ships every part asked for (Observation 12), so docz builds
  none of the nine locally.

<!--docz:conclusion:end-->

<!--docz:recommendation:start-->
## Recommendation

1. The spike (step 5) is run and its four results are in Observation
   10.
2. The gaps were filed upstream as design-system#32 and are answered
   (Observation 12). The DESIGN targets 0.6.0 or later and imports
   `core.css` and the per-part sheets, not `globals.css`.
3. Write a DESIGN for A from the open questions' answers below, then an
   IMPL in repo-guardian's phase order: tokens first, components after,
   shell last. The DESIGN also has to set a new bundle budget, rename
   docz's `accent`, drop the radius wipe, add `ui/bun.lock` to CI's `go`
   path filter, decide whether the generated `charcoal` carries a light
   mode (Observation 11), and move site-rendered code to a Shiki theme
   pair that follows the page (Observation 12).
4. Record B as not doing. If one place to go from compliance to
   documents is still wanted, that is a link from repo-guardian's
   repository page to docz-site, which needs nothing from this work.

<!--docz:recommendation:end-->

<!--docz:open-questions:start-->
## Open Questions

### 1. Does docz-site take the package's frame?

- a. Yes: the package's sidebar and top bar, with the repository rail as
  sidebar groups. One frame across both apps.
- b. No: keep the top bar and rail, built from the package's primitives.
  Less change for readers, and a frame the package does not test.

> **Resolved 2026-11-21: (a).**

### 2. How does the mode switch change?

- a. The site sets both `.dark` and `data-mode`; exported pages and
  `pkg/theme` stay as they are.
- b. Everything moves to `.dark`, the export included, as the package's
  IMPL-0001 Phase 6 says.

> **Resolved 2026-11-21: (a).**

### 3. How does `pkg/theme` get the package's tokens?

- a. A recipe copies `tokens.css` from `ui/node_modules` and generates
  `charcoal.css` from it, with a test holding the two to the lockfile's
  version, as `just vendor-mermaid` does.
- b. `charcoal.css` stays hand-written, and a test in `ui/` compares its
  values with the package's.
- c. `charcoal` is left alone and only the shell adopts the package.

> **Resolved 2026-11-21: (a).**

### 4. Where do docz's extra colours live?

- a. In docz, as a small local token file beside the package's.
- b. Upstream, for the ones a second app would use (the surface and text
  steps), and in docz for the rest (types, hashes, alerts).

> **Resolved 2026-11-21: (b).** The two that go upstream are a stronger
> border and a pressed surface (`ui/design/candidates.html`, candidate 9).

### 5. Does docz-review's page adopt the package too?

- a. Yes, in the same IMPL: its controls are buttons, tabs, and dialogs.
- b. Later: it ships in a binary, and its bundle size and offline build
  are their own questions.

> **Resolved 2026-11-21: (a).**

### 6. Do the widgets move onto the package's `flow` and `stepper` blocks?

- a. No: the three surfaces are held to one corpus and `runtime.js`
  draws them without React.
- b. For the React surfaces only, if the blocks can draw the same
  validated data.

> **Resolved 2026-11-21: (a).**

### 7. When does docz adopt, given the package is 0.x?

- a. Now, pinned exactly, as repo-guardian did; docz's adoption is what
  the package's 1.0 waits for.
- b. After 1.0.

> **Resolved 2026-11-21: (a).**

<!--docz:open-questions:end-->

<!--docz:references:start-->
## References

- [DESIGN-0026](../design/0026-docz-sites-shell-in-repo-guardians-look-and-the-readers.md):
  the shell in repo-guardian's look, tokens copied in
- [DESIGN-0025](../design/0025-docz-html-standalone-html-pages-from-docz-documents-and-one.md):
  `pkg/theme` and the document themes
- [DESIGN-0023](../design/0023-a-read-only-mcp-server-in-docz-api-oauth-21-tokens-tools-and.md):
  docz-api's gate, scopes, and bearer tokens
- [`ui/design/mockup.html`](../../ui/design/mockup.html) and
  [`ui/design/candidates.html`](../../ui/design/candidates.html): the
  sketch of A and the parts to propose upstream
- [donaldgifford/design-system](https://github.com/donaldgifford/design-system):
  RFC-0001, IMPL-0001 Phase 6, and
  [issue 11](https://github.com/donaldgifford/design-system/issues/11)
- [donaldgifford/repo-guardian](https://github.com/donaldgifford/repo-guardian):
  INV-0025, DESIGN-0039, IMPL-0037, and
  [pull request 255](https://github.com/donaldgifford/repo-guardian/pull/255)

<!--docz:references:end-->
