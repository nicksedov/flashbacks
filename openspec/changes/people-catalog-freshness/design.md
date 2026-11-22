# Design

## Context

See [`proposal.md`](openspec/changes/people-catalog-freshness/proposal.md:1) for
motivation. The relevant current state:

- [`PeopleTab`](webapp/src/components/tabs/PeopleTab.tsx:46) keeps two pieces of
  catalog state: `people` (the grid) and `selected` (the open person). Only
  `people` is refreshed; `selected` and the detail's `photos` are never
  reconciled, so a person merged away from the detail's own lightbox stays on
  screen.
- The People tab is mounted per activation (Radix `TabsContent` unmounts the
  inactive tab), which covers tab switching but not a catalog change that
  happens while the tab stays open (another window/session).
- [`sw.js`](webapp/public/sw.js:55) caches every successful `/api/*` GET
  response. The Cache API ignores the backend's
  [`Cache-Control: no-store`](backend/core/internal/interfaces/handler/router.go:16),
  and the network-first fallback replays a cached payload whenever `fetch`
  throws — exactly what a deployment restart produces.

No backend, OpenAPI, Wire DI, database or i18n change is required; the backend
catalog was already correct.

## Goals / Non-Goals

**Goals:**

- Make the visible People state a function of the catalog response the tab most
  recently loaded, including after a catalog change initiated from the tab's own
  lightbox.
- Never let a payload cached by an earlier build be presented as the current
  catalog.

**Non-Goals:**

- Preventing the clustering job from creating a duplicate unnamed cluster of an
  already-named person (recognition quality, separate concern).
- Offline support for the People tab; with network-only API requests an offline
  catalog request fails and the tab keeps its last in-memory state.
- Changing `GET /api/people` or any other contract.

## Decisions

### 1. One refresh path that reconciles the selection

`reloadPeople` becomes a `refreshPeople` helper that loads the catalog and then
applies both states in one pass:

- `setPeople(fresh)`;
- if `selected` is set and `fresh` has no entry with that id, clear `selected`
  and `photos` (returning the UI to the grid); otherwise replace `selected` with
  the fresh DTO so its name/cover stay current, keeping the same object identity
  only when it did not change.

Clearing on absence (instead of keeping a stale detail) is what satisfies the
"detail follows the catalog" requirement. The helper is the single place used by
the initial mount, the lightbox's `onPeopleChanged`, the visibility listener and
the mutation handlers, so no path can leave the two states inconsistent.

Alternative considered: keep `selected` and let the detail fetch 404s. Rejected —
it leaves a ghost header and the former cluster's photos on screen while the
user waits for a failing request.

### 2. Refresh the catalog on visibility, not by polling

A `document.visibilitychange` (visible only) plus `window.focus` listener calls
`refreshPeople`. This covers another window/session and the tab regaining focus
without a timer, and costs one request per focus. Radix already remounts the tab
on tab switching, so no extra work is needed there.

Alternative considered: polling every N seconds. Rejected — constant load for a
low-frequency change and unnecessary state churn.

### 3. Service worker: network-only for `/api/*`, cache version bump

The `/api/` branch of the fetch handler becomes
`event.respondWith(fetch(request))` — no `caches.match`, no `cache.put`. Keeping
static-asset and JS/CSS strategies unchanged. `CACHE_NAME` moves from
`flashbacks-v3` to `flashbacks-v4`; `activate` deletes every other cache, so the
existing cached catalog payloads are purged on the first load after deploy.

Alternative considered: keep network-first-with-fallback but scope it to
non-mutating endpoints. Rejected — every `/api/` response is user-specific and
can name entities that no longer exist; there is no safe stale window.

### 4. Evidence (bindings are written in the apply phase)

Native items are already marked in the delta spec
([`specs/face-recognition/spec.md`](openspec/changes/people-catalog-freshness/specs/face-recognition/spec.md:7)).
The intended evidence, to be registered in `.focused-spec/evidence.yaml` and
implemented during apply:

- `face-recognition.people-detail-closes-when-person-gone` →
  `vitest::webapp/src/components/tabs/PeopleTab.test.tsx::PeopleTab > returns to the grid when the open person disappears from the refreshed catalog`
- `face-recognition.people-list-refreshes-on-visibility` →
  `vitest::webapp/src/components/tabs/PeopleTab.test.tsx::PeopleTab > reloads the catalog when the tab becomes visible again`
- `face-recognition.catalog-request-network-only` →
  `vitest::webapp/src/service-worker.test.ts::service worker > serves API requests from the network without touching the cache`

The service-worker test reads `public/sw.js` as text (the pattern already used
by [`branding.test.ts`](webapp/src/branding.test.ts:61) for the precache list),
extracts the `/api/` branch and asserts it calls `fetch(request)` and never
`caches.match`/`cache.put`; it also asserts the cache name was bumped.

Mutation proof planned for apply: temporarily restore the old behavior (do not
clear `selected`, skip the visibility listener, route `/api/` through the cache
helper) and observe each exact test fail, then restore.

## Risks / Trade-offs

- **Extra `/api/people` request on every focus** → the endpoint is a cheap
  indexed read and the payload is small; accepted for correctness.
- **A failed refresh must not clear the detail** → reconcile only against a
  successfully loaded catalog; on error keep the current list/selection.
- **Users with an installed PWA keep the old worker until it activates** → the
  new worker calls `skipWaiting`/`clients.claim` already; the version bump
  guarantees the old cache is dropped on the first activation.
- **Text-contract test for `sw.js` is brittle** → keep the assertion limited to
  the `/api/` branch shape and the cache-name constant, matching the existing
  project precedent; no new build step for the service worker.

## Migration Plan

1. Ship the webapp build (no backend deploy required).
2. First load registers the `v4` worker; `activate` deletes `v3`, removing the
   stale catalog payloads.
3. Rollback: redeploy the previous webapp bundle; the older worker re-registers
   with `v3` and re-enables caching. No data migration to undo.
