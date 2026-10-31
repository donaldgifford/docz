---
id: DESIGN-0001
title: A flow widget
status: Draft
---

## Overview

The request path.

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
