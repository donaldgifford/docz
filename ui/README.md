# docz-site

Web UI for [docz-api](https://github.com/donaldgifford/docz-api) — a
cross-repo reader, search directory, and repo pages for docz-managed
documentation (RFCs, ADRs, designs, guides, and friends) ingested from
GitHub repos.

Vite + React 19 SPA · TypeScript strict · Tailwind CSS v4 · TanStack
Query with an orval-generated client · Bun.

The reader renders sanitized markdown with Shiki-highlighted code
blocks (language badge + filename chrome), GitHub-style alert
callouts, lazily-loaded mermaid diagrams, doc-id cross-links, a
metadata table with an html/md/json format switch, and copy-link
section headings. The ⌘K palette searches every indexed repo, leads
with your recently-opened docs, and prefetches the highlighted hit.

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

Working against the full local stack instead? With docz-api's local
compose stack running:

```sh
just local-up      # build + run the site container on :8090, joined
                   # to the docz-api stack's network
just local-down    # tear it down
```

Re-run `local-up` after changes — it rebuilds and recreates the
container.

## Auth

Authentication is docz-api's: the site never sees a token, only the
httpOnly `docz_session` cookie. `/login` renders one button per
enabled OAuth provider; the set comes from `VITE_AUTH_PROVIDERS`
(build-time, comma-separated — e.g. `github,google,okta` — defaulting
to `github`). On a 401 the app stashes the intended destination,
sends you to `/login`, and restores the deep link after the OAuth
callback lands.

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
in CI, along with a gzip size budget over the eager JS — the entry
chunk plus its modulepreload'd closure (`just bundle-budget`).

## Build

```sh
just build         # tsc -b + vite build → dist/
just preview       # serve the production build locally
```

Routes, Shiki grammars, the markdown pipeline, and mermaid are
separate lazy chunks — mermaid (~700 KB) only downloads on documents
that actually contain a diagram. `scripts/bundle-budget.ts` fails CI
if the eager JS exceeds its gzip budget.

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
`deploy/compose.local.yaml` (via `just local-up`) runs just the site
container joined to docz-api's local development stack.

## How it fits together

- `api/openapi.yaml` — vendored copy of docz-api's spec. `just gen-api`
  regenerates the typed TanStack Query client into
  `src/api/__generated__/` (gitignored). A scheduled workflow watches
  upstream for spec drift.
- `src/app/` — router (library-mode react-router) and app shell;
  `src/routes/` — one lazy module per route.
- `src/markdown/` — the only place markdown is rendered: remark/rehype
  with sanitization after raw-HTML expansion, GitHub-alert
  admonitions, Shiki highlighting with codeblock chrome, mermaid
  diagram rendering, ToC collection, heading copy-links, and doc-id
  cross-reference linking.
- `src/theme/tokens.css` — design tokens ported from `mockup.html`, the
  visual source of truth (contrast-checked in tests).
- Dev proxies `/api`, `/auth`, and `/openapi.yaml` to docz-api
  (override with `DOCZ_API_URL`); production deploys same-origin behind
  the API's cookie auth.

## Docs

- Design: `docs/design/0001-docz-site-cross-repo-docz-reader-and-search-ui.md`
- Build plan: `docs/impl/0001-docz-site-mvp-phased-build-of-the-reader-directory-and-repo.md`
- Reader polish plan: `docs/impl/0002-reader-polish-rendering-pipeline-and-qol-backlog-from-inv-0001.md`
  (from `docs/investigation/0001-reader-ux-polish-qol-fixes-and-follow-ups.md`)
- Operating notes for coding agents: `CLAUDE.md`

## License

Apache-2.0
