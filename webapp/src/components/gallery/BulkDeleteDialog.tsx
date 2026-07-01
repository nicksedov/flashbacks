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
import { Checkbox } from "@/components/ui/checkbox"
import { Label } from "@/components/ui/label"

interface BulkDeleteDialogProps {
  /** Number of files selected for deletion. */
  count: number
  /** Whether the dialog is open. */
  open: boolean
  /** Called when the dialog is dismissed without deleting. */
  onCancel: () => void
  /** Called when the user confirms deletion. */
  onConfirm: () => void
  /** Whether to use trash. */
  useTrash: boolean
  /** Called when the useTrash toggle changes. */
  onUseTrashChange: (useTrash: boolean) => void
  /** The configured trash directory path (empty string if not set). */
  trashDir: string | undefined
  /** Whether a delete operation is in progress. */
  loading?: boolean
  /** Optional unique id suffix for the checkbox (to avoid id conflicts when multiple dialogs exist). */
  idSuffix?: string
}

/**
 * Shared bulk delete confirmation dialog with trash toggle.
 * Used in both Gallery and Smart Search tabs.
 */
export function BulkDeleteDialog({
  count,
  open,
  onCancel,
  onConfirm,
  useTrash,
  onUseTrashChange,
  trashDir,
  loading = false,
  idSuffix = "",
}: BulkDeleteDialogProps) {
  const { t } = useTranslation()
  const checkboxId = `bulk-delete-use-trash${idSuffix}`

  return (
    <Dialog open={open} onOpenChange={(open) => !open && onCancel()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("deleteFiles.title")}</DialogTitle>
          <DialogDescription>
            {t("deleteFiles.description", { count })}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="rounded-md bg-destructive/10 border border-destructive/20 p-3 text-sm text-destructive">
            {t("deleteFiles.warning")}
          </div>
          <div className="flex items-center gap-2">
            <Checkbox
              id={checkboxId}
              checked={useTrash}
              onCheckedChange={(checked) => onUseTrashChange(checked === true)}
            />
            <Label htmlFor={checkboxId} className="text-sm cursor-pointer">
              {t("deleteFiles.useTrash")}
            </Label>
          </div>
          {useTrash && !trashDir && (
            <p className="text-xs text-destructive">
              {t("deleteFiles.trashNotConfigured")}
            </p>
          )}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onCancel} disabled={loading}>
            {t("common.cancel")}
          </Button>
          <Button variant="destructive" onClick={onConfirm} disabled={loading}>
            {loading ? t("deleteFiles.deleting") : t("deleteFiles.button")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
