# CLAUDE.md: `plugins/`

Loaded when a session works under this directory. The repository's map is the root `CLAUDE.md`.

## Plugins and the marketplace

**Every docz plugin is written here and copied out.** The repository root's
`.claude-plugin/marketplace.json` is the `docz` marketplace, listing
**docz** (the general skills and agents, brought home from claude-skills at
2.0.2), **docz-review**, and **docz-html**, each with its own version in its
`plugin.json`, independent of docz's. `/plugin install <name>@docz`
installs from `main`.

- **Change a plugin here, never in claude-skills.** A plugin change is a
  PR in this repository with a bump of that plugin's `plugin.json`
  version; the copy follows it.
- **The copy** into `donaldgifford/claude-skills` is a PR there that
  replaces `plugins/<name>/` with this repository's directory, keeping
  that repository's departures: `plugin.json`'s `homepage` and
  `repository` point at claude-skills, `CARD.md` and the README's card
  link block are generated there, and `tests/test.sh` is its harness (in
  place of docz-review's Bun suite, which reads `ui/review/`). Its
  marketplace metadata version is bumped and its cards regenerated in the
  same PR.
- **The skills drive the CLI** and ship no template copies: a skill names
  a command and its flags, so a CLI change that renames or removes either
  updates the skill in the same PR. `plugins/docz/example.docz.yaml` is
  `docz init`'s output verbatim; regenerate it when the generated
  `.docz.yaml` changes.
- **claude-skills' harness is stricter than this repository's CI**: its
  `tests/test.sh` for docz holds the core `SKILL.md` under 150 lines and
  checks `config.md` and `example.docz.yaml` against the CLI's defaults.
  Keep the source within those limits so the copy needs no edits.
- **CI** (the `plugin` job) checks every `plugin.json` has a name, version,
  and description, and that every marketplace source exists. markdownlint
  does not run over `plugins/`.

## docz-review (`ui/review/`, `plugins/docz-review/`)

docz-review (DESIGN-0024, IMPL-0027) holds the end of a Claude Code turn
until a person has reviewed, in a browser, the docz documents the agent
created. It is three pieces, and `ui/CLAUDE.md` has the detail.

- **The page** is docz-site's second Vite entry, `ui/review.html`, built by
  `just ui build-review` into `ui/dist-review/`, reusing the reader's
  markdown pipeline in a review render mode. Its controls are the design
  system's parts (DESIGN-0028 §10): buttons, a tab per document, the
  status as a segmented control, and a dialog before it discards a
  draft. The package comes from GitHub Packages, so building the page
  or the binary needs `NPM_TOKEN` in the environment; running the binary
  needs nothing, and its page asks no other origin for anything.
- **The binary** is a Bun server in `ui/review/`, compiled by
  `bun build --compile` with the page embedded, for linux and darwin on
  x64 and arm64. It drives `docz inspect` and `docz validate` through the
  versioned JSON contract in `api/cli/` (`schema_version` 1; `cmd/`'s
  `TestCLIJSONContractFixtures` is the producer's side). State lives under
  `DOCZ_REVIEW_STATE_DIR`, else `$XDG_STATE_HOME/docz/review`, else
  `~/.local/state/docz/review`, one directory per repository root and
  session, 0700, written atomically under `flock`.
- **The credential** is a random token in the URL the binary opens, sent
  back as a bearer header. There is no cookie, the server listens on
  `127.0.0.1` only, any other `Host` is 421, and a mutation needs the exact
  `Origin` and a JSON body, because a forged submission is a prompt
  injection into the agent.
- **The plugin** is `plugins/docz-review/`: a `PostToolUse` hook on `Bash`
  that arms what `docz-review arm` named, and what `docz create` reported
  only while `DOCZ_REVIEW_AUTO_ARM` is on (off by default), a `Stop` hook
  that runs `docz-review --pending --hook` for up to 24 hours, and the
  `/docz-review:review` skill. The repository root's
  `.claude-plugin/marketplace.json` makes this repository the `docz`
  marketplace, so `/plugin install docz-review@docz` installs from `main`.
  The plugin's version is its own, in `plugin.json`, not docz's.
- **Recipes**: `just ui dev-review`, `test-review` (vitest over the page
  and `bun test ./review/`), `e2e-review`, `build-review [version]`, and
  `bun test plugins/docz-review/tests/` for the hooks.
- **Release**: `release.yml` and `prerelease.yml` run
  `just ui build-review "<tag>"` before GoReleaser, whose `extra_files`
  attach the four binaries, `docz-review_checksums.txt`, and the syft SBOM
  of `ui/`; a step after it signs that checksum file with the release GPG
  key and uploads the signature.

## docz-html (`plugins/docz-html/`, `pkg/theme`, `pkg/export/html`)

docz-html (DESIGN-0025, IMPL-0028) turns one docz document into one
standalone HTML page in docz's theme. `docz export html` makes the page
(`pkg/export/html`, above); the plugin, the marketplace's second, with its
own version in `plugin.json` (0.3.1), is at **stage 2**: a skill that
validates, runs the command, reports the page, and improves a page through
the document's **markdown**, never its HTML. `references/layout.md`
(region kind to tab and component) and `widgets.md` (the closed YAML of
`docz-flow`, `docz-stepper`, `docz-scorecard`) stay as the author's
reference, and `components.md` as the markup the renderer writes. Stage 1
carried a copy of `pkg/theme` and had the agent assemble pages by hand
from `docz inspect`; the copy, `just plugin-theme-sync`/`-check`, and the
plugin job's drift check went with it.

- **The theme** is the repository's (IMPL-0030 Phase 2): `docz export
  html` reads `.docz.yaml`'s `html.theme` when `--theme` is empty (the
  flag's default) and `ExportOptions.Theme` empty means the same, so a
  library caller gets the CLI's default; `--theme` is a one-off and is
  still checked against `theme.Names()` (exit 2). The JSON report names
  the theme a run used (`theme`, `export-html.schema.json` `$version`
  1.1), and `docz inspect --format json` reports the repository's theme as
  a top-level `theme` (`inspect.schema.json` `$version` 1.1, not required,
  so an older docz's output still validates and docz-review reads its
  absence as `paper`). The skill passes no `--theme` unless the user asks
  for a theme.

- **Widgets** are drawn by `pkg/theme/runtime.js` from a
  `figure.widget` holding a `script[type="application/json"][data-widget]`
  island, an empty `.widget-mount`, and a `.widget-fallback`. The drawers
  build DOM with `textContent` only, mark the figure `drawn` (hiding the
  fallback), and redraw on a mode change; an island that fails to parse
  or draw leaves the fallback. `pkg/export/html/widgets.go` writes the
  figure from a validated block. The reader's document pane gets that
  figure in its fragment and the runtime draws it (IMPL-0032); docz-site's
  pages and fallback render and docz-review draw the same blocks in React
  (`ui/src/markdown/widgets/`, IMPL-0030 Phase 5) through a TypeScript port of the validator, held to `widgets.go` by
  the shared corpus `pkg/export/html/testdata/widgets/` (`*.yaml` with a
  `.json.golden` or `.errors.golden` each; `widgets_corpus_test.go` is
  the Go side, `validate.test.ts` the UI's), so a block draws, or fails
  with the same errors at the same lines, on all three surfaces.
- **The skill's evals.** `plugins/docz-html/evals/` is an eight-case
  `claude plugin eval` suite over a fixture repository in
  `evals/fixture/repo/`, run with `just plugin-eval docz-html` (local only:
  each run is a Claude session). The run's sandboxed Bash gets a login
  PATH, not the caller's, and cannot read the home directory or `/tmp`, so
  `evals/fixture/setup.sh` copies the build's docz into the run's own
  `$HOME/.nix-profile/bin`, the first entry of that PATH. An export is
  graded by the page existing (`file_exists`), never by the command being
  called, because a `docz` that is not found still counts as a call.
- **The browser test.** `ui/e2e/export-html.spec.ts` (in `just ui e2e`,
  so the `ui-e2e` job, which builds docz first) exports copies of the
  parity fixtures with that docz and opens pages from `file://`: every
  tab, the mode toggle, the IMPL task filter, an open question, and the
  three widgets, and a fixture whose `.docz.yaml` says `html: {theme:
  classic}` exporting classic pages with no flag, with no console error and
  no network request. It needs
  `just build` locally (or `DOCZ_EXPORT_DOCZ`).
