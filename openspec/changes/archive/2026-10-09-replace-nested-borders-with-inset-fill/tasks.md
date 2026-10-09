# Tasks

## 1. Refactor Admin General tab to inset fills

- [x] 1.1 In
  [`AdminGeneralTab.tsx`](../../webapp/src/components/tabs/AdminGeneralTab.tsx:492),
  change the Sync Status panel class from
  `rounded-md border p-3 space-y-2` to `rounded-lg bg-muted p-3 space-y-2`
  (borderless inset fill). Run `npm run lint && npx tsc -b`.
- [x] 1.2 In
  [`AdminGeneralTab.tsx`](../../webapp/src/components/tabs/AdminGeneralTab.tsx:593),
  change the Trash status panel class from
  `flex items-center justify-between rounded-md border p-3` to
  `flex items-center justify-between rounded-lg bg-muted p-3`. Run
  `npm run lint && npx tsc -b`.
- [x] 1.3 In
  [`AdminGeneralTab.tsx`](../../webapp/src/components/tabs/AdminGeneralTab.tsx:666),
  change the Thumbnail Cache status panel class from
  `flex items-center justify-between rounded-lg border p-3` to
  `flex items-center justify-between rounded-lg bg-muted p-3`. Run
  `npm run lint && npx tsc -b`.

## 2. Refactor Admin Analysis tab to inset fills

- [x] 2.1 In
  [`AdminAnalysisTab.tsx`](../../webapp/src/components/tabs/AdminAnalysisTab.tsx:403),
  change the OCR status row class from
  `flex items-center justify-between rounded-lg border p-3` to
  `flex items-center justify-between rounded-lg bg-muted p-3`. Run
  `npm run lint && npx tsc -b`.
- [x] 2.2 In
  [`AdminAnalysisTab.tsx`](../../webapp/src/components/tabs/AdminAnalysisTab.tsx:454),
  change the EXIF status row class from
  `flex items-center justify-between rounded-lg border p-3` to
  `flex items-center justify-between rounded-lg bg-muted p-3`. Run
  `npm run lint && npx tsc -b`.
- [x] 2.3 In
  [`AdminAnalysisTab.tsx`](../../webapp/src/components/tabs/AdminAnalysisTab.tsx:496),
  change the embedding provider/model panel class from
  `space-y-3 rounded-lg border p-4` to `space-y-3 rounded-lg bg-muted p-4`.
  Run `npm run lint && npx tsc -b`.
- [x] 2.4 In
  [`AdminAnalysisTab.tsx`](../../webapp/src/components/tabs/AdminAnalysisTab.tsx:570),
  change the tag-scan enabled row class from
  `flex items-center space-x-2 rounded-lg border p-3` to
  `flex items-center space-x-2 rounded-lg bg-muted p-3`. Run
  `npm run lint && npx tsc -b`.
- [x] 2.5 In
  [`AdminAnalysisTab.tsx`](../../webapp/src/components/tabs/AdminAnalysisTab.tsx:612),
  change the tag-scan status row class from
  `flex items-center gap-4 rounded-lg border p-3` to
  `flex items-center gap-4 rounded-lg bg-muted p-3`. Run
  `npm run lint && npx tsc -b`.

## 3. Refactor Admin LLM Providers tab to inset fills

- [x] 3.1 In
  [`AdminLlmProvidersTab.tsx`](../../webapp/src/components/tabs/AdminLlmProvidersTab.tsx:625),
  change the model list class from
  `mt-2 space-y-0.5 max-h-64 overflow-y-auto border rounded-md` to
  `mt-2 space-y-0.5 max-h-64 overflow-y-auto bg-muted rounded-md`. Run
  `npm run lint && npx tsc -b`.
- [x] 3.2 In
  [`AdminLlmProvidersTab.tsx`](../../webapp/src/components/tabs/AdminLlmProvidersTab.tsx:673),
  change the new-provider form class from `space-y-3 rounded-lg border p-4` to
  `space-y-3 rounded-lg bg-muted p-4`. Run `npm run lint && npx tsc -b`.

## 4. Refactor settings forms and audit the user Settings tab

- [x] 4.1 In
  [`ProviderConfigForm.tsx`](../../webapp/src/components/settings/ProviderConfigForm.tsx:56),
  change the form wrapper class from `space-y-4 rounded-lg border p-4` to
  `space-y-4 rounded-lg bg-muted p-4`. Run `npm run lint && npx tsc -b`.
- [x] 4.2 In
  [`ProviderConfigForm.tsx`](../../webapp/src/components/settings/ProviderConfigForm.tsx:194),
  change the available-models list class from
  `max-h-40 overflow-y-auto border rounded-md p-2 text-xs text-muted-foreground space-y-1`
  to
  `max-h-40 overflow-y-auto bg-muted rounded-md p-2 text-xs text-muted-foreground space-y-1`.
  Run `npm run lint && npx tsc -b`.
- [x] 4.3 Audit [`SettingsTab.tsx`](../../webapp/src/components/tabs/SettingsTab.tsx:47):
  confirm it renders a single `Card` with no nested borders and therefore needs
  no change; record the result in the change notes.

## 5. Update the design-system reconciliation backlog

- [x] 5.1 Add a row to
  [`docs/design-system.md`](../../docs/design-system.md:19) for the
  nested-borders drift (target: borderless `bg-muted` inset fill), mark it done,
  and add a Notes entry dated 2026-10-09.

## 6. Verification

- [x] 6.1 Run `cd webapp && npm run lint && npx tsc -b` (must pass with no
  errors).
- [x] 6.2 Run `cd webapp && npm test` (Vitest, including the i18n
  `translations.test.ts`).
- [x] 6.3 Manually open Administration → Settings (General, Analysis, LLM
  Providers) and the user Settings tab in both English and Russian; verify no
  `border` appears inside any `Card`, the inset fills are legible in light and
  dark themes, and all controls, labels, and save affordances are unchanged.
