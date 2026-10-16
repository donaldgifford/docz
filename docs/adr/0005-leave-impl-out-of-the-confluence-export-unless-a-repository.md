---
id: ADR-0005
title: "Leave impl out of the Confluence export unless a repository lists it"
status: Accepted
author: Donald Gifford
created: 2026-10-16
---

<!-- markdownlint-disable-file MD025 MD041 -->

# ADR-0005: Leave impl out of the Confluence export unless a repository lists it

<!--toc:start-->
<!-- markdownlint-capture -->
<!-- markdownlint-disable MD013 -->
- [Summary](#summary)
- [Context](#context)
- [Decision](#decision)
  - [Supporting Data](#supporting-data)
- [Consequences](#consequences)
  - [Positive](#positive)
  - [Negative](#negative)
  - [Neutral](#neutral)
- [Alternatives Considered](#alternatives-considered)
- [References](#references)
<!-- markdownlint-restore -->
<!--toc:end-->

<!--docz:summary:start-->
## Summary

The Confluence export stops writing IMPL documents by default. When
`sync.confluence.types` is empty, a run exports every enabled type except
`impl`; a repository that wants its IMPLs in Confluence lists its types with
`impl` among them. The rule lives in the library, so `docz export confluence`
and docz-api export the same set, and `impl` is named by a flag on its
registry entry rather than by a config key, so a custom type can never be
left out by accident. On the first full run after the change, a repository
that does not list `impl` has its existing IMPL subtree moved under
`<folder>: Archive`; nothing is deleted.

<!--docz:summary:end-->

<!--docz:context:start-->
## Context

An IMPL document is a working plan: phases, tasks, and verification lines
that change with every commit until the work is done, and that nobody
outside the repository reads once it is. The Confluence export (DESIGN-0020,
DESIGN-0021) was built to put a repository's decisions where people who do
not read git can find them, and a fleet repository's IMPLs are the largest
share of its pages and the least useful there. [INV-0022] asked whether the
export should leave them out, and where the default should live.

Its findings:

- The mechanism already existed. `sync.confluence.types` narrows a
  repository's export to the types it lists, validated against the enabled
  set (`validateSyncTypes`). Only the default was missing: an empty list
  meant every enabled type (Observation 1).
- A type dropped from the set is handled by the orphan pass. On a full run
  its type page is no longer planned, so it moves under `Archive` with its
  documents, readable and never deleted (Observation 2).
- A link from a DESIGN to its IMPL still resolves. Links are resolved
  against what the run writes first and the caller's resolver second, so an
  unexported IMPL becomes a GitHub blob link, in the CLI and on the server
  alike (Observation 3).
- Which types to export is the repository's call, not the operator's. The
  server's space allow-list limits where a credential may write, and a
  repository cannot widen it; the type set is about what is useful to read,
  which the repository knows best (Observation 4).
- A server-only default would make the CLI and docz-api disagree: the same
  repository would export one set from a laptop and another from the
  server, and the two would archive and restore each other's pages in turn
  (Observation 5).

The investigation resolved four questions, each as recommended: the default
lives in the library; a repository opts in by listing its types; `impl` is
the only type left out, named in the registry; and existing IMPL pages are
archived on the next full run.

<!--docz:context:end-->

<!--docz:decision:start-->
## Decision

1. **`DocTypeDef.ExportOptIn`** marks a built-in the export leaves out
   unless a repository lists it. `impl` carries it; no other entry does, and
   a registry test says so. The flag is on the registry entry and not a
   `.docz.yaml` key because a custom type has no registry entry: it can
   never carry the flag, so it can never be left out by accident.
2. **`Config.DefaultExportTypes()`** is `EnabledTypes()` without the
   built-ins whose entry is `ExportOptIn`. It is the set a run exports when
   `sync.confluence.types` is empty, in the CLI and on the server, because
   both reach `confluence.Export` through the same `canonicalTypes`.
3. **A repository opts in by listing its types** with `impl` among them:
   `types: [rfc, adr, design, impl, investigation]`. No new key. The
   existing validation covers the list, and the generated `.docz.yaml`
   comment says what an empty list means.
4. **A narrowed run that asks for a type a full run would not write is
   refused**, with `NotExportedError` (exit 2 from the CLI) naming the type
   and the fix: `type "impl" is not exported; list it in
   sync.confluence.types`. Before this decision `--type impl` against a list
   that left `impl` out exported nothing and said nothing; the refusal now
   covers that case too, since a narrowed run must never write a page a full
   run would archive.
5. **Existing IMPL pages are archived** by the first full run that no longer
   plans them, as any dropped type is. The release note, RUNBOOK-0003's
   enable step, and DESIGN-0021 §8 say it will happen. There is no frozen
   state; a repository that wants the pages back lists `impl`, and the next
   full run restores the subtree.

### Supporting Data

- INV-0022, Observations 1 to 5, decided 2026-10-15 from a reading of
  `plan.go` (`canonicalTypes`), `orphans.go`, and `sync.go`.
- `TestExport_ImplIsOptIn` (`pkg/export/confluence`): a bare run writes no
  IMPL page; `--type impl` and an IMPL id are refused; listing the five
  types creates the subtree; emptying the list again archives it and the
  page is still there.
- The plan goldens (`testdata/plan/*.golden.json`) over the corpus
  repository, which carries ten IMPLs, lost every IMPL item and the
  `docz:type:impl` key under `-update`, and the narrowed cases lost the
  IMPL link targets, which is the Observation 3 fallback made visible.
- A dry run of this repository's own export on 2026-10-16, with the change
  built and no `types` list in `.docz.yaml`, planned 68 pages and one
  archive: `docz: Implementation Plans  moved under Archive`. The live
  check INV-0022 deferred is that run without `--dry-run`, the first full
  run after the change.

<!--docz:decision:end-->

<!--docz:consequences:start-->
## Consequences

<!--docz:positive:start-->
### Positive

- A repository's Confluence tree holds its decisions and designs, not its
  task lists. Fewer pages, fewer `updated` lines per export, fewer orphans
  to archive when a plan is renamed or folded.
- One rule in one place. The CLI and docz-api cannot disagree about the
  default set, and a change to it is a registry edit with a test.
- A custom type is safe by construction: nothing a repository defines can
  be opted out by a flag it cannot set.
- A narrowed run that would have exported nothing now says why.

<!--docz:positive:end-->

<!--docz:negative:start-->
### Negative

- The meaning of an empty `sync.confluence.types` changes on a stable
  surface. A repository that relied on it to export IMPLs sees them
  archived on its next full run, with a release note and a runbook line as
  the warning. Nothing is lost, and listing the types restores the subtree.
- A repository that wants everything must now list every type, and must
  add a new type to the list when it enables one. The alternative, a
  wildcard or an `include_impl` key, was judged not worth a new config
  surface (INV-0022 question 2).
- A second opt-in built-in needs an ADR, because the flag is a decision
  about what Confluence is for and not a per-repository setting.

<!--docz:negative:end-->

<!--docz:neutral:start-->
### Neutral

- `Config.EnabledTypes()` is unchanged. `impl` is still enabled, created,
  listed, validated, and indexed; only the export's default set narrows.
- `ExportOptions.Types` and `IDs` keep their meaning as a narrowing of the
  full run. The full run's set is what changed.
- The plan goldens and three first-run tests lost their `Implementation
  Plans` line; that is the change, recorded where a regression would show.

<!--docz:neutral:end-->
<!--docz:consequences:end-->

<!--docz:alternatives:start-->
## Alternatives Considered

- **A server default, `CONFLUENCE_EXCLUDE_TYPES=impl`**, applied when a
  repository's list is empty. The CLI would keep exporting everything, and
  the same repository would export two sets depending on who ran it
  (INV-0022 Observation 5). Rejected.
- **A server switch the repository cannot override**,
  `CONFLUENCE_EXPORT_IMPL=false`. The simplest, and the wrong layer: the
  type set is about what is useful to read, which the repository knows and
  the operator does not (Observation 4). Rejected.
- **A per-type `export: false` key in `.docz.yaml`**, so a repository could
  opt out its own custom types the same way. Nothing has asked for it; it
  can follow as an addition if one does, without reversing this decision.
  Deferred.
- **A wildcard, `types: ["*"]`, or a `sync.confluence.include_impl` key**
  as the way to say "everything". Listing the types is explicit and already
  validated, and either alternative adds a config surface for one case.
  Rejected.
- **Freezing existing IMPL pages** rather than archiving them. The export
  has no frozen state, and inventing one for a one-time transition is more
  machinery than the archive pass, which already does the right thing and
  deletes nothing. Rejected.

<!--docz:alternatives:end-->

<!--docz:references:start-->
## References

- [INV-0022](../investigation/0022-leaving-impl-documents-out-of-the-confluence-export-by-default.md):
  the investigation and its four resolved questions
- [DESIGN-0020](../design/0020-confluence-export-docz-documents-as-confluence-cloud-pages.md):
  the `sync:` block and the CLI export
- [DESIGN-0021](../design/0021-confluence-export-from-docz-api-per-repository-folders-in.md):
  the server export, its §8 CLI changes
- [RUNBOOK-0003](../runbook/0003-enable-confluence-export-on-docz-api.md):
  enabling the export, with the opt-in note
- Issue [#183](https://github.com/donaldgifford/docz/issues/183)
- [`pkg/doczcore/config/doctype.go`](../../pkg/doczcore/config/doctype.go)
  (`ExportOptIn`), [`config.go`](../../pkg/doczcore/config/config.go)
  (`DefaultExportTypes`), and
  [`pkg/export/confluence/plan.go`](../../pkg/export/confluence/plan.go)
  (`canonicalTypes`, `NotExportedError`)

[INV-0022]: ../investigation/0022-leaving-impl-documents-out-of-the-confluence-export-by-default.md

<!--docz:references:end-->
