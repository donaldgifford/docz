---
id: IMPL-0024
title: "Confluence export from docz-api: folders, comments, and the server job (DESIGN-0021)"
status: Draft
author: Donald Gifford
created: 2026-10-07
---

<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL-0024: Confluence export from docz-api: folders, comments, and the server job (DESIGN-0021)

<!--toc:start-->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: Configuration: layout and folder](#phase-1-configuration-layout-and-folder)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2: Reading through fs.FS](#phase-2-reading-through-fsfs)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 3: The folder layout, ownership, recorded ids, and Overwrite](#phase-3-the-folder-layout-ownership-recorded-ids-and-overwrite)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 4: The CLI in the folder layout](#phase-4-the-cli-in-the-folder-layout)
    - [Tasks](#tasks-3)
    - [Success Criteria](#success-criteria-3)
  - [Phase 5: Keeping inline comments (#158)](#phase-5-keeping-inline-comments-158)
    - [Tasks](#tasks-4)
    - [Success Criteria](#success-criteria-4)
  - [Phase 6: docz-api storage and configuration](#phase-6-docz-api-storage-and-configuration)
    - [Tasks](#tasks-5)
    - [Success Criteria](#success-criteria-5)
  - [Phase 7: The export task](#phase-7-the-export-task)
    - [Tasks](#tasks-6)
    - [Success Criteria](#success-criteria-6)
  - [Phase 8: The API and docz-site](#phase-8-the-api-and-docz-site)
    - [Tasks](#tasks-7)
    - [Success Criteria](#success-criteria-7)
  - [Phase 9: Deployment, documentation, live run, and release](#phase-9-deployment-documentation-live-run-and-release)
    - [Tasks](#tasks-8)
    - [Success Criteria](#success-criteria-8)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Dependencies](#dependencies)
- [Open Questions](#open-questions)
  - [1. How is the work split into pull requests?](#1-how-is-the-work-split-into-pull-requests)
  - [2. How many betas does this ship in?](#2-how-many-betas-does-this-ship-in)
  - [3. How does Phase 5's live check create an inline comment?](#3-how-does-phase-5s-live-check-create-an-inline-comment)
  - [4. Where does the in-memory Confluence used by tests live?](#4-where-does-the-in-memory-confluence-used-by-tests-live)
  - [5. How does the server get a type's index header without reading its own disk?](#5-how-does-the-server-get-a-types-index-header-without-reading-its-own-disk)
  - [6. How long does a Confluence client live in docz-api?](#6-how-long-does-a-confluence-client-live-in-docz-api)
  - [7. Where is enabling the export documented for operators?](#7-where-is-enabling-the-export-documented-for-operators)
  - [8. Does Phase 2 keep rp.List as a fallback?](#8-does-phase-2-keep-rplist-as-a-fallback)
- [References](#references)
<!--toc:end-->

<!--docz:objective:start-->
## Objective

Deliver DESIGN-0021, Phase B of the Confluence export: docz-api exports
every opted-in repository to Confluence Cloud after each ingest, with no
checkout, into a folder per repository in whatever allowed space the
repository names. The library and the CLI change first and are proven live
on their own: `Export` reads through an `fs.FS`, builds the folder layout
with prefixed titles, finds pages by recorded id, overwrites on request,
and carries inline comments across an update (#158). Then the server gains
the export task, its tables, the allow-list, the API surface, and the
docz-site link.

**Implements:** DESIGN-0021 (all thirteen open questions resolved
2026-10-06), from INV-0020 and issue
[#154](https://github.com/donaldgifford/docz/issues/154), with
[#158](https://github.com/donaldgifford/docz/issues/158) built inside it.

<!--docz:objective:end-->

<!--docz:scope:start-->
## Scope

<!--docz:in-scope:start-->
### In Scope

- `sync.confluence.layout` and `folder`, with `parent` optional in the
  folder layout
- `pkg/export/confluence`: reading through `fs.FS` (`os.DirFS` by default),
  the folder layout and title prefix, repository ownership on folders and
  pages, `ExportOptions.{FS, Repository, Pages, Folder, Overwrite}`, the
  new `Client` methods in `HTTPClient` and the fake, the new report fields,
  and the inline-comment splice
- `docz export confluence`: the repository name from the remote, the
  `WARNING` lines, and this repository's block moved to the folder layout
- docz-api: the migration and sqlc queries, `CONFLUENCE_*` configuration
  and the startup check, the `export:confluence` task, `internal/export`,
  the enqueue from ingest, the `-export` flag, metrics, spans, and logs
- `documentDTO.confluence_url`, `GET …/confluence`, the spec bump, and the
  docz-site link
- `charts/docz` values, Secret key, and alert; `contrib/`; a runbook
  procedure; README, CLAUDE.md, and DEVELOPMENT.md
- Live runs on the scratch site for the CLI, the comment splice, and the
  server, and the `v2.0.0-beta.8` cut

<!--docz:in-scope:end-->

<!--docz:out-of-scope:start-->
### Out of Scope

- Per-repository credentials, a trigger endpoint, or any admin surface
  (DESIGN-0021 Non-Goals)
- More than one Atlassian site per deployment
- Deleting pages (`--prune`, #157), images (#156), Jira (#155)
- Two-way sync, and re-anchoring a comment whose text was rewritten
- A sync-status view on docz-site beyond the document link

<!--docz:out-of-scope:end-->
<!--docz:scope:end-->

## Implementation Phases

Each phase builds on the previous one. A phase is complete when all its tasks
are checked off and its success criteria are met. Phases 1 to 5 change only
the library and the CLI and end in a live CLI run; nothing in docz-api
changes until the package is proven. Phases 6 to 9 are the server, its API,
and the deployment.

---

<!--docz:phase:start-->
### Phase 1: Configuration: `layout` and `folder`

After this phase a `.docz.yaml` can say which layout it wants and what its
folder is called. Nothing exports differently yet; the package reads the
new fields in Phase 3.

<!--docz:tasks:start-->
#### Tasks

- [x] `pkg/doczcore/config/sync.go`: `ConfluenceSyncConfig` gains
  `Layout string` (`yaml:"layout" json:"layout"`) and `Folder string`
  (`yaml:"folder,omitempty" json:"folder,omitempty"`), with exported
  constants `LayoutFolder = "folder"` and `LayoutPage = "page"`.
  `DefaultConfig()` sets `Layout` to `folder`
- [x] `normalizeSync` backfills an empty `layout` to `folder`, folds it to
  lower case, and trims `folder`. It never rejects
- [x] `validateSync`, still only when enabled:
  - `layout` is `folder` or `page`;
  - in the page layout, `parent` is required and `folder` must be empty;
  - in the folder layout, `parent` is optional, and `folder`, when set,
    has no control characters, no leading or trailing space, and at most
    255 characters
- [x] `sync_test.go`: one case per rule; the Phase A shape
  (`parent` set, no `layout`) now loads as the folder layout with
  `parent` as the folder's parent page; a dormant block with every field
  wrong still loads; `ParseBytes` and both `Load` paths agree
- [x] `json_test.go`: `TestConfigJSON_MarshaledShape` gains `layout` and
  `folder`; `TestJSONTags_MirrorYAML` passes unedited
- [x] `docz_yaml.tmpl`: the disabled block gains `layout: folder`, a
  commented `folder:` line saying it defaults to the repository's name, and
  `parent` commented as optional; the comment lists the six scopes from
  DESIGN-0021 Background. The parity `sync` normaliser already drops the
  block; `just parity` proves it still does

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `just test` and `just parity` pass
- An enabled block in the page layout without `parent`, or with `folder`,
  fails `Validate` naming the field; the same block dormant loads
- `docz init` writes `layout: folder`, and `docz config` round-trips it

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 2: Reading through `fs.FS`

After this phase `Export` plans from an `fs.FS` and never from `rp.List`
or `os.ReadFile`. The CLI's output is byte-identical, which the parity
test proves before anything else changes (INV-0020 decision 1's
experiment, DESIGN-0021 question 4).

<!--docz:tasks:start-->
#### Tasks

- [x] Before changing `plan.go`, add `plan_parity_test.go` capturing
  today's plan over this repository's `docs/` and `.docz.yaml` and over
  the Phase A corpus fixtures: every item's key, title, source, parent
  index, and a hash of its bytes, plus `keys`, `titles`, and `targets`.
  Commit it as a golden (`testdata/plan/*.golden.json`, `-update`) while
  `rp.List` still builds the plan. The golden is over a repository
  built at test time from the corpus fixtures (every `.orig.md` as a
  numbered document, runbook enabled, an `api:` landing page and
  additional doc), for a full run and runs narrowed by type and by id;
  this repository's own `docs/` is covered by the `--dry-run --out`
  comparison in the success criteria, since a golden over live docs
  would move with every edit
- [ ] `ExportOptions.FS fs.FS`, documented as the repository's files rooted
  at the repository root with slash paths. `Export` uses
  `os.DirFS(rp.Root)` when it is nil
- [ ] `plan.go`: `listDocs` walks each type directory with `fs.ReadDir`,
  keeps names `document.IsDoczFile` accepts, parses with
  `document.ParseFrontmatter` (a file with no frontmatter is skipped, as
  `repo.Scan` skips it), and builds the same `repo.Entry` values with
  repo-relative slash paths. Sort order matches `repo.List`'s
- [ ] `selectDocs` resolves `opts.IDs` against the listed entries instead
  of `rp.Find`: the id prefix picks the type as `repo.Find` does, a miss is
  `*repo.NotFoundError`, and an id with no `-` is `*repo.UnknownTypeError`
- [ ] `typePage`, `pageItem`, and `parentItem` read with `fs.ReadFile`.
  `excluded` works on slash paths only and no longer looks at `rp.Root`
- [ ] The golden from the first task passes unchanged through
  `os.DirFS`, and again through an `fstest.MapFS` built from the same
  files (`TestPlanFromMapFS`)
- [ ] `export_test.go`: an `Export` with `rp.Root` pointing nowhere and
  `FS` set succeeds, proving nothing reads the disk
- [ ] `just lint`, `just test`, and the existing `cmd/export_test.go` pass
  untouched

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- The plan golden is identical before and after the change, through
  `os.DirFS` and through `fstest.MapFS`
- `grep -n 'os\.ReadFile\|rp\.List\|rp\.Find' pkg/export/confluence/*.go`
  finds nothing outside tests
- `docz export confluence --dry-run --out` over this repository writes the
  same bodies as on `main`

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 3: The folder layout, ownership, recorded ids, and `Overwrite`

After this phase the package builds DESIGN-0021 §2's tree: a folder per
repository, a home page, prefixed titles, and properties that name their
repository. It can find a page by a recorded id and overwrite an edited
one when asked. Everything is proven against the fake client.

<!--docz:tasks:start-->
#### Tasks

- [ ] `client.go`: `Node` (a `Page` with `Type` `page` or `folder`),
  `Folder`, `NewFolder`, and the methods `Page`, `Body`, `Folder`,
  `CreateFolder`, `SpaceHome`; `Children` now returns `[]Node`.
  `Property`/`SetProperty` take a `Target{Type, ID}` so they address a
  folder or a page. Doc comments state the `(nil, nil)` rule for each
- [ ] `httpclient.go`: the new methods on the v2 endpoints in
  DESIGN-0021 §3 (`GET /pages/{id}`, `GET /pages/{id}?body-format=storage`,
  `GET /folders/{id}`, `POST /folders`, `GET /spaces/{id}`,
  `GET /{pages,folders}/{id}/direct-children` with every page of results
  followed, and the folder property endpoints). A 404 is `(nil, nil)`.
  `httpclient_test.go` covers each against `httptest`, including
  pagination and a 404
- [ ] `fake_test.go`: the fake keeps folders, their properties, and a
  per-space title index for pages and one for folders, and rejects a
  duplicate title in either with Confluence's 400 shape
- [ ] `ExportOptions.Repository`, `Pages map[string]string`,
  `Folder string`, `Overwrite bool`. `Force` implies `Overwrite`
- [ ] `pageProperty` gains `Repo string` (`json:"repo,omitempty"`). A new
  `owned(repo)` method: true when `Repo` is empty or equal. A
  `folderProperty{Repo, Docz}` for the folder
- [ ] The plan, in the folder layout: the folder title is `sync.Folder`,
  else the last element of `Repository`, else the base of the root; every
  composed and document title starts with the folder name and a colon
  (`docz: ADR-0001: …`); the home page (key `docz:parent`) is titled with the folder
  name alone; `Archive` becomes `<folder>: Archive`. The page layout
  keeps Phase A's titles exactly, which the Phase 2 golden proves once
  its fixtures set `layout: page`
- [ ] `export.go`: in the folder layout, resolve the folder first:
  `opts.Folder` through `Client.Folder`, else a `direct-children` search
  of the `parent` page (found by title) or of `SpaceHome`, else
  `CreateFolder`. A folder whose property is missing or names another
  repository is a `ConfigError` naming the title and
  `sync.confluence.folder`; a create that 400s on the title is a
  `ConfigError` too. The folder's property is written on create.
  `Report.Folder` is set
- [ ] `HTTPClient.FindPage` gains a `status` argument through an unexported
  helper, and `create` handles a 400 on the title: it looks the title up
  among archived pages and fails the page with
  `title held by archived page <id>; restore, rename, or delete it in
  Confluence`. A fake-client test covers it, since the fake keeps archived
  titles reserved as Confluence does
- [ ] `reconcile.go`: a key in `opts.Pages` is looked up with
  `Client.Page` first, falling back to `FindPage` when it is gone or in
  another space. A page whose property is not `owned` is `Skipped`
  "belongs to <repo>" whatever `Force` says. `decide` takes `Overwrite`:
  a moved version under `Overwrite` is `Updated` with `Edited` set; without
  it, `Skipped` with `Edited` set. The unchanged check compares the title
  as well
- [ ] `orphans.go`: containers are the folder and each type page, listed
  with `Children` (`direct-children`); only `page` nodes are candidates;
  a page not `owned` is never moved
- [ ] `PageResult` gains `Key`, `Hash`, `Edited *Edit`; `Report` gains
  `Folder *Node`. JSON tags follow the existing snake_case
- [ ] `export_test.go`, table-driven over the fake: the folder tree and
  titles; a folder found under the parent page and under the space home;
  another repository's folder (`ConfigError`); a folder title taken
  elsewhere (`ConfigError`); a page found by recorded id and renamed; a
  recorded id that is gone; `Overwrite` vs `Force` vs neither on an edited
  page, a property-less page, and another repository's page; orphans in a
  folder; two repositories sharing the fake space with no collision; and
  one repository alone in its space still prefixed
- [ ] `live_test.go` (`//go:build live`): the round trip gains a folder:
  create it, set and read its property, create a page in it, list it with
  `direct-children`, archive a page inside it, and delete everything it
  made
- [ ] `test/consumer/consumer_v2_export_test.go`: an `Export` with an
  `fstest.MapFS`, `Repository`, and `Overwrite`, against the consumer's own
  `Client`, asserting the folder and the prefixed titles

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `just test`, `just test-consumer`, and `just lint` pass
- The fake-client tests show two repositories exporting into one space
  with no title collision, and neither able to touch the other's folder
  or pages, under `Force` included
- `just export-live` passes against the scratch site with the new token

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 4: The CLI in the folder layout

After this phase `docz export confluence` builds the same tree the server
will, warns about edited pages the way INV-0020 decision 9 specifies, and
this repository exports to the scratch site in the folder layout.

<!--docz:tasks:start-->
#### Tasks

- [ ] `cmd/export.go`: `ExportOptions.Repository` from
  `GitResolver.RemoteURL` (`https://github.com/<owner>/<name>` →
  `owner/name`), empty when there is no GitHub remote
- [ ] For each result with `Edited` set and action `Skipped`, print to
  stderr, whatever the log level:
  `WARNING: <id> was edited in Confluence (v<N>, expected v<M>); not overwritten, use --force`,
  where `<id>` is the document id or, for a composed page, its title. The
  stdout line and the exit codes do not change
- [ ] A skipped page owned by another repository prints
  `WARNING: <title> belongs to <repo>; not written`
- [ ] The `--format json` report carries `key`, `hash`, `edited`, and
  `folder`
- [ ] `cmd/export_test.go`: the warning text for an edited page and for a
  foreign page, on stderr only; the repository name from SSH and HTTPS
  remotes and with no remote; `--strict` still exits 1 on a skip
- [x] Before the first folder-layout run, the Phase A tree on the scratch
  site (the `docz` page and its 76 descendants) is moved out of the way:
  archived in the Confluence UI on 2026-10-07
- [x] The archived `docz` page (65860) still held its title, which the
  home page needs (DESIGN-0021 §3 amendment, tested 2026-10-07). It was
  deleted on 2026-10-07, and a page titled `docz` was then created and
  removed to confirm the title is free
- [ ] `.docz.yaml`: `layout: folder`, `parent:` removed, so the folder sits
  at the top of `DOCZ`
- [ ] Live: `docz export confluence --dry-run`, then a real run, then a
  rerun reporting every page unchanged. Edit one page in the browser,
  rerun, see the `WARNING`, rerun with `--force`. Record the counts and the
  folder URL in this document

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- The scratch site's `DOCZ` space has a `docz` folder holding the `docz`
  home page, every type page, every document, and the additional docs,
  all titled `docz: …`
- A second run reports every page unchanged
- An edited page produces exactly the `WARNING` line DESIGN-0021 §8 gives,
  and `--force` overwrites it

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 5: Keeping inline comments (#158)

After this phase an update carries every inline comment whose text is still
present, reports the ones it cannot, and we know whether a comment bumps a
page's version (DESIGN-0021 question 11).

<!--docz:tasks:start-->
#### Tasks

- [ ] `comments.go`: `collectMarkers(body []byte) []marker` reads the
  current body's `ac:inline-comment-marker` elements with their `ac:ref`
  and text content, using the token reader `wellformed.go` already uses
- [ ] `carryMarkers(rendered []byte, markers []marker) (out []byte, kept int, lost []string)`:
  for each marker, find its text in character data only (not in a tag,
  an attribute, an `ac:parameter`, or a `plain-text-body` CDATA), outside
  markers already placed; wrap it when it occurs exactly once inside one
  text node; else add it to `lost`. The input is never modified
- [ ] `reconcile.go`: on `Updated` only, `Client.Body` the current page,
  carry the markers, check the result well-formed, and write it. The
  property's `hash` stays the hash before markers. `PageResult.Comments`
  is set. A body read that fails fails the page, as any request does
- [ ] `testdata/comments/`: pairs of a current body with markers and a new
  render, with `.golden` outputs, covering a kept comment, a moved
  paragraph, duplicated text, deleted text, text inside a code macro, text
  spanning `<strong>`, two markers in one paragraph, and an entity in the
  text (`&amp;`)
- [ ] `FuzzCarryMarkers`: the output is always well-formed, and stripping
  the markers it added gives back the render byte for byte
- [ ] CLI: one `WARNING: <id>: an inline comment on "<text>" lost its anchor`
  line per lost comment
- [ ] Live: on a scratch page, add an inline comment (Open Question 3), read the page's
  version, change the source markdown around the commented text, export
  with `--force`, and confirm the comment still shows anchored in the
  browser. Whether a comment bumps the version was settled on 2026-10-07
  ahead of this phase: it does not (DESIGN-0021 §4 amendment). The live
  test asserts it stays that way

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- The comment corpus goldens pass and the fuzz target runs 60 seconds clean
- A live update keeps an inline comment anchored on unchanged text
- DESIGN-0021 records whether an inline comment bumps the page version

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 6: docz-api storage and configuration

After this phase docz-api has somewhere to record exports and knows whether
it may run them. Nothing is enqueued yet.

<!--docz:tasks:start-->
#### Tasks

- [ ] `internal/store/migrations/<timestamp>_add_confluence.sql`: the two
  tables and the index from DESIGN-0021 Data Model, with a `-- +goose Down`
  that drops them
- [ ] `internal/store/queries/confluence.sql` and `just api generate`:
  `UpsertConfluenceSync`, `GetConfluenceSync`, `UpsertConfluencePage`,
  `ListConfluencePages`, `ListConfluencePageIDs` (key → page id, for
  `ExportOptions.Pages`). `ListDocumentsByType` and `GetDocumentByID` gain a
  `LEFT JOIN confluence_pages` on `(repo_id, doc_id)` returning
  `confluence_url` as `''` when absent. `just api generate-check` passes
- [ ] `internal/store`: `Store` methods over the queries, and
  `ExportInputs(ctx, repoID) (ExportInputs, error)` reading the repo row,
  its documents with `raw_md`, its pages, its sync row, and its page ids in
  one `REPEATABLE READ` read-only transaction
- [ ] `internal/config`: `ConfluenceConfig{Site string, Email string,
  APIToken Secret, Spaces []string}` from `CONFLUENCE_SITE`,
  `CONFLUENCE_EMAIL`, `CONFLUENCE_API_TOKEN`, `CONFLUENCE_SPACES`
  (comma-separated, trimmed, empty entries dropped). `Enabled()` is true
  when the token is set; then `Site` (an `https` URL with a host and no
  path), `Email`, and at least one space are required, and `Load` reports
  each missing one in its single `ErrInvalidConfig`. `Allowed(site, space)`
  compares the site with its trailing `/` trimmed and the space key exactly
- [ ] `config_test.go`: unset is disabled; each missing field; the token
  never appears in `%v`, `%+v`, or a slog line
- [ ] `cmd/docz-api/confluence.go`: `checkConfluenceCredentials` at
  startup when enabled: resolve the cloud id, then
  `GET /spaces?keys=<spaces>`. A 401 fails startup; anything else,
  including a 403 or a space that is not found, logs a warning naming it
  and continues. A unit test with an `httptest` server covers each branch
- [ ] Store integration tests: the migration up and down, the upserts, the
  `confluence_url` join on both document reads, and `ExportInputs` seeing a
  consistent snapshot while a concurrent reconcile commits

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `just api test` and `just api test-integration` pass
- docz-api starts with no `CONFLUENCE_*` set exactly as it does today, and
  refuses to start with a token that answers 401

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 7: The export task

After this phase a push that touches a repository's docs ingests it and
then exports it, recording what happened. The e2e test proves it against
an in-memory Confluence.

<!--docz:tasks:start-->
#### Tasks

- [ ] `internal/queue/export.go`: `TaskTypeExport = "export:confluence"`,
  queue `export`, `ExportJob{RepoID, Owner, Name, Reason, TraceParent,
  TraceState}`, `EnqueueExport` with task id `export:owner/name`,
  `MaxRetry(5)`, and no `ProcessIn`
- [ ] Generalise `resolveTaskIDConflict` and `classifyConflict` over a
  `{taskType, queue, id}` value so ingest and export share them. Every
  existing `client_test.go` case passes unedited, and each gains an export
  twin
- [ ] `worker.go`: `Queues: {ingest: 2, export: 1}`; an `Exporter`
  interface (`Run(ctx, repoID int64) (export.Result, error)`) registered
  for `TaskTypeExport` when non-nil; a `queue.export` consumer span from
  the job's trace fields; `isFailure` unchanged
- [ ] `internal/export/service.go`: `Service.Run(ctx, repoID)` per
  DESIGN-0021 §5 steps 1 to 7: `ExportInputs`; decode `config_snapshot`
  into `doczcfg.Config`; record `disabled`, or `refused` (site, space, or
  `layout: page`) and return nil; record `running`; build the
  `fstest.MapFS`; run `confluence.Export`; record the report; loop on a
  moved head SHA at most three times
- [ ] `internal/export/files.go`: the `fstest.MapFS` from rows; one README
  per enabled type from `index.Splice(nil, header, index.GenerateTable(...))`
  with the header from the embedded tier only (see Open Question 5); the
  landing page from `repos.index_md` at `api_landing_page`; `repo_pages` at
  their `repo_path`
- [ ] `internal/export/links.go`: the resolver returning
  `https://github.com/<owner>/<name>/blob/<default_branch>/<path>` for any
  repository path, with no existence check
- [ ] `internal/export/retry.go`: `classify(err, report)` per DESIGN-0021
  §5's table, returning the status and whether to wrap
  `asynq.SkipRetry`
- [ ] One `confluence.HTTPClient` per process, built from
  `ConfluenceConfig` in `cmd/docz-api` and shared by every job (see Open
  Question 6)
- [ ] `internal/ingest/service.go`: an optional `Exporter` (the queue's
  `EnqueueExport`) beside `Indexer`; after a successful reconcile with
  `cfg.Sync.Confluence.Enabled`, enqueue `{RepoID, Owner, Name, Reason}`
  and log a failure at error without failing the ingest.
  `cmd/docz-api/runner.go` passes it only when `ConfluenceConfig.Enabled()`
- [ ] `cmd/docz-api/main.go`: wire the export service into the worker;
  add `-export owner/name`, which looks up the repository, enqueues one
  export with reason `manual`, and exits, mirroring `-onboard`
- [ ] `internal/telemetry/metrics.go`: `ObserveExport(reason, status, d)`
  for `docz_api_export_jobs_total` and
  `docz_api_export_job_duration_seconds` (buckets to 600s), and
  `docz_api_export_pages_total{action}`
- [ ] Logs: `export job complete` with counts and the folder; a warn line
  per refused repository, per foreign page, and per lost comment; the
  failure path through `logIngestFailure`'s twin with the task id,
  `retried`, `max_retry`, repository, and reason
- [ ] Unit tests for `internal/export` with a fake client and a fake
  store: each status; the README bytes for a fixture type; the resolver;
  each row of the retry table; the head-SHA loop stopping at three
- [ ] `internal/e2e/confluence_integration_test.go` (real Postgres, fake
  fetcher, an `httptest` Confluence that keeps pages, folders, and
  properties in memory): onboard a fixture repository, run the export,
  assert the folder tree and the recorded rows; rename a document, ingest
  again, assert the page was renamed and not archived; remove a document,
  assert it was archived; disable the block, assert `disabled` and no
  writes

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `just api test` and `just api test-integration` pass, the e2e test included
- An ingest of an enabled repository is followed by exactly one export,
  and a burst of five pushes by one
- A failed Confluence request is retried; a 401 is not; both are logged
  with their cause and recorded in `confluence_syncs`

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 8: The API and docz-site

After this phase a reader can follow a document to its Confluence page, and
the API reports each repository's last export.

<!--docz:tasks:start-->
#### Tasks

- [ ] `internal/httpapi/dto.go`: `documentDTO.ConfluenceURL`
  (`json:"confluence_url"`, `""` when none) on both document responses
- [ ] `internal/httpapi`: `GET /api/v1/repos/{owner}/{name}/confluence`
  returning DESIGN-0021 §7's shape, behind `resolveRepo`'s existence
  hiding. A repository with no row is `never` with empty fields and
  `pages: []`; with the server's export off it is `disabled` with the
  reason `disabled on this server`
- [ ] `api/openapi.yaml`: the `confluence_url` property, the
  `ConfluenceSync` and `ConfluencePage` schemas with
  `additionalProperties: false`, the operation, and `info.version`
  `1.5.0` → `1.6.0`. `just api lint-openapi` scores 100
- [ ] `openapi_contract_test.go`: the new route for a synced repository, a
  never-synced one, and a document with and without a URL
- [ ] `ui/`: `just ui gen-api`; in `src/routes/doc.tsx`, a "View in
  Confluence" link opening in a new tab when `confluence_url` is set; the
  MSW fixtures gain the field; `doc.test.tsx` covers both cases.
  `just ui ci` passes

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- The contract test and `just ui ci` pass
- The link renders for a document with a page and is absent for one
  without

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:phase:start-->
### Phase 9: Deployment, documentation, live run, and release

After this phase an operator can turn the export on from the chart, the
server has run it live for repositories sharing a space and for one alone
in its own, and `v2.0.0-beta.8` ships it.

<!--docz:tasks:start-->
#### Tasks

- [ ] `charts/docz`: `api.confluence.{site, email, spaces}` in
  `values.yaml` and the schema; a `confluence-api-token` key in
  `api-secret.yaml` (and accepted through `secrets.existingSecret`); the
  `CONFLUENCE_*` env block in `api-deployment.yaml` rendered only when
  `api.confluence.site` is set, with the token key `required` then.
  `tests/api/deployment_env_test.yaml` and `secret_test.yaml` cover set
  and unset. Chart `version` 0.2.1 → 0.3.0, `appVersion` to the beta,
  `just chart docs` regenerated
- [ ] `DoczAPIExportFailures` in `api-prometheusrule.yaml`,
  `tests/api/prometheusrule_test.yaml`, and
  `contrib/prometheus/alerts.yaml`; `just api lint-alerts` passes;
  `contrib/README.md` lists the three new metrics
- [ ] A new runbook, "Enable Confluence export on docz-api" (see Open
  Question 7): creating the token with its six scopes, the four
  variables and chart values, adding a space to the allow-list, reading
  `GET …/confluence`, and what each status means. Its Last Verified row
  comes from the live run below
- [ ] README, CLAUDE.md, and DEVELOPMENT.md: the folder layout, the new
  keys, the server's export, its configuration, and the `WARNING` lines;
  `pkg/export/confluence`'s package doc
- [ ] DESIGN-0021 to Implemented, with any amendment the live runs called
  for
- [ ] Live server run: `deferred - human required` to provision. Run
  docz-api locally against the scratch site with this repository and a
  second one sharing `DOCZ`, and a third repository naming a second space
  on the allow-list. Then a fourth naming a space not on the list,
  expecting `refused`. Record each repository's `GET …/confluence` in this
  document
- [ ] `just ci` green; the PR merged by a person; `just release
  v2.0.0-beta.8` from the merge commit: `deferred - human required`

<!--docz:tasks:end-->

<!--docz:criteria:start-->
#### Success Criteria

- `just chart lint`, `just chart unittest`, and `just ci` pass
- The live run shows two repositories' folders in one space, a third in
  its own space, and a refused fourth, with nothing written for it
- `v2.0.0-beta.8` is tagged, and its images and chart are published

<!--docz:criteria:end-->
<!--docz:phase:end-->

---

<!--docz:file-changes:start-->
## File Changes

| File | Action | Description |
| ---- | ------ | ----------- |
| `pkg/doczcore/config/sync.go`, `sync_test.go`, `json_test.go` | Modify | `layout`, `folder`, and their rules |
| `pkg/doczcore/doctemplate/templates/docz_yaml.tmpl` | Modify | the generated block's new keys and scopes |
| `pkg/export/confluence/plan.go` | Modify | `fs.FS` listing and reading; the folder layout and prefixes |
| `pkg/export/confluence/export.go` | Modify | new options and report fields; the folder resolution |
| `pkg/export/confluence/reconcile.go` | Modify | lookup by recorded id, ownership, `Overwrite`, `Edited`, comment carrying |
| `pkg/export/confluence/orphans.go` | Modify | folder containers, `direct-children`, ownership |
| `pkg/export/confluence/client.go`, `httpclient.go` | Modify | `Node`, `Folder`, and the new methods |
| `pkg/export/confluence/comments.go` | Create | collecting and carrying inline-comment markers |
| `pkg/export/confluence/confluencetest/` | Create | the in-memory client and `httptest` handler (Open Question 4) |
| `pkg/export/confluence/testdata/plan/`, `testdata/comments/` | Create | the plan and comment goldens |
| `pkg/export/confluence/live_test.go` | Modify | folders in the live round trip |
| `test/consumer/consumer_v2_export_test.go` | Modify | `FS`, `Repository`, `Overwrite`, the folder |
| `cmd/export.go`, `cmd/export_test.go` | Modify | the repository name and the `WARNING` lines |
| `.docz.yaml` | Modify | `layout: folder`, no `parent` |
| `internal/store/migrations/*_add_confluence.sql` | Create | `confluence_syncs`, `confluence_pages` |
| `internal/store/queries/confluence.sql`, `documents.sql` | Create, Modify | the new queries; the `confluence_url` join |
| `internal/store/*.go` | Modify | the methods and `ExportInputs` |
| `internal/config/config.go` | Modify | `ConfluenceConfig` |
| `internal/queue/export.go`, `client.go`, `worker.go` | Create, Modify | the task, the shared conflict handling, the second queue |
| `internal/export/` | Create | the service, the file system, the resolver, the retry classification |
| `internal/ingest/service.go` | Modify | the optional `Exporter` |
| `internal/telemetry/metrics.go` | Modify | the export metrics |
| `internal/httpapi/dto.go`, `handler.go`, a new `confluence.go` | Modify, Create | the field and the endpoint |
| `internal/e2e/confluence_integration_test.go` | Create | the end-to-end test |
| `cmd/docz-api/main.go`, `runner.go`, `confluence.go` | Modify, Create | wiring, `-export`, the startup check |
| `api/openapi.yaml` | Modify | the field, the schemas, the operation, `1.6.0` |
| `ui/src/routes/doc.tsx`, `doc.test.tsx`, `src/mocks/` | Modify | the link |
| `charts/docz/values.yaml`, `values.schema.json`, `templates/api-*.yaml`, `tests/api/*`, `Chart.yaml`, `README.md` | Modify | values, Secret key, env, alert, 0.3.0 |
| `contrib/prometheus/alerts.yaml`, `contrib/README.md` | Modify | the alert and metrics |
| `docs/runbook/0003-*.md` | Create | enabling the export on a deployment |
| `README.md`, `CLAUDE.md`, `DEVELOPMENT.md` | Modify | the folder layout and the server export |
| `docs/design/0021-*.md` | Modify | status and amendments |

<!--docz:file-changes:end-->

<!--docz:testing:start-->
## Testing Plan

- [ ] The plan golden over this repository and the Phase A corpus, captured
  before Phase 2 changes anything, passes through `os.DirFS` and
  `fstest.MapFS`
- [ ] Table-driven `Export` tests over the in-memory client for the folder
  layout, ownership, recorded ids, and `Overwrite`, including two
  repositories in one space
- [ ] `httptest` tests for every new `HTTPClient` method, with pagination
  and 404s
- [ ] The comment corpus goldens and `FuzzCarryMarkers`
- [ ] `cmd/export_test.go` for the `WARNING` lines and the repository name
- [ ] The consumer module exercises the new options from outside
- [ ] `internal/config`, `internal/queue`, and `internal/export` unit tests,
  including the retry classification table
- [ ] Store integration tests for the migration, the queries, the join, and
  the read transaction
- [ ] The e2e integration test: export, rename, remove, disable
- [ ] The OpenAPI contract test and `doc.test.tsx`
- [ ] helm-unittest for the env block, the Secret key, and the alert
- [ ] Live: `just export-live` with folders; the CLI run of Phase 4; the
  comment check of Phase 5; the server run of Phase 9

<!--docz:testing:end-->

<!--docz:dependencies:start-->
## Dependencies

- DESIGN-0021, all questions resolved, Approved with this plan
- The scoped token in `~/.config/docz/atlassian.env` with the six scopes,
  confirmed on 2026-10-06 (INV-0020 Observation 9), plus
  `read:comment:confluence` and `write:comment:confluence` for Phase 5's
  live check (Open Question 3)
- The scratch site's Phase A tree moved aside by hand before Phase 4's
  first real run
- A second and third GitHub repository with `.docz.yaml`, installed on the
  development GitHub App, and a second space on the scratch site, for
  Phase 9's live run
- No new Go module: `testing/fstest` is in the standard library, and
  goldmark is already allowed under `pkg/export/`

<!--docz:dependencies:end-->

<!--docz:open-questions:start-->
## Open Questions

### 1. How is the work split into pull requests?

- a. **One PR per phase, each branched from `main` after the previous one
  merges, all labelled `dont-release`.** Each PR is small enough to review
  and lands green on its own, and the stacked-PR auto-close problem never
  comes up because nothing is stacked.
- b. Two PRs: the library and the CLI (Phases 1 to 5), then the server
  (Phases 6 to 9). Fewer merges, larger reviews.
- c. One PR on this branch for everything, as IMPL-0023 did. One review at
  the end, the largest diff.
- d. Other.

> **Resolved 2026-10-07: (d).** Every phase on `feat/docz-api-confluence-export`,
> with the INV, DESIGN, and IMPL, in one PR, as IMPL-0023 did.

### 2. How many betas does this ship in?

- a. **One, `v2.0.0-beta.8`, after Phase 9.** DESIGN-0021's rollout plans
  one beta, and the CLI's folder layout is only interesting next to the
  server that shares it.
- b. Two: `v2.0.0-beta.8` after Phase 5 for the CLI's folder layout and
  comment carrying, and `v2.0.0-beta.9` after Phase 9 for the server.
- c. Other.

> **Resolved 2026-10-07: (a).**

### 3. How does Phase 5's live check create an inline comment?

- a. **Through the v2 API, `POST /inline-comments` with a
  `textSelection`, inside `live_test.go`.** The check is repeatable and
  needs no browser, at the cost of two more scopes on the token,
  `read:comment:confluence` and `write:comment:confluence`.
- b. By hand in the browser, with the test reading the result. No new
  scopes, but the check needs a person every time it runs.
- c. Other.

> **Resolved 2026-10-07: (a).**

### 4. Where does the in-memory Confluence used by tests live?

- a. **A public `pkg/export/confluence/confluencetest` package: an
  in-memory `Client` and an `http.Handler` over the same state.** The
  package tests, the consumer module, and docz-api's e2e test use one fake
  that enforces Confluence's title rules, instead of three that can drift.
  It is EXPERIMENTAL like its parent.
- b. Keep `fake_test.go` private and write a second fake inside
  `internal/e2e`.
- c. Other.

> **Resolved 2026-10-07: (a).**

### 5. How does the server get a type's index header without reading its own disk?

- a. **Add `doctemplate.EmbeddedIndexHeader(docType, data)`, the embedded
  tiers of `ResolveIndexHeader` only.** It is a small addition to an
  EXPERIMENTAL package, and the server can never pick up a stray
  `templates/index_<type>.md` from its working directory.
- b. Call `ResolveIndexHeader` with a docs directory that cannot exist on
  the server. No library change, but it relies on a path never existing.
- c. Fetch `templates/index_<type>.md` at ingest and store it, so header
  overrides are honoured. More fetching and a column for a rare case
  INV-0020 decision 2 chose not to cover.
- d. Other.

> **Resolved 2026-10-07: (a).**

### 6. How long does a Confluence client live in docz-api?

- a. **One `HTTPClient` per process, built at startup and shared by every
  job.** One site per deployment means one cloud id, resolved once and
  memoised, and one place for the 429 backoff to apply.
- b. A new client per job, as ingest builds a GitHub client per job.
  Simpler lifetime, but every job resolves the cloud id again.
- c. Other.

> **Resolved 2026-10-07: (a).**

### 7. Where is enabling the export documented for operators?

- a. **A new runbook, "Enable Confluence export on docz-api", in
  `docs/runbook/`.** It is a procedure an operator follows with
  verification steps, which is what the runbook type is for, and it gets a
  Last Verified row from Phase 9's live run.
- b. A section in `charts/docz/README.md` and `api/README.md`. Close to
  the values, but no verification steps.
- c. Other.

> **Resolved 2026-10-07: (a).**

### 8. Does Phase 2 keep `rp.List` as a fallback?

- a. **No. Once the golden passes through `os.DirFS`, the `rp.List` path is
  deleted in the same phase.** DESIGN-0021 question 4 chose one planning
  path; keeping two would be the drift it avoided.
- b. Keep `rp.List` when `FS` is nil until Phase 4's live run, then delete
  it.
- c. Other.

> **Resolved 2026-10-07: (a).**

<!--docz:open-questions:end-->

<!--docz:references:start-->
## References

- [DESIGN-0021](../design/0021-confluence-export-from-docz-api-per-repository-folders-in.md):
  the design this plan implements
- [INV-0020](../investigation/0020-docz-api-confluence-export-running-confluenceexport-without-a.md):
  the investigation behind it
- [DESIGN-0020](../design/0020-confluence-export-docz-documents-as-confluence-cloud-pages.md)
  and [IMPL-0023](0023-confluence-export-the-package-and-the-cli-design-0020-phase-a.md):
  Phase A
- Issues [#154](https://github.com/donaldgifford/docz/issues/154) and
  [#158](https://github.com/donaldgifford/docz/issues/158)
- [Confluence Cloud REST API v2](https://developer.atlassian.com/cloud/confluence/rest/v2/intro/)

<!--docz:references:end-->
