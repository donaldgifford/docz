# kinds fixtures

Every fixture under `openquestions/`, `references/`, `decisions/`,
`criteria/`, and `alternatives/` is a **verbatim cut of one section of a real
document in this repository**: the section's heading on line 1, then its
content, which is exactly what a region yields to a reader.

They are verbatim on purpose. A reader tested only against prose written to
suit it proves nothing, and every tolerance in this package exists because a
document in the corpus needed it. Do not reflow, re-wrap, or tidy a fixture.
If a fixture looks wrong, the reader is what should change.

The `golden/*.golden.txt` files record what each reader makes of its fixture.
Regenerate them with:

```bash
go test ./pkg/doczcore/kinds/... -update
```

## What each fixture is evidence of

| Fixture | Source | What it pins |
| ------- | ------ | ------------ |
| `openquestions/adr-0002.md` | ADR-0002 | A qualified recommendation marker (`*(recommendation, revised …)*`), a bold span left open across two lines, a resolution preceded by prose and by a five-row table |
| `openquestions/adr-0003.md` | ADR-0003 | A resolution whose choice is not the recommendation |
| `openquestions/design-0014.md` | DESIGN-0014 | A resolution with no letter at all (`superseded by DESIGN-0015`), an amendment in a second paragraph of the same blockquote, a section preamble and a table that belong to no question |
| `openquestions/design-0015.md` | DESIGN-0015 | Two dates and two letters in one resolution, marker-shaped text inside code spans |
| `references/linked.md` | DESIGN-0014 | Several links in one bullet, and twelve of thirteen bullets wrapping |
| `references/unlinked.md` | INV-0002 | Thirteen bullets with no link at all, which is what the empty `URL` reports |
| `references/wrapped.md` | INV-0009 | A fifteen-line bullet whose every link is on a continuation line |
| `decisions/adr-0001.md` | ADR-0001 | A four-column table headed `Choice` and `Notes`, with its em-dash rows interleaved rather than appended |
| `decisions/impl-0018.md` | IMPL-0018 | The three-column `# / Question / Resolution` shape |
| `decisions/table-inside-open-questions.md` | DESIGN-0015 | A decisions table buried mid-region under 140 lines of subsections, which is where the corpus actually puts it |
| `criteria/rfc-position.md` | the parity suite's RFC fixture | The kind at the top level. No RFC has ever been committed under `docs/`, so this is the only RFC-position criteria writing in the repository |
| `criteria/impl-position.md` | IMPL-0018 Phase 1 | The kind inside a phase: nine bullets, six opening with a command, three with a backtick span mid-line that is not one |
| `criteria/checkboxes.md` | IMPL-0009 | Criteria written as a checklist, with continuations indented six spaces under the checkbox |
| `criteria/wrapped.md` | IMPL-0017 Phase 1 | Wrapped criteria containing `≥` and em dashes |
| `alternatives/bullets.md` | ADR-0002 | Six lettered bullets whose label sits **inside** the bold, which is how every ADR in this repo writes them |

## Shapes the corpus does not contain

Named here so the next reader knows the coverage is a fact about the corpus
rather than an oversight:

- **An `## Alternatives Considered` section written as level-3 headings.** All
  three real ones are lettered bullet lists. `Alternatives` reads the heading
  shape too, covered by a unit test, and prefers it — see the INV-0003
  evidence in `alternative.go`.
- **Alternatives as plain unlabelled bullets.**
- **An option letter that is not a single letter, a question numbered out of
  order, or a question with no options.** Unit tests and the fuzz targets
  cover those.

## `fuzz/`

Go writes a failing input here when a fuzz target finds one, and it is
committed as a regression seed. Each of the three present is a bug this
package had:

| Input | Bug |
| ----- | --- |
| `FuzzCriteria/20fa3171d551b3b6` | An empty backtick span made a criterion executable with no command to run |
| `FuzzReferences/a9929e473028d9ec` | A link target containing a tab was reported as a URL |
| `FuzzOpenQuestions/fe6497c8ee33383b` | A heading numbered `00` |
