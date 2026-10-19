# Test ↔ Spec Coverage Audit

Date: 2026-10-19
Scope: every executable test in the repository versus `.focused-spec/evidence.yaml`
and the master specifications in `openspec/specs/**/spec.md`.

## Summary

- Enumerated **608** executable test cases: Go `Test*`/`Benchmark*`, Vitest
  `it`/`test`, and pytest `test_*` (52 added by this pass).
- A test counts as *mentioned* only when its exact selector is registered in the
  evidence registry (and therefore bound to a `focused-spec:id` in a master
  specification).
- **All Vitest tests are registered**, including every webapp component, i18n,
  theme, library, and HTTP-integration test.
- **372 tests remain unregistered**; all are Go unit, characterization, or
  benchmark tests of internal packages that no master specification describes.
  They are listed by area below and are intentionally outside spec scope.
- Every previously uncovered master specification that had executable evidence
  gaps now has bound scenarios: auth-localization, account-registration,
  ocr-lightbox, branding, third-party-components, and trash.

## Changes made

| Specification | Focused items before | Focused items after | Notes |
|---|---:|---:|---|
| account-registration | 24 | 29 | `UserDTO` status serialization, audit entries, pending queue, reject dialog, status badges |
| auth-localization | 11 | 15 | Auth language switcher presence, registration mode, immediate re-render, accessibility |
| branding | 0 | 11 | Auth/sidebar logos, logo accessibility, favicon + precache, PWA icons, maskable safe zone, manifest |
| ocr-lightbox | 0 | 5 | Fit-to-visible (text/no-text/rotated) and bounding-box alignment/resize |
| third-party-components | 0 | 7 | Combined list, backend attribution, own-module exclusion, license links, locale consistency |
| trash | 10 | 17 | Cursor pagination, empty response, thumbnails, grid uniformity, tile actions, localization |
| **Total** | **160** | **199** | 229 unique evidence targets |

Validation and execution:

```text
focused-spec validate --scope current --syntax-only   # structurally valid
focused-spec validate --scope current --strict        # 199 items, 229 targets
focused-spec run --scope current                      # 199 PASS, 0 FAIL/SKIP/ERROR
```

Component checks:

```text
backend/core: gofmt + go test ./... -count=1          # PASS
webapp:       npm run lint && npx tsc -b               # PASS
```

Sensitivity was verified with controlled product regressions. Every probe was
reverted and the full scope re-ran green afterwards:

- `auth-localization.switcher-*` — removed the switcher's accessible name
  (three scenarios failed); made `onValueChange` a no-op (`switcher-rerender`
  failed).
- `branding.auth-logo*` — changed the welcome-logo `src` (all three auth-logo
  scenarios failed).
- `branding.sidebar-*` / `branding.logo-accessible` — changed the base-logo
  `alt`, then the collapsed rail width (`sidebar-logo`, `logo-accessible`,
  `sidebar-collapsed-mark` failed).
- `account-registration.admin-queue-*` — swapped the Approve/Reject labels,
  disabled the empty-reason guard, and remapped the pending badge
  (queue, reject-dialog, and status-badge scenarios failed).
- `ocr-lightbox.*` — dropped `max-w-full` from the image and broke the
  bounding-box scale factor (all five scenarios failed).
- `trash.overlay-two-buttons` / `trash.restore-from-tile` / `trash.thumbnails` /
  `trash.grid-uniform` — relabelled the Restore action, inverted the thumbnail
  branch, and removed a grid breakpoint (all four scenarios failed).
- `branding.favicon*` / `branding.pwa-icons` / `branding.maskable-safe-zone` /
  `branding.manifest-icons` — changed the favicon link and precache entry,
  removed the 384px icon, and changed the maskable purpose (all five scenarios
  failed).
- `third-party.*` — removed the React and GORM entries, redirected the Gin
  license link, changed the component link target, and injected a
  Flashbacks-owned module (combined-list, backend-attributed,
  backend-license-links, frontend-preserved, license-links, and
  own-modules-excluded failed).
- `trash.cursor-pagination` — reverted the date-group overflow query to
  `id < ?` (scenario failed); `trash.empty-response` — returned a nil group
  slice (failed); `trash.thumbnails` — dropped the generated thumbnail
  assignment (failed).
- `account-registration.user-dto-status-fields` — changed the `accountStatus`
  JSON tag (failed); `account-registration.audit-entries-recorded` — rewrote
  the three audit actions (failed).
- `trash.localization` — removed `trashTab.restore` from the Russian locale
  (parity scenarios failed).

