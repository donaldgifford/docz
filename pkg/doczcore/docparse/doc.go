// Package docparse extracts structural facts from markdown documents:
// ATX headings with GitHub-compatible anchor slugs (Headings) and
// checkbox task-list items (TaskItems), each carrying the 1-based line
// number where it appears in the input.
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
