---
id: IMPL-0025
title: "Pre-cut markdown and lint fixes"
status: Draft
author: Donald Gifford
created: 2026-10-12
---

<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL-0025: Pre-cut markdown and lint fixes

<!--toc:start-->
<!-- markdownlint-capture -->
<!-- markdownlint-disable MD013 -->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: Heading anchors that match GitHub (#82, #96)](#phase-1-heading-anchors-that-match-github-82-96)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2: Index table, templates, and in-document lint settings (#127)](#phase-2-index-table-templates-and-in-document-lint-settings-127)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 3: The lint gate](#phase-3-the-lint-gate)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 4: A fresh repository validates clean (#112)](#phase-4-a-fresh-repository-validates-clean-112)
    - [Tasks](#tasks-3)
    - [Success Criteria](#success-criteria-3)
  - [Phase 5: Custom types without a template (#92)](#phase-5-custom-types-without-a-template-92)
    - [Tasks](#tasks-4)
    - [Success Criteria](#success-criteria-4)
  - [Phase 6: The blob SHA label in docz-site (#169)](#phase-6-the-blob-sha-label-in-docz-site-169)
    - [Tasks](#tasks-5)
    - [Success Criteria](#success-criteria-5)
  - [Phase 7: Release notes and close-out](#phase-7-release-notes-and-close-out)
    - [Tasks](#tasks-6)
    - [Success Criteria](#success-criteria-6)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Dependencies](#dependencies)
- [Open Questions](#open-questions)
  - [1. Which walkers learn about HTML comment blocks?](#1-which-walkers-learn-about-html-comment-blocks)
  - [2. How do IMPL and runbook stop failing MD024?](#2-how-do-impl-and-runbook-stop-failing-md024)
  - [3. What does docz do about MD013 on lines it writes from content?](#3-what-does-docz-do-about-md013-on-lines-it-writes-from-content)
  - [4. How does the parity suite accept the changed bytes?](#4-how-does-the-parity-suite-accept-the-changed-bytes)
  - [5. What shape is the lint gate?](#5-what-shape-is-the-lint-gate)
  - [6. Does Phase 3 also gate this repository's own docs/ (#98)?](#6-does-phase-3-also-gate-this-repositorys-own-docs-98)
  - [7. Where does the #92 fallback live?](#7-where-does-the-92-fallback-live)
  - [8. Should docz validate --format json count index drift?](#8-should-docz-validate---format-json-count-index-drift)
  - [9. What does the reader show instead of "Commit"?](#9-what-does-the-reader-show-instead-of-commit)
  - [10. How does this ship?](#10-how-does-this-ship)
  - [11. Does this document merge on its own first?](#11-does-this-document-merge-on-its-own-first)
- [References](#references)
<!-- markdownlint-restore -->
<!--toc:end-->

<!--docz:objective:start-->
## Objective

Make everything docz writes pass markdownlint. That covers the embedded
templates, the ToC region, the README index region, and the wiki landing
page. Fix the three other defects that a brand-new repository hits on its
first day, and pin all of it with a gate. Everything here lands before the
v2.0.0 cut, ahead of #133, #135, and #140.

The cost today is a loop. An agent runs `docz update`, markdownlint fails on
bytes docz generated, the agent hand-fixes them, and the next `docz update`
puts them back. Nothing a person typed is involved. A fresh `docz init`,
followed by one `docz create` per built-in and `docz update`, fails
markdownlint-cli2 0.23.3 with default rules in 48 places:

| Rule | Count | Where | Cause |
| --- | --- | --- | --- |
| MD060 | 36 | every README index | `index.GenerateTable` writes an unpadded separator row (`\|----\|`) under padded rows |
| MD051 | 5 | IMPL and runbook ToCs | the phase, procedure, and scenario headings carry their placeholder as an inline comment, and `AnchorSlug` slugs the comment |
| MD024 | 5 | IMPL and runbook bodies | `#### Tasks`, `#### Success Criteria`, and `#### Steps` repeat by design |
| MD013 | 1 | RFC body | a 90-character guidance comment in the template |
| MD012 | 1 | `docs/index.md` | the wiki index template ends with two blank lines |

Headings with an underscore (#82), a non-ASCII letter (#96), or a `##` line
inside a comment block (#96) add more MD051 findings once real content
arrives. The repository's `mise.toml` pins markdownlint-cli2 0.18.1, which
predates MD060, so CI has never seen the table failures.

**Implements:** #82, #96, #127, #112, #92, #169; INV-0009 Decisions 3 and 5
(the slug fix and the gate)

<!--docz:objective:end-->

<!--docz:scope:start-->
## Scope

<!--docz:in-scope:start-->
### In Scope

- `docparse.AnchorSlug` and `docparse.Headings` produce the anchors GitHub,
  markdownlint MD051, and docz-site's `rehype-slug` produce. All three use
  github-slugger's rules (#82, #96)
- The README index separator row, the embedded templates' long lines, the
  wiki index's trailing blank line, and the in-document lint directives
  docz needs where it cannot change what it writes (#127)
- A gate: `just lint-generated` scaffolds a repository with every command
  that writes markdown, then lints the result with markdownlint-cli2 0.23.3,
  once with default rules and once with this repository's config. It runs
  in CI
- `docz init` writes the README index a render would write, so a fresh
  repository passes `docz validate --strict` (#112)
- A custom type with no template falls back to the embedded generic
  template, as the README already promises (#92)
- docz-site stops labelling a blob SHA "Commit", and the spec describes
  document `git_sha` (#169)
- Parity normalisers for every byte these fixes change in a v1.2.2 golden,
  each tied to its issue

<!--docz:in-scope:end-->

<!--docz:out-of-scope:start-->
### Out of Scope

- Linting this repository's own `docs/` in CI (#98). Its 880 findings
  under 0.23.3 are mostly hand-written prose and tables. Phase 2 removes the
  index rows among them, but the rest is the pre-cut revisit unless
  question 6 says otherwise
- Wrapping a long title. The H1 is written once from the title a person
  chose, and docz does not rewrite it (question 3)
- The claude-skills docz plugin's bundled fallback templates. The plugin
  re-syncs after v2.0.0 proper
- The other open issues the pre-cut revisit will triage

<!--docz:out-of-scope:end-->
<!--docz:scope:end-->

## Implementation Phases

All seven phases land in one pull request from `fix/pre-cut-lint-fixes`,
labelled `dont-release`. The phases run in this order on that branch, each
as its own commits, so the history reads phase by phase and a phase can be
reverted on its own. A phase is complete when all its tasks are checked
off and its success criteria are met. The pull request is ready for review
when every phase is complete and `just ci` passes on the branch. Phases 1 and 2 change consumer-visible bytes in frozen v1 packages
(`docparse`, `toc`, `index`): a behaviour fix toward their documented
contract, not a shape change. The release notes say so.

---

<!--docz:phase:start-->
### Phase 1: Heading anchors that match GitHub (#82, #96)

After this phase every anchor docz writes resolves on GitHub, in
markdownlint, and in docz-site. Ground truth is github-slugger, which
markdownlint's MD051 and the site's `rehype-slug` both implement. Its rules
were checked against markdownlint 0.23.3 while this document was written:

- lowercase
- keep Unicode letters, marks, numbers, `_`, `-`, and spaces
- spaces become `-`
- no trimming
- duplicates get `-1`, `-2`
- the slug input is the heading's rendered text: HTML comments and inline
  tags are removed, but the spaces around them stay

So `### Phase 1: <!-- Foundation -->` gives `#phase-1-`, and
`## Foo <!-- c --> bar` gives `#foo--bar`.

<!--docz:tasks:start-->
#### Tasks

- [x] Add `pkg/doczcore/docparse/testdata/anchors.md`. It holds a heading
  for each case, each followed by a hand-written link to its anchor:
  - underscores, including intraword `a_b_c` and `id_prefix`
  - non-ASCII letters (`Über uns`, `Café résumé`)
  - digits and punctuation runs
  - a trailing comment, a mid-heading comment, and a comment-only heading
  - inline code, emphasis, and links
  - an inline `<code>` tag
  - a closing ATX sequence (`## Foo ##`)
  - leading and trailing hyphens
  - emoji
  - three duplicates, plus a literal `dup-1` heading that collides with a
    generated suffix
- [x] Confirm `anchors.md` passes markdownlint-cli2 0.23.3 MD051 before
  any Go changes. It is the ground truth from here on, and Phase 3's gate
  lints it on every run
- [x] `AnchorSlug`: replace the `[a-z0-9 -]` filter with github-slugger's
  character class (Unicode L, M, N, and Pc, plus space and `-`). Drop
  `strings.Trim(s, "-")`. Rewrite the doc comment to name github-slugger as
  the contract
- [x] `Headings`: build the slug input from the raw heading text. Strip the
  closing ATX sequence, remove HTML comments and inline tags (keeping the
  spaces around them), strip inline markdown, then slug. `Heading.Text` keeps
  its comments: `pkg/impl`, `kinds.InferRegions`, and `kinds.Alternatives`
  read the placeholder comment from it
- [x] `stripInlineMarkdown`: `_` emphasis only opens and closes at a word
  boundary (CommonMark's intraword rule), so `a_b_c` stays `a_b_c` in both
  `Text` and the slug
- [x] Duplicate suffixes follow github-slugger's loop. When `dup-1` is
  already taken, the counter moves on to `dup-2`, and a generated suffix is
  itself recorded as taken
- [x] Comment awareness (question 1): a line whose first non-space
  characters are `<!--`, with no `-->` after them on the same line, opens a
  comment block. The block ends on the first line containing `-->`. That is
  CommonMark's HTML-block type 2. Lines inside it are skipped by the walkers
  question 1 picks. Fences are tested first, so `<!--` inside a fence opens
  nothing. A single-line comment, which includes every `<!--docz:…-->`
  marker, never opens a block. `Markers` and `Regions` are untouched
- [x] `TestAnchorsMatchGitHub`: run `toc.GenerateToC` over `anchors.md`.
  Every generated link must equal the hand-written link under its heading
- [x] Unit tests for the comment-block rule:
  - a heading inside a block is skipped
  - a heading carrying a single-line comment is kept
  - an unclosed block runs to EOF
  - a fence inside a comment and a comment inside a fence
  - plus the same cases for each other walker question 1 picks
- [x] Regenerate the goldens (`go test ./... -update`) for `docparse`,
  `toc`, `kinds`, the six type packages, `repo`, and
  `pkg/export/confluence`. Review each diff, which should only touch slugs
  and anything that cites them. Confluence's anchor map
  (`render.go`, `out[h.Slug]`) now resolves the links the regenerated ToCs
  carry
- [x] Parity: add the `lint` normaliser (question 4) to
  `test/parity/parity.go`. It applies to both sides in `runCase` and holds
  an explicit table of v1.2.2 lines and their v2 replacements, each entry
  naming its issue. Phase 1's entries are the three IMPL phase ToC lines
  plus any other slug the replay reports. There are no regex catch-alls, so
  an unlisted change is still a regression. Unit-test the normaliser.
  *Built:* the table maps each v2 line back to its v1.2.2 line, and
  `LintNormalizer` sits in the per-run `norms` list beside `markers`. A
  comparison-time normaliser could not work: v2 rewrites the `impl`
  fixture's stale ToC, so it records as written a file whose v1.2.2
  golden has only a size and digest. v1.2.2 never writes a v2 line, so
  the rewrite is still symmetric and no golden is re-captured
- [x] `docz update` over this repository's `docs/`. Commit the regenerated
  ToCs separately from the code
- [x] CLAUDE.md: update the `docparse` slug and comment rules and the parity
  normaliser list
- [x] `just ci` passes. *Locally `license-check` fails the same way on a
  clean `main` (the installed go-licenses rejects standard-library
  packages); every other gate passes*

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `anchors.md` passes MD051, and docz's ToC for it equals its hand-written
  links
- A fresh IMPL and runbook have no MD051 finding under markdownlint-cli2
  0.23.3
- The #82 and #96 reproductions produce the anchors their issues expect
- `just parity` passes with the `lint` normaliser and no other change to a
  golden
- `docz validate` over this repository reports no `toc.stale`

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 2: Index table, templates, and in-document lint settings (#127)

After this phase a fresh scaffold passes default-rule markdownlint. docz
fixes what it writes wherever it can. It adds an in-document directive only
where the repetition or the length is the point, and the directive never
re-enables a rule the repository turned off. Both behaviours were checked
against 0.23.3:

- a `markdownlint-configure-file` object turns a disabled rule back on
- `markdownlint-enable` turns a config-disabled rule back on
- `markdownlint-capture` followed by `markdownlint-restore` does neither

<!--docz:tasks:start-->
#### Tasks

- [x] `index.GenerateTable`: the separator row becomes
  `| --- | --- | --- | --- | --- | --- |`, which MD060 accepts under padded
  rows. Update the `index` goldens
- [x] MD024 (question 2): add `MD024` to the existing
  `markdownlint-disable-file` line in `impl.md` and `runbook.md`. Update
  each template's `templates/schema/` skeleton too if the derivation test
  says it must change
- [x] MD013 (question 3): docz writes these lines from document content and
  regenerates them on every `docz update`. The index region gets:

  ```markdown
  <!-- markdownlint-capture -->
  <!-- markdownlint-disable MD013 -->
  ...table...
  <!-- markdownlint-restore -->
  ```

  `toc.GenerateToC` wraps the ToC list the same way. Both are inside
  docz's own markers, so a re-render rewrites them, and `toc.stale` and
  `index.drift` compare against the same bytes
- [x] Wrap the three embedded template lines over 80 characters:
  `rfc.md:26`, `runbook.md:26`, and `index_impl.md:7`. Add a test in
  `doctemplate` that fails on any embedded template line over 80 characters
  outside a table or a fence
- [x] `wiki_index.md`: end with exactly one newline (MD012), and update the
  `pkg/wiki` goldens
- [x] Check the generic `default.md` and every `index_*.md` header against
  default rules, and fix what they report. *`default.md` is clean. Each
  header ends with the blank line that separates it from the marker pair,
  which is MD012 only when the header is linted alone; a generated README
  passes, so nothing changed*
- [x] Parity: add entries to the `lint` normaliser for:
  - the separator row
  - the three wrapped lines
  - the capture, disable, and restore lines, dropped on both sides the way
    `MarkerNormalizer` drops markers
  - the IMPL and runbook `disable-file` lines
  - the wiki index's trailing blank line

  *Added with each change, so every commit replays clean. The wrapper
  lines also make every fixture's v1.2.2 ToC stale, so each dry-run line
  reporting one is listed, and the wiki ending needed an end-of-file
  anchored entry per last list line*
- [x] `docz update` over this repository's `docs/`. Commit the regenerated
  READMEs and ToCs separately
- [x] CLAUDE.md: the `index` and `toc` paragraphs, and the template rule
  (`MD025 MD041` gains `MD024` for IMPL and runbook)
- [x] `just ci` passes (`license-check` aside, as in Phase 1)

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- The scaffold from the Objective has zero markdownlint-cli2 0.23.3
  findings, both with default rules and with this repository's
  `.markdownlint.yaml`
- With `MD013` and `MD024` turned off in the repository's config, the
  scaffold still has zero findings, and a long prose line a person adds is
  still not reported. Together these show no directive re-enabled a rule
- `just parity` passes, and every new normaliser entry names its issue

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 3: The lint gate

After this phase a change that makes docz's output fail markdownlint fails
CI in the same pull request.

<!--docz:tasks:start-->
#### Tasks

- [x] `mise.toml`: markdownlint-cli2 `0.18.1` → `0.23.3`. Renovate keeps it
  current from here
- [x] `just lint-generated` (`docz.just`, question 5): build docz, then in
  a temp directory:
  - `git init` and `docz init`
  - turn the runbook on, and add a custom `frameworks` type with no
    template (it starts working in Phase 5; until then the recipe skips
    it)
  - one `docz create` per type, with titles chosen to give long index rows
    and ToC lines
  - `docz update` and `docz wiki init`
  - `docz template override` for one type
  - copy `anchors.md` in

  Then run markdownlint-cli2 twice, once with `{}` (default rules) and once
  with this repository's `.markdownlint.yaml`, and fail on any finding.
  Print the temp path on failure
- [x] CI: a `lint-generated` step in the `test-go` job, which runs on the
  `go` path filter. Install markdownlint-cli2 at the version in `mise.toml`,
  read from the file so there is one pin. Add the recipe to `just ci`
- [x] Prove the gate catches each regression by reverting it locally:
  - restore the old separator row: MD060
  - restore the `[a-z0-9 -]` slug filter: MD051
  - drop `MD024` from the IMPL `disable-file` line: MD024

  Record the three results in the pull request description.
  *Results: 36 × MD060, 6 × MD051, 8 × MD024. The slug drill first passed,
  because no generated ToC carried an underscore or a non-ASCII letter, so
  the gate now files `anchors.md` as an RFC with a ToC region and lints
  the ToC docz writes for it*
- [x] Comment on #98: the generated output is gated here, and what is left
  is this repository's own `docs/` (question 6)
- [x] CLAUDE.md: the recipe, the CI step, and the version pin

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `just lint-generated` passes locally and in CI
- Each of the three reverts fails the gate with the expected rule
- `just ci` runs the gate

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 4: A fresh repository validates clean (#112)

After this phase `docz init` alone leaves a repository that
`docz validate --strict` accepts.

<!--docz:tasks:start-->
#### Tasks

- [x] `repo.Init`: write each README as `index.Scaffold(header)` spliced
  with `index.GenerateTable(nil, "All "+label)`. That is the bytes
  `updateType` writes for a type with no documents. It stays
  scaffold-then-splice, so there is still exactly one marker pair (#99).
  Share one helper with `update.go:139` and `validate.go:480`, so all three
  render the empty table the same way
- [x] Test: `Init` followed by `Validate` gives no `index.drift`. `Init`
  followed by `Update` writes nothing. `index.Scaffold` on a header that
  ends in a pair still yields one pair
- [x] `--format json` (question 8): drift stays in the separate `index`
  array, and `docz validate --help` and the README say so in one sentence
- [ ] Parity: an empty-table collapse, applied to both sides after the
  `lint` normaliser, so an index region holding only a heading and a header
  table reads as an empty pair. Count the 14 init goldens it touches
- [ ] `just lint-generated` runs `docz validate --strict` straight after
  `docz init` and expects exit 0
- [ ] CLAUDE.md: the `repo.Init` paragraph

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- The #112 reproduction prints `0` drift and exits 0 under `--strict`
- `just parity` passes
- `docz init` still writes one marker pair per README

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 5: Custom types without a template (#92)

After this phase a custom type with no template creates, shows, and
validates with the embedded generic template, as the README says. A note
on stderr points at `docz template override` when that happens.

<!--docz:tasks:start-->
#### Tasks

- [ ] `doctemplate.ResolveOrGeneric(docType, configPath, docsDir)
  (body string, generic bool, err error)` (question 7). It runs the three
  tiers and falls back to `GenericTemplate()` on `ErrNoTemplate` only.
  `Resolve` keeps its contract, and a configured path that does not exist
  is still an error
- [ ] Callers: `docwrite.Render`, `repo.Template` (show), and
  `repo.Validate`'s template tier. `repo.ExportTemplate` keeps calling
  `Resolve`, so `docz template override` still scaffolds the generic pair
  with `Scaffolded: true`
- [ ] `repo.Validate`: a type on the generic fallback validates against the
  embedded `default` schema. It no longer reports `template.unresolved` or
  `schema.unresolved`
- [ ] `repo.CreateResult` gains `GenericTemplate bool`. `docz create` prints
  `note: <type> has no template; used the generic one. Run "docz template
  override <type>" to write one.` on stderr. `docz template show` prints the
  same note
- [ ] A legacy repository with `types.plan` enabled and no template now
  creates a generic document instead of failing (ADR-0003). The parity
  fixture's `plan` block is dormant, so no golden moves. Note this in
  `docs/migrating-to-v2.md`
- [ ] Tests: create, show, and validate for a template-less custom type, in
  `repo` and in `cmd` (a new test file; `cmd/*_test.go` stays frozen).
  Extend `test/consumer`'s `Create` coverage to a template-less custom type
- [ ] `just lint-generated` stops skipping the template-less type
- [ ] README § Custom Document Types and CLAUDE.md: the fallback and the
  note

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- The #92 reproduction creates `FW-0001` from the generic template, and
  `docz validate --strict` passes on it
- `docz template override frameworks` still writes the template and its
  schema
- `just parity` and `just test-consumer` pass

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 6: The blob SHA label in docz-site (#169)

After this phase the document reader no longer shows a blob SHA as a
commit, and the contract says what `git_sha` is.

<!--docz:tasks:start-->
#### Tasks

- [ ] `ui/src/components/doc-meta-table.tsx`: apply question 9's choice to
  the `git_sha` row, and update `ui/src/routes/doc.test.tsx`
- [ ] `api/openapi.yaml`: describe `git_sha` on `Document` and
  `DocumentSummary` the way `PageSummary` does. Bump `info.version` from
  `1.6.0` to `1.6.1` (editorial)
- [ ] `just ui ci` and `just api lint` pass, and `gen-api-check` shows the
  regenerated client
- [ ] CLAUDE.md: the spec version

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- The reader shows no "Commit" row backed by a blob SHA
- The contract test and vacuum pass on spec 1.6.1

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 7: Release notes and close-out

<!--docz:tasks:start-->
#### Tasks

- [ ] `docs/migrating-to-v2.md`: the first `docz update` on v2 rewrites
  every ToC with an affected heading and every README index table once.
  `docz validate` reports `toc.stale` until it runs. Also cover the
  `disable-file` and capture lines, and the generic-template fallback
- [ ] Draft the release notes for question 10's beta. Name the three frozen
  packages whose output changed and why
- [ ] The pull request closes #82, #96, #127, #112, #92, and #169
  (`Closes` lines in its description). Comment on #98 with what is left
- [ ] Set this document to Completed
- [ ] Human: cut the beta (`just release`), then do #133, #135, and #140

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- All six issues are closed
- A repository scaffolded by the release binary passes
  `just lint-generated`'s two configs and `docz validate --strict`

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:file-changes:start-->
## File Changes

| File | Action | Description |
| --- | --- | --- |
| `pkg/doczcore/docparse/headings.go` | Modify | github-slugger slugs, comment blocks, closing ATX sequence, intraword `_` |
| `pkg/doczcore/docparse/testdata/anchors.md` | Create | slug ground truth, linted by the gate |
| `pkg/doczcore/docparse/{tasks,title}.go` | Modify | comment blocks, if question 1 picks them |
| `pkg/doczcore/toc/toc.go` | Modify | capture, disable MD013, restore around the list |
| `pkg/doczcore/index/index.go` | Modify | padded separator; capture, disable MD013, restore |
| `pkg/doczcore/doctemplate/templates/*.md` | Modify | `MD024` on IMPL and runbook, three wrapped lines, wiki index newline |
| `pkg/doczcore/doctemplate/template.go` | Modify | `ResolveOrGeneric` |
| `pkg/doczcore/docwrite/create.go` | Modify | `Render` uses the generic fallback |
| `pkg/doczcore/repo/{init,update,validate,template,create}.go` | Modify | empty-table helper, generic fallback, `GenericTemplate` |
| `cmd/{create,template}.go` | Modify | the stderr note |
| `test/parity/parity.go` | Modify | the `lint` normaliser and the empty-table collapse |
| `docz.just`, `justfile`, `mise.toml`, `.github/workflows/ci.yml` | Modify | `lint-generated`, the 0.23.3 pin, the CI step |
| `ui/src/components/doc-meta-table.tsx`, `ui/src/routes/doc.test.tsx` | Modify | the `git_sha` row |
| `api/openapi.yaml` | Modify | `git_sha` descriptions, 1.6.1 |
| `docs/**` | Modify | regenerated ToCs and README tables |
| `README.md`, `docs/migrating-to-v2.md`, `CLAUDE.md` | Modify | custom-type fallback, upgrade note, conventions |

<!--docz:file-changes:end-->

<!--docz:testing:start-->
## Testing Plan

- [ ] `anchors.md` is checked by markdownlint (MD051) and by
  `TestAnchorsMatchGitHub`, so the slug has one external ground truth
- [ ] Every regenerated golden is reviewed as a diff, not accepted wholesale
- [ ] `just lint-generated` with default rules and with the repository
  config, plus the three revert drills from Phase 3
- [ ] `just parity`: every delta is a named normaliser entry tied to an issue
- [ ] `docz validate --strict` straight after `docz init`
- [ ] Template-less custom type: create, show, validate, and override, in
  `repo`, `cmd`, and `test/consumer`
- [ ] `just ui ci` for the reader change

<!--docz:testing:end-->

<!--docz:dependencies:start-->
## Dependencies

- markdownlint-cli2 0.23.3 (markdownlint 0.41.1), for MD060 and current MD051
- github-slugger's rules, reached through markdownlint and `rehype-slug`.
  There is no new Go or npm dependency
- #133, #135, #140, and the v2.0.0 cut wait for this document

<!--docz:dependencies:end-->

<!--docz:open-questions:start-->
## Open Questions

### 1. Which walkers learn about HTML comment blocks?

- **a. `Headings`, `Title`, and `TaskItems`, as INV-0009 Decision 3
  says.** *(recommendation)* A task in a comment is not a task, and
  `CheckTask` validates its line through `TaskItems`
- b. `Headings` only, which is all MD051 needs.
- c. Every fact walker except `Markers` and `Regions` (`ListItems` and
  `Tables` too). This is the most consistent, but it moves the runbook step
  goldens.
- d. Other.

> **Resolved 2026-10-12: (a).** Headings, Title, and TaskItems, as INV-0009 Decision 3 said.

### 2. How do IMPL and runbook stop failing MD024?

- **a. Add `MD024` to the `markdownlint-disable-file` line both templates
  already carry.** *(recommendation)* One line that already exists, and it
  never turns a rule on
- b. `markdownlint-configure-file` with `siblings_only`. This keeps the
  check, but it re-enables MD024 in a repository that turned it off
  (checked against 0.23.3).
- c. Make the headings unique (`#### Phase 1 tasks`). This breaks the
  heading specs, inference, and every existing IMPL.
- d. Other.

> **Resolved 2026-10-12: (a).** MD024 joins the existing disable-file line in the IMPL and runbook templates.

### 3. What does docz do about MD013 on lines it writes from content?

- **a. Capture, disable MD013, and restore inside the ToC and index
  regions, the lines `docz update` rewrites. Nothing on the H1.**
  *(recommendation)* Regenerated lines are what make agents loop. The H1 is
  written once, and the person who chose the title can shorten it
- b. (a), plus `markdownlint-disable-next-line MD013` above the H1 in every
  template.
- c. Add `MD013` to the file-level `disable-file` line. This is simplest,
  but it switches line length off for the author's prose too.
- d. Nothing; MD013 is the repository's config to set.
- e. Other.

> **Resolved 2026-10-12: (a).** Capture, disable, and restore inside the ToC and index regions only; the H1 is left alone.

### 4. How does the parity suite accept the changed bytes?

- **a. A `lint` normaliser applied to both sides, with an explicit table of
  v1.2.2 lines and their v2 replacements (each naming its issue), a list of
  directive lines to drop, and an empty-table collapse for #112.**
  *(recommendation)* It is the same pattern as `plan`, `runbook`, and
  `sync`, and anything not in the table is still a regression
- b. An expected-delta file beside each affected golden. This is more
  precise per case, but it means about 50 new files.
- c. Other.

> **Resolved 2026-10-12: (a).** One `lint` normaliser with an explicit per-issue table, applied to both sides.

### 5. What shape is the lint gate?

- **a. A `just lint-generated` recipe (a bash body), with a step in the
  `test-go` CI job and a place in `just ci`.** *(recommendation)* It is
  readable, runnable by hand, and prints the temp repository to inspect
- b. A Go test behind a build tag that runs `markdownlint-cli2` through
  `os/exec`, like the parity driver.
- c. Fold it into `just parity`.
- d. Other.

> **Resolved 2026-10-12: (a).** A `just lint-generated` recipe, in the `test-go` CI job and `just ci`.

### 6. Does Phase 3 also gate this repository's own docs/ (#98)?

- **a. No. Gate generated output now, and leave #98 for the pre-cut
  revisit.** *(recommendation)* What remains after Phase 2 is hand-written
  content, and fixing it is a separate, mechanical pull request
- b. Yes. Fix every finding and gate `docs/` in Phase 3.
- c. Other.

> **Resolved 2026-10-12: (a).** Generated output only; #98 waits for the pre-cut revisit.

### 7. Where does the #92 fallback live?

- **a. A new `doctemplate.ResolveOrGeneric` that reports when it fell back,
  used by `docwrite.Render`, `repo.Template`, and `repo.Validate`.
  `Resolve` and `ExportTemplate`'s scaffolding keep their contracts.**
  *(recommendation)* A legacy `plan` block enabled with no template then
  creates a generic document instead of failing
- b. A fourth tier inside `Resolve`. There is one resolver, but
  `ErrNoTemplate` becomes unreachable, and override can no longer tell
  "nothing resolved" from "fell back".
- c. Change the README to say a custom type needs a template, and improve
  the error. No behaviour change.
- d. Other.

> **Resolved 2026-10-12: (a).** `doctemplate.ResolveOrGeneric`; `Resolve` and `ExportTemplate` keep their contracts.

### 8. Should `docz validate --format json` count index drift?

- **a. No. Keep drift in the separate `index` array and document it in
  `--help` and the README.** *(recommendation)* Drift is about a README,
  not a document's finding, and Phase 4 removes the case that surprised
  people
- b. Count drift in `warnings` as well.
- c. Other.

> **Resolved 2026-10-12: (a).** Drift stays in the `index` array and is documented.

### 9. What does the reader show instead of "Commit"?

- **a. Drop the row.** *(recommendation)* A truncated blob SHA is a cache
  key and tells a reader nothing
- b. Rename it "Blob".
- c. Replace it with the repository's `last_synced_sha`, linked as "Synced
  from", which needs the repository record on the reader page.
- d. Other.

> **Resolved 2026-10-12: (a).** Drop the row. The value stays in docz-api's database, so a view can come back later if it is ever wanted.

### 10. How does this ship?

- **a. One beta (`v2.0.0-beta.9`) after Phase 7, before #133, #135, and
  #140.** *(recommendation)* The homelab and real repositories exercise
  the regenerated output before the cut
- b. No beta. Go straight to the v2.0.0 cut once the other three land.
- c. A beta per phase.
- d. Other.

> **Resolved 2026-10-12: (a).** One beta, `v2.0.0-beta.9`, after Phase 7.

### 11. Does this document merge on its own first?

- **a. Yes. This docs pull request merges, then Phase 1 starts from `main`
  and checks its boxes in its own pull request.** *(recommendation)*
- b. It rides with Phase 1's pull request.
- c. Other.

> **Resolved 2026-10-12: (a).** This document merges on its own as a docs pull request; the work starts from `main`. Amended the same day: every phase lands in a single pull request rather than one per phase.

<!--docz:open-questions:end-->

<!--docz:references:start-->
## References

- [#127](https://github.com/donaldgifford/docz/issues/127): docz-generated markdown must pass markdownlint
- [#82](https://github.com/donaldgifford/docz/issues/82) and [#96](https://github.com/donaldgifford/docz/issues/96): anchor slugs
- [#112](https://github.com/donaldgifford/docz/issues/112): `docz init` leaves README drift
- [#92](https://github.com/donaldgifford/docz/issues/92): custom types without a template
- [#169](https://github.com/donaldgifford/docz/issues/169): blob SHA labelled "Commit"
- [#98](https://github.com/donaldgifford/docz/issues/98): markdownlint over this repository's `docs/`
- [INV-0009](../investigation/0009-toc-regeneration-in-docz-update-and-markdownlint-md051.md):
  ToC regeneration and MD051 (Decisions 3 and 5)
- [ADR-0003](../adr/0003-remove-plan-from-the-built-in-document-types.md):
  removing the `plan` type
- [DESIGN-0015](../design/0015-structured-regions-and-docz-validate.md):
  region markers and validation
- [github-slugger](https://github.com/Flet/github-slugger)
- markdownlint [MD051](https://github.com/DavidAnson/markdownlint/blob/main/doc/md051.md),
  [MD060](https://github.com/DavidAnson/markdownlint/blob/main/doc/md060.md),
  and [inline configuration](https://github.com/DavidAnson/markdownlint#configuration)

<!--docz:references:end-->
