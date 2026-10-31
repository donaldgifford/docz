---
name: docz-html
description: >-
  Make a standalone HTML page from a docz document (an RFC, ADR, design,
  IMPL, investigation, or runbook) in docz's theme, for a reader outside
  the repository. Use when the user asks for an HTML version, a
  walkthrough page, a page to share, or "docz html" for a document.
argument-hint: "<id|path> [classic]"
allowed-tools: Bash(docz export html *), Bash(docz validate *), Bash(open build/html/*), Read, Edit
---

# docz-html

Turn one docz document into one HTML file a person outside the
repository can open: no network, no build step, the theme inlined.
`docz export html` makes the page; this skill decides when to make one
and how to make it read better.

The document is `$ARGUMENTS` (an id like `DESIGN-0025` or a path under
`docs/`). A second argument `classic` picks the classic theme; otherwise
the page is `paper`.

## When

Make a page when someone asks for it: an HTML version, a page to send to
someone without the repository, a walkthrough of a design or a plan. Do
not make one unasked, and do not make one for a file that is not a docz
document (it has no docz frontmatter and no id).

## Make the page

1. Validate first:

   ```bash
   docz validate <id>
   ```

   A finding usually means a region the page cannot lay out. Tell the
   user about any finding, and go on: the page still renders.

2. Export:

   ```bash
   docz export html <id>             # paper
   docz export html <id> --theme classic
   ```

   The page is written under `build/html/`, at the document's path below
   `docs/` with `.html` for `.md`; the command's report names it. A page
   whose bytes already match is reported `unchanged`. `build/` is not
   committed.

3. Report the path, the tabs the page has if the user asked about them,
   and anything the report lists: an unresolved link, or a problem such
   as a widget whose YAML did not validate. Open the page only when the
   user asks to see it (`open build/html/<path>.html` on macOS).

## Make it read better through the markdown

Never edit the HTML: the next export replaces it. When the page would read
better, the change belongs in the **markdown**, where every other reader
of the document gets it too. Suggest it, and make it only if the user
asks:

- a one-paragraph summary that states the scope and the ask, which
  becomes the page's header;
- alternatives as one subsection each, which become one card each, the
  chosen one marked "(chosen)" in its heading;
- risks and decisions as tables;
- a `docz-flow`, `docz-stepper`, or `docz-scorecard` block, or a mermaid
  diagram, where a flow or a scenario carries more than prose
  (`references/widgets.md` has each block's schema; an invalid block
  shows as its source with the error);
- open questions as numbered `###` headings with lettered `- a.` options
  and a `> **Resolved <date>: (a).**` line when settled.

`references/layout.md` says which region goes in which tab, so it tells
you which section of the markdown a part of the page comes from. Then
validate and export again.
