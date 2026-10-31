# Components

Every class here is defined in `theme/components.css` and coloured only
through the theme's tokens. Use these and nothing else: no `style`
attributes, no colours, no new classes. When a document needs something
this list cannot express, say so in your report rather than inventing it;
that is a change to `pkg/theme`.

Escape every piece of document text you put into HTML (`&`, `<`, `>`, and
`"` inside attributes). Markdown inside a region becomes the ordinary
elements: `p`, `ul`/`ol`, `table`, `pre><code`, `blockquote`, `a`, `code`,
`strong`, `em`, `h3`–`h6`.

## The page

```html
<!doctype html>
<html lang="en" data-theme="paper" data-doc="DESIGN-0025">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <meta
      http-equiv="Content-Security-Policy"
      content="default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; img-src data: https:"
    />
    <title>DESIGN-0025: docz-html: standalone HTML pages</title>
    <!-- docz:theme -->
    <style>
      /* theme/base.css, theme/tokens/<theme>.css, theme/components.css,
         concatenated in that order */
    </style>
    <!-- /docz:theme -->
  </head>
  <body>
    <div class="wrap">
      <header class="page-header">…</header>
      <nav class="tabs" aria-label="Sections">…</nav>
      <section class="panel" id="p-overview">…</section>
      <!-- one section per tab, in tab order -->
      <footer class="page-footer">…</footer>
    </div>
    <!-- docz:theme -->
    <script>
      /* theme/runtime.js, verbatim */
    </script>
    <!-- /docz:theme -->
  </body>
</html>
```

- `data-theme` is `paper` unless the user asks for `classic`. Inline only
  that theme's token file.
- `data-doc` is the document id. The runtime namespaces the remembered
  tab and task filter by it.
- Never set `data-mode`; the reader's system decides until they press the
  toggle.
- The Content-Security-Policy line is required: nothing on the page may
  load from the network.

## Header

```html
<header class="page-header">
  <p class="eyebrow">Design · docz</p>
  <h1>docz-html: standalone HTML pages</h1>
  <p class="summary">One paragraph: the scope and the ask.</p>
  <div class="meta">
    <span class="pill">DESIGN-0025</span>
    <span class="pill st-proposed">In Review</span>
    <span class="pill">Donald Gifford</span>
    <span class="pill">2026-10-31</span>
    <button type="button" class="mode-toggle">Dark</button>
  </div>
  <p class="inferred">Sections were inferred from headings; this document carries no region markers.</p>
</header>
```

- The eyebrow is the type's label and the repository name.
- The title is the frontmatter title, without the id.
- The summary is the first region of `summary`, `overview`, `objective`,
  `question`, `hypothesis`, or `context`, as one paragraph of plain text
  (inline code and emphasis kept). Leave the summary out rather than
  writing one.
- `.inferred` appears only when `docz inspect` reports `inferred: true`.
- The status pill's class:

  | Status | Class |
  | --- | --- |
  | Draft | `st-draft` |
  | Proposed, In Review | `st-proposed` |
  | Active, In Progress, Open | `st-active` |
  | Accepted, Approved, Implemented, Completed, Concluded | `st-accepted` |
  | Adopted | `st-adopted` |
  | Rejected, Abandoned | `st-rejected` |
  | Superseded | `st-superseded` |
  | Deprecated | `st-deprecated` |

  Anything else gets a plain `pill`.

## Tabs and panels

```html
<nav class="tabs" aria-label="Sections">
  <button type="button" data-tab="overview" aria-controls="p-overview">Overview</button>
  <button type="button" data-tab="design" aria-controls="p-design">Design</button>
</nav>

<section class="panel" id="p-overview">
  <h2 class="panel-title">Overview</h2>
  …
</section>
```

- A tab's name is the lower-case label with spaces as `-`
  (`open-questions`), and its panel's id is `p-` plus that name.
- Every panel starts with an `h2.panel-title` naming the tab. The runtime
  hides it while tabs are on; without JavaScript every panel shows,
  stacked, under its title.
