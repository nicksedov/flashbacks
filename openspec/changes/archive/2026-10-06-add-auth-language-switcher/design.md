# Design

## Context

See [`proposal.md`](proposal.md) for motivation and [`specs/auth-localization/spec.md`](specs/auth-localization/spec.md) for the required behavior.

Current state that shapes the approach:

- The provider tree is `AuthProvider` → `SettingsProvider` → `App` (see [`main.tsx`](../../../webapp/src/main.tsx)). `SettingsProvider` owns the display `language`, initialises it to `"en"`, and only loads the account's saved language once a user is authenticated ([`SettingsProvider.tsx`](../../../webapp/src/providers/SettingsProvider.tsx)). It renders `I18nProvider language={language}`.
- `App` renders the auth screens ([`LoginScreen.tsx`](../../../webapp/src/components/auth/LoginScreen.tsx), `BootstrapSetupScreen`) when unauthenticated — inside that same `I18nProvider`.
- Because no account fetch happens while unauthenticated, the auth screen currently always renders in `en`; there is also no language control on it.
- `SettingsProvider.setLanguage` persists to the account via `updateUserSettings`, so it is unusable before login.
- The design system requires shared primitives (`Button`, `Select`), semantic tokens, accessible names, keyboard operability, and 44×44 touch targets.

## Goals / Non-Goals

**Goals:**

- Let an unauthenticated user switch the authentication screen between English and Русский.
- Resolve the initial auth-screen language from the system locale, falling back to English.
- Keep the pre-auth language fully independent from any account's saved `language`.
- Remember the pre-auth choice across reloads without contacting the backend.

**Non-Goals:**

- Any backend, API, schema, or migration work.
- Adding languages beyond `en`/`ru`.
- A switcher on the bootstrap admin setup screen.
- Changing the authenticated settings flow.

## Decisions

### D1: Drive `I18nProvider` from a separate pre-auth language source

Introduce an `authLanguage` value that controls the UI while unauthenticated, and keep the existing account `language` for when authenticated. A pure helper (D2) selects which one is active:

```
I18nProvider language={selectActiveLanguage(isAuthenticated, language, authLanguage)}
```

`authLanguage` starts from a synchronous resolver (D2) and is exposed with a setter so the auth screen can change it.

- Alternative A — reuse `language`/`setLanguage`: rejected. `setLanguage` writes to the account (`updateUserSettings`) and there is no account before login, which would violate the independence requirement and could not default from the system locale.
- Alternative B — a brand-new standalone `AuthLanguageProvider` wrapping `SettingsProvider`: viable, but it duplicates provider wiring and risks two sources of truth around auth state. Extending `SettingsProvider` (which already knows `isAuthenticated`) keeps a single language authority.

The auth-language state and setter are exposed through the existing settings context (extend `SettingsContextValue` with `authLanguage` / `setAuthLanguage`).

### D2: Pure, testable language-resolution helpers

Add `webapp/src/i18n/language.ts` with pure functions and a storage key:

- `AUTH_LANGUAGE_STORAGE_KEY = "flashbacks.authLanguage"`.
- `resolveSystemLanguage(languages?: readonly string[]): Language` — take the first locale (`navigator.languages[0] ?? navigator.language`), lowercase, keep the primary subtag; return `"ru"` for `ru*`, `"en"` for `en*`, otherwise `"en"`.
- `resolveInitialAuthLanguage(): Language` — return a stored supported value if present, else `resolveSystemLanguage()`.
- `readStoredAuthLanguage(): Language | null` / `writeStoredAuthLanguage(language: Language): void` — storage access guarded by `try/catch`.
- `selectActiveLanguage(isAuthenticated, accountLanguage, authLanguage): Language` — return the account language when authenticated, otherwise the pre-auth language; extracted as a pure helper so the active-language selection is directly unit-testable (see `activeLanguage.test.ts`).

Rationale: pure functions are directly unit-testable with Vitest (project convention) and keep the React layer thin. Alternatives considered: inline logic in the provider (untestable) and an external i18n library (unnecessary dependency for two locales).

### D3: Persist the choice in browser local storage

Store under the namespaced key `flashbacks.authLanguage`, accepting only `"en"`/`"ru"`.

- Rationale: survives reloads and browser sessions, needs no backend, and stays separate from the account.
- Alternatives: `sessionStorage` (lost on browser close), a cookie (sent to the server unnecessarily; no server consumer), in-memory only (lost on reload).
- Storage failures (private mode, disabled storage) are swallowed; the app falls back to the system-locale default and simply does not remember the choice.

### D4: Render the switcher with the shared `Select` primitive

Place a compact language switcher in the auth card header so it is visible in both login and registration modes, and implement it with the shared `Select` primitive already used for language in `SettingsTab`, labelled with the globe icon.

- Rationale: satisfies the design-system rule that exactly one shared `Select`/tabs primitive exists; avoids introducing a bespoke segmented control.
- Alternative — a two-button segmented toggle mirroring the existing login/register toggle: more discoverable, but the existing toggle is a raw-`<button>` control (design-system drift) and inventing a second control type is discouraged; rejected.
- Accessibility: expose an accessible name on the trigger, keep it keyboard operable with a visible focus ring, and respect the 44×44 touch target.

### D5: Show language names as endonyms

The options read `English` and `Русский` regardless of the active UI language, so a user can always locate their language. The control's accessible label reuses the existing `settings.language` key (`Language` / `Язык`); no new translation keys are required. If any new key is added it MUST be added to both `translations.en.ts` and `translations.ru.ts` to keep the parity test green.

### D6: The account setting wins after login, without copying

Once `isAuthenticated` is true, `I18nProvider` uses the account `language` (D1). The switcher never calls `updateUserSettings`, so the pre-auth choice cannot overwrite the account and does not persist server-side.

## Risks / Trade-offs

- **Wrong-language flash on first paint** → resolve `authLanguage` synchronously during initial state (local storage + `navigator`), never via an async fetch.
- **local storage unavailable or blocked** (private mode, disabled) → all storage access is wrapped in `try/catch`; fall back to the system-locale default and skip persistence.
- **Extended settings context surface** (`authLanguage` alongside `language`) may read ambiguously → document both fields and their scopes in the context type; the authenticated path is unchanged.
- **Endonym labels vs UI language** → English/Русский stay recognisable to both audiences; recorded as the intended behavior in the spec.
- **Locale detection is first-locale only** → deterministic and matches the requirement; a user whose top locale is unsupported but whose second is Russian sees English until they switch (acceptable, and the switcher is right there).

## Migration Plan

No data migration. This is a webapp-only change with no backend, API, or database impact. Deploy by shipping the frontend build; roll back by reverting the change's commit. Existing users are unaffected because the authenticated language path is untouched.
