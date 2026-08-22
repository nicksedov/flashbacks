import { useEffect, useRef, useState } from "react"
import { useAuth } from "@/providers/useAuth"
import { login as apiLogin, register as apiRegister, fetchAuthStatus } from "@/api/endpoints"
import { toast } from "sonner"
import { Loader2, ShieldAlert, WifiOff } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { useTranslation } from "@/i18n"

const HEALTH_CHECK_INTERVAL_MS = 5000

type AuthMode = "login" | "register"

export function LoginScreen() {
  const { login, isBootstrapMode, setBootstrapVerified, accountCreationMode } = useAuth()
  const { t } = useTranslation()
  const [mode, setMode] = useState<AuthMode>("login")

  const [loginInput, setLoginInput] = useState("")
  const [password, setPassword] = useState("")
  const [registerLogin, setRegisterLogin] = useState("")
  const [registerDisplayName, setRegisterDisplayName] = useState("")
  const [registerPassword, setRegisterPassword] = useState("")
  const [registerPasswordConfirm, setRegisterPasswordConfirm] = useState("")
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState("")
  const [backendOnline, setBackendOnline] = useState<boolean | null>(null)
  const healthTimerRef = useRef<ReturnType<typeof setInterval> | null>(null)

  // Periodic backend health check
  useEffect(() => {
    let cancelled = false

    async function checkHealth() {
      try {
        await fetchAuthStatus()
        if (!cancelled) setBackendOnline(true)
      } catch {
        if (!cancelled) setBackendOnline(false)
      }
    }

    // Immediate first check
    checkHealth()

    // Poll every interval
    healthTimerRef.current = setInterval(checkHealth, HEALTH_CHECK_INTERVAL_MS)

    return () => {
      cancelled = true
      if (healthTimerRef.current) {
        clearInterval(healthTimerRef.current)
        healthTimerRef.current = null
      }
    }
  }, [])

  const isBackendOffline = backendOnline === false
  const isFormDisabled = isLoading || isBackendOffline

  useEffect(() => {
    const handleNavigateToProfile = () => {
      toast.info(t("adminPanel.loginAgain"))
    }
    window.addEventListener("navigate-to-profile", handleNavigateToProfile as EventListener)
    return () => {
      window.removeEventListener("navigate-to-profile", handleNavigateToProfile as EventListener)
    }
  }, [t])

  // Self-service registration is hidden in admin_only mode and during bootstrap
  const canRegister = accountCreationMode !== "admin_only" && !isBootstrapMode
  const isRegisterMode = mode === "register" && canRegister

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError("")

    if (!loginInput.trim() || !password.trim()) {
      setError(t("adminPanel.wrongFields"))
      return
    }

    setIsLoading(true)
    try {
      const response = await apiLogin({ login: loginInput, password })

      if (response.isBootstrap) {
        // Show bootstrap setup screen
        setBootstrapVerified(true)
        return
      }

      if (response.user) {
        login(response.user)
        window.dispatchEvent(new CustomEvent("login-success"))
        toast.success(t("adminPanel.loginSuccess"))
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : t("adminPanel.loginFailed")
      setError(message)
    } finally {
      setIsLoading(false)
    }
  }

  const handleRegisterSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError("")

    if (
      !registerLogin.trim() ||
      !registerDisplayName.trim() ||
      !registerPassword.trim() ||
      !registerPasswordConfirm.trim()
    ) {
      setError(t("adminPanel.wrongFields"))
      return
    }

    if (registerPassword.length < 8) {
      setError(t("adminPanel.passwordTooShort"))
      return
    }

    if (registerPassword !== registerPasswordConfirm) {
      setError(t("adminPanel.passwordsMismatch"))
      return
    }

    setIsLoading(true)
    try {
      const response = await apiRegister({
        login: registerLogin,
        displayName: registerDisplayName,
        password: registerPassword,
      })

      if (response.pending || response.user) {
        // Account created (active or pending approval) - switch to the login form so the user can sign in
        setMode("login")
        setRegisterLogin("")
        setRegisterDisplayName("")
        setRegisterPassword("")
        setRegisterPasswordConfirm("")
        toast.success(response.pending ? t("adminPanel.pendingTitle") : t("adminPanel.registerSuccess"))
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : t("adminPanel.loginFailed")
      setError(message)
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-gradient-to-br from-background to-muted p-4">
      <Card className="w-full max-w-md">
        <CardHeader className="space-y-2 text-center">
          <div className="mx-auto mb-2 flex h-12 w-12 items-center justify-center rounded-full bg-primary/10">
            <ShieldAlert className="h-6 w-6 text-primary" />
          </div>
          <CardTitle className="text-2xl font-bold">
            {isBootstrapMode
              ? t("adminPanel.bootstrapTitle")
              : isRegisterMode
                ? t("adminPanel.registerTitle")
                : t("adminPanel.loginTitle")}
          </CardTitle>
          <CardDescription>
            {isBootstrapMode
              ? t("adminPanel.bootstrapDescAdmin")
              : isRegisterMode
                ? t("adminPanel.registerDesc")
                : t("adminPanel.loginDesc")}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {canRegister && (
            <div className="mb-4 grid grid-cols-2 gap-1 rounded-lg bg-muted p-1">
              <button
                type="button"
                className={`rounded-md px-3 py-1.5 text-sm font-medium transition-colors ${
                  mode === "login" ? "bg-background shadow-sm" : "text-muted-foreground hover:text-foreground"
                }`}
                onClick={() => {
                  setMode("login")
                  setError("")
                }}
              >
                {t("adminPanel.signIn")}
              </button>
              <button
                type="button"
                className={`rounded-md px-3 py-1.5 text-sm font-medium transition-colors ${
                  mode === "register" ? "bg-background shadow-sm" : "text-muted-foreground hover:text-foreground"
                }`}
                onClick={() => {
                  setMode("register")
                  setError("")
                }}
              >
                {t("adminPanel.registerTab")}
              </button>
            </div>
          )}

          {isRegisterMode ? (
            <form onSubmit={handleRegisterSubmit} className="space-y-4">
              {accountCreationMode === "self_service_with_approval" && (
                <div className="flex items-start gap-2 rounded-md bg-amber-500/10 p-3 text-sm text-amber-600 dark:text-amber-400">
                  <ShieldAlert className="mt-0.5 h-4 w-4 shrink-0" />
                  <span>{t("adminPanel.registrationApprovalWarning")}</span>
                </div>
              )}
              <div className="space-y-2">
                <Label htmlFor="register-login">{t("adminPanel.login")}</Label>
                <Input
                  id="register-login"
                  type="text"
                  autoComplete="username"
                  placeholder={t("adminPanel.loginPlaceholder")}
                  value={registerLogin}
                  onChange={(e) => setRegisterLogin(e.target.value)}
                  disabled={isFormDisabled}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="register-displayName">{t("adminPanel.displayName")}</Label>
                <Input
                  id="register-displayName"
                  type="text"
                  placeholder={t("adminPanel.displayName")}
                  value={registerDisplayName}
                  onChange={(e) => setRegisterDisplayName(e.target.value)}
                  disabled={isFormDisabled}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="register-password">{t("adminPanel.newPassword")}</Label>
                <Input
                  id="register-password"
                  type="password"
                  autoComplete="new-password"
                  placeholder={t("adminPanel.passwordPlaceholder")}
                  value={registerPassword}
                  onChange={(e) => setRegisterPassword(e.target.value)}
                  disabled={isFormDisabled}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="register-password-confirm">{t("adminPanel.confirmPassword")}</Label>
                <Input
                  id="register-password-confirm"
                  type="password"
                  autoComplete="new-password"
                  placeholder={t("adminPanel.confirmPassword")}
                  value={registerPasswordConfirm}
                  onChange={(e) => setRegisterPasswordConfirm(e.target.value)}
                  disabled={isFormDisabled}
                />
              </div>
              {isBackendOffline && (
                <div className="flex items-center gap-2 rounded-md bg-amber-500/10 p-3 text-sm text-amber-600 dark:text-amber-400">
                  <WifiOff className="h-4 w-4 shrink-0" />
                  <span>{t("adminPanel.backendOffline")}</span>
                </div>
              )}
              {error && !isBackendOffline && (
                <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">{error}</div>
              )}
              <Button type="submit" className="w-full" disabled={isFormDisabled}>
                {isLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                {isLoading ? t("adminPanel.setup") : t("adminPanel.registerButton")}
              </Button>
            </form>
          ) : (
            <form onSubmit={handleSubmit} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="login">{t("adminPanel.login")}</Label>
                <Input
                  id="login"
                  type="text"
                  autoComplete="username"
                  placeholder={t("adminPanel.loginPlaceholder")}
                  value={loginInput}
                  onChange={(e) => setLoginInput(e.target.value)}
                  disabled={isFormDisabled}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="password">{t("adminPanel.passwordPlaceholder")}</Label>
                <Input
                  id="password"
                  type="password"
                  autoComplete="current-password"
                  placeholder={t("adminPanel.passwordPlaceholder")}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  disabled={isFormDisabled}
                />
              </div>
              {isBackendOffline && (
                <div className="flex items-center gap-2 rounded-md bg-amber-500/10 p-3 text-sm text-amber-600 dark:text-amber-400">
                  <WifiOff className="h-4 w-4 shrink-0" />
                  <span>{t("adminPanel.backendOffline")}</span>
                </div>
              )}
              {error && !isBackendOffline && (
                <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">{error}</div>
              )}
              <Button type="submit" className="w-full" disabled={isFormDisabled}>
                {isLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                {isLoading ? t("adminPanel.setup") : t("adminPanel.signIn")}
              </Button>
            </form>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
