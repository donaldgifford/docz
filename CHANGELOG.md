# Changelog

All notable changes to this project are documented here. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
this project adheres to [Semantic Versioning](https://semver.org/).
## [unreleased]

### Documentation

- *(impl)* Record the v2.0.0-beta.4 release checks for IMPL-0020
- IMPL-0020 Completed, DESIGN-0017 Implemented
- *(inv)* INV-0014, consolidating the docz-api and docz-site charts
- *(inv)* Conclude INV-0014, one merged chart for new installs
- *(design)* DESIGN-0018, one Helm chart for docz

## [2.0.0-beta.4] - 2026-09-24

### Features

- *(ui)* Generate the client from the one spec at api/openapi.yaml
- *(chart)* Docz-site chart 0.2.0 at appVersion 2.0.0-beta.4

### Bug Fixes

- *(ui)* Clear trivy's two HIGH advisories in bun.lock
- Stop quoting placeholder DSNs and a docz-api URL in new files

### Documentation

- IMPL-0019 Phase 5 integration and release checks pass; tag deferred to a human
- IMPL-0019 Phase 5 beta.3 released and verified
- Complete IMPL-0019 and implement DESIGN-0016
- *(design)* DESIGN-0017 move docz-site in as ui/
- *(design)* Resolve DESIGN-0017 open questions, all (a)
- *(design)* DESIGN-0017 §7 — Go jobs are unfiltered, drop the spec output
- *(impl)* IMPL-0020 docz-site move-in, v2.0.0-beta.4
- Resolve IMPL-0020 open questions (all a); approve DESIGN-0017
- *(impl)* IMPL-0020 lint-actions is clean after the publish changes
- *(impl)* Record the IMPL-0020 Phase 0 publish dry run
- *(impl)* Record IMPL-0020 Phase 1 — docz-site sweep merged at f1203c9
- *(impl)* Record the IMPL-0020 Phase 2 graft commands
- *(ui)* Retarget links to docz-api's old repository at docz
- *(impl)* IMPL-0020 Phase 2 local gates pass
- *(impl)* IMPL-0020 Phase 2 PR opened ([#130](https://github.com/donaldgifford/docz/issues/130))
- *(impl)* Record the IMPL-0020 Phase 2 CI fixes
- *(impl)* Record the IMPL-0020 Phase 3 spec drift drill
- *(impl)* Record the IMPL-0020 Phase 3 ui image probe
- *(ui)* Point docz-site's repository URLs at the monorepo
- *(archive)* State the docz-site namespace rule and index both trees
- *(impl)* The archive excludes already cover docs/archive/ui
- *(ui)* Bump the docz-site chart once per release, before the tag
- *(claude)* A Frontend section for ui/
- A frontend section in README, DEVELOPMENT, and CONTRIBUTING
- *(inv)* Conclude INV-0012, the client generates from the one spec
- *(inv)* INV-0012's answer opens with a verdict validate can read
- *(adr)* Correct ADR-0004's two claims DESIGN-0017 disproved
- *(impl)* Record the IMPL-0020 follow-up issues and local release checks

### Testing

- *(ui)* Snapshot the archived fixtures into src/mocks/content
- *(archive)* Pin the docz-site URL sweep and the ui/ module fence

### Miscellaneous Tasks

- *(docker)* Keep ui/ out of the docz-api build context
- *(docker)* Split bake targets per component (-api)
- *(just)* Point the api bake recipes at the -api targets
- *(publish)* Parameterise ghcr.yml and ecr.yml by component
- *(release)* Pass component: api to both publish calls
- Add the ui path-filter output
- *(mise)* Pin bun 1.3.14 and node 24.14.0 for ui/
- *(renovate)* Extend the node preset for ui/
- *(ui)* Fence ui/ off from the root Go module
- *(ui)* Fold docz-site's repository-level files into the root
- Fold docz-site's .github into the root and delete ui/.github
- Add the ui and ui-e2e jobs; helm-unittest walks every chart
- *(just)* Run ui.just from ui/ and point it at the moved chart and stack
- *(just)* Root ci gate runs the ui chain
- *(ui)* Install prettier for the format check
- *(trufflehog)* Exclude the exclude file's own history
- *(codeql)* Ignore two non-production ui files, with reasons
- *(ui)* Feed the spec to Dockerfile.ui as a named build context
- *(docker)* Add the -ui bake family for docz-site
- *(docker)* Bake both images when docker or ui changed
- *(just)* Add ui docker-build, the no-bake spelling of dev-ui
- *(deploy)* Build both images from this repository in deploy/ui
- *(deploy)* Build the local site from ui/ against the api local stack
- *(release)* Publish docz-site's image and chart from the same tag

## [2.0.0-beta.3] - 2026-09-23

### Features

- *(config)* ParseBytes decodes .docz.yaml bytes with Load's normalisation (IMPL-0019 Phase 1)

### Bug Fixes

- *(kinds,validate)* Read the option spellings the corpus actually uses

### Refactor

- *(api)* Import docz-api's packages under github.com/donaldgifford/docz/v2
- *(api)* Import docz's pkg/ under the /v2 module path
- *(ingest)* Parse .docz.yaml with doczcfg.ParseBytes
- *(search)* Name the index attributes the attribute lists share

### Documentation

- IMPL-0018 Completed — v2.0.0-beta.1 is released
- INV-0011 — consolidating docz-api and docz-site into one repo
- *(inv-0011)* Resolve seven open questions — one module, pkg/ stays
- *(adr-0004)* One repository, one module — record the consolidation
- *(adr-0004)* Resolve OQ 1 — archive all 43 incoming docs, renumber none
- *(adr-0004)* Successors for unfinished work, and a status sweep first
- *(adr-0004)* Resolve OQ 2/3/4, delete doczcontract, flip to Accepted
- Correct the no-shim claim for make build and make test
- Correct the log-% replacement claim in CLAUDE.md
- DESIGN-0016 — move docz-api in (internal/, cmd/docz-api, api/, charts/)
- Resolve DESIGN-0016's six open questions, status Approved
- IMPL-0019 — docz-api move-in, v2.0.0-beta.3
- Resolve IMPL-0019's six open questions
- Describe api and ui as just modules (IMPL-0019 Phase 0)
- IMPL-0019 Phase 0 govulncheck task checked off
- IMPL-0019 In Progress; Phase 0 complete
- Document config.ParseBytes in CLAUDE.md (IMPL-0019 Phase 1)
- IMPL-0019 Phase 2 docz-api sweep tasks deferred to a human
- IMPL-0019 Phase 2 status sweep merged, forward pointers in docz-api #44
- Record IMPL-0019 Phase 2 clone and filter-repo invocation
- Check off IMPL-0019 Phase 2 graft merge tasks
- Check off IMPL-0019 Phase 2 Dockerfile references
- Check off IMPL-0019 Phase 2 cliff.toml task
- Check off IMPL-0019 Phase 2 PR task
- Check off IMPL-0019 Phase 3 rewrite and tidy tasks
- Check off IMPL-0019 Phase 3 api.just task
- Check off IMPL-0019 Phase 3 docker.just task
- Check off IMPL-0019 Phase 3 ci gate task
- Check off IMPL-0019 Phase 3 ParseBytes swap
- Check off IMPL-0019 Phase 3 doczcontract task
- Record IMPL-0019 Phase 3 lint baseline (8 findings)
- Check off IMPL-0019 Phase 3 reflow task
- Check off IMPL-0019 Phase 3 lint findings task
- Clone docz, not docz-api, in the server's first-time setup
- Check off IMPL-0019 Phase 3 archive-spared test
- Record IMPL-0019 Phase 3 consumer go.sum count
- CLAUDE.md for IMPL-0019 Phase 3 (api module, archive test, ParseBytes in ingest)
- Check off IMPL-0019 Phase 3 PR task
- Name docz-api's old module path in CLAUDE.md without spelling it
- Check off IMPL-0019 Phase 4 layer rule and GHCR access
- *(archive)* Say what an ID means inside docs/archive/api/
- Check off IMPL-0019 Phase 4 archive README
- *(config)* Keep docs/archive/ out of the wiki and the api listing
- Check off IMPL-0019 Phase 4 exclude tasks
- Check off IMPL-0019 Phase 4 wiki nav check (0 archive pages, 55 total)
- Check off IMPL-0019 Phase 4 publish-image task
- Check off IMPL-0019 Phase 4 chart task
- *(investigation)* INV-0012 and INV-0013, successors to docz-api INV-0002 and INV-0003
- Check off IMPL-0019 Phase 4 successor investigations
- CLAUDE.md for IMPL-0019 Phase 4 (internal/ is the server's, layer rule, archive, beta publishing)
- Check off IMPL-0019 Phase 4 CLAUDE.md task

### Styling

- *(api)* Gofumpt/golines reflow of internal/ and cmd/docz-api/

### Testing

- *(config)* Pin ParseBytes to Load and to reading nothing (IMPL-0019 Phase 1)
- *(consumer)* Prove config.ParseBytes reachable from outside the module (IMPL-0019 Phase 1)
- *(ingest)* Keep the fixture-manifest clause, delete internal/doczcontract
- Pin that the import rewrite spared docz-api's archive
- Exempt the archive test from its own old-module-path scan
- *(layer)* Pkg/ never imports the server's internal/
- Wiki.exclude and api.exclude agree on docs/archive/

### Miscellaneous Tasks

- Claude settings and gitignore
- Cover the incoming repos in .gitignore, converge claude settings
- *(claude)* Allow docz validate and gh issue
- Replace make with just (ADR-0004 Decision 4)
- Drop makefmt and checkmake with the Makefile
- Go 1.26.5 across both modules and mise (IMPL-0019 Phase 0)
- Delete .checkmake.ini (IMPL-0019 Phase 0)
- *(just)* Compose api and ui as optional modules (IMPL-0019 Phase 0)
- Skip the Go jobs on a PR labelled graft (IMPL-0019 Phase 0)
- Graft docz-api's history into docz (IMPL-0019 Phase 2)
- *(api)* Point Docker references at Dockerfile.api and deploy/api/
- *(changelog)* Skip docz-api's grafted history in git-cliff
- Skip the license check on the graft PR
- *(api)* Point docz-api's image, chart, and changelog metadata at the docz repo
- Go mod tidy the merged module
- *(api)* Load api.just as the root's api module
- *(api)* Fold docker.just into api.just
- Run the api module's lint, tests, and chart lint in just ci
- *(lint)* Exclude two gosec false positives in the server, with reasons
- *(consumer)* Go mod tidy after the merge
- *(prerelease)* Publish docz-api's image and chart on a beta tag
- *(chart)* Docz-api chart 0.9.0 for v2.0.0-beta.3
- *(api)* Stop tagging a beta image as latest

## [2.0.0-beta.1] - 2026-09-21

### Features

- *(go.mod)* [**breaking**] Move to the /v2 module path
- *(docparse)* Add the region marker walker
- *(docparse)* Add ListItems and Tables
- *(document)* Add the optional schema frontmatter field
- *(template)* Wrap every built-in template section in a region
- *(template)* Embed a marker skeleton per built-in type
- *(kinds)* Add the shared region readers and heading inference
- *(validate)* Add the generic document validator
- *(impl)* Typed IMPL documents with the phase and task grammar
- *(impl)* Type-tier validator for IMPL documents
- *(docwrite)* Byte cores for status, task state, and render
- *(rfc)* Typed RFC documents
- *(adr)* Typed ADR documents
- *(design)* Typed DESIGN documents
- *(investigation)* Typed INV documents
- *(doctemplate)* Schema resolution with an enforced name grammar
- *(wiki)* Add Init and UpdateNav orchestration (IMPL-0018 Phase 2)
- *(repo)* Add pkg/doczcore/repo with Repo, typed errors, hooks, and the reads
- *(repo)* Add Update and Create (IMPL-0018 Phase 3 tasks 4-5)
- *(repo)* Add SetStatus, Init, and the template pair (IMPL-0018 Phase 3 tasks 6-8)
- *(repo)* Add Validate and InsertRegions (IMPL-0018 Phase 3 tasks 9-11)
- *(config)* [**breaking**] ADR-0003 — remove plan from the built-in catalogue
- *(cmd)* Runner carries the repo API, wired in PersistentPreRunE
- *(cmd)* Signal context and the hook-to-slog mapping
- *(cmd)* [**breaking**] Every command orchestrates through the repo and wiki API
- *(cmd)* Docz validate composes the three tiers
- *(cmd)* Docz validate --fix marks the regions inference finds

### Bug Fixes

- *(kinds)* Number reader lines from the region, not its body
- *(validate)* Stop reporting toc.missing on an unmarked document with a real ToC
- *(cmd,repo)* Act on the Phase 5 review pass (IMPL-0018)
- *(parity)* Compute $DATE in UTC and freeze the fixture date it hid

### Refactor

- *(toc)* Locate the ToC region through docparse.Regions
- *(document)* Move ErrUnsupportedLineEndings to the facts layer
- *(kinds)* Own the region-to-document line conversion
- *(doctemplate)* Promote internal/template to pkg/doczcore/doctemplate
- *(index)* Promote internal/index and add Splice, Scaffold, and the markers
- *(wiki)* Promote internal/wiki to pkg/wiki, emptying internal/

### Documentation

- INV-0008, DESIGN-0012, IMPL-0017 — the updated frontmatter field ([#93](https://github.com/donaldgifford/docz/issues/93))
- INV-0009 — ToC regeneration in docz update and markdownlint MD051 ([#101](https://github.com/donaldgifford/docz/issues/101))
- INV-0010 and DESIGN-0013 — IMPL plan API and a library-first docz ([#102](https://github.com/donaldgifford/docz/issues/102))
- V2 API unit — ADR-0002/0003, DESIGN-0014/0015, IMPL-0018 ([#104](https://github.com/donaldgifford/docz/issues/104))
- Defer the claude-skills plugin update past v2, retarget #97 to docz validate ([#105](https://github.com/donaldgifford/docz/issues/105))
- Spell the /v2 module path in README, DEVELOPMENT, and CLAUDE.md
- *(claude)* Describe the parity suite in CLAUDE.md
- Record how pr-semver-bump v1.7.4 treats a beta tag
- Document the Phase 1 type layer in CLAUDE.md and DEVELOPMENT.md
- Record the Phase 2 promotions in CLAUDE.md and DEVELOPMENT.md
- *(impl-0018)* Record the five wiki behaviour deltas Phase 5 must reconcile
- *(repo)* Cover pkg/doczcore/repo in the consumer module and the guides
- Mark regions in docz's own documents (docz validate --fix)
- Fix the 56 validation errors the corpus migration exposed (IMPL-0018 Phase 5)
- The ADR-0001 amendment, living docs, and the EXPERIMENTAL markers (IMPL-0018 Phase 5)
- DESIGN-0014/0015 Implemented, IMPL-0017 retargeted to v2 (IMPL-0018 Phase 5)
- Correct the issue #97 deferral note (IMPL-0018 Phase 5)
- Tick the Testing Plan, mark the four deferred tasks (IMPL-0018 Phase 5)
- Record the review-pass behaviour in CLAUDE.md (IMPL-0018 Phase 5)
- Record the parity date fix and clear the design index drift

### Testing

- *(consumer)* Import the /v2 path from the external module
- *(parity)* Add the seven fixture repositories
- *(parity)* Add the golden driver behind the parity build tag
- *(parity)* Add the parity and parity-capture targets with v1.2.2 goldens
- *(parity)* Make the suite hermetic and reject irregular fixture files
- *(docparse)* Golden corpus and fuzz target for the region walker
- *(kinds)* Pin the readers against the real corpus
- *(validate)* Tables per code family and the derivation goldens
- *(doczcore)* Enforce the layer rules mechanically
- *(impl)* Golden corpus of ten real plans
- *(impl)* Pin what the rendered template parses to
- *(types)* Bind each heading table to its embedded template
- *(consumer)* Prove the v2 type layer from outside the module
- *(rfc,adr,design,investigation)* Golden corpora from the real fleet
- *(rfc,adr,design,investigation)* Parse each package's own rendered template
- *(consumer)* Cover the three promoted packages from outside the module

### Miscellaneous Tasks

- Point the version ldflags at the /v2 path
- Build pre-releases from hand-pushed beta tags
- Make parity and docz validate part of make ci (IMPL-0018 Phase 5)
- *(make)* Name test-consumer in the ci target's help line

## [1.2.2] - 2026-08-30

### Bug Fixes

- *(config)* Add json struct tags mirroring .docz.yaml key spellings ([#91](https://github.com/donaldgifford/docz/issues/91))

### Documentation

- Flip DESIGN-0011 to Implemented and IMPL-0016 to Completed ([#90](https://github.com/donaldgifford/docz/issues/90))

## [1.2.1] - 2026-08-29

### Miscellaneous Tasks

- *(docz)* Dogfood the api block — publish docs pages + contributor guides ([#88](https://github.com/donaldgifford/docz/issues/88))

## [1.2.0] - 2026-08-26

### Features

- V1.2.0 — api config block and docparse.Title (IMPL-0016) ([#84](https://github.com/donaldgifford/docz/issues/84))

## [1.1.1] - 2026-08-11

### Features

- *(template,config)* Prefix-qualified H1s, PLAN off by default, DESIGN-0011/INV-0007 ([#83](https://github.com/donaldgifford/docz/issues/83))

### Documentation

- Mark DESIGN-0010 Implemented and IMPL-0015 Completed ([#79](https://github.com/donaldgifford/docz/issues/79))

## [1.1.0] - 2026-08-03

### Features

- *(config,document)* V1.1.0 — changelog config block and ParseChangelog (DESIGN-0010/IMPL-0015) ([#78](https://github.com/donaldgifford/docz/issues/78))

### Documentation

- Flip IMPL-0014 to Completed post-v1.0.0 tag ([#72](https://github.com/donaldgifford/docz/issues/72))

## [1.0.0] - 2026-07-05

### Features

- *(config)* Replace viper with yaml.v3 in config.Load (IMPL-0014 Phase 1) ([#70](https://github.com/donaldgifford/docz/issues/70))
- [**breaking**] V1.0.0 — the five-package pkg/doczcore public core (IMPL-0014) ([#71](https://github.com/donaldgifford/docz/issues/71))

### Documentation

- Mark DESIGN-0007 Implemented, resolve OQ3/OQ5/OQ8 (post-v0.5.0) ([#67](https://github.com/donaldgifford/docz/issues/67))
- Mark IMPL-0013 Completed (post-v0.5.0) ([#68](https://github.com/donaldgifford/docz/issues/68))
- Accept ADR-0001 single public core, add INV-0006 + IMPL-0014 ([#69](https://github.com/donaldgifford/docz/issues/69))

## [0.5.0] - 2026-07-01

### Features

- Promote parsing core to public pkg/doczcore surface ([#66](https://github.com/donaldgifford/docz/issues/66))

## [0.4.1] - 2026-06-24

### Documentation

- Mark DESIGN-0006 Implemented and IMPL-0012 Completed (post-merge) ([#57](https://github.com/donaldgifford/docz/issues/57))
- INV-0005 + DESIGN-0007/0008/0009 for docz-api and docz-site ([#64](https://github.com/donaldgifford/docz/issues/64))

### Miscellaneous Tasks

- Rm dependabot.yml ([#58](https://github.com/donaldgifford/docz/issues/58))
- Schema store additions ([#63](https://github.com/donaldgifford/docz/issues/63))
- Upgrade Go to 1.26.4, bump CI actions and tooling ([#65](https://github.com/donaldgifford/docz/issues/65))

## [0.4.0] - 2026-06-23

### Features

- Custom document type support (DESIGN-0006 / IMPL-0012) ([#56](https://github.com/donaldgifford/docz/issues/56))

## [0.3.0] - 2026-06-15

### Features

- *(cmd,config)* IMPL-0009 Runner pattern + DocType registry implementation (phases 2-11) ([#49](https://github.com/donaldgifford/docz/issues/49))
- *(cmd,document)* Docz status set CLI primitive (DESIGN-0005, IMPL-0011) ([#53](https://github.com/donaldgifford/docz/issues/53))

### Refactor

- *(wiki)* IMPL-0008 Phase 1 move writeMkDocsYAML into internal/wiki
- *(toc)* IMPL-0008 Phase 2 move updateToCs into internal/toc
- *(wiki)* IMPL-0008 Phase 3 extract wiki.BuildNav helper
- *(document,index)* IMPL-0008 Phase 4 split internal/index
- *(document)* IMPL-0008 Phase 5 single LoadFrontmatter helper
- *(document)* IMPL-0008 Phase 6 single DoczFilePattern + IsDoczFile
- *(template,toc)* IMPL-0008 Phase 7 rename Slugify variants
- *(wiki)* IMPL-0008 Phase 8 DocTitle returns "", err on failure
- *(index)* IMPL-0008 Phase 9 typed UpdateOutcome
- IMPL-0008 Phase 10 rename TemplateData and ToCConfig

### Documentation

- *(inv)* INV-0004 mdp integration resolved -- pkg/ now public ([#42](https://github.com/donaldgifford/docz/issues/42))
- *(inv)* INV-0002 Wave 4 status update for IMPL-0008
- *(impl)* IMPL-0008 Phase 11 partial - check off non-merge tasks
- *(impl)* Mark IMPL-0008 Completed after PRs #44 and #45 merged
- *(design)* IMPL-0009 Phase 1 DESIGN-0004 Runner pattern + DocType registry ([#47](https://github.com/donaldgifford/docz/issues/47))

### Performance

- *(update)* IMPL-0007 eliminate redundant file reads and heading parses ([#43](https://github.com/donaldgifford/docz/issues/43))

### Miscellaneous Tasks

- Nudge workflow trigger

## [0.2.0] - 2026-05-25

### Features

- *(config)* IMPL-0006 correctness and duplication cleanup ([#41](https://github.com/donaldgifford/docz/issues/41))

### Other

- Catalog-info and codeowners

### Documentation

- *(inv)* Add INV-0003 init/update should respect config-listed types
- *(inv)* Add INV-0004 v1 release plan — TUI, mdp preview, CLI parity
- *(inv)* Record INV-0004 design-review decisions
- *(inv)* Reopen INV-0004 mdp integration question
- *(impl)* Mark IMPL-0005 as Completed
- *(inv)* Add IMPL-0006..0009 prerequisite gate to INV-0004

### Miscellaneous Tasks

- Add .github/CODEOWNERS
- Add .github/dependabot.yml
- Add catalog-info.yaml

## [0.1.0] - 2026-05-15

### Bug Fixes

- Fmt

### Refactor

- *(config)* Centralize file modes, filenames, and minHeadings constants
- Modernize Go stdlib idioms
- Cobra hygiene and Validate style polish

### Documentation

- *(inv)* Add INV-0002 architectural review and cleanup opportunities
- *(impl)* Add IMPL-0005 through IMPL-0009 for INV-0002 cleanup waves
- *(impl)* Check off IMPL-0005 Phase 4 PR task with PR #36 link

### Testing

- Add regression tests for time.DateOnly and bytes.NewReader swaps

### Miscellaneous Tasks

- *(claude)* Enable additional skills plugins and permissions allowlist

## [0.0.12] - 2026-04-02

### Bug Fixes

- *(update)* Skip disabled types in docz update

## [0.0.11] - 2026-04-02

### Features

- *(config)* Add markdown_extensions, docs_dir, repo_url, site_url, theme to WikiConfig
- *(wiki)* Write docs_dir, repo_url, site_url, theme, markdown_extensions to mkdocs.yml

### Bug Fixes

- *(release)* Add ldflags to goreleaser and fix binary name typo

### Documentation

- Add markdown_extensions and optional fields to wiki config docs

### Testing

- Add tests for markdown_extensions and optional mkdocs fields

## [0.0.10] - 2026-04-02

### Features

- *(config)* Add Plugins field to WikiConfig
- *(init)* Add wiki.plugins to default config
- *(wiki)* Write configured plugins to mkdocs.yml
- *(template)* Add embedded wiki_index.md template
- *(template)* Add EmbeddedWikiIndex() accessor
- *(template)* Add WikiIndexData, ResolveWikiIndex, RenderWikiIndex
- *(wiki)* Rewrite ensureDocsIndex to use wiki index template

### Bug Fixes

- *(init)* Skip disabled types in docz init

### Documentation

- *(inv)* Add INV-0001 wiki init template and init enabled fix
- *(inv)* Conclude INV-0001 with decisions
- *(impl)* Add IMPL-0004 wiki init template and init enabled fix
- *(impl)* Resolve IMPL-0004 open questions
- Add wiki plugins, index template, and init enabled docs to README
- Add wiki_index.md and ResolveWikiIndex to DEVELOPMENT.md
- *(impl)* Mark IMPL-0004 as completed

### Testing

- Add tests for init disabled types and wiki plugins
- Add wiki index template unit and integration tests

## [0.0.9] - 2026-03-22

### Features

- *(toc)* Add core ToC package with heading parsing and slug generation
- *(config)* Add ToCConfig struct with defaults
- *(cmd)* Add toc section to default config in docz init
- *(cmd)* Wire ToC generation into docz update
- *(template)* Add ToC markers to all six document templates

### Bug Fixes

- All dependabots
- *(toc)* Fix Slugify doc comment and check off testing plan

### Documentation

- *(design)* Add DESIGN-0003 table of contents generation
- Add ToC feature documentation to README
- Add internal/toc package to DEVELOPMENT.md
- Add internal/toc to CLAUDE.md architecture section
- *(impl)* Mark IMPL-0003 as completed
- Add note about adding ToC markers to pre-v0.0.8 documents

### Testing

- *(toc)* Add unit tests for ToC package
- *(toc)* Add golden file test for ToC output
- *(config)* Add ToCConfig default and round-trip tests
- *(template)* Regenerate golden files with ToC markers
- *(cmd)* Add integration tests for ToC in update and create

### Miscellaneous Tasks

- Disable dependabot

## [0.0.8] - 2026-03-14

### Bug Fixes

- *(wiki)* Use "Home" title for root index.md in nav

### Documentation

- Update README, DEVELOPMENT, design statuses, and add CLAUDE.md

### Miscellaneous Tasks

- Initialize docz and wiki for the project

## [0.0.7] - 2026-03-14

### Features

- *(config)* Add WikiConfig struct with defaults and wire into config system
- *(wiki)* Add titles package for nav title extraction
- *(wiki)* Add NavEntry, ScanDocs, and SortEntries for nav tree building
- *(wiki)* Add MkDocs YAML read/write and nav serialization
- *(cmd)* Add wiki init and wiki update commands
- *(cmd)* Wire wiki nav auto-update into docz create
- *(cmd)* Add verbose output and edge case handling to wiki commands

### Documentation

- *(impl)* Add implementation plan for wiki command (DESIGN-0002)
- Add wiki command documentation to README and DEVELOPMENT
- *(impl)* Mark IMPL-0002 as completed

### Testing

- *(config)* Add wiki config default and round-trip tests
- *(wiki)* Add unit tests for title extraction functions
- *(wiki)* Add unit tests for nav tree scanning and sorting
- *(wiki)* Add unit tests for MkDocs YAML I/O and nav merging
- *(cmd)* Add integration tests for wiki init, update, and create
- *(wiki)* Add golden file test for nav output

## [0.0.6] - 2026-03-11

### Documentation

- *(design)* Add wiki command design doc for MkDocs TechDocs integration
- *(design)* Resolve open questions in wiki command design

## [0.0.5] - 2026-03-08

### Features

- *(cmd)* Add type aliases and fix help text for all commands

## [0.0.4] - 2026-03-08

### Bug Fixes

- *(config)* Wire plan type into config and fix docs

## [0.0.3] - 2026-03-07

### Features

- *(docs)* Add investigation document type

## [0.0.2] - 2026-03-07

### Features

- *(docs)* Add plan template, update impl template, and add project docs

### Bug Fixes

- *(templates)* Suppress MD025/MD041 lint errors and add frontmatter to design doc

## [0.0.1] - 2026-02-24

### Features

- *(template)* Add embedded default templates for all document types
- Implement core internal packages for template, config, and document
- *(cmd)* Implement init, create, and version commands
- *(index)* Implement index generation and update command
- *(cmd)* Add list command and fix all lint issues
- *(cmd)* Add template and config commands with tests
- Add Phase 5 polish - verbose flag, config validation, Makefile updates

### Bug Fixes

- *(deps)* Move cobra and viper to direct dependencies
- *(cmd)* Extract json format string to constant

### Refactor

- Move main.go to cmd/docz/main.go

### Documentation

- Add design document and example scripts
- Mark implementation plan as completed

### Testing

- Add unit tests for template, config, and document packages
- Add integration and golden file tests for create and templates

