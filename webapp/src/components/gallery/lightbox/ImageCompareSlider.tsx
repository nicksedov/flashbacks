import { useCallback, useRef, useState, type MouseEvent, type TouchEvent } from "react"
import { Check, X, GripVertical } from "lucide-react"
import { Button } from "@/components/ui/button"
import { useTranslation } from "@/i18n"

interface ImageCompareSliderProps {
  originalUrl: string
  enhancedUrl: string
  onAccept: () => void
  onReject: () => void
  isAccepting?: boolean
  isRejecting?: boolean
}

export function ImageCompareSlider({
  originalUrl,
  enhancedUrl,
  onAccept,
  onReject,
  isAccepting = false,
  isRejecting = false,
}: ImageCompareSliderProps) {
  const { t } = useTranslation()
  const containerRef = useRef<HTMLDivElement>(null)
  const [sliderPosition, setSliderPosition] = useState(50) // percentage from left
  const [isDragging, setIsDragging] = useState(false)

  const getPositionPercent = useCallback((clientX: number): number => {
    if (!containerRef.current) return 50
    const rect = containerRef.current.getBoundingClientRect()
    return Math.max(2, Math.min(98, ((clientX - rect.left) / rect.width) * 100))
  }, [])

  const handleMouseDown = useCallback((e: MouseEvent) => {
    e.preventDefault()
    setIsDragging(true)
    setSliderPosition(getPositionPercent(e.clientX))
  }, [getPositionPercent])

  const handleMouseMove = useCallback((e: MouseEvent) => {
    if (!isDragging) return
    setSliderPosition(getPositionPercent(e.clientX))
  }, [isDragging, getPositionPercent])

  const handleMouseUp = useCallback(() => {
    setIsDragging(false)
  }, [])

  const handleTouchStart = useCallback((e: TouchEvent) => {
    setIsDragging(true)
    setSliderPosition(getPositionPercent(e.touches[0].clientX))
  }, [getPositionPercent])

  const handleTouchMove = useCallback((e: TouchEvent) => {
    if (!isDragging) return
    setSliderPosition(getPositionPercent(e.touches[0].clientX))
  }, [isDragging, getPositionPercent])

  const handleTouchEnd = useCallback(() => {
    setIsDragging(false)
  }, [])

  return (
    <div className="flex flex-col gap-3 w-full h-full">
      {/* Comparison area */}
      <div
        ref={containerRef}
        className="relative flex-1 overflow-hidden bg-black select-none"
        onMouseMove={handleMouseMove}
        onMouseUp={handleMouseUp}
        onMouseLeave={handleMouseUp}
        onTouchMove={handleTouchMove}
        onTouchEnd={handleTouchEnd}
      >
        {/* Original - full image behind */}
        <img
          src={originalUrl}
          alt="Original"
          className="absolute inset-0 w-full h-full object-contain"
          draggable={false}
        />

        {/* Enhanced - on top, clipped from left, showing only the right portion */}
        <img
          src={enhancedUrl}
          alt="Enhanced"
          className="absolute inset-0 w-full h-full object-contain"
          draggable={false}
          style={{ clipPath: `inset(0 0 0 ${sliderPosition}%)` }}
        />

        {/* Vertical slider line */}
        <div
          className="absolute top-0 bottom-0 w-0.5 bg-white shadow-lg cursor-ew-resize z-10"
          style={{ left: `${sliderPosition}%` }}
          onMouseDown={handleMouseDown}
          onTouchStart={handleTouchStart}
        >
          {/* Slider handle */}
          <div className="absolute top-1/2 -translate-y-1/2 -translate-x-1/2 w-8 h-8 bg-white rounded-full shadow-lg flex items-center justify-center">
            <GripVertical className="h-4 w-4 text-gray-600" />
          </div>
        </div>

        {/* Labels */}
        <div className="absolute top-3 left-3 bg-black/60 text-white text-xs px-2 py-1 rounded pointer-events-none z-20">
          {t("enhance.labelBefore")}
        </div>
        <div className="absolute top-3 right-3 bg-black/60 text-white text-xs px-2 py-1 rounded pointer-events-none z-20">
          {t("enhance.labelAfter")}
        </div>
      </div>

      {/* Action buttons */}
      <div className="flex justify-center gap-3 pb-2">
        <Button
          variant="outline"
          size="sm"
          onClick={onReject}
          disabled={isAccepting || isRejecting}
          className="gap-1.5"
        >
          <X className="h-4 w-4" />
          {t("enhance.reject")}
        </Button>
        <Button
          size="sm"
          onClick={onAccept}
          disabled={isAccepting || isRejecting}
          className="gap-1.5"
        >
          <Check className="h-4 w-4" />
          {isAccepting ? t("enhance.applying") : t("enhance.accept")}
        </Button>
      </div>
    </div>
  )
}
