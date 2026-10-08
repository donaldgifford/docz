# Confluence live fixtures

The live check for docz-api's Confluence export (DESIGN-0021, IMPL-0024
Phase 9). Each directory under `fixtures/` is the whole contents of one
public GitHub repository under `donaldgifford`, staged so that exporting it
ends one specific way. docz-api ingests them through a GitHub App, exports
them to the scratch site in `~/.config/docz/atlassian.env`, and each
repository's `GET /api/v1/repos/{owner}/{name}/confluence` is compared
with the table below.

Nothing here runs in `just test` or CI. Unit tests and the in-memory
`confluencetest` site cover what content cannot stage: rate limits, 5xx
responses, network failures, and malformed renders.

**Edit a fixture here, never in its repository.** `just
confluence-fixtures-push` force-pushes every fixture (or the ones named)
as a single commit, which is also how a fixture is reset after the
hand-driven scenarios below change it. The fixtures hold no credentials:
the site URL and space keys are already public in docz's own `.docz.yaml`.

## Fixtures

| Repository | Staged | Expected status |
| ---------- | ------ | --------------- |
| `docz-fixture-basic` | `DOCZ`; RFC-0002 carries links to a document, to a non-exported file, and to nothing, plus mermaid, a table, tasks, an alert, and code | `succeeded`; folder `docz-fixture-basic`; `../missing.md` unresolved |
| `docz-fixture-shared` | `DOCZ` again; `api:` enabled with `docs/index.md` and `CONTRIBUTING.md`, `api_pages: true` | `succeeded`; a second folder in `DOCZ`, no title collision with `basic`; the landing page as the home page's body |
| `docz-fixture-solo` | `DOCZ2`, the second allowed space | `succeeded`; alone in `DOCZ2` |
| `docz-fixture-refused` | `NOTALLOWED`, a space not in `CONFLUENCE_SPACES` | `refused`; nothing written, not retried |
| `docz-fixture-layout-page` | `DOCZ`, `layout: page` | `refused`; the server writes the folder layout only |
| `docz-fixture-folder-clash` | `DOCZ`, `folder: docz-fixture-basic` | `failed` with a configuration error naming `sync.confluence.folder`; `basic` untouched; not retried |
| `docz-fixture-disabled` | `sync.confluence.enabled: false` | `never` after an ingest (nothing is enqueued); `disabled` after `-export` |

Each fixture's own `README.md` states its expectation in more detail.
**Order matters for one pair:** `docz-fixture-basic` must export before
`docz-fixture-folder-clash`, or the clash creates the folder and `basic`
is the one that fails.

## One-time setup

1. Create the seven repositories (public, empty):

   ```sh
   for r in basic shared solo refused layout-page folder-clash disabled; do
     gh repo create "donaldgifford/docz-fixture-$r" --public \
       --description "docz Confluence export live fixture ($r); generated from donaldgifford/docz test/live/confluence"
   done
   ```

2. Push the fixtures: `just confluence-fixtures-push`.
3. In the scratch site, create a space with the key `DOCZ2` (`DOCZ`
   already exists).
4. Install the development GitHub App on the seven repositories only, and
   note the installation id (the number at the end of the installation's
   settings URL).

## Running the check

1. Start the dependencies and point docz-api at the scratch site, with
   login off so `curl` needs no session:

   ```sh
   just api dev-up
   set -a; source .env; source ~/.config/docz/atlassian.env; set +a
   export AUTH_PROVIDERS=none \
     CONFLUENCE_SITE="https://${ATLASSIAN_SITE#https://}" \
     CONFLUENCE_EMAIL="$ATLASSIAN_EMAIL" \
     CONFLUENCE_API_TOKEN="$ATLASSIAN_API_TOKEN" \
     CONFLUENCE_SPACES="DOCZ,DOCZ2"
   just api run
   ```

   **Expected:** `confluence credentials verified` in the log.

2. In a second shell, onboard the fixtures, `basic` first. Each ingest
   enqueues its export:

   ```sh
   inst=<installation id>
   for r in basic shared solo refused layout-page folder-clash disabled; do
     build/bin/docz-api -onboard "donaldgifford/docz-fixture-$r@$inst"
   done
   ```

   The `-onboard` process needs the same environment as the server.

3. Read every status:

   ```sh
   for r in basic shared solo refused layout-page folder-clash disabled; do
     curl -s "localhost:8080/api/v1/repos/donaldgifford/docz-fixture-$r/confluence" |
       jq -c '{repo, status, reason, folder: .folder.title, counts}'
   done
   ```

4. Run `build/bin/docz-api -export donaldgifford/docz-fixture-disabled` and
   read its status again: `disabled`.

## Hand-driven scenarios

Against `docz-fixture-basic` after its first export. Edit a clone of the
repository, push, and re-ingest with its `-onboard` line again, or let
the webhook do it when the server is reachable through ngrok
(DEVELOPMENT.md, "Receiving GitHub webhooks locally").

1. **Rename.** Change RFC-0001's `title:` and H1. Its page is renamed in
   place: same page id and URL, new title.
2. **Remove.** Delete ADR-0001 and run `docz update`. Its page moves under
   `docz-fixture-basic: Archive`; nothing is deleted.
3. **Edit in Confluence.** Edit RFC-0002's page in the browser, then push
   any change to RFC-0002. The server overwrites the edit, and the page's
   entry in `pages[]` carries `edited`.
4. **Inline comment.** Comment on a sentence of RFC-0002's page, then push
   a change elsewhere in RFC-0002: the comment survives. Change the
   commented sentence and push again: the page's entry reports one lost
   comment.
5. **Disable.** Set `enabled: false` and push. The status becomes
   `disabled` once `-export` runs, and the pages stay in Confluence.

Afterwards, `just confluence-fixtures-push docz-fixture-basic` resets the
repository.

## Starting over

- Delete the fixture folders in Confluence (and their `Archive` folders)
  by hand; the export never deletes.
- `just api dev-nuke` drops the database, so recorded page ids go too.
- `just confluence-fixtures-push` resets every repository.

## Recording a run

Record each repository's status line in IMPL-0024's live-run task (or the
IMPL of whatever change is being checked), and fill RUNBOOK-0003's Last
Verified row with the date, the PR, the commit the server ran, and who ran
it.
