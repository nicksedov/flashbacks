import { describe, it, expect, vi } from "vitest"
import { render, screen } from "@testing-library/react"
import { I18nProvider } from "@/i18n/I18nProvider"
import { Sidebar } from "./Sidebar"
import { hasClass } from "@/test/dom"

vi.mock("@/providers/useAuth", () => ({
  useAuth: () => ({
    user: { id: 1, login: "admin", displayName: "Admin", role: "admin", isActive: true, accountStatus: "active" },
  }),
}))

function renderSidebar(collapsed: boolean) {
  return render(
    <I18nProvider language="en">
      <Sidebar
        activeTab="gallery-all-images"
        onTabChange={vi.fn()}
        collapsed={collapsed}
        onToggleCollapse={vi.fn()}
        mobileOpen={false}
        onMobileClose={vi.fn()}
      />
    </I18nProvider>
  )
}

describe("Sidebar branding", () => {
  it("shows the base logo in the expanded header without growing its height", () => {
    renderSidebar(false)

    const logo = screen.getByAltText("Flashbacks") as HTMLImageElement
    expect(logo.getAttribute("src")).toBe("/flashbacks_logo_base.png")
    expect(hasClass(logo, "h-16", "object-fill")).toBe(true)

    const header = logo.closest("div")?.parentElement as HTMLElement
    expect(hasClass(header, "h-16")).toBe(true)
  })

  it("keeps a compact logo mark instead of the wordmark when collapsed", () => {
    const { container } = renderSidebar(true)

    expect(screen.queryByAltText("Flashbacks")).toBeNull()

    const aside = container.querySelector("aside") as HTMLElement
    expect(hasClass(aside, "md:w-16")).toBe(true)

    const header = aside.firstElementChild as HTMLElement
    expect(header.querySelector("svg")).toBeTruthy()
  })

  it("labels the logo for screen readers", () => {
    renderSidebar(false)

    const logo = screen.getByAltText("Flashbacks")
    expect(logo.getAttribute("alt")).toBe("Flashbacks")
  })
})
