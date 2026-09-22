---
id: ADR-0004
title: "One repository: docz, docz-api, and docz-site as a single Go module"
status: Proposed
author: Donald Gifford
created: 2026-09-21
---

<!-- markdownlint-disable-file MD025 MD041 -->

# ADR-0004: One repository: docz, docz-api, and docz-site as a single Go module

<!--toc:start-->
- [Summary](#summary)
- [Context](#context)
- [Decision](#decision)
  - [Supporting Data](#supporting-data)
  - [What this amends in ADR-0002 and ADR-0001](#what-this-amends-in-adr-0002-and-adr-0001)
- [Consequences](#consequences)
  - [Positive](#positive)
  - [Negative](#negative)
  - [Neutral](#neutral)
- [Alternatives Considered](#alternatives-considered)
- [Open Questions](#open-questions)
  - [1. What happens to the incoming document trees?](#1-what-happens-to-the-incoming-document-trees)
  - [2. How are the root files and the duplicated directories reconciled?](#2-how-are-the-root-files-and-the-duplicated-directories-reconciled)
  - [3. Do the doczcontract tests survive the move?](#3-do-the-doczcontract-tests-survive-the-move)
  - [4. Does ui/ build in CI on every pull request, or only when it changes?](#4-does-ui-build-in-ci-on-every-pull-request-or-only-when-it-changes)
- [References](#references)
<!--toc:end-->

<!--docz:summary:start-->
## Summary

docz-api and docz-site move into this repository, which stays one Go module at
`github.com/donaldgifford/docz/v2` with one version line. `pkg/` does not move:
no public import path changes, so the restructure that INV-0011 investigated
does not happen and its deadline stops existing. The tree grows around `pkg/`
— `cmd/docz` and `cmd/docz-api` for the two binaries, `internal/` for the
server, `api/` for the single OpenAPI spec, `ui/` for the frontend, `charts/`
for both charts — with `just` as the task runner and `go 1.26.5`. The one API
change either move needs is an additive `config.ParseBytes`. The 43 incoming
docz documents are archived verbatim under `docs/archive/` and none are
renumbered, because their IDs collide wholesale and 757 of their 953
cross-references are ambiguous across the two namespaces. This fills the gap
ADR-0002 Decision 6 left open deliberately, and amends it in three places: the
server is `internal/`'s documented reason to exist, `pkg/` may no longer import
`internal/`, and two charts ship side by side rather than one.

<!--docz:summary:end-->

<!--docz:context:start-->
## Context

ADR-0002 Decision 6 committed to this and declined to specify it: "docz-api
moves into this repo and builds from `cmd/docz-api`, then the UI follows, so
one chart ships the API and the UI together. **Each of those is its own design,
sequenced after the CLI, and this ADR does not specify them.**" IMPL-0018
shipped `v2.0.0-beta.1`, so the CLI those moves were sequenced after is done,
and the gap is now the next thing in the way.

INV-0011 inventoried both repositories and resolved seven of its ten questions
on 2026-09-21; the remaining three were answered the same day. Two of its
findings set the terms of this decision.

The first is that **the v2 upgrade is nearly free and was not the reason to
move.** docz-api consumes 19 symbols from three packages — `config`,
`document`, `docparse` — all three frozen at v1.0.0 and carried into v2
unchanged. The entire output of IMPL-0018 Phases 1–3 (`kinds`, `validate`,
`repo`, five type packages) is available to it and unused. "Upgrading docz-api
to v2" is an import-path rewrite plus one additive function.

The second is that the **strongest argument for consolidation has nothing to do
with v2 at all**: `api/openapi.yaml` exists byte-identically in both
repositories, 916 lines, hand-copied, with no sync step. It is linted in
docz-api (`spec.go`, `vacuum-ruleset.yaml`) and consumed in docz-site (orval
reads *its own copy*). That is a correctness problem today, and one repository
dissolves it. The lived cost is the same shape: one logical change needs a pull
request here, then one in docz-api, then one in docz-site, and two Helm charts
release on two schedules.

Against that, INV-0011 Observation 5 is the real objection: docz has 2 direct
requirements and docz-api has 120, so one module makes the library's version
line move when a database driver does. That trade was accepted knowingly, and
this ADR records what has to hold for it to be survivable.

<!--docz:context:end-->

<!--docz:decision:start-->
## Decision

1. **One repository, one module, one version line.** docz-api and docz-site
   move in. The module stays `github.com/donaldgifford/docz/v2`: one `go.mod`,
   one tag, one changelog, one CI pipeline. Multi-module with a `go.work` is
   rejected (Alternative B) — it fixes the dependency graph and leaves the
   three-repository workflow in place for a workspace file to paper over, which
   is the cost actually being paid.

   The consequence is accepted rather than mitigated: `go get
   github.com/donaldgifford/docz/v2` resolves a graph of roughly 120
   requirements instead of 2, and the library's version numbers move with the
   service's dependency churn. Module graph pruning means a consumer of
   `pkg/doczcore/config` still *builds* only what it imports; what it cannot
   avoid is the release cadence.

2. **The layout: `pkg/` does not move.**

   ```text
   cmd/docz/main.go       the CLI binary (unchanged)
   cmd/docz-api/main.go   the server binary
   cmd/*.go               the Cobra tree (unchanged)
   pkg/                   the library, unchanged
   internal/              the server's internals
   api/                   openapi.yaml, the single copy, plus its linter
   ui/                    the Bun/React application
   charts/                docz-api and docz-site, side by side
   test/                  parity and consumer suites (unchanged)
   docs/                  this repository's docz documents
   docs/archive/api/      docz-api's 24 documents, verbatim
   docs/archive/ui/       docz-site's 19 documents, verbatim
   ```

   `pkg/` exists because the library and its consumers were in different
   repositories. That stops being true, and renaming it anyway buys nothing and
   costs every import path. **This is the load-bearing half of the decision**:
   INV-0011 Observation 7 found that a `core/`/`cli/`/`api/`/`ui/` restructure
   changed sixteen import paths, five of them frozen-shape, and therefore had a
   *deadline* — a rename before v2.0.0, a v3 after it. Keeping `pkg/` deletes
   the deadline, the ADR-0001 Decision 6 amendment it would have needed, and the
   restructure as a unit of work. Nothing `v2.0.0-beta.1` published becomes a
   lie.

   `cmd/docz-api` rather than `cmd/server`, because under `cmd/` the directory
   name *is* the binary name (INV-0011 Observation 10): `go install` has no
   override, so a descriptive directory is a name the user types. docz-api's
   incoming tree already spells it `cmd/docz-api`.

   `ui/` rather than `site/` or `website/`: it is the majority precedent for a
   frontend bundled in a Go repository (Vault, Consul, Argo CD; Prometheus uses
   `web/ui`), and `website/` conventionally means the *documentation* site,
   which would collide with `docs/` and the MkDocs wiki.

3. **`internal/` is the server's, and `pkg/` may not import it.** This is the
   one rule the move adds rather than inherits.

   ADR-0002 R5 says `internal/` holds only what has a documented reason to stay
   private, and observed that after the `cmd/` swap nothing did. The server is
   that reason: it is not a public surface, nobody outside this repository
   should import its store or its queue, and `internal/` says so to the
   compiler.

   But ADR-0001 Decision 3 permitted a public package to import `internal/`
   ("moot after the swap, still legal"). It stops being legal here. `internal/`
   is now 120 requirements including seven OpenTelemetry modules, and DESIGN-0014
   §7 R8 says the library imports no telemetry and no logger. One import from
   `pkg/` into `internal/` would make that false silently.

   `pkg/doczcore/layer_test.go` already fails on it —
   `TestLayerRules_ThirdPartyDependencies` allows exactly `go.yaml.in/yaml/v3`
   under `pkg/...`, so any server dependency reaching the library fails there
   today. The move adds one explicit rule forbidding a `pkg/` package from
   importing `modulePath + "/internal/"`, so the failure names the rule instead
   of naming whichever database driver happened to be at the end of the chain.
   That test becomes load-bearing: it is what preserves R8 as a property once
   the module graph stops preserving it as a fact.

4. **`just` is the task runner; the toolchain goes to `go 1.26.5`.** A root
   `justfile` composes `docz.just`, `api.just`, and `ui.just`, so each half
   keeps its own recipes and the root file is composition. docz-api already
   splits `docker.just` out this way, so the pattern is imported rather than
   invented, and the incoming recipes arrive as files rather than as
   translations.

   `go 1.26.5` is forced by sharing a module with docz-api and closes
   **GO-2026-4970**, which is open against docz's current 1.26.4.

   The cost is named so no design treats it as free: `make ci`, `make parity`,
   `make parity-capture`, `make validate`, `make test-consumer`, and `make
   release TAG=` are referenced by the CI workflow, the release path,
   `CLAUDE.md`, `DEVELOPMENT.md`, and `CONTRIBUTING.md`. **The migration lands
   on its own, before either service arrives**, so that if the harness that
   proved this line green breaks, what broke is the runner and not a migration.

5. **`main` stays the v2 line.** `main` already carries `/v2`; every PR is
   `dont-release`; a `v1` maintenance branch is cut from `v1.2.2` only on
   demand. A long-lived `v2` branch would mean reverting `main` to v1 and
   maintaining a months-long divergence for no reader, and would contradict
   what `v2.0.0-beta.1` was cut from. Each milestone below gets its own
   `v2.0.0-beta.N`, which keeps proving the module proxy path and gives each
   milestone a citable artifact.

6. **`config.ParseBytes` is the only API addition either move requires.**
   docz-api fetches `.docz.yaml` through the GitHub API and never checks a
   repository out, so today it writes the fetched bytes to a temp directory on
   **every ingest** and loads from there — and neutralises `$HOME` in its tests
   so a developer's global config cannot leak into assertions (INV-0011
   Observation 2).

   ```go
   // ParseBytes decodes one .docz.yaml's bytes onto the defaults. It reads no
   // file and merges no global config, which is the point: a consumer holding
   // bytes it fetched has neither.
   func ParseBytes(b []byte) (*Config, error)
   ```

   It is additive, so it does not break ADR-0001 Decision 6's freeze
   (roll-forward semver, ADR-0001 Open Question 7). Two things are required of
   it rather than left to the implementation, because this is the third byte
   core docz has added and the first two established the shape: it runs the
   same normalisation `Load` runs (`fillTypeFieldDefaults`, `normalizeChangelog`,
   `normalizeAPI`) so a config parsed from bytes and the same config parsed from
   a file cannot disagree, and a test pins exactly that — `Load` of a
   single-file repository and `ParseBytes` of its bytes produce equal values.
   Validation stays the caller's call, as it is for `Load`.

   This is the whole of the v2 "upgrade" beyond rewriting the import path. It
   ships with the docz-api move, not before it.

7. **The git history comes with them.** Each service is merged with
   `--allow-unrelated-histories` after its incoming paths are rewritten
   (`git filter-repo`), so the commits themselves carry target paths and `git
   log`/`git blame` on `internal/` and `ui/` still explain why a line exists.

   This is cheaper than it sounds for the server: **docz-api's tree already is
   the target layout** for `internal/`, `cmd/docz-api/`, `api/`, and
   `charts/docz-api/`, because Decision 2's layout was derived from it. Only
   `docs/` and the root files need rewriting. docz-site is
   `--to-subdirectory-filter ui` with `charts/docz-site` lifted back to the
   root. The mechanics belong to each move's own design; what this ADR fixes is
   that history is preserved rather than squashed, and the old repositories are
   archived read-only afterwards rather than serving as the historical record.

8. **Sequence, and what v2.0.0 waits for.** Three units, each its own design,
   its own IMPL, and its own beta:

   ```mermaid
   flowchart LR
     j["just migration<br/>v2.0.0-beta.2"] --> a["docz-api in<br/>internal/ · cmd/docz-api<br/>api/ · charts/docz-api<br/>+ config.ParseBytes<br/>v2.0.0-beta.3"]
     a --> u["docz-site in<br/>ui/ · charts/docz-site<br/>orval on the one spec<br/>v2.0.0-beta.4"]
     u --> g(["v2.0.0<br/>eleven packages freeze"])
   ```

   v2.0.0 proper is cut after docz-site lands, and that cut is what freezes the
   eleven experimental packages (ADR-0002 Decision 7). The beta window exists
   so the API can still be reshaped while the consumer that motivated it moves
   in; closing it before the UI is in would spend that window for nothing.

   **Two charts ship side by side**, which amends ADR-0002 Decision 6's "one
   chart ships the API and the UI together". The pain being solved is that one
   logical change needs three pull requests; two charts in one repository do not
   have that problem, and they release on one tag. An umbrella chart stays
   available later as a chart-level decision rather than a repository-level one.

9. **DESIGN-0008 and DESIGN-0009 stand as they are.** Neither is amended and
   neither is superseded. Consolidation is the goal, and an older design that
   contradicts it is evidence of what was true when it was written. Two
   contradictions are therefore accepted deliberately: DESIGN-0008 describes a
   service consuming a pinned library, which stops being how it works, and
   DESIGN-0009 will be migrated while still Draft. DESIGN-0008's R1–R12 keep
   their meaning as a list of behaviours `doczcontract` asserts and lose their
   meaning as a contract against a pin (Open Question 3).

### Supporting Data

| Fact | Value | Source |
| ---- | ----- | ------ |
| docz symbols used by docz-api production code | 19, from 3 packages, all frozen at v1.0.0 | INV-0011 Obs 1 |
| IMPL-0018 Phase 1–3 output used by docz-api | none | INV-0011 Obs 1 |
| `api/openapi.yaml` | 916 lines, byte-identical in both repositories, hand-copied, no sync step | INV-0011 Obs 4 |
| Direct requirements in `go.mod` | docz 2, docz-api 120 | INV-0011 Obs 5 |
| OpenTelemetry modules imported by docz-api | 7 | INV-0011 Obs 5 |
| Import paths changed by this ADR | 0 | Decision 2 |
| Import paths a `core/`/`cli/` restructure would have changed | 16 packages, 5 frozen-shape | INV-0011 Obs 7 |
| `go` directive | docz 1.26.4 (GO-2026-4970 open), docz-api 1.26.5 (fixed) | INV-0011 Obs 8 |
| docz-api paths already matching the target layout | `internal/`, `cmd/docz-api/`, `api/`, `charts/docz-api/` | this ADR, Decision 7 |
| docz documents in the incoming repositories | 43 (docz-api 24, docz-site 19), every ID colliding with one of docz's own | INV-0011 Obs 11 |
| Doc-ID references inside them | 953, of which **757 are ambiguous** across the two namespaces | Open Question 1 |
| Incoming documents already finished | 34 of 43 (Concluded, Completed, Implemented, Approved) | Open Question 1 |
| Duplicated root files and directories | 12 files, 2 directories | INV-0011 Obs 11 |
| Secrets in either repository's tracked history | none; `deploy/secrets/` and `.env.local` are gitignored and appear in no commit | INV-0011 Obs 11 |
| Repository visibility | all three public | `gh repo view` |

### What this amends in ADR-0002 and ADR-0001

| Item | Status under this ADR |
| ---- | --------------------- |
| ADR-0002 Decision 3 — one module at major version 2 | Kept, and extended to cover the service and the UI |
| ADR-0002 Decision 6 — "docz-api moves into this repo and builds from `cmd/docz-api`" | Specified here; the directory name is confirmed as `cmd/docz-api` |
| ADR-0002 Decision 6 — "one chart ships the API and the UI together" | **Amended**: two charts, side by side, on one tag (Decision 8) |
| ADR-0002 Decision 7 — experimental until v2.0.0 | Kept; the cut moves to after docz-site lands (Decision 8) |
| ADR-0002 R5 — `internal/` holds only what has a documented reason | Kept; the server is that reason, recorded here (Decision 3) |
| ADR-0002 R8 / DESIGN-0014 §7 — the library imports no telemetry | Kept, and now enforced by test rather than by the module graph (Decision 3) |
| ADR-0001 Decision 3 — a public package may import `internal/` | **Superseded**: forbidden, and checked (Decision 3) |
| ADR-0001 Decision 6 — the five packages' v1.0.0 freeze | Kept in full; no path and no shape changes. `config.ParseBytes` is additive (Decision 6) |
| INV-0011 Open Questions 1–10 | All ten resolved; 1, 2, 5, 6, 8, 9, 10 are recorded as decisions here |
<!--docz:decision:end-->

<!--docz:consequences:start-->
## Consequences

<!--docz:positive:start-->
### Positive

- **The spec stops being copied by hand.** One `api/openapi.yaml` that the
  server serves, `vacuum` lints, and orval generates from. The only
  correctness problem INV-0011 found in the current arrangement disappears
  rather than being managed.
- **One logical change is one pull request.** The three-repository lockstep
  that motivated this — change docz, then docz-api, then docz-site — collapses
  to one branch, one review, one tag.
- **No import path churn, and no deadline.** Keeping `pkg/` means the
  restructure stops existing as work, `v2.0.0-beta.1`'s published paths stay
  true, and nothing here risks becoming a v3.
- **The library's contract is unchanged.** Five frozen packages keep their
  paths and shapes; the eleven experimental ones keep theirs and freeze at
  v2.0.0 as ADR-0002 said they would.
- **GO-2026-4970 closes as a side effect** of the toolchain bump one module
  forces.
- **Each half keeps its own recipes.** `api.just` and `ui.just` arrive as
  files, which is also what makes the incoming repositories' CI readable
  during the migration.
<!--docz:positive:end-->

<!--docz:negative:start-->
### Negative

- **The library's `go.mod` becomes the service's.** 120 requirements where
  there were 2, and a version line that moves when a database driver does.
  Pruning protects what consumers *build*, not when they get a new version.
  Accepted knowingly (Decision 1); the mitigation is that sdk-booty-sh and
  tempy are on v1 pins and move to v2 on their own schedule.
- **`layer_test.go` becomes load-bearing.** R8 survives as a test rather than
  as a fact about the graph. If that test is ever weakened, the library can
  acquire an OTel dependency and nothing else will notice.
- **`just` is a migration of the harness that proved this line green.** Every
  parity and consumer target moves. Mitigated by landing it alone, first, with
  no other change in the diff.
- **License scanning grows.** `license-check` starts covering the service's
  dependency tree: more work per run and more licences to accept.
- **Two document namespaces coexist permanently.** `docs/archive/api/` and
  `docs/archive/ui/` keep their own ID sequences, so `DESIGN-0003` means one
  thing in `docs/design/` and another two directories away. A one-line rule in
  each archive README is all that distinguishes them, and the MkDocs nav will
  show repeated IDs across its `Archive` and `Design` sections. The
  alternative was 757 ambiguous references resolved by hand (Open Question 1),
  and that trade is deliberate.
- **The archived trees leave docz's own tooling.** Not being under a type
  directory is what keeps them out of `docz update` and `docz validate`, and
  it also means their frontmatter, ToCs, and index tables are never checked
  again. Correct for an archive, and worth knowing before anyone edits one.
- **Root-file reconciliation is a real chore** that the layout diagram hides:
  two `Dockerfile`s, two `ct.yaml`s, two `cliff.toml`s, two `mise.toml`s, two
  `renovate.json5`s, two `catalog-info.yaml`s, two `CLAUDE.md`s, and two
  `deploy/` and `scripts/` directories. Open Question 2.
- **`node_modules/` lands inside the module tree**, which slows `go` tooling
  that walks directories, and `//go:embed` cannot follow a symlink — so any
  embed of the UI points at the built `dist/` only.
<!--docz:negative:end-->

<!--docz:neutral:start-->
### Neutral

- **The v2 "upgrade" is an import rewrite.** Nothing docz-api consumes
  changed on the v2 line, so the move carries a path rewrite and
  `config.ParseBytes` and no port.
- **`plan` is a non-event for both services.** Neither names it; the catalogue
  reaches them only through whatever a repository's `.docz.yaml` declares
  (ADR-0003, INV-0011 Obs 3).
- **`ui/` is invisible to Go tooling.** It has no `go.mod` and no Go files.
- **The old repositories become read-only archives** after their history is
  merged, rather than the historical record (Decision 7).
- **CLI behaviour is untouched.** Nothing in this ADR changes a command, a
  flag, an exit code, or `.docz.yaml`, and the parity suite keeps recording
  v1.2.2 behaviour.
<!--docz:neutral:end-->
<!--docz:consequences:end-->

<!--docz:alternatives:start-->
## Alternatives Considered

- **A. Three repositories, as today.** *Pros:* no work; the library's
  dependency graph stays 2 requirements wide; DESIGN-0008's pin guard keeps
  its mechanism. *Cons:* the 916-line spec stays hand-copied with no sync
  step, which is a correctness problem and not a preference; one logical
  change stays three pull requests and two chart releases.
- **B. Two modules in one repository — the root for library + CLI, `api/` for
  the service — with a `go.work`.** *Pros:* the honest fix for INV-0011
  Observation 5: the library keeps its 2 requirements and its own version
  line, and R8 stays a fact about the module graph rather than a test.
  *Cons:* two version lines, submodule tags (`api/v0.1.0`), and a release
  pipeline that has to know which module it is cutting — and the
  three-repository workflow survives inside one repository, with `go.work`
  papering over it. Rejected in INV-0011 Open Question 2 with the trade
  recorded.
- **C. Restructure into `core/`, `cli/`, `api/`, `ui/`.** *Pros:* the tree
  would say which packages are the library, and `core/config` loses
  `pkg/doczcore`'s redundant element. *Cons:* sixteen import paths change,
  five of them frozen-shape; it needs an ADR-0001 Decision 6 amendment; and it
  has a deadline — a rename before v2.0.0, a v3 after. Declining it is what
  makes this ADR short.
- **D. Stay in three repositories and fix only the spec**, with CI in
  docz-site fetching `api/openapi.yaml` from a docz-api release. *Pros:*
  addresses the one real correctness problem for a fraction of the work.
  *Cons:* leaves every other cost in place, and adds a cross-repository fetch
  to maintain; the spec becomes versioned against a release rather than a
  commit.
- **E. Move docz-api in first and restructure afterwards.** Moot: Decision 2
  removes the restructure.
- **F. This ADR.** One repository, one module, `pkg/` unchanged, the tree
  grown around it, `just`, history preserved, one additive function.
<!--docz:alternatives:end-->

<!--docz:open-questions:start-->
## Open Questions

> Each question is numbered; option `a` is my recommendation, later letters
> are alternatives, and the last is a free-form "other".

### 1. What happens to the incoming document trees?

docz-api carries 24 docz documents (DESIGN-0001–0005, IMPL-0001–0010,
INV-0001–0009) and docz-site carries 19 (DESIGN-0001–0006, IMPL-0001–0007,
INV-0001–0006). docz's own tree holds ADR-0001–0004, DESIGN-0001–0015,
IMPL-0001–0018, INV-0001–0011. **Every incoming ID collides**, and each
document's cross-references — plus code comments in docz-api citing its own
IMPL numbers — resolve in the old namespace.

> **Resolved 2026-09-22: (a) — archive all 43, renumber none.** The question was
> whether to recreate the incoming documents in docz's sequence, and the
> measurement that settled it is the reference load rather than the file count.
>
> | | docz-api | docz-site | total |
> | --- | --- | --- | --- |
> | Doc-ID references in their documents | 626 | 327 | 953 |
> | **Ambiguous** — the ID exists in both namespaces | 482 | 275 | **757** |
> | Unambiguously docz's, must **not** be rewritten | 131 | 15 | 146 |
> | Resolving in neither namespace | 13 | 37 | 50 |
>
> Every ID docz-api owns is also a real docz ID, and its documents already
> cite both namespaces interchangeably — 42 references to `DESIGN-0011`, which
> docz-api does not have, mean docz's `api:` block design. So "DESIGN-0003" is
> resolvable only by reading the sentence around it, which makes the rewrite
> unscriptable: 757 human decisions whose errors are **silent**, because a
> wrong one does not dangle, it points at a different real document.
>
> The second measurement makes the first one moot. **34 of the 43 are finished
> records** — Concluded, Completed, Implemented, or Approved — and renumbering
> a concluded investigation buys nothing: nobody will edit it again, and its
> value is as evidence of what was true when it was written.
>
> So both trees land verbatim under `docs/archive/api/` and `docs/archive/ui/`,
> and **nothing is renumbered, promoted, or rewritten** — not even the nine
> still-live documents, since a hybrid would need a mapping table and would
> break exactly the references the archive exists to keep valid. Work that
> continues gets a **new** docz document in this repository's sequence, citing
> the archived one. Zero renames, zero reference rewrites, and `git log`
> survives on every file.
>
> Three mechanical consequences for the move's own design. They are not under
> a type directory, so `docz update` and `docz validate` never see them —
> which also means their frontmatter is never validated again, and that is
> correct for an archive. `wiki.ScanDocs` walks all of `DocsDir`, so they
> appear in the MkDocs nav under an `Archive` section unless `wiki.exclude`
> names them; the same is true of `api.exclude` for what docz-api publishes.
> And each tree needs a `README.md` stating the namespace rule in one line —
> *inside this directory, an ID means docz-api's* — because that sentence is
> the only thing keeping 757 references honest.

- a. **Archive them in place under `docs/archive/api/` and `docs/archive/ui/`,
  read-only.** Zero renames, and every cross-reference inside them stays
  valid because the namespace travels with the tree: `INV-0003` inside
  `docs/archive/api/` still means docz-api's. They are not under a type
  directory, so `docz update` and `docz validate` never see them. New
  documents about the server or the UI are written in docz's own sequence
  from the move onward. The costs are that `wiki.ScanDocs` walks all of
  `docs/`, so they appear in the MkDocs nav under an `Archive` section with
  IDs that repeat elsewhere in the nav, and that the `api:` block needs an
  `exclude` entry if they should not be published. *(recommendation)*
- b. Renumber into docz's sequence — docz-api's DESIGN-0001–0005 become
  DESIGN-0016–0020, and so on — rewriting every cross-reference. The
  cleanest mechanism for it is `docz create` for each and paste the body,
  since that allocates the ID, the slug, and the index row by construction
  rather than by `git mv` and `sed`. One namespace, at the cost of 43
  recreations, **757 ambiguous references resolved by hand**, severed
  history on every document, and `created:` dates that survive only if the
  frontmatter is pasted with the body.
- c. Give the incoming trees custom types with their own prefixes
  (`docs/api-design/` with `id_prefix: ADESIGN`, and so on), so both
  namespaces are live and distinguishable. Keeps them under `docz update`,
  at the cost of six custom type blocks in `.docz.yaml` and prefixes nobody
  writing a new document would choose.
- d. A hybrid: archive the 34 finished records and recreate only the 9 live
  documents in docz's sequence, with a mapping table in the archive README.
  Rejected on measurement: the live documents are among the most-referenced
  in their own trees — docz-api's `INV-0003` is still Open and is its
  second-most-cited document at 47 references — so promoting them out of the
  archive breaks precisely the links archiving exists to keep working.

### 2. How are the root files and the duplicated directories reconciled?

Two `Dockerfile`s, two `ct.yaml`s, two `cliff.toml`s, two `mise.toml`s, two
`renovate.json5`s, two `catalog-info.yaml`s, two `CLAUDE.md`s, two `README.md`s,
two `deploy/` trees, two `scripts/` directories, and `docker-bake.hcl` +
`compose.yaml` from docz-api. The `.gitignore`s must also merge, since
`deploy/secrets/` and `.env.local` are only untracked because docz-api ignores
them.

- a. **One of each at the root, service-scoped where a name allows it:**
  `Dockerfile.api` and `Dockerfile.ui` with one `docker-bake.hcl`, one
  `ct.yaml` covering `charts/`, one `cliff.toml`, one merged `.gitignore`,
  `deploy/` merged by subdirectory (`deploy/api/`, `deploy/ui/`), and
  `scripts/` merged by filename with the two identical `labels.sh` collapsed.
  The two `CLAUDE.md`s fold into this repository's, which is already the
  longest of the three. *(recommendation)*
- b. Keep each service's build files inside its own directory —
  `internal/`-adjacent for the server, `ui/` for the frontend — accepting that
  `docker build` then needs a context argument per service and that `ct.yaml`
  cannot live anywhere but the root.
- c. A `build/` directory holding every Dockerfile, bake file, and compose
  file for both services. Tidiest root; one more indirection for anyone
  following a container build.
- d. Other.

### 3. Do the `doczcontract` tests survive the move?

`internal/doczcontract` has no runtime code. Its whole purpose is that a docz
bump fails *there* rather than deep inside `internal/ingest`. In one repository
there is no pin, so the drift it guards against cannot happen between releases
(INV-0011 Observation 9).

- a. **Keep them, in place and unchanged.** The early-warning property is
  still worth having — "the contract changed" is a better failure than "an
  ingest test failed" — and it now guards against a library change made in the
  same commit rather than against a pin bump. Cheap, and the R1–R12 numbering
  keeps a home. *(recommendation)*
- b. Fold them into `test/consumer`, which already proves the public surface
  importable from outside the module, and delete the duplicate coverage.
  One place for "the library's shape is as promised"; loses the framing that
  ties each assertion to a DESIGN-0008 clause.
- c. Delete them: the pin is gone, so is the drift.
- d. Other.

### 4. Does `ui/` build in CI on every pull request, or only when it changes?

The Go jobs are seconds; a Bun install, a Vite build, a Vitest run, and
Playwright e2e are not. One repository means one `pull_request` trigger over
both.

- a. **Path-filtered jobs.** `ui/**` runs the frontend jobs; everything else
  runs the Go jobs; a change touching `api/openapi.yaml` runs both, because
  that is exactly the drift consolidation exists to catch. *(recommendation)*
- b. Everything on every pull request. Simplest and slowest; a doc-only change
  waits for Playwright.
- c. A merge queue with the full suite on the queue and path-filtered jobs on
  the pull request.
- d. Other.
<!--docz:open-questions:end-->

<!--docz:references:start-->
## References

- [INV-0011](../investigation/0011-consolidating-docz-api-and-docz-site-into-one-repo-layout.md)
  — the inventory this ADR records: ten open questions, all resolved, and the
  observations every Supporting Data row above cites.
- [ADR-0002](../adr/0002-docz-is-an-api-package-whose-first-consumer-is-the-cli.md)
  — Decision 6 commits to the consolidation and declines to specify it;
  Decision 7 is the experimental-until-v2.0.0 rule, and R5/R8 are the two
  rules Decision 3 above amends.
- [ADR-0001](../adr/0001-pkgdoczcore-as-the-single-public-core-cmd-as-a-thin-cli-shell.md)
  — Decision 3 (public may import `internal/`), superseded here; Decision 6's
  freeze, kept; Open Question 7's roll-forward semver, which makes
  `config.ParseBytes` legal.
- [ADR-0003](../adr/0003-remove-plan-from-the-built-in-document-types.md)
  — the `plan` removal, a non-event for both services.
- [DESIGN-0008](../design/0008-docz-api-cross-repo-docz-registry-and-ingestion-service.md)
  — docz-api as a separate-repo service, and the R1–R12 clauses `doczcontract`
  asserts. Left as it stands (Decision 9).
- [DESIGN-0009](../design/0009-docz-site-cross-repo-docz-viewer-and-search-ui.md)
  — docz-site, still Draft, migrating in that state (Decision 9).
- [DESIGN-0014](../design/0014-the-docz-api-as-one-unit-packages-types-functions-and-the-cmd.md)
  — §7 R8, the library is traceable and not tracing, which Decision 3 keeps
  alive by test.
- [IMPL-0018](../impl/0018-v200-beta1-the-docz-api-as-one-unit-structured-regions-and-the.md)
  — shipped `v2.0.0-beta.1`, the CLI these moves were sequenced after.
- [docz-api](https://github.com/donaldgifford/docz-api) — module
  `github.com/donaldgifford/docz-api`, `go 1.26.5`, public.
- [docz-site](https://github.com/donaldgifford/docz-site) — TypeScript on Bun,
  no `go.mod`, public.
- [GO-2026-4970](https://pkg.go.dev/vuln/GO-2026-4970) — open against
  `go 1.26.4`, closed by Decision 4's bump.
- [`git filter-repo`](https://github.com/newren/git-filter-repo) — the path
  rewriting Decision 7 relies on.
- Layout precedent for a bundled frontend:
  [Vault `ui`](https://github.com/hashicorp/vault/tree/main/ui),
  [Consul `ui`](https://github.com/hashicorp/consul/tree/main/ui),
  [Argo CD `ui`](https://github.com/argoproj/argo-cd/tree/master/ui),
  [Prometheus `web/ui`](https://github.com/prometheus/prometheus/tree/main/web/ui).
<!--docz:references:end-->
