---
id: DESIGN-0007
title: "docz changes to support docz-api and docz-site"
status: Draft
author: Donald Gifford
created: 2026-06-23
---
<!-- markdownlint-disable-file MD025 MD041 -->

# DESIGN 0007: docz changes to support docz-api and docz-site

**Status:** Draft
**Author:** Donald Gifford
**Date:** 2026-06-23

<!--toc:start-->
- [Overview](#overview)
- [Goals and Non-Goals](#goals-and-non-goals)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Background](#background)
- [Detailed Design](#detailed-design)
  - [What becomes public](#what-becomes-public)
  - [Proposed package name and path](#proposed-package-name-and-path)
  - [Move-wholesale vs. shim — recommendation](#move-wholesale-vs-shim--recommendation)
  - [How existing packages relate afterward](#how-existing-packages-relate-afterward)
  - [Key exported signatures (post-promotion)](#key-exported-signatures-post-promotion)
  - [The semver obligation that promotion creates](#the-semver-obligation-that-promotion-creates)
  - [How docz-api pins and consumes it](#how-docz-api-pins-and-consumes-it)
  - [Optional: docz export --json (secondary)](#optional-docz-export---json-secondary)
- [API / Interface Changes](#api--interface-changes)
- [Data Model](#data-model)
  - [Exported Go shapes (the contract)](#exported-go-shapes-the-contract)
  - [Optional JSON manifest schema (only if docz export ships)](#optional-json-manifest-schema-only-if-docz-export-ships)
- [Testing Strategy](#testing-strategy)
- [Migration / Rollout Plan](#migration--rollout-plan)
- [Open Questions](#open-questions)
  - [1. What is the public package name/path and shape?](#1-what-is-the-public-package-namepath-and-shape)
  - [2. Move wholesale (update all imports) vs. leave permanent re-export shims?](#2-move-wholesale-update-all-imports-vs-leave-permanent-re-export-shims)
  - [3. Add docz export --json now, or defer it?](#3-add-docz-export---json-now-or-defer-it)
  - [4. Module/versioning strategy?](#4-moduleversioning-strategy)
  - [5. How much surface to expose?](#5-how-much-surface-to-expose)
  - [6. If docz export ships, what is the manifest schema shape?](#6-if-docz-export-ships-what-is-the-manifest-schema-shape)
  - [7. Does docz-api vendor a docz checkout or always consume the library via go.mod?](#7-does-docz-api-vendor-a-docz-checkout-or-always-consume-the-library-via-gomod)
  - [8. Where does the consumer import smoke test live?](#8-where-does-the-consumer-import-smoke-test-live)
- [References](#references)
<!--toc:end-->

## Overview

INV-0005 concluded that a cross-repo aggregation service (**docz-api**) and a
viewer (**docz-site**) are feasible, and that the dominant simplification over a
general system like rfc-api is that docz already standardizes location
(`.docz.yaml`), structure (typed directories), and metadata (typed
frontmatter). The investigation's Decision 7 (accepted) commits to one
prerequisite *inside the docz CLI repo*: stop the API from re-implementing —
and therefore drifting from — docz's own config and frontmatter parsing, by
**promoting that parsing out of `internal/` into a reusable, importable public
Go package** that both the CLI and docz-api consume.

This design is the docz-side enabler and nothing more. It does not design the
service or the viewer (those are DESIGN-0008 and DESIGN-0009). It specifies
exactly which types and functions become public, the package name and path, the
package boundary (what stays `internal/` vs. what is promoted), how the existing
`internal/config` and `internal/document` packages relate to the new package
afterward, the semver obligation that promotion creates, how docz-api pins and
consumes the result, and — as a clearly secondary, optional item — whether a
`docz export --json` whole-repo manifest is worth shipping now for non-Go
consumers.

The work is behavior-preserving for the CLI: no command output changes, no new
required flags, no config-format change. It is a code-organization change plus a
new public import surface and a tagged release.

## Goals and Non-Goals

### Goals

- **One source of truth for "what docz docs exist in a tree."** docz-api imports
  the same `Load` / `Validate` / type-resolution and `ScanDocuments` /
  `LoadFrontmatter` code the CLI runs, so a custom type, an alias, or a
  frontmatter quirk parses identically in both.
- **Promote the parsing core to an importable public package.** Today the
  relevant code lives under `internal/config` and `internal/document`, which Go
  forbids any other module from importing. Move the consumable surface to a
  `pkg/…` path that external modules can import.
- **Keep the CLI behavior byte-for-byte identical.** All `cmd/` packages
  continue to compile and produce the same output; golden files do not change.
- **Establish a deliberate, minimal public API surface.** Promote what an
  ingester actually needs (config load + validate + type resolution + document
  scan + frontmatter parse) and keep CLI-only concerns (`SetStatus` byte
  mutation, template rendering, index/ToC splicing, wiki nav) out of the public
  surface unless there is a concrete consumer.
- **Acknowledge the semver obligation that promotion creates** and pick a
  module/versioning strategy docz-api can pin against.

### Non-Goals

- **Designing docz-api or docz-site.** Fetch strategy, Postgres schema, webhook
  refresh, Meilisearch indexing, and the pluggable auth layer are all
  out of scope here (DESIGN-0008 / DESIGN-0009).
- **Changing the `.docz.yaml` format or any frontmatter contract.** The shapes
  are promoted as-is; this is a visibility change, not a schema change.
- **Building a server-side renderer.** Decision 3 of INV-0005 renders markdown
  in docz-site (JS); docz exposes no rendering API here.
- **Exposing the template / index / toc / wiki packages.** Those are CLI output
  concerns the API does not need; they stay `internal/`.
- **Auto-discovering types from the filesystem.** A type is still whatever
  `.docz.yaml` declares (DESIGN-0006); the public package reads that config, it
  does not invent types.

## Background

INV-0005 Observation 4 ("don't re-implement the parser") laid out three ways the
API could obtain a repo's doc list: (1) a shared Go library, (2) a
machine-readable `docz export --json`, or (3) shelling out to the binary.
Decision 7 chose **option 1** — the shared library — because it makes docz-api a
true peer of the CLI with zero drift risk, where shelling out couples deployment
to a checkout + installed binary and a JSON export is a serialization the API
must still trust to match the CLI.

The thing standing in the way is purely Go's visibility rule. Everything the API
wants is already written and tested; it just lives in import-blocked locations:

- `internal/config` — `Config`, `TypeConfig`, `DocType`, `Status`,
  `DocTypeDef`, `Load`, `Validate`, `DefaultConfig`, `EnabledTypes`,
  `ValidateType`, `resolveType`, `DocTypeNames`, `AllDocTypes`,
  `LookupDocType`, `ResolveTypeAlias`.
- `internal/document` — `Frontmatter`, `DocEntry`, `ScanDocuments`,
  `LoadFrontmatter`, `ParseFrontmatter`, `IsDoczFile`, `SetStatus` (and its
  sentinels `ErrNoFrontmatter`, `ErrStatusFieldMissing`,
  `ErrUnsupportedLineEndings`).

An ingester reading a checked-out (or API-fetched) repo needs almost exactly the
sequence the CLI already runs:

```text
Load(configFile, repoRoot) → Validate() → for each EnabledTypes():
    ScanDocuments(cfg.TypeDir(name)) → []DocEntry{ Frontmatter, Filename, Content }
```

That maps one-to-one onto the registry row described in INV-0005 Observation 1
(`id`, `title`, `status`, `author`, `created`, path) and onto the per-type
metadata (`dir`, `id_prefix`, `id_width`, `statuses`, `plural_label`,
`aliases`). The registry must be **type-agnostic** — driven entirely by the
repo's config, never a hardcoded list of the six built-ins (INV-0005 Obs 1,
DESIGN-0006). Promoting `Config` + `EnabledTypes` + `ScanDocuments` is precisely
what makes that possible without a second parser.

One subtlety worth stating up front: `document.Frontmatter.Status` is typed
`config.Status` and the registry's `DocType` is typed `config.DocType` — so
`document` already depends on `config`. Any promotion must keep those two
packages on the same side of the public/internal line, or the typed fields break
the import graph. This design keeps them together in one public module.

## Detailed Design

### What becomes public

The promoted surface is the union of the two packages' export-worthy symbols,
trimmed to what an external consumer (docz-api) actually needs. Concretely:

**From `internal/config`:**

| Symbol | Kind | Why an external consumer needs it |
|---|---|---|
| `Config` | struct | The parsed `.docz.yaml`; the manifest |
| `TypeConfig` | struct | Per-type metadata (dir, id_prefix, statuses, …) |
| `DocType`, `Status` | typed strings | Boundary types on frontmatter / type fields |
| `DocTypeDef`, `AllDocTypes`, `LookupDocType`, `DocTypeNames` | registry | Enumerate/validate built-in types |
| `Load(configFile, repoRoot string) (Config, error)` | func | Parse a repo's config |
| `DefaultConfig() Config` | func | Defaults when no `.docz.yaml` present |
| `(*Config).Validate() ([]string, error)` | method | Reject ambiguous/invalid config before ingest |
| `(*Config).EnabledTypes() []string` | method | The type set to scan (incl. custom types) |
| `(*Config).TypeDir(string) string` | method | Resolve `<docs_dir>/<dir>` to scan |
| `(*Config).ValidateType(string) (string, error)` | method | Canonicalize a user/URL token to a type |
| `ResolveTypeAlias`, `ErrUnknownType` | func / sentinel | Alias resolution + typed error branch |

**From `internal/document`:**

| Symbol | Kind | Why an external consumer needs it |
|---|---|---|
| `Frontmatter` | struct | The five typed metadata fields |
| `DocEntry` | struct | `Frontmatter` + `Filename` + `Content` per doc |
| `ScanDocuments(dir string) ([]DocEntry, error)` | func | Scan one type directory |
| `LoadFrontmatter(path string) (Frontmatter, []byte, error)` | func | Parse one file (with fallback semantics) |
| `ParseFrontmatter(content []byte) (Frontmatter, error)` | func | Parse already-fetched bytes (no disk read) |
| `IsDoczFile(name string) bool` | func | Filter a Trees/Contents listing |
| `ErrNoFrontmatter` | sentinel | `errors.Is` fallback branch |

`ParseFrontmatter` deserves emphasis: docz-api fetches bytes over the GitHub
Contents API (INV-0005 Decision 1) and never touches a local checkout, so a
content-from-bytes entry point — not just the disk-reading `LoadFrontmatter` —
is the one the service uses most. It already exists; promotion just exposes it.

**What stays `internal/` (recommended):**

- `SetStatus` and its CR/CRLF and status-field-missing sentinels — a byte-level
  *mutator* the CLI's `status set` owns. The API ingests read-only (default
  branch HEAD, Decision 4/5); it does not write back. Keep it internal until a
  consumer appears (tracked as an open question).
- `internal/template`, `internal/index`, `internal/toc`, `internal/wiki` —
  output/rendering/splicing concerns docz-site replaces with a JS renderer
  (Decision 3). No API need; keep internal.
- `cmd/`, `cmd.Runner` — the CLI shell.

### Proposed package name and path

Recommended: preserve the existing two-package split under `pkg/` so the typed
`Status`/`DocType` boundary and the existing test layout move wholesale with
minimal churn, grouped under a `doczcore` namespace:

```text
github.com/donaldgifford/docz
├── pkg/
│   └── doczcore/
│       ├── config/        # was internal/config (moved wholesale)
│       │   ├── config.go
│       │   ├── doctype.go
│       │   └── constants.go
│       └── document/      # was internal/document (moved wholesale)
│           ├── document.go
│           ├── scan.go
│           └── status.go  # SetStatus stays here but is a candidate to keep
│                          #   internal; see open question 5
├── internal/
│   ├── template/          # unchanged, now imports pkg/doczcore/config
│   ├── index/
│   ├── toc/
│   └── wiki/
└── cmd/                   # unchanged behavior, imports updated
```

`pkg/doczcore/config` and `pkg/doczcore/document` keep the package names
`config` and `document`, so within the moved files almost nothing changes —
only the *import paths* that reference them elsewhere do. The `doczcore` grouping
namespaces the public surface and signals "this is the importable core,"
distinct from the CLI-private `internal/` tree.

(If a single flat package is preferred for a smaller surface, `pkg/doczcore`
with both files merged is viable but forces a rename of every `config.` /
`document.` qualifier across the moved code and the CLI — more churn for a
cosmetic gain. The two-subpackage layout is recommended; see open question 1.)

### Move-wholesale vs. shim — recommendation

Two ways to relate `internal/` to the new `pkg/`:

1. **Move wholesale + update all `cmd/` and `internal/` imports.** Physically
   `git mv internal/config → pkg/doczcore/config` and
   `internal/document → pkg/doczcore/document`, then update every import path in
   `cmd/`, `internal/template`, `internal/index`, `internal/toc`,
   `internal/wiki`, and the moved files' cross-references. No code logic
   changes; only import strings and the package's on-disk location.
2. **Move + leave thin re-export shims** at the old `internal/config` /
   `internal/document` paths (`type Config = doczcore.Config`,
   `var Load = doczcore.Load`, …) so existing CLI imports keep compiling
   untouched.

**Recommendation: option 1 (move wholesale, update imports).** Rationale:

- The repo controls all of its own callers — `cmd/` and the four `internal/`
  output packages — so updating imports is a mechanical, compiler-verified sweep
  (`goimports` + build), not a risky refactor. The shim only buys value when
  *external* callers you don't control would break, which is not the case for a
  single repo's own internal imports.
- A permanent shim leaves two import paths for the same type forever, inviting
  confusion ("which `Config` is canonical?") and a slow rot where new code
  imports the deprecated path. Decision 7's whole point is *one* source of
  truth; two paths undercuts it.
- Type aliases (`=`) make a shim *type-identical*, so it would work — but it is
  unjustified indirection for in-repo callers. Drop it.

The one acceptable use of a shim is **transitional**, inside a single PR, if the
import sweep is too large to land atomically — re-export, migrate callers in
follow-ups, delete the shim before release. Even then, the shim must not survive
the release that introduces `pkg/doczcore`.

### How existing packages relate afterward

- `internal/template` keeps importing the config package, now at
  `pkg/doczcore/config` (it already references `config.TemplatesDir` and friends).
  One import line changes per file.
- `internal/index`, `internal/toc`, `internal/wiki` already depend on
  `document.DocEntry` / `config` types; their imports repoint to
  `pkg/doczcore/...`. No logic change.
- `cmd/` handlers (`runner.go` bundles `config.Config`; `create`, `update`,
  `list`, `status`, `init`, `wiki` all reference both packages) repoint their
  imports. `cmd.Runner.Cfg` is `config.Config` → now `doczcore/config.Config`,
  same struct.
- The `internal/document` → `internal/config` dependency becomes
  `pkg/doczcore/document` → `pkg/doczcore/config`, preserved verbatim, so the
  typed `Status` / `DocType` fields keep compiling.

### Key exported signatures (post-promotion)

These are the existing signatures, unchanged except for their import path:

```go
package config // github.com/donaldgifford/docz/pkg/doczcore/config

type DocType string
type Status  string

type TypeConfig struct {
    Enabled     bool
    Dir         string
    Template    string
    IDPrefix    string
    IDWidth     int
    Statuses    []string
    StatusField string
    PluralLabel string
    Aliases     []string
}

type Config struct {
    DocsDir string
    Types   map[string]TypeConfig
    Index   IndexConfig
    Author  AuthorConfig
    Wiki    WikiConfig
    TOC     TOCConfig
}

func Load(configFile, repoRoot string) (Config, error)
func DefaultConfig() Config

func (c *Config) Validate() ([]string, error)
func (c *Config) EnabledTypes() []string
func (c *Config) TypeDir(docType string) string
func (c *Config) ValidateType(name string) (string, error)

func DocTypeNames() []string
func AllDocTypes() []DocTypeDef
func LookupDocType(name string) (DocTypeDef, bool)
func ResolveTypeAlias(name string) string

var ErrUnknownType error
```

```go
package document // github.com/donaldgifford/docz/pkg/doczcore/document

type Frontmatter struct {
    ID      string
    Title   string
    Status  config.Status
    Author  string
    Created string
}

type DocEntry struct {
    Frontmatter
    Filename string
    Content  []byte
}

func ScanDocuments(dir string) ([]DocEntry, error)
func LoadFrontmatter(path string) (Frontmatter, []byte, error)
func ParseFrontmatter(content []byte) (Frontmatter, error)
func IsDoczFile(name string) bool

var ErrNoFrontmatter error
```

A docz-api ingester then becomes, in full:

```go
import (
    "github.com/donaldgifford/docz/pkg/doczcore/config"
    "github.com/donaldgifford/docz/pkg/doczcore/document"
)

func ingest(repoRoot string) ([]document.DocEntry, error) {
    cfg, err := config.Load("", repoRoot) // reads <repoRoot>/.docz.yaml
    if err != nil {
        return nil, err
    }
    if _, err := cfg.Validate(); err != nil {
        return nil, err
    }
    var all []document.DocEntry
    for _, t := range cfg.EnabledTypes() { // incl. custom types
        docs, err := document.ScanDocuments(cfg.TypeDir(t))
        if err != nil {
            return nil, err
        }
        all = append(all, docs...)
    }
    return all, nil
}
```

When fetching over the Contents API (no checkout), the service substitutes its
own directory walk and calls `document.ParseFrontmatter(bytes)` per fetched file
instead of `ScanDocuments(dir)`, reusing `config.Config` + `IsDoczFile` to know
*which* paths to fetch.

### The semver obligation that promotion creates

Moving a symbol from `internal/` to `pkg/` is a one-way commitment: once an
external module imports `pkg/doczcore`, **its types and signatures become part
of docz's public API and are governed by semver**. A field rename on
`TypeConfig`, a signature change on `Load`, or a removal of `ScanDocuments` is
then a breaking change requiring a major-version bump — not the free internal
refactor it is today (compare DESIGN-0006, which freely reshaped
`internal/index` signatures).

This is the real cost of Decision 7 and the reason to keep the promoted surface
**minimal**: every public symbol is a future compatibility constraint. Concrete
implications:

- The promoted structs (`Config`, `TypeConfig`, `Frontmatter`, `DocEntry`)
  freeze their field names/tags as API. Adding fields is non-breaking; renaming
  or removing is breaking.
- `Load` / `Validate` / `ScanDocuments` / `LoadFrontmatter` /
  `ParseFrontmatter` signatures freeze. Future needs should be met with new
  functions or option structs, not signature changes.
- The sentinel errors (`ErrUnknownType`, `ErrNoFrontmatter`) become a supported
  `errors.Is` contract.
- CLI-only churn (template/index/toc/wiki refactors like DESIGN-0006's) stays
  free because those packages remain `internal/`.

### How docz-api pins and consumes it

docz-api adds an ordinary module dependency:

```text
// docz-api/go.mod
require github.com/donaldgifford/docz vX.Y.Z
```

```go
import "github.com/donaldgifford/docz/pkg/doczcore/config"
import "github.com/donaldgifford/docz/pkg/doczcore/document"
```

The docz module is tagged (e.g. `vX.Y.0`, the minor release that introduces
`pkg/doczcore`) and docz-api pins that tag in `go.mod` like any third-party
dependency; Go's module proxy + `go.sum` give reproducible builds. Because the
public surface is semver-governed, docz-api can rely on `go get -u` within a
major version being non-breaking. No `replace` directive, no vendored checkout
of docz's source is required (whether docz-api *additionally* vendors for
hermetic builds is an open question, not a requirement).

Note the module path. If docz is still pre-`v1` (`v0.x`), the
"minor versions may break" caveat of `v0` applies and docz-api pins exact tags;
once docz hits `v1`, the `/v2` import-path rule kicks in for future majors.
Whether to cut a `v1.0.0` *because* this is now a public API, or keep iterating
in `v0.x` with exact pins, is open question 4.

### Optional: `docz export --json` (secondary)

INV-0005 Observation 4 lists a `docz export --json` whole-repo manifest as
option 2 — explicitly *not* the chosen primary. It is worth keeping on the table
as a **convenience for non-Go consumers** (a script, a CI step, a future
non-Go service) that cannot import a Go package, and as a debugging aid
("what does docz think is in this tree?"). It is **not** how docz-api consumes
docz — the library is. Treat it as secondary and gate it behind an open
question (3).

If shipped, it is a thin CLI command over the *same* promoted library — it must
not grow a second parser — emitting the resolved config + every enabled type +
every doc's frontmatter + path as one JSON document. Sketch in
[Data Model](#data-model). Because it serializes the public structs, its JSON
shape also becomes a compatibility surface (golden-tested), which is a second
reason to defer it unless a concrete non-Go consumer materializes.

## API / Interface Changes

- **New public package path(s):** `github.com/donaldgifford/docz/pkg/doczcore/config`
  and `…/pkg/doczcore/document`, containing the symbols listed in
  [What becomes public](#what-becomes-public). These are *moved*, not new code.
- **Removed import paths (internal):** `internal/config` and
  `internal/document` cease to exist (no transitional shim survives release).
  This is invisible outside the repo — `internal/` was never importable.
- **No breaking CLI changes:** no command renamed, no flag added or removed
  (excepting the optional `docz export`, below), no output format changed, no
  `.docz.yaml` schema change. The CLI is a consumer of the moved package.
- **Optional new CLI command (open question 3):** `docz export [--format=json]`,
  read-only, emitting the whole-repo manifest to stdout (respecting the existing
  `Runner.Out` seam). No effect on any existing command. If deferred, no CLI
  change ships at all.
- **New module-level public API surface** subject to semver, per
  [the semver obligation](#the-semver-obligation-that-promotion-creates). This is
  the only "breaking-in-the-future-is-now-expensive" consequence, and it is
  intentional.

## Data Model

No persisted-data, frontmatter, README-marker, or `.docz.yaml` schema changes.
The "data model" of this design is the *exported Go shapes* that become the
contract, plus the optional JSON manifest if `docz export` ships.

### Exported Go shapes (the contract)

The promoted structs, frozen as public API (fields per the real definitions in
`internal/config/config.go` and `internal/document/`):

- `config.Config{ DocsDir, Types map[string]TypeConfig, Index, Author, Wiki,
  TOC }`
- `config.TypeConfig{ Enabled, Dir, Template, IDPrefix, IDWidth, Statuses,
  StatusField, PluralLabel, Aliases }`
- `config.DocType`, `config.Status` (typed strings)
- `document.Frontmatter{ ID, Title, Status config.Status, Author, Created }`
- `document.DocEntry{ Frontmatter, Filename, Content []byte }`

`DocEntry.Content` carries the raw file bytes (`scan.go` populates it
unconditionally) — exactly the `raw_md` the registry caches at ingest (INV-0005
Decision 2). The API does not need a separate read.

### Optional JSON manifest schema (only if `docz export` ships)

A single object — resolved config, every enabled type, every doc:

```json
{
  "schema_version": 1,
  "docs_dir": "docs",
  "types": [
    {
      "name": "rfc",
      "dir": "rfc",
      "id_prefix": "RFC",
      "id_width": 4,
      "plural_label": "RFCs",
      "statuses": ["Draft", "Proposed", "Accepted", "Rejected", "Superseded"],
      "aliases": [],
      "enabled": true
    }
  ],
  "documents": [
    {
      "type": "rfc",
      "id": "RFC-0001",
      "title": "Example",
      "status": "Accepted",
      "author": "Donald Gifford",
      "created": "2026-01-01",
      "path": "docs/rfc/0001-example.md"
    }
  ]
}
```

`path` is repo-root-relative (`<docs_dir>/<dir>/<filename>`). `documents` is
sorted by `(type, id)` for golden stability. `schema_version` is an explicit
integer so a future field addition does not silently break a non-Go consumer.
Raw markdown is intentionally omitted from the default export (it bloats the
manifest and the Go library already exposes `DocEntry.Content`); a `--content`
flag could include it if a consumer needs it (open question 6).

## Testing Strategy

- **Existing tests move with the code, unchanged.** `internal/config/*_test.go`
  and `internal/document/*_test.go` (table-driven, `t.Parallel()`, golden
  fixtures under `internal/document/testdata/golden/status/`) move to
  `pkg/doczcore/config` and `pkg/doczcore/document`. The move is import-path-only;
  assertions and golden bytes are unchanged, proving behavior is preserved.
  Golden files are regenerated only to confirm *no diff* (`go test ./...
  -update` must produce zero churn).
- **CLI regression suite stays green.** `cmd/` tests (serial, `Runner` +
  `bytes.Buffer`, `RepoRoot: t.TempDir()`) must pass untouched except for
  import paths, proving the CLI is behavior-identical after the move.
- **New consumer / import smoke test.** Add a tiny test module (a separate
  `go.mod` under `test/consumer/` or an in-repo peer test that exercises the
  package as an external caller would) that:
  - imports `pkg/doczcore/config` and `pkg/doczcore/document` *by their public
    paths*,
  - writes a `.docz.yaml` + a couple of docs into a `t.TempDir()`,
  - runs `Load → Validate → EnabledTypes → ScanDocuments` and asserts the
    `DocEntry` set, including a **custom type** (to lock in the type-agnostic
    contract from INV-0005 Obs 1), and
  - calls `ParseFrontmatter` on raw bytes (the no-checkout path) and asserts
    the parsed `Frontmatter`.
  This is the test that would have caught a regression where promotion
  accidentally narrowed visibility or broke the `document → config` typed-field
  link.
- **Golden stability for `docz export` (only if shipped).** A golden JSON
  fixture for a fixed repo + `.docz.yaml`; regenerated with `-update`, never
  hand-edited; asserts deterministic ordering (`documents` sorted by
  `(type, id)`) so the manifest is diff-stable across runs.
- **`make ci` unchanged** — lint (`golangci-lint` + `golines`),
  license-check, build, and the full test suite gate the move.

## Migration / Rollout Plan

Behavior-preserving, single-repo, mechanical. Sequencing:

1. **Move the packages.** `git mv internal/config → pkg/doczcore/config` and
   `internal/document → pkg/doczcore/document`, preserving package names
   (`config`, `document`) so only on-disk location and import paths change.
2. **Update internal imports.** Repoint `cmd/` and `internal/template` /
   `index` / `toc` / `wiki` to the new `pkg/doczcore/...` paths
   (`goimports` + `go build ./...`). The `document → config` import repoints
   too. This is compiler-verified — the build fails loudly until every path is
   fixed.
3. **Run the suite.** `make test` must be green with zero golden churn,
   confirming behavior is identical. Add the consumer import smoke test.
4. **Keep CLI behavior identical.** No command, flag, output, or config change
   in this step. (`docz export` is a *separate, optional, later* increment — do
   not bundle it into the move; see open question 3.)
5. **Tag a minor release.** Cut `vX.Y.0` (or `v1.0.0` if the decision is to
   declare the public API stable — open question 4) so docz-api has a concrete
   tag to `require`. Note in release notes that `pkg/doczcore` is now a
   supported, semver-governed public surface and that `internal/config` /
   `internal/document` import paths are gone (invisible to anyone outside the
   repo).
6. **(Optional, deferred) ship `docz export --json`** as its own minor release
   if/when a non-Go consumer needs it, built strictly on the promoted library.

**Sequencing relative to building docz-api:** this design is a prerequisite for
docz-api's ingestion code, but not a blocker for *starting* docz-api. Per
INV-0005 Decision 8 (thin vertical slice), docz-api can begin against a
hand-pinned local `replace` of docz during prototyping; cut the real tag (step
5) before docz-api's first non-prototype release so it pins a published version.
The move itself is low-risk and can land immediately — it ships value to the CLI
repo (a cleaner public surface) independent of docz-api's timeline.

**Rollback:** the move is a pure code relocation behind a single release tag; if
a problem surfaces, reverting the PR restores the prior `internal/` layout with
no data or config migration to undo.

## Open Questions

### 1. What is the public package name/path and shape?

- **a. (Recommended)** `pkg/doczcore/config` + `pkg/doczcore/document` — preserve
  the existing two-package split under a `doczcore` namespace, moving each
  wholesale with minimal in-file churn and keeping the typed `Status`/`DocType`
  boundary intact.
- b. A single flat `pkg/doczcore` merging both — smaller surface, but forces
  renaming every `config.`/`document.` qualifier across moved + CLI code.
- c. Promote in place as `pkg/config` + `pkg/document` (no `doczcore` grouping)
  — shortest paths, but less obvious that they form one "core."
- d. Other.

### 2. Move wholesale (update all imports) vs. leave permanent re-export shims?

- **a. (Recommended)** Move wholesale and update every in-repo import; no shim
  survives the release. One canonical path, no rot.
- b. Move + permanent type-alias shims at `internal/config` /
   `internal/document` so existing imports never change.
- c. Move + *transitional* shim inside one PR, deleted before release, only if
  the import sweep is too large to land atomically.
- d. Other.

### 3. Add `docz export --json` now, or defer it?

- **a. (Recommended)** Defer. Decision 7 chose the library as primary; ship the
  promotion first and add `export` only when a concrete non-Go consumer appears.
- b. Ship `docz export --json` in the same release as the promotion, as a
  convenience + debugging aid, accepting its JSON shape as a new golden surface.
- c. Never add it — the library plus the existing `docz list --format json`
  cover the need.
- d. Other.

### 4. Module/versioning strategy?

- **a. (Recommended)** Same module, new `pkg/doczcore` path; tag a normal minor
  `vX.Y.0` and let docz-api pin it. Stay in the current major; cut `v1.0.0` only
  when the broader CLI is ready, not solely because of this surface.
- b. Cut `v1.0.0` now precisely *because* promotion creates a public API that
  deserves an explicit stability guarantee.
- c. Split the core into a *separate* module
  (`github.com/donaldgifford/docz-core`) with its own version line, decoupling
  library semver from CLI releases.
- d. Other.

### 5. How much surface to expose?

- **a. (Recommended)** Minimal: config (`Load`/`Validate`/resolution/
  `EnabledTypes`/`TypeDir`) + document (`ScanDocuments`/`LoadFrontmatter`/
  `ParseFrontmatter`/`IsDoczFile`) + the structs and sentinels. Smallest semver
  obligation.
- b. Broader: also promote `SetStatus` and its sentinels so a future
  write-capable consumer can mutate status through the shared library.
- c. Broadest: also promote template/index/toc/wiki so a renderer-less consumer
  could reproduce docz output too (large, ongoing semver cost).
- d. Other.

### 6. If `docz export` ships, what is the manifest schema shape?

- **a. (Recommended)** One top-level object `{schema_version, docs_dir, types[],
  documents[]}`, `documents` sorted by `(type, id)`, raw markdown omitted by
  default (opt-in via `--content`).
- b. Two separate top-level arrays / NDJSON streaming for very large repos.
- c. Mirror docz-api's eventual Postgres row shape exactly so ingest is a direct
  load.
- d. Other.

### 7. Does docz-api vendor a docz checkout or always consume the library via go.mod?

- **a. (Recommended)** Always consume `pkg/doczcore` as a normal `go.mod`
  dependency pinned to a tag; no vendored source, no shelling out.
- b. Additionally `go mod vendor` docz for hermetic, network-free builds, still
  via the public import path.
- c. Vendor a full docz checkout and shell out to the binary (INV-0005 option 3)
  — rejected by Decision 7, listed only for completeness.
- d. Other.

### 8. Where does the consumer import smoke test live?

- **a. (Recommended)** A separate minimal module under `test/consumer/` with its
  own `go.mod`, importing the published-style path — the truest proof an
  external module can import and scan.
- b. An in-repo peer-package test exercising the public API — simpler CI wiring,
  slightly weaker "external" guarantee.
- c. Both — in-repo test for fast feedback, plus a periodic external-module CI
  job.
- d. Other.

## References

- **INV-0005** — *docz-api and docz-site: centralized cross-repo docz registry
  and viewer* — the source investigation; Decision 7 (shared `pkg/…` library)
  drives this design, and Observation 4 enumerates the library-vs-export-vs-shell
  options.
- **DESIGN-0008** — docz-api (the companion service: GitHub App, ingestion,
  Postgres registry, webhook refresh, Meilisearch). Consumes `pkg/doczcore`;
  not in scope here.
- **DESIGN-0009** — docz-site (the viewer: repos → types → docs nav, search,
  client-side markdown render per INV-0005 Decision 3). Not in scope here.
- **DESIGN-0006** — *Custom Document Type Support* — establishes that the type
  set is config-driven and type-agnostic; the promoted `EnabledTypes` /
  `resolveType` surface must preserve that for the registry.
- **Code being promoted:**
  - `internal/config` — `Config`, `TypeConfig`, `DocType`, `Status`,
    `DocTypeDef`, `Load`, `Validate`, `DefaultConfig`, `EnabledTypes`,
    `TypeDir`, `ValidateType`, `resolveType`, `DocTypeNames`, `AllDocTypes`,
    `LookupDocType`, `ResolveTypeAlias`, `ErrUnknownType`
    (`config.go`, `doctype.go`, `constants.go`).
  - `internal/document` — `Frontmatter`, `DocEntry`, `ScanDocuments`,
    `LoadFrontmatter`, `ParseFrontmatter`, `IsDoczFile`, `SetStatus` (candidate
    to stay internal), and the sentinels `ErrNoFrontmatter`,
    `ErrStatusFieldMissing`, `ErrUnsupportedLineEndings`
    (`document.go`, `scan.go`, `status.go`).
