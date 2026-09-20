---
id: {{ .Prefix }}-{{ .Number }}
title: "{{ .Title }}"
status: {{ .Status }}
author: {{ .Author }}
created: {{ .Date }}
---

<!-- markdownlint-disable-file MD025 MD041 -->

# {{ .Prefix }}-{{ .Number }}: {{ .Title }}

<!--toc:start-->
<!--toc:end-->

<!--docz:question:start-->
## Question

<!-- What specific question are we trying to answer? Be precise — a good
     investigation question has a clear yes/no or concrete answer.
     Example: "Can we use X library to achieve Y without Z limitation?" -->

<!--docz:question:end-->

<!--docz:hypothesis:start-->
## Hypothesis

<!-- What do you expect to find, and why? This forces upfront thinking and
     makes the conclusion more meaningful. -->

<!--docz:hypothesis:end-->

<!--docz:context:start-->
## Context

<!-- Why is this investigation needed right now? What design, plan, or
     error triggered it? Link to the parent document(s). -->

**Triggered by:** <!-- RFC-XXXX / DESIGN-XXXX / issue #XXX -->

<!--docz:context:end-->

<!--docz:approach:start-->
## Approach

<!-- How will you test the hypothesis? List the specific steps, experiments,
     or code paths you will exercise. Keep it concrete enough that someone
     else could replicate the investigation. -->

1.
2.
3.

<!--docz:approach:end-->

<!--docz:environment:start-->
## Environment

<!-- Versions, configuration, or setup details relevant to reproducibility.
     Delete this section if not applicable. -->

| Component | Version / Value |
| --------- | --------------- |
|           |                 |

<!--docz:environment:end-->

<!--docz:findings:start-->
## Findings

<!-- What did you actually observe? Include command output, logs, benchmark
     numbers, or code snippets as evidence. Fill this in as you go. -->

### Observation 1

### Observation 2

<!--docz:findings:end-->

<!--docz:conclusion:start-->
## Conclusion

<!-- Answer the original question directly. State clearly what was found:
     confirmed / refuted / inconclusive, and why. -->

**Answer:** <!-- Yes / No / Inconclusive -->

<!--docz:conclusion:end-->

<!--docz:recommendation:start-->
## Recommendation

<!-- What should happen next based on this conclusion? Update the parent
     doc, unblock the design decision, open a follow-up investigation, etc. -->

<!--docz:recommendation:end-->

<!--docz:references:start-->
## References

<!-- Links to parent docs, related investigations, issues, external sources -->

<!--docz:references:end-->
