---
id: RUNBOOK-0003
title: "Enable Confluence export on docz-api"
status: Draft
author: Donald Gifford
created: 2026-10-07
---

<!-- markdownlint-disable-file MD025 MD041 -->

# RUNBOOK-0003: Enable Confluence export on docz-api

<!--toc:start-->
- [Last Verified](#last-verified)
- [Overview](#overview)
- [When to Use](#when-to-use)
- [Prerequisites](#prerequisites)
- [Procedures](#procedures)
  - [Procedure 1: Turn the export on](#procedure-1-turn-the-export-on)
    - [Steps](#steps)
    - [Verification](#verification)
    - [Rollback](#rollback)
  - [Procedure 2: Add a space to the allow-list](#procedure-2-add-a-space-to-the-allow-list)
    - [Steps](#steps-1)
    - [Verification](#verification-1)
    - [Rollback](#rollback-1)
- [Troubleshooting](#troubleshooting)
  - [Scenario: A repository reports refused](#scenario-a-repository-reports-refused)
    - [Steps](#steps-2)
  - [Scenario: DoczAPIExportFailures fires](#scenario-doczapiexportfailures-fires)
    - [Steps](#steps-3)
- [Escalation](#escalation)
- [References](#references)
<!--toc:end-->

<!--docz:last-verified:start-->
## Last Verified

| Date | PR | Commit | Verified by |
| ---- | -- | ------ | ----------- |
|      |    |        |             |

**Notes:** Not yet run end to end. Written from DESIGN-0021 and IMPL-0024;
the row comes from IMPL-0024 Phase 9's live server run.

<!--docz:last-verified:end-->

<!--docz:overview:start-->
## Overview

docz-api can export every opted-in repository to Confluence Cloud after each
ingest, into a folder per repository in a space the repository names
(DESIGN-0021). The export is off until the server has a Confluence
credential and a list of spaces it may write to. This runbook turns it on,
widens the list, and reads what each repository's last export did.

**Service:** docz-api (`charts/docz`, the `api` workload)

**Owner:** @donaldgifford

<!--docz:overview:end-->

<!--docz:when:start-->
## When to Use

- Turning on the Confluence export for a docz-api deployment
- A repository asks to export into a space the server does not yet allow
- `DoczAPIExportFailures` fires, or a repository's
  `GET /api/v1/repos/{owner}/{name}/confluence` is not `succeeded`

<!--docz:when:end-->

<!--docz:prerequisites:start-->
## Prerequisites

- An Atlassian account that can create pages and folders in every space
  the server will write to
- Access to the release's values (or its `existingSecret`) and `helm`
- A docz-api session or an `AUTH_PROVIDERS=none` install to call the API
- `curl` and `jq`

<!--docz:prerequisites:end-->

## Procedures

<!--docz:procedure:start-->
### Procedure 1: Turn the export on

Give the server a scoped token, its site, and the spaces it may write to.

<!--docz:steps:start-->
#### Steps

1. Create a scoped API token for the account at
   <https://id.atlassian.com/manage-profile/security/api-tokens> ("Create
   API token with scopes", app Confluence) with exactly these six scopes:
   `read:space:confluence`, `read:page:confluence`,
   `write:page:confluence`, `read:folder:confluence`,
   `write:folder:confluence`, and `read:hierarchical-content:confluence`.
   No delete scope: the export never deletes.

   **Expected:** the token is shown once; copy it into the secret manager,
   not a shell history

2. Put the token in the API's Secret under the key `confluence-api-token`.
   With `api.secrets.create: true` that is `api.secrets.confluenceApiToken`;
   with `existingSecret`, add the key to that Secret.

3. Set the three values and upgrade. The chart renders
   `CONFLUENCE_SITE`, `CONFLUENCE_EMAIL`, `CONFLUENCE_SPACES`, and
   `CONFLUENCE_API_TOKEN` only while `api.confluence.site` is set, and fails
   the render if the email, the spaces, or the token is missing.

   ```yaml
   api:
     confluence:
       site: https://acme.atlassian.net
       email: docz-bot@acme.com
       spaces: [DOCS]
   ```

   ```sh
   helm upgrade docz oci://ghcr.io/donaldgifford/charts/docz -f values.yaml
   ```

   **Expected:** the API pod restarts and logs
   `confluence credentials verified`; a rejected token (401) fails startup

4. In each repository to export, enable the block in `.docz.yaml` and
   merge it to the default branch.

   ```yaml
   sync:
     confluence:
       enabled: true
       site: https://acme.atlassian.net
       space: DOCS
       layout: folder
   ```

   **Expected:** the push re-ingests the repository, and the ingest
   enqueues an export

<!--docz:steps:end-->

<!--docz:verification:start-->
#### Verification

- `GET /api/v1/repos/{owner}/{name}/confluence` reports `succeeded`, the
  folder's title and URL, and the page counts:

  ```sh
  curl -s -b "docz_session=$SESSION" \
    https://docz.example.com/api/v1/repos/acme/widgets/confluence |
    jq '{status, reason, folder: .folder.title, counts}'
  ```

- The space has a folder named for the repository, holding a page per type
  and the documents under them, each titled `<folder>: …`
- docz-site shows "View in Confluence" on an exported document

<!--docz:verification:end-->

<!--docz:rollback:start-->
#### Rollback

1. Unset `api.confluence.site` and upgrade. The env is no longer rendered,
   no export is enqueued, and every repository reports
   `disabled on this server`. Pages already written stay in Confluence;
   delete them there by hand if they should go.

<!--docz:rollback:end-->
<!--docz:procedure:end-->

<!--docz:procedure:start-->
### Procedure 2: Add a space to the allow-list

Let repositories export into another space. A space may hold one repository
or several, each in its own folder.

<!--docz:steps:start-->
#### Steps

1. Confirm the account can create pages and folders in the new space.

2. Append the key to `api.confluence.spaces` and upgrade.

   ```sh
   helm upgrade docz oci://ghcr.io/donaldgifford/charts/docz -f values.yaml
   ```

   **Expected:** no `an allowed confluence space was not found` warning
   names the new space at startup

3. Re-run a repository that was refused for that space, or wait for its
   next push.

   ```sh
   kubectl exec deploy/docz-api -- /docz-api -export acme/widgets
   ```

   **Expected:** the export is enqueued with reason `manual`

<!--docz:steps:end-->

<!--docz:verification:start-->
#### Verification

- The repository's `GET …/confluence` moves from `refused` to `succeeded`

<!--docz:verification:end-->

<!--docz:rollback:start-->
#### Rollback

1. Remove the key and upgrade. Repositories naming it report `refused` on
   their next export; nothing in the space is touched.

<!--docz:rollback:end-->
<!--docz:procedure:end-->

## Troubleshooting

The `status` in `GET /api/v1/repos/{owner}/{name}/confluence` names the
outcome of the last attempt, and `reason` says why:

| Status | Meaning |
| ------ | ------- |
| `never` | no export has run for the repository |
| `disabled` | the repository's `sync.confluence` is off, or the server has no token (`disabled on this server`) |
| `refused` | the repository names a site or space this server does not allow, or `layout: page`; not retried |
| `running` | an export is in progress |
| `succeeded` | every page was created, updated, unchanged, or archived |
| `partial` | some pages failed; `pages[]` names them, and a 429, 5xx, or network failure is retried |
| `failed` | the run could not start or Confluence refused the credential; not retried until the cause is fixed |

<!--docz:scenario:start-->
### Scenario: A repository reports refused

**Likely cause:** its `.docz.yaml` names a space not in
`api.confluence.spaces`, a different site, or `layout: page`.

<!--docz:steps:start-->
#### Steps

1. Diagnose: read the `reason`; it names the space and site asked for.
2. Resolve: either add the space (Procedure 2) or have the repository name
   an allowed one. `layout: page` is for the CLI; the server writes the
   folder layout only.

<!--docz:steps:end-->
<!--docz:scenario:end-->

<!--docz:scenario:start-->
### Scenario: DoczAPIExportFailures fires

**Alert:** DoczAPIExportFailures

**Likely cause:** a revoked or under-scoped token (`failed`), or Confluence
rate limiting or errors on some pages (`partial`).

<!--docz:steps:start-->
#### Steps

1. Diagnose: read the failing repository's `GET …/confluence`. A `failed`
   status with an auth reason means the token; `pages[]` entries with a
   reason name each failed page.
2. Diagnose: search the API's logs for `export` with the repository; every
   failed attempt is logged with its cause, `retried`, and `max_retry`.
3. Resolve: for a 401 or 403, replace the token (Procedure 1, steps 1–3)
   with all six scopes. A page edited in Confluence is not a failure: the
   server always overwrites, and records the edit in that page's `edited`.

<!--docz:steps:end-->
<!--docz:scenario:end-->

<!--docz:escalation:start-->
## Escalation

| Who | When | How |
| --- | ---- | --- |
| @donaldgifford | the export fails with a valid token and allowed space | a GitHub issue on donaldgifford/docz |

<!--docz:escalation:end-->

<!--docz:references:start-->
## References

- [DESIGN-0021](../design/0021-confluence-export-from-docz-api-per-repository-folders-in.md):
  the design
- [IMPL-0024](../impl/0024-confluence-export-from-docz-api-folders-comments-and-the-server.md):
  the implementation plan
- [`charts/docz` README](../../charts/docz/README.md), "Confluence export"
- [`contrib/README.md`](../../contrib/README.md), the export metrics

<!--docz:references:end-->
