import { ShieldCheck } from "lucide-react"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { useTranslation } from "@/i18n"

interface PendingApprovalScreenProps {
  onBackToLogin: () => void
}

export function PendingApprovalScreen({ onBackToLogin }: PendingApprovalScreenProps) {
  const { t } = useTranslation()

  return (
    <div className="flex min-h-screen items-center justify-center bg-gradient-to-br from-background to-muted p-4">
      <Card className="w-full max-w-md">
        <CardHeader className="space-y-2 text-center">
          <div className="mx-auto mb-2 flex h-12 w-12 items-center justify-center rounded-full bg-primary/10">
            <ShieldCheck className="h-6 w-6 text-primary" />
          </div>
          <CardTitle className="text-2xl font-bold">{t("adminPanel.pendingTitle")}</CardTitle>
          <CardDescription>{t("adminPanel.pendingDesc")}</CardDescription>
        </CardHeader>
        <CardContent>
          <Button type="button" variant="outline" className="w-full" onClick={onBackToLogin}>
            {t("adminPanel.signIn")}
          </Button>
        </CardContent>
      </Card>
    </div>
  )
}
