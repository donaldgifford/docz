---
id: DESIGN-0014
title: "The docz API as one unit: packages, types, functions, and the cmd swap"
status: Draft
author: Donald Gifford
created: 2026-09-14
---

<!-- markdownlint-disable-file MD025 MD041 -->

# DESIGN-0014: The docz API as one unit: packages, types, functions, and the cmd swap

<!--toc:start-->
- [Overview](#overview)
- [Goals and Non-Goals](#goals-and-non-goals)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Background](#background)
- [Detailed Design](#detailed-design)
  - [1. Module layout and dependency graph](#1-module-layout-and-dependency-graph)
  - [2. Package specifications](#2-package-specifications)
    - [2.1 config (L0, unchanged)](#21-config-l0-unchanged)
    - [2.2 document (L0, additive)](#22-document-l0-additive)
    - [2.3 docparse (L0, additive)](#23-docparse-l0-additive)
    - [2.4 docwrite (L1, additive)](#24-docwrite-l1-additive)
    - [2.5 toc (L1, unchanged)](#25-toc-l1-unchanged)
    - [2.6 index (L1, promoted)](#26-index-l1-promoted)
    - [2.7 doctemplate (L1, promoted)](#27-doctemplate-l1-promoted)
    - [2.8 repo (L3, new)](#28-repo-l3-new)
    - [2.9 impl (L2, new)](#29-impl-l2-new)
    - [2.10 wiki (integration, promoted)](#210-wiki-integration-promoted)
    - [2.11 validate (L2, new)](#211-validate-l2-new)
  - [3. The IMPL grammar](#3-the-impl-grammar)
  - [4. The cmd swap](#4-the-cmd-swap)
  - [5. Consumer map](#5-consumer-map)
  - [6. Enforcing the layer rules](#6-enforcing-the-layer-rules)
  - [7. Context, cancellation, and hooks](#7-context-cancellation-and-hooks)
- [API / Interface Changes](#api--interface-changes)
- [Data Model](#data-model)
- [Testing Strategy](#testing-strategy)
- [Migration / Rollout Plan](#migration--rollout-plan)
- [Open Questions](#open-questions)
  - [1. Phase and task grammar](#1-phase-and-task-grammar)
  - [2. Continuation folding and verify lines](#2-continuation-folding-and-verify-lines)
  - [3. Marker parsing on the read side](#3-marker-parsing-on-the-read-side)
  - [4. Criteria classification](#4-criteria-classification)
  - [5. Creation without I/O](#5-creation-without-io)
  - [6. Resolving a document by ID](#6-resolving-a-document-by-id)
  - [7. Where the status no-op short-circuit lives](#7-where-the-status-no-op-short-circuit-lives)
  - [8. How much of wiki is orchestration](#8-how-much-of-wiki-is-orchestration)
  - [9. Error shapes in repo](#9-error-shapes-in-repo)
  - [10. Generic facts for the next type package](#10-generic-facts-for-the-next-type-package)
  - [11. Delivery granularity](#11-delivery-granularity)
  - [12. Hooks for wiki](#12-hooks-for-wiki)
- [References](#references)
<!--toc:end-->

## Overview

ADR-0002 decides that docz is an API package whose first consumer is the
CLI, built as one unit and then swapped under `cmd/`. This design is that
unit: every package, type, function, and error the library exposes; the
IMPL grammar `pkg/impl` implements; the repository operations, wiki
integration, and config rendering that today live only in `cmd/`; the map
of which consumer calls what; and the `cmd/` swap that is the final step and
the acceptance test. DESIGN-0013's inventory and `pkg/impl` specification
carry over here; DESIGN-0013 itself is Abandoned. DESIGN-0015 (structured
regions and `docz validate`) is a **requirement** of this unit: the IMPL
grammar locates its spans by region, the validator is part of the API, and
no release carries one design without the other.

## Goals and Non-Goals

### Goals

- **Specify the complete API once.** After this design, building the
  library is an IMPL with phases, not further design.
- **Every CLI command is one library call plus printing.** The swap PR's
  `cmd/` diff proves the API is complete, and the parity suite (§4) proves
  it reproduces v1.2.2 byte for byte. That is how the API is validated: the
  CLI is the real-world test, and no external consumer is part of it.
- **Byte cores under every path helper** (ADR-0002 R3), so a no-checkout
  consumer — the shape tempy and docz-api have — is first-class for
  mutation as well as reading.
- **`pkg/impl` fully specified** — surface, grammar, identity rules — with
  `Doc` as the root type.
- **Typed results everywhere; wording nowhere but `cmd/`** (R4).
- **No CLI behaviour change** from the swap (ADR-0001 Decision 7).
- **DESIGN-0015 ships in the same unit.** Region markers, the validator,
  and `docz validate` are part of the API; a release without them is not
  this API.
- **Traceable, not tracing.** Context enters at L3 and hooks let a consumer
  instrument the library; docz imports no telemetry and logs nothing (§7).

### Non-Goals

- Typed models for RFC, ADR, DESIGN, or INV. The pattern is fixed (§2.9,
  §3); each waits for a consumer.
- A runtime type system or a library-defined interface over type packages
  (R6).
- Consumer policy in docz: deferred/skipped writers, plan diffs, iteration
  budgets, gates. `Task.Line` and `Task.EndLine` are the hook; the writers
  are the consumer's (ADR-0002 Decision 5).
- New CLI features beyond an optional read-only `docz task list` (ADR-0002
  Open Question 4).
- Changing `.docz.yaml`, the embedded template contents, or the
  `NNNN-slug.md` filename convention.
- The `updated:` field (DESIGN-0012 / IMPL-0017), which retargets to the
  release after the swap.
- Telemetry or logging inside the library. The CLI, docz-api, and tempy
  wire their own over the context and hooks in §7.

## Background

ADR-0002's decisions this design implements:

| ADR-0002 | Decision | Where it lands here |
| -------- | -------- | ------------------- |
| 1 | API package; CLI is the first consumer; one call per command | §4 swap table |
| 2 | Layers L0–L4, rules R1–R7 | §1 graph, §6 |
| 3 | One module | §1 |
| 4 | Standalone `pkg/<type>` with `Doc` root | §2.9, §3 |
| 5 | Primitives in docz, policy in consumers | §2.4 byte cores, §5 consumer map |
| 6 | Build the whole API, swap `cmd/` last | Rollout |
| 7 | ADR-0001 freeze lifted for new packages until v2.0.0 is cut; this unit ships betas only | API changes table |
| — | DESIGN-0015: regions, the validator, `docz validate` — a requirement of this unit | §2.3, §2.11, §3, §4 |

Carried over from DESIGN-0013 without change in substance: the inventory of
what the CLI does and where each piece lands (§3 there, folded into §2 and
§4 here), the `pkg/impl` surface and grammar (§5 there, §2.9 and §3 here
with `Plan` renamed `Doc`), and the byte cores (§6 there, §2.4 here).
Resolved in the DESIGN-0013 review and taken as settled: `impl.Doc` (its
OQ 2), `pkg/doczcore/repo` as the home of repository operations (OQ 7),
`doctemplate` promoted with the embed unexported (OQ 8), `pkg/wiki`
promoted (OQ 9), no depguard for now (OQ 13), one module (OQ 15).

Added in the 2026-09-19 review: explicit region markers replace the
heading-text heuristics for span finding, the IMPL template takes the full
marker set, a validator and `docz validate` join the unit (all specified in
DESIGN-0015), and the library exposes a context parameter and hooks at L3
so every consumer instruments it its own way (§7).

Today's dependency graph is in DESIGN-0013 Background. The distance from it
to §1 below is: three packages move out of `internal/`, two packages are
new, one package grows four functions, and `cmd/` shrinks.

## Detailed Design

### 1. Module layout and dependency graph

```text
github.com/donaldgifford/docz
├── pkg/doczcore/
│   ├── config        L0  load · merge · validate · type registry              unchanged
│   ├── document      L0  frontmatter · scan · changelog                      unchanged
│   ├── docparse      L0  headings · task items · title · markers · regions  additive
│   ├── docwrite      L1  byte cores + path wrappers · render · create        +4 funcs
│   ├── toc           L1  ToC splice                                          unchanged
│   ├── index         L1  README index table + marker splice                  promoted
│   ├── doctemplate   L1  resolve + render · embed unexported · default yaml  promoted
│   ├── validate      L2  Document · Schema · Finding · kind catalogue        new
│   └── repo          L3  Scan · List · Find · Create · Update · SetStatus · Init · Validate · InsertRegions · templates   new
├── pkg/impl          L2  Parse([]byte) → Doc · Validate                      new
├── pkg/wiki          --  MkDocs / TechDocs: nav, mkdocs.yml, Init, UpdateNav promoted
├── cmd/              L4  cobra shell: flags → one call → print               swapped
├── internal/         --  empty
└── test/consumer/    --  external-module proof over every pkg/ package
```

```mermaid
flowchart TD
  subgraph L4 ["L4 — cmd/"]
    cmd
  end
  subgraph L3 ["L3 — repository operations"]
    repo["doczcore/repo"]
    wiki["pkg/wiki"]
  end
  subgraph L2 ["L2 — interpretation"]
    impl["pkg/impl"]
    validate["doczcore/validate"]
  end
  subgraph L1 ["L1 — mutation primitives"]
    docwrite["doczcore/docwrite"]
    toc["doczcore/toc"]
    index["doczcore/index"]
    doctemplate["doczcore/doctemplate"]
  end
  subgraph L0 ["L0 — facts"]
    config["doczcore/config"]
    document["doczcore/document"]
    docparse["doczcore/docparse"]
  end
  cmd --> repo
  cmd --> wiki
  cmd -.->|validate, task list| impl
  cmd -.-> validate
  repo --> validate
  impl --> validate
  validate --> docparse
  validate --> document
  validate --> config
  repo --> docwrite
  repo --> toc
  repo --> index
  repo --> doctemplate
  repo --> document
  repo --> config
  wiki --> doctemplate
  wiki --> docparse
  wiki --> config
  impl --> document
  impl --> docparse
  impl --> config
  docwrite --> doctemplate
  docwrite --> docparse
  docwrite --> document
  toc --> docparse
  index --> document
  doctemplate --> config
  document --> config
```

Every edge points downward or sideways within a layer; `doczcore` has no
edge into `pkg/impl` (R2); nothing points at `internal/` because nothing is
there (R5).

### 2. Package specifications

Each subsection lists the complete exported surface after the swap.
"Unchanged" packages are listed for completeness so the API is readable in
one place; their signatures are today's.

#### 2.1 config (L0, unchanged)

```go
package config // import "github.com/donaldgifford/docz/v2/pkg/doczcore/config"

const ConfigFileName, IndexFileName, WikiIndexName, MkDocsFileName,
      TemplatesDir, DefaultChangelogFile, APILandingFileName string
const FileMode, DirMode os.FileMode

var ErrUnknownType, ErrInvalidChangelogFile, ErrInvalidAPIPath error

type Config struct { DocsDir; Types map[string]TypeConfig; Index; Author; Wiki; TOC; Changelog; API }
type TypeConfig struct { Enabled; Dir; Template; IDPrefix; IDWidth; Statuses; StatusField; PluralLabel; Aliases }
type IndexConfig, AuthorConfig, WikiConfig, TOCConfig, ChangelogConfig, APIConfig struct
type DocType, Status string
type DocTypeDef struct { Name; Aliases; DefaultConfig func() TypeConfig; NavTitle; PluralLabel; TemplateName; HelpDescription }

func DefaultConfig() Config
func Load(configFile, repoRoot string) (Config, error)
func (c *Config) Validate() error
func (c *Config) ValidateType(name string) (canonical string, err error)
func (c *Config) EnabledTypes() []string
func (c *Config) TypeDir(typeName string) string
func DocTypeNames() []string
func AllDocTypes() []DocTypeDef
func LookupDocType(name string) (DocTypeDef, bool)
func ResolveTypeAlias(name string) string
func DefaultNavTitles() map[string]string
func TypesHelp() string
```

`ValidateType` already resolves a name, an alias, or an `id_prefix` to the
canonical type, which is what `repo.Find` needs to map `IMPL-0017` to
`impl`. After ADR-0003 the registry has five entries; nothing here changes
shape. Rendering the default `.docz.yaml` is *not* added here — it needs
the template embed, which is `doctemplate`'s (§2.7).

#### 2.2 document (L0, additive)

```go
package document // import "github.com/donaldgifford/docz/v2/pkg/doczcore/document"

var DoczFilePattern *regexp.Regexp
var ErrNoFrontmatter, ErrNoVersions error

type Frontmatter struct { ID, Title string; Status config.Status; Author, Created string; Schema string } // Schema: new, optional, DESIGN-0015 §3
type DocEntry struct { Frontmatter; Filename string; Content []byte }
type Changelog, ChangelogVersion, ChangelogGroup struct

func IsDoczFile(name string) bool
func ParseFrontmatter(content []byte) (Frontmatter, error)
func LoadFrontmatter(path string) (Frontmatter, []byte, error)
func ScanDocuments(dir string) ([]DocEntry, error)
func ParseChangelog(content []byte) (*Changelog, error)
```

`DocEntry.Content` stays populated by `ScanDocuments`; `repo.Update` feeds
it to `toc.UpdateFiles` and `repo.Find` returns it, so no operation reads a
document twice.

#### 2.3 docparse (L0, additive)

```go
package docparse // import "github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"

type Heading struct { Level int; Text, Anchor string; Line int }
type TaskItem struct { Text string; Checked bool; Indent, Line int }

func Headings(content []byte) []Heading
func TaskItems(content []byte) []TaskItem
func Title(content []byte) string
func AnchorSlug(text string) string

// New (DESIGN-0015 §1): region markers, under the same contract —
// bytes in, values out, no errors, never panics, byte-accurate lines.
type Role int
type Marker struct { Kind string; Role Role; Line int; Canonical bool }
type Region struct { Kind string; Start, End, Depth int; Closed bool }
func Markers(content []byte) []Marker
func Regions(content []byte) []Region
```

`pkg/impl` builds on `Regions`, `Headings`, and `TaskItems` and re-walks raw
lines only for what they do not report (§3). `Sections` and a `document.Kind` helper
are deferred until a second type package exists (Open Question 10). The
slug and comment-heading defects in #96 are bug fixes inside the frozen
contract and are not part of this design.

#### 2.4 docwrite (L1, additive)

```go
package docwrite // import "github.com/donaldgifford/docz/v2/pkg/doczcore/docwrite"

// Existing, unchanged.
var ErrUnsupportedLineEndings, ErrStatusFieldMissing error
var ErrLineOutOfRange, ErrNotTaskItem, ErrTaskAlreadyChecked error
type CreateOptions struct { Type config.DocType; Title, Author, Status, Prefix string; IDWidth int; DocsDir, TypeDir, TemplatePath string; CreatedAt time.Time }
type CreateResult struct { FilePath, Number, Filename string }
func Create(opts *CreateOptions) (CreateResult, error)
func SetStatus(path, newStatus string) (oldStatus string, err error)
func CheckTask(path string, line int) error

// New: byte cores (R3). The path helpers above become wrappers over these.
func SetStatusBytes(doc []byte, status string) (out []byte, old string, err error)
func SetTaskStateBytes(doc []byte, line int, checked bool) ([]byte, error)
var ErrTaskAlreadyUnchecked error // SetTaskStateBytes(…, false) on an unchecked item

// New: path wrapper in both directions, for a future docz task check/uncheck.
func SetTaskState(path string, line int, checked bool) error

// New: creation without I/O (ADR-0001 Decision 5's promised name+content form).
type Rendered struct { Filename string; Content []byte }
func NextNumber(dir string, width int) (string, error)          // "0001" for a missing or empty dir
func Render(opts *CreateOptions, number string) (Rendered, error) // resolve template, build Data, render
// Create == NextNumber + Render + write, behaviour unchanged.
```

Contracts that carry over from the path helpers to the byte cores:
`SetStatusBytes` rewrites only the value bytes and preserves key order,
quoting shape, spacing, and trailing comments (DESIGN-0005); it returns
`ErrUnsupportedLineEndings` on any CR, `ErrStatusFieldMissing` for an
absent or block/flow-scalar status key, and `document.ErrNoFrontmatter`
without a frontmatter block. `SetTaskStateBytes` validates the target line
by running `docparse.TaskItems` over it (one task-item definition
module-wide) and splices a single byte. `Render` resolves the template with
`doctemplate.Resolve` and never writes; `Create` keeps its exact current
behaviour, including the auto-increment scan (Open Question 5).

#### 2.5 toc (L1, unchanged)

```go
package toc // import "github.com/donaldgifford/docz/v2/pkg/doczcore/toc"

const BeginMarker, EndMarker string
type FileInput struct { Path string; Content []byte }
type FileResult struct { Path string; Headings int }
type FileError struct { Path string; Err error }
type UpdateResult struct { Updated string; Headings []docparse.Heading; Found bool }
type UpdateReport struct { Updated, Unchanged, WouldUpdate []FileResult; Skipped []string; WriteErrors []FileError }

func GenerateToC(headings []docparse.Heading, minHeadings int) string
func UpdateToC(content string, minHeadings int) UpdateResult
func UpdateFiles(files []FileInput, minHeadings int, dryRun bool) (UpdateReport, error)
```

`repo.Update` calls `UpdateFiles` exactly as `cmd/update.go` does today and
embeds the report. The near-miss marker warning (#95) is a `Skipped`
consumer in `cmd/`, not a change here.

#### 2.6 index (L1, promoted)

Promoted whole from `internal/index` as `pkg/doczcore/index`, plus a
bytes-in splice and the scaffold `init` writes.

```go
package index // import "github.com/donaldgifford/docz/v2/pkg/doczcore/index"

const BeginMarker = "<!-- BEGIN DOCZ AUTO-GENERATED -->"
const EndMarker   = "<!-- END DOCZ AUTO-GENERATED -->"

type UpdateAction int
const (
    ActionCreated UpdateAction = iota + 1
    ActionUpdated
    ActionNoMarkers
    ActionDryRunCreated
    ActionDryRunUpdated
)
type UpdateOutcome struct { Action UpdateAction; Path, Body string }

func GenerateTable(docs []document.DocEntry, heading string) string
func Splice(existing []byte, header, table string) (out []byte, action UpdateAction) // new; nil existing → ActionCreated
func Scaffold(header string) []byte                                                   // new; header + one empty marker pair
func UpdateReadme(readmePath, header, tableContent string) (UpdateOutcome, error)     // read → Splice → write
func DryRunReadme(readmePath, header, tableContent string) (UpdateOutcome, error)     // read → Splice, no write
```

`Splice` is the byte core; the two path helpers become wrappers and keep
their outcomes byte-for-byte. `Scaffold` exists so the README `docz init`
writes and the README `docz update` splices agree on the marker block in
one place — #99's doubled markers came from the header file and the
scaffold each emitting a pair. The action enum is the typed result ADR-0001
called CLI-flavoured; under R4 it is exactly the right shape, and the
"Warning: … has no markers" sentence stays in `cmd/`.

#### 2.7 doctemplate (L1, promoted)

Promoted whole from `internal/template` as `pkg/doczcore/doctemplate`
(the alias `cmd/` already imports it under), with the `embed.FS` unexported
and template contents outside the contract.

```go
package doctemplate // import "github.com/donaldgifford/docz/v2/pkg/doczcore/doctemplate"

var ErrNoTemplate error // no embedded, on-disk, or configured template for the type
var ErrNoSchema   error // no on-disk or embedded schema of that name (DESIGN-0015 §3)

type Data struct { Number, Title, Date, Author string; Status config.Status; Type config.DocType; Prefix, Slug, Filename string }
type IndexHeaderData struct { TypeName, PluralLabel string }
type WikiIndexData struct { SiteName string; Types []WikiIndexType }
type WikiIndexType struct { Name, Label, Dir string }

func Resolve(docType, configPath, docsDir string) (string, error)                       // config path → <docsDir>/templates/<type>.md → embedded
func ResolveIndexHeader(docType, docsDir string, data IndexHeaderData) (string, error)  // on-disk override → embedded per-type → rendered generic
func ResolveWikiIndex(docsDir string) (string, error)
func Render(tmpl string, data *Data) (string, error)
func RenderWikiIndex(tmpl string, data *WikiIndexData) (string, error)
func FilenameSlug(title string) string
func EmbeddedDocumentTemplate(docType config.DocType) (string, error)
func EmbeddedWikiIndex() (string, error)
func GenericTemplate() (string, error)                    // new: embedded default.md, scaffolding for a custom type (DESIGN-0015 §3)
func ResolveSchema(name, docsDir string) ([]byte, error)  // new: <docsDir>/templates/schema/<name>.md → embedded schema/<name>.md
func EmbeddedSchema(name string) ([]byte, error)          // new: baked-in only, for consumers with no checkout
func DefaultConfigYAML() (string, error) // new: docz_yaml.tmpl rendered over config.DefaultConfig()
```

`DefaultConfigYAML` absorbs the twenty lines `cmd/init.go` spends rendering
the default config, so `docz init` and any consumer that scaffolds a repo
produce the same file from the same source of defaults. `Resolve` returns
`ErrNoTemplate` (wrapped, naming the type and the on-disk path it looked
for) when a custom type has no template — the clear error #92 asks for and
the one ADR-0003's legacy `plan:` block hits. The embedded *contents* remain
observable through `docz template show` but are not semver-governed; a
consumer that wants a template's text calls `Resolve` and treats the result
as data.

#### 2.8 repo (L3, new)

The package that makes "the CLI is one call plus printing" true. A `Repo`
is a root directory and a loaded config; every method is the orchestration
one `cmd/` handler performs today, with the printing removed.

```go
package repo // import "github.com/donaldgifford/docz/v2/pkg/doczcore/repo"

type Repo struct {
    Root string         // repository root; every config-relative path is joined under it
    Cfg  *config.Config // loaded and validated by the caller
}

// Every method takes a context (rule R8, §7): checked between per-type
// iterations, passed to hooks, never stored.
func Open(ctx context.Context, root, configFile string) (*Repo, error) // config.Load(configFile, root); a struct literal is equally valid

// Paths (pure).
func (r *Repo) Path(rel string) string           // filepath.Join(Root, rel)
func (r *Repo) TypeDir(typeName string) string   // Path(Cfg.TypeDir(typeName))
func (r *Repo) ReadmePath(typeName string) string

// Reads.
func (r *Repo) Scan(ctx context.Context, typeName string) ([]document.DocEntry, error)  // ValidateType → ScanDocuments(TypeDir)
func (r *Repo) List(ctx context.Context, types []string) ([]Entry, error)              // nil types → Cfg.EnabledTypes()
func (r *Repo) Find(ctx context.Context, id string) (Entry, error)                     // type from the ID prefix, then FindIn
func (r *Repo) FindIn(ctx context.Context, typeName, id string) (Entry, error)         // frontmatter id, case-sensitive

// Writes.
func (r *Repo) Create(ctx context.Context, opts CreateOptions) (CreateResult, error)
func (r *Repo) Update(ctx context.Context, types []string, opts UpdateOptions) (UpdateReport, error)
func (r *Repo) SetStatus(ctx context.Context, typeName, id, status string, opts StatusOptions) (StatusResult, error)
func (r *Repo) Init(ctx context.Context, opts InitOptions) (InitReport, error)

// Validation and migration (DESIGN-0015 §4, §6; types there).
func (r *Repo) Validate(ctx context.Context, types []string, opts ValidateOptions) (ValidateReport, error)
func (r *Repo) InsertRegions(ctx context.Context, types []string, opts InsertRegionsOptions) (InsertRegionsReport, error)

// Templates.
func (r *Repo) Template(ctx context.Context, typeName string) (string, error)                                     // doctemplate.Resolve with the type's config path
func (r *Repo) ExportTemplate(ctx context.Context, typeName, dest string, opts ExportOptions) (ExportResult, error) // dest "" → <DocsDir>/templates/<type>.md (override)

// Hooks (§7): optional callbacks carried in the context, the httptrace pattern.
type Hooks struct { /* ScanStart, ScanDone, TypeSkipped, FileWritten, FileSkipped */ }
func WithHooks(ctx context.Context, h *Hooks) context.Context
```

```go
type Entry struct {
    document.DocEntry
    Type string // canonical type name
    Path string // repo-relative, e.g. "docs/impl/0017-….md"
}

type CreateOptions struct {
    Type   string    // any token ValidateType accepts: name, alias, id_prefix
    Title  string
    Author string    // resolved by the caller — git is an L4 dependency
    Status string    // "" → the type's first status
    Now    time.Time // zero → time.Now()
    Update bool      // run Update for the type afterwards
}
type CreateResult struct {
    Type string
    docwrite.CreateResult          // FilePath, Number, Filename
    Update *TypeReport             // nil when Update was false
}

type UpdateOptions struct { DryRun bool }
type UpdateReport struct { Types []TypeReport }
type TypeReport struct {
    Type    string
    Dir     string             // repo-relative type dir
    Docs    int
    ToC     *toc.UpdateReport  // nil when Cfg.TOC.Enabled is false
    Index   index.UpdateOutcome
    Elapsed time.Duration      // wall time for this type — cheap profiling data
}

type StatusOptions struct { DryRun bool }
type StatusResult struct {
    ID, Type, Path string
    Old, New      string
    Changed       bool // false when Old == New (no write) or DryRun
}

type InitOptions struct { Force bool }
type InitAction int
const ( InitCreated InitAction = iota + 1; InitSkipped; InitOverwritten )
type InitFile struct { Path string; Action InitAction }
type InitReport struct { Files []InitFile } // .docz.yaml, each type dir, each README

type ExportOptions struct { Overwrite bool }
type ExportResult struct { Path string; Overwritten bool; Scaffolded bool; SchemaPath string } // Scaffolded: no template resolved, the generic pair was written and SchemaPath names the schema file (DESIGN-0015 §3)

// Typed errors everywhere (Open Question 9, resolved b): every failure a
// caller may branch on carries its facts, and cmd/ maps them to exit codes
// with errors.As. UnknownTypeError wraps config.ErrUnknownType so the frozen
// sentinel still answers errors.Is.
type NotFoundError      struct { Type, ID string }                        // Find, FindIn, SetStatus
type TypeDisabledError  struct { Type string }                            // Scan, Create on a disabled type
type ExistsError        struct { Path string }                            // Init without Force, ExportTemplate without Overwrite
type InvalidStatusError struct { Type, Status string; Allowed []string }
type UnknownTypeError   struct { Token string; Valid []string }           // Unwrap() returns config.ErrUnknownType
type WriteError         struct { Path string; Err error }                 // Unwrap() returns Err
```

What each write does, in order:

```mermaid
sequenceDiagram
  participant cmd as cmd/create.go
  participant R as repo.Create
  participant C as config
  participant W as docwrite
  participant U as repo.Update
  cmd->>R: Create(ctx, {Type:"adr", Title, Author, Update:true})
  R->>C: ValidateType("adr") → "adr", then Types["adr"].Enabled?
  R->>W: Create(&CreateOptions{…, CreatedAt: Now})
  W-->>R: CreateResult{FilePath, Number, Filename}
  R->>U: Update(["adr"], {})
  U-->>R: TypeReport{ToC, Index}
  R-->>cmd: CreateResult{Type, CreateResult, Update: &TypeReport}
  cmd->>cmd: print "Created ADR: …" then the update lines
```

```mermaid
sequenceDiagram
  participant cmd as cmd/update.go
  participant R as repo.Update
  participant D as document
  participant T as toc
  participant H as doctemplate
  participant X as index
  cmd->>R: Update(ctx, ["adr"], {DryRun})
  loop each type
    R->>R: ctx.Err()? return the partial report
    R->>D: ScanDocuments(TypeDir)
    D-->>R: []DocEntry (Content cached)
    opt Cfg.TOC.Enabled
      R->>T: UpdateFiles(files, MinHeadings, DryRun)
      T-->>R: toc.UpdateReport
    end
    R->>X: GenerateTable(docs, "All "+label)
    R->>H: ResolveIndexHeader(type, DocsDir, {TypeName, PluralLabel})
    R->>X: UpdateReadme | DryRunReadme (ReadmePath, header, table)
    X-->>R: UpdateOutcome
  end
  R-->>cmd: UpdateReport{Types}
  cmd->>cmd: one line per report entry
```

```mermaid
sequenceDiagram
  participant cmd as cmd/status.go
  participant R as repo.SetStatus
  participant W as docwrite
  cmd->>R: SetStatus(ctx, "design", "DESIGN-0013", "Abandoned", {DryRun})
  R->>R: FindIn → Entry (NotFoundError → exit 1 in cmd)
  R->>R: status ∈ Types[t].Statuses? (InvalidStatusError → exit 2)
  alt Old == New or DryRun
    R-->>cmd: StatusResult{Changed:false | DryRun}
  else
    R->>W: SetStatus(Path, New)
    W-->>R: old
    R-->>cmd: StatusResult{Changed:true}
  end
```

Rules the package keeps: `List` and `Update` with `nil` types iterate
`Cfg.EnabledTypes()` (built-ins first, custom sorted), so custom types are
never skipped; `Scan` on a disabled type returns a `TypeDisabledError`
rather than an empty slice, which is what `cmd/update.go`'s debug "type
disabled, skipping" branch tests today; `Init` writes `.docz.yaml` via
`doctemplate.DefaultConfigYAML`, creates each enabled type's directory, and
writes `index.Scaffold(header)` per README, reporting one `InitFile` each;
`ExportTemplate` with an empty destination is `docz template override`.
`Repo` holds no logger and prints nothing (R4); the debug narration `cmd/`
emits today arrives live through the hooks in §7, and every method takes a
context it checks between types (rule R8), returning the report completed
so far with the context's error.

#### 2.9 impl (L2, new)

Carried over from DESIGN-0013 §5 with the root type renamed. Spans are
located by region (DESIGN-0015 §5), never by heading text. `Parse` is keyed
on the `phase`, `tasks`, and `criteria` kinds and never reads the type name
(ADR-0002 R7 as restated under its Open Question 5): a custom type whose
documents carry those kinds parses with it, and `docz task list` accepts
its IDs.

```go
package impl // import "github.com/donaldgifford/docz/v2/pkg/impl"

// Parse interprets an IMPL document. It never touches the filesystem.
func Parse(doc []byte) (Doc, error)

type Doc struct {
    ID     string        // frontmatter id, e.g. "IMPL-0017"
    Title  string        // frontmatter title
    Status config.Status // frontmatter status, typed like document.Frontmatter
    Phases []Phase
}

type Phase struct {
    Index       int         // 1-based ordinal among phases, document order
    Token       string      // heading token: "1", "A", "2B" — equals Index for template docs
    Title       string      // text after "Phase <token>:", inline markdown stripped
    Description string      // prose between the phase heading and its first nested region, trimmed, comments removed
    Tasks       []Task
    Criteria    []Criterion // nil when the phase has no criteria region
    Line        int         // heading line, 1-based
}

type Task struct {
    ID       string  // "<phase token>.<index>", e.g. "2.3"; index is 1-based, document order
    Text     string  // folded across continuation lines; verify line and markers removed
    Checked  bool
    Verify   string  // command from the task's verify line, "" when absent
    Deferred *Marker // nil unless a deferred marker is present
    Skipped  *Marker // nil unless the task is struck through with a skipped note
    Line     int     // the checkbox line, byte-accurate — the splice target
    EndLine  int     // last continuation line (== Line for one-liners)
}

type Marker struct {
    Note string // reason text, "" when the marker carries none
    Line int    // line the marker sits on
}

type Criterion struct {
    Text       string
    Executable bool   // the bullet starts with a backtick span
    Command    string // the span's contents when Executable
    Line       int
}

func (d Doc) Task(id string) (Task, bool)
func (d Doc) Phase(token string) (Phase, bool)
func (d Doc) Tasks() []Task              // every task across phases, document order
func (d Doc) Progress() (done, total int) // checked over non-skipped

var ErrNoPhases = errors.New("impl: no phases found")
type DuplicatePhaseError struct { Token string; Lines []int }

// Validate reports IMPL-specific findings over Parse (DESIGN-0015 §4);
// the generic checks are validate.Document's, and cmd/ composes both.
func Validate(doc []byte) []validate.Finding
```

Every `Line` is byte-accurate against the input (the `docparse` contract),
so a consumer can splice at `Task.Line` with `docwrite.SetTaskStateBytes`,
or append a marker after `Task.EndLine` with its own code, and stay
byte-minimal. `Doc` holds no reference to the input; every string is
copied (the `ParseChangelog` retention lesson).

```mermaid
sequenceDiagram
  participant C as consumer
  participant I as pkg/impl
  participant D as doczcore/document
  participant P as doczcore/docparse
  C->>I: Parse(doc []byte)
  I->>D: ParseFrontmatter(doc)
  D-->>I: Frontmatter{ID, Title, Status}
  I->>P: Headings(doc)
  P-->>I: []Heading{Level, Text, Line}
  I->>P: TaskItems(doc)
  P-->>I: []TaskItem{Text, Checked, Indent, Line}
  I->>I: phase spans → task spans → fold continuations → markers → criteria
  I-->>C: Doc
```

The convention for the next type package is this package: `pkg/<type>`,
`Parse([]byte) (Doc, error)`, value-typed helpers, its own sentinel errors,
imports only L0, no interface. A consumer that wants "parse whatever this
is" maps `Frontmatter.ID`'s prefix to a type with `config.ValidateType`
and switches — still concrete types.

#### 2.10 wiki (integration, promoted)

Promoted whole from `internal/wiki` as `pkg/wiki`, a sibling of `pkg/impl`
rather than part of `doczcore`: it is an integration with MkDocs and
TechDocs, not core and not a type. The existing primitives keep their
signatures; two orchestration functions absorb `cmd/wiki.go`.

```go
package wiki // import "github.com/donaldgifford/docz/v2/pkg/wiki"

// Existing primitives, unchanged.
type MkDocsConfig struct { SiteName, SiteDescription, DocsDir, RepoURL, SiteURL, Theme string; Plugins, MarkdownExtensions []string }
type NavEntry struct { Title, Path string; Children []NavEntry }
func CreateMkDocs(path string, cfg *MkDocsConfig) error
func ReadMkDocs(path string) (map[string]any, error)
func WriteMkDocs(path string, data map[string]any) error
func ScanDocs(docsDir string, exclude []string, navTitles map[string]string) ([]NavEntry, error)
func BuildNav(docsDir string, exclude []string, navTitles map[string]string, existingOrder []string) ([]NavEntry, error)
func SortEntries(entries []NavEntry) []NavEntry
func MergeNavOrder(existing []string, newEntries []NavEntry) []NavEntry
func ExistingNavOrder(data map[string]any) []string
func NavToYAML(entries []NavEntry) []any
func CountPages(entries []NavEntry) int
func DocTitle(filePath string) (string, error)   // frontmatter "ID: Title" → docparse.Title → FilenameTitle
func DirTitle(dir string, navTitles map[string]string) string
func FilenameTitle(filename string) string

// New: the orchestration cmd/wiki.go performs today.
type Action int
const ( Created Action = iota + 1; Skipped; Overwritten )
type InitOptions struct { SiteName, SiteDescription, RepoURL, SiteURL, Theme string; Force bool }
type InitReport struct { MkDocsPath string; MkDocs Action; IndexPath string; Index Action }
func Init(ctx context.Context, root string, cfg *config.Config, opts InitOptions) (InitReport, error) // mkdocs.yml + <DocsDir>/index.md from ResolveWikiIndex

type NavOptions struct { DryRun bool }
type NavReport struct { Path string; Entries []NavEntry; Pages int; Written bool }
func UpdateNav(ctx context.Context, root string, cfg *config.Config, opts NavOptions) (NavReport, error) // Read → ExistingNavOrder → BuildNav → NavToYAML → Write
```

What stays in `cmd/wiki.go`: defaulting the site name from the git remote
(an L4 dependency like author resolution), the `.docz.yaml`-exists
precondition, and the printed nav tree. `repo.Create` does **not** call
`wiki.UpdateNav`; `cmd/create.go` keeps that call behind
`Cfg.Wiki.AutoUpdate` so `doczcore` never imports `pkg/wiki` (R1 —
`wiki` is a sibling, not below `repo`). Open Question 8 asks whether the two
orchestration functions belong here at all.

#### 2.11 validate (L2, new)

Specified in DESIGN-0015 §3 and §4; listed here so the unit reads in one
place.

```go
package validate // import "github.com/donaldgifford/docz/v2/pkg/doczcore/validate"

type Severity int
const ( Error Severity = iota + 1; Warning )
type Finding struct { Code string; Severity Severity; Line int; Kind, Detail string }
type SchemaRegion struct { Kind, Parent string }
type Schema struct { Regions []SchemaRegion }
type Options struct { Schema Schema; Type config.TypeConfig; Filename string }

func SchemaFromMarkers(b []byte) Schema // a skeleton, a template, or a document: only its markers count
func Document(content []byte, opts Options) []Finding
```

Type-agnostic and importing only L0. The schema is a marker skeleton the
document names in frontmatter, baked in per built-in type or a file under
`templates/schema/`, resolved by the caller through `doctemplate` and
passed in `Options` (DESIGN-0015 §3); `Document` never resolves anything
itself. `impl.Validate` returns the same `Finding`;
`repo.Validate` runs `Document` over a tree and adds the ToC and index drift
checks; `cmd/validate.go` composes the three because the core cannot import
a type package (R2).

### 3. The IMPL grammar

The grammar is a contract over the `phase`, `tasks`, and `criteria` kinds
(R7). Spans come from regions (DESIGN-0015 §5); which kinds a document must
carry is its schema's to say (DESIGN-0015 §3), and the content rules
inside a region belong to this package, so neither a template override nor
a schema changes them. The tolerances for the fleet's hand-written
documents (INV-0010) are unchanged.

| Element | Rule | Source |
| ------- | ---- | ------ |
| Frontmatter | `document.ParseFrontmatter`; `ErrNoFrontmatter` is fatal | facts layer |
| Phase | A `phase` region at depth 0. The first level-3 heading inside it is the phase heading; its text, with inline markdown and HTML comments stripped, matches `^Phase\s+([^\s/:]+):\s*(.*)$` for the token and title, else the `impl.phase.no-heading` finding. A title empty after stripping (the template's placeholder is a comment) is the `impl.phase.no-title` warning. Headings outside a region (`### Phase 1` under File Changes) are never phases. | DESIGN-0015 §5; the regex now reads the token, it no longer finds the span; `impl.md` line 39 |
| Phase token and ID | Token is capture 1; duplicate tokens → `DuplicatePhaseError`; `Index` is the 1-based ordinal | the corpus is numeric and contiguous |
| Description | Lines strictly between the phase heading and the first depth-1 region inside the phase, HTML comments removed, trimmed | template guidance lives in comments |
| Tasks span | The `tasks` region at depth 1 inside the phase; a phase without one yields the `impl.phase.no-tasks` finding and no tasks | DESIGN-0015 §5 |
| Task | A `docparse.TaskItem` with `Indent == 0` inside the tasks span; nested items are never tasks | INV-0010 Obs 3 |
| Continuation | Following lines that are non-blank, indented deeper than the bullet, and not themselves list items belong to the task; `EndLine` is the last such line | 48 of 56 tasks wrap in IMPL-0017 |
| Text | Continuation lines joined with single spaces after trimming; the verify line and marker text removed; other inline markdown kept verbatim | consumers match on Text |
| Verify | A continuation line whose trimmed text starts with `verify:` case-insensitively; `Verify` is the contents of the first backtick span on that line; the rest of the line is ignored | docz-api IMPL-0004 |
| Deferred | The token `deferred` followed by a hyphen, en dash, or em dash, on the task line or a continuation, optionally inside bold; `Note` is the text after the dash, after an optional `human required:`, folded to the task's end | docz-api IMPL-0006 prefix form; issue #100 suffix form |
| Skipped | Task text wrapped in `~~…~~` followed by `skipped:` after any dash; `Note` is the text after the colon | issue #100 |
| Criteria | Top-level dash bullets inside the phase's `criteria` region, wrapped lines folded, an optional leading checkbox tolerated; `Executable` iff the bullet text starts with a backtick span; no region means `Criteria == nil` | issue #100; INV-0010 Obs 5 caveat |
| Outside phases | Any checkbox outside a `tasks` region is not a task; the `testing` region needs no special rule | DESIGN-0015 §5 |
| Fences | Inherited from `docparse`: nothing inside a fence is a heading or a task | facts layer |
| Line endings | LF only; any CR → error, matching `docwrite` | DESIGN-0005 Decision 7 |

**Continuation folding, worked** (Open Question 2). This repo wraps markdown
at 80 columns, so most task bullets run onto indented follow-on lines, and
`docparse.TaskItems` reports only the first line of each. `impl` folds the
rest:

```markdown
- [ ] Add `docparse.Regions` with fence-aware marker matching and
      byte-accurate lines, following the `Headings` walker.
      Verify: `go test ./pkg/doczcore/docparse/...` passes
- [ ] Document the walker — deferred: needs the kind catalogue first
```

That yields two tasks. The first has `Text` "Add `docparse.Regions` with
fence-aware marker matching and byte-accurate lines, following the
`Headings` walker.", `Verify` "go test ./pkg/doczcore/docparse/...",
`Line` 1, and `EndLine` 3. The second has `Text` "Document the walker",
`Deferred.Note` "needs the kind catalogue first", and `Line` and `EndLine`
both 4. A follow-on line counts when it is non-blank, indented deeper than
the bullet, and not itself a list item; a blank line or a nested list item
ends the task. The verify line is matched case-insensitively, its first
backtick span is the command, and prose after the span ("passes") is
ignored rather than rejected, because docz-api's IMPL-0004 wrote four such
lines by hand and they must parse.

The line walker inside the tasks span is a small state machine; it is the
only place `impl` reads raw lines:

```mermaid
stateDiagram-v2
  direction LR
  [*] --> Between
  Between --> Task: checkbox item, Indent 0
  Task --> Continuation: deeper-indented non-list line
  Continuation --> Continuation: another such line
  Continuation --> Task: next checkbox item
  Continuation --> Between: blank, list item, or heading
  Task --> Task: next checkbox item
  Task --> Between: blank, list item, or heading
  Between --> [*]: end of tasks span
  note right of Continuation
    verify line → Task.Verify
    deferred / skipped → Marker
    otherwise folded into Text
    EndLine advances
  end note
```

Identity and stability, which consumers rely on for structural-change
detection:

| Key | Stable across | Shifts when |
| --- | ------------- | ----------- |
| `Phase.Token` | any edit that keeps the heading | the heading's token is edited |
| `Task.ID` | checkbox flips, marker edits, text edits, skips | a task is inserted, removed, or moved within its phase |
| `Task.Line` and `EndLine` | nothing above the task changes | any earlier line-count change — re-parse before splicing |

Skipped tasks remain checkbox items, so positional IDs do not shift when a
task is skipped. A consumer that wants "what changed between two runs"
parses twice and compares by `Task.ID`; docz does not diff.

### 4. The cmd swap

The final step. Every handler becomes flags → one call → print, and the
existing `cmd/` tests must pass unchanged, byte-for-byte, on the new code
paths. `Runner` gains a `Repo *repo.Repo` built in `loadAndValidateConfig`
from `RepoRoot` and the loaded config; tests that construct a `Runner`
directly build one the same way. Every handler derives its context from
the process signal context and attaches hooks that carry today's debug
narration to the logger (§7), so `--verbose` output is unchanged.

The acceptance test is functional parity with v1.2.2 (ADR-0002 Decision
6), run across every built-in type. `test/parity/` holds one fixture repo
per type — rfc, adr, design, impl, investigation — plus one custom type
declared in `.docz.yaml` and one legacy `plan:` block (ADR-0003), each with
a few documents. A driver runs every command in the table below against
each fixture — `init`, `create`, `update` with and without `--dry-run`,
`list` in text, json, and csv, `status set` including its error paths,
`template show`/`export`/`override`, `config`, `wiki init`/`update` — and
records stdout, stderr, the exit code, and every file written or changed.
The goldens are captured once from the released v1.2.2 binary, before any
template gains a marker, and checked in; `make parity` replays them
against the freshly built binary and joins `make ci` in the swap PR. Two
deltas are permitted and live in the comparison, not in the goldens:
`<!--docz:…-->` lines are ignored in `create` and `template` output,
because the templates gain region markers (DESIGN-0015), and `validate`,
`update --regions`, and `task list` have no v1.2.2 golden. The legacy
`plan:` fixture is compared for every command except `create plan`, whose
v2 behaviour — the no-template error — is pinned by its own test
(ADR-0003 Decision 3). Anything else that differs blocks the PR. The suite
is the proof that the API reproduces the CLI; the `cmd/` unit tests pin
behaviour per handler and do not cross types.

| Command | Library call | What stays in `cmd/` |
| ------- | ------------ | -------------------- |
| `docz init` | `repo.Init(InitOptions{Force})` | one line per `InitFile` |
| `docz create <type> <title>` | `repo.Create(CreateOptions{Type, Title, Author, Status, Now, Update: !noUpdate})`, then `wiki.UpdateNav` if `Wiki.AutoUpdate` | author resolution via `GitResolver`, printing |
| `docz update [type]` | `repo.Update(types, UpdateOptions{DryRun})` | wording per `TypeReport`; the #95 near-miss warning over `ToC.Skipped` |
| `docz list [type]` | `repo.List(types)` | `--status` filter, text/json/csv rendering of `listEntry` |
| `docz status set <type> <id> <status>` | `repo.SetStatus(ctx, type, id, status, StatusOptions{DryRun})` | `errors.As` → exit 1 (`NotFoundError`, `WriteError`) or exit 2 (`UnknownTypeError`, `InvalidStatusError`, `ErrUnsupportedLineEndings`); text/json |
| `docz template show <type>` | `repo.Template(type)` | printing |
| `docz template export <type> [path]` | `repo.ExportTemplate(type, path, ExportOptions{})` | printing |
| `docz template override <type>` | `repo.ExportTemplate(type, "", ExportOptions{})`; a custom type with no template gets the generic template and schema pair (DESIGN-0015 §3) | printing |
| `docz config` | `config.Load` (via `Open`) | YAML printing |
| `docz wiki init` | `wiki.Init(root, cfg, InitOptions{…})` | site-name default from git, `.docz.yaml` precondition, printing |
| `docz wiki update` | `wiki.UpdateNav(root, cfg, NavOptions{DryRun})` | nav tree printing |
| `docz validate [type]` (DESIGN-0015 §4) | `repo.Validate(ctx, types, ValidateOptions{Strict})`, then `impl.Validate` per IMPL entry | composing the tiers, text/json, exit codes |
| `docz update --regions` (DESIGN-0015 §6) | `repo.InsertRegions(ctx, types, InsertRegionsOptions{DryRun})` | printing |
| `docz task list <impl-id>` (ADR-0002 OQ 4) | `repo.Find(id)` then `impl.Parse(entry.Content)` | text/json rendering of tasks |
| `docz version` | — | all |

Removed from `cmd/` by the swap: `updateType`, `runToCUpdate`,
`indexLabel`, `writeDefaultConfig`, `writeIndexReadme`, `findByID`,
`updateWikiNav`, `updateWikiNavDryRun`, `ensureDocsIndex`, and the
type-validation and status-validation branches of `create` and
`status set`. Kept: `Runner`, `GitResolver`, `buildLogger`, `exitCodeFor`,
every `print*`/`emit*`/`format*` function, and the flag definitions.
Expected size: `cmd/` non-test lines fall from about 4 600 to roughly half.

### 5. Consumer map

| Package | docz CLI | tempy | sdk-booty-sh doczwork | docz-api |
| ------- | :------: | :---: | :-------------------: | :------: |
| `config` | yes | statuses, prefix → type | yes | yes |
| `document` | yes | via `impl` | yes | yes (`ParseFrontmatter`) |
| `docparse` | via `toc`, `wiki` | via `impl` | yes | yes |
| `docwrite` | yes | `SetTaskStateBytes`, `SetStatusBytes` | `SetTaskStateBytes` | — |
| `toc` | yes | — | — | — |
| `index` | yes | — | — | maybe (`GenerateTable` for its own indexes) |
| `doctemplate` | yes | — | — | `EmbeddedSchema` (DESIGN-0015 §3) |
| `repo` | yes | — | — | — (no checkout) |
| `impl` | `task list` | yes | yes (migrates off its own model) | maybe (progress rendering) |
| `wiki` | yes | — | — | — |
| `validate` | yes | maybe (workspace gate) | — | yes (ingest warnings) |

The CLI is the consumer this unit is built for and validated against. The
other three columns are awareness, not scope: the packages each external
consumer relies on today, or would, kept so the API is not shaped in a way
that strands them. None of them is part of this work. tempy's adoption,
and any move of docz-api or sdk-booty-sh off v1, happens in that repo
after the API and the CLI are done.

```mermaid
flowchart LR
  cli["docz CLI"] --> repo & wiki & impl & validate
  tempy["tempy (GitHub API, no checkout)"] --> impl & docwrite
  booty["sdk-booty-sh doczwork"] --> impl & docwrite
  api["docz-api (no checkout)"] --> document & docparse & config & validate & doctemplate
  api -.-> index & impl
  repo --> docwrite & toc & index & doctemplate & document & config
  impl --> document & docparse & config
```

No consumer needs a package it does not import: a binary that wants only
the IMPL model compiles `impl`, `validate` (for the `Finding` type),
`docwrite`, `doctemplate` (through `docwrite.Create`'s dependency — the
one ride-along ADR-0001 accepted), `document`, `docparse`, `config`, and
`yaml.v3`. Nothing from `repo`, `index`, `toc`, or `wiki`, and no
telemetry module from anywhere.

### 6. Enforcing the layer rules

No depguard for now (DESIGN-0013 OQ 13, resolved in review). Enforcement is
three things that already exist or fall out of the swap: reviewers read
imports against §1; `test/consumer` imports every `pkg/` package from
outside the module so a package that quietly depends on `cmd/` or
`internal/` fails to build there; and after the swap `internal/` is empty,
so an import of it is a compile error. A `depguard` rule can be added when
the tree is stable if drift appears.

### 7. Context, cancellation, and hooks

**Rule R8: context enters at L3.** Every function in `repo` and `wiki`
takes a `context.Context` first. Nothing in L0–L2 does: a bytes-in function
neither blocks nor touches the filesystem, so there is nothing to cancel
and no span to attach, and the frozen packages could not change anyway.
The new L1 path helpers stay context-free to match their frozen siblings;
a single-file read-modify-write has no cancellation point worth a
parameter. The reason to decide this now is mechanical: `repo` and `wiki`
are experimental until the swap, so a context parameter costs nothing
today and is a breaking change the day after.

What the library does with the context: checks it between per-type
iterations in `Update`, `List`, `Validate`, `InsertRegions`, and `Init`,
returning the report completed so far together with `ctx.Err()`; reads
hooks from it; passes it to nothing else, because nothing below L3 takes
one. What the library does not do: import a telemetry module, log, or
print. The public core stays stdlib plus `yaml.v3` (ADR-0001 Neutral), and
tracing and logging policy is the consumer's (ADR-0002 Decision 5). The
library is **traceable, not tracing**.

Hooks are the `net/http/httptrace.ClientTrace` pattern: a struct of
optional callbacks carried in the context, called synchronously when
non-nil, never retained. A struct of funcs, not an interface (R6).

```go
package repo

type FileKind int   // FileToC, FileIndex, FileDocument, FileConfig
type SkipReason int // SkipTypeDisabled, SkipExists, SkipNoMarkers, SkipNotDoczFile

// Hooks carries optional callbacks. Every field may be nil.
type Hooks struct {
    ScanStart   func(typeName, dir string)
    ScanDone    func(typeName string, docs int)
    TypeSkipped func(typeName string, reason SkipReason)
    FileWritten func(path string, kind FileKind)
    FileSkipped func(path string, reason SkipReason)
}

func WithHooks(ctx context.Context, h *Hooks) context.Context
func HooksFrom(ctx context.Context) *Hooks // never nil; the zero value is all no-ops
```

The five events are exactly the debug lines `cmd/update.go`, `cmd/init.go`,
and `cmd/create.go` emit today, so the swap wires them back one-to-one:

```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
defer stop()
ctx = repo.WithHooks(ctx, &repo.Hooks{
    ScanStart: func(_, dir string) { r.Logger.Debug("scanning type", "dir", dir) },
    ScanDone:  func(t string, n int) { r.Logger.Debug("scan complete", "type", t, "count", n) },
})
rep, err := r.Repo.Update(ctx, types, repo.UpdateOptions{DryRun: dryRun})
```

Reports carry `Elapsed` durations per type and per operation, which is the
profiling data a consumer wants without any callback. `pprof` labels ride
on the context automatically for a consumer that uses them.

```mermaid
flowchart TB
  subgraph L4 ["L4 consumer: cmd/, docz-api, tempy"]
    src["context source: signal, request, activity"]
    hooks["Hooks wired to slog, spans, heartbeat"]
  end
  subgraph L3 ["L3 repo and wiki: context enters here (R8)"]
    op["Update · Validate · Create · Init · UpdateNav"]
    chk["ctx.Err() between types"]
    fire["call each non-nil hook"]
  end
  subgraph pure ["L0–L2 impl, validate, docparse, document, docwrite byte cores"]
    fn["bytes in, values out: no context, no hooks"]
  end
  hooks -.-> src
  src -->|"WithHooks(ctx, hooks)"| op
  op --> chk
  op --> fire
  op --> fn
```

The three flows the design must support today:

```mermaid
sequenceDiagram
  participant U as user
  participant cmd as cmd/update.go
  participant R as repo.Update
  participant H as Hooks (slog debug)
  U->>cmd: docz update --verbose
  cmd->>cmd: ctx from signal.NotifyContext, then WithHooks(ctx, slog hooks)
  cmd->>R: Update(ctx, nil, {})
  loop each enabled type
    R->>R: ctx.Err()? stop with the partial report
    R->>H: ScanStart(type, dir)
    H-->>U: DEBUG scanning type dir=docs/adr
    R->>R: scan, ToC pass, index pass
    R->>H: ScanDone(type, n) and FileWritten(path, kind)
    H-->>U: DEBUG scan complete type=adr count=12
  end
  R-->>cmd: UpdateReport
  cmd->>U: Updated docs/adr/README.md … (wording from the report)
  Note over U,R: Ctrl-C cancels ctx. Update returns after the current type with ctx.Err().
```

```mermaid
sequenceDiagram
  participant C as client
  participant A as docz-api handler
  participant G as GitHub API
  participant V as validate.Document
  participant I as impl.Parse
  participant L as slog + OTel
  C->>A: POST /ingest {repo, path, ref}
  A->>L: start span "docz.ingest" on the request ctx
  A->>G: fetch bytes at ref
  G-->>A: content
  A->>V: Document(content, {Schema, Type, Filename})
  V-->>A: []Finding
  A->>I: Parse(content)
  I-->>A: Doc, or an error
  A->>L: InfoContext(ctx, "findings", codes…), handler attaches trace_id
  A-->>C: 200 with findings, or 422 when Errors > 0
  Note over A,I: pure calls take no context. The caller's span already covers them.
```

```mermaid
sequenceDiagram
  participant T as tempy activity (Temporal worker)
  participant R as repo.Validate
  participant H as Hooks (heartbeat + slog)
  participant W as workspace checkout
  T->>T: ctx from the activity: timeout, cancellation, OTel interceptor span
  T->>T: WithHooks(ctx, heartbeat on ScanDone)
  T->>R: Validate(ctx, ["impl"], {Strict})
  loop each document
    R->>W: read, validate.Document, ToC drift
    R->>H: ScanDone / FileSkipped
    H->>T: activity.RecordHeartbeat(ctx)
  end
  alt activity timeout
    T-->>R: ctx cancelled
    R-->>T: partial ValidateReport, ctx.Err()
    T->>T: retry or fail per tempy policy
  else completed
    R-->>T: ValidateReport
    T->>T: gate: Errors == 0 before the next task
  end
```

| Consumer | Context source | Hooks wired to | Span |
| -------- | -------------- | -------------- | ---- |
| docz CLI | `signal.NotifyContext` | `slog` at debug, the lines `cmd/` logs today | none; a 50 ms process has nothing to trace |
| docz-api | request context | `slog` with the handler's `trace_id`, or nothing | its own, around each library call |
| tempy | activity context | heartbeat plus `slog` | Temporal's OTel interceptor, already in the context |

`wiki.Init` and `wiki.UpdateNav` take a context for cancellation and return
reports; whether they need hooks of their own is Open Question 12.

## API / Interface Changes

| Package | Change | Kind | Frozen from |
| ------- | ------ | ---- | ----------- |
| `pkg/doczcore/config` | none | — | v1.0.0 |
| `pkg/doczcore/document` | `Frontmatter.Schema` (DESIGN-0015 §3) | additive | v1.0.0 (existing), v2.0.0 (new) |
| `pkg/doczcore/docparse` | `Markers`, `Regions`, `Marker`, `Region`, `Role` (DESIGN-0015); #96 fixes are bugs | additive | v1.0.0 (existing), v2.0.0 (new) |
| `pkg/doczcore/docwrite` | `SetStatusBytes`, `SetTaskStateBytes`, `SetTaskState`, `NextNumber`, `Render`, `Rendered`, `ErrTaskAlreadyUnchecked` | additive | v1.0.0 (existing), v2.0.0 (new) |
| `pkg/doczcore/toc` | none | — | v1.0.0 |
| `pkg/doczcore/index` | promoted whole; `Splice`, `Scaffold`, marker constants exported | new public | v2.0.0 |
| `pkg/doczcore/doctemplate` | promoted whole; `DefaultConfigYAML`, `ErrNoTemplate`; `ResolveSchema`, `EmbeddedSchema`, `GenericTemplate`, `ErrNoSchema` and the embedded schema skeletons (DESIGN-0015 §3) | new public | v2.0.0 |
| `pkg/doczcore/validate` | new (DESIGN-0015) | new public | v2.0.0 |
| `pkg/doczcore/repo` | new; every method takes a context; `Hooks`; `Validate`, `InsertRegions` | new public | v2.0.0 |
| `pkg/impl` | new; `Parse` over regions; `Validate` | new public | v2.0.0 |
| `pkg/wiki` | promoted whole; `Init`, `UpdateNav` with a context, options and reports | new public | v2.0.0 |
| embedded templates | region markers added (DESIGN-0015 §2) | contents, not contract | — |
| `internal/` | emptied | — | — |
| `cmd/` | re-pointed; `validate`, `update --regions`; optional `task list` | new commands only, existing behaviour unchanged | — |
| `.docz.yaml` | none | — | — |
| `config.DocTypeNames()` | loses `plan` (ADR-0003) | catalogue change, same release | — |
| module path | `github.com/donaldgifford/docz` → `github.com/donaldgifford/docz/v2` (ADR-0002 Decision 3); v1.x tags keep the old path | major | v2.0.0 |

Per ADR-0002 Decision 7, every "v2.0.0" row is experimental until v2.0.0
proper ships: its package doc comment opens with `EXPERIMENTAL` and a link
to ADR-0002, and it may change between betas. This unit ends at
`v2.0.0-beta.1` with the markers still in place; the v2.0.0 cut, after
docz-api and the UI move into the repo, removes them and the whole table
is contract from then on.

## Data Model

```mermaid
classDiagram
  class Repo {
    Root string
    Cfg *config.Config
    Scan(type) []DocEntry
    List(types) []Entry
    Find(id) Entry
    FindIn(type, id) Entry
    Create(CreateOptions) CreateResult
    Update(types, UpdateOptions) UpdateReport
    SetStatus(type, id, status, StatusOptions) StatusResult
    Init(InitOptions) InitReport
    Template(type) string
    ExportTemplate(type, dest, ExportOptions) ExportResult
  }
  class Entry {
    DocEntry
    Type string
    Path string
  }
  class CreateResult {
    Type string
    FilePath string
    Number string
    Filename string
    Update *TypeReport
  }
  class UpdateReport {
    Types []TypeReport
  }
  class TypeReport {
    Type string
    Dir string
    Docs int
    ToC *toc.UpdateReport
    Index index.UpdateOutcome
  }
  class StatusResult {
    ID string
    Type string
    Path string
    Old string
    New string
    Changed bool
  }
  class InitReport {
    Files []InitFile
  }
  Repo --> Entry
  Repo --> CreateResult
  Repo --> UpdateReport
  UpdateReport "1" --> "*" TypeReport
  CreateResult --> "0..1" TypeReport
  Repo --> StatusResult
  Repo --> InitReport
```

```mermaid
classDiagram
  class Doc {
    ID string
    Title string
    Status config.Status
    Phases []Phase
    Task(id) (Task, bool)
    Phase(token) (Phase, bool)
    Tasks() []Task
    Progress() (done, total)
  }
  class Phase {
    Index int
    Token string
    Title string
    Description string
    Tasks []Task
    Criteria []Criterion
    Line int
  }
  class Task {
    ID string
    Text string
    Checked bool
    Verify string
    Deferred *Marker
    Skipped *Marker
    Line int
    EndLine int
  }
  class Marker {
    Note string
    Line int
  }
  class Criterion {
    Text string
    Executable bool
    Command string
    Line int
  }
  Doc "1" --> "*" Phase
  Phase "1" --> "*" Task
  Phase "1" --> "*" Criterion
  Task --> "0..1" Marker : Deferred
  Task --> "0..1" Marker : Skipped
```

No storage anywhere: every value is computed from bytes or from a scan and
holds no handle to the filesystem. `Repo` is two fields and is safe to
construct per call.

## Testing Strategy

- **`pkg/impl`**: golden fixtures snapshotted under `pkg/impl/testdata/`
  (never read from `docs/`): docz IMPL-0009 and IMPL-0017 (wrapping),
  IMPL-0001 (non-phase `### Phase N` headings), IMPL-0007 (phases without
  criteria), docz-api IMPL-0004 (verify lines), docz-api IMPL-0006 (prefix
  deferred marker), sdk-booty-sh's clean and messy fixtures, a synthetic doc
  with a skipped task and one with a task inserted mid-run. Each with a
  `.golden.txt` fact file regenerated by `-update`. Invariants goldens
  cannot express: every `Task.Line` is a line `docparse.TaskItems` reports;
  IDs are unique; no `Text` contains a verify prefix or a marker;
  `EndLine >= Line`; a skipped task keeps its ID. `FuzzParse` pins
  never-panic.
- **`docwrite` byte cores**: the existing `status` and `checktask` goldens
  run through the path wrappers unchanged, plus a bytes-only table for the
  uncheck direction and for `Render` (output equals what `Create` writes).
- **`index` and `doctemplate`**: `git mv` carries the tests; `Splice` gets a
  table pinning that `UpdateReadme`'s outcomes are unchanged; `Scaffold`
  gets the #99 regression (exactly one marker pair for every type).
- **`repo`**: table tests over `t.TempDir()` repos for every method, each
  asserting the typed report and the on-disk result; error tests for every
  sentinel and `InvalidStatusError`.
- **`wiki`**: existing goldens carry over; `Init` and `UpdateNav` get
  temp-dir tests mirroring today's `cmd/wiki` tests.
- **The swap**: the `cmd/` test suite runs unchanged. Any test that has to
  change is a behaviour change and blocks the PR (ADR-0001 Decision 7).
- **Parity with v1.2.2**: `test/parity/` (§4) — goldens captured from the
  v1.2.2 binary, replayed over the five built-in types, one custom type,
  and a legacy `plan:` block; region-marker lines and the new commands are
  the only permitted deltas.
- **Consumer proof**: `test/consumer` imports every `pkg/` package —
  `repo`, `index`, `doctemplate`, `impl`, `wiki` join the existing five —
  and exercises one call each from outside the module.
- **Layer rule R2**: a test in `pkg/doczcore` walks `go list -deps` for
  each core package and fails if `pkg/impl` or `pkg/wiki` appears, and a
  sibling test fails if any `go.opentelemetry.io` or logging module appears
  anywhere under `pkg/`.
- **Context and hooks**: a cancelled context makes `repo.Update` return
  after the current type with the partial report intact and `ctx.Err()`;
  a hooks test asserts the exact event sequence for a two-type update,
  which the CLI's debug-log test pins from the other side.
- **Regions and validation**: DESIGN-0015's testing strategy. Its parity
  proof (the heuristic parse equals the region parse on every task ID and
  line over the migrated fixtures) is what lets the heuristics be deleted.

## Migration / Rollout Plan

```mermaid
timeline
  title Build the whole API, then swap — every landing under dont-release until the last
  section Module path
    go.mod becomes docz/v2 : parity goldens captured from v1.2.2
  section Type layer first
    docparse Markers and Regions : validate package : pkg/impl Parse and Doc over regions : docwrite byte cores and Render : templates gain markers : consumer proof
  section Promotions
    internal/template → doctemplate with DefaultConfigYAML : internal/index → index with Splice and Scaffold : internal/wiki → pkg/wiki with Init and UpdateNav
  section Repository core
    pkg/doczcore/repo : Scan List Find Create Update SetStatus Init Template ExportTemplate : Validate and InsertRegions : context and Hooks
  section Catalogue
    ADR-0003 plan removal : goldens and docs
  section Swap (v2.0.0-beta.1)
    cmd/ re-pointed, tests unchanged : parity suite green : docz validate and update --regions : docs/ migrated : ADR-0001 amended : CLAUDE.md README
```

| Step | Delivers | PR label |
| ---- | -------- | -------- |
| 0 | Module path → `github.com/donaldgifford/docz/v2` (`go.mod`, every import, `Makefile` and `.goreleaser.yml` ldflags, `test/consumer`); parity goldens captured from the v1.2.2 binary into `test/parity/`; `release.yml` gains a tag trigger for `v*-beta.*` pre-release tags and `pr-semver-bump`'s pre-release-base behaviour is checked (ADR-0002 Open Question 3). v1.x tags keep the old path; a `v1` branch is cut from v1.2.2 only on demand | `dont-release` |
| 1 | `docparse.Markers`/`Regions`; `validate`; `pkg/impl` over regions with `Validate`; `docwrite` byte cores, `SetTaskState`, `NextNumber`, `Render`; every embedded template gains markers and its schema skeleton; consumer proof | `dont-release` |
| 2 | `doctemplate`, `index`, `wiki` promotions (`git mv` + additions); schema resolution; `internal/` emptied | `dont-release` |
| 3 | `repo` with context, `Hooks`, `Validate`, `InsertRegions`; `ExportTemplate` scaffolds custom types | `dont-release` |
| 4 | ADR-0003: `plan` removed, goldens regenerated, docs | `dont-release` |
| 5 | `cmd/` swap; `docz validate`, `docz update --regions`; docz's own `docs/` migrated; optional `task list`; parity suite green and in `make ci`; ADR-0001 amendment; CLAUDE.md, README library section, release notes; claude-skills issue for the plugin's bundled templates | `dont-release`, then tag `v2.0.0-beta.1` by hand |
| — | IMPL-0017 (`updated:` field) retargets from v1.3.0 to the v2 line, after this unit | — |

The table has no consumer column on purpose. The API is step one; the
CLI's migration onto it is step two and the validation of step one, by
byte-for-byte comparison against v1.2.2 (§4). tempy, docz-api, and
sdk-booty-sh are not part of this work: what they rely on is recorded in
§5 so the API is shaped with them in mind, and their adoption happens in
their own repos once the API and the CLI are done.

```mermaid
gitGraph
  commit id: "v1.2.2" tag: "v1.2.2"
  commit id: "module path docz/v2, parity goldens"
  branch feat/impl
  commit id: "regions, validate, pkg/impl, byte cores"
  checkout main
  merge feat/impl id: "dont-release"
  branch feat/promote
  commit id: "doctemplate, index, wiki"
  checkout main
  merge feat/promote id: "dont-release "
  branch feat/repo
  commit id: "pkg/doczcore/repo with ctx and hooks"
  checkout main
  merge feat/repo id: "dont-release  "
  branch chore/plan
  commit id: "ADR-0003 removal"
  checkout main
  merge chore/plan id: "dont-release   "
  branch feat/cmd-swap
  commit id: "cmd/ on the API, validate, docs migrated"
  checkout main
  merge feat/cmd-swap id: "dont-release    " tag: "v2.0.0-beta.1"
```

One IMPL document covers this design and DESIGN-0015 together, one phase
per step above (Open Question 11). Steps 0–4 each leave the CLI on its
current code paths, so a `main` build at any point behaves exactly like
v1.2.2 for CLI users while carrying the new packages for library consumers.
Every PR in this unit is `dont-release`, the swap included: once the
module path is `/v2`, a `patch` or `minor` label would tag a v1 version
that `go get` rejects for a `/v2` module, and `major` is reserved for
v2.0.0 proper. Step 5 is tagged `v2.0.0-beta.1` by hand from its merge
commit (`make release TAG=v2.0.0-beta.1`); goreleaser marks the tag a
pre-release and the step-0 tag trigger builds its binaries. Its release
notes are the library changelog for everything above. Further betas follow
as the API settles; between tags a v2 pseudo-version exists without
cutting anything (ADR-0002 Open Question 3).

v2.0.0 proper is not this unit's to cut. It is the milestone at which
docz-api has moved into this repo and builds from `cmd/docz-api`, and the
UI has followed so one chart ships the API and the UI together — each its
own design, after the CLI. The EXPERIMENTAL markers come off then.

Docs touched by the work: ADR-0001 (dated amendment, Open Question 2 of
ADR-0002), IMPL-0014 (Decision 3 note), DESIGN-0013 (Abandoned — done with
this design), CLAUDE.md (architecture bullets for the six new packages and
the emptied `internal/`), README (library section; six types become five;
`docz validate`), DEVELOPMENT.md (the "add a type" walkthrough moves its
template paths and gains the region markers), every embedded template
(markers), docz's own `docs/` (migrated with `update --regions` in the swap
PR), the docz skills plugin (claude-skills issue for the bundled templates
and the validate step), `mkdocs.yml` (`pymdownx.superfences` so these
diagrams render in the wiki).

## Open Questions

> Each question is numbered; option `a` is my recommendation, later letters
> are alternatives, and the last is a free-form "other". Questions 2–4 are
> DESIGN-0013's 4–6, carried over unresolved; question 1 was DESIGN-0013's 3
> and is restated for regions.
>
> **Update 2026-09-19:** all twelve questions are resolved — see the
> Decisions table. DESIGN-0015's own questions remain open.

| # | Question | Decision |
| - | -------- | -------- |
| 1 | Phase and task grammar | (a), corrected against `impl.md`: HTML comments are stripped from the phase heading before the token regex and an empty title is a warning; a region wraps its own heading; the `---` separators sit outside every region |
| 2 | Continuation folding and verify lines | (a); worked example added to §3 |
| 3 | Marker parsing on the read side | (a) lenient |
| 4 | Criteria classification | (a) as issue #100 states |
| 5 | Creation without I/O | (a) `NextNumber` plus `Render` |
| 6 | Resolving a document by ID | (a) both `Find` and `FindIn` |
| 7 | Status no-op short-circuit | (a) in `repo.SetStatus`, reported as `Changed: false` |
| 8 | How much of wiki is orchestration | (a) `Init` and `UpdateNav` in `pkg/wiki` |
| 9 | Error shapes in repo | **(b) typed errors everywhere** — the typed API should give all of its benefits; §2.8 updated |
| 10 | Generic facts for the next type package | **superseded by DESIGN-0015**: regions are the typed spans a future package would want, and `Repo.Find` already maps a prefix to a type through config; both helpers dropped |
| 11 | Delivery granularity | **one IMPL covering this design and DESIGN-0015 together**, a phase per rollout step; designs map to IMPLs many-to-one when they ship as a unit |
| 12 | Hooks for wiki | (a) none; the nav report is enough |

### 1. Phase and task grammar

> **Resolved 2026-09-19: (a)**, after checking the assumptions against
> `internal/template/templates/impl.md`: the template's phase title is an
> HTML comment placeholder, so comments are stripped before the token
> regex and an empty title is the `impl.phase.no-title` warning; the
> `#### Tasks` and `#### Success Criteria` headings sit inside their
> regions; the `---` separators between phases sit outside every region.
> DESIGN-0015 §1, §5, and §6 carry the same corrections.

- a. **Phases are `phase` regions; the token comes from the first level-3
  heading inside; `tasks` and `criteria` are nested regions; top-level
  checkbox items only; a duplicate token is an error** — §3 as amended by
  DESIGN-0015 §5. *(recommendation)*
- b. The heading heuristics as DESIGN-0013 had them (`^Phase\s+…:` finds
  the span, `#### Tasks` scopes the list), kept as a permanent fallback
  beside the regions — two grammars to maintain.
- c. Every level-3 heading is a phase with ordinal fallback (sdk-booty-sh
  today).
- d. Other.

### 2. Continuation folding and verify lines

> **Resolved 2026-09-19: (a).** The worked example in §3 shows the fold,
> the verify extraction, and the marker removal on a real-shaped task.

- a. **Fold continuations into `Text`; verify is case-insensitive, first
  backtick span, trailing prose ignored** — the four hand-written docz-api
  lines parse. *(recommendation)*
- b. Strict lowercase `verify:` whose remainder is exactly one backtick
  span.
- c. First-line-only `Text` (the facts literal) — truncates most recent
  tasks.
- d. Other.

### 3. Marker parsing on the read side

> **Resolved 2026-09-19: (a).**

- a. **Lenient**: `deferred` after any dash, prefix or suffix, emphasis
  tolerated; strikethrough plus `skipped:` after any dash. Canonical
  spellings are documented for writers (consumers), not enforced by the
  parser. *(recommendation)*
- b. Canonical suffix forms only; docz-api IMPL-0006 is hand-fixed.
- c. Other.

### 4. Criteria classification

> **Resolved 2026-09-19: (a).**

- a. **As issue #100 states**: executable iff the bullet starts with a
  backtick span; the caveat that symbol-subject criteria (about a tenth in
  this repo) classify as executable is documented, and docz never runs
  anything. *(recommendation)*
- b. Executable only when the whole bullet is a backtick span.
- c. Token heuristics on the span.
- d. Other.

### 5. Creation without I/O

> **Resolved 2026-09-19: (a).**

- a. **`NextNumber(dir, width)` plus `Render(opts, number)`**, with `Create`
  composed from them. Two small functions; a no-checkout consumer supplies
  its own number (it knows its tree) and gets `Rendered{Filename, Content}`
  to commit through its own API. *(recommendation)*
- b. One `Prepare(opts) (Rendered, error)` that scans for the next number
  itself — fewer names, but it needs the filesystem, which defeats the
  purpose.
- c. Neither now; `Create` stays the only creation entry point until a
  consumer asks.
- d. Other.

### 6. Resolving a document by ID

> **Resolved 2026-09-19: (a).**

- a. **Both `Find(id)` and `FindIn(type, id)`.** `Find` derives the type
  from the prefix through `ValidateType` (which already resolves
  `id_prefix` tokens, and `validateResolution` guarantees uniqueness) and
  serves consumers that only hold an ID, such as `task list`; `FindIn`
  mirrors `status set <type> <id>` exactly. *(recommendation)*
- b. `FindIn` only; callers split the prefix themselves.
- c. `Find` only; `status set` ignores its `<type>` argument beyond
  validation.
- d. Other.

### 7. Where the status no-op short-circuit lives

> **Resolved 2026-09-19: (a).** DESIGN-0005 Decision 8 is superseded for
> the library path; the CLI's output is unchanged because it prints from
> the result.

DESIGN-0005 Decision 8 put "current equals new → no write" in `cmd/`.

- a. **In `repo.SetStatus`, reported as `Changed: false`.** It is not
  wording; it is the operation's semantics, and every consumer wants the
  same rule. `cmd/` output is unchanged because it prints from the result.
  *(recommendation)*
- b. Keep it in `cmd/`; `repo.SetStatus` always writes when called, like
  the `docwrite` helper.
- c. Other.

### 8. How much of wiki is orchestration

> **Resolved 2026-09-19: (a).**

- a. **`Init` and `UpdateNav` live in `pkg/wiki`.** They are the two things
  a consumer would otherwise copy from `cmd/wiki.go`, and the primitives
  stay exported for anyone who wants a different composition.
  *(recommendation)*
- b. Primitives only; the orchestration stays in `cmd/wiki.go` and `wiki`
  is the one command with no single-call equivalent.
- c. Put `Init` and `UpdateNav` on `repo` — but then `doczcore` imports
  `pkg/wiki`, breaking R1's sibling rule.
- d. Other.

### 9. Error shapes in repo

> **Resolved 2026-09-19: (b)** — typed errors everywhere. Review rationale:
> a typed API should deliver all of its benefits, and a caller that can
> `errors.As` into `NotFoundError{Type, ID}` should not have to re-derive
> those facts from a message. `UnknownTypeError` unwraps to the frozen
> `config.ErrUnknownType` so `errors.Is` keeps working. §2.8 and the
> `status set` row in §4 are updated.

- a. **Sentinels for the yes/no cases (`ErrNotFound`, `ErrTypeDisabled`,
  `ErrExists`) and one typed `InvalidStatusError` carrying the allowed
  list**, with `config.ErrUnknownType` passing through. `cmd/` maps them to
  exit codes with `errors.Is`/`errors.As` and keeps its current messages.
  *(recommendation)*
- b. Typed errors everywhere (`NotFoundError{Type, ID}` …) — richer, more
  surface to freeze.
- c. Sentinels everywhere; the allowed-status list is re-derived by the
  caller from `Cfg`.
- d. Other.

### 10. Generic facts for the next type package

> **Resolved 2026-09-19: superseded by DESIGN-0015.** The question asked
> whether to add two helpers ahead of a second type package: a
> level-2-heading span reader (`docparse.Sections`) and an ID-prefix-to-type
> mapper (`document.Kind`). Regions make the first redundant — typed spans
> are exactly what the next package wants, and heading spans are the
> heuristic being retired — and `Repo.Find` already does the second
> through `config.ValidateType`. Both are dropped.

- a. **Defer** `docparse.Sections` and `document.Kind` until a second type
  package exists; the shapes are noted so they are not redesigned.
  *(recommendation)*
- b. Add `docparse.Sections` now — small, but a frozen API with no caller.
- c. Other.

### 11. Delivery granularity

> **Resolved 2026-09-19: one IMPL covering this design and DESIGN-0015
> together**, a phase per rollout step with the swap last. Designs and
> IMPLs are not one-to-one: designs that ship as a unit are consumed by a
> single IMPL.

- a. **One IMPL document with a phase per rollout step** (five phases, the
  swap last), so the unit of delivery matches the unit of design and the
  acceptance criteria of the last phase are the swap's. *(recommendation)*
- b. One IMPL per package, tracked in a parent issue.
- c. Other.

### 12. Hooks for wiki

> **Resolved 2026-09-19: (a).** Hooks would give the wiki almost nothing:
> `UpdateNav` is one YAML read, a title walk over the docs tree, and one
> YAML write, and the two debug lines `cmd/wiki.go` logs today are
> derivable from `NavReport` after the call. Cancelling the title walk on
> a large tree is what matters, and the context covers that.

- a. **None; the nav report is enough.** `wiki.Init` and `UpdateNav` take a
  context for cancellation and return reports; the two debug lines
  `cmd/wiki.go` logs today are derivable from `NavReport` after the call.
  A `wiki.Hooks` can be added later without breaking anything.
  *(recommendation)*
- b. A `wiki.Hooks` mirroring `repo.Hooks` from day one, so both L3
  packages instrument the same way.
- c. Other.

## References

- [ADR-0002](../adr/0002-docz-is-an-api-package-whose-first-consumer-is-the-cli.md)
  — the decisions this design implements
- [ADR-0003](../adr/0003-remove-plan-from-the-built-in-document-types.md)
  — the catalogue change that ships in the same release
- [ADR-0001](../adr/0001-pkgdoczcore-as-the-single-public-core-cmd-as-a-thin-cli-shell.md)
  — the frozen base and the API principles kept
- [DESIGN-0015](0015-structured-regions-and-docz-validate.md) — structured
  regions, the validator, `docz validate`, the corpus migration; a
  requirement of this unit
- [DESIGN-0013](0013-library-first-docz-per-type-document-packages-and-a-core-api.md)
  — Abandoned; inventory and `pkg/impl` specification carried over
- `net/http/httptrace` — the hooks-in-context pattern §7 follows
- [INV-0010](../investigation/0010-impl-plan-parse-and-write-back-api-for-doczcore-issue-100.md)
  — corpus grammar facts, existing primitives, consumer model
- [DESIGN-0005](0005-status-set-cli-primitive.md) — byte-preservation
  contract; [IMPL-0011](../impl/0011-status-set-cli-primitive.md)
- [DESIGN-0006](0006-custom-document-type-support.md) — type resolution
  and custom types; [DESIGN-0007](0007-docz-changes-to-support-docz-api-and-docz-site.md)
  — the promotion pattern
- [IMPL-0014](../impl/0014-v100-the-five-package-pkgdoczcore-public-core.md)
  — the v1.0.0 core; [IMPL-0017](../impl/0017-v130-updated-frontmatter-field-and-the-docz-update-stamp-pass.md)
  — retargets to v2.1.0
- Issues [#100](https://github.com/donaldgifford/docz/issues/100),
  [#103](https://github.com/donaldgifford/docz/issues/103),
  [#92](https://github.com/donaldgifford/docz/issues/92),
  [#95](https://github.com/donaldgifford/docz/issues/95),
  [#96](https://github.com/donaldgifford/docz/issues/96),
  [#99](https://github.com/donaldgifford/docz/issues/99)
- tempy `docs/design/0001-temporal-orchestrated-impl-loop-execution.md`,
  `docs/impl/0001-temporal-orchestrated-impl-loop-worker-mvp.md`;
  sdk-booty-sh `pkg/loop/doczwork`
- `cmd/{update,create,init,status,wiki,template,list}.go`,
  `internal/{index,template,wiki}`, `pkg/doczcore/*`
