---
id: IMPL-0008
title: "Serve docz yaml key spellings in config snapshot via docz v1.2.2"
status: Completed
author: Donald Gifford
created: 2026-08-30
---
<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL 0008: Serve docz yaml key spellings in config snapshot via docz v1.2.2

**Status:** Completed
**Author:** Donald Gifford
**Date:** 2026-08-30

<!--toc:start-->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Background](#background)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: Pin bump + zero-delta gate](#phase-1-pin-bump--zero-delta-gate)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2: Contract clause R11 — the marshaled snapshot shape](#phase-2-contract-clause-r11--the-marshaled-snapshot-shape)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 3: Wire-shape proof, docs, and close-out](#phase-3-wire-shape-proof-docs-and-close-out)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Dependencies](#dependencies)
- [Open Questions](#open-questions)
  - [1. How deep does the R11 clause pin the marshaled shape?](#1-how-deep-does-the-r11-clause-pin-the-marshaled-shape)
  - [2. Does the OpenAPI spec ride this change?](#2-does-the-openapi-spec-ride-this-change)
  - [3. What does the R11 clause marshal — a loaded config or a struct literal?](#3-what-does-the-r11-clause-marshal--a-loaded-config-or-a-struct-literal)
- [References](#references)
<!--toc:end-->

## Objective

Implement issue [#25]: bump the docz pin `v1.2.0 → v1.2.2` so
`config_snapshot` — `json.Marshal` over docz's post-`Load` `Config` — serves
the `.docz.yaml` key spellings (`{"changelog": {"enabled": …}}`,
`{"api": {"landing_page": …}}`) instead of Go field names (`{"Changelog":
{"Enabled": …}}`). docz-site's snapshot gates read the yaml spellings and
silently never match today; after this lands they match without any
dual-casing reader (docz-site DESIGN-0004 OQ-1: "fix upstream first").

The serialized shape is ratified upstream as **DESIGN-0008 R11** — a de-jure
consumer-contract clause — so this repo's `internal/doczcontract` gains a
matching clause file: a future docz field shipped without tags (or a key
rename) fails **our** suite on the next pin bump, not a downstream consumer.

**Implements:** [#25] (unblocked by docz#89 → docz#91 → docz `v1.2.2`)

## Scope

### In Scope

- docz pin bump `v1.2.0 → v1.2.2`; R1–R6+R10 re-run as the zero-delta gate.
- New `internal/doczcontract` clause **R11** pinning the marshaled snapshot
  key spellings, the `omitempty` presence semantics, and the
  `null`-vs-absent slice behavior.
- Consumer-side wire proof: the served `config_snapshot` carries yaml
  spellings after a real ingest (tighten the assertions that today only
  check non-emptiness).
- Docs: CLAUDE.md pin note; `deploy/README.md` rollout note (natural
  snapshot refresh, no backfill); spec touch per OQ-2.

### Out of Scope

- Any docz-site change — it consumes the normalized shape only (its
  DESIGN-0004 OQ-1 resolved "fix upstream first"; coordination already
  exists in docz-site#21's spec re-vendor loop).
- A snapshot backfill or migration. Each repo's `config_snapshot` rewrites
  on its next ingest; no deployment exists today, so there are no stale
  rows anywhere. The push / `-onboard` nudge is documented, not automated.
- Typing `config_snapshot` in the OpenAPI spec beyond a free-form object
  (see OQ-2c — deliberately rejected).

## Background

What the bump crosses, per the upstream releases:

- **`v1.2.1`** is config-only — docz's own `.docz.yaml` api-block dogfood
  merge (docz#88). No library change.
- **`v1.2.2`** adds `json` tags to **every exported field of all eight
  config structs** (`Config`, `TypeConfig`, `IndexConfig`, `AuthorConfig`,
  `WikiConfig`, `TOCConfig`, `ChangelogConfig`, `APIConfig`), mirroring
  each `yaml` tag exactly — name **and** `omitempty`. Nothing else: no
  decode/merge or CLI behavior changed, so the contract suite re-run should
  show zero behavior deltas. The only observable change is what
  `json.Marshal` emits.

Key spellings R11 pins (from the issue's table). Top-level: `docs_dir`,
`types`, `index`, `author`, `wiki`, `toc`, `changelog`, `api`. Nested:

| Block | Keys |
| --- | --- |
| `types.<name>` | `enabled`, `dir`, `template`, `id_prefix`, `id_width`, `statuses`, `status_field`, `plural_label`†, `aliases`† |
| `index` | `auto_update`, `preserve_header` |
| `author` | `from_git`, `default` |
| `wiki` | `auto_update`, `mkdocs_path`, `plugins`†, `markdown_extensions`†, `exclude`, `nav_titles`, `docs_dir`†, `repo_url`†, `site_url`†, `theme`† |
| `toc` | `enabled`, `min_headings` |
| `changelog` | `enabled`, `file` |
| `api` | `enabled`, `landing_page`, `exclude`, `additional_docs` |

† = `omitempty`: **absent** when zero-valued (post-`Load` snapshots
commonly omit these — built-in types have empty `aliases`, most repos
leave `wiki.repo_url`/`site_url`/`theme` unset). Fields **without**
`omitempty` marshal a nil slice as **`null`** — not `[]`, not absent
(`api.exclude`, `api.additional_docs`, `wiki.exclude`,
`types.<name>.statuses`).

Upstream pins to lean on (all in `pkg/doczcore/config/json_test.go` at
`v1.2.2`): `TestJSONTags_MirrorYAML` (reflection walk — a future untagged
field can't ship), `TestConfigJSON_MarshaledShape` (exact-string pin),
`TestConfigJSON_OmitEmptyParity` (which keys a zero-value marshal drops).

## Implementation Phases

Each phase builds on the previous one. A phase is complete when all its
tasks are checked off and its success criteria are met.

---

### Phase 1: Pin bump + zero-delta gate

The module moves to docz `v1.2.2` under the usual pin-bump discipline. The
bump crosses `v1.2.1` (config-only) and `v1.2.2` (tags-only), so the
existing R1–R6+R10 suite doubles as the regression gate and must come back
green **unchanged** — any delta means the "tags + tests + docs only" claim
is wrong, and the bump stops until it's understood.

#### Tasks

- [x] Bump the pin: `go get github.com/donaldgifford/docz@v1.2.2`
      (**never** a bare `go mod tidy`), `go mod edit -fmt`; confirm no
      import-path changes and no new transitive requirements.
- [x] Re-run `internal/doczcontract` R1–R6+R10 — green **unchanged** (zero
      behavior deltas expected; see phase intro).
- [x] Update the CLAUDE.md docz-pin note (`v1.2.0` → `v1.2.2`, the
      json-tags rationale, R11 forward-reference).
- [x] `just test` / `just lint` / `just fmt` green; commit
      (`fix(doczcontract): pin docz v1.2.2 — json-tagged config marshal`).

#### Success Criteria

- `go.mod` requires docz `v1.2.2`, no `replace`; change set confined to
  `go.mod`/`go.sum`, CLAUDE.md, and this doc.
- R1–R6+R10 pass without a single pinned expectation edited — proving the
  bump changed only what `json.Marshal` emits.

**Status: COMPLETE ✅** (2026-08-30) — pin bumped `v1.2.0 → v1.2.2`
(one-line `go.mod` change, two `go.sum` lines, no new transitive deps, no
import-path changes); R1–R6+R10 re-ran green with zero edits; CLAUDE.md
pin note updated with the json-tags rationale + R11 reference.

---

### Phase 2: Contract clause R11 — the marshaled snapshot shape

The new clause file freezes what production serializes: `ingest.Run`
marshals the post-`Load`, post-`Validate` config into `config_snapshot`,
so the clause pins that exact path's output — spellings, `omitempty`
presence, and `null`-vs-absent slices. Shape and depth per OQ-1; fixture
source per OQ-3.

#### Tasks

- [x] Add `internal/doczcontract/snapshot_test.go` (R11, the R6 mold: own
      file, doc-comment header naming the clause, hermetic loader): pin
      the top-level key set and every nested block's key set from the
      Background table; pin the † `omitempty` keys as **absent** on a
      minimal config; pin nil-slice → `null` for the non-`omitempty`
      slice fields (`api.exclude`, `api.additional_docs`, `wiki.exclude`,
      `types.<name>.statuses`).
- [x] Prove the clause by revert drill: flip one pinned spelling (or one
      presence expectation), watch it fail loudly, restore green.
- [x] Update `internal/doczcontract/doc.go` (R1–R6+R10 → +R11, noting the
      upstream DESIGN-0008 R11 ratification it mirrors).
- [x] `just test` / `just lint` / `just fmt` green; commit
      (`test(doczcontract): freeze the marshaled snapshot shape (R11)`).

#### Success Criteria

- A docz bump that renames a serialized key, drops a tag, or flips an
  `omitempty` fails R11 in this repo's suite with a message naming the
  block and key — verified by the revert drill.

**Status: COMPLETE ✅** (2026-08-30) — `snapshot_test.go` pins the exact
per-block key sets on a full fixture (`TestSnapshotKeySpellings`) and the
presence semantics on a minimal one (`TestSnapshotPresenceSemantics`):
† keys absent, nil non-`omitempty` slices `null`, `template` present when
empty, and dormant `changelog:`/`api:` blocks serializing an explicit
`enabled: false` (the exact value the docz-site gates read). Probed
empirically before pinning: with an explicit `types:` block the built-in
types do **not** merge in, and the default wiki carries
`plugins`/`exclude`/`nav_titles` (so the minimal fixture asserts the five
† wiki keys absent rather than an exact default set). Revert drill proven
(flipped `additional_docs` → `additionalDocs` → loud failure naming the
block → restored). `doc.go` now reads R1–R6+R10+R11.

---

### Phase 3: Wire-shape proof, docs, and close-out

The consumer-visible half: prove the served wire shape end to end, say it
in the docs, and close the issue.

#### Tasks

- [x] Tighten the assertions that only check non-emptiness today:
      `internal/e2e` repo-detail decodes the served `config_snapshot` and
      asserts the yaml spellings for the blocks docz-site gates on
      (`docs_dir`, `changelog.enabled`, `api.enabled` present under their
      lowercase keys; no `Changelog`/`API` capitalized keys);
      `internal/ingest/service_test.go` asserts the marshaled snapshot
      carries `"docs_dir"` rather than just being non-empty. (The
      handler/e2e fixtures already use lowercase keys — the bump makes
      them correct rather than aspirational; leave them as-is.)
- [x] Docs sweep for the old shape: `deploy/README.md` gains the rollout
      note (snapshots refresh **naturally** on each repo's next ingest;
      a push or `-onboard` nudge clears stragglers at fleet scale; no
      backfill — and no deployment exists today, so no stale rows exist
      anywhere); spec touch per OQ-2; confirm CLAUDE.md/`api/README.md`
      make no old-shape claims.
- [x] Final gates: `just ci` green; `docz update` (restore any TOC
      underscore-anchor damage); flip this doc → Completed.
- [x] Open the PR (`patch` label — a wire fix, no new surface) with
      `Closes #25`; note the docz-site follow-through lives in
      docz-site#21's re-vendor loop.

#### Success Criteria

- `TestE2EOnboardAndServe` (or a sibling) proves a real ingest serves
  `config_snapshot` with yaml-spelled keys through the read API.
- `just ci` green; the PR closes [#25] on merge; docs state the natural
  refresh so nobody hunts for a backfill job later.

**Status: COMPLETE ✅** (2026-08-30) — `TestE2EOnboardAndServe` now
decodes the served `config_snapshot` and asserts the yaml-spelled
`docs_dir`/`changelog`/`api` keys present with no capitalized Go field
names (proven against real Postgres); `service_test` asserts the
marshaled snapshot the same way at the unit seam. Spec `1.4.1`
(config_snapshot description: spellings, optional keys, `null` lists;
vacuum still 100/100); `api/README.md` version history + `deploy/
README.md` natural-refresh rollout note landed. `just ci` green.

---

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `go.mod` / `go.sum` | Modify | docz `v1.2.0 → v1.2.2` |
| `internal/doczcontract/snapshot_test.go` | Create | Clause R11: marshaled snapshot key spellings + presence semantics |
| `internal/doczcontract/doc.go` | Modify | R1–R6+R10 → +R11 |
| `internal/e2e/onboard_integration_test.go` | Modify | Decode + assert served snapshot spellings |
| `internal/ingest/service_test.go` | Modify | Snapshot assertion: `docs_dir` key, not just non-empty |
| `deploy/README.md` | Modify | Natural-refresh rollout note |
| `api/openapi.yaml` | Modify | Per OQ-2 (description touch + patch bump, if `a`) |
| `CLAUDE.md` | Modify | Pin note `v1.2.2` + R11 |

## Testing Plan

Standards carried from prior IMPLs: table-driven, stdlib `testing` only;
the R6 clause mold (own file, hermetic `HOME`-override loader); new guard
tests verified by **revert drill**. The zero-delta re-run of R1–R6+R10 is
itself the Phase 1 test. Integration proof rides the existing
`internal/e2e` suite (`//go:build integration`, real Postgres) — no new
harness.

## Dependencies

- docz `v1.2.2` on proxy.golang.org — **shipped** (docz#89 → docz#91,
  released 2026-08-30). Nothing else blocks.

## Open Questions

Answer each with a letter — **a is the recommendation**, b onward are
alternatives; anything else, write it in.

### 1. How deep does the R11 clause pin the marshaled shape?

**Answered `1a` (2026-08-30).** Key-set + presence-semantics pin.

- **a. (Recommendation) Key-set + presence-semantics pin.** Marshal, decode
  to `map[string]json.RawMessage` per block, and assert each block's
  **exact key set** from the Background table, the † keys' absence on a
  minimal config, and `null` for nil non-`omitempty` slices. Field-order
  and formatting insensitive, so it fails only on what consumers can
  observe (a spelling, a presence, a null-ness) — and an *additive*
  upstream key still fails the exact-set assert, which is the pin-bump
  discipline working as intended (bump → suite names the new key → clause
  updated deliberately).
- b. Exact-string pin of `MarshalIndent` output (the upstream
  `TestConfigJSON_MarshaledShape` template). Strongest possible freeze,
  but it also fails on field *order* and indentation — churn no consumer
  can observe — and duplicates an upstream test byte for byte.
- c. Reflection walk mirroring upstream `TestJSONTags_MirrorYAML` from
  this side (every field reachable from `Config` has a json tag matching
  its yaml tag). Catches untagged fields in blocks that don't exist yet,
  but pins tag *parity*, not the spellings themselves — a coordinated
  yaml+json rename upstream would sail through while breaking every
  snapshot consumer.

### 2. Does the OpenAPI spec ride this change?

`config_snapshot` is `type: object, additionalProperties: true` in the
spec — the wire *type* doesn't change, only the keys inside it.

**Answered `2a` (2026-08-30).** Description update + patch bump `1.4.1`.

- **a. (Recommendation) Editorial description update + patch bump
  (`1.4.0 → 1.4.1`).** Extend the `config_snapshot` description: keys use
  the `.docz.yaml` spellings; `omitempty` keys are optional (absent when
  unset); nil-slice fields serialize as `null`. Per the versioning rules
  that's editorial (descriptions only, no wire change) — exactly what
  patch is for, and it gives docz-site a version to pin its snapshot-gate
  fix against.
- b. Leave the spec untouched. The shape inside the object was never
  specced, so arguably nothing changed contractually — but then the one
  place consumers look says nothing about the very thing this fix exists
  for.
- c. Type `config_snapshot` fully in the spec (properties for every docz
  config block). Rejected up front: it couples the API spec to docz's
  config surface, doubling the maintenance of every future docz config
  change for zero consumer benefit over the R11 clause.

### 3. What does the R11 clause marshal — a loaded config or a struct literal?

**Answered `3a` (2026-08-30).** Fixture yaml through `doczcfg.Load`.

- **a. (Recommendation) A fixture `.docz.yaml` through `doczcfg.Load`**
  (the hermetic `HOME`-override loader every existing clause uses). That
  is byte-for-byte the production path — `ingest.Run` marshals the
  post-`Load` config — so the clause pins what docz-api actually
  serializes, defaults and normalization included. Two fixtures cover the
  presence semantics: a full one (every block populated) and a minimal one
  († keys absent, nil slices `null`).
- b. A hand-built `Config` struct literal (the upstream
  `TestConfigJSON_MarshaledShape` template). Simpler to read, but it pins
  the marshaler against a config no `Load` ever produces — drift between
  `Load`'s output and the literal (a default, a normalization) goes
  unnoticed, which is precisely the class of gap R-clauses exist to catch.

## References

- [#25](https://github.com/donaldgifford/docz-api/issues/25) — the issue
  this implements.
- docz#89 (the tags ask) → docz#91 (the fix) →
  [docz v1.2.2](https://github.com/donaldgifford/docz/releases/tag/v1.2.2)
  — upstream DESIGN-0008 **R11** ratifies the serialized shape.
- `internal/doczcontract` — R1–R6+R10, the clause mold this extends.
- docz-site#21 — the spec re-vendor loop the downstream fix rides.
- docz-site DESIGN-0004 OQ-1 — resolved "fix upstream first" (no
  dual-casing reader).

[#25]: https://github.com/donaldgifford/docz-api/issues/25
