## References

- [DESIGN-0003: Table of Contents Generation](../design/0003-table-of-contents-generation.md)
  — Decisions 1 (scoping), 4 (no standalone `docz toc`), Marker Format section
- [IMPL-0003: Table of Contents Generation](../impl/0003-table-of-contents-generation.md)
  — Decision 2 (concise dry-run summary)
- [DESIGN-0012](../design/0012-updated-frontmatter-field-opt-in-stamp-pass-in-docz-update.md)
  — Decision 7 (silent normal-run output for the `updated:` stamp)
- [Issue #82](https://github.com/donaldgifford/docz/issues/82) — underscores
  stripped from ToC anchors
- Follow-up issues filed from this investigation:
  [docz #94](https://github.com/donaldgifford/docz/issues/94) (help text,
  config comment, README attribution, DESIGN-0003),
  [docz #95](https://github.com/donaldgifford/docz/issues/95) (normal-run
  ToC output, near-miss marker warning),
  [docz #96](https://github.com/donaldgifford/docz/issues/96) (non-ASCII and
  HTML-comment slug divergences, with #82),
  [docz #97](https://github.com/donaldgifford/docz/issues/97)
  (`docz update --check`),
  [docz #98](https://github.com/donaldgifford/docz/issues/98) (markdownlint
  in CI),
  [docz #99](https://github.com/donaldgifford/docz/issues/99) (doubled index
  markers),
  [claude-skills #98](https://github.com/donaldgifford/claude-skills/issues/98)
  (docz plugin skills)
- `cmd/update.go` (`updateCmd.Long`, `runToCUpdate`), `pkg/doczcore/toc/toc.go`
  (`BeginMarker`/`EndMarker`, `parseHeadings`, `UpdateToC`),
  `pkg/doczcore/docparse` (`Headings`, `AnchorSlug`),
  `internal/index/index.go` (`createNewReadme`),
  `internal/template/templates/index_investigation.md`, `index_plan.md`
- `README.md` "Table of Contents" section (lines 528–571 at `9c2f11f`)
- claude-skills docz plugin 1.3.0: `skills/update/SKILL.md`,
  `skills/docz/SKILL.md`, `skills/docz/references/`
- [markdownlint MD051/link-fragments](https://github.com/DavidAnson/markdownlint/blob/main/doc/md051.md)
- [Marksman](https://github.com/artempyanykh/marksman) — source of the
  `<!--toc:start-->` / `<!--toc:end-->` fence
- [markdown-toc.nvim](https://github.com/hedyhli/markdown-toc.nvim) — default
  fences `<!-- mtoc-start -->` / `<!-- mtoc-end -->`, always wrapped as
  `<!-- % -->`
