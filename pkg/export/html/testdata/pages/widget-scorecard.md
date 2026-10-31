---
id: DESIGN-0001
title: A scorecard widget
status: Draft
---

## Overview

How ready the release is.

```docz-scorecard
title: Release readiness
note: Measured from the release checklist.
checks:
  - id: C1
    name: Signed artifacts
    target: Every archive and image signed
    weight: 2
    fixes: Testing
  - id: C2
    name: SBOM attached
    target: An SBOM per archive
example:
  generated_at: "2026-10-31"
  subject: docz v2.3.0
  results:
    - id: C1
      score: 1
      observed: cosign signatures on all images
    - id: C2
      score: null
```

## Testing

Run the release checklist.
