import { useCallback, useEffect, useRef, useState } from "react"
import { toast } from "sonner"
import { approveUser, deleteUser, fetchUsers, rejectUser, updateUser } from "@/api/endpoints"
import { translateApiMessage } from "@/api/client"
import type { UserDTO } from "@/types"
import { useTranslation } from "@/i18n"

export interface UseAdminUsersResult {
  users: UserDTO[]
  pendingUsers: UserDTO[]
  isLoading: boolean
  isPendingLoading: boolean
  approve: (user: UserDTO) => Promise<void>
  reject: (user: UserDTO, reason: string) => Promise<void>
  removeUser: (user: UserDTO) => Promise<void>
  toggleActive: (user: UserDTO) => Promise<void>
  refresh: () => void
}

export function useAdminUsers(): UseAdminUsersResult {
  const { t } = useTranslation()
  const [users, setUsers] = useState<UserDTO[]>([])
  const [pendingUsers, setPendingUsers] = useState<UserDTO[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [isPendingLoading, setIsPendingLoading] = useState(true)

  const loadUsers = useCallback(async () => {
    try {
      const response = await fetchUsers()
      setUsers(response.users)
    } catch {
      toast.error(t("adminPanel.toastUsersLoadFailed"))
    } finally {
      setIsLoading(false)
    }
  }, [t])

  const loadPendingUsers = useCallback(async () => {
    try {
      const response = await fetchUsers("pending")
      setPendingUsers(response.users)
    } catch (err) {
      // Non-critical: the full users list still renders status badges
      console.error("Failed to load pending users:", err)
    } finally {
      setIsPendingLoading(false)
    }
  }, [])

  // Keep mutable references to current callbacks so they can be called from
  // event handlers without needing to be in dependency arrays, avoiding stale
  // closure issues with mutate callbacks created per-render.
  const loadUsersRef = useRef(loadUsers)
  const loadPendingUsersRef = useRef(loadPendingUsers)

  useEffect(() => {
    loadUsersRef.current()
    loadPendingUsersRef.current()
  }, [])

  useEffect(() => {
    loadUsersRef.current = loadUsers
  }, [loadUsers])

  useEffect(() => {
    loadPendingUsersRef.current = loadPendingUsers
  }, [loadPendingUsers])

  const refresh = useCallback(() => {
    loadUsersRef.current()
    loadPendingUsersRef.current()
  }, [])

  const approve = useCallback(
    async (user: UserDTO) => {
      try {
        await approveUser(user.id)
        toast.success(t("adminPanel.approveSuccess"))
        refresh()
      } catch (err) {
        const errorMessage = err instanceof Error ? translateApiMessage(err.message) : t("adminPanel.updateFailed")
        toast.error(errorMessage)
      }
    },
    [t, refresh]
  )

  const reject = useCallback(
    async (user: UserDTO, reason: string) => {
      try {
        await rejectUser(user.id, { reason })
        toast.success(t("adminPanel.rejectSuccess"))
        refresh()
      } catch (err) {
        const errorMessage = err instanceof Error ? translateApiMessage(err.message) : t("adminPanel.updateFailed")
        toast.error(errorMessage)
      }
    },
    [t, refresh]
  )

  const removeUser = useCallback(
    async (user: UserDTO) => {
      try {
        await deleteUser(user.id)
        toast.success(t("adminPanel.deleteSuccess"))
        refresh()
      } catch {
        toast.error(t("adminPanel.deleteFailed"))
      }
    },
    [t, refresh]
  )

  const toggleActive = useCallback(
    async (user: UserDTO) => {
      try {
        await updateUser(user.id, { isActive: !user.isActive })
        toast.success(user.isActive ? t("adminPanel.deactivateSuccess") : t("adminPanel.activateSuccess"))
        refresh()
      } catch {
        toast.error(t("adminPanel.updateFailed"))
      }
    },
    [t, refresh]
  )

  return {
    users,
    pendingUsers,
    isLoading,
    isPendingLoading,
    approve,
    reject,
    removeUser,
    toggleActive,
    refresh,
  }
}
