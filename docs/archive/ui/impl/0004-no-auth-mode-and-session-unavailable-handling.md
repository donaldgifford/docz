---
id: IMPL-0004
title: "No-auth mode and session-unavailable handling"
status: Completed
author: Donald Gifford
created: 2026-08-27
---

<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL-0004: No-auth mode and session-unavailable handling

**Status:** Completed
**Author:** Donald Gifford
**Date:** 2026-08-27

<!--toc:start-->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: foundations — spec, typed error, classification](#phase-1-foundations--spec-typed-error-classification)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2: chrome — topbar and login surfaces](#phase-2-chrome--topbar-and-login-surfaces)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 3: ship — docs, chart, release](#phase-3-ship--docs-chart-release)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
- [Standing gates (every phase)](#standing-gates-every-phase)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Dependencies](#dependencies)
- [Open Questions](#open-questions)
- [References](#references)
<!--toc:end-->

## Objective

Execute DESIGN-0003 (Approved 2026-08-12; decisions 1a 2a 3a 4a 5a
6a): make the site's auth chrome mode-aware for docz-api's
`AUTH_PROVIDERS=none` first-setup mode, and teach the client that the
session gate's new `503` is a transient backend fault that must never
read as a logout. One `minor` release; docz-api's side already shipped
(spec `1.2.1`, upstream donaldgifford/docz-api#19).

**Implements:** DESIGN-0003 (from issue
[#17](https://github.com/donaldgifford/docz-site/issues/17))

## Scope

### In Scope

- Vendored spec re-vendor `1.2.0` → `1.2.1` (editorial `sessionCookie`
  note; no generated type changes).
- `SessionUnavailableError` in `src/api/fetcher.ts`, keyed on status
  503 alone (OQ-3a).
- `src/lib/session.ts`: `SessionState` +
  `classifySession` — `pending / signed-in / anonymous / signed-out /
  unavailable`; "signed-out" only ever from an actual 401 (OQ-4a).
- `SessionMenu` rendering by classification: `anonymous` → nothing
  (OQ-1a), `unavailable` → inert placeholder, everything else
  unchanged.
- `/login` in none-mode: quiet "authentication is disabled" panel in
  place of the provider buttons (OQ-2a).
- Unit + axe coverage per the design's testing strategy (OQ-5a: no
  e2e).

### Out of Scope

- Server-side logging / `DOCZ_LOG_LEVEL` — tracked separately in
  issue [#18](https://github.com/donaldgifford/docz-site/issues/18),
  pending its own INV.
- Non-docz page rendering (docz PR #84's `api:` block) — blocked on a
  docz-api serving surface that doesn't exist yet.
- Any new chart/env/`__DOCZ_CONFIG__` configuration — none-mode is
  detected from the session response only.
- Auth enforcement, offline behavior, retry-policy changes beyond the
  existing query-client defaults.

## Implementation Phases

Each phase builds on the previous one. A phase is complete when all
its tasks are checked off and its success criteria are met. Per OQ-1,
the phases land as commits on ONE branch/PR (`feat/none-auth-mode`),
not one PR per phase.

---

### Phase 1: foundations — spec, typed error, classification

The pieces with no visible behavior change: the re-vendored contract,
the 503 error type, and the pure classification helper both chrome
surfaces will consume.

#### Tasks

- [x] Re-vendor `api/openapi.yaml` at `1.2.1` from docz-api main and
      run `bun run gen-api`; confirm the diff is the `sessionCookie`
      description note only and `gen-api-check` is current
- [x] `src/api/fetcher.ts`: add `SessionUnavailableError extends
      ApiError` (status 503, name set, doc comment "transient, NEVER
      a logout"); wire the `case 503:` into `toApiError`
- [x] Fetcher tests: 503 with the `{"error":"session unavailable"}`
      envelope → `SessionUnavailableError` carrying the envelope
      message; 503 with a non-JSON body → same class, status-line
      message; existing 401/404 mappings pinned unchanged
- [x] New `src/lib/session.ts`: `SessionState` discriminated union +
      `classifySession(query)` per the design's rules — `pending`;
      200 + `provider === "none"` → `anonymous`; 200 otherwise →
      `signed-in`; `SessionRequiredError` → `signed-out`; any other
      error → `unavailable`
- [x] `src/lib/session.test.ts`: the full classification table —
      pending, github identity, `provider:"none"`, 401, 503, generic
      network error — one assertion per row

#### Success Criteria

- `gen-api-check` green; generated client byte-identical apart from
  the spec-version comment
- Fetcher and classification suites green; no existing test touched
- No rendered surface changes yet — `just ci` passes exactly as on
  main

---

### Phase 2: chrome — topbar and login surfaces

The user-visible half: SessionMenu and `/login` consume the
classification, and the 503-never-logout invariant gets pinned at a
route surface.

#### Tasks

- [x] `SessionMenu` renders by `classifySession`: `pending` →
      existing placeholder; `signed-in` → existing avatar disclosure;
      `signed-out` → existing "Sign in" link (still suppressed on
      `/login`); `anonymous` → `null`; `unavailable` → the inert
      placeholder — "Sign in" is unreachable for any non-401 failure
- [x] Session-query recovery from `unavailable` (OQ-2a): a
      `refetchInterval` callback returning ~30 s only while the query
      is errored (`false` otherwise), so the topbar self-heals after
      a backend blip without a reload (SessionMenu never unmounts and
      `refetchOnWindowFocus` is off, so nothing else re-triggers it);
      the `unavailable` placeholder is visually identical to
      `pending` with a distinct `data-testid` only (OQ-4a)
- [x] `src/routes/login.tsx`: consume the shared session
      classification; `anonymous` → the auth-disabled panel (same
      card chrome, copy per the design, link home, zero
      `/auth/login` anchors); every other state — including a still-
      pending probe — renders the provider buttons unchanged (OQ-3a:
      buttons immediately, swap only on confirmed anonymous)
- [x] `SessionMenu` tests (MSW per-test overrides; fixture default
      identity stays `donaldgifford`): none-mode → no avatar, no
      Sign in, no Sign out anywhere; 503 → placeholder and
      explicitly NOT the Sign in link; 401 → Sign in unchanged;
      recovery — 503 then 200 renders the avatar
- [x] `/login` tests: none-mode panel with no `/auth/login` anchors;
      normal-mode buttons unchanged
- [x] Route-surface pin: a doc route answering 503 renders
      `ErrorPanel` with a working retry and never navigates to
      `/login` (asserted against `SessionUnavailableError`)
- [x] Axe sweep: none-mode `/login` panel entry added

#### Success Criteria

- A none-mode deployment (unit-proven) renders zero auth affordances:
  no login link, no login buttons, no logout, no anonymous identity
  chrome
- A 503 during a session probe leaves the topbar in the placeholder
  state and recovers to the avatar when the backend does
- A 503 on a data route renders the retryable error panel; the login
  redirect fires only on 401
- Axe sweep green including the new entry

---

### Phase 3: ship — docs, chart, release

#### Tasks

- [x] Update CLAUDE.md (session classification, none-mode chrome,
      `SessionUnavailableError`, the 401-only Sign in rule) and the
      root README's Auth section (none-mode paragraph: what the site
      shows, that detection is automatic)
- [x] Chart bump per the IMPL-0003 OQ-2a convention: appVersion
      `0.4.0` → `0.5.0` (label `minor`, design OQ-6a), chart version
      `0.1.5` → `0.1.6`, deployment unittest image-tag assert updated,
      helm-docs regenerated
- [x] Full local gate + changelog sync (`git fetch --tags`, regen,
      cliff-skipped `chore(changelog):` commit)
- [x] PR `feat/none-auth-mode` labeled `minor`; merge on green; sync
      main
      ([#20](https://github.com/donaldgifford/docz-site/pull/20))
- [x] Mark this IMPL Completed and DESIGN-0003 Implemented; close
      issue #17 (the merge commit references it)

#### Success Criteria

- `just ci` chain green (unit, test-server, lint, `tsc -b --force`,
  build, format, bundle-budget, e2e, gen-api-check); bundle delta
  negligible (no new eager dependencies)
- helm lint + unittest green with the `0.5.0` image assert
- PR merged; v0.5.0 train cut; issue #17 closed with all its
  checkboxes satisfied

---

## Standing gates (every phase)

- `just ci` semantics: test, test-server, lint, `tsc -b --force`,
  build, format:check, bundle-budget, e2e, gen-api-check
- No credential-shaped strings; no `schema.ts` widening; no tokens or
  new JS-readable storage (none-mode must add zero storage)
- Check tasks off here as completed; update CLAUDE.md when guidance
  changes; conventional commits per task

## File Changes

| File | Action | Description |
| ---- | ------ | ----------- |
| `api/openapi.yaml` | Modify | re-vendor at 1.2.1 (editorial) |
| `src/api/fetcher.ts` | Modify | `SessionUnavailableError` (503) |
| `src/api/fetcher.test.ts` | Modify | 503 mapping cases |
| `src/lib/session.ts` | Create | `SessionState`, `classifySession` |
| `src/lib/session.test.ts` | Create | classification table |
| `src/components/session-menu.tsx` | Modify | render by classification; recovery re-poll |
| `src/components/session-menu.test.tsx` | Modify | none-mode / 503 / recovery cases |
| `src/routes/login.tsx` | Modify | none-mode auth-disabled panel |
| `src/routes/login.test.tsx` | Modify | none-mode panel cases |
| `src/routes/doc.test.tsx` | Modify | 503 route-surface pin |
| `src/a11y/axe.test.tsx` | Modify | none-mode login entry |
| `charts/docz-site/*` | Modify | appVersion 0.5.0, chart 0.1.6, assert |
| `CLAUDE.md`, `README.md` | Modify | guidance + Auth section |

## Testing Plan

- [x] Unit: fetcher 503 mapping; `classifySession` table; SessionMenu
      per-state rendering incl. recovery; login none-mode panel
- [x] Integration (route-level, MSW): 503 doc route → `ErrorPanel`,
      no login navigation
- [x] Axe: none-mode login panel; existing sweep stays green
- [x] No e2e additions (design OQ-5a)

## Dependencies

- docz-api ≥ spec `1.2.1` behavior — already shipped and merged
  upstream (donaldgifford/docz-api#19); nothing blocking.
- The design-doc branch (`docs/design-none-auth-and-session-503`)
  merged first per OQ-1a's docs-first precedent, so checkbox updates
  land on main-based branches.

## Open Questions

**Reviewed 2026-08-12 — decided: 1a, 2a, 3a, 4a.** Options preserved
below for the record. Consequences are folded into the phase tasks:
one branch/PR carries all three phases (1a); the session query
re-polls ~30 s only while errored so the topbar self-heals (2a);
`/login` renders buttons immediately and swaps only on a confirmed
anonymous session (3a); the `unavailable` placeholder is visually
identical to `pending` with only a distinct test id (4a).

**OQ-1 — PR granularity?**

- **a (recommended).** One branch, one `minor` PR carrying all three
  phases (per-task conventional commits inside it). The design's
  rollout plan says single PR; the whole change is smaller than any
  one IMPL-0003 phase, and Phases 1–2 aren't independently valuable —
  an unused error class ships nothing.
- **b.** One PR per phase, IMPL-0003 style — finer review units and a
  releasable checkpoint after Phase 2, at the cost of two extra
  release-train runs for a change this size.
- other: —

**OQ-2 — Topbar recovery from the `unavailable` state?**

- **a (recommended).** Gentle re-poll only while the session query is
  errored (`refetchInterval` callback returning ~30 s in the error
  state, `false` otherwise). SessionMenu mounts once in AppShell and
  never unmounts, and the client sets `refetchOnWindowFocus: false` —
  without this, an exhausted-retries 503 leaves the topbar stuck on
  the placeholder until a full reload, which contradicts the design's
  "quietly resolves when the backend recovers". Bounded: one request
  per 30 s only during an outage.
- **b.** Enable `refetchOnWindowFocus` for the session query only —
  zero polling, but recovery then needs a tab blur/focus, which a
  user staring at the outage never performs.
- **c.** Do nothing — the placeholder persists until reload or a
  remount. Simplest, but the topbar silently lies ("still checking")
  long after the backend recovered.
- other: —

**OQ-3 — `/login` while the session probe is pending?**

- **a (recommended).** Render the provider buttons immediately and
  swap to the auth-disabled panel only when an `anonymous`
  classification confirms. Normal deployments (the overwhelming case)
  see zero change and zero added latency; a none-mode visitor gets at
  worst a brief button flash on a cold first visit — and clicking
  before the swap just 404s, which is today's behavior, not a
  regression.
- **b.** Hold the card in a skeleton until the probe resolves — no
  flash anywhere, but every deployment's sign-in page now waits on a
  session round-trip before showing its buttons.
- other: —

**OQ-4 — Does the `unavailable` placeholder carry an operator hint?**

- **a (recommended).** No — identical inert placeholder to `pending`
  (a distinct `data-testid` for the suites, no user-visible
  difference). Route surfaces already announce the outage loudly with
  the retryable error panel; the topbar's job is just to not lie.
- **b.** Add a `title`/tooltip ("session backend unreachable") to the
  placeholder — a breadcrumb for operators, permanent hover noise for
  everyone else, and a second place outage copy must stay accurate.
- other: —

## References

- DESIGN-0003 — Support docz-api's no-auth mode and the
  session-unavailable 503 (Approved; decisions 1a 2a 3a 4a 5a 6a)
- Issue [#17](https://github.com/donaldgifford/docz-site/issues/17) —
  the ask, with verified none-mode behavior tables
- donaldgifford/docz-api#19 — upstream change (INV-0007 / IMPL-0006)
- Issue [#18](https://github.com/donaldgifford/docz-site/issues/18) —
  server logging, deliberately out of scope here
- IMPL-0003 — phase/PR/chart-bump conventions this doc follows
