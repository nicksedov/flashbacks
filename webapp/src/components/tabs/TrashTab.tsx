import { useCallback, useEffect, useState } from "react"
import { toast } from "sonner"
import { useTranslation } from "@/i18n"
import { ErrorBanner } from "@/components/ui/error-banner"
import { ConfirmDialog } from "@/components/ui/confirm-dialog"
import { Trash2, RotateCcw, XCircle, Loader2, FolderOpen } from "lucide-react"
import { fetchTrashList, restoreTrashFile, deleteTrashFile, cleanTrash } from "@/api/endpoints"
import { EmptyState } from "@/components/EmptyState"
import type { TrashFileDTO } from "@/types"

export function TrashTab() {
  const { t } = useTranslation()
  const [files, setFiles] = useState<TrashFileDTO[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<TrashFileDTO | null>(null)
  const [cleanAllOpen, setCleanAllOpen] = useState(false)
  const [isDeleting, setIsDeleting] = useState(false)
  const [isCleaning, setIsCleaning] = useState(false)

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      try {
        const data = await fetchTrashList()
        if (!cancelled) {
          setFiles(data)
          setError(null)
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "Failed to load trash")
        }
      } finally {
        if (!cancelled) setLoading(false)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [])

  const handleRestore = useCallback(async (file: TrashFileDTO) => {
    try {
      await restoreTrashFile({ fileName: file.fileName })
      setFiles((prev) => prev.filter((f) => f.fileName !== file.fileName))
    } catch (err) {
      console.error("Failed to restore:", err)
      toast.error(t("trashTab.restoreFailed"))
    }
  }, [t])

  const handleConfirmDelete = useCallback(async () => {
    if (!deleteTarget) return
    setIsDeleting(true)
    try {
      await deleteTrashFile({ fileName: deleteTarget.fileName })
      setFiles((prev) => prev.filter((f) => f.fileName !== deleteTarget.fileName))
      setDeleteTarget(null)
    } catch (err) {
      console.error("Failed to delete:", err)
      toast.error(t("trashTab.deleteFailed"))
    } finally {
      setIsDeleting(false)
    }
  }, [deleteTarget, t])

  const handleConfirmCleanAll = useCallback(async () => {
    setIsCleaning(true)
    try {
      await cleanTrash()
      setFiles([])
      setCleanAllOpen(false)
    } catch (err) {
      console.error("Failed to clean trash:", err)
    } finally {
      setIsCleaning(false)
    }
  }, [])

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Trash2 className="h-5 w-5 text-muted-foreground" />
          <span className="text-sm text-muted-foreground">
            {files.length === 1
              ? t("trashTab.fileCountOne", { count: files.length })
              : t("trashTab.fileCount", { count: files.length })}
          </span>
        </div>
        {files.length > 0 && (
          <button
            onClick={() => setCleanAllOpen(true)}
            className="flex items-center gap-2 px-3 py-1.5 text-sm bg-destructive/10 hover:bg-destructive/20 text-destructive rounded transition-colors"
          >
            <XCircle className="h-4 w-4" />
            {t("trashTab.cleanAll")}
          </button>
        )}
      </div>

      {error && <ErrorBanner message={error} />}

      {loading ? (
        <div className="flex justify-center py-12">
          <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
        </div>
      ) : files.length === 0 ? (
        <EmptyState
          bordered
          icon={FolderOpen}
          title={t("trashTab.empty")}
          description={t("trashTab.emptyHint")}
        />
      ) : (
        <div className="rounded-lg border overflow-hidden">
          <table className="w-full">
            <thead className="bg-muted">
              <tr>
                <th className="text-left px-4 py-2 text-xs font-medium text-muted-foreground">
                  {t("galleryList.fileName")}
                </th>
                <th className="text-left px-4 py-2 text-xs font-medium text-muted-foreground hidden sm:table-cell">
                  {t("galleryList.size")}
                </th>
                <th className="text-right px-4 py-2 text-xs font-medium text-muted-foreground">
                  {t("common.actions")}
                </th>
              </tr>
            </thead>
            <tbody>
              {files.map((file) => (
                <tr key={file.fileName} className="border-t hover:bg-muted/30 transition-colors">
                  <td className="px-4 py-2 text-sm font-medium truncate" title={file.fileName}>
                    {file.fileName}
                  </td>
                  <td className="px-4 py-2 text-sm text-muted-foreground hidden sm:table-cell">
                    {file.sizeHuman}
                  </td>
                  <td className="px-4 py-2 text-right">
                    <div className="flex items-center justify-end gap-1">
                      <button
                        onClick={() => handleRestore(file)}
                        className="p-1.5 rounded-lg hover:bg-emerald-100 dark:hover:bg-emerald-900/30 text-emerald-600 transition-colors"
                        title={t("trashTab.restore")}
                      >
                        <RotateCcw className="h-4 w-4" />
                      </button>
                      <button
                        onClick={() => setDeleteTarget(file)}
                        className="p-1.5 rounded-lg hover:bg-destructive/10 text-destructive transition-colors"
                        title={t("trashTab.deletePermanently")}
                      >
                        <XCircle className="h-4 w-4" />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !isDeleting && !open && setDeleteTarget(null)}
        title={t("trashTab.deletePermanently")}
        description={t("trashTab.deleteConfirm", { fileName: deleteTarget?.fileName ?? "" })}
        confirmLabel={isDeleting ? t("trashTab.deleting") : t("common.delete")}
        cancelLabel={t("common.cancel")}
        onConfirm={() => void handleConfirmDelete()}
        loading={isDeleting}
        destructive
      />

      <ConfirmDialog
        open={cleanAllOpen}
        onOpenChange={(open) => !isCleaning && setCleanAllOpen(open)}
        title={t("trashTab.cleanAll")}
        description={t("trashTab.cleanAllConfirm")}
        confirmLabel={isCleaning ? t("trashTab.cleaning") : t("trashTab.cleanAll")}
        cancelLabel={t("common.cancel")}
        onConfirm={() => void handleConfirmCleanAll()}
        loading={isCleaning}
        destructive
      />
    </div>
  )
}
