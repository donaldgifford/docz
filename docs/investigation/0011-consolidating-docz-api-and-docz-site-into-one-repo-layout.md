---
id: INV-0011
title: "Consolidating docz-api and docz-site into one repo: layout, module topology, and the v2 upgrade"
status: Concluded
author: Donald Gifford
created: 2026-09-21
---

<!-- markdownlint-disable-file MD025 MD041 -->

# INV-0011: Consolidating docz-api and docz-site into one repo: layout, module topology, and the v2 upgrade

<!--toc:start-->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
  - [Observation 1: docz-api's entire docz surface is 19 symbols from three frozen packages](#observation-1-docz-apis-entire-docz-surface-is-19-symbols-from-three-frozen-packages)
  - [Observation 2: the one real API gap is that config.Load cannot take bytes](#observation-2-the-one-real-api-gap-is-that-configload-cannot-take-bytes)
  - [Observation 3: ADR-0003 is a non-event for docz-api, and the contract pins more than the pipeline uses](#observation-3-adr-0003-is-a-non-event-for-docz-api-and-the-contract-pins-more-than-the-pipeline-uses)
  - [Observation 4: the OpenAPI spec is duplicated byte-for-byte across the two repos, by hand](#observation-4-the-openapi-spec-is-duplicated-byte-for-byte-across-the-two-repos-by-hand)
  - [Observation 5: the dependency asymmetry is the real argument about module topology](#observation-5-the-dependency-asymmetry-is-the-real-argument-about-module-topology)
  - [Observation 6: docz-site is not a Go directory, and site/ is not a Go idiom](#observation-6-docz-site-is-not-a-go-directory-and-site-is-not-a-go-idiom)
  - [Observation 7: the restructure has a deadline, not a backlog position](#observation-7-the-restructure-has-a-deadline-not-a-backlog-position)
  - [Observation 8: consolidation forces three tooling decisions that are otherwise invisible](#observation-8-consolidation-forces-three-tooling-decisions-that-are-otherwise-invisible)
  - [Observation 9: the contract tests change meaning, and that is worth preserving deliberately](#observation-9-the-contract-tests-change-meaning-and-that-is-worth-preserving-deliberately)
  - [Observation 10: under cmd/, the directory name is the binary name, and three things already rely on it](#observation-10-under-cmd-the-directory-name-is-the-binary-name-and-three-things-already-rely-on-it)
  - [Observation 11: all 43 incoming docz documents collide, and the root files collide too](#observation-11-all-43-incoming-docz-documents-collide-and-the-root-files-collide-too)
- [Open Questions](#open-questions)
  - [1. What is the directory layout?](#1-what-is-the-directory-layout)
  - [2. One module or several?](#2-one-module-or-several)
  - [3. Does wiki move under core/?](#3-does-wiki-move-under-core)
  - [4. How is the frozen five's path change recorded?](#4-how-is-the-frozen-fives-path-change-recorded)
  - [5. Does the v2 line stay on main, or move to a v2 branch?](#5-does-the-v2-line-stay-on-main-or-move-to-a-v2-branch)
  - [6. Which toolchain and which task runner?](#6-which-toolchain-and-which-task-runner)
  - [7. In what order do the restructure and the two moves land?](#7-in-what-order-do-the-restructure-and-the-two-moves-land)
  - [8. Does config gain a bytes API in this line?](#8-does-config-gain-a-bytes-api-in-this-line)
  - [9. What happens to DESIGN-0008 and DESIGN-0009?](#9-what-happens-to-design-0008-and-design-0009)
  - [10. Does the git history come with them?](#10-does-the-git-history-come-with-them)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
- [References](#references)
<!--toc:end-->

<!--docz:question:start-->
## Question

Three questions, in the order their answers constrain each other.

1. **Layout.** Should the tree be restructured — `core/` holding the library,
   `cli/` the command tree, `api/` the service, and a fourth directory for the
   UI — and is that restructure separable from the moves themselves?
2. **Module topology.** One Go module for everything, or several in one
   repository? docz's library is imported by other people's code; docz-api is a
   service with a service's dependency tree.
3. **The upgrade.** What does moving each consumer onto the v2 API actually
   require, beyond the `/v2` import path?

And one question that is independent of all three: does the v2 line keep living
on `main` with hand-cut `v2.0.0-beta.N` tags, or on a long-lived `v2` branch
that becomes `main` at the cutover?

<!--docz:question:end-->

<!--docz:hypothesis:start-->
## Hypothesis

That the two halves of this work have been mentally filed under one heading
while having opposite cost profiles.

The **upgrade** should be cheap: docz-api was built against the promoted
surface on purpose (DESIGN-0007, DESIGN-0008's R1–R12), and that surface is the
five frozen packages, which the v2 line carries forward unchanged. If that
holds, "upgrading docz-api to v2" is a path rewrite rather than a port.

The **layout** should be the expensive, irreversible part — not because moving
directories is hard, but because every public import path changes, and the
eleven experimental packages freeze at v2.0.0. If that holds, the restructure
has a deadline rather than a backlog position, and it should go first, while
the only things importing those paths are `cmd/` and `test/consumer`.

<!--docz:hypothesis:end-->

<!--docz:context:start-->
## Context

ADR-0002 Decision 6 committed to the consolidation and deliberately did not
specify it: "docz-api moves into this repo and builds from `cmd/docz-api`, then
the UI follows, so one chart ships the API and the UI together. **Each of those
is its own design, sequenced after the CLI, and this ADR does not specify
them.**" IMPL-0018 lists the move under Out of Scope, and that unit is now
Completed — `v2.0.0-beta.1` is released, so the CLI it was sequenced after is
done.

What exists today describes the services as *separate repositories consuming
docz v1*: DESIGN-0008 (Approved) is docz-api as a cross-repo ingestion service
with contract clauses R1–R12 against a pinned library, and DESIGN-0009 (Draft)
is docz-site as a separate viewer. Neither describes the move, and neither
describes a v2 upgrade. There is no design, ADR, or IMPL for either half.

The restructure question arrived with the consolidation question and is treated
here as part of it, because the answer to "where does `api/` go" is not
separable from "where does the library go".

**Triggered by:** [ADR-0002](../adr/0002-docz-is-an-api-package-whose-first-consumer-is-the-cli.md)
Decision 6, after [IMPL-0018](../impl/0018-v200-beta1-the-docz-api-as-one-unit-structured-regions-and-the.md)
shipped `v2.0.0-beta.1`.

<!--docz:context:end-->

<!--docz:approach:start-->
## Approach

Both repositories are checked out locally, so this is an inventory rather than
a reading of their design docs.

1. Enumerate every docz symbol docz-api's **production** code uses, separately
   from what its tests use, resolving the import aliases first — docz-api has
   its own `internal/config`, so counting by package name alone conflates the
   two.
2. Read each call site that touches the filesystem, since the no-checkout path
   is where DESIGN-0008 R3 predicted friction.
3. Check docz-api for `plan` references, to size ADR-0003's impact on it.
4. Establish docz-site's stack and its coupling to docz-api, to find out
   whether it is a Go directory at all.
5. Compare the three dependency graphs and toolchain directives.
6. Check what ADR-0001's freeze and ADR-0002's beta window permit, to find out
   whether the restructure is legal now and whether it stays legal.

<!--docz:approach:end-->

<!--docz:environment:start-->
## Environment

| Component | Version / Value |
| --------- | --------------- |
| docz | `v2.0.0-beta.1` (`e96dc7c`), module `github.com/donaldgifford/docz/v2`, `go 1.26.4` |
| docz direct dependencies | 2 (`spf13/cobra`, `go.yaml.in/yaml/v3`) |
| docz-api | `~/code/docz-api`, module `github.com/donaldgifford/docz-api`, `go 1.26.5`, pins `github.com/donaldgifford/docz v1.2.2` |
| docz-api size | 106 Go files, 17 882 lines, 17 packages, 120 requirements in `go.mod` |
| docz-site | `~/code/docz-site`, TypeScript + React, Bun, no `go.mod` |
| docz-site size | 152 `.ts`/`.tsx` source files, plus a generated API client |
| Shared artifacts | `api/openapi.yaml` (916 lines) in both; one Helm chart, `Dockerfile`, and `ct.yaml` in each |
| Build tooling | `make` in docz; `just` in docz-api and docz-site |

<!--docz:environment:end-->

<!--docz:findings:start-->
## Findings

### Observation 1: docz-api's entire docz surface is 19 symbols from three frozen packages

Resolving the aliases (`doczcfg`, `doczdoc`, `doczparse`) and counting
production code separately from tests:

| Package | Symbols used in production |
| ------- | -------------------------- |
| `pkg/doczcore/config` | `Config`, `Load`, `APIConfig`, `DefaultConfig`, `TypeConfig`, `DefaultChangelogFile`, `APILandingFileName`, `WikiIndexName`, `TemplatesDir`, `ErrUnknownType`, `ErrInvalidAPIPath` |
| `pkg/doczcore/document` | `ParseFrontmatter`, `Frontmatter`, `IsDoczFile`, `ParseChangelog`, `ErrNoFrontmatter`, `ErrNoVersions`, `ScanDocuments` |
| `pkg/doczcore/docparse` | `Title` |

Eleven files import docz, five of them production: `internal/ingest/{parse,pages,mapper,service}.go`
and `internal/githubapp/client.go`.

**All three packages are among the five frozen at v1.0.0**, and the v2 line
carries their v1 shapes forward unchanged — the single breaking change to them
is `plan` leaving the catalogue (ADR-0003). Nothing docz-api consumes comes from
the eleven experimental packages, from `repo`, or from any type package.

So the v2 upgrade for docz-api is: rewrite the import path to `/v2`, and
nothing else that this inventory can find. The whole of Phases 1–3 —
`kinds`, `validate`, `repo`, five type packages — is *available* to it and
unused by it.

### Observation 2: the one real API gap is that `config.Load` cannot take bytes

docz-api fetches `.docz.yaml` through the GitHub API and never checks a
repository out. `config.Load` is filesystem-based, so `internal/ingest/parse.go`
writes the fetched bytes to a temp dir on **every ingest** and loads from there.
Its own comment documents the consequence:

> `doczcfg.Load` is filesystem-based (it needs a repo root on disk), so the
> bytes are written to a private temp dir and loaded from there; the dir is
> removed before returning. Doc blobs never touch disk — they are parsed
> byte-wise via `doczdoc.ParseFrontmatter`.
>
> `doczcfg.Load` merges `$HOME/.docz.yaml` when present. In the container there
> is no such file, so nothing is merged; tests neutralize HOME with
> `t.Setenv("HOME", t.TempDir())` to stay hermetic against a developer's
> config.

Two costs: a temp directory per ingest, and a global-config merge that has to
be neutralised in tests so a developer's `~/.docz.yaml` cannot leak into
assertions. This is the same shape as the gap Phase 1 closed for the mutators
with `SetStatusBytes` and `SetTaskStateBytes`, and as `ParseFrontmatter`
already is for documents — `config` is the one package still missing its byte
core.

Adding one is **additive**, so it does not break the freeze (ADR-0001 Open
Question 7, roll-forward semver). It is the only API change this investigation
found a consumer actually needing.

### Observation 3: ADR-0003 is a non-event for docz-api, and the contract pins more than the pipeline uses

`grep` for `"plan"` and `PLAN-` across docz-api's Go and OpenAPI sources returns
nothing. The type catalogue reaches it only through `config`, which reads
whatever a repository's `.docz.yaml` declares, so a repository that keeps its
`types.plan` block ingests as a custom type with no code change.

Separately, `ScanDocuments` appears **only** in `internal/doczcontract`'s tests
and in no production path — the ingest pipeline is byte-wise throughout. The
contract package guards a slightly larger surface than the service actually
depends on, which is cheap and harmless, but worth knowing before treating the
contract list as the requirements list.

### Observation 4: the OpenAPI spec is duplicated byte-for-byte across the two repos, by hand

`api/openapi.yaml` is 916 lines in docz-api and 916 identical lines in
docz-site. docz-site's `orval.config.ts` generates its TypeScript client from
**its own copy**:

```text
input:  ./api/openapi.yaml
output: ./src/api/__generated__/docz-api.ts
```

There is no sync step — no fetch from the API repo, no submodule, no
`gen-api` input pointing across repositories. The only thing keeping the client
honest is that someone copies the file. docz-api additionally has `spec.go` and
a `vacuum-ruleset.yaml` linting its copy, so the spec is *validated* in one repo
and *consumed* in the other.

This is the strongest concrete argument for consolidation found anywhere in this
investigation, and it is independent of docz v2: one repository makes the spec a
single file that the server serves, the linter checks, and the client generator
reads.

### Observation 5: the dependency asymmetry is the real argument about module topology

| Module | Requirements in `go.mod` | Nature |
| ------ | ------------------------ | ------ |
| docz | **2** direct (`cobra`, `yaml`) + 4 indirect | a library other people import |
| docz-api | **120** | a service: HTTP, GitHub App, store + migrations, queue, search, session, auth, OpenTelemetry |

One module makes those 120 requirements the library's requirements. Module graph
pruning means a consumer importing `core/config` still *builds* only what it
imports, but the library's `go.mod` becomes the service's, and — the part that
does not prune away — **the library's version numbers become coupled to the
service's dependency churn**: a Renovate bump to a database driver produces a
new version of the package sdk-booty-sh and tempy import.

It also sits badly with DESIGN-0014 §7 R8, which says the library is
*traceable, not tracing*, and imports no OTel and no logger. docz-api imports
seven OTel modules. In one module that rule survives as a convention enforced by
`layer_test.go`; in separate modules it survives as a fact about the graph.

### Observation 6: docz-site is not a Go directory, and `site/` is not a Go idiom

docz-site has no `go.mod` and no Go files: it is React + TypeScript on Bun,
with Vite, ESLint, Playwright-style e2e, MSW mocks, OpenTelemetry, Shiki, and
Mermaid. Whatever directory it lands in is a directory Go tooling ignores.

On naming, there is no official Go idiom, and the widely-copied
`golang-standards/project-layout` is not an official standard. What real Go
projects that bundle a frontend actually do:

| Project | Directory |
| ------- | --------- |
| Prometheus | `web/ui/` |
| HashiCorp Vault | `ui/` |
| HashiCorp Consul | `ui/` |
| Argo CD | `ui/` |

`website/` is used by HashiCorp repositories for the **documentation** site,
which is the opposite of what is meant here and would collide with docz's own
`docs/` tree and its MkDocs wiki. `site/` reads the same way.

`ui/` is what the majority precedent uses and is unambiguous: it is the
application's frontend, not the project's marketing pages. `web/ui/` is the
Prometheus spelling and is equally defensible if a `web/` directory is wanted
for server-side web assets too.

One practical note for whichever name wins: `node_modules/` inside the module
tree slows `go` tooling that walks directories, and `//go:embed` cannot embed a
path containing a symlink, so the embed should point at the built `dist/` only.

### Observation 7: the restructure has a deadline, not a backlog position

Every import path in the table below changes if the tree is restructured:
sixteen packages, five of them frozen-shape.

Today the only importers are `cmd/` and `test/consumer`, both in-repo, plus
whatever a reader has pinned to a `v2.0.0-beta.N` tag — and the beta window
exists precisely so that can change (ADR-0002 Decision 7: the packages added
under that ADR "may change between betas, and freeze at v2.0.0"). External v1
consumers are on the unversioned path and cannot see any of it.

After v2.0.0 ships, moving `core/config` is a v3. Before it, the cost is one
mechanical rename plus a re-capture of nothing — the parity goldens record CLI
behaviour, not import paths.

The five frozen packages need one explicit decision, not just a rename: ADR-0001
Decision 6 froze their **shapes**, and the v2 line carries those forward. A path
change is not a shape change, but it is breaking to a v2 consumer, so it wants a
dated amendment saying so rather than arriving as a side effect of a directory
move.

### Observation 8: consolidation forces three tooling decisions that are otherwise invisible

| Thing | docz | docz-api | docz-site |
| ----- | ---- | -------- | --------- |
| `go` directive | 1.26.4 | 1.26.5 | — |
| Task runner | `make` | `just` | `just` |
| Container | — | `Dockerfile`, `compose.yaml`, `docker-bake.hcl` | `Dockerfile` |
| Chart | — | `charts/docz-api` + `ct.yaml` | `charts/docz-site` + `ct.yaml` |
| Changelog | none | `cliff.toml` | `cliff.toml` |

The `go` directive is the interesting one: docz sits at 1.26.4 with
**GO-2026-4970** open against it, while docz-api is already at 1.26.5, which is
the fix. A single module forces the bump and closes that finding as a side
effect. Two modules mean it remains a decision.

`make` versus `just` is a real chore rather than a preference: `make ci` is
wired into this repo's CI job, its parity target, and its release path, and both
other repos have `justfile`s with their own container and chart recipes.

### Observation 9: the contract tests change meaning, and that is worth preserving deliberately

`internal/doczcontract` contains no runtime code. Its entire purpose is that a
docz bump fails *there* — cheaply and unambiguously — rather than deep inside
`internal/ingest`. It guards R1–R5, R6 (changelog), R10 (`api:` block and
`docparse.Title`), and R11 (json tags mirroring yaml).

In one repository there is no pin, so the drift it guards against cannot happen
between releases: a library change and its consumer land in the same commit.
The early-warning property is still worth keeping — it is the difference between
"the contract changed" and "an ingest test failed" — but it becomes a
convention rather than a mechanism, and the R-clause numbering in DESIGN-0008
loses its anchor.

### Observation 10: under `cmd/`, the directory name is the binary name, and three things already rely on it

Go names an installed binary after the last element of its package path, so
`cmd/cli/main.go` produces a binary called `cli` and `cmd/server/main.go` one
called `server`. Three things in this repository already depend on that
element being `docz`:

| Thing | How it depends |
| ----- | -------------- |
| `.goreleaser.yml` | `main: ./cmd/docz/main.go` |
| `Makefile` | `PARITY_PKG := $(GO_PACKAGE)/cmd/$(PROJECT_NAME)`, used by `make parity-capture` to install the v1.2.2 binary |
| `v2.0.0-beta.1` release notes | publish `go install github.com/donaldgifford/docz/v2/cmd/docz@v2.0.0-beta.1` |

goreleaser can rename its output with `builds[].binary`, so a release artefact
is not the problem. `go install` is: it has no such override, so a descriptive
directory name is a name the user ends up typing and living with.

### Observation 11: all 43 incoming docz documents collide, and the root files collide too

Added 2026-09-21 while drafting ADR-0004. The inventory above counted Go
symbols and skipped the thing both repositories have most of.

| Repository | DESIGN | IMPL | INV | Total |
| ---------- | ------ | ---- | --- | ----- |
| docz | 0001–0015 | 0001–0018 | 0001–0011 | 44 + 4 ADRs |
| docz-api | 0001–0005 | 0001–0010 | 0001–0009 | 24 |
| docz-site | 0001–0006 | 0001–0007 | 0001–0006 | 19 |

**The intersection is total: not one incoming ID is free.** Each of those 43
documents cross-references others by ID, and docz-api's Go comments cite its
own IMPL numbers, so renumbering is not a file rename — it is a rewrite of
every reference in and to the tree. This is the one piece of the move with no
cheap answer, and it is ADR-0004 Open Question 1.

The root files collide the same way and were equally invisible from a symbol
inventory: two `Dockerfile`s, two `ct.yaml`s, two `cliff.toml`s, two
`mise.toml`s, two `renovate.json5`s, two `catalog-info.yaml`s, two `CLAUDE.md`s,
two `README.md`s, two `deploy/` trees, two `scripts/` directories, plus
docz-api's `compose.yaml`, `docker-bake.hcl`, and `sqlc.yaml`. docz-site also
carries a `server/` directory — a TypeScript serving layer, not Go — which
lands inside `ui/`. ADR-0004 Open Question 2.

One fact checked rather than assumed, because it would have been the expensive
kind of surprise: **nothing sensitive is in either tracked history.** docz-api's
`deploy/secrets/github-app.pem` and `deploy/.env.local` exist on disk but are
matched by `.gitignore` lines 27 and 22 and appear in no commit on any branch,
and all three repositories are public already. The `.gitignore`s have to merge
with the trees, though: without those two lines, the first `git add -A` in a
developer's checkout commits a private key.

<!--docz:findings:end-->

<!--docz:open-questions:start-->
## Open Questions

### 1. What is the directory layout?

- a. **`core/`, `cli/`, `api/`, `ui/`, with `cmd/` holding only `main`
  packages** — `core/{config,document,docparse,docwrite,toc,doctemplate,index,
  kinds,validate,repo}` plus `core/{rfc,adr,design,impl,investigation}`,
  `cli/` for the Cobra tree that is `cmd/*.go` today, `api/` for the service,
  `ui/` for the frontend, and `cmd/docz/main.go` + `cmd/docz-api/main.go` as
  the two entry points. Collapses the `pkg/doczcore` double element — `core/config`
  rather than `pkg/doczcore/config` — and leaves `cmd/` meaning exactly what it
  means in every other Go repository: binaries, nothing else. *(recommendation)*
- b. Keep `pkg/` as it is and add `api/` and `ui/` beside it. No import path
  churn at all; the price is that the tree never says which packages are the
  library, and `pkg/doczcore/` keeps a redundant element.
- c. `core/`, `cli/`, `api/`, `ui/` as in (a), but with the service under
  `internal/api/` so nothing outside the repository can import it. Honest about
  the service not being a public surface, at the cost of making `api/`'s own
  packages untestable from an external module.
- d. Other.

> **Resolved 2026-09-21: (b), extended.** `pkg/` stays exactly where it is and
> the tree grows around it:
>
> ```text
> cmd/docz/main.go       the CLI binary
> cmd/docz-api/main.go   the server binary
> pkg/                   the library, unchanged
> internal/              the server's internals (docz-api's internal/ verbatim)
> api/                   openapi.yaml, the single copy
> ui/                    the Bun/React app
> charts/                docz-api and docz-site, side by side
> ```
>
> `pkg/` exists because the library and its consumers were in different
> repositories. That stops being the reason it is there, but renaming it buys
> nothing and costs every import path. **This is the resolution that matters
> most, because it deletes Observation 7's deadline:** no public path changes,
> so there is no ADR-0001 amendment to write, nothing that `v2.0.0-beta.1`
> published becomes a lie, and no risk of the move turning into a v3. The
> restructure stops existing as a distinct piece of work, which also answers
> Open Question 7.
>
> `cmd/docz` and `cmd/docz-api` rather than `cmd/cli` and `cmd/server`, for
> Observation 10's reason: under `cmd/`, the directory name *is* the binary
> name.
>
> `internal/` takes the **server's** internals, not the library's. The library
> stays public — that is ADR-0002's thesis and what the published beta
> advertises — so nothing moves out of `pkg/`.
>
> `charts/` takes both charts side by side rather than one umbrella chart. The
> pain being solved is that one logical change needs three pull requests in
> three repositories; two charts in one repository does not have that problem.
> An umbrella chart stays possible later and is not what ADR-0002 Decision 6's
> "one chart" wording forecloses on.

### 2. One module or several?

- a. **Two modules: the root (library + CLI) and `api/`** — `ui/` has no
  `go.mod` at all. The library keeps its two dependencies and its own version
  line; the service's 120 requirements and its OTel imports stay out of the
  graph that sdk-booty-sh and tempy resolve. A `go.work` covers local
  development so a change to `core/` is visible to `api/` without a `replace`.
  The cost is real: two version lines, submodule tags (`api/v0.1.0`), and a
  release pipeline that has to know which module it is cutting. *(recommendation)*
- b. One module for everything. Simplest release story — one tag, one chart,
  one `go.mod` — and the CI already works that way. Pays Observation 5's price:
  the library's versions move when the service's dependencies do.
- c. Three modules (root library, `cli/`, `api/`). Maximal hygiene; the CLI
  gains a version line nobody asked for.
- d. Other.

> **Resolved 2026-09-21: (b) — one module.** The dependency coupling in
> Observation 5 is accepted knowingly, against a cost that is being paid today
> rather than theorised: a change to docz needs a pull request here, then one in
> docz-api, then one in docz-site, and the spec in Observation 4 is copied by
> hand between two of them. Multi-module fixes the graph and leaves the
> three-repository workflow intact for `go.work` to paper over.
>
> Consequences to carry into the design rather than discover later:
>
> - The module's `go.mod` becomes the union, so `go get
>   github.com/donaldgifford/docz/v2` resolves a graph with roughly 120
>   requirements instead of 2. Pruning means consumers still *build* only what
>   they import, but the library's version line now moves when a server
>   dependency does.
> - `make license-check` (whatever it is called after Open Question 6) starts
>   scanning the service's dependencies, which is more work per run and more
>   licences to accept.
> - DESIGN-0014 §7 R8 — the library imports no OpenTelemetry and no logger —
>   survives as a rule enforced by `pkg/doczcore/layer_test.go` rather than as a
>   fact about the module graph. That test becomes load-bearing, and should
>   grow a case naming the server's packages explicitly.
> - One `go` directive for everything, which forces Open Question 6's toolchain
>   half.

### 3. Does `wiki` move under `core/`?

- a. **`core/wiki`** — inside the library, since that is what it is: a
  package the library ships and the CLI consumes. *(recommendation)*
- b. Top-level `wiki/`, preserving DESIGN-0014 §2.10's deliberate placement of
  it as "a sibling of the type packages … an integration, not core and not a
  document type". Keeps the distinction the design argued for, at the cost of a
  top-level directory holding one package.
- c. Other.

> **Resolved 2026-09-21: moot — (b) by consequence.** Open Question 1 keeps
> `pkg/`, so there is no `core/` for `wiki` to move into and
> `pkg/wiki` stays where DESIGN-0014 §2.10 put it. The question only existed
> because of the `core/` proposal.

### 4. How is the frozen five's path change recorded?

- a. **A dated amendment to ADR-0001 Decision 6**, stating that the freeze binds
  shapes and that the v2 restructure changes paths once, before v2.0.0, with the
  shapes carried forward unchanged. Matches how DESIGN-0007's read-only stance
  was amended rather than superseded. *(recommendation)*
- b. A new ADR for the layout as a whole, which subsumes the amendment.
- c. Nothing: the beta window already permits it and `/v2` is already a new
  path. Cheapest, and leaves the one thing a v1 reader might care about
  unwritten.
- d. Other.

> **Resolved 2026-09-21: moot — (c) by consequence.** Open Question 1 changes no
> import paths, so the frozen five keep theirs and there is nothing to amend.
> If a later change does move them, this question comes back unanswered.

### 5. Does the v2 line stay on `main`, or move to a `v2` branch?

- a. **Stay on `main`, cut `v2.0.0-beta.N` per completed milestone** — this is
  what is already true and already decided: `main` carries the `/v2` module
  path, every PR is `dont-release`, and a `v1` maintenance branch is cut from
  `v1.2.2` only on demand. A long-lived `v2` branch would mean reverting `main`
  to v1 and maintaining a months-long divergence for no reader. Milestone betas
  (restructure, api, ui) keep proving the proxy path and give each milestone a
  citable artifact. *(recommendation)*
- b. Cut a `v2` branch, return `main` to v1.2.2, and merge at the cutover.
  Makes `main` match what is published as Latest; costs a continuous merge
  burden and contradicts ADR-0002 Decision 6 and IMPL-0018 as shipped.
- c. Stay on `main` but stop cutting betas until a single `v2.0.0-rc.1` at the
  end. Less tag noise; loses the per-milestone proof and leaves a long stretch
  with nothing installable.
- d. Other.

> **Resolved 2026-09-21: (a) — confirmed, nothing to do.** `main` is the v2
> line and stays it. Each of the three units below gets its own
> `v2.0.0-beta.N`, and v2.0.0 proper is cut after the UI lands, which is also
> when the eleven experimental packages freeze. Recorded as ADR-0004
> Decision 5.

### 6. Which toolchain and which task runner?

- a. **`go 1.26.5` and `make`** — the bump is required to share a module with
  docz-api under Open Question 2(b) and desirable regardless, since it closes
  GO-2026-4970; `make` because it is already wired into this repository's CI
  job, `make parity`, `make validate`, and the release path, and porting those
  is work with no product in it. The incoming `justfile` recipes become make
  targets. *(recommendation)*
- b. `just`, converting this repository's Makefile. Matches two of the three
  repositories and the recipes that exist for containers and charts.
- c. Both, with `just` delegating to `make`. No conversion work; two files to
  keep honest.
- d. Other.

> **Resolved 2026-09-21: (b) — `just`, and `go 1.26.5`.** The task runner
> becomes `just`, matching the two repositories moving in and letting their
> recipes arrive as files rather than as translations: `justfile` at the root
> importing `docz.just`, `api.just`, and `ui.just`, so each half keeps its own
> recipes and the root file is composition. docz-api already splits
> `docker.just` out this way, so the pattern is imported rather than invented.
>
> The toolchain goes to `go 1.26.5`, which one module forces anyway and which
> closes **GO-2026-4970** as a side effect.
>
> The cost is named here so the design does not treat it as free: `make ci`,
> `make parity`, `make parity-capture`, `make validate`, `make test-consumer`,
> and `make release TAG=` are referenced by the CI workflow, the release path,
> `CLAUDE.md`, `DEVELOPMENT.md`, and `CONTRIBUTING.md`. It is mechanical, it
> touches the harness that proved the v2 line green, and it deserves its own
> pull request landing before either service moves — so that if something
> breaks, what broke is the runner and not the migration.

### 7. In what order do the restructure and the two moves land?

- a. **Restructure first, then `api/`, then `ui/`** — path churn is cheapest
  while the only importers are in-repo, and it is the one step with a deadline
  (Observation 7). Each is its own design and its own beta. *(recommendation)*
- b. Move docz-api first, restructure afterwards: the move is the thing with
  product value, and doing it under the current layout proves the module
  question before committing to directories.
- c. One design and one unit for all three. Fewer documents; a very large diff
  that mixes a mechanical rename with two migrations.
- d. Other.

> **Resolved 2026-09-21: the premise is gone.** Open Question 1 removed the
> restructure, so there is no rename to sequence against. The remaining order
> is:
>
> 1. the `just` migration, on its own, while the tree is still small;
> 2. docz-api into `internal/` + `cmd/docz-api` + `api/` + `charts/docz-api`,
>    with the spec deduplicated;
> 3. docz-site into `ui/` + `charts/docz-site`, with its generated client
>    reading the one remaining spec.
>
> Each is its own design and its own `v2.0.0-beta.N`.

### 8. Does `config` gain a bytes API in this line?

- a. **Yes — `config.ParseBytes` (or `Decode`), additive to a frozen
  package**, removing the temp-directory-per-ingest and the `$HOME` merge that
  docz-api documents and neutralises in tests (Observation 2). It is the only
  API gap a real consumer was found to have, and additive changes are legal
  under roll-forward semver. *(recommendation)*
- b. Not yet: once docz-api is in-repo it can call an unexported helper, and the
  public surface stays as it is.
- c. Yes, but as part of a wider "byte cores everywhere" pass that also revisits
  `ScanDocuments`.
- d. Other.

> **Resolved 2026-09-21: (a) — `config.ParseBytes`, shipping with the docz-api
> move.** The one gap a real consumer was found to have, and additive to a
> frozen package, so the freeze holds. Not (c): `ScanDocuments` appears only in
> `doczcontract`'s tests and in no production path (Observation 3), so a wider
> pass would be designing for a need nothing has demonstrated — the same
> promotion-on-speculation ADR-0002 exists to stop.
>
> Two obligations carried into ADR-0004 Decision 6 rather than left to the
> implementation: it runs the same normalisation `Load` runs, and a test pins
> that `Load` of a single-file repository and `ParseBytes` of its bytes agree.
> Without those the two paths drift and the byte core becomes a second,
> subtly different config loader.

### 9. What happens to DESIGN-0008 and DESIGN-0009?

- a. **Both get dated amendments and stay as the service designs; the move and
  the v2 upgrade are new designs that reference them.** DESIGN-0008 is Approved
  and describes a system that will still exist, only in a different repository;
  its R1–R12 clauses become in-repo test obligations rather than pin guards.
  DESIGN-0009 is still Draft and should be finished or abandoned explicitly
  before the UI moves, rather than being moved in its unfinished state.
  *(recommendation)*
- b. Supersede both with consolidation designs.
- c. Leave both untouched; the new designs stand alone.
- d. Other.

> **Resolved 2026-09-21: (c) — the new designs stand alone.** Consolidation is
> the goal, and an older design that contradicts it is evidence of what was
> true when it was written, not an obstacle. DESIGN-0008 and DESIGN-0009 are
> left as they are; the consolidation designs reference them where useful and
> do not wait on amendments to either.
>
> Two contradictions are therefore accepted rather than resolved: DESIGN-0008
> describes a service consuming a pinned library, which stops being how it
> works, and DESIGN-0009 is Draft and will be migrated in that state. The
> R1–R12 clause numbering keeps its meaning as a list of behaviours the
> `doczcontract` tests assert, and loses its meaning as a contract against a
> pin.

### 10. Does the git history come with them?

- a. **Import with history** (`git subtree add`, or a read-tree merge), so
  `git log` on `api/` still explains why a line exists. Costs a one-off
  merge-commit oddity and a larger object store. *(recommendation)*
- b. Squash-import each as a single commit, with the old repositories archived
  read-only as the historical record.
- c. Other.

> **Resolved 2026-09-21: (a) — import with history.** Merge with
> `--allow-unrelated-histories` after rewriting the incoming paths with
> `git filter-repo`, so the commits themselves carry target paths and blame
> lands on the commit that wrote the line rather than on an import commit.
>
> Cheaper than this question assumed, for a reason that only became visible
> once Open Question 1 settled: **docz-api's tree already is the target
> layout** — `internal/`, `cmd/docz-api/`, `api/`, `charts/docz-api/` — because
> the layout was derived from it. Only its `docs/` and its root files need
> rewriting. docz-site is `--to-subdirectory-filter ui` with `charts/docz-site`
> lifted back out. `git subtree add --prefix=` is *not* the mechanism: it puts
> an entire incoming repository under one prefix, which is not what either
> move wants.
>
> One thing checked rather than assumed: nothing sensitive is in either
> tracked history. docz-api's `deploy/secrets/github-app.pem` and
> `deploy/.env.local` are gitignored and were never committed, and all three
> repositories are public — so merging history exposes nothing. The
> `.gitignore`s must merge with the trees, or those files become tracked by
> accident on the first developer's `git add`.
>
> The old repositories are archived read-only after the merge, as a pointer
> rather than as the record.

<!--docz:open-questions:end-->

<!--docz:conclusion:start-->
## Conclusion

**Answer:** Yes — consolidate, and do the restructure first. The hypothesis
holds in both halves, and the two halves have almost nothing to do with each
other.

The **upgrade is nearly free**. docz-api consumes 19 symbols from three
packages, all three frozen, none of them changed by the v2 line except `plan`
leaving a catalogue docz-api never names. Moving it onto v2 is an import-path
rewrite. The eleven experimental packages, `repo`, and the five type packages —
the entire output of IMPL-0018 Phases 1–3 — are available to it and unused,
which means the v2 API was not, in the end, built for the consumer that
motivated it. That is worth knowing before writing the design: the upgrade's
value is not in adopting the new surface, it is in the one gap Observation 2
found and in deleting the pin.

The **layout was the expensive half, and the decision made it cheap.** As
investigated, a restructure changed sixteen import paths and therefore had a
deadline: before v2.0.0 it is a rename, after it a v3. Open Question 1 resolved
to keep `pkg/` and grow the tree around it, which removes the path change
entirely — so the deadline, the ADR-0001 amendment, and the restructure as a
unit of work all stop existing. That is the single cheapest resolution in this
document, and it was available only because the thing being added (`api/`,
`internal/`, `ui/`, `charts/`) does not collide with the thing already there.

The strongest argument for the move itself turned out not to be docz v2 at all:
a 916-line OpenAPI spec exists byte-identically in both repositories, hand-copied,
with no sync mechanism, validated in one and consumed in the other. That is a
correctness problem today, and one repository dissolves it.

The sharpest argument *against* one big module is the dependency asymmetry: 2
requirements against 120, and a library whose version line starts moving
whenever a service dependency does. **Open Question 2 accepted that knowingly**,
and the reasoning is sound on its own terms: the coupling is a cost that might
bite later, while the three-repository lockstep — one logical change, three pull
requests, plus a spec copied by hand — is a cost being paid now. Multi-module
would have fixed the graph and left that workflow in place for `go.work` to
paper over. The trade is real and recorded; what it buys is that DESIGN-0014 §7
R8 now survives as a test rather than as a fact about the module graph, so
`layer_test.go` becomes load-bearing.

On the branch question there was nothing to decide so much as something to
confirm: `main` already **is** the v2 line, and a `v2` branch would mean
reverting `main` to v1 and maintaining the divergence for no reader. All ten
questions are resolved as of 2026-09-21 and recorded in ADR-0004.

What the investigation got wrong is worth saying plainly. It inventoried Go
symbols, dependency graphs, and toolchains, and from that concluded the layout
was the expensive half and the upgrade nearly free. Both halves held. What it
missed entirely is Observation 11: the two repositories' **43 docz documents**,
every one of whose IDs collides with one of docz's own, and the two dozen
root files that collide the same way. The expensive part of this move is not
Go code and not import paths — it is the documentation and the scaffolding, the
two things an API inventory does not look at.

<!--docz:conclusion:end-->

<!--docz:recommendation:start-->
## Recommendation

All ten open questions were resolved on 2026-09-21, which changes the shape of
what follows: there is no restructure to plan, and no ADR needed for a path
change that is no longer happening. What remains is one decision record and
three units of work.

1. **An ADR for the consolidated repository** — written as
   [ADR-0004](../adr/0004-one-repository-docz-docz-api-and-docz-site-as-a-single-go-module.md),
   recording Open Questions 1, 2, 5, 6, 8, 9, and 10: one module, `pkg/`
   unchanged, `cmd/docz` and `cmd/docz-api`, `internal/` for the server, `api/`
   for the one spec, `ui/` for the frontend, `charts/` for both charts, `just`
   as the task runner, `go 1.26.5`, `config.ParseBytes`, history preserved. It
   is an ADR rather than a design because ADR-0002 Decision 6 left the
   consolidation unspecified and this is what fills that gap. It stayed short
   where this investigation expected length — the expensive layout option was
   declined — and grew in the place this investigation did not look, carrying
   four open questions of its own about the incoming documents, the root files,
   the contract tests, and CI cost.
2. **The `just` migration, on its own**, before either service arrives.
   `justfile` plus `docz.just`, with `api.just` and `ui.just` landing as the
   services do. It touches the harness that proved this line green — `make ci`,
   `make parity`, `make validate`, `make test-consumer`, `make release TAG=`,
   the CI workflow, and three guide documents — so it wants isolating from
   anything that could be blamed for its breakage.
3. **docz-api in**, as its own design and IMPL: `internal/` verbatim,
   `cmd/docz-api/main.go`, `api/openapi.yaml` as the single copy,
   `charts/docz-api`, the Dockerfile and compose/bake files, the `/v2` import
   rewrite that is the whole of the v2 upgrade, and `config.ParseBytes`. The
   `doczcontract` tests come with it and keep their early-warning role by
   convention.
4. **docz-site in**, as its own design and IMPL: `ui/`, `charts/docz-site`, its
   generated client reading the one remaining spec, and a decision on what
   `orval` runs against now that the spec is a sibling rather than a copy.

Each of 2, 3, and 4 gets its own `v2.0.0-beta.N`, and v2.0.0 proper follows 4 —
which is also when the eleven experimental packages freeze, so the beta window
lasts exactly as long as the consolidation does.

The one thing 3 must not inherit from this investigation is its blind spot.
Observation 11 was added after the fact, and the 43 colliding document IDs are
the largest single piece of work in either move; a design for 3 that treats
`docs/` as a directory to copy will discover that during the copy.

Two follow-ups this investigation surfaced that are not part of the
consolidation: **GO-2026-4970** is closed for free by the `go 1.26.5` bump that
Open Question 6 resolved to, and DESIGN-0009 will be migrated while Draft, which
Open Question 9 accepts deliberately rather than by oversight.

<!--docz:recommendation:end-->

<!--docz:references:start-->
## References

- [ADR-0004](../adr/0004-one-repository-docz-docz-api-and-docz-site-as-a-single-go-module.md)
  — the decision record this investigation recommended, carrying Open Questions
  1, 2, 5, 6, 8, 9, and 10 as decisions.
- [ADR-0002](../adr/0002-docz-is-an-api-package-whose-first-consumer-is-the-cli.md)
  — Decision 6 commits to the consolidation and declines to specify it;
  Decision 7 is the experimental-until-v2.0.0 rule this investigation's deadline
  argument rests on.
- [ADR-0001](../adr/0001-pkgdoczcore-as-the-single-public-core-cmd-as-a-thin-cli-shell.md)
  — Decision 6's v1.0.0 freeze on the five packages, and Open Question 7's
  roll-forward semver, which is what makes an additive `config` bytes API legal.
- [ADR-0003](../adr/0003-remove-plan-from-the-built-in-document-types.md)
  — the `plan` removal that Observation 3 sizes as a non-event for docz-api.
- [DESIGN-0007](../design/0007-docz-changes-to-support-docz-api-and-docz-site.md)
  — the original promotion of the surface docz-api consumes.
- [DESIGN-0008](../design/0008-docz-api-cross-repo-docz-registry-and-ingestion-service.md)
  — docz-api as a separate-repo service, and the R1–R12 contract clauses.
- [DESIGN-0009](../design/0009-docz-site-cross-repo-docz-viewer-and-search-ui.md)
  — docz-site, still Draft.
- [DESIGN-0014](../design/0014-the-docz-api-as-one-unit-packages-types-functions-and-the-cmd.md)
  — §2.10 on `wiki`'s placement and §7 R8 on the library importing no OTel.
- [IMPL-0018](../impl/0018-v200-beta1-the-docz-api-as-one-unit-structured-regions-and-the.md)
  — Out of Scope lists the move; Phases 1–3 built the surface Observation 1
  finds unused by docz-api.
- [docz-api](https://github.com/donaldgifford/docz-api) — inventoried from the
  local checkout at `~/code/docz-api`; module `github.com/donaldgifford/docz-api`.
- [docz-site](https://github.com/donaldgifford/docz-site) — inventoried from the
  local checkout at `~/code/docz-site`; no Go module.
- Layout precedents for a bundled frontend in a Go repository:
  [Prometheus `web/ui`](https://github.com/prometheus/prometheus/tree/main/web/ui),
  [Vault `ui`](https://github.com/hashicorp/vault/tree/main/ui),
  [Consul `ui`](https://github.com/hashicorp/consul/tree/main/ui),
  [Argo CD `ui`](https://github.com/argoproj/argo-cd/tree/master/ui).
- [Go modules reference on multi-module repositories](https://go.dev/ref/mod#modules-overview)
  and [`go.work` workspaces](https://go.dev/ref/mod#workspaces), for Open
  Question 2.
- [GO-2026-4970](https://pkg.go.dev/vuln/GO-2026-4970) — open against docz's
  `go 1.26.4`, fixed in 1.26.5, which docz-api already uses.

<!--docz:references:end-->
