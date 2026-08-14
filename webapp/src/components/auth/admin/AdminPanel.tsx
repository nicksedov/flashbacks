import { useState } from "react"
import { useAuth } from "@/providers/useAuth"
import { UserPlus } from "lucide-react"
import { Button } from "@/components/ui/button"
import type { UserDTO } from "@/types"
import { useTranslation } from "@/i18n"
import { useAdminUsers } from "./useAdminUsers"
import { ApprovalQueue } from "./ApprovalQueue"
import { UserList } from "./UserList"
import { CreateUserDialog } from "./CreateUserDialog"
import { EditUserDialog } from "./EditUserDialog"
import { ResetPasswordDialog } from "./ResetPasswordDialog"
import { RejectUserDialog } from "./RejectUserDialog"

export function AdminPanel() {
  const { user: currentUser } = useAuth()
  const { t } = useTranslation()
  const {
    users,
    pendingUsers,
    isLoading,
    isPendingLoading,
    approve,
    reject,
    removeUser,
    toggleActive,
    refresh,
  } = useAdminUsers()

  const [isCreateOpen, setIsCreateOpen] = useState(false)
  const [editingUser, setEditingUser] = useState<UserDTO | null>(null)
  const [resettingUser, setResettingUser] = useState<UserDTO | null>(null)
  const [rejectingUser, setRejectingUser] = useState<UserDTO | null>(null)

  if (currentUser?.role !== "admin") {
    return (
      <div className="flex items-center justify-center py-20">
        <p className="text-muted-foreground">{t("adminPanel.accessDenied")}</p>
      </div>
    )
  }

  return (
    <div className="mx-auto max-w-6xl space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold">{t("adminPanel.title")}</h2>
          <p className="text-muted-foreground">{t("adminPanel.description")}</p>
        </div>
        <Button onClick={() => setIsCreateOpen(true)}>
          <UserPlus className="mr-2 h-4 w-4" />
          {t("adminPanel.createButton")}
        </Button>
      </div>

      {/* Registration approval queue */}
      <ApprovalQueue
        users={pendingUsers}
        isLoading={isPendingLoading}
        onApprove={approve}
        onRejectRequest={setRejectingUser}
      />

      <UserList
        users={users}
        isLoading={isLoading}
        currentUserId={currentUser?.id}
        onEdit={setEditingUser}
        onResetPassword={setResettingUser}
        onDelete={removeUser}
        onToggleActive={toggleActive}
      />

      <CreateUserDialog open={isCreateOpen} onOpenChange={setIsCreateOpen} onSuccess={refresh} />
      {editingUser && (
        <EditUserDialog user={editingUser} onClose={() => setEditingUser(null)} onSuccess={refresh} />
      )}
      {resettingUser && (
        <ResetPasswordDialog user={resettingUser} onClose={() => setResettingUser(null)} />
      )}
      {rejectingUser && (
        <RejectUserDialog
          user={rejectingUser}
          onClose={() => setRejectingUser(null)}
          onReject={reject}
        />
      )}
    </div>
  )
}
