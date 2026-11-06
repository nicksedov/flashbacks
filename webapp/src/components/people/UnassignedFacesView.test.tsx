import { describe, it, expect, vi, beforeEach } from "vitest"
import { fireEvent, render, screen, waitFor } from "@testing-library/react"
import { I18nProvider } from "@/i18n/I18nProvider"
import { UnassignedFaceTile, UnassignedFacesView } from "./UnassignedFacesView"
import type { PersonDTO, UnassignedFaceDTO } from "@/types"

const endpoints = vi.hoisted(() => ({
  fetchUnassignedFaces: vi.fn(),
  assignFaceToPerson: vi.fn(),
  createPerson: vi.fn(),
}))

vi.mock("@/api/endpoints", () => endpoints)

const toast = vi.hoisted(() => ({ showToastError: vi.fn(), showToastSuccess: vi.fn() }))
vi.mock("@/lib/toast-utils", () => toast)

const anna: PersonDTO = { id: 7, name: "Anna", photoCount: 2, coverThumbnail: "" }

function unassignedFace(overrides: Partial<UnassignedFaceDTO> = {}): UnassignedFaceDTO {
  return {
    id: 1,
    personId: 0,
    path: "/gallery/group.jpg",
    fileName: "group.jpg",
    x: 50,
    y: 25,
    width: 40,
    height: 20,
    confidence: 0.9,
    ...overrides,
  }
}

function page(faces: UnassignedFaceDTO[], overrides: Record<string, unknown> = {}) {
  return {
    faces,
    totalFaces: faces.length,
    currentPage: 1,
    pageSize: 24,
    totalPages: 1,
    hasNextPage: false,
    ...overrides,
  }
}

function renderView(people: PersonDTO[] = [anna]) {
  const onPeopleChanged = vi.fn()
  const utils = render(
    <I18nProvider language="en">
      <UnassignedFacesView people={people} onPeopleChanged={onPeopleChanged} />
    </I18nProvider>,
  )
  return { ...utils, onPeopleChanged }
}

beforeEach(() => {
  endpoints.fetchUnassignedFaces.mockReset()
  endpoints.assignFaceToPerson.mockReset()
  endpoints.createPerson.mockReset()
  toast.showToastError.mockReset()
})

describe("UnassignedFaceTile", () => {
  it("draws the face box over the photo, scaled to the analysed frame", () => {
    render(
      <I18nProvider language="en">
        <UnassignedFaceTile
          face={unassignedFace({ x: 50, y: 25, width: 40, height: 20 })}
          selected={false}
          onToggle={vi.fn()}
          naturalDimensions={{ width: 200, height: 100 }}
        />
      </I18nProvider>,
    )

    const box = screen.getByTestId("unassigned-face-box")
    expect(box.style.left).toBe("25%")
    expect(box.style.top).toBe("25%")
    expect(box.style.width).toBe("20%")
    expect(box.style.height).toBe("20%")
  })

  it("reports the selected state on the button", () => {
    render(
      <I18nProvider language="en">
        <UnassignedFaceTile face={unassignedFace()} selected onToggle={vi.fn()} />
      </I18nProvider>,
    )

    expect(screen.getByRole("button").getAttribute("aria-pressed")).toBe("true")
  })
})