- A heading inside a panel keeps the id docz gives it (the `anchor` in
  `docz inspect`'s headings), so a `#slug` link in the text lands on it;
  the runtime switches to the tab holding the target.
- A panel with more than four headings opens with a heading list:

  ```html
  <ul class="heading-list">
    <li><a href="#the-theme">The theme</a></li>
  </ul>
  ```

- A region left as a template placeholder (only comments, or nothing)
  becomes `<p class="empty">Not written yet.</p>`.

## Cards and grids

```html
<div class="grid g2">
  <div class="card ok">
    <p class="card-label">Goals</p>
    <ul>…</ul>
  </div>
  <div class="card neutral">
    <p class="card-label">Non-goals</p>
    <ul>…</ul>
  </div>
</div>
```

- `g2` is two columns, `g3` three; both stack on a narrow screen.
- Card variants: `ok` (positive, goals, in scope), `bad` (negative, out of
  scope), `neutral`, `accent` (the chosen option). A plain `card` for
  anything else.

## Callouts

```html
<div class="callout note">
  <p class="callout-title">Note</p>
  <p>…</p>
</div>
```

- A GitHub alert (`> [!NOTE]`, `TIP`, `IMPORTANT`, `WARNING`, `CAUTION`)
  becomes the callout of the same name, in lower case, titled with the
  alert's name.
- `ok` holds success criteria; `bad` holds a failure or a rollback;
  a plain `callout` (accent) holds an ADR's decision.

## Expandables

```html
<details class="expand" id="q-3">
  <summary>3. How does the stage-1 skill read the theme?</summary>
  <div class="body">
    <ul>
      <li class="option recommended">a. A copy inside the plugin…</li>
      <li class="option">b. A hidden command…</li>
    </ul>
    <p class="resolution">Resolved 2026-10-31: (a).</p>
  </div>
</details>
```

- An open question is one expandable. Mark the option the document
  recommends (`*(recommendation)*`) with `recommended`.
- A resolved question shows its resolution in `.resolution` and is closed;
  an unresolved one has the `open` attribute.
- A runbook scenario is one expandable too, titled with its name.

## Tables

```html
<div class="table-wrap">
  <table>
    <thead><tr><th>Risk</th><th>Likelihood</th><th>Impact</th><th>Mitigation</th></tr></thead>
    <tbody><tr><td>…</td><td class="high">High</td><td class="low">Low</td><td>…</td></tr></tbody>
  </table>
</div>
```

- Every table goes in a `.table-wrap` so it scrolls sideways on a narrow
  screen.
- In a risks table, a likelihood or impact cell reading High, Medium, or
  Low gets that class.

## IMPL phases

```html
<div class="filters" role="group" aria-label="Show tasks">
  <button type="button" data-filter="all">All</button>
  <button type="button" data-filter="open">Open</button>
  <button type="button" data-filter="done">Done</button>
</div>

<div class="card phase" id="phase-2-the-skill-stage-1-and-the-widgets">
  <div class="phase-head">
    <h3>Phase 2: The skill, stage 1, and the widgets</h3>
    <span class="progress">3 / 12</span>
  </div>
  <div class="bar"><i style="width: 25%"></i></div>
  <ul class="tasks">
    <li data-state="done">
      <input type="checkbox" checked disabled />
      <span class="text">Create <code>plugins/docz-html/</code>…</span>
    </li>
    <li data-state="open">
      <input type="checkbox" disabled />
      <span class="text">Write the stage-1 <code>SKILL.md</code>…</span>
    </li>
    <li data-state="deferred">
      <input type="checkbox" disabled />
      <span class="text">Review the pages in a browser…
        <span class="pill warn">deferred</span>
        <span class="note">human required</span></span>
    </li>
  </ul>
  <div class="callout ok">
    <p class="callout-title">Success criteria</p>
    <ul>…</ul>
  </div>
</div>
```

- One filter bar at the top of the Phases panel. It is hidden without
  JavaScript.
- `data-state` is `done`, `open`, `deferred`, or `skipped`. A deferred or
  skipped task carries a `warn` pill and its note, and counts as neither
  done nor open: the progress is done over done plus open. A task is
  deferred or skipped when its text ends `**deferred** - <note>` or
  `**skipped** - <note>`.
- The bar's width is the one `style` attribute allowed on the page: done
  over total, as a percentage.

## Runbook steps

```html
<ol class="steps">
  <li id="step-2b-3">
    <span class="step-id">2B.3</span> Roll the deployment.
    <pre><code class="language-bash">kubectl rollout restart deploy/docz-api</code></pre>
    <p class="expected"><b>Expected:</b> every pod Ready within two minutes.</p>
    <ol class="steps">…children…</ol>
  </li>
</ol>
```

- The step id is built from the procedure heading's token, not its
  position: `2B.3` is the third step of `### Procedure 2B: …`, `2B.3.1`
  its first child, `2B.R1` the first rollback step, and `S1.2` the second
  step of the first scenario. A procedure heading with no token takes its
  index. The element id is `step-` plus the id in lower case with `.` as
  `-`.
- A rollback is the same list inside a `callout bad` titled Rollback.
- Verification is a task list (`ul.tasks`, all `open`).

## An investigation's answer

```html
<div class="verdict yes">
  <p class="card-label">Answer</p>
  <p>Yes: the anchors survive an API update.</p>
</div>
```

`yes`, `no`, or `partial`, from how the `**Answer:**` line begins. Leave
the class off when it begins with none of them.

## Diagrams

Stage 1 carries no mermaid, so a mermaid fence stays its source:

```html
<pre class="mermaid"><code>flowchart LR
  a --> b</code></pre>
```

## Widgets

A widget block in the markdown (see `widgets.md`) becomes a figure the
runtime draws:

```html
<figure class="widget" data-widget="flow">
  <script type="application/json" data-widget="flow">
    {"title":"Request path","gates":[2],"stages":[…]}
  </script>
  <div class="widget-mount"></div>
  <div class="widget-fallback">
    <!-- the same data, readable without JavaScript -->
  </div>
</figure>
```

- The island is the block's YAML converted to JSON, unchanged. Write `<`
  as `\u003c` inside it so no string can close the script element.
- The runtime finds every `script[type="application/json"][data-widget]`,
  draws into the sibling `.widget-mount`, marks the figure `drawn` (which
  hides the fallback), and redraws when the mode changes.
- The fallback is required. If the island fails to parse, or the reader
  has JavaScript off, the fallback is what they read. Each widget's
  fallback shape is in `widgets.md`.
- A block whose YAML is invalid is not a widget: show its source as a
  code block under a `callout caution` naming the error and the line.

## Footer

```html
<footer class="page-footer">
  From <code>docs/design/0025-docz-html.md</code>,
  sha256 <code>9f2c…</code>. Assembled by the docz-html skill.
</footer>
```

The SHA-256 is `content_sha256` from `docz inspect <id> --format json`.
Write nothing time-dependent, so the same document gives the same page.
