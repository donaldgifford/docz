---
id: RUNBOOK-0002
title: "docz-api webhook deliveries fail"
status: Draft
author: Donald Gifford
created: 2026-10-01
---

<!-- markdownlint-disable-file MD025 MD041 -->

# RUNBOOK-0002: docz-api webhook deliveries fail

<!--toc:start-->
- [Last Verified](#last-verified)
- [Overview](#overview)
- [When to Use](#when-to-use)
- [Prerequisites](#prerequisites)
- [Procedures](#procedures)
  - [Procedure 1: Redeliver and confirm](#procedure-1-redeliver-and-confirm)
    - [Steps](#steps)
    - [Verification](#verification)
    - [Rollback](#rollback)
- [Troubleshooting](#troubleshooting)
  - [Scenario: GitHub reports a TLS EOF on every delivery](#scenario-github-reports-a-tls-eof-on-every-delivery)
    - [Steps](#steps-1)
  - [Scenario: The Ingress has no hostname](#scenario-the-ingress-has-no-hostname)
    - [Steps](#steps-2)
  - [Scenario: Deliveries are answered 401](#scenario-deliveries-are-answered-401)
    - [Steps](#steps-3)
  - [Scenario: Deliveries succeed but the change never appears](#scenario-deliveries-succeed-but-the-change-never-appears)
    - [Steps](#steps-4)
  - [Scenario: A repository stopped re-ingesting after repeated failures](#scenario-a-repository-stopped-re-ingesting-after-repeated-failures)
    - [Steps](#steps-5)
- [Escalation](#escalation)
- [References](#references)
<!--toc:end-->

<!--docz:last-verified:start-->
## Last Verified

| Date | PR | Commit | Verified by |
| ---- | -- | ------ | ----------- |
|      |    |        |             |

**Notes:** Not yet run end to end. Written from
`deploy/tailscale-operator.md` and INV-0007's failure drill.

<!--docz:last-verified:end-->

<!--docz:overview:start-->
## Overview

GitHub delivers `installation`, `installation_repositories`, and `push`
events to docz-api's `POST /webhooks/github`. Each accepted delivery
enqueues an ingest, and the worker then refetches the repository. This
runbook finds where a delivery stopped: at the edge before it reached
docz-api, at docz-api's signature check, or in the ingest the delivery
queued.

**Service:** docz-api (`charts/docz`, the `api` workload)

**Owner:** @donaldgifford

<!--docz:overview:end-->

<!--docz:when:start-->
## When to Use

- The GitHub App's Recent Deliveries page shows failed deliveries
- A push to a repository's `docs/` did not appear in docz-site
- `DoczAPIIngestFailures` fires

<!--docz:when:end-->

<!--docz:prerequisites:start-->
## Prerequisites

- Admin on the GitHub App, for its settings page, Recent Deliveries, and
  the webhook secret
- `kubectl` access to the namespace docz is installed in
- For the Tailscale edge: admin on the tailnet, for its policy file and
  DNS settings
- docz-api running with `LOG_FORMAT=json`, so its logs can be filtered
  with `jq`

<!--docz:prerequisites:end-->

## Procedures

<!--docz:procedure:start-->
### Procedure 1: Redeliver and confirm

After a fix, replay the failed delivery and follow it through docz-api.

<!--docz:steps:start-->
#### Steps

1. In the GitHub App's settings, open Advanced, then Recent Deliveries.
   Pick the failed delivery and choose Redeliver. Note its delivery ID
   (the `X-GitHub-Delivery` header).
2. Find the delivery in docz-api's logs.

   ```sh
   kubectl -n docz logs deploy/docz-api --since 10m \
     | jq -c 'select(.delivery == "<delivery-id>" or (.msg | test("webhook|ingest job")))'
   ```

   **Expected:** no `webhook signature verification failed` line, and an
   `ingest job enqueued` line for the repository

3. Follow the ingest the delivery queued to the end.

   ```sh
   kubectl -n docz logs deploy/docz-api --since 10m \
     | jq -c 'select(.repo == "<owner>/<name>")'
   ```

   **Expected:** `processing ingest job`, then `ingest job complete`

<!--docz:steps:end-->

<!--docz:verification:start-->
#### Verification

- GitHub shows the redelivery with a `202`
- docz-site shows the change the delivery was for

<!--docz:verification:end-->

<!--docz:rollback:start-->
#### Rollback

1. Not applicable: redelivery is idempotent. A replayed delivery ID is
   answered `200` and does no work.

<!--docz:rollback:end-->
<!--docz:procedure:end-->

## Troubleshooting

<!--docz:scenario:start-->
### Scenario: GitHub reports a TLS EOF on every delivery

**Alert:** none; GitHub's Recent Deliveries shows the failure

**Likely cause:** the Tailscale Funnel cannot serve TLS. Either the
proxy's tag lacks the `funnel` node attribute, or the tailnet has HTTPS
certificates turned off

<!--docz:steps:start-->
#### Steps

1. Diagnose: confirm nothing reached docz-api. A TLS EOF fails before
   HTTP, so there is no webhook log line for the delivery's time.

   ```sh
   kubectl -n docz logs deploy/docz-api --since 30m | jq -c 'select(.msg | test("webhook"))'
   ```

   **Expected:** nothing near the failed delivery's timestamp

2. Diagnose: check Funnel on the operator's proxy pod.

   ```sh
   kubectl -n tailscale exec <proxy-pod> -- tailscale funnel status
   ```

   **Expected:** a Funnel entry for the docz-api host. An empty result
   means the grant is missing

3. Resolve: add the `funnel` attribute for the proxy's tag in the tailnet
   policy file.

   ```json
   "nodeAttrs": [
     { "target": ["tag:k8s"], "attr": ["funnel"] }
   ]
   ```

4. Resolve: in the admin console, open DNS, then HTTPS Certificates, and
   turn them on.
5. Run Procedure 1.

<!--docz:steps:end-->
<!--docz:scenario:end-->

<!--docz:scenario:start-->
### Scenario: The Ingress has no hostname

**Alert:** none; the GitHub App's webhook URL does not resolve

**Likely cause:** the Tailscale operator could not create the proxy for
`api.ingress`

<!--docz:steps:start-->
#### Steps

1. Diagnose: read the Ingress status.

   ```sh
   kubectl -n docz get ingress docz-api -o jsonpath='{.status.loadBalancer.ingress}'
   ```

   **Expected:** a `*.ts.net` hostname. Empty means no proxy

2. Diagnose: read the operator's logs for the Ingress.

   ```sh
   kubectl -n tailscale logs deploy/operator --since 1h | grep -i docz-api
   ```

3. Resolve: fix what the operator reports. It is usually its OAuth client
   or the tags it may assign. Then check that the hostname appears.
4. Resolve: set the GitHub App's webhook URL to
   `https://<tls-host>.<tailnet>.ts.net/webhooks/github`, then run
   Procedure 1.

<!--docz:steps:end-->
<!--docz:scenario:end-->

<!--docz:scenario:start-->
### Scenario: Deliveries are answered 401

**Alert:** none; GitHub shows `401` responses

**Likely cause:** the GitHub App's webhook secret differs from the one
docz-api holds, so the HMAC check fails before any work is done

<!--docz:steps:start-->
#### Steps

1. Diagnose: find the verification failures.

   ```sh
   kubectl -n docz logs deploy/docz-api --since 1h \
     | jq -c 'select(.msg == "webhook signature verification failed")'
   ```

   **Expected:** one line per failed delivery

2. Resolve: set one new secret in both places. Put it in the GitHub App's
   settings, then in the chart's `api.secrets.webhookSecret`, or in the
   `existingSecret` the release uses.
3. Resolve: roll the API so it reads the new secret, then run Procedure 1.

   ```sh
   kubectl -n docz rollout restart deploy/docz-api
   kubectl -n docz rollout status deploy/docz-api
   ```

<!--docz:steps:end-->
<!--docz:scenario:end-->

<!--docz:scenario:start-->
### Scenario: Deliveries succeed but the change never appears

**Alert:** `DoczAPIIngestFailures`

**Likely cause:** the ingest the delivery queued is failing. Usually the
cause is a malformed `.docz.yaml` at HEAD, or the GitHub App lacking
access to the repository

<!--docz:steps:start-->
#### Steps

1. Diagnose: confirm the push was one docz-api acts on. Only a push to the
   default branch that touches `.docz.yaml`, `docs/`, or a watched file
   enqueues an ingest. A push for a repository that was never onboarded
   logs `push for unknown repo; skipping until onboarded`.
2. Diagnose: read the failed attempts. Each one logs its cause.

   ```sh
   kubectl -n docz logs deploy/docz-api --since 1h \
     | jq -c 'select(.msg == "ingest job attempt failed") | {repo, reason, retried, max_retry, err}'
   ```

   **Expected:** the `err` field names the cause

3. Resolve: fix the cause at its source. Fix the `.docz.yaml` and push, or
   grant the App access to the repository. A new push re-enqueues the
   ingest.

<!--docz:steps:end-->
<!--docz:scenario:end-->

<!--docz:scenario:start-->
### Scenario: A repository stopped re-ingesting after repeated failures

**Alert:** `DoczAPIIngestFailures`, then silence

**Likely cause:** the ingest exhausted its retries. Since INV-0007 the
next trigger clears the finished task and re-enqueues, so a new push or
delivery is all it takes

<!--docz:steps:start-->
#### Steps

1. Diagnose: confirm the task was exhausted rather than still retrying.

   ```sh
   kubectl -n docz logs deploy/docz-api --since 24h \
     | jq -c 'select(.component == "asynq" and (.msg | test("Retry exhausted")))'
   ```

2. Resolve: fix the cause, as in the previous scenario.
3. Resolve: run Procedure 1. The re-enqueue logs
   `cleared a finished ingest task and re-enqueued` with the
   `cleared_state` it removed.

<!--docz:steps:end-->
<!--docz:scenario:end-->

<!--docz:escalation:start-->
## Escalation

| Who | When | How |
| --- | ---- | --- |
| @donaldgifford | A cause not listed here, or a fix that does not hold | GitHub issue on `donaldgifford/docz` |

<!--docz:escalation:end-->

<!--docz:references:start-->
## References

- [The Tailscale operator in front of docz-api](../../deploy/tailscale-operator.md):
  the Funnel edge and its tailnet policy
- [INV-0007](../archive/api/investigation/0007-ingest-failures-are-silent-and-block-re-ingestion.md):
  why ingest failures used to be silent, and the self-heal
- [GitHub: Redelivering webhooks](https://docs.github.com/en/webhooks/testing-and-troubleshooting-webhooks/redelivering-webhooks)

<!--docz:references:end-->
