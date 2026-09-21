## Open Questions

> Each question is numbered; option `a` is my recommendation, later letters
> are alternatives, and the last is a free-form "other". Questions 2–4 are
> DESIGN-0013's 4–6, carried over unresolved; question 1 was DESIGN-0013's 3
> and is restated for regions.
>
> **Update 2026-09-19:** all twelve questions are resolved — see the
> Decisions table. DESIGN-0015's own questions remain open.

| # | Question | Decision |
| - | -------- | -------- |
| 1 | Phase and task grammar | (a), corrected against `impl.md`: HTML comments are stripped from the phase heading before the token regex and an empty title is a warning; a region wraps its own heading; the `---` separators sit outside every region |
| 2 | Continuation folding and verify lines | (a); worked example added to §3 |
| 3 | Marker parsing on the read side | (a) lenient |
| 4 | Criteria classification | (a) as issue #100 states |
| 5 | Creation without I/O | (a) `NextNumber` plus `Render` |
| 6 | Resolving a document by ID | (a) both `Find` and `FindIn` |
| 7 | Status no-op short-circuit | (a) in `repo.SetStatus`, reported as `Changed: false` |
| 8 | How much of wiki is orchestration | (a) `Init` and `UpdateNav` in `pkg/wiki` |
| 9 | Error shapes in repo | **(b) typed errors everywhere** — the typed API should give all of its benefits; §2.8 updated |
| 10 | Generic facts for the next type package | **superseded by DESIGN-0015**: regions are the typed spans a future package would want, and `Repo.Find` already maps a prefix to a type through config; both helpers dropped. **Amended 2026-09-19**: every built-in ships its package in this unit; the shared readers live in `kinds` (§2.12) and `docparse` gains `ListItems` and `Tables` (§2.3) |
| 11 | Delivery granularity | **one IMPL covering this design and DESIGN-0015 together**, a phase per rollout step; designs map to IMPLs many-to-one when they ship as a unit |
| 12 | Hooks for wiki | (a) none; the nav report is enough |

### 1. Phase and task grammar

> **Resolved 2026-09-19: (a)**, after checking the assumptions against
> `internal/template/templates/impl.md`: the template's phase title is an
> HTML comment placeholder, so comments are stripped before the token
> regex and an empty title is the `impl.phase.no-title` warning; the
> `#### Tasks` and `#### Success Criteria` headings sit inside their
> regions; the `---` separators between phases sit outside every region.
> DESIGN-0015 §1, §5, and §6 carry the same corrections.

- a. **Phases are `phase` regions; the token comes from the first level-3
  heading inside; `tasks` and `criteria` are nested regions; top-level
  checkbox items only; a duplicate token is an error** — §3 as amended by
  DESIGN-0015 §5. *(recommendation)*
- b. The heading heuristics as DESIGN-0013 had them (`^Phase\s+…:` finds
  the span, `#### Tasks` scopes the list), kept as a permanent fallback
  beside the regions — two grammars to maintain.
- c. Every level-3 heading is a phase with ordinal fallback (sdk-booty-sh
  today).
- d. Other.

### 2. Continuation folding and verify lines

> **Resolved 2026-09-19: (a).** The worked example in §3 shows the fold,
> the verify extraction, and the marker removal on a real-shaped task.

- a. **Fold continuations into `Text`; verify is case-insensitive, first
  backtick span, trailing prose ignored** — the four hand-written docz-api
  lines parse. *(recommendation)*
- b. Strict lowercase `verify:` whose remainder is exactly one backtick
  span.
- c. First-line-only `Text` (the facts literal) — truncates most recent
  tasks.
- d. Other.

### 3. Marker parsing on the read side

> **Resolved 2026-09-19: (a).**

- a. **Lenient**: `deferred` after any dash, prefix or suffix, emphasis
  tolerated; strikethrough plus `skipped:` after any dash. Canonical
  spellings are documented for writers (consumers), not enforced by the
  parser. *(recommendation)*
- b. Canonical suffix forms only; docz-api IMPL-0006 is hand-fixed.
- c. Other.

### 4. Criteria classification

> **Resolved 2026-09-19: (a).**

- a. **As issue #100 states**: executable iff the bullet starts with a
  backtick span; the caveat that symbol-subject criteria (about a tenth in
  this repo) classify as executable is documented, and docz never runs
  anything. *(recommendation)*
- b. Executable only when the whole bullet is a backtick span.
- c. Token heuristics on the span.
- d. Other.

### 5. Creation without I/O

> **Resolved 2026-09-19: (a).**

- a. **`NextNumber(dir, width)` plus `Render(opts, number)`**, with `Create`
  composed from them. Two small functions; a no-checkout consumer supplies
  its own number (it knows its tree) and gets `Rendered{Filename, Content}`
  to commit through its own API. *(recommendation)*
- b. One `Prepare(opts) (Rendered, error)` that scans for the next number
  itself — fewer names, but it needs the filesystem, which defeats the
  purpose.
- c. Neither now; `Create` stays the only creation entry point until a
  consumer asks.
- d. Other.

### 6. Resolving a document by ID

> **Resolved 2026-09-19: (a).**

