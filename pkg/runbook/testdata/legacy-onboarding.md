---
id: RUNBOOK-0003
title: "Onboard a repository to docz-api"
status: Active
author: Jane Operator
created: 2025-11-12
---

# RUNBOOK-0003: Onboard a repository to docz-api

<!--docz:last-verified:start-->
## Last Verified

| Date | PR | Commit | Verified by |
| ---- | -- | ------ | ----------- |
| 2025-11-12 | #41 | 9f2c1ab | @jane, @sam |

**Notes:** Onboarded `acme/widgets` by hand while the App install was
broken.
<!--docz:last-verified:end-->

<!--docz:overview:start-->
## Overview

Adds a repository to docz-api without waiting for the GitHub App's
installation event.

**Service:** docz-api

**Owner:** Platform team
<!--docz:overview:end-->

<!--docz:when:start-->
## When to Use

- A repository was added to the App but never appeared in docz-site
<!--docz:when:end-->

<!--docz:prerequisites:start-->
## Prerequisites

- `kubectl` access to the docz namespace
- The App's installation ID
<!--docz:prerequisites:end-->

## Procedures

<!--docz:procedure:start-->
### Procedure 1: Onboard by hand

Written before the template, so its steps are headings.

<!--docz:steps:start-->
#### Steps

##### Step 1: Find the installation ID

Open the App's settings and copy the ID from the URL.

##### Step 2: Run the onboard flag

```sh
docz-api -onboard acme/widgets@12345
```
<!--docz:steps:end-->

<!--docz:verification:start-->
#### Verification

- The repository is listed by `GET /api/v1/repos`
<!--docz:verification:end-->

<!--docz:rollback:start-->
#### Rollback

1. Delete the repository row; the next install event re-adds it.
<!--docz:rollback:end-->
<!--docz:procedure:end-->

<!--docz:procedure:start-->
### Procedure 2: Re-index search

<!--docz:steps:start-->
#### Steps

- Restart the API so it re-applies the index settings
- Push any change under `docs/` to trigger an ingest
<!--docz:steps:end-->
<!--docz:procedure:end-->

## Troubleshooting

<!--docz:scenario:start-->
### Scenario: The onboard flag exits with "repo not found"

**Alert:** none

**Likely cause:** the App is not installed on the repository

<!--docz:steps:start-->
#### Steps

1. Diagnose: list the installation's repositories.

   ```sh
   gh api /installation/repositories --jq '.repositories[].full_name'
   ```

2. Resolve: add the repository to the installation, then retry Procedure 1.
<!--docz:steps:end-->
<!--docz:scenario:end-->

<!--docz:escalation:start-->
## Escalation

| Who | When | How |
| --- | ---- | --- |
| Platform on-call | The onboard flag fails twice | #platform in Slack |
<!--docz:escalation:end-->

<!--docz:references:start-->
## References

- [docz-api onboarding](https://example.com/docz-api/onboarding)
<!--docz:references:end-->
