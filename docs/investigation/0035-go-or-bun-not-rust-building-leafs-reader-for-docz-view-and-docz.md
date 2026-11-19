---
id: INV-0035
title: "Go or Bun, not Rust: building leaf's reader for docz view and docz-review-rex without a third toolchain"
status: Concluded
author: Donald Gifford
created: 2026-11-18
---

<!-- markdownlint-disable-file MD025 MD041 -->

# INV-0035: Go or Bun, not Rust: building leaf's reader for docz view and docz-review-rex without a third toolchain

<!--toc:start-->
<!-- markdownlint-capture -->
<!-- markdownlint-disable MD013 -->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
  - [Observation 1: leaf is now docz-review's loop for any markdown, and still not docz](#observation-1-leaf-is-now-docz-reviews-loop-for-any-markdown-and-still-not-docz)
  - [Observation 2: the four Rex viewers map the field, and none knows a region](#observation-2-the-four-rex-viewers-map-the-field-and-none-knows-a-region)
  - [Observation 3: every piece of leaf's reader has a Go answer and a Bun answer](#observation-3-every-piece-of-leafs-reader-has-a-go-answer-and-a-bun-answer)
  - [Observation 4: mermaid in Go, no browser, no cgo](#observation-4-mermaid-in-go-no-browser-no-cgo)
  - [Observation 5: mermaid in Bun, the same, faster](#observation-5-mermaid-in-bun-the-same-faster)
  - [Observation 6: one licence catch on the Go side, none on the Bun side](#observation-6-one-licence-catch-on-the-go-side-none-on-the-bun-side)
  - [Observation 7: a Rust plugin would reuse nothing a Bun client does not, and Bazel has no job](#observation-7-a-rust-plugin-would-reuse-nothing-a-bun-client-does-not-and-bazel-has-no-job)
  - [Observation 8: Rex Custom UI asks a question anywhere a Rex app is open, but not on this Mac yet](#observation-8-rex-custom-ui-asks-a-question-anywhere-a-rex-app-is-open-but-not-on-this-mac-yet)
  - [Observation 9: a Claude Code pane is a third client, over the same frame](#observation-9-a-claude-code-pane-is-a-third-client-over-the-same-frame)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
- [References](#references)
<!-- markdownlint-restore -->
<!--toc:end-->

<!--docz:question:start-->
## Question

Can everything leaf does that DESIGN-0027 wants (prose laid out as
styled text with offsets that survive reflow, pictures through the
Kitty graphics protocol, mermaid drawn without a browser, a pane
opened beside the agent in Rex, and a verdict handed back to the
agent) be built in Go inside the `docz` binary, or in Bun inside the
docz-review binary, without duplicating docz's Go work? If neither
can, is a Rust binary the right shape for a Rex plugin, and does a
third toolchain mean Bazel has to manage the repository?

Three constraints are given: the docz CLI and docz-api stay Go; the
review record, server, and page stay as DESIGN-0024 has them; and
whatever the Rex plugin is, it must reuse the Go work rather than
rewrite it.
<!--docz:question:end-->

<!--docz:hypothesis:start-->
## Hypothesis

Yes in both runtimes. Every piece of leaf but one has a plain answer
in Go (goldmark, Bubble Tea, stdlib PNG, escape sequences, `os/exec`
to `rex`) and in Bun (remark, which `ui/src` already parses with, a
raw-mode loop, `bun build --compile`, which docz-review already
ships). The one piece in doubt is mermaid without a browser: docz
vendors `mermaid.min.js`, which wants a DOM, and INV-0034 Observation
6 called it "the one piece with no Go renderer". tkmp shows the way
out, Pele (Obsidian's DOM-free mermaid renderer) inside an embedded
QuickJS with resvg for the SVG, so the expectation is that Pele runs
under a QuickJS compiled to wasm in Go, and natively in Bun, and that
resvg is reachable as wasm in both. If that holds, Rust buys nothing
but leaf's own code to copy, and Bazel has no job: `just` already
runs the Go and Bun halves as modules.
<!--docz:hypothesis:end-->

<!--docz:context:start-->
## Context

**Triggered by:** DESIGN-0027's review on 2026-11-13, and the
direction that followed it: find out whether the Rex client can be
Go or Bun; if not, look at a Rust binary as the Rex plugin, and at
Bazel to hold Go, Rust, and Bun in one repository.

DESIGN-0027 (Draft, 2026-11-12) proposed `docz view`, a terminal
renderer over docz-html's structure, and docz-review-rex, a second
client of the docz-review server written in Go as `docz view
--review`. It took leaf as prior art only. Between the draft and the
review, leaf moved from 0.11 to 0.17 and grew Margin, a plan review
that is docz-review's loop for any markdown, and four more Rex
markdown viewers turned up (tkmp, mddog, rex-md-viewer, rexworks
md-viewer), each solving one of the reader's problems a different
way. Rex also shipped Custom UI on 2026-11-12, palette prompts a
custom action can ask from any connected Rex app, which is a fourth
way a review could reach a person.

The question matters because DESIGN-0027's §7 proposed rewriting
docz-review's anchor logic in Go with a parity test against the page,
which is exactly the duplication the direction forbids if a Bun client
could reuse `anchor.ts` as it is.
<!--docz:context:end-->

<!--docz:approach:start-->
## Approach

1. Read leaf 0.17's source and README for what changed since
   INV-0034, and read tkmp, mddog, rex-md-viewer, and rexworks
   md-viewer for how each draws, places pictures, runs mermaid, and
   owns its Rex pane. (Observations 1 and 2.)
2. Map every piece of leaf's reader to its Go and Bun answers, and
   name the one in doubt. (Observation 3.)
3. Spike mermaid in Go: load Pele's bundle into a QuickJS compiled
   to wasm (`fastschema/qjs` under wazero, no cgo), render a
   four-node flowchart to SVG, then rasterise the SVG with resvg as a
   wasm module; measure load, render, and binary size. (Observation 4.)
4. Spike the same in a Bun compiled binary: Pele from npm, built into
   an IIFE, and `@resvg/resvg-wasm`; measure the same. (Observation 5.)
5. Check the licence of every module either spike pulled in against
   `just license-check`'s allow list. (Observation 6.)
6. Cost a Rust plugin honestly: what it would reuse from Go, what it
   would rewrite, and what a third toolchain adds to CI, releases,
   and remote hosts. (Observation 7.)
7. Probe Rex Custom UI on the local server: whether `rex.ui` exists,
   what a prompt returns, and which versions the docs require.
   (Observation 8.)
8. Read mddog's Claude Code hook module and the mods API for the pane
   it draws beside the chat, as a possible third client. (Observation 9.)
<!--docz:approach:end-->

<!--docz:environment:start-->
## Environment

| Component | Version / Value |
| --------- | --------------- |
| docz | `docs/inv-rex-review-pane` at `185e281`, on `main` `c8c929c` (v2.6.1); Go 1.26.10, Bun 1.4.4 |
| leaf | `leaf-reader` 0.17.x (Rust, MIT): Margin plan review, agent hooks, Rex / tmux / Herdr, mermaid by shelling to `bun` or `node` |
| tkmp | Rust, MIT: Pele in embedded QuickJS (rquickjs) with a text-measuring callback, resvg, Kitty Unicode placeholders (`U=1`), one 6 MB binary |
| mddog | Rust, macOS only: CoreText rasteriser to Kitty tiles, `mermaid-rs-renderer` (23 diagram types), a Claude Code plugin drawing a pane through the mods API |
| rex-md-viewer | Go, MIT: goldmark, chroma, `x/image/font` with bundled Go fonts, page to PNG to Kitty tiles; panes labelled `mdv:<token>` and verified before close |
| rexworks md-viewer | Swift, macOS 14: selection, find, and links over Kitty pictures, Rex's theme read live, Neovim sync, agent skills |
| Pele | 1.13.0 (MIT, node ≥ 20, every Mermaid 12.1 diagram type); npm ships ESM and CJS, no IIFE |
| QuickJS in Go | `fastschema/qjs` (MIT): QuickJS compiled to wasm, run under wazero, ES2023, no cgo |
| resvg | resvg itself MPL-2.0; `kanrichan/resvg-go` GPL-3.0; `@resvg/resvg-wasm` 2.6.2 MPL-2.0 |
| TUI | Bubble Tea v2.1.1 (Go); OpenTUI 0.9.2 (Bun, Zig native core, bun ≥ 1.3.14) |
| Rex | server `0.1.0-preview.21` (control API revision 18) after an upgrade from `0.0.1793645657` (revision 10) during the investigation; Rex Beta for Mac 0.1.0 build 2042, Preview channel |
| Rex Custom UI | docs dated 2026-11-12; prompts need control API revision 17 (server preview.17) |
<!--docz:environment:end-->

<!--docz:findings:start-->
## Findings

### Observation 1: leaf is now docz-review's loop for any markdown, and still not docz

leaf 0.17 adds Margin: `leaf plan plan.md` splits a markdown plan into
blocks, a person comments on, removes, or replaces blocks and lines,
sends approve, approve-with-notes, or changes-requested, and the
verdict is typed into the agent's pane as its next message. It opens
beside the agent in Rex, tmux, or Herdr, carries a Claude Code
`PermissionRequest` hook on `ExitPlanMode`, and installs skills into
seven agents. That is the shape of docz-review's hold-the-turn loop,
built for any file.

Two things keep it prior art. It knows nothing of docz: no regions, no
questions with lettered options, no findings, no statuses, no review
record, so its verdict is prose and docz-review's is a record the
agent's instruction reads. And its mermaid runs by shelling out to
`bun` or `node`; the embedded engine that avoids a runtime is tkmp's,
not leaf's. Nothing leaf does needs Rust in particular: ratatui is its
loop, pulldown-cmark its parser, and both have Go and Bun peers.

### Observation 2: the four Rex viewers map the field, and none knows a region

| Viewer | Draws | Pictures | Mermaid | Teaches |
| ------ | ----- | -------- | ------- | ------- |
| tkmp | text, pictures only for diagrams and large headings | Kitty Unicode placeholders, so a picture sits in the text grid and scrolls with it | Pele inside embedded QuickJS, resvg to PNG, no runtime | the mermaid answer; placeholders; a 15 ms debounce and a changed-block gutter mark for watching an agent write |
| rex-md-viewer | the whole page as a PNG | Kitty tiles | none | Go can rasterise a page with no cgo; pane ownership by a labelled block, a state file, and a check before close; `rex -S $REX_SERVER --autostart=false --timeout 10s` |
| rexworks md-viewer | the page as pictures | Kitty frames composed for partial updates | in the page | selection, find, and links work over pictures; Rex's theme read live; a Neovim plugin with cursor sync |
| mddog | the page through CoreText | Kitty tiles, macOS only | `mermaid-rs-renderer` | a browserless mermaid renderer exists (in Rust); a Claude Code pane beside the chat through the mods API |

All four reload on save, because the use is a preview beside an agent
while it writes. None of them reads a `<!--docz:…-->` marker, so a
docz document in any of them is its source with the markers gone,
which is the niche `docz view` fills. Two corrections to INV-0034
fall out: a picture can be selected and searched (md-viewer and mddog
do it by keeping the text in a display list), though reflow is still
a re-render; and mermaid does not need a DOM, it needs a renderer,
and two exist outside a browser.

### Observation 3: every piece of leaf's reader has a Go answer and a Bun answer

| leaf piece | Go, in `docz` | Bun, in the docz-review binary |
| ---------- | ------------- | ------------------------------ |
| Styled rows with text offsets, selection, reflow | own writer over goldmark, as DESIGN-0027 §3 | own writer over remark, which `ui/src` already parses with |
| TUI loop, keys, mouse, resize | Bubble Tea v2 in `cmd/` | OpenTUI, or a raw-mode loop written here |
| Kitty graphics | escape sequences, stdlib PNG | escape sequences, PNG from resvg-wasm |
| Mermaid to SVG | Pele under QuickJS in wazero (`fastschema/qjs`), spiked | Pele natively, spiked |
| SVG to PNG | resvg as wasm under wazero, spiked | `@resvg/resvg-wasm`, spiked |
| Review logic: anchors, feedback, draft, API | a rewrite plus a parity test | `anchor.ts`, `feedback.ts`, `draft.ts`, `api.ts` unchanged |
| docz structure | `pkg/` directly | `docz inspect --format json`, which the server already consumes, or a frame the Go renderer emits |
| Rex, tmux, Herdr opener | `os/exec` to `rex` and `tmux` | `Bun.spawn` to the same |
| One binary, four targets | `go build` | `bun build --compile`, which docz-review already does |

The only row with a question mark before the spikes was mermaid.
Everything else is a known library or a hundred lines of escape
sequences in either language.

### Observation 4: mermaid in Go, no browser, no cgo

The Go spike loaded Pele's bundle into `fastschema/qjs` (QuickJS
compiled to wasm, run by wazero, so a pure-Go module with no cgo),
gave it a text-measuring callback, rendered a four-node flowchart to
SVG, and rasterised the SVG through a resvg wasm module under the
same wazero runtime.

| Step | Cost |
| ---- | ---- |
| Load Pele into QuickJS | 328 ms |
| Render the flowchart to SVG | 1 ms |
| Initialise resvg wasm | 231 ms |
| SVG to PNG (800 × 289) | 5 ms |
| Binary, `go build`, both wasm modules embedded | 8.5 MB |

The load costs are paid once per process and can be paid lazily, on
the first diagram, so a document with no mermaid pays nothing. The
output matched the Bun spike's picture. The `docz` binary today is
about 30 MB with `mermaid.min.js.gz` inside it, so the two modules
are a modest addition, and the CDN mode of `docz export html` is
unaffected.

### Observation 5: mermaid in Bun, the same, faster

The Bun spike installed `pele` from npm, built an IIFE from an entry
of `import * as p from "pele"; globalThis.pele = p;` with
`bun build --format=iife` (npm ships only ESM and CJS), added
`@resvg/resvg-wasm`, and compiled the result with `bun build
--compile`.

| Step | Cost |
| ---- | ---- |
| Pele load | inside the compiled binary's start |
| Render the flowchart to SVG | 7 ms |
| SVG to PNG (800 × 289) | 22 ms |
| Binary | 93 MB, the same size as docz-review today |

Bun runs Pele as it runs any module, so there is no engine to embed
and the only cost is the wasm rasteriser. The docz-review binary is
already a compiled Bun program of this size, so a review client inside
it adds a few hundred kilobytes.

### Observation 6: one licence catch on the Go side, none on the Bun side

`just license-check` runs `go-licenses` over the module.
`kanrichan/resvg-go`, the ready-made resvg wasm wrapper the Go spike
used, is **GPL-3.0** and cannot ship in docz. resvg itself is MPL-2.0,
Pele is MIT, and `fastschema/qjs` is MIT. The path is the one docz
already takes for mermaid: a wrapper of about fifty lines of Rust
around resvg, compiled to wasm once with `wasm32-wasip1`, vendored
under `pkg/export/term/assets/` with its version and hash pinned by a
test, as `just vendor-mermaid` pins `mermaid.min.js.gz`. A Rust
toolchain is needed at vendor time, never at build time, and never in
CI. `@resvg/resvg-wasm` is MPL-2.0 and ships in Bun with no wrapper.

### Observation 7: a Rust plugin would reuse nothing a Bun client does not, and Bazel has no job

A Rust binary as the Rex plugin would rewrite every row of
Observation 3's first column (the writer, the loop, the pictures, the
review logic) and reuse docz's Go work only through `docz inspect
--format json`, the same seam a Bun client gets for free. On top of
that it adds a third toolchain to `mise.toml` and CI, cross-compiles
for four targets, `cargo install` or a release fetch on every remote
Rex host, and a second copy of the anchor logic to keep in parity
with the page. What it buys is leaf's source to copy and ratatui.
Neither is worth the cost when both existing runtimes can do the
work.

Bazel's case was a repository holding Go, Rust, and Bun. Without Rust
there is nothing new to hold: `justfile` already composes `docz.just`,
`api.just`, `ui.just`, and `chart.just` as modules, `just ci` runs the
Go and Bun gates, and a Bun review client lives in `ui/review/` under
`ui.just`'s recipes with no new build system.

### Observation 8: Rex Custom UI asks a question anywhere a Rex app is open, but not on this Mac yet

Rex's Custom UI (docs dated 2026-11-12) lets a custom action pause
and ask through the command palette: `rex.ui.input` for a line of
text, `rex.ui.pick` for a list (up to 2,000 items, `multiple`,
`allow_other`, icons). Every connected Rex app is asked, other
devices included, the first answer wins, and cancelling on one cancels
everywhere. The timeout is 300 s by default and 3,600 s at most. A
prompt works only inside an action's `run` and the functions it calls
directly: not in a `rex.bind` function, a `rex.on` handler, anything
under `pcall`, or an action run by `rex.invoke`. An action has 30 s to
finish and stops any command it started, so a long-lived process must
be `rex.exec.start{detach = true}`. When nobody can answer, the call
returns `nil` and a reason: `cancelled`, `timeout`, `no_client`,
`unsupported` (no connected app can show prompts), `busy`, or
`failed`.

The probe was a throwaway action calling `rex.ui.pick` with a 5 s
timeout, run with `rex do <file> --action`:

| Server | Mac app | `rex.ui` | Result |
| ------ | ------- | -------- | ------ |
| `0.0.1793645657`, revision 10 | build 1996 | absent | `{"ui":"absent"}` |
| `0.1.0-preview.20`, revision 16 | build 2042 | absent | `{"ui":"absent"}` |
| `0.1.0-preview.21`, revision 18 | build 2042 | present | `{"ui":"present","reason":"unsupported"}` |

The server shipped prompts one release ahead of the Mac app: on
2026-11-18 the Preview build connected to a server with `rex.ui`
answers every prompt `unsupported`, and `client.inspect` on it lists
217 actions with nothing prompt-related. So a client must handle
`unsupported` per call, falling back to a pane or the browser, because
a server can support prompts while every app connected to it does
not. The decision was to wait for the Preview channel rather than
switch the app to Bleeding Edge.

What a prompt is good for in a review: a lettered open question is a
`pick` with one item per letter and `allow_other` for "Other"; a
status change is a `pick` over the type's statuses; findings to keep
are a `pick` with `multiple`; a verdict is a three-item `pick`. What
it cannot do: show a document, or anchor a comment to a block. It is a
companion to the pane, not a replacement, and its hour-long ceiling
sits inside docz-review's 24-hour hold as one round.

### Observation 9: a Claude Code pane is a third client, over the same frame

mddog's plugin uses Claude Code's mods API: `$.ui.open`, a `ui.render`
hook drawing `Box`, `Text`, `Image`, and `Svg`, and `$.process.spawn`
running its own binary headless. The API also offers `Button` with
hotkeys, `Input`, and `Select`, and draws on the terminal, the desktop
app, VS Code, and mobile. A docz-review client in that form would run
`docz` and draw a tree, which is what DESIGN-0027 §3's Frame already
is (tabs, sections, lines, spans, blocks with their line and text).
If `docz view` can emit that frame as JSON, the Go renderer stays the
one renderer and the pane is a third front end over it, as a Bun
client in Rex would be a second. Whether a pane inside the same Claude
Code process can hold the turn the way the Stop hook does is untested
and is the first thing to check before designing it.
<!--docz:findings:end-->

<!--docz:conclusion:start-->
## Conclusion

**Answer:** Yes. Everything leaf does that DESIGN-0027 wants can be
built in Go or in Bun, and the right split uses both without
duplicating either: **`docz view` stays Go**, in the `docz` binary,
over docz-html's structure, with mermaid through Pele in an embedded
QuickJS and a vendored resvg wasm module, so the reader needs no
runtime and the library's feature stays in the library; and **the
review client is Bun**, inside the docz-review binary, so
`anchor.ts`, `feedback.ts`, `draft.ts`, and `api.ts` are reused as
they are and the Go anchor rewrite and its parity test disappear. The
client draws either from its own writer over remark or from a frame
the Go renderer emits as JSON; the frame is the better seam, because
it also serves a Claude Code pane later and keeps one renderer.

No Rust: a Rust plugin would reuse nothing a Bun client does not,
and would add a toolchain, cross-compiles, and remote installs for
the sake of copying leaf. No Bazel: without Rust there is nothing new
for it to hold, and `just`'s modules already run both halves.

Rex Custom UI is a fourth surface, a palette prompt for the parts of
a review that are a choice, usable from a phone; it complements the
pane and must be handled per call, since on 2026-11-18 the server
supports it and the Preview Mac app does not.
<!--docz:conclusion:end-->

<!--docz:recommendation:start-->
## Recommendation

1. Revise DESIGN-0027 against this investigation before it leaves
   Draft: the review client becomes a Bun command in the docz-review
   binary (question 3), mermaid becomes Pele under `fastschema/qjs`
   with a vendored resvg wasm module and `mmdc` as the fallback
   (question 5), §7's Go anchors become the frame's block text held
   equal to `sourceBlocks` (question 6), pictures use Kitty Unicode
   placeholders first (§4), the opener labels its block and verifies
   it before closing (§8), `docz view --watch` joins the reader (§5),
   and the rollout's client phase moves to `ui/review/`.
2. Add three open questions to DESIGN-0027: palette prompts beside
   the pane (Custom UI), a Claude Code pane as a third client over
   `docz view --frame json`, and whether the opener also learns tmux
   and Herdr as leaf did.
3. Vendor resvg as docz's own MPL wrapper, never `resvg-go`; pin its
   version and hash in a test as `TestVendoredMermaid` pins mermaid.
4. Correct INV-0034 Observations 1 and 6 by amendment: selection over
   pictures works, and mermaid needs a renderer, not a DOM.
5. Re-run the Custom UI probe when Rex for Mac Preview carries
   prompts, and only then write the prompt path's drill.
6. Do not add Rust or Bazel to the repository for this work.
<!--docz:recommendation:end-->

<!--docz:references:start-->
## References

- [DESIGN-0027](../design/0027-docz-view-and-docz-review-rex-a-terminal-renderer-for-docz.md): the design this revises
- [INV-0033](0033-docz-review-inside-a-rex-floating-pane-instead-of-a-browser.md) and [INV-0034](0034-a-terminal-viewer-for-docz-documents-docz-htmls-page-as-a-cli.md): the investigations before it
- [DESIGN-0024](../design/0024-docz-review-a-blocking-review-page-for-agent-written-documents.md): docz-review, whose `ui/src/review/anchor.ts`, `feedback.ts`, `draft.ts`, and `api.ts` the Bun client reuses
- [leaf](https://github.com/DeepanshuMishraa/leaf) and [its site](https://leaf.dipxsy.app/): Margin, the agent hooks, the Rex / tmux / Herdr opener
- [tkmp](https://github.com/tfcace/tkmp): Pele in embedded QuickJS, Kitty Unicode placeholders
- [mddog](https://github.com/qu8n/mddog): CoreText rasteriser, `mermaid-rs-renderer`, the Claude Code pane
- [rex-md-viewer](https://github.com/zztkm/rex-md-viewer): page to PNG in Go, pane ownership by label
- [rexworks md-viewer](https://github.com/good-banana/rexworks/tree/main/md-viewer): selection over pictures, Rex's theme
- [Pele](https://github.com/obsidianmd/pele) and [pele.run](https://pele.run): the DOM-free mermaid renderer
- [fastschema/qjs](https://github.com/fastschema/qjs): QuickJS as wasm under wazero
- [resvg](https://github.com/linebender/resvg) (MPL-2.0), [resvg-go](https://github.com/kanrichan/resvg-go) (GPL-3.0, not usable), [@resvg/resvg-wasm](https://www.npmjs.com/package/@resvg/resvg-wasm)
- [Bubble Tea](https://github.com/charmbracelet/bubbletea), [OpenTUI](https://github.com/sst/opentui)
- [Rex: Custom UI](https://www.superlogical.com/rex/docs/customize/custom-ui), [Rex: Custom Actions](https://www.superlogical.com/rex/docs/customize/actions), [Rex: Server API Methods](https://www.superlogical.com/rex/docs/reference/api)
- [Claude Code plugins](https://docs.claude.com/en/docs/claude-code/plugins): the mods API read through the `plugin-authoring` skill (`$.ui.open`, `ui.render`, `$.process.spawn`)
<!--docz:references:end-->
