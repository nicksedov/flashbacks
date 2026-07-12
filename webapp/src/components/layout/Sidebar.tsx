import { useCallback, useState, useEffect } from "react"
import { useTranslation } from "@/i18n"
import {
  Settings, ImageIcon, FileScan, Shield, Users, ChevronDown, ChevronRight,
  Folder, Calendar, FileText, MapPin, Trash2, Database, Search,
  PanelLeftClose, PanelLeftOpen, X,
} from "lucide-react"
import { useAuth } from "@/providers/AuthProvider"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

interface SidebarProps {
  activeTab: string
  onTabChange: (tab: string) => void
  collapsed: boolean
  onToggleCollapse: () => void
  mobileOpen: boolean
  onMobileClose: () => void
}

type SubItem = {
  value: string
  icon: React.ElementType
  label: string
}

type GroupItem = {
  id: string
  icon: React.ElementType
  label: string
  subItems: SubItem[]
  defaultTab: string
  isActive: boolean
}

export function Sidebar({
  activeTab,
  onTabChange,
  collapsed,
  onToggleCollapse,
  mobileOpen,
  onMobileClose,
}: SidebarProps) {
  const { t } = useTranslation()
  const { user } = useAuth()
  const [galleryExpanded, setGalleryExpanded] = useState(true)
  const [toolsExpanded, setToolsExpanded] = useState(true)
  const [adminExpanded, setAdminExpanded] = useState(true)

  // Close mobile drawer on tab change
  const handleTabChangeAndClose = useCallback(
    (tab: string) => {
      onTabChange(tab)
      onMobileClose()
    },
    [onTabChange, onMobileClose]
  )

  const gallerySubItems: SubItem[] = [
    { value: "gallery-all-images", icon: Folder, label: t("gallery.subModes.allImages") },
    { value: "gallery-folders", icon: Folder, label: t("gallery.subModes.folders") },
    { value: "gallery-calendar", icon: Calendar, label: t("gallery.subModes.calendar") },
    { value: "gallery-geolocation", icon: MapPin, label: t("gallery.subModes.geolocation") },
  ]

  const toolsSubItems: SubItem[] = [
    { value: "smart-search", icon: Search, label: t("tabs.smartSearch") },
    { value: "deduplication", icon: FileScan, label: t("tabs.deduplication") },
    { value: "ocr", icon: FileText, label: t("tabs.ocr") },
    { value: "exif", icon: Database, label: t("tabs.exif") },
  ]

  const adminSubItems: SubItem[] = [
    { value: "admin-users", icon: Users, label: t("adminPanel.title") },
    { value: "admin-settings", icon: Settings, label: t("adminPanel.adminSettings") },
  ]

  const isGalleryActive = activeTab.startsWith("gallery")
  const isToolsActive = ["deduplication", "ocr", "exif", "smart-search"].includes(activeTab)
  const isTrashActive = activeTab === "gallery-trash"
  const isAdminActive = activeTab === "admin-users" || activeTab === "admin-settings"

  const groups: GroupItem[] = [
    {
      id: "gallery",
      icon: ImageIcon,
      label: t("tabs.gallery"),
      subItems: gallerySubItems,
      defaultTab: "gallery-all-images",
      isActive: isGalleryActive,
    },
    {
      id: "tools",
      icon: FileScan,
      label: t("tabs.tools"),
      subItems: toolsSubItems,
      defaultTab: "deduplication",
      isActive: isToolsActive,
    },
  ]

  const isExpanded = (groupId: string) => {
    if (groupId === "gallery") return galleryExpanded
    if (groupId === "tools") return toolsExpanded
    if (groupId === "admin") return adminExpanded
    return false
  }

  const toggleExpanded = (groupId: string) => {
    if (groupId === "gallery") setGalleryExpanded((v) => !v)
    else if (groupId === "tools") setToolsExpanded((v) => !v)
    else if (groupId === "admin") setAdminExpanded((v) => !v)
  }

  // Close mobile drawer on Escape key
  useEffect(() => {
    if (!mobileOpen) return
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") onMobileClose()
    }
    window.addEventListener("keydown", handleKeyDown)
    return () => window.removeEventListener("keydown", handleKeyDown)
  }, [mobileOpen, onMobileClose])

  const renderSubItems = (subItems: SubItem[], handleSelect: (tab: string) => void) => (
    <div className="ml-4 space-y-0.5 pl-2">
      {subItems.map((subItem) => {
        const isActive = activeTab === subItem.value
        return (
          <Button
            key={subItem.value}
            variant={isActive ? "default" : "ghost"}
            size="sm"
            className={cn(
              "w-full justify-start gap-2 h-8",
              isActive && "bg-primary text-primary-foreground hover:bg-primary/90"
            )}
            onClick={() => handleSelect(subItem.value)}
          >
            <subItem.icon className="h-3.5 w-3.5 flex-shrink-0" />
            <span className="text-sm font-medium">{subItem.label}</span>
          </Button>
        )
      })}
    </div>
  )

  const renderGroupExpanded = (
    group: GroupItem,
    expanded: boolean,
    handleSelect: (tab: string) => void
  ) => (
    <div className="space-y-1" key={group.id}>
      <Button
        variant={group.isActive ? "default" : "ghost"}
        className={cn(
          "w-full justify-start gap-3",
          group.isActive && "bg-primary text-primary-foreground hover:bg-primary/90"
        )}
        onClick={() => {
          toggleExpanded(group.id)
          if (!group.isActive) handleSelect(group.defaultTab)
        }}
      >
        <group.icon className="h-4 w-4 flex-shrink-0" />
        <span className="flex-1 font-medium text-left">{group.label}</span>
        {expanded ? (
          <ChevronDown className="h-3 w-3 flex-shrink-0" />
        ) : (
          <ChevronRight className="h-3 w-3 flex-shrink-0" />
        )}
      </Button>
      {expanded && renderSubItems(group.subItems, handleSelect)}
    </div>
  )

  const handleSelect = (tab: string) => handleTabChangeAndClose(tab)

  // -- Sidebar inner content --
  const sidebarContent = (
    <>
      {/* Header */}
      <div className="flex h-16 items-center px-4">
        {collapsed ? (
          <div className="flex w-full items-center justify-center">
            <div className="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-lg bg-primary text-primary-foreground">
              <svg className="h-5 w-5" viewBox="0 0 48 46" fill="currentColor" xmlns="http://www.w3.org/2000/svg">
                <path d="M25.946 44.938c-.664.845-2.021.375-2.021-.698V33.937a2.26 2.26 0 0 0-2.262-2.262H10.287c-.92 0-1.456-1.04-.92-1.788l7.48-10.471c1.07-1.497 0-3.578-1.842-3.578H1.237c-.92 0-1.456-1.04-.92-1.788L10.013.474c.214-.297.556-.474.92-.474h28.894c.92 0 1.456 1.04.92 1.788l-7.48 10.471c-1.07 1.498 0 3.579 1.842 3.579h11.377c.943 0 1.473 1.088.89 1.83L25.947 44.94z" />
              </svg>
            </div>
          </div>
        ) : (
          <div className="flex flex-1 items-center gap-3 overflow-hidden">
            <div className="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-lg bg-primary text-primary-foreground">
              <svg className="h-5 w-5" viewBox="0 0 48 46" fill="currentColor" xmlns="http://www.w3.org/2000/svg">
                <path d="M25.946 44.938c-.664.845-2.021.375-2.021-.698V33.937a2.26 2.26 0 0 0-2.262-2.262H10.287c-.92 0-1.456-1.04-.92-1.788l7.48-10.471c1.07-1.497 0-3.578-1.842-3.578H1.237c-.92 0-1.456-1.04-.92-1.788L10.013.474c.214-.297.556-.474.92-.474h28.894c.92 0 1.456 1.04.92 1.788l-7.48 10.471c-1.07 1.498 0 3.579 1.842 3.579h11.377c.943 0 1.473 1.088.89 1.83L25.947 44.94z" />
              </svg>
            </div>
            <div className="overflow-hidden">
              <h1 className="truncate font-semibold text-lg">{t("header.title")}</h1>
            </div>
          </div>
        )}
        {/* Mobile close button */}
        <button
          type="button"
          className="ml-auto flex h-8 w-8 items-center justify-center rounded-md hover:bg-muted transition-colors md:hidden"
          onClick={onMobileClose}
          aria-label="Close sidebar"
        >
          <X className="h-5 w-5" />
        </button>
      </div>

      {/* Navigation */}
      <nav className="flex-1 px-2 py-4 space-y-1 overflow-y-auto">
        {collapsed ? (
          <>
            {/* Collapsed: all individual sub-item icons */}
            <div className="space-y-1">
              {gallerySubItems.map((item) => (
                <Button
                  key={item.value}
                  variant={activeTab === item.value ? "default" : "ghost"}
                  className={cn(
                    "w-full justify-center gap-0 h-10",
                    activeTab === item.value && "bg-primary text-primary-foreground hover:bg-primary/90"
                  )}
                  title={item.label}
                  onClick={() => handleSelect(item.value)}
                >
                  <item.icon className="h-5 w-5 flex-shrink-0" />
                </Button>
              ))}
            </div>

            <div className="space-y-1 mt-2">
              {toolsSubItems.map((item) => (
                <Button
                  key={item.value}
                  variant={activeTab === item.value ? "default" : "ghost"}
                  className={cn(
                    "w-full justify-center gap-0 h-10",
                    activeTab === item.value && "bg-primary text-primary-foreground hover:bg-primary/90"
                  )}
                  title={item.label}
                  onClick={() => handleSelect(item.value)}
                >
                  <item.icon className="h-5 w-5 flex-shrink-0" />
                </Button>
              ))}
            </div>

            <div className="space-y-1 mt-2">
              <Button
                variant={isTrashActive ? "default" : "ghost"}
                className={cn(
                  "w-full justify-center gap-0 h-10",
                  isTrashActive && "bg-primary text-primary-foreground hover:bg-primary/90"
                )}
                title={t("tabs.trash")}
                onClick={() => handleSelect("gallery-trash")}
              >
                <Trash2 className="h-5 w-5 flex-shrink-0" />
              </Button>
            </div>

            {user?.role === "admin" && (
              <div className="mt-2 space-y-1">
                {adminSubItems.map((item) => (
                  <Button
                    key={item.value}
                    variant={activeTab === item.value ? "default" : "ghost"}
                    className={cn(
                      "w-full justify-center gap-0 h-10",
                      activeTab === item.value && "bg-primary text-primary-foreground hover:bg-primary/90"
                    )}
                    title={item.label}
                    onClick={() => handleSelect(item.value)}
                  >
                    <item.icon className="h-5 w-5 flex-shrink-0" />
                  </Button>
                ))}
              </div>
            )}
          </>
        ) : (
          <>
            {/* Expanded: full layout with sub-items */}
            <div className="space-y-1">
              {groups.map((group) =>
                renderGroupExpanded(group, isExpanded(group.id), handleSelect)
              )}
            </div>

            {/* Trash */}
            <div className="space-y-1 mt-4">
              <Button
                variant={isTrashActive ? "default" : "ghost"}
                className={cn(
                  "w-full justify-start gap-3 h-9",
                  isTrashActive && "bg-primary text-primary-foreground hover:bg-primary/90"
                )}
                onClick={() => handleSelect("gallery-trash")}
              >
                <Trash2 className="h-4 w-4 flex-shrink-0" />
                <span className="flex-1 font-medium text-left">{t("tabs.trash")}</span>
              </Button>
            </div>

            {/* Admin */}
            {user?.role === "admin" && (
              <div className="mt-4">
                <Button
                  variant={isAdminActive ? "default" : "ghost"}
                  className={cn(
                    "w-full justify-start gap-3",
                    isAdminActive && "bg-primary text-primary-foreground hover:bg-primary/90"
                  )}
                  onClick={() => {
                    setAdminExpanded((v) => !v)
                    if (!isAdminActive) handleSelect("admin-settings")
                  }}
                >
                  <Shield className="h-4 w-4 flex-shrink-0" />
                  <span className="flex-1 font-medium text-left">{t("adminPanel.administration")}</span>
                  {adminExpanded ? (
                    <ChevronDown className="h-3 w-3 flex-shrink-0" />
                  ) : (
                    <ChevronRight className="h-3 w-3 flex-shrink-0" />
                  )}
                </Button>
                {adminExpanded && renderSubItems(adminSubItems, handleSelect)}
              </div>
            )}
          </>
        )}
      </nav>

      {/* Collapse toggle (desktop only) */}
      <div className="hidden md:block px-2 py-2">
        <Button
          variant="ghost"
          size="sm"
          className={cn("w-full gap-2", collapsed ? "justify-center" : "justify-start")}
          onClick={onToggleCollapse}
          title={collapsed ? t("sidebar.expand") : t("sidebar.collapse")}
        >
          {collapsed ? (
            <PanelLeftOpen className="h-4 w-4 flex-shrink-0" />
          ) : (
            <>
              <PanelLeftClose className="h-4 w-4 flex-shrink-0" />
              <span className="text-sm">{t("sidebar.collapse")}</span>
            </>
          )}
        </Button>
      </div>
    </>
  )

  return (
    <>
      {/* Mobile backdrop */}
      {mobileOpen && (
        <div
          className="sidebar-backdrop fixed inset-0 z-40 md:hidden"
          onClick={onMobileClose}
          aria-hidden="true"
        />
      )}

      {/* Sidebar */}
      <aside
        className={cn(
          "sidebar-transition sticky top-0 flex h-screen flex-shrink-0 flex-col z-50",
          // Desktop width
          collapsed ? "hidden md:flex md:w-16" : "hidden md:flex md:w-64",
          // Mobile: overlay
          mobileOpen ? "fixed inset-y-0 left-0 flex w-64" : ""
        )}
        style={{ backgroundColor: 'var(--color-sidebar)' }}
      >
        {sidebarContent}
      </aside>
    </>
  )
}
