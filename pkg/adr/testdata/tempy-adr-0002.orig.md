---
id: ADR-0002
title: "Loop control belongs to the workflow, not the prompt"
status: Proposed
author: Donald Gifford
created: 2026-09-12
---

<!-- markdownlint-disable-file MD025 MD041 -->

# ADR-0002: Loop control belongs to the workflow, not the prompt

<!--toc:start-->
- [Status](#status)
- [Context](#context)
- [Decision](#decision)
- [Consequences](#consequences)
  - [Positive](#positive)
  - [Negative](#negative)
  - [Neutral](#neutral)
- [Alternatives Considered](#alternatives-considered)
- [References](#references)
<!--toc:end-->

## Status

Accepted

## Context

IMPL docs are executed today by a single donald-loop (Ralph loop) invocation
whose prompt carries both the agent's job and the loop's control logic:

| Prompt clause                                              | Actually is                          |
| ---------------------------------------------------------- | ------------------------------------ |
| Start at first unchecked task, continue where you left off | Resume logic                         |
| Work phases sequentially                                   | Iteration order                      |
| Run `just lint` / `just fmt` after each task               | Evaluation step                      |
| Commit after each task, conventional commits               | Checkpoint, unverified               |
| Check off the task in the doc                              | Completion marker, unverified        |
| If phase acceptance criteria all met, move on              | Phase gate                           |
| Mark `deferred – human required`                           | Blocked state, no human signal       |
| Output `<promise>MVP COMPLETE</promise>`                   | Completion condition, agent-asserted |
| `--max-iterations 75`                                      | Run-wide cap, poor failure locality  |

This works with a human watching. Unattended, every one of these clauses is
something the loop _hopes_ the agent did. The characteristic failure is not an
agent that fails; it is an agent that ticks a box after partial work and the
loop moves on.

Moving execution onto Temporal (DESIGN-0001) forces the question of which layer
owns which responsibility.

## Decision

The workflow owns loop control; the agent owns one task at a time.

Concretely:

1. **The workflow derives position from the document.** The next task is always
   computed by parsing the IMPL doc at `HEAD` of the work branch. The workflow
   keeps a cursor for efficiency, never as the source of truth.
2. **Completion is verified, not asserted.** A task is done only when the
   workflow observes that `HEAD` advanced, the task's checkbox flipped and no
   other task changed, and the repository's mechanical checks pass. The agent's
   own "done" status is a hint that triggers evaluation.
3. **Git is the state boundary.** The workflow records the last good SHA per
   task. A retry resets the branch to that SHA before the agent runs again. The
   workflow never carries file content, only references.
4. **Blocked is a state, not a marker.** The agent returns `blocked` with a
   reason; the workflow writes the deferred marker, exposes it via search
   attributes, and waits for a human `unblock` or `skip` signal.
5. **Caps are per task and per run.** Failure locality: the run reports which
   task exhausted its iterations.
6. **Completion is computed.** The run finishes when every task is done or
   skipped and every phase gate passes. The agent never emits a completion
   promise.

The per-task prompt is reduced to skill routing, style, referenced design docs,
the commit/checkbox convention, and the result-block contract.

## Consequences

### Positive

- Each clause the prompt used to hope for becomes a check with a named failure.
  Evaluation output feeds the next iteration's prompt, giving the agent precise
  feedback instead of a re-read of the whole doc.
- Runs survive worker restarts and agent crashes without corrupting the branch,
  because retries start from a recorded good SHA.
- The prompt shrinks by roughly two thirds and stops changing when the loop
  semantics change.
- Resume after any interruption, including loss of the Temporal namespace, costs
  at most one task's iterations.

### Negative

- A rejected-but-committed iteration is discarded (reset to last good SHA)
  rather than fixed forward. This wastes tokens when the agent was close. Chosen
  deliberately: fixing forward lets untracked work accumulate on the branch.
  Open to revisiting once evaluation feedback proves reliable.
- The IMPL format gains conventions the workflow depends on: positional task
  IDs, the `verify:` continuation line, executable-vs-assertive success
  criteria. Agents must be told not to add, remove, or reorder tasks, and the
  workflow must detect it.
- Mechanical checks run twice — in the runner (agent habit) and on the worker
  (authoritative). Accepted cost for a trustworthy gate.

### Neutral

- The manual donald-loop keeps working unchanged; nothing in this decision
  requires it to be retired.
- Assertive success criteria still need a human or an explicit
  `--auto-confirm-criteria` opt-in. The workflow does not pretend to verify what
  it cannot run.

## Alternatives Considered

- **Keep the full prompt; wrap it in a Temporal activity for durability only.**
  Rejected: gains restartability, none of the verification. The 75 iteration cap
  and agent-asserted completion remain.
- **Let the agent drive Temporal (agent calls signals/activities).** Rejected:
  inverts the trust relationship; the component being verified would control the
  verifier.
- **Fix forward from rejected commits.** Deferred, see Negative above.
- **Store task state in a database instead of the doc.** Rejected: the doc is
  already the checkpoint humans read and the manual loop uses; two sources of
  truth would drift.

## References

- DESIGN-0001 Temporal-orchestrated IMPL loop execution
- `docs/reference/donald-loop-prompt.md` — the prompt being replaced
- ADR-0001 Run the IMPL loop worker as its own repository
