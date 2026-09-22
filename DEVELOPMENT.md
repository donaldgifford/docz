# Development Guide

This document covers the internals of `docz` — architecture, package
responsibilities, and how to extend the tool with new document types.

## Project Layout

```
docz/
├── cmd/
│   ├── docz/
│   │   └── main.go          # entry point (imports cmd package)
│   ├── root.go              # root command, config init, global flags
│   ├── init.go              # docz init
│   ├── create.go            # docz create
│   ├── update.go            # docz update
│   ├── list.go              # docz list
│   ├── template.go          # docz template show/export/override
│   ├── config.go            # docz config
│   ├── wiki.go              # docz wiki init/update
│   └── version.go           # docz version (Version/Commit vars for ldflags)
├── pkg/doczcore/            # PUBLIC, semver-governed core (ADR-0001; frozen at v1.0.0)
│   ├── config/
│   │   ├── config.go        # Config structs, Load(), Validate(), DefaultConfig()
│   │   ├── constants.go     # FileMode/DirMode + filename constants
│   │   └── doctype.go       # allDocTypes registry, DocTypeNames(), TypesHelp()
│   ├── document/            # read side: frontmatter, scanning, changelog
│   │   ├── document.go      # Frontmatter, ParseFrontmatter(), LoadFrontmatter()
│   │   ├── scan.go          # ScanDocuments(), DoczFilePattern, IsDoczFile()
│   │   └── changelog.go     # Changelog, ParseChangelog(), ErrNoVersions
│   ├── docparse/            # markdown fact extractor (stdlib-only)
│   │   ├── headings.go      # Heading, Headings(), AnchorSlug()
│   │   ├── taskitems.go     # TaskItem, TaskItems()
│   │   ├── listitems.go     # ListItem, ListItems()
│   │   ├── tables.go        # Table, Tables()
│   │   ├── title.go         # Title() (ATX + setext H1)
│   │   └── regions.go       # Marker, Region, Markers(), Regions()
│   ├── kinds/               # region readers + heading inference
│   │   ├── kinds.go         # RegionBytes(), Body(), bodyOffset
│   │   ├── infer.go         # HeadingSpec, InferRegions(), ResolveRegions()
│   │   ├── shift.go         # Shift* region-line → document-line helpers
│   │   └── item.go …        # Items/Sections/References/Criteria/… readers
│   ├── validate/            # generic validator (never fails, returns findings)
│   │   ├── document.go      # Document(), Options
│   │   ├── schema.go        # Schema, SchemaFromMarkers()
│   │   └── kindrule.go      # the 41-kind catalogue
│   ├── docwrite/            # write side: create + status + checkbox
│   │   ├── create.go        # Create(), NextNumber(), Render()
│   │   ├── status.go        # SetStatus()/SetStatusBytes() frontmatter mutator
│   │   └── checktask.go     # CheckTask()/SetTaskState[Bytes]() checkbox splice
│   ├── toc/
│   │   ├── toc.go           # GenerateToC(), UpdateToC() (walks via docparse)
│   │   └── update.go        # UpdateFiles() batch splice + UpdateReport
│   ├── index/               # README index table + marker splice
│   │   └── index.go         # GenerateTable(), Scaffold(), Splice(), UpdateReadme()
│   ├── doctemplate/         # embedded templates, skeletons, resolution
│   │   ├── embed.go         # //go:embed, EmbeddedDocumentTemplate(), EmbeddedSchema()
│   │   ├── template.go      # Resolve(), ResolveSchema(), Render(), Data
│   │   ├── errors.go        # ErrNoTemplate, ErrNoSchema, ErrBadSchemaName
│   │   └── templates/       # embedded files (doc + schema/ + index_* + wiki_index)
│   └── repo/                # the repository tier: operations, not primitives
│       ├── repo.go          # Repo{Root, Cfg}, Open(), Path/TypeDir/ReadmePath
│       ├── errors.go        # the six typed errors
│       ├── hooks.go         # Hooks, WithHooks(), FileKind, SkipReason
│       ├── scan.go          # Entry, Scan(), List(), Find(), FindIn()
│       ├── create.go …      # Create/Update/SetStatus/Init/Template/Export
│       ├── validate.go      # Validate(), ValidateReport, DocFindings
│       └── regions.go       # InsertRegions() — the migration write
├── pkg/{rfc,adr,design,impl,investigation}/   # one typed reader per built-in
│   ├── doc.go               # the Doc struct
│   ├── headings.go          # kind constants + the HeadingSpec table
│   ├── parse.go             # Parse(doc []byte) (Doc, error)
│   └── validate.go          # Validate() + the type's Code* constants
├── pkg/wiki/                # MkDocs / TechDocs integration (not core, not a type)
│   ├── orchestrate.go       # Action, Init(), UpdateNav(), InitReport, NavReport
│   ├── titles.go            # DirTitle(), DocTitle(), FilenameTitle()
│   ├── wiki.go              # NavEntry, ScanDocs(), BuildNav(), CountPages()
│   └── mkdocs.go            # ReadMkDocs(), WriteMkDocs(), NavToYAML(), MergeNavOrder()
├── test/consumer/           # separate module proving the public surface externally
└── testdata/
    └── golden/              # golden file fixtures for template, toc, and wiki tests
```

## Package Responsibilities

### `pkg/doczcore/config`

