---
id: DESIGN-0001
title: Widgets
status: Draft
---

# DESIGN-0001: Widgets

## Overview

A flow, a stepper, and a scorecard each fold into an expand.

```docz-flow
stages:
  - name: Ingress
    does: Terminates TLS
  - name: Auth
    does: Verifies the token
```

```docz-stepper
steps:
  - {key: parse, name: Parse, does: Reads it}
  - {key: verify, name: Verify, does: Checks it}
scenarios:
  - name: Valid
```

```docz-scorecard
checks:
  - {id: C1, name: Signed, target: "<all> & signed"}
```
