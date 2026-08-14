import { useState, type FormEvent } from "react"
import { Loader2 } from "lucide-react"
import { toast } from "sonner"
import { resetUserPassword } from "@/api/endpoints"
import { translateApiMessage } from "@/api/client"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import type { UserDTO } from "@/types"
import { useTranslation } from "@/i18n"

export interface ResetPasswordDialogProps {
  user: UserDTO
  onClose: () => void
}

export function ResetPasswordDialog({ user, onClose }: ResetPasswordDialogProps) {
  const { t } = useTranslation()
  const [newPassword, setNewPassword] = useState("")
  const [isLoading, setIsLoading] = useState(false)

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    if (newPassword.length < 8) return

    setIsLoading(true)
    try {
      await resetUserPassword(user.id, { newPassword })
      toast.success(t("adminPanel.resetPasswordSuccess"))
      onClose()
    } catch (err) {
      const errorMessage = err instanceof Error ? translateApiMessage(err.message) : t("adminPanel.resetPasswordFailed")
      toast.error(errorMessage)
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <Dialog open={true} onOpenChange={onClose}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("adminPanel.resetPasswordTitle")}</DialogTitle>
          <DialogDescription>{t("adminPanel.resetPasswordDesc", { displayName: user.displayName })}</DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="reset-password">{t("adminPanel.newPassword")}</Label>
            <Input
              id="reset-password"
              type="password"
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
            />
            <p className="text-xs text-muted-foreground">{t("adminPanel.minPasswordLength")}</p>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              {t("adminPanel.cancel")}
            </Button>
            <Button type="submit" disabled={isLoading || newPassword.length < 8}>
              {isLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              {t("adminPanel.resetPassword")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
