import { useCallback, useMemo, useRef } from "react"
import { useTranslation } from "@/i18n"
import { Folder } from "lucide-react"
import type { GalleryImageDTO, GalleryFolderDTO } from "@/types"
import { ImageTile } from "./ImageTile"

interface GalleryImageGridProps {
  images: GalleryImageDTO[]
  onImageClick: (image: GalleryImageDTO) => void
  onImageDownload?: (image: GalleryImageDTO) => void
  onImageDelete?: (image: GalleryImageDTO) => void
  rootFolders?: GalleryFolderDTO[]
  selectedIds?: Set<number>
  activeSelectionFolder?: string | null
  onToggleSelection?: (image: GalleryImageDTO) => void
  onRangeSelection?: (startImage: GalleryImageDTO, endImage: GalleryImageDTO) => void
}

function normalizePath(p: string): string {
  return p.replace(/\\/g, "/").replace(/\/+$/, "")
}

function getRelativeFolderName(dirPath: string, rootFolders?: GalleryFolderDTO[]): string {
  const normalized = normalizePath(dirPath)

  if (rootFolders?.length) {
    // Find the matching root folder (longest match first)
    const sorted = [...rootFolders].sort((a, b) => b.path.length - a.path.length)
    for (const root of sorted) {
      const rootNorm = normalizePath(root.path)
      if (normalized === rootNorm) {
        // Image is directly in the root folder — show root folder name
        return rootNorm.substring(rootNorm.lastIndexOf("/") + 1)
      }
      if (normalized.startsWith(rootNorm + "/")) {
        // Image is in a subfolder — show relative path including root name
        const rootName = rootNorm.substring(rootNorm.lastIndexOf("/") + 1)
        const relative = normalized.substring(rootNorm.length + 1)
        return rootName + "/" + relative
      }
    }
  }

  // Fallback: return last segment
  const lastSlash = normalized.lastIndexOf("/")
  return lastSlash >= 0 ? normalized.substring(lastSlash + 1) : normalized
}

export function GalleryImageGrid({
  images,
  onImageClick,
  onImageDownload,
  onImageDelete,
  rootFolders,
  selectedIds,
  activeSelectionFolder,
  onToggleSelection,
  onRangeSelection,
}: GalleryImageGridProps) {
  const { t } = useTranslation()
  const lastShiftImageIdRef = useRef<number | null>(null)

  // Build a lookup from image.id -> its folder group images for shift-range selection
  const groupImagesMap = useMemo(() => {
    const map = new Map<number, GalleryImageDTO[]>()
    for (const image of images) {
      let group = map.get(image.id)
      if (!group) {
        group = images.filter((img) => img.dirPath === image.dirPath)
        map.set(image.id, group)
      }
    }
    return map
  }, [images])

  const handleSelectToggle = useCallback((e: React.MouseEvent | React.KeyboardEvent, image: GalleryImageDTO) => {
    if (e.shiftKey && lastShiftImageIdRef.current !== null && onRangeSelection) {
      const groupImages = groupImagesMap.get(lastShiftImageIdRef.current)
      const startImage = groupImages?.find((img) => img.id === lastShiftImageIdRef.current)
      if (startImage && startImage.dirPath === image.dirPath) {
        onRangeSelection(startImage, image)
      }
      return
    }
    if (e.ctrlKey || e.metaKey) {
      lastShiftImageIdRef.current = image.id
      onToggleSelection?.(image)
      return
    }
    if (selectedIds !== undefined) {
      lastShiftImageIdRef.current = image.id
      onToggleSelection?.(image)
    } else {
      onImageClick(image)
    }
  }, [groupImagesMap, onRangeSelection, onToggleSelection, selectedIds, onImageClick])

  const groupedByFolder = useMemo(() => {
    const groups: { dirPath: string; folderName: string; images: GalleryImageDTO[] }[] = []
    const map = new Map<string, GalleryImageDTO[]>()
    const order: string[] = []

    for (const image of images) {
      const dir = image.dirPath
      if (!map.has(dir)) {
        map.set(dir, [])
        order.push(dir)
      }
      map.get(dir)!.push(image)
    }

    for (const dir of order) {
      groups.push({
        dirPath: dir,
        folderName: getRelativeFolderName(dir, rootFolders),
        images: map.get(dir)!,
      })
    }

    return groups
  }, [images, rootFolders])

  return (
    <div className="space-y-5">
      {groupedByFolder.map((group) => (
        <div key={group.dirPath}>
          <div className="flex items-center gap-2 mb-2 px-0.5">
            <Folder className="h-4 w-4 text-muted-foreground shrink-0" />
            <span className="text-sm font-medium truncate" title={group.dirPath}>
              {group.folderName}
            </span>
            <span className="text-xs text-muted-foreground shrink-0">
              {t("gallery.folderImageCount", { count: group.images.length.toString() })}
            </span>
          </div>
          <div className="grid grid-cols-3 sm:grid-cols-4 md:grid-cols-6 lg:grid-cols-7 xl:grid-cols-8 gap-1.5">
            {group.images.map((image) => (
              <ImageTile
                key={image.id}
                image={image}
                onClick={onImageClick}
                onImageDownload={onImageDownload}
                onImageDelete={onImageDelete}
                selected={selectedIds?.has(image.id)}
                selectionActiveInOtherFolder={activeSelectionFolder !== null && image.dirPath !== activeSelectionFolder}
                onSelectToggle={handleSelectToggle}
              />
            ))}
          </div>
        </div>
      ))}
    </div>
  )
}
