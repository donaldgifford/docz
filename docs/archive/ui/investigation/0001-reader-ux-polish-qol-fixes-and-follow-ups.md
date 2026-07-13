---
id: INV-0001
title: "Reader UX polish: QoL fixes and follow-ups"
status: Open
author: Donald Gifford
created: 2026-07-12
---
<!-- markdownlint-disable-file MD025 MD041 -->

# INV 0001: Reader UX polish: QoL fixes and follow-ups

**Status:** Open
**Author:** Donald Gifford
**Date:** 2026-07-12

<!--toc:start-->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
  - [Observation 1: the repo nav does not scale past a handful of docs](#observation-1-the-repo-nav-does-not-scale-past-a-handful-of-docs)
  - [Observation 2: the "goofy" repo-home title was not frontmatter](#observation-2-the-goofy-repo-home-title-was-not-frontmatter)
  - [Observation 3: the few-pixel layout shift is the viewport scrollbar](#observation-3-the-few-pixel-layout-shift-is-the-viewport-scrollbar)
  - [Observation 4: rendering gaps on real RFC content](#observation-4-rendering-gaps-on-real-rfc-content)
  - [Also landed in this pass](#also-landed-in-this-pass)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
  - [Follow-up backlog](#follow-up-backlog)
- [References](#references)
<!--toc:end-->

## Question

Dogfooding the site against a real, doc-heavy repository
(`donaldgifford/repo-guardian`, 43 docs) surfaced three papercuts.
Which are real bugs vs. missing polish, what are the root causes, and
what is the remaining quality-of-life backlog worth tracking?

1. The repo nav becomes one long ugly scroll when a repo has many docs.
2. The repo-home title from `index.md` renders as plain small text.
3. Navigating from a type page to a doc shifts the layout a few pixels.

## Hypothesis

Going in: (1) the nav just needed its doc lists collapsed, (2) the
title was suspected to be frontmatter leaking through the renderer,
and (3) the shift was some CSS difference between the two pages. Two
of the three root causes turned out to be different than assumed.

## Context

**Triggered by:** first real-world use after IMPL-0001 merged to main
(PR #7) — browsing `repo-guardian` through the live local stack. The
MVP was built and tested against demo fixtures with 1–2 docs per type;
a 16–18-docs-per-type repo exercised layouts the fixtures never did.

## Approach

1. Reproduce against the live stack (`just local-up`, site :8090 →
   docz-api :8080) rather than fixtures.
2. Pull the actual wire payloads (e.g. `getRepoIndex` for
   `repo-guardian`) to test the frontmatter theory before touching the
   renderer.
3. Diagnose each issue in code, fix, and re-verify with headless
   screenshots against the same live repo plus the full local gate.

## Environment

| Component | Version / Value |
|-----------|----------------|
| docz-site | `main` post-IMPL-0001 (PR #7, `93e93b6`) + `chore/qol-tweaks` |
| docz-api  | local compose stack (`docz-api-local`, :8080) |
| Test repo | `donaldgifford/repo-guardian` — 43 docs (design 18, impl 16, investigation 7, rfc 2) |
| Test repo | `donaldgifford/rfcs` — RFC-0002 (GitHub alerts), RFC-0006 (mermaid + go fences) |

## Findings

### Observation 1: the repo nav does not scale past a handful of docs

`RepoNav` rendered every doc of every type unconditionally, one
`listDocs` request per type. At 43 docs the sticky rail overflowed its
`max-h` container and showed a heavy default scrollbar.

**Fix (`feat(repo-nav)`, `b4f37ec`):** per-type collapsible drawers.
Type rows show name + count; the route's active `:type` auto-expands
(manual toggles reset when navigation changes type); a caret button
peeks a drawer open/closed without navigating; zero-doc types disable
the caret. `listDocs` now only fires for open drawers, so a collapsed
rail costs one request instead of one per type. The rail on
repo-guardian went from ~43 rows to 5.

Facet subtlety: search facets omit zero-hit types, so a missing
`typeCounts` key after facts load **is** a zero — that drives both the
count text and the disabled caret.

### Observation 2: the "goofy" repo-home title was not frontmatter

The `index_md` payload for repo-guardian is clean — no frontmatter, it
opens with `# repo-guardian`. The real cause: Tailwind preflight
unstyles all headings, and `.doc-prose` never defined an `h1` rule
because the reader strips body h1s — the repo home is the **only**
surface that keeps one, so it rendered with inherited body sizing.

**Fix (`fix(theme)`, `0189032`):** `.doc-prose h1` styled to match the
reader's doc title (serif, 2rem), plus `scroll-margin-top` so anchors
land below the sticky topbar.

### Observation 3: the few-pixel layout shift is the viewport scrollbar

Short pages (type listing) have no viewport scrollbar; long pages
(reader) do. When it appears the whole layout nudges left by the
scrollbar width — the classic gutter shift, visible on macOS with
always-on scrollbars.

**Fix (`fix(theme)`, `0189032`):** `scrollbar-gutter: stable` on
`html` reserves the gutter permanently; `scrollbar-width: thin` +
`scrollbar-color` recolor classic scrollbars on inner scroll regions
(the nav rail) so they read against the dark surfaces.

### Observation 4: rendering gaps on real RFC content

Browsing `donaldgifford/rfcs` (RFC-0002, RFC-0006) surfaced three
pipeline gaps the demo fixtures never exercised:

1. **GitHub alerts render as literal pull-quotes.** RFC-0002 uses
   `> [!WARNING]` / `> [!CAUTION]`. The pipeline has no alert
   handling, so they fall through as ordinary blockquotes — which the
   theme styles as serif-italic *pull-quotes* (per the mockup), with
   the raw `[!WARNING]` text as the first line. Doubly wrong: the
   marker leaks, and a technical warning gets literary-quote styling.
2. **Mermaid fences render as plain code.** RFC-0006 has a
   ` ```mermaid ` fence; Shiki has no mermaid grammar and nothing
   renders the diagram.
3. **Code blocks have no chrome.** Fences render as bare `<pre>`.
   Wanted: a header bar — uppercase language badge (accent) left,
   filename right when the fence meta carries one (RFC-0006's
   ` ```go ` fence has language only).

### Also landed in this pass

- `chore(deploy)` `c5117f0` — `deploy/compose.local.yaml` +
  `just local-up`/`local-down`: one-command rebuild/restart of the
  site container against the running docz-api local stack.
- `chore` `3ba9749` — untracked the accidentally-committed
  session-local loop state; `.claude/*.local.*` ignored.

All fixes verified with headless screenshots against repo-guardian
live (styled h1, 5-row collapsed rail, drawer peek, pixel-identical
nav position across pages, zero console errors) and the full local
gate (276 unit tests, lint, `tsc -b`, build, bundle budget, 9 e2e).

## Conclusion

**Answer:** Yes — all three papercuts were real defects, and two of
three root causes differed from the initial theory (missing `h1` rule
rather than frontmatter; scrollbar gutter rather than page CSS). All
three are fixed on `chore/qol-tweaks`. The remaining polish items are
enumerated below; this investigation stays **Open** until that backlog
is done or explicitly dropped.

## Recommendation

Work the backlog top-down on `chore/qol-tweaks` (or successors), one
conventional commit per item, checking items off here as they land.

### Follow-up backlog

- [ ] **Reader metadata table in the doc header** — long documents
      produce long right-rail ToCs, burying the metadata (and the
      html/md/json format switch) below the fold of the rail. Adopt
      the mockup's doc-portal treatment: a bordered key/value metadata
      table directly under the title/status header (owner, repo · dir,
      source path, git sha, created — whatever the Document DTO
      carries; the mockup's relationship rows and tags stay gated on
      the DESIGN-0001 API asks). Move the format switch into that
      header area too. Lifecycle moves to a closed-by-default drawer
      (same disclosure pattern as the nav's type drawers) instead of
      always burning rail space.
- [ ] **GitHub alert callouts** — transform `> [!NOTE|TIP|IMPORTANT|`
      `WARNING|CAUTION]` blockquotes into styled callout boxes (kind
      label + per-kind accent border/color); plain blockquotes keep
      the pull-quote treatment.
      _Prior art: `rfc-site/src/portal/markdown/plugins/`
      `github-alerts.ts` ports nearly verbatim (mdast plugin after
      gfm, `hName`/`hProperties` → `div.admonition.kind` +
      `span.adm-label`; IMPORTANT normalized to note). Because it runs
      before OUR sanitizer, `schema.ts` widens narrowly —
      value-restricted classNames on div/span only — and the XSS suite
      grows with it. CSS ports from rfc-site `styles.css` §281–360
      (adm-label, ::before icon slot, per-kind color-mix rows) onto
      docz tokens._
- [ ] **Mermaid rendering** — render ` ```mermaid ` fences as
      diagrams, lazily (the 130 KB gz entry budget exists precisely to
      catch mermaid's ~700 KB landing eagerly).
      _Prior art: `mermaid-marker.ts` + `mermaid-hydrate.ts` in
      rfc-site. Our sanitize-first pipeline makes the marker simpler
      (runs post-sanitize, pre-Shiki: read `language-mermaid` class,
      set `data-mermaid-source`, strip class — zero schema change),
      and our hast-to-JSX render replaces their DOM-query hydration:
      `MarkdownPre` routes marked blocks to a lazy `<MermaidBlock>`.
      Carry their hard-won lessons: SVG cache keyed by source
      (StrictMode double-render flash), MINIMAL documented
      themeVariables set (extras break `mermaid.render` silently),
      token-driven theme via getComputedStyle with fallbacks. Open
      decision: injecting mermaid's SVG needs a documented, narrow
      exception to the "no dangerouslySetInnerHTML" rule (try
      `securityLevel: "strict"` first — rfc-site's "loose" note
      conflates strict with sandbox) + mermaid-label payloads in the
      XSS suite. Failure keeps the source text visible._
- [x] **Code block chrome** — header bar on fenced blocks: uppercase
      language badge left, filename right when fence meta provides
      one.
      _Prior art: `wrap-codeblock.ts` + the `pre()` half of rfc-site's
      Shiki transformer + `capture-code-meta.ts`. Our order: a remark
      capture pass surfaces fence meta as a validated `metastring`
      property on `code` (the one schema addition); our Shiki stage
      gains a transformer stamping `dataLanguage`/`dataCaption` on the
      emitted pre; the wrapper runs post-Shiki (post-sanitize —
      structure is ours, no schema surface) building
      `div.codeblock` + header; `MarkdownPre`'s region aria-label
      gains the language. Skips mermaid blocks._
- [x] **Palette prefetch** — wire `usePrefetchDoc` to cmdk's active
      item so palette → reader navigation is instant (nav/reader links
      already prefetch on hover/focus; the palette is the gap).
- [ ] **Heading anchor copy** — hover affordance on reader headings to
      copy the deep link (anchors already exist via rehype-slug).
- [ ] **Recent docs in the palette** — empty-query "recent" section
      fed from a small localStorage list of last-opened docs (UI
      preference only; respects the no-tokens storage rule).
- [x] **Hide the topbar "Sign in" link on `/login`** — redundant while
      already on the login page.
- [ ] **README refresh** — cover Phase 5 auth (`VITE_AUTH_PROVIDERS`),
      `just local-up`/`local-down`, and the deploy compose story.

## References

- DESIGN-0001 — docz-site: cross-repo docz reader and search UI
- IMPL-0001 — docz-site MVP: phased build (all phases merged via PR #7)
- Branch `chore/qol-tweaks`: `3ba9749`, `c5117f0`, `0189032`,
  `b4f37ec`, `a1e2dbb`
