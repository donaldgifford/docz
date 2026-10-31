# Layout

A page's tabs come from the document's **regions**, never from its
heading text, so a document whose headings differ from its template keeps
its layout. `docz inspect <id> --format json` lists the regions in
document order (`kind`, `start`, `end`, `depth`), with `inferred: true`
when the document carries no markers and docz found them from headings.

## The rules

1. Walk the depth-0 regions in document order. Each one goes to the tab
   the table below names, with the component the table names.
2. A tab appears where its first region appears; regions that share a tab
   are placed in document order within it.
3. A kind the table does not list becomes a tab of its own, labelled with
   the region's first heading. Nothing is dropped.
4. A region's children (depth 1 and deeper) are placed by their parent's
   row: `positive` inside `consequences`, `tasks` inside `phase`, `steps`
   inside `procedure`. A child whose kind has a row of its own that its
   parent's row does not mention (`open-questions` inside
   `recommendation`) gets that row's tab instead.
5. Text outside every region is not dropped either:
   - an H2 section outside every region becomes a tab of its own,
     labelled with the heading, like rule 3;
   - a heading that only introduces the regions under it (`## Goals and
     Non-Goals` above the `goals` and `non-goals` regions, `## Procedures`
     above the procedures) is not repeated: its anchor goes on the first
     element after the panel title, and any paragraph under it opens that
     tab;
   - a thematic break (`---`) between regions is dropped.
6. The `toc` region is dropped. The tabs and each panel's heading list
   replace it.
7. A document with no regions at all, marked or inferred, gets one tab per
   H2, labelled with the heading.
8. A region left as a template placeholder becomes `p.empty`, "Not
   written yet.", in its tab, rather than an empty tab.
9. A region's markdown is rendered in full inside its component, except
   what the component takes over: the heading, the header summary's
   paragraph, and the `**Answer:**` line, which becomes the verdict.

## Headings and anchors

Every heading's anchor (from `docz inspect`) must land on the page, so a
`#slug` link anywhere still works:

- The panel title names the **tab**, and carries the anchor of the first
  region's heading when that heading's text is the tab label, compared
  without case.
- Otherwise each region's own heading is written as an `h3` with its
  anchor, in document order: a tab holding `summary` and `context` shows
  both headings.
- A heading that becomes a card's label puts its anchor on the card, and
  an open question's anchor goes on its `details`.
- Every heading in a region moves down by the same amount as the
  region's own heading, so the panel title is the only `h2` and the
  nesting is kept (`## Detailed Design` → `h3`, its `###`s → `h4`).
- A phase's `#### Tasks` and `#### Success Criteria` anchors go on its
  `ul.tasks` and its criteria callout.
- Take a heading's text from the markdown line, not from `inspect`'s
  `text`, which drops inline code.
- The heading list counts the panel's `h3`s only and lists them; card
  labels are not counted.

## The header summary

The first paragraph of the first `summary`, `overview`, `objective`,
`question`, `hypothesis`, or `context` region is the header summary. It is
**not repeated** in the panel; the rest of the region is. A region with no
paragraph before its first list, table, or heading gives no summary.

## Per-kind details

- **Alternatives.** One card per H3; with no H3s, one per bold-led
  paragraph or list item, the bold text as the label. `card accent` only
  for an alternative the document says was chosen; rejected and deferred
  ones are plain cards.
- **Decision.** The callout holds the decision's first block, a
  paragraph or a list; the rest follows as text.
- **Risks.** A likelihood or impact cell is tinted by its first word:
  High or Critical is `high`, Medium `medium`, Low `low`; anything else
  stays plain.
- **Success criteria.** A bold group label (`**Phase 1:**`) stays as a
  paragraph, followed by the cards for its items.
- **Procedures.** Each procedure's heading as an `h3` with its anchor,
  its intro paragraph, then `Steps`, `Verification`, and `Rollback` as
  `h4`s. A verification item holding a code block keeps the block inside
  its `li`, after the `span.text`.
- **Scenarios.** Closed by default. The summary is the scenario's name
  without the `Scenario:` prefix. Symptom, alert, and likely cause are a
  `dl` (`dt` the label, `dd` the text).
