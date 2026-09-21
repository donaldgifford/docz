# pkg/investigation fixtures

Two files per document, plus a fact file:

| File | What it is |
|---|---|
| `<name>.orig.md` | A verbatim snapshot of a real investigation, byte for byte as written |
| `<name>.md` | The same document with canonical region markers added and nothing else changed |
| `<name>.golden.txt` | The facts `Parse` reads out of the migrated copy |

Regenerate the last two with `go test ./pkg/investigation/... -update`. Never
edit them by hand. Never edit a `.orig.md` either: it is a snapshot, and its
whole value is that it is not maintained alongside the parser.

`grammar.md` is not part of this corpus. It is the hand-written fixture
`parse_test.go` and `validate_test.go` assert against, it has no `.orig.md`, and
the corpus glob (`*.orig.md`) does not find it.

Snapshots rather than reads from `docs/` because a fixture that followed this
repo's own documents would rewrite its own expectations every time somebody
edited an investigation. These exist to pin the grammar against documents as
they were actually written, by one author over a year, before the grammar
existed.

## Where each came from

| Fixture | Source | What it is here for |
|---|---|---|
| `docz-inv-0001` | docz INV-0001 | The oldest: no Environment section, decisions as a numbered list, an answer with no verdict in it |
| `docz-inv-0002` | docz INV-0002 | The largest (1,005 lines); fourteen findings that are severity buckets holding fifty `#### F<n>.` findings |
| `docz-inv-0003` | docz INV-0003 | An `## Options` section the grammar does not name |
| `docz-inv-0004` | docz INV-0004 | A three-column environment table; a hypothesis written as a bullet list |
| `docz-inv-0005` | docz INV-0005 | Eight open questions, none resolved by the grammar's blockquote; a decisions table headed `Topic` |
| `docz-inv-0006` | docz INV-0006 | The only document whose questions carry `> **Resolved …: (a)**` blockquotes, and whose decisions table parses |
| `docz-inv-0007` | docz INV-0007 | Option letters in bold; a table inside the conclusion; `**Answer: No — …**` |
| `docz-inv-0008` | docz INV-0008 | Nine observations, heavy wrapping, ten unlinked references |
| `docz-inv-0009` | docz INV-0009 | HTML comments inside code spans in the question and the context; the first document with linked references |
| `docz-inv-0010` | docz INV-0010 | Nine open questions with 3–5 lettered options each and no Decisions section |
| `no-trigger` | written for this package | A Context section with no `**Triggered by:**` field, which no real investigation has |

`no-trigger` exists because all ten real documents carry the field, so
`inv.context.no-trigger` had no live corpus instance without it. It is the only
fixture here that was not written by somebody solving a real problem, and the
only one this package is allowed to grow.

## The migration rule

A `.md` copy is the spans inference already found, written out as markers.
Nothing else: no heading is added, no prose is moved, no blank line is inserted.
`docz-inv-0001` keeps its decisions as a numbered list where the template asks
for a table, so its migrated copy reports the same zero decisions the original
does. `docz-inv-0003` keeps its `## Options` section outside every region. A
migration that tidied the documents would make the pair test prove nothing.

That rule makes these files the expected output of Phase 3's `InsertRegions`. It
adds 16 to 22 lines per document — two per region — and
`TestCorpusMigrationChangesNothingButMarkers` asserts that every field, step,
row, finding, question, decision, and reference reads the same on both sides.

## What the corpus taught us

Each of these is a fact about how people write investigations, found by running
the parser over documents rather than over fixtures somebody wrote to pass.

- **Two spellings of the answer field are written, and only one is read.**
  Seven documents write `**Answer:** …`. Three — INV-0005, INV-0006,
  INV-0007 — write `**Answer: Yes — …**`, with the colon inside the bold and
  the bold closing later in the sentence. `kinds.Field` matches `**Answer:**`
  and `**Answer**:`, and that third shape is neither, so all three come back
  with `Answer` empty and `Verdict` unknown. INV-0005 and INV-0007 are
  `Concluded`, so both report `inv.conclusion.no-answer` — an error against a
  document that answers its question in the first sentence of its conclusion.
  INV-0006 escapes only because its status is still `Open` and the rule is
  gated on the status, which is the gate doing its job for the wrong reason.
  Nobody wrote these three wrong on purpose, which is exactly why they are this
  rule's corpus instance: the finding is about the field grammar, not about the
  documents.
- **An answer that wraps loses everything after its first line.** `kinds.Field`
  reads one line, and six of the seven `**Answer:**` documents wrap:
  `Answer` ends "…cleanup opportunity. The" (INV-0002), "…not a" (INV-0003),
  "…The TUI work is real" (INV-0004), "…roughly four days, with" (INV-0008),
  "…(and `docz create`)" (INV-0009), "…with the design" (INV-0010). The
  verdict survives, because it is the first word; the sentence a consumer
  renders beside it is cut mid-clause. Only INV-0001's answer fits on one line.
