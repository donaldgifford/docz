# Changelog

All notable changes to this project are documented here. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
this project adheres to [Semantic Versioning](https://semver.org/).
## [unreleased]

### Bug Fixes

- *(chart)* Scope the main Service selector to the API pods (0.2.2) ([#12](https://github.com/donaldgifford/docz-api/issues/12))

### Documentation

- *(inv)* INV-0005 changelog as a first-class docz artifact
- *(inv)* Record INV-0005 review decisions (parser in docz, OQ-2=a)
- *(inv)* Conclude INV-0005 (all OQs=a) + portable docz design handoff

### Miscellaneous Tasks

- Fix for external valkey secret

## [0.4.2] - 2026-07-23

### Bug Fixes

- *(ci)* Drop stale goreleaser GPG signing of archives ([#10](https://github.com/donaldgifford/docz-api/issues/10))

### Miscellaneous Tasks

- *(release)* Cut v0.4.2 (first working GitHub Release) ([#11](https://github.com/donaldgifford/docz-api/issues/11))

## [0.4.1] - 2026-07-22

### Features

- *(helm)* Adapt the Helm chart, CI/publish pipeline, and observability scaffolding (IMPL-0004) ([#7](https://github.com/donaldgifford/docz-api/issues/7))
- *(helm)* Baked Meilisearch existing-secret; + CI cache fix & security dep bumps ([#9](https://github.com/donaldgifford/docz-api/issues/9))

### Documentation

- *(repo-index)* Check off the IMPL-0003 testing plan
- Add DEVELOPMENT.md for new-developer onboarding
- *(deploy)* Document the GitHub App requirements for ingestion
- *(deploy)* Document reusing the GitHub App as the OAuth login provider
- *(deploy)* Note the email-permission exception in the permissions section
- *(deploy)* Add an "Enabling Okta (OIDC)" section ([#8](https://github.com/donaldgifford/docz-api/issues/8))

### Miscellaneous Tasks

- *(just)* Add dev-stack recipes wrapping docker compose
- *(dev)* Add an ngrok webhook tunnel for local GitHub App dev
- *(dev)* Add a full local environment stack (just local-up)
