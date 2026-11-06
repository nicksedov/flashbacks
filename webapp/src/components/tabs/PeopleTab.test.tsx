import { describe, it, expect, vi, beforeEach } from "vitest"
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react"
import { PeopleTab } from "./PeopleTab"

// The lightbox owns its own data fetching; the People tab only has to hand it
// the clicked photo and the initial mode, so replace it with a probe.
vi.mock("@/components/gallery/UnifiedLightbox", () => ({
  UnifiedLightbox: (props: { imagePath: string | null; initialMode?: string; onClose: () => void }) =>
    props.imagePath ? (
      <div data-slot="lightbox" data-path={props.imagePath} data-mode={props.initialMode}>
        <button type="button" onClick={props.onClose}>
          close-lightbox
        </button>
      </div>
    ) : null,
}))

// Controls which locale the mocked translator resolves.
const languageState = vi.hoisted(() => ({ language: "en" as "en" | "ru" }))

const THUMBNAIL_DATA_URL = "data:image/webp;base64,UklGRhIAAABXRUJQVlA4"

const endpoints = vi.hoisted(() => ({
  fetchPeople: vi.fn(),
  fetchPersonPhotos: vi.fn(),
  fetchThumbnail: vi.fn(),
  renamePerson: vi.fn(),
  mergePeople: vi.fn(),
  createPerson: vi.fn(),
  disbandPerson: vi.fn(),
  assignFaceToPerson: vi.fn(),
  fetchUnassignedFaces: vi.fn(),
}))

vi.mock("@/api/endpoints", () => endpoints)

const toast = vi.hoisted(() => ({ showToastError: vi.fn(), showToastSuccess: vi.fn() }))

