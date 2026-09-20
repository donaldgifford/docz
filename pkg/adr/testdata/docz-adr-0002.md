---
id: ADR-0002
title: "docz is an API package whose first consumer is the CLI"
status: Accepted
author: Donald Gifford
created: 2026-09-14
---

<!-- markdownlint-disable-file MD025 MD041 -->

# ADR-0002: docz is an API package whose first consumer is the CLI

<!--toc:start-->
- [Summary](#summary)
- [Context](#context)
- [Decision](#decision)
  - [Supporting Data](#supporting-data)
  - [What this supersedes in ADR-0001](#what-this-supersedes-in-adr-0001)
  - [The compatibility contract, before and after the swap](#the-compatibility-contract-before-and-after-the-swap)
- [Consequences](#consequences)
  - [Positive](#positive)
  - [Negative](#negative)
  - [Neutral](#neutral)
- [Alternatives Considered](#alternatives-considered)
- [Open Questions](#open-questions)
  - [1. How is "not yet frozen" signalled on the new packages?](#1-how-is-not-yet-frozen-signalled-on-the-new-packages)
  - [2. How is ADR-0001's record amended?](#2-how-is-adr-0001s-record-amended)
  - [3. How are pre-releases cut before v2.0.0?](#3-how-are-pre-releases-cut-before-v200)
  - [4. Does the swap ship a first-party consumer of the type layer?](#4-does-the-swap-ship-a-first-party-consumer-of-the-type-layer)
  - [5. Are document types built on generics?](#5-are-document-types-built-on-generics)
- [References](#references)
<!--toc:end-->

<!--docz:summary:start-->
## Summary

docz is a Go API package. The `docz` CLI is its first consumer, not its
product. Every operation the CLI performs is a library call with a typed
result, laid out in layers with fixed dependency rules, in one module, with a
standalone `pkg/<type>` package per document type whose root type is `Doc`.
docz ships primitives; consumers own policy. The whole API is built first as
one unit (DESIGN-0014) and `cmd/` is swapped onto it last. This supersedes
the parts of ADR-0001 that kept `template`, `index`, and `wiki` internal and
dropped the plan model, and it lifts ADR-0001's freeze for the new packages
until v2.0.0 is cut. This work does not cut it: it ships `v2.0.0-beta.N`
pre-releases, and v2.0.0 proper waits for docz-api and then the UI to move
into this repo, after the CLI.
<!--docz:summary:end-->

<!--docz:context:start-->
## Context

ADR-0001 (2026-07-03) made `pkg/doczcore/{config,document,docparse,docwrite,toc}`
the public core and froze it at v1.0.0. It was a promotion on demand: only
what an external consumer had asked for went public, and `internal/`
(`template`, `index`, `wiki`) stayed the CLI's private half. That was right
for the demand at the time, and it left the read side and the byte-level
mutators in good shape.

What it did not do is make docz *usable as docz* from a program. The
orchestration that makes `docz update`, `docz create`, `docz init`, and
`docz status set` do their jobs lives in `cmd/`; a consumer that wants any of
it re-implements it. INV-0010 found that happening: sdk-booty-sh carries its
own 260-line IMPL plan model over `docparse`, tempy's IMPL-0001 is blocked on
issue #100 asking docz for one, and docz-api (no checkout) can only reach
facts. IMPL-0014 Decision 3(d) had dropped the plan model on principle —
"docz picking a workflow semantic it has no use for itself" — and the same
principle now cuts the other way: two consumers have the same need, the
grammar is docz's own template, and the CLI has no task surface only because
nothing forced one.

DESIGN-0013 (2026-09-13) worked the problem from first principles and its
review settled the direction: rather than promote one more package on
demand, decide what docz *is*. If docz were built again today it would be an
API package with the CLI as its first consumer — every command one library
call plus printing — and that shape is reachable additively from today's
tree. This ADR records that decision; DESIGN-0014 specifies the API it
implies.
<!--docz:context:end-->

<!--docz:decision:start-->
## Decision

1. **docz is an API package; the CLI is its first consumer.** The API is
   extracted from what the docz CLI, docz-api, sdk-booty-sh, and tempy do
   with docz today and specified on its own terms (DESIGN-0014), not around
   any one of them. Every operation the CLI performs is reachable as a Go
   API with typed results. `cmd/` is flags → one call → print. "First
   consumer" means the CLI is the first tool converted onto the API and the
   proof of its shape: each command reduces to a single call, anything a
   command does beyond that line is a gap in the library, and the converted
   CLI must reproduce v1.x's inputs and outputs across every built-in type
   (Decision 6).

   ```go
   // cmd/update.go after the swap — the whole handler.
   func (r *Runner) Update(types []string, dryRun bool) error {
       rep, err := r.Repo.Update(types, repo.UpdateOptions{DryRun: dryRun})
       if err != nil {
           return err
       }
       return r.printUpdate(rep) // wording lives here and nowhere else
   }
   ```

2. **Layers with fixed dependency rules.** The library is five layers; a
   layer imports only layers beneath it.

   ```mermaid
   block-beta
     columns 2
     L4["L4 Presentation — cmd/: cobra, flags, text/json/csv, exit codes, logging"]:2
     L3["L3 Repository operations — pkg/doczcore/repo, index, doctemplate; pkg/wiki"]
     L2["L2 Interpretation — pkg/impl · rfc · adr · design · investigation (Doc), doczcore/validate, doczcore/kinds"]
     L1["L1 Mutation primitives — docwrite byte cores · toc splice · create"]:2
     L0["L0 Facts — document (frontmatter, scan) · docparse (headings, tasks, title) · config"]:2
   ```

   | Rule | Statement |
   | ---- | --------- |
   | R1 | A layer imports only layers below it. L3 and L2 sit side by side: a repository operation may parse a typed document; a type package never needs the repository. |
   | R2 | `pkg/doczcore/*` is type-agnostic. `pkg/<type>` imports `doczcore`; `doczcore` never imports a type package. |
   | R3 | Any operation that needs no filesystem takes `[]byte` and returns values. Path-based variants exist only where in-place mutation is the point, and they call the byte core. |
   | R4 | Results are typed structs and enums. English wording, exit codes, and formatting exist only in L4. |
   | R5 | `internal/` holds only what has a documented reason to stay private. After the swap, nothing does: the template embed moves with `doctemplate` and stays unexported there. |
   | R6 | No library-defined interfaces. Consumers define their own at the call site. |
   | R7 | A type package's grammar is a contract over region *kinds*, not over the type name or the template file (restated 2026-09-19, Open Question 5). `impl.Parse` reads `phase`, `tasks`, and `criteria` regions and never asks which type declared them, so any type whose documents carry those kinds parses with it. A document that carries no markers at all is read by inference from the built-in headings and flagged (DESIGN-0015 §6, amended 2026-09-20); one that carries markers but lacks the kinds has left the contract, whatever its headings say. |

3. **One module, at major version 2.** docz stays a single module with one
   version line. Go compiles per imported package, so a consumer of
   `pkg/impl` pays for nothing else; a second module would add a release
   train and `replace` friction for the CLI itself with no isolation gain.
   ADR-0001 Alternative B stays rejected. The API ships on the **v2 line**,
   so the module path becomes `github.com/donaldgifford/docz/v2` with the
   first landing of this work; every tag this work cuts is a
   `v2.0.0-beta.N` pre-release (Decision 6), and no consumer rewrites
   imports when v2.0.0 proper follows. The v1.x
   tags stay importable at the old path indefinitely — docz-api's and
   sdk-booty-sh's v1 pins are untouched, and neither moves to v2 until
   that is decided on its own — and a `v1` branch is cut from v1.2.2 only
   if a patch is ever needed there. The order of work is the API, then the
   CLI, then consumers.

4. **Standalone `pkg/<type>` packages with `Doc` as the root type.** Per-type
   interpretation lives in a sibling of `doczcore`, one package per document
   type, starting with `pkg/impl`. The root type of every type package is
   `Doc` — `impl.Doc` now, `rfc.Doc` or `adr.Doc` if they ever exist — a
   naming convention, not an interface. There is no registry, no dispatcher,
   and no runtime type system: a caller knows which type it asked for.

   ```go
   package impl // import "github.com/donaldgifford/docz/v2/pkg/impl"

   // Parse interprets an IMPL document. It never touches the filesystem.
   func Parse(doc []byte) (Doc, error)

   type Doc struct {
       ID     string
       Title  string
       Status config.Status
       Phases []Phase
   }
   ```

   Types with no package keep the generic facts API (`document`,
   `docparse`, `config`), which already works on every type including
   custom ones. A type package is a view over region kinds (R7), not over
   a type name: a custom type whose documents carry the kinds a package
   reads gets that package too, and no generics or dispatch are involved
   (Open Question 5).

   > **Amended 2026-09-19.** Every built-in type is a structured type and
   > ships its package in the DESIGN-0014 unit: `pkg/impl`, `pkg/rfc`,
   > `pkg/adr`, `pkg/design`, `pkg/investigation`, with the readers they
   > share (open questions, references, decisions, criteria, alternatives)
   > in `pkg/doczcore/kinds`. "Types with no package" now means custom
   > types on their own schema. Unstructured markdown is not a type; it
   > reaches a consumer through the `api:` block's additional docs. The
   > convention above is unchanged: one package per type, `Doc` as the
   > root, no registry.

5. **Primitives in docz, policy in consumers.** docz reports facts, gives
   typed structure with byte-accurate lines, and performs byte-minimal
   mutations. What a consumer *does* with that — iteration budgets, gates,
   the deferred/skipped lifecycle, diff and retry policy — is the consumer's
   code, in the consumer's repo. The line is drawn by three questions:

   ```mermaid
   flowchart TD
     q1{"Is it a fact of the bytes<br/>or of the type's grammar?"}
     q2{"Would docz's own CLI<br/>call it?"}
     q3{"Does it encode a workflow<br/>choice docz does not make?"}
     docz[["docz: pkg/…"]]
     consumer[["consumer code"]]
     q1 -- yes --> docz
     q1 -- no --> q2
     q2 -- yes --> docz
     q2 -- no --> q3
     q3 -- yes --> consumer
     q3 -- no --> docz
   ```

   `impl.Parse` and `docwrite.SetTaskStateBytes` are docz. A writer that
   appends `— deferred: <note>` to a task is a consumer's few lines over
   `Task.EndLine`. ADR-0001's "a public API docz itself never calls is a
   smell" survives as the second question.

6. **Build the whole API, then swap `cmd/`.** The unit of delivery is the
   complete API specified in DESIGN-0014 and DESIGN-0015 — every package,
   type, function, and error, including the promoted `index`,
   `doctemplate`, and `wiki`, the new `repo` core, `pkg/impl`, the region
   walker, and the validator. Packages land on `main` additively
   under `dont-release` PRs as they are built; the CLI keeps its current
   code paths throughout. No external consumer is part of this work: the
   API is step one, and the CLI's migration onto it is both step two and
   the validation of step one. tempy, docz-api, and sdk-booty-sh are
   awareness only — the packages they rely on shape the API (DESIGN-0014
   §5) — and their moves happen in their own repos, afterwards. The final
   step is one PR that re-points `cmd/` at the library with no change to
   commands, flags, output, or exit codes (ADR-0001 Decision 7, the CLI
   benchmark, still binding). Its acceptance test is functional parity with
   v1.2.2 (DESIGN-0014 §4): every command run over every built-in type —
   rfc, adr, design, impl, investigation — and one custom type, comparing
   outputs, exit codes, and written files against goldens captured from the
   v1.2.2 binary. The only permitted deltas are the region markers in
   created documents (DESIGN-0015) and the commands that are new. `plan`
   is not in the suite: it leaves the catalogue in the same release
   (ADR-0003).

   **Nothing in this work cuts v2.0.0.** Every PR in it, the swap
   included, is `dont-release`; the swap is tagged `v2.0.0-beta.1` by hand
   and later betas follow as needed (Open Question 3). The `major` label
   is reserved for v2.0.0 proper, which is a different milestone: docz-api
   moves into this repo and builds from `cmd/docz-api`, then the UI
   follows, so one chart ships the API and the UI together. Each of those
   is its own design, sequenced after the CLI, and this ADR does not
   specify them.

7. **Supersession and the freeze.** This ADR supersedes ADR-0001 Decision 2's
   retention of `template`, `index`, and `wiki` in `internal/` and IMPL-0014
   Decision 3(d)'s drop of the plan model. It keeps Decisions 1 (whole-package
   promotion), 3 (public may import internal — moot after the swap, still
   legal), 5 (API principles, now rules R3, R4, R6), and 7 (CLI benchmark).
   ADR-0001 Decision 6's v1.0.0 freeze continues to bind the five original
   packages: the v2 line carries their v1 shapes forward unchanged, and the
   one breaking change it makes to them is `plan` leaving the catalogue
   (ADR-0003). The freeze **does not apply** to packages added under this
   ADR until v2.0.0 proper ships — which this work does not cut (Decision
   6): they are marked experimental, may change between betas, and freeze
   at v2.0.0. The beta window is deliberate: docz-api moving into the repo
   is the last chance to reshape them cheaply. From v2.0.0 forward this
   ADR governs the whole surface and ADR-0001 is superseded in part.

   ```mermaid
   stateDiagram-v2
     direction LR
     [*] --> Frozen: five packages, v1.0.0 (ADR-0001)
     [*] --> Experimental: new package lands on main (dont-release)
     Experimental --> Experimental: change between pre-release pins
     Experimental --> Experimental: cmd swap ships as v2.0.0-beta.1
     Experimental --> Frozen: v2.0.0 cut, after docz-api and the UI move in
     Frozen --> Frozen: additive minors only
   ```

8. **Record keeping.** ADR-0001 gets a dated amendment pointing here (its
   status stays Accepted; only parts are superseded — see Open Question 2).
   DESIGN-0013 is Abandoned with a pointer to this ADR and DESIGN-0014; its
   inventory and `pkg/impl` specification carry over into DESIGN-0014
   unchanged in substance. IMPL-0014 Decision 3 gets a note.

### Supporting Data

| Fact | Value | Source |
| ---- | ----- | ------ |
| Public packages | 5 (`config`, `document`, `docparse`, `docwrite`, `toc`) | ADR-0001 |
| CLI-private packages | 3 (`internal/{index,template,wiki}`) | ADR-0001 Decision 2 |
| `cmd/` non-test lines | 4 643 across 11 files; `wiki.go` 368, `status.go` 291, `update.go` 196, `create.go` 160, `init.go` 135 | `wc -l cmd/*.go` (2026-09-14) |
| Orchestration with no library equivalent | `updateType`, `Create`, `Init`/`writeIndexReadme`, `statusSet`/`findByID`, `WikiInit`/`updateWikiNav`, `TemplateExport`/`Override` | DESIGN-0013 §3 inventory |
| Commands reducible to one call today | `config`, `template show`, `version` | DESIGN-0013 §2 |
| Consumers re-implementing docz | sdk-booty-sh `doczwork` (260 lines over `docparse`, pinned v1.0.0); tempy IMPL-0001 blocked on #100 | INV-0010 Obs 1, 2 |
| `cmd/` callers of `docwrite.CheckTask` | 0 | INV-0010 Obs 8 |
| IMPL docs in the fleet the grammar must parse | docz 17, docz-api 10, sdk-booty-sh 3 | INV-0010 Obs 3 |
| Module dependencies of the public core | stdlib + `yaml.v3` | ADR-0001 Neutral |

### What this supersedes in ADR-0001

| ADR-0001 item | Status under this ADR |
| ------------- | --------------------- |
| Decision 1 — whole-package promotion where demand exists | Kept; demand is now the CLI's own needs (Decision 1 above) |
| Decision 2 — the five-package core | Kept as the frozen base |
| Decision 2 — `template`, `index`, `wiki` stay `internal/` | **Superseded**: promoted as `doctemplate`, `index`, `wiki` (DESIGN-0014) |
| Decision 3 — public may import `internal/` | Kept; unused after the swap |
| Decision 4 — `cmd/` + `internal/` are the CLI product | **Superseded**: `cmd/` is the CLI product; `internal/` is empty unless R5 justifies an entry |
| Decision 5 — API principles | Kept as R3, R4, R6 |
| Decision 6 — v1.0.0 freeze | Kept for the five packages, whose shapes carry into v2.0.0 unchanged (ADR-0003 excepted); **does not apply** to new packages before v2.0.0 |
| Decision 7 — CLI feature set is the benchmark | Kept; it is the acceptance test of the swap PR |
| IMPL-0014 Decision 3(d) — no plan model | **Superseded**: `pkg/impl` under R2 and R7 |

### The compatibility contract, before and after the swap

| Surface | Before the swap | After the swap |
| ------- | --------------- | -------------- |
| Module path | `github.com/donaldgifford/docz` | `github.com/donaldgifford/docz/v2` from the first landing; v1.x tags keep the old path |
| `pkg/doczcore/{config,document,docparse,docwrite,toc}` | frozen (v1.0.0) | frozen; additions only |
| `config.DocTypeNames()` | six names | five; `plan` removed (ADR-0003) |
| `pkg/doczcore/{repo,index,doctemplate}`, `pkg/impl`, `pkg/wiki` | experimental; may change between pins | experimental through the betas; frozen at v2.0.0 |
| `cmd/` behaviour (commands, flags, text, exit codes) | unchanged | unchanged by the swap; the looser CLI-stability note applies as before |
| `.docz.yaml` | unchanged | unchanged |
| Embedded template contents and names | not contract | not contract (the embed stays unexported inside `doctemplate`) |
| `internal/` | not contract | empty |
<!--docz:decision:end-->

<!--docz:consequences:start-->
## Consequences

<!--docz:positive:start-->
### Positive

- **docz is usable as docz from a program.** A consumer calls
  `repo.Update` or `impl.Parse` instead of copying `cmd/`.
- **One design, built once.** The API is specified as a whole and landed as a
  whole, so the promotion drip ADR-0001 tried to end actually ends.
- **The CLI becomes the reference consumer** and the forcing function that
  keeps logic out of the shell; the swap PR's diff *is* the proof.
- **Type packages are additive and isolated.** Adding `pkg/rfc` later is a
  package that follows R2 and R7, not a redesign; `doczcore` never learns
  what a phase is.
- **Consumers stay in charge of their workflows.** tempy and sdk-booty-sh
  keep their policy; docz gives them exact lines to act on.
- **The CLI is untouched until the swap.** Every landing before it is
  additive, so `main` behaves as v1.2.2 for CLI users throughout the
  build.
<!--docz:positive:end-->

<!--docz:negative:start-->
### Negative

- **A larger frozen surface at v2.0.0.** `repo`, `index`,
  `doctemplate`, and `wiki` become contract, including shapes ADR-0001 called
  CLI-flavoured (`index.UpdateOutcome`'s action enum). Mitigated by R4 —
  enums and reports are typed, wording is not exported — and by the beta
  window, which lasts until v2.0.0 is cut and in which they may still
  change.
- **Two contracts in the meantime.** Until v2.0.0, the tree carries a
  frozen tier and an experimental tier and contributors must know which is
  which (Open Question 1).
- **The swap is one large PR.** Every command's tests must pass
  byte-for-byte on the new code paths. Mitigated by the existing `cmd/`
  test suite, which already pins output and exit codes.
- **`ParsePlan`'s reversal.** IMPL-0014 dropped a plan model for a stated
  reason; reversing it needs the reason to be recorded as changed (two
  consumers, docz's own grammar), not forgotten. Done in Context.
<!--docz:negative:end-->

<!--docz:neutral:start-->
### Neutral

- **No behaviour change for CLI users.** The swap is a relocation; the
  release notes are for library consumers.
- **Dependencies do not change.** The promoted packages are stdlib +
  `yaml.v3`, like the core.
- **Release mechanics.** `dont-release` on every PR in this work, the
  swap included: a `patch` or `minor` label after the module path flips
  would tag a v1 version on a `/v2` module, which `go get` rejects, and
  `major` is reserved for v2.0.0 proper. Pre-releases are annotated tags
  cut by hand (`make release TAG=v2.0.0-beta.N`); goreleaser already marks
  a suffixed tag as a pre-release (`prerelease: auto`), and `release.yml`
  gains a tag trigger for them in step 0 so a beta builds binaries (Open
  Question 3). IMPL-0017's `updated:` field retargets to the v2 line,
  after this unit.
<!--docz:neutral:end-->
<!--docz:consequences:end-->

<!--docz:alternatives:start-->
## Alternatives Considered

- **A. Promote on demand, again.** Ship `pkg/impl` and the two `docwrite`
  byte cores for tempy and stop (DESIGN-0013 Phase A). *Pros:* smallest
  change. *Cons:* leaves the orchestration gap that produced the re-implementation
  in the first place; the next consumer gets the next drip; the CLI never
  becomes a consumer.
- **B. Typed models inside `doczcore`** (`pkg/doczcore/impl`). *Pros:* one
  import root. *Cons:* every consumer of the core compiles every type model;
  the core stops being type-agnostic; the "pass-through" story for custom
  types gets murky. Rejected in the DESIGN-0013 review.
- **C. A runtime type system** (go-cty style, schema-driven types so custom
  types get models too). *Pros:* uniform. *Cons:* Go consumers want concrete
  structs; docz's grammars are fixed by its own templates; heavy machinery
  for six types.
- **D. Swap `cmd/` incrementally, command by command, releasing as we go.**
  *Pros:* smaller PRs. *Cons:* the API is designed around what each command
  needs in isolation rather than as one unit, which is how today's shape
  happened; partial swaps leave `cmd/` on two code paths for months.
- **E. A separate library module.** Rejected in ADR-0001 and again here
  (Decision 3).
- **F. This ADR.** API first, layers and rules, one module, standalone type
  packages with `Doc`, primitives versus policy, whole API then swap.
<!--docz:alternatives:end-->

<!--docz:open-questions:start-->
## Open Questions

> Each question is numbered; option `a` is my recommendation, later letters
> are alternatives, and the last is a free-form "other".

### 1. How is "not yet frozen" signalled on the new packages?

> **Resolved 2026-09-19: (a).** Low stakes while docz is its own only
> consumer; the `EXPERIMENTAL` line costs nothing and `go doc` shows it.
> With the module already at `/v2`, the pseudo-version says pre-release
> too.

- a. **Package doc comment plus release notes.** Each new package's doc
  comment opens with an `EXPERIMENTAL` line pointing at this ADR; the
  v2.0.0 cut removes it. No import-path churn, no build tags; `go doc` shows it.
  *(recommendation)*
- b. Land them under an `x/` path (`pkg/x/repo`) and move to the final path
  at the swap — visible in the import path, but every beta consumer rewrites
  imports at GA.
- c. Other.

### 2. How is ADR-0001's record amended?

> **Resolved 2026-09-19: (a).**

- a. **Dated amendment note in ADR-0001, status stays Accepted.** The
  DESIGN-0007 pattern: a block under Decision stating which items ADR-0002
  supersedes, with a link. Most of ADR-0001 still governs the five packages.
  *(recommendation)*
- b. Flip ADR-0001 to Superseded and let this ADR restate the surviving
  decisions in full.
- c. Other.

### 3. How are pre-releases cut before v2.0.0?

> **Resolved 2026-09-19: (b) — manual `v2.0.0-beta.N` tags; nothing in
> this work cuts v2.0.0.** The swap is tagged `v2.0.0-beta.1` from its
> `main` merge commit with `make release TAG=…`, and further betas follow
> as the API settles; between tags a v2 pseudo-version
> (`v2.0.0-0.<timestamp>-<sha>`) is available without cutting anything.
> Every PR stays `dont-release`. Two mechanics land in step 0:
> `release.yml` gains an `on: push: tags` trigger for `v*-beta.*` so a beta
> builds and publishes binaries (goreleaser's `prerelease: auto` already
> marks it), and `pr-semver-bump`'s behaviour with a pre-release as the
> latest tag is checked, since the eventual `major` must produce v2.0.0
> and not v3.0.0 — if it cannot, v2.0.0 is tagged by hand the same way.
> That eventual cut is not this work's (Decision 6).

- a. **Pseudo-versions, no tags.** `go get github.com/donaldgifford/docz/v2@<sha>`
  on the `main` commit that carries what the consumer needs. Nothing to cut,
  nothing to clean up, and the release workflow's base version is untouched.
  A tag is added only if a consumer wants a human-readable version.
  *(recommendation)*
- b. Manual `v2.0.0-beta.N` tags on `main` merge commits via `make release
  TAG=…` — readable, but confirm first that `pr-semver-bump` does not adopt a
  pre-release tag as the base for the next bump.
- c. A tag-triggered pre-release workflow with binaries — more than a library
  pin needs.
- d. Other.

### 4. Does the swap ship a first-party consumer of the type layer?

"First consumer is the CLI" read literally means `pkg/impl` should have a
CLI caller when it freezes.

> **Resolved 2026-09-19: (e) — the question was mis-framed.** "First
> consumer" does not mean a first-party caller of `pkg/impl`. It means the
> CLI is the first tool converted onto the API, and the proof is functional
> parity with v1.2.2 across rfc, adr, design, impl, and investigation
> (Decisions 1 and 6, DESIGN-0014 §4). `docz validate` still ships in the
> swap because DESIGN-0015 is part of the unit, not as validation of the
> type layer; `docz task list` stays optional. The API is independent of
> every consumer it was extracted from, and custom types must have the
> same functionality as the built-ins — Open Question 5.

- a. **Yes: `docz validate`.** DESIGN-0015's validator composes
  `repo.Validate` with `impl.Validate`, which runs `impl.Parse`, so the
  swap PR ships a first-party caller of the type layer that also gates the
  corpus. The CLI and the library agree on task IDs by construction.
  *(recommendation, revised 2026-09-19)*
- b. `docz task list <impl-id> [--format text|json]` — read-only over
  `impl.Parse` and `repo.Find`, about a hundred lines, no writers;
  `check` / `uncheck` wait for a later minor.
- c. Both a and b in the swap.
- d. No new command in the swap; tempy is the first consumer of `pkg/impl`
  in practice and the CLI follows later.
- e. Other.

### 5. Are document types built on generics?

Raised 2026-09-19 with the v2 decision: custom types (DESIGN-0006) must
have the same functionality as the built-ins, from the CLI and from the
API, and `plan` becomes the first type that exists only as a custom type
(ADR-0003). What "the same functionality" is, per surface:

| Functionality | Built-in type | Custom type |
| ------------- | ------------- | ----------- |
| `create`, `list`, `update`, `status set`, `init`, `template`, `wiki`, and the `repo` methods behind them | yes | yes today (DESIGN-0006); unchanged by this ADR |
| `validate` against a schema; `validate --fix` | from the embedded template | from its own template, the same way (DESIGN-0015 §3) |
| Facts and regions (`document`, `docparse`) | yes | yes; type-agnostic by R2 |
| `impl.Parse` — phases, tasks, criteria; `docz task list` | IMPL | any document whose template carries the `phase`, `tasks`, and `criteria` regions |
| A Go struct named after the type | `pkg/impl`, in docz | needs Go code: the consumer's own package over `docparse.Regions`, written the way `pkg/impl` is |

> **Resolved 2026-09-19: (a).** R7 and Decision 4 restated above; DESIGN-0014
> §2.9 notes that `impl.Parse` is keyed on kinds. Where the set of kinds a
> type must carry is *declared* — template or config — is DESIGN-0015 Open
> Question 1, reopened the same day.

- a. **Types are data; type packages are views; no generics.** A document
  type is a `types.<name>` config entry plus a template, and the template's
  regions are its schema (DESIGN-0015). A built-in differs from a custom
  type only in shipping a default entry and an embedded template — the
  registry in `doctype.go` is a table of defaults, not a type system.
  Everything in L0, L1, L3, and `validate` is type-agnostic and already
  runs on any type, which is the first three rows above. A type package is
  a Go *view* over a set of region kinds: `impl.Parse` reads `phase`,
  `tasks`, and `criteria` regions and never looks at the type name, so a
  custom `runbook` whose template carries those kinds parses with it and
  `docz task list` works on it. R7 is restated to say so: the grammar is a
  contract over region kinds, not over the type name or the template file.
  The last row is the only thing a custom type cannot have from docz, and
  no design can give it, because a named Go struct is Go code; the
  extension point is that L0 is public and `pkg/impl` is the worked
  example. Go generics do not enter: a type parameter is fixed at compile
  time and a custom type is declared at run time in `.docz.yaml`, so there
  is nothing to instantiate one with, and mapping a type name to a `T`
  needs the registry or `switch` Decision 4 removed — whose arms could only
  ever be the built-in packages anyway. *(recommendation)*
- b. **A generic document shell.** `document.Doc[B any]{Frontmatter; Body B}`
  with `impl.Doc = document.Doc[impl.Body]` and a custom type as
  `Doc[[]docparse.Region]`. One spelling across types, but `repo.Find`
  still cannot return a typed body without knowing `B` at compile time,
  every consumer writes `document.Doc[impl.Body]`, and a type parameter
  with one instantiation per package is the generics anti-pattern.
- c. **A runtime type system** (Alternative C): every type, built-ins
  included, is a schema and `impl.Doc` is a projection over a generic value
  tree. Uniform, but Go consumers want structs, and it is heavy machinery
  for five types.
- d. **A library interface** (`Document` with `ID()`, `Title()`, `Status()`)
  that every `Doc` implements, with a `docparse`-backed implementation for
  custom types. Breaks R6, and the methods would exist only to satisfy it.
- e. Other.
<!--docz:open-questions:end-->

<!--docz:references:start-->
## References

- [ADR-0001](0001-pkgdoczcore-as-the-single-public-core-cmd-as-a-thin-cli-shell.md)
  — the decisions kept, superseded, and amended above
- [ADR-0003](0003-remove-plan-from-the-built-in-document-types.md) — the
  `plan` type removal that lands alongside this work
- [DESIGN-0014](../design/0014-the-docz-api-as-one-unit-packages-types-functions-and-the-cmd.md)
  — the API this ADR implies, specified as one unit
- [DESIGN-0015](../design/0015-structured-regions-and-docz-validate.md)
  — structured regions and `docz validate`, a requirement of that unit
- [DESIGN-0013](../design/0013-library-first-docz-per-type-document-packages-and-a-core-api.md)
  — Abandoned; the first-principles working that led here
- [INV-0010](../investigation/0010-impl-plan-parse-and-write-back-api-for-doczcore-issue-100.md)
  — consumer demand and corpus facts
- [IMPL-0014](../impl/0014-v100-the-five-package-pkgdoczcore-public-core.md)
  Decision 3(d) — the plan-model drop this ADR reverses
- [Issue #100](https://github.com/donaldgifford/docz/issues/100) — tempy's
  request; [Issue #103](https://github.com/donaldgifford/docz/issues/103) —
  remove the `plan` type
<!--docz:references:end-->
