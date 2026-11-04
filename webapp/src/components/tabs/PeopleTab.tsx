import { useCallback, useEffect, useState } from "react"
import { Users, ArrowLeft } from "lucide-react"
import { useTranslation } from "@/i18n"
import type { PersonDTO, GalleryImageDTO } from "@/types"
import { fetchPeople, fetchPersonPhotos } from "@/api/endpoints"
import { EmptyState } from "@/components/EmptyState"
import { Skeleton } from "@/components/ui/skeleton"
import { Button } from "@/components/ui/button"
import { buildImageUrl } from "@/utils/buildImageUrl"

const API_BASE = import.meta.env.VITE_API_URL || ""

export function PeopleTab() {
  const { t } = useTranslation()
  const [people, setPeople] = useState<PersonDTO[]>([])
  const [loading, setLoading] = useState(true)
  const [selected, setSelected] = useState<PersonDTO | null>(null)
  const [photos, setPhotos] = useState<GalleryImageDTO[]>([])
  const [photosLoading, setPhotosLoading] = useState(false)

  useEffect(() => {
    let cancelled = false
    fetchPeople()
      .then((res) => {
        if (!cancelled) setPeople(res.people ?? [])
      })
      .catch(() => {
        if (!cancelled) setPeople([])
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [])

  useEffect(() => {
    if (!selected) return
    let cancelled = false
    fetchPersonPhotos(selected.id, 1, 200)
      .then((res) => {
        if (!cancelled) setPhotos(res.images ?? [])
      })
      .catch(() => {
        if (!cancelled) setPhotos([])
      })
      .finally(() => {
        if (!cancelled) setPhotosLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [selected])

  const handleSelect = useCallback((person: PersonDTO) => {
    setSelected(person)
    setPhotosLoading(true)
    setPhotos([])
  }, [])

  const handleBack = useCallback(() => {
    setSelected(null)
    setPhotos([])
    setPhotosLoading(false)
  }, [])

  const displayName = (person: PersonDTO): string =>
    person.name || t("people.unnamed", { id: person.id })

  if (selected) {
    return (
      <div data-slot="people-detail">
        <Button variant="ghost" size="sm" className="mb-4 gap-2" onClick={handleBack}>
          <ArrowLeft className="h-4 w-4" />
          {t("people.back")}
        </Button>
        <h1 className="mb-4 text-xl font-semibold">{displayName(selected)}</h1>
        {photosLoading ? (
          <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6">
            {Array.from({ length: 6 }).map((_, i) => (
              <Skeleton key={i} className="aspect-square w-full" />
            ))}
          </div>
        ) : (
          <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6">
            {photos.map((photo) => (
              <div key={photo.id} className="overflow-hidden rounded-lg border">
                <img
                  src={buildImageUrl(photo.path, "/api/thumbnail")}
                  alt={photo.fileName}
                  loading="lazy"
                  className="aspect-square w-full object-cover"
                />
              </div>
            ))}
          </div>
        )}
      </div>
    )
  }

  return (
    <div>
      <h1 className="mb-6 text-2xl font-semibold">{t("people.title")}</h1>
      {loading ? (
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6">
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="aspect-square w-full rounded-lg" />
          ))}
        </div>
      ) : people.length === 0 ? (
        <EmptyState icon={Users} title={t("people.emptyTitle")} hint={t("people.emptyHint")} />
      ) : (
        <div data-slot="people-grid" className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6">
          {people.map((person) => (
            <button
              key={person.id}
              type="button"
              data-slot="person-tile"
              className="group overflow-hidden rounded-lg border text-left"
              onClick={() => handleSelect(person)}
            >
              <img
                src={person.coverThumbnail ? `${API_BASE}${person.coverThumbnail}` : undefined}
                alt={displayName(person)}
                loading="lazy"
                className="aspect-square w-full bg-muted object-cover"
              />
              <div className="p-2">
                <p className="truncate text-sm font-medium">{displayName(person)}</p>
                <p className="text-xs text-muted-foreground">{t("people.photoCount", { count: person.photoCount })}</p>
              </div>
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
