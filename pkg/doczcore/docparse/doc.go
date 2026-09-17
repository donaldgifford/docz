// Package docparse extracts structural facts from markdown documents:
// ATX headings with GitHub-compatible anchor slugs (Headings), checkbox
// task-list items (TaskItems) — each carrying the 1-based line number
// where it appears in the input — and the document's H1 title (Title).
//
// Headings and Title divide the ATX heading levels between them:
// Headings reports H2 through H6, Title reports the first H1, and no
// line is ever reported by both. The two answer different questions — a
// table of contents wants section structure without the document's own
// name, and a nav entry for a file with no frontmatter wants only the
// name.
//
// The division is not a partition of all markdown headings. Title also
// recognizes a setext H1 ("Title" underlined with "="), which Headings
// does not; a setext H2 (underlined with "-") is reported by neither.
// Title is the only fact here without a Line, since a title names the
// document rather than a place in it.
//
// The package reports facts, not interpretation (ADR-0001 Decision 5):
// it attaches no meaning to headings or tasks — no plan or phase
// grouping, no section scoping, no workflow model. Consumers that need
// such a model (for example, an agent harness grouping task items under
// phase headings) build it on these primitives.
//
// All functions are bytes-in/values-out and never return an error:
// arbitrary markdown is never "invalid" for a fact extractor — the
// worst case is an empty result. Line numbers use LF ("\n") accounting,
// matching the docz write contract; write helpers that consume these
// line numbers (the sibling docwrite package) enforce LF-only input
// themselves.
package docparse
