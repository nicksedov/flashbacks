import { describe, it, expect, vi } from "vitest"
import { render, screen } from "@testing-library/react"
import { AdminSettingsTab } from "./AdminSettingsTab"
import { hasClass } from "@/test/dom"

vi.mock("@/i18n", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

vi.mock("./AdminGeneralTab", () => ({ AdminGeneralTab: () => null }))
vi.mock("./AdminAnalysisTab", () => ({ AdminAnalysisTab: () => null }))
vi.mock("./AdminLlmProvidersTab", () => ({ AdminLlmProvidersTab: () => null }))

describe("AdminSettingsTab", () => {
  it("wraps the settings tab list instead of overflowing on mobile", () => {
    render(<AdminSettingsTab />)

    const tablist = screen.getByRole("tablist")
    expect(hasClass(tablist, "flex-wrap")).toBe(true)
  })
})
