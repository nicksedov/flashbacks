import { useCallback, useState } from "react"
import { UnifiedLightbox } from "@/components/gallery/UnifiedLightbox"
import { ExifFoldersView } from "@/components/gallery/ExifFoldersView"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { deleteFiles } from "@/api/endpoints"
import { useSettings } from "@/providers/useSettings"
import { useTranslation } from "@/i18n"
import type { GalleryImageDTO } from "@/types"

export function ExifTab() {
  const { trashDir } = useSettings()
  const { t } = useTranslation()
  const [selectedImagePath, setSelectedImagePath] = useState<string | null>(null)
  const [showGeoForm, setShowGeoForm] = useState(false)
  const [deleteConfirm, setDeleteConfirm] = useState<{ image: GalleryImageDTO; removeThumbnail: () => void } | null>(null)
  const [isDeleting, setIsDeleting] = useState(false)

  const handleImageClick = (image: GalleryImageDTO) => {
    setSelectedImagePath(image.path)
  }

  const handleAddGeo = (image: GalleryImageDTO) => {
    setSelectedImagePath(image.path)
    setShowGeoForm(true)
  }

  const handleImageDelete = useCallback((image: GalleryImageDTO, removeThumbnail: () => void) => {
    setDeleteConfirm({ image, removeThumbnail })
  }, [])

  const handleConfirmDelete = useCallback(async () => {
    if (!deleteConfirm) return
    setIsDeleting(true)
    try {
      await deleteFiles({
        filePaths: [deleteConfirm.image.path],
        trashDir: trashDir || "",
      })
      deleteConfirm.removeThumbnail()
    } catch (err) {
      console.error("Failed to delete file:", err)
      alert("Failed to delete file")
    } finally {
      setIsDeleting(false)
      setDeleteConfirm(null)
    }
  }, [deleteConfirm, trashDir])

  return (
    <>
      <ExifFoldersView
        onImageClick={handleImageClick}
        onImageDownload={(image) => {
          const link = document.createElement("a")
          link.href = `/api/image?path=${encodeURIComponent(image.path)}`
          link.download = image.fileName
          link.click()
        }}
        onImageDelete={handleImageDelete}
        onAddGeo={handleAddGeo}
      />

      <UnifiedLightbox
        imagePath={selectedImagePath}
        initialMode="exif"
        onClose={() => {
          setSelectedImagePath(null)
          setShowGeoForm(false)
        }}
        showGeoForm={showGeoForm}
        onShowGeoFormChange={setShowGeoForm}
      />

      <Dialog open={!!deleteConfirm} onOpenChange={(open) => !open && setDeleteConfirm(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("gallery.deleteConfirm.title")}</DialogTitle>
            <DialogDescription>
              {deleteConfirm && t("gallery.deleteConfirm.description", { fileName: deleteConfirm.image.fileName })}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteConfirm(null)} disabled={isDeleting}>
              {t("gallery.deleteConfirm.cancel")}
            </Button>
            <Button variant="destructive" onClick={handleConfirmDelete} disabled={isDeleting}>
              {isDeleting ? t("gallery.deleteConfirm.deleting") : t("gallery.deleteConfirm.delete")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
