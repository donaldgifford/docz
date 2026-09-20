---
id: ADR-0001
title: "Run the IMPL loop worker as its own repository"
status: Proposed
author: Donald Gifford
created: 2026-09-12
---

<!-- markdownlint-disable-file MD025 MD041 -->

# ADR-0001: Run the IMPL loop worker as its own repository

<!--toc:start-->
- [Status](#status)
- [Context](#context)
- [Decision](#decision)
- [Consequences](#consequences)
  - [Positive](#positive)
  - [Negative](#negative)
  - [Neutral](#neutral)
- [Alternatives Considered](#alternatives-considered)
- [References](#references)
<!--toc:end-->

## Status

Accepted

## Context

The Temporal worker that executes docz IMPL docs (see DESIGN-0001) could live in
one of three places: the homelab GitOps repository where Temporal itself is
deployed, the docz repository alongside `docz-api` and `doczcore` which it
depends on, or a repository of its own.

Forces at play:

- A Temporal worker is a deployable with a determinism contract. Workflow code
  changes need replay tests against recorded histories, and a worker version is
  pinned to a task queue during rollouts. That is easiest to reason about when
  the repository boundary is the worker boundary.
- The homelab repository holds cluster live state (ArgoCD/Kargo, Helm,
  Kustomize). Its commit stream is unrelated to workflow logic.
- The stated goal is to lift this worker to the work platform once proven. Code
  embedded in the homelab repository cannot be lifted without surgery.
- docz is a documentation lifecycle tool used by humans and CI in every
  repository. The worker pulls in Temporal SDK, Kubernetes client, git plumbing,
  and an agent runner image — a much larger dependency surface.
- Every comparable tool in this ecosystem (fwsync, sluice, champs,
  repo-guardian) is one repository with an engine package, a thin CLI, and a
  deployment.

## Decision

The Temporal worker, its CLI, and its agent runner image live in a dedicated
repository, `tempy`. The repository is the worker: the IMPL loop is its first
workflow package, and later workflows are added as packages that register on
the same worker, not as new repositories or deployables. It imports `doczcore`
as a versioned Go module. The homelab repository references the worker only by
container image tag in its GitOps manifests.

## Consequences

### Positive

- One worker version ↔ one task queue ↔ one repository tag. Rollouts, rollbacks,
  and replay testing map onto git history directly.
- The repository can be moved to the work forge with a change of module path and
  runner image, nothing else.
- docz keeps its small dependency surface; `doczcore` changes needed by the
  worker are ordinary versioned releases.
- Same shape as every other tool: engine package, CLI, deployment.

### Negative

- One more repository to bootstrap (docz config, justfile, CI, release).
- `doczcore` changes require a release-then-bump cycle rather than an in-place
  edit. Acceptable; `go.work` covers local development.

### Neutral

- Shared Temporal worker scaffolding (client setup, interceptors, OTel and
  Langfuse wiring, search-attribute helpers, git activities) will be duplicated
  until a second worker exists. It is extracted into a library then, not before.

## Alternatives Considered

- **Inside the homelab repository.** Rejected: couples workflow versioning to
  cluster state commits; cannot be lifted to work.
- **Inside the docz repository next to `docz-api`.** Rejected: drags a large
  runtime dependency surface into a tool that must stay light; release cadence
  of a worker differs from a CLI library.
- **A monorepo of all Temporal workers.** Deferred: revisit when the SSO/IAM and
  Wiz workflows exist and the shared-scaffolding library is extracted.

## References

- DESIGN-0001 Temporal-orchestrated IMPL loop execution
- Homelab Temporal execution-plane RFC
