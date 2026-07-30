import { useMemo } from 'react'
import { Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import type { ColumnDef, ColumnFiltersState } from '@tanstack/react-table'
import { AlertCircleIcon } from 'lucide-react'
import { parseAsArrayOf, parseAsString, useQueryStates } from 'nuqs'
import { api, formatTime, type Issue } from '@/api'
import { DataTable } from '@/components/data-table/data-table'
import { DataTableColumnHeader } from '@/components/data-table/data-table-column-header'
import { ListDataTableFilters } from '@/components/list-data-table-filters'
import { ListFilterModeToggle } from '@/components/list-filter-mode-toggle'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { useDataTable } from '@/hooks/use-data-table'
import {
  clearBasicFilterRecord,
  useListFilterMode,
} from '@/hooks/use-list-filter-mode'
import {
  applyAdvancedIssueFilters,
  columnFiltersToIssueParams,
} from '@/lib/issue-filters'
import { statusVariant } from '@/lib/status'

const BASIC_FILTER_KEYS = [
  'title',
  'status',
  'project_id',
  'environment',
  'release',
  'tag',
  'last_seen',
] as const

export default function IssuesPage() {
  const {
    filterMode,
    setFilterMode,
    advancedFilters,
    joinOperator,
    enableAdvancedFilter,
    clearAdvancedFilters,
  } = useListFilterMode<Issue>()

  const [basicFilterValues, setBasicFilterValues] = useQueryStates({
    title: parseAsString,
    status: parseAsArrayOf(parseAsString, ','),
    project_id: parseAsArrayOf(parseAsString, ','),
    environment: parseAsArrayOf(parseAsString, ','),
    release: parseAsArrayOf(parseAsString, ','),
    tag: parseAsArrayOf(parseAsString, ','),
    last_seen: parseAsString,
  })

  const basicColumnFilters = useMemo<ColumnFiltersState>(() => {
    const filters: ColumnFiltersState = []
    for (const key of BASIC_FILTER_KEYS) {
      const value = basicFilterValues[key]
      if (value == null || value === '') continue
      if (key === 'last_seen' && typeof value === 'string') {
        try {
          const parsed = JSON.parse(value) as unknown
          filters.push({
            id: key,
            value: Array.isArray(parsed) ? parsed : [value],
          })
        } catch {
          const parts = value.split(',').filter(Boolean)
          filters.push({ id: key, value: parts.length > 1 ? parts : value })
        }
        continue
      }
      filters.push({ id: key, value })
    }
    return filters
  }, [basicFilterValues])

  const apiParams = useMemo(
    () =>
      columnFiltersToIssueParams({
        mode: filterMode,
        columnFilters: basicColumnFilters,
        advancedFilters,
      }),
    [filterMode, basicColumnFilters, advancedFilters]
  )

  const projectIdForFacets = apiParams.project_id ?? ''

  const projectsQuery = useQuery({
    queryKey: ['projects'],
    queryFn: () => api.projects(),
  })

  const facetsQuery = useQuery({
    queryKey: ['facets', projectIdForFacets],
    queryFn: () => api.facets(projectIdForFacets || undefined),
    placeholderData: (previousData) => previousData,
  })

  const issuesQuery = useQuery({
    queryKey: ['issues', apiParams],
    queryFn: () => api.issues(apiParams),
  })

  const projects = projectsQuery.data ?? []
  const facets = facetsQuery.data

  const projectOptions = useMemo(
    () => projects.map((p) => ({ label: p.name, value: String(p.id) })),
    [projects]
  )
  const envOptions = useMemo(
    () => (facets?.environments ?? []).map((v) => ({ label: v, value: v })),
    [facets]
  )
  const releaseOptions = useMemo(
    () => (facets?.releases ?? []).map((v) => ({ label: v, value: v })),
    [facets]
  )
  const tagOptions = useMemo(
    () => (facets?.tags ?? []).map((v) => ({ label: v, value: v })),
    [facets]
  )
  const statusOptions = useMemo(
    () =>
      ['open', 'resolved', 'ignored'].map((v) => ({
        label: v,
        value: v,
      })),
    []
  )

  const columns = useMemo<ColumnDef<Issue>[]>(
    () => [
      {
        id: 'title',
        accessorKey: 'title',
        enableColumnFilter: true,
        meta: {
          label: 'Title',
          placeholder: 'Search titles...',
          variant: 'text',
        },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} label="Title" />
        ),
        cell: ({ row }) => (
          <div className="flex max-w-xs flex-wrap items-center gap-2 whitespace-normal">
            <Link
              to={`/issues/${row.original.id}`}
              className="font-medium text-primary underline-offset-4 hover:underline"
            >
              {row.original.title}
            </Link>
            {row.original.regressed && (
              <Badge variant="outline">regressed</Badge>
            )}
          </div>
        ),
      },
      {
        id: 'status',
        accessorKey: 'status',
        enableColumnFilter: true,
        meta: {
          label: 'Status',
          variant: 'select',
          options: statusOptions,
        },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} label="Status" />
        ),
        cell: ({ row }) => (
          <Badge variant={statusVariant(row.original.status)}>
            {row.original.status}
          </Badge>
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
        id: 'environment',
        accessorFn: (r) => r.environments ?? [],
        enableColumnFilter: true,
        enableHiding: true,
        meta: {
          label: 'Environment',
          variant: 'select',
          options: envOptions,
        },
        filterFn: (row, _id, value) => {
          const selected = Array.isArray(value)
            ? value.map(String)
            : [String(value)]
          const envs = row.original.environments ?? []
          return selected.some((v) => envs.includes(v))
        },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} label="Environment" />
        ),
        cell: ({ row }) => (
          <span className="text-muted-foreground">
            {(row.original.environments ?? []).join(', ') || '—'}
          </span>
        ),
      },
      {
        id: 'release',
        accessorFn: (r) => r.last_release ?? r.first_release ?? '',
        enableColumnFilter: true,
        enableHiding: true,
        meta: {
          label: 'Release',
          variant: 'select',
          options: releaseOptions,
        },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} label="Release" />
        ),
        cell: ({ row }) => (
          <span className="font-mono text-muted-foreground text-xs">
            {row.original.last_release ?? row.original.first_release ?? '—'}
          </span>
        ),
      },
      {
        id: 'tag',
        accessorFn: () => '',
        enableColumnFilter: true,
        enableSorting: false,
        enableHiding: true,
        meta: {
          label: 'Tag',
          variant: 'select',
          options: tagOptions,
        },
        filterFn: () => true,
        header: ({ column }) => (
          <DataTableColumnHeader column={column} label="Tag" />
        ),
        cell: () => <span className="text-muted-foreground">—</span>,
      },
      {
        id: 'owner',
        accessorFn: (r) => r.assignee ?? '',
        meta: { label: 'Owner' },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} label="Owner" />
        ),
        cell: ({ row }) => (
          <span className="text-muted-foreground">
            {row.original.assignee || '—'}
          </span>
        ),
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
        id: 'first_seen',
        accessorKey: 'first_seen',
        meta: { label: 'First seen' },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} label="First seen" />
        ),
        cell: ({ row }) => (
          <span className="text-muted-foreground">
            {formatTime(row.original.first_seen)}
          </span>
        ),
      },
      {
        id: 'last_seen',
        accessorKey: 'last_seen',
        enableColumnFilter: true,
        meta: {
          label: 'Last seen',
          variant: 'dateRange',
        },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} label="Last seen" />
        ),
        cell: ({ row }) => (
          <span className="text-muted-foreground">
            {formatTime(row.original.last_seen)}
          </span>
        ),
      },
      {
        id: 'culprit',
        accessorKey: 'culprit',
        meta: { label: 'Culprit' },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} label="Culprit" />
        ),
        cell: ({ row }) => (
          <span className="max-w-[12rem] truncate font-mono text-muted-foreground">
            {row.original.culprit || '—'}
          </span>
        ),
      },
    ],
    [
      statusOptions,
      projectOptions,
      envOptions,
      releaseOptions,
      tagOptions,
      projects,
    ]
  )

  const rawIssues = issuesQuery.data ?? []
  const issues = useMemo(() => {
    if (filterMode === 'basic') return rawIssues
    return applyAdvancedIssueFilters(rawIssues, advancedFilters, joinOperator)
  }, [filterMode, rawIssues, advancedFilters, joinOperator])

  const { table } = useDataTable({
    data: issues,
    columns,
    pageCount: -1,
    enableAdvancedFilter,
    manualFiltering: false,
    manualPagination: false,
    manualSorting: false,
    initialState: {
      sorting: [{ id: 'last_seen', desc: true }],
      pagination: { pageIndex: 0, pageSize: 20 },
      columnVisibility: {
        project_id: false,
        environment: false,
        release: false,
        tag: false,
      },
    },
  })

  const error = issuesQuery.error ? String(issuesQuery.error) : ''

  if (error) {
    return (
      <Alert variant="destructive">
        <AlertCircleIcon />
        <AlertTitle>Failed to load issues</AlertTitle>
        <AlertDescription>{error}</AlertDescription>
      </Alert>
    )
  }

  return (
    <section className="flex flex-col gap-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
        <div className="flex flex-col gap-1">
          <h1 className="font-heading text-2xl font-medium tracking-tight">
            Issues
          </h1>
          <p className="text-sm text-muted-foreground">
            Filter with toolbar chips, advanced rules, or a command menu.
          </p>
        </div>
        <ListFilterModeToggle
          filterMode={filterMode}
          setFilterMode={setFilterMode}
          clearAdvanced={clearAdvancedFilters}
          clearBasic={() =>
            void setBasicFilterValues(clearBasicFilterRecord(BASIC_FILTER_KEYS))
          }
          table={table}
        />
      </div>

      {issuesQuery.isLoading ? (
        <Skeleton className="h-48 w-full" />
      ) : (
        <DataTable table={table}>
          <ListDataTableFilters table={table} filterMode={filterMode} />
        </DataTable>
      )}
    </section>
  )
}
