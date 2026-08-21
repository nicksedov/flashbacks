import { KeyRound, Pencil, Save, Trash2, X } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent } from "@/components/ui/card"
import type { UserDTO } from "@/types"
import { useTranslation } from "@/i18n"
import { AccountStatusBadge } from "./AccountStatusBadge"

export interface UserCardProps {
  user: UserDTO
  isCurrentUser: boolean
  onEdit: () => void
  onResetPassword: () => void
  onDelete: () => void
  onToggleActive: () => Promise<void>
}

export function UserCard({
  user,
  isCurrentUser,
  onEdit,
  onResetPassword,
  onDelete,
  onToggleActive,
}: UserCardProps) {
  const { t } = useTranslation()
  return (
    <Card>
      <CardContent className="flex items-center justify-between p-4">
        <div className="flex items-center gap-4">
          <div className="flex h-10 w-10 items-center justify-center rounded-full bg-primary/10">
            <span className="font-medium text-primary">{user.displayName.charAt(0).toUpperCase()}</span>
          </div>
          <div>
            <p className="font-medium">
              {user.displayName}
              {isCurrentUser && <span className="ml-2 text-xs text-muted-foreground">{t("adminPanel.you")}</span>}
            </p>
            <p className="text-sm text-muted-foreground">{user.login}</p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <Badge variant={user.role === "admin" ? "default" : "secondary"}>
            {user.role === "admin" ? t("adminPanel.roleAdmin") : t("adminPanel.roleUser")}
          </Badge>
          <AccountStatusBadge status={user.accountStatus} />
          <Badge variant={user.isActive ? "outline" : "destructive"}>
            {user.isActive ? t("adminPanel.statusActive") : t("adminPanel.statusDisabled")}
          </Badge>
          {!isCurrentUser && (
            <>
              <Button variant="ghost" size="icon" onClick={onEdit}>
                <Pencil className="h-4 w-4" />
              </Button>
              <Button variant="ghost" size="icon" onClick={onResetPassword}>
                <KeyRound className="h-4 w-4" />
              </Button>
              <Button variant="ghost" size="icon" onClick={onToggleActive}>
                {user.isActive ? <X className="h-4 w-4" /> : <Save className="h-4 w-4" />}
              </Button>
              <Button variant="ghost" size="icon" onClick={onDelete}>
                <Trash2 className="h-4 w-4 text-destructive" />
              </Button>
            </>
          )}
        </div>
      </CardContent>
    </Card>
  )
}
