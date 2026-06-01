---
id: IMPL-0010
title: "mdp Preview Integration"
status: Draft
author: Donald Gifford
created: 2026-05-29
---
<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL 0010: mdp Preview Integration

**Status:** Draft
**Author:** Donald Gifford
**Date:** 2026-05-29

<!--toc:start-->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: Foundation — go.mod, package skeleton, Renderer](#phase-1-foundation--gomod-package-skeleton-renderer)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2: HTTP server + livereload injection](#phase-2-http-server--livereload-injection)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 3: File watcher + broadcast loop](#phase-3-file-watcher--broadcast-loop)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 4: Browser opener (Unix)](#phase-4-browser-opener-unix)
    - [Tasks](#tasks-3)
    - [Success Criteria](#success-criteria-3)
  - [Phase 5: docz preview Cobra command](#phase-5-docz-preview-cobra-command)
    - [Tasks](#tasks-4)
    - [Success Criteria](#success-criteria-4)
  - [Phase 6: Verify and ship](#phase-6-verify-and-ship)
    - [Tasks](#tasks-5)
    - [Success Criteria](#success-criteria-5)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Decisions](#decisions)
- [Dependencies](#dependencies)
- [References](#references)
<!--toc:end-->

## Objective

Add a `docz preview <path>` CLI verb that renders a docz markdown
document to GitHub-styled HTML in the user's browser with live-reload
on save. The implementation links `github.com/donaldgifford/mdp` at
the `pkg/parser`, `pkg/theme`, and `pkg/livereload` level — no
subprocess, no PATH dependency, no vendored copy. The preview surface
is encapsulated behind a small `internal/preview/` API so that
IMPL-0011 (TUI) can wire the `p` key to it without re-importing mdp
or duplicating server logic.

**Implements:** INV-0004 — v1 Release Plan (Recommendation §Phasing,
Decision §3 "mdp dependency: library import").

## Scope

### In Scope

- Add `github.com/donaldgifford/mdp` (v0.1.8 or newer) as a go.mod
  dependency
- New `internal/preview/` package owning the renderer, HTTP server,
  livereload hub, file watcher, and browser launcher
- New `cmd/preview.go` Cobra command (`docz preview <path>`) with
  `--port`, `--theme`, `--no-watch`, `--no-open`, and `--no-livereload`
  flags
- Frontmatter-aware rendering: strip the leading YAML block before
  passing to `mdp/pkg/parser` so the preview shows just the prose
- Unix-only browser open (macOS `open`, Linux `xdg-open`, BSD
  `xdg-open`) — Windows is out of scope, matching the v1 TUI
- Unit tests for every exported function in `internal/preview/` and
  table-driven CLI tests through a constructed Runner

### Out of Scope

- TUI integration. IMPL-0011 wires the `p` key to the same
  `internal/preview/` surface; that work does not land here
- Custom theme authoring beyond what `mdp/pkg/theme` already exposes
  (no JSON theme files, no per-doc theme overrides)
- Multi-file preview (browse a directory). One file per invocation
- Persistent preview state (`$XDG_STATE_HOME/docz/preview/`) —
  deferred per INV-0004 §Decisions 4
- A `--print` flag that dumps raw HTML to stdout. Users who need that
  can run the mdp binary directly
- Windows support — INV-0004 §Decisions 6 marks the v1 TUI as
  Unix-only; Decision §D in this doc locks the preview verb to the
  same scope

## Implementation Phases

Each phase builds on the previous one. A phase is complete when every
task is checked off and the success criteria are met. Phases 1-5 each
produce a self-contained commit; Phase 6 is the ship gate.

---

### Phase 1: Foundation — go.mod, package skeleton, Renderer

Bring `mdp` into the module, sketch the `preview.Renderer` and its
options, and prove the parser+theme integration in isolation. No HTTP,
no file IO. This is the layer the rest of the package composes onto.

#### Tasks

- [ ] `go get github.com/donaldgifford/mdp@latest` and `go mod tidy`;
      record the resolved version in this doc's References section
- [ ] Create `internal/preview/preview.go` with the `Renderer` struct,
      `Options` struct, `NewRenderer(opts Options) *Renderer`,
      `(*Renderer).Render(md []byte) ([]byte, error)`, and a
      `(*Renderer).Theme() theme.Theme` accessor
- [ ] Implement frontmatter stripping in `internal/preview/frontmatter.go`
      (reuse `internal/document.LoadFrontmatter` where it fits;
      otherwise add a small `StripFrontmatter([]byte) []byte` helper
      and document why a separate copy was needed)
- [ ] Wire `mdp/pkg/parser.New(...)` with GFM, syntax highlighting,
      callouts, and math enabled; Mermaid stays client-side (matches
      mdp default) so we don't require `mmdc`
- [ ] Wire `mdp/pkg/theme.Resolve(name)` with `auto` as the default
      and validation against `theme.Names()` for the `--theme` flag
- [ ] Write `internal/preview/preview_test.go` with table-driven cases:
      empty input, frontmatter-only doc, plain markdown, GFM table,
      fenced code block, callout, math expression, invalid theme name
- [ ] Document the public API in a leading package comment that names
      the IMPL-0011 contract: `Renderer` and `Server` (Phase 2) are
      the seams the TUI will call into

#### Success Criteria

- `go build ./...` and `go vet ./...` clean
- `make lint` returns zero issues
- `go test ./internal/preview/...` green with every test calling
  `t.Parallel()`
- `Renderer.Render` produces HTML that contains the expected
  `data-source-line` attributes (mdp parser emits these by default)

---

### Phase 2: HTTP server + livereload injection

Stand up the `preview.Server` that serves the rendered HTML, the
theme CSS, the optional hljs vendor stylesheet, and the livereload
script. The hub is created but no events are broadcast yet — that's
Phase 3's job.

#### Tasks

- [ ] Add `Server` struct to `internal/preview/server.go` with fields
      for `*Renderer`, `*livereload.Hub`, source file path, and
      `*http.Server`
- [ ] Implement `NewServer(opts ServerOptions) (*Server, error)`
      where `ServerOptions` bundles `Port int`, `Renderer *Renderer`,
      `SourcePath string`, `EnableLiveReload bool`
- [ ] Routes: `GET /` returns the rendered HTML (wrapped in mdp
      livereload `WrapHandler` when live-reload is on); `GET /theme.css`
      serves the resolved theme CSS; `GET /hljs.css` serves the
      vendored hljs sheet when the theme defines one; `GET /ws` and
      `GET /events` are wired to `livereload.Hub` via its
      `HandleWebSocket` / `HandleSSE` methods
- [ ] HTML page template (small inline `text/template`): `<html>` →
      `<link rel="stylesheet" href="/theme.css">`, optional hljs link,
      `<body>` → `<main class="markdown-body">…rendered HTML…</main>`,
      Mermaid client script when the theme says so
- [ ] `(*Server).Start(ctx context.Context) error` listens on
      `127.0.0.1:Port` (zero = ephemeral), blocks until the context
      is cancelled, then calls `http.Server.Shutdown` with a 2s grace
- [ ] `(*Server).URL() string` returns the bound `http://127.0.0.1:<port>/`
      so the Cobra layer can pass it to the browser opener
- [ ] Server tests using `httptest.NewRecorder` (no real listener) for
      route correctness, plus a single `httptest.NewServer`-based
      integration test asserting that `/` returns 200 with the rendered
      body inside it

#### Success Criteria

- `curl http://127.0.0.1:<port>/` returns 200 with the rendered HTML
  wrapped in the theme shell
- `curl http://127.0.0.1:<port>/theme.css` returns 200 with the
  resolved theme CSS body
- Live-reload script is present in `/` output when `EnableLiveReload`
  is true and absent when it is false
- Server shuts down cleanly within 2s of context cancellation
- All Phase 2 tests call `t.Parallel()` and pass under
  `go test -race`

---

### Phase 3: File watcher + broadcast loop

Wire `fsnotify` so that saving the watched file re-renders the body
and broadcasts a payload over the livereload hub. Handle the editor
save patterns that trip up naïve watchers (atomic rename, in-place
truncate, the Vim swap dance).

#### Tasks

- [ ] Add `github.com/fsnotify/fsnotify` to go.mod
- [ ] Create `internal/preview/watcher.go` with
      `Watcher` struct holding `*fsnotify.Watcher`, source path,
      a `Render func([]byte) ([]byte, error)`, and a
      `Broadcast func([]byte)` callback
- [ ] `(*Watcher).Run(ctx context.Context) error` watches the parent
      directory (so atomic rename-into-place is observable) and
      filters events down to the source path. On Write / Create /
      Rename, re-reads the file, re-renders, broadcasts the new HTML
- [ ] Debounce events: collapse a burst of writes within 50ms into a
      single render. Make the window a constant; not a flag in v1
- [ ] If the source file is deleted (mv away with no replacement),
      log a warning via `slog` and keep the watcher running so a
      subsequent re-create resumes preview
- [ ] Hook watcher start/stop into `Server.Start` so the Cobra layer
      doesn't have to glue the two together
- [ ] Tests: use `t.TempDir()` + `os.WriteFile` to drive the watcher
      under `httptest`; assert that the broadcast callback fires
      exactly once per debounced burst and carries the re-rendered
      HTML. Also test the rename-into-place case (write to `tmp`,
      `os.Rename` over the target)

#### Success Criteria

- Saving the watched file in `$EDITOR` triggers a single livereload
  broadcast (no duplicate fires from editor save patterns)
- Deleting and re-creating the watched file resumes preview without
  crashing the server
- Watcher tests pass under `go test -race -count=3` with shuffle
- The `--no-watch` flag (added in Phase 5) is a no-op pathway: the
  server runs without ever starting the watcher

---

### Phase 4: Browser opener (Unix)

A tiny helper that opens the bound URL in the user's default browser
on macOS, Linux, and BSD. Windows is deliberately out of scope per
Decision §D so the preview verb's platform matrix matches IMPL-0011's
TUI scope.

#### Tasks

- [ ] Create `internal/preview/open.go` with the
      `OpenBrowser(ctx context.Context, url string) error` signature
      and a package-level `openCmd` function variable so tests can
      substitute a stub
- [ ] Build-tagged platform files (Decision §C, hand-rolled `os/exec`
      switch): `open_darwin.go` shells out to `open <url>`,
      `open_linux.go` and `open_freebsd.go`/`open_openbsd.go`/
      `open_netbsd.go` shell out to `xdg-open <url>`. Each path uses
      `exec.CommandContext` so Ctrl+C cancels a hung opener cleanly
      (same pattern as `cmd/git.go`)
- [ ] `open_other.go` with a build tag excluding the Unix targets
      above returns an "unsupported platform" error so the build
      stays green on Windows even though the verb won't run there
- [ ] Treat opener failure as non-fatal: log the URL via `slog.Warn`
      and keep the server running. The user can paste the URL
      themselves
- [ ] Inject the opener as a func field on `Server` so tests can
      substitute a stub that records the URL and returns nil. The
      Cobra layer's `--no-open` flag substitutes a no-op stub
- [ ] Tests: stub-driven assertions that the opener is called with
      the bound URL exactly once and that failure is logged but not
      returned as an error

#### Success Criteria

- `docz preview` opens the default browser on macOS at the bound
  URL without printing the URL twice or leaking the opener
  subprocess
- Opener failure (e.g. PATH missing `xdg-open`) is logged but does
  not stop the server
- The injected opener stub model lets every cmd test verify
  open-vs-no-open behavior without spawning a real browser

---

### Phase 5: `docz preview` Cobra command

The user-visible surface. A single new subcommand glues the
`internal/preview/` building blocks together and hands control to the
server's `Start` loop.

#### Tasks

- [ ] Create `cmd/preview.go` with `var previewCmd = &cobra.Command{...}`
      and `func (r *Runner) preview(ctx context.Context, args []string) error`
- [ ] Flags: `--port int` (default 0 = ephemeral), `--theme string`
      (default `auto`, validated against `theme.Names()`),
      `--no-watch bool`, `--no-open bool`, `--no-livereload bool`
- [ ] Argument: required positional `<path>`. Resolve relative paths
      against `r.RepoRoot` via `r.inRepo(path)`. Reject directories
      with a clear error
- [ ] Validate the path exists and has a `.md` extension; emit a
      clear error otherwise (no silent fallback)
- [ ] Build `preview.Options`, construct the `Server`, install the
      opener stub vs real opener based on `--no-open`, install a
      no-op watcher when `--no-watch` is set
- [ ] Honor `cmd.Context()` so Ctrl+C cancels the server cleanly
- [ ] Register `previewCmd` in `cmd/root.go` next to the other
      command registrations
- [ ] Tests in `cmd/preview_test.go`: every flag combination via a
      constructed `Runner` with `Out: &bytes.Buffer{}`. Use
      `httptest`-style server-stub injection where possible to avoid
      binding real listeners

#### Success Criteria

- `docz preview docs/rfc/0001-foo.md` starts a server on an
  ephemeral port, opens the default browser, prints the bound URL
  to stdout, and blocks until Ctrl+C
- `docz preview --port 8123 --theme github-dark --no-open
  --no-watch path.md` honors every flag
- Invalid `--theme` name returns a non-zero exit with the list of
  valid names in the error message
- `go test -race ./cmd/... -run TestRunPreview` green
- `--help` output documents every flag with one-line descriptions

---

### Phase 6: Verify and ship

#### Tasks

- [ ] Full `make ci` green
- [ ] Manual smoke test on macOS: preview an existing RFC, save it in
      `$EDITOR`, watch the browser reload. Capture a short note in
      this doc if any rough edges surface
- [ ] Manual smoke test on Linux if a local VM is available; mark as
      "deferred" otherwise
- [ ] Update CLAUDE.md architecture section to add
      `internal/preview/` (one-line summary + the
      `Renderer`/`Server`/`Watcher` seam names)
- [ ] Update `docs/investigation/0004-...md` to mark IMPL-0010 row as
      `Completed` in the v1 phasing table once this PR merges
- [ ] Flip this doc's frontmatter `status` to `Completed` after the
      PR merges to main
- [ ] Open the PR with the `dont-release` label (matches IMPL-0009
      convention; this is feature work but the v1 release engineering
      ships under IMPL-0012)

#### Success Criteria

- `make ci` green
- A new docz user can run `docz preview <path>` against any of the
  six built-in doc types and see the rendered output in a browser
- The `internal/preview/` package exposes exactly the seams
  IMPL-0011 needs (`NewRenderer`, `NewServer`, `Server.Start`,
  `Server.URL`) with godoc that names IMPL-0011 as the intended
  consumer

---

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `go.mod` / `go.sum` | Modify | Add `github.com/donaldgifford/mdp` and `github.com/fsnotify/fsnotify` |
| `internal/preview/preview.go` | Create | `Renderer` struct + options |
| `internal/preview/frontmatter.go` | Create | YAML frontmatter stripper |
| `internal/preview/server.go` | Create | HTTP server + routes + page template |
| `internal/preview/watcher.go` | Create | fsnotify wrapper + debounce |
| `internal/preview/open.go` | Create | Cross-platform browser opener |
| `internal/preview/*_test.go` | Create | Unit tests for every exported symbol |
| `cmd/preview.go` | Create | Cobra subcommand + Runner method |
| `cmd/preview_test.go` | Create | Flag/arg coverage via constructed Runner |
| `cmd/root.go` | Modify | Register `previewCmd` |
| `CLAUDE.md` | Modify | Add `internal/preview/` to the architecture section |
| `docs/investigation/0004-...md` | Modify | Flip IMPL-0010 row to `Completed` on merge |
| `docs/impl/README.md` | Modify | Auto-regenerated by `docz update` |

## Testing Plan

- [ ] Every exported function in `internal/preview/` has a focused
      unit test using `httptest`, `t.TempDir()`, and stub-injected
      dependencies (no real listeners, no real browser spawn in tests)
- [ ] Renderer table tests cover: empty input, frontmatter-only,
      plain markdown, GFM table, fenced code, callout, math, invalid
      theme
- [ ] Watcher tests cover: write-in-place, atomic rename, delete +
      recreate, debounced burst
- [ ] Server tests cover: `/` returns rendered HTML, `/theme.css`
      returns theme CSS, `/hljs.css` 404s when theme has none,
      livereload script presence toggled by flag
- [ ] cmd/preview_test.go covers: missing path arg, non-existent
      path, directory path, non-`.md` extension, invalid theme name,
      `--no-open` / `--no-watch` / `--no-livereload` each separately
- [ ] `go test -race -shuffle=on -count=3 ./...` green at the end of
      Phase 5

## Decisions

Resolved by user review on 2026-05-30. Every recommendation accepted.

| # | Topic | Decision |
|---|-------|----------|
| A | Package name | `internal/preview/` |
| B | CLI verb | `docz preview <path>` |
| C | Browser opener implementation | Hand-rolled GOOS switch (`os/exec.CommandContext`, build-tagged `open_darwin.go` / `open_linux.go` / `open_freebsd.go` / etc.) — zero new deps; reuses the `cmd/git.go` ctx-cancellation pattern |
| D | Windows support | Out of scope. Matches INV-0004 §Decisions 6 (v1 TUI is Unix-only) |
| E | Frontmatter handling | Strip the leading `---` YAML block before rendering — clean preview body that matches github.com's behavior |
| F | Default theme | `auto` — browser-driven via `prefers-color-scheme`; smallest binary surface |
| G | Default port | Ephemeral (`:0`) — never collides; bound URL is printed and passed to the opener |
| H | Live-reload toggle | `--no-livereload` boolean flag — symmetric with `--no-watch` and `--no-open` |
| I | Frontmatter helper location | `internal/preview/frontmatter.go` — preview-local helper that slices off the leading block; avoids pulling `internal/document` into the preview dependency graph for what is structurally a substring operation |
| J | File watcher backend | `github.com/fsnotify/fsnotify` — battle-tested, handles every editor save pattern we care about with a parent-directory watch |

## Dependencies

- **Blocking:** IMPL-0006, IMPL-0007, IMPL-0008, IMPL-0009 all merged
  to `main` with `status: Completed` (INV-0004 §Prerequisites).
  IMPL-0009 is the most load-bearing of the four because Runner's
  `Out`/`Err` writers and `RepoRoot` field are the seam every cmd
  test in this PR will exercise
- **External:** `github.com/donaldgifford/mdp` (v0.1.8 or newer) for
  `pkg/parser`, `pkg/theme`, `pkg/livereload`; `github.com/fsnotify/fsnotify`
  for the watcher; mdp's transitive deps (`goldmark`, `goldmark-mathjax`,
  `goldmark-highlighting`, `chroma`, `gorilla/websocket`,
  `gm-alert-callouts`, `goldmark-mermaid`) come along for the ride
- **Followed by:** IMPL-0011 (Bubble Tea v2 TUI Layer) wires the `p`
  key to `(*Runner).preview` or directly to
  `preview.NewServer(...).Start(ctx)` — that contract is the reason
  the preview package keeps a small, named public surface

## References

- INV-0004 — v1 Release Plan: TUI, Markdown Preview, and CLI Parity
  (`docs/investigation/0004-v1-release-plan-tui-markdown-preview-and-cli-parity.md`)
- mdp — https://github.com/donaldgifford/mdp
- mdp/pkg/parser — https://github.com/donaldgifford/mdp/tree/main/pkg/parser
- mdp/pkg/theme — https://github.com/donaldgifford/mdp/tree/main/pkg/theme
- mdp/pkg/livereload — https://github.com/donaldgifford/mdp/tree/main/pkg/livereload
- fsnotify — https://github.com/fsnotify/fsnotify
- goldmark — https://github.com/yuin/goldmark (mdp's parser backend)
- IMPL-0009 — Runner pattern + DocType registry (the prerequisite
  IMPL-0010 builds the new `cmd/preview.go` on top of)
