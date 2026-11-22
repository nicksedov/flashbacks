# Tasks

## 1. People detail follows the catalog

- [ ] 1.1 In `webapp/src/components/tabs/PeopleTab.tsx`, turn `reloadPeople` into a single refresh helper that loads the catalog, replaces `people`, and reconciles `selected`/`photos` (clear both when the selected id is absent, otherwise adopt the fresh DTO); route the mount load, the lightbox `onPeopleChanged`, and the mutation handlers through it. Verify `npm run lint && npx tsc -b` in `webapp/` reports no errors.
- [ ] 1.2 Add the PeopleTab test `returns to the grid when the open person disappears from the refreshed catalog` (mock `UnifiedLightbox` to invoke `onPeopleChanged`; `fetchPeople` returns a list without the open person) and verify `npx vitest run src/components/tabs/PeopleTab.test.tsx` passes.

## 2. People list refreshes when the tab becomes visible

- [ ] 2.1 Add a `document.visibilitychange` (visible only) and `window.focus` listener in `PeopleTab` that calls the refresh helper, with cleanup on unmount. Verify `npm run lint && npx tsc -b` in `webapp/` reports no errors.
- [ ] 2.2 Add the PeopleTab test `reloads the catalog when the tab becomes visible again` (change the `fetchPeople` mock result, dispatch `visibilitychange` with `document.visibilityState = "visible"`) and verify `npx vitest run src/components/tabs/PeopleTab.test.tsx` passes.

## 3. Service worker never serves cached API payloads

- [ ] 3.1 In `webapp/public/sw.js`, make the `/api/` fetch branch call `event.respondWith(fetch(request))` with no cache read or write, and bump `CACHE_NAME` to `flashbacks-v4`. Verify `node --check webapp/public/sw.js` succeeds.
- [ ] 3.2 Add `webapp/src/service-worker.test.ts` asserting the `/api/` branch is network-only (contains `fetch(request)`, no `caches.match`/`cache.put`) and that the cache name was bumped. Verify `npx vitest run src/service-worker.test.ts` passes.

## 4. Focused evidence

- [ ] 4.1 Register the three evidence references for `face-recognition.people-detail-closes-when-person-gone`, `face-recognition.people-list-refreshes-on-visibility` and `face-recognition.catalog-request-network-only` in `.focused-spec/evidence.yaml`, then verify `node_modules/.bin/focused-spec validate --scope current --strict` and `node_modules/.bin/focused-spec run --scope current` report PASS.
- [ ] 4.2 Mutation proof: reintroduce each protected behavior regression in turn (keep `selected` on absence; remove the visibility listener; route `/api/` through the cache helper), observe the exact registered test fail, restore the product code, and verify `focused-spec run --scope current` passes again.

## 5. Integration checks

- [ ] 5.1 Run the full webapp suite (`npm test`) and `npm run lint && npx tsc -b` in `webapp/`; verify no failures and no type/lint errors.
- [ ] 5.2 Confirm the change touched only webapp sources/tests and `.focused-spec/evidence.yaml` (`git diff --stat`), with no backend, OpenAPI, MCP or i18n files.

## Workflow follow-up

- Validate the apply result against `docs/agent/sdd-workflow.md` §7 and record the verdict.
- Sync the delta spec into `openspec/specs/face-recognition/spec.md`.
- Archive the change under `openspec/changes/archive/`.
