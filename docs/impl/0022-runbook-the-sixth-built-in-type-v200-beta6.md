---
id: IMPL-0022
title: "Runbook: the sixth built-in type (v2.0.0-beta.6)"
status: In Progress
author: Donald Gifford
created: 2026-09-30
---

<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL-0022: Runbook: the sixth built-in type (v2.0.0-beta.6)

<!--toc:start-->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: The registry entry, its templates, and parity](#phase-1-the-registry-entry-its-templates-and-parity)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2: The validation catalogue](#phase-2-the-validation-catalogue)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 3: The first runbooks](#phase-3-the-first-runbooks)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 4: pkg/runbook](#phase-4-pkgrunbook)
    - [Tasks](#tasks-3)
    - [Success Criteria](#success-criteria-3)
  - [Phase 5: cmd/, the consumer module, and the CLI surface](#phase-5-cmd-the-consumer-module-and-the-cli-surface)
    - [Tasks](#tasks-4)
    - [Success Criteria](#success-criteria-4)
  - [Phase 6: docz-api and docz-site](#phase-6-docz-api-and-docz-site)
    - [Tasks](#tasks-5)
    - [Success Criteria](#success-criteria-5)
  - [Phase 7: Documentation](#phase-7-documentation)
    - [Tasks](#tasks-6)
    - [Success Criteria](#success-criteria-6)
  - [Phase 8: v2.0.0-beta.6](#phase-8-v200-beta6)
    - [Tasks](#tasks-7)
    - [Success Criteria](#success-criteria-7)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Dependencies](#dependencies)
- [Open Questions](#open-questions)
  - [1. What goes in the first corpus?](#1-what-goes-in-the-first-corpus)
  - [2. Is Verification.Date a string or a time.Time?](#2-is-verificationdate-a-string-or-a-timetime)
  - [3. What if Scenario: does not generalise to a prefix rule?](#3-what-if-scenario-does-not-generalise-to-a-prefix-rule)
  - [4. How are rollback steps addressed?](#4-how-are-rollback-steps-addressed)
  - [5. How far does the docz-api e2e test go?](#5-how-far-does-the-docz-api-e2e-test-go)
  - [6. What version does charts/docz take?](#6-what-version-does-chartsdocz-take)
  - [7. How does the Phase 8 install prove a runbook reaches the site?](#7-how-does-the-phase-8-install-prove-a-runbook-reaches-the-site)
  - [8. Does Phase 3 enable runbook before the typed reader exists?](#8-does-phase-3-enable-runbook-before-the-typed-reader-exists)
- [References](#references)
<!--toc:end-->

<!--docz:objective:start-->
## Objective

Add `runbook` as a sixth built-in document type, **disabled by default**,
with its template, schema, validation kinds, and typed reader
`pkg/runbook`. A repository that enables it gets a first-class "Runbooks"
type in docz, docz-api, and docz-site. This repository enables it and
writes the first runbooks. The work ships as `v2.0.0-beta.6`.

The plan has eight phases, all in **one PR** from `feat/runbook-type`, the
branch that already carries DESIGN-0019 and this plan. Each numbered task
is its own commit, and the PR's merge commit is the one Phase 8 tags.

**Implements:** DESIGN-0019 (Approved; all fourteen open questions
resolved).

Tasks marked **(human)** need a person: a merge or a tag push. An
automated run marks them `deferred - human required` and continues.

The order keeps `just ci` green after every phase:

- The registry entry cannot land without its three embedded files: about
  fifteen tests loop the registry and need a template, an index header,
  and a schema for every entry.
- It also cannot land without the parity normaliser: the entry changes
  fourteen v1.2.2 goldens the moment it exists.

So Phase 1 carries all three.
<!--docz:objective:end-->

<!--docz:scope:start-->
## Scope

<!--docz:in-scope:start-->
### In Scope

- The registry entry, the disabled-by-default surfaces, and the generated
  `.docz.yaml` (DESIGN-0019 §1, §2)
- `runbook.md`, `index_runbook.md`, `schema/runbook.md`, and the template
  golden (§3)
- The nine catalogue kinds and `checkSteps` (§5)
- `pkg/runbook`: `Doc`, the step grammar, the Last Verified table,
  `Validate`, the corpus, and the fuzz target (§4)
- `cmd/`: the sixth `typeValidator` arm, the disabled-type error that names
  the flag, `init` help, and the `(disabled by default)` help suffix (§2,
  §6)
- The `runbook` parity normaliser and its documentation as a permitted
  delta (§7)
- `test/consumer` and `layer_test.go` for the new package (§6)
- The docz-api ingest and e2e proof, and the docz-site colour, blurb, and
  mocks (§8)
- The version-skew note in the chart NOTES, the chart README, and the repo
  README (§8, OQ 14)
- Enabling runbook in this repository and writing the first runbooks (§10)
- README, CLAUDE.md, DEVELOPMENT.md, CONTRIBUTING.md
- `v2.0.0-beta.6`, cut by following RUNBOOK-0001, and the follow-up issues

<!--docz:in-scope:end-->

<!--docz:out-of-scope:start-->
### Out of Scope

- Executing runbooks: no `docz run`, no dry-run of `Step.Commands`
- A step-aware docz-site view (DESIGN-0019 OQ 12). Phase 8 files it
- Serving runbook metadata as structured API fields. Phase 8 files it
- One ADR per built-in type (DESIGN-0019 OQ 13). Phase 8 files it
- Any HTTP API or OpenAPI change, and any docz-api migration
- Any change to `document.Frontmatter`
- Re-capturing any parity golden
- Updating the claude-skills docz plugin, which waits for v2.0.0 proper

<!--docz:out-of-scope:end-->
<!--docz:scope:end-->

## Implementation Phases

Each phase builds on the previous one. A phase is complete when all its tasks
are checked off and its success criteria are met. The phases are
sequential commits on one branch, and the one PR carries `dont-release`.
Push after each phase so CI runs every phase, including the path-filtered
ui jobs from Phase 6.

---

<!--docz:phase:start-->
### Phase 1: The registry entry, its templates, and parity

This phase makes `runbook` a type. It is disabled, it renders in the
generated `.docz.yaml`, `docz template show runbook` prints its template,
and every registry-looping test covers it. Nothing validates a runbook
beyond the generic tier yet.

<!--docz:tasks:start-->
#### Tasks

- [x] `pkg/doczcore/config/doctype.go`: append the `runbook` entry exactly
  as DESIGN-0019 §1 gives it:
  - `Enabled: false`;
  - `rb` as the alias;
  - the four statuses;
  - `Runbooks` for both `NavTitle` and `PluralLabel`;
  - the `HelpDescription`.

  Fix the stale `plan` comments in `config.go` (`:184-191`, `:401`) so
  they name runbook as the disabled built-in.
- [x] `TypesHelp()` appends ` (disabled by default)` after the aliases of
  any entry whose `DefaultConfig().Enabled` is false (DESIGN-0019 OQ 11).
  `TestTypesHelp` asserts the suffix on runbook and on no other line.
- [x] `pkg/doczcore/doctemplate/templates/runbook.md`: the body template
  from DESIGN-0019 §3. It has these regions:
  - `last-verified` first, with the four-column table, one empty row, and
    `**Notes:**`;
  - `overview` with `**Service:**` and `**Owner:**`;
  - `when` and `prerequisites`;
  - a bare `## Procedures` heading with one `procedure`. The procedure
    holds `steps`, `verification`, and `rollback`;
  - a bare `## Troubleshooting` heading with one `scenario`. The scenario
    holds `**Alert:**`, `**Likely cause:**`, and `steps`;
  - `escalation` (a `Who | When | How` table) and `references`.

  Placeholder headings are `### Procedure 1: <!-- … -->` and
  `### Scenario: <!-- … -->`. The file has the
  `markdownlint-disable-file MD025 MD041` line after the frontmatter.
  Done. Inference needed two changes in `kinds/infer.go`, made here because Phase 1's template tests need them. A kind now gets one rule per parent, since `steps` sits under both procedure and scenario. A bare `Word:` placeholder generalises to a colon-terminated prefix (`scenario:`), per Open Question 3. `nest` and `descends` follow every parent of a kind. A test in `kinds/infer_test.go` pins both.
- [x] `templates/index_runbook.md`: the README index header, in the same
  form as `index_impl.md`, with exactly one index marker pair
  (`index/splice_test.go`)
- [x] `templates/schema/runbook.md`: the marker skeleton, with each
  region once and nesting preserved. The derivation test
  (`doctemplate/schema_test.go`, `validate/schema_test.go`) proves template
  ≡ skeleton
  Done. The skeleton lists `steps` twice, once under procedure and once under scenario, because a schema is a set of (kind, parent) pairs. `validate`'s `checkParents` assumed one parent per kind, so it now accepts any parent the schema lists for that kind, and names all of them in the message. Two `families_test.go` cases pin this.
- [x] `pkg/doczcore/doctemplate/golden_test.go`: add runbook to the `Data`
  map, then regenerate `testdata/golden/runbook.md` with `-update`
- [x] `docz_yaml.tmpl`: the preamble says six built-in types, and adds the
  two lines from DESIGN-0019 §2 on enabling runbook. It also warns that a
  `types:` block listing only runbook switches the other five off
  (DESIGN-0019 Migration). `parity_baseline_test.go`'s round trip still
  passes, and the rendered file carries `runbook:` with `enabled: false`
  Done. `docz init` in an empty directory writes `runbook:` with `enabled: false`, and creates `adr design impl investigation rfc` under `docs/`, with no `runbook`.
- [x] Config tests:
  - `config_test.go`: `disabledByDefault` becomes `{"runbook": true}`,
    both `len(cfg.Types) != 5` checks become 6, and `TestDocTypeNames`
    gains `runbook` last;
  - `doctype_test.go`: add `runbook` and `rb` to the `LookupDocType`
    table;
  - a new test pins that a disabled built-in is absent from
    `EnabledTypes()` and present in `DefaultConfigYAML()`;
  - a new test pins the short-block rule. `types: {rfc: {}, runbook:
    {enabled: true}}` loads with runbook's `dir`, `id_prefix`, and
    statuses filled from the registry.
  Done. The short-block fixture lists `rfc` as `enabled: true`, because `rfc: {}` decodes as disabled. The `DefaultConfigYAML` test lives in `doctemplate/promoted_test.go`, which is where that function is.
- [x] `pkg/doczcore/repo/init_test.go:192`: count `EnabledTypes()` rather
  than `DocTypeNames()`. The `plan` comment at `:23-27` gets rewritten
  for runbook. Add a case that enables runbook and sees `docs/runbook/`
  and its README created. The default case sees neither
  Done. `TestInit_ExactlyOneMarkerPair` turns runbook on so every built-in's header is still checked, and counts `EnabledTypes()`. `TestInit_RunbookOnlyWhenEnabled` covers both cases.
- [x] Parity: add `RunbookNormalizer()` to `test/parity/parity.go`,
  following `PlanNormalizer()`. It drops the `runbook:` key and its
  indented body under `types:`, and the `runbook:` line under
  `nav_titles`. It runs on both sides (`parity_test.go`'s `both` list),
  and has unit tests in `parity.go`'s test file, which `just test` runs.
  `test/parity/README.md` lists "a new built-in type's config block and
  nav title" as a permitted delta, with the reasoning in DESIGN-0019 §7
  Verify: `just parity`
  Done. `just parity` replays all 213 goldens clean, and `TestRunbookNormalizer` has four cases.
- [x] `just ci` passes
  Done. `just ci` passes, including lint, test, the consumer module, parity, validate, build, the licence check, the api and ui chains, and `chart::lint`. **Criterion correction:** `docz template show runbook` does not print the template while runbook is disabled. It says `document type "runbook" is disabled`, because all three `template` subcommands reject a disabled type, and existing cmd tests pin that (CLAUDE.md, Architecture). The behaviour is left as it is and the criterion below is amended. Once the type is enabled, which happens in this repo in Phase 3, the template prints.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `DocTypeNames()` ends in `runbook`, and `EnabledTypes()` on a default
  config does not contain it
- `docz init` in an empty directory writes a `.docz.yaml` with a
  `runbook:` block at `enabled: false` and creates no `docs/runbook/`
- `docz --help` lists runbook with `(disabled by default)`. `docz
  template show runbook` prints the template once the type is enabled; while
  it is disabled it refuses, as every `template` subcommand does for a
  disabled type (amended in Phase 1)
- `just parity` passes against the unchanged v1.2.2 goldens
- `just ci` passes

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 2: The validation catalogue

This phase teaches the generic validator the nine runbook kinds, so a
runbook's regions are checked for shape before any typed reader exists.

<!--docz:tasks:start-->
#### Tasks

- [x] `pkg/doczcore/validate/kindrule.go`: add the catalogue entries from
  DESIGN-0019 §5. The two table kinds are wrappers over the existing
  `checkTable`: `last-verified` over `date`, `pr`, `commit`, and
  `verified by`, and `escalation` over `who`, `when`, and `how`.

  | Kind | Singleton | Check |
  | ---- | --------- | ----- |
  | `last-verified` | yes | `checkTable` |
  | `when` | yes | `checkItems` |
  | `prerequisites` | yes | `checkItems` |
  | `procedure` | no | none |
  | `steps` | yes, per parent | `checkSteps` |
  | `verification` | yes, per parent | `checkItems` |
  | `rollback` | yes, per parent | none |
  | `scenario` | no | none |
  | `escalation` | yes | `checkTable` |
  Done. `steps` is added here without a check, and `checkSteps` arrives in the next task.

- [x] `checkSteps`: a region whose list items are all unordered is
  `steps.not-ordered` (error). One with no list items at all returns
  nothing, because an empty region is incomplete, not malformed. That
  follows `checkTable`'s rule for a region with no table
- [x] Correct the kind count in the file's two comments (`:16`, `:42`) to
  the number of keys in the map, counted rather than incremented
  Done. Counting the map's keys with `go/ast` gives 50. The old comments' 41 was accurate before runbook's nine were added, so both comments now say fifty.
- [x] `validate` tests:
  - one case per new kind;
  - `checkSteps` with ordered, unordered, mixed, nested, and empty regions;
  - the two table wrappers with right, missing, and reordered columns;
  - a singleton case proving one `steps` per procedure and one per
    scenario is clean, while two in one procedure is
    `region.duplicate-singleton`.
  Done, in `validate/runbook_kinds_test.go`.
- [x] `document_test.go`'s rendered-template check passes for runbook with
  zero findings, with and without inference, and `kinds/infer_test.go`'s
  `TestInferenceEqualsMarkers` passes. That requires `SpecFromTemplate` to
  generalise `Procedure 1:` and `Scenario:` to prefix rules. If
  `placeholderHeading` does not match `### Scenario: <!-- … -->`, which has
  no token, fix it in `kinds/infer.go` and pin the fix with a case
  (Open Question 3)
  Done; it passes for runbook. The `infer.go` fix for Open Question 3 landed in Phase 1 with the template (see Phase 1, the template task), together with a pinning test.
- [x] `just ci` passes
  Done.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- A freshly rendered runbook validates clean through the generic tier,
  marked and unmarked
- A runbook whose `steps` region holds bullets is `steps.not-ordered`
- `just ci` passes

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 3: The first runbooks

This phase enables runbook in this repository and writes the real
documents that become `pkg/runbook`'s corpus. They are written before
the reader, so the reader is built against real runbooks and not against
its own assumptions. That is how the other five type packages were built.

<!--docz:tasks:start-->
#### Tasks

- [x] `.docz.yaml`: add `runbook` to the existing `types:` block with
  `enabled: true`, using the short block. Then run `docz update`, which
  creates `docs/runbook/README.md` and adds Runbooks to the wiki nav
  Done. `docz update` created `docs/runbook/README.md`. The wiki nav gains its Runbooks section with the first runbook, because a directory that holds only a README adds no nav pages.
- [x] `docz create runbook "Cut a v2 beta release"` → RUNBOOK-0001. Write
  it from IMPL-0021 Phase 6 and CLAUDE.md's release notes. It has three
  procedures, Prepare, Tag and verify, and Install and smoke-test, and two
  scenarios:
  - **Prepare** covers the chart bump, bare `appVersion`, and
    `just release-check` on `main`.
  - **Tag and verify** covers:
    - `just release vX`;
    - watching `prerelease.yml`;
    - checking the pre-release and archives;
    - image tags with no `latest`;
    - `helm show chart` and `cosign tree` per chart.
  - **Install and smoke-test** is the OCI install on kind with `helm test`.
    It is optional for a beta (Open Question 7).
  - **The first `charts/<name>` push fails `403 write_package`**. The fix
    is the Actions access grant plus a job re-run.
  - **A `v`-prefixed `appVersion` 404s on pull.**

  Each command sits in a fenced block under its step, with an
  `**Expected:**` line where the output is checkable. Its Last Verified row
  is the `v2.0.0-beta.5` run: `2026-09-25`, `#137`, `e41203e`, the
  maintainer.
  Done: `docs/runbook/0001-cut-a-v2-beta-release.md`, status Active. It has the three procedures per Open Question 7, so Install and smoke-test is its own optional procedure, and the two scenarios. Its Last Verified row is the beta.5 run. The wiki nav now has a Runbooks section.
- [x] `docz create runbook "docz-api webhook deliveries fail"` →
  RUNBOOK-0002 (Open Question 1). It is troubleshooting-first, with
  scenarios from `deploy/tailscale-operator.md` and INV-0007:
  - TLS EOF, caused by the Funnel `nodeAttrs` grant or by HTTPS
    certificates;
  - no Ingress hostname;
  - `401` on delivery, a webhook secret mismatch;
  - ingest failing after a delivery was accepted, read from the queue's
    logs;
  - a retry-exhausted task blocking a repo.

  It has one procedure, "Redeliver and confirm".
  Done: troubleshooting-first, five scenarios and one procedure; Last Verified row left blank until it is run.
- [x] `docz validate` on the repository is clean for both runbooks, with
  no errors. The generic tier and the Phase 2 kinds run, and the typed
  tier arrives in Phase 5
  Done: zero findings for docs/runbook/ even under --strict.
- [x] `just ci` and `just validate` pass
  Done.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `docs/runbook/` exists with a README index listing RUNBOOK-0001 and
  RUNBOOK-0002, and `mkdocs.yml` has a Runbooks section
- Both runbooks validate clean through the generic tier
- RUNBOOK-0001 is complete enough that Phase 8 can cut the release by
  following it and nothing else

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 4: `pkg/runbook`

This phase builds the typed reader. It has the same four files as every
type package plus `walk.go`, following `pkg/impl`.

<!--docz:tasks:start-->
#### Tasks

- [x] `pkg/runbook/doc.go`: the package comment with the `EXPERIMENTAL`
  paragraph, and these types from the DESIGN-0019 §4 class diagram:
  - `Doc`, `Procedure`, `Scenario`, `Step`;
  - `Command`, `Contact`, `Verification`;
  - `DuplicateProcedureError`.

  Add the lookups `Procedure(token)`, `Step(id)`, and `Steps()`, with value
  receivers as `impl.Doc` has. `Verification.Date` is a string
  (Open Question 2).
  Done: plus a Notes field on Verification for the line under the table.
- [x] `headings.go`: the `kind*` constants and the `headings`
  `kinds.HeadingSpec`, exported through `Headings()`. `headings_test.go`
  pins it equal to `kinds.SpecFromTemplate` over the embedded template.
  This runs from the test only, because `pkg/runbook` may not import
  `doctemplate` (R2)
  Done.
- [x] `walk.go`, the step grammar from DESIGN-0019 §4 rules 1–6:
  - ordered items become steps;
  - children are the indented ordered items;
  - continuation lines fold into `Text`;
  - a fenced block belongs to the step above it, as a `Command` with its
    info-string language;
  - `**Expected:**` becomes `Step.Expected`;
  - bullets stay in `Text`.

  IDs:
  - procedure steps are `<token>.<n>[.<m>…]`;
  - scenario steps are `S<index>.<n>[.<m>…]`;
  - rollback steps are `<token>.R<n>` (Open Question 4).

  Build it on `docparse.ListItems` plus the fence-aware line walk
  `impl/walk.go` uses.
  Done: a single pass with a stack of open steps; an Expected line folds its own continuations.
- [x] `parse.go`: `Parse(doc []byte) (Doc, error)`. It fails only for no
  frontmatter and for CR line endings. It resolves regions with
  `kinds.ResolveRegions` and sets `Inferred`. It switches on region kind,
  never on the type name. It reads:
  - the Last Verified table's first data row into `*Verification`, which
    stays nil while every cell is empty;
  - `Verified by`, split on commas and trimmed;
  - the overview and scenario fields with `kinds.Field`;
  - the escalation table into `[]Contact`.

  Lines are shifted to document lines with the `kinds.Shift*` helpers. Two
  procedures claiming one token return `*DuplicateProcedureError`.
  Done: wrapped bold fields (Notes, Likely cause) fold their continuation lines, which kinds.Field alone does not.
- [x] `validate.go`: `Validate(doc []byte) []validate.Finding`, with the
  `Code*` constants for every code in DESIGN-0019 §4's table, including
  the six `runbook.last-verified.*` codes. Exactly one `runbook.parse`
  finding when `Parse` fails. No rule reads the clock
  Done: runbook.step.empty reads the item as written, so the template's comment placeholder is not an empty step.
- [x] Corpus, `pkg/runbook/testdata/`:
  - RUNBOOK-0001 and RUNBOOK-0002 are snapshotted as `.orig.md` with
    markers stripped, plus their marked `.md`;
  - one hand-written legacy runbook (`legacy-onboarding.orig.md`, no
    markers, "Step 1:" headings rather than lists) proves inference and
    the `steps.not-ordered` path (Open Question 1);
  - `.golden.txt` fact files are generated with `-update`;
  - `testdata/README.md` records what the corpus taught.
  Done: the generated marked copies differ from docs/runbook only in blank lines before end markers; the README records why.
- [x] Tests:
  - table tests for the step grammar: nesting three deep, two commands
    under one step, `Expected`, bullets as prose, continuation lines,
    rollback IDs, and scenario IDs;
  - Last Verified: empty row → nil, filled row, multiple verifiers,
    and each `runbook.last-verified.*` code;
  - one test per validation code;
  - the corpus goldens;
  - the `.orig.md` inference invariant: every field equal but `Inferred`
    and the shifted lines;
  - `FuzzParse`, pinning no panic, and that every `Step.Line` is within
    the input.
  Done: parse_test, validate_test (one case per code, plus the rendered template has no errors), and FuzzParse, run for 45s clean.
- [x] `pkg/doczcore/layer_test.go`: add `/pkg/runbook` to `forbidden`.
  `pkg/doczcore/repo/migration_test.go`: add runbook to
  `migrationTypePackages` and `migrationCorpusTypes`, so
  `InsertRegions` reproduces the marked corpus byte for byte and a
  second run is a no-op
  Done: the migration repo turns runbook on, since InsertRegions over a disabled type is an error.
- [x] `just ci` passes
  Done.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `runbook.Parse` over RUNBOOK-0001 returns its procedures, scenarios,
  commands, and a `Verification` of `2026-09-25` / `#137` / `e41203e`
- Every corpus `.orig.md` infers to the same facts as its marked sibling
- `InsertRegions` migrates the corpus byte for byte
- The core imports no `pkg/runbook`, and `layer_test.go` enforces it
- `just ci` passes

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 5: `cmd/`, the consumer module, and the CLI surface

This phase wires the typed tier into `docz validate`, makes the disabled
type's error say how to fix it, and proves `pkg/runbook` importable from
outside.

<!--docz:tasks:start-->
#### Tasks

- [x] `cmd/validate.go`: `typeValidator` gains `case "runbook": return
  runbook.Validate`, the sixth arm
  Done.
- [x] `cmd/create.go`: the disabled-type error becomes
  `document type %q is disabled in configuration; set
  types.%s.enabled: true in .docz.yaml`. The exit code stays 1. It names
  the canonical type, so `docz create rb` says `types.runbook`
  Done.
- [x] `cmd/init.go`: the Long help says "the enabled built-in types"
  instead of "all five"
  Done: it also names the key that turns runbook on.
- [x] New `cmd/runbook_test.go`, since existing `cmd/*_test.go` files are
  not edited to fit:
  - `docz create runbook` on a default config exits 1 with the flag named;
  - enabled, it creates `docs/runbook/0001-*.md` and updates the README;
  - `docz create rb` and `RUNBOOK` resolve;
  - `docz validate` reports a `runbook.*` finding for a broken runbook;
  - `docz list runbook` lists it.
  Done.
- [x] `test/consumer/consumer_v2_test.go`: add a `pkg/runbook` case. It
  runs `Parse` over an inline fixture carrying a procedure, a scenario, a
  command, and a filled Last Verified row, then `Validate`, with lines
  located by `lineOf`. Update `test/consumer/doc.go`'s count to seventeen
  `pkg/` packages
  Done.
- [x] `docz validate` on this repository runs the typed tier over both
  runbooks with no errors. Fix the runbooks rather than the rules if it
  finds something real
  Done: zero findings for docs/runbook/, --strict included.
- [x] `just ci` passes
  Done.

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `docz validate` runs `runbook.Validate`, and both runbooks are clean
- `docz create runbook` on a default config tells the user which flag to
  set
- `just test-consumer` imports and exercises `pkg/runbook`
- `just ci` passes

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 6: docz-api and docz-site

This phase proves that an enabled runbook reaches the API and the site as
a first-class type, and gives the site a curated colour and blurb for it.

<!--docz:tasks:start-->
#### Tasks

- [x] `internal/ingest/service_test.go`: add
  `TestRunMapsRunbookType`. It uses the fake fetcher over a
  `.docz.yaml` whose `types:` enables runbook with the **short** block,
  and asserts:
  - the `doc_types` input carries `runbook` / `Runbooks` / `RUNBOOK` /
    `["rb"]`;
  - a `docs/runbook/0001-*.md` blob becomes a document of type
    `runbook`;
  - a disabled runbook produces neither.
  Done: it found that mapDocType stored only .docz.yaml aliases, never the registry's, so `rb` (and `inv`, `implementation`) never reached the row; the mapper now merges them.
- [x] `internal/httpapi`: a `typeresolver_test.go` case resolving `rb`,
  `RUNBOOK`, and `runbook` to the canonical type from a stored row
  Done.
- [x] `internal/e2e/onboard_integration_test.go`: add
  `TestE2ERunbookType` (`//go:build integration`, real Postgres). It
  onboards a fixture with an enabled runbook, then asserts `GET
  …/types` lists it and `GET …/types/rb/docs` lists the document. It also
  checks that `GET …/docs/RUNBOOK-0001` serves it. If the search e2e
  fixture helpers make it cheap, it also checks that search with
  `type=runbook` returns it (Open Question 5)
  Done against real Postgres; the search half (Open Question 5) is skipped, since this harness has no Meilisearch and adding one is not cheap.
- [x] `ui/src/lib/colors.ts`: add `runbook` to `CURATED_TYPES`.
  `ui/src/theme/tokens.css`: add `--color-t-runbook` in both themes, in
  a hue the other curated types do not use
  Done: `rb` maps too, as `inv` does. tokens.css defines one theme, so the token is added once.
- [x] `ui/src/lib/docTypes.ts`: add the `runbook` blurb and remove the
  stale `plan` one
  Done. The `plan` mentions left in ui/ are prose ("Implementation Plans", "build plan"), not the type.
- [x] `ui/src/lib/colors.test.ts`: switch the uncurated-type example from
  `runbook` to `postmortem`, and add a case pinning runbook's curated
  token
  Done.
- [x] `ui/src/mocks/fixtures.ts`: add a runbook type and one runbook
  document to a mocked repo, and add a component or route test pinning the
  "Runbooks" nav entry and its blurb
  Done: a small dedicated runbook fixture (a copy of RUNBOOK-0001 matched the palette's queries); five suites' fixture counts moved by one document.
- [x] Version skew (DESIGN-0019 OQ 14):
  - `charts/docz/templates/NOTES.txt` adds one line: enable runbook in a
    repo only after this release is deployed;
  - `charts/docz/README.md.gotmpl` gets a sentence to the same effect,
    regenerated with `just chart docs`;
  - the chart's NOTES test, if one pins the text, is updated.
  Done: no chart test pins the NOTES text beyond the no-Tailscale check, which still passes.
- [ ] `just api test`, `just ui ci`, `just chart unittest`, and `just ci`
  pass, and CI's `ui` and `ui-e2e` jobs pass on the pushed phase

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- An enabled runbook, declared with the short block, becomes a
  `runbook` type in docz-api and is served by name, prefix, and alias
- docz-site shows "Runbooks" in the type nav with its own colour and
  blurb
- `ui/` no longer mentions `plan`
- `just ci` passes, and so do CI's ui jobs

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 7: Documentation

This phase makes every piece of prose that lists the built-in types say
six, and says how to enable the one that is off.

<!--docz:tasks:start-->
#### Tasks

- [x] `README.md`:
  - the type count (`:10`) and the types table;
  - a RUNBOOK section, placed with the other five, covering the Last
    Verified table, procedures and scenarios, ordered steps, and how to
    enable (both `types:` cases, from DESIGN-0019 Migration);
  - "Beyond the five built-ins" becomes six;
  - the sample config and `nav_titles`;
  - the package table gains `pkg/runbook`;
  - the version-skew note.
  Done: the RUNBOOK section carries the enable instructions and the version-skew note.
- [x] `CLAUDE.md`:
  - "Five built-in doc types" becomes six, with runbook disabled by
    default;
  - the aliases line;
  - `pkg/{rfc,adr,design,impl,investigation,runbook}/`;
  - a `pkg/runbook` paragraph with the step grammar, IDs, Last Verified,
    and codes;
  - the catalogue kind count;
  - the parity paragraph's new `runbook` normaliser;
  - the consumer module's seventeen packages.
  Done: also the experimental-package count (twelve) and the alias row docz-api now stores.
- [x] `DEVELOPMENT.md` (`:69`, `:290-325`, the "Adding a Built-In
  Document Type" walkthrough, `:844-850`) and `CONTRIBUTING.md`
  (`:133-145`): runbook is the worked example of a built-in that ships
  disabled. Name the Phase 1 lesson: the registry entry, three embedded
  files, and a parity normaliser land together
  Done: CONTRIBUTING's template paths were stale (internal/template) and now name pkg/doczcore/doctemplate and the schema skeleton.
- [ ] `docs/index.md`: link the Runbooks README
- [ ] `just validate` and `just ci` pass, and `git-cliff` regenerates the
  changelog

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- No tracked prose outside `docs/archive/`, `testdata/`, and CHANGELOG
  says "five built-in" types. A `grep -rn "five built-in"` finds only
  historical documents
- `just ci` passes

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 8: v2.0.0-beta.6

This phase cuts the release by following RUNBOOK-0001's first two
procedures and closes out. It checks that the artifacts are published and
signed, and it does not run an install. The whole beta is tested by
swapping a running 1.x environment to it after this release, before
v2.0.0 is cut (Open Question 7).

<!--docz:tasks:start-->
#### Tasks

- [ ] `charts/docz/Chart.yaml`: `version: 0.2.0`, `appVersion:
  "2.0.0-beta.6"` (bare), with the chart CHANGELOG and README regenerated
  and the chart's version test updated (Open Question 6). `charts/docz-api`
  and `charts/docz-site` stay at their deprecated finals, and the publish
  job's idempotency check skips them
- [ ] Open the PR with `dont-release`, and push. CI is green
- [ ] **(human)** Merge the PR **with a merge commit**
- [ ] Follow RUNBOOK-0001 Procedure 1 on `main`: `just release-check` and
  `just api release-check` pass
- [ ] **(human)** `just release v2.0.0-beta.6` from the PR's merge
  commit
- [ ] Follow RUNBOOK-0001 Procedure 2's verification steps, recording each
  result here:
  - the pre-release and its archives;
  - both images at `2.0.0-beta.6`, with `latest` not moved;
  - `docz` 0.2.0 signed and attested with a bare `appVersion`;
  - the deprecated charts skipped;
  - ECR skipped.
- [ ] Replace RUNBOOK-0001's Last Verified row with this run, including
  its date, PR, merge commit, and verifier. The note says that Install and
  smoke-test was not run, and records anything the steps got wrong. Fix those steps in the same change. This is the table's
  first real use
- [ ] File the follow-ups as issues:
  - a step-aware docz-site view: step anchors, copy-command buttons,
    procedure navigation, and a Last Verified badge;
  - runbook metadata as structured API fields: owner, service, and last
    verification;
  - one ADR per built-in type, starting with runbook;
  - **v2.0.0 acceptance: swap a running 1.x environment to the latest
    beta and test the whole product.** That includes a repository with
    runbook enabled appearing in docz-api and docz-site. It runs
    RUNBOOK-0001's Install and smoke-test against a real cluster, and it
    gates the v2.0.0 cut. Check first whether #135 or #140 already tracks
    it, and link rather than duplicate.
- [ ] `docz status set impl IMPL-0022 Completed` and
  `docz status set design DESIGN-0019 Implemented`, then `docz update`.
  This goes through a branch and PR, since `main` is protected

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `v2.0.0-beta.6` is published: binaries, both images, and `charts/docz`
  0.2.0, signed and attested
- RUNBOOK-0001 carries a Last Verified row for the beta.6 run
- IMPL-0022 is Completed and DESIGN-0019 Implemented, with the follow-ups
  filed

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:file-changes:start-->
## File Changes

| File | Action | Phase | Description |
| ---- | ------ | ----- | ----------- |
| `pkg/doczcore/config/doctype.go`, `config.go` | Modify | 1 | Registry entry, help suffix, stale `plan` comments |
| `pkg/doczcore/doctemplate/templates/{runbook,index_runbook,schema/runbook}.md` | Create | 1 | Template, index header, skeleton |
| `pkg/doczcore/doctemplate/templates/docz_yaml.tmpl` | Modify | 1 | Six types, how to enable runbook |
| `pkg/doczcore/doctemplate/testdata/golden/runbook.md` | Create | 1 | Template golden |
| `pkg/doczcore/config/*_test.go`, `repo/init_test.go` | Modify | 1 | Six types, disabled default, init count |
| `test/parity/parity.go`, `parity_test.go`, `README.md` | Modify | 1 | `runbook` normaliser, permitted delta |
| `pkg/doczcore/validate/kindrule.go` and tests | Modify | 2 | Nine kinds, `checkSteps` |
| `pkg/doczcore/kinds/infer.go` | Modify | 2 | Only if `Scenario:` does not generalise |
| `.docz.yaml`, `docs/runbook/` | Modify / Create | 3 | Enable runbook, RUNBOOK-0001, RUNBOOK-0002 |
| `pkg/runbook/*` | Create | 4 | The typed reader, corpus, fuzz |
| `pkg/doczcore/layer_test.go`, `repo/migration_test.go` | Modify | 4 | Layer rule, migration corpus |
| `cmd/validate.go`, `create.go`, `init.go` | Modify | 5 | Sixth validator arm, error, help |
| `cmd/runbook_test.go` | Create | 5 | CLI coverage |
| `test/consumer/consumer_v2_test.go`, `doc.go` | Modify | 5 | `pkg/runbook` smoke test |
| `internal/ingest/service_test.go`, `internal/httpapi/typeresolver_test.go`, `internal/e2e/onboard_integration_test.go` | Modify | 6 | API proof |
| `ui/src/lib/{colors,docTypes}.ts`, `colors.test.ts`, `theme/tokens.css`, `mocks/fixtures.ts` | Modify | 6 | Curated colour, blurb, mocks |
| `charts/docz/templates/NOTES.txt`, `README.md.gotmpl`, `README.md` | Modify | 6 | Version-skew note |
| `README.md`, `CLAUDE.md`, `DEVELOPMENT.md`, `CONTRIBUTING.md`, `docs/index.md` | Modify | 7 | Six types |
| `charts/docz/Chart.yaml`, `CHANGELOG.md` | Modify | 8 | 0.2.0 / beta.6 |

<!--docz:file-changes:end-->

<!--docz:testing:start-->
## Testing Plan

- [ ] Registry-looping tests cover runbook with no per-test edits beyond
  the counts and lists Phase 1 names
- [ ] Template ≡ skeleton, the rendered template is clean, inference
  equals markers, and render equals create, all for runbook
- [ ] `just parity` passes against the unchanged v1.2.2 goldens, with the
  `runbook` normaliser unit-tested
- [ ] `pkg/runbook`: grammar tables, Last Verified tables, one test per
  code, corpus goldens, the inference invariant, and `FuzzParse`
- [ ] `InsertRegions` migrates the runbook corpus byte for byte, and the
  second run is a no-op
- [ ] `cmd/runbook_test.go`, and `just test-consumer` with `pkg/runbook`
- [ ] docz-api: the ingest unit test, the resolver case, and the
  real-Postgres e2e test
- [ ] docz-site: the colour, blurb, and nav tests, and CI's ui jobs
- [ ] The published beta.6 artifacts verify: binaries, images, and
  `charts/docz` 0.2.0, signed and attested (Phase 8). The installed,
  whole-product test is the v2.0.0 environment swap, a follow-up

<!--docz:testing:end-->

<!--docz:dependencies:start-->
## Dependencies

- DESIGN-0019's resolved open questions. This plan follows them and
  re-opens none
- Docker for Phase 6's e2e test (testcontainers)
- `v2.0.0-beta.5`'s release path, unchanged: `prerelease.yml`, the
  idempotent chart publish, and the GHCR grants that are already in place

<!--docz:dependencies:end-->

<!--docz:open-questions:start-->
## Open Questions

> Each question is numbered. Option `a` is my recommendation, the later
> letters are alternatives, and the last is "other" for your own answer.
> All eight were resolved on 2026-10-01.

### 1. What goes in the first corpus?

- a. **Two real runbooks and one legacy fixture.**
  - RUNBOOK-0001 (cut a beta) is procedure-first.
  - RUNBOOK-0002 (docz-api webhook deliveries fail) is
    troubleshooting-first. It is drawn from `deploy/tailscale-operator.md`
    and INV-0007.
  - A hand-written unmarked `legacy-onboarding.orig.md` exercises
    inference and `steps.not-ordered`.

  Both shapes are covered by documents this repo actually needs.
- b. RUNBOOK-0001 only, plus synthetic fixtures for the troubleshooting
  shape.
- c. Synthetic fixtures only, with no runbooks written in this repo yet.
- d. Other.

> **Resolved 2026-10-01: (a).**

### 2. Is `Verification.Date` a string or a `time.Time`?

- a. **A string**, as the `Created` frontmatter field is in every `Doc`.
  `Validate` checks its shape (`YYYY-MM-DD`). A consumer that wants a time
  parses it, and `Parse` never fails on a bad date.
- b. `time.Time`, with the zero value for an unparseable cell, so a
  consumer gets a typed value and a bad date silently becomes zero.
- c. Both: `Date string` plus a `Time() (time.Time, bool)` method.
- d. Other.

> **Resolved 2026-10-01: (a).**

### 3. What if `Scenario:` does not generalise to a prefix rule?

`kinds`' `placeholderHeading` pattern expects `<prefix> <token>:`, as in
`Phase 1:` and `Procedure 1:`. `### Scenario: <!-- … -->` has no token.

- a. **Extend `placeholderHeading` to accept a bare `<word>:` placeholder**,
  so `Scenario:` becomes a `Prefix: "Scenario"` rule, and pin it with a
  case. This is one regex, and every type benefits.
- b. Give scenarios a token too (`### Scenario 1: …`). This keeps `kinds`
  unchanged, but it numbers something people describe by symptom.
- c. Hand-write the scenario rule in `pkg/runbook`'s `headings` and skip
  it in the template-derivation test.
- d. Other.

> **Resolved 2026-10-01: (a).**

### 4. How are rollback steps addressed?

- a. **`<token>.R<n>`**, so procedure 2's first rollback step is `2.R1`.
  It never collides with the forward steps, and it reads as "procedure 2,
  rollback 1".
- b. Their own sequence after the forward steps (`2.7`, `2.8`, …), which
  shifts every time a forward step is added.
- c. No IDs. Rollback steps are text only.
- d. Other.

> **Resolved 2026-10-01: (a).**

### 5. How far does the docz-api e2e test go?

- a. **Types, docs list, and doc fetch in the onboard e2e, plus search
  only if the Meilisearch e2e helpers make it a few lines.** Search is
  generic over `type`, and a new test file for it would test Meilisearch,
  not runbook.
- b. Every surface, including search, in a new `runbook_integration_test.go`.
- c. Unit tests only (ingest + resolver), with no e2e.
- d. Other.

> **Resolved 2026-10-01: (a).**

### 6. What version does `charts/docz` take?

- a. **0.2.0.** A new `appVersion` with new user-facing behaviour is a
  minor bump on a 0.x chart, and the NOTES text changes.
- b. 0.1.1, since the chart's templates barely change.
- c. Other.

> **Resolved 2026-10-01: (a).**

### 7. How does the Phase 8 install prove a runbook reaches the site?

- a. **Onboard this repository itself** with `-onboard`, using a GitHub
  App installation and a real key, to a throwaway kind install. This
  repo enables runbook in Phase 3, so the site should list Runbooks with
  RUNBOOK-0001 and 0002. It is the real path end to end.
- b. Rely on Phase 6's e2e test against the same commit, and have the kind
  install check only `/api/v1/repos` and `helm test`, as beta.5 did.
- c. Seed Postgres directly with a runbook row.
- d. Other.

> **Resolved 2026-10-01: (d).** No install test at the release. Phase 8
> checks only that the artifacts are published, signed, and attested.
> Phase 6's e2e test already proves an enabled runbook reaches the API from
> the same commit. The whole beta, including runbook in docz-site, is
> tested after the release by swapping a running 1.x environment to it.
> That is part of v2.0.0 acceptance and is filed as a follow-up.
> RUNBOOK-0001 keeps the install as its own procedure, which is skipped
> for a beta.

### 8. Does Phase 3 enable runbook before the typed reader exists?

- a. **Yes.** It is enabled and written in Phase 3, validated by the
  generic tier until Phase 5 adds the typed tier. That lets the reader be
  built against real documents, and the generic tier already catches
  shape errors.
- b. No. Write the runbooks in Phase 3 but enable the type in Phase 5, so
  they sit in `docs/runbook/` unlisted and unindexed for two phases.
- c. Move the runbooks to after Phase 5 and build the reader against
  synthetic fixtures only.
- d. Other.

> **Resolved 2026-10-01: (a).**

<!--docz:open-questions:end-->

<!--docz:references:start-->
## References

- [DESIGN-0019](../design/0019-runbook-a-sixth-built-in-document-type-disabled-by-default.md):
  the design this plan implements
- [ADR-0002](../adr/0002-docz-is-an-api-package-whose-first-consumer-is-the-cli.md):
  every built-in is a structured type
- [ADR-0003](../adr/0003-remove-plan-from-the-built-in-document-types.md):
  the last disabled-by-default built-in, and the parity normaliser
  precedent
- [IMPL-0021](0021-chartsdocz-one-helm-chart-v200-beta5.md): Phase 6,
  the release procedure RUNBOOK-0001 writes down
- [INV-0007](../archive/api/investigation/0007-ingest-failures-are-silent-and-block-re-ingestion.md):
  the ingest failure modes RUNBOOK-0002 draws on

<!--docz:references:end-->
