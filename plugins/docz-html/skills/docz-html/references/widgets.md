# Widgets

A widget is a fenced block in the document whose info string names it and
whose body is YAML. Any renderer that does not know the widget shows the
YAML, which is readable on GitHub, in MkDocs, and in Confluence; the page
draws it.

Three widgets exist: `docz-flow`, `docz-stepper`, and `docz-scorecard`.
Each schema below is **closed**: a key it does not list is an error, as is
a missing required key or a value of the wrong type. A block that fails is
not drawn. Write its source as a code block under a `callout caution`
that names the widget, the key, and the problem, and report it.

On the page every widget is the same figure (`components.md`, Widgets):
the YAML converted to JSON as the island, an empty `.widget-mount`, and
the fallback this file gives for each. Key names in the island are the
YAML's, unchanged.

## docz-flow

A horizontal flow of stages. Selecting a stage shows what it does; a gate
stage is outlined.

````markdown
```docz-flow
title: Request path
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
```
````

| Key | Type | Required | Meaning |
| --- | --- | --- | --- |
| `title` | string | no | The figure's caption |
| `stages` | list of stage, 2 to 8 | yes | The stages, left to right |
| `stages[].name` | string | yes | The box's label; keep it to two or three words |
| `stages[].does` | string | yes | What the stage does, shown when it is selected |
| `stages[].note` | string | no | A detail shown beneath `does` |
| `gates` | list of integers | no | 1-based positions of the stages that are gates; each between 1 and the number of stages |
| `label` | string | no | A line above the flow |
| `legend` | string | no | A line below the flow |
| `alert` | string | no | A line below the legend, in the bad colour, for what goes wrong |

Fallback: the stages as an ordered list, a gate marked with a pill.

```html
<div class="widget-fallback">
  <ol>
    <li><b>Ingress</b>: Terminates TLS, routes by host</li>
    <li><b>Auth</b> <span class="pill ok">gate</span>: Verifies the bearer token
      <span class="note">30 s clock skew</span></li>
    <li><b>Handler</b>: Reads through the store</li>
  </ol>
  <p class="note">Every request passes all three.</p>
</div>
```

## docz-stepper

"What happens when": named scenarios over one list of steps. Picking a
scenario walks the steps; the step it fails at is marked and the steps
after it are greyed out as never reached.

````markdown
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
````

| Key | Type | Required | Meaning |
| --- | --- | --- | --- |
| `title` | string | no | The figure's caption |
| `steps` | list of step, 2 to 12 | yes | The steps, in order |
| `steps[].key` | string | yes | A short unique key, `[a-z0-9-]+`; also the label under the step's dot |
| `steps[].name` | string | yes | The step's button label |
| `steps[].title` | string | no | The heading when the step is shown; `name` when absent |
| `steps[].does` | string | yes | What the step does |
| `scenarios` | list of scenario, at least 1 | yes | The scenarios, the first shown first |
| `scenarios[].name` | string | yes | The scenario's button label; unique |
| `scenarios[].fails_at` | string | no | The `key` of the step this scenario stops at |
| `scenarios[].fail_label` | string | no | What the failing step's note is called; `Denied` when absent |
| `scenarios[].notes` | map of step key to string | no | What happens at that step in this scenario; every key is a step's |

Fallback: a table, one row per step and one column per scenario.

```html
<div class="widget-fallback table-wrap">
  <table>
    <thead><tr><th>Step</th><th>What it does</th><th>A valid token</th><th>An expired token</th></tr></thead>
    <tbody>
      <tr><td>Parse</td><td>Reads the bearer token…</td><td></td><td></td></tr>
      <tr><td>Verify</td><td>Checks the signature…</td><td></td>
        <td class="high">Rejected: exp is 40 s in the past…</td></tr>
      <tr><td>Scope</td><td>Resolves the repositories…</td>
        <td>Two repositories match public:read.</td><td>Never reached</td></tr>
    </tbody>
  </table>
</div>
```

## docz-scorecard

Criteria with weights and targets. The reader pastes JSON results and the
page scores each criterion and the whole; the last results are kept in
the browser for that document.

````markdown
```docz-scorecard
title: Release readiness
note: Measured from the release checklist.
checks:
  - id: C1
    name: Signed artifacts
    target: Every archive and image signed
    weight: 2
    fixes: Rollout
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
      score: 0.5
      observed: SBOM for docz-api only
```
````

| Key | Type | Required | Meaning |
| --- | --- | --- | --- |
| `title` | string | no | The figure's caption |
| `note` | string | no | A line under the total |
| `checks` | list of check, at least 1 | yes | The criteria |
| `checks[].id` | string | yes | A unique id the results refer to |
| `checks[].name` | string | yes | The criterion |
| `checks[].target` | string | yes | What full marks looks like |
| `checks[].weight` | number greater than 0 | no | The criterion's weight; 1 when absent |
| `checks[].fixes` | string | no | A tab's label (a link to it) or text naming what fixes it |
| `checks[].source` | string | no | Where the measurement comes from |
| `example` | results | no | Results the reader can load to see the widget work, labelled as an example |

The results a reader pastes, and `example`, have this shape:

| Key | Type | Required | Meaning |
| --- | --- | --- | --- |
| `generated_at` | string | no | When the results were taken |
| `subject` | string | no | What was measured |
| `results` | list of result | yes | One per check measured |
| `results[].id` | string | yes | A check's `id` |
| `results[].score` | number from 0 to 1, or null | yes | null when it could not be measured |
| `results[].observed` | string | no | What was found |
| `results[].evidence` | string | no | Where it was found |

A criterion's points are `score × weight`. The total is the sum of the
points over the sum of the weights of the checks that have a non-null
score, and a check with no result or a null score is shown as not
measured and counts in neither sum.

Fallback: the checks as a table, without scores.

```html
<div class="widget-fallback table-wrap">
  <table>
    <thead><tr><th>#</th><th>Check</th><th>Target</th><th>Weight</th><th>Fixes it</th></tr></thead>
    <tbody>
      <tr><td>C1</td><td>Signed artifacts</td><td>Every archive and image signed</td><td>2</td><td>Rollout</td></tr>
      <tr><td>C2</td><td>SBOM attached</td><td>An SBOM per archive</td><td>1</td><td></td></tr>
    </tbody>
  </table>
</div>
```
