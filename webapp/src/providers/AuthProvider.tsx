import { createContext, useCallback, useContext, useEffect, useState } from "react"
import { fetchAuthStatus, logout as apiLogout } from "@/api/endpoints"
import type { UserDTO } from "@/types"

interface AuthContextType {
  user: UserDTO | null
  isAuthenticated: boolean
  isBootstrapMode: boolean
  isBootstrapVerified: boolean
  isLoading: boolean
  login: (user: UserDTO) => void
  logout: () => Promise<void>
  updateUser: (user: UserDTO) => void
  setBootstrapVerified: (verified: boolean) => void
}

const AuthContext = createContext<AuthContextType | null>(null)

interface AuthProviderProps {
  children: React.ReactNode
}

export function AuthProvider({ children }: AuthProviderProps) {
  const [user, setUser] = useState<UserDTO | null>(null)
  const [isBootstrapMode, setIsBootstrapMode] = useState(false)
  const [isBootstrapVerified, setIsBootstrapVerified] = useState(false)
  const [isLoading, setIsLoading] = useState(true)

  const checkAuthStatus = useCallback(async () => {
    try {
      const status = await fetchAuthStatus()
      if (status.isAuthenticated && status.user) {
        setUser(status.user)
        setIsBootstrapMode(false)
      } else {
        setUser(null)
        setIsBootstrapMode(status.isBootstrapMode)
      }
    } catch {
      setUser(null)
      setIsBootstrapMode(false)
    } finally {
      setIsLoading(false)
    }
  }, [])

  useEffect(() => {
    checkAuthStatus()
  }, [checkAuthStatus])

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

export function useAuth(): AuthContextType {
  const context = useContext(AuthContext)
  if (!context) {
    throw new Error("useAuth must be used within an AuthProvider")
  }
  return context
}
