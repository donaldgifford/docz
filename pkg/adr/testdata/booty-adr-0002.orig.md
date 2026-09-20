---
id: ADR-0002
title: "Use OPA (embedded) for scope policy evaluation"
status: Proposed
author: Donald Gifford
created: 2026-05-10
---

<!-- markdownlint-disable-file MD025 MD041 -->

# 0002. Use OPA (embedded) for scope policy evaluation

<!--toc:start-->

- [Status](#status)
- [Decision](#decision)
- [Context](#context)
- [Decision rationale](#decision-rationale)
- [Consequences](#consequences)
  - [Positive](#positive)
  - [Negative](#negative)
  - [Neutral](#neutral)
- [Alternatives Considered](#alternatives-considered)
- [Implementation notes](#implementation-notes)
- [References](#references)
<!--toc:end-->

## Status

Proposed

## Decision

Use **Open Policy Agent (OPA) embedded as a Go library** via
`github.com/open-policy-agent/opa/rego` for evaluating agent capability-token
scope checks at the framework's tool dispatcher and at out-of-process
enforcement points (egress proxy, Vault JIT).

Policies are written in Rego, loaded from a `policies/` directory at framework
startup, compiled once, and evaluated per call against cached compiled queries.

The OPA daemon is **not** used. The framework links the OPA evaluator as a
library; no IPC, no subprocess, no shared OPA service.

## Context

The Agent Capability Tokens RFC (RFC-NNNN) requires a policy engine to evaluate
capability claims against tool / egress / filesystem / AWS-JIT access requests.
Realistic candidates:

1. **Open Policy Agent (OPA)** with Rego — Go-native, broadest ecosystem,
   declarative
2. **Cedar** — AWS's open-source IAM-shaped authorization language, Go SDK
   available (`github.com/cedar-policy/cedar-go`)
3. **Custom Go matcher** — write `Allowed(scope string, granted []string) bool`
   and call it a day
4. **Casbin** — Go-native access-control library with multiple built-in models
   (RBAC, ABAC, ACL)

The capability model's policy needs include:

- Simple scope membership checks (`scope ∈ granted_scopes`)
- Resource-bound constraints (this scope, only for resources in tenant X)
- Time-bound constraints (action allowed during business hours, for some agents)
- Aggregate constraints (max N invocations of this scope per agent run)
- Policies shareable across the framework, egress proxy, and Vault JIT (same
  policy language, possibly shared rules)

The org's existing policy-engine surface area:

- **Conftest for Terraform IaC** — already in production in the homelab
  Kubernetes security stack. Rego.
- No existing Cedar surface
- No existing Casbin surface

## Decision rationale

**One policy language across the stack.** Adopting OPA puts Rego in the
agent-authz path alongside the existing Conftest-for-Terraform usage. Engineers
who learn Rego for one use case can apply it to the other. Adopting Cedar would
mean two policy languages, both used by the same teams, with no shared mental
model.

**Go-native, embedded, no daemon.** The `github.com/open-policy-agent/opa/rego`
package compiles Rego policies into a query evaluator that runs in-process. No
subprocess, no IPC, no failure mode where the agent runs while OPA is down.
Per-call evaluation is microsecond-scale on cached compiled queries.

**Mature tooling.** OPA's tooling ecosystem (regal for linting, `opa test` for
policy tests, `opa fmt` for formatting, `opa eval` for ad-hoc debugging, the OPA
Playground for prototyping, the Decision Logs feature for audit) is broader and
more mature than Cedar's Go-side ecosystem at the time of this decision.

**Future K8s admission alignment.** If/when we add admission policies on
AgentType / AgentRun CRDs (validating that a new AgentType doesn't request
scopes its owning team isn't authorized to grant), those policies are the same
language as the agent-side policies — Gatekeeper is the natural integration
point and is also OPA-based.

**Future Envoy / service-mesh alignment.** If/when the egress proxy grows into
something Envoy-shaped, Envoy's `ext_authz` filter natively talks to OPA, no
glue.

## Consequences

### Positive

- **Single policy language across IaC, K8s admission, agent capability, and
  egress.** One Rego skill set serves four use cases.
- **No new daemon to operate.** Embedded library; if the agent runs, the policy
  engine runs.
- **Decision Logs feature gives free audit trail.** OPA can emit every
  evaluation as a structured log; ties cleanly into Langfuse / OTel.
- **regal lint** catches Rego anti-patterns in CI; the language is notorious for
  spaghetti without discipline, and the linter is the discipline.
- **Familiar idiom.** OPA's `data.<package>.allow` and `input.<...>` patterns
  are widely documented; new engineers ramp in days.

### Negative

- **Rego is its own language.** New hires unfamiliar with it pay a learning
  cost. Mitigated by the existing Conftest usage — several team members already
  know Rego.
- **Cedar's IAM-shape syntax is genuinely nicer for our specific use case.**
  Cedar's `permit(principal, action, resource)` reads more naturally for
  capability authz than Rego's general-purpose data structure. Accepted cost;
  consistency wins.
- **OPA bundle size.** The embedded evaluator adds ~5MB to the framework binary.
  Not a meaningful cost in a containerized deployment.
- **Policy debugging requires Rego comfort.** `opa eval --explain` is excellent
  but assumes Rego literacy. Cedar has similar tooling but with a more
  approachable language. Net wash given the consistency benefit.

### Neutral

- **Both OPA and Cedar are OSS, both have Go SDKs.** Licensing and
  language-support concerns are equivalent.
- **Both evaluate in microseconds.** Performance is not the deciding factor.
- **Both are likely to remain healthy projects.** OPA is CNCF graduated; Cedar
  is AWS-backed with growing community adoption. Neither is a survival risk.

## Alternatives Considered

**Cedar (cedar-policy/cedar-go).** AWS's open-source IAM-shaped authorization
language. Genuinely better syntax for our specific use case —
`permit(principal, action, resource)` is exactly the shape of authz we're doing.
Rejected for v1 because the team already has Rego muscle (Conftest), one policy
language across the stack beats two cleaner ones, and the consistency win
matters more than the syntax win. Cedar will be re-evaluated at the v2 review
point if the OPA story shows pain.

**Custom Go matcher.** Write `Allowed(scope, granted []string) bool` and ship
it. Works for trivial scope membership. Breaks down at the first resource-bound
or time-bound policy. Rejected as a v1 dead end — same problem we'd have if we'd
hand-rolled IAM evaluation instead of using `iam:SimulateCustomPolicy`.

**Casbin.** Go-native, multiple built-in models, mature library. Doesn't share
an idiom with Conftest; less mainstream for the admission-control case; less
mature ecosystem than OPA at the relevant scale. Rejected — OPA's ecosystem
advantage is real.

**Inline policies in Go code (no engine, no DSL).** Tools declare their authz
inline as Go functions. Rejected — defeats the point of separating policy from
code. Changes require recompile and redeploy of the agent; policies should be
reviewable independently.

## Implementation notes

- **Embed:** `github.com/open-policy-agent/opa/rego`. Pin a stable version
  (currently `v0.65.0` or newer); track OPA releases.
- **Policy layout:** Each agent / framework component has its own `policies/`
  directory baked into the container image. Shared policy fragments live in
  `policies/lib/` and are imported via Rego's `import` statement.
- **Compilation:** Policies are compiled once at framework startup via
  `rego.New(...).PrepareForEval(ctx)`. The compiled `PreparedEvalQuery` is
  cached for the lifetime of the run.
- **Lint:** `regal lint policies/` in CI. Block on warnings, not just errors,
  until the codebase settles.
- **Test:** `opa test policies/` in CI. Each rule has an `allow` test and a
  `deny` test minimum.
- **Decision Logs:** wired into Langfuse / OTel via a custom decision logger
  that emits structured events per evaluation. The agent capability RFC's audit
  story uses these.
- **Hot-reload:** explicitly not supported in v1. Policies ship as new container
  builds. The trust-boundary argument that motivates the capability RFC also
  rules out hot-reload.

## References

- Sibling: RFC-NNNN Agent Capability Tokens — uses this engine
- Sibling: ADR-NNNN Use JWT for v1 — produces the claims this engine evaluates
- Internal: Existing Conftest-for-Terraform-IaC usage in homelab Kubernetes
  security stack — same Rego skill set
- External: [Open Policy Agent](https://www.openpolicyagent.org/)
- External:
  [OPA Go library](https://pkg.go.dev/github.com/open-policy-agent/opa/rego)
- External: [regal Rego linter](https://github.com/StyraInc/regal)
- External: [Cedar](https://www.cedarpolicy.com/) — rejected; documented for the
  v2 review point
- External: [Casbin](https://casbin.org/) — rejected
