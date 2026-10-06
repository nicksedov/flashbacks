# Tasks

## 1. Language resolution helpers

- [x] 1.1 Add `webapp/src/i18n/language.ts` exporting `AUTH_LANGUAGE_STORAGE_KEY` (`"flashbacks.authLanguage"`), `resolveSystemLanguage`, `readStoredAuthLanguage`, `writeStoredAuthLanguage`, `resolveInitialAuthLanguage`, and `selectActiveLanguage`, and re-export them from `webapp/src/i18n/index.ts`; verify `cd webapp && npm run lint && npx tsc -b` passes.
- [x] 1.2 Add `webapp/src/i18n/language.test.ts` covering `ru`/`en`/unsupported primary subtags, remembered-value precedence over the system default, ignored invalid or missing stored values, and a throwing storage stub; verify `cd webapp && npm test` passes.

## 2. Pre-authentication language state

- [x] 2.1 Extend `SettingsProvider` with `authLanguage` state seeded from `resolveInitialAuthLanguage()` and a `setAuthLanguage` setter that persists the choice via `writeStoredAuthLanguage`, and add both to `SettingsContextValue` in `webapp/src/providers/settingsContext.ts`; verify `cd webapp && npm run lint && npx tsc -b` passes.
- [x] 2.2 Drive `I18nProvider` from `selectActiveLanguage(isAuthenticated, language, authLanguage)` in `webapp/src/providers/SettingsProvider.tsx` so the account language wins only after login; verify `cd webapp && npm run lint && npx tsc -b` passes and the Vitest render test `webapp/src/i18n/activeLanguage.test.ts` asserts the unauthenticated tree uses `authLanguage` while the authenticated tree uses the account `language`.

## 3. Language switcher on the auth screen

- [x] 3.1 Add a compact language switcher to the auth card header in `webapp/src/components/auth/LoginScreen.tsx` using the shared `Select` primitive (globe icon, options `English` and `Русский`) bound to `authLanguage`/`setAuthLanguage`, shown in both login and registration modes; verify `cd webapp && npm run lint && npx tsc -b` passes and that switching updates the screen text immediately in both modes.
- [x] 3.2 Give the switcher an accessible name (reuse the existing `settings.language` key) and ensure keyboard operability and a 44×44 touch target; if any new translation key is introduced, add it to `webapp/src/i18n/translations.en.ts` and `webapp/src/i18n/translations.ru.ts` together and verify `cd webapp && npm run lint && npx tsc -b && npm test` passes.

## 4. Integration verification

- [x] 4.1 Verify observable behavior end-to-end: an unsupported system locale renders English, a `ru` system locale renders Russian, a remembered choice survives a page reload, and selecting Русский before login does not change an account whose saved language is `en`; run `cd webapp && npm run lint && npx tsc -b && npm test`.
- [x] 4.2 Confirm the switcher uses only shared primitives and semantic tokens and record in `docs/design-system.md` whether any new drift was introduced (expected: none); verify the note matches the final UI.
