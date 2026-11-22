import { useCallback, useEffect, useMemo, useState } from "react"
import { useTranslation } from "@/i18n"
import { fetchPersonPhotos } from "@/api/endpoints"
import { personNameSuggestions } from "@/lib/person-name"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { SuggestionChip } from "@/components/ui/suggestion-chip"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import type { PersonDTO } from "@/types"

/**
 * Total number of distinct photos the selected people cover.
 *
 * The dialog is mounted for the current selection, so the preview starts as an
 * upper bound (the sum) and narrows to the exact union once the photo lists are
 * read. A failed lookup keeps the fallback rather than showing zero.
 */
function useDistinctPhotoCount(people: PersonDTO[]) {
  const fallback = useMemo(
    () => people.reduce((total, person) => total + person.photoCount, 0),
    [people],
  )
  const [count, setCount] = useState<number | null>(null)

  useEffect(() => {
    let cancelled = false
    Promise.all(
      people.map((person) => fetchPersonPhotos(person.id, 1, 200).catch(() => null)),
    ).then((results) => {
      if (cancelled) return
      const paths = new Set<string>()
      let resolved = 0
      for (const result of results) {
        if (!result) continue
        resolved++
        for (const image of result.images ?? []) paths.add(image.path)
      }
      if (resolved === people.length) setCount(paths.size)
    })
    return () => {
      cancelled = true
    }
  }, [people])

  return count ?? fallback
}

/** Shared dialog shell: closing it always means "cancel". */
function useCancelHandler(onCancel: () => void, loading: boolean) {
  return useCallback(
    (next: boolean) => {
      if (!next && !loading) onCancel()
    },
    [loading, onCancel],
  )
}

interface RenamePersonDialogProps {
  initialName: string
  /** The catalog the name suggestions come from; omit it to offer none. */
  people?: PersonDTO[]
  /** The person being renamed, which is never suggested. */
  excludePersonId?: number
  loading?: boolean
  onCancel: () => void
  onSubmit: (name: string) => void
}

