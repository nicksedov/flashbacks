import { Loader2, Users } from "lucide-react"
import { Card, CardContent } from "@/components/ui/card"
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

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-20">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    )
  }

  if (users.length === 0) {
    return (
      <Card>
        <CardContent className="flex flex-col items-center justify-center py-12">
          <Users className="mb-4 h-12 w-12 text-muted-foreground" />
          <p className="text-lg font-medium">{t("adminPanel.noUsers")}</p>
          <p className="text-sm text-muted-foreground">{t("adminPanel.noUsersHint")}</p>
        </CardContent>
      </Card>
    )
  }

  return (
    <div className="grid gap-4">
      {users.map((u) => (
        <UserCard
          key={u.id}
          user={u}
          isCurrentUser={u.id === currentUserId}
          onEdit={() => onEdit(u)}
          onResetPassword={() => onResetPassword(u)}
          onDelete={async () => {
            if (!confirm(t("adminPanel.deleteConfirm", { displayName: u.displayName }))) return
            await onDelete(u)
          }}
          onToggleActive={() => onToggleActive(u)}
        />
      ))}
    </div>
  )
}
