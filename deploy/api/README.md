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

## First setup: get ingest working before login

A first deploy has to get two independent credential sets right at once — the
GitHub App and a login provider — and a failure in either looks identical from
the outside: no docs. Start with login turned off so the only thing that can be
wrong is the GitHub App.

Set `AUTH_PROVIDERS=none` in `.env.production` and leave `SESSION_SECRET`,
`AUTH_REDIRECT_BASE`, and every `*_CLIENT_ID`/`*_CLIENT_SECRET` unset. The
service boots, logs a warning that auth is disabled, serves every request as a
synthetic anonymous identity, and mounts no `/auth/login` or `/auth/callback`
route. `"none"` must be the only entry — pairing it with a real provider is a
config error, not a precedence rule.

Now verify ingestion end to end:

```sh
docker compose logs -f docz-api        # expect one "github app authenticated" line
curl -s localhost:8080/readyz          # postgres, redis, meilisearch all ok
curl -s localhost:8080/api/v1/repos    # your onboarded repos, no cookie needed
```

**This leaves the read API open to anyone who can reach it.** Use it on a
private network or a local stack, never on a public endpoint.

When ingestion is proven, add login: set `AUTH_PROVIDERS` to your provider(s),
fill in `SESSION_SECRET`, `AUTH_REDIRECT_BASE`, and that provider's credentials
per the sections below, and redeploy. Nothing else changes — the API surface,
routes, and response shapes are identical in both modes, so a docz-site pointed
at a none-mode API keeps working unmodified once login is on.

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

For a **Kubernetes / homelab** deployment, expose docz-api behind its **own**
Tailscale Funnel node. The Helm chart ships a Tailscale sidecar
(`tailscale.enabled=true`) that joins the tailnet as a separate node, so
docz-api gets its **own** MagicDNS hostname (`tailscale.hostname`, default
`docz-api`) and its webhook lives at
`https://docz-api.<tailnet>.ts.net/webhooks/github`. Each app runs its own
sidecar and therefore its own Funnel hostname, so the shared `/webhooks/github`
path never collides with a sibling service — there is no need to override the
path. Enable Funnel for the node's tag in your tailnet ACLs and supply a
Tailscale auth key via `tailscale.authKeySecret`.

Three things that produce a **TLS EOF** on every webhook delivery if missed:

1. **Funnel must be permitted in the tailnet policy** — a `nodeAttrs` entry
   granting the `funnel` attribute to the node's tag, and HTTPS certificates
   enabled for the tailnet (the serve config resolves `${TS_CERT_DOMAIN}`).
   Without it tailscaled refuses to serve and the public ingress drops
   connections.
2. **Node state must persist.** `tailscale.persistState` (default `true`,
   chart ≥ 0.4.0) stores tailscaled's node key in a Secret via
   `TS_KUBE_SECRET`. With ephemeral state the key is regenerated on every
   restart, the old node keeps the `docz-api` hostname, the new one becomes
   `docz-api-1`, and the Funnel DNS record is left pointing at a dead node.
   Requires `tailscale.rbac.create` (the default).
3. **The namespace's Pod Security level.** The sidecar satisfies `restricted`
   as of chart 0.4.0; earlier charts set no `allowPrivilegeEscalation` or
   capability drop, so a `restricted`-enforcing namespace rejected the pod
   outright — which looks like a Funnel outage, because nothing is running.

Diagnose with `tailscale status` / `tailscale funnel status` in the sidecar: a
node named `docz-api-1` (or higher) is the state problem, an empty funnel
status is the ACL one.

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

