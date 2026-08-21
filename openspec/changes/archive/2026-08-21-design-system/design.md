## Context

The webapp is a React 19 + TypeScript SPA using Tailwind CSS 4, Radix UI primitives (dialog, select, tabs, label, slot), `class-variance-authority`, `lucide-react` icons, and `sonner` toasts. UI primitives live in [`src/components/ui/`](../../webapp/src/components/ui/), theme tokens in [`src/styles/globals.css`](../../webapp/src/styles/globals.css), and page/section composition in [`src/components/{layout,tabs,gallery,settings,auth}`](../../webapp/src/components/). The codebase already follows the shadcn/ui structure but has drifted: see [`proposal.md`](./proposal.md) for the motivation and [`specs/design-system/spec.md`](./specs/design-system/spec.md) for the normative requirements.

The concrete drift found during analysis, and its target resolution, is the reconciliation backlog this design drives:

| # | Current state | Location | Target |
|---|---|---|---|
| 1 | Two near-identical Select implementations | [`select.tsx`](../../webapp/src/components/ui/select.tsx) + [`theme-select.tsx`](../../webapp/src/components/ui/theme-select.tsx), used side-by-side in [`SettingsTab.tsx`](../../webapp/src/components/tabs/SettingsTab.tsx) | Keep `select.tsx`; delete `theme-select.tsx` |
| 2 | Two tab styles | [`tabs.tsx`](../../webapp/src/components/ui/tabs.tsx) (segmented) + [`underline-tabs.tsx`](../../webapp/src/components/ui/underline-tabs.tsx) (underline) | One `Tabs` primitive with `variant` |
| 3 | Raw `<button>` with ad-hoc classes | [`Header.tsx`](../../webapp/src/components/layout/Header.tsx), [`Sidebar.tsx`](../../webapp/src/components/layout/Sidebar.tsx), [`GalleryFoldersView.tsx`](../../webapp/src/components/gallery/GalleryFoldersView.tsx), [`GalleryAllImagesView.tsx`](../../webapp/src/components/gallery/GalleryAllImagesView.tsx) | `Button`/`IconButton` primitives; add a `Breadcrumbs` component for folder navigation |
| 4 | Raw `<input>` search field | [`GalleryAllImagesView.tsx`](../../webapp/src/components/gallery/GalleryAllImagesView.tsx) | `Input` primitive with an icon-slot variant |
| 5 | Three+ empty-state layouts (different icons/padding) | [`EmptyState.tsx`](../../webapp/src/components/EmptyState.tsx), [`FolderList.tsx`](../../webapp/src/components/settings/FolderList.tsx), [`GalleryAllImagesView.tsx`](../../webapp/src/components/gallery/GalleryAllImagesView.tsx), [`UserList.tsx`](../../webapp/src/components/auth/admin/UserList.tsx) | Single `EmptyState` primitive with size variants |
| 6 | "Add item" in different regions | Folders add-form inline at top ([`AddFolderForm.tsx`](../../webapp/src/components/settings/AddFolderForm.tsx)) vs top-right `Plus` dialog launchers (LLM providers, admin users) | Top-right primary "+ Add" button everywhere |
| 7 | Mixed feedback | `alert()`/`confirm()` in [`GalleryTab.tsx`](../../webapp/src/components/tabs/GalleryTab.tsx), [`UserList.tsx`](../../webapp/src/components/auth/admin/UserList.tsx) vs `sonner` toasts in settings | Toasts for results; `Dialog` for confirmations |
| 8 | Dialog hardcodes `bg-white/90` / `bg-black/90` | [`dialog.tsx`](../../webapp/src/components/ui/dialog.tsx) | Token-based `bg-popover`/`bg-card` |
| 9 | `--color-foreground` undefined | [`globals.css`](../../webapp/src/styles/globals.css) | Define for all 9 themes |
| 10 | Per-tab content gutter special case | [`App.tsx`](../../webapp/src/App.tsx) (`px-8 py-6` vs geolocation `px-3 py-3`) | Single gutter/max-width policy |
| 11 | Hardcoded English strings | [`ProviderConfigForm.tsx`](../../webapp/src/components/settings/ProviderConfigForm.tsx) | i18n keys, en + ru |

## Goals / Non-Goals

**Goals:**

- Produce an authoritative, Material-3-grounded design system spec that agents can apply mechanically when building new screens.
- Reconcile the token layer so every semantic role (including `foreground`) is defined in every theme.
- Collapse duplicate primitives and standardize "add", settings, feedback, empty/loading, and confirmation patterns.
- Keep the change behavior-neutral: the app must look no worse after each reconciliation step, and all existing tests/lint/type-checks must pass.

**Non-Goals:**

- No visual redesign, no new accent hues, no removal of existing themes.
- No new component library or new runtime dependencies.
- No backend, API, or data-model changes.
- Not fixing every drift item in one giant commit — the backlog is drained via small, separately reviewable follow-up changes.

## Decisions

### D1 — Material Design 3 as the conceptual model, shadcn/Radix as the implementation

The design system is grounded in Google Material Design 3's concepts — semantic color roles with `primary/secondary/surface/outline` and state layers, 4px spacing grid, 8dp shape scale, elevation, and visible focus rings — but implemented with the existing shadcn/ui-style primitives over Radix + Tailwind 4 tokens, not by adopting MUI.

