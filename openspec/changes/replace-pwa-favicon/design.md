## Context

The webapp currently serves a favicon and PWA app icons generated from a hand-crafted SVG glyph (`webapp/public/favicon.svg`, 48×46 viewBox). See proposal.md - Why for motivation. The user provided a new high-resolution branded icon, `webapp/public/OIG4.png`, which is a 728×728 square PNG with an alpha channel. All existing PWA icon sizes (72–512px, `apple-touch-icon`, maskable variants) are generated at build time by `webapp/scripts/generate-pwa-icons.js` using `sharp`, reading `favicon.svg` as the source.

Constraints:
- Asset filenames are referenced by `webapp/public/manifest.json`, `webapp/index.html`, and `webapp/public/sw.js`. Keeping the existing PWA icon filenames stable avoids unnecessary churn.
- `sharp` is already a devDependency of the webapp, so no new dependencies are required.

## Goals / Non-Goals

**Goals:**
- Make `OIG4.png` the single canonical source for the browser favicon and all PWA app icons.
- Regenerate the full set of PNG icons (regular + maskable) from the new source without changing their public filenames.
- Update HTML and service worker references so the branded icon is actually served and precached.

**Non-Goals:**
- Not changing the auth/sidebar logos (`flashbacks_logo_welcomescreen.png`, `flashbacks_logo_base.png`) — see proposal.md - Non-goals.
- Not introducing a build-time pipeline beyond the existing `generate-pwa-icons.js` script.
- Not altering the visual content of `OIG4.png` (used as-is, scaled as needed).

## Decisions

### D1: Use `OIG4.png` as the canonical icon source

`webapp/scripts/generate-pwa-icons.js` will read `../public/OIG4.png` instead of `../public/favicon.svg`. Because the source is already a square raster (728×728), the script no longer needs the SVG-specific `SVG_W`/`SVG_H` constants; it will read the source dimensions via `sharp().metadata()` and scale the square image to each target size with `fit: "contain"` on a transparent canvas.

- Alternative considered: converting `OIG4.png` into an SVG. Rejected — a raster logo should not be re-encoded to SVG for a favicon; serving the PNG directly is simpler and lossless.

### D2: Keep existing PWA icon filenames; add a generated `favicon.png`

The `icons` array in `generate-pwa-icons.js` stays unchanged (same output filenames → `manifest.json` entries remain valid), and a `["favicon.png", 64, false]` entry is added so the favicon is generated from the same source with the same tooling.

### D3: Reference the branded favicon from `index.html`

Change the favicon link in `webapp/index.html` from `/favicon.svg` (SVG) to `/favicon.png` with `type="image/png"`. This is the only place the favicon is wired into the document head.

### D4: Update the service worker precache list

`webapp/public/sw.js` currently precaches `/favicon.svg`. Replace it with `/favicon.png`. Keep `/manifest.json`, `/apple-touch-icon.png`, `/icon-192.png`, `/icon-512.png`.

### D5: Remove the now-unused `favicon.svg`

After the switch, `webapp/public/favicon.svg` is referenced nowhere (index.html and sw.js are updated, and the icon generator no longer reads it). Delete the file to avoid shipping a stale generic glyph.

## Risks / Trade-offs

- [Small sizes may lose detail] → The 72px icon is the smallest generated; `sharp`'s high-quality resize plus `fit: contain` on a transparent canvas keeps the mark centered and non-clipped. Accepted trade-off of a raster source.
- [Maskable clipping] → The existing script already reserves a ~75% content safe zone for maskable icons; this behavior is preserved, so the brand mark stays inside the platform-safe circle.
- [Stale caches after deployment] → The service worker precache list is updated in the same change; regenerating icons with unchanged filenames means installed PWAs update on next fetch. No version-bump requirement since filenames are stable.
- [Removing `favicon.svg` could break an out-of-tree reference] → A repo-wide search shows references only in `index.html` and `sw.js`, both updated here.

## Migration Plan

1. Update `generate-pwa-icons.js` to source `OIG4.png` and emit `favicon.png`.
2. Run `node scripts/generate-pwa-icons.js` from `webapp/` to regenerate all PNGs.
3. Update `index.html` favicon link and `sw.js` precache list.
4. Delete `webapp/public/favicon.svg`.
5. Verify with `npm run lint && npx tsc -b && npm test`.
