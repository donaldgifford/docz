# pkg/impl fixtures

Two files per document, plus a fact file:

| File | What it is |
|---|---|
| `<name>.orig.md` | A verbatim snapshot of a real plan, byte for byte as written |
| `<name>.md` | The same document with canonical region markers added and nothing else changed |
| `<name>.golden.txt` | The facts `Parse` reads out of the migrated copy |

Regenerate the last two with `go test ./pkg/impl/... -update`. Never edit them
by hand. Never edit a `.orig.md` either: it is a snapshot, and its whole value
is that it is not maintained alongside the parser.

Snapshots rather than reads from `docs/` because a fixture that followed this
repo's own documents would rewrite its own expectations every time somebody
edited a plan. These exist to pin the grammar against documents as they were
actually written, by people who had never heard of the grammar.

## Where each came from

| Fixture | Source | What it is here for |
|---|---|---|
| `docz-impl-0001` | docz IMPL-0001 | Five `### Phase N` headings under File Changes that are not phases |
| `docz-impl-0007` | docz IMPL-0007 | A phase with no Success Criteria section |
| `docz-impl-0009` | docz IMPL-0009 | Eleven phases, 66 tasks, heavy wrapping |
| `docz-impl-0017` | docz IMPL-0017 | Wrapping, eight open questions, eight decisions |
| `api-impl-0004` | docz-api IMPL-0004 | Verify lines, fenced code inside task bullets |
| `api-impl-0006` | docz-api IMPL-0006 | The prefix deferred marker |
| `booty-clean` | sdk-booty-sh `doczwork` testdata | The minimal well-formed plan |
| `booty-messy` | sdk-booty-sh `doczwork` testdata | A phase heading in bold, checkboxes with no Tasks heading |
| `synthetic-skipped` | written for this package | Every spelling of an abandoned task |
| `synthetic-mid-run` | written for this package | A task inserted into a half-finished phase |

## The migration rule

A `.md` copy is the spans inference already found, written out as markers.
Nothing else: no heading is added, no prose is moved, no blank line is
inserted. `booty-messy` keeps its checkboxes under a phase heading with no
Tasks heading above them, so its migrated copy reports the same zero tasks the
original does. A migration that tidied the document would make the pair test
prove nothing.

That rule makes these files the expected output of Phase 3's `InsertRegions`.

## What the corpus taught us

Each of these is a fact about how people write plans, found by running the
parser over documents rather than over fixtures somebody wrote to pass.

- **Wrapping is the norm, not an edge case.** 40 of 43 tasks in docz-api
  IMPL-0004 wrap, 32 of 32 in IMPL-0006, 44 of 47 in docz IMPL-0017. A reader
  that took only a checkbox's first line would truncate almost every task in
  the fleet.
- **A level-2 section can swallow a level-3 phase.** `booty-messy` puts
  `### Phase A:` directly under `## Objective` with no level-2 heading between
  them. Inference by heading level alone made the objective run to the end of
  the document and contain the phase, which no arrangement of markers can
  express — so the migrated copy came back with the phase nested a level too
  deep and no phases at all. This is why a span also ends at a deeper heading
  that opens a region it cannot contain.
- **Most verify steps are written inline, where the grammar does not read
  them.** docz-api IMPL-0004 has 43 tasks and only four lines that start with
  `Verify:`; the rest say `…get their modelines then. Verify: \`yamllint …\``
  mid-sentence. `Verify` is empty for those and the text stays in `Text`, which
  is correct — reading a code span from the middle of a sentence would name a
  filename as a command as often as a command.
- **A blank line inside a task bullet ends the task.** docz-api IMPL-0004 puts
  fenced code blocks inside task bullets, with blank lines around them, so a
  task's `Text` stops at the first blank line and the `Verify:` line below the
  fence belongs to no task. That is the documented continuation rule, and the
  alternative — letting a bullet absorb everything filed beneath it until the
  next bullet — would swallow whole sections.
- **A task can be nothing but its marker.** docz-api IMPL-0006 writes
  `- [ ] **deferred - blocked, no cluster available** — redeploy the new
  image + chart; …`, where the work is after the marker rather than before it.
  The note folds to the end of the task, so `Text` is empty and
  `impl.task.empty` reports it. The rule cannot tell where such an author meant
  the note to stop, and guessing would be worse than saying so.
- **A skipped task must keep its ID.** `synthetic-skipped` has tasks 1.2 and
  1.4 abandoned; 1.3 and 1.5 keep the IDs they had. Renumbering would move
  every address in the phase the moment one task was given up on, and those
  addresses are in commit messages and CI logs.
- **Inserting a task mid-run does move the IDs after it.** `synthetic-mid-run`
  records it, because there is no way around it: an ID is positional, so a
  consumer that stored "1.3 is done" is wrong once a task is inserted above it.
  The golden exists so the behaviour is a decision somebody can read rather
  than a surprise.
