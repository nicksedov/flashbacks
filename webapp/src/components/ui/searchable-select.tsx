import * as React from "react"
import { createPortal } from "react-dom"
import { Check, ChevronDown } from "lucide-react"
import { cn } from "@/lib/utils"
import { Input } from "@/components/ui/input"
import { filterOptionsByQuery } from "@/lib/option-filter"

/** One selectable row. `label` is both the rendered text and the search text. */
export type SearchableOption = { value: string; label: string }

export interface SearchableSelectProps {
  /** Currently saved value; empty string means "nothing selected yet". */
  value: string
  /** Called only on an explicit choice (click or Enter on an active option). */
  onValueChange: (value: string) => void
  options: SearchableOption[]
  /** Text shown on the trigger while no option is selected. */
  placeholder: string
  /** Localized placeholder and accessible name of the search field. */
  searchPlaceholder: string
  /** Localized message rendered in place of the options when nothing matches. */
  noResultsText: string
  disabled?: boolean
  triggerId?: string
}

// Layout constants mirror the shared Select content cap so both dropdown
// families read as one control family.
const PANEL_MAX_HEIGHT = 288
const PANEL_MIN_WIDTH = 128
const PANEL_GAP = 4

const TRIGGER_CLASSES =
  "flex h-9 w-full items-center justify-between gap-2 whitespace-nowrap rounded-md border border-input bg-transparent px-3 py-2 text-sm shadow-sm ring-offset-background focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50"

/**
 * Dropdown with an integrated search field.
 *
 * The panel is portaled to `document.body` and positioned from the trigger's
 * viewport rect (flipping above the trigger when it does not fit below), so a
 * long model list is never clipped by a scrolling settings card.
 *
 * The typed query belongs to this component and is discarded on close, which
 * makes every reopening start unfiltered and keeps typing from touching the
 * saved value — only `onValueChange` reports a choice.
 */
