import { useState } from "react"
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Label } from "@/components/ui/label"
import { deleteFiles } from "@/api/endpoints"
import { useSettings } from "@/providers/useSettings"
import { useTranslation } from "@/i18n"

interface DeleteFilesModalProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  selectedPaths: string[]
  onSuccess: (message: string) => void
  onError: (message: string) => void
  onComplete: () => void
}

export function DeleteFilesModal({
  open,
  onOpenChange,
  selectedPaths,
  onSuccess,
  onError,
  onComplete,
}: DeleteFilesModalProps) {
  const [useTrash, setUseTrash] = useState(true)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const { trashDir } = useSettings()
  const { t } = useTranslation()

  const handleDelete = async () => {
    if (!useTrash || !trashDir) {
      if (!window.confirm(t("deleteFiles.confirmPermanent"))) {
        return
      }
    }

    setIsSubmitting(true)
    try {
      const result = await deleteFiles({
        filePaths: selectedPaths,
        trashDir: useTrash ? trashDir : "",
      })
      onOpenChange(false)
      const message =
        result.failed > 0
          ? t("deleteFiles.successWithFailed", { count: result.success, failed: result.failed })
          : t("deleteFiles.success", { count: result.success })
      onSuccess(message)
      onComplete()
    } catch (err) {
      onError(err instanceof Error ? err.message : t("deleteFiles.errorFailed"))
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("deleteFiles.title")}</DialogTitle>
          <DialogDescription>
            {t("deleteFiles.description", { count: selectedPaths.length })}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="rounded-md bg-destructive/10 border border-destructive/20 p-3 text-sm text-destructive">
            {t("deleteFiles.warning")}
          </div>
          <div className="flex items-center gap-2">
            <Checkbox
              id="delete-use-trash"
              checked={useTrash}
              onCheckedChange={(checked) => setUseTrash(checked === true)}
            />
            <Label htmlFor="delete-use-trash" className="text-sm cursor-pointer">
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
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={isSubmitting}>
            {t("common.cancel")}
          </Button>
          <Button variant="destructive" onClick={handleDelete} disabled={isSubmitting}>
            {isSubmitting ? t("deleteFiles.deleting") : t("deleteFiles.button")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
