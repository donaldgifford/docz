# docz-review

A Claude Code plugin that holds the end of a turn until you have reviewed
every docz document the agent created in it. When the turn ends, a page
opens in your browser with each document rendered, its open questions as
choices, its validation findings, and room to comment on any passage.
Approve, ask for changes, or dismiss each one. Changes go back to the
agent as its next instruction, and the next round shows what it changed.

The plugin is two hooks and a skill. The work is done by the
`docz-review` binary, which ships with docz's releases and is installed
separately.

## Install

Install the binary and docz with mise. Both come from the same
repository, so `docz-review` needs a tool alias:

```toml
[tool_alias]
docz-review = "github:donaldgifford/docz"

[tools]
docz = "latest"
docz-review = { version = "latest", asset_pattern = "docz-review_{{ os(macos=\"darwin\") }}_{{ arch(x64=\"amd64\") }}", bin = "docz-review" }
```

Then add this repository as a marketplace and install the plugin from
it, inside Claude Code:

```text
/plugin marketplace add donaldgifford/docz
/plugin install docz-review@docz
```

docz-review needs docz 2.2.0 or later. Check what it found with:

```bash
docz-review --version
```

Without the binary the plugin never blocks a session. After a
`docz create` it shows one message naming this page; at the end of a
turn it writes one line to stderr, which Claude Code shows only with
`claude --debug`.

When a review asks for changes, Claude Code shows the feedback as
"Stop hook error:" followed by the annotations. It is not an error:
that is how Claude Code labels a `Stop` hook that sends the agent back
to work.

The plugin needs Claude Code 2.1.280 or later, the oldest version
observed to honour the hooks' `if` filter and the `Stop` hook's 24-hour
timeout. See [the hook drill](docs/hook-drill.md) for what was observed.

## What arms a review

A document is queued for review, or armed, in two ways:

- **`docz create`** run by the agent. The hook reads the path from the
  command's output. Pass `--format json` to `docz create` so the hook
  reads a JSON envelope rather than a `Created` line.
- **`/docz-review:review <id|path>...`** for a document the agent did
  not create in this session, such as one it edited. With no arguments
  the skill lists what is queued.

Each candidate is checked through `docz inspect` before it is recorded.
One that does not resolve is named in a message to you and skipped. A
session that never runs either sees nothing.

## States

Each armed document is in one of five states:

| State | Meaning |
| --- | --- |
| `pending` | Armed; the page opens it when the turn ends |
| `open` | On the page now |
| `changes` | You asked for changes; it opens again at the next turn end |
| `approved` | Done |
| `dismissed` | Closed without feedback |

`docz-review pending` lists the current repository's documents and their
states. `docz-review pending --purge` removes state older than 30 days.

## Walking away

The page holds the agent while it is open, and you can leave it:

- **Close the tab.** After two minutes with no page connected, every
  document still open is dismissed and the turn ends.
- **Click Done.** The same, at once.
- **Do nothing.** The review ends a minute before the hook's 24-hour
  timeout.
- **Interrupt the session.** The review ends and dismisses what is open.

A dismissed document is never reopened unless it is armed again.

## Round limit

A document that has been through ten rounds since it was last armed is
dismissed instead of opened, with a message saying so, so an agent and a
reviewer who disagree cannot loop forever. Arming it again starts the
count over. The hook always uses the default. `--max-rounds N` sets the
limit for a review you start by hand:

```bash
docz-review DESIGN-0024 --max-rounds 3
```

## From your own marketplace

A team with its own private marketplace can offer docz-review there in
either of two ways.

**Reference this repository.** Add it beside your own, so your users
install from both:

```text
/plugin marketplace add donaldgifford/docz
/plugin install docz-review@docz
```

To make the reference part of a project, list it in the project's
`.claude/settings.json`:

```json
{
  "extraKnownMarketplaces": {
    "docz": {
      "source": { "source": "github", "repo": "donaldgifford/docz" }
    }
  },
  "enabledPlugins": { "docz-review@docz": true }
}
```

**Copy the directory.** Copy `plugins/docz-review/` from this repository
into your marketplace's repository and add an entry to its
`.claude-plugin/marketplace.json`:

```json
{
  "name": "docz-review",
  "source": "./plugins/docz-review",
  "description": "Blocking browser review of the docz documents an agent creates."
}
```

Your users then run `/plugin install docz-review@<your marketplace>`.
To resync, compare the `version` in this directory's
`.claude-plugin/plugin.json` with your copy's and copy the directory
again when it moves. The plugin's version changes whenever its hooks or
skill do, independent of docz's releases.

Either way, the binary is installed separately, as above.

## What to expect

A transcript of a first review lands here after the trial in IMPL-0027
Phase 5.
