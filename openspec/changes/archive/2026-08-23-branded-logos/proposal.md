## Why

The webapp currently uses a generic pictogram (an inline SVG "F" glyph in the sidebar, a `ShieldAlert` icon on the login card) plus text for the product identity. Branded logo assets already exist in `webapp/public/` but are not used in the UI. Replacing the placeholder icons with the official branded logos gives the auth screen and sidebar a polished, on-brand identity without any functional changes.

## What Changes

- **Auth screen (login/register/bootstrap):** Replace the `ShieldAlert` icon circle + `CardTitle` heading with the branded welcome-screen logo [`flashbacks_logo_welcomescreen.png`](../../../webapp/public/flashbacks_logo_welcomescreen.png). The existing `CardDescription` text stays.
- **Sidebar header (top):** Replace the inline SVG glyph + "Flashbacks" `h1` with the branded base logo [`flashbacks_logo_base.png`](../../../webapp/public/flashbacks_logo_base.png).
- **Collapsed sidebar:** The full wordmark logo cannot fit in the narrow collapsed rail, so the collapsed header keeps a small centered logo mark rather than the full wordmark (see design for the exact treatment).
- **Scaling:** Both images render with `height: auto` and a constrained width so they are proportional to the area they replace and do not dominate the layout. Auth logo is sized to fit comfortably within the card header; sidebar logo fits within the sidebar header height and horizontal padding.
- **Accessibility:** The `<img>` elements get meaningful `alt`/`aria-label` text; no new i18n keys are introduced (the logo itself carries the brand name).

## Capabilities

### New Capabilities

- `branding`: Defines where the official Flashbacks logos appear in the webapp UI (auth screen and sidebar), their rendering/scaling rules, and the collapsed-sidebar fallback behavior.

### Modified Capabilities

- *(none — no existing spec-level behavior is changing)*

## Non-goals

- No changes to colors, fonts, spacing tokens, or the design system tokens themselves.
- No changes to the logo asset files (bitmaps stay exactly as provided in `webapp/public/`).
- No changes to authentication logic, forms, or the `Header` component.
- No backend/API changes and no OpenAPI contract changes.
- No new localizations or i18n keys.

## Impact

- **Code:** `webapp/src/components/auth/LoginScreen.tsx`, `webapp/src/components/layout/Sidebar.tsx`.
- **Assets:** referenced (not modified) `webapp/public/flashbacks_logo_welcomescreen.png`, `webapp/public/flashbacks_logo_base.png`.
- **Tests:** `npm run lint && npx tsc -b`; `npm test` (Vitest) must stay green. No new unit tests required (pure presentational change).
- **Backend/API:** none.
