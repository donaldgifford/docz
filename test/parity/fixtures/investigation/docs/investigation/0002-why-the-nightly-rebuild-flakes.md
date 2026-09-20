---
id: INV-0002
title: "Why the nightly rebuild flakes"
status: Concluded
author: Parity Fixture
created: 2026-03-04
---

<!-- markdownlint-disable-file MD025 MD041 -->

# INV-0002: Why the nightly rebuild flakes

<!--toc:start-->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
  - [Cache directory is shared across jobs](#cache-directory-is-shared-across-jobs)
  - [Failure signature matches a truncated manifest read](#failure-signature-matches-a-truncated-manifest-read)
  - [Overlap window correlates with failure nights](#overlap-window-correlates-with-failure-nights)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
- [References](#references)
<!--toc:end-->

<!--docz:question:start-->
## Question

Why does the nightly documentation rebuild job fail intermittently,
roughly one run in five, with no obvious pattern in the commit history
that triggered it? We need a concrete root cause, not just a retry
workaround, because the flake is starting to erode trust in the
pipeline's status checks.
<!--docz:question:end-->

<!--docz:hypothesis:start-->
## Hypothesis

Two concurrent nightly jobs (the docs rebuild and the link checker) are
racing on a shared cache directory. If the theory holds, failures
should cluster on nights when both jobs happen to overlap in their
scheduling window, and the failure signature should point at a missing
or partially written file rather than a network timeout.
<!--docz:hypothesis:end-->

<!--docz:context:start-->
## Context

The nightly rebuild has been in place for about six months without
issue. Failures started showing up after we added a second scheduled
job (the link checker) that runs on an overlapping cron window and
reuses the same build cache mount to save on cold-start time.

**Triggered by:** issue #412
<!--docz:context:end-->

<!--docz:approach:start-->
## Approach

1. Reproduce the flake locally by running both jobs concurrently
   against a shared cache directory and watching for corrupted or
   half-written cache entries.
2. Add verbose logging around cache reads and writes in the rebuild
   job, then trigger ten consecutive nightly runs and capture the
   full logs for each.
3. Compare timestamps between the two jobs' cache writes on failing
   runs versus passing runs to confirm the race window.
<!--docz:approach:end-->

<!--docz:environment:start-->
## Environment

| Component | Value |
| --------- | ----- |
| CI runner | GitHub Actions, ubuntu-22.04 |
| Cache path | `/opt/build-cache/docs` |
| Rebuild job schedule | `17 2 * * *` (cron, UTC) |
| Link checker schedule | `20 2 * * *` (cron, UTC) |
<!--docz:environment:end-->

<!--docz:findings:start-->
## Findings

The failures were not random. Once we had logging in place, a clear
pattern emerged within three nights.

### Cache directory is shared across jobs

Both the rebuild job and the link checker mount the same
`/opt/build-cache/docs` path, and neither job takes a lock before
writing to it. When their schedules overlap by even a few minutes, one
job can delete or truncate a file the other is mid-write on.

### Failure signature matches a truncated manifest read

The rebuild job's crash log consistently points at a JSON decode
error on the cache manifest, not a missing file. That is consistent
with a partial write being read by the other process.

```text
2026-03-02T02:19:44Z ERROR rebuild: failed to parse cache manifest
    /opt/build-cache/docs/manifest.json: unexpected end of JSON input
2026-03-02T02:19:44Z ERROR rebuild: falling back to cold cache
2026-03-02T02:19:47Z ERROR rebuild: exit status 1 (nondeterministic)
```

### Overlap window correlates with failure nights

Cross-referencing job start times against the incident log confirmed
the race window directly.

- Nights with a rebuild failure: link checker start time was within
  90 seconds of the rebuild's cache-write phase.
- Nights with a clean rebuild: link checker either finished before
  the rebuild started or started more than five minutes after it.
- No failure was correlated with network errors, disk pressure, or a
  specific commit range.
<!--docz:findings:end-->

<!--docz:conclusion:start-->
## Conclusion

**Answer:** Confirmed. The nightly rebuild flakes because the rebuild
job and the link checker job share a build cache directory with no
locking or job-scoped path, so an overlapping run can truncate the
cache manifest mid-write and corrupt the other job's read.
<!--docz:conclusion:end-->

<!--docz:recommendation:start-->
## Recommendation

Give each job its own cache subdirectory (for example
`/opt/build-cache/docs-rebuild` and `/opt/build-cache/link-checker`)
so the two jobs never touch the same files, and stagger the cron
schedules by at least ten minutes as a defense-in-depth measure while
the subdirectory change rolls out.
<!--docz:recommendation:end-->

<!--docz:references:start-->
## References

- [issue #412](https://github.com/example/docz/issues/412)
- [CI runner logs dashboard](https://ci.example.com/dashboards/nightly-docs)
- [Build cache design notes](https://wiki.example.com/build-cache)
<!--docz:references:end-->
