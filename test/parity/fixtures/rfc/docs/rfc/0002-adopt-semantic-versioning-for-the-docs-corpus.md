---
id: RFC-0002
title: "Adopt semantic versioning for the docs corpus"
status: Proposed
author: Parity Fixture
created: 2026-03-04
---

<!-- markdownlint-disable-file MD025 MD041 -->

# RFC-0002: Adopt semantic versioning for the docs corpus

<!--toc:start-->
- [Summary](#summary)
- [Problem Statement](#problem-statement)
  - [Supporting Data](#supporting-data)
- [Proposed Solution](#proposed-solution)
- [Alternatives Considered](#alternatives-considered)
- [Risks and Mitigations](#risks-and-mitigations)
- [Success Criteria](#success-criteria)
- [References](#references)
<!--toc:end-->

<!--docz:summary:start-->
## Summary

We want to adopt semantic versioning (MAJOR.MINOR.PATCH) for the docs
corpus itself, independent of the software release cadence, so that
downstream consumers like docz-api and the wiki build can pin a
known-good snapshot instead of always tracking `main`. This RFC
proposes a `docs-vX.Y.Z` tag cut on every merge that changes a
published page, plus a small CLI helper to compute the next version
from the change type.

**Owner:** docs-platform team
<!--docz:summary:end-->

<!--docz:problem:start-->
## Problem Statement

Right now the docs corpus has no version identity of its own. A
consumer that checks out `main` today and one that checks out `main`
next week can see materially different content for the same doc id,
and there is no cheap way to tell them apart. This has caused at
least two incidents where a generated site shipped a page that
referenced a section which had since been renamed or removed.

The pain shows up in a few different places:

- Downstream builds
  - docz-api serves whatever commit it last ingested, so a rollback
    on the API side does not roll back the docs it reads
  - the wiki build has no concept of "last known good," so a bad
    merge to a single RFC can break the entire nav
- Contributor workflow
  - reviewers cannot easily diff "what changed since the last
    release" without hand-picking commit ranges
  - there is no changelog entry that maps to a docs-only release

None of this is fatal today because the corpus is small, but it will
not scale past a handful of consumers.

### Supporting Data

Incident counts pulled from the last two quarters, grouped by root
cause:

| Quarter | Docs-only incidents | Root cause |
| ------- | -------------------- | ---------- |
| Q1 2026 | 2 | stale anchor after a heading rename |
| Q2 2026 | 3 | consumer read a doc mid-merge |
| Q3 2026 (partial) | 1 | wiki nav referenced a moved page |
<!--docz:problem:end-->

<!--docz:proposal:start-->
## Proposed Solution

Tag the docs corpus independently of the module's Go version. A tag
looks like `docs-v1.4.0` and follows semver rules scoped to doc
content: a PATCH bump is a typo/formatting fix, a MINOR bump adds or
substantially rewrites a doc, and a MAJOR bump removes or renames a
published doc id (a breaking change for any consumer that pinned it).

The tagging step would run in CI after merge to main, driven by a
small config block:

```yaml
docs_release:
  enabled: true
  tag_prefix: docs-v
  changelog_file: docs/CHANGELOG.md
```

and the actual tag is cut with a short script:

```bash
docz status set rfc RFC-0002 Accepted
git tag -a "docs-v${NEXT_VERSION}" -m "docs release ${NEXT_VERSION}"
git push origin "docs-v${NEXT_VERSION}"
```

Consumers then pin a tag instead of a branch, and can bump on their
own schedule.
<!--docz:proposal:end-->

<!--docz:alternatives:start-->
## Alternatives Considered

We considered dating the corpus instead of versioning it, e.g.
`docs-2026-03-04`, which is simpler to generate but does not carry
any signal about whether the change was breaking. We also considered
piggybacking on the existing software release tags, but the docs and
code change at different rates and coupling them would either starve
the docs of releases or force noisy releases just to ship a typo fix.
<!--docz:alternatives:end-->

<!--docz:risks:start-->
## Risks and Mitigations

| Risk | Impact | Likelihood | Mitigation |
| ---- | ------ | ---------- | ---------- |
| Contributors forget to classify the bump type | Medium | High | add a PR template checkbox and a CI lint that fails on an unclassified docs change |
| Tag sprawl makes the release list noisy | Low | Medium | squash same-day PATCH tags in the changelog view, keep raw tags in git |
| Consumers pin an old tag and miss a security-relevant doc fix | High | Low | publish a `docs-latest-secure` moving pointer alongside the semver tags |
<!--docz:risks:end-->

<!--docz:criteria:start-->
## Success Criteria

- docz-api and the wiki build both consume a pinned `docs-vX.Y.Z` tag
  instead of `main` within one quarter of this RFC shipping
- zero docs-only incidents attributable to a mid-merge read in the
  two quarters after rollout
- every merged doc change carries a changelog entry mapped to a tag
<!--docz:criteria:end-->

<!--docz:references:start-->
## References

- [ADR-0002: API-first refactor](../adr/0002-api-first-refactor.md)
- [git-cliff configuration docs](https://git-cliff.org/docs/configuration)
- [Semantic Versioning 2.0.0](https://semver.org/)
<!--docz:references:end-->
