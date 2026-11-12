---
id: INV-0033
title: "docz-review inside a Rex floating pane instead of a browser"
status: Open
author: Donald Gifford
created: 2026-11-09
---

<!-- markdownlint-disable-file MD025 MD041 -->

# INV-0033: docz-review inside a Rex floating pane instead of a browser

<!--toc:start-->
<!-- markdownlint-capture -->
<!-- markdownlint-disable MD013 -->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
  - [Observation 1: the preview has no web view, and drawing the page into a terminal is closed](#observation-1-the-preview-has-no-web-view-and-drawing-the-page-into-a-terminal-is-closed)
  - [Observation 2: the browser pane exists, and it forwards a remote port to the client's browser](#observation-2-the-browser-pane-exists-and-it-forwards-a-remote-port-to-the-clients-browser)
  - [Observation 3: Ladybird draws a page into the terminal, and its author says it works in Rex](#observation-3-ladybird-draws-a-page-into-the-terminal-and-its-author-says-it-works-in-rex)
  - [Observation 4: floating layers are an API, with a command, a window, and a state](#observation-4-floating-layers-are-an-api-with-a-command-a-window-and-a-state)
  - [Observation 5: the hook can own the wait as it does now](#observation-5-the-hook-can-own-the-wait-as-it-does-now)
  - [Observation 6: the Program Status Protocol is the status a browser never had](#observation-6-the-program-status-protocol-is-the-status-a-browser-never-had)
  - [Observation 7: the review's logic is already DOM-free; the page is not](#observation-7-the-reviews-logic-is-already-dom-free-the-page-is-not)
  - [Observation 8: a Rex integration is a CLI call and an init.lua snippet, not a plugin](#observation-8-a-rex-integration-is-a-cli-call-and-an-initlua-snippet-not-a-plugin)
  - [Observation 9: what is not yet measured](#observation-9-what-is-not-yet-measured)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
- [References](#references)
<!-- markdownlint-restore -->
<!--toc:end-->

<!--docz:question:start-->
## Question

When the agent runs inside Rex, can docz-review hold the review in a
floating pane of the session the agent is in, instead of opening a browser,
and what does Rex offer today to build that on?

Three parts, each with a concrete answer:

1. Can Rex show the review page itself, in a pane? (A web view, or the
   page drawn into a terminal.)
2. If not, can a terminal client of the review server stand in for the
   page, launched by the `Stop` hook into a floating layer of the agent's
   window, and end the review the way the page does?
3. What does the floating pane give that the browser cannot, and what
   does it cost?
<!--docz:question:end-->

<!--docz:hypothesis:start-->
## Hypothesis

**Part 1: not in the preview, and yes soon.** Rex's released blocks are
terminals, and drawing the page into a terminal through a Kitty-graphics
browser is a performance problem Rex has already deferred. But a native
browser pane is built and demonstrated (Observation 2), opened from a
terminal's detected dev-server URL, with the port forwarded from a remote
host to the client's own browser process. When it ships with a flavor the
API can create, the review page goes into it unchanged. Before it ships,
Ladybird's new terminal mode (Observation 3) may draw the page into a
floating terminal well enough, which terminal-browser did not.

**Part 2: yes, as the fallback.** `session.new_layer` creates a floating
terminal running a command, in the window the agent's `REX_SESSION` and
`REX_BLOCK` name. The review server, its token, the submission flow, and
the review record (DESIGN-0024 §6–§8) stay exactly as they are; a terminal
client is a second client of `GET /api/review` and
`POST /api/review/<doc>/submit`, and `ui/src/review`'s DOM-free modules
(`anchor.ts`, `feedback.ts`, `draft.ts`, `api.ts`) carry over. The hook
keeps owning the wait: a submit ends it as now, and the pane closing is
the dismiss. Given part 1, this is the path only if the browser pane does
not arrive or cannot be opened from the API.

**Part 3.** Two things a browser cannot do: report the review as
*blocked on the person* through the Program Status Protocol, which Rex
shows in the session picker and tab header; and work on a remote Rex
server, where today's opener runs `xdg-open` on a headless host. The cost
is a terminal review surface, which is a port of the page's decisions
(INV-0032: labels, question cards, a block-level annotation) rather than
its React.
<!--docz:hypothesis:end-->

<!--docz:context:start-->
## Context

**Triggered by:** DESIGN-0024 §2 step 8, where the `Stop` hook opens
`http://127.0.0.1:PORT/?t=TOKEN` in a browser, and running Claude Code
inside Rex, where that step leaves the terminal the agent and the person
share.

docz-review (DESIGN-0024, IMPL-0027) holds the end of a Claude Code turn
until a person has reviewed the documents the agent made. The review is a
page served by a Bun binary on loopback and opened with
`DOCZ_REVIEW_BROWSER`, `BROWSER`, or the platform opener
(`browserOpener`, `ui/review/main.ts`). Every other part of the flow is
already terminal-side: the arming hook, the `Stop` hook, `docz inspect`
and `docz validate`, the record under the state directory, and the
feedback the hook returns as the agent's next instruction.

Rex (superlogical.com) is a terminal multiplexer with a server API, a
Lua configuration, a CLI, and a built-in agent skill that tells Claude
Code which session and block it runs in. Its floating panes shipped in
October and November 2026 and are scriptable. Its community has asked for
"a special version of plannotator" and a browser block; the browser block
was deferred in October, and a demo of it was posted on 2026-11-06.
Rex's own docs, changelog, and feedback are reachable from an agent as the
community MCP, which is how this investigation read them; the demo is in a
channel the MCP does not index, so it was watched frame by frame.

INV-0032 (Open) decided docz-review's page is a port of Plannotator's
layout, not a fork, and set five gates on that port. A terminal client
would share those decisions, so the two investigations should not pull
the page in different directions.
<!--docz:context:end-->

<!--docz:approach:start-->
## Approach

1. Read Rex's block types, the floating-layer methods, the Program Status
   Protocol, and the community's requests, through the community MCP and
   the installed `rex` CLI, and watch the two demos. (Done;
   Observations 1–8.)
2. From a `Stop` hook started by Claude Code inside Rex, confirm the hook
   inherits `REX_SESSION` and `REX_BLOCK`, create a floating layer in that
   window with `rex api call session.new_layer` running a command, and
   end a wait on it with `rex wait` and the `child_exited` event. Record
   what the person sees when the agent's block is not in the active
   window.
3. From the floating pane, write an OSC 7501 `blocked` record with
   `kind=input` and a message, and check the session picker and tab
   header show it; write `done` on exit and check it survives the pane.
4. Open the existing review page in a floating pane through a
   Kitty-graphics browser and try one round: a selection, a label,
   Submit. Ladybird `--headless=terminal` first, built from source at or
   after its PR 14186; terminal-browser once, for the record. Measure
   whether it is usable at all (CPU while scrolling, torn frames, whether
   a DOM selection takes) before measuring anything else. This needs no
   change to docz-review: `DOCZ_REVIEW_BROWSER` names a script that calls
   `session.new_layer` with the browser command and the URL.
5. Spike a terminal client of the review server: fetch `GET /api/review`
   with the token, show the document by region with the findings and the
   question cards, take a block-level annotation and a question answer,
   compute the anchor with `anchor.ts`, and `POST` the submission. Try
   it over three candidates, Bun-native first since docz-review is Bun:
   OpenTUI, Ink, and a Go bubbletea client in `cmd/`. Count the modules of
   `ui/src/review` each reuses untouched.
6. Repeat step 2 against a Rex Linux server with Rex for Mac attached:
   the hook runs on the server, the pane must be drawn on the Mac, and
   today's `xdg-open` must be shown to open nothing there.
7. Decide the opener's rule: a `--open rex|browser` flag and a
   `DOCZ_REVIEW_OPEN` variable, with `auto` meaning Rex when
   `REX_SESSION` is set, and an `init.lua` snippet for minimizing and
   restoring the review layer.
8. When a Rex build with the browser pane is installable: check
   `rex api call block_creator.list` for its flavor and options (a `url`
   is the one that matters), create one in a floating layer with
   `session.new_layer` pointing at a running review server's token URL,
   do one round in it, and repeat against the remote server of step 6.
   If the flavor exists but takes no `url`, or only the terminal's
   detection badge can open it, write that up as the ask for the Rex
   team. This step decides between parts 1 and 2; run it before step 5.
<!--docz:approach:end-->

<!--docz:environment:start-->
## Environment

| Component | Version / Value |
| --------- | --------------- |
| Rex CLI and local server | `rex 0.0.1793645657-gd018d2`, API revision 10 (built 2026-11-02) |
| Rex for Mac | v0.1.0-preview.26 (1829), changelog 2026-11-05, the current preview |
| Rex docs, changelog, feedback | the community MCP, `superlogical.com/rex/community-mcp`, read 2026-11-09 |
| Browser-pane demo | `multi-machine-multi-server.mp4`, posted to the Rex Discord 2026-11-06 19:57 UTC, 43 s at 3226×2160, read as frames at 4 s and 0.5 s intervals |
| Ladybird in the terminal | PR 14186, merged 2026-11-09 13:57 UTC; the author's 17 s demo on X the same day, read as frames at 2 s intervals |
| docz | `main` at `c8c929c` (v2.6.1) |
| docz-review | `ui/review/` and `ui/src/review/` at the same commit |
| Claude Code | inside Rex, with Rex's agent hook installed (`rex skill setup`) |
<!--docz:environment:end-->

<!--docz:findings:start-->
## Findings

### Observation 1: the preview has no web view, and drawing the page into a terminal is closed

`rex api call block_creator.list` on the preview server returns two
creators. The terminal, `com.superlogical.terminal`, has one flavor,
`shell`, whose options are `shell`, `command`, `login`, `cwd`,
`path_prepend`, and `size`. The other is the onboarding walkthrough,
which the client draws. Nothing takes a URL or HTML.

A Rex Discord request for "Browser: a web view in a tab or split" and a
file browser (2026-10-23) was deferred the same day: "Planned but not
short term", where the backlog covers about two weeks. A second request
(2026-10-25), that a browser block on a remote server use the client's
browser process, was deferred "until we get to browser blocks. Good
context to have when we do." Two more (2026-10-31 and 2026-11-03, active)
ask for "some special version of plannotator" and "canvas, browser panes
or ability to preview HTML", with a cmux user asking for its integrated
browser on 2026-11-09. The want is shared.

Drawing the page into a terminal with terminal-browser is closed. It
streams a page as Kitty graphics frames, and the Rex feedback on it is
deferred: scrolling a page "is slow and uses about 3 CPU cores", 230% of
it Rex's own, frames of 10 MB or more are dropped silently, and the
canvas request says it "sometimes crashes Rex". Whether that is the
protocol in Rex or that program is what Observation 3 reopens.

### Observation 2: the browser pane exists, and it forwards a remote port to the client's browser

A 43-second demo posted to the Rex Discord on 2026-11-06, in a channel the
community MCP does not index, shows a Rex for Mac build with a native
browser pane. Read frame by frame:

- A terminal runs `npm run dev`. When Next.js reports
  `http://localhost:3000`, a badge appears in the terminal's top-right
  corner: a signal icon, **Next.js**, **localhost:3000**, and **Open
  Beside ↗**. Rex detected the listening server from the terminal, not
  from a link in its text; the badge names the framework.
- Clicking Open Beside splits the window and opens a **browser pane** on
  the right: back, forward, reload, a URL bar reading `localhost:3000`,
  a pin button, and a pop-out button. The Next.js page renders as a
  browser renders it ("This page is hosted on Mac!"). The badge in the
  terminal collapses to `Next.js :3000`, and the tab title becomes
  `localhost:3000`.
- A second terminal is on a remote host, `pserbs-oracle`, through the
  Rex server there. Its `npm run dev` gets a badge reading
  **localhost:59309 — Open Beside**, and the browser pane it opens shows
  `localhost:3000` with a host chip, **pserbs-oracle**, on the right of
  its URL bar, rendering "This page is hosted on Oracle!". So Rex
  **forwarded the remote port** to a local one and the pane is the
  **client's** browser process, the answer to the deferred request of
  2026-10-25.
- Dragging the pane out makes a standalone browser window with a tab
  strip and a URL bar, the page intact.

Nothing in the demo shows a floating layer holding a browser pane, a CLI
or API call creating one, or how the server names it; those are step 8.
What it settles is that part 1's "no" is temporary: the pane renders real
pages, it is tied to the terminal that started the server, and it works
across hosts in the one way docz-review needs.

### Observation 3: Ladybird draws a page into the terminal, and its author says it works in Rex

Ladybird merged `--headless=terminal` on 2026-11-09 (PR 14186, 1,418
lines, `Libraries/LibWebView/TerminalWebView.*` and a terminal input
parser). `Ladybird --headless=terminal <url>` draws the frames the
Compositor presents with the Kitty graphics protocol, turns the
terminal's pixel mouse reports, Kitty keyboard reports, pastes, and focus
changes into page input, and keeps a status line with a URL prompt
(Ctrl-L), reload, back and forward (Alt-arrows), and quit. The PR's own
reason is the remote machine: "checking how a page renders there usually
means setting up a remote desktop or copying screenshots back", and on
macOS it fixed torn frames for CPU-read clients. `--device-pixel-ratio`
is set by hand, since terminals do not report one.

The author's demo on X the same day, "works perfectly in Rex", shows
ladybird.org and superlogical.com in a Rex for Mac tab: a full-width
rendered page with its fonts, a hover cursor, scrolling, a link followed,
and the URL prompt typed into. The frames are clean; no tearing is
visible at 2 s intervals, which says nothing about CPU.

For docz-review this is the page, unchanged, in a floating terminal, with
no change to Rex and none to docz-review: `DOCZ_REVIEW_BROWSER` already
names any opener, so a script that calls `session.new_layer` with
`Ladybird --headless=terminal "$1"` is the whole integration, and on a
remote Rex server it runs where the review server runs, so no port is
forwarded and the token never leaves the host. Three things stand between
it and a recommendation. Ladybird has no release: its site says the first
alpha for macOS and Linux is "coming later this year", and the repository
has no GitHub release. It does have a `Build Release` workflow that makes
attested, versioned packages as workflow artifacts (`2026.11.1`: a macOS
arm64 tarball, Debian and RPM packages, flatpaks), downloadable by anyone
logged into GitHub with `gh run download`; but its latest run is
2026-11-05, four days before the terminal mode merged, so until the next
run a user builds from source (Qt 6.10, CMake 3.30, clang 22 or gcc 16,
Rust, nasm). The review page is React with DOM selection and `fetch`, and
Ladybird's compatibility with it is unmeasured; and the Rex Kitty-graphics
numbers in Observation 1 were measured on terminal-browser, so step 4 is
where Ladybird is measured. A Kitty-keyboard dead-key bug in Rex (active
feedback) would reach typed annotations with accents.

### Observation 4: floating layers are an API, with a command, a window, and a state

`session.new_layer` takes a `session_id`, `bounds` as `[x, y, w, h]`
fractions of the window, a `layout` of `block` and `split` nodes, an
optional `window_id` (the active window when left out), and `focus`. A
`block` node names a `flavor` and its options, so one call starts a
terminal running a program in a directory with a PATH prefix:

```sh
rex api call -s "$REX_SESSION" session.new_layer '{
  "bounds": [0.1, 0.08, 0.8, 0.84],
  "focus": true,
  "layout": {"block": {
    "flavor": "com.superlogical.terminal.shell",
    "label": "docz review",
    "options": {"command": ["docz-review", "attach"], "cwd": "/repo"}
  }}
}'
```

It returns the `layer_id` and `block_ids`. `session.set_layer_state`
minimizes a layer or returns it to normal with focus; `session.close_layer`
closes it and every block in it; `session.resize_layer`, `raise_layer`,
and `lower_layer` do what they say. The Lua side has the same calls, and
a community `init.lua` already does exactly the shape a review pane needs:
one `rex.action` that creates a centred layer the first time and toggles
`minimized` after, bound to a key.

In Rex for Mac the pane actions are `pane.float` (⌥⌘↩, preview.22,
2026-10-27), `pane.new_floating` (preview.26, 2026-11-05),
`pane.minimize_floating` (⌥⌘M, preview.25), and
`pane.minimize_all_floating`. A minimized layer is a capsule at the
bottom right of the window. Floating panes reached iPad on 2026-11-06.
Session restore brings floating panes back with the session (preview.23).
A request to hide a layer's chrome (header, border, shadow) is active;
a review pane would want it, but does not need it.

What the window is matters: a layer belongs to a window, and the agent's
block is in one. `rex block inspect "$REX_BLOCK"` reports
`.block.window_id`, so the hook can put the review over the agent, not
over whatever the person is looking at. The person may be in another
session; `session.set_layer_state` with `focus` makes the window active,
and step 2 records what that looks like from the other session.

### Observation 5: the hook can own the wait as it does now

Rex sets `REX_SESSION` and `REX_BLOCK` in every terminal it opens, and
Claude Code's hooks run with the agent's environment, so
`docz-review --pending --hook` should see both (step 2 confirms it). It
already runs a server and blocks until a submission; nothing about that
changes. What changes is the opener and the end:

- The opener is one `session.new_layer` call instead of `open`. The
  server's URL carries the token (DESIGN-0024 §6), and today's opener
  passes it as a process argument. On a Rex **shared server** every
  program runs as `rex` and `ps` is shared, so the token must reach the
  client through its environment or stdin, never `argv`.
- The end is the submit, as now. A client that exits without submitting
  is the dismiss: `rex wait BLOCK` returns the terminal child's exit
  status, and the terminal emits `child_exited`, so the hook can tell a
  closed pane from a crashed one and keep the review pending for the
  next turn (DESIGN-0024 question 2's rule).
- `rex block inspect` lists a block's methods; the terminal's include
  `format` (its screen as text) and `program_status`. Nothing here is
  needed for the review, but the agent could read the pane.

### Observation 6: the Program Status Protocol is the status a browser never had

OSC 7501 lets a program tell the terminal it is `idle`, `working`, `done`,
`blocked`, or `error`, with `kind` and `msg` on `blocked`. Rex shows a
blocked or done task in the session picker and in the tab header of an
unfocused tab, Lua scripts get `terminal.program_status_changed`, and the
docs plan push notifications to the mobile apps and "a unified priority
inbox". `blocked` is dropped when the process exits or a new prompt
begins; `done` and `error` survive both.

A review pane reports `blocked` with `kind=input` and `msg="DESIGN-0024,
round 2: 3 findings"` while it waits, and `done` as it exits. Then a
person in another session sees that a review is waiting without a browser
tab, a bell, or the agent's block. The browser page has no equivalent, and
Claude Code's own status in Rex (the spinner on its tab) says only that
the agent is stopped. Step 3 confirms the rendering.

### Observation 7: the review's logic is already DOM-free; the page is not

`ui/src/review/anchor.ts` resolves an annotation's anchor by parsing the
document with the remark pipeline and matching rendered text with its
source position (DESIGN-0024 §7); its imports are unified, mdast, and
hast, no DOM. `feedback.ts` builds the round's message from the record,
`draft.ts` holds the unsent draft, and `api.ts` is the typed client of
the server. A terminal client reuses all four. What does not carry over is
`app.tsx`, `document-review.tsx`, `regions.tsx`, `findings.tsx`,
`questions.tsx`, `diff.tsx`, and `controls.tsx`: the rendering, the DOM
selection, and the controls, about half the directory by line count.

A terminal selection is a line range, not a DOM range, which is what
Plannotator's diff annotation already is and what INV-0032 Observation 7
found acceptable: a block-level annotation carries the block's text as its
`exact`, and `anchor.ts` resolves it the same way. The question cards,
the ten labels, and "accept all recommendations" (INV-0032) are lists and
keys, which a terminal does well; the rendered diff since the last round
(INV-0032 gate 2) is the hard part, and a line diff of the source is the
fallback the page started with.

### Observation 8: a Rex integration is a CLI call and an `init.lua` snippet, not a plugin

Rex has no plugin registry. Its extension points are the server API and
CLI (`rex api call`, `rex do`), `init.lua` actions and key bindings
(`rex.action`, `rex.bind`), event handlers (`rex.on`), and the agent skill
Rex installs into `~/.claude/skills/rex` with a session-start hook in
`~/.claude/settings.json`. That hook and docz-review's plugin hooks are
unrelated entries in the same file and coexist.

So "a plugin for Rex" is: docz-review's opener learning the layer call;
the review pane reporting OSC 7501; and a documented `init.lua` action
in `plugins/docz-review/docs/` that minimizes and restores the review
layer by label, after the community example. Nothing is installed into
Rex.

### Observation 9: what is not yet measured

Steps 2–8 have not run. In particular: whether a hook started by Claude
Code sees `REX_SESSION` (it should; `env` in a Bash tool call does); how a
layer over an inactive window reads from another session; whether OSC
7501 from a floating pane is shown; Ladybird on the page, its CPU in Rex,
and whether a selection takes; the browser pane's flavor and whether the
API can open it at a URL, in a floating layer, on a remote server; and
only if both fail, which TUI candidate reuses the most.

One thing the demo makes testable now: the badge detects a listening
server in the terminal's process tree. docz-review's server is a child of
Claude Code in the agent's terminal and listens on `127.0.0.1`, so a Rex
build with the pane may show **Open Beside** on the agent's pane the
moment the hook starts the server. The badge's URL would lack the token,
and DESIGN-0024 §6 refuses a page load without it, so detection alone is
not an opener; but it is a free signal that a review is waiting.
<!--docz:findings:end-->

<!--docz:conclusion:start-->
## Conclusion

**Answer:** Yes, provisionally, and as the page once Rex's browser pane
ships.

Part 1 is no on the preview and yes on two demos: the preview's only
user-facing block is a terminal and terminal-browser in Rex is a deferred
performance problem, but a native browser pane is built, opens from a
terminal's detected server, and forwards a remote port to the client's
browser; and Ladybird now draws a page into a terminal with mouse and
keyboard, in Rex, from a source build. With the pane's flavor the opener
becomes one `session.new_layer` call; with Ladybird it is one
`DOCZ_REVIEW_BROWSER` script; in neither case does anything else in
docz-review change.
Part 2 is the fallback, yes on the API's evidence and unrun on the hook's:
the review server and record need no change, and the review's anchor,
feedback, draft, and client modules are DOM-free, but the rendering would
be a terminal port of the page. Part 3 is the Program Status Protocol and
the remote server, both of which the browser pane keeps.

The conclusion closes when steps 2, 3, 4, 6, and 8 have run and the
hook, the status, Ladybird's numbers, the remote case, and the pane's API
are seen rather than read.
<!--docz:conclusion:end-->

<!--docz:recommendation:start-->
## Recommendation

1. Do not build a terminal client now. The browser pane, and Ladybird
   before it, make the page the review surface in Rex too, and INV-0032's
   page work carries over whole. Step 5 waits on steps 4 and 8 both
   failing.
2. Run steps 2 and 3 this week; they are an hour, need no new Rex build,
   and pay off either way. The `Stop` hook can write OSC 7501 `blocked`
   to the agent's terminal today, so Rex marks the agent's pane as
   waiting on the person before any pane exists. If the hook does not see
   `REX_SESSION`, the opener takes the session from `rex whoami --json`.
3. Run step 8 the day a build with the pane is installable, and step 6
   with it. Three questions: the flavor's name and whether it takes a
   `url`; whether `session.new_layer` accepts it, so the review floats
   over the agent's window; and whether the forwarded port and the
   client's browser hold the token URL on a remote server. Put the
   answers here, and anything missing to the Rex team as feedback with
   docz-review as the use case.
4. Then write a DESIGN for the opener: `--open rex|browser` with
   `DOCZ_REVIEW_OPEN` and `auto` on `REX_SESSION`; the browser pane in a
   floating layer when the flavor exists, the browser as now otherwise;
   the token kept out of `argv` on shared servers; OSC 7501 from the
   hook; the `init.lua` snippet in the plugin's docs. It is a small
   DESIGN, and most of it is the opener.
5. Run step 4 with Ladybird as soon as a build is to hand: a source build
   now, or the first `Build Release` artifact after 2026-11-09. It is
   the one path that needs nothing from Rex and nothing from docz-review,
   so a good number there is a working review in a floating pane this
   month.
   If it holds, document the `DOCZ_REVIEW_BROWSER` script in the plugin's
   docs as the Rex recipe until the pane ships, with the `argv` caveat
   for shared servers. Run terminal-browser once, for the record.

> **Amended 2026-11-12.** Recommendation 1 is superseded. The decision
> is to build the terminal client now, as **docz-review-rex**: a second
> client of the docz-review server, drawn by `docz view`'s renderer in a
> Rex floating layer, which a person installs as an option beside the
> browser page. docz-review is not replaced, and the browser pane and
> Ladybird stay the path for the page when they ship. Recommendation 4's
> opener (`--open rex|browser|auto`, `DOCZ_REVIEW_OPEN`, the token out of
> `argv`, OSC 7501, `init.lua`) is part of
> [DESIGN-0027](../design/0027-docz-view-and-docz-review-rex-a-terminal-renderer-for-docz.md).

<!--docz:recommendation:end-->

<!--docz:references:start-->
## References

- [DESIGN-0024](../design/0024-docz-review-a-blocking-review-page-for-agent-written-documents.md): docz-review, §2 the session end to end, §6 the local
  server, §7 anchors, §8 the record, §9 the plugin
- [IMPL-0027](../impl/0027-docz-review-the-cli-contract-the-review-page-the-binary-and-the.md): docz-review's implementation
- [INV-0030](0030-docz-review-follow-ups-after-v230.md): docz-review follow-ups after v2.3.0
- [INV-0032](0032-docz-reviews-page-after-plannotator-adopt-its-layout-or-fork-it.md): docz-review's page after Plannotator, Observation 7 and the
  five gates
- [`ui/review/main.ts`](../../ui/review/main.ts) (`browserOpener`), and the DOM-free review modules
  [`anchor.ts`](../../ui/src/review/anchor.ts), [`feedback.ts`](../../ui/src/review/feedback.ts),
  [`draft.ts`](../../ui/src/review/draft.ts), [`api.ts`](../../ui/src/review/api.ts)
- Rex docs: [The CLI](https://www.superlogical.com/rex/docs/automate/cli),
  [Server API](https://www.superlogical.com/rex/docs/automate/server-api),
  [Server API Methods](https://www.superlogical.com/rex/docs/reference/api),
  [Mac Actions](https://www.superlogical.com/rex/docs/reference/mac-actions),
  [Lua API](https://www.superlogical.com/rex/docs/reference/lua),
  [Events](https://www.superlogical.com/rex/docs/reference/events),
  [Program Status Protocol](https://www.superlogical.com/rex/docs/build/program-status),
  [Agents](https://www.superlogical.com/rex/docs/automate/agent-skill),
  [Community MCP](https://www.superlogical.com/rex/docs/beta/community-mcp)
- Rex feedback, read through the community MCP (Discord links need
  membership of the Rex Beta server):
  [Browser and file browser block types](https://discord.com/channels/1546797889009881158/1563186888418787401) (deferred, 2026-10-23),
  [Some feedback](https://discord.com/channels/1546797889009881158/1566084925738713158) (active, 2026-10-31, the plannotator ask),
  [Floating panes starting as floating panes](https://discord.com/channels/1546797889009881158/1566094627407269929) (active, the `session.new_layer` `init.lua` example),
  [Programmatically hide chrome on floating windows](https://discord.com/channels/1546797889009881158/1566968591062929548) (active, 2026-11-03),
  [Kitty graphics: high CPU and slow scrolling](https://discord.com/channels/1546797889009881158/1567294780377931869) (deferred)
- [Ladybird PR 14186](https://github.com/LadybirdBrowser/ladybird/pull/14186), "Add a headless mode that shows pages in the terminal", merged 2026-11-09, and
  [its author's demo in Rex](https://x.com/KalenikW/status/2119783674870997168) the same day
- Rex changelog, in the Discord `#changelog` channel:
  [Mac preview.22](https://discord.com/channels/1546797889009881158/1547349572299395172/1564758652316033105) (2026-10-27, Float Pane),
  [Mac preview.25](https://discord.com/channels/1546797889009881158/1547349572299395172/1566944303526453400) (2026-11-02, Minimize Floating Window),
  [Mac preview.26](https://discord.com/channels/1546797889009881158/1547349572299395172/1567964239929286739) (2026-11-05, New Floating Pane),
  [CLI v0.1.0-d018d2d](https://discord.com/channels/1546797889009881158/1547349572299395172/1566944836957900911) (2026-11-02, minimized floating panes on the server),
  [iOS 1853](https://discord.com/channels/1546797889009881158/1547349572299395172/1568331876983377983) (2026-11-06, floating windows on iPad)

<!--docz:references:end-->
