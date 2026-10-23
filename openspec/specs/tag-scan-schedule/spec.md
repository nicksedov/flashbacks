# Tag Scan Schedule Specification

## Purpose

Schedules the automatic image tagging job on the weekdays the administrator chooses, so tagging runs only on selected days and can be aligned with the gallery sync schedule.

## Requirements

### Requirement: Tagging runs only on the selected weekdays

The automatic image tagging job SHALL open its scan window only on the selected
weekdays, within the configured start–end window in the administrator's
timezone.

<!-- focused-spec:id=tag-scan-schedule.selected-day-opens-window -->
#### Scenario: Window opens on a selected weekday

- **WHEN** the current local weekday is one of the selected weekdays and the local time is inside the tagging window
- **THEN** the tag scan manager reports its scan window as open

<!-- focused-spec:id=tag-scan-schedule.unselected-day-closed -->
#### Scenario: Window stays closed on an unselected weekday

- **WHEN** the current local weekday is not selected although the local time is inside the tagging window
- **THEN** the tag scan manager reports its scan window as closed

<!-- focused-spec:id=tag-scan-schedule.no-days-selected-disables -->
#### Scenario: No selected weekday keeps the window closed

- **WHEN** no weekday is selected
- **THEN** the tag scan manager reports its scan window as closed at every local time

### Requirement: The next window opening lands on a selected weekday

When the tag scan manager waits outside the window, it SHALL target the next
window opening on a selected weekday instead of the next calendar day.

<!-- focused-spec:id=tag-scan-schedule.next-opening-skips-unselected-days -->
#### Scenario: Next opening skips an unselected weekday

- **WHEN** the manager calculates the next window opening from a local day whose weekday is not selected
- **THEN** the returned opening time falls on the next selected weekday

### Requirement: The tagging weekday set is persisted and returned

The core service SHALL store the tagging weekday set with the tag scan settings
and return it in the LLM settings response, numbered like the sync schedule
(0 = Sunday … 6 = Saturday).

<!-- focused-spec:id=tag-scan-schedule.days-persisted -->
#### Scenario: Saved weekdays come back in the settings response

- **WHEN** an administrator saves the tagging schedule with a weekday set
- **THEN** the LLM settings response reports that same weekday set

### Requirement: The Analysis tab offers weekday selection for tagging

The Analysis tab SHALL let the administrator choose the tagging weekdays and
SHALL save the chosen set together with the tagging schedule.

<!-- focused-spec:id=tag-scan-schedule.days-saved-from-ui -->
#### Scenario: Selected weekdays are sent with the schedule

- **WHEN** an administrator selects a weekday in the tagging schedule and saves it
- **THEN** the settings update request carries a weekday set that contains the selected day
