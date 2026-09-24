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
  Bringing docz-api up for a real-stack check: its compose publishes
  :8080, which is often already taken on this machine. The site reaches
  it by service NAME over the shared network, so that publish is
  unnecessary — start it with a throwaway override setting
  `ports: !override []` rather than stopping whatever owns the port.

## Architecture

- `vite.config.ts` — react + tailwind plugins, `@/` → `src/` alias, dev
  proxy (same-origin in prod; docz-api sends no CORS headers)
- Routing: `src/app/router.tsx` (`createBrowserRouter`, library mode)
  wraps everything in `src/app/AppShell.tsx` (topbar + `<Outlet/>`).
  Route modules live in `src/routes/*` and export a named `Component`;
  register new routes with `lazy: () => import("@/routes/<name>")` so
  each stays its own chunk.
- Fonts are self-hosted `@fontsource` imports in `src/main.tsx` —
  never add a third-party font URL (mockup.html may use CDN links; it
  ships nowhere). DESIGN-0005 as amended: **Source Serif 4 Variable**
  (`--font-serif`) sets the article — reader prose, the headings inside
  it, pull quotes, and article titles; **Mona Sans Variable**
  (`--font-sans`) sets UI chrome and the heroes outside the article;
  **Monaspace Neon** (`--font-mono`) sets every mono surface, and its
  true italic is required for Shiki's italic scopes. Monaspace Xenon
  was removed with the seventh amendment — a mono can't set body text.
  Both variable families need the wght AND wght-italic imports.
  Prose sizing is dialed in on the specimen page, not in the abstract:
  body 19px/1.78 on a 66ch measure, headings in em so the hierarchy
  scales with the body, chrome one step above the mockup's original
  10–13.5px scale.
