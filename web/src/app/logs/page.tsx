import { useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import type { ColumnDef, ColumnFiltersState } from '@tanstack/react-table'
import { AlertCircleIcon } from 'lucide-react'
import { parseAsArrayOf, parseAsString, useQueryStates } from 'nuqs'
import { api, formatRelativeTime, formatTime, type LogEntry } from '@/api'
import { DataTable } from '@/components/data-table/data-table'
import { DataTableColumnHeader } from '@/components/data-table/data-table-column-header'
import { DataTableSkeleton } from '@/components/data-table/data-table-skeleton'
import { ListDataTableFilters } from '@/components/list-data-table-filters'
import {
  CreateProjectEmpty,
  PageEmpty,
  SelectProjectEmpty,
} from '@/components/page-empty'
import { PageHeader } from '@/components/page-header'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { useDataTable } from '@/hooks/use-data-table'
import { EMPTY_PROJECTS } from '@/hooks/use-project-filter'
import { firstFilterValue } from '@/lib/row-filters'

const LEVEL_OPTIONS = [
  { label: 'Trace', value: 'trace' },
  { label: 'Debug', value: 'debug' },
  { label: 'Info', value: 'info' },
  { label: 'Warn', value: 'warn' },
  { label: 'Error', value: 'error' },
  { label: 'Fatal', value: 'fatal' },
]

function levelVariant(level: string) {
  switch (level) {
    case 'error':
    case 'fatal':
      return 'destructive' as const
    case 'warn':
      return 'outline' as const
    default:
      return 'secondary' as const
  }
}

export default function LogsPage() {
  const [selected, setSelected] = useState<LogEntry | null>(null)
  const [basicFilterValues] = useQueryStates({
    project_id: parseAsArrayOf(parseAsString, ','),
    level: parseAsArrayOf(parseAsString, ','),
    body: parseAsString,
  })

  const basicColumnFilters = useMemo<ColumnFiltersState>(() => {
    const filters: ColumnFiltersState = []
    for (const key of ['project_id', 'level', 'body'] as const) {
      const value = basicFilterValues[key]
      if (value == null || value === '') continue
      filters.push({ id: key, value })
    }
    return filters
  }, [basicFilterValues])

  const projectsQuery = useQuery({
    queryKey: ['projects'],
    queryFn: () => api.projects(),
  })

  const projects = projectsQuery.data ?? EMPTY_PROJECTS
  const projectOptions = useMemo(
    () => projects.map((p) => ({ label: p.name, value: String(p.id) })),
    [projects]
  )

  const projectId = firstFilterValue(
    basicColumnFilters.find((f) => f.id === 'project_id')?.value
  )
  const levelFilter = firstFilterValue(
    basicColumnFilters.find((f) => f.id === 'level')?.value
  )
  const bodyFilter =
    (basicColumnFilters.find((f) => f.id === 'body')?.value as string | undefined) ??
    ''

  const logsQuery = useQuery({
    queryKey: ['logs', projectId, levelFilter, bodyFilter],
    queryFn: () =>
      api.logs({
        project_id: projectId,
        level: levelFilter || undefined,
        q: bodyFilter || undefined,
      }),
    enabled: !!projectId,
  })

  const columns = useMemo<ColumnDef<LogEntry>[]>(
    () => [
      {
        id: 'timestamp',
        accessorKey: 'timestamp',
        meta: { label: 'Time' },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} label="Time" />
        ),
        cell: ({ row }) => (
          <span title={formatTime(row.original.timestamp)}>
            {formatRelativeTime(row.original.timestamp)}
          </span>
        ),
      },
      {
        id: 'level',
        accessorKey: 'level',
        enableColumnFilter: true,
        meta: {
          label: 'Level',
          variant: 'select',
          options: LEVEL_OPTIONS,
        },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} label="Level" />
        ),
        cell: ({ row }) => (
          <Badge variant={levelVariant(row.original.level)}>
            {row.original.level}
          </Badge>
        ),
      },
      {
        id: 'body',
        accessorKey: 'body',
        enableColumnFilter: true,
        meta: {
          label: 'Message',
          placeholder: 'Search logs...',
          variant: 'text',
        },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} label="Message" />
        ),
        cell: ({ row }) => (
          <Button
            type="button"
            variant="link"
            className="h-auto max-w-xl justify-start px-0 text-left font-normal whitespace-normal"
            onClick={() => setSelected(row.original)}
          >
            <span className="line-clamp-2">{row.original.body || '—'}</span>
          </Button>
        ),
      },
      {
        id: 'origin',
        accessorKey: 'origin',
        meta: { label: 'Origin' },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} label="Origin" />
        ),
        cell: ({ row }) => (
          <span className="font-mono text-xs text-muted-foreground">
            {row.original.origin || '—'}
          </span>
        ),
      },
      {
        id: 'environment',
        accessorKey: 'environment',
        meta: { label: 'Environment' },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} label="Environment" />
        ),
        cell: ({ row }) => row.original.environment || '—',
      },
      {
        id: 'project_id',
        accessorKey: 'project_id',
        enableColumnFilter: true,
        enableHiding: true,
        meta: {
          label: 'Project',
          variant: 'select',
          options: projectOptions,
        },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} label="Project" />
        ),
        cell: ({ row }) => {
          const project = projects.find((p) => p.id === row.original.project_id)
          return (
            <span className="text-muted-foreground">
              {project?.name ?? row.original.project_id}
            </span>
          )
        },
        filterFn: (row, _id, value) => {
          const selected = Array.isArray(value)
            ? value.map(String)
            : [String(value)]
          return selected.includes(String(row.original.project_id))
        },
      },
      {
        id: 'trace_id',
        accessorKey: 'trace_id',
        meta: { label: 'Trace' },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} label="Trace" />
        ),
        cell: ({ row }) =>
          row.original.trace_id ? (
            <Link
              to={`/traces/${row.original.trace_id}`}
              className="font-mono text-xs underline-offset-4 hover:underline"
            >
              {row.original.trace_id.slice(0, 8)}
            </Link>
          ) : (
            '—'
          ),
      },
    ],
    [projectOptions, projects]
  )

  const { table } = useDataTable({
    data: logsQuery.data ?? [],
    columns,
    pageCount: -1,
    enableAdvancedFilter: false,
    manualFiltering: false,
    manualPagination: false,
    manualSorting: false,
    initialState: {
      sorting: [{ id: 'timestamp', desc: true }],
      pagination: { pageIndex: 0, pageSize: 20 },
    },
  })

  const error = logsQuery.error
    ? String(logsQuery.error)
    : projectsQuery.error
      ? String(projectsQuery.error)
      : ''

  if (error) {
    return (
      <Alert variant="destructive">
        <AlertCircleIcon />
        <AlertTitle>Failed to load logs</AlertTitle>
        <AlertDescription>{error}</AlertDescription>
      </Alert>
    )
  }

  const loading = projectsQuery.isLoading || (!!projectId && logsQuery.isLoading)
  const rows = logsQuery.data ?? []

  return (
    <section className="flex flex-col gap-4">
      <PageHeader
        title="Logs"
        description="Structured logs from enableLogs and consoleLoggingIntegration."
      />

      {loading ? (
        <DataTableSkeleton columnCount={6} filterCount={3} rowCount={8} />
      ) : projects.length === 0 ? (
        <CreateProjectEmpty />
      ) : !projectId ? (
        <SelectProjectEmpty />
      ) : rows.length === 0 ? (
        <PageEmpty
          title="No logs yet"
          description="Set enableLogs: true and add consoleLoggingIntegration (or Sentry.logger) on your Sentry SDK, then send traffic."
        />
      ) : (
        <DataTable table={table}>
          <ListDataTableFilters table={table} />
        </DataTable>
      )}

      <LogDetailSheet log={selected} onOpenChange={(open) => !open && setSelected(null)} />
    </section>
  )
}

