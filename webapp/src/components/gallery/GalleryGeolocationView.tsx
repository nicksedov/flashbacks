import { useCallback, useEffect, useRef, useState } from "react"
import { MapContainer, TileLayer, Marker, Popup, useMapEvents, useMap } from "react-leaflet"
import L from "leaflet"
import { GalleryImageGrid } from "@/components/gallery/GalleryImageGrid"
import { useGeoClusters } from "@/hooks/useGeoClusters"
import { useGeoImages } from "@/hooks/useGeoImages"
import { useTranslation } from "@/i18n"
import { ArrowLeft, MapPin, ImageIcon } from "lucide-react"
import { useIntersectionObserver } from "@/hooks/useIntersectionObserver"
import { PaginationFooter } from "@/components/ui/pagination-footer"
import { ViewHeader } from "@/components/ui/view-header"
import type { GalleryImageDTO, GeoCluster } from "@/types"
import "@/lib/leaflet-icon-fix"

interface GalleryGeolocationViewProps {
  onImageClick: (image: GalleryImageDTO) => void
  onImageDownload?: (image: GalleryImageDTO) => void
  onImageDelete?: (image: GalleryImageDTO, removeThumbnail: () => void) => void
}

type GeoViewMode = "map" | "grid"

interface GeoBounds {
  minLat: number
  maxLat: number
  minLng: number
  maxLng: number
}

function createClusterIcon(cluster: GeoCluster): L.DivIcon {
  const { count } = cluster
  let size = 32
  if (count > 100) size = 48
  else if (count > 10) size = 40

  return L.divIcon({
    html: `<div style="
      width: ${size}px;
      height: ${size}px;
      border-radius: 50%;
      background: var(--primary, hsl(270 80% 60%));
      color: var(--primary-foreground, white);
      display: flex;
      align-items: center;
      justify-content: center;
      font-weight: bold;
      font-size: ${size < 40 ? 12 : 14}px;
      box-shadow: 0 2px 4px rgba(0,0,0,0.3);
      cursor: pointer;
    ">${count}</div>`,
    className: "geo-cluster-marker",
    iconSize: [size, size],
    iconAnchor: [size / 2, size / 2],
  })
}

function MapEventHandler({ onBoundsChange }: { onBoundsChange: (bounds: GeoBounds) => void }) {
  const map = useMapEvents({
    moveend: () => {
      const bounds = map.getBounds()
      onBoundsChange({
        minLat: bounds.getSouth(),
        maxLat: bounds.getNorth(),
        minLng: bounds.getWest(),
        maxLng: bounds.getEast(),
      })
    },
    zoomend: () => {
      const bounds = map.getBounds()
      onBoundsChange({
        minLat: bounds.getSouth(),
        maxLat: bounds.getNorth(),
        minLng: bounds.getWest(),
        maxLng: bounds.getEast(),
      })
    },
  })

  // Report initial bounds
  useEffect(() => {
    const bounds = map.getBounds()
    onBoundsChange({
      minLat: bounds.getSouth(),
      maxLat: bounds.getNorth(),
      minLng: bounds.getWest(),
      maxLng: bounds.getEast(),
    })
  }, [map, onBoundsChange])

  return null
}

// Prevents the map from showing the world more than once by setting
// maxBounds and dynamically calculating minZoom based on container width
function MapBoundsController() {
  const map = useMap()

  useEffect(() => {
    // Restrict panning to world bounds so the map can't scroll past the edges
    map.setMaxBounds(L.latLngBounds(L.latLng(-90, -180), L.latLng(90, 180)))

    const updateMinZoom = () => {
      const container = map.getContainer()
      const width = container.clientWidth
      const height = container.clientHeight
      // World is 256px at zoom 0; find the zoom where world fits both dimensions
      const minZoom = Math.ceil(Math.min(Math.log2(width / 256), Math.log2(height / 256)))
      map.setMinZoom(minZoom)
      // Re-fit view if current zoom is now below the new minimum
      if (map.getZoom() < minZoom) {
        map.setZoom(minZoom)
      }
    }

    updateMinZoom()

    const observer = new ResizeObserver(updateMinZoom)
    observer.observe(map.getContainer())
    return () => observer.disconnect()
  }, [map])

  return null
}

// MapEvents component to track zoom — must be declared outside the parent component
function MapZoomTracker({ onZoomChange }: { onZoomChange: (zoom: number) => void }) {
  const map = useMap()
  useMapEvents({
    zoomend: () => {
      onZoomChange(map.getZoom())
    },
  })
  return null
}

