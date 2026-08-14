import { useState, type FormEvent } from "react"
import { Loader2 } from "lucide-react"
import { toast } from "sonner"
import { createUser } from "@/api/endpoints"
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
import type { UserRole } from "@/types"
import { useTranslation } from "@/i18n"

export interface CreateUserDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSuccess: () => void
}

export function CreateUserDialog({ open, onOpenChange, onSuccess }: CreateUserDialogProps) {
  const { t } = useTranslation()
  const [login, setLogin] = useState("")
  const [displayName, setDisplayName] = useState("")
  const [role, setRole] = useState<UserRole>("user")
  const [password, setPassword] = useState("")
  const [isLoading, setIsLoading] = useState(false)

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    if (!login.trim() || !displayName.trim() || !password.trim()) return

    setIsLoading(true)
    try {
      await createUser({ login, displayName, role, password })
      toast.success(t("adminPanel.createUserSuccess"))
      setLogin("")
      setDisplayName("")
      setPassword("")
      onOpenChange(false)
      onSuccess()
    } catch (err) {
      const errorMessage = err instanceof Error ? translateApiMessage(err.message) : t("adminPanel.create")
      toast.error(errorMessage)
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("adminPanel.createUserTitle")}</DialogTitle>
          <DialogDescription>{t("adminPanel.createUserDesc")}</DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="create-login">{t("adminPanel.login")}</Label>
            <Input id="create-login" value={login} onChange={(e) => setLogin(e.target.value)} />
          </div>
          <div className="space-y-2">
            <Label htmlFor="create-displayName">{t("adminPanel.displayName")}</Label>
            <Input id="create-displayName" value={displayName} onChange={(e) => setDisplayName(e.target.value)} />
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
          <div className="space-y-2">
            <Label htmlFor="create-password">{t("adminPanel.tempPassword")}</Label>
            <Input id="create-password" type="password" value={password} onChange={(e) => setPassword(e.target.value)} />
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              {t("adminPanel.cancel")}
            </Button>
            <Button type="submit" disabled={isLoading}>
              {isLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              {t("adminPanel.create")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
