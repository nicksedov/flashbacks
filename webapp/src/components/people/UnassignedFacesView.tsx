import { useCallback, useEffect, useState } from "react"
import { Loader2, ScanFace } from "lucide-react"
import { useTranslation } from "@/i18n"
import { assignFaceToPerson, createPerson, fetchUnassignedFaces } from "@/api/endpoints"
import { buildImageUrl } from "@/utils/buildImageUrl"
import { EmptyState } from "@/components/EmptyState"
import { ErrorBanner } from "@/components/ui/error-banner"
import { Button } from "@/components/ui/button"
import { showToastError } from "@/lib/toast-utils"
import { cn } from "@/lib/utils"
import { CreatePersonDialog, PersonPickerDialog } from "@/components/people/PeopleDialogs"
import type { PersonDTO, UnassignedFaceDTO } from "@/types"

const PAGE_SIZE = 24

interface UnassignedFaceTileProps {
  face: UnassignedFaceDTO
  selected: boolean
  onToggle: (faceId: number) => void
  /**
   * The analysed image's pixel size. Tests provide it; in the app it comes from
   * the loaded photo, which is the frame the box coordinates are stored in.
   */
  naturalDimensions?: { width: number; height: number } | null
}

/**
 * One unassigned face: its photo with the face box drawn over it, selectable.
 *
 * The box is positioned in percentages of the analysed frame, so it stays
 * correct at any rendered size without measuring the element.
 */
export function UnassignedFaceTile({
  face,
  selected,
  onToggle,
  naturalDimensions,
}: UnassignedFaceTileProps) {
  const [measured, setMeasured] = useState<{ width: number; height: number } | null>(null)
  const natural = naturalDimensions ?? measured

  return (
    <button
      type="button"
      data-slot="unassigned-face"
      data-face-id={face.id}
      aria-pressed={selected}
      aria-label={face.fileName}
      className={cn(
        "relative block overflow-hidden rounded-lg border text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
        selected && "ring-2 ring-primary",
      )}
      onClick={() => onToggle(face.id)}
    >
      <img
        src={buildImageUrl(face.path, "/api/image")}
        alt={face.fileName}
        loading="lazy"
        className="block h-auto w-full"
        onLoad={(event) =>
          setMeasured({
            width: event.currentTarget.naturalWidth,
            height: event.currentTarget.naturalHeight,
          })
        }
      />
      {natural && natural.width > 0 && natural.height > 0 && (
        <span
          data-testid="unassigned-face-box"
          className="absolute border-2 border-primary bg-primary/10"
          style={{
            left: `${(face.x / natural.width) * 100}%`,
            top: `${(face.y / natural.height) * 100}%`,
            width: `${(face.width / natural.width) * 100}%`,
            height: `${(face.height / natural.height) * 100}%`,
          }}
        />
      )}
      <span className="absolute bottom-1 left-1 rounded-sm bg-black/60 px-1 text-[10px] font-medium text-white">
        {Math.round(face.confidence * 100)}%
      </span>
    </button>
  )
}

interface UnassignedFacesViewProps {
  /** The whole catalog, for the attach picker. */
  people: PersonDTO[]
  /** Called after a face leaves the view, so the People grid can refresh. */
  onPeopleChanged: () => void
}

/**
 * The Unassigned faces mode of the People tab: a paginated grid of faces no
 * person claims, where a selection can be attached to an existing person or
 * become a new person.
 */
