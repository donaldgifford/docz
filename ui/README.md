# docz-site

Web UI for [docz-api](https://github.com/donaldgifford/docz-api) — a
cross-repo reader, search directory, and repo pages for docz-managed
documentation (RFCs, ADRs, designs, guides, and friends) ingested from
GitHub repos.

Vite + React 19 SPA · TypeScript strict · Tailwind CSS v4 · TanStack
Query with an orval-generated client · Bun.

## Quickstart

```sh
mise install       # pinned toolchain (bun, just, linters)
bun install        # dependencies
just dev           # dev server, proxied to a local docz-api on :8080
```

No docz-api running? Use the MSW-backed dev server instead:

```sh
just dev-msw       # same app, API served from mock fixtures
```

`just` with no arguments lists every task. `just ci` runs the same
chain as the CI workflow.

## Test

```sh
just test          # vitest unit/component suite (jsdom + MSW fixtures)
just e2e           # Playwright against an MSW-enabled preview build
```

The unit suite includes the XSS sanitization suites, an axe
accessibility sweep of every core view, and a mathematical WCAG
contrast check over the color tokens. The e2e suite drives the real
browser journeys (directory → filter → read, palette search, deep
links, 404/401) plus full-rule axe including color-contrast. Both run
in CI, along with a gzip size budget for the entry chunk
(`just bundle-budget`).

## Build

```sh
just build         # tsc -b + vite build → dist/
just preview       # serve the production build locally
```

Routes, Shiki grammars, and the markdown pipeline are separate lazy
chunks; `scripts/bundle-budget.ts` fails CI if the entry chunk exceeds
its gzip budget.

## Deploy

```sh
docker build -t docz-site .
docker run -p 8080:8080 -e DOCZ_API_URL=http://docz-api:8080 docz-site
```

The image is a multi-stage build: dist/ plus `server/serve.ts`, a small
`Bun.serve` that serves hashed assets immutably (precompressed where it
pays), falls back to `index.html` for SPA routes, answers `/healthz`,
and proxies `/api`, `/auth`, `/webhooks`, and `/openapi.yaml` to
docz-api — browser and API share one origin, so the httpOnly session
cookie is first-party and there is no CORS.

`deploy/compose.yaml` is a reference single-host stack: the site is the
only published port, with docz-api and its dependencies (Postgres,
Redis, Meilisearch) on a private network. See docz-api's
`deploy/README.md` for the env store and secret conventions it shares.

## How it fits together

- `api/openapi.yaml` — vendored copy of docz-api's spec. `just gen-api`
  regenerates the typed TanStack Query client into
  `src/api/__generated__/` (gitignored). A scheduled workflow watches
  upstream for spec drift.
- `src/app/` — router (library-mode react-router) and app shell;
  `src/routes/` — one lazy module per route.
- `src/markdown/` — the only place markdown is rendered: remark/rehype
  with sanitization after raw-HTML expansion, Shiki highlighting, ToC
  collection, and doc-id cross-reference linking.
- `src/theme/tokens.css` — design tokens ported from `mockup.html`, the
  visual source of truth (contrast-checked in tests).
- Dev proxies `/api`, `/auth`, and `/openapi.yaml` to docz-api
  (override with `DOCZ_API_URL`); production deploys same-origin behind
  the API's cookie auth.

## Docs

- Design: `docs/design/0001-docz-site-cross-repo-docz-reader-and-search-ui.md`
- Build plan: `docs/impl/0001-docz-site-mvp-phased-build-of-the-reader-directory-and-repo.md`
- Operating notes for coding agents: `CLAUDE.md`

## License

Apache-2.0
