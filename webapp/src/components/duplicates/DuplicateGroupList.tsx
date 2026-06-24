import { DuplicateGroupCard } from "./DuplicateGroupCard"
import type { DuplicateGroupDTO, FileDTO } from "@/types"

interface DuplicateGroupListProps {
  groups: DuplicateGroupDTO[]
  allFiles: FileDTO[]
  isSelected: (path: string) => boolean
  onToggleFile: (path: string) => void
  onSelectFolder: (dirPath: string, allFiles: FileDTO[]) => void
}

export function DuplicateGroupList({
  groups,
  allFiles,
  isSelected,
  onToggleFile,
  onSelectFolder,
}: DuplicateGroupListProps) {
  return (
    <div className="space-y-3">
      {groups.map((group) => (
        <DuplicateGroupCard
          key={`${group.hash}-${group.size}`}
          group={group}
          isSelected={isSelected}
          onToggleFile={onToggleFile}
          onSelectFolder={(dirPath) => onSelectFolder(dirPath, allFiles)}
        />
      ))}
    </div>
  )
}
