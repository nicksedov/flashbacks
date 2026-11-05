import { describe, it, expect } from "vitest"
import { render, screen } from "@testing-library/react"
import { I18nProvider } from "@/i18n/I18nProvider"
import { PeoplePanel } from "./PeoplePanel"
import type { ImagePerson } from "@/hooks/useImageFaces"

const anna: ImagePerson = { number: 1, personId: 7, name: "Anna" }
const unnamed: ImagePerson = { number: 2, personId: 9, name: "" }

interface RenderOptions {
  people?: ImagePerson[]
  loading?: boolean
  error?: boolean
  language?: "en" | "ru"
}

function renderPanel({
  people = [anna, unnamed],
  loading = false,
  error = false,
  language = "en",
}: RenderOptions = {}) {
  return render(
    <I18nProvider language={language}>
      <PeoplePanel people={people} loading={loading} error={error} />
    </I18nProvider>
  )
}

describe("PeoplePanel", () => {
  it("lists each person once with the number drawn on the image", () => {
    renderPanel()

    expect(screen.getAllByTestId("people-list-item")).toHaveLength(2)
    const numbers = screen
      .getAllByTestId("people-list-item")
      .map((item) => item.textContent?.charAt(0))
    expect(numbers).toEqual(["1", "2"])
  })

  it("renders the placeholder for an unnamed person", () => {
    renderPanel({ people: [unnamed] })

    expect(screen.getByTestId("people-list-item").textContent).toContain("Person 9")
  })

  it("shows the empty state when the image has no faces", () => {
    renderPanel({ people: [] })

    expect(screen.getByText("No faces on this photo")).toBeTruthy()
    expect(screen.queryByTestId("people-list")).toBeNull()
  })

  it("renders Russian labels under the Russian interface", () => {
    renderPanel({ people: [unnamed], language: "ru" })

    expect(screen.getByTestId("people-list-item").textContent).toContain("Человек 9")
    expect(screen.queryByText("people.unnamed")).toBeNull()
  })

  it("reports a failed face lookup", () => {
    renderPanel({ people: [], error: true })

    expect(screen.getByText("Could not load the faces of this photo")).toBeTruthy()
  })
})
