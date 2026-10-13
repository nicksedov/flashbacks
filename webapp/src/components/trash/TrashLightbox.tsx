import { useState } from "react"
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from "@/components/ui/dialog"
import { useTranslation } from "@/i18n"
import { formatDateTime } from "@/lib/format"
import { buildTrashImageUrl } from "@/api/endpoints"
import { MobileDetailPanel } from "@/components/gallery/lightbox/MobileDetailPanel"
import type { TrashItemDTO } from "@/types"

interface TrashLightboxProps {
  item: TrashItemDTO | null
  onOpenChange: (open: boolean) => void
}

/** Original location + deletion date, shared by the header (md+) and the mobile panel. */
function TrashMetadata({ item }: { item: TrashItemDTO }) {
  const { t } = useTranslation()
  return (
    <>
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
    </>
  )
}

/**
 * Dialog-based lightbox for a trashed file: renders the full-size image via
 * GET /api/trash/image plus the original location and deletion date.
 *
 * Below `md` the image fills the lightbox and the location/date metadata moves
 * into a collapsible `MobileDetailPanel` overlay; at `md` and above the layout
 * is unchanged (metadata in the header, image below).
 */
export function TrashLightbox({ item, onOpenChange }: TrashLightboxProps) {
  const [panelOpen, setPanelOpen] = useState(false)

  return (
    <Dialog open={item !== null} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-3xl">
        <DialogHeader className="min-w-0 pr-10">
          <DialogTitle className="min-w-0 truncate">{item?.fileName ?? ""}</DialogTitle>
          {item && (
            <DialogDescription asChild>
              <div className="hidden space-y-1 md:block">
                <TrashMetadata item={item} />
              </div>
            </DialogDescription>
          )}
        </DialogHeader>
        {item && (
          <div className="relative flex min-h-0">
            <div className="flex-1 min-w-0 overflow-auto rounded-lg border bg-black/5">
              <img
                src={buildTrashImageUrl(item.trashPath)}
                alt={item.fileName}
                className="mx-auto max-h-[70vh] w-auto object-contain"
              />
            </div>
            <MobileDetailPanel
              open={panelOpen}
              onOpenChange={setPanelOpen}
              className="md:hidden"
            >
              <div className="space-y-1 p-4 text-sm text-muted-foreground">
                <TrashMetadata item={item} />
              </div>
            </MobileDetailPanel>
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}
