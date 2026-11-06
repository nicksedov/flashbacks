import { useCallback, useEffect, useMemo, useState } from "react"
import { ArrowLeft, Check, Pencil, Trash2, UserPlus, Users } from "lucide-react"
import { useTranslation } from "@/i18n"
import type { PersonDTO, GalleryImageDTO } from "@/types"
import {
  disbandPerson,
  fetchPeople,
  fetchPersonPhotos,
  mergePeople,
  renamePerson,
} from "@/api/endpoints"
import { EmptyState } from "@/components/EmptyState"
import { LazyThumbnail } from "@/components/gallery/LazyThumbnail"
import { UnifiedLightbox } from "@/components/gallery/UnifiedLightbox"
import { Skeleton } from "@/components/ui/skeleton"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { ConfirmDialog } from "@/components/ui/confirm-dialog"
import { showToastError, showToastSuccess } from "@/lib/toast-utils"
import {
  MergePeopleDialog,
  PersonPickerDialog,
  RenamePersonDialog,
} from "@/components/people/PeopleDialogs"
import { UnassignedFacesView } from "@/components/people/UnassignedFacesView"

/**
 * Resolve the file path of a person's cover thumbnail.
 *
 * `/api/thumbnail` is a JSON API (`{"thumbnail": "<data URL>"}`), so its URL can
 * never be used as an `<img src>`; the People view renders thumbnails through
 * `LazyThumbnail` and therefore needs the raw file path of the cover face.
 */
function coverPath(person: PersonDTO): string {
  if (!person.coverThumbnail) return ""
  const queryIndex = person.coverThumbnail.indexOf("?")
  if (queryIndex === -1) return ""
  return new URLSearchParams(person.coverThumbnail.slice(queryIndex + 1)).get("path") ?? ""
}

type CatalogView = "grid" | "unassigned"

