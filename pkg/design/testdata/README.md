# pkg/design fixtures

Two files per document, plus a fact file:

| File | What it is |
|---|---|
| `<name>.orig.md` | A verbatim snapshot of a real design, byte for byte as written |
| `<name>.md` | The same document with canonical region markers added and nothing else changed |
| `<name>.golden.txt` | The facts `Parse` reads out of the migrated copy |

`grammar.md` is the hand-written fixture `parse_test.go` and `validate_test.go`
read; it is not part of the corpus and the golden harness ignores it.

Regenerate the migrated copies and the fact files with
`go test ./pkg/design/... -update`. Never edit them by hand. Never edit a
`.orig.md` either: it is a snapshot, and its whole value is that it is not
maintained alongside the parser.

Snapshots rather than reads from `docs/` because a fixture that followed this
repo's own documents would rewrite its own expectations every time somebody
edited a design. These exist to pin the grammar against documents as they were
actually written, by people who had never heard of the grammar.

## Where each came from

| Fixture | Source | What it is here for |
|---|---|---|
| `docz-design-0002` | docz DESIGN-0002 | No Open Questions, a `## Decisions` section that is a numbered list, and five body headings the template does not name |
| `docz-design-0004` | docz DESIGN-0004 | Questions numbered as an ordered list, and a decisions table headed `## Decisions Locked or Revised` |
| `docz-design-0009` | docz DESIGN-0009 | Eight questions, no decisions, every recommendation written with a bolded letter |
| `docz-design-0014` | docz DESIGN-0014 | The largest design in the fleet: 2,037 lines, a 64 KB Detailed Design, twelve resolved questions, and a decisions table filed inside Open Questions |
| `api-design-0001` | docz-api DESIGN-0001 | Both sections present: thirteen questions, a thirteen-row decisions table, and one section-level resolution blockquote instead of thirteen |

They were chosen for shape variety — a design with Open Questions and no
Decisions, one with Decisions and no Open Questions, one with both, and one
very large one. What a person reads as "has a decisions table" and what the
heading rules find are not the same set, which is most of what the corpus
taught us.

`docz-design-0014` is a snapshot of the document that specifies this package.
That is deliberate, and it is not updated to match anything: it is the longest
design available, and the only one that resolves a question in a second
blockquote paragraph.

## The migration rule

A `.md` copy is the spans inference already found, written out as markers.
Nothing else: no heading is added, no prose is moved, no blank line is
inserted. `docz-design-0002` keeps its decisions as a numbered list and keeps
its `## CLI Interface` / `## Nav Generation` / `## Implementation` sections
outside every region, so its migrated copy reports the same zero decisions and
the same empty `DetailedDesign` the original does. A migration that tidied the
document would make the pair test prove nothing.

That rule makes these files the expected output of Phase 3's `InsertRegions`.

## What the corpus exercises, and what it does not

`design.Validate` has four codes. Over both copies of all five documents the
corpus reaches two of them:

| Code | Corpus | Where |
|---|---|---|
| `design.parse` | not reached | A snapshot of a real design has frontmatter and LF endings. `TestValidate_ParseErrorIsOneFinding` covers it against bytes docz would not have written. |
| `design.goals.empty` | not reached | Every design in the corpus states its goals as bullets under `### Goals`. `TestValidate_Rules` covers it by deleting them from `grammar.md`. |
| `design.status.open-question` | 12 findings | `api-design-0001`, which is Approved and resolves its questions in one section-level blockquote |
| `design.decisions.mismatch` | 1 finding | `api-design-0001` question 2, whose row the reader cannot see |

The gaps are recorded rather than filled. A fixture written to trip a code
would be a synthetic document wearing a real one's clothes, and
`validate_test.go` already covers every code with one passing and one failing
shape of `grammar.md`. `TestCorpusCodeCoverage` asserts this table, so a code
that starts firing — or stops — is a decision somebody makes rather than a
fact that drifts.

One further gap is worth stating on its own: **not one document in this corpus
yields a single `kinds.Decision` row.** Two of the five have a decisions region
and both come back empty — `docz-design-0002`'s numbered list and
`api-design-0001`'s "Topic"-headed table — a third (`docz-design-0004`) has a
real table under a heading the rules do not match, and the other two have no
decisions heading at all (`docz-design-0014` files its table inside Open
Questions instead). The row shape is covered by `grammar.md` in `parse_test.go`
and by nothing real, which is itself the corpus's loudest finding about how
designs record their decisions.

## What the corpus taught us

Each of these is a fact about how people write designs, found by running the
parser over documents rather than over fixtures somebody wrote to pass.

- **The question column has to be called "Question".** `api-design-0001` heads
  its decisions table `# | Topic | Choice | Rationale / notes`, and
  `kinds.Decisions` locates the question column by name — "question" or "open
  question" and nothing else. So a thirteen-row table reads as zero rows, and
  `design.decisions.mismatch` fires on question 2 while the row answering it
  sits in the table further down the page. **This is the one finding in the
  corpus that lands on a correctly written document, and the document is not
  the thing that is wrong**: the reader accepts four spellings of the decision
  column ("decision", "resolution", "answer", "choice") and only two of the
  question column. A design that writes "Topic", "Subject", or "Area" is read
  as having no decisions at all, which is both a lossy parse and a false
  warning.
