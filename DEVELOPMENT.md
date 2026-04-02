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
├── internal/
│   ├── config/
│   │   └── config.go        # Config structs, Load(), Validate(), DefaultConfig()
│   ├── document/
│   │   ├── document.go      # Frontmatter struct, ParseFrontmatter()
│   │   ├── create.go        # Create(), nextID(), document file writing
│   │   └── time.go          # var timeNow (overridable in tests)
│   ├── index/
│   │   └── index.go         # ScanDocuments(), GenerateTable(), UpdateReadme(), DryRunReadme()
│   ├── template/
│   │   ├── embed.go          # //go:embed, EmbeddedDocumentTemplate(), EmbeddedIndexHeader()
│   │   ├── template.go       # Slugify(), Resolve(), Render(), TemplateData
│   │   └── templates/        # embedded template files
│   │       ├── rfc.md
│   │       ├── adr.md
│   │       ├── design.md
│   │       ├── impl.md
│   │       ├── plan.md
│   │       ├── investigation.md
│   │       ├── index_rfc.md
│   │       ├── index_adr.md
│   │       ├── index_design.md
│   │       ├── index_impl.md
│   │       ├── index_plan.md
│   │       ├── index_investigation.md
│   │       └── wiki_index.md
│   ├── toc/
│   │   └── toc.go            # Slugify(), ParseHeadings(), GenerateToC(), UpdateToC()
│   └── wiki/
│       ├── titles.go         # DirTitle(), DocTitle(), FilenameTitle()
│       ├── wiki.go           # NavEntry, ScanDocs(), SortEntries(), CountPages()
│       └── mkdocs.go         # ReadMkDocs(), WriteMkDocs(), NavToYAML(), MergeNavOrder()
└── testdata/
    └── golden/              # golden file fixtures for template, toc, and wiki tests
