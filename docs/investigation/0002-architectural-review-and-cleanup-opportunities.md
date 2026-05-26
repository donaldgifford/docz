---
id: INV-0002
title: "Architectural Review and Cleanup Opportunities"
status: In Progress
author: Donald Gifford
created: 2026-05-15
---

<!-- markdownlint-disable-file MD025 MD041 -->

# INV 0002: Architectural Review and Cleanup Opportunities

**Status:** In Progress **Author:** Donald Gifford **Date:** 2026-05-15

<!--toc:start-->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
  - [Critical: Testability blockers](#critical-testability-blockers)
    - [F1. Package-level globals in cmd/ prevent parallel testing [multi-agent]](#f1-package-level-globals-in-cmd-prevent-parallel-testing-multi-agent)
    - [F2. Direct fmt.Printf / os.Stdout instead of cmd.OutOrStdout() [multi-agent]](#f2-direct-fmtprintf--osstdout-instead-of-cmdoutorstdout-multi-agent)
    - [F3. if verbose { fmt.Fprintf(os.Stderr, ...) } pattern repeated 20+ times [multi-agent]](#f3-if-verbose--fmtfprintfosstderr---pattern-repeated-20-times-multi-agent)
    - [F4. internal/document/time.go uses mutable package-global timeNow [multi-agent]](#f4-internaldocumenttimego-uses-mutable-package-global-timenow-multi-agent)
    - [F5. gitUserName() exec call not injectable](#f5-gitusername-exec-call-not-injectable)
    - [F6. config.Load reads .docz.yaml from working directory implicitly [multi-agent]](#f6-configload-reads-doczyaml-from-working-directory-implicitly-multi-agent)
  - [Critical: Correctness bugs](#critical-correctness-bugs)
    - [F7. Config validation error swallowed at startup [multi-agent]](#f7-config-validation-error-swallowed-at-startup-multi-agent)
    - [F8. mergeConfigFile silently swallows YAML parse errors](#f8-mergeconfigfile-silently-swallows-yaml-parse-errors)
    - [F9. currentDate() calls timeNow() three times [multi-agent]](#f9-currentdate-calls-timenow-three-times-multi-agent)
    - [F10. Bare return err without context wrap [multi-agent]](#f10-bare-return-err-without-context-wrap-multi-agent)
    - [F11. DocTitle returns fallback value alongside non-nil error](#f11-doctitle-returns-fallback-value-alongside-non-nil-error)
  - [High: Three-way duplication of defaults [multi-agent]](#high-three-way-duplication-of-defaults-multi-agent)
    - [F12. cmd/init.go:writeDefaultConfig() hardcodes YAML that duplicates DefaultConfig()](#f12-cmdinitgowritedefaultconfig-hardcodes-yaml-that-duplicates-defaultconfig)
  - [High: Library returns user-facing strings](#high-library-returns-user-facing-strings)
    - [F13. internal/index returns user-facing messages as (msg, err) [multi-agent]](#f13-internalindex-returns-user-facing-messages-as-msg-err-multi-agent)
  - [High: Domain modeling — DocType scattered](#high-domain-modeling--doctype-scattered)
    - [F14. "Document type" expressed in 6+ locations with no compile-time enforcement [multi-agent]](#f14-document-type-expressed-in-6-locations-with-no-compile-time-enforcement-multi-agent)
    - [F15. ValidTypes() iteration vs appCfg.Types map disconnect](#f15-validtypes-iteration-vs-appcfgtypes-map-disconnect)
    - [F16. DocType and Status are bare string everywhere](#f16-doctype-and-status-are-bare-string-everywhere)
  - [High: Stranded business logic in cmd/](#high-stranded-business-logic-in-cmd)
    - [F17. cmd/wiki.go:writeMkDocsYAML is business logic, not command wiring](#f17-cmdwikigowritemkdocsyaml-is-business-logic-not-command-wiring)
    - [F18. cmd/update.go:updateToCs is business logic stranded in cmd](#f18-cmdupdategoupdatetocs-is-business-logic-stranded-in-cmd)
    - [F19. runWikiUpdateNav and runWikiUpdateDryRun duplicate nav-building logic](#f19-runwikiupdatenav-and-runwikiupdatedryrun-duplicate-nav-building-logic)
  - [Medium: Package boundary issues](#medium-package-boundary-issues)
    - [F20. internal/index has three responsibilities](#f20-internalindex-has-three-responsibilities)
    - [F21. Frontmatter parsing duplicated across packages](#f21-frontmatter-parsing-duplicated-across-packages)
    - [F22. Three different regexes for "is this a docz file"](#f22-three-different-regexes-for-is-this-a-docz-file)
    - [F23. Two Slugify functions with different algorithms [multi-agent]](#f23-two-slugify-functions-with-different-algorithms-multi-agent)
  - [Medium: Stutter and naming](#medium-stutter-and-naming)
    - [F24. template.TemplateData stutters with package name [multi-agent]](#f24-templatetemplatedata-stutters-with-package-name-multi-agent)
    - [F25. WikiIndexType / WikiIndexData stutter](#f25-wikiindextype--wikiindexdata-stutter)
    - [F26. ToCConfig / ToC inconsistent initialism casing](#f26-tocconfig--toc-inconsistent-initialism-casing)
    - [F27. Named return values used as variables in Validate()](#f27-named-return-values-used-as-variables-in-validate)
  - [Medium: Mechanical / style fixes](#medium-mechanical--style-fixes)
    - [F28. os.IsNotExist(err) instead of errors.Is(err, fs.ErrNotExist) [multi-agent]](#f28-osisnotexisterr-instead-of-errorsiserr-fserrnotexist-multi-agent)
    - [F29. sort.Slice instead of slices.SortFunc](#f29-sortslice-instead-of-slicessortfunc)
    - [F30. strings.NewReader(string(data)) instead of bytes.NewReader(data) [multi-agent]](#f30-stringsnewreaderstringdata-instead-of-bytesnewreaderdata-multi-agent)
    - [F31. String concatenation for paths instead of filepath.Join [multi-agent]](#f31-string-concatenation-for-paths-instead-of-filepathjoin-multi-agent)
    - [F32. Hand-rolled itoa instead of strconv.Itoa [multi-agent]](#f32-hand-rolled-itoa-instead-of-strconvitoa-multi-agent)
    - [F33. defer enc.Close() missing in cmd/config.go](#f33-defer-encclose-missing-in-cmdconfiggo)
    - [F34. Magic strings "adr" and "csv" without constants](#f34-magic-strings-adr-and-csv-without-constants)
    - [F35. MinHeadings: 3 magic number](#f35-minheadings-3-magic-number)
    - [F36. File mode 0o644 / 0o750 scattered across 17 sites](#f36-file-mode-0o644--0o750-scattered-across-17-sites)
    - [F37. Literal filenames ".docz.yaml" / "README.md" / "mkdocs.yml" / "templates" scattered](#f37-literal-filenames-doczyaml--readmemd--mkdocsyml--templates-scattered)
  - [Medium: Duplication patterns](#medium-duplication-patterns)
    - [F38. unknown document type %q (valid types: %s) error repeated 4 times [multi-agent]](#f38-unknown-document-type-q-valid-types-s-error-repeated-4-times-multi-agent)
    - [F39. docType := config.ResolveTypeAlias(strings.ToLower(args[0])) repeated 6 times](#f39-doctype--configresolvetypealiasstringstolowerargs0-repeated-6-times)
    - [F40. Enabled-type guard block repeated](#f40-enabled-type-guard-block-repeated)
  - [Medium: Performance — actually worth fixing](#medium-performance--actually-worth-fixing)
    - [F41. Files read twice during docz update [multi-agent]](#f41-files-read-twice-during-docz-update-multi-agent)
    - [F42. ParseHeadings called twice on dry-run path [multi-agent]](#f42-parseheadings-called-twice-on-dry-run-path-multi-agent)
  - [Low: Performance — leave alone](#low-performance--leave-alone)
  - [Low: Minor cleanups](#low-minor-cleanups)
    - [F43. Run vs RunE inconsistency](#f43-run-vs-rune-inconsistency)
    - [F44. Missing SilenceUsage: true on root command](#f44-missing-silenceusage-true-on-root-command)
    - [F45. Package comment in wrong file](#f45-package-comment-in-wrong-file)
    - [F46. Pointer-to-options where value would suffice](#f46-pointer-to-options-where-value-would-suffice)
    - [F47. No input validation in Create](#f47-no-input-validation-in-create)
    - [F48. Frontmatter parse misses \r\n line endings](#f48-frontmatter-parse-misses-rn-line-endings)
    - [F49. setDefaults does not cover all Config fields](#f49-setdefaults-does-not-cover-all-config-fields)
    - [F50. ExistingNavOrder silently drops bare-string nav entries](#f50-existingnavorder-silently-drops-bare-string-nav-entries)
  - [What's already good](#whats-already-good)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
  - [Wave 1 — Mechanical wins (1 PR, ~1 day)](#wave-1--mechanical-wins-1-pr-1-day)
  - [Wave 2 — Correctness and duplication (1 PR, ~1 day)](#wave-2--correctness-and-duplication-1-pr-1-day)
  - [Wave 3 — Performance worth fixing (1 PR, ~half day)](#wave-3--performance-worth-fixing-1-pr-half-day)
  - [Wave 4 — Stranded business logic (2 PRs, ~2 days)](#wave-4--stranded-business-logic-2-prs-2-days)
  - [Wave 5 — Architecture refactor (separate DESIGN doc, ~1 week)](#wave-5--architecture-refactor-separate-design-doc-1-week)
  - [Low priority / defer](#low-priority--defer)
- [References](#references)
<!--toc:end-->

## Question

What architectural debt, idiomatic-Go gaps, style violations, and performance
issues exist in the docz codebase today, and how should they be prioritized into
a cleanup roadmap that improves testability, extensibility, and maintainability
without breaking the existing public surface?

## Hypothesis

The codebase predates the team's standardized Go style guides and architecture
patterns, so we expect to find:

1. **Testability blockers** — package-level globals in `cmd/`, hardcoded
   `os.Stdout`/`os.Stderr` writes, and time/exec calls that cannot be
   intercepted.
2. **Domain-modeling drift** — "document type" represented as scattered strings
   across config maps, hardcoded slices, alias maps, and template filenames with
   no compile-time enforcement that they stay in sync.
3. **Three-way duplication of defaults** — `internal/config.DefaultConfig()`,
   the hardcoded YAML in `cmd/init.go:writeDefaultConfig()`, and Viper's
   `setDefaults()` all encode the same truth.
4. **Stranded business logic in `cmd/`** — file I/O and YAML generation that
   belongs in `internal/` packages.
5. **Stutter and style drift** — `template.TemplateData`, `ToCConfig`,
   `os.IsNotExist`, hand-rolled `itoa`, manual date formatting, etc.

We expect most issues to be mechanical fixes; the larger architectural items
(Runner pattern, DocType registry) will need a dedicated design doc.

## Context

The docz codebase (~4,200 LOC across `cmd/` and `internal/`) was written before
the team adopted standardized go-development style guides, the Uber Go Style
Guide as a reference, and structured architecture-review practices. Dogfooding
docz on its own repository surfaced several pain points (documented in
INV-0001), and recent bug fixes (`fix/update-skips-disabled-types`,
`fix/version-and-mkdocs-extensions`) revealed that the defaults-duplication
problem is actively producing drift bugs.

We want a single source of truth for outstanding cleanup work so the team can
plan a phased refactor and ratchet quality up over time.

**Triggered by:** Dogfooding pain points; bug pattern of three-way default drift
(`markdown_extensions` PR #30, disabled-types PR #31).

## Approach

Ran four specialized review agents in parallel against the full codebase, each
with a distinct focus, then synthesized and deduplicated the findings:

1. **Architecture review** (`go-development:go-architect`) — package boundaries,
   global state, dependency injection, domain modeling, extensibility,
   testability gaps, error-handling strategy.
2. **Uber Go Style audit** (`go-development:go-style`) — naming, receivers,
   error wrapping, magic constants, named returns, initialism casing,
   time/path/sort idioms.
3. **Performance review** (`go-development:go-performance`) — algorithmic
   complexity, allocations, I/O patterns, regex/parsing efficiency, and honest
   "do not optimize" calls for code that is fine at CLI scale.
4. **Idiomatic Go review** (general-purpose) — error handling pitfalls,
   stringly-typed code, magic constants, Cobra-specific conventions (`Run` vs
   `RunE`, `cmd.OutOrStdout()`, `SilenceUsage`), logging, duplication.

All four agents read the same set of files; findings were cross-referenced to
identify the highest-confidence (multi-agent agreement) issues.

## Environment

| Component               | Value                                 |
| ----------------------- | ------------------------------------- |
| Go module version       | 1.25.7                                |
| Codebase size           | ~4,200 LOC (`cmd/` + `internal/`)     |
| Built-in document types | 6 (rfc, adr, design, impl, plan, inv) |
| CLI framework           | Cobra + Viper                         |
| Template engine         | `text/template` with `//go:embed`     |
| Current docz version    | v0.0.10-5-g2272af5                    |

## Findings

The findings are grouped by severity and theme. Items marked **[multi-agent]**
were flagged independently by two or more review agents.

### Critical: Testability blockers

#### F1. Package-level globals in `cmd/` prevent parallel testing [multi-agent]

**Files:** `cmd/root.go:29-34`, `cmd/init.go:14`, `cmd/create.go:16-20`,
`cmd/update.go:16`, `cmd/list.go:19-22`, `cmd/wiki.go:16-21`

The `cmd` package declares 14+ mutable package-level variables for Cobra flags
plus the resolved `appCfg`. Every test must manually reset these between cases;
`t.Parallel()` would cause races. Tests in `cmd/` capture stdout via `os.Pipe`
tricks (~20 occurrences) because handlers call `fmt.Printf` directly against
`os.Stdout`. The combination of these two patterns is the single biggest blocker
to clean unit tests.

**Impact:** Tests cannot run in parallel. Flag state leaks between test cases.
Output-capture is fragile (a `t.Fatal` before restoration leaks stdout). Adding
any new flag adds another implicit dependency to every existing cmd test.

#### F2. Direct `fmt.Printf` / `os.Stdout` instead of `cmd.OutOrStdout()` [multi-agent]

**Files:** all of `cmd/*.go` (50+ call sites); enumerated in
`cmd/create.go:65,72-74,93`, `cmd/init.go:40,56,65,181,189,208`,
`cmd/update.go:49,67,76,95,104,115,126,133,142,147`,
`cmd/template.go:105,135,150-154`,
`cmd/wiki.go:107,137-138,151,162,167,173,183,227,233,296,337,345-348,357-361`,
`cmd/version.go:22`, `cmd/list.go:107-122,125-129,131-145`

Cobra commands receive a `*cobra.Command` whose `OutOrStdout()`,
`ErrOrStderr()`, `Println()`, and `PrintErrf()` route through writers that tests
can override via `cmd.SetOut(buf)` / `cmd.SetErr(buf)`. Every handler currently
discards the `*cobra.Command` parameter (`_ *cobra.Command`) and writes directly
to `os.Stdout` / `os.Stderr`. This is the single biggest testability win
available.

#### F3. `if verbose { fmt.Fprintf(os.Stderr, ...) }` pattern repeated 20+ times [multi-agent]

**Files:** `cmd/create.go:65,72-74`, `cmd/init.go:40,65,189`,
`cmd/update.go:49,67,76,115,126,133,142,147`,
`cmd/wiki.go:137-138,151,162,167,173,227,233,296`, `cmd/template.go:150-154`

There is no logger abstraction; every cmd file has its own copies. Cannot
configure log level beyond on/off. Cannot capture in tests. Should be replaced
with `log/slog` (stdlib since Go 1.21; we are on 1.25). `--verbose` becomes a
slog handler level. Plumbing slog through internal packages is straightforward.

#### F4. `internal/document/time.go` uses mutable package-global `timeNow` [multi-agent]

**File:** `internal/document/time.go:7`

```go
var timeNow = time.Now
```

Tests override this and restore via `t.Cleanup`. Not safe for parallel tests;
not explicit at call sites. The idiomatic alternative is passing `time.Time` or
a `func() time.Time` as a parameter via `CreateOptions`.

#### F5. `gitUserName()` exec call not injectable

**File:** `cmd/create.go:132-138`

```go
out, err := exec.CommandContext(context.Background(), "git", "config", "user.name").Output()
```

Cannot be intercepted in tests. The author-resolution path is therefore not
exercised in cmd tests — they either set `createAuthor = ""` and get "Unknown"
or whatever `git config` returns in CI. Should accept a `func() string` or
`GitUserResolver` interface; should also accept `cmd.Context()` instead of
`context.Background()` so the lookup is cancellable.

#### F6. `config.Load` reads `.docz.yaml` from working directory implicitly [multi-agent]

**File:** `internal/config/config.go:151-178`, `cmd/wiki_test.go:12-27`,
`internal/config/config_test.go:127-191`

`Load` reads `.docz.yaml` from `os.Getwd()`. Every test that exercises config
loading must `os.Chdir` into a temp dir and use `t.Cleanup` to restore. This is
process-wide state — tests cannot run in parallel even within a single test
file. Fix: change `Load(configFile string)` to
`Load(configFile, repoRoot string)` and read from `repoRoot` instead.

### Critical: Correctness bugs

#### F7. Config validation error swallowed at startup [multi-agent]

**File:** `cmd/root.go:72-79`

```go
warnings, validErr := cfg.Validate()
for _, w := range warnings { fmt.Fprintf(os.Stderr, "Warning: %s\n", w) }
if validErr != nil { fmt.Fprintf(os.Stderr, "Config error: %v\n", validErr) }
appCfg = cfg
```

Validation error is printed to stderr but the program continues with the invalid
config. A user with `statuses: []` in `.docz.yaml` sees a warning and then
cryptic downstream failures. Fix: use `PersistentPreRunE` on `rootCmd` instead
of `cobra.OnInitialize` and propagate the error as a non-zero exit.

#### F8. `mergeConfigFile` silently swallows YAML parse errors

**File:** `internal/config/config.go:262-272`

```go
func mergeConfigFile(v *viper.Viper, path string) error {
    if _, err := os.Stat(path); err != nil {
        return nil //nolint:nilerr // missing config file is not an error
    }
    fileV := viper.New()
    fileV.SetConfigFile(path)
    if err := fileV.ReadInConfig(); err != nil {
        return nil //nolint:nilerr // unreadable config file is silently skipped
    }
    return v.MergeConfigMap(fileV.AllSettings())
}
```

A malformed `.docz.yaml` (YAML syntax error, parse failure) is silently
discarded — the user gets defaults plus no diagnostic, then later wonders why
their config "doesn't work". Distinguish "file does not exist" (return nil) from
"file exists but cannot be parsed" (return wrapped error).

#### F9. `currentDate()` calls `timeNow()` three times [multi-agent]

**File:** `internal/document/create.go:118-121`

```go
return fmt.Sprintf("%d-%02d-%02d",
    timeNow().Year(), timeNow().Month(), timeNow().Day())
```

Three clock reads. Theoretical midnight-skew bug. Also: should use
`time.DateOnly` (Go 1.20+) instead of manual `fmt.Sprintf`:

```go
return timeNow().Format(time.DateOnly)
```

#### F10. Bare `return err` without context wrap [multi-agent]

**Files:** `cmd/update.go:54`, `cmd/create.go:89`, `cmd/init.go:51`,
`cmd/wiki.go:103,109,154-156,178-180,197-199`, `internal/wiki/wiki.go:58`

Multiple sites return errors without wrapping in
`fmt.Errorf("context: %w", err)`. When these surface to the user, the stack
trace is lost and the message provides no context about which operation failed.

#### F11. `DocTitle` returns fallback value alongside non-nil error

**File:** `internal/wiki/titles.go:27-43`

```go
func DocTitle(filePath string) (string, error) {
    data, err := os.ReadFile(filePath)
    if err != nil {
        return FilenameTitle(filepath.Base(filePath)), err  // unusual contract
    }
    ...
}
```

Returning both a usable value and an error is an unusual API contract that
invites bugs in callers. The wiki package already checks `err != nil` and
discards the value, but the contract should be one-or-the-other.

### High: Three-way duplication of defaults [multi-agent]

#### F12. `cmd/init.go:writeDefaultConfig()` hardcodes YAML that duplicates `DefaultConfig()`

**Files:** `cmd/init.go:60-183` (100+ lines of literal YAML),
`internal/config/config.go:66-144` (Go-literal version),
`internal/config/config.go:291-319` (`setDefaults` Viper version)

The same defaults are encoded in three places. Adding `markdown_extensions` to
`DefaultConfig` would not propagate to the written file; PR #30 was exactly this
drift. `setDefaults` currently misses `MarkdownExtensions`, `DocsDir`,
`RepoURL`, `SiteURL`, and `Theme` — added later but never wired into Viper
defaults.

**Fix:** Replace the literal YAML in `writeDefaultConfig` with
`yaml.Marshal(config.DefaultConfig())`. Either derive `setDefaults` from the
struct via reflection or remove it in favor of pre-defaulted struct
unmarshalling.

### High: Library returns user-facing strings

#### F13. `internal/index` returns user-facing messages as `(msg, err)` [multi-agent]

**Files:** `internal/index/index.go:92` (`UpdateReadme(...) (string, error)`),
`internal/index/index.go:117` (`DryRunReadme`)

Returns `"Updated %s"`, `"Created %s"`, `"Warning: %s has no DOCZ markers..."`
as the success-path string. The warning case returns `nil` error but a string
starting with `"Warning:"` that the caller is supposed to inspect.

**Fix:** Return a typed result like
`type UpdateOutcome { Action UpdateAction; Path string }`. The cmd layer formats
the user-facing message.

### High: Domain modeling — `DocType` scattered

#### F14. "Document type" expressed in 6+ locations with no compile-time enforcement [multi-agent]

| Location                            | Content                                     |
| ----------------------------------- | ------------------------------------------- |
| `internal/config/config.go:204`     | `ValidTypes()` — hardcoded ordered slice    |
| `internal/config/config.go:66-144`  | `DefaultConfig().Types` — map of TypeConfig |
| `internal/config/config.go:209-211` | `typeAliases` — private alias map           |
| `internal/config/config.go:192-201` | `DefaultNavTitles()` — separate map         |
| `internal/config/config.go:224-232` | `TypesHelp()` — hardcoded prose             |
| `internal/template/templates/*.md`  | One file per type, named by convention      |
| `cmd/init.go:70-175`                | YAML duplicating all of the above           |

Adding a new type (e.g., `runbook`) requires changes in all of these places with
no compile-time signal if any step is missed. A type added to `ValidTypes()`
without an embedded template panics at runtime.

**Fix:** Introduce a `DocType` value struct that bundles canonical name,
aliases, default `TypeConfig`, nav title, and template name. `ValidTypes()`
derives from a registration list. Add `TestAllTypesHaveEmbeddedTemplates` as
compile-time-style enforcement.

#### F15. `ValidTypes()` iteration vs `appCfg.Types` map disconnect

**Files:** `cmd/create.go:54-57`, `cmd/update.go:35-43`, `cmd/list.go:52-59`,
`cmd/init.go:36`, `cmd/wiki.go:307`

Every command iterates `config.ValidTypes()` (hardcoded slice) and looks each
name up in `appCfg.Types` (the actual config). A user who adds a custom type to
the map (which `Validate()` only warns about) sees their type ignored by all
iteration. Two orderings exist: the hardcoded slice and the map iteration order.

**Fix:** Add `Config.EnabledTypes() []string` returning sorted enabled canonical
type names. Iterate that single method instead of the slice + map lookup.

#### F16. `DocType` and `Status` are bare `string` everywhere

**Files:** `internal/config/config.go:204`, `internal/document/create.go:14-15`,
`internal/template/template.go:20`, `internal/template/embed.go:14`,
`internal/document/document.go:16`

Should be `type DocType string` and `type Status string` with constants and
validation methods. Currently every command does manual `strings.ToLower` +
`ResolveTypeAlias` + map lookup + error wrapping.

### High: Stranded business logic in `cmd/`

#### F17. `cmd/wiki.go:writeMkDocsYAML` is business logic, not command wiring

**File:** `cmd/wiki.go:247-288`

Constructs the initial `mkdocs.yml` content by manual string building. Knows the
structure of a valid mkdocs.yml. Cannot be unit-tested without going through the
cmd layer and the `appCfg` global. `internal/wiki` already handles
reading/writing mkdocs.yml via `ReadMkDocs`/`WriteMkDocs`.

**Fix:** Move to `internal/wiki` as
`wiki.CreateMkDocs(path string, cfg MkDocsConfig) error`.

#### F18. `cmd/update.go:updateToCs` is business logic stranded in cmd

**File:** `cmd/update.go:110-150`

Reads files from disk, calls `toc.UpdateToC`, compares content, writes back.
Distinct from command I/O concerns. Cannot be unit-tested in isolation — must go
through `updateType` which also writes README.

**Fix:** Move to `internal/toc` as
`toc.UpdateFiles(paths []string, minHeadings int, dryRun bool) (UpdateReport, error)`.

#### F19. `runWikiUpdateNav` and `runWikiUpdateDryRun` duplicate nav-building logic

**File:** `cmd/wiki.go:135-210`

Both functions have nearly identical bodies: call `wiki.ScanDocs`,
`wiki.ReadMkDocs`, `wiki.ExistingNavOrder`, then either `MergeNavOrder` or
`SortEntries`. Diverge only at the final output step.

**Fix:** Extract shared nav-building helper that returns `[]wiki.NavEntry`.

### Medium: Package boundary issues

#### F20. `internal/index` has three responsibilities

**File:** `internal/index/index.go`

Package currently handles:

1. Directory scanning (`ScanDocuments`, `DocEntry`)
2. Markdown table generation (`GenerateTable`)
3. README marker splicing (`UpdateReadme`, `DryRunReadme`, `spliceMarkers`,
   `createNewReadme`)

The package comment says "document scanning and README index generation" — the
"and" is the tell.

**Fix:** Move `ScanDocuments` + `DocEntry` to `internal/document` (it already
uses `document.Frontmatter` via embedding). Keep README splicing in
`internal/index`.

#### F21. `Frontmatter` parsing duplicated across packages

**Files:** `internal/index/index.go:48-56`, `internal/wiki/titles.go:33-43`

Both packages read a file and call `document.ParseFrontmatter`, with divergent
failure handling (index silently skips; wiki falls back to filename). Should be
one helper `document.LoadFrontmatter(path string)` with a single documented
error contract.

#### F22. Three different regexes for "is this a docz file"

**Files:** `internal/wiki/wiki.go:12` (`^\d{4,}-.*\.md$`),
`internal/index/index.go:21` (`^\d+-.*\.md$`), `internal/document/create.go:36`
(`^(\d+)-.*\.md$`)

The wiki regex requires 4+ digits; the others accept 1+. A file `1-foo.md` would
index but not appear in the wiki nav. Inconsistent invariant.

**Fix:** Single exported `document.DoczFilePattern` (or
`document.IsDoczFile(name) bool`).

#### F23. Two `Slugify` functions with different algorithms [multi-agent]

**Files:** `internal/template/template.go:38-54`, `internal/toc/toc.go:38-56`

- `template.Slugify`: filename slug, strips to `[a-z0-9-]`, max 64 chars,
  word-boundary truncation.
- `toc.Slugify`: GitHub anchor slug, keeps `[a-z0-9 -]`, no truncation.

Both exported, both named `Slugify`. Confusing for any code that imports both.

**Fix:** Rename to `template.FilenameSlug` and `toc.AnchorSlug`. Document the
GitHub anchor algorithm reference in the toc one.

### Medium: Stutter and naming

#### F24. `template.TemplateData` stutters with package name [multi-agent]

**File:** `internal/template/template.go:14`

Reads as `template.TemplateData` — `templateTemplateData`. Per Go convention,
the package's purpose is clear from the import path. Rename to `template.Data`.

#### F25. `WikiIndexType` / `WikiIndexData` stutter

**File:** `internal/template/template.go:82,88`

Same as F24. Consider renaming or moving these to a sub-context.

#### F26. `ToCConfig` / `ToC` inconsistent initialism casing

**File:** `internal/config/config.go:50,62`

Per Uber style and Go convention, initialisms should be all uppercase:
`TOCConfig`, `TOC`. Currently uses `ToC` which is inconsistent (would write
`URLConfig`, not `UrlConfig`).

**Fix:** Rename `ToCConfig` → `TOCConfig`, field `ToC` → `TOC`. Keep YAML tag
`"toc"` so users' config files don't break.

#### F27. Named return values used as variables in `Validate()`

**File:** `internal/config/config.go:236`

```go
func (c *Config) Validate() (warnings []string, err error) {
```

Named returns add no value here — every return statement names both explicitly.
Per Uber guide, named returns should be reserved for clarification or `defer`
use. Inverted footgun: a future bare `return` silently returns whatever was
assigned.

### Medium: Mechanical / style fixes

#### F28. `os.IsNotExist(err)` instead of `errors.Is(err, fs.ErrNotExist)` [multi-agent]

**Files:** `cmd/wiki.go:120`, `internal/index/index.go:36,95,120`

The legacy form does not unwrap error chains. Modern idiom since Go 1.13 is
`errors.Is(err, fs.ErrNotExist)`.

#### F29. `sort.Slice` instead of `slices.SortFunc`

**Files:** `internal/index/index.go:64`,
`internal/wiki/wiki.go:104,109,114,148`, `internal/wiki/mkdocs.go:123`

We're on Go 1.25.7; `slices.SortFunc` (generics, type-safe) is preferred over
`sort.Slice` (interface-based).

#### F30. `strings.NewReader(string(data))` instead of `bytes.NewReader(data)` [multi-agent]

**File:** `internal/wiki/titles.go:57`

```go
scanner := bufio.NewScanner(strings.NewReader(string(data)))
```

`data` is already `[]byte`. The conversion copies the entire file content into a
new string allocation, then wraps it in `strings.Reader`. Use
`bytes.NewReader(data)` directly.

#### F31. String concatenation for paths instead of `filepath.Join` [multi-agent]

**Files:** `internal/template/template.go:72,98`

```go
localPath := docsDir + "/templates/" + docType + ".md"
localPath := docsDir + "/templates/wiki_index.md"
```

Not portable (Windows uses `\`). Use `filepath.Join`. The
`internal/template/embed.go` uses of `"templates/" + name` are correct because
`embed.FS` always uses forward slashes.

#### F32. Hand-rolled `itoa` instead of `strconv.Itoa` [multi-agent]

**File:** `internal/toc/toc.go:209-219`

```go
// itoa converts a small integer to a string without importing strconv.
func itoa(n int) string { ... }
```

The "without importing strconv" rationale saves nothing — `strconv` is already
in the standard library and adds no binary weight. The custom implementation
prepends bytes via `append([]byte{...}, digits...)` — worse than `strconv.Itoa`
and has subtly different `n=0` behavior.

#### F33. `defer enc.Close()` missing in `cmd/config.go`

**File:** `cmd/config.go:24-30`

```go
enc := yaml.NewEncoder(os.Stdout)
enc.SetIndent(2)
if err := enc.Encode(appCfg); err != nil {
    return fmt.Errorf("encoding config: %w", err)  // enc.Close() never called
}
return enc.Close()
```

If `Encode` fails, `Close` is never called. Standard `defer enc.Close()` fixes
it.

#### F34. Magic strings `"adr"` and `"csv"` without constants

**Files:** `cmd/update.go:85-87`, `cmd/list.go:92`

```go
heading := "All " + strings.ToUpper(typeName) + "s"
if typeName == "adr" { heading = "All ADRs" }   // magic "adr"
```

```go
case "csv":  // formatJSON is a constant, but "csv" is inline
```

The "adr" branch is a special-case for English pluralization that should be a
`PluralLabel` field on `TypeConfig`. The `"csv"` literal should be a `formatCSV`
constant matching `formatJSON`.

#### F35. `MinHeadings: 3` magic number

**File:** `internal/config/config.go:141`

Should be `const defaultMinHeadings = 3` referenced from `DefaultConfig()`.

#### F36. File mode `0o644` / `0o750` scattered across 17 sites

**Files:** `cmd/init.go:46,177,204`, `cmd/template.go:127,131`,
`internal/index/index.go:109,162,166`, `internal/document/create.go:43,78`

Define in a single location:

```go
const (
    FileMode os.FileMode = 0o644
    DirMode  os.FileMode = 0o750
)
```

#### F37. Literal filenames `".docz.yaml"` / `"README.md"` / `"mkdocs.yml"` / `"templates"` scattered

**Files:** `cmd/init.go:50,61`, `cmd/update.go:64`, `cmd/template.go:115`,
`cmd/wiki.go:216,292`, `internal/config/config.go:134,163,169`,
`internal/template/template.go:72,98`

Define filename constants once:

```go
const (
    ConfigFileName = ".docz.yaml"
    IndexFileName  = "README.md"
    WikiIndexName  = "index.md"
    MkDocsFileName = "mkdocs.yml"
    TemplatesDir   = "templates"
)
```

### Medium: Duplication patterns

#### F38. `unknown document type %q (valid types: %s)` error repeated 4 times [multi-agent]

**Files:** `cmd/create.go:55-56`, `cmd/list.go:56-57`,
`cmd/template.go:141-142`, `cmd/update.go:39-40`

Plus a fifth variation in `internal/config/config.go:249`.

**Fix:** `config.ValidateType(name string) (canonicalName string, err error)`
that resolves alias, lowercases, validates, and returns a typed error. Single
call site per command.

#### F39. `docType := config.ResolveTypeAlias(strings.ToLower(args[0]))` repeated 6 times

**Files:** `cmd/create.go:50`, `cmd/list.go:54`, `cmd/update.go:37`,
`cmd/template.go:71,86,110`

Roll into the `ValidateType` helper from F38 (lowercase internally).

#### F40. Enabled-type guard block repeated

**Files:** `cmd/init.go:37-43`, `cmd/update.go:46-52`, `cmd/wiki.go:307-311`

```go
tc, ok := appCfg.Types[typeName]
if !ok || !tc.Enabled {
    if verbose { fmt.Fprintf(os.Stderr, "Type %s is disabled, skipping.\n", typeName) }
    continue
}
```

**Fix:** `Config.EnabledTypes() []string` (see F15).

### Medium: Performance — actually worth fixing

#### F41. Files read twice during `docz update` [multi-agent]

**Files:** `cmd/update.go:70` + `cmd/update.go:113`,
`internal/index/index.go:48`

`ScanDocuments` reads every file to parse frontmatter, then `updateToCs` calls
`os.ReadFile` again on the exact same files. For 1000 docs that's 2000
`ReadFile` calls. The ToC update doesn't need the frontmatter, but the scan
threw away the bytes after parsing.

**Fix:** Either cache `[]byte` on `DocEntry`, or restructure the call order so
ToC update happens during scan.

#### F42. `ParseHeadings` called twice on dry-run path [multi-agent]

**Files:** `cmd/update.go:131-136`, `internal/toc/toc.go:181-206`

```go
updated, found := toc.UpdateToC(string(data), ...)   // calls ParseHeadings internally
...
headings := toc.ParseHeadings(string(data))           // called again, same content
```

**Fix:** Change `UpdateToC` to return headings:
`UpdateToC(content string, minHeadings int) (updated string, headings []Heading, found bool)`.

### Low: Performance — leave alone

Per the performance review, these were considered and rejected as premature
optimization for a CLI:

- `internal/template/template.go:38` multi-pass `Slugify` — called once per
  create.
- `internal/toc/toc.go:92` `strings.Split` on full content — appropriate for
  typical file sizes.
- `internal/toc/toc.go:165` `strings.Repeat("  ", ...)` in loop — under 100
  headings, unmeasurable.
- `cmd/update.go:45` sequential type processing — parallelism adds complexity
  with no measurable gain.
- `internal/wiki/wiki.go:24` sequential directory walk — OS page cache handles
  this efficiently.

### Low: Minor cleanups

#### F43. `Run` vs `RunE` inconsistency

**File:** `cmd/version.go:21` uses `Run`; everything else uses `RunE`.

Use `RunE` consistently. Costs nothing, future-proofs error returns.

#### F44. Missing `SilenceUsage: true` on root command

**File:** `cmd/root.go:36`

When `RunE` returns an error, Cobra prints the usage block in addition to the
error. Production CLIs typically suppress this. Set `SilenceUsage: true` on
`rootCmd`.

#### F45. Package comment in wrong file

**File:** `internal/wiki/titles.go:1` (has the package doc) vs
`internal/wiki/wiki.go` (does not)

Convention places the canonical package comment in the file whose name matches
the package.

#### F46. `Pointer-to-options` where value would suffice

**Files:** `internal/document/create.go:40` (`Create(opts *CreateOptions)`),
`internal/template/template.go:123`
(`Render(tmplContent string, data *TemplateData)`)

Neither function mutates the input. Small structs (8 fields) can be passed by
value without performance impact and clarify that the function is pure-in.

#### F47. No input validation in `Create`

**File:** `internal/document/create.go:40`

Doesn't check `opts.Title != ""`, `opts.Prefix != ""`, `opts.IDWidth > 0`. Empty
prefix produces filename `0001-.md`. Zero IDWidth produces `1-foo.md`.

#### F48. Frontmatter parse misses `\r\n` line endings

**File:** `internal/document/document.go:30-39`

Requires `---\n` (optionally with leading space/tab) — fails on `---\r\n`. Many
markdown tools emit CRLF. Normalize or accept both.

#### F49. `setDefaults` does not cover all `Config` fields

**File:** `internal/config/config.go:291-319`

Missing `MarkdownExtensions`, `DocsDir`, `RepoURL`, `SiteURL`, `Theme` — added
later but never wired into Viper defaults. This is a maintenance trap: adding a
field to `Config` requires manually updating `setDefaults`.

**Fix:** Derive via reflection, or remove `setDefaults` and rely on
pre-defaulted struct unmarshalling.

#### F50. `ExistingNavOrder` silently drops bare-string nav entries

**File:** `internal/wiki/mkdocs.go:66-89`

A nav entry can legally be a bare string in mkdocs.yml. The function only
handles `map[string]any` items. A user-written `mkdocs.yml` with a top-level
bare-string page could lose that entry on rewrite.

### What's already good

The reviews explicitly called out the following as well-done patterns that
should not change:

- All regexes are compiled at package init (`var` declarations), never per-call.
- `ParseFrontmatter` works directly on `[]byte` with `bytes.Cut` — no
  unnecessary string conversion.
- `GenerateTable` and `GenerateToC` use `strings.Builder` throughout.
- `spliceMarkers` uses `strings.Cut` (Go 1.18 idiomatic).
- `MergeNavOrder` uses a map for O(1) lookup.
- `NavToYAML` pre-allocates the result slice.
- `%w` wrapping is used consistently for non-bare errors.
- `embed.FS` usage is correct (forward slashes throughout).
- Test coverage is solid — golden files under `testdata/golden/`, table-driven
  tests, `t.TempDir()` for filesystem isolation.
- No goroutine sins, no resource leaks (no manual `os.Open` to leak).
- The `0o644` / `0o750` octal literal style is modern.
- `filepath.ToSlash` is correctly used in `internal/wiki/wiki.go:86` for
  cross-platform nav paths.

## Conclusion

**Answer:** Yes — there is significant, well-defined cleanup opportunity. The
codebase is fundamentally sound (good package structure, solid test coverage, no
concurrency or resource bugs) but suffers from three categories of debt:

1. **Testability debt** — package globals in `cmd/`, direct `os.Stdout` writes,
   implicit working-directory dependencies, and uninjected side-effects (time,
   exec). This blocks `t.Parallel()` and makes every new feature add hidden
   state coupling.
2. **Domain-modeling debt** — `DocType` represented as scattered strings across
   config, aliases, templates, and help prose. Adding a new type requires
   changes in 6+ places with no compile-time enforcement.
3. **Style and idiom debt** — written before standardized style guides; contains
   stutter (`TemplateData`), legacy idioms (`os.IsNotExist`, `sort.Slice`,
   hand-rolled `itoa`), three-way duplication of defaults, and 50+ direct
   `fmt.Printf` writes that prevent test capture.

None of these are blocking bugs in production usage today, but the
defaults-drift category has already produced two real bugs (PR #30, PR #31) and
will produce more. The testability and domain-modeling work unblocks future
feature work.

## Recommendation

A phased refactor in five waves, ordered by impact-to-effort and by what
unblocks subsequent work:

### Wave 1 — Mechanical wins (1 PR, ~1 day)

Pure mechanical fixes with no design questions. Low risk, high readability gain.
Can land as a single "style sweep" PR with focused commits.

- F28: `os.IsNotExist` → `errors.Is(err, fs.ErrNotExist)`
- F29: `sort.Slice` → `slices.SortFunc`
- F30: `strings.NewReader(string(data))` → `bytes.NewReader(data)`
- F31: `filepath.Join` for paths in `internal/template/template.go`
- F32: replace `itoa` with `strconv.Itoa`
- F33: `defer enc.Close()` in `cmd/config.go`
- F36: centralize file mode constants
- F37: centralize literal filename constants
- F35: `defaultMinHeadings` constant
- F43: `Run` → `RunE` in `cmd/version.go`
- F44: `SilenceUsage: true` on root
- F45: move package doc-comment to `internal/wiki/wiki.go`
- F27: drop named returns in `Validate()`
- F9: `time.DateOnly` in `currentDate`; capture `t := timeNow()` once

### Wave 2 — Correctness and duplication (1 PR, ~1 day)

Fix the active bug class: defaults drift and silent error swallowing.

- F12: derive `writeDefaultConfig` from `DefaultConfig()` via `yaml.Marshal`
- F49: regenerate or remove `setDefaults` to cover all fields
- F7: propagate config validation error via `PersistentPreRunE`
- F8: distinguish "file missing" from "file unparseable" in `mergeConfigFile`
- F10: wrap all bare `return err` sites with context
- F38–F40: extract `ValidateType` helper and `EnabledTypes()` method; collapse
  the four "unknown type" sites and three enabled-type-guard sites.
- F34: special-case `"adr"` → `PluralLabel` field on `TypeConfig`

### Wave 3 — Performance worth fixing (1 PR, ~half day)

- F41: cache `[]byte` on `DocEntry` to eliminate double file reads
- F42: change `UpdateToC` signature to return `[]Heading`

### Wave 4 — Stranded business logic (2 PRs, ~2 days) — **In Progress (IMPL-0008)**

Move logic out of `cmd/` into testable `internal/` packages. These changes land
independently and unblock Wave 5.

Status as of 2026-05-25:

- **PR A (Phases 1–3)** — open at #44, awaiting CI/merge
  - [x] F17: `writeMkDocsYAML` → `internal/wiki.CreateMkDocs`
  - [x] F18: `updateToCs` → `internal/toc.UpdateFiles` (with categorized
        `UpdateReport`)
  - [x] F19: `wiki.BuildNav` extracted; cmd functions <30 lines
- **PR B (Phases 4–10)** — pushed at `feat/impl-0008-pr-b`, opens after
  PR A merges per IMPL-0008 Decisions §6
  - [x] F20: `internal/index` split — `ScanDocuments` + `DocEntry` moved to
        `internal/document/scan.go`
  - [x] F21: `document.LoadFrontmatter(path)` — single read+parse site
  - [x] F22: single `document.DoczFilePattern` + `IsDoczFile`
  - [x] F23: `template.Slugify` → `FilenameSlug`,
        `toc.Slugify` → `AnchorSlug`, `nonAlphanumHyphen` → `nonSlugChar`
  - [x] F11: `DocTitle` returns strict `(string, error)` per Decisions §3
  - [x] F13: typed `index.UpdateOutcome` + `UpdateAction` enum
  - [x] F24: `template.TemplateData` → `template.Data`
  - [x] F25: `WikiIndexType` / `WikiIndexData` left as-is per Decisions §4
  - [x] F26: `ToCConfig` → `TOCConfig`, field `ToC` → `TOC` (YAML tag
        stays `toc:` for back-compat — guarded by `TestLoad_TOCConfig`)

### Wave 5 — Architecture refactor (separate DESIGN doc, ~1 week)

These changes require alignment on the new architecture before implementation.
Worth a dedicated DESIGN doc.

- F1: introduce `Runner` struct holding config + `io.Writer`s; convert cmd
  handlers to methods; per-command options struct binds flags
- F2: switch all `fmt.Printf` / `os.Stdout` to `cmd.Println` /
  `cmd.OutOrStdout()`
- F3: introduce `log/slog` logger; replace 20+ `if verbose { ... }` sites
- F4: pass time as `CreateOptions.CreatedAt` instead of `timeNow` global
- F5: make `gitUserName` injectable; accept `cmd.Context()`
- F6: change `config.Load(configFile, repoRoot string)`; eliminate `os.Chdir` in
  tests
- F14: introduce `DocType` registry struct; derive `ValidTypes()`,
  `DefaultConfig().Types`, nav titles, and template names from a single
  registration list; add `TestAllTypesHaveEmbeddedTemplates`
- F15: drive iteration from `EnabledTypes()`
- F16: introduce `type DocType string` and `type Status string` typed constants

### Low priority / defer

The performance review explicitly flagged these as "do not optimize": multi-pass
`Slugify`, `strings.Split` of full content, `strings.Repeat` in tight loops,
sequential directory walking, sequential type processing. Honor that judgment —
they're fine for CLI scale.

F47 (`Create` input validation) and F48 (CRLF frontmatter) are nice-to-have
robustness fixes; bundle into Wave 2 or land opportunistically.

## References

- INV-0001: Wiki Init Template and Init Enabled Fix — earlier dogfooding pass
  that surfaced related issues
- PR #30 (`fix/version-and-mkdocs-extensions`) — instance of three-way defaults
  drift
- PR #31 (`fix/update-skips-disabled-types`) — instance of `ValidTypes()`
  iteration vs config-map disconnect
- Uber Go Style Guide — naming, error handling, initialism, defer, named returns
- Effective Go — package layout, interface design, error wrapping
- `cmd/` source — all command files reviewed
- `internal/config/config.go` — config struct, defaults, validation
- `internal/document/{document,create,time}.go` — frontmatter, create, time
- `internal/index/index.go` — scanning, table generation, README splicing
- `internal/template/{template,embed}.go` — template resolution and rendering
- `internal/toc/toc.go` — table-of-contents generation
- `internal/wiki/{wiki,titles,mkdocs}.go` — MkDocs nav generation
- Review agents used: `go-development:go-architect`, `go-development:go-style`,
  `go-development:go-performance`, and a general-purpose idiomatic Go review
