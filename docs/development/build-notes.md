# Build notes

Moved from the repository's `CLAUDE.md`: how the `just` recipes are
laid out and what replaced each `make` form, what
`just lint-generated` checks, and the two recipes that write a committed
file from `ui/node_modules`.

## `just` replaced `make`

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
`chart::lint` in the `ci` gate. All four live at the root; `ui.just` will carry
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

## `just lint-generated`

`just lint-generated` (IMPL-0025, #127) is the gate on docz's own output: it
builds docz, scaffolds a repository in a temp directory (`init`, the runbook
enabled, a template-less custom type, one `create` per type, `update`,
`wiki init`, `template override impl`, and docparse's `anchors.md` filed as an
RFC with a ToC region, so MD051 checks the ToC docz writes for every slug
case), and lints `docs/**` with markdownlint-cli2 twice — default rules and
this repository's `.markdownlint.yaml` — keeping the scaffold on failure. It
resolves the linter with `mise which` before leaving the repository, because
mise picks a tool's version by working directory and a temp directory gets
the global one. The pin is `mise.toml`'s `markdownlint-cli2` (0.23.3, which
has MD060; 0.18.1 hid the table failures), and the Test Go job installs that
version with npm, read from the file. Three revert drills prove it bites:
the old delimiter row fails MD060, the ASCII-only slug filter MD051, and
`MD024` off the IMPL template's disable line MD024.

## Files generated from `ui/node_modules`

Two committed files under `pkg/` are written from a package `ui/bun.lock`
pins, each by a root recipe that needs `just ui install` first and fails
with that hint when `ui/node_modules` lacks the package.

| File | Recipe | Package | Held by |
| --- | --- | --- | --- |
| `pkg/export/html/assets/mermaid.min.js.gz` | `just vendor-mermaid` | `mermaid` | `TestVendoredMermaid` (version and SRI hash) |
| `pkg/theme/tokens/charcoal.css` | `just gen-charcoal` | `@donaldgifford/design-system` | `TestCharcoalMatchesPinnedDesignSystem` (version) and `just ui gen-charcoal-check` (bytes) |

`charcoal.css` (IMPL-0033 Phase 3, DESIGN-0028 section 11) is the design
system's dark set under `pkg/theme`'s token names. `ui/scripts/gen-charcoal.ts`
reads the package's `tokens.json`, resolves its references, converts oklch
to hex, and fills the token block of `ui/scripts/charcoal.template.css`,
which holds the hand-written header and font block. The table of where each
of the 58 names comes from is `ui/src/theme/charcoal.ts`; the 26 the package
has no value for come from the dark block of `ui/src/theme/docz-tokens.css`.
A name left without a value, or given two, fails the run and names the
token. Running the recipe twice writes the same bytes.

The Go job has no `node_modules`, so the two checks split: the Go test
holds the version recorded in the file's header to the lockfile, and the
`ui` job's `gen-charcoal-check` regenerates to a temporary file and fails on
a difference. Neither rewrites the tree.

A bump of the package is three things in one pull request: the lockfile,
`just gen-charcoal`, and the export's goldens (`go test ./... -update`).
The goldens that move are `pkg/export/html/testdata/golden/full-page-charcoal.html`
and the page hashes in `cmd/testdata/cli-contract/v1/export-html-*.json`,
since an exported page inlines its theme. `test/parity` reports a failure
under `-update` because it defines no such flag; run `go test ./...` again
without it to see the real result.
