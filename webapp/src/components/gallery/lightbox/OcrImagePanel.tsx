import { Loader2 } from "lucide-react"
import { useTranslation } from "@/i18n"
import type { TranslationKey } from "@/i18n"
import type { OcrDataResponse } from "@/types"

interface OcrImagePanelProps {
  imageUrl: string
  ocrData: OcrDataResponse | null
  isTextDocument: boolean
  loading: boolean
  /** True when the displayed image is the OCR-rotated (preprocessed) variant. */
  isRotated?: boolean
  imageRef: React.RefObject<HTMLImageElement | null>
  displayDimensions: { width: number; height: number } | null
  imageLoaded: boolean
  handleImageLoad: () => void
  className?: string
}

export function OcrImagePanel({
  imageUrl,
  ocrData,
  isTextDocument,
  loading,
  isRotated = false,
  imageRef,
  displayDimensions,
  imageLoaded,
  handleImageLoad,
  className,
}: OcrImagePanelProps) {
  const { t } = useTranslation()

  const scaleX = ocrData && displayDimensions && ocrData.boundingBoxWidth
    ? displayDimensions.width / ocrData.boundingBoxWidth
    : 1
  const scaleY = ocrData && displayDimensions && ocrData.boundingBoxHeight
    ? displayDimensions.height / ocrData.boundingBoxHeight
    : 1

  // While OCR data is being fetched or the (possibly rotated) image is still
  // loading, dim the panel and show a spinner so the user can see the stage.
  const showLoadingOverlay = loading || (!!imageUrl && !imageLoaded)
  const statusKey: TranslationKey | null = showLoadingOverlay
    ? loading
      ? "lightbox.ocr.loading"
      : isRotated
        ? "lightbox.ocr.preparing"
        : "lightbox.ocr.loadingImage"
    : null

  return (
    <div className={className ?? "w-[50%] flex items-center justify-center relative h-full"}>
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

      {isTextDocument && ocrData && ocrData.boxes.length > 0 && imageLoaded && displayDimensions && (
        <div
          className="absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2 pointer-events-none"
          style={{
            width: displayDimensions.width,
            height: displayDimensions.height,
          }}
        >
          {ocrData.boxes.map((box, index) => (
            <div
              key={index}
              className="absolute border-2 border-yellow-400/70 bg-yellow-400/10"
              style={{
                left: box.x * scaleX,
                top: box.y * scaleY,
                width: box.width * scaleX,
                height: box.height * scaleY,
              }}
              title={`${box.word} (${(box.confidence * 100).toFixed(0)}%)`}
            ></div>
          ))}
        </div>
      )}

      {showLoadingOverlay && (
        <div className="absolute inset-0 z-10 flex flex-col items-center justify-center gap-2 bg-black/50">
          <Loader2 className="h-8 w-8 animate-spin text-white" />
          {statusKey && <p className="text-xs text-white/90">{t(statusKey)}</p>}
        </div>
      )}
    </div>
  )
}
