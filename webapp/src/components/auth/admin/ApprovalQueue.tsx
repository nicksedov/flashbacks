import { Check, Loader2, UserCheck, X } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import type { UserDTO } from "@/types"
import { useTranslation } from "@/i18n"

export interface ApprovalQueueProps {
  users: UserDTO[]
  isLoading: boolean
  onApprove: (user: UserDTO) => void
  onRejectRequest: (user: UserDTO) => void
}

export function ApprovalQueue({ users, isLoading, onApprove, onRejectRequest }: ApprovalQueueProps) {
  const { t } = useTranslation()
  return (
    <Card>
      <CardContent className="p-4">
        <div className="mb-3 flex items-center gap-2">
          <UserCheck className="h-5 w-5 text-primary" />
          <h3 className="text-lg font-semibold">{t("adminPanel.registrationRequests")}</h3>
        </div>
        {isLoading ? (
          <div className="flex items-center justify-center py-6">
            <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
          </div>
        ) : users.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t("adminPanel.registrationRequestsEmpty")}</p>
        ) : (
          <div className="grid gap-3">
            {users.map((u) => (
              <div key={u.id} className="flex items-center justify-between rounded-lg border p-3">
                <div className="min-w-0">
                  <p className="truncate font-medium">{u.displayName}</p>
                  <p className="truncate text-sm text-muted-foreground">{u.login}</p>
                  <p className="text-xs text-muted-foreground">{u.createdAt}</p>
                </div>
                <div className="flex shrink-0 items-center gap-2">
                  <Button variant="default" size="sm" onClick={() => onApprove(u)}>
                    <Check className="mr-1 h-4 w-4" />
                    {t("adminPanel.approve")}
                  </Button>
                  <Button variant="destructive" size="sm" onClick={() => onRejectRequest(u)}>
                    <X className="mr-1 h-4 w-4" />
                    {t("adminPanel.reject")}
                  </Button>
                </div>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  )
}
