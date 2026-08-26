---
id: IMPL-0006
title: "Error observability: log every failure, self-heal the queue, optional no-auth mode"
status: In Progress
author: Donald Gifford
created: 2026-08-22
---
<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL 0006: Error observability: log every failure, self-heal the queue, optional no-auth mode

**Status:** Draft
**Author:** Donald Gifford
**Date:** 2026-08-22

<!--toc:start-->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: Queue failure visibility — ErrorHandler, Logger, comment fix](#phase-1-queue-failure-visibility--errorhandler-logger-comment-fix)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2: Queue self-heal — un-swallow the archived-task conflict](#phase-2-queue-self-heal--un-swallow-the-archived-task-conflict)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 3: Boot-time GitHub App credential self-check](#phase-3-boot-time-github-app-credential-self-check)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 4: The four mute HTTP sinks](#phase-4-the-four-mute-http-sinks)
    - [Tasks](#tasks-3)
    - [Success Criteria](#success-criteria-3)
  - [Phase 5: AUTH_PROVIDERS=none — first-setup no-auth mode](#phase-5-auth_providersnone--first-setup-no-auth-mode)
    - [Tasks](#tasks-4)
    - [Success Criteria](#success-criteria-4)
  - [Phase 6: Deliberate-failure verification, deploy, close-out](#phase-6-deliberate-failure-verification-deploy-close-out)
    - [Tasks](#tasks-5)
    - [Success Criteria](#success-criteria-5)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Dependencies](#dependencies)
- [Open Questions](#open-questions)
- [References](#references)
<!--toc:end-->

## Objective

Implement INV-0007's recommendations so that **no error terminates in a sink
that cannot report it**, the queue **recovers on its own** after retry
exhaustion, credential problems surface **at boot**, and first setup can
optionally skip login auth entirely. The acceptance bar is INV-0007's own
origin story run in reverse: a deliberately failing ingest must be fully
diagnosable **from structured logs alone** — no `redis-cli`, no asynq CLI, no
decoding a stored task record — and a fixed root cause must self-heal on the
next trigger without an operator touching Valkey.

**Implements:** INV-0007 recommendations 1–6 (F1–F7)

## Scope

### In Scope

- `asynq.Config.ErrorHandler` + `Logger` (slog adapter) + `LogLevel` in
  `internal/queue`; correct the false comment at the `handleIngest` failure
  site; raise the coalesce log to Info.
- Archived-task conflict handling on the enqueue path (`Inspector` check →
  delete + re-enqueue), killing the 90-day silent swallow (F4).
- Boot-time GitHub App self-check (`AppsTransport` JWT → `GET /app`) in
  `internal/githubapp`, wired into `run()`.
- Logging at the four mute HTTP sinks (F7): `authorize` middleware, `session`
  middleware (with `ErrSessionNotFound` discrimination), `webhook` body-read +
  payload-parse, `writeJSON` in `httpapi`/`authhttp`.
- `AUTH_PROVIDERS=none` (F6): config validation, anonymous-session gate,
  chart gating, docs. SPA untouched by design.
- Deliberate-failure verification + the EKS redeploy that closes INV-0007
  OQ-1 from logs.

### Out of Scope

- The active-window drop's *mechanical* fix (INV-0007 OQ-5) — this plan makes
  it **visible** (Info log), not impossible; see OQ-6 below.
- Periodic full resync / webhook replay (the standing backstop idea from the
  funnel incidents) — separate feature, separate doc.
- RFC 7807 error envelopes, log sampling, or any log-pipeline work.
- Tailscale/ingress topology changes (handled elsewhere).

## Implementation Phases

### Phase 1: Queue failure visibility — ErrorHandler, Logger, comment fix

The one-change-site fix for F1 + F7.5: every failed attempt logs through
slog, and asynq's internal errors stop bypassing the structured pipeline.

#### Tasks

- [x] Add an slog-backed `asynq.Logger` adapter in `internal/queue`
      (five methods, `Debug`…`Fatal`; map `Fatal` → `slog.Error` since slog
      has no fatal level and asynq only calls it during startup config
      validation). Set `Logger` + `LogLevel` (map from `LOG_LEVEL`) in the
      `asynq.Config` at `worker.go:54-59`.
- [x] Add `ErrorHandler: asynq.ErrorHandlerFunc(...)` in the same `Config`
      block: `slog.Error("ingest job failed", "task_id", …, "type",
      task.Type(), "err", err)` — unmarshal the payload for `repo`/`reason`
      attrs when it parses (best-effort; never fail the handler on a
      malformed payload).
- [x] Rewrite the false comment at `worker.go:106-107` to state the real
      contract: the returned error reaches asynq, which hands it to **our**
      `ErrorHandler` (F1: asynq itself never logs it).
- [x] Raise the coalesce log at `client.go:95` from Debug to Info and extend
      its message to name the active-window caveat (`client.go:70-73`), so
      the one place a push can vanish is visible at default log level.
- [x] Unit tests: capture slog output (swap `slog.SetDefault` handler for a
      recording one) — a failing fake ingestor produces an ErrorHandler line
      per attempt carrying repo + error; the Logger adapter routes asynq
      levels to slog levels.

#### Success Criteria

- A job that fails N attempts emits N structured `ingest job failed` lines
  (repo, reason, error) plus the terminal archive WARN — proven by a queue
  integration test asserting on captured log records.
- `LOG_FORMAT=json` produces **zero** non-JSON lines from the queue
  subsystem under a Redis outage (asynq's "Dequeue error" path now routes
  through the adapter) — proven by stopping the testcontainer Redis
  mid-test and scanning stderr.
- `just ci` green.

**Phase 1 complete (2026-08-24).** All five tasks done; `just ci` green;
`golangci-lint --build-tags=integration` clean. Two criteria were met by a
different proof than written, deliberately:

- **"N lines for N attempts"** is proven for the **first** attempt only
  (`TestFailedIngestLogsTheError`). asynq's default retry backoff is tens of
  seconds per step, so asserting all five would make the test run for
  minutes. The hook is per-attempt by construction (asynq calls
  `ErrorHandler` from `handleFailedMessage`, on every failure including the
  last), so one real attempt through real Redis proves the wiring; the
  multi-attempt lifecycle is covered end-to-end by the Phase 6 drill.
- **JSON purity** is proven structurally rather than by scanning stderr:
  `TestAsynqInternalErrorsReachSlog` points a worker at an unreachable Redis
  and asserts `component=asynq` records arrive through slog. Setting
  `Config.Logger` means asynq has no path to stderr at all, so this asserts
  the cause rather than the symptom — and it avoids stopping the container
  `TestMain` shares with every other case in the package.

### Phase 2: Queue self-heal — un-swallow the archived-task conflict

Kills F4: retry exhaustion must never silently eat future triggers.

#### Tasks

- [x] Add an `*asynq.Inspector` to `queue.Client` (same parsed redis opts;
      close it in `Client.Close` via the existing `errors.Join`).
- [x] On `ErrTaskIDConflict`/`ErrDuplicateTask` in `EnqueueIngest`: call
      `Inspector.GetTaskInfo(queueName, taskID)`. State
      pending/scheduled/retry → today's coalesce (now Info, Phase 1). State
      **archived** → `DeleteTask` + re-enqueue once (single retry, no loop);
      log Info `"archived ingest task cleared; re-enqueued"`. State
      **active** → keep coalesce semantics but log the drop caveat (OQ-6
      defers the mechanical fix). `GetTaskInfo` error → log Warn, treat as
      coalesced (fail open: never turn a webhook 202 into a 500 over an
      inspection).
- [x] Unit test the state-dispatch with a fake inspector seam (consumer-side
      interface, matching house style).
- [x] Integration test (`queue_integration_test.go`, real Redis): drive a
      task to archived via a failing fake ingestor, then enqueue the same
      repo → assert the archived task is deleted, a fresh task runs, and
      the success path completes. Assert the pending-state coalesce still
      works (existing burst test keeps passing).

#### Success Criteria

- The INV-0007 F4 scenario is dead: after retry exhaustion, the **next**
  trigger ingests (proven by the new integration test), with an Info line
  explaining what happened.
- No behavior change for the healthy coalesce path (existing debounce/burst
  integration tests unchanged and green).

**Phase 2 complete (2026-08-24).** All four tasks done; unit + integration
suites and `golangci-lint --build-tags=integration` green.

**Scope grew during implementation.** Probing the conflict path turned up a
second, worse instance of the same trap, now recorded as INV-0007 **F4b**:
`Retention(24h)` made asynq keep the task key after every *successful* run
(`markAsComplete` HSETs the key; the no-retention path DELs it), so the
completed state held the repo's id for 24 hours and every push in that
window was silently dropped — **one ingest per repo per day, on the happy
path**. `TestReingestAfterSuccessfulRun` fails against the pre-fix code.

The fix therefore covers both terminal states rather than archived alone,
and drops `Retention` so a success frees the id immediately; the inspector
path still clears ids left behind by earlier deployments. The decision is a
pure `classifyConflict` function so the state table is unit-tested directly
rather than through a re-implementation.

`TestReingestAfterArchivedRun` archives the task via the inspector instead
of waiting out five retries of asynq's default backoff, which would take
minutes.

### Phase 3: Boot-time GitHub App credential self-check

F5's fix: bad App credentials fail the deploy, not the fifth silent retry.

#### Tasks

- [x] Add `githubapp.SelfCheck(ctx, appID, pemKey, apiBase) (slug string,
      err error)`: `ghinstallation.NewAppsTransport` (app JWT, not an
      installation transport) → go-github `Apps.Get(ctx, "")` → return the
      app slug. Honor `apiBase` exactly as `NewClient` does.
- [x] Wire into `run()` after config load, bounded by a short context
      (reuse the `oidcDiscoveryTimeout` precedent): log
      `"github app authenticated" slug=…` on success; on failure apply the
      OQ-1 decision (recommendation: fail boot on credential rejection,
      warn-and-continue on transport errors, discriminated via
      `*github.ErrorResponse` status).
- [x] Skip the check in `-migrate` mode (no GitHub involvement); run it for
      serve and `-onboard`.
- [x] Unit tests with a stub `http.RoundTripper` (house pattern from
      `githubapp`): 200 → slug; 401 → credential-shaped error; connection
      refused → transport-shaped error.
- [x] Document the health-check trio in `deploy/README.md` (and the chart
      README's probes note): `/healthz` = liveness ("pod is up", bare
      200); `/readyz` = readiness ("dependencies connected, can accept
      traffic" — postgres/meili/redis, 503 names the offender); the boot
      self-check = startup credential validation (crashes the boot on bad
      creds, in **neither** probe — GitHub down must not pull the read
      API from rotation).

#### Success Criteria

- A deploy with a mangled PEM or wrong app id logs one unambiguous
  credential error at boot and (per OQ-1a) exits nonzero — observable in
  `kubectl logs` within seconds of pod start, no ingest attempt needed.
- A valid deploy logs the authenticated app slug once at startup.
- GitHub being temporarily unreachable does not crash-loop an otherwise
  healthy API (transport errors warn only).

**Phase 3 complete (2026-08-24).** All five tasks done; `just lint` and the
full unit suite green.

Implementation note beyond the plan: a malformed PEM fails inside
`ghinstallation.NewAppsTransport` — before any HTTP request — so that path is
classified as `ErrCredentialsRejected` too. It is the single most likely
credential fault when moving between secret stores, and checking only the
API response would have missed it entirely. Rate limiting is excluded from
the rejection class explicitly (`*github.RateLimitError` /
`*github.AbuseRateLimitError` arrive as 4xx but are transient), so throttling
can never be mistaken for a bad key and crash-loop a deploy.

### Phase 4: The four mute HTTP sinks

F7's fix: every wrapped error chain now terminates in a logging sink.

#### Tasks

- [x] `internal/authorize/authorize.go:52` — `slog.Error("authorize failed",
      "err", err)` before the 500. (The chain carries the store context;
      nothing else needed.)
- [x] `internal/session/middleware.go:33-35` — discriminate:
      `errors.Is(err, ErrSessionNotFound)` → 401 quietly (expected churn);
      anything else (Redis down, corrupt JSON) → `slog.Error` + the OQ-2
      status decision (recommendation: 503 with the standard error
      envelope, because "session backend down" is not "logged out").
- [x] `internal/webhook/webhook.go:96` — log Warn on body-read failure
      (names the `MaxBytesReader` cap case); `:126` — log Error on
      `ParseWebHook` failure with the event header (post-HMAC ⇒ provably
      GitHub ⇒ schema drift or unsubscribed event).
- [x] `internal/httpapi/handler.go:112-113` and
      `internal/authhttp/handler.go:90-91` — route marshal failures through
      the existing `serverError` helpers (which log) instead of the silent
      `writeError`/inline 500. Leave the error-envelope-marshal fallback
      (`authhttp:105`) as-is but add a `slog.Error` (practically
      unreachable; one line buys completeness).
- [x] Unit tests per sink with a recording slog handler: erroring fake
      authorizer / lookuper / marshal-poisoned DTO (`json.RawMessage` with
      invalid bytes) each produce exactly one error record; the
      session-middleware test pins 401-stays-quiet for
      `ErrSessionNotFound`.
- [x] Contract test still green; if OQ-2a (503) is chosen, confirm the spec
      needs no change (the contract test exercises defined scenarios only;
      5xx paths are not specced per-op today).

#### Success Criteria

- A Postgres outage on `/api/v1` and a Redis outage on the session gate
  each produce structured error logs naming the failing dependency (proven
  by unit tests; observed live in Phase 6).
- Review of the four cited sites shows no error path that neither logs nor
  delegates to a logging helper; `just lint` green.
- 401 behavior for genuinely missing/expired sessions is byte-identical
  (contract test unchanged on that path).

**Phase 4 complete (2026-08-24).** All six tasks done; `just lint` (0 issues)
and the full unit suite — including the OpenAPI contract test — green.

Implementation notes beyond the plan:

- The session-gate outage status is a new **503 `{"error":"session
  unavailable"}`** (OQ-2a), written by a dedicated `writeSessionUnavailable`
  helper alongside the existing 401 one. The distinction matters downstream:
  docz-site drives its login UI off 401, so answering 401 during a Redis blip
  would bounce every signed-in user to the login screen for the duration of
  the outage. The 401 path for a genuinely absent session is byte-identical.
- The webhook body-read sink logs at **Warn with headers only** — that
  failure happens *before* HMAC verification, so the payload is unverified
  and must not be logged. The `ParseWebHook` sink logs at Error and says so
  in its comment, because HMAC has already passed there: the payload
  provably came from GitHub, making it schema drift rather than a bad caller.
- The `writeJSON` marshal sinks in `httpapi` and `authhttp` now route through
  each package's existing `serverError` helper rather than a new one, so the
  response envelope and the log line stay in one place per package.
- No spec change was needed: the contract test exercises defined scenarios
  only, and 5xx paths are not specced per-op today.

### Phase 5: AUTH_PROVIDERS=none — first-setup no-auth mode

F6: one knob to run the read surface open, so first setup fights only the
GitHub App credentials. Loud opt-in, documented as such.

#### Tasks

- [x] `internal/config`: accept the literal `none` in `AUTH_PROVIDERS`
      (OQ-3 spelling). `validate.go`: under `none` (which must be the
      *only* entry — `none,okta` is a config error), skip provider
      credential checks, `AUTH_REDIRECT_BASE`, and `SESSION_SECRET`
      requirements. Add an `AuthDisabled()` helper alongside
      `AuthEnabled`.
- [x] `cmd/docz-api`: when disabled, `buildAuthProviders` returns an empty
      registry; `runServer` swaps `session.Middleware` for an
      anonymous-identity injector (synthetic `session.Session` with
      provider `none`, subject `anonymous`, injected via the same ctx key
      so `FromContext` and `/api/v1/auth/session` work unchanged) composed
      over the same `authorize.Middleware`. Skip `MountPublic` (no
      `/auth/login`/`/auth/callback` routes — 404, the absent-route
      precedent from search; OQ-4). Log one startup Warn:
      `"auth disabled (AUTH_PROVIDERS=none): the read API is open"`.
- [x] Session store: still constructed (Redis is required anyway for the
      queue) but the cookie path is never exercised; logout becomes a no-op
      revoke on the synthetic id — verify it 200s harmlessly rather than
      500s.
- [x] Chart: gate the `required` on `config.authRedirectBase` (and the
      session-secret Secret key) off `has "none" $providers`; extend the
      `docz-api.authProviders` helper docs; `values.yaml` comment block +
      README.md.gotmpl section documenting the exposure caveat;
      helm-unittest cases (none-mode renders no provider env, no
      oauth/okta/keycloak Secret keys, no authRedirectBase requirement).
- [x] Contract test: a none-mode handler variant asserting `/api/v1` reads
      succeed with no cookie and `/api/v1/auth/session` returns the
      synthetic identity (spec shape unchanged — `sessionDTO` already fits;
      no spec version bump expected, confirm during implementation).
- [x] Docs: `deploy/README.md` first-setup section rewritten to lead with
      none-mode ("get ingest working first, add login after"), plus the
      revert path (set real providers, redeploy).

#### Success Criteria

- `AUTH_PROVIDERS=none` with **no** session secret, redirect base, or
  provider credentials boots clean, serves `/api/v1` reads without a
  cookie, and returns the synthetic identity on `/api/v1/auth/session` —
  proven by the contract-test variant.
- Default (`github`) behavior is byte-identical: full test suite + contract
  test green with no fixture churn outside the new none-mode cases.
- `just helm-unittest` green with the new cases; a none-mode
  `helm template` renders no provider env vars and no provider Secret keys.
- The docz-site, pointed at a none-mode API, renders docs with no login
  panel (manual smoke; the SPA's login UI is 401-driven and none-mode
  never 401s).

**Phase 5 complete (2026-08-25).** All six tasks done; `just lint` (0 issues),
the full unit suite, `just helm-lint` / `helm-template` / `helm-unittest`
(80 tests) and `just lint-openapi` (100/100) all green. Chart bumped to
**0.5.0**, spec to **1.2.1** (editorial — see below).

The one success criterion that cannot be proven here is the docz-site smoke
("no login panel against a none-mode API") — it needs a browser against a
running pair. It is carried into Phase 6's live verification rather than
claimed.

Implementation notes beyond the plan:

- `AuthDisabled()` requires `none` to be the **only** entry; `none,github` is
  a config error naming the conflict, not a silent precedence rule. Two
  incompatible shapes cannot both be described at once, and guessing which
  one the operator meant is exactly the kind of ambiguity that produces a
  "why is my API open?" incident.
- `session.AnonymousMiddleware` lives in `internal/session` because the
  context key is unexported there — which is the point: the synthetic session
  goes in through the *same* key as a real one, so `authorize`, every handler,
  and `/api/v1/auth/session` need no no-auth branch. The anonymous identity is
  `provider "none" / subject "anonymous"`, and its session id is never in
  Redis, so logout is a harmless no-op delete of an absent key (verified, not
  assumed).
- **The spec gained an editorial note** (`1.2.1`) rather than staying silent.
  A generated client works against either mode — only the auth requirement
  changes, never a request or response shape — and saying so in the
  `sessionCookie` description is what makes that safe to rely on. No version
  bump beyond patch: nothing about the wire contract changed.
- The chart gates on a new `docz-api.authDisabled` helper so both the
  Deployment env and the Secret key list stay in one decision. Every surface
  that documents none-mode (values.yaml, chart README, `deploy/README.md`,
  `.env.production.example`) also states the exposure — an open read API is
  the whole trade, and it should not be discoverable only by reading
  templates.

### Phase 6: Deliberate-failure verification, deploy, close-out

INV-0007's origin story, rerun against the fixed binary — the phase that
proves "we are actually covering the error exposure required" instead of
assuming it.

#### Tasks

- [x] Local deliberate-failure drill (compose stack): onboard a repo whose
      fetch **must** fail (no `.docz.yaml`), watch `just run` logs only:
      confirm N per-attempt error lines with the real cause, the terminal
      archive WARN, and — after "fixing" the repo — a next trigger that
      self-heals through the Phase 2 path. **No Redis inspection allowed**;
      if any step needs it, that's a Phase 1/2 bug to fix before shipping.
- [x] Repeat the drill with `LOG_FORMAT=json` piped through `jq -e` to
      prove every queue-subsystem line parses (Phase 1's second criterion,
      end to end).
- [x] Release: minor version bump (new env mode + logging surface), chart
      version bump for the Phase 5 template changes; `pr-semver` label
      `minor`.
- [ ] **deferred - blocked, no cluster available** — redeploy the new image +
      chart; re-trigger the INV-0007 failing repo; **close INV-0007 OQ-1 from
      the logs** — record the actual root cause in INV-0007 (edit the OQ, note
      "answered via the Phase 6 deploy"). Confirmed blocked on 2026-08-26, not
      assumed: no reachable cluster context (only the unreachable homelab
      ones) and no AWS credentials, and the operator confirmed there is no
      cluster to use at present. INV-0007's runbook path — reading the archived
      task straight out of Valkey — needs the same access, so OQ-1 cannot be
      answered by any route until a cluster exists. Nothing in the branch
      depends on this; it is the last verification step, not a prerequisite.
- [x] Downstream: file the docz-site configuration issue for the auth/no-auth
      surface — donaldgifford/docz-site#17 (none-mode login UI, the new 503
      versus 401 split, re-vendoring the spec at `1.2.1`).
- [x] Close-out: mark INV-0007 recommendations 1–6 as implemented in the
      INV; update CLAUDE.md (new conventions: asynq Logger/ErrorHandler
      contract, SelfCheck at boot, none-mode); `docz update`.

#### Success Criteria

- The full failure lifecycle — first failure, every retry, exhaustion,
  root-cause identification, fix, self-heal — is reconstructed **from
  structured logs alone** in both drills. Valkey is never opened.
- INV-0007 OQ-1 is answered and recorded, from production logs, not from
  the stored task record.
- CI green, release published, EKS running the new version with the
  failing repo ingested (or its genuine blocker named in the logs).

**Phase 6 complete except the EKS redeploy (2026-08-25).** Everything that
can be proven without cluster access is proven; the one remaining task is
marked `deferred - human required` above.

**The drill (both tasks 1 and 2, run as one `LOG_FORMAT=json` session).**
Local compose stack (real Postgres/Redis/Meilisearch) plus the real
`donaldgifford-docz-api` GitHub App. `-onboard
donaldgifford/no-such-docz-repo@145915803` forced a permanent fetch failure.
Reading **only** `/tmp/drill.log` — Valkey was never opened — the full
lifecycle was reconstructed:

- boot: `github app authenticated app_slug=donaldgifford-docz-api` (fix 2)
- 6 x `ingest job attempt failed` at ERROR, `retried` 0→5, each naming the
  real cause: `GET https://api.github.com/repos/donaldgifford/no-such-docz-repo:
  404 Not Found` (fix 1)
- `WARN Retry exhausted for task id=ingest:donaldgifford/no-such-docz-repo`
  with `component=asynq` — the operator's original production line, now
  preceded by six lines that say *why* (fix 5)
- next trigger: `cleared a finished ingest task and re-enqueued
  cleared_state=archived` — the swallow is gone (fix 3)
- a healthy repo ingests: `ingest job complete docs_upserted=16`
- a 5-trigger burst: `ingest job enqueued` once, four
  `ingest job coalesced into an existing task` at **Info** carrying the live
  `state` (`active` / `scheduled`) (fix 5 / F7.6)

`jq -e . /tmp/drill.log` succeeded on all 33 lines, so every queue-subsystem
line — including asynq's own internals — parses as structured JSON.

**Post-phase review (2026-08-25).** Two independent reviewers went over the
whole branch diff. They found five real defects, all now fixed with regression
tests; both reviewers independently found the first one, which is why it is
listed first:

1. **The chart's `authDisabled` helper used `has "none"`** (membership) where
   the binary requires `none` to be the *only* entry. `authProviders:
   "none,github"` therefore rendered a manifest that dropped `SESSION_SECRET`
   and `AUTH_REDIRECT_BASE` while still emitting `GITHUB_OAUTH_*` — it
   installed cleanly and then crash-looped on the binary's own validation. The
   helper now `fail`s at render time with the same message, which is the house
   rule (catch it at `helm install`, not in CrashLoopBackOff).
2. **`isCredentialRejection` classified any 4xx as permanent.** go-github only
   produces a typed `*RateLimitError` when the response carries the matching
   headers, so a throttled response from a proxy, CDN, or GHES front end
   arrived as a plain `*ErrorResponse` and failed startup — precisely the
   crash-loop the boot check is supposed to prevent. A first attempt excluded
   `408`/`429` by status code; **a third review pass found that incomplete**,
   because GitHub documents `403` as a primary rate-limit status too, so a bare
   `403` still crash-looped. The permanent set is now **401 only**: `GET /app`
   authenticates the App with a JWT and needs no permissions, so a bad key is
   the only thing 401 can mean there. The two misclassifications are not
   symmetric — calling a transient failure permanent takes down the read API
   for an ingest-only problem, while calling a permanent one transient costs a
   single startup warning now that every ingest attempt logs its own cause.
3. **`DeleteTask` returning `ErrTaskNotFound` aborted the clear-and-retry.**
   Two pushes seconds apart both classify `clearAndRetry`; the loser found the
   task already gone and answered 500. Because `RecordDelivery` had already
   recorded the delivery id, GitHub's redelivery was deduped to a no-op rather
   than retrying. "Already gone" is the state this path wants, so it now falls
   through to the re-enqueue.
4. **An undecodable session was a 503 dead end.** `Lookup` wraps a JSON decode
   failure, which the gate treated as an outage — but `/api/v1/auth/logout`
   sits behind that same gate, and docz-site only offers login on a 401, so the
   holder could never recover. A new `ErrSessionCorrupt` sentinel keeps that
   case on the 401 path (logged at Warn); only a backend that cannot answer
   gets the 503.
5. **`resolveTaskIDConflict` dereferenced a nil `TaskInfo`** that
   `classifyConflict` explicitly accepts. Unreachable through the real
   `*asynq.Inspector`, but the seam exists to be swapped, and the test suite
   pins `(nil, nil)` as supported.

A third pass adversarially verified those five fixes rather than trusting
them. It confirmed four sound (checking asynq's actual `ErrTaskNotFound`
sentinel, Go's double-`%w` behavior, every legitimate chart provider
combination, and that no nil path remained) and found fix 2 incomplete, which
is folded into item 2 above. It also caught that the corrupt-session test
built its error with a single `%w` while production uses two — a test weaker
than the code it guards, now pinned to the real shape.

Each of the four fixes with a new test was then proven by reverting the
production change and confirming the test actually fails — a test that passes
against the broken code proves nothing. All four failed as intended. That
exercise found one more gap worth recording: the corrupt-session fix has two
halves, and reverting the **producer** half alone (`Store.Lookup` no longer
labelling the error) left the whole `internal/session` package green, because
the middleware test injects the sentinel itself through a fake. A refactor of
`Lookup`'s error wrapping would therefore have pushed corrupt sessions back
onto the 503 dead end with a fully passing suite.
`TestLookupCorruptValueIsLabelled` (integration, real Redis: seed a non-JSON
value, assert `ErrSessionCorrupt` and *not* `ErrSessionNotFound`) closes it,
and was itself verified by the same revert drill.

**Integration suites.** `just test-integration` (the `//go:build integration`
tests, which CI does not run) is green across all 16 packages, including the
testcontainers-backed `queue` (real Redis — debounce, drain, and the new
archived-task self-heal), `store`, `search`, and `e2e` suites.

**Release.** Chart `0.5.0` (Phase 5 template changes) and OpenAPI `1.2.1`
(editorial). The binary version comes from the release tag, so the `minor`
bump is the tag itself plus the `minor` semver label on the PR — both are
release-time actions, not repo edits.

**What the EKS step is still for.** INV-0007 OQ-1 (the actual root cause of
the triggering production failure) is the one question that cannot be
answered locally: it needs the fixed binary running in the cluster and one
re-trigger of the failing repo. With fix 1 in place the cause now appears
directly in `kubectl logs`. The same deploy covers the docz-site none-mode
smoke carried over from Phase 5.

## File Changes

| File | Change |
| ---- | ------ |
| `internal/queue/worker.go` | `Logger`/`LogLevel`/`ErrorHandler` in Config; comment fix |
| `internal/queue/logger.go` (new) | slog-backed `asynq.Logger` adapter |
| `internal/queue/client.go` | Inspector; conflict state-dispatch; coalesce log Info |
| `internal/queue/queue_integration_test.go` | archived self-heal + log-capture cases |
| `internal/githubapp/selfcheck.go` (new) | `SelfCheck` via `AppsTransport` + `Apps.Get` |
| `cmd/docz-api/main.go` | boot self-check; none-mode gate swap; startup Warn |
| `cmd/docz-api/auth.go` | empty registry under none-mode |
| `internal/config/validate.go` | `none` semantics; conditional requireds |
| `internal/config/config.go` | `AuthDisabled` helper |
| `internal/authorize/authorize.go` | log before 500 |
| `internal/session/middleware.go` | error discrimination + log; OQ-2 status |
| `internal/webhook/webhook.go` | body-read Warn; parse Error |
| `internal/httpapi/handler.go` | `writeJSON` → `serverError` |
| `internal/authhttp/handler.go` | `writeJSON` → `serverError`; envelope fallback log |
| `internal/httpapi/openapi_contract_test.go` | none-mode handler variant |
| `charts/docz-api/templates/*` + `values.yaml` + tests | none-mode gating + unittest cases |
| `deploy/README.md`, `charts/docz-api/README.md.gotmpl` | none-mode + caveat docs |
| `CLAUDE.md`, `docs/investigation/0007-*.md` | conventions; OQ-1 answer |

## Testing Plan

- **Log assertions** are the novel surface: a small recording
  `slog.Handler` helper (swapped via `slog.SetDefault`, restored per test)
  shared by the queue/middleware/sink tests. Standard library only, per
  house rules.
- **Queue**: unit (adapter level-mapping, ErrorHandler attrs, conflict
  state-dispatch on a fake inspector) + integration (real Redis: archived
  self-heal, burst coalesce regression, JSON-purity under Redis outage).
- **githubapp**: stub RoundTripper trio (200 / 401 / connection refused).
- **Sinks**: one unit test per sink asserting exactly one error record and
  unchanged HTTP behavior (except the OQ-2 decision, pinned by its own
  test).
- **none-mode**: contract-test variant (open reads + synthetic session) and
  helm-unittest render cases; default-mode fixtures untouched proves
  no-regression.
- **Phase 6** is manual-but-scripted: both drills recorded in the PR
  description with log excerpts.

## Dependencies

- No new Go modules: `asynq` (ErrorHandler/Logger/Inspector),
  `ghinstallation/v2` (`NewAppsTransport`), go-github (`Apps.Get`) are all
  already direct deps — verified against the module cache
  (asynq `v0.26.0`, ghinstallation `v2.19.0`).
- Chart changes ride the existing helpers (`docz-api.authProviders`).
- Phase 6's EKS step needs the cluster access already in use for testing.

## Open Questions

Numbered for review; `a` is the recommendation. Reply with a letter per
question, or "other: …".

**All answered `a` (2026-08-22).** OQ-1a comes with a probe-semantics note
from review: the operator asked whether `/healthz` vs `/readyz` should be
made more explicit — `/healthz` = "pod is up", `/readyz` = "dependencies
connected, can accept traffic". That is exactly the current semantics
(liveness is a bare 200; readiness checks postgres/meili/redis and 503s
naming the offender), so no endpoint changes — but the distinction gets
**documented**, and the boot self-check is deliberately a *third* thing:
startup credential validation, in **neither** probe. GitHub stays out of
`/readyz` on purpose (readiness gates traffic routing; the read API serves
fine with GitHub down), and a credential failure crashes the boot rather
than flapping readiness. A doc task was added to Phase 3.

1. **Boot self-check failure mode (Phase 3)** — what does a failed
   `GET /app` do to startup?
   - **a. Discriminate: credential rejection (4xx from GitHub) fails the
     boot; transport errors (DNS, refused, timeout) log Warn and
     continue.** Bad creds are permanent and deserve a crash-loop an
     operator sees immediately; a GitHub blip is transient and should not
     take down an otherwise healthy read API. *(Recommended.)* **Answered: (a).**
   - b. Always fail fast — simplest, matches the OIDC-discovery precedent
     exactly, but a GitHub outage at pod-restart time crash-loops the
     whole API.
   - c. Always warn-only — never blocks a deploy, but a mangled key still
     reaches the first real ingest before anyone notices (a softer version
     of today's F5).

2. **Session-lookup infra errors (Phase 4)** — Redis down / corrupt JSON
   currently 401s. After logging, what status?
   - **a. 503 with the standard `{"error":…}` envelope.** Honest ("service
     can't answer authn right now" ≠ "you are logged out"), and it stops
     the SPA from bouncing users to the login panel during a Redis blip.
     5xx is not per-op specced, so no OpenAPI change. *(Recommended.)* **Answered: (a).**
   - b. Keep 401, just add the Error log — zero behavior change, but the
     SPA logs everyone out for the duration of any Redis hiccup.
   - c. 500 — same honesty as (a) with a less precise status code.

3. **No-auth switch spelling (Phase 5)** — how does the operator say "no
   login auth"?
   - **a. `AUTH_PROVIDERS=none`, required to be the sole entry.** One
     existing knob, reads naturally next to `github,okta`, impossible to
     enable by omission (the default stays `github`), and the chart's
     provider-gating helper handles it for free. *(Recommended.)* **Answered: (a).**
   - b. A separate `AUTH_DISABLED=true` boolean — more explicit, but two
     knobs can now contradict each other (`AUTH_DISABLED=true` +
     `AUTH_PROVIDERS=okta`) and both sides need precedence rules.
   - c. Empty `AUTH_PROVIDERS` means disabled — smallest change, but a
     typo'd provider list could silently open the API; `none` keeps
     "open" an explicit word.

4. **`/auth/login` + `/auth/callback` in none-mode (Phase 5)** — mounted
   or not?
   - **a. Not mounted → 404.** The absent-route precedent (the search
     route is absent when no searcher is wired); nothing advertises the
     endpoints, and the SPA never links them (its login UI is 401-driven
     and none-mode never 401s). *(Recommended.)* **Answered: (a).**
   - b. Mounted, returning 503 + an "auth disabled" envelope — friendlier
     to a human poking with curl, at the cost of a bespoke handler for a
     mode meant to be temporary.

5. **Synthetic identity shape (Phase 5)** — what does
   `/api/v1/auth/session` return in none-mode?
   - **a. `{provider: "none", subject: "anonymous"}` with empty
     email/login/groups**, via the normal `sessionDTO` — the SPA renders
     its session menu unchanged, no spec change. *(Recommended.)* **Answered: (a).**
   - b. 404/absent endpoint in none-mode — cleaner conceptually, but the
     SPA treats a session-endpoint failure as logged-out and may render
     login affordances that lead nowhere (404 per OQ-4a).

6. **The active-window drop (INV-0007 OQ-5)** — a trigger landing while
   the repo's job is *running* is still dropped (asynq holds the task id
   until completion). This plan logs it (Phase 1); fix it too?
   - **a. Defer to a follow-up.** Rare (requires a push inside the
     seconds-long ingest window), now visible at Info, and an eventual
     periodic-resync backstop erases the class entirely; a dirty-flag
     re-enqueue in the worker completion path is real complexity for a
     shrinking gap. *(Recommended.)* **Answered: (a).**
   - b. Fix now: on active-state conflict, re-enqueue with
     `ProcessIn(debounce)` under a *different* task id (e.g.
     `ingest:<repo>:followup`), accepting the second-slot bookkeeping.

## References

- INV-0007 — the findings this implements (F1–F7, recommendations 1–6)
- `docs/impl/0005-*.md` — house phase/OQ format precedent
- asynq `v0.26.0` — `server.go` (`ErrorHandler` :278, `Logger` :191),
  `inspector.go` (`GetTaskInfo` :240, `DeleteTask` :617),
  `processor.go:335-349`, `internal/rdb/rdb.go:98-108,841`
- ghinstallation `v2.19.0` — `appsTransport.go:48` (`NewAppsTransport`)
- CLAUDE.md — Phase 4 queue conventions; Phase 6 auth architecture
- docz-site `src/api/fetcher.ts` — 401-driven login UI (why the SPA needs
  no none-mode changes)