- **Push** — a push to the repo's default branch that touches `.docz.yaml`,
  anything under `docs_dir/`, or the repo's configured changelog file (when the
  `changelog:` block is enabled) triggers a full re-ingest (debounced; content-hash
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

Okta is a first-class login provider alongside GitHub. In Okta, create the app
as **Applications → Create App Integration → OIDC - OpenID Connect → Web
Application**, grant type _Authorization Code_. It must be that type: the
service holds a client secret and exchanges the code server-side, so it is a
**confidential client**. A Single-Page Application or Native app is a public
client with no secret and cannot complete the exchange.

Then add `okta` to `AUTH_PROVIDERS` and set the three `OKTA_*` variables:

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

On Kubernetes the Helm chart wires the same variables from
`config.authProviders`, `config.oktaIssuer`, `config.oktaClientID` and
`secrets.oktaClientSecret` (Secret key `okta-client-secret`); only the enabled
providers' env and Secret keys are rendered. To source the client secret from a
secret manager, set `secrets.create=false` and point `secrets.existingSecret` at
a Secret you populate however you like — see
[the chart README](../charts/docz-api/README.md).

For local development you usually run **Keycloak** instead of a hosted Okta
tenant (same OIDC code path); see
[DEVELOPMENT.md](../DEVELOPMENT.md#local-monitoring-stack-just-monitor-up).

## Health and observability

Three separate mechanisms answer three different questions. Keeping them
distinct is deliberate — conflating them makes an outage in one dependency
either restart healthy pods or silently do nothing.

- **Liveness:** `GET /healthz` — _the process is up_. A bare 200 that checks
  nothing downstream, because failing it makes the kubelet **restart the pod**,
  and restarting docz-api cannot fix a database that is down. Never add
  dependency checks here.
- **Readiness:** `GET /readyz` — _dependencies are connected and this pod can
  accept traffic_. Checks Postgres, Redis, and Meilisearch, returning 503 with a
  per-dependency body naming the offender. Failing it removes the pod from
  Service endpoints but leaves it running. Point your orchestrator's probes at
  both; the distroless image has no shell, so there is no in-container
  healthcheck for the service.
- **Startup credential check:** at boot the service authenticates as the GitHub
  App (`GET /app`) and logs the app id, slug and name. This is deliberately in
  **neither** probe. GitHub is not a serving dependency — the read API answers
  from Postgres and Meilisearch while GitHub is unreachable — so it must not
  gate readiness. Startup fails only on a **401** (a bad app id or a key GitHub
  will not accept) or a private key that will not parse at all — between them
  the realistic bad-credential cases. Everything else, including a 403 from a
  _suspended_ App, a rate limit, or GitHub simply being unreachable, logs a
  warning and startup continues. That asymmetry is deliberate: a false
  "permanent" would crash-loop the pod and take the read API down for a problem
  that only affects ingest, whereas a missed one costs a single warning and then
  shows up in the per-job ingest error logs.
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
- **Changelog opt-in (IMPL-0005):** the
  `/api/v1/repos/{owner}/{name}/changelog` endpoint serves the changelog cached
  at each repo's **last ingest**, and it is opt-in per repo — the repo's
  `.docz.yaml` must enable the `changelog:` block (`file` defaults to
  `CHANGELOG.md`; a subpath such as `charts/<name>/CHANGELOG.md` is supported).
  Repos that never enable it return 404, and any changelog cached before this
  feature shipped is **cleared** on their next ingest, since the block is
  desired state rather than a sticky cache. Nothing served that data before, so
  no consumer regresses. Opting in takes effect on the next re-ingest: a
  default-branch push touching `.docz.yaml`, `docs_dir/`, or the configured
  changelog file, or a manual
  `docz-api -onboard owner/name@installationID`.
- **Pages opt-in (IMPL-0007):** the `/api/v1/repos/{owner}/{name}/pages`
  endpoints serve the non-docz markdown a repo's `.docz.yaml` `api:` block
  publishes (docz v1.2.0). **Enabling the block publishes every `.md` under
  `docs_dir`** — brief operators accordingly: `api.exclude` is the guard rail
  for drafts and internal notes (`templates/` is always excluded), and
  `api.additional_docs` opts in repo-root files such as `CONTRIBUTING.md`.
  Like the changelog, the block is desired state: disabling it deletes the
  served pages (and their search entries) on the next ingest. Repos without
  the block serve an empty list, cost zero extra GitHub requests at fetch
  time, and are byte-for-byte untouched. Opting in takes effect on the next
  re-ingest, same triggers as the changelog above.
- **Snapshot key spellings (IMPL-0008):** as of docz v1.2.2 the
  `config_snapshot` served on repo detail carries the `.docz.yaml` key
  spellings (`changelog.enabled`, `api.landing_page`) instead of Go field
  names (`Changelog`, `API`). Rows written earlier refresh **naturally** —
  each repo's snapshot rewrites on its next ingest; a default-branch push
  or a `docz-api -onboard owner/name@installationID` nudge clears any
  stragglers at fleet scale. No backfill job exists or is needed.
- **Type dirs publish no pages (IMPL-0009):** a docz type directory's own
  `README.md` (the generated index table) is no longer served under
  `/pages`; the type surface belongs to the consumer's type route. Existing
  rows retire **naturally** — each repo's next ingest deletes them and
  purges them from the search index. The same triggers apply: a
  default-branch push under `docs_dir/`, or a `docz-api -onboard
  owner/name@installationID` nudge. Until then those stale page rows keep
  serving; nothing breaks. No backfill job exists or is needed.
- The images pin major/minor tags (`postgres:17-alpine`, `redis:7.4-alpine`,
  `getmeili/meilisearch:v1.12`); Renovate PRs updates.
- For Kubernetes, translate this to a Deployment (service) plus StatefulSets or
  managed equivalents for the three dependencies, HTTP liveness/readiness probes
  against `/healthz` and `/readyz`, and secrets from a `Secret`/CSI provider.
