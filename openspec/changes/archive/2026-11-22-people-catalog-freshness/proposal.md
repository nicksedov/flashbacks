# Proposal

## Why

The production People tab can keep showing a cluster that the catalog no longer
contains — for example "Человек 11033" after that cluster was folded into
"Василиса" — and its photos then belong to the surviving person. The backend
data is already correct, but the tab never reconciles the person it has open,
never reloads the catalog when it becomes visible again, and the PWA service
worker replays a cached `/api/people` payload when the API is briefly
unreachable (for example during a deployment). Users see a person that no
longer exists.

## What Changes

- The People tab reconciles its selected person after every catalog refresh: a
  person that is no longer in the fresh list closes the detail view back to the
  grid instead of leaving a ghost header and its old photo grid on screen.
- The People tab reloads the catalog when it becomes visible again, so a cluster
  merged in another view, tab or session does not linger in the list.
- The service worker stops reading and writing its cache for `/api/*` requests:
  catalog calls are network-only, and the cache version is bumped so payloads
  cached by earlier builds are purged.
- Webapp tests cover all three behaviors.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `face-recognition`: the People tab SHALL reflect the live catalog — its
  selected person is reconciled after a refresh, the catalog is reloaded when
  the tab becomes visible, and a catalog response cached by an earlier build is
  never replayed as the current list.

## Non-goals

- Changing face clustering or recognition accuracy, including whatever created
  the unnamed duplicate cluster in the first place.
- Changing merge/rename API semantics, person-name uniqueness, or the backend
  `GET /api/people` contract.
- Adding offline support for the People tab.

## Impact

- `webapp/src/components/tabs/PeopleTab.tsx` — detail-view reconciliation and
  refresh on visibility.
- `webapp/public/sw.js` — network-only handling for `/api/*` and a cache-version
  bump.
- `webapp/src/components/tabs/PeopleTab.test.tsx` and a new service-worker
  contract test.
- `.focused-spec/evidence.yaml` — evidence entries for the new scenarios.
- No backend, OpenAPI, MCP, database or dependency changes.
