import { useState, useEffect, useCallback, useRef } from "react"

interface UseImageDimensionsReturn {
  imageRef: React.RefObject<HTMLImageElement | null>
  displayDimensions: { width: number; height: number } | null
  imageLoaded: boolean
  handleImageLoad: () => void
}

export function useImageDimensions(imageUrl?: string): UseImageDimensionsReturn {
  const [imageLoaded, setImageLoaded] = useState(false)
  const [displayDimensions, setDisplayDimensions] = useState<{ width: number; height: number } | null>(null)
  const imageRef = useRef<HTMLImageElement>(null)
  const prevUrlRef = useRef<string | undefined>(imageUrl)

  // Reset state when imageUrl changes
  useEffect(() => {
    if (prevUrlRef.current !== imageUrl) {
      prevUrlRef.current = imageUrl
      setImageLoaded(false)
      setDisplayDimensions(null)
    }
  }, [imageUrl])

  const handleImageLoad = useCallback(() => {
    // Use requestAnimationFrame to ensure dimensions are read after paint
    requestAnimationFrame(() => {
      if (imageRef.current) {
        const { clientWidth, clientHeight } = imageRef.current
        if (clientWidth > 0 && clientHeight > 0) {
          setDisplayDimensions({ width: clientWidth, height: clientHeight })
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

  return { imageRef, displayDimensions, imageLoaded, handleImageLoad }
}
