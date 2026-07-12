import { useEffect, useState, lazy, Suspense, useCallback } from "react"
import { Toaster } from "sonner"
import { Tabs, TabsContent } from "@/components/ui/tabs"
import { Sidebar } from "@/components/layout/Sidebar"
import { Header } from "@/components/layout/Header"
import { fetchFolders } from "@/api/endpoints"
import { useTranslation } from "@/i18n"
import { useSettings } from "@/providers/useSettings"
import { useAuth } from "@/providers/AuthProvider"
import { GallerySelectionProvider } from "@/providers/GallerySelectionContext"
import { LoginScreen } from "@/components/auth/LoginScreen"
import { BootstrapSetupScreen } from "@/components/auth/BootstrapSetupScreen"
import { UserProfile } from "@/components/auth/UserProfile"

// Lazy load tab components for code splitting
const SettingsTab = lazy(() => import("@/components/tabs/SettingsTab").then(module => ({ default: module.SettingsTab })))
const GalleryTab = lazy(() => import("@/components/tabs/GalleryTab").then(module => ({ default: module.GalleryTab })))
const TrashTab = lazy(() => import("@/components/tabs/TrashTab").then(module => ({ default: module.TrashTab })))
const DeduplicationTab = lazy(() => import("@/components/tabs/DeduplicationTab").then(module => ({ default: module.DeduplicationTab })))
const OcrTab = lazy(() => import("@/components/tabs/OcrTab").then(module => ({ default: module.OcrTab })))
const ExifTab = lazy(() => import("@/components/tabs/ExifTab").then(module => ({ default: module.ExifTab })))
const AdminSettingsTab = lazy(() => import("@/components/tabs/AdminSettingsTab").then(module => ({ default: module.AdminSettingsTab })))
const AdminPanel = lazy(() => import("@/components/auth/AdminPanel").then(module => ({ default: module.AdminPanel })))
const SmartSearchTab = lazy(() => import("@/components/tabs/SmartSearchTab").then(module => ({ default: module.SmartSearchTab })))

type TabValue = "settings" | "gallery-all-images" | "gallery-calendar" | "gallery-geolocation" | "gallery-trash" | "deduplication" | "ocr" | "exif" | "smart-search" | "profile" | "admin-settings" | "admin-users"

