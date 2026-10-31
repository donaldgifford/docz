---
name: docz-html
description: >-
  Make a standalone HTML page from a docz document (an RFC, ADR, design,
  IMPL, investigation, or runbook) in docz's theme, for a reader outside
  the repository. Use when the user asks for an HTML version, a
  walkthrough page, a page to share, or "docz html" for a document.
argument-hint: "<id|path> [classic]"
allowed-tools: Bash(docz inspect *), Bash(docz validate *), Read, Write
---

# docz-html

Turn one docz document into one HTML file a person outside the
repository can open: no network, no build step, the theme inlined.

The document is `$ARGUMENTS` (an id like `DESIGN-0025` or a path under
`docs/`). A second argument `classic` picks the classic theme; otherwise
the page is `paper`.

## When

Make a page when someone asks for it: an HTML version, a page to send to
someone without the repository, a walkthrough of a design or a plan. Do
not make one unasked, and do not make one for a document that is not a
docz document (it has no docz frontmatter and no id).

## Read the document through docz

Never parse the markdown yourself for structure. Run:

```bash
docz inspect <id> --format json
```

It gives you everything the layout needs:

- `frontmatter`: id, title, status, author, created, for the header;
- `regions`: every region in document order, with `kind`, `start` and
  `end` lines, and `depth`; and `inferred`, true when docz found them from
  headings because the document has no markers;
- `headings`: each heading's `text`, `level`, `line`, and `anchor`, the id
  the heading must carry on the page;
- `tasks`: every checkbox with its line, text, and `checked`;
- `open_questions`: each question's number, title, options (with
  `recommended`), and `resolved`;
- `content`: the document's markdown, and `content_sha256` for the
  footer.

A region's markdown is the lines strictly between its `start` and `end`.
Run `docz validate <id>` too: a finding usually means a region the page
cannot lay out, and is worth telling the user about.

## Assemble the page

1. Read `references/layout.md` and place every region in its tab.
2. Read `references/components.md` and write each region with the
   components it names. Use only the classes it lists.
3. For any `docz-flow`, `docz-stepper`, or `docz-scorecard` fenced block,
   read `references/widgets.md` and write the widget: its JSON island, its
   mount point, and its fallback.
4. Inline the theme from this skill's `theme/` directory: `base.css`,
   then `tokens/<theme>.css`, then `components.css` in one `<style>`, and
   `runtime.js` in one `<script>` at the end of the body. Read the files;
   never retype or abridge them.
5. Write the page to `build/html/<ID>.html` under the repository root
   (create the directory). `build/` is not committed.

The page must open from `file://` with no request leaving it and no error
in the console. Escape every piece of document text. Do not add
JavaScript of your own: the runtime does tabs, the mode toggle, copy
buttons, the task filter, and the widgets.

## Never edit the document to suit the page

The page shows the document as it is. When it would read better (no
summary paragraph for the header, alternatives in one long paragraph
instead of one subsection each, a risks list that should be a table, a
flow in prose that a `docz-flow` block would show), tell the user what to
change in the **markdown**, and change it only if they ask. A change to
the markdown is the one kind of edit worth making: the next page, and
every other reader of the document, gets it too.

## Report

Say where the page is, which tabs it has, and anything you could not
place: a region kind the layout table does not list, a widget whose YAML
did not validate, a link that resolves nowhere. Open the page only when
the user asks to see it (`open build/html/<ID>.html` on macOS).
