# docz-html

A Claude Code skill that turns a docz document (an RFC, ADR, design,
IMPL, investigation, or runbook) into one standalone HTML page: a
header with the summary and status, a tab per section, cards, callouts,
phase progress, and the light/dark toggle, in docz's own theme. The
page is one file with no network requests, so it opens from disk, an
email attachment, or a chat upload.

The page comes from `docz export html`, which the skill drives (stage 2
of DESIGN-0025 §10); the skill decides when a page is worth making and
how to make it read better by improving the document's markdown.

## Install

Add this repository as a marketplace and install the plugin, inside
Claude Code:

```text
/plugin marketplace add donaldgifford/docz
/plugin install docz-html@docz
```

The skill runs `docz export html`, so docz v2.4.0 or later must be on
the `PATH`.

## Use

Ask for it by name or by what you want:

```text
make an HTML page for DESIGN-0025
/docz-html:docz-html IMPL-0028
```

The page is written under `build/html/`, at the document's path below
`docs/`. The skill never edits the
document to suit the page; when the page would read better, it suggests
the change to the markdown.

## What the skill carries

| Path | What it is |
| --- | --- |
| `skills/docz-html/SKILL.md` | The instructions |
| `skills/docz-html/references/layout.md` | Which region goes in which tab, per type |
| `skills/docz-html/references/widgets.md` | The `docz-flow`, `docz-stepper`, and `docz-scorecard` blocks |
| `skills/docz-html/references/components.md` | The page's markup, as `docz export html` writes it |

Stage 1 carried a copy of docz's theme and had the agent assemble the
page by hand; `docz export html` inlines the theme itself, so the copy is
gone.
