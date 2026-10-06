import type { Theme } from "./context"

/** Default theme used before an account is authenticated (for example the login screen). */
export const DEFAULT_THEME: Theme = "light-purple"

/**
 * Pick the active theme: the authenticated account's theme, otherwise the default theme.
 *
 * The account theme is intentionally ignored while unauthenticated so that the
 * authorization screen never inherits a previous user's theme after logout.
 */
export function selectActiveTheme(isAuthenticated: boolean, accountTheme: Theme): Theme {
  return isAuthenticated ? accountTheme : DEFAULT_THEME
}
