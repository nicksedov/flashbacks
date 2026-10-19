# Test ↔ Spec Coverage Audit

Date: 2026-10-19
Scope: every executable test in the repository versus `.focused-spec/evidence.yaml`
and the master specifications in `openspec/specs/**/spec.md`.

## Summary

- Enumerated **556** executable test cases: Go `Test*`/`Benchmark*`, Vitest
  `it`/`test`, and pytest `test_*`.
- A test counts as *mentioned* only when its exact selector is registered in the
  evidence registry (and therefore bound to a `focused-spec:id` in a master
  specification).
- **All Vitest tests are now registered**, including every webapp component,
  i18n, theme, library, and HTTP-integration test.
- **372 tests remain unregistered**; all are Go unit, characterization, or
  benchmark tests of internal packages that no master specification describes.
  They are listed by area below and are intentionally outside spec scope.
- Six master specifications previously had **zero** focused-spec bindings:
  account-registration, auth-localization, branding, ocr-lightbox,
  third-party-components, trash. Four of them had executable tests and are now
  bound; branding, ocr-lightbox, and third-party-components have no tests at all.

## Changes made

| Specification | Focused items before | Focused items after | Notes |
|---|---:|---:|---|
| account-registration | 0 | 24 | Bootstrap setup, lifecycle, registration modes, validation, approval queue API, admin-created accounts |
| auth-localization | 0 | 11 | Locale defaults, pre-auth persistence, active-language rendering, en/ru parity |
| design-system | 42 | 49 | Mobile tabs, admin action row, theme selection, determinate progress, info trigger, consistent lightbox stage; extra evidence on mobile toolbar/lightbox rules |
| gallery-browsing | 12 | 18 | New requirement: gallery folder configuration (add/list/remove/validation) |
| llm-settings | 7 | 8 | Unknown instrument type fails closed |
| service-registry | 14 | 17 | Registry client registration, heartbeat, configuration watch |
| trash | 0 | 10 | Record on delete, grouping, restore, permanent delete, clean, image serving |
| **Total** | **98** | **160** | 184 unique evidence targets |

Validation and execution:

```text
focused-spec validate --scope current --syntax-only   # structurally valid
focused-spec validate --scope current --strict        # 160 items, 184 targets
focused-spec run --scope current                      # 160 PASS, 0 FAIL/SKIP/ERROR
```

Sensitivity was verified with controlled product regressions for representative
new bindings (each failed on the regression and passed after restore):

- `auth-localization.default-unsupported-fallback` — changed the unsupported
  locale fallback in `language.ts`; the test failed on the behavior assertion.
- `design-system.admin-settings-tabs-wrap` — removed `flex-wrap` from the admin
  tab list; the test failed.
- `trash.file-moved-recorded` — wrote an empty `OriginalPath` in the trash
  mover; the test failed.

A per-reference mutation sweep of all 62 newly registered items was not
performed; the remaining references bind pre-existing, already passing tests and
should be mutation-checked incrementally when the corresponding product code is
next changed.

## Remaining specification gaps (behaviour without tests)

These master-spec scenarios have no automated test and therefore cannot be
bound. They are coverage gaps in the test suite, not missing specification text.

### account-registration

- `ACCOUNT_CREATION_MODE` default (`admin_only`) and invalid-value startup
  rejection.
- `UserDTO` camelCase status fields (`accountStatus`, `statusChangedAt`,
  `rejectionReason`) serialization.
- Audit entries (`register_user`, `approve_user`, `reject_user`).
- Migration compatibility (existing users become `active`, `lower(login)` unique
  index).
- Mode-switching semantics for existing accounts.
- Frontend registration flow (auto-login, pending screen, admin-only hiding).
- Admin approval queue UI (pending queue, reject dialog, status badges,
  re-approve).
- Specific localized account/approval strings.

### auth-localization

- Language switcher on the login and registration screens (presence, re-render,
  keyboard accessibility, absence on bootstrap setup).
- "Pre-authentication choice does not overwrite the account" end-to-end.

### branding

- All 11 scenarios (auth/sidebar logos, favicon, PWA icons, manifest) have no
  automated test. Logo rendering is verified manually or through the build.

### ocr-lightbox

- All 5 scenarios (image fits the visible area, bounding boxes stay aligned,
  resize behaviour) have no dedicated test. The generic lightbox image-stage
  consistency is covered, but fit-to-visible and overlay alignment are not.

### third-party-components

- All 7 scenarios (combined list, backend attribution, license links, locale
  consistency) have no automated test.

### trash

- Cursor pagination across date groups, empty-trash response, thumbnails, grid
  uniformity with the gallery, two-button tile overlay, lightbox metadata,
  restore-from-tile, infinite scroll, empty state, localization.

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

1. Add component tests for the auth language switcher and the admin approval
   queue; those are the largest remaining user-facing spec gaps.
2. Add OCR-lightbox tests for fit-to-visible and bounding-box alignment.
3. Add trash tests for cursor pagination, thumbnails, and tile overlay actions.
4. Add a persistence/DTO test for the account status fields and audit entries.
5. Keep branding and third-party-components as review/asset checks; if
   regression protection is required, add a manifest/asset test and a
   component-list snapshot test.
