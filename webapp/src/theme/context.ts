import { createContext } from "react"

export type Theme = 
  | "light-purple"
  | "dark-purple"
  | "light-green"
  | "dark-green"
  | "light-blue"
  | "dark-blue"
  | "light-orange"
  | "dark-orange"
  | "dark-contrast"

export interface ThemeContextValue {
  theme: Theme
}

export const ThemeContext = createContext<ThemeContextValue | null>(null)
