# Migrating from docz v1 to v2

docz v2 is in beta (`v2.0.0-beta.8` at the time of writing). This guide
covers the move from v1.2.2 for three kinds of user. Read the sections that
apply to you:

- [CLI users](#cli-users): you run `docz` over a repository's `docs/`.
- [Go library consumers](#go-library-consumers): you import
  `github.com/donaldgifford/docz/...`.
- [Server operators](#server-operators): you run docz-api and docz-site,
  usually with their Helm charts.

The short version: **the CLI is a drop-in replacement**. The library needs
an import-path change, and the server needs a new Helm release.

## What stays the same

- **Repositories need no changes.** A v1 `.docz.yaml` and its documents load
  unchanged. v2 adds config blocks but removes none.
- **CLI behaviour is pinned to v1.2.2.** The parity suite (`test/parity/`)
  replays 213 cases captured from the v1.2.2 binary against every v2 build:
  stdout, stderr, exit codes, and every file written. It allows only these
  differences:
  - region marker lines in new documents;
  - new commands and flags;
  - new findings from existing commands;
  - the removed `plan` type;
  - the disabled `runbook` and `sync:` blocks that `docz init` writes.
- **The five v1 library packages are frozen.** `config`, `document`,
  `docparse`, `docwrite`, and `toc` keep their v1 shapes under
  `pkg/doczcore/`. The one exception is the removal of `plan` from the type
  catalogue.

## CLI users

### Install

```bash
mise use -g github:donaldgifford/docz@v2.0.0-beta.8
# or
go install github.com/donaldgifford/docz/v2/cmd/docz@v2.0.0-beta.8
```

Note the `/v2` in the `go install` path. The unversioned path stays on v1,
so `go install github.com/donaldgifford/docz/cmd/docz@v1.2.2` pins the last
v1 release, and that is also how you roll back.

### `plan` is no longer built in

Through v1, `plan` was a sixth built-in type. v2 drops it (ADR-0003).

- **A repository that keeps its `types.plan` block loses nothing.** The
  block now declares a *custom* type. `list`, `update`, `status`,
  `validate`, and the wiki nav keep working over `docs/plan/`, and each
  command prints a warning that `plan` is not a built-in.
- **Only `docz create plan` fails**, because v2 no longer bundles a plan
  template. To restore it, scaffold the template:

  ```bash
  docz template override plan
  ```

  This writes `docs/templates/plan.md`. Commit it.
- **A repository with no `plan:` block, or one with `enabled: false`, needs
  nothing.** Plan was disabled by default from v1.1.1 on.

### New: `runbook`, disabled by default

v2 adds a `runbook` type (DESIGN-0019) for operational procedures: steps,
commands, expected output, rollback, and a Last Verified table. It ships
disabled, and `docz create runbook` fails until you enable it.

How you enable it depends on your `.docz.yaml`:

- **It has a `types:` block.** Most v1 repositories do, because `docz init`
  wrote one. A `types:` block lists every type the repository uses, so add
  runbook to it. The short form is enough; docz fills in the rest from its
  defaults:

  ```yaml
  types:
    # ...your existing types...
    runbook:
      enabled: true
  ```

- **It has no `types:` block.** Add the same block. The other built-ins
  keep their defaults, because a `types:` block only *replaces* the type
  set when it lists types.

Then `docz init` creates `docs/runbook/` and its README without touching
anything else.

### New commands

| Command | What it does |
| --- | --- |
| `docz validate` | Checks every document against its type's schema: frontmatter, required sections, and per-type rules such as an IMPL phase with no tasks or an ADR with no decision. It also reports stale ToCs and README index drift. Use `--strict` to fail on warnings and drift too, and `--format json` for machine-readable output. |
| `docz validate --fix` | Adds region markers to documents that have none, then validates again. |
| `docz export confluence` | Publishes the repository to Confluence Cloud. It is opt-in through the `sync.confluence` block; see the README. |

### Region markers

Every v2 template wraps each section in a pair of HTML comments:

```markdown
<!--docz:goals:start-->
## Goals
...
<!--docz:goals:end-->
```

Markers are invisible when rendered. They tell docz where each section
starts and ends, so `validate`, the typed readers, and docz-api do not have
to guess from headings.

**Your v1 documents do not need them.** Without markers, docz infers
sections from the headings, and it always will (DESIGN-0015 §6). Inference
is not a deprecated path. `docz validate` reports one `region.inferred`
warning per unmarked document and checks everything else as normal.

To add the markers, run:

```bash
docz validate            # see what it finds first
docz validate --fix      # add markers, then re-validate
git diff                 # review: only marker lines are added
```

`--fix` never touches a document that already has markers, other than to
correct a misspelled one. A second run changes nothing.

### Every command respects `enabled`

In v1, some commands ignored a type's `enabled: false`. In v2:

- `docz list <type>` on a disabled type prints an empty listing;
- `docz status set` and the `docz template` subcommands fail with an error
  that names `types.<type>.enabled`;
- `docz update <type>` stays a silent no-op.

If a script relied on reaching a disabled type, enable the type.

### New config blocks

`docz init` on a new repository now writes two blocks that v1 did not:
`types.runbook` and `sync:`, both disabled. **`docz init` never rewrites an
existing `.docz.yaml`, even with `--force`**, so an existing repository
gets neither block unless you add it. You only need them if you want the
feature.

### Checklist

- [ ] Install v2 and run `docz --version`.
- [ ] Run `docz update --dry-run`. Expect no changes.
- [ ] Run `docz validate` and fix any errors. Warnings are optional.
- [ ] If you use plan documents, run `docz template override plan`.
- [ ] Optional: enable `runbook`.
- [ ] Optional: run `docz validate --fix` to add markers.
- [ ] Optional: add `docz validate` to CI. Add `--strict` once the
  repository is clean.

## Go library consumers

### The module path changes

```diff
-require github.com/donaldgifford/docz v1.2.2
+require github.com/donaldgifford/docz/v2 v2.0.0-beta.8
```

```diff
-import "github.com/donaldgifford/docz/pkg/doczcore/config"
+import "github.com/donaldgifford/docz/v2/pkg/doczcore/config"
```

One `sed` handles the imports:

```bash
grep -rl 'github.com/donaldgifford/docz/' --include='*.go' . \
  | xargs sed -i '' 's#github.com/donaldgifford/docz/#github.com/donaldgifford/docz/v2/#g'
go mod tidy
```

On Linux, drop the `''` after `-i`.

### Frozen packages

`config`, `document`, `docparse`, `docwrite`, and `toc` keep their v1
behaviour. Things that changed:

- **`plan` is gone from `config.DocTypeNames()`** and from
  `DefaultConfig().Types`. Code that assumed six built-ins, or looked for
  `plan` by name, needs updating. A config that declares `types.plan` still
  decodes, as a custom type.
- **`runbook` is in the catalogue but disabled by default.**
  `Config.EnabledTypes()` leaves it out until a repository enables it.
- **There are new additions, but nothing was removed:**
  - `config.ParseBytes(b)` parses a `.docz.yaml` you already have in memory.
    It matches `Load(path, "")` and never merges a global config.
  - `docwrite` has byte-level versions of its writers: `SetStatusBytes`,
    `SetTaskStateBytes`, and `NextNumber` + `Render`.
  - `docparse` has `Markers`, `Regions`, `ListItems`, and `Tables`.
  - `config` has the `SyncConfig` block.

### Packages made public

These were `internal/` in v1 and are importable in v2:

| v1 (internal) | v2 |
| --- | --- |
| `internal/index` | `pkg/doczcore/index` |
| `internal/template` | `pkg/doczcore/doctemplate` |
| `internal/wiki` | `pkg/wiki` |

### New packages

| Package | Purpose |
| --- | --- |
| `pkg/doczcore/repo` | Every CLI operation on a repository: `Open`, `List`, `Find`, `Create`, `Update`, `SetStatus`, `Validate`, `InsertRegions`. Errors are typed and paths are repo-relative. |
| `pkg/doczcore/kinds` | Readers for a region's contents: fields, items, sections, criteria, decisions, open questions. |
| `pkg/doczcore/validate` | The generic validator: `validate.Document(content, opts) []Finding`. |
| `pkg/{rfc,adr,design,impl,investigation,runbook}` | One typed reader per built-in, e.g. `impl.Parse(doc)` gives `doc.Phases[2].Tasks[0].Checked`. |
| `pkg/export/confluence` | Confluence Cloud export. `confluencetest` is an in-memory fake site. |

**These are EXPERIMENTAL until v2.0.0** (ADR-0002 Decision 7). Their API may
change between `v2.0.0-beta.N` tags, so pin an exact beta. The five frozen
packages are not experimental.

### Dependencies

`pkg/` brings in one third-party module, `gopkg.in/yaml.v3`. Importing
`pkg/export/confluence` adds a second, `github.com/yuin/goldmark`. The
server's dependencies, such as pgx, asynq, and Meilisearch, are not visible
to anything that imports `pkg/`; `test/consumer` checks this.

## Server operators

### What changed

docz-api and docz-site used to be separate repositories with their own
charts. In v2 they live in this repository (ADR-0004):

| | v1 era | v2 |
| --- | --- | --- |
| API image | `ghcr.io/donaldgifford/docz-api`, docz-api's own version | `ghcr.io/donaldgifford/docz-api:<docz version>` |
| Site image | `ghcr.io/donaldgifford/docz-site`, docz-site's own version | `ghcr.io/donaldgifford/docz-site:<docz version>` |
| Charts | `charts/docz-api`, `charts/docz-site` | **one chart**, `oci://ghcr.io/donaldgifford/charts/docz` |

Image tags are bare semver, such as `2.0.0-beta.8`; a `v`-prefixed tag
does not exist. The `docz` chart's `appVersion` selects the images, so leave
`image.tag` unset.

`docz-api` 0.10.0 and `docz-site` 0.3.0 are the final versions of the old
charts. Both are marked deprecated, and the charts are deleted at v2.0.0.

### There is no in-place upgrade

The `docz` chart renames every object to `<release>-docz-<role>` and adds
`app.kubernetes.io/component` to every selector. `helm upgrade` from either
old chart therefore fails. **Install `docz` as a new release next to the old
ones, switch traffic over, then uninstall the old releases.**

**You do not need to move the data.** docz-api rebuilds its state from
GitHub:

- **Documents, pages, the search index, changelogs, and index pages** are
  re-ingested from each repository's default branch.
- **Users** are re-created the next time each person logs in.
- **Sessions** are not carried over; everyone logs in again.
- **Confluence export** (if enabled) finds its existing pages by the
  properties it wrote on them, so it does not create duplicates.

### Values

The chart README's
[Coming from docz-api or docz-site](https://github.com/donaldgifford/docz/tree/main/charts/docz#coming-from-docz-api-or-docz-site)
section has the full key mapping. In short:

- docz-api's per-workload keys move under `api.` and docz-site's under
  `site.`.
- `config.authProviders` from either chart becomes the shared
  `auth.providers`.
- `otel.endpoint`/`otel.sampleRate`, `metrics`, `serviceMonitor`, and
  `prometheusRule` move to the top level, shared by both workloads.
- `store`, `queue`, and `search` are unchanged and stay at the top level.
- `site.config.doczApiUrl` is now optional; the chart points the site at
  the API Service by default.
- **The Tailscale sidecar is gone.** See `deploy/tailscale-operator.md` for
  running the Tailscale operator in front of the chart's Ingress.

### Steps

1. **Write the new values file.** Translate both old values files using the
   mapping. Reuse the same GitHub App (app ID, private key, webhook secret)
   and the same OAuth client, so nothing changes on GitHub except URLs.

   Never leave a top-level key such as `site:` with nothing under it. YAML
   reads that as `null`, which deletes the chart's whole block and fails the
   render.

2. **Install into a new release name or namespace:**

   ```bash
   helm install docz oci://ghcr.io/donaldgifford/charts/docz --version 0.3.0 \
     -n docz --create-namespace -f docz-values.yaml \
     --set-file api.secrets.privateKey=app.pem --wait
   helm test docz -n docz
   ```

   To check the deployment before any login credentials are set up, use
   `auth.providers: "none"`. Every request is then served as an anonymous
   user, so do not expose that install publicly.

3. **Check ingestion.** Repositories are ingested on install events and
   pushes, so a new release starts empty. To ingest a repository straight
   away, push a commit that touches its `docs/`, or run the onboard flag in
   the API pod. The image has no shell, so call the binary directly:

   ```bash
   kubectl -n docz exec deploy/docz-api -- \
     /usr/local/bin/docz-api -onboard owner/name@<installationID>
   ```

   Then check `GET /api/v1/repos`.

4. **Switch over.** Either:
   - move the old hostnames to the new Ingress or HTTPRoute objects, which
     changes nothing on GitHub; or
   - use new hostnames and update the GitHub App in **two** places:
     - the **webhook URL**, `https://<api host>/webhooks/github`;
     - the **callback URL**, `<auth redirect base>/auth/callback`. The new
       release's `api.config.authRedirectBase` must match it.

   If deliveries fail afterwards, use RUNBOOK-0002 to diagnose and redeliver
   them.

5. **Smoke-test.** Check that:
   - you can log in;
   - repositories are listed;
   - you can open a document;
   - search returns results;
   - a new push shows up within the ingest debounce (5s by default).

6. **Uninstall the old releases.** If they used baked backends, their PVCs
   survive `helm uninstall`. Delete those yourself once you are sure you
   will not roll back.

### Optional: Confluence export

v2's docz-api can export each repository to Confluence after every ingest.
The export is off unless `api.confluence.site` is set, and a repository is
only exported if its own `.docz.yaml` enables `sync.confluence`. RUNBOOK-0003
covers the token, the space allow-list, and how to read each repository's
status.

### Rolling back

Until you uninstall them, the old releases still work. To roll back:

- move the hostnames back (or point the GitHub App URLs back);
- `helm uninstall` the new release.

Nothing the new release did needs undoing on GitHub.

## References

- [ADR-0002](adr/0002-docz-is-an-api-package-whose-first-consumer-is-the-cli.md):
  the v2 library and the `/v2` path
- [ADR-0003](adr/0003-remove-plan-from-the-built-in-document-types.md): removing
  `plan`
- [ADR-0004](adr/0004-one-repository-docz-docz-api-and-docz-site-as-a-single-go-module.md): one
  repository for the CLI, the server, and the site
- [DESIGN-0015](design/0015-structured-regions-and-docz-validate.md): region
  markers and validation
- [DESIGN-0019](design/0019-runbook-a-sixth-built-in-document-type-disabled-by-default.md):
  the runbook type
- [DESIGN-0018](design/0018-one-helm-chart-for-docz-chartsdocz-replaces-docz-api-and-docz.md):
  the single chart
- [RUNBOOK-0002](runbook/0002-docz-api-webhook-deliveries-fail.md) and
  [RUNBOOK-0003](runbook/0003-enable-confluence-export-on-docz-api.md)
