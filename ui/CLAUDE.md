# CLAUDE.md

Guidance for Claude Code when working in docz-site.

## What this is

Web UI for [docz-api](https://github.com/donaldgifford/docz-api) — a
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

- `bun run dev` — Vite dev server; proxies `/api`, `/auth`,
  `/openapi.yaml` to a local docz-api (`:8080`; override `DOCZ_API_URL`)
- `bun run dev:msw` — dev server against MSW fixtures (no API needed)
- `bun run typecheck` — `tsc -b` (solution-style tsconfig; plain
  `tsc --noEmit` checks NOTHING at the root — don't "simplify" it back)
- `bun run build` / `preview` / `test` / `lint` / `format:check`
- `bun run gen-api` — orval regenerates `src/api/__generated__/` from
  the vendored `api/openapi.yaml` (generated dir is gitignored)
- `just local-up` / `just local-down` — build + run the site container
  (`deploy/compose.local.yaml`, :8090) joined to the docz-api local
  stack's network; re-run `local-up` after changes to rebuild/recreate

## Architecture

- `vite.config.ts` — react + tailwind plugins, `@/` → `src/` alias, dev
  proxy (same-origin in prod; docz-api sends no CORS headers)
- Routing: `src/app/router.tsx` (`createBrowserRouter`, library mode)
  wraps everything in `src/app/AppShell.tsx` (topbar + `<Outlet/>`).
  Route modules live in `src/routes/*` and export a named `Component`;
  register new routes with `lazy: () => import("@/routes/<name>")` so
  each stays its own chunk.
- Fonts are self-hosted `@fontsource` imports in `src/main.tsx` (IBM
  Plex Sans/Mono, Source Serif 4) — never add a third-party font URL.
  New weights = new per-weight CSS import there.
- `src/theme/tokens.css` — the single global stylesheet: Tailwind v4
  import + `@theme static` tokens ported from `mockup.html` `:root`.
  Token names keep mockup prefixes, so utilities read `bg-bg-raised`,
  `text-fg-tertiary`, `text-t-rfc`. Radius scale is wiped (sharp
  corners); only `rounded-pill` exists. Don't add other CSS entrypoints.
- `tsconfig.json` is solution-style → `tsconfig.app.json` (browser code,
  strict + `noUncheckedIndexedAccess`) + `tsconfig.node.json` (config
  files). New config files at the repo root go in `tsconfig.node.json`'s
  `include`.
- `src/api/fetcher.ts` — the orval fetch mutator and the typed errors
  (`SessionRequiredError` 401, `NotFoundError` 404, `ApiError` rest).
  Match on these classes in UI code; never `fetch` the API directly.
  Success returns orval's `{ data, status, headers }` envelope —
  narrow on `status === 200` before touching `.data`. Query defaults
  live in `src/app/query-client.ts` (no retry on 401/404).
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
  collector → Shiki core highlighter, tokyo-night, slim lazy grammar
  set, chrome transformer stamping data-language/data-caption →
  wrap-codeblock (div.codeblock header chrome; skips mermaid) →
  xref linkify → hast-to-JSX). Sanitize AFTER rehype-raw, highlight
  AFTER sanitize. Mermaid: `mermaid-marker` runs post-sanitize/
  pre-Shiki (strips language-mermaid so Shiki can't replace the pre,
  moves source onto `data-mermaid-source`), and MarkdownPre routes
  marked pres to `MermaidBlock` (`src/markdown/mermaid-block.tsx`),
  which lazy-imports mermaid (~700 KB, own chunk — e2e asserts it
  never loads on diagram-free docs) and holds the ONE sanctioned
  innerHTML in the codebase: mermaid.render() output under
  `securityLevel: "strict"` AND `htmlLabels: false` — BOTH required
  (strict alone still materializes purified `<img src>` elements in
  foreignObject labels); render failure keeps the source visible.
  No `dangerouslySetInnerHTML` anywhere. Never widen
  `schema.ts` without extending the XSS suite — its only non-default
  allowances are `language-*` classes + the charset-validated
  `metastring` on `code`, and value-RESTRICTED admonition classNames
  on div/span (forged markup gets the same inert styling at most);
  data-* on `pre` dies in sanitize, which is exactly why the
  post-Shiki chrome can trust it. Admonition tint backgrounds are
  PRECOMPUTED hex tokens (`--color-adm-*-bg`) so contrast.test.ts can
  enforce label/body pairs — don't swap them for color-mix.
- Xrefs (`src/markdown/xrefs.ts`): doc-id tokens linkify only when they
  resolve in the caller-supplied map (UPPERCASED doc_id → href, built
  by `useRepoDocIndex` from listDocs) — the map is the whitelist and
  hrefs come from API data, never document text. Tokens inside
  `a`/`code`/`pre` stay text; the reader drops the doc's own id.
  `MarkdownAnchor` turns `data-xref` anchors into router Links (tests
  need a router around rendered content). Render-cache keys carry an
  fnv1a fingerprint of the sorted resolver ids, so bodies re-render at
  most once when the doc index finishes loading.
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
- Auth UX (Phase 5): `/login` (`src/routes/login.tsx`) renders provider
  buttons as REAL `<a href="/auth/login?provider=…">` anchors — the
  OAuth 302 must reach the browser, so never convert them to router
  Links. The enabled set comes from `VITE_AUTH_PROVIDERS`
  (`src/lib/authProviders.ts`; build-time, comma-separated, default
  `github`, unknown keys dropped, empty result falls back to GitHub).
  On 401, `SessionRequiredRedirect` (query-states.tsx) stashes
  `pathname+search` via `src/lib/authReturn.ts` and replaces to
  `/login`; `RestoreAfterLogin` (AppShell) probes getSession on "/"
  when a stash exists and restores it only on 200 (the OAuth callback
  always lands on "/"). The stash validates paths on BOTH write and
  read — keep it that way (open-redirect guard), and never stash
  anything but a same-origin path. Test setup clears session/local
  storage after each test — a leaked stash arms RestoreAfterLogin in
  unrelated tests. Topbar identity is `SessionMenu`
  (`src/components/session-menu.tsx`): getSession-driven (fixtures
  answer with a deterministic `donaldgifford` github identity), avatar
  is a disclosure (not `role="menu"`), and logout runs `onSettled` —
  navigate to `/login` BEFORE `queryClient.clear()`, or the page being
  left refetches everything under the dead session.
- Reader lives in `src/routes/doc.tsx` + `src/components/doc-rail.tsx`
  + `src/components/query-states.tsx` (shared 401/404/error panels).
- Directory (`src/routes/directory.tsx`): the URL is the only source of
  filter truth — read via `parseSearchParams`, write via
  `serializeSearchState` (`src/lib/searchParams.ts`; its
  `toSearchDocsParams` maps state → API params, first-of-array facets).
  Typed queries debounce ~200 ms and commit with `replace: true`;
  discrete filter actions must push so back/forward walks history.
  `SearchHit` has NO `updated_at` (additive ask in DESIGN-0001) — the
  updated column renders "—"; `src/lib/relativeTime.ts` takes over when
  the field lands.
- Faceted controls exclude their own dimension via separate limit-0
  searchDocs queries (directory picker/chips AND palette pills) so
  every option stays offered while one is selected. URL `offset` means
  "rows 0..offset+PAGE_SIZE shown"; the query grows `limit` from 0 so
  deep links render identical rows.
- Repo pages share `RepoFrame` (`src/components/repo-frame.tsx`): the
  three-column grid (sticky RepoNav · content · 190px rail, collapsing
  at 1181px/861px) plus RepoBreadcrumbs — home, type pages, AND the
  reader all mount inside it. Counts everywhere come from
  `useRepoFacts` (repo-filtered limit-0 facet query) so numbers agree.
  RepoNav's per-type doc lists are collapsible drawers: the route's
  `:type` auto-expands, the caret button peeks without navigating, and
  listDocs only fires for open drawers. Facets omit zero-hit types —
  a missing typeCounts key after facts load means 0, which also
  disables the caret. The repo home is the ONLY surface rendering an
  h1 inside `.doc-prose` (the reader strips body h1s) — its style
  lives in tokens.css; don't remove it as "unused".
  URL `{type}` resolves by name/id_prefix/alias via
  `lib/docTypes.resolveDocType` (links always generated from the
  canonical name); fixtures mirror this and 404 unknown types.
- Repo home renders getRepoIndex's index.md (spec 1.1.0) through
  `useRenderedSource` with its h1 KEPT; the reader strips the h1 via
  the `useRenderedMarkdown` wrapper. 404 from getRepoIndex = generated
  home fallback, not an error.
- Palette (`src/components/command-palette.tsx`, mounted in AppShell):
  state is palette-local, never the URL. cmdk normalizes item values —
  keys are lowercased and navigation resolves hits through the list for
  original casing. Snippets render ONLY through
  `src/components/snippet.tsx` (splits on literal <em> markers, emits
  <mark>, everything else stays text) — never parse snippet HTML.
  jsdom setup stubs scrollIntoView/ResizeObserver for cmdk.

## Toolchain notes

- Never commit credential-shaped strings — even fake ones in fixture
  prose or docs (including THIS file). trufflehog scans the full PR
  commit range with unverified findings fatal; any database URI
  carrying a user-colon-password pair trips it regardless of the
  password's value (a REDACTED placeholder still matches — drop the
  password component entirely), and a purge means rewriting branch
  history.