```

## Package Responsibilities

### `internal/config`

Loads and validates the `docz` configuration. The entry point is `Load()`.

**Config precedence (lowest to highest):**
1. Built-in defaults (`DefaultConfig()`)
2. Global config (`~/.docz.yaml`)
3. Repo config (`.docz.yaml`)
4. Flags (`--docs-dir`, etc.)

Deep merge is implemented with two Viper instances: global config loaded first,
repo config merged on top via `MergeConfigMap`. This means the repo config
overrides only the keys it explicitly sets; unset keys inherit from global.
Slices (e.g. `statuses`) are replaced entirely, not appended.

```go
cfg, err := config.Load(cfgFile)
cfg.Validate()  // returns (warnings []string, err error)
```

### `internal/template`

Handles template resolution and rendering.

**Resolution order (first match wins):**
1. `types.<type>.template` path in config (absolute or relative to repo root)
2. `<docs_dir>/templates/<type>.md` (local override file)
3. Embedded default (`internal/template/templates/<type>.md`)

```go
content, err := template.Resolve(docType, tc.Template, cfg.DocsDir)
rendered, err := template.Render(content, &template.TemplateData{...})
```

`Slugify(title)` converts a title to kebab-case, strips non-alphanumeric
characters, and truncates to 64 characters on a word boundary.

`ResolveWikiIndex(docsDir)` resolves the wiki homepage template:
1. Local override at `<docs_dir>/templates/wiki_index.md`
2. Embedded default (`internal/template/templates/wiki_index.md`)

`RenderWikiIndex(tmpl, data)` renders the template with `WikiIndexData`
(site name and enabled types).

### `internal/document`

Creates document files on disk.

```go
result, err := document.Create(&document.CreateOptions{
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
// result.Path      → "docs/rfc/0001-my-proposal.md"
// result.Number    → "0001"
```

`nextID()` scans the target directory for `NNNN-*.md` files and returns
`max(existing IDs) + 1`. It starts at 1 if the directory is empty or missing.

### `internal/index`

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

### `internal/toc`

Generates table of contents for markdown documents. Uses `<!--toc:start-->` /
`<!--toc:end-->` markers (compatible with markdown-toc.nvim). Single file:

- **`toc.go`** — `Slugify()` generates GitHub-compatible anchor slugs.
  `ParseHeadings()` extracts H2-H6 headings, skipping H1, fenced code blocks,
  and inline markdown. Handles duplicate slug suffixes (`-1`, `-2`).
  `GenerateToC()` builds indented markdown list with relative indentation.
  `UpdateToC()` splices the generated ToC between markers.

### `internal/wiki`

Generates and maintains MkDocs nav from the docs directory tree. Split across
three files:

- **`titles.go`** — Title extraction: `DirTitle()` maps directory names to
  nav titles using configurable overrides. `DocTitle()` extracts titles from
  frontmatter, H1 headings, or filename fallback.
- **`wiki.go`** — Nav tree building: `ScanDocs()` recursively walks the docs
  directory and builds a `[]NavEntry` tree. `SortEntries()` sorts top-level
  entries (Home first, rest alphabetical). `CountPages()` counts leaf entries.
- **`mkdocs.go`** — MkDocs YAML I/O: `ReadMkDocs()`/`WriteMkDocs()` preserve
  non-nav fields. `NavToYAML()` converts `[]NavEntry` to MkDocs nav format.
  `MergeNavOrder()` preserves existing section order when updating.

## Adding a Built-In Document Type

To add a new document type to `docz` (e.g. `plan`):

### Step 1: Add the document template

Create `internal/template/templates/plan.md`. The file is a Go `text/template`
with access to all `TemplateData` fields.

```
internal/template/templates/plan.md
```

Available template variables: `{{ .Number }}`, `{{ .Title }}`, `{{ .Slug }}`,
`{{ .Filename }}`, `{{ .Date }}`, `{{ .Author }}`, `{{ .Status }}`, `{{ .Type }}`,
`{{ .Prefix }}`.

### Step 2: Add the index header template

Create `internal/template/templates/index_plan.md`. This file is written to
`docs/plan/README.md` when `docz init` is run. It must include the auto-generated
markers so that `docz update` can splice the table:

```markdown
# Plans

Description of what plan documents are for.

<!-- BEGIN DOCZ AUTO-GENERATED -->
<!-- END DOCZ AUTO-GENERATED -->
```

### Step 3: Register the type in `embed.go`

The `//go:embed templates/*.md` directive picks up the new files automatically
since they match the glob. No changes needed to `embed.go` unless you need to
handle the new type name explicitly.

Verify `EmbeddedDocumentTemplate("plan")` returns the content:

```go
// internal/template/embed.go
// No changes needed — the glob covers *.md files automatically.
```

### Step 4: Add default config in `config.go`

Add the new type to `DefaultConfig()`:

```go
// internal/config/config.go
"plan": {
    Enabled:     true,
    Dir:         "plan",
    IDPrefix:    "PLAN",
    IDWidth:     4,
    Statuses:    []string{"Draft", "In Progress", "Completed", "Cancelled"},
    StatusField: "status",
},
```

### Step 5: Add the type to `ValidTypes()`

```go
func ValidTypes() []string {
    return []string{"rfc", "adr", "design", "impl", "plan", "investigation"}
}
```

### Step 6: Add golden file test fixtures

Run the template tests with `-update` to generate new golden files:

```bash
go test ./internal/template/... -update
```

This creates `testdata/golden/plan.md` from a sample render. Review it before
committing.

### Step 7: Update tests

The golden tests in `internal/template/golden_test.go` iterate over the types
from `config.ValidTypes()`. Adding `"plan"` to that list automatically includes
it in the golden test suite once the fixture file exists.

Add a test case for the new type in `internal/template/template_test.go` if it
has any template-specific behavior.

### Step 8: Verify

```bash
make build
./build/bin/docz init --force    # creates docs/plan/README.md
./build/bin/docz create plan "First Plan"
./build/bin/docz list plan
./build/bin/docz template show plan
make ci
```

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

**Limitations of custom types in v1:**
- Custom types do not appear in `ValidTypes()` and will emit a config warning
- `docz init` does not create directories for custom types automatically — create
  the directory and its README manually or use `docz update runbook`
- The `list` command includes custom types if their directories exist

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

Viper deep-merges nested maps recursively but replaces slices entirely. This
means a repo config that sets `types.rfc.statuses` replaces the whole list —
it does not append to the global default. This is intentional: a status list
should be a complete, coherent set, not a combination of global and local
entries.

To keep global defaults and extend them, explicitly list all statuses in the
repo config.

## Version Injection

Version and commit information are injected at build time via ldflags:

```makefile
-X github.com/donaldgifford/docz/cmd.Version=$(VERSION)
-X github.com/donaldgifford/docz/cmd.Commit=$(COMMIT_HASH)
```

The variables live in `cmd/version.go`:

```go
var (
    Version = "dev"
    Commit  = "none"
)
```

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
go test ./internal/template/... -update
```

Do not hand-edit golden files; always regenerate them via `-update` and review
the diff.

### Time injection

`internal/document/time.go` exports `var timeNow = time.Now` so tests can
override the current time:

```go
document.TimeNow = func() time.Time {
    return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
}
t.Cleanup(func() { document.TimeNow = time.Now })
```

### Config in command tests

Tests that exercise command functions (`runCreate`, `runList`, etc.) set `appCfg`
directly:

```go
appCfg = config.DefaultConfig()
appCfg.DocsDir = filepath.Join(t.TempDir(), "docs")
```

## Makefile Targets

| Target | Description |
|--------|-------------|
| `make build` | Build the binary to `build/bin/docz` |
| `make test` | Run all tests |
| `make test-coverage` | Tests with coverage report |
| `make lint` | Run golangci-lint |
| `make lint-fix` | Auto-fix lint issues |
| `make fmt` | Run gofmt + goimports |
| `make ci` | Full CI pipeline (lint + test + build + license-check) |
| `make docs-init` | Run `docz init` |
| `make docs-update` | Run `docz update` |
| `make docs-list` | Run `docz list` |
| `make docs-config` | Run `docz config` |
