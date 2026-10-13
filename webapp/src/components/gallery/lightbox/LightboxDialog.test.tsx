import type { ReactNode } from "react"
import { describe, it, expect, vi } from "vitest"
import { render, screen, fireEvent } from "@testing-library/react"
import { Check } from "lucide-react"
import { LightboxDialog } from "./LightboxDialog"
import { LightboxAction, LightboxInfoReadout, LightboxTabBar } from "./lightboxChrome"
import { MobileDetailPanel } from "./MobileDetailPanel"
import { hasClass } from "@/test/dom"

vi.mock("@/i18n", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

/** Walk ancestors checking for a class token (Tailwind `:` classes break CSS selectors). */
function hasAncestorWithClass(el: Element | null, className: string): boolean {
  let node: Element | null = el
  while (node) {
    if (node.classList.contains(className)) return true
    node = node.parentElement
  }
  return false
}

function renderDialog(children: ReactNode) {
  return render(
    <LightboxDialog
      open
      onOpenChange={vi.fn()}
      titleKey="lightbox.title"
      descriptionKey="lightbox.description"
    >
      {children}
    </LightboxDialog>
  )
}

describe("LightboxDialog", () => {
  it("wraps the tab bar without horizontal overflow below md", () => {
    renderDialog(<LightboxTabBar value="ai" onValueChange={vi.fn()} />)

    const tabBar = screen.getByTestId("lightbox-tab-bar")
    // The list wraps below md so the tabs stay within the lightbox width.
    expect(hasClass(tabBar, "max-md:flex-wrap")).toBe(true)
  })

  it("renders a single close control", () => {
    renderDialog(<div />)

    // The lightbox owns its close control; the shared dialog close (the one the
    // primitive labels "Close") is suppressed, because two overlapping controls of
    // different styles is the regression this guards.
    expect(screen.getAllByTestId("lightbox-close")).toHaveLength(1)
    expect(screen.queryByRole("button", { name: "Close" })).toBeNull()
  })

  it("draws the close control above the detail panel", () => {
    renderDialog(
      <MobileDetailPanel open onOpenChange={vi.fn()}>
        <span>details</span>
      </MobileDetailPanel>
    )

    // The panel overlay sits at z-20 and its scrim at z-10; the close control is
    // stacked above both, so the panel can never cover it.
    expect(hasClass(screen.getByTestId("mobile-detail-panel"), "z-20")).toBe(true)
    expect(hasClass(screen.getByTestId("lightbox-close"), "z-30")).toBe(true)
  })

  it("keeps the tab bar on a single row at md and above", () => {
    renderDialog(<LightboxTabBar value="ai" onValueChange={vi.fn()} />)

    const tabBar = screen.getByTestId("lightbox-tab-bar")
    // The wrap is scoped below md, so the desktop tab row stays single-line.
    expect(tabBar.classList.contains("flex-wrap")).toBe(false)
    expect(hasClass(tabBar, "max-md:flex-wrap")).toBe(true)
  })

  it("opens the info popup inside the lightbox on mobile", () => {
    const text = "1 234 / 8 000"
    renderDialog(<LightboxInfoReadout text={text} />)

    fireEvent.click(screen.getByRole("button", { name: text }))

    const popup = screen.getByRole("status")
    // The readout sits at the panel's left edge, so the popup is left-anchored
    // and width-clamped: it opens inside the lightbox instead of overflowing the
    // lightbox's left edge and being clipped.
    expect(hasClass(popup, "left-0", "right-auto")).toBe(true)
    expect(popup.classList.contains("max-w-[min(16rem,calc(100vw_-_7rem))]")).toBe(true)
  })

  it("hides action text labels below md while keeping the icon and accessible name", () => {
    renderDialog(<LightboxAction icon={<Check />} label="Do thing" />)

    const action = screen.getByRole("button", { name: "Do thing" })
    expect(action.querySelector("svg")).not.toBeNull()
    expect(hasClass(action.querySelector("span"), "hidden", "md:inline")).toBe(true)
  })

  it("gives the close control a 44px touch target below md", () => {
    renderDialog(<div />)

    const close = screen.getByTestId("lightbox-close")
    expect(hasClass(close, "h-11", "w-11")).toBe(true)
    expect(hasClass(close, "md:h-8", "md:w-8")).toBe(true)
  })

  it("collapses a long informational readout to an info icon below md", () => {
    const text = "1 234 / 8 000"
    renderDialog(<LightboxInfoReadout text={text} />)

    const readout = screen.getByTestId("lightbox-info-readout")
    // The long label is hidden below md and only shown from md up…
    expect(hasClass(readout.querySelector("span"), "hidden", "md:inline")).toBe(true)
    // …while an icon-only control takes its place below md.
    const icon = screen.getByRole("button", { name: text })
    expect(hasAncestorWithClass(icon, "md:hidden")).toBe(true)
  })
})
