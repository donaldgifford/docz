# docz-api OpenAPI contract

`openapi.yaml` is the **hand-authored, machine-readable contract** for
docz-api's HTTP surface (OpenAPI 3.1). It is the single source of truth for the
wire shapes docz-api ships, and it is the artifact the **docz-site** vendors to
generate a typed client.

## What is here

- **`openapi.yaml`** — the OAS 3.1 contract: the `/api/v1` read + search routes,
  the site auth endpoints (`/api/v1/auth/*`, public `/auth/login` +
  `/auth/callback`), and the GitHub webhook receiver (`/webhooks/github`).
- **`spec.go`** — a one-line `//go:embed openapi.yaml` exposing
  `var Spec []byte`, so the runtime server and the contract test consume the
  exact same bytes.
- **`vacuum-ruleset.yaml`** — the linter ruleset used by `just lint-openapi`.

## How it stays honest

An in-process `kin-openapi` contract test
(`internal/httpapi/openapi_contract_test.go`) loads this file, drives the real
chi handler stack in-memory, and validates every request and response against it
on every `go test ./...` / CI run. Response schemas are
`additionalProperties: false`, so any added, renamed, or retyped wire field
fails the contract test — code and spec cannot silently drift.
`just lint-openapi` additionally runs `vacuum` (100/100) and `yamlfmt` so the
file stays standards-clean and canonical.

## Versioning

`info.version` is **SemVer, starting at `1.0.0`** (independent of the binary's
release version). Bump it by hand on any change to a specced wire shape:

- **patch** — editorial only (descriptions, examples); no wire change.
- **minor** — additive, backward-compatible (a new endpoint, a new optional
  field, a new enum value).
- **major** — breaking (a removed or renamed field, a changed type, a removed
  endpoint, a newly required **request** field or header). "Required" is
  request-side only: a response property moving into `required` is a stronger
  promise to the consumer, never a breaking one, so it is additive — `1.4.0`
  set that precedent by adding `source` and `path` as required on `SearchHit`.

The version is the signal consumers pin against, so a wire change without a bump
is a contract bug.

