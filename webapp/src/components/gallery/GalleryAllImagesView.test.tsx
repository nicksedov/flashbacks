import { describe, it, expect, vi } from "vitest"
import { render, screen } from "@testing-library/react"
import { GalleryAllImagesView } from "./GalleryAllImagesView"
import { hasClass } from "@/test/dom"

vi.mock("@/i18n", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

vi.mock("@/api/endpoints", () => ({
  moveFiles: vi.fn(),
}))

vi.mock("@/hooks/useGalleryImages", () => ({
  useGalleryImages: () => ({
    images: [],
    totalImages: 0,
    hasMore: false,
    isLoading: false,
    error: null,
    initialized: false,
    loadMore: vi.fn(),
    removeImage: vi.fn(),
  }),
}))

vi.mock("@/hooks/useGalleryFolders", () => ({
  useGalleryFolders: () => ({ folders: [] }),
}))

vi.mock("@/hooks/useIntersectionObserver", () => ({
  useIntersectionObserver: () => ({ current: null }),
}))

vi.mock("@/providers/useGallerySelection", () => ({
  useGallerySelection: () => ({ registerActions: vi.fn() }),
}))

vi.mock("@/components/gallery/GalleryImageGrid", () => ({
  GalleryImageGrid: () => null,
}))

vi.mock("@/components/gallery/BulkMoveDialog", () => ({
  BulkMoveDialog: () => null,
}))

describe("GalleryAllImagesView", () => {
  it("stacks the search and sort controls vertically on mobile", () => {
    render(<GalleryAllImagesView onImageClick={vi.fn()} />)

    // The toolbar row stacks below md and becomes a row from md up.
    const searchInput = screen.getByPlaceholderText("gallery.search.placeholder")
    const headerRow = searchInput.closest(".flex-col")
    expect(headerRow).not.toBeNull()
    expect(hasClass(headerRow, "md:flex-row")).toBe(true)

    // The search input is full-width on mobile, fixed width from sm up.
    expect(hasClass(searchInput, "w-full", "sm:w-70")).toBe(true)

    // The sort control keeps an accessible name with its label hidden below md.
    const sortButton = screen.getByRole("button", { name: "gallery.sortNewest" })
    expect(hasClass(sortButton.querySelector("span"), "hidden", "md:inline")).toBe(true)
  })

  it("collapses the image count to an info icon on mobile", () => {
    render(<GalleryAllImagesView onImageClick={vi.fn()} />)

    // The info icon carries the full count text and is shown only below md.
    const infoButton = screen.getByRole("button", { name: "gallery.imageCount" })
    expect(infoButton).toBeTruthy()
    expect(infoButton.closest(".md\\:hidden")).not.toBeNull()
  })

  // jsdom does not run CSS media queries or real layout, so "mobile" is
  // verified through the component's base (unprefixed) Tailwind classes: those
  // apply below the `md` breakpoint and are exactly what the browser uses on
  // mobile. The search field, the info icon and the sort icon must share one
  // non-wrapping flex row whose only shrinkable slot is the search field, so the
  // fixed-size icon controls can never overflow the viewport width.
  it("fits the search field, info and sort controls in one non-overflowing row on mobile", () => {
    render(<GalleryAllImagesView onImageClick={vi.fn()} />)

    // The search cell is the only element with `min-w-0` in this toolbar.
    const searchInput = screen.getByPlaceholderText("gallery.search.placeholder")
    const searchCell = searchInput.closest(".min-w-0")
    expect(searchCell).not.toBeNull()

    const row = searchCell!.parentElement
    expect(row).not.toBeNull()

    const infoButton = screen.getByRole("button", { name: "gallery.imageCount" })
    const sortButton = screen.getByRole("button", { name: "gallery.sortNewest" })
    const infoCell = infoButton.closest(".md\\:hidden")
    expect(infoCell).not.toBeNull()

    // -- "In one row" --------------------------------------------------------
    // A single horizontal flex row at the mobile breakpoint: `flex` +
    // `items-center` with neither wrapping nor vertical stacking.
    expect(hasClass(row, "flex", "items-center")).toBe(true)
    expect(hasClass(row, "flex-wrap")).toBe(false)
    expect(hasClass(row, "flex-wrap-reverse")).toBe(false)
    expect(hasClass(row, "flex-col")).toBe(false)

    // Search, info and sort are direct children of that same row.
    expect(searchCell!.parentElement).toBe(row)
    expect(infoCell!.parentElement).toBe(row)
    expect(sortButton.parentElement).toBe(row)

    // DOM order is search -> info -> sort (the info icon sits before sort).
    const cells = Array.from(row!.children)
    const searchIndex = cells.indexOf(searchCell!)
    const infoIndex = cells.indexOf(infoCell!)
    const sortIndex = cells.indexOf(sortButton)
    expect(searchIndex).toBeGreaterThanOrEqual(0)
    expect(infoIndex).toBeGreaterThan(searchIndex)
    expect(sortIndex).toBeGreaterThan(infoIndex)

    // The info icon is mobile-only; from md up the row shows search + sort.
    expect(hasClass(infoCell, "md:hidden")).toBe(true)

    // -- "Does not overflow the screen" -------------------------------------
    // The row is full-width and constrained to its parent, never wider.
    expect(hasClass(row, "w-full")).toBe(true)
    // The search cell is the flexible, shrinkable slot (`flex-1` + `min-w-0`):
    // it absorbs the free space and can shrink toward zero instead of pushing
    // the fixed-size info/sort icon controls off the right edge of the screen.
    expect(hasClass(searchCell, "flex-1", "min-w-0")).toBe(true)
    // The input fills its shrinkable cell; the fixed `sm:w-70` width only
    // applies from the `sm` breakpoint up, so it cannot force overflow on mobile.
    expect(hasClass(searchInput, "w-full", "sm:w-70")).toBe(true)
  })
})
