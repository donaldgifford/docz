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

<!--docz:last-verified:start-->
## Last Verified

<!-- The most recent end-to-end run. Re-verifying replaces the row; git
     keeps the history. -->

| Date | PR | Commit | Verified by |
| ---- | -- | ------ | ----------- |
|      |    |        |             |

**Notes:** <!-- one or two sentences: what was run, and anything that differed -->

<!--docz:last-verified:end-->

<!--docz:overview:start-->
## Overview

<!-- What this runbook is for, in two or three sentences. -->

**Service:** <!-- the service, tool, or system -->

**Owner:** <!-- team or person -->

<!--docz:overview:end-->

<!--docz:when:start-->
## When to Use

- <!-- the trigger: an alert name, a request, a schedule -->

<!--docz:when:end-->

<!--docz:prerequisites:start-->
## Prerequisites

- <!-- access, credentials, tools, and versions -->

<!--docz:prerequisites:end-->

## Procedures

<!-- One procedure per task this runbook performs. Steps are an ordered
     list: number them, nest sub-steps under them, and put the command a
     step runs in a fenced block beneath it. -->

<!--docz:procedure:start-->
### Procedure 1: <!-- Onboard / Rotate / Deploy -->

<!-- One sentence on what this procedure achieves. -->

<!--docz:steps:start-->
#### Steps

1. Step description

   ```sh
   command to run
   ```

   **Expected:** what you should see

2. Step description
   1. Sub-step
   2. Sub-step

<!--docz:steps:end-->

<!--docz:verification:start-->
#### Verification

- <!-- how to tell the procedure worked -->

<!--docz:verification:end-->

<!--docz:rollback:start-->
#### Rollback

1. <!-- how to undo it; "Not applicable" is an answer -->

<!--docz:rollback:end-->
<!--docz:procedure:end-->

## Troubleshooting

<!-- One scenario per symptom, named the way the person paged would
     describe it. -->

<!--docz:scenario:start-->
### Scenario: <!-- the symptom -->

**Alert:** <!-- alert name, if one fires -->

**Likely cause:** <!-- one line -->

<!--docz:steps:start-->
#### Steps

1. Diagnose: <!-- what to check -->
2. Resolve: <!-- what to change -->

<!--docz:steps:end-->
<!--docz:scenario:end-->

<!--docz:escalation:start-->
## Escalation

| Who | When | How |
| --- | ---- | --- |
|     |      |     |

<!--docz:escalation:end-->

<!--docz:references:start-->
## References

<!-- Links to related designs, alerts, dashboards, and other runbooks -->

<!--docz:references:end-->