*Alternatives considered:* adopting MUI or Chakra (larger migration, new API, conflicts with existing Tailwind styles) — rejected. Writing a fully custom token system (unnecessary; the CSS variable layer already exists) — rejected.

### D2 — Two-level token architecture, formalized

Tokens remain two-level: (1) per-theme raw values (`--color-<theme>-<role>`), and (2) semantic roles (`--color-<role>`) that the theme class repoints. The fix is to add the missing `--color-foreground` (and remove the stray unused `--color-light-green-foreground` typo) and to forbid components from referencing level-1 names. `--radius` stays the single radius token; shadows follow the `shadow-sm`/`shadow` convention already in the primitives.

*Alternatives considered:* collapsing to a single accent-hue scale generated from one `--theme-base-hue` (cleaner but a visual redesign; existing per-theme tuning like `dark-contrast` would be lost) — rejected to stay behavior-neutral.

### D3 — One `Tabs` primitive with `variant: "underline" | "segmented"`

Both current tab components are merged into a single `Tabs` primitive exposing a `variant` prop. Underline is the default for content tabs (MD3 primary tabs); segmented is for filter/segmented controls. This satisfies the spec's "single tabs primitive" without losing either presentation.

*Alternatives considered:* deleting one style outright (loses a legitimately useful presentation); keeping both components (perpetuates the duplicate) — both rejected.

### D4 — One `Select`, theme items become data

`theme-select.tsx` is deleted. Its only real difference from `select.tsx` (an inline `backgroundColor` style for popover theming) is folded into `select.tsx`. The theme picker options are generated from a single exported `THEMES` array (id, label key, icon, swatch) instead of hardcoded JSX.

### D5 — "Add" is a top-right primary action

Creation of a collection item is always a primary `Button` (default variant) with a `Plus` icon in the view's top-right action area or toolbar. The folders add-form is moved out of its inline position into the settings "Folders" card header/top-right (or converted to a small dialog launched from that button). This is the single answer to the user's "top-right vs bottom-of-list" example.

### D6 — Feedback and confirmation channels

`sonner` toasts are the only result-feedback channel. `alert()`/`confirm()` are removed: `GalleryTab` delete failures become toasts, and the permanent-delete `confirm()` becomes a `Dialog`. `UserList`'s `confirm()` becomes a `Dialog`. A `useConfirm`-style helper is not required yet — explicit `Dialog` composition matches the existing code.

### D7 — Canonical composition patterns (the rules agents follow)

Each of these is documented in the master spec with concrete class guidance:

- **View shell**: `Sidebar` + `Header` + scrolling `main` with one gutter/width policy.
- **View header**: `ViewHeader` (icon + count/title) on the left, view-level actions (search, sort, add) on the right.
- **Toolbar**: a `rounded-lg border bg-card p-3` strip of `IconButton`s for batch actions, with a trailing page-size `Select`.
- **List item**: a `Card` row (`flex items-center gap-3 p-3`) with a leading icon, a title + meta column, and trailing icon-only ghost/destructive actions.
- **Settings form**: fields in cards with `Label` above each control, one primary Save that is disabled when unchanged.
- **Empty/loading**: `EmptyState` / `Skeleton` everywhere.

### D8 — Spec home and agent access

The normative spec lives at `openspec/specs/design-system/spec.md` (created at apply from this delta). A one-line pointer is added to the root [`AGENTS.md`](../../AGENTS.md) and to [`webapp/AGENTS.md`](../../webapp/AGENTS.md) so every agent sees the design system before writing UI. The reconciliation backlog (table above) is carried into `docs/design-system.md` as the living audit trail.

## Risks / Trade-offs

- **Large surface area** → Mitigation: backlog drained in small, behavior-neutral changes, each verified with `npm run lint && npx tsc -b` and manual theme/breakpoint spot checks.
- **Removing `theme-select.tsx` regresses popover theming** → Mitigation: port the popover background style into `select.tsx` first; verify the theme picker still follows dark/light.
- **Adding `--color-foreground` changes default text color** → Mitigation: derive it from the existing `--color-*-card-foreground` value per theme, so text is unchanged in practice; test all 9 themes.
- **Standardizing "add" placement changes muscle memory** → Mitigation: it is a deliberate UX unification; both patterns currently coexist and one must win (top-right, per MD3 FAB-in-toolbar convention).
- **Replacing `confirm()` with `Dialog` is more code per destructive flow** → Mitigation: acceptable; correctness/accessibility outrank terseness, and a shared confirm dialog can be added later if duplication grows.

## Migration Plan

1. Token normalization: add `--color-foreground` to all themes; remove the stray `light-green-foreground` typo. No component changes yet.
2. Primitive consolidation: merge `Tabs`, delete `theme-select.tsx`, add `Input` icon-slot and `EmptyState` size variants. Keep old exports as one-release aliases if needed to avoid a big-bang.
3. Swap raw elements for primitives in layout/gallery/settings (backlog rows 3–5), one component per change.
4. Unify add/settings/feedback/confirmation patterns (backlog rows 6–8, 11).
5. Apply the single page-gutter policy in [`App.tsx`](../../webapp/src/App.tsx) (backlog row 10).
6. After each step: `npm run lint && npx tsc -b` in [`webapp/`](../../webapp/) and a visual pass across the 9 themes at desktop and mobile widths.
7. Rollback: each step is an independent revertable commit; no cross-service or data migration exists, so rollback is a plain `git revert`.

## Open Questions

<!-- none -->