export function SearchableSelect({
  value,
  onValueChange,
  options,
  placeholder,
  searchPlaceholder,
  noResultsText,
  disabled = false,
  triggerId,
}: SearchableSelectProps) {
  const [open, setOpen] = React.useState(false)
  const [query, setQuery] = React.useState("")
  const [activeIndex, setActiveIndex] = React.useState(0)
  const [panelStyle, setPanelStyle] = React.useState<React.CSSProperties | null>(null)

  const triggerRef = React.useRef<HTMLButtonElement>(null)
  const panelRef = React.useRef<HTMLDivElement>(null)
  const searchRef = React.useRef<HTMLInputElement>(null)
  const listboxId = React.useId()

  const selected = options.find((option) => option.value === value)
  const filtered = React.useMemo(
    () => filterOptionsByQuery(options, query),
    [options, query],
  )
  // The list can shrink under the highlight while typing; clamp instead of
  // resetting so arrow navigation stays predictable.
  const highlightedIndex = filtered.length === 0 ? -1 : Math.min(activeIndex, filtered.length - 1)

  const close = React.useCallback(() => {
    setOpen(false)
    setQuery("")
    setActiveIndex(0)
  }, [])

  const openPanel = React.useCallback(() => {
    setQuery("")
    setActiveIndex(0)
    setOpen(true)
  }, [])

  const positionPanel = React.useCallback(() => {
    const trigger = triggerRef.current
    if (!trigger) return
    const rect = trigger.getBoundingClientRect()
    const viewportHeight = window.innerHeight || 0
    const spaceBelow = viewportHeight - rect.bottom
    const openUpwards = spaceBelow < PANEL_MAX_HEIGHT && rect.top > spaceBelow
    const style: React.CSSProperties = {
      position: "fixed",
      left: rect.left,
      minWidth: Math.max(rect.width, PANEL_MIN_WIDTH),
      zIndex: 50,
    }
    if (openUpwards) {
      style.bottom = viewportHeight - rect.top + PANEL_GAP
    } else {
      style.top = rect.bottom + PANEL_GAP
    }
    setPanelStyle(style)
  }, [])

  React.useEffect(() => {
    if (!open) return
    positionPanel()
    searchRef.current?.focus()

    const handlePointerDown = (event: PointerEvent) => {
      const target = event.target as Node | null
      if (panelRef.current?.contains(target)) return
      if (triggerRef.current?.contains(target)) return
      close()
    }
    const handleViewportChange = () => positionPanel()

    document.addEventListener("pointerdown", handlePointerDown)
    window.addEventListener("resize", handleViewportChange)
    window.addEventListener("scroll", handleViewportChange, true)
    return () => {
      document.removeEventListener("pointerdown", handlePointerDown)
      window.removeEventListener("resize", handleViewportChange)
      window.removeEventListener("scroll", handleViewportChange, true)
    }
  }, [open, close, positionPanel])

  const handleTriggerClick = () => {
    if (disabled) return
    if (open) {
      close()
      return
    }
    openPanel()
  }

  const handleTriggerKeyDown = (event: React.KeyboardEvent<HTMLButtonElement>) => {
    if (event.key === "ArrowDown" || event.key === "ArrowUp" || event.key === "Enter" || event.key === " ") {
      event.preventDefault()
      if (!open) openPanel()
      return
    }
    if (event.key === "Escape" && open) {
      event.preventDefault()
      close()
    }
  }

  const handlePanelKeyDown = (event: React.KeyboardEvent<HTMLDivElement>) => {
    if (event.key === "Escape") {
      event.preventDefault()
      close()
      return
    }
    if (event.key === "Tab") {
      close()
      return
    }
    if (event.key === "ArrowDown") {
      event.preventDefault()
      setActiveIndex(() =>
        filtered.length === 0 ? 0 : (highlightedIndex + 1) % filtered.length,
      )
      return
    }
    if (event.key === "ArrowUp") {
      event.preventDefault()
      setActiveIndex(() =>
        filtered.length === 0 ? 0 : (highlightedIndex - 1 + filtered.length) % filtered.length,
      )
      return
    }
    if (event.key === "Enter") {
      event.preventDefault()
      const option = highlightedIndex >= 0 ? filtered[highlightedIndex] : undefined
      if (option) {
        onValueChange(option.value)
        close()
      }
    }
  }

  const selectOption = (option: SearchableOption) => {
    onValueChange(option.value)
    close()
  }

  return (
    <>
      <button
        ref={triggerRef}
        id={triggerId}
        type="button"
        role="combobox"
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={open ? listboxId : undefined}
        aria-label={selected?.label ?? placeholder}
        disabled={disabled}
        onClick={handleTriggerClick}
        onKeyDown={handleTriggerKeyDown}
        className={cn(TRIGGER_CLASSES, "text-left")}
      >
        <span className={cn("line-clamp-1", !selected && "text-muted-foreground")}>
          {selected?.label ?? placeholder}
        </span>
        <ChevronDown className="h-4 w-4 shrink-0 opacity-50" aria-hidden="true" />
      </button>

      {open && panelStyle
        ? createPortal(
            <div
              ref={panelRef}
              style={panelStyle}
              onKeyDown={handlePanelKeyDown}
              className="rounded-md border bg-popover text-popover-foreground shadow-md"
            >
              <div className="p-1">
                <Input
                  ref={searchRef}
                  type="text"
                  value={query}
                  onChange={(event) => {
                    setQuery(event.target.value)
                    setActiveIndex(0)
                  }}
                  placeholder={searchPlaceholder}
                  aria-label={searchPlaceholder}
                  autoComplete="off"
                  spellCheck={false}
                  className="h-8"
                />
              </div>
              <div
                id={listboxId}
                role="listbox"
                aria-label={placeholder}
                className="max-h-72 overflow-y-auto p-1"
              >
                {filtered.length === 0 ? (
                  <p role="status" className="px-2 py-3 text-sm text-muted-foreground">
                    {noResultsText}
                  </p>
                ) : (
                  filtered.map((option, index) => {
                    const isActive = index === highlightedIndex
                    const isSelected = option.value === value
                    return (
                      <button
                        key={option.value}
                        type="button"
                        role="option"
                        aria-selected={isSelected}
                        onClick={() => selectOption(option)}
                        onMouseEnter={() => setActiveIndex(index)}
                        className={cn(
                          "relative flex w-full cursor-default select-none items-center gap-2 rounded-sm py-1.5 pl-8 pr-2 text-left text-sm outline-none focus-visible:ring-1 focus-visible:ring-ring",
                          isActive && "bg-accent text-accent-foreground",
                        )}
                      >
                        <span className="absolute left-2 flex h-3.5 w-3.5 items-center justify-center">
                          {isSelected ? <Check className="h-4 w-4" aria-hidden="true" /> : null}
                        </span>
                        <span className="line-clamp-1">{option.label}</span>
                      </button>
                    )
                  })
                )}
              </div>
            </div>,
            document.body,
          )
        : null}
    </>
  )
}
