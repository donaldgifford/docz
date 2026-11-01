---
id: DESIGN-0004
title: "Widget blocks"
status: Draft
author: Fixture Author
created: 2026-03-04
---

# DESIGN-0004: Widget blocks

## Overview

Each widget fence a document can carry, and one that is invalid.

```docz-flow
title: Request path
label: Every request, left to right.
gates: [2]
stages:
  - name: Ingress
    does: Terminates TLS, routes by host
  - name: Auth
    does: Verifies the bearer token
    note: 30 s clock skew
  - name: Handler
    does: Reads through the store
legend: Every request passes all three.
alert: A token past its skew is rejected.
```

## Detailed Design

```docz-stepper
title: A token request
steps:
  - key: parse
    name: Parse
    does: Reads the bearer token from the Authorization header.
  - key: verify
    name: Verify
    title: Verify the signature
    does: Checks the signature against the issuer's JWKS.
  - key: scope
    name: Scope
    does: Resolves the repositories the token's scopes allow.
scenarios:
  - name: A valid token
    notes:
      scope: Two repositories match public:read.
  - name: An expired token
    fails_at: verify
    fail_label: Rejected
    notes:
      verify: exp is 40 s in the past, beyond the 30 s skew.
```

## Testing Strategy

```docz-scorecard
title: Release readiness
note: Measured from the release checklist.
checks:
  - id: C1
    name: Signed artifacts
    target: Every archive and image signed
    weight: 2
  - id: C2
    name: SBOM attached
    target: An SBOM per archive
example:
  generated_at: "2026-03-04"
  subject: docz v2.3.0
  results:
    - id: C1
      score: 1
      observed: cosign signatures on all images
    - id: C2
      score: null
```

## Open Questions

```docz-flow
gates: [4]
stages:
  - name: Ingress
    does: Terminates TLS
    colour: blue
  - name: Auth
```
