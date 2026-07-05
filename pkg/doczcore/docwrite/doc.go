// Package docwrite is the write side of the docz core, with a
// deliberately narrow surface: creating documents from templates
// (Create), rewriting the frontmatter status value in place
// (SetStatus), and checking off a checkbox task item (CheckTask). It is
// not a general markdown or frontmatter editor — anything beyond these
// three operations is out of scope by design (ADR-0001, amending
// DESIGN-0007's read-only-public stance).
//
// The two mutators share a byte-preservation contract: they read the
// whole file, splice only the bytes that must change (the status value;
// the checkbox state byte), and write the result back, so the on-disk
// diff is a single line. Both are LF-only and reject CR/CRLF input via
// ErrUnsupportedLineEndings.
//
// The package depends on its read-side siblings — document for
// frontmatter parsing and docparse for the task-item shape — and keeps
// the embedded template machinery private behind Create.
package docwrite
