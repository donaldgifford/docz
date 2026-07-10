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
- `tsconfig.json` is solution-style → `tsconfig.app.json` (browser code,
  strict + `noUncheckedIndexedAccess`) + `tsconfig.node.json` (config
  files). New config files at the repo root go in `tsconfig.node.json`'s
  `include`.
- `src/api/__generated__/` — orval output; never hand-edit, never commit
- Generated OpenAPI types ARE the data model; don't hand-roll DTO types
- Markdown reader pipeline (Phase 1): remark-parse → remark-gfm →
  remark-rehype (allowDangerousHtml) → rehype-raw → **rehype-sanitize** →
  rehype-slug → @shikijs/rehype → React. Sanitize AFTER rehype-raw,
  highlight AFTER sanitize. No `dangerouslySetInnerHTML`.

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
