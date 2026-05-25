---
id: INV-0004
title: "v1 Release Plan: TUI, Markdown Preview, and CLI Parity"
status: Open
author: Donald Gifford
created: 2026-05-22
---
<!-- markdownlint-disable-file MD025 MD041 -->

# INV 0004: v1 Release Plan: TUI, Markdown Preview, and CLI Parity

**Status:** Open
**Author:** Donald Gifford
**Date:** 2026-05-22

<!--toc:start-->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
  - [Bubble Tea v2 — what changed and what we need](#bubble-tea-v2--what-changed-and-what-we-need)
  - [Bubbles v2 components catalog vs docz workflows](#bubbles-v2-components-catalog-vs-docz-workflows)
  - [Glow / Glamour for in-TUI markdown rendering](#glow--glamour-for-in-tui-markdown-rendering)
  - [mdp integration approach](#mdp-integration-approach)
  - [lstk reference architecture](#lstk-reference-architecture)
  - [Mapping docz workflows to TUI screens](#mapping-docz-workflows-to-tui-screens)
  - [Proposed package layout](#proposed-package-layout)
  - [TUI vs CLI mode selection](#tui-vs-cli-mode-selection)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
  - [Prerequisites (hard gate)](#prerequisites-hard-gate)
  - [Phasing into IMPL docs](#phasing-into-impl-docs)
  - [Out of scope for v1](#out-of-scope-for-v1)
  - [Decisions](#decisions)
- [References](#references)
<!--toc:end-->

## Question

What does v1 of docz need to ship — and in what order — to give us a CLI
that also opens a Bubble Tea v2 TUI when run interactively, with in-TUI
markdown preview (Glow/Glamour) and a "press a key to open this in the
browser" preview powered by mdp? Specifically:

1. Is Bubble Tea v2 + Bubbles v2 the right TUI foundation today, given
   their stability and breaking-change story?
2. How do CLI subcommands (Cobra) and the TUI coexist — is the model "no
   args → TUI; args → CLI", or do we need a `docz tui` subcommand?
3. Should mdp be linked as a library, vendored, or shelled out from docz?
4. What's the minimum TUI surface that earns the v1 label — and what
   intentionally waits for v1.x?

## Hypothesis

- Bubble Tea v2 is stable enough (v2.0.6, April 2026) and lstk is a
  proven production reference, so the framework choice is low-risk.
- The lstk pattern — TUI when interactive, plain CLI when piped or when
  `--non-interactive` is passed — is the right default. A bare `docz`
  opens the TUI; every existing subcommand keeps working unchanged.
- Glamour (Glow's underlying renderer) is the right in-TUI viewer: it's
  the library Glow ships and is import-friendly. The Glow binary itself
  is too heavyweight to embed.
- mdp ships its server in `internal/server`, which is not importable
  today. The preferred long-term path is to factor a shared package
  out of mdp that docz can link against. That work is mdp-side and
  needs its own investigation; v1's integration approach is left
  open here as a result.
- v1 is realistic in **4 IMPL waves**: finish IMPL-0006/0007/0008/0009
  to land the architectural cleanup, then layer the TUI in IMPL-0010
  (preview integration) and IMPL-0011 (full TUI surface).

## Context

The user wants docz to feel modern: keep the scriptable CLI for CI and
power-use, but pop into a real TUI when you launch it from a terminal.
The TUI should be able to do everything the CLI does (create docs, list,
filter, update indexes, run wiki nav generation) and add things the CLI
can't easily express (preview a doc inline, jump to it in a browser via
mdp, filter as you type).

**Triggered by:** Direct user request, 2026-05-22, with named
dependencies (Bubble Tea v2, Bubbles, Glow) and reference projects
(localstack/lstk, donaldgifford/mdp).

The existing IMPL-0009 (Runner Pattern and DocType Registry Refactor)
already calls for evaluating `charmbracelet/fang` and Bubble Tea v2 in
its Phase 1 DESIGN doc. This investigation supersedes that evaluation
with a v1-scoped roadmap.

## Approach

1. Confirm current versions of every named dependency via `gh repo
   view`.
2. Walk lstk's repo (`internal/ui/`, `internal/ui/components/`,
   `cmd/root.go`) to extract the proven CLI+TUI hybrid pattern.
3. Walk mdp's repo (`internal/server/`, `internal/cli/`) to determine
   whether the server can be imported, vendored, or only invoked as a
   subprocess.
4. Map each existing docz CLI command (`create`, `list`, `update`,
   `template`, `wiki`, `config`) to either a TUI screen, a TUI action,
   or a CLI-only operation.
5. Sketch the package layout and the CLI/TUI handoff.
6. Phase the work into IMPL docs.

## Environment

| Component | Version / Value | Notes |
|-----------|-----------------|-------|
| `charmbracelet/bubbletea` | v2.0.6 (2026-04-16) | v2 line, breaking changes from v1 — see Findings |
| `charmbracelet/bubbles` | v2.1.0 (2026-03-26) | Tracks Bubble Tea v2 |
| `charmbracelet/glow` | v2.1.2 (2026-04-09) | We consume `glamour` (Glow's renderer), not the binary |
| `donaldgifford/mdp` | v0.1.8 (2026-04-03) | Server is in `internal/server` — not a public API |
| `localstack/lstk` | v0.9.0 (2026-05-21) | Reference architecture |
| docz baseline | post-IMPL-0005 merge | INV-0003 outstanding |
| Go runtime | 1.25.7 | |

## Findings

### Bubble Tea v2 — what changed and what we need

Bubble Tea v2 is a stable line as of April 2026. The breakage from v1 is
manageable and well-documented: imports move from
`github.com/charmbracelet/bubbletea` (still works but resolves to v2 via
the major-version suffix), the `tea.KeyMsg.Type` switch shifts to
explicit `key.Matches`, and the renderer is more strict about commands
returning `tea.Cmd` not `tea.Msg`. We never adopted v1 in docz, so we
pay the migration cost zero times.

The v2 API still centers on the Elm-architecture triple: `Init() Cmd`,
`Update(Msg) (Model, Cmd)`, `View() string`. Compared to v1, v2 added
first-class cell-based input handling, mouse support, and tighter
integration with `lipgloss` for theming.

**Implication:** picking v2 today is the correct call. There's no
"should we wait for v3" risk on a v2.0.x line that's already shipping.

### Bubbles v2 components catalog vs docz workflows

The components docz will actually use:

- `list.Model` — for `docz list` filtered by type/status; for the
  per-type browse screen; for the "pick a doc to view/edit" picker.
- `textinput.Model` — for the create flow (title input).
- `textarea.Model` — optional, if we ever want inline edit; **out of
  scope for v1**.
- `viewport.Model` — to host Glamour's rendered markdown for the
  in-TUI preview.
- `spinner.Model` — for the long-running ops (wiki update on big
  repos, mdp launch).
- `help.Model` — keybindings hint at the bottom of every screen.
- `key.Binding` — for declarative key maps.

We do not need `table.Model` — `list.Model` with custom delegate
covers the table-ish "rows of docs" view at lower complexity.

### Glow / Glamour for in-TUI markdown rendering

Glow itself is a TUI binary. Its underlying renderer is
`github.com/charmbracelet/glamour`, which Glow exports for import. We
want the renderer, not Glow. Glamour:

- Takes markdown bytes + a style (`dark`, `light`, `notty`, or a custom
  JSON) and returns ANSI-styled text.
- Pairs naturally with `viewport.Model` from Bubbles: render once,
  stuff into the viewport, let the user scroll.
- Has known weak spots: long fenced code blocks wrap awkwardly, tables
  beyond a few columns are unreadable below ~80 columns. Acceptable
  for v1; documented as a known limitation.

### mdp integration approach

Originally there were three options here (shell out, vendor, public
API) because mdp's server lived under `internal/server` and was not
importable. That blocker is now gone: mdp `main` has factored the
library-consumable pieces into the public `pkg/` tree —
`pkg/livereload`, `pkg/parser`, and `pkg/theme` — at
<https://github.com/donaldgifford/mdp/tree/main/pkg>. docz can link
those packages directly.

**Decision:** option 3 (library import) is the v1 path. docz adds
`github.com/donaldgifford/mdp` as a go.mod dependency and IMPL-0010
builds `internal/preview/` on top of `mdp/pkg/parser` and
`mdp/pkg/theme` (for rendering) and `mdp/pkg/livereload` (for the
browser preview lifecycle). No subprocess, no PATH dependency, no
vendored copy.

The shell-out and vendor options are retained here only as historical
context — they are no longer on the table for v1.

**Status:** resolved. Non-blocking for v1 — mdp's `pkg/` is available
on `main` today; no coordinated mdp release is required before
IMPL-0010 starts. The remaining IMPL-0010 work is docz-side: choosing
which `pkg/` symbols to depend on, designing `internal/preview/`'s
public surface, and wiring the TUI `p` key to it.

### lstk reference architecture

lstk's pattern is the one to copy:

- `cmd/` holds Cobra commands as today (`root.go`, `start.go`,
  `status.go`, etc.). Root's `RunE` decides whether to open the TUI or
  emit plain output based on terminal detection and the
  `--non-interactive` flag.
- `internal/ui/app.go` defines the top-level Bubble Tea model.
- `internal/ui/run.go` has the program entry — `tea.NewProgram(app)`
  and a `Run(ctx, RunOptions)` that wraps everything.
- `internal/ui/run_<command>.go` — one file per command-shaped UI flow
  (`run_login.go`, `run_status.go`). For docz we'd map this to
  `run_list.go`, `run_create.go`, `run_view.go`, etc.
- `internal/ui/components/` — reusable widgets (`header.go`,
  `spinner.go`, `input_prompt.go`, `error_display.go`,
  `message.go`). Each has a `_test.go`. docz copies this pattern.
- `internal/ui/styles/` and `internal/ui/wrap/` — theming and ANSI
  width-aware text helpers.
- `internal/terminal/` — terminal capability detection (TTY, color,
  size).

Key insight from lstk: **the TUI is *one* tea.Program that hosts
many "screens" via a router**, not a fresh program per command. The
top-level model holds a current-screen variant and dispatches
`Update` to it. We adopt this pattern verbatim.

### Mapping docz workflows to TUI screens

| docz operation | CLI today | TUI v1 plan |
|----------------|-----------|-------------|
| `docz list` | Tabular stdout | List screen with type/status filter, fuzzy search |
| `docz create <type> "<title>"` | Prompts via flags | Create screen: type picker → title input → preview → confirm |
| `docz update [type]` | Stdout summary | Update screen with progress spinner and per-type report |
| `docz template show <type>` | Stdout template | View screen using Glamour |
| `docz template export/override` | CLI only | CLI only for v1 |
| `docz wiki init/update` | Stdout summary | Wiki screen with the nav tree visible during build |
| `docz config` | YAML to stdout | Config screen, read-only YAML view |
| `docz version` | One line | About modal |
| Preview a doc | Not in CLI | View screen: Glamour viewport, `p` key launches mdp |
| Browse all docs across types | Not in CLI | "All docs" screen sorted by ID |

### Proposed package layout

```text
cmd/
  root.go              # Cobra root; RunE decides TUI vs CLI
  create.go list.go update.go template.go wiki.go config.go version.go
internal/
  config/  document/  index/  template/  toc/  wiki/   # existing
  preview/             # NEW. Owns the mdp integration. Concrete
                       # shape (library import vs subprocess) is
                       # decided in IMPL-0010.
  terminal/            # NEW. TTY/size/color detection — small,
                       # ~50 LOC over x/term.
  tui/                 # NEW. Bubble Tea program + screens + components.
    app.go             # top-level model + screen router
    run.go             # tea.NewProgram entry
    screen_list.go screen_create.go screen_view.go
    screen_update.go screen_wiki.go screen_config.go
    components/        # header, footer, help, spinner, error
    styles/            # lipgloss styles
    keys/              # key bindings
```

### TUI vs CLI mode selection

The decision tree at root command's `RunE`:

1. Subcommand present? → Cobra dispatches normally.
2. `--non-interactive` flag or `DOCZ_NON_INTERACTIVE=1`? → Print short
   usage and exit 0.
3. stdout is not a TTY (piped / redirected)? → Same as above.
4. stdin is not a TTY? → Same as above.
5. Otherwise → call `tui.Run(ctx)` and exit on the model's quit.

Match lstk exactly. Document the env var alongside the flag in
`--help`.

## Conclusion

**Answer:** Yes, v1 is well-shaped and reachable. The TUI work is real
but bounded — about three weeks of focused implementation on top of the
already-planned IMPL-0006 through IMPL-0009 cleanup. The dependency
risk is low: Bubble Tea v2 is stable, Bubbles tracks it, Glamour is a
mature import-only library, and mdp now exposes its renderer + theme +
live-reload under `pkg/` (`mdp/pkg/parser`, `pkg/theme`,
`pkg/livereload`) so docz can link it in-process. The integration is
fully under our control end-to-end.

The two leverage points are:

1. **Adopt lstk's pattern verbatim.** A `cmd/root.go` that dispatches
   to TUI on a bare invocation, an `internal/tui/` with one
   `tea.Program` hosting many screens, and `internal/ui/components/`
   for reusable widgets.
2. **Land the TUI on top of clean ground.** Don't pile a TUI onto the
   current `cmd/` package with its global `appCfg`. IMPL-0009 (Runner
   pattern, output injection, slog) is a prerequisite.

The previously-open item was the mdp integration shape; it is now
resolved. mdp's `pkg/` namespace exists on `main` today, so IMPL-0010
lands the library-import path directly — no shell-out fallback, no
mdp-side prerequisite, no coordinated release.

## Recommendation

### Prerequisites (hard gate)

**No design or implementation work for v1 starts until IMPL-0006,
IMPL-0007, IMPL-0008, and IMPL-0009 are all merged to `main` with
their status set to `Completed`.**

Reasons:

- IMPL-0006 settles the config semantics (defaults drift, INV-0003's
  config-listed-types fix, `EnabledTypes` helper). The TUI surfaces
  the type list directly — building on the current behavior would
  bake the bug into the UI.
- IMPL-0007 cuts the file-read/parse cost in `docz update`. The TUI
  exposes update as an interactive screen with progress feedback; a
  slow update path is much more visible there than at the CLI.
- IMPL-0008 moves business logic out of `cmd/` into stable internal
  packages. TUI screens need to call those packages without
  re-importing `cmd/` or duplicating logic.
- IMPL-0009 introduces the Runner pattern, `io.Writer` injection,
  and `log/slog`. Without it, the TUI cannot capture command output
  cleanly — it would have to redirect `os.Stdout` for every screen.

Concretely, the gate is: `IMPL-0006 ∧ IMPL-0007 ∧ IMPL-0008 ∧
IMPL-0009` all show `status: Completed` in their frontmatter and
have a merged PR linked from their Phase-N PR task. Until that is
true, the DESIGN doc for IMPL-0011 (TUI) is not opened and IMPL-0010
(mdp preview) does not begin coding.

mdp is no longer part of the gate: its `pkg/` surface is published on
`main` (`pkg/livereload`, `pkg/parser`, `pkg/theme`), so IMPL-0010 can
proceed via library import as soon as IMPL-0006..0009 are merged.
There is no mdp-side prerequisite work left to track.

### Phasing into IMPL docs

The path to v1 is six implementation waves. Three exist; three are new:

| IMPL | Title | Status | Why for v1 |
|------|-------|--------|------------|
| IMPL-0006 | Correctness and Duplication Cleanup | Drafted | Defaults drift + INV-0003 semantics fix |
| IMPL-0007 | Eliminate Redundant File Reads and Heading Parses | Drafted | Update performance for big repos |
| IMPL-0008 | Move Stranded Business Logic Into Internal Packages | Drafted | Stable seams the TUI screens will call |
| IMPL-0009 | Runner Pattern and DocType Registry Refactor | Drafted | Output routing for the TUI, no globals |
| **IMPL-0010** | **mdp Preview Integration** | NEW | Browser preview via library import of `mdp/pkg/{parser,theme,livereload}` (no shell-out, no vendoring); now non-blocking — proceeds as soon as IMPL-0006..0009 are merged |
| **IMPL-0011** | **Bubble Tea v2 TUI Layer** | NEW | The actual v1 user-facing change |

The choice to slot IMPL-0010 before IMPL-0011 is deliberate: getting
the preview pipeline working as a `docz preview <path>` CLI verb
first lets us validate it without dragging the TUI into the same PR.
The TUI then wires a key binding to the existing preview verb. The
integration shape is decided: IMPL-0010 imports `mdp/pkg/`
(parser/theme/livereload) directly.

A separate **IMPL-0012: v1 release engineering** wraps it: changelog,
goreleaser polish, semver bump to v1.0.0, the v1 README rewrite, and
the migration notes for any user who built scripts against v0
behavior.

### Out of scope for v1

- Inline editing inside the TUI (`textarea.Model`). Open the file in
  `$EDITOR` instead; lstk-style.
- A `docz tui` explicit subcommand. We use bare-invocation dispatch
  exclusively; `--non-interactive` is the escape hatch.
- Vendoring mdp's server. mdp now exposes `pkg/livereload`,
  `pkg/parser`, and `pkg/theme` on `main`, so IMPL-0010 takes a direct
  library import — no vendor copy, no shell-out fallback.
- Multi-pane layouts. v1 is one screen at a time.
- Custom Glamour themes. Ship with `dark` / `light` / `notty`; let
  v1.x add user themes if asked.

### Decisions

Resolved during design review on 2026-05-23.

1. **Cobra + Bubble Tea coexistence:** keep stock Cobra. No fang.
   lstk's pattern translates directly; fang offers no v1-blocking
   capability we'd miss.
2. **TUI keybindings:** the proposed set is accepted (`q` quit,
   `?` help, `/` filter, `enter` select, `p` preview-in-browser).
   No conflict surfaced; revisit if a screen needs a sixth chord.
3. **mdp dependency:** **resolved — library import.** mdp's `pkg/`
   namespace exists on `main` (`pkg/livereload`, `pkg/parser`,
   `pkg/theme`) at
   <https://github.com/donaldgifford/mdp/tree/main/pkg>. docz takes a
   direct go.mod dependency on `github.com/donaldgifford/mdp` and
   builds `internal/preview/` on top of those packages. No subprocess,
   no PATH dependency, no vendored copy. Non-blocking for v1 — the
   integration begins as soon as IMPL-0006..0009 are merged.
4. **State persistence:** v1 starts fresh every invocation. No
   `$XDG_STATE_HOME/docz/` writes. Deferred entirely; revisit only
   if users ask for sticky filters.
5. **Telemetry:** none. docz v1 ships with zero phone-home.
6. **Windows support:** **out of scope for v1.** Document that the
   TUI is Unix-only (macOS, Linux, *BSD). Re-evaluate when there's
   user demand; the Bubble Tea side already works on Windows, so the
   work would be confined to the mdp shell-out path and `$EDITOR`
   conventions.

## References

- Bubble Tea v2 — https://github.com/charmbracelet/bubbletea v2.0.6
- Bubbles v2 — https://github.com/charmbracelet/bubbles v2.1.0
- Glamour — https://github.com/charmbracelet/glamour (Glow's renderer)
- Glow — https://github.com/charmbracelet/glow v2.1.2
- mdp — https://github.com/donaldgifford/mdp v0.1.8
- localstack/lstk — https://github.com/localstack/lstk v0.9.0
- INV-0002 — Architectural review (sets up the cleanup waves)
- INV-0003 — Config-listed types semantics (must resolve before TUI
  surfaces the type list)
- IMPL-0009 — Runner pattern, DocType registry; Phase 1 DESIGN doc
  evaluated fang/Bubble Tea — superseded by this investigation
