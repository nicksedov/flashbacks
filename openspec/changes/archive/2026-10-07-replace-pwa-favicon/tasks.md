## 1. Icon generation script

- [x] 1.1 Update `webapp/scripts/generate-pwa-icons.js` to read `../public/OIG4.png` as the source instead of `../public/favicon.svg`, deriving dimensions from the PNG metadata, and verify the script references no remaining `favicon.svg` path
- [x] 1.2 Add a `["favicon.png", 64, false]` entry to the icon definitions in `webapp/scripts/generate-pwa-icons.js` and verify the entry is present before the output loop

## 2. Regenerate assets

- [x] 2.1 Run `node scripts/generate-pwa-icons.js` from `webapp/` and verify it completes successfully and prints an entry for every icon including `favicon.png`
- [x] 2.2 Verify the regenerated `webapp/public/icon-*.png`, `webapp/public/apple-touch-icon.png`, `webapp/public/maskable-*.png`, and `webapp/public/favicon.png` exist and are valid PNG files

## 3. Asset wiring

- [x] 3.1 Update the favicon `<link>` in `webapp/index.html` from `/favicon.svg` to `/favicon.png` with `type="image/png"` and verify the file no longer references `/favicon.svg`
- [x] 3.2 Update the precache list in `webapp/public/sw.js` to replace `/favicon.svg` with `/favicon.png` and verify no `favicon.svg` reference remains
- [x] 3.3 Delete `webapp/public/favicon.svg` and verify it is no longer referenced anywhere in the repo (search for `favicon.svg`)

## 4. Verification

- [x] 4.1 Run `npm run lint && npx tsc -b` from `webapp/` and verify both complete with no errors
- [x] 4.2 Run `npm test` from `webapp/` and verify all tests pass
