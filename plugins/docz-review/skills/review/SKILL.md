---
name: review
description: >-
  Queue docz documents for a browser review at the end of this turn, or
  list what is already queued. Use when the user asks to review a docz
  document (an RFC, ADR, design, IMPL, investigation, or runbook) the
  agent did not just create with `docz create`, or asks what is waiting
  for review.
argument-hint: "[<id|path>...]"
allowed-tools: Bash(docz-review arm *), Bash(docz-review pending*)
---

# Review docz documents

The docz-review plugin opens a review page when a turn ends, for every
document queued in this session. `docz create` queues what it creates on
its own. This skill queues anything else, and reports the queue.

## With arguments

The arguments are document ids (`DESIGN-0024`) or repository paths
(`docs/design/0024-x.md`): `$ARGUMENTS`.

1. Run them through one Bash command, exactly as given:

   ```bash
   docz-review arm <each id or path>
   ```

   The command prints `<ID> <path>` for each document that resolves and a
   `docz-review: <target>: <reason>` line on stderr for each that does
   not. The plugin's hook records the ones that resolve; running the
   command is what queues them.
2. Tell the user which documents are queued and which were not, with the
   reason.
3. Finish the turn. Do not open the page, wait, or poll: the review opens
   when the turn ends, and its answers arrive as your next instruction.

## Without arguments

Run `docz-review pending` and report the list it prints. Each line is the
session, the document id, its state (`pending`, `open`, `changes`,
`approved`, or `dismissed`), its round, and its path. Do not arm or
change anything.

## When the binary is missing

If `docz-review` is not on `PATH`, say so and point the user at the
plugin README's install section. Do not try to install it.