export function UnassignedFacesView({ people, onPeopleChanged }: UnassignedFacesViewProps) {
  const { t } = useTranslation()
  const [faces, setFaces] = useState<UnassignedFaceDTO[]>([])
  const [page, setPage] = useState(1)
  const [hasNextPage, setHasNextPage] = useState(false)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)
  const [selectedIds, setSelectedIds] = useState<number[]>([])
  const [pickerOpen, setPickerOpen] = useState(false)
  const [createOpen, setCreateOpen] = useState(false)
  const [busy, setBusy] = useState(false)

  const loadPage = useCallback((targetPage: number) => {
    return fetchUnassignedFaces(targetPage, PAGE_SIZE)
      .then((response) => {
        setFaces(response.faces ?? [])
        setHasNextPage(response.hasNextPage)
        setPage(response.currentPage)
        setError(false)
      })
      .catch(() => {
        setFaces([])
        setHasNextPage(false)
        setError(true)
      })
      .finally(() => setLoading(false))
  }, [])

  useEffect(() => {
    void loadPage(1)
  }, [loadPage])

  const toggleFace = useCallback((faceId: number) => {
    setSelectedIds((current) =>
      current.includes(faceId)
        ? current.filter((id) => id !== faceId)
        : [...current, faceId],
    )
  }, [])

  const handleAttach = useCallback(
    async (personId: number) => {
      setBusy(true)
      try {
        await Promise.all(selectedIds.map((faceId) => assignFaceToPerson(faceId, personId)))
        setPickerOpen(false)
        setSelectedIds([])
        onPeopleChanged()
        setLoading(true)
        await loadPage(page)
      } catch {
        showToastError(t("people.actionFailed"))
      } finally {
        setBusy(false)
      }
    },
    [loadPage, onPeopleChanged, page, selectedIds, t],
  )

  const handleCreate = useCallback(
    async (name: string) => {
      setBusy(true)
      try {
        await createPerson({
          ...(name.trim() ? { name: name.trim() } : {}),
          faceIds: selectedIds,
        })
        setCreateOpen(false)
        setSelectedIds([])
        onPeopleChanged()
        setLoading(true)
        await loadPage(page)
      } catch {
        showToastError(t("people.actionFailed"))
      } finally {
        setBusy(false)
      }
    },
    [loadPage, onPeopleChanged, page, selectedIds, t],
  )

  return (
    <div data-slot="unassigned-view">
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 className="text-lg font-semibold">{t("people.unassignedTitle")}</h2>
          <p className="text-sm text-muted-foreground">{t("people.unassignedHint")}</p>
        </div>
        <div className="flex items-center gap-2">
          {selectedIds.length > 0 && (
            <span data-slot="unassigned-selected" className="text-sm text-muted-foreground">
              {t("people.unassignedSelected", { count: selectedIds.length })}
            </span>
          )}
          <Button
            type="button"
            data-slot="unassigned-attach"
            variant="outline"
            disabled={selectedIds.length === 0 || busy}
            onClick={() => setPickerOpen(true)}
          >
            {t("people.attach")}
          </Button>
          <Button
            type="button"
            data-slot="unassigned-create"
            disabled={selectedIds.length === 0 || busy}
            onClick={() => setCreateOpen(true)}
          >
            {t("people.createConfirm")}
          </Button>
        </div>
      </div>

      {loading ? (
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <Loader2 className="h-4 w-4 animate-spin" />
          {t("lightbox.ocr.loading")}
        </div>
      ) : error ? (
        <ErrorBanner message={t("people.unassignedLoadFailed")} />
      ) : faces.length === 0 ? (
        <EmptyState
          icon={ScanFace}
          title={t("people.unassignedEmptyTitle")}
          hint={t("people.unassignedEmptyHint")}
        />
      ) : (
        <>
          <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6">
            {faces.map((face) => (
              <UnassignedFaceTile
                key={face.id}
                face={face}
                selected={selectedIds.includes(face.id)}
                onToggle={toggleFace}
              />
            ))}
          </div>
          {hasNextPage && (
            <div className="mt-4 flex justify-center">
              <Button
                type="button"
                variant="outline"
                data-slot="unassigned-next"
                disabled={loading}
                onClick={() => {
                  setLoading(true)
                  void loadPage(page + 1)
                }}
              >
                {t("people.loadMore")}
              </Button>
            </div>
          )}
        </>
      )}

      {pickerOpen && (
        <PersonPickerDialog
          title={t("people.attachTitle")}
          people={people}
          loading={busy}
          onCancel={() => setPickerOpen(false)}
          onPick={(personId) => void handleAttach(personId)}
        />
      )}

      {createOpen && (
        <CreatePersonDialog
          loading={busy}
          onCancel={() => setCreateOpen(false)}
          onSubmit={(name) => void handleCreate(name)}
        />
      )}
    </div>
  )
}
