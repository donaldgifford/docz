---
id: DESIGN-0002
title: "Nav pins, changelog page, and doc link resolution"
status: Implemented
author: Donald Gifford
created: 2026-08-10
---
<!-- markdownlint-disable-file MD025 MD041 -->

# DESIGN 0002: Nav pins, changelog page, and doc link resolution

**Status:** Implemented
**Author:** Donald Gifford
**Date:** 2026-08-10

<!--toc:start-->
- [Overview](#overview)
- [Goals and Non-Goals](#goals-and-non-goals)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Background](#background)
- [Detailed Design](#detailed-design)
  - [Component 1: runtime nav pins](#component-1-runtime-nav-pins)
  - [Component 2: doc link resolution](#component-2-doc-link-resolution)
  - [Component 3: the repo changelog page](#component-3-the-repo-changelog-page)
  - [Cross-cutting: security invariants](#cross-cutting-security-invariants)
  - [Cross-cutting: bundle budget](#cross-cutting-bundle-budget)
- [API / Interface Changes](#api--interface-changes)
- [Data Model](#data-model)
- [Testing Strategy](#testing-strategy)
- [Migration / Rollout Plan](#migration--rollout-plan)
- [Open Questions](#open-questions)
- [References](#references)
<!--toc:end-->

## Overview

One design for the buildable slices of INV-0003, INV-0004, and
INV-0005, which overlap in machinery: per-deployment **nav pins** in
the topbar (RFCs, Frameworks, later Docs), the **repo changelog
page** under the Home nav section, and **relative doc-link
resolution** so the docz convention of `[ADR-0013](../adr/0013-….md)`
links works in the reader. All three ride infrastructure that already
exists — the v0.1.2 runtime-config injection, the repo doc index
behind xrefs, the post-sanitize rewrite point, and the repo-home
"cached file as page" pattern — and none of them needs a docz-api
change.

## Goals and Non-Goals

### Goals

1. Curated topbar tabs configurable **per deployment, without a
   rebuild** — same mechanism and security posture as
   `DOCZ_AUTH_PROVIDERS`.
2. The opt-in repo changelog (docz-api spec 1.2.0) rendered as a
   first-class page, with a nav row under `Home · index.md` shown
   only for repos that serve one.
3. Relative markdown links between docs resolve to working in-app
   router links; already-shared filename-shaped URLs redirect to the
   canonical doc route.
4. Hold the line on the standing invariants: no new docz-api asks, no
   sanitize-schema widening, no meaningful eager-bundle growth, hrefs
   emitted only from API data or validated config.

### Non-Goals

- **Team-docs pages ingest** (INV-0003 option c) — future DESIGN pair
  with the `.docz.yaml` "extras" contract; the DOCS-XXXX-type v1
  needs no site work.
- **Referenced-by backlinks / the cross-doc link graph** — stays a
  DESIGN-0001 additive API ask; this design only fixes forward links.
- **Changelog in search** — it is repo metadata, consistent with
  `index.md` today.
- **External URLs in nav pins** — same-origin app paths only, by
  policy (INV-0003 Obs 3).

## Background

Concluded inputs and what each contributes:

- **INV-0003** (nav tabs): RFC/Frameworks tabs are links to surfaces
  that already work; the only build is the pin mechanism, decided as
  a runtime-config extension. Same-origin-path validation (the
  `authReturn.ts` rule) and charset-capped labels (the `metastring`
  rule) keep the injected script closed.
- **INV-0004** (relative links): `listDocs` already serves each doc's
  `path`; a post-sanitize resolver beside `linkifyDocIds` rewrites
  relative hrefs through a path→href map — the map is the whitelist,
  misses stay untouched. A doc-route fallback redirects
  filename-shaped `:docId` 404s.
- **INV-0005** (changelog, all OQs → a): `getRepoChangelog` at spec
  1.2.0; existence signal from `config_snapshot.changelog` in the
  getRepo response RepoNav already fetches (`repo-nav.tsx:117`);
  `/changelog` static route; h1 kept; `TocList` rail; basename hint.

Existing machinery being extended (not invented): `server/serve.ts`
runtime-config injection + Bun test suite; `src/lib/authProviders.ts`
as the reader-module template; `src/markdown/xrefs.ts` +
`useRepoDocIndex` + the fnv1a render-cache fingerprint;
`src/routes/repo-home.tsx` (`useRenderedSource`, `RepoFrame`,
`TocList` rail, shared query-state panels).

## Detailed Design

### Component 1: runtime nav pins

Data flow (mirrors auth providers end to end):

```text
Helm config.navLinks ──► DOCZ_NAV_LINKS env ──► serve.ts
  resolveNavLinks() [validate + drop invalid] ──► __DOCZ_CONFIG__.nav
    ──► src/lib/navLinks.ts [re-validate] ──► AppShell <NavLink>s
```

- **`server/serve.ts`**: `resolveNavLinks(raw: string | undefined)`
  parses the env value (format per OQ-1), validates each entry, and
  returns at most **6** links (excess dropped, `log`-visible at
  startup). Validation per entry:
  - `label`: 1–24 chars matching `/^[\w .&+-]+$/` — no `<`, `>`,
    quotes, or backslash can ever reach the injected `<script>`, so
    the existing no-breakout property of the config script is
    preserved by construction.
  - `href`: the `authReturn.ts` rule — starts with `/`, not `//`, no
    whitespace/control characters, ≤ 200 chars. App paths only;
    query strings allowed so a pin may target
    `/?type=rfc`-style filtered directory views.
  - Anything invalid (entry or whole payload) is dropped, never
    "fixed": misconfiguration degrades to fewer/no pins, exactly like
    unknown auth providers degrade to GitHub.
- **`src/lib/navLinks.ts`**: same shape as `authProviders.ts` —
  reads `window.__DOCZ_CONFIG__.nav`, re-validates with the same
  rules (both-ends validation, as with the auth-return stash), falls
  back to build-time `VITE_NAV_LINKS` (per OQ-2), else `[]`.
- **`AppShell.tsx`**: pins render as `NavLink`s between `Repos` and
  `SessionMenu`, `navLinkClass` active styling for free. The same
  list feeds the small-viewport nav treatment so mobile parity is
  automatic.
- **Helm chart**: `config.navLinks` (list of `{label, href}`),
  rendered into the deployment env as JSON via `toJson`;
  `values.schema.json` types it; helm-unittest asserts default
  (unset → env absent) and an override.

### Component 2: doc link resolution

**Index.** `useRepoDocIndex` grows from returning `XrefResolver` to:

```ts
interface RepoDocIndex {
  byId: ReadonlyMap<string, string>;   // "ADR-0013" -> href (today's map)
  byPath: ReadonlyMap<string, string>; // "docs/adr/0013-….md" -> href
}
```

Both maps come from the same getRepo + per-type listDocs queries
already being fetched (no new requests). The render-cache fingerprint
extends to cover the path keys, so bodies still re-render at most
once when the index finishes loading.

**Transform.** New `src/markdown/relative-links.ts`, run
post-sanitize beside `linkifyDocIds`:

1. Visit `a` elements; skip hrefs that are absolute (`scheme:`,
   `//`), root-absolute (`/`), or fragment-only (`#…`).
2. Resolve the href posix-style against the *directory of the
   rendering source's own repo path* (the `base`), normalizing
   `.`/`..`; a path that escapes the repo root can never match a map
   key, so traversal fails closed.
3. Split any `#fragment`; look the normalized path up in `byPath`
   (matching rule per OQ-3).
4. Hit → replace `href` with the map's canonical route, reattach the
   fragment, set `dataXref` so `MarkdownAnchor` renders a router
   `Link` (client-side nav + prefetch path). Miss → leave the anchor
   byte-identical to today.

**Base paths per surface** (the parameter INV-0004 designed for):

| Surface     | `base`                                        |
| ----------- | --------------------------------------------- |
| Reader doc  | the doc's own `path` from getDoc              |
| Repo home   | `docs_dir` + `/index.md` (docs_dir from getRepo) |
| Changelog   | the configured changelog file path (Component 3) |

`renderMarkdown` / `useRenderedSource` options gain the optional
`base` + path map alongside the existing xref resolver.

**Route fallback.** In `doc.tsx`: when the doc query fails with
`NotFoundError` *and* `:docId` matches `/\.md$/i`, look for a doc in
the loaded index whose `path` basename matches; exactly one match →
`<Navigate replace>` to the canonical route (fragment preserved via
`location.hash`); zero or multiple → the existing not-found panel.
This repairs URLs shared before the fix — including the live
libtftest-tf-modules example that triggered INV-0004.

### Component 3: the repo changelog page

**Spec.** Re-vendor `api/openapi.yaml` at 1.2.0 (+ `just gen-api`) —
additive: `getRepoChangelog` → `RepoChangelog
{repo, changelog_md, changelog_sha}`.

**Route.** `{ path: ":owner/:repo/changelog", lazy: … }` registered
with the repo routes; the static segment outranks
`/:owner/:repo/:type`, and the accepted caveat (INV-0005 OQ-2a) is
that a custom type literally *named* `changelog` remains reachable
only via its id_prefix/alias URL.

**`src/routes/repo-changelog.tsx`** (pattern: `repo-home.tsx`):

| State                          | Render                                            |
| ------------------------------ | ------------------------------------------------- |
| 401                            | shared `SessionRequiredRedirect`                  |
| repo 404                       | shared not-found panel                            |
| changelog 404                  | quiet "this repo doesn't serve a changelog" panel (reachable via direct URL, or the enabled-but-absent-at-HEAD edge) |
| `changelog_md: ""`             | explicit empty state ("changelog is empty at HEAD") |
| content                        | `useRenderedSource` with the file's own h1 kept; `RepoFrame` crumbs `home · changelog`; `TocList` rail (version jump list); render memoized per `(repo, changelog_sha)`; xref resolver + `byPath`/`base` passed so doc-id mentions and relative links in the changelog resolve (base = the configured file's directory — repo root by default, **not** `docs_dir`) |

**Nav row** (`repo-nav.tsx`, directly under Home):

- Gate: defensive read of the `useGetRepo` data RepoNav already has —
  `config_snapshot.changelog.enabled === true` (the snapshot is
  untyped JSON; wrong shapes read as "no row").
- Label `Changelog`, hint = basename of
  `config_snapshot.changelog.file ?? "CHANGELOG.md"`, full subpath as
  `title` when it differs (chart-style `charts/<name>/CHANGELOG.md`).
- Hover/focus prefetches `getRepoChangelog` (the `usePrefetchDoc`
  convention).

**Fixtures.** The demo-org docz-site repo serves its real
`CHANGELOG.md` via `?raw` import (the pattern the doc fixtures
already use); one fixture repo returns `changelog_md: ""` for the
empty state; all other repos fall through to 404 so the row stays
hidden — which also exercises the gate.

### Cross-cutting: security invariants

- **Injected script stays closed.** Nav labels are the first
  free-text values in `__DOCZ_CONFIG__`; the charset above excludes
  every character that could terminate or escape the script context,
  validated at the injector *and* the reader. `serve.test.ts` gains
  no-breakout assertions for hostile `DOCZ_NAV_LINKS`.
- **Hrefs from API data or validated config only.** Document text
  contributes lookup keys, never emitted hrefs (Component 2 misses
  stay untouched; traversal fails closed). Nav hrefs pass the
  same-origin path rule twice.
- **No `schema.ts` change anywhere** — the resolver runs
  post-sanitize like xrefs; the XSS suite gains traversal/hostile-
  href payloads without widening any allowance.

### Cross-cutting: bundle budget

`navLinks.ts` + the AppShell render are the only eager additions
(≲1 KB gz; ~8.6 KB headroom). The changelog route, the resolver
transform, and the fixtures all live in lazy chunks. `just
bundle-budget` gates the PR as usual.

## API / Interface Changes

| Surface                  | Change                                                       |
| ------------------------ | ------------------------------------------------------------ |
| docz-api                 | **none** (consumes shipped 1.2.0)                            |
| Vendored spec            | 1.1.0 → 1.2.0 (additive)                                     |
| `window.__DOCZ_CONFIG__` | gains optional `nav: {label, href}[]`                        |
| Env                      | new `DOCZ_NAV_LINKS` (+ `VITE_NAV_LINKS` per OQ-2)           |
| Helm chart               | new `config.navLinks` value + schema + env wiring            |
| Routes                   | new `/:owner/:repo/changelog`; `changelog` becomes a reserved segment |
| `useRepoDocIndex`        | return type widens to `{byId, byPath}` (internal)            |

## Data Model

No server-side model changes. Client-side: the nav-link entry type
(`{label: string; href: string}`), the doc index's second map, and
the changelog query cached under the generated orval key with
`changelog_sha` keying the render memo (mirrors the reader's
`(doc_id, content_hash)`). `recentDocs` is untouched — the changelog
is not a doc and never enters the palette's recents.

## Testing Strategy

| Layer            | Coverage                                                                 |
| ---------------- | ------------------------------------------------------------------------ |
| `bun test server/` | `resolveNavLinks` (parse, caps, drop-invalid, no-breakout injection)   |
| Unit (vitest)    | `navLinks.ts` precedence (runtime → build-time → none, hostile input); resolver fixtures (three link shapes, fragments, miss, traversal, encoded); `useRepoDocIndex` map building incl. `aliases: null`-style wire nulls; route fallback (unique/ambiguous/none); changelog page states (404/empty/content, row gating on config snapshots incl. malformed) |
| XSS suite        | hostile relative hrefs never resolve to emitted links; nav-label payloads |
| a11y             | changelog view joins the axe sweep; AppShell sweep re-covers pins        |
| Helm             | unittest for `navLinks` default + override                               |
| e2e              | nav pins render + navigate (via `VITE_NAV_LINKS` in the MSW build, per OQ-2); changelog per OQ-4 |

## Migration / Rollout Plan

All three components are additive and independently shippable; no
flags needed. Proposed order (rationale: user-visible ask first,
live-bug fix second, estate feature last — delivery shape per OQ-5):

1. **Phase 1 — changelog**: spec re-vendor + gen-api → fixtures →
   route/page → nav row → tests. Requires nothing upstream; the
   `rfcs`/`docs` repos don't exist yet but changelog-enabled repos
   do.
2. **Phase 2 — link resolution**: index widening → transform → route
   fallback → XSS/unit fixtures. Fixes the live broken References
   links.
3. **Phase 3 — nav pins**: serve.ts + navLinks.ts + AppShell → chart
   value/schema/unittest → docs, plus the `docs` type curation
   (OQ-6b: `CURATED_TYPES` entry, contrast-checked token, blurb).
   Most useful once the `rfcs` repo is onboarded, hence last.

Each phase ends green on `just ci` semantics; chart-touching Phase 3
bumps the chart patch version. CLAUDE.md gains the new invariants
(reserved `changelog` segment, nav-pin validation rules, base-path
parameter) as each lands.

## Open Questions

**Reviewed 2026-08-11 — decided: 1a, 2a, 3a, 4a, 5a, 6b.** Options
preserved below for the record. On 6b, one clarification recorded:
type colors are **not** runtime-configurable and were never designed
to be — DESIGN-0001's color system is a curated in-code map
(`CURATED_TYPES` in `src/lib/colors.ts`, values referencing
`tokens.css` custom properties) with the deterministic FNV-1a
fallback for everything else. "Curate the `docs` type" therefore
means: add `docs` to `CURATED_TYPES`, add its `--color-t-docs` token
(which must pass `contrast.test.ts`), and give it a blurb in
`docTypes.ts` — pre-styling a type we know is coming, exactly like
`framework` was.

**OQ-1 — `DOCZ_NAV_LINKS` wire format?**

- **a (recommended).** A JSON array:
  `DOCZ_NAV_LINKS='[{"label":"RFCs","href":"/donaldgifford/rfcs/rfc"}]'`.
  Structured and extensible (a future `external` or `icon` field
  costs nothing), maps 1:1 from the chart's `config.navLinks` list
  via `toJson`, and `values.schema.json` can type it fully. Slightly
  noisier to hand-type in a compose file, but this value is
  config-managed, not hand-typed.
- **b.** Compact pairs: `"RFCs=/donaldgifford/rfcs/rfc,Frameworks=/…"` —
  friendlier in a bare env var, but labels can never contain `,`/`=`,
  it needs a bespoke parser on both ends, and it can't grow fields.
- other: —

**OQ-2 — Build-time `VITE_NAV_LINKS` fallback?**

- **a (recommended).** Yes — exact parity with the auth-providers
  precedence (runtime → build-time → default). It is also what makes
  the feature *testable*: dev and the MSW e2e build have no
  `serve.ts` in front of them, so without a build-time source the
  pins are invisible in dev and unverifiable in Playwright.
- **b.** Runtime-only — one less env var, but dev/e2e never see pins
  and the e2e suite can't cover the nav surface.
- other: —

**OQ-3 — Relative-link matching rule?**

- **a (recommended).** Exact normalized-path match only. Predictable,
  collision-proof, and honest: a relative link wrong on GitHub stays
  wrong here. (The *route fallback* still matches by unique basename —
  it has no base directory to resolve against, and uniqueness guards
  ambiguity.)
- **b.** Exact match, then unique-basename fallback for in-body links
  too — more forgiving of hand-typed `../` depth mistakes, at the
  cost of two matching semantics in one transform and silent
  "success" on links GitHub would 404.
- other: —

**OQ-4 — Changelog e2e coverage?**

- **a (recommended).** Unit + axe only. The e2e suite is reserved for
  core cross-page journeys (directory → read, palette, auth); the
  changelog is a leaf page whose states are fully exercisable in
  vitest with MSW fixtures. Keeps the Playwright wall-clock flat.
- **b.** Add an e2e journey (repo → changelog row → rendered
  versions → ToC jump) — one more real-browser guarantee, one more
  spec to maintain.
- other: —

**OQ-5 — Delivery shape?**

- **a (recommended).** One IMPL-0003 doc with the three phases of the
  rollout plan, checked off as each lands (the IMPL-0001/0002
  workflow this repo already runs). Keeps the overlap visible in one
  place and matches how CLAUDE.md/tooling expect work to be tracked.
- **b.** Three standalone feature PRs with no IMPL doc — less
  ceremony, but the shared machinery (index widening feeding both
  Phase 2 and Phase 3's future Docs pin) loses its single tracking
  surface.
- other: —

**OQ-6 — Curate the `docs` type (color + blurb) now?**

- **a (recommended).** Defer until the `docs` repo actually exists
  and is onboarded (INV-0003's DOCS-XXXX v1) — the deterministic
  fallback colors already handle unknown types, so curation before
  content is speculative polish.
- **b.** Include the two-line curation in Phase 3 now, so the estate
  lands styled the moment the repo is onboarded.
- other: —

## References

- INV-0003 — curated nav tabs; runtime-config pin decision,
  team-docs option space
  (`docs/investigation/0003-curated-nav-tabs-for-rfcs-frameworks-and-team-docs.md`)
- INV-0004 — relative doc links; map-as-whitelist resolver + route
  fallback
  (`docs/investigation/0004-resolving-relative-doc-links-in-the-reader.md`)
- INV-0005 — changelog page; all open questions resolved to (a)
  (`docs/investigation/0005-rendering-the-repo-changelog-from-docz-api.md`)
- DESIGN-0001 — parent design; additive-asks register (link graph),
  one-front-door principle
- docz-api `d4399ea` — spec 1.2.0, `getRepoChangelog`
- `server/serve.ts`, `src/lib/authProviders.ts` — the runtime-config
  pattern Component 1 extends
- `src/markdown/xrefs.ts`, `src/hooks/useRepoDocIndex.ts` — the
  whitelist-map machinery Component 2 extends
- `src/routes/repo-home.tsx`, `src/components/repo-nav.tsx` — the
  patterns Component 3 copies
