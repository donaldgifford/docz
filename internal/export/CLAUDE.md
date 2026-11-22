# CLAUDE.md: `internal/export`

Loaded when a session works under this directory. The repository's map is the root `CLAUDE.md`.

## Confluence export (DESIGN-0021 / IMPL-0024)

docz-api runs `pkg/export/confluence` itself after each ingest, in the
folder layout, and records what it wrote.

- **Storage** (migration `20261007000000_add_confluence`):
  `confluence_syncs` (the last run per repo: folder, status, error, counts)
  and `confluence_pages` (one row per property key: doc id, page id, URL,
  version, hash, action). A failed page's upsert keeps the previous page id
  and URL (`COALESCE(NULLIF(...), ...)`). Both document reads LEFT JOIN
  `confluence_pages` for `confluence_url` (`''` when absent), so
  `GetDocumentByID` returns `GetDocumentByIDRow{Document, ConfluenceUrl}`
  via `sqlc.embed`. `ExportInputs` reads repo, documents with `raw_md`,
  pages, sync, and recorded page ids in one `REPEATABLE READ` read-only
  transaction, so an export never sees half a reconcile;
  `RecordConfluenceExport` writes a run in one transaction.
- **Config** (`internal/config`): `CONFLUENCE_SITE`, `CONFLUENCE_EMAIL`,
  `CONFLUENCE_API_TOKEN` (a `Secret`), `CONFLUENCE_SPACES`. Enabled iff the
  token is set; then the rest are required. `Allowed(site, space)` is the
  allow-list a repository's `.docz.yaml` cannot widen.
- **Startup** (`cmd/docz-api/confluence.go`): one `confluence.HTTPClient`
  per process; `checkConfluenceCredentials` resolves the cloud id and looks
  the spaces up with `HTTPClient.Spaces`. Only a 401 fails startup, the
  same asymmetry as the GitHub App self-check.
- **The task** (`internal/queue/export.go`): `export:confluence` on queue
  `export`, task id `export:owner/name`, `MaxRetry(5)`, no `ProcessIn`
  (ingest already debounced). `resolveTaskIDConflict` is shared with ingest
  through a `taskSpec{taskType, queue, kind}` and a `queuedJob` interface,
  so a finished export's id is cleared exactly as ingest's is. The worker
  weights `{ingest: 2, export: 1}`, registers `handleExport` only when given
  an `Exporter`, and its `ErrorHandler` is `logTaskFailure`, which sends an
  export attempt to `logExportFailure`.
- **Enqueueing**: `ingest.Service.WithExporter(queueClient)` enqueues an
  export (reason `ingest`) after every successful ingest of a repo with
  `sync.confluence.enabled`, changed or not; a failed enqueue is logged and
  the ingest succeeds. `ingestRunner.exporter` is set only when the server
  has a credential. `-export owner/name` enqueues one with reason `manual`
  and refuses when the export is off (no worker would run it).
- **`internal/export.Service.Run`**: `ExportInputs` → decode
  `config_snapshot` (the marshalled, normalised config) → `disabled`, or
  `refused` for a site/space off the allow-list or `layout: page` (recorded,
  warned, not an error) → record `running` → `files()` builds an
  `fstest.MapFS` (documents at their paths; a README per enabled type from
  `index.Splice(nil, doctemplate.EmbeddedIndexHeader(...), GenerateTable)`,
  pinned equal to what `docz update` writes; `index_md` at the landing page;
  `repo_pages` at `repo_path`) → `confluence.Export` with `Overwrite`, the
  recorded `Pages` and `Folder`, and `blobResolver` (GitHub blob URL with
  no existence check, except that `missingDocument` leaves a link to a
  document file in an enabled type's directory that was not ingested
  unresolved, since that is a removed document and its blob URL 404s) →
  record the report on an uncancelled context. The repo
  root it hands `repo.Repo` names a directory that does not exist, so a
  stray disk read fails rather than reading the server's files. It runs
  again while an ingest moved `last_synced_sha` meanwhile, three runs at
  most. `classify` is DESIGN-0021 §5's table: config and auth errors are
  `failed` with `asynq.SkipRetry`, a page failing on 429/5xx/network is
  `partial` and retried, a malformed render, a taken title, or another 4xx
  is not, and a cancelled context is `failed` and re-queued.
- **API** (spec 1.6.0): both document responses carry `confluence_url`
  (`''` when the last export left no page; a failed page keeps its previous
  URL). `GET /api/v1/repos/{owner}/{name}/confluence` serves the last run
  (`internal/httpapi/confluence.go`): `never` with empty fields and
  `pages: []` for a repo with no row, and `disabled` / `disabled on this
  server` when `Handler.WithConfluenceExport(false)`, which `runServer`
  sets from whether a Confluence client exists. `confluence_pages` carries
  `source` and `edited_expected` beyond DESIGN-0021's Data Model because
  §7's response names them. docz-site's reader shows "View in Confluence"
  when the URL is set.
- **Telemetry**: `docz_api_export_jobs_total{reason,status}`,
  `docz_api_export_job_duration_seconds` (to 600s),
  `docz_api_export_pages_total{action}`; spans `queue.export` →
  `export.load`, `export.run` (an event per Confluence request via
  `confluence.Hooks.Request`), `export.record`.
- **Tests** use `pkg/export/confluence/confluencetest`, the public in-memory
  site that is both a `confluence.Client` and, through `Handler` /
  `HTTPClient(tb, email, token)`, the v2 endpoints the real `HTTPClient`
  calls, with `RequireAuth` and `FailOn`. The package's own tests keep
  `fake_test.go`: in-package tests importing `confluencetest` would cycle.
  `internal/e2e/confluence_integration_test.go` runs ingest → export →
  rename → removal → disable over real Postgres and the HTTP fake. The
  **live** check runs over seven public fixture repositories,
  `donaldgifford/docz-fixture-{basic,shared,solo,refused,layout-page,folder-clash,disabled}`,
  whose contents are `test/live/confluence/fixtures/<repo>/` and are
  published only by `just confluence-fixtures-push` (a force-push of one
  commit, which also resets a fixture after the hand-driven scenarios).
  Each is staged to end one way; `test/live/confluence/README.md` is the
  table of expected statuses, the setup (a `DOCZ2` space beside `DOCZ`,
  the App on those seven repositories only), the run, and the
  hand-driven rename/remove/edit/comment/disable scenarios. `basic` must
  export before `folder-clash`. Edit a fixture there, never in its
  repository; nothing in `just test` or CI reads the directory.
- **Deploy** (chart 0.3.0): `api.confluence.{site,email,spaces}` render the
  four `CONFLUENCE_*` variables **only while `site` is set**, with the
  token from the API Secret's `confluence-api-token` key (`required` then,
  as are the email and spaces). `DoczAPIExportFailures` (failed or partial
  exports over 30 minutes) is in the chart's PrometheusRule and
  `contrib/prometheus/alerts.yaml`. Turning it on, widening the allow-list,
  and reading each status are RUNBOOK-0003.
