# Proposal

## Why

Every scheduled background job stores its own copy of schedule data in a
different place: the gallery sync keeps weekdays and run time as four columns on
`app_settings`, the automatic tagging job keeps a weekday set plus a start–end
window on `tag_scan_settings`, and both managers duplicate the same weekday
parsing and timezone arithmetic. Adding a third scheduled job would mean a third
ad-hoc column set and a third copy of the date math.

## What Changes

- Introduce one `schedules` table and a `domain.Schedule` value that describes a
  recurring job: kind, enabled flag, weekday set, start time, optional end time,
  and timezone offset.
- Add `ScheduleRepository` (`Get`, `GetAll`, `Upsert`) as the only read/write path
  for schedule data.
- Migrate the existing schedule kinds (`gallery_sync`, `tag_scan`) onto the table
  with an idempotent boot-time backfill from the legacy columns.
- Change `BackgroundSyncManager` and `TagScanManager` to consume a
  `domain.Schedule` instead of separate `(days, hour, minute, timezoneOffset)`
  parameters, and move the shared weekday/window calculations into
  `domain.Schedule`.
- Stop reading the legacy schedule columns/structs; the public settings and LLM
  settings responses keep exactly the same JSON shape.

No requirements change: this is an internal storage and API refactor, so the
change declares `skip_specs: true`. The behavior specified by
`tag-scan-schedule` (weekday gating, persistence, next-window selection) stays
identical and keeps its existing scenarios and evidence.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. `skip_specs: true` is set in `.openspec.yaml` because the observable
behavior (settings payloads, scheduling semantics) does not change.

## Non-goals

- No change to the public API, DTO field names, or the webapp.
- No change to sync single-run or tagging window semantics.
- No conversion of the on-demand OCR and embedding-backfill managers into
  scheduled jobs.
- Dropping the legacy columns/table is not part of this change (kept for
  rollback); it can follow once the release is proven.

## Impact

- Backend (core): new `domain.Schedule` + `ScheduleRepository`; `db.AutoMigrate`
  gains the table; boot-time seed/backfill; `BackgroundSyncManager`,
  `TagScanManager`, `app.Init`, `handleUpdateSettings`,
  `handleUpdateLlmSettings`, `SettingsLoader`; `AppSettings` loses its schedule
  fields and `TagScanSettings` its window fields.
- Database: one additive table plus a one-time backfill; no destructive
  migration.
- API/Webapp: none — same `syncDays/dailySyncHour/dailySyncMinute/syncTimezoneOffset`
  and `tagScan.{...}` fields, so `docs/api-contracts` and `webapp/src/types` stay
  as they are.
