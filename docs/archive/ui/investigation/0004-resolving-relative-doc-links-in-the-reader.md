---
id: INV-0004
title: "Resolving relative doc links in the reader"
status: Concluded
author: Donald Gifford
created: 2026-07-25
---
<!-- markdownlint-disable-file MD025 MD041 -->

# INV 0004: Resolving relative doc links in the reader

**Status:** Concluded
**Author:** Donald Gifford
**Date:** 2026-07-25

<!--toc:start-->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
  - [Observation 1: anatomy of the breakage](#observation-1-anatomy-of-the-breakage)
  - [Observation 2: this is the docz convention, not one bad link](#observation-2-this-is-the-docz-convention-not-one-bad-link)
  - [Observation 3: the machinery to fix it already exists](#observation-3-the-machinery-to-fix-it-already-exists)
  - [Observation 4: the naive resolver, concretely](#observation-4-the-naive-resolver-concretely)
  - [Observation 5: a complementary route-level fallback](#observation-5-a-complementary-route-level-fallback)
  - [Observation 6: what this is not — the link graph stays an API ask](#observation-6-what-this-is-not--the-link-graph-stays-an-api-ask)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
- [References](#references)
<!--toc:end-->

## Question

In production, RFC-0001 of `donaldgifford/libtftest-tf-modules` renders
its References list with dead links — e.g.
`…/libtftest-tf-modules/adr/0013-use-terraform-test-for-plan-time-module-invariants.md`
404s. The markdown source is a *relative file link*:

```markdown
- [ADR-0013: Use `terraform test` for plan-time module
  invariants](../adr/0013-use-terraform-test-for-plan-time-module-invariants.md)
```

Can docz-site resolve these relative doc links to working in-app
links — ideally with a simple, client-side approach where the link is
parsed and rewritten to the canonical reader route
(`/donaldgifford/libtftest-tf-modules/adr/ADR-0013`)?

## Hypothesis

Going in: the xref machinery (doc-id tokens → whitelisted links)
suggests the same map-as-whitelist pattern could resolve *paths* too,
if the API exposes each doc's file path. Expected a small pipeline
feature, not an API ask.

## Context

**Triggered by:** live use of docz.fartlab.dev — the References
section of
`donaldgifford/libtftest-tf-modules` RFC-0001 links four sibling ADRs
by relative path; all four 404 in the reader while working fine on
GitHub.

Current linking behavior in the reader:

- **Xrefs** (`src/markdown/xrefs.ts`): doc-id-shaped *text* tokens
  (ADR-0013 in prose) linkify when they resolve in the
  `useRepoDocIndex` map (UPPERCASED doc_id → href). The map is the
  whitelist; hrefs are built from API data, never document text.
- Markdown *anchors* pass through sanitize with their literal hrefs.
  A relative href like `../adr/0013-….md` is emitted as-is; the
  browser resolves it against the current route, producing
  `/:owner/:repo/adr/0013-….md`, and the doc route treats
  `0013-….md` as a `:docId` — getDoc 404s → NotFoundPanel.
- Crucially, the xref linkifier **skips text inside `a` tags**
  (`OPAQUE_TAGS`) — correct behavior, but it means the common docz
  reference style `[ADR-0013: title](../adr/0013-….md)` gets no help:
  the resolvable ID is right there in the anchor text, and the anchor
  still points into the void.

## Approach

Desk investigation against the live example and both codebases:

1. Reproduce the breakage anatomy from the RFC-0001 source (local
   checkout) and the reader's routing rules.
2. Survey how widespread relative doc links are in real docz repos
   (docz's own generated tables, References sections, index.md).
3. Check whether the API already serves the data a path resolver
   needs (listDocs `path` field) and whether the existing xref
   plumbing (index hook, post-sanitize rewrite, render-cache
   fingerprint, MarkdownAnchor) generalizes.
4. Specify the naive resolver and its edge cases; separate what
   stays out of scope (true referenced-by backlinks).

## Environment

| Component | Version / Value                                     |
| --------- | --------------------------------------------------- |
| docz-site | main @ 3304248 (v0.1.2) + this branch               |
| Example   | libtftest-tf-modules RFC-0001 §References (4 links) |
| Live      | docz.fartlab.dev (production deployment)            |

## Findings

### Observation 1: anatomy of the breakage

Three parts line up to produce the 404:

1. The author writes GitHub-idiomatic relative links:
   `../adr/0013-use-terraform-test-for-plan-time-module-invariants.md`
   — correct on GitHub, where docs live at real file paths.
2. The sanitize schema rightly allows plain relative hrefs (they are
   harmless), so the anchor reaches the DOM verbatim and the browser
   resolves it against the SPA route, yielding
   `/:owner/:repo/adr/<filename>.md`.
3. The doc route's `:docId` segment expects a doc id (ADR-0013), not
   a filename — `useGetDoc` 404s and the reader shows the not-found
   panel.

Nobody is wrong individually; the SPA's URL space simply is not the
repo's file tree.

### Observation 2: this is the docz convention, not one bad link

Survey of `libtftest-tf-modules` alone: RFC-0001 links four ADRs
relatively; RFC-0002 links ADR-0016 and DESIGN-0006; RFC-0003 links
DESIGN-0009 — all `../<type-dir>/<file>.md`. And docz's *own
generated README tables* link every doc by bare filename
(`[0001-….md](0001-….md)`), as does the repo-home `index.md`
convention. Three shapes recur:

| Shape                  | Example                     | Where              |
| ---------------------- | --------------------------- | ------------------ |
| Cross-type relative    | `../adr/0013-slug.md`       | References footers |
| Same-dir sibling       | `0002-slug.md`              | generated tables   |
| Explicit same-dir      | `./0002-slug.md`            | hand-written prose |

Any fix must handle all three (plus optional `#fragment` suffixes).
This is worth fixing precisely because it is the convention: every
docz repo browsed through the site carries these links.

### Observation 3: the machinery to fix it already exists

The naive fix needs a map from repo-relative file path → canonical
route. Every ingredient is already on hand:

- **The API serves the path.** listDocs summaries carry `path` —
  e.g. `docs/frameworks/0001-intro.md` — exactly the string a
  relative href resolves to when applied against the current doc's
  own `path` (`docs/rfc/0001-….md` + `../adr/0013-….md` →
  `docs/adr/0013-….md`). No API change needed.
- **The queries are already fetched.** `useRepoDocIndex` builds the
  xref map from getRepo + per-type listDocs (5-minute staleTime).
  The same data yields a second map keyed by path; one hook returns
  both.
- **The rewrite point exists.** Xref linkify runs post-sanitize on
  trusted structure; a sibling transform can visit `a` elements the
  same way. `MarkdownAnchor` already turns `data-xref` anchors into
  router `Link`s (client-side navigation, and `usePrefetchDoc`-style
  hover prefetch can join later).
- **Re-render on late index is solved.** Render-cache keys carry an
  fnv1a fingerprint of the sorted resolver ids; the path map joins
  the fingerprint, so bodies re-render at most once when the index
  finishes loading — the mechanism xrefs already paid for.
- **The security posture carries over unchanged.** The document text
  supplies only a *lookup key*; the emitted href always comes from
  the API-derived map — the same "map is the whitelist" rule quoted
  at the top of `xrefs.ts`. Unresolved links stay exactly as they
  are today (status quo, no new surface). No sanitize-schema change,
  so the XSS suite is untouched.

### Observation 4: the naive resolver, concretely

A post-sanitize transform, run alongside xref linkify:

1. Visit `a` elements whose `href` is relative: skip `http(s):`,
   protocol-relative `//`, root-absolute `/`, `mailto:`, and
   pure-fragment `#…` hrefs.
2. Resolve the href against the directory of the current doc's own
   `path` (posix semantics; the repo-home `index.md` resolves from
   the docs root). Normalize `.`/`..`; a path escaping the repo root
   fails the lookup naturally.
3. Split off any `#fragment`. Look the normalized path up in the
   path→doc map (exact match; paths are compared case-sensitively,
   matching GitHub).
4. On hit: replace the href with the canonical route from the map
   (`/owner/repo/type/DOC-ID`), reattach the fragment, and mark the
   anchor `data-xref` so `MarkdownAnchor` renders a router `Link`.
   Fragments ride along; rehype-slug ids are GitHub-style, so
   same-convention section links mostly survive.
5. On miss (non-doc files like `README.md`, assets, typos): leave
   the anchor untouched — today's behavior, no worse. Rewriting
   misses to GitHub blob URLs was considered and rejected: it would
   embed document-text-derived paths into an emitted href, breaking
   the whitelist rule for marginal benefit.

Testing: unit fixtures for the three shapes plus fragment/miss/
escape cases in the pipeline suite; one fixture doc in the demo org
exercising a relative References footer end-to-end; the XSS suite
gains a payload proving hostile hrefs (`../../evil`, encoded
traversal) never resolve to emitted links.

### Observation 5: a complementary route-level fallback

Even with the renderer fixed, filename URLs will keep arriving from
outside the reader: links shared before the fix, GitHub-copied paths
pasted into chat, or external sites deep-linking the file shape. A
small resilience layer in the doc route: when `useGetDoc` 404s and
`:docId` ends in `.md`, consult the (already cached) doc index for a
`path` whose basename matches, and `Navigate replace` to the
canonical route. Cheap, uses the same map, and turns the live
broken URL in this INV's Question into a working redirect. Optional
but recommended — it fixes the *URL space*, where Obs 4 fixes the
*rendering*.

### Observation 6: what this is not — the link graph stays an API ask

The word "backlink" covers two different features. This INV makes
*forward* references work — the link you click. "Referenced-by" (ADR-
0013 listing the RFCs that cite it, relationship banners, a
References/Referenced-by footer) requires parsing every doc's body
server-side — that is DESIGN-0001's cross-doc **link graph** ask on
docz-api, unchanged by this fix. Notably, the resolver's parsed
(source-path → target-doc) pairs are exactly the edges that graph
needs, so the client-side fix also prototypes the data model the API
ask should serve. Cross-repo relative links cannot exist (relative
paths are repo-scoped); cross-repo references are doc-id xrefs or
absolute URLs, both already handled.

## Conclusion

**Answer: yes — and the naive way is the right way.** Relative doc
links break because the SPA's URL space is not the repo file tree,
and they are pervasive because relative file links are the docz/
GitHub convention (docz's own generated tables use them). The fix
needs no API change: listDocs already serves each doc's `path`, the
xref machinery already established the map-as-whitelist rewrite
pattern, the render cache already handles late-arriving indexes, and
the sanitize schema stays untouched. A post-sanitize path resolver
(Obs 4) plus an optional filename-fallback redirect in the doc route
(Obs 5) turns the live RFC-0001 → ADR-0013 example into a working
in-app link. True referenced-by backlinks remain the DESIGN-0001
link-graph API ask — out of scope here, but this fix prototypes its
edge data.

## Recommendation

1. **Build the relative-link resolver** as a feature PR on docz-site:
   extend `useRepoDocIndex` to also return a path→href map, add the
   post-sanitize rewrite (Obs 4) beside xref linkify, fold the path
   map into the render-cache fingerprint, and cover the three link
   shapes + fragment + miss + traversal cases in tests (XSS suite
   included).
2. **Include the route-level filename fallback** (Obs 5) in the same
   PR if cheap, or as a fast follow — it repairs already-shared URLs,
   including this INV's motivating link.
3. **Keep unresolved links untouched** (no GitHub-blob rewriting) to
   preserve the hrefs-from-API-data-only rule.
4. **Carry the edge data forward**: when the DESIGN-0001 link-graph
   ask is written up for docz-api, cite this resolver's
   (source, target) pairs as the client-validated edge model.

## References

- Live broken example:
  `https://docz.fartlab.dev/donaldgifford/libtftest-tf-modules/rfc/RFC-0001`
  → References → ADR-0013 (renders
  `…/adr/0013-use-terraform-test-for-plan-time-module-invariants.md`,
  404)
- Source: `libtftest-tf-modules/docs/rfc/0001-….md` §References
  (four relative ADR links, verified in local checkout)
- `src/markdown/xrefs.ts` — the map-as-whitelist precedent and the
  `OPAQUE_TAGS` skip that leaves anchors unhelped
- `src/hooks/useRepoDocIndex.ts` — the index the path map extends
- `src/routes/doc.tsx` — `:docId` handling; site of the Obs 5
  fallback
- `api/openapi.yaml` — DocSummary `path`
  (`docs/frameworks/0001-intro.md` example)
- DESIGN-0001 §API asks — the cross-doc link graph (referenced-by)
  this INV explicitly does not solve
- INV-0003 — sibling investigation; its pages design (option c)
  needs the same base-path-aware resolution for non-docz pages
