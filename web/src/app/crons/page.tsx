import { useMemo, useState, type FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { ColumnDef, ColumnFiltersState } from '@tanstack/react-table'
import { AlertCircleIcon, PlusIcon, Trash2Icon } from 'lucide-react'
import { parseAsArrayOf, parseAsString, useQueryStates } from 'nuqs'
import { api, formatRelativeTime, formatTime, type CronMonitor } from '@/api'
import { DataTable } from '@/components/data-table/data-table'
import { DataTableColumnHeader } from '@/components/data-table/data-table-column-header'
import { DataTableSkeleton } from '@/components/data-table/data-table-skeleton'
import { ListDataTableFilters } from '@/components/list-data-table-filters'
import { CreateProjectEmpty, PageEmpty } from '@/components/page-empty'
import {
  PageHeader,
  PageHeaderActionLabel,
} from '@/components/page-header'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogMedia,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
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
import { useDataTable } from '@/hooks/use-data-table'
import { EMPTY_PROJECTS } from '@/hooks/use-project-filter'
import { firstFilterValue } from '@/lib/row-filters'

const BASIC_FILTER_KEYS = [
  'project_id',
  'name',
  'status',
  'environment',
] as const

const EMPTY_MONITORS: CronMonitor[] = []

function statusVariant(status: string) {
  switch (status) {
    case 'ok':
      return 'default' as const
    case 'late':
      return 'secondary' as const
    case 'missed':
      return 'destructive' as const
    default:
      return 'outline' as const
  }
}

export default function CronsPage() {
  const qc = useQueryClient()
  const [basicFilterValues] = useQueryStates({
    project_id: parseAsArrayOf(parseAsString, ','),
    name: parseAsString,
    status: parseAsArrayOf(parseAsString, ','),
    environment: parseAsArrayOf(parseAsString, ','),
  })

  const [open, setOpen] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<CronMonitor | null>(null)
  const [formProjectId, setFormProjectId] = useState('')
  const [name, setName] = useState('')
  const [scheduleSec, setScheduleSec] = useState('60')
  const [graceSec, setGraceSec] = useState('30')
  const [formError, setFormError] = useState('')

  const basicColumnFilters = useMemo<ColumnFiltersState>(() => {
    const filters: ColumnFiltersState = []
    for (const key of BASIC_FILTER_KEYS) {
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
  const projectItems = projectOptions

  const selectedProjectId = firstFilterValue(
    basicColumnFilters.find((f) => f.id === 'project_id')?.value
  )

  const projectId = selectedProjectId

  const formProject =
    formProjectId || projectId || (projects[0] ? String(projects[0].id) : '')

  const cronsQuery = useQuery({
    queryKey: ['crons', projectId || 'all'],
    queryFn: () => api.crons(projectId || undefined),
  })

  const createMutation = useMutation({
    mutationFn: () =>
      api.createCron({
        project_id: Number(formProject),
        name: name.trim(),
        schedule_sec: Number(scheduleSec) || 60,
        grace_sec: Number(graceSec) || 30,
      }),
    onSuccess: () => {
      setName('')
      setFormError('')
      setOpen(false)
      void qc.invalidateQueries({ queryKey: ['crons'] })
    },
    onError: (err) => setFormError(String(err)),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => api.deleteCron(id),
    onSuccess: () => {
      setDeleteTarget(null)
      void qc.invalidateQueries({ queryKey: ['crons'] })
    },
  })

  const rawMonitors = cronsQuery.data ?? EMPTY_MONITORS

  const envOptions = useMemo(() => {
    const values = new Set<string>()
    for (const m of rawMonitors) {
      if (m.environment) values.add(m.environment)
    }
    return [...values].map((v) => ({ label: v, value: v }))
  }, [rawMonitors])

  const statusOptions = useMemo(
    () =>
      ['ok', 'late', 'missed'].map((v) => ({
        label: v,
        value: v,
      })),
    []
  )

  const publicBase =
    typeof window !== 'undefined'
      ? window.location.origin.replace(':5173', ':8080')
      : ''

  const columns = useMemo<ColumnDef<CronMonitor>[]>(
    () => [
      {
        id: 'name',
        accessorKey: 'name',
        enableColumnFilter: true,
        meta: {
          label: 'Name',
          placeholder: 'Search monitors...',
          variant: 'text',
        },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} label="Name" />
        ),
        cell: ({ row }) => (
          <div>
            <div className="font-medium">{row.original.name}</div>
            <div className="font-mono text-xs text-muted-foreground">
              {row.original.slug}
            </div>
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
        accessorFn: (r) => r.environment ?? '',
        enableColumnFilter: true,
        enableHiding: true,
        meta: {
          label: 'Environment',
          variant: 'select',
          options: envOptions,
        },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} label="Environment" />
        ),
        cell: ({ row }) => (
          <span className="text-muted-foreground">
            {row.original.environment || '—'}
          </span>
        ),
      },
      {
        id: 'last_checkin_at',
        accessorKey: 'last_checkin_at',
        meta: { label: 'Last check-in' },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} label="Last check-in" />
        ),
        cell: ({ row }) => (
          <span
            className="text-muted-foreground"
            title={formatTime(row.original.last_checkin_at)}
          >
            {formatRelativeTime(row.original.last_checkin_at)}
          </span>
        ),
      },
      {
        id: 'next_expected_at',
        accessorKey: 'next_expected_at',
        meta: { label: 'Next expected' },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} label="Next expected" />
        ),
        cell: ({ row }) => (
          <span
            className="text-muted-foreground"
            title={formatTime(row.original.next_expected_at)}
          >
            {formatRelativeTime(row.original.next_expected_at)}
          </span>
        ),
      },
      {
        id: 'schedule',
        accessorFn: (r) => r.schedule_sec,
        enableSorting: false,
        meta: { label: 'Schedule' },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} label="Schedule" />
        ),
        cell: ({ row }) => (
          <span className="text-sm">
            every {row.original.schedule_sec}s (+{row.original.grace_sec}s)
          </span>
        ),
      },
      {
        id: 'checkin_url',
        accessorFn: (r) => r.token,
        enableSorting: false,
        meta: { label: 'Check-in URL' },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} label="Check-in URL" />
        ),
        cell: ({ row }) => (
          <span className="max-w-xs truncate font-mono text-xs">
            {`POST ${publicBase}/api/cron/check-in/${row.original.token}`}
          </span>
        ),
      },
      {
        id: 'actions',
        enableSorting: false,
        enableHiding: false,
        header: () => <span className="sr-only">Actions</span>,
        cell: ({ row }) => (
          <Button
            type="button"
            size="sm"
            variant="outline"
            aria-label={`Delete ${row.original.name}`}
            onClick={() => setDeleteTarget(row.original)}
          >
            <Trash2Icon data-icon="inline-start" />
            Delete
          </Button>
        ),
      },
    ],
    [
      statusOptions,
      projectOptions,
      envOptions,
      publicBase,
      projects,
    ]
  )

  const { table } = useDataTable({
    data: rawMonitors,
    columns,
    pageCount: -1,
    enableAdvancedFilter: false,
    manualFiltering: false,
    manualPagination: false,
    manualSorting: false,
    initialState: {
      sorting: [{ id: 'name', desc: false }],
      pagination: { pageIndex: 0, pageSize: 20 },
      columnVisibility: { environment: false },
    },
  })

  const error = cronsQuery.error
    ? String(cronsQuery.error)
    : deleteMutation.error
      ? String(deleteMutation.error)
      : ''

  function onCreate(e: FormEvent) {
    e.preventDefault()
    if (!name.trim() || !formProject) return
    createMutation.mutate()
  }

  return (
    <section className="flex flex-col gap-4">
      <PageHeader
        title="Crons"
        description="Heartbeat monitors for scheduled jobs."
        actions={
          <Dialog
            open={open}
            onOpenChange={(next) => {
              setOpen(next)
              if (next && !formProjectId && projectId) {
                setFormProjectId(projectId)
              }
            }}
          >
            <DialogTrigger
              render={
                <Button disabled={!formProject} aria-label="Create monitor" />
              }
            >
              <PlusIcon data-icon="inline-start" />
              <PageHeaderActionLabel>Create monitor</PageHeaderActionLabel>
            </DialogTrigger>
            <DialogContent className="sm:max-w-md">
              <DialogHeader>
                <DialogTitle>Create monitor</DialogTitle>
                <DialogDescription>
                  Expected frequency and grace period for project check-ins.
                </DialogDescription>
              </DialogHeader>
              <form onSubmit={onCreate} className="flex flex-col gap-4">
                <FieldGroup className="grid gap-3">
                  <Field>
                    <FieldLabel>Project</FieldLabel>
                    <Select
                      items={projectItems}
                      value={formProject || undefined}
                      onValueChange={(v) =>
                        setFormProjectId(v == null ? '' : String(v))
                      }
                    >
                      <SelectTrigger className="w-full">
                        <SelectValue placeholder="Select project" />
                      </SelectTrigger>
                      <SelectContent>
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
                    <FieldLabel htmlFor="cron-name">Name</FieldLabel>
                    <Input
                      id="cron-name"
                      value={name}
                      onChange={(e) => setName(e.target.value)}
                      placeholder="nightly-backup"
                    />
                  </Field>
                  <Field>
                    <FieldLabel htmlFor="schedule">
                      Expected every (sec)
                    </FieldLabel>
                    <Input
                      id="schedule"
                      value={scheduleSec}
                      onChange={(e) => setScheduleSec(e.target.value)}
                    />
                  </Field>
                  <Field>
                    <FieldLabel htmlFor="grace">Grace (sec)</FieldLabel>
                    <Input
                      id="grace"
                      value={graceSec}
                      onChange={(e) => setGraceSec(e.target.value)}
                    />
                  </Field>
                </FieldGroup>
                {formError && (
                  <Alert variant="destructive">
                    <AlertCircleIcon />
                    <AlertTitle>Create failed</AlertTitle>
                    <AlertDescription>{formError}</AlertDescription>
                  </Alert>
                )}
                <DialogFooter>
                  <Button
                    type="submit"
                    disabled={!formProject || createMutation.isPending}
                  >
                    Create
                  </Button>
                </DialogFooter>
              </form>
            </DialogContent>
          </Dialog>
        }
      />

      <AlertDialog
        open={deleteTarget != null}
        onOpenChange={(next) => {
          if (!next) setDeleteTarget(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogMedia>
              <Trash2Icon />
            </AlertDialogMedia>
            <AlertDialogTitle>Delete cron monitor?</AlertDialogTitle>
            <AlertDialogDescription>
              This permanently deletes{' '}
              <span className="font-medium text-foreground">
                {deleteTarget?.name}
              </span>{' '}
              and its check-in history. The check-in URL will stop working.
              This cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleteMutation.isPending}>
              Cancel
            </AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              disabled={deleteMutation.isPending || deleteTarget == null}
              onClick={() => {
                if (deleteTarget) deleteMutation.mutate(deleteTarget.id)
              }}
            >
              Delete monitor
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {error && (
        <Alert variant="destructive">
          <AlertCircleIcon />
          <AlertTitle>Error</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      {cronsQuery.isLoading || projectsQuery.isLoading ? (
        <DataTableSkeleton columnCount={7} filterCount={3} rowCount={6} />
      ) : projects.length === 0 ? (
        <CreateProjectEmpty />
      ) : rawMonitors.length === 0 ? (
        <PageEmpty
          title="No cron monitors"
          description="Create a heartbeat monitor, then POST to the check-in URL from your job."
        />
      ) : (
        <DataTable table={table}>
          <ListDataTableFilters table={table} />
        </DataTable>
      )}
    </section>
  )
}
