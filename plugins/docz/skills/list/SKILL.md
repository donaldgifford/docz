---
name: docz:list
description: >
  List docz documents in the current repository, filtered by type or status.
when_to_use: >
  "what RFCs do we have", "show me open investigations", "list our ADRs",
  "what docs exist", "show accepted decisions", "what's in Draft status".
argument-hint: "[type] [status]"
---

# Docz List

List documents with `docz list [type] [--status <status>]`.

## Process

1. **Determine filters** from the user's request:
   - Type filter: "RFCs", "ADRs", "investigations", "design docs", "plans", "impls"
   - Status filter: "open", "draft", "accepted", "in progress", "proposed", etc.

2. **Run the list command**:
   ```bash
   docz list                                    # all documents
   docz list <type>                             # filtered by type
   docz list --status "<status>"               # filtered by status
   docz list <type> --status "<status>"        # both filters
   docz list --format json                      # machine-readable output
   ```

3. **Format the output** for readability — group by type, show ID, title, status, and date.

   Custom types appear under their `id_prefix` in the TYPE column (`FW-0001` shows as `FRAMEWORKS`), and their aliases work as the type filter: `docz list fw`.

4. **Surface actionable items**:
   - Note any documents stuck in Draft that may be stale
   - If filtering by a status yields no results, suggest adjacent statuses

5. **Offer next steps** — if the list is empty, suggest `docz create`. If there are Draft items, offer to help complete them.

## Status Values by Type

| Type | Statuses |
|------|----------|
| RFC | Draft, Proposed, Accepted, Rejected, Superseded |
| ADR | Proposed, Accepted, Deprecated, Superseded |
| DESIGN | Draft, In Review, Approved, Implemented, Abandoned |
| IMPL | Draft, In Progress, Completed, Paused, Cancelled |
| PLAN | Draft, In Progress, Completed, Cancelled — *opt-in type; `docz list plan` returns an empty table when disabled, not an error* |
| INV | Open, In Progress, Concluded, Inconclusive, Abandoned |
