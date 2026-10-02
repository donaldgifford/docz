---
id: INV-0015
title: "v2.0.0 acceptance: swap a running 1.x environment to the latest beta"
status: Open
author: Donald Gifford
created: 2026-10-02
---

<!-- markdownlint-disable-file MD025 MD041 -->

# INV-0015: v2.0.0 acceptance: swap a running 1.x environment to the latest beta

<!--toc:start-->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
  - [Observation 1: there is no in-place upgrade, by design](#observation-1-there-is-no-in-place-upgrade-by-design)
  - [Observation 2: only Postgres has to survive](#observation-2-only-postgres-has-to-survive)
  - [Observation 3: the migration chain is the same code](#observation-3-the-migration-chain-is-the-same-code)
  - [Observation 4: the webhook path only changes if the host does](#observation-4-the-webhook-path-only-changes-if-the-host-does)
  - [Observation 5: most of the acceptance checks are already written](#observation-5-most-of-the-acceptance-checks-are-already-written)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
  - [1. How does the data move?](#1-how-does-the-data-move)
  - [2. Where is the procedure recorded?](#2-where-is-the-procedure-recorded)
  - [3. How is traffic cut over?](#3-how-is-traffic-cut-over)
  - [4. What does passing mean for v2.0.0?](#4-what-does-passing-mean-for-v200)
- [References](#references)
<!--toc:end-->

<!--docz:question:start-->
## Question

Can the environment that runs the pre-move docz-api and docz-site releases
(the "1.x" environment of issue #147) be swapped to the latest
`v2.0.0-beta.N` through `charts/docz`, keeping its repositories, its GitHub
App, and its data, and does the whole product then pass the checks that gate
the v2.0.0 cut?

<!--docz:question:end-->

<!--docz:hypothesis:start-->
## Hypothesis

Yes, as a **new install in external mode against the existing Postgres**.
The server's goose migration chain came across unchanged in the graft
(IMPL-0019), so a v2 binary pointed at the 1.x database applies whatever is
missing at startup. Meilisearch is rebuilt by the next ingest and Valkey holds
only the queue and the sessions, so losing both costs one re-login and any
pending ingests, which the next push re-enqueues. The chart takes new
installs only (DESIGN-0018, INV-0014 question 3c), so the old releases are
uninstalled rather than upgraded. Nothing in the desk review blocks the
swap. What has never been done is the swap itself, and that is what closes
this investigation.

<!--docz:hypothesis:end-->

<!--docz:context:start-->
## Context

**Triggered by:** issue
[#147](https://github.com/donaldgifford/docz/issues/147), from IMPL-0022
Open Question 7.

`v2.0.0-beta.6` shipped without an install test. IMPL-0022 resolved its
question 7 by saying the whole v2 product is tested once, by swapping an
environment that runs 1.x to the latest beta, rather than by a kind-cluster
install on every beta. That swap gates the v2.0.0 cut together with #135
(restore the `latest` image tag, drop the EXPERIMENTAL markers) and #140
(delete the deprecated charts). RUNBOOK-0001 Procedure 3 covers a throwaway
kind install; it is explicitly optional for a beta and is not the acceptance
test. RUNBOOK-0002 covers webhook delivery failures. Neither covers moving a
live environment from the two old charts to the one new one.

<!--docz:context:end-->

<!--docz:approach:start-->
## Approach

1. Inventory the running environment: release names and namespace, which
   backends are baked and which external, the Ingress or HTTPRoute host, the
   GitHub App's webhook URL, the auth providers, and the OTel endpoint. Fill
   the Environment table from it.
2. Decide the data path (Recommendation question 1) and rehearse it: take a
   dump of the 1.x Postgres, restore it into a scratch database, and start a
   `docz-api:2.0.0-beta.N` container against it to watch the migrations
   apply.
3. Write the swap as a runbook **before** running it, so the first run
   verifies the runbook, the way RUNBOOK-0001's first use did (Recommendation
   question 2).
4. Translate the two old values files into one `charts/docz` values file
   using DESIGN-0018 §3's block map: `api.*`, `site.*`, top-level `store`,
   `queue`, `search`, and the shared `auth`, `otel`, `metrics`. Carry the
   secrets across by `existingSecret`. Drop `config.doczApiUrl`, which the
   chart derives.
5. Install the new release beside the old ones, confirm every pod is ready on
   the beta images, then move the front door to the new site Service and
   uninstall the old releases (Recommendation question 3).
6. Run RUNBOOK-0001 Procedure 3's checks against the real cluster. Its
   expected `{"repos":[]}` becomes the environment's actual repository list.
   Record the run in RUNBOOK-0001's Last Verified row.
7. Confirm the runbook type end to end with a repository that enables it.
   This repository already does (`.docz.yaml` `types.runbook.enabled: true`,
   two documents under `docs/runbook/`), so after its ingest:
   `GET /api/v1/repos/donaldgifford/docz/types` lists `runbook` with the
   `rb` alias, `GET …/types/rb/docs` lists RUNBOOK-0001 and RUNBOOK-0002, and
   docz-site's type nav for the repository shows the type.
8. Push a documentation change and follow RUNBOOK-0002 Procedure 1 to
   confirm the delivery is accepted and the change appears.
9. Record the #135 note: the EXPERIMENTAL markers sit on twelve v2
   packages, `pkg/runbook` included (see Environment).

<!--docz:approach:end-->

<!--docz:environment:start-->
## Environment

| Component | Version / Value |
| --------- | --------------- |
| Target chart | `charts/docz` 0.2.0, `appVersion: 2.0.0-beta.6`, signed and attested (RUNBOOK-0001 Procedure 2, 2026-10-01) |
| Target images | `ghcr.io/donaldgifford/docz-api:2.0.0-beta.6`, `ghcr.io/donaldgifford/docz-site:2.0.0-beta.6`; `latest` deliberately not moved |
| Old charts | `charts/docz-api` 0.10.0 and `charts/docz-site` 0.3.0, the deprecated finals; deleted at v2.0.0 (#140) |
| Server migrations | `20260702000000_initial_schema`, `20260710000000_add_repo_index`, `20260803000000_add_repo_changelog_file`, `20260828000000_add_repo_pages`; applied by goose at startup |
| EXPERIMENTAL packages | twelve: `pkg/{adr,design,impl,investigation,rfc,runbook,wiki}` and `pkg/doczcore/{doctemplate,index,kinds,repo,validate}` |
| Running environment | to be inventoried in step 1: release names, backend modes, host, webhook URL, providers |

<!--docz:environment:end-->

<!--docz:findings:start-->
## Findings

The findings so far are a desk review. The run itself adds its own.

### Observation 1: there is no in-place upgrade, by design

INV-0014 question 3 resolved to (c), new installs only, and DESIGN-0018
built the chart on it. Every object is `<fullname>-<role>` and the role
label `app.kubernetes.io/component` is in every selector, Deployment
`matchLabels` included. A `helm upgrade` from a `docz-api` or `docz-site`
release to `charts/docz` would fail on the immutable selector before it got
to anything else, so the swap is uninstall-and-install whatever the data
path is.

### Observation 2: only Postgres has to survive

INV-0014 Observation 4 still describes the state: Postgres is the source of
truth; Meilisearch is rebuilt from it on the next reconcile; Valkey holds the
asynq queue and the login sessions on one `REDIS_URL`. In baked mode the old
API chart owns the Postgres StatefulSet, and uninstalling its release
removes the StatefulSet and Service but leaves the PVC from its
`volumeClaimTemplates` behind. Three ways to keep the data were already
listed there: external mode against the old database, adopting the PVC, or
dump and restore. A fourth is to start empty and re-onboard. docz-api's
`-onboard owner/name@installationID` flag still exists for that, but
`installation` webhook events fire only on install, so re-onboarding is one
manual command per repository and the GitHub App stays as it is.

### Observation 3: the migration chain is the same code

The four migrations under `internal/store/migrations/` are the ones the
standalone docz-api carried; the graft kept its history and `.cliffignore`
only hides it from the changelog. goose runs them at startup
(`main()` auto-migrates, `-migrate` applies and exits). So a 2.0.0-beta.N
binary against a 1.x database applies nothing, or the tail of the chain if
the environment is behind, and either way the goose version table is the
one it expects. Step 2 of the Approach checks this on a copy before it is
relied on.

### Observation 4: the webhook path only changes if the host does

GitHub delivers to `/webhooks/github`, and docz-site proxies `/webhooks/`
to the API (`ui/server/route-class.ts`), so the public host is the site's.
If the new release takes over the same hostname, the GitHub App needs no
change. The webhook secret, the App private key, the session secret, and the
OAuth client secret have to arrive in the new release's Secret, or through
`existingSecret`, with the same values, or every delivery is answered 401
(RUNBOOK-0002, the third scenario).

### Observation 5: most of the acceptance checks are already written

RUNBOOK-0001 Procedure 3 has the install-and-query steps and the `helm test`
hook; RUNBOOK-0002 Procedure 1 has redeliver-and-confirm. What is missing is
the swap procedure itself, the values translation, and the runbook-type
check from #147's third item. This repository is the natural test subject
for that check: it enables `runbook` and has two documents, and it dogfoods
the `api:` block already.

<!--docz:findings:end-->

<!--docz:conclusion:start-->
## Conclusion

**Answer:** Inconclusive until the swap is run. The desk review found no
blocker: the chart is new-installs-only by design (Observation 1), only the
Postgres data has to be carried (Observation 2), the migration chain is the
same code (Observation 3), and the webhook keeps working if the host does
(Observation 4). The verdict comes from the run, recorded in the runbook
this investigation asks for and in RUNBOOK-0001's Last Verified row.

<!--docz:conclusion:end-->

<!--docz:recommendation:start-->
## Recommendation

Write the swap as a runbook first, rehearse the data path on a copy of the
database, then run it against the real environment with the new release
installed beside the old ones. Closing #147 is the gate: with #135 and #140
it licenses the v2.0.0 cut.

### 1. How does the data move?

- a. **External mode against the existing Postgres.** The new release sets
  `store.postgres.mode: external` and points at the old database, goose
  applies any missing migrations at startup, and nothing is re-onboarded.
  Meilisearch and Valkey start empty and refill. *(recommendation)*
- b. **Dump and restore** into the new release's baked Postgres, then
  uninstall the old. Same end state as (a) with the data inside the new
  release, at the cost of a restore and a window where the two diverge.
- c. **Start empty and re-onboard** every repository with `-onboard`. No
  migration risk at all, but a manual command per repository, and the old
  ingest history is gone.
- d. Other.

### 2. Where is the procedure recorded?

- a. **A new RUNBOOK-0003, "Swap an environment to a v2 beta"**, written
  before the run and verified by it, so its Last Verified row is filled on
  first use the way RUNBOOK-0001's was. *(recommendation)*
- b. A fourth procedure in RUNBOOK-0001.
- c. In this investigation's Findings only.
- d. Other.

### 3. How is traffic cut over?

- a. **Blue/green**: install the new release beside the old ones, confirm
  readiness, move the Ingress or HTTPRoute host to `<fullname>-site`, then
  uninstall the old releases. The old stack is the rollback for as long as
  it is still installed. *(recommendation)*
- b. Uninstall the old releases, then install the new one. Simpler, with
  downtime and no rollback but reinstalling the old charts.
- c. Other.

### 4. What does passing mean for v2.0.0?

- a. **The swap passing closes #147**, and v2.0.0 is cut once #135 and #140
  are done in the same cut. *(recommendation)*
- b. Require a soak period on the beta before the cut.
- c. Other.

<!--docz:recommendation:end-->

<!--docz:references:start-->
## References

- [#147](https://github.com/donaldgifford/docz/issues/147): this issue;
  [#135](https://github.com/donaldgifford/docz/issues/135) and
  [#140](https://github.com/donaldgifford/docz/issues/140): the rest of the
  v2.0.0 cut
- [IMPL-0022](../impl/0022-runbook-the-sixth-built-in-type-v200-beta6.md)
  Open Question 7: no install test for beta.6
- [DESIGN-0018](../design/0018-one-helm-chart-for-docz-chartsdocz-replaces-docz-api-and-docz.md)
  and [INV-0014](0014-consolidate-the-docz-api-and-docz-site-helm-charts-into-one.md):
  one chart, new installs only
- [IMPL-0019](../impl/0019-docz-api-move-in-v200-beta3.md): the docz-api
  graft that carried the migrations
- [RUNBOOK-0001](../runbook/0001-cut-a-v2-beta-release.md) Procedure 3 and
  [RUNBOOK-0002](../runbook/0002-docz-api-webhook-deliveries-fail.md)
- [`deploy/tailscale-operator.md`](../../deploy/tailscale-operator.md): the
  only Tailscale documentation, for the front door

<!--docz:references:end-->
