import type { Table } from '@tanstack/react-table'
import { Button } from '@/components/ui/button'
import { ButtonGroup } from '@/components/ui/button-group'
import {
  onListFilterModeChange,
  type ListFilterMode,
} from '@/hooks/use-list-filter-mode'

const MODES: { value: ListFilterMode; label: string }[] = [
  { value: 'basic', label: 'Filters' },
  { value: 'advanced', label: 'Advanced filters' },
  { value: 'command', label: 'Command filters' },
]

interface ListFilterModeToggleProps<TData> {
  filterMode: ListFilterMode
  setFilterMode: (mode: ListFilterMode) => void | Promise<URLSearchParams>
  clearAdvanced: () => void
  clearBasic: () => void
  table: Table<TData>
}

export function ListFilterModeToggle<TData>({
  filterMode,
  setFilterMode,
  clearAdvanced,
  clearBasic,
  table,
}: ListFilterModeToggleProps<TData>) {
  return (
    <ButtonGroup aria-label="Filter mode">
      {MODES.map((mode) => (
        <Button
          key={mode.value}
          type="button"
          size="sm"
          variant={filterMode === mode.value ? 'secondary' : 'outline'}
          aria-pressed={filterMode === mode.value}
          onClick={() =>
            onListFilterModeChange({
              next: mode.value,
              current: filterMode,
              setFilterMode,
              clearAdvanced,
              clearBasic,
              table,
            })
          }
        >
          {mode.label}
        </Button>
      ))}
    </ButtonGroup>
  )
}
