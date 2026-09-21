---
id: ADR-0001
title: "Use JWT for v1 agent capability tokens; defer SPIFFE to v2"
status: Proposed
author: Donald Gifford
created: 2026-05-10
---

<!-- markdownlint-disable-file MD025 MD041 -->

# 0001. Use JWT for v1 agent capability tokens; defer SPIFFE to v2

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

Proposed

<!--docz:context:start-->
## Context

The Agent Capability Tokens RFC (sibling, RFC-NNNN) specifies a signed-token
model for tool / egress / filesystem / Vault JIT / budget authorization. The
token format is a load-bearing choice with three realistic candidates:

1. **JWT** signed by Vault Transit, verified against Vault Transit's public-key
   endpoint
2. **SPIFFE JWT-SVID** issued by a SPIRE server, verified via SPIRE agents on
   each node
3. **Macaroon / Biscuit** capability tokens with native attenuation (the holder
   can derive more-restricted tokens without involving the issuer)

The v1 use case is single-cluster: ADK controller issues, framework verifies,
Vault verifies, egress proxy verifies. All trust roots are inside the same
Kubernetes cluster or the same Vault instance. There is no federation
requirement (cross-cluster identity, external workload identity) for v1.

SPIFFE/SPIRE is the industry-standard answer for workload identity and would
slot into the capability model identically — JWT-SVIDs are a real SPIFFE shape,
and the claim structure in the RFC works in either form. The cost of SPIFFE is
operational: running SPIRE servers (highly-available), SPIRE agents on every
node, the registration API, the trust-domain configuration, the federation
design.

Macaroons and Biscuit support delegation/attenuation natively, which is
interesting for future scenarios where agent A delegates a subset of its
capability to a downstream agent B. The v1 case does not need this.
<!--docz:context:end-->

<!--docz:decision:start-->
## Decision

Use **Vault-Transit-signed JWT** as the v1 capability token format. Defer
SPIFFE/SPIRE migration to v2, conditional on a real federation or cross-cluster
identity requirement emerging.

Specifically:

- Signing: Vault Transit, key `agent-capability-token-v1`
- Verification: public-key fetch from Vault Transit's
  `/transit/keys/agent-capability-token-v1` endpoint, cached with 5-minute TTL
- Algorithm: ES256 (ECDSA P-256, SHA-256) — small signatures, fast verification,
  supported natively by Vault Transit
- Token shape: standard RFC 7519 JWT with the claim structure defined in the
  sibling RFC

The SPIFFE migration path is preserved by:

- Keeping the JWT verification logic behind an `Authenticator` interface in the
  framework's `auth/` package
- Designing the claim structure to be SPIFFE-SVID-compatible (the RFC's claims
  map cleanly onto SPIFFE JWT-SVID claims with minimal renaming)
- Documenting the migration as a v2 candidate in the RFC's References
<!--docz:decision:end-->

<!--docz:consequences:start-->
## Consequences

<!--docz:positive:start-->
### Positive

- **No new operational footprint.** Vault is already in the stack (existing
  Vault JIT pattern). Using Vault Transit for signing adds one secrets-engine
  mount; no new servers, no new agents.
- **Familiar trust model.** JWTs are well-understood across the team; the
  verification story is "fetch public key, validate signature" —
  straightforward.
- **Direct Vault JWT-auth integration.** Vault's existing `jwt`/`oidc` auth
  method consumes our tokens natively for STS issuance, no glue code.
- **Lower learning curve.** No SPIFFE concepts (trust domains, federation,
  SVIDs, registration entries) to onboard new team members to.
- **Faster v1.** Phase 2 of the capability RFC (controller as token issuer) is
  days of work with JWT; would be weeks with SPIFFE bootstrapping.
<!--docz:positive:end-->

<!--docz:negative:start-->
### Negative

- **No native federation.** Cross-cluster or external-workload identity will
  require a migration. The RFC documents that migration target as v2 work.
- **No native attenuation.** If we want delegated capability tokens later (agent
  A → agent B with subset scopes), we'd need either controller-mediated
  re-issuance (works fine, less elegant) or a migration to Macaroons / Biscuit
  (heavier change). For v1's single-agent-per-run model, this doesn't bite.
- **Revocation is partial.** Vault Transit doesn't natively revoke JWTs. The RFC
  handles this with a small Redis-backed `jti` revocation set with TTL ≤ max
  token lifetime. SPIFFE has the same limitation; not a meaningful
  differentiator.
- **Public-key cache is a soft consistency point.** Verifiers cache the Vault
  Transit public key with a 5-minute TTL. During key rotation, there is a window
  where a verifier might still trust the old key. SPIFFE has analogous
  trust-bundle caching.
<!--docz:negative:end-->

<!--docz:neutral:start-->
### Neutral

- **Both options end up with claim-bearing tokens.** The capability RFC's claim
  structure works in either JWT or JWT-SVID form. The migration cost is mostly
  issuer / verifier infrastructure, not the claim schema.
- **Performance is comparable.** Both verify in microseconds. The difference is
  operational complexity, not runtime cost.
<!--docz:neutral:end-->
<!--docz:consequences:end-->

<!--docz:alternatives:start-->
## Alternatives Considered

**SPIFFE / SPIRE with JWT-SVIDs.** The industry-standard option. Right answer
for federated / multi-cluster workload identity. Deferred for v1 — no federation
requirement, operational cost unjustified at current scale. Migration path
documented in the RFC's References.

**Macaroons or Biscuit.** Both support attenuation natively, which would matter
if v1 needed delegation. It doesn't. Single-agent-per- run is the model. If
delegated capabilities become a real need (an agent calling another agent with a
restricted token), revisit at that point.

**OAuth 2.0 access tokens.** Effectively the same as JWT when using JWT-shaped
access tokens, with additional ceremony around token endpoints and grant flows.
No advantage for our internal-only, controller-issued case.

**X.509 certificates as the credential.** SPIFFE SVID's other form. Requires
mTLS at every enforcement point, which is heavy for in- process tool dispatch
(the primary enforcement point). Rejected.

**Custom signed-blob format.** Roll our own. Rejected on principle — JWT exists,
has libraries in every language we'll need, has well-understood verification
semantics, and has Vault-side infrastructure.
<!--docz:alternatives:end-->

<!--docz:references:start-->
## References

- Sibling: RFC-NNNN Agent Capability Tokens — specifies the full capability
  model this ADR implements the token format for
- Sibling: ADR-NNNN Use OPA for scope policy evaluation — the policy engine the
  verified claims feed into
- Internal: Agent platform RFC + DESIGN suite — existing Vault integration this
  builds on
- External:
  [RFC 7519: JSON Web Token](https://datatracker.ietf.org/doc/html/rfc7519)
- External:
  [Vault Transit secrets engine](https://developer.hashicorp.com/vault/docs/secrets/transit)
- External: [SPIFFE / SPIRE documentation](https://spiffe.io/docs/) — deferred
  to v2
- External:
  [Macaroons paper](https://research.google/pubs/macaroons-cookies-with-contextual-caveats-for-decentralized-authorization-in-the-cloud/)
  — for future-attenuation-needs reference
<!--docz:references:end-->
