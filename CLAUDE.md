# CLAUDE.md

## Project

`docz` is a Go **library** for generating and managing standardized
documentation (RFCs, ADRs, design docs, implementation plans, investigations)
whose first consumer is its own CLI (ADR-0002). `pkg/` is the product; `cmd/`
is flags, printing, and exit codes over it. Since IMPL-0019 the repository is
also home to **docz-api**, the server (ADR-0004, DESIGN-0016): `cmd/docz-api/`
is its binary and `internal/` is **the server's**, not the library's — one
module, with `pkg/` never importing `internal/` (`TestLayerRules_PkgNeverImportsInternal`
in `pkg/doczcore/layer_test.go`, the fifth layer rule). docz-api's planning
records are archived under `docs/archive/api/`, where an ID means docz-api's;
`wiki.exclude` and `api.exclude` both carry `archive` so neither the wiki nor
the `api:` listing publishes them (pinned together by `test/archive`), and
work they left open continues as INV-0012 and INV-0013. Since IMPL-0020 it
also holds **docz-site**, the frontend (DESIGN-0017): `ui/` is its Bun/Vite/React
project, `charts/docz-site/` its (now deprecated) chart, `deploy/ui/` its compose stacks, and
`Dockerfile.ui` its image, with its records archived under `docs/archive/ui/`
on the same rule. See "Frontend (`ui/`)" below and `ui/CLAUDE.md`. Since
IMPL-0021 both deploy with **one Helm chart**, `charts/docz` (DESIGN-0018):
the API, the site, and the API's backends in one release, with
`charts/docz-api` and `charts/docz-site` kept only as their deprecated final
versions. See "Helm chart (`charts/docz`)" below.

## Build & Test

```bash
just build          # build to build/bin/docz
just test           # run all tests (root module only)
just test-consumer  # the external-module smoke test (test/consumer, own go.mod)
just parity         # replay the v1.2.2 CLI goldens against build/bin/docz
just validate       # docz validate over this repo's own docs/, non-strict
just lint           # golangci-lint + golines
just fmt            # gofmt + goimports
just ci             # lint + test + test-consumer + parity + validate + build + license-check + api::{lint,test,helm-lint} + the ui:: chain
just api build      # the server binary to build/bin/docz-api (api.just, a module)
just api test       # the server's packages only: ./cmd/docz-api/... ./internal/... ./api/...
just ui ci          # the frontend's CI parity: install, gen-api, lint, fmt-check, typecheck, test, build, bundle-budget, gen-api-check
just chart lint     # charts/docz: helm lint with ci/ci-values.yaml (chart.just, a module; also template, unittest, docs)
```

`just` replaced `make` (ADR-0004 Decision 4) and the `Makefile` is gone — there
is no shim. Most old targets now fail loudly (`No rule to make target 'ci'`),
but `make build` and `make test` **exit 0 and do nothing**: `build/` and `test/`
are real directories, so make reads each as an up-to-date target needing no
rule. Muscle memory gets a silent no-op there, not an error. The root `justfile`
is composition only: `set shell`, the imports, `_default` (which is
`just --list`), and the two gates `ci` and `check`. Every docz recipe lives in
`docz.just`, imported flat so its recipes own the root namespace. `api.just` and
`ui.just` are **optional modules** (`mod?`, DESIGN-0016 OQ 1), not imports:
docz-api's recipes share 27 names with `docz.just` and just rejects a duplicate
across sibling imports at parse time, so each arriving half is namespaced —
`just api build`, `just ui build`, and `api::lint` from a root gate — and loads
the moment its file lands. `chart.just` (IMPL-0021) is a third module of the
same kind, for `charts/docz`: `just chart lint|template|unittest|docs`, with
`chart::lint` in the `ci` gate beside `api::helm-lint` until the old charts are
deleted at v2.0.0. All four live at the root; `ui.just` will carry
`set working-directory := "ui"` for its Bun commands. Two forms
changed shape rather than name: `make release TAG=vX` is `just release vX` (a
positional parameter), and `make parity BIN=<path>` is `just bin=<path> parity`
(a variable override, lower-case because just variables are). `just --list`
replaces the awk `help` target, and the `log-%` per-target banner is gone with
nothing in its place: every recipe line is prefixed `@`, so just echoes nothing,
and `lint` and `license-check` run silently to completion — the `✓` lines are
each recipe's own, and `just --dry-run <recipe>` is how you see the commands
without running them. `mise.toml` carries `just` for local development, and CI
installs it with `extractions/setup-just` rather than through mise — this
repository's CI has never used mise-action, pinning each tool with its own action
(`setup-go`, `golangci-lint-action`), and matching that was the smaller change.
`.makefmt.yml`, the `makefmt` tool entry, and `checkmake` went with the Makefile.

The eleven packages this repo made public on the v2 line are **experimental
until v2.0.0 proper ships**: each one's doc comment carries an `EXPERIMENTAL`
paragraph pointing at ADR-0002 Decision 7, and the surface may change between
`v2.0.0-beta.N` tags. The five promoted at v1.0.0 — `config`, `document`,
`docparse`, `docwrite`, `toc` — stay frozen at their v1 shapes, ADR-0003's
removal of `plan` from the catalogue excepted. The markers come off at the
v2.0.0 cut, which no work in IMPL-0018 makes.

