# Helm Chart Changelog

Changes to the `docz` Helm chart only. For application-level
changes, see the root [CHANGELOG.md](../../CHANGELOG.md).
## [2.0.0-beta.6] - 2026-10-01

### Documentation

- *(chart)* Enable runbooks only after this release is deployed

### Miscellaneous Tasks

- *(chart)* Bump charts/docz to 0.2.0 for 2.0.0-beta.6

## [2.0.0-beta.5] - 2026-09-25

### Features

- *(chart)* Charts/docz Chart.yaml
- *(chart)* Shared helpers for charts/docz
- *(chart)* Charts/docz values skeleton
- *(chart)* Charts/docz values schema skeleton
- *(chart)* Port docz-api dependency helpers to charts/docz
- *(chart)* Api block and backends in charts/docz values
- *(chart)* API workload templates in charts/docz
- *(chart)* Backend templates in charts/docz
- *(chart)* Site values block and one otel endpoint for both
- *(chart)* Site workload templates in charts/docz
- *(chart)* One Ingress and HTTPRoute pair per workload
- *(chart)* One helm test hook for both workloads
- *(chart)* Charts/docz NOTES for both workloads

### Bug Fixes

- *(chart)* Anchor charts/docz helmignore so the test hook ships
- *(chart)* Omit the site's DOCZ_AUTH_PROVIDERS when auth is none

### Refactor

- *(chart)* Shared docz.hpa helper

### Documentation

- *(chart)* Charts/docz README template
- *(chart)* Charts/docz README

### Testing

- *(chart)* Charts/docz ci-values skeleton
- *(chart)* Pin charts/docz naming and label helpers
- *(chart)* Charts/docz ci-values for the API
- *(chart)* Port docz-api suites to charts/docz
- *(chart)* Point the helpers suite at api-deployment
- *(chart)* Charts/docz ci-values for the site
- *(chart)* Port docz-site suites to charts/docz
- *(chart)* Rename a ported site test to auth.providers
- *(chart)* Wiring, selector, edge, extraLabels, Tailscale, version suites

### Miscellaneous Tasks

- *(chart)* Charts/docz .helmignore
- *(chart)* Charts/docz changelog config

