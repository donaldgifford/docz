---
id: IMPL-0026
title: "A read-only MCP server in docz-api: tokens, scoped access, and the six tools"
status: Draft
author: Donald Gifford
created: 2026-10-17
---

<!-- markdownlint-disable-file MD024 MD025 MD041 -->

# IMPL-0026: A read-only MCP server in docz-api: tokens, scoped access, and the six tools

<!--toc:start-->
<!-- markdownlint-capture -->
<!-- markdownlint-disable MD013 -->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: The access: block in pkg/doczcore/config](#phase-1-the-access-block-in-pkgdoczcoreconfig)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2: Server storage and configuration](#phase-2-server-storage-and-configuration)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 3: One authn, scoped authorize, and the token verifier](#phase-3-one-authn-scoped-authorize-and-the-token-verifier)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 4: The authorization server and the consent page](#phase-4-the-authorization-server-and-the-consent-page)
    - [Tasks](#tasks-3)
    - [Success Criteria](#success-criteria-3)
  - [Phase 5: The MCP server](#phase-5-the-mcp-server)
    - [Tasks](#tasks-4)
    - [Success Criteria](#success-criteria-4)
  - [Phase 6: Integration tests, chart, runbook, and release](#phase-6-integration-tests-chart-runbook-and-release)
    - [Tasks](#tasks-5)
    - [Success Criteria](#success-criteria-5)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Dependencies](#dependencies)
- [Open Questions](#open-questions)
  - [1. How is the work split into pull requests?](#1-how-is-the-work-split-into-pull-requests)
  - [2. How does it release?](#2-how-does-it-release)
  - [3. How does an operator make the signing key?](#3-how-does-an-operator-make-the-signing-key)
  - [4. Does /auth/login need return_to?](#4-does-authlogin-need-return_to)
  - [5. When are a cookie session's scopes computed?](#5-when-are-a-cookie-sessions-scopes-computed)
  - [6. Where do the tools get their store and search interfaces?](#6-where-do-the-tools-get-their-store-and-search-interfaces)
  - [7. What is the anonymous principal's rate-limit key under AUTH_PROVIDERS=none?](#7-what-is-the-anonymous-principals-rate-limit-key-under-auth_providersnone)
  - [8. Which issuer proves the trusted-issuer path live?](#8-which-issuer-proves-the-trusted-issuer-path-live)
- [References](#references)
<!-- markdownlint-restore -->
<!--toc:end-->

<!--docz:objective:start-->
## Objective

Build the read-only MCP server [DESIGN-0023] decided: `/mcp` in the
docz-api binary on the stateless 2026-07-28 revision, JWT access tokens
from docz-api's own key or any trusted issuer, consent on docz-site, scoped
access declared by each repository's `.docz.yaml`, six read tools with
typed fields, and the chart, runbook, and release that let an operator turn
it on. Six phases, each shipping green on its own: the library's `access:`
block, the server's storage and configuration, the authentication and
authorization middleware, the authorization server, the MCP server, and
the deploy and release work.

**Implements:** [DESIGN-0023](../design/0023-a-read-only-mcp-server-in-docz-api-oauth-21-tokens-tools-and.md)
(its fourteen questions resolved 2026-10-17), which concluded
[INV-0024](../investigation/0024-an-mcp-server-for-docz-api-with-oauth-and-token-auth.md).

<!--docz:objective:end-->

<!--docz:scope:start-->
## Scope

<!--docz:in-scope:start-->
### In Scope

- `pkg/doczcore/config`: the `access:` block (`AccessConfig{Scopes}`),
  the scope grammar, normalisation and validation on both `Load` paths,
  the `docz init` block, and the parity normaliser for it (DESIGN-0023 §7,
  API changes)
- `internal/store`: the `add_oauth` migration (`repos.access_scopes`,
  `oauth_refresh_tokens`, `oauth_consents`) and its sqlc queries (Data
  Model)
- `internal/config`: `MCPConfig`, `OAuthConfig`, `AuthzConfig` and their
  validation; the `envKeys` pin in `config_test.go`
- `internal/ingest`: `access.scopes` mapped onto the repository row
- `internal/oauth` (new): the token verifier for docz-api's key and every
  trusted issuer; the authorization server (metadata documents, JWKS,
  `/oauth/authorize`, `/oauth/token`, `/oauth/revoke`, the two consent
  endpoints, the Client ID Metadata Document fetcher) (§5)
- `internal/session`, `cmd/docz-api`: `Session{Via, ClientID, Scopes}` and
  the one `authn` middleware (§6)
- `internal/authorize`: `ScopeAuthorizer` with its cache; `AllReposAuthorizer`
  kept for `AUTH_PROVIDERS=none` (§7)
- `internal/export`: the scope refusal (§7)
- `internal/mcp` (new): the Go SDK server, the six tools, the typed-field
  switch, the `docz://` resource templates, the rate limiter, the audit
  line, metrics, and traces (§2, §3, §4, §8)
- `api/openapi.yaml` to `1.7.0`, `api/README.md`'s "Tokens" section, and
  docz-site's `/oauth/consent` route
- `charts/docz`: `api.mcp.*`, `api.authz.*`, the `oauth-signing-key`
  Secret key, the `DoczAPITokenFailures` alert; `contrib/prometheus`
- RUNBOOK-0004, `CLAUDE.md`, `deploy/README.md`, the CHANGELOG's tool
  contract, and the homelab live check

<!--docz:in-scope:end-->

<!--docz:out-of-scope:start-->
### Out of Scope

- Any tool or scope with a `write` action; MRTR; the Tasks extension;
  `subscriptions/listen`; prompts (DESIGN-0023 Non-Goals)
- Client credentials, pre-registered clients, personal access tokens, and
  admin flags on the binary (DESIGN-0023 questions 5 and 6)
- Dynamic Client Registration
- Advertising the trusted external issuers in the PRM's
  `authorization_servers`; a shared rate limiter in Redis; per-document
  authorization; organisation membership looked up from GitHub
- Changes to the login providers themselves

<!--docz:out-of-scope:end-->
<!--docz:scope:end-->

## Implementation Phases

Each phase builds on the previous one. A phase is complete when all its tasks
are checked off and its success criteria are met.

---

<!--docz:phase:start-->
### Phase 1: The `access:` block in `pkg/doczcore/config`

After this phase a `.docz.yaml` can say which scopes unlock the
repository, `docz init` writes the block with its default, and the parity
suite knows to ignore it. Nothing reads it yet; the server maps it in
Phase 2. This is a `minor` on the stable v2 line and can ship alone.

<!--docz:tasks:start-->
#### Tasks

- [ ] `pkg/doczcore/config/access.go`: `AccessConfig struct{ Scopes
  []string }` (`yaml:"scopes" json:"scopes"`) and `Config.Access
  AccessConfig` (`yaml:"access" json:"access"`); `DefaultConfig()` sets
  `Scopes` to `[]string{"public:read"}`; the exported constants
  `ScopePublicRead = "public:read"`, `ActionRead = "read"`, and
  `ErrInvalidAccess = errors.New("invalid access config")`
- [ ] `normalizeAccess`, on both `Load` paths after `normalizeSync` and
  in `parseBytes`: trims each entry, folds it to lower case, backfills an
  empty or absent list to `[public:read]`, and removes exact duplicates
  after folding. It never rejects
- [ ] `validateAccess`, called from `Validate()` unconditionally (the block
  has a default, so it is never dormant): each entry is
  `<audience>:<action>` with the audience matching
  `^[a-z0-9][a-z0-9-]{0,62}$` and the action `read`; `write` is rejected
  with a message saying it is reserved; a duplicate after folding is
  rejected; a bare `public` with no action is rejected naming the fix. The
  error is bare and wrapped with `ErrInvalidAccess` by the caller, as
  `paths.go` does
- [ ] `access_test.go`: one case per rule above, the default on an absent
  block, an explicitly empty `scopes: []` backfilled, `ParseBytes` equal
  to both `Load` paths, and `t.Parallel()` on every test
- [ ] `json_test.go`: `TestConfigJSON_MarshaledShape` gains the `access`
  object; `TestJSONTags_MirrorYAML` passes unedited
- [ ] `doctemplate/templates/docz_yaml.tmpl`: a top-level `access:` block
  after `sync:` with `scopes: [public:read]` and a comment saying what a
  scope is, that `public:read` means every signed-in reader of docz-api,
  and that a team audience such as `platform-team:read` hides the
  repository from everyone else; `DefaultConfigYAML()` renders it
- [ ] `test/parity/parity.go`: an `AccessNormalizer()` named `access`
  beside `SyncNormalizer`, dropping the `access:` block with its comments
  from the generated `.docz.yaml` on both sides, applied in `runCase` with
  the other both-sides normalisers; a unit test for it outside the build
  tag, as the others have
- [ ] `cmd/config.go` prints the block as it prints `sync:`; `docz config`
  golden regenerated with `-update`
- [ ] `test/consumer/consumer_v2_test.go` (or a sibling): one inline
  manifest with `access.scopes: [platform-team:read]` parsed through
  `config.ParseBytes`, proving the field is reachable from outside
- [ ] `CLAUDE.md`'s `pkg/doczcore/config` bullet gains the `access:` block,
  its grammar, and the reserved `write`

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `just test`, `just test-consumer`, `just parity`, and `just
  lint-generated` pass
- A manifest with `scopes: [impl:write]` fails `Validate` naming the
  reserved action; one with `scopes: [Platform-Team:read]` loads as
  `platform-team:read`; one without the block loads as `[public:read]`
- `docz init` in a temp directory writes the block, and `docz config`
  round-trips it

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 2: Server storage and configuration

After this phase the server stores what the design needs and refuses a
misconfiguration at boot, and every ingested repository carries its scope
list. Nothing reads a token yet. A deployment that upgrades to this phase
gets the migration and nothing else, because `MCP_ENABLED` is unset.

<!--docz:tasks:start-->
#### Tasks

- [ ] Migration `20261017000000_add_oauth.sql`: `ALTER TABLE repos ADD
  COLUMN access_scopes JSONB NOT NULL DEFAULT '["public:read"]'` with a
  GIN index; `oauth_refresh_tokens` and `oauth_consents` as DESIGN-0023's
  Data Model draws them (hashed token, family id, user and client,
  scope, issued, last used, revoked; the consents unique on `(user_id,
  client_id)`); a `-- +goose Down` that drops all three
- [ ] `sqlc.yaml` and `internal/store/queries/`: `UpsertRepo` takes
  `access_scopes`; `ListRepoIDsForScopes(scopes text[])` as `SELECT id
  FROM repos WHERE access_scopes ?| @scopes`; `InsertRefreshToken`,
  `RotateRefreshToken` (one statement marking the old row used and
  inserting the new), `GetRefreshTokenByHash`, `RevokeRefreshFamily`,
  `UpsertConsent`, `GetConsent`; `just api generate` and
  `generate-check` clean
- [ ] `internal/store`: `RepoInput.AccessScopes []string` marshalled to
  JSONB; `ReconcileRepo` writes it; the integration test
  (`//go:build integration`) round-trips the column and the two tables,
  and runs the migration up and down
- [ ] `internal/ingest/service.go`: `AccessScopes: cfg.Access.Scopes` on
  the `RepoInput`; `mapper_test.go` covers the default and a team scope
- [ ] `internal/config`: `MCPConfig{Enabled bool, RateLimit int}`,
  `OAuthConfig{Issuer string, SigningKey Secret, TrustedIssuers []string,
  AccessTTL, RefreshTTL time.Duration}`, `AuthzConfig{ScopeMap
  map[string][]string, CacheTTL time.Duration}` read from `MCP_ENABLED`,
  `MCP_RATE_LIMIT`, `OAUTH_ISSUER`, `OAUTH_SIGNING_KEY`,
  `OAUTH_TRUSTED_ISSUERS`, `OAUTH_ACCESS_TTL`, `OAUTH_REFRESH_TTL`,
  `DOCZ_SITE_URL`, `AUTHZ_SCOPE_MAP`, `AUTHZ_CACHE_TTL`, with the defaults
  in DESIGN-0023's configuration table
- [ ] Validation, every problem in the one `ErrInvalidConfig`: `OAUTH_ISSUER`
  defaults to `AUTH_REDIRECT_BASE`, must be `https` with a host and no
  query or fragment, and is required when MCP is enabled; each trusted
  issuer the same rule with a path allowed; `OAUTH_SIGNING_KEY` and
  `DOCZ_SITE_URL` required when MCP is enabled and `AUTH_PROVIDERS` is not
  `none`; the signing key must parse as an EC P-256 private key (PEM, or a
  path to one, as `GITHUB_APP_PRIVATE_KEY` accepts); `AUTHZ_SCOPE_MAP`
  parsed from `provider:group:<name>=<scope>[,<scope>];…` and
  `provider:login:<name>=…`, every scope checked against the Phase 1
  grammar through `config.ValidateScope` (exported for this), a `public`
  audience or a `write` action rejected; `MCP_RATE_LIMIT` positive
- [ ] `config_test.go`: the new variables added to `envKeys` so
  `TestEnvKeysCoverLoad` passes; a case per validation rule; the defaults
  pinned
- [ ] `.env.example`, `deploy/.env.production.example`, and
  `deploy/README.md` list the variables, commented out

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `just api test`, `just api lint`, and `just api test-integration` pass
- A fresh database migrates up and down; an existing row reads
  `["public:read"]` after the migration
- `MCP_ENABLED=true` with no signing key fails boot with one
  `ErrInvalidConfig` naming every missing variable; with `MCP_ENABLED`
  unset the server starts exactly as before

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 3: One `authn`, scoped `authorize`, and the token verifier

After this phase `/api/v1` accepts a JWT from docz-api's key or any
trusted issuer, every request carries a principal with scopes, and a
repository outside a principal's scopes is not found. Every existing
deployment behaves as before, because every repository is `public:read`
and every principal holds it. This is DESIGN-0023 Rollout step 3 and is
observable through the session response.

<!--docz:tasks:start-->
#### Tasks

- [ ] `internal/oauth/verify.go`: `Verifier` built from the signing key's
  public half (`kid` = the JWK's RFC 7638 thumbprint) and the trusted
  issuers, each discovered at startup under `oidcDiscoveryTimeout` (a bad
  issuer fails boot, as a login provider does) with its JWKS cached in
  Redis under `jwks:<sha256(issuer)>` for an hour and refreshed on an
  unknown `kid` at most once per minute; `Verify(ctx, raw) (Claims, error)`
  checks the signature, `iss` against the set, `aud` contains
  `<OAUTH_ISSUER>/mcp`, `exp` and `nbf` with 30 s skew, and reads `scope`
  (space-separated) or `scp` (array). `Claims{Issuer, Subject, ClientID,
  Scopes, Email, Login, Expiry}`
- [ ] `internal/session`: `Session` gains `Via SessionVia` (`cookie`,
  `token`, `anonymous`, with a `String()`), `ClientID`, `Issuer`, and
  `Scopes []string`; `sessionData` stores `groups` already and gains
  nothing, because scopes for a cookie session are computed per request
  (question 5)
- [ ] `internal/authorize/scopes.go`: `ScopesFor(identity, scopeMap)
  []string`: `public:read` plus every scope the map yields for
  `<provider>:group:<g>` over `Identity.Groups` and for
  `<provider>:login:<login>`, sorted and de-duplicated; a table test
- [ ] `cmd/docz-api/main.go`: `authn` becomes `session.Authn(store,
  verifier, scopeMap, anonymous bool)` in `internal/session/authn.go`:
  bearer first (`401` with `WWW-Authenticate: Bearer
  resource_metadata="<issuer>/.well-known/oauth-protected-resource",
  scope="public:read"` on any failure, never a fall-through), then the
  cookie through the existing `Lookup` with its `401`/`503` split, then
  the anonymous session under `none`; the challenge header on every `401`
  the middleware writes; a verifier of nil means no bearer path (MCP
  off). A table test over every edge of DESIGN-0023 §6's flowchart with
  a fake verifier and a fake lookup
- [ ] `internal/authorize`: `ScopeAuthorizer{lister, cache}` whose
  `Allowed` reads `session.FromContext`, calls `ListRepoIDsForScopes`
  with the session's scopes, and caches the result per principal key
  (`<issuer>:<sub>` or `user:<provider>:<subject>`) for `AUTHZ_CACHE_TTL`;
  `runServer` wires it, keeping `AllReposAuthorizer` under `none`; unit
  tests for the rule (public, team, no scopes, cache expiry) with a fake
  lister
- [ ] `internal/authhttp`: `/api/v1/auth/session` gains `via`, `scopes`,
  and, for a token, `client_id` and `issuer`; `sessionDTO` and the spec's
  `SessionResponse` updated together; `api/openapi.yaml` `info.version`
  to `1.7.0` with the `bearerToken` security scheme listed on every
  `/api/v1` operation beside `sessionCookie` (the remaining 1.7.0 additions
  land in Phase 4 before any release)
- [ ] `internal/httpapi/openapi_contract_test.go`: every read route with a
  bearer from a test key, a bad bearer's `401` and its challenge, the
  session response for a token caller; `TestNoAuthModeReportsAnonymousIdentity`
  extended with `via: anonymous`
- [ ] `internal/export/service.go`: after the allow-list check, a
  repository whose `access_scopes` contains neither `public:read` nor
  `confluence:write` is `StatusRefused` with the reason `repository
  scopes grant the export nothing; add confluence:write to access.scopes`;
  `ExportInputs` reads the column; a table test with the three cases
- [ ] `internal/telemetry`: `docz_api_token_verifications_total{issuer,status}`,
  recorded in `authn`; `contrib/README.md` lists it
- [ ] `ui/`: `just ui gen-api`; the session hook tolerates the new fields
  (additive, so no change expected); `just ui ci` green

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `just api test`, `just ui ci`, and the contract test pass
- A JWT signed by a test issuer's key with `aud` set to the resource
  reads `/api/v1/repos`; the same token with another `aud`, an expired
  `exp`, or an untrusted `iss` is `401` with the challenge header
- A repository ingested with `access.scopes: [team:read]` is absent from
  `/api/v1/repos` for a `public:read` principal and present for a
  `team:read` one; under `AUTH_PROVIDERS=none` it is present
- The export of a repository with `[team:read]` is recorded `refused`
  with the reason above; adding `confluence:write` makes the next run
  proceed

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 4: The authorization server and the consent page

After this phase an MCP client with a Client ID Metadata Document can get
a token from docz-api by signing in through docz-site, and a script author
can read how from the spec. Everything is mounted only when `MCP_ENABLED`
is true and `AUTH_PROVIDERS` is not `none`.

<!--docz:tasks:start-->
#### Tasks

- [ ] `internal/oauth/metadata.go`: the three documents at
  `/.well-known/oauth-protected-resource`,
  `/.well-known/oauth-authorization-server`, and `/.well-known/jwks.json`
  with the fields DESIGN-0023 §5 lists; `scopes_supported` is
  `public:read` plus every scope the scope map can grant; golden JSON
  tests
- [ ] `internal/oauth/client.go`: the Client ID Metadata Document fetcher:
  `https` only, no redirects, 5 s timeout, 64 KiB cap, a dialer whose
  resolver refuses loopback, link-local, and RFC 1918 and ULA ranges
  (injected for tests), `client_id` equal to the fetched URL, `redirect_uris`
  required; cached in Redis under `cimd:<sha256(url)>` for the response's
  `max-age` capped at an hour; tests with an `httptest` server for each
  refusal
- [ ] `internal/oauth/authorize.go`: `GET /oauth/authorize` validates in
  the order §5 gives (`response_type`, `client_id`, `redirect_uri`
  exact-match, `code_challenge` + `S256`, `resource` equal to the PRM's,
  `scope` a subset of what docz-api can grant with empty meaning
  `public:read`, `state` present), stores the request in Redis under
  `authz:<32 random bytes>` for 10 minutes, and redirects to
  `<DOCZ_SITE_URL>/oauth/consent?authz=<id>`. Pre-trust failures render a
  plain error page; post-trust failures redirect with `error`, `state`,
  and `iss`
- [ ] `internal/oauth/consent.go`, mounted through `httpapi.Handler.Mount`'s
  extras behind the gate: `GET /api/v1/oauth/authorizations/{id}` returns
  `client_name`, `client_uri`, `scopes_requested`, `scopes_granted` (the
  intersection with the session's scopes) and binds the pending request
  to the session id; `POST` with `{"decision": "allow"|"deny"}` from the
  same session mints a code (`code:<32 random bytes>`, 60 s, single use,
  holding the pending request, the user id, and the granted scopes),
  upserts `oauth_consents`, deletes the pending key, and returns
  `{"redirect_to": …}` with `code`, `state`, `iss`, or the `access_denied`
  redirect. A `POST` from another session, a consumed id, or an expired
  one is `404`
- [ ] `internal/oauth/token.go`: `POST /oauth/token` with
  `grant_type=authorization_code` (code, `code_verifier` against the
  stored challenge, `resource`, `client_id`, `redirect_uri` equal to the
  request's; a second redemption revokes the family the first created)
  and `grant_type=refresh_token` (hash lookup, family check, rotation in
  one statement, reuse of a rotated-out token revokes the family, scopes
  recomputed from the user's current groups through the scope map);
  `POST /oauth/revoke` (RFC 7009, the refresh token's family). Responses
  carry `access_token`, `token_type: Bearer`, `expires_in`, `refresh_token`,
  `scope`; errors use the RFC 6749 shape
- [ ] `internal/oauth/issue.go`: `Issue(claims) (string, error)` signs ES256
  with the configured key: `iss`, `sub` (`user:<provider>:<subject>`),
  `aud`, `exp` (`OAUTH_ACCESS_TTL`), `iat`, `jti`, `scope`, `client_id`,
  `email`, `login`; round-trip tests through the Phase 3 verifier
- [ ] `cmd/docz-api/main.go`: mount the three documents and the three
  endpoints on the root router outside both gates when enabled and a login
  exists; the consent endpoints through the gated group
- [ ] `ui/src/routes/consent.tsx` at `/oauth/consent`: behind the session
  gate like every route (a `401` stashes the route in
  `docz:auth:return-to` and lands on `/login`, which is how the person
  comes back after the provider round trip; question 4), reads the
  authorization with the generated client, shows the client name and
  link, the scopes it will get in words, Allow and Deny, and sets
  `window.location` to `redirect_to`; unit tests with MSW for the happy
  path, a denied decision, a `404`, and the unauthenticated stash
- [ ] `api/openapi.yaml`: the two consent endpoints, `/oauth/token`,
  `/oauth/revoke`, and the three well-known documents with
  `additionalProperties: false` schemas and `security: []` where public;
  `just api lint-openapi` at 100; `api/README.md` gains "Tokens" (how a
  script gets one through any MCP client, and how a platform-minted one is
  accepted); `just ui gen-api-check` clean
- [ ] `openapi_contract_test.go`: the metadata documents, the token
  endpoint's two grants against a fake Redis and the real store fakes, the
  consent endpoints, and the revoke
- [ ] `internal/telemetry`: `docz_api_oauth_tokens_total{grant,status}`;
  `oauth token` audit line with `grant`, `client_id`, `principal`, `status`
  and never the token

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `just api test`, `just ui ci`, and `just api lint-openapi` pass
- Against a local stack with the compose Keycloak realm as the login, a
  hand-driven round trip (`curl` for the metadata, a browser for
  `/oauth/authorize` and the consent page, `curl` for `/oauth/token`)
  yields a JWT the Phase 3 verifier accepts on `/api/v1/repos`
- Redeeming a code twice revokes the refresh token it issued; presenting a
  rotated-out refresh token revokes its family
- A metadata document on a private address, over `http`, or behind a
  redirect is refused as `invalid_client`

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 5: The MCP server

After this phase an MCP client reads documentation through `/mcp` with
the six tools and the resource templates, every call is limited, metered,
traced, and logged with its principal, and the tool contract is pinned
against the type packages' corpora.

<!--docz:tasks:start-->
#### Tasks

- [ ] `go.mod`: `github.com/modelcontextprotocol/go-sdk` at `v1.8.0` or
  later as a direct requirement, settled with targeted `go get` and never a
  bare `go mod tidy`; `go-licenses` and the licence check clean;
  `layer_test.go` untouched, because nothing under `pkg/` imports it
- [ ] `internal/mcp/server.go`: `New(deps) *mcp.Server` with
  `Implementation{Name: "docz-api", Version}`,
  `SupportedProtocolVersions` narrowed to `2026-07-28`, `SetCacheable`
  giving every list a `ttlMs` of one hour; `Handler()` returns the SDK's
  `StreamableHTTPHandler` with `Stateless: true`
- [ ] `internal/mcp/tools.go`: `list_repos`, `list_docs`, `get_doc`,
  `search_docs`, `get_impl_progress`, `get_page` with the argument and
  output schemas of DESIGN-0023 §3, `readOnlyHint`, `idempotentHint`, and
  `openWorldHint: false`, each over the `storeReader` and `Searcher`
  interfaces `internal/httpapi` declares (moved to a small shared
  `internal/httpapi/ports.go` or duplicated consumer-side, question 6),
  the caller's `authorize.FromContext` applied to every repository
  lookup, `resolveType` reused, errors as `isError` with the REST wording,
  and one `text` block beside `structuredContent`
- [ ] `internal/mcp/fields.go`: the typed-field switch over the six
  canonical type names to `rfc.Parse`, `adr.Parse`, `design.Parse`,
  `impl.Parse`, `investigation.Parse`, `runbook.Parse`, each marshalled
  under `kind`; a custom type or a parse failure yields `generic` with
  `docparse.Headings` and `kinds.ResolveRegions` and `parse_error` when
  there was one; `get_impl_progress` derives `done`, `total`, and the
  deferred count per phase from `impl.Doc`
- [ ] `internal/mcp/resources.go`: one `docz://{owner}/{name}/{type}/{id}`
  template per built-in type and one generic, `resources/read` returning
  `text/markdown` and `application/json` contents, a URI outside
  `AllowedRepos` not found
- [ ] `internal/mcp/limit.go`: a `rate.Limiter` per principal key kept in
  a map with idle eviction, `MCP_RATE_LIMIT` per minute with an equal
  burst, applied to `/mcp` and to `/api/v1` for `Via == token`, `429`
  with `Retry-After` (the anonymous principal's key per question 7)
- [ ] `internal/mcp/audit.go` and `internal/telemetry`: the `mcp call`
  line with `principal`, `via`, `client_id`, `issuer`, `scopes`, `tool`,
  `repo`, `doc_id`, `status`, `duration`;
  `docz_api_mcp_calls_total{tool,status}` and
  `docz_api_mcp_call_duration_seconds{tool}` labelled from the `Mcp-Name`
  header; a child span `mcp.<method>` carrying the tool name with the
  `traceparent` from `_meta` as parent when the HTTP headers carry none
- [ ] `cmd/docz-api/main.go`: `POST /mcp` mounted behind `authn`, the
  authorizer, and the limiter when `MCP_ENABLED`; the `401` challenge
  verified to fire before the SDK sees the request
- [ ] Tests: the SDK's in-memory transport for `server/discover`,
  `tools/list`, each tool over the `httpapi` fakes, `resources/list`, and
  `resources/read`; `get_doc.fields` pinned equal to each type package's
  `Parse` over every `.md` under `pkg/<type>/testdata`; the `generic`
  fallback; a conformance test with the SDK's Streamable HTTP client
  against the full `runServer` stack for the headers, the `401`, the
  `405`s, the limiter's `429`, and a tool call through real middleware;
  the audit line and the metric label asserted from a captured slog
  handler
- [ ] `CLAUDE.md`: a "MCP server (DESIGN-0023 / IMPL-0026)" section in
  the server part naming the packages, the tool contract rule, and the
  gotchas met

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `just api test` and `just ci` pass; `go test ./internal/mcp/...
  -run Fields` agrees with every type package corpus
- Claude Code (`claude mcp add --transport http docz http://localhost:8080/mcp`)
  against a local stack lists the six tools, reads a document with its
  typed fields, and shows search hits; with `MCP_ENABLED` unset the route
  is `404`
- A burst past `MCP_RATE_LIMIT` is `429` with `Retry-After`, and the audit
  line for every call names the principal and the tool and never the
  query text

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 6: Integration tests, chart, runbook, and release

After this phase an operator can turn the feature on from the chart,
follow a runbook to connect a client and trust an issuer, and the release
carries the tool contract. The integration suite proves the whole flow
over real Postgres and Redis before the homelab sees it.

<!--docz:tasks:start-->
#### Tasks

- [ ] `internal/e2e/mcp_integration_test.go` (`//go:build integration`):
  the code flow with a fake provider and an `httptest` consent page that
  plays docz-site; a `none` deployment needing no token; a repository
  ingested with `[team:read]` invisible to `public:read` and visible to
  `team:read`; a token from a fake trusted issuer (an `httptest` discovery
  and JWKS) reading with the scopes it carries; refresh reuse revoking a
  family; the export refusing and then accepting a private repository
- [ ] `charts/docz`: `api.mcp.{enabled, issuer, siteUrl, trustedIssuers,
  rateLimit}` and `api.authz.{scopeMap, cacheTtl}` rendered only while
  `enabled`, with `issuer` defaulting to the API's external URL from its
  ingress or route host and `siteUrl` to the site's; `oauth-signing-key`
  in the API Secret, `required` while enabled, through the
  `existingSecret` seam; `values.schema.json` typed; `DoczAPITokenFailures`
  in the PrometheusRule and `contrib/prometheus/alerts.yaml`
  (`just lint-alerts`); helm-unittest cases for disabled (nothing
  rendered), enabled (every variable), and the Secret key; the README
  regenerated with `just chart docs`; chart version bumped
- [ ] `docs/runbook/0004-enable-mcp-on-docz-api.md` (`docz create
  runbook`): generate the signing key with `openssl ecparam -name
  prime256v1 -genkey -noout` (question 3), set the issuer and the site
  URL, turn the flag on, trust the platform's issuer, map a group to a
  scope, mark a repository private, connect Claude Code, mint a token from
  the platform for an agent and call `/api/v1/repos` with it, read the
  three metrics, and the three failure modes (a refused metadata document,
  a wrong audience, a principal whose scopes unlock nothing), with a
  rollback procedure (turn the flag off) and a filled Last Verified row
- [ ] `deploy/README.md` and `CLAUDE.md`'s chart section updated;
  `contrib/README.md` lists the new metrics
- [ ] The homelab deployment turned on with GitHub login and the homelab
  Keycloak as a trusted issuer (question 8); the runbook followed end to
  end; what it taught written back into the runbook
- [ ] `CHANGELOG.md` carries the tool contract under `mcp` and the
  `access:` block under `config`; the release PR labelled `minor`
  (question 2); INV-0024 and DESIGN-0023 already point at each other,
  DESIGN-0023's status flipped to `Implemented` in the release PR

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `just api test-integration` passes the new suite; `just chart lint`,
  `just chart unittest`, and `ct lint` pass
- `helm template` with `api.mcp.enabled: false` renders no `MCP_*` or
  `OAUTH_*` variable and no Secret key; with it enabled and the key unset,
  the render fails naming `oauth-signing-key`
- Claude Code connects to the homelab deployment through the consent page
  and reads a document; a Keycloak-minted token reads `/api/v1/repos`
- The release is cut by label, the images and chart publish, and
  RUNBOOK-0004's Last Verified row names the commit

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:file-changes:start-->
## File Changes

| File | Action | Description |
| ---- | ------ | ----------- |
| `pkg/doczcore/config/access.go`, `access_test.go` | Create | `AccessConfig`, the scope grammar, normalise and validate (Phase 1) |
| `pkg/doczcore/config/config.go`, `json_test.go` | Modify | `Config.Access`, the default, the marshalled shape |
| `pkg/doczcore/doctemplate/templates/docz_yaml.tmpl` | Modify | The `access:` block `docz init` writes |
| `test/parity/parity.go` | Modify | The `access` normaliser |
| `internal/store/migrations/20261017000000_add_oauth.sql` | Create | `repos.access_scopes`, `oauth_refresh_tokens`, `oauth_consents` (Phase 2) |
| `internal/store/queries/*.sql`, generated code | Modify | The queries listed in Phase 2 |
| `internal/ingest/service.go` | Modify | `AccessScopes` on the repository input |
| `internal/config/config.go`, `config_test.go` | Modify | `MCPConfig`, `OAuthConfig`, `AuthzConfig`, `envKeys` |
| `internal/oauth/{verify,metadata,client,authorize,consent,token,issue}.go` | Create | The verifier and the authorization server (Phases 3 and 4) |
| `internal/session/authn.go`, `session.go` | Create, Modify | The one middleware; `Via`, `ClientID`, `Issuer`, `Scopes` |
| `internal/authorize/scopes.go`, `authorize.go` | Create, Modify | `ScopesFor`, `ScopeAuthorizer` with its cache |
| `internal/authhttp/handler.go` | Modify | The session response fields |
| `internal/export/service.go` | Modify | The scope refusal |
| `internal/mcp/{server,tools,fields,resources,limit,audit}.go` | Create | The MCP server (Phase 5) |
| `internal/telemetry/metrics.go`, `middleware.go` | Modify | Token, OAuth, and MCP metrics; the `mcp.<method>` span |
| `cmd/docz-api/main.go` | Modify | Wiring behind `MCP_ENABLED` |
| `api/openapi.yaml`, `api/README.md` | Modify | `1.7.0`: bearer scheme, session fields, consent, token, well-known |
| `ui/src/routes/consent.tsx`, `consent.test.tsx`, `ui/src/app/router.tsx` | Create, Modify | The consent page (Phase 4) |
| `internal/e2e/mcp_integration_test.go` | Create | The integration suite (Phase 6) |
| `charts/docz/values.yaml`, `templates/api-*.yaml`, `tests/`, `README.md` | Modify | `api.mcp.*`, `api.authz.*`, the Secret key, the alert |
| `contrib/prometheus/alerts.yaml`, `contrib/README.md` | Modify | `DoczAPITokenFailures`; the metrics |
| `docs/runbook/0004-enable-mcp-on-docz-api.md` | Create | RUNBOOK-0004 |
| `CLAUDE.md`, `deploy/README.md`, `.env.example`, `CHANGELOG.md` | Modify | Conventions, variables, the tool contract |

<!--docz:file-changes:end-->

<!--docz:testing:start-->
## Testing Plan

- [ ] Phase 1: table tests for the grammar; `ParseBytes` equal to both
  `Load` paths; the json shape; the parity normaliser's unit test and
  `just parity`; `just lint-generated` over the scaffold with the new block
- [ ] Phase 2: store integration round trips and the migration down;
  config validation cases and the `envKeys` pin; the mapper's default and
  team scope
- [ ] Phase 3: the verifier against a test key and a fake issuer (signature,
  `iss`, `aud`, `exp`, `nbf`, unknown `kid` refresh, `scope` and `scp`);
  the `authn` flowchart as a table; the scope rule and cache; the export
  refusal table; the contract test with bearer tokens
- [ ] Phase 4: the metadata goldens; every authorization refusal by name;
  the fetcher's SSRF refusals through an injected resolver; code
  single-use and PKCE; rotation and family revocation; the consent
  binding; MSW tests for the consent route; the contract test for every
  new endpoint
- [ ] Phase 5: the in-memory transport for every tool and resource;
  `fields` pinned to the type corpora; the conformance test over the full
  stack; the limiter; the audit line and metric label
- [ ] Phase 6: the integration suite over real Postgres and Redis;
  helm-unittest; the hand-driven live check with Claude Code and a
  Keycloak-minted token, recorded in RUNBOOK-0004

<!--docz:testing:end-->

<!--docz:dependencies:start-->
## Dependencies

- `github.com/modelcontextprotocol/go-sdk` v1.8.0 or later (2026-07-28
  support, `Stateless`, `SupportedProtocolVersions`, `SetCacheable`), a
  new direct dependency in Phase 5
- `github.com/go-jose/go-jose/v4` for JWS, JWK, and JWKS (already an
  indirect dependency through go-oidc; promoted to direct in Phase 3,
  question 4)
- `golang.org/x/time/rate` for the limiter (already indirect)
- A Keycloak realm for the live checks: the compose stack's
  `--profile auth` realm locally, and the homelab's for Phase 6
- Phases 1 and 2 have no dependency on each other beyond the config
  field's name; Phases 3 to 6 are sequential
- Unreleased work on `main` (#185 ADR-0005, #186 inline-comment anchors)
  rides the same `minor` if it has not shipped first

<!--docz:dependencies:end-->

<!--docz:open-questions:start-->
## Open Questions

### 1. How is the work split into pull requests?

- a. **One PR per phase, each branched from `main` after the previous one
  merges, all labelled `dont-release` until the last.** *(recommendation)*
  Phase 1 is a stable-line library change that deserves its own review;
  Phases 3 and 4 each touch the authentication path and are safest read
  alone; nothing is stacked, so the auto-close problem never comes up.
- b. Three PRs: the library (Phase 1), the server's auth (Phases 2 to 4),
  and the MCP server with its rollout (Phases 5 and 6). Fewer merges,
  larger reviews.
- c. One PR for everything, as IMPL-0024 did. One review at the end, the
  largest diff this repository has had since the move-ins.
- d. Other.

### 2. How does it release?

- a. **One `minor` after Phase 6, carrying the whole feature and whatever
  is unreleased on `main` (#185, #186).** *(recommendation)* The `access:`
  block is inert without the server reading it, and a release note that
  says "MCP" should point at a working server.
- b. A `minor` after Phase 1 for the library alone, then another after
  Phase 6. Lets a repository write `access.scopes` early, and gives
  #185 and #186 a release sooner, at the cost of a second release cycle.
- c. Other.

### 3. How does an operator make the signing key?

- a. **An `openssl` command in the runbook and the chart's README:
  `openssl ecparam -name prime256v1 -genkey -noout`, stored in the API
  Secret under `oauth-signing-key`.** *(recommendation)* No new flag on
  the binary, nothing to maintain, and the key is made where the secret
  manager wants it made.
- b. A `-generate-signing-key` flag on `docz-api` that prints a PEM. One
  less tool to have, and one more flag on a binary the design just took
  the admin flags off.
- c. Generate at first boot and store in Postgres. Nothing for the
  operator to do, and a private key in the database that the design
  rejected.
- d. Other.

### 4. Does `/auth/login` need `return_to`?

- a. **No. docz-site already stashes the route it was on in
  `sessionStorage` (`docz:auth:return-to`) before sending the person to
  `/auth/login`, and restores it when the callback lands on `/`; the
  consent route uses that path like every other route.**
  *(recommendation)* No API change, no new input to validate, and
  DESIGN-0023 §5's `return_to` is struck in an amendment note.
- b. Add `return_to` as DESIGN-0023 wrote it, a path signed into the
  state, so the hand-off survives a browser with `sessionStorage`
  disabled or a consent link opened in a fresh profile.
- c. Both: the site's stash first, `return_to` as the fallback.
- d. Other.

### 5. When are a cookie session's scopes computed?

- a. **Per request, from the groups and login stored in the session
  record and the current scope map.** *(recommendation)* The map is
  environment and changes only on restart, so a mapping change is live
  the moment the process is; the computation is a map lookup per group,
  and nothing new is stored. DESIGN-0023 §7 said "once per cookie session"
  and is amended.
- b. Once at login, stored in the session record. One computation per
  session, and a mapping change waits for the next login, up to
  `SESSION_TTL` (30 days) away.
- c. Other.

### 6. Where do the tools get their store and search interfaces?

- a. **Consumer-side, in `internal/mcp`: a `storeReader` and a `Searcher`
  interface declared for the methods the tools use, satisfied by
  `*store.Store` and `*search.Client`.** *(recommendation)* The house
  rule: each consumer declares the interface it needs, as `httpapi`,
  `ingest`, and `webhook` do; a tool that needs one more method adds it
  without touching `httpapi`.
- b. Export `httpapi`'s interfaces and reuse them. One definition, and
  `mcp` then imports `httpapi` for two interfaces and `resolveType`.
- c. Other.

### 7. What is the anonymous principal's rate-limit key under `AUTH_PROVIDERS=none`?

- a. **The client IP (the request's `RemoteAddr`, or
  `X-Forwarded-For`'s first hop when `HTTP_TRUSTED_PROXIES` names the
  peer).** *(recommendation)* One bucket for every anonymous caller would
  let one runaway agent throttle the whole first-setup deployment; per IP
  keeps the guard meaningful where there is no identity.
- b. No limit under `none`. The mode already means "anyone on the
  network reads everything", and the limiter is a guard on identified
  callers.
- c. One shared bucket for the anonymous principal. Simplest, and one
  caller can exhaust it for all.
- d. Other.

### 8. Which issuer proves the trusted-issuer path live?

- a. **The compose stack's Keycloak (`deploy/compose.monitoring.yaml
  --profile auth`, realm `docz-api`) for Phases 4 and 5, and the homelab's
  Keycloak for Phase 6.** *(recommendation)* The local realm already has
  a confidential client and a verified user; a client-credentials client
  added to the realm JSON mints the agent token the runbook describes.
- b. OpenBao's OIDC provider, locally. Closer to the platform Donald
  described, and a new service to stand up in the compose stack first.
- c. The local realm only; the homelab check uses GitHub login and
  docz-api's own tokens, with the external issuer untested outside the
  integration suite's fake.
- d. Other.

<!--docz:open-questions:end-->

<!--docz:references:start-->
## References

- [DESIGN-0023](../design/0023-a-read-only-mcp-server-in-docz-api-oauth-21-tokens-tools-and.md):
  the design, every section number above
- [INV-0024](../investigation/0024-an-mcp-server-for-docz-api-with-oauth-and-token-auth.md):
  the protocol reading
- [IMPL-0024](0024-confluence-export-from-docz-api-folders-comments-and-the-server.md):
  the previous server IMPL, whose phase and PR shape this follows
- [DESIGN-0021](../design/0021-confluence-export-from-docz-api-per-repository-folders-in.md)
  and [RUNBOOK-0003](../runbook/0003-enable-confluence-export-on-docz-api.md):
  the export refusal this extends and the runbook shape RUNBOOK-0004 copies
- [The official Go SDK](https://github.com/modelcontextprotocol/go-sdk)
  and the [2026-07-28 specification](https://modelcontextprotocol.io/specification/2026-07-28)
- [`internal/`](../../internal/): `auth`, `authhttp`, `session`,
  `authorize`, `httpapi`, `export`, `config`, `telemetry`;
  [`cmd/docz-api/main.go`](../../cmd/docz-api/main.go);
  [`ui/src/app/restore-after-login.test.tsx`](../../ui/src/app/restore-after-login.test.tsx)
  (question 4); [`pkg/doczcore/config/sync.go`](../../pkg/doczcore/config/sync.go)

[DESIGN-0023]: ../design/0023-a-read-only-mcp-server-in-docz-api-oauth-21-tokens-tools-and.md

<!--docz:references:end-->