export function GalleryGeolocationView({ onImageClick, onImageDownload, onImageDelete }: GalleryGeolocationViewProps) {
  const { t } = useTranslation()
  const [viewMode, setViewMode] = useState<GeoViewMode>("map")
  const [selectedClusterId, setSelectedClusterId] = useState<string | null>(null)
  const [mapBounds, setMapBounds] = useState<GeoBounds | null>(null)
  const [mapZoom, setMapZoom] = useState(2)
  const [mapSize, setMapSize] = useState({ width: 800, height: 600 })
  const mapContainerRef = useRef<HTMLDivElement>(null)
  const [hasAnyGeoImages, setHasAnyGeoImages] = useState(false)

  const { clusters, totalImages, isLoading: clustersLoading, initialized: clustersInitialized } = useGeoClusters({
    bounds: viewMode === "map" ? (mapBounds || { minLat: -90, maxLat: 90, minLng: -180, maxLng: 180 }) : null,
    zoom: mapZoom,
    width: mapSize.width,
    height: mapSize.height,
  })

  // Track if we ever found any geo images globally
  useEffect(() => {
    if (totalImages > 0) {
      /* eslint-disable react-hooks/set-state-in-effect */
      setHasAnyGeoImages(true)
      /* eslint-enable react-hooks/set-state-in-effect */
    }
  }, [totalImages])

  const { images, hasMore, isLoading: imagesLoading, loadMore, reset: resetImages, removeImage: removeGeoImage } = useGeoImages(
    viewMode === "grid" ? selectedClusterId : null
  )

  // Update map size from container
  useEffect(() => {
    const updateSize = () => {
      if (mapContainerRef.current) {
        const rect = mapContainerRef.current.getBoundingClientRect()
        setMapSize({ width: Math.round(rect.width), height: Math.round(rect.height) })
      }
    }

    updateSize()
    const observer = new ResizeObserver(updateSize)
    if (mapContainerRef.current) {
      observer.observe(mapContainerRef.current)
    }
    return () => observer.disconnect()
  }, [])

  // Reset images when switching to grid view or changing cluster selection
  const prevSelectedClusterIdRef = useRef<string | null>(null)
  useEffect(() => {
    if (viewMode === "grid" && selectedClusterId) {
      if (prevSelectedClusterIdRef.current !== selectedClusterId) {
        prevSelectedClusterIdRef.current = selectedClusterId
        resetImages()
        // Trigger initial load after reset
        setTimeout(() => loadMore(), 0)
      }
    }
  }, [viewMode, selectedClusterId, resetImages, loadMore])

  const handleBoundsChange = useCallback((bounds: GeoBounds) => {
    setMapBounds(bounds)
  }, [])

  const handleClusterClick = useCallback((cluster: GeoCluster) => {
    setSelectedClusterId(cluster.id)
    setViewMode("grid")
  }, [])

  const handleBackToMap = useCallback(() => {
    setViewMode("map")
    setSelectedClusterId(null)
  }, [])

  // Infinite scroll for grid view
  const sentinelRef = useIntersectionObserver({
    onIntersect: loadMore,
    enabled: viewMode === "grid" && hasMore && !imagesLoading,
    dependencies: [viewMode, hasMore, imagesLoading, loadMore],
  })

  if (viewMode === "grid" && selectedClusterId) {
    return (
      <div className="space-y-4">
        <button
          onClick={handleBackToMap}
          className="flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground transition-colors"
        >
          <ArrowLeft className="h-4 w-4" />
          {t("geolocation.backToMap")}
        </button>

        <ViewHeader
          icon={MapPin}
          textKey="geolocation.totalGeoImages"
          textValues={{ count: images.length.toString() }}
        />

        {images.length === 0 && !imagesLoading ? (
          <div className="rounded-lg border border-dashed p-12 text-center">
            <ImageIcon className="mx-auto h-10 w-10 text-muted-foreground/50" />
            <p className="mt-2 text-sm font-medium text-muted-foreground">
              {t("geolocation.noGpsData")}
            </p>
            <p className="text-xs text-muted-foreground/70">
              {t("geolocation.noGpsDataHint")}
            </p>
          </div>
        ) : (
          <>
            <GalleryImageGrid
              images={images}
              onImageClick={onImageClick}
              onImageDownload={onImageDownload}
              onImageDelete={(image) => onImageDelete?.(image, () => removeGeoImage(image.id))}
            />
            <div ref={sentinelRef} className="h-4" />
            <PaginationFooter
              isLoading={imagesLoading}
              hasMore={hasMore}
              totalCount={images.length}
            />
          </>
        )}
      </div>
    )
  }

  // Map view
  return (
    <div className="space-y-2">
      {totalImages > 0 && clustersInitialized && (
        <div className="flex items-center gap-2">
          <MapPin className="h-5 w-5 text-muted-foreground" />
          <span className="text-sm text-muted-foreground">
            {t("geolocation.totalGeoImages", { count: totalImages.toString() })}
          </span>
        </div>
      )}

      {totalImages === 0 && !clustersLoading && clustersInitialized && !hasAnyGeoImages ? (
        <div className="rounded-lg border border-dashed p-12 text-center">
          <ImageIcon className="mx-auto h-10 w-10 text-muted-foreground/50" />
          <p className="mt-2 text-sm font-medium text-muted-foreground">
            {t("geolocation.noGpsData")}
          </p>
          <p className="text-xs text-muted-foreground/70">
            {t("geolocation.noGpsDataHint")}
          </p>
        </div>
      ) : (
        <div ref={mapContainerRef} className="h-[calc(100vh-8rem)] rounded-lg border overflow-hidden">
          <MapContainer
            center={[20, 0]}
            zoom={2}
            maxBoundsViscosity={1.0}
            worldCopyJump={true}
            style={{ height: "100%", width: "100%" }}
            zoomControl={true}
            scrollWheelZoom={true}
            dragging={true}
          >
            <TileLayer
              attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
              url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
            />
            <MapBoundsController />
            <MapZoomTracker onZoomChange={setMapZoom} />
            <MapEventHandler onBoundsChange={handleBoundsChange} />

            {clusters.map((cluster) => (
              <Marker
                key={cluster.id}
                position={[cluster.latitude, cluster.longitude]}
                icon={createClusterIcon(cluster)}
                eventHandlers={{
                  click: () => handleClusterClick(cluster),
                }}
              >
                <Popup>
                  <div className="text-center">
                    <p className="font-semibold">{cluster.count} image(s)</p>
                    <p className="text-xs text-muted-foreground">Click to view</p>
                  </div>
                </Popup>
              </Marker>
            ))}
          </MapContainer>
        </div>
      )}

    </div>
  )
}
