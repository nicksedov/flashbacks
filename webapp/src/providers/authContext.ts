import { createContext } from "react"
import type { UserDTO } from "@/types"

export interface AuthContextType {
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

export const AuthContext = createContext<AuthContextType | null>(null)
