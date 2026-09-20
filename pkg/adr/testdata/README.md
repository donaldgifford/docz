# pkg/adr fixtures

Two files per document, plus a fact file:

| File | What it is |
|---|---|
| `<name>.orig.md` | A verbatim snapshot of a real ADR, byte for byte as written |
| `<name>.md` | The same document with canonical region markers added and nothing else changed |
| `<name>.golden.txt` | The facts `Parse` reads out of the migrated copy |

`grammar.md` is not part of the corpus. It is the hand-written fixture
`parse_test.go` and `validate_test.go` assert against, and it has no `.orig.md`
sibling, so `corpus()` does not pick it up.

Regenerate the last two with `go test ./pkg/adr/... -update`. Never edit them
by hand. Never edit a `.orig.md` either: it is a snapshot, and its whole value
is that it is not maintained alongside the parser.

Snapshots rather than reads from `docs/` because a fixture that followed this
repo's own documents would rewrite its own expectations every time somebody
edited an ADR. These exist to pin the grammar against documents as they were
actually written, by people who had never heard of the grammar — and, because
they come from three repositories, by three sets of habits that disagree with
each other.

## Where each came from

| Fixture | Source | What it is here for |
|---|---|---|
| `docz-adr-0001` | docz ADR-0001 | Seven open questions whose options are bolded away; `## Corner Cases` and a `## Decisions` table no field reads |
| `docz-adr-0002` | docz ADR-0002 | A 190-line decision with mermaid, tables, and three subsections; supersession named in prose |
| `docz-adr-0003` | docz ADR-0003 | Fenced Go and mermaid inside the decision; options the reader does read, recommendation markers and all |
| `booty-adr-0001` | sdk-booty-sh ADR-0001 | Five alternatives written as bold paragraphs, so the field is nil |
| `booty-adr-0002` | sdk-booty-sh ADR-0002 | `## Decision` above `## Context`, plus `## Decision rationale` and `## Implementation notes` — two sections the model has no home for |
| `booty-adr-0003` | sdk-booty-sh ADR-0003 | The same repo's later habit: bulleted alternatives, subsections inside the decision |
| `tempy-adr-0001` | tempy ADR-0001 | The minimal shape: no summary, no open questions, references with no links |
| `tempy-adr-0002` | tempy ADR-0002 | A table in the context; a `## Status` section that disagrees with the frontmatter |

Headline facts, as the goldens record them:

| Fixture | Status | pos/neg/neu | Alternatives | Questions | References |
|---|---|---|---|---|---|
| `docz-adr-0001` | Accepted | 5 / 4 / 3 | 4 | 7 | 5 |
| `docz-adr-0002` | Accepted | 6 / 4 / 3 | 6 | 5 | 8 |
| `docz-adr-0003` | Accepted | 3 / 3 / 2 | 4 | 2 | 6 |
| `booty-adr-0001` | Proposed | 5 / 4 / 2 | 0 | 0 | 7 |
| `booty-adr-0002` | Proposed | 5 / 4 / 3 | 0 | 0 | 8 |
| `booty-adr-0003` | Accepted | 4 / 3 / 2 | 3 | 0 | 6 |
| `tempy-adr-0001` | Proposed | 4 / 2 / 1 | 3 | 0 | 2 |
| `tempy-adr-0002` | Proposed | 4 / 3 / 2 | 4 | 0 | 3 |

## The migration rule

A `.md` copy is the spans inference already found, written out as markers.
Nothing else: no heading is added, no prose is moved, no blank line is
inserted. `booty-adr-0002` keeps its alternatives as bold paragraphs with no
bullet in sight, so its migrated copy reports the same zero alternatives the
original does. A migration that tidied the document would make the pair test
prove nothing.

An ADR is the type that nests, so the marker order matters: the consequences
region ends on the same line its neutral list does, and the two closers land
together with the inner one written first.

```markdown
- Shared Temporal worker scaffolding … not before.
<!--docz:neutral:end-->
<!--docz:consequences:end-->
```

