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
- Markdown reader pipeline (Phase 1): remark-parse → remark-gfm →
  remark-rehype (allowDangerousHtml) → rehype-raw → **rehype-sanitize** →
  rehype-slug → @shikijs/rehype → React. Sanitize AFTER rehype-raw,
  highlight AFTER sanitize. No `dangerouslySetInnerHTML`.

## Toolchain notes

- TypeScript is pinned to the 5.9 series: typescript-eslint's parser
  cannot load the TS 7 (native compiler) line. Don't bump the major
  until typescript-eslint supports it.
- ESLint is flat config (`eslint.config.js`): typescript-eslint
  strict + stylistic type-checked (projectService), react-hooks flat
  recommended, jsx-a11y, eslint-config-prettier last. Generated dir is
  ignored.

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