export default function App() {
  const [activeTab, setActiveTab] = useState<TabValue>("gallery-all-images")
  const [isCheckingGallery, setIsCheckingGallery] = useState(true)
  const [forceLogout, setForceLogout] = useState(false)
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false)
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false)
  const { t } = useTranslation()
  const { isLoading: isLoadingSettings } = useSettings()
  const { user, isAuthenticated, isBootstrapMode, isBootstrapVerified, isLoading: isLoadingAuth } = useAuth()

  // Listen for navigate-to-login event
  useEffect(() => {
    const handleNavigateToLogin = () => {
      setForceLogout(true)
    }
    window.addEventListener("navigate-to-login", handleNavigateToLogin as EventListener)
    return () => {
      window.removeEventListener("navigate-to-login", handleNavigateToLogin as EventListener)
    }
  }, [])

  // On mount, check if gallery has folders. If not, force settings tab.
  useEffect(() => {
    async function checkGallery() {
      try {
        const result = await fetchFolders()
        if (result.totalFolders === 0) {
          setActiveTab("settings")
        }
      } catch {
        // If API fails, still allow normal navigation
      } finally {
        setIsCheckingGallery(false)
      }
    }
    if (isAuthenticated) {
      checkGallery()
    } else {
      setIsCheckingGallery(false)
    }
  }, [isAuthenticated])

  const handleTabChange = useCallback((tab: string) => {
    setActiveTab(tab as TabValue)
  }, [])

  const handleToggleCollapse = useCallback(() => {
    setSidebarCollapsed((v) => !v)
  }, [])

  const handleMobileMenuToggle = useCallback(() => {
    setMobileMenuOpen((v) => !v)
  }, [])

  const handleMobileMenuClose = useCallback(() => {
    setMobileMenuOpen(false)
  }, [])

  // Loading state
  if (isLoadingAuth || (isAuthenticated && (isCheckingGallery || isLoadingSettings))) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background">
        <div className="text-center">
          <div className="mx-auto mb-4 h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
          <p className="text-sm text-muted-foreground">{t("common.loading")}</p>
        </div>
      </div>
    )
  }

  // Not authenticated or forced logout - show login or bootstrap setup
  if (!isAuthenticated || forceLogout) {
    if (isBootstrapMode && isBootstrapVerified) {
      return (
        <>
          <BootstrapSetupScreen />
          <Toaster richColors position="top-right" />
        </>
      )
    }
    return (
      <>
        <LoginScreen />
        <Toaster richColors position="top-right" />
      </>
    )
  }

  return (
    <GallerySelectionProvider>
      <div className="flex h-screen bg-background">
        <Sidebar
          activeTab={activeTab}
          onTabChange={handleTabChange}
          collapsed={sidebarCollapsed}
          onToggleCollapse={handleToggleCollapse}
          mobileOpen={mobileMenuOpen}
          onMobileClose={handleMobileMenuClose}
        />

        <div className="flex flex-1 flex-col overflow-hidden">
          <Header onTabChange={handleTabChange} onMobileMenuToggle={handleMobileMenuToggle} />

        <main className={`flex-1 overflow-auto ${activeTab === "gallery-geolocation" ? "px-3 py-3" : "px-8 py-6"}`}>
          <div className={activeTab === "gallery-geolocation" ? "mx-auto" : "mx-auto max-w-7xl"}>
            <Tabs value={activeTab} onValueChange={(v) => handleTabChange(v)}>
              <TabsContent value="settings">
                <Suspense fallback={<div className="flex items-center justify-center py-20"><div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" /></div>}>
                  <SettingsTab />
                </Suspense>
              </TabsContent>

              <TabsContent value="gallery-all-images">
                <Suspense fallback={<div className="flex items-center justify-center py-20"><div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" /></div>}>
                  <GalleryTab galleryMode="allImages" />
                </Suspense>
              </TabsContent>

              <TabsContent value="gallery-calendar">
                <Suspense fallback={<div className="flex items-center justify-center py-20"><div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" /></div>}>
                  <GalleryTab galleryMode="calendar" />
                </Suspense>
              </TabsContent>

              <TabsContent value="gallery-geolocation">
                <Suspense fallback={<div className="flex items-center justify-center py-20"><div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" /></div>}>
                  <GalleryTab galleryMode="geolocation" />
                </Suspense>
              </TabsContent>

              <TabsContent value="gallery-trash">
                <Suspense fallback={<div className="flex items-center justify-center py-20"><div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" /></div>}>
                  <TrashTab />
                </Suspense>
              </TabsContent>

              <TabsContent value="deduplication">
                <Suspense fallback={<div className="flex items-center justify-center py-20"><div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" /></div>}>
                  <DeduplicationTab />
                </Suspense>
              </TabsContent>

              <TabsContent value="ocr">
                <Suspense fallback={<div className="flex items-center justify-center py-20"><div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" /></div>}>
                  <OcrTab />
                </Suspense>
              </TabsContent>

              <TabsContent value="exif">
                <Suspense fallback={<div className="flex items-center justify-center py-20"><div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" /></div>}>
                  <ExifTab />
                </Suspense>
              </TabsContent>

              <TabsContent value="smart-search">
                <Suspense fallback={<div className="flex items-center justify-center py-20"><div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" /></div>}>
                  <SmartSearchTab />
                </Suspense>
              </TabsContent>

              <TabsContent value="admin-users">
                <Suspense fallback={<div className="flex items-center justify-center py-20"><div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" /></div>}>
                  {user?.role === "admin" ? <AdminPanel /> : (
                    <div className="flex items-center justify-center py-20">
                      <p className="text-muted-foreground">{t("adminPanel.accessDenied")}</p>
                    </div>
                  )}
                </Suspense>
              </TabsContent>

              <TabsContent value="admin-settings">
                <Suspense fallback={<div className="flex items-center justify-center py-20"><div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" /></div>}>
                  {user?.role === "admin" ? <AdminSettingsTab /> : (
                    <div className="flex items-center justify-center py-20">
                      <p className="text-muted-foreground">{t("adminPanel.accessDenied")}</p>
                    </div>
                  )}
                </Suspense>
              </TabsContent>

              <TabsContent value="profile">
                <UserProfile />
              </TabsContent>
            </Tabs>
          </div>
        </main>
      </div>

      <Toaster richColors position="top-right" />
    </div>
    </GallerySelectionProvider>
  )
}