Not mutation-checked: `third-party.locale-consistent`, because the component
list is static data and the component renders the same entries for both locales,
so there is no product behaviour to regress. `branding.auth-logo-all-modes`
covers registration mode only; the bootstrap setup path renders the same logo
but is not separately asserted. `ocr-lightbox.*` assertions are component-level
(fit classes and computed overlay geometry) rather than real browser layout
measurements.

## Product issues found while adding coverage

- **Trash cursor pagination duplicated a boundary date group** when `trash_items`
  ids did not descend together with `deleted_at` (for example rows backfilled in
  arbitrary order). `handleGetTrash` selected same-day overflow rows by `id` only;
  it now continues after the ordering key `(deleted_at, id)`. Covered by
  `trash.cursor-pagination`.
- **Re-approving a rejected account is not supported**, although the
  specification's admin-approval-queue requirement lists it. `ApproveUser`
  returns `auth.user_not_pending` for anything that is not `pending`, and the
  user card has no approve action. The queue tests therefore cover the pending
  queue, reject dialog, and status badges only; the re-approve scenario remains
  an open product gap.

## Remaining specification gaps (behaviour without tests)

These master-spec scenarios have no automated test and therefore cannot be
bound. They are coverage gaps in the test suite, not missing specification text.

### account-registration

- `ACCOUNT_CREATION_MODE` default (`admin_only`) and invalid-value startup
  rejection.
- Migration compatibility (existing users become `active`, `lower(login)` unique
  index).
- Mode-switching semantics for existing accounts.
- Frontend registration flow (auto-login, pending screen, admin-only hiding).
- Re-approving a rejected account (product support missing, see above).
- Specific localized account/approval strings.

### auth-localization

- "Pre-authentication choice does not overwrite the account" end-to-end.

### branding

- Bootstrap setup mode is not separately asserted for the logo (the shared auth
  card renders it unconditionally).

### ocr-lightbox

- No remaining scenario gaps; residual risk is that fit and alignment are
  asserted through component classes and computed geometry rather than a real
  browser layout pass.

### third-party-components

- No remaining scenario gaps; the locale scenario is asserted at the rendered
  list level rather than by switching a live authenticated locale.

### trash

- Infinite scroll (intersection observer) and the shared empty state.
- Lightbox metadata display (original location and deletion date).

### design-system

- The specification contains many design-review and accessibility rules
  (tokens, button/select primitives, dialogs, empty/loading states, settings
  patterns, typography, drift inventory) that are enforced by review rather than
  executable tests. New executable mobile/lightbox/theme/progress scenarios are
  bound.

## Remaining unregistered tests (internal implementation)

These tests are valid engineering tests but describe implementation details that
no master specification owns. They should stay in the suite; registering them
would create requirements the product does not expose to users.

| Area | Tests | Scope |
|---|---:|---|
| LLM agent & conversations | 41 | Agent loop, token accounting, summarization, conversation store |
| Imaging pipeline | 166 | Scanner, OCR workers/managers/batches, smart search, background sync, embedding backfill, thumbnails, statement counter, tag scan |
| Auth internals | 60 | User CRUD, session repository, password helpers, rate limiter, bootstrap service, account approval units |
| OCR classifier & image processing | 35 | Preprocessing, skew detection, confidence decision rule, characterization and benchmarks |
| Geocoding & image utilities | 26 | Nominatim client, geographic clustering, image resize |
| Handler helpers & DTOs | 16 | Gallery builders, LLM handler helpers, media DTO |
| LLM provider clients | 12 | DeepSeek/OpenAI/Ollama protocol tests beyond the capability parsing already bound |
| Registry internals | 10 | Config validation, secret codec, embedded key-value backend |
| i18n & repositories | 3 | Backend i18n service, LLM repository |
| Exif config | 3 | Environment loading |

The user-visible behaviour of the imaging pipeline that does have a master
specification (scan status counters, result equivalence, hashing, batching) is
already registered under `gallery-browsing`.

## Follow-up recommendations

1. Implement re-approval of `rejected` accounts (service guard, admin action, and
   the corresponding component/integration test) so the admin approval queue
   requirement is fully satisfied.
2. Add an end-to-end test for "pre-authentication choice does not overwrite the
   account" once a browser-level harness is available.
3. Add Trash tests for infinite scroll, the shared empty state, and lightbox
   metadata (original location and deletion date).
4. Add frontend registration-flow tests (auto-login, pending screen, admin-only
   hiding) to close the remaining account-registration UI gap.
5. Keep the brand/third-party asset and snapshot checks green when dependencies
   or brand assets change; update the AboutTab snapshot deliberately in the same
   commit as the dependency change.
