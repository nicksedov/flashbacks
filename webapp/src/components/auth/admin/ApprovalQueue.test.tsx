import type { ReactNode } from "react"
import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, fireEvent, waitFor } from "@testing-library/react"
import { I18nProvider } from "@/i18n/I18nProvider"
import { ApprovalQueue } from "./ApprovalQueue"
import { RejectUserDialog } from "./RejectUserDialog"
import { AccountStatusBadge } from "./AccountStatusBadge"
import { UserCard } from "./UserCard"
import type { AccountStatus, UserDTO } from "@/types"

const toastError = vi.fn()
vi.mock("sonner", () => ({
  toast: { error: (...args: unknown[]) => toastError(...args), success: vi.fn() },
}))

function makeUser(overrides: Partial<UserDTO> = {}): UserDTO {
  return {
    id: 10,
    login: "alice",
    displayName: "Alice Liddell",
    role: "user",
    accountStatus: "pending",
    hasAvatar: false,
    isActive: true,
    mustChangePassword: false,
    statusChangedAt: null,
    rejectionReason: "",
    createdAt: "2026-10-01 10:20:30",
    lastLoginAt: null,
    ...overrides,
  }
}

function renderEn(node: ReactNode) {
  return render(<I18nProvider language="en">{node}</I18nProvider>)
}

describe("ApprovalQueue", () => {
  beforeEach(() => {
    toastError.mockReset()
  })

  it("lists pending requests with login, display name, request date and both actions", () => {
    const onApprove = vi.fn()
    const onRejectRequest = vi.fn()
    const users = [makeUser(), makeUser({ id: 11, login: "bob", displayName: "Bob Builder" })]

    renderEn(
      <ApprovalQueue
        users={users}
        isLoading={false}
        onApprove={onApprove}
        onRejectRequest={onRejectRequest}
      />
    )

    expect(screen.getByText("Registration Requests")).toBeTruthy()
    expect(screen.getByText("Alice Liddell")).toBeTruthy()
    expect(screen.getByText("alice")).toBeTruthy()
    expect(screen.getAllByText("2026-10-01 10:20:30")).toHaveLength(2)
    expect(screen.getAllByRole("button", { name: "Approve" })).toHaveLength(2)
    expect(screen.getAllByRole("button", { name: "Reject" })).toHaveLength(2)
  })

  it("dispatches approve and reject for the clicked request", () => {
    const onApprove = vi.fn()
    const onRejectRequest = vi.fn()
    const user = makeUser()

    renderEn(
      <ApprovalQueue
        users={[user]}
        isLoading={false}
        onApprove={onApprove}
        onRejectRequest={onRejectRequest}
      />
    )

    fireEvent.click(screen.getByRole("button", { name: "Approve" }))
    expect(onApprove).toHaveBeenCalledWith(user)

    fireEvent.click(screen.getByRole("button", { name: "Reject" }))
    expect(onRejectRequest).toHaveBeenCalledWith(user)
  })

  it("shows the empty state when no request is pending", () => {
    renderEn(
      <ApprovalQueue users={[]} isLoading={false} onApprove={vi.fn()} onRejectRequest={vi.fn()} />
    )

    expect(screen.getByText("No pending requests")).toBeTruthy()
    expect(screen.queryByRole("button", { name: "Approve" })).toBeNull()
  })
})

describe("RejectUserDialog", () => {
  beforeEach(() => {
    toastError.mockReset()
  })

  it("prompts for a reason before confirming the rejection", async () => {
    const onReject = vi.fn().mockResolvedValue(undefined)
    const onClose = vi.fn()

    renderEn(<RejectUserDialog user={makeUser()} onClose={onClose} onReject={onReject} />)

    expect(screen.getByRole("dialog")).toBeTruthy()
    expect(screen.getByLabelText("Rejection reason")).toBeTruthy()

    fireEvent.change(screen.getByLabelText("Rejection reason"), {
      target: { value: "Duplicate account" },
    })
    fireEvent.click(screen.getByRole("button", { name: "Reject" }))

    await waitFor(() => {
      expect(onReject).toHaveBeenCalledWith(expect.objectContaining({ login: "alice" }), "Duplicate account")
    })
    expect(onClose).toHaveBeenCalled()
  })

  it("refuses to confirm without a reason", () => {
    const onReject = vi.fn()

    renderEn(<RejectUserDialog user={makeUser()} onClose={vi.fn()} onReject={onReject} />)

    fireEvent.click(screen.getByRole("button", { name: "Reject" }))

    expect(onReject).not.toHaveBeenCalled()
    expect(toastError).toHaveBeenCalledTimes(1)
  })
})

describe("AccountStatusBadge", () => {
  it.each<[AccountStatus, string]>([
    ["active", "Active"],
    ["pending", "Pending"],
    ["rejected", "Rejected"],
  ])("shows the %s status label", (status, label) => {
    renderEn(<AccountStatusBadge status={status} />)

    expect(screen.getByText(label)).toBeTruthy()
  })
})

describe("UserCard", () => {
  const noop = vi.fn()

  function renderCard(user: UserDTO, isCurrentUser = false) {
    return renderEn(
      <UserCard
        user={user}
        isCurrentUser={isCurrentUser}
        onEdit={noop}
        onResetPassword={noop}
        onDelete={noop}
        onToggleActive={async () => {}}
      />
    )
  }

  it("shows a pending badge for a pending account", () => {
    renderCard(makeUser({ accountStatus: "pending" }))

    expect(screen.getByText("Pending")).toBeTruthy()
    expect(screen.queryByText("Disabled")).toBeNull()
  })

  it("shows a rejected badge for a rejected account", () => {
    renderCard(makeUser({ accountStatus: "rejected" }))

    expect(screen.getByText("Rejected")).toBeTruthy()
  })

  it("renders the active/enabled state only once for an approved active account", () => {
    renderCard(makeUser({ accountStatus: "active", isActive: true }))

    expect(screen.getAllByText("Active")).toHaveLength(1)
  })

  it("shows the disabled badge for an active but deactivated account", () => {
    renderCard(makeUser({ accountStatus: "active", isActive: false }))

    expect(screen.getByText("Disabled")).toBeTruthy()
  })
})
