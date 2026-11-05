import { useState, useEffect, useCallback, useRef } from "react"

interface DisplayDimensions {
  width: number
  height: number
}

interface UseImageDimensionsReturn {
  imageRef: React.RefObject<HTMLImageElement | null>
  displayDimensions: DisplayDimensions | null
  /**
   * The image's own pixel size, read from the loaded element. Overlays whose
   * coordinates are stored in the source frame (for example face boxes) scale by
   * this, not by the rendered size.
   */
  naturalDimensions: DisplayDimensions | null
  imageLoaded: boolean
  handleImageLoad: () => void
}

export function useImageDimensions(imageUrl?: string): UseImageDimensionsReturn {
  const [imageLoaded, setImageLoaded] = useState(false)
  const [displayDimensions, setDisplayDimensions] = useState<DisplayDimensions | null>(null)
  const [naturalDimensions, setNaturalDimensions] = useState<DisplayDimensions | null>(null)
  const imageRef = useRef<HTMLImageElement>(null)
  const prevUrlRef = useRef<string | undefined>(imageUrl)

  // Reset state when imageUrl changes
  useEffect(() => {
    if (prevUrlRef.current !== imageUrl) {
      prevUrlRef.current = imageUrl
      setImageLoaded(false)
      setDisplayDimensions(null)
      setNaturalDimensions(null)
    }
  }, [imageUrl])

  const handleImageLoad = useCallback(() => {
    // Use requestAnimationFrame to ensure dimensions are read after paint
    requestAnimationFrame(() => {
      if (imageRef.current) {
        const { clientWidth, clientHeight, naturalWidth, naturalHeight } = imageRef.current
        if (clientWidth > 0 && clientHeight > 0) {
          setDisplayDimensions({ width: clientWidth, height: clientHeight })
          if (naturalWidth > 0 && naturalHeight > 0) {
            setNaturalDimensions({ width: naturalWidth, height: naturalHeight })
          }
          setImageLoaded(true)
        }
      }
    })
  }, [])

  useEffect(() => {
    if (!imageLoaded || !imageRef.current) return

    const updateDimensions = () => {
      if (imageRef.current) {
        setDisplayDimensions({
          width: imageRef.current.clientWidth,
          height: imageRef.current.clientHeight,
        })
      }
    }

    updateDimensions()
    window.addEventListener("resize", updateDimensions)
    return () => window.removeEventListener("resize", updateDimensions)
  }, [imageLoaded])

  return { imageRef, displayDimensions, naturalDimensions, imageLoaded, handleImageLoad }
}
