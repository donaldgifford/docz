---
id: INV-0038
title: "A relative --repo-root or --config doubles the root: empty reads and misplaced writes"
status: Concluded
author: Donald Gifford
created: 2026-11-23
---

<!-- markdownlint-disable-file MD025 MD041 -->

# INV-0038: A relative --repo-root or --config doubles the root: empty reads and misplaced writes

<!--toc:start-->
<!-- markdownlint-capture -->
<!-- markdownlint-disable MD013 -->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
  - [1. The brief's cause is correct](#1-the-briefs-cause-is-correct)
  - [2. A relative --config fails the same way](#2-a-relative---config-fails-the-same-way)
  - [3. Read commands: wrong answers, mostly exit 0](#3-read-commands-wrong-answers-mostly-exit-0)
  - [4. Write commands write into the doubled path](#4-write-commands-write-into-the-doubled-path)
  - [5. --docs-dir does not help unless it is absolute](#5---docs-dir-does-not-help-unless-it-is-absolute)
  - [6. It is a v2 regression, and parity does not cover it](#6-it-is-a-v2-regression-and-parity-does-not-cover-it)
  - [7. The library is correct on its own](#7-the-library-is-correct-on-its-own)
  - [8. The brief's fix works](#8-the-briefs-fix-works)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
- [Open Questions](#open-questions)
  - [1. Where is the root made absolute?](#1-where-is-the-root-made-absolute)
  - [2. Should printed paths stay absolute for a relative root?](#2-should-printed-paths-stay-absolute-for-a-relative-root)
  - [3. Does the library change?](#3-does-the-library-change)
  - [4. How is the regression pinned?](#4-how-is-the-regression-pinned)
  - [5. Does the fix warn about a stray tree?](#5-does-the-fix-warn-about-a-stray-tree)
- [References](#references)
<!-- markdownlint-restore -->
<!--toc:end-->

<!--docz:question:start-->
## Question

Another agent wrote a brief for fixing `docz --repo-root <relative path>`,
which lists nothing and exits 0. The brief names a likely cause from
reading the code and marks two points as unverified. Before the fix is
handed out:

1. Is the brief's cause the real one?
2. Does a relative `--config` have the same fault?
3. Do write commands write into the wrong place, or only read from it?
4. Is the brief's fix (make the root absolute in `resolveRepoRoot`)
   enough, and is the library wrong too?
5. Did v1.2.2 have the fault, which decides what `just parity` can say
   about the fix?

<!--docz:question:end-->

<!--docz:hypothesis:start-->
## Hypothesis

The brief is right about the cause: `cmd` joins the root onto `DocsDir`
once and the library joins it again, so a relative root appears twice in
every path. If so, a relative `--config` fails the same way, write
commands create a second tree at `<root>/<root>/docs`, and one
`filepath.Abs` in `resolveRepoRoot` fixes all of it.

<!--docz:hypothesis:end-->

<!--docz:context:start-->
## Context

INV-0037 finding 1 recorded that
`docz --repo-root services/billing list adr` printed an empty table and
exited 0, and its recommendation says to fix that separately from
DESIGN-0029. The brief under review is that separate fix. It asks for a
branch `fix/relative-repo-root`, a `patch` PR, and a report on whether
write commands were affected.

A relative root is the natural spelling in a monorepo
(`docz --repo-root services/billing ...` from the repository root, or
the same in a CI step), which is the layout DESIGN-0029 is about.

**Triggered by:** INV-0037 finding 1; DESIGN-0029

<!--docz:context:end-->

<!--docz:approach:start-->
## Approach

1. Read `cmd/root.go`, `cmd/runner.go`, and
   `pkg/doczcore/repo/repo.go` for every place a path is joined to the
   root.
2. Build `main` and reproduce in a scratch directory: a nested
   `services/billing` with its own `.docz.yaml` and one ADR, driven
   from the parent directory.
3. Run each read command and each write command with a relative
   `--repo-root` and a relative `--config`, and diff the file tree after
   every write.
4. Run the same reproduction against the release binaries 0.4.1, 1.2.2,
   2.0.0-beta.1, and 2.0.0 to date the fault.
5. Call `repo.Open` with a relative root from a throwaway program to see
   whether the library alone is wrong.
6. Apply the brief's fix in the working tree, rebuild, rerun the
   reproduction, run `just test` and `just parity`, then revert. No code
   change is part of this document's branch.

<!--docz:approach:end-->

<!--docz:environment:start-->
## Environment

| Component | Version / Value |
| --------- | --------------- |
| docz under test | `main` at `74df5e9` (`v2.6.2-52-g74df5e9`) |
| Installed release | 2.6.2 (`c8c929c1`), the brief's version |
| Older releases | 0.4.1, 1.2.2, 2.0.0-beta.1, 2.0.0 (mise installs) |
| Platform | macOS (Darwin 27.2.0) |
| Scratch layout | `<tmp>/services/billing/{.docz.yaml,docs/adr/0001-billing-decision.md}` |

<!--docz:environment:end-->

<!--docz:findings:start-->
## Findings

### 1. The brief's cause is correct

`resolveRepoRoot` (`cmd/root.go:212`) returns `--repo-root` verbatim,
or `filepath.Dir(--config)`, and only the working-directory fallback is
absolute. `loadAndValidateConfig` then rewrites the config
(`cmd/root.go:175`):

```go
if !filepath.IsAbs(rp.Cfg.DocsDir) {
    rp.Cfg.DocsDir = filepath.Join(root, rp.Cfg.DocsDir)
}
```

With `root = "services/billing"` this gives `services/billing/docs`,
still relative. Every library path goes through `(*Repo).Path`
(`pkg/doczcore/repo/repo.go:85`), which joins `Root` onto any path that
is not absolute, so `TypeDir("adr")` is
`services/billing/services/billing/docs/adr`. That directory does not
exist and a scan of a missing directory is an empty listing, not an
error.

`Wiki.MkDocsPath` gets the same rewrite on the next lines and is joined
again by `(*Runner).inRepo` and by `pkg/wiki` (`orchestrate.go:421`).

### 2. A relative --config fails the same way

```text
$ docz --config services/billing/.docz.yaml list adr
ID  TITLE  STATUS  DATE  AUTHOR  TYPE
--  -----  ------  ----  ------  ----
[exit 0]
```

The absolute spelling of the same path lists ADR-0001. `./services/billing`
fails like `services/billing`.

### 3. Read commands: wrong answers, mostly exit 0

| Command (relative `--repo-root`) | Result | Exit |
| --- | --- | --- |
| `list adr`, `list` | Empty table | 0 |
| `validate` | `index.drift` warning for each type's README, no documents checked | 0 |
| `inspect ADR-0001` | `no document "ADR-0001"` | 1 |
| `status set adr ADR-0001 Accepted` | `no adr document with id "ADR-0001" found in services/billing/docs/adr` | 1 |
| `export html` (no doubled tree yet) | `0 pages in build/html` | 0 |

Two of these mislead beyond being empty. `validate` and `status set`
print paths relative to the root, so the message names
`services/billing/docs/adr`, the directory that does hold the document,
while the code looked in the doubled one.

### 4. Write commands write into the doubled path

This is the brief's second unverified point, and the answer is yes for
every write command tried. Each exits 0.

| Command (relative `--repo-root`) | What it wrote |
| --- | --- |
| `init` (fresh root) | `.docz.yaml` at the real root; every type directory and README under `<root>/<root>/docs/` |
| `create adr "Second decision"` | `<root>/<root>/docs/adr/0001-second-decision.md` and a new README |
| `update` | A README for every type under `<root>/<root>/docs/` |
| `template override adr` | `<root>/<root>/docs/templates/adr.md` |
| `wiki init` | `<root>/<root>/mkdocs.yml` and `<root>/<root>/docs/index.md` |
| `export html` | Pages from the doubled tree only, written to `<root>/build/html` |
| `create` with a relative `--config` | Same doubled path as `--repo-root` |

Three consequences are worse than an empty listing:

- **`create` reissues an ID.** The real tree holds ADR-0001. The doubled
  tree is empty, so the new document is also numbered ADR-0001. Moving
  it into place later gives two documents with one ID.
- **The trees then diverge.** Once the doubled tree exists, `list`,
  `update`, and `export html` work against it and report success, so
  the tool looks healthy while the real documents are ignored.
- **`export html` is split.** It reads from the doubled tree and writes
  to the single-joined output directory (`pkg/export/html/export.go:129`
  joins `Out` once), so the published site holds only the misplaced
  documents.

`status set` and `validate --fix` did not touch the real tree, because
neither can find a document there.

### 5. --docs-dir does not help unless it is absolute

`--repo-root services/billing --docs-dir docs` is empty, since the flag
value replaces `DocsDir` before the same join. An absolute `--docs-dir`
lists ADR-0001, because both joins pass an absolute path through. That
is a workaround for reads, alongside an absolute `--repo-root`.

### 6. It is a v2 regression, and parity does not cover it

| Release | Relative `--repo-root list adr` | `create adr` writes to |
| --- | --- | --- |
| 0.4.1 | Lists ADR-0001 | `services/billing/docs/adr/0002-second.md` |
| 1.2.2 | Lists ADR-0001 | `services/billing/docs/adr/0002-second.md` |
| 2.0.0-beta.1 | Empty | not run |
| 2.0.0 | Empty | `services/billing/services/billing/docs/adr/0001-second.md` |
| 2.6.2, `main` | Empty | doubled, as above |

v1.2.2 has the same join in `cmd/root.go` but its handlers used
`cfg.DocsDir` directly, so the root was applied once. The second join
arrived with `a135b10` ("Runner carries the repo API, wired in
PersistentPreRunE", 2026-09-20), when handlers moved onto
`repo.Repo` and its `Path` helper. The comment above the join says the
library "is indifferent" because `Repo.Path` passes an absolute path
through, which holds only for an absolute root.

The brief's constraint "if v1.2.2 had the same bug, the fix must not
change any golden" therefore does not apply as written: v1.2.2 did not
have the bug. The parity suite still cannot see it, because the driver
sets `cmd.Dir` to the fixture and passes no relative root, and no
`cmd` test sets `repoRoot` to a relative path (four tests set it, all
to `t.TempDir()`).

### 7. The library is correct on its own

A throwaway program calling `repo.Open(ctx, "services/billing", "")`
from the parent directory printed:

```text
Root: services/billing DocsDir: docs TypeDir(adr): services/billing/docs/adr
stat TypeDir: <nil>
```

One join, and the directory exists. A library consumer passing a
relative root is not affected; the fault is the pre-join in `cmd`. The
only library inaccuracy is the doc comment on `TypeDir`, "returns the
absolute directory", which is true only for an absolute root.

### 8. The brief's fix works

With `resolveRepoRoot` changed to return `filepath.Abs(repoRoot)` and
`filepath.Abs(filepath.Dir(cfgFile))`, on a fresh scratch tree:

- `list adr` lists ADR-0001 with a relative `--repo-root`, a relative
  `--config`, and `--repo-root` with `--docs-dir docs`.
- `create adr` writes `0002-second-decision.md` into the real tree.
- `status set` through a relative `--config` changes ADR-0001.
- `wiki init` and `export html` write under the real root, and no
  `services/billing/services` directory appears.
- `just test` passes and `just parity` replays every golden against the
  patched binary.

`just lint` was not run on the trial. The change was reverted.

One visible difference from v1.2.2 remains after the fix. v1.2.2
printed the path as the user spelled it; the fixed build prints it
absolute, which is what v2 already prints when the root comes from the
working directory:

```text
v1.2.2:  Created ADR: services/billing/docs/adr/0002-second.md
fixed:   Created ADR: /tmp/rr/services/billing/docs/adr/0002-second-decision.md
```

<!--docz:findings:end-->

<!--docz:conclusion:start-->
## Conclusion

**Answer:** Yes on every point. The brief's cause is the real one, and
its fix is sufficient.

1. The cause is the double join: `cmd` pre-joins a relative root onto
   `DocsDir` and `Wiki.MkDocsPath`, and `Repo.Path`, `Runner.inRepo`,
   and `pkg/wiki` join it again.
2. A relative `--config` has the same fault.
3. Write commands are affected. `init`, `create`, `update`,
   `template override`, and `wiki init` write into
   `<root>/<root>/...`, and `create` reissues an existing ID.
4. `filepath.Abs` in `resolveRepoRoot` fixes every case tried. The
   library is correct with a relative root and needs no change beyond
   one doc comment.
5. v1.2.2 does not have the fault. It is a regression first released in
   2.0.0-beta.1 and present in every v2 release through 2.6.2.

The brief should be corrected in three places before it is used: the
symptom is misplaced writes and a duplicate ID as well as an empty
listing, the v1.2.2 condition on the parity goldens is moot, and the
two "unverified" items are now verified.

<!--docz:conclusion:end-->

<!--docz:recommendation:start-->
## Recommendation

Proceed with the brief's fix as a `patch` PR on
`fix/relative-repo-root`, with these changes to the brief:

1. **Describe it as a regression that misplaces writes.** The PR and
   changelog entry should say that v2.0.0 through v2.6.2 scaffold and
   create under `<root>/<root>/` for a relative `--repo-root` or
   `--config`, and that `create` can reissue an ID, so a user who hit
   it knows to look for a stray tree.
2. **Fix it in `resolveRepoRoot` only** (question 1), and correct the
   comment above the `DocsDir` join, which claims more than the code
   did.
3. **Leave `repo.Open` alone** and fix the `TypeDir` doc comment
   (question 3).
4. **Test it without `os.Chdir`** by setting `repoRoot` to
   `filepath.Rel(cwd, t.TempDir())` and driving `loadAndValidateConfig`
   plus a `list` and a `create`, next to a unit test that
   `resolveRepoRoot` returns an absolute path for a relative flag and
   for a relative `--config` (question 4). The `filepath.Rel` approach
   is untried; a temp directory on another volume would have no
   relative path and the test should skip there.
5. **Drop the v1.2.2 condition.** `just parity` must still pass, and it
   did on the trial, but it says nothing about this fault.

Until a release carries the fix, the workaround is an absolute
`--repo-root` or `--config`.

This stays independent of DESIGN-0029. Its walk up to the git root and
`--project` flag will want an absolute root as well, so the fix does
not get in its way.

<!--docz:recommendation:end-->

<!--docz:open-questions:start-->
## Open Questions

### 1. Where is the root made absolute?

- **a. In `resolveRepoRoot`, with `filepath.Abs`.** *(recommendation)*
  Two lines, every downstream join then passes an absolute path
  through, and it was tried here against `just test` and `just parity`.
- b. Remove the pre-join in `loadAndValidateConfig` and let `Repo.Path`
  apply the root once. Keeps printed paths as the user spelled them,
  but `cmd` reads `DocsDir` directly in places (`cmd/wiki.go:341`,
  the debug logging) and each would need checking.
- c. Both.

### 2. Should printed paths stay absolute for a relative root?

- **a. Yes, print absolute paths.** *(recommendation)* It is what v2
  prints today for the default root, and it falls out of 1a.
- b. Print them relative to the working directory, as v1.2.2 did. This
  is a parity-shaped argument, but no golden records it and it needs
  option 1b or a display step.

### 3. Does the library change?

- **a. No code change; correct the `TypeDir` doc comment.**
  *(recommendation)* `repo.Open` with a relative root resolves paths
  correctly, and `pkg/` is semver-governed.
- b. Make `repo.Open` return an absolute `Root`. Changes what a
  consumer reads back from `Repo.Root` for no fault it has.

### 4. How is the regression pinned?

- **a. `cmd` tests only: a relative `repoRoot` from `filepath.Rel`, and
  a unit test on `resolveRepoRoot`.** *(recommendation)* Stays inside
  the no-`os.Chdir` rule and covers both flags.
- b. Also add parity cases that pass a relative `--repo-root`, captured
  from v1.2.2 as the rules require. v1.2.2 behaves correctly, so a
  capture is legitimate, but its relative printed paths would need a
  new normaliser if question 2 is answered (a).
- c. Unit test on `resolveRepoRoot` alone, as the brief suggests. Does
  not prove the commands work end to end.

### 5. Does the fix warn about a stray tree?

- **a. No. Name it in the PR and the release notes only.**
  *(recommendation)* The brief asks for a minimal fix, and the tree is
  easy to find by hand.
- b. Have `docz validate` report a `<root>/<root>/docs` directory when
  one exists. A new finding for a fault that existed for a few
  releases.

<!--docz:open-questions:end-->

<!--docz:references:start-->
## References

- [INV-0037](0037-nested-doczyaml-files-in-one-repository-monorepo-projects-in.md),
  finding 1 and the last line of its recommendation
- [DESIGN-0029](../design/0029-projects-nested-doczyaml-files-in-one-repository.md),
  projects and root resolution
- [`cmd/root.go`](../../cmd/root.go): `loadAndValidateConfig` (line 133)
  and `resolveRepoRoot` (line 212)
- [`cmd/runner.go`](../../cmd/runner.go): `(*Runner).inRepo` (line 143)
- [`pkg/doczcore/repo/repo.go`](../../pkg/doczcore/repo/repo.go): `Open`
  (line 67), `(*Repo).Path` (line 85), `TypeDir` (line 99)
- [`pkg/wiki/orchestrate.go`](../../pkg/wiki/orchestrate.go) (line 421)
  and [`pkg/export/html/export.go`](../../pkg/export/html/export.go)
  (line 129), the other two root joins
- [Commit `a135b10`](https://github.com/donaldgifford/docz/commit/a135b10),
  where handlers moved onto `repo.Repo`
- [`test/CLAUDE.md`](../../test/CLAUDE.md), on where parity goldens
  come from

<!--docz:references:end-->
