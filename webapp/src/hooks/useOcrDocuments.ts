import { fetchOcrDocuments } from "@/api/endpoints"
import type { OcrDocumentDTO, OcrDocumentsResponse } from "@/types"
import { useInfiniteScroll } from "./useInfiniteScroll"

const PAGE_SIZE = 50

export function useOcrDocuments() {
  const { items, total, hasMore, isLoading, error, initialized, loadMore, reset } =
    useInfiniteScroll<OcrDocumentDTO, OcrDocumentsResponse>({
      fetchFn: (page, pageSize) => fetchOcrDocuments(page, pageSize),
      pageSize: PAGE_SIZE,
      transform: (response) => response.documents,
      responseTotal: (response) => response.total,
      responseHasNext: (response) => response.hasNextPage,
    })

  return {
    documents: items,
    totalDocuments: total,
    hasMore,
    isLoading,
    error,
    initialized,
    loadMore,
    reset,
  }
}
