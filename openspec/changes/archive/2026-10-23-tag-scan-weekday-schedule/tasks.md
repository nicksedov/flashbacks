# Tasks

## 1. Core persistence and API

- [x] 1.1 Add `Days` to `domain.TagScanSettings` (`gorm:"default:'1,2,3,4,5'"`, json `days`), to `dto.TagScanSettingsDTO`, to `dto.UpdateLlmSettingsRequest` (`tagScanDays`) and to `buildTagScanDTO`; verify with `go test ./internal/interfaces/handler/... -run TestUpdateLlmSettingsTagScanDaysPersisted -count=1` (scenario `tag-scan-schedule.days-persisted`)
- [x] 1.2 Persist `tagScanDays` in `handleUpdateLlmSettings` (dirty check, save, pass to `UpdateSchedule`) and pass the parsed days to the manager; verify the same handler test asserts both the stored row and the returned DTO
- [x] 1.3 Seed the default weekday set in `infrastructure/database/database.go` and the `SettingsLoader.TagScanSettings` fallback; verify `go test ./internal/interfaces/handler/... -run TestGetLlmSettingsDefaults -count=1`
- [x] 1.4 Replace `isWithinWindow` with clock-injected `isWindowOpenAt(days, now, window, tzOffset)` and add `nextWindowOpenAt(days, now, window, tzOffset)` in `application/imaging/tag_scan.go`; verify `go test ./internal/application/imaging/... -count=1` (scenarios `tag-scan-schedule.selected-day-opens-window`, `tag-scan-schedule.unselected-day-closed`, `tag-scan-schedule.no-days-selected-disables`, `tag-scan-schedule.next-opening-skips-unselected-days`)
- [x] 1.5 Thread the weekday set through `TagScanManager.Start` / `UpdateSchedule` / `GetStatus` / `scheduleLoop` (empty set parks the loop until the schedule changes); verify the imaging package tests still pass with `go test ./internal/application/imaging/... -count=1`
- [x] 1.6 Read the stored weekday set at startup in `cmd/server/app.go` and pass it to `tagScanManager.Start`; verify `go build ./...` in `backend/core`
- [x] 1.7 Document `TagScanSettingsDTO.days` and `UpdateLlmSettingsRequest.tagScanDays` in `docs/api-contracts/core.yaml`, run `make generate-types`, and verify `webapp/src/types/api.ts` exposes `days` on the tag scan type

## 2. Webapp shared schedule editor

- [x] 2.1 Create `webapp/src/components/ui/schedule-editor.tsx`: weekday row in Mon→Sun order with `data-slot="schedule-editor"`, localized labels from `schedule.day.*`, `mode="at"` and `mode="window"` time controls, plus `parseDaysString` / `daysToString` helpers; verify `webapp/src/components/ui/schedule-editor.test.tsx` covers order and Russian labels (scenarios `design-system.schedule-editor-weekday-order`, `design-system.schedule-editor-weekday-labels-localized`)
- [x] 2.2 Move the sync schedule weekday/time controls in `AdminGeneralTab` onto the shared widget and delete its local weekday helpers; verify the extended `AdminGeneralTab.test.tsx` asserts the sync card renders `[data-slot="schedule-editor"]` (scenario `design-system.schedule-editor-shared`)
- [x] 2.3 Add weekday selection to the tagging schedule in `AdminAnalysisTab` through the shared widget, load `settings.tagScan.days` into state and send `tagScanDays` with the save request; verify the extended `AdminAnalysisTab.test.tsx` asserts the tagging card renders the shared editor and that saving sends the selected day (scenarios `design-system.schedule-editor-shared`, `tag-scan-schedule.days-saved-from-ui`)
- [x] 2.4 Add the shared `schedule.day.*` keys to `translations.en.ts` and `translations.ru.ts` together, drop the now-unused `settings.dailySync.day.*` keys, and verify `npm test -- translations` inside `webapp`
- [x] 2.5 Run `npm run lint && npx tsc -b` inside `webapp` and fix every error

## 3. Integration checks and evidence

- [x] 3.1 Run the full core suite `go test ./internal/application/... -count=1` plus `go test ./internal/interfaces/handler/... -count=1` inside `backend/core`
- [x] 3.2 Run the full webapp suite `npm test` inside `webapp`
- [x] 3.3 Register every scenario in `.focused-spec/evidence.yaml` and verify `npx --no-install focused-spec validate --scope <id>` passes for all nine scopes
- [x] 3.4 Execute `npx --no-install focused-spec run --scope <id>` for every scope, then run the mutation check (revert the behaviour, observe failure, restore, observe pass) for each new evidence entry

## Workflow follow-up

- Validate stage (§7 of `docs/agent/sdd-workflow.md`) on the branch diff and record the verdict.
- Sync the delta specs into `openspec/specs/tag-scan-schedule/` and `openspec/specs/design-system/spec.md`, then run `openspec validate --specs`.
- Archive the change under `openspec/changes/archive/YYYY-MM-DD-tag-scan-weekday-schedule/`.
