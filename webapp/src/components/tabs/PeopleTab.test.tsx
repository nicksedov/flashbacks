import { describe, it, expect, vi, beforeEach } from "vitest"
import { fireEvent, render, screen, waitFor } from "@testing-library/react"
import { PeopleTab } from "./PeopleTab"

// Controls which locale the mocked translator resolves.
const languageState = vi.hoisted(() => ({ language: "en" as "en" | "ru" }))

const THUMBNAIL_DATA_URL = "data:image/webp;base64,UklGRhIAAABXRUJQVlA4"

const endpoints = vi.hoisted(() => ({
  fetchPeople: vi.fn(),
  fetchPersonPhotos: vi.fn(),
  fetchThumbnail: vi.fn(),
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
  endpoints.fetchPersonPhotos.mockReset()
  endpoints.fetchPersonPhotos.mockResolvedValue({
    images: [],
    totalImages: 0,
    currentPage: 1,
    pageSize: 50,
    totalPages: 1,
    hasNextPage: false,
  })
  endpoints.fetchThumbnail.mockReset()
  endpoints.fetchThumbnail.mockResolvedValue({ thumbnail: THUMBNAIL_DATA_URL })
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
    await waitFor(() => expect(container.querySelectorAll('[data-slot="person-tile"] img')).toHaveLength(2))
    expect(endpoints.fetchThumbnail).toHaveBeenCalledWith("/gallery/a.jpg")
    expect(endpoints.fetchThumbnail).toHaveBeenCalledWith("/gallery/b.jpg")
  })

  it("renders cover thumbnails from the JSON thumbnail API as image data URLs", async () => {
    endpoints.fetchPeople.mockResolvedValue({
      people: [
        { id: 1, name: "Alice", photoCount: 3, coverThumbnail: "/api/thumbnail?path=%2Fgallery%2Fa.jpg" },
      ],
    })

    const { container } = render(<PeopleTab />)

    // `/api/thumbnail` answers with JSON, so an <img> must never point at it.
    await waitFor(() => expect(container.querySelector('[data-slot="person-tile"] img')).toBeTruthy())
    const cover = container.querySelector('[data-slot="person-tile"] img') as HTMLImageElement
    expect(cover.getAttribute("src")).toBe(THUMBNAIL_DATA_URL)
  })

  it("renders a person's photos through the thumbnail API", async () => {
    endpoints.fetchPeople.mockResolvedValue({
      people: [
        { id: 1, name: "Alice", photoCount: 1, coverThumbnail: "/api/thumbnail?path=%2Fgallery%2Fa.jpg" },
      ],
    })
    endpoints.fetchPersonPhotos.mockResolvedValue({
      images: [{ id: 7, path: "/gallery/a.jpg", fileName: "a.jpg" }],
      totalImages: 1,
      currentPage: 1,
      pageSize: 50,
      totalPages: 1,
      hasNextPage: false,
    })

    const { container } = render(<PeopleTab />)

    await waitFor(() => expect(container.querySelector('[data-slot="person-tile"]')).toBeTruthy())
    fireEvent.click(container.querySelector('[data-slot="person-tile"]') as HTMLElement)

    await waitFor(() => expect(container.querySelector('[data-slot="people-detail"] img')).toBeTruthy())
    expect(endpoints.fetchPersonPhotos).toHaveBeenCalledWith(1, 1, 200)
    expect((container.querySelector('[data-slot="people-detail"] img') as HTMLImageElement).getAttribute("src")).toBe(THUMBNAIL_DATA_URL)
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
