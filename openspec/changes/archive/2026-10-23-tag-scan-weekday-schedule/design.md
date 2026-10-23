# Design

## Context

See [`proposal.md`](proposal.md) for motivation. Current state that shapes the
approach:

- [`TagScanSettings`](../../../backend/core/internal/domain/media.go:298) is a
  singleton row (`ID = 1`) holding `enabled`, a start time, an end time and a
  timezone offset; it has no weekday data.
- [`TagScanManager`](../../../backend/core/internal/application/imaging/tag_scan.go:41)
  loops on `isWithinWindow()` — a pure clock check that ignores the weekday —
  and on `calculateNextWindowOpen()`, which always targets the next calendar day.
- The sync schedule already models weekdays as a comma-separated string
  (`SyncDays`, `0` = Sunday … `6` = Saturday) parsed by
  `imaging.ParseSyncDays` / `imaging.FormatSyncDays`.
- The sync schedule UI in
  [`AdminGeneralTab`](../../../webapp/src/components/tabs/AdminGeneralTab.tsx:414)
  and the tagging schedule UI in
  [`AdminAnalysisTab`](../../../webapp/src/components/tabs/AdminAnalysisTab.tsx:636)
  each own their own weekday/time markup; only the sync one has weekday
  checkboxes.

## Goals / Non-Goals

**Goals:**

- Reuse the existing sync weekday representation and helpers instead of
  introducing a second weekday format.
- Keep the gating logic pure and clock-injectable so the scenarios can be
  evidenced with deterministic unit tests.
- One schedule-editor component that renders both sections with identical
  weekday/time controls and localized labels.

**Non-Goals:**

- No change to the tagging window semantics (still start–end, overnight allowed).
- No change to the sync schedule persistence or manager.
- No new dependency; the widget composes the existing `Checkbox`, `Label` and
  `Select` primitives from `components/ui`.

## Decisions

1. **Store the weekday set as `days string` on `TagScanSettings`.**
   Alternative: a separate join table of weekday rows. Rejected — the sync
   schedule already proves the comma-separated form works, it needs no
   migration beyond a nullable column with a default, and `ParseSyncDays`
   already parses and validates it. The JSON tag is `days` in the settings
   response and `tagScanDays` in the update request, mirroring
   `syncDays` naming without colliding with it.

2. **Extend the manager API with the weekday set instead of embedding it in the
   time window.** `Start` / `UpdateSchedule` take `days []time.Weekday` next to
   the existing window fields, so `GetStatus().WindowOpen` stays meaningful and
   callers keep a single source of truth. The zero-value window (both times
   equal) keeps meaning "always open" *within selected days*.

3. **Replace `isWithinWindow` with a weekday-aware, clock-injected helper.**
   `isWindowOpenAt(days, now, window, tzOffset)` and
   `nextWindowOpenAt(days, now, window, tzOffset)` become pure functions of an
   explicit `now`, and the manager passes `time.Now()`. Alternative: keep
   `time.Now()` inside and test around it — rejected because the weekday
   scenarios would only be testable on specific days of the week.

4. **Empty weekday set means "never open".** It matches the sync schedule, where
   unchecking every day disables the job, and avoids a surprising "runs every
   day" default for a user who deselected everything. `nextWindowOpenAt` returns
   a far-future time in that case so `scheduleLoop` parks until the schedule
   changes.

5. **Shared widget `components/ui/schedule-editor.tsx`.** It owns the weekday
   row (`data-slot="schedule-editor"`), the Mon→Sun order, the day-label
   translation keys and the hour/minute `Select`s. Its props carry either a
   single time (`mode="at"`, used by sync) or a start/end window
   (`mode="window"`, used by tagging), so both cards share the layout while each
   keeps its own semantics. Alternative: force tagging into a single run time —
   rejected as a behaviour change outside this proposal.

6. **Weekday labels move to a shared i18n namespace** (`schedule.day.*`) used by
   the widget; the sync-specific keys are removed once the sync section renders
   through the widget, so no duplicate copy of the same labels is kept.

## Risks / Trade-offs

- **GORM auto-migration adds `days` to an existing table** → the column carries a
  `'1,2,3,4,5'` default, so the migration backfills existing rows and they keep
  the weekday behaviour they have today. An explicitly empty value stays empty
  and disables tagging, matching the "no selected weekday" scenario.
- **`WindowOpen` could disagree with the loop** → both read the same helper and
  the same stored fields; the loop only adds the wait for the next opening.
- **A tab left open can save a stale weekday set** → the widget edits local state
  and saves it in one request with the rest of the schedule, exactly like the
  sync card; no partial weekday writes exist.
- **Two layouts in one component** (`at` vs `window`) → kept as a discriminated
  prop union so the compiler rejects mixed usage.

## Migration Plan

1. Ship the core change first (column, DTO, handler, manager). The migration
   backfills existing rows with the default weekday set; a row whose `days` is
   empty afterwards is an explicit "disabled" choice and stays empty.
2. Ship the webapp once `make generate-types` has refreshed `api.ts`.
3. Rollback is a redeploy of the previous images; the extra column is additive
   and ignored by older code.