That rule makes these files the expected output of Phase 3's `InsertRegions`.

## What Validate reports on the corpus

Nothing. All eight documents validate clean, on both copies, and
`TestCorpusValidateCodeCoverage` pins that: each code below is recorded as
absent, with the reason, and the test fails if one starts firing.

| Code | Why the corpus never trips it |
|---|---|
| `adr.parse` | Every fixture has frontmatter and LF endings; a document the parser rejects is not an ADR corpus |
| `adr.decision.empty` | The four accepted fixtures all record their decision; the four proposed ones are exempt by the rule |
| `adr.consequences.empty` | All eight fill positive, negative, *and* neutral — the template's three headings are load-bearing habit |
| `adr.superseded.no-reference` | No ADR in the fleet carries the Superseded status at all |

No fixture is invented to close one of those gaps. `validate_test.go` already
has a passing and a failing document per code against `grammar.md`; a fixture
written to trip a rule would only prove that the rule fires on a fixture
written to trip it. What this corpus adds is the other half of the question —
what eight real ADRs actually trip — and the answer is worth recording.

## What the corpus taught us

Each of these is a fact about how people write ADRs, found by running the
parser over documents rather than over fixtures somebody wrote to pass.

- **Half the fleet's ADRs have a `## Status` section, and the model does not
  read it.** Six of the eight carry one; `status` is not a region kind, so the
  prose under it belongs to no field. Twice it disagrees with the frontmatter:
  `tempy-adr-0001` and `tempy-adr-0002` both say "Accepted" under the heading
  while their frontmatter says `status: Proposed`. Every rule that reads a
  status reads the frontmatter, which is the right source — it is the field
  `docz status set` writes — but it means an ADR its author considers accepted
  can hold an empty decision and earn no finding.
- **Alternatives written as bold paragraphs are not alternatives.**
  `booty-adr-0001` names five and `booty-adr-0002` names four, each as
  `**Cedar (cedar-policy/cedar-go).** AWS's open-source …` with no bullet and
  no heading. `kinds.Alternatives` reads level-3 sections or top-level bullets,
  so both report nil, and nine alternatives from a real decision record are not
  in the model. The same repo's third ADR uses bullets and reads fine, so this
  is a habit that changed rather than a repo that is wrong. It is also the one
  case in the corpus where a present region legitimately gives a zero field,
  which `TestCorpusFieldsFollowTheirRegions` logs by name instead of skipping.
- **A bolded option letter loses the option.** `docz-adr-0001` writes every
  recommended option as `- **a. (Recommended)** Keep the subpackage family`,
  and the option grammar matches a letter at the *start* of the bullet text.
  All seven of its questions therefore report the options they did not choose
  and drop the one they did — five come out `options=2 [b c]` and two
  `options=3 [b c d]`, while four of the seven resolve to `(a)`, the option
  that is missing. `docz-adr-0002` and `docz-adr-0003` write
  `- a. **Package doc comment…** … *(recommendation)*` and read correctly,
  marker included. Two spellings in one repo, two outcomes; the Resolved
  blockquote is what saves the reader, because it records the letter even when
  the option itself is gone.
- **`Recommended` is a marker, not a letter.** `*(recommendation)*` is read;
  `(Recommended)` inside bold is not. Across the corpus that means every
  option of `docz-adr-0002` and `docz-adr-0003` carries the flag and none of
  `docz-adr-0001`'s does — the same fleet convention, spelled two ways, one of
  which the grammar does not see.
- **A resolution without a parenthesised letter still resolves.**
  `docz-adr-0001` closes two of its questions with "none of the drafted
  options" and "moot", so `Choice` is empty and the note carries the answer.
  That is the documented behaviour, and it is the right one: an unparenthesised
  letter would read the "n" of "none" as the choice.
