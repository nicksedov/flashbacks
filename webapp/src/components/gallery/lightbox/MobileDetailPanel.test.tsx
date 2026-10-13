import { useState } from "react"
import { describe, it, expect, vi } from "vitest"
import { render, screen, fireEvent } from "@testing-library/react"
import { MobileDetailPanel } from "./MobileDetailPanel"
import { hasClass } from "@/test/dom"

vi.mock("@/i18n", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

function Harness({
  initiallyOpen = false,
  onChange,
}: {
  initiallyOpen?: boolean
  onChange?: (open: boolean) => void
}) {
  const [open, setOpen] = useState(initiallyOpen)
  return (
    <div className="relative h-full">
      <MobileDetailPanel
        open={open}
        onOpenChange={(next) => {
          onChange?.(next)
          setOpen(next)
        }}
      >
        <button type="button">Panel action</button>
      </MobileDetailPanel>
    </div>
  )
}

const panel = () => screen.getByTestId("mobile-detail-panel")
const toggle = () => screen.getByTestId("mobile-detail-toggle")

describe("MobileDetailPanel", () => {
  it("overlays the panel without adding a side column below md", () => {
    render(<Harness />)

    // Below md the panel is an absolute overlay (out of the flex flow) and only
    // becomes a static side column from md up, so no side column is added.
    expect(hasClass(panel(), "absolute")).toBe(true)
    expect(hasClass(panel(), "md:static")).toBe(true)
    expect(panel().classList.contains("w-full")).toBe(false)
  })

  it("keeps the fixed-width column at md and above", () => {
    render(<Harness />)

    // The desktop branch is the static fixed-width column beside the image…
    expect(hasClass(panel(), "md:static", "md:h-full", "md:w-[400px]", "lg:w-[450px]")).toBe(true)
    // …and every mobile-only sizing rule is scoped below md. Tailwind emits the
    // orientation media block after the md block, so a bare `landscape:` utility
    // would win at desktop widths and squeeze the image area to ~20%.
    expect(hasClass(panel(), "max-md:landscape:w-[80%]")).toBe(true)
    expect(panel().className.split(" ").some((c) => c.startsWith("landscape:"))).toBe(false)
  })

  it("starts collapsed below md", () => {
    render(<Harness />)

    expect(panel().getAttribute("data-open")).toBe("false")
    expect(hasClass(panel(), "hidden")).toBe(true)
    expect(screen.queryByTestId("mobile-detail-scrim")).toBeNull()
  })

  it("expands the panel when the toggle is activated", () => {
    render(<Harness />)

    fireEvent.click(toggle())

    expect(panel().getAttribute("data-open")).toBe("true")
    expect(screen.queryByTestId("mobile-detail-scrim")).not.toBeNull()
  })

  it("collapses the panel when the toggle is activated again", () => {
    render(<Harness />)

    fireEvent.click(toggle())
    fireEvent.click(toggle())

    expect(panel().getAttribute("data-open")).toBe("false")
    expect(screen.queryByTestId("mobile-detail-scrim")).toBeNull()
  })

  it("keeps the toggle at a consistent reachable position in both states", () => {
    render(<Harness />)

    const collapsedClasses = toggle().className
    expect(hasClass(toggle(), "absolute")).toBe(true)

    fireEvent.click(toggle())

    expect(toggle().className).toBe(collapsedClasses)
  })

  it("keeps the panel absolutely positioned so the image keeps its size", () => {
    render(<Harness />)

    fireEvent.click(toggle())

    // Only absolute positioning keeps the overlay out of the flex flow, so the
    // image is never re-measured when the panel opens.
    expect(hasClass(panel(), "absolute")).toBe(true)
    expect(panel().classList.contains("static")).toBe(false)
  })

  it("covers about 80% of the area when expanded below md", () => {
    render(<Harness />)

    fireEvent.click(toggle())

    expect(hasClass(panel(), "h-[80dvh]")).toBe(true)
    expect(hasClass(panel(), "max-md:landscape:w-[80%]")).toBe(true)
  })

  it("renders a scrim over the exposed image when expanded", () => {
    render(<Harness />)

    fireEvent.click(toggle())

    const scrim = screen.getByTestId("mobile-detail-scrim")
    expect(hasClass(scrim, "absolute", "inset-0", "bg-background/60")).toBe(true)
  })

  it("collapses when the exposed image area is tapped", () => {
    render(<Harness />)

    fireEvent.click(toggle())
    fireEvent.click(screen.getByTestId("mobile-detail-scrim"))

    expect(panel().getAttribute("data-open")).toBe("false")
  })

  it("anchors to the bottom in portrait", () => {
    render(<Harness />)

    fireEvent.click(toggle())

    // Portrait is the base overlay: pinned to the bottom, spanning full width.
    expect(hasClass(panel(), "left-0", "right-0", "bottom-0")).toBe(true)
  })

  it("anchors to the right in landscape", () => {
    render(<Harness />)

    fireEvent.click(toggle())

    expect(hasClass(panel(), "max-md:landscape:top-0", "max-md:landscape:left-auto", "max-md:landscape:right-0")).toBe(true)
  })

  it("keeps the panel expanded across orientation changes", () => {
    render(<Harness />)

    fireEvent.click(toggle())
    fireEvent(window, new Event("orientationchange"))
    fireEvent(window, new Event("resize"))

    expect(panel().getAttribute("data-open")).toBe("true")
    expect(hasClass(panel(), "max-md:landscape:right-0")).toBe(true)
  })

  it("exposes an accessible name and a 44px touch target", () => {
    render(<Harness />)

    const button = screen.getByRole("button", { name: "lightbox.panel.show" })
    expect(button).toBeTruthy()
    expect(hasClass(button, "h-11", "min-w-11")).toBe(true)
  })

  it("exposes aria-expanded and aria-controls on the toggle", () => {
    render(<Harness />)

    expect(toggle().getAttribute("aria-expanded")).toBe("false")
    expect(toggle().getAttribute("aria-controls")).toBe(panel().id)

    fireEvent.click(toggle())

    expect(toggle().getAttribute("aria-expanded")).toBe("true")
    expect(toggle().getAttribute("aria-controls")).toBe(panel().id)
  })

  it("shows a visible label on the collapsed toggle", () => {
    render(<Harness />)

    expect(toggle().textContent).toContain("lightbox.panel.details")
  })

  it("uses a dynamic viewport height so the input stays above the keyboard", () => {
    render(<Harness />)

    fireEvent.click(toggle())

    // `dvh` tracks the dynamic visual viewport, so a focused input stays above
    // the on-screen keyboard without a visualViewport listener.
    expect(hasClass(panel(), "h-[80dvh]")).toBe(true)
  })

  it("insets the panel from the safe-area edges", () => {
    render(<Harness />)

    fireEvent.click(toggle())

    expect(hasClass(panel(), "pb-[env(safe-area-inset-bottom)]")).toBe(true)
    expect(hasClass(panel(), "max-md:landscape:pr-[env(safe-area-inset-right)]")).toBe(true)
  })

  it("suppresses the transition under reduced motion", () => {
    render(<Harness />)

    fireEvent.click(toggle())

    // Transitions are gated behind `motion-safe:`, so `prefers-reduced-motion`
    // suppresses the animation.
    expect(hasClass(panel(), "motion-safe:transition-all")).toBe(true)
    expect(panel().classList.contains("transition-all")).toBe(false)
  })

  it("moves focus into the panel and back to the toggle", () => {
    render(<Harness />)

    fireEvent.click(toggle())
    expect(panel().contains(document.activeElement)).toBe(true)

    fireEvent.click(toggle())
    expect(document.activeElement).toBe(toggle())
  })

  it("collapses on Escape before the dialog closes", () => {
    const onChange = vi.fn()
    render(<Harness onChange={onChange} />)

    fireEvent.click(toggle())
    expect(panel().getAttribute("data-open")).toBe("true")

    fireEvent.keyDown(document, { key: "Escape" })

    expect(onChange).toHaveBeenCalledWith(false)
    expect(panel().getAttribute("data-open")).toBe("false")
  })
})