Loads and validates the `docz` configuration. The entry point is `Load()`.

**Config precedence (lowest to highest):**
1. Built-in defaults (`DefaultConfig()`)
2. Global config (`~/.docz.yaml`)
3. Repo config (`.docz.yaml`)
4. Flags (`--docs-dir`, etc.)

Deep merge is dependency-free (viper was removed in IMPL-0014 Phase 1):
each file is read into a raw `map[string]any` with `yaml.v3`
(`readConfigMap`), the maps are merged recursively with repo keys winning
(`mergeMaps`), and the result is decoded onto a pre-populated
`DefaultConfig()` (`decodeSettings`). The repo config overrides only the
keys it explicitly sets; unset keys inherit from global. Slices (e.g.
`statuses`) are replaced entirely, not appended. Unknown keys are ignored
(lenient decode) and config keys are case-sensitive — both pinned by the
parity tests in `parity_baseline_test.go`.

```go
cfg, err := config.Load(configFile, repoRoot)
cfg.Validate()  // returns (warnings []string, err error)
```

`Load` normalizes, `Validate` judges. `normalizeChangelog` is the one
place that backfills an explicitly empty value: `changelog.file: ""`
resolves to `DefaultChangelogFile` and a leading `./` is stripped, so
every consumer sees one canonical path. (An *omitted* key needs no help —
decoding happens onto a pre-populated `DefaultConfig()`.) `Validate` then
rejects an absolute, traversing, or directory-shaped `changelog.file`,
**but only when the block is enabled**: a dormant block must never fail
config load, so a repo can add it before the feature is turned on
(DESIGN-0010).

### `pkg/doczcore/doctemplate`

Handles template resolution and rendering.

**Resolution order (first match wins):**
1. `types.<type>.template` path in config (absolute or relative to repo root)
2. `<docs_dir>/templates/<type>.md` (local override file)
3. Embedded default (`pkg/doczcore/doctemplate/templates/<type>.md`)

```go
content, err := doctemplate.Resolve(docType, tc.Template, cfg.DocsDir)
rendered, err := doctemplate.Render(content, &doctemplate.Data{...})
```

