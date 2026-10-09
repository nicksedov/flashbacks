# Proposal

## Why

The Administration → Settings screen groups its configuration into `Card`
containers, but many cards contain additional bordered boxes inside them: the
sync status, trash status, and thumbnail-cache status panels in the General tab,
the OCR/EXIF status rows and embedding/tag-scan panels in the Analysis tab, and
the model list and "new provider" form in the LLM Providers tab. The result is
frames within frames, which clutters the visualization and makes the settings
hierarchy hard to scan. The design system spec currently has no rule limiting
how deeply bordered surfaces may nest.

## What Changes

- Add a design-system rule: grouping surfaces SHALL be limited to one level of
  bordered container (the shared `Card`); a nested sub-area inside a card SHALL
  use a borderless muted inset fill (`bg-muted` + shared radius) or a labeled
  section, never a nested `border`.
- Refactor the Admin Settings tabs (General, Analysis, LLM Providers), the user
  Settings tab (audit only — it already has a single level), and the settings
  forms (`ProviderConfigForm`) to replace nested bordered panels with
  borderless inset fills.
- Record the fix in the design-system reconciliation backlog
  ([`docs/design-system.md`](../../docs/design-system.md)).

## Capabilities

### New Capabilities

<!-- none -->

### Modified Capabilities

- `design-system`: adds a requirement limiting bordered-surface nesting and
  specifying the borderless inset-fill alternative for nested sub-areas, and
  extends the "Consistent settings manipulation pattern" requirement to forbid
  nested borders within a card.

## Non-goals

- No full visual redesign: the shadcn/ui-based primitives and the 9-theme token
  set are unchanged.
- No change to interactive form controls (`Input`, `Select`, `Checkbox`), badges,
  alert/error banners, or floating dialog/popover surfaces — their borders are
  defined by their own primitive contracts.
- No new i18n strings: the change is `className`-only and changes no user-facing
  text.
- No backend, API contract, OpenAPI, MCP, or database changes.

## Impact

- webapp:
  [`AdminGeneralTab.tsx`](../../webapp/src/components/tabs/AdminGeneralTab.tsx),
  [`AdminAnalysisTab.tsx`](../../webapp/src/components/tabs/AdminAnalysisTab.tsx),
  [`AdminLlmProvidersTab.tsx`](../../webapp/src/components/tabs/AdminLlmProvidersTab.tsx),
  [`SettingsTab.tsx`](../../webapp/src/components/tabs/SettingsTab.tsx) (audit),
  [`ProviderConfigForm.tsx`](../../webapp/src/components/settings/ProviderConfigForm.tsx).
- Design system: the master spec
  [`openspec/specs/design-system/spec.md`](../../openspec/specs/design-system/spec.md)
  gains the new requirement when the change is applied.
- Documentation: the reconciliation backlog
  [`docs/design-system.md`](../../docs/design-system.md) gains a row and note.
- No backend, API contract, OpenAPI, MCP, or database changes.
