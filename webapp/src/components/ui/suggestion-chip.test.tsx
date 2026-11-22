import { describe, expect, it, vi } from "vitest"
import { fireEvent, render, screen } from "@testing-library/react"
import { SuggestionChip } from "./suggestion-chip"
import { hasClass } from "@/test/dom"

describe("SuggestionChip", () => {
  it("renders the shared pill styles", () => {
    render(<SuggestionChip label="Иван" onClick={vi.fn()} />)

    const chip = document.querySelector('[data-slot="suggestion-chip"]') as HTMLElement
    expect(chip).toBeTruthy()
    expect(hasClass(chip, "rounded-full")).toBe(true)
    expect(hasClass(chip, "border-border/70")).toBe(true)
    expect(hasClass(chip, "text-xs")).toBe(true)
    expect(hasClass(chip, "text-muted-foreground")).toBe(true)
    expect(chip.textContent).toBe("Иван")
  })

  it("reports a click", () => {
    const onClick = vi.fn()
    render(<SuggestionChip label="Иван" onClick={onClick} />)

    fireEvent.click(screen.getByRole("button", { name: "Иван" }))

    expect(onClick).toHaveBeenCalledTimes(1)
  })
})
