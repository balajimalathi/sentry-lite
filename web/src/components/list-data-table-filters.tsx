import type { Table } from '@tanstack/react-table'
import { DataTableAdvancedToolbar } from '@/components/data-table/data-table-advanced-toolbar'
import { DataTableFilterList } from '@/components/data-table/data-table-filter-list'
import { DataTableFilterMenu } from '@/components/data-table/data-table-filter-menu'
import { DataTableSortList } from '@/components/data-table/data-table-sort-list'
import { DataTableToolbar } from '@/components/data-table/data-table-toolbar'
import type { ListFilterMode } from '@/hooks/use-list-filter-mode'

interface ListDataTableFiltersProps<TData> {
  table: Table<TData>
  /** Defaults to basic toolbar chips. Only Issues uses advanced / command. */
  filterMode?: ListFilterMode
}

export function ListDataTableFilters<TData>({
  table,
  filterMode = 'basic',
}: ListDataTableFiltersProps<TData>) {
  if (filterMode === 'basic') {
    return (
      <DataTableToolbar table={table}>
        <DataTableSortList table={table} />
      </DataTableToolbar>
    )
  }

  return (
    <DataTableAdvancedToolbar table={table}>
      {filterMode === 'advanced' ? (
        <DataTableFilterList table={table} />
      ) : (
        <DataTableFilterMenu table={table} />
      )}
      <DataTableSortList table={table} />
    </DataTableAdvancedToolbar>
  )
}
