---
id: RUNBOOK-0003
title: "Onboard a repository to docz-api"
status: Active
author: Jane Operator
created: 2025-11-12
---

# RUNBOOK-0003: Onboard a repository to docz-api

## Last Verified

| Date | PR | Commit | Verified by |
| ---- | -- | ------ | ----------- |
| 2025-11-12 | #41 | 9f2c1ab | @jane, @sam |

**Notes:** Onboarded `acme/widgets` by hand while the App install was
broken.

## Overview

Adds a repository to docz-api without waiting for the GitHub App's
installation event.

**Service:** docz-api

**Owner:** Platform team

## When to Use

- A repository was added to the App but never appeared in docz-site

## Prerequisites

- `kubectl` access to the docz namespace
- The App's installation ID

## Procedures

### Procedure 1: Onboard by hand

Written before the template, so its steps are headings.

#### Steps

##### Step 1: Find the installation ID

Open the App's settings and copy the ID from the URL.

##### Step 2: Run the onboard flag

```sh
docz-api -onboard acme/widgets@12345
```

#### Verification

- The repository is listed by `GET /api/v1/repos`

#### Rollback

1. Delete the repository row; the next install event re-adds it.

### Procedure 2: Re-index search

#### Steps

- Restart the API so it re-applies the index settings
- Push any change under `docs/` to trigger an ingest

## Troubleshooting

### Scenario: The onboard flag exits with "repo not found"

**Alert:** none

**Likely cause:** the App is not installed on the repository

#### Steps

1. Diagnose: list the installation's repositories.

   ```sh
   gh api /installation/repositories --jq '.repositories[].full_name'
   ```

2. Resolve: add the repository to the installation, then retry Procedure 1.

## Escalation

| Who | When | How |
| --- | ---- | --- |
| Platform on-call | The onboard flag fails twice | #platform in Slack |

## References

- [docz-api onboarding](https://example.com/docz-api/onboarding)
