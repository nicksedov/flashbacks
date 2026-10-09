# Design

## Context

See [`proposal.md`](proposal.md) for motivation.

The admin settings screen
([`AdminSettingsTab.tsx`](../../webapp/src/components/tabs/AdminSettingsTab.tsx:17))
delegates to three tabs — General, Analysis, LLM Providers — each of which
stacks several `Card` containers. Inside many of those cards, read-only status
panels and nested configuration groups are wrapped in additional
`rounded-* border` boxes, producing frames within frames:

| File | Line | Current classes | Nested inside |
|---|---|---|---|
| [`AdminGeneralTab.tsx`](../../webapp/src/components/tabs/AdminGeneralTab.tsx:492) | 492 | `rounded-md border p-3 space-y-2` | Sync Schedule card |
| [`AdminGeneralTab.tsx`](../../webapp/src/components/tabs/AdminGeneralTab.tsx:593) | 593 | `flex items-center justify-between rounded-md border p-3` | Trash card |
| [`AdminGeneralTab.tsx`](../../webapp/src/components/tabs/AdminGeneralTab.tsx:666) | 666 | `flex items-center justify-between rounded-lg border p-3` | Thumbnail Cache card |
| [`AdminAnalysisTab.tsx`](../../webapp/src/components/tabs/AdminAnalysisTab.tsx:403) | 403 | `flex items-center justify-between rounded-lg border p-3` | OCR status card |
| [`AdminAnalysisTab.tsx`](../../webapp/src/components/tabs/AdminAnalysisTab.tsx:454) | 454 | `flex items-center justify-between rounded-lg border p-3` | EXIF status card |
| [`AdminAnalysisTab.tsx`](../../webapp/src/components/tabs/AdminAnalysisTab.tsx:496) | 496 | `space-y-3 rounded-lg border p-4` | Embedding card |
| [`AdminAnalysisTab.tsx`](../../webapp/src/components/tabs/AdminAnalysisTab.tsx:570) | 570 | `flex items-center space-x-2 rounded-lg border p-3` | Tag scan card |
| [`AdminAnalysisTab.tsx`](../../webapp/src/components/tabs/AdminAnalysisTab.tsx:612) | 612 | `flex items-center gap-4 rounded-lg border p-3` | Tag scan card |
| [`AdminLlmProvidersTab.tsx`](../../webapp/src/components/tabs/AdminLlmProvidersTab.tsx:625) | 625 | `mt-2 space-y-0.5 max-h-64 overflow-y-auto border rounded-md` | Provider card |
| [`AdminLlmProvidersTab.tsx`](../../webapp/src/components/tabs/AdminLlmProvidersTab.tsx:673) | 673 | `space-y-3 rounded-lg border p-4` | Provider card |
| [`ProviderConfigForm.tsx`](../../webapp/src/components/settings/ProviderConfigForm.tsx:56) | 56 | `space-y-4 rounded-lg border p-4` | provider form wrapper |
| [`ProviderConfigForm.tsx`](../../webapp/src/components/settings/ProviderConfigForm.tsx:194) | 194 | `max-h-40 overflow-y-auto border rounded-md p-2 text-xs text-muted-foreground space-y-1` | provider form |

[`SettingsTab.tsx`](../../webapp/src/components/tabs/SettingsTab.tsx:47) uses a
single `Card` with no nested borders and needs no change.

## Goals / Non-Goals

**Goals:**

- Define a single-level-bordered-container rule in the design system.
- Replace nested bordered panels in the Admin Settings tabs and settings forms
  with borderless inset fills.
- Record the drift in the reconciliation backlog.

**Non-Goals:**

- No change to form controls (`Input`, `Select`, `Checkbox`), badges,
  alert/error banners, or floating dialog/popover surfaces.
- No new component or CSS: reuse the `Card`, `Separator`, and `Label`
  primitives and the existing `bg-muted` token.
- No i18n changes (className-only) and no backend/API/DB changes.
- The `SyncHistoryDialog` history table keeps its border — it is a data-table
  grid, not a grouping frame.

## Decisions

### D1 — One level of bordered container

The `Card` is the only bordered grouping surface. No grouping element nested
inside a `Card` may carry a `border` utility.

### D2 — Inset fill is `bg-muted` + shared radius, no border

Nested sub-areas use `rounded-lg bg-muted` with their existing padding and drop
the `border` class. `bg-muted` resolves via tokens in all themes; a sub-area
needing stronger emphasis MAY use `bg-accent` or `bg-secondary` instead (still
borderless). Adjacent groups MAY use the `Separator` primitive.

### D3 — Scope

Apply to the files in the inventory above. `SettingsTab.tsx` is audited only
(it already has a single level). Alert/error banners (for example,
[`error-banner.tsx`](../../webapp/src/components/ui/error-banner.tsx:9) and the
SmartSearchTab warning banners) and data-table grids (for example,
[`SyncHistoryDialog.tsx`](../../webapp/src/components/settings/SyncHistoryDialog.tsx:148))
are out of scope.

### D4 — No new primitives or CSS

Reuse the existing `Card`, `Separator`, and `Label` primitives and token
utilities; no changes to [`globals.css`](../../webapp/src/styles/globals.css) or
[`ui/*`](../../webapp/src/components/ui/).

### D5 — No i18n changes

Only `className` values change; no new `TranslationKey` and no en/ru edits.

### D6 — Reconciliation backlog

Add a row to [`docs/design-system.md`](../../docs/design-system.md:19) for this
drift, mark it done, and add a Notes entry dated 2026-10-09.

## Risks / Trade-offs

- **[Inset fill has lower contrast than a border]** `bg-muted` on `background`
  may be subtle. → Mitigation: shared radius and padding preserve grouping;
  verify legibility in light and dark themes in task 6.3.
- **[A nested panel could be mistaken for a control or empty state]** →
  Mitigation: keep the existing label/icon/header inside the fill so the group
  reads as a section, not an input.
- **[Scope creep into unrelated borders]** form controls and alert banners
  remain bordered by design. → Mitigation: the spec rule explicitly excludes
  them; tasks touch only the inventory files.
- **[A missed nested border]** a class-level sweep could miss one. →
  Mitigation: task 6.3 performs a visual sweep of every admin tab.

## Migration Plan

- Frontend-only. Rebuild and redeploy the webapp; no backend, database, or
  contract changes.
- Rollback is a revert of the affected `className` changes.

## Open Questions

None. The inset-fill approach was confirmed with the user: borderless muted
fill across the Admin Settings tabs plus the user Settings tab and settings
forms.