- **A resolution normally sits above the options, not below them.** Every
  resolved question in the corpus puts its blockquote directly under the
  heading. The reader scans the whole question span rather than the tail of it,
  which is the only reason this works.
- **Wrapping is the norm, not an edge case.** 76 of the 81 consequence bullets
  in the corpus wrap. A reader that took only a bullet's first line would
  truncate almost every consequence in the fleet.
- **A consequence keeps its bold lead-in.** `**Familiar trust model.** JWTs are
  well-understood…` is one `Item.Text`, markdown and all, because `kinds.Item`
  reports the bullet verbatim. A consumer that wants "title, then reasoning"
  splits it itself — the same split `kinds.Alternatives` does for a region
  where the shape is declared.
- **Prose inside a consequence section is not a consequence.**
  `docz-adr-0001` closes its Negative list with an italic paragraph explaining
  which negatives the revision dissolved. It is inside the region and it is not
  a bullet, so it is not in the model. A list field holds the list.
- **The decision swallows its subsections, and that is the point.**
  `booty-adr-0003`'s "Layering rules" and "Split triggers",
  `docz-adr-0002`'s "Supporting Data", "What this supersedes in ADR-0001", and
  "The compatibility contract", `docz-adr-0003`'s fenced Go and two mermaid
  diagrams — all of it is `Doc.Decision`. Cutting a subsection out would drop
  the evidence from the one field a reader asks for the decision.
- **A decision can precede its context.** `booty-adr-0002` puts `## Decision`
  first, then `## Context`, then `## Decision rationale`. Regions are located
  by heading and not by order, so both read correctly and the model has no
  opinion about which came first.
- **A section the template does not name is dropped whole.**
  `booty-adr-0002`'s `## Decision rationale` — the actual argument for its
  decision, five paragraphs of it — and its `## Implementation notes` belong to
  no kind, so a consumer reading `Doc.Decision` gets the ruling without the
  reasoning. `docz-adr-0001`'s `## Corner Cases` (nine numbered items) goes the
  same way. This is the cost of a fixed schema, and the `api:` block's
  additional docs is where a document that needs its own sections goes.
- **A `## Decisions` table is resolved and then read by nothing.**
  `docz-adr-0001` closes with a nine-row table recording how each question was
  settled, headed `#` / `Question` / `Choice` / `Notes`. `decisions` is in the
  heading table, so the region is found and the surrounding sections are not
  mis-nested — but `adr.Doc` has no field for it and `Parse`'s switch has no
  arm, so the table is dropped. `kinds.Decisions` exists and reads exactly this
  shape, `Choice` column included.
- **Supersession is recorded in prose at least as often as in a list.**
  `docz-adr-0002` names ADR-0001 in its summary, in its context, and again in
  its references, which is why `adr.superseded.no-reference` reads all three.
  It is also why the corpus has no Superseded document: ADR-0001 is superseded
  *in part* and keeps the status Accepted, which is how this fleet records a
  decision that is still half in force.
- **`ADR-NNNN` is not an ADR id.** sdk-booty-sh cites its siblings as
  "Sibling: ADR-NNNN Use OPA for scope policy evaluation" — a placeholder for
  an id that did not exist yet. The rule matches `ADR-<digits>`, so those
  pointers do not count, and one of those documents flipped to Superseded would
  earn the finding despite naming its successor in plain English.
- **A reference is often not a link.** All five of `docz-adr-0001`'s and all of
  tempy's references are prose, so `URL` is empty and `Text` is the whole
  bullet. Where there are two links in one bullet — `booty-adr-0003`'s
  multi-module monorepo precedents, `docz-adr-0002`'s two issue numbers — `URL`
  is the first and `Text` keeps both.
- **The ToC is bullets in no region.** Every document in the fleet opens with a
  generated `<!--toc:start-->` list. It sits above the first region, so nothing
  reads it, and `kinds.ResolveRegions` deliberately does not count the toc and
  index markers as markers — otherwise no document in the fleet would ever be
  inferred, since every one of them has a ToC.