A type with none of the three is `ErrNoTemplate`, wrapped with the type name
and the on-disk path a person would create (issue #92).

`FilenameSlug(title)` converts a title to kebab-case, strips non-alphanumeric
characters, and truncates to 64 characters on a word boundary.

**Marker skeletons** resolve the same way, one tier shorter:
`ResolveSchema(name, docsDir)` checks `<docs_dir>/templates/schema/<name>.md`
then the embedded `schema/<name>.md`, and `EmbeddedSchema(name)` is the
baked-in tier alone for a consumer with no checkout. Neither is rendered — a
skeleton is markers, not a template. The name grammar `[a-z0-9][a-z0-9_-]*` is
enforced before either lookup, because the name arrives from a document's own
frontmatter and may say anything at all; an illegal one returns both
`ErrNoSchema` and `ErrBadSchemaName`.

`DefaultConfigYAML()` renders the embedded `docz_yaml.tmpl` over
`config.DefaultConfig()`, so `docz init` and any consumer that scaffolds a repo
produce the same `.docz.yaml` from the same source of defaults.

`ResolveWikiIndex(docsDir)` resolves the wiki homepage template:
1. Local override at `<docs_dir>/templates/wiki_index.md`
2. Embedded default (`pkg/doczcore/doctemplate/templates/wiki_index.md`)

`RenderWikiIndex(tmpl, data)` renders the template with `WikiIndexData`
(site name and enabled types).

### `pkg/doczcore/docwrite`

The public **write side** of the docz core (promoted whole in IMPL-0014;
the read side is `pkg/doczcore/document`). Three operations, deliberately
not a general editor: `Create` renders a template into a new document,
`SetStatus` rewrites only the frontmatter status value bytes, and
`CheckTask(path, line)` flips the `[ ]` on a 1-based line (typically a
`docparse.TaskItem.Line`) to `[x]` with a single-byte splice. Both
mutators are LF-only (`ErrUnsupportedLineEndings`).

Each has a **byte core** underneath it — `SetStatusBytes`,
`SetTaskStateBytes`, and `NextNumber` + `Render` — for a consumer holding
bytes it fetched rather than a path it can write. A core never modifies its
input and returns bare sentinels; the path wrappers add the path and the line.

```go
result, err := docwrite.Create(&docwrite.CreateOptions{
    Type:    "rfc",
    Title:   "My Proposal",
    Author:  "Alice",
    Status:  "Draft",
    Prefix:  "RFC",
    IDWidth: 4,
    DocsDir: "docs",
    TypeDir: "rfc",
})
// result.Filename  → "0001-my-proposal.md"
// result.FilePath  → "docs/rfc/0001-my-proposal.md"
// result.Number    → "0001"
```

`nextID()` scans the target directory for `document.DoczFilePattern`
(`NNNN-*.md`) files and returns `max(existing IDs) + 1`. It starts at 1 if the
directory is empty or missing.

### `pkg/doczcore/index`

Scans document directories and generates README tables.

```go
docs, err := index.ScanDocuments(dir)   // returns []DocEntry sorted by ID
table := index.GenerateTable(docs)       // markdown table string
msg, err := index.UpdateReadme(path, docType, table)   // splices between markers
```

`UpdateReadme` uses HTML comment markers to splice the table into the README:
```
<!-- BEGIN DOCZ AUTO-GENERATED -->
...generated table...
<!-- END DOCZ AUTO-GENERATED -->
```

If a README exists but has no markers, it is left untouched and a warning is
printed. If the README does not exist, it is created using the embedded index
header template.

### `pkg/doczcore/document`

The read side: frontmatter parsing (`ParseFrontmatter`,
`LoadFrontmatter`), directory scanning (`ScanDocuments`), the docz
filename convention (`IsDoczFile`), and changelog parsing.

`ParseChangelog(content []byte) (*Changelog, error)` turns git-cliff /
Keep-a-Changelog markdown into `Changelog{Preamble, Versions}` →
`ChangelogVersion{Version, Unreleased, Date, Groups}` →
`ChangelogGroup{Title, Items}`. The contract is total: either a non-nil
result with a nil error, or `(nil, ErrNoVersions)` — no partial results,
no other error kind, and no panics (pinned by `FuzzParseChangelog`).

Grammar rules worth knowing before you touch it:

- Headings match **only at column 0**, unlike `docparse.Headings` which
  trims each line first. Here an indented `### x` is content nested under
  a bullet, not a heading.
- **Only indented lines continue an item.** Column-0 prose closes the
  open item and is discarded, which is what keeps one `Items` entry per
  commit bullet.
- Fence-aware using a locally duplicated copy of the module's fence rule
  (no `docparse` import), so neither package's frozen behavior is coupled
  to the other's. A fence line inside an item stays in the item.
- Duplicate versions and duplicate group titles produce separate entries;
  the parser reports what the file says.

Fixtures live in `testdata/changelog/` — a verbatim snapshot of a real
git-cliff changelog plus chart and edge cases — with `.golden.txt` fact
files regenerated by `go test ./pkg/doczcore/document/ -update`.

### `pkg/doczcore/docparse`

The module's one markdown fact extractor (stdlib-only, no errors):
`Headings()` returns every H2–H6 heading with inline markdown stripped,
GitHub-compatible anchor slugs (`AnchorSlug()`, duplicate `-1`/`-2`
suffixes), and 1-based line numbers; `TaskItems()` returns every `- [ ]` /
`- [x]` checkbox item with text, checked state, raw indent width, and line.
Every walker skips fenced code blocks. Facts only — plan/phase
interpretation is left to consumers (ADR-0001).

`ListItems()` and `Tables()` are the other two walkers: bullet and numbered
list items (`{Text, Ordered, Indent, Line}` — the ordered item's *number* is
not reported, because markdown renumbers a list from its first value), and GFM
pipe tables (`{Header, Rows, Line}` — a row may be shorter or longer than the
header, since padding it would invent content).

`Markers()` and `Regions()` are the region walker (DESIGN-0015 §1).
`Markers()` reports every `<!--docz:<kind>:start-->` / `:end-->` line in
document order, *including* stray and non-canonical ones, so validation can
explain why a region is missing rather than silently skipping the file — which
is what used to happen to a marker with a stray space in it (INV-0009 F4).
`Regions()` pairs them with a stack into `{Kind, Start, End, Depth, Closed}`.
`Start` and `End` are the marker lines themselves, so a region's *content* is
the lines strictly between them, and an unclosed region still comes back usable
with `Closed: false`. The legacy ToC and README-index pairs report as the `toc`
and `index` kinds, so one vocabulary covers every span docz owns.

### `pkg/doczcore/kinds`

The layer between a region *span* and a typed *field*. `RegionBytes(doc, r)`
cuts a region's bytes; `Body`, `Field`, `Items`, `Sections`, `References`,
`Criteria`, `Alternatives`, `Decisions`, and `OpenQuestions` each read one
shape out of them. Same contract as `docparse`: bytes in, values out, no
errors.

**Every reader's `Line` is 1-based within the region**, counting the region's
own first line as 1. A caller converts to a document line exactly one way:

```go
documentLine := region.Start + reader.Line
```

The seven `Shift*` helpers do that for the type packages, so five packages
don't carry five chances to be off by one.

`HeadingSpec` is the inference grammar: `InferRegions(doc, spec)` synthesizes
regions from a document's headings, and `ResolveRegions(doc, spec)` prefers
real markers and returns whether it had to infer. **Inference is permanent,
not a migration aid** (DESIGN-0015 §6) — markers, once present, are
authoritative, and a repo that never runs the fixer keeps working forever.
`SpecFromTemplate(tmpl)` derives a spec from a marked template, which is what
binds each type package's heading table to its own template in a test.

### `pkg/doczcore/validate`

`Document(content, opts) []Finding` **never fails**. A document with no
frontmatter is a document with a *finding*, because a validator that refused
to look at a broken document would be useless on exactly the documents that
need it most. Every `Options` field is optional and the zero value still
yields a useful run: marker well-formedness, frontmatter shape, and the content
rules of whatever kinds the document happens to carry.

`Schema` is the set of regions a document must carry, read from a marker
skeleton by `SchemaFromMarkers`. **There is no schema language**, so nothing a
schema can require is something a document cannot show (DESIGN-0015 §3), and a
schema only tightens by growing. The `catalogue` in `kindrule.go` holds what is
known about each of the 41 region kinds — `Singleton` (scoped by *parent*) and
an optional content `Check`. It is data, not an interface, which is why it grew
from nine kinds to forty-one without a redesign.

Findings carry a `Code` from a small set of families (`file.*`,
`frontmatter.*`, `marker.*`, `region.*`, `content.*`, `open-questions.*`,
`references.*`, `tasks.*`, `toc.*`, `schema.name`). Consumers filter on the
code, never on the wording.

### `pkg/{rfc,adr,design,impl,investigation}`

One typed reader per built-in type: `Parse` for the model, `Validate` for the
findings only the model can see. See *Adding a Built-In Document Type* below
for the four files and the contract each package keeps.

The generic tier and the type tier run side by side and never repeat each
other. An *absent* alternatives section is `region.missing` from
`validate.Document`; a section that is present and says nothing is
`rfc.alternatives.empty` from `rfc.Validate`.

### `pkg/doczcore/toc`

Generates table of contents for markdown documents. Uses `<!--toc:start-->` /
`<!--toc:end-->` markers (compatible with markdown-toc.nvim).

- **`toc.go`** — heading walks delegate to `docparse.Headings` (only the
  slice-past-end-marker policy lives here). `GenerateToC()` builds an
  indented markdown list from `[]docparse.Heading` with relative
  indentation. `UpdateToC()` splices the generated ToC between markers.
- **`update.go`** — `UpdateFiles()` batch splice over in-memory docs,
  returning a categorized `UpdateReport`.

### `pkg/doczcore/repo`

The repository tier: the package that makes "the CLI is one call plus
printing" true. A `Repo` is a root directory and a loaded config, and every
method is the orchestration one `cmd/` handler performs today with the
printing removed.

**Context enters here, and only here** (rule R8). Nothing in L0-L2 takes a
`context.Context`: a bytes-in function neither blocks nor opens anything, so
there is nothing to cancel. `repo` checks the context between per-type
iterations, never mid-file, and a cancelled run returns the report completed
so far together with `ctx.Err()`. What was already written stays written.

**Hooks are how a package that prints nothing still narrates.** `Hooks` is a
struct of five optional callbacks carried in the context, the
`net/http/httptrace.ClientTrace` pattern — a struct of funcs rather than an
interface, so adding an event breaks nobody. `HooksFrom(ctx)` never returns
nil, so a call site checks the field and not the struct.

```go
ctx = repo.WithHooks(ctx, &repo.Hooks{
    ScanStart: func(_, dir string) { logger.Debug("scanning type", "dir", dir) },
})
rep, err := r.Update(ctx, types, repo.UpdateOptions{DryRun: dryRun})
```

**Every failure a caller may branch on is a typed error**, so `cmd/` maps
them to exit codes with `errors.As` and docz-api maps them to status codes:
`NotFoundError`, `TypeDisabledError`, `ExistsError`, `InvalidStatusError`,
`UnknownTypeError`, `WriteError`. Two of them wrap:
`UnknownTypeError.Unwrap` returns `config.ErrUnknownType` so the frozen v1
sentinel still answers `errors.Is`, and `WriteError.Unwrap` returns the cause
so `docwrite.ErrUnsupportedLineEndings` stays reachable while the path comes
off the struct. `TypeDisabledError` is deliberately distinct from
`UnknownTypeError`: "you typed something wrong" and "you turned this off"
have different fixes, which is also why `Scan` on a disabled type is an error
rather than an empty slice.

Three rules the package keeps that are easy to break by accident:

- **`repo` never imports a type package** (R2). It is in `layer_test.go`'s
  `corePackages`, so the rule is enforced rather than asserted. The per-type
  tier — that an IMPL's phases are numbered, that an investigation must
  answer its question — is the caller's to compose, and `cmd/validate.go`
  does exactly that.
- **`Create` and `Update` share one unexported `updateType`.** Two copies of
  the index-and-ToC pass would drift.
- **Nothing consults the process working directory.** Every config-relative
  path resolves under `Repo.Root`.

`Validate` is the repository tier of DESIGN-0015 §4, and `InsertRegions` is
the migration pass of §6. Neither holds a heuristic: the heading-to-kind map
comes from `kinds.SpecFromTemplate` over the type's resolved template, and
the spans come from `kinds.InferRegions`. `InsertRegions` runs `Regions` over
its own output and refuses to write a malformed result.

### `pkg/wiki`

Generates and maintains MkDocs nav from the docs directory tree. Promoted whole
from `internal/wiki` in IMPL-0018 Phase 2, and deliberately a sibling of the
type packages rather than a member of `pkg/doczcore`: this is an integration
with MkDocs and Backstage TechDocs, not core and not a document type.

**The two operations** live in `orchestrate.go` and are what `cmd/wiki.go` used
to compose by hand, so a consumer gets the operation and not just the parts:

- **`Init(ctx, root, cfg, InitOptions) (InitReport, error)`** — writes
  `mkdocs.yml` and the docs landing page. Each file is created when absent,
  left alone when present, and rewritten when `Force` is set; `InitReport`
  carries both paths and an `Action` (`Created` / `Skipped` / `Overwritten`)
  for each. A file already there is not an error at this layer — refusing over
  a skipped `mkdocs.yml` is `docz wiki init`'s policy, not the package's.
- **`UpdateNav(ctx, root, cfg, NavOptions) (NavReport, error)`** — rebuilds the
  `nav:` key from the documents under `cfg.DocsDir`, preserving every other key
  in the file and the existing top-level section order. `NavReport` returns the
  `[]NavEntry` tree rather than rendered YAML, so the caller decides how to
  print it; `DryRun` fills the same report with `Written` false.

Both resolve every config-relative path under `root`, so neither consults the
process working directory, and both check `ctx` between steps — a cancelled run
returns the report so far and leaves what it already wrote in place.

Three things stay in `cmd/` on purpose: deriving the site name from the git
remote (an L4 dependency, the same as the author name), requiring `.docz.yaml`
to exist first, and all printing. `InitOptions.SiteName` arrives already
resolved; the package's own fallback reaches no further than `cfg` and `root`.

**The primitives** the two are built from stay exported for anyone who wants a
different composition:

- **`titles.go`** — Title extraction: `DirTitle()` maps directory names to
  nav titles using configurable overrides. `DocTitle()` extracts titles from
  frontmatter, then `docparse.Title`, then a filename fallback.
- **`wiki.go`** — Nav tree building: `ScanDocs()` recursively walks the docs
  directory and builds a `[]NavEntry` tree. `SortEntries()` sorts top-level
  entries (Home first, rest alphabetical). `CountPages()` counts leaf entries.
- **`mkdocs.go`** — MkDocs YAML I/O: `ReadMkDocs()`/`WriteMkDocs()` preserve
  non-nav fields. `NavToYAML()` converts `[]NavEntry` to MkDocs nav format.
  `MergeNavOrder()` preserves existing section order when updating.

## Adding a Built-In Document Type

Since IMPL-0009 (DocType registry, DESIGN-0004 §E) the config side of a
built-in type is a single Go edit plus two embedded templates. Since IMPL-0018
(DESIGN-0014, ADR-0002) a built-in is also a **structured type**, so it needs a
marker skeleton and a `pkg/<type>` package as well. The example below walks
through adding a `postmortem` type.

> A type that only needs a template and an index — no typed reader, no
> validation rules — is a **custom type**, not a built-in. See *Custom Types via
> Configuration* below; it is one `.docz.yaml` block and no Go at all.

### Step 1: Add the document template

Create `pkg/doczcore/doctemplate/templates/postmortem.md`. The file is a Go `text/template`
with access to all `template.Data` fields:

| Variable | Type | Notes |
|----------|------|-------|
| `{{ .Number }}` | string | Zero-padded ID, e.g. `0001` |
| `{{ .Title }}` | string | Document title as provided on the CLI |
| `{{ .Slug }}` | string | Kebab-case title for the filename |
| `{{ .Filename }}` | string | e.g. `0001-my-postmortem.md` |
| `{{ .Date }}` | string | ISO date (`YYYY-MM-DD`) |
| `{{ .Author }}` | string | Resolved from config/flag/git |
| `{{ .Status }}` | `config.Status` (typed string) | Initial status |
| `{{ .Type }}` | `config.DocType` (typed string) | Canonical type name |
| `{{ .Prefix }}` | string | ID prefix from the registry entry |

The typed-string types render via their underlying value; no template-syntax
changes are needed when working with `Status` / `Type`.

Every section the type's reader will read must be wrapped in a **canonical
region marker pair** (DESIGN-0015 §1), so a document created by `docz create` is
marked from birth and never needs migrating:

```markdown
<!--docz:timeline:start-->
## Timeline

<!-- What happened, in order. -->
<!--docz:timeline:end-->
```

Reuse an existing kind's name wherever the section means the same thing
(`summary`, `context`, `criteria`, `references`, `open-questions`, `decisions`).
The grammar is over region kinds and never over type names (ADR-0002 R7), so a
section that reuses a kind gets that kind's reader and validation rule for
free. A kind nobody else uses is fine too — an unknown kind is allowed, and
`validate` then checks only that its markers pair.

### Step 2: Add the marker skeleton

Create `pkg/doczcore/doctemplate/templates/schema/postmortem.md`: the *schema* for the type,
which is a markdown body of nothing but the template's marker pairs, in the same
order and nesting, with no headings and no prose.

```markdown
<!--toc:start-->
<!--toc:end-->
<!--docz:timeline:start-->
<!--docz:timeline:end-->
<!--docz:decisions:start-->
<!--docz:decisions:end-->
<!--docz:references:start-->
<!--docz:references:end-->
```

There is no schema language: a skeleton is read by the same walker as a
document (`validate.SchemaFromMarkers`), so nothing a schema can require is
something a document cannot show (DESIGN-0015 §3). Every kind listed is
required at least once under the same parent; a kind *not* listed is optional
and still checked by its content rule when present. A schema therefore only
tightens by growing.

The template and its skeleton must yield the **same** `validate.Schema`, pinned
by a derivation test, so the pair can only ever be edited together. Documents
may override the choice of skeleton with a `schema:` frontmatter field; an empty
or absent field means the document's own type name, which is what every
built-in template ships.

### Step 3: Add the index header template

Create `pkg/doczcore/doctemplate/templates/index_postmortem.md`. This is written to
`docs/postmortem/README.md` by `docz init` and must include the auto-generated
markers so `docz update` can splice the table:

```markdown
# Postmortems

Description of what postmortem documents are for.

<!-- BEGIN DOCZ AUTO-GENERATED -->
<!-- END DOCZ AUTO-GENERATED -->
```

### Step 4: Embed pick-up

The `//go:embed templates/*.md` directive in
`pkg/doczcore/doctemplate/embed.go`
picks the new files up automatically. The Phase 8 consistency tests in
`pkg/doczcore/config/doctype_test.go`
(`TestDocTypeRegistry_AllHaveEmbeddedTemplate` and
`TestDocTypeRegistry_AllHaveEmbeddedIndexHeader`) fail loudly if either
template is missing.

### Step 5: Register the type in `pkg/doczcore/config/doctype.go`

Append one entry to the `allDocTypes` slice. This is the only Go code
change required — `DefaultConfig().Types`, `Wiki.NavTitles`,
`DocTypeNames()`, the `typeAliases` map, `Config.EnabledTypes()`, and the
`valid types` list in `Config.ValidateType` are all derived from it.

```go
// pkg/doczcore/config/doctype.go
{
    Name:    "postmortem",
    Aliases: nil, // or []string{"pm"}
    DefaultConfig: func() TypeConfig {
        return TypeConfig{
            Enabled:     true,
            Dir:         "postmortem",
            IDPrefix:    "PM",
            IDWidth:     4,
            Statuses:    []string{"Draft", "In Review", "Final"},
            StatusField: "status",
            PluralLabel: "Postmortems",
        }
    },
    NavTitle:     "Postmortems",
    PluralLabel:  "Postmortems",
    TemplateName: "postmortem",
},
```

`DefaultConfig` is a `func() TypeConfig` (not a value) so each lookup
yields a fresh `Statuses` slice — a Config that mutates `Statuses` won't
poison the next caller (DESIGN-0004 §E).

### Step 6: Add the type package

A built-in is a structured type, so it also needs `pkg/postmortem/` — four files,
the same four every type package has (DESIGN-0014 §2):

| File | Holds |
|------|-------|
| `doc.go` | the `Doc` struct, one field per region the type reads, plus any lookups |
| `headings.go` | the `kind*` constants and `var headings kinds.HeadingSpec`, exported through `Headings()` |
| `parse.go` | `Parse(doc []byte) (Doc, error)` |
| `validate.go` | `Validate(doc []byte) []validate.Finding` and its `Code*` constants |

Copy the nearest existing package and change the table. The contract each one
keeps:

- **`Parse` never touches the filesystem** and fails for exactly two things: no
  frontmatter, and CR line endings. Everything else a document might be missing
  leaves its field zero for `validate.Document` to report against the schema — a
  half-written document is the normal state of a document, and a parser that
  refused one would be useless during the review it is written for.
- **`Parse` switches on the region kind, never on the document's type name**
  (ADR-0002 R7). That is what lets a custom type carrying the same regions parse
  with a built-in's package.
- **Line numbers are document lines.** The `kinds` readers number from the start
  of the region they were handed, so pass every result through the matching
  `kinds.Shift*` helper rather than adding `region.Start` by hand.
- **Codes are `<type>.<family>.<rule>`**, declared as exported constants because
  a consumer filters on them. A document `Parse` rejected yields exactly one
  `<type>.parse` finding and nothing else, since every other check reads a
  parsed `Doc`.
- **Imports stop at L0 plus `kinds` and `validate`.** A type package may import
  `docparse`, `document`, `config`, `kinds`, and `validate`, and nothing else
  under `pkg/`. The core must never import a type package;
  `pkg/doczcore/layer_test.go` fails if it does.

The heading table must equal `kinds.SpecFromTemplate` over the type's embedded
template, so copy that test too — it is what keeps the table and the template
from drifting apart. Then add a golden corpus under `pkg/postmortem/testdata/`:
real documents snapshotted as `.orig.md` (never read from `docs/` at test time),
their generated marked `.md` siblings, `.golden.txt` fact files, and a
`FuzzParse`. See `pkg/impl/golden_test.go` for the harness and any
`testdata/README.md` for how the corpus is documented.

### Step 7: Optionally extend the help string

`config.TypesHelp` is still a static string. The
`TestDocTypeRegistry_DocTypeNamesMatchesTypesHelp` test fails if the
new type's canonical name is missing from the help block — add a line
for the new type so `docz --help` lists it.

### Step 8: Add golden file test fixtures

Run the template tests with `-update` to generate new golden files:

```bash
go test ./pkg/doczcore/doctemplate/... -update
go test ./pkg/postmortem/... -update      # the corpus fact files from Step 6
```

This creates `testdata/golden/postmortem.md` from a sample render, and the
`.golden.txt` fact files for each corpus fixture. **Review both before
committing** — a golden nobody read pins whatever the code happened to do.

### Step 9: Verify

```bash
just build
./build/bin/docz init --force    # creates docs/postmortem/README.md
./build/bin/docz create postmortem "First Postmortem"
./build/bin/docz list postmortem
./build/bin/docz template show postmortem
just ci
```

Once `docz validate` lands (IMPL-0018 Phase 5) also run
`./build/bin/docz validate postmortem`, which checks a created document against the
Step 2 skeleton and should report nothing.

## Custom Types via Configuration

Users can add custom document types by extending the `types` map in `.docz.yaml`
without modifying the `docz` source code. Custom types use a local or config-
specified template and are not validated against the built-in type list.

```yaml
# .docz.yaml
types:
  runbook:
    enabled: true
    dir: runbooks
    id_prefix: RUN
    id_width: 4
    template: docs/templates/runbook.md   # path to a custom template
    statuses:
      - Draft
      - Active
      - Retired
```

Create a template at the specified path:

```bash
docz template export rfc docs/templates/runbook.md   # start from an existing template
$EDITOR docs/templates/runbook.md                    # customize it
```

Then create documents with the custom type:

```bash
docz create runbook "Database Failover Procedure"
# → docs/runbooks/0001-database-failover-procedure.md
```

A custom type resolves by canonical name, by any `aliases` entry, and by its
`id_prefix`, all case-insensitively, so `docz create RUN`, `run`, and `runbook`
all reach the same type. `Config.EnabledTypes()` includes it — built-ins first in
registry order, then custom types sorted — so no-argument `docz init`, `update`,
`list`, and `wiki update` all reach it, and `init` scaffolds its directory and
README like any other type.

**What a custom type does not get:**

- No `pkg/<type>` reader. Nothing parses its regions into a typed `Doc`, so
  `docz validate` checks only that its markers pair and that any kind it reuses
  satisfies that kind's content rule.
- No embedded template. `docz create` needs one at the type's `template` path or
  at `docs/templates/<type>.md`, and says so by naming the file when it finds
  neither.
- No embedded index header. `doctemplate.ResolveIndexHeader` renders the generic
  one from the type's plural label instead (DESIGN-0006 Decision 3).
- A startup warning that the type is not built in, which is advisory only.

### `plan` is the worked example

ADR-0003 removes `plan` from the built-in catalogue on the v2 line, and a repo
that already has `docs/plan` keeps it by keeping its `types.plan` block — the
block now declares a custom type. Everything above applies: `init`, `update`,
`list`, and `wiki update` all still reach the directory, and only `docz create
plan` stops working until the repo supplies `docs/templates/plan.md`.

```bash
docz template export impl docs/templates/plan.md   # closest built-in
$EDITOR docs/templates/plan.md
```

`cmd/legacy_plan_test.go` is the promise: it loads a v1-era config with the block
still in it and asserts that the type resolves three ways, that `init`, `update`,
and `list` all see the document, and that `create` fails by naming the template
path rather than by saying the type does not exist.

## Template System Internals

Templates use Go's `text/template` package. The `TemplateData` struct is:

```go
type TemplateData struct {
    Number   string // zero-padded: "0001"
    Title    string // as provided by the user
    Date     string // YYYY-MM-DD
    Author   string // resolved from flag/config/git/fallback
    Status   string // first configured status by default
    Type     string // document type, e.g. "rfc"
    Prefix   string // ID prefix, e.g. "RFC"
    Slug     string // kebab-case title
    Filename string // full filename: "0001-my-title.md"
}
```

### Slug generation

`Slugify(title)` applies these transformations in order:

1. Lowercase
2. Spaces → hyphens
3. Strip non-alphanumeric, non-hyphen characters (handles unicode by stripping)
4. Collapse multiple hyphens
5. Trim leading/trailing hyphens
6. Truncate at 64 characters on a word boundary

Empty slug after these transforms is valid — the document is still created.

## Config Deep Merge Behavior

The config loader deep-merges nested maps recursively but replaces slices
entirely. This
means a repo config that sets `types.rfc.statuses` replaces the whole list —
it does not append to the global default. This is intentional: a status list
should be a complete, coherent set, not a combination of global and local
entries.

To keep global defaults and extend them, explicitly list all statuses in the
repo config.

## Version Injection

Version and commit information are injected at build time via ldflags:

```just
-X {{ module_path }}/cmd.Version={{ version }}
-X {{ module_path }}/cmd.Commit={{ commit_hash }}
```

`module_path` is `github.com/donaldgifford/docz/v2` — the import path with
its major-version suffix. A `-X` flag that names a path no package has is
ignored without a warning, so the suffix is load-bearing: drop it and
`docz version` reports `dev` from a release build. v1.x tags keep the
unversioned path and its ldflags.

The variables live in `cmd/version.go`:

```go
var (
    Version = "dev"
    Commit  = "none"
)
```

## Releasing

Two paths, and on the v2 line only the second one is used.

### The label path (how v1 shipped)

`.github/workflows/release.yml` runs on every push to `main`. It reads the
merged pull request's label, and `jefflinse/pr-semver-bump` computes and
pushes the next tag; goreleaser then builds the release. One of `major`,
`minor`, `patch`, or `dont-release` must be on the PR or **the job fails**:
a missing label is an error, not a skip.

### The beta path (the whole v2 line)

Nothing on the v2 line is released by label. Every pull request carries
`dont-release`, and a beta is cut by hand from the merge commit:

```bash
just release v2.0.0-beta.1     # tags and pushes
```

`.github/workflows/prerelease.yml` fires on `v*-beta.*`, runs goreleaser
only, and `prerelease: auto` marks the GitHub release from the suffix.

A `/v2` module may not carry a v1 tag: `go get` rejects the mismatch. That
is the hard reason every PR after the module move is `dont-release`, not a
matter of taste.

### The two stability tiers

Not every public package is equally settled, and a consumer has to be able to
tell which is which from `go doc` alone (ADR-0002 Decision 7, Open Question 1).

**Frozen** — `config`, `document`, `docparse`, `docwrite`, `toc`. Promoted at
v1.0.0 and carried onto the v2 line in their v1 shapes. Additions only. The one
breaking change v2 makes to them is `plan` leaving the catalogue `config`
describes (ADR-0003).

**Experimental** — the eleven the v2 line added: `kinds`, `validate`, `repo`,
`doctemplate`, `index`, `wiki`, and the five type packages. Each one's doc
comment carries this paragraph, and the surface may change between
`v2.0.0-beta.N` tags:

```go
// EXPERIMENTAL until v2.0.0: the surface may change between betas
// (ADR-0002 Decision 7). The five packages frozen at v1.0.0 are not
// affected; this one is not among them.
```

It sits in the second paragraph rather than the first because `revive`'s
`package-comments` rule and staticcheck's ST1000 both require a package
comment to open with `Package <name>`. `go doc` shows it immediately either
way. **A new public package added before the v2.0.0 cut needs this paragraph**;
the cut is what removes all of them, and from then on the whole surface is
contract.

### What pr-semver-bump v1.7.4 actually does with a beta tag

Read from the action's source at that tag (ADR-0002 Open Question 3),
because the answer decides whether the real `v2.0.0` can be cut by label at
all:

| Question | Answer |
| -------- | ------ |
| How does it find the current version? | The GitHub **git matching-refs API**, not local tags, so `fetch-depth` is irrelevant to it. Every parseable ref is sorted with `semver.rcompare` and the top one wins. |
| Does a pre-release count as latest? | **Yes.** The only filter is "does it parse", with no pre-release check, and `2.0.0-beta.1` outranks `1.2.2` by precedence. |
| `major` on top of `v2.0.0-beta.1`? | `v2.0.0`, via npm `semver.inc`. The intended cut works. |
| `minor` or `patch` on top of it? | **Also `v2.0.0`.** semver short-circuits any increment from a pre-major pre-release. |
| No tags at all? | `0.0.0` is the base, so `major` gives `v1.0.0`. |
| A typo'd tag such as `v2.0.0.beta.1`? | Silently unparseable, so discovery falls back to `v1.2.2` with no warning. |

Two consequences worth knowing before they bite:

- **Any release label merged during the beta window cuts the real
  `v2.0.0`,** not just `major`. A single `patch`-labelled PR would consume
  the version, and the later deliberate `major` PR would then compute
  `v3.0.0`. This is why `dont-release` is not optional.
- **`base-branch` is off,** so discovery sees every tag in the repository
  regardless of branch. Once a beta tag is the highest semver tag, a
  `patch`-labelled PR on `main` yields `v2.0.0` rather than `v1.2.3`, so
  **v1 maintenance cannot use the label path.**

**Fallback.** If the `major` label misbehaves when v2.0.0 is finally cut,
tag it by hand and let `prerelease.yml`'s sibling `release.yml` stay out of
it:

```bash
just release v2.0.0
```

### The v1 maintenance branch

There is no `v1` branch and none is created until something needs one. Cut
it from `v1.2.2` on demand:

```bash
git switch -c v1 v1.2.2
```

It keeps the unversioned module path. Release from it by hand
(`just release v1.2.3`) for the reason above: the label path would
compute a v2 version from the repository's highest tag.

## Testing Patterns

### Filesystem tests

Use `t.TempDir()` — the standard library creates and cleans up the directory
automatically at the end of the test. Do not use `afero` or other filesystem
abstractions.

```go
func TestCreate(t *testing.T) {
    dir := t.TempDir()
    // ... create files in dir
}
```

### Golden files

Golden files live under `testdata/golden/`. Update them with:

```bash
go test ./pkg/doczcore/doctemplate/... -update
```

Do not hand-edit golden files; always regenerate them via `-update` and review
the diff.

### Time injection

`docwrite.Create` stamps the document's `created:` date from
`CreateOptions.CreatedAt`; a zero value falls back to `time.Now()`. Tests (and
`cmd/create.go`, which sources it from `runner.Now()`) pin the date by setting
the field explicitly — no package-level clock global to reset:

```go
opts := docwrite.CreateOptions{
    // ...
    CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
}
```

### Config in command tests

Tests that exercise command functions (`runCreate`, `runList`, etc.) set `appCfg`
directly:

```go
appCfg = config.DefaultConfig()
appCfg.DocsDir = filepath.Join(t.TempDir(), "docs")
```

## Just Recipes

| Recipe | Description |
|--------|-------------|
| `just build` | Build the binary to `build/bin/docz` |
| `just test` | Run all tests |
| `just test-coverage` | Tests with coverage report |
| `just test-consumer` | Run the external-module consumer smoke test (separate `go.mod`) |
| `just parity` | Replay the v1.2.2 parity goldens (`bin=<path>` to drive another binary) |
| `just parity-capture` | Re-install v1.2.2 and re-capture the goldens (see `test/parity/README.md` first) |
| `just validate` | Run `docz validate` over this repo's own `docs/`, non-strict |
| `just lint` | Run golangci-lint |
| `just lint-fix` | Auto-fix lint issues |
| `just fmt` | Run gofmt + goimports |
| `just ci` | lint + test + test-consumer + parity + validate + build + license-check |
| `just release vX.Y.Z` | Tag and push a release by hand |
| `just release-local` | Goreleaser snapshot, nothing published |
| `just docs-init` | Run `docz init` |
| `just docs-update` | Run `docz update` |
| `just docs-list` | Run `docz list` |
| `just docs-config` | Run `docz config` |
