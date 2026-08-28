---
id: DESIGN-0003
title: "Support docz-api's no-auth mode and the session-unavailable 503"
status: Implemented
author: Donald Gifford
created: 2026-08-27
---

<!-- markdownlint-disable-file MD025 MD041 -->

# DESIGN-0003: Support docz-api's no-auth mode and the session-unavailable 503

**Status:** Implemented
**Author:** Donald Gifford
**Date:** 2026-08-27

<!--toc:start-->
- [Overview](#overview)
- [Goals and Non-Goals](#goals-and-non-goals)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Background](#background)
  - [What docz-api changed](#what-docz-api-changed)
  - [Audit: how the site behaves today](#audit-how-the-site-behaves-today)
- [Detailed Design](#detailed-design)
  - [Component 1: session-state classification](#component-1-session-state-classification)
  - [Component 2: none-mode chrome](#component-2-none-mode-chrome)
  - [Component 3: the 503 typed error](#component-3-the-503-typed-error)
  - [Component 4: spec re-vendor at 1.2.1](#component-4-spec-re-vendor-at-121)
  - [Cross-cutting: security invariants](#cross-cutting-security-invariants)
- [API / Interface Changes](#api--interface-changes)
- [Data Model](#data-model)
- [Testing Strategy](#testing-strategy)
- [Migration / Rollout Plan](#migration--rollout-plan)
- [Open Questions](#open-questions)
- [References](#references)
<!--toc:end-->

## Overview

docz-api (spec `1.2.1`, upstream donaldgifford/docz-api#19) gained two
auth behaviors the site must handle: `AUTH_PROVIDERS=none`, a login-free
first-setup mode where every request is served as a synthetic anonymous
identity and the `/auth/login`–`/auth/callback` routes are not mounted;
and a split of session-gate failures into `401` (not signed in) versus
`503` (session backend unreachable). This design makes the site's auth
chrome mode-aware — no login or logout affordances on a none-mode
deployment, no "signed in as anonymous" noise — and teaches the client
that a `503` is a transient backend fault that must never read as a
logout. Tracked as issue [#17].

## Goals and Non-Goals

### Goals

- A none-mode deployment renders no login affordance anywhere (topbar,
  `/login`, 401 panels) and no dead links to the unmounted
  `/auth/login` route.
- The anonymous identity is presented sensibly: no avatar chrome
  claiming "signed in as anonymous", no no-op Sign out control.
  `provider === "none"` on the session response is the single
  detection signal.
- A `503` from any `/api/v1` route renders as a retryable transient
  error and never bounces the user to `/login`; the 401 cases keep
  their exact current meaning.
- The topbar never claims "signed out" (a Sign in link) unless the API
  actually answered 401.
- `api/openapi.yaml` re-vendored at `1.2.1` with the client
  regenerated (editorial bump — no generated types change).

### Non-Goals

- Encouraging none-mode. It deliberately leaves the read API open to
  anyone who can reach it; the exposure statement lives upstream in
  docz-api's chart and docs, and the site adds no marketing for it.
- Site-side auth enforcement of any kind — docz-api owns the gate;
  the site only presents what the API reports.
- Offline behavior, request queueing, or optimistic retry beyond the
  query client's existing policy for a 503 outage.
- New site/chart configuration. None-mode is detected dynamically from
  the session probe, not declared in `DOCZ_AUTH_PROVIDERS` — one
  signal, no way for site config to disagree with the API's actual
  mode. (A none-mode operator can still leave `DOCZ_AUTH_PROVIDERS`
  unset; the login page it configures is simply never offered.)

## Background

### What docz-api changed

Verified against a none-mode server (issue [#17]):

| Request | None-mode response |
| --- | --- |
| `GET /api/v1/auth/session` | `200 {"provider":"none","subject":"anonymous","login":"anonymous"}` |
| `POST /api/v1/auth/logout` | `200 {"status":"logged out"}` — a no-op |
| `GET /auth/login`, `/auth/callback` | `404` — routes not mounted |
| `GET /api/v1/*` | `200` with no cookie — never 401 |

And the session gate, which wraps every `/api/v1` route, now
distinguishes:

| Condition | Status | Body |
| --- | --- | --- |
| No cookie / expired / unknown session | `401` | `{"error":"authentication required"}` |
| Session value exists but won't decode | `401` | `{"error":"authentication required"}` |
| Session backend unreachable | `503` | `{"error":"session unavailable"}` |

Previously any session-lookup failure was a 401, so a Redis blip
logged every user out — the 503 exists precisely so clients stop
treating an outage as a logout.

The spec bump to `1.2.1` is editorial: a note on the `sessionCookie`
security scheme documenting that a none-mode server accepts cookieless
requests. Request/response shapes are unchanged, so a client generated
from `1.2.1` works against either mode.

### Audit: how the site behaves today

What already falls out correctly:

- **Route surfaces on 503.** `src/api/fetcher.ts` maps 401 →
  `SessionRequiredError`, 404 → `NotFoundError`, everything else →
  `ApiError`. Every route funnels through
  `src/components/query-states.tsx`, and only `SessionRequiredError`
  triggers `SessionRequiredRedirect` — a 503 already renders the
  retryable `ErrorPanel`, not a login bounce. The issue's "worth
  confirming rather than assuming" checkbox confirms as: correct
  today, but by accident of the catch-all — nothing pins it.
- **`RestoreAfterLogin`** arms only when a return-path stash exists,
  and only a 401 writes the stash — inert in none-mode.
- **Query retry policy** (`src/app/query-client.ts`) already declines
  to retry 401/404 and retries everything else twice — a transient
  503 gets retried before any panel shows.

What does not:

- **`SessionMenu` in none-mode** (`src/components/session-menu.tsx`):
  the session query returns 200, so the topbar renders an "A" avatar,
  an "anonymous · via none" identity panel, and a Sign out button that
  round-trips a no-op logout and lands the user on `/login` — a page
  whose buttons 404. All three none-mode checkboxes in the issue fail
  here.
- **`SessionMenu` on session-query errors**: any error (including the
  new 503, after retries) leaves `session === undefined`, which
  renders the "Sign in" link — the topbar claims you're signed out
  during a backend blip, inviting exactly the re-login the 503 was
  invented to prevent.
- **`/login` in none-mode** (`src/routes/login.tsx`): renders provider
  buttons from `DOCZ_AUTH_PROVIDERS`/`VITE_AUTH_PROVIDERS` config that
  is independent of the API's actual mode; every button is a real
  anchor to the unmounted `/auth/login` → 404.

## Detailed Design

### Component 1: session-state classification

The root fix for both surfaces is that `SessionMenu` currently
collapses five distinct situations into "have session / don't". A new
`src/lib/session.ts` classifies the getSession query once:

```ts
export type SessionState =
  | { kind: "pending" }
  | { kind: "signed-in"; session: Session }
  | { kind: "anonymous" }   // provider === "none": auth is disabled
  | { kind: "signed-out" }  // the API answered 401
  | { kind: "unavailable" }; // 503 or any other failure — unknown, not signed out

export function classifySession(query: {
  isPending: boolean;
  error: unknown;
  data?: { status: number; data: Session };
}): SessionState;
```

Classification rules:

- `isPending` → `pending`.
- status 200 with `provider === "none"` → `anonymous`. The provider
  string is the intended signal (issue [#17]); no other field is
  consulted.
- status 200 otherwise → `signed-in`.
- `error instanceof SessionRequiredError` → `signed-out`. This is the
  ONLY path that may render a Sign in affordance — "signed out" is an
  API answer, never an inference from a failure.
- any other error (including the new `SessionUnavailableError`) →
  `unavailable`.

The helper is pure and unit-testable; `SessionMenu` and the none-mode
`/login` treatment both consume it, so the two surfaces can never
disagree about what the session response meant.

### Component 2: none-mode chrome

**Topbar (`SessionMenu`).** Renders by state:

| State | Render |
| --- | --- |
| `pending` | inert placeholder (unchanged) |
| `signed-in` | avatar disclosure (unchanged) |
| `signed-out` | "Sign in" link (unchanged, still suppressed on `/login`) |
| `anonymous` | nothing (per OQ-1) — no avatar, no Sign out, no Sign in |
| `unavailable` | the inert placeholder, never "Sign in" (per OQ-4) |

Rendering nothing in none-mode is deliberate: an anonymous-mode
deployment has no account concept, so account chrome of any kind is
noise, and hiding the disclosure also hides the no-op logout — three
issue checkboxes with one branch. The `unavailable` placeholder keeps
the topbar footprint stable through a blip and quietly resolves to the
avatar when the backend recovers (the query refetches on its normal
cadence).

**`/login` route.** The page consults the same classification (the
session query is shared cache — on any realistic path SessionMenu has
already issued it, so this adds no request). In the `anonymous` state
it renders a quiet panel in place of the provider buttons (per OQ-2):
the same card chrome, copy along the lines of "Authentication is
disabled on this deployment — everything here is readable without
signing in", and a link home. Every other state renders the existing
button list unchanged. No redirect: a quiet explanation is more honest
than silently teleporting away from a bookmarked URL, and it keeps
`/login` render-stable while the probe is pending.

**Logout.** Unreachable in none-mode (the disclosure never renders),
so no logout special-casing is needed — the no-op POST simply never
fires.

### Component 3: the 503 typed error

`src/api/fetcher.ts` grows one case in `toApiError`:

```ts
/** 503 — session backend unreachable; transient, NEVER a logout. */
export class SessionUnavailableError extends ApiError {
  constructor(message: string, url: string) {
    super(message, 503, url);
    this.name = "SessionUnavailableError";
  }
}
```

keyed on status 503 alone (per OQ-3). Within this API surface a 503 is
the session gate's answer, and an infrastructure 503 (ingress, kube
Service with no endpoints) deserves identical treatment anyway:
transient, retryable, not a logout. The envelope message ("session
unavailable") flows into `ErrorPanel` as it does for every ApiError; a
non-JSON 503 body falls back to the status line, exactly like today.

Behavioral consequences, all falling out of existing seams:

- Route surfaces keep rendering `ErrorPanel` with retry — but now the
  invariant "503 is never treated as signed-out" is pinned by tests
  against the class, not an accident of the catch-all.
- `SessionMenu` maps it to `unavailable` (Component 1) instead of
  "Sign in".
- Retry policy is unchanged: `SessionUnavailableError` is not
  401/404, so the query client already retries it twice before any
  panel renders. No backoff changes (per OQ-4 in DESIGN-0001's
  states table the panels own persistent failures).

### Component 4: spec re-vendor at 1.2.1

`cp ~/code/docz-api/api/openapi.yaml api/` + `bun run gen-api`. The
diff is the `sessionCookie` description note; `gen-api-check` proves
the generated client is byte-identical apart from the version comment.
The `Session.provider` field is already `string`, so `"none"` is
in-band — no type changes, no `arr()`-style wire hazards.

### Cross-cutting: security invariants

- No tokens, no new storage: none-mode never writes anything
  JS-readable; the auth-return stash stays 401-driven and is never
  armed in none-mode.
- No new config injection surface: nothing new flows into the inline
  `__DOCZ_CONFIG__` script; `DOCZ_AUTH_PROVIDERS` validation is
  untouched.
- The none-mode detection consumes an API response field already
  typed by the generated client — no document text, no env text.
- The open-read exposure of none-mode is docz-api's documented
  trade-off; the site neither widens nor narrows it.

## API / Interface Changes

| Surface | Change |
| --- | --- |
| `src/api/fetcher.ts` | new exported `SessionUnavailableError` (503) |
| `src/lib/session.ts` | new — `SessionState`, `classifySession` |
| `src/components/session-menu.tsx` | renders by classification; `anonymous` → nothing, `unavailable` → placeholder |
| `src/routes/login.tsx` | `anonymous` → auth-disabled panel instead of provider buttons |
| `api/openapi.yaml` | re-vendored at 1.2.1 (editorial) |
| Chart / env / `__DOCZ_CONFIG__` | none |

No route changes, no bundle-relevant additions (all eager code is
tiny; the budget delta is noise).

## Data Model

None. `Session.provider` is already `string`; `"none"` is a value, not
a schema change. No storage changes — none-mode specifically must not
introduce any.

## Testing Strategy

- **`session.ts` unit suite**: classification table — pending, 200
  github identity, 200 `provider:"none"`, `SessionRequiredError`,
  `SessionUnavailableError`, generic network error.
- **`fetcher` suite**: 503 with the envelope → `SessionUnavailableError`
  (message from envelope); 503 with a non-JSON body → same class,
  status-line message; existing 401/404 mappings unchanged.
- **`SessionMenu` suite** (MSW overrides per test): none-mode session
  → no avatar, no Sign in, no Sign out anywhere in the topbar; 503
  session → placeholder, explicitly NOT the Sign in link; 401 →
  Sign in link unchanged; recovery — 503 then 200 on refetch renders
  the avatar.
- **`/login` suite**: none-mode → auth-disabled panel, zero
  `/auth/login` anchors; normal mode unchanged.
- **Route-surface pin**: a doc route answering 503 renders `ErrorPanel`
  with working retry and never navigates to `/login` (the invariant
  the issue exists for, pinned against `SessionUnavailableError`).
- **Fixtures**: the demo org keeps its deterministic `donaldgifford`
  github identity as the default; none-mode and 503 shapes are
  per-test `server.use(...)` overrides, mirroring the repo's
  established override style. e2e coverage per OQ-5.
- **axe**: the none-mode `/login` panel joins the a11y sweep.

## Migration / Rollout Plan

Single PR off this design (implementation tracked in an IMPL docz per
the DESIGN-0002 OQ-5a precedent): re-vendor first (no generated
changes to review), then fetcher + `session.ts`, then the two chrome
surfaces, tests alongside each. Release label per OQ-6, with the
chart `appVersion`/version bump and image-tag unittest assert
following the IMPL-0003 OQ-2a convention. Backwards compatibility is
free in both directions: against a pre-1.2.1 docz-api the 503 case
simply never fires and no session ever reports `provider:"none"`; a
none-mode API serves the current site build too (it just shows the
awkward anonymous chrome this design removes).

## Open Questions

**Reviewed 2026-08-12 — decided: 1a, 2a, 3a, 4a, 5a, 6a.** Options
preserved below for the record.

**OQ-1 — None-mode topbar treatment?**

- **a (recommended).** Render nothing: no account chrome at all. An
  anonymous deployment has no account concept; this also removes the
  no-op Sign out and dead Sign in with one branch, and matches the
  issue's "don't render a login affordance" directly.
- **b.** A muted, non-interactive "read-only" / "anonymous" indicator
  in the avatar slot (tooltip: "authentication is disabled on this
  deployment") — keeps the mode discoverable at the cost of permanent
  chrome for information nobody acts on.
- **c.** Keep the avatar disclosure showing the anonymous identity but
  without the Sign out action — maximally "honest" about what the API
  returned, but reads as a broken login to end users.
- other: —

**OQ-2 — `/login` behavior in none-mode?**

- **a (recommended).** Replace the provider buttons with a quiet
  "authentication is disabled on this deployment" panel plus a link
  home. Honest for bookmarked/shared URLs, no dead buttons, no
  navigation surprise, render-stable while the probe resolves.
- **b.** Redirect to `/` (replace). Less code, but a silent teleport
  from a URL the user deliberately opened, and it needs the same
  session probe anyway to decide.
- **c.** Leave the page as-is; buttons 404 at the proxy when clicked.
  Zero work, worst experience — a visible sign-in page that errors is
  indistinguishable from an outage.
- other: —

**OQ-3 — Scope of the 503 typed error?**

- **a (recommended).** `SessionUnavailableError` keyed on status 503
  alone. Within this API surface the session gate owns 503, and an
  infrastructure 503 wants identical handling (transient, retryable,
  never a logout); one status, one class, no body sniffing.
- **b.** Key on status 503 AND the `"session unavailable"` envelope
  message; other 503s stay generic `ApiError`. Tighter attribution,
  but brittle against upstream copy changes and buys no behavioral
  difference — both cases render the same retry panel.
- **c.** No new class; check `error.status === 503` inline where
  needed. Fewer exports, but the "never a logout" invariant ends up
  duplicated as magic numbers instead of pinned on one type like the
  401/404 precedents.
- other: —

**OQ-4 — Topbar on session-query failures other than 401?**

- **a (recommended).** Generalize: only an actual 401 renders "Sign
  in"; every other failure (503, network error, 5xx) renders the
  inert placeholder. "You are signed out" is an API answer, not an
  inference from a failure — this fixes the same lie for network
  blips that the 503 fixes for Redis blips.
- **b.** Special-case only `SessionUnavailableError`; other errors
  keep today's "Sign in". Minimal diff, but preserves the false
  signed-out claim for the equally-transient network-failure case.
- other: —

**OQ-5 — e2e coverage?**

- **a (recommended).** Unit + axe only. Both behaviors are leaf
  presentation states fully exercisable with MSW overrides in vitest;
  the e2e suite is reserved for cross-page journeys, and none-mode
  would need a new browser-worker override flag for marginal signal.
- **b.** Add an e2e none-mode journey (sessionStorage flag like the
  existing `docz:e2e:force-401`, browser-worker override returning
  the anonymous session, assert the topbar has no account chrome) —
  one real-browser guarantee, one more worker flag and wall-clock.
- other: —

**OQ-6 — Release label for the implementation PR?**

- **a (recommended).** `minor` (0.4.0 → 0.5.0): the site newly
  supports a deployment mode it previously mishandled — feature-grade,
  matching the IMPL-0003 phase convention.
- **b.** `patch` (0.4.0 → 0.4.1): arguably all fixes to existing
  chrome; smaller version story, same code either way.
- other: —

## References

- Issue [#17] — this ask, with the verified none-mode behavior tables
- donaldgifford/docz-api#19 — upstream change (INV-0007 / IMPL-0006):
  error observability, queue self-heal, and the login-free first-setup
  mode; session gate 401/503 split
- docz-api `api/openapi.yaml` `1.2.1` — `sessionCookie` scheme note
  documenting none-mode's cookieless acceptance
- DESIGN-0001 — auth and session handling (401-driven login flow, the
  states table the panels implement)
- IMPL-0001 Phase 5 — login page, `SessionRequiredRedirect`,
  `RestoreAfterLogin`, `SessionMenu`
- PR #11 — runtime auth providers (`DOCZ_AUTH_PROVIDERS`), the config
  surface deliberately NOT extended here

[#17]: https://github.com/donaldgifford/docz-site/issues/17
