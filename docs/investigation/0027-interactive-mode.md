---
id: INV-0027
title: "Interactive mode"
status: Open
author: Donald Gifford
created: 2026-10-10
---

<!-- markdownlint-disable-file MD025 MD041 -->

# INV-0027: Interactive mode

<!--toc:start-->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
  - [Observation 1: the concept is a page that blocks a hook until a person decides, and the page is the hard part](#observation-1-the-concept-is-a-page-that-blocks-a-hook-until-a-person-decides-and-the-page-is-the-hard-part)
  - [Observation 2: docz create arms the review and the Stop hook opens it](#observation-2-docz-create-arms-the-review-and-the-stop-hook-opens-it)
  - [Observation 3: the page's controls come from docz's readers](#observation-3-the-pages-controls-come-from-doczs-readers)
  - [Observation 4: regions make anchors that survive the agent's edits](#observation-4-regions-make-anchors-that-survive-the-agents-edits)
  - [Observation 5: the CLI's JSON output is the bridge, and two pieces are missing](#observation-5-the-clis-json-output-is-the-bridge-and-two-pieces-are-missing)
  - [Observation 6: the tool reuses docz-site's renderer from inside ui/](#observation-6-the-tool-reuses-docz-sites-renderer-from-inside-ui)
  - [Observation 7: feedback is one message, in an order the agent can act on](#observation-7-feedback-is-one-message-in-an-order-the-agent-can-act-on)
  - [Observation 8: the local server still needs a token](#observation-8-the-local-server-still-needs-a-token)
  - [Observation 9: what to keep ready for a hosted version](#observation-9-what-to-keep-ready-for-a-hosted-version)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
- [Open Questions](#open-questions)
  - [1. Where does docz-review live?](#1-where-does-docz-review-live)
  - [2. What does the docz CLI expose?](#2-what-does-the-docz-cli-expose)
  - [3. How is the JSON contract pinned and typed?](#3-how-is-the-json-contract-pinned-and-typed)
  - [4. What starts a review?](#4-what-starts-a-review)
  - [5. What does a submitted review do to the document?](#5-what-does-a-submitted-review-do-to-the-document)
  - [6. How is an annotation anchored?](#6-how-is-an-annotation-anchored)
  - [7. How long does the Stop hook wait, and what if I walk away?](#7-how-long-does-the-stop-hook-wait-and-what-if-i-walk-away)
  - [8. How is it released and installed?](#8-how-is-it-released-and-installed)
  - [9. How does Claude Code get the hooks?](#9-how-does-claude-code-get-the-hooks)
- [References](#references)
<!--toc:end-->

<!--docz:question:start-->
## Question

What would a `docz-review` add-on take? It is a Bun/TypeScript tool, not
part of the docz CLI. When an agent writes a docz document in a session, a
docz plugin hook starts `docz-review`, which opens the document in a local
page. There I click an option for each open question or write my own,
annotate passages, and submit. Everything returns to the agent as one
message. What must docz expose so a TypeScript tool can see a document's
structure, and how does the tool ship?

<!--docz:question:end-->

<!--docz:hypothesis:start-->
## Hypothesis

The work is mostly front end. Text selection, highlighting, the
annotation popover and sidebar, question cards, and a diff between
rounds are browser code, and
[Plannotator](https://docs.plannotator.ai/) needs React, a highlighter,
CodeMirror, and two diff libraries to do it well. So the tool is
TypeScript on Bun, compiled to one binary per platform with
`bun build --compile`. It is installed only by people who want it, the
way docz-site is deployed only by people who want it, and so is the
Claude Code plugin that wires it into a session: a separate `docz-review`
plugin. The docz plugin keeps working as it does today.

What docz contributes is structure: regions, open questions with lettered
options and a marked recommendation, tasks, statuses, and findings. The
Go readers already compute all of it. The bridge is the CLI's JSON
output, as `api/openapi.yaml` is the bridge between docz-api and
docz-site. Two things are missing:

- a command that reports one document's structure;
- validation of a single document.

Each JSON output gets a committed schema, a Go test that holds the CLI to
it, and generated TypeScript types.

<!--docz:hypothesis:end-->

<!--docz:context:start-->
## Context

**Triggered by:** the day-to-day loop of writing docz documents with an
agent.

Today that loop goes like this:

1. The agent runs `docz create` and fills in the document.
2. I read it in the mdp neovim plugin, which is read-only.
3. In chat I re-locate each thing I want changed ("in Observation 2, the
   sentence about…"). I answer the open questions as prose, such as "1a,
   2b, 3c but cap it at five minutes".
4. The agent edits the document, and I read the whole thing again to find
   what changed.

Each round costs the same re-location, retyping, and re-reading.

Scope:

- **A local tool for a working session, opt-in.** docz-site stays the
  read-only reader it is.
- **The docz CLI stays headless.** It gains JSON output, not a server or a
  page.
- **A hosted version later**, through docz-api and docz-site, for example
  as a "wait for a person" step in
  [INV-0023](0023-a-temporal-control-plane-for-docz-with-docz-api-as-coordinator.md)'s
  Temporal workflow. Observation 9 covers what to keep ready for it.

The tool's shape went through two revisions. The first draft put a review
server in the `docz` binary, rendered in Go with goldmark plus one plain
script. That would work for question cards and plain comments, but it
cannot reach the annotation experience that makes the idea worth having.
And a real front end inside a Go binary means committed bundles and Bun
anyway. So the review is its own thing, in the language its front end
needs.

Related:

- [INV-0004](0004-v1-release-plan-tui-markdown-preview-and-cli-parity.md)
  planned a `docz preview` built on mdp. It never shipped.
- [INV-0021](0021-confluence-comments-as-a-view-layer-kept-across-source-changes.md),
  open question 2, asks how to describe a comment's anchor. Both should
  get the same answer.
- [Issue #116](https://github.com/donaldgifford/docz/issues/116) is about
  generating the decisions table from the open questions' resolutions.

<!--docz:context:end-->

<!--docz:approach:start-->
## Approach

1. Read how Plannotator works mechanically:
   - its local server;
   - its gates and hook outputs;
   - its annotation model;
   - its front-end stack;
   - its Claude Code integration.

   Keep the concept and leave the product.
2. Work out when a hook should open a review. `docz create` is too early,
   because the document is still an empty template.
3. List the structure the page needs and which Go reader computes each
   part.
4. Read the CLI's current `--format json` outputs and find the gaps.
5. Choose how the JSON contract is pinned on the Go side and typed on the
   TypeScript side.
6. Work out where the tool lives, how it reuses docz-site's renderer, and
   how it ships.
7. Optional: run Plannotator over one docz document for comparison.

<!--docz:approach:end-->

<!--docz:environment:start-->
## Environment

| Component | Version / Value |
| --------- | --------------- |
| docz | `v2.0.0-beta.8` (main at `056c611`) |
| CLI JSON today | `list`, `validate` (by type), `status set`, `export confluence` |
| Plannotator | docs.plannotator.ai and `backnotprop/plannotator`, read 2026-10-10 |
| Claude Code hooks | `PostToolUse`, `Stop` (command hooks, JSON on stdin) |
| docz-site renderer | `ui/src/markdown`: unified, remark-gfm, alerts, Shiki, Mermaid, sanitize |

<!--docz:environment:end-->

<!--docz:findings:start-->
## Findings

### Observation 1: the concept is a page that blocks a hook until a person decides, and the page is the hard part

Plannotator's mechanics, reduced to what matters here:

- **A hook or command starts a local HTTP server on a random port** and
  opens the browser. The server exits when the person approves, sends
  feedback, or closes the tab.
- **Annotations:** comment, delete, label, "looks good", and a global
  comment. Each is anchored by the quoted text the person selected.
- **The decision goes back on stdout in the shape the caller needs:**
  - plain text;
  - `{"decision":"annotated","feedback":"…"}` with `--json`;
  - `{"decision":"block","reason":"…"}` with `--hook`, which Claude Code
    reads as "keep going, with this as your next instruction". Approval
    prints nothing, so the agent stops.
- **With `--require-approval`, the exit code carries the decision:** `0`
  approved, `1` annotated or closed, `2` bad input.

The server is small. **The front end is most of the product.**
`packages/ui` depends on:

- React 19 and Tailwind;
- `@plannotator/web-highlighter`, its own fork of a text highlighter;
- CodeMirror (`@codemirror/*`, plus `@codemirror/merge`) for direct edits
  and diffs;
- `@pierre/diffs` and `diff`.

Like docz-site, it is TypeScript on Bun.

Out of scope here:

- code and PR review;
- URL and HTML annotation;
- share links;
- "Ask AI";
- the Inbox and Artifact Server;
- its own history store;
- unauthenticated access, which Observation 8 rejects.

### Observation 2: `docz create` arms the review and the `Stop` hook opens it

A hook on `docz create` would fire before the document has any content:
the agent fills the template with `Write` and `Edit` afterwards. The
right moment is **the end of the agent's turn**, and only for documents
created in this session. Reviewing every document the agent touched would
open a page each time it ticks an IMPL task during implementation.

A separate **`docz-review` plugin** in claude-skills installs two hooks.
The docz plugin is not touched, so nothing changes for anyone who does not
install the new one:

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Bash",
        "hooks": [{ "type": "command", "command": "docz-review arm --from-hook" }]
      }
    ],
    "Stop": [
      {
        "hooks": [{ "type": "command", "command": "docz-review --pending --hook", "timeout": 86400 }]
      }
    ]
  }
}
```

- **The arming hook** reads the hook's JSON from stdin. When the command
  was `docz create`, it records the created path, taken from the
  `Created <TYPE>: <path>` line in the tool output, in a pending list
  under `$XDG_STATE_HOME/docz/review/<session id>`. Any other command
  exits 0 silently. A `docz create --format json` that reports the path
  would make this robust against wording changes.
- **The `Stop` hook** runs `docz-review --pending --hook`:
  - with nothing pending, it prints nothing and the agent stops as usual;
  - otherwise it opens one page for every pending document, with a
    switcher when there are several, and waits.
- **On submit with changes requested,** it prints
  `{"decision":"block","reason":"<feedback>"}`. The agent revises the
  document and finishes its turn. The `Stop` hook fires again and opens
  round two, with a diff since round one. **The rounds repeat by
  themselves until I approve**, and approving or dismissing clears the
  document and lets the agent stop.
- **`/docz-review:review <id>`** runs `docz-review <id> --hook` on any
  existing document.
- **If the binary is missing**, the plugin's hooks print one line saying
  how to install it and exit 0. A plugin installed without its binary
  never blocks a session.

Two details need checking against Claude Code's current hook contract
before a design commits to them: the exact `PostToolUse` input fields for
a `Bash` call, and how a long-running `Stop` hook interacts with the
user's ability to interrupt.

### Observation 3: the page's controls come from docz's readers

| On the page | Computed by |
| --- | --- |
| **Open-question cards:** one button per option with the recommended option preselected, an "Other" text box, and an optional note. A resolved question shows its resolution, read-only. | `kinds.OpenQuestions`: `Question{Number, Title, Options[]{Letter, Text, Recommended}, Resolved, Line}` |
| **Annotations** on any selection: comment, suggest a replacement, mark for deletion, "looks good"; plus document-level comments | `docparse.Regions` for the anchor (Observation 4) |
| **Findings shown before reading**, each with a dismiss action or "fix this" | `repo.Validate` plus the per-type validator, as `docz validate` composes them |
| **A status suggestion** | `TypeConfig.Statuses` from the resolved config |
| **IMPL tasks** with checkboxes | `docparse.TaskItems` |
| **A diff since the previous round** | `docz-review` itself, from a snapshot taken when each round opens |

**Question cards appear only where a document has questions to answer.**
`kinds.OpenQuestions` reads questions inside an `open-questions` region,
which the DESIGN template carries and any other document may add (this
one does). Not every type has open questions, and a document without the
region is reviewed with annotations alone, which cover the same ground: a
comment on a passage is how a question gets answered there. The cards are
a shortcut for documents built around decisions, not a requirement on
every type.

All of this is in Go today. None of it should be re-implemented in
TypeScript: a second parser for regions and open questions would drift
from the one `docz validate` uses. That is the reason for Observation 5.

### Observation 4: regions make anchors that survive the agent's edits

Plannotator anchors to the selected text. When the agent rewrites the
passage, or the text appears twice, the anchor is lost or ambiguous.
`docz-review` can anchor to the region the selection sits in and narrow
it with a quote:

```text
Anchor{region: "findings", ordinal: 0, heading: "Observation 2",
       exact: "Anchors are quotes.", prefix: "…", suffix: "…", line: 212}
```

The anchor is resolved again when the feedback is written, so the line
the agent sees is where the text is *now*. When the quote no longer
matches, the region and heading still tell the agent where to look.
INV-0021 question 2 recommends the same quote-with-context shape for
Confluence comments.

This needs the rendered page to know each block's source line. docz-site's
preprocessing loses that: it cuts the frontmatter and the ToC block, so
every later line shifts, and `rehype-sanitize` drops the `docz:` markers.
The review renderer must blank those lines instead of cutting them (as
`pkg/export/confluence` does), stamp a `data-line` attribute on each block
from the mdast `position`, and wrap each region's span using the lines
`docz inspect` reports.

### Observation 5: the CLI's JSON output is the bridge, and two pieces are missing

The CLI already has JSON output for `docz list`, `docz validate` (by
type), `docz status set`, and `docz export confluence`. Two pieces are
missing:

- **One document's structure.** A new
  `docz inspect <id|path> --format json` would print everything in
  Observation 3's table except the diff:
  - path, type, schema, frontmatter, and the type's allowed statuses;
  - `regions[]{kind, start, end, depth, closed}` and whether they were
    inferred;
  - `open_questions[]{number, title, line, options[]{letter, text,
    recommended}, resolved}`;
  - `tasks[]{line, text, checked, indent}`;
  - `headings[]{level, text, anchor, line}`.

  Each is a direct call to a `docparse` or `kinds` reader. Agents and the
  docz skills could use it too, for example to list a document's
  unresolved questions without parsing markdown.
- **Validation of one document.** `docz validate` takes type names, not a
  document. It needs to accept an ID or path as well
  (`docz validate INV-0027 --format json`), or `inspect` embeds the
  document's findings.

To keep the two languages in step:

- **Schemas.** Each JSON output gets a committed JSON Schema, for example
  `api/cli/inspect.schema.json`, with a `schema_version` field.
- **Contract test.** A Go test runs the command over fixtures and
  validates its output against the schema, as
  `internal/httpapi/openapi_contract_test.go` does for the API.
- **TypeScript types** for `docz-review` are generated from the schemas,
  with a check recipe like `gen-api-check`.
- **Version check.** `docz-review` checks `schema_version` and names the
  minimum docz version when the installed CLI is too old.

### Observation 6: the tool reuses docz-site's renderer from inside `ui/`

`ui/` already has Bun, React, Vite, the markdown pipeline (unified,
remark-gfm, GitHub alerts, Shiki, Mermaid, cross-references, sanitize
with an XSS test), linting, tests, and Playwright. `docz-review` needs all
of that plus a highlighter. Building it as a second entry point in `ui/`:

- shares `src/markdown` directly, with a position-preserving mode;
- shares one `package.json` and lockfile, and the same lint, test, and
  licence tooling.

Its server is a small Bun HTTP server, like `ui/server/serve.ts`, that:

- runs `docz inspect`;
- serves the page;
- holds the session state;
- writes the hook output.

`bun build --compile` bundles the server, the page's assets, and the Bun
runtime into one executable per platform target.

It also lets the two front ends share code beyond the markdown pipeline:
the document header, status badges, the frontmatter table, and the
cross-reference links. A component written for one serves the other.

The cost: `ui/` stops being only docz-site. Its CLAUDE.md, CI job, and
path filter cover two products, and a change to `src/markdown` must keep
both working.

### Observation 7: feedback is one message, in an order the agent can act on

```text
docz review: INV-0027 Interactive mode, round 2: changes requested

Open questions:
- Q1 Where does docz-review live? → a
- Q3 How are the TypeScript types produced? → other:
  "put the shapes in api/openapi.yaml"
- Q5 (no answer; leave open)

Annotations:
1. [findings › Observation 2, line 212] "Anchors are quotes."
   comment: Say what INV-0021 decided once it closes.
2. [recommendation › Q4, line 401] delete: option (c)
3. [context, line 96] replace "day-to-day" → "everyday"
4. [document] Shorten the Context section.

Status: set to In Progress
Validation: 1 warning dismissed; fix references.no-link at line 589
```

Each annotation carries its region and its current line, so the agent
never searches. Open-question answers arrive as decisions, not prose,
and "other" keeps my words verbatim. `--json` prints the same content as
a structure.

The review itself **writes nothing to the document**. The agent is
already going to edit it in response, so it writes the resolutions in
the fleet's format at the same time. A second writer racing the agent
would be worse.

### Observation 8: the local server still needs a token

A page from any origin can send a request to `127.0.0.1:<port>`. The port
is not a secret: it can be scanned, and DNS rebinding gets around origin
checks. A submission becomes the agent's next instruction, so forging
one is a prompt injection. The defence is cheap:

- generate a random per-session token, put it in the URL the tool opens,
  turn it into a cookie on first load, and require it on every request;
- reject a `Host` header that is not `127.0.0.1:<port>` or
  `localhost:<port>`;
- send no CORS headers.

### Observation 9: what to keep ready for a hosted version

A docz-api or Temporal version (INV-0023) would show the same review in
docz-site and turn a submitted review into a workflow signal. Building
the tool in `ui/` keeps that within reach:

- the review components are React, in the same project as docz-site;
- the structure `docz inspect` prints is what docz-api would serve from an
  endpoint, so the schema can move into `api/openapi.yaml` when that day
  comes;
- the submitted review's JSON is the signal payload.

What would block it is unchanged: docz-api ingests only the default
branch, and a document under review lives on a feature branch.

<!--docz:findings:end-->

<!--docz:conclusion:start-->
## Conclusion

**Answer:** Yes, as an add-on, in `ui/`, with its own Claude Code plugin. `docz-review` is a Bun/TypeScript binary;
the docz CLI gains JSON output and nothing else.

| Where | Change |
| --- | --- |
| `cmd/` (docz CLI) | `docz inspect <id\|path> --format json`. `docz validate` accepts an ID or path. Optionally `docz create --format json`, for a robust arming hook. |
| JSON contract | A committed JSON Schema per JSON output, with `schema_version`, and a Go contract test over fixtures. |
| `ui/` | A second entry point: the review page (React, a highlighter, question cards, a diff) and a Bun server, both reusing `src/markdown` with a position-preserving mode. TypeScript types are generated from the schemas. |
| Release | `bun build --compile` for each platform, from the same tag as `docz` and `docz-api`, attached to the same GitHub release. |
| `docz-review` plugin (claude-skills, new) | A `PostToolUse` hook to arm, a `Stop` hook to review, and a `/docz-review:review` command. It is separate from the docz plugin, which does not change. |
| docz-api, docz-site, `api/openapi.yaml` | No change now. |

<!--docz:conclusion:end-->

<!--docz:recommendation:start-->
## Recommendation

All nine questions were decided on 2026-10-10. Next, write a DESIGN for
`docz-review`, the `docz inspect` and single-document `docz validate`
additions, and their JSON contract. Two things to check first, both in
Observation 2: Claude Code's `PostToolUse` input for a `Bash` call, and
what interrupting a session does to a `Stop` hook that is still
waiting.

<!--docz:open-questions:start-->
## Open Questions

### 1. Where does `docz-review` live?

- **a. In this repository, as a second entry point in `ui/`, sharing
  its markdown pipeline, lockfile, and tooling.** *(recommendation)*
- b. A sibling `review/` project in this repository, with the markdown
  pipeline moved into a shared Bun workspace package. It is cleaner, but
  docz-site has to move first.
- c. Its own repository, depending on docz only through the CLI's JSON.
- d. Other.

> **Resolved 2026-10-10: (a).** In `ui/`, which also lets the review page and
> docz-site share components.

### 2. What does the docz CLI expose?

- **a. `docz inspect <id|path> --format json` for structure, and
  `docz validate` accepting an ID or path for findings.** Both are general
  commands that agents and scripts can use too. *(recommendation)*
- b. One purpose-built `docz review-data` command printing everything the
  page needs.
- c. Nothing new: `docz-review` parses regions and open questions itself
  in TypeScript.
- d. Other.

> **Resolved 2026-10-10: (a).** `docz inspect`, and `docz validate` on one
> document.

### 3. How is the JSON contract pinned and typed?

- **a. A committed JSON Schema per output, a Go contract test validating
  the CLI's real output, and TypeScript types generated from the schemas
  with a check recipe.** *(recommendation)*
- b. Define the shapes as components in `api/openapi.yaml` now, and
  generate the types the way docz-site does, ready for an API endpoint
  later.
- c. Hand-written TypeScript types, with a Go golden test only.
- d. Other.

> **Resolved 2026-10-10: (a).** Committed JSON Schemas, a Go contract test, and generated
> TypeScript types.

### 4. What starts a review?

- **a. `docz create` arms it and the `Stop` hook opens it, and
  `/docz:review <id>` covers documents created elsewhere.** Only documents
  created in the session are reviewed, and the rounds repeat until
  approval. *(recommendation)*
- b. The `Stop` hook reviews every document changed under `docs_dir` in
  the turn. Simpler, but it fires on every IMPL checkbox tick.
- c. Only the explicit `/docz:review` command.
- d. Other.

> **Resolved 2026-10-10: (a).** `docz create` arms, the `Stop` hook opens, and
> `/docz-review:review` covers the rest.

### 5. What does a submitted review do to the document?

- **a. Nothing. Every answer, annotation, and status choice goes to the
  agent as feedback, and the agent edits.** *(recommendation)*
- b. `docz-review` writes open-question resolutions and the status by
  calling the docz CLI (new `docz` write commands), and sends the agent
  only the rest.
- c. (b) plus free-form edits in the page, saved with a conflict check.
- d. Other.

> **Resolved 2026-10-10: (a).** The review writes nothing; the agent edits.

### 6. How is an annotation anchored?

- **a. Region kind and ordinal, the nearest heading, and a quote with
  about 32 characters of context each side, with the line as a hint.** The
  same shape serves INV-0021. *(recommendation)*
- b. A quote only.
- c. A line and column only.
- d. Other.

> **Resolved 2026-10-10: (a).** Region, heading, and a quote with context, with the line as a
> hint.

### 7. How long does the `Stop` hook wait, and what if I walk away?

- **a. Block, with a long hook timeout (24 hours). Closing the tab counts
  as "dismiss", which lets the agent stop and keeps the document pending
  for `/docz:review`.** *(recommendation)*
- b. Don't block: end the turn and deliver the review later as a message,
  as Plannotator's newest Claude Code mode does.
- c. Block with a short timeout, and treat the timeout as "dismiss".
- d. Other.

> **Resolved 2026-10-10: (a).** As a trial. The hook timeout is a ceiling, not the
> expected wait: a review normally ends in minutes, and closing the tab,
> interrupting the session, or reaching the timeout all count as
> "dismiss" and leave the document pending. Plannotator's ceiling is four
> days (345,600 seconds). Revisit once it has been used; if 24 hours turns
> out to be too long, `docz-review` can take its own shorter `--timeout`
> so it can be tuned without editing the plugin.

### 8. How is it released and installed?

- **a. From the same `v*` tag as `docz`: compiled binaries for
  linux and darwin on x64 and arm64, attached to the same GitHub release,
  installed with mise's `github:` backend using an asset pattern.** Check
  whether goreleaser's Bun builder can drive this, or whether it needs a
  separate job. *(recommendation)*
- b. Publish to npm and run it with `bunx docz-review`.
- c. Its own tags (`review-v*`) and release cadence.
- d. Other.

> **Resolved 2026-10-10: (a).** The same tag and GitHub release as `docz`; check whether
> goreleaser's Bun builder can drive it.

### 9. How does Claude Code get the hooks?

- **a. A separate `docz-review` plugin in claude-skills, holding the two
  hooks and the review command.** The docz plugin stays as it is.
  *(recommendation)*
- b. Add the hooks to the docz plugin, as no-ops when the binary is
  missing.
- c. No plugin: document the hook configuration for people to add
  themselves.
- d. Other.

> **Resolved 2026-10-10: (a).** A separate plugin, so everything works as today
> and the review is opt-in.

<!--docz:open-questions:end-->

<!--docz:recommendation:end-->

<!--docz:references:start-->
## References

- [Plannotator: OSS overview](https://docs.plannotator.ai/open-source/)
- [Plannotator: approval gates and hooks](https://docs.plannotator.ai/open-source/reference/hooks)
- [Plannotator: annotations and feedback](https://docs.plannotator.ai/open-source/workflows/annotations-and-feedback)
- [Plannotator: local server and API](https://docs.plannotator.ai/open-source/reference/local-api)
- [Plannotator: Claude Code integration](https://docs.plannotator.ai/open-source/agents/claude-code)
- [backnotprop/plannotator](https://github.com/backnotprop/plannotator)
- [Claude Code hooks](https://docs.claude.com/en/docs/claude-code/hooks)
- [Bun single-file executables](https://bun.sh/docs/bundler/executables)
- [INV-0004](0004-v1-release-plan-tui-markdown-preview-and-cli-parity.md):
  the unshipped `docz preview`
- [INV-0021](0021-confluence-comments-as-a-view-layer-kept-across-source-changes.md):
  comment anchoring
- [INV-0023](0023-a-temporal-control-plane-for-docz-with-docz-api-as-coordinator.md):
  the Temporal control plane
- [DESIGN-0015](../design/0015-structured-regions-and-docz-validate.md):
  region markers and validation
- [Issue #116](https://github.com/donaldgifford/docz/issues/116): the
  open-questions decisions table

<!--docz:references:end-->