/** Set or clear one person's display name, with catalog names as suggestions. */
export function RenamePersonDialog({
  initialName,
  people = [],
  excludePersonId,
  loading = false,
  onCancel,
  onSubmit,
}: RenamePersonDialogProps) {
  const { t, language } = useTranslation()
  const [name, setName] = useState(initialName)
  // Until the user edits the field it keeps the seeded name, and the list shows
  // the most-photographed names instead of filtering itself down to nothing.
  const [pristine, setPristine] = useState(true)
  const handleOpenChange = useCancelHandler(onCancel, loading)
  // Filling a suggestion is only a shortcut for typing: the input stays
  // authoritative and Save is the only action that sends a request.
  const suggestions = useMemo(
    () =>
      personNameSuggestions(people, pristine ? "" : name, {
        excludeIds: excludePersonId === undefined ? [] : [excludePersonId],
        locale: language,
      }),
    [excludePersonId, language, name, people, pristine],
  )

  return (
    <Dialog open onOpenChange={handleOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("people.renameTitle")}</DialogTitle>
          <DialogDescription>{t("people.renameHint")}</DialogDescription>
        </DialogHeader>
        <div className="space-y-2">
          <Label htmlFor="person-rename-name">{t("people.renameNameLabel")}</Label>
          <Input
            id="person-rename-name"
            data-slot="rename-input"
            value={name}
            maxLength={200}
            onChange={(event) => {
              setPristine(false)
              setName(event.target.value)
            }}
          />
          {suggestions.length > 0 && (
            <div data-slot="rename-suggestions" className="flex flex-wrap gap-1.5">
              {suggestions.map((person) => (
                <SuggestionChip
                  key={person.id}
                  label={person.name}
                  className="max-w-full"
                  onClick={() => {
                    setPristine(false)
                    setName(person.name)
                  }}
                />
              ))}
            </div>
          )}
        </div>
        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            data-slot="rename-cancel"
            onClick={onCancel}
            disabled={loading}
          >
            {t("people.cancel")}
          </Button>
          <Button
            type="button"
            data-slot="rename-save"
            onClick={() => onSubmit(name)}
            disabled={loading}
          >
            {t("people.save")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

/** What the merge alternative would fold into the person that holds the name. */
export type PersonNameConflictMode = "merge-person" | "merge-selection" | "attach-faces"

interface PersonNameConflictDialogProps {
  /** The name the user typed; it is never saved. */
  pendingName: string
  /** The person that already holds that name. */
  conflict: PersonDTO
  mode: PersonNameConflictMode
  loading?: boolean
  /** Merge the pending person, selection or faces into `conflict`. */
  onMerge: () => void
  /** Go back to the name input and save nothing. */
  onChangeName: () => void
  /** Close the flow and save nothing. */
  onCancel: () => void
}

/**
 * Resolve a taken person name. The only two ways forward are the ones the user
 * asked for: join the person that already holds the name, or pick another name.
 */
export function PersonNameConflictDialog({
  pendingName,
  conflict,
  mode,
  loading = false,
  onMerge,
  onChangeName,
  onCancel,
}: PersonNameConflictDialogProps) {
  const { t } = useTranslation()
  const handleOpenChange = useCancelHandler(onCancel, loading)

  const mergeLabel =
    mode === "merge-selection"
      ? t("people.nameConflictMergeSelection", { name: conflict.name })
      : mode === "attach-faces"
        ? t("people.nameConflictAttachFaces", { name: conflict.name })
        : t("people.nameConflictMergePerson", { name: conflict.name })

  return (
    <Dialog open onOpenChange={handleOpenChange}>
      <DialogContent data-slot="name-conflict">
        <DialogHeader>
          <DialogTitle data-slot="name-conflict-title">{t("people.nameConflictTitle")}</DialogTitle>
          <DialogDescription>
            {t("people.nameConflictDescription", {
              name: pendingName.trim(),
              count: t("people.photoCount", { count: conflict.photoCount }),
            })}
          </DialogDescription>
        </DialogHeader>
        <p data-slot="name-conflict-hint" className="text-sm text-muted-foreground">
          {t("people.nameConflictHint")}
        </p>
        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            data-slot="name-conflict-cancel"
            onClick={onCancel}
            disabled={loading}
          >
            {t("people.cancel")}
          </Button>
          <Button
            type="button"
            variant="outline"
            data-slot="name-conflict-rename"
            onClick={onChangeName}
            disabled={loading}
          >
            {t("people.nameConflictChangeName")}
          </Button>
          <Button
            type="button"
            data-slot="name-conflict-merge"
            onClick={onMerge}
            disabled={loading}
          >
            {mergeLabel}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

interface MergePeopleDialogProps {
  /** The selected people, target first. */
  people: PersonDTO[]
  loading?: boolean
  onCancel: () => void
  onSubmit: (name?: string) => void
}

/** Confirm merging the selected people, previewing name and photo count. */
export function MergePeopleDialog({
  people,
  loading = false,
  onCancel,
  onSubmit,
}: MergePeopleDialogProps) {
  const { t } = useTranslation()
  const [name, setName] = useState("")
  const distinctCount = useDistinctPhotoCount(people)
  const handleOpenChange = useCancelHandler(onCancel, loading)

  const label = useCallback(
    (person: PersonDTO) => person.name || t("people.unnamed", { id: person.id }),
    [t],
  )

  const target = people[0]
  const previewName = name.trim() || (target ? label(target) : "")

  return (
    <Dialog open onOpenChange={handleOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("people.mergeTitle")}</DialogTitle>
          <DialogDescription>{t("people.mergeDescription")}</DialogDescription>
        </DialogHeader>
        <div className="space-y-3">
          <p data-slot="merge-preview" className="text-sm font-medium">
            {t("people.mergePreview", { name: previewName, count: distinctCount })}
          </p>
          <div className="space-y-2">
            <Label htmlFor="person-merge-name">{t("people.mergeNameLabel")}</Label>
            <Input
              id="person-merge-name"
              data-slot="merge-input"
              value={name}
              maxLength={200}
              onChange={(event) => setName(event.target.value)}
            />
          </div>
        </div>
        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            data-slot="merge-cancel"
            onClick={onCancel}
            disabled={loading}
          >
            {t("people.cancel")}
          </Button>
          <Button
            type="button"
            data-slot="merge-confirm"
            onClick={() => onSubmit(name.trim() ? name.trim() : undefined)}
            disabled={loading}
          >
            {t("people.mergeConfirm")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

interface CreatePersonDialogProps {
  loading?: boolean
  onCancel: () => void
  onSubmit: (name: string) => void
}

/** Create a person from the current face selection. */
export function CreatePersonDialog({
  loading = false,
  onCancel,
  onSubmit,
}: CreatePersonDialogProps) {
  const { t } = useTranslation()
  const [name, setName] = useState("")
  const handleOpenChange = useCancelHandler(onCancel, loading)

  return (
    <Dialog open onOpenChange={handleOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("people.createTitle")}</DialogTitle>
        </DialogHeader>
        <div className="space-y-2">
          <Label htmlFor="person-create-name">{t("people.createNameLabel")}</Label>
          <Input
            id="person-create-name"
            data-slot="create-input"
            value={name}
            maxLength={200}
            onChange={(event) => setName(event.target.value)}
          />
        </div>
        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            data-slot="create-cancel"
            onClick={onCancel}
            disabled={loading}
          >
            {t("people.cancel")}
          </Button>
          <Button
            type="button"
            data-slot="create-confirm"
            onClick={() => onSubmit(name)}
            disabled={loading}
          >
            {t("people.createConfirm")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

interface PersonPickerDialogProps {
  title: string
  /** Optional explanation of what picking a person does. */
  description?: string
  /** The whole catalog, not only the people already on a photo. */
  people: PersonDTO[]
  loading?: boolean
  /** Omit to hide the "create a new person" section. */
  onCreate?: (name: string) => void
  onCancel: () => void
  onPick: (personId: number) => void
}

/** Searchable list of every person, with an optional create-new path. */
export function PersonPickerDialog({
  title,
  description,
  people,
  loading = false,
  onCreate,
  onCancel,
  onPick,
}: PersonPickerDialogProps) {
  const { t } = useTranslation()
  const [query, setQuery] = useState("")
  const [newName, setNewName] = useState("")
  const handleOpenChange = useCancelHandler(onCancel, loading)

  const label = useCallback(
    (person: PersonDTO) => person.name || t("people.unnamed", { id: person.id }),
    [t],
  )

  const filtered = useMemo(() => {
    const needle = query.trim().toLowerCase()
    if (!needle) return people
    return people.filter((person) => label(person).toLowerCase().includes(needle))
  }, [label, people, query])

  return (
    <Dialog open onOpenChange={handleOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          {description && (
            <DialogDescription data-slot="picker-description">{description}</DialogDescription>
          )}
        </DialogHeader>
        <div className="space-y-3">
          <Input
            data-slot="picker-search"
            value={query}
            placeholder={t("people.pickerSearchPlaceholder")}
            onChange={(event) => setQuery(event.target.value)}
          />
          {filtered.length === 0 ? (
            <p data-slot="picker-empty" className="text-sm text-muted-foreground">
              {t("people.pickerEmpty")}
            </p>
          ) : (
            <ul className="max-h-64 space-y-1 overflow-y-auto">
              {filtered.map((person) => (
                <li key={person.id}>
                  <button
                    type="button"
                    data-slot="picker-person"
                    className="flex w-full items-center justify-between gap-2 rounded-md border px-3 py-2 text-left text-sm hover:bg-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                    onClick={() => onPick(person.id)}
                    disabled={loading}
                  >
                    <span className="truncate font-medium">{label(person)}</span>
                    <span className="shrink-0 text-xs text-muted-foreground">
                      {t("people.photoCount", { count: person.photoCount })}
                    </span>
                  </button>
                </li>
              ))}
            </ul>
          )}

          {onCreate && (
            <div className="space-y-2 border-t pt-3">
              <Label htmlFor="picker-create-name">{t("people.pickerCreate")}</Label>
              <div className="flex gap-2">
                <Input
                  id="picker-create-name"
                  data-slot="picker-create-name"
                  value={newName}
                  maxLength={200}
                  onChange={(event) => setNewName(event.target.value)}
                />
                <Button
                  type="button"
                  data-slot="picker-create"
                  onClick={() => onCreate(newName)}
                  disabled={loading}
                >
                  {t("people.createConfirm")}
                </Button>
              </div>
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}
