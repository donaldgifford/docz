---
id: DESIGN-0001
title: An invalid stepper widget
status: Draft
---

## Overview

```docz-stepper
steps:
  - key: Parse
    name: Parse
    does: Reads it.
  - key: verify
    name: Verify
    does: Checks it.
scenarios:
  - name: Expired
    fails_at: scope
```