- **An HTML comment inside a code span is stripped along with the comments.**
  INV-0009 asks whether `docz update` regenerates the
  "`` `<!--toc:start-->` `` / `` `<!--toc:end-->` `` fence". `kinds.Body` walks
  for `<!--` before anything has looked for backticks, so `Question` reads
  "regenerate the `` / `` fence" and `Context` quotes the other session as "It
  does not touch the `` / `` block". This is the only place in the corpus where
  a field loses words the author wrote. The list readers do not strip comments,
  so the same text survives verbatim in a `Reference.Text` two hundred lines
  further down — the asymmetry is between `Body` and `foldItems`, not between
  documents.
- **A findings section's level-3 headings are the author's outline, not a list
  of findings.** INV-0002's fourteen sections are severity buckets — "Critical:
  Testability blockers", "Medium: Mechanical / style fixes" — with fifty
  `#### F<n>.` findings (F1 through F50) filed under them, all of which land inside
  `Section.Body`. INV-0001 numbers them "Finding 1"–"Finding 4", INV-0005
  "Observation 1"–"Observation 6", INV-0007 "F1."–"F7.". Four conventions in ten
  documents, so nothing may depend on the wording, and `len(Findings)` is not
  what a person means by "how many findings".
- **A decisions table is read only when its columns are named the way the reader
  knows them.** INV-0006 (`# | Question | Choice | Notes`) and INV-0007
  (`# | Question | Resolution`) parse. INV-0005 heads the same table
  `# | Topic | Choice | Rationale` and reports no decisions at all, so the eight
  choices it records are invisible to the typed model. INV-0001 writes its
  decisions as a numbered list and has none either. Three shapes of one section
  across ten documents, and only one of them reaches a consumer.
- **An option letter in bold is not an option.** INV-0007 writes
  `- **a. (Recommendation) …**`, so all three of its questions report zero
  options while INV-0010's nine plain `- a.` questions report three to five
  each. The grammar wants the letter at the very start of the bullet, and there
  is no way to loosen that without reading the "b" of "**b**old" as a choice.
- **Almost nothing resolves a question the way the grammar reads a
  resolution.** Of 24 open questions in the corpus, 4 are resolved — all of
  them INV-0006's, the only document using `> **Resolved 2026-07-03: (a)**`.
  INV-0005 records its resolutions in the decisions table ("Resolved by user
  review on 2026-06-23"); INV-0007 in a prose line above its table ("All three
  open questions resolved **(a)** on 2026-08-10"). Both come back with every
  question open. The resolution blockquote is a real convention, and it is one
  document's convention.
- **Most references are not links.** 84 references across the corpus, 13 with a
  URL — eight in INV-0009, four in INV-0010, one in `no-trigger`. The other
  eight fixtures have none: their bullets are backticked file paths
  (`` `cmd/wiki.go` — `ensureDocsIndex()` ``), bare URLs after an em dash
  (`Bubble Tea v2 — https://github.com/…`), doc IDs (`DESIGN-0002: Wiki
  Command…`), or wiki links (`[[0004-v1-release-plan…]]`). A bare URL is not a
  markdown link and neither is a wiki link, so `Reference.URL` is empty and
  `Reference.Text` is the whole bullet. Reporting the bullet with an empty URL
  rather than dropping it is what lets validate name which one is unlinked.
- **A section the grammar does not name is simply not read, and nothing says
  so.** INV-0003 files its options in an `## Options` section between the
  conclusion and the recommendation. No heading rule matches it, so lines
  193–232 of the migrated copy sit outside every region, and no field and no
  finding mentions them.
- **The legacy ToC pair is why inference works at all.** All ten real documents
  carry `<!--toc:start-->`/`<!--toc:end-->`. If `kinds.marked` counted that pair
  as "this document carries markers", every one of them would resolve to a
  single `toc` region and every field would be zero — the whole fleet read as
  empty. The exclusion is load-bearing and this corpus is the proof.
- **The environment table's second column is headed four different ways, and a
  third column is dropped.** "Version / Value" in seven documents, "Value" in
  INV-0002, "Proposed choice" in INV-0005. INV-0004 adds a "Notes" column whose
  contents ("v2 line, breaking changes from v1 — see Findings") go nowhere,
  because the mapping is by position. Mapping by name would have read three of
  the nine tables. INV-0001 has no environment section at all: it predates that
  part of the template.
- **A string field keeps whatever the author put in the region.** INV-0004's
  hypothesis is a bullet list and `Hypothesis` keeps the `- ` markers;
  INV-0001's `Recommendation` opens "### Change 1: Wiki index template", because
  only `findings` and `open-questions` section by heading and every other string
  field is its region's body verbatim. Both are the grammar working, and both
  look wrong the first time you read a golden.
- **The fact files truncate on a byte, not a rune.** `excerpt` cuts at 90 bytes,
  so an em dash landing on the boundary renders as `\xe2` in a golden. It is an
  artifact of the fact file, not of the parse — `pkg/impl`'s goldens have the
  same three — and it is left alone so the two packages' harnesses stay the
  same code.
