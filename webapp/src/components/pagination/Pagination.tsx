import { Button } from "@/components/ui/button"
import { ChevronFirst, ChevronLast, ChevronLeft, ChevronRight } from "lucide-react"
import { useTranslation } from "@/i18n"

interface PaginationProps {
  currentPage: number
  totalPages: number
  hasPrevPage: boolean
  hasNextPage: boolean
  onPageChange: (page: number) => void
}

export function Pagination({
  currentPage,
  totalPages,
  hasPrevPage,
  hasNextPage,
  onPageChange,
}: PaginationProps) {
  const { t } = useTranslation()

  if (totalPages <= 1) return null

  return (
    <div className="flex items-center justify-center gap-2 py-4">
      <Button
        variant="outline"
        size="sm"
        onClick={() => onPageChange(1)}
        disabled={!hasPrevPage}
      >
        <ChevronFirst className="h-4 w-4" />
        <span className="sr-only sm:not-sr-only sm:ml-1">{t("pagination.first")}</span>
      </Button>
      <Button
        variant="outline"
        size="sm"
        onClick={() => onPageChange(currentPage - 1)}
        disabled={!hasPrevPage}
      >
        <ChevronLeft className="h-4 w-4" />
        <span className="sr-only sm:not-sr-only sm:ml-1">{t("pagination.prev")}</span>
      </Button>
      <span className="text-sm text-muted-foreground px-3">
        {t("pagination.pageInfo", { current: currentPage, total: totalPages })}
      </span>
      <Button
        variant="outline"
        size="sm"
        onClick={() => onPageChange(currentPage + 1)}
        disabled={!hasNextPage}
      >
        <span className="sr-only sm:not-sr-only sm:mr-1">{t("pagination.next")}</span>
        <ChevronRight className="h-4 w-4" />
      </Button>
      <Button
        variant="outline"
        size="sm"
        onClick={() => onPageChange(totalPages)}
        disabled={!hasNextPage}
      >
        <span className="sr-only sm:not-sr-only sm:mr-1">{t("pagination.last")}</span>
        <ChevronLast className="h-4 w-4" />
      </Button>
    </div>
  )
}
