import { useState } from "react"
import { Loader2, Users } from "lucide-react"
import { Card, CardContent } from "@/components/ui/card"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { EmptyState } from "@/components/EmptyState"
import type { UserDTO } from "@/types"
import { useTranslation } from "@/i18n"
import { UserCard } from "./UserCard"

export interface UserListProps {
  users: UserDTO[]
  isLoading: boolean
  currentUserId?: number
  onEdit: (user: UserDTO) => void
  onResetPassword: (user: UserDTO) => void
  onDelete: (user: UserDTO) => Promise<void>
  onToggleActive: (user: UserDTO) => Promise<void>
}

export function UserList({
  users,
  isLoading,
  currentUserId,
  onEdit,
  onResetPassword,
  onDelete,
  onToggleActive,
}: UserListProps) {
  const { t } = useTranslation()
  const [pendingDelete, setPendingDelete] = useState<UserDTO | null>(null)
  const [isDeleting, setIsDeleting] = useState(false)

  const handleConfirmDelete = async () => {
    if (!pendingDelete) return
    setIsDeleting(true)
    try {
      await onDelete(pendingDelete)
    } finally {
      setIsDeleting(false)
      setPendingDelete(null)
    }
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-20">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    )
  }

  return (
    <>
      {users.length === 0 ? (
        <Card>
          <CardContent className="p-0">
            <EmptyState
              size="md"
              icon={Users}
              title={t("adminPanel.noUsers")}
              description={t("adminPanel.noUsersHint")}
            />
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-4">
          {users.map((u) => (
            <UserCard
              key={u.id}
              user={u}
              isCurrentUser={u.id === currentUserId}
              onEdit={() => onEdit(u)}
              onResetPassword={() => onResetPassword(u)}
              onDelete={() => setPendingDelete(u)}
              onToggleActive={() => onToggleActive(u)}
            />
          ))}
        </div>
      )}

      <Dialog open={!!pendingDelete} onOpenChange={() => !isDeleting && setPendingDelete(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("adminPanel.deleteTitle")}</DialogTitle>
            <DialogDescription>
              {t("adminPanel.deleteConfirm", { displayName: pendingDelete?.displayName ?? "" })}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setPendingDelete(null)} disabled={isDeleting}>
              {t("common.cancel")}
            </Button>
            <Button variant="destructive" onClick={handleConfirmDelete} disabled={isDeleting}>
              {isDeleting ? t("adminPanel.deleting") : t("adminPanel.delete")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
