import { createContext } from "react"
import type { AccountCreationMode, UserDTO } from "@/types"

export interface AuthContextType {
  user: UserDTO | null
  isAuthenticated: boolean
  isBootstrapMode: boolean
  isBootstrapVerified: boolean
  accountCreationMode: AccountCreationMode
  isLoading: boolean
  login: (user: UserDTO) => void
  logout: () => Promise<void>
  updateUser: (user: UserDTO) => void
  setBootstrapVerified: (verified: boolean) => void
}

export const AuthContext = createContext<AuthContextType | null>(null)
