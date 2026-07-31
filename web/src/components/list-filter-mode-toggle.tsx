import type { Table } from '@tanstack/react-table'
import {
  ListFilterIcon,
  SlidersHorizontalIcon,
  TerminalIcon,
} from 'lucide-react'
import { PageHeaderActionLabel } from '@/components/page-header'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import {
  onListFilterModeChange,
  type ListFilterMode,
} from '@/hooks/use-list-filter-mode'

const MODES: {
  value: ListFilterMode
  label: string
  icon: typeof ListFilterIcon
}[] = [
  { value: 'basic', label: 'Filters', icon: ListFilterIcon },
  {
    value: 'advanced',
    label: 'Advanced filters',
    icon: SlidersHorizontalIcon,
  },
  { value: 'command', label: 'Command filters', icon: TerminalIcon },
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
  function handleValueChange(next: string[]) {
    onListFilterModeChange({
      next: next[0] as ListFilterMode | undefined,
      current: filterMode,
      setFilterMode,
      clearAdvanced,
      clearBasic,
      table,
    })
  }

  return (
    <ToggleGroup
      value={[filterMode]}
      onValueChange={handleValueChange}
      variant="outline"
      size="sm"
      spacing={0}
      aria-label="Filter mode"
    >
      {MODES.map((mode) => {
        const Icon = mode.icon
        return (
          <ToggleGroupItem
            key={mode.value}
            value={mode.value}
            aria-label={mode.label}
          >
            <Icon data-icon="inline-start" />
            <PageHeaderActionLabel>{mode.label}</PageHeaderActionLabel>
          </ToggleGroupItem>
        )
      })}
    </ToggleGroup>
  )
}
