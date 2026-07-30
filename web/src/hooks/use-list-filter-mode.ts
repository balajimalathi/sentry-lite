import type { Table } from '@tanstack/react-table'
import { parseAsStringEnum, useQueryState } from 'nuqs'
import { getFiltersStateParser } from '@/lib/parsers'
import type { ExtendedColumnFilter, JoinOperator } from '@/types/data-table'

export type ListFilterMode = 'basic' | 'advanced' | 'command'

export function useListFilterMode<TData = unknown>() {
  const [filterMode, setFilterMode] = useQueryState(
    'filterMode',
    parseAsStringEnum<ListFilterMode>([
      'basic',
      'advanced',
      'command',
    ]).withDefault('basic')
  )

  const [advancedFilters, setAdvancedFilters] = useQueryState(
    'filters',
    getFiltersStateParser<TData>().withDefault([])
  )

  const [joinOperator, setJoinOperator] = useQueryState(
    'joinOperator',
    parseAsStringEnum<JoinOperator>(['and', 'or']).withDefault('and')
  )

  const enableAdvancedFilter = filterMode !== 'basic'

  function clearAdvancedFilters() {
    void setAdvancedFilters([])
    void setJoinOperator('and')
  }

  return {
    filterMode,
    setFilterMode,
    advancedFilters: advancedFilters as ExtendedColumnFilter<TData>[],
    setAdvancedFilters,
    joinOperator,
    setJoinOperator,
    enableAdvancedFilter,
    clearAdvancedFilters,
  }
}

export function clearBasicFilterRecord(
  keys: readonly string[]
): Record<string, null> {
  return Object.fromEntries(keys.map((key) => [key, null]))
}

export function onListFilterModeChange<TData>(opts: {
  next: ListFilterMode | undefined
  current: ListFilterMode
  setFilterMode: (mode: ListFilterMode) => void | Promise<URLSearchParams>
  clearAdvanced: () => void
  clearBasic: () => void
  table: Table<TData>
}) {
  const { next, current, setFilterMode, clearAdvanced, clearBasic, table } =
    opts
  if (!next || next === current) return
  void setFilterMode(next)
  clearAdvanced()
  clearBasic()
  table.resetColumnFilters()
}