- Bordered mono chips MUST use `.mono-chip-y` (tokens.css) for their
  vertical padding and MUST NOT carry a `py-*` utility beside it —
  Tailwind's utilities layer would win. An inline box is as tall as the
  font's ascent+descent, and Monaspace Neon leaves 0.032em above its
  capitals against 0.19em below the baseline, so symmetric padding
  renders every chip visibly high. The class holds the compensating
  difference; `.doc-prose code` carries its own copy with descender
  clearance. Verify by measuring, not by eye.
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
  collector → Shiki core highlighter, tokyo-night run through
  `theme-contrast.ts` (every token color failing 4.5:1 on `code-bg`
  is nudged toward white — the comment family is ~2.5:1 raw; the test
  pins `CODE_BG` to tokens.css), slim lazy grammar set, chrome
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
  never loads on diagram-free docs) and holds the ONE sanctioned
  innerHTML in the codebase: mermaid.render() output under
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
  defaults to neo, which rounds node corners — diagrams would be the
  only rounded surface under a wiped radius scale. `layout` is NOT a
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
  h2–h4 map to `markdown-heading.tsx`, which appends the
  hover/focus-revealed copy-link button (a labeled BUTTON, not a
  link — the underline rule for prose links stays untouched).
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
  Links. The enabled set (`src/lib/authProviders.ts`; comma-separated,
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
  unrelated tests. Topbar identity is `SessionMenu`
  (`src/components/session-menu.tsx`): getSession-driven (fixtures
  answer with a deterministic `donaldgifford` github identity), avatar
  is a disclosure (not `role="menu"`), and logout runs `onSettled` —
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
  a root-level one swaps out AppShell and takes the topbar with it —
  stranding the user on a panel with no navigation, which is the exact
  failure it exists to fix. `route-error.test.tsx` pins both, including
  that the root placement loses the topbar, so the extra nesting level
  can't be refactored away as redundant. It CATCHES AND DISPLAYS only:
  no `window.onerror`, no `unhandledrejection`, no beacon (a test
  verifies their absence). A link home is the ONLY affordance — a retry
  would re-render the same crashed route and throw again. Panel chrome
  is shared via `ErrorPanelFrame` in query-states.tsx; the error state
  is in the axe sweep.
- Session classification (DESIGN-0003): auth chrome renders from
  `classifySession` (`src/lib/session.ts`) — pending / signed-in /
  anonymous (`provider === "none"`, docz-api's AUTH_PROVIDERS=none) /
  signed-out / unavailable. "Sign in" is reachable ONLY from
  signed-out, which only a real 401 produces; 503 and every other
  failure are `unavailable` → SessionMenu keeps an inert placeholder
  (`session-unavailable` testid, visually identical to pending) and
  the session query re-polls ~30 s via an error-gated
  `refetchInterval` (it never unmounts and focus-refetch is off — a
  healthy query must never poll). `anonymous` renders NO auth chrome
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
  AppShell renders pins between Repos and SessionMenu (one nav row =
  small-viewport parity). `build:msw` bakes an RFCs pin so e2e and
  dev-msw exercise the surface; the deployable build stays pin-free.
  Chart value `config.navLinks` (toJson; env omitted when empty).
  `docs` is a curated type: `--color-t-docs` (contrast-checked) +
  CURATED_TYPES + blurb — type colors are never runtime-configurable.
- Reader lives in `src/routes/doc.tsx` + `src/components/doc-rail.tsx`
  + `src/components/query-states.tsx` (shared 401/404/error panels).
  Since IMPL-0002 Phase 5 the right rail is ToC-ONLY: metadata is a
  bordered table under the doc header (`doc-meta-table.tsx`, fields
  ""-omitting, format switch html/md/json right-aligned above it) and
  the lifecycle is a closed-by-default `<details>` owned by
  `LifecycleRail` (renders nothing — shell included — for unknown
  types). Gated mockup rows (relationships, tags) slot into the table
  when the DESIGN-0001 API asks land. `TocList` runs a scroll spy
  (`src/hooks/useActiveHeading.ts`, IntersectionObserver over the
  heading ids, top-of-viewport band) and marks the current row
  `aria-current="location"` — it returns undefined where the observer
  is missing (jsdom) and HOLDS the last heading when a long section
  fills the band, so the rail never flickers to nothing.
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
  onboard. `src/lib/updatedAt.ts` holds `formatUpdatedStamp` (splits
  RFC3339 into the two lines; locale pinned en-US, zone is the
  reader's, `timeZone` arg for tests) and `formatRelativeTime` for
  surfaces wanting relative. KEEP `formatUpdatedStamp`'s `Number.isNaN`
  guard — it is what makes a deployment on a pre-1.5.0 docz-api render
  "—" instead of "Invalid Date", since the type promises a string the
  old API simply omits. Fixtures: docs forward their own stamp and
  `created`; pages take `PAGE_INGESTED_AT[repo]`, one value per repo
  because that is genuinely how onboard stamps behave.
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
  disables the caret. In-group rows (type drawers, pages tree) share
  the left rail from `src/components/nav-rail.ts` — the group draws a
  hairline, each row a 2px border over it, active rows color it in.
  Both files import from there, never from each other (RepoNav renders
  the pages section, so the other direction is a cycle). The repo home is the ONLY surface rendering an
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
  via its id_prefix/alias URL. The RepoNav row under Home gates on
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
  Discovery: RepoNav's Pages tree (`repo-nav-pages.tsx`, mounted only
  on an apiConfig hit), palette + directory render `source: "page"`
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
- Palette (`src/components/command-palette.tsx`, mounted in AppShell):
  state is palette-local, never the URL. cmdk normalizes item values —
  keys are lowercased and navigation resolves through a unified
  `entries` list (recents get a `recent:` value prefix so the same doc
  in the results below keeps its own key). The empty query leads with
  recently-opened docs from `src/lib/recentDocs.ts` (localStorage
  `docz:recent-docs`, cap 8, coordinates+title ONLY — never tokens;
  reads are segment-validated and malformed payloads reset the store;
  the reader records entries on successful load). The highlighted hit
  prefetches getDoc. Snippets render ONLY through
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
- Changing anything under `charts/` requires bumping `Chart.yaml`'s
  `version` in the SAME PR. `ghcr.yml`'s chart job reads that value and
  runs a `helm pull` idempotency precheck, so a chart change without a
  bump publishes NOTHING and the run is still all-green — the only
  signal is `SLSA provenance (chart)` showing `skipped`. `appVersion`
  tracks the app release and must be BARE semver (metadata-action
  strips the `v`); `tests/deployment_test.yaml` pins the image tag to
  it, so the two move together. `charts/` is in `.prettierignore` — do
  not reformat those files with a JSON/YAML round-trip.
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
