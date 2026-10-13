import { describe, it, expect, vi } from "vitest"
import { render, screen } from "@testing-library/react"
import { Header } from "./Header"
import { hasClass } from "@/test/dom"

vi.mock("@/i18n", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

vi.mock("@/api/endpoints", () => ({
  getAvatarUrl: (id: number) => `/avatar/${id}`,
}))

vi.mock("@/providers/useAuth", () => ({
  useAuth: () => ({
    user: { id: 1, displayName: "Ada Lovelace", role: "admin", hasAvatar: false },
    logout: vi.fn(),
  }),
}))

vi.mock("@/providers/useGallerySelection", () => ({
  useGallerySelection: () => ({
    isActive: false,
    selectedCount: 0,
    clearSelection: vi.fn(),
    moveSelected: vi.fn(),
    deleteSelected: vi.fn(),
  }),
}))

describe("Header", () => {
  it("hides the logout text label below md while keeping the icon and accessible name", () => {
    render(<Header onTabChange={vi.fn()} onMobileMenuToggle={vi.fn()} />)

    const logout = screen.getByRole("button", { name: "adminPanel.logout" })

    // Accessible name is retained even when only the icon shows.
    expect(logout).toBeTruthy()
    // The icon is always rendered.
    expect(logout.querySelector("svg")).not.toBeNull()
    // The text label is hidden below md and only shown from md up.
    expect(hasClass(logout.querySelector("span"), "hidden", "md:inline")).toBe(true)
  })

  it("hides the profile name and role below md while keeping an accessible name", () => {
    render(<Header onTabChange={vi.fn()} onMobileMenuToggle={vi.fn()} />)

    const profile = screen.getByRole("button", { name: "Ada Lovelace" })
    expect(profile).toBeTruthy()
    expect(hasClass(profile.querySelector("span"), "hidden", "md:flex")).toBe(true)
  })
})
