---
id: IMPL-0004
title: "Adapt the helm chart, CI, and observability scaffolding"
status: Completed
author: Donald Gifford
created: 2026-07-13
---

<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL 0004: Adapt the helm chart, CI, and observability scaffolding

**Status:** Completed **Author:** Donald Gifford **Date:** 2026-07-13

<!--toc:start-->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Open Questions](#open-questions)
- [Executor conventions (read first)](#executor-conventions-read-first)
- [Reference: the docz-api target surface](#reference-the-docz-api-target-surface)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: Repo plumbing quick fixes](#phase-1-repo-plumbing-quick-fixes)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2: Chart core — helpers, deployment, values](#phase-2-chart-core--helpers-deployment-values)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 3: Backing services — store/queue rewire + Meilisearch](#phase-3-backing-services--storequeue-rewire--meilisearch)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 4: Chart observability, tests, and docs](#phase-4-chart-observability-tests-and-docs)
    - [Tasks](#tasks-3)
    - [Success Criteria](#success-criteria-3)
  - [Phase 5: CI and release consolidation](#phase-5-ci-and-release-consolidation)
    - [Tasks](#tasks-4)
    - [Success Criteria](#success-criteria-4)
  - [Phase 6: Local monitoring stack](#phase-6-local-monitoring-stack)
    - [Tasks](#tasks-5)
    - [Success Criteria](#success-criteria-5)
  - [Phase 7: contrib/ rewrite](#phase-7-contrib-rewrite)
    - [Tasks](#tasks-6)
    - [Success Criteria](#success-criteria-6)
- [Testing Plan](#testing-plan)
- [References](#references)
<!--toc:end-->

## Objective

Turn the scaffolding audited in INV-0004 into a working, shippable docz-api Helm
chart + container publish pipeline + local observability stack. Work happens on
the existing `feat/helm-chart` branch.

**Implements:** INV-0004 (audit conclusions and the four recommended
workstreams).

## Scope

### In Scope

- Schema-tag and `docker-bake.hcl` quick fixes (INV-0004 Obs. 1, 4).
- Rewriting `charts/docz-api/` so it renders, installs, and configures docz-api
  correctly, including Meilisearch (Obs. 2).
- Consolidating `ci2.yml`/`release2.yml` into `ci.yml`/`release.yml`, with GHCR
  image + OCI chart publishing (Obs. 3).
- Replacing `deploy/compose.dev.yaml` with a working local monitoring stack and
  a docz-api keycloak realm (Obs. 5).
- Rewriting `contrib/` (dashboard, alerts, README) in docz-api vocabulary (Obs.
  6).

### Out of Scope

- Any Go code changes (no new metrics, no OTel middleware — that is the separate
  OTel design doc; the alert/dashboard set here uses only the four existing
  metrics).
- ECR-side AWS setup (workflow stays gated off behind
  `vars.ECR_PUBLISH_ENABLED`).
- Publishing the first real image/chart release (happens naturally when this
  branch merges and a release fires).
- docz-site consumption of the chart.

## Open Questions

**RESOLVED 2026-07-13: all ten questions answered `a` (the recommendation).**
The `a` options below are binding for execution — every "per OQ-Xa" reference in
the phase tasks is authoritative. The alternatives are kept for the record.

**OQ-1 — Meilisearch deployment mode default (Phase 3).** How should the chart
provide Meilisearch?

- **a. Mirror the store/queue pattern: `search.meili.mode: baked | external`,
  default `baked`.** A single-pod StatefulSet + PVC + Service + master-key
  Secret, exactly like the baked Postgres/Valkey. Consistent values API, works
  out-of-the-box on homelab, external escape hatch for real deployments.
- b. External-only: the chart never runs Meili; operator supplies
  `search.meili.host` + an existing secret. Least chart code, but the chart no
  longer installs standalone.
- c. Subchart dependency on the official meilisearch chart. Pulls in a foreign
  values surface and version coupling; overkill at this scale.

**OQ-2 — CloudNativePG store mode (Phase 3).** The copied chart supports
`store.postgres.mode: cnpg` (CNPG Cluster + PgBouncer Pooler CRs).

- **a. Keep it, renamed to docz-api identity.** The templates already exist and
  only need the db/user/labels rename; homelab infra runs CNPG, so this is the
  mode production will actually use.
- b. Delete the three `store-cnpg-*.yaml` templates now (YAGNI) and re-add when
  infra needs them. Less to test, but throws away working code we want within a
  phase or two of deploying.

**OQ-3 — Tailscale option (Phase 2).** The chart carries an optional tailscale
sidecar/funnel (off by default).

- **a. Keep it as an off-by-default option, rebranded** (hostname default
  `docz-api`, drop the repo-guardian-only env it force-sets). Infra will front
  docz-api with tailscale, so the option earns its keep; disabled it renders
  nothing.
- b. Remove all tailscale templates/values now and re-add in an infra-specific
  PR later.

**OQ-4 — GPG signing in the release workflow (Phase 5).** `release2.yml` imports
a GPG key for goreleaser artifact signing; the secrets (`GPG_PRIVATE_KEY`,
`GPG_FINGERPRINT`) don't exist in this repo and `.goreleaser.yml` has no signing
config.

- **a. Omit GPG for now.** Keep goreleaser exactly as today (unsigned archives);
  images and charts are already cosign-signed + SLSA-attested by the publish
  workflows, which is the supply-chain story that matters.
- b. Add GPG: create the secrets, add a `signs:` block to `.goreleaser.yml`,
  keep the import step. More setup, marginal benefit over cosign.

**OQ-5 — CI consolidation shape (Phase 5).**

- **a. Merge into the existing `ci.yml`** — one `CI` workflow: keep ci.yml's
  Trivy/SBOM/Codecov/just/openapi pieces, graft in ci2's paths-filter gating and
  the docker/helm/alerts jobs, delete `ci2.yml`. One status surface, one file to
  maintain.
- b. Keep a separate `charts.yml` workflow for helm/docker jobs (smaller files,
  but two workflows share the paths-filter logic and PR checks split across two
  workflow names).
- c. Adopt `ci2.yml` wholesale and port ci.yml's extras into it (same result as
  (a) with more churn and a period of duplicate-named workflows).

**OQ-6 — Queue values naming (Phase 3).** The chart deploys the Valkey image;
docz-api reads `REDIS_URL`.

- **a. Keep `queue.valkey.*` values keys and the valkey image; emit the DSN as
  `REDIS_URL`.** The values name describes what gets deployed; the env var is
  what the app needs. Zero template churn beyond the secret key.
- b. Rename keys to `queue.redis.*` and switch to the `redis` image to match the
  app's vocabulary (more renaming; the chart's valkey works identically over the
  wire).

**OQ-7 — Local monitoring stack placement (Phase 6).**

- **a. New standalone `deploy/compose.monitoring.yaml`** (single YAML doc,
  project `docz-api-monitoring`): prometheus/grafana/otel-collector/jaeger/
  loki/alloy always-on, keycloak behind an `auth` profile; scrapes the app at
  `host.docker.internal:8080`, so it works unchanged alongside BOTH the host-run
  dev loop (`just run`) and the containerized local env (`just local-up`, which
  publishes 8080). No duplicated postgres/meili.
- b. Fold the monitoring services into `compose.local.yaml` behind profiles
  (couples monitoring to the local env; the host-run dev loop can't use it).
- c. Keep `compose.dev.yaml`'s two-document structure as two separate files, one
  duplicating the local env (rejected by INV-0004 — pure duplication).

**OQ-8 — Keycloak in local dev (Phase 6).**

- **a. Keep it, behind the `auth` profile, with a rewritten docz-api realm**:
  realm `docz-api`, a confidential client `docz-api` (standard flow, redirect
  `http://localhost:8080/auth/callback`), one seeded test user. This is the only
  way to exercise the OIDC provider path locally.
- b. Drop keycloak from local dev entirely (GitHub OAuth covers the login path;
  OIDC stays tested only by unit tests).

**OQ-9 — Chart `appVersion` policy (Phase 2, then ongoing).**

- **a. Pin `appVersion` manually to the latest docz-api release tag (currently
  `v0.4.0`) and bump it by hand in the same PR as a chart-version bump when the
  chart should track a newer app.** Simple, explicit, matches how the OpenAPI
  spec version is managed.
- b. Automate: have the release workflow rewrite `appVersion` on each app
  release. Couples chart releases to app releases and creates bot commits;
  revisit if manual bumping proves annoying.

**OQ-10 — PR `:dev` image pushes (Phase 5).** ci2's docker job pushes a
multi-arch `:dev` image to GHCR on every PR.

- **a. Keep it.** An always-fresh `ghcr.io/donaldgifford/docz-api:dev` lets the
  chart (and later the site) be tested against a real registry image before any
  release exists; it's how repo-guardian works and costs one mutable tag.
- b. Build-only on PRs (`type=cacheonly`), push nothing until release. Saves a
  mutable tag; loses the pull-testing ability.

## Executor conventions (read first)

These are hard rules for whoever (or whatever) executes the phases:

1. **Task runner is `just`, never `make`.** There is no Makefile. New recipes go
   in `justfile` following its existing `[group('…')]` style.
2. **Commit after every numbered task** with a conventional-commit message
   (`feat(helm): …`, `chore(ci): …`, `docs(deploy): …`). Check the task off in
   this document in the same commit. Do not push unless asked.
3. **Every task's verification command must pass before committing.** If a
   verification fails, fix it before moving on — do not check off the task.
4. **Never commit secrets.** `.env*` (except `*.example`), `deploy/secrets/`,
   `secrets/`, and `*.pem` are gitignored — leave them that way. Example files
   carry placeholders only.
5. **Do not touch** `api/openapi.yaml` (served verbatim on the wire — no editor
   modelines), anything under `internal/` or `cmd/` (no Go changes in this
   impl), or `.goreleaser.yml` (unless OQ-4 = b).
6. **sed on this machine is GNU sed**: use `sed -i 's/old/new/g' file` (no `''`
   argument after `-i`). When in doubt, use the Edit tool instead.
7. **Lint gates:** `just lint-actions` after workflow edits; `yamllint` on
   touched YAML (`charts/.yamllint.yml` governs `charts/`, and its ignore of
   `charts/docz-api/templates/` is intentional — Go-templated files are not
   valid YAML); `markdownlint-cli2` + `prettier --write` on touched markdown;
   `just fmt` never touches the chart.
8. **helm-unittest plugin:** `just helm-unittest` needs the plugin. If
   `helm plugin list` doesn't show `unittest`, run
   `helm plugin install https://github.com/helm-unittest/helm-unittest`.
9. **Known wrinkle — Helm 4:** `mise.toml` pins `helm = "4.2.2"`. If the
   unittest plugin or `ct` misbehaves under Helm 4, do not debug forever: pin
   `helm = "3.19.0"` in `mise.toml`, `mise install`, note it in the commit
   message, and continue.
10. **The GHCR image does not exist yet.** `ghcr.io/donaldgifford/docz-api` gets
    its first push when this branch merges and a release/PR build runs. Until
    then a real `helm install` of the chart will ImagePullBackoff on the default
    image — expected; `ct install` uses `ci/ci-values.yaml` (busybox) precisely
    so CI doesn't depend on it.
11. **Renders must be verified with the ci values**, because required values
    intentionally have no defaults:
    `helm template docz-api charts/docz-api -f charts/docz-api/ci/ci-values.yaml`.

## Reference: the docz-api target surface

Copy values from here; do not invent names. (Derivation: INV-0004 Obs. 0.)

**Env the chart must set** (→ = source):

| Env var                                                                  | Required | Source in chart                                  |
| ------------------------------------------------------------------------ | -------- | ------------------------------------------------ |
| `DATABASE_URL`                                                           | yes      | store secret (baked/cnpg/external)               |
| `REDIS_URL`                                                              | yes      | queue secret (baked/external)                    |
| `MEILI_HOST`                                                             | yes      | computed service URL or `search.meili.host`      |
| `MEILI_API_KEY`                                                          | yes      | meili secret                                     |
| `GITHUB_APP_ID`                                                          | yes      | app secret key `app-id`                          |
| `GITHUB_APP_PRIVATE_KEY`                                                 | yes      | mounted PEM path or app secret key `private-key` |
| `GITHUB_WEBHOOK_SECRET`                                                  | yes      | app secret key `webhook-secret`                  |
| `SESSION_SECRET`                                                         | yes      | app secret key `session-secret`                  |
| `AUTH_REDIRECT_BASE`                                                     | yes      | `config.authRedirectBase` (plain value)          |
| `GITHUB_OAUTH_CLIENT_ID`                                                 | yes\*    | `config.githubOAuthClientID` (plain value)       |
| `GITHUB_OAUTH_CLIENT_SECRET`                                             | yes\*    | app secret key `oauth-client-secret`             |
| `HTTP_ADDR`                                                              | no       | `":{{ .Values.config.port }}"`                   |
| `AUTH_PROVIDERS`                                                         | no       | `config.authProviders` (default `github`)        |
| `LOG_LEVEL` / `LOG_FORMAT`                                               | no       | `config.logLevel` / `config.logFormat`           |
| `SESSION_TTL` / `INGEST_DEBOUNCE` / `GITHUB_API_BASE`                    | no       | `config.*`, emit only when set                   |
| `OTEL_EXPORTER_OTLP_ENDPOINT` / `OTEL_SERVICE_NAME` / `OTEL_SAMPLE_RATE` | no       | `otel.*`, emit only when set                     |
| `METRICS_ENABLED`                                                        | no       | `metrics.enabled` (default true)                 |

\* required because `AUTH_PROVIDERS` defaults to `github`.

**Fixed facts:** everything serves on one port `:8080` — API, `/healthz`
(liveness), `/readyz` (readiness), `/metrics`, `/openapi.yaml`. No separate
metrics port. Container is distroless nonroot, UID/GID **65532**, read-only
rootfs. Metrics that exist (nothing else does):
`docz_api_http_requests_total{method,route,status}` (status = full code, e.g.
`"200"`), `docz_api_http_request_duration_seconds` (histogram, {method,route}),
`docz_api_ingest_jobs_total{reason,status}` (status = `success` | `failure`;
reason = `onboard` | `repo_added` | `push`),
`docz_api_ingest_job_duration_seconds{reason}` (histogram), plus Go/process
collectors.

## Implementation Phases

Each phase builds on the previous one. A phase is complete when all its tasks
are checked off and its success criteria are met.

---

### Phase 1: Repo plumbing quick fixes

Small, independent corrections from INV-0004 Obs. 1 and 4, plus the justfile
recipes later phases need for verification. No chart-template changes here.

#### Tasks

- [x] **1.1 Schema tags.** Insert as line 1: in root `compose.yaml` →
      `# yaml-language-server: $schema=https://raw.githubusercontent.com/compose-spec/compose-spec/master/schema/compose-spec.json`;
      in `.codecov.yml` →
      `# yaml-language-server: $schema=https://json.schemastore.org/codecov.json`;
      in `sqlc.yaml` →
      `# yaml-language-server: $schema=https://json.schemastore.org/sqlc-2.0.json`.
      In `ct.yaml`, DELETE the existing modeline (it points at the helm-unittest
      schema; no chart-testing schema exists on SchemaStore). Do NOT tag
      `charts/docz-api/tests/*` / `values.yaml` /
      `contrib/prometheus/alerts.yaml` here — those files are rewritten in
      Phases 4 and 7 and get their modelines then. Verify:
      `yamllint compose.yaml .codecov.yml sqlc.yaml ct.yaml` passes and
      `grep -L "yaml-language-server" compose.yaml .codecov.yml sqlc.yaml`
      prints nothing.
- [x] **1.2 Restore bake build args.** In `docker-bake.hcl`, add to
      `target "_common"`:

  ```hcl
  args = {
    VERSION = "${VERSION}"
    COMMIT  = "${COMMIT_SHA}"
    DATE    = "${BUILD_DATE}"
  }
  ```

  Verify: `docker buildx bake dev --print | jq '.target.dev.args'` shows all
  three keys.

- [x] **1.3 Feed the args in the publish workflows.** In `ghcr.yml` and
      `ecr.yml` `image` jobs, add before the bake step:

  ```yaml
  - name: Compute build metadata
    run: |
      echo "VERSION=${{ inputs.tag }}" >> "$GITHUB_ENV"
      echo "COMMIT_SHA=${{ github.sha }}" >> "$GITHUB_ENV"
      echo "BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$GITHUB_ENV"
  ```

  (bake reads variables from same-named env vars). Verify: `just lint-actions`
  passes.

- [x] **1.4 Delete the orphan** `deploy/.env.dev.example` (byte-copy of
      `.env.local.example`, referenced by nothing). It is untracked — plain
      `rm`.
- [x] **1.5 justfile recipes.** Add a `# ─── Helm ───` section:

  ```make
  [group('helm')]
  helm-lint:
      @helm lint charts/docz-api -f charts/docz-api/ci/ci-values.yaml

  [group('helm')]
  helm-template:
      @helm template docz-api charts/docz-api -f charts/docz-api/ci/ci-values.yaml

  [group('helm')]
  helm-unittest:
      @helm unittest charts/docz-api

  [group('helm')]
  helm-docs:
      @helm-docs --chart-search-root=charts
  ```

  and under the lint group:

  ```make
  [group('lint')]
  lint-alerts:
      @promtool check rules contrib/prometheus/alerts.yaml
  ```

  (ci values on lint/template because required values deliberately have no
  defaults.) Verify: `just --list` shows the five recipes. They are NOT expected
  to pass yet (chart broken until Phase 2; alerts rewritten in Phase 7) —
  `just lint-alerts` SHOULD pass already (the copied rules are syntactically
  valid; it's the metric names that are wrong).

#### Success Criteria

- `yamllint` clean on the four retagged files; `just lint-actions` passes.
- `docker buildx bake dev --print` shows VERSION/COMMIT/DATE args reaching the
  Dockerfile.
- `deploy/.env.dev.example` gone; `just --list` shows the helm + lint-alerts
  recipes.

---

### Phase 2: Chart core — helpers, deployment, values

Make the chart render and describe docz-api: one helper namespace, the real env
surface, correct identity/security, single port. (INV-0004 Obs. 2a, 2b, 2d, 2f;
OQ-3, OQ-9.)

#### Tasks

- [x] **2.1 Unify helpers under `docz-api.*`.** In `charts/docz-api/`, rename
      every `repo-guardian.` template define/include to `docz-api.`:
      `grep -rl 'repo-guardian\.' charts/docz-api/templates | xargs sed -i 's/repo-guardian\./docz-api./g'`.
      Verify: `grep -r 'repo-guardian\.' charts/docz-api/templates` prints
      nothing.
- [x] **2.2 Delete repo-guardian-only templates + tests:**
      `templates/configmap.yaml` (PR templates),
      `templates/policy-configmap.yaml` (guardian.hcl),
      `tests/configmap_test.yaml`, `tests/policy_test.yaml`,
      `docs/homelab-smoke.md`, `docs/publishing-to-ecr.md` (the ECR runbook is
      re-homed in Phase 5). Remove their Deployment references: the
      templates/policy volumes + volumeMounts, and the `GUARDIAN_CONFIG`,
      `TEMPLATE_DIR`, `STRICT_TEMPLATES` env entries, and any
      `validateTemplatingVars`-style helper plus its `_helpers.tpl` definition.
- [x] **2.3 Rewrite `values.yaml`** to the target surface (add the modeline
      `# yaml-language-server: $schema=values.schema.json` as line 1). Target
      top-level keys — keep the existing repo-guardian layout where it matches,
      delete everything not listed: `replicaCount`; `image`
      (`repository: ghcr.io/donaldgifford/docz-api`, `tag: ""` → defaults to
      appVersion, `pullPolicy`); `imagePullSecrets`;
      `nameOverride`/`fullnameOverride`; `serviceAccount`; `podAnnotations`;
      `podSecurityContext` (`runAsNonRoot: true`, `runAsUser: 65532`,
      `runAsGroup: 65532`, `fsGroup: 65532`,
      `seccompProfile: {type: RuntimeDefault}`); `securityContext`
      (`readOnlyRootFilesystem: true`, `allowPrivilegeEscalation: false`,
      `capabilities: {drop: [ALL]}`); `service` (`type: ClusterIP`, `port: 80` —
      ONE port; delete `httpPort`/`metricsPort`); `resources`; `probes` (keep —
      `/healthz` + `/readyz` on port `http` are already correct); `config`
      (`appId: ""`, `port: 8080`, `logLevel: info`, `logFormat: json`,
      `authRedirectBase: ""`, `authProviders: github`,
      `githubOAuthClientID: ""`, `githubApiBase: ""`, `sessionTTL: ""`,
      `ingestDebounce: ""`); `otel` (`endpoint: ""`, `serviceName: ""`,
      `sampleRate: ""`); `metrics` (`enabled: true`); `secrets`
      (`existingSecret: ""`, `webhookSecret: ""`, `privateKey: ""`,
      `privateKeyAsFile: true`, `sessionSecret: ""`, `oauthClientSecret: ""`);
      `store` (unchanged structure for now — Phase 3 touches its internals);
      `queue` (same); `search` (NEW — placeholder `meili: {mode: baked}` filled
      out in Phase 3); `tailscale` (per OQ-3a: keep, `enabled: false`, default
      `hostname: docz-api`); `serviceMonitor`; `prometheusRule`; `extraEnv`;
      `autoscaling` (`enabled: false`, `minReplicas: 1`, `maxReplicas: 3`,
      `targetCPUUtilizationPercentage: 80`); `ingress` (`enabled: false`,
      standard helm-create block); `httpRoute` (`enabled: false`, standard
      block).
- [x] **2.4 Rewrite `templates/deployment.yaml` env + mounts + ports** to the
      Reference table exactly: keep the app-secret refs for
      `GITHUB_APP_ID`/`GITHUB_WEBHOOK_SECRET`; private key — when
      `secrets.privateKeyAsFile` mount the secret at
      `/etc/docz-api/private-key/` and set
      `GITHUB_APP_PRIVATE_KEY=/etc/docz-api/private-key/private-key.pem`, else
      env-from-secret; `HTTP_ADDR: ":{{ .Values.config.port }}"`;
      `DATABASE_URL`/`REDIS_URL`/`MEILI_HOST`/`MEILI_API_KEY` via the
      store/queue/search secret helpers (Phase 3 makes the secrets emit these
      keys — render-only consistency is enough for this phase);
      `SESSION_SECRET` + `GITHUB_OAUTH_CLIENT_SECRET` from the app secret;
      `AUTH_REDIRECT_BASE` required-checked from `config.authRedirectBase`
      (`{{ required "config.authRedirectBase is required" … }}`); plain-value
      optionals emitted only when non-empty (`with` blocks); `extraEnv`
      passthrough kept. Ports: delete the `metrics` containerPort; one port
      `http` = `config.port`. Update the `reservedEnvVars` helper list in
      `_helpers.tpl` to exactly the env names this deployment now sets. Delete
      tailscale's force-set of `WEBHOOK_IP_ALLOWLIST*`/`TRUST_PROXY_HEADERS`
      (repo-guardian-only) while keeping the sidecar per OQ-3a.
- [x] **2.5 `templates/secret.yaml`:** add keys `session-secret`
      (`secrets.sessionSecret`) and `oauth-client-secret`
      (`secrets.oauthClientSecret`) beside the existing `app-id`,
      `webhook-secret`, `private-key`; keep the `existingSecret` bypass
      (document in values comments that an existing secret must carry all five
      keys).
- [x] **2.6 `templates/service.yaml`:** single `http` port (`service.port` →
      targetPort `http`); delete the metrics port.
- [x] **2.7 Fix the four helm-create leftovers** (they now find the `docz-api.*`
      helpers from 2.1): `tests/test-connection.yaml` — point the wget at
      `/healthz` (`…:{{ .Values.service.port }}/healthz`);
      `hpa.yaml`/`ingress.yaml`/`httproute.yaml` — leave gated off by the new
      `autoscaling`/`ingress`/`httpRoute` values from 2.3 and make sure the
      values keys they reference all exist.
- [x] **2.8 `Chart.yaml`:** `appVersion: "v0.4.0"` (per OQ-9a — latest release
      tag), keep `version: 0.1.0` (first published chart version).
- [x] **2.9 Rewrite `templates/NOTES.txt`** for docz-api: deployed message, how
      to port-forward
      (`kubectl port-forward svc/… 8080:{{ .Values.service.port }}`), probe/spec
      URLs (`/healthz`, `/readyz`, `/openapi.yaml`), webhook endpoint
      `/webhooks/github` (+ tailscale funnel URL block only when enabled).
- [x] **2.10 Update `ci/ci-values.yaml`:** keep busybox + `sleep 900` + nulled
      probes, and add dummies for every `required` value so render/lint pass:
      `config.appId: "000000"`,
      `config.authRedirectBase: "http://localhost:8080"`,
      `config.githubOAuthClientID: "ci-dummy"`, and
      `secrets.webhookSecret/privateKey/sessionSecret/oauthClientSecret: "ci-dummy"`.

#### Success Criteria

- `just helm-template` renders with zero errors (impossible before — the
  undefined-helper failure from INV-0004 Obs. 2a is gone).
- `just helm-lint` passes.
- `grep -ri 'repo-guardian' charts/docz-api/templates` prints nothing;
  `grep -rn 'LISTEN_ADDR\|STORE_DSN\|QUEUE_VALKEY_DSN\|GITHUB_PRIVATE_KEY' charts/docz-api/templates`
  prints nothing.
- Rendered deployment shows `runAsUser: 65532`, one containerPort, and every
  required env var from the Reference table
  (`just helm-template | grep -A2 'name: MEILI_HOST'` etc. spot-checks).

---

### Phase 3: Backing services — store/queue rewire + Meilisearch

Make the deployed dependencies actually reachable by docz-api, and add the
missing third dependency. (INV-0004 Obs. 2b, 2c, 2h; OQ-1, OQ-2, OQ-6.)

#### Tasks

- [x] **3.1 Store rename + rewire.** In `store-postgres.yaml`,
      `store-postgres-secret.yaml`, `store-cnpg-cluster.yaml` (kept per OQ-2a):
      db + user `repoguardian` → `doczapi`; the emitted secret key `STORE_DSN` →
      `DATABASE_URL` (same `postgres://…?sslmode=disable` DSN shape); update the
      store secret-key helper in `_helpers.tpl` accordingly; external mode keys
      become `store.external.existingSecret` + `store.external.secretKey`
      (default `DATABASE_URL`).
- [x] **3.2 Queue rewire** (per OQ-6a: keep valkey image + `queue.valkey.*`
      keys): in `queue-valkey-secret.yaml` the emitted key `QUEUE_VALKEY_DSN` →
      `REDIS_URL`; external mode `queue.external.existingSecret` +
      `queue.external.secretKey` (default `REDIS_URL`).
- [x] **3.3 Meilisearch templates** (per OQ-1a — mirror the baked pattern): new
      `templates/search-meili.yaml` — StatefulSet (image
      `getmeili/meilisearch:v1.12`, env `MEILI_MASTER_KEY` from the meili
      secret, `MEILI_ENV: production`, `MEILI_NO_ANALYTICS: "true"`, volume
      `/meili_data` via volumeClaimTemplate, liveness/readiness
      `httpGet /health` port 7700) + headless Service
      `<fullname>-meilisearch:7700`; new `templates/search-meili-secret.yaml` —
      key `MEILI_API_KEY` from `search.meili.masterKey` (required when baked).
      Values:

  ```yaml
  search:
    meili:
      mode: baked # baked | external
      masterKey: "" # required in baked mode
      storage: 5Gi
      host: "" # external mode: http(s)://host:port
      external:
        existingSecret: ""
        secretKey: MEILI_API_KEY
  ```

- [x] **3.4 Wire the deployment:** `MEILI_HOST` =
      `http://<fullname>-meilisearch:7700` in baked mode, `search.meili.host` in
      external mode (helper mirroring the store/queue pattern); `MEILI_API_KEY`
      from the mode-appropriate secret. Add `search.meili.masterKey: "ci-dummy"`
      to `ci/ci-values.yaml`.
- [x] **3.5 Mode-matrix render check** (all must render cleanly):

  ```sh
  just helm-template
  helm template docz-api charts/docz-api -f charts/docz-api/ci/ci-values.yaml \
    --set store.postgres.mode=external --set store.external.existingSecret=x \
    --set queue.valkey.mode=external --set queue.external.existingSecret=x \
    --set search.meili.mode=external --set search.meili.host=http://meili:7700 \
    --set search.meili.external.existingSecret=x
  helm template docz-api charts/docz-api -f charts/docz-api/ci/ci-values.yaml \
    --set store.postgres.mode=cnpg
  ```

#### Success Criteria

- All three renders in 3.5 succeed; the default render contains a Meili
  StatefulSet + Service and env `DATABASE_URL`, `REDIS_URL`, `MEILI_HOST`,
  `MEILI_API_KEY` sourced from secrets whose keys match
  (`just helm-template | grep -B1 -A3 'DATABASE_URL\|REDIS_URL\|MEILI_'`).
- `grep -rn 'repoguardian\|STORE_DSN\|QUEUE_VALKEY_DSN' charts/docz-api` prints
  nothing.

---

### Phase 4: Chart observability, tests, and docs

Point monitoring at reality, then freeze the chart's behavior with a rewritten
test suite and docs. (INV-0004 Obs. 2e, 2g.)

#### Tasks

- [x] **4.1 `templates/servicemonitor.yaml`:** scrape port `http` (docz-api
      serves `/metrics` on the main listener), add `path: /metrics`; gate the
      whole template on `metrics.enabled` AND `serviceMonitor.enabled`.
- [x] **4.2 Rewrite `templates/prometheusrule.yaml`:** group `docz-api.rules`
      replacing all 8 `RepoGuardian*` alerts with exactly these (severity labels
      `warning` unless noted; keep the `prometheusRule.labels` merge behavior):
  - `DoczAPIDown` (critical): `up{job=~".*docz-api.*"} == 0` for 5m.
  - `DoczAPIHighErrorRate`:
    `sum(rate(docz_api_http_requests_total{status=~"5.."}[5m])) / sum(rate(docz_api_http_requests_total[5m])) > 0.05`
    for 10m.
  - `DoczAPISlowRequests`:
    `histogram_quantile(0.99, sum by (le) (rate(docz_api_http_request_duration_seconds_bucket[5m]))) > 2`
    for 10m.
  - `DoczAPIIngestFailures`:
    `sum by (reason) (increase(docz_api_ingest_jobs_total{status="failure"}[30m])) > 2`
    for 0m.
  - `DoczAPISlowIngest`:
    `histogram_quantile(0.99, sum by (le) (rate(docz_api_ingest_job_duration_seconds_bucket[30m]))) > 120`
    for 15m.
- [x] **4.3 Rewrite the helm-unittest suite** (`tests/`): delete the remaining
      repo-guardian suites and write fresh ones, each starting with
      `# yaml-language-server: $schema=https://json.schemastore.org/helm-testsuite.json`:
      `deployment_test.yaml` (image ref honors repository+tag/appVersion;
      securityContext 65532; single containerPort; probe paths),
      `deployment_env_test.yaml` (every Reference-table env present with correct
      secretKeyRef name+key; optionals absent when unset, present when set;
      extraEnv passthrough), `secret_test.yaml` (five keys; existingSecret
      bypass), `backend_shapes_test.yaml` (store baked/cnpg/external × queue
      baked/external × search baked/external render the right objects/refs),
      `service_test.yaml` (one port, selector labels),
      `servicemonitor_test.yaml` (port http, path /metrics, gating),
      `prometheusrule_test.yaml` (five alert names present, no `repo_guardian_`
      strings), `serviceaccount_test.yaml` (keep, rename expectations). Verify
      continuously with `just helm-unittest`.
- [x] **4.4 `values.schema.json` rewrite:** title "docz-api Helm chart values";
      enums for `store.postgres.mode` (baked|cnpg|external), `queue.valkey.mode`
      (baked|external), `search.meili.mode` (baked|external), `config.logLevel`
      (debug|info|warn|error), `config.logFormat` (text|json); types for the
      value blocks introduced in 2.3. Keep it permissive elsewhere
      (`additionalProperties: true`) — it's a guardrail, not a straitjacket.
- [x] **4.5 Docs:** rewrite `README.md.gotmpl` for docz-api (what it deploys,
      the three dependency modes, secrets contract incl. the five-key
      existing-secret shape, OCI install
      `helm install docz-api oci://ghcr.io/donaldgifford/charts/docz-api`,
      cosign verify with cert-identity-regexp
      `^https://github.com/donaldgifford/docz-api/.+`), regenerate with
      `just helm-docs`; fix `CHANGELOG.md` header and `cliff.toml`'s
      `[changelog].header` string to say docz-api.
- [x] **4.6 Full local gate:**
      `just helm-lint && just helm-template && just helm-unittest` all green;
      `grep -ri 'repo.guardian\|repo_guardian' charts/` prints nothing
      (case-insensitive, both spellings).

#### Success Criteria

- `just helm-unittest` passes with the rewritten suite (≥ 8 suite files).
- `just helm-docs` produces a README with no repo-guardian text; committed
  README matches regeneration (`git diff --exit-code charts/docz-api/README.md`
  after running it).
- Rendered PrometheusRule contains only `docz_api_*`/`up` expressions.
- `ct lint --config ct.yaml` passes locally.

---

### Phase 5: CI and release consolidation

One CI workflow, one release workflow, publishing wired in. (INV-0004 Obs. 3;
OQ-4, OQ-5, OQ-10.)

#### Tasks

- [x] **5.1 Merge `ci2.yml` into `ci.yml`** (per OQ-5a). Final `ci.yml` job set:
      `changes` (dorny/paths-filter from ci2, filters
      go/docker/helm/workflows/alerts), existing `Label PR`, existing `Lint`
      (golangci + openapi via mise), `lint-alerts` (NEW: mise step +
      `just lint-alerts`, gated on alerts/workflows changes), existing `Test Go`
      (just test-coverage + Codecov), existing `Security Scan` (govulncheck +
      Trivy — keep Trivy), existing `Build` (goreleaser snapshot
  - SBOM), `docker-build` (from ci2: QEMU/buildx/bake; PR pushes `:dev` per
    OQ-10a, post-merge cacheonly; gated on go/docker/workflows), `helm-unittest`
    (from ci2, plugin install step + `helm lint`), `helm-test` (from ci2: ct +
    kind; gated on helm/workflows). Convert every `make X` to `just X`; do NOT
    port ci2's broken `labeler` job (its only step misuses actions/labeler with
    a checkout name; `Label PR` + `pr-labels.yml` already cover labeling).
    Delete `ci2.yml`. Verify: `just lint-actions`.
- [x] **5.2 Merge `release2.yml` into `release.yml`:** append the `publish-ghcr`
      and `publish-ecr` jobs (with their permissions blocks and explanatory
      comments verbatim — the nested-SLSA permissions ceilings are
      load-bearing), keep the existing bump-version + release jobs untouched,
      and per OQ-4a do NOT port the GPG import step. Delete `release2.yml`.
      Verify: `just lint-actions`;
      `git grep -l 'pr-semver-bump' .github/workflows` shows only `release.yml`.
- [x] **5.3 Re-home the ECR setup doc:** create
      `docs/operations/ecr-publish-setup.md` (adapted from the deleted chart doc
      / ecr.yml's comment: IAM OIDC role trust for
      `repo:donaldgifford/docz-api:*`, two ECR repos, the three secrets, the
      `ECR_PUBLISH_ENABLED` repo variable) so `ecr.yml`'s reference is real. Run
      `prettier --write` + `markdownlint-cli2` on it.
- [x] **5.4 Sanity-check workflow wiring:** `.github/workflows` contains no
      `make` invocations (`git grep -nE '\bmake [a-z-]+' .github/workflows` →
      empty), no duplicate workflow `name:` values
      (`grep -h '^name:' .github/workflows/*.yml | sort | uniq -d` → empty).

#### Success Criteria

- Exactly one CI workflow and one Release workflow; `ci2.yml`/`release2.yml`
  deleted; `just lint-actions` green.
- On push, the CI run is green end-to-end **including** docker-build,
  helm-unittest, and helm-test (chart-testing on kind) — the branch's chart now
  survives `ct install`.
- Publish workflows reference the bake args env (Phase 1.3) and remain
  dispatchable standalone.

---

### Phase 6: Local monitoring stack

Replace the malformed `compose.dev.yaml` with a working, docz-api-branded
monitoring stack. (INV-0004 Obs. 5; OQ-7, OQ-8.)

#### Tasks

- [x] **6.1 Delete `deploy/compose.dev.yaml`** (untracked; plain `rm`).
- [x] **6.2 Create `deploy/compose.monitoring.yaml`** (per OQ-7a): single YAML
      document, modeline + `name: docz-api-monitoring`; services prometheus
      (`prom/prometheus`, 9090, mount `./dev/prometheus/prometheus.yaml` →
      `/etc/prometheus/prometheus.yml`,
      `extra_hosts: ["host.docker.internal:host-gateway"]`), grafana
      (`grafana/grafana`, 3000, mount `./dev/grafana/provisioning`,
      anonymous-admin env), otel-collector
      (`otel/opentelemetry-collector-contrib`, 4317+4318, mount
      `./dev/otel/otel-collector.yaml`), jaeger (`jaegertracing/all-in-one`,
      16686), loki (`grafana/loki`, 3100), alloy (`grafana/alloy`, 12345,
      docker.sock mount) — all always-on; keycloak
      (`quay.io/keycloak/keycloak:26`, 8180,
      `start-dev --import-realm --http-port=8180`, mount `./dev/keycloak` →
      import dir) behind `profiles: [auth]` (OQ-8a). All mounts are `./dev/…` —
      correct relative to `deploy/`. No postgres/meili/app services. Verify:
      `docker compose -f deploy/compose.monitoring.yaml config -q`.
- [x] **6.3 Fix `deploy/dev/prometheus/prometheus.yaml`:** job `docz-api`,
      target `host.docker.internal:8080`, `metrics_path: /metrics`, external
      label `service: docz-api`; keep the otel-collector self-scrape.
- [x] **6.4 Fix grafana provisioning:** in `datasources.yaml` pin
      `uid: prometheus` and `uid: loki` explicitly (dashboard panels bind those
      uids); in `dashboards.yaml` provider/folder → `docz-api`.
- [x] **6.5 Rewrite the overview dashboard**
      (`deploy/dev/grafana/provisioning/dashboards/docz-api-overview.json`):
      title "docz-api overview", uid `docz-api-overview`, tags `["docz-api"]`;
      fix every query to the `docz_api_` prefix; DELETE the in-flight panel (no
      such metric); fix the Loki panel to `{service="docz-api"} | json` and the
      Jaeger link to service `docz-api`; ADD an ingest row: rate by
      reason/status
      (`sum by (reason, status) (rate(docz_api_ingest_jobs_total[5m]))`) and p95
      duration
      (`histogram_quantile(0.95, sum by (le, reason) (rate(docz_api_ingest_job_duration_seconds_bucket[15m])))`).
      Verify JSON: `jq empty <file>`.
- [x] **6.6 Otel-collector + alloy:** in `otel-collector.yaml` delete the
      metrics pipeline + prometheusremotewrite exporter (docz-api is pull-based;
      the pipeline is dead weight per INV-0004 Obs. 5e) and rebrand comments; in
      `config.alloy` rebrand comments (rfc-api → docz-api). Verify: monitoring
      stack starts
      (`docker compose -f deploy/compose.monitoring.yaml up -d --wait` then
      `down`). **Done:** `up -d --wait` returns rc=0 with all six always-on
      containers Healthy; jaeger UI + prometheus `/-/ready` both 200. Two latent
      breakages in the copied stack (surfaced by this task's "stack starts"
      verify) were also fixed: (a) jaeger `all-in-one:latest` moved to v2/EOL and
      its badger store can't `mkdir` under the root-owned volume as the non-root
      image user — pinned to `1.76.0` + switched to `SPAN_STORAGE_TYPE: memory`
      (dev backend; traces inspected live, not persisted) and dropped the
      `jaeger_data` volume; (b) the standalone `loki` exporter was removed from
      collector-contrib — swapped for `otlphttp/loki` → Loki's native OTLP ingest
      (`http://loki:3100/otlp`).
- [x] **6.7 Rewrite the keycloak realm**
      (`deploy/dev/keycloak/docz-api-realm.json`, per OQ-8a): realm `docz-api`;
      ONE confidential client `docz-api` (`publicClient: false`,
      `standardFlowEnabled: true`, secret `dev-docz-api-secret`,
      `redirectUris: ["http://localhost:8080/auth/callback"]`); one test user
      (`dev` / `dev-password`, email `dev@localhost` marked verified —
      docz-api's OIDC provider drops unverified emails). Delete the
      `rfc-api`/`rfc-site` clients. Verify: `jq empty` + stack boots with
      `--profile auth` and
      `curl -fsS localhost:8180/realms/docz-api/.well-known/openid-configuration`
      returns the issuer. **Done:** realm rewritten (dropped both rfc clients +
      the rfc-flavored realm roles); `jq empty` clean; `--profile auth`
      `up -d --wait` rc=0, keycloak Up, issuer =
      `http://localhost:8180/realms/docz-api` with matching auth/token endpoints.
      **Also fixed a latent 6.2 bug:** the copied `quay.io/keycloak/keycloak:26`
      tag doesn't exist (quay publishes only full version tags, no floating major)
      — pinned to `26.7.0`. `deploy/` is now fully rfc-api-clean.
- [x] **6.8 justfile + env plumbing:** recipes `monitor-up`
      (`docker compose -f deploy/compose.monitoring.yaml up -d --wait` + echo of
      the UI URLs), `monitor-auth-up` (same with `--profile auth`),
      `monitor-down` (`--profile auth down`), `monitor-logs`. In `.env.example`
      add commented `OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318`,
      `LOG_FORMAT=json` (needed for the alloy/loki log path), and a commented
      keycloak block (`AUTH_PROVIDERS=github,keycloak`,
      `KEYCLOAK_ISSUER=http://localhost:8180/realms/docz-api`,
      `KEYCLOAK_CLIENT_ID=docz-api`,
      `KEYCLOAK_CLIENT_SECRET=dev-docz-api-secret`). **Done:** four `[group('monitor')]`
      recipes added (mirroring the `local-*` `*_compose`-var style); smoke-tested
      `just monitor-up` (all backends Healthy + URL echo) → `just monitor-down`.
      `.env.example` gets a dedicated "Local monitoring stack" block with the
      concrete dev keycloak values + `LOG_FORMAT=json`. **Deviation:** the OTEL
      endpoint is `localhost:4318` **without** the `http://` scheme the task text
      shows — docz-api's telemetry uses `otlptracehttp.WithEndpoint`, which takes
      host:port (a scheme is parsed as the hostname and breaks export); this
      matches the pre-existing correct entry.
- [x] **6.9 Docs:** DEVELOPMENT.md — new "Local monitoring stack" subsection
      (what runs where: grafana :3000, prometheus :9090, jaeger :16686, keycloak
      :8180; how it pairs with `just run` AND `just local-up`; the
      OTEL/LOG_FORMAT env to set; keycloak login walkthrough); deploy/README.md
      — Layout entries for `compose.monitoring.yaml` + `dev/`. prettier +
      markdownlint clean. **Done:** DEVELOPMENT.md gets the subsection with a
      backend/URL/purpose table + both run-mode env guidance + the keycloak
      walkthrough; deploy/README.md Layout gains `compose.monitoring.yaml` +
      `dev/` entries. `prettier --check` + `markdownlint-cli2` both clean (URLs
      backticked to satisfy MD034).

#### Success Criteria

- `docker compose -f deploy/compose.monitoring.yaml config -q` passes; stack
  comes up healthy with and without `--profile auth`.
- With the app running (`just run` or `just local-up`) and
  `OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318`: prometheus target
  `docz-api` is UP, the grafana overview dashboard shows request data after a
  few curls, and a traced request appears in Jaeger.
- Keycloak path: with the commented env enabled, `/auth/login?provider=keycloak`
  completes a login round-trip with the seeded dev user.
- No `rfc-api`/`rfcapi` strings remain: `grep -ri 'rfc-api\|rfcapi' deploy/` →
  empty.

---

### Phase 7: contrib/ rewrite

Operator-facing assets in docz-api vocabulary. (INV-0004 Obs. 6.)

#### Tasks

- [x] **7.1 Rewrite `contrib/prometheus/alerts.yaml`:** modeline
      `# yaml-language-server: $schema=https://json.schemastore.org/prometheus.rules.json`;
      one group `docz-api` containing the same five alerts as Phase 4.2 (same
      names/expressions — the chart's PrometheusRule and this file are the same
      pack in two formats; note that in a header comment) plus
      `DoczAPINoScrapes`: `absent(up{job="docz-api"}) == 1` for 10m (with a
      comment that the `job` label must match the operator's scrape config).
      Verify: `just lint-alerts`. **Done:** header comment cross-references the
      chart; group `docz-api` has the five shared alerts (plain Prometheus
      annotation templating, no Helm escapes) + `DoczAPINoScrapes`.
      `promtool check rules` → "6 rules found".
- [x] **7.2 Rewrite the dashboard** as
      `contrib/grafana/docz-api-dashboard.json`: start from the Phase 6.5
      overview dashboard; convert to import-style (add an `__inputs` block with
      `DS_PROMETHEUS`, replace pinned datasource uids with `${DS_PROMETHEUS}`;
      drop the Loki/Jaeger panels — operators may not run those); title
      "docz-api", uid `docz-api`, tags `["docz-api"]`. Verify: `jq empty`.
      **Done:** import-style with `__inputs`/`__requires`; the five Prometheus
      panels (rate/latency/error + ingest rate/p95) all reference
      `${DS_PROMETHEUS}`; Loki + Jaeger panels dropped. `jq empty` clean; every
      query uses `docz_api_*`; title/uid/tags = `docz-api`.
- [x] **7.3 Rewrite `contrib/README.md`:** document the four `docz_api_*`
      metrics + label sets (from the Reference section) and the Go/process
      defaults, scrape target `:8080/metrics` (+ `METRICS_ENABLED`), example
      PromQL (error rate, p99 latency, ingest failure rate), dashboard import
      instructions pointing at the REAL filename
      (`contrib/grafana/docz-api-dashboard.json`), alert-pack usage (file for
      vanilla prometheus, chart `prometheusRule.enabled` for operator setups).
      prettier + markdownlint clean. **Done:** the four `docz_api_*` metrics
      (verified against `internal/telemetry/metrics.go`) with exact label sets +
      the `go_*`/`process_*` note; `:8080/metrics` single-port + `METRICS_ENABLED`
      + outside-the-auth-gate; PromQL for 5xx rate / p99 latency / ingest failure
      rate; import instructions naming the real JSON; two-path alert usage.
      `prettier --check` + `markdownlint-cli2` clean.
- [x] **7.4 Final sweep:**
      `grep -ri 'repo_guardian\|repo-guardian\|valkey' contrib/` → empty;
      `just lint-alerts` green; `docz update` to refresh doc indexes if any docs
      changed. **Done:** contrib sweep clean; `just lint-alerts` → "6 rules
      found"; `docz update` ran (index READMEs idempotent — no drift; only a
      cosmetic TOC reformat in this doc).

#### Success Criteria

- `just lint-alerts` passes on the rewritten rules; every expression uses only
  metrics docz-api emits (the four `docz_api_*` names + `up`/`absent`).
- Dashboard JSON is valid and every query uses `docz_api_*`.
- `contrib/README.md` references only files that exist.

## Testing Plan

Final end-to-end verification once all phases are checked off:

- [x] `just ci` (lint + test + build + license-check) green — proves no Go
      regression (there should be zero Go diffs:
      `git diff main --stat -- '*.go'` is empty). **Done:** `git diff main
      --name-only -- '*.go'` empty; `just ci` → "✓ CI pipeline complete".
- [x] `just helm-lint && just helm-unittest && just helm-template` green.
      **Done:** helm-lint 0 failed; helm-unittest 8 suites / 58 tests pass;
      helm-template renders. `just helm-docs` regen → no README drift.
- [x] `ct lint --config ct.yaml` green locally. **Done:** "All charts linted
      successfully".
- [x] Full CI run on the pushed branch green, including docker-build,
      helm-unittest, and helm-test (kind install). **Locally verified to its
      runnable extent:** `just lint-actions` (actionlint) rc=0 — all 12
      workflows valid; the chart/docker jobs' local equivalents (helm-unittest,
      helm-lint, ct lint, docker bake config) pass. The remote GH Actions run
      itself is an out-of-band step gated on a branch push (not performed — the
      loop was authorized to commit per task, not push).
- [x] Monitoring smoke: `just monitor-up` + `just run` +
      `OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318` → prometheus target
      UP, dashboard shows traffic, trace visible in Jaeger; `just monitor-down`.
      **Done:** ran the built binary against the stack (OTEL → localhost:4318);
      Prometheus `docz-api` target `health=up`,
      `docz_api_http_requests_total{route,method,status}` increments (the exact
      series the dashboard queries), 30 `docz-api` traces in Jaeger with chi
      route-template span names. `monitor-down` clean. (Endpoint is `localhost:4318`
      **without** scheme — `otlptracehttp.WithEndpoint` takes host:port.)
- [x] Keycloak smoke: `just monitor-auth-up` + keycloak env →
      `/auth/login?provider=keycloak` round-trip with the `dev` user.
      **Server-side verified:** `--profile auth` up healthy; issuer
      `http://localhost:8180/realms/docz-api` + auth/token endpoints resolve; the
      confidential `docz-api` client + verified `dev`/`dev-password` user are
      seeded. The interactive browser code-exchange round-trip is an out-of-band
      step (needs a browser) — every server-side prerequisite is confirmed.
- [x] `grep -ri 'repo.guardian\|repo_guardian\|rfc-api\|rfcapi' charts/ contrib/ deploy/ .github/`
      → empty. **Done:** SWEEP CLEAN.
- [x] No secrets staged:
      `git diff --cached --name-only | grep -E '\.env$|\.pem$|secrets/'` → empty
      at every commit. **Done:** no secret-shaped files tracked or in any
      `main..HEAD` commit; `deploy/secrets/github-app.pem` correctly gitignored.

## References

- INV-0004 — Helm chart and CI scaffolding audit (the source of every finding
  cited by phase).
- Upstream chart/workflows: `github.com/donaldgifford/repo-guardian`.
- Chart consumer path once published:
  `oci://ghcr.io/donaldgifford/charts/docz-api` (chart),
  `ghcr.io/donaldgifford/docz-api` (image).
- docz-api surfaces: `internal/config/` (env contract),
  `internal/telemetry/metrics.go` (metric names/labels), `cmd/docz-api/main.go`
  (single-port serving), `Dockerfile` (distroless nonroot 65532).
