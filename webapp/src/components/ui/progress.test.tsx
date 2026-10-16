import { describe, it, expect, vi } from "vitest"
import { render, screen } from "@testing-library/react"
import { I18nProvider } from "@/i18n"
import { Card, CardContent } from "@/components/ui/card"
import { ProgressIndicator } from "./progress"
import { ScanProgressBanner } from "@/components/ScanProgressBanner"
import { TagScanProgress } from "@/components/tabs/AdminAnalysisTab"
import { hasClass } from "@/test/dom"
import type { ScanStatusResponse, TagScanStatusResponse } from "@/types"

// AdminAnalysisTab transitively imports the API layer; stub it — TagScanProgress
// does not call any endpoint.
vi.mock("@/api/endpoints", () => ({}))

const baseScan: ScanStatusResponse = {
  scanning: true,
  phase: "scanning",
  progress: "",
  currentDirectory: "",
  totalFiles: 0,
  filesProcessed: 0,
  filesAdded: 0,
  filesDeleted: 0,
  filesUpdated: 0,
}

const baseTag: TagScanStatusResponse = {
  running: true,
  paused: false,
  enabled: true,
  schedule: "23:00 - 07:00",
  scanned: 34,
  remaining: 5699,
  total: 5733,
}

describe("ProgressIndicator", () => {
  it("renders the shared progress-indicator primitive for both scan displays", () => {
    render(
      <I18nProvider language="en">
        <ScanProgressBanner status={baseScan} />
      </I18nProvider>
    )
    expect(document.querySelector('[data-slot="progress-indicator"]')).not.toBeNull()

    render(
      <I18nProvider language="en">
        <TagScanProgress status={baseTag} />
      </I18nProvider>
    )
    // Both the gallery scan banner and the tagging progress use the same primitive.
    expect(document.querySelectorAll('[data-slot="progress-indicator"]').length).toBeGreaterThanOrEqual(2)
  })

  it("renders an indeterminate bar when no total is known", () => {
    render(<ProgressIndicator label="scan" />)

    const root = document.querySelector('[data-slot="progress-indicator"]')
    expect(root?.getAttribute("data-state")).toBe("indeterminate")
  })

  it("renders a determinate bar when a value is known", () => {
    render(<ProgressIndicator value={40} label="scan" />)

    const root = document.querySelector('[data-slot="progress-indicator"]')
    expect(root?.getAttribute("data-state")).toBe("determinate")
  })

  it("shows the phase label in Russian for the ru locale", () => {
    render(
      <I18nProvider language="ru">
        <ScanProgressBanner status={baseScan} />
      </I18nProvider>
    )

    expect(screen.getByText("Сканирование…")).toBeTruthy()
  })

  it("uses semantic tokens and no raw palette literals", () => {
    render(<ProgressIndicator value={50} label="scan" statusText="dir" statsText="1 of 2" />)

    const root = document.querySelector('[data-slot="progress-indicator"]')
    expect(hasClass(root, "bg-muted")).toBe(true)
    expect(root?.outerHTML).not.toMatch(/bg-(blue|gray|slate|zinc|green|red|yellow|amber)-/)
    expect(root?.outerHTML).not.toMatch(/#[0-9a-fA-F]{3,8}\b/)
  })

  it("renders as a borderless inset fill inside a card", () => {
    render(
      <Card>
        <CardContent>
          <ProgressIndicator value={10} />
        </CardContent>
      </Card>
    )

    const root = document.querySelector('[data-slot="progress-indicator"]')
    expect(hasClass(root, "bg-muted")).toBe(true)
    expect(root?.className.includes("border")).toBe(false)
  })
})
