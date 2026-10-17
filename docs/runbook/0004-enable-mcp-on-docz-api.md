---
id: RUNBOOK-0004
title: "Enable MCP on docz-api"
status: Draft
author: Donald Gifford
created: 2026-10-17
---

<!-- markdownlint-disable-file MD024 MD025 MD041 -->

# RUNBOOK-0004: Enable MCP on docz-api

<!--toc:start-->
<!-- markdownlint-capture -->
<!-- markdownlint-disable MD013 -->
- [Last Verified](#last-verified)
- [Overview](#overview)
- [When to Use](#when-to-use)
- [Prerequisites](#prerequisites)
- [Procedures](#procedures)
  - [Procedure 1: Turn MCP on](#procedure-1-turn-mcp-on)
    - [Steps](#steps)
    - [Verification](#verification)
    - [Rollback](#rollback)
  - [Procedure 2: Trust a Keycloak realm](#procedure-2-trust-a-keycloak-realm)
    - [Steps](#steps-1)
    - [Verification](#verification-1)
    - [Rollback](#rollback-1)
  - [Procedure 3: Trust OpenBao's OIDC provider](#procedure-3-trust-openbaos-oidc-provider)
    - [Steps](#steps-2)
    - [Verification](#verification-2)
    - [Rollback](#rollback-2)
  - [Procedure 4: Map a group to a scope and mark a repository private](#procedure-4-map-a-group-to-a-scope-and-mark-a-repository-private)
    - [Steps](#steps-3)
    - [Verification](#verification-3)
    - [Rollback](#rollback-3)
  - [Procedure 5: Connect Claude Code](#procedure-5-connect-claude-code)
    - [Steps](#steps-4)
    - [Verification](#verification-4)
    - [Rollback](#rollback-4)
- [Troubleshooting](#troubleshooting)
  - [Scenario: A client's metadata document is refused](#scenario-a-clients-metadata-document-is-refused)
    - [Steps](#steps-5)
  - [Scenario: A token is rejected with 401](#scenario-a-token-is-rejected-with-401)
    - [Steps](#steps-6)
  - [Scenario: A principal sees no repositories](#scenario-a-principal-sees-no-repositories)
    - [Steps](#steps-7)
- [Escalation](#escalation)
- [References](#references)
<!-- markdownlint-restore -->
<!--toc:end-->

<!--docz:last-verified:start-->
## Last Verified

| Date | PR | Commit | Verified by |
| ---- | -- | ------ | ----------- |
| 2026-10-17 | [feat/access-scopes-config](https://github.com/donaldgifford/docz/tree/feat/access-scopes-config) | be9111a | @donaldgifford (via the integration suite) |

**Notes:** Verified by `just api test-integration` (`TestE2EMCP`: the code
flow through a consent page, a none deployment, `access.scopes`, a trusted
issuer's token, refresh reuse, the export refusal) and the chart's render
and unit tests, not yet by hand. The homelab run of Procedures 1, 2, 4, and
5 with Claude Code and a Keycloak-minted token is pending (IMPL-0026
Phase 6); re-verifying replaces this row.

<!--docz:last-verified:end-->

<!--docz:overview:start-->
## Overview

docz-api can serve its documents to an agent over MCP, behind a bearer
token (DESIGN-0023). The server mints tokens for people through the site's
login and a consent page, and accepts tokens an identity platform the
operator trusts mints for agents. A repository is visible only to a
principal holding one of its `access.scopes`. This runbook turns the
feature on, trusts an issuer, maps a group to a scope, marks a repository
private, connects a client, and reads what the gate is doing.

**Service:** docz-api (`charts/docz`, the `api` workload)

**Owner:** @donaldgifford

<!--docz:overview:end-->

<!--docz:when:start-->
## When to Use

- Turning on MCP for a docz-api deployment
- An agent or a job needs a token from Keycloak or OpenBao rather than a
  person's login
- A team wants its repositories visible only to itself
- `DoczAPITokenFailures` fires, a client cannot connect, or a person sees
  fewer repositories than expected

<!--docz:when:end-->

<!--docz:prerequisites:start-->
## Prerequisites

- Access to the release's values (or its `existingSecret`) and `helm`
- docz-site deployed from the same chart release, with a login
  (`auth.providers` not `none`), since consent happens there
- A public hostname for the API and one for the site, on an ingress or an
  HTTPRoute; the issuer is the API's URL and every token's audience
- `openssl`, `curl`, `jq`, and for Procedure 5 Claude Code
- For Procedure 2 an admin of the Keycloak realm; for Procedure 3 a token
  with write access to OpenBao's `identity/oidc` paths

<!--docz:prerequisites:end-->

## Procedures

<!--docz:procedure:start-->
### Procedure 1: Turn MCP on

Give the server a signing key, its public URLs, and the flag.

<!--docz:steps:start-->
#### Steps

1. Generate the ES256 signing key (IMPL-0026 question 3). The server reads
   a PEM body or a path to one; the chart passes the body from the Secret.

   ```sh
   openssl ecparam -name prime256v1 -genkey -noout -out oauth-signing-key.pem
   ```

   **Expected:** a file starting `-----BEGIN EC PRIVATE KEY-----`; keep it
   in the secret manager, not in the values file committed to git

2. Put the key in the API's Secret under the key `oauth-signing-key`. With
   `api.secrets.create: true` that is `api.secrets.oauthSigningKey`; with
   `existingSecret`, add the key to that Secret.

   ```sh
   kubectl create secret generic docz-api --dry-run=client -o yaml \
     --from-file=oauth-signing-key=oauth-signing-key.pem | kubectl apply -f -
   ```

3. Set the flag and the two URLs and upgrade. The chart renders
   `MCP_ENABLED`, `MCP_RATE_LIMIT`, `OAUTH_ISSUER`, `DOCZ_SITE_URL`,
   `OAUTH_SIGNING_KEY`, and the `api.authz` block only while
   `api.mcp.enabled` is true, and fails the render if the key is missing.
   The issuer defaults to `https://` on the API's first ingress host or
   HTTPRoute hostname and the site URL to the site's; set them when the
   public names differ from the edge's.

   ```yaml
   api:
     mcp:
       enabled: true
       issuer: https://docz-api.example.com
       siteUrl: https://docz.example.com
   ```

   ```sh
   helm upgrade docz oci://ghcr.io/donaldgifford/charts/docz -f values.yaml
   ```

   **Expected:** the API pod restarts and logs `bearer tokens accepted`
   with the audience `https://docz-api.example.com/mcp`,
   `authorization server mounted` with the consent URL, and
   `mcp server mounted`

4. Read the metadata documents the server now publishes. A client finds
   the authorization server through the first and its endpoints and the
   signing key through the other two.

   ```sh
   curl -s https://docz-api.example.com/.well-known/oauth-protected-resource | jq .
   curl -s https://docz-api.example.com/.well-known/oauth-authorization-server | jq .
   curl -s https://docz-api.example.com/.well-known/jwks.json | jq .
   ```

   **Expected:** `resource` is `https://docz-api.example.com/mcp`,
   `authorization_servers` names the issuer, `scopes_supported` lists
   `public:read` and every scope the map grants, and the JWKS holds one
   ES256 key

<!--docz:steps:end-->

<!--docz:verification:start-->
#### Verification

- An unauthenticated request to `/mcp` is a 401 carrying the challenge,
  and the SDK never sees it:

  ```sh
  curl -si -X POST https://docz-api.example.com/mcp | grep -i www-authenticate
  ```

  prints `Bearer resource_metadata="https://docz-api.example.com/.well-known/oauth-protected-resource", scope="public:read"`.

- The three metrics are present on `/metrics` (`contrib/README.md`):
  `docz_api_token_verifications_total{issuer,status}` counts every bearer
  the gate checked, `docz_api_oauth_tokens_total{grant,status}` every
  `POST /oauth/token`, and `docz_api_mcp_calls_total{tool,status}` every
  MCP request. `docz_api_rate_limited_total{surface,via}` counts the 429s.
  Right after the upgrade each is zero or absent; after Procedure 5 the
  first two carry `status="ok"` and the third names the tools called.

<!--docz:verification:end-->

<!--docz:rollback:start-->
#### Rollback

1. Set `api.mcp.enabled: false` and upgrade. Nothing in the block is
   rendered, `/mcp` and the OAuth endpoints are not mounted, an
   `Authorization` header is ignored, every outstanding token stops
   working at once (there is no revocation list; the gate is gone), and
   the cookie login is unchanged. The Secret key may stay.

<!--docz:rollback:end-->
<!--docz:procedure:end-->

<!--docz:procedure:start-->
### Procedure 2: Trust a Keycloak realm

Accept tokens a Keycloak realm mints for an agent or a job. The server has
no Keycloak-specific code: any issuer with OIDC discovery and a JWKS works
the same way, so this procedure is the realm's side plus one chart value.

<!--docz:steps:start-->
#### Steps

1. In the realm, create a client for the agent with client authentication
   on and the service accounts (client credentials) flow enabled. Note its
   client id and secret.

2. Create one client scope per docz scope the agent should hold, named
   exactly as the scope (`public:read`, `platform:read`), with "Include in
   token scope" on, and add them to the client as default scopes. Keycloak
   writes the names of a token's client scopes into its `scope` claim,
   which is what the gate reads.

3. Add an Audience mapper to the client (Mappers, "Add mapper by
   configuration", Audience) with the custom audience
   `https://docz-api.example.com/mcp` included in the access token. The
   gate requires `aud` to contain the issuer's `/mcp` URL exactly.

4. Trust the realm's issuer and upgrade. The value is the realm's base
   URL, discovered at startup; one that cannot be discovered fails the
   boot, as a login provider does.

   ```yaml
   api:
     mcp:
       trustedIssuers:
         - https://keycloak.example.com/realms/platform
   ```

   ```sh
   helm upgrade docz oci://ghcr.io/donaldgifford/charts/docz -f values.yaml
   ```

   **Expected:** `bearer tokens accepted` logs the realm under
   `trusted_issuers`

5. Mint a token for the agent and call the API with it.

   ```sh
   TOKEN=$(curl -s -X POST \
     https://keycloak.example.com/realms/platform/protocol/openid-connect/token \
     -d grant_type=client_credentials -d client_id=docz-agent \
     -d client_secret="$CLIENT_SECRET" | jq -r .access_token)
   curl -s -H "Authorization: Bearer $TOKEN" \
     https://docz-api.example.com/api/v1/repos | jq '.repos[].repo'
   ```

   **Expected:** the repositories whose `access.scopes` the token's scopes
   unlock; `/api/v1/auth/session` with the same header reports
   `via: token`, the realm as `issuer`, and the scopes

<!--docz:steps:end-->

<!--docz:verification:start-->
#### Verification

- `docz_api_token_verifications_total{issuer="https://keycloak.example.com/realms/platform",status="ok"}`
  increments with each call
- Decoding the token (`jq -R 'split(".")[1] | @base64d | fromjson'`)
  shows `iss` equal to the trusted value with no trailing slash, `aud`
  containing `https://docz-api.example.com/mcp`, and `scope` holding the
  docz scopes

<!--docz:verification:end-->

<!--docz:rollback:start-->
#### Rollback

1. Remove the issuer from `api.mcp.trustedIssuers` and upgrade. Its tokens
   are rejected on the next request; docz-api's own keep working.

<!--docz:rollback:end-->
<!--docz:procedure:end-->

<!--docz:procedure:start-->
### Procedure 3: Trust OpenBao's OIDC provider

Accept identity tokens OpenBao mints for a job (IMPL-0026 question 8).
OpenBao's identity secrets engine signs tokens with a named key, publishes
discovery and a JWKS under `identity/oidc`, and fills the claims from a
role's template, so a job with an OpenBao token can mint a docz token
without a person.

<!--docz:steps:start-->
#### Steps

1. Create a named key and a role whose `client_id` is the docz audience
   and whose template writes the scopes. The role's `client_id` becomes
   the token's `aud`, which the gate requires to be the issuer's `/mcp`
   URL.

   ```sh
   bao write identity/oidc/key/docz allowed_client_ids="https://docz-api.example.com/mcp" \
     algorithm=ES256 rotation_period=24h verification_ttl=48h
   bao write identity/oidc/role/docz-agent key=docz ttl=15m \
     client_id="https://docz-api.example.com/mcp" \
     template='{"scope": "public:read platform:read"}'
   ```

2. Set the issuer OpenBao advertises to its public URL, so the `iss` claim
   and the discovery document match what docz-api will trust.

   ```sh
   bao write identity/oidc/config issuer=https://bao.example.com
   curl -s https://bao.example.com/v1/identity/oidc/.well-known/openid-configuration | jq .issuer
   ```

   **Expected:** `https://bao.example.com/v1/identity/oidc`

3. Trust that issuer and upgrade.

   ```yaml
   api:
     mcp:
       trustedIssuers:
         - https://bao.example.com/v1/identity/oidc
   ```

   ```sh
   helm upgrade docz oci://ghcr.io/donaldgifford/charts/docz -f values.yaml
   ```

   **Expected:** `bearer tokens accepted` logs the issuer under
   `trusted_issuers`

4. Mint a token from a job's OpenBao token and call the API with it. The
   policy the job holds needs `read` on `identity/oidc/token/docz-agent`.

   ```sh
   TOKEN=$(bao read -field=token identity/oidc/token/docz-agent)
   curl -s -H "Authorization: Bearer $TOKEN" \
     https://docz-api.example.com/api/v1/repos | jq '.repos[].repo'
   ```

   **Expected:** the repositories `public:read` and `platform:read` unlock

<!--docz:steps:end-->

<!--docz:verification:start-->
#### Verification

- `docz_api_token_verifications_total{issuer="https://bao.example.com/v1/identity/oidc",status="ok"}`
  increments with each call
- Rotating the key (`bao write -f identity/oidc/key/docz/rotate`) is
  invisible to callers: the gate refreshes a key set on an unknown `kid`,
  at most once a minute

<!--docz:verification:end-->

<!--docz:rollback:start-->
#### Rollback

1. Remove the issuer from `api.mcp.trustedIssuers` and upgrade, or delete
   the role so OpenBao mints nothing more.

<!--docz:rollback:end-->
<!--docz:procedure:end-->

<!--docz:procedure:start-->
### Procedure 4: Map a group to a scope and mark a repository private

Give a team a scope, and let a repository require it (DESIGN-0023 §7).
Every principal holds `public:read`; the scope map adds scopes to a
person's session and the tokens minted from it, and a repository's
`access.scopes` says which scopes unlock it.

<!--docz:steps:start-->
#### Steps

1. Map the group, or a login, to the scope and upgrade. A key is
   `<provider>:group:<name>`, matched against the groups the login's
   `id_token` carries (Okta and Keycloak; GitHub has none), or
   `<provider>:login:<name>`. The scope is `<audience>:read` with the
   audience lower-case letters, digits, and hyphens. The chart renders the
   map as `AUTHZ_SCOPE_MAP`.

   ```yaml
   api:
     authz:
       scopeMap:
         keycloak:group:platform: [platform:read]
         github:login:donaldgifford: [platform:read]
   ```

   ```sh
   helm upgrade docz oci://ghcr.io/donaldgifford/charts/docz -f values.yaml
   ```

   **Expected:** after a fresh login, `GET /api/v1/auth/session` lists
   `platform:read` under `scopes` for a member and not for anyone else

2. In the repository, require the scope in `.docz.yaml` and merge it to
   the default branch. An absent block means `[public:read]`; listing
   scopes replaces that, so a repository that should stay public to
   readers and also export to Confluence lists both.

   ```yaml
   access:
     scopes:
       - platform:read
   ```

   **Expected:** the push re-ingests the repository, and `docz validate`
   on it is clean

3. Confirm the repository moved. A principal without the scope gets a 404
   from every route under the repository and does not see it in
   `/api/v1/repos` or in search; one with it does.

   ```sh
   curl -s -b "docz_session=$SESSION" \
     https://docz-api.example.com/api/v1/repos | jq '.repos[].repo'
   ```

<!--docz:steps:end-->

<!--docz:verification:start-->
#### Verification

- A member's session and the tokens it mints list the scope; the
  repository appears for them and is a 404 for a `public:read` principal
- With `sync.confluence` enabled, the repository's export reports
  `refused` ("repository scopes grant the export nothing") until
  `confluence:write` is added to `access.scopes` (ADR-0005's rule is
  unchanged; this is a second gate)

<!--docz:verification:end-->

<!--docz:rollback:start-->
#### Rollback

1. Remove the `access` block from the repository, or the key from the
   map, and upgrade or push. The allowed set is cached per principal for
   `AUTHZ_CACHE_TTL` (30s by default), so a change shows within that.

<!--docz:rollback:end-->
<!--docz:procedure:end-->

<!--docz:procedure:start-->
### Procedure 5: Connect Claude Code

Connect a person's Claude Code to the server through the login and the
consent page. The client identifies itself with a Client ID Metadata
Document, a URL the server fetches; no registration step.

<!--docz:steps:start-->
#### Steps

1. Add the server.

   ```sh
   claude mcp add --transport http docz https://docz-api.example.com/mcp
   ```

2. Start Claude Code, run `/mcp`, and choose `docz` to authenticate. The
   browser opens the site's login, then the consent page
   `https://docz.example.com/oauth/consent`, which names the client and
   the scopes the person can grant (their own, never more); allow it.

   **Expected:** the browser lands back in the client, and `/mcp` shows
   `docz` connected with six tools

3. Ask for a document.

   ```text
   Use the docz get_doc tool to read RFC-0001 in acme/widgets.
   ```

   **Expected:** the document's markdown, with its typed fields beside it;
   a repository the person's scopes do not unlock is `repo not found`

<!--docz:steps:end-->

<!--docz:verification:start-->
#### Verification

- `docz_api_oauth_tokens_total{grant="authorization_code",status="ok"}`
  and then `{grant="refresh_token",status="ok"}` increment: the client
  redeemed a code and, after 15 minutes, refreshed
- `docz_api_mcp_calls_total{tool="get_doc",status="ok"}` increments, and
  the API log carries one `mcp call` line per request with the principal,
  the client, the tool, and the repository

<!--docz:verification:end-->

<!--docz:rollback:start-->
#### Rollback

1. `claude mcp remove docz`. The refresh token the client holds stops
   being used; a replay of a rotated-out refresh token revokes the whole
   family, so a stolen one is bounded by the 15-minute access lifetime.

<!--docz:rollback:end-->
<!--docz:procedure:end-->

## Troubleshooting

Every bearer the gate checks is counted in
`docz_api_token_verifications_total` by issuer and status, every token
request in `docz_api_oauth_tokens_total` by grant and status, and every
MCP request in `docz_api_mcp_calls_total` by tool and status. A request
the limiter answered 429 is in `docz_api_rate_limited_total`. The API log
names the cause of each refusal.

<!--docz:scenario:start-->
### Scenario: A client's metadata document is refused

**Likely cause:** the `client_id` is not an `https` URL the server can
fetch, the document is not what RFC 7591 describes, or the `redirect_uri`
the client sent is not one the document lists.

<!--docz:steps:start-->
#### Steps

1. Diagnose: the browser shows an `invalid_client` error page from
   `/oauth/authorize` before any redirect, since the server trusts no
   `redirect_uri` until the document is read. With `LOG_LEVEL=debug` the
   API logs `oauth authorize: client refused` with the URL and the cause.
2. Diagnose: fetch the document yourself. It must be JSON whose
   `client_id` equals its own URL, with `redirect_uris` listing the
   client's callback; the server refuses a host that resolves to a
   private or loopback address and a document over 64 KiB. A loopback
   `http` callback matches on any port (RFC 8252 §7.3), because a native
   client such as Claude Code binds an ephemeral one; every other
   callback must match exactly.

   ```sh
   curl -s "$CLIENT_ID" | jq '{client_id, client_name, redirect_uris}'
   ```

3. Resolve: fix the document or the client's callback. The server caches
   a document for its `Cache-Control` max-age (at most an hour), so a
   corrected one is read on the next attempt after that.

<!--docz:steps:end-->
<!--docz:scenario:end-->

<!--docz:scenario:start-->
### Scenario: A token is rejected with 401

**Alert:** DoczAPITokenFailures

**Likely cause:** the token's `aud` is not `<OAUTH_ISSUER>/mcp`, its `iss`
is not docz-api's own nor one in `api.mcp.trustedIssuers`, it has expired,
or the issuer rotated its key and the `kid` is unknown.

<!--docz:steps:start-->
#### Steps

1. Diagnose: read the series. The `issuer` label is the token's `iss`
   claim, or `unknown` when the token did not parse at all.

   ```promql
   sum by (issuer) (increase(docz_api_token_verifications_total{status="rejected"}[15m]))
   ```

2. Diagnose: decode the token and compare. `aud` must contain the issuer's
   `/mcp` URL character for character (scheme, host, no trailing slash);
   `iss` must equal a trusted value, which the server stores without a
   trailing slash; `exp` and `nbf` are checked with 30 seconds of skew.

   ```sh
   echo "$TOKEN" | jq -R 'split(".")[1] | @base64d | fromjson | {iss, aud, exp, scope}'
   ```

3. Resolve: for a wrong audience, fix the issuer's mapper (Procedure 2,
   step 3) or the role's `client_id` (Procedure 3, step 1); for an
   untrusted issuer, add it (Procedure 2, step 4); for an unknown `kid`
   right after a rotation, wait a minute, since the key set is refreshed
   at most that often; for a wrong `OAUTH_ISSUER` (the chart derived a
   host that is not the public one), set `api.mcp.issuer` and upgrade,
   which invalidates every token minted under the old value.

<!--docz:steps:end-->
<!--docz:scenario:end-->

<!--docz:scenario:start-->
### Scenario: A principal sees no repositories

**Likely cause:** the principal's scopes unlock nothing: the scope map's
key does not match the group the `id_token` carries, the login provider
does not assert groups, or every repository lists scopes the principal
lacks. The server answers an empty list, not an error.

<!--docz:steps:start-->
#### Steps

1. Diagnose: read the principal's scopes. For a person that is
   `GET /api/v1/auth/session` with the cookie (`scopes`, and `groups` as
   the provider asserted them); for a token, the same route with the
   bearer, or the `scope` claim.
2. Diagnose: compare with the repository's `access.scopes` in its
   `.docz.yaml` at the default branch; a principal needs any one of them.
3. Resolve: for a group that is missing from the session, the provider is
   not sending it: Keycloak needs the `groups` client scope on the login
   client and Okta a groups claim on the authorization server, and the
   name in the map must match what the claim carries. For a GitHub login,
   use a `github:login:<name>` key. For an agent, the issuer's template
   or client scopes must name the scope. A map change applies to a
   session on its next request; a group change needs a fresh login,
   since groups are read at login; a token's scopes are fixed when it is
   minted.

<!--docz:steps:end-->
<!--docz:scenario:end-->

<!--docz:escalation:start-->
## Escalation

| Who | When | How |
| --- | ---- | --- |
| @donaldgifford | a valid token with a matching audience and a trusted issuer is rejected | a GitHub issue on donaldgifford/docz |

<!--docz:escalation:end-->

<!--docz:references:start-->
## References

- [DESIGN-0023](../design/0023-a-read-only-mcp-server-in-docz-api-oauth-21-tokens-tools-and.md):
  the design
- [IMPL-0026](../impl/0026-a-read-only-mcp-server-in-docz-api-tokens-scoped-access-and-the.md):
  the implementation plan
- [`charts/docz` README](../../charts/docz/README.md), "MCP server and
  bearer tokens"
- [`contrib/README.md`](../../contrib/README.md), the token, OAuth, MCP,
  and rate-limit metrics
- [RUNBOOK-0003](0003-enable-confluence-export-on-docz-api.md): the
  Confluence export this feature's `confluence:write` scope gates

<!--docz:references:end-->
