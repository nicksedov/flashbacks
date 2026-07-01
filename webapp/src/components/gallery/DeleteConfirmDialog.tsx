import { useTranslation } from "@/i18n"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"

interface DeleteConfirmDialogProps {
  /** The file name to display in the confirmation message. */
  fileName: string | undefined
  /** Whether the dialog is open. */
  open: boolean
  /** Called when the dialog is dismissed without deleting. */
  onCancel: () => void
  /** Called when the user confirms deletion. */
  onConfirm: () => void
  /** Whether a delete operation is in progress. */
  loading?: boolean
}

/**
 * Shared single-file delete confirmation dialog.
 * Used in both Gallery and Smart Search tabs.
 */
export function DeleteConfirmDialog({
  fileName,
  open,
  onCancel,
  onConfirm,
  loading = false,
}: DeleteConfirmDialogProps) {
  const { t } = useTranslation()

  return (
    <Dialog open={open} onOpenChange={(open) => !open && onCancel()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("gallery.deleteConfirm.title")}</DialogTitle>
          <DialogDescription>
            {fileName && t("gallery.deleteConfirm.description", { fileName })}
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" onClick={onCancel} disabled={loading}>
            {t("gallery.deleteConfirm.cancel")}
          </Button>
          <Button variant="destructive" onClick={onConfirm} disabled={loading}>
            {loading ? t("gallery.deleteConfirm.deleting") : t("gallery.deleteConfirm.delete")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
