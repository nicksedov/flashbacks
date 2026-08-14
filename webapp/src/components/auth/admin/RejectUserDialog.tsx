import { useState, type FormEvent } from "react"
import { Loader2 } from "lucide-react"
import { toast } from "sonner"
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

export interface RejectUserDialogProps {
  user: UserDTO
  onClose: () => void
  onReject: (user: UserDTO, reason: string) => Promise<void>
}

export function RejectUserDialog({ user, onClose, onReject }: RejectUserDialogProps) {
  const { t } = useTranslation()
  const [reason, setReason] = useState("")
  const [isLoading, setIsLoading] = useState(false)

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    if (!reason.trim()) {
      toast.error(t("adminPanel.rejectionReasonRequired"))
      return
    }

    setIsLoading(true)
    try {
      await onReject(user, reason.trim())
      onClose()
    } catch {
      // Error toast is handled by the caller
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <Dialog open={true} onOpenChange={onClose}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("adminPanel.rejectTitle")}</DialogTitle>
          <DialogDescription>{t("adminPanel.rejectDesc", { displayName: user.displayName })}</DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="reject-reason">{t("adminPanel.rejectionReason")}</Label>
            <Input
              id="reject-reason"
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              placeholder={t("adminPanel.rejectionReasonPlaceholder")}
            />
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              {t("adminPanel.cancel")}
            </Button>
            <Button type="submit" variant="destructive" disabled={isLoading}>
              {isLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              {t("adminPanel.reject")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
