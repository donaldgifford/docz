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

### Bug Fixes

- *(ci)* Unbreak PR checks — bash shell, changelog, trufflehog pin

### Documentation

- *(impl)* Add inherited-workflow pruning and dependabot removal to phase 0
- *(readme)* Rewrite quickstart for the docz-site stack
- *(impl)* Clarify rfc-site sweep criterion to exclude self-references

### Testing

- *(scaffold)* Vitest + Testing Library + MSW jsdom suite with smoke test

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

