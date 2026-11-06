import { Loader2, Pencil, Users } from "lucide-react"
import { useTranslation } from "@/i18n"
import { EmptyState } from "@/components/EmptyState"
import { ErrorBanner } from "@/components/ui/error-banner"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import type { ImagePerson } from "@/hooks/useImageFaces"

interface PeoplePanelProps {
  /** The people of the photo, numbered; the numbers match the box labels. */
  people: ImagePerson[]
  loading: boolean
  error: boolean
  /** Opens the rename action for a person of this photo. */
  onRename?: (personId: number) => void
  className?: string
}

/**
 * The right-hand panel of the lightbox People mode: one entry per person on the
 * photo, carrying the same number that is drawn on that person's boxes.
 */
export function PeoplePanel({ people, loading, error, onRename, className }: PeoplePanelProps) {
  const { t } = useTranslation()

  return (
    <div className={cn("w-full bg-card p-4 h-full flex flex-col", className)}>
      {loading ? (
        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          <Loader2 className="h-4 w-4 animate-spin" />
          {t("lightbox.ocr.loading")}
        </div>
      ) : error ? (
        <ErrorBanner message={t("lightbox.people.loadFailed")} />
      ) : people.length === 0 ? (
        <EmptyState
          icon={Users}
          title={t("lightbox.people.emptyTitle")}
          hint={t("lightbox.people.emptyHint")}
        />
      ) : (
        <ul data-testid="people-list" className="min-h-0 flex-1 space-y-2 overflow-y-auto">
          {people.map((person) => (
            <li
              key={person.personId}
              data-testid="people-list-item"
              className="flex items-center gap-3 rounded-md border p-2"
            >
              <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-primary text-xs font-semibold text-primary-foreground">
                {person.number}
              </span>
              <span className="truncate text-sm font-medium">
                {person.name || t("people.unnamed", { id: person.personId })}
              </span>
              {onRename && (
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  data-testid="people-list-rename"
                  aria-label={t("lightbox.people.rename")}
                  className="ml-auto h-11 w-11 shrink-0 md:h-8 md:w-8"
                  onClick={() => onRename(person.personId)}
                >
                  <Pencil className="h-4 w-4" />
                </Button>
              )}
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
