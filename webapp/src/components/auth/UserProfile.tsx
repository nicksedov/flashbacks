import { useEffect, useRef, useState } from "react"
import { useAuth } from "@/providers/AuthProvider"
import { changePassword as apiChangePassword, updateProfile as apiUpdateProfile, uploadAvatar, deleteAvatar, getAvatarUrl } from "@/api/endpoints"
import type { ChangePasswordResponse } from "@/types"
import { toast } from "sonner"
import { Loader2, Lock, Trash2, Upload, User } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Separator } from "@/components/ui/separator"
import { Badge } from "@/components/ui/badge"
import { useTranslation } from "@/i18n"
import { translateApiMessage } from "@/api/client"
import { AvatarCropDialog } from "@/components/auth/AvatarCropDialog"

export function UserProfile() {
  const { user, updateUser, logout } = useAuth()
  const { t } = useTranslation()
  const [displayName, setDisplayName] = useState(user?.displayName || "")
  const [isSavingProfile, setIsSavingProfile] = useState(false)

  const [oldPassword, setOldPassword] = useState("")
  const [newPassword, setNewPassword] = useState("")
  const [confirmPassword, setConfirmPassword] = useState("")
  const [isChangingPassword, setIsChangingPassword] = useState(false)

  const [cropOpen, setCropOpen] = useState(false)
  const [cropFile, setCropFile] = useState<File | null>(null)
  const [isUploading, setIsUploading] = useState(false)
  const [avatarVersion, setAvatarVersion] = useState(Date.now())
  const fileInputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    const handleNavigateToLogin = () => {
      toast.info(t("adminPanel.sessionExpired"))
    }
    window.addEventListener("navigate-to-login", handleNavigateToLogin as EventListener)
    return () => {
      window.removeEventListener("navigate-to-login", handleNavigateToLogin as EventListener)
    }
  }, [])

  if (!user) return null

  const handleSaveProfile = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!displayName.trim()) return

    setIsSavingProfile(true)
    try {
      const response = await apiUpdateProfile({ displayName })
      updateUser(response.user)
      toast.success(t("adminPanel.profileUpdated"))
    } catch (err) {
      const errorMessage = err instanceof Error ? translateApiMessage(err.message) : t("adminPanel.updateProfile")
      toast.error(errorMessage)
    } finally {
      setIsSavingProfile(false)
    }
  }

  const handleChangePassword = async (e: React.FormEvent) => {
    e.preventDefault()

    if (newPassword.length < 8) {
      toast.error(t("adminPanel.passwordTooShort"))
      return
    }

    if (newPassword !== confirmPassword) {
      toast.error(t("adminPanel.passwordsMismatch"))
      return
    }

    setIsChangingPassword(true)
    try {
      const response: ChangePasswordResponse = await apiChangePassword({ oldPassword, newPassword })
      setOldPassword("")
      setNewPassword("")
      setConfirmPassword("")

      if (response.mustLogin) {
        toast.success(t("adminPanel.passwordChanged"))
        await logout()
        navigateToLogin()
      } else {
        toast.success(t("adminPanel.passwordChangedSuccess"))
      }
    } catch (err: unknown) {
      const error = err instanceof Error ? translateApiMessage(err.message) : t("adminPanel.updatePassword")
      toast.error(error)
      if (error.includes("Требуется авторизация") || error.includes("Authorization required")) {
        await logout()
        navigateToLogin()
      }
    } finally {
      setIsChangingPassword(false)
    }
  }

  const navigateToLogin = () => {
    logout()
    const event = new CustomEvent("navigate-to-login")
    window.dispatchEvent(event)
  }

  return (
    <div className="mx-auto max-w-2xl space-y-6">
      <div>
        <h2 className="text-2xl font-bold">{t("adminPanel.updateProfile")}</h2>
        <p className="text-muted-foreground">{t("adminPanel.profileDescription")}</p>
      </div>

      {/* Avatar Card */}
      {user && (
        <Card>
          <CardHeader>
            <CardTitle>{t("avatar.title")}</CardTitle>
            <CardDescription>{t("avatar.description")}</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="flex items-center gap-4">
              <div className="h-20 w-20 rounded-full overflow-hidden bg-muted flex items-center justify-center shrink-0">
                {user.hasAvatar ? (
                  <img
                    src={`${getAvatarUrl(user.id)}?v=${avatarVersion}`}
                    alt="Avatar"
                    className="h-full w-full object-cover"
                  />
                ) : (
                  <User className="h-10 w-10 text-muted-foreground" />
                )}
              </div>
              <div className="flex gap-2">
                <input
                  ref={fileInputRef}
                  type="file"
                  accept="image/jpeg,image/png,image/webp,image/gif"
                  className="hidden"
                  onChange={(e) => {
                    const file = e.target.files?.[0]
                    if (!file) return
                    // Validate type
                    const allowed = ["image/jpeg", "image/png", "image/webp", "image/gif"]
                    if (!allowed.includes(file.type)) {
                      toast.error(t("avatar.invalidType"))
                      return
                    }
                    setCropFile(file)
                    setCropOpen(true)
                    e.target.value = ""
                  }}
                />
                <Button
                  variant="outline"
                  size="sm"
                  disabled={isUploading}
                  onClick={() => fileInputRef.current?.click()}
                >
                  {isUploading ? (
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  ) : (
                    <Upload className="mr-2 h-4 w-4" />
                  )}
                  {t("avatar.upload")}
                </Button>
                {user.hasAvatar && (
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={isUploading}
                    onClick={async () => {
                      setIsUploading(true)
                      try {
                        const response = await deleteAvatar()
                        updateUser(response.user)
                        setAvatarVersion(Date.now())
                        toast.success(t("avatar.removeSuccess"))
                      } catch (err) {
                        const msg = err instanceof Error ? translateApiMessage(err.message) : t("avatar.removeFailed")
                        toast.error(msg)
                      } finally {
                        setIsUploading(false)
                      }
                    }}
                  >
                    <Trash2 className="mr-2 h-4 w-4" />
                    {t("avatar.remove")}
                  </Button>
                )}
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      <AvatarCropDialog
        open={cropOpen}
        onOpenChange={setCropOpen}
        imageFile={cropFile}
        onApply={async (blob) => {
          setCropOpen(false)
          setIsUploading(true)
          try {
            const response = await uploadAvatar(blob)
            updateUser(response.user)
            setAvatarVersion(Date.now())
            toast.success(t("avatar.uploadSuccess"))
          } catch (err) {
            const msg = err instanceof Error ? translateApiMessage(err.message) : t("avatar.uploadFailed")
            toast.error(msg)
          } finally {
            setIsUploading(false)
          }
        }}
      />

      <Card>
        <CardContent className="space-y-4 pt-6">
          <div className="flex items-center justify-between">
            <p className="font-medium">{user.displayName}</p>
            <Badge variant={user.role === "admin" ? "default" : "secondary"}>
              {user.role === "admin" ? t("adminPanel.admin") : t("adminPanel.user")}
            </Badge>
          </div>
          <div className="space-y-2">
            <Label htmlFor="login">{t("adminPanel.account")}</Label>
            <Input id="login" value={user.login} readOnly className="bg-muted text-muted-foreground" />
          </div>
          <Separator />
          <form onSubmit={handleSaveProfile} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="displayName">{t("adminPanel.displayName")}</Label>
              <Input
                id="displayName"
                value={displayName}
                onChange={(e) => setDisplayName(e.target.value)}
              />
            </div>
            <Button type="submit" disabled={isSavingProfile || displayName === user.displayName}>
              {isSavingProfile && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              {t("adminPanel.save")}
            </Button>
          </form>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <Lock className="h-5 w-5" />
            <CardTitle>{t("adminPanel.changePassword")}</CardTitle>
          </div>
          <CardDescription>{t("adminPanel.updatePassword")}</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleChangePassword} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="oldPassword">{t("adminPanel.currentPassword")}</Label>
              <Input
                id="oldPassword"
                type="password"
                autoComplete="current-password"
                value={oldPassword}
                onChange={(e) => setOldPassword(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="newPassword">{t("adminPanel.newPassword")}</Label>
              <Input
                id="newPassword"
                type="password"
                autoComplete="new-password"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="confirmPassword">{t("adminPanel.confirmPassword")}</Label>
              <Input
                id="confirmPassword"
                type="password"
                autoComplete="new-password"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
              />
            </div>
            <Button type="submit" disabled={isChangingPassword || !oldPassword || !newPassword || !confirmPassword}>
              {isChangingPassword && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              {t("adminPanel.updatePassword")}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
