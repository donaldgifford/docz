# CLAUDE.md

Guidance for Claude Code when working in docz-site.

## What this is

Web UI for [docz-api](https://github.com/donaldgifford/docz) — a
cross-repo docz reader, search directory, and repo pages. Vite + React 19
SPA, no SSR. The authoritative docs:

- `docs/design/0001-docz-site-cross-repo-docz-reader-and-search-ui.md` —
  design decisions 1–11 (stack, routes, reader pipeline, colors, auth)
- `docs/impl/0001-docz-site-mvp-phased-build-of-the-reader-directory-and-repo.md`
  — the phased build plan being executed (check off tasks as completed)
- `mockup.html` — visual source of truth (demo content; excluded from
  formatting, don't treat its text as project naming)
- `docs/input.md` — historical exploratory design; input only

## Commands

Bun is the package manager and script runner (pinned in `mise.toml`).

- `NPM_TOKEN=$(gh auth token) just ui install` — `@donaldgifford/design-system`
  (pinned exactly, no caret) comes from GitHub Packages, and `ui/.npmrc`
  reads the token from `NPM_TOKEN`. The token comes from the environment
  and is NEVER written to a file: not `.npmrc`, not `.env*`, not
  `mise.toml`, not a compose file. The recipe fails before Bun runs when
  the variable is unset. Every other install path takes the same
  variable: CI's install steps from the job's `GITHUB_TOKEN`
  (`packages: read`), and every image build as a BuildKit secret
  (`--mount=type=secret,id=NPM_TOKEN,env=NPM_TOKEN` in `Dockerfile.ui`;
  `_common_ui` in `docker-bake.hcl`, `just ui docker-build`, both compose
  files). Never an `ARG` or `ENV`: either would put the token in a layer.
  Secrets are not part of BuildKit's cache key, so a cached install
  layer hides a missing secret; test the wiring with `--no-cache`.
- `bun run dev` — Vite dev server; proxies `/api`, `/auth`,
  `/openapi.yaml` to a local docz-api (`:8080`; override `DOCZ_API_URL`)
- `bun run dev:msw` — dev server against MSW fixtures (no API needed)
- `bun run typecheck` — `tsc -b` (solution-style tsconfig; plain
  `tsc --noEmit` checks NOTHING at the root — don't "simplify" it back)
- `bun run build` / `preview` / `test` / `lint` / `format:check`
- `bun run gen-api` — orval regenerates `src/api/__generated__/` from
  docz-api's spec at the repository root, `../api/openapi.yaml`, read in
  place and no longer vendored (generated dir is gitignored)
- `bun run gen-themes` — writes `src/theme/__generated__/themes.ts`
  (`THEMES`, `ThemeName`, `DEFAULT_THEME`, `THEMES_WITH_MODES`) from
  `../pkg/theme/tokens/*.css`, and `page.css` beside it: pkg/theme's
  base, tokens, and components scoped to `.docz-page` by
  `src/theme/scope-css.ts` (DESIGN-0026 §5), which refuses any CSS shape
  it does not know; gitignored like the client, run by
  `just ui ci`, the image build, and CI, with `gen-themes-check` as the
  drift gate. A theme added to `pkg/theme` reaches every list in the UI
  (the server's `DOCZ_THEME` check, `defaultTheme()`, the review e2e
  matrix) and the document stylesheet through these files alone
- `just gen-charcoal` (at the ROOT, not `just ui`) — writes
  `../pkg/theme/tokens/charcoal.css` from the installed design system's
  `tokens.json` dark set (`scripts/gen-charcoal.ts`, IMPL-0033 Phase 3).
  The table of where each of the 58 names comes from is
  `src/theme/charcoal.ts` (pure, unit-tested; the script is the file
  I/O), the header prose and font block are
  `scripts/charcoal.template.css`, and docz's own 26 come from the dark
  block of `src/theme/docz-tokens.css`: change a value there and
  `charcoal.css` changes with it. `just ui gen-charcoal-check` is the
  drift gate (regenerates to a temporary file, diffs, rewrites nothing).
  A bump of the package is the lockfile, the recipe, and
  `go test ./... -update` in one pull request
- `just local-up` / `just local-down` — build + run the site container
  (`deploy/compose.local.yaml`, :8090) joined to the docz-api local
  stack's network; re-run `local-up` after changes to rebuild/recreate
  Bringing docz-api up for a real-stack check: its compose publishes
  :8080, which is often already taken on this machine. The site reaches
  it by service NAME over the shared network, so that publish is
  unnecessary — start it with a throwaway override setting
  `ports: !override []` rather than stopping whatever owns the port.

## Architecture

- `vite.config.ts` — react + tailwind plugins, `@/` → `src/` alias, dev
  proxy (same-origin in prod; docz-api sends no CORS headers).
  `optimizeDeps.include` names use-sync-external-store's two shim
  entries, for the DEV SERVER only: the design system ships TypeScript
  source, Vite neither pre-bundles that nor discovers what a file under
  `node_modules` imports, so Base UI arrives as its own ESM and its
  store (under every dialog, tabs, popover, and menu) imports a
  CommonJS shim with no named export. Without the two entries the page
  is blank in dev and fine in a build, so the preview-based e2e never
  sees it; `e2e/review.spec.ts`, which runs against the review dev
  server, does. A part that brings another CommonJS dependency needs
  its entry added there.
- Routing: `src/app/router.tsx` (`createBrowserRouter`, library mode).
  Every route renders inside `src/app/AppShell.tsx`, the frame, except
  `/login` and `/oauth/consent`, which sit in a second top-level route
  with its own error boundary and draw a card on the bare page: neither
  has a session to show navigation for. The frame's route stays FIRST in
  `routes` (`src/test/throwing-routes.ts` grafts onto `routes[0]`).
  Route modules live in `src/routes/*` and export a named `Component`;
  register new routes with `lazy: () => import("@/routes/<name>")` so
  each stays its own chunk.
- **The frame** (`src/app/AppShell.tsx`, DESIGN-0028 §6) is the design
  system's: `SidebarProvider`, `ShellSidebar`, and beside it
  `TopBarLayout` over the `Outlet`.
  The sidebar holds the docz mark (a link home), `Browse` (Directory,
  Repositories), `Pinned` (the deployment's nav links, absent when
  there are none), on a repository's routes that repository's groups,
  and the session at its foot. The top bar holds `SidebarTrigger`, the
  route's breadcrumbs, `SearchTrigger` (opens the palette), and
  `ModeToggle` (driven by `useAppearance`).
  - The column beside the sidebar is a plain `div`, NOT the package's
    `SidebarInset`: that is a `<main>`, and every route brings its own.
  - The package keeps the sidebar's open state and a tree's open
    branches in React state only: no cookie, no `localStorage`. A
    reload opens the sidebar again, and `ui/server` has nothing of it
    to forward or log. ⌘B / Ctrl+B, the icon rail, and the sheet below
    768px come with `SidebarProvider`.
  - **Breadcrumbs are the route's, drawn in the shell's bar.** AppShell
    puts the element at the bar's start in `TopBarSlotContext`
    (`components/top-bar-slot.ts`) and `RepoBreadcrumbs` portals the
    package's `Breadcrumb` into it; outside the frame the context is
    null and it draws nothing. Only the LAST crumb is the
    `BreadcrumbPage` (`aria-current="page"`); below `md` it is the only
    one shown.
  - **The repository's groups** (`components/repo-sidebar.tsx`) are a
    `lazy()` chunk: they draw on a repository's routes only and from its
    data, and the tree under them brings Base UI's popover, collapsible,
    and scroll area. AppShell reads `owner`, `repo`, `type`, `docId`,
    and the pages splat from the deepest match and hands them down; the
    component is keyed by repository. Its rows are
    `ShellSidebarTreeItem`s: its own group (Home, Changelog), `Pages`
    (`lib/pageTree.ts` builds the tree; `useTreeOpen` takes the current
    page and every directory above it), and `Documents`, one row per
    type with its `TypeDot`, its count as the badge, and a link to the
    type's index. A closed branch is not in the DOM, so a type's
    `TypeDocuments` mounts, and `listDocs` fires, when its row opens or
    the icon rail's popover does; while it loads the branch holds an
    `InlineLoading` in a `ShellSidebarTreeNote` (a list item, which a
    bare paragraph is not). Facets omit zero-hit types, so a missing
    `typeCounts` key after facts load means 0, which disables the caret.
    A repository docz-api 404s gets no groups.
  - The app's own rows (`NavRow` in AppShell) are the plain
    `SidebarMenuButton`, so the tree's parts stay out of the eager
    graph. A pin has no icon and leads with its first letter, which is
    what the icon rail shows.
  - **A type's colour is not the frame's** (DESIGN-0028 question 3). It
    is on the type row's dot, on a `Dot` or `Badge` docz hands a `tone`,
    and inside the document. The mark, the breadcrumbs, and the active
    row are the package's colours on every route; there is no
    `--type-accent`.
  - In a test, find the sidebar as the navigation named
    `docz navigation` and query `within` it: the route beside it has
    links and list items of its own. The repository's groups arrive
    after a lazy import (`findByRole("link", { name: "Home" })`). A
    type's caret is the button `<type>, show contents`; a directory with
    no page is the button `<name>/`; a count sits beside the link, in
    the row, not in the link's name. `src/test/setup.ts` stubs
    `Element.prototype.getAnimations`, which Base UI calls before it
    unmounts a closing panel.
- Fonts are self-hosted `@fontsource` imports in `src/main.tsx` —
  never add a third-party font URL (mockup.html may use CDN links; it
  ships nowhere). **IBM Plex Sans Variable** (`--font-sans`) sets the
  shell's chrome and **JetBrains Mono Variable** (`--font-mono`) every
  mono surface (DESIGN-0026 §2; the mono's true italic is what Shiki's
  italic scopes use); **Source Serif 4 Variable** (`--font-serif`) sets
  markdown the site renders itself. A document's fragment brings
  pkg/theme's own stacks. Every variable family needs the wght AND
  wght-italic imports.
  Prose sizing is dialed in on the specimen page, not in the abstract:
  body 19px/1.78 on a 66ch measure, headings in em so the hierarchy
  scales with the body, chrome one step above the mockup's original
  10–13.5px scale.
- Bordered mono chips MUST use `.mono-chip-y` (tokens.css) for their
  vertical padding and MUST NOT carry a `py-*` utility beside it —
  Tailwind's utilities layer would win. An inline box is as tall as the
  font's ascent+descent, and a mono's capitals sit high in it (the
  values were measured on Monaspace Neon, the mono before DESIGN-0026),
  so symmetric padding renders every chip visibly high. The class holds the compensating
  difference; `.doc-prose code` carries its own copy with descender
  clearance. Verify by measuring, not by eye.
- `src/theme/tokens.css` — the single global stylesheet (DESIGN-0028
  §3). Don't add other CSS entrypoints. Its head is
  `@import "@donaldgifford/design-system/core.css" source(none)`, then
  `@source` lines for `src/`, `index.html`, and `review.html`.
  `core.css`, never `globals.css` (which scans the whole package and
  ships five static font files), and `source(none)` is REQUIRED: without
  it Tailwind takes the whole directory of any file it sees under
  `node_modules`. Each package part docz imports gets one
  `@import "@donaldgifford/design-system/parts/…"` line; docz writes no
  `@source` into `node_modules`. The fonts stay the three
  `@fontsource-variable` imports in `main.tsx`; the package's
  `fonts.css` is not imported, and docz's `--font-sans`/`--font-mono`
  (the variable builds) override the package's theme entries.
  Two vocabularies live here until IMPL-0033 Phase 9. The package's
  (`bg-card`, `text-muted-foreground`, `border-border`) come from
  `core.css`'s `@theme inline` map. docz's (`bg-bg-raised`,
  `text-fg-tertiary`, `text-t-rfc`) are `pkg/theme`'s public token names,
  registered in `@theme static`: `static` so every token is emitted (the
  colour module resolves them at runtime through `var()`), and NOT
  `inline`, so a utility compiles to `var(--color-bg-raised)` and
  resolves where it is used, against the shell in chrome and against the
  document's theme inside `.docz-page`. Two of docz's names are NOT registered: `code-bg`
  (the package maps `bg-code-bg` to its own `--code-bg`, which follows
  the page, and a docz entry would take that utility and paint it dark
  in both modes) and `st-superseded` (a superseded status is the
  package's `ignored` badge). Both stay in `docz-tokens.css` for
  `charcoal`, and `shell.test.ts` holds both halves.
  **`accent` is the package's name.** `bg-accent`, `text-accent`, and
  `border-accent` compile to its hover surface wherever they are written
  (its map is inline), so docz's brand colour is the `brand` utility:
  `bg-brand`, `text-brand`, `brand-dim`, `brand-bg`, `brand-border`,
  `on-brand` (`accent-utility.test.ts` fails on the old three). `brand`
  is registered `@theme inline` against `--color-accent`, which keeps its
  name because it is `pkg/theme`'s public token; inline is what lets a
  type's routes and a `.docz-page` re-point it. Never add
  `--color-accent` to a docz `@theme` block: it would take the package's
  utility and paint its hover surface in docz's blue.
  There is no radius of docz's own. The scale is the package's: `sm`,
  `md`, and `lg` are its one 3px corner (docz's bordered controls use
  `rounded-sm`), and `rounded-pill` is its fully rounded shape. The old
  wipe (`--radius-*: initial`) is gone because it removed `rounded-md`
  and `rounded-lg`, which the package's components use.
- **The shell and the document theme** (DESIGN-0026, DESIGN-0028). The
  site's chrome reads the design system's tokens. docz's 58 `--color-*`
  names sit in two files: `src/theme/shell.css` aliases the 32 whose
  value is the package's (`bg-base` at `--background`, `accent` at
  `--primary`, `hash-0..7` at `--category-1..8`; DESIGN-0028 §4 has the
  table), and `src/theme/docz-tokens.css` holds docz's own 26 (two
  surface and border steps, three text steps, `accent-dim`,
  `st-superseded`, the alert backgrounds, the type colours, `code-bg`),
  light at `:root` and dark under `.dark`. `shell.css` and its aliases go
  in IMPL-0033 Phase 9; `docz-tokens.css` stays, and
  `scripts/gen-charcoal.ts` reads its dark block. The mode is on `<html>`
  twice: `data-mode` for `pkg/theme`'s document themes and the `dark`
  class for the package, both written by `appearance.ts`'s one
  `writeMode()`; with nothing stored `data-mode` stays unset and the
  class follows the system through a media-query listener, since the
  package has no `prefers-color-scheme` block.
  `<html>` carries no `data-theme`. `shell.test.ts` holds the split (32
  and 26, 58 in all), `tokens.css`'s `@theme` block to both files, and
  every shared name to the hex the shell carried before the package.
  The type, status, and hash colours keep the filter chips' hues, fitted
  per mode to 4.5:1. `contrast.test.ts` checks both modes through
  `src/test/shell-look.ts`, which follows every `var()` into the
  package's `tokens.css` and converts oklch through `src/theme/oklch.ts`;
  it throws on a token that resolves to nothing, so a lost mapping fails
  and is never skipped. **A token read from computed style is now
  `oklch(...)`**, not hex: anything that hands one to a library (mermaid's
  theme does) converts it with `oklchToHex` first, and a spec that
  compares a token to a painted colour paints it onto a canvas and reads
  the pixel. Code the site renders follows the page (see the markdown
  pipeline); code inside a document fragment is `pkg/theme`'s and stays
  dark.
  There is no theme selector and no `docz:theme`: the top bar's
  `ModeToggle` (the package's `shell/mode-toggle`) is the light/dark
  toggle alone (`useAppearance(refreshMermaidTheme)`, the mode and
  nothing else; the review page wears the same toggle). A document's theme is on
  its own `.docz-page` (`DocSurface`, `DocPane`), never on `<html>`.
  `src/lib/theme.ts`'s `defaultTheme()` is the theme documents render in:
  runtime `__DOCZ_CONFIG__.theme` (from `DOCZ_THEME`) → build-time
  `VITE_THEME` → `paper`, each validated against the generated `THEMES`.
  `server/serve.ts`'s `resolveTheme` checks `DOCZ_THEME` at startup and an
  unknown value is a config error that fails `/readyz`. `refreshMermaidTheme`'s
  signal lives in `src/lib/diagram-theme.ts` so the eager bundle never
  imports `mermaid-block`. The file is NOT named for mermaid: the
  bundler can emit it as a preloaded chunk named for the module, and
  `e2e/rendering.spec.ts` fails a diagram-free page on any request
  whose URL says "mermaid". No component may carry a literal colour.
  A type's colour reaches the chrome only where docz hands it over as a
  `tone` (see the frame); inside a document `accentProperties` sets the
  `.docz-page`'s `--color-accent` alone, so it resolves against the
  document theme's own `--color-t-*`. The site's e2e specs loop over
  `SHELL_MODES`; the review spec crosses them with every theme
  (`e2e/appearances.ts`).
- **The document pane** (`src/components/doc-pane.tsx`, DESIGN-0026 §7).
  The reader's html view is `useGetDocHtml`'s fragment, injected with
  `dangerouslySetInnerHTML` — the one place outside MermaidBlock that
  sets HTML, and only docz-api's renderer's output, which escapes raw
  HTML and drops unsafe URLs. Its gate is `doc-pane.xss.test.tsx`, which
  mounts every payload in `mocks/content/xss-payloads.json` as docz-api
  really renders it (`mocks/__generated__/hostile-fragments.json`, from
  `TestUIHostileFragments`; `fragments.json` is the demo documents', from
  `TestUIFragmentFixtures`, both `go test ./internal/httpapi -update`).
  Three React 19 traps: the `{__html}` object must be memoised (React
  compares it by identity and rewrites innerHTML on a new one, wiping
  what the runtime and mermaid did); `setSearchParams` changes identity
  on every navigation, so the pane reads it through `useEffectEvent` and
  remounts only on `[html, accent]`; and the runtime announces the tab
  it opens while mounting, which the pane ignores so a plain URL stays
  plain. The runtime is `pkg/theme/runtime.js`'s `docz.mount` with
  `hash: false` (the tab lives in `?tab=`); each `pre.mermaid` becomes a
  `MermaidBlock` root; a click on a `/`-path with no `target` routes
  in-app. A fragment that fails renders the site's own markdown in a
  `DocSurface` with `doc-fallback-note`; tests that exercise that render
  call `failDocFragments()` (`src/test/server.ts`). Pages, the repo home,
  the changelog, and a type's index render in a `DocSurface` too.
  Tailwind's preflight strips link underlines, so pkg/theme underlines
  links itself (axe link-in-text-block).
- **The design system's parts** (DESIGN-0028 §7, IMPL-0033 Phase 4 on).
  Import a part by its subpath (`…/components/button`,
  `…/blocks/empty-state`, `…/shell/…`), never a barrel, and add its
  `parts/<kind>/<name>.css` line to `tokens.css`, in order:
  `src/theme/parts.test.ts` fails on a part with no line (it would render
  unstyled, and jsdom cannot see that) and on a line nothing imports.
  The package ships TypeScript source, so `tsc -b` and eslint check it
  with docz's settings. What docz keeps is the meaning, in small
  wrappers:
  `components/badges.tsx` (`StatusPill` is `Badge` in the variant
  `statusVariant()` gives a status, by DESIGN-0028 §8's table;
  `StatusBadge` a `Dot` and a label; `TypeDot` and `TypeBadge` take
  `typeColor()` as a `tone`; a status has no colour of docz's own in the
  chrome, and `Superseded` is the package's `ignored`) and
  `components/query-states.tsx` (`EmptyPanel`, `NotFoundPanel`,
  `ErrorPanel`, `ProseSkeleton`, and the re-exported `Skeleton` and
  `InlineLoading`; a route takes every state from that module).
  `ErrorPanel` takes the error itself: the package's `ErrorState` prints
  `String(error)` for a rejection that is not an `Error`, so the wrapper
  swaps in a `fallback` (default "Request failed"). A skeleton is
  `aria-hidden` with a `data-testid`. Icons are Tabler components
  (`icon={IconSearch}`, never an element), from `@tabler/icons-react`,
  a direct dependency pinned to the version the package pins.
  A heading through `render` (`<CardTitle render={<h1 />}>`) trips
  `jsx-a11y/heading-has-content`, since the children arrive through the
  part; disable that one line with the reason. A route outside
  `RepoFrame` keeps its own `max-w-[940px]` column inside the frame, so
  `PageHeader` takes `className="px-0"` there.
  The directory's filters are `FilterChip`s inside one `FilterBar`
  (`components/directory-controls.tsx`): a type is a `pressed` chip, the
  repository a `repo = …` chip opening `OptionItem` rows in a `Popover`,
  and "Clear" and the count are the bar's own. A part that brings Base
  UI's menu, popover, or dialog costs 15 to 30 KB gz: keep it behind a
  route or a `lazy()` boundary, and run `just ui bundle-budget` after
  adding one to anything AppShell imports. Three chunks are lazy for
  that reason: the palette, the account menu, and the repository's
  sidebar groups. `ListRow` puts `trailing`
  above its stretched link, so a badge or a time there is not part of
  the link.
- `tsconfig.json` is solution-style → `tsconfig.app.json` (browser code,
  strict + `noUncheckedIndexedAccess`) + `tsconfig.node.json` (config
  files). New config files at the repo root go in `tsconfig.node.json`'s
  `include`.
- `src/api/fetcher.ts` — the orval fetch mutator and the typed errors
  (`BadRequestError` 400 — spec 1.5.0, currently only an unrecognized
  `sort`, which the generated unions make unreachable from our own code
  so in practice it means a hand-edited URL — `SessionRequiredError`
  401, `NotFoundError` 404, `SessionUnavailableError` 503 — transient,
  NEVER a logout — `ApiError` rest). Match on these classes in UI code;
  never `fetch` the API directly.
  Success returns orval's `{ data, status, headers }` envelope —
  narrow on `status === 200` before touching `.data`. Query defaults
  live in `src/app/query-client.ts` (no retry on 400/401/404 — all
  three are stable answers, so asking again gets the same reply).
- Tests: Vitest + Testing Library in jsdom (`vitest.config.ts`).
  `src/test/setup.ts` starts one MSW node server from the generated
  handlers with `onUnhandledRequest: "error"` — override per-test with
  `server.use(...)` from `src/test/server.ts`. Mount routes with
  `createMemoryRouter(routes)` (`routes` is exported from
  `src/app/router.tsx`); first paint is async (lazy routes), so use
  `findBy*`, not `getBy*`, for the initial assertion.
- `src/api/__generated__/` — orval output; never hand-edit, never commit
- Generated OpenAPI types ARE the data model; don't hand-roll DTO types
- BUT the real docz-api marshals empty Go slices as JSON null while
  the spec (and thus the generated types) say array — crashed the
  reader in live testing. Normalize every wire array with `arr()` from
  `src/lib/wire.ts` before iterating; several fixture types carry
  `aliases: null` deliberately so the suites exercise the real shape.
  Upstream ask: marshal `[]` or mark the fields nullable in the spec.
- Markdown rendering lives in `src/markdown/` and ONLY there:
  `preprocess.ts` (strip frontmatter + docz toc block) →
  `processor.ts` `renderMarkdown()` (remark-parse → remark-gfm →
  github-alerts (`> [!KIND]` blockquotes → div.admonition.kind, five
  kinds) → capture-code-meta (fence meta → `metastring` property) →
  remark-rehype allowDangerousHtml → rehype-raw → **rehype-sanitize
  with `schema.ts`** → double-clobber collapse → rehype-slug + ToC
  collector → Shiki core highlighter with `github-light` and
  `github-dark` at once (`themes: { light, dark }`,
  `defaultColor: false`, DESIGN-0028 §9), each run ONCE through
  `theme-contrast.ts` against its mode's `CODE_BG` (every token color
  failing 4.5:1 is nudged toward the far end of the scale; the test
  pins the pair to the package's `--code-bg` through
  `shellToken()`), slim lazy grammar set, chrome
  transformer stamping data-language/data-caption → wrap-codeblock
  (div.codeblock header chrome; skips mermaid) → wrap-table
  (div.table-wrap, overflow-x scroll; className unforgeable
  post-sanitize) → xref linkify → hast-to-JSX, where task-list
  `input`s map to `MarkdownInput` for an aria-label — axe's "label"
  rule is critical and the schema strips aria-* from inputs). Sanitize
  AFTER rehype-raw, highlight AFTER sanitize. Mermaid: `mermaid-marker` runs post-sanitize/
  pre-Shiki (strips language-mermaid so Shiki can't replace the pre,
  moves source onto `data-mermaid-source`), and MarkdownPre routes
  marked pres to `MermaidBlock` (`src/markdown/mermaid-block.tsx`),
  which lazy-imports mermaid (~700 KB, own chunk — e2e asserts it
  never loads on diagram-free docs) and holds the one sanctioned
  innerHTML for document-derived SVG: mermaid.render() output under
  `securityLevel: "strict"` AND `htmlLabels: false` — BOTH required
  (strict alone still materializes purified `<img src>` elements in
  foreignObject labels) — AND `secure: MERMAID_SECURE_KEYS`, because
  mermaid's own secure list omits `htmlLabels`, so without it a
  diagram's YAML front matter turns the labels back on at BOTH the
  global and the nested `flowchart` path (audited on 11.16.0; one
  top-level entry covers both, the sanitizer recurses). That list
  restates mermaid's defaults in full rather than appending to them —
  the array union is an implementation detail — and
  `mermaid-config.test.ts` asserts the EFFECTIVE list stays a superset
  of the installed mermaid's, so a version that protects a new key
  fails CI. The config lives in `mermaidInitConfig()`, exported so the
  test exercises what ships. Render failure keeps the source visible.
  Diagrams are MONOCHROME unless the document says otherwise: tokens.css
  pins only label font-family and fill, and mermaid scopes a diagram's
  own `classDef` rules by render id, so those outrank the stylesheet —
  that's the supported way to color nodes (see the specimen's Figure 2,
  asserted on computed stroke in `e2e/rendering.spec.ts`).
  themeVariables stay the minimal documented set with an ASCII font
  name; extras break `mermaid.render` silently. `nodeBorder` is NOT
  just a color — mermaid's `neo` look gradients node strokes whenever
  the theme sets `useGradient` (`base` does), and `Theme.calculate`
  clears it exactly when the overrides carry `nodeBorder` without
  `useGradient`. `look` is PINNED (`MERMAID_LOOK` = classic): v12
  defaults to neo, which rounds node corners well past the shell's one
  3px corner. `layout` is NOT a
  constant: `src/lib/mermaidLayout.ts` resolves runtime
  `DOCZ_MERMAID_LAYOUT` → build-time `VITE_MERMAID_LAYOUT` → `elk`,
  closed set `dagre|elk`, validated at BOTH ends like nav pins — the
  value goes straight into `mermaid.initialize`, so an unvalidated
  string would be config injection on the library that renders
  untrusted text. Unlike nav pins an invalid injected value is not
  authoritative; it falls through. Chart value `config.mermaidLayout`,
  schema-constrained to the two names because the server's fallback is
  silent. ELK is its own ~436 KB gz chunk that mermaid fetches ONLY
  when the layout is elk; e2e asserts both directions, and the
  diagram-free chunk assertion matches `elk` as well as `mermaid`
  because the ELK filename contains neither the word nor the library.
  Widgets (IMPL-0030 Phase 5): `widget-marker.ts` runs beside the
  mermaid marker (post-sanitize, pre-Shiki) and moves a `docz-flow`,
  `docz-stepper`, or `docz-scorecard` fence's YAML onto
  `data-widget-source` with its name and fence line; MarkdownPre routes
  it to `WidgetBlock`, which lazy-loads `widgets/widget-view.tsx` (the
  `yaml` parser, `validate.ts`, and the three components, one chunk that
  e2e asserts never loads on a widget-free page) and shows the YAML as
  code until it arrives. `validate.ts` is a port of
  `pkg/export/html/widgets.go`, held to it by the shared corpus under
  `pkg/export/html/testdata/widgets/` (same JSON, same error lines); a
  change to either side changes the corpus. A valid block is
  `figure.widget[data-widget]`, an invalid one `div.widget-invalid` with
  a caution callout listing each error at its document line, then the
  YAML. Components use token utilities only, so a theme or mode change
  needs no redraw, and never a `style` attribute: the XSS gate allows
  one only inside Shiki, which is why the scorecard bar is a
  `<progress>`. Every string is a React text child. The reader hook
  renders with `preserveLines`, so error lines are document lines on
  both surfaces; the review mode carries `data-line` to the widget
  root as MermaidBlock does.
  h2–h4 map to `markdown-heading.tsx`, which appends the
  hover/focus-revealed copy-link button (a labeled BUTTON, not a
  link — the underline rule for prose links stays untouched).
  No `dangerouslySetInnerHTML` in the markdown pipeline (the document
  pane's fragment is the one use, below). Never widen
  `schema.ts` without extending the XSS suite — its only non-default
  allowances are `language-*` classes + the charset-validated
  `metastring` on `code`, and value-RESTRICTED admonition classNames
  on div/span (forged markup gets the same inert styling at most);
  data-* on `pre` dies in sanitize, which is exactly why the
  post-Shiki chrome can trust it. Admonition tint backgrounds are
  PRECOMPUTED hex tokens (`--color-adm-*-bg`) so contrast.test.ts can
  enforce label/body pairs — don't swap them for color-mix.
- Xrefs (`src/markdown/xrefs.ts`): doc-id tokens linkify only when they
  resolve in the caller-supplied map (UPPERCASED doc_id → href; the
  `byId` half of `useRepoDocIndex`'s `{byId, byPath}`) — the map is
  the whitelist and hrefs come from API data, never document text.
  Tokens inside `a`/`code`/`pre` stay text; the reader drops the doc's
  own id. `MarkdownAnchor` turns `data-xref` anchors into router Links
  (tests need a router around rendered content). Render-cache keys
  carry an fnv1a fingerprint over BOTH sorted key sets plus the base
  path, so bodies re-render at most once when the doc index loads.
- Relative doc links (`src/markdown/relative-links.ts`, post-sanitize
  beside xrefs): author-written relative hrefs posix-resolve against
  the source's own repo path — the `base` render option: reader = doc
  `path`, repo home = `docs_dir`/index.md, changelog = the configured
  file (repo root by default) — then rewrite ONLY on an exact `byPath`
  hit (fragment reattached, `dataXref` set). Absolute/`//`/root/`#`
  hrefs and misses stay byte-identical; traversal past the repo root
  fails closed (so does bad percent-encoding). Resolution needs both
  `paths` AND `base` passed to `useRenderedSource`. The reader also
  repairs pasted filename URLs: a 404 whose `:docId` ends `.md`
  redirects (replace, hash kept) when exactly one doc's path basename
  matches — ambiguity keeps the panel. The XSS suite has a
  resolver-active section; extend it when touching the transform.
- Known false positive: typescript-eslint computes an error type for
  the `processor.run`/`toJsxRuntime` pair in processor.ts while tsc and
  the TS API are clean — narrowly eslint-disabled there with explicit
  annotations. Don't blanket-disable the rule.

- MSW fixtures: `src/mocks/fixtures.ts` is a curated demo org (real
  docz markdown — docz-site docs via `?raw` imports, docz-api docs as
  snapshots in `src/mocks/content/`) layered BEFORE the generated faker
  handlers in both `src/test/server.ts` and `src/mocks/browser.ts`.
  Fixture resolvers return `undefined` to fall through to faker for
  anything outside the demo org.
- Rendering specimen: `docs/guides/markdown-specimen.md` is the
  kitchen sink — every construct the pipeline renders on one page. It
  is a `?raw` fixture page (`/donaldgifford/docz-site/pages/guides/
  markdown-specimen.md` under `dev:msw`) AND a real published page in
  production, because `.docz.yaml` now carries the `api:` block
  (docz-site dogfoods DESIGN-0004: README, type-dir indexes,
  `docs/input.md`, and `docs/guides/*` all publish). Both axe sweeps
  render it (jsdom with mermaid mocked to the fallback; e2e with the
  real diagrams). When a pipeline feature lands, add a section to the
  specimen in the same commit; judge typography changes there first.
- Auth UX (Phase 5): `/login` (`src/routes/login.tsx`) renders provider
  buttons as REAL `<a href="/auth/login?provider=…">` anchors — the
  OAuth 302 must reach the browser, so never convert them to router
  Links, and never to the design system's `Button` with `render`: Base
  UI's Button gives a rendered anchor `role="button"`. A link that looks
  like a button is the anchor (or router `Link`) wearing
  `buttonVariants({ variant, size })`, here and everywhere else. The enabled set (`src/lib/authProviders.ts`; comma-separated,
  keys `github`/`okta`/`keycloak`, unknown dropped, empty falls back to
  GitHub) resolves at RUNTIME from `window.__DOCZ_CONFIG__.authProviders`
  — injected into index.html by the prod server (`server/serve.ts` reads
  `DOCZ_AUTH_PROVIDERS`, whitelist-validated so the inline `<script>`
  never carries raw env; chart value `config.authProviders`) — falling
  back to the build-time `VITE_AUTH_PROVIDERS`, then GitHub. So one image
  serves any provider combo per deployment; no rebuild. docz-api owns the
  actual OAuth/OIDC exchange AND the GitHub App ingest ("machine
  identity"), which is independent of the login provider. `server/` is
  Bun-only (outside the vitest `src/` graph) — its `serve.test.ts` runs
  under `bun test server/` (`just test-server`, in the CI chain); guard
  new top-level side effects with `import.meta.main`.
  Three traps that all pass locally and fail on CI. (1) `test-server`
  runs BEFORE `build`, so a clean checkout has NO `dist/` and every SPA
  path answers 500 — server tests must never assume a built dist, and a
  stale local one hides it. Assert on responses the router makes itself
  (a POST to a non-proxied path is a deterministic 405). (2) Bun's
  `Request` constructor SILENTLY rewrites an unrecognised method token
  to GET (`BREW`, `EVIL-METHOD` → `GET`), so hostile methods cannot be
  driven through `handleRequest` that way — assert `normalizeMethod`
  directly. (3) Never substring-assert over a whole prom-client
  exposition: `process_open_fds` is `/proc`-only (absent on macOS) and
  its help text "file descriptors" CONTAINS "script". Filter to
  `docz_site_` lines first.
  On 401, `SessionRequiredRedirect` (query-states.tsx) stashes
  `pathname+search` via `src/lib/authReturn.ts` and replaces to
  `/login`; `RestoreAfterLogin` (AppShell) probes getSession on "/"
  when a stash exists and restores it only on 200 (the OAuth callback
  always lands on "/"). The stash validates paths on BOTH write and
  read — keep it that way (open-redirect guard), and never stash
  anything but a same-origin path. Test setup clears session/local
  storage after each test — a leaked stash arms RestoreAfterLogin in
  unrelated tests. The session sits at the sidebar's foot:
  `SessionMenu` (`src/components/session-menu.tsx`), drawn from
  `useSessionState()`, which AppShell calls so it can leave the footer
  out altogether for an anonymous session (fixtures answer with a
  deterministic `donaldgifford` github identity). The signed-in half is
  `account-menu.tsx`: a `SidebarMenuButton` holding the identity
  (`ShellSidebarIdentity`, initials on the icon rail) that opens the
  design system's `DropdownMenu`, beside the sidebar and above it in
  the sheet. It is a lazy chunk of its own: Base UI's menu is about
  30 KB gz, which only a signed-in session needs, so the placeholder
  holds its place while it loads. A test finds the trigger with
  `findByRole` (`Account: <identity>`) and Sign out as a `menuitem`.
  The sign-in link's test id is `session-sign-in`; `/login` is outside
  the frame, so it never shows there. Logout runs `onSettled` —
  navigate to `/login` BEFORE `queryClient.clear()`, or the page being
  left refetches everything under the dead session.
- Server observability (DESIGN-0006, IMPL-0007): every signal shares
  ONE classifier, `server/route-class.ts` — a closed 8-value
  `RouteClass` union plus `normalizeMethod()`. BOTH inputs are
  attacker-controlled (SPA paths are unbounded; `fetch()` accepts
  arbitrary method tokens), so nothing is ever echoed from the request
  into a label. Never build a label any other way — the closed set is
  what keeps log volume and metric cardinality bounded by construction.
  `server/redact.ts` is an ALLOWLIST: query keys survive, every value
  becomes `<redacted>`, and `SAFE_QUERY_VALUE_KEYS` is deliberately
  EMPTY so a parameter nobody thought about is redacted by default.
  Headers are never recorded (cookie → `has_cookie` presence only);
  `Location` → host only. NEVER log a raw URL. `server/logger.ts`
  (hand-written, no dependency) does NOT redact — callers pass values
  already through redact.ts, and that boundary stays sharp: a logger
  that redacts is one someone eventually trusts with a raw URL.
  `server/redaction-gate.test.ts` is a security gate in the spirit of
  the XSS suite — parameterised over every level × format, asserting on
  SERIALISED output, with a self-check proving it fails when redaction
  is bypassed. Extend it when adding a log field. Config resolvers fail
  CLOSED (unknown level → `info`, never something noisier, because
  debug carries request paths).
  Probes short-circuit BEFORE any signal (a kubelet every 10 s would
  otherwise be most of the log volume). `/healthz`, `/readyz`, and
  `/metrics` are RESERVED server paths and must never fall through to
  the SPA — a probe path answering 200 text/html silently poisons
  whatever scraped it, which is why `/metrics` 404s until its phase
  lands. `/healthz` is UNCONDITIONAL (liveness restarts the container,
  and restarting cannot conjure a `dist/`); `/readyz` checks (readiness
  holds traffic, so a bad deploy stalls the rollout and the previous
  ReplicaSet keeps serving). `/readyz` makes NO call to docz-api —
  gating readiness on the API would evict every pod from the Service
  when it blipped, turning a degradation DESIGN-0003 already handles
  into a total outage; upstream health is a metric. `checkReady` and
  `invalidConfigVars` take `distDir`/`env` as arguments (module-level
  `DIST` is read once at import), and `invalidConfigVars` runs the REAL
  resolvers and reports whether they fell back rather than restating
  any whitelist. It is deliberately narrow — only a non-empty value
  that validates empty-handed counts, because readiness failure stalls
  a rollout and that is too blunt for a cosmetic typo.
- Metrics and tracing (DESIGN-0006 Components 5-6) label from
  `route-class.ts` and NOWHERE else. Metric labels are the one place
  where unbounded input is expensive rather than merely noisy — each
  distinct combination is a series stored forever — so never build a
  label from request text. `metrics.test.ts` drives 10 000 hostile
  paths and asserts the series count stays bounded, with a companion
  test proving that guard fails on an unbounded label.
  `DOCZ_METRICS_ENABLED` defaults ON; when off, `/metrics` returns an
  EXPLICIT 404 rather than being unregistered — an unregistered route
  falls through to the SPA handler and hands a scraper `index.html`
  with a 200, silently poisoning a dashboard.
  Tracing is HAND-INSTRUMENTED and must stay that way: OTel's HTTP
  auto-instrumentation records `url.full`, which on this proxy means
  shipping OAuth codes to a collector. NEVER install an
  `@opentelemetry/*instrumentation*` package — a test asserts none is
  present. Span attributes are an allowlist (method, route class,
  status, REDACTED url.path); `tracing-spans.test.ts` is the tracing
  counterpart to the log redaction gate. `traceparent` is injected on
  the proxy hop and docz-api already Extracts it, so end-to-end traces
  need zero upstream change — OBSERVED, not assumed: one proxied request
  produced a single trace whose docz-api server span is parented by our
  `proxy.upstream` span. The two exporters disagree on the wire —
  docz-api's Go `otlptracehttp` posts PROTOBUF to a bare `host:port`,
  our SDK posts JSON to a full URL — so point them at the same collector
  with the spellings each expects and let it absorb the difference;
  don't "fix" one to match the other. An unconfigured `OTEL_EXPORTER_OTLP_ENDPOINT`
  means no provider, no export, no network call; the resolver fails
  CLOSED on anything that is not an absolute http(s) URL, because this
  value decides where telemetry is SENT.
  `nodejs_gc_duration_seconds` is declared but NEVER samples under Bun
  (measured) — stock Node dashboards show empty GC panels.
  The server ships BUNDLED (`bun build --target=bun` in the Dockerfile,
  `just build-server`): the runtime image has no node_modules, so a
  dependency is only possible bundled. It also retired the enumerated
  `COPY server/serve.ts`, which broke the container the moment serve.ts
  gained a sibling module — CI never builds and runs the image, so that
  shipped green. Build and run the image by hand when touching
  `server/` or the Dockerfile.
  Env read once at import (`DOCZ_METRICS_ENABLED`, the OTEL_* vars)
  cannot be re-tested by setting `process.env` and re-importing —
  `bun test` shares ONE module registry across the whole run. Those
  paths use a child process (`metrics-disabled.test.ts`,
  `tracing-spans.test.ts`), and the child must MARK its stdout payload
  because the server's own structured logs share that stream.
- The route error boundary (`src/app/route-error.tsx`) sits on a
  PATHLESS layout route directly below `AppShell`, NOT on the root
  route. A boundary replaces the element of the route that owns it, so
  a root-level one swaps out AppShell and takes the frame with it —
  stranding the user on a panel with no navigation, which is the exact
  failure it exists to fix. `route-error.test.tsx` pins both, including
  that the root placement loses the frame, so the extra nesting level
  can't be refactored away as redundant. The pages outside the frame
  have the same boundary on their own pathless route. It CATCHES AND DISPLAYS only:
  no `window.onerror`, no `unhandledrejection`, no beacon (a test
  verifies their absence). A link home is the ONLY affordance — a retry
  would re-render the same crashed route and throw again. Panel chrome
  is `ErrorPanel` in query-states.tsx, with the link home as its
  `children` (the package's `RouteErrorLayout` is not used: its one
  action is Retry); the error state
  is in the axe sweep.
- Session classification (DESIGN-0003): auth chrome renders from
  `classifySession` (`src/lib/session.ts`) — pending / signed-in /
  anonymous (`provider === "none"`, docz-api's AUTH_PROVIDERS=none) /
  signed-out / unavailable. "Sign in" is reachable ONLY from
  signed-out, which only a real 401 produces; 503 and every other
  failure are `unavailable` → SessionMenu keeps an inert placeholder
  (`session-unavailable` testid, visually identical to pending) and
  the session query re-polls ~30 s via an error-gated
  `refetchInterval` in `useSessionState()` (the shell never unmounts
  and focus-refetch is off — a healthy query must never poll). `anonymous` renders NO auth chrome
  anywhere and `/login` swaps to the auth-disabled panel (buttons
  render immediately; swap only on confirmed anonymous). None-mode is
  detected from the session response alone — no config, no storage.
- Nav pins (DESIGN-0002): `DOCZ_NAV_LINKS` (JSON `[{label, href}]`)
  resolves in `server/serve.ts` — label `/^[\w .&+-]{1,24}$/`, hrefs
  same-origin app paths in printable ASCII with NO HTML-significant
  chars (stricter than authReturn: a href must never terminate the
  inline `<script>`, which also escapes `</`), cap 6, invalid entries
  dropped — into `__DOCZ_CONFIG__.nav`. `src/lib/navLinks.ts`
  re-validates (both-ends rule); an injected array is authoritative
  even when it validates empty, then `VITE_NAV_LINKS`, then no pins.
  AppShell renders pins as the sidebar's `Pinned` group, under
  `Browse`; with none the group is absent. `build:msw` bakes an RFCs pin so e2e and
  dev-msw exercise the surface; the deployable build stays pin-free.
  Chart value `config.navLinks` (toJson; env omitted when empty).
  `docs` is a curated type: `--color-t-docs` (contrast-checked) +
  CURATED_TYPES + blurb — type colors are never runtime-configurable.
- Reader lives in `src/routes/doc.tsx` + `src/components/doc-rail.tsx`
  + `src/components/query-states.tsx` (shared 401/404/error panels).
  Since IMPL-0002 Phase 5 the right rail is ToC-ONLY: metadata sits
  under the doc header (`doc-meta-table.tsx`: the design system's
  `DetailsGrid` in its `list` layout, fields ""-omitting, a
  `SegmentedControl` for html/md with the json link beside it) and the
  lifecycle is a closed-by-default `Disclosure` holding a vertical
  `Stepper`, owned by `LifecycleRail` (renders nothing — box included —
  for unknown types; closed, it shows the status in force, and its
  steps are not in the DOM until it opens, so a test clicks it first).
  The header meta row ends in a "View in Confluence" link
  (new tab, `noopener noreferrer`) when spec 1.6.0's `confluence_url` is
  non-empty, and in nothing otherwise. Gated mockup rows (relationships,
  tags) slot into the details when the DESIGN-0001 API asks land.
  `TocList` is the package's `Outline` with its `use-active-heading`
  scroll spy (IntersectionObserver over the heading ids; docz passes
  the band, whose top margin clears the sticky top bar) and marks the
  current row `aria-current="location"` — it is undefined where the
  observer is missing (jsdom) and HOLDS the last heading when a long
  section fills the band, so the rail never flickers to nothing. The
  Outline draws its own "On this page" heading, so a route hands the
  rail `<TocList toc={…} />` and nothing else, and it draws nothing for
  a page with no headings. Only a page docz renders itself has an
  outline: a fragment in the document pane has its own tabs and takes
  no rail. A trigger's accessible name joins its label and value with
  no space in jsdom (`/^Lifecycle\s*Approved$/`).
- Type index (`src/routes/repo-type.tsx`): the docz-create hint is a
  document, in a `DocSurface`; the type's documents are the package's
  `DataTable` UNDER it, outside the surface, whose prose rules would
  restyle a table inside. `sortable={false}`: the rows come in
  docz-api's order and the table offers no other. Build columns with
  `dataTableColumns<T>()` and wrap them in its `.columns([...])`, or
  the accessors' value types do not fit `DataTableColumn<T>`. A cell
  is `whitespace-nowrap` unless its column's `meta.className` says
  otherwise.
- Directory (`src/routes/directory.tsx`): hit rows are CARDS since the
  DESIGN-0005 dial-in — `DOC-ID / repo` over the title, `StatusPill` in
  the middle, the updated stamp (date over time) on the right — NOT the
  mockup's six-column `.doc-row`, which this surface has now diverged
  from (mockup.html still leads for prose and chrome tokens). Both
  outer columns are two lines so they align with each other. There is
  no type badge anywhere: the doc id already spells the type out and a
  second colored chip fought the status for attention. Type color
  survives on the filter chips only. The URL is the only source of
  filter truth — read via `parseSearchParams`, write via
  `serializeSearchState` (`src/lib/searchParams.ts`; its
  `toSearchDocsParams` maps state → API params, first-of-array facets).
  Typed queries debounce ~200 ms and commit with `replace: true`;
  discrete filter actions must push so back/forward walks history.
  ORDERING (IMPL-0006 OQ-3): `sort=updated_at:desc` goes out ONLY when
  `q` is empty — `sort` is a TOTAL order over matches, not a tie-break
  within relevance, so sorting a text search ranks recent-but-irrelevant
  hits above the best match. It is DERIVED, never stored: no control
  selects it, so it has no URL key, and `toSearchDocsParams` takes an
  explicit `{ ordered: true }` passed only by the row-rendering query
  (facet queries run at limit 0 — ordering them is work for a count
  nobody reads). Same reasoning for `source`: no control, no URL key;
  it is sent as a fixed `doc` by `useRepoFacts`, whose total is now
  `estimated_total_hits` rather than a source-facet lookup.
- The updated column: since spec 1.5.0 `SearchHit.updated_at` is a
  typed REQUIRED property on BOTH record kinds — read `hit.updated_at`
  directly, there is no accessor. `created` (frontmatter `YYYY-MM-DD`)
  is the field that is "" on page hits; a published page has no
  authored date. `updated_at` is INGEST-observed, not commit time: one
  reconcile is one transaction, so every doc an ingest touches shares
  the stamp to the microsecond and a fresh DB restamps a repo at
  onboard. A stamp is drawn by the design
  system's `Time` (`style="stacked"` on a directory row), which reads
  "—" for one that is missing or unparseable, so a deployment on a
  pre-1.5.0 docz-api still renders. How a stamp reads is the package's
  to test; a docz test asserts the `<time datetime>` instead.
  Fixtures: docs forward their own stamp and
  `created`; pages take `PAGE_INGESTED_AT[repo]`, one value per repo
  because that is genuinely how onboard stamps behave.
- Faceted controls exclude their own dimension via separate limit-0
  searchDocs queries (directory picker/chips AND palette pills) so
  every option stays offered while one is selected. URL `offset` means
  "rows 0..offset+PAGE_SIZE shown"; the query grows `limit` from 0 so
  deep links render identical rows.
- Repo pages share `RepoFrame` (`src/components/repo-frame.tsx`): the
  content and, from 1181px, a 190px rail to its right, with the route's
  breadcrumbs portalled into the top bar — home, changelog, pages, type
  pages, AND the reader all mount inside it. It takes `crumbs` and
  `rail` and nothing about the repository: the repository's navigation
  is the shell's sidebar (see the frame). Its `max-w-[1062px]` is the
  content's 784px measure plus the rail and the gap. Counts everywhere
  come from `useRepoFacts` (repo-filtered limit-0 facet query) so
  numbers agree. The repo home is the ONLY surface rendering an
  h1 inside `.doc-prose` (the reader strips body h1s) — its style
  lives in tokens.css; don't remove it as "unused".
  URL `{type}` resolves by name/id_prefix/alias via
  `lib/docTypes.resolveDocType` (links always generated from the
  canonical name); fixtures mirror this and 404 unknown types.
- Repo home renders getRepoIndex's index.md (spec 1.1.0) through
  `useRenderedSource` with its h1 KEPT; the reader strips the h1 via
  the `useRenderedMarkdown` wrapper. 404 from getRepoIndex = generated
  home fallback, not an error.
- Repo changelog (spec 1.2.0, IMPL-0003 Phase 1):
  `/:owner/:repo/changelog` is a RESERVED static segment — it outranks
  `:type`, so a doc type literally named "changelog" is reachable only
  via its id_prefix/alias URL. The sidebar's row under Home gates on
  `changelogConfig()` (`src/lib/changelogConfig.ts`) reading getRepo's
  UNTYPED `config_snapshot` defensively — any wrong shape means "no
  row", zero extra requests — and hover/focus-prefetches
  getRepoChangelog. The page copies repo home: h1 KEPT, TocList rail,
  render memoized per (repo, changelog_sha). A changelog 404 is NEVER
  an error (quiet panel says config-disabled vs file-absent-at-HEAD;
  wait for repo detail before choosing the copy); `changelog_md: ""`
  gets an explicit empty state. The demo fixture is this repo's real
  CHANGELOG.md via `?raw` — which is why cliff.toml's
  commit_preprocessors backtick tag-shaped tokens in subjects: a raw
  `<em>` in a commit subject once reached the rendered page and
  rehype-raw mangled the list structure (axe caught it).
- Published pages (DESIGN-0004, spec 1.4.1): repos with an enabled
  docz 1.2.0 `api:` block publish non-docz markdown. `/pages/*` is the
  SECOND reserved segment (registered above `:type` beside changelog);
  the empty splat redirects to the repo home (the landing page IS the
  home — `repo-home.tsx` anchors its link base at
  `apiConfig(snapshot)?.landingPage`). Everything gates on
  `apiConfig(config_snapshot)` (`src/lib/apiConfig.ts`, defensive like
  changelogConfig — wrong shape means the whole surface stays dark
  with ZERO pages requests; tests pin that for nav, reader, and
  index). The wire omits source repo paths; `src/lib/pagePaths.ts`
  reconstructs them (additional_docs member → itself; `.md` →
  docs_dir join; extensionless directory → BOTH README.md and
  index.md keys) — those keys join `useRepoDocIndex`'s `byPath` (docs
  win collisions) so docs and pages cross-link both directions, and
  the page reader (`src/routes/page.tsx`) drops its own keys and uses
  the first as its relative-link base. ORVAL GOTCHA: generated URL
  builders interpolate path params RAW — always pass
  `encodeURIComponent(path)` (one segment, the spec-blessed spelling;
  prefetch in `usePrefetchPage` must match or cache keys diverge).
  Discovery: the sidebar's Pages group (`repo-sidebar.tsx`, mounted
  only on an apiConfig hit), palette + directory render `source: "page"`
  hits keyed/linked by published path with a neutral mono marker
  (doc-only columns "—"; the directory count line appends
  "· X docs · Y pages" ONLY when pages matched, so non-opted
  deployments stay byte-identical). searchDocs gained the `source`
  filter in spec 1.5.0 (docz-api#27, closed) — `useRepoFacts` sends a
  fixed `doc` and reads `estimated_total_hits`, so the total is a
  doc count by construction rather than a facet lookup.
  Recents (`recentDocs.ts`) are kind-discriminated (`doc` | `page`);
  page entries store the published path, validated per segment with
  dot-only segments rejected, and a stored payload without `kind`
  resets the store. Fixture pages live in `DEMO_PAGES` (docz-site
  only; docz-api stays non-opted for the gate tests).
- Palette (`src/components/command-palette.tsx`): the design system's
  `CommandDialog` with `CommandFilters`, `CommandFooter`, and `preview`
  (DESIGN-0028 §7). It is a lazy chunk. AppShell mounts
  `command-palette-lazy.tsx`, which holds the ⌘K and `/` shortcuts and
  fetches the chunk shortly after the shell mounts and on the top
  bar's `SearchTrigger`'s hover, so cmdk and the dialog are never in the eager graph.
  A test waits for the dialog AND for its input to have focus: a key
  pressed before the dialog hands focus over goes to the page behind.
  State is palette-local, never the URL. cmdk normalizes item values —
  keys are lowercased and navigation resolves through a unified
  `entries` list (recents get a `recent:` value prefix so the same doc
  in the results below keeps its own key). The empty query leads with
  recently-opened docs from `src/lib/recentDocs.ts` (localStorage
  `docz:recent-docs`, cap 8, coordinates+title ONLY — never tokens;
  reads are segment-validated and malformed payloads reset the store;
  the reader records entries on successful load). The highlighted hit
  prefetches getDoc. A hit is a `ListRow` inside cmdk's item with NO
  link of its own: the item is a `role="option"`, and a link inside one
  fails axe's `nested-interactive`, so a click and Enter both go
  through `onSelect`. Snippets render ONLY through the package's
  `blocks/marked` (splits on literal <em> markers, emits <mark>,
  everything else stays text), in the palette and on the directory —
  never parse snippet HTML. `src/components/snippet.xss.test.tsx` is
  docz's gate on that part, over `mocks/content/xss-payloads.json`.
  jsdom setup stubs scrollIntoView/ResizeObserver for cmdk.

## docz-review (`src/review/`, DESIGN-0024, IMPL-0027)

The blocking review page for agent-written documents. Its own entry,
`review.html` → `src/review/main.tsx`, built by `vite build --mode
review` (`bun run build:review`) into `dist-review/` with a manifest;
the site's `dist/` and its bundle budget never see it. Recipes:
`just ui dev-review` (Vite against the fake server, prints the URL with
its token), `just ui test-review` (vitest over `src/review` and
`src/cli`), `just ui e2e-review` (the chromium spec alone; `just ui e2e`
runs it too), `just ui gen-cli` / `gen-cli-check`, and
`just ui gen-review-fixture`.

- **The CLI contract.** `scripts/gen-cli.ts` turns `../api/cli/*.schema.json`
  into types plus an OPEN copy of each schema (`additionalProperties`
  stripped) under the gitignored `src/cli/__generated__/`.
  `src/cli/validate.ts` validates docz's output with ajv against the open
  copies (a consumer ignores fields it does not know; the closed schema is
  the producer's drift detector) and refuses another `schema_version` or a
  docz older than `SUPPORTED.minDocz`, naming both versions.
- **The review render mode.** `renderMarkdown(raw, { review: true,
  regions })` takes the WHOLE document: `preprocess.ts`'s `preserveLines`
  blanks the frontmatter and ToC instead of cutting them, so the parser's
  positions are document lines, and `rehype-data-line.ts` stamps
  `data-line` on block elements AFTER sanitize, so an author's own
  `data-line` is stripped first and can never become a coordinate. Shiki
  rebuilds every `<pre>`, so `preLineCarrier` records the stamps before it
  and writes them back after; `MermaidBlock` carries its stamp to the root.
  `regions.tsx` then wraps blocks in `<section data-region data-ordinal>`
  from `docz inspect`'s regions (deepest wins; a crossed or malformed
  region stays unwrapped).
- **Anchors** (`anchor.ts`) are quotes of rendered text plus 32 code
  points of context, the region, its ordinal, the heading, and the line.
  The resolver parses the current source with the same remark pipeline and
  searches each block's `mdast-util-to-string` text (raw HTML blocks by
  their tag-stripped text), using `carryMarkers`' rules: one hit is exact,
  several are settled by context and heading or are ambiguous, none is
  lost, and the line hint never breaks a tie.
- **Painting without DOM.** Annotations are painted through the CSS
  Custom Highlight API (`::highlight(docz-review)` in tokens.css) and a
  changed region is marked by a `data-changed` attribute the stylesheet
  badges; nothing is inserted into the sanitized markdown. Lightning CSS
  warns that it does not know `::highlight` while minifying; the rule
  survives, so the warning is noise.
- **Controls** (DESIGN-0028 §10, IMPL-0033 Phase 8). The page's
  controls are the design system's parts, and its review logic is not
  theirs. `controls.tsx` holds the wrappers: `ReviewButton` (the
  package's `Button`, outlined unless `primary`; `selected` is the
  chosen one of a set, and the `role="radio"`, `aria-checked`, or
  `aria-pressed` stays the caller's; a disabled one keeps its pointer
  so its `title` still says why), `StateBadge` (a document's state),
  `ConfirmDialog` (the package's `Dialog`; there is no
  `window.confirm`), and the field and label classes. The documents
  are the package's `Tabs` in the `line` form, rendered as the page's
  `<main>`, with the open document in the one `TabsContent`; the status
  is a `SegmentedControl` named `Status` whose segments wrap, because a
  type can have more statuses than the sidebar's 340px holds, and a
  document with no status gets no segment for it. Text boxes stay
  native `textarea` and `input` elements in the package's colours.
  The page's chrome is written in the package's names only
  (`bg-card`, `text-muted-foreground`, `border`); docz's names are for
  the document column. A selected control is `bg-muted`, never
  `bg-accent`, which `accent-utility.test.ts` forbids under `src`. The
  round diff (`diff.tsx`) keeps its rendering and sits on
  `bg-background` with `foreground` on a 12% `success` or
  `destructive` tint and `muted-foreground` for unchanged lines, each
  pair held to 4.5:1 in both modes by `contrast.test.ts` (v2.6.1's
  light-mode fix). The page imports nothing from `shell/sidebar`: no
  JavaScript file in `dist-review` mentions it, though the shared
  stylesheet carries the sidebar's rules. In a test the documents are
  the tablist `Documents`, the status the tablist `Status` (the
  current one reads `Draft (current)`), and a dialog is found by its
  question; an e2e that runs axe over an open dialog waits for its
  animations first, or the half-faded dialog reads as low contrast. A
  heading locator for the document's title needs `exact: true`: the
  document's own h1 contains the title too.
- **Appearance** (IMPL-0030 Phase 2, DESIGN-0026 §9). The page wears the
  site's shell: its own single-column frame with a header holding the
  docz mark, the package's `ModeToggle`, and Done. The document column is a `DocSurface` in the repository's theme:
  `docz inspect` reports `.docz.yaml`'s `html.theme` as `theme`, the
  server passes it in the session (`sessionTheme`, the first document's,
  `paper` for a docz too old to say), and `main.tsx` fetches the session
  BEFORE the first render and seeds the query cache, so the column never
  paints in `paper` first. `<html>` carries no `data-theme`. There is no
  theme selector and no `docz:theme` key.
  The mode is the reader's: `applyStoredMode()` sets `data-mode` and the
  `dark` class from `docz:mode` (the key exported pages use too) before
  the first render; with nothing stored `data-mode` stays unset, the
  theme's `prefers-color-scheme` block applies, and the class follows
  the system. `useAppearance(onChange)`
  dispatches `docz:mode` and calls `refreshMermaidTheme` after a change
  (never on mount), which re-initializes mermaid from the new tokens and
  redraws every mounted diagram. The toggle always shows: the shell has
  both modes, and a document theme without them keeps its one look. The page
  drops its token from the URL, so a reload shows the "open the link"
  message: an e2e that reloads must `goto` the link again.
  `e2e/review.spec.ts` runs every spec and full-rule axe under every
  theme in both shell modes, picking the theme through
  `POST /__fake/reset?theme=<name>`, which gives the fixture repository
  that `html.theme`. `fake-server.ts` imports `sessionTheme` by a
  RELATIVE path: `vite.config.ts` loads it through
  `scripts/review-fake.ts`, where `@/` does not resolve, so only type
  imports there may use the alias.
- **The draft** (`draft.ts`) decides the outcome from the actionable set
  (an answer, comment, replace, delete, document comment, status change,
  or finding to fix); Approve is enabled only while it is empty, and
  Submit only while the draft holds something (`isEmptyDraft`), so an
  approval always comes from the Approve button. Both are disabled while
  a comment box holds text not yet added, and the selection popover is
  held open once it has text, so a click that clears the selection
  cannot throw a comment away (v2.2.1's trial lost one that way).
  `feedback.ts` writes the round's message, golden-tested under
  `src/review/testdata/feedback/` (`bunx vitest -u` regenerates).
- **The server seam.** `api.ts` takes the token from `?t=` with
  `history.replaceState` and sends it as a bearer header; there is no
  cookie, and the event stream is read through fetch because EventSource
  carries no header. `fake-server.ts` answers the same endpoints for
  tests, the axe sweep, and `dev-review` (through `scripts/review-fake.ts`,
  a Vite plugin in `--mode review` with unauthenticated `/__fake/records`,
  `/__fake/feedback`, and `/__fake/reset` hooks for the e2e spec).
- **The fixture.** `e2e/fixtures/review-repo/` is a real docz repository
  and `e2e/fixtures/review-session.json` its captured `inspect` and
  `validate` output, so nothing on this side needs a docz binary.
  `cmd/review_fixture_test.go` (Go, in `just test`) fails when the capture
  drifts from what docz prints; `just build && just ui
  gen-review-fixture` is the fix. Both are in `.prettierignore`: the
  capture is docz's formatting, not prettier's.
- **The local server** lives in `review/` (Bun, outside the vitest
  graph; `bun test ./review/`, the `./` matters because `review/` also
  matches `src/review`). `state.ts` is the only writer of
  `<state>/<sha256(root)>/<session>/` (0700/0600, `O_NOFOLLOW` reads,
  atomic writes, `flock` through `bun:ffi` around every read-modify-write,
  synchronous sections only); `lease.ts` keeps one server per session;
  `docz.ts` runs docz with `execFile`, `cwd` the root, and an argument
  list; `round.ts` checks a submission in the design's order and writes
  the record before the state moves; `server.ts` is the 421/401/403/415/
  413 wall and every response's headers; `main.ts` is the CLI, with the
  hook's `{"decision":"block"}` printed once and last. Every run ending
  but `settled` dismisses what is still open.
- **The binary.** `just ui build-review` builds the page, generates
  `review/__generated__/assets.ts` from the Vite manifest
  (`scripts/embed-manifest.ts`, a `with { type: "file" }` import per
  asset), compiles `review/compiled.ts` for four targets into
  `dist-review-bin/`, and runs `review/smoke.ts` over the host binary
  from an empty directory. Both outputs are gitignored, and
  `compiled.ts` is outside tsc and eslint because its import exists only
  after a build.
- **The plugin** (`../plugins/docz-review/`) runs `review/main.ts` from
  source in its hook tests (`bun test plugins/docz-review/tests/` from the
  repository root), through a `docz-review` shim on `PATH` and the fake
  docz in `review/testdata/`. A change to arming or to the `--pending
  --hook` path runs that job too, and the hook fixtures under
  `review/testdata/hook/` are shared by both suites.

## Toolchain notes

- Never commit credential-shaped strings — even fake ones in fixture
  prose or docs (including THIS file). trufflehog scans the full PR
  commit range with unverified findings fatal; any database URI
  carrying a user-colon-password pair trips it regardless of the
  password's value (a REDACTED placeholder still matches — drop the
  password component entirely), and a purge means rewriting branch
  history.
- The site deploys with `charts/docz` (IMPL-0021), under `site.*`;
  `charts/docz-site` was deleted at v2.0.0 (#140). Bump `charts/docz/Chart.yaml`'s
  `version` once per release, before the tag that should publish it — not
  in every PR that touches the chart (IMPL-0020 Open Question 6).
  A chart publishes from a `v*-beta.*` tag or a release, never from a
  `dont-release` merge (`release.yml` calls component `chart` only when
  it cut a tag, IMPL-0021 Open Question 2), and
  `ghcr.yml`'s chart job runs a `helm pull` idempotency precheck, so a
  tag cut with an unbumped chart publishes NO chart and the run is still
  all-green: the chart summary shows `Already published: true`. `appVersion`
  tracks the app release and must be BARE semver (metadata-action
  strips the `v`); `tests/chart/version_test.yaml` pins both image tags
  to it, so the three move together. `charts/` is in `.prettierignore` — do
  not reformat those files with a JSON/YAML round-trip.
- Per-task local gate is `just ci` semantics: test, lint, `tsc -b
  --force`, build, AND `bun run format:check` — formatting misses fail
  CI even when everything else is green.
- Bundle budget: CI fails if the eager JS tops 165 KB gz
  (`scripts/bundle-budget.ts`, `just ui bundle-budget`). It was 130 KB
  until IMPL-0033 Phase 4: the design system's parts arrive before
  docz's own leave, and its frame is the first paint, so the budget
  holds that overlap until Phase 9 sets it to the measured total plus
  6 KB. The script's comment carries the numbers. With the frame
  mounted (Phase 7) the eager set measures 171.6 KB, OVER the 165 KB
  ceiling: the package's `components/sidebar` imports its sheet and
  tooltip statically, about 45 KB gz of Base UI that is the first
  paint. The number is left as it is until that is decided (IMPL-0033
  Phase 7's note).
  "Eager" = the entry chunk PLUS every modulepreload'd chunk in
  index.html — Rollup splits shared statics out of `index-*.js` as
  the graph shifts, so measuring only the entry file would let an
  eager import hide in a preloaded chunk. Keep the markdown
  pipeline/Shiki behind lazy imports — an eager import is exactly
  what the budget exists to catch. Doc links prefetch getDoc on
  hover/focus via `usePrefetchDoc` (`src/hooks/usePrefetchDoc.ts`) —
  new doc-link surfaces should wire it up. One-off node scripts live in `scripts/*.ts` under
  tsconfig.node.json (node types, typechecked + linted there).

- TypeScript is pinned to the 5.9 series: typescript-eslint's parser
  cannot load the TS 7 (native compiler) line. Don't bump the major
  until typescript-eslint supports it.
- package.json `overrides` holds a `dompurify` floor of `^3.4.13`.
  mermaid asks for `^3.4.12` and GHSA-55q2-fjhq-7xh7 covers `<=3.4.12`;
  it isn't reachable through mermaid (it needs `IN_PLACE` plus an
  element hook that detaches a node — mermaid sanitizes a string and
  only hooks attributes) but this is the sanitizer behind the one
  `innerHTML` here, so it stays above the advisory. Fix a transitive
  advisory with an override, never `bun update <pkg>` — that adds a
  direct dependency on a package nothing imports.
- ESLint is flat config (`eslint.config.js`): typescript-eslint
  strict + stylistic type-checked (projectService), react-hooks flat
  recommended, jsx-a11y, eslint-config-prettier last. Generated dir is
  ignored. One `no-restricted-imports` pattern: never deep-path into
  `mermaid/*` — the bare specifier maps to `dist/mermaid.core.mjs`,
  while the minified sibling carries syntax es-module-lexer (Vite)
  rejects.
- react-hooks v7 forbids `setState` inside effects
  (`set-state-in-effect`) — sync prop→state with the react.dev
  "adjust state during render" pattern (guarded `setState` in render
  body, see `SearchBox` in directory.tsx), not a `useEffect`.
- Accessibility gate: `src/a11y/axe.test.tsx` runs axe-core over every
  core view and tolerates zero serious/critical violations — new views
  belong in that sweep. jsdom can't compute color-contrast, so text
  token contrast (fg-/st-/t-/hash-/accent vs the three bg surfaces,
  4.5:1) is enforced mathematically in `src/theme/contrast.test.ts`
  (for tokens.css and for every theme and mode in `../pkg/theme/tokens`),
  and full-rule axe (contrast included) runs in `e2e/a11y.spec.ts`;
  changing `tokens.css` colors means keeping both green. The math also
  holds accent on its translucent `accent-bg` tint composited over
  `bg-base` and `bg-raised` (a selected nav item, a pressed toggle),
  which real-browser axe caught 0.05 short on paper light. The a11y and
  rendering specs run once per shell mode in `e2e/appearances.ts`'s
  `SHELL_MODES`, stored through `useShellMode(page, mode)` before the
  first script;
  a check on a themed colour reads the token from the page rather
  than naming one theme's value. Every
  top-level route needs exactly one `<main>` (RepoFrame provides it
  for repo-scoped pages). Doc-prose links stay underlined
  (link-in-text-block). vitest stubs CSS imports even with `?raw` —
  read CSS source in tests via node:fs (per-file
  `/// <reference types="node" />`; the app tsconfig stays
  browser-only).
- e2e: `just e2e` = Playwright against an MSW-enabled preview build
  (`build:msw` → dist-msw/, worker gated on VITE_API_MODE=msw; the
  deployable dist/ never contains MSW). MSW answers requests in-page
  before Playwright can intercept — drive error journeys through
  sessionStorage flags read by browser-worker-only overrides in
  src/mocks/browser.ts (`docz:e2e:force-401`). The flip side: MSW's
  worker BYPASSES document navigations, so those (e.g. the
  `/auth/login` anchor hop) are mocked with Playwright `page.route` —
  in ONE fulfill; Playwright doesn't re-route browser-followed
  redirects and the preview proxy (preview.proxy defaults to
  server.proxy) would leak a mocked 302 chain to a real docz-api.
  cmdk gotcha: with a controlled `value`, cmdk never auto-selects, so
  keep the active key pointed at a real item (adjust-during-render in
  command-palette.tsx) or Enter does nothing.

## Non-negotiables

- This GitHub repo is PUBLIC. Secrets go in `.env.local` (gitignored),
  never in `mise.toml` or committed files.
- No tokens in JS-readable storage — auth is docz-api's httpOnly
  `docz_session` cookie only.
- XSS sanitization gates CI: search snippets (`<em>`-highlighted) and
  doc markdown are untrusted input.
- Conventional commits (git-cliff changelog); branches `<type>/<kebab>`.
- After each IMPL-0001 task: check it off in the impl doc, update this
  file if guidance changed, commit.

## In the docz repository

What moving docz-site into docz's repository changed (IMPL-0020,
DESIGN-0017, ADR-0004). Publishing and CI's path filters are in
`.github/CLAUDE.md`.

- **`ui/go.mod` fences the npm tree.** It is a stub
  (`module github.com/donaldgifford/docz/v2/ui`) with no Go in it, and it
  exists because `ui/node_modules` ships stray `.go` files (flatted's) that
  the root module's `./...` would otherwise walk into and fail to build. A
  nested `go.mod` ends the root module at `ui/`, so `go test ./...`,
  golangci-lint, and go-licenses need no exclusion. `TestUIModuleFence` in
  `test/archive` pins it exists and that `go list` stops there; the ui CI job,
  the one place `node_modules` is populated, proves it still works.
- **One spec, four readers.** `api/openapi.yaml` is the only copy: docz-api
  embeds it (`api/spec.go`) and serves it at `GET /openapi.yaml`, the
  contract test in `internal/httpapi` reads the same embed, `api::lint-openapi`
  runs vacuum over it, and orval generates the client from it
  (`ui/orval.config.ts` → `../api/openapi.yaml`). The vendored
  `ui/api/openapi.yaml` and docz-site's `spec-drift.yml` are gone, so a spec
  change that breaks the client fails the same PR in `typecheck` or
  `gen-api-check`.
- **The image sees the spec through a named context.** `Dockerfile.ui` builds
  with context `ui/` and gets the spec from bake's `contexts = { spec = "api" }`
  via `COPY --from=spec openapi.yaml /api/openapi.yaml`, laid out so orval's
  `../api/openapi.yaml` resolves inside the stage, and `api/cli/` the same way
  for `gen-cli`, since `tsc -b` checks the review page too. `pkg/theme` arrives
  as a second named context, `theme`, copied to `/pkg/theme`, where
  `gen-themes` reads the token files and writes `page.css` (pkg/theme
  scoped to `.docz-page`) and the pane imports `runtime.js`
  (IMPL-0028, IMPL-0032); `just ui docker-build` passes the same
  `--build-context`, and `vite.config.ts` lists `../pkg/theme` in
  `server.fs.allow` beside the workspace root. **Both pages wear the
  shell** (DESIGN-0026, DESIGN-0028): the design system's tokens through
  `core.css`, with docz's own in `ui/src/theme/docz-tokens.css`, in light
  and dark, with no `data-theme` on `<html>` and no theme selector. **A document's
  theme sits on its own `.docz-page`**: the deployment's `DOCZ_THEME` in
  the reader, the repository's `html.theme` (which `docz inspect` reports
  and the session carries, IMPL-0030) in the review page's document
  column. The light/dark toggle shares `docz:mode` with exported pages,
  and a mode change redraws mermaid through `refreshMermaidTheme`.
  `contrast.test.ts` holds the shell in both modes and every theme and
  mode to 4.5:1, reading `pkg/theme`. Only the spec, the CLI schemas, and
  the themes cross the boundary; `.dockerignore` excludes `ui` from the
  api image's root context.
- **The reader shows a document as its docz-html fragment** (DESIGN-0026,
  IMPL-0032). `pkg/export/html`'s `RenderOptions.Fragment` writes the
  page's `div.wrap` inside one `div.docz-page` with no stylesheet, runtime,
  mermaid loader, or mode toggle (`docz export html --fragment`; the page
  and the fragment share one template, so the page goldens pin both).
  docz-api serves it at `GET /api/v1/repos/{owner}/{name}/types/{type}/docs/{doc_id}/html`
  (`internal/httpapi/fragment.go`, spec 1.8.0): the type's heading spec
  from its embedded template, `html.theme` from the config snapshot,
  relative links mapped to docz-site's routes, and the same 404 as
  `getDoc` for a hidden document. `pkg/theme/runtime.js`'s
  `docz.mount(scope, opts)` runs the runtime inside one element (`hash`,
  `tab`, `onTab`; returns `showTab`, `unmount`) and starts itself only when
  `<html>` has `data-doc`, so exported pages are unchanged. `ui`'s
  `DocPane` injects the fragment, mounts the runtime with the tab in
  `?tab=`, draws `pre.mermaid` with the site's mermaid, routes `/`-paths
  in-app, and falls back to the site's own render with a note when the
  fragment fails. `TestUIFragmentFixtures` and `TestUIHostileFragments`
  (`internal/httpapi/ui_fixtures_test.go`, `-update`) render the MSW demo
  documents and the XSS payloads through the handler's `renderFragment`
  into `ui/src/mocks/__generated__/`, so the site's tests embed what
  docz-api sends; both files are in the `go` path filter.
