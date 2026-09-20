# Helm Chart Changelog

Changes to the `docz-api` Helm chart only. For application-level changes,
see the root [CHANGELOG.md](../../CHANGELOG.md).

## 0.8.0

### Added

- **Meilisearch metrics.** With `metrics.enabled` (default) the baked
  Meilisearch now sets `MEILI_EXPERIMENTAL_ENABLE_METRICS=true`, and with
  `serviceMonitor.enabled` the chart renders a second ServiceMonitor,
  `<release>-docz-api-meilisearch`, that scrapes its `/metrics` with the
  master key as a bearer token. Meilisearch gates that route behind the
  key like every route but `/health` (401 without it), which is why it is
  a separate ServiceMonitor and not a second endpoint on the API's. It
  follows the same two switches the CNPG `enablePodMonitor` already does.
  **Upgrading restarts the Meilisearch pod once** for the new env var.
  **Know what the scrape token grants:** it is the master key — full
  admin, not a read-only credential — and Prometheus Operator copies it
  into a generated Secret in the Prometheus namespace. A `metrics.get`
  scoped key would shrink that, but it has to be minted against the
  running instance and cannot share `search.meili.existingSecret` with
  the pod's master key, so a separate scrape-credential value is a
  follow-up. See the README's monitoring notes.

### Fixed

- **The API ServiceMonitor scraped Meilisearch too.** Its selector matched
  on name/instance only, every Service in the release carries those, and
  the Meilisearch Service also has an `http`-named port — so with the
  baked Meilisearch the operator scraped `:7700/metrics`, got 400, and kept
  a permanently-down target. The selector now requires
  `app.kubernetes.io/component: server`, which the API Service now carries
  on its metadata (it was only in its pod selector). Same family as the
  0.2.2 Service fix, on the monitoring side.
- **`DoczAPIDown` fired on healthy installs.** Its `up{job=~".*docz-api.*"}`
  matched the down Meilisearch target above, since Prometheus Operator
  sets `job` to the Service name and every Service starts with the
  fullname. It is now pinned to the API Service exactly:
  `up{job="<release>-docz-api"}`. The plain-Prometheus pack in
  `contrib/prometheus/alerts.yaml` got the same tightening, to
  `job="docz-api"`, matching its own `DoczAPINoScrapes`.

### Not in this release

- Valkey and baked Postgres metrics. Neither image exposes Prometheus
  metrics natively, so each needs an exporter sidecar, a metrics port, and
  its own ServiceMonitor — a matched pair worth its own change. CNPG mode
  already has metrics through the operator's PodMonitor.

## 0.7.1

### Fixed

- **Chart `0.7.0` published before the image its `appVersion` names existed.**
  It declares `appVersion: "0.10.0"`, but the release that carried it was
  tagged `v0.9.1`, so `ghcr.io/donaldgifford/docz-api:0.10.0` was absent and
  a default install would `ImagePullBackOff`. App `v0.10.0` publishes that
  image, which repairs `0.7.0` in place — anyone already pinned to it needs
  no action. This release exists so the fix ships as its own chart version
  rather than rewriting bytes already published under `0.7.0`.

The cause was release-process only; no template, value, or default changed
between `0.7.0` and `0.7.1`.

## 0.7.0

### Changed

- **`appVersion` `0.6.0` → `0.10.0`**, which moves the default image tag.
  It had not been bumped since chart `0.5.1` (PR #20), so a `helm install`
  that did not override `image.tag` deployed an image predating app
  `v0.7.0` — missing the whole `api:`-block pages surface, the
  `config_snapshot` key-spelling fix, the type-dir README and frontmatter
  reporting fixes, and the OIDC scope fix that unblocked Okta and Keycloak
  clients without a `groups` scope. **Anyone on the chart default has been
  four app minors behind; upgrading to `0.7.0` moves them across all of
  it.** Pin `image.tag` if that is not wanted yet.
- App `v0.10.0` itself adds `created` and `updated_at` to search hits plus
  `sort` and `source` query parameters (OpenAPI `1.5.0`). No chart-visible
  config: no new values, no new env, no migration.

### Notes

- `deployment_test.yaml` pins the rendered default tag, so the assertion
  moved with `appVersion`. It also asserts the tag is **bare semver** — a
  `v`-prefixed `appVersion` renders an image ref that 404s, which is what
  PR #20 fixed.
