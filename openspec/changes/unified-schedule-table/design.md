# Design

## Context

See [`proposal.md`](proposal.md) for motivation. Current state that shapes the
approach:

- [`AppSettings`](../../../backend/core/internal/domain/media.go:127) carries
  `SyncDays`, `DailySyncHour`, `DailySyncMinute`, `SyncTimezoneOffset`.
- [`TagScanSettings`](../../../backend/core/internal/domain/media.go:298) carries
  `Enabled`, `Days`, `StartHour`, `StartMinute`, `EndHour`, `EndMinute`,
  `TimezoneOffset`.
- [`BackgroundSyncManager`](../../../backend/core/internal/application/imaging/background_sync.go:34)
  and [`TagScanManager`](../../../backend/core/internal/application/imaging/tag_scan.go:41)
  each own their own schedule fields, their own `Start`/`UpdateSchedule`
  signatures, and their own weekday/timezone math
  (`calculateNextRunTime`, `isWindowOpenAt`, `nextWindowOpenAt`, `ParseSyncDays`).
- GORM `AutoMigrate` is the only schema mechanism; there are no migration files.
  The full design rationale lives in
  [`plans/unified-schedule-table.md`](../../../plans/unified-schedule-table.md:1).

## Goals / Non-Goals

**Goals:**

- One `schedules` table, one `domain.Schedule` value, one repository, one set of
  weekday/window helpers, consumed by both managers.
- Behavior-preserving migration: no settings payload, scheduling semantic, or
  webapp change.
- Rollback is a redeploy of the previous build (additive schema only).

**Non-Goals:**

- Dropping the legacy columns or the `tag_scan_settings` table in this change.
- Converting OCR / embedding-backfill into scheduled jobs.
- Unifying the two managers into one generic loop (they have different work
  units: a single pass vs. a window-limited scan).

## Decisions

1. **String `kind` column instead of a numeric enum or three nullable columns.**
   `gallery_sync` and `tag_scan` are rows in the same table with a unique index on
   `kind`; adding a kind needs no schema change and no second lookup table. A
   numeric enum would hide intent in logs and require a mapping table in code.

2. **`end == start` means "single shot".** The sync schedule has one run time,
   tagging has a start–end window. Rather than two shapes, a Schedule where end
   equals start is a single run at that time; `NextRunAt` implements it and
   `IsWindowOpenAt` treats equal bounds as a 24-hour window only for tagging, which
   is why the two helpers stay distinct methods on the same value.

3. **`enabled` moves into the unified table.** `tag_scan_settings.enabled` becomes
   `schedules.enabled` for the `tag_scan` kind. The sync job has no explicit
   enabled flag today; an empty weekday set keeps meaning "disabled", and the row's
   `enabled` defaults to true so sync behavior is unchanged.

4. **Seed/backfill runs on boot, idempotently, and only fills missing kinds.**
   It reads the legacy columns once to create the row for each kind; if the row
   exists it does nothing, so user edits are never overwritten on restart.

5. **Managers take `domain.Schedule` and keep their loops.** `Start(s)`,
   `UpdateSchedule(s)` replace the exploded arguments; each loop reads the fields
   it needs. This keeps the diff mechanical and the existing concurrency /
   pause-resume behavior untouched.

6. **Handlers become write-through.** `handleUpdateSettings` and
   `handleUpdateLlmSettings` build a `domain.Schedule`, call
   `ScheduleRepository.Upsert`, then call the manager's `UpdateSchedule`. Responses
   keep serializing the same DTO fields from the schedule row, so no contract edit.

## Risks / Trade-offs

- **Double source of truth during rollout** → the seed runs before managers start
  and handlers write only to the new table, so the legacy columns freeze at their
  pre-upgrade values; a redeploy of the old build still finds them intact.
- **Partial write between schedule and manager state** → the handler upserts first
  and only then updates the in-memory manager; a failed upsert leaves both the row
  and the manager unchanged.
- **A schedule row is edited while the manager waits** → unchanged from today: the
  manager is signalled through its existing `scheduleCh`.
- **Sync `enabled` semantics** → sync keeps "empty days = do nothing"; the row's
  `enabled` is true by default so no behavior shifts.
- **Wisdom of keeping the legacy columns** → extra dead columns until a follow-up
  cleanup; accepted to preserve the rollback path.

## Migration Plan

1. `AutoMigrate(&domain.Schedule{})` plus a unique index on `kind`.
2. Boot seed: create `gallery_sync` from `app_settings` and `tag_scan` from
   `tag_scan_settings` when absent (defaults `1,2,3,4,5`, sync 03:00, tagging
   22:00–07:00).
3. Switch managers and handlers to the repository; remove the schedule fields from
   the Go domain structs (DB columns remain).
4. Rollback = redeploy the previous build; the new table is ignored by it.

## Open Questions

None that change the specs, the approach, or the task breakdown. The follow-up
cleanup (dropping `app_settings` schedule columns and `tag_scan_settings`) is
explicitly deferred.