- **Last verified.** The card holds the table and the notes.
- **Open questions.** Options come from the markdown (each `- a.`
  item); `recommended` comes from `docz inspect`, whatever spelling the
  document used. Unresolved questions are open, resolved ones closed.
  Each resolution or revision paragraph (`> **Resolved …**`,
  `> **Revised …**`) is its own `p.resolution`, in document order.

## Region kinds

| Region kinds | Tab | Component |
| --- | --- | --- |
| `summary`, `overview`, `objective`, `question`, `hypothesis`, `context` | Overview | Text; the first paragraph of the first is the header summary instead |
| `goals`, `non-goals` | Overview | `grid g2`: `card ok` Goals, `card neutral` Non-goals |
| `scope` with `in-scope`, `out-of-scope` | Overview | `grid g2`: `card ok` In scope, `card bad` Out of scope |
| `when` (runbook) | Overview | Text |
| `prerequisites` (runbook) | Overview | A task list, every item `open` |
| `last-verified` (runbook) | Overview | A `card` labelled Last verified: date, PR, commit, verified by |
| `problem` | Problem | Text |
| `background` | Background | Text |
| `proposal` | Proposal | Text |
| `detailed-design` | Design | Text; its H3s in the heading list |
| `decision` (ADR) | Decision | A `callout` holding the decision's first block, then the rest as text |
| `consequences` with `positive`, `negative`, `neutral` | Consequences | `grid g3`: `card ok`, `card bad`, `card neutral` |
| `alternatives` | Alternatives | One `card` per alternative, the chosen one `card accent` |
| `risks` | Risks | The table, likelihood and impact cells tinted `high`/`medium`/`low` |
| `criteria` at depth 0 (RFC) | Success | One `card ok` per criterion |
| `api-changes` | Interfaces | Text; each contract in a code block |
| `data-model` | Data model | Text |
| `phase` with `tasks`, `criteria` (IMPL) | Phases | The filter bar once, then one `card phase` per phase: tasks, progress, deferred and skipped pills, criteria in a `callout ok` |
| `file-changes` | Changes | The table |
| `testing` | Testing | Text; a checklist as a task list |
| `rollout` | Rollout | Text |
| `dependencies` | Dependencies | Text |
| `approach`, `environment` | Approach | The steps, then the environment table |
| `findings` | Findings | One `card` per finding (each H3) |
| `conclusion`, `recommendation` | Conclusion | The `**Answer:**` as a `verdict`, then the rest, then the recommendation |
| `procedure` with `steps`, `verification`, `rollback` (runbook) | Procedures | Per procedure an H3, then `ol.steps`, verification as a task list, rollback as `ol.steps` in a `callout bad` |
| `scenario` with `steps` (runbook) | Scenarios | One `details.expand` per scenario: symptom, alert, likely cause as a definition list, then `ol.steps` |
| `escalation` (runbook) | Escalation | The table |
| `open-questions` | Open questions | One `details.expand` per question |
| `decisions` | Decisions | The table |
| `references` | References | The list, links resolved |
| A `docz-flow`, `docz-stepper`, or `docz-scorecard` block anywhere | Where the block is | The widget (`widgets.md`) |

A Decisions or Open questions tab appears in any type whose document
carries the region, which hand-edited IMPLs and investigations often do.

## Tabs per type

What each built-in's template gives, in order:

| Type | Tabs |
| --- | --- |
| RFC | Overview, Problem, Proposal, Alternatives, Risks, Success, References |
| ADR | Overview, Decision, Consequences, Alternatives, References |
| DESIGN | Overview, Background, Design, Interfaces, Data model, Testing, Rollout, Open questions, References |
| IMPL | Overview, Phases, Changes, Testing, Dependencies, References |
| Investigation | Overview, Approach, Findings, Conclusion, References |
| Runbook | Overview, Procedures, Scenarios, Escalation, References |

A custom type follows the same table: a custom type carrying a
built-in's regions gets that built-in's components, because the layout
reads region kinds, never type names.

## Links

- A `#slug` link stays as it is; the runtime opens the tab that holds the
  heading.
- A relative link to another document: a link to its page when you are
  writing that page in the same run, else its GitHub blob URL (the
  repository's `origin` and default branch, `main` when `origin/HEAD` is
  not set).
- A relative link to a file that does not exist, or in a repository with
  no GitHub `origin`, becomes its text alone and is listed in your report.
- An image: a relative image under 1 MB inlined as a `data:` URL, else its
  blob URL. Nothing is fetched from the network when the page opens.
