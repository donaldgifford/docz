# docz-html

A Claude Code skill that turns a docz document (an RFC, ADR, design,
IMPL, investigation, or runbook) into one standalone HTML page: a
header with the summary and status, a tab per section, cards, callouts,
phase progress, and the light/dark toggle, in docz's own theme. The
page is one file with no network requests, so it opens from disk, an
email attachment, or a chat upload.

This is **stage 1** of the skill (DESIGN-0025 §10): the agent assembles
the page by hand from the document, against the theme the skill carries.
It is how the design is iterated before `docz export html` encodes it.
Once that command ships, the skill drives it instead.

## Install

Add this repository as a marketplace and install the plugin, inside
Claude Code:

```text
/plugin marketplace add donaldgifford/docz
/plugin install docz-html@docz
```

The skill reads documents with `docz inspect`, so docz v2.2.0 or later
must be on the `PATH`.

## Use

Ask for it by name or by what you want:

```text
make an HTML page for DESIGN-0025
/docz-html:docz-html IMPL-0028
```

The page is written to `build/html/<ID>.html`. The skill never edits the
document to suit the page; when the page would read better, it suggests
the change to the markdown.

## What the skill carries

| Path | What it is |
| --- | --- |
| `skills/docz-html/SKILL.md` | The instructions |
| `skills/docz-html/references/components.md` | The page skeleton and every component's markup |
| `skills/docz-html/references/layout.md` | Which region goes in which tab, per type |
| `skills/docz-html/references/widgets.md` | The `docz-flow`, `docz-stepper`, and `docz-scorecard` blocks |
| `skills/docz-html/theme/` | A copy of docz's `pkg/theme`, inlined into every page |

The theme is a copy because a marketplace install copies only this
directory. It is written by `just plugin-theme-sync` and checked against
`pkg/theme` in CI; edit `pkg/theme`, never the copy.
