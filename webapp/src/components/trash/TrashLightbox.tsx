import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from "@/components/ui/dialog"
import { useTranslation } from "@/i18n"
import { formatDateTime } from "@/lib/format"
import { buildTrashImageUrl } from "@/api/endpoints"
import type { TrashItemDTO } from "@/types"

interface TrashLightboxProps {
  item: TrashItemDTO | null
  onOpenChange: (open: boolean) => void
}

/**
 * Dialog-based lightbox for a trashed file: renders the full-size image via
 * GET /api/trash/image plus the original location and deletion date.
 */
export function TrashLightbox({ item, onOpenChange }: TrashLightboxProps) {
  const { t } = useTranslation()

  return (
    <Dialog open={item !== null} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-3xl">
        <DialogHeader>
          <DialogTitle>{item?.fileName ?? ""}</DialogTitle>
          {item && (
            <DialogDescription className="space-y-1">
              <div>
                <span className="font-medium">{t("trashTab.originalLocation")}: </span>
                <span className="break-all" title={item.originalPath}>
                  {item.originalPath || t("trashTab.originalLocationUnknown")}
                </span>
              </div>
              <div>
                <span className="font-medium">{t("trashTab.deletedDate")}: </span>
                {formatDateTime(item.deletedAt)}
              </div>
            </DialogDescription>
          )}
        </DialogHeader>
        {item && (
          <div className="overflow-auto rounded-lg border bg-black/5">
            <img
              src={buildTrashImageUrl(item.trashPath)}
              alt={item.fileName}
              className="mx-auto max-h-[70vh] w-auto object-contain"
            />
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}
