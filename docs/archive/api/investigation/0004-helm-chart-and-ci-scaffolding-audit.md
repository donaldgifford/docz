---
id: INV-0004
title: "Helm chart and CI scaffolding audit"
status: Concluded
author: Donald Gifford
created: 2026-07-13
---

<!-- markdownlint-disable-file MD025 MD041 -->

# INV 0004: Helm chart and CI scaffolding audit

**Status:** Concluded **Author:** Donald Gifford **Date:** 2026-07-13

<!--toc:start-->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
  - [Observation 0 — docz-api's authoritative surface (baseline)](#observation-0--docz-apis-authoritative-surface-baseline)
  - [Observation 1 — yaml-language-server schema tags](#observation-1--yaml-language-server-schema-tags)
  - [Observation 2 — the Helm chart is a repo-guardian shell](#observation-2--the-helm-chart-is-a-repo-guardian-shell)
  - [Observation 3 — CI/release workflows: duplication, make-vs-just, racing bumps](#observation-3--cirelease-workflows-duplication-make-vs-just-racing-bumps)
  - [Observation 4 — docker-bake.hcl dropped the build args](#observation-4--docker-bakehcl-dropped-the-build-args)
  - [Observation 5 — deploy/dev monitoring stack (rfc-api lineage)](#observation-5--deploydev-monitoring-stack-rfc-api-lineage)
  - [Observation 6 — contrib/ dashboard + alerts (repo-guardian lineage)](#observation-6--contrib-dashboard--alerts-repo-guardian-lineage)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
- [References](#references)
<!--toc:end-->

## Question

Is the scaffolding copied onto `feat/helm-chart` — the Helm chart
(`charts/docz-api/`, from repo-guardian), the CI/publish workflows (`ci2.yml`,
`release2.yml`, `ghcr.yml`, `ecr.yml`), the rewritten `docker-bake.hcl`, the dev
observability stack (`deploy/compose.dev.yaml` + `deploy/dev/`), and the
operator assets (`contrib/`) — usable as-is for docz-api, and if not, what
exactly must change before the chart and image can ship to GHCR? Secondary: did
the yaml-language-server `$schema` tag sweep miss any files?

## Hypothesis

Mostly rename-and-rewire: repo-guardian is also a Go GitHub-App service with
Postgres + Valkey/Redis, so its chart shape (baked/CNPG/external store modes,
baked/external queue, ServiceMonitor/PrometheusRule, tailscale option) should
map onto docz-api with Meilisearch as the one known structural addition, and the
workflows should need only repo-name substitutions.

## Context

**Triggered by:** the `feat/helm-chart` branch — goal is to ship the docz-api
container image and an OCI Helm chart to GHCR (ECR optional/gated), with a chart
good enough to run on homelab infra (tailscale-fronted there, but not required
by the chart). Sources copied: `charts/repo-guardian` from
`github.com/donaldgifford/repo-guardian`, plus that repo's CI/publish workflows
and `contrib/` assets; the `deploy/dev` monitoring stack and keycloak realm
turned out to come from a second upstream (`rfc-api` — see Observation 5). A
follow-up design doc for OTel middleware is expected separately (see References
for the repo-guardian examples to model it on).

This audit is the review gate before any adaptation work: every finding below is
grounded in file:line reads of the branch and the authoritative runtime surface
in `internal/config`, `internal/telemetry`, `internal/auth*`, and
`cmd/docz-api`.

## Approach

1. Inventory the branch: committed (`f175b84` schema tags, `78551ac` helm
   scaffolding, `badaf5e` bake rewrite) + untracked chart/deploy/contrib files.
2. Establish docz-api's real surface: required/optional env vars
   (`internal/config/config.go`, `validate.go`), metric names
   (`internal/telemetry/metrics.go`), ports/probes (`cmd/docz-api/main.go`),
   auth flow (`internal/authhttp`), container identity (`Dockerfile`, distroless
   nonroot).
3. Read every file under `charts/docz-api/`, `deploy/dev/`, `contrib/`, the four
   new workflows, `docker-bake.hcl` (vs the `main` version), `ct.yaml`,
   `charts/.yamllint.yml`, `mise.toml`, and the justfile; diff the modified
   compose files.
4. For the schema-tag question: enumerate all `*.yml`/`*.yaml` lacking a
   `yaml-language-server` modeline, then verify candidate schemas actually exist
   on SchemaStore (HTTP status check).

## Environment

| Component         | Version / Value                                                                                                                                                                         |
| ----------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Branch            | `feat/helm-chart` (3 commits over `main`@`ce5b78a`)                                                                                                                                     |
| helm (mise)       | 4.2.2 (+ helm-ct, helm-diff, helm-docs, cosign, promtool — new in `mise.toml`)                                                                                                          |
| Chart under audit | `charts/docz-api` `version: 0.1.0`, `appVersion: "0.6.1"`                                                                                                                               |
| docz-api metrics  | `docz_api_http_requests_total`, `docz_api_http_request_duration_seconds`, `docz_api_ingest_jobs_total`, `docz_api_ingest_job_duration_seconds` (+ Go/process collectors) — nothing else |
| docz-api ports    | everything on `:8080` (`HTTP_ADDR`): API, `/healthz`, `/readyz`, `/metrics`, `/openapi.yaml`. No separate metrics/admin port                                                            |
| Container         | distroless `nonroot`, UID **65532**, read-only rootfs                                                                                                                                   |

## Findings

### Observation 0 — docz-api's authoritative surface (baseline)

Everything else in this doc is measured against this.

**Required env (config validation fails fast without):** `DATABASE_URL`,
`REDIS_URL`, `MEILI_HOST`, `MEILI_API_KEY`, `GITHUB_APP_ID`,
`GITHUB_APP_PRIVATE_KEY` (PEM body **or** file path — `resolvePrivateKey` reads
a file when the value doesn't start with `-----BEGIN`), `GITHUB_WEBHOOK_SECRET`,
`SESSION_SECRET`, `AUTH_REDIRECT_BASE`, and — since `AUTH_PROVIDERS` defaults to
`github` — `GITHUB_OAUTH_CLIENT_ID` + `GITHUB_OAUTH_CLIENT_SECRET`.

**Optional (defaulted):** `GITHUB_API_BASE`, `AUTH_PROVIDERS`,
`OKTA_*`/`KEYCLOAK_ISSUER`/`KEYCLOAK_CLIENT_ID`/`KEYCLOAK_CLIENT_SECRET` (only
when that provider is listed), `SESSION_TTL` (720h), `INGEST_DEBOUNCE` (5s),
`HTTP_ADDR` (`:8080`), `LOG_LEVEL`, `LOG_FORMAT` (text|json),
`OTEL_SERVICE_NAME`, `OTEL_EXPORTER_OTLP_ENDPOINT` (empty = tracing off),
`OTEL_SAMPLE_RATE`, `METRICS_ENABLED` (true).

Config is env-only via viper `AutomaticEnv()`: unknown vars are silently
ignored, missing required ones abort startup with `ErrInvalidConfig`. OTLP
traces go out over **HTTP** to `:4318` `/v1/traces`; metrics are **pull**
(`/metrics` on `:8080`) — the binary never emits OTLP metrics or OTLP logs.

### Observation 1 — yaml-language-server schema tags

The sweep (`f175b84`) tagged all workflows, root linter configs,
`catalog-info.yaml`, `.goreleaser.yml`, both deploy compose files,
`charts/.yamllint.yml`, `Chart.yaml`, `compose.dev.yaml`, and `ct.yaml`.

**Missed (schemas verified to exist on SchemaStore):**

- root `compose.yaml` — `compose-spec.json` (the two deploy compose files got
  it; the root one didn't)
- `.codecov.yml` — `https://json.schemastore.org/codecov.json`
- `sqlc.yaml` — `https://json.schemastore.org/sqlc-2.0.json`

**Wrong:** `ct.yaml` is tagged with
`https://json.schemastore.org/helm-testsuite.json` — that is the
**helm-unittest** suite schema, not chart-testing config. `chart-testing.json`
does not exist on SchemaStore (verified 404), so the tag should be removed;
`helm-testsuite.json` belongs on `charts/docz-api/tests/*_test.yaml` instead
(currently untagged).

**Worth adding:** `charts/docz-api/values.yaml` → its own sibling
(`# yaml-language-server: $schema=values.schema.json`);
`contrib/prometheus/alerts.yaml` → `prometheus.rules.json` (exists).

**Correctly left untagged:** helm `templates/` (Go-templated, not valid YAML),
`api/openapi.yaml` (served verbatim at `GET /openapi.yaml` — an editor modeline
would ship in the wire artifact), `.docz.yaml` / `api/vacuum-ruleset.yaml` /
grafana provisioning / otel collector config (no published schemas).

### Observation 2 — the Helm chart is a repo-guardian shell

Only `Chart.yaml` (name/description/home) and most of `cliff.toml` were
rebranded. Every template, helper, env var, secret slot, backing service, test,
and doc is still repo-guardian. Zero occurrences of `DATABASE_URL`, `REDIS_URL`,
`MEILI_*`, `SESSION_SECRET`, `AUTH_*`, `GITHUB_OAUTH_*`, `HTTP_ADDR`, `OTEL_*`,
or `METRICS_ENABLED` anywhere in the chart.

**2a. It does not render.** The templates carry two naming lineages:
repo-guardian-copied files use `repo-guardian.*` helpers (defined in
`_helpers.tpl`), while four fresh `helm create`-style files reference
**undefined** `docz-api.*` helpers — `templates/tests/test-connection.yaml`,
`hpa.yaml`, `ingress.yaml`, `httproute.yaml`. The latter three are gated behind
values keys that don't exist (`autoscaling.*`, `ingress.*`, `httpRoute.*` — nil
→ false, dormant), but **test-connection has no guard**, so `helm template` /
`helm lint` / `ct install` fail immediately on the undefined include. They also
reference `.Values.service.port`, which doesn't exist (values has
`service.httpPort`/`metricsPort`).

**2b. Zero config overlap.** The Deployment sets ~35 env vars; docz-api consumes
exactly three (`GITHUB_APP_ID`, `GITHUB_WEBHOOK_SECRET`, `LOG_LEVEL`). The fatal
mismatches:

| Chart sets (repo-guardian)                       | docz-api needs           | Effect                                                                                             |
| ------------------------------------------------ | ------------------------ | -------------------------------------------------------------------------------------------------- |
| `STORE_DSN` (from baked PG secret)               | `DATABASE_URL`           | Postgres deployed but unreachable                                                                  |
| `QUEUE_VALKEY_DSN`                               | `REDIS_URL`              | Valkey deployed but unreachable                                                                    |
| `GITHUB_PRIVATE_KEY_PATH` / `GITHUB_PRIVATE_KEY` | `GITHUB_APP_PRIVATE_KEY` | mounted PEM never seen (remapping the same mount works — our var accepts a path)                   |
| `LISTEN_ADDR`                                    | `HTTP_ADDR`              | ignored; `:8080` only matches by default-luck — retuning `config.port` would NOT move the listener |

Required vars with **no way to set them at all** (no value, no secret slot):
`MEILI_HOST`, `MEILI_API_KEY`, `SESSION_SECRET`, `AUTH_REDIRECT_BASE`,
`GITHUB_OAUTH_CLIENT_ID`, `GITHUB_OAUTH_CLIENT_SECRET`. ~30 other repo-guardian
vars (`DRY_RUN`, `WORKER_COUNT`, `DISCOVERY_*`, `GUARDIAN_CONFIG`,
`WEBHOOK_IP_ALLOWLIST*`, …) are dead weight viper silently ignores. Net: a live
install deploys Postgres + Valkey and then **CrashLoopBackoffs on config
validation**.

**2c. Meilisearch does not exist in the chart.** No workload, no
external-endpoint values, no `MEILI_*` env, no secret slot, no subchart
(Chart.yaml has no `dependencies:` — all backing services are hand-rolled
templates). This is the largest structural gap.

**2d. Secret template (`secret.yaml`) has only three slots** (`app-id`,
`webhook-secret`, `private-key`). Needs slots for `SESSION_SECRET`,
`MEILI_API_KEY`, `GITHUB_OAUTH_CLIENT_SECRET` (and values plumbing for the
non-secret `AUTH_REDIRECT_BASE`, `GITHUB_OAUTH_CLIENT_ID`).

**2e. Observability wiring is wrong.** The chart assumes repo-guardian's split
ports: container/Service port `metrics: 9090` + ServiceMonitor scraping port
`metrics` — dead against docz-api (`/metrics` is on `:8080`, gated by
`METRICS_ENABLED`). The PrometheusRule ships 8 `RepoGuardian*` alerts over
`repo_guardian_*` metrics we never emit (permanently inert or spuriously firing
for `== 0`-style expressions). **Probes are correct** — `/healthz` + `/readyz`
on port `http` ✓.

**2f. Identity details.**
`image.repository: ghcr.io/donaldgifford/repo-guardian` (would pull the wrong
binary); `appVersion: "0.6.1"` (docz-api is at v0.4.x); `runAsUser: 65534`
(nobody) instead of distroless nonroot **65532** (also missing
`runAsGroup`/`fsGroup`, `allowPrivilegeEscalation: false`,
`capabilities.drop: [ALL]`, `seccompProfile`); Postgres DB/user hardcoded
`repoguardian` (+ `sslmode=disable`); mount paths under `/etc/repo-guardian/…`;
`NOTES.txt` announces "repo-guardian has been deployed"; tailscale values
default `hostname: repo-guardian` and force-set repo-guardian-only env when
enabled.

**2g. Tests and docs.** The helm-unittest suite (10 files) asserts repo-guardian
strings — several already fail (hardcoded `RELEASE-NAME-repo-guardian-*` names
now render as `…-docz-api-*`; an image assertion expects `repo-guardian:1.9.0`
vs appVersion 0.6.1), and the passing remainder tests repo-guardian features (PR
templates, `guardian.hcl` policy, discovery). `ci/ci-values.yaml` (busybox +
sleep for ct install) can't save `ct` from the 2a render error.
`README.md`/`README.md.gotmpl`/`docs/*` are repo-guardian prose end-to-end
(including cosign verify examples with `repo-guardian` cert-identity regexes);
`values.schema.json` is titled "repo-guardian Helm chart values" and only
constrains repo-guardian concepts (`store`/`queue`/`scheduler`/`discovery`
enums); `CHANGELOG.md` header and `cliff.toml`'s emitted `[changelog].header`
still say repo-guardian.

**2h. Worth keeping.** The store/queue _shape_ is good: `store.postgres.mode` =
baked | cnpg | external (CNPG Cluster + PgBouncer Pooler CRs included) and
`queue.valkey.mode` = baked | external. Valkey is Redis-wire-compatible and
docz-api uses one Redis for queue + sessions, so a single Valkey works. The
adaptation is rename + rewire + add a Meilisearch analog (probably the same
baked | external pattern), not a rearchitecture.

### Observation 3 — CI/release workflows: duplication, make-vs-just, racing bumps

**3a. `make` → `just`.** `ci2.yml` runs `make test-coverage` (test-go job) and
`make lint-alerts` (lint-alerts job). There is no Makefile; the repo convention
is `just`, and no `lint-alerts` or helm recipes exist in the justfile yet (the
tools are now in `mise.toml`; the recipes are the gap — needed: `lint-alerts`
via promtool, helm lint/unittest/docs, plus whatever `fmt` should cover for the
chart).

**3b. Duplicate CI.** `ci2.yml` and the existing `ci.yml` are both `name: CI` on
identical triggers with overlapping jobs (both carry a Label PR job;
lint/test/build duplicated). ci2 adds real value the current CI lacks:
`dorny/paths-filter` change gating, helm lint, helm-unittest, chart-testing with
a kind cluster, alert linting, and a docker bake build (PRs push a `:dev`
multi-arch image to GHCR). ci.yml has things ci2 lacks: Trivy scan, SBOM on the
goreleaser snapshot, `just`-based invocations, the openapi lint. These need
merging, not coexistence.

**3c. Racing releases.** `release2.yml` and the existing `release.yml` are
**both** on main-push and both run `jefflinse/pr-semver-bump` — two runs racing
to create the same tag on every merge. `release.yml` is _already_ the
label-driven bump model; `release2.yml` differs only by: GPG key import (needs
`GPG_PRIVATE_KEY` + `GPG_FINGERPRINT` secrets — not verified to exist, and
`.goreleaser.yml` has no signing config today), newer action pins, and the
`publish-ghcr` / `publish-ecr` reusable-workflow calls. The right merge is
grafting the publish jobs (and optionally GPG) into `release.yml` and deleting
`release2.yml`.

**3d. The publish workflows themselves are sound.** `ghcr.yml` / `ecr.yml` are
already docz-api-named (image `ghcr.io/donaldgifford/docz-api`, chart
`oci://ghcr.io/donaldgifford/charts/docz-api`), idempotent on chart version
(`helm pull` precheck), cosign-signed, SLSA-attested, and chart-only publishes
work on `dont-release` PRs by design. ECR is gated off behind
`vars.ECR_PUBLISH_ENABLED` — fine — but its comment references
`docs/operations/ecr-publish-setup.md`, which was not copied over (the chart's
`docs/publishing-to-ecr.md` is the repo-guardian equivalent, itself needing
rebrand or deletion).

### Observation 4 — docker-bake.hcl dropped the build args

The rewrite (`badaf5e`, repo-guardian's shape) defines `VERSION` / `COMMIT_SHA`
/ `BUILD_DATE` variables and uses them in OCI **labels**, but the old file's
`args = { VERSION = "${VERSION}" }` block was lost — the Dockerfile's
`ARG VERSION/COMMIT/DATE → -ldflags -X main.version…` never receives values.
Every baked image (including release publishes) compiles in
`version=dev, commit=none, date=unknown`. Two fixes needed: an `args` block in
`_common` mapping all three, **and** the publish workflows must export
`VERSION`/`COMMIT_SHA`/`BUILD_DATE` env when invoking `docker/bake-action`
(metadata-action's bake files fix tags/labels only, not the compiled-in
version).

### Observation 5 — deploy/dev monitoring stack (rfc-api lineage)

Framing surprise: this tier is **not from repo-guardian** — the compose doc,
`deploy/dev/**`, and the keycloak realm are from a project named `rfc-api`
(realm/clients/dashboards/scrape jobs all say so). So the branch carries two
foreign vocabularies, neither matching docz-api.

**5a. `deploy/compose.dev.yaml` is malformed** — two YAML documents in one file
(`---` at line ~118). Compose treats a file as one document. Doc 1 is a
**verbatim duplicate of `compose.local.yaml`** (same `name: docz-api-local` —
would collide with the real local env; loads `.env.local`, not `.env.dev`). Doc
2 (`name: rfc-api`) is the observability stack: postgres:18 (creds `rfcapi`),
meilisearch:v1, keycloak 26 (profile `auth`, port 8180), otel-collector + jaeger
(profile `tracing`, 4317/4318/16686), prometheus + grafana (profile `metrics`,
9090/3000), loki + alloy (profile `logs`, 3100/12345) — duplicating
postgres/meili that every existing compose already provides.

**5b. All five Doc-2 bind mounts are broken**: `./deploy/dev/…` from a file
already inside `deploy/` resolves to `deploy/deploy/dev/…` (nonexistent), and
the prometheus mount also says `prometheus.yml` while the real file is
`prometheus.yaml`. Header comments cite a Makefile with `compose-*` targets and
`docs/development/local-dev.md` — neither exists here.

**5c. Orphans and dead targets.** `deploy/.env.dev.example` is byte-identical to
`.env.local.example` and referenced by nothing. The prometheus scrape job
targets `host.docker.internal:8081` (rfc-api's admin port) — docz-api serves
`/metrics` on `:8080`; the job/labels say `rfc-api`. No just recipe invokes any
of this.

**5d. Keycloak realm can't log docz-api in.** Realm `rfc-api` with two clients:
`rfc-api` (bearer-only, standard flow disabled — cannot do interactive login)
and `rfc-site` (public SPA, PKCE, redirects `localhost:3001/5173`). docz-api's
keycloak provider is a **confidential** OIDC web-app doing the auth-code flow to
`<AUTH_REDIRECT_BASE>/auth/callback`
(`KEYCLOAK_ISSUER`/`KEYCLOAK_CLIENT_ID`/`KEYCLOAK_CLIENT_SECRET`). Any login
attempt dies on `invalid redirect_uri`. The realm needs a docz-api-named
confidential client with `standardFlowEnabled: true` and redirect
`http://localhost:8080/auth/callback` (and `.env` plumbing to add `keycloak` to
`AUTH_PROVIDERS` + the three `KEYCLOAK_*` vars).

**5e. What already works.** The otel-collector listens on **4318 HTTP** —
matches our `otlptracehttp` exporter exactly, so
`OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4318` (or
`host.docker.internal:4318` for the host-run loop) delivers traces to Jaeger
today with zero code changes. The collector's metrics pipeline is dead for us
(we're pull-based; its prometheusremotewrite target also lacks
`--web.enable-remote-write-receiver` on the prometheus command), and the
alloy→loki log path needs `LOG_FORMAT=json` (and expects
`trace_id`/`span_id`/`http.route` keys in the JSON — a fit check for the future
OTel-middleware design).

**5f. Grafana.** The provisioned dashboard (`docz-api-overview.json`, titled
"rfc-api overview") queries unprefixed names (`http_requests_total`,
`http_request_duration_seconds_bucket`) — wrong; ours carry the `docz_api_`
prefix. Its in-flight panel queries `http_requests_in_flight`, which docz-api
has no analog for (drop it or add the gauge). The `status=~"5.."` regexes are
compatible with our full-code `status` label once names are fixed. Nothing
covers our ingest metrics. Datasource UID bug: panels bind
`uid: "prometheus"`/`"loki"`, but `datasources.yaml` only pins an explicit uid
for Jaeger — Prometheus/Loki get auto-generated UIDs, so those panels fail to
bind even after query fixes.

### Observation 6 — contrib/ dashboard + alerts (repo-guardian lineage)

All three files are 100% repo-guardian vocabulary:

- `contrib/grafana/docz-api-dashboard.json` — titled "Repo Guardian", uid
  `repo-guardian`, ~40 panels, every query `repo_guardian_*` (none exist here).
  Uses import-time `${DS_PROMETHEUS}`, so it's an import-and-pick dashboard
  rather than a provisioned one.
- `contrib/prometheus/alerts.yaml` — 20 alerts across four groups
  (`repo-guardian`, `.multi-replica`, `.pr-convergence`, `.discovery`), every
  expression on `repo_guardian_*` metrics; one description says "Check Valkey"
  (we run Redis); references `guardian.hcl` and `docs/operations/scaling.md`
  (neither exists here).
- `contrib/README.md` — operates repo-guardian: ~50 `repo_guardian_*` metric
  docs, `:9090/metrics` default, and import instructions pointing at
  `contrib/grafana/repo-guardian-dashboard.json` — a filename that doesn't exist
  on disk (the file was renamed to `docz-api-dashboard.json` without updating
  the README).

A docz-api-true contrib set is small: four app metrics + Go/process defaults.
Realistic alerts: instance down / no scrapes, 5xx rate, request p99, ingest
failure ratio + duration p99 by reason, absence-of-webhook-driven-ingests. The
dashboard needs the RED panels renamed to `docz_api_*` plus an ingest row; the
~35 repo-guardian panels (queue depth, PR convergence, discovery, budget,
rate-limit) have no docz-api analog and should be deleted rather than
translated.

## Conclusion

**Answer: No — not usable as-is.** The hypothesis ("mostly rename-and-rewire")
held for the chart's _skeleton_ (store/queue modes, publish workflows, probes)
but underestimated three hard breaks: the chart does not even render
(`helm template` fails on undefined `docz-api.*` helpers), the config surface
overlap is effectively zero (an install CrashLoopBackoffs on validation, with
six required vars unsettable), and Meilisearch — a hard runtime dependency — is
entirely absent from the chart. On top of that, both workflow pairs (`ci`/`ci2`,
`release`/`release2`) would run duplicated (the release pair racing to create
the same tag), `ci2` calls Makefile targets that don't exist, the bake rewrite
silently ships `version=dev` binaries in every image, the dev observability
stack is an unrunnable two-document compose file with broken mounts and a
keycloak realm that rejects our login flow, and every dashboard/alert queries
metrics docz-api does not emit. The schema-tag sweep missed three files and
mis-tagged one.

## Recommendation

Four workstreams, roughly in this order:

1. **Quick-fix batch (no design needed, ~one PR):** schema tags (add root
   `compose.yaml`, `.codecov.yml`, `sqlc.yaml`; fix `ct.yaml`; tag the
   helm-unittest files), restore `args` in `docker-bake.hcl` `_common` + export
   `VERSION`/`COMMIT_SHA`/`BUILD_DATE` in the bake-invoking workflow steps, and
   delete or park the orphan `deploy/.env.dev.example`.
2. **CI/release consolidation:** merge `ci2.yml` into `ci.yml` (keep ci2's
   paths-filter gating + helm/docker/alerts jobs; keep ci.yml's
   Trivy/SBOM/just/openapi pieces; make→just throughout; add justfile recipes
   `lint-alerts`, `helm-lint`, `helm-unittest`, `helm-docs`), graft
   `release2.yml`'s `publish-ghcr`/`publish-ecr` jobs into `release.yml`, delete
   both `*2` files. Decide GPG signing (create secrets + goreleaser config) or
   drop that step for now. Port `ecr-publish-setup` docs or strip the reference.
3. **Chart rewrite (the core work — DESIGN + IMPL docs recommended, then run the
   loop):** unify helpers under `docz-api.*`; rewire env to the Observation 0
   surface; add Meilisearch (baked | external, mirroring the store/queue
   pattern) + secret slots (`SESSION_SECRET`, `MEILI_API_KEY`,
   `GITHUB_OAUTH_CLIENT_SECRET`) + values for `AUTH_REDIRECT_BASE`/OAuth client
   id/providers; metrics on `:8080` (drop port 9090, fix
   Service/ServiceMonitor); `HTTP_ADDR` driven from `config.port`; UID 65532
   - hardened securityContext; rename DB identity; guard or fix
     test-connection/hpa/ingress/httproute; rewrite PrometheusRule alerts,
     values.schema.json, unittest suite, NOTES/README/docs; decide tailscale
     keep-as-option vs drop (infra uses tailscale, chart shouldn't require it —
     keeping the option seems right); bump `appVersion` to the real docz-api
     release and repoint `image.repository`. Design OQs: baked-vs-external
     default for Meilisearch, keep CNPG mode?, chart-version vs app-version
     policy.
4. **Observability assets:** split `compose.dev.yaml` doc 2 into a proper
   profile-gated monitoring compose (fix the five mounts, drop the duplicated
   doc 1 and duplicate postgres/meili — compose profiles or an `include` of the
   local stack are both viable), rebrand rfc-api→docz-api, pin Prometheus/Loki
   datasource UIDs, fix the keycloak realm (confidential client,
   `/auth/callback` redirect, `KEYCLOAK_*` env plumbing), and rewrite `contrib/`
   (dashboard RED+ingest panels on `docz_api_*`, ~5 real alerts, README) — with
   just recipes to drive it. The OTel middleware design doc (traces already work
   against the collector; log-correlation keys are the open piece) stays a
   separate follow-up as planned.

## References

- Branch: `feat/helm-chart` — commits `f175b84`, `78551ac`, `badaf5e` +
  untracked chart/deploy/contrib files
- Upstream chart: `github.com/donaldgifford/repo-guardian`
  (`charts/repo-guardian`, `contrib/`, workflows)
- OTel design examples to model the follow-up on:
  [repo-guardian DESIGN-0010](https://github.com/donaldgifford/repo-guardian/blob/main/docs/design/0010-per-org-rule-scoping-and-observability.md),
  [repo-guardian IMPL-0009](https://github.com/donaldgifford/repo-guardian/blob/main/docs/impl/0009-per-org-rule-scoping-and-observability.md)
- Authoritative surfaces: `internal/config/config.go` + `validate.go`,
  `internal/telemetry/metrics.go`, `cmd/docz-api/main.go`,
  `internal/authhttp/handler.go`, `Dockerfile`
- Related: DESIGN-0001 (service architecture), IMPL-0002 (OpenAPI contract —
  `/openapi.yaml` served verbatim, why it stays unmodelined)
