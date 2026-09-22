## [unreleased]

### 🐛 Bug Fixes

- *(kinds,validate)* Read the option spellings the corpus actually uses

### 📚 Documentation

- IMPL-0018 Completed — v2.0.0-beta.1 is released
- INV-0011 — consolidating docz-api and docz-site into one repo
- *(inv-0011)* Resolve seven open questions — one module, pkg/ stays
- *(adr-0004)* One repository, one module — record the consolidation
- *(adr-0004)* Resolve OQ 1 — archive all 43 incoming docs, renumber none
- *(adr-0004)* Successors for unfinished work, and a status sweep first
- *(adr-0004)* Resolve OQ 2/3/4, delete doczcontract, flip to Accepted

### ⚙️ Miscellaneous Tasks

- Claude settings and gitignore
## [2.0.0-beta.1] - 2026-09-21

### 🚀 Features

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

### 🐛 Bug Fixes

- *(kinds)* Number reader lines from the region, not its body
- *(validate)* Stop reporting toc.missing on an unmarked document with a real ToC
- *(cmd,repo)* Act on the Phase 5 review pass (IMPL-0018)
- *(parity)* Compute $DATE in UTC and freeze the fixture date it hid

### 📚 Documentation

- INV-0008, DESIGN-0012, IMPL-0017 — the updated frontmatter field (#93)
- INV-0009 — ToC regeneration in docz update and markdownlint MD051 (#101)
- INV-0010 and DESIGN-0013 — IMPL plan API and a library-first docz (#102)
- V2 API unit — ADR-0002/0003, DESIGN-0014/0015, IMPL-0018 (#104)
- Defer the claude-skills plugin update past v2, retarget #97 to docz validate (#105)
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

### 🚜 Refactor

- *(toc)* Locate the ToC region through docparse.Regions
- *(document)* Move ErrUnsupportedLineEndings to the facts layer
- *(kinds)* Own the region-to-document line conversion
- *(doctemplate)* Promote internal/template to pkg/doczcore/doctemplate
- *(index)* Promote internal/index and add Splice, Scaffold, and the markers
- *(wiki)* Promote internal/wiki to pkg/wiki, emptying internal/

### 🧪 Testing

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

### ⚙️ Miscellaneous Tasks

- Build pre-releases from hand-pushed beta tags
- Make parity and docz validate part of make ci (IMPL-0018 Phase 5)
- *(make)* Name test-consumer in the ci target's help line

### 💼 Other

- Point the version ldflags at the /v2 path
## [1.2.2] - 2026-08-30

### 🐛 Bug Fixes

- *(config)* Add json struct tags mirroring .docz.yaml key spellings (#91)

### 📚 Documentation

- Flip DESIGN-0011 to Implemented and IMPL-0016 to Completed (#90)
## [1.2.1] - 2026-08-29

### ⚙️ Miscellaneous Tasks

- *(docz)* Dogfood the api block — publish docs pages + contributor guides (#88)
## [1.2.0] - 2026-08-26

### 🚀 Features

- V1.2.0 — api config block and docparse.Title (IMPL-0016) (#84)
## [1.1.1] - 2026-08-11

### 🚀 Features

- *(template,config)* Prefix-qualified H1s, PLAN off by default, DESIGN-0011/INV-0007 (#83)

### 📚 Documentation

- Mark DESIGN-0010 Implemented and IMPL-0015 Completed (#79)
## [1.1.0] - 2026-08-03

### 🚀 Features

- *(config,document)* V1.1.0 — changelog config block and ParseChangelog (DESIGN-0010/IMPL-0015) (#78)

### 📚 Documentation

- Flip IMPL-0014 to Completed post-v1.0.0 tag (#72)
## [1.0.0] - 2026-07-05

### 🚀 Features

- *(config)* Replace viper with yaml.v3 in config.Load (IMPL-0014 Phase 1) (#70)
- [**breaking**] V1.0.0 — the five-package pkg/doczcore public core (IMPL-0014) (#71)

### 📚 Documentation

- Mark DESIGN-0007 Implemented, resolve OQ3/OQ5/OQ8 (post-v0.5.0) (#67)
- Mark IMPL-0013 Completed (post-v0.5.0) (#68)
- Accept ADR-0001 single public core, add INV-0006 + IMPL-0014 (#69)
## [0.5.0] - 2026-07-01

### 🚀 Features

- Promote parsing core to public pkg/doczcore surface (#66)
## [0.4.1] - 2026-06-24

### 📚 Documentation

- Mark DESIGN-0006 Implemented and IMPL-0012 Completed (post-merge) (#57)
- INV-0005 + DESIGN-0007/0008/0009 for docz-api and docz-site (#64)

### ⚙️ Miscellaneous Tasks

- Rm dependabot.yml (#58)
- Schema store additions (#63)
- Upgrade Go to 1.26.4, bump CI actions and tooling (#65)
## [0.4.0] - 2026-06-23

### 🚀 Features

- Custom document type support (DESIGN-0006 / IMPL-0012) (#56)
## [0.3.0] - 2026-06-15

### 🚀 Features

- *(cmd,config)* IMPL-0009 Runner pattern + DocType registry implementation (phases 2-11) (#49)
- *(cmd,document)* Docz status set CLI primitive (DESIGN-0005, IMPL-0011) (#53)

### 📚 Documentation

- *(inv)* INV-0004 mdp integration resolved -- pkg/ now public (#42)
- *(inv)* INV-0002 Wave 4 status update for IMPL-0008
- *(impl)* IMPL-0008 Phase 11 partial - check off non-merge tasks
- *(impl)* Mark IMPL-0008 Completed after PRs #44 and #45 merged
- *(design)* IMPL-0009 Phase 1 DESIGN-0004 Runner pattern + DocType registry (#47)

### ⚡ Performance

- *(update)* IMPL-0007 eliminate redundant file reads and heading parses (#43)

### 🚜 Refactor

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

### ⚙️ Miscellaneous Tasks

- Nudge workflow trigger
## [0.2.0] - 2026-05-25

### 🚀 Features

- *(config)* IMPL-0006 correctness and duplication cleanup (#41)

### 📚 Documentation

- *(inv)* Add INV-0003 init/update should respect config-listed types
- *(inv)* Add INV-0004 v1 release plan — TUI, mdp preview, CLI parity
- *(inv)* Record INV-0004 design-review decisions
- *(inv)* Reopen INV-0004 mdp integration question
- *(impl)* Mark IMPL-0005 as Completed
- *(inv)* Add IMPL-0006..0009 prerequisite gate to INV-0004

### ⚙️ Miscellaneous Tasks

- Add .github/CODEOWNERS
- Add .github/dependabot.yml
- Add catalog-info.yaml

### 💼 Other

- Catalog-info and codeowners
## [0.1.0] - 2026-05-15

### 🐛 Bug Fixes

- Fmt

### 📚 Documentation

- *(inv)* Add INV-0002 architectural review and cleanup opportunities
- *(impl)* Add IMPL-0005 through IMPL-0009 for INV-0002 cleanup waves
- *(impl)* Check off IMPL-0005 Phase 4 PR task with PR #36 link

### 🚜 Refactor

- *(config)* Centralize file modes, filenames, and minHeadings constants
- Modernize Go stdlib idioms
- Cobra hygiene and Validate style polish

### 🧪 Testing

- Add regression tests for time.DateOnly and bytes.NewReader swaps

### ⚙️ Miscellaneous Tasks

- *(claude)* Enable additional skills plugins and permissions allowlist
## [0.0.12] - 2026-04-02

### 🐛 Bug Fixes

- *(update)* Skip disabled types in docz update
## [0.0.11] - 2026-04-02

### 🚀 Features

- *(config)* Add markdown_extensions, docs_dir, repo_url, site_url, theme to WikiConfig
- *(wiki)* Write docs_dir, repo_url, site_url, theme, markdown_extensions to mkdocs.yml

### 🐛 Bug Fixes

- *(release)* Add ldflags to goreleaser and fix binary name typo

### 📚 Documentation

- Add markdown_extensions and optional fields to wiki config docs

### 🧪 Testing

- Add tests for markdown_extensions and optional mkdocs fields
## [0.0.10] - 2026-04-02

### 🚀 Features

- *(config)* Add Plugins field to WikiConfig
- *(init)* Add wiki.plugins to default config
- *(wiki)* Write configured plugins to mkdocs.yml
- *(template)* Add embedded wiki_index.md template
- *(template)* Add EmbeddedWikiIndex() accessor
- *(template)* Add WikiIndexData, ResolveWikiIndex, RenderWikiIndex
- *(wiki)* Rewrite ensureDocsIndex to use wiki index template

### 🐛 Bug Fixes

- *(init)* Skip disabled types in docz init

### 📚 Documentation

- *(inv)* Add INV-0001 wiki init template and init enabled fix
- *(inv)* Conclude INV-0001 with decisions
- *(impl)* Add IMPL-0004 wiki init template and init enabled fix
- *(impl)* Resolve IMPL-0004 open questions
- Add wiki plugins, index template, and init enabled docs to README
- Add wiki_index.md and ResolveWikiIndex to DEVELOPMENT.md
- *(impl)* Mark IMPL-0004 as completed

### 🧪 Testing

- Add tests for init disabled types and wiki plugins
- Add wiki index template unit and integration tests
## [0.0.9] - 2026-03-22

### 🚀 Features

- *(toc)* Add core ToC package with heading parsing and slug generation
- *(config)* Add ToCConfig struct with defaults
- *(cmd)* Add toc section to default config in docz init
- *(cmd)* Wire ToC generation into docz update
- *(template)* Add ToC markers to all six document templates

### 🐛 Bug Fixes

- All dependabots
- *(toc)* Fix Slugify doc comment and check off testing plan

### 📚 Documentation

- *(design)* Add DESIGN-0003 table of contents generation
- Add ToC feature documentation to README
- Add internal/toc package to DEVELOPMENT.md
- Add internal/toc to CLAUDE.md architecture section
- *(impl)* Mark IMPL-0003 as completed
- Add note about adding ToC markers to pre-v0.0.8 documents

### 🧪 Testing

- *(toc)* Add unit tests for ToC package
- *(toc)* Add golden file test for ToC output
- *(config)* Add ToCConfig default and round-trip tests
- *(template)* Regenerate golden files with ToC markers
- *(cmd)* Add integration tests for ToC in update and create

### ⚙️ Miscellaneous Tasks

- Disable dependabot
## [0.0.8] - 2026-03-14

### 🐛 Bug Fixes

- *(wiki)* Use "Home" title for root index.md in nav

### 📚 Documentation

- Update README, DEVELOPMENT, design statuses, and add CLAUDE.md

### ⚙️ Miscellaneous Tasks

- Initialize docz and wiki for the project
## [0.0.7] - 2026-03-14

### 🚀 Features

- *(config)* Add WikiConfig struct with defaults and wire into config system
- *(wiki)* Add titles package for nav title extraction
- *(wiki)* Add NavEntry, ScanDocs, and SortEntries for nav tree building
- *(wiki)* Add MkDocs YAML read/write and nav serialization
- *(cmd)* Add wiki init and wiki update commands
- *(cmd)* Wire wiki nav auto-update into docz create
- *(cmd)* Add verbose output and edge case handling to wiki commands

### 📚 Documentation

- *(impl)* Add implementation plan for wiki command (DESIGN-0002)
- Add wiki command documentation to README and DEVELOPMENT
- *(impl)* Mark IMPL-0002 as completed

### 🧪 Testing

- *(config)* Add wiki config default and round-trip tests
- *(wiki)* Add unit tests for title extraction functions
- *(wiki)* Add unit tests for nav tree scanning and sorting
- *(wiki)* Add unit tests for MkDocs YAML I/O and nav merging
- *(cmd)* Add integration tests for wiki init, update, and create
- *(wiki)* Add golden file test for nav output
## [0.0.6] - 2026-03-11

### 📚 Documentation

- *(design)* Add wiki command design doc for MkDocs TechDocs integration
- *(design)* Resolve open questions in wiki command design
## [0.0.5] - 2026-03-08

### 🚀 Features

- *(cmd)* Add type aliases and fix help text for all commands
## [0.0.4] - 2026-03-08

### 🐛 Bug Fixes

- *(config)* Wire plan type into config and fix docs
## [0.0.3] - 2026-03-07

### 🚀 Features

- *(docs)* Add investigation document type
## [0.0.2] - 2026-03-07

### 🚀 Features

- *(docs)* Add plan template, update impl template, and add project docs

### 🐛 Bug Fixes

- *(templates)* Suppress MD025/MD041 lint errors and add frontmatter to design doc
## [0.0.1] - 2026-02-24

### 🚀 Features

- *(template)* Add embedded default templates for all document types
- Implement core internal packages for template, config, and document
- *(cmd)* Implement init, create, and version commands
- *(index)* Implement index generation and update command
- *(cmd)* Add list command and fix all lint issues
- *(cmd)* Add template and config commands with tests
- Add Phase 5 polish - verbose flag, config validation, Makefile updates

### 🐛 Bug Fixes

- *(deps)* Move cobra and viper to direct dependencies
- *(cmd)* Extract json format string to constant

### 📚 Documentation

- Add design document and example scripts
- Mark implementation plan as completed

### 🚜 Refactor

- Move main.go to cmd/docz/main.go

### 🧪 Testing

- Add unit tests for template, config, and document packages
- Add integration and golden file tests for create and templates
