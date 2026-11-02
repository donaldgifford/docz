---
name: docz:create
description: >
  Create a new docz document using the CLI — ADR, RFC, design doc,
  investigation, implementation plan, or planning doc.
when_to_use: >
  "create an ADR for", "write up an RFC about", "start an investigation
  into", "document this decision", "write a proposal for", "create a
  design doc", "track implementation of", "open an investigation".
allowed-tools: Bash(docz *)
argument-hint: "<type> <title>"
---

# Docz Create

Create a new document with `docz create <type> <title>`.

## Type Resolution

Resolve natural language to the correct CLI type argument:

| User says | CLI type |
|-----------|----------|
| ADR, architecture decision, decision record | `adr` |
| RFC, proposal, request for comments | `rfc` |
| design doc, design document, design spec | `design` |
| implementation, impl, implementation plan | `impl` |
| plan, planning doc | `plan` — **opt-in**, see below |
| investigation, spike, research, INV | `inv` |
| anything else | possibly a **custom type** — check `docz config` first |

**Unrecognized type words may be custom types.** `.docz.yaml` can define any type with its own `id_prefix`, `aliases`, and `statuses`. Before treating a word as a typo, run `docz config` and check the `types:` block — resolution is case-insensitive by canonical name, alias, or `id_prefix`. Custom types need a template at `<docs_dir>/templates/<name>.md` or creation fails; see `../docz/references/config.md`.

**PLAN is opt-in.** Since docz v1.1.1 `types.plan.enabled` defaults to `false`. If `docz create plan` returns `document type "plan" is disabled in configuration`, explain that PLAN is opt-in and offer the one-line fix (`types.plan.enabled: true` in `.docz.yaml`) — do not edit the config without being asked. IMPL or RFC is often the better fit anyway.

## Process

1. **Resolve the type** from user's description using the table above. If ambiguous, ask one clarifying question: "Is this a decision that's already been made (ADR) or a proposal that needs sign-off first (RFC)?"

2. **Confirm the title** with the user before writing. Suggest a concise, imperative title (e.g. "Use PostgreSQL for primary storage" not "PostgreSQL decision").

3. **Create the document** with the CLI:
   ```bash
   docz create <type> "<title>"
   # Optional flags:
   # --author "Name"    override author (default: git config user.name)
   # --status <status>  set initial status
   # --no-update        skip README regeneration (useful in batch scripts)
   ```

   With docz 2.2.0 or later, add `--format json`, which prints one JSON object carrying the new document's `id`, `type`, `path`, and `title`, so read the path from it rather than from the `Created` line (and the docz-review plugin's hook reads the same object).

   If `docz` is not on `PATH`, stop and tell the user to install it (≥ v1.2.0, e.g. `mise use github:donaldgifford/docz@latest`). There is no offline path — the CLI owns the templates and the ID numbering.

4. **Show the created file path** and open it for editing.

5. **Guide section completion** — after creating, prompt for the sections that carry the document. These are the template's real section names; run `docz template show <type>` for the full set:
   - **ADR**: Summary, Context, Decision (+ Supporting Data), Consequences (Positive/Negative/Neutral)
   - **RFC**: Summary, Problem Statement (+ Supporting Data), Proposed Solution, Alternatives Considered, Risks and Mitigations, Success Criteria
   - **DESIGN**: Overview, Goals and Non-Goals, Detailed Design, API / Interface Changes, Testing Strategy, Open Questions
   - **IMPL**: Objective (+ Implements), Scope, Implementation Phases (Tasks + Success Criteria per phase), Testing Plan
   - **PLAN**: Goal, Context, Approach (Part N), Components, Verification, Open Questions
   - **INV**: Question, Hypothesis, Context (+ Triggered by), Approach, Findings, Conclusion, Recommendation

## Reference

Read `../docz/references/doc-types.md` for detailed guidance on what belongs in each section.
