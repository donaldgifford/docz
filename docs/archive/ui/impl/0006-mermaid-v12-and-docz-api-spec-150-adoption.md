---
id: IMPL-0006
title: "Mermaid v12 and docz-api spec 1.5.0 adoption"
status: Completed
author: Donald Gifford
created: 2026-09-14
---

<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL-0006: Mermaid v12 and docz-api spec 1.5.0 adoption

<!--toc:start-->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Decisions](#decisions)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: Vendor the 1.5.0 contract](#phase-1-vendor-the-150-contract)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2: Light up the timestamps](#phase-2-light-up-the-timestamps)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 3: Order and filter the directory](#phase-3-order-and-filter-the-directory)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 4: Mermaid v12 and ELK](#phase-4-mermaid-v12-and-elk)
    - [Tasks](#tasks-3)
    - [Success Criteria](#success-criteria-3)
  - [Phase 5: Deployment-selectable diagram layout](#phase-5-deployment-selectable-diagram-layout)
    - [Tasks](#tasks-4)
    - [Success Criteria](#success-criteria-4)
  - [Phase 6: Guards, docs, release](#phase-6-guards-docs-release)
    - [Tasks](#tasks-5)
    - [Success Criteria](#success-criteria-5)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Dependencies](#dependencies)
- [Open Questions](#open-questions)
  - [OQ-11 (answered: b)](#oq-11-answered-b)
    - [Finding (2026-09-14, mermaid 11.16.0, before the v12 bump)](#finding-2026-09-14-mermaid-11160-before-the-v12-bump)
  - [OQ-1 to OQ-10 (answered)](#oq-1-to-oq-10-answered)
- [References](#references)
<!--toc:end-->

## Objective

Adopt two upstream changes that landed independently and both touch
surfaces this repo owns.

1. **docz-api spec 1.5.0** ([#29]) adds `created` and `updated_at` to
   `SearchHit`, a `sort` parameter, and a `source` filter. The
   directory's updated column has been rendering an em dash since
   IMPL-0005 because the field did not exist; it now does.
2. **mermaid 12.0.0** ([#30]) is a breaking major: ELK replaces dagre as
   the default layout, appearance defaults change, and the browser floor
   rises to ES2024 / Safari 17.4.

**Implements:** no RFC or DESIGN of its own. The diagram layout decision
becomes a ninth amendment to DESIGN-0005, which already owns the
monochrome diagram policy; the search changes complete asks recorded in
DESIGN-0001 and DESIGN-0004.

Neither change is forced. Every 1.5.0 change is additive, and mermaid 11
keeps working. This is deliberate adoption, not a forced migration,
which is why the phases below are ordered so each is independently
revertable.

## Scope

### In Scope

- Re-vendoring `api/openapi.yaml` at 1.5.0 and regenerating the orval
  client.
- Consuming `SearchHit.updated_at` and `SearchHit.created` as typed
  fields, replacing the defensive read.
- Sending `sort` from the directory, and handling the new `400`.
- Sending `source`, replacing client-side doc/page separation.
- Teaching the MSW fixtures to honor `sort` and `source` so `dev:msw`
  and e2e reflect real ordering.
- Upgrading mermaid to 12.x and **adopting ELK as the default layout**.
- A **deployment-selectable layout override** — `DOCZ_MERMAID_LAYOUT`
  through the container and the Helm chart — so a deployment can fall
  back to dagre without a rebuild.
- Auditing whether a document's own front matter can override the two
  settings that make `MermaidBlock` safe (see [OQ-11](#oq-11-answered-b)).
- Widening the e2e chunk assertion so an eagerly-imported ELK cannot
  pass a test written to catch exactly that.
- Updating `CLAUDE.md`, DESIGN-0005, and the comments 1.5.0 falsifies.

### Out of Scope

- **Any change in docz-api.** Both upstream changes are merged and
  released there.
- **Multi-value facet filters.** `searchDocs` still accepts one value
  per facet; `toSearchDocsParams` still sends the first of each array.
  Unchanged by 1.5.0.
- **Re-ranking the palette.** The palette wants relevance, not recency.
  It sends no `sort`.
- **ELK sub-algorithms as deployment configuration.** `elk.stress`,
  `elk.box` and friends stay selectable per diagram in front matter, not
  per deployment. The override is a two-value escape hatch, not a
  layout-engine control panel.
- **A diagram-authoring guide.** If ELK changes how authors should write
  flowcharts, that is a follow-up doc.

## Decisions

All eleven questions were answered on 2026-09-14. Recorded here so the
phases below read as instructions rather than options; the original
alternatives are preserved in [Open Questions](#open-questions).

| # | Decision |
| - | -------- |
| OQ-1 | **Adopt ELK as the default**, matching v12, plus a deployment override so a site can select `dagre` without a rebuild. Phase 5. |
| OQ-2 | Release as **`minor` → v0.8.0**. |
| OQ-3 | Sort `updated_at:desc` **only when the query is empty**; relevance once the user types. |
| OQ-4 | Set an **explicit `build.target`** and state the browser floor in `README.md`. |
| OQ-5 | Add a **`BadRequestError`** class and add it to the no-retry list. |
| OQ-6 | **Do not** surface `created` on the directory card. |
| OQ-7 | **Delete `hitUpdatedAt`**; read the typed property directly. |
| OQ-8 | Record the layout decision as a **ninth amendment to DESIGN-0005**. |
| OQ-9 | **One PR**, phases in order, mermaid last. |
| OQ-10 | Resolved by fact: **docz-api v0.10.0 carries spec 1.5.0** (so does v0.9.1). No release gate needed. |
| OQ-11 | **Audit and fix the `secure`-list gap inside Phase 4**, not as a separate PR. It predates the upgrade, so it gains no urgency from it. |

No questions remain open. OQ-11 arrived after the others, raised by a
finding made while planning Phase 5; it is answered on the same terms.

## Implementation Phases

Each phase builds on the previous one. A phase is complete when all its
tasks are checked off and its success criteria are met.

Phases 1–3 are the API work; Phases 4–5 are the mermaid work. They share
no files, so the mermaid phases can still be lifted into their own PR if
review gets unwieldy.

---

### Phase 1: Vendor the 1.5.0 contract

Bring the spec and the generated client up to 1.5.0 with **no behavior
change**. This phase should be reviewable as "types got wider, nothing
moved".

#### Tasks

- [x] Flip this document's status from Draft to In Progress. Every
      question is answered; from here it is the code that changes, not the
      plan.
- [x] Copy `api/openapi.yaml` from docz-api (currently 1.4.1 here, 1.5.0
      upstream). The diff is: `source` and `sort` query params on
      `searchDocs`, a `400` response on that operation, a new
      `BadRequest` response component, `created` and `updated_at` added
      to `SearchHit` and both listed `required`, plus description-only
      edits to `Document.updated_at` and `Session.groups`.
- [x] Run `bun run gen-api`; confirm the generated client gains the two
      `SearchHit` properties and union types for `sort` and `source`.
- [x] Run `just gen-api-check` — the drift gate must be clean against
      the freshly vendored spec.
- [x] Confirm `bunx tsc -b --force` passes with no source change.
      `SearchHit` gaining required properties is a widening for readers;
      the only expected breakage is in code that *constructs* a
      `SearchHit`.
- [x] Fix the construction sites the typecheck reveals by adding
      `created` and `updated_at`. **Four sites, not the three planned**
      — `src/components/command-palette.tsx` was missed, where recents
      are synthesized into hits. There both fields are `""` permanently,
      not as placeholders: recents store coordinates and title only, so
      the stamps are genuinely unknown, exactly like the `status` and
      `author` already sitting at `""` beside them.
- [x] Confirm no runtime behavior changed: full `bun run test` green
      with no edits beyond those construction sites.

#### Success Criteria

- `api/openapi.yaml` reports `info.version: 1.5.0`.
- `just gen-api-check` is clean.
- `bunx tsc -b --force`, `bun run lint`, `bun run format:check` pass.
- All existing tests pass with no assertion changes.
- The OpenAPI Spec Drift workflow reports no drift on the PR.

---

### Phase 2: Light up the timestamps

Replace the defensive read with the typed field, correct the premise the
old comments encode, and make the fixtures carry real dates.

#### Tasks

- [x] Delete `hitUpdatedAt` (OQ-7) and read `hit.updated_at` directly at
      the call site in `src/routes/directory.tsx`. The wrapper existed
      only to hide an untyped probe.
- [x] Rewrite the `src/lib/updatedAt.ts` module comment. It currently
      describes the field as absent from the schema and explains the
      upstream ask — all now false and actively misleading.
- [x] Keep `formatUpdatedStamp` and its en-US/`timeZone` contract
      unchanged; it is already correct for an RFC3339 UTC input. Keep
      its `Number.isNaN` guard in particular: it is what makes a missing
      field from an older API degrade to the em dash rather than
      rendering "Invalid Date".
- [x] Correct the "pages have no timestamp" premise, which 1.5.0
      falsifies, in all three places it appears: the
      `src/lib/updatedAt.ts` module comment, the `UpdatedCell` comment
      in `src/routes/directory.tsx`, and the fixtures comment.
      **`created` is the field that is empty on page hits**, not
      `updated_at`.
- [x] Give fixture page records a timestamp. `Page` has no `updated_at`
      in the spec (verified — the schema is `repo`, `path`, `title`,
      `raw_md`, `git_sha`), so `FixturePageInput` needs its own
      `updatedAt`, used when building search hits.
- [x] Give fixture doc hits a `created` from the document's frontmatter
      date, and page hits `created: ""`.
- [x] Remove the `SearchHit & { updated_at: string }` intersection in
      `src/mocks/fixtures.ts`; the plain generated type now suffices.
- [x] Update `src/lib/updatedAt.test.ts`: the "returns '' for the hits
      today's API actually sends" case describes a world that no longer
      exists. Cover the real shapes instead, including `""`.
- [x] Update `src/routes/directory.test.tsx`: page rows now render a
      stamp rather than an em dash, so the current
      `getAllByText("—").length > 0` assertion will fail, and should.
      Replace it with one asserting both kinds render a date.

#### Success Criteria

- No file in `src/` casts, probes, or otherwise second-guesses the
  presence of `updated_at`.
- No comment in `src/` claims pages have no timestamp.
- Under `bun run dev:msw`, every directory row — doc and page — renders
  a real date and time.
- `bun run test` green; the tests that change do so because the contract
  changed, not because assertions were loosened.

---

### Phase 3: Order and filter the directory

Send `sort` and `source`. First phase with user-visible behavior change,
and the first that can regress search quality.

#### Tasks

- [x] **Representation decided while implementing: neither key goes in
      the URL, and `DirectorySearchState` is unchanged.** OQ-3 makes
      `sort` *derived* from whether `q` is empty, and no control selects
      a `source`, so a URL key for either would be state nothing can
      set — speculative surface, not filter truth. `toSearchDocsParams`
      grows an explicit `{ ordered }` option instead, passed only by the
      query that renders rows; `source` is applied where a fixed value
      is genuinely wanted (`useRepoFacts`). If a sort or source control
      ever lands, the URL key lands with it.
- [x] Map them in `toSearchDocsParams` using the generated union types,
      never string literals.
- [x] Implement the OQ-3 ordering: send `sort=updated_at:desc` when
      `state.q` is empty, and send no `sort` once the user has typed.
      The spec is explicit that `sort` is a **total order over the
      matches, not a tie-break within relevance**, so sorting during a
      text search ranks recent-but-irrelevant hits above the best match.
      Put that reasoning in a comment; it is not self-evident from the
      code.
- [x] Confirm the grow-the-window pagination still holds. The directory
      sends `offset: 0` with `limit: state.offset + PAGE_SIZE`, so rows
      0..N are refetched on every "load more". A total-order sort makes
      this *more* stable than relevance ranking did — assert it rather
      than assume it.
- [x] Teach the fixture `searchDocs` resolver to honor `sort`, including
      the documented quirk that **records with no value for the sort key
      sort last in both directions**, so `created:*` puts page hits after
      every document either way. A fixture that ignores this makes
      `dev:msw` and e2e lie about ordering, and every downstream test
      inherits the lie.
- [x] Teach the fixture resolver to honor `source`, and to return
      `400 {"error":"invalid sort"}` for an unrecognized `sort` so the
      error path is reachable in tests.
- [x] Add a `BadRequestError` class in `src/api/fetcher.ts` beside
      `SessionRequiredError`/`NotFoundError`/`SessionUnavailableError`,
      and add it to the no-retry predicate in
      `src/app/query-client.ts` (OQ-5). A 400 is a stable answer;
      retrying twice is waste.
- [x] Replace client-side doc/page separation with `source` where the UI
      already distinguishes them. Keep the directory count line's
      "· X docs · Y pages" behavior byte-identical for deployments that
      publish no pages.
- [x] Add `src/lib/searchParams.test.ts` cases for the new keys: parse,
      serialize, round-trip, default omission, and rejection of a value
      outside the enum.
- [x] Add a directory route test that the ordering flips when the query
      empties and back when it fills.

#### Success Criteria

- Empty directory lists newest-first; typing a query returns to
  relevance ordering. Both verified in `dev:msw` against fixture dates.
- A deep link carrying the new params reproduces the same rows in the
  same order.
- "Load more" never reorders rows already rendered.
- An unrecognized `sort` surfaces as a handled `BadRequestError` with no
  retry.
- `bun run test` and `just e2e` green.

---

### Phase 4: Mermaid v12 and ELK

Independent of Phases 1–3; shares no files with them.

The security posture is the part that cannot be rushed. The v12 release
notes say **nothing** about `securityLevel` or `htmlLabels`. Silence is
not evidence. `strict` alone is known to be insufficient —
purified-but-real elements still materialize inside `foreignObject`
labels, which is why `htmlLabels: false` is set both globally and
per-flowchart.

#### Tasks

- [x] **Audit the `secure` list first, before the upgrade** — see
      [OQ-11](#oq-11-answered-b). **Confirmed on 11.16.0: a diagram's own
      front matter could set `htmlLabels: true` at both the global and
      the nested `flowchart` path**, with `securityLevel` correctly held
      at `strict` throughout. Full write-up, including the config path
      and the merge semantics that shaped the fix, in
      [the OQ-11 finding](#finding-2026-09-14-mermaid-11160-before-the-v12-bump).
- [x] If the audit confirms it, pass an explicit `secure` array in
      `initialize()` covering `htmlLabels` and the nested path, and add
      a hostile-front-matter row to the XSS suite. `secure` is itself in
      the secure list, so a document cannot unset it.
      **Done as `MERMAID_SECURE_KEYS` +
      `mermaidInitConfig()` in `src/markdown/mermaid-block.tsx`.** The
      hostile-front-matter coverage did *not* land in
      `processor.xss.test.tsx` as planned: that suite tests the markdown
      pipeline, and mermaid config is merged inside `MermaidBlock`,
      downstream of everything the processor does. It went to a new
      `src/markdown/mermaid-config.test.ts` (the sibling MermaidBlock
      suite mocks mermaid, so it could not host this) plus a second
      figure in the e2e rendering fixture, which is where the
      *rendered-output* half of the guarantee already lives.
- [x] Bump `mermaid` to `^12.0.0` and update the lockfile. Resolves to
      12.0.0 exactly (the only 12.x published), pulling `elkjs ^0.9.3`
      and `dompurify ^3.4.12`, and declaring `engines.node >=22.12.0`.
      Unit suite, typecheck, lint, and `just e2e` are all green on the
      bump alone, before any layout change — including the OQ-11 audit,
      so `secure`, `securityLevel`, and `htmlLabels` keep their 11.x
      semantics in 12.x.
- [x] Adopt ELK as the default layout (OQ-1) by setting `layout`
      explicitly in `getMermaid()` rather than relying on the new
      default. An explicit value is what Phase 5's override reads, and
      it keeps the config self-describing. Landed in
      `mermaidInitConfig()`, which `getMermaid()` now calls — the config
      moved out of the closure in the OQ-11 commit so tests could
      exercise the shipped object rather than a copy of it. Confirmed
      against the v12 schema that `layout` already defaults to `"elk"`,
      so this is a statement of intent, not a behavior change.
- [x] Keep `nodeBorder` in `mermaidThemeFromTokens()`. The new `neo`
      look paints node strokes with a gradient when the theme sets
      `useGradient`, which `base` does, and a custom `nodeBorder` is what
      turns that off. Removing it would grow gradients silently and
      break the monochrome policy. **Verified, and the mechanism is
      exactly as described**: `look` does default to `"neo"` in 12.0.0,
      `theme-base` sets `useGradient = true`, and `Theme.calculate`
      clears it when the overrides carry `nodeBorder` and *not*
      `useGradient` — which is this map. The node stroke then reads
      `nodeBorder` instead of `url(#…-gradient)`. Pinned by a test on
      the merged `themeVariables`, so the key cannot be dropped as
      "just a color".
- [x] Re-validate every key in `mermaidThemeFromTokens()` against the
      v12 theme schema. The map is commented as the "minimal documented
      v11 set", and an unknown variable breaks `mermaid.render`
      *silently* — the failure mode is a blank figure, not an error.
      All 16 keys are still consumed by v12's `theme-base`; none were
      renamed or dropped. Rather than leave that as a one-time reading,
      the test now asserts each key survives the merge into
      `themeVariables` with its value intact, so a future rename fails
      CI instead of blanking a figure.
- [x] Verify `securityLevel: "strict"` and `htmlLabels: false` still
      exist, are still honored, and still mean the same thing in 12.x.
      Read the v12 config schema; do not infer from the absence of a
      release note. **Both intact.** `securityLevel` keeps its four
      levels (`strict | loose | antiscript | sandbox`) and its "level of
      trust for parsed diagram" meaning. The root `htmlLabels` is still
      a boolean and v12 adds a *strengthening*: it now explicitly
      outranks every per-diagram copy, and those (`flowchart.htmlLabels`
      and the same key on other diagram blocks) are deprecated in its
      favour. So one root `false` covers diagram types this repo never
      names. The nested one stays set regardless — precedence is
      upstream's rule to change, and the line is cheap. Pinned by a test
      that walks the whole merged config for any `htmlLabels: true`
      after hostile front matter, rather than naming blocks.
- [x] Run the XSS suite (`src/markdown/processor.xss.test.tsx`) and the
      e2e hostile-label row in `e2e/rendering.spec.ts`, which renders a
      node label carrying an `<img>` payload. Confirm no element
      materializes from document text. All 48 XSS rows green under v12,
      including both mermaid rows; e2e green with zero `img`,
      `foreignObject`, or `script` under `.doc-prose` and no dialog,
      across both hostile figures.
- [x] Confirm no advisory applies to the DOMPurify version v12 resolves
      (`^3.3.3` → `^3.4.12` transitively). **One did.**
      GHSA-55q2-fjhq-7xh7 (moderate, XSS) covers `<= 3.4.12` — exactly
      the version the bump landed on — and is fixed in 3.4.13. Read in
      full rather than taken at severity: it needs `IN_PLACE`
      sanitization *plus* a `beforeSanitizeElements`/
      `uponSanitizeElement` hook that detaches a node, and mermaid does
      neither. It sanitizes a **string** (`innerHTML` of the render
      container, so never `IN_PLACE`) and its only hooks are
      `before/afterSanitizeAttributes`, which copy an anchor `target`
      and never remove anything. Not reachable.
      Pinned to `^3.4.13` anyway via a `dompurify` entry in package.json
      `overrides` (resolves 3.4.15), because it costs nothing and this
      library is the sanitizer behind the one `innerHTML` in the
      codebase. `bun update dompurify` is the wrong tool — it adds a
      direct dependency on a package we never import.
- [x] Widen the chunk assertion in `e2e/rendering.spec.ts` from
      `/mermaid/i` to also match ELK. ELK ships as its own ESM chunk
      whose filename will not contain "mermaid", so an eagerly-imported
      ELK would pass the test written to prevent exactly that. Confirmed
      from the build output — the chunk is `assets/elk-<hash>.js`, no
      "mermaid" anywhere in the name. The diagram page now also asserts
      the ELK chunk **is** fetched, which both proves ELK is the layout
      actually in use at runtime and keeps the diagram-free assertion
      from passing on a pattern that matches nothing.
- [x] Re-check the jsdom mermaid mock in `src/a11y/axe.test.tsx` still
      matches the v12 module shape. It does, and the surface it has to
      match is narrower than before: `default.initialize` and
      `default.render` are everything `MermaidBlock` touches, because
      `mermaidInitConfig()` builds the config without a mermaid
      instance. (Had `secureKeys` kept its first shape — reading
      upstream's list off the live module — the mock would have needed
      `mermaidAPI` too. Worth remembering as an argument for config
      that does not interrogate the library it configures.) Recorded in
      the mock's comment so the next person knows when it must grow.
- [x] Confirm all three specimen fences render under ELK, including
      Figure 2's per-node `classDef` colors. Mermaid scopes a diagram's
      own `classDef` by render id, which is why it outranks the
      stylesheet; confirm that still holds. It does — all three render,
      and Figure 2's three colors arrive verbatim
      (`#9ece6a` / `#e0af68` / `#7dcfff`). Now an e2e test reading
      computed `stroke` rather than a one-time look, since a theme or
      layout change can break this without throwing. The uncolored nodes
      assert `--color-border-strong` as a flat `rgb()`, which doubles as
      the runtime proof that node gradients stay off — a gradient would
      read `url(#…-gradient)`.
- [x] **Decision not in the original task list: pin `look: "classic"`.**
      v12 moved the default from `classic` to `neo`, which rounds node
      corners, thickens strokes, and adds a drop shadow. tokens.css
      wipes the radius scale — sharp corners everywhere, `rounded-pill`
      the only exception — so `neo` would make diagrams the single
      rounded surface on the site. Adopting the new appearance defaults
      was also a *distinct* [OQ-1](#oq-1-to-oq-10-answered) alternative
      from the one chosen, so inheriting them would quietly decide a
      question that was answered the other way. Both looks were rendered
      and compared on the specimen before choosing. Flipping is one
      line (`MERMAID_LOOK`) and is asserted by a test so it cannot drift
      back.
- [x] Run `just bundle-budget`. The eager budget should be untouched
      (mermaid is behind a dynamic import), but record the new
      diagram-page chunk cost — ELK is roughly 500 KB gzipped in the
      inlined build. **Eager total 122.4 KB gz against the 130 KB
      budget**, up 0.2 KB from 122.2 — noise, and no mermaid or ELK
      chunk is in the eager set.
      The diagram-page cost is the real number to hold onto: ELK lands
      as `assets/elk-<hash>.js`, **1.39 MB raw / 436 KB gzipped**, which
      did not exist before this upgrade. It is fetched only by a
      document containing a diagram, and only when `layout` is `elk` —
      mermaid's layout loader imports it on demand — so Phase 5's
      `dagre` override should skip it entirely. Verify that there rather
      than assuming it.
- [x] Confirm `import("mermaid")` still resolves to the package
      specifier. Upstream notes `dist/mermaid.esm.min.mjs` now contains
      syntax `es-module-lexer` (which Vite uses) rejects; we are
      unaffected only because we import the bare specifier. Do not
      deep-path into `dist`. Confirmed: 12.0.0's `exports["."]` maps to
      `dist/mermaid.core.mjs`, a different file from the minified one,
      and the only mermaid specifiers in the repo are bare. Since "do
      not deep-path" is a rule a future edit can break silently, it is
      now an ESLint `no-restricted-imports` pattern on `mermaid/*` with
      the reason in the message — checked to fire before being trusted.
- [x] Set an explicit `build.target` in `vite.config.ts` matching
      mermaid's ES2024 / Safari 17.4 floor, and state the supported
      browsers in `README.md` (OQ-4). The floor exists whether or not it
      is written down; unstated, it arrives as a bug report from an old
      iPad. Written as browser versions
      (`chrome119`, `edge119`, `firefox122`, `safari17.4` — the first
      releases with full ES2024 support, Safari being the binding one)
      rather than the string `es2024`, so `cssTarget`, which inherits
      from `build.target`, gets something it can act on. Confirmed the
      setting is live rather than assumed: building against an ancient
      target moves the entry chunk from 78.5 to 81.7 KB gz, so Vite is
      reading it. At the real floor the output is byte-identical to the
      previous default, which is the expected result — nothing here was
      relying on downlevelling.

#### Success Criteria

- `mermaid` resolves to 12.x in the lockfile.
- All three specimen diagrams render under ELK, monochrome, with
  Figure 2's `classDef` colors intact.
- The `secure`-list question is answered in writing, and if the gap is
  real it is closed and covered by an XSS-suite row.
- The XSS suite and the e2e hostile-label row are green, and the
  rendered SVG contains no element originating in document text.
- `just e2e` green, including the widened chunk assertion.
- `just bundle-budget` green; eager total unchanged from 122.2 KB except
  for noise.
- Before/after screenshots of the specimen page attached to the PR. This
  is the only check that answers "do the diagrams still look right", so
  it is recorded rather than automated.
  **Met in substance, not in form.** All three figures were rendered and
  inspected individually under v12 with ELK, at both `look: "neo"` and
  `look: "classic"` — that comparison is what decided the pin. The
  images could not be uploaded from where the work ran, so
  [a PR comment][shots] records what was compared and how to reproduce
  it. Two assertions now cover what the screenshots could only show:
  computed `stroke` on Figure 2's coloured and uncoloured nodes, and
  every theme variable surviving the merge — the latter matters
  precisely because an unknown variable blanks a figure *silently*,
  which is the failure a screenshot review is most likely to skim past.

---

### Phase 5: Deployment-selectable diagram layout

ELK is the default, but a deployment must be able to select `dagre`
without rebuilding the image. This follows the runtime-config pattern
already established twice — `DOCZ_AUTH_PROVIDERS` and `DOCZ_NAV_LINKS` —
and must follow it exactly, including the both-ends validation rule.

#### Tasks

- [x] Add `resolveMermaidLayout(raw)` to `server/serve.ts`,
      whitelist-validating `DOCZ_MERMAID_LAYOUT` against the closed set
      `dagre | elk`. Anything else, including empty, resolves to the
      default. **Only whitelist values may reach the injected
      `<script>`** — this is the rule that keeps raw env out of inline
      script, and it is not negotiable.
- [x] Extend `runtimeConfigScript` to publish the value on
      `window.__DOCZ_CONFIG__`, keeping the existing `</` escaping.
      Signature moved from positional arguments to a `RuntimeConfig`
      object on the way past. Three independent values were one too many
      for positional order to stay readable, and the parameter now
      mirrors the JSON it produces. The keys are still written out
      explicitly inside rather than stringifying the argument, so the
      emitted order cannot depend on how a caller built the object.
- [x] Add `src/lib/mermaidLayout.ts` that **re-validates** the injected
      value against the same closed set, then falls back to a build-time
      `VITE_MERMAID_LAYOUT`, then `elk`. The both-ends rule: neither end
      trusts the other. One deliberate difference from nav pins: an
      injected value that fails validation is **not** authoritative, it
      falls through. Pins respect "the deployment chose no pins"; there
      is no equivalent here, so a bad value means something is wrong
      rather than that something was chosen. `VITE_MERMAID_LAYOUT` is
      declared in `src/vite-env.d.ts` as a plain string, so the call
      site passes typed input to the validator instead of casting.
- [x] Read it in `getMermaid()` (`src/markdown/mermaid-block.tsx`) when
      calling `initialize()`. Note the module memoizes `mermaidPromise`,
      so the layout resolves once per page load, which is correct — the
      value cannot change without a new document. A test injects
      `dagre` and asserts the shipped config follows, so "reads the
      resolver" cannot quietly decay back into a constant.
- [x] Add `config.mermaidLayout` to `charts/docz-site/values.yaml` and
      wire the `DOCZ_MERMAID_LAYOUT` env in
      `charts/docz-site/templates/deployment.yaml`, following how
      `authProviders` is templated (always set) rather than `navLinks`
      (omitted when empty). Also constrained in
      `values.schema.json` to the enum `dagre | elk`, which was not in
      the plan and earns its place: the server falls back to `elk` on
      anything it does not recognize, so without the schema a typo in a
      values file is *invisible* — diagrams keep rendering the default
      and nothing reports a problem. With it, `helm template` fails
      immediately. Verified by running one.
- [x] Regenerate the chart README (`just helm-docs`) and add a
      helm-unittest case asserting the env var renders for both values.
      `just helm-lint`, `just helm-unittest` (36 tests), and
      `just helm-template` all pass.
- [x] Add `server/serve.test.ts` cases (runs under `bun test server/`,
      not vitest): valid values pass through, unknown values and empty
      fall back, and no unvalidated string can reach the script.
- [x] Add `src/lib/mermaidLayout.test.ts` covering the injected-value,
      build-time-fallback, and default paths, plus a hostile injected
      value.
- [x] Decide whether `build:msw` bakes a layout for e2e, as it bakes an
      RFCs nav pin. Default behavior is the thing e2e should exercise,
      so probably not — but make it a decision, not an oversight.
      **Decided: it does not.** The nav pin is baked because there is no
      other way to reach that surface — an unpinned build renders
      nothing to assert on. The layout is the opposite: it has a real
      default, that default is the thing worth exercising, and baking
      `dagre` would quietly stop covering ELK.
      The override is still exercised end to end, by setting
      `window.__DOCZ_CONFIG__` in a Playwright init script. That is more
      faithful than a build-time variable would have been: it is the
      exact channel `server/serve.ts` injects on, so the test covers the
      path a deployment actually takes rather than the fallback. It
      asserts all three specimen diagrams render **and** that the ELK
      chunk is never fetched — which is what makes the 436 KB gzipped
      layout engine a cost only ELK deployments pay, rather than a
      claim about mermaid's loader that this repo never checked.

#### Success Criteria

- `DOCZ_MERMAID_LAYOUT=dagre` on the container changes rendered diagram
  layout with no rebuild; unset yields ELK.
- An invalid value falls back to ELK and never appears in the page
  source.
- `just test-server`, `just helm-lint`, `just helm-unittest`, and
  `just helm-template` all pass.
- The chart README documents the new value.

---

### Phase 6: Guards, docs, release

#### Tasks

- [x] Update `CLAUDE.md`: the mermaid paragraph (ELK default, the
      override, any moved invariant), the directory paragraph (ordering
      and the source filter), and the updated-column paragraph, which
      currently documents the field as absent and the upstream ask as
      open. A fourth paragraph needed it too and was not in the plan —
      the published-pages one still claimed "searchDocs has NO source
      filter param yet (additive upstream ask)" and described
      `useRepoFacts.total` reading `facets.source.doc`. Both were
      falsified in Phase 3.
- [x] Add the ninth amendment to DESIGN-0005 (OQ-8) recording the ELK
      adoption, the override, and whatever the `secure`-list audit
      found. It also closes the eighth amendment's loose end — that
      section ends "until it lands", describing `hitUpdatedAt` and the
      em dash — and corrects its parting claim that page hits send `""`
      for `updated_at`; it is `created` that is empty on a page.
      DESIGN-0004 carried the same two expired premises in its search
      component and gets a dated update note rather than an edited body:
      it is a record of what was decided when, not a description of
      today.
- [x] Add a specimen section if v12 introduces a construct worth
      rendering — the standing rule is that a pipeline feature lands
      with a specimen section in the same commit. Use-case diagrams are
      new in v12. **Evaluated and declined, for two reasons rather than
      for convenience.** The construct was built and rendered first: a
      `usecase-beta` figure with two actors and a `systemBoundary` does
      render under strict mode with `htmlLabels: false`, monochrome and
      in keeping with the rest of the page. But every ellipse label sits
      right of its shape's centre, and a label wide enough relative to
      its ellipse spills outside it — reproduced under **both** ELK and
      dagre, so it is the use-case renderer's own measurement and not
      a consequence of the layout adopted here. The specimen is a real
      published page and the reference for visual judgement; a figure
      with text outside its shape does not belong in it. Second, the
      keyword still carries `-beta`, so the syntax is expected to move,
      and the specimen is asserted on in e2e. Revisit when the shape
      lands non-beta. Nothing else in v12 adds a construct this pipeline
      renders differently.
- [x] Close [#29] and [#30] with a pointer to the merge commit. Both are
      `Closes #NN` in [PR #31][pr], so GitHub closes them on merge and
      records the merge commit itself as the closer — a more durable
      pointer than a comment written by hand before the SHA exists.
- [x] Regenerate `CHANGELOG.md` with `git fetch --tags` first, commit as
      `chore(changelog): Auto-sync`, and make it the **last** commit on
      the branch.
- [x] Apply the `minor` label (OQ-2) so the merge cuts v0.8.0. Applied
      at PR creation; it is the only release label on the PR, which is
      what the release workflow requires.
- [x] Flip this document's status to Completed and tick every box.

#### Success Criteria

- `just ci` passes end to end.
- No comment or document in the repo still describes
  `SearchHit.updated_at` as missing or the sort/source asks as open.
- Both issues closed, changelog clean, exactly one release label.

---

## File Changes

| File | Action | Description |
| ---- | ------ | ----------- |
| `api/openapi.yaml` | Modify | Re-vendor at 1.5.0 |
| `src/api/__generated__/` | Regenerate | orval output; gitignored, never hand-edited |
| `src/api/fetcher.ts` | Modify | `BadRequestError` for the new 400 |
| `src/app/query-client.ts` | Modify | Add it to the no-retry predicate |
| `src/lib/updatedAt.ts` | Modify | Delete `hitUpdatedAt`; rewrite the now-false comment |
| `src/lib/updatedAt.test.ts` | Modify | Replace the "field is absent" cases |
| `src/lib/searchParams.ts` | Modify | `sort` and `source` in URL state and query params |
| `src/lib/searchParams.test.ts` | Modify | Parse/serialize/round-trip for the new keys |
| `src/lib/mermaidLayout.ts` | Create | Re-validate the injected layout; both-ends rule |
| `src/lib/mermaidLayout.test.ts` | Create | Injected, build-time, default, hostile paths |
| `src/routes/directory.tsx` | Modify | Query-aware ordering, source filter, comment fixes |
| `src/routes/directory.test.tsx` | Modify | Dated page rows; ordering assertions |
| `src/mocks/fixtures.ts` | Modify | `created`, page timestamps, honor `sort`/`source`, emit 400 |
| `src/markdown/mermaid-block.tsx` | Modify | ELK default, layout override, `secure` list, v12 theme keys |
| `src/markdown/mermaid-config.test.ts` | Create | The OQ-11 audit, kept as a test (real mermaid, merged config) |
| `src/mocks/browser.ts` | Modify | Second e2e figure whose front matter attacks the `secure` list |
| `src/a11y/axe.test.tsx` | Modify | Confirm the mermaid mock matches the v12 module shape |
| `server/serve.ts` | Modify | `DOCZ_MERMAID_LAYOUT` whitelist + injection |
| `server/serve.test.ts` | Modify | Validation and fallback cases |
| `e2e/rendering.spec.ts` | Modify | Widen the chunk assertion to cover ELK |
| `package.json`, `bun.lock` | Modify | `mermaid ^12.0.0` |
| `vite.config.ts` | Modify | Explicit `build.target` for the ES2024 floor |
| `charts/docz-site/values.yaml` | Modify | `config.mermaidLayout` |
| `charts/docz-site/templates/deployment.yaml` | Modify | `DOCZ_MERMAID_LAYOUT` env |
| `charts/docz-site/README.md` | Regenerate | `just helm-docs` |
| `charts/docz-site/tests/` | Modify | helm-unittest case for the new env |
| `docs/design/0005-*.md` | Modify | Ninth amendment: ELK, the override, the audit result |
| `CLAUDE.md` | Modify | Mermaid, directory ordering, updated-column paragraphs |
| `README.md` | Modify | Supported-browser statement |
| `docs/guides/markdown-specimen.md` | Modify | Only if a v12 construct is worth rendering |

## Testing Plan

- [x] `src/lib/searchParams.test.ts` — table-driven cases for `sort` and
      `source`: parse, serialize, round-trip, default omission, and a
      value outside the enum.
- [x] `src/lib/updatedAt.test.ts` — real 1.5.0 shapes, including `""`
      for an unset stamp and `created: ""` on page hits.
- [x] `src/routes/directory.test.tsx` — ordering flips with the query;
      both record kinds render a date; "load more" does not reshuffle; a
      filter change pushes history.
- [x] Fixture-level test that the MSW resolver honors `sort`, including
      the sorts-last-in-both-directions quirk. Without this the fixture
      diverges silently from the real API.
- [x] `src/lib/mermaidLayout.test.ts` — injected value, build-time
      fallback, default, and a hostile injected value.
- [x] `server/serve.test.ts` — `DOCZ_MERMAID_LAYOUT` validation and
      fallback. Runs under `bun test server/`, outside the vitest graph.
- [x] `src/markdown/processor.xss.test.tsx` — existing rows green under
      v12.
- [x] `src/markdown/mermaid-config.test.ts` — hostile front matter
      cannot re-enable `htmlLabels` at either path or lower
      `securityLevel`, a non-secure key still applies (non-vacuity), and
      the effective `secure` list stays a superset of whatever the
      installed mermaid protects.
- [x] `e2e/rendering.spec.ts` — hostile-label rows green under v12,
      including the front-matter figure; widened chunk assertion; all
      three specimen fences render.
- [x] `e2e/a11y.spec.ts` — full-rule axe over the specimen with real v12
      diagrams, contrast included.
- [x] `just helm-unittest` — the new env renders for both layout values.
- [x] `just bundle-budget` — eager total unchanged (122.5 KB gz).
- [x] Manual: `bun run dev:msw` for ordering and dates; the specimen for
      diagrams; a container run with `DOCZ_MERMAID_LAYOUT=dagre` to
      prove the escape hatch. Screenshots to the PR. The escape hatch
      was proven against `server/serve.ts` itself — the same entrypoint
      the image runs, driven by the same env — rather than through a
      container, which would have added a build step and tested nothing
      further. Unset injects `"mermaidLayout":"elk"`, `dagre` injects
      `"dagre"`, and `elk</script><script>alert(1)</script>` injects
      `"elk"` with the hostile string absent from the page source
      entirely.

## Dependencies

- **docz-api v0.10.0** carries spec 1.5.0 (as does v0.9.1). The earlier
  concern about shipping ahead of a release is resolved: the release
  exists. Worth knowing anyway — a deployment running an older image
  omits both new fields, and the existing `Number.isNaN` guard in
  `formatUpdatedStamp` already degrades that to the em dash rather than
  "Invalid Date". Keep that guard.
- `mermaid@12.0.0` (published 2026-09-10) and its bundled ELK.
- No docz-api change is required.

## Open Questions

Answer format: **a** is the recommendation, **b** onward are the
alternatives, and *other* is free text. All questions were answered on
2026-09-14 and are summarized in [Decisions](#decisions); the
alternatives are kept below for the record.

### OQ-11 (answered: b)

**Can a document's own front matter re-enable HTML labels?** Raised
while planning Phase 5. Not yet verified, and it may describe code we
ship *today* rather than anything v12 introduces.

Mermaid filters diagram-supplied config through a `secure` list. In the
installed 11.16.0 the default list is:

```text
["secure", "securityLevel", "startOnLoad", "maxTextSize",
 "suppressErrorRendering", "maxEdges"]
```

`htmlLabels` is not in it. If diagram front matter can set
`htmlLabels: true` — globally or nested under `flowchart` — then a
document could restore the exact vector that setting exists to close,
with `securityLevel: "strict"` still in force and still insufficient on
its own. `securityLevel` *is* secured, so this would not be a full
escape; it would be the tracking-pixel class of problem documented in
`mermaid-block.tsx`.

What is established: the default `secure` list, read from the installed
package. What is not: whether front-matter config actually reaches the
`htmlLabels` merge, and whether a top-level `secure` entry also covers
the nested `flowchart.htmlLabels` path. Both need a test, not a reading.

#### Finding (2026-09-14, mermaid 11.16.0, before the v12 bump)

**Confirmed. The gap was real and is now closed.** Both open parts were
settled by running the shipped config against hostile front matter and
reading the merged config back out:

| Front matter under…       | `htmlLabels` | `flowchart.htmlLabels` | `securityLevel` |
| ------------------------- | ------------ | ---------------------- | --------------- |
| mermaid's default `secure` | `true`      | `true`                 | `strict`        |
| `MERMAID_SECURE_KEYS`      | `false`     | `false`                | `strict`        |

So a document *could* re-enable HTML labels at both paths, and one
top-level `secure` entry closes both — mermaid's directive sanitizer
recurses into nested config objects, deleting secure keys at every
level. `securityLevel` was never reachable, which is what made the
mechanism legible: it is in the default list, and it held.

The path, read from the shipped bundle and then exercised:
`render` → `processAndSetConfigs` → `preprocessDiagram` (extracts the
front-matter `config:`) → `addDirective` → `sanitizeDirective` (drops
keys outside the config schema) → `updateCurrentConfig` → `sanitize`
(deletes `secure`-listed keys, recursively) → merged over the site
config.

Two things worth carrying forward. Mermaid *unions* a supplied `secure`
array with its defaults rather than replacing it, so a one-element
addition would work today — but that is an implementation detail, and
under clobber semantics it would silently drop `securityLevel` from the
protected set. `MERMAID_SECURE_KEYS` therefore restates the defaults in
full, and the test asserts the **effective** list is a superset of
whatever the installed mermaid protects, so a future version adding a
key fails CI either way. Second, `mermaidAPI` is deprecated but is the
only handle on the merged config; it appears in the test only, never in
shipped code.

Covered by `src/markdown/mermaid-config.test.ts` (config-level, jsdom)
and by a second figure in the e2e rendering fixture whose front matter
tries the same thing (rendered-output level, real browser).

- **a.** Verify it first, as a standalone `patch` PR ahead of this
  branch, and if confirmed, ship the `secure`-array fix plus an XSS-suite
  row on its own. *Was the recommendation:* if the gap is real it affects
  production now, and a security fix should not wait behind a dependency
  upgrade or arrive buried in a 6-phase diff.
- **b. → CHOSEN.** Verify and fix inside Phase 4, as written. The gap,
  if real, has been shipped since IMPL-0002 and carries no new urgency
  from the upgrade; folding it in keeps one PR rather than two.
  Consequence to hold onto: the audit is the **first** task of Phase 4,
  before the version bump, so the answer is established against the
  version actually in production. If it turns out to be exploitable,
  reconsider splitting it out rather than letting a live fix wait on ELK
  review.
- **c.** Verify only, record the finding, and decide afterward.
- **d.** Treat it as acceptable: `securityLevel: "strict"` still applies
  and the residual risk is a purified `<img>`, so no change.

### OQ-1 to OQ-10 (answered)

Retained so the decisions have their alternatives on record.

- **OQ-1 — mermaid layout.** *Chosen: adopt ELK plus a deployment
  override.* Alternatives: pin dagre and evaluate ELK separately; adopt
  ELK with no override; adopt ELK and the new appearance defaults too;
  pin dagre permanently.
- **OQ-2 — release label.** *Chosen: `minor`, v0.8.0.* Alternatives:
  `patch`; `dont-release`.
- **OQ-3 — default ordering.** *Chosen: `updated_at:desc` only when the
  query is empty.* Alternatives: always sort, as [#29] literally asks;
  never sort; expose a user control.
- **OQ-4 — ES2024 / Safari 17.4 floor.** *Chosen: explicit
  `build.target` plus a documented browser floor.* Alternatives: leave
  the Vite default; transpile mermaid down.
- **OQ-5 — the new 400.** *Chosen: a `BadRequestError` class on the
  no-retry list.* Alternatives: generic `ApiError`; broaden the retry
  predicate to all 4xx.
- **OQ-6 — `created` on the card.** *Chosen: no.* Alternatives: replace
  `updated_at` with it; show both; show it in the reader's metadata
  table.
- **OQ-7 — `hitUpdatedAt`.** *Chosen: delete it.* Alternatives: keep as
  a one-line accessor; keep but normalize `""` to `undefined`.
- **OQ-8 — where the layout decision lives.** *Chosen: ninth amendment
  to DESIGN-0005.* Alternatives: a new DESIGN doc; `CLAUDE.md` only.
- **OQ-9 — one PR or two.** *Chosen: one PR, mermaid last.*
  Alternatives: two stacked PRs; defer mermaid entirely.
- **OQ-10 — shipping ahead of a docz-api release.** *Resolved by fact:
  v0.10.0 carries 1.5.0.* Alternatives are moot.

## References

- [#29] — docz-api spec 1.5.0: dated, sortable, source-filterable search
  hits
- [#30] — Upgrade mermaid to v12
- donaldgifford/docz-api#34 — the original ask, closed by
  donaldgifford/docz-api#37, released in v0.9.1 and v0.10.0
- docz-api `docs/design/0005-timestamped-and-sortable-search-hits.md`
- [mermaid 12.0.0 release](https://github.com/mermaid-js/mermaid/releases/tag/mermaid%4012.0.0)
- [mermaid#8155](https://github.com/mermaid-js/mermaid/pull/8155) — ELK
  bundled and made the default layout
- [mermaid#8213](https://github.com/mermaid-js/mermaid/pull/8213) —
  ES2024, Safari 17.4+, Node 22.12+
- DESIGN-0005 — reader typography, monochrome diagram policy, `classDef`
  precedence
- DESIGN-0002 / IMPL-0003 — the `DOCZ_NAV_LINKS` runtime-config pattern
  Phase 5 follows
- IMPL-0005 — published pages, where the em-dash updated column
  originated

[pr]: https://github.com/donaldgifford/docz-site/pull/31
[shots]: https://github.com/donaldgifford/docz-site/pull/31#issuecomment-5681592757
[#29]: https://github.com/donaldgifford/docz-site/issues/29
[#30]: https://github.com/donaldgifford/docz-site/issues/30
