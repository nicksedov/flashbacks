import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { useTranslation } from "@/i18n"
import { GeoSearchForm } from "./GeoSearchForm"
import type { GalleryImageDTO } from "@/types"

interface BulkGeoDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  date: string
  label: string
  imagesWithoutGps: GalleryImageDTO[]
  onSaved: () => void
}

export function BulkGeoDialog({
  open,
  onOpenChange,
  date,
  label,
  imagesWithoutGps,
  onSaved,
}: BulkGeoDialogProps) {
  const { t } = useTranslation()

  const paths = imagesWithoutGps.map((img) => img.path)

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-[34rem]">
        <DialogHeader>
          <DialogTitle>{t("geo.bulkSetTitle", { date: label })}</DialogTitle>
          <DialogDescription>
            {t("geo.bulkSetDescription", { count: imagesWithoutGps.length })}
          </DialogDescription>
        </DialogHeader>

        <div className="py-2">
          <GeoSearchForm
            date={date}
            paths={paths}
            affectedCount={imagesWithoutGps.length}
            onGpsSaved={() => {
              onSaved()
              onOpenChange(false)
            }}
          />
        </div>
      </DialogContent>
    </Dialog>
  )
}
