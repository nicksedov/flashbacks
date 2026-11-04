import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, waitFor } from "@testing-library/react"
import { PeopleTab } from "./PeopleTab"

// Controls which locale the mocked translator resolves.
const languageState = vi.hoisted(() => ({ language: "en" as "en" | "ru" }))

const endpoints = vi.hoisted(() => ({
  fetchPeople: vi.fn(),
  fetchPersonPhotos: vi.fn(() => Promise.resolve({ images: [], totalImages: 0, currentPage: 1, pageSize: 50, totalPages: 1, hasNextPage: false })),
}))

vi.mock("@/api/endpoints", () => endpoints)

vi.mock("@/i18n", async () => {
  const en = await import("@/i18n/translations.en")
  const ru = await import("@/i18n/translations.ru")
  const translate = (key: string, vars?: Record<string, unknown>) => {
    const dictionary = (
      languageState.language === "ru" ? ru.translationsRu : en.translationsEn
    ) as Record<string, string>
    const text = dictionary[key] ?? key
    if (!vars) return text
    return text.replace(/\{\{?\s*(\w+)\s*\}?\}/g, (_match, name: string) => String(vars[name] ?? ""))
  }
  const useTranslation = () => ({ language: languageState.language, t: translate })
  return { useTranslation }
})

beforeEach(() => {
  languageState.language = "en"
  endpoints.fetchPeople.mockReset()
  endpoints.fetchPersonPhotos.mockClear()
})

describe("PeopleTab", () => {
  it("renders one tile per person with the person's name and cover thumbnail", async () => {
    endpoints.fetchPeople.mockResolvedValue({
      people: [
        { id: 1, name: "Alice", photoCount: 3, coverThumbnail: "/api/thumbnail?path=%2Fgallery%2Fa.jpg" },
        { id: 2, name: "Bob", photoCount: 5, coverThumbnail: "/api/thumbnail?path=%2Fgallery%2Fb.jpg" },
      ],
    })

    const { container } = render(<PeopleTab />)

    await waitFor(() => expect(container.querySelector('[data-slot="people-grid"]')).toBeTruthy())
    expect(container.querySelectorAll('[data-slot="person-tile"]')).toHaveLength(2)
    expect(screen.getByText("Alice")).toBeTruthy()
    expect(screen.getByText("Bob")).toBeTruthy()
    expect(container.querySelectorAll('[data-slot="person-tile"] img')).toHaveLength(2)
  })

  it("shows the empty-state title and hint when no people have been discovered", async () => {
    endpoints.fetchPeople.mockResolvedValue({ people: [] })

    render(<PeopleTab />)

    await waitFor(() => expect(screen.getByText("No people yet")).toBeTruthy())
    expect(screen.getByText("Run face recognition to group faces into people.")).toBeTruthy()
  })

  it("renders People labels in the selected language without falling back to a raw key", async () => {
    languageState.language = "ru"
    endpoints.fetchPeople.mockResolvedValue({ people: [] })

    render(<PeopleTab />)

    await waitFor(() => expect(screen.getByText("Люди")).toBeTruthy())
    expect(screen.getByText("Людей пока нет")).toBeTruthy()
  })
})
