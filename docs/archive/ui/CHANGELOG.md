# Changelog

All notable changes to this project are documented here. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
this project adheres to [Semantic Versioning](https://semver.org/).
## [unreleased]

### Features

- *(scaffold)* Add Vite + React 19 + strict TS project scaffold
- *(theme)* Port mockup :root tokens into Tailwind v4 @theme
- *(theme)* Self-host IBM Plex + Source Serif 4 via @fontsource
- *(shell)* App shell topbar + lazy createBrowserRouter route table
- *(api)* TanStack Query provider and typed fetch mutator
- *(api)* Vendor docz-api openapi.yaml at info.version 1.0.0
- *(api)* Orval config generating react-query client from vendored spec
- *(ci)* Add gen-api-check.sh drift gate for the generated client
- *(dev)* MSW-backed dev mode for just dev-msw
- *(markdown)* Preprocess raw_md — strip frontmatter and docz toc block
- *(markdown)* Sanitize schema — GitHub default with pinned code classes
- *(markdown)* Unified sanitizing pipeline with ToC collection and Shiki
- *(colors)* Type/status color system — curated map, FNV-1a fallback
- *(components)* TypeBadge, StatusBadge, StatusPill per mockup styles
- *(reader)* Doc route with four fetch states and pipeline rendering
- *(reader)* Center column header and doc-prose styles
- *(reader)* Right rail — ToC, trimmed metadata, formats list
- *(reader)* Position-only lifecycle rail from listTypes statuses
- *(mocks)* Curated demo-org fixtures layered over faker handlers
- *(directory)* Typed URL search-param helper with round-trip tests
- *(directory)* URL-bound search directory with debounced query
- *(directory)* Repo picker, type chips, result count, clear filters
- *(directory)* Load-more pagination windowed by URL offset
- *(directory)* Contextual empty states completing the four-state matrix
- *(search)* Inert snippet renderer honoring only <em> match markers
- *(palette)* Cmdk command palette with grouped results and preview
- *(shell)* Topbar search affordance opens the command palette
- *(api)* Re-vendor docz-api spec 1.1.0 with getRepoIndex
- *(repos)* Facts hook and /repos grid backed by search facets
- *(repos)* Shared TechDocs-style repo nav
- *(repos)* Repo home rendering index.md with generated fallback
- *(repos)* Synthesized README-style type pages in a shared frame
- *(reader)* Mount the reader inside the three-column portal
- *(responsive)* Full-screen palette and repo-nav drawer on narrow viewports
- *(xrefs)* Link sibling doc ids in rendered markdown bodies
- *(a11y)* Axe sweep, badge contrast gate, and keyboard fixes
- *(perf)* Bundle-size budget in CI and hover prefetch for doc links
- *(e2e)* Playwright journeys and full-rule axe against an MSW preview build
- *(deploy)* Multi-stage Dockerfile, Bun static server, same-origin compose stack

### Bug Fixes

- *(ci)* Unbreak PR checks — bash shell, changelog, trufflehog pin
- *(ci)* Format phase-1 files and redact fixture DSN example
- *(api)* Tolerate JSON null where docz-api marshals empty Go slices

### Documentation

- *(impl)* Add inherited-workflow pruning and dependabot removal to phase 0
- *(readme)* Rewrite quickstart for the docz-site stack
- *(impl)* Clarify rfc-site sweep criterion to exclude self-references
- *(impl)* Record Phase 1 verification note
- *(claude)* Record fixtures layout and phase-1 CI lessons
- *(readme)* Add test, build, and deploy sections; close Phase 4 sweep
- *(impl)* Record Phase 4 success-criteria verification

### Testing

- *(scaffold)* Vitest + Testing Library + MSW jsdom suite with smoke test
- *(markdown)* XSS gate — 22 hostile payloads, benign suite, slug stability
- *(reader)* Pin pipeline memoization per (doc_id, content_hash)
- *(reader)* Four-state matrix, metadata omission, lifecycle, ToC anchors
- *(phase-2)* Close out component-test coverage and verify criteria
- *(phase-3)* Four-state coverage for repo pages and criteria verification

### Miscellaneous Tasks

- Initial import of design docs, impl plan, mockup, and scaffold [skip ci]
- *(sweep)* Remove rfc-site template scaffold
- *(sweep)* Remove superseded docz-site-mockup3.html
- *(sweep)* Prune Go-specific workflows, retarget CodeQL to TypeScript
- *(sweep)* Remove dependabot in favor of renovate
- *(scaffold)* Rewrite package.json as docz-site with core deps
- *(spec)* Add informational OpenAPI spec-drift workflow
- *(lint)* ESLint flat config + Prettier normalization
- *(tooling)* Rewrite mise.toml and justfile for the docz-site stack
- Add ci.yml running the just ci chain via mise

