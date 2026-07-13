---
id: IMPL-0002
title: "Reader polish: rendering pipeline and QoL backlog from INV-0001"
status: Draft
author: Donald Gifford
created: 2026-07-12
---
<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL 0002: Reader polish: rendering pipeline and QoL backlog from INV-0001

**Status:** Draft
**Author:** Donald Gifford
**Date:** 2026-07-12

<!--toc:start-->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: Quick QoL wins](#phase-1-quick-qol-wins)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2: Code block chrome](#phase-2-code-block-chrome)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 3: GitHub alert callouts](#phase-3-github-alert-callouts)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 4: Mermaid diagrams](#phase-4-mermaid-diagrams)
    - [Tasks](#tasks-3)
    - [Success Criteria](#success-criteria-3)
  - [Phase 5: Reader metadata header](#phase-5-reader-metadata-header)
    - [Tasks](#tasks-4)
    - [Success Criteria](#success-criteria-4)
  - [Phase 6: Palette recents and heading anchors](#phase-6-palette-recents-and-heading-anchors)
    - [Tasks](#tasks-5)
    - [Success Criteria](#success-criteria-5)
  - [Phase 7: Docs and closeout](#phase-7-docs-and-closeout)
    - [Tasks](#tasks-6)
    - [Success Criteria](#success-criteria-6)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Dependencies](#dependencies)
- [Open Questions](#open-questions)
- [References](#references)
<!--toc:end-->

## Objective

Work off the INV-0001 follow-up backlog: three rendering-pipeline gaps
surfaced by real RFC content (GitHub alerts, mermaid, code chrome),
the reader metadata-header restructure, and the small QoL items
(palette prefetch, recents, heading anchors, `/login` redundancy,
README refresh). Ordered quick-wins-first, then ascending
pipeline/security risk, so every phase ships independently.

**Implements:** INV-0001 (Recommendation → Follow-up backlog);
prior art from `~/code/rfc-site` `src/portal/markdown/` mapped in
INV-0001 Observation 4.

## Scope

### In Scope

- All nine INV-0001 backlog items, phased below.
- The narrow `schema.ts` widenings each rendering feature needs, with
  the XSS suite growing in the same commit (CI-gated surface).
- A lazy mermaid chunk with a bundle-budget guard — the 130 KB gz
  entry budget must be unaffected.

### Out of Scope

- Anything gated on the DESIGN-0001 additive docz-api asks:
  relationship banners/rows ("applied by →"), tags/labels, lifecycle
  dates, `updated_at` sort. The metadata table lands with the fields
  the Document DTO carries today; gated rows slot in later.
- Server-side rendering of mermaid or Shiki (no SSR in this app).
- docz/docz-api changes of any kind.

## Implementation Phases

Each phase builds on the previous one. A phase is complete when all
its tasks are checked off and its success criteria are met. Per-task
gate as in IMPL-0001: test, lint, `tsc -b --force`, build,
bundle-budget, format:check (e2e per phase), conventional commit per
task, check the box here, update CLAUDE.md when guidance changes.

---

### Phase 1: Quick QoL wins

Two small, independent items — momentum and immediate value while the
pipeline phases get reviewed.

#### Tasks

- [x] Hide the topbar "Sign in" link when already on `/login`
      (`SessionMenu` reads `useLocation`; render nothing — the page IS
      the affordance). Update `session-menu.test.tsx`.
- [x] Palette prefetch: when cmdk's active item changes (keyboard or
      pointer), call `usePrefetchDoc` for the highlighted hit —
      `command-palette.tsx` already resolves the active hit through
      the hits map. Test: highlight → getDoc request observed (mirror
      the repo-nav hover-prefetch test).

#### Success Criteria

- Signed-out `/login` shows exactly one sign-in affordance (the
  provider buttons).
- Arrowing through palette results warms the doc cache: opening the
  highlighted hit renders from cache without a second getDoc in the
  test's MSW request log.

---

### Phase 2: Code block chrome

The mockup's codeblock header — uppercase language badge left,
optional filename right — on every fenced block. Prior art:
rfc-site `wrap-codeblock.ts`, the `pre()` half of its Shiki
transformer, and `capture-code-meta.ts`; our sanitize-first order
means the wrapper itself needs no schema surface.

#### Tasks

- [x] `src/markdown/capture-code-meta.ts` (remark): surface the fence
      meta string as a `metastring` property on the `code` node so it
      survives sanitize as a real property (node `data` does not).
- [x] `schema.ts`: allow `metastring` on `code`, value-validated per
      OQ-6. Extend the XSS suite with hostile meta payloads
      (`onclick=`, quotes, angle brackets) asserting they render as
      inert text or are dropped.
- [x] Shiki transformer in `processor.ts`: stamp `dataLanguage` (from
      `this.options.lang`, skip `text`/`plain`) and `dataCaption`
      (from validated meta) onto the emitted `<pre>`.
- [x] `src/markdown/wrap-codeblock.ts` (rehype, post-Shiki): wrap
      `pre[data-language]` in `div.codeblock` with a
      `div.codeblock-header` (lang span + optional caption span);
      idempotent; skips mermaid-marked blocks (Phase 4 dependency
      noted in-code from day one).
- [x] `MarkdownPre`: region `aria-label` becomes
      `"<lang> code block"` when `data-language` is present.
- [x] `tokens.css`: `.codeblock` / `.codeblock-header` styles ported
      from rfc-site `styles.css` §167–200 onto docz tokens (sharp
      corners — no radius).
- [x] Tests: processor test for header structure (lang only, lang +
      caption, plain fence untouched); axe sweep still green (header
      is decorative text, the region label carries the language).

#### Success Criteria

- RFC-0006's ` ```go ` fence renders with a `GO` badge header; a
  fence with meta renders the caption; plain-text fences stay bare.
- XSS suite green including the new metastring payloads; no other
  schema surface widened.
- Live check on `donaldgifford/rfcs` RFC-0006 shows the header.

---

### Phase 3: GitHub alert callouts

`> [!NOTE|TIP|IMPORTANT|WARNING|CAUTION]` blockquotes become styled
admonitions; plain blockquotes keep the mockup pull-quote look.
Prior art: rfc-site `github-alerts.ts` ports nearly verbatim.

#### Tasks

- [x] `src/markdown/github-alerts.ts` (remark, after gfm): blockquote
      whose first text starts with the alert marker →
      `div.admonition.<kind>` via `hName`/`hProperties`, marker text
      stripped, `span.adm-label` prepended with the kind label. Kind
      set per OQ-2.
- [x] `schema.ts`: allow `div` + `span` with value-RESTRICTED
      classNames (`admonition`, kind names, `adm-label`) — nothing
      else. XSS suite: raw-HTML forged `<div class="admonition">`
      stays inert styling; forged other classes are stripped;
      alert-marker text inside `code`/`pre` is untouched.
- [x] `tokens.css`: admonition styles ported from rfc-site
      `styles.css` §281–360 onto docz tokens (per-kind `color-mix`
      rows against existing status tokens; contrast pairs added to
      `contrast.test.ts` if any new text/bg combination lands).
- [x] Tests: each kind renders label + class; multi-paragraph body;
      nested markdown inside the alert; non-alert blockquote keeps
      pull-quote rendering; marker mid-document (not first line) is
      NOT lifted.

#### Success Criteria

- RFC-0002's `[!WARNING]` and `[!CAUTION]` render as visually
  distinct callouts with no leaked marker text, verified live.
- Plain blockquotes are visually unchanged from before the phase.
- XSS + contrast + axe gates green.

---

### Phase 4: Mermaid diagrams

` ```mermaid ` fences render as diagrams via a lazy chunk. Prior art:
rfc-site `mermaid-marker.ts` + `mermaid-hydrate.ts` — but our
hast-to-JSX render routes marked blocks to a React component instead
of DOM-query hydration. Carries rfc-site's lessons: SVG cache keyed
by source (StrictMode double-render flash), minimal documented
`themeVariables` (extras silently break `mermaid.render`),
token-driven theme via `getComputedStyle` with fallbacks.

#### Tasks

- [ ] Add `mermaid` dependency (pinned); confirm the chunk NEVER
      lands in the entry: `bundle-budget` must not move.
- [ ] `src/markdown/mermaid-marker.ts` (rehype, post-sanitize,
      pre-Shiki): `code.language-mermaid` → set `dataMermaidSource`
      on the `pre`, strip the language class (Shiki never sees it) —
      zero schema change, runs on trusted structure.
- [ ] `src/markdown/mermaid-block.tsx`: `MarkdownPre` routes
      `data-mermaid-source` pres to `<MermaidBlock source=…>`; lazy
      `import("mermaid")` on first mount; module-scope SVG cache;
      injection mechanism per OQ-1; failure path keeps the source
      text visible as a code block.
- [ ] Theme: `mermaidThemeFromTokens()` reading docz tokens
      (bg-raised/bg-elevated/border-strong/fg-primary/fg-tertiary)
      with hex fallbacks, minimal variable set only.
- [ ] A11y: rendered diagram gets `role="img"` + an `aria-label`
      (first line of the source or fence caption); axe sweep grows a
      mermaid fixture.
- [ ] XSS suite: hostile mermaid source (HTML/script in labels,
      `click` directives) — assert nothing executes and hostile
      labels render inert under the chosen securityLevel.
- [ ] Tests: marker transform unit tests; MermaidBlock renders
      fallback in jsdom (mermaid needs real SVG measurement — mock
      the import); e2e: RFC-0006-shaped fixture renders an `<svg>` in
      the preview build.

#### Success Criteria

- RFC-0006's diagram renders as themed SVG live; a syntactically
  broken diagram shows its source, never a blank box.
- `bundle-budget` unchanged (entry stays ~118 KB gz); mermaid loads
  ONLY on docs containing a mermaid fence (network assertion in e2e).
- XSS suite green with the mermaid payload rows.

---

### Phase 5: Reader metadata header

Long right-rail ToCs bury metadata and formats (INV-0001 backlog
item 1, mockup doc-portal treatment): a bordered key/value table
under the title/status header, formats moved beside it, lifecycle
demoted from always-on rail block.

#### Tasks

- [ ] `src/components/doc-meta-table.tsx`: bordered two-column table
      under the doc header — fields per OQ-3 (`""`-omitting like the
      current rail metadata); repo · dir and source path rows; xref
      styling for values that are links.
- [ ] Move the format switch (html/md/json) into the header area per
      OQ-5; keep deep-linkable behavior identical.
- [ ] Lifecycle per OQ-4 (default from the OQ answer); reuse the
      disclosure pattern from the nav drawers if a drawer.
- [ ] Slim the right rail to ToC-first; remove the now-duplicated
      metadata block; keep the rail's 1181px collapse behavior.
- [ ] Update `doc.test.tsx` (four-state matrix + metadata omission
      tests move to the table), axe sweep (table semantics), and the
      e2e reader journey if selectors moved.

#### Success Criteria

- On a long doc (RFC-0006 or DESIGN-0001), metadata and formats are
  visible without scrolling the rail; ToC occupies the rail.
- Empty-string metadata fields still render nothing (existing
  contract), and every existing reader test passes migrated.
- axe: table has proper header semantics; zero serious/critical.

---

### Phase 6: Palette recents and heading anchors

#### Tasks

- [ ] `src/lib/recentDocs.ts`: localStorage-backed list per OQ-7
      (validated on read — malformed JSON or wrong shape resets);
      recorded on successful doc load in the reader route.
- [ ] Palette empty-query state: "recent" group above the default
      results, rendered through the existing hit components; entries
      prefetch like other items (Phase 1 plumbing).
- [ ] Heading anchor copy per OQ-8 in the reader prose (h2–h4, the
      ToC-collected set); clipboard write via `navigator.clipboard`
      with a fallback; announced for screen readers.
- [ ] Tests: recents record/evict/validate; palette shows recents on
      empty query only; anchor copy puts the absolute URL on the
      clipboard (jsdom clipboard stub) and announces.

#### Success Criteria

- Open three docs → palette empty state lists them most-recent-first;
  a stored entry that no longer resolves is dropped, not rendered
  broken.
- Only UI-preference data in storage (doc coordinates and titles —
  never tokens); test setup still clears storage between tests.
- axe green with the anchor affordance present.

---

### Phase 7: Docs and closeout

#### Tasks

- [ ] README refresh: Phase-5 auth (`VITE_AUTH_PROVIDERS`), local
      loop (`just local-up`/`local-down`), deploy compose story, and
      the rendering features from this plan.
- [ ] Check off the corresponding INV-0001 backlog boxes; flip
      INV-0001 status to Concluded with a closing note.
- [ ] CLAUDE.md: final guidance pass (markdown pipeline stages now
      include alerts/chrome/mermaid; schema.ts widening rules).
- [ ] Changelog regen; stacked-or-single PR per the merge-order
      lesson from IMPL-0001 (single branch preferred).

#### Success Criteria

- `just ci` green end-to-end on the final branch; PR checks green
  including CodeQL.
- INV-0001 backlog fully checked; no doc references a feature that
  doesn't exist.

---

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `src/components/session-menu.tsx` | Modify | No "Sign in" link on `/login` (Phase 1) |
| `src/components/command-palette.tsx` | Modify | Active-item prefetch; recents group (Phases 1, 6) |
| `src/markdown/capture-code-meta.ts` | Create | Fence meta → validated `metastring` property (Phase 2) |
| `src/markdown/wrap-codeblock.ts` | Create | Post-Shiki `div.codeblock` header chrome (Phase 2) |
| `src/markdown/github-alerts.ts` | Create | Alert blockquotes → admonition divs (Phase 3) |
| `src/markdown/mermaid-marker.ts` | Create | Post-sanitize mermaid tagging (Phase 4) |
| `src/markdown/mermaid-block.tsx` | Create | Lazy client mermaid render + theme + cache (Phase 4) |
| `src/markdown/schema.ts` | Modify | `metastring` on code; restricted admonition classes (Phases 2–3) |
| `src/markdown/processor.ts` | Modify | Transformer + plugin wiring (Phases 2–4) |
| `src/markdown/markdown-pre.tsx` | Modify | Language-aware label; MermaidBlock routing (Phases 2, 4) |
| `src/theme/tokens.css` | Modify | codeblock + admonition styles (Phases 2–3) |
| `src/components/doc-meta-table.tsx` | Create | Header metadata table (Phase 5) |
| `src/routes/doc.tsx` | Modify | Header restructure; recents recording (Phases 5–6) |
| `src/components/doc-rail.tsx` | Modify | ToC-first rail; lifecycle per OQ-4 (Phase 5) |
| `src/lib/recentDocs.ts` | Create | Validated localStorage recents (Phase 6) |
| `README.md` | Modify | Feature/tooling refresh (Phase 7) |

## Testing Plan

- [ ] XSS suite grows with EVERY schema widening in the same commit:
      metastring payloads (Phase 2), forged admonition markup
      (Phase 3), hostile mermaid source (Phase 4).
- [ ] Processor unit tests per pipeline stage; live-shape fixtures
      copied from RFC-0002/RFC-0006 content patterns.
- [ ] Bundle budget asserted every phase; Phase 4 adds an explicit
      "mermaid not in entry chunk" check.
- [ ] axe sweep additions: codeblock header, admonitions, mermaid
      figure, metadata table, anchor affordance.
- [ ] e2e: rendering journey over a fixture doc containing alerts +
      code chrome + mermaid; reader metadata header visible on deep
      link.

## Dependencies

- `mermaid` (new runtime dep, lazy chunk only — Phase 4).
- rfc-site checkout at `~/code/rfc-site` as reference (read-only).
- No docz-api changes; gated rows wait on DESIGN-0001 asks.

## Open Questions

Numbered for review; **a** is my recommendation, later letters are
alternatives. Answer with the letter (or write in "other").

> **Answered 2026-07-12: all questions resolved as option a.** The
> **a** options below are the decisions of record; phases implement
> them as written.

**OQ-1 — Mermaid SVG injection mechanism.** Mermaid returns an SVG
string; putting it in the DOM conflicts with the repo rule "no
`dangerouslySetInnerHTML` anywhere".
  - **a (recommended):** Narrow documented exception in
    `MermaidBlock` only: ref-based `innerHTML` of
    `mermaid.render()` output with `securityLevel: "strict"` (labels
    encoded, click directives disabled), hostile-source rows in the
    XSS suite, and the CLAUDE.md rule amended to name this single
    carve-out. Rationale: the string comes from mermaid's renderer,
    not from document HTML; strict mode encodes user text; rfc-site's
    "strict doesn't work" note actually described `sandbox` mode.
  - **b:** `securityLevel: "sandbox"` — mermaid wraps the render in a
    sandboxed `<iframe srcdoc>`; strongest isolation, but styling/
    sizing inside the iframe fights the token theme and the a11y
    tree is opaque.
  - **c:** Parse mermaid's SVG output through rehype-parse +
    rehype-sanitize with an SVG-scoped schema, then hast-to-JSX like
    the rest of the pipeline. Purest, but an SVG sanitize schema is a
    large new attack-surface spec to maintain (foreignObject, xlink,
    style) and mermaid output changes across versions.
  - **d:** Defer mermaid entirely (render fences as code with a
    "diagram not rendered" hint).

**OQ-2 — Alert kind set.** GitHub defines five kinds; rfc-site
normalized IMPORTANT→note and shipped four visuals.
  - **a (recommended):** All five kinds, five visuals, colors mapped
    onto existing status tokens (note→proposed blue, tip→accepted
    green, important→accent purple, warning→draft amber,
    caution→rejected red). No new color tokens; contrast test covers
    any new text-on-tint pairs.
  - **b:** rfc-site's four-kind normalization (IMPORTANT renders as
    note) — smaller CSS, loses GitHub parity.

**OQ-3 — Metadata table fields (Phase 5).**
  - **a (recommended):** Ship now with what the Document DTO carries:
    status, author, created, repo · docs_dir, source path, git_sha
    (short), doc id/type — `""`-omitting rows. Relationship rows
    ("applied by →"), tags, and lifecycle dates slot in when the
    DESIGN-0001 API asks land.
  - **b:** Wait for the API asks and build the full mockup table
    once.

**OQ-4 — Lifecycle placement (Phase 5).**
  - **a (recommended):** Closed-by-default disclosure directly under
    the metadata table (same pattern as the nav type drawers), out of
    the rail entirely.
  - **b:** A single compact row inside the metadata table ("stage 3
    of 5 · Accepted") expanding on click.
  - **c:** Keep it in the rail but below the ToC, still
    always-open.

**OQ-5 — Format switch placement (Phase 5).**
  - **a (recommended):** Right-aligned in the metadata header block
    (same visual row as the table's top edge) — visible without
    scrolling on every doc.
  - **b:** Keep in the rail but pinned above the ToC.
  - **c:** Both (header + rail duplicate).

**OQ-6 — Code caption meta syntax (Phase 2).**
  - **a (recommended):** Treat the whole trimmed fence meta as the
    caption, validated to a conservative charset
    (`[\w ./#+:=@()-]`, length-capped); anything failing validation
    drops the caption (never the block). Matches rfc-site and
    renders ` ```go internal/ingest/parse.go ` directly.
  - **b:** Parse only `title="…"`/`filename=…` keys — stricter
    grammar, but real-world fences (like the mockup's) use bare
    paths.

**OQ-7 — Palette recents storage (Phase 6).**
  - **a (recommended):** `localStorage` `docz:recent-docs`, JSON
    array of `{repo,type,docId,title}` capped at 8, most-recent
    first, validated + reset on malformed read. Survives restarts;
    it's cross-session "recent", matching editor muscle memory.
  - **b:** `sessionStorage` (per-tab, dies with the tab) — more
    conservative, less useful.

**OQ-8 — Heading anchor affordance (Phase 6).**
  - **a (recommended):** Hover/focus-revealed `#` button appended to
    h2–h4 that copies the absolute section URL and flashes
    "copied"; a real button with `aria-label="Copy link to
    section"`, keyboard reachable. Prose links stay underlined-only
    (the button isn't a link, so link-in-text-block is unaffected).
  - **b:** GitHub-style: the heading itself becomes a link wrapping
    an anchor icon — simpler, but heading-as-link changes the axe
    surface and the reading experience.

## References

- INV-0001 — Reader UX polish (Findings → Observation 4; backlog)
- DESIGN-0001 — reader pipeline decisions (sanitize order, tokens)
- IMPL-0001 — per-task gate + phase conventions this doc reuses
- rfc-site prior art: `src/portal/markdown/plugins/github-alerts.ts`,
  `wrap-codeblock.ts`, `mermaid-marker.ts`, `mermaid-hydrate.ts`,
  `pipeline.ts`, `styles.css`
