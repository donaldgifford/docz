# Deploying docz-api

A reference single-host deployment: the `docz-api` service plus Postgres, Redis,
and Meilisearch on a private Docker network. For local development of the
service itself, use the repo-root `compose.yaml` (dependencies only) with
`just run`.

## Layout

- `compose.yaml` — the full production-shaped stack (service + three
  dependencies).
- `compose.local.yaml` — the **local environment**: the same stack built from
  the working tree plus an ngrok webhook tunnel, driven by `just local-up`; see
  [DEVELOPMENT.md](../DEVELOPMENT.md#full-local-environment-just-local-up).
- `compose.monitoring.yaml` — the **local observability stack** (prometheus,
  grafana, jaeger, loki, otel-collector, alloy; keycloak behind
  `--profile auth`), driven by `just monitor-up`. Backends only — pair it with
  the app from `just run` or `just local-up`; see
  [DEVELOPMENT.md](../DEVELOPMENT.md#local-monitoring-stack-just-monitor-up).
- `dev/` — config mounted by `compose.monitoring.yaml`: `prometheus/`,
  `grafana/provisioning/` (datasources + the docz-api overview dashboard),
  `otel/otel-collector.yaml`, `alloy/config.alloy`, and `keycloak/` (the seeded
  `docz-api` realm import).
- `.env.production.example` — configuration template; copy to `.env.production`
  (gitignored) and fill in from your secret manager.
- `.env.local.example` — the local environment's template; copy to `.env.local`
  (gitignored).
- `secrets/github-app.pem` — the GitHub App private key, mounted into the
  service as a Docker secret (gitignored; you create it; shared by both stacks).

## Bring-up

```sh
cd deploy
cp .env.production.example .env.production      # then fill in real values
mkdir -p secrets && cp /path/to/your/app.pem secrets/github-app.pem
docker compose up -d --build
docker compose ps
```

The service applies database migrations automatically on startup, so there is no
separate migration step.

## Configuration and secrets

All configuration is read from the environment. `compose.yaml` loads
`.env.production` as the service's env store; source that file from your secret
manager (SOPS, Vault, 1Password, ...) rather than committing it.

`compose.yaml` overrides the networking values (`DATABASE_URL`, `REDIS_URL`,
`MEILI_HOST`) so the service reaches its dependencies by their compose service
names, and delivers the GitHub App private key as a mounted secret file
(`GITHUB_APP_PRIVATE_KEY=/run/secrets/github_app_key`). Everything else —
webhook secret, session secret, OAuth/OIDC credentials, Meili key — comes from
`.env.production`.

## GitHub App setup (ingestion)

docz-api ingests repos as a **GitHub App**: install-driven onboarding, HMAC
webhooks, and content fetches over the Git Trees API with short-lived
installation tokens. Create one under _Settings → Developer settings → GitHub
Apps → New GitHub App_ (org or user account both work).

### App configuration

| Setting              | Value                                                            |
| -------------------- | ---------------------------------------------------------------- |
| Homepage URL         | Anything (e.g. this repo).                                       |
| Webhook URL          | `https://<your-host>/webhooks/github`                            |
| Webhook content type | `application/json`                                               |
| Webhook secret       | A strong random value — the same one as `GITHUB_WEBHOOK_SECRET`. |
| SSL verification     | Enabled.                                                         |
| Where installable    | "Only on this account" is fine for a homelab.                    |

The webhook receiver authenticates by HMAC-SHA256 over the raw body
(constant-time compare, fails closed), so the endpoint itself can be public; no
session or extra auth applies to `/webhooks/github`.

For **local development** the webhook URL can be an ngrok tunnel to your machine
— `just dev-tunnel` prints it; see
[DEVELOPMENT.md](../DEVELOPMENT.md#receiving-github-webhooks-locally-ngrok).

### Repository permissions

| Permission | Access    | Why                                                                                                          |
| ---------- | --------- | ------------------------------------------------------------------------------------------------------------ |
| Contents   | Read-only | Git refs/trees/blobs for `.docz.yaml`, docs, `CHANGELOG.md`, `index.md`; also gates the push/release events. |
| Metadata   | Read-only | Mandatory for every GitHub App (repo lookup, default branch).                                                |

Nothing else — no write access of any kind. Ingestion needs no account
permissions; the one exception is **Email addresses: Read-only** if you reuse
this app for site login (see below).

### Webhook events

Subscribe to:

- **Push** — a push to the repo's default branch that touches `.docz.yaml` or
  anything under `docs_dir/` triggers a full re-ingest (debounced; content-hash
  gated, so unchanged docs are no-ops). Pushes to other branches or unrelated
  paths are ignored.
- **Release** — received and logged only today; reserved for the future versions
  feature.

**Installation** and **Installation repositories** events are delivered to every
GitHub App automatically (no checkbox): installing the app or adding repos to an
installation onboards and enqueues an ingest per repo; uninstalling or removing
a repo offboards it (rows deleted, search index purged). A repo without a
`.docz.yaml` at HEAD fails its ingest and is logged — add the manifest and push
to onboard it.

### Keys and identifiers

After creating the app:

1. Note the **App ID** (the app's About page) → `GITHUB_APP_ID`.
2. **Generate a private key** (PEM) → save as `secrets/github-app.pem`;
   `GITHUB_APP_PRIVATE_KEY` takes the file path (the compose stack mounts it as
   a Docker secret at `/run/secrets/github_app_key`) or the PEM body itself.
3. Set `GITHUB_WEBHOOK_SECRET` to the webhook secret from above.
4. **Install the app** on the account and select the docz repos — installation
   is the onboarding; there is no separate registration step. The manual
   fallback for a missed installation event is
   `docz-api -onboard owner/name@<installationID>`.

For GitHub Enterprise, point `GITHUB_API_BASE` at your instance's API root;
everything else is unchanged.

### Site login: reuse the GitHub App, or a separate OAuth app

Site login (`AUTH_PROVIDERS=github`) needs OAuth client credentials in
`GITHUB_OAUTH_CLIENT_ID` / `GITHUB_OAUTH_CLIENT_SECRET`. Two ways to get them:

**Reuse the GitHub App above** — every GitHub App supports the same OAuth web
flow, so one app can serve both ingestion and login. **Recommended for local
development** (one app to create and configure) and perfectly fine for a homelab
deployment. Three settings on the existing app:

1. Set the **Callback URL** (a separate field from the webhook URL) to
   `<AUTH_REDIRECT_BASE>/auth/callback`. Leave "Request user authorization
   during installation" unchecked — users just visit `/auth/login`.
2. **Generate a client secret** → `GITHUB_OAUTH_CLIENT_SECRET`; the app's
   **Client ID** (`Iv1.…`) → `GITHUB_OAUTH_CLIENT_ID`. The private key stays
   ingest-only.
3. Add the **account permission "Email addresses: Read-only"**. GitHub Apps
   ignore OAuth scopes (permissions replace them), and without this the email
   lookup 403s and login fails for any user whose profile email is private.
   Existing installations must re-approve the permission change.

Notes: user-token expiry is irrelevant (the service discards the GitHub token
right after the exchange — its own Redis session governs login lifetime), and
authorizing the app to log in is separate from installing it, so login access is
not limited to accounts that installed the app.

**Or a separate OAuth app** (_Settings → Developer settings → OAuth Apps_) with
the authorization callback URL `<AUTH_REDIRECT_BASE>/auth/callback` — the
cautious default for a production deployment, keeping the ingest and login
credentials in separate blast radii. The service requests the `read:user` and
`user:email` scopes; no scopes are configured on the app itself.

### Enabling Okta (OIDC)

Okta is a first-class login provider alongside GitHub. Add `okta` to
`AUTH_PROVIDERS` and set the three `OKTA_*` variables:

```sh
AUTH_PROVIDERS=github,okta            # or just okta
OKTA_ISSUER=https://acme.okta.com/oauth2/default
OKTA_CLIENT_ID=...
OKTA_CLIENT_SECRET=...
```

The service performs OIDC discovery against `OKTA_ISSUER` **at startup** (a bad
issuer fails the boot, not the first login), then runs the standard
authorization-code flow and verifies the returned `id_token` (signature via the
issuer's JWKS, audience, issuer, expiry). Okta and Keycloak share this exact
code path — Keycloak is enabled the same way with `KEYCLOAK_*` variables. Three
Okta-specific things to get right:

1. **Match the issuer form exactly.** Okta exposes two: the org authorization
   server (`https://acme.okta.com`) and a custom/default one
   (`https://acme.okta.com/oauth2/default`). `OKTA_ISSUER` must be the value
   Okta's own discovery document reports for that app — a mismatch fails
   `id_token` verification with an issuer error. When in doubt, open
   `<issuer>/.well-known/openid-configuration` and copy the `issuer` field
   verbatim.
2. **Groups need a claim mapping.** The service requests the `groups` scope and
   reads a `groups` claim, but Okta does **not** emit one by default — add a
   groups claim to the authorization server (Security → API → Authorization
   Servers → your server → Claims) if you want `Identity.Groups` populated.
   Authorization is currently a pass-through seam, so an empty `groups` is
   harmless today; this only matters once group-based access lands.
3. **Register the redirect URI.** The Okta app's **Sign-in redirect URIs** must
   include `<AUTH_REDIRECT_BASE>/auth/callback` (same value the GitHub flow
   uses). Also confirm the user's email is **verified** in Okta — the service
   drops an email the issuer marks `email_verified:false`.

For local development you usually run **Keycloak** instead of a hosted Okta
tenant (same OIDC code path); see
[DEVELOPMENT.md](../DEVELOPMENT.md#local-monitoring-stack-just-monitor-up).

## Health and observability

- **Liveness:** `GET /healthz` — process is up.
- **Readiness:** `GET /readyz` — Postgres, Redis, and Meilisearch are all
  reachable (503 with a per-dependency body otherwise). Point your
  orchestrator's probes here; the distroless image has no shell, so there is no
  in-container healthcheck for the service.
- **Metrics:** `GET /metrics` — Prometheus exposition (disable with
  `METRICS_ENABLED=false`). Scrape it on the internal network; it is not behind
  the auth gate.
- **Tracing:** set `OTEL_EXPORTER_OTLP_ENDPOINT` (host:port, OTLP/HTTP) to a
  collector to export traces; unset, tracing is a no-op.

## Notes

- The service listens on `:8080`; only that port is published. Postgres, Redis,
  and Meilisearch stay on the private network.
- **Repo index backfill (DESIGN-0003):** the
  `/api/v1/repos/{owner}/{name}/index` endpoint serves the `docs_dir/index.md`
  cached at each repo's **last ingest**. Repos onboarded before this feature
  shipped return 404 until their next default-branch push touching `docs_dir/`
  (or `.docz.yaml`) re-ingests them — or run a manual
  `docz-api -onboard owner/name@installationID` per repo. No migration or
  backfill job is required; the docz-site's metadata fallback covers the gap.
- The images pin major/minor tags (`postgres:17-alpine`, `redis:7.4-alpine`,
  `getmeili/meilisearch:v1.12`); Renovate PRs updates.
- For Kubernetes, translate this to a Deployment (service) plus StatefulSets or
  managed equivalents for the three dependencies, HTTP liveness/readiness probes
  against `/healthz` and `/readyz`, and secrets from a `Secret`/CSI provider.