- **A resolution recorded once for a whole section resolves nothing.**
  `api-design-0001` opens `## Open Questions` with a single blockquote —
  "Resolved 2026-06-30 — see the Decisions table below for the chosen option
  per question" — and only question 2 carries a blockquote of its own. A
  resolution is read from under its question, so twelve of thirteen read as
  open, and at Approved that is twelve `design.status.open-question` errors on
  a design its author considers closed. The rule is doing what it says; the
  document answers its questions somewhere the grammar does not look.
- **A bolded option letter is not an option.** `docz-design-0009` and
  `api-design-0001` both write the recommendation as
  `- **a. (Recommended)** Vite + React SPA …`. The lettered-bullet rule reads
  the letter from the head of the folded item text, which begins `**`, so
  every one of 0009's eight questions reports three options rather than four —
  and the one always missing is the recommendation. `docz-design-0014`, which
  writes `- a. … *(recommendation)*`, reports all four, and is the only
  fixture in which any option comes back `Recommended`.
- **A design that predates the template's section names loses its middle.**
  `docz-design-0002` heads its body `## CLI Interface`, `## Nav Generation`,
  `## Document Types`, `## Configuration`, and `## Implementation`. None is a
  region, so close to 300 of its 454 lines are in no span at all, and
  `DetailedDesign`, `APIChanges`, `DataModel`, and `Rollout` are all empty.
  Nothing is broken — the
  heading table is the template's, and a section the template does not name is
  not a region — but a consumer that summarised a design from its fields would
  report this one as nearly blank, and this is the commonest shape of an old
  design in the fleet.
- **A decisions section need not be a table.** `docz-design-0002` heads a
  five-item numbered list `## Decisions` — "Ordering behavior", "`wiki init`
  runs `docz init`", and so on. The region is present and `Decisions` is nil,
  because `kinds.Decisions` reads rows and there is no table to read. That is
  present-and-empty rather than absent, and the mismatch rule is correctly
  silent for it: the document asks no numbered questions either, so there is
  nothing for a row to disagree with.
- **A renamed heading is not a region.** `docz-design-0004` heads its table
  `## Decisions Locked or Revised`. The rule is an exact folded match, so there
  is no decisions region, the ten locked decisions are invisible, and the
  mismatch rule is silent rather than wrong. Presence, not content, is what
  turns that rule on.
- **Numbering questions as an ordered list gives a document no questions.**
  `docz-design-0004`'s `## Open Questions` is three numbered list items, not
  `### N.` headings. The region is present and `OpenQuestions` is nil, which is
  why an Approved design with three questions written in plain sight earns no
  `design.status.open-question` finding. The corpus has three
  present-but-empty regions in all — this one and the two decisions sections
  above (`docz-design-0002`'s numbered list and `api-design-0001`'s
  "Topic"-headed table) — and `TestCorpusFieldIsZeroOnlyWhenItsRegionIs` logs
  each of them rather than skipping the case.
- **A decisions table can live inside Open Questions.** `docz-design-0014`
  files a `| # | Question | Decision |` summary under `## Open Questions`,
  above the first question, and has no `## Decisions` heading anywhere. So the
  table is not read, `Decisions` is nil, and the mismatch rule is silent —
  correct for the rule (a design with no decisions region must report nothing)
  and lossy for the table, which is the document's own summary of twelve
  resolutions.
- **A resolution with no parenthesised letter keeps its note and loses its
  choice.** `docz-design-0014`'s questions 10 and 11 resolve in prose —
  "superseded by DESIGN-0015", "one IMPL covering this design and DESIGN-0015
  together". `Choice` is `""` and `Note` holds the sentence, which is the only
  way to avoid reading the "s" of "superseded" as the letter chosen.
- **An empty note is a real state.** Five of 0014's resolutions are
  `> **Resolved 2026-09-19: (a).**` and nothing more. The date and the choice
  are the whole resolution, so `Note` is `""` — the trailing `.**` is
  punctuation, not a note that was dropped.
- **Wrapping is the norm, not an edge case.** Every list field in the corpus
  folds continuation lines: `docz-design-0002`'s first goal wraps onto the next
  line, and `api-design-0001`'s non-goals run to five. A `Line` is always the
  bullet's own line, never the first continuation, which is why
  `TestCorpusLinesAreFacts` checks every `Line` against `docparse.ListItems`
  over the whole document instead of against the field's text.
- **Most references have no URL.** All eight of `docz-design-0004`'s and the
  first five of `api-design-0001`'s are prose naming a document by its ID.
  `Reference.URL` is `""` and the bullet is still reported, which is what lets
  validate say *which* bullet is missing a link rather than only that one is.
- **Detailed Design is the field that needs a length, not an excerpt.** It is
  64,370 bytes in `docz-design-0014` and 20,458 in `api-design-0001`, so the
  golden records `len=` beside a 90-character excerpt: a span end that moved by
  a hundred lines is invisible in the first ninety characters and obvious in
  the byte count.