function LogDetailSheet({
  log,
  onOpenChange,
}: {
  log: LogEntry | null
  onOpenChange: (open: boolean) => void
}) {
  const attrs = log?.attributes ?? {}
  const attrKeys = Object.keys(attrs)

  return (
    <Sheet open={Boolean(log)} onOpenChange={onOpenChange}>
      <SheetContent side="right" className="w-full sm:max-w-lg">
        <SheetHeader>
          <SheetTitle>Log</SheetTitle>
          <SheetDescription>
            {log ? `${log.level} · ${formatRelativeTime(log.timestamp)}` : 'Log details'}
          </SheetDescription>
        </SheetHeader>
        {log ? (
          <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-4 pb-4">
            <Badge variant={levelVariant(log.level)}>{log.level}</Badge>
            <p className="text-sm whitespace-pre-wrap">{log.body || '—'}</p>
            <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-sm">
              <dt className="text-muted-foreground">Time</dt>
              <dd title={formatTime(log.timestamp)}>
                {formatRelativeTime(log.timestamp)}
              </dd>
              <dt className="text-muted-foreground">Origin</dt>
              <dd className="font-mono text-xs">{log.origin || '—'}</dd>
              <dt className="text-muted-foreground">Environment</dt>
              <dd>{log.environment || '—'}</dd>
              <dt className="text-muted-foreground">Release</dt>
              <dd className="font-mono text-xs">{log.release || '—'}</dd>
              {log.trace_id ? (
                <>
                  <dt className="text-muted-foreground">Trace</dt>
                  <dd>
                    <Link
                      to={`/traces/${log.trace_id}`}
                      className="font-mono text-xs underline-offset-4 hover:underline"
                    >
                      {log.trace_id}
                    </Link>
                  </dd>
                </>
              ) : null}
            </dl>
            {attrKeys.length > 0 ? (
              <div className="flex flex-col gap-2">
                <h3 className="text-sm font-medium">Attributes</h3>
                <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 font-mono text-xs">
                  {attrKeys.map((k) => (
                    <div key={k} className="contents">
                      <dt className="text-muted-foreground">{k}</dt>
                      <dd className="break-all">{formatAttr(attrs[k])}</dd>
                    </div>
                  ))}
                </dl>
              </div>
            ) : null}
          </div>
        ) : null}
      </SheetContent>
    </Sheet>
  )
}

function formatAttr(v: unknown) {
  if (v == null) return '—'
  if (typeof v === 'string') return v
  try {
    return JSON.stringify(v)
  } catch {
    return String(v)
  }
}
