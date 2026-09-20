## Open Questions

> Each question is numbered; option `a` is my recommendation, later letters
> are alternatives, and the last is a free-form "other".
>
> **Update 2026-09-19:** all nine questions are resolved — see the
> Decisions table.

| # | Question | Decision |
| - | -------- | -------- |
| 1 | Where does the schema come from? | **(d)** a marker skeleton the document names in frontmatter, baked in per built-in type or a repo file under `templates/schema/`; templates become golden tests (§3) |
| 2 | Marker spelling on the read side | (a) lenient read, canonical write, `marker.spelling` warning, fixed by the migration pass |
| 3 | Is the finding message part of the contract? | (a) `Code` is the contract; `Detail` is a default a consumer may replace by code |
| 4 | How is the corpus migrated? | (a) `docz update --regions`; **amended 2026-09-20 to (c)** `docz validate --fix`, since inference is permanent and the pass is defined by the findings `validate` reports |
| 5 | Do ToC and index drift belong to validate? | (a) yes: `toc.stale` per document, `IndexDrift` per type; subsumes issue #97's `update --check` |
| 6 | Which regions does the IMPL template mark? | (a) the full set: `phase` with nested `tasks` and `criteria`, plus `testing` and `references`; **amended 2026-09-19** to every section (§3) |
| 7 | Hand-rolled walker or a CommonMark AST? | (a) hand-rolled, stdlib-only; goldmark behind the frozen contract only if CommonMark-fidelity bugs keep arriving |
| 8 | Unknown kinds | (a) allowed; well-formedness only |
| 9 | ToC and index splices on the region walker? | (a) internally yes: `Regions` reports the legacy ToC pair as `toc` and the README pair as `index`, and `toc.UpdateToC` and `index.Splice` locate their spans through it; externally nothing changes (DESIGN-0014 §2.5, §2.6) |
| — | **Amendment 2026-09-19: every built-in is a structured type** | The catalogue grows from nine to forty-one kinds and every template section is a region (§2); each built-in's skeleton lists all of them (§3); `InsertRegions` derives its heading map from the marked template (§6); `docz validate` dispatches each document's type package on the resolved schema name with the type name as fallback, carried in `DocFindings.Schema` (§4). Unstructured markdown is the `api:` block's additional docs, not a type. DESIGN-0014 §2.9 and §2.12 hold the packages; IMPL-0018 Open Question 10 records the dispatch rule |
| — | **Amendment 2026-09-20: documents without markers still parse** | A document with no `docz:` marker is inferred from its headings by `kinds.InferRegions`, in every type package's `Parse` (`Doc.Inferred`) and in `validate.Document` (`Options.Headings`, one `region.inferred` warning); a heading that is absent is still `region.missing`; markers, once present, are authoritative. Migration is `docz validate --fix`, which writes what inference found and re-validates (Open Question 4 as amended). The heuristic is permanent and pinned by the inference-equals-markers proof (§4, §6, DESIGN-0014 §2.9, §2.12) |

### 1. Where does the schema come from?

> **Resolved 2026-09-19: (d) — a marker skeleton the document names.**
> Neither the template nor a config block. The schema is its own file
> under `templates/schema/<name>.md`, with a baked-in one per built-in
> type versioned with the library; a document names it in an optional
> `schema:` frontmatter field, absent meaning the type's own; the
> template becomes the golden test against it, and a custom type gets the
> pair scaffolded by `template override`. Reasoning and the full shape in
> §3. Raised by Donald on reading ADR-0002 Open Question 5: the template
> is a rendering artifact, and a schema that lives in it cannot be reached
> by docz-api, cannot be golden-tested, and is silently loosened by an
> override.

- a. **The resolved template, read through `Markers`.** Zero config, custom
  types get it for free, and `docz template override` is literally
  "edit the schema." Presence and nesting come from the template; content
  rules come from the kind. *(recommendation)*
- b. A `regions:` block under each type in `.docz.yaml`, with registry
  defaults for built-ins — explicit, but a second place to keep in step
  with the template.
- c. Both: the template by default, the config block as an override.
- d. Other.

### 2. Marker spelling on the read side

> **Resolved 2026-09-19: (a).**

- a. **Lenient read, canonical write, spelling reported as a warning and
  fixed by the migration pass.** The INV-0009 lesson applied to the new
  markers on day one. *(recommendation)*
- b. Canonical spelling only; a variant is not a marker and validate
  reports the resulting missing region.
- c. Other.

### 3. Is the finding message part of the contract?

> **Resolved 2026-09-19: (a).**

