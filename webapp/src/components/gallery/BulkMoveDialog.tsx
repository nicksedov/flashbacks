import { useState, useEffect } from "react"
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
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { fetchFolders } from "@/api/endpoints"
import type { GalleryFolderDTO } from "@/types"

interface BulkMoveDialogProps {
  /** Number of files selected for moving. */
  count: number
  /** Whether the dialog is open. */
  open: boolean
  /** Called when the dialog is dismissed without moving. */
  onCancel: () => void
  /** Called when the user confirms the move. */
  onConfirm: (targetDir: string) => void
  /** Whether a move operation is in progress. */
  loading?: boolean
}

/**
 * Dialog for selecting a target folder and confirming bulk file move.
 * Shows available gallery root folders and allows custom path input.
 */
export function BulkMoveDialog({
  count,
  open,
  onCancel,
  onConfirm,
  loading = false,
}: BulkMoveDialogProps) {
  const { t } = useTranslation()
  const [folders, setFolders] = useState<GalleryFolderDTO[]>([])
  const [selectedFolder, setSelectedFolder] = useState<string>("")
  const [customPath, setCustomPath] = useState("")
  const [useCustomPath, setUseCustomPath] = useState(false)

  // Load gallery folders when dialog opens
  useEffect(() => {
    if (open) {
      fetchFolders().then((res) => {
        setFolders(res.folders)
      }).catch(() => {
        setFolders([])
      })
      setSelectedFolder("")
      setCustomPath("")
      setUseCustomPath(false)
    }
  }, [open])

  const handleConfirm = () => {
    const targetDir = useCustomPath ? customPath.trim() : selectedFolder
    if (!targetDir) return
    onConfirm(targetDir)
  }

  const canConfirm = useCustomPath
    ? customPath.trim().length > 0
    : selectedFolder.length > 0

  return (
    <Dialog open={open} onOpenChange={(open) => !open && onCancel()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("moveFiles.title")}</DialogTitle>
          <DialogDescription>
            {t("moveFiles.description", { count })}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          {/* Gallery folders */}
          <div className="space-y-2">
            <Label className="text-sm font-medium">
              {t("moveFiles.selectFolder")}
            </Label>
            {folders.length === 0 ? (
              <p className="text-sm text-muted-foreground">
                {t("moveFiles.noFolders")}
              </p>
            ) : (
              <div className="max-h-48 overflow-y-auto space-y-1 rounded-md border p-2">
                {folders.map((folder) => (
                  <button
                    key={folder.id}
                    type="button"
                    className={`w-full text-left px-3 py-2 rounded-md text-sm transition-colors ${
                      !useCustomPath && selectedFolder === folder.path
                        ? "bg-primary text-primary-foreground"
                        : "hover:bg-accent hover:text-accent-foreground"
                    }`}
                    onClick={() => {
                      setSelectedFolder(folder.path)
                      setUseCustomPath(false)
                    }}
                  >
                    {folder.path}
                  </button>
                ))}
              </div>
            )}
          </div>

          {/* Custom path toggle */}
          <div className="flex items-center gap-2">
            <label className="flex items-center gap-2 text-sm cursor-pointer">
              <input
                type="checkbox"
                checked={useCustomPath}
                onChange={(e) => setUseCustomPath(e.target.checked)}
                className="h-4 w-4 rounded border-gray-300"
              />
              {t("moveFiles.customPath")}
            </label>
          </div>

          {useCustomPath && (
            <div className="space-y-2">
              <Label htmlFor="target-path" className="text-sm">
                {t("moveFiles.targetPathLabel")}
              </Label>
              <Input
                id="target-path"
                value={customPath}
                onChange={(e) => setCustomPath(e.target.value)}
                placeholder={t("moveFiles.targetPathPlaceholder")}
              />
            </div>
          )}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onCancel} disabled={loading}>
            {t("common.cancel")}
          </Button>
          <Button onClick={handleConfirm} disabled={!canConfirm || loading}>
            {loading ? t("moveFiles.moving") : t("moveFiles.button")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
