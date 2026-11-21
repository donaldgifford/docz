# CLAUDE.md: `internal/oauth`

Loaded when a session works under this directory. The repository's map is the root `CLAUDE.md`. It also covers
`internal/session`'s gate and `internal/authorize`'s scopes.

## Scoped access, tokens, and the MCP server (DESIGN-0023 / IMPL-0026)

docz-api stays read-only (ADR on scope: no write tools; `write` is a
reserved action except the literal `confluence:write`, the export's
alone). IMPL-0026 lands in six phases, one `dont-release` PR each and one
`minor` after the last. What has landed so far:

- **Phase 1, the library** (`pkg/doczcore/config/access.go`): the
  `access.scopes` block, `ValidateScope`, `public:read` the default, and
  the parity normaliser `AccessNormalizer` for the block `docz init` now
  writes. See the config bullet above.
- **Phase 2, storage and settings**: migration `20261017000000_add_oauth`
  (`repos.access_scopes` JSONB, `oauth_clients`, `oauth_refresh_tokens`,
  `oauth_consents`), `queries/oauth.sql` (`ListRepoIDsForScopes` through
  the JSONB `?|` operator, `RotateRefreshToken` as one CTE), ingest maps
  `access.scopes` onto the row, and `internal/config/mcp.go` reads
  `MCP_*`, `OAUTH_*`, `AUTHZ_*`: `ParseScopeMap` rejects a public
  audience and any write action, `ParseSigningKey` takes a P-256 key in
  SEC 1 or PKCS #8 PEM (or a path to one), MCP on requires an `https`
  issuer with no path and, with a login, a key and `DOCZ_SITE_URL`.
- **Phase 3, the gate** (`internal/oauth`, `internal/session/authn.go`,
  `internal/authorize/scopes.go`, `cmd/docz-api/authn.go`):
  `oauth.Verifier` checks a JWT's signature against docz-api's own key or
  a trusted issuer's JWKS (discovered at boot, cached in Redis under
  `jwks:<sha256(issuer)>`, refreshed on an unknown `kid` at most once a
  minute), `iss`, `aud` = `<OAUTH_ISSUER>/mcp`, `exp`/`nbf` with 30s of
  skew, and reads `scope` or `scp`; `oauth.Signer` mints ES256 tokens
  with the RFC 7638 thumbprint as `kid`. `session.Authn` is the one
  middleware: **a bearer first, and any bearer failure is a `401` with
  `WWW-Authenticate: Bearer resource_metadata="<issuer>/.well-known/oauth-protected-resource", scope="public:read"`
  and never a fall-through to the cookie**; then the cookie with the
  `401`/`503` split; then the anonymous principal under
  `AUTH_PROVIDERS=none`. A nil verifier (MCP off, the default) ignores
  the header. `Session` carries `Via` (`cookie`/`token`/`anonymous`),
  `Scopes`, `ClientID`, `Issuer`, and `PrincipalKey()`
  (`<issuer>:<sub>` for an external token, `user:<provider>:<subject>`
  for a person, so a person's cookie and their docz-api-minted token are
  one principal). A cookie session's scopes are computed per request by
  `authorize.ScopesFor` (public:read plus the scope map's grants over
  groups and login). `authorize.ScopeAuthorizer` resolves the allowed set
  from the session's scopes through `ListRepoIDsForScopes`, cached per
  principal key for `AUTHZ_CACHE_TTL` and recomputed when the scopes
  change inside the window; `AllReposAuthorizer` stays under `none`.
  Spec **1.7.0**: the `bearerToken` scheme beside `sessionCookie` on
  every `/api/v1` operation, and `Session` gains required `via` and
  `scopes` plus `client_id`/`issuer` for a token. The export refuses a
  repository whose `access_scopes` holds neither `public:read` nor
  `confluence:write` ("repository scopes grant the export nothing; add
  confluence:write to access.scopes"). Metric
  `docz_api_token_verifications_total{issuer,status}`.
- **Phase 4, the authorization server** (`internal/oauth`, the rest):
  `oauth.Metadata` serves the RFC 9728, RFC 8414, and JWKS documents
  (`scopes_supported` = `public:read` plus the scope map's, pinned by
  goldens under `testdata/metadata/`); `oauth.ClientFetcher` resolves a
  `client_id` as a Client ID Metadata Document (`https` only, no
  redirects, 5 s, 64 KiB, a dialer whose injected `Resolver` refuses
  loopback/link-local/private/ULA addresses, `client_id` equal to the
  URL, `redirect_uris` required and `https` or loopback `http`, a
  loopback one matching on any port (RFC 8252 §7.3, `matchRedirect`), cached
  under `cimd:<sha256(url)>` for `max-age` capped at an hour);
  `oauth.Server` is `GET /oauth/authorize` (validation in §5's order:
  pre-trust failures are an HTML error page, post-trust ones an error
  redirect with `state` and `iss`; the request pending under
  `authz:<id>` for ten minutes, then a redirect to
  `<DOCZ_SITE_URL>/oauth/consent?authz=<id>`), the consent endpoints
  `GET`/`POST /api/v1/oauth/authorizations/{id}` (the first `GET` binds
  the request to the session; another session's, unknown, expired, and
  consumed are all `404`; `allow` mints `code:<sha256(code)>` for 60 s
  and upserts `oauth_consents`), `POST /oauth/token` (code grant with
  PKCE `S256`, client, redirect_uri, and resource checked against the
  pending request; a replayed code revokes its family through
  `redeemed:<hash>`; refresh grant rotates in one statement and reuse of
  a rotated-out token revokes the family), and `POST /oauth/revoke`.
  The package takes the site session through `ServerOptions.Session`
  (a `func(ctx) (Principal, bool)`), because `session` imports `oauth`
  for `Claims`. **A refresh recomputes the scope against what the map
  can still grant, not against the person's current groups**: groups
  live only in the Redis session and are never persisted, so a group
  lost mid-family keeps its scope until the family ends; the design's
  "re-reads the person's scopes" is met for the login and
  operator-removed-scope cases only. `cmd/docz-api/authn.go`
  `buildAuthStack` builds the verifier and the server together (the
  server reuses the verifier's Redis cache) and `runServer` mounts the
  public routes outside both gates and the consent routes through
  `Mount`'s extras; under `AUTH_PROVIDERS=none` none of it is mounted.
  Metric `docz_api_oauth_tokens_total{grant,status}`; the `oauth token`
  audit line carries `grant`, `client_id`, `principal`, `status`, never
  a token. The site's `/oauth/consent` route (`ui/src/routes/consent.tsx`)
  renders from the two consent endpoints and sets `window.location` to
  `redirect_to`. The contract test mounts the server over
  `oauth.MemoryCache`, a `RoundTripper` serving the client document, and
  an in-memory store, so kin-openapi validates every OAuth route; cases
  the validator rejects before the handler (an unknown `grant_type`, a
  missing `token`) are covered in `internal/oauth` instead.
  **Prettier is a `ui` devDependency** (IMPL-0027): `format:check`
  runs the lockfile's version locally and in CI alike, since the
  global `prettier@3.7.4` CI used to install lost to the copy
  `json-schema-to-typescript` brought into `node_modules`.
