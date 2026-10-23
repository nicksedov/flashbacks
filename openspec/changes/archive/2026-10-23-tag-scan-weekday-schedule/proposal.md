# Proposal

## Why

The automatic image tagging job currently runs on every day of the week inside a
time window, while the gallery sync schedule already lets an administrator choose
the weekdays it runs on. The two schedule editors are also implemented separately
with different controls, so the tagging card looks and behaves unlike the sync
card and the two drift apart over time.

## What Changes

- Add a weekday set to the automatic image tagging schedule: persisted with the
  tag scan settings, returned by the LLM settings API, and used by the tag scan
  manager to decide whether its scan window is open.
- Make the tagging status and the next-window calculation honour the weekday
  set, so an unselected weekday never opens the scan window and the manager waits
  for the next selected weekday.
- Extract the weekday/time controls of the sync schedule into one reusable
  `ScheduleEditor` widget and render both schedule sections (sync and tagging)
  through it, so the two settings cards look and behave the same.
- Add the weekday labels the shared widget needs to both locales (en, ru).
- Update the core OpenAPI contract for the new tag scan fields and regenerate the
  webapp TypeScript types.

## Capabilities

### New Capabilities

- `tag-scan-schedule`: weekday-based scheduling of the automatic image tagging
  job — persisting the weekday set, gating the scan window by weekday, choosing
  the next window opening, and the tagging schedule controls in the Analysis tab.

### Modified Capabilities

- `design-system`: adds a requirement that both schedule sections render their
  weekday and time controls through a single shared schedule editor.

## Non-goals

- Changing the tagging window semantics: tagging keeps its start–end window and
  the sync schedule keeps its single daily run time.
- Changing what tagging does, its pause/resume controls, or its progress display.
- Changing sync schedule behaviour, persistence, or wording beyond reusing the
  shared widget.

## Impact

- Backend (core): `domain.TagScanSettings`, `dto.TagScanSettingsDTO`,
  `dto.UpdateLlmSettingsRequest`, `handleUpdateLlmSettings`, the settings loader
  defaults, the DB seed and `TagScanManager` (start/update/gating helpers).
  The new column is added by GORM auto-migration.
- API: `docs/api-contracts/core.yaml` gains `TagScanSettingsDTO.days` and
  `UpdateLlmSettingsRequest.tagScanDays`; `make generate-types` refreshes
  `webapp/src/types/api.ts`.
- Webapp: new shared `components/ui/schedule-editor.tsx`; `AdminGeneralTab` and
  `AdminAnalysisTab` consume it; `translations.en.ts` + `translations.ru.ts` gain
  the shared weekday labels.
