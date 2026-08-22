# Design System — Reconciliation Backlog

> Living audit trail of known UI drift vs the authoritative design system spec at [`openspec/specs/design-system/spec.md`](../openspec/specs/design-system/spec.md).
>
> Each row is a known inconsistency between the current UI and the spec, mapped to its target resolution. Rows are drained via small, separately reviewable changes. This table is the canonical inventory; update it whenever drift is fixed or new drift is discovered.

| # | Current state | Location | Target | Status |
|---|---|---|---|---|
| 1 | Two near-identical Select implementations | [`select.tsx`](../webapp/src/components/ui/select.tsx) + ~~`theme-select.tsx`~~, used side-by-side in [`SettingsTab.tsx`](../webapp/src/components/tabs/SettingsTab.tsx) | Keep `select.tsx`; delete `theme-select.tsx` | ☑ |
| 2 | Two tab styles | [`tabs.tsx`](../webapp/src/components/ui/tabs.tsx) (segmented) + ~~`underline-tabs.tsx`~~ (underline) | One `Tabs` primitive with `variant` | ☑ |
| 3 | Raw `<button>` with ad-hoc classes | [`Header.tsx`](../webapp/src/components/layout/Header.tsx), [`Sidebar.tsx`](../webapp/src/components/layout/Sidebar.tsx), [`GalleryFoldersView.tsx`](../webapp/src/components/gallery/GalleryFoldersView.tsx), [`GalleryAllImagesView.tsx`](../webapp/src/components/gallery/GalleryAllImagesView.tsx) | `Button`/`IconButton` primitives; add a `Breadcrumbs` component for folder navigation | ☑ |
| 4 | Raw `<input>` search field | [`GalleryAllImagesView.tsx`](../webapp/src/components/gallery/GalleryAllImagesView.tsx) | `Input` primitive with an icon-slot variant | ☑ |
| 5 | Three+ empty-state layouts (different icons/padding) | [`EmptyState.tsx`](../webapp/src/components/EmptyState.tsx), [`FolderList.tsx`](../webapp/src/components/settings/FolderList.tsx), [`GalleryAllImagesView.tsx`](../webapp/src/components/gallery/GalleryAllImagesView.tsx), [`UserList.tsx`](../webapp/src/components/auth/admin/UserList.tsx) | Single `EmptyState` primitive with size variants | ☑ |
| 6 | "Add item" in different regions | Folders add-form inline at top ([`AddFolderForm.tsx`](../webapp/src/components/settings/AddFolderForm.tsx)) vs top-right `Plus` dialog launchers (LLM providers, admin users) | Top-right primary "+ Add" button everywhere | ☑ |
| 7 | Mixed feedback | `alert()`/`confirm()` in [`GalleryTab.tsx`](../webapp/src/components/tabs/GalleryTab.tsx), [`UserList.tsx`](../webapp/src/components/auth/admin/UserList.tsx) vs `sonner` toasts in settings | Toasts for results; `Dialog` for confirmations | ☑ |
| 8 | Dialog hardcodes `bg-white/90` / `bg-black/90` | [`dialog.tsx`](../webapp/src/components/ui/dialog.tsx) | Token-based `bg-popover`/`bg-card` | ☑ |
| 9 | `--color-foreground` undefined | [`globals.css`](../webapp/src/styles/globals.css) | Define for all 9 themes | ☑ |
| 10 | Per-tab content gutter special case | [`App.tsx`](../webapp/src/App.tsx) (`px-8 py-6` vs geolocation `px-3 py-3`) | Single gutter/max-width policy | ☑ |
| 11 | Hardcoded English strings | [`ProviderConfigForm.tsx`](../webapp/src/components/settings/ProviderConfigForm.tsx) | i18n keys, en + ru | ☑ |

## Notes

- **Status legend:** ☐ = pending, ☑ = done (drift fixed), ✎ = intentionally deferred with a note.
- Keep this file in sync with the master spec. When fixing a row, mark it done here **and** update the corresponding tasks/change artifacts.
