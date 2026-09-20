# pkg/rfc fixtures

Two files per document, plus a fact file:

| File | What it is |
|---|---|
| `<name>.orig.md` | A verbatim snapshot of a real RFC, byte for byte as written |
| `<name>.md` | The same document with canonical region markers added and nothing else changed |
| `<name>.golden.txt` | The facts `Parse` reads out of the migrated copy |

Regenerate the last two with `go test ./pkg/rfc/... -update`. Never edit them
by hand. Never edit a `.orig.md` either: it is a snapshot, and its whole value
is that it is not maintained alongside the parser.

`grammar.md` is not part of the corpus. It is the hand-written fixture the
grammar and validate tests read, and it is deliberately *not* a `.orig.md`: a
fixture somebody wrote to exercise every rule proves the rules work on itself.

Snapshots rather than reads from `docs/` because a fixture that followed this
repo's own documents would rewrite its own expectations every time somebody
edited a proposal. These exist to pin the grammar against documents as they
were actually written, by people who had never heard of the grammar.

## Where each came from

| Fixture | Source | What it is here for |
|---|---|---|
| `booty-rfc-0001` | sdk-booty-sh RFC 0001 (Agent Dev Kit) | 540 lines; alternatives as bold paragraphs, criteria grouped under bold phase labels, a 224-line `## Design` section no region covers |
| `booty-rfc-0002` | sdk-booty-sh RFC 0002 (AWS Access Triage Slack Bot) | A reference whose link sits on a continuation line, and one that cites three links in one bullet |
| `booty-rfc-0003` | sdk-booty-sh RFC 0003 (Agent Capability Tokens) | 673 lines; a fenced ` ```markdown ` block holding a column-0 `##` heading, 19 criteria, 14 references |
| `template-rendered` | the embedded RFC template, rendered | Every region present, and every field but one zero — the state a freshly created RFC is in |

`template-rendered.orig.md` is the embedded template rendered the way
`docz create` renders it, with every `<!--docz:` line then stripped. It is
written out and committed rather than generated in `TestMain`, because a
snapshot that regenerates itself is not a snapshot: its value is that
migrating it back has to find the regions the template marks, and a fixture
derived from the template at test time could not fail that.

## The migration rule

A `.md` copy is the spans inference already found, written out as markers.
Nothing else: no heading is added, no prose is moved, no blank line is
inserted. All three booty RFCs keep their alternatives as bold-lead-in
paragraphs, so their migrated copies report the same zero alternatives the
originals do. A migration that tidied the document would make the pair test
prove nothing.

That rule makes these files the expected output of Phase 3's `InsertRegions`.

## What the corpus taught us

Each of these is a fact about how people write RFCs, found by running the
parser over documents rather than over fixtures somebody wrote to pass.

- **The fleet does not write alternatives as a list.** All 18 alternatives in
  the corpus — six in RFC 0001, five in 0002, seven in 0003 — are written as
  `**Title.** why it was rejected` *paragraphs*, with a blank line between
  them and no bullet in sight. `kinds.Alternatives` reads bullets or level-3
  headings, so `Alternatives` is nil for every real RFC here and
  `rfc.alternatives.empty` fires on all four fixtures. This is the corpus's
  headline finding and the only one that is arguably a defect rather than a
  documented loss: a reader looking at RFC 0003 sees seven carefully argued
  alternatives, and the typed model sees none. The repo's ADRs write theirs as
  bullets, which is where the grammar came from, and the RFC template says
  nothing either way.
- **A freshly created RFC's problem statement is a heading.** The template's
  problem region holds a guidance comment, a `### Supporting Data` heading,
  and a second comment. Comments are stripped and the subsection is
  deliberately part of the statement, so `Problem` comes back as the literal
  string `"### Supporting Data"` — non-empty, and carrying no information.
  Every other field of the rendered template is correctly zero. No real RFC in
  the corpus has a Supporting Data subsection at all: all three write their
  evidence inline in the problem prose, so the rule that keeps the subsection
  is exercised only by the document where it produces nothing but a heading.
- **A "brief 2-3 sentence summary" is 26 lines.** RFC 0001's summary region runs
  to three paragraphs and closes with a blockquote amendment added two months
  later, linking the ADR that superseded part of it. `Summary` is the region's
  body verbatim, which is right — the amendment is part of what the summary now
  says — but a consumer that renders `Summary` as a one-line teaser gets a wall
  of text with two relative links in it.
