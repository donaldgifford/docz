# Local development against a real docz-api

The dev server proxies `/api`, `/auth`, and `/openapi.yaml` to a local
docz-api on `:8080` (override with `DOCZ_API_URL`), so the browser and
the API stay same-origin — exactly the deployed contract.

## Fixture-only mode

No API running? `bun run dev:msw` serves the site against the MSW
demo-org fixtures — real docz markdown, deterministic identity, and
(since IMPL-0005) the published-pages surface you are reading now.

## The container pair

`just local-up` builds the site image and joins it to the docz-api
local stack's network on `:8090`; re-run it after changes — the
recipe rebuilds and recreates the container.

```sh
just local-up
just local-down
```

> [!NOTE]
> This page is a demo fixture (a snapshot in `src/mocks/content/`): a
> nested file page published from the `api:` block, exercising the
> `guides/local-dev.md` published-path shape end-to-end.
