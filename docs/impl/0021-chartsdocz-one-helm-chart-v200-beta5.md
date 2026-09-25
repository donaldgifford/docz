---
id: IMPL-0021
title: "charts/docz: one Helm chart, v2.0.0-beta.5"
status: Draft
author: Donald Gifford
created: 2026-09-25
---

<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL-0021: charts/docz: one Helm chart, v2.0.0-beta.5

<!--toc:start-->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: Scaffold and tooling](#phase-1-scaffold-and-tooling)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2: The API and its backends](#phase-2-the-api-and-its-backends)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 3: The site, the wiring, and the edge](#phase-3-the-site-the-wiring-and-the-edge)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 4: README, the operator guide, and a real install](#phase-4-readme-the-operator-guide-and-a-real-install)
    - [Tasks](#tasks-3)
    - [Success Criteria](#success-criteria-3)
  - [Phase 5: Publishing, the deprecated finals, and the docs](#phase-5-publishing-the-deprecated-finals-and-the-docs)
    - [Tasks](#tasks-4)
    - [Success Criteria](#success-criteria-4)
  - [Phase 6: v2.0.0-beta.5](#phase-6-v200-beta5)
    - [Tasks](#tasks-5)
    - [Success Criteria](#success-criteria-5)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Dependencies](#dependencies)
- [Open Questions](#open-questions)
  - [1. How many PRs?](#1-how-many-prs)
  - [2. Should a merge to main publish charts?](#2-should-a-merge-to-main-publish-charts)
  - [3. How does the resolve table express a chart-only row?](#3-how-does-the-resolve-table-express-a-chart-only-row)
  - [4. How is the render-parity check run and kept?](#4-how-is-the-render-parity-check-run-and-kept)
  - [5. When is the charts/docz GHCR grant made?](#5-when-is-the-chartsdocz-ghcr-grant-made)
  - [6. Does CI run the helm test hook?](#6-does-ci-run-the-helm-test-hook)
  - [7. Does the root ci gate keep linting the old charts?](#7-does-the-root-ci-gate-keep-linting-the-old-charts)
  - [8. What happens to the sidecar docs in deploy/api/README.md?](#8-what-happens-to-the-sidecar-docs-in-deployapireadmemd)
  - [9. What appVersion do the deprecated finals carry?](#9-what-appversion-do-the-deprecated-finals-carry)
- [References](#references)
<!--toc:end-->

<!--docz:objective:start-->
## Objective

Build `charts/docz`, a single chart that installs docz-api, docz-site, and
their baked Postgres, Valkey, and Meilisearch. Publish it signed from
`v2.0.0-beta.5`, and ship the final deprecated versions of
`charts/docz-api` (0.10.0) and `charts/docz-site` (0.3.0) in the same
release. There are six phases, all in **one PR** from
`feat/helm-chart-migrations`, the branch that already carries INV-0014,
DESIGN-0018, and this plan (Open Question 1). Each numbered task is its own
commit, and the PR's merge commit is the one Phase 6 tags.

**Implements:** DESIGN-0018 (Draft; all seventeen open questions resolved),
which implements INV-0014's resolutions 1b, 2a, and 3c.

Tasks marked **(human)** need a person: a merge, a tag push, a GitHub
settings change, or a cluster. An automated run marks them
`deferred - human required` and continues.

Three numbers carry through every phase:

| | Suites | Tests |
| --- | --- | --- |
| `charts/docz-api` today | 10 | 96 |
| of which `tailscale_test.yaml`, not ported (DESIGN-0018 OQ 9) | 1 | 6 |
| `charts/docz-site` today | 6 | 49 |
| **Ported floor for `charts/docz`** | **15** | **139** |

New `tests/chart/` suites are counted on top of the floor, never toward it.

<!--docz:objective:end-->

<!--docz:scope:start-->
## Scope

<!--docz:in-scope:start-->
### In Scope

- The `charts/docz` chart and its supporting files, all per DESIGN-0018:
  - layout (§1);
  - names, the role label in every selector, and `extraLabels` (§2);
  - values (§3);
  - derived `DOCZ_API_URL`, one `auth.providers`, shared `otel` (§4);
  - one Ingress and one HTTPRoute per workload (§5);
  - helpers (§6);
  - template port (§7);
  - schema, README, CHANGELOG, and `cliff.toml` (§8).
- `deploy/tailscale-operator.md`, the one file that mentions Tailscale:
  the operator in front of docz-api through `api.ingress` (DESIGN-0018 §5,
  OQ 9)
- Porting the 15 suites and adding the `tests/chart/` suites (DESIGN-0018
  Testing Strategy)
- A one-time render-parity check against the old charts, recorded in this
  document (Open Question 4)
- `chart.just` as an optional module, `chart::lint` in the root `ci` gate,
  and the CI unittest loop's glob (DESIGN-0018 §10, OQ 13 and 14)
- A chart-only `component: chart` row in `ghcr.yml` and `ecr.yml`, its
  caller jobs in `prerelease.yml`, and charts publishing only from a tag
  (DESIGN-0018 §9; Open Question 2 amends where the callers go)
- Final deprecated versions of both old charts (DESIGN-0018 §11)
- Repository docs that name the old chart paths or the Tailscale sidecar
- `v2.0.0-beta.5`, and the follow-up issues

<!--docz:in-scope:end-->

<!--docz:out-of-scope:start-->
### Out of Scope

- Any change to either binary, or to any environment variable name
- Migrating an existing docz-api or docz-site release (INV-0014 3c)
- Deleting `charts/docz-api/`, `charts/docz-site/`, their resolve-row
  chart columns, and their `helm-*` recipes. That happens at the v2.0.0 cut
  (DESIGN-0018 OQ 15)
- `charts/docz` 1.0.0, which also waits for v2.0.0 (DESIGN-0018 OQ 2)
- A site alert (DESIGN-0018 OQ 16). Phase 6 files it as an issue
- Enabling ECR publishing
- Making `ct install` run the `helm test` hook in CI (Open Question 6)
- A single-route option (one HTTPRoute to the site, serving both workloads
  through its proxy), and testing the Tailscale operator against the chart.
  Phase 6 files both as one follow-up (DESIGN-0018 §5)
- Any Tailscale template, value, or README section

<!--docz:out-of-scope:end-->
<!--docz:scope:end-->

## Implementation Phases

Each phase builds on the previous one. A phase is complete when all its tasks
are checked off and its success criteria are met. Phases are strictly
sequential commits on one branch, and the one PR carries `dont-release`.
Push after each phase, so CI's helm jobs run against every phase and not
only the last.

The old charts are **not touched** until Phase 5. Until then CI lints,
unit-tests, and `ct install`s all three charts side by side. Nothing
publishes before the tag, because Phase 5 also stops `release.yml`
publishing charts on merge (Open Question 2).

---

<!--docz:phase:start-->
### Phase 1: Scaffold and tooling

Phase 1 builds a chart that renders, lints, and runs an empty test tree,
and wires it into `just` and CI. Later phases only add templates and
suites to it.

<!--docz:tasks:start-->
#### Tasks

- [x] `charts/docz/Chart.yaml`: `name: docz`, `version: 0.1.0`,
  `appVersion: "2.0.0-beta.5"` (bare), `type: application`, a description,
  `home`, `sources`, and a `maintainers` entry copied from
  `charts/docz-api/Chart.yaml`. Add a `kubeVersion` only if docz-api's chart
  has one
  Done; `charts/docz-api` has no `kubeVersion`, so none is set.
- [x] `charts/docz/.helmignore`, copied from `charts/docz-api/`, with
  `tests/` and `ci/` still ignored
- [x] `templates/_helpers.tpl` (DESIGN-0018 §2, §6):
  - `docz.name`, `docz.fullname`, and `docz.chart`;
  - `docz.componentFullname`, `docz.selectorLabels`, `docz.labels`,
    `docz.serviceAccountName`, and `docz.image`, each over
    `dict "ctx" $ "component" "<role>"`;
  - `docz.selectorLabels` `required`s the component;
  - `docz.labels` appends `extraLabels` and `fail`s on a key the chart
    itself sets, naming the key.
  Done, plus `docz.extraLabels` (the collision check, shared) and `docz.podLabels` (selector labels + `extraLabels` for pod templates).
- [ ] `values.yaml` skeleton in DESIGN-0018 §3's order:
  - `nameOverride`, `fullnameOverride`, `imagePullSecrets`;
  - `extraLabels: {}`, `auth`, `otel`, `metrics`, `serviceMonitor`,
    `prometheusRule`;
  - empty `api:` and `site:` blocks;
  - `ingress` and `httpRoute`.

  Each key gets a `# --` helm-docs comment. The `# yaml-language-server`
  modeline points at `values.schema.json`
- [ ] `values.schema.json` skeleton, permissive
  (`additionalProperties: true`), with `extraLabels` as a string map whose
  values match the Kubernetes label-value pattern (≤63 characters) and
  `auth.providers` matching comma-separated `github|okta|keycloak|none`
- [ ] `ci/ci-values.yaml`, starting with the top-level keys only. Phases 2
  and 3 add each workload's busybox override and dummies
- [ ] `cliff.toml`, copied from `charts/docz-site/cliff.toml` with the
  header naming `docz`. Add `CHANGELOG.md` with that header and no entries,
  since `ghcr.yml`'s git-cliff step rewrites it at publish
- [ ] `README.md.gotmpl`: a title, a badges line, and the helm-docs values
  table. Phase 4 writes the prose
- [ ] `tests/chart/helpers_test.yaml`, with no templates yet: a suite over a
  throwaway `templates/tests/_render-helpers.yaml`, or failing that a
  `NOTES.txt` stub, that pins:
  - `docz.fullname` for release `docz` → `docz`;
  - release `prod` → `prod-docz`;
  - `fullnameOverride`;
  - an `extraLabels` collision failing the render.

  If a throwaway template is needed to reach the helpers, Phase 2 deletes
  it once real templates exist
- [ ] `chart.just` (DESIGN-0018 §10, OQ 13) with `[group('helm')]` recipes:
  - `lint`: `helm lint charts/docz -f charts/docz/ci/ci-values.yaml`;
  - `template`;
  - `unittest`: `helm unittest -f 'tests/**/*_test.yaml' charts/docz`;
  - `docs`: `helm-docs --chart-search-root=charts/docz`.

  A header comment explains why the chart is its own module. Add
  `mod? chart 'chart.just'` to the root `justfile`, and list it in the
  header block
- [ ] Root `ci` gate: add `chart::lint` next to `api::helm-lint` (Open
  Question 7)
- [ ] `ci.yml` `helm-unittest`: pass `-f 'tests/**/*_test.yaml'` inside the
  `for chart in charts/*/` loop. Confirm the flat old charts still find
  their suites through the glob: 96 and 49 tests, unchanged
- [ ] `just api lint-actions` clean; `just chart lint`, `just chart unittest`,
  and `just ci` pass

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `just chart lint` and `just chart unittest` pass, and `just --list`
  shows the `chart` module
- The CI `Helm Unit Tests` job reports 96, 49, and the new helper tests
- An `extraLabels` key that collides with a chart label fails the render
  with a message naming it
- `charts/docz-api` and `charts/docz-site` are byte-identical to `main`

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 2: The API and its backends

Port everything docz-api's chart renders except the Tailscale sidecar,
along with its 90 non-Tailscale tests.

<!--docz:tasks:start-->
#### Tasks

- [ ] `templates/_api.tpl`: the 16 helpers from DESIGN-0018 §6, renamed
  `docz-api.X` → `docz.api.X`. Rewrite `.Values.config.authProviders` →
  `.Values.auth.providers` and every fullname call → `docz.fullname`, and
  prefix each backend's name as DESIGN-0018 §2's table says
  (`<fullname>-postgres`, `-valkey`, `-meilisearch`). Add
  `docz.api.internalUrl`
  (`http://<fullname>-api.<ns>.svc.cluster.local:<api.service.port>`).
  `tailscaleStateSecret` is not ported
- [ ] `values.yaml` `api:` block: docz-api's per-workload keys, `config`
  (without `authProviders`), `otel.serviceName`, `secrets`, `autoscaling`,
  and the extras, with their current defaults and comments. `store`, `queue`,
  and `search` are copied to the top level **verbatim** (DESIGN-0018 OQ 3)
- [ ] Workload templates, from `charts/docz-api/templates/`:
  - `api-deployment.yaml`, without the Tailscale container, its volumes,
    and the `TS_*` env;
  - `api-service.yaml`, `api-serviceaccount.yaml`, `api-secret.yaml`,
    `api-hpa.yaml`, `api-servicemonitor.yaml`,
    `api-prometheusrule.yaml` (`up{job="<fullname>-api"}`).

  Every object uses `docz.labels` with component `api`, and every selector
  uses `docz.selectorLabels`, **including `spec.selector.matchLabels`**
  (DESIGN-0018 OQ 11). The undocumented `command` value becomes
  `api.command`, still used only by `ci-values.yaml`
- [ ] Backend templates with unchanged file names:
  - `store-postgres{,-secret}`;
  - `store-cnpg-{cluster,pooler,pooler-service}`;
  - `queue-valkey{,-secret}`;
  - `search-meili{,-secret,-servicemonitor}`.

  Each pod template gets `extraLabels`. `volumeClaimTemplates` and CNPG
  `inheritedMetadata` must **not** get it (DESIGN-0018 §2)
- [ ] Share the HPA: extract `docz.hpa` (`dict ctx component`) now, so the
  site's HPA in Phase 3 is a one-line include
- [ ] `ci/ci-values.yaml` `api:` block: busybox with `command: [sleep, "900"]`,
  `config.appId`, `authRedirectBase`, `githubOAuthClientID`, dummy secrets
  with `privateKeyAsFile: false`, nulled probes, empty resources. Keep the
  ≥16-byte `search.meili.masterKey`
- [ ] Port the nine suites to `tests/api/`:
  - `backend_shapes`, `deployment_env`, `deployment`, `prometheusrule`,
    `search-meili-servicemonitor`, `secret`, `service`, `serviceaccount`,
    `servicemonitor`;
  - rewrite the `set:` paths, `templates:` paths, and expected names;
  - keep every assertion;
  - where a test cannot survive, delete it and name it with its reason in
    the commit body (DESIGN-0018 Testing Strategy).
- [ ] Delete Phase 1's throwaway helper template, if there was one, and
  repoint `tests/chart/helpers_test.yaml` at `api-deployment.yaml`
- [ ] Render parity, API half (Open Question 4): render `charts/docz-api` and
  `charts/docz` in four cases:
  - each chart's `ci-values`;
  - a CNPG case;
  - an all-external case;
  - `auth.providers: none`.

  Normalise names to §2 and diff container specs, env, volumes, and Secret
  keys with `dyff between` or `diff <(yq -P … | sort)`. Record in this task
  every difference other than names, labels, and the Tailscale removal,
  with its cause. There should be none
- [ ] `just chart lint`, `just chart unittest`, `just ci`

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `tests/api/` holds 9 suites and at least 90 tests, all passing, and any
  deletion is named in a commit body
- The API half of the parity check shows no unexplained difference in any of
  the four cases
- `helm template` in baked, CNPG, and all-external modes renders no
  `tailscale` string (case-insensitive)
- Every Deployment, StatefulSet, Service, and ServiceMonitor selector
  carries `app.kubernetes.io/component`

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 3: The site, the wiring, and the edge

Phase 3 adds the second workload and the three things that make the chart
more than two charts in one directory: the derived API URL, one provider
list, and one front door.

<!--docz:tasks:start-->
#### Tasks

- [ ] `values.yaml` `site:` block: docz-site's per-workload keys, `config`
  (`port`, `doczApiUrl: ""`, `mermaidLayout`, `navLinks`, `logLevel`,
  `logFormat`), and `otel.serviceName: docz-site`. There is no `enabled`
  key (DESIGN-0018 OQ 5)
- [ ] `site-deployment.yaml`, `site-service.yaml`, `site-serviceaccount.yaml`,
  `site-hpa.yaml` (the `docz.hpa` include), and `site-servicemonitor.yaml`,
  from `charts/docz-site/templates/`, with component `site` throughout
- [ ] Wiring (DESIGN-0018 §4):
  - `DOCZ_API_URL` = `site.config.doczApiUrl`, or else
    `docz.api.internalUrl`, with no `required`;
  - `AUTH_PROVIDERS` and `DOCZ_AUTH_PROVIDERS` both come from
    `auth.providers`;
  - the `OTEL_*` env in both Deployments is gated on `otel.endpoint`, with
    each workload's own `OTEL_SERVICE_NAME`.
- [ ] `api.config.authRedirectBase`: keep the `required` unless
  `docz.api.authDisabled`, and reword its values comment to say it is the
  **site's** public URL (DESIGN-0018 OQ 17)
- [ ] Edges, one pair per workload (DESIGN-0018 §5, OQ 8 revised):
  - `docz.ingress` and `docz.httpRoute` helpers (`dict ctx component`),
    ported from the two charts' identical templates;
  - `api-ingress.yaml`, `api-httproute.yaml`, `site-ingress.yaml`, and
    `site-httproute.yaml` as one-line includes, each named
    `<fullname>-<component>` and routed to its own Service and port;
  - `ingress` and `httpRoute` blocks under both `api:` and `site:` in
    `values.yaml`, with the old charts' shapes and defaults (disabled);
  - an empty `rules` renders the default rule for both. docz-site did this
    already; docz-api's route had no rules at all.
- [ ] `templates/tests/test-connection.yaml`: one hook pod with two
  containers, each `wget`ting one Service's `/healthz`, and the `docz.labels`
  of component `test`
- [ ] `NOTES.txt`:
  - each workload's URL (its Ingress host or HTTPRoute hostname, or else a
    port-forward to its Service);
  - the derived or overridden API URL;
  - a warning when `auth.providers` is `none`, carried over from docz-api's
    NOTES;
  - a reminder that `authRedirectBase` must be the site's URL.
- [ ] `ci/ci-values.yaml` `site:` block: busybox with `sleep`, and no
  `doczApiUrl`, so the derived path is what CI installs
- [ ] Port the six docz-site suites to `tests/site/`:
  - `deployment`, `httproute`, `ingress`, `service`, `serviceaccount`,
    `servicemonitor`;
  - the `doczApiUrl`-required assertion becomes an "unset derives" and a
    "set overrides" pair, which moves to `tests/chart/wiring_test.yaml`;
  - httproute and ingress assertions render `site-*.yaml` and expect
    `<fullname>-site`.
- [ ] New suites under `tests/chart/`, from DESIGN-0018's Testing Strategy:
  - `wiring_test.yaml`:
    - the URL is derived, or overridden when set;
    - one `auth.providers` value reaches both env vars;
    - with `otel.endpoint` empty, `OTEL_*` is absent from both workloads.
  - `selectors_test.yaml`:
    - every selector carries a role;
    - no two workloads share one;
    - no selector carries an `extraLabels` key.
  - `edge_test.yaml`:
    - each workload's Ingress and HTTPRoute send traffic to that workload's
      Service and port, and to no other;
    - enabling one workload's edge renders nothing for the other;
    - an empty `rules` renders the default rule for both;
    - `api.ingress.className: tailscale` renders as given, which is the
      only edge setting the operator guide needs.
  - `extralabels_test.yaml`, with every optional template enabled:
    - `extraLabels` is on every object and pod template;
    - it is absent from `volumeClaimTemplates` and CNPG
      `inheritedMetadata`;
    - a collision fails the render;
    - the empty default adds nothing.
  - `notailscale_test.yaml`: with default values, across baked, CNPG, and
    external, no document `matchRegex`es `(?i)tailscale`.
  - `version_test.yaml`: both images default to `.Chart.AppVersion`, which
    is bare semver.
- [ ] Render parity, site half and both edges: diff `charts/docz-site`
  against `charts/docz` on the same basis as Phase 2, with each chart's
  Ingress and HTTPRoute enabled. Diff docz-api's edges the same way. The
  only differences allowed are names, labels, `DOCZ_API_URL`, and the API
  HTTPRoute's new default rule. Record the result here
- [ ] `just chart lint`, `just chart unittest`, `just ci`, and CI's
  `Helm Chart Test` (`ct install` on kind) green on the PR

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `tests/api/` and `tests/site/` together hold at least 139 passing tests,
  and `tests/chart/` adds the six suites above
- `ct install` of `charts/docz` on kind succeeds with no `doczApiUrl` set
- Both parity halves are recorded with no unexplained difference
- `helm template` with default values sets `DOCZ_API_URL` to
  `http://docz-api.default.svc.cluster.local:80` for release `docz`

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 4: README, the operator guide, and a real install

Phase 4 covers what a person installing the chart reads, and the one time
the chart runs the real images.

<!--docz:tasks:start-->
#### Tasks

- [ ] `README.md.gotmpl` prose (DESIGN-0018 §8), with these sections:
  - Install: `helm install docz oci://ghcr.io/donaldgifford/charts/docz`
    and the values every install needs;
  - The two edges: `api.*` and `site.*` each take an Ingress and an
    HTTPRoute, and `authRedirectBase` is the site's URL;
  - Backend modes (baked, CNPG, external), adapted from docz-api's
    README;
  - Login providers, and `none` with its exposure warning;
  - `extraLabels`;
  - Observability;
  - "Coming from docz-api or docz-site": the DESIGN-0018 Data Model tables,
    "no in-place upgrade", and "the Tailscale sidecar is gone".

  The README does not mention Tailscale beyond that one line. Regenerate
  with `just chart docs`
- [ ] `deploy/tailscale-operator.md` (DESIGN-0018 §5, OQ 9), the one file
  that describes the operator. It says that the Tailscale operator works in
  front of docz-api, and that how the operator itself behaves is the
  operator's own documentation. It covers:
  - the `api.ingress` values: `className: tailscale`, the operator's Funnel
    annotation, and a tailnet host;
  - the resulting GitHub App webhook URL,
    `https://<host>.<tailnet>.ts.net/webhooks/github`;
  - the two tailnet-policy settings carried over from
    `deploy/api/README.md` whose absence shows as a TLS EOF on every
    delivery: Funnel granted to the proxy's tag in `nodeAttrs`, and HTTPS
    certificates enabled.

  Link it from the chart README's edges section and from
  `deploy/api/README.md`
- [ ] `values.schema.json` completed under the new paths:
  - the three backend `mode` enums;
  - both workloads' `logLevel`/`logFormat`;
  - `site.config.mermaidLayout`.

  Prove it rejects `store.postgres.mode=memory` and
  `api.config.logLevel=trace` with `helm template`
- [ ] **(human)** Real-image install on a kind (or homelab) cluster with
  `auth.providers: none` and `api.config.githubApiBase: http://127.0.0.1:1`.
  The unreachable API base makes the boot self-check warn and continue
  rather than fail on a dummy App key, because only a 401 is fatal (see
  IMPL-0006). Then check:
  - `kubectl port-forward svc/docz-site 8080:80`, and
    `curl -fsS localhost:8080/api/v1/repos` returns `{"repos":[]}` through
    the site's proxy, which closes IMPL-0020's open round trip;
  - `helm test docz` passes both `/healthz` checks.

  Record the output here
- [ ] `just chart docs` leaves no diff; `just validate`; `just ci`

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- The generated `README.md` is current (`git diff --exit-code` after
  `just chart docs`) and covers every section above
- A real install answers `/api/v1/repos` through the site with no API URL
  configured, and `helm test` passes
- The schema rejects the two bad values

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 5: Publishing, the deprecated finals, and the docs

Phase 5 is everything that has to be in place before the tag. It is the
first phase that touches the old charts and the workflows.

<!--docz:tasks:start-->
#### Tasks

- [ ] `ghcr.yml` and `ecr.yml` `resolve`: add
  `chart) row="- - docz charts/docz"`, where `-` fills the image and bake
  fields `read -r` needs (Open Question 3). Add `chart` to the
  `workflow_dispatch` choice list and to both `component` descriptions.
  The `image` job already skips when `tag` is empty, so a chart-only call
  passes no tag and needs no new gate
- [ ] Publish on merge (Open Question 2). Add a `publish_chart` boolean input
  to `ghcr.yml` and `ecr.yml`, default `true`, gating the `chart` job, and
  pass `false` from `release.yml`'s four existing calls. Charts then publish
  only from a `v*-beta.*` tag, which is what `ui/CLAUDE.md` already claims
  and #132 showed was untrue
- [ ] `prerelease.yml`: add `publish-chart` (`ghcr.yml`, `component: chart`,
  no `tag`) and `publish-ecr-chart` (gated on `vars.ECR_PUBLISH_ENABLED`),
  with the same permissions ceiling as the existing four jobs. `release.yml`
  gets no chart job
- [ ] Deprecated finals (DESIGN-0018 §11):
  - `charts/docz-api/Chart.yaml`: `version: 0.10.0`,
    `appVersion: "2.0.0-beta.5"`, `deprecated: true`;
  - `charts/docz-site/Chart.yaml`: `version: 0.3.0`, the same
    `appVersion`, `deprecated: true`;
  - each chart's `NOTES.txt` opens with a deprecation block naming
    `oci://ghcr.io/donaldgifford/charts/docz`, "no in-place upgrade", and
    (docz-api's only) "the Tailscale sidecar does not carry over";
  - each `README.md.gotmpl` opens with the same notice;
  - regenerate with `just api helm-docs` and `just ui helm-docs`;
  - each chart's bare-semver unit test still passes.
- [ ] Repository docs name `charts/docz`:
  - `CLAUDE.md`:
    - the opening paragraph, the Build & Test block (`just chart …`), and
      the justfile paragraph (a third module);
    - the "Two components publish from one tag" bullet, which becomes
      three with the chart row;
    - a pointer from the docz-api "Helm chart + publish pipeline" section
      to `charts/docz`, marking `charts/docz-api` deprecated.
  - `README.md`, `DEVELOPMENT.md`, `CONTRIBUTING.md` (the chart-bump rule
    names `charts/docz`), and `ui/CLAUDE.md` (its chart-bump rule, now
    true about tags);
  - the comments in `cliff.toml`, `mise.toml`, and
    `contrib/prometheus/alerts.yaml`, which point at
    `charts/docz/templates/api-prometheusrule.yaml`;
  - `deploy/api/README.md`'s Kubernetes webhook section: remove the
    Tailscale sidecar text and its three failure modes entirely. Leave a
    short paragraph saying that GitHub needs a public path to the API's
    `/webhooks/github` through `api.ingress` or `api.httpRoute`, with a
    link to `deploy/tailscale-operator.md` (Open Question 8).
- [ ] `just api lint-actions`, `just validate`, `just ci`, and CI green,
  including `ct lint` on all three charts

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- actionlint is clean, and `release.yml` passes `publish_chart: false` on
  every call, so merging the PR publishes no chart. After the merge, check
  its `release.yml` run: every `chart` job is skipped
- Both old charts carry `deprecated: true`, a bumped version, and
  `appVersion: 2.0.0-beta.5`, and still pass their unit tests
- No tracked file outside `docs/`, `CHANGELOG.md`, and the old charts
  themselves presents `charts/docz-api` or `charts/docz-site` as current

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 6: v2.0.0-beta.5

<!--docz:tasks:start-->
#### Tasks

- [ ] **(human)** Review and merge the PR **with a merge commit**. After the
  merge, confirm that its `release.yml` run skipped every `chart` job
  (Phase 5's criterion)
- [ ] `just release-check` and `just api release-check` pass on `main`
- [ ] **(human)** `just release v2.0.0-beta.5` from **the PR's merge
  commit**, the last commit that changes a workflow
- [ ] **(human)** The GHCR grant (Open Question 5). The tag's
  `publish-chart` run creates `charts/docz` in GHCR and fails
  `403 write_package`. Then go to GHCR → Packages → `charts/docz` → Manage
  Actions access, add `donaldgifford/docz` with **Write**, and re-run the
  failed job. The chart job is idempotent, and nothing else in the run
  depends on it
- [ ] Check the `prerelease.yml` run:
  - the release is a pre-release with the `docz_*` and `docz-api_*`
    archives;
  - both images are pushed as `2.0.0-beta.5`, with no `latest`;
  - `publish-chart` pushed `docz` 0.1.0;
  - the api and ui jobs pushed `docz-api` 0.10.0 and `docz-site` 0.3.0;
  - both ECR jobs are skipped.
- [ ] Check the charts. Each of `docz` 0.1.0, `docz-api` 0.10.0, and
  `docz-site` 0.3.0 must show a signature and SLSA provenance, and report
  the bare `appVersion`, with `deprecated: true` on the last two:

  ```sh
  helm show chart oci://ghcr.io/donaldgifford/charts/<name> --version <v>
  cosign tree ghcr.io/donaldgifford/charts/<name>:<v>
  ```
- [ ] **(human)** `helm install docz oci://ghcr.io/donaldgifford/charts/docz
  --version 0.1.0` with Phase 4's values. `/api/v1/repos` answers through
  the site, and `helm test docz` passes
- [ ] File the follow-ups as issues:
  - a single-route option: one HTTPRoute (or Ingress) to the site that
    serves both workloads through the site's proxy, as an alternative to
    the two per-workload edges. Include testing the Tailscale operator
    against the chart, both through `api.ingress` as
    `deploy/tailscale-operator.md` describes and through the single route
    (DESIGN-0018 §5, rollout step 7);
  - a `DoczSiteDown` / proxy-error alert (DESIGN-0018 OQ 16);
  - at v2.0.0: delete `charts/docz-api/` and `charts/docz-site/`, their
    resolve-row chart columns, `api.just`'s and `ui.just`'s `helm-*`
    recipes, and `api::helm-lint` from the `ci` gate, and bump
    `charts/docz` to 1.0.0 (DESIGN-0018 §11). Add both items to #135's
    v2.0.0 list, or file one issue for them
- [ ] `docz status set impl IMPL-0021 Completed` and
  `docz status set design DESIGN-0018 Implemented`, then `docz update`

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `oci://ghcr.io/donaldgifford/charts/docz` 0.1.0 is published, signed,
  attested, and installs the real product
- `docz-api` 0.10.0 and `docz-site` 0.3.0 are published as deprecated
- IMPL-0021 is Completed and DESIGN-0018 Implemented, with the follow-ups
  filed

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:file-changes:start-->
## File Changes

| File | Action | Phase | Description |
| ---- | ------ | ----- | ----------- |
| `charts/docz/Chart.yaml`, `.helmignore`, `cliff.toml`, `CHANGELOG.md` | Create | 1 | Chart metadata |
| `charts/docz/values.yaml`, `values.schema.json` | Create | 1–4 | Grow per phase |
| `charts/docz/ci/ci-values.yaml` | Create | 1–3 | busybox for both workloads |
| `charts/docz/templates/_helpers.tpl` | Create | 1 | Names, labels, selectors, `extraLabels` |
| `charts/docz/templates/_api.tpl` | Create | 2 | 16 ported helpers, plus `internalUrl` |
| `charts/docz/templates/api-*.yaml` (7) | Create | 2 | API workload |
| `charts/docz/templates/{store,queue,search}-*.yaml` (10) | Create | 2 | Backends |
| `charts/docz/templates/site-*.yaml` (5) | Create | 3 | Site workload |
| `charts/docz/templates/{api,site}-{ingress,httproute}.yaml` | Create | 3 | One edge pair per workload, over `docz.ingress`/`docz.httpRoute` |
| `charts/docz/templates/NOTES.txt`, `tests/test-connection.yaml` | Create | 3 | |
| `charts/docz/tests/{api,site,chart}/*_test.yaml` | Create | 1–3 | 15 ported, 7 new suites |
| `charts/docz/README.md.gotmpl`, `README.md` | Create | 1, 4 | No Tailscale section |
| `deploy/tailscale-operator.md` | Create | 4 | The operator in front of docz-api |
| `chart.just` | Create | 1 | `just chart lint/template/unittest/docs` |
| `justfile` | Modify | 1 | `mod? chart`, `chart::lint` in `ci` |
| `.github/workflows/ci.yml` | Modify | 1 | Unittest glob |
| `.github/workflows/{ghcr,ecr}.yml` | Modify | 5 | `chart` row, `publish_chart` input |
| `.github/workflows/prerelease.yml` | Modify | 5 | `publish-chart`, `publish-ecr-chart` |
| `.github/workflows/release.yml` | Modify | 5 | `publish_chart: false` on four calls |
| `charts/docz-api/{Chart.yaml,templates/NOTES.txt,README.md.gotmpl,README.md}` | Modify | 5 | 0.10.0, deprecated |
| `charts/docz-site/{Chart.yaml,templates/NOTES.txt,README.md.gotmpl,README.md}` | Modify | 5 | 0.3.0, deprecated |
| `CLAUDE.md`, `README.md`, `DEVELOPMENT.md`, `CONTRIBUTING.md`, `ui/CLAUDE.md` | Modify | 5 | Name `charts/docz` |
| `deploy/api/README.md` | Modify | 5 | Sidecar section removed; link to the operator guide |
| `cliff.toml`, `mise.toml`, `contrib/prometheus/alerts.yaml` | Modify | 5 | Comments |

<!--docz:file-changes:end-->

<!--docz:testing:start-->
## Testing Plan

- [ ] helm-unittest: at least 139 ported tests across `tests/api/` (9 suites)
  and `tests/site/` (6), plus the seven `tests/chart/` suites, all run by
  `just chart unittest` and CI's loop
- [ ] The old charts' suites keep passing unchanged through Phase 4, and with
  only the deprecation edits in Phase 5
- [ ] Render parity against both old charts in four value cases, recorded in
  Phases 2 and 3 (Open Question 4)
- [ ] `ct lint` and `ct install` on kind for `charts/docz`, with the derived
  API URL
- [ ] A real-image install with `helm test` and the proxy round trip
  (Phase 4), repeated from the published chart (Phase 6)
- [ ] Schema rejection of an unknown backend mode and log level
- [ ] Publishing: no chart publishes on merge (Phase 5), and all three publish,
  signed and attested, from the tag (Phase 6)

<!--docz:testing:end-->

<!--docz:dependencies:start-->
## Dependencies

- DESIGN-0018's resolved open questions. This plan follows them and
  re-opens none
- The `helm-unittest` plugin locally, with `-f` glob support (current
  releases have it)
- A kind or homelab cluster for Phase 4's real-image install and Phase 6's
  install from GHCR
- GHCR package settings access for the `charts/docz` grant (Phase 6)
- Nothing else pending on `main`. #112, the fix slate, and GO-2026-4970 are
  independent and may land before or after

<!--docz:dependencies:end-->

<!--docz:open-questions:start-->
## Open Questions

> Each question is numbered. Option `a` is my recommendation, the later
> letters are alternatives, and the last is "other" for your own answer.
> All nine were resolved on 2026-09-25.

### 1. How many PRs?

- a. **One per phase, six in all**, as IMPL-0019 and IMPL-0020 did. Each is
  reviewable on its own, and Phases 1–4 are invisible to users, since
  nothing publishes and the old charts are untouched.
- b. Two: Phases 1–4 (the chart) and Phase 5 (publishing, deprecation,
  docs). There are fewer merges, but the first PR is a whole new chart.
- c. One PR for Phases 1–5.
- d. Other.

> **Resolved 2026-09-25: (d).** One PR for all six phases, from
> `feat/helm-chart-migrations`, which already carries INV-0014, DESIGN-0018,
> and this plan. Each task is a commit, and the PR's merge commit is the one
> Phase 6 tags.

### 2. Should a merge to `main` publish charts?

`release.yml` runs on every merge. With `dont-release` its tag is empty, so
images skip, but the chart jobs publish any chart version not yet in GHCR.
That is how `docz-site` 0.2.0 went out at #132's merge, before its
`2.0.0-beta.4` image existed, and it contradicts `ui/CLAUDE.md`'s "charts
publish only from a tag".

- a. **No.** Add a `publish_chart` input to `ghcr.yml`/`ecr.yml`, default
  `true`, and pass `false` from `release.yml`. Charts then publish only from
  a tag, together with the images their `appVersion` names. It is small,
  and it lives in files Phase 5 edits anyway.
- b. Leave it. `charts/docz` 0.1.0 and both deprecated finals would publish
  at Phase 5's merge, pointing at `2.0.0-beta.5` images that do not exist
  until the tag. The GHCR grant then has to precede the merge.
- c. Remove the chart calls from `release.yml` entirely. This has the same
  effect as (a), but loses the ability to publish a chart-only fix at
  v2.0.0 proper, when `release.yml` becomes the real release path again.
- d. Other.

> **Resolved 2026-09-25: (a).** Charts publish only from a tag.
> DESIGN-0018 §9 is amended to match.

### 3. How does the resolve table express a chart-only row?

- a. **`-` placeholders in the image and bake fields, with no new gate.**
  The `image` job already runs only when `tag != ''`, and a chart call
  passes no tag.
- b. Add an explicit `has_image` output and gate the image job on it as
  well. This is belt and braces, but it is a second condition to keep in
  step.
- c. Other.

> **Resolved 2026-09-25: (a).** `-` placeholders, no new gate.

### 4. How is the render-parity check run and kept?

- a. **A throwaway script** (`helm template` + `yq` name normalisation +
  `dyff`) run in Phases 2 and 3, with its result recorded in those tasks
  and not committed. DESIGN-0018 calls it one-time, and the old charts are
  frozen from Phase 5.
- b. Commit it as `test/chart-parity/` and run it in CI until v2.0.0
  deletes the old charts.
- c. Skip it, and rely on the ported unit tests.
- d. Other.

> **Resolved 2026-09-25: (a).** A throwaway script, with the result
> recorded in Phases 2 and 3.

### 5. When is the `charts/docz` GHCR grant made?

A package's Actions access can only be granted once the package exists.

- a. **After the tag's first push creates it.** The `publish-chart` job
  fails `403 write_package` once; then grant **Write** and re-run the
  failed job. The chart job is idempotent, and nothing else in the run
  depends on it.
- b. Create it ahead of the tag with a one-off `workflow_dispatch` of
  `ghcr.yml` (`component: chart`) after Phase 5 merges. This publishes
  0.1.0 before its images, which is what OQ 2 (a) exists to avoid.
- c. Push a placeholder by hand (`helm push` of a `0.0.0` chart from a
  workstation), grant, then tag.
- d. Other.

> **Resolved 2026-09-25: (a).** After the tag's first push creates the
> package; then re-run the failed job (Phase 6).

### 6. Does CI run the `helm test` hook?

CI's `ct install` logs show `TEST SUITE: None` for the existing charts, and
with busybox `sleep` containers a `/healthz` check could not pass anyway.

- a. **No, not in this plan.** The hook is proven by hand against real
  images in Phases 4 and 6, and making `ct install` run it with real images
  and credentials is a separate change.
- b. Investigate why the hook does not run, and make `ct install` run it
  with a lightweight HTTP stub in place of busybox.
- c. Other.

> **Resolved 2026-09-25: (a).** Proven by hand in Phases 4 and 6.

### 7. Does the root `ci` gate keep linting the old charts?

- a. **Yes, until v2.0.0: `chart::lint` joins `api::helm-lint`.** The old
  charts still change once in Phase 5, and local `just ci` should catch a
  broken deprecated final before CI does. Both go at the v2.0.0 cut.
- b. Replace `api::helm-lint` with `chart::lint` now, as DESIGN-0018 §10
  words it, and rely on CI's `ct lint` for the old charts.
- c. Other.

> **Resolved 2026-09-25: (a).** `chart::lint` joins `api::helm-lint`
> until v2.0.0. DESIGN-0018 §10 is amended to match.

### 8. What happens to the sidecar docs in `deploy/api/README.md`?

- a. **Replace them with the operator approach and a link to the chart
  README**, keeping one line saying the sidecar applies only to the
  deprecated `docz-api` chart. The three failure modes that still apply
  (the Funnel `nodeAttrs` and HTTPS certificates) move to the chart
  README.
- b. Keep the sidecar section intact until v2.0.0, beside a new operator
  section, since `docz-api` 0.10.0 still ships the sidecar.
- c. Other.

> **Resolved 2026-09-25: (c).** Remove all Tailscale from
> `deploy/api/README.md`, sidecar and failure modes alike. How the operator
> works is the operator's business. `deploy/tailscale-operator.md` is the one
> file that says it works in front of docz-api (Phase 4). Testing it, and
> a single-route option, is a follow-up (Phase 6).

### 9. What `appVersion` do the deprecated finals carry?

- a. **`2.0.0-beta.5`**, the release they publish in. This ends INV-0014's
  skew, and a user who installs a deprecated chart gets current images.
- b. Leave each at its current `appVersion` (`2.0.0-beta.3` /
  `2.0.0-beta.4`) and change only `deprecated`. That is the smallest diff,
  but it ships a known-stale default.
- c. Other.

> **Resolved 2026-09-25: (a).** `2.0.0-beta.5`.

<!--docz:open-questions:end-->

<!--docz:references:start-->
## References

- [DESIGN-0018](../design/0018-one-helm-chart-for-docz-chartsdocz-replaces-docz-api-and-docz.md):
  the chart this plan builds
- [INV-0014](../investigation/0014-consolidate-the-docz-api-and-docz-site-helm-charts-into-one.md):
  why one chart, and the three resolutions
- [IMPL-0020](0020-docz-site-move-in-v200-beta4.md): the per-component
  publish path, and the proxy round trip left open in its Phase 5
- [Tailscale Kubernetes operator: cluster ingress](https://tailscale.com/kb/1439/kubernetes-operator-cluster-ingress)
- [helm-unittest](https://github.com/helm-unittest/helm-unittest)
- [chart-testing](https://github.com/helm/chart-testing)

<!--docz:references:end-->
