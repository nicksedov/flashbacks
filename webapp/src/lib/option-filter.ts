/**
 * A single searchable option. The filter only looks at `label`, so any value
 * type carried alongside it stays opaque to the search.
 */
export type FilterableOption = { label: string }

/**
 * Narrows a list of options by a free-text query.
 *
 * Semantics:
 * - the query is trimmed first, so whitespace-only input counts as empty;
 * - an empty query returns every option (the same array instance, in input order);
 * - otherwise an option is kept when its `label` contains the query as a
 *   case-insensitive substring.
 *
 * Input order is always preserved — the filter never sorts or deduplicates.
 */
export function filterOptionsByQuery<T extends FilterableOption>(
  options: readonly T[],
  query: string,
): readonly T[] {
  const needle = query.trim().toLowerCase()
  if (needle === "") return options
  return options.filter((option) => option.label.toLowerCase().includes(needle))
}
