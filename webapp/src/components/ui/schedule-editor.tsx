import type { ReactNode } from "react"
import { Checkbox } from "@/components/ui/checkbox"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { useTranslation, type TranslationKey } from "@/i18n"

// Weekday order for the UI: Mon..Sun, while the API numbers weekdays like Go's
// time.Weekday (1=Mon .. 6=Sat, 0=Sun).
const WEEKDAY_ORDER = [1, 2, 3, 4, 5, 6, 0] as const
const WEEKDAY_KEYS = ["mon", "tue", "wed", "thu", "fri", "sat", "sun"] as const

const HOUR_OPTIONS = Array.from({ length: 24 }, (_, i) => i)
const MINUTE_OPTIONS = Array.from({ length: 12 }, (_, i) => i * 5)

interface HourMinuteSelectsProps {
  idPrefix: string
  hour: number
  minute: number
  onHourChange: (hour: number) => void
  onMinuteChange: (minute: number) => void
}

/** Hour and minute pickers shared by every schedule time field. */
function HourMinuteSelects({ idPrefix, hour, minute, onHourChange, onMinuteChange }: HourMinuteSelectsProps) {
  return (
    <div className="flex items-end gap-2">
      <Select value={String(hour)} onValueChange={(value) => onHourChange(Number(value))}>
        <SelectTrigger id={`${idPrefix}-hour`} className="w-24">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {HOUR_OPTIONS.map((option) => (
            <SelectItem key={option} value={String(option)}>
              {String(option).padStart(2, "0")}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <span className="pb-2 text-muted-foreground">:</span>
      <Select value={String(minute)} onValueChange={(value) => onMinuteChange(Number(value))}>
        <SelectTrigger id={`${idPrefix}-minute`} className="w-24">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {MINUTE_OPTIONS.map((option) => (
            <SelectItem key={option} value={String(option)}>
              {String(option).padStart(2, "0")}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  )
}

interface ScheduleEditorBaseProps {
  /** Prefix for the generated control ids, e.g. "sync" or "tag-scan". */
  idPrefix: string
  /** Selected weekdays indexed 0=Sunday ... 6=Saturday. */
  days: boolean[]
  /** Called with the weekday index (0=Sunday ... 6=Saturday) when a day is toggled. */
  onToggleDay: (dayIndex: number) => void
  /**
   * Action rendered at the end of the time row (e.g. the section's save
   * button), so every schedule section presents its action the same way.
   */
  action?: ReactNode
}

/** A schedule with one run time per selected day. */
interface SingleTimeScheduleProps extends ScheduleEditorBaseProps {
  mode: "at"
  hour: number
  minute: number
  onHourChange: (hour: number) => void
  onMinuteChange: (minute: number) => void
}

/** A schedule with a start–end window inside the selected days. */
interface WindowScheduleProps extends ScheduleEditorBaseProps {
  mode: "window"
  startHour: number
  startMinute: number
  endHour: number
  endMinute: number
  onStartHourChange: (hour: number) => void
  onStartMinuteChange: (minute: number) => void
  onEndHourChange: (hour: number) => void
  onEndMinuteChange: (minute: number) => void
}

export type ScheduleEditorProps = SingleTimeScheduleProps | WindowScheduleProps

/**
 * Shared schedule editor used by every settings section that configures a
 * recurring job. It renders the same weekday row and time controls so the sync
 * and tagging schedules stay visually and behaviourally identical.
 */
export function ScheduleEditor(props: ScheduleEditorProps) {
  const { t } = useTranslation()

  return (
    <div data-slot="schedule-editor" className="space-y-4">
      <div className="space-y-2">
        <Label>{t("schedule.days")}</Label>
        <div className="flex flex-wrap gap-3">
          {WEEKDAY_ORDER.map((dayIndex, uiIndex) => (
            <div key={dayIndex} data-slot="schedule-day" className="flex items-center space-x-1.5">
              <Checkbox
                id={`${props.idPrefix}-day-${dayIndex}`}
                checked={props.days[dayIndex]}
                onCheckedChange={() => props.onToggleDay(dayIndex)}
              />
              <Label htmlFor={`${props.idPrefix}-day-${dayIndex}`} className="cursor-pointer text-sm">
                {t(`schedule.day.${WEEKDAY_KEYS[uiIndex]}` as TranslationKey)}
              </Label>
            </div>
          ))}
        </div>
      </div>

      {props.mode === "at" ? (
        <div data-slot="schedule-time-row" className="flex flex-wrap items-end gap-4">
          <div className="space-y-2">
            <Label htmlFor={`${props.idPrefix}-hour`}>{t("schedule.hour")}</Label>
            <HourMinuteSelects
              idPrefix={props.idPrefix}
              hour={props.hour}
              minute={props.minute}
              onHourChange={props.onHourChange}
              onMinuteChange={props.onMinuteChange}
            />
          </div>
          {props.action}
        </div>
      ) : (
        <div data-slot="schedule-time-row" className="flex flex-wrap items-end gap-4">
          <div className="space-y-2">
            <Label htmlFor={`${props.idPrefix}-start-hour`}>{t("schedule.startTime")}</Label>
            <HourMinuteSelects
              idPrefix={`${props.idPrefix}-start`}
              hour={props.startHour}
              minute={props.startMinute}
              onHourChange={props.onStartHourChange}
              onMinuteChange={props.onStartMinuteChange}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor={`${props.idPrefix}-end-hour`}>{t("schedule.endTime")}</Label>
            <HourMinuteSelects
              idPrefix={`${props.idPrefix}-end`}
              hour={props.endHour}
              minute={props.endMinute}
              onHourChange={props.onEndHourChange}
              onMinuteChange={props.onEndMinuteChange}
            />
          </div>
          {props.action}
        </div>
      )}
    </div>
  )
}
