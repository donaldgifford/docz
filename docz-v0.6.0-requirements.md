# docz v0.6.0 requirements — public surface for the sdk-booty-sh loop harness

> **Handoff doc.** This file lives in `sdk-booty-sh` only as the requirements
> capture; copy it into the docz repo and seed that repo's own docz documents
> from it (likely one IMPL doc via `docz create impl`, plus a dated amendment to
> DESIGN-0007). docz owns final API naming — everything below marked _proposed
> shape_ is a consumer's sketch, not a contract on names.

## Consumer context

`sdk-booty-sh` is building a doc-driven agent loop harness (its DESIGN-0002 /
IMPL-0002): an autonomous runner that uses a docz IMPL doc as its work source —
parse phases and checkbox tasks, execute one task per iteration, check tasks
off, and transition doc status, all programmatically and byte-preservingly.

Its `pkg/loop/doczwork` package needs three things docz v0.5.0 doesn't expose: a
body/task parser, a write API, and a viper-free config load. These close
**upstream, once, in docz** (sdk-booty-sh INV-0008 Observation 9 decision)
rather than as parsing workarounds in every consumer. **This slate blocks
sdk-booty-sh IMPL-0002 Phase W** — the harness work source cannot start until
`v0.6.0` tags (its decided policy is to wait for the tag; no `replace`
directives).

## Baseline: what v0.5.0 already provides

Public (`pkg/doczcore`), all read-only:

- `config.Load(configFile, repoRoot string) (Config, error)` — viper-backed
- `Config.TypeDir(docType string) string`, `ValidateType`, `EnabledTypes`,
  status/type definitions
- `document.ParseFrontmatter(content []byte) (Frontmatter, error)`
- `document.LoadFrontmatter(path string) (Frontmatter, []byte, error)`
- `document.ScanDocuments(dir string) ([]DocEntry, error)` (sorted by ID;
  invalid frontmatter silently skipped), `IsDoczFile`
- `Frontmatter{ID, Title, Status config.Status, Author, Created}`;
  `DocEntry{Frontmatter, Filename, Content}`

Internal (not importable) but promotion-ready:

- `internal/docwrite.SetStatus(path, newStatus string) (oldStatus string, err error)`
  — byte-preserving frontmatter status rewrite, LF-only
- `internal/toc.ParseHeadings(content string) []Heading` — dependency-free,
  fence-aware, slug-generating heading walk (`Heading{Level, Text, Slug}` —
  H2–H6, no line numbers today)

viper appears in exactly one non-CLI path: `config.Load` →
`mergeConfigFile(v *viper.Viper, ...)` / `loadFromFile`. All config structs
already carry dual `mapstructure` + `yaml` tags, so removal is mechanical.

## R1 — viper-free `config.Load`

**Requirement:** same exported API, yaml-only implementation. Importing
`pkg/doczcore/...` must not compile viper (or its transitive tree) into a
consumer's build.

- Keep the signature: `Load(configFile, repoRoot string) (Config, error)`.
- Replace the viper merge path (`mergeConfigFile` / `loadFromFile`) with
  `gopkg.in/yaml.v3` decoding onto the existing structs (the `yaml` tags are
  already there).
- Preserve current behavior: defaults fill, `.docz.yaml` discovery from
  `repoRoot`, explicit `configFile` override, the types-replace-on-presence
  logic, validation.
- **Document the decode delta:** viper matches keys case-insensitively; yaml.v3
  is case-sensitive. Call it out in the changelog / release notes (real-world
  `.docz.yaml` files use lowercase keys, so breakage is unlikely but must be
  stated). The existing parity baseline test should pin this.

**Acceptance:** `go list -deps` of a scratch module importing only
`pkg/doczcore/config` + `pkg/doczcore/document` contains no
`github.com/spf13/viper` packages; existing config tests and the parity baseline
stay green.

## R2 — new `pkg/doczcore/docparse` (heading walk + plan model)

**Requirement:** a public body parser producing (1) a heading walk with line
positions and (2) an IMPL-style plan model — phases containing checkbox tasks
with checked state and line numbers. Generalize `internal/toc.ParseHeadings`
rather than writing a second walker; note the current `toc.Heading` has no line
field, so `docparse` defines its own richer type (internal/toc can stay as-is or
delegate).

_Proposed shape:_

