# Spec Delta

## Purpose

Defines how the webapp chooses and lets users change the display language before they authenticate, so the login and registration screens are readable in the user's own language independently of any account settings.

## ADDED Requirements

### Requirement: Language switcher on the authentication screen

While no user is authenticated, the webapp SHALL display a language switcher on the login/registration authentication screen that offers English and Русский and applies the selected language immediately. The switcher SHALL be available in both the login and the registration mode of that screen, and is not required on the bootstrap admin setup screen.

#### Scenario: Switcher shown on the login screen

- **WHEN** an unauthenticated user opens the webapp
- **THEN** the authentication screen shows a language switcher offering English and Русский

#### Scenario: Switcher available in registration mode

- **WHEN** the user switches the authentication screen from login to registration mode
- **THEN** the language switcher remains available and reflects the currently selected language

#### Scenario: Selecting a language re-renders the screen

- **WHEN** the user selects Русский in the language switcher
- **THEN** the authentication screen text is shown in Russian immediately, without a page reload

#### Scenario: Switcher is accessible

- **WHEN** the language switcher is rendered
- **THEN** it exposes an accessible name and is operable by keyboard

#### Scenario: Bootstrap setup screen is out of scope

- **WHEN** the app shows the bootstrap admin setup screen
- **THEN** the language switcher is not required on that screen

### Requirement: Default language derived from the system locale

The webapp SHALL derive the initial authentication-screen language from the browser/system locale: Russian when that locale is Russian, English when it is English, and English for any other locale.

#### Scenario: Russian system locale selects Russian

- **WHEN** the browser/system locale is Russian (for example `ru` or `ru-RU`)
- **THEN** the authentication screen initially renders in Russian

#### Scenario: English system locale selects English

- **WHEN** the browser/system locale is English (for example `en` or `en-US`)
- **THEN** the authentication screen initially renders in English

#### Scenario: Unsupported system locale falls back to English

- **WHEN** the browser/system locale is neither English nor Russian (for example `de` or `fr`)
- **THEN** the authentication screen initially renders in English

### Requirement: Pre-authentication language is independent of account settings

The authentication-screen language SHALL be chosen and changed independently of any authenticated user's saved language preference, because no account is logged in while that screen is shown. When a user authenticates, the UI language SHALL follow the account's saved `language` setting, and a language chosen before login MUST NOT overwrite that saved setting.

#### Scenario: No account setting is consulted before login

- **WHEN** the authentication screen resolves its language
- **THEN** it uses the system-locale default or the remembered pre-authentication choice, not a per-user setting

#### Scenario: Authenticated UI follows the account setting

- **WHEN** an authenticated user's saved language is `ru`
- **THEN** the application renders in Russian regardless of the pre-authentication choice

#### Scenario: Pre-authentication choice does not overwrite the account

- **WHEN** the user selects Русский on the authentication screen and then signs in to an account whose saved language is `en`
- **THEN** the account's saved language remains `en` and the application follows it after login

### Requirement: Pre-authentication language choice is remembered

The webapp SHALL remember the user's pre-authentication language choice across page reloads and browser sessions. A remembered supported language SHALL take precedence over the system-locale default, and an unsupported or missing remembered value SHALL be ignored in favour of the system-locale default.

#### Scenario: Choice persists across reloads

- **WHEN** the user selects Русский on the authentication screen and reloads the page
- **THEN** the authentication screen opens in Russian

#### Scenario: Remembered choice overrides the system default

- **WHEN** a remembered supported language exists
- **THEN** that language is used for the authentication screen instead of the system-locale default

#### Scenario: Unsupported remembered value is ignored

- **WHEN** the remembered value is missing or is not a supported language
- **THEN** the authentication screen falls back to the system-locale default

### Requirement: Localized switcher text

The language switcher SHALL expose an accessible label in both English and Russian, kept in sync, and SHALL introduce no new translation keys: that label reuses the existing `settings.language` key. The option names SHALL be rendered as literal endonyms, identical in both locales.

#### Scenario: en/ru parity for the switcher label

- **WHEN** the language switcher is rendered
- **THEN** its accessible label resolves from the existing `settings.language` key, which exists in both `translations.en.ts` and `translations.ru.ts`, and the translation parity test passes
- **AND** the options read `English` and `Русский` as literal endonyms in both locales

The following strings MUST be present (the accessible label as the `settings.language` translation key, and the option names as literal endonyms shown identically in both locales):

| Context | English (en) | Russian (ru) |
|---|---|---|
| Switcher accessible label (`settings.language`) | Language | Язык |
| Option: English (literal endonym) | English | English |
| Option: Russian (literal endonym) | Русский | Русский |