Releases on the v2 line are **hand-cut betas**: every PR carries
`dont-release`, `release.yml` stays on `push: branches: [main]` so
`pr-semver-bump` never sees a tag, and a beta is `just release v2.0.0-beta.N`
from the merge commit. `prerelease.yml` triggers on the `v*-beta.*` tag and
goreleaser's `prerelease: auto` marks the GitHub release. The same tag builds
both binaries (docz-api's archives carry a syft SBOM) and its `publish-image`
job calls `ghcr.yml` to push `ghcr.io/donaldgifford/docz-api` and the chart;
ECR stays behind `vars.ECR_PUBLISH_ENABLED`. The image tag is metadata-action's
`{{version}}`, which strips the `v`, so the chart's `appVersion` is **bare**
(`2.0.0-beta.3`) — a `v`-prefixed default 404s on pull, and the chart's
bare-semver test says so.

## Code Conventions

- Go modules with Cobra for CLI, `text/template` for rendering (viper removed in IMPL-0014 Phase 1 — config merging is plain yaml.v3 + a raw-map deep merge)
- `//go:embed` for bundled templates in `pkg/doczcore/doctemplate/templates/` (and the marker skeletons in `templates/schema/`)
- Five built-in doc types: rfc, adr, design, impl, investigation. `plan` was a sixth through v1 and was removed on the v2 line (ADR-0003, IMPL-0018 Phase 4); a repo that keeps its `types.plan` block gets a **custom** type, so everything but `docz create plan` keeps working
- Type aliases: `implementation` -> `impl`, `inv` -> `investigation`
- Templates must have `<!-- markdownlint-disable-file MD025 MD041 -->` after frontmatter
- Lint: `golangci-lint` with `golines` for line length
- Tests: `t.TempDir()` for filesystem tests, golden files under `testdata/golden/`. `pkg/*` tests run in parallel (`t.Parallel()` on every top-level test and subtest); cmd/ tests stay serial because of the package-level `runner` and flag globals. cmd/ tests no longer use `os.Pipe` for output capture or `os.Chdir` for cwd manipulation — they construct a `Runner` with `Out: &bytes.Buffer{}` (or `io.Discard`) and `RepoRoot: t.TempDir()`, and set `repoRoot = dir` when going through `rootCmd.Execute()` so PersistentPreRunE picks up the test dir without process-level cwd changes
- Golden files are regenerated with `go test ./... -update`, never hand-edited
- The module path is `github.com/donaldgifford/docz/v2` (ADR-0002 Decision 3, IMPL-0018 Phase 0). Every in-repo import carries the `/v2` element; released v1.x tags keep the unversioned path, and a consumer pinned there is unaffected by the v2 line. `docz.just` derives the version-injection ldflags from `module_path`, because a `-X` naming a path no package has is ignored silently
- `test/parity/` is the **functional-parity suite** (IMPL-0018 Phase 0, ADR-0002 Decision 4): seven fixture repositories under `fixtures/` (one per built-in type with a template-faithful and a messier hand-written document, one with a custom `frameworks` type resolvable by name/`fw`/`FW`, one whose config carries the dormant pre-v1.1.1 `plan:` block), a driver in `parity_test.go` behind `//go:build parity` that drives a binary named by `DOCZ_PARITY_BIN` with `os/exec`, and 213 goldens under `testdata/<fixture>/<case>.golden` recording stdout, stderr, the exit code, and the file tree (every file with size + digest; full bodies only for what the case wrote). **Goldens are captured from the v1.2.2 release binary and from nowhere else** — `just parity-capture` installs `github.com/donaldgifford/docz/cmd/docz@v1.2.2` into a temp `GOBIN`; re-capturing from a v2 build would make the suite agree with whatever it measures. `just parity` replays (`bin=` to point at another binary) and is part of `just ci` and the `test-go` job (Phase 5). Three named normalisers live outside the build tag with unit tests that run in `just test`: `root` (temp root → `$ROOT`, both the `/var` and `/private/var` spellings), `date` (today **in UTC** → `$DATE`, because `run` pins the child to `TZ=UTC` and the child is what stamps the date — computing it in the runner's own zone made the two agree only where local and UTC name the same day, which is every CI runner and a workstation for part of the day, so a capture baked a literal date in and a replay missed one), `markers` (whole `<!--docz:…-->` lines dropped — the DESIGN-0014 §4 permitted delta; a marker with text beside it is kept, as the walker also does). Permitted deltas are marker lines, new commands/flags, new findings from existing commands, and (from Phase 4) **every trace of the removed `plan` type**, handled by a fourth normaliser `plan`. That one is the only normaliser applied to **both** sides at comparison time — in `runCase`, not in the `norms` list — because the golden is the side carrying the removed type; it drops the `types.plan` block, the `plan: Plans` nav title, the "non-built-in type" warning a legacy repo now gets on every command, the generated `.docz.yaml` comment preamble (v2 says five types, not six), and the `PLAN-XXXX` hint in the IMPL/INV templates. Two consequences of running on both sides: a stdout/stderr block it empties is rewritten to `(empty)`, and a file whose body the golden records loses its size and digest to `$SIZE`/`$SUM` — the body is compared line by line anyway, and the rule has to be symmetric because the side without the trace cannot know one was there. Everything else that differs is a regression. Fixture documents already carry canonical region markers so no later phase migrates them, and provenance (tag, both checksums, toolchain, platform) is in `test/parity/README.md`. **A fixture file may never contain today's date**, pinned by `TestFixturesCarryNoCurrentDate`: the `date` normaliser rewrites it on both sides, so such a file is recorded one way on the day it was authored and read another way afterwards — the `investigation` README carried the authoring day's date in the vestigial second index pair issue #99 left there (which the splice, touching only the first pair, never refreshes), and eleven cases started failing on a file none of them touched once the calendar moved. Fixture dates are frozen to 2026-03-04
- `test/consumer/` is a **separate module** (its own `go.mod`, `replace github.com/donaldgifford/docz/v2 => ../..`, with a `v2.0.0` placeholder in `require` since Go rejects a v0 version on a `/v2` path) that imports **all five** frozen `pkg/doczcore/{config,document,docparse,docwrite,toc}` packages by their public paths as an external consumer would — the DESIGN-0007/IMPL-0013/IMPL-0014 proof that the promoted surface is importable. `internal/` is the server's since IMPL-0019 and Go already refuses it to an outside module; what this module proves instead is that the server's dependencies never reach a consumer of `pkg/` (its `go.sum` fell from 12 lines to 4 when docz-api moved in). `consumer_v2_test.go` (IMPL-0018 Phase 1) adds the v2 type layer: `pkg/doczcore/{kinds,validate}` plus all five type packages, with one `Parse` per package over an **inline** fixture (nothing here writes a file or names a path — that is the point of the layer), `validate.Document` against a schema resolved from an inline skeleton, and both `docwrite` byte cores. Two of the five fixtures are deliberately unmarked so the inference path is proven from outside too, and expected line numbers are *located* in the fixture by a `lineOf` helper rather than written down, so a fixture edit moves them with it. `consumer_v2_promotions_test.go` (Phase 2) adds `doctemplate`, `index`, and `pkg/wiki`; `consumer_v2_repo_test.go` (Phase 3) adds `pkg/doczcore/repo` — the one file that proves the *operations* are reachable rather than just the primitives, exercising `Open`, `Init` twice over (created then skipped), `Create`, the three lookups, `SetStatus` dry-run/live/no-op, `Update`, `Validate`, and `InsertRegions` preview-then-write-then-no-op. Sixteen `pkg/` packages in all. It is outside root `./...`, so `just test-consumer` runs it via a dedicated `go test`, wired into `just ci` and the `test-go` CI job

## Git Workflow

- Always create feature branches + PRs, never push directly to main
- Branch naming: `feat/`, `fix/`, `chore/`, `docs/` prefixes
- Conventional commits required (e.g. `feat(wiki):`, `fix(config):`, `docs:`)
- `docs/examples/plans/` contains user reference material — never commit these files

## Architecture

- `cmd/` — Cobra commands (root, init, create, update, list, status, template, config, validate, wiki, version). **Every command orchestrates through `pkg/doczcore/repo` or `pkg/wiki`** (IMPL-0018 Phase 5, DESIGN-0014 §4): a handler resolves flags, makes one call, prints the report it gets back, and maps a typed error to an exit code. Nothing in `cmd/` walks a directory, renders a template, or splices a marker any more. `cmd.Runner` (runner.go) bundles the resolved Config with injectable dependencies (Out/Err writers, slog logger, time source, GitResolver, `RepoRoot`) **and a `Repo *repo.Repo`**; `repoOrOpen()` (list.go) returns it or builds `&repo.Repo{Root: r.RepoRoot, Cfg: &r.Cfg}` on the spot, which is what lets a test construct a Runner directly and still reach the operations. `cmdContext(cmd)` (runner.go) is `cmd.Context()` with a nil guard, because every RunE wrapper is also called as `runX(nil, args)` by a test and `(*cobra.Command)(nil).Context()` panics. `Execute()` wraps the root in `signal.NotifyContext(…, os.Interrupt)` so ctrl-C cancels mid-run; `repo` checks the context between per-type iterations and never mid-file, so what was written stays written. `loadAndValidateConfig` (root.go) resolves the repo root via `resolveRepoRoot()` — precedence `--repo-root` > `filepath.Dir(--config)` > `os.Getwd()` — then `repo.Open(ctx, root, cfgFile)`, applies `--docs-dir`, and calls `Cfg.Validate()` a **second** time: `Open` drops validation warnings by design (R4) and cmd is the layer that prints them. It then points `rp.Cfg` at `&r.Cfg` so there is one config object, not two that can drift. `(*Runner).hooks()` (hooks.go) maps `repo.Hooks` to slog debug lines and is installed with `cmd.SetContext(repo.WithHooks(…))`, which is how a library that prints nothing still narrates `--verbose`. Log-level resolution: explicit `--log-level` wins, else `--verbose`→debug, else info; invalid values fail at startup. Stable exit codes are signaled by wrapping the `errExitCode1`/`errExitCode2` marker sentinels in an `exitCodeError{msg, marker}`; `exitCodeFor` reads the marker with `errors.Is`. `status.go`'s `statusExitError` is the translation table from `repo`'s typed errors: `UnknownTypeError`/`TypeDisabledError`/`InvalidStatusError` → 2, `NotFoundError` → 1, `WriteError` unwrapped (line endings → 2, else 1). `docz validate` (validate.go, DESIGN-0015 §4) is the one command that composes all **three** validation tiers, because `repo` may not import a type package (R2) and ADR-0002 Decision 4 forbids a registry that would dispatch for it: `repo.Validate` returns the generic and repository findings, then `typeValidator(schema, typeName)` — an explicit five-arm switch, the only type-name switch in the repo — adds the per-type tier. Flags `--strict` (warnings and index drift fail too), `--format text|json`, `--fix` (`repo.InsertRegions`, then re-validate and exit on the **second** report; the first is never printed, since printing findings the next line has already repaired is the most confusing thing it could do). **Routing through `repo` made every command respect `enabled`**, which `config.ValidateType` never did: `list` on a disabled type is an empty listing, `status set` and the three `template` subcommands error, and `update` keeps its silent no-op only because cmd resolves the token itself before calling. Two policies stay in cmd on purpose: `docz init` never overwrites `.docz.yaml` even under `--force` (the library enforces it in `initConfig`, pinned by a test), and `docz wiki init --force` spends the flag by removing `mkdocs.yml` and calling `wiki.Init` with Force **off**, so a hand-edited `docs/index.md` survives — and it removes it **only when it is a regular file** (`os.Lstat`, so a symlink is not followed), because `wiki.mkdocs_path` is a config key that `config.Validate` does *not* run through the repo-relative path rules: before the swap such a value reached `os.WriteFile` and a directory failed there, where unlinking first would delete it (Phase 5 review pass, pinned by `cmd/wiki_force_test.go`). Out-of-root *overwrite* is still reachable and is v1.2.2 behaviour; closing it means validating `wiki.mkdocs_path`, `DocsDir`, and each type's `Dir`, which is a new hard config error and not a permitted parity delta. `getRunner()` is the accessor for tests that bypass PersistentPreRunE — wiki/list tests reset `runner = nil` in their setup helper so the previous test's writer does not leak. **`cmd/*_test.go` is frozen through the swap** (ADR-0001 Decision 7): `git diff --stat <pre-swap> -- 'cmd/*_test.go'` prints nothing, and the only new file is `cmd/validate_test.go` for the new command — the tests are the acceptance criterion, so editing one to make the swap pass would remove the evidence that it did.
- `pkg/doczcore/config/` — **public, semver-governed** (DESIGN-0007 / IMPL-0013: promoted from `internal/config`; imported by docz-api). Config structs, Load(), Validate(), DefaultConfig(), WikiConfig, TOCConfig (YAML key still `toc:` for back-compat), ChangelogConfig; centralized file-mode (`FileMode`, `DirMode`) and filename constants (`ConfigFileName`, `IndexFileName`, `WikiIndexName`, `MkDocsFileName`, `TemplatesDir`, `DefaultChangelogFile`, `APILandingFileName`) in `constants.go`. `ChangelogConfig{Enabled, File}` (IMPL-0015, DESIGN-0010) maps the opt-in `changelog:` block — yaml tags only, since the siblings' `mapstructure` tags are vestigial post-viper. Every config struct also carries `json` tags mirroring its yaml names (issue #89, DESIGN-0008 R11) so docz-api's `config_snapshot` serves `.docz.yaml` spellings rather than Go field names; `json_test.go`'s `TestJSONTags_MirrorYAML` recursively walks every struct reachable from `Config` and fails on a missing/divergent tag (name or `omitempty`), so a new field can't ship untagged, and `TestConfigJSON_MarshaledShape` pins the exact serialized shape. `normalizeChangelog` runs on both `Load` paths after `fillTypeFieldDefaults`: it backfills an explicitly empty `file: ""` to `DefaultChangelogFile` (an omitted key already inherits it via decode-onto-defaults) and strips leading `./` prefixes — deliberately not `filepath.Clean`, which would resolve the `..` that `validateChangelog` must still see. `validateChangelog` (called from `Validate()`) hard-errors on a bad path — but **only when the block is enabled** (DESIGN-0010 Decision 7), so a dormant block never fails load and repos can add it before turning it on. `APIConfig{Enabled, LandingPage, Exclude, AdditionalDocs}` (IMPL-0016, DESIGN-0011) maps the opt-in `api:` block — what docz-api ingests and docz-site renders beyond the type directories; no `cmd/` code reads it, `docz config` just prints it. `api.go` holds `normalizeAPI` (runs on both `Load` paths right after `normalizeChangelog`: backfills `LandingPage` to `<DocsDir>/index.md` only while enabled, and collapses each `Exclude` entry's trailing `/` so the deny-list stores one spelling — a consumer prefix-matching `templates/` as `templates//…` would exclude nothing and publish what the repo meant to withhold) and `validateAPI` (same dormancy rule; wraps `ErrInvalidAPIPath`). Beyond the shared path rules, `validateAPI` rejects a `LandingPage` that its own `Exclude` or `<DocsDir>/templates` covers, an `AdditionalDocs` entry that duplicates another or the landing page (folded case), one under `DocsDir` (compared against a `path.Clean`ed `DocsDir`, since `DocsDir` itself is **not** validated — IMPL-0016 OQ 9), and one whose first path segment — raw or percent-decoded — is a token an enabled type resolves from. That last check reads `Config.resolutionTokens()`, the single definition of the name/alias/registry-alias/`id_prefix` union that `validateResolution` also consumes; a smaller set here would let `inv/x.md` shadow a type route. The shared repo-relative path rules live in `paths.go` (`validateRepoRelativeFile` / `validateRepoRelativeDir` over `validateRepoRelativePath(field, value, allowDir)`, plus `normalizeRepoPath`): empty, control characters (C0/C1/DEL/`unicode.Cf`), backslash, absolute or Windows volume name, leading `~`, trailing `/` unless `allowDir`, `..`/`.`/empty segment, and any segment ending in a space or period (Win32 trims both, so `.. ` and `...` resolve as `..`). Every rule is docz's own rather than the host OS's, because the config is validated on one machine for paths resolved on another; the returned error is bare and each caller wraps it with its own sentinel. Defaults are sourced exclusively from `DefaultConfig()`. `Load` is viper-free (IMPL-0014 Phase 1): `readConfigMap` reads each file into a raw `map[string]any` via yaml.v3 (missing file → empty map), `mergeMaps` deep-merges global-then-repo with repo keys winning, and `decodeSettings` round-trips the merged map through yaml onto a pre-populated `DefaultConfig()`, so sibling fields are preserved. **`ParseBytes(b) (Config, error)`** (IMPL-0019 Phase 1, DESIGN-0016) is the no-checkout entry point for a consumer holding a `.docz.yaml` it fetched: it reads no file and merges no global config, and it is the **same code** as `Load(path, "")` — `loadFromFile` is `os.ReadFile` + the unexported byte core `parseBytes`, which decodes onto `DefaultConfig()` and runs the same four normalisers as the merge path. The INV-0003 types-replace rule needed a bytes form to get there (`applyTypesReplaceOnPresenceIn` / `userListedTypeNamesIn`), because it used to re-read the file at a path to learn whether `types:` was declared. `parsebytes_test.go` pins `ParseBytes` equal to both `Load` paths with `HOME` neutralised (serial, for `t.Setenv`), and proves it ignores a global config `Load` would merge. Unknown keys are ignored (lenient decode) and keys are case-sensitive — both pinned by `parity_baseline_test.go`. `Config.ValidateType(name)` is the single source of "unknown document type" errors (wraps the `ErrUnknownType` sentinel); it delegates to `Config.resolveType` (IMPL-0012), which maps a user token to a canonical `Types` key case-insensitively with precedence **canonical name → alias (built-in registry alias `inv`/`implementation`, or a per-type `TypeConfig.Aliases` entry) → `id_prefix`** — so `docz create FW` / `fw` / a declared alias all resolve a custom `frameworks` type, and name always beats a colliding alias/prefix. `TypeConfig.Aliases []string` holds per-type CLI shorthands (empty for built-ins, whose aliases live in the registry). `Config.EnabledTypes()` returns the enabled-type slice with built-ins first in registry-declaration order, then enabled custom keys appended in sorted order (IMPL-0012 Phase 4 — previously it iterated `DocTypeNames()` and silently dropped custom types, so no-arg `docz update`/`init`/`list`/wiki skipped them); the sort keeps map iteration from making output unstable. `Validate()` calls `validateResolution()`, which builds a `map[token]owner` by "claiming" every token `Config.resolutionTokens()` yields over the enabled-type set — canonical name, each `TypeConfig.Aliases` entry, `id_prefix`, plus the built-in registry aliases of enabled built-ins — all case-insensitively; a token claimed by two *different* types is a hard error ("…collides with type %q: resolution would be ambiguous"), while a token a single type claims twice (e.g. a built-in whose name and `id_prefix` both fold to `rfc`) is fine. This makes duplicate `id_prefix` and alias/name/prefix collisions fail fast at load time. Per-type metadata (canonical name, aliases, default `TypeConfig`, nav title, plural label, template name, help description) lives in the `allDocTypes` registry in `doctype.go` (`DocTypeDef` struct; `DefaultConfig` is a `func() TypeConfig` constructor so each lookup yields a fresh `Statuses` slice). `DefaultConfig().Types`, `DefaultNavTitles()`, the `typeAliases` map, `DocTypeNames()` (the canonical type-name list), and `TypesHelp()` (the `docz --help` body — appends `(alias: …)` automatically from each entry's `Aliases`) all derive from the registry — adding a new doc type is one registry entry plus two embedded templates. `DocTypeNames()` preserves registry-declaration order because `ValidateType`'s error string and existing tests depend on it. The old `ValidTypes()` shim is gone; callers use `DocTypeNames()` (built-in catalog) or `Config.EnabledTypes()` (user's effective set)
- `pkg/doczcore/document/` — **public, semver-governed** (DESIGN-0007 / IMPL-0013: promoted from `internal/document`; the read side). Frontmatter parsing and ID scanning. `ScanDocuments(dir)` (scan.go) walks a type directory, parses frontmatter, and returns `[]DocEntry` with `Content []byte` cached so downstream callers (notably `cmd/update.go`'s ToC pass) do not re-read each document. `LoadFrontmatter(path)` (document.go) is the single read+parse helper; returns `(fm, content, ErrNoFrontmatter)` for files lacking frontmatter so callers (`scanner`, `wiki.DocTitle`) can fall back without a fatal error. `ParseFrontmatter([]byte)` parses already-fetched bytes (docz-api's no-checkout path — DESIGN-0008 R3). `Frontmatter.Status` is typed `config.Status` (DESIGN-0004 §F) — yaml/v3 round-trips it transparently; callers convert at boundaries (e.g. `cmd/list.go` does `string(doc.Status)` for its plain-string listEntry). `Frontmatter.Schema` (yaml `schema,omitempty`, IMPL-0018 Phase 1 / DESIGN-0015 §3) names the marker skeleton a document claims to follow; **empty means the document's own type name**, which is the common case and what every built-in template ships, so a document validates against the baked-in schema without carrying a line for it. This package only round-trips the field — resolving the name to a skeleton and judging whether the name is well-formed belong to the `validate` layer. `DoczFilePattern` / `IsDoczFile` are the single source of truth for the docz filename convention. `ParseChangelog(content []byte) (*Changelog, error)` (changelog.go, IMPL-0015 / DESIGN-0010) parses git-cliff / Keep-a-Changelog markdown into `Changelog{Preamble, Versions}` → `ChangelogVersion{Version, Unreleased, Date, Groups}` → `ChangelogGroup{Title, Items []string}`; the contract is total — either a non-nil result with a nil error or `(nil, ErrNoVersions)`, never a partial result, never a panic (pinned by `FuzzParseChangelog`). Grammar: headings match **only at column 0** (a deliberate divergence from `docparse.Headings`, which trims first — here an indented `### x` is content nested under a bullet), brackets are required, the date separator is lenient, a leading `v`/`V` is trimmed only before a digit, `unreleased` canonicalizes to lowercase and drops any date, and **only indented lines continue an item** so column-0 prose can never be swallowed into commit text. Fence-aware via a locally duplicated copy of the module's fence rule (no `docparse` import — neither package's frozen behavior is coupled to the other's); a fence line inside an item stays in the item. Duplicate versions and duplicate group titles emit separate entries in document order. Fixtures under `testdata/changelog/` (a verbatim snapshot of docz-api's real git-cliff output plus chart/edge cases) with `.golden.txt` fact files (regenerate with `-update`), backed by invariant tests goldens can't express (preamble is a byte-prefix of the input; `Unreleased` iff `Version == "unreleased"`; no item keeps its bullet marker). The **write side** (`Create`, `SetStatus`, `CheckTask`) lives in the sibling `pkg/doczcore/docwrite`.
- `pkg/doczcore/docparse/` — **public, semver-governed** (IMPL-0014 Phase 2, ADR-0001). The module's single markdown **fact extractor**: stdlib-only, bytes-in/values-out, no error returns, facts-not-interpretation (no plan/phase model — that's consumer-side, e.g. sdk-booty-sh's doczwork). `Headings(content []byte) []Heading` walks the whole input for H2–H6 ATX headings (H1 excluded), fence-aware (trimmed-line ``` toggle only; ~~~ does not toggle), strips inline markdown from `Text`, derives GitHub anchor slugs via exported `AnchorSlug` with duplicate `-1`/`-2` suffixes, and reports 1-based LF-accounted `Line`. `TaskItems(content []byte) []TaskItem` returns every `- [ ]`/`- [x]`/`- [X]` checkbox item (`-` or `*` bullets, GFM-style whitespace required after bullet and after `]`): `Text` keeps inline markdown verbatim (only bullet+marker stripped, whitespace trimmed), `Checked` covers x/X, other bracket contents (`[-]`) are omitted entirely, `Indent` is the raw leading-whitespace character count (tabs = 1, never expanded), `Line` is byte-accurate so it feeds `docwrite.CheckTask` directly (proven by the write-through golden test). `Title(content []byte) string` (title.go, IMPL-0016 / DESIGN-0011) returns the first H1's text with inline markdown stripped, or `""` — a normal outcome, not an error, since the consumer supplies the fallback (INV-0007 F2: it exists because a frontmatter-less `CONTRIBUTING.md` has no other title signal). It and `Headings` divide the ATX levels and never report the same line. Unlike `Headings` it also accepts a **setext** H1 (`Title` underlined with `=`), but only from a paragraph line — a list item, blockquote, table row, HTML tag, or punctuation run underlined with `=` is not a title — and it skips a leading YAML frontmatter block, requiring **both** `---` delimiters at column 0 (matching `document.ParseFrontmatter`, and keeping an indented `---` inside a block scalar from exposing YAML as the title) with leading blank lines tolerated and an opener followed by a blank line read as a thematic break. `internal/wiki.firstH1` is gone; `wiki.DocTitle` delegates here, so there is one H1 definition module-wide. Fixtures under `pkg/doczcore/docparse/testdata/` with golden fact files (regenerate with `-update`), plus `FuzzTitle` pinning the never-panic/single-line/trimmed contract. `Markers(content []byte) []Marker` and `Regions(content []byte) []Region` (regions.go, IMPL-0018 Phase 1 / DESIGN-0015 §1) are the **region walker**: `Marker{Kind, Role, Line, Canonical}` reports every `<!--docz:<kind>:start-->` / `:end-->` line in document order *including* stray and non-canonical ones (leniency exists because INV-0009 F4 found that a marker spelled with a stray space made docz skip the file and say nothing), and `Regions` pairs them with a stack into `Region{Kind, Start, End, Depth, Closed}` so nesting is real. `Start`/`End` are the marker lines themselves, so a region's *content* is the lines strictly between them. The legacy `<!--toc:start-->` and `<!-- BEGIN DOCZ AUTO-GENERATED -->` pairs report as the `toc` and `index` kinds (`TocKind`, `IndexKind`), so one vocabulary covers every span docz owns. Fence-aware; an unclosed region comes back `Closed: false` with `End` at the line it was cut off at, so a span is always usable even in a malformed document. `ListItems(content []byte) []ListItem` (`{Text, Ordered, Indent, Line}`) is the bullet/numbered-list walker that `TaskItems` is the checkbox subset of — the ordered item's number is deliberately *not* reported, since markdown renumbers a list from its first value. `Tables(content []byte) []Table` (`{Header, Rows, Line}`) extracts GFM pipe tables: leading/trailing pipes optional, `\|` does not split a cell, a row may be shorter or longer than the header (padding or truncating would invent content, so consumers index defensively or report the mismatch), and `Line` is the header row
- `pkg/doczcore/kinds/` — **public, semver-governed** region readers (IMPL-0018 Phase 1; DESIGN-0014 §2/§5, DESIGN-0015). The layer between a region *span* and a typed *field*: `RegionBytes(doc, r)` cuts a region's bytes, and `Body`, `Field(region, label)` (the `**Answer:**` / `**Answer**:` one-liners), `Items`, `Sections`, `References`, `Criteria`, `Alternatives`, `Decisions`, and `OpenQuestions` each read one shape — all bytes-in/values-out, no error returns, same as `docparse`. **Every reader's `Line` is 1-based within the region, counting the region's own first line as 1**; the `bodyOffset = 1` constant is applied at each public construction site, so a caller converts to a document line exactly one way: `region.Start + Line`. The seven `Shift*` helpers (shift.go) do that conversion for the type packages so five packages don't carry five chances to be off by one (`ShiftQuestions` also shifts the nested `Options` and `Resolved`). `HeadingSpec []HeadingRule` (`{Kind, Level, Text, Prefix, Parent}`) is the inference grammar: `InferRegions(doc, spec)` synthesizes regions from headings for an unmarked document, and `ResolveRegions(doc, spec) (regions, inferred)` prefers real markers and returns `inferred` so a caller can say where the facts came from. **Inference is permanent, not a migration aid** (DESIGN-0015 §6): markers, once present, are authoritative, and a repo that never runs the fixer keeps working forever. A rule matches by exact folded `Text` or by `Prefix` (the IMPL template's `### Phase 1: <!-- Foundation -->` generalizes to `phase`, matching `Phase 3: CI Readiness` and `Phase A: Groundwork` alike). `spanEnd` closes a region at the next heading of the same-or-shallower level **or** at any deeper heading matching a rule that does not descend from it — without that second stop, a document whose `## Objective` is followed only by `### Phase A:` produced two overlapping depth-0 regions that markers cannot express. `SpecFromTemplate(tmpl)` derives a spec from a marked template (the first heading inside each region becomes that kind's rule, the enclosing region its parent, and the shared kinds' defaults are always added), which is what binds each type package's `headings` table to its embedded template in a test. Goldens under `testdata/` (regenerate with `-update`) plus a fuzz target per reader
- `pkg/doczcore/validate/` — **public, semver-governed** generic validator (IMPL-0018 Phase 1; DESIGN-0015 §2–§4). `Document(content []byte, opts Options) []Finding` **never fails**: a document with no frontmatter is a document with a *finding*, because a validator that refused to look at a broken document would be useless on exactly the documents that need it most. `Options{Schema, Type, Filename, Headings, MinHeadings}` is all-optional and the zero value still yields a useful run (marker well-formedness, frontmatter shape, and the content rules of whatever kinds the document happens to carry) — `Filename` empty skips the filename checks, which is what docz-api's no-checkout path needs. `Finding{Code, Severity, Line, Kind, Detail}` with `Severity` `Error`/`Warning`; consumers filter on `Code`, never on wording. `Schema` is the set of regions a document must carry, read from a marker skeleton by `SchemaFromMarkers(skeleton)` — **there is no schema language**, so nothing a schema can require is something a document cannot show, and a schema only tightens by growing. `SchemaFromMarkers` accepts any marked document, which is what makes the derivation test possible: a template and its skeleton must yield the same `Schema`, so the pair can only be edited together. The `catalogue` map (kindrule.go) is the 41-kind per-kind rule set (`KindRule{Singleton, Check}`) — **data, not an interface**, which is why it grew from nine kinds to forty-one without a redesign when every built-in became a structured type; `Singleton` is scoped *by parent*, so an ADR has one positive list inside each consequences region while an IMPL has one tasks region inside each phase, and a kind absent from the catalogue is well-formedness only (an unknown kind is allowed — DESIGN-0015 OQ 8 — so a repo can mark up something docz has no opinion about and still have the markers checked). Code families: `file.*`, `frontmatter.*`, `marker.*`, `region.*`, `content.*`, `open-questions.*`, `references.*`, `tasks.*`, `toc.*`, `schema.name`. When `Options.Headings` is set and the document carries no markers, regions are inferred, one `region.inferred` warning is emitted, and every other check then runs over the inferred regions as if they were marked. The ToC is the one span never reported as `region.missing` — a generated section gets `toc.missing`/`toc.stale` instead, and staleness is checked by regenerating through `toc` rather than by eye, so a finding and `docz update` can never disagree about whether a document needs one
- `pkg/{rfc,adr,design,impl,investigation}/` — **public, semver-governed** typed readers, one package per built-in (IMPL-0018 Phase 1; DESIGN-0014 §2, ADR-0002). **Every built-in is a structured type, rigid by design**: a package per type rather than a generic `kinds`-only surface, because docz-api and docz-site want `doc.Phases[2].Tasks[0].Checked`, not a bag of regions; the escape hatch for everything else is the `api:` block's additional docs. Each package is the same four files — `doc.go` (the `Doc` struct and its lookups), `headings.go` (the `kind*` constants and the `var headings kinds.HeadingSpec` table, exported through `Headings()`), `parse.go` (`Parse(doc []byte) (Doc, error)`), `validate.go` (`Validate(doc []byte) []validate.Finding` and its `Code*` constants) — and the same contract: `Parse` never touches the filesystem and fails for exactly two things, no frontmatter and CR line endings. Everything else a document might be missing leaves its field zero for `validate.Document` to report against the schema, because a half-written proposal is the normal state of a proposal; `pkg/impl` is the one exception, with `ErrNoPhases`, since an IMPL with no phases has nothing of the type in it. `Parse` switches on the **region kind and never on the document's type name** (ADR-0002 R7), which is what lets a custom type carrying these regions parse with a built-in's package. Codes are `<type>.<family>.<rule>` — `rfc.{alternatives.empty,risks.no-mitigation,status.open-question}`, `adr.{decision.empty,consequences.empty,superseded.no-reference}`, `design.{goals.empty,status.open-question,decisions.mismatch}`, `impl.{phase.duplicate-token,phase.no-heading,phase.no-title,phase.no-tasks,task.empty,task.verify-no-command,task.skipped-no-note}`, `inv.{context.no-trigger,conclusion.no-answer,conclusion.verdict}` — plus exactly one `<type>.parse` finding for a document `Parse` rejected, since every other check reads a parsed `Doc`. A type package's production imports stop at L0 (`docparse`, `document`, `config`) plus `kinds` and `validate`; **the core never imports a type package** (DESIGN-0014 §6 R2, pinned by `TestLayerRules_*` in `pkg/doczcore/layer_test.go`), and nothing under `pkg/` imports the server's `internal/` (DESIGN-0016 §7). Each carries a golden corpus under `testdata/` of real fleet documents **snapshotted** as `.orig.md` (never read from `docs/` at test time) plus a generated marked `.md` sibling, with `.golden.txt` fact files (regenerate with `-update`), a `FuzzParse`, and the invariant that inference over the `.orig.md` yields the same facts as its marked sibling in every field but `Inferred` and the line numbers the marker lines shift; each `testdata/README.md` records what the corpus taught us
- `pkg/doczcore/docwrite/` — **public, semver-governed** write side, promoted whole from `internal/docwrite` (IMPL-0014 Phase 3; ADR-0001 amends DESIGN-0007's read-only stance). Deliberately narrow — status + checkbox + create, **not** a general editor. `Create` (create.go) renders a template and writes a new doc with an auto-incremented ID (reuses `document.DoczFilePattern` for the next-ID scan); it imports `pkg/doczcore/doctemplate` (`internal/template` until IMPL-0018 Phase 2, when the public→internal allowance of ADR-0001 Decision 3 stopped having anything to reach) so the embed stays private; `CreateOptions.Type` is typed `config.DocType` (DESIGN-0004 §F). `SetStatus(path, newStatus) (oldStatus, err)` (status.go) is the byte-level frontmatter `status:` mutator (IMPL-0011): it rewrites only the value bytes, preserving key order, quoting shape (bare/`"..."`/`'...'`), spacing, and trailing comments, so the diff is a single line. It rejects CR/CRLF with `ErrUnsupportedLineEndings`, returns `ErrStatusFieldMissing` for an absent or unsupported-shape (block-scalar/flow) status key, and `document.ErrNoFrontmatter` for a file with no `---` block; all IO errors are wrapped with the path. `CheckTask(path, line)` (checktask.go) flips `[ ]`→`[x]` on the given 1-based line with a single-byte splice; the target line is validated by running `docparse.TaskItems` over it (one task-item definition module-wide), with distinguishable sentinels `ErrLineOutOfRange`/`ErrNotTaskItem`/`ErrTaskAlreadyChecked`. The cmd layer owns the current-vs-new no-op short-circuit (DESIGN-0005 Decision 8); the helpers always write when invoked. Golden fixtures live under `pkg/doczcore/docwrite/testdata/golden/{status,checktask}/` (regenerate with `-update`). Every mutator now has a **byte core** (IMPL-0018 Phase 1, DESIGN-0014 §2.7) so a consumer holding bytes it fetched rather than a path it can write has an entry point at all — docz-api reads through the GitHub API and commits through it too: `SetStatusBytes(doc []byte, newStatus string) (out []byte, oldStatus string, err error)`, `SetTaskStateBytes(doc []byte, line int, checked bool) ([]byte, error)`, and `NextNumber(dir string, width int) (string, error)` + `Render(opts *CreateOptions, number string) (Rendered{Filename, Content}, error)`, with `Create` now just `NextNumber` → `Render` → write (a test pins `Render`'s output against what `Create` writes for the same inputs). Each core **never modifies its input** (`bytes.Clone` / fresh slices), so a caller may keep or reuse what it passed in, and returns bare sentinels with no path or line in them — the path wrappers add both. `SetTaskState(path, line, checked)` generalizes the checkbox direction with `ErrTaskAlreadyUnchecked` as `ErrTaskAlreadyChecked`'s mirror, and `CheckTask` is `SetTaskState(path, line, true)` kept by name because that is what IMPL-0011's callers ask for
- `pkg/doczcore/index/` — **public, semver-governed** README index generation, promoted whole from `internal/index` (IMPL-0018 Phase 2, DESIGN-0014 §2.6). `GenerateTable(docs, heading)` builds the table; **`Splice(existing []byte, header, table string) ([]byte, UpdateAction)`** is the whole of the package as a pure function — bytes in, bytes and an action out, no path and no error — for a consumer holding a README it fetched rather than a path it can write. `UpdateReadme`/`DryRunReadme` are now just the filesystem around it and still return the typed `UpdateOutcome{Action, Path, Body}` (`ActionCreated`, `ActionUpdated`, `ActionNoMarkers`, `ActionDryRunCreated`, `ActionDryRunUpdated`); cmd/ owns the user-facing wording. `readExisting` passes **nil for a missing file and the bytes for a present one**, which is the distinction `Splice` reads: a README that exists and is empty has no markers and is left alone, while one that does not exist is docz's to create. The pair is located via `docparse.Regions` kind `index` rather than a string scan, so a marker spelled with a stray space is found and normalised instead of making docz skip the file silently (INV-0009 F4); the splice is line-based and reconstructed to stay **byte-identical** for a canonically marked README, including whether the file ends in a newline. The first index region wins when a README carries two. `BeginMarker`/`EndMarker` are exported because a consumer that scaffolds a README writes the pair and one that renders it finds it. **`Scaffold(header string) []byte`** builds the body for a header with no index yet and appends a pair **only when the header does not already end with one** — issue #99: `index_investigation.md` ends with its own pair (as did `index_plan.md` before ADR-0003) and the creation path appended another, giving every new repo a second permanently empty pair since the splice only touches the first. Checking the header rather than editing that file also covers a repo's own `templates/index_<type>.md` override. `cmd/init` goes through `repo.Init`, which calls `Scaffold` and nothing else, so `docz init` writes exactly one pair (Phase 5). Scanning lives in `pkg/doczcore/document`; the header is resolved by the caller via `doctemplate.ResolveIndexHeader`
- `pkg/doczcore/doctemplate/` — **public, semver-governed** templates, resolution, and rendering, promoted whole from `internal/template` (IMPL-0018 Phase 2, DESIGN-0014 §2.7) under the alias `cmd/` already imported it as. The `embed.FS` stays **unexported** and the template *contents* stay outside the semver contract: they are observable through `docz template show`, and a consumer that wants a template's text calls `Resolve` and treats the result as data. What is governed is the resolution order, the render data (`Data`, `IndexHeaderData`, `WikiIndexData`), and the errors. `Resolve(docType, configPath, docsDir)` is the body-template 3-tier resolver (config path → `<docsDir>/templates/<type>.md` → embedded); a type with none of the three is **`ErrNoTemplate`** wrapped with the type name and the on-disk path a person would create — issue #92, where the old error was the embed FS's "file does not exist" and named neither. `ResolveIndexHeader(docType, docsDir, IndexHeaderData{TypeName, PluralLabel})` (IMPL-0012, DESIGN-0006) is the matching resolver for the README index header: tier 1 on-disk override `<docsDir>/templates/index_<type>.md` (verbatim), tier 2 embedded `templates/index_<type>.md` (verbatim — keeps the five built-in headers byte-identical), tier 3 the embedded generic `templates/index_default.md` rendered with the type's label. Only tier 3 is run through `text/template`, so a literal `{{` in a built-in header or user override is never reinterpreted; this is what makes a custom type's index header resolvable instead of failing embedded-only. **Every embedded body template carries canonical region markers around every section** (IMPL-0018 Phase 1, DESIGN-0015 §1), so a document created by `docz create` is marked from birth and never needs migrating. The matching **marker skeletons** live at `templates/schema/<type>.md` — one per built-in plus `default` for a custom type — and resolve through **`ResolveSchema(name, docsDir) ([]byte, error)`** (`<docsDir>/templates/schema/<name>.md` → embedded) or **`EmbeddedSchema(name) ([]byte, error)`** (baked-in only, for a consumer with no checkout). **Neither tier is rendered**: a skeleton is markers, not a template, and running one through `text/template` would give a literal `{{` in somebody's override a meaning it does not have. The name grammar `[a-z0-9][a-z0-9_-]*` is enforced **before either lookup** (the name arrives from a document's own frontmatter and may say anything), and an illegal name returns both `ErrNoSchema` and **`ErrBadSchemaName`** through one shared helper, so a caller that only wants "there is no schema" tests one sentinel while `validate` can still tell them apart for `schema.name`. A template and its skeleton must yield the same `validate.Schema`, pinned by a derivation test, so the pair can only be edited together. `GenericTemplate()` is the embedded `default.md` under its own name ("default" is not a document type). **`DefaultConfigYAML()`** renders `docz_yaml.tmpl` over `config.DefaultConfig()` and absorbs the rendering its three callers each did, so `docz init` and any consumer that scaffolds a repo produce the same file from the same source of defaults; the template source is deliberately not exported
- `pkg/doczcore/toc/` — **public, semver-governed** ToC splice package, promoted from `internal/toc` (IMPL-0014 Phase 4). Owns only the marker-splice concern: its old walker (`ParseHeadings`/`Heading`/`AnchorSlug`) is retired and every heading walk delegates to `docparse.Headings` via unexported `parseHeadings`, which slices past the first `<!--toc:end-->` line (that skip is toc policy, not a markdown fact) — pinned byte-identical by the golden fixture. `GenerateToC([]docparse.Heading, minHeadings)` builds the list; `UpdateToC` returns `UpdateResult{Updated, Headings []docparse.Heading, Found}` so callers (e.g. `docz update --dry-run`) reuse the parsed headings without a second walk. `UpdateFiles([]FileInput, minHeadings, dryRun)` (update.go) walks a list of in-memory docs, performs the ToC splice + optional write-back, and returns a categorized `UpdateReport` (Updated / Unchanged / WouldUpdate / Skipped / WriteErrors); cmd/ owns all user-facing formatting
- `pkg/doczcore/repo/` — **public, semver-governed** repository tier (IMPL-0018 Phase 3, DESIGN-0014 §2.8), the package that makes "the CLI is one call plus printing" true. `Repo{Root, Cfg}` with **both fields exported** and a struct literal as valid as `Open(ctx, root, configFile)` — docz-api already holds a validated config per request and should not have to re-read it from disk to get the operations; `Open` is `config.Load` + `Validate` so a consumer gets **one** error site instead of five. Pure path helpers `Path`/`TypeDir`/`ReadmePath`/`RelPath`; `RelPath` treats a `..` result as failure even though `filepath.Rel` succeeded, because `../../etc/passwd` reads as a path inside the repo until you count the segments. **Every path the package reports is repo-relative** — the `Entry`/report fields, each `WriteError.Path`, and every hook event — which the Phase 5 review pass found two sites breaking: `InsertRegions`'s `FileWritten` and `writeIndex`'s error plus its two hooks fired absolute, so `docz validate --fix --verbose` logged a different shape from `docz init --verbose` and an index write failure would have put a server's checkout layout into a docz-api error body. Reads: `Scan(ctx, typeName)`, `List(ctx, types)`, `Find(ctx, id)`, `FindIn(ctx, typeName, id)` → `Entry{DocEntry, Type, Path}` with `Path` **repo-relative** (the form every consumer reports). `Find` derives the type from the id prefix, so a consumer holding `IMPL-0018` out of a commit message never says which directory to search; an id with no `-` is `UnknownTypeError` rather than a guess. Writes: `Create`, `Update(ctx, types, UpdateOptions)`, `SetStatus`, `Init`, `Template`, `ExportTemplate`, `Validate`, `InsertRegions`. **`Create` and `Update` share one unexported `updateType`**, so the two index/ToC paths cannot drift; `TypeReport.ToC` is a `*toc.UpdateReport` where nil means "ToC disabled" and non-nil-and-empty means "ran, nothing to do", and every report carries an `Elapsed` per type — the profiling data a consumer wants with no callback wired. **Typed errors everywhere** (errors.go, DESIGN-0014 OQ 9): `NotFoundError{Type,ID}`, `TypeDisabledError{Type}`, `ExistsError{Path}`, `InvalidStatusError{Type,Status,Allowed}`, `UnknownTypeError{Token,Valid}` (**`Unwrap` → `config.ErrUnknownType`**, so the frozen v1 sentinel still answers `errors.Is`), `WriteError{Path,Err}` (unwraps to the cause, so `docwrite.ErrUnsupportedLineEndings` stays reachable while the path comes off the struct). `TypeDisabledError` is deliberately **not** `UnknownTypeError`: "you typed something wrong" and "you turned this off" have different fixes, and `Scan` on a disabled type is this error rather than an empty slice — returning `nil, nil` is what made the two indistinguishable. **Hooks** (hooks.go, DESIGN-0014 §7): `Hooks{ScanStart, ScanDone, TypeSkipped, FileWritten, FileSkipped}` carried in the context via `WithHooks`/`HooksFrom` (the `httptrace.ClientTrace` pattern — a struct of funcs, not an interface, R6), `FileKind`, `SkipReason`, both with a `String()`. This is how a package that prints nothing (R4) still narrates; the five events are exactly the debug lines `cmd/` emits today, so the Phase 5 swap wires them one-to-one. `HooksFrom` never returns nil and `WithHooks(ctx, nil)` clears inherited hooks. **Context enters here** (R8): checked between per-type iterations, never mid-file, and a cancelled run returns the report completed so far with `ctx.Err()` — what was written stays written. `SetStatus` holds the **no-op short-circuit** (DESIGN-0014 OQ 7): `Old == New` returns `Changed: false` without writing, and lookup happens **before** status validation so the two failures keep cmd's exit codes (1 for not-found, 2 for an invalid status). `Init` writes each README as **`index.Scaffold(header)` and nothing more**, which is the issue #99 fix on the `docz init` path. `ExportTemplate` with an empty dest is `docz template override`, and for a custom type whose template resolves nowhere it **scaffolds the generic pair** (template + schema) and returns `Scaffolded: true` — gated on the empty dest, since a schema written beside `/tmp/out.md` resolves for nobody. `Validate` is DESIGN-0015 §4's repository tier: per type the rendered template against its schema, then each document against the schema it names, schemas **cached by name for the run**, plus `toc.stale` and `IndexDrift`. It adds the schema resolver's **third tier** — the type's own template read for its markers when the name is the type's own — which `doctemplate.ResolveSchema` does not have and without which every custom-type document reports `schema.unresolved`. A type whose template resolves nowhere is one `template.unresolved` finding and the run continues (issue #92) rather than aborting every other type. `opts.Strict` is the **caller's** failure threshold and does not change the counts. `InsertRegions` is DESIGN-0015 §6: a document with any `docz:` region is never given more, non-canonical spellings are rewritten and counted in `Fixed`, and everything else gets markers around what `kinds.InferRegions` found. **No heuristic lives here** — `InferRegions` already trims trailing blank lines and a trailing thematic break and re-cuts a parent to its last child, so the pass is arithmetic plus the write; it runs `Regions` over its own output and refuses to write a malformed result (`ErrMalformedOutput`). Canonicalisation runs on **both** branches, not only the already-marked one the flowchart shows, because a non-canonical legacy `toc` marker would otherwise make the second run rewrite the spelling and idempotence is the stronger promise. All **38** `.orig.md` fixtures across the five type packages reproduce their hand-migrated siblings byte-for-byte and a second run is a no-op, docz's own corpus migrates cleanly, and a test-only heading locator over every IMPL fixture yields the same task IDs `impl.Parse` does — the proof that licenses deleting the heuristics. **`repo` never imports a type package** (R2) and is in `layer_test.go`'s `corePackages`, so the rule is enforced rather than asserted; the per-type tier is composed by `cmd/validate.go`, which is the only place in the repo that switches on a document's type name to pick a validator. `Repo` holds no logger and prints nothing.
- `pkg/wiki/` — **public, semver-governed** MkDocs / Backstage TechDocs integration, promoted whole from `internal/wiki` (IMPL-0018 Phase 2, DESIGN-0014 §2.10). A **sibling** of the type packages rather than a member of `pkg/doczcore`, because it is an integration, not core and not a document type. `orchestrate.go` holds the two **operations** — the orchestration `cmd/wiki.go` used to compose inline, so a consumer gets the operation and not just the primitives. `Init(ctx, root, cfg, InitOptions) (InitReport, error)` writes `mkdocs.yml` (at `cfg.Wiki.MkDocsPath`) and the docs landing page (`<DocsDir>/index.md`, rendered from `doctemplate.ResolveWikiIndex`): each file is created when absent, `Skipped` when present, `Overwritten` when present and `InitOptions.Force` is set, and `InitReport{MkDocsPath, MkDocs, IndexPath, Index}` reports both paths plus an `Action` (`Created`/`Skipped`/`Overwritten`, with a `String()` so two consumers can't word the same run differently) for each — the paths are filled before any work, so a report returned alongside an error still says which files were at stake and a zero `Action` means that file was never reached. A file already present is **not** an error at this layer; refusing over a skipped `mkdocs.yml` is `docz wiki init`'s policy. `Init` deliberately does **not** touch the nav — `CreateMkDocs` leaves the `nav: - Home: index.md` placeholder and the caller calls `UpdateNav` next, which keeps the page count in `NavReport` instead of discarded. `UpdateNav(ctx, root, cfg, NavOptions) (NavReport, error)` rebuilds the `nav:` key from the documents under `cfg.DocsDir`, preserving every other key and the existing top-level section order (it reads `ExistingNavOrder` back out of the file even on a dry run, since that ordering is the one behaviour a user notices); `NavReport{Path, Entries, Pages, Written}` returns the `[]NavEntry` tree rather than rendered YAML, so the indented tree `docz wiki update --dry-run` prints is the *caller's* rendering. A missing `mkdocs.yml` surfaces as the read error wrapping `fs.ErrNotExist`; "run `docz wiki init` first" is cmd's wording. Both operations resolve config-relative paths under `root` via unexported `underRoot` (the same rule as `cmd.Runner.inRepo`, so neither calls `os.Getwd`) and check `ctx` between steps, returning the report so far with `ctx.Err()` and leaving already-written files written. Three things stay in `cmd/` on purpose: deriving the site name from the git remote (an L4 dependency, like the author name — `InitOptions.SiteName` arrives resolved, and the package's own fallback reaches no further than `filepath.Base(root)` then `"my-project"`), the `.docz.yaml` precondition, and all printing/logging. The **primitives** stay exported for a different composition: `CreateMkDocs(path, *MkDocsConfig)` builds the initial `mkdocs.yml` (cmd/ no longer constructs YAML strings inline), `ScanDocs`/`BuildNav`/`SortEntries`/`CountPages`, `ReadMkDocs`/`WriteMkDocs`/`NavToYAML`/`MergeNavOrder`, and `DirTitle`/`DocTitle`/`FilenameTitle`. `DocTitle` resolves a nav title as frontmatter `ID: Title` → `docparse.Title` → `FilenameTitle`; the local `firstH1` it used to call is gone (IMPL-0016 Phase 2), which made the H1 scan fence-aware, markdown-stripping, setext-capable, and no longer confused by a mid-document `---`

## Frontend (`ui/`)

docz-site arrived with its full history in IMPL-0020 Phase 2 (DESIGN-0017,
ADR-0004). Its own conventions — Bun, orval, MSW, Playwright, the server
bundle, the chart — are in **`ui/CLAUDE.md`**; this section is only what
the move changed about the repository.

- **`ui/go.mod` fences the npm tree.** It is a stub
  (`module github.com/donaldgifford/docz/v2/ui`) with no Go in it, and it
  exists because `ui/node_modules` ships stray `.go` files (flatted's) that
  the root module's `./...` would otherwise walk into and fail to build. A
  nested `go.mod` ends the root module at `ui/`, so `go test ./...`,
  golangci-lint, and go-licenses need no exclusion. `TestUIModuleFence` in
  `test/archive` pins it exists and that `go list` stops there; the ui CI job,
  the one place `node_modules` is populated, proves it still works.
- **One spec, four readers.** `api/openapi.yaml` is the only copy: docz-api
  embeds it (`api/spec.go`) and serves it at `GET /openapi.yaml`, the
  contract test in `internal/httpapi` reads the same embed, `api::lint-openapi`
  runs vacuum over it, and orval generates the client from it
  (`ui/orval.config.ts` → `../api/openapi.yaml`). The vendored
  `ui/api/openapi.yaml` and docz-site's `spec-drift.yml` are gone, so a spec
  change that breaks the client fails the same PR in `typecheck` or
  `gen-api-check`.
- **The image sees the spec through a named context.** `Dockerfile.ui` builds
  with context `ui/` and gets the spec from bake's `contexts = { spec = "api" }`
  via `COPY --from=spec openapi.yaml /api/openapi.yaml`, laid out so orval's
  `../api/openapi.yaml` resolves inside the stage. Only the spec crosses the
  boundary; `.dockerignore` excludes `ui` from the api image's root context.
- **Three components publish from one tag.** Bake has `-api` and `-ui` target
  families, and `ghcr.yml`/`ecr.yml` take a `component` input
  (`api`|`ui`|`chart`) resolved in one table. `chart` is `charts/docz` alone,
  with `-` for the image fields, so it is called with no tag and the image job
  skips. `prerelease.yml` calls each component, so a `v*-beta.*` tag pushes the
  `docz-api` and `docz-site` images and the `docz` chart. **Charts publish
  only from a tag**: the `chart` job is gated on a `publish_chart` input that
  `release.yml`'s merge-to-main calls set to `false` (IMPL-0021 OQ 2).
  **Each GHCR package needs its own Actions access grant** for this repository
  (`docz-site`, `docz-api`, and `charts/docz` each need one); without it the
  push fails `403 write_package`, and granting it after the first 403 and
  re-running the job is the expected path. `charts/docz` is bumped once per
  release, before the tag, and its `appVersion` is bare semver.
- **CI stays path-filtered** (ADR-0004 OQ 4, revisited as DESIGN-0017 OQ 7).
  The `ui` and `ui-e2e` jobs run on `ui/**`, `Dockerfile.ui`, and
  `api/openapi.yaml`, so a spec change runs both halves; the Go jobs are
  unfiltered. **Revisit the filter the moment a Go change can break the UI
  other than through the spec** — an `//go:embed` of `ui/dist`, a shared
  generated file, a Go test that reads `ui/`. Until then the UI reaches the
  server only over the specced HTTP surface.

## Helm chart (`charts/docz`)

One chart deploys everything (IMPL-0021, DESIGN-0018): `api.*` and `site.*`
are the two workloads, and `store`, `queue`, and `search` are the API's
backends, unchanged from docz-api's chart. `auth`, `otel`, `metrics`,
`serviceMonitor`, `prometheusRule`, and `extraLabels` are shared at the top
level. `chart.just` drives it, and its README is the operator's guide.

- **Names and selectors.** Every object is `<fullname>-<role>` (api, site,
  postgres, postgres-pooler, valkey, meilisearch, test), and the role is
  `app.kubernetes.io/component`, which is in **every** selector, Deployment
  `matchLabels` included. It is safe there because the chart takes new
  installs only (DESIGN-0018 OQ 3c); there is no in-place upgrade from the old
  charts. The workload helpers take `dict "ctx" $ "component" "api"` and read
  `index .ctx.Values .component`, so `docz.hpa`, `docz.ingress`, and
  `docz.httpRoute` serve both workloads as one-line includes.
- **`extraLabels`** reaches metadata and pod templates, never a selector, a
  `volumeClaimTemplate`, or CNPG `inheritedMetadata`, and a key the chart sets
  fails the render.
- **Wiring.** `DOCZ_API_URL` derives from `docz.api.internalUrl` unless
  `site.config.doczApiUrl` is set. `auth.providers` feeds both `AUTH_PROVIDERS`
  and `DOCZ_AUTH_PROVIDERS`, except with `none`: docz-site's whitelist has no
  `none`, and its `/readyz` fails on it, so the site's variable is omitted.
  `otel.endpoint` is one collector URL rendered two ways, because the runtimes
  read it differently: `host:port` for the API's
  `otlptracehttp.WithEndpoint`, and the full `/v1/traces` URL for the site's
  exporter.
- **Edges.** There is one Ingress and one HTTPRoute per workload, each routed
  to its own Service. No Tailscale is in the chart:
  `deploy/tailscale-operator.md` is the only Tailscale documentation, and a
  single shared route is a follow-up (DESIGN-0018 Rollout step 7).
- **Tests.** `tests/api/` and `tests/site/` are the old charts' suites ported;
  `tests/chart/` covers what only a merged chart has (wiring, selectors,
  edges, extraLabels, no Tailscale, versions). helm-unittest runs with
  `-f 'tests/**/*_test.yaml'`. The `.helmignore` anchors `/tests/` and `/ci/`,
  because an unanchored `tests/` also drops `templates/tests/`: the old
  charts never packaged their `helm test` hook. The ci-values therefore run
  busybox `httpd` serving `/healthz`, since `ct install` now runs the hook.

## Server (internal/, cmd/docz-api)

The docz-api server arrived with its full history in IMPL-0019 Phase 2
(DESIGN-0016, ADR-0004): `cmd/docz-api/` is its binary, `internal/` its
library code, `api/` its OpenAPI contract, `charts/docz` its Helm chart (shared
with the site since IMPL-0021; `charts/docz-api/` is the deprecated final),
`deploy/api/` its compose stacks, and `Dockerfile.api` its image. Its recipes
live in `api.just` and run as `just api <recipe>` (`test`, `lint`, and `fmt` scoped to the server's
packages; release tagging, the licence check, and the gates are the root's;
the buildx recipes that were `docker.just` are its `[group('docker')]`). Its
planning documents are archived under `docs/archive/api/` and are **not
rewritten**: `test/archive` pins both that no tracked file outside `docs/`,
`testdata/`, and `CHANGELOG.md` names docz-api's old module path, and
that the five archived documents citing it still do (IMPL-0019 Phase 3).
`internal/ingest` parses a fetched `.docz.yaml` with `config.ParseBytes` — no
temp dir, no `$HOME` merge — and `internal/doczcontract` is gone: its one
surviving clause is `TestConfigLoadsFixtureManifest` in `internal/ingest`, and
`pkg/doczcore`'s own tests pin the rest. `.cliffignore` keeps the grafted
history out of `git-cliff`. The material below is docz-api's own `CLAUDE.md`
from the graft, headings demoted one level; where it names `make`, the
`Makefile`, `internal/doczcontract`, or `docs/` paths, read `just api`,
nothing, nothing, and `docs/archive/api/`.

### Go-specific conventions

- **`go.mod` go directive matches `mise.toml`** (currently `go 1.26.4`). Bump
  both together — Renovate's Go updater handles `go.mod`; bump `mise.toml` in
  the same commit.
- **No `vendor/`**. Modules are resolved at build time; the Docker cache mount
  handles offline-ish builds.
- **`internal/` is a hard wall** — packages there can't be imported by other
  modules. Use it liberally; promote to a separate module only when something
  outside this repo actually needs it.
- **`slog` for structured logs**, not `log` or third-party loggers. Set the
  default handler in `main()` so library code doesn't have to thread loggers.
- **No `init()` for behavior**. `init()` runs at import time — it breaks test
  isolation and surprises future-you. Wire dependencies in `main()`.
- **Tests live next to the code** (`foo_test.go` alongside `foo.go`).
  Integration tests that need external services go under a
  `// +build integration` (or `//go:build integration`) tag and run via
  `go test -tags=integration ./...`.
- **Errors wrap with `%w`**: `fmt.Errorf("loading config: %w", err)`. Top of the
  call stack handles via `errors.Is` / `errors.As`.

### CI matrix

- **GitHub Actions is the only CI** — everything lives under
  `.github/workflows/`. There is **no `.forgejo/workflows/`**; the Forgejo
  mirror is aspirational, so don't assume a Forgejo-primary setup.
- `.github/workflows/ci.yml` runs on every push/PR: `Lint` (golangci-lint via
  the action, **not** `just lint`), `Test Go` (`just test-coverage` + Codecov),
  `Security Scan` (govulncheck + Trivy), and `Build` (goreleaser `--snapshot`
  + SBOM scan). Sibling workflows add CodeQL, license-check (`go-licenses`,
  which needs a root `LICENSE`), trufflehog (verified-only), a changelog drift
  check + auto-regen (`git-cliff` via `cliff.toml`; sync commits match
  `^chore.*changelog` and are skipped to stay idempotent), and a required
  semver label (`major`/`minor`/`patch`/`dont-release`) on PRs.
- `release.yml` fires only on `v*` tag push; `goreleaser` consumes
  `.goreleaser.yml` with `GITHUB_TOKEN` (the `GITEA_TOKEN`/`gitea_urls` path is
  stubbed for a future Forgejo mirror).

### Gotchas

- **`go mod tidy` on first scaffold**: the post-create hook runs it
  automatically. If you skip hooks (`--no-hooks`), run it manually before the
  first `just build` or imports will be unresolved.
- **`goreleaser` v2 config**: the v1 → v2 migration moved `archives[].format` to
  `archives[].formats` (slice). If you copy a pre-v2 `.goreleaser.yml` from
  elsewhere, validate with `goreleaser check`.
- **Distroless `nonroot` UID is 65532**. If the binary needs to write state,
  mount a writable volume — the rootfs is read-only.
- **goreleaser + Forgejo**: the v6 action defaults to GitHub-shaped release
  URLs. The `gitea_urls` block in `.goreleaser.yml` is commented by default —
  uncomment for Forgejo releases, and ensure `GITEA_TOKEN` is set in repo
  Secrets (PAT with `write:repository`).

### Implementation (DESIGN-0001 / IMPL-0001)

The service is being built out per `docs/design/0001-*.md` (Approved) and
`docs/impl/0001-*.md` (the phased plan). Conventions established as the build
progresses:

- **Build/lint/test entry points are `just`** — `just build`, `just test`,
  `just lint`, `just fmt`. There is no `Makefile`; any "`make lint`/`make fmt`"
  instruction maps to the corresponding `just` recipe.
- **docz parsing library** is pinned at `github.com/donaldgifford/docz v1.2.2`
  (a plain `require`, no `replace`; bumped from `v1.2.0` per IMPL-0008 for
  the json-tagged config structs — every config field now carries a `json`
  tag mirroring its `yaml` tag in name and `omitempty`, so the marshaled
  `config_snapshot` serves `.docz.yaml` key spellings; guarded by contract
  clause **R11**, mirroring upstream DESIGN-0008 R11. The crossed `v1.2.1`
  is config-only — docz's own api-block dogfood merge. The v1.1.0→v1.2.0
  bump added the `api:` block surface — `APIConfig` + `ErrInvalidAPIPath` +
  `docparse.Title`, contract clause R10 per IMPL-0007; the v1.0.0→v1.1.0
  bump added the changelog surface — `ChangelogConfig` + `ParseChangelog`,
  contract clause R6 per IMPL-0005; the v0.5.0→v1.0.0 bump was INV-0001).
  The v1.2.0 bump crossed
  `v1.1.1`, which flipped the built-in PLAN type default to disabled — the
  IMPL-0007 fleet check confirmed every fleet manifest pins an explicit
  `types:` block, so no repo was exposed. `pkg/doczcore/docparse` imports
  with the alias `doczparse` (joining `doczcfg`/`doczdoc`). As of `v1.0.0` docz **no longer pulls
  `spf13/viper` transitively** (it moved `config.Load` to `yaml.v3`); `viper`
  (`v1.21.0`) is now a **standalone direct dep** used only by `internal/config`,
  so DESIGN-0001 Decision 2's "reuse viper from docz" rationale no longer holds.
  Import its packages with the **aliases `doczcfg` / `doczdoc`** everywhere, per
  DESIGN-0001 — this repo has its own `internal/config`, so the alias keeps "the
  docz library" unambiguous at every call site even in files that don't
  currently import `internal/config`. This deliberately overrides the generic
  "avoid import aliases" style rule.
- **`internal/doczcontract`** is a runtime-code-free package whose tests guard
  the pinned docz surface (R1–R5). If a docz bump breaks the contract, it fails
  here, not deep in ingest. Re-run after any docz version change.
- **Tests use the standard-library `testing` package only** — no testify or
  other assertion deps. Prefer table-driven tests; positional struct literals
  are fine for tables with ≤3 fields.
- **Secrets use `config.Secret`** (a `string` type that redacts on slog, `%s`,
  `%v`, `%+v`, `%#v`). Read the real value only via `.Reveal()` — every unwrap
  is explicit and greppable. Never log a raw credential.
- **`internal/config` is env-only** (`spf13/viper` + `AutomaticEnv`, no config
  file). `Load()` returns one `ErrInvalidConfig` listing **all** problems.
  Config value receivers are heavy — `AuthEnabled` uses a pointer receiver on
  purpose (gocritic `hugeParam`); don't flip it back to a value receiver.

#### Phase progress

- **Phase 0 — Foundations: COMPLETE ✅** — docz `v0.5.0` pinned +
  `internal/doczcontract` smoke test; core deps pinned
  (chi/pgx/go-redis/asynq/meilisearch); `internal/config` (typed env config,
  validation, `Secret`, 100% cover); `main()` wiring (chi server, `/healthz`
  liveness, graceful shutdown, `-version`); `compose.yaml` + `.env.example`
  (Postgres/Redis/Meili, all healthy). All success criteria met; skeleton green
  (`build`/`test`/`lint`/`fmt`).
- **Phase 1 — Persistence: COMPLETE ✅** — initial goose migration (all 6
  tables + indexes, verified up/down); migrations embedded + `store.Migrate`/
  `MigrateDown` runner, `main()` auto-migrates on startup, `-migrate` flag for
  CI/ops (idempotent); sqlc config + query sets generated (typed access in
  package `store`; `just generate`/`generate-check`); `internal/store`
  `ReconcileRepo` tx (repo upsert + doc_types reconcile + documents
  content-hash gate + delete-absent, one tx) with plain-Go input DTOs + a
  `ReconcileResult` summary; `store.NewPool` + `main()` wires the runtime
  pgxpool/`Store` and serves `/readyz` (Postgres reachability via a narrow
  `readyChecker` interface, 200/503, unit-tested with a stub); testcontainers
  integration tests (`//go:build integration`, `just test-integration`) covering
  reconcile/gate/delete-absent against a real Postgres. All success criteria met.
  - Migrations run via goose's global-free
    `goose.NewProvider(DialectPostgres, db, migrations.FS)`; `db` is a
    `database/sql` conn from `sql.Open("pgx", …)` (pgx stdlib adapter),
    **separate from** the runtime pgxpool. `-migrate` applies + exits; normal
    startup applies then serves.
  - **Persistence conventions** (per go-architect): pgx v5 + pgxpool at runtime;
    goose runs migrations via the `pgx/v5/stdlib` `database/sql` adapter (never
    shares the pool); sqlc (`sql_package: pgx/v5`) generates typed queries into
    `internal/store`. Only JSONB is overridden (→ `json.RawMessage`); nullable
    TEXT/time/date stay as sqlc's `pgtype.Text`/`pgtype.Timestamptz`/`pgtype.Date`
    defaults (deviated from the architect's local `NullableText` — simpler; the
    `pgtype` values get mapped to clean DTOs at the Phase 2 boundary).
    `ReconcileRepo` is one tx:
    `pool.Begin` → `queries.WithTx(tx)` → deferred `Rollback` → explicit
    `Commit`; content-hash gate lives in Go, not SQL. Store constructor is
    `NewStore` (avoids colliding with sqlc's generated `New`). Integration tests
    behind `//go:build integration` with testcontainers.
  - `users` / `webhook_deliveries` Go code is **YAGNI until Phases 6/5** — the
    tables exist now; queries/methods come when first needed.
  - `cmd/docz-api` is the composition root — `run()`/`serve()` are covered by a
    live smoke test, not unit tests, so its statement coverage is low by design.
  - Local infra: `docker compose up -d` (Postgres 5432 / Redis 6379 / Meili
    7700); copy `.env.example` → `.env` for `just run`. CI uses testcontainers
    (later phases), not compose.
  - Core deps are still staged `// indirect` until their packages import them —
    **do not run a bare `go mod tidy`** while they're unused (it prunes them);
    use `go get`. `viper` is now direct (used by `internal/config`).
- **Phase 2 — Thin vertical slice: COMPLETE ✅** — synchronous hand-onboarded
  fetch→parse→upsert→serve, all 7 tasks done and all acceptance criteria proven
  by the `internal/e2e` integration test (five endpoints match DESIGN-0001, the
  custom type is addressable by name/prefix/alias, the content-hash gate makes an
  unchanged re-onboard a no-op, changed docs rewrite and removed docs delete).
  Architecture (per go-architect):
  - **`internal/ingest`** owns the consumer-side boundary: `RepoFetcher`
    interface (`Fetch(ctx, owner, name) (*RepoSnapshot, error)`) + `RepoSnapshot`
    {HeadSHA, DefaultBranch, ConfigYAML []byte, ChangelogMD []byte, ChangelogSHA,
    Blobs []BlobEntry{Path,GitSHA,Content}}. `Service` (`NewService(reconciler,
    RepoFetcher)`, `Run(ctx, installationID, owner, name) (ReconcileResult,
    error)`) does fetch → `loadConfig` → `Validate` → per-blob
    `doczdoc.ParseFrontmatter` (skip `ErrNoFrontmatter` with a warn, don't abort)
    → map → `store.ReconcileRepo`. Narrow `reconciler` interface (just
    `ReconcileRepo`).
  - **`loadConfig` bridge**: `doczcfg.Load` is disk-based, so write ONLY
    `.docz.yaml` to an `os.MkdirTemp` dir + point `HOME` at an empty temp dir
    (suppress the `$HOME/.docz.yaml` merge, like doczcontract tests), `Load("",
    tmp)`, deferred `RemoveAll`. Doc blobs never touch disk (byte-based
    `ParseFrontmatter`). `config_snapshot` stores the **raw `.docz.yaml` bytes**
    (`json.RawMessage(snap.ConfigYAML)`) — faithful to HEAD, no marshal risk.
  - **mapper** (`internal/ingest/mapper.go`): `TypeConfig`→`DocTypeInput`
    (Statuses/Aliases→`json.Marshal`), blob+`Frontmatter`→`DocumentInput`
    (DocID=`fm.ID`, Type=canonical name, ContentHash=`hex(sha256(raw))`,
    Created=`time.Parse("2006-01-02")` zero-on-empty, Status=`string(fm.Status)`).
  - **`internal/githubapp`**: concrete `Client` implementing `ingest.RepoFetcher`
    via `ghinstallation/v2` (App JWT→installation token transport, auto-refresh) +
    `google/go-github/v66`. `NewClient(appID, pemKey []byte, apiBase,
    installationID, httpClient)` — inject `*http.Client` (stub RoundTripper in
    tests). Fetch: get `.docz.yaml` blob first → parse for DocsDir/type dirs →
    resolve default-branch HEAD → recursive tree → filter to `.docz.yaml` +
    `docs_dir/<type.dir>/` via `doczdoc.IsDoczFile` → fetch blobs (base64) +
    optional root `CHANGELOG.md`.
  - **`internal/httpapi`**: chi `Handler.Mount(r, authzMiddleware)` at `/api/v1`.
    Response **DTOs** (own structs, map `pgtype` nullables → `string`/`YYYY-MM-DD`,
    never expose sqlc types). `{type}` resolved by `resolveType(types
    []store.DocType, input) (canonical, ok)` — pure match over name/id_prefix/
    aliases (no live doczcfg at serve time). Narrow `storeReader` interface.
  - **`internal/authorize`**: seam middleware. `Authorizer.Allowed(ctx, r)
    (AllowedRepos, error)`; `Middleware(a)` injects `AllowedRepos []int64` into
    ctx; `FromContext(ctx)` + `AllowedRepos.Contains(id)`. Phase 2 stub
    `AllReposAuthorizer` (narrow `repoLister`) returns all repo IDs; Phase 5 swaps
    impl only. Handlers use allowed-set for **existence hiding** (404 when a repo
    id isn't allowed).
  - **onboard**: `-onboard owner/name@installationID` flag on the binary (like
    `-migrate`); seeds installation+repo, runs one `Service.Run`. No admin HTTP
    surface in Phase 2.
  - **New store read methods/queries**: `ListRepos :many`, `ListDocumentsByType
    :many` (no `raw_md`), `GetDocumentByID :one` (with `raw_md`); reuse
    `ListDocTypes` for `GetDocTypesForRepo`.
  - **New deps**: `google/go-github/v66`, `bradleyfalzon/ghinstallation/v2` (add
    via `go get`, direct).
  - **Testing**: unit mapper tests (custom `frameworks`/`FW-0001` fixture +
    missing-frontmatter skip); hermetic e2e via an in-memory **fake
    `RepoFetcher`** at the ingest boundary (not a network VCR); `githubapp` token/
    tree-filter logic tested with a stub `http.RoundTripper` + `testdata/` JSON
    fixtures.
- **Phase 3 — Search: COMPLETE ✅** — Meilisearch indexer + faceted search, all
  5 tasks done and all success criteria proven. The headline criterion is
  proven end-to-end by `internal/e2e/search_integration_test.go`: onboarding a
  repo through the real ingest pipeline (real Postgres + real Meilisearch
  indexer) makes its docs searchable via `GET /api/v1/search`, returning hits,
  facet counts, and `<em>` snippets. Deletion removes from the index and the
  content-hash gate skips unchanged docs (proven by the search integration
  tests + ingest unit tests).
  Architecture (per go-architect):
  - **`internal/search`** wraps `meilisearch.ServiceManager` (meilisearch-go
    `v0.36.3`, now a direct dep). `Client` (`New(host, apiKey)`) satisfies the
    consumer-side `ingest.Indexer` and `httpapi.Searcher` interfaces. Boundary
    types in `types.go` keep meilisearch out of ingest/httpapi: `IndexDoc`
    (index schema, PK `id="<repo_id>:<doc_id>"`, `created` `YYYY-MM-DD`,
    `updated_at` Unix secs), `SearchParams` (`Query`, `AllowedRepoIDs` from the
    authorize seam, `Repo`/`Type`/`Status`/`Author` facet filters), `SearchHit`,
    `SearchResult` (matches DESIGN-0001 wire shape), `FacetMap`.
  - **`EnsureIndex(ctx)`** creates the `documents` index (PK `id`) + applies
    settings idempotently, called once at startup: searchable `title`,`body`
    (title first → higher relevance via the `attribute` ranking rule);
    filterable `repo`,`repo_id`,`type`,`status`,`author` (`repo_id` for the
    authorize `repo_id IN […]` filter); sortable `created`,`updated_at`. FIFO
    per-index task ordering means the enqueued create runs before the settings
    update (fresh index gets its PK); on an existing index the create task fails
    harmlessly (never waited on).
  - **meilisearch-go usage**: use the `…WithContext` API variants everywhere
    (`CreateIndexWithContext`, `UpdateSettingsWithContext`,
    `HealthWithContext`, `WaitForTaskWithContext`, later `AddDocumentsWithContext`
    /`DeleteDocumentsWithContext`/`SearchWithContext`) — `contextcheck` +
    revive `unused-parameter` require the ctx be threaded, not dropped.
    `WaitForTask` only errors on ctx-cancel/fetch-fail, so `waitTask` inspects
    `Task.Status != TaskStatusSucceeded` and surfaces `Task.Error.Message`.
    `Settings.SearchableAttributes` order sets relevance priority.
  - **content-hash-gated indexing** (task 2): the store reconcile is the single
    source of "what changed" — `ReconcileResult` now carries `UpsertedDocIDs`
    /`DeletedDocIDs`, populated by `reconcileDocuments` exactly where the
    content-hash gate decides. A new `GetDocumentsByIDs` store read (`= ANY
    (@doc_ids::text[])` → sqlc param `DocIds`, returns `[]Document`) fetches the
    changed rows. `ingest.Service` broadened its store interface to `repoStore`
    (`ReconcileRepo` + `GetDocumentsByIDs`) and gained a narrow `Indexer` dep
    (`IndexDocuments`/`DeleteDocuments`, satisfied by `*search.Client`). After
    the Postgres commit, `Run`→`indexSearch`→`syncIndex` deletes removed PKs
    then indexes upserted rows via `toIndexDoc` (`internal/ingest/indexmap.go`:
    PK `<repo_id>:<doc_id>`, repo label `owner/name`, `created` `YYYY-MM-DD`,
    `updated_at` Unix secs). **Indexing is best-effort**: an index failure logs
    at error and does NOT fail the ingest (Postgres is the source of truth; the
    next reconcile re-indexes — eventual consistency, Phase 4's queue makes it
    reliable). `NewService(st, fetcher, indexer)` — pass `nil` indexer to
    disable (e2e/unit paths that don't need Meili). `IndexDocuments`/
    `DeleteDocuments` wait on their tasks for read-after-write consistency.
  - **search endpoint** (task 3): `Client.Search(ctx, *SearchParams)
    SearchResult` uses `AttributesToCrop`+`AttributesToHighlight` on `body`
    (`<em>`/`</em>`, 40-word crop) so `_formatted.body` IS the snippet; facets
    `repo`/`type`/`status`/`author`. Decode hits via **`Hits.DecodeInto`** (NOT
    the deprecated `Hits.Decode` — staticcheck SA1019); it populates the nested
    `_formatted` struct field. `buildFilter` composes `repo_id IN [ids] AND
    field = "value"`, escaping `\` and `"` in user values; **nil** AllowedRepoIDs
    disables the repo scope (library/test convenience), **empty** slice matches
    nothing (`repo_id IN [-1]`, since ids are positive serials). Set
    `req.Filter` only when non-empty (empty-string filter is invalid). httpapi:
    `Searcher` seam + `NewHandlerWithSearch(st, s)`; `Mount` registers `GET
    /api/v1/search` only when a searcher is present (nil → route absent). The
    `searchDocs` handler injects `authorize.FromContext` as `AllowedRepoIDs`
    (the route is always behind `authorize.Middleware`, so the set is present).
    `main` wires `search.New(cfg.Meili…)` → `EnsureIndex` → both the onboard
    ingest indexer and `NewHandlerWithSearch`.
  - **/readyz multi-dep** (task 4): the single `readyChecker` interface is
    replaced by a `[]namedChecker` (`{name string; check func(ctx) error}`).
    `handleReadyz` runs each, reports a per-dependency status map (sorted keys
    via `json.Marshal` → deterministic body), and returns 503 if ANY fails so
    the body names the offender. `main` wires `postgres`→`st.Ping`,
    `meilisearch`→`searchClient.Health`. `newRouter` now takes `[]namedChecker`
    (pass `nil` when only `/healthz` matters). Body shape changed from
    `{"status":"ok"}` to `{"postgres":"ok","meilisearch":"ok"}`.
  - **integration tests** (task 5): `internal/search/search_integration_test.go`
    (`//go:build integration`) spins up `getmeili/meilisearch:v1.12` via
    testcontainers (generic container, `wait.ForHTTP("/health")` 200), shared
    across cases via TestMain. Covers index+search, facet counts, `<em>`
    snippet highlight, deletion, and the repo-scope filter seam.
  - **GOTCHA — Meilisearch document ids** allow only `[a-zA-Z0-9-_]`. The
    composite primary key uses `_` as the separator (`<repo_id>_<doc_id>`, e.g.
    `1_RFC-0001`), NOT the `:` DESIGN-0001 illustrates — a `:` id makes the add
    task fail with "Document identifier … is invalid". The PK is internal to the
    index and never appears in the search response, so this is a safe deviation;
    `repo_id` is numeric so the first `_` splits the two parts unambiguously.
  - **GOTCHA — `AttributesToRetrieve` is an explicit allow-list** (the
    `retrieveAttributes` package var in `internal/search/search.go`). A new
    index attribute must be added **there** as well as to `IndexDoc`, `rawHit`,
    and `decodeHits` — three drop sites, not two. Miss the retrieve list and
    Meilisearch simply omits the field from the response: the decoder yields a
    zero value, every faked-searcher unit test still passes, and only a real
    Meilisearch sees it. That is exactly how issue #34's plan came up short, so
    `TestSearchRetrievesDatedAttributes` now pins the list.
  - **GOTCHA — `sort` leads the ranking rules** (`rankingRules`, moved ahead of
    `words` from its Meilisearch default slot between `attribute` and
    `exactness`). That placement is what makes a requested sort a **total
    order** over the matches; at the default position it only breaks ties
    *within* relevance, so the most relevant hit leads no matter what the caller
    asked for. The move is safe because the rule is **inert on a request that
    carries no `sort`** — unsorted searches rank exactly as before. Don't move
    it back: `TestRankingRulesSortLeads` and `TestIntegrationSortBeatsRelevance`
    both fail by name (revert-drilled, IMPL-0010 Phase 4).
  - **GOTCHA — Meilisearch sorts an empty value LAST in both directions.** It
    treats `""` as absent rather than as the lexicographic minimum, so page
    records (no authored `created`) trail documents ascending *and* descending.
    DESIGN-0005 originally predicted the lexicographic reading and was
    corrected in place.
  - **GOTCHA — one reconcile stamps every row identically.** Postgres `now()`
    is `transaction_timestamp()` and `ReconcileRepo` is a single transaction,
    so every record one ingest touches shares `updated_at` to the microsecond —
    and a fresh database restamps a whole repo at onboard. A sort on
    `updated_at` alone would therefore be arbitrary within a repo, which is why
    `sortKeys` appends an implicit secondary key (`created`, matching
    direction). A test that seeds distinct stamps cannot see this;
    `TestIntegrationSecondarySortKey` seeds a colliding pair on purpose.

- **Phase 4 — Async ingestion: COMPLETE ✅** — ingest moved off the request
  path onto an asynq + Redis job queue, in-process with the API binary. All
  acceptance criteria proven by `internal/queue/queue_integration_test.go`
  (real Redis via testcontainers): a job drains through the worker, a
  five-trigger burst coalesces to one run, and shutdown drains an in-flight
  job. Task 5 (installation-token cache) is deferred to Phase 5, where the
  webhook/App-auth work lives.
  Architecture (per go-architect):
  - **`internal/queue`** owns the job contract and both ends of the queue.
    `IngestJob{InstallationID, Owner, Name, Reason}` carries **no HeadSHA** —
    the worker refetches HEAD at process time, so a debounced burst always
    ingests the latest commit ("latest-HEAD-wins" is free). `job.go` marshals
    via `encoding/json`; `repoLabel()` = `owner/name` is the coalesce key.
  - **`Client`** (`client.go`) wraps both `asynq.Client` (enqueue) and a
    go-redis client (`Ping` for `/readyz`), parsing the one `redisURL` for
    both. `EnqueueIngest` sets **`TaskID = "ingest:" + owner/name`** (dedup key)
    + **`ProcessIn(debounce)`** (schedules into the future so repeat triggers
    within the window collapse onto the pending task) + `MaxRetry(5)` +
    `Retention(24h)`. `asynq.ErrTaskIDConflict`/`ErrDuplicateTask` are the
    coalesce signal → treated as success (nil error). `Enqueuer` interface +
    `var _ Enqueuer = (*Client)(nil)` so `main`/onboard depend on the seam.
    `Close()` joins the asynq + redis close errors via `errors.Join`.
  - **`Worker`** (`worker.go`) wraps `asynq.Server` (holds a pointer — must not
    be copied). `NewWorker(redisURL, concurrency, ing)`; `Start()` is
    non-blocking (registers `handleIngest` on a `ServeMux`, calls
    `srv.Start`), `Shutdown()` drains in-flight handlers. The `Ingestor`
    interface (`Run(ctx, installationID, owner, name) (store.ReconcileResult,
    error)`) matches `ingest.Service.Run` and is declared consumer-side so the
    worker tests with a fake. `handleIngest`: a malformed payload is unfixable
    → `asynq.SkipRetry`; any ingest error is returned so asynq retries with
    backoff (the content-hash gate makes retries idempotent). `isFailure`
    excludes `context.Canceled` so a shutdown re-queues the task instead of
    burning a retry.
  - **GOTCHA — `DelayedTaskCheckInterval` defaults to 5s.** asynq forwards
    scheduled (`ProcessIn`) and retry tasks to the pending queue only every 5s
    by default, so a debounced job could sit up to 5s past its window. Set to
    **`1 * time.Second`** in the worker `Config` — snappier ingestion in prod
    and it makes the debounce/drain integration tests pass within their waits.
  - **`ingestRunner`** (`cmd/docz-api/runner.go`) is the production `Ingestor`.
    It builds a **per-installation** `githubapp` client for each job (from
    `config.GitHubConfig` — `AppID`, `PrivateKey.Reveal()`, `APIBase`,
    `installationID`), then runs `ingest.NewService(store, ghClient,
    indexer).Run(...)`. This adapter avoids a `RepoFetcher.Fetch(installationID)`
    interface refactor: one worker serves every installation, building the
    client per job (cheap — ghinstallation caches the JWT; jobs are infrequent).
  - **in-process worker + wiring** (`cmd/docz-api/main.go`): `run()` builds one
    `queue.NewClient(cfg.Store.RedisURL, cfg.Ingest.Debounce)`. The
    `-onboard` path calls `runOnboard(ctx, st, enq queue.Enqueuer, spec)` —
    `UpsertInstallation` (synchronous) then `EnqueueIngest` (Reason `"onboard"`).
    The serve path builds the `ingestRunner`, `queue.NewWorker(..,
    workerConcurrency=2, ..)`, `worker.Start()`, and a router with **three**
    `namedChecker`s (`postgres`→`st.Ping`, `meilisearch`→`search.Health`,
    `redis`→`queueClient.Ping`). `serveWithWorker` drains in order: HTTP
    first (stop accepting requests/enqueues) → `worker.Shutdown()` (drain
    in-flight) → `closeQueueClient` — so no enqueue races the drain.

- **Phase 5 — GitHub App onboarding + webhooks: COMPLETE ✅** — install-driven
  onboarding and HMAC-verified webhooks now drive ingestion; the `-onboard` flag
  remains as a manual fallback. All acceptance criteria met; the new store SQL
  and index purge are proven by integration tests (real Postgres + Meilisearch).
  Architecture (per go-architect):
  - **`internal/webhook`** owns HMAC verification, event routing, and delivery
    idempotency only — business logic is delegated through unexported consumer
    interfaces (`webhookStore`/`enqueuer`/`indexPurger`, satisfied by
    `*store.Store`/`*queue.Client`/`*search.Client`). `Handler` is an
    `http.Handler`; `New(secret, store, enq, purger)` (nil purger disables index
    cleanup in tests). `ServeHTTP` flow: read raw body **once** (HMAC is over the
    exact bytes GitHub signed) with a 5 MiB `MaxBytesReader` cap → `verifyHMAC`
    (`hmac.Equal` constant-time, fails closed on bad/missing/malformed
    `X-Hub-Signature-256`) → `401` on mismatch **before any work** → dedupe via
    `RecordDelivery(X-GitHub-Delivery)` (`200` no-op on replay) → `ParseWebHook`
    (`go-github` v88 typed events; first arg is the `X-GitHub-Event` header, NOT
    the media type) → `route` → `202`.
  - **payloads via `go-github`** (`github.ParseWebHook`): `InstallationEvent`,
    `InstallationRepositoriesEvent`, `PushEvent`, `ReleaseEvent`. Onboarding
    reads the repo list **from the event payload** (`ev.Repositories` /
    `RepositoriesAdded`), deriving `owner`/`name` by splitting `full_name` — no
    `GET /installation/repositories` call (payload is complete at homelab scale).
    App auth is reused via the existing per-installation `githubapp.Client` that
    the worker builds per job.
  - **onboarding / offboarding** (`events.go`): `installation` created → upsert
    installation + enqueue an ingest per repo (Reason `onboard`); `deleted` →
    `ListRepoIDsByInstallation` (collect ids **before** delete) →
    `DeleteInstallation` (CASCADE wipes repos/doc_types/documents) → purge each
    repo from Meili by `repo_id` filter (best-effort). `installation_repositories`
    added → upsert + enqueue (Reason `repo_added`); removed → `DeleteRepo` +
    purge. `.docz.yaml` detection is left to the ingest worker (a repo with no
    manifest fails `Fetch` and is logged; no pre-check / `configured` flag).
  - **push handling**: `shouldIngest` requires the default branch
    (`refs/heads/<default_branch>`) AND a changed path (union of every commit's
    added/modified/removed — not just `head_commit`) equal to `.docz.yaml` or
    under `docs_dir/`. On a match it enqueues a **full** re-ingest (Reason
    `push`, no HeadSHA — worker refetches HEAD). The reconcile's content-hash
    gate + desired-state replace already do diff/delete/`doc_types` reconcile
    idempotently, so full re-ingest is correct; **narrow blob fetches are
    deferred** (a fetch-cost optimization only). A push for an un-onboarded repo
    (GetRepo → `ErrNoRows`) is skipped. `release` is **log-only** (`logRelease`;
    versions feature deferred).
  - **idempotency store surface** (`store.go` + `queries/deliveries.sql`):
    `RecordDelivery(ctx, id, event) (isNew bool, err error)` backed by `INSERT …
    ON CONFLICT (delivery_id) DO NOTHING RETURNING` — a conflicting insert
    returns no row (`pgx.ErrNoRows`) → mapped to `isNew=false`. Offboard surface:
    `DeleteInstallation`, `DeleteRepo(owner,name) (id, err)` (returns id for the
    index purge; `ErrNoRows` = already absent), `ListRepoIDsByInstallation`.
    `search.Client.DeleteRepoDocuments(repoID)` uses
    `DeleteDocumentsByFilterWithContext("repo_id = <id>")`.
  - **wiring** (`main.go`): `POST /webhooks/github` mounts on the **root** router
    (inherits `RequestID`/`Recoverer`, but NOT `/api/v1`'s `authorize`
    middleware — HMAC is the auth). `webhook.New([]byte(cfg.GitHub.WebhookSecret
    .Reveal()), st, queueClient, searchClient)`.
  - **GOTCHA — `github.ParseWebHook(event, body)`**: the first arg is the
    `X-GitHub-Event` header value (`"push"`, `"installation"`, …), NOT the
    Content-Type media type. `github.ValidatePayload`/`ValidateSignature` exist
    but we hand-roll `verifyHMAC` to satisfy the "constant-time via `hmac.Equal`"
    requirement and keep the taint out of a shared helper.
  - **GOTCHA — gosec G706** (log injection) fires on `slog` calls that log
    webhook payload fields. It is a false positive for structured logging (slog
    escapes attribute values); excluded globally in `.golangci.yml` with a
    justification, alongside a `_test.go` `unparam` relaxation (test helpers keep
    intent-documenting params).

- **Phase 6 — Authentication (pluggable providers + Redis sessions): COMPLETE
  ✅** — site users log in via one `Provider` abstraction (GitHub default, plus
  discovery-driven Okta/Keycloak) and carry an opaque Redis-backed session.
  Authorization stays the pass-through seam (Decision 10) — now keyed off a real
  identity. Architecture:
  - **`internal/auth`** — `Provider` interface (`Name`/`AuthCodeURL`/`Exchange`)
    returning an `Identity{Provider, Subject, Email, Login, Groups}`.
    `GitHubProvider` (OAuth via `golang.org/x/oauth2`; `Exchange` pulls the user
    with go-github and requires a **primary + verified** email). `OIDCProvider`
    backs **both** Okta and Keycloak — they differ only by issuer/credentials;
    discovery (`oidc.NewProvider`) runs **at startup** under a bounded context so
    a bad issuer fails the boot, not the first login, and `Exchange` verifies the
    `id_token` (JWKS signature + audience + issuer + expiry via go-oidc defaults)
    before reading claims, dropping an email the issuer marks
    `email_verified:false`. `Registry` is a name→provider map with sorted
    `Names()`.
  - **GOTCHA — requested OIDC scopes are per-provider config, not hardcoded**
    (`{OKTA,KEYCLOAK}_SCOPES`, default `profile,email`). An issuer rejects the
    **whole** authorize request with `invalid_scope` when the client is not
    assigned a requested scope — so login breaks entirely rather than
    degrading, and a scope no deployment universally has must never be
    hardcoded. `groups` used to be, which blocked any Okta/Keycloak client
    without it (stock Keycloak has no `groups` client scope; Okta needs one
    defined on the authorization server). It is now opt-in. `openid` is always
    prepended by `withOpenID` and de-duplicated, so config can neither drop
    the scope OIDC mandates nor request it twice. The scopes are per-provider
    because one issuer may publish groups while the other cannot.
  - **`Identity.Groups` is passthrough only.** It is read from the `id_token`
    (unconditionally — a claim mapper can supply it with no scope), carried in
    the Redis session, and served on `GET /api/v1/auth/session` as an optional
    `groups` array for docz-site. **Nothing in docz-api reads it for an access
    decision** — `internal/authorize` has zero references — and it is never
    persisted (the `users` table has no groups column). So dropping the scope
    costs no server-side behavior.
  - **stateless CSRF via signed state** (`auth/state.go`): the OAuth `state` is
    `base64url(payload).hex(HMAC-SHA256(secret, payload))` where payload carries
    `{Provider, Nonce, ExpiresAt}` (5-min TTL). `VerifyState` is constant-time
    (`hmac.Equal`), fails closed on any tamper/expiry, and is checked **before**
    the payload is decoded. The signing secret is `SESSION_SECRET`. **No
    server-side state row** — the provider is recovered from the verified state.
  - **`internal/session`** — opaque **32-byte `crypto/rand`** session id (the id
    is the *only* credential; no identity is encoded in it). `sess:<id>` → JSON
    identity in Redis with a `SESSION_TTL` expiry. `Issue`/`Lookup`/`Revoke`
    (`redis.Nil` → `ErrSessionNotFound` via `errors.Is`). `SetCookie` is
    `HttpOnly` + `SameSite=Lax` (Lax, so the cookie survives the top-level
    provider redirect back to `/auth/callback`) + `Secure`-when-https +
    `Path:/`; `ClearCookie` mirrors it with `MaxAge:-1`. The store owns its
    **own** Redis client (separate from the queue's), closed via `closeSession`.
    `Middleware` resolves the cookie → `Lookup` → injects `Session` into the
    request context (or `401`); `FromContext` reads it back.
  - **`internal/authhttp`** — four endpoints over unexported consumer interfaces
    (`userUpserter`/`sessionStore`, satisfied by `*store.Store`/`*session.Store`).
    `MountPublic`: `GET /auth/login` (sign state → redirect to `AuthCodeURL`),
    `GET /auth/callback` (verify state → `Exchange` → `UpsertUser` → `Issue` →
    `SetCookie` → redirect `/`). `MountAPI` (behind the gate): `GET
    /api/v1/auth/session` (current user or `401`), `POST /api/v1/auth/logout`
    (`Revoke` + `ClearCookie`, idempotent). `UpsertUser` (`users.sql`) is `INSERT
    … ON CONFLICT (provider,subject) DO UPDATE`.
  - **the `/api/v1` gate** (`main.go` `runServer`): `session.Middleware` composed
    **over** `authorize.Middleware` — session runs **first** so authorization
    resolves behind a real identity. `authHandler.MountAPI` is threaded into the
    gated group via `httpapi.Handler.Mount(r, gate, extras…)`; `MountPublic` sits
    on the root router (login has no session yet; callback is authed by its
    state). The webhook receiver stays outside both (HMAC is its auth).
  - **config**: `AUTH_REDIRECT_BASE` (required, trailing-slash-trimmed) builds
    each provider's absolute `/auth/callback` `redirect_uri`; `cookiesSecure`
    keys `Secure` off its `https` scheme. Providers built by
    `cmd/docz-api/auth.go` `buildAuthProviders` (OIDC discovery bounded by
    `oidcDiscoveryTimeout=15s`).
  - **GOTCHA — funlen on `run()`**: wiring the auth stack + worker + router
    pushed `run()` over the 50-statement `funlen` limit; the serve path is
    extracted into `runServer(cfg, st, searchClient, queueClient)` (the
    `-onboard` early-return stays in `run()`).
  - **GOTCHA — `Issue(*auth.Identity)`**: `auth.Identity` is ~88 B, so gocritic
    `hugeParam` flags passing it by value; `session.Store.Issue` and the
    `sessionStore` interface take a **pointer**. Same for `session.Middleware`
    fakes (pointer receivers, `&fakeLookuper{}`).
  - **new deps**: `golang.org/x/oauth2` + `github.com/coreos/go-oidc/v3` (direct);
    `github.com/go-jose/go-jose/v4` (indirect, go-oidc's transitive). Promote via
    the go.mod direct-require block + `go mod edit -fmt`, never a bare
    `go mod tidy`.
  - **deferred (documented)**: OIDC `nonce` binding (the signed state already
    guards the code flow's CSRF; `statePayload` already carries a nonce, so it's a
    cheap hardening follow-up).

- **Phase 7 — Hardening, deploy, contract, observability: COMPLETE ✅** — the
  docz `v0.5.0` pin is confirmed (no `replace`); the read+search wire contract is
  frozen by golden fixtures (`internal/httpapi/contract_test.go`,
  `testdata/contract/*.json`, `-update` to regenerate); the distroless image
  builds/runs and `deploy/` is the reference stack; the full OQ 8 observability
  stack is wired; the error/TODO audit is clean; coverage reviewed (78% internal
  aggregate, 7/13 packages ≥80%; sub-80% is network-bound provider/fetch code +
  the cross-covered store + the composition root, no CI coverage gate).
  Observability architecture (per go-architect):
  - **`internal/telemetry`** is the single observability package.
    `Setup(ctx, Config) (shutdown, error)` installs the **global** W3C
    `TextMapPropagator` (TraceContext + Baggage) always, and — only when
    `OTEL_EXPORTER_OTLP_ENDPOINT` is set — a batching `sdktrace.TracerProvider`
    exporting over **OTLP/HTTP** (`otlptracehttp`, `WithInsecure`, host:port →
    `/v1/traces` on :4318). Empty endpoint ⇒ the global no-op tracer stays: spans
    are created but not exported, so a collector-less homelab pays ~zero overhead
    and needs no config. `Setup` does **no** network I/O (the HTTP exporter
    connects lazily on first export), so it never blocks startup.
  - **decision — metrics via `prometheus/client_golang`, OTel for traces only.**
    Metrics are package-level `promauto` instruments on the **default registry**
    (idiomatic; also exposes Go-runtime/process collectors). RED for HTTP
    (`docz_api_http_requests_total`, `..._request_duration_seconds`) and ingest
    (`docz_api_ingest_jobs_total{reason,status}`, `..._job_duration_seconds` with
    **wide buckets** up to 120s for slow GitHub-bound ingests). `ObserveIngest` is
    the one exported metric helper (called by the queue worker).
  - **HTTP middleware** (mounted in `newRouter`, order
    `RequestID → RequestLogger → Instrument → Recoverer` so a recovered panic is
    still logged + metered as 500): `RequestLogger` emits one structured slog line
    per request; `Instrument` starts a server span and records HTTP metrics. Both
    **skip** `/healthz`, `/readyz`, `/metrics` (`skipPaths`). The span name and
    the metric `route` label use chi's **matched route template**
    (`chi.RouteContext(r.Context()).RoutePattern()`, read **after**
    `next.ServeHTTP`), never the expanded URL — bounded cardinality; falls back to
    `"unmatched"`. 5xx ⇒ `span.SetStatus(codes.Error, …)`.
  - **trace propagation across the asynq boundary** (`internal/queue/job.go`):
    `IngestJob` carries `traceparent`/`tracestate` (`json:",omitempty"`).
    `injectTrace` (called in `EnqueueIngest` before marshal) writes the active
    span's W3C context via `otel.GetTextMapPropagator().Inject` into a
    `propagation.MapCarrier`; `extractTrace` (called in `handleIngest`) rebuilds
    the remote parent. The worker starts a `queue.ingest` **consumer** span; the
    ingest pipeline starts `ingest.run` with `ingest.fetch` / `ingest.reconcile`
    / `ingest.index` child spans. So webhook request → enqueue → worker → ingest
    is **one trace**.
  - **GOTCHA — global tracer/propagator are safe before `Setup`**: package-level
    `var tracer = otel.Tracer(...)` in `queue`/`ingest` and
    `otel.GetTextMapPropagator()` return **delegating** globals that retroactively
    bind to the real provider when `Setup` calls `SetTracerProvider` /
    `SetTextMapPropagator`. Never nil, so tests and the `-onboard` path (no
    `Setup`) just no-op.
  - **`/metrics`** is mounted in `runServer` (not `newRouter`) gated on
    `cfg.Telemetry.MetricsEnabled`, alongside the probes and **outside** the auth
    gate (pull-based, scraped internally). `run()` calls `telemetry.Setup` right
    after the logger and `defer shutdownTelemetry(...)` (bounded by
    `shutdownTimeout`), covering every return path.
  - **new deps** (all otel v1.43.0 line): `go.opentelemetry.io/otel/sdk`,
    `.../exporters/otlp/otlptrace/otlptracehttp`, `.../otel` + `.../otel/trace`
    (direct); `github.com/prometheus/client_golang`. `grpc` rides along
    transitively via `proto/otlp` even with the HTTP exporter. **Offline `go mod
    tidy` fails** (some dependencies' *test* deps aren't cached); settle go.sum
    with targeted `GOPROXY=off go get <pkg>@<ver>` instead.
  - **deploy** (`deploy/`): `deploy/compose.yaml` is the full reference stack
    (service + Postgres + Redis + Meili, health-gated, only `:8080` published);
    the repo-root `compose.yaml` stays deps-only for `just run`. Config is an
    env store (gitignored `deploy/.env.production`, template committed); the
    GitHub App private key is a mounted Docker **secret** referenced by path
    (`GITHUB_APP_PRIVATE_KEY` accepts a path or a PEM body). The distroless image
    has **no shell**, so there is no in-container healthcheck for the service —
    orchestrators probe `/healthz` + `/readyz` over HTTP.

### OpenAPI contract (DESIGN-0002 / IMPL-0002)

A machine-readable OpenAPI 3.1 contract for the `/api/v1` surface, kept honest by
an in-process `kin-openapi` test and served at `GET /openapi.yaml`. Built per
`docs/design/0002-*.md` (**Implemented**) and `docs/impl/0002-*.md` (the phased
plan) — **all four phases COMPLETE ✅** (spec foundation + read/search contract
test; full auth/webhook surface + golden fixtures retired; serve `/openapi.yaml`;
version + document consumption). `api/README.md` is the consumer-facing guide.

- **Spec at `api/openapi.yaml`** (OAS 3.1.0), hand-authored (not generated). It is
  embedded by the tiny **`api` package** (`api/spec.go`: `//go:embed openapi.yaml`
  → `var Spec []byte`) so both the runtime server and the contract test consume
  the **same bytes**. Root-level `api/` is deliberate (fleet convention + the file
  the docz-site vendors); it is not under `internal/` because the wall governs
  Go-import visibility, and this artifact is consumed by file path/HTTP.
  It covers the **whole consumer-facing surface**: the read/search `/api/v1`
  routes, the auth endpoints (`/api/v1/auth/session` + `/logout`, public
  `/auth/login` + `/auth/callback` 302s), and the HMAC `/webhooks/github`
  receiver. Operational routes (`/openapi.yaml`, `/healthz`, `/readyz`,
  `/metrics`) stay **out** (OQ-6a).
- **Contract test** `internal/httpapi/openapi_contract_test.go` (`kin-openapi
  v0.135.0`, a **direct test-path dep**) — the **sole** wire-contract gate (the
  byte-frozen golden fixtures were retired at parity in Phase 2):
  `loadContractSpec` (`LoadFromData(api.Spec)` → `doc.Validate` →
  `gorillamux.NewRouter`) + `buildContractHandler` (wires the **full** stack
  exactly as `runServer` — read/search + gated auth behind the real
  `session.Middleware` ∘ `authorize.Middleware`, the public redirects, and the
  webhook — with in-package fakes) + `validateRoundTrip` (`openapi3filter`
  request + response, `MultiError`, snapshots the body so it survives both). **No
  build tag** — it rides `just test` / CI `Test Go`. Security is a no-op via
  `openapi3filter.NoopAuthenticationFunc` (the middleware runs for real with a
  fake session; the test asserts schemas, not the auth mechanism).
- **Schemas mirror the DTOs 1:1** (`internal/httpapi/dto.go`,
  `internal/search/types.go`, `internal/authhttp/handler.go`'s `sessionDTO`) with
  **`additionalProperties: false`** everywhere — that strictness is the drift
  detector (an added/renamed field fails the test). Nullable columns are empty
  strings (never `null`); JSONB arrays are `[]`. The read/search error envelope
  stays `{"error": string}` and list responses stay envelope objects
  (`{"repos":[…]}`); the **webhook + logout** use a separate `{"status": string}`
  envelope (`StatusResponse`). RFC 7807 + bare arrays are deferred (FU-1/FU-2).
- **Security model:** the top-level default is `sessionCookie` (apiKey in the
  `docz_session` cookie); `/api/v1/*` inherits it, and the three public routes
  (`login`, `callback`, `githubWebhook`) override with `security: []`. The webhook
  HMAC-SHA256 scheme is documented in the op `description` (OpenAPI has no
  first-class HMAC-body scheme).
- **Served + versioned:** `GET /openapi.yaml` serves `api.Spec` verbatim
  (`handleOpenAPISpec` in `newRouter`, `application/yaml`, **public** — outside
  the `/api/v1` gate, so the docz-site can fetch it without a session). No `/docs`
  UI (OQ-3d). `info.version` is **SemVer from `1.0.0`** (OQ-5a), bumped by hand on
  any specced wire change (patch = editorial, minor = additive, major = breaking),
  independent of the binary version — the consumer-pin signal. **Currently
  `1.1.0`** (IMPL-0003 added `getRepoIndex`). The docz-site
  vendors the file (or fetches the served spec at a pinned version) and generates
  a typed client; see `api/README.md`.
- **Spec lint/format (OQ-7b):** `just lint-openapi` runs **`vacuum`** (`aqua:
  daveshanley/vacuum`, pinned in `mise.toml`) against `api/vacuum-ruleset.yaml`
  (`-n warn`, fails on warnings) + `yamlfmt -lint`; `just fmt` yamlfmt-
  canonicalizes the spec. The ruleset disables **`camel-case-properties`** (the
  wire contract is snake_case by design) plus two over-strict-for-us rules, with
  rationale in-file; the spec scores **100/100**. CI's Lint job runs
  `just lint-openapi` behind a mise step.
- **Dep-settling gotcha:** `go get kin-openapi@v0.135.0` alone does **not** pull
  the `openapi3`/`openapi3filter`/`routers/gorillamux` subpackages' transitive
  `go.sum` entries (nothing imports them yet). Settle them with targeted `go get
  <subpkg>@v0.135.0` (or a `go mod tidy` once the harness imports them, which also
  promotes `kin-openapi` to a direct require).

### Repo index endpoint (DESIGN-0003 / IMPL-0003)

The docz-site repo home: `docs_dir/index.md` (docz's wiki landing page,
`doczcfg.WikiIndexName`) is fetched at ingest, cached on the repo row, and
served at **`GET /api/v1/repos/{owner}/{name}/index`** as
`{repo, index_md, index_sha}` (spec `1.1.0`, additive). Built per
`docs/design/0003-*.md` (all six design OQs = `a`) and `docs/impl/0003-*.md`
(all five impl OQs = `a`) — **all four phases COMPLETE ✅**.

- **Persistence:** `repos.index_md` / `repos.index_sha` (nullable TEXT,
  migration `20260710000000_add_repo_index.sql`, mirrors the
  `changelog_md`/`changelog_sha` precedent); `UpsertRepo` writes both;
  `RepoInput.IndexMD/IndexSHA` map through `textOrNull`.
  **GOTCHA — presence keys off `index_sha`:** `textOrNull("")` is NULL, so an
  empty-but-present `index.md` stores a NULL body with a **valid sha** — the
  handler gates on `IndexSha.Valid` and `nullText` yields the `""` body, which
  is exactly DESIGN OQ-3a's "empty file ⇒ 200 + empty string; absent ⇒ 404".
- **Fetch (githubapp):** `docsDirHint(configYAML)` does a fetch-scoped
  one-field `yaml.Unmarshal` of `docs_dir` (trailing-`/` trimmed; default
  `doczcfg.DefaultConfig().DocsDir`; the authoritative parse stays in ingest's
  `loadConfig`, so a malformed config still fails there). `findBlobSHA` looks
  up `docsDirHint(...)+"/"+doczcfg.WikiIndexName` in the already-listed
  recursive tree (exact path, blob type) — **at most one extra blob request,
  zero when absent**. `gopkg.in/yaml.v3` is a **direct** require (same dialect
  docz uses — no drift; promoted via the require block, never a bare tidy).
  `IsDoczFile` requires leading digits so `index.md` never collides with doc
  ingest; the webhook `shouldIngest` already re-ingests on `docs_dir/` pushes,
  so index refresh needed **no webhook change**.
- **Serve (httpapi):** `getRepoIndex` = `resolveRepo` (existence hiding) →
  `IndexSha.Valid` check → 404 `{"error":"index not found"}` or 200
  `repoIndexDTO`. Contract-tested happy + 404 via the second bare fixture repo
  `acme/bare` (OQ-2a; the fixture growth shifted the `list repos` count and
  the search allowed-set assertions).
- **Rollout:** natural refresh only (DESIGN OQ-4a) — pre-existing repos 404
  until their next ingest; note in `deploy/README.md`. No size cap on the
  cached body (capless changelog precedent, OQ-4a).
- **Proof:** `TestE2ERepoIndexServeAndRemoval` (real Postgres) covers ingest →
  serve → removal-at-HEAD → 404; store round-trip + migration up/down live in
  the store integration tests.

### Changelog endpoint (INV-0005 / IMPL-0005)

The repo changelog became **opt-in and served**: `.docz.yaml`'s `changelog:`
block (docz **v1.1.0**, upstream DESIGN-0010) drives the fetch, and the cached
raw markdown is served at **`GET /api/v1/repos/{owner}/{name}/changelog`**
(spec `1.2.0`, additive). Built per `docs/investigation/0005-*.md` (Concluded,
all OQs `a`) and `docs/impl/0005-*.md` — **all five phases COMPLETE ✅**. It
deliberately mirrors the `index.md` slice (DESIGN-0003 / IMPL-0003); only the
deltas are noted here.

- **docz pin is `v1.1.0`** and `internal/doczcontract` gained **clause R6**
  (`internal/doczcontract/changelog_test.go` + the frozen
  `testdata/changelog_fleet.md`): `ChangelogConfig` defaults/partial-merge/
  `./`-normalization, **enabled-only validation** (a dormant block with a bad
  path must NOT fail `Validate`), unknown-sibling-key tolerance, and
  `ParseChangelog`'s parse shape + `ErrNoVersions`. `ParseChangelog` has **no
  runtime caller** — it is pinned now so feature 2 (per-doc backlinks) starts
  on a frozen surface.
- **Opt-in is desired state, not a cache.** A disabled or absent block means
  ingest maps an empty `ChangelogFile`/`MD`/`SHA`, so the reconcile **nulls all
  three columns** and the endpoint 404s. Pre-config-era cached changelogs are
  cleared on the next ingest of a repo that never opts in (nothing served them,
  so nothing regresses).
- **`repos.changelog_file`** (migration `20260803000000_add_repo_changelog_file`)
  stores the resolved path. **Why a column and not a re-parse:** the changelog
  lives **outside `docs_dir`**, so `shouldIngest`'s prefix check would never
  match it and a release's changelog-sync push (which touches nothing else)
  would leave the served copy a release behind. `handlePush` already holds the
  full repo row, so the webhook match is one exact-path comparison.
  Deliberately one plain column — the shape is expected to generalize to other
  API-consumed files later, so no bespoke abstraction was introduced.
- **`changelogHint`** (`internal/githubapp`) is the `docsDirHint` twin:
  fetch-scoped one-field unmarshal, docz defaults on malformed yaml, `./`
  trimmed. `classifyTree` **no longer recognizes** `CHANGELOG.md` at all —
  the path is resolved by `findBlobSHA` like `index.md`, so a repo that never
  opts in costs **zero** extra requests. Subpaths
  (`charts/<name>/CHANGELOG.md`) work by construction.
- **`ingest.changelogFile(cfg)`** maps the **authoritative** post-`Load` value
  (normalized + validated), never the hint. An invalid `changelog.file` on an
  enabled block fails `Validate` → fails the whole ingest, like any other
  malformed `.docz.yaml` (IMPL OQ-2a — one error path, no partial-ingest mode).
- **Serve:** `getRepoChangelog` gates on `ChangelogSha.Valid`, so an
  empty-but-present file is 200 with `""` and absence is 404 — the same
  `textOrNull` presence-keys-off-the-sha gotcha as the index pair.
- **Proof:** `TestE2ERepoChangelogServeAndDisable` (real Postgres) covers
  ingest → serve → disable-at-HEAD → 404; `TestReconcileRepoChangelogTriple`
  round-trips the columns; `TestFetchRepoChangelog` proves the no-fetch cases
  by **withholding the blob** (the stub 404s on an unfetched sha).

### Pages endpoints + search source facet (DESIGN-0004 / IMPL-0007)

> **Amended by IMPL-0009 (issue #28):** enabled **type dirs publish
> nothing** — rule 4's type-dir `README.md` carve-out is gone. docz-site
> synthesizes its own type page at `/:owner/:repo/:type` from the live
> document list, so publishing the docz-generated index table duplicated
> that surface at `/pages/<dir>` (repo nav, search hit, stale body). The
> README skips **silently** — `docz update` writes it there on every run,
> so a Warn would fire on correct configuration. No schema or spec change;
> reconcile's desired-state delete retires existing rows on each repo's
> next ingest. **Adjacent noise, since fixed:** `buildDocuments` matches by
> type dir rather than `IsDoczFile`, so an enabled `api:` block (which
> widens the fetch to every `.md` under `docs_dir`) sent that same README
> through `ParseFrontmatter` and logged `skipping doc without frontmatter`
> on every ingest. Its `ErrNoFrontmatter` warn is now gated on
> `doczdoc.IsDoczFile(path.Base(...))`: an unparseable **document** is a
> real mistake worth surfacing, while a README or page candidate is not —
> and it kills the double-report, since `buildPages` already names genuine
> strays. **Logging only** — which blobs become documents is unchanged, so
> a non-convention file carrying valid frontmatter still ingests exactly
> as before.

docz v1.2.0's `api:` block publishes a repo's **non-docz markdown** as pages:
fetched at ingest, reconciled into `repo_pages`, served at
`GET /api/v1/repos/{owner}/{name}/pages[/{path}]` (spec `1.3.0`), and
searchable under a `source` facet (spec `1.4.0`). Built per
`docs/design/0004-*.md` (grounded by INV-0008) and `docs/impl/0007-*.md`.
Per IMPL OQ-1a it shipped as **two PRs**: PR-1 = Phases 1–5 (pages surface),
PR-2 = Phases 6–7 (search facet + close-out). Deltas from the
index/changelog precedents:

- **Contract clause R10** (`internal/doczcontract/api_test.go`):
  `Config.API{Enabled, LandingPage, Exclude, AdditionalDocs}`,
  `ErrInvalidAPIPath`, `APILandingFileName`, normalization-at-Load (landing
  backfill `docs_dir/index.md` **only when enabled**; dormant blocks are
  never validated or backfilled), and `docparse.Title` (first H1, ATX +
  setext, frontmatter skipped — frontmatter `title:` deliberately unread).
  Import alias **`doczparse`** joins `doczcfg`/`doczdoc`.
- **Dormancy is byte-for-byte.** A repo without an enabled block fetches
  exactly what v1.1.0 fetched (proven by withheld-blob stubs: the test stub
  404s any blob request the dormant path shouldn't make). `classifyTree`
  widens to every `.md` under `docs_dir` **only when** `apiHint` reports
  enabled; `additional_docs` fetches are per-entry, skipped for non-`.md`
  entries without a request.
- **`ingest.buildPages`** is the six-rule classifier (DESIGN-0004): landing
  skip → over-fetch guard → templates/`api.exclude` pruning → type-dir
  discrimination (**enabled type dirs publish nothing** per IMPL-0009 —
  `IsDoczFile` matches and the dir's own `README.md` stay silent; strays warn
  + skip) → README-wins-over-index directory
  precedence (lone `index.md` serves the directory; `docs_dir` root never
  forms a directory page) → repo-relative `additional_docs`, classified
  second so a cross-namespace collision deterministically goes to the
  docs_dir page (design OQ-1a). **`validPublishedPath` guards the single
  writer**: git-tree paths (dot segments, control bytes, backslashes) never
  become store keys; the serve layer re-applies the same rules to decoded URL
  paths (`validPagePath`) and 404s — indistinguishable from a miss.
- **Pages are desired state.** `reconcileRepoPages` mirrors the documents
  reconcile (content-hash gate, delete-absent) keyed on the published path;
  a dormant block maps `Pages: nil` + empty api fields, so the reconcile
  wipes every row and nulls `repos.api_landing_page`/`api_additional_docs`
  (migration `20260828000000_add_repo_pages`). The nullable JSONB column
  needed its **own sqlc override entry** (`nullable: true` →
  `json.RawMessage`).
- **Webhook:** `shouldIngest`'s changelog exact-path check generalized to a
  `watched []string` built by `watchedFiles(repo)` (changelog + api landing
  page + decoded `api_additional_docs`; NULL columns contribute nothing) —
  a push touching only a root additional doc re-ingests.
- **Search (PR-2):** `IndexDoc`/`SearchHit` carry `Source` (`doc`/`page`) +
  `Path` (repo-relative on docs, published on pages); `source` is filterable
  + faceted. Page PK is `<repo_id>_p_<hex(sha256(published_path))[:16]>` —
  published paths carry chars Meilisearch ids reject, so the path is hashed;
  the `p` marker keeps it out of the doc-id namespace. `syncIndex` folds page
  deletes/upserts into its one delete + one index call via
  `GetRepoPagesByPaths`; the repo-id purge covers pages by construction.
- **Serve:** `GET .../pages` returns an **empty list, not 404**, for a repo
  without the block (the repo exists; its page set is empty); `GET
  .../pages/*` is a chi wildcard + `url.PathUnescape`, so both the literal
  and percent-encoded spellings resolve. Exact-byte lookup, no case folding.
- **Proof:** `TestE2ERepoPagesServeAndDisable` (real Postgres + Meilisearch)
  covers onboard → list/serve (directory page, file page, additional doc) →
  search with `source: page` → disable-at-HEAD → empty list, 404s, index
  purged. This repo **dogfoods** the block (`.docz.yaml`: `api.enabled` +
  `additional_docs: [DEVELOPMENT.md]`).

### Helm chart + publish pipeline (INV-0004 / IMPL-0004)

> **Superseded for the chart by `charts/docz`** (IMPL-0021, DESIGN-0018; see
> "Helm chart (`charts/docz`)" above). `charts/docz-api` is **deprecated**:
> 0.10.0 is its final version, and it is deleted at v2.0.0. What follows is
> its history. The publish-pipeline, monitoring, and `contrib/` conventions
> still hold.

The `charts/docz-api/` Helm chart, the container/chart publish workflows
(`ghcr.yml`/`ecr.yml` called from `release.yml`), the local monitoring stack
(`deploy/compose.monitoring.yaml` + `deploy/dev/`), and the operator assets
(`contrib/`) are being adapted from a repo-guardian/rfc-api copy-paste per
`docs/impl/0004-*.md` (the phased plan) — **IN PROGRESS**. Conventions
established as the build progresses:

- **Helm tooling is in `mise.toml`**: `helm` (4.2.2), `helm-ct`, `helm-diff`,
  `helm-docs`, `cosign`, `promtool`. `just` recipes drive it — `just helm-lint`,
  `just helm-template`, `just helm-unittest` (needs the `helm-unittest` plugin),
  `just helm-docs`; `just lint-alerts` runs `promtool check rules` on the
  contrib pack. Chart lint/template **must** pass `-f charts/docz-api/ci/ci-values.yaml`
  because the required values (secrets, app id, redirect base) have no defaults.
- **`docker-bake.hcl` `_common` sets `args = {VERSION, COMMIT, DATE}`** →
  Dockerfile ARGs → `-ldflags`. The bake vars are `VERSION`/`COMMIT_SHA`/
  `BUILD_DATE`; the publish workflows export them as env before the bake step,
  else images compile in `version=dev`.

#### Phase progress

- **Phase 1 — Repo plumbing quick fixes: COMPLETE ✅** — schema tags fixed
  (root `compose.yaml` / `.codecov.yml` / `sqlc.yaml` modelines added, wrong
  `ct.yaml` tag removed); bake build args restored; publish workflows compute
  build metadata; orphan `deploy/.env.dev.example` removed; helm + `lint-alerts`
  just recipes added.
- **Phase 2 — Chart core made renderable: COMPLETE ✅** — the chart's own
  surface (helpers, deployment, service, secret, NOTES, Chart.yaml, ci-values)
  now renders + lints clean against the docz-api config. `just helm-template`
  and `just helm-lint` both pass (Phase 2's only acceptance gate). Conventions:
  - **All helpers are `docz-api.*`** (renamed from `repo-guardian.*`); the dead
    `validateTemplatingVars`/`reservedEnvVars`/`templating.vars` machinery is
    gone. Added `docz-api.meiliFullname` (used by the deployment now; the
    Meilisearch resources land in Phase 3.3).
  - **`deployment.yaml` env is the IMPL-0004 Reference table verbatim**:
    `HTTP_ADDR` from `config.port`; app-secret refs for
    `GITHUB_APP_ID`/`GITHUB_WEBHOOK_SECRET`/`SESSION_SECRET`/
    `GITHUB_OAUTH_CLIENT_SECRET`; `GITHUB_APP_PRIVATE_KEY` as a mounted file at
    `/etc/docz-api/private-key/private-key.pem` (when `secrets.privateKeyAsFile`)
    else env-from-secret; `DATABASE_URL`/`REDIS_URL`/`MEILI_HOST`/`MEILI_API_KEY`
    via the store/queue/search secret helpers; `AUTH_REDIRECT_BASE`
    `required`-checked; plain-value optionals (`GITHUB_API_BASE`, `SESSION_TTL`,
    `INGEST_DEBOUNCE`, `OTEL_*`) emitted only when non-empty via `with`. One
    `http` containerPort (no metrics port); distroless `runAsUser: 65532`.
  - **`secret.yaml` carries five keys** (app-id, webhook-secret, private-key,
    session-secret, oauth-client-secret); `existingSecret` bypass must supply all
    five. (Superseded in chart 0.3.0 — the provider client-secret keys are now
    conditional; see the login-provider bullet below.) **`service.yaml` is one
    `http` port** (`service.port` → targetPort `http`). **`Chart.yaml`
    `appVersion` is the published **image** tag — bare semver, never
    `v`-prefixed** (it drives the default image ref, and the publish workflow's
    `type=semver,pattern={{version}}` strips the `v` from the git tag, so
    `v0.6.0` ships as `:0.6.0`). Charts 0.3.2–0.5.0 pinned `v`-prefixed
    appVersions and their default image ref 404s; `ci-values.yaml` swaps in
    busybox, so only `deployment_test.yaml`'s bare-semver assertion catches it.
    Chart `version: 0.1.0` at scaffold.
    - **Login providers are gated per-provider** (chart ≥ 0.3.0).
      `config.authProviders` is parsed once by the **`docz-api.authProviders`**
      helper (comma-split, whitespace-stripped, `compact`, `toJson`) and consumed
      as `include "docz-api.authProviders" . | fromJsonArray` → `has "okta" $providers`.
      Each enabled provider gates **both** its Deployment env block
      (`GITHUB_OAUTH_*` / `OKTA_*` / `KEYCLOAK_*`) **and** its Secret key
      (`oauth-client-secret` / `okta-client-secret` / `keycloak-client-secret`),
      so a github-only install renders neither `OKTA_*` env nor an okta key —
      and an okta-only install no longer demands a dummy GitHub OAuth secret.
      Issuer/client-id are plain `config.*` values (non-secret, mirroring
      `githubOAuthClientID`); only the client secret is a Secret key. Every
      enabled provider's fields are `required` at **render** time (house style,
      like `authRedirectBase`), so a missing issuer fails `helm install` instead
      of crash-looping — which is why the three deployment-rendering
      helm-unittest suites must set `config.githubOAuthClientID` at suite level.
      **`secrets.existingSecret` is the secret-manager seam** (1Password
      Operator / External Secrets / sealed-secrets): the chart references the
      Secret by key only, so there is deliberately **no ESO/ExternalSecret
      template** in the chart.
    - **GOTCHA — the main Service must scope on `app.kubernetes.io/component:
      server`** (chart ≥ 0.2.2). The baked postgres/valkey/meilisearch pods
      carry the **same** `docz-api.selectorLabels` (name+instance) AND expose an
      `http`-named port, so a Service selecting on selectorLabels alone enrolls
      them as endpoints → ~half of API traffic round-robins to meilisearch:7700
      → intermittent 404s. Fix: `component: server` on the API pod template
      **and** the Service selector. Deliberately **NOT** in the Deployment's
      `spec.selector.matchLabels` (immutable — would break `helm upgrade` from
      0.2.1). Regression-guarded in `service_test.yaml`/`deployment_test.yaml`.
  - **`ci/ci-values.yaml`** is the render/lint fixture: busybox + `sleep 900`,
    nulled probes, and a dummy for every `required` value — needed because the
    chart has no defaults for secrets/app-id/redirect-base.
  - **Deferred by design** (the whole-templates-dir grep criteria clear only when
    these land): `STORE_DSN`→`DATABASE_URL` / `QUEUE_VALKEY_DSN`→`REDIS_URL`
    secret-key renames + cnpg `repo-guardian` comment (Phase 3.1/3.2); the stale
    repo-guardian **helm-unittest suite** (`tests/`) rewrite (Phase 4.3); the
    `prometheusrule.yaml` `repo-guardian` alert text + `README.md.gotmpl` /
    `just helm-docs` regen (Phase 4.4/4.5). So `just helm-unittest` and
    `just helm-docs` are intentionally NOT green mid-Phase-2/3.
- **Phase 3 — Backing services wired: COMPLETE ✅** — the three deployed deps
  (Postgres, Valkey, Meilisearch) are now reachable by docz-api and render
  across every backend mode. Conventions:
  - **Store/queue DSN keys are the app's env names**: baked Postgres secret
    emits `DATABASE_URL` (db + user `doczapi`, `postgres://…/doczapi?sslmode=
    disable`); baked Valkey secret emits `REDIS_URL` (`redis://…/0`). The
    `docz-api.storeSecretName/Key` + `queueSecretName/Key` helpers resolve
    baked (chart secret) / cnpg (`<name>-app`, key `uri`) / external.
  - **External-mode refs live under `store.external.*` / `queue.external.*`**
    (`existingSecret` + `secretKey`, defaulting to `DATABASE_URL`/`REDIS_URL`),
    siblings of `store.postgres` / `queue.valkey` — the `mode` field stays under
    `store.postgres` / `queue.valkey`. Same shape for `search.meili.external.*`.
  - **Meilisearch is a first-class baked dep** (`search-meili.yaml` +
    `search-meili-secret.yaml`): StatefulSet `getmeili/meilisearch:v1.12` on a
    headless `<fullname>-meilisearch:7700` Service, `/health` probes,
    `/meili_data` PVC. Its master key is **operator-supplied**
    (`search.meili.masterKey`, required in baked mode) — NOT auto-generated like
    the pg/valkey passwords — because Meilisearch (`MEILI_MASTER_KEY`) and
    docz-api (`MEILI_API_KEY`) must share the exact value; one secret key
    `MEILI_API_KEY` feeds both. `docz-api.meiliHost/searchSecretName/
    searchSecretKey` mirror the store/queue helpers.
    - **Baked-mode `existingSecret` escape hatch** (post-IMPL-0004): set
      `search.meili.existingSecret` (+ `existingSecretKey`, default
      `MEILI_API_KEY`) to source the master key from a pre-existing Secret
      instead of the plaintext `search.meili.masterKey` — the chart then renders
      **no** `search-meili-secret.yaml`, and both `searchSecretName`/`Key`
      helpers (so the baked StatefulSet's `MEILI_MASTER_KEY` **and** docz-api's
      `MEILI_API_KEY`) point at that Secret. `masterKey` is required only when
      baked AND `existingSecret` is unset. This is baked-only and distinct from
      `search.meili.external.existingSecret` (which switches to an external
      Meili). The baked pg/valkey passwords are deliberately **not** given this
      hatch — they're chart-generated + `lookup`-preserved, and an existing
      secret there is the signal to use `mode=external`.
  - **The `search.meili.image` field** was added (not in the plan's values
    block) for parity with `store.postgres.baked.image` / `queue.valkey.baked
    .image` and Renovate.
  - **Mode matrix renders clean**: default (all baked), all-external, and
    `store.postgres.mode=cnpg` all `helm template` with exit 0. `ci-values.yaml`
    gained `search.meili.masterKey: ci-dummy`. The `repoguardian`/`STORE_DSN`/
    `QUEUE_VALKEY_DSN` grep is clean in `templates/`; residual hits live only in
    the generated `README.md` (Phase 4.5) and `tests/` (Phase 4.3).
- **Phase 4 — Chart observability, tests, docs: COMPLETE ✅** — monitoring
  points at real metrics, the chart's behavior is frozen by a rewritten test
  suite, and the docs are docz-api. **`charts/` is now completely free of
  `repo-guardian`/`repo_guardian`** (both spellings, case-insensitive).
  - **ServiceMonitor** scrapes port `http` at `/metrics`, gated on
    `metrics.enabled AND serviceMonitor.enabled`. **PrometheusRule** is 5
    docz-api RED alerts (`DoczAPIDown` critical, `DoczAPIHighErrorRate`,
    `DoczAPISlowRequests`, `DoczAPIIngestFailures`, `DoczAPISlowIngest`) over
    the four real metrics — `docz_api_http_requests_total`,
    `docz_api_http_request_duration_seconds_bucket`,
    `docz_api_ingest_jobs_total`, `docz_api_ingest_job_duration_seconds_bucket`
    — plus `up`.
  - **helm-unittest suite: 8 files / 58 tests, all green** (`just helm-unittest`).
    Each starts with the `helm-testsuite` `$schema` modeline. helm-unittest
    renders only the templates a suite lists, so each suite provides just its
    own template's `required` values via a suite-level `set:` (the deployment
    only needs `config.authRedirectBase`; the meili secret needs
    `search.meili.masterKey`). Fullname under the default release is
    `RELEASE-NAME-docz-api`.
  - **deployment `volumeMounts` is guarded** (`{{- if or .privateKeyAsFile
    .extraVolumeMounts }}`) so it's omitted, not emitted as `null`, when the
    key is passed via env and there are no extra mounts.
  - **`values.schema.json`** is a permissive guardrail (`additionalProperties:
    true` everywhere): enums for the three backend `mode`s + `config.logLevel`
    /`logFormat`, typed blocks for config/metrics/otel/secrets/store/queue/
    search. Rejects `store.postgres.mode=memory` / `config.logLevel=trace` at
    render time.
  - **README** is generated from `README.md.gotmpl` via `just helm-docs`; the
    committed `README.md` is regen-idempotent (`git diff --exit-code`). Chart
    `CHANGELOG.md` + `cliff.toml` `[changelog].header` say docz-api. `ct lint
    --config ct.yaml` passes locally.
- **Phase 5 — CI + release consolidation: COMPLETE ✅** — one CI workflow, one
  Release workflow; the `ci2.yml`/`release2.yml` scaffolding duplicates are
  deleted; `just lint-actions` (actionlint) is clean.
  - **`ci.yml`** folds in the `changes` (dorny/paths-filter) job + the four
    path-gated jobs: `lint-alerts` (`just lint-alerts`), `docker-build`
    (buildx/bake — PR pushes `:dev`, post-merge cache-only), `helm-unittest`
    (plugin install + unittest), `helm-test` (ct lint + kind install). The
    richer Go jobs (Lint incl. openapi, Test Go + Codecov, Security =
    govulncheck + Trivy, Build + SBOM scan) + Label PR stay. All `make` →
    `just`.
  - **`release.yml`** appends the `publish-ghcr` + `publish-ecr` reusable-workflow
    jobs. **Caller-job permissions are a hard ceiling for a called reusable
    workflow** — omitting one the callee needs fails the whole run at startup
    (zero jobs created), so `attestations: write` must be granted there as well
    as in `ghcr.yml`/`ecr.yml`. Top-level `id-token: write`. **No GPG signing**
    (OQ-4a): the GPG import step + `GPG_FINGERPRINT` are dropped (secrets don't
    exist, `.goreleaser.yml` has no signing config); goreleaser keeps producing
    unsigned archives, while images/charts are cosign-signed + provenance-
    attested by the publish workflows. `pr-semver-bump` lives only in
    `release.yml`.
  - **Build provenance is `actions/attest-build-provenance`** (INV-0006), run
    as a step **inside** the `image`/`chart` jobs so it shares their digest and
    registry login — not a nested reusable workflow. It replaced
    `slsa-github-generator`, which hardcodes cosign **v2.2.3** (no input to
    override, unchanged on `main`) and therefore wrote provenance in cosign's
    legacy `.att` attachment while our own v3 `cosign sign` wrote the
    referrers-fallback `sha256-<digest>` attachment. The result was that **no
    single cosign major could verify both** — v3 couldn't read the provenance,
    v2 couldn't read the signature. GHCR serves no OCI referrers API, so the
    fallback-tag scheme is what's in play. Trade accepted knowingly: GitHub
    attestations are SLSA v1 **Build L2**, not the generator's L3 claim; docs
    say L2. Artifacts published before chart 0.3.2 / image v0.5.1 keep the old
    split and need `cosign v2.x` (or `gh attestation verify`).
  - **ECR publishing** is gated on the `ECR_PUBLISH_ENABLED` repo variable and
    documented in `docs/operations/ecr-publish-setup.md` (OIDC role trust
    `repo:donaldgifford/docz-api:*`, the `docz-api` ECR repo, the three
    `ECR_*` secrets). `ecr.yml`/`ghcr.yml` keep `workflow_dispatch` + the
    Phase-1.3 bake-metadata step.
- **Phase 6 — Local monitoring stack: COMPLETE ✅** (6.1–6.9) —
  `deploy/compose.monitoring.yaml` (`name: docz-api-monitoring`) runs the
  observability backends only (prometheus/grafana/otel-collector/jaeger/loki/
  alloy always-on; keycloak behind `--profile auth`), paired with the app run on
  the host (`just run`) or in containers, pointed at
  `OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318`. All bind mounts are
  `./dev/…` relative to `deploy/`. Verify: `up -d --wait` → all six always-on
  containers Healthy (rc=0).
  - **GOTCHA — jaeger `all-in-one:latest` is a trap.** `latest` moved to the v2
    line (v1 hit EOL 2025-12-31) which restructures the CLI/config, and the
    non-root image user can't `mkdir` under a root-owned badger volume
    (`/badger/key: permission denied`). Pin **`jaegertracing/all-in-one:1.76.0`**
    + **`SPAN_STORAGE_TYPE: memory`** (dev backend — traces are inspected live,
    not persisted; no volume needed). Dropped the `jaeger_data` volume.
  - **GOTCHA — the `loki` exporter was removed from
    `otel/opentelemetry-collector-contrib`.** Push logs via **`otlphttp`** to
    Loki's native OTLP ingest: `endpoint: http://loki:3100/otlp` (otlphttp
    appends `/v1/logs`). Loki 3.x accepts OTLP by default.
  - **GOTCHA — `quay.io/keycloak/keycloak` has no floating major tag.** `:26`
    404s (quay publishes only full version tags); pin a concrete patch
    (`26.7.0`). Keycloak (`--profile auth`) imports `dev/keycloak/
    docz-api-realm.json` on boot: realm `docz-api`, one **confidential** client
    `docz-api` (secret `dev-docz-api-secret`, redirect
    `http://localhost:8080/auth/callback`), dev user `dev`/`dev-password` with a
    **verified** `dev@localhost` (docz-api's OIDCProvider drops unverified
    emails). Issuer: `http://localhost:8180/realms/docz-api`. No healthcheck on
    the keycloak service, so `--wait` doesn't gate on realm-import readiness —
    poll the `.well-known/openid-configuration` endpoint.
  - **otel-collector has NO metrics pipeline** — docz-api metrics are pull-based
    (Prometheus scrapes `/metrics` directly per `dev/prometheus/prometheus.yaml`),
    so the copied `prometheusremotewrite` exporter + metrics pipeline were deleted
    (INV-0004 Obs. 5e). Only traces (→ jaeger) and logs (→ loki) pipelines remain.
    App logs also flow via **alloy** (docker-stdout tail → `loki.write`), so LOG
    output needs `LOG_FORMAT=json` for alloy's JSON stage to extract
    `trace_id`/`span_id` labels.
  - **verified end-to-end (6.9 close-out):** ran the built binary on `:8081`
    against the stack with `OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4318`; after a
    traffic burst, Prometheus's `docz-api` target is `health=up`,
    `docz_api_http_requests_total{route,method,status}` increments (the exact
    series the overview dashboard queries), and Jaeger shows `docz-api` traces
    with span names = chi route templates (`GET /api/v1/*`, `GET /openapi.yaml`)
    + `http.route`/`http.response.status_code` tags. Keycloak login is verified
    structurally (realm imports, issuer + auth/token endpoints resolve, confidential
    client + verified dev user seeded); the browser round-trip is a manual step.
  - **NOTE — collector logs two benign deprecation warnings** (`otlp` →
    `otlp_grpc`, `otlphttp` → `otlp_http` exporter-type aliases). Still functional
    in collector-contrib v0.156; left as the canonical names. The always-on images
    (collector/prometheus/grafana/loki/alloy) still float `:latest` — jaeger +
    keycloak are pinned because `:latest`/`:26` broke (see gotchas above); a full
    pin-all pass is a possible follow-up.
- **Phase 7 — contrib/ rewrite: COMPLETE ✅** (7.1–7.4) — operator-facing assets
  in docz-api vocabulary; **no Go changes** (the pack uses only the four existing
  `docz_api_*` metrics).
  - `contrib/prometheus/alerts.yaml` is the **same pack as the chart's
    `prometheusrule.yaml`** in plain-Prometheus format: one `docz-api` group with
    the five shared alerts (DoczAPIDown / HighErrorRate / SlowRequests /
    IngestFailures / SlowIngest) **plus** a file-only `DoczAPINoScrapes`
    (`absent(up{job="docz-api"})`, only meaningful with a static scrape config
    owning the `job` label). `just lint-alerts` (`promtool check rules`) → 6 rules.
  - `contrib/grafana/docz-api-dashboard.json` is **import-style** (`__inputs`
    `DS_PROMETHEUS` + `__requires`, all panels `${DS_PROMETHEUS}`), Prometheus-only
    (the Loki/Jaeger panels from the dev overview are dropped — operators may run
    just metrics). uid/title/tags = `docz-api`.
  - `contrib/README.md` documents the four metrics (verified against
    `internal/telemetry/metrics.go`), the single-port `:8080/metrics` scrape
    (+ `METRICS_ENABLED`, outside the auth gate), PromQL examples, dashboard
    import, and the two alert-delivery paths.
  - **IMPL-0004 is COMPLETE** — all 7 phases done; local acceptance gates all
    green (`just ci`, helm-lint/unittest[58]/template, `ct lint`, actionlint,
    monitoring live-smoke, cross-dir rfc/repo-guardian sweep, no-secrets). The
    two out-of-band Testing-Plan items — the **remote** GH Actions run (needs a
    branch push) and the **browser** keycloak login round-trip — are verified to
    their runnable/server-side extent only.

### Error observability + queue self-heal (INV-0007 / IMPL-0006)

Ingest failures used to be invisible: asynq never logged the handler's error,
and an exhausted task blocked every future trigger for that repo. INV-0007
(Concluded) diagnosed it and IMPL-0006 fixed it. Conventions established:

- **asynq's `Config` needs three fields wired, not one.** `Logger` (an slog
  adapter, `internal/queue/logger.go`, tagging `component=asynq`), `LogLevel`
  (derived from the slog logger's own enabled level), and `ErrorHandler`
  (`logIngestFailure`). **asynq logs a handler's returned error nowhere unless
  an `ErrorHandler` is registered** — the error is written only to the task
  record in Redis. Every failed attempt now logs the real cause plus task id,
  `retried`, `max_retry`, repo, and reason; the terminal `Retry exhausted`
  WARN arrives through the same pipeline.
- **A finished task must be cleared, not coalesced onto.** asynq's `TaskID`
  uniqueness is a bare `EXISTS`, so an **archived** (retry-exhausted) or
  **completed**-with-retention task keeps owning the id and every later
  enqueue returns `ErrTaskIDConflict` — which the old code treated as
  successful coalescing. `EnqueueIngest` now inspects the conflicting task
  (`taskInspector` seam over `*asynq.Inspector`): a live pending/scheduled/
  active task coalesces (logged at **Info** with its `state`), while a
  terminal one is deleted and re-enqueued once (`cleared_state`). **`Retention`
  was removed entirely** — with it set, a *successful* ingest blocked that
  repo for 24 h (F4b), which is a live-production bug, not a theoretical one.
- **The GitHub App is verified at boot, not by a probe.** `githubapp.SelfCheck`
  mints the App JWT and calls `GET /app`, logging the authenticated slug.
  **Only a 401 fails startup** (`ErrCredentialsRejected`), plus a malformed PEM,
  which fails inside `NewAppsTransport` before any request and is the most
  common real-world credential fault. Everything else — 403, 429, 408, 404, 5xx,
  transport errors — warns and continues. Do **not** widen this back to "any
  4xx": GitHub uses 403 for primary rate limiting as well as for a suspended
  App, and go-github only produces a typed `*RateLimitError` when the response
  carries `X-RateLimit-Remaining: 0`, so a 403 from a proxy, a CDN, or a GHES
  front end is indistinguishable from a refusal and would crash-loop a deploy
  whose key is fine. `GET /app` needs no permissions, so 401 is the only
  unambiguous bad-key signal. The two errors are not symmetric — a false
  "permanent" takes down the read API for an ingest-only problem, while a false
  "transient" costs one startup warning, and every ingest attempt now logs its
  own cause. **Probe trio:** `/healthz` = liveness (checks
  nothing downstream — a restart cannot fix a dependency), `/readyz` =
  readiness (postgres/redis/meili, 503 names the offender), boot self-check =
  neither (GitHub is not a serving dependency).
- **Every error path terminates in a logging sink.** The recurring bug was a
  wrapped error chain reaching the client as a bare status code and dying
  there. `session.Middleware` splits on whether the caller has a usable session,
  not on whether an error occurred: `ErrSessionNotFound` (ordinary churn) and
  `ErrSessionCorrupt` (the value exists but will not decode) are both **401**,
  the latter logged at Warn; only a backend that cannot answer at all gets
  **503** `{"error":"session unavailable"}`. Both halves matter. 503 for a Redis
  blip stops docz-site — which drives its login UI off 401 — from reading an
  outage as a logout; 401 for a corrupt session avoids a dead end, since
  `/api/v1/auth/logout` sits behind this same gate, so a 503 there would leave
  the holder unable to clear the bad cookie or log in again. `Store.Lookup` is
  the producer of both sentinels and is tested as such — testing only the
  middleware leaves a refactor free to drop the label and silently restore the
  dead end. `authorize` logs before its 500;
  `webhook` logs a body-read failure at **Warn with headers only** (pre-HMAC,
  so the payload is unverified) and a `ParseWebHook` failure at **Error**
  (post-HMAC, so it is schema drift, not a bad caller); both `writeJSON`
  marshal failures route through their package's `serverError` helper.
- **`AUTH_PROVIDERS=none`** is the login-free first-setup mode (`config
  .AuthDisabled()`; `none` must be the only entry). `session
  .AnonymousMiddleware` injects a synthetic `Session` (`provider "none"` /
  `subject "anonymous"`) through the **same unexported ctx key**, so
  `authorize`, every handler, and `/api/v1/auth/session` need no no-auth
  branch and the wire contract is unchanged. `/auth/login` + `/auth/callback`
  are left unmounted (404). The chart gates on `docz-api.authDisabled`;
  values.yaml, both READMEs, and the spec all state the exposure — the read
  API is open to anyone who can reach the Service.
- **The verification standard is "reconstruct it from logs alone."** IMPL-0006
  Phase 6 ran the full failure lifecycle against real Postgres/Redis/Meili and
  a real GitHub App — first failure, five retries, exhaustion, root cause,
  fix, self-heal, successful ingest — reading only `LOG_FORMAT=json` output
  piped through `jq -e`. Valkey was never opened. Re-run that drill after any
  change to the queue's failure path.

### Renovate

- `go.mod` updates are PR'd by Renovate's Go module manager.
- Container base images in `Dockerfile` are PR'd by the Docker manager.
- `mise.toml` versions are handled by a custom regex manager configured upstream
  in `donaldgifford/renovate-config`.
