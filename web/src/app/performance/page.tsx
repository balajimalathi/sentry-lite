import { useMemo } from 'react'
import { Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import type { ColumnDef, ColumnFiltersState } from '@tanstack/react-table'
import { AlertCircleIcon } from 'lucide-react'
import { parseAsArrayOf, parseAsString, useQueryStates } from 'nuqs'
import { api, type TransactionSummary } from '@/api'
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
import { useDataTable } from '@/hooks/use-data-table'
import { EMPTY_PROJECTS } from '@/hooks/use-project-filter'
import { firstFilterValue } from '@/lib/row-filters'

function fmtMs(n: number) {
  if (!Number.isFinite(n)) return '—'
  if (n < 1) return `${n.toFixed(2)} ms`
  if (n < 1000) return `${n.toFixed(1)} ms`
  return `${(n / 1000).toFixed(2)} s`
}

export default function PerformancePage() {
  const [basicFilterValues] = useQueryStates({
    project_id: parseAsArrayOf(parseAsString, ','),
    name: parseAsString,
  })

  const basicColumnFilters = useMemo<ColumnFiltersState>(() => {
    const filters: ColumnFiltersState = []
    for (const key of ['project_id', 'name'] as const) {
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

  const transactionsQuery = useQuery({
    queryKey: ['transactions', projectId],
    queryFn: () => api.transactions(projectId),
    enabled: !!projectId,
  })

  const columns = useMemo<ColumnDef<TransactionSummary>[]>(
    () => [
      {
        id: 'name',
        accessorKey: 'name',
        enableColumnFilter: true,
        meta: {
          label: 'Transaction',
          placeholder: 'Search transactions...',
          variant: 'text',
        },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} label="Transaction" />
        ),
        cell: ({ row }) => (
          <Link
            to={`/performance/${encodeURIComponent(row.original.name)}?project_id=${projectId}`}
            className="font-mono text-sm underline-offset-4 hover:underline"
          >
            {row.original.name}
          </Link>
        ),
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
        id: 'count',
        accessorKey: 'count',
        meta: { label: 'Count' },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} label="Count" />
        ),
      },
      {
        id: 'p95_ms',
        accessorKey: 'p95_ms',
        meta: { label: 'p95' },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} label="p95" />
        ),
        cell: ({ row }) => fmtMs(row.original.p95_ms),
      },
      {
        id: 'p99_ms',
        accessorKey: 'p99_ms',
        meta: { label: 'p99' },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} label="p99" />
        ),
        cell: ({ row }) => fmtMs(row.original.p99_ms),
      },
    ],
    [projectOptions, projectId, projects]
  )

  const { table } = useDataTable({
    data: transactionsQuery.data ?? [],
    columns,
    pageCount: -1,
    enableAdvancedFilter: false,
    manualFiltering: false,
    manualPagination: false,
    manualSorting: false,
    initialState: {
      sorting: [{ id: 'p95_ms', desc: true }],
      pagination: { pageIndex: 0, pageSize: 20 },
    },
  })

  const error = transactionsQuery.error
    ? String(transactionsQuery.error)
    : projectsQuery.error
      ? String(projectsQuery.error)
      : ''

  if (error) {
    return (
      <Alert variant="destructive">
        <AlertCircleIcon />
        <AlertTitle>Failed to load performance</AlertTitle>
        <AlertDescription>{error}</AlertDescription>
      </Alert>
    )
  }

  const loading = projectsQuery.isLoading || (!!projectId && transactionsQuery.isLoading)
  const rows = transactionsQuery.data ?? []

  return (
    <section className="flex flex-col gap-4">
      <PageHeader
        title="Performance"
        description="Transaction latency (p95 / p99)."
      />

      {loading ? (
        <DataTableSkeleton columnCount={5} filterCount={2} rowCount={8} />
      ) : projects.length === 0 ? (
        <CreateProjectEmpty />
      ) : !projectId ? (
        <SelectProjectEmpty />
      ) : rows.length === 0 ? (
        <PageEmpty
          title="No transactions yet"
          description="Enable tracing in your Sentry SDK (tracesSampleRate) and send traffic."
        />
      ) : (
        <DataTable table={table}>
          <ListDataTableFilters table={table} />
        </DataTable>
      )}
    </section>
  )
}
