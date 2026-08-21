## 1. Design system authority and documentation

- [ ] 1.1 Verify the delta spec at `openspec/changes/design-system/specs/design-system/spec.md` covers all requirements from the proposal (tokens, primitives, layout, interaction, responsive, theming, accessibility, anti-patterns).
- [ ] 1.2 Add a pointer to the design system in the root `AGENTS.md` ("Design System" section referencing `openspec/specs/design-system/spec.md`) so agents consult it before implementing UI.
- [ ] 1.3 Add the same pointer to `webapp/AGENTS.md`.
- [ ] 1.4 Create `docs/design-system.md` containing the reconciliation backlog table (copy from `design.md` Decisions section) as the living audit trail of drift vs the spec.

## 2. Token normalization

- [ ] 2.1 In `webapp/src/styles/globals.css`, define `--color-foreground` for every theme (all light/dark accent variants), derived from each theme's existing `-card-foreground` value.
- [ ] 2.2 Remove the stray unused `--color-light-green-foreground` typo from `globals.css`.
- [ ] 2.3 In `webapp/src/styles/globals.css`, ensure `body` uses `var(--color-foreground)` for base text color (keeping existing contrast behavior).
- [ ] 2.4 Run `npm run lint && npx tsc -b` in `webapp/` and manually verify all 9 themes render legible text.

## 3. Primitive consolidation

- [ ] 3.1 Merge `webapp/src/components/ui/tabs.tsx` and `webapp/src/components/ui/underline-tabs.tsx` into a single `Tabs` primitive exposing `variant: "underline" | "segmented"`; update the existing `TabsContent` usage in `App.tsx` if needed.
- [ ] 3.2 Delete `webapp/src/components/ui/theme-select.tsx`; port its popover background style into `webapp/src/components/ui/select.tsx`; update `webapp/src/components/tabs/SettingsTab.tsx` to use `Select`.
- [ ] 3.3 Extract the theme picker options in `SettingsTab.tsx` into a single exported `THEMES` array (id, label key, icon, swatch) instead of hardcoded JSX.
- [ ] 3.4 Add an icon-slot variant to `webapp/src/components/ui/input.tsx` (leading/trailing icon support).
- [ ] 3.5 Extend `webapp/src/components/EmptyState.tsx` with a `size` prop and a `hint` translation key so gallery, folders, and users empty states can share it.
- [ ] 3.6 Run `npm run lint && npx tsc -b` in `webapp/`; run `npm test` (Vitest).

## 4. Replace raw elements with primitives

- [ ] 4.1 In `webapp/src/components/layout/Header.tsx`, replace the raw mobile-menu and profile `<button>` elements with `Button variant="ghost"` / `IconButton`.
- [ ] 4.2 In `webapp/src/components/layout/Sidebar.tsx`, replace the raw close `<button>` with an `IconButton`.
- [ ] 4.3 In `webapp/src/components/gallery/GalleryFoldersView.tsx`, replace the raw home/breadcrumb `<button>` elements with a `Button variant="ghost"` based breadcrumb row (or a dedicated `Breadcrumbs` component).
- [ ] 4.4 In `webapp/src/components/gallery/GalleryAllImagesView.tsx`, replace the raw search `<input>` with the `Input` icon-slot variant and the raw sort/clear buttons with `Button variant="ghost"`.
- [ ] 4.5 In `webapp/src/components/settings/FolderList.tsx`, `webapp/src/components/gallery/GalleryAllImagesView.tsx`, and `webapp/src/components/auth/admin/UserList.tsx`, replace bespoke empty-state markup with the shared `EmptyState` primitive.
- [ ] 4.6 Run `npm run lint && npx tsc -b` in `webapp/` after each component swap.

## 5. Unify interaction patterns

- [ ] 5.1 Move folder creation to a top-right primary "+ Add" action in the settings Folders card (convert `AddFolderForm` into a small `Dialog` launched from that button, or place it in the card header action area); remove the inline top-of-list add form.
- [ ] 5.2 In `webapp/src/components/tabs/GalleryTab.tsx`, replace `alert(...)` error handling with `sonner` toasts and replace the permanent-delete `window.confirm(...)` with a `Dialog`.
- [ ] 5.3 In `webapp/src/components/auth/admin/UserList.tsx`, replace the `confirm(...)` delete prompt with a `Dialog` confirmation.
- [ ] 5.4 Ensure the `Dialog` surface in `webapp/src/components/ui/dialog.tsx` uses `bg-popover`/`text-popover-foreground` (or `bg-card`) tokens instead of `bg-white/90`/`bg-black/90`.
- [ ] 5.5 Standardize settings screens (preferences, LLM providers, analysis) on cards with `Label` above each control and a single Save button disabled when unchanged.
- [ ] 5.6 Replace hardcoded English strings in `webapp/src/components/settings/ProviderConfigForm.tsx` with `t()` keys; add the new keys to `webapp/src/i18n/translations.en.ts` AND `webapp/src/i18n/translations.ru.ts`.
- [ ] 5.7 Run `npm run lint && npx tsc -b` in `webapp/`; run `npm test` (Vitest, includes the i18n en/ru parity test).

## 6. Layout policy and final verification

- [ ] 6.1 In `webapp/src/App.tsx`, apply a single horizontal padding + max-width policy to all tab content (remove the geolocation `px-3 py-3` special case or formalize it as a layout constant).
- [ ] 6.2 Run `npm run lint && npx tsc -b` in `webapp/` and manually verify each tab at desktop and mobile widths across all 9 themes.
- [ ] 6.3 Update the reconciliation backlog in `docs/design-system.md` to mark completed items and note any remaining drift for follow-up changes.
