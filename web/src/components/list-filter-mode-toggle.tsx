import type { Table } from '@tanstack/react-table'
import {
  ListFilterIcon,
  SlidersHorizontalIcon,
  TerminalIcon,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { ButtonGroup } from '@/components/ui/button-group'
import { PageHeaderActionLabel } from '@/components/page-header'
import {
  onListFilterModeChange,
  type ListFilterMode,
} from '@/hooks/use-list-filter-mode'
import { cn } from '@/lib/utils'

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
  return (
    <ButtonGroup aria-label="Filter mode">
      {MODES.map((mode) => {
        const Icon = mode.icon
        return (
          <Button
            key={mode.value}
            type="button"
            size="sm"
            variant="outline"
            aria-label={mode.label}
            aria-pressed={filterMode === mode.value}
            className={cn(
              filterMode === mode.value &&
                'bg-primary/25 text-foreground hover:bg-primary/25 hover:text-foreground',
            )}
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
            <Icon data-icon="inline-start" />
            <PageHeaderActionLabel>{mode.label}</PageHeaderActionLabel>
          </Button>
        )
      })}
    </ButtonGroup>
  )
}
