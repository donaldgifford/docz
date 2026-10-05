---
id: IMPL-0023
title: "Confluence export: the package and the CLI (DESIGN-0020 Phase A)"
status: Draft
author: Donald Gifford
created: 2026-10-05
---

<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL-0023: Confluence export: the package and the CLI (DESIGN-0020 Phase A)

<!--toc:start-->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: The sync block, goldmark, and the layer rule](#phase-1-the-sync-block-goldmark-and-the-layer-rule)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2: The renderer](#phase-2-the-renderer)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 3: The client](#phase-3-the-client)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 4: Export, the reconcile](#phase-4-export-the-reconcile)
    - [Tasks](#tasks-3)
    - [Success Criteria](#success-criteria-3)
  - [Phase 5: docz export confluence](#phase-5-docz-export-confluence)
    - [Tasks](#tasks-4)
    - [Success Criteria](#success-criteria-4)
  - [Phase 6: Live validation on the scratch site](#phase-6-live-validation-on-the-scratch-site)
    - [Tasks](#tasks-5)
    - [Success Criteria](#success-criteria-5)
  - [Phase 7: Documentation](#phase-7-documentation)
    - [Tasks](#tasks-6)
    - [Success Criteria](#success-criteria-6)
  - [Phase 8: v2.0.0-beta.7](#phase-8-v200-beta7)
    - [Tasks](#tasks-7)
    - [Success Criteria](#success-criteria-7)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Dependencies](#dependencies)
- [Open Questions](#open-questions)
  - [1. Where do the hooks live?](#1-where-do-the-hooks-live)
  - [2. What is exclude relative to?](#2-what-is-exclude-relative-to)
  - [3. Where does the property's docz version come from?](#3-where-does-the-propertys-docz-version-come-from)
  - [4. Is there an offline render?](#4-is-there-an-offline-render)
  - [5. How does a page without frontmatter get its title?](#5-how-does-a-page-without-frontmatter-get-its-title)
  - [6. Does this repository enable the block?](#6-does-this-repository-enable-the-block)
  - [7. Does Phase A ship as its own beta?](#7-does-phase-a-ship-as-its-own-beta)
  - [8. Which remotes get blob URLs?](#8-which-remotes-get-blob-urls)
  - [9. What does the live test look like?](#9-what-does-the-live-test-look-like)
  - [10. What happens after a failed request?](#10-what-happens-after-a-failed-request)
- [References](#references)
<!--toc:end-->

<!--docz:objective:start-->
## Objective

Deliver Phase A of DESIGN-0020: `pkg/export/confluence`, a goldmark
renderer from a docz document to Confluence storage format with a
`net/http` client for the Cloud v2 API and an `Export` operation that
keeps one page per document under a parent; the dormant `sync.confluence`
block in `.docz.yaml`; and `docz export confluence`, validated live on the
scratch site over this repository's own `docs/` before anything server-side
is attempted. The docz-api job is Phase B, designed separately (INV-0019
decision 3).

**Implements:** DESIGN-0020 (all thirteen open questions resolved
2026-10-05; moved to Approved with this plan), from INV-0019 and issue
[#142](https://github.com/donaldgifford/docz/issues/142).

<!--docz:objective:end-->

<!--docz:scope:start-->
## Scope

<!--docz:in-scope:start-->
### In Scope

- `config.SyncConfig` and `ConfluenceSyncConfig` with normalisation,
  validation, the generated `.docz.yaml` block, and a parity normaliser
- goldmark as a direct dependency, allow-listed for `pkg/export/...` only
- `pkg/export/confluence`: `Render`, the `Client` interface and
  `HTTPClient`, `Export`, typed errors, hooks, goldens, fakes, and a fuzz
  target
- `docz export confluence` with `--type`, `--force`, `--dry-run`, `--out`,
  `--format`, and `--strict`; `GitResolver.RemoteURL`
- The consumer-module smoke test for the new package
- A live validation on the scratch site, recorded in this document, with a
  scoped API token
- README, CLAUDE.md, and DEVELOPMENT.md, and the `v2.0.0-beta.7` cut

<!--docz:in-scope:end-->

<!--docz:out-of-scope:start-->
### Out of Scope

- The docz-api job, its table, API surface, and the site's link (Phase B)
- Jira work items (INV-0019 decision 8, its own DESIGN)
- Page attachments for local images, `--prune`, and preserving
  Confluence-side comments across an update (DESIGN-0020 Non-Goals)
- Any change to how documents are written in markdown. DESIGN-0019's nested
  fence and the three broken links are fixed in their own `docs:` PR, which
  lands before Phase 6 reads the link report

<!--docz:out-of-scope:end-->
<!--docz:scope:end-->

## Implementation Phases

Each phase builds on the previous one. A phase is complete when all its tasks
are checked off and its success criteria are met. The phases are ordered so
that nothing talks to Confluence until the renderer is proven over the
corpus and the client over `httptest`, and nothing is documented until the
live run has shown what the words should say.

---

<!--docz:phase:start-->
### Phase 1: The `sync` block, goldmark, and the layer rule

This phase gives the repository somewhere to say which site and space it
exports to, and lets `pkg/export/` import a markdown parser without
loosening the rule for the core. After it, `docz config` prints a
`sync:` block, `docz init` writes one disabled, and nothing else is
different.

<!--docz:tasks:start-->
#### Tasks

- [x] `pkg/doczcore/config/sync.go`: `SyncConfig{Confluence
  ConfluenceSyncConfig}` and `ConfluenceSyncConfig{Enabled, Site, Space,
  Parent, Types []string, Exclude []string, APIPages bool, Mermaid
  MermaidSyncConfig}` with `MermaidSyncConfig{Viewer string}`. Every
  field carries `yaml` and `json` tags with the DESIGN-0020 §5 spellings
  (`sync`, `confluence`, `enabled`, `site`, `space`, `parent`, `types`,
  `exclude`, `api_pages`, `mermaid`, `viewer`). `Config.Sync` sits after
  `API`, and `DefaultConfig()` sets `Mermaid.Viewer` to `auto`
- [x] `normalizeSync(cfg *Config)` on both `Load` paths, right after
  `normalizeAPI` (`config.go:284` and `:732`, and therefore `ParseBytes`):
  trims one trailing `/` from `site`, runs each `exclude` entry through
  `normalizeExcludePrefix`, and backfills an empty `viewer` to `auto`. It
  never rejects
- [x] `validateSync()` from `Validate()` after `validateAPI`, **only when
  enabled** (the DESIGN-0010 dormancy rule), wrapping a new
  `ErrInvalidSync`:
  - `site` parses with `url.Parse`, scheme `https`, non-empty host, no
    path, query, fragment, or userinfo;
  - `space` and `parent` non-empty after trimming;
  - each `types` entry resolves through `resolveType` to an **enabled**
    type, else the error names the token and the enabled set;
  - each `exclude` entry passes `validateRepoRelativeDir`;
  - `api_pages: true` requires `API.Enabled`;
  - `viewer` is `auto`, `off`, or matches
    `^[0-9a-f-]{36}/[0-9a-f-]{36}/static/[a-z0-9-]+$`
- [x] `sync_test.go`: a dormant block with every field wrong loads and
  validates; one case per rule above; `ParseBytes` and both `Load` paths
  agree on a block (`parsebytes_test.go`'s pattern); a `types:` entry
  naming a disabled built-in (`runbook` on defaults) is rejected
- [x] `json_test.go`: `TestConfigJSON_MarshaledShape` gains the `sync`
  object with its nested `confluence` and `mermaid`;
  `TestJSONTags_MirrorYAML` passes without edits, which is the point of it
- [x] `pkg/doczcore/doctemplate/templates/docz_yaml.tmpl`: a `sync:` block
  after `api:`, disabled, with a comment per field, the two credential
  variables named (`ATLASSIAN_EMAIL`, `ATLASSIAN_API_TOKEN`), and the three
  scopes a token needs. `parity_baseline_test.go`'s round trip and
  `doctemplate/promoted_test.go`'s `DefaultConfigYAML` test pass; the
  rendered file carries `sync:` with `enabled: false`
- [x] `test/parity/parity.go`: a `sync` normaliser that drops the `sync:`
  block `docz init` now writes, applied on both sides in `runCase` beside
  `runbook` (the golden was captured from v1.2.2, which never wrote one).
  Unit tests in `parity_norm_test.go` in the shape of
  `TestRunbookNormalizer`; `test/parity/README.md` lists the delta
- [x] `go get github.com/yuin/goldmark@v1.8.6` as a direct require (the
  version the prototype proved), `go mod edit -fmt`, and go.sum settled
  with targeted `go get`, never a bare `go mod tidy`. `just license-check`
  passes: goldmark is MIT
- [x] `pkg/doczcore/layer_test.go`: `TestLayerRules_ThirdPartyDependencies`
  takes its allow-list as module → package-prefix: `go.yaml.in/yaml/v3`
  for every package, `github.com/yuin/goldmark` for packages under
  `pkg/export/` only. The matcher is a function with its own table test,
  including the negative case of goldmark named from a core package. The
  rule that the core never imports a type package gains `pkg/export/...`
  to its forbidden set, so `corePackages` stays as it is
- [x] `docz config` prints the block; `docz init` in a `t.TempDir()`
  writes it disabled (`cmd/init_test.go` is frozen, so this goes in a new
  `cmd/export_test.go` file that Phase 5 extends)

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `just test` and `just parity` pass, with the v1.2.2 goldens unchanged
- A `.docz.yaml` whose enabled block breaks any rule fails `Validate`
  naming the field; a dormant block with the same content loads
- `docz init` writes `sync:` disabled, and `docz config` round-trips it
- `go.mod` carries goldmark, and the layer test allows it under
  `pkg/export/` and nowhere else

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 2: The renderer

This phase ports the prototype's renderer into the package and finishes
the DESIGN-0020 §2 table. After it, every row of that table has a golden,
every corpus fixture renders well-formed, and nothing in the package has
opened a file or a socket.

<!--docz:tasks:start-->
#### Tasks

- [x] `pkg/export/confluence/doc.go`: the package comment with the
  EXPERIMENTAL paragraph every v2-line package carries (ADR-0002 Decision
  7), the layering statement (a sibling of `pkg/wiki`, imports the core,
  never imported by it), and the one-paragraph description of what a page
  is
- [x] `render.go`: `Render(src []byte, opts RenderOptions) (Rendered,
  error)` with `RenderOptions{Resolve LinkResolver, Mermaid MermaidMode,
  ViewerKey, Source, SourceURL, Title string}`, `Rendered{ID, Title,
  Body, Hash, Links}`, `Link{Href, Text, Line}`, `LinkTarget`,
  `LinkResolver func(from, href string) LinkTarget`, `MermaidMode`
  (`MermaidViewer` zero value, `MermaidCode`), and the `DefaultViewerKey`
  constant from DESIGN-0020 §4. Frontmatter is cut with
  `document.ParseFrontmatter`; `Title` empty means `ID: Title` from the
  frontmatter, then `docparse.Title`, and a document with no frontmatter
  and no `opts.Title` is `document.ErrNoFrontmatter` wrapped (Open Question
  5). CR line endings are rejected the way the type packages reject them.
  `Render` clones its input and never modifies it
- [x] `nodes.go`: the goldmark node renderer ported from the prototype's
  `render.go` (`storage.RegisterFuncs` and its twelve functions), one
  function per §2 row: heading (H1 dropped, H2–H6 passed), fenced and
  indented code to the `code` macro with the language map (`sh`, `shell`,
  `zsh`, `console` → `bash`; `yml` → `yaml`; unknown → `none`) and `]]>`
  split across two CDATA sections, table with `data-layout="full-width"`,
  HTML block and inline raw HTML through the allow-list (`br` rewritten,
  `kbd`, `sub`, `sup`, `span`, `details`, `summary`) or escaped whole with
  `xml.EscapeText`, alert blockquotes to the panel macros with the marker
  cut from the AST, task lists to `ac:task-list`, images to `ac:image`
  with `ri:url` for a remote source and a link for a local one, and every
  `<!--…-->` comment dropped
- [x] `links.go`: in-page `#slug` links mapped back to heading text through
  `docparse.Headings` over the body and written as
  `#<TitleNoSpaces>-<HeadingNoSpaces>` with `:` as `%3A`; relative links
  through `opts.Resolve`, written as `ac:link`/`ri:page` for a `PageTitle`
  (fragment appended) and `<a href>` for a `URL`; a zero `LinkTarget`
  writes the text alone and appends to `Rendered.Links`; absolute links
  and autolinks pass through
- [x] `macros.go`: the `toc` macro for a `<!--toc:start-->`…`<!--toc:end-->`
  span (located with `docparse.Regions` kind `toc`, body dropped), the
  viewer macro and `expand` pair for a mermaid fence in `MermaidViewer`
  mode or the open `code` macro in `MermaidCode`, and the `info` banner
  that opens the body when `Source` is set, with `SourceURL` as its link
  when present. The viewer's `local-id` is rendered as a fixed placeholder,
  the body is hashed, and then each placeholder is replaced with a
  `crypto/rand` UUID v4, so `Hash` is stable across runs and two macros on
  one page never share an id
- [x] `wellformed.go`: the body wrapped in a root element declaring the
  `ac` and `ri` namespaces and decoded with `encoding/xml` in strict mode;
  a failure is `*MalformedError{Line, Err}` and `Render` returns it with
  no body. This is the check that turned the prototype's five failures
  into findings rather than pushes
- [x] Goldens under `testdata/render/<case>.md` with `.xhtml` siblings,
  regenerated with `-update`, one case per §2 row and the specimen the
  prototype used: the banner with and without a URL, every language in
  the map, `]]>` in a code body, mermaid in both modes, two mermaid fences
  on one page, the ToC span, region markers, each alert kind, a nested
  task list, a wide table, an in-page anchor whose page title has a colon,
  a cross-document link with a fragment, an unresolved link, a remote and
  a local image, a `<details>` block, a `<status>` placeholder in prose,
  and a footnote
- [x] `corpus_test.go`: every `.orig.md` under
  `pkg/{rfc,adr,design,impl,investigation,runbook}/testdata/` (38) and
  each embedded template rendered through `doctemplate.Resolve` must come
  back well-formed with zero unresolved in-page anchors. The test reads
  the snapshots, never `docs/`
- [x] `invariants_test.go` and `FuzzRender`: the input is unmodified, two
  renders of one input match byte for byte after the `local-id`
  placeholders are restored, `Hash` is equal across the two, and the fuzz
  target pins never-panic and well-formed-or-error over the goldens as
  seeds, run for thirty seconds locally and recorded here
  Done: `FuzzRender` ran 30s clean, 5.2M executions, on 2026-10-05.
- [x] `go vet`, `just lint`, and `just fmt` are clean; no non-test file in
  the package imports `os`, `io/fs`, or `net`

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- Every row of DESIGN-0020 §2 has a golden case, and the goldens match
- The 38 fixtures and seven templates render well-formed with no
  unresolved in-page anchor
- `FuzzRender` runs thirty seconds clean
- `grep -l '"os"\|"net' pkg/export/confluence/*.go` lists only test files

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 3: The client

This phase speaks to Confluence Cloud and nothing else. After it, every
endpoint `Export` needs has a request-shape test against `httptest`, both
token kinds work through one base URL, and a rate limit is retried.

<!--docz:tasks:start-->
#### Tasks

- [x] `client.go`: the `Client` interface of DESIGN-0020's API changes
  (`SpaceID`, `FindPage`, `CreatePage`, `UpdatePage`, `Property`,
  `SetProperty`, `Children`) and its value types: `Page{ID, Title,
  ParentID, SpaceID, Version int, WebURL}`, `NewPage{SpaceID, ParentID,
  Title, Body}`, `PageUpdate{Title, ParentID, Body, Version int, Message}`,
  `Property{ID, Key, Value json.RawMessage, Version int}`. A missing page
  or property is `(nil, nil)`, not an error
- [x] `httpclient.go`: `NewHTTPClient(site, email, token string)
  *HTTPClient` with a functional `WithHTTPClient(*http.Client)` for tests.
  The cloud id is resolved once, lazily, from
  `GET <site>/_edge/tenant_info` (unauthenticated; the error is memoised so
  a bad site fails every call the same way), and every request goes to
  `https://api.atlassian.com/ex/confluence/<cloudId>/wiki/api/v2/…` with
  basic auth. `WebURL` is `<site>/wiki` joined with `_links.webui`
- [x] The endpoints, each a method of its own: `GET /spaces?keys=`,
  `GET /pages?space-id=&title=` (version and parent come back; the body
  is never fetched), `POST /pages` with `status: current` and a storage
  body, `PUT /pages/{id}` with `parentId`, the body, and `version{number,
  message}`, `GET /pages/{id}/properties?key=docz`, `POST` and
  `PUT /pages/{id}/properties[/{propertyId}]` (the update carries
  `version.number + 1`), and `GET /pages/{id}/children` following
  `_links.next` cursors until exhausted
- [x] `errors.go`: `AuthError{Status}` for 401 and 403, `ConflictError{Title,
  Have, Want int}` for a 409 on a stale version, `RequestError{Op, Status
  int, Body string}` for everything else with `Body` cut to 512 bytes, and
  `MalformedError` from Phase 2. No error, log, or report ever carries the
  `Authorization` header or the token
- [x] A `429` honours `Retry-After` (seconds or an HTTP date), waits, and
  retries up to three times before returning a `RequestError`; a `429`
  with no header backs off two seconds doubling. The wait is a `time.Timer`
  select against `ctx.Done()`, so cancellation does not sleep
- [x] `httpclient_test.go` against `httptest.NewServer`: the cloud id is
  requested exactly once across many calls and the gateway host receives
  everything after it; each endpoint's method, path, query, and JSON body
  against a golden; each response decoded; children paginated across
  three pages; 401 → `AuthError`; 429 twice then 200 → success with two
  waits observed through a fake clock; 429 four times → `RequestError`;
  409 → `ConflictError`; a context cancelled mid-wait returns `ctx.Err()`
- [x] `live_test.go` behind `//go:build live` (Open Question 9): reads
  `ATLASSIAN_SITE`, `ATLASSIAN_EMAIL`, `ATLASSIAN_API_TOKEN`, and
  `DOCZ_LIVE_SPACE` from the environment, resolves the space, and round
  trips one page and its property under a parent titled `docz live`. A
  `just export-live` recipe in `docz.just` sources
  `~/.config/docz/atlassian.env` and runs it; nothing in `just test` or CI
  sets the tag

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- Every `Client` method has a request-shape and a decoding test, and
  `just test` opens no socket outside the package's own `httptest` server
- The cloud id is fetched once per client, and a scoped-token-only URL
  shape is the only shape the tests see
- `AuthError`, `ConflictError`, and `RequestError` are each produced by a
  test, and none contains the token
- `just export-live` passes against the scratch site with both the scoped
  and the unscoped token
- Status 2026-10-05: `just export-live` passes with the unscoped token.
  The scoped-token run is **deferred - human required**: it needs a scoped
  token minted in the Atlassian account, which Phase 6 asks for

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 4: `Export`, the reconcile

This phase is the operation: list, render, find, decide, write, archive,
report. After it, a fake client can drive every path of DESIGN-0020 §3's
flowchart and the report says what happened to each page.

<!--docz:tasks:start-->
#### Tasks

- [x] `export.go`: `Export(ctx, rp *repo.Repo, opts ExportOptions)
  (Report, error)` with `ExportOptions{Client, Types, IDs, Force, DryRun,
  Resolve LinkResolver, Version string}` (Open Question 3 for `Version`),
  `Report{Pages []PageResult, DryRun bool}`, `PageResult{ID, Title,
  Source, Action, PageID, URL, Version, Reason, Links}`, and `Action`
  with `Created`, `Updated`, `Unchanged`, `Skipped`, `Archived`, `Failed`
  and a `String()`. A block that is absent or disabled is a `ConfigError`
  before any request
- [x] The tree, in this order: the space id; the parent page found by
  title or created (its body is the rendered `api:` landing page when
  `api_pages` is on, else one paragraph naming the repository); one type
  page per exported type, titled with the type's nav title, body the
  type's README index rendered with `Title` set; then the documents of
  each type in id order from `rp.List`, minus `exclude` prefixes; then
  the `api:` additional docs under the parent when `api_pages` is on.
  `Types` and `IDs` narrow the document set; an id resolves through
  `rp.Find`, and an unknown one is `repo.NotFoundError` passed up
- [x] The resolver: `Export` builds the export set (repository-relative
  path → page title) first and resolves every relative link against it
  before falling back to `opts.Resolve`, so a caller supplies only the
  rule for what is *not* exported (the blob URL) and the index pages'
  links land on the pages beneath them
- [x] The per-page reconcile exactly as the §3 flowchart: find by title;
  no page → create; page with no `docz` property → skip `not docz's page`
  unless `Force`, which adopts it; property whose `version` differs from
  the page's → skip `edited in Confluence (v<have>, expected v<want>)`
  unless `Force`; equal hash → `Unchanged` with no write; else update with
  `version + 1`, the message `docz sync <id> <hash-prefix>`, and the
  expected parent id. Every create and update is followed by the property
  `{id, source, hash, version, docz}`, created or updated through its own
  version
- [x] Orphans, **only on a full export** (no `Types`, no `IDs`): the
  children of the parent and of every type page that carry a `docz`
  property whose `id` is not in the export set are moved under an
  `Archive` child of the parent (created when first needed) and reported
  `Archived`. A narrowed run never archives, because a partial set would
  orphan everything outside it. DESIGN-0020 §3 does not say this and is
  amended in Phase 7
- [x] `DryRun`: every read happens, no write does, and the report carries
  the action each page *would* take with `Report.DryRun` set
- [x] A failed request fails that page (`Failed`, with the reason) and the
  run continues to the next; `Export` returns the report and an error
  wrapping the first failure once the loop ends (Open Question 10)
- [x] `hooks.go` (Open Question 1): `Hooks{PageDone func(PageResult),
  Request func(method, path string, status int)}` carried in the context
  by `WithHooks`/`HooksFrom` in `repo`'s shape, with `HooksFrom` never
  nil. `HTTPClient` fires `Request` after each response; `Export` fires
  `PageDone` after each page
- [x] Context is checked between pages, never mid-page; a cancelled run
  returns the report so far with `ctx.Err()`, and what was written stays
  written
- [x] `fake_test.go`: an in-memory `Client` holding pages by id with
  titles, parents, versions, bodies, properties, and a request log, plus
  a `fail` switch for one operation. `export_test.go` drives `Export` over
  a `t.TempDir()` repository created through `repo.Init` and `Create`
  through: first run all `Created` with the tree shape asserted; second
  run all `Unchanged` with zero writes in the log; a changed document →
  `Updated` at `version + 1`; a page edited in Confluence → `Skipped`
  with both versions, then `Updated` under `Force`; a page with no
  property → `Skipped`, then adopted under `Force`; a removed document →
  `Archived` under `Archive`, then back under its type page when restored;
  a narrowed run archiving nothing; `api_pages` on and off; `exclude`;
  explicit ids; `DryRun` with no writes; a cancelled context after the
  first page; one failing update → `Failed`, the rest written, the error
  returned
- [x] `golangci-lint` clean, including `funlen` on `Export` (split the tree
  build, the reconcile, and the orphan pass into their own functions from
  the start)

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- Every arm of the §3 flowchart is reached by a fake-driven test, and the
  report is asserted action by action
- A second run over an unchanged repository makes no write
- A narrowed run archives nothing; a full run archives exactly the pages
  whose documents are gone
- `Export` prints nothing, holds no logger, and the layer tests still pass

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 5: `docz export confluence`

This phase is the command: one call plus printing. After it, a person
with the block enabled and the two variables set can export a checkout,
and the consumer module proves the package is reachable from outside.

<!--docz:tasks:start-->
#### Tasks

- [x] `cmd/git.go`: `GitResolver` gains `RemoteURL(ctx) string`, from
  `git remote get-url origin` normalised to `https://github.com/<o>/<r>`
  from both the SSH and the HTTPS spellings with `.git` dropped, and
  `DefaultBranch(ctx) string` from `git symbolic-ref --short
  refs/remotes/origin/HEAD` with `main` as the fallback. `staticGit` gains
  both. A remote that is not GitHub-shaped yields `""` and every
  unexported link stays unresolved (Open Question 8)
- [x] `cmd/export.go`: an `export` parent command and the `confluence`
  subcommand with the DESIGN-0020 §6 flags (`--type` repeatable,
  `--force`, `--dry-run`, `--out`, `--format text|json`, `--strict`), the
  Long help naming the block, the variables, and the scopes.
  `runExportConfluence` resolves flags, reads `ATLASSIAN_EMAIL` and
  `ATLASSIAN_API_TOKEN` (missing → exit 2 before any request), builds
  `confluence.NewHTTPClient` from `Cfg.Sync.Confluence.Site`, builds the
  blob-URL resolver from `RemoteURL` and `DefaultBranch`, sets
  `Version` from the binary's version, installs the hooks, and calls
  `(*Runner).ExportConfluence(ctx, opts)`, which is `confluence.Export`
  plus printing
- [x] `--out <dir>`: each page's body written as `<dir>/<id>.xhtml`
  alongside the run (and, with `--dry-run`, as the way to inspect a render
  without a write; Open Question 4 decides whether that needs credentials)
- [x] The text report in the §6 shape: one line per page with the action,
  title, version, and URL, an indented `unresolved link:` line per link,
  and the totals line; `--format json` marshals the `Report` with
  snake_case field names
- [x] Exit codes: `0` for a run with no `Failed` page (skips included);
  `1` when any page `Failed`, or under `--strict` when any was `Skipped`;
  `2` for `ConfigError`, `AuthError`, missing credentials, an unknown
  `--type`, or an id that resolves to nothing. `AuthError` is `2` and not
  `1` because its fix is configuration, which this task records in the
  design's exit-code paragraph (Phase 7)
- [x] `cmd/hooks.go`: `(*Runner).exportHooks()` maps `PageDone` and
  `Request` to debug lines, installed with
  `cmd.SetContext(confluence.WithHooks(…))`, so `--verbose` narrates each
  request and each page
- [x] `cmd/export_test.go` (new; the frozen files are untouched): a
  `Runner` with a fake `Client` injected through a package-level seam in
  the `runner` style, covering text and JSON output against goldens,
  each exit code, `--out`, `--strict`, `--dry-run`, a disabled block,
  missing credentials with the fake asserting zero calls, and `--verbose`
  reaching the logger
- [x] `test/consumer/consumer_v2_export_test.go`: `Render` over an inline
  document and `Export` over `repoFixture` against a minimal fake `Client`
  defined in the test, asserting one `Created` per document;
  `test/consumer/doc.go` counts eighteen `pkg/` packages
- [x] `docz --help` lists `export`; `just parity` is green with no change,
  since a new command is a permitted delta; `just ci` passes

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `docz export confluence --dry-run` against the fake reports every page
  and writes nothing; `--out` writes one `.xhtml` per page
- The three exit codes are each produced by a `cmd/export_test.go` case
- `just test-consumer` imports and exercises `pkg/export/confluence`
- `just ci` passes

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 6: Live validation on the scratch site

This phase is DESIGN-0020's Rollout step 1 run for real, over this
repository's `docs/`, against the DOCZ space INV-0019 used. Each result is
recorded under its task with the page ids, as IMPL-0022 recorded its
release run.

<!--docz:tasks:start-->
#### Tasks

- [ ] **(human)** Create a **scoped** API token with
  `read:space:confluence`, `read:page:confluence`, and
  `write:page:confluence` and put it in `~/.config/docz/atlassian.env`
  as `ATLASSIAN_API_TOKEN`, keeping the unscoped one under another name
  for the comparison run
  - **Deferred - human required** (2026-10-05): the token is minted in
    the Atlassian account. Every run below used the unscoped token
- [ ] Confirm the scope list: a `--dry-run` with the scoped token either
  reads every page or returns a `403` naming the missing scope. Record
  the final list here and correct DESIGN-0020 §5 and the README if it
  differs
  - **Deferred - human required**: waits on the scoped token above
- [x] `.docz.yaml`: enable the block (Open Question 6) with the scratch
  site, space `DOCZ`, parent `docz`, `api_pages: true`, and `exclude:
  [examples, archive]` in step with `api.exclude`
- [ ] The `docs:` PR fixing DESIGN-0019's nested fence and the three
  broken links has merged, so the link report below is a clean baseline
  - PR [#152](https://github.com/donaldgifford/docz/pull/152) fixes the
    fence and four broken links (`docs/index.md`'s plan entry, DESIGN-0016's
    DESIGN-0015 slug, `DEVELOPMENT.md`'s `deploy/README.md`); an offline
    `--dry-run --out` over it reports no unresolved link. **Merging is
    deferred - human required**; until then the runs below report the six
    links it fixes
- [ ] First full run: every document, both type index pages per enabled
  type, and the two additional docs are `Created` or, for the prototype's
  pages that carry its `{id, hash}` property shape, `Skipped` and then
  adopted under `--force`. In the browser: RUNBOOK-0001's tables are wide
  with the page centred; DESIGN-0019's four diagrams draw with the source
  folded beneath each; DESIGN-0020's three draw; an in-page anchor in
  INV-0019 scrolls; a cross-document link opens the right page; a type
  page's index table links to the pages beneath it; the banner names the
  source and links to GitHub; a `[!NOTE]` renders as a panel; an IMPL's
  task list renders with its checkboxes
  - Done 2026-10-05, unscoped token: `81 pages: 72 created, 4 skipped, 5
    archived`. The four skips were the prototype's parent and its
    DESIGN-0019, IMPL-0022, and RUNBOOK-0001 pages. That run found two bugs,
    fixed with tests before going on. The prototype's `{id, hash}` property
    read as "edited in Confluence (expected v-1)". The orphan pass also
    archived the skipped pages along with the prototype's `Markdown rendering
    specimen` and `TEST-0001` pages. `--force` then adopted the four
    (`4 updated, 72 unchanged`), moving the three documents back out of
    Archive. The two prototype test pages stay under Archive (parent page
    65860)
  - **The browser checks are deferred - human required**: wide tables,
    diagrams, anchors, cross-page links, banner, panels, task lists
- [x] Second run: every page `Unchanged`, and `--verbose` shows one
  `FindPage` and one `Property` request per page and no `POST` or `PUT`
  - Done: `76 pages: 76 unchanged`, and `--verbose` logged 161 GETs: one
    `GET /pages` (the title lookup) and one `GET /properties` per page, seven
    `children` lists, `tenant_info` once, and `spaces` once. There were no
    `POST` and no `PUT`. The run before this one found that the parent was
    rewritten every run, because Confluence files it under the space homepage.
    It also found the orphan pass reading the property of every page it had
    just written. Both are fixed with tests
- [x] Edit one page in Confluence; a run reports it `Skipped` with both
  versions; `--force` reports it `Updated` and the edit is gone
  - Done: a hand edit to ADR-0001 (page 426032) through the API gave
    `skipped ... edited in Confluence (v2, expected v1); use --force`, and
    `--strict` exited 1. `--force` gave `updated ... v3`, and the page body no
    longer holds the edit
- [x] Add a scratch document on the branch, run, see it `Created`; delete
  it, run, see it `Archived` under `Archive`; restore it, run, see it
  `Updated` and back under its type page
  - Done with INV-0020 (page 492210, never committed). It was created at v1,
    archived under Archive after the file was removed, then updated to v3 and
    back under Investigations after it was restored. The scratch file is now
    removed, and its page sits under Archive
- [ ] The whole first-run check repeated with the unscoped token, with no
  difference in the report
  - **Deferred - human required**: needs the scoped token for the other
    half of the comparison
- [ ] Anything the eye finds goes back into Phases 2 to 5 as a task with
  a test before this phase is called done
  - The runs found three bugs, each fixed with a test: a foreign property
    was never edited or archived, the parent page stays where it is, and the
    orphan pass skips known pages. The browser pass is deferred with the
    first run's checks
  - **Deferred - human required**: what the browser pass finds

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- The acceptance list of DESIGN-0020's Rollout step 1 is met, with each
  item's evidence recorded in this document
- A second run makes no write, an edit is skipped and then overwritten
  under `--force`, and a deleted document's page sits under `Archive`
- Both token kinds produce the same report

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 7: Documentation

This phase makes the prose say what Phase 6 showed. It is after the live
run on purpose, so the words describe behaviour that has been seen.

<!--docz:tasks:start-->
#### Tasks

- [x] `README.md`: a `docz export confluence` row in the Commands table, a
  flags section beside `docz validate`'s, a `Sync` subsection under
  Configuration with the block, the two variables, how to create a scoped
  token and which scopes, that the Mermaid diagrams viewer must be
  installed on the site, and what `--force` does; the package table gains
  `pkg/export/confluence`
- [x] `CLAUDE.md`: a `pkg/export/confluence/` paragraph (renderer, client
  through the gateway, `Export`, hooks, typed errors, the full-export-only
  archive rule, the fixed `local-id` placeholder); the `config` paragraph's
  `SyncConfig`; the `cmd/` paragraph's `export.go` and `RemoteURL`; the
  parity paragraph's `sync` normaliser; the consumer module's eighteen
  packages; the layer-rule sentence on the scoped allow-list
- [x] `DEVELOPMENT.md`: the package in the layout section and
  `just export-live` beside the other local recipes
- [x] DESIGN-0020: amend §3 with the full-export-only archive rule, §6
  with `AuthError` as exit `2`, and §5 if Phase 6 changed the scope list,
  each as a dated note rather than a rewrite
- [x] `just validate` and `just ci` pass, and `git-cliff` regenerates the
  changelog

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- README documents the command, the block, and the token well enough that
  Phase 6 could be repeated from it alone
- CLAUDE.md names the package, the rule changes, and the new normaliser
- `just ci` passes

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 8: v2.0.0-beta.7

This phase cuts the beta by RUNBOOK-0001's first two procedures and
closes out. The binary is `docz`; `docz-api` ships unchanged in behaviour,
since its `config.ParseBytes` now merely accepts a `sync:` block.

<!--docz:tasks:start-->
#### Tasks

- [x] `charts/docz/Chart.yaml`: `version: 0.2.1`, `appVersion:
  "2.0.0-beta.7"` (bare), with the chart CHANGELOG and README regenerated
  and the version test updated (Open Question 7)
- [ ] Open the PR with `dont-release` and push; CI is green
  - PR [#153](https://github.com/donaldgifford/docz/pull/153) is open with
    `dont-release`. Every check passes except Security Scan, where
    govulncheck reports GO-2026-6505 (otel exporter and sdk v1.44.0, fixed
    in v1.45.0) and GO-2026-6218 (`net/url` in Go 1.26.5, fixed in 1.26.6).
    Both are on `main` too; this branch introduces neither. **Deferred - human required**: a
    `chore(deps)` bump of otel and the Go toolchain is
    a separate decision
- [ ] **(human)** Merge the PR **with a merge commit**
  - **Deferred - human required**: PR [#153](https://github.com/donaldgifford/docz/pull/153)
- [ ] Follow RUNBOOK-0001 Procedure 1 on `main`: `just release-check` and
  `just api release-check` pass
  - **Deferred - human required**: runs on `main` after the merge
- [ ] **(human)** `just release v2.0.0-beta.7` from the merge commit
  - **Deferred - human required**
- [ ] Follow RUNBOOK-0001 Procedure 2's verification steps and record each
  result here: the pre-release and its archives, both images at
  `2.0.0-beta.7` with `latest` not moved, `docz` 0.2.1 signed and attested
  with a bare `appVersion`, the deprecated charts skipped, ECR skipped
  - **Deferred - human required**: needs the published tag
- [ ] Replace RUNBOOK-0001's Last Verified row with this run
  - **Deferred - human required**: needs the release run above
- [x] File the follow-ups as issues: Phase B (the docz-api job, its own
  DESIGN); the Jira DESIGN (INV-0019 decision 8); page attachments for
  local images; `--prune`; an in-place update that keeps Confluence-side
  comments. Link rather than duplicate where #142 already covers one
  - Filed 2026-10-05: Phase B [#154](https://github.com/donaldgifford/docz/issues/154), Jira [#155](https://github.com/donaldgifford/docz/issues/155), image attachments [#156](https://github.com/donaldgifford/docz/issues/156), `--prune` [#157](https://github.com/donaldgifford/docz/issues/157), comment-preserving updates [#158](https://github.com/donaldgifford/docz/issues/158). #142 is closed and covered none of them
- [ ] `docz status set impl IMPL-0023 Completed` and
  `docz status set design DESIGN-0020 Implemented`, then `docz update`,
  through a branch and PR
  - **Deferred - human required**: only true once the beta ships

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `v2.0.0-beta.7` is published: binaries, both images, and `charts/docz`
  0.2.1, signed and attested
- RUNBOOK-0001 carries a Last Verified row for the beta.7 run
- IMPL-0023 is Completed and DESIGN-0020 Implemented, with the follow-ups
  filed

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:file-changes:start-->
## File Changes

| File | Action | Phase | Description |
| ---- | ------ | ----- | ----------- |
| `pkg/doczcore/config/sync.go`, `sync_test.go` | Create | 1 | The block, normalisation, validation |
| `pkg/doczcore/config/config.go`, `json_test.go` | Modify | 1 | `Config.Sync`, the two call sites, the JSON shape |
| `pkg/doczcore/doctemplate/templates/docz_yaml.tmpl` | Modify | 1 | The disabled block with its comments |
| `test/parity/parity.go`, `parity_norm_test.go`, `README.md` | Modify | 1 | `sync` normaliser, permitted delta |
| `go.mod`, `go.sum` | Modify | 1 | goldmark v1.8.6 |
| `pkg/doczcore/layer_test.go` | Modify | 1 | Scoped allow-list, `pkg/export/` forbidden to the core |
| `pkg/export/confluence/{doc,render,nodes,links,macros,wellformed}.go` | Create | 2 | The renderer |
| `pkg/export/confluence/testdata/render/*` | Create | 2 | Goldens |
| `pkg/export/confluence/{corpus,invariants,fuzz}_test.go` | Create | 2 | Corpus, invariants, fuzz |
| `pkg/export/confluence/{client,httpclient,errors}.go` and tests | Create | 3 | The interface, the gateway client, typed errors |
| `pkg/export/confluence/live_test.go`, `docz.just` | Create / Modify | 3 | The `live`-tagged round trip, `just export-live` |
| `pkg/export/confluence/{export,hooks,fake_test,export_test}.go` | Create | 4 | The operation, hooks, the fake, the scenarios |
| `cmd/git.go`, `git_test.go` | Modify | 5 | `RemoteURL`, `DefaultBranch` |
| `cmd/export.go`, `export_test.go`, `hooks.go` | Create / Modify | 5 | The command, its tests, the hook mapping |
| `test/consumer/consumer_v2_export_test.go`, `doc.go` | Create / Modify | 5 | The smoke test, eighteen packages |
| `.docz.yaml` | Modify | 6 | The enabled block |
| `README.md`, `CLAUDE.md`, `DEVELOPMENT.md`, DESIGN-0020 | Modify | 7 | The command, the block, the amendments |
| `charts/docz/Chart.yaml`, `CHANGELOG.md` | Modify | 8 | 0.2.1 / beta.7 |

<!--docz:file-changes:end-->

<!--docz:testing:start-->
## Testing Plan

- [x] Config: one case per validation rule, dormancy, `ParseBytes` ≡
  `Load`, the JSON shape, and the parity normaliser (Phase 1)
- [x] Renderer goldens for every §2 row, the 38-fixture and seven-template
  corpus, the invariants, and `FuzzRender` (Phase 2)
- [x] `HTTPClient` against `httptest`: request shapes, decoding,
  pagination, the cloud id fetched once, 401, 409, and 429 (Phase 3)
- [x] `Export` against the fake through every flowchart arm, the orphan
  rules, `api_pages`, `DryRun`, cancellation, and a failing request
  (Phase 4)
- [x] `cmd/export_test.go` output goldens and exit codes, and `just
  test-consumer` with the new package (Phase 5)
- [ ] The live run over this repository, recorded in Phase 6, with both
  token kinds
  - **Deferred - human required**: the unscoped half is recorded in Phase 6; the scoped half waits on the token
- [ ] `just ci` green on the PR, and the published beta.7 artifacts
  verified by RUNBOOK-0001 Procedure 2 (Phase 8)
  - **Deferred - human required**: govulncheck's two advisories from `main`, and the release

<!--docz:testing:end-->

<!--docz:dependencies:start-->
## Dependencies

- DESIGN-0020's resolved open questions. This plan follows them and
  re-opens none; the two amendments it makes (the full-export-only
  archive rule, `AuthError` as exit `2`) tighten rather than change them
- goldmark v1.8.6 (MIT), the version the INV-0019 prototype ran
- The scratch Atlassian site with the Mermaid diagrams viewer installed, a
  scoped API token created by hand, and `~/.config/docz/atlassian.env`
  holding the credentials (never committed, never echoed)
- The `docs:` PR fixing DESIGN-0019's nested fence and the three broken
  links, merged before Phase 6
- `v2.0.0-beta.6`'s release path, unchanged

<!--docz:dependencies:end-->

<!--docz:open-questions:start-->
## Open Questions

> Each question is numbered. Option `a` is my recommendation, the later
> letters are alternatives, and the last is "other" for your own answer.
> All ten were resolved on 2026-10-05.

### 1. Where do the hooks live?

- a. **`confluence.Hooks` in the package, carried in the context by its
  own `WithHooks`/`HooksFrom`, in `repo`'s shape.** Two events, `PageDone`
  and `Request`. The package narrates its own work, `repo.Hooks` stays
  the repository tier's, and `cmd/hooks.go` maps both structs
- b. Add `PageDone` and `Request` fields to `repo.Hooks`, so one context
  value carries everything. It makes the core's hook struct know about an
  integration it never imports
- c. A callback field on `ExportOptions`, with no context plumbing
- d. Other.

> **Resolved 2026-10-05: (a).**

### 2. What is `exclude` relative to?

- a. **`docs_dir`, exactly as `api.exclude` is** (`templates/` and
  `archive` are the entries people will write), and DESIGN-0020 §5's
  table, which says repo-relative, is corrected in Phase 7. One rule for
  both deny-lists
- b. The repository root, as the design table says, so a path outside
  `docs_dir` can be excluded too. Nothing outside `docs_dir` is exported
  except the `api:` additional docs, which have their own list
- c. Other.

> **Resolved 2026-10-05: (a).**

### 3. Where does the property's `docz` version come from?

- a. **`ExportOptions.Version`, set by `cmd/` from the binary's ldflags
  version** (the same value `docz version` prints) and by docz-api from
  its own. The package has no version of its own to report
- b. `runtime/debug.ReadBuildInfo()` inside the package, so every caller
  gets the module version for free. It reports `(devel)` for a local
  build and the module version rather than the binary's
- c. Other.

> **Resolved 2026-10-05: (a).**

### 4. Is there an offline render?

- a. **`--dry-run --out <dir>` with no credentials set renders every page
  to disk and makes no request**, reporting each as `rendered`. A dry run
  with credentials still reads Confluence to say `created` or
  `unchanged`. This is the prototype's `-mode render`, which is how most
  renderer bugs were found, at the cost of one branch in the handler
- b. A separate `--render-only` flag, explicit rather than inferred from
  the absence of credentials
- c. No offline mode; `Render` is reachable from a test and that is
  enough
- d. Other.

> **Resolved 2026-10-05: (a).**

### 5. How does a page without frontmatter get its title?

- a. **`RenderOptions.Title` overrides, and `Export` sets it for the type
  index pages (the nav title), the parent page (`sync.confluence.parent`),
  and the additional docs (`docparse.Title`, then the path).** A document
  with no frontmatter and no override is an error, as in the design
- b. No override: `Render` always derives the title (`ID: Title`, then
  `docparse.Title`, then the path), and the type page is titled by its
  README's H1. The README's H1 (`Design Documents`) and the nav title
  (`Design documents`) differ in case, and a repository that overrides
  the index header could title the page anything
- c. Other.

> **Resolved 2026-10-05: (a).**

### 6. Does this repository enable the block?

- a. **Yes, in `.docz.yaml`, against the scratch site's DOCZ space, with
  `api_pages: true`**, dogfooding the way the `api:` block does. The site
  is already named in INV-0019 and DESIGN-0020, so nothing new is exposed,
  and a clone without credentials gets exit `2`
- b. Keep the committed block absent and put the enabled block in
  `~/.docz.yaml`, which `Load` merges under the repository's. Nothing
  personal lands in the repository, but Phase B's ingest reads only the
  repository's file, so the block has to land there eventually
- c. Other.

> **Resolved 2026-10-05: (a).**

### 7. Does Phase A ship as its own beta?

- a. **Yes, `v2.0.0-beta.7`, with `charts/docz` 0.2.1 (an `appVersion`
  bump only).** Decision 3 wanted the CLI validated on its own, and a tag
  is how a consumer gets it
- b. No tag until Phase B lands, and both ship in one beta
- c. Other.

> **Resolved 2026-10-05: (a).**

### 8. Which remotes get blob URLs?

- a. **GitHub-shaped remotes only** (`git@github.com:o/r.git` and
  `https://github.com/o/r[.git]`), rendered as
  `https://github.com/o/r/blob/<default branch>/<path>`. Any other
  remote leaves unexported links unresolved and reported. Every fleet
  repository is on GitHub, and GitLab's `/-/blob/` shape can be added
  when one is not
- b. GitHub and GitLab shapes now
- c. A `sync.confluence.blob_url` template in config (`{path}`
  substituted), so any host works and nothing is inferred from the remote
- d. Other.

> **Resolved 2026-10-05: (a).**

### 9. What does the live test look like?

- a. **A `//go:build live` test that round-trips one page under a `docz
  live` parent and does not clean up**, driven by `just export-live`
  sourcing the credential file. A rerun finds its page and reports it
  `Unchanged`, which is itself the idempotence check; Phase 6 is the
  real proof
- b. The same test with cleanup through an unexported delete, which needs
  `delete:page:confluence` on the token for the tests alone
- c. No live test file; Phase 6's manual run is the only live proof
- d. Other.

> **Resolved 2026-10-05: (a).**

### 10. What happens after a failed request?

- a. **That page is `Failed` and the run continues**; `Export` returns the
  full report with an error wrapping the first failure, and the command
  exits `1`. A transient `5xx` on one page should not hold the other
  hundred, and the report says exactly which to rerun
- b. Stop at the first failure, as the design's exit-code paragraph can be
  read, leaving later pages untouched
- c. Other.

> **Resolved 2026-10-05: (a).**

<!--docz:open-questions:end-->

<!--docz:references:start-->
## References

- [DESIGN-0020](../design/0020-confluence-export-docz-documents-as-confluence-cloud-pages.md):
  the design this plan implements, with its thirteen resolved questions
- [INV-0019](../investigation/0019-docz-api-sync-to-external-documentation-services-confluence-and.md):
  the investigation, the prototype, and the eleven decisions
- [#142](https://github.com/donaldgifford/docz/issues/142): the issue
- [IMPL-0022](0022-runbook-the-sixth-built-in-type-v200-beta6.md): the
  plan this one follows in shape, and the beta.6 release run
- [RUNBOOK-0001](../runbook/0001-cut-a-v2-beta-release.md): the release
  procedure Phase 8 follows
- [DESIGN-0014](../design/0014-the-docz-api-as-one-unit-packages-types-functions-and-the-cmd.md):
  hooks, typed errors, and "one call plus printing"
- [`pkg/doczcore/layer_test.go`](../../pkg/doczcore/layer_test.go): the
  allow-list Phase 1 scopes
- [Confluence Cloud REST API v2](https://developer.atlassian.com/cloud/confluence/rest/v2/intro/)
- [Mermaid diagrams viewer](https://github.com/atlassian-labs/mermaid-diagrams-viewer)
- [goldmark](https://github.com/yuin/goldmark)

<!--docz:references:end-->
