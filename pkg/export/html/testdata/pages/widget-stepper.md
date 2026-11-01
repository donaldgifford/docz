---
id: DESIGN-0001
title: A stepper widget
status: Draft
---

## Overview

What happens to a token.

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