```go
package docparse

type Heading struct {
    Level int    // 2–6, H1 excluded (matches internal/toc behavior)
    Text  string // inline markdown stripped
    Slug  string // GitHub-compatible anchor
    Line  int    // 1-based line of the heading
}

func Headings(content []byte) []Heading

type Plan struct{ Phases []Phase }

type Phase struct {
    Title string // heading text, e.g. "Phase T: Toolbelt (pkg/tool)"
    Line  int    // heading line
    Tasks []Task
}

type Task struct {
    Text    string // task text, checkbox marker stripped
    Checked bool
    Line    int // 1-based line of the "- [ ]" / "- [x]" item
}

func ParsePlan(content []byte) (Plan, error)
```

**Leniency rules** (the consumer's fixture corpus includes messy hand-written
IMPL docs, not just template output):

- A phase is any `###` heading (tolerate `**bold**` phase names — inline
  markdown stripped, per the existing walker).
- Tasks are **top-level** checkbox list items (`- [ ]` / `- [x]`, tolerant of
  leading whitespace and `*` bullets) within the phase's span — under a
  `#### Tasks` heading when one exists, otherwise anywhere in the phase.
- Nested checkbox sub-items belong to their parent task (they are not
  independent tasks).
- Prose, tables, and non-checkbox lists between tasks are ignored, not errors.
- Checkboxes inside code fences are ignored (the fence-awareness the toc walker
  already has).
- Line accounting is LF-only, matching `docwrite`'s contract, so `Task.Line`
  feeds `CheckTask` directly with no re-scan.

**Acceptance:** golden tests over template-conformant and messy fixtures;
`Task.Line` values verified against the raw file bytes (a consumer will write
through them).

## R3 — promote `docwrite` + add `CheckTask`

**Requirement:** promote `internal/docwrite` to `pkg/doczcore/docwrite` (keeping
the CLI on the same implementation) and add a checkbox toggler under the same
contract.

- `SetStatus(path, newStatus string) (oldStatus string, err error)` — as-is, now
  public. Byte-preserving single-splice rewrite, LF-only.
- New:

```go
// CheckTask flips the unchecked task item on the given 1-based line to
// checked ("[ ]" -> "[x]"), preserving every other byte. LF-only files.
func CheckTask(path string, line int) error
```

- Semantics: the target line must be an unchecked checkbox list item (leading
  whitespace + `-`/`*` bullet + `[ ]`); exactly the three marker bytes change.
  Typed, distinguishable errors for: line out of range, line is not a task item,
  task already checked. Same read-whole-file / single-splice / write-back
  approach as `SetStatus`.
- This revises DESIGN-0007's read-only-public stance for `pkg/doczcore` — add a
  dated amendment note there (write surface = status + checkbox only, by design;
  not a general editor).

**Acceptance:** golden byte-diff tests — the on-disk diff after `CheckTask`
touches exactly one line, byte-identical elsewhere; `SetStatus` behavior
unchanged under its existing tests from the new import path.

## Non-goals for v0.6.0

- A general markdown AST or editing API — the write surface is status +
  checkbox, deliberately.
- Frontmatter editing beyond status.
- CRLF support — LF-only stays the contract.
- New CLI commands — the CLI may adopt `docparse`/`docwrite` internally, but
  nothing here requires user-facing changes.

## Release / compatibility

- Additive minor release: `v0.6.0`. No changes to any v0.5.0 exported signature.
- Suggested phasing for the docz IMPL doc: viper removal → `docparse` →
  `docwrite` promotion + `CheckTask` → tag. (The three work items are
  independent; any order works, the tag gates the consumer.)
- Downstream gate to flip when done: sdk-booty-sh IMPL-0002 Phase 0 checks off
  against the pushed tag.

## References

- sdk-booty-sh (consumer):
  `docs/investigation/0008-doc-driven-agent-loop-harness-scoping-hcl-jobs-over-impl-docs.md`
  Observation 9 (the promotion slate + import-cleanliness audit),
  `docs/design/0002-doc-driven-agent-loop-harness-v1-design.md` (work-source
  section — the exact call sequence `doczwork` makes),
  `docs/impl/0002-doc-driven-agent-loop-harness-v1-implementation.md` Phase 0
  (the milestone mirror of this slate).
- docz: `docs/design/0007-docz-changes-to-support-docz-api-and-docz-site.md`
  (read-only-public stance being revised by R3).
