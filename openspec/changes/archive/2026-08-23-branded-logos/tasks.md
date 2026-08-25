## 1. Auth screen logo

- [x] 1.1 In `webapp/src/components/auth/LoginScreen.tsx`, replace the `ShieldAlert` icon circle and the `CardTitle` heading in `CardHeader` with a centered `<img>` rendering `/flashbacks_logo_welcomescreen.png` (alt text "Flashbacks"), sized `w-72 max-w-full h-auto mx-auto` with `object-contain` (50% upscale of `w-48`); keep the mode-specific `CardDescription` below the logo
- [x] 1.2 Remove the now-unused `CardTitle` import from `LoginScreen.tsx` (keep `ShieldAlert` — it is still used by the registration approval warning) to satisfy `noUnusedLocals`
- [x] 1.3 Verify the logo renders proportionally (no distortion/clipping) in login, registration, and bootstrap modes on desktop and mobile widths

## 2. Sidebar header logo

- [x] 2.1 In `webapp/src/components/layout/Sidebar.tsx`, replace the expanded-state glyph box + `h1` heading with an `<img>` rendering `/flashbacks_logo_base.png` (alt text "Flashbacks"), sized `h-16 w-auto` inside the existing `h-16` header so the header height does not change (100% upscale of `h-8`)
- [x] 2.2 Keep the collapsed-state compact glyph mark unchanged so the narrow rail stays compact
- [x] 2.3 Verify the expanded sidebar shows the wordmark logo centered vertically and legible, and that the collapsed sidebar still shows the icon mark (note: low contrast in dark themes is an accepted trade-off — see design.md)

## 3. Validation

- [x] 3.1 Run `npm run lint && npx tsc -b` in `webapp/` and fix any errors (no unused imports/vars)
- [x] 3.2 Run `npm test` (Vitest) in `webapp/` and confirm no regressions
- [x] 3.3 Manually verify the auth screen and sidebar logo placement/sizing against the `branding` spec scenarios (auth shows welcome logo, sidebar shows base logo, collapsed keeps compact mark, alt text present)
