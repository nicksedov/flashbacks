import { describe, expect, it, vi } from "vitest"
import { fireEvent, render, screen } from "@testing-library/react"
import { SearchableSelect } from "./searchable-select"

const options = [
  { value: "minicpm-v", label: "MiniCPM-V (5.5 GB)" },
  { value: "llama3", label: "llama3 (4.7 GB)" },
]

function renderSelect(onValueChange = vi.fn()) {
  render(
    <SearchableSelect
      value=""
      onValueChange={onValueChange}
      options={options}
      placeholder="Select a model"
      searchPlaceholder="Search models…"
      noResultsText="No models match your search"
    />,
  )
  return onValueChange
}

function openDropdown() {
  fireEvent.click(screen.getByRole("combobox", { name: "Select a model" }))
  return screen.getByLabelText("Search models…")
}

describe("SearchableSelect", () => {
  it("renders the search field when opened", () => {
    renderSelect()

    const search = openDropdown()

    expect(search.getAttribute("placeholder")).toBe("Search models…")
    expect(screen.getAllByRole("option")).toHaveLength(2)
  })

  it("filters options by the typed substring", () => {
    renderSelect()

    fireEvent.change(openDropdown(), { target: { value: "mini" } })

    const listed = screen.getAllByRole("option")
    expect(listed).toHaveLength(1)
    expect(listed[0].textContent).toContain("MiniCPM-V")
  })

  it("shows the no-results message when nothing matches", () => {
    renderSelect()

    fireEvent.change(openDropdown(), { target: { value: "zzz" } })

    expect(screen.queryAllByRole("option")).toHaveLength(0)
    expect(screen.getByText("No models match your search")).not.toBeNull()
  })

  it("closes on Escape without reporting a change", () => {
    const onValueChange = renderSelect()

    fireEvent.keyDown(openDropdown(), { key: "Escape" })

    expect(screen.queryByRole("listbox")).toBeNull()
    expect(onValueChange).not.toHaveBeenCalled()
  })

  it("reports the chosen option value", () => {
    const onValueChange = renderSelect()

    openDropdown()
    fireEvent.click(screen.getByRole("option", { name: "llama3 (4.7 GB)" }))

    expect(onValueChange).toHaveBeenCalledWith("llama3")
    expect(screen.queryByRole("listbox")).toBeNull()
  })

  it("reopens with an empty query", () => {
    renderSelect()
    fireEvent.change(openDropdown(), { target: { value: "mini" } })

    fireEvent.keyDown(screen.getByLabelText("Search models…"), { key: "Escape" })
    const search = openDropdown()

    expect((search as HTMLInputElement).value).toBe("")
    expect(screen.getAllByRole("option")).toHaveLength(2)
  })
})
