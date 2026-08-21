import { useState } from "react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog"
import { FolderPlus, Plus } from "lucide-react"
import { useTranslation } from "@/i18n"

interface AddFolderFormProps {
  onAdd: (path: string) => Promise<void>
  disabled?: boolean
}

export function AddFolderForm({ onAdd, disabled }: AddFolderFormProps) {
  const [open, setOpen] = useState(false)
  const [path, setPath] = useState("")
  const [isSubmitting, setIsSubmitting] = useState(false)
  const { t } = useTranslation()

  const handleSubmit = async (e: React.SyntheticEvent<HTMLFormElement>) => {
    e.preventDefault()
    const trimmed = path.trim()
    if (!trimmed) return

    setIsSubmitting(true)
    try {
      await onAdd(trimmed)
      setPath("")
      setOpen(false)
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(next) => !isSubmitting && setOpen(next)}>
      <Button
        type="button"
        size="sm"
        onClick={() => setOpen(true)}
        disabled={disabled}
        aria-haspopup="dialog"
      >
        <Plus className="mr-1.5 h-3.5 w-3.5" />
        {t("addFolder.button")}
      </Button>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("addFolder.title")}</DialogTitle>
          <DialogDescription>{t("addFolder.description")}</DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4">
          <Input
            value={path}
            onChange={(e) => setPath(e.target.value)}
            placeholder={t("addFolder.placeholder")}
            disabled={disabled || isSubmitting}
            className="font-mono text-sm"
            autoFocus
          />
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => setOpen(false)}
              disabled={isSubmitting}
            >
              {t("common.cancel")}
            </Button>
            <Button
              type="submit"
              disabled={disabled || isSubmitting || !path.trim()}
            >
              <FolderPlus className="mr-1.5 h-3.5 w-3.5" />
              {isSubmitting ? t("common.saving") : t("addFolder.button")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
