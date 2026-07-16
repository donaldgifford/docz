# Changelog

All notable changes to this project are documented here. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
this project adheres to [Semantic Versioning](https://semver.org/).
## [unreleased]

### Bug Fixes

- *(chart)* Align appVersion with the bare-semver image tag

## [0.1.0] - 2026-07-16

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
- *(auth)* /login provider selection page
- *(auth)* 401 redirects to /login with destination stash and restore
- *(auth)* Session-backed avatar menu with logout
- *(auth)* Remember last-used provider in localStorage
- *(repo-nav)* Collapsible per-type doc drawers
- *(session-menu)* Hide the topbar Sign in link on /login
- *(palette)* Prefetch the highlighted hit's doc
- *(markdown)* Codeblock chrome — language badge and fence-meta caption
- *(markdown)* GitHub alert callouts as styled admonitions
- *(markdown)* Render mermaid fences as lazy, token-themed diagrams
- *(reader)* Metadata table header, ToC-first rail, lifecycle drawer
- *(palette)* Lead the empty query with recently-opened docs
- *(reader)* Copy-link affordance on section headings
- *(chart)* Add docz-site Helm chart

### Bug Fixes

- *(ci)* Unbreak PR checks — bash shell, changelog, trufflehog pin
- *(ci)* Format phase-1 files and redact fixture DSN example
- *(api)* Tolerate JSON null where docz-api marshals empty Go slices
- *(a11y)* Make scrollable code blocks keyboard-focusable named regions
- *(markdown)* Eliminate exponential backtracking in leading-h1 regex
- *(theme)* Style doc-prose h1 and stabilize the viewport gutter
- *(bundle-budget)* Measure the eager import closure, not just index-*.js

### Documentation

- *(impl)* Add inherited-workflow pruning and dependabot removal to phase 0
- *(readme)* Rewrite quickstart for the docz-site stack
- *(impl)* Clarify rfc-site sweep criterion to exclude self-references
- *(impl)* Record Phase 1 verification note
- *(claude)* Record fixtures layout and phase-1 CI lessons
- *(readme)* Add test, build, and deploy sections; close Phase 4 sweep
- *(impl)* Record Phase 4 success-criteria verification
- *(impl)* Verify Phase 5 acceptance criteria against the live stack
- *(impl)* Check off the completed testing-plan items
- *(claude)* Note nav drawer + doc-prose h1 guidance
- *(inv)* INV-0001 — reader UX polish, root causes and QoL backlog
- *(inv)* Add reader metadata-header item to the QoL backlog
- *(inv)* Record RFC rendering gaps — alerts, mermaid, code chrome
- *(inv)* Map rfc-site prior art onto the rendering backlog
- *(impl)* IMPL-0002 — phased plan for the INV-0001 reader-polish backlog
- *(impl)* Record IMPL-0002 open-question answers (all a)
- README refresh, changelog regen, INV-0001 concluded

### Testing

- *(scaffold)* Vitest + Testing Library + MSW jsdom suite with smoke test
- *(markdown)* XSS gate — 22 hostile payloads, benign suite, slug stability
- *(reader)* Pin pipeline memoization per (doc_id, content_hash)
- *(reader)* Four-state matrix, metadata omission, lifecycle, ToC anchors
- *(phase-2)* Close out component-test coverage and verify criteria
- *(phase-3)* Four-state coverage for repo pages and criteria verification
- *(auth)* E2e login loop with a mocked provider callback
- *(palette)* Prove the prefetched hit opens without a second getDoc

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
- Untrack session-local Claude loop state
- *(deploy)* Local compose + just targets for the site container
- *(chart)* Lint/test the chart in CI and publish to GHCR

