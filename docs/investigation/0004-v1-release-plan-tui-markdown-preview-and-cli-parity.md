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

mdp's server lives at `internal/server` in donaldgifford/mdp, which
means it cannot be imported from docz today. Three options:

1. **Shell out** — `exec.Command("mdp", "serve", path)`. Lowest
   friction. Requires mdp on PATH. Detected at runtime; the
   preview-in-browser TUI action is hidden when mdp is absent.
   Trade-off: a separate process, weak lifecycle coupling, and the
   user has to install mdp themselves.
2. **Vendor the server** — copy `internal/server/`, `internal/parser/`,
   `internal/theme/`, `internal/watcher/` into docz under
   `internal/preview/`. Heaviest option; couples docz to mdp's release
   cadence; would need to be re-synced manually.
3. **Expose a public API in mdp** — factor the relevant pieces of
   `internal/server` (and likely `internal/parser` + `internal/theme`)
   into a `pkg/` package in a new mdp release, then import it from
   docz. Cleanest long-term answer and the **preferred path**, but it
   needs design work on mdp's side first: deciding the API shape,
   teasing apart the Neovim-plugin couplings from the
   library-consumable bits, and shipping a coordinated mdp release.

**Status:** open. The preferred path is option 3; the prerequisite is
an mdp-side investigation that designs the public package and ships
it. Until that lands, docz's preview integration is unimplemented and
IMPL-0010 owns picking the v1 approach once we know mdp's timeline.
If the mdp work is on the v1 critical path, IMPL-0010 lands the
library import directly; if not, IMPL-0010 lands option 1 (shell out)
as a placeholder and IMPL-0011+ migrates to the library when ready.

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
mature import-only library, and mdp is our own project so the
shell-out plus future-API path is fully under our control.

The two leverage points are:

1. **Adopt lstk's pattern verbatim.** A `cmd/root.go` that dispatches
   to TUI on a bare invocation, an `internal/tui/` with one
   `tea.Program` hosting many screens, and `internal/ui/components/`
   for reusable widgets.
2. **Land the TUI on top of clean ground.** Don't pile a TUI onto the
   current `cmd/` package with its global `appCfg`. IMPL-0009 (Runner
   pattern, output injection, slog) is a prerequisite.

The one genuinely open item is the mdp integration shape — the
preferred path imports mdp packages directly, but that requires
mdp-side work to expose them. IMPL-0010 picks the v1 approach (import
or shell out) once that mdp investigation lands.

## Recommendation

### Phasing into IMPL docs

The path to v1 is six implementation waves. Three exist; three are new:

| IMPL | Title | Status | Why for v1 |
|------|-------|--------|------------|
| IMPL-0006 | Correctness and Duplication Cleanup | Drafted | Defaults drift + INV-0003 semantics fix |
| IMPL-0007 | Eliminate Redundant File Reads and Heading Parses | Drafted | Update performance for big repos |
| IMPL-0008 | Move Stranded Business Logic Into Internal Packages | Drafted | Stable seams the TUI screens will call |
| IMPL-0009 | Runner Pattern and DocType Registry Refactor | Drafted | Output routing for the TUI, no globals |
| **IMPL-0010** | **mdp Preview Integration** | NEW | Browser preview; integration approach (library import vs shell out) decided by the doc itself, pending the mdp-side packaging investigation |
| **IMPL-0011** | **Bubble Tea v2 TUI Layer** | NEW | The actual v1 user-facing change |

The choice to slot IMPL-0010 before IMPL-0011 is deliberate: getting
the preview pipeline working as a `docz preview <path>` CLI verb
first lets us validate it without dragging the TUI into the same PR.
The TUI then wires a key binding to the existing preview verb. The
exact integration shape (library import vs subprocess) is decided
inside IMPL-0010 once the mdp-side packaging investigation lands.

A separate **IMPL-0012: v1 release engineering** wraps it: changelog,
goreleaser polish, semver bump to v1.0.0, the v1 README rewrite, and
the migration notes for any user who built scripts against v0
behavior.

### Out of scope for v1

- Inline editing inside the TUI (`textarea.Model`). Open the file in
  `$EDITOR` instead; lstk-style.
- A `docz tui` explicit subcommand. We use bare-invocation dispatch
  exclusively; `--non-interactive` is the escape hatch.
- Vendoring mdp's server. Migrating mdp to a public API is on the
  table for v1 if the mdp-side investigation lands in time; tracked
  inside IMPL-0010 rather than deferred wholesale.
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
3. **mdp dependency:** **open — needs investigation.** The
   preferred path is to import mdp packages directly so docz links
   the renderer in-process. That requires mdp-side work to factor
   `internal/server` (and likely `internal/parser` + `internal/theme`)
   into a public `pkg/` package. A separate mdp-side investigation
   designs that API and ships the release. Filing that work is part
   of IMPL-0010; IMPL-0010 itself decides which integration shape
   ships in v1 based on the mdp timeline (library import preferred;
   shell out as the fallback).
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
