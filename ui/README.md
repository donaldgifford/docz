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

`just` with no arguments lists every task (lint, fmt, test, build,
preview, gen-api, ci, …). `just ci` runs the same chain as the CI
workflow.

## How it fits together

- `api/openapi.yaml` — vendored copy of docz-api's spec. `just gen-api`
  regenerates the typed TanStack Query client into
  `src/api/__generated__/` (gitignored). A scheduled workflow watches
  upstream for spec drift.
- `src/app/` — router (library-mode react-router) and app shell;
  `src/routes/` — one lazy module per route.
- `src/theme/tokens.css` — design tokens ported from `mockup.html`, the
  visual source of truth.
- Dev proxies `/api`, `/auth`, and `/openapi.yaml` to docz-api
  (override with `DOCZ_API_URL`); production deploys same-origin behind
  the API's cookie auth.

## Docs

- Design: `docs/design/0001-docz-site-cross-repo-docz-reader-and-search-ui.md`
- Build plan: `docs/impl/0001-docz-site-mvp-phased-build-of-the-reader-directory-and-repo.md`
- Operating notes for coding agents: `CLAUDE.md`

## License

Apache-2.0
