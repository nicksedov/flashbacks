import type { ReactNode } from "react"
import { describe, it, expect, vi } from "vitest"
import { render, screen, fireEvent } from "@testing-library/react"
import { I18nProvider } from "@/i18n/I18nProvider"
import { TrashTile } from "./TrashTile"
import { TrashTileGrid } from "./TrashTileGrid"
import { hasClass } from "@/test/dom"
import type { TrashDateGroup, TrashItemDTO } from "@/types"

function makeItem(overrides: Partial<TrashItemDTO> = {}): TrashItemDTO {
  return {
    id: 1,
    fileName: "photo.jpg",
    trashPath: "/trash/photo.jpg",
    originalPath: "/gallery/2026/photo.jpg",
    size: 2048,
    sizeHuman: "2 KB",
    deletedAt: "2026-10-01T10:20:30+03:00",
    thumbnail: "data:image/webp;base64,AAAA",
    ...overrides,
  }
}

function renderEn(node: ReactNode) {
  return render(<I18nProvider language="en">{node}</I18nProvider>)
}

describe("TrashTile", () => {
  it("shows exactly the View and Restore actions in the hover overlay", () => {
    renderEn(<TrashTile item={makeItem()} onView={vi.fn()} onRestore={vi.fn()} />)

    const view = screen.getByRole("button", { name: "View" })
    expect(screen.getByRole("button", { name: "Restore" })).toBeTruthy()
    // The tile frame itself is a role=button; the overlay owns exactly two actions.
    const overlay = view.parentElement?.parentElement as HTMLElement
    expect(overlay.querySelectorAll("button")).toHaveLength(2)
  })

  it("opens the lightbox through the View action without restoring", () => {
    const onView = vi.fn()
    const onRestore = vi.fn()
    const item = makeItem()

    renderEn(<TrashTile item={item} onView={onView} onRestore={onRestore} />)
    fireEvent.click(screen.getByRole("button", { name: "View" }))

    expect(onView).toHaveBeenCalledWith(item)
    expect(onRestore).not.toHaveBeenCalled()
  })

  it("restores the file through the Restore action without opening the lightbox", () => {
    const onView = vi.fn()
    const onRestore = vi.fn()
    const item = makeItem()

    renderEn(<TrashTile item={item} onView={onView} onRestore={onRestore} />)
    fireEvent.click(screen.getByRole("button", { name: "Restore" }))

    expect(onRestore).toHaveBeenCalledWith(item)
    expect(onView).not.toHaveBeenCalled()
  })

  it("renders the image thumbnail when the server generated one", () => {
    renderEn(<TrashTile item={makeItem()} onView={vi.fn()} onRestore={vi.fn()} />)

    const image = screen.getByAltText("photo.jpg") as HTMLImageElement
    expect(image.tagName).toBe("IMG")
    expect(image.getAttribute("src")).toBe("data:image/webp;base64,AAAA")
  })

  it("renders the shared placeholder when the thumbnail is empty", () => {
    renderEn(
      <TrashTile item={makeItem({ thumbnail: "" })} onView={vi.fn()} onRestore={vi.fn()} />
    )

    expect(screen.queryByAltText("photo.jpg")).toBeNull()
    expect(screen.getByText("No preview")).toBeTruthy()
  })
})

describe("TrashTileGrid", () => {
  const groups: TrashDateGroup[] = [
    {
      date: "2026-10-01",
      label: "Wednesday, October 1, 2026",
      itemCount: 2,
      items: [makeItem(), makeItem({ id: 2, fileName: "second.jpg" })],
    },
    {
      date: "2026-09-30",
      label: "Tuesday, September 30, 2026",
      itemCount: 1,
      items: [makeItem({ id: 3, fileName: "third.jpg" })],
    },
  ]

  it("groups tiles by deletion date with the label and item count", () => {
    renderEn(<TrashTileGrid groups={groups} onView={vi.fn()} onRestore={vi.fn()} />)

    expect(screen.getByText("Wednesday, October 1, 2026")).toBeTruthy()
    expect(screen.getByText("2 item(s)")).toBeTruthy()
    expect(screen.getByText("Tuesday, September 30, 2026")).toBeTruthy()
    expect(screen.getByText("1 item(s)")).toBeTruthy()
    expect(screen.getByText("photo.jpg")).toBeTruthy()
    expect(screen.getByText("second.jpg")).toBeTruthy()
    expect(screen.getByText("third.jpg")).toBeTruthy()
  })

  it("uses the same responsive grid breakpoints as the gallery", () => {
    const { container } = renderEn(
      <TrashTileGrid groups={groups} onView={vi.fn()} onRestore={vi.fn()} />
    )

    const grids = container.querySelectorAll("div.grid")
    expect(grids.length).toBe(2)
    for (const grid of grids) {
      expect(hasClass(grid, "grid-cols-3", "sm:grid-cols-4", "md:grid-cols-6", "lg:grid-cols-7", "xl:grid-cols-8")).toBe(true)
    }
  })
})
