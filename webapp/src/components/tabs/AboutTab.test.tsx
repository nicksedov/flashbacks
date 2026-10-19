import { describe, it, expect } from "vitest"
import { render, screen, fireEvent } from "@testing-library/react"
import { I18nProvider } from "@/i18n/I18nProvider"
import { AboutTab } from "./AboutTab"
import type { Language } from "@/i18n"

function openThirdParty(language: Language) {
  const view = render(
    <I18nProvider language={language}>
      <AboutTab />
    </I18nProvider>
  )

  const tabLabel = language === "ru" ? "Сторонние компоненты" : "Third-party components"
  fireEvent.mouseDown(screen.getByRole("tab", { name: tabLabel }))

  return view
}

function componentItems(container: HTMLElement): string[] {
  return Array.from(container.querySelectorAll("ul.divide-y > li")).map(
    (item) => item.textContent?.replace(/\s+/g, " ").trim() ?? ""
  )
}

describe("AboutTab third-party components", () => {
  it("renders a single combined list with every component name and license", () => {
    const { container } = openThirdParty("en")
    const items = componentItems(container)

    expect(items.length).toBeGreaterThan(0)
    for (const item of items) {
      expect(item.length).toBeGreaterThan(0)
    }

    const links = Array.from(container.querySelectorAll("ul.divide-y a")) as HTMLAnchorElement[]
    expect(links.length).toBe(items.length)
    expect(links.every((link) => link.getAttribute("target") === "_blank")).toBe(true)
    expect(links.every((link) => link.getAttribute("rel") === "noopener noreferrer")).toBe(true)
    expect(links.every((link) => (link.getAttribute("href") ?? "").startsWith("https://"))).toBe(true)
  })

  it("attributes the backend Go frameworks", () => {
    const { container } = openThirdParty("en")
    const items = componentItems(container)

    for (const name of ["Gin", "GORM"]) {
      expect(items.some((item) => item.startsWith(name))).toBe(true)
    }

    // Backend entries link to the dependency license text, not to the project itself.
    const ginLink = Array.from(container.querySelectorAll("ul.divide-y a")).find(
      (link) => link.textContent?.trim() === "Gin"
    ) as HTMLAnchorElement
    expect(ginLink.getAttribute("href")).toContain("gin-gonic/gin")
  })

  it("excludes modules owned by the Flashbacks project", () => {
    const { container } = openThirdParty("en")
    const html = container.querySelector("ul.divide-y")?.innerHTML ?? ""

    expect(html).not.toContain("flashbacks")
    expect(html).not.toContain("github.com/flashbacks")
  })

  it("keeps the frontend attributions in the combined list", () => {
    const { container } = openThirdParty("en")
    const items = componentItems(container)

    for (const name of ["React", "lucide-react", "react-leaflet", "sonner"]) {
      expect(items.some((item) => item.startsWith(name))).toBe(true)
    }
  })

  it("shows the same components and licenses in both locales", () => {
    const en = openThirdParty("en")
    const enItems = componentItems(en.container)
    en.unmount()

    const ru = openThirdParty("ru")
    const ruItems = componentItems(ru.container)

    expect(ruItems).toEqual(enItems)
  })

  it("matches the reviewed component list", () => {
    const { container } = openThirdParty("en")

    expect(componentItems(container)).toMatchSnapshot()
  })
})
