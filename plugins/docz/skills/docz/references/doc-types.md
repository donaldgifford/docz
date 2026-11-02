# Doc Types Reference

Detailed guidance on when to use each docz document type and what each section should contain.

## Choosing the Right Type

Use this decision tree:

1. **Is this a broad proposal that needs buy-in before work starts?** → RFC
2. **Has a decision already been made and needs to be recorded?** → ADR
3. **Is this working out the implementation details of an accepted decision?** → DESIGN
4. **Is this a phased execution plan with tasks to check off?** → IMPL
5. **Is this mid-level planning that bridges an RFC and an IMPL?** → PLAN (opt-in; see below)
6. **Is this time-boxed research to answer a question before deciding?** → INV

## RFC — Request for Comments

**Purpose**: High-level proposals for significant changes that require team alignment before work begins.

**When to write an RFC**:
- Introducing a new technology or platform
- Changing a major architectural pattern
- Significant API contract changes
- Cross-team initiatives
- Anything that will affect how multiple people work

**Key sections to fill** (these are the template's actual sections — run `docz template show rfc` to see them):
- **Summary** — the proposal in 2–3 sentences
- **Problem Statement** — the current pain, its impact, and why it matters now, with a **Supporting Data** subsection for the evidence
- **Proposed Solution** — what you want to do at a high level
- **Alternatives Considered** — other approaches evaluated and why they were rejected
- **Risks and Mitigations** — a table of risk, impact, likelihood, mitigation
- **Success Criteria** — how you will measure whether this achieved its goals

**Status flow**: Draft → Proposed → Accepted | Rejected | Superseded

**Tips**:
- Keep RFCs decision-focused, not implementation-focused — save the details for a DESIGN doc
- Circulate as Draft for informal feedback before moving to Proposed
- An RFC can spawn one or more ADRs, DESIGN docs, and IMPLs once accepted

---

## ADR — Architecture Decision Record

**Purpose**: Lightweight, permanent records of architectural decisions and their context.

**When to write an ADR**:
- Choosing a database, queue, or infrastructure component
- Settling a recurring technical debate
- Documenting a trade-off the team has decided to live with
- Capturing a decision made in a meeting so it doesn't get lost

**Key sections to fill** (run `docz template show adr` to see them):
- **Summary** — the decision in 2–3 sentences
- **Context** — the situation and forces at play when the decision was made
- **Decision** — the specific choice made, stated clearly, with a **Supporting Data** subsection for the evidence behind it
- **Consequences** — split into **Positive**, **Negative**, and **Neutral**
- **Alternatives Considered** — what else was evaluated

**Status flow**: Proposed → Accepted → Deprecated | Superseded

**Tips**:
- ADRs are immutable once Accepted — if the decision changes, create a new ADR that supersedes the old one
- Keep them short: 1–2 pages is ideal
- Write them close to when the decision is made, not months later
- Link to the RFC or design doc that led to the decision

---

## DESIGN — Design Document

**Purpose**: Work out implementation details after a decision has been made.

**When to write a DESIGN doc**:
- You have an accepted RFC or ADR and need to specify how to build it
- A feature is complex enough that you want to think through the interfaces before coding
- You need sign-off on an approach before implementation starts

**Key sections to fill** (run `docz template show design` to see them):
- **Overview** — what is being designed and why, in 2–3 sentences
- **Goals and Non-Goals** — explicit **Goals** and **Non-Goals** subsections; the non-goals do most of the scoping work
- **Background** — context, prior art, prerequisites
- **Detailed Design** — the core: architecture, component interactions, data flow
- **API / Interface Changes** — public API, CLI, and config surface changes
- **Data Model** — schema and storage considerations
- **Testing Strategy** — unit, integration, end-to-end approach
- **Migration / Rollout Plan** — how to get from here to there safely
- **Open Questions** — unresolved decisions that reviewers should weigh in on

**Status flow**: Draft → In Review → Approved → Implemented | Abandoned

**Tips**:
- Update the DESIGN doc as implementation reveals surprises — it's a living document until Implemented
- Link to the IMPL doc(s) that execute the design

---

## IMPL — Implementation Document

**Purpose**: Phase-based execution tracking with checkboxes, bridging planning and actual work.

**When to write an IMPL doc**:
- Breaking a large feature into trackable phases
- Coordinating multi-person implementation work
- Tracking migration progress (e.g., "Phase 1: schema changes, Phase 2: backfill, Phase 3: cutover")

**Key sections to fill** (run `docz template show impl` to see them):
- **Objective** — what is being implemented, with an **Implements:** line linking the RFC/DESIGN/PLAN
- **Scope** — explicit **In Scope** and **Out of Scope** subsections
- **Implementation Phases** — numbered phases, each with its own **Tasks** checkbox list and **Success Criteria**
- **File Changes** — a table of file, action, description
- **Testing Plan** — the checks that prove the work landed
- **Dependencies** — external dependencies, blocking work, prerequisites

**Status flow**: Draft → In Progress → Completed | Paused | Cancelled

**Tips**:
- One IMPL per coherent unit of work; large projects may have multiple IMPLs per phase
- Keep phase tasks small enough to check off within a day or two
- Update status promptly — stale IMPLs are worse than none

---

## PLAN — Planning Document

**Enabled**: opt-in. Since docz v1.1.1, `types.plan.enabled` defaults to `false` — `docz init` does not scaffold `docs/plan/`, and `docz create plan` fails with `document type "plan" is disabled in configuration`. Set `types.plan.enabled: true` in `.docz.yaml` to use it.

**Purpose**: Mid-level planning that sits between a high-level RFC and a detailed IMPL.

**When to write a PLAN doc**:
- Quarterly or sprint planning artifacts
- Feature roadmaps that aren't yet at implementation detail
- Organizing work across multiple IMPLs

**Key sections to fill** (run `docz template show plan` to see them):
- **Goal** — the end state, ideally with a concrete example of success
- **Context** — why this work is needed now, and what breaks without it
- **Approach** — how to get there, broken into named **Part N** subsections
- **Components** — a table of the major pieces and their purpose
- **File Changes** — a table of file, action, description
- **Verification** — commands to run and behaviours to check when complete
- **Dependencies** — prerequisite work, tools, or blocking issues
- **Open Questions** — unresolved decisions, moved out as they are answered

**Status flow**: Draft → In Progress → Completed | Cancelled

---

## INV — Investigation

**Purpose**: Time-boxed research and validation experiments to answer a question before deciding.

**When to write an INV**:
- Evaluating two or more technical options before writing an RFC
- Prototyping to validate feasibility
- Researching a poorly understood problem space

**Key sections to fill** (run `docz template show investigation` to see them):
- **Question** — the specific question, phrased to have a clear yes/no or concrete answer
- **Hypothesis** — what you expect to find and why, stated before you start
- **Context** — what triggered this, with a **Triggered by:** link to the parent doc or issue
- **Approach** — the numbered steps or experiments, concrete enough to replicate
- **Environment** — versions and configuration relevant to reproducibility
- **Findings** — what you actually observed, as numbered **Observation** subsections
- **Conclusion** — a direct **Answer:** to the question (Yes / No / Inconclusive)
- **Recommendation** — what should happen next

**Status flow**: Open → In Progress → Concluded | Inconclusive | Abandoned

**Tips**:
- State the question before starting — vague investigations produce vague results
- Timebox aggressively; if you don't have a recommendation by the deadline, write one anyway with uncertainty noted
- An INV should feed directly into an RFC or ADR
