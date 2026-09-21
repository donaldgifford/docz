---
id: ADR-0003
title: "Develop SDK, harness, and ADK layers in one repo; defer repo split"
status: Accepted
author: Donald Gifford
created: 2026-07-02
---

<!-- markdownlint-disable-file MD025 MD041 -->

# 0003. Develop SDK, harness, and ADK layers in one repo; defer repo split

<!--toc:start-->

- [Status](#status)
- [Context](#context)
- [Decision](#decision)
  - [Layering rules (the split insurance)](#layering-rules-the-split-insurance)
  - [Split triggers (when to revisit this ADR)](#split-triggers-when-to-revisit-this-adr)
- [Consequences](#consequences)
  - [Positive](#positive)
  - [Negative](#negative)
  - [Neutral](#neutral)
- [Alternatives Considered](#alternatives-considered)
- [References](#references)
<!--toc:end-->

## Status

Accepted (2026-07-02)

## Context

The original plan was two artifacts in two places: this repo as the
"lower-level" framework SDK (`pkg/llm`, `pkg/agent`, `pkg/skill`,
`pkg/llmhttp`), and the Agent Dev Kit as a product layer built on top of it —
with [RFC-0001](../rfc/0001-agent-dev-kit-on-the-agentic-go-framework.md)
assuming its own repository ("reference agents … under `examples/` in the ADK
repo") and its risk table budgeting for cross-repo version pinning and a nightly
CI run against framework HEAD.

Two things eroded that boundary:

- [DESIGN-0002](../design/0002-doc-driven-agent-loop-harness-v1-design.md)
  pulled RFC-0001's `adk run` slice — the doc-driven loop harness (`pkg/loop`,
  `pkg/tool`, `cmd/dloop`) — into this repo, because the short-term goal (run
  workloads from an IMPL doc via donald loops) needs none of the ADK's
  multi-tenant machinery and can't wait for it.
- Both surfaces are still churning. RFC-0001's own top risk — "framework churn
  breaking the ADK" — exists _because_ of the repo split: every core change
  would need a release, a pin bump, and a compat window before the layer above
  could use it.

Meanwhile the standard Go mechanics for "one repo, isolated weight" are already
documented in this repo:
[INV-0007](../investigation/0007-distribution-model-for-the-copilot-sdk-provider.md)'s
Axis A analysis (sub-modules per heavy component — the otel-contrib /
aws-sdk-go-v2 pattern) was written for the Copilot provider but is generic.

## Decision

**All layers — framework SDK, loop harness, and future ADK components — develop
in this repo**, structured so that extraction later is mechanical if a real
trigger fires. We do **not** pre-build for the split: no speculative
sub-modules, no pre-carved package trees, no work that isn't clearly useful for
the near-term goal. If we hold to the existing design principles and idiomatic
Go, moving code later is cheap; optimizing for a split we may never do is
premature.

**The near-term priority is explicit:** get to "run a workload from an IMPL doc
as a donald loop" (DESIGN-0002 through its self-hosting phase). Anything beyond
what that requires is deferred until it is explicitly needed.

### Layering rules (the split insurance)

1. **One-way imports.** Higher layers import lower layers, never the reverse:
   `cmd/*` → harness (`pkg/loop`, `pkg/tool`) → framework core (`pkg/llm`,
   `pkg/agent`, `pkg/skill`, `pkg/llmhttp`). The core must never import harness
   or ADK packages. This is the only invariant that makes a future extraction a
   `git filter-repo` + import-path rewrite instead of a refactor.
2. **Per-layer `internal/`.** The existing convention (INV-0006) continues to
   apply within each layer.
3. **Sub-module escalation, on demand only.** The moment a component drags in a
   heavy dependency tree (`client-go`, Postgres drivers, River, a controller
   runtime), that component gets its own `go.mod` (INV-0007 Option A3) so
   `pkg/llm`-only consumers never carry it in `go.sum`. The harness's
   dependencies (hashicorp/hcl, docz) are light enough to live in the root
   module; a future `adk-server` / `adk-controller` almost certainly is not.
4. **No new shared abstractions without two consumers.** Layer boundaries are
   kept honest by need, not by anticipation — same spirit as the framework's "no
   fallbacks / no premature generalization" decisions.

### Split triggers (when to revisit this ADR)

Revisit — do not preemptively act on — a repo split when one of these becomes
real:

- **Public release of the SDK core** (INV-0001 Q1's deferred decision). A repo
  can't be partially published; this is the natural forcing function.
- **Uncontainable dependency weight** — a heavy component whose isolation needs
  exceed what sub-modules handle cleanly.
- **Divergent release cadence or ownership** — a layer needs its own versioning
  story or maintainer set.
- **Operational artifacts overwhelm the library repo** — helm charts, images,
  and deploy tooling growing past what a library repo should carry.

## Consequences

### Positive

- Atomic cross-layer changes while both surfaces churn — no release/pin/bump
  cycle between the core and the harness; RFC-0001's "framework churn breaks the
  ADK" risk largely dissolves.
- One docz suite (IDs keep cross-referencing), one CI, one set of conventions,
  one `CLAUDE.md`.
- Fastest path to the actual goal: donald-loop workloads running, without
  repo-plumbing work in front of it.
- INV-0007's sub-module analysis gets adopted as living guidance instead of
  dying with the Copilot pivot.

### Negative

- Root-module `go.sum` growth must be watched — the escalation rule (rule 3) is
  the containment, and it requires discipline to apply _when_ a heavy dep lands,
  not after consumers already carry it.
- CI blast radius: every layer shares the gate. Acceptable at current scale; a
  future path-filtered matrix is available if `just ci` gets slow.
- If the public-release trigger fires, the split becomes a bigger one-time event
  than it would have been with pre-separated repos. Accepted: that event may
  never come, and the layering rules bound its cost.

### Neutral

- The private-repo posture (INV-0001 Q1) now covers all layers equally.
- RFC-0001 remains the ADK's product definition; only its placement assumption
  changes. Its phases still gate on their own prerequisites.

## Alternatives Considered

- **Two repos now (original implicit plan).** Rejected: pays the version-pinning
  and compat tax at peak churn, for a boundary we can't yet draw confidently —
  we likely won't know the right split line until the harness and first ADK
  pieces exist.
- **One repo, sub-modules everywhere from day one.** Rejected: INV-0007 itself
  documents the release-ordering and CI overhead of multi-module repos; paying
  it before any heavy dependency exists is exactly the premature optimization
  this ADR declines.
- **Harness here, ADK in its own repo later regardless.** Rejected as a standing
  commitment — it re-creates the churn tax the moment ADK work starts. If a
  trigger fires, the split happens then, on evidence.

## References

- [RFC-0001 Agent Dev Kit on the Agentic Go Framework](../rfc/0001-agent-dev-kit-on-the-agentic-go-framework.md)
  — placement assumption amended by this ADR
- [DESIGN-0002 Doc-Driven Agent Loop Harness v1 Design](../design/0002-doc-driven-agent-loop-harness-v1-design.md)
  — the first ADK slice landing in-repo
- [INV-0008 Doc-Driven Agent Loop Harness Scoping](../investigation/0008-doc-driven-agent-loop-harness-scoping-hcl-jobs-over-impl-docs.md)
  — the scoping that pulled the slice forward
- [INV-0007 Distribution Model for the Copilot SDK Provider](../investigation/0007-distribution-model-for-the-copilot-sdk-provider.md)
  — Axis A sub-module analysis, adopted here as the escalation mechanism
- [INV-0001 Agentic Go Framework Scoping](../investigation/0001-agentic-go-framework-scoping.md)
  — Q1: private module, public release deferred (the main split trigger)
- External precedent:
  [opentelemetry-go-contrib](https://github.com/open-telemetry/opentelemetry-go-contrib),
  [aws-sdk-go-v2](https://github.com/aws/aws-sdk-go-v2) — multi-module monorepos
  via per-component `go.mod`
