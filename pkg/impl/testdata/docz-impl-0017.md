---
id: IMPL-0017
title: "v1.3.0 — updated frontmatter field and the docz update stamp pass"
status: Draft
author: Donald Gifford
created: 2026-09-12
---

<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL-0017: v1.3.0 — updated frontmatter field and the docz update stamp pass

<!--toc:start-->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: docwrite.SetUpdated](#phase-1-docwritesetupdated)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2: read side and the updated: config block](#phase-2-read-side-and-the-updated-config-block)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 3: git resolver and the stream parser](#phase-3-git-resolver-and-the-stream-parser)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 4: CLI integration](#phase-4-cli-integration)
    - [Tasks](#tasks-3)
    - [Success Criteria](#success-criteria-3)
  - [Phase 5: living docs, consumer contract, release, dogfood](#phase-5-living-docs-consumer-contract-release-dogfood)
    - [Tasks](#tasks-4)
    - [Success Criteria](#success-criteria-4)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Dependencies](#dependencies)
- [Open Questions](#open-questions)
  - [1. Where does the git log -p stream parser live?](#1-where-does-the-git-log--p-stream-parser-live)
  - [2. PR and release strategy?](#2-pr-and-release-strategy)
  - [3. Does the real-git integration test cover the shallow-clone path?](#3-does-the-real-git-integration-test-cover-the-shallow-clone-path)
  - [4. Should docz update get a flag to skip the stamp pass?](#4-should-docz-update-get-a-flag-to-skip-the-stamp-pass)
  - [5. Does the Updated column also appear in docz list --format=csv?](#5-does-the-updated-column-also-appear-in-docz-list---formatcsv)
  - [6. Where does the dogfood backfill in this repo land?](#6-where-does-the-dogfood-backfill-in-this-repo-land)
  - [7. Does the resolver stop reading git log -p early?](#7-does-the-resolver-stop-reading-git-log--p-early)
  - [8. What clock does "today" use for an edit-time stamp?](#8-what-clock-does-today-use-for-an-edit-time-stamp)
- [Decisions](#decisions)
- [References](#references)
<!--toc:end-->

<!--docz:objective:start-->
## Objective

Implement DESIGN-0012: an opt-in `updated:` config block and an
`updated: YYYY-MM-DD` frontmatter field that `docz update` backfills from git
and keeps current, stamped at edit time so the pass converges without
bookkeeping commits. Ships as docz `v1.3.0` (a `minor`: new public field,
config block, and `docwrite` function), ratified for consumers as
DESIGN-0008 R12, with docz-api #36 waiting on the tag.

**Implements:** DESIGN-0012 (all eight decisions locked 2026-09-12), scoped
by INV-0008 (Concluded).
<!--docz:objective:end-->

<!--docz:scope:start-->
## Scope

<!--docz:in-scope:start-->
### In Scope

- `docwrite.SetUpdated` — the package's first upsert (rewrite in place, or
  insert after `created:`), plus `ErrUpdatedFieldUnsupported`.
- `document.Frontmatter.Updated` and `config.UpdatedConfig` /
  `Config.Updated`, dormant by default, rendered into `docz init` output.
- `GitResolver` growth (`Shallow`, `State`, `LastChange`) with the
  `git log -p` stream parser and its bookkeeping-commit filter.
- The stamp pass in `docz update` (once-per-invocation git probe, per-doc
  stamping model, dry-run lines, shallow-clone warning, table
  write-through); stamps in `docz create` and `docz status set`.
- The enabled-only index layout (`Created` + `Updated` columns) and the
  matching `docz list` column; `updated` in `docz list` JSON.
- Living docs (README, CLAUDE.md), DESIGN-0008 R12, the external-consumer
  proof, the `v1.3.0` release, the post-tag flips **and** dogfooding the
  block in this repo.
<!--docz:in-scope:end-->

<!--docz:out-of-scope:start-->
### Out of Scope

- The commit sha (`updated_sha:`) — deferred by INV-0008; DESIGN-0012
  Non-Goals. `Change.SHA` is populated by the resolver but nothing reads it.
- Template changes (DESIGN-0012 Decision 2) and any per-type key name
  (INV-0008 Decision 3).
- docz-api / docz-site work — tracked in docz-api #36.
- Any general frontmatter editor, `--repo-root` cleanup beyond what the git
  path routing needs, or changes to `created`.
- A `--no-stamp` / `--no-updated` flag on `docz update` (Decision 4): the
  config block is the switch and a shallow clone already skips with a
  warning.
<!--docz:out-of-scope:end-->
<!--docz:scope:end-->

## Implementation Phases

Each phase builds on the previous one. A phase is complete when all its
tasks are checked off and its success criteria are met. Phases are commit
boundaries inside one `minor` PR (Decision 2); every phase ends with
`make fmt`, `make lint`, `make ci` green and a go-review pass over the
phase's diff.

---

<!--docz:phase:start-->
### Phase 1: `docwrite.SetUpdated`

The byte-level write primitive, in `pkg/doczcore/docwrite`. Built first
because Phases 4 and 5 call it and its golden suite is the proof that the
status locator refactor changed nothing.

<!--docz:tasks:start-->
#### Tasks

- [ ] go-architect pass on the insert design before coding: confirm the
      generalized locator's shape, the insertion rules, and that a
      purpose-named `SetUpdated` (not a generic `SetScalar`) is what ships
      (DESIGN-0012 §`docwrite.SetUpdated`, ADR-0001 Decision 5).
- [ ] Generalize the status locator: `findStatusValue` becomes
      `findScalarValue(content []byte, keyRE *regexp.Regexp) (start, end
      int, value string, err error)`; `parseStatusValue` becomes
      `parseScalarValue`, returning a **bare** `errUnsupportedShape` /
      `errFieldMissing` that each caller wraps with its own sentinel so
      `errors.Is(err, ErrStatusFieldMissing)` keeps working for existing
      consumers. `statusKeyRE` stays; add `updatedKeyRE =
      ^updated:[ \t]*` and `createdKeyRE = ^created:`.
- [ ] Rewire `SetStatus` onto the generalized locator. **Behavior and error
      messages must not change** — the 12 fixtures under
      `testdata/golden/status/` are the proof and must not be regenerated.
- [ ] Add the insert helper: `insertFrontmatterLine(content []byte,
      blockStart, blockEnd int, afterKeyRE *regexp.Regexp, line string)
      []byte`. Inserts `line + "\n"` immediately after the first line in the
      block matching `afterKeyRE` (the `created:` line); when no line
      matches, appends it as the last line of the block, just before the
      closing `---`. `frontmatterBounds` already tolerates the one leading
      blank line; reuse it, do not re-derive the bounds.
- [ ] Add `ErrUpdatedFieldUnsupported` and `SetUpdated(path, date string)
      (old string, err error)`: read; reject CR/CRLF with
      `ErrUnsupportedLineEndings`; `document.ErrNoFrontmatter` when no block;
      locate `^updated:` — supported shape → splice the value bytes only
      (return the old value); unsupported shape → `ErrUpdatedFieldUnsupported`
      **before any write** (never a duplicate key); absent → insert a bare
      `updated: <date>` line after `created:` (return `""`). Write with
      `config.FileMode`; wrap IO errors with the path. No value validation,
      matching `SetStatus`.
- [ ] Golden suite `TestSetUpdated_Golden` mirroring `TestSetStatus_Golden`
      (`testdata/golden/updated/<case>.input.md` / `.output.md`,
      `-update` regenerates): `bare`, `double-quoted`, `single-quoted`,
      `trailing-comment`, `insert-after-created`, `insert-no-created`
      (goes last in the block), `insert-created-last-line`,
      `leading-blank-line`, `same-value` (output byte-identical to input),
      and one fixture per built-in type's real template output so the
      insert lands where a real document expects it.
- [ ] Error-table test: CRLF → `ErrUnsupportedLineEndings`; no frontmatter
      → `document.ErrNoFrontmatter`; block scalar / flow mapping / flow
      sequence / anchor / alias → `ErrUpdatedFieldUnsupported` **and the
      file on disk is byte-identical afterward**; unterminated quote →
      `ErrUpdatedFieldUnsupported`; unreadable path → wrapped IO error naming
      the path.
- [ ] `FuzzSetUpdated` pinning: never panics; when it returns nil the output
      contains exactly one `^updated:` line inside the frontmatter block; and
      when `document.ParseFrontmatter(input)` succeeded,
      `ParseFrontmatter(output)` succeeds with `Updated == date` and every
      other field unchanged. If the fuzzer finds a YAML shape where a
      column-0 insert breaks the parse, **refuse with
      `ErrUpdatedFieldUnsupported`** rather than weaken the invariant.
- [ ] Package doc (`doc.go`): "three operations" becomes four; describe the
      upsert and why it is not a general editor. Godoc on every new exported
      symbol.
- [ ] `make fmt`, `make lint`, `make ci` green; go-review pass on the diff.
<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `git diff --stat pkg/doczcore/docwrite/testdata/golden/status/` is empty
  after the refactor — the locator generalization is provably
  byte-preserving.
- `TestSetStatus_*` pass unchanged; `TestSetUpdated_Golden`, the error
  table, and `FuzzSetUpdated` (≥ 1M execs locally, seed corpus committed)
  green.
- A document written by `SetUpdated` round-trips through
  `document.ParseFrontmatter` with `Updated` set and no other field changed.
- `make ci` green.
<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 2: read side and the `updated:` config block

`document.Frontmatter.Updated`, `config.UpdatedConfig`, defaults, the
generated-config template, and the JSON data field in `docz list`. Nothing
here changes runtime behavior; it is the schema Phases 4 and 5 build on.

<!--docz:tasks:start-->
#### Tasks

- [ ] `document.Frontmatter` gains `Updated string \`yaml:"updated"\`` with a
      doc comment stating the `YYYY-MM-DD` raw-string contract and the
      `""`-means-unknown rule (DESIGN-0012 §The field). `DocEntry` inherits.
      Tests: present, absent → `""`, quoted, and unknown-key leniency still
      holds.
- [ ] `config.UpdatedConfig{Enabled bool}` with `yaml:"enabled"
      json:"enabled"`; `Config.Updated UpdatedConfig` with `yaml:"updated"
      json:"updated"`, placed after `API`. `DefaultConfig()` sets
      `Enabled: false`. No `mapstructure` tags (vestigial post-viper).
      Nothing to normalize or validate — `Validate()` is untouched.
- [ ] Add the block to `internal/template/templates/docz_yaml.tmpl` after
      `api:`, rendered from `DefaultConfig()` with the DESIGN-0012 §Config
      comment (needs git history, rewrites every document once). **Do this
      in the same commit as the struct:**
      `TestDoczYAMLTemplate_RoundTripsToDefaultConfig` fails the moment a
      default field has no template line (IMPL-0015 / IMPL-0016 precedent).
- [ ] Add the block to `.docz.example.yaml` with the same comment, and to
      this repo's `.docz.yaml` as `enabled: false` (enabling is Phase 5's
      dogfood step).
- [ ] Config tests: decode (full / `enabled` only / absent); defaults; merge
      (repo `updated.enabled` wins over global); dormancy — a config with the
      block enabled **and** disabled both pass `Validate()` since there are
      no rules; `parity_baseline_test.go` case pinning that a dormant
      `updated:` block now decodes rather than being ignored. Confirm
      `TestJSONTags_MirrorYAML` and `TestConfigJSON_MarshaledShape` cover the
      new struct — the shape pin needs its expected JSON extended by one
      block.
- [ ] `docz list`: `listEntry.Updated string \`json:"updated,omitempty"\``
      populated from `doc.Updated`; JSON output only in this phase (the text
      and CSV columns are layout and land in Phase 4). Test: a doc with the
      field emits it; one without omits the key.
- [ ] Godoc on every new exported symbol; `make fmt`, `make lint`,
      `make ci` green; go-review pass.
<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `docz config` prints the resolved `updated:` block with no `cmd/` change.
- `docz init` output contains the disabled block with its comment, and
  round-trips to `DefaultConfig()`.
- `TestJSONTags_MirrorYAML` green with the new struct, and
  `TestConfigJSON_MarshaledShape` pins `"updated": {"enabled": …}`.
- `docz list --format=json` on a fixture with `updated:` emits the key; on
  one without, omits it.
- `make ci` green.
<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 3: git resolver and the stream parser

Everything that talks to git, in `cmd/` — plus the pure `git log -p`
stream parser in its own `internal/gitlog` package (Decision 1). Built before the CLI
integration so Phase 4 is wired against a tested resolver and a scriptable
stub.

<!--docz:tasks:start-->
#### Tasks

- [ ] go-architect pass before the new package (Decision 1: `internal/gitlog`):
      confirm the parser's signature and that it is bytes-in/values-out with
      no `exec` inside it.
- [ ] Stream parser `gitlog.FirstSubstantiveCommit(r io.Reader, ignore
      *regexp.Regexp) (Commit{SHA, Date string}, ok bool, err error)` over
      `git log --format=%x00%H%x09%cs -p` output: split on the NUL record
      marker; for each commit scan patch lines; a line is a *change* when it
      starts with `+` or `-` and is not a `+++ ` / `--- ` file header; the
      commit is substantive when any change line's text after the marker
      does **not** match `ignore` (`^updated:`); return the first
      substantive commit. Pure-rename commits (no change lines) are skipped.
      `\ No newline at end of file` lines are ignored. Truncated input never
      panics; empty input → `ok == false`.
- [ ] Parser tests (parallel, table-driven) on committed fixtures under
      `testdata/`: a real `git log -p` capture from this repo (DESIGN-0011's
      two-commit history); synthetic cases — first commit substantive;
      first commit `updated:`-only → second returned; `updated:`-only
      insertion (`+updated: …` with no `-` line) skipped; `status:` flip
      counted as substantive; pure rename skipped; headers and
      `\ No newline` ignored; empty; truncated mid-record. `FuzzFirstSubstantiveCommit`
      pins never-panic.
- [ ] `cmd/git.go`: add `WorkTreeState` (`StateUnavailable`, `StateClean`,
      `StateModified`, `StateUntracked`), `Change{SHA, Date}`, and grow
      `GitResolver` with `Shallow(ctx) (bool, error)`, `State(ctx, path)
      WorkTreeState`, `LastChange(ctx, path) (Change, bool, error)`. Doc
      comments carry the DESIGN-0012 §Git resolver command table.
- [ ] `realGit` gains a `Dir string` (the repo root every command runs in)
      and implements the three methods with `exec.CommandContext`:
      `rev-parse --is-shallow-repository`; `status --porcelain
      --untracked-files=all -- <path>` (empty → Clean, `??` → Untracked,
      else Modified; exit 128 / missing binary → Unavailable); `log --follow
      --format=%x00%H%x09%cs -p -- <path>` streamed through the parser with
      early stop (Decision 7: once the parser returns, close the process's
      stdout and `Wait`, treating the broken-pipe exit as success). Wire
      `Dir` in `loadAndValidateConfig` where
      `RepoRoot` is resolved, so `NewRunner`'s default `realGit{}` still
      means "cwd".
- [ ] Route the path handed to git through the same resolution as the file
      path: git runs in `RepoRoot`, so pass the doc path relative to
      `RepoRoot` (`filepath.Rel` when the path is absolute). Add one test
      with `RepoRoot ≠ cwd` — the pre-existing looseness DESIGN-0012 calls
      out, closed only as far as this feature needs.
- [ ] `staticGit` becomes scriptable **without breaking the ~20 existing
      `staticGit{Name: …}` / `staticGit{}` literals**: add `ShallowFn func()
      (bool, error)`, `StateFn func(path string) WorkTreeState`,
      `LastChangeFn func(path string) (Change, bool, error)`, and a `Calls
      []string` recorder (method name + path) for the dormancy proof. Nil
      functions return `(false, nil)`, `StateUnavailable`, and
      `(Change{}, false, nil)`.
- [ ] Real-git integration test, one file (`cmd/git_integration_test.go`),
      guarded by `exec.LookPath("git")` and hermetic: `GIT_CONFIG_GLOBAL=/dev/null`,
      `GIT_CONFIG_NOSYSTEM=1`, `GIT_AUTHOR_DATE` / `GIT_COMMITTER_DATE`
      pinned with an explicit offset, `-c user.name -c user.email -c
      commit.gpgsign=false -c init.defaultBranch=main`. Scenarios: `State`
      classifies clean / modified / untracked (and a file added but not yet
      committed → Modified); `LastChange` returns the pinned committer date;
      an `updated:`-only follow-up commit is skipped; `git mv` + a content
      edit is followed across the rename; `Shallow` is false; and
      (Decision 3) a `git clone --depth 1 file://…` of that repo reports
      `Shallow` true, with the update pass run against it printing the
      single warning and writing nothing.
- [ ] Context-cancellation tests for the three new `realGit` methods,
      matching `TestRealGit_UserName_CtxCancel`.
- [ ] Godoc; `make fmt`, `make lint`, `make ci` green; go-review pass.
<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- Parser table + fuzz green in parallel; the captured real-history fixture
  returns `004491f…` / `2026-08-30` for DESIGN-0011 (its latest commit is a
  `status:` flip and therefore substantive — the filter is narrow by design).
- Integration test green locally and in the `Test Go` CI job (Ubuntu runner
  git ≥ 2.21 for `%cs`).
- Every pre-existing `cmd/` test compiles and passes with the grown
  interface and stub.
- `make ci` green.
<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 4: CLI integration

The stamp pass, the two command-time stamps, and the enabled-only layouts.
This is where DESIGN-0012's convergence table becomes a test table.

<!--docz:tasks:start-->
#### Tasks

- [ ] `Runner.Update`: when `r.Cfg.Updated.Enabled`, probe git **once per
      invocation**: `State` unavailable → debug log, pass disabled; `Shallow`
      true → exactly one warning on `r.Err` (`Warning: shallow clone;
      skipping updated-field pass (clone with full history, e.g.
      fetch-depth: 0)`), pass disabled. Carry the result into `updateType`
      as a small `stampGate` value rather than a Runner field.
- [ ] `Runner.updateType`: after the ToC pass and before
      `index.GenerateTable`, call `r.stampUpdated(typeDir, docs, dryRun,
      gate)` implementing DESIGN-0012 §The stamping model: `State` → today
      (`r.Now().Format(time.DateOnly)`, the local date — Decision 8) for
      Modified/Untracked, `LastChange`
      date for Clean, skip with debug when no history; compare against the
      scan's parsed `doc.Updated` (cmd owns the no-op short-circuit); on a
      difference, dry-run prints `Would set updated in <path>: <old|(absent)>
      -> <want>`, otherwise `docwrite.SetUpdated`. Copy the new value into
      `docs[i].Updated` so the table rendered next sees it. Silent on success
      with a debug log per document (Decision 7); write errors are warnings
      on `r.Err` and the pass continues.
- [ ] `index.GenerateTable(docs, heading, layout index.Layout)` with
      `Layout{Updated bool}`: zero value renders today's header
      byte-for-byte; `Updated: true` renders `| ID | Title | Status | Created
      | Updated | Author | Link |` (Decisions 5 and 6). `updateType` passes
      `Layout{Updated: r.Cfg.Updated.Enabled}`. Existing `TestGenerateTable_*`
      pass unchanged; add the enabled-layout cases.
- [ ] `docz create`: when enabled, after `docwrite.Create`, call
      `SetUpdated(result.FilePath, today)`; `--no-update` does **not** skip
      it (creation metadata, not index maintenance). The `Created …` output
      line is unchanged.
- [ ] `docz status set`: when enabled and `res.changed && !opts.dryRun`,
      call `SetUpdated(docPath, today)` after a successful `SetStatus`; a
      failure goes through `statusWriteError` (exit 1). `statusResult` and
      `statusJSON` gain `Updated string \`json:"updated,omitempty"\``; the
      text line is unchanged (Decision 4). `--quiet` behavior unchanged.
- [ ] `docz list`: `outputTable` and `outputCSV` gain an `UPDATED` column
      after `DATE` only when enabled (Decision 5 — the three formats agree).
      JSON is done (Phase 2).
- [ ] cmd tests for the pass with the scripted stub, one subtest per row of
      DESIGN-0012's convergence table: same-day edit; create-then-clean;
      status-set-then-clean; commit-without-update (git-derived write);
      backfill of a multi-doc fixture; day-boundary (one write). Each runs
      the pass **twice** and asserts the second run writes nothing (file
      bytes identical, dry-run prints nothing).
- [ ] The dormancy proof: with the block disabled, `docz update`, `create`,
      and `status set` make **zero** `GitResolver` calls beyond `UserName`
      (assert `staticGit.Calls`), write no `updated:` line, and render the
      pre-existing README layout byte-for-byte.
- [ ] Guard tests: shallow → one warning, no writes, README still updated;
      git unavailable → silent skip; write error on one doc → warning, other
      docs still stamped; `docz update --dry-run` lines pinned; `docz update
      <type>` stamps only that type; table write-through (the README rendered
      in the same run shows the new date).
- [ ] `create` / `status set` tests: enabled → `updated:` equals the pinned
      `Now` date; disabled → absent; `create --no-update` still stamps;
      `status set --dry-run` and same-value no-op stamp nothing; JSON shape
      with and without `updated`.
- [ ] Godoc; `make fmt`, `make lint`, `make ci` green; go-review pass over
      the whole phase (the largest diff in the plan).
<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- All convergence-table subtests green, each proving a zero-write second
  run.
- Dormancy proof green: a disabled repo is byte-identical in every file and
  every README after `docz update`, `create`, and `status set`.
- `docz update --dry-run` on a fixture prints exactly the pinned lines and
  nothing else; a real run prints only the README lines.
- `cmd/` tests remain serial; `internal/*` and `pkg/*` tests remain parallel.
- `make ci` green.
<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 5: living docs, consumer contract, release, dogfood

Ships `v1.3.0`, then flips statuses and enables the block in this repo.

<!--docz:tasks:start-->
#### Tasks

- [ ] README: the Features bullet lists `updated`; the Configuration example
      gains the block; new `### Updated` subsection after `### API` covering
      the semantics (edit-time stamp vs committer date, the substantive-commit
      rule), the one-time backfill diff on enabling, the shallow-clone
      warning and `fetch-depth: 0`, that `create` / `status set` stamp too,
      and the `Created` / `Updated` table layout; `docz update` Flags section
      notes the pass; "Using docz as a Go Library" table gains `SetUpdated`;
      Index Tables section notes the enabled layout.
- [ ] CLAUDE.md architecture map: `docwrite` (SetUpdated, upsert rules),
      `document` (Updated field), `config` (UpdatedConfig, dormancy), `cmd/`
      (stamp pass, GitResolver growth, stub shape), `internal/index`
      (Layout), and `internal/gitlog` (Decision 1).
- [ ] DESIGN-0008: add R12 verbatim from DESIGN-0012 §Consumer contract with
      `v1.3.0` filled in, and extend the "R10 raises that pin…" sentence.
- [ ] `test/consumer/consumer_v13_test.go` (one file per release): parse a
      document carrying `updated:` and assert `Frontmatter.Updated`; decode
      `updated: {enabled: true}` into `config.Config`; call
      `docwrite.SetUpdated` on a temp file and assert `errors.Is(err,
      docwrite.ErrUpdatedFieldUnsupported)` on a block-scalar fixture.
      Verify it goes **red** when `Frontmatter.Updated` is temporarily
      renamed, then restore — this module sits outside root `./...`, so only
      `make test-consumer` / `make ci` catches it.
- [ ] Open the release PR with the `minor` label carrying Phases 1–5, a
      `### RELEASE NOTES` section (new field, block, `SetUpdated`, the
      enabled-only layout, the R12 pin, and that `v1.2.2` already changed
      `config_snapshot` spellings for anyone bumping from `v1.2.1`). Merge
      when green; `pr-semver-bump` + goreleaser cut `v1.3.0`.
- [ ] After the tag: fix the release body with the extracted notes (`gh
      release edit`), since goreleaser overwrote them on `v1.2.0` and
      `v1.2.2`; verify `go list -m github.com/donaldgifford/docz@v1.3.0`
      resolves from the proxy.
- [ ] Post-tag `dont-release` PR (Decision 6 — one bookkeeping PR carrying
      both the flips and the backfill): flip DESIGN-0012 → Implemented and
      IMPL-0017 → Completed; set `updated: enabled: true` in this repo's
      `.docz.yaml`; run `docz update` and commit the backfill (every
      document gains `updated:`, every README re-renders with the
      `Created` / `Updated` layout); run `docz update --dry-run` again on
      the result and confirm it prints nothing — the convergence proof on
      real history.
- [ ] Comment on docz-api #36 with the tag, the R12 text, and the
      `config_snapshot` key; note on claude-skills #95 that the config and
      workflow references need the block (templates unchanged).
- [ ] `make ci` green at the end; go-review pass on the docs diff.
<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `v1.3.0` tagged, published with the real release notes, and resolvable
  from `proxy.golang.org`.
- `make test-consumer` green against the promoted surface, and proven red
  on a rename.
- DESIGN-0012 Implemented, IMPL-0017 Completed, this repo enabled and
  converged (`docz update --dry-run` silent on `main`).
- docz-api #36 unblocked with everything it needs to start.
<!--docz:criteria:end-->
<!--docz:phase:end-->

<!--docz:file-changes:start-->
## File Changes

| File | Action | Description |
| ---- | ------ | ----------- |
| `pkg/doczcore/docwrite/status.go` | Modify | `findScalarValue` / `parseScalarValue` generalization; `SetStatus` becomes a wrapper |
| `pkg/doczcore/docwrite/updated.go` | Create | `SetUpdated`, `ErrUpdatedFieldUnsupported`, `insertFrontmatterLine` |
| `pkg/doczcore/docwrite/updated_test.go` | Create | Golden harness, error table, `FuzzSetUpdated` |
| `pkg/doczcore/docwrite/testdata/golden/updated/` | Create | Input/output fixtures |
| `pkg/doczcore/docwrite/doc.go` | Modify | Four operations, upsert rationale |
| `pkg/doczcore/document/document.go` | Modify | `Frontmatter.Updated` |
| `pkg/doczcore/document/document_test.go` | Modify | Field parse cases |
| `pkg/doczcore/config/config.go` | Modify | `UpdatedConfig`, `Config.Updated`, default |
| `pkg/doczcore/config/config_test.go`, `parity_baseline_test.go`, `json_test.go` | Modify | Decode / merge / dormancy / parity / shape pin |
| `internal/template/templates/docz_yaml.tmpl` | Modify | Dormant `updated:` block |
| `.docz.example.yaml`, `.docz.yaml` | Modify | The block (disabled until Phase 5) |
| `internal/gitlog/gitlog.go` (+ tests, testdata) | Create | `git log -p` stream parser (Decision 1) |
| `cmd/git.go` | Modify | `WorkTreeState`, `Change`, grown `GitResolver`, `realGit.Dir`, scriptable `staticGit` |
| `cmd/git_test.go`, `cmd/git_integration_test.go` | Modify / Create | Stub tests; hermetic real-git test |
| `cmd/root.go` | Modify | Wire `realGit{Dir: repoRoot}` |
| `cmd/update.go` | Modify | Once-per-run probe, `stampUpdated`, layout selection |
| `cmd/create.go`, `cmd/status.go`, `cmd/list.go` | Modify | Command-time stamps; JSON/text/CSV columns |
| `cmd/update_test.go`, `create_test.go`, `status_test.go`, `list_test.go` | Modify | Convergence table, dormancy proof, guards, shapes |
| `internal/index/index.go` (+ test) | Modify | `Layout` parameter, enabled header |
| `test/consumer/consumer_v13_test.go` | Create | R12 proof |
| `README.md`, `CLAUDE.md` | Modify | Living docs |
| `docs/design/0008-*.md` | Modify | R12 clause |
<!--docz:file-changes:end-->

<!--docz:testing:start-->
## Testing Plan

- [ ] `docwrite`: status goldens byte-identical; updated goldens; error
      table with on-disk-unchanged assertions; `FuzzSetUpdated`
- [ ] `document`: `Updated` present / absent / quoted; leniency unchanged
- [ ] `config`: decode / defaults / merge / dormancy / parity; json tag and
      shape pins extended
- [ ] `internal/gitlog`: parser table on real + synthetic captures; fuzz
- [ ] `cmd`: convergence rows (each with a zero-write second run); dormancy
      proof via `staticGit.Calls`; shallow / unavailable / write-error
      guards; dry-run pins; `create` and `status set` stamps; layout tests
- [ ] Real git: hermetic `git init` integration test incl. rename and
      shallow clone
- [ ] `test/consumer`: R12 proof, verified red on rename
- [ ] Dogfood: `docz update --dry-run` silent on `main` after the post-tag PR
- [ ] `make ci` green at the end of every phase
<!--docz:testing:end-->

<!--docz:dependencies:start-->
## Dependencies

- **DESIGN-0012** — approved; all eight decisions locked 2026-09-12
- **INV-0008** — Concluded; the convergence analysis and Decisions 1–6
- **`v1.2.2`** — this branch's base; nothing else is in flight, so the next
  `minor` is `v1.3.0` (the R12 text assumes it)
- **git ≥ 2.21** on developer machines and CI runners (`%cs`); no new Go
  module dependencies — `go.mod` stays cobra + yaml
- **docz-api #36** — downstream, blocked on the tag; nothing here waits on it
<!--docz:dependencies:end-->

<!--docz:open-questions:start-->
## Open Questions

All eight resolved **(a)** on 2026-09-12; see [Decisions](#decisions). The
options are kept for the alternatives, which record what was weighed. Each
question lists the recommended option first as **(a)**.

### 1. Where does the `git log -p` stream parser live?

**Resolved: (a)** — locked 2026-09-12; see [Decisions](#decisions).

The parser is pure (reader in, `Commit` out, no `exec`), the only
non-trivial logic in the git work, and the thing most worth fuzzing.

- **a. (Recommended)** A new `internal/gitlog` package. Bytes-in/values-out
  like the rest of the module, parallel tests (`cmd/` tests must stay
  serial), a fuzz target, and `cmd/git.go` stays what it is today — process
  plumbing. Private, so it adds nothing to the semver surface.
- b. In `cmd/` next to `realGit`. One fewer package, but the parser's tests
  join the serial `cmd/` suite and the fuzz target sits beside Cobra globals.
- c. In `pkg/doczcore`. Rejected by construction: the public core is
  git-free (ADR-0001 Decision 5), and the parser only exists because of git.
- d. Other.

### 2. PR and release strategy?

**Resolved: (a)** — locked 2026-09-12; see [Decisions](#decisions).

- **a. (Recommended)** One `minor` PR carrying all five phases, phases as
  commit boundaries, then the post-tag `dont-release` PR — the IMPL-0015 and
  IMPL-0016 shape (#78 → `v1.1.0` → #79; #84 → `v1.2.0` → #90), which has
  now worked cleanly twice. Review still reads phase by phase.
- b. Per-phase `dont-release` PRs with a final `minor`. More checkpoints,
  five times the CI and merge overhead, and the stacked-PR auto-close gotcha
  to manage.
- c. Two releases: Phases 1–2 as a `minor` (the library surface), Phases 3–5
  as a second `minor` (the CLI). Would ship a public `SetUpdated` and
  `Frontmatter.Updated` that nothing populates — a contract without a
  producer.
- d. Other.

### 3. Does the real-git integration test cover the shallow-clone path?

**Resolved: (a)** — locked 2026-09-12; see [Decisions](#decisions).

- **a. (Recommended)** Yes — `git clone --depth 1 file://<tmp repo>` inside
  the same test, assert `Shallow()` is true there, and run the update pass
  against it to see the single warning and zero writes end to end. It is
  the one failure mode that writes *plausible* wrong dates silently, so it
  deserves a real-git proof, not just a stub.
- b. Stub only: `staticGit.ShallowFn` returns true in a `cmd/update` test.
  Cheaper, but never exercises `rev-parse --is-shallow-repository` for real.
- c. Other.

### 4. Should `docz update` get a flag to skip the stamp pass?

**Resolved: (a)** — locked 2026-09-12; see [Decisions](#decisions).

- **a. (Recommended)** No flag. The config block is the switch, a shallow
  clone already skips with a warning, and `--dry-run` shows what would
  happen. Every flag is contract; add one when a user needs it, not before.
- b. Add `--no-updated` (or `--no-stamp`) for one-off runs — e.g. a CI drift
  check that wants a quiet run on a shallow clone.
- c. An environment variable (`DOCZ_NO_STAMP=1`) for CI without touching the
  CLI contract.
- d. Other.

### 5. Does the `Updated` column also appear in `docz list --format=csv`?

**Resolved: (a)** — locked 2026-09-12; see [Decisions](#decisions).

DESIGN-0012 specifies text (enabled-only) and JSON (`omitempty`); CSV was
not mentioned.

- **a. (Recommended)** Yes, under the same enabled-only rule as the text
  table, so the three formats agree on what a document has.
- b. CSV unchanged; JSON is the machine-readable path and CSV consumers get
  a stable column set.
- c. Other.

### 6. Where does the dogfood backfill in this repo land?

**Resolved: (a)** — locked 2026-09-12; see [Decisions](#decisions).

Enabling the block here rewrites every document (35+ files) and every
README table.

- **a. (Recommended)** In the same post-tag `dont-release` PR as the status
  flips — one bookkeeping PR per release, matching DESIGN-0012's rollout
  item 4 and the #79 / #90 pattern. The diff is large but mechanical and
  reviewable by `docz update --dry-run` being silent afterward.
- b. A separate PR so the backfill reviews alone and the status flips stay
  a two-line change.
- c. Other.

### 7. Does the resolver stop reading `git log -p` early?

**Resolved: (a)** — locked 2026-09-12; see [Decisions](#decisions).

A full history is unbounded, but the answer is almost always in the first
or second commit.

- **a. (Recommended)** Stop early: once the parser returns, close the
  process's stdout and `Wait`, treating the resulting broken-pipe exit as
  success. Cost stays proportional to the answer, not the history.
- b. Read to EOF. Simplest process handling — no special-cased exit status
  — with cost bounded by history size (8 ms for 594 lines here). Fine
  today; grows with every commit to every document.
- c. Cap with `-n 50` and fall back to an uncapped read when nothing
  substantive is found. Bounded and simple, at the cost of a rare second
  process.
- d. Other.

### 8. What clock does "today" use for an edit-time stamp?

**Resolved: (a)** — locked 2026-09-12; see [Decisions](#decisions).

- **a. (Recommended)** The local date from `r.Now()` — the same source and
  zone `created` already uses, and the same convention as git's `%cs`,
  which reports the committer's local date. A developer stamping at 23:00
  local and committing at 23:30 gets the same date from both paths.
- b. UTC. Deterministic across machines, but disagrees with both `created`
  and `%cs` near midnight, which manufactures exactly the one-write churn
  case Decision 3 tolerates only as rare.
- c. Other.
<!--docz:open-questions:end-->

<!--docz:decisions:start-->
## Decisions

All eight open questions resolved **(a)** on 2026-09-12.

| #   | Question                          | Resolution                                                                                                                                     |
| --- | --------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | Stream parser home                | New `internal/gitlog` package — pure, parallel tests, fuzzable, private; `cmd/git.go` stays process plumbing                                    |
| 2   | PR / release strategy             | One `minor` PR carrying all five phases as commit boundaries → `v1.3.0`, then a post-tag `dont-release` PR (the #84 → #90 shape)               |
| 3   | Shallow-clone integration proof   | Yes — `git clone --depth 1 file://…` in the real-git test; assert `Shallow()` true and the pass warns once and writes nothing                  |
| 4   | Skip flag on `docz update`        | No flag — the config block is the switch; a shallow clone already skips with a warning; `--dry-run` previews                                   |
| 5   | CSV `Updated` column              | Yes, under the same enabled-only rule as the text table, so text, CSV, and JSON agree                                                           |
| 6   | Dogfood backfill placement        | Same post-tag `dont-release` PR as the status flips — one bookkeeping PR per release, verified by a silent `docz update --dry-run` afterward   |
| 7   | Early stop on `git log -p`        | Stop early — close the process's stdout after the first substantive commit and `Wait`, treating the broken-pipe exit as success                |
| 8   | Clock for edit-time stamps        | Local date from `r.Now()` — the same source and zone as `created`, matching git's committer-local `%cs`                                          |
<!--docz:decisions:end-->

<!--docz:references:start-->
## References

- [DESIGN-0012](../design/0012-updated-frontmatter-field-opt-in-stamp-pass-in-docz-update.md)
  — the design this implements; Decisions 1–8
- [INV-0008](../investigation/0008-last-updated-frontmatter-field-scope-git-semantics-and-effort.md)
  — scoping; Observation 6 (convergence), Observation 9 (sizing)
- IMPL-0016 / IMPL-0015 — the phase, PR, and post-tag flip shape this
  follows
- IMPL-0011 / DESIGN-0005 — `SetStatus`, whose locator and goldens Phase 1
  generalizes without changing
- DESIGN-0008 — R7, R10, R11 and the R12 clause Phase 5 adds
- DESIGN-0004 §A / §H — `Runner` and the `GitResolver` seam
- ADR-0001 Decision 5 — bytes-in/values-out; narrow write helpers
- [docz-api #36](https://github.com/donaldgifford/docz-api/issues/36) — the
  downstream follow-up blocked on `v1.3.0`
- claude-skills #95 — plugin references to update after release
<!--docz:references:end-->
