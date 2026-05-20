---
id: INV-0003
title: "Init and Update Should Respect Config-Listed Types Only"
status: Open
author: Donald Gifford
created: 2026-05-20
---
<!-- markdownlint-disable-file MD025 MD041 -->

# INV 0003: Init and Update Should Respect Config-Listed Types Only

**Status:** Open
**Author:** Donald Gifford
**Date:** 2026-05-20

<!--toc:start-->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
  - [Reproduction](#reproduction)
  - [Root cause](#root-cause)
  - [Why IMPL-0006 alone does not fix this](#why-impl-0006-alone-does-not-fix-this)
- [Conclusion](#conclusion)
- [Options](#options)
  - [A. Replace-on-presence](#a-replace-on-presence)
  - [B. Disabled-by-default seeds](#b-disabled-by-default-seeds)
  - [C. Per-type opt-in flag](#c-per-type-opt-in-flag)
- [Recommendation](#recommendation)
  - [Tests to add (drives the e2e portion of the user's request)](#tests-to-add-drives-the-e2e-portion-of-the-users-request)
- [References](#references)
<!--toc:end-->

## Question

When `.docz.yaml` lists only a subset of doc types (e.g. only `rfc`), should
`docz init` and `docz update` operate on only those listed types? If so, what
is the smallest change that yields that behavior without breaking the "no
config file → scaffold everything" default?

The user-facing concern is concrete: a project that only wants `rfc` runs
`docz init` and watches all six default type directories (`rfc`, `adr`,
`design`, `impl`, `plan`, `investigation`) get scaffolded anyway.

## Hypothesis

Init and update both iterate `config.ValidTypes()` (the hardcoded built-in
list) and check `appCfg.Types[name].Enabled`. Because `DefaultConfig()`
pre-seeds every built-in type with `enabled: true`, and Viper deep-merges the
user's config onto those defaults, omitting a type from `.docz.yaml` does not
disable it — the default seed survives the merge.

If the hypothesis is correct, the fix is a semantics change at the config
layer, not at the command sites.

## Context

A user noticed that `docz init` and `docz update` ignore the apparent intent
of their `.docz.yaml`: even when only `rfc` is listed, every type is
scaffolded and indexed. They asked whether they were misconfiguring the file.

**Triggered by:** User report in repo working session, 2026-05-20.

INV-0002 (Wave 2) already flagged the underlying defaults-drift issue as
finding F1 and routed the fix into IMPL-0006 Phase 1. This investigation
narrows the scope to the *user-visible* symptom and asks whether IMPL-0006
as written actually closes the loop, or whether a follow-up is needed.

## Approach

1. Reproduce the bug in an empty temp directory with an rfc-only
   `.docz.yaml`.
2. Trace the data flow through `config.Load`, viper's merge, and the
   command-side iteration in `cmd/init.go`, `cmd/update.go`, `cmd/wiki.go`,
   `cmd/list.go`.
3. Compare against IMPL-0006's planned changes (defaults template, sparse
   `setDefaults`, `EnabledTypes()` helper) to determine which planned step,
   if any, actually disables a type omitted from the user's config.
4. Enumerate options for fixing the semantics and pick a recommendation.

## Environment

| Component | Version / Value |
|-----------|----------------|
| docz binary | `v0.0.12-8-gf29a7b8` (post-IMPL-0005 merge) |
| Go runtime | 1.25.7 |
| Config layer | `viper v1.21.0` |
| Branch | `docs/inv-0003-init-respects-config` |

## Findings

### Reproduction

Empty temp dir, `.docz.yaml` mentions only `rfc`, then `docz init`:

```yaml
# .docz.yaml
docs_dir: docs
types:
  rfc:
    enabled: true
    dir: rfc
    id_prefix: "RFC"
    id_width: 4
    statuses: [Draft, Accepted]
    status_field: status
```

```text
$ docz init
Created docs/rfc/README.md
Created docs/adr/README.md
Created docs/design/README.md
Created docs/impl/README.md
Created docs/plan/README.md
Created docs/investigation/README.md
Initialized docz successfully.
```

`docz config` confirms the resolved config has all six types with
`enabled: true`. The user's omissions had no effect.

### Root cause

Three contributing facts:

1. `config.DefaultConfig()` at `internal/config/config.go:66` returns a
   `Config` value with every built-in type pre-populated and
   `Enabled: true`.
2. `setDefaults()` at `internal/config/config.go:291` calls
   `v.SetDefault("types.<name>.enabled", true)` for every type in the
   defaults map. Viper's merge semantics are "fill in missing keys from
   defaults" — there is no opt-out path. A type omitted from the user file
   inherits the default seed.
3. `cmd/init.go:36`, `cmd/update.go:35`, `cmd/list.go:52`, `cmd/wiki.go:309`
   all iterate `config.ValidTypes()` (the hardcoded list of all six). They
   gate work on `appCfg.Types[name].Enabled`, which is `true` for every
   default-seeded type.

The net effect: only an *explicit* `enabled: false` in the user's config
suppresses a type. Omission is treated as "use the default", and the default
is `enabled: true`.

### Why IMPL-0006 alone does not fix this

IMPL-0006 Phase 1 swaps `DefaultConfig()` from a hand-built Go struct to a
parse of the embedded `.docz.yaml.tmpl` template. That fixes drift between
the in-binary defaults and what `init` writes to disk, but the template
itself ships with every type `enabled: true`. So the merge result is
unchanged: omitted types remain enabled.

IMPL-0006 Phase 5's planned `EnabledTypes()` helper centralizes the
iteration, but the predicate it applies (`Enabled == true`) still passes
every default-seeded type. The CLI commands switch to a tidier loop but
operate on the same set.

Therefore: IMPL-0006 as written normalizes the *plumbing*. The semantic
question this investigation raises — does omission mean "skip" or "use the
default" — is orthogonal and must be answered separately.

## Conclusion

**Answer:** Yes, the user's report is a real bug-shaped behavior, not a
misconfiguration. The current contract is: `.docz.yaml` opts *out* by
setting `enabled: false`, not in by listing. Most users (and the original
DESIGN-0001 phrasing) would expect the opposite.

IMPL-0006 in its current form does not change this contract. A separate
decision is required.

## Options

### A. Replace-on-presence

If the user's `.docz.yaml` contains a `types:` block, treat it as a
*replacement* for the default types map rather than a merge target.
Omission → not configured → skipped. Absent `types:` block → fall through
to the full default set.

- **Pros:** Matches user mental model. One narrow change in `Load()` (use
  `Set` instead of `SetDefault` for the `types` key when the user's file
  has it, or skip the per-type `SetDefault` calls when a user `types:` map
  is detected). Keeps "no config" UX (`docz init` on a green field scaffolds
  everything).
- **Cons:** Diverges from Viper's normal merge story; needs a clear test
  matrix to prevent regressions. Users who relied on "list rfc, get all
  six" (unlikely but possible) would see a behavior change.

### B. Disabled-by-default seeds

Flip the template / defaults so every type ships with `enabled: false`. The
user opts in by setting `enabled: true`.

- **Pros:** Mechanically simple. Existing `EnabledTypes()` planning works
  unchanged.
- **Cons:** Catastrophic for first-run UX: `docz init` on a fresh repo with
  no config does nothing. Forces every user to learn the toggle. Strongly
  recommended against.

### C. Per-type opt-in flag

Add a new field, e.g. `configured: bool`, that the user must set explicitly.
Init/update gate on `configured && enabled`. Defaults seed `configured:
false`.

- **Pros:** Backward-compatible — a config that sets both flags continues
  to work; a config that sets neither acts like option B (silent init).
- **Cons:** Two flags is one too many. Confusing to document. Same UX
  failure mode as option B on first run.

## Recommendation

Adopt option **A**, and ship it as a small, standalone follow-up — call it
IMPL-0010 — that lands *after* IMPL-0006 Phase 1 (so the defaults template
is already the source of truth) and *before* IMPL-0006 Phase 5 (so
`EnabledTypes()` lands on the corrected semantics).

Scope of the IMPL-0010 follow-up:

1. In `config.Load`, after merging the file, detect whether the user file
   had a `types:` key at all. If yes, intersect `appCfg.Types` with the
   user's listed type names — drop any default-seeded types the user did
   not mention.
2. Document the new contract in `internal/config/config.go` and in the
   `docz init` long-help text: "If `.docz.yaml` lists `types:`, only those
   types are scaffolded and updated. Omit the `types:` block to fall back
   to the full default set."
3. Add the e2e tests requested by the user (see "Tests to add" below).
4. Update INV-0002 Wave 2 cross-references so future readers see the
   complete picture.

### Tests to add (drives the e2e portion of the user's request)

- E2E: temp dir + rfc-only `.docz.yaml` + `docz init` → assert that only
  `docs/rfc/` exists and no other type dirs were created.
- E2E: same fixture + `docz update` → assert only `docs/rfc/README.md`
  is touched; no `docs/adr/README.md` etc. is created.
- E2E: no `.docz.yaml` present + `docz init` → assert all six default type
  dirs scaffolded (regression guard for the green-field UX).
- E2E: `.docz.yaml` with `types: {rfc: {...}, adr: {enabled: false}}` →
  assert only rfc is created and adr is *not* (existing semantics
  preserved).
- E2E: incremental scenario — start with rfc-only, run `docz init`, then
  append an `adr:` block to `.docz.yaml`, re-run `docz init`, assert
  `docs/adr/` is added without disturbing `docs/rfc/`.

These belong in `cmd/init_test.go` and `cmd/update_test.go` as
`TestInit_RespectsListedTypes_*` cases using `t.TempDir()`.

## References

- INV-0002 — Architectural Review and Cleanup Opportunities, finding F1
  (defaults drift) and Wave 2 recommendation
- IMPL-0006 — Correctness and Duplication Cleanup, Phases 1 and 5
- `internal/config/config.go:66` — `DefaultConfig()`
- `internal/config/config.go:291` — `setDefaults`
- `cmd/init.go:36`, `cmd/update.go:35`, `cmd/list.go:52`, `cmd/wiki.go:309`
- Viper merge semantics — https://github.com/spf13/viper#establishing-defaults
