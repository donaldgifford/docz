---
id: INV-0037
title: "Nested .docz.yaml files in one repository: monorepo projects in the CLI, API, site, and MCP"
status: Concluded
author: Donald Gifford
created: 2026-11-23
---

<!-- markdownlint-disable-file MD025 MD041 -->

# INV-0037: Nested .docz.yaml files in one repository: monorepo projects in the CLI, API, site, and MCP

<!--toc:start-->
<!-- markdownlint-capture -->
<!-- markdownlint-disable MD013 -->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
  - [1. A nested config already works as an independent root](#1-a-nested-config-already-works-as-an-independent-root)
  - [2. Writing into the root docs/ works, with three defects](#2-writing-into-the-root-docs-works-with-three-defects)
  - [3. The server reads one config, at the tree's root](#3-the-server-reads-one-config-at-the-trees-root)
  - [4. Everything on the server is keyed on owner/name](#4-everything-on-the-server-is-keyed-on-ownername)
  - [5. Webhooks would miss a project](#5-webhooks-would-miss-a-project)
  - [6. What each layout costs](#6-what-each-layout-costs)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
- [Open Questions](#open-questions)
  - [1. How is a project found?](#1-how-is-a-project-found)
  - [2. Where do a project's files live?](#2-where-do-a-projects-files-live)
  - [3. How is a project addressed in the API, the site, and MCP?](#3-how-is-a-project-addressed-in-the-api-the-site-and-mcp)
  - [4. Does a project inherit from the root config?](#4-does-a-project-inherit-from-the-root-config)
  - [5. Is a root config still required?](#5-is-a-root-config-still-required)
  - [6. How does the site list projects?](#6-how-does-the-site-list-projects)
  - [7. Do wiki, Confluence, and HTML export follow projects?](#7-do-wiki-confluence-and-html-export-follow-projects)
- [References](#references)
<!-- markdownlint-restore -->
<!--toc:end-->

<!--docz:question:start-->
## Question

Can one git repository hold several `.docz.yaml` files, one per service
or project at any depth outside the root `docs/`, so that the docz CLI
manages each one and docz-api, docz-site, and the MCP server ingest and
show each one? If so, what has to change, and which of these layouts
costs least:

1. The root `.docz.yaml` names the paths where nested configs live.
2. A nested config registers as its own repository, shown as
   `repo/my/service/path`.
3. Nested configs write into the root `docs/` as `docs/<project>/`, with
   the normal layout below that.

"Project" below means one nested `.docz.yaml` and the documents it
governs.

<!--docz:question:end-->

<!--docz:hypothesis:start-->
## Hypothesis

The CLI is close: it takes its root from the working directory, so a
nested config probably already works alone. The server is the expensive
side, because a repository's identity there is `owner/name` and the
config is read from the tree's root. Layout 3 was expected to be the
cheapest, since the root `docs/` is already ingested.

<!--docz:hypothesis:end-->

<!--docz:context:start-->
## Context

docz assumes one config and one `docs/` per repository. A monorepo with
many services has no way to give each service its own RFCs, ADRs, and
runbooks with its own numbering, statuses, and owners, and still have
all of them appear in docz-api.

**Triggered by:** a request from the maintainer on 2026-11-23; no issue
yet.

<!--docz:context:end-->

<!--docz:approach:start-->
## Approach

1. Read how the CLI finds its root and config (`cmd/root.go`,
   `pkg/doczcore/config`, `pkg/doczcore/repo`).
2. Run docz 2.6.2 in a scratch directory with a root config and three
   nested ones: independent (`services/billing`), writing into the root
   `docs/` (`libs/auth/v2` with `docs_dir: ../../../docs/auth`), and a
   config placed inside `docs/billing` with `docs_dir: .`.
3. Read how the server fetches, stores, addresses, and authorizes a
   repository (`internal/githubapp`, `internal/ingest`,
   `internal/webhook`, `internal/store`, `internal/mcp`,
   `api/openapi.yaml`, `ui/src/app/router.tsx`).
4. Cost each layout against what steps 1 to 3 found.

Nothing was run against a live docz-api; the server findings are from
the code.

<!--docz:approach:end-->

<!--docz:environment:start-->
## Environment

| Component | Version / Value |
| --------- | --------------- |
| docz CLI  | 2.6.2 (`c8c929c1`) |
| Source    | `main` at `7d3127d` |
| Platform  | macOS, scratch directory under `$TMPDIR` |

<!--docz:environment:end-->

<!--docz:findings:start-->
## Findings

### 1. A nested config already works as an independent root

`resolveRepoRoot` takes `--repo-root`, else the directory of `--config`,
else the working directory. Nothing walks up or down. Running
`docz init` and `docz create adr` inside `services/billing` scaffolded
`services/billing/docs/` and created `ADR-0001` there, beside the root's
own `ADR-0001`. `list`, `validate`, and `update` all worked from that
directory.

The same rule has three consequences:

- From `services/billing/src/deep`, `docz list adr` printed an empty
  table and exit 0. No config was found, the defaults applied, and
  `docz create` there would start a third tree.
- From the root, `list`, `validate`, `update`, and `wiki update` see
  only the root's documents. The root has no knowledge of a project.
- `docz --repo-root services/billing list adr` (a relative path) printed
  an empty table; the absolute path and `--config <path>` both listed
  the document. This looks like the root being joined twice and is a
  bug on its own, not chased here.

### 2. Writing into the root docs/ works, with three defects

`libs/auth/v2/.docz.yaml` holding `docs_dir: ../../../docs/auth` passed
`init`, `create`, `validate`, and `update`, and produced
`docs/auth/adr/0001-auth-decision.md`. `docs_dir` is not run through the
repo-relative path rules, which is the only reason this works.

- `docz create --format json` reported the `path` as absolute, because
  the file is outside the root the CLI was given. The CLI's JSON
  contract and docz-review read that field.
- Adding `api: {enabled: true}` to the same config failed to load:
  `api.landing_page "../../../docs/auth/index.md" must not traverse
  outside the repo root`. A project written this way cannot carry an
  `api:` block.
- The root's `docz wiki update` added `auth/adr/README.md` and
  `auth/adr/0001-auth-decision.md` to `mkdocs.yml` as plain pages. The
  root publishes the project's files without knowing their types.

A config placed at `docs/billing/.docz.yaml` with `docs_dir: .` has
none of the three: paths stay relative (`adr/0001-billing.md`) and the
`api:` block loads. It costs the ergonomics instead, since the config
no longer sits in the service's directory.

### 3. The server reads one config, at the tree's root

`githubapp.Client.Fetch` looks up `.docz.yaml` by exact path in the
recursive tree and fails with `has no .docz.yaml at HEAD` when it is
absent. A monorepo with only nested configs cannot be onboarded.

Two details make discovery cheap. The fetch already holds the whole
tree, so finding every nested `.docz.yaml` costs no request and reading
each costs one blob. And `classifyTree` already fetches every file that
matches the docz filename convention anywhere in the tree, so a
project's documents are downloaded today and then dropped by
`buildDocuments`, which keeps a blob only when its directory equals
`<docs_dir>/<type.dir>` of the root config.

For layout 3 that means `docs/auth/adr/0001-auth-decision.md` is not a
document to the server. With the root's `api:` block enabled it is
published as a page at `auth/adr/0001-auth-decision.md`: no type, no
status, absent from every typed listing and from `get_impl_progress`.

### 4. Everything on the server is keyed on owner/name

| Surface | Key today |
| ------- | --------- |
| `repos` table | `UNIQUE (owner, name)` |
| `doc_types`, `documents`, pages | `repo_id`; `UNIQUE (repo_id, doc_id)` |
| Search index | primary key `<repo_id>_<doc_id>` |
| REST | `/api/v1/repos/{owner}/{name}/…` |
| Site | `/:owner/:repo/:type/:docId` |
| MCP | `repo` as `owner/name`; a second `/` is refused |
| Access | `access.scopes` stored on the repo row |
| Config | `config_snapshot`, `docs_dir`, index, changelog on the repo row |

Two projects each have an `ADR-0001`, so folding projects into one
repository row breaks `UNIQUE (repo_id, doc_id)` and the search key
unless the document id gains a project prefix. Giving each project its
own `repos` row breaks nothing below the row: types, documents, pages,
search, access scopes, and the config snapshot all follow `repo_id`.
What that leaves is the address, because `{name}` cannot hold a slash
in the REST path, the site's route, or the MCP argument.

### 5. Webhooks would miss a project

`shouldIngest` re-ingests on a push that touches the root `.docz.yaml`,
a path under the row's `docs_dir`, or a watched file. A push that
changes only `services/billing/docs/…` or `services/billing/.docz.yaml`
is skipped. With one row per project the existing check is right for
each row, given a per-project config path; the handler then has to look
up every row for the repository and not one.

### 6. What each layout costs

| | 1. Root lists paths | 2. Own repository | 3. Root `docs/<project>/` |
| --- | --- | --- | --- |
| CLI today | Not present | Works from the project directory | Works, with the defects in finding 2 |
| CLI work | A `projects:` block; root commands that fan out | Walk up to the nearest config; a project selector | Validate `docs_dir`; relative paths; keep the root from publishing a project |
| Server work | Read nested configs from the tree | A row per project, a project address | Same as 2, plus the root row must skip `docs/<project>/` |
| IDs | Per project | Per project | Per project, in one directory tree |
| Owners | Docs beside the code, one CODEOWNERS rule | Same | Docs away from the code |

Layouts 1 and 2 are not alternatives. Layout 1 is how a project is
found, and layout 2 is how it is shown. Layout 3 is a choice about
where the files sit, and it does not save the server work: the server
still needs each project's config for its types and statuses, and still
needs to be told which directories are projects.

<!--docz:findings:end-->

<!--docz:conclusion:start-->
## Conclusion

**Answer:** Yes, and most of the CLI half already exists.

The hypothesis held for the CLI and the server and was wrong about
layout 3. Writing into the root `docs/` is the cheapest thing to try
today, but it is not the cheapest to support: it depends on an
unvalidated `docs_dir`, breaks the `api:` block and the JSON `path`,
and has the root publish the project's files as untyped pages. On the
server it needs everything the other layouts need.

The cheapest complete shape is a project that is a whole docz root in
its own directory (which works now), found through a list in the root
config, and stored as its own `repos` row.

<!--docz:conclusion:end-->

<!--docz:recommendation:start-->
## Recommendation

Write a design for the following. The seven open questions were all
resolved as (a) on 2026-11-23, which is this list:

1. **Discovery.** A `projects:` block in the root `.docz.yaml` holding
   paths or globs (`services/*`, `libs/**`). A root config stays
   required. The CLI and the server read the same list; the server
   matches it against the tree it already fetched.
2. **A project is a complete config.** No inheritance from the root in
   the first version, so `config.ParseBytes` keeps working on one file
   and each project's types and statuses are its own.
3. **Files stay with the project** (`services/billing/docs/`). Writing
   into the root `docs/<project>/` is not a supported layout (question
   2).
4. **CLI.** Walk up from the working directory to the nearest
   `.docz.yaml`; add a selector (`docz --project services/billing …`)
   and let `validate`, `update`, and `list` at the root take
   `--all-projects`. Validate `docs_dir` as a repo-relative path once
   projects make the traversal unnecessary; that is a new hard error and
   needs a release note.
5. **Server.** One `repos` row per project, with a `project` column
   (empty for the root) and `UNIQUE (owner, name, project)`. Ingest
   reconciles the root and every project from one fetch; the webhook
   checks each row. The root row excludes project directories that fall
   under its own `docs_dir`.
6. **Address.** One spelling across REST, the site, and MCP; see
   question 3.

Fix the relative `--repo-root` defect from finding 1 separately.

<!--docz:recommendation:end-->

<!--docz:open-questions:start-->
## Open Questions

### 1. How is a project found?

- **a. A `projects:` list of paths and globs in the root config.**
  *(recommendation)* Explicit, one place to read, and the server
  matches it against a tree it already holds.
- b. Any `.docz.yaml` in the tree is a project, with a root-level
  `projects.exclude` for vendored code and fixtures.
- c. Each nested config opts in with its own key (`project: {register:
  true}`), and the root lists nothing.
- d. Other.

> **Resolved 2026-11-23: (a).**

### 2. Where do a project's files live?

- **a. In the project's directory, `services/billing/docs/`.**
  *(recommendation)* Works today, keeps one CODEOWNERS rule and one
  path filter per service.
- b. In the root, `docs/<project>/`, with the config in the project's
  directory and `docs_dir` pointing back. Needs `docs_dir` to allow
  traversal inside the git root, and fixes for finding 2.
- c. In the root, `docs/<project>/`, with the config there too and
  `docs_dir: .`. No traversal; the CLI is run with `--project`.
- d. Both a and c are supported and each project chooses.
- e. Other.

> **Resolved 2026-11-23: (a).**

### 3. How is a project addressed in the API, the site, and MCP?

- **a. Its own repository entry, with the path folded into one segment:
  `owner/name~services~billing`.** *(recommendation)* Every existing
  route, the site's router, and the MCP `repo` argument keep their
  shape; `list_repos` adds `project` and `parent` fields.
- b. A new route level: `/repos/{owner}/{name}/projects/{project}/…`,
  `/:owner/:repo/p/:project/:type/:docId`, and a `project` argument on
  each MCP tool.
- c. A `?project=` query parameter on the existing routes.
- d. Projects fold into the root's entry and document ids gain a
  prefix (`billing/ADR-0001`).
- e. Other.

> **Resolved 2026-11-23: (a).**

### 4. Does a project inherit from the root config?

- **a. No. Each project config is complete.** *(recommendation)* for
  the first version.
- b. Yes, the root config is merged under the project's, the way
  `$HOME/.docz.yaml` is merged under a repository's today.
- c. Only named blocks inherit (`access`, `html`, `sync`).
- d. Other.

> **Resolved 2026-11-23: (a).**

### 5. Is a root config still required?

- **a. Yes.** *(recommendation)* It carries `projects:` and the server
  already refuses a repository without one. A monorepo with no root
  documents sets every type disabled.
- b. No. A repository with only nested configs is onboarded from them.
- c. Other.

> **Resolved 2026-11-23: (a).**

### 6. How does the site list projects?

- **a. Grouped under their repository on the repositories page, and as
  a project switcher inside a repository.** *(recommendation)*
- b. Flat, each project a row like any repository.
- c. Other.

> **Resolved 2026-11-23: (a).**

### 7. Do wiki, Confluence, and HTML export follow projects?

- **a. Not in the first version. Each runs per project from the
  project's own config, as it does today.** *(recommendation)*
- b. The root's `wiki update` builds one nav across every project.
- c. Other.

> **Resolved 2026-11-23: (a).**

<!--docz:open-questions:end-->

<!--docz:references:start-->
## References

- [`cmd/root.go`](../../cmd/root.go): `resolveRepoRoot`, `loadAndValidateConfig`
- [`pkg/doczcore/config/config.go`](../../pkg/doczcore/config/config.go):
  `Load`, `ParseBytes`, `Validate`
- [`internal/githubapp/client.go`](../../internal/githubapp/client.go):
  `Fetch`, `classifyTree`
- [`internal/ingest/service.go`](../../internal/ingest/service.go):
  `Run`, `buildDocuments`
- [`internal/ingest/pages.go`](../../internal/ingest/pages.go):
  `classifyDocsDir`
- [`internal/webhook/events.go`](../../internal/webhook/events.go):
  `handlePush`, `shouldIngest`
- [The initial schema](../../internal/store/migrations/20260702000000_initial_schema.sql)
- [`internal/mcp/tools.go`](../../internal/mcp/tools.go): the `repo` argument
- [`api/openapi.yaml`](../../api/openapi.yaml) and
  [`ui/src/app/router.tsx`](../../ui/src/app/router.tsx)
- [INV-0005](0005-docz-api-and-docz-site-centralized-cross-repo-docz-registry-and.md),
  the cross-repository registry that set `owner/name` as the
  key
- [DESIGN-0011](../design/0011-api-config-block-index-page-and-additionaldocs-for-docz-api-and.md),
  the `api:` block and its path rules
- [DESIGN-0023](../design/0023-a-read-only-mcp-server-in-docz-api-oauth-21-tokens-tools-and.md),
  access scopes on the repository row

<!--docz:references:end-->
