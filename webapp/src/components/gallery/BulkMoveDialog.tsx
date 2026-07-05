import { useState, useEffect, useCallback } from "react"
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
import { Separator } from "@/components/ui/separator"
import { fetchFolders, createFolder } from "@/api/endpoints"
import { GalleryFolderTree } from "./GalleryFolderTree"
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
 * Shows a collapsible folder tree of gallery folders with subdirectory navigation
 * and the ability to create new subfolders.
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
  const [newFolderName, setNewFolderName] = useState("")
  const [isCreating, setIsCreating] = useState(false)
  const [createError, setCreateError] = useState<string | null>(null)

  // Load gallery folders and reset form state when dialog opens
  useEffect(() => {
    if (open) {
      setSelectedFolder("")
      setNewFolderName("")
      setCreateError(null)
      fetchFolders()
        .then((res) => setFolders(res.folders))
        .catch(() => setFolders([]))
    }
  }, [open])

  const handleConfirm = () => {
    if (!selectedFolder) return
    onConfirm(selectedFolder)
  }

  const handleSelect = useCallback((path: string) => {
    setSelectedFolder(path)
  }, [])

  const handleCreateFolder = useCallback(async () => {
    const name = newFolderName.trim()
    if (!name || !selectedFolder) return

    setIsCreating(true)
    setCreateError(null)

    try {
      const res = await createFolder({
        parentPath: selectedFolder,
        folderName: name,
      })
      // Select the newly created folder
      setSelectedFolder(res.path)
      setNewFolderName("")
    } catch (err) {
      setCreateError(err instanceof Error ? err.message : "Failed to create folder")
    } finally {
      setIsCreating(false)
    }
  }, [newFolderName, selectedFolder])

  const rootPaths = folders.map((f) => f.path)
  const canConfirm = selectedFolder.length > 0
  const canCreate = selectedFolder.length > 0 && newFolderName.trim().length > 0

  return (
    <Dialog open={open} onOpenChange={(open) => !open && onCancel()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{t("moveFiles.title")}</DialogTitle>
          <DialogDescription>
            {t("moveFiles.description", { count })}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          {/* Folder tree */}
          <div className="space-y-2">
            <Label className="text-sm font-medium">
              {t("moveFiles.selectFolder")}
            </Label>
            <GalleryFolderTree
              rootPaths={rootPaths}
              selectedPath={selectedFolder}
              onSelect={handleSelect}
              isLoading={folders.length === 0}
              className="max-h-60 overflow-y-auto p-1"
            />
          </div>

          {/* Selected folder display */}
          {selectedFolder && (
            <div className="text-xs text-muted-foreground bg-muted rounded-md px-3 py-2 truncate">
              {selectedFolder}
            </div>
          )}

          <Separator />

          {/* Create new folder */}
          <div className="space-y-2">
            <Label className="text-sm font-medium">
              {t("moveFiles.createFolder")}
            </Label>
            <div className="flex gap-2">
              <Input
                value={newFolderName}
                onChange={(e) => {
                  setNewFolderName(e.target.value)
                  setCreateError(null)
                }}
                placeholder={t("moveFiles.newFolderPlaceholder")}
                disabled={!selectedFolder || isCreating}
                onKeyDown={(e) => {
                  if (e.key === "Enter" && canCreate && !isCreating) {
                    handleCreateFolder()
                  }
                }}
              />
              <Button
                variant="outline"
                size="sm"
                onClick={handleCreateFolder}
                disabled={!canCreate || isCreating}
                className="shrink-0"
              >
                {isCreating ? t("common.saving") : t("moveFiles.createButton")}
              </Button>
            </div>
            {createError && (
              <p className="text-xs text-destructive">{createError}</p>
            )}
            {!selectedFolder && (
              <p className="text-xs text-muted-foreground">
                {t("moveFiles.selectFolderFirst")}
              </p>
            )}
          </div>
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
