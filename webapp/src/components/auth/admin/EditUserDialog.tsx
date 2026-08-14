import { useState, type FormEvent } from "react"
import { Loader2 } from "lucide-react"
import { toast } from "sonner"
import { updateUser } from "@/api/endpoints"
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import type { UserDTO, UserRole } from "@/types"
import { useTranslation } from "@/i18n"

export interface EditUserDialogProps {
  user: UserDTO
  onClose: () => void
  onSuccess: () => void
}

export function EditUserDialog({ user, onClose, onSuccess }: EditUserDialogProps) {
  const { t } = useTranslation()
  const [displayName, setDisplayName] = useState(user.displayName)
  const [role, setRole] = useState<UserRole>(user.role)
  const [isActive, setIsActive] = useState(user.isActive)
  const [isLoading, setIsLoading] = useState(false)

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    if (!displayName.trim()) return

    setIsLoading(true)
    try {
      await updateUser(user.id, { displayName, role, isActive })
      toast.success(t("adminPanel.profileUpdated"))
      onSuccess()
      onClose()
    } catch (err) {
      const errorMessage = err instanceof Error ? translateApiMessage(err.message) : t("adminPanel.updateFailed")
      toast.error(errorMessage)
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <Dialog open={true} onOpenChange={onClose}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("adminPanel.editUserTitle")}</DialogTitle>
          <DialogDescription>{t("adminPanel.editUserDesc")}</DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label>{t("adminPanel.login")}</Label>
            <Input value={user.login} disabled />
          </div>
          <div className="space-y-2">
            <Label>{t("adminPanel.displayName")}</Label>
            <Input value={displayName} onChange={(e) => setDisplayName(e.target.value)} />
          </div>
          <div className="space-y-2">
            <Label>{t("adminPanel.role")}</Label>
            <Select value={role} onValueChange={(v) => setRole(v as UserRole)}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="user">{t("adminPanel.roleUser")}</SelectItem>
                <SelectItem value="admin">{t("adminPanel.adminRole")}</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="flex items-center gap-2">
            <input
              type="checkbox"
              id="isActive"
              checked={isActive}
              onChange={(e) => setIsActive(e.target.checked)}
              className="h-4 w-4"
            />
            <Label htmlFor="isActive">{t("adminPanel.statusActive")}</Label>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              {t("adminPanel.cancel")}
            </Button>
            <Button type="submit" disabled={isLoading}>
              {isLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              {t("adminPanel.save")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
