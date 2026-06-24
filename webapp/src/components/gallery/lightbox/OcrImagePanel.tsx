import { Loader2 } from "lucide-react"
import { useTranslation } from "@/i18n"
import type { OcrDataResponse } from "@/types"

interface OcrImagePanelProps {
  imageUrl: string
  ocrData: OcrDataResponse | null
  isTextDocument: boolean
  loading: boolean
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

  return (
    <div className={className ?? "w-[50%] flex items-center justify-center p-8 relative h-full"}>
      {loading && (
        <div className="absolute inset-0 flex items-center justify-center">
          <Loader2 className="h-8 w-8 animate-spin text-white" />
        </div>
      )}

      <div className="relative inline-block">
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
            className="absolute inset-0 pointer-events-none"
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
      </div>
    </div>
  )
}
