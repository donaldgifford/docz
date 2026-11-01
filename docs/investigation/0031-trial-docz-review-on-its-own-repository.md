---
id: INV-0031
title: "Trial: docz-review on its own repository"
status: Open
author: Donald Gifford
created: 2026-10-31
---

<!-- markdownlint-disable-file MD025 MD041 -->

# INV-0031: Trial: docz-review on its own repository

<!--toc:start-->
<!-- markdownlint-capture -->
<!-- markdownlint-disable MD013 -->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
  - [Observation 1](#observation-1)
  - [Observation 2](#observation-2)
  - [Observation 3](#observation-3)
  - [Observation 4](#observation-4)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
- [References](#references)
<!-- markdownlint-restore -->
<!--toc:end-->

<!--docz:question:start-->
## Question

Does docz-review, installed from this repository's own marketplace, hold
the end of a real Claude Code turn in this repository until a person has
reviewed the documents the agent created, and does the review's feedback
reach the agent, without getting in the way of ordinary work?

<!--docz:question:end-->

<!--docz:hypothesis:start-->
## Hypothesis

Yes. The hook drill (#203) already exercised `PostToolUse` arming and the
`Stop` hook against captured payloads, so the remaining risk is friction in
day-to-day use (turns held when nothing needs review, a page that is slow
to open, feedback that does not come back), not the hook mechanics.

<!--docz:hypothesis:end-->

<!--docz:context:start-->
## Context

v2.2.1 shipped docz-review, and the repository trial is the first open
follow-up left by DESIGN-0024 and IMPL-0027. The drill proved the
mechanics; this trial checks the experience on the repository that
builds it.

**Triggered by:** DESIGN-0024, IMPL-0027, #203

<!--docz:context:end-->

<!--docz:approach:start-->
## Approach

1. Install the plugin with `/plugin install docz-review@docz` on the
   `trial/docz-review` branch.
2. Create documents with `docz create` in ordinary sessions (this one
   included) and let the `Stop` hook open the review page.
3. For each review, record the time to open, whether comments and the
   verdict reached the agent, and any turn held that should not have been.

<!--docz:approach:end-->

<!--docz:environment:start-->
## Environment

| Component   | Version / Value                     |
| ----------- | ----------------------------------- |
| docz        | 2.2.1 (bf5fece)                     |
| docz-review | v2.2.1, contract `schema_version` 1 |
| Platform    | darwin (macOS 27.2)                 |
| Branch      | `trial/docz-review`                 |

<!--docz:environment:end-->

<!--docz:findings:start-->
## Findings

### Observation 1

An earlier attempt at this document opened the review page at the end of
the turn that created it, as designed. The reviewer left a comment, but
both recorded rounds were `approved` with empty `annotations` and
`questions`, and no feedback reached the agent. The document was
unchanged between rounds, so the comment was not lost to a stale
snapshot. That attempt was deleted and the trial restarted with this one.

### Observation 2

Round 3 of this document's review was submitted with Request changes and
one comment on this observation ("Can you see this comment"). The `Stop`
hook delivered it to the agent as a blocking message carrying the
annotation's region, heading, line, quoted text, and comment, so a comment
submitted with Request changes does reach the agent. Round 4 was a
clean approval with no comment; it closed the review and sent nothing.

### Observation 3

Round 5 was meant to test a comment submitted with Approve. One comment on
the Hypothesis reached the agent, but the round was recorded with outcome
`changes`, not `approved`, so it does not settle whether Approve keeps a
comment. The anchor resolved `exact`, with the selection cut mid-word at
both ends and the remainder kept in `prefix`/`suffix`.

### Observation 4

Round 7 set out to submit a comment with Approve and found that the page
does not allow it: while the draft holds a comment, Approve is disabled
with the tooltip "The draft holds changes; submit them or discard the
draft" (`ui/src/review/document-review.tsx`). A comment goes out only
through Submit, recorded as `changes` (rounds 3, 5, and 7), and the one
way to approve with a comment in the draft is "Discard draft and
approve", which drops it after a confirmation. That is the likely path
for the comment lost in the first attempt, whose two rounds were
`approved` with no annotations, but it was not observed directly.

<!--docz:findings:end-->

<!--docz:conclusion:start-->
## Conclusion

**Answer:** Yes, so far. The turn is held as designed, every comment sent
with Submit reached the agent with its anchor, and Approve cannot carry a
comment by design. The first attempt's lost comment most likely went
through "Discard draft and approve".

<!--docz:conclusion:end-->

<!--docz:recommendation:start-->
## Recommendation

Consider whether "Discard draft and approve" should say how many comments
it drops, since it is the one action that loses a reviewer's words. Then
run the trial for a week of ordinary sessions, conclude here,
and tick the repository-trial follow-up in IMPL-0027.

<!--docz:recommendation:end-->

<!--docz:references:start-->
## References

- [DESIGN-0024: docz-review](../design/0024-docz-review-a-blocking-review-page-for-agent-written-documents.md)
- [IMPL-0027: docz-review implementation](../impl/0027-docz-review-the-cli-contract-the-review-page-the-binary-and-the.md)
- [#203: the recorded hook drill](https://github.com/donaldgifford/docz/pull/203)

<!--docz:references:end-->
