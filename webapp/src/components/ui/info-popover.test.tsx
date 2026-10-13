import { describe, it, expect, vi, afterEach } from "vitest"
import { render, screen, fireEvent, act } from "@testing-library/react"
import { InfoPopover, INFO_POPOVER_AUTO_HIDE_MS } from "./info-popover"

const TEXT = "1 234 images in gallery"

afterEach(() => {
  vi.useRealTimers()
})

describe("InfoPopover", () => {
  it("renders an icon-only trigger with an accessible name", () => {
    render(<InfoPopover text={TEXT} />)

    // The icon trigger carries the full text as its accessible name.
    expect(screen.getByRole("button", { name: TEXT })).toBeTruthy()
    // The full text is not rendered as visible popup content until activated.
    expect(screen.queryByRole("status")).toBeNull()
  })

  it("reveals the full text in a popup when activated", () => {
    render(<InfoPopover text={TEXT} />)

    fireEvent.click(screen.getByRole("button", { name: TEXT }))

    expect(screen.getByRole("status").textContent).toContain(TEXT)
  })

  it("auto-hides the popup after three seconds", () => {
    vi.useFakeTimers()
    render(<InfoPopover text={TEXT} />)

    fireEvent.click(screen.getByRole("button", { name: TEXT }))
    expect(screen.queryByRole("status")).not.toBeNull()

    act(() => {
      vi.advanceTimersByTime(INFO_POPOVER_AUTO_HIDE_MS)
    })

    expect(screen.queryByRole("status")).toBeNull()
  })
})
