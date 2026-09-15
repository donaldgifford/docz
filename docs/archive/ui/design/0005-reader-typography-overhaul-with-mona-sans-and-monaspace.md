---
id: DESIGN-0005
title: "Reader typography overhaul with Mona Sans and Monaspace"
status: In Review
author: Donald Gifford
created: 2026-09-08
---

<!-- markdownlint-disable-file MD025 MD041 -->

# DESIGN-0005: Reader typography overhaul with Mona Sans and Monaspace

**Status:** In Review
**Author:** Donald Gifford
**Date:** 2026-09-08

<!--toc:start-->
- [Overview](#overview)
- [Goals and Non-Goals](#goals-and-non-goals)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Background](#background)
  - [Where typography lives today](#where-typography-lives-today)
  - [Measurements](#measurements)
  - [Diagnosis, ranked](#diagnosis-ranked)
  - [Why the body is a proportional face, not Monaspace Argon](#why-the-body-is-a-proportional-face-not-monaspace-argon)
  - [Candidate research](#candidate-research)
- [Detailed Design](#detailed-design)
  - [Component 1: the font stack](#component-1-the-font-stack)
  - [Component 2: prose sizes and faces](#component-2-prose-sizes-and-faces)
  - [Component 3: the measured column](#component-3-the-measured-column)
  - [Component 4: inline code de-emphasis](#component-4-inline-code-de-emphasis)
  - [Component 5: code blocks, ligatures, italics](#component-5-code-blocks-ligatures-italics)
  - [Component 6: mockup.html and documentation](#component-6-mockuphtml-and-documentation)
  - [Cross-cutting invariants](#cross-cutting-invariants)
- [API / Interface Changes](#api--interface-changes)
- [Data Model](#data-model)
- [Testing Strategy](#testing-strategy)
- [Migration / Rollout Plan](#migration--rollout-plan)
- [Open Questions](#open-questions)
- [References](#references)
<!--toc:end-->

## Overview

Long docz documents are hard to read on the site. Design docs in
particular are walls of grey serif text broken up by bright blue inline
code chips, and the same content on a comparable site (Oxide's RFD
reader) reads comfortably at a higher density. This design replaces the
reader's typography with three coordinated changes: a measured prose
column, a quieter inline-code treatment, and a new font stack — Mona
Sans for prose and UI, Monaspace Neon for code (with ligatures and true
italics), and Monaspace Xenon for the slots that were serif. Color
tokens are untouched; the measurements below show body text already
clears WCAG AAA, so contrast is not the lever.

The work ships on `feat/reader-typography` together with the IMPL doc
that executes this design. All eleven open questions were decided on
2026-09-08 (recommendation (a) everywhere except OQ-6, which takes all
nine ligature sets); `mockup.html` on the same branch already renders
the decided look so it can be dialed in live before the IMPL doc is
written.

## Goals and Non-Goals

### Goals

- Make dense prose (design docs, investigations, changelogs) read at
  least as comfortably as Oxide's RFD pages at the same density.
- Reserve the accent color for links. Inline code stops competing with
  links for attention.
- Replace IBM Plex Mono with Monaspace Neon everywhere mono is used
  (code blocks, inline code, and the ~85 mono UI labels), with
  Monaspace's ligature sets enabled in code blocks only and its true
  italic loaded so Shiki's italic scopes render as designed.
- Replace IBM Plex Sans and Source Serif 4 with Mona Sans (prose and
  UI) and Monaspace Xenon (headings and pull quotes), dropping the
  serif body entirely.
- Keep every existing gate green: mathematical contrast, axe unit
  sweep, full-rule e2e axe, bundle budget, format check.
- Keep the "no third-party font requests" invariant: every face stays
  self-hosted through `@fontsource` packages under OFL-1.1.

### Non-Goals

- Changing any color token. `fg-secondary` at 9.3:1 already exceeds
  AAA's 7:1; the perceived dimness comes from stroke weight and
  measure, not the ratio (OQ-10 offers a lift as an option, not a
  goal).
- Light theme, user-selectable fonts, or a per-deployment font
  override. Fonts are static assets, so a deployment-time override for
  a licensed face like Berkeley Mono is feasible later, but it is a
  separate design.
- Restyling non-reader surfaces beyond what the font swap forces
  (topbar, directory, palette keep their layouts).
- Touching the markdown pipeline, sanitizer, or Shiki configuration
  beyond CSS. Ligatures and italics are font-level; no pipeline stage
  changes.

## Background

### Where typography lives today

- `src/theme/tokens.css` is the single stylesheet: `--font-sans`
  (IBM Plex Sans), `--font-mono` (IBM Plex Mono), `--font-serif`
  (Source Serif 4), the `body` rule (15px sans, line-height 1.6), and
  every `.doc-prose` rule (ported verbatim from `mockup.html`).
- `src/main.tsx` imports eleven per-weight `@fontsource` CSS files:
  Plex Sans 400/500/600/700, Plex Mono 400/600/700, Source Serif 4
  400/400-italic/600/700. Latin woff2 files are 15–23 KB each.
- Tailwind utilities reference the tokens: `font-mono` appears 85
  times across `src/**/*.tsx`, `font-serif` four times (the doc title
  in `doc.tsx`, the h1s in `repo-home.tsx`, `repo-type.tsx`,
  `repo-changelog.tsx`), `font-sans` never (body default).
- `mockup.html` is the documented visual source of truth. It loads the
  same three families from Google Fonts and its `:root` declares the
  same `--font-*` variables, so any change here must be mirrored or the
  mockup formally demoted for typography (OQ-9).
- DESIGN-0001 recorded the font choice only as "dark IBM Plex
  Mono/Sans/Source Serif aesthetic" inherited from the mockup; there is
  no separate typography decision to supersede.

### Measurements

Contrast of the current text tokens against the base surface (WCAG
2.x relative luminance, same math as `src/theme/contrast.test.ts`):

| Token          | Used for                 | On `bg-base` | On `bg-raised` | On `bg-elevated` |
| -------------- | ------------------------ | -----------: | -------------: | ---------------: |
| `fg-primary`   | headings, `strong`       |      15.94:1 |        15.00:1 |          13.95:1 |
| `fg-secondary` | body prose               |       9.34:1 |         8.79:1 |           8.17:1 |
| `accent`       | links **and** inline code |      11.11:1 |        10.45:1 |           9.72:1 |
| `fg-tertiary`  | meta lines               |       6.11:1 |         5.75:1 |           5.35:1 |
| `fg-muted`     | list markers, labels     |       5.20:1 |         4.89:1 |           4.55:1 |

AAA asks 7:1 for body text and 4.5:1 for large text. Body prose passes
AAA today; only `fg-tertiary` and `fg-muted` sit in the AA band, and
neither carries paragraphs.

Prose column width, from `RepoFrame`'s grid (`max-w-[1360px]`,
`px-5`, `250px` nav, `190px` rail, `gap-x-12` at ≥1181px, `gap-x-10`
between 861px and 1180px):

| Viewport      | Content column | Source Serif 4 @16px | Mona Sans @16.5px |
| ------------- | -------------: | -------------------: | ----------------: |
| ≥1181px       |         ~784px |      ~100–110 chars |     ~90–95 chars |
| 861px–1180px  |        ~1030px |      ~135–145 chars |    ~120–125 chars |

Nothing caps `.doc-prose`, so the paragraph runs the full column.
Comfortable measure for continuous reading is 60–80 characters; Oxide
holds roughly 85. The screenshots that prompted this design were in the
861px–1180px band.

Current prose spec (`.doc-prose`): container 15.5px / line-height 1.7 /
`fg-secondary`; `> p` and `li` in Source Serif 4 at 16px; paragraph
margin 1.1rem; `h2` serif 1.5rem 600; `h3`/`h4` sans 1.02rem 600;
`strong` `fg-primary` 600; links `accent` underlined; `code` mono
0.82em in `accent` on `code-bg` with a 1px hairline border and
`1px 5px` padding; `pre` mono 12.5px / 1.55.

### Diagnosis, ranked

1. **Measure.** Lines run 100–145 characters. This is the single
   biggest cause of the "wall of text" feel and it is independent of
   the fonts.
2. **Inline code styled as a chip.** Accent-colored text plus
   background plus border plus vertical padding, in paragraphs that
   contain five to ten spans. The paragraph fragments into UI chips,
   the padded boxes inflate the line box so code-dense lines get taller
   leading than plain lines, and a code span is distinguishable from a
   link only by the underline.
3. **A thin serif at 16px on a dark surface.** Light-on-dark blooms;
   Source Serif 4's hairlines lose stroke and read dimmer than 9.3:1
   suggests. `strong` in `fg-primary` then looks like a different color
   rather than a heavier weight, which makes the surrounding regular
   text look greyer still. IBM Plex Mono's slab serifs on i, l, r, j
   add a second serif texture inside the same line.
4. **Size and leading are already reasonable.** 16px at 1.7 is 27px of
   leading, comparable to Oxide. Paragraph spacing is slightly tight and
   `h3` at 1.02rem barely rises above body text.

### Why the body is a proportional face, not Monaspace Argon

All five Monaspace variants are monospaced; "humanist" (Argon) and
"slab" (Xenon) describe the letterforms, not the pitch. Fixed-pitch
prose gives every letter the same advance, flattens word shapes, and
reads as a terminal README — the opposite of the reference. Oxide's
body is proportional with mono reserved for labels and code, and GitHub
pairs Monaspace with Mona Sans for prose on the Monaspace site itself.
Berkeley Mono, the face the author would actually prefer, is
commercially licensed with a domain-bound web license and cannot be
redistributed from a public repository; Monaspace Neon is the closest
open answer for the same itch (ligature sets, true italics).

### Candidate research

Packages (all `5.3.0`, all OFL-1.1, all self-hostable via the existing
`@fontsource` pattern):

| Package                            | Shape                        | Latin woff2                              |
| ---------------------------------- | ---------------------------- | ---------------------------------------- |
| `@fontsource-variable/mona-sans`   | variable, `wght` 200–900     | 39,788 B upright / 41,812 B italic       |
| `@fontsource-variable/inter`       | variable, `wght`             | 48,256 B upright                         |
| `@fontsource/monaspace-neon`       | static 200–800, both styles  | 44,476 B (400) / 48,428 B (400i) / 44,632 B (600) |
| `@fontsource/monaspace-xenon`      | static 200–800, both styles  | comparable to Neon                       |
| `@fontsource/monaspace-argon`      | static 200–800, both styles  | comparable to Neon                       |

No `@fontsource-variable/monaspace-*` package exists; Monaspace ships
static per-weight files only. Fontsource's variable CSS names the family
`"Mona Sans Variable"`; the static Monaspace CSS names them
`"Monaspace Neon"` / `"Monaspace Xenon"`. Every fontsource CSS file
declares one `@font-face` per Unicode subset with `unicode-range`, so a
browser fetches only the subsets a page actually uses (latin, in
practice) and `font-display: swap` is already set.

Google Fonts serves Mona Sans (weights 200–900) but not Monaspace, which
only matters for `mockup.html` (OQ-9).

Monaspace OpenType features, from the upstream README:

- `calt` enables **texture healing** (adjacent narrow/wide glyphs
  borrow space from each other). Browsers enable `calt` by default
  (`font-variant-ligatures: normal`), so healing is on everywhere
  Monaspace renders with no CSS at all.
- `liga` (also default-on) customizes spacing of repeated characters
  such as `///` and `||`.
- The ligatures proper are opt-in stylistic sets: `ss01` equals-family
  (`!=`, `===`), `ss02` greater/less-or-equal, `ss03` arrows (`->`,
  `~>`), `ss04` markup (`</`, `/>`), `ss05` F# pipes (`|>`), `ss06`
  repeated `#`, `+`, `_`, `=`, `&`, `ss07` colons (`::`, `=:=`), `ss08`
  period combinations (`..=`, `.-`), `ss09` greater/less-than with
  equals.

Shiki emits each token as its own `<span>` with the theme's
`font-style` inline. The bundled `tokyo-night` theme italicizes
comments, docstrings, control-flow keywords (`return`, `if`, …), and
`meta.var.expr storage.type`; those spans currently fall back to a
synthesized oblique because no Plex Mono italic is loaded. Ligatures
and texture healing cannot cross a span boundary, so a pair split across
two tokens (rare — `!=`, `=>`, `</` are single tokens in the shipped
grammars) stays unligated. Sanitization runs before Shiki, so the inline
`font-style` survives today and this design does not change that order.

## Detailed Design

### Component 1: the font stack

`src/theme/tokens.css`:

```css
--font-sans: "Mona Sans Variable", system-ui, -apple-system, "Segoe UI", sans-serif;
--font-mono: "Monaspace Neon", ui-monospace, "SF Mono", Menlo, monospace;
--font-serif: "Monaspace Xenon", ui-monospace, Georgia, serif;
```

`--font-serif` keeps its name so the four `font-serif` utility usages
and the `.doc-prose` rules keep working; only its value changes (OQ-5
decides whether the token survives at all).

`src/main.tsx` imports become:

```ts
import "@fontsource-variable/mona-sans/wght.css";
import "@fontsource-variable/mona-sans/wght-italic.css";
import "@fontsource/monaspace-neon/400.css";
import "@fontsource/monaspace-neon/400-italic.css";
import "@fontsource/monaspace-neon/600.css";
import "@fontsource/monaspace-neon/700.css";
import "@fontsource/monaspace-xenon/600.css";
import "@fontsource/monaspace-xenon/400-italic.css";
```

Xenon's 400 italic is for blockquotes (Component 2); without it the
browser would slant the 600 upright into a heavy faux italic.

`package.json` swaps `@fontsource/ibm-plex-mono`,
`@fontsource/ibm-plex-sans`, `@fontsource/source-serif-4` for
`@fontsource-variable/mona-sans`, `@fontsource/monaspace-neon`,
`@fontsource/monaspace-xenon`. The `wght` (weight-only) Mona Sans file
is deliberate: the `wdth`/`standard` files carry a width axis this
design does not use and are 2.5× the size.

Latin payload estimate: today ~215 KB across eleven files; after
~355 KB across eight files (Neon's files are roughly three times Plex
Mono's because of the healing alternates). Fonts sit outside the JS
bundle budget and load with `swap`, so first paint is unaffected; the
increase is the cost of ligatures and italics and is accepted.

Weight mapping for the mono utilities: `font-medium` (one usage, the
codeblock `.lang` badge) resolved to 400 under Plex and continues to;
`font-semibold` (two usages) → 600; `font-bold` (one usage, the brand
mark) → 700, loaded so the browser never synthesizes bold (OQ-7 offers
dropping the 700 file by restyling that one usage).

### Component 2: prose sizes and faces

`.doc-prose` changes:

| Rule                     | Today                          | After                                                        |
| ------------------------ | ------------------------------ | ------------------------------------------------------------ |
| `.doc-prose`             | 15.5px / 1.7 / `fg-secondary`  | 16.5px / 1.65 / `fg-secondary` (OQ-2, OQ-10)                 |
| `> p`, `li`              | serif 16px                     | `--font-sans` 16.5px, weight 400 (inherits container)        |
| `p` margin               | `0 0 1.1rem`                   | `0 0 1.25rem`                                                |
| `strong`                 | `fg-primary` 600               | unchanged (Mona Sans 600 is a weight step, not a color jump) |
| `h1`                     | serif 2rem 600                 | `--font-sans` 2rem 500, tracking -0.02em (OQ-5)              |
| `h2`                     | serif 1.5rem 600               | `--font-serif` (Xenon) 1.35rem 600 (OQ-5)                    |
| `h3`, `h4`               | sans 1.02rem 600               | `--font-serif` (Xenon) 1.05rem / 0.95rem 600 (OQ-5)          |
| `blockquote`             | serif italic on `bg-raised`    | `--font-serif` (Xenon) italic, same surface (OQ-5)           |
| `li` margin              | 0.4rem                         | 0.45rem                                                      |

**Dial-in amendment (2026-09-08, live review).** Sections still blurred
together on real documents: h2, h3, and every bold run-in shared
`fg-primary` at 600, so only size separated them, and Xenon at 1.35rem
barely outranked Mona Sans's x-height. Coloring heading text was
rejected (the accent now means link; any other hue needs a new
contrast-gated token). Sections break structurally instead, in the
site's existing vocabulary:

| Rule             | Amended value                                                                          |
| ---------------- | -------------------------------------------------------------------------------------- |
| `h2`             | 1.5rem; margin-top 3.4rem; padding-top 1.4rem; hairline `border-top`                   |
| `h2:first-child` | no rule, no padding (sits directly under the header's own rule)                        |
| `h3`             | 1.15rem; margin-top 2.2rem                                                             |
| `strong`         | weight 500 (the color step to `fg-primary` carries the emphasis; 600 competed with h3) |

A numbered mono eyebrow above each h2 (a CSS counter rendering "01",
"02" in the doc-id label style, Oxide-fashion) was tried in the same
pass and **rejected on review** — the rule and the size steps do the
job without it. It is recorded here so it is not proposed again; the
changelog needed an opt-out modifier for it, which went with it.

**Third amendment (2026-09-09, weight).** On the live page the 400
body read as hairline while the 500 run-ins in the same screenshot read
comfortably — the classic light-on-dark thinning, not color (`fg-prose`
was already at 11.8:1). Body weight moves to 450 and `strong` to 600;
the variable axis makes 450 free, and the `strong` row of the dial-in
table above is superseded. A same-page comparison against Inter at 450
is recorded as OQ-12, because weight alone still leaves Mona Sans's
wide, rounded texture behind the Oxide reference.

**Fourth amendment (2026-09-09, size).** Live review still read as small
next to PlanetScale's blog — a monospace body measured at ~14–15px with
~76 characters per line, so what reads as "bigger" there is a uniform
mono rhythm on a shorter line. Body moves to 18px / 1.6 and the measure
to 60ch (~78 characters of Mona Sans, the line length both reference
sites settle on); h2–h6 switch from rem to em (1.4 / 1.15 / 1 / 0.95 /
0.8) so the hierarchy scales with the body instead of collapsing into
it; code blocks move 13px → 14px. A same-page comparison against Inter
17px and a Monaspace Argon 16px mono body is filed under OQ-12, which
gains option (d).

**Fifth amendment (2026-09-09, chrome and components).** Raising prose
exposed everything that had not moved with it, all found on the
specimen page:

- **Chrome scale.** Nav, outline, breadcrumbs, and meta lines were still
  on the mockup's 10-13.5px scale against 18px prose. Every arbitrary
  text size in the app, the matching tokens.css rules, and the body base
  go up one step.
- **Inline code chips.** They read bottom-heavy because an inline box is
  as tall as the font's ascent and descent, not its line-height:
  Monaspace Neon leaves ~0.5px above the capitals against ~3px below the
  baseline, so even padding pins the text to the top edge. Vertical
  padding is asymmetric now (0.15em over 0.04em).
- **Admonitions.** The filled icon discs read as stickers next to
  everything else on the page. They are gone, replaced by the mockup's
  banner idiom: thin tinted rule, quiet fill, mono uppercase run-in
  label, one step down in size and leading. The precomputed tint tokens
  are untouched, so the contrast pairs still hold.
- **Nav and outline rails.** In-group nav rows and outline rows hang off
  a hairline, and the active row colors a 2px segment of it. The outline
  needs to know what is being read, so `useActiveHeading` adds an
  IntersectionObserver scroll spy over the heading ids; it holds the
  last heading when a long section fills the band and no-ops where the
  observer is missing.
- **Diagrams.** The reference the review compared against turned out to
  be a hand-drawn SVG in the old portal's mockup, not a mermaid render,
  so there is nothing to port. Instead the figure gets real inset, the
  labels get the mono face and a readable fill, and per-node color stays
  where mermaid puts it: a document's own `classDef`, which outranks the
  stylesheet because mermaid scopes those rules by render id.

**Sixth amendment (2026-09-10, the directory).** Review asked for the
discovery page to follow the RFD-index shape: cards rather than table
rows, and the document's STATE as the only colored thing in the row.
Hit rows now carry the doc id over the title, a bordered status pill in
the middle, and the repo on the right. The type badge is gone
everywhere — the doc id already spells the type out, and two colored
chips per row read as two competing signals. Type color still exists
and still identifies the filter chips, so DESIGN-0001's color system is
intact; only the badge is retired. `SearchHit` has no date field at
all, so the reference's date column has nothing to render and the repo
takes that slot until the additive `updated_at` ask lands. The status
groups gain `inconclusive`, the one docz status that had no mapping.
This surface is now the first place docz-site has deliberately diverged
from `mockup.html`, which keeps leading for prose and chrome tokens.

**Seventh amendment (2026-09-10, the serif body — OQ-12 RESOLVED).**
Four candidates were rendered on a real design doc and compared:
today's Mona Sans at 1.6, Mona Sans on the reference's airier rhythm,
Source Serif 4, and a Monaspace body. **Source Serif 4 wins**, which
reverses this design's original decision to drop the serif body
outright, and closes OQ-12 on an option none of (a)-(d) named.

The original diagnosis was not wrong, it was incomplete: the old reader
was hard to read because a 16px serif sat at `fg-secondary` grey on a
full-width unmeasured column. Fixing the color, the measure, the size,
and the leading fixes the serif — the face was never the problem. What
the sans could not fix was voice: with prose in Mona Sans, the Xenon
slab headings read as a second family on every page.

The article is now one family. Body is Source Serif 4 Variable at 19px
on 1.78 leading over a 66ch measure; h1-h4 are the same serif at 700;
pull quotes are its italic; `strong` is 650, since a serif's bold sits
closer to its regular than Mona Sans's did. Monaspace Xenon is removed
from the stack entirely. Mona Sans keeps every UI surface, so the
chrome still reads as an application around a document, and Monaspace
Neon is untouched. Article titles (reader, repo home, type, changelog)
join the serif; the directory and repos heroes stay sans, because they
label the app rather than a document.

**Eighth amendment (2026-09-11, the card's two ends).** Review asked
for the repo to join the id rather than sit across the card from it,
and for the vacated right-hand column to carry the updated stamp the
RFD-index references show. Both ends of the row are now two lines that
align with each other: `DESIGN-0001 / docz-site` over the title on the
left, `Sep 10, 2026` over `1:52 PM` on the right. The coordinate reads
as one phrase instead of asking the eye to travel sideways and
reassemble it, and the separator inherits `fg-muted` rather than taking
a border token, so every color in the line is one the contrast gate
already gets to see.

This supersedes the sixth amendment's "the repo takes that slot", but
not its reasoning about why the slot was empty. `SearchHit` still has
no timestamp property. The finding that changes the ask: docz-api
already HAS the value — `internal/search/types.go` stores `updated_at`
(Unix seconds) on the Meilisearch record and `internal/search/
client.go` declares it a sortable attribute — and `decodeHits` simply
never copies it onto the wire struct, so it never reaches the schema or
the generated type. The upstream ask is therefore two lines of Go plus
an additive schema property, not an indexing change.

Until it lands, `hitUpdatedAt` (`src/lib/updatedAt.ts`) reads the
property defensively, the way `apiConfig` and `changelogConfig` read
`config_snapshot`, and the column renders the em dash. The demo
fixtures forward each document's own `updated_at` so the surface is
reviewable under `dev:msw`; page hits send `""` there too, because
nothing in the contract dates a published page. The stamp formats in
the reader's own zone with the locale pinned to en-US, so month
abbreviations and the 12-hour clock cannot drift per machine.

**Ninth amendment (2026-09-15, diagrams — and the eighth's loose end).**

The column is no longer empty. docz-api shipped the two-line decode as
spec 1.5.0 (docz-api v0.9.1/v0.10.0), so `SearchHit.updated_at` is a
typed, required property on **both** record kinds, `hitUpdatedAt` is
deleted, and callers read the field directly. One correction to the
eighth amendment's closing paragraph: it is `created`, not
`updated_at`, that is `""` on a page hit — a published page has no
authored date, but it does have an ingest stamp. Fixture pages carry
one per repo, which is genuinely how onboard behaves.

The diagram decisions this document now owns (IMPL-0006, OQ-1 and
OQ-8):

- **ELK is the layout**, matching mermaid 12's own default, but named
  explicitly rather than inherited — a config that says which algorithm
  draws the diagrams, and one place for an override to replace. It
  arrives as its own ~436 KB gzipped chunk, fetched only by documents
  that contain a diagram and only when the layout is `elk`.
- **A deployment can select `dagre` without a rebuild**, through
  `DOCZ_MERMAID_LAYOUT` and the chart's `config.mermaidLayout`,
  following the runtime-config pattern established by
  `DOCZ_AUTH_PROVIDERS` and `DOCZ_NAV_LINKS` — closed set, validated at
  both ends. The e2e suite asserts both directions: ELK's chunk is
  fetched by default and never fetched under the override.
- **The `look` stays `classic`.** v12 moved the default to `neo`, which
  rounds node corners, thickens strokes, and adds a shadow. This
  document's own radius position is that the scale is wiped — sharp
  corners everywhere, `rounded-pill` the only exception — so `neo`
  would leave diagrams the single rounded surface on the site. Both
  were rendered on the specimen and compared before choosing. The
  monochrome policy is unchanged and now asserted on computed stroke
  rather than by eye: Figure 2's three `classDef` colors arrive
  verbatim, and every node the document does not color reads
  `--color-border-strong` as a flat `rgb()` — which is also the proof
  that `neo`'s node gradients stay off, since a gradient would read
  `url(#…-gradient)`.

**The security finding, recorded here because it predates the upgrade
and shipped for months.** `MermaidBlock`'s guarantee rests on
`securityLevel: "strict"` *and* `htmlLabels: false`, and mermaid lets a
diagram's own YAML front matter override config. Its default `secure`
list — the keys front matter may not set — covers `securityLevel` but
**not** `htmlLabels`. Audited against the installed 11.16.0 before the
version bump, the gap was real: a hostile `config:` block set
`htmlLabels: true` at both the global and the nested `flowchart` path,
re-enabling the exact vector strict mode does not close (a
purified-but-real `<img src>` inside a `foreignObject` label).
`MERMAID_SECURE_KEYS` closes it; one top-level entry covers every
nesting depth, because mermaid's directive sanitizer recurses. Covered
by `src/markdown/mermaid-config.test.ts` and by a second e2e figure
whose front matter attempts exactly this.



The doc title (`DocHeader` in `doc.tsx`) and the three page-level h1s
that use the `font-serif` utility switch to `font-sans` at weight 500:
long titles set in a monospace slab wrap to three lines in a 784px
column (0.6em × 40 characters at 41px ≈ 1000px), which is why h1s are
excluded from the Xenon slots even under OQ-5(a).

`body` stays at 15px sans; Mona Sans has a larger x-height than Plex
Sans, so UI text will read slightly larger at the same size. OQ-8
decides whether that is corrected globally or per surface.

One more `--font-serif` consumer hides outside the prose rules: the
note admonition's circled "i" icon (`.admonition.note::before`) is set
in the serif italic. It follows the token to Xenon italic under OQ-5(a)
and (b); under OQ-5(c) it switches to `--font-sans` italic when the
token is deleted. Either way the glyph is reviewed in the axe fixture
that renders all five admonition kinds.

### Component 3: the measured column

Text-bearing block children of `.doc-prose` get a maximum width; code
blocks, tables, mermaid figures, and images keep the full column so
wide content is not squeezed (OQ-3):

```css
.doc-prose :is(p, ul, ol, dl, h1, h2, h3, h4, blockquote, .admonition, hr) {
  max-width: 72ch;
}
```

`72ch` is measured in the element's own font, so a heading in Xenon
and a paragraph in Mona Sans each get a width proportional to their
glyphs. For Mona Sans at 16.5px this is roughly 700px and 80–85 average
characters per line, the Oxide measure. The rule applies to every
`.doc-prose` consumer (reader, repo home, type pages, changelog, page
reader) because they share the class; the raw-markdown `<pre>` view is
not `.doc-prose` and keeps the column. The `DocMetaTable` and header
sit above the article and are unaffected.

At ≥1181px the column is 784px, so the cap removes ~85px of measure;
between 861px and 1180px it removes ~330px, which is where the
complaint originated. Below 861px the frame is single-column and the
cap rarely binds.

### Component 4: inline code de-emphasis

```css
.doc-prose code {
  font-family: var(--font-mono);
  font-size: 0.875em;
  color: var(--color-fg-primary);
  background: var(--color-code-bg);
  padding: 0 0.3em;
}
```

Border removed, vertical padding removed (the line box no longer
grows), color moves from `accent` to `fg-primary` so the accent means
"link" and nothing else, size lifted from 0.82em to 0.875em because
Neon's x-height sits closer to Mona Sans's than Plex Mono's did to
Source Serif's. Because the rule sets `color` explicitly, a `code`
span inside a link would lose the link color; a companion
`.doc-prose a code { color: inherit; }` keeps linked code in the accent
so the "accent means link" rule has no exception (today both are accent
by coincidence, so no such rule exists yet). `.doc-prose pre code`
keeps its reset (`background: none; padding: 0; color: inherit`).
No stylistic sets on inline code: mid-sentence ligatures read as typos
and `calt` healing already applies by default.

`fg-primary` on `code-bg` (#161b28) is a pair `contrast.test.ts` does
not enumerate today (`code-bg` is not one of the three enforced
surfaces). The ratio is ~14:1; the test gains the pair so the invariant
is enforced rather than assumed.

**Amendment (2026-09-09, live review).** The quiet treatment was
rejected on sight: the blue-on-blue chip of the shipped site (accent
text, `code-bg`, hairline border) is the preferred look. Restored as
`color: accent; border: 1px solid border-hairline; padding: 0 0.35em`
at 0.875em — the size lift and the removal of vertical padding stay,
so the line box still does not grow. Consequences: the "accent means
link only" goal is withdrawn (links remain distinguishable by their
underline, which axe already requires); the contrast pair to add to
`contrast.test.ts` becomes `accent` on `code-bg` (~10.9:1) rather than
`fg-primary` on `code-bg`; OQ-4 is effectively option (b) with the
padding change.

**Task-list checkboxes (same review).** GFM `- [x]` items render as
disabled `<input type="checkbox">` elements (the default sanitize
schema keeps `type`, `disabled`, and `checked`), and browsers draw
disabled controls grey-on-grey, which is unreadable on the dark
surfaces. `.doc-prose` now draws them with `appearance: none`:
unchecked is the pill outline (`accent-border` on `accent-bg`), checked
is a filled `accent` box with a `bg-base` tick drawn by a rotated
`::before` border, and the box takes the bullet's place — the `li` that
directly contains a checkbox (or whose first paragraph does, for loose
lists) drops its marker via `:has()`, and the input carries a negative
left margin into the marker gutter so wrapped lines stay aligned with
ordinary items. The inputs stay `disabled`, so nothing becomes
interactive and the axe sweep is unaffected.

**Second amendment (2026-09-09, against the Oxide reference).** The
restored blue-on-blue was "too much blue" once every identifier in a
dense paragraph wore it. Oxide's convention is adopted instead:

- Plain inline code is a **neutral chip** — `fg-primary` text on
  `bg-elevated` with a `border-default` border (visible, unlike the
  hairline), so an identifier reads as text with a box around it.
- Code that **is a link** keeps the inherited accent text and gains an
  accent-tinted border (`accent-border`) and fill (`accent-bg`), so
  the color now carries meaning: blue chip means "this goes somewhere".
  The anchor's underline stays (axe link-in-text-block).
- **Body text brightens** via a new `--color-fg-prose` token
  (#c6ccd6, ~11.8:1 on base, 10.3:1 on elevated) used by `.doc-prose`,
  blockquotes, and admonition bodies. It is a deliberate step below
  `fg-primary` so bold and headings keep their lift, and it is scoped
  to prose so UI secondary text is untouched. This resolves OQ-10 as a
  variant of (b). The `fg-` prefix puts the token under
  `contrast.test.ts`'s existing sweep automatically; the admonition
  body assertion moves from `fg-secondary` to `fg-prose`.

### Component 5: code blocks, ligatures, italics

```css
.doc-prose pre {
  font-family: var(--font-mono);
  font-size: 13px;
  line-height: 1.55;
  font-feature-settings:
    "ss01", "ss02", "ss03", "ss04", "ss05", "ss06", "ss07", "ss08", "ss09";
}
```

Size moves from 12.5px to 13px (Neon is slightly narrower per em than
Plex Mono; the same 80-column snippet fits). All nine stylistic sets
are on (OQ-6, decided (b)): operators, comparisons, arrows, markup,
F# pipes, repeated `#`/`+`/`_`/`=`/`&`, colons, periods, and
greater/less-with-equals. The recommendation had omitted `ss05` and
`ss06` because `&&`, `##`, and `__` are structural in shell, Markdown,
and Python; the decision accepts that as cosmetic. If a fused glyph
ever misleads in a real sample, dropping a single set from this list is
the whole fix.

Italics need no CSS: Shiki already emits `font-style: italic` for the
theme's italic scopes; loading `monaspace-neon/400-italic.css` makes
those spans render Neon's true italic instead of a synthesized slant.
The raw-markdown `<pre>` in `doc.tsx` uses the `font-mono` utility and
gets Neon with default features only (no sets — it is source, not
code).

The codeblock header chrome (`.lang`, `.caption`), the mermaid
figcaption, table headers, and every mono UI label take Neon through
`--font-mono` with default features. Uppercase tracked labels
(`tracking-[0.14em]`, 10–11px) are reviewed on the demo org for glyph
fit; Neon's capitals are slightly wider than Plex Mono's.

### Component 6: mockup.html and documentation

`mockup.html` loads its fonts from Google Fonts, which is acceptable
for an unshipped design artifact but cannot serve Monaspace. OQ-9
(decided (a)) mirrors the new stack into the mockup: Mona Sans from
its Google Fonts link (Google serves it with italics; the family is
named `"Mona Sans"` there versus fontsource's `"Mona Sans Variable"`
in the app) and Monaspace Neon/Xenon from jsDelivr's copy of the
fontsource packages, pinned to `5.3.0` with the same weights
`main.tsx` imports. The mockup carries every rule from Components 2–5
verbatim, its demo comments render italic to stand in for Shiki's
italic scopes, and it gains one dense specimen document
(`DESIGN-0002 · Ownership digest delivery pipeline`, the reader's
default) whose paragraphs, list, blockquote, and code block exercise
the measure cap, inline-code density, and every ligature set. The
mockup review is Phase 0 of the rollout; the mockup stays the visual
source of truth. `CLAUDE.md`'s font paragraph is rewritten to
name the new families, the weight-import rule, the "ligature sets in
`pre` only" rule, and the measure cap, and `README.md`'s stack list is
updated. The IMPL doc for this design records the before/after
screenshots of the DESIGN-0003 page (the page from the original
complaint) as the acceptance artifact.

### Cross-cutting invariants

- **No third-party font requests from the app.** Every face is an
  `@fontsource` package bundled by Vite; a unit test (Testing Strategy)
  pins that `main.tsx` imports only `@fontsource` CSS and that every
  family named in `tokens.css` has a matching import, so a token and its
  import cannot drift apart silently.
- **Licensing.** Mona Sans and all Monaspace variants are SIL OFL 1.1.
  Berkeley Mono is explicitly out (commercial, domain-bound web
  license, no redistribution).
- **Sanitizer and pipeline untouched.** Nothing in `schema.ts`,
  `processor.ts`, or the Shiki transformer changes. Ligatures and
  italics are properties of the loaded font files and CSS.
- **Color tokens untouched** unless OQ-10 chooses a lift, in which case
  ratios only rise and both contrast gates stay green by construction.

## API / Interface Changes

None to docz-api or the chart. Repository-level changes:

- `package.json`: three `@fontsource` dependencies removed, three
  added.
- `src/main.tsx`: font imports replaced (seven files).
- `src/theme/tokens.css`: `--font-*` values, `.doc-prose` sizes, the
  measure cap, the inline-code rule, the `pre` feature settings.
- Four `font-serif` utility usages become `font-sans font-medium`.
- `CLAUDE.md`, `README.md`, and (per OQ-9) `mockup.html`.

No runtime configuration, no `__DOCZ_CONFIG__` keys, no chart values.

## Data Model

None.

## Testing Strategy

- **Rendering specimen (added 2026-09-09)**:
  `docs/guides/markdown-specimen.md` renders every construct the
  pipeline handles on one published page — a `?raw` fixture page under
  `dev:msw` and a real page in production, since `.docz.yaml` now
  carries the `api:` block — and both axe sweeps render it. Its first
  run surfaced three pre-existing defects, fixed alongside it:
  task-list checkboxes had no accessible name (critical; now
  `MarkdownInput` labels them Done / Not done), tokyo-night's comment
  family sat at ~2.5:1 on `code-bg` (serious; `theme-contrast.ts` lifts
  every failing token color to AA at highlighter load, keeping italics),
  and wide tables pushed into the rail instead of scrolling
  (`wrap-table.ts`). Long unbroken tokens now wrap
  (`overflow-wrap: break-word` on `.doc-prose`) and h5/h6 gained
  styles. Judge typography changes on the specimen first.
- **Contrast (`src/theme/contrast.test.ts`)**: unchanged tokens keep
  the existing assertions; add the `fg-primary` on `code-bg` pair that
  Component 4 introduces. If OQ-10 lifts `fg-secondary`, the existing
  assertions cover it.
- **Font wiring (new `src/theme/fonts.test.ts`)**: reads `main.tsx` and
  `tokens.css` through `node:fs` (the same pattern `contrast.test.ts`
  uses for CSS) and asserts (a) every import matching `/fontsource/`
  is a local package path, never a URL; (b) each family named in a
  `--font-*` token has at least one import whose package name matches;
  (c) `.doc-prose pre` declares `font-feature-settings` and
  `.doc-prose code` does not; (d) the measure cap rule exists. This is
  a drift guard, not a rendering test.
- **Axe unit sweep (`src/a11y/axe.test.tsx`)**: no new views; runs
  as-is. jsdom does not load fonts, so it cannot regress on this change.
- **e2e (`e2e/a11y.spec.ts`, full-rule axe including contrast)**: runs
  as-is against the MSW preview build. Add one assertion to the reader
  spec: `document.fonts.check('16px "Mona Sans Variable"')` and
  `document.fonts.check('13px "Monaspace Neon"')` both true after the
  doc renders, so a broken import surfaces as a test failure instead of
  a silent fallback to the system font.
- **Ligature smoke (e2e)**: a fixture code block containing `=>` and
  `!=` renders with computed `font-feature-settings` including `ss01`
  and `ss03` on the `pre`, and the inline-code span in the same doc
  reports `normal`. Glyph shapes are not asserted (no reliable API);
  the feature settings are the contract.
- **Bundle budget (`just bundle-budget`)**: unaffected by design
  (fonts are CSS assets, not modulepreloaded JS) but run in the chain
  as always.
- **Visual acceptance**: before/after screenshots of the DESIGN-0003
  reader page at 1440px and 1024px viewports, attached to the IMPL doc.
  This is the only check that answers the original question ("does it
  read better"), so it is recorded, not automated.

## Migration / Rollout Plan

One branch, `feat/reader-typography`, carrying this design, its IMPL
doc, and the change. The IMPL doc phases the work so each step is
independently revertable and screenshot-able:

0. Live review (done on this branch before the IMPL doc exists): the
   decided stack is applied to the app itself — Components 1–5 as CSS
   and imports, no tests yet — so it can be reviewed on real documents
   with `bun run dev:msw` (the demo org's own design docs and the
   published guide page). `mockup.html` carries the same values so the
   two do not drift. Dial-in edits happen in `tokens.css` and are
   mirrored to the mockup; this design is amended if a decision
   changes. The IMPL doc then tracks the remaining phases (tests,
   documentation, e2e assertions, release).
1. Font swap only (Component 1) — the stack changes, sizes do not.
   Screenshot.
2. Prose sizes, faces, and the measure cap (Components 2 and 3).
   Screenshot.
3. Inline code and code-block features (Components 4 and 5), plus the
   contrast pair and the font-wiring test.
4. Mockup and documentation (Component 6), e2e assertions, CLAUDE.md.

Release as a minor version (visible change, no API impact): docz-site
`v0.7.0`, chart `0.1.8` with `appVersion` `0.7.0` (OQ-11). Rollback is
a git revert; no data or configuration migrates.

## Open Questions

Each question lists **(a)** as the recommendation and the alternatives
after it. **All decided 2026-09-08:** (a) for every question except
OQ-6, which takes (b). The options are kept for the record.

**OQ-1. Body face for prose and UI.** _Decided: (a)._

- (a) Mona Sans, variable weight-only file (`wght.css` +
  `wght-italic.css`, ~82 KB latin). Designed as Monaspace's
  proportional companion; large x-height and even strokes hold up on
  dark; one file per style covers every weight.
- (b) Inter, variable weight-only (~48 KB upright). The safe generic
  choice; slightly heavier per style once italic is added and visually
  unrelated to Monaspace.
- (c) Keep IBM Plex Sans. Zero new bytes, but it belongs to the family
  being removed and reads as a leftover next to Monaspace.
- (d) Keep Source Serif 4 for paragraphs at 17px with the variable
  package at weight 450. Preserves the serif identity; does the least
  for the thin-on-dark problem.

**OQ-2. Prose size and leading.** _Decided: (a)._

- (a) 16.5px / line-height 1.65 / paragraph gap 1.25rem. Matches the
  Oxide reference's apparent size and keeps ~27px of leading.
- (b) 16px / 1.7 / 1.1rem (today's values, only the face changes).
- (c) 17px / 1.6 / 1.25rem. Larger and slightly tighter; pushes the
  ≥1181px column to ~88 characters even before the cap.

**OQ-3. Where the measure cap applies.** _Decided: (a)._

- (a) Cap text-bearing children at `72ch` (paragraphs, lists,
  headings, blockquotes, admonitions); code blocks, tables, mermaid
  figures, and images keep the full column. Wide content is not
  squeezed and no grid change is needed.
- (b) Cap the whole `.doc-prose` article at `72ch`. Simplest rule;
  tables and code blocks narrow with it and rely on horizontal scroll.
- (c) Narrow the `RepoFrame` content column instead (e.g. `minmax(0,
  760px)`). Also narrows the doc header, meta table, and page reader
  chrome; changes three layouts to fix one.
- (d) No cap; rely on the font change alone. Leaves the 120–145
  character lines in the 861px–1180px band.

**OQ-4. Inline code treatment.** _Decided: (a)._

- (a) `fg-primary` text on the faint `code-bg`, no border, no vertical
  padding, `0.875em`. Accent becomes link-only; line boxes stop
  inflating.
- (b) As (a) but keep the 1px hairline border. Slightly more chip-like;
  clearer edges on very short spans like `x`.
- (c) No background at all: mono face and `fg-primary` only. Quietest;
  code spans become hard to spot when scanning.
- (d) Keep the accent color, drop only the border and padding. Keeps
  the mockup's blue but leaves links and code sharing a color.

**OQ-5. Monaspace Xenon's slots (the former serif slots).** _Decided: (a)._

- (a) Xenon 600 for `h2`–`h4` section headings and blockquote pull
  quotes; the doc title and page h1s in Mona Sans 500. Long titles wrap
  badly in a wide monospace slab; section headings are short enough.
- (b) Xenon everywhere Source Serif 4 was, including the doc title h1
  at `clamp(1.9rem, 4.5vw, 2.6rem)`. Strongest RFD flavor; titles over
  ~30 characters wrap to three lines at 784px.
- (c) No Xenon: Mona Sans for every heading, `--font-serif` token
  deleted, one fewer family (~45 KB saved). Cleanest stack; loses the
  slab accent entirely.

**OQ-6. Ligature sets enabled in code blocks.** _Decided: (b)._

- (a) `ss01 ss02 ss03 ss04 ss07 ss08 ss09` — operators, comparisons,
  arrows, markup, colons, periods. Omits `ss05` (F# pipes) and `ss06`
  (repeated `#`, `+`, `_`, `=`, `&`), which alter glyphs that are
  structural in Markdown, YAML, and shell — most docz content.
- (b) All nine sets.
- (c) None: texture healing and `liga` only (both default-on). No
  ligatures anywhere; italics still land.

Inline code and UI mono labels get no sets under every option; that is
a decision, not a question.

**OQ-7. Monaspace Neon weights to load.** _Decided: (a)._

- (a) 400, 400 italic, 600, 700 (~182 KB latin). Covers `font-mono`
  paired with `font-semibold` (2 usages) and `font-bold` (1 usage, the
  brand mark) without synthesized bold.
- (b) 400, 400 italic, 600 (~137 KB) and restyle the single
  `font-bold` brand usage to 600. Saves one file; the brand mark gets
  lighter.
- (c) Add 500 for the `.lang` badge's `font-medium` (~45 KB more). It
  resolves to 400 today and nobody has noticed; not recommended.

**OQ-8. UI text after the sans swap.** _Decided: (a)._

- (a) Keep every non-prose size as-is, review the topbar, directory,
  palette, and RepoNav on the demo org, and adjust only where Mona
  Sans's larger x-height causes wrapping or crowding.
- (b) Drop `body` from 15px to 14.5px globally to compensate for the
  x-height, then review. More uniform, more churn in the diff.

**OQ-9. Keeping `mockup.html` truthful.** _Decided: (a)._

- (a) Update the mockup: Mona Sans via its existing Google Fonts link
  (Google serves it), Monaspace Neon/Xenon via jsDelivr's copy of the
  fontsource CSS. The no-third-party rule governs the shipped app, not
  the unshipped design artifact, and the mockup stays the visual
  source of truth.
- (b) Leave the mockup on Plex/Source Serif and amend `CLAUDE.md` so
  `tokens.css` is the typography source of truth while the mockup
  remains the layout/color reference. No mockup diff; two sources of
  truth by dimension.
- (c) Vendor the woff2 files into the repository for the mockup. Adds
  ~300 KB of binaries to git for a design artifact; not recommended.

**OQ-10. Body text color.** _Decided: (a)._

- (a) Keep `fg-secondary` (#afb6c2, 9.34:1). The face and measure
  changes are expected to remove the perceived dimness; re-evaluate on
  the after-screenshots before touching tokens.
- (b) Lift body prose to a new `fg-prose` token around #c3c9d3
  (~11.5:1) and leave `fg-secondary` alone for UI. Adds a token and a
  contrast assertion.
- (c) Set prose in `fg-primary` (15.9:1). Brightest; `strong` loses
  its color contrast against body and must rely on weight alone.

**OQ-11. Release versioning.** _Decided: (a)._

- (a) Minor: docz-site `v0.7.0`, chart `0.1.8` / `appVersion 0.7.0`.
  A visible presentation change users will notice.
- (b) Patch: `v0.6.1`, chart `0.1.8`. Nothing functional changed.

**OQ-12. Body face after live review.** _Open._ Raised by the third
amendment under Component 2: at 450 the page is sturdier but still
reads "airy" next to Oxide's RFD site, whose body is Suisse Int'l, a
neo-grotesque with a tall x-height and low stroke contrast.

- (a) Switch the sans to Inter (`@fontsource-variable/inter`, OFL), the
  nearest open neighbor to Suisse. Injected on the same page at 450 it
  is the closest match to the reference texture, and its narrower
  letterforms fit ~5% more words into the 72ch measure. Mona Sans drops
  from the stack entirely (UI and h1 follow), the mockup's Google Fonts
  link swaps, and this document's title becomes historical.
- (b) Keep Mona Sans at 450 (what is applied). No further churn; the
  wide, rounded letterforms remain the residual airiness.
- (c) Mona Sans at 500 with `strong` at 650. Reads solid in the same
  comparison, but the page goes monotone-heavy; last resort before (a).

**RESOLVED 2026-09-10: none of the above — Source Serif 4, see the
seventh amendment.**
- (d) Monospace body — **recommended after the size review.** Monaspace
  Argon 16px / 1.65 as the body face, 76ch measure, Neon kept for code,
  Xenon for headings: the shape of PlanetScale's blog and the request
  this design originally argued away from. On the specimen it reads
  calm and cohesive with the site's mono chrome; the costs are ~20%
  taller documents and inline code chips distinguished from body only
  by their box (Argon and Neon share metrics, so the seam is invisible).
  Adds `@fontsource/monaspace-argon` 400 / 400-italic / 600 (~135 KB of
  woff2 as CSS assets, outside the JS budget).

## References

- Monaspace: <https://github.com/githubnext/monaspace> (OpenType
  feature list and texture healing, README; SIL OFL 1.1).
- Mona Sans: <https://github.com/github/mona-sans> (SIL OFL 1.1).
- Fontsource packages: `@fontsource-variable/mona-sans`,
  `@fontsource/monaspace-neon`, `@fontsource/monaspace-xenon` (all
  `5.3.0`, sizes measured from the npm tarballs on 2026-09-08).
- Oxide RFD reader (the readability reference):
  <https://rfd.shared.oxide.computer>.
- WCAG 2.2 SC 1.4.6 Contrast (Enhanced): 7:1 body, 4.5:1 large text.
- CSS Fonts Level 4, `font-variant-ligatures: normal` — `calt` and
  `liga` on by default; stylistic sets off.
- DESIGN-0001 (stack and mockup inheritance), IMPL-0002 Phases 2 and
  6 (codeblock chrome, heading anchors — the surfaces Component 5
  touches), `src/theme/contrast.test.ts`, `e2e/a11y.spec.ts`.
- `CLAUDE.md` "Fonts are self-hosted" rule and the bundle-budget
  definition (eager JS only).
