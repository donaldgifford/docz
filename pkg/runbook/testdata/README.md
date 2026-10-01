# pkg/runbook fixtures

Two files per document, plus a fact file:

| File | What it is |
|---|---|
| `<name>.orig.md` | A snapshot of the document with no region markers |
| `<name>.md` | The same document with canonical region markers added and nothing else changed |
| `<name>.golden.txt` | The facts `Parse` reads out of the migrated copy |

Regenerate the last two with `go test ./pkg/runbook/... -update`. Never edit
them by hand, and never edit a `.orig.md`: it is a snapshot, and its value is
that nobody maintains it alongside the parser.

## Where each came from

| Fixture | Source | What it is here for |
|---|---|---|
| `runbook-0001` | docz RUNBOOK-0001, markers removed | Three procedures, nested sub-steps, wrapped `**Expected:**` lines, a filled Last Verified row |
| `runbook-0002` | docz RUNBOOK-0002, markers removed | Troubleshooting first: five scenarios, a `json` command beside `sh` ones |
| `legacy-onboarding` | Hand-written | A runbook written before the template: steps as `##### Step N:` headings in one procedure, bullets in another, two verifiers |

The two docz runbooks were written from the template, so they are snapshots of
a document the template shaped and then had its markers taken out. There was no
fleet of runbooks to snapshot when the type arrived, which is why the third is
hand-written. Replace it with a real one when there is one.

## What the corpus taught

- **A wrapped field is the normal case.** Runbook fields are sentences:
  `**Likely cause:**` and `**Notes:**` wrap in both docz runbooks, and
  `kinds.Field` reads one line. `field` in `parse.go` folds continuation lines
  up to a blank line or the next bold label.
- **Generated markers sit tighter than hand-written ones.** Inference trims a
  region's trailing blank lines, so a migrated end marker follows the content
  directly, where both docz runbooks leave a blank line before it. The facts are
  the same either way, which `TestCorpusMigrationChangesNothingButMarkers` pins.
- **Heading-shaped steps parse as no steps.** `legacy-onboarding`'s first
  procedure has a steps region with no numbered item in it, so it reads as a
  procedure with no steps (`runbook.procedure.no-steps`). That is the honest
  reading: its steps have no addresses, and a migration that invented them
  would be renumbering a document by guesswork. Its second procedure's bullets
  are `steps.not-ordered` in the generic tier, and silent here.
