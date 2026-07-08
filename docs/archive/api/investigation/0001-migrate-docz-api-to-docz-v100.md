---
id: INV-0001
title: "Migrate docz-api to docz v1.0.0"
status: Concluded
author: Donald Gifford
created: 2026-07-07
---
<!-- markdownlint-disable-file MD025 MD041 -->

# INV 0001: Migrate docz-api to docz v1.0.0

**Status:** Concluded
**Author:** Donald Gifford
**Date:** 2026-07-07

<!--toc:start-->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
  - [Spike results](#spike-results)
  - [Consumed surface (the baseline to diff against)](#consumed-surface-the-baseline-to-diff-against)
  - [Go-module implications of a v0 to v1 bump](#go-module-implications-of-a-v0-to-v1-bump)
  - [Version skew: CLI vs library](#version-skew-cli-vs-library)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
- [References](#references)
<!--toc:end-->

## Question

Can `docz-api` adopt **docz `v1.0.0`** with a minimal, low-risk change —
ideally a `go.mod` pin bump plus `go.sum` settle — given that the library is
used only through a narrow, contract-guarded surface at the ingest boundary?

Concretely: does the `v0.5.0 → v1.0.0` jump change any of the exact symbols,
types, or struct fields we consume, and if so, what code changes are required
and how big is the blast radius?

## Hypothesis

The migration is **low-risk and probably close to a one-line pin bump**, for
three reasons:

1. **The surface is small and walled off.** docz is confined to ~7 symbols in
   `pkg/doczcore/{config,document}`, all imported via the `doczcfg` / `doczdoc`
   aliases and all exercised by `internal/doczcontract` — a runtime-code-free
   guard whose whole job is to fail *here* on a surface change, not deep in
   ingest.
2. **A v0→v1 bump does not move import paths.** Go's module rules only require a
   `/vN` path suffix at **v2 and above**; `v1.0.0` keeps
   `github.com/donaldgifford/docz/...` unchanged, so the largest mechanical
   breakage class (rewriting every import) is off the table.
3. **What remains is intra-major API drift** — a renamed/removed symbol, a
   changed `Config` / `TypeConfig` / `Frontmatter` field, or a changed `Load` /
   `ParseFrontmatter` signature. That is exactly what the contract test detects.

Expected outcome: the contract tests either pass (→ trivial bump) or pinpoint
the precise deltas (→ a scoped follow-up). Because `v1.0.0` is the **first
stable major after `v0.5.0`** (no `v0.6+` in between) and 0.x carries no
stability guarantee, breaking changes are *possible* and must be verified, not
assumed absent.

## Context

- docz is pinned at `v0.5.0` in `go.mod` (a plain `require`, no `replace`) — the
  version selected in **DESIGN-0001**.
- `v1.0.0` is now published and is the latest tag
  (`go list -m -versions github.com/donaldgifford/docz` ends `… v0.5.0 v1.0.0`).
- Two motivations to move: (1) track the first stable major; (2) resolve a
  **version skew** — the Go library is pinned `v0.5.0`, but the docz *CLI* in
  `mise.toml` floats to `latest`, which now resolves to `v1.0.0`. The build tool
  and the linked library already disagree.

**Triggered by:** DESIGN-0001 (the docz pin decision) and the
`internal/doczcontract` surface guard.

## Approach

Run the spike on a throwaway branch; the contract test is the first gate.

1. **Confirm the target.** Read the docz `v1.0.0` release notes / `CHANGELOG` for
   a documented breaking-change list; skim the tag's
   `pkg/doczcore/config` and `pkg/doczcore/document` public API.
2. **Bump in isolation.** `go get github.com/donaldgifford/docz@v1.0.0` +
   `go mod tidy` (targeted; the repo forbids a bare tidy while deps are staged —
   settle `go.sum` with `go get` if needed). Note any `go` directive bump and
   any transitive shifts (docz pulls `spf13/viper`).
3. **Gate on the contract.** `go test ./internal/doczcontract/...` — this alone
   tells us whether the seven consumed symbols still hold.
4. **Prove the pipeline.** `just build` → `just test` → `just test-integration`
   (the `internal/e2e` onboarding + search tests exercise the real
   fetch→parse→map→reconcile→index path against Postgres + Meilisearch).
5. **Diff the surface by hand** against the baseline in
   [Findings](#consumed-surface-the-baseline-to-diff-against): every symbol,
   type, and field below must still exist with a compatible shape.
6. **Exercise the CLI at v1.0.0** against this repo's own `.docz.yaml`:
   `docz update` (regenerate ToCs/indices) and `docz create inv --no-update` on a
   scratch title — confirm the manifest schema and generated frontmatter are
   unchanged.
7. **Decide the rollout** (see [Recommendation](#recommendation)).

## Environment

| Component | Version / Value |
|-----------|-----------------|
| docz library (current pin) | `v0.5.0` (`go.mod`, plain `require`, no `replace`) |
| docz library (target) | `v1.0.0` (confirmed latest published tag) |
| docz CLI (mise) | `mise.toml`: `"github:donaldgifford/docz" = "latest"` → resolves `v1.0.0` (local install lags at `v0.4.1`) |
| Go directive | `go 1.26.4` (`go.mod`) — kept in lock-step with `mise.toml` |
| Module path | `github.com/donaldgifford/docz` (unchanged at v1 — no `/v1` suffix) |
| Consumed packages | `pkg/doczcore/config` (`doczcfg`), `pkg/doczcore/document` (`doczdoc`) |
| Contract guard | `internal/doczcontract` (test-only; asserts R1–R5) |

## Findings

The spike was run on branch `feat/inv-docz-v100` (`go get docz@v1.0.0`). The
consumed surface held; results are first, with the baseline surface it was
diffed against kept below for reference.

### Spike results

**The consumed surface is fully intact — the bump is mechanical.** Evidence:

- **`go.mod`:** single-line bump `docz v0.5.0 → v1.0.0`; the `go` directive is
  unchanged (`1.26.4`) and no other direct `require` moved.
- **Contract gate green:** `go test ./internal/doczcontract/...` passes — all
  seven guarded symbols (R1–R5) still resolve with compatible shapes.
- **Consumers green:** `internal/ingest`, `internal/githubapp`, and
  `internal/config` unit tests pass; `just build`, `just test`,
  `just test-integration`, and `just lint` (0 issues) all pass; `go mod verify`
  reports all modules verified.
- **Graph impact is minimal and correct.** The `go.sum` delta (~61 lines out,
  ~32 in) is the honest `v0.5.0 → v1.0.0` change, **not** cruft: `go mod tidy`
  run against the branch is a **no-op** — the `go get` output already equals the
  tidy output. `spf13/cobra` and other docz-CLI-only transitives leave the graph.
- **v1.0.0 is a breaking (`feat!`) release** — the *five-package `pkg/doczcore`
  public core* (docz IMPL-0014) — that also **replaced viper with `yaml.v3` in
  `config.Load`**. Neither break reaches us: our two packages (`config`,
  `document`) survived the restructure (the contract proves it), and the viper
  drop is inert because `internal/config` already requires `spf13/viper`
  **directly** (`v1.21.0`), so it stays regardless of docz.
  - **Consequence to record:** DESIGN-0001 Decision 2's rationale — *"importing
    `pkg/doczcore/config` pulls in viper transitively, reused for service
    config"* — is now **obsolete**. viper is a standalone direct dependency, not
    a docz side-effect. No code change; documentation only.
- **CLI/library skew closed:** `mise.toml` is pinned to `1.0.0` (the local
  install still lags at `v0.4.1` until `mise install`).

### Consumed surface (the baseline to diff against)

Everything docz-api depends on, and where. If `v1.0.0` preserves all of this,
the bump is mechanical.

**`pkg/doczcore/config` (aliased `doczcfg`)**

| Symbol | Kind | Consumed at |
|--------|------|-------------|
| `Load(root, dir string) (Config, error)` | func | `internal/ingest/parse.go:35`; `internal/doczcontract` |
| `Config` | struct | returned/threaded through ingest |
| `Config.Types` (`map[string]TypeConfig`) | field | `internal/ingest/service.go:230,246` |
| `Config.Validate() (…, error)` | method | `internal/ingest/service.go:84` |
| `Config.ValidateType(string) (…, error)` | method | `internal/doczcontract` |
| `ErrUnknownType` | sentinel err | `internal/doczcontract` |
| `TypeConfig.Dir` / `.IDPrefix` / `.Statuses` / `.Aliases` | fields | `internal/ingest/mapper.go:32-33`, `service.go` |

**`pkg/doczcore/document` (aliased `doczdoc`)**

| Symbol | Kind | Consumed at |
|--------|------|-------------|
| `ParseFrontmatter([]byte) (*Frontmatter, error)` | func | `internal/ingest/service.go:258`; `internal/doczcontract` |
| `ErrNoFrontmatter` | sentinel err | `internal/ingest/service.go:259` |
| `Frontmatter.ID` / `.Status` / `.Created` | fields | `internal/ingest/mapper.go` (DocID, Status, Created) |
| `IsDoczFile(string) bool` | func | `internal/githubapp/client.go:122` (git-tree filter) |
| `ScanDocuments(string) (…, error)` | func | `internal/doczcontract` **only** (guarded but not on the runtime path — runtime parses bytes via `ParseFrontmatter`) |

Blast radius if a symbol changes: **contained to `internal/ingest`,
`internal/githubapp`, and `internal/doczcontract`.** `httpapi`, `search`, and
`store` use the repo's own DTOs and never touch docz, so a v1.0.0 change cannot
ripple past the ingest boundary.

### Go-module implications of a v0 to v1 bump

- Import paths are **unaffected** — `v1.x` needs no module-path suffix (only
  `v2+` does). No import rewrites.
- `v0.5.0 → v1.0.0` is a stability inflection, not a path change; the risk is
  API content, caught by step 3 above.
- Watch the `go` directive and `spf13/viper` (docz's transitive dep) for
  incidental bumps that could pull `go.mod`/`mise.toml` out of lock-step.

### Version skew: CLI vs library

`mise.toml` floats the CLI to `latest` (now `v1.0.0`) while `go.mod` pins the
library at `v0.5.0`. This is pre-existing drift the migration should close:
after bumping the library, either pin `mise.toml` to `v1.0.0` explicitly or
accept `latest` knowingly, so the tool that scaffolds docs and the library that
parses them are the same major.

## Conclusion

**Answer:** **Yes.** docz-api migrates to `v1.0.0` with a one-line `go.mod` bump
(plus the `go.sum` settle via `go get` and the `mise.toml` CLI pin). Despite
`v1.0.0` being a breaking `feat!` release, the contract-guarded surface is
unchanged and every gate — contract, unit, integration, lint, build, module
verify — is green on `feat/inv-docz-v100`. The one conceptual shift, docz
dropping viper, is inert because `internal/config` depends on viper directly.

## Recommendation

The contract held, so this lands as a single **dependency** change on
`feat/inv-docz-v100` — no IMPL needed:

- Ship `go.mod` (`v1.0.0`), `go.sum` (settled via `go get`, **not** `go mod
  tidy` — it is a no-op here, and the offline-dev caveat that motivates the
  convention still stands), and `mise.toml` (CLI pinned `1.0.0`).
- Update the **current-pin** statement in `CLAUDE.md` and note that viper is now
  a standalone direct dependency (DESIGN-0001 Decision 2's rationale no longer
  applies).
- **Leave** the phase-history lines in `CLAUDE.md` and all of DESIGN-0001 /
  IMPL-0001 as-is — they are frozen point-in-time records of the original
  `v0.5.0` decision, not current-state claims.

## References

- [DESIGN-0001](../design/0001-docz-api-cross-repo-docz-registry-and-ingestion-service.md) — the docz `v0.5.0` pin + `doczcfg`/`doczdoc` alias decision
- `internal/doczcontract/` — the surface guard (R1–R5)
- `go.mod` (docz pin) · `mise.toml` (CLI pin)
- docz releases: <https://github.com/donaldgifford/docz/releases/tag/v1.0.0>
- [Go Modules Reference — major version suffixes](https://go.dev/ref/mod#major-version-suffixes)