- a. **Both `Find(id)` and `FindIn(type, id)`.** `Find` derives the type
  from the prefix through `ValidateType` (which already resolves
  `id_prefix` tokens, and `validateResolution` guarantees uniqueness) and
  serves consumers that only hold an ID, such as `task list`; `FindIn`
  mirrors `status set <type> <id>` exactly. *(recommendation)*
- b. `FindIn` only; callers split the prefix themselves.
- c. `Find` only; `status set` ignores its `<type>` argument beyond
  validation.
- d. Other.

### 7. Where the status no-op short-circuit lives

> **Resolved 2026-09-19: (a).** DESIGN-0005 Decision 8 is superseded for
> the library path; the CLI's output is unchanged because it prints from
> the result.

DESIGN-0005 Decision 8 put "current equals new → no write" in `cmd/`.

- a. **In `repo.SetStatus`, reported as `Changed: false`.** It is not
  wording; it is the operation's semantics, and every consumer wants the
  same rule. `cmd/` output is unchanged because it prints from the result.
  *(recommendation)*
- b. Keep it in `cmd/`; `repo.SetStatus` always writes when called, like
  the `docwrite` helper.
- c. Other.

### 8. How much of wiki is orchestration

> **Resolved 2026-09-19: (a).**

- a. **`Init` and `UpdateNav` live in `pkg/wiki`.** They are the two things
  a consumer would otherwise copy from `cmd/wiki.go`, and the primitives
  stay exported for anyone who wants a different composition.
  *(recommendation)*
- b. Primitives only; the orchestration stays in `cmd/wiki.go` and `wiki`
  is the one command with no single-call equivalent.
- c. Put `Init` and `UpdateNav` on `repo` — but then `doczcore` imports
  `pkg/wiki`, breaking R1's sibling rule.
- d. Other.

### 9. Error shapes in repo

> **Resolved 2026-09-19: (b)** — typed errors everywhere. Review rationale:
> a typed API should deliver all of its benefits, and a caller that can
> `errors.As` into `NotFoundError{Type, ID}` should not have to re-derive
> those facts from a message. `UnknownTypeError` unwraps to the frozen
> `config.ErrUnknownType` so `errors.Is` keeps working. §2.8 and the
> `status set` row in §4 are updated.

- a. **Sentinels for the yes/no cases (`ErrNotFound`, `ErrTypeDisabled`,
  `ErrExists`) and one typed `InvalidStatusError` carrying the allowed
  list**, with `config.ErrUnknownType` passing through. `cmd/` maps them to
  exit codes with `errors.Is`/`errors.As` and keeps its current messages.
  *(recommendation)*
- b. Typed errors everywhere (`NotFoundError{Type, ID}` …) — richer, more
  surface to freeze.
- c. Sentinels everywhere; the allowed-status list is re-derived by the
  caller from `Cfg`.
- d. Other.

### 10. Generic facts for the next type package

> **Resolved 2026-09-19: superseded by DESIGN-0015.** The question asked
> whether to add two helpers ahead of a second type package: a
> level-2-heading span reader (`docparse.Sections`) and an ID-prefix-to-type
> mapper (`document.Kind`). Regions make the first redundant — typed spans
> are exactly what the next package wants, and heading spans are the
> heuristic being retired — and `Repo.Find` already does the second
> through `config.ValidateType`. Both are dropped.
>
> **Amended 2026-09-19.** The second type package did not wait: every
> built-in is a structured type and ships its package in this unit (§2.9).
> What they share — open questions, references, decisions, criteria,
> alternatives, fields, list items — lives in `pkg/doczcore/kinds` (§2.12),
> and `docparse` gains the two facts they need, `ListItems` and `Tables`.
> `document.Kind` stays dropped.

- a. **Defer** `docparse.Sections` and `document.Kind` until a second type
  package exists; the shapes are noted so they are not redesigned.
  *(recommendation)*
- b. Add `docparse.Sections` now — small, but a frozen API with no caller.
- c. Other.

### 11. Delivery granularity

> **Resolved 2026-09-19: one IMPL covering this design and DESIGN-0015
> together**, a phase per rollout step with the swap last. Designs and
> IMPLs are not one-to-one: designs that ship as a unit are consumed by a
> single IMPL.

- a. **One IMPL document with a phase per rollout step** (five phases, the
  swap last), so the unit of delivery matches the unit of design and the
  acceptance criteria of the last phase are the swap's. *(recommendation)*
- b. One IMPL per package, tracked in a parent issue.
- c. Other.

### 12. Hooks for wiki

> **Resolved 2026-09-19: (a).** Hooks would give the wiki almost nothing:
> `UpdateNav` is one YAML read, a title walk over the docs tree, and one
> YAML write, and the two debug lines `cmd/wiki.go` logs today are
> derivable from `NavReport` after the call. Cancelling the title walk on
> a large tree is what matters, and the context covers that.

- a. **None; the nav report is enough.** `wiki.Init` and `UpdateNav` take a
  context for cancellation and return reports; the two debug lines
  `cmd/wiki.go` logs today are derivable from `NavReport` after the call.
  A `wiki.Hooks` can be added later without breaking anything.
  *(recommendation)*
- b. A `wiki.Hooks` mirroring `repo.Hooks` from day one, so both L3
  packages instrument the same way.
- c. Other.
