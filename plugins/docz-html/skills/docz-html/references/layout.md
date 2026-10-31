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
   inside `procedure`.
5. The `toc` region is dropped. The tabs and each panel's heading list
   replace it.
6. A document with no regions at all, marked or inferred, gets one tab per
   H2, labelled with the heading.
7. A region left as a template placeholder becomes `p.empty`, "Not
   written yet.", in its tab, rather than an empty tab.
8. A region's markdown is rendered in full inside its component, except
   what the component itself takes over (the first heading becomes the
   card label or the panel title, the `**Answer:**` line becomes the
   verdict).

## Region kinds

| Region kinds | Tab | Component |
| --- | --- | --- |
| `summary`, `overview`, `objective`, `question`, `hypothesis`, `context` | Overview | The first of these is the header summary as well; each is text in the panel |
| `goals`, `non-goals` | Overview | `grid g2`: `card ok` Goals, `card neutral` Non-goals |
| `scope` with `in-scope`, `out-of-scope` | Overview | `grid g2`: `card ok` In scope, `card bad` Out of scope |
| `when` (runbook) | Overview | Text |
| `prerequisites` (runbook) | Overview | A task list, every item `open` |
| `last-verified` (runbook) | Overview | A `card` labelled Last verified: date, PR, commit, verified by |
| `problem` | Problem | Text |
| `background` | Background | Text |
| `proposal` | Proposal | Text |
| `detailed-design` | Design | Text; its H3s in the heading list |
| `decision` (ADR) | Decision | A `callout` holding the decision's first paragraph, then the rest as text |
| `consequences` with `positive`, `negative`, `neutral` | Consequences | `grid g3`: `card ok`, `card bad`, `card neutral` |
| `alternatives` | Alternatives | One `card` per alternative (each H3), the chosen one `card accent` |
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
  repository's `origin` and default branch).
- An image: a relative image under 1 MB inlined as a `data:` URL, else its
  blob URL. Nothing is fetched from the network when the page opens.
