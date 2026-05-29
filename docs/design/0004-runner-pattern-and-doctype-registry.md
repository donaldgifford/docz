---
id: DESIGN-0004
title: "Runner Pattern and DocType Registry"
status: Approved
author: Donald Gifford
created: 2026-05-27
---
<!-- markdownlint-disable-file MD025 MD041 -->

# DESIGN 0004: Runner Pattern and DocType Registry

**Status:** Approved
**Author:** Donald Gifford
**Date:** 2026-05-27

<!--toc:start-->
- [Overview](#overview)
- [Goals and Non-Goals](#goals-and-non-goals)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Background](#background)
- [Detailed Design](#detailed-design)
  - [A. Runner struct shape and lifecycle](#a-runner-struct-shape-and-lifecycle)
  - [B. Flags binding model](#b-flags-binding-model)
  - [C. Output writers and --quiet](#c-output-writers-and---quiet)
  - [D. Logger handler choice](#d-logger-handler-choice)
  - [E. DocType registry API](#e-doctype-registry-api)
  - [F. Typed DocType and Status strings](#f-typed-doctype-and-status-strings)
  - [G. Library/pattern evaluation (Cobra-only)](#g-librarypattern-evaluation-cobra-only)
  - [H. Test strategy](#h-test-strategy)
- [API / Interface Changes](#api--interface-changes)
- [Data Model](#data-model)
- [Testing Strategy](#testing-strategy)
- [Migration / Rollout Plan](#migration--rollout-plan)
  - [Per-commit sequence](#per-commit-sequence)
- [Decisions Locked or Revised](#decisions-locked-or-revised)
- [Open Questions](#open-questions)
- [References](#references)
<!--toc:end-->

## Overview

DESIGN-0004 specifies the architectural shape that IMPL-0009 (Wave 5 of
INV-0002) will implement: a `cmd.Runner` struct that holds resolved
config and injected dependencies, command handlers as methods on
`*Runner`, a `config.DocTypeDef` registry that consolidates the
scattered per-type metadata, and typed `DocType` / `Status` strings at
internal API boundaries. The refactor eliminates the two systemic
testability blockers in `docz` today — `cmd/` package-level globals and
the 6+ scattered DocType definitions — without changing user-visible
CLI behavior or the YAML config schema.

## Goals and Non-Goals

### Goals

- Define a `Runner` struct that is the single injection surface for
  config, writers, logger, time, and git resolution.
- Replace every `runFoo` package-level handler with a method on
  `*Runner`, allowing tests to construct a Runner with `bytes.Buffer`
  writers and a fixed `Now`/`Git` instead of mutating package globals.
- Add a `log/slog` logger that writes to `Runner.Err` so test buffers
  capture log output without shared-stderr races.
- Consolidate the six built-in doc types into a single
  `internal/config/doctype.go` registry, derived from a literal
  `allDocTypes` slice with `DefaultConfig` as a constructor func.
- Introduce `type DocType string` and `type Status string` as compile-time
  signal at internal API boundaries, with no YAML schema change.
- Enable `t.Parallel()` on every existing cmd-layer test by removing the
  `os.Chdir`, package-global, and `os.Pipe` patterns blocking it today.

### Non-Goals

- Adding new user-facing commands or flags (beyond the `--log-format` and
  `--log-level` already in IMPL-0009 scope).
- Migrating from Cobra/Viper to another CLI framework.
- Adopting Bubble Tea, `fang`, or other Charm libraries — explicitly
  rejected below.
- Internationalization, structured-log shipping, or daemon mode.
- Changing the `.docz.yaml` schema. Existing config files must continue
  to parse byte-identically.

## Background

`docz` today carries the following testability debts (cross-referenced
to INV-0002 findings):

- **F1–F2: package-level globals in `cmd/`** — every flag (`createStatus`,
  `updateDryRun`, `wikiInitForce`, etc.) and `appCfg` is a `var` at file
  scope. Tests must reset these between runs, which prevents
  `t.Parallel()` and forces serialized execution.
- **F3: `if verbose { fmt.Fprintf(os.Stderr, ...) }` scattered across the
  cmd layer** — 20+ such blocks. No structured logging, no level
  control, no test capture.
- **F4–F5: package globals `timeNow` and `gitUserName`** — tests must
  monkey-patch these to inject test values; impossible to parallelize.
- **F6: `os.Chdir` in tests** — `internal/config/config_test.go` and
  several `cmd/*_test.go` files change the process working directory to
  steer config loading, fundamentally a process-wide side effect.
- **F14–F16: DocType scattered across 6+ locations** —
  `config.DefaultConfig`, `config.ValidTypes`, `config.ResolveTypeAlias`,
  `config.TypesHelp`, the embedded template filenames, the default nav
  titles. No single source of truth.

IMPL-0009 closes all of these in a single architectural pass. DESIGN-0004
is the prerequisite gate that locks in shape before code lands.

## Detailed Design

### A. Runner struct shape and lifecycle

The exact field set:

```go
// cmd/runner.go
type Runner struct {
    Cfg    config.Config
    Out    io.Writer
    Err    io.Writer
    Logger *slog.Logger
    Now    func() time.Time
    Git    GitResolver
}
```

No additional fields belong at construction time. `repoRoot` is **not**
a Runner field; it is a parameter to `config.Load(configFile, repoRoot)`
plumbed in `PersistentPreRunE` by calling `os.Getwd()` once. Keeping it
out of Runner avoids confusing "where this process is running" with
"what config does this command hold."

`context.Context` is **not** a Runner field. It belongs in method
signatures:

```go
func (r *Runner) Create(
    ctx context.Context,
    cmd *cobra.Command,
    opts createOpts,
    args []string,
) error
```

Cobra passes `cmd.Context()`; the Runner method threads it to any
context-aware call (git lookup, future timeouts).

**Construction model:** plain `NewRunner(cfg config.Config) *Runner`
with defaults. Functional options are rejected here — six fields with
unambiguous defaults do not justify the `WithXxx` ceremony, and
struct-literal construction in tests is clearer:

```go
func NewRunner(cfg config.Config) *Runner {
    return &Runner{
        Cfg: cfg,
        Out: os.Stdout,
        Err: os.Stderr,
        Logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
            Level: slog.LevelInfo,
        })),
        Now: time.Now,
        Git: realGit{},
    }
}
```

Tests construct directly:

```go
r := &cmd.Runner{
    Cfg:    config.DefaultConfig(),
    Out:    &bytes.Buffer{},
    Err:    &bytes.Buffer{},
    Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
    Now:    func() time.Time { return fixedTime },
    Git:    staticGit{Name: "Test User"},
}
```

**Lifecycle:** the Runner is built once in `PersistentPreRunE` and
stored in a single package-level variable `runner *Runner` in
`cmd/runner.go`. Per Decision §4 this matches the current `appCfg`
model. One global mutable replaces six — a net win even if it is not
zero-global. The Runner is effectively read-only after construction
(handlers do not mutate `r.Cfg`). Tests bypass `PersistentPreRunE`
entirely and construct the Runner directly.

`cobra.Command.Annotations` and context-stored runners are explicitly
rejected — both add type-assertion noise with no benefit at this scale.

### B. Flags binding model

Each command file owns a package-private options struct and a build
function `newXxxCmd(r *Runner) *cobra.Command`:

```go
// cmd/create.go
type createOpts struct {
    status   string
    author   string
    noUpdate bool
}

func newCreateCmd(r *Runner) *cobra.Command {
    var opts createOpts
    c := &cobra.Command{
        Use:  "create <type> <title>",
        RunE: func(cmd *cobra.Command, args []string) error {
            return r.Create(cmd.Context(), cmd, opts, args)
        },
    }
    c.Flags().StringVar(&opts.status, "status", "", "initial status")
    c.Flags().StringVar(&opts.author, "author", "", "document author")
    c.Flags().BoolVar(&opts.noUpdate, "no-update", false, "skip index update")
    return c
}
```

The opts struct is allocated per `newCreateCmd` call, so each test
invocation gets fresh flag state. This eliminates the current leak where
`createStatus = ""` must be reset between tests.

`--verbose` remains a persistent root flag. It is parsed before
`PersistentPreRunE`, so by the time `NewRunner(cfg)` runs the flag value
is final. The flag's package-level `var verbose bool` declaration is the
**only** acceptable package-level global remaining; tracking
`--log-level` and `--log-format` similarly. All other former globals are
folded into per-command options structs.

### C. Output writers and `--quiet`

Handlers write to `r.Out` / `r.Err` directly, never to
`cmd.OutOrStdout()` and never to `os.Stdout`. The Runner is the single
injection surface. Cobra's `cmd.OutOrStdout()` is redundant once
`runner.Out` covers the same need with simpler test wiring (no
`cmd.SetOut(&buf)` + `t.Cleanup` ceremony).

A `--quiet` persistent flag replaces `r.Out` with `io.Discard` in
`PersistentPreRunE`, silencing user-facing success lines (`"Created
RFC-0001"`) while preserving `r.Err` for warnings and errors. The logger
also writes to `r.Err` (see §D), so `--quiet` does not suppress log
output — that is the role of `--log-level=warn` or higher.

**Note on IMPL-0009 Phase 3 task wording.** IMPL-0009 Phase 3's task
list predates this design and refers to `cmd.OutOrStdout()` and
`cmd.SetOut(&buf)` as the replacement pattern. That wording is
superseded by this section: handlers write through `r.Out`/`r.Err`
exclusively, and tests construct a Runner with `bytes.Buffer` writers
rather than using `cmd.SetOut`/`cmd.SetErr`.

### D. Logger handler choice

The logger is constructed eagerly in `NewRunner`:

```go
slog.New(slog.NewTextHandler(r.Err, &slog.HandlerOptions{Level: level}))
```

Lazy initialization is rejected — it would require a mutex or `sync.Once`
to be safe across future goroutines, where eager construction is
trivially safe.

The handler writes to **`r.Err`**, not `os.Stderr`. This means tests
capturing `r.Err` also capture log lines, and `t.Parallel()` tests do
not race on a shared stderr.

**Flags:**

- `--verbose` (existing) — shorthand for `--log-level=debug`.
- `--log-level=debug|info|warn|error` (new) — explicit level. Default
  `info`. When both `--verbose` and `--log-level` are set,
  `--log-level` wins.
- `--log-format=text|json` (new, Decision §1) — `TextHandler` is the
  default; `--log-format=json` swaps in `JSONHandler` with identical
  options.

Internal packages remain silent. None of `config`, `document`, `index`,
`toc`, `wiki`, or `template` take a logger handle. They return typed
errors and structured data; the cmd layer logs. The one borderline case
(`toc.UpdateFiles` write errors collected in `UpdateReport`) is the
correct pattern — return data, let the caller decide whether to log.

### E. DocType registry API

```go
// internal/config/doctype.go
type DocTypeDef struct {
    Name          string         // canonical name ("rfc")
    Aliases       []string       // alternate input names ("implementation")
    DefaultConfig func() TypeConfig // fresh TypeConfig per call
    NavTitle      string         // "RFCs"
    PluralLabel   string         // "RFCs" (may differ from NavTitle)
    TemplateName  string         // "rfc" — embedded template lookup key
}

var allDocTypes = []DocTypeDef{
    {
        Name:    "rfc",
        Aliases: nil,
        DefaultConfig: func() TypeConfig {
            return TypeConfig{
                Enabled:  true,
                Dir:      "rfc",
                IDPrefix: "RFC",
                IDWidth:  4,
                Statuses: []string{"Draft", "Proposed", "Accepted", "Rejected", "Implemented"},
                // ...
            }
        },
        NavTitle:     "RFCs",
        PluralLabel:  "RFCs",
        TemplateName: "rfc",
    },
    // 5 more entries
}
```

**Registration model:** explicit literal, rejected in favor of `init()`
registration. `docz` has a closed set of built-in types with no
extension point; the literal is readable, greppable, and statically
analyzable. `init()` ordering would be invisible in tests.

**Export policy:** `allDocTypes` is package-private. The exported
accessors are:

- `AllDocTypes() []DocTypeDef` — returns `slices.Clone(allDocTypes)`.
- `LookupDocType(name string) (DocTypeDef, bool)` — case-insensitive,
  whitespace-trimmed, checks canonical then aliases.
- `DocTypeNames() []string` — sorted canonical names.

**`DefaultConfig` as constructor:** this is the most important deviation
from the IMPL-0009 sketch. Holding `TypeConfig` inline as a value would
share its `Statuses []string` slice across all callers — a single
errant `append` or assignment in a test silently corrupts the global. A
`func() TypeConfig` constructor costs one extra call per lookup and
eliminates the entire class of shared-mutation bugs.

**Name collision warning.** The `DocTypeDef` field is named
`DefaultConfig` (a `func() TypeConfig`) and the package-level accessor
that builds the full `Config` is also named `DefaultConfig()`. These
are distinct symbols at different scopes (struct field vs. package
function) but the collision is easy to miss in review. If it causes
confusion during implementation, prefer renaming the struct field to
`NewTypeConfig func() TypeConfig`. The DESIGN doc retains
`DefaultConfig` to match the IMPL-0009 sketch but the implementer may
choose either name.

**Derived state:** `DefaultConfig().Types`, `ValidTypes()`,
`DefaultNavTitles()`, `typeAliases`, and `TypesHelp()` all become
package-private build functions that iterate `allDocTypes`. The
top-level exported names stay the same — the change is internal.

**Consistency invariants (each in its own `t.Run`):**

- `TestDocTypeRegistry_AllHaveEmbeddedTemplate`
- `TestDocTypeRegistry_AllHaveEmbeddedIndexHeader`
- `TestDocTypeRegistry_NoDuplicateNames`
- `TestDocTypeRegistry_NoAliasCollidesWithCanonical`
- `TestDocTypeRegistry_DefaultConfigValidates`
- `TestDocTypeRegistry_DefaultConfigStatusesNonEmpty`
- `TestDocTypeRegistry_DerivedDefaultConfigMatchesHardcoded`

### F. Typed `DocType` and `Status` strings

Per Decision §2 the registry struct is `config.DocTypeDef`; the
typed-string wrapper is `config.DocType`. Plus:

```go
type DocType string
type Status string
```

**Enforcement surface** (sites that switch to the typed forms):

- `document.CreateOptions.Type` → `DocType`
- `template.Data.Type` → `DocType`
- `template.Data.Status` → `Status`
- `template.EmbeddedDocumentTemplate(docType DocType)` parameter
- `document.Frontmatter.Status` → `Status`

The `Config.Types` map **key remains plain `string`** — Go requires
explicit casting at every typed-string map access, which adds more noise
than value at the map-key layer. Typed wrappers stay at function
parameter and struct field boundaries.

**YAML compatibility — revising Decision §3.** Decision §3 said a
custom `UnmarshalYAML` is required for `Status` at the YAML boundary.
This is incorrect: `go.yaml.in/yaml/v3` unmarshals YAML string scalars
transparently into any Go type whose underlying kind is `string`. A
`type Status string` field tagged `yaml:"status"` will unmarshal without
any custom code. The same holds for `DocType`. **Decision §3 is revised:
no custom unmarshaler needed.** Verify with a parse golden test on a
`.docz.yaml` fixture.

### G. Library/pattern evaluation (Cobra-only)

Per IMPL-0009 Decisions §5 the DESIGN doc evaluates `charmbracelet/fang`,
Bubble Tea v2 alongside Cobra, and `localstack/lstk`.

- **`fang`** — has dropped its Bubble Tea v1 dependency; styling comes
  through `lipgloss/v2` and `charm.land/x/ansi`. The repo describes it
  as "small, experimental." It wraps Cobra to provide styled help,
  styled errors, automatic `--version`, manpage generation, and shell
  completion. The "experimental" label is the blocker: charm.land
  projects iterate fast and could pull in a TUI dependency later.
  Features it offers are out of IMPL-0009 scope. **Reject.**
- **Bubble Tea v2 alongside Cobra** — Bubble Tea v2.0.6 is stable. A
  legitimate pattern for future interactive prompts (e.g., interactive
  `docz create`). But docz has no current TUI requirement. The Runner
  pattern keeps the door open via a future `Runner.Prompt` injection
  field; adding Bubble Tea today is premature. **Reject.**
- **`localstack/lstk`** — useful **pattern reference**, not a
  dependency. The per-command options struct pattern in §B is the main
  borrowing. lstk's Runner is less formalized than what IMPL-0009
  proposes, but the command organization is clean. **Reference only.**

**Recommendation:** stay on plain Cobra + Viper. The Runner pattern
alone resolves every testability and DI concern; new library coupling
buys marginal UX gains for non-trivial volatility risk.

### H. Test strategy

Example post-refactor handler test:

```go
func TestRunner_Create_GeneratesFile(t *testing.T) {
    t.Parallel()

    dir := t.TempDir()
    rfcDir := filepath.Join(dir, "docs", "rfc")
    if err := os.MkdirAll(rfcDir, 0o750); err != nil {
        t.Fatal(err)
    }
    readme := "# RFCs\n\n" +
        "<!-- BEGIN DOCZ AUTO-GENERATED -->\n" +
        "<!-- END DOCZ AUTO-GENERATED -->\n"
    if err := os.WriteFile(filepath.Join(rfcDir, "README.md"),
        []byte(readme), 0o644); err != nil {
        t.Fatal(err)
    }

    cfg := config.DefaultConfig()
    cfg.DocsDir = filepath.Join(dir, "docs")

    fixedTime := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
    var out bytes.Buffer
    r := &Runner{
        Cfg:    cfg,
        Out:    &out,
        Err:    io.Discard,
        Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
        Now:    func() time.Time { return fixedTime },
        Git:    staticGit{Name: "Test User"},
    }

    opts := createOpts{status: "Draft"}
    if err := r.Create(context.Background(), nil, opts,
        []string{"rfc", "API Rate Limiting"}); err != nil {
        t.Fatalf("Create() error: %v", err)
    }

    if !strings.Contains(out.String(), "RFC-0001") {
        t.Errorf("output = %q, want RFC-0001", out.String())
    }
}
```

**Conditions for `t.Parallel()` safety:**

1. `t.TempDir()` for all filesystem state.
2. No mutation of package-level vars (`appCfg`, `updateDryRun`, etc.) —
   replaced by per-test Runner construction.
3. `Now` injected via `Runner.Now`; no mutation of `timeNow`.
4. `Git` injected via `Runner.Git`; no exec side effects.
5. Logger writes to `r.Err` (a `bytes.Buffer` or `io.Discard`); no
   shared stderr.

After Phase 7 (`config.Load(configFile, repoRoot string)`) there is no
`os.Chdir`, eliminating the final process-wide state mutation.

## API / Interface Changes

| Surface | Before | After |
|---------|--------|-------|
| `config.Load` | `Load(configFile string) (Config, error)` | `Load(configFile, repoRoot string) (Config, error)` — empty `repoRoot` falls back to `os.Getwd()` for back-compat |
| `document.CreateOptions.Type` | `string` | `DocType` |
| `template.Data.Type` | `string` | `DocType` |
| `template.Data.Status` | `string` | `Status` |
| `template.EmbeddedDocumentTemplate` | `(name string)` | `(docType DocType)` |
| `document.Frontmatter.Status` | `string` | `Status` |
| `config.ValidateType` | `(string, error)` | `(DocType, error)` |
| New: `config.DocTypeDef`, `config.AllDocTypes`, `config.LookupDocType`, `config.DocTypeNames` | n/a | exported |
| New: `cmd.Runner`, `cmd.NewRunner`, `cmd.GitResolver` | n/a | exported (within cmd) |
| New CLI flags | n/a | `--log-level`, `--log-format`, `--quiet` |

`.docz.yaml` schema: **unchanged**. All typed-string fields use the same
YAML tags as today (`yaml:"status"` etc.) and `go.yaml.in/yaml/v3`
handles the typed-string round trip without a custom unmarshaler.

## Data Model

No persisted-data schema changes. The in-memory `Config` and
`Frontmatter` structs gain typed-string field types (`DocType`,
`Status`) but the YAML representation is byte-identical.

The registry `allDocTypes` is the new in-memory source of truth for
DocType metadata. All previously hardcoded lookup tables (`typeAliases`,
`defaultNavTitles`, etc.) are derived at package init.

## Testing Strategy

- **Per-handler unit tests:** one focused test per `(*Runner).Xxx`
  method, using a struct-literal Runner with `bytes.Buffer` writers.
- **DocType registry consistency tests:** seven `t.Run` cases inside a
  single table loop (see §E).
- **`t.Parallel()` smoke test:** a single test that fans out 10
  concurrent runner invocations against distinct `t.TempDir()` roots to
  catch any remaining shared state.
- **slog handler capture test:** assert that `r.Logger.Debug(...)`
  output lands in the `bytes.Buffer` attached as `r.Err`.
- **Git resolver context test:** `staticGit` with a cancellable context
  proves `Ctrl+C` during `docz create` would cancel the git lookup.
- **YAML back-compat fixtures:** 3–5 real-world `.docz.yaml` files (this
  repo's plus 2–3 synthetic edge cases per Decision §7) parsed in a
  parameterized golden test. Includes the typed-string round trip.

## Migration / Rollout Plan

Per Decision §6: phases 2–11 ship as a **single PR**. DESIGN-0004
(Phase 1) ships separately as the prerequisite gate.

The single-PR call is validated by the phase interdependencies:
Phase 5 (time injection) needs Phase 3 (Runner methods) to have a call
site for `r.Now`. Phase 8 (DocType registry) needs Phase 3 to type
handlers against `DocType`. Splitting would require temporary
scaffolding or keeping both code paths live, which is more complexity
than the large single PR.

### Per-commit sequence

Each commit leaves `make ci` green:

1. `cmd/runner.go`: define `Runner` + `NewRunner` + `runner *Runner`
   global; wire in `PersistentPreRunE`. No handlers converted.
2. `cmd/git.go`: define `GitResolver`, `realGit`, `staticGit`. No
   callers yet.
3. Convert `runCreate` → `(*Runner).Create`. Bind `createOpts` in
   `newCreateCmd` closure. Replace all `fmt.Printf` / `os.Stdout` calls
   inside `runCreate` and its callees with `r.Out`/`r.Err` writes.
4. Convert `runUpdate`, `updateType`, `runToCUpdate` → Runner methods.
5. Convert `runInit` and related helpers; fold in Phase 7 (`config.Load`
   `repoRoot` parameter) at the same commit since the test rewrites are
   coupled.
6. Convert remaining handlers (`runWikiInit`, `runWikiUpdate`,
   `runList`, `runTemplateShow`, etc.); replace verbose-guard blocks
   with `r.Logger.Debug(...)` (Phases 3 + 4 merged for small commands).
   Verify the `ensureDoczInit → (*Runner).Init(ctx, nil, nil)` path
   (per Open Question 2) — a nil `*cobra.Command` argument must not
   crash, which is naturally satisfied because all output flows through
   `r.Out`/`r.Err`.
7. Delete `internal/document/time.go`; add `CreateOptions.CreatedAt`;
   populate from `runner.Now()`.
8. `internal/config/doctype.go`: `DocTypeDef`, `allDocTypes`, helpers;
   derive `DefaultConfig().Types`, `ValidTypes()`, etc.; add the seven
   consistency tests.
9. Define `type DocType string` + `type Status string`. Update
   `CreateOptions.Type`, `template.Data.Type/Status`,
   `Frontmatter.Status`. Add YAML back-compat golden tests.
10. Add `--log-level` and `--log-format` flags; wire the JSON handler;
    write the `--quiet` integration test.
11. Final sweep: `make ci`, add `t.Parallel()` annotations everywhere
    eligible, regenerate any drifted goldens.

## Decisions Locked or Revised

From IMPL-0009 §Decisions:

1. ✅ **Locked.** `slog.TextHandler` default; `--log-format=json` opt-in.
2. ✅ **Locked.** Registry struct is `config.DocTypeDef`; typed-string
   wrapper is `config.DocType`.
3. ⚠️ **Revised.** No custom `UnmarshalYAML` required —
   `go.yaml.in/yaml/v3` handles typed-string fields with underlying
   kind `string` transparently. The DESIGN doc supersedes IMPL-0009
   Decision §3, and IMPL-0009 Phase 10's "custom YAML marshaler" task
   is cancelled — drop it during implementation rather than reopening
   the question.
4. ✅ **Locked.** Runner is a per-process singleton.
5. ✅ **Locked.** Stay on plain Cobra + Viper; reject `fang` and Bubble
   Tea (see §G).
6. ✅ **Locked.** Phases 2–11 ship as a single PR; Phase 1 (this
   document) ships separately.
7. ✅ **Locked.** Collect 3–5 real-world `.docz.yaml` fixtures for parse
   golden tests.
8. ✅ **Locked.** No external repositories import
   `github.com/donaldgifford/docz/cmd`. Verified via `gh search code`
   on 2026-05-27. No build-tag fences or `internal/cmd` move needed. If
   the module ever becomes a library dependency, revisit by moving
   `cmd/` under `internal/` or exporting only `cmd.Execute()`.
9. ✅ **Locked.** `slog` shorthand by default; `slog.Attr` only on a
   profiled hot path.
10. ✅ **Locked.** Rollback strategy: revert the single Wave 5 PR.

## Open Questions

1. **`--log-format=json` test coverage scope.** Do we add a single
   smoke test or a full parameterized matrix? Recommendation: single
   smoke test asserting valid JSON output, leave the matrix for if/when
   a real consumer surfaces.
2. **`ensureDoczInit` calling `(*Runner).Init` with a nil `*cobra.Command`.**
   The current `cmd/wiki.go:ensureDoczInit` calls `runInit(nil, nil)`.
   After conversion, the Runner method must guard against a nil cmd
   when writing output — handled naturally because all output migrates
   to `r.Out`/`r.Err` instead of `cmd.Println`. Confirm during Phase 6
   conversion.
3. **Integration tests that call `rootCmd.Execute()`.** If any are added
   later, they must build a fresh command tree per test
   (`newCreateCmd(r)` etc.) because `rootCmd` flags do not reset between
   `Execute()` calls. Document this constraint in CONTRIBUTING.md when
   the runner pattern lands.

## References

- IMPL-0009 — Runner Pattern and DocType Registry Refactor
  (implementation plan this design gates)
- INV-0002 — Architectural Review and Cleanup Opportunities
  (findings F1–F6 and F14–F16 are the underlying problems)
- IMPL-0008 — Move Stranded Business Logic Into Internal Packages
  (provides the slim `cmd/` and `internal/document` shape this refactor
  builds on)
- `log/slog` package documentation (Go 1.21+) — structured logging API
- `go.yaml.in/yaml/v3` — typed-string unmarshal behavior verified for
  Decision §3 revision
- `charmbracelet/fang` — evaluated and rejected; see §G
- `charmbracelet/bubbletea` v2 — evaluated and rejected; see §G
- `localstack/lstk` — pattern reference for per-command options; see §G
