import { useMemo, useState, type FormEvent } from 'react'
import { Link } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { ColumnDef } from '@tanstack/react-table'
import { AlertCircleIcon } from 'lucide-react'
import { api, formatTime, type Project } from '@/api'
import { DataTable } from '@/components/data-table/data-table'
import { DataTableColumnHeader } from '@/components/data-table/data-table-column-header'
import { ListDataTableFilters } from '@/components/list-data-table-filters'
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
import { Skeleton } from '@/components/ui/skeleton'
import { useDataTable } from '@/hooks/use-data-table'

function slugify(name: string) {
  return name
    .toLowerCase()
    .trim()
    .replace(/\s+/g, '-')
    .replace(/[^a-z0-9-]/g, '')
    .replace(/^-+|-+$/g, '')
    .slice(0, 48)
}

export default function HomePage() {
  const qc = useQueryClient()
  const [open, setOpen] = useState(false)
  const [name, setName] = useState('')
  const [slug, setSlug] = useState('')
  const [slugTouched, setSlugTouched] = useState(false)
  const [createdDsn, setCreatedDsn] = useState('')
  const [formError, setFormError] = useState('')

  const projectsQuery = useQuery({
    queryKey: ['projects'],
    queryFn: () => api.projects(),
  })

  const createMutation = useMutation({
    mutationFn: () =>
      api.createProject({
        name: name.trim(),
        slug: slug.trim() || undefined,
      }),
    onSuccess: (res) => {
      setCreatedDsn(res.dsn)
      setName('')
      setSlug('')
      setSlugTouched(false)
      setFormError('')
      setOpen(false)
      void qc.invalidateQueries({ queryKey: ['projects'] })
    },
    onError: (e) => setFormError(String(e)),
  })

  function onCreate(e: FormEvent) {
    e.preventDefault()
    if (!name.trim()) return
    createMutation.mutate()
  }

  const columns = useMemo<ColumnDef<Project>[]>(
    () => [
      {
        id: 'name',
        accessorKey: 'name',
        enableColumnFilter: true,
        meta: {
          label: 'Name',
          placeholder: 'Search projects...',
          variant: 'text',
        },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} label="Name" />
        ),
        cell: ({ row }) => (
          <Link
            to={`/issues?project_id=${row.original.id}`}
            className="font-medium text-primary underline underline-offset-4"
          >
            {row.original.name}
          </Link>
        ),
      },
      {
        id: 'slug',
        accessorKey: 'slug',
        enableColumnFilter: true,
        meta: {
          label: 'Slug',
          placeholder: 'Search slugs...',
          variant: 'text',
        },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} label="Slug" />
        ),
        cell: ({ row }) => (
          <span className="font-mono text-muted-foreground">
            {row.original.slug}
          </span>
        ),
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
        id: 'latest_activity_at',
        accessorKey: 'latest_activity_at',
        meta: { label: 'Latest activity' },
        header: ({ column }) => (
          <DataTableColumnHeader column={column} label="Latest activity" />
        ),
        cell: ({ row }) => (
          <span className="text-muted-foreground">
            {formatTime(row.original.latest_activity_at)}
          </span>
        ),
      },
    ],
    []
  )

  const { table } = useDataTable({
    data: projectsQuery.data ?? [],
    columns,
    pageCount: -1,
    enableAdvancedFilter: false,
    manualFiltering: false,
    manualPagination: false,
    manualSorting: false,
    initialState: {
      sorting: [{ id: 'latest_activity_at', desc: true }],
      pagination: { pageIndex: 0, pageSize: 20 },
    },
  })

  const error = projectsQuery.error ? String(projectsQuery.error) : ''

  if (error) {
    return (
      <Alert variant="destructive">
        <AlertCircleIcon />
        <AlertTitle>Failed to load projects</AlertTitle>
        <AlertDescription>{error}</AlertDescription>
      </Alert>
    )
  }

  return (
    <section className="flex flex-col gap-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex flex-col gap-1">
          <h1 className="font-heading text-2xl font-medium tracking-tight">
            Projects
          </h1>
          <p className="text-sm text-muted-foreground">
            Create a project, copy its DSN into your SDK. Environment, release,
            and tags come from the SDK on each event — not from this UI.
          </p>
        </div>
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger render={<Button />}>New project</DialogTrigger>
          <DialogContent className="sm:max-w-md">
            <DialogHeader>
              <DialogTitle>New project</DialogTitle>
              <DialogDescription>
                A DSN is generated on create for your Sentry SDK.
              </DialogDescription>
            </DialogHeader>
            <form onSubmit={onCreate} className="flex flex-col gap-4">
              <FieldGroup className="grid gap-3">
                <Field>
                  <FieldLabel htmlFor="project-name">Name</FieldLabel>
                  <Input
                    id="project-name"
                    value={name}
                    onChange={(e) => {
                      const v = e.target.value
                      setName(v)
                      if (!slugTouched) setSlug(slugify(v))
                    }}
                    placeholder="Backend API"
                  />
                </Field>
                <Field>
                  <FieldLabel htmlFor="project-slug">Slug</FieldLabel>
                  <Input
                    id="project-slug"
                    value={slug}
                    onChange={(e) => {
                      setSlugTouched(true)
                      setSlug(e.target.value)
                    }}
                    placeholder="backend-api"
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
                <Button type="submit" disabled={createMutation.isPending}>
                  Create
                </Button>
              </DialogFooter>
            </form>
          </DialogContent>
        </Dialog>
      </div>

      {createdDsn && (
        <Alert>
          <AlertTitle>DSN ready</AlertTitle>
          <AlertDescription className="flex flex-col gap-2">
            <code className="break-all font-mono text-xs">{createdDsn}</code>
            <Button
              type="button"
              size="sm"
              variant="outline"
              className="w-fit"
              onClick={() => void navigator.clipboard.writeText(createdDsn)}
            >
              Copy DSN
            </Button>
          </AlertDescription>
        </Alert>
      )}

      {projectsQuery.isLoading ? (
        <Skeleton className="h-40 w-full" />
      ) : (
        <DataTable table={table}>
          <ListDataTableFilters table={table} />
        </DataTable>
      )}
    </section>
  )
}
