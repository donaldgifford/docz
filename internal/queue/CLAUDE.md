# CLAUDE.md: `internal/queue`

Loaded when a session works under this directory. The repository's map is the root `CLAUDE.md`.

## Error observability + queue self-heal (INV-0007 / IMPL-0006)

Ingest failures used to be invisible: asynq never logged the handler's error,
and an exhausted task blocked every future trigger for that repo. INV-0007
(Concluded) diagnosed it and IMPL-0006 fixed it. Conventions established:

- **asynq's `Config` needs three fields wired, not one.** `Logger` (an slog
  adapter, `internal/queue/logger.go`, tagging `component=asynq`), `LogLevel`
  (derived from the slog logger's own enabled level), and `ErrorHandler`
  (`logIngestFailure`). **asynq logs a handler's returned error nowhere unless
  an `ErrorHandler` is registered** — the error is written only to the task
  record in Redis. Every failed attempt now logs the real cause plus task id,
  `retried`, `max_retry`, repo, and reason; the terminal `Retry exhausted`
  WARN arrives through the same pipeline.
- **A finished task must be cleared, not coalesced onto.** asynq's `TaskID`
  uniqueness is a bare `EXISTS`, so an **archived** (retry-exhausted) or
  **completed**-with-retention task keeps owning the id and every later
  enqueue returns `ErrTaskIDConflict` — which the old code treated as
  successful coalescing. `EnqueueIngest` now inspects the conflicting task
  (`taskInspector` seam over `*asynq.Inspector`): a live pending/scheduled/
  active task coalesces (logged at **Info** with its `state`), while a
  terminal one is deleted and re-enqueued once (`cleared_state`). **`Retention`
  was removed entirely** — with it set, a *successful* ingest blocked that
  repo for 24 h (F4b), which is a live-production bug, not a theoretical one.
- **The GitHub App is verified at boot, not by a probe.** `githubapp.SelfCheck`
  mints the App JWT and calls `GET /app`, logging the authenticated slug.
  **Only a 401 fails startup** (`ErrCredentialsRejected`), plus a malformed PEM,
  which fails inside `NewAppsTransport` before any request and is the most
  common real-world credential fault. Everything else — 403, 429, 408, 404, 5xx,
  transport errors — warns and continues. Do **not** widen this back to "any
  4xx": GitHub uses 403 for primary rate limiting as well as for a suspended
  App, and go-github only produces a typed `*RateLimitError` when the response
  carries `X-RateLimit-Remaining: 0`, so a 403 from a proxy, a CDN, or a GHES
  front end is indistinguishable from a refusal and would crash-loop a deploy
  whose key is fine. `GET /app` needs no permissions, so 401 is the only
  unambiguous bad-key signal. The two errors are not symmetric — a false
  "permanent" takes down the read API for an ingest-only problem, while a false
  "transient" costs one startup warning, and every ingest attempt now logs its
  own cause. **Probe trio:** `/healthz` = liveness (checks
  nothing downstream — a restart cannot fix a dependency), `/readyz` =
  readiness (postgres/redis/meili, 503 names the offender), boot self-check =
  neither (GitHub is not a serving dependency).
- **Every error path terminates in a logging sink.** The recurring bug was a
  wrapped error chain reaching the client as a bare status code and dying
  there. `session.Middleware` splits on whether the caller has a usable session,
  not on whether an error occurred: `ErrSessionNotFound` (ordinary churn) and
  `ErrSessionCorrupt` (the value exists but will not decode) are both **401**,
  the latter logged at Warn; only a backend that cannot answer at all gets
  **503** `{"error":"session unavailable"}`. Both halves matter. 503 for a Redis
  blip stops docz-site — which drives its login UI off 401 — from reading an
  outage as a logout; 401 for a corrupt session avoids a dead end, since
  `/api/v1/auth/logout` sits behind this same gate, so a 503 there would leave
  the holder unable to clear the bad cookie or log in again. `Store.Lookup` is
  the producer of both sentinels and is tested as such — testing only the
  middleware leaves a refactor free to drop the label and silently restore the
  dead end. `authorize` logs before its 500;
  `webhook` logs a body-read failure at **Warn with headers only** (pre-HMAC,
  so the payload is unverified) and a `ParseWebHook` failure at **Error**
  (post-HMAC, so it is schema drift, not a bad caller); both `writeJSON`
  marshal failures route through their package's `serverError` helper.
- **`AUTH_PROVIDERS=none`** is the login-free first-setup mode (`config
  .AuthDisabled()`; `none` must be the only entry). `session
  .AnonymousMiddleware` injects a synthetic `Session` (`provider "none"` /
  `subject "anonymous"`) through the **same unexported ctx key**, so
  `authorize`, every handler, and `/api/v1/auth/session` need no no-auth
  branch and the wire contract is unchanged. `/auth/login` + `/auth/callback`
  are left unmounted (404). The chart gates on `docz-api.authDisabled`;
  values.yaml, both READMEs, and the spec all state the exposure — the read
  API is open to anyone who can reach the Service.
- **The verification standard is "reconstruct it from logs alone."** IMPL-0006
  Phase 6 ran the full failure lifecycle against real Postgres/Redis/Meili and
  a real GitHub App — first failure, five retries, exhaustion, root cause,
  fix, self-heal, successful ingest — reading only `LOG_FORMAT=json` output
  piped through `jq -e`. Valkey was never opened. Re-run that drill after any
  change to the queue's failure path.
