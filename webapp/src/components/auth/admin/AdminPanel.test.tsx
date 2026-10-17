import { describe, it, expect, vi } from "vitest"
import { render } from "@testing-library/react"
import { AdminPanel } from "./AdminPanel"
import { hasClass } from "@/test/dom"

vi.mock("@/i18n", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

vi.mock("@/providers/useAuth", () => ({
  useAuth: () => ({
    user: { id: 1, login: "admin", displayName: "Ada Lovelace", role: "admin", isActive: true, accountStatus: "active" },
  }),
}))

vi.mock("./useAdminUsers", () => ({
  useAdminUsers: () => ({
    users: [],
    pendingUsers: [],
    isLoading: false,
    isPendingLoading: false,
    approve: vi.fn(),
    reject: vi.fn(),
    removeUser: vi.fn(),
    toggleActive: vi.fn(),
    refresh: vi.fn(),
  }),
}))

describe("AdminPanel", () => {
  it("inherits the shared page width instead of adding its own max-width or centring wrapper", () => {
    const { container } = render(<AdminPanel />)

    const page = container.firstElementChild
    expect(hasClass(page, "space-y-6")).toBe(true)
    // The app shell owns the page width: no per-tab max-width or centring wrapper.
    expect(hasClass(page, "mx-auto")).toBe(false)
    expect(page?.className ?? "").not.toMatch(/(^|\s)max-w-/)
  })
})
