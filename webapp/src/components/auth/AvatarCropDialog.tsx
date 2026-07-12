import { useCallback, useEffect, useRef, useState } from "react"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { useTranslation } from "@/i18n"

interface AvatarCropDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  imageFile: File | null
  onApply: (blob: Blob) => void
}

const CROP_SIZE = 200
const OUTPUT_SIZE = 256
const CANVAS_SIZE = 300

export function AvatarCropDialog({ open, onOpenChange, imageFile, onApply }: AvatarCropDialogProps) {
  const { t } = useTranslation()
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const [image, setImage] = useState<HTMLImageElement | null>(null)
  const [zoom, setZoom] = useState(1.0)
  const [offsetX, setOffsetX] = useState(0)
  const [offsetY, setOffsetY] = useState(0)
  const [isDragging, setIsDragging] = useState(false)
  const dragStart = useRef({ x: 0, y: 0, ox: 0, oy: 0 })

  // Load image when file changes
  useEffect(() => {
    if (!imageFile) return
    const url = URL.createObjectURL(imageFile)
    const img = new Image()
    img.onload = () => {
      setImage(img)
      setZoom(1.0)
      setOffsetX(0)
      setOffsetY(0)
    }
    img.src = url
    return () => URL.revokeObjectURL(url)
  }, [imageFile])

  const draw = useCallback(() => {
    const canvas = canvasRef.current
    if (!canvas || !image) return
    const ctx = canvas.getContext("2d")
    if (!ctx) return

    const cx = CANVAS_SIZE / 2
    const cy = CANVAS_SIZE / 2
    const radius = CROP_SIZE / 2

    // Calculate image draw dimensions (fit inside canvas at zoom=1)
    const scale = Math.min(CANVAS_SIZE / image.width, CANVAS_SIZE / image.height)
    const drawW = image.width * scale * zoom
    const drawH = image.height * scale * zoom
    const drawX = cx - drawW / 2 + offsetX
    const drawY = cy - drawH / 2 + offsetY

    // Clear
    ctx.clearRect(0, 0, CANVAS_SIZE, CANVAS_SIZE)

    // Draw image
    ctx.drawImage(image, drawX, drawY, drawW, drawH)

    // Draw dark overlay with circular cutout
    ctx.save()
    ctx.globalCompositeOperation = "destination-in"
    ctx.beginPath()
    ctx.arc(cx, cy, radius, 0, Math.PI * 2)
    ctx.fillStyle = "white"
    ctx.fill()
    ctx.restore()

    // Draw overlay outside circle
    ctx.save()
    ctx.globalCompositeOperation = "source-over"
    ctx.beginPath()
    ctx.rect(0, 0, CANVAS_SIZE, CANVAS_SIZE)
    ctx.arc(cx, cy, radius, 0, Math.PI * 2, true)
    ctx.fillStyle = "rgba(0, 0, 0, 0.6)"
    ctx.fill()
    ctx.restore()

    // Draw circle border
    ctx.beginPath()
    ctx.arc(cx, cy, radius, 0, Math.PI * 2)
    ctx.strokeStyle = "rgba(255, 255, 255, 0.5)"
    ctx.lineWidth = 2
    ctx.stroke()
  }, [image, zoom, offsetX, offsetY])

  // Redraw on state changes
  useEffect(() => {
    draw()
  }, [draw])

  // Clamp offsets so image always covers the crop circle
  const clampOffsets = useCallback((ox: number, oy: number, currentZoom: number) => {
    if (!image) return { ox: 0, oy: 0 }
    const scale = Math.min(CANVAS_SIZE / image.width, CANVAS_SIZE / image.height)
    const drawW = image.width * scale * currentZoom
    const drawH = image.height * scale * currentZoom
    const maxOx = Math.max(0, (drawW - CROP_SIZE) / 2)
    const maxOy = Math.max(0, (drawH - CROP_SIZE) / 2)
    return {
      ox: Math.max(-maxOx, Math.min(maxOx, ox)),
      oy: Math.max(-maxOy, Math.min(maxOy, oy)),
    }
  }, [image])

  const handlePointerDown = (e: React.PointerEvent) => {
    setIsDragging(true)
    dragStart.current = { x: e.clientX, y: e.clientY, ox: offsetX, oy: offsetY }
    ;(e.target as HTMLElement).setPointerCapture(e.pointerId)
  }

  const handlePointerMove = (e: React.PointerEvent) => {
    if (!isDragging) return
    const dx = e.clientX - dragStart.current.x
    const dy = e.clientY - dragStart.current.y
    const clamped = clampOffsets(dragStart.current.ox + dx, dragStart.current.oy + dy, zoom)
    setOffsetX(clamped.ox)
    setOffsetY(clamped.oy)
  }

  const handlePointerUp = () => {
    setIsDragging(false)
  }

  const handleZoomChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const newZoom = parseFloat(e.target.value)
    setZoom(newZoom)
    const clamped = clampOffsets(offsetX, offsetY, newZoom)
    setOffsetX(clamped.ox)
    setOffsetY(clamped.oy)
  }

  const handleApply = () => {
    if (!image) return
    const offscreen = document.createElement("canvas")
    offscreen.width = OUTPUT_SIZE
    offscreen.height = OUTPUT_SIZE
    const ctx = offscreen.getContext("2d")
    if (!ctx) return

    const cx = CANVAS_SIZE / 2
    const cy = CANVAS_SIZE / 2
    const radius = CROP_SIZE / 2

    const scale = Math.min(CANVAS_SIZE / image.width, CANVAS_SIZE / image.height)
    const drawW = image.width * scale * zoom
    const drawH = image.height * scale * zoom
    const drawX = cx - drawW / 2 + offsetX
    const drawY = cy - drawH / 2 + offsetY

    // Clip to circle and draw scaled to output size
    ctx.beginPath()
    ctx.arc(OUTPUT_SIZE / 2, OUTPUT_SIZE / 2, OUTPUT_SIZE / 2, 0, Math.PI * 2)
    ctx.clip()

    const outputScale = OUTPUT_SIZE / CROP_SIZE
    const outX = (drawX - (cx - radius)) * outputScale
    const outY = (drawY - (cy - radius)) * outputScale
    ctx.drawImage(image, outX, outY, drawW * outputScale, drawH * outputScale)

    offscreen.toBlob((blob) => {
      if (blob) {
        onApply(blob)
      }
    }, "image/png")
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[400px]">
        <DialogHeader>
          <DialogTitle>{t("avatar.crop")}</DialogTitle>
          <DialogDescription>{t("avatar.cropDescription")}</DialogDescription>
        </DialogHeader>
        <div className="flex flex-col items-center gap-4">
          <canvas
            ref={canvasRef}
            width={CANVAS_SIZE}
            height={CANVAS_SIZE}
            className="rounded-lg cursor-grab active:cursor-grabbing touch-none"
            onPointerDown={handlePointerDown}
            onPointerMove={handlePointerMove}
            onPointerUp={handlePointerUp}
          />
          <div className="w-full flex items-center gap-3">
            <span className="text-sm text-muted-foreground whitespace-nowrap">{t("avatar.zoom")}</span>
            <input
              type="range"
              min="1"
              max="3"
              step="0.05"
              value={zoom}
              onChange={handleZoomChange}
              className="w-full"
            />
            <span className="text-sm font-mono w-10 text-right">{zoom.toFixed(1)}×</span>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t("common.cancel")}
          </Button>
          <Button onClick={handleApply} disabled={!image}>
            {t("avatar.apply")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
