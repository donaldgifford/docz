---
id: INV-0008
title: "Last Updated frontmatter field: scope, git semantics, and effort"
status: Open
author: Donald Gifford
created: 2026-09-12
---

<!-- markdownlint-disable-file MD025 MD041 -->

# INV-0008: Last Updated frontmatter field: scope, git semantics, and effort

<!--toc:start-->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
  - [Observation 1 — the read side is one additive field](#observation-1--the-read-side-is-one-additive-field)
  - [Observation 2 — the write side needs a capability docwrite does not have](#observation-2--the-write-side-needs-a-capability-docwrite-does-not-have)
  - [Observation 3 — templates: add the field at birth, or let update insert it](#observation-3--templates-add-the-field-at-birth-or-let-update-insert-it)
  - [Observation 4 — docz update has a slot for the pass, with one ordering caveat](#observation-4--docz-update-has-a-slot-for-the-pass-with-one-ordering-caveat)
  - [Observation 5 — git access exists but is minimal; extending it is cheap](#observation-5--git-access-exists-but-is-minimal-extending-it-is-cheap)
  - [Observation 6 — the circularity problem, and why date and sha differ](#observation-6--the-circularity-problem-and-why-date-and-sha-differ)
  - [Observation 7 — configuration shape and defaults](#observation-7--configuration-shape-and-defaults)
  - [Observation 8 — consumer contract: additive, and the consumer cannot compute it alone](#observation-8--consumer-contract-additive-and-the-consumer-cannot-compute-it-alone)
  - [Observation 9 — sizing against comparable past work](#observation-9--sizing-against-comparable-past-work)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
  - [1. Field name?](#1-field-name)
  - [2. Sha representation, if recorded?](#2-sha-representation-if-recorded)
  - [3. Configurable key name per type (updatedfield, like statusfield)?](#3-configurable-key-name-per-type-updatedfield-like-statusfield)
  - [4. Date source for the git-derived path?](#4-date-source-for-the-git-derived-path)
  - [5. What does docz update do in a shallow clone?](#5-what-does-docz-update-do-in-a-shallow-clone)
  - [6. Add an Updated column to the README index tables?](#6-add-an-updated-column-to-the-readme-index-tables)
- [References](#references)
<!--toc:end-->

## Question

How much work, and which changes, does it take to add a "last updated"
frontmatter field to docz documents — at minimum a date, optionally the
(short) sha of the last commit that modified the file — populated and kept
current by `docz update` for any document that has changed?

Three sub-questions fall out of that:

1. **Surface:** which packages, templates, commands, and consumer contracts
   does a sixth frontmatter field touch, and is any of it a breaking change?
2. **Semantics:** what does "last commit that modified the file" mean once
   the file *contains* the answer, and can `docz update` keep the field
   current without generating an endless stream of bookkeeping diffs?
3. **Effort:** what is the phased size of the work, and which parts are
   optional?

## Hypothesis

The read side is trivial (one additive struct field). The write side is
moderate: `docwrite` has never had to *insert* a frontmatter key, only
rewrite one that exists. The hard part is not code volume but git
semantics — a value derived from git history cannot describe the commit
that contains it, so a naive implementation churns. The expected shape of
the answer is "date is cheap and stable, sha is cheap but inherently one
commit behind," and the recommendation will lean on that asymmetry.

## Context

Every docz document carries five typed frontmatter fields — `id`, `title`,
`status`, `author`, `created` — and nothing records when it last changed.
The README index tables and `docz list` show only `created`. docz-api's
`documents` table (DESIGN-0008) has an `updated_at`, but it is the
**ingest** timestamp, not the document's; and its `git_sha` is the **blob**
sha of the file, not a commit. docz-site (DESIGN-0009) surfaces
`updated_at` in its lists, so today a consumer's "last updated" is "when we
last synced," which is not what a reader wants.

The natural home for the fix is the document itself: docz already owns the
frontmatter schema, already has a byte-level frontmatter mutator (`SetStatus`,
DESIGN-0005), and already runs a per-document pass in `docz update` (the
ToC splice). This investigation sizes that path before a DESIGN doc commits
to it.

**Triggered by:** a request to scope the feature ahead of a DESIGN doc; no
issue yet.

## Approach

1. Map every code path that reads or writes frontmatter: `document.Frontmatter`
   and its consumers, the six embedded templates, `docwrite.SetStatus`'s
   locator machinery, and `cmd/update.go`'s per-type flow.
2. Inventory docz's existing git access (`cmd/git.go`) and its test doubles,
   to see what a "last commit for path" lookup would need.
3. Probe git directly in this repo for the facts an implementation depends
   on: what `git log -1 -- path` reports, its per-file cost across all 34
   documents, how a bookkeeping-only commit looks in the history, whether
   `-G` can filter such commits natively, and how dirty / untracked / shallow
   states behave.
4. Model the update loop on paper for both "date" and "sha" and check
   whether each converges (no diff on a second run) or churns.
5. Check the consumer contract (DESIGN-0008 R7, R10, R11) for what an
   additive frontmatter field means downstream, and whether docz-api could
   derive the value itself instead.
6. Size the phases against comparable past work (IMPL-0011 status set,
   IMPL-0015 changelog block, IMPL-0016 api block).

## Environment

| Component        | Version / Value                                             |
| ---------------- | ----------------------------------------------------------- |
| docz             | `main` at `e359500` (v1.2.2)                                |
| Go               | 1.26.4 (`go.mod`)                                           |
| git              | system git; POSIX ERE for `-G` (no PCRE lookahead)          |
| Repo             | 34 docz documents across 6 type dirs; full (non-shallow) clone |
| Frontmatter keys | exactly `id`, `title`, `status`, `author`, `created` in all 34 |

## Findings

### Observation 1 — the read side is one additive field

`document.Frontmatter` (`pkg/doczcore/document/document.go`) is a plain
yaml/v3 struct with five fields. yaml decoding is lenient — unknown keys are
ignored — so a document carrying `updated:` today parses fine and the value
is simply dropped. Adding a typed field is additive under ADR-0001 and
DESIGN-0007's semver rule. Its only consumers are:

| Consumer                       | Reads              | Change needed                          |
| ------------------------------ | ------------------ | -------------------------------------- |
| `cmd/list.go` `listEntry`      | `Created` → `date` | add `updated` to text + JSON output    |
| `internal/index` `GenerateTable` | `Created` → `Date` column | optional new column (see OQ 6)  |
| `document.DocEntry`            | embeds Frontmatter | none                                   |
| `internal/wiki`                | title only         | none                                   |
| docz-api (`ParseFrontmatter`)  | five fields        | pin bump + new column; see Observation 8 |

No golden in the repo asserts the *absence* of a sixth key, so the field
lands without touching existing fixtures — unless the templates change
(Observation 3).

### Observation 2 — the write side needs a capability `docwrite` does not have

`docwrite.SetStatus` (`pkg/doczcore/docwrite/status.go`, 221 lines + 318 of
tests + 12 golden fixtures) is a byte-level value substitution: it locates
`^status:[ \t]*` inside the frontmatter bounds, parses the scalar shape
(bare / `"…"` / `'…'`, trailing `# comment`), and splices only the value
bytes so the diff is one line. Two properties matter here:

- **The locator is status-specific in name only.** `frontmatterBounds`,
  `nextLine`, `parseStatusValue`, `quotedValue`, and `bareValueLen` are
  key-agnostic; only `statusKeyRE` hard-codes the key. Generalizing to
  `findScalarValue(content, key)` is a mechanical refactor with `SetStatus`
  becoming a thin wrapper, and the existing goldens prove the refactor
  byte-identical for free.
- **There is no insert path.** A missing key is `ErrStatusFieldMissing`, by
  design — `status:` is always present because every template emits it. An
  `updated:` field will be absent from every one of the 34 existing docs,
  from any doc created by an on-disk template override
  (`docs/templates/<type>.md`) that predates the field, and from any doc
  hand-authored without it. So the mutator must **upsert**: rewrite the
  value when the key exists, otherwise insert a new `key: value` line. The
  obvious insertion point is directly after the `created:` line (keeps the
  two dates adjacent), falling back to just before the closing `---`.

That upsert is the first thing `docwrite` would do that is not a
single-value substitution. ADR-0001 Decision 5 admits "path-based write
helpers only where a consumer requires in-place, byte-preserving mutation"
(`SetStatus`, `CheckTask`) — the package is deliberately narrow, not a
general editor. A purpose-named `SetUpdated(path, Updated) (old Updated,
err)` respects that stance; a generic `SetScalar(path, key, value)` is the
general-editor slope and should be avoided even though it would be the same
code. Same LF-only and byte-preservation contract as `SetStatus`.

### Observation 3 — templates: add the field at birth, or let `update` insert it

All six embedded templates share an identical frontmatter block ending in
`created: {{ .Date }}` (`time.DateOnly` → `2026-09-12`). Two choices:

- **Add `updated: {{ .Date }}` to the templates.** New docs carry the field
  from creation (`created == updated` on day one), and the upsert's insert
  path is only exercised for pre-existing and override-template docs. Cost:
  six template edits, regeneration of the 14 docwrite fixtures that embed
  frontmatter, and touch-ups in the ten test files that assert `created:`
  literally (`cmd/list_test.go`, `cmd/update_test.go`, `cmd/wiki_test.go`,
  `internal/wiki/*_test.go`, `document/*_test.go`, `docwrite/create_test.go`,
  `toc/golden_test.go`). Also makes the claude-skills plugin templates stale
  again (claude-skills #95 already tracks template drift).
- **Leave templates alone.** Zero fixture churn; the field appears only
  after the first `docz update`. Simpler, but a freshly created doc lies by
  omission until then, and `docz create` already runs the index update by
  default so the asymmetry would be visible.

Adding it to the templates is the better product; the fixture cost is real
but mechanical (`go test ./... -update`).

### Observation 4 — `docz update` has a slot for the pass, with one ordering caveat

`Runner.updateType` (`cmd/update.go`) scans the type dir, runs the ToC pass
if `toc.enabled`, then regenerates the README table. A "stamp updated" pass
fits between the ToC pass and the table (so a new `Updated` column, if
added, sees fresh values). Caveats:

- `document.ScanDocuments` caches `DocEntry.Content`, and the ToC pass
  writes back to disk **without refreshing that cache**. The stamp pass must
  therefore read from disk itself (as `SetStatus` does) rather than reuse
  `doc.Content`, or run before the ToC pass. Reading from disk is the safer
  choice — it also makes the pass correct for a doc the ToC pass just
  changed.
- `--dry-run` needs a "Would set updated in <path>: <old> → <new>" line in
  the style of the ToC and README dry-run output.
- The pass is per-type, so `docz update rfc` stamps only RFCs — consistent
  with the ToC behavior.

### Observation 5 — git access exists but is minimal; extending it is cheap

`cmd/git.go` defines `GitResolver` with one method, `UserName(ctx)`, backed
by `exec.CommandContext(ctx, "git", "config", "user.name")` and a
`staticGit` test double. There is no go-git dependency (`go.mod` has cobra
and yaml only) and none is warranted. The lookup this feature needs:

```text
git log -1 --format='%H%x09%cI' -- <path>      # last commit touching path
git diff --quiet HEAD -- <path>                # exit 1 => modified, uncommitted
git status --porcelain -- <path>               # '?? path' => untracked
git rev-parse --is-shallow-repository          # true => history is truncated
```

Measured in this repo:

| Probe                                             | Result                       |
| ------------------------------------------------- | ---------------------------- |
| `git log -1` for one doc                          | `004491f`, `2026-08-30T06:35:26-04:00` |
| `git log -1` across all 34 docs, sequential       | 0.19 s total (~5 ms each)    |
| `git log -p` full history for one doc (594 lines) | 8 ms                         |
| `git diff --quiet HEAD -- path`, clean / dirty    | exit 0 / exit 1              |
| `git log -1` for an untracked path                | empty output                 |

Cost is a non-issue at doc-set scale. The interface grows by two or three
methods (`LastChange`, `IsDirty`, `IsShallow`) that live in `cmd/`, keeping
`pkg/doczcore` free of git — consistent with the bytes-in/values-out rule
(ADR-0001 Decision 5): `docwrite` receives the resolved date and sha, it
does not compute them. Tests for the update pass use the stub; one
integration test guarded by `exec.LookPath("git")` can `git init` a
`t.TempDir()` to exercise the real resolver (a new pattern in `cmd/`, but
small).

Edge states the resolver must classify, because each one produces a
*wrong* value if ignored:

- **Untracked file:** no history; `git log` is empty. Skip (or stamp today,
  see Observation 6).
- **Modified, uncommitted:** `git log -1` returns the *previous* commit —
  stale sha, stale date. Must be detected via `diff --quiet`.
- **Shallow clone (CI default — `ci.yml` uses `actions/checkout` at
  default depth 1):** `git log -1 -- path` silently returns the shallow
  boundary commit for any file whose real last change is older. `docz
  update` in a shallow clone would rewrite every stamp to the boundary
  commit's date. Must warn and skip, or require `fetch-depth: 0`.
- **Renames:** a title change renames the file; `--follow` keeps history
  across it. Measured: identical output with and without `--follow` for the
  doc probed, since it has never been renamed. Use `--follow`.
- **No git at all** (exported tarball, non-git consumer): resolver returns
  "unknown"; the pass skips with a debug log, as `UserName` already degrades
  to `""`.

### Observation 6 — the circularity problem, and why date and sha differ

This is the finding that shapes the recommendation. Write the field from git
history and the file now contains a value derived from its own history, so
the next commit that includes the write is itself a "commit that modified
the file."

Model it. Doc `D` gets a content edit in commit `A`; `docz update` runs:

```text
run 1:  git log -1 → A          write  updated: date(A) / sha A     → diff
commit B  ("docs: docz update")
run 2:  git log -1 → B          write  updated: date(B) / sha B     → diff
commit C
run 3:  git log -1 → C          …                                    → diff
```

Every run after every commit produces a new diff. **Naive git-derived
stamping never converges.** Two ways out, and they differ by field:

**a. Filter bookkeeping commits.** Define "last commit that modified the
file" as the last commit whose diff to the file touches anything *other
than* the `updated:` line. With that filter run 2 skips `B` (its diff is
one `updated:` line), finds `A`, computes the same value, writes nothing.
Converged. The filter cannot be expressed with `git log -G`: git's `-G`
takes a POSIX ERE and rejects lookahead (`-G'^(?!updated:)'` → `fatal:
invalid regex`). It has to be done in Go over `git log -p --format=%x00%H
-- path` (8 ms for a full history here), inspecting `+`/`-` lines per
commit and stopping at the first substantive one. Typically the first or
second commit qualifies. Note the filter must be narrow: the status flip in
`004491f` is a one-line frontmatter diff (`-status: Draft` /
`+status: Implemented`) and it **is** a real update — only the `updated:`
line itself is bookkeeping.

Even with the filter, **sha is always one commit behind**: the sha is
unknowable until the commit exists, so the stamp for content commit `A'`
can only be written *after* `A'` — in a trailing bookkeeping commit. One
trailing commit per content commit is the floor for any sha-in-file design.
(The blob sha is worse: embedding a hash of the content in the content is
circular by construction.)

**b. Stamp at edit time — which only works for the date.** A date needs no
commit to exist: if `docz update` stamps *today* on any doc that is dirty
relative to `HEAD` (or untracked), the stamp goes into the same commit as
the edit. On the next run the doc is clean, the git-derived date of that
commit equals the stamp (same day), nothing is written. **No trailing
commit, no churn.** The sha has no equivalent — the value the user wants
does not exist yet at stamp time.

Residual edge: edit on day 1, commit on day 2 → stamp says day 1, commit
says day 2; a later git-derived pass would "correct" it with one trailing
commit. Tolerable, and avoidable by treating the stamped value as
authoritative when it is within a day of the commit — a detail for the
DESIGN doc.

The practical model is a **hybrid**: dirty or untracked → stamp today
(edit-time semantics); clean → git-derived with the bookkeeping filter
(handles the one-time backfill of the 34 existing docs, and any edit
committed without running `docz update` first). The sha, if recorded at
all, is only ever git-derived and therefore only ever written in a trailing
commit.

### Observation 7 — configuration shape and defaults

Turning this on in an existing repo produces a one-time diff on every
document (34 files here) on the next `docz update`, plus a `docz update`
that now needs git. That is exactly the situation the `changelog:` and
`api:` blocks solved with the dormancy pattern (DESIGN-0010 Decision 7): an
opt-in block that is inert until `enabled: true`. A shape consistent with
`toc:`:

```yaml
updated:
  enabled: false        # dormant by default in v1.x
  sha: none             # none | short | full — sha adds a trailing commit per change
```

`toc.enabled` defaults to true, but ToC regeneration is idempotent and
git-free; this pass is neither, so default-off is the right v1.x call and
a v2 could flip it. A per-type `updated_field` name (mirroring
`status_field`) is possible but premature — see OQ 3.

### Observation 8 — consumer contract: additive, and the consumer cannot compute it alone

DESIGN-0008 R7 names "the five typed frontmatter fields" as docz-api's
column contract and commits to coordinating breaking changes; adding a sixth
is additive and needs a pin bump plus a `documents.updated DATE` column on
the docz-api side (the `created DATE` pattern), and an R12 clause in the
R-series ratifying the field's semantics, as R10 and R11 did for their
surfaces.

The alternative — "don't put it in the file, let the consumer derive it" —
does not hold up for docz-api. Its ingestion is deliberately checkout-less
(DESIGN-0008 Decision 1: Git Trees / Contents API, no clone), so it cannot
run `git log`. Deriving last-modified would mean one GitHub Commits API call
per document per ingest against the rate limit. The frontmatter field is
the mechanism that carries a git-derived fact to a consumer that has no
git. That is the strongest argument for the feature living in docz rather
than downstream.

The same reasoning says the **sha is of marginal value to consumers**:
docz-api already stores `last_synced_sha` per repo and the blob `git_sha`
per document, and a per-document commit sha in the file is one commit
stale by construction (Observation 6). A date satisfies sorting and display
(docz-site's `updated_at` column); the sha mostly serves humans reading
the raw file.

### Observation 9 — sizing against comparable past work

| Phase | Scope                                                                                           | Comparable         | Size      |
| ----- | ----------------------------------------------------------------------------------------------- | ------------------ | --------- |
| 1     | `docwrite`: generalize the scalar locator, add upsert/insert, `SetUpdated`, goldens, fuzz        | IMPL-0011 `SetStatus` | ~1 day  |
| 2     | `document.Frontmatter.Updated`, `docz list` output, six templates, fixture regen, README docs   | small              | ~½ day    |
| 3     | `config.UpdatedConfig` block, defaults, `docz_yaml.tmpl`, `.docz.example.yaml`, json/yaml tag parity test | IMPL-0015 block | ~½ day |
| 4     | `cmd/`: `GitResolver` extension (last-change walk with bookkeeping filter, dirty, shallow), update pass, dry-run, stub + one `git init` integration test | new pattern | ~1–1½ days |
| 5     | DESIGN doc, IMPL doc, DESIGN-0008 R12, `test/consumer` proof, `minor` release, docz-api follow-up issue | IMPL-0016 tail | ~½ day |

Roughly **3½–4 days** for date + optional sha with the hybrid model. A
**date-only** cut drops the sha plumbing and the `sha:` config knob but not
Phase 4's git work (dirty detection and the backfill walk still need git),
so it saves perhaps half a day. Dropping the git-derived path entirely
(stamp-on-dirty only, no backfill, no filter) is the smallest cut at
~2–2½ days, at the price of never populating the 34 existing docs unless
someone edits them.

## Conclusion

**Answer:** Yes — feasible and mostly mechanical, at roughly four days, with
one genuine design decision rather than an implementation risk.

- The read side is one additive struct field; no existing behavior or golden
  breaks unless the templates also change, and that cost is a fixture regen.
- The write side needs `docwrite`'s first insert capability. The existing
  `SetStatus` machinery generalizes cleanly; the insert is the new code, and
  it should be exposed as a purpose-named `SetUpdated`, not a generic field
  editor.
- `docz update` has a natural slot for the pass; the ToC pass's stale
  content cache means the stamp pass must read from disk.
- Git access is a small extension of the existing `GitResolver`; cost is
  ~5 ms per document. Shallow clones, dirty files, and untracked files each
  produce silently wrong values unless explicitly classified.
- **The date and the sha are not the same feature.** A date can be stamped
  before commit and converges with no bookkeeping commits. A sha can only
  ever name the *previous* content commit and costs one trailing commit per
  change, forever. Any git-derived value also needs a bookkeeping-commit
  filter to converge, and that filter cannot be expressed in `git log`
  itself.
- The field belongs in docz, not downstream: docz-api ingests without a
  checkout and cannot derive it.

## Recommendation

1. **Proceed, date-first.** Write a DESIGN doc for an opt-in `updated:`
   config block that adds an `updated: YYYY-MM-DD` frontmatter field, using
   the hybrid model from Observation 6 (stamp today when dirty/untracked;
   git-derived with the bookkeeping filter when clean).
2. **Ship the sha as opt-in (`sha: short`), off by default**, with the
   trailing-commit behavior documented on the config key itself. If that
   trade-off reads as unacceptable during design review, drop it — the
   consumer case for it is weak (Observation 8) and it can be added later
   without a breaking change.
3. **Add the field to the six templates** so new docs carry it from
   creation, and have `docz status set` stamp it too — a status flip is a
   real update, and the command already rewrites the file.
4. **Default `enabled: false` in v1.x** (dormancy pattern); consider flipping
   the default in v2 once the backfill diff is behind existing repos.
5. **Guard the failure modes explicitly**: warn-and-skip on shallow clones,
   classify dirty/untracked before consulting history, degrade to no-op when
   git is absent.
6. Ratify the field for consumers as **DESIGN-0008 R12** and file the
   docz-api column follow-up alongside the release.

Open questions for the DESIGN doc, with the recommended option first:

### 1. Field name?

- a. `updated` — symmetric with `created`, both bare dates; the pair reads
  naturally in the frontmatter.
- b. `last_updated` — matches the request wording; longer, and no other key
  is prefixed.
- c. `modified` — filesystem connotation, which is exactly the semantics
  this is *not* (mtime is meaningless after a checkout).
- d. Other.

### 2. Sha representation, if recorded?

- a. A separate `updated_sha:` key holding the short sha — keeps `updated`
  a clean `DATE` for docz-api's column; sha length via config.
- b. A combined scalar such as `2026-09-12 (004491f)` — one key, but every
  consumer has to parse it.
- c. Full 40-char sha in its own key — unambiguous forever, ugly in a doc.
- d. Other.

### 3. Configurable key name per type (`updated_field`, like `status_field`)?

- a. No — fixed key in v1; `status_field` exists for legacy repos that
  predate docz and there is no such legacy for a brand-new field.
- b. Yes, mirror `status_field` from day one for symmetry.
- c. Global (not per-type) override only.
- d. Other.

### 4. Date source for the git-derived path?

- a. Committer date (`%cI`) — what `git log` shows by default and what a
  reader means by "when did this land."
- b. Author date (`%aI`) — survives rebases and cherry-picks, but can
  predate the commit by days.
- c. Other.

### 5. What does `docz update` do in a shallow clone?

- a. Warn once and skip the pass — never write a value known to be wrong.
- b. Skip only docs whose last-change walk hits the shallow boundary.
- c. Hard error, forcing `fetch-depth: 0`.
- d. Other.

### 6. Add an `Updated` column to the README index tables?

- a. Yes, in the same release — it is the reason a human wants the field —
  accepting that every README table in every repo re-renders once.
- b. Defer to a follow-up so the frontmatter change lands alone.
- c. Only when the block is enabled (column presence keyed on config).
- d. Other.

## References

- DESIGN-0005 — status set CLI primitive; the byte-preservation contract
  `SetUpdated` must inherit
- ADR-0001 — public core scope; Decision 5's narrow-`docwrite` and
  bytes-in/values-out rules
- DESIGN-0004 §A / §H — Runner pattern and the `GitResolver` seam
- DESIGN-0007 / DESIGN-0008 R7, R10, R11 — the frontmatter column contract
  and the R-series pattern for ratifying a new surface
- DESIGN-0008 Decision 1 — checkout-less ingestion (why docz-api cannot
  derive the value)
- DESIGN-0009 — docz-site's `updated_at` in document lists
- DESIGN-0010 Decision 7 — the dormancy pattern for opt-in config blocks
- IMPL-0011 — `SetStatus` (the closest comparable for Phase 1)
- claude-skills #95 — bundled-template drift, which Phase 2 would extend
- `cmd/git.go`, `cmd/update.go`, `pkg/doczcore/docwrite/status.go`,
  `pkg/doczcore/document/document.go` — the code paths inventoried
