---
id: INV-0014
title: "Consolidate the docz-api and docz-site Helm charts into one chart"
status: Concluded
author: Donald Gifford
created: 2026-09-25
---

<!-- markdownlint-disable-file MD025 MD041 -->

# INV-0014: Consolidate the docz-api and docz-site Helm charts into one chart

<!--toc:start-->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
  - [Observation 1: two charts, one release train](#observation-1-two-charts-one-release-train)
  - [Observation 2: the site cannot run without the API](#observation-2-the-site-cannot-run-without-the-api)
  - [Observation 3: the charts are the same shape](#observation-3-the-charts-are-the-same-shape)
  - [Observation 4: the API chart carries stateful backends](#observation-4-the-api-chart-carries-stateful-backends)
  - [Observation 5: the pipeline is built for two](#observation-5-the-pipeline-is-built-for-two)
  - [Pros and cons](#pros-and-cons)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
  - [1. What shape does one chart take?](#1-what-shape-does-one-chart-take)
  - [2. What happens to the two published charts?](#2-what-happens-to-the-two-published-charts)
  - [3. How does an existing release move over?](#3-how-does-an-existing-release-move-over)
- [References](#references)
<!--toc:end-->

<!--docz:question:start-->
## Question

Should `charts/docz-api` and `charts/docz-site` become one chart that
installs the whole docz product, and if so, in what shape: a single merged
chart, or an umbrella over the two?

<!--docz:question:end-->

<!--docz:hypothesis:start-->
## Hypothesis

Yes. Since IMPL-0020 the two services ship from one repository, one tag, and
one version. docz-site does nothing without a docz-api behind it. So two
charts duplicate a version, a set of shared settings, and a publish path,
and none of that buys anything. The cost of consolidating is not in the
templates. It is in moving existing releases without losing the API chart's
stateful backends.

<!--docz:hypothesis:end-->

<!--docz:context:start-->
## Context

**Triggered by:** the docz-site move-in (IMPL-0020, DESIGN-0017), which
brought `charts/docz-site` next to `charts/docz-api` and made
`v2.0.0-beta.4` the first tag to publish both.

ADR-0004 moved docz-api and docz-site into this repository as one Go module
with one release. The charts came across as they were: two independent
charts with two `Chart.yaml` versions, two `appVersion`s, and two OCI
packages (`charts/docz-api`, `charts/docz-site`). DESIGN-0017 kept them
separate on purpose, to keep the move mechanical. It did not decide whether
separate is right for the product afterwards. This investigation takes that
question up.

<!--docz:context:end-->

<!--docz:approach:start-->
## Approach

1. Compare the two charts file by file: `Chart.yaml`, `values.yaml` keys,
   templates, helpers, and the unit suites.
2. List every setting one chart has to agree with in the other, and every
   one that is duplicated.
3. Trace what `v2.0.0-beta.4` actually published for each chart.
4. Identify the stateful resources a release migration would have to keep.
5. Read how `ghcr.yml`, `ci.yml`, and `ct.yaml` assume two charts.

<!--docz:approach:end-->

<!--docz:environment:start-->
## Environment

| Component         | Version / Value                                      |
| ----------------- | ---------------------------------------------------- |
| `charts/docz-api` | `0.9.0`, `appVersion: 2.0.0-beta.3`, 96 unit tests   |
| `charts/docz-site`| `0.2.0`, `appVersion: 2.0.0-beta.4`, 49 unit tests   |
| Release           | `v2.0.0-beta.4` (run 36017974151)                    |
| Helm              | 4.2.2, helm-unittest, chart-testing via `ct.yaml`    |

<!--docz:environment:end-->

<!--docz:findings:start-->
## Findings

### Observation 1: two charts, one release train

Both images are built from the same tag and carry the same version:
`docz-api:2.0.0-beta.4` and `docz-site:2.0.0-beta.4` were pushed by the same
`prerelease.yml` run. The charts still record that version twice, and the two
copies have already come apart. `charts/docz-api` is at `0.9.0` with
`appVersion: 2.0.0-beta.3`, because IMPL-0020 did not bump it; its chart job
logged `Chart 0.9.0 already published, skipping`. `charts/docz-site` is at
`0.2.0` with `appVersion: 2.0.0-beta.4`. So an install from both charts'
defaults today runs a beta.4 site against a beta.3 API. Nothing checks that
the pair matches, and the "bump once per release, before the tag" rule has to
be remembered twice.

### Observation 2: the site cannot run without the API

Four settings in docz-site depend on docz-api:

- `config.doczApiUrl` (`DOCZ_API_URL`) is `required`. The chart cannot know
  the API Service's name, so every install writes it by hand, for example
  `http://docz-api:8080`, and a typo shows up only as proxy 502s.
- `config.authProviders` "must match docz-api's own AUTH_PROVIDERS" (its
  values comment). The two charts each hold a copy, and nothing compares them.
- The site proxies `/api`, `/auth`, `/webhooks`, and `/openapi.yaml` to the
  API (`ui/server/route-class.ts`). The site is therefore the public front
  door, and docz-api's `config.authRedirectBase` has to name the site's host,
  not the API's.
- The OTel settings (`otel.*`) exist in both and only give one trace across
  the proxy hop when both are set.

Nothing flows the other way: docz-api has no setting that names the site.

### Observation 3: the charts are the same shape

The top-level values keys are nearly identical: `image`, `serviceAccount`,
`podSecurityContext`, `securityContext`, `service`, `resources`, both probes,
`config`, `otel`, `metrics`, `serviceMonitor`, `autoscaling`, `ingress`,
`httpRoute`, `extraEnv`, `extraVolumes`, `extraVolumeMounts`, and scheduling.
docz-api adds `secrets`, `store`, `queue`, `search`, `tailscale`, and
`prometheusRule`. Each of docz-site's seven templates (`deployment`,
`service`, `serviceaccount`, `hpa`, `ingress`, `httproute`,
`servicemonitor`) has a counterpart in docz-api, and docz-site's six helpers are a subset of
docz-api's 23. `values.yaml` is 503 lines for the API and 234 for the site.

In one merged chart the two Deployments would share `selectorLabels`, so a
Service needs a component label to tell them apart. docz-api already has that
problem once (the `app.kubernetes.io/component: server` fix for its backends,
chart 0.2.2), so a merged chart needs the same fix applied to a second
workload.

### Observation 4: the API chart carries stateful backends

In baked mode, docz-api's chart owns three StatefulSets with
`volumeClaimTemplates`: Postgres, Valkey, and Meilisearch. Helm cannot move a
resource from one release to another. A consolidated chart installed as a new
release cannot take over the existing PVCs unless someone does one of these:

- adopts them, by rewriting the `meta.helm.sh/release-name` and
  `release-namespace` annotations and matching the old resource names;
- uses external mode (`store.postgres.mode: external` and its siblings) and
  points the new release at the old databases; or
- dumps and restores the data.

Postgres is the source of truth. Meilisearch is rebuilt from it by the next
ingest. Valkey holds the ingest queue and the login sessions (`REDIS_URL`
serves both), so losing it drops pending ingests, which the next push or
onboard re-enqueues, and signs every user out. So only the Postgres volume
needs a real migration, and a fresh Valkey costs one re-login.
docz-site has no state and can be uninstalled and reinstalled freely.

### Observation 5: the pipeline is built for two

`ghcr.yml` resolves `component: api|ui` to a chart directory and publishes
one chart per call. The chart changelog step scopes git-cliff to
`--include-path "${CHART_DIR}/**"` because there are two charts. `ci.yml`'s
`helm-unittest` loops over `charts/*/`. `ct.yaml` lints every chart under
`charts/`. All of this works with one chart as well. The chart publish would
move from the per-component jobs to one job, and the `component` table would
lose its `chart_dir` column.

### Pros and cons

| | One chart | Two charts (today) |
| --- | --- | --- |
| Version | One `appVersion` for one product; the beta.3/beta.4 skew cannot happen | Two copies that must be bumped together, already out of step |
| Wiring | `doczApiUrl` derived from the release's own Service; `authProviders` set once | Hand-written URL; a shared setting duplicated and unchecked |
| Install | One `helm install` gets a working product | Two installs, in order, with values copied between them |
| Front door | One Ingress/HTTPRoute pointing at the site, API kept internal | Each chart has its own; nothing stops exposing both |
| Independent use | An API-only install needs `site.enabled: false` | docz-api alone is a plain install |
| Scaling/rollout | Still two Deployments with their own replicas and HPA; but a values change to one rolls through a shared release | Fully separate release histories and rollbacks |
| Chart size | One large chart (roughly 740 lines of values, 145 unit tests) | Two charts the size of their services |
| Migration | New release; the Postgres PVC needs adopting or restoring | None |
| Published packages | One package, plus a deprecation path for the two old names | Two packages, each needing its own GHCR access grant |

<!--docz:findings:end-->

<!--docz:conclusion:start-->
## Conclusion

**Answer:** Yes, on balance. The two charts describe one product that ships
from one tag, and keeping them separate already produced a version skew
(Observation 1) and a set of settings that must match by hand
(Observation 2). The main cost is moving existing releases, and only
docz-api's baked Postgres volume makes that expensive (Observation 4). The
templates are similar enough that combining them is routine work
(Observation 3).

The three questions under Recommendation were resolved on 2026-09-25: a
single merged chart (1b), the two published charts deprecated (2a), and new
installs only, with no migration path (3c).

<!--docz:conclusion:end-->

<!--docz:recommendation:start-->
## Recommendation

Write a DESIGN for one merged chart, `charts/docz`, with `api:` and `site:`
value blocks, and an IMPL whose last phase publishes it in a beta alongside
final, deprecation-noted versions of `charts/docz-api` and
`charts/docz-site`. Existing releases are not migrated: consolidation is for
new installs, before v2.0.0. Separately: bump `charts/docz-api` to
`appVersion: 2.0.0-beta.4` before the next tag, so a defaults install stops
pairing mismatched versions while both charts are still published.

### 1. What shape does one chart take?

- a. **An umbrella chart `docz`** with `docz-api` and `docz-site` as
  `file://` dependencies. The site's `doczApiUrl` is derived from the API
  subchart's Service, and the shared settings (`authProviders`, `otel`) move
  to `global`. Both unit suites keep working unchanged, the subcharts stay
  small, and each can still be turned off with `condition`. The cost is
  Helm's global/subchart plumbing and a third `Chart.yaml`. *(recommendation)*
- b. **A single merged chart** with `api:` and `site:` value blocks and
  component-scoped templates and labels. This gives the flattest values and
  one test suite, but every template, helper, and 145 tests are rewritten,
  and the `component` selector fix has to be applied to two workloads.
- c. **Keep two charts** and fix only what hurts: one `appVersion` bump rule
  enforced by a test, and documented install order and values. This is the
  cheapest option, but the duplication stays.
- d. Other.

> **Resolved 2026-09-25: (b).** One merged chart, not an umbrella. The
> templates and all 145 tests are rewritten against component-scoped
> values, and the `component` selector label goes on both workloads.

### 2. What happens to the two published charts?

- a. **Deprecate them.** Publish a final version of each whose `NOTES.txt`
  and README point at `charts/docz`, then stop publishing them at v2.0.0.
  *(recommendation)*
- b. **Keep publishing them** as standalone charts alongside the new one
  (under 1a they are the subcharts anyway). This means three packages and
  three GHCR grants.
- c. Delete them immediately.
- d. Other.

> **Resolved 2026-09-25: (a).** A final version of each carries a
> deprecation note pointing at `charts/docz`, and publishing stops at
> v2.0.0.

### 3. How does an existing release move over?

- a. **Document a migration**, tested on kind: install the new release in
  external mode against the old Postgres (or adopt its PVC by relabelling),
  let the first ingest rebuild Meilisearch, and uninstall the old releases.
  Meilisearch and Valkey data are deliberately thrown away, which signs
  users out once. *(recommendation)*
- b. **Make the new chart adopt the old resources in place** by matching
  names and annotating ownership. This avoids downtime, but it is brittle and
  specific to one release's naming.
- c. No migration path: consolidation is for new installs only, before
  v2.0.0.
- d. Other.

> **Resolved 2026-09-25: (c).** New installs only. No migration is
> documented or tested, so Observation 4's Postgres cost does not arise;
> the deprecation note in question 2 says so.

<!--docz:recommendation:end-->

<!--docz:references:start-->
## References

- [ADR-0004](../adr/0004-one-repository-docz-docz-api-and-docz-site-as-a-single-go-module.md): one repository, one release
- [DESIGN-0017](../design/0017-move-docz-site-in-ui-chartsdocz-site-and-orval-on-the-one-spec.md) and
  [IMPL-0020](../impl/0020-docz-site-move-in-v200-beta4.md): the docz-site move-in and `v2.0.0-beta.4`
- `charts/docz-api/`, `charts/docz-site/`, `ui/server/route-class.ts`
- `.github/workflows/ghcr.yml`, `.github/workflows/ci.yml`, `ct.yaml`
- Helm: [chart dependencies and subchart values](https://helm.sh/docs/chart_template_guide/subcharts_and_globals/)

<!--docz:references:end-->