- Per-task local gate is `just ci` semantics: test, lint, `tsc -b
  --force`, build, AND `bun run format:check` — formatting misses fail
  CI even when everything else is green.
- Bundle budget: CI fails if the eager JS tops 130 KB gz
  (`scripts/bundle-budget.ts`, `just bundle-budget`, ~120 KB today).
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
- ESLint is flat config (`eslint.config.js`): typescript-eslint
  strict + stylistic type-checked (projectService), react-hooks flat
  recommended, jsx-a11y, eslint-config-prettier last. Generated dir is
  ignored.
- react-hooks v7 forbids `setState` inside effects
  (`set-state-in-effect`) — sync prop→state with the react.dev
  "adjust state during render" pattern (guarded `setState` in render
  body, see `SearchBox` in directory.tsx), not a `useEffect`.
- Accessibility gate: `src/a11y/axe.test.tsx` runs axe-core over every
  core view and tolerates zero serious/critical violations — new views
  belong in that sweep. jsdom can't compute color-contrast, so text
  token contrast (fg-/st-/t-/hash-/accent vs the three bg surfaces,
  4.5:1) is enforced mathematically in `src/theme/contrast.test.ts`,
  and full-rule axe (contrast included) runs in `e2e/a11y.spec.ts`;
  changing `tokens.css` colors means keeping both green. Every
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
