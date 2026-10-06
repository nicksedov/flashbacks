## Why

The webapp currently serves a generic/outdated favicon and PWA app icons that do not reflect the official Flashbacks brand. The user has provided a new high-resolution branded icon (`webapp/public/OIG4.png`) and wants the favicon and PWA icons to be replaced with it so the product identity is consistent across the browser tab, installed PWA, and home screen.

## What Changes

- Replace the browser favicon with the new branded icon derived from `OIG4.png`.
- Replace the PWA app icons (all sizes: 72–512px, `apple-touch-icon`, and maskable variants) with versions generated from the new branded icon.
- Update the PWA icon generation script to source the new branded image instead of the current SVG glyph.
- Update any asset references (`index.html`, `manifest.json`, service worker cache list) to the new icon files.

## Capabilities

### New Capabilities

<!-- None introduced. -->

### Modified Capabilities

- `branding`: The existing branding spec governs the auth/sidebar logos; this change extends brand identity to the favicon and PWA app icons by adding requirements for the app-icon source and its consistent rendering.

## Non-goals

- No changes to the welcome-screen or sidebar logos (`flashbacks_logo_welcomescreen.png`, `flashbacks_logo_base.png`).
- No changes to the visual design of the source image itself (used as-is, scaled as needed).
- No backend, API, or i18n changes.

## Impact

- `webapp/public/favicon.svg` (or its replacement) — favicon asset.
- `webapp/public/icon-*.png`, `webapp/public/apple-touch-icon.png`, `webapp/public/maskable-*.png` — regenerated PWA icons.
- `webapp/scripts/generate-pwa-icons.js` — updated to read the new branded source image.
- `webapp/index.html` — favicon/apple-touch-icon references.
- `webapp/public/manifest.json` — icon entries (filenames/types/sizes).
- `webapp/public/sw.js` — precache list for renamed/new icon assets.
