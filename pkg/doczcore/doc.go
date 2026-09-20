// Package doczcore is a documentation-only package naming the type-agnostic
// core of the docz API. It has no code; its subpackages hold it all.
//
// The core is five packages, in dependency order (DESIGN-0014 §1):
//
//   - config    — .docz.yaml, the type registry, and the path rules
//   - document  — frontmatter, document scanning, and the changelog
//   - docparse  — markdown facts: headings, task items, list items, tables,
//     region markers, and the H1 title
//   - docwrite  — the narrow write side: status, checkbox, create
//   - toc       — the table-of-contents splice
//   - kinds     — readers for the region kinds more than one type shares
//   - validate  — the generic document checks
//
// Rule R2 is what "type-agnostic" means: a type package such as pkg/impl or
// pkg/rfc imports doczcore, and doczcore never imports a type package (ADR-0002
// R2). A grammar is therefore a contract over region kinds rather than over
// document types (R7), and a repo's custom type gets the same readers a
// built-in does.
//
// The tests beside this file enforce both halves of that mechanically, because
// DESIGN-0014 §6 chose tests over a depguard rule: a core package that grows an
// edge into a type package fails TestLayerRules, and one that grows a
// dependency on a telemetry or logging module fails its sibling. The library is
// traceable, not tracing — tracing and logging policy belong to the consumer
// (ADR-0002 Decision 5).
package doczcore
