import { Badge } from "@/components/ui/badge"
import type { AccountStatus } from "@/types"
import { useTranslation } from "@/i18n"

export function AccountStatusBadge({ status }: { status: AccountStatus }) {
  const { t } = useTranslation()
  const variant: "default" | "secondary" | "outline" | "destructive" =
    status === "pending" ? "secondary" : status === "rejected" ? "destructive" : "outline"
  const label =
    status === "pending"
      ? t("adminPanel.statusPending")
      : status === "rejected"
        ? t("adminPanel.statusRejected")
        : t("adminPanel.statusActive")
  return <Badge variant={variant}>{label}</Badge>
}
