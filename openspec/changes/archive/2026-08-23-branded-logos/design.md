## Context

The webapp already ships two branded logo assets in [`webapp/public/`](webapp/public) — `flashbacks_logo_welcomescreen.png` (1248×800, 1.56:1) and `flashbacks_logo_base.png` (1040×500, 2.08:1) — but the UI still shows placeholder marks: a `ShieldAlert` icon + title on the auth card ([`LoginScreen.tsx`](webapp/src/components/auth/LoginScreen.tsx:162)) and an inline SVG glyph + "Flashbacks" heading in the sidebar header ([`Sidebar.tsx`](webapp/src/components/layout/Sidebar.tsx:184)). Motivation is in [`proposal.md`](openspec/changes/branded-logos/proposal.md); the observable requirements are in [`specs/branding/spec.md`](openspec/changes/branded-logos/specs/branding/spec.md). This is a presentational, webapp-only change with no backend or API impact.

## Goals / Non-Goals

**Goals:**
- Render the welcome-screen logo at the top of the auth card in place of the icon + title block.
- Render the base logo at the top of the expanded sidebar in place of the glyph + heading.
- Keep the collapsed sidebar visually compact by retaining a small centered mark.
- Preserve aspect ratio and avoid layout bloat on both surfaces.

**Non-Goals:**
- No modification of the PNG assets.
- No design-token, color, font, or spacing changes (see the design-system spec — tokens are untouched).
- No change to auth logic, forms, or the `Header` component.
- No new i18n keys (the logos carry the brand name; `alt` text is static English "Flashbacks" which is a proper noun, identical in both locales).

## Decisions

### D1: Render logos as `<img>` with public asset URLs

Both assets already live in `webapp/public/`, so Vite serves them at the site root (`/flashbacks_logo_welcomescreen.png`, `/flashbacks_logo_base.png`). The components reference them directly with `<img src="/flashbacks_logo_welcomescreen.png" ...>` rather than importing through the bundler or duplicating the assets.

- Rationale: zero new tooling; the files are already static public assets used by the PWA setup; matches how other public assets (avatars) are referenced.
- Alternatives considered: (a) importing via `import logo from "@/assets/..."` — would require moving/copying assets; (b) inlining SVG — not possible since the assets are bitmaps.

### D2: Auth card — logo replaces icon circle + title, description stays

In [`LoginScreen.tsx`](webapp/src/components/auth/LoginScreen.tsx:162) the `CardHeader` currently contains a `h-12 w-12` icon circle and a `CardTitle`. Replace the icon circle and the title with a centered `<img>` of the welcome-screen logo, keeping the `CardDescription` below it (the description is mode-specific and remains meaningful for all three modes).

Sizing: the card is `max-w-md` (448px) with header padding. The welcome logo is 1.56:1, so cap width at `w-72` (288px) → ≈185px tall (a 50% upscale of the original `w-48`). Use `mx-auto h-auto` and `object-contain` so aspect ratio is always preserved on narrower viewports (`max-w-full`).

- Rationale: 1.56:1 at 288px wide yields a prominent brand lockup that still fits the card content width; on narrow phones `max-w-full` scales it down so it never overflows.
- Alternative: rendering the full logo full-card-width — rejected, it dominates the card and pushes the form below the fold on small screens.

### D3: Sidebar — base logo replaces glyph + heading in expanded state only

In [`Sidebar.tsx`](webapp/src/components/layout/Sidebar.tsx:184) the `h-16` header renders the glyph box (expanded) plus the `h1` title. Replace the glyph + title with a single `<img>` of the base logo constrained to the header height. Because the logo is 2.08:1, cap height at `h-16` (64px) → width ≈133px, vertically centered in the 64px header with the existing `px-4` padding. Use `w-auto h-16` so width derives from the aspect ratio and the header does not grow (this is a 100% upscale of the original `h-8`).

For the collapsed rail, the full wordmark does not fit, so keep the existing compact glyph mark (the current collapsed branch) unchanged — this satisfies the spec's collapsed-state scenario without new assets.

- Rationale: 64px fills the existing `h-16` header height with the wordmark legible and prominent; the header height itself is unchanged.
- Alternatives considered: (a) shrinking the full logo to fit the collapsed rail — rejected, the wordmark is unreadable at ~32px width; (b) a cropped variant asset — out of scope, no asset changes.

### D4: Accessibility

Both `<img>` elements get `alt="Flashbacks"` (a proper noun, so no translation needed). The sidebar `h1` heading is removed since the logo conveys the product name; the logo remains the visual/accessible identity of the app shell.

- Rationale: no unlabeled images; no duplicate adjacent brand text; keeps en/ru in sync with zero i18n changes.

## Risks / Trade-offs

- [Logo legibility at 64px sidebar height] → The base logo is a 1040×500 wordmark; at ~133×64px it is large and legible even on high-DPI displays, relying on the PNG's intrinsic resolution (retina-friendly). No further bump anticipated; `w-auto h-16` keeps the header height unchanged.
- [Dark-theme contrast: base logo is dark-on-transparent] → Measured average luminance of `flashbacks_logo_base.png` is ~87 (dark) with transparency; on dark-theme sidebars (`--color-sidebar` ≈ RGB 41) contrast is only ≈2.1:1, so the wordmark is faint in dark themes. **Decision (accepted):** keep the logo as-is in all themes and accept the low contrast for now; no CSS filter, no asset swap. Revisit with a light logo variant in a future change if needed.
- [Removing the sidebar `h1` could regress layout or a11y landmarking] → The header row is flex-centered and the `h1` is removed as part of the same block replacement; the implementation step verifies vertical centering and screen-reader label.
- [Auth logo size could still feel large on narrow phones] → `mx-auto h-auto` + `max-w-full` guarantees the image scales down with the card; no fixed height is imposed.

## Migration Plan

No data migration. Deploy is a normal webapp rebuild (`npm run build` / docker build). Rollback is a one-line revert of the two component files; assets are unchanged so there is no cache-poisoning risk from stale images.

## Open Questions

None — sizing fallbacks (D4) are resolved at implementation time by visual check without changing the spec or task breakdown.
