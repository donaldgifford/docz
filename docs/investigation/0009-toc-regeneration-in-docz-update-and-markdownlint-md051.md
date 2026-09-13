---
id: INV-0009
title: "ToC regeneration in docz update and markdownlint MD051"
status: Concluded
author: Donald Gifford
created: 2026-09-13
---

<!-- markdownlint-disable-file MD025 MD041 -->

# INV-0009: ToC regeneration in docz update and markdownlint MD051

<!--toc:start-->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
  - [Observation 1: docz update regenerates ToC fences — the excerpt is wrong](#observation-1-docz-update-regenerates-toc-fences--the-excerpt-is-wrong)
  - [Observation 2: the pass is silent and undocumented where an agent looks](#observation-2-the-pass-is-silent-and-undocumented-where-an-agent-looks)
  - [Observation 3: cases where a fence really is left stale, with no signal](#observation-3-cases-where-a-fence-really-is-left-stale-with-no-signal)
  - [Observation 4: MD051 fires right after a successful docz update](#observation-4-md051-fires-right-after-a-successful-docz-update)
  - [Observation 5: the marker format is Marksman's, and README names the wrong plugin](#observation-5-the-marker-format-is-marksmans-and-readme-names-the-wrong-plugin)
  - [Observation 6: nothing gates drift, so MD051 in a downstream CI is the first alarm](#observation-6-nothing-gates-drift-so-md051-in-a-downstream-ci-is-the-first-alarm)
  - [Observation 7 (side finding): doubled index markers in two generated READMEs](#observation-7-side-finding-doubled-index-markers-in-two-generated-readmes)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
  - [Decisions](#decisions)
  - [1. Should docz update report ToC changes in normal (non-dry-run) output?](#1-should-docz-update-report-toc-changes-in-normal-non-dry-run-output)
  - [2. Should docz recognise marker spelling variants such as the spaced mtoc form?](#2-should-docz-recognise-marker-spelling-variants-such-as-the-spaced-mtoc-form)
  - [3. How far should the slug-fidelity fix go?](#3-how-far-should-the-slug-fidelity-fix-go)
  - [4. Should docz update gain a CI-gating mode?](#4-should-docz-update-gain-a-ci-gating-mode)
  - [5. Should this repo run markdownlint in CI now?](#5-should-this-repo-run-markdownlint-in-ci-now)
  - [6. Where do the documentation fixes land?](#6-where-do-the-documentation-fixes-land)
  - [7. What to do with the doubled index-marker pair (Observation 7)?](#7-what-to-do-with-the-doubled-index-marker-pair-observation-7)
- [References](#references)
<!--toc:end-->

## Question

Does `docz update` (v1.2.2) regenerate the `<!--toc:start-->` /
`<!--toc:end-->` fence in existing documents? If it does, why did another
Claude session conclude that it does not and recommend deleting the fence or
overriding the templates to drop it? And if there are cases where a fence is
left stale, are those cases documented anywhere — in the repo, in
`docz --help`, or in the `docz` claude-skills plugin — or is that a gap?

The practical goal behind the question: keep markdownlint rule
`MD051/link-fragments` enabled on documentation directories and have
docz-managed documents pass it without hand-editing generated ToCs.

## Hypothesis

`docz update` does regenerate ToC fences — DESIGN-0003 and IMPL-0003 shipped
exactly that, and `CLAUDE.md` describes the pass. The other session's
conclusion most likely came from the surfaces an agent reads first:
`docz update --help` and the `docz:update` skill both describe only README
index tables, and a normal run prints nothing about ToCs. Any MD051 failures
it saw on docz-managed files were more likely caused by anchor-slug
divergences from GitHub (issue #82 is one) or by files the update pass
skips silently, not by a missing feature.

## Context

An excerpt from another session (working in a different repo with docz
v1.2.2) stated:

> `docz update` only regenerates the README index tables ... It does not
> touch the `<!--toc:start-->` / `<!--toc:end-->` block inside each
> document. ... version 1.2.2 does not do that.

and offered three ways out: delete the fence from the affected doc, override
the templates to drop the fence from new docs, or "make `docz update`
regenerate ToCs". Two of those would remove ToC support entirely, and the
third would be re-implementing a feature that shipped in IMPL-0003. Before
filing an issue it was worth establishing what the CLI actually does.

Issue #82 (open) is related but narrower: `docz update` strips `_` from ToC
anchors, GitHub keeps them, so any heading containing an underscore produces
an MD051 failure that `docz update` re-creates on every run.

**Triggered by:** the excerpt above; issue #82; DESIGN-0003 (Table of
Contents Generation).

## Approach

1. Read the code path end to end: `cmd/update.go` (`updateType`,
   `runToCUpdate`) → `pkg/doczcore/toc` (`UpdateFiles`, `UpdateToC`,
   `parseHeadings`, `GenerateToC`) → `pkg/doczcore/docparse` (`Headings`,
   `AnchorSlug`) → `pkg/doczcore/document.ScanDocuments`.
2. Read every place the behaviour is (or should be) documented: DESIGN-0003,
   IMPL-0003, `README.md` "Table of Contents", `docz update --help`,
   `internal/template/templates/docz_yaml.tmpl`, `CLAUDE.md`, and the
   claude-skills `docz` plugin v1.3.0 (`skills/update/SKILL.md`,
   `skills/docz/SKILL.md`, `skills/docz/references/*.md`).
3. Reproduce in a throwaway repo (`git init` + `docz init`, default config)
   using a build of `main` at `9c2f11f` (one docs-only commit past the
   v1.2.2 tag) and this repo's `.markdownlint.yaml` (MD051 enabled):
   restructure a freshly created ADR, count MD051 failures, run
   `docz update --dry-run` and `docz update`, count again. Then probe the
   edge cases: anchor slugs, marker spelling variants, disabled types, files
   outside the `NNNN-*.md` pattern, files without frontmatter, `min_headings`,
   marker literals inside code fences, a global `~/.docz.yaml` override, and
   the `--dry-run` exit code.
4. Run markdownlint-cli2 over this repo's `docs/**` to measure what MD051
   reports today and what else a lint gate would trip on.
5. Check which editor tool actually emits the `<!--toc:start-->` marker
   format, since README and DESIGN-0003 disagree about the plugin name.

## Environment

| Component        | Version / Value                                                           |
| ---------------- | ------------------------------------------------------------------------- |
| docz             | `v1.2.2-1-g9c2f11f` (build of `main` at `9c2f11f`; no CLI changes since v1.2.2) |
| markdownlint-cli2 | 0.18.1                                                                   |
| lint config      | this repo's `.markdownlint.yaml` (MD004/MD007/MD013/MD030/MD033 off, MD051 on) |
| repro config     | `docz init` defaults: `toc.enabled: true`, `toc.min_headings: 3`, PLAN disabled |
| skills plugin    | `donaldgifford-claude-skills` docz plugin 1.3.0                           |

## Findings

### Observation 1: `docz update` regenerates ToC fences — the excerpt is wrong

A fresh `docz create adr "Repro stale toc"` produces a fence populated from
the template's headings (the create path runs the same update pass). Renaming
the first two H2s after the fence makes the two ToC links dangle:

```text
$ grep -n "^## " docs/adr/0001-repro-stale-toc.md | head -3
26:## Summary Renamed
30:## Context Also Renamed
34:## Decision
$ markdownlint-cli2 docs/adr/0001-repro-stale-toc.md | grep -c MD051
2

$ docz update --dry-run | grep "Would update ToC"
Would update ToC in .../docs/adr/0001-repro-stale-toc.md (10 headings)

$ docz update
Updated .../docs/rfc/README.md
Updated .../docs/adr/README.md
Updated .../docs/design/README.md
Updated .../docs/impl/README.md
Updated .../docs/investigation/README.md

$ markdownlint-cli2 docs/adr/0001-repro-stale-toc.md | grep -c MD051
0
$ grep -n Renamed docs/adr/0001-repro-stale-toc.md | head -2
14:- [Summary Renamed](#summary-renamed)
15:- [Context Also Renamed](#context-also-renamed)
```

The code agrees: `updateType` in `cmd/update.go` calls `runToCUpdate`
whenever `Cfg.TOC.Enabled` is true (the default), before the README table
pass, over every document `ScanDocuments` returned. `toc.UpdateFiles`
splices a fresh list between the markers and writes the file only when the
bytes changed. Nothing in the repo documents a reason *not* to regenerate;
DESIGN-0003 Decision 4 is the opposite — it folded ToC generation into
`docz update` instead of adding a standalone `docz toc` command.

### Observation 2: the pass is silent and undocumented where an agent looks

Every surface the other session would plausibly have consulted says "README
tables" and nothing else:

| Surface | What it says about ToCs | Notes |
| ------- | ----------------------- | ----- |
| `docz update --help` | Nothing. `Long` reads "Regenerate the auto-generated table in the README.md for the specified document type directory." | Written 2026-02-22 in `23092dc`, before IMPL-0003 added the ToC pass; never updated. |
| Normal `docz update` output | Nothing. Only `Updated <dir>/README.md` per type (see Observation 1). | ToC writes are `Logger.Debug` only (`cmd/update.go:186`); skipped files are never reported at any level. |
| `docz update --dry-run` | One line per changed file, `Would update ToC in <path> (N headings)`. | In the repro that line was line 38 of 165, because the README pass prints each type's *entire* would-be README body. IMPL-0003 Decision 2 asked for a concise per-file ToC summary; the README dump predates it and drowns it. |
| claude-skills `docz:update` skill (1.3.0) | "Regenerate README index tables". The umbrella `docz` skill: "`docz update` — regenerates all README tables". | No occurrence of "toc" or "table of contents" in `skills/update/SKILL.md` or `skills/docz/references/*.md`. |
| `docz_yaml.tmpl` `toc:` block | Two bare keys, no comment. | Never had one (`git log -p` on the template). The excerpt's "your `.docz.yaml` comment says the ToC is generated during update" did not come from docz's generated config. |
| `README.md` "Table of Contents" | Correct: "`docz update` regenerates the ToC". | The only user-facing place that states it. |
| `CLAUDE.md` | Correct. | Not visible outside this repo. |
| DESIGN-0003 / IMPL-0003 | Correct and detailed. | Internal design docs. |

An agent following the plugin's skill, then confirming with `--help`, then
glancing at a normal run's output, has three independent signals that
`docz update` is README-only and zero that it is not. That is sufficient to
explain the excerpt without any bug in the CLI.

### Observation 3: cases where a fence really is left stale, with no signal

| # | Condition | Behaviour | Evidence |
| - | --------- | --------- | -------- |
| 1 | Marker spelling variant: `<!-- toc:start -->` (inner spaces) or `<!-- mtoc-start -->` | Not recognised. Markers are exact-match constants (`toc.go:16-17`) located with `strings.Cut`. No message in dry-run or normal run. | Repro 5: dry-run lines for the file: 0; stale link kept; MD051: 1. |
| 2 | Filename does not match `document.DoczFilePattern` (`NNNN-*.md`), e.g. `notes.md` | Never scanned. | Repro (earlier pass): no dry-run line, fence untouched. |
| 3 | File matches the pattern but has no valid frontmatter | `ScanDocuments` drops it. | Same. |
| 4 | Type disabled — PLAN is off by default since v1.1.1 (DESIGN-0011) | Whole directory skipped, so plan docs written before the default flipped rot silently. | Repro 6: `plan.enabled: false`; dry-run mentions of `docs/plan`: 0; stale link kept. |
| 5 | `docz update <type>` | Other types untouched. | By design — DESIGN-0003 Decision 1 ties ToC scoping to update scoping. |
| 6 | `toc.enabled: false` in the repo `.docz.yaml` | Pass off. A *global* `~/.docz.yaml` cannot do this to an `init`-generated repo, because the template writes `enabled: true` explicitly and repo keys win the merge. | Repro (earlier pass): global override → `docz config` still shows `enabled: true`, ToC line still printed. |
| 7 | A marker literal inside a code sample that precedes the real fence | `strings.Cut` takes the sample's markers; `parseHeadings` resumes after the sample's end marker with inverted fence state, sees zero headings, writes nothing. Real fence stays stale. | Repro (earlier pass, ADR-0010): no dry-run line, `Old Link` kept, MD051: 1. Contrived; noted for completeness. |
| 8 | Fewer than `min_headings` headings after the fence | Fence is *emptied*, so stale entries are removed. Not an MD051 source. | Repro (earlier pass). |

Rows 1–4 are the ones that would bite a real repo: the user sees a fence,
runs `docz update`, and nothing happens, with no explanation. Row 1 is
directly connected to Observation 5.

### Observation 4: MD051 fires right after a *successful* `docz update`

Three heading shapes produce a ToC entry whose anchor GitHub (and therefore
markdownlint, which uses GitHub's slug algorithm) does not generate:

```text
$ docz update adr && sed -n '/toc:start/,/toc:end/p' docs/adr/0002-slug-cases.md
<!--toc:start-->
- [The id_prefix rule](#the-idprefix-rule)
- [Über uns](#ber-uns)
- [Hidden by a comment](#hidden-by-a-comment)
- [Plain](#plain)
<!--toc:end-->
$ markdownlint-cli2 docs/adr/0002-slug-cases.md | grep MD051
  MD051/link-fragments ... [Context: "[The id_prefix rule](#the-idprefix-rule)"]
  MD051/link-fragments ... [Context: "[Über uns](#ber-uns)"]
  MD051/link-fragments ... [Context: "[Hidden by a comment](#hidden-by-a-comment)"]
```

- **Underscores** — `docparse.AnchorSlug` keeps only `[a-z0-9 -]`, GitHub keeps
  `_`. This is issue #82. It is also the *only* MD051 shape in this repo
  today: markdownlint-cli2 over `docs/**` reports five MD051 failures, all
  from headings containing `docs_dir`, `id_prefix`, `additional_docs`,
  `status_field`, or `updated_field` (DESIGN-0006, INV-0007, IMPL-0016,
  INV-0008). Every one is regenerated on each `docz update`, which is why
  #82 calls the failure "sticky".
- **Non-ASCII letters** — the same character class drops `Ü`, so `Über uns`
  becomes `#ber-uns`; GitHub produces `#über-uns`. Not previously reported.
- **Headings inside HTML comments** — `docparse.Headings` is fence-aware but
  not comment-aware, so a `## …` line inside `<!-- … -->` gets a ToC entry
  for a heading no renderer emits. Not previously reported. (The doc
  templates themselves put guidance in HTML comments, so this is not
  hypothetical — a template comment that happens to start with `##` would
  trigger it.)

`toc.go`'s package comment promises "GitHub-compatible anchor slugs", so all
three are bugs against docz's own stated contract, not a markdownlint
configuration problem.

### Observation 5: the marker format is Marksman's, and README names the wrong plugin

`README.md` lines 554–555 say the markers "are compatible with the
[markdown-toc.nvim](https://github.com/hedyhli/markdown-toc.nvim) plugin, so
documents edited in Neovim/lazyvim will work with both tools." They are not:

- markdown-toc.nvim's default fences are `<!-- mtoc-start -->` /
  `<!-- mtoc-end -->`. The fence *text* is configurable, but the plugin
  always wraps it as `<!-- % -->`, so the closest it can get is
  `<!-- toc:start -->` with inner spaces — which docz does not recognise
  (Observation 3, row 1). A user who follows the README and sets up mtoc gets
  fences docz silently ignores.
- `<!--toc:start-->` / `<!--toc:end-->` with no spaces is the fence emitted by
  **Marksman**'s "create table of contents" code action — the markdown
  language server that LazyVim's `lang.markdown` extra ships. A GitHub code
  search for the literal in Lua files turns up LazyVim users' markdown
  ftplugins working around Marksman's ToC output, not mtoc configs.

DESIGN-0003 ("use the same markers as the lazyvim plugin") is right in
spirit but never names the tool. The README attribution is simply wrong and
should name Marksman, and both should say the match is exact.

### Observation 6: nothing gates drift, so MD051 in a downstream CI is the first alarm

- This repo ships `.markdownlint.yaml` with MD051 enabled but nothing in the
  `Makefile` or `.github/workflows/` runs markdownlint. Running
  markdownlint-cli2 0.18.1 over `docs/**` today gives 243 findings across 13
  rules; MD051 accounts for 5 (Observation 4). The bulk is MD024 (142,
  duplicate sibling headings such as the per-phase "Success Criteria" in IMPL
  docs), then MD049, MD040, MD031, MD038. A lint gate here is a separate
  config exercise, not a one-line addition.
- `docz update --dry-run` exits 0 whether or not anything would change (repro
  7), so it cannot gate CI. The `docz` skill's "use `--dry-run` in CI to detect
  drift" line is aspirational — a CI job would have to grep the output.

### Observation 7 (side finding): doubled index markers in two generated READMEs

`index_investigation.md` and `index_plan.md` — alone among the embedded index
headers — end with their own `<!-- BEGIN/END DOCZ AUTO-GENERATED -->` pair, and
`internal/index.createNewReadme` appends another. Every `docz init` therefore
produces `docs/investigation/README.md` and `docs/plan/README.md` with two
marker pairs; the splice only ever touches the first, and this repo's
`docs/investigation/README.md` carries the empty second pair today. Cosmetic
and unrelated to MD051, but it came out of the same header read and deserves
its own issue.

## Conclusion

**Answer:** No — the claim is refuted. `docz update` (and `docz create`)
regenerate `<!--toc:start-->` / `<!--toc:end-->` fences in every scanned
document whenever `toc.enabled` is true, which it is by default. The
reproduction took a document from two MD051 failures to zero with a single
`docz update`, and the dry-run reported the change beforehand.

The other session reached the wrong conclusion because every surface it
would have consulted — `docz update --help`, the `docz:update` skill, the
umbrella `docz` skill, and the CLI's normal-run output — describes README
tables only. That is a documentation and UX gap on docz's side, not a
misreading on theirs. None of the three options it offered (delete the fence,
override the templates, re-implement regeneration) was the right move.

There are, however, three real ways MD051 fires on docz-managed docs, and
they were probably what the other session was actually looking at:

1. **Slug divergence from GitHub** — underscores (#82), non-ASCII letters,
   and headings inside HTML comments. These fail *after* a successful
   `docz update` and are re-created on every run.
2. **Silently skipped files** — marker variants (notably the spaced form
   every mtoc.nvim configuration produces), disabled types (PLAN by
   default), non-pattern filenames, and files without frontmatter. The fence
   stays stale and nothing says why.
3. **No gate** — dry-run exits 0 on drift and there is no lint job, so a
   downstream repo's markdownlint is the first place drift becomes visible.

Nowhere in the repo is there a documented reason for *not* regenerating;
DESIGN-0003 documents the opposite. The gaps are in `--help`, the generated
config, the skills plugin, and the README's plugin attribution.

## Recommendation

In priority order. Items 1–2 are what closes the loop on the excerpt; 3 is
what actually makes MD051 pass; 4–5 make drift visible before a downstream
lint job does.

1. **Fix the words (docz, patch release).** Rewrite `docz update`'s `Long`
   help to name both passes (ToC fences in every scanned doc, then README
   tables); add a comment above the `toc:` block in `docz_yaml.tmpl` saying
   the fence is regenerated by `docz update` and the markers must match
   exactly; correct README's plugin attribution to Marksman and state the
   exact-match rule; amend DESIGN-0003's Marker Format section to name
   Marksman.
2. **Fix the skills (claude-skills, plugin 1.3.x).** `skills/update/SKILL.md`,
   the umbrella `docz` SKILL, and `references/workflow.md` should say
   `docz update` regenerates ToC fences as well as README tables, and that a
   stale fence means "run `docz update`", never "delete the fence". Tracked
   as an issue in that repo (claude-skills #98), not as a PR from this work
   (Decision 6).
3. **Fix slug fidelity in `docparse` (docz, one `fix(docparse)` release).**
   Make `AnchorSlug` match GitHub for underscores (#82) and Unicode
   letters/digits, and make `Headings` skip HTML-comment-enclosed lines.
   Pin with goldens taken from GitHub's rendered anchors. This changes
   generated ToC bytes for affected headings, so call it out in the
   changelog as consumer-visible even though it is a bug fix toward the
   documented contract.
4. **Give the pass a voice.** Print `Updated ToC in <path>` in normal runs
   (mirroring the README lines), and warn once per file when a document
   contains a near-miss marker (`<!-- toc:start -->`, `mtoc-start`) but no
   exact fence, so Observation 3 row 1 stops being silent.
5. **Add a CI-gating mode.** `docz update --check`: dry-run semantics, drift
   summary, non-zero exit when any fence or README would change. Then the
   skill's "detect drift in CI" claim becomes true.
6. **Issues filed 2026-09-13** once the decisions below were locked:
   docz #94 (item 1), claude-skills #98 (item 2), docz #96 alongside #82
   (item 3), docz #95 (item 4), docz #97 (item 5), docz #98 (a markdownlint
   job for this repo's own docs — 243 findings need config decisions first,
   and MD051 should be fixed at the source before the gate goes in so the
   linter never fights `docz update`), and docz #99 (the doubled index
   markers, Observation 7).

Do **not** accept marker variants as a fix for row 1 — see Decision 2.

### Decisions

All seven open questions were resolved on 2026-09-13 in review — 1–5 and
7 as **(a)**, 6 as (a) for the docz half with the claude-skills half
becoming an issue in that repo rather than a PR from this work. The
lettered options are kept below for the alternatives, which record what
was weighed.

| #   | Question                          | Decision                                                                                      | Tracked in |
| --- | --------------------------------- | --------------------------------------------------------------------------------------------- | ---------- |
| 1   | Report ToC changes in normal runs | yes — `Updated ToC in <path>` per changed file, silent when nothing changed                   | docz #95 |
| 2   | Accept marker spelling variants   | no — exact Marksman markers stay; warn once per file on a near-miss with no exact fence       | docz #95 |
| 3   | Slug-fidelity scope               | all three divergences (underscores #82, Unicode letters/digits, HTML-comment headings) in one `fix(docparse)` release, goldens from GitHub's rendered anchors, changelog note | docz #96 + #82 |
| 4   | CI-gating mode                    | `docz update --check` — dry-run semantics, drift summary only, exit 1 on drift; `--dry-run` unchanged | docz #97 |
| 5   | markdownlint in this repo's CI    | not now — chore issue; land the slug fix first, then config decisions (MD024 siblings, MD040 languages) | docz #98 |
| 6   | Where the doc fixes land          | docz: help text, `docz_yaml.tmpl` comment, README attribution, DESIGN-0003 amendment as a `patch` PR; claude-skills: a gh issue in that repo | docz #94; claude-skills #98 |
| 7   | Doubled index-marker pair         | separate `fix(index)` issue, plus a one-off cleanup of this repo's investigation README         | docz #99 |

Open questions as put to review, recommended option first:

### 1. Should `docz update` report ToC changes in normal (non-dry-run) output?

- a. **Yes, one line per changed file** — `Updated ToC in <path>`, silent when
  nothing changed, matching the existing `Updated <dir>/README.md` lines.
  This is the signal the other session lacked. DESIGN-0012 Decision 7 chose
  silence for the `updated:` stamp, but that stamp is bookkeeping on a doc
  the user already changed; a ToC rewrite is a content change they need to
  see and commit. *(recommendation)*
- b. A single summary line per run (`Updated ToC in 3 documents`).
- c. Keep it debug-only (status quo) and rely on `--verbose`.
- d. Other.

### 2. Should docz recognise marker spelling variants such as the spaced mtoc form?

The variants in question are `<!-- toc:start -->` (inner spaces, the closest
markdown-toc.nvim can be configured to produce) and mtoc's default
`<!-- mtoc-start -->`.

- a. **No — keep the exact Marksman markers, but warn on near-misses.** Print a
  one-line warning in both dry-run and normal runs when a scanned file
  contains `toc:start` or `mtoc-start` inside an HTML comment but no exact
  fence. Preserves the byte-stable splice contract (DESIGN-0003, toc
  goldens, docz-api's ingest) while removing the silent-rot case.
  *(recommendation)*
- b. Accept whitespace-tolerant `<!--\s*toc:start\s*-->` and rewrite to the
  canonical spelling on the next update.
- c. Also accept mtoc's fences, via a `toc.markers` config pair.
- d. Other.

### 3. How far should the slug-fidelity fix go?

- a. **All three divergences in one `fix(docparse)` release** — underscores
  (#82), Unicode letters/digits, and HTML-comment-enclosed headings — with
  goldens copied from GitHub's rendered anchors and a changelog note that ToC
  bytes change for affected headings. One consumer-visible change instead of
  three. *(recommendation)*
- b. Underscores only, as #82 is scoped; file the other two for later.
- c. Underscores and Unicode now; HTML comments later (it needs a
  comment-aware walker in `docparse.Headings`, which `Title` and `TaskItems`
  would inherit).
- d. Other.

### 4. Should `docz update` gain a CI-gating mode?

- a. **Yes, `docz update --check`** — dry-run semantics, prints only the drift
  summary lines (no README body dumps), exits 1 when any ToC or README would
  change and 0 otherwise. `--dry-run` keeps its current behaviour and exit
  code. *(recommendation)*
- b. Make `--dry-run` itself exit non-zero on drift. Simpler, but changes an
  existing flag's contract for anyone scripting it.
- c. No CLI change; rely on markdownlint MD051 in consumers' CI as the gate.
- d. Other.

### 5. Should this repo run markdownlint in CI now?

- a. **Not in this fix — file a chore issue.** Today's 243 findings across 13
  rules need config decisions (MD024 `siblings_only` for per-phase "Success
  Criteria" headings, fence languages for MD040, and so on). Land the slug
  fix first so the gate never contradicts `docz update`. *(recommendation)*
- b. Add the job now with a CI-only config enabling MD051 alone; the five
  current failures are all #82 and would be fixed by Decision 3.
- c. Add the job now with the full config and fix all 243 findings in the
  same PR.
- d. Other.

### 6. Where do the documentation fixes land?

- a. **Two PRs.** docz: help text, `docz_yaml.tmpl` comment, README
  attribution, DESIGN-0003 amendment — a `patch` release since help text is
  code. claude-skills: `docz:update` skill, umbrella SKILL, workflow
  reference — plugin 1.3.1. *(recommendation)*
- b. docz only for now; skills in the next plugin bump.
- c. Fold the docz text changes into the slug-fix PR (Decision 3) as one
  release.
- d. Other.

### 7. What to do with the doubled index-marker pair (Observation 7)?

- a. **Separate `fix(index)` issue** — strip the trailing pair from the two
  embedded headers (or teach `createNewReadme` not to append one when the
  header already carries it), plus a one-off cleanup of this repo's
  `docs/investigation/README.md`. Out of scope here. *(recommendation)*
- b. Fold into the slug-fix PR.
- c. Leave it; the splice ignores the second pair so it is cosmetic.
- d. Other.

## References

- [DESIGN-0003: Table of Contents Generation](../design/0003-table-of-contents-generation.md)
  — Decisions 1 (scoping), 4 (no standalone `docz toc`), Marker Format section
- [IMPL-0003: Table of Contents Generation](../impl/0003-table-of-contents-generation.md)
  — Decision 2 (concise dry-run summary)
- [DESIGN-0012](../design/0012-updated-frontmatter-field-opt-in-stamp-pass-in-docz-update.md)
  — Decision 7 (silent normal-run output for the `updated:` stamp)
- [Issue #82](https://github.com/donaldgifford/docz/issues/82) — underscores
  stripped from ToC anchors
- Follow-up issues filed from this investigation:
  [docz #94](https://github.com/donaldgifford/docz/issues/94) (help text,
  config comment, README attribution, DESIGN-0003),
  [docz #95](https://github.com/donaldgifford/docz/issues/95) (normal-run
  ToC output, near-miss marker warning),
  [docz #96](https://github.com/donaldgifford/docz/issues/96) (non-ASCII and
  HTML-comment slug divergences, with #82),
  [docz #97](https://github.com/donaldgifford/docz/issues/97)
  (`docz update --check`),
  [docz #98](https://github.com/donaldgifford/docz/issues/98) (markdownlint
  in CI),
  [docz #99](https://github.com/donaldgifford/docz/issues/99) (doubled index
  markers),
  [claude-skills #98](https://github.com/donaldgifford/claude-skills/issues/98)
  (docz plugin skills)
- `cmd/update.go` (`updateCmd.Long`, `runToCUpdate`), `pkg/doczcore/toc/toc.go`
  (`BeginMarker`/`EndMarker`, `parseHeadings`, `UpdateToC`),
  `pkg/doczcore/docparse` (`Headings`, `AnchorSlug`),
  `internal/index/index.go` (`createNewReadme`),
  `internal/template/templates/index_investigation.md`, `index_plan.md`
- `README.md` "Table of Contents" section (lines 528–571 at `9c2f11f`)
- claude-skills docz plugin 1.3.0: `skills/update/SKILL.md`,
  `skills/docz/SKILL.md`, `skills/docz/references/`
- [markdownlint MD051/link-fragments](https://github.com/DavidAnson/markdownlint/blob/main/doc/md051.md)
- [Marksman](https://github.com/artempyanykh/marksman) — source of the
  `<!--toc:start-->` / `<!--toc:end-->` fence
- [markdown-toc.nvim](https://github.com/hedyhli/markdown-toc.nvim) — default
  fences `<!-- mtoc-start -->` / `<!-- mtoc-end -->`, always wrapped as
  `<!-- % -->`