describe("UnassignedFacesView", () => {
  it("renders one entry per unassigned face", async () => {
    endpoints.fetchUnassignedFaces.mockResolvedValue(
      page([unassignedFace({ id: 1 }), unassignedFace({ id: 2, fileName: "other.jpg" })]),
    )

    const { container } = renderView()

    await waitFor(() =>
      expect(container.querySelectorAll('[data-slot="unassigned-face"]')).toHaveLength(2),
    )
    expect(endpoints.fetchUnassignedFaces).toHaveBeenCalledWith(1, 24)
  })

  it("shows the empty state when no face is unassigned", async () => {
    endpoints.fetchUnassignedFaces.mockResolvedValue(page([]))

    renderView()

    await waitFor(() => expect(screen.getByText("No unassigned faces")).toBeTruthy())
    expect(screen.queryByText("Every detected face belongs to a person.")).toBeTruthy()
  })

  it("pages forward to the next faces", async () => {
    endpoints.fetchUnassignedFaces
      .mockResolvedValueOnce(
        page([unassignedFace({ id: 1 })], { totalPages: 2, hasNextPage: true }),
      )
      .mockResolvedValueOnce(
        page([unassignedFace({ id: 2, fileName: "second.jpg" })], { currentPage: 2 }),
      )

    const { container } = renderView()
    await waitFor(() =>
      expect(container.querySelectorAll('[data-slot="unassigned-face"]')).toHaveLength(1),
    )
    expect(container.querySelector('[data-slot="unassigned-face"]')?.getAttribute("data-face-id")).toBe("1")

    fireEvent.click(container.querySelector('[data-slot="unassigned-next"]') as HTMLElement)

    await waitFor(() =>
      expect(container.querySelector('[data-slot="unassigned-face"]')?.getAttribute("data-face-id")).toBe("2"),
    )
    expect(endpoints.fetchUnassignedFaces).toHaveBeenLastCalledWith(2, 24)
  })

  it("attaches a selected face to an existing person", async () => {
    endpoints.fetchUnassignedFaces
      .mockResolvedValueOnce(page([unassignedFace({ id: 5 })]))
      .mockResolvedValueOnce(page([]))
    endpoints.assignFaceToPerson.mockResolvedValue(unassignedFace({ id: 5, personId: anna.id }))

    const { container, onPeopleChanged } = renderView()
    await waitFor(() =>
      expect(container.querySelectorAll('[data-slot="unassigned-face"]')).toHaveLength(1),
    )

    fireEvent.click(container.querySelector('[data-slot="unassigned-face"]') as HTMLElement)
    fireEvent.click(container.querySelector('[data-slot="unassigned-attach"]') as HTMLElement)

    await waitFor(() => expect(screen.getByRole("dialog")).toBeTruthy())
    fireEvent.click(screen.getByText("Anna"))

    await waitFor(() => expect(endpoints.assignFaceToPerson).toHaveBeenCalledWith(5, anna.id))
    await waitFor(() => expect(onPeopleChanged).toHaveBeenCalled())
    await waitFor(() => expect(screen.getByText("No unassigned faces")).toBeTruthy())
  })

  it("searches the whole catalog in the picker", async () => {
    endpoints.fetchUnassignedFaces.mockResolvedValue(page([unassignedFace({ id: 5 })]))

    const { container } = renderView([
      anna,
      { id: 9, name: "Василиса", photoCount: 1, coverThumbnail: "" },
    ])
    await waitFor(() =>
      expect(container.querySelectorAll('[data-slot="unassigned-face"]')).toHaveLength(1),
    )

    fireEvent.click(container.querySelector('[data-slot="unassigned-face"]') as HTMLElement)
    fireEvent.click(container.querySelector('[data-slot="unassigned-attach"]') as HTMLElement)
    await waitFor(() => expect(screen.getByRole("dialog")).toBeTruthy())

    fireEvent.change(screen.getByPlaceholderText("Search people"), { target: { value: "вас" } })

    await waitFor(() => expect(screen.queryByText("Anna")).toBeNull())
    expect(screen.getByText("Василиса")).toBeTruthy()
  })

  it("creates a named person from the selected faces", async () => {
    endpoints.fetchUnassignedFaces
      .mockResolvedValueOnce(page([unassignedFace({ id: 5 }), unassignedFace({ id: 6 })]))
      .mockResolvedValueOnce(page([]))
    endpoints.createPerson.mockResolvedValue({
      id: 12,
      name: "Василиса",
      photoCount: 1,
      coverThumbnail: "",
    })

    const { container, onPeopleChanged } = renderView()
    await waitFor(() =>
      expect(container.querySelectorAll('[data-slot="unassigned-face"]')).toHaveLength(2),
    )

    fireEvent.click(container.querySelectorAll('[data-slot="unassigned-face"]')[0])
    fireEvent.click(container.querySelectorAll('[data-slot="unassigned-face"]')[1])
    fireEvent.click(container.querySelector('[data-slot="unassigned-create"]') as HTMLElement)

    await waitFor(() => expect(screen.getByRole("dialog")).toBeTruthy())
    fireEvent.change(screen.getByLabelText("Name (optional)"), {
      target: { value: "Василиса" },
    })
    fireEvent.click(screen.getByText("Create", { selector: '[data-slot="create-confirm"]' }))

    await waitFor(() =>
      expect(endpoints.createPerson).toHaveBeenCalledWith({
        name: "Василиса",
        faceIds: [5, 6],
      }),
    )
    await waitFor(() => expect(onPeopleChanged).toHaveBeenCalled())
    await waitFor(() => expect(screen.getByText("No unassigned faces")).toBeTruthy())
  })

  it("reports a failed attach and keeps the face in the view", async () => {
    endpoints.fetchUnassignedFaces.mockResolvedValue(page([unassignedFace({ id: 5 })]))
    endpoints.assignFaceToPerson.mockRejectedValue(new Error("boom"))

    const { container } = renderView()
    await waitFor(() =>
      expect(container.querySelectorAll('[data-slot="unassigned-face"]')).toHaveLength(1),
    )

    fireEvent.click(container.querySelector('[data-slot="unassigned-face"]') as HTMLElement)
    fireEvent.click(container.querySelector('[data-slot="unassigned-attach"]') as HTMLElement)
    await waitFor(() => expect(screen.getByRole("dialog")).toBeTruthy())
    fireEvent.click(screen.getByText("Anna"))

    await waitFor(() => expect(toast.showToastError).toHaveBeenCalledWith("Could not update the catalog"))
    expect(container.querySelectorAll('[data-slot="unassigned-face"]')).toHaveLength(1)
  })
})
