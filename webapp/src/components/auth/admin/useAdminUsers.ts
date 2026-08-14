import { useCallback, useEffect, useState } from "react"
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

  // Event-driven loaders (manual refresh after mutations). Loading flags are
  // initialized to `true`, so these only clear them and never trigger
  // synchronous state updates when awaited from the mount effect below.
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

  // Initial load on mount. Fetches directly inside the effect with a
  // cancellation guard instead of invoking the loaders, so no setState runs
  // synchronously in the effect body — state updates only happen in async
  // callbacks after the requests resolve.
  useEffect(() => {
    let cancelled = false

    void (async () => {
      try {
        const response = await fetchUsers()
        if (!cancelled) setUsers(response.users)
      } catch {
        if (!cancelled) toast.error(t("adminPanel.toastUsersLoadFailed"))
      } finally {
        if (!cancelled) setIsLoading(false)
      }
    })()

    void (async () => {
      try {
        const response = await fetchUsers("pending")
        if (!cancelled) setPendingUsers(response.users)
      } catch (err) {
        // Non-critical: the full users list still renders status badges
        if (!cancelled) console.error("Failed to load pending users:", err)
      } finally {
        if (!cancelled) setIsPendingLoading(false)
      }
    })()

    return () => {
      cancelled = true
    }
  }, [t])

  const refresh = useCallback(() => {
    setIsLoading(true)
    setIsPendingLoading(true)
    loadUsers()
    loadPendingUsers()
  }, [loadUsers, loadPendingUsers])

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