Current: **`1.6.2`** — editorial: `comments_lost` on a Confluence page counts
every comment without an anchor after the last update, remembered ones
included, since a lost anchor is now kept in the page property and tried
again (INV-0021). `1.6.1` — editorial: `git_sha` on `Document` is described as
the source file's blob SHA, a cache key and not a commit; the repository's
synced commit is `last_synced_sha` (IMPL-0025, #169). `1.6.0` — both
document responses carry `confluence_url`, and
`GET /api/v1/repos/{owner}/{name}/confluence` serves the last export
(IMPL-0024). `1.5.0` — every search hit carries `created` (the authored
`YYYY-MM-DD`, `""` on pages) and `updated_at` (RFC 3339 UTC, the same
value the document endpoint serves), `searchDocs` accepts `sort`
(`updated_at`/`created`, each direction) and `source` (`doc`/`page`), and
the operation gains its first `400` for an unrecognized `sort`
(IMPL-0010). The sort is a total order over the matches, and a record
with no value for the key sorts last in either direction. `1.4.2` —
editorial: `groups` on the session response is documented as always
optional, since the OIDC scope that supplies it is now per-provider
configuration rather than a hardcoded request (PR #32).
`1.4.1` — editorial: documents `config_snapshot`'s key
spellings (docz v1.2.2's json-tagged config marshal serves the
`.docz.yaml` names — `changelog.enabled`, `api.landing_page` — with
`omitempty` keys absent when unset and unset list fields as `null`).
The wire type is unchanged; snapshot readers can pin this version for
the spelling guarantee. `1.4.0` — search hits carry `source`
(`doc`/`page`) and `path`,
and `source` joins the facet counts, so page results are distinguishable and
deep-linkable (IMPL-0007 Phase 6). `1.3.0` added the pages surface:
`listRepoPages` (`GET /api/v1/repos/{owner}/{name}/pages`) and `getRepoPage`
(`GET .../pages/{path}`), publishing the markdown a repository's `.docz.yaml`
`api:` block opts into (docz v1.2.0, DESIGN-0004). **Enabling the block
publishes every `.md` under the docs dir** — `api.exclude` is the guard rail
for drafts and internal notes; `templates/` is always excluded. A repository
without the block serves an empty page list (200, not 404) and 404s every page
path. `1.2.0` added `getRepoChangelog` (opt-in via the `changelog:` block;
IMPL-0005). `1.1.0` added `getRepoIndex`.

## Consuming it (the docz-site)

The docz-site vendors-and-generates, mirroring the `rfc-site` model:

1. **Vendor** this `openapi.yaml` into the site repo (pinning a known version),
   **or** fetch it at runtime from `GET /openapi.yaml` (served verbatim from the
   embed, public — no session required).
2. **Generate** a typed client from the vendored spec with the site's toolchain
   (e.g. `openapi-typescript` or `orval`).
3. **Re-vendor** when `info.version` bumps; a major bump signals a breaking
   change to reconcile before upgrading.

There is **no bundled Swagger/Scalar UI** in docz-api (IMPL-0002 OQ-3d) — the
served `/openapi.yaml` is the machine contract; human browsing is left to the
consumer's tooling or an optional docz mkdocs render.

## Tokens

Spec `1.7.0` (DESIGN-0023) lets a script or an MCP client call `/api/v1`
and `/mcp` with `Authorization: Bearer <jwt>` instead of the session
cookie, while `MCP_ENABLED` is true on the server. A token is a JWT whose
`aud` is `<OAUTH_ISSUER>/mcp`, minted by one of two kinds of issuer.

**Through any MCP client.** docz-api is its own authorization server. A
client that gets a `401` reads `WWW-Authenticate: Bearer
resource_metadata=…`, fetches `/.well-known/oauth-protected-resource` and
`/.well-known/oauth-authorization-server`, and runs the OAuth 2.1 code flow
with PKCE (`S256`), a `resource` of `<OAUTH_ISSUER>/mcp`, and its Client ID
Metadata Document URL as `client_id` (no registration). `GET
/oauth/authorize` sends the person to docz-site's `/oauth/consent`, where
they sign in if they must and approve the client; the browser lands on the
client's `redirect_uri` with `code`, `state`, and `iss`; `POST /oauth/token`
exchanges the code for a 15-minute access token and a rotating refresh
token. The scopes a person can grant are `public:read` plus whatever
`AUTHZ_SCOPE_MAP` gives their groups or login; a repository is visible when
its `.docz.yaml` `access.scopes` holds one of them. The same flow works by
hand with a browser and `curl`:

```sh
# 1. the metadata
curl -s https://docz.example/.well-known/oauth-authorization-server | jq .
# 2. a PKCE pair
verifier=$(openssl rand -base64 48 | tr -d '=+/' | cut -c1-64)
challenge=$(printf %s "$verifier" | openssl dgst -sha256 -binary | openssl base64 -A | tr '+/' '-_' | tr -d '=')
# 3. in a browser: approve, then read `code` off the redirect
open "https://docz.example/oauth/authorize?response_type=code&client_id=https://you.example/oauth.json&redirect_uri=http://127.0.0.1:9999/cb&code_challenge=$challenge&code_challenge_method=S256&resource=https://docz.example/mcp&scope=public:read&state=$(openssl rand -hex 8)"
# 4. the token
curl -s https://docz.example/oauth/token -d grant_type=authorization_code -d code="$code" \
  -d code_verifier="$verifier" -d client_id=https://you.example/oauth.json \
  -d redirect_uri=http://127.0.0.1:9999/cb -d resource=https://docz.example/mcp
```

**Minted by the platform.** A token from an issuer in
`OAUTH_TRUSTED_ISSUERS` (Keycloak, Okta, OpenBao's OIDC provider) is
accepted as-is when its `iss` matches, its `aud` contains
`<OAUTH_ISSUER>/mcp`, its signature checks against the issuer's JWKS, and
it is in date; its `scope` or `scp` claim is the principal's scopes. That is
how an unattended job gets access: docz-api mints nothing for a machine.

## References

- [DESIGN-0002](../docs/design/0002-openapi-contract-for-docz-api-and-the-docz-site.md)
  — the design this contract implements.
- [IMPL-0002](../docs/impl/0002-openapi-contract-for-docz-api-and-the-docz-site.md)
  — the phased implementation plan.
