# Proposal

## Why

Unauthenticated users reach the login/registration screen before any account settings can be loaded, so the app currently hardcodes the initial language to English. A Russian-speaking user therefore reads the entire sign-in flow in English until they log in and change their preference, and there is no way to switch language before authenticating.

## What Changes

- Add a compact language switcher (English / Русский) to the authentication screen, available in both the login and registration modes.
- The switcher drives the language of the pre-authentication experience only, independently of any stored user settings (no account is logged in while the screen is shown).
- Resolve the default pre-authentication language from the browser/system locale: `ru` when the system language is Russian, `en` when it is English, and `en` for any other system language.
- Remember the chosen pre-authentication language across reloads (browser local storage) so the auth screen reopens in the chosen language.
- After a successful login the UI language returns to the account's saved user setting (existing behavior); the pre-auth choice never overwrites the account setting.

## Capabilities

### New Capabilities
- `auth-localization`: language selection and resolution for the authentication screens — a language switcher on the auth (login/registration) screen, a system-locale-based default that falls back to English, and independence from authenticated user settings.

### Modified Capabilities
<!-- No existing capability requirements change. -->

## Impact

- Webapp only; no backend, API, or database changes.
- Affected code: `webapp/src/providers/SettingsProvider.tsx` (extended with the pre-auth language state), `webapp/src/i18n/*`, `webapp/src/components/auth/LoginScreen.tsx`, and the `translations.en.ts` / `translations.ru.ts` locale files.
- Reuses the existing `Language` union (`"en" | "ru"`), the `useTranslation` hook, and the shared `Button`/`Select` primitives under the design-system rules.
- No OpenAPI or MCP contract changes.

## Non-goals

- No change to the authenticated language preference flow (the settings tab keeps persisting `language` to the account).
- No support for languages beyond English and Russian.
- No language switcher on the bootstrap admin setup screen — this change scopes the switcher to the login and registration screen only.
- No server-side persistence of the pre-authentication choice.
