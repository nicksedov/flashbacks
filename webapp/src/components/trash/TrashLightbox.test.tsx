import { describe, it, expect, vi } from "vitest"
import { render, screen, fireEvent } from "@testing-library/react"
import { TrashLightbox } from "./TrashLightbox"
import { hasClass } from "@/test/dom"
import type { TrashItemDTO } from "@/types"

vi.mock("@/i18n", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

vi.mock("@/api/endpoints", () => ({
  buildTrashImageUrl: (path: string) => `/api/trash/image/${path}`,
}))

const item: TrashItemDTO = {
  id: 1,
  fileName: "a-very-long-trash-file-name-that-should-truncate-on-mobile.jpg",
  trashPath: "trash/x.jpg",
  originalPath: "/photos/2024/some/deeply/nested/location/x.jpg",
  size: 1234,
  sizeHuman: "1.2 KB",
  deletedAt: "2024-01-02T03:04:05Z",
  thumbnail: "thumb.jpg",
}

/** Walk ancestors checking for a class token (Tailwind `:` classes break CSS selectors). */
function hasAncestorWithClass(el: Element | null, className: string): boolean {
  let node: Element | null = el
  while (node) {
    if (node.classList.contains(className)) return true
    node = node.parentElement
  }
  return false
}

describe("TrashLightbox", () => {
  it("shows the image with the metadata panel collapsed on mobile and expands via the toggle", () => {
    render(<TrashLightbox item={item} onOpenChange={vi.fn()} />)

    expect(screen.getByAltText(item.fileName)).toBeTruthy()

    const panel = screen.getByTestId("mobile-detail-panel")
    expect(panel.getAttribute("data-open")).toBe("false")

    fireEvent.click(screen.getByTestId("mobile-detail-toggle"))
    expect(panel.getAttribute("data-open")).toBe("true")
  })

  it("truncates the title and keeps a 44px chrome target below md", () => {
    render(<TrashLightbox item={item} onOpenChange={vi.fn()} />)

    const title = screen.getByRole("heading", { name: item.fileName })
    expect(hasClass(title, "truncate")).toBe(true)
    expect(hasAncestorWithClass(title, "min-w-0")).toBe(true)

    const close = screen.getByRole("button", { name: "Close" })
    expect(hasClass(close, "h-11", "w-11")).toBe(true)
  })
})
