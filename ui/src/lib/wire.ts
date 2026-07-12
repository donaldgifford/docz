/*
 * docz-api marshals empty Go slices as JSON null (observed live:
 * every DocType.aliases on a real repo), while the OpenAPI spec — and
 * therefore the orval-generated types — declare plain arrays. The MSW
 * fixtures mirror the null shape so tests catch regressions. Until the
 * spec grows `nullable` for these fields (additive ask upstream),
 * normalize any wire array at the boundary before iterating it.
 */
export function arr<T>(value: readonly T[] | null | undefined): readonly T[] {
  return value ?? [];
}
