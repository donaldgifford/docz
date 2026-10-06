---
id: INV-0020
title: "docz-api Confluence export: running confluence.Export without a checkout"
status: Open
author: Donald Gifford
created: 2026-10-06
---

<!-- markdownlint-disable-file MD025 MD041 -->

# INV-0020: docz-api Confluence export: running confluence.Export without a checkout

<!--toc:start-->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
  - [Observation 1: Export reads everything from a checkout](#observation-1-export-reads-everything-from-a-checkout)
  - [Observation 2: Postgres holds almost every input](#observation-2-postgres-holds-almost-every-input)
  - [Observation 3: pages are found by title, and titles are unique per space](#observation-3-pages-are-found-by-title-and-titles-are-unique-per-space)
  - [Observation 4: a server needs its own answer to credentials](#observation-4-a-server-needs-its-own-answer-to-credentials)
  - [Observation 5: the queue already has the shape a job needs](#observation-5-the-queue-already-has-the-shape-a-job-needs)
  - [Observation 6: links and the banner need the repository, not a remote](#observation-6-links-and-the-banner-need-the-repository-not-a-remote)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
  - [1. How does Export get its inputs without a checkout?](#1-how-does-export-get-its-inputs-without-a-checkout)
  - [2. Where do the inputs come from?](#2-where-do-the-inputs-come-from)
  - [3. What triggers an export?](#3-what-triggers-an-export)
  - [4. Whose credentials, and where may they write?](#4-whose-credentials-and-where-may-they-write)
  - [5. Can repositories share a space?](#5-can-repositories-share-a-space)
  - [6. Is a table of page ids needed, and what is it for?](#6-is-a-table-of-page-ids-needed-and-what-is-it-for)
  - [7. What does the API expose?](#7-what-does-the-api-expose)
  - [8. How does docz-site show it?](#8-how-does-docz-site-show-it)
  - [9. When does the server use --force, and what counts as failure?](#9-when-does-the-server-use---force-and-what-counts-as-failure)
  - [10. How are links to files it does not export resolved?](#10-how-are-links-to-files-it-does-not-export-resolved)
- [References](#references)
<!--toc:end-->

<!--docz:question:start-->
## Question

Can docz-api run Phase A's `confluence.Export` for every repository it
indexes, when it has no checkout of any of them? If so, where do its inputs
come from, what triggers it, whose Confluence credentials does it use, and
what does it record?

<!--docz:question:end-->

<!--docz:hypothesis:start-->
## Hypothesis

Yes, with one change to the package. Issue #154 says `pkg/export/confluence`
"is built for" docz-api. That is true of `Render` (bytes in) and of the
injected `Client`, but `Export` reads every document from disk. Postgres
already holds nearly every input as bytes, so a source that is not a
filesystem path, and a queue job after ingest, should be enough. Credentials
and spaces are the parts a single-user CLI never had to decide.

<!--docz:hypothesis:end-->

<!--docz:context:start-->
## Context

**Triggered by:** issue [#154](https://github.com/donaldgifford/docz/issues/154),
DESIGN-0020 §7 (Phase B).

Phase A (IMPL-0023, `v2.0.0-beta.7`) shipped `docz export confluence` for a
person with a checkout and their own token. Phase B is the same export run
by docz-api for every repository that opts in. The issue asks for a DESIGN
covering the job and queue wiring, a table keyed by document id, the API
surface, and the docz-site link. This investigation settles the questions
that shape that DESIGN before it is written.

<!--docz:context:end-->

<!--docz:approach:start-->
## Approach

1. Trace every place `Export` reads from the filesystem.
2. List what docz-api holds for a repository after ingest: the
   `RepoSnapshot` it fetches, and the rows it keeps in Postgres.
3. Compare the two to find the gaps.
4. Read how Phase A finds, identifies, and archives pages, and what changes
   when many repositories share one server and possibly one space.
5. Read docz-api's queue, config, and API conventions for where a job, a
   credential, and a link would go.

<!--docz:approach:end-->

<!--docz:environment:start-->
## Environment

| Component | Version / Value |
| --------- | --------------- |
| docz | `v2.0.0-beta.7` (`main` at `ee1f73a`) |
| `pkg/export/confluence` | Phase A, EXPERIMENTAL |
| docz-api queue | asynq on Redis, in-process worker, concurrency 2 |
| docz-api store | Postgres; `repos`, `doc_types`, `documents`, `repo_pages` |

<!--docz:environment:end-->

<!--docz:findings:start-->
## Findings

### Observation 1: Export reads everything from a checkout

`Export` takes a `*repo.Repo` and reads four kinds of input from disk:

| Input | How it is read |
| ----- | -------------- |
| Documents | `rp.List` and `rp.Find`, which scan type directories (`plan.go:213`) |
| Type pages | each type's `README.md` index, by `os.ReadFile` (`plan.go:324`) |
| The parent page body | the `api:` landing page, by `os.ReadFile` |
| Additional docs | each `api.additional_docs` entry, by `os.ReadFile` (`plan.go:385`) |

`Render`, the `Client`, and the reconcile logic never touch the
filesystem. The disk dependency is entirely in planning: deciding which
pages exist and loading their bytes.

### Observation 2: Postgres holds almost every input

| Export input | docz-api has it as |
| ------------ | ------------------ |
| Documents | `documents.raw_md`, with `path`, `doc_id`, `type`, and frontmatter fields |
| Config, including `sync.confluence` | `repos.config_snapshot`, the full `.docz.yaml` |
| Landing page | `repos.index_md`, which is `docs_dir/index.md`: the landing page by default, though `api.landing_page` can name another file |
| Additional docs | `repo_pages.raw_md`, when the repository's `api:` block is enabled |
| Type READMEs | **nothing.** Ingest skips them on purpose (docz-api's IMPL-0009, archived under `docs/archive/api/`) |
| Index header overrides (`docs/templates/index_<type>.md`) | **nothing.** Never fetched |

The README is the index table `docz update` generates, so it can be rebuilt
from the document rows with `index.GenerateTable` and the embedded header.
What is lost is a repository's own header override, which is rare.

Reading from Postgres rather than from the ingest snapshot also means the
export can run after the ingest transaction commits, and be retried without
fetching from GitHub again.

### Observation 3: pages are found by title, and titles are unique per space

`reconcile.go:46` finds a document's page with `FindPage(spaceID, title)`.
The `docz` property then confirms it is docz's page. Two consequences:

- **Renaming a document moves it to a new page.** The new title finds
  nothing, so a new page is created, and the old page is archived as an
  orphan. Keying pages by document id, as #154 proposes, would update the
  page in place instead.
- **Two repositories cannot share a space as-is.** Confluence requires page
  titles to be unique within a space. Two repositories that both have
  `ADR-0001`, or both have a "Design" type page, or both need an
  `Archive` page, would collide. A CLI user picks their own space and never
  meets this. A server syncing many repositories will.

`PageResult` already reports `PageID`, `URL`, and `Version` for each
document, which is what a table keyed by id would store.

### Observation 4: a server needs its own answer to credentials

Phase A reads `ATLASSIAN_EMAIL` and `ATLASSIAN_API_TOKEN` from the person
running it, and writes wherever that person can write. In docz-api, the
site and space come from each repository's `.docz.yaml`, which the
repository's owners control. With one server-wide token, any repository the
GitHub App is installed on could direct the server to write into any space
that token can reach. The server needs an allow-list of the sites and
spaces it will write to, whatever the credential model is.

docz-api's config is env-only, and secrets use `config.Secret`, which
redacts itself in logs and formatting. A token fits that pattern; per-repository
or per-installation tokens would need storage and an admin surface docz-api
does not have.

### Observation 5: the queue already has the shape a job needs

Ingest runs as an asynq task coalesced per repository (`TaskID =
"ingest:owner/name"`, debounced, `MaxRetry(5)`), and a terminal task is
cleared rather than coalesced onto. An export task can reuse every part of
that: one task per repository, enqueued after a successful ingest, coalesced
the same way, so a burst of pushes exports once. Phase A's `Export` already
checks the context between pages, so a worker shutdown leaves written pages
written.

### Observation 6: links and the banner need the repository, not a remote

Phase A resolves a link to a file it does not export into a GitHub blob URL
from `git remote`, and only when the file exists. docz-api knows the owner,
name, and default branch of every repository, so the URL is easy to build.
What it does not know is whether an arbitrary file exists: it fetches only
`.docz.yaml`, the type directories, and the `api:` files, not the full tree.

<!--docz:findings:end-->

<!--docz:conclusion:start-->
## Conclusion

**Answer:** Yes. docz-api can run the export with one change to
`pkg/export/confluence`: a way to supply its inputs as bytes instead of a
directory (Observation 1). Postgres holds every input except the type READMEs,
which can be regenerated from the document rows (Observation 2). The queue
needs only a second task type modelled on ingest (Observation 5).

Two decisions Phase A never faced are what the DESIGN mostly has to settle.
Titles are unique per space, so repositories cannot share a space without a
rule for it (Observation 3). A server-wide credential needs an allow-list of
sites and spaces, because repository owners choose where their pages go
(Observation 4).

<!--docz:conclusion:end-->

<!--docz:recommendation:start-->
## Recommendation

Resolve the questions below, then write the DESIGN from them.

### 1. How does Export get its inputs without a checkout?

- **(a) Add `ExportOptions.FS fs.FS`.** When it is set, `Export` lists and
  reads documents, READMEs, the landing page, and additional docs through it
  instead of `rp.Root`, and `rp` supplies only the config. docz-api builds an
  in-memory `fs.FS` from Postgres rows. `fs.FS` is a standard-library
  interface, so this adds no docz-defined interface, and the change stays
  inside the EXPERIMENTAL `pkg/export/confluence`. The frozen `document`
  package is untouched. *(recommendation)*
- (b) Write the rows to a temporary directory and call `Export` unchanged.
  It needs no library change. But docz-api's image has a read-only root
  filesystem, so it needs a writable volume, plus cleanup on every path.
- (c) Split `Export` into a public plan step and a public reconcile step,
  and let docz-api build the plan from rows. This exposes more surface than
  (a) and duplicates the planning rules on the docz-api side.
- (d) Other.

### 2. Where do the inputs come from?

- **(a) Postgres rows**, read after the ingest transaction commits. Type
  READMEs are regenerated with `index.GenerateTable` and the embedded
  header. A repository's own index header override is not used.
  *(recommendation)*
- (b) The ingest `RepoSnapshot`, passed straight to the export inside the
  ingest run. It is fresher, but couples export failures and retries to
  ingest.
- (c) A fresh GitHub fetch per export. It sees everything, header overrides
  included, but doubles the GitHub API calls.
- (d) Other.

### 3. What triggers an export?

- **(a) A separate asynq task**, enqueued after a successful ingest when the
  repository's `sync.confluence.enabled` is true. It is coalesced per
  repository (`export:owner/name`) and retried like ingest, so an export
  failure never fails or retries the ingest. *(recommendation)*
- (b) A step at the end of the ingest run, so one failure path covers both.
- (c) Manual only, through an API call.
- (d) Other.

### 4. Whose credentials, and where may they write?

- **(a) One server-wide Atlassian account**, set as `CONFLUENCE_EMAIL` and
  `CONFLUENCE_API_TOKEN` (a `config.Secret`), plus an allow-list,
  `CONFLUENCE_ALLOWED`, of the site and space pairs the server will write
  to. A repository whose `sync.confluence` names anything else is skipped,
  and the skip is logged and shown in the API. *(recommendation)*
- (b) Per-installation credentials stored encrypted in Postgres. This needs
  an admin API and key management docz-api does not have.
- (c) The (a) account with no allow-list. This lets any installed
  repository write into any space the token can reach.
- (d) Other.

### 5. Can repositories share a space?

- **(a) No: one space per repository.** The allow-list maps each space to at
  most one repository, so titles never collide, and Phase A's titles and
  archive rules apply unchanged. *(recommendation)*
- (b) Yes, with every title prefixed by the repository (`owner/name:
  ADR-0001: …`). This changes Phase A's titles for docz-api pages only, and
  each repository needs its own type pages and Archive.
- (c) Yes, in one shared tree with the repository as an extra level. This
  still needs (b)'s prefixes, because titles are unique across the whole
  space.
- (d) Other.

### 6. Is a table of page ids needed, and what is it for?

- **(a) Yes: `confluence_pages (repo_id, doc_id, page_id, version, hash,
  synced_at)`.** It is the record the API and docz-site read, and it is
  passed back into `Export`, so a document is found by page id before title.
  A renamed document then updates its page instead of being archived and
  recreated. That needs one more option on the package, for known page ids.
  The `docz` page property stays what reconcile trusts. *(recommendation)*
- (b) Yes, as a read-only record for the API and site. Pages are still
  found by title, so renames still move them.
- (c) No: the API asks Confluence when it needs a URL.
- (d) Other.

### 7. What does the API expose?

- **(a) Read-only.** Each document DTO gains a `confluence_url` (empty
  string when there is none, per the spec's no-null rule). A new `GET
  /api/v1/repos/{owner}/{name}/confluence` returns the last run's time,
  counts, and per-page results. *(recommendation)*
- (b) (a) plus a `POST` that triggers an export. `authorize` is still a
  pass-through seam, so any signed-in user could trigger writes.
- (c) Nothing until docz-site needs it.
- (d) Other.

### 8. How does docz-site show it?

- **(a) A "View in Confluence" link** on a document's page when
  `confluence_url` is set. Nothing else changes. *(recommendation)*
- (b) (a) plus a sync-status badge on the repository page.
- (c) Nothing in Phase B.
- (d) Other.

### 9. When does the server use `--force`, and what counts as failure?

- **(a) Never force.** A page edited in Confluence stays Skipped and is
  shown in the API. A failed page fails the task, so asynq retries it with
  backoff, which is safe because unchanged pages are no-ops. Skipped pages do
  not fail the task. *(recommendation)*
- (b) Force on a schedule (say nightly), overwriting Confluence edits.
- (c) Never force, and never fail the task. Results are only recorded.
- (d) Other.

### 10. How are links to files it does not export resolved?

- **(a) To GitHub blob URLs**, built from the repository's owner, name, and
  default branch with no existence check. A link to a missing file 404s on
  GitHub, as it already does there. *(recommendation)*
- (b) Only for paths seen in the ingest tree listing. This needs the full
  tree, which ingest does not fetch.
- (c) Left unresolved and reported, as Phase A does with no remote.
- (d) Other.

<!--docz:recommendation:end-->

<!--docz:references:start-->
## References

- Issue [#154](https://github.com/donaldgifford/docz/issues/154): Phase B
- [DESIGN-0020](../design/0020-confluence-export-docz-documents-as-confluence-cloud-pages.md):
  the export design; §7 is Phase B
- [IMPL-0023](../impl/0023-confluence-export-the-package-and-the-cli-design-0020-phase-a.md):
  Phase A as built
- [INV-0019](0019-docz-api-sync-to-external-documentation-services-confluence-and.md):
  Confluence and its API
- `pkg/export/confluence/plan.go`, `reconcile.go`: the disk reads and the
  title lookup
- `internal/ingest/fetcher.go`, `internal/queue/`, and
  `internal/store/migrations/`: what docz-api fetches, queues, and stores

<!--docz:references:end-->
