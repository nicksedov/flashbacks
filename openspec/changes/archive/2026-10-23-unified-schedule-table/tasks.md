# Tasks

## 1. Domain value and repository

- [x] 1.1 Add `domain/schedule.go` with `ScheduleKind` (`gallery_sync`, `tag_scan`), `Schedule` (kind, enabled, days, start/end hour+minute, timezoneOffset, timestamps, `TableName() = "schedules"`), weekday parsing/formatting and the helpers `Weekdays()`, `IsWindowOpenAt(now)`, `NextRunAt(now)`, `NextWindowOpenAt(now)`; verify with `go test ./internal/domain/... -run TestSchedule -count=1`
- [x] 1.2 Point `imaging.ParseSyncDays` / `imaging.FormatSyncDays` at the domain helpers so existing callers keep working; verify `go test ./internal/application/imaging/... -run TestParseSyncDays -count=1` and `-run TestFormatSyncDays -count=1`
- [x] 1.3 Add `domain/repository/schedule_repo.go` (`ScheduleRepository`: `Get(kind)`, `GetAll()`, `Upsert()`) plus its in-memory-SQLite tests for get/upsert round-trip, `ErrRecordNotFound` on a missing kind, and empty `days` preservation; verify `go test ./internal/domain/repository/... -run TestScheduleRepository -count=1`

## 2. Table and backfill

- [x] 2.1 Add `&domain.Schedule{}` to `db.AutoMigrate` and a unique index on `kind`; verify with a test that migrates a fresh SQLite DB and asserts the `schedules` table exists with the `kind` index
- [x] 2.2 Implement the idempotent boot seed that creates `gallery_sync` from `app_settings` and `tag_scan` from `tag_scan_settings` when absent (defaults: weekdays `1,2,3,4,5`, sync 03:00, tagging 22:00–07:00); verify with tests for backfill-from-legacy, defaults on empty tables, and no overwrite when a row already exists
- [x] 2.3 Seed both schedule rows in `testutil.NewTestDB` so handler/manager tests share the same fixture; verify `go test ./internal/interfaces/handler/... -count=1`

## 3. Managers consume domain.Schedule

- [x] 3.1 Change `BackgroundSyncManager.Start` / `UpdateSchedule` to take `domain.Schedule`, keeping the loop and `syncCh` signalling; verify `go test ./internal/application/imaging/... -run TestBackgroundSync -count=1`
- [x] 3.2 Change `TagScanManager.Start` / `UpdateSchedule` to take `domain.Schedule` (enabled from the schedule, window from start/end) and drop its private copies of the weekday/window helpers; verify `go test ./internal/application/imaging/... -count=1` including the existing `TestTagScan*` weekday tests

## 4. Wiring and legacy field removal

- [x] 4.1 Inject `ScheduleRepository` into `Server`/`App` and read both schedules at boot in `app.Init`; verify `go build ./...` in `backend/core`
- [x] 4.2 Make `handleUpdateSettings` upsert the `gallery_sync` schedule then call `backgroundSync.UpdateSchedule`; verify `go test ./internal/interfaces/handler/... -run TestUpdateSettings -count=1` and that the settings response still carries `syncDays`, `dailySyncHour`, `dailySyncMinute`, `syncTimezoneOffset`
- [x] 4.3 Make `handleUpdateLlmSettings` upsert the `tag_scan` schedule and build the `tagScan` DTO from it; verify `go test ./internal/interfaces/handler/... -run TestUpdateLlmSettingsTagScanDaysPersisted -count=1` and the unchanged `tagScan` JSON shape
- [x] 4.4 Remove the schedule fields from `AppSettings` and the window fields from `TagScanSettings` (plus their defaults, `SettingsLoader` fallbacks and `llm_repo` tag-scan read/upsert methods), mapping DTOs from the schedule rows instead; verify `go test ./internal/interfaces/... ./internal/domain/... -count=1`

## 5. Verification

- [x] 5.1 Run `go vet ./...`, `go test ./internal/application/... -count=1` and `go test ./internal/interfaces/... -count=1` inside `backend/core`
- [x] 5.2 Run `npx --no-install focused-spec validate --scope current --strict` and re-run the scenarios that cover the migrated behavior (`tag-scan-schedule.days-persisted`, `tag-scan-schedule.selected-day-opens-window`, `tag-scan-schedule.unselected-day-closed`, `tag-scan-schedule.no-days-selected-disables`, `tag-scan-schedule.next-opening-skips-unselected-days`) to prove the refactor is behavior-preserving
- [x] 5.3 Copy the relevant evidence entries in `.focused-spec/evidence.yaml` if a scenario's test selector moved, and confirm `focused-spec run --scope current --scenario <id>` still passes for each one

## Workflow follow-up

- Validate stage (§7 of `docs/agent/sdd-workflow.md`) on the branch diff; the review must confirm the public DTO/OpenAPI shape is untouched.
- No delta specs to sync (`skip_specs: true`); confirm `openspec validate --specs` still passes.
- Archive the change under `openspec/changes/archive/YYYY-MM-DD-unified-schedule-table/`.
