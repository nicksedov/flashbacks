import { useCallback, useEffect, useState } from "react"
import { useTranslation } from "@/i18n"
import { Trash2, RotateCcw, XCircle, Loader2, FolderOpen } from "lucide-react"
import { fetchTrashList, restoreTrashFile, deleteTrashFile, cleanTrash } from "@/api/endpoints"
import type { TrashFileDTO } from "@/types"

export function TrashTab() {
  const { t } = useTranslation()
  const [files, setFiles] = useState<TrashFileDTO[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const loadTrash = useCallback(() => {
    setLoading(true)
    setError(null)
    fetchTrashList()
      .then((data) => {
        setFiles(data)
        setLoading(false)
      })
      .catch((err) => {
        setError(err.message)
        setLoading(false)
      })
  }, [])

  useEffect(() => {
    loadTrash()
  }, [loadTrash])

  const handleRestore = useCallback(async (file: TrashFileDTO) => {
    try {
      await restoreTrashFile({ fileName: file.fileName })
      setFiles((prev) => prev.filter((f) => f.fileName !== file.fileName))
    } catch (err) {
      console.error("Failed to restore:", err)
      alert(t("trashTab.restoreFailed"))
    }
  }, [t])

  const handleDelete = useCallback(async (file: TrashFileDTO) => {
    if (!confirm(`Permanently delete "${file.fileName}"?`)) return
    try {
      await deleteTrashFile({ fileName: file.fileName })
      setFiles((prev) => prev.filter((f) => f.fileName !== file.fileName))
    } catch (err) {
      console.error("Failed to delete:", err)
      alert(t("trashTab.deleteFailed"))
    }
  }, [t])

  const handleCleanAll = useCallback(async () => {
    if (!confirm(t("trashTab.cleanAllConfirm"))) return
    try {
      await cleanTrash()
      setFiles([])
    } catch (err) {
      console.error("Failed to clean trash:", err)
    }
  }, [t])

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
            onClick={handleCleanAll}
            className="flex items-center gap-2 px-3 py-1.5 text-sm bg-destructive/10 hover:bg-destructive/20 text-destructive rounded transition-colors"
          >
            <XCircle className="h-4 w-4" />
            {t("trashTab.cleanAll")}
          </button>
        )}
      </div>

      {error && (
        <div className="rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-sm text-destructive">
          {error}
        </div>
      )}

      {loading ? (
        <div className="flex justify-center py-12">
          <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
        </div>
      ) : files.length === 0 ? (
        <div className="rounded-lg border border-dashed p-12 text-center">
          <FolderOpen className="mx-auto h-10 w-10 text-muted-foreground/50" />
          <p className="mt-2 text-sm font-medium text-muted-foreground">
            {t("trashTab.empty")}
          </p>
          <p className="text-xs text-muted-foreground/70">
            {t("trashTab.emptyHint")}
          </p>
        </div>
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
                        onClick={() => handleDelete(file)}
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
    </div>
  )
}
