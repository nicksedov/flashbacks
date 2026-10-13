/**
 * Test helper: assert an element carries every given class token.
 *
 * Kept dependency-free (no jest-dom matchers) so the same tests run under both
 * the webapp's local Vitest config and the root focused-spec Vitest runner.
 */
export function hasClass(el: Element | null | undefined, ...classes: string[]): boolean {
  return !!el && classes.every((c) => el.classList.contains(c))
}
