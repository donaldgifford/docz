---
id: IMPL-0027
title: "docz-review: the CLI contract, the review page, the binary, and the plugin"
status: Draft
author: Donald Gifford
created: 2026-10-25
---

<!-- markdownlint-disable-file MD024 MD025 MD041 -->

# IMPL-0027: docz-review: the CLI contract, the review page, the binary, and the plugin

<!--toc:start-->
<!-- markdownlint-capture -->
<!-- markdownlint-disable MD013 -->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: The hook drill](#phase-1-the-hook-drill)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2: The CLI's JSON contract](#phase-2-the-clis-json-contract)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 3: Generated types and the review page](#phase-3-generated-types-and-the-review-page)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 4: The server, the state machine, and the binary](#phase-4-the-server-the-state-machine-and-the-binary)
    - [Tasks](#tasks-3)
    - [Success Criteria](#success-criteria-3)
  - [Phase 5: The plugin](#phase-5-the-plugin)
    - [Tasks](#tasks-4)
    - [Success Criteria](#success-criteria-4)
  - [Phase 6: Release and close-out](#phase-6-release-and-close-out)
    - [Tasks](#tasks-5)
    - [Success Criteria](#success-criteria-5)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Dependencies](#dependencies)
- [Open Questions](#open-questions)
  - [1. How is the work split into pull requests?](#1-how-is-the-work-split-into-pull-requests)
  - [2. Which JSON Schema validator does the Go contract test use?](#2-which-json-schema-validator-does-the-go-contract-test-use)
  - [3. Where do the hook drill's write-up and fixtures live?](#3-where-do-the-hook-drills-write-up-and-fixtures-live)
  - [4. What repository does e2e-review run against?](#4-what-repository-does-e2e-review-run-against)
  - [5. Which runtime validator does docz-review use?](#5-which-runtime-validator-does-docz-review-use)
  - [6. How much Playwright does the review flow get?](#6-how-much-playwright-does-the-review-flow-get)
  - [7. Where are the four compiled binaries built?](#7-where-are-the-four-compiled-binaries-built)
  - [8. When does the docz plugin's create skill start saying --format json?](#8-when-does-the-docz-plugins-create-skill-start-saying---format-json)
  - [9. What is the schemas' $id base?](#9-what-is-the-schemas-id-base)
- [References](#references)
<!-- markdownlint-restore -->
<!--toc:end-->

<!--docz:objective:start-->
## Objective

Build `docz-review` as DESIGN-0024 specifies: the docz CLI's JSON
contract (`docz inspect`, a single-document `docz validate`,
`docz create --format json`, committed schemas, a Go contract test), the
review page and its Bun server as a second entry point in `ui/`, the
compiled binaries on the same release as `docz`, and the `docz-review`
Claude Code plugin, which lives in this repository under `plugins/` and
installs from it as a marketplace. The phases follow the design's
rollout plan one to one, shifted by one because this document numbers
from 1: Phase 1 here is the design's Phase 0, the hook drill, and
Phase 6 is its Phase 5, the release.

**Implements:** [DESIGN-0024](../design/0024-docz-review-a-blocking-review-page-for-agent-written-documents.md)
(Approved 2026-10-25, every question at its recommendation), from
[INV-0027](../investigation/0027-interactive-mode.md).

<!--docz:objective:end-->

<!--docz:scope:start-->
## Scope

<!--docz:in-scope:start-->
### In Scope

- The hook drill (design Testing Strategy, "Hook contract"), its recorded
  fixtures, and the Decisions rows it adds to DESIGN-0024
- `pkg/doczcore/repo`: `ValidateDocument` and `InsertRegionsIn`
- `cmd/`: `docz inspect`, `docz validate <id|path>`, `docz create
  --format json`, `schema_version` and `content_sha256` on `validate`'s
  JSON
- `api/cli/`: the five schemas, the embedding Go package, the contract
  test, and the `ui` CI filter change
- `ui/`: generated types and runtime validation, the position-preserving
  render mode, region wrappers, question cards, annotations and anchors,
  findings, the baseline diff, the server with its state machine, lease,
  credential, and staleness checks, the compiled binaries, and the
  recipes and CI steps for all of it
- `release.yml` and `prerelease.yml`: the Bun build, the second checksum
  file and its signature, the SBOM
- `plugins/docz-review/` here (hooks, wrappers, skill, README, tests, the
  drill write-up) and a root `.claude-plugin/marketplace.json`
- claude-skills: one sentence in the docz plugin's `create` skill
- `CLAUDE.md`, `ui/CLAUDE.md`, `ui/README.md`, and the memory

<!--docz:in-scope:end-->

<!--docz:out-of-scope:start-->
### Out of Scope

- Writing the document from the page, in any form (design non-goal)
- docz-api, docz-site, `api/openapi.yaml`, and `charts/docz`: nothing
  changes
- The word-level diff toggle and task toggles from the page (design
  rollout step 7, "Later")
- A neovim, terminal, or other front end; other agents' hook formats
- Typed per-type fields on `docz inspect` (a later `--typed` flag)
- A rebase workflow for stale feedback
- Resolving INV-0021's anchor question for Confluence: the shape is
  shared, the Confluence side is not touched here

<!--docz:out-of-scope:end-->
<!--docz:scope:end-->

## Implementation Phases

Each phase builds on the previous one. A phase is complete when all its
tasks are checked off and its success criteria are met. One pull request
per phase (open question 1); Phase 2 and Phase 6 are the two that
release.

---

<!--docz:phase:start-->
### Phase 1: The hook drill

Before any code, observe what Claude Code actually does with a `Stop`
hook that waits, and record it. The drill is a throwaway plugin whose hooks are shell scripts, kept
outside the tree (a directory under `/tmp` added as a local marketplace);
what it leaves behind is the write-up, the fixtures, and DESIGN-0024's
Decisions section. Everything the design
marked "observed in Phase 0" is settled here by observation, and each
observation is written down with the Claude Code version it was made on.

<!--docz:tasks:start-->
#### Tasks

- [ ] A throwaway plugin `docz-review-drill` outside the tree with
  a `PostToolUse` hook on `Bash` that appends its full stdin to
  `$CLAUDE_PLUGIN_DATA/posttooluse.jsonl`, and a `Stop` hook script that
  appends its stdin, then sleeps for a configured number of seconds,
  then prints a configured JSON object; both read their configuration
  from a file so a case is a file edit, not a plugin edit
- [ ] Case 1, finite stdin: the `Stop` script records whether stdin
  reached EOF before it slept and whether the process was still running
  when the sleep ended (write a line on each side of the sleep)
- [ ] Case 2, the 600-second default: with no `timeout` on the hook
  entry, sleep 700 seconds; record whether the hook was killed and what
  the session did
- [ ] Case 3, a configured timeout: with `timeout: 1200`, sleep 700
  seconds, then print `block`; record whether the block was honoured;
  then with `timeout: 86400`, sleep 700 seconds; record the same, and
  look for any ceiling in the session's own diagnostics
- [ ] Case 4, block and continue: print `{"decision":"block","reason":"…",
  "systemMessage":"…"}`; record that the agent resumed with the reason
  and that the person saw the system message
- [ ] Case 5, the no-tool repeat: block repeatedly from a session whose
  agent makes no tool call between turns; record how many consecutive
  blocks the host accepted, whether any `stop_hook_active` or similar
  field appeared in the input, and what ended the loop
- [ ] Case 6, interrupt: while the `Stop` hook sleeps, press Escape, then
  in a second run Ctrl-C; record for each whether the process received a
  signal (trap and log `TERM`, `HUP`, `INT`), was killed outright, or
  kept running, and what happened to output printed after the interrupt
- [ ] Case 7, hard termination: kill the hook's process from outside with
  `SIGKILL`; record the session's behaviour
- [ ] Case 8, `tool_response` shapes: run `docz create` successfully, a
  failing `docz create`, a command with very long output, and a compound
  command (`docz create … && echo done`); keep the four recorded JSON
  objects as fixtures under `plugins/docz-review/tests/fixtures/` (open
  question 3)
- [ ] Case 9, the `if` field: add `"if": "Bash(docz *)"` to the
  `PostToolUse` entry; record whether the hook ran for `docz list`,
  for `echo docz`, for `FOO=1 docz list`, and for `ls && docz list`,
  and note the Claude Code version
- [ ] Case 10, the arming echo: run `echo "Created DESIGN: docs/x.md"`;
  confirm the response text carries the line, so the arming parser's
  rule (validate every candidate through `docz inspect`) is the right
  defence rather than response-text trust
- [ ] Write the drill up at `plugins/docz-review/docs/hook-drill.md` in
  this repository: a table of case, observation,
  Claude Code version, and date
- [ ] DESIGN-0024: add a row per observation to the Decisions section
  (timeout ceiling, interrupt behaviour, host continuation cap,
  `stop_hook_active`, `tool_response` shape, `if` support) and amend
  open questions 1, 2, 4, and 12 only where an observation contradicts
  the resolved option; regenerate with `docz update design`
- [ ] Delete the drill plugin; it is never committed or listed in the
  marketplace

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `hook-drill.md` answers all ten cases with observed results, not
  assumptions, each with the Claude Code version
- DESIGN-0024's Decisions section carries the observations and
  `docz validate design --strict` reports nothing for it
- The four `tool_response` fixtures exist and parse as JSON
- The deadline arithmetic in the design (hook timeout minus sixty
  seconds) is either confirmed or replaced by a number the drill
  measured

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 2: The CLI's JSON contract

After this phase `docz inspect` reports one document's structure,
`docz validate` takes an ID or path and fixes one document without
touching its siblings, `docz create --format json` prints one envelope,
and every governed output is held to a committed schema by a Go test.
This is a `minor` on the stable v2 line and ships alone; agents and the
docz plugin's skills can use `inspect` before any page exists.

<!--docz:tasks:start-->
#### Tasks

- [x] `api/cli/schemas.go`: package `cli` with `//go:embed *.schema.json`
  into an `embed.FS`, `Schema(name string) ([]byte, error)`, and the
  constants `SchemaVersionInspect`, `SchemaVersionValidate`,
  `SchemaVersionCreate`, `SchemaVersionReview`, all `1`; a package test
  parses every schema, asserts `additionalProperties: false` on every
  object, asserts each `$id` matches
  `<base>/<name>/v<schema_version>` (open question 9), and asserts each
  file's `schema_version` `const` equals the Go constant
- [x] `api/cli/inspect.schema.json`, `validate.schema.json` (a `oneOf`
  of the plain `ValidateReport` and `validateFixOutput`),
  `create.schema.json`, `list.schema.json` (no version field, the
  documented exception), and `review.schema.json` (the record in design
  section 8, written by docz-review and tested in Phase 3), all draft
  2020-12, each with `$id`, `$version: "1.0"`, and a top-level
  description naming the command
- [x] `pkg/doczcore/repo/validate.go`: `ValidateDocument(ctx, path string)
  (DocumentReport, error)` with `DocumentReport{Entry, Content []byte,
  ContentSHA256 string, Findings []validate.Finding, Schema string}`:
  resolves the path under the root and under an enabled type's
  directory through the real path (`filepath.EvalSymlinks`), reads the
  file once, runs the generic tier against the schema the document
  names (the same resolver `Validate` uses, including the third tier),
  and never scans a sibling; a path outside the root or outside a type
  directory is a typed `PathError{Path, Reason}` added to `errors.go`;
  a missing or malformed frontmatter is findings, not an error
- [x] `pkg/doczcore/repo/regions.go`: `InsertRegionsIn(ctx, paths
  []string) (InsertRegionsReport, error)` over the named files only,
  sharing the per-file body with `InsertRegions`; the report shape is
  unchanged
- [x] `repo` tests: `ValidateDocument` on a document with a broken
  sibling present (the sibling never appears in the report),
  `ValidateDocument` on a file with no frontmatter (reports
  `frontmatter.missing`, returns the content), a symlink escaping the
  root, a path under a disabled type, and `InsertRegionsIn` with a
  digest of every file in the fixture tree before and after proving
  only the named file changed; `t.Parallel()` throughout
- [x] `cmd/inspect.go`: `docz inspect <id|path> [--format text|json]`;
  resolution order type name (refused with exit 2 and a message saying
  `inspect` takes a document), then `repo.Find`, then path; DTOs
  `inspectOutput`, `inspectFrontmatter`, `inspectRegion`,
  `inspectHeading`, `inspectQuestion`, `inspectOption`, `inspectTask`
  with json tags, built from one read through `ValidateDocument`'s
  bytes (no second read), `docparse.Regions` via `kinds.ResolveRegions`
  with the type's spec from the resolved template, `docparse.Headings`
  (anchors as assigned), `kinds.OpenQuestions` shifted to document
  lines, `docparse.TaskItems`, and `TypeConfig.Statuses`; `frontmatter`
  and `resolved` are `null` when absent, arrays are `[]`, `start: 0` is
  allowed; `docz_version` from `cmd.Version`; the text form prints the
  id, type, status, counts, and unresolved questions
- [x] `cmd/validate.go`: accept an ID or path as the argument (type name
  first), call `ValidateDocument`, run `typeValidator` over the returned
  bytes rather than re-reading, print the same `ValidateReport` shape
  with one type and one document, skip index drift and the template
  check for a single document, route `--fix` through `InsertRegionsIn`
  with the one path; `applyTypeTier` takes bytes from the report for
  every path (the `os.ReadFile` goes away); add `schema_version` and,
  per document, `content_sha256` to the JSON output of both the plain
  report and `validateFixOutput`
- [x] `cmd/create.go`: `--format json` makes stdout one envelope
  `{schema_version, id, type, path, title, generic_template, update, nav}`
  where `update` is the type report and `nav` the MkDocs nav result as
  structures; the prose for both is suppressed under JSON; the
  generic-template note stays on stderr; a failed creation is an error
  on stderr with no JSON
- [ ] `cmd/inspect_test.go`: the resolution grammar (type name refused,
  ID, repo-relative path, absolute path inside the root, a path outside
  the root, a path under a disabled type, a symlink escaping the root,
  an unknown ID, a duplicate ID resolved by path), the text form, and
  the JSON form over a document of each built-in type and the custom
  `frameworks` type from the parity fixtures; an unmarked document
  reports `inferred: true` and `start: 0` for a heading on line 1; a
  document with no frontmatter reports `frontmatter: null` and its
  regions
- [ ] `cmd/validate_test.go` gains: findings on one document, a clean
  document, `--fix` on one document with a tree-digest check that no
  sibling byte changed, type-name-first resolution, `content_sha256`
  equal to the file's digest, and before-and-after JSON goldens pinning
  the `schema_version` addition
- [ ] `cmd/create_test.go` gains `--format json`: stdout parses as one
  object carrying `update` and `nav`, the note is on stderr, and
  `--format json` with a failed creation prints no JSON
- [ ] `cmd/cli_contract_test.go`: every governed command run through a
  `Runner` with `Out: &bytes.Buffer{}` over the parity fixtures, each
  output validated against its embedded closed schema with
  `github.com/santhosh-tekuri/jsonschema/v6` (open question 2); the
  committed fixtures under `cmd/testdata/cli-contract/v1/*.json`
  re-validated; a revert drill in the PR description shows a renamed
  field failing the test
- [ ] `.github/workflows/ci.yml`: the `ui` path filter gains `api/cli/**`
  and `ui.just`; the `go` filter already covers `api/**`
- [ ] `test/consumer/consumer_v2_repo_test.go`: exercise `ValidateDocument`
  and `InsertRegionsIn` from outside the module
- [ ] `CLAUDE.md`: the `cmd/` bullet gains `docz inspect` and the
  single-document `validate`; the `repo` bullet gains the two operations;
  a new bullet for `api/cli`; `docs/README.md` or the CLI reference
  gains the commands
- [ ] `just lint`, `just fmt`, `just test`, `just test-consumer`,
  `just parity`, `just lint-generated`, `just validate`

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `just ci` passes; the parity suite needs no new normaliser
- `docz inspect DESIGN-0024 --format json` over this repository validates
  against `inspect.schema.json` and reports twelve open questions, each
  with `resolved` set
- `docz validate docs/design/0024-*.md --format json` and
  `docz inspect` on the same file report the same `content_sha256`
- `docz validate <path> --fix` on a fixture with an unmarked document and
  an unmarked sibling changes only the named file (the tree-digest test)
- `docz create design "x" --format json | jq .path` works in a temp
  repository and nothing else reaches stdout
- Renaming a field in `inspectOutput` fails `TestCLIJSONContract`
- A PR touching only `api/cli/inspect.schema.json` runs the `ui` job

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 3: Generated types and the review page

After this phase the page renders a document with positions, regions,
question cards, annotations with resolvable anchors, findings, and the
baseline diff, against a fixture repository through `just ui dev-review`
and under vitest and Playwright. No server, no hook, no binary yet; the
page talks to a small in-process fake of the server's API. `dont-release`.

<!--docz:tasks:start-->
#### Tasks

- [ ] `ui/package.json`: add `json-schema-to-typescript`, `ajv`, and
  `diff` (with `@types/diff`); `ui/scripts/gen-cli.ts` reads
  `../api/cli/*.schema.json`, writes `src/cli/__generated__/<name>.ts`
  and, beside each, the derived **open** variant of the schema
  (`additionalProperties` removed recursively) as
  `src/cli/__generated__/<name>.open.json`; `.gitignore` covers the
  directory; `ui.just` gains `gen-cli` and `gen-cli-check` (snapshot,
  regenerate, diff, like `gen-api-check.sh`)
- [ ] `ui/src/cli/validate.ts`: ajv (2020 dialect) compiled once per open
  schema; `parseInspect`, `parseValidate`, `parseCreate` take a string,
  enforce a size limit (16 MiB), parse, validate, and return the typed
  value or throw a `ContractError{schema, errors}`; `checkSchemaVersion`
  compares `schema_version` and `docz_version` against the supported
  pair and throws naming both versions
- [ ] `ui/src/cli/validate.test.ts`: each committed fixture accepted; a
  fixture with an extra unknown field accepted; a missing required field
  refused; an oversized input refused; a wrong `schema_version` refused
  with both versions in the message
- [ ] `ui/src/markdown/preprocess.ts`: a `preserveLines` option that
  blanks the frontmatter and ToC lines instead of cutting them, with a
  test that every block's `position.start.line` equals its document line
  over the fixture documents
- [ ] `ui/src/markdown/rehype-data-line.ts`: a rehype plugin that runs
  **after** sanitize and stamps `data-line` on block elements from
  `position`; `schema.ts` unchanged, so an author-supplied `data-line` is
  stripped first; `processor.ts` gains a `review` mode wiring preprocess
  and the plugin; tests for the stamp, the forged attribute, nested
  lists, raw HTML, and a block inside an alert, a table wrapper, and a
  Mermaid block
- [ ] `ui/src/review/regions.tsx`: wraps rendered blocks into
  `<section data-region data-ordinal>` from `inspect`'s `regions[]`,
  depth-first, deepest region wins; a block crossing a boundary and a
  malformed region leave the content unwrapped; tests
- [ ] `ui/src/review/anchor.ts`: `anchorFromRange(range, blocks)` builds
  the anchor from a DOM `Range` (region, ordinal, heading, `exact`,
  `prefix`, `suffix` within the block in code points, `line`,
  `selection`), refusing nothing but flagging `multiBlock`;
  `resolveAnchor(anchor, source)` parses the source with the same remark
  pipeline, takes each block's `mdast-util-to-string` text with its
  position, and returns `{resolution: "exact"|"ambiguous"|"lost", line,
  ordinal}` with the rules in design section 7 (one hit; a strict-best
  context and heading score; the line hint never breaks a tie)
- [ ] `ui/src/review/anchor.test.ts`: the `carryMarkers` cases ported
  (one hit, several with context, a tie, none), plus `use **Postgres**`,
  a link, a soft-wrapped paragraph, an entity, a code span, Unicode text
  with combining marks and an emoji, a moved region found by heading,
  and a multi-block selection
- [ ] `ui/src/review/questions.tsx`: the cards, unanswered by default,
  the recommended option badged, "accept all recommendations", "Other"
  text, note, read-only when `resolved`; tests
- [ ] `ui/src/review/annotations.tsx`: the selection popover (comment,
  replace, delete, looks good; delete and replace disabled on a
  multi-block selection), the document comment box, the sidebar list
  in document order with edit and remove, painting through
  `CSS.highlights`; tests
- [ ] `ui/src/review/findings.tsx`, `status.tsx`: findings with dismiss
  and fix, the status select; tests
- [ ] `ui/src/review/diff.tsx`: whole-source line diff with `diff`,
  deleted regions and frontmatter changes listed, per-region "changed"
  badges matched by kind and heading, collapse unchanged, annotations
  disabled on diff text, a size bound that falls back to badges; tests
- [ ] `ui/src/review/draft.ts`: the draft model, the actionable set
  (question answer, comment, replace, delete, document comment, status
  change, finding fix), `outcomeOf(draft)`, and the rule that Approve is
  enabled only with no actionable item; tests
- [ ] `ui/src/review/feedback.ts`: the text message from a record
  (design section 8), golden-tested, with "no action" lines and the
  closing instruction naming only the answered questions
- [ ] `ui/src/review/api.ts`: the typed client for the server's endpoints
  with the bearer header from memory and `history.replaceState` on
  load; `ui/src/review/fake-server.ts` for tests and `dev-review`
- [ ] `ui/review.html` and `ui/src/review/main.tsx`: the entry, the
  document switcher, per-document Submit, Approve, Dismiss, Done, the
  stale banner, and the before-unload warning while a draft exists;
  `vite.config.ts` gains the second input and `dist-review` output
- [ ] `ui/e2e/fixtures/review-repo/` (open question 4): a `.docz.yaml`
  and documents covering open questions, tasks, duplicate headings, an
  unmarked document, and a document with no frontmatter;
  `ui.just` gains `dev-review` (Vite against the fake server over the
  fixture) and `test-review` (vitest over `src/review` and `src/cli`)
- [ ] `ui/e2e/review.spec.ts` against the fake server: open, answer a
  question, accept all, annotate formatted, multiline, Unicode,
  repeated, and cross-block text, submit, and assert the message; a
  forged `data-line` in the fixture never becomes a coordinate
- [ ] `ui/CLAUDE.md`: a docz-review section (the entry point, the
  review render mode, the generated types, the recipes)

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `just ui gen-cli`, `gen-cli-check`, `lint`, `typecheck`, `test`,
  `test-review`, and `build` pass; `e2e-review` passes under chromium
- Every block in the review render carries a `data-line` equal to its
  source line for every fixture document, and a forged attribute is
  stripped
- The anchor resolver reports `exact` for the single-hit cases,
  `ambiguous` for the tie, and `lost` for the removed quote, with no
  case resolved by the line hint alone
- Submitting one annotation with every question untouched sends no
  question decisions; Approve is disabled while a replace is drafted
- A fixture with an unknown field parses; one with a wrong
  `schema_version` is refused naming both versions

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 4: The server, the state machine, and the binary

After this phase `docz-review <id> --json` runs end to end against a
real `docz` on this machine, `docz-review --pending --hook` implements
the state machine, and `just ui build-review` produces the four
compiled binaries from one checkout. `dont-release`, with a beta tag
cut by hand at the end to prove the release path.

<!--docz:tasks:start-->
#### Tasks

- [ ] `ui/review/state.ts`: the state directory (`DOCZ_REVIEW_STATE_DIR`,
  `$XDG_STATE_HOME`, `$HOME/.local/state`), 0700 directories and 0600
  files, `O_NOFOLLOW` opens, `sha256(root)/<session>/` layout, `lock`
  through `flock` around every read-modify-write, `state.json` replaced
  atomically, the per-document states `pending`, `open`, `changes`,
  `approved`, `dismissed` with the transitions in the design's Data
  Model, round counters kept across re-arming, expiry after 30 days, and
  `purge`; tests with two concurrent writers and a crash between record
  and state reconciled by submission id
- [ ] `ui/review/lease.ts`: `lease.json` with pid and expiry, refuse while
  live, take over when expired, renewed by the event stream; tests
- [ ] `ui/review/docz.ts`: spawn `docz` (`DOCZ_REVIEW_DOCZ`, else `PATH`)
  with `execFile` and an argument list, `--repo-root` and `--config` from
  the entry, `version` check once, `inspect` and `validate` parsed
  through `src/cli/validate.ts`, digests compared with one retry, a
  non-zero exit with no JSON or a spawn failure surfaced as an error
  distinct from a findings report; tests with a fake `docz` script
- [ ] `ui/review/arm.ts`: `arm --from-hook` reads stdin to EOF, recognises
  `docz create` (JSON envelope, else `Created <TYPE>: <path>` lines) and
  `docz-review arm <args>` in `tool_input.command` by word presence only,
  validates every candidate through `docz inspect --repo-root <cwd>`,
  records `pending` entries, and reports a dropped candidate through a
  `systemMessage`; `arm <id|path>` by hand only validates and prints;
  tests over the Phase 1 fixtures (success, failure, long output,
  compound command, and the echo case)
- [ ] `ui/review/server.ts`: `Bun.serve` on `127.0.0.1` with a random or
  configured port; the bootstrap `GET /?t=` serving the page; bearer
  check in constant time; `Host` other than `127.0.0.1:<port>` is 421;
  exact `Origin`, `Sec-Fetch-Site`, and `Content-Type` on mutations;
  the CSP, `Referrer-Policy`, and `Cache-Control` headers; no CORS; the
  endpoints `GET /api/review`, `POST /api/review/{doc}/submit`,
  `/approve`, `/dismiss`, `POST /api/review/done`,
  `PUT /api/review/{doc}/draft`, `GET /api/events`; embedded assets
  served by MIME type; stdin read to EOF under `--hook` and closed
- [ ] `ui/review/round.ts`: round ids, snapshots, the previous round for
  the diff, staleness on every submission (identity and digest; 409 with
  the draft kept), semantic checks against the served round (option
  letters, status, finding identities; 400), submission ids (a replay
  acknowledged once; a closed round refused), record written and
  fsynced before the response
- [ ] `ui/review/main.ts`: the CLI (`<id|path>…`, `--pending --hook`,
  `--resume`, `arm`, `pending [--purge]`, `--version`, `--hook`,
  `--json`, `--repo-root`, `--config`, `--no-open`, `--timeout`,
  `--grace`, `--max-rounds`, `--port`); the deadline defaulting to the
  hook timeout minus sixty seconds (or the Phase 1 number); `SIGTERM`,
  `SIGHUP`, `SIGINT` as dismiss; the browser opened through `execFile`
  (`DOCZ_REVIEW_BROWSER`, `BROWSER`, `open`, `xdg-open`); `--no-open`
  printing the URL to stderr under `--hook`; the hook output printed
  once and last; `--json` printing the records; the text message
  otherwise; `--version` printing the binary, docz, and schema versions
- [ ] `ui/review/*.test.ts` (`bun test ui/review/`): the server tests the
  design's Testing Strategy lists, including the three-document batch
  (approve A, changes on B, C untouched), the stale cases (edited,
  replaced, deleted while open), the replayed submission, two arming
  calls racing, the crash reconciliation, each signal and the deadline
  as dismiss, the lease refusing a second server, and the hook output
  printed once and last
- [ ] `ui/src/cli/review.test.ts`: the review record validated against
  `review.schema.json` over committed fixtures (the record's contract
  test)
- [ ] `ui/scripts/embed-manifest.ts`: generates
  `ui/review/__generated__/assets.ts` from `dist-review/.vite/manifest.json`
  with one file import per emitted asset and its MIME type;
  `ui.just` gains `build-review` (Vite build of `review.html`, the
  manifest module, then `bun build --compile` for the four targets into
  `ui/dist-review-bin/`, outside the inputs); a test runs a compiled
  binary from an empty temp directory with `dist-review` absent and
  fetches the page and one lazy chunk
- [ ] `ui/README.md`: how to run `docz-review` from a checkout and the
  mise install block from design section 10
- [ ] `.github/workflows/ci.yml`: the `ui` job gains `test-review`,
  `build-review` with the empty-directory run; `e2e-review` joins the
  existing `ui-e2e` job as steps (install Go, build `docz`, run the
  review spec over the fixture repository through the real server),
  sharing its browser install rather than adding a job
- [ ] `release.yml` and `prerelease.yml`: a Bun setup step, `just ui
  build-review`, `release.extra_files` for the four binaries and
  `docz-review_checksums.txt`, a step signing that checksum file with
  the imported GPG key and uploading the signature with
  `gh release upload`, syft over `ui/` for the SBOM with the Bun version
  recorded, and a release-notes line stating the Bun runtime gap
- [ ] A hand-cut `v*-beta.1` from the merge commit, as the v2 betas
  were; download the four binaries and both checksum files, verify both
  signatures, and run one binary from `/tmp`

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `bun test ui/review/` and `just ui build-review` pass; the compiled
  binary serves the page and a lazy chunk from an empty directory
- `docz-review DESIGN-0024 --json --no-open` on this repository starts,
  prints the URL to stderr, and after a submission from a browser prints
  one record that validates against `review.schema.json`
- In the three-document batch test only B contributes feedback, A ends
  `approved`, C stays `pending`, and a second server started during the
  run is refused by the lease
- Editing the file while the page is open makes Approve return 409 and
  the draft survives
- A `localhost` `Host`, a wrong bearer, a cross-origin `Origin`, and a
  replayed submission id are each refused with the documented status
- The beta release carries the four binaries, two checksum files, two
  signatures, and the SBOM, and both signatures verify

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 5: The plugin

After this phase a session with the `docz-review` plugin installed
reviews the documents it creates, and one without it is unchanged. The
plugin lives in this repository under `plugins/docz-review/`, and the
repository is itself a Claude Code marketplace through a root
`.claude-plugin/marketplace.json`, so a user installs it on day one with
two `/plugin` commands and a team with a private marketplace copies or
references the directory (the README says how). `dont-release`: a plugin
installs from a git ref, not a release asset.

<!--docz:tasks:start-->
#### Tasks

- [ ] `plugins/docz-review/.claude-plugin/plugin.json` (name, version
  `0.1.0`, description, author) and `hooks/hooks.json` (the `PostToolUse` entry on `Bash` with the `if`
  pre-filters `Bash(docz *)` and `Bash(docz-review *)` if Phase 1
  confirmed the field, `timeout: 30`; the `Stop` entry with the timeout
  Phase 1 settled, 86400 unless contradicted)
- [ ] `hooks/arm.sh` and `hooks/stop.sh`: `exec docz-review …` when the
  binary is on `PATH`, else one stderr line naming the install page and
  `exit 0`; stdin passed through; no interpolation of paths or commands;
  `shellcheck` clean
- [ ] `skills/review/SKILL.md`: `/docz-review:review [<id|path>…]`, which
  tells the agent to run `docz-review arm <args>` as a Bash command and
  finish its turn, or `docz-review pending` with no arguments and report
  the list
- [ ] `.claude-plugin/marketplace.json` at the repository root listing
  `docz-review` with `source: ./plugins/docz-review`, checked with
  `claude plugin validate` (or a JSON parse of both manifests if the CLI
  has no such command); the marketplace name is `docz`
- [ ] `plugins/docz-review/README.md`: install (`/plugin marketplace add
  donaldgifford/docz`, `/plugin install docz-review@docz`, and the mise
  block for the binary), what arms a review, the states, walking away,
  `--max-rounds`, the plugin's minimum Claude Code version from Phase 1,
  and a section "From your own marketplace" for a team with a private
  one: reference this repository as a second marketplace, or copy the
  directory into theirs and track `plugin.json`'s version to resync
- [ ] `.github/workflows/ci.yml`: a `plugin` job path-filtered on
  `plugins/**` and `.claude-plugin/**` running `shellcheck` over the
  wrappers, the hook tests, and the manifest check
- [ ] Follow the README's "From your own marketplace" section verbatim
  against a private marketplace of our own, both the reference path and
  the copy path: the plugin installs and the hooks fire; fix the
  instructions before the PR merges
- [ ] `plugins/docz-review/tests/`: the hook tests over the Phase 1
  fixtures through the wrappers (`bun test plugins/docz-review/tests/`),
  including the missing-binary path
- [ ] claude-skills, the docz plugin's `create` skill: one sentence
  telling the agent to pass `--format json` to `docz create` (open
  question 8); the only change there
- [ ] A trial on this repository: create a document in a session with
  the plugin, revise through two rounds, approve; record the transcript's
  shape (not its content) in the plugin README's "what to expect"

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- With the plugin installed and the binary absent, `docz create` and the
  end of a turn each print one install line and the session is never
  blocked
- With both installed, `docz create design "x"` followed by the end of
  the turn opens the page; a submission with one change resumes the
  agent with the message; a second `Stop` opens round two with the
  diff; Approve ends the loop
- A session that never ran `docz create` sees nothing
- On a machine with no marketplace configured,
  `/plugin marketplace add donaldgifford/docz` then
  `/plugin install docz-review@docz` installs the plugin from `main`
- The plugin's tests pass in this repository's `plugin` CI job
- Both private-marketplace paths in the README were followed verbatim on
  a private marketplace and worked

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 6: Release and close-out

The first tag that carries the binaries, with the documentation and
memory brought up to date. `minor`.

<!--docz:tasks:start-->
#### Tasks

- [ ] `charts/docz/Chart.yaml`: bump `version` and `appVersion` to the
  release, the two deployment-test image tags, and `just chart docs`
  (the chart content is unchanged; the release rule requires the bump)
- [ ] `CLAUDE.md`: a "docz-review" section under Frontend (`ui/`) with
  the second entry point, the state directory, the credential model,
  the recipes, the plugin under `plugins/`, and the release step; the CI
  path-filter paragraph gains `api/cli/**` and the `plugin` job
- [ ] `ui/CLAUDE.md` and `ui/README.md` final pass; `api/README.md`
  gains a section on `api/cli/`
- [ ] DESIGN-0024 status to Implemented; INV-0027's Recommendation gains
  a line pointing here; `docz update`
- [ ] Memory: `project_docz_review_design.md` updated with the release,
  the plugin version, and anything the trial taught
- [ ] The release PR labelled `minor`; after the tag: verify the GitHub
  release lists the Go archives, the four binaries, both checksum
  files and signatures, and the SBOM; install `docz` and `docz-review`
  side by side with mise on macOS and Linux and run one review from
  outside the checkout
- [ ] Open follow-up issues: the word-level diff toggle, task toggles
  from the page, INV-0021's Confluence side of the anchor shape, and
  anything Phase 5's trial surfaced

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- The release exists with every artifact listed and both signatures
  verifying
- `mise install` of the two tools from the release works on both
  platforms and `docz-review --version` names the release and
  `schema_version` 1
- `docz validate --strict` over this repository reports nothing new;
  `just ci` passes on `main`
- Follow-up issues exist for everything deferred

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:file-changes:start-->
## File Changes

| File | Action | Description |
| ---- | ------ | ----------- |
| `api/cli/schemas.go`, `api/cli/*.schema.json`, `api/cli/schemas_test.go` | Create | The embedded schemas, the version constants, and their package test |
| `pkg/doczcore/repo/validate.go`, `regions.go`, `errors.go` | Modify | `ValidateDocument`, `InsertRegionsIn`, `PathError` |
| `cmd/inspect.go`, `cmd/inspect_test.go` | Create | `docz inspect` and its DTOs |
| `cmd/validate.go`, `cmd/create.go` and their tests | Modify | Single-document validation and `--fix`; the JSON create envelope |
| `cmd/cli_contract_test.go`, `cmd/testdata/cli-contract/` | Create | The Go contract test and its committed fixtures |
| `test/consumer/consumer_v2_repo_test.go` | Modify | The two new operations from outside |
| `.github/workflows/ci.yml` | Modify | `ui` filter gains `api/cli/**` and `ui.just`; the `ui` job gains the review steps; a `plugin` job |
| `.github/workflows/release.yml`, `prerelease.yml` | Modify | Bun build, extra files, second signature, SBOM |
| `ui/package.json`, `bun.lock` | Modify | `json-schema-to-typescript`, `ajv`, `diff` |
| `ui/scripts/gen-cli.ts`, `ui/scripts/embed-manifest.ts` | Create | Type generation and the asset import module |
| `ui/src/cli/` | Create | Generated types, open schemas, runtime validation |
| `ui/src/markdown/preprocess.ts`, `processor.ts`, `rehype-data-line.ts` | Modify / Create | The position-preserving render mode |
| `ui/src/review/` | Create | Regions, anchors, questions, annotations, findings, status, diff, draft, feedback, api, main |
| `ui/review.html`, `ui/vite.config.ts` | Create / Modify | The second entry point |
| `ui/review/` | Create | The Bun server: state, lease, docz, arm, server, round, main, tests |
| `ui/e2e/fixtures/review-repo/`, `ui/e2e/review.spec.ts` | Create | The fixture repository and the Playwright flow |
| `ui.just` | Modify | `gen-cli`, `gen-cli-check`, `dev-review`, `test-review`, `build-review`, `e2e-review` |
| `ui/CLAUDE.md`, `ui/README.md`, `CLAUDE.md`, `api/README.md` | Modify | Documentation |
| `charts/docz/Chart.yaml` and tests | Modify | The release bump |
| `plugins/docz-review/`, `.claude-plugin/marketplace.json` | Create | The plugin (hooks, wrappers, skill, README, tests, drill write-up) and the marketplace that publishes it |
| claude-skills `plugins/docz/skills/create/SKILL.md` | Modify | The `--format json` sentence |

<!--docz:file-changes:end-->

<!--docz:testing:start-->
## Testing Plan

- [ ] Go: `cmd/inspect_test.go`, the `validate` and `create` additions,
  `cmd/cli_contract_test.go` with committed fixtures and the revert
  drill, `repo`'s scoped-operation tests with the tree digest, the
  `api/cli` package test, and the consumer-module test
- [ ] TypeScript unit (vitest): runtime validation, preprocess positions,
  the post-sanitize stamp, region wrapping, anchor computation and
  resolution with the ported `carryMarkers` cases, question cards,
  annotations, the draft's actionable set, the diff, and the feedback
  goldens
- [ ] Bun server tests: every case in the design's Testing Strategy,
  including the batch, staleness, replay, races, crash recovery,
  signals, the deadline, the lease, and the headers
- [ ] Playwright: `e2e-review` against the fake server in Phase 3 and the
  real server with a built `docz` from Phase 4
- [ ] The compiled binary from an empty directory
- [ ] The plugin's hook tests over the drill fixtures through the
  wrappers, in the `plugin` CI job
- [ ] The Phase 1 drill, recorded; the Phase 5 trial on this repository
- [ ] Release verification: artifacts, both signatures, side-by-side mise
  install on macOS and Linux, a review from outside the checkout
- [ ] A security pass over the server before Phase 4 merges

<!--docz:testing:end-->

<!--docz:dependencies:start-->
## Dependencies

- DESIGN-0024 Approved (done 2026-10-25); PR #199 merged so the IMPL and
  the design are on `main` before Phase 1's Decisions edit
- Claude Code at a version with the `if` hook field and documented
  `Stop` semantics; Phase 1 names it
- Go: `github.com/santhosh-tekuri/jsonschema/v6` promoted from indirect
  to a test-path direct require (never a bare `go mod tidy`)
- Bun 1.4.4 (pinned), with `bun build --compile --target` for the four
  targets from one host
- npm: `json-schema-to-typescript`, `ajv`, `diff`, `@types/diff`
- claude-skills: the docz plugin's `create` skill only; the review plugin
  needs nothing there
- The GPG key already imported in `release.yml` for the second signature
- Phase 5's trial needs the Phase 4 beta installed through mise

<!--docz:dependencies:end-->

<!--docz:open-questions:start-->
## Open Questions

### 1. How is the work split into pull requests?

> **Resolved 2026-10-25: (a).**

- a. **One PR per phase, each from `main` after the previous merges:
  Phase 1 touches only DESIGN-0024 here (`dont-release`), Phase 2 is
  the CLI (`minor`, released alone), Phases 3 to 5 are `dont-release`,
  Phase 6 is the release (`minor`).** *(recommendation)* The CLI is
  useful by itself and lands on the stable line early; the page and the
  server are large diffs best read alone; nothing is stacked.
- b. Three PRs: the CLI (Phases 1 and 2), the page and server (Phases 3
  and 4), and the release (Phase 6), with the plugin in claude-skills
  between them.
- c. One PR for everything in this repository.
- d. Other.

### 2. Which JSON Schema validator does the Go contract test use?

Both candidates are already in the module graph and neither can fall
out of it: `santhosh-tekuri/jsonschema/v6` arrives through kin-openapi
(a direct test-path dependency) and `google/jsonschema-go` through the
MCP SDK (a direct production dependency, and the library that infers
the MCP server's tool schemas). Either is a promotion to a direct
require, not a new module.

> **Resolved 2026-10-25: (a).** Equal effort either way; the stable
> major and the asserted formats decide it.

- a. **`github.com/santhosh-tekuri/jsonschema/v6`.** *(recommendation)*
  It asserts `format` when asked (`AssertFormat`), so the schemas'
  `date` and `uri` formats are checked rather than recorded, and a
  failure's `DetailedOutput` names the instance path and the keyword,
  which is what a contract test's failure message needs.
- b. `github.com/google/jsonschema-go`, which would make it the one JSON
  Schema library in the repository, since the MCP server's schemas
  already come from it. As of v0.4.3 its documentation says `format` is
  recorded as an annotation and never asserted, so `date` and `uri`
  would be checked by nothing on the Go side and the test would carry
  its own checks for those two. It is also pre-1.0 (v0.4.3), and its
  documentation says minor and patch releases may change behaviour,
  with a `JSONSCHEMAGODEBUG` variable to restore the old one; the
  santhosh library is at a stable major.
- c. kin-openapi's schema validation, already a direct test dependency,
  which speaks OpenAPI's dialect rather than draft 2020-12 and would
  need the schemas written to it.
- d. Other.

### 3. Where do the hook drill's write-up and fixtures live?

> **Resolved 2026-10-25: other.** The plugin itself, not only the
> drill's artefacts, lives in this repository: `plugins/docz-review/`
> with the write-up under `docs/` and the fixtures under
> `tests/fixtures/`, and a root `.claude-plugin/marketplace.json` makes
> the repository a marketplace. A user gets the plugin on day one from
> here, and a team with a private marketplace copies or references the
> directory, with the README saying how. claude-skills keeps only the
> docz plugin's `create` sentence (question 8). Phases 1 and 5, the
> File Changes, and the Dependencies were rewritten to match, and
> DESIGN-0024 was amended in the places that said claude-skills. The
> README names no marketplace but this repository; the
> private-marketplace instructions are tested against one of our own
> before Phase 5 merges.

- a. **In claude-skills under `plugins/docz-review/docs/hook-drill.md`
  and `plugins/docz-review/tests/fixtures/`, with the conclusions copied
  into DESIGN-0024's Decisions section.** *(recommendation)* The plugin
  is what depends on the observations and its tests consume the
  fixtures; the design records the decisions, not the raw captures.
- b. In this repository as an investigation (`docz create investigation`)
  with the fixtures under `test/`, which puts captured session JSON in
  the docz tree for no consumer here.
- c. Other.

### 4. What repository does `e2e-review` run against?

> **Resolved 2026-10-25: (a).**

- a. **A committed fixture repository under `ui/e2e/fixtures/review-repo/`
  with a `.docz.yaml` and one document per shape the page needs (open
  questions, tasks, duplicate headings, unmarked, no frontmatter).**
  *(recommendation)* Deterministic, readable in review, and free to
  change; the parity fixtures are frozen goldens and must not be edited.
- b. The parity fixtures read-only, extended by copying a document into a
  temp directory at test time.
- c. A repository generated at test time with `docz init` and
  `docz create` from the built binary, which proves the templates
  render but has no content to annotate.
- d. Other.

### 5. Which runtime validator does docz-review use?

> **Resolved 2026-10-25: (a).**

- a. **ajv with the 2020-12 dialect, compiled once per open schema at
  startup.** *(recommendation)* The reference implementation for the
  draft, already common in Bun projects, and the compiled validators
  are what the size-limited parse functions call.
- b. `@cfworker/json-schema`, smaller and without code generation, with
  less complete draft coverage.
- c. None beyond the generated types, which the review R8 showed is no
  validation at all.
- d. Other.

### 6. How much Playwright does the review flow get?

> **Resolved 2026-10-25: (a).** The spec joins the existing `ui-e2e`
> job rather than adding one and reuses its browser install. On CI
> growth generally: jobs already run in parallel and each is
> path-filtered by the `changes` job, so what grows is one job's wall
> clock on `ui/` changes only; a sharper filter (`ui/review/**` and
> `ui/src/review/**` for the review steps) is a follow-up if that job
> gets slow.

What a real browser proves that vitest under jsdom cannot: a
`Selection` and `Range` over laid-out text (the anchors),
`CSS.highlights` painting, a CSP that actually blocks, the bearer
bootstrap through `history.replaceState`, the event stream, the
before-unload warning, and the compiled binary serving a lazy chunk.
Those are the places the adversarial review found the risks. The cost
today: the `ui-e2e` job is path-filtered to `ui/**`, was skipped on
four of the last eight CI runs, and takes 49 to 60 seconds when it
runs (one 631-second outlier was a cold browser-install cache). A
review spec adds under a minute to that job, on `ui/` changes only.

- a. **One spec, chromium only, in the existing path-filtered `ui-e2e`
  job; everything else under vitest, including a check that the page
  degrades to no painting when `CSS.highlights` is absent.**
  *(recommendation)* It runs only when `ui/` changes and covers exactly
  the behaviours no unit test can.
- b. The same spec, run only on `v*` tags and `workflow_dispatch`, with
  vitest carrying every PR. A minute cheaper per `ui/` PR; a regression
  in selection or highlighting surfaces at release time instead of in
  the PR that caused it.
- c. No Playwright for the review flow: vitest plus happy-dom, accepting
  that the anchors and highlights are tested only against a simulated
  DOM.
- d. Other.

### 7. Where are the four compiled binaries built?

> **Resolved 2026-10-25: (a).**

- a. **In the existing `goreleaser` job on its Ubuntu runner, with
  `bun build --compile --target` for each of the four targets from one
  checkout.** *(recommendation)* Bun cross-compiles by downloading the
  target runtime; one job keeps the artifacts beside goreleaser's
  `extra_files` step and the GPG key already imported there.
- b. A matrix job per OS and architecture, uploading artifacts for the
  release job to collect; native builds, four runners, and a hand-off.
- c. Other.

### 8. When does the docz plugin's `create` skill start saying `--format json`?

> **Resolved 2026-10-25: (a).**

- a. **In a small claude-skills PR right after Phase 2 releases, before
  the review plugin exists.** *(recommendation)* The envelope is useful
  to any agent at once (the path without parsing prose), and the arming
  hook later finds it already there.
- b. In the Phase 5 claude-skills PR with the new plugin, one change
  instead of two.
- c. Other.

### 9. What is the schemas' `$id` base?

The design wrote `https://docz.dev/schemas/cli/…` as an illustration;
the domain is not this project's.

> **Resolved 2026-10-25: (a).**

- a. **`https://github.com/donaldgifford/docz/schemas/cli/<name>/v<n>`,
  a URI under a name this project controls, not expected to resolve.**
  *(recommendation)* `$id` is an identifier, and this one is stable,
  unique, and honest about where the schema comes from.
- b. `https://docz.dev/schemas/cli/<name>/v<n>` as written, registering
  the domain or accepting that it is someone else's.
- c. A URN, `urn:docz:cli:<name>:v<n>`.
- d. Other.

<!--docz:open-questions:end-->

<!--docz:references:start-->
## References

- [DESIGN-0024](../design/0024-docz-review-a-blocking-review-page-for-agent-written-documents.md):
  the design this implements, with the adversarial review and its
  responses
- [INV-0027](../investigation/0027-interactive-mode.md): the
  investigation
- [INV-0021](../investigation/0021-confluence-comments-as-a-view-layer-kept-across-source-changes.md):
  the shared anchor shape
- [DESIGN-0015](../design/0015-structured-regions-and-docz-validate.md):
  regions, open questions, and validation
- [IMPL-0026](0026-a-read-only-mcp-server-in-docz-api-tokens-scoped-access-and-the.md):
  the previous phased build with one PR per phase
- [Claude Code hooks reference](https://code.claude.com/docs/en/hooks)
- [Claude Code plugin components](https://code.claude.com/docs/en/plugins/components)
- [Bun single-file executables](https://bun.sh/docs/bundler/executables)
- [mise `github:` backend](https://mise.jdx.dev/dev-tools/backends/github.html)
- [santhosh-tekuri/jsonschema](https://github.com/santhosh-tekuri/jsonschema)
- [json-schema-to-typescript](https://github.com/bcherny/json-schema-to-typescript)
- [ajv](https://ajv.js.org/)

<!--docz:references:end-->
