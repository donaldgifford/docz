# CLAUDE.md: `internal/mcp`

Loaded when a session works under this directory. The repository's map is the root `CLAUDE.md`.

## MCP server (DESIGN-0023 / IMPL-0026)

`internal/mcp` is the read-only MCP server, mounted at `POST /mcp` by
`mountMCP` (`cmd/docz-api/mcp.go`) when `MCP_ENABLED`; off, the route is
the router's 404. It is the Go SDK (`github.com/modelcontextprotocol/go-sdk`
v1.8.0, imported as `sdk`) with `Implementation{Name: "docz-api"}`,
`SupportedProtocolVersions` narrowed to **`2026-07-28`** (`mcp.ProtocolVersion`),
`SetCacheable` giving `server/discover` and every list a `ttlMs` of an
hour (a `resources/read` stays stale), and `Handler()` the
`StreamableHTTPHandler` with `Stateless: true`, so `GET` and `DELETE` are
the SDK's `405` and no session id is read or set. The order on the route
is the gate (`session.Authn` then `authorize.Middleware`), then the
limiter, then the SDK: an unauthenticated `POST` is the `401` challenge
before the SDK sees a byte, pinned by `TestHTTPUnauthenticatedIsChallengedBeforeTheSDK`.

- **The tool contract rule** (IMPL-0026 question 6): every tool reads
  through `httpapi.StoreReader` and `httpapi.Searcher`, resolves a type
  through `httpapi.ResolveType`, and builds its output from the exported
  DTO builders (`ToRepoSummary`, `ToTypeDTOs`, `ToDocumentSummary`,
  `ToDocument`, `ToPage`, `ToRepoIndex`, `ToRepoChangelog`), so a tool's
  `structuredContent` is the REST response's JSON and the two surfaces
  cannot drift. `authorize.FromContext` gates every repository lookup
  (`tools.repo`): a missing repository and one outside the set are the
  same `repo not found`. Errors are `isError` in the REST wording
  (`repo not found`, `document not found`, `type not found`, `page not
  found`, `index not found`, `changelog not found`, `search unavailable`,
  `internal error`); a store error's text is logged and never returned.
  Every result carries one `text` block beside `structuredContent`: the
  markdown for `get_doc` and `get_page`, a line per item otherwise.
  Annotations are `readOnlyHint`, `idempotentHint`, `openWorldHint: false`
  on all six (`list_repos`, `list_docs`, `get_doc`, `search_docs`,
  `get_impl_progress`, `get_page`). `get_doc` resolves the type from the
  id's prefix as `repo.Find` does and refuses a row of another type;
  `search_docs` passes the caller's allowed set and turns a nil set into
  an empty one, because nil is the search layer's "unscoped" convenience;
  `get_page` takes `index`, `changelog`, or a path `httpapi.ValidPagePath`
  accepts.
- **Typed fields** (`fields.go`): `Fields(typeName, content)` is the one
  type-name switch in the server, the six canonical names to their
  packages' `Parse` wrapped as `{Kind string; <type>.Doc}` so the Doc's
  fields are promoted under `kind` with the Go field names the packages
  publish; anything else, or a parse failure, is `GenericFields{kind:
  generic, headings, regions, inferred, parse_error}` over
  `docparse.Headings` and `kinds.ResolveRegions` (the type's own spec
  after a failed parse, markers only for a custom type).
  `TestFieldsMatchCorpora` pins the fields equal to `Parse` over every
  `.md` under each `pkg/<type>/testdata`. `get_impl_progress` counts as
  `impl.Doc.Progress` does (a skipped task in neither count) and adds a
  per-phase `deferred` count.
- **Resources** (`resources.go`): one `docz://{owner}/{name}/<type>/{id}`
  template per built-in plus the generic `{type}` one; all seven share
  `readResource`, which returns `text/markdown` and `application/json`
  contents or `sdk.ResourceNotFoundError`. The URI's type must be the
  document's, by name or by a token `ResolveType` folds to it.
- **The limiter** (`limit.go`): one `httprate.RateLimiter` at
  `MCP_RATE_LIMIT` per minute serves both surfaces, `Limiter.MCP` on
  `/mcp` and `Limiter.API` inside the `/api/v1` gate for `Via == token`
  only, so a token caller's two surfaces draw on one budget and the
  cookie path is never limited. `429` with `Retry-After` (httprate sets
  it), body `{"error":"rate limited"}`, and
  `docz_api_rate_limited_total{surface,via}`. **GOTCHA — both of
  httprate's IP key functions are deprecated**: `KeyByRealIP` trusts a
  forgeable header and `KeyByIP` collapses everyone behind a proxy; the
  anonymous key is the connection peer (`clientIP`), the one a client
  cannot choose, which under `AUTH_PROVIDERS=none` behind an ingress is
  one shared bucket. The IMPL named `KeyByRealIP`; the linter refuses it.
- **Observability** (`audit.go`): `observe` is an SDK receiving
  middleware, not HTTP middleware, because the outcome a tool reports
  (`isError`) is not an HTTP status. It opens `mcp.<method>` under the
  request span, or under the `traceparent`/`tracestate` the call's
  `_meta` carries when the HTTP headers carried none; meters
  `docz_api_mcp_calls_total{tool,status}` and
  `docz_api_mcp_call_duration_seconds{tool}` with `tool` the `Mcp-Name`
  header, else the body's tool name, else the method; and writes the
  `mcp call` line at Info for `tools/call` and `resources/read` with
  `principal`, `via`, `client_id`, `issuer`, `scopes`, `tool`, `repo`,
  `doc_id`, `status`, `duration`, reading only `repo` and `id` from the
  arguments, so a search query never reaches a log.
- **The SDK keeps context values**: in stateless mode the handler's
  context derives from the HTTP request's through jsonrpc2's `notDone`
  wrapper (values kept, cancellation dropped), so `session.FromContext`
  and `authorize.FromContext` work inside a tool; the in-memory tests
  pass them on the context given to `Server.Connect`, through
  `session.ContextWithSession` and `authorize.ContextWithAllowed`.
  **GOTCHA — a hand-rolled 2026-07-28 request** needs the
  `Mcp-Protocol-Version` and `Mcp-Method` headers and the
  `io.modelcontextprotocol/{protocolVersion,clientInfo,clientCapabilities}`
  `_meta` keys, or the handler answers `400` before any handler runs;
  `http_test.go`'s `discover` constant is the minimal body.
  `jsonschema-go` infers a tool's schemas from the struct tags: a field
  is required unless `omitempty`, `any` is unrestricted (how `fields`
  rides along), and an embedded struct's fields are promoted.
