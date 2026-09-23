# Publishing to ECR — one-time AWS setup

`ecr.yml` publishes the docz-api container image and Helm chart to Amazon ECR,
called from `release.yml` after the version bump (and dispatchable standalone
via `workflow_dispatch`). It is **disabled by default**: the `publish-ecr` job
in `release.yml` is gated on the `ECR_PUBLISH_ENABLED` repository variable, and
`ecr.yml` authenticates to AWS with short-lived OIDC credentials (no static
access keys).

This document is the one-time AWS + GitHub prep that makes that workflow work.
Once complete, set `ECR_PUBLISH_ENABLED=true` (last step) to turn it on.

## What the workflow needs

- An **IAM OIDC identity provider** for GitHub Actions.
- An **IAM role** the workflow assumes via OIDC, trust-scoped to this
  repository, with permission to push to ECR.
- **ECR repositories** for the image and the chart.
- Three **GitHub repository secrets**: `ECR_AWS_ACCOUNT_ID`, `ECR_REGION`,
  `ECR_ROLE_ARN`.
- The **`ECR_PUBLISH_ENABLED`** repository variable set to `true`.

## 1. IAM OIDC identity provider

If the account does not already trust GitHub's OIDC issuer, create the provider
once:

- **Provider URL:** `https://token.actions.githubusercontent.com`
- **Audience:** `sts.amazonaws.com`

```bash
aws iam create-open-id-connect-provider \
  --url https://token.actions.githubusercontent.com \
  --client-id-list sts.amazonaws.com
```

## 2. IAM role (trust + permissions)

Create a role the workflow can assume. The **trust policy** must restrict the
assume to this repository — `repo:donaldgifford/docz-api:*` — so no other repo
can mint credentials against it:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "arn:aws:iam::<ACCOUNT_ID>:oidc-provider/token.actions.githubusercontent.com"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "token.actions.githubusercontent.com:aud": "sts.amazonaws.com"
        },
        "StringLike": {
          "token.actions.githubusercontent.com:sub": "repo:donaldgifford/docz-api:*"
        }
      }
    }
  ]
}
```

Attach a **permissions policy** allowing ECR auth + push:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": "ecr:GetAuthorizationToken",
      "Resource": "*"
    },
    {
      "Effect": "Allow",
      "Action": [
        "ecr:BatchCheckLayerAvailability",
        "ecr:InitiateLayerUpload",
        "ecr:UploadLayerPart",
        "ecr:CompleteLayerUpload",
        "ecr:PutImage",
        "ecr:BatchGetImage"
      ],
      "Resource": "arn:aws:ecr:<REGION>:<ACCOUNT_ID>:repository/docz-api"
    }
  ]
}
```

Record the role ARN — it becomes `ECR_ROLE_ARN` below.

## 3. ECR repositories

`ecr.yml` pushes both artifacts to the `docz-api` repository (the workflow's
`IMAGE_NAME` and `CHART_REPO` env). The container image and the Helm chart are
distinct OCI artifacts and coexist in the one repository — the image carries
version/`latest` tags and the chart carries its `Chart.yaml` version tag:

```bash
aws ecr create-repository --repository-name docz-api --region <REGION>
```

To keep the image and chart in separate repositories instead, create a second
repository and change `CHART_REPO` in `ecr.yml` accordingly.

## 4. GitHub repository secrets

Under **Settings → Secrets and variables → Actions → Secrets**, add:

| Secret               | Value                              |
| -------------------- | ---------------------------------- |
| `ECR_AWS_ACCOUNT_ID` | The 12-digit AWS account id.       |
| `ECR_REGION`         | The ECR region (e.g. `us-east-1`). |
| `ECR_ROLE_ARN`       | The ARN of the role from step 2.   |

## 5. Enable ECR publishing

Under **Settings → Secrets and variables → Actions → Variables**, set:

| Variable              | Value  |
| --------------------- | ------ |
| `ECR_PUBLISH_ENABLED` | `true` |

With that set, the `publish-ecr` job runs on the next push to `main`.

## Testing without a release

`ecr.yml` is directly dispatchable regardless of the `ECR_PUBLISH_ENABLED` gate.
For a smoke test, trigger it via **Actions → Publish to ECR → Run workflow**
with `dry_run: true` — it authenticates, packages the chart, and uploads the
`.tgz` as a run artifact, but skips the push and signing steps.
