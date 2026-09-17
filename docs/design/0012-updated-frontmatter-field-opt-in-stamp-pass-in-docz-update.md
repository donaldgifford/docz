---
id: DESIGN-0012
title: "Updated frontmatter field: opt-in stamp pass in docz update"
status: Draft
author: Donald Gifford
created: 2026-09-12
---

<!-- markdownlint-disable-file MD025 MD041 -->

# DESIGN-0012: Updated frontmatter field: opt-in stamp pass in docz update

<!--toc:start-->
- [Overview](#overview)
- [Goals and Non-Goals](#goals-and-non-goals)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Background](#background)
- [Detailed Design](#detailed-design)
  - [The field](#the-field)
  - [The stamping model](#the-stamping-model)
  - [Config](#config)
  - [docwrite.SetUpdated](#docwritesetupdated)
  - [Git resolver](#git-resolver)
  - [The update pass](#the-update-pass)
  - [docz create](#docz-create)
  - [docz status set](#docz-status-set)
  - [Index table and docz list](#index-table-and-docz-list)
  - [Consumer contract — DESIGN-0008 R12](#consumer-contract--design-0008-r12)
- [API / Interface Changes](#api--interface-changes)
- [Data Model](#data-model)
- [Testing Strategy](#testing-strategy)
- [Migration / Rollout Plan](#migration--rollout-plan)
- [Open Questions](#open-questions)
  - [1. What is the config block called?](#1-what-is-the-config-block-called)
  - [2. How does a new document get the field?](#2-how-does-a-new-document-get-the-field)
  - [3. Does a clean document's stored stamp ever get "corrected" to the committer date?](#3-does-a-clean-documents-stored-stamp-ever-get-corrected-to-the-committer-date)
  - [4. Should docz status set stamp updated, and how is it reported?](#4-should-docz-status-set-stamp-updated-and-how-is-it-reported)
  - [5. Does the README index table gain the Updated column unconditionally, or only when enabled?](#5-does-the-readme-index-table-gain-the-updated-column-unconditionally-or-only-when-enabled)
  - [6. Is the existing Date column renamed Created when Updated appears?](#6-is-the-existing-date-column-renamed-created-when-updated-appears)
  - [7. What does a normal (non-dry-run) docz update print for the pass?](#7-what-does-a-normal-non-dry-run-docz-update-print-for-the-pass)
  - [8. How narrow is the "bookkeeping commit" filter?](#8-how-narrow-is-the-bookkeeping-commit-filter)
- [Decisions](#decisions)
- [References](#references)
<!--toc:end-->

## Overview

Add a sixth frontmatter field, `updated: YYYY-MM-DD`, recording the date a
docz document's content last changed, and an opt-in `updated:` config block
that makes `docz update` keep it current. The value comes from git when a
document is clean and from the clock when it is dirty, which is the
combination INV-0008 showed converges without generating bookkeeping commits.
The field is additive for consumers (docz-api gains a real "last updated"
where today it only has an ingest timestamp) and lands via a new
purpose-built `docwrite.SetUpdated` — the package's first upsert, kept
deliberately narrow. The commit sha is explicitly out of scope; INV-0008
Decision 2 already fixed its future shape (`updated_sha:`) so a follow-up
does not reopen it.

## Goals and Non-Goals

### Goals

- Every docz document in an opted-in repo carries `updated: YYYY-MM-DD`,
  populated for existing documents on the first `docz update` after enabling
  and kept current thereafter.
- `docz update` run twice in a row writes nothing the second time — the
  pass **converges** — and a normal edit → `docz update` → commit flow never
  produces a trailing "docz bookkeeping" commit.
- A repo that has not enabled the block sees **no change** in any file,
  output, or git access (the dormancy rule, DESIGN-0010 Decision 7).
- The field is a public, semver-governed part of `document.Frontmatter`,
  ratified for docz-api as DESIGN-0008 R12, and reaches docz-api without a
  checkout — the reason it lives in the file at all (INV-0008 Observation 8).
- Wrong values are never written silently: shallow clones, uncommitted
  edits, untracked files, and a missing git are each classified and
  handled, not guessed at.
- `docwrite` gains exactly one new operation, `SetUpdated`, with the same
  byte-preservation and LF-only contract as `SetStatus`.

### Non-Goals

- **No commit sha.** Deferred to a follow-up by INV-0008's recommendation;
  its shape (`updated_sha:`, a separate key) is decided, its cost (one
  trailing commit per content commit, by construction) is recorded in
  INV-0008 Observation 6, and nothing here should be designed around it.
- No general frontmatter editor. `SetUpdated` is purpose-named; there is no
  `SetScalar(path, key, value)`.
- No per-type key name (`updated_field`) — INV-0008 Decision 3.
- No `created` changes, no mtime, no pre-commit hook, no `git` dependency
  in `pkg/doczcore` (git stays in `cmd/`, ADR-0001 Decision 5).
- No backfill on the docz-api side; snapshots refresh on each repo's next
  ingest, as with every prior contract addition.
- Not a change to `toc:` or `index:` behavior beyond the `Updated` column.

## Background

Every docz document carries `id`, `title`, `status`, `author`, `created`.
Nothing records when it last changed. The README index tables and
`docz list` show `created` only; docz-api's `documents.updated_at` is the
ingest time and its `git_sha` is the blob sha — neither is "when did this
document last change" (INV-0008 Context).

INV-0008 scoped the feature and settled six questions:

| #   | Decision                                                                                  |
| --- | ----------------------------------------------------------------------------------------- |
| 1   | Field name `updated`, symmetric with `created`                                            |
| 2   | Sha, when it comes, is a separate `updated_sha:` key — deferred with the sha itself       |
| 3   | No per-type key override                                                                  |
| 4   | Git-derived dates use the **committer** date                                              |
| 5   | Shallow clone: warn once and skip the pass                                                |
| 6   | `Updated` column in the README index tables, same release (refined by Decision 5 below: only when enabled) |

Its central finding shapes everything here: a value derived from git history
cannot describe the commit that contains it, so naive stamping never
converges. A **date** escapes this because it can be stamped before the
commit exists; any git-derived path additionally needs a filter that ignores
commits whose only change to the file is the `updated:` line itself. That
filter cannot be expressed in `git log -G` (POSIX ERE, no lookahead) and is
done in Go.

Prior art this design leans on:

- **DESIGN-0005 / IMPL-0011** — `SetStatus`, the byte-level frontmatter
  mutator whose locator machinery generalizes to `SetUpdated`, and whose
  "cmd owns the no-op short-circuit" rule (Decision 8) carries over.
- **DESIGN-0010 Decision 7** — the dormancy pattern: an opt-in block is inert
  until `enabled: true`, so a repo can commit the block before turning it on.
- **DESIGN-0004 §A/§H** — the `Runner` and its `GitResolver` seam, which is
  where the git work goes.
- **DESIGN-0008 R7, R10, R11** — the frontmatter column contract and the
  R-series pattern for ratifying a new surface.

## Detailed Design

### The field

```yaml
---
id: DESIGN-0012
title: "Updated frontmatter field: opt-in stamp pass in docz update"
status: Draft
author: Donald Gifford
created: 2026-09-12
updated: 2026-09-12
---
```

- **Key:** `updated`, bare scalar, `YYYY-MM-DD`, same shape as `created`.
- **Meaning:** the date the document's content last changed, as best docz
  can determine — the clock date when docz stamps an uncommitted edit, the
  committer date of the last substantive commit otherwise.
- **Absent** means unknown. It is absent in every repo that has not enabled
  the block, and in an enabled repo only for a document docz could not
  classify (no git history and not on disk as dirty — in practice, never).
- **Raw string** on the Go side (`Frontmatter.Updated string`), matching
  `Created` and DESIGN-0010 Decision 2: no timezone opinion, consumers parse.

### The stamping model

For each document in a type directory, `docz update` computes a *wanted*
value and writes it only if it differs from the value the document holds:

```text
state := git.State(path)                      # Clean | Modified | Untracked | Unavailable
switch state:
  Untracked, Modified:  want = today (r.Now(), local date)
  Clean:                chg, ok := git.LastChange(path)   # last SUBSTANTIVE commit
                        if !ok: skip (debug log)         # no history — cannot happen for a clean file
                        want = chg.Date                   # committer date, YYYY-MM-DD
  Unavailable:          skip the whole pass (debug log)
if doc.Updated == want: no-op
else if dryRun:         print "Would set updated in <path>: <old|(absent)> -> <want>"
else:                   docwrite.SetUpdated(path, want)
```

**Substantive commit.** A commit counts as the document's last change only
if its diff to the file touches at least one line other than the
`updated:` frontmatter line. Walking `git log -p` newest-first and stopping
at the first such commit is what lets the pass converge: the commit that
lands a stamp is, for that file, an `updated:`-only diff and is skipped.
The filter is deliberately narrow — a one-line `status:` flip is a real
update (INV-0008 Observation 6 showed exactly that diff for DESIGN-0011).

**Why it converges.** The cases that matter, from INV-0008 Observation 6:

| Flow                                                             | First run             | Commit           | Next run                                                                |
| ---------------------------------------------------------------- | --------------------- | ---------------- | ----------------------------------------------------------------------- |
| Edit, `docz update`, commit (same day)                           | dirty → today         | stamp rides along | clean; last substantive commit is today's → equal → **no write**       |
| `docz create` (stamped at birth), commit                         | —                     | stamp rides along | same as above → **no write**                                            |
| `docz status set`, commit                                        | stamped by the command | stamp rides along | same → **no write**                                                     |
| Edit, commit *without* `docz update`, run later                  | clean → commit date   | trailing commit `B` | `B` is `updated:`-only → skipped; original commit → equal → **no write** |
| Enable in an existing repo (all docs clean)                      | git-derived backfill, N writes | one commit | that commit is `updated:`-only per file → skipped → **no write**   |
| Edit day 1 + `docz update`, commit day 2                         | dirty → day 1         | stamp rides along | clean; committer date is day 2 ≠ day 1 → **one write** (Decision 3)    |

The last row is the only residual churn and it is bounded: one trailing
commit, once, in the uncommon case where an edit and its commit straddle
midnight. Rebases and cherry-picks move committer dates and have the same
one-time effect — the known trade-off of INV-0008 Decision 4.

### Config

```yaml
# Maintain an `updated: YYYY-MM-DD` frontmatter field on every document.
# Off by default: enabling it needs git history (not a shallow clone) and
# rewrites every document once on the next `docz update`.
updated:
  enabled: false
```

```go
// UpdatedConfig maps the updated: block of .docz.yaml (DESIGN-0012).
type UpdatedConfig struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
}

type Config struct {
	// …
	Updated UpdatedConfig `yaml:"updated" json:"updated"`
}
```

- `DefaultConfig()` sets `Enabled: false`. There is nothing to normalize or
  validate; `Validate()` is untouched. The block deep-merges like its
  siblings (global then repo, repo wins).
- **Dormancy is total.** When disabled: `docz update` runs no stamp pass and
  makes no git call; `docz create` and `docz status set` do not stamp; the
  README index and `docz list` layouts are unchanged (Decision 5). A repo
  can commit the block disabled and enable it later.
- `docz_yaml.tmpl` and `.docz.example.yaml` gain the block, disabled, with
  the comment above, so `docz init` output stays a complete map of the
  schema (DESIGN-0010 Decision 6 precedent). The existing
  `TestJSONTags_MirrorYAML` walk covers the new struct automatically, so it
  cannot ship without json tags.
- The block is named `updated:`, after the field it maintains (Decision 1).

### `docwrite.SetUpdated`

```go
// SetUpdated sets the updated: field in path's YAML frontmatter to date and
// returns the value it replaced ("" when the key was absent and inserted).
func SetUpdated(path, date string) (old string, err error)

// ErrUpdatedFieldUnsupported: an updated: key exists but its value is a
// shape the byte-level mutator refuses to rewrite (block scalar, flow
// collection, anchor, alias). SetUpdated never inserts a second key.
var ErrUpdatedFieldUnsupported = errors.New("updated field has unsupported YAML shape")
```

Contract, inheriting `SetStatus` (DESIGN-0005 §Frontmatter mutation):

- **Rewrite in place** when `^updated:` exists with a bare, `"…"`, or `'…'`
  scalar: only the value bytes change; key, spacing, quotes, trailing
  comment, and every other line are preserved. Diff is one line.
- **Insert** when the key is absent: a new line `updated: <date>\n`
  immediately after the `created:` line, so the two dates sit together;
  when there is no `created:` line, as the last line of the frontmatter
  block. Diff is one added line. Inserted values are bare, matching the
  templates.
- **Refuse** — `ErrUpdatedFieldUnsupported` — when the key exists in an
  unsupported shape. Inserting would create a duplicate key and turn a
  readable document into an unparseable one.
- `document.ErrNoFrontmatter` for no `---` block; `ErrUnsupportedLineEndings`
  for CR/CRLF; IO errors wrapped with the path.
- **Idempotent at the byte level:** calling it with the value already held
  produces identical output. As with `SetStatus`, the helper always writes
  when invoked; the cmd layer owns the current-vs-wanted short-circuit
  (DESIGN-0005 Decision 8).
- **No value validation**, as with `SetStatus`. The only caller is the cmd
  layer, which formats the date itself.

Implementation: the status locator (`frontmatterBounds`, `nextLine`,
`parseStatusValue`, `quotedValue`, `bareValueLen`) is key-agnostic except
for `statusKeyRE`. It becomes `findScalarValue(content, keyRE)`; `SetStatus`
is a thin wrapper and its 12 golden fixtures prove the refactor
byte-identical. The insert path is the only new code. The package doc's
"three operations" sentence becomes four.

### Git resolver

`cmd/git.go`'s `GitResolver` grows from one method to four. The interface is
`cmd`-internal (ADR-0001: no library interfaces), so the growth breaks
nothing; `staticGit` becomes a scriptable stub.

```go
type WorkTreeState int

const (
	StateUnavailable WorkTreeState = iota // no git binary, or not a repository
	StateClean
	StateModified  // differs from HEAD: staged or unstaged
	StateUntracked
)

// Change is the last substantive commit that touched a file.
type Change struct {
	SHA  string // full sha; unused by this design, kept so the follow-up needs no interface change
	Date string // committer date, YYYY-MM-DD
}

type GitResolver interface {
	UserName(ctx context.Context) string
	Shallow(ctx context.Context) (bool, error)
	State(ctx context.Context, path string) WorkTreeState
	// LastChange walks history newest-first and returns the first commit
	// whose diff to path touches a line other than `updated:`. ok is false
	// when path has no such history.
	LastChange(ctx context.Context, path string) (chg Change, ok bool, err error)
}
```

Commands behind `realGit` (all run with `Dir` = the Runner's `RepoRoot`,
falling back to the process cwd when empty):

| Method       | Command                                                          | Notes                                                                                       |
| ------------ | ---------------------------------------------------------------- | ------------------------------------------------------------------------------------------- |
| `Shallow`    | `git rev-parse --is-shallow-repository`                          | `true`/`false`; git ≥ 2.15                                                                  |
| `State`      | `git status --porcelain --untracked-files=all -- <path>`         | empty → Clean; `??` → Untracked; anything else → Modified. Works in a repo with no commits yet, unlike `diff HEAD` |
| `LastChange` | `git log --follow --format=%x00%H%x09%cs -p -- <path>`           | streamed; parse NUL-delimited commits, stop at the first with a `+`/`-` line (not `+++`/`---`) whose text does not match `^updated:`; close the pipe early. `%cs` needs git ≥ 2.21; `--follow` keeps history across the rename a title change causes |

Exit status 128 or a missing binary map to `StateUnavailable` / an error
the pass treats as "skip". Cost measured in INV-0008: ~5 ms per document
for the common one-commit case, 8 ms to stream a full 594-line history.

Paths are passed to git exactly as `updateType` builds them
(`TypeDir`-relative). `updateType` already resolves those against the
process cwd, so the two agree whenever cwd is the repo root — the
production case. IMPL should route both through `Runner.inRepo` so
`--repo-root` behaves; that is a pre-existing looseness, not new to this
design.

### The update pass

In `Runner.updateType`, between the ToC pass and README table generation,
gated on `r.Cfg.Updated.Enabled`:

1. **Once per `docz update` invocation** (not per type): probe git. If
   `State` reports unavailable, log at debug and skip the pass everywhere.
   If `Shallow` is true, print one warning to `r.Err` —
   `Warning: shallow clone; skipping updated-field pass (clone with full
   history, e.g. fetch-depth: 0)` — and skip the pass everywhere
   (INV-0008 Decision 5).
2. **Per document**, apply the stamping model. The comparison uses the
   scan's parsed `doc.Updated` — frontmatter does not change in the ToC
   pass, so the cached parse is still right even though `DocEntry.Content`
   is stale; `SetUpdated` reads the file from disk itself and is therefore
   correct for a document the ToC pass just rewrote (INV-0008
   Observation 4).
3. **Write-through to the table:** a stamped value is copied into
   `docs[i].Updated` so the README table rendered next reflects it in the
   same run.
4. **Dry run:** one line per document that would change, in the ToC pass's
   style: `Would set updated in docs/rfc/0001-foo.md: (absent) -> 2026-09-12`.
5. **Normal run output:** silent on success, with a debug log per document,
   matching the ToC pass (Decision 7). Write errors go to `r.Err` as
   warnings and the pass continues, as the ToC pass does.
6. `docz update <type>` stamps only that type, like the ToC pass.

### `docz create`

When enabled, after `docwrite.Create` renders the file, the handler calls
`SetUpdated(path, today)` so a new document carries `updated` from birth
(equal to `created`). This is creation metadata, not index maintenance, so
`--no-update` does not skip it. Templates are untouched (Decision 2) —
which is what keeps on-disk template overrides, the claude-skills bundled
copies, and disabled repos all correct without conditionals.

### `docz status set`

When enabled and the status actually changes (not `--dry-run`, not a
same-value no-op), the handler stamps `updated` with today's date after
`SetStatus`, so the CI flow *flip → commit* is self-contained and needs no
later `docz update`. Reporting per Decision 4: the JSON output gains
`"updated"` when a stamp was written; the text line is unchanged. Exit
codes are unchanged; a failed
stamp after a successful status write is reported as a write error (exit 1)
with the status change already on disk — the same partial-write exposure
`SetStatus` has today, now spanning two lines.

### Index table and `docz list`

The README table gains an `Updated` column after `Date` **only when the
block is enabled** (Decision 5, which refines INV-0008 Decision 6a so the
dormancy rule holds: a repo that never enables the block never sees its
tables re-render). In that enabled layout the existing `Date` header is
renamed `Created` (Decision 6); dormant repos keep `Date` byte-for-byte.
`GenerateTable` takes the layout as a parameter; the internal package
stays free of config.

`docz list` follows the same rule: the text layout gains an `UPDATED`
column only when enabled; the JSON output gains `"updated"` (`omitempty`)
whenever a document carries the field, because that is data, not layout.

### Consumer contract — DESIGN-0008 R12

Added to DESIGN-0008's R-series alongside this release:

> **R12 — `Frontmatter.Updated` (docz `vX.Y.0`).** `document.Frontmatter`
> gains `Updated string` (yaml `updated`), the date the document's content
> last changed as `YYYY-MM-DD`, or `""` when the repo has not enabled the
> block or docz could not determine it. Raw string, like `Created`. The
> value is the clock date when docz stamped an uncommitted edit and the
> committer date of the last substantive commit otherwise, so it may lag a
> commit by at most one bookkeeping rewrite when an edit and its commit
> straddle a day boundary. `Config.Updated` (`UpdatedConfig{Enabled}`)
> appears in `config_snapshot` as `updated.enabled` under R11's tag rule.
> docz-api adds `documents.updated DATE NULL` and may sort by it, falling
> back to `created`.

`test/consumer` gains a parse assertion for the field and a decode of the
config block, so the surface is proven importable from outside the module.

## API / Interface Changes

**Public (`pkg/doczcore`, additive, `minor` release):**

| Package    | Change                                                                                  |
| ---------- | --------------------------------------------------------------------------------------- |
| `config`   | `UpdatedConfig{Enabled bool}`; `Config.Updated`; default disabled; yaml + json tags      |
| `document` | `Frontmatter.Updated string` (yaml `updated`); `DocEntry` inherits                        |
| `docwrite` | `SetUpdated(path, date string) (old string, err error)`; `ErrUpdatedFieldUnsupported`   |

**CLI:**

| Command           | Change                                                                                                 |
| ----------------- | ------------------------------------------------------------------------------------------------------ |
| `docz update`     | stamp pass when enabled; `--dry-run` lines; one shallow-clone warning; `Updated`/`Created` table layout when enabled (Decisions 5, 6) |
| `docz create`     | stamps the new file when enabled; templates unchanged (Decision 2)                                     |
| `docz status set` | stamps on a real change when enabled; JSON gains `updated`, text unchanged (Decision 4)                |
| `docz list`       | JSON `updated` (omitempty); text column when enabled                                                   |
| `docz init`       | generated `.docz.yaml` includes the disabled block                                                     |
| `docz config`     | prints the block (no code change)                                                                      |

**Config:** the `updated:` block above. **Templates:** unchanged.
**`cmd`-internal:** `GitResolver` grows three methods; `staticGit` becomes
scriptable; `index.GenerateTable` takes a layout parameter.

## Data Model

Frontmatter, before and after enabling:

```yaml
created: 2026-08-11          # unchanged, forever
updated: 2026-08-30          # new; backfilled from git on first run, then maintained
```

Config struct addition and its `config_snapshot` JSON shape:

```json
"updated": { "enabled": true }
```

docz-api (follow-up on that side): `documents.updated DATE NULL`, populated
from the parsed frontmatter on each ingest; no backfill.

## Testing Strategy

- **`docwrite`** — golden fixtures under `testdata/golden/updated/`
  (regenerate with `-update`): rewrite bare / double-quoted / single-quoted /
  with trailing comment; insert after `created:`; insert at block end when
  no `created:`; refuse block-scalar / flow / anchor shapes with **no**
  duplicate key written; CRLF rejected; no frontmatter rejected; same-value
  call is byte-identical. `FuzzSetUpdated` pins never-panic and
  at-most-one-`updated:`-key invariants. The existing `status` goldens stay
  byte-identical through the locator refactor.
- **`document`** — parse `updated`; absent → `""`; unknown-key leniency
  unchanged.
- **`config`** — decode / defaults / merge; dormancy (disabled block never
  fails `Validate`); `TestJSONTags_MirrorYAML` and
  `TestDoczYAMLTemplate_RoundTripsToDefaultConfig` cover the new struct and
  template block without new code.
- **`cmd/update`** — scriptable `staticGit` keyed by path (state + last
  change): each row of the convergence table above as a subtest, including
  running the pass twice and asserting zero writes on the second run;
  dry-run output; **disabled block makes no git call** (assert the stub was
  never invoked — the dormancy proof); shallow → one warning, no writes; git
  unavailable → silent skip; table write-through.
- **`cmd/create`, `cmd/status`** — stamp when enabled, absent when disabled;
  `--no-update` still stamps; `status set --dry-run` stamps nothing; JSON
  shape.
- **Real git, one integration test** — `git init` in `t.TempDir()`, identity
  and `GIT_COMMITTER_DATE` pinned via env, guarded by
  `exec.LookPath("git")`: commit content, assert `LastChange` returns that
  commit's date; commit an `updated:`-only change, assert `LastChange` still
  returns the earlier commit (the filter); rename the file, assert
  `--follow` holds; assert `State` classifies clean / modified / untracked;
  assert `Shallow` is false. A new pattern in `cmd/`, kept to one file.
- **`test/consumer`** — R12 proof from outside the module.
- **`make ci`** green at the end of every IMPL phase.

## Migration / Rollout Plan

1. **Ship as a `minor` release** (new public field, block, and function),
   with a `### RELEASE NOTES` section — and the goreleaser body fix-up
   afterward that v1.2.0 and v1.2.2 both needed.
2. **Nothing changes for existing repos.** The block is dormant by default;
   `docz init` emits it disabled. A repo enabling it runs `docz update` once
   and commits the resulting one-line-per-document backfill (34 files in
   this repo) as a single "enable updated frontmatter" change. Every later
   run converges.
3. **CI users need full history.** Any pipeline running `docz update` (or
   `--dry-run` drift checks) on a shallow clone gets the warning and no
   stamps; document `fetch-depth: 0` in the README section for the block.
4. **Dogfood:** enable the block in this repo in the post-release
   `dont-release` PR that also flips DESIGN-0012 → Implemented.
5. **docz-api:** pin bump, `updated` column, R12 clause honored — filed as
   [docz-api #36](https://github.com/donaldgifford/docz-api/issues/36),
   blocked on this design's release. docz-site shows the field and sorts by
   it with `created` fallback; its issue is filed from the docz-api side
   once #36 lands.
6. **claude-skills docz plugin:** config and workflow references gain the
   block; bundled templates unchanged (issue #95 scope grows by one section).
7. **Follow-up, not now:** `updated_sha:` per INV-0008 Decision 2, designed
   around the trailing-commit cost recorded in INV-0008 Observation 6.

## Open Questions

All eight resolved **(a)** on 2026-09-12; see [Decisions](#decisions). The
options are kept for the alternatives, which record what was weighed. Each
question lists the recommended option first as **(a)**.

### 1. What is the config block called?

**Resolved: (a)** — locked 2026-09-12; see [Decisions](#decisions).

The frontmatter key is fixed (`updated`, INV-0008 Decision 1); this is only
the `.docz.yaml` block that switches the pass on.

- **a. (Recommended)** `updated:` — a block named after the thing it
  configures, like `toc:`, `changelog:`, and `api:`. Reads as "the updated
  field: enabled." One word, mirrors the frontmatter key exactly, and the
  json tag rule (R11) makes `config_snapshot` show `updated.enabled` with no
  translation.
- b. `frontmatter:` with `updated: { enabled }` nested — leaves room for
  future per-field settings under one roof, but introduces a nesting level
  no other block has, for one boolean.
- c. `stamp:` — names the mechanism rather than the field; nothing else in
  the schema is named for what docz *does* rather than what it manages.
- d. Other.

### 2. How does a new document get the field?

**Resolved: (a)** — locked 2026-09-12; see [Decisions](#decisions).

INV-0008 recommended adding `updated: {{ .Date }}` to the six embedded
templates. Working through dormancy changed the picture: a template emits
the line whether or not the block is enabled, so a disabled repo would get
a field nobody maintains — a field that is true on day one and wrong
forever after.

- **a. (Recommended)** `docz create` calls `SetUpdated` after rendering,
  only when the block is enabled. Templates are untouched: on-disk overrides
  and the claude-skills bundled copies stay correct with no conditional,
  disabled repos get nothing, the 14 docwrite fixtures and ten test files
  that embed frontmatter do not churn, and new and old documents get the
  field through the one code path that also backfills and maintains it.
- b. Add it to the templates unconditionally, as INV-0008 said. New docs
  carry it from birth even in disabled repos; the fixture and plugin cost
  is real but mechanical.
- c. Add it to the templates behind a template variable
  (`{{ if .UpdatedEnabled }}`). Correct in both modes, but every on-disk
  override and the plugin's bundled templates would need the same
  conditional to stay in sync, and template data grows a config-derived
  field for the first time.
- d. Other.

### 3. Does a clean document's stored stamp ever get "corrected" to the committer date?

**Resolved: (a)** — locked 2026-09-12; see [Decisions](#decisions).

The one residual churn case: edit on day 1 and run `docz update` (stamp
says day 1), commit on day 2. On the next run the document is clean and its
last substantive commit is dated day 2.

- **a. (Recommended)** Write the committer date — one trailing commit, once,
  in this uncommon case. The field always converges to git's truth, the
  rule stays one sentence, and a stamp that was never committed alongside a
  later edit gets corrected the same way.
- b. Treat a stored value within one day *before* the committer date as
  authoritative. No trailing commit; the field can be a day early, and the
  rule has a tolerance window to explain.
- c. When clean, only insert if absent — never rewrite an existing value.
  Never churns, but a document edited and committed without `docz update`
  keeps a stale date until its next edit, which is the case the git-derived
  path exists to catch.
- d. Other.

### 4. Should `docz status set` stamp `updated`, and how is it reported?

**Resolved: (a)** — locked 2026-09-12; see [Decisions](#decisions).

`status set` is the CI/automation primitive (IMPL-0011) with a stable text
and JSON contract.

- **a. (Recommended)** Stamp on a real status change when enabled. JSON
  gains `"updated": "2026-09-12"` (omitted when nothing was stamped) — an
  additive field. Text output is **unchanged**; the stamp is visible in the
  diff and at `--verbose`. Automation that parses the text line keeps
  working.
- b. Stamp, and append ` (updated: 2026-09-12)` to the text line too.
  More discoverable, but a change to a line CI scripts may grep.
- c. Do not stamp here; rely on the next `docz update`. Simpler, but a
  *flip → commit* pipeline then always produces a trailing bookkeeping
  commit later, which is the exact churn the stamp-at-edit model exists to
  avoid.
- d. Other.

### 5. Does the README index table gain the `Updated` column unconditionally, or only when enabled?

**Resolved: (a)** — locked 2026-09-12; see [Decisions](#decisions).

INV-0008 Decision 6 chose "yes, in the same release." Designing it exposed
a conflict: an unconditional column changes every README table in every
repo on its next `docz update` — including repos that never enable the
block, which would show an empty column — and that breaks the dormancy
rule every other opt-in block honors. The "same release" half of the
decision holds either way; this question is about the "every repo" half.

- **a. (Recommended)** Add the column only when the block is enabled.
  Dormant repos see no table change; enabled repos get the column in the
  same `docz update` run that backfills the field, so the table is never
  empty. Costs `GenerateTable` a layout parameter.
- b. Unconditional, as decided in INV-0008 — one layout everywhere,
  simpler code, at the price of an empty column and a one-time README
  re-render in every repo, enabled or not.
- c. Defer the column to a follow-up so the frontmatter change lands alone.
- d. Other.

### 6. Is the existing `Date` column renamed `Created` when `Updated` appears?

**Resolved: (a)** — locked 2026-09-12; see [Decisions](#decisions).

With two date columns, a header that just says `Date` is ambiguous.

- **a. (Recommended)** Rename it to `Created` only in the enabled layout
  (per OQ 5a, that table is re-rendering anyway). Dormant repos keep `Date`
  byte-for-byte.
- b. Keep `Date` in both layouts; `Updated` beside it is clear enough from
  context.
- c. Rename everywhere, now — consistent, but touches every README in
  every repo, which OQ 5a exists to avoid.
- d. Other.

### 7. What does a normal (non-dry-run) `docz update` print for the pass?

**Resolved: (a)** — locked 2026-09-12; see [Decisions](#decisions).

The ToC pass is silent on success (debug log per file); the README step
prints one `Updated <path>` line per type.

- **a. (Recommended)** Silent on success, debug log per document, matching
  the ToC pass — the stamped files are visible in `git status` and the
  README line already signals the run did work. Warnings (shallow clone,
  write errors) still print.
- b. One summary line per type when at least one document was stamped
  (`Stamped updated in 3 rfc documents`). Cheap signal for the backfill
  run, but a new output shape to keep stable.
- c. One line per stamped document. Honest but noisy on the 34-file backfill.
- d. Other.

### 8. How narrow is the "bookkeeping commit" filter?

**Resolved: (a)** — locked 2026-09-12; see [Decisions](#decisions).

- **a. (Recommended)** Only the `updated:` line is bookkeeping. Everything
  else — a `status:` flip, a ToC regeneration, whitespace — counts as a
  change. One regex, no judgment calls, and a `status` flip really is an
  update.
- b. Also treat a diff confined to the `<!--toc:start-->` … `<!--toc:end-->`
  region as bookkeeping, since docz regenerates it. Avoids counting a
  first-time ToC insertion as a content change, but a ToC changes because
  headings changed, so in every later case it *is* content — and the
  parser has to track a region instead of a line.
- c. Other.

## Decisions

All eight open questions resolved **(a)** on 2026-09-12.

| #   | Question                          | Resolution                                                                                                                                  |
| --- | --------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | Config block name                 | `updated:` — named after the field it maintains, like `toc:` / `changelog:` / `api:`; `config_snapshot` shows `updated.enabled` untranslated |
| 2   | How new documents get the field   | `docz create` calls `SetUpdated` after rendering, only when enabled; templates untouched, so overrides, the plugin's bundled copies, and disabled repos need no conditional |
| 3   | Day-boundary correction           | Write the committer date — one trailing commit, once, in the edit-day ≠ commit-day case; the field always converges to git's truth          |
| 4   | `docz status set` stamping        | Stamp on a real status change when enabled; JSON gains `updated` (omitted when nothing was stamped); text line unchanged                     |
| 5   | README `Updated` column           | Only when the block is enabled — refines INV-0008 Decision 6a so dormant repos never re-render; `GenerateTable` takes a layout parameter    |
| 6   | `Date` → `Created` rename         | Only in the enabled layout, which re-renders anyway; dormant repos keep `Date` byte-for-byte                                                 |
| 7   | Normal-run output                 | Silent on success, debug log per document, matching the ToC pass; warnings still print                                                      |
| 8   | Bookkeeping filter scope          | Only the `updated:` line is bookkeeping; `status:` flips, ToC regeneration, and whitespace all count as changes                             |

## References

- [INV-0008](../investigation/0008-last-updated-frontmatter-field-scope-git-semantics-and-effort.md)
  — the scoping investigation; Observation 6 (circularity), Observation 8
  (checkout-less consumers), and the six decisions this design builds on
- DESIGN-0005 / IMPL-0011 — `SetStatus`, the byte-preservation contract and
  the cmd-owns-the-no-op rule (Decision 8)
- DESIGN-0010 — the `changelog:` block; Decision 6 (`docz init` emits the
  block) and Decision 7 (dormancy)
- DESIGN-0011 / IMPL-0016 — the `api:` block, the most recent opt-in block
  and the template for the IMPL that follows this design
- DESIGN-0004 §A / §H — `Runner` and the `GitResolver` seam
- DESIGN-0007 / DESIGN-0008 R7, R10, R11 — the consumer contract this adds
  R12 to
- ADR-0001 Decision 5 — bytes-in/values-out; narrow path-based write helpers
- DESIGN-0009 — docz-site's document lists and `updated_at`
- claude-skills #95 — bundled-template drift (unaffected under OQ 2a)
- `cmd/git.go`, `cmd/update.go`, `cmd/create.go`, `cmd/status.go`,
  `pkg/doczcore/docwrite/status.go`, `internal/index/index.go` — the code
  paths this design changes
