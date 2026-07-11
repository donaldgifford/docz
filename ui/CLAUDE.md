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
- Markdown rendering lives in `src/markdown/` and ONLY there:
  `preprocess.ts` (strip frontmatter + docz toc block) →
  `processor.ts` `renderMarkdown()` (remark-parse → remark-gfm →
  remark-rehype allowDangerousHtml → rehype-raw → **rehype-sanitize
  with `schema.ts`** → double-clobber collapse → rehype-slug + ToC
  collector → Shiki core highlighter, tokyo-night, slim lazy grammar
  set → hast-to-JSX). Sanitize AFTER rehype-raw, highlight AFTER
  sanitize. No `dangerouslySetInnerHTML` anywhere. Never widen
  `schema.ts` without extending the XSS suite.
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
