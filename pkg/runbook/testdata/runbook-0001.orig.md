---
id: RUNBOOK-0001
title: "Cut a v2 beta release"
status: Active
author: Donald Gifford
created: 2026-10-01
---

<!-- markdownlint-disable-file MD025 MD041 -->

# RUNBOOK-0001: Cut a v2 beta release

<!--toc:start-->
- [Last Verified](#last-verified)
- [Overview](#overview)
- [When to Use](#when-to-use)
- [Prerequisites](#prerequisites)
- [Procedures](#procedures)
  - [Procedure 1: Prepare](#procedure-1-prepare)
    - [Steps](#steps)
    - [Verification](#verification)
    - [Rollback](#rollback)
  - [Procedure 2: Tag and verify](#procedure-2-tag-and-verify)
    - [Steps](#steps-1)
    - [Verification](#verification-1)
    - [Rollback](#rollback-1)
  - [Procedure 3: Install and smoke-test](#procedure-3-install-and-smoke-test)
    - [Steps](#steps-2)
    - [Verification](#verification-2)
    - [Rollback](#rollback-2)
- [Troubleshooting](#troubleshooting)
  - [Scenario: A chart push fails with 403 write_package](#scenario-a-chart-push-fails-with-403-writepackage)
    - [Steps](#steps-3)
  - [Scenario: Pods fail to pull the image after install](#scenario-pods-fail-to-pull-the-image-after-install)
    - [Steps](#steps-4)
- [Escalation](#escalation)
- [References](#references)
<!--toc:end-->

## Last Verified

| Date | PR | Commit | Verified by |
| ---- | -- | ------ | ----------- |
| 2026-09-25 | #137 | e41203e | @donaldgifford |

**Notes:** Run end to end for `v2.0.0-beta.5`, install included. The
`charts/docz` push did not fail with a 403, so that scenario was not
exercised.


## Overview

This runbook cuts a `v2.0.0-beta.N` release of docz, docz-api, docz-site,
and the `charts/docz` chart from one tag, and checks what the tag published.
Betas on the v2 line are cut by hand from a merge commit on `main`. Every PR
carries `dont-release`, so nothing publishes on merge (ADR-0002 Decision 6).

**Service:** docz release pipeline (`prerelease.yml`, `ghcr.yml`, goreleaser)

**Owner:** @donaldgifford


## When to Use

- A PR that should ship has merged to `main` with a merge commit, and its
  IMPL's release phase says to cut the next beta
- A beta needs re-cutting after a broken publish (tag a new `N`; never
  move a pushed tag)


## Prerequisites

- Push access to `donaldgifford/docz`, and permission to push tags
- `just`, `goreleaser`, `gh`, `helm` (4.x), `cosign`, and `docker`
  installed (`mise install` provides them)
- `gh auth status` is logged in to github.com
- For the install procedure: `kind` and a local Docker daemon
- The previous beta's number, from `git tag --list 'v2.0.0-beta.*'`


## Procedures

### Procedure 1: Prepare

Get `main` into the state the tag will record: the chart bumped and the
release configuration valid.

#### Steps

1. Before the PR merges, bump `charts/docz/Chart.yaml`: raise `version`,
   and set `appVersion` to the new beta **without** a `v` prefix. Then
   regenerate the chart docs and run its tests.

   ```sh
   just chart docs
   just chart unittest
   ```

   **Expected:** `appVersion: "2.0.0-beta.N"`; the unit tests pass

2. Once the PR is merged, check out the merge commit on `main`.

   ```sh
   git checkout main && git pull --ff-only --no-tags origin main
   gh pr view <PR> --json mergeCommit -q .mergeCommit.oid
   git rev-parse HEAD
   ```

   **Expected:** both commands print the same SHA

3. Confirm the merge's own `release.yml` run published no chart.

   ```sh
   gh run list --workflow release.yml --commit "$(git rev-parse HEAD)" \
     --json databaseId -q '.[0].databaseId' | xargs gh run view \
     --json jobs -q '.jobs[] | "\(.name): \(.conclusion)"'
   ```

   **Expected:** every `Publish chart` job is `skipped`

4. Validate the release configuration for both binaries.

   ```sh
   just release-check
   just api release-check
   ```

   **Expected:** `1 configuration file(s) validated`, twice


#### Verification

- `HEAD` is the merge commit, and `charts/docz` carries the new bare
  `appVersion`
- Both release checks pass


#### Rollback

1. Nothing has been published yet. Fix forward on a branch and merge
   again.


### Procedure 2: Tag and verify

Push the tag, let `prerelease.yml` publish, and check every artifact it
was supposed to produce.

#### Steps

1. Tag the merge commit and push the tag.

   ```sh
   just release v2.0.0-beta.N
   ```

   **Expected:** the tag appears with `git ls-remote --tags origin 'v2.0.0-beta.N'`

2. Watch the pre-release run to the end.

   ```sh
   run=$(gh run list --workflow prerelease.yml -L 1 --json databaseId -q '.[0].databaseId')
   gh run watch "$run" --exit-status --interval 30
   gh run view "$run" --json jobs -q '.jobs[] | "\(.name): \(.conclusion)"'
   ```

   **Expected:** every job `success`, except the ECR jobs and the chart
   component's image job, which are `skipped`

3. Check the GitHub release.

   ```sh
   gh release view v2.0.0-beta.N --json isPrerelease,assets -q '"prerelease=\(.isPrerelease)", (.assets[].name)'
   ```

   **Expected:** `prerelease=true`, the `docz_*` and `docz-api_*` archives
   with their `.spdx.json` SBOMs, and `checksums.txt` with its `.sig`

4. Check both images. The tag is bare semver, and `latest` must not have
   moved to this release.
   1. Inspect the new tag on each image.

      ```sh
      for img in docz-api docz-site; do
        docker buildx imagetools inspect "ghcr.io/donaldgifford/${img}:2.0.0-beta.N" \
          --format '{{json .Manifest.Digest}}'
      done
      ```

   2. Inspect `latest` the same way. Write `${img}`, not `$img`: zsh reads
      `$img:l` as a lower-case modifier.

   **Expected:** each image's `latest` digest differs from its beta digest

5. Check each chart's metadata and supply-chain artifacts.

   ```sh
   for c in docz:<version>; do
     helm show chart "oci://ghcr.io/donaldgifford/charts/${c%%:*}" --version "${c##*:}" \
       | grep -E '^(name|version|appVersion|deprecated):'
     cosign tree "ghcr.io/donaldgifford/charts/${c%%:*}:${c##*:}" \
       | grep -oE 'sigstore.dev/cosign/sign|slsa.dev/provenance' | sort -u
   done
   ```

   **Expected:** a bare `appVersion: 2.0.0-beta.N`, plus both a signature
   and SLSA provenance. A chart whose version did not change is skipped by
   the publish job's idempotency check and keeps its old artifacts


#### Verification

- The pre-release exists with both binaries' archives and signed checksums
- Both images are at the new bare tag, signed, and `latest` did not move
- `charts/docz` is published at its new version, signed and attested


#### Rollback

1. Do not delete or move the tag: a consumer may already have pulled it.
2. Mark the GitHub release as broken in its notes.

   ```sh
   gh release edit v2.0.0-beta.N --notes "Broken: see v2.0.0-beta.N+1"
   ```

3. Fix forward and cut the next beta with this runbook.


### Procedure 3: Install and smoke-test

Install the published chart on a throwaway kind cluster and check that the
site reaches the API. This is optional for a beta (IMPL-0022 OQ 7). The
whole product is tested by swapping a real environment to the beta before
v2.0.0.

#### Steps

1. Create a cluster and a values file with login off and a dummy GitHub
   App key.

   ```sh
   kind create cluster --name docz-beta --wait 2m
   openssl genrsa -out /tmp/dummy.pem 2048
   cat > /tmp/beta.yaml <<'YAML'
   auth:
     providers: "none"
   api:
     config:
       appId: "12345"
       githubApiBase: "http://127.0.0.1:1"
     secrets:
       webhookSecret: "dummy-webhook-secret"
   search:
     meili:
       masterKey: "smoke-test-master-key-0123456789"
   YAML
   ```

   Never leave a top-level key such as `site:` with nothing under it. YAML
   reads it as `null`, which deletes the chart's whole block and fails the
   render with a nil pointer.

2. Install the chart from OCI with no image override, so the chart's own
   `appVersion` picks the images.

   ```sh
   helm install docz oci://ghcr.io/donaldgifford/charts/docz --version <version> \
     --kube-context kind-docz-beta -n docz --create-namespace -f /tmp/beta.yaml \
     --set-file api.secrets.privateKey=/tmp/dummy.pem --wait --timeout 8m
   ```

   **Expected:** `STATUS: deployed`; every pod ready on the
   `2.0.0-beta.N` images

3. Query the API through the site's Service, then run the chart's test
   hook.

   ```sh
   kubectl --context kind-docz-beta -n docz port-forward svc/docz-site 18080:80 &
   curl -s -w ' %{http_code}\n' localhost:18080/api/v1/repos
   kill %1
   helm test docz --kube-context kind-docz-beta -n docz
   ```

   **Expected:** `{"repos":[]} 200`, and the test suite `Succeeded`

4. Delete the cluster.

   ```sh
   kind delete cluster --name docz-beta
   ```


#### Verification

- The site answers `/api/v1/repos` with 200, and `helm test` succeeds


#### Rollback

1. Not applicable: the cluster is throwaway. Delete it.


## Troubleshooting

### Scenario: A chart push fails with 403 write_package

**Alert:** none; the `Publish chart` job fails

**Likely cause:** the GHCR package is new and this repository has no
Actions access to it yet

#### Steps

1. Confirm the failure is the access grant, not a token or login problem.

   ```sh
   gh run view <run> --log-failed | grep -i 'write_package\|403'
   ```

   **Expected:** a `403` naming `write_package`

2. In GitHub, open Packages, then `charts/<name>`, then Package settings,
   then Manage Actions access. Add `donaldgifford/docz` with the **Write**
   role.
3. Re-run only the failed job. The chart job is idempotent, and nothing
   else in the run depends on it.

   ```sh
   gh run rerun <run> --failed
   ```


### Scenario: Pods fail to pull the image after install

**Alert:** none; pods sit in `ImagePullBackOff`

**Likely cause:** a `v`-prefixed `appVersion`. The image tag is bare
semver (`2.0.0-beta.N`), because the publish workflow's metadata action
strips the `v`.

#### Steps

1. Read the chart's `appVersion`.

   ```sh
   helm show chart oci://ghcr.io/donaldgifford/charts/docz --version <version> | grep appVersion
   ```

   **Expected:** no leading `v`

2. If it has one, fix `charts/docz/Chart.yaml`, bump the chart `version`,
   and cut the next beta. A published chart version is never overwritten.


## Escalation

| Who | When | How |
| --- | ---- | --- |
| @donaldgifford | Any publish job fails for a reason not listed here | GitHub issue on `donaldgifford/docz` |


## References

- [IMPL-0021](../impl/0021-chartsdocz-one-helm-chart-v200-beta5.md): Phase 6,
  the beta.5 release this runbook writes down
- [ADR-0002](../adr/0002-docz-is-an-api-package-whose-first-consumer-is-the-cli.md):
  hand-cut betas (Decision 6)
- [DESIGN-0018](../design/0018-one-helm-chart-for-docz-chartsdocz-replaces-docz-api-and-docz.md):
  the chart and its publish path

