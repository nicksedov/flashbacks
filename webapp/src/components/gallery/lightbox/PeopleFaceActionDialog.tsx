import { useTranslation } from "@/i18n"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import type { ImageFaceDTO } from "@/types"

interface PeopleFaceActionDialogProps {
  face: ImageFaceDTO
  /** The person's display label (name or localized placeholder), empty when unassigned. */
  personLabel: string
  busy?: boolean
  onCancel: () => void
  onAssign: () => void
  onDetach: () => void
  onCreate: () => void
}

/**
 * The actions of one selected face in the lightbox People mode. An assigned
 * face can move to another person or leave its own; an unassigned face can join
 * a person or become the first face of a new one.
 */
export function PeopleFaceActionDialog({
  face,
  personLabel,
  busy = false,
  onCancel,
  onAssign,
  onDetach,
  onCreate,
}: PeopleFaceActionDialogProps) {
  const { t } = useTranslation()
  const assigned = face.personId !== 0

  return (
    <Dialog open onOpenChange={(next) => !next && !busy && onCancel()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("lightbox.people.faceTitle")}</DialogTitle>
          <DialogDescription>
            {assigned && personLabel ? personLabel : t("lightbox.people.unassignedFace")}
          </DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-2">
          <Button type="button" data-slot="face-assign" onClick={onAssign} disabled={busy}>
            {t("lightbox.people.assign")}
          </Button>
          {assigned && (
            <Button
              type="button"
              variant="outline"
              data-slot="face-detach"
              onClick={onDetach}
              disabled={busy}
            >
              {personLabel
                ? t("lightbox.people.detach", { name: personLabel })
                : t("lightbox.people.detachUnnamed")}
            </Button>
          )}
          {!assigned && (
            <Button
              type="button"
              variant="outline"
              data-slot="face-create"
              onClick={onCreate}
              disabled={busy}
            >
              {t("lightbox.people.create")}
            </Button>
          )}
        </div>
        <DialogFooter>
          <Button type="button" variant="ghost" data-slot="face-cancel" onClick={onCancel} disabled={busy}>
            {t("people.cancel")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
