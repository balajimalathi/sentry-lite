import { useMemo, useState, type FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { ColumnDef, ColumnFiltersState } from '@tanstack/react-table'
import { AlertCircleIcon, PlusIcon } from 'lucide-react'
import { parseAsArrayOf, parseAsString, useQueryStates } from 'nuqs'
import { api, formatRelativeTime, formatTime, type Release } from '@/api'
import { DataTable } from '@/components/data-table/data-table'
import { DataTableColumnHeader } from '@/components/data-table/data-table-column-header'
import { DataTableSkeleton } from '@/components/data-table/data-table-skeleton'
import { ListDataTableFilters } from '@/components/list-data-table-filters'
import {
  CreateProjectEmpty,
  PageEmpty,
  SelectProjectEmpty,
} from '@/components/page-empty'
import {
  PageHeader,
  PageHeaderActionLabel,
} from '@/components/page-header'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
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

const BASIC_FILTER_KEYS = ['project_id', 'version'] as const

export default function ReleasesPage() {
  const qc = useQueryClient()
  const [basicFilterValues] = useQueryStates({
    project_id: parseAsArrayOf(parseAsString, ','),
    version: parseAsString,
  })

  const [open, setOpen] = useState(false)
  const [formProjectId, setFormProjectId] = useState('')
  const [version, setVersion] = useState('')
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

  const releasesQuery = useQuery({
    queryKey: ['releases', projectId],
    queryFn: () => api.releases(projectId),
    enabled: !!projectId,
  })

  const createMutation = useMutation({
    mutationFn: () =>
      api.createRelease({
        project_id: Number(formProject),
        version: version.trim(),
      }),
    onSuccess: () => {
      setVersion('')
      setFormError('')
      setOpen(false)
      void qc.invalidateQueries({ queryKey: ['releases'] })
    },
    onError: (err) => setFormError(String(err)),
  })

  const columns = useMemo<ColumnDef<Release>[]>(
    () => [
      {
        id: 'version',
        accessorKey: 'version',
        enableColumnFilter: true,
        meta: {
          label: 'Version',
          placeholder: 'Search versions...',
          variant: 'text',
        },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} label="Version" />
        ),
        cell: ({ row }) => (
          <span className="font-mono">{row.original.version}</span>
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
        id: 'issue_count',
        accessorKey: 'issue_count',
        meta: { label: 'Issues' },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} label="Issues" />
        ),
      },
      {
        id: 'event_count',
        accessorKey: 'event_count',
        meta: { label: 'Events' },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} label="Events" />
        ),
      },
      {
        id: 'created_at',
        accessorKey: 'created_at',
        meta: { label: 'Created' },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} label="Created" />
        ),
        cell: ({ row }) => (
          <span
            className="text-muted-foreground"
            title={formatTime(row.original.created_at)}
          >
            {formatRelativeTime(row.original.created_at)}
          </span>
        ),
      },
    ],
    [projectOptions, projects]
  )

  const rawReleases = releasesQuery.data ?? []
  const { table } = useDataTable({
    data: rawReleases,
    columns,
    pageCount: -1,
    enableAdvancedFilter: false,
    manualFiltering: false,
    manualPagination: false,
    manualSorting: false,
    initialState: {
      sorting: [{ id: 'created_at', desc: true }],
      pagination: { pageIndex: 0, pageSize: 20 },
    },
  })

  const error = releasesQuery.error ? String(releasesQuery.error) : ''

  function onCreate(e: FormEvent) {
    e.preventDefault()
    if (!version.trim() || !formProject) return
    createMutation.mutate()
  }

  return (
    <section className="flex flex-col gap-4">
      <PageHeader
        title="Releases"
        description="Versions and release health."
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
              render={<Button aria-label="Register release" />}
            >
              <PlusIcon data-icon="inline-start" />
              <PageHeaderActionLabel>Register release</PageHeaderActionLabel>
            </DialogTrigger>
            <DialogContent className="sm:max-w-md">
              <DialogHeader>
                <DialogTitle>Register release</DialogTitle>
                <DialogDescription>
                  Link a version to a project for release health.
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
                    <FieldLabel htmlFor="version">Version</FieldLabel>
                    <Input
                      id="version"
                      value={version}
                      onChange={(e) => setVersion(e.target.value)}
                      placeholder="1.2.3"
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

      {error && (
        <Alert variant="destructive">
          <AlertCircleIcon />
          <AlertTitle>Error</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      {projectsQuery.isLoading || (!!projectId && releasesQuery.isLoading) ? (
        <DataTableSkeleton columnCount={5} filterCount={2} rowCount={6} />
      ) : projects.length === 0 ? (
        <CreateProjectEmpty />
      ) : !projectId ? (
        <SelectProjectEmpty />
      ) : rawReleases.length === 0 ? (
        <PageEmpty
          title="No releases yet"
          description="Register a version to track issue and event health for this project."
        />
      ) : (
        <DataTable table={table}>
          <ListDataTableFilters table={table} />
        </DataTable>
      )}
    </section>
  )
}