vi.mock("@/lib/toast-utils", () => toast)

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
  endpoints.renamePerson.mockReset()
  endpoints.mergePeople.mockReset()
  endpoints.createPerson.mockReset()
  endpoints.disbandPerson.mockReset()
  endpoints.assignFaceToPerson.mockReset()
  endpoints.fetchUnassignedFaces.mockReset()
  toast.showToastError.mockReset()
  toast.showToastSuccess.mockReset()
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

  it("opens a person's photo in the People mode of the unified lightbox and closes it", async () => {
    endpoints.fetchPeople.mockResolvedValue({
      people: [
        { id: 1, name: "Alice", photoCount: 1, coverThumbnail: "/api/thumbnail?path=%2Fgallery%2Fa.jpg" },
      ],
    })
    endpoints.fetchPersonPhotos.mockResolvedValue({
      images: [
        { id: 7, path: "/gallery/a.jpg", fileName: "a.jpg" },
        { id: 8, path: "/gallery/b.jpg", fileName: "b.jpg" },
      ],
      totalImages: 2,
      currentPage: 1,
      pageSize: 50,
      totalPages: 1,
      hasNextPage: false,
    })

    const { container } = render(<PeopleTab />)

    await waitFor(() => expect(container.querySelector('[data-slot="person-tile"]')).toBeTruthy())
    expect(container.querySelector('[data-slot="lightbox"]')).toBeNull()
    fireEvent.click(container.querySelector('[data-slot="person-tile"]') as HTMLElement)

    await waitFor(() => expect(container.querySelectorAll('[data-slot="person-photo"]')).toHaveLength(2))
    fireEvent.click(container.querySelectorAll('[data-slot="person-photo"]')[1] as HTMLElement)

    await waitFor(() => expect(container.querySelector('[data-slot="lightbox"]')).toBeTruthy())
    const lightbox = container.querySelector('[data-slot="lightbox"]') as HTMLElement
    expect(lightbox.getAttribute("data-mode")).toBe("people")
    expect(lightbox.getAttribute("data-path")).toBe("/gallery/b.jpg")

    fireEvent.click(within(lightbox).getByText("close-lightbox"))

    await waitFor(() => expect(container.querySelector('[data-slot="lightbox"]')).toBeNull())
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

  it("filters people by name case-insensitively and reports no matches", async () => {
    endpoints.fetchPeople.mockResolvedValue({
      people: [
        { id: 1, name: "Anna", photoCount: 2, coverThumbnail: "" },
        { id: 2, name: "Василиса", photoCount: 4, coverThumbnail: "" },
      ],
    })

    const { container } = render(<PeopleTab />)
    await waitFor(() => expect(container.querySelectorAll('[data-slot="person-tile"]')).toHaveLength(2))

    fireEvent.change(container.querySelector('[data-slot="people-search"]') as HTMLElement, {
      target: { value: "ВАС" },
    })
    await waitFor(() => expect(container.querySelectorAll('[data-slot="person-tile"]')).toHaveLength(1))
    expect(screen.getByText("Василиса")).toBeTruthy()

    fireEvent.change(container.querySelector('[data-slot="people-search"]') as HTMLElement, {
      target: { value: "zzz" },
    })
    await waitFor(() => expect(container.querySelector('[data-slot="people-no-matches"]')).toBeTruthy())
    expect(screen.getByText("No people match “zzz”")).toBeTruthy()
  })

  it("enables merge only for two selected people and merges them into one tile", async () => {
    endpoints.fetchPeople.mockResolvedValue({
      people: [
        { id: 1, name: "Anna", photoCount: 1, coverThumbnail: "" },
        { id: 2, name: "Bob", photoCount: 1, coverThumbnail: "" },
      ],
    })
    endpoints.fetchPersonPhotos.mockResolvedValue({
      images: [{ id: 1, path: "/gallery/shared.jpg", fileName: "shared.jpg" }],
      totalImages: 1,
      currentPage: 1,
      pageSize: 200,
      totalPages: 1,
      hasNextPage: false,
    })
    endpoints.mergePeople.mockResolvedValue({
      id: 1,
      name: "Василиса",
      photoCount: 1,
      coverThumbnail: "",
    })

    const { container } = render(<PeopleTab />)
    await waitFor(() => expect(container.querySelectorAll('[data-slot="person-tile"]')).toHaveLength(2))

    const mergeButton = container.querySelector('[data-slot="people-merge"]') as HTMLButtonElement
    expect(mergeButton.disabled).toBe(true)

    fireEvent.click(container.querySelectorAll('[data-slot="person-select"]')[0])
    expect(mergeButton.disabled).toBe(true)

    fireEvent.click(container.querySelectorAll('[data-slot="person-select"]')[1])
    expect(mergeButton.disabled).toBe(false)

    fireEvent.click(mergeButton)
    await waitFor(() => expect(screen.getByRole("dialog")).toBeTruthy())
    // Both people share one photo, so the preview must count it once.
    await waitFor(() => expect(screen.getByText("Result: Anna, 1 photos")).toBeTruthy())

    fireEvent.change(screen.getByLabelText("Name (optional)"), {
      target: { value: "Василиса" },
    })
    fireEvent.click(screen.getByText("Merge", { selector: '[data-slot="merge-confirm"]' }))

    await waitFor(() =>
      expect(endpoints.mergePeople).toHaveBeenCalledWith({
        targetPersonId: 1,
        sourcePersonIds: [2],
        name: "Василиса",
      }),
    )
    await waitFor(() => expect(container.querySelectorAll('[data-slot="person-tile"]')).toHaveLength(1))
    expect(screen.getByText("Василиса")).toBeTruthy()
  })

  it("folds one cluster into a chosen known person through the picker", async () => {
    endpoints.fetchPeople.mockResolvedValue({
      people: [
        { id: 5, name: "", photoCount: 2, coverThumbnail: "" },
        { id: 2, name: "Василиса", photoCount: 4, coverThumbnail: "" },
      ],
    })
    endpoints.mergePeople.mockResolvedValue({
      id: 2,
      name: "Василиса",
      photoCount: 6,
      coverThumbnail: "",
    })

    const { container } = render(<PeopleTab />)
    await waitFor(() => expect(container.querySelectorAll('[data-slot="person-tile"]')).toHaveLength(2))

    fireEvent.click(container.querySelectorAll('[data-slot="person-merge-into"]')[0] as HTMLElement)
    await waitFor(() => expect(screen.getByRole("dialog")).toBeTruthy())
    expect(screen.getByText("Add “Person 5” to a person")).toBeTruthy()

    // The cluster being folded in must not be offered as its own target.
    const picks = document.querySelectorAll('[data-slot="picker-person"]')
    expect(picks).toHaveLength(1)
    expect(within(picks[0] as HTMLElement).getByText("Василиса")).toBeTruthy()

    fireEvent.click(picks[0] as HTMLElement)

    await waitFor(() =>
      expect(endpoints.mergePeople).toHaveBeenCalledWith({
        targetPersonId: 2,
        sourcePersonIds: [5],
      }),
    )
    await waitFor(() => expect(container.querySelectorAll('[data-slot="person-tile"]')).toHaveLength(1))
    expect(screen.queryByText("Person 5")).toBeNull()
    expect(toast.showToastSuccess).toHaveBeenCalledWith("Added to Василиса")
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull())
  })

  it("renames a tile in place", async () => {
    endpoints.fetchPeople.mockResolvedValue({
      people: [{ id: 1, name: "Anna", photoCount: 1, coverThumbnail: "" }],
    })
    endpoints.renamePerson.mockResolvedValue({
      id: 1,
      name: "Василиса",
      photoCount: 1,
      coverThumbnail: "",
    })

    const { container } = render(<PeopleTab />)
    await waitFor(() => expect(container.querySelector('[data-slot="person-tile"]')).toBeTruthy())

    fireEvent.click(container.querySelector('[data-slot="person-rename"]') as HTMLElement)
    await waitFor(() => expect(screen.getByRole("dialog")).toBeTruthy())
    fireEvent.change(screen.getByLabelText("Name"), { target: { value: "Василиса" } })
    fireEvent.click(screen.getByText("Save", { selector: '[data-slot="rename-save"]' }))

    await waitFor(() => expect(screen.getByText("Василиса")).toBeTruthy())
    expect(endpoints.renamePerson).toHaveBeenCalledWith(1, "Василиса")
  })

  it("sends no rename request when the dialog is cancelled", async () => {
    endpoints.fetchPeople.mockResolvedValue({
      people: [{ id: 1, name: "Anna", photoCount: 1, coverThumbnail: "" }],
    })

    const { container } = render(<PeopleTab />)
    await waitFor(() => expect(container.querySelector('[data-slot="person-tile"]')).toBeTruthy())

    fireEvent.click(container.querySelector('[data-slot="person-rename"]') as HTMLElement)
    await waitFor(() => expect(screen.getByRole("dialog")).toBeTruthy())
    fireEvent.change(screen.getByLabelText("Name"), { target: { value: "Василиса" } })
    fireEvent.click(screen.getByText("Cancel", { selector: '[data-slot="rename-cancel"]' }))

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull())
    expect(endpoints.renamePerson).not.toHaveBeenCalled()
    expect(screen.getByText("Anna")).toBeTruthy()
  })

  it("removes a disbanded tile only after confirmation", async () => {
    endpoints.fetchPeople.mockResolvedValue({
      people: [
        { id: 1, name: "Anna", photoCount: 1, coverThumbnail: "" },
        { id: 2, name: "Bob", photoCount: 1, coverThumbnail: "" },
      ],
    })
    endpoints.disbandPerson.mockResolvedValue({ message: "Person disbanded; photos kept" })

    const { container } = render(<PeopleTab />)
    await waitFor(() => expect(container.querySelectorAll('[data-slot="person-tile"]')).toHaveLength(2))

    fireEvent.click(container.querySelector('[data-slot="person-disband"]') as HTMLElement)
    await waitFor(() => expect(screen.getByRole("dialog")).toBeTruthy())
    expect(
      screen.getByText(
        "The photos stay. The faces return to unassigned and the person is removed.",
      ),
    ).toBeTruthy()

    const dialog = screen.getByRole("dialog")
    fireEvent.click(within(dialog).getByText("Disband"))

    await waitFor(() => expect(endpoints.disbandPerson).toHaveBeenCalledWith(1))
    await waitFor(() => expect(container.querySelectorAll('[data-slot="person-tile"]')).toHaveLength(1))
    expect(screen.queryByText("Anna")).toBeNull()
  })

  it("keeps the person when disbanding is cancelled", async () => {
    endpoints.fetchPeople.mockResolvedValue({
      people: [{ id: 1, name: "Anna", photoCount: 1, coverThumbnail: "" }],
    })

    const { container } = render(<PeopleTab />)
    await waitFor(() => expect(container.querySelector('[data-slot="person-tile"]')).toBeTruthy())

    fireEvent.click(container.querySelector('[data-slot="person-disband"]') as HTMLElement)
    await waitFor(() => expect(screen.getByRole("dialog")).toBeTruthy())
    const dialog = screen.getByRole("dialog")
    fireEvent.click(within(dialog).getByText("Cancel"))

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull())
    expect(endpoints.disbandPerson).not.toHaveBeenCalled()
    expect(container.querySelectorAll('[data-slot="person-tile"]')).toHaveLength(1)
  })

  it("matches unnamed people by their localized placeholder", async () => {
    endpoints.fetchPeople.mockResolvedValue({
      people: [
        { id: 42, name: "", photoCount: 1, coverThumbnail: "" },
        { id: 1, name: "Anna", photoCount: 1, coverThumbnail: "" },
      ],
    })

    const { container } = render(<PeopleTab />)
    await waitFor(() => expect(container.querySelectorAll('[data-slot="person-tile"]')).toHaveLength(2))
    expect(screen.getByText("Person 42")).toBeTruthy()

    fireEvent.change(container.querySelector('[data-slot="people-search"]') as HTMLElement, {
      target: { value: "person 42" },
    })
    await waitFor(() => expect(container.querySelectorAll('[data-slot="person-tile"]')).toHaveLength(1))
    expect(screen.getByText("Person 42")).toBeTruthy()
    expect(screen.queryByText("Anna")).toBeNull()
  })

  it("reports a failed rename and keeps the previous grid state", async () => {
    endpoints.fetchPeople.mockResolvedValue({
      people: [{ id: 1, name: "Anna", photoCount: 1, coverThumbnail: "" }],
    })
    endpoints.renamePerson.mockRejectedValue(new Error("boom"))

    const { container } = render(<PeopleTab />)
    await waitFor(() => expect(container.querySelector('[data-slot="person-tile"]')).toBeTruthy())

    fireEvent.click(container.querySelector('[data-slot="person-rename"]') as HTMLElement)
    await waitFor(() => expect(screen.getByRole("dialog")).toBeTruthy())
    fireEvent.change(screen.getByLabelText("Name"), { target: { value: "Василиса" } })
    fireEvent.click(screen.getByText("Save", { selector: '[data-slot="rename-save"]' }))

    await waitFor(() => expect(toast.showToastError).toHaveBeenCalledWith("Could not update the catalog"))
    expect(screen.getByText("Anna")).toBeTruthy()
  })
})
