import { useCallback, useMemo, useState } from "react"
import type { FileDTO } from "@/types"

export function useSelection() {
  const [selected, setSelected] = useState<Set<string>>(new Set())

  const toggle = useCallback((path: string) => {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(path)) {
        next.delete(path)
      } else {
        next.add(path)
      }
      return next
    })
  }, [])

  const selectByFolder = useCallback((dirPath: string, allFiles: FileDTO[]) => {
    setSelected((prev) => {
      const next = new Set(prev)
      for (const f of allFiles) {
        if (f.dirPath === dirPath) {
          next.add(f.path)
        }
      }
      return next
    })
  }, [])

  const selectAll = useCallback((paths: string[]) => {
    setSelected(new Set(paths))
  }, [])

  const reset = useCallback(() => {
    setSelected(new Set())
  }, [])

  const isSelected = useCallback((path: string) => selected.has(path), [selected])

  const selectedPaths = useMemo(() => Array.from(selected), [selected])

  return {
    selected,
    selectedPaths,
    selectedCount: selected.size,
    toggle,
    selectByFolder,
    selectAll,
    reset,
    isSelected,
  }
}