- **Criteria lose their phase grouping, and that is the type's decision.** All
  three RFCs file their success criteria under bold `**Phase 1:**` /
  `**Org-level outcomes:**` paragraph labels. Those are not list items, so the
  11, 14, and 19 criteria come back as one flat list per document with no
  attribution. An RFC has one set of criteria for the whole proposal by design
  (unlike an IMPL, which has one set per phase), so there is nowhere on `Doc`
  to put the grouping — but it is information the document has and the model
  does not.
- **Not one of the 44 criteria is executable.** The corpus writes measurable
  outcomes — "≥50% of threads resolved self-serve in <60 seconds median" — not
  commands. Backtick spans do appear in them (`adk new agent`, `scopes.fs.write`,
  `#aws-access-support`, `policies/`) but always mid-sentence, which is exactly
  why `Executable` requires a *leading* span: reading a command from the middle
  of a sentence would offer to run `#aws-access-support`.
- **Wrapping is the norm.** 21 of the 44 criteria and 20 of the 31 references
  are longer than the documents' 80-column wrap, so they necessarily span more
  than one source line. A reader that took only a bullet's first line would
  truncate half the corpus.
- **A reference's link is often not on the reference's line.** RFC 0002's first
  reference is `- Upstream:` followed by
  `[tenable/access-undenied-aws](https://github.com/tenable/access-undenied-aws)`
  on the next line. The URL is found because folding happens before the link
  scan; a first-line-only reader would report an empty URL and validate would
  flag a reference that has a perfectly good link.
- **Most references have no link at all, and that is not a defect.** Only 12 of
  31 carry a URL. The rest are prose pointers — "Forthcoming: Agentic Go
  Framework RFC", "Internal: Agent platform RFC + DESIGN suite" — to documents
  that do not exist yet or are not on the web. An empty `URL` is the common
  case for an RFC written before its siblings.
- **A reference bullet may cite several links; `URL` is the first.** RFC 0002's
  last reference lists Service Control Policies, Permissions Boundaries, and
  DecodeAuthorizationMessage in one bullet, and reports only the first. `URL`
  is a lossy summary of a bullet that cites three things, which is worth
  knowing before building a link checker on it.
- **The risks table is the one section the fleet fills completely.** All 28 rows
  across the three RFCs fill every one of the four columns, which is why the
  corpus never trips `rfc.risks.no-mitigation`. The template's single empty row
  is dropped rather than reported as a blank risk, so the rendered template
  reports no risks rather than one nameless one.
- **Two whole sections of every real RFC belong to no region.** The template
  ships neither `## Design` nor `## Implementation Phases`, and all three booty
  RFCs have both — 224 lines of RFC 0001 is its Design section alone, complete
  with its own comparison table. Inference correctly stops the proposal region
  at `## Design` instead of letting it run to the end of the document, and
  nothing reads what follows. An RFC's design and its phases are not part of
  the typed model.
- **No RFC in the corpus has an Open Questions or Decisions section.** Both
  kinds are read anyway, from `kinds.sharedDefaults`, which is why the
  field-presence invariant exercises its "region absent" branch on every
  fixture and why `rfc.status.open-question` is untested by real documents.
- **Migrating the rendered template finds its own regions, but not its own
  bytes.** All seven spans come back exactly where the template marks them; the
  closers land one line higher, on each region's last content line, because
  inference trims trailing blank lines. The spans round-trip, the marker lines
  do not.
- **A real document does put a column-0 `##` inside a fence.** RFC 0003 embeds a
  ` ```markdown ` block whose first line is `## aws.iam.simulate`, one of six
  fenced blocks in its Design section. It is skipped, so it neither opens a
  region nor is mistaken for the end of one. Checked rather than assumed — and
  in *this* document nothing depended on it, because the nearest region
  boundary is the `## Design` heading 276 lines above. The mechanism matters
  for the next RFC that shows a fenced `## Risks and Mitigations` example,
  which without the fence rule would open a phantom risks region in the middle
  of its design.

### Which `Validate` codes the corpus reaches

Exercised: `rfc.alternatives.empty`, on all four fixtures, for the reason
above.

Not exercised, and recorded in `codesAbsentFromCorpus` rather than
manufactured:

- `rfc.parse` — every fixture is a real markdown document with frontmatter and
  LF endings, and `Parse` rejects only those two things.
- `rfc.risks.no-mitigation` — all 28 rows fill every column, and the template's
  one row is wholly empty and dropped.
- `rfc.status.open-question` — every fixture is `status: Draft`, and none has an
  Open Questions section.

Each of the three is covered by `validate_test.go`, which reaches it by editing
`grammar.md`. Inventing a corpus fixture to hit a code would make the corpus
agree with the rules by construction, which is the one thing it is here to
avoid.
