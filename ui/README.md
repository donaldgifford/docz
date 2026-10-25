# docz-site

Web UI for [docz-api](https://github.com/donaldgifford/docz) — a
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
with your recently-opened docs and pages, and prefetches the
highlighted hit.

## Pages

Repos that enable the docz 1.2.0 `api:` config block publish non-docz
markdown — READMEs, guides, any `additional_docs` — alongside their
typed documents. The site renders them at
`/:owner/:repo/pages/<published path>` through the same sanitizing
reader pipeline, adds a collapsible Pages tree to the repo nav, lists
page hits inline in the directory and palette (a neutral `page`
marker, never a type badge), and resolves relative links between docs
and pages in both directions. Repos without the block are untouched —
no extra requests, byte-identical surfaces.

## Quickstart

```sh
mise install       # pinned toolchain (bun, just, linters)
just ui install    # dependencies
just ui dev        # dev server, proxied to a local docz-api on :8080
```

No docz-api running? Use the MSW-backed dev server instead:

```sh
just ui dev-msw    # same app, API served from mock fixtures
```

`just --list ui` lists every task. `just ui ci` runs the same chain as
the CI workflow's `ui` job.

Working against the full local stack instead? With docz-api's local
compose stack running:

```sh
just ui local-up   # build + run the site container on :8090, joined
                   # to the docz-api stack's network
just ui local-down # tear it down
```

Re-run `local-up` after changes — it rebuilds and recreates the
container.

## Auth

Authentication is docz-api's: the site never sees a token, only the
httpOnly `docz_session` cookie. `/login` renders one button per
enabled provider (`github`, `okta`, `keycloak`). In a container the set
is chosen at runtime via `DOCZ_AUTH_PROVIDERS` (comma-separated; the
server injects it into the page, so one image serves any combo — the
Helm chart's `config.authProviders`), falling back to the build-time
`VITE_AUTH_PROVIDERS`, then `github`. It must match docz-api's own
`AUTH_PROVIDERS`; docz-api owns the OAuth/OIDC exchange and the GitHub
App ingest, which is independent of the login provider. On a 401 the app
stashes the intended destination, sends you to `/login`, and restores
the deep link after the OAuth callback lands.

When docz-api runs with `AUTH_PROVIDERS=none` (its login-free
first-setup mode), the site detects it automatically from the session
response (`provider: "none"`) — no site-side configuration. The topbar
shows no sign-in or account chrome at all, and `/login` explains that
authentication is disabled instead of offering provider buttons. A
`503` from the session gate (backend unreachable) is treated as a
transient outage, never a logout: the topbar keeps its inert
placeholder and re-probes every ~30 s until the backend recovers,
while data routes show their normal retryable error panel.

The site also renders the MCP consent page at `/oauth/consent`. When
docz-api runs with `MCP_ENABLED=true` and a login provider, its
`/oauth/authorize` sends a person here with an `authz` id; the page reads
the pending request from `/api/v1/oauth/authorizations/{id}` (behind the
session gate, so a logged-out person sees `/login` first and returns),
shows the client and the scopes it asks for, and on allow follows the
`redirect_to` the API answers with back to the client. docz-api finds the
page through `DOCZ_SITE_URL` (the chart's `api.mcp.siteUrl`).

## Nav pins

`DOCZ_NAV_LINKS` pins up to six deployment-chosen links into the topbar
(the Helm chart's `config.navLinks`): a JSON array of `{label, href}`,
whitelist-validated on both ends — short label charset, same-origin
app-path hrefs — so invalid entries degrade to fewer or no pins, never
a broken page. Runtime config wins; the build-time `VITE_NAV_LINKS`
(same JSON) is the dev/e2e fallback; unset means no pins.

## Test

```sh
just ui test       # vitest unit/component suite (jsdom + MSW fixtures)
just ui e2e        # Playwright against an MSW-enabled preview build
```

The unit suite includes the XSS sanitization suites, an axe
accessibility sweep of every core view, and a mathematical WCAG
contrast check over the color tokens. The e2e suite drives the real
browser journeys (directory → filter → read, palette search, deep
links, 404/401) plus full-rule axe including color-contrast. Both run
in CI, along with a gzip size budget over the eager JS — the entry
chunk plus its modulepreload'd closure (`just ui bundle-budget`).

## Build

```sh
just ui build      # tsc -b + vite build → dist/
just ui preview    # serve the production build locally
```

Routes, Shiki grammars, the markdown pipeline, and mermaid are
separate lazy chunks — mermaid (~700 KB) and the ELK layout engine it
uses (~436 KB gzipped) only download on documents that actually contain
a diagram. `scripts/bundle-budget.ts` fails CI if the eager JS exceeds
its gzip budget.

### Supported browsers

**Chrome/Edge 119+, Firefox 122+, Safari 17.4+** — the first releases
with full ES2024 support, which is what mermaid 12 ships. Safari is the
binding constraint. The same list is `build.target` in
`vite.config.ts`, so the bundler stops claiming support it cannot
deliver; older browsers are not transpiled for, they are unsupported.

## Deploy

The image builds from the repository root's `Dockerfile.ui`, with `ui/`
as its context and the OpenAPI spec as a second, named context, because
orval generates the client at build time from `../api/openapi.yaml`:

```sh
just ui docker-build        # docker build -f ../Dockerfile.ui --build-context spec=../api …
docker buildx bake dev-ui   # the same through docker-bake.hcl, run from the root
docker run -p 8080:8080 -e DOCZ_API_URL=http://docz-api:8080 donaldgifford/docz-site:dev
```

A plain `docker build .` from `ui/` finds no Dockerfile. Releases publish
`ghcr.io/donaldgifford/docz-site` from the same tag as docz-api, and the
chart deploys both; see
[charts/docz/README.md](../charts/docz/README.md).

The image is a multi-stage build: dist/ plus `server/serve.ts`, a small
`Bun.serve` that serves hashed assets immutably (precompressed where it
pays), falls back to `index.html` for SPA routes, answers the `/healthz`
and `/readyz` probes, and proxies `/api`, `/auth`, `/webhooks`, and
`/openapi.yaml` to
docz-api — browser and API share one origin, so the httpOnly session
cookie is first-party and there is no CORS.

### Observability

The server logs one structured object per line to stdout — JSON by
default, `text` for local runs (`DOCZ_LOG_FORMAT`). `DOCZ_LOG_LEVEL`
takes `debug`, `info` (default), `warn`, or `error`; an unrecognized
value falls back to `info` rather than to something noisier.

At the default level you get the startup line and any proxy failure,
naming the cause — an unreachable docz-api and a missing `DOCZ_API_URL`
are distinct reasons, where both used to be a silent `502`. `debug`
adds a line per request plus the proxied `/auth/*` flow, which is the
level to reach for when troubleshooting an Okta or Keycloak login.

**Credential-bearing values never reach a log at any level.** Query
*keys* are kept and every *value* is replaced with `<redacted>`
(allowlist, so a new parameter is redacted by default), headers are
never recorded — a session cookie shows up as `has_cookie: true` and
nothing more — and a `Location` is reduced to its host. A test drives a
realistic OAuth callback through the serialized output at every level
and fails if a `code`, `state`, or cookie value appears.

The probes are split, because Kubernetes responds to them differently:

| Path | Probe | Behaviour |
| --- | --- | --- |
| `/healthz` | liveness | unconditional `200 ok` |
| `/readyz` | readiness | `200`/`503` with per-check status |

A failing liveness probe **restarts** the container; a failing
readiness probe **holds traffic**. A missing or mis-mounted `dist/` is
not fixed by restarting, but it should stall the rollout and leave the
previous ReplicaSet serving — so `/readyz` checks that `dist/` is
servable and that the runtime config validated, while `/healthz` stays
unconditional. `/readyz` makes **no call to docz-api**: gating
readiness on the API would evict every pod from the Service the moment
it blipped, turning a degradation the SPA already handles into a total
outage.

`/healthz`, `/readyz`, and `/metrics` are reserved server paths — no
SPA route can claim them.

Prometheus metrics are on `/metrics`, enabled by default
(`DOCZ_METRICS_ENABLED`): request counts and durations by method and
route class, upstream proxy durations, and `docz_site_proxy_errors_total`
— the docz-api health signal that can page a human without evicting a
pod. Labels are drawn from a closed set, so no request can inflate
cardinality. When disabled the endpoint returns an explicit `404`
rather than the SPA shell. One caveat: `nodejs_gc_duration_seconds` is
declared but never samples under Bun, so stock Node dashboards show
empty GC panels.

Tracing is OpenTelemetry over OTLP/HTTP, off until
`OTEL_EXPORTER_OTLP_ENDPOINT` names an absolute `http(s)` URL — with no
endpoint no provider is registered and no network call is attempted.
It is **hand-instrumented on purpose**: OTel's HTTP auto-instrumentation
records `url.full`, which on a server that proxies `/auth/*` would ship
OAuth codes to a collector. Span attributes are an allowlist and the
proxy hop carries a `traceparent`, so docz-api — which already extracts
it — joins the same trace with no configuration of its own.

The server ships as a single bundled file (`bun build --target=bun`),
because the runtime image carries no `node_modules`. `just ui build-server`
produces it; `just ui serve-bundle` runs what the container runs.

`deploy/ui/compose.yaml` is a reference single-host stack: the site is the
only published port, with docz-api and its dependencies (Postgres,
Redis, Meilisearch) on a private network. See
`deploy/api/README.md` for the env store and secret conventions it shares.
`deploy/ui/compose.local.yaml` (via `just ui local-up`) runs just the site
container joined to docz-api's local development stack.

## docz-review

`docz-review` is the blocking review page for documents an agent writes
(DESIGN-0024). It shares this project's markdown pipeline but is its
own entry (`review.html`), its own Bun server (`review/`), and its own
binary. With the Claude Code plugin installed, a session's `Stop` hook
opens the page for every document the session created; it also runs by
hand.

### From a checkout

```bash
just build                # the docz it drives, at build/bin/docz
cd ui
bun run build:review      # the page, into ui/dist-review
DOCZ_REVIEW_DOCZ=../build/bin/docz bun review/main.ts DESIGN-0024 --repo-root ..
```

The page opens in your browser; `--no-open` prints its URL to stderr
instead (the URL carries the session's token, so treat it as a secret).
Submit, Approve, or Dismiss each document, or click Done. `--json`
prints the review records when the run ends.

| Command | Does |
| --- | --- |
| `docz-review <id\|path>...` | Review the named documents now |
| `docz-review --resume <review id>` | Resume a review started by hand |
| `docz-review --pending --hook` | The `Stop` hook: review the session's armed documents |
| `docz-review arm <id\|path>...` | Check the documents resolve, as the plugin's skill does |
| `docz-review pending [--purge]` | List this repository's documents under review, or purge expired state |
| `docz-review --version` | Its version, the contract it speaks, and the docz it found |

Environment: `DOCZ_REVIEW_DOCZ` (the docz binary, else `PATH`),
`DOCZ_REVIEW_STATE_DIR` (else `$XDG_STATE_HOME/docz/review`, else
`$HOME/.local/state/docz/review`), `DOCZ_REVIEW_BROWSER` (else
`BROWSER`, else `open` or `xdg-open`), `DOCZ_REVIEW_PORT` (else
random). docz-review needs docz 2.2.0 or later.

Recipes: `just ui dev-review` runs the page against an in-process fake
server with no docz at all, `just ui test-review` runs the page's and
the server's tests, `just ui e2e-review` the browser test, and
`just ui build-review` the four compiled binaries into
`ui/dist-review-bin/`, smoke-testing the host's from an empty
directory.

### Install with mise

The binaries are attached to docz's GitHub releases. `docz` installs
from the same repository, so `docz-review` needs a tool alias to be a
second tool rather than a second entry for the first:

```toml
[tool_alias]
docz-review = "github:donaldgifford/docz"

[tools]
docz = "latest"
docz-review = { version = "latest", asset_pattern = "docz-review_{{ os(macos=\"darwin\") }}_{{ arch(x64=\"amd64\") }}", bin = "docz-review" }
```

Verify a download against `docz-review_checksums.txt` and its
signature, both attached to the same release.

## How it fits together

- `../api/openapi.yaml` — docz-api's spec, at the repository root and
  read in place: `just ui gen-api` regenerates the typed TanStack Query
  client into `src/api/__generated__/` (gitignored). A spec change that
  breaks the client fails the same pull request's `ui` job, so there is
  no vendored copy and no drift workflow.
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
