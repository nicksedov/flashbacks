import { useCallback, useEffect, useState } from "react"
import { fetchAuthStatus, logout as apiLogout } from "@/api/endpoints"
import { AuthContext } from "./authContext"
import type { UserDTO } from "@/types"

interface AuthProviderProps {
  children: React.ReactNode
}

export function AuthProvider({ children }: AuthProviderProps) {
  const [user, setUser] = useState<UserDTO | null>(null)
  const [isBootstrapMode, setIsBootstrapMode] = useState(false)
  const [isBootstrapVerified, setIsBootstrapVerified] = useState(false)
  const [isLoading, setIsLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      try {
        const status = await fetchAuthStatus()
        if (cancelled) return
        if (status.isAuthenticated && status.user) {
          setUser(status.user)
          setIsBootstrapMode(false)
        } else {
          setUser(null)
          setIsBootstrapMode(status.isBootstrapMode)
        }
      } catch {
        if (cancelled) return
        setUser(null)
        setIsBootstrapMode(false)
      } finally {
        if (!cancelled) setIsLoading(false)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [])

  const login = useCallback((loggedInUser: UserDTO) => {
    setUser(loggedInUser)
    setIsBootstrapMode(false)
    setIsBootstrapVerified(false)
  }, [])

  const logout = useCallback(async () => {
    try {
      await apiLogout()
    } catch {
      // Ignore logout errors
    } finally {
      setUser(null)
    }
  }, [])

  const updateUser = useCallback((updatedUser: UserDTO) => {
    setUser(updatedUser)
  }, [])

  const setBootstrapVerified = useCallback((verified: boolean) => {
    setIsBootstrapVerified(verified)
  }, [])

  return (
    <AuthContext.Provider
      value={{
        user,
        isAuthenticated: !!user,
        isBootstrapMode,
        isBootstrapVerified,
        isLoading,
        login,
        logout,
        updateUser,
        setBootstrapVerified,
      }}
    >
      {children}
    </AuthContext.Provider>
  )
}
