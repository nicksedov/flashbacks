# Proposal

## Why

The browser suite in `tests/e2e` currently covers only the People catalog. The
rest of the core service — sign-in, gallery browsing, the lightbox modes, moving
between workspace tabs, the empty/degraded states of search and deduplication,
and the maintenance flows (trash, folders, settings) — is verified only by Vitest
units and the HTTP integration suite. Those layers cannot catch a broken tab, a
lightbox that fails to switch mode, a missing empty state or a selector that no
longer matches: only a real browser over a real deployment can.

The harness and the CI command already exist, so extending coverage now is cheap
and closes the largest remaining gap in the project's automated verification.

## What Changes

- Extend the Playwright suite with flow-oriented specs:
  - **auth** — unauthenticated start shows the login screen, a wrong password
    reports an error, a valid sign-in reaches the workspace, sign-out returns to
    the login screen.
  - **gallery** — the all-images grid renders seeded photos, opens a photo in the
    shared lightbox, switches between EXIF / OCR / AI / Tags modes, shows image
    metadata, and closes back to the grid.
  - **navigation and degraded states** — every sidebar destination (Gallery
    sub-modes, Smart Search, Deduplication, Text Recognition, Metadata, Trash,
    People, About, admin settings/users) renders its screen without a blank page
    or uncaught console error; Smart Search without an embedding index and
    deduplication without groups show their documented empty states instead of
    errors.
  - **destructive maintenance** (gated by `E2E_DESTRUCTIVE=1`) — trash a
    disposable photo and restore it, add and remove a temporary gallery folder,
    run a duplicate batch delete over a copied pair, and save-then-restore a
    setting. Each flow creates and removes its own fixtures.
- Harden the shared harness: anonymous-page fixture for auth, console-error
  collection, navigation helpers, a temporary gallery-folder fixture built from
  a copied dataset image, and unique per-run names.
- Add any missing `data-slot` hooks needed to target gallery tiles, lightbox
  mode tabs and maintenance dialogs stably.
- Document the new specs, the `E2E_DESTRUCTIVE` flag and the fixture lifecycle in
  `tests/e2e/README.md` and `docs/agent/webapp.md`.

## Capabilities

### New Capabilities

None — this change adds browser coverage for behavior that is already specified;
it does not change requirements or scenarios (`skip_specs: true`).

### Modified Capabilities

None — no requirement or scenario changes.

## Non-goals

- No product behavior, API contract or localization changes; UI hooks added for
  testing are inert attributes only.
- No new CI service or runner provisioning: the suite keeps running through
  `npm run test:e2e` (optionally with the existing tunnel mode).
- No visual-regression snapshots, no OCR/LLM quality assertions (those need model
  services and belong to their own suites).
- Destructive flows never run by default and never touch a shared environment
  without `E2E_DESTRUCTIVE=1`.
- No coverage of mobile layout beyond the existing responsive breakpoints.

## Impact

- **Tests**: `tests/e2e/` gains auth, gallery, navigation/degraded-state and
  destructive specs plus fixture helpers; `playwright.config.ts` may gain a
  project-level flag for the destructive suite.
- **Webapp**: a small number of `data-slot`/`data-testid` attributes on gallery
  tiles, lightbox mode tabs and maintenance dialogs; no behavior change.
- **Docs**: `tests/e2e/README.md`, `docs/agent/webapp.md`.
- **Runtime**: read-only specs run against any authenticated deployment; the
  destructive suite requires a disposable test deployment (the local test stack)
  and a writable media folder.
