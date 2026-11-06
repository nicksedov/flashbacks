# Tasks

## 1. Harness foundations

- [x] 1.1 Extend [`tests/e2e/support/env.ts`](../../../tests/e2e/support/env.ts:1) with `E2E_DESTRUCTIVE` / `E2E_ALLOW_REMOTE_DESTRUCTIVE` and their SKIP reasons, export a signed-out base test (no `api` fixture) from [`tests/e2e/fixtures.ts`](../../../tests/e2e/fixtures.ts:1), and add a `pageerror` collector that fails a test on an uncaught browser error; verify `cd webapp && npx tsc -p ../tests/e2e` and `npx playwright test --list` still enumerate the three existing specs
- [x] 1.2 Add API helpers for the new flows to [`tests/e2e/support/api.ts`](../../../tests/e2e/support/api.ts:1) — gallery listing, folder add/remove, trash list/restore/clean, app settings read/update, duplicates listing — and sidebar/tab navigation helpers to [`tests/e2e/support/ui.ts`](../../../tests/e2e/support/ui.ts:1) matching en/ru labels; verify `cd webapp && npx tsc -p ../tests/e2e`
- [x] 1.3 Add a temporary-gallery fixture: copy one `tests/dataset` image into a unique OS temp directory, register it with `POST /api/folders`, poll `/api/gallery` until indexed, and remove the folder plus directory in teardown even on failure; refuse a destructive run when `E2E_BASE_URL` is not localhost/127.0.0.1 unless `E2E_ALLOW_REMOTE_DESTRUCTIVE=1`; verify the guard with `E2E_DESTRUCTIVE=1 E2E_BASE_URL=https://staging.flashbacks.life` reporting the refusal and creating nothing

## 2. Authentication

- [x] 2.1 Add [`tests/e2e/auth.spec.ts`](../../../tests/e2e/auth.spec.ts:1): an unauthenticated visit shows the login form, a wrong password shows the error message, a valid sign-in reaches the workspace sidebar, and sign-out returns to the login form; verify the spec passes against the locally booted stack (or SKIPs without `E2E_LOGIN`/`E2E_PASSWORD`) with `npx playwright test auth`

## 3. Gallery and lightbox (read-only)

- [x] 3.1 Add [`tests/e2e/gallery.spec.ts`](../../../tests/e2e/gallery.spec.ts:1): the all-images grid renders the seeded photos, a tile opens the shared lightbox, the EXIF metadata panel shows the file section, the OCR/AI/Tags modes switch without losing the image, and closing returns to the grid; add any missing `data-slot` hooks to the gallery tiles and lightbox mode tabs; verify `npx playwright test gallery` and `cd webapp && npm run lint && npx tsc -b && npm test`
- [x] 3.2 Cover the calendar sub-mode (a seeded photo appears under its date) and the folders sub-mode (the registered folder is listed) in the same spec, skipping each when the deployment has no dated photo or no folder; verify `npx playwright test gallery`

## 4. Navigation and degraded states

- [x] 4.1 Add [`tests/e2e/navigation.spec.ts`](../../../tests/e2e/navigation.spec.ts:1): every sidebar destination — Gallery all-images/calendar/folders/geolocation, Smart Search, Deduplication, Text Recognition, Metadata, Trash, People and About (admin settings/users for an admin) — renders its screen with no uncaught page error and no blank page; verify `npx playwright test navigation`
- [x] 4.2 Add the degraded/empty states to the same spec: Smart Search without an embedding index shows its configuration/index hint, Deduplication without duplicate groups shows its empty state, and an empty Trash shows its empty state (each skipped when the deployment contradicts the precondition); verify `npx playwright test navigation` and `cd webapp && npm run lint && npx tsc -b`
- [x] 4.3 Fix the Smart Search hint the spec exposed: [`SmartSearchTab.tsx`](../../../webapp/src/components/tabs/SmartSearchTab.tsx:44) kept the embedding status only while polling, so the "no embeddings" banner never rendered on a first visit; keep the status read on mount (polled data wins while a backfill runs) and cover the change with the existing Vitest suite; verify `npx playwright test navigation` and `cd webapp && npm run lint && npx tsc -b && npm test`

## 5. Destructive maintenance (opt-in)

- [x] 5.1 Add [`tests/e2e/maintenance.spec.ts`](../../../tests/e2e/maintenance.spec.ts:1) gated by `E2E_DESTRUCTIVE=1`: move the copied fixture photo to trash through the UI, see it in the trash list and restore it, then delete it permanently; verify it leaves no folder, file or trash row behind (`npx playwright test maintenance` with the flag on the local stack)
- [x] 5.2 Cover the remaining maintenance flows in the same spec: add/remove the temporary gallery folder, save a setting and restore the previous value, and run a duplicate batch delete over a copied pair created for the run; verify the same command reports PASS and the cleanup assertions hold

## 6. Documentation and suite verification

- [x] 6.1 Document the new specs, the `E2E_DESTRUCTIVE` flag, the remote guard and the fixture lifecycle in [`tests/e2e/README.md`](../../../tests/e2e/README.md:1) and `docs/agent/webapp.md`; verify the documented commands appear (`grep -n "E2E_DESTRUCTIVE" tests/e2e/README.md docs/agent/webapp.md`)
- [x] 6.2 Run the whole suite both ways and the component checks: `npx playwright test` (read-only, destructive specs SKIP), `E2E_DESTRUCTIVE=1 npx playwright test` on the local stack, `cd webapp && npx tsc -p ../tests/e2e && npm run lint && npx tsc -b && npm test`

## Workflow follow-up

- Run the SDD validate step, then sync/archive this change once the full suite is green.
