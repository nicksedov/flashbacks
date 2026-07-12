import { useState, useCallback, useEffect, useRef, type ReactNode } from "react"
import { ChevronRight, ChevronDown, Folder, FolderOpen, Loader2, Check } from "lucide-react"
import { cn } from "@/lib/utils"
import type { SubdirEntry } from "@/types"
import { fetchSubdirs } from "@/api/endpoints"

interface TreeNode {
  name: string
  path: string
  children: TreeNode[] | null // null = not yet loaded, [] = no children
  isRoot: boolean
}

interface GalleryFolderTreeProps {
  /** Flat list of root gallery folder paths */
  rootPaths: string[]
  /** Currently selected folder path */
  selectedPath: string | null
  /** Called when a folder is selected */
  onSelect: (path: string) => void
  /** Whether the tree is in loading state */
  isLoading?: boolean
  /** Optional className */
  className?: string
}

/**
 * A recursive, lazy-loading folder tree for gallery folder selection.
 * Top-level nodes come from the gallery root folders.
 * Children are loaded on expand via the backend API.
 */
export function GalleryFolderTree({
  rootPaths,
  selectedPath,
  onSelect,
  isLoading = false,
  className,
}: GalleryFolderTreeProps) {
  const [treeNodes, setTreeNodes] = useState<TreeNode[]>(() =>
    rootPaths.map((p) => ({
      name: p.split("/").filter(Boolean).pop() || p,
      path: p,
      children: null,
      isRoot: true,
    }))
  )
  const [expandedPaths, setExpandedPaths] = useState<Set<string>>(new Set())
  const [loadingPaths, setLoadingPaths] = useState<Set<string>>(new Set())
  const treeNodesRef = useRef(treeNodes)

  // Sync treeNodes to ref after render
  useEffect(() => {
    treeNodesRef.current = treeNodes
  }, [treeNodes])

  /**
   * Auto-expand ancestor folders of the selected path, and reload a parent's
   * children when a newly created subfolder is detected (exists server-side
   * but is not yet present in the cached tree).
   */
  /* eslint-disable react-hooks/set-state-in-effect */
  useEffect(() => {
    if (!selectedPath) return

    const parts = selectedPath.split("/").filter(Boolean)
    if (parts.length <= 1) return // root-level path, nothing to expand

    // Compute all ancestor paths (excluding the selected path itself)
    const ancestors: string[] = []
    let acc = ""
    for (const part of parts) {
      acc += "/" + part
      if (acc !== selectedPath) ancestors.push(acc)
    }

    // Expand all ancestors
    setExpandedPaths((prev) => {
      const next = new Set(prev)
      for (const a of ancestors) next.add(a)
      return next
    })

    const parentPath = ancestors[ancestors.length - 1]

    // Find the parent node using the ref (avoids stale closure)
    const findNode = (nodes: TreeNode[], path: string): TreeNode | null => {
      for (const n of nodes) {
        if (n.path === path) return n
        if (n.children) {
          const found = findNode(n.children, path)
          if (found) return found
        }
      }
      return null
    }

    const currentNodes = treeNodesRef.current
    const parentNode = findNode(currentNodes, parentPath)
    if (!parentNode) return

    // If children aren't loaded yet, load them
    if (parentNode.children === null) {
      setLoadingPaths((prev) => new Set(prev).add(parentPath))

      fetchSubdirs(parentPath)
        .then((res) => {
          const children: TreeNode[] = res.subdirs.map(
            (entry: SubdirEntry) => ({
              name: entry.name,
              path: entry.path,
              children: null,
              isRoot: false,
            })
          )
          setTreeNodes((prev) => updateNodeChildren(prev, parentPath, children))
        })
        .catch(() => {
          setTreeNodes((prev) => updateNodeChildren(prev, parentPath, []))
        })
        .finally(() => {
          setLoadingPaths((prev) => {
            const next = new Set(prev)
            next.delete(parentPath)
            return next
          })
        })
      return
    }

    // Children are already loaded – check if the selected child is among them
    const childExists = parentNode.children.some((c) => c.path === selectedPath)
    if (childExists) return // nothing to reload

    // The selected child is not in the cache – reload (new folder was created)
    setLoadingPaths((prev) => new Set(prev).add(parentPath))

    fetchSubdirs(parentPath)
      .then((res) => {
        const children: TreeNode[] = res.subdirs.map(
          (entry: SubdirEntry) => ({
            name: entry.name,
            path: entry.path,
            children: null,
            isRoot: false,
          })
        )
        setTreeNodes((prev) => updateNodeChildren(prev, parentPath, children))
      })
      .catch(() => {
        setTreeNodes((prev) => updateNodeChildren(prev, parentPath, []))
      })
      .finally(() => {
        setLoadingPaths((prev) => {
          const next = new Set(prev)
          next.delete(parentPath)
          return next
        })
      })
/* eslint-enable react-hooks/set-state-in-effect */
}, [selectedPath])

  // Sync tree nodes when rootPaths change
  if (rootPaths.length !== treeNodes.length ||
      rootPaths.some((p, i) => treeNodes[i]?.path !== p)) {
    const newNodes = rootPaths.map((p) => ({
      name: p.split("/").filter(Boolean).pop() || p,
      path: p,
      children: null,
      isRoot: true,
    }))
    // Preserve existing expanded state
    if (treeNodes.length > 0) {
      setExpandedPaths((prev) => {
        const next = new Set(prev)
        // Remove stale expanded paths
        const validPaths = new Set(rootPaths)
        for (const p of next) {
          if (!validPaths.has(p) && !Array.from(validPaths).some((vp) => p.startsWith(vp + "/"))) {
            next.delete(p)
          }
        }
        return next
      })
    }
    // We need to update after render, so use setTimeout
    setTimeout(() => setTreeNodes(newNodes), 0)
  }

  const loadChildren = useCallback(async (node: TreeNode) => {
    if (node.children !== null) return // Already loaded

    setLoadingPaths((prev) => new Set(prev).add(node.path))

    try {
      const res = await fetchSubdirs(node.path)
      const children: TreeNode[] = res.subdirs.map((entry: SubdirEntry) => ({
        name: entry.name,
        path: entry.path,
        children: null,
        isRoot: false,
      }))

      setTreeNodes((prev) => updateNodeChildren(prev, node.path, children))
    } catch {
      setTreeNodes((prev) => updateNodeChildren(prev, node.path, []))
    } finally {
      setLoadingPaths((prev) => {
        const next = new Set(prev)
        next.delete(node.path)
        return next
      })
    }
  }, [])

  const toggleExpand = useCallback(
    async (node: TreeNode) => {
      const isExpanded = expandedPaths.has(node.path)

      if (isExpanded) {
        setExpandedPaths((prev) => {
          const next = new Set(prev)
          next.delete(node.path)
          return next
        })
      } else {
        setExpandedPaths((prev) => new Set(prev).add(node.path))
        if (node.children === null) {
          await loadChildren(node)
        }
      }
    },
    [expandedPaths, loadChildren]
  )

  const handleSelect = useCallback(
    (path: string) => {
      onSelect(path)
    },
    [onSelect]
  )

  const renderNode = (node: TreeNode, depth: number): ReactNode => {
    const isExpanded = expandedPaths.has(node.path)
    const isLoading = loadingPaths.has(node.path)
    const isSelected = selectedPath === node.path
    const hasChildren = node.children === null || node.children.length > 0

    return (
      <div key={node.path}>
        <button
          type="button"
          className={cn(
            "w-full flex items-center gap-1.5 px-2 py-1.5 text-sm rounded-sm transition-colors text-left relative",
            "hover:bg-accent hover:text-accent-foreground",
            isSelected && "bg-primary/10 text-primary font-semibold"
          )}
          style={{ paddingLeft: `${8 + depth * 16}px` }}
          onClick={() => {
            handleSelect(node.path)
          }}
          onDoubleClick={() => {
            if (hasChildren) toggleExpand(node)
          }}
        >
          {/* Expand/collapse chevron */}
          <span
            className={cn(
              "flex-shrink-0 w-4 h-4 flex items-center justify-center",
              !hasChildren && "invisible"
            )}
            onClick={(e) => {
              e.stopPropagation()
              if (hasChildren) toggleExpand(node)
            }}
          >
            {isLoading ? (
              <Loader2 className="h-3 w-3 animate-spin" />
            ) : isExpanded ? (
              <ChevronDown className="h-3 w-3" />
            ) : (
              <ChevronRight className="h-3 w-3" />
            )}
          </span>

          {/* Folder icon */}
          {isExpanded ? (
            <FolderOpen className={cn("h-3.5 w-3.5 flex-shrink-0", isSelected && "text-primary")} />
          ) : (
            <Folder className={cn("h-3.5 w-3.5 flex-shrink-0", isSelected && "text-primary")} />
          )}

          {/* Folder name */}
          <span className="truncate flex-1">{node.name}</span>

          {/* Selection checkmark */}
          {isSelected && (
            <Check className="h-3.5 w-3.5 flex-shrink-0 text-primary" />
          )}
        </button>

        {/* Children */}
        {isExpanded && node.children !== null && node.children.length > 0 && (
          <div>{node.children.map((child) => renderNode(child, depth + 1))}</div>
        )}

        {isExpanded && node.children !== null && node.children.length === 0 && (
          <div
            className="text-xs text-muted-foreground italic px-2 py-1"
            style={{ paddingLeft: `${8 + (depth + 1) * 16}px` }}
          >
            (empty)
          </div>
        )}
      </div>
    )
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-4">
        <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
      </div>
    )
  }

  if (rootPaths.length === 0) {
    return (
      <div className="text-sm text-muted-foreground italic px-2 py-2">
        No gallery folders available
      </div>
    )
  }

  return (
    <div className={cn("overflow-y-auto rounded-md border", className)}>
      {treeNodes.map((node) => renderNode(node, 0))}
    </div>
  )
}

/**
 * Recursively updates children for a node identified by path.
 */
function updateNodeChildren(
  nodes: TreeNode[],
  targetPath: string,
  children: TreeNode[]
): TreeNode[] {
  return nodes.map((node) => {
    if (node.path === targetPath) {
      return { ...node, children }
    }
    if (node.children) {
      return { ...node, children: updateNodeChildren(node.children, targetPath, children) }
    }
    return node
  })
}