- a. **`Code` is the contract; `Detail` is a default a consumer may replace
  by code.** Linters work this way, docz-api can map codes to its own
  wording, and the CLI prints `Detail` as-is. R4's "wording only in L4"
  bends here on purpose: a validator with dozens of codes and no default
  text would force every consumer to write the same table.
  *(recommendation)*
- b. Codes and structured fields only, no `Detail`; `cmd/` owns a wording
  table.
- c. Other.

### 4. How is the corpus migrated?

> **Resolved 2026-09-19: (a). Amended 2026-09-20: (c).** Once inference
> became a permanent fallback rather than a one-shot heuristic (§6), the
> pass is defined by findings — `region.inferred` and `marker.spelling`
> — and belongs to the command that reports them: `docz validate --fix`
> writes the markers inference found, re-validates, and prints what
> remains. "Fix" promises exactly that and nothing more. Plain `validate`
> is the preview, so there is no dry-run flag, and no `update --regions`.

- a. **`docz update --regions`, dry-run aware, one-shot by nature.** No new
  command family for a pass each repo runs once; the flag can be removed
  in a later major without anyone noticing. *(recommendation)*
- b. `docz migrate regions` — clearer name, one more command family.
- c. `docz validate --fix` — but the fixer only inserts and canonicalizes
  markers, and "fix" promises more.
- d. Other.

### 5. Do ToC and index drift belong to validate?

> **Resolved 2026-09-19: (a).** Issue #97's `update --check` is subsumed;
> the issue was retargeted at `docz validate` on 2026-09-20 (title, body
> note, and comment) and closes when IMPL-0018 Phase 5 lands. v1 never
> gets `--check`.

- a. **Yes.** A stale ToC is a document finding (`toc.stale`) and a stale
  README is a repository finding; `docz validate` therefore subsumes
  issue #97's `update --check` and the CI story is one command.
  *(recommendation)*
- b. Keep drift in `update --check` as #97 proposes and let validate check
  documents only.
- c. Other.

### 6. Which regions does the IMPL template mark?

> **Resolved 2026-09-19: (a).**

- a. **The full set: `phase` with nested `tasks` and `criteria`, plus
  `testing` and `references`.** Fifteen pairs on a five-phase document is
  the cost; the spans a program needs are all explicit and the testing
  exclusion stops being a heading-text rule. *(recommendation, per review)*
- b. `phase` only, with `tasks` and `criteria` still found by heading
  inside the region — fewer markers, one heuristic kept.
- c. Other.

### 7. Hand-rolled walker or a CommonMark AST?

> **Resolved 2026-09-19: (a).**

- a. **Hand-rolled, stdlib-only, matching the other `docparse` walkers.**
  Marker lines are trivially recognized at the line level; the public core
  stays stdlib plus yaml; the frozen fence rule is reused. Trigger for
  revisiting: if CommonMark-fidelity bugs like #96 keep arriving, migrate
  the walkers onto goldmark *behind* the frozen `docparse` contract with
  parity goldens, the way `toc` was moved onto `docparse`. The markers
  are unaffected either way. *(recommendation)*
- b. Adopt goldmark now for all of `docparse`, with a docz extension that
  parses markers into AST nodes.
- c. Other.

### 8. Unknown kinds

> **Resolved 2026-09-19: (a).**

- a. **Allowed; well-formedness only.** A custom type's template can
  declare `<!--docz:risks:start-->` and validate checks pairing, nesting,
  and presence but no content rule. *(recommendation)*
- b. Warn on kinds outside the catalogue.
- c. Other.

### 9. Should the ToC and index splices sit on the region walker?

Both existing splices find their markers with `strings.Cut` and their own
spellings; the review asked how much existing behaviour the markers can
absorb.

> **Resolved 2026-09-19: (a).** §1's index-marker rule and §2's catalogue
> are definitive; DESIGN-0014 §2.5 and §2.6 record the internal change to
> `toc` and `index`.

- a. **Internally yes, externally nothing changes.** `Regions` reports
  `<!--toc:start-->` as kind `toc` and the README `DOCZ AUTO-GENERATED`
  pair as kind `index`, both under their legacy spellings; `toc.UpdateToC`
  and `index.Splice` locate their span through it. One marker walker
  module-wide, and validate's `toc.stale` check reads the same span the
  splicer writes. The frozen `toc` behaviour stays pinned by its golden.
  *(recommendation)*
- b. Leave both splices alone; `Regions` reports `toc` for validation
  only and never sees the index pair.
- c. Other.
