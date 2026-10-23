import { describe, it, expect, vi, beforeEach } from "vitest"
import { render } from "@testing-library/react"
import { ScheduleEditor } from "./schedule-editor"

// Controls which locale the mocked translator resolves.
const languageState = vi.hoisted(() => ({ language: "en" as "en" | "ru" }))

vi.mock("@/i18n", async () => {
  const en = await import("@/i18n/translations.en")
  const ru = await import("@/i18n/translations.ru")
  const translate = (key: string) => {
    const dictionary = (
      languageState.language === "ru" ? ru.translationsRu : en.translationsEn
    ) as Record<string, string>
    return dictionary[key] ?? key
  }
  return { useTranslation: () => ({ language: languageState.language, t: translate }) }
})

const days = [false, true, true, true, true, true, false] // Mon-Fri selected

// weekdayControls returns the rendered weekday controls in DOM order, pairing
// each control's label with the API weekday number encoded in its checkbox id.
function weekdayControls(container: HTMLElement): { label: string; dayId: string }[] {
  return Array.from(container.querySelectorAll('[data-slot="schedule-day"]')).map((node) => {
    const checkbox = node.querySelector("[id]")
    return {
      label: node.textContent ?? "",
      dayId: checkbox?.getAttribute("id") ?? "",
    }
  })
}

function renderAtEditor() {
  return render(
    <ScheduleEditor
      mode="at"
      idPrefix="test-schedule"
      days={days}
      onToggleDay={() => {}}
      hour={3}
      minute={30}
      onHourChange={() => {}}
      onMinuteChange={() => {}}
    />
  )
}

beforeEach(() => {
  languageState.language = "en"
})

describe("ScheduleEditor", () => {
  // design-system.schedule-editor-weekday-order
  it("orders weekday controls Monday through Sunday", () => {
    const { container } = renderAtEditor()

    const controls = weekdayControls(container)

    expect(controls.map((control) => control.label)).toEqual([
      "Mon",
      "Tue",
      "Wed",
      "Thu",
      "Fri",
      "Sat",
      "Sun",
    ])
    // The checkbox id carries the API weekday number, so this proves the row
    // starts at Monday (1) and ends at Sunday (0).
    expect(controls.map((control) => control.dayId)).toEqual([
      "test-schedule-day-1",
      "test-schedule-day-2",
      "test-schedule-day-3",
      "test-schedule-day-4",
      "test-schedule-day-5",
      "test-schedule-day-6",
      "test-schedule-day-0",
    ])
  })

  it("renders the section action at the end of the time row", () => {
    const { container } = render(
      <ScheduleEditor
        mode="at"
        idPrefix="test-schedule"
        days={days}
        onToggleDay={() => {}}
        hour={3}
        minute={30}
        onHourChange={() => {}}
        onMinuteChange={() => {}}
        action={<button type="button">Save</button>}
      />
    )

    const timeRow = container.querySelector('[data-slot="schedule-time-row"]')
    expect(timeRow).not.toBeNull()
    expect(timeRow?.lastElementChild?.textContent).toBe("Save")
  })

  // design-system.schedule-editor-weekday-labels-localized
  it("labels weekdays in Russian under the Russian interface", () => {
    languageState.language = "ru"

    const { container } = renderAtEditor()

    expect(weekdayControls(container).map((control) => control.label)).toEqual([
      "Пн",
      "Вт",
      "Ср",
      "Чт",
      "Пт",
      "Сб",
      "Вс",
    ])
  })
})
