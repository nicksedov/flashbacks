import { Loader2 } from "lucide-react"
import { useTranslation } from "@/i18n"
import { cn } from "@/lib/utils"
import type { ImagePerson } from "@/hooks/useImageFaces"
import type { ImageFaceDTO } from "@/types"

interface PeopleImagePanelProps {
  imageUrl: string
  /** Every detected face of the image, including the ones no person claims. */
  faces: ImageFaceDTO[]
  /** The image's people, numbered; the same numbers are drawn on the boxes. */
  people: ImagePerson[]
  loading: boolean
  imageRef: React.RefObject<HTMLImageElement | null>
  /** The size the image is rendered at. */
  displayDimensions: { width: number; height: number } | null
  /** The image's own pixel size — the frame the box coordinates are stored in. */
  sourceDimensions: { width: number; height: number } | null
  imageLoaded: boolean
  handleImageLoad: () => void
  /** The face whose actions are open, drawn with a selected outline. */
  selectedFaceId?: number | null
  /** Turns each box into a button that opens the face actions. */
  onSelectFace?: (face: ImageFaceDTO) => void
  className?: string
}

/**
 * The image stage of the lightbox People mode: the photo with one bounding box
 * per detected face. A face clustered into a person carries that person's number,
 * a face nobody claims is drawn muted and unnumbered.
 */
export function PeopleImagePanel({
  imageUrl,
  faces,
  people,
  loading,
  imageRef,
  displayDimensions,
  sourceDimensions,
  imageLoaded,
  handleImageLoad,
  selectedFaceId = null,
  onSelectFace,
  className,
}: PeopleImagePanelProps) {
  const { t } = useTranslation()

  const scaleX =
    sourceDimensions && displayDimensions && sourceDimensions.width > 0
      ? displayDimensions.width / sourceDimensions.width
      : 0
  const scaleY =
    sourceDimensions && displayDimensions && sourceDimensions.height > 0
      ? displayDimensions.height / sourceDimensions.height
      : 0

  const numbers = new Map(people.map((person) => [person.personId, person.number]))
  const showBoxes = imageLoaded && displayDimensions !== null && scaleX > 0 && scaleY > 0
  const showLoadingOverlay = loading || (!!imageUrl && !imageLoaded)

  return (
    // The stage matches the other lightbox modes: the same dark, flexible area.
    <div className={cn("bg-black", className ?? "w-[50%] flex items-center justify-center relative h-full")}>
      {imageUrl ? (
        <img
          ref={imageRef}
          src={imageUrl}
          alt={t("lightbox.alt")}
          className="max-w-full max-h-full object-contain"
          onLoad={handleImageLoad}
        />
      ) : loading && (
        <div className="w-[600px] h-[400px] bg-muted/30 rounded flex items-center justify-center">
          <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
        </div>
      )}

      {showBoxes && displayDimensions && (
        <div
          data-testid="people-face-overlay"
          className="absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2 pointer-events-none"
          style={{ width: displayDimensions.width, height: displayDimensions.height }}
        >
          {faces.map((face) => {
            const number = numbers.get(face.personId)
            const numbered = number !== undefined
            const selected = selectedFaceId === face.id
            return (
              <button
                key={face.id}
                type="button"
                data-testid="people-face-box"
                data-face-id={face.id}
                data-numbered={numbered ? "true" : "false"}
                aria-pressed={selected}
                aria-label={
                  numbered
                    ? t("lightbox.people.faceBox", { number })
                    : t("lightbox.people.unassignedFace")
                }
                className={cn(
                  "absolute border-2 text-left pointer-events-auto focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
                  numbered
                    ? "border-primary bg-primary/10"
                    : "border-muted-foreground/60 bg-muted-foreground/10",
                  selected && "ring-2 ring-ring",
                )}
                style={{
                  left: face.x * scaleX,
                  top: face.y * scaleY,
                  width: face.width * scaleX,
                  height: face.height * scaleY,
                }}
                onClick={() => onSelectFace?.(face)}
              >
                {numbered && (
                  <span
                    data-testid="people-face-number"
                    className="absolute -left-0.5 -top-0.5 flex h-5 min-w-5 items-center justify-center rounded-sm bg-primary px-1 text-xs font-semibold text-primary-foreground"
                  >
                    {number}
                  </span>
                )}
              </button>
            )
          })}
        </div>
      )}

      {showLoadingOverlay && (
        <div className="absolute inset-0 z-10 flex flex-col items-center justify-center gap-2 bg-black/50">
          <Loader2 className="h-8 w-8 animate-spin text-white" />
          <p className="text-xs text-white/90">{t("lightbox.ocr.loadingImage")}</p>
        </div>
      )}
    </div>
  )
}
