---
id: ADR-0002
title: "Vendor the markdown walker"
status: Accepted
author: Parity Fixture
created: 2026-03-04
---

<!-- markdownlint-disable-file MD025 MD041 -->

# ADR-0002: Vendor the markdown walker

<!--toc:start-->
- [Summary](#summary)
- [Context](#context)
- [Decision](#decision)
  - [Supporting Data](#supporting-data)
- [Consequences](#consequences)
  - [Positive](#positive)
  - [Negative](#negative)
  - [Neutral](#neutral)
- [Alternatives Considered](#alternatives-considered)
- [References](#references)
<!--toc:end-->

<!--docz:summary:start-->
## Summary

We are vendoring a small, line-oriented markdown walker directly into
the tree instead of pulling in a full CommonMark parser as a third
party dependency. The walker only needs to answer a handful of
factual questions about a document (headings, task items, the title)
and a hand-rolled scanner gets us there with far less surface area
than a spec-complete parser would.
<!--docz:summary:end-->

<!--docz:context:start-->
## Context

`docparse` needs to extract headings, checkbox task items, and an
optional H1 title from arbitrary markdown files that live in a user's
repo. We evaluated pulling in a CommonMark-compliant library early on,
but every candidate we tried (`goldmark`, `blackfriday`, `mdp/parser`)
pulls in an AST, a renderer, and a extension registry we would never
use. The module's public surface is supposed to stay small and
bytes-in/values-out (see ADR-0001 and INV-0006), and a general purpose
parser drags in exactly the kind of interface-heavy dependency graph
that philosophy is meant to avoid.

**Status quo pain:** every minor version bump of a CommonMark library
we tested changed heading-anchor slugging or fence handling in some
subtle way, which broke golden fixtures downstream in docz-api without
any change on our side. We do not want our own release cadence coupled
to a third party's.
<!--docz:context:end-->

<!--docz:decision:start-->
## Decision

We will write and vendor a small, dependency-free line scanner inside
`pkg/doczcore/docparse` that walks a document line by line and
recognizes exactly the constructs docz cares about: ATX headings
(H2-H6), GFM task-list checkboxes, and a single H1 or setext title.
Anything outside that grammar (tables, nested blockquotes, inline
HTML) is treated as opaque content and passed through unexamined.

A rough sketch of the walker's inner loop looks like this:

```go
func Headings(content []byte) []Heading {
    var out []Heading
    inFence := false
    for i, line := range splitLines(content) {
        trimmed := bytes.TrimSpace(line)
        if isFenceToggle(trimmed) {
            inFence = !inFence
            continue
        }
        if inFence {
            continue
        }
        if h, ok := parseATXHeading(trimmed, i+1); ok {
            out = append(out, h)
        }
    }
    return out
}
```

The scanner is intentionally line-oriented rather than block-oriented,
which keeps it fast and keeps the mental model close to what a human
reading the raw markdown would infer.

### Supporting Data

We benchmarked the vendored walker against `goldmark` on the module's
own `docs/` tree (currently 47 files) purely for heading and task-item
extraction:

| Approach            | Time (47 files) | Allocations | Binary size delta |
| -------------------- | ---------------- | ----------- | ------------------ |
| vendored walker       | 1.2ms            | 340         | +0 KB (no dep)      |
| goldmark (AST walk)   | 6.8ms            | 5,100       | +740 KB             |
| blackfriday (visitor) | 4.1ms            | 2,900       | +410 KB             |

The vendored walker is roughly 5x faster for this narrow use case
because it never builds an intermediate tree, and it adds nothing to
the dependency graph docz-api and downstream consumers have to
vet.
<!--docz:decision:end-->

<!--docz:consequences:start-->
## Consequences

Vendoring the walker trades a wide, well-tested dependency surface for
a narrow one we own outright. The tradeoffs mostly land where you
would expect: less risk from upstream churn, more responsibility for
edge cases we used to get for free.

**Owner:** docz core team

<!--docz:positive:start-->
### Positive

- No third-party markdown dependency to track for security advisories
  or breaking changes.
- The docz core team owns the entire parsing surface end to end.
- Fast and low-allocation, since there is no intermediate AST to
  build and discard for a handful of facts.
- Behavior is pinned by our own golden fixtures and fuzz tests
  instead of a moving upstream target.
  - This also means a fence-handling bug fix ships on our schedule,
    not whenever the upstream maintainer gets to a similar issue.
<!--docz:positive:end-->

<!--docz:negative:start-->
### Negative

- We now own bugs in markdown edge cases (nested fences, nonstandard
  list markers) that a mature parser would already have handled.
- The walker does not build a full AST, so any future feature that
  needs real nesting (for example arbitrary block quoting of
  headings) would require a rewrite rather than an incremental
  extension.
<!--docz:negative:end-->

<!--docz:neutral:start-->
### Neutral

- The walker's grammar is deliberately narrower than CommonMark,
  which is fine for docz's own templates but means it is not a
  general purpose markdown tool and should not be marketed as one.
<!--docz:neutral:end-->
<!--docz:consequences:end-->

<!--docz:alternatives:start-->
## Alternatives Considered

- **A. Take the dependency.** Pull in `goldmark` (the most actively
  maintained CommonMark implementation in Go) and walk its AST for the
  facts we need. Pro: spec compliance and battle-tested fence/table
  handling. Con: pulls a large transitive surface into a module whose
  whole pitch is a minimal, promoted API; version bumps upstream
  would ripple into our golden fixtures without any code change on
  our side.
- **B. Regex-only extraction.** Skip a walker entirely and pull
  headings and task items with a handful of regular expressions. Pro:
  even less code than the line-oriented walker. Con: fence-awareness
  and duplicate-anchor slugging are genuinely stateful problems that
  regex handles poorly; we tried an early spike of this and it broke
  on the first document with a code block containing a `#` comment.
- **C. Shell out to a CLI markdown tool.** Invoke `mdp` or a similar
  binary as a subprocess and parse its output. Pro: reuses an
  existing, tested renderer. Con: adds a runtime dependency outside
  the Go toolchain, complicates cross-platform builds, and is far
  slower than an in-process scan for a CLI that already runs on every
  `docz update`.
<!--docz:alternatives:end-->

<!--docz:references:start-->
## References

- [ADR-0001: Use PostgreSQL for the registry store](0001-use-postgresql-for-the-registry-store.md)
- [INV-0006: doczcore public-API philosophy](../inv/0006-doczcore-public-api-philosophy.md)
- [goldmark](https://github.com/yuin/goldmark)
- [CommonMark spec](https://spec.commonmark.org/)
<!--docz:references:end-->
