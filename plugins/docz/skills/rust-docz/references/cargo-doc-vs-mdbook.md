# cargo doc vs mdbook

Two doc generators in the Rust ecosystem; they answer
different questions.

## Tools

| Tool | Purpose | Source | Output |
|---|---|---|---|
| `cargo doc` | API docs from `///` comments and item signatures | `.rs` source files | HTML at `target/doc/` |
| `rustdoc` (used by `cargo doc`) | The underlying renderer | — | — |
| `mdbook` | Long-form prose docs | `.md` files in `src/` | HTML book |

## When to use which

| Need | Tool |
|---|---|
| "Document my crate's public API for consumers" | `cargo doc` |
| "Write a tutorial / guide / book" | `mdbook` |
| "Document a library's design rationale" | both — API in `cargo doc`, design in `mdbook` |
| "Publish to docs.rs" | `cargo doc` (docs.rs is rustdoc) |

`cargo doc` is **structural** — it walks the type graph
and emits one HTML page per type/trait/function. mdbook
is **narrative** — you write chapters in markdown and it
emits a sequential book.

## What this skill uses

`docz rust-docz` runs `cargo doc` and converts the HTML
output to markdown. It does **not** invoke `mdbook` —
that's a separate authoring workflow.

If the user has an mdbook setup (`book.toml` at the repo
root), this skill detects it and steps aside. The
reasoning:

- Users with mdbook are authoring narrative docs and
  likely don't want the API tree dumped into their book.
- Mixing the two requires editorial decisions about nav
  and cross-linking that a tool can't make safely.

## Why convert cargo doc HTML to markdown

The output target (mkdocs / docusaurus / wiki) consumes
markdown. Three options were considered:

| Option | Trade-off |
|---|---|
| **Convert HTML → markdown** (chosen) | Some loss of fidelity (rustdoc's complex layouts), but integrates cleanly with the wiki/website |
| Embed `cargo doc` HTML in an iframe | Awful UX; search and navigation don't compose |
| Parse `.rs` source ourselves | Duplicates rustdoc's logic; misses extension traits and re-exports |

The converter handles:

- Item signatures (function, struct, trait, impl) — code
  blocks with syntax highlighting.
- `///` doc comments — markdown body.
- Cross-links (`[Type]`) — rewritten to relative paths.
- Examples — code blocks with `# Examples` heading.
- Deprecation markers — admonition blocks.

Things it can't preserve:

- The collapsible struct field tables (rustdoc's HTML is
  fancy).
- The auto-generated trait implementations list.
- The "Source" link that jumps to the `.rs` file (no
  direct markdown equivalent).

Users who need full fidelity should publish `cargo doc`
output directly (e.g. to a separate subdomain) instead of
embedding it in the wiki.

## docs.rs interplay

For public crates, [docs.rs](https://docs.rs) hosts the
canonical rustdoc output. This skill does **not** publish
there — docs.rs builds automatically when you publish to
crates.io.

The skill is for **internal docs** where:

- The crate isn't on crates.io.
- The team wants API docs alongside design docs in one
  wiki/website.
- The team wants the API docs in the same SCM/repo as
  the prose docs.

## Workspace handling

`cargo doc --workspace --no-deps` builds docs for every
member crate. The skill writes them into per-crate
subdirectories:

```
docs/api/rust/
├── index.md             # workspace overview
├── crate-a/
│   ├── index.md
│   ├── struct-User.md
│   └── ...
├── crate-b/
└── ...
```

`--crate <name>` limits to one member. Multi-root
workspaces (separate `Cargo.toml` trees) are not
supported in v1 — the skill refuses with an error.

## See also

- [cargo doc](https://doc.rust-lang.org/cargo/commands/cargo-doc.html)
- [rustdoc book](https://doc.rust-lang.org/rustdoc/)
- [mdbook book](https://rust-lang.github.io/mdBook/)
- DESIGN-0014 §"Skill: rust-docz" — scope and trade-offs
