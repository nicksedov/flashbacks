# Design

## Context

See [proposal.md](proposal.md) for motivation. The existing Playwright harness
(committed with the people-catalog work) already provides:

- [`playwright.config.ts`](../../../playwright.config.ts:15) with `E2E_BASE_URL`,
  headless Chromium by default and the optional `PW_TUNNEL=1` mode;
- [`tests/e2e/fixtures.ts`](../../../tests/e2e/fixtures.ts:45) with a form login
  that yields an authenticated page and a `FlashbacksApi` bound to the browser's
  cookie jar;
- `tests/e2e/support/api.ts`, `env.ts` and `ui.ts` helpers.

The suite is credential-gated and seeds through the catalog API only. The rest
of the app has no browser coverage, and the HTTP integration suite in
`tests/webapp` cannot exercise tab navigation, the lightbox shell, dialogs or
empty states.

## Goals / Non-Goals

**Goals:**

- Cover the core user journeys in a real browser: sign-in/out, gallery browsing,
  lightbox modes, cross-tab navigation, documented empty/degraded states, and the
  maintenance flows.
- Product changes stay limited to inert `data-slot` hooks, plus the one behavior
  fix the harness exposed (the unreachable Smart Search index hint).
- Keep every spec independent and self-cleaning, so partial failures never leave
  rows or files behind.
- Make destructive coverage opt-in and safe on a disposable deployment.

**Non-Goals:**

- No new requirement/scenario contract: `.openspec.yaml` sets
  `skip_specs: true`; behavior lives in the existing specs.
- No focused-spec evidence bindings for Playwright (the project has no Playwright
  runner); Playwright remains an integration verification command.
- No upload endpoint, no model-service assertions, no visual snapshots.

## Decisions

### 1. One spec file per flow area, plus an opt-in destructive file

`auth.spec.ts`, `gallery.spec.ts`, `navigation.spec.ts` and
`maintenance.spec.ts` (destructive). A flow file is skipped as a whole when its
preconditions are absent (no credentials, no photos, no duplicate groups), with a
message naming the missing prerequisite.

*Alternative considered:* one large “smoke” spec — rejected because a single
failure would hide which screen regressed and because preconditions differ.

### 2. Read-only specs never mutate shared state

They assert on what the deployment already holds and skip when the gallery is
empty. They may open the lightbox and switch modes, but never save, delete or
rename. This keeps them safe to run against staging at any time.

### 3. Destructive specs own a temporary gallery folder

When `E2E_DESTRUCTIVE=1`, the maintenance spec:

1. creates a unique directory under the OS temp dir and copies one image from
   `tests/dataset/` into it;
2. registers it with `POST /api/folders` and polls `/api/gallery` until the photo
   is indexed;
3. runs the flow (trash → restore, duplicate batch delete, folder removal,
   settings save → restore) against fixtures whose names carry the run suffix;
4. removes the folder through `DELETE /api/folders/:id` and deletes the temp
   directory in `afterAll`, even on failure.

Without the flag the file reports SKIP with the reason, so the default run never
touches a shared deployment.

*Alternative considered:* reuse any existing photo — rejected: deleting or
trashing a user photo on staging is unacceptable and unverifiable.

### 4. Prefer `data-slot` hooks over visible text; localize only navigation

Empty states, buttons and tiles are targeted with `data-slot` / `data-testid`
attributes, adding a hook to product markup where one is missing (inert
attribute only). Sidebar destinations are matched with a small en/ru regex, the
same approach the People helper already uses.

### 5. Assert the page is healthy, not just that a selector exists

The fixture collects `pageerror` events and fails the test when one occurs; a
read-only spec may additionally assert a known empty-state heading. Console
messages are not asserted (third-party warnings are noisy), only uncaught errors.

### 6. Keep the login and tunnel plumbing unchanged

Auth specs use a separate base test without the signed-in `api` fixture so they
can start from the login screen. `PW_TUNNEL=1` continues to swap only the
`browser` fixture.

### 7. Fix the Smart Search hint the harness exposed

`SmartSearchTab` loaded the embedding status on mount but discarded it unless a
backfill was already running, so the "no embeddings" hint never rendered on a
first visit even though the API reported an empty index. The degraded-state spec
could not observe the documented hint, so the tab now keeps the mount snapshot
(polled data still wins while a backfill runs). This is the only behavior change
in the change; the E2E assertion stays strict instead of skipping the state.

## Risks / Trade-offs

- **Staging gallery assumptions** → specs skip with the missing prerequisite
  (no photos, no duplicates) instead of failing; the local test deployment seeds
  what the destructive suite needs.
- **Scan timing after adding a folder** → poll the gallery listing with a bounded
  timeout instead of a fixed sleep.
- **Destructive run on the wrong host** → the flag is off by default and the
  harness refuses when `E2E_BASE_URL` does not point at localhost/127.0.0.1
  unless `E2E_ALLOW_REMOTE_DESTRUCTIVE=1` is also set.
- **Selector drift** → `data-slot` attributes are part of the UI contract now and
  documented in `docs/agent/webapp.md`; text asserts are limited to stable
  empty-state copy.
- **Longer suite** → flow specs stay independent and run with one worker, so the
  wall-clock cost is linear and failures remain attributable.

## Migration Plan

No data or deployment migration. The suite is additive; the destructive file is
inert without the flag. Rollback is deleting the new specs/helpers.

## Open Questions

None.
