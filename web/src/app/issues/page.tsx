import { useMemo } from 'react'
import { Link } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { ColumnDef, ColumnFiltersState } from '@tanstack/react-table'
import { AlertCircleIcon, MoreHorizontalIcon } from 'lucide-react'
import { parseAsArrayOf, parseAsString, useQueryStates } from 'nuqs'
import { api, formatRelativeTime, formatTime, type Issue } from '@/api'
import { DataTable } from '@/components/data-table/data-table'
import { DataTableColumnHeader } from '@/components/data-table/data-table-column-header'
import { DataTableSkeleton } from '@/components/data-table/data-table-skeleton'
import { ListDataTableFilters } from '@/components/list-data-table-filters'
import { ListFilterModeToggle } from '@/components/list-filter-mode-toggle'
import { PageEmpty } from '@/components/page-empty'
import { PageHeader } from '@/components/page-header'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { useDataTable } from '@/hooks/use-data-table'
import {
  clearBasicFilterRecord,
  useListFilterMode,
} from '@/hooks/use-list-filter-mode'
import {
  applyAdvancedIssueFilters,
  columnFiltersToIssueParams,
} from '@/lib/issue-filters'
import { EMPTY_PROJECTS } from '@/hooks/use-project-filter'
import { toTitleCase } from '@/lib/format'
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

const EMPTY_ISSUES: Issue[] = []

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

  const projects = projectsQuery.data ?? EMPTY_PROJECTS
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
        size: 280,
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
          <div className="flex max-w-sm flex-wrap items-center gap-2 whitespace-normal">
            <Link
              to={`/issues/${row.original.id}`}
              className="font-medium underline-offset-4 hover:underline"
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
            {toTitleCase(row.original.status)}
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
        cell: ({ row }) => {
          const envs = row.original.environments ?? []
          if (envs.length === 0) {
            return <span className="text-muted-foreground">—</span>
          }
          return (
            <div className="flex max-w-40 flex-wrap gap-1">
              {envs.map((env) => (
                <Badge key={env} variant="outline" className="font-normal">
                  {env}
                </Badge>
              ))}
            </div>
          )
        },
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
        accessorFn: (r) => r.tags ?? [],
        enableColumnFilter: true,
        enableSorting: false,
        enableHiding: true,
        meta: {
          label: 'Tags',
          variant: 'select',
          options: tagOptions,
        },
        filterFn: (row, _id, value) => {
          const selected = Array.isArray(value)
            ? value.map(String)
            : [String(value)]
          const tags = row.original.tags ?? []
          return selected.some((v) => tags.includes(v))
        },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} label="Tags" />
        ),
        cell: ({ row }) => {
          const tags = row.original.tags ?? []
          if (tags.length === 0) {
            return <span className="text-muted-foreground">—</span>
          }
          const shown = tags.slice(0, 3)
          return (
            <div className="flex max-w-xs flex-wrap items-center gap-1">
              {shown.map((tag) => (
                <Badge
                  key={tag}
                  variant="outline"
                  className="max-w-32 truncate font-mono text-xs font-normal"
                >
                  {tag}
                </Badge>
              ))}
              {tags.length > 3 && (
                <span className="text-xs text-muted-foreground">
                  +{tags.length - 3}
                </span>
              )}
            </div>
          )
        },
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
          <span
            className="text-muted-foreground"
            title={formatTime(row.original.first_seen)}
          >
            {formatRelativeTime(row.original.first_seen)}
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
          <span
            className="text-muted-foreground"
            title={formatTime(row.original.last_seen)}
          >
            {formatRelativeTime(row.original.last_seen)}
          </span>
        ),
      },
      {
        id: 'actions',
        enableSorting: false,
        enableHiding: false,
        header: () => <span className="sr-only">Actions</span>,
        cell: ({ row }) => <IssueRowActions issue={row.original} />,
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

  const rawIssues = issuesQuery.data ?? EMPTY_ISSUES
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
      <PageHeader
        title="Issues"
        description="Browse and filter error groups."
        actions={
          <ListFilterModeToggle
            filterMode={filterMode}
            setFilterMode={setFilterMode}
            clearAdvanced={clearAdvancedFilters}
            clearBasic={() =>
              void setBasicFilterValues(clearBasicFilterRecord(BASIC_FILTER_KEYS))
            }
            table={table}
          />
        }
      />

      {issuesQuery.isLoading ? (
        <DataTableSkeleton columnCount={8} filterCount={4} rowCount={8} />
      ) : rawIssues.length === 0 ? (
        <PageEmpty
          title="No issues yet"
          description="Send an event with your project DSN, or clear filters if you expected results."
          action={
            <Link
              to="/projects"
              className="text-sm font-medium underline-offset-4 hover:underline"
            >
              Copy a DSN from Projects
            </Link>
          }
        />
      ) : (
        <DataTable table={table}>
          <ListDataTableFilters table={table} filterMode={filterMode} />
        </DataTable>
      )}
    </section>
  )
}

function IssueRowActions({ issue }: { issue: Issue }) {
  const qc = useQueryClient()
  const mutation = useMutation({
    mutationFn: (status: string) => api.updateStatus(issue.id, status),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['issues'] })
      void qc.invalidateQueries({ queryKey: ['issue', String(issue.id)] })
    },
  })

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            aria-label={`Triage ${issue.title}`}
          />
        }
      >
        <MoreHorizontalIcon />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem
          disabled={mutation.isPending || issue.status === 'resolved'}
          onClick={() => mutation.mutate('resolved')}
        >
          Resolve
        </DropdownMenuItem>
        <DropdownMenuItem
          disabled={mutation.isPending || issue.status === 'ignored'}
          onClick={() => mutation.mutate('ignored')}
        >
          Ignore
        </DropdownMenuItem>
        <DropdownMenuItem
          disabled={mutation.isPending || issue.status === 'open'}
          onClick={() => mutation.mutate('open')}
        >
          Reopen
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