export function PeopleTab() {
  const { t } = useTranslation()
  const [people, setPeople] = useState<PersonDTO[]>([])
  const [loading, setLoading] = useState(true)
  const [selected, setSelected] = useState<PersonDTO | null>(null)
  const [photos, setPhotos] = useState<GalleryImageDTO[]>([])
  const [photosLoading, setPhotosLoading] = useState(false)
  const [lightboxPath, setLightboxPath] = useState<string | null>(null)
  const [view, setView] = useState<CatalogView>("grid")
  const [search, setSearch] = useState("")
  const [selectedIds, setSelectedIds] = useState<number[]>([])
  const [renameTarget, setRenameTarget] = useState<PersonDTO | null>(null)
  const [mergeOpen, setMergeOpen] = useState(false)
  const [mergeIntoSource, setMergeIntoSource] = useState<PersonDTO | null>(null)
  const [disbandTarget, setDisbandTarget] = useState<PersonDTO | null>(null)
  const [busy, setBusy] = useState(false)

  const reloadPeople = useCallback(() => {
    return fetchPeople()
      .then((res) => setPeople(res.people ?? []))
      .catch(() => showToastError(t("people.loadFailed")))
  }, [t])

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

  const displayName = useCallback(
    (person: PersonDTO): string => person.name || t("people.unnamed", { id: person.id }),
    [t],
  )

  const handleSelect = useCallback((person: PersonDTO) => {
    setSelected(person)
    setPhotosLoading(true)
    setPhotos([])
  }, [])

  const handleBack = useCallback(() => {
    setSelected(null)
    setPhotos([])
    setPhotosLoading(false)
    setLightboxPath(null)
  }, [])

  const handlePhotoOpen = useCallback((photo: GalleryImageDTO) => {
    setLightboxPath(photo.path)
  }, [])

  const handleLightboxClose = useCallback(() => {
    setLightboxPath(null)
  }, [])

  const toggleSelection = useCallback((personId: number) => {
    setSelectedIds((current) =>
      current.includes(personId)
        ? current.filter((id) => id !== personId)
        : [...current, personId],
    )
  }, [])

  const filteredPeople = useMemo(() => {
    const needle = search.trim().toLowerCase()
    if (!needle) return people
    return people.filter((person) => displayName(person).toLowerCase().includes(needle))
  }, [displayName, people, search])

  // The merged person is the lowest-id selection; the rest are its sources.
  const selectedPeople = useMemo(
    () =>
      people
        .filter((person) => selectedIds.includes(person.id))
        .sort((left, right) => left.id - right.id),
    [people, selectedIds],
  )

  const handleRenameSubmit = useCallback(
    async (name: string) => {
      if (!renameTarget) return
      setBusy(true)
      try {
        const updated = await renamePerson(renameTarget.id, name)
        setPeople((current) =>
          current.map((person) => (person.id === updated.id ? updated : person)),
        )
        setSelected((current) => (current && current.id === updated.id ? updated : current))
        setRenameTarget(null)
      } catch {
        showToastError(t("people.actionFailed"))
      } finally {
        setBusy(false)
      }
    },
    [renameTarget, t],
  )

  const handleMergeSubmit = useCallback(
    async (name?: string) => {
      if (selectedPeople.length < 2) return
      const [target, ...sources] = selectedPeople
      setBusy(true)
      try {
        const merged = await mergePeople({
          targetPersonId: target.id,
          sourcePersonIds: sources.map((person) => person.id),
          ...(name ? { name } : {}),
        })
        const sourceIds = new Set(sources.map((person) => person.id))
        setPeople((current) =>
          current
            .filter((person) => !sourceIds.has(person.id))
            .map((person) => (person.id === target.id ? merged : person)),
        )
        setSelectedIds([])
        setMergeOpen(false)
      } catch {
        showToastError(t("people.actionFailed"))
      } finally {
        setBusy(false)
      }
    },
    [selectedPeople, t],
  )

  // Folding one cluster into an already-known person: the cluster's faces and
  // photos move to the chosen person and the cluster row disappears.
  const handleMergeIntoSubmit = useCallback(
    async (targetPersonId: number) => {
      const source = mergeIntoSource
      if (!source || source.id === targetPersonId) return
      setBusy(true)
      try {
        const merged = await mergePeople({
          targetPersonId,
          sourcePersonIds: [source.id],
        })
        setPeople((current) =>
          current
            .filter((person) => person.id !== source.id)
            .map((person) => (person.id === targetPersonId ? merged : person)),
        )
        setSelectedIds((current) => current.filter((id) => id !== source.id))
        setSelected((current) => (current && current.id === source.id ? null : current))
        setMergeIntoSource(null)
        showToastSuccess(
          t("people.mergeIntoDone", {
            name: merged.name || t("people.unnamed", { id: merged.id }),
          }),
        )
      } catch {
        showToastError(t("people.actionFailed"))
      } finally {
        setBusy(false)
      }
    },
    [mergeIntoSource, t],
  )

  const handleDisbandConfirm = useCallback(async () => {
    if (!disbandTarget) return
    setBusy(true)
    try {
      await disbandPerson(disbandTarget.id)
      setPeople((current) => current.filter((person) => person.id !== disbandTarget.id))
      setSelectedIds((current) => current.filter((id) => id !== disbandTarget.id))
      setSelected((current) => (current && current.id === disbandTarget.id ? null : current))
      setDisbandTarget(null)
    } catch {
      showToastError(t("people.actionFailed"))
    } finally {
      setBusy(false)
    }
  }, [disbandTarget, t])

  if (selected) {
    return (
      <div data-slot="people-detail">
        <div className="mb-4 flex flex-wrap items-center gap-2">
          <Button variant="ghost" size="sm" className="gap-2" onClick={handleBack}>
            <ArrowLeft className="h-4 w-4" />
            {t("people.back")}
          </Button>
          <Button
            variant="outline"
            size="sm"
            data-slot="person-detail-rename"
            className="gap-2"
            onClick={() => setRenameTarget(selected)}
          >
            <Pencil className="h-4 w-4" />
            {t("people.rename")}
          </Button>
          <Button
            variant="outline"
            size="sm"
            data-slot="person-detail-merge-into"
            className="gap-2"
            disabled={people.length < 2 || busy}
            onClick={() => setMergeIntoSource(selected)}
          >
            <UserPlus className="h-4 w-4" />
            {t("people.mergeInto")}
          </Button>
          <Button
            variant="destructive"
            size="sm"
            data-slot="person-detail-disband"
            className="gap-2"
            onClick={() => setDisbandTarget(selected)}
          >
            <Trash2 className="h-4 w-4" />
            {t("people.disband")}
          </Button>
        </div>
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
              <button
                key={photo.id}
                type="button"
                data-slot="person-photo"
                className="group aspect-square overflow-hidden rounded-lg border transition-all hover:ring-2 hover:ring-ring focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                onClick={() => handlePhotoOpen(photo)}
              >
                <LazyThumbnail path={photo.path} fileName={photo.fileName} loadingIcon="spinner" />
              </button>
            ))}
          </div>
        )}

        {/* Photos of a person open in the People mode of the shared lightbox */}
        <UnifiedLightbox
          imagePath={lightboxPath}
          initialMode="people"
          onClose={handleLightboxClose}
          onPeopleChanged={() => void reloadPeople()}
        />

        {renameTarget && (
          <RenamePersonDialog
            initialName={renameTarget.name}
            loading={busy}
            onCancel={() => setRenameTarget(null)}
            onSubmit={(name) => void handleRenameSubmit(name)}
          />
        )}

        {disbandTarget && (
          <ConfirmDialog
            open
            onOpenChange={(open) => {
              if (!open && !busy) setDisbandTarget(null)
            }}
            title={t("people.disbandTitle")}
            description={t("people.disbandDescription")}
            confirmLabel={t("people.disbandConfirm")}
            cancelLabel={t("people.cancel")}
            onConfirm={() => void handleDisbandConfirm()}
            loading={busy}
            destructive
          />
        )}
      </div>
    )
  }

  return (
    <div>
      <div className="mb-6 flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-2xl font-semibold">{t("people.title")}</h1>
        <div className="flex gap-2" role="group">
          <Button
            type="button"
            data-slot="people-view-grid"
            variant={view === "grid" ? "default" : "outline"}
            aria-pressed={view === "grid"}
            onClick={() => setView("grid")}
          >
            {t("people.viewGrid")}
          </Button>
          <Button
            type="button"
            data-slot="people-view-unassigned"
            variant={view === "unassigned" ? "default" : "outline"}
            aria-pressed={view === "unassigned"}
            onClick={() => setView("unassigned")}
          >
            {t("people.viewUnassigned")}
          </Button>
        </div>
      </div>

      {view === "unassigned" ? (
        <UnassignedFacesView people={people} onPeopleChanged={() => void reloadPeople()} />
      ) : (
        <>
          <div className="mb-4 flex flex-wrap items-center gap-2">
            <Input
              data-slot="people-search"
              value={search}
              placeholder={t("people.searchPlaceholder")}
              className="max-w-xs"
              onChange={(event) => setSearch(event.target.value)}
            />
            {selectedIds.length > 0 && (
              <span data-slot="people-selected-count" className="text-sm text-muted-foreground">
                {t("people.selectedCount", { count: selectedIds.length })}
              </span>
            )}
            <Button
              type="button"
              variant="outline"
              data-slot="people-merge"
              disabled={selectedIds.length < 2 || busy}
              onClick={() => setMergeOpen(true)}
            >
              {t("people.merge")}
            </Button>
          </div>

          {loading ? (
            <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6">
              {Array.from({ length: 6 }).map((_, i) => (
                <Skeleton key={i} className="aspect-square w-full rounded-lg" />
              ))}
            </div>
          ) : people.length === 0 ? (
            <EmptyState icon={Users} title={t("people.emptyTitle")} hint={t("people.emptyHint")} />
          ) : filteredPeople.length === 0 ? (
            <p data-slot="people-no-matches" className="text-sm text-muted-foreground">
              {t("people.noMatches", { query: search })}
            </p>
          ) : (
            <div
              data-slot="people-grid"
              className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6"
            >
              {filteredPeople.map((person) => {
                const name = displayName(person)
                const isSelected = selectedIds.includes(person.id)
                return (
                  <div key={person.id} className="group relative overflow-hidden rounded-lg border">
                    <button
                      type="button"
                      data-slot="person-tile"
                      aria-label={name}
                      className="block w-full text-left"
                      onClick={() => handleSelect(person)}
                    >
                      <div className="aspect-square w-full bg-muted">
                        {coverPath(person) ? (
                          <LazyThumbnail
                            path={coverPath(person)}
                            fileName={name}
                            loadingIcon="spinner"
                          />
                        ) : null}
                      </div>
                      <div className="p-2">
                        <p className="truncate text-sm font-medium">{name}</p>
                        <p className="text-xs text-muted-foreground">
                          {t("people.photoCount", { count: person.photoCount })}
                        </p>
                      </div>
                    </button>
                    <div className="absolute right-1 top-1 flex max-w-[calc(100%-0.5rem)] flex-wrap justify-end gap-1">
                      <Button
                        type="button"
                        size="icon"
                        variant={isSelected ? "default" : "outline"}
                        data-slot="person-select"
                        aria-pressed={isSelected}
                        aria-label={t("people.selectPerson", { name })}
                        className="h-11 w-11 md:h-8 md:w-8"
                        onClick={() => toggleSelection(person.id)}
                      >
                        <Check className="h-4 w-4" />
                      </Button>
                      <Button
                        type="button"
                        size="icon"
                        variant="outline"
                        data-slot="person-merge-into"
                        aria-label={t("people.mergeInto")}
                        title={t("people.mergeInto")}
                        className="h-11 w-11 md:h-8 md:w-8"
                        disabled={people.length < 2 || busy}
                        onClick={() => setMergeIntoSource(person)}
                      >
                        <UserPlus className="h-4 w-4" />
                      </Button>
                      <Button
                        type="button"
                        size="icon"
                        variant="outline"
                        data-slot="person-rename"
                        aria-label={t("people.rename")}
                        className="h-11 w-11 md:h-8 md:w-8"
                        onClick={() => setRenameTarget(person)}
                      >
                        <Pencil className="h-4 w-4" />
                      </Button>
                      <Button
                        type="button"
                        size="icon"
                        variant="outline"
                        data-slot="person-disband"
                        aria-label={t("people.disband")}
                        className="h-11 w-11 md:h-8 md:w-8"
                        onClick={() => setDisbandTarget(person)}
                      >
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    </div>
                  </div>
                )
              })}
            </div>
          )}
        </>
      )}

      {renameTarget && (
        <RenamePersonDialog
          initialName={renameTarget.name}
          loading={busy}
          onCancel={() => setRenameTarget(null)}
          onSubmit={(name) => void handleRenameSubmit(name)}
        />
      )}

      {mergeIntoSource && (
        <PersonPickerDialog
          title={t("people.mergeIntoTitle", { name: displayName(mergeIntoSource) })}
          description={t("people.mergeIntoHint")}
          people={people.filter((person) => person.id !== mergeIntoSource.id)}
          loading={busy}
          onCancel={() => setMergeIntoSource(null)}
          onPick={(personId) => void handleMergeIntoSubmit(personId)}
        />
      )}

      {mergeOpen && selectedPeople.length > 1 && (
        <MergePeopleDialog
          people={selectedPeople}
          loading={busy}
          onCancel={() => setMergeOpen(false)}
          onSubmit={(name) => void handleMergeSubmit(name)}
        />
      )}

      {disbandTarget && (
        <ConfirmDialog
          open
          onOpenChange={(open) => {
            if (!open && !busy) setDisbandTarget(null)
          }}
          title={t("people.disbandTitle")}
          description={t("people.disbandDescription")}
          confirmLabel={t("people.disbandConfirm")}
          cancelLabel={t("people.cancel")}
          onConfirm={() => void handleDisbandConfirm()}
          loading={busy}
          destructive
        />
      )}
    </div>
  )
}
