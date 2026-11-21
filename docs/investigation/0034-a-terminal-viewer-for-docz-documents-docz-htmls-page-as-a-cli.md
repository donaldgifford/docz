---
id: INV-0034
title: "A terminal viewer for docz documents: docz-html's page as a CLI, wrapped by Neovim and Rex"
status: Open
author: Donald Gifford
created: 2026-11-12
---

<!-- markdownlint-disable-file MD025 MD041 -->

# INV-0034: A terminal viewer for docz documents: docz-html's page as a CLI, wrapped by Neovim and Rex

<!--toc:start-->
<!-- markdownlint-capture -->
<!-- markdownlint-disable MD013 -->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
  - [Observation 1: the "browser-looking" panes are pictures placed with the Kitty graphics protocol](#observation-1-the-browser-looking-panes-are-pictures-placed-with-the-kitty-graphics-protocol)
  - [Observation 2: leaf is the shape the question describes, without docz's model](#observation-2-leaf-is-the-shape-the-question-describes-without-doczs-model)
  - [Observation 3: mdp is the wrapping pattern, not the renderer](#observation-3-mdp-is-the-wrapping-pattern-not-the-renderer)
  - [Observation 4: what docz has is the structure; it is behind an HTML writer](#observation-4-what-docz-has-is-the-structure-it-is-behind-an-html-writer)
  - [Observation 5: glamour is the text renderer to measure against, not to ship](#observation-5-glamour-is-the-text-renderer-to-measure-against-not-to-ship)
  - [Observation 6: mermaid is the one piece with no Go renderer](#observation-6-mermaid-is-the-one-piece-with-no-go-renderer)
  - [Observation 7: Rex as a plugin host is a Lua file and a layer](#observation-7-rex-as-a-plugin-host-is-a-lua-file-and-a-layer)
  - [Observation 8: the wrappers are small; the renderer is the work](#observation-8-the-wrappers-are-small-the-renderer-is-the-work)
  - [Observation 9: what is not yet measured](#observation-9-what-is-not-yet-measured)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
- [References](#references)
<!-- markdownlint-restore -->
<!--toc:end-->

<!--docz:question:start-->
## Question

What would it take to show a docz document in a terminal the way docz-html
shows it in a page (DESIGN-0025: tabs by region, phase cards, question
cards, the three widgets, mermaid, the repository's theme), as a CLI that
a Neovim plugin and a Rex action wrap, the way `mdp` is a Go binary with a
Lua plugin around it?

Three parts, each with a concrete answer:

1. What draws it: styled text, a picture of the page placed with the
   Kitty graphics protocol, or text with pictures for the figures? The
   two screenshots that prompted this (an agent drawing a PNG into a Rex
   pane) and INV-0033's Ladybird frames are all the picture kind.
2. What docz already has that a terminal renderer reuses, and what it
   does not have: `pkg/export/html` computes the outline, the layout by
   region kind, and the typed facts, then writes HTML; `pkg/theme` is hex
   tokens per mode.
3. What the wrapping is: `mdp`'s `build.lua` downloads a release binary
   for Neovim; `leaf rex install` writes a palette action into Rex's
   `init.lua`. And where it lives: `docz view` in the one binary, or a
   plugin.
<!--docz:question:end-->

<!--docz:hypothesis:start-->
## Hypothesis

**Part 1: text, with pictures for figures.** A page rasterised whole and
placed as one Kitty image reads, but cannot be selected, searched, or
reflowed, and every scroll or tab is a new frame, which is the cost Rex
deferred on terminal-browser. leaf's shape is the right one: prose as
styled text laid out at the pane's width, with selection offsets that
survive reflow, and mermaid diagrams, images, and the widgets as Kitty
images where the terminal answers the protocol, text boxes where it does
not. The picture of the page is at most a preview mode.

**Part 2: the structure is there, behind an HTML writer.** `outline.go`
(tabs), `layout.go` (kind to component), `facts.go` (typed parse per
region kind), `questions.go`, and `widgets.go` are what the page is made
of and are unexported in `pkg/export/html`, 4,932 lines built around one
goldmark walk that writes HTML. A terminal renderer needs the same walk
with a different writer. Expect the work to be a promotion of the
outline, layout, and facts into a package both writers consume, plus a
terminal writer of about the size of `components.go` and `nodes.go`.
`pkg/theme`'s tokens are hex per mode, so a theme maps to truecolor ANSI
directly; fonts and radii do not apply. Mermaid is the one piece with no
Go renderer and needs a JS runtime, so it is optional, as it is in leaf.

**Part 3: `docz view`, with two small wrappers.** docz already releases
binaries for four platforms, so a Neovim plugin is `mdp`'s `build.lua`
pointed at docz's releases and a `:DoczView` that opens the viewer in a
terminal split, and a Rex action is leaf's installer: a marked block in
`init.lua`, a layer laid over the pane, `rex config check` then
`reload`. Neither wrapper is more than a few hundred lines of Lua.
<!--docz:hypothesis:end-->

<!--docz:context:start-->
## Context

**Triggered by:** two screenshots of Rex panes (2026-11-12): an agent
explaining a diff algorithm by drawing a PNG and placing it in the pane
beside its conversation with the Kitty graphics protocol, so the "web
browser" look of a pane is a picture; and leaf, a terminal reader with a
Rex plugin. Also INV-0033, where the review in a Rex pane turned out to
be the page in a browser pane or in Ladybird's terminal mode, which
raised the question of what docz itself could draw in a terminal.

docz-html (DESIGN-0025, IMPL-0028, IMPL-0030) turns one document into
one page: tabs by depth-0 region, a component per region kind, phase
cards with progress, question cards with options and resolutions, the
`docz-flow`, `docz-stepper`, and `docz-scorecard` widgets, mermaid, and
`pkg/theme`'s three themes. docz-site shows the same fragment (DESIGN-0026,
IMPL-0032). Nothing in docz draws into a terminal; `docz inspect` prints
facts, and `docz list` prints a table.

`mdp` (donaldgifford/mdp, v0.6.2) is the pattern the question names: a Go
binary (`mdp serve`, 1,988 lines, goldmark with chroma, mermaid, MathJax,
callouts, every asset embedded) and a Neovim plugin (`lua/mdp/init.lua`,
`build.lua`) that downloads the release binary on install and drives the
server from `:MdpPreview`, opening a browser. Its `pkg/parser` and
`pkg/theme` are its library surface.

leaf (DeepanshuMishraa/leaf, `leaf-reader` v0.11.0, Rust, MIT) reads
PDFs, markdown, articles, diffs, and pull requests in the terminal, draws
images and mermaid as Kitty graphics, and installs itself into Rex's
command palette. It is the closest existing thing to the question, and
Observation 2 is what it does and does not do.

Rex has had full Kitty graphics since Mac preview.19 and Linux server
preview.5 (2026-10-22), with remote images translated to in-band for
clients on other machines; large frames got cheaper in Mac preview.25 and
Linux preview.13 (2026-11-02/03), and terminals report their pixel size
from the start since server preview.15 (2026-11-10).
<!--docz:context:end-->

<!--docz:approach:start-->
## Approach

1. Read mdp's plugin and build script, leaf's renderer, Kitty drawing,
   and Rex installer, glamour, and Rex's Kitty changelog and feedback.
   (Done; Observations 1–8.)
2. Spike A, the picture: `docz export html` one long document
   (DESIGN-0024), rasterise it with the Playwright Chromium `ui/` already
   has, and place it in a Rex floating pane as one Kitty upload, scrolling
   by re-placing a crop of the upload (the protocol's source rectangle).
   Measure the PNG size at 2× against Rex's per-block image store, CPU
   while scrolling, and whether a tab or an expandable can be clicked at
   all. Expect this to close the route for reading and keep it for a
   glance.
3. Spike B, the text: render one document of each built-in with glamour
   first, then with a writer over the same goldmark walk
   `pkg/export/html` does, region-aware: tabs from `outline.go`'s rules,
   phase cards from `impl.Parse`, question cards from
   `kinds.OpenQuestions`, callouts, tables, task lists, code through
   chroma. Count what glamour's stylesheet cannot express and what the
   own writer needs from `pkg/export/html` that is unexported.
4. Spike C, the figures: mermaid to an image without a browser. Try
   Pele under bun, as leaf does, then the vendored `mermaid.min.js` under
   a headless DOM, then SVG to PNG in Go (resvg through wasm, or
   tdewolff/canvas). Record the dependency each brings to the docz binary
   and the fallback when the runtime is absent (the fence as code).
5. The widgets: draw `docz-flow`, `docz-stepper`, and `docz-scorecard` as
   text boxes from the validated block (`widgets.go` is Go already) and
   judge whether an image is ever needed.
6. The theme: map `pkg/theme`'s tokens in both modes to truecolor ANSI,
   hold them to `contrast.test.ts`'s pairs, and check the three themes
   against the terminal's own background (a theme sets the page's, a
   terminal has its own; leaf uses the terminal's colours and nothing
   else).
7. The wrappers: a Neovim plugin that fetches docz from its GitHub release
   as `build.lua` does and opens the viewer in a `:terminal` split, or in
   a Rex pane beside when `REX_SESSION` is set; and `docz rex install`
   writing a palette action as leaf's `plugin.rs` does, opening the viewer
   in a layer over the pane. Test both over a remote Rex host.
8. Decide where it lives: `docz view` in `cmd/` against a separate
   binary, and whether the Neovim plugin is this repository (lazy.nvim
   clones the repository it names) or `docz.nvim`.
<!--docz:approach:end-->

<!--docz:environment:start-->
## Environment

| Component | Version / Value |
| --------- | --------------- |
| docz | `main` at `c8c929c` (v2.6.1); `pkg/export/html` 4,932 lines over 11 files; `pkg/theme` 2,705 lines of CSS and JS |
| mdp | v0.6.2 at `bf5313e`, 1,988 lines of Go, goldmark 1.8.6, chroma 2.27, `lua/mdp/init.lua`, `build.lua`, goreleaser |
| leaf | `leaf-reader` 0.11.0, Rust 2024, ratatui 0.30, pulldown-cmark 0.13, resvg 0.48, mupdf 0.8, wasmtime 50 (Highlights lexers), Pele vendored; `src/render.rs` 49 KB, `src/viewer.rs` 166 KB, `src/kitty.rs` 5.5 KB, `src/plugin.rs` 14 KB, `lua/leaf/init.lua` 15 KB |
| glamour | v2.0.1 (2026-06-12), 3.8k stars; glow 28k stars |
| Kitty in Go | BourgeoisBear/rasterm (Kitty, iTerm, Sixel encoders, pushed 2026-11-05), dolmen-go/kittyimg |
| Rex | Mac preview.19 full Kitty graphics (2026-10-22); Linux server preview.5 the same; Mac preview.25 and Linux preview.13 large-frame CPU; server preview.15 pixel size at start (2026-11-10) |
| The screenshots | a post on X by @amadeus, 2026-11-12: a Rex window with an editor, a Claude Code pane, and a pane holding a PNG the agent drew at `…/scratchpad/draw/join-scope.png` (116 columns by 72 rows) |
<!--docz:environment:end-->

<!--docz:findings:start-->
## Findings

### Observation 1: the "browser-looking" panes are pictures placed with the Kitty graphics protocol

In the screenshots, the right-hand pane holds a diagram with the typeface,
colours, and hatching of a drawing, not of terminal cells: the agent's
own message says "The drawing is in the pane on the right (116 columns by
72 rows)" and names the PNG it wrote. The pane is a terminal showing one
image, and the pane header's **Exited** says the program that placed it
has already quit; the image stays because the protocol keeps it until the
terminal is cleared. That is the same mechanism as Ladybird's terminal
mode (INV-0033 Observation 3), which sends each composited frame the same
way, and as leaf's figures.

Rex's side is settled: Mac preview.19 "Kitty graphics now works
completely (including animation, unicode placeholders, etc.)", and
"remote servers that load local files for Kitty graphics automatically
translate those to in-band images for remote replicas". What remains in
the feedback is a budget, not a gap: a per-block image store of
10,000,000 bytes with silent eviction, frames of 10 MB or more dropped,
and the CPU of streaming full-pane frames continuously. A still image
placed once is the cheap case; a page re-sent on every scroll is the
expensive one.

Two ways to place an image: **unicode placeholders**, where cells carry
a placeholder glyph the terminal replaces (what ratatui-image does), and
**direct placement** at the cursor with nothing in the cells. leaf's
`kitty.rs` chose direct placement because "some terminals show
[placeholders] as glyphs around and through the image", and queues
uploads as 4,096-byte base64 chunks with fast PNG compression "while the
mouse moves". The protocol's placement carries a source rectangle, so one
tall upload can be shown a window at a time without re-sending.

> **Amended 2026-11-18.** "Cannot be selected, searched, or reflowed"
> in the hypothesis and here is half right. rexworks md-viewer and
> mddog select, search, and follow links over Kitty pictures by
> keeping the page's text in a display list beside the tiles; what
> still holds is that reflow is a re-render and every scroll a new
> frame. And tkmp draws its pictures with Unicode placeholders rather
> than direct placement, in Rex, so the glyph concern is a per-terminal
> fallback, not a reason to avoid them. INV-0035 Observation 2.

### Observation 2: leaf is the shape the question describes, without docz's model

leaf is a terminal reader, 0.11.0 on crates.io, that already does most of
the mechanics:

- **Rendering.** `render.rs` "lays markdown out into rows of styled text
  for a given column width", appending every readable run to a plain
  text with offsets, "so selection and highlights work in those offsets
  and survive a change of width". Headings, tables, task lists,
  strikethrough, footnotes, alerts, definition lists; code coloured by
  Highlights' WebAssembly lexers through wasmtime; front matter hidden;
  HTML blocks read as markdown.
- **Figures.** Images beside the file inline through Kitty, iTerm2, or
  Sixel, "whichever the terminal answers to", falling back to coloured
  half blocks; a figure capped at a few rows so it "never fills several
  screens"; mermaid fences drawn "in the terminal's colors, with Pele
  run under bun or node; without either, or for a diagram Pele can't
  read, the block shows as code". SVG through resvg.
- **Rex.** `leaf rex install` writes `plugins/leaf/init.lua` beside
  Rex's config and a marked `-- >>> leaf >>>` block in `init.lua` that
  `dofile`s it with the binary's path, runs `rex config check` (reverting
  on failure) then `rex config reload`, and refreshes itself on every
  start so a `cargo install` updates the palette. The actions: Open
  Document… (a picker floated over the window), Review Diff, Pull
  Requests, Choose Agent…. A document "runs in a layer laid exactly over
  that pane, so nothing is typed into its shell", a busy pane gets it
  beside instead, and on a remote host the Lua runs locally while the
  pane calls go to the host, bootstrapping `cargo install` there.
- **The agent.** A selection or a diff comment goes to "an agent pane
  already open in Rex" found by `LEAF_AGENT_PANE` or by a pane running
  `claude`, `codex`, and the rest, typed and submitted; with no Rex, the
  prompt goes to the clipboard through OSC 52. `leaf comments --json`
  prints saved comments for scripts.

What it does not do is docz. `leaf open doc.md` shows the markdown as
written: the `<!--docz:…-->` markers are comments and vanish, so there
are no tabs by region, no phase cards or progress, no question cards, no
widgets (a `docz-flow` fence is a YAML code block), and no theme (it
"uses your terminal's own colors"). It is a reader of the source, where
docz-html is a renderer of the document model. The diff review with
comments to the agent also overlaps docz-review's job, which INV-0033
should weigh.

### Observation 3: mdp is the wrapping pattern, not the renderer

mdp is a server: `mdp serve README.md` renders through goldmark to HTML,
watches the file, and the plugin opens a browser with live reload and
scroll sync. The parts that carry over to a terminal viewer are the
wrappers, not the pipeline:

- `build.lua` is run by lazy.nvim on install and update, detects the
  platform, downloads the matching binary from GitHub releases into the
  plugin's `bin/`, and falls back to `go build`. docz already publishes
  darwin and linux binaries on both architectures through goreleaser, so
  the same script points at docz's releases with no release change.
- `lua/mdp/init.lua` is defaults, a job for the binary, autocommands for
  buffer changes, and the commands. For a terminal viewer there is no
  server and no browser; `:DoczView` opens the viewer in a `:terminal`
  split on the current buffer's file, or, when `REX_SESSION` is set,
  beside the editor's pane with `rex split`.

mdp's `pkg/parser` and `pkg/theme` are an HTML pipeline (goldmark with
`data-source-line` on every block, CSS themes) and do not apply.

### Observation 4: what docz has is the structure; it is behind an HTML writer

`pkg/export/html` is the page: `outline.go` cuts a document into tabs
(one per depth-0 region in order, the ToC dropped, an H2 outside a region
its own tab); `layout.go` is DESIGN-0025 §4's table, region kind to tab
and component; `facts.go` parses `impl`, `runbook`, `adr`, and
`investigation` lazily by region kind, never by type name; `questions.go`
writes a question card from `kinds.OpenQuestions`; `widgets.go` decodes
and validates the three widget blocks; `nodes.go` and `components.go`
(629 and 721 lines) write the HTML. Of all that, the package exports
`Render`, `Export`, the options, the report types, and nothing of the
structure: `Page` carries the HTML, not the outline.

A terminal renderer needs the outline, the layout, the facts, the
questions, and the validated widgets, with a different writer. Two
shapes:

- **A second writer in `pkg/export/html`**, a `Terminal` option on
  `Render` producing styled rows instead of HTML. Smallest change, wrong
  package name, and the terminal's concerns (width, reflow, selection
  offsets, image placement) have nothing in common with a page's.
- **A promotion**: the outline, layout, and facts into a package both
  writers consume, under `pkg/export/` so it may import the type packages
  (`layer_test.go`'s rule), with `html` and a new `pkg/export/term` (or
  `view`) each a writer over it. More files moved, and the right shape if
  a third writer ever comes (docz-site already has one in TypeScript).

`pkg/theme`'s tokens are hex per mode (`--color-fg-*`, `--color-st-*`,
`--color-bg-*`, `--color-ok`, …), so each theme is a truecolor palette
with no translation, and `contrast.test.ts`'s pairs hold as they are.
What does not carry over: fonts, radii, the page's own background (a
terminal has one; a theme that paints the page's background over the
pane would look like a card, which is a choice to make in step 6), and
`runtime.js` (tabs, the task filter, the mode toggle), which becomes
keys.

### Observation 5: glamour is the text renderer to measure against, not to ship

glamour (charmbracelet, v2.0.1, what glow uses) renders markdown to ANSI
from a JSON stylesheet over goldmark: headings, lists, tables, code
through chroma, with width. It has no region awareness, no images, and a
stylesheet that cannot express a phase card or a question card; a docz
document through it is its source with the markers gone, as in leaf.
Step 3 uses it as the floor: whatever it does well (wrapping, tables,
lists, code) the own writer must match, and whatever it cannot (cards,
callouts with a label and a body colour, a progress line) is the reason
for the own writer. Writing over the goldmark AST is what
`pkg/export/html` already does, so the own writer is the same walk again.

### Observation 6: mermaid is the one piece with no Go renderer

Mermaid is a browser library; docz vendors `mermaid.min.js` for pages and
docz-site draws it in the browser. In a terminal the diagram has to
become an image. leaf's answer is Pele (Obsidian's mermaid-compatible
renderer with no DOM) under bun or node, then SVG to PNG with resvg; with
neither runtime the fence shows as code. For docz the choices are the
same: Pele under bun or node (docz-review is Bun already), the vendored
mermaid under a headless DOM, or nothing. Go has SVG rasterisers (resvg
through wasm, tdewolff/canvas) for the second half. Whatever step 4
finds, mermaid stays optional and the viewer works without it; a
`docz view` that needed a JS runtime to show an ADR would be wrong.

> **Amended 2026-11-18.** Mermaid needs a renderer, not a DOM, and
> two run with no browser: Pele (Obsidian's, MIT, every Mermaid 12.1
> diagram type) and `mermaid-rs-renderer` (Rust). INV-0035 spiked
> Pele inside QuickJS compiled to wasm under wazero in Go, and
> natively in a compiled Bun binary, with resvg as wasm for the PNG
> in both: no cgo, no runtime on the machine, loading paid once and
> a render in milliseconds. `docz view` therefore draws diagrams
> itself (DESIGN-0027 question 5), and "optional behind a JS runtime"
> in the Hypothesis and Conclusion no longer applies.

The widgets are the opposite case: `widgets.go` already validates a
block into a Go value, and a flow, a stepper, and a scorecard are a row
of boxes, a numbered list with a state, and a table, which text draws
well. An image is not needed for them.

### Observation 7: Rex as a plugin host is a Lua file and a layer

leaf's installer is the whole pattern, and docz would copy it: a
`plugins/docz/init.lua` beside Rex's config, a marked block in
`init.lua` that loads it with the binary's path, `rex config check`
before `rex config reload`, a version hash so the next start refreshes
it. The actions are `rex.action` entries in the command palette; the
viewer opens with `session.new_layer` over the pane's bounds, or beside
with `session.new_split` when the pane is busy; on a remote host the
action's pane calls go to the host and a bootstrap installs the binary
there (leaf runs `cargo install`; docz would fetch its release tarball).
INV-0033 Observation 8 said the same for docz-review: nothing is
installed into Rex but a Lua file.

### Observation 8: the wrappers are small; the renderer is the work

Counting leaf: the reader and renderer are `viewer.rs` and `render.rs`,
215 KB of the 1 MB of source; the Rex plugin is `plugin.rs` 14 KB plus
15 KB of Lua. mdp's plugin is one Lua file and `build.lua`. For docz the
proportion holds: the Neovim plugin and the Rex action are each a few
hundred lines of Lua, the release pipeline exists, and the renderer, the
promotion of the structure out of `pkg/export/html`, the theme mapping,
and the figures are the work, in that order.

### Observation 9: what is not yet measured

Steps 2–8 have not run. In particular: the page as one Kitty image, its
size and CPU in Rex and whether anything in it can be clicked; what
glamour cannot express; how much of `pkg/export/html` the terminal writer
needs promoted; which mermaid route costs what; the themes against a
terminal's background; and both wrappers over a remote host.
<!--docz:findings:end-->

<!--docz:conclusion:start-->
## Conclusion

**Answer:** Yes, provisionally, as `docz view`: a terminal writer over
the structure `pkg/export/html` already computes, prose as styled text
in `pkg/theme`'s colours, figures as Kitty images where the terminal
answers and text where it does not, mermaid optional behind a JS
runtime, wrapped by a Neovim plugin on mdp's `build.lua` pattern and a
`docz rex install` on leaf's.

Part 1 is text with pictures: the panes in the screenshots are pictures,
Rex draws them well, and a still figure is cheap, but a page as a picture
cannot be read as a document and costs a frame per scroll. Part 2 is a
promotion before a renderer: the outline, layout, facts, questions, and
widgets exist in Go and are unexported, and the theme maps to truecolor
with no translation. Part 3 is two small Lua wrappers over a binary docz
already releases.

The conclusion closes when steps 2–4 have run and the picture's cost,
the writer's size, and the mermaid route are measured rather than read.
<!--docz:conclusion:end-->

<!--docz:recommendation:start-->
## Recommendation

1. Run spike B first (step 3): glamour over one document of each
   built-in, then the region-aware writer over `pkg/export/html`'s walk.
   It decides whether the writer is days or weeks and how much to
   promote, and it needs nothing from Rex or Neovim.
2. Run spike A (step 2) once, in half a day, to record the number and
   decide whether a picture preview is worth keeping at all.
3. Leave mermaid (step 4) last and ship without it if it costs a
   runtime; the fence as code is leaf's answer and a fine first one.
4. Then write a DESIGN for `docz view`: the promotion out of
   `pkg/export/html` (which needs an ADR if the layer rules change; they
   should not, since both stay under `pkg/export/`), the writer, the
   theme mapping, the keys that replace `runtime.js`, the Neovim plugin
   (and whether it is `docz.nvim`, since lazy.nvim clones the repository
   it names), and `docz rex install`.
5. Weigh leaf before building the reader parts: a selection that
   survives reflow, images and the protocol fallbacks, the Rex installer,
   and the agent pane are all solved there under MIT, in Rust. docz's
   value is the document model, not the reader; the smallest `docz view`
   is the structure in text and nothing leaf already does better, and
   the largest is a second leaf. Decide which before the DESIGN.
6. Record what leaf's diff comments to an agent pane mean for docz-review
   in INV-0033.

> **Amended 2026-11-12.** Recommendation 5 is decided: leaf is prior art
> only, and nothing in docz depends on it. The reader is docz's own,
> `docz view`, over docz-html's structure, and it is also the base of
> docz-review-rex, the Rex client of docz-review. Recommendation 4's
> DESIGN is
> [DESIGN-0027](../design/0027-docz-view-and-docz-review-rex-a-terminal-renderer-for-docz.md);
> the Neovim plugin is outside it, a later piece.

<!--docz:recommendation:end-->

<!--docz:references:start-->
## References

- [DESIGN-0025](../design/0025-docz-html-standalone-html-pages-from-docz-documents-and-one.md): docz-html, §3 the theme, §4 layout by region kind
- [IMPL-0028](../impl/0028-docz-html-pkgtheme-the-skill-docz-export-html-and-the-review.md): docz-html's implementation
- [IMPL-0030](../impl/0030-one-theme-system-across-docz-html-docz-review-and-docz-site.md): the themes, the widgets, and the site
- [DESIGN-0026](../design/0026-docz-sites-shell-in-repo-guardians-look-and-the-readers.md): docz-site's shell and the reader's fragment
- [INV-0033](0033-docz-review-inside-a-rex-floating-pane-instead-of-a-browser.md): docz-review inside a Rex floating pane, Observations 1–3 and 8
- [`pkg/export/html`](../../pkg/export/html/): `outline.go`, `layout.go`, `facts.go`, `questions.go`, `widgets.go`, `nodes.go`, `components.go`; [`pkg/theme`](../../pkg/theme/)
- [mdp](https://github.com/donaldgifford/mdp): `lua/mdp/init.lua`, `build.lua`, `pkg/parser`, `pkg/theme`
- [leaf](https://github.com/DeepanshuMishraa/leaf): `src/render.rs`, `src/kitty.rs`, `src/markdown.rs`, `src/mermaid.rs`, `src/plugin.rs`, `lua/leaf/init.lua`, the README's Rex section
- [glamour](https://github.com/charmbracelet/glamour), [glow](https://github.com/charmbracelet/glow), [rasterm](https://github.com/BourgeoisBear/rasterm), [kittyimg](https://github.com/dolmen-go/kittyimg)
- [Pele](https://github.com/obsidianmd/pele), the mermaid renderer leaf runs under bun or node
- [Kitty graphics protocol](https://sw.kovidgoyal.net/kitty/graphics-protocol/)
- Rex changelog: [Mac preview.19](https://discord.com/channels/1546797889009881158/1547349572299395172/1562927111927234657) (full Kitty graphics, remote translation),
  [Linux server preview.5](https://discord.com/channels/1546797889009881158/1547349572299395172/1562932417516281978),
  [Mac preview.25](https://discord.com/channels/1546797889009881158/1547349572299395172/1566944303526453400) (large images faster),
  [Linux server preview.13](https://discord.com/channels/1546797889009881158/1547349572299395172/1567297176072224819),
  [server preview.15](https://discord.com/channels/1546797889009881158/1547349572299395172/1569850825394753642) (pixel size at start)
- Rex feedback: [Kitty graphics: inline images of 10 MB or more are silently dropped](https://discord.com/channels/1546797889009881158/1562465271518859266) (deferred),
  [kitty image protocol size limit](https://discord.com/channels/1546797889009881158/1562496369539678279) (deferred, the per-block store),
  [Kitty graphics: high CPU and slow scrolling with large animation frames](https://discord.com/channels/1546797889009881158/1567294780377931869) (deferred)
- [Ladybird PR 14186](https://github.com/LadybirdBrowser/ladybird/pull/14186), the terminal mode INV-0033 reads

<!--docz:references:end-->
