import { useMemo } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import {
  flexRender,
  getCoreRowModel,
  getSortedRowModel,
  useReactTable,
  type ColumnDef,
  type SortingState,
} from '@tanstack/react-table'
import { useState } from 'react'
import { AlertCircleIcon, SearchIcon } from 'lucide-react'
import { api, formatTime, type Issue } from '@/api'
import { DateTimePicker } from '@/components/date-time-picker'
import { statusVariant } from '@/lib/status'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

const ALL = 'all'

export default function IssuesPage() {
  const [params, setParams] = useSearchParams()
  const [sorting, setSorting] = useState<SortingState>([
    { id: 'last_seen', desc: true },
  ])

  const projectId = params.get('project_id') ?? ''
  const environment = params.get('environment') ?? ''
  const release = params.get('release') ?? ''
  const q = params.get('q') ?? ''
  const tag = params.get('tag') ?? ''
  const from = params.get('from') ?? ''
  const to = params.get('to') ?? ''

  function patchParams(patch: Record<string, string>) {
    const next = new URLSearchParams(params)
    Object.entries(patch).forEach(([k, v]) => {
      if (!v || v === ALL) next.delete(k)
      else next.set(k, v)
    })
    setParams(next)
  }

  const projectsQuery = useQuery({
    queryKey: ['projects'],
    queryFn: () => api.projects(),
  })

  const facetsQuery = useQuery({
    queryKey: ['facets', projectId],
    queryFn: () => api.facets(projectId || undefined),
  })

  const issuesQuery = useQuery({
    queryKey: ['issues', { projectId, environment, release, q, tag, from, to }],
    queryFn: () =>
      api.issues({
        project_id: projectId,
        environment,
        release,
        q,
        tag,
        from,
        to,
      }),
  })

  const columns = useMemo<ColumnDef<Issue>[]>(
    () => [
      {
        accessorKey: 'title',
        header: 'Title',
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
        accessorKey: 'status',
        header: 'Status',
        cell: ({ row }) => (
          <Badge variant={statusVariant(row.original.status)}>
            {row.original.status}
          </Badge>
        ),
      },
      {
        id: 'owner',
        accessorFn: (r) => r.assignee ?? '',
        header: 'Owner',
        cell: ({ row }) => (
          <span className="text-muted-foreground">
            {row.original.assignee || '—'}
          </span>
        ),
      },
      {
        accessorKey: 'count',
        header: 'Count',
      },
      {
        accessorKey: 'first_seen',
        header: 'First seen',
        cell: ({ row }) => (
          <span className="text-muted-foreground">
            {formatTime(row.original.first_seen)}
          </span>
        ),
      },
      {
        accessorKey: 'last_seen',
        header: 'Last seen',
        cell: ({ row }) => (
          <span className="text-muted-foreground">
            {formatTime(row.original.last_seen)}
          </span>
        ),
      },
      {
        accessorKey: 'culprit',
        header: 'Culprit',
        cell: ({ row }) => (
          <span className="max-w-[12rem] truncate font-mono text-muted-foreground">
            {row.original.culprit || '—'}
          </span>
        ),
      },
    ],
    []
  )

  const issues = issuesQuery.data ?? []
  const table = useReactTable({
    data: issues,
    columns,
    state: { sorting },
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
  })

  const projects = projectsQuery.data ?? []
  const facets = facetsQuery.data
  const projectItems = [
    { label: 'All projects', value: ALL },
    ...projects.map((p) => ({ label: p.name, value: String(p.id) })),
  ]
  const envItems = [
    { label: 'All environments', value: ALL },
    ...(facets?.environments ?? []).map((v) => ({ label: v, value: v })),
  ]
  const releaseItems = [
    { label: 'All releases', value: ALL },
    ...(facets?.releases ?? []).map((v) => ({ label: v, value: v })),
  ]
  const tagItems = [
    { label: 'All tags', value: ALL },
    ...(facets?.tags ?? []).map((v) => ({ label: v, value: v })),
  ]

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
      <div className="flex flex-col gap-1">
        <h1 className="font-heading text-2xl font-medium tracking-tight">
          Issues
        </h1>
        <p className="text-sm text-muted-foreground">
          Filter by project, environment, release, tag, and timeframe.
        </p>
      </div>

      <FieldGroup className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <Field>
          <FieldLabel>Project</FieldLabel>
          <Select
            items={projectItems}
            value={projectId || ALL}
            onValueChange={(v) =>
              patchParams({ project_id: v == null ? '' : String(v) })
            }
          >
            <SelectTrigger className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false} align="start">
              <SelectGroup>
                {projectItems.map((item) => (
                  <SelectItem key={item.value} value={item.value}>
                    {item.label}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </Field>

        <Field>
          <FieldLabel>Environment</FieldLabel>
          <Select
            items={envItems}
            value={environment || ALL}
            onValueChange={(v) =>
              patchParams({ environment: v == null ? '' : String(v) })
            }
          >
            <SelectTrigger className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false} align="start">
              <SelectGroup>
                {envItems.map((item) => (
                  <SelectItem key={item.value} value={item.value}>
                    {item.label}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </Field>

        <Field>
          <FieldLabel>Release</FieldLabel>
          <Select
            items={releaseItems}
            value={release || ALL}
            onValueChange={(v) =>
              patchParams({ release: v == null ? '' : String(v) })
            }
          >
            <SelectTrigger className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false} align="start">
              <SelectGroup>
                {releaseItems.map((item) => (
                  <SelectItem key={item.value} value={item.value}>
                    {item.label}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </Field>

        <Field>
          <FieldLabel>Tag</FieldLabel>
          <Select
            items={tagItems}
            value={tag || ALL}
            onValueChange={(v) =>
              patchParams({ tag: v == null ? '' : String(v) })
            }
          >
            <SelectTrigger className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false} align="start">
              <SelectGroup>
                {tagItems.map((item) => (
                  <SelectItem key={item.value} value={item.value}>
                    {item.label}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </Field>

        <Field>
          <FieldLabel htmlFor="q">Search</FieldLabel>
          <Input
            id="q"
            value={q}
            onChange={(e) => patchParams({ q: e.target.value })}
            placeholder="title, culprit, or message"
          />
        </Field>

        <DateTimePicker
          label="From"
          value={from}
          onChange={(v) => patchParams({ from: v })}
        />
        <DateTimePicker
          label="To"
          value={to}
          onChange={(v) => patchParams({ to: v })}
        />
      </FieldGroup>

      {issuesQuery.isLoading ? (
        <Skeleton className="h-48 w-full" />
      ) : issues.length === 0 ? (
        <Empty className="border border-dashed">
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <SearchIcon />
            </EmptyMedia>
            <EmptyTitle>No issues found</EmptyTitle>
            <EmptyDescription>
              No issues match these filters.
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      ) : (
        <div className="overflow-hidden rounded-xl ring-1 ring-foreground/10">
          <Table>
            <TableHeader>
              {table.getHeaderGroups().map((hg) => (
                <TableRow key={hg.id}>
                  {hg.headers.map((header) => (
                    <TableHead
                      key={header.id}
                      className={
                        header.column.getCanSort()
                          ? 'cursor-pointer select-none'
                          : undefined
                      }
                      onClick={header.column.getToggleSortingHandler()}
                    >
                      {flexRender(
                        header.column.columnDef.header,
                        header.getContext()
                      )}
                    </TableHead>
                  ))}
                </TableRow>
              ))}
            </TableHeader>
            <TableBody>
              {table.getRowModel().rows.map((row) => (
                <TableRow key={row.id}>
                  {row.getVisibleCells().map((cell) => (
                    <TableCell key={cell.id}>
                      {flexRender(
                        cell.column.columnDef.cell,
                        cell.getContext()
                      )}
                    </TableCell>
                  ))}
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </section>
  )
}
