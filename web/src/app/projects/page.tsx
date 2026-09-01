import { useMemo, useState, type FormEvent } from 'react'
import { Link } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { ColumnDef } from '@tanstack/react-table'
import { AlertCircleIcon, MoreHorizontalIcon, PlusIcon } from 'lucide-react'
import { api, formatRelativeTime, formatTime, type Project } from '@/api'
import { DataTable } from '@/components/data-table/data-table'
import { DataTableColumnHeader } from '@/components/data-table/data-table-column-header'
import { DataTableSkeleton } from '@/components/data-table/data-table-skeleton'
import { ListDataTableFilters } from '@/components/list-data-table-filters'
import { PageEmpty } from '@/components/page-empty'
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
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Field, FieldDescription, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
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

function parseOrigins(text: string) {
  return text
    .split(/[\n,]+/)
    .map((o) => o.trim())
    .filter(Boolean)
}

export default function ProjectsPage() {
  const qc = useQueryClient()
  const [open, setOpen] = useState(false)
  const [name, setName] = useState('')
  const [slug, setSlug] = useState('')
  const [slugTouched, setSlugTouched] = useState(false)
  const [originsText, setOriginsText] = useState('')
  const [createdDsn, setCreatedDsn] = useState('')
  const [formError, setFormError] = useState('')
  const [editTarget, setEditTarget] = useState<Project | null>(null)
  const [editName, setEditName] = useState('')
  const [editOrigins, setEditOrigins] = useState('')
  const [deleteTarget, setDeleteTarget] = useState<Project | null>(null)
  const [rotateTarget, setRotateTarget] = useState<Project | null>(null)

  const projectsQuery = useQuery({
    queryKey: ['projects'],
    queryFn: () => api.projects(),
  })

  const createMutation = useMutation({
    mutationFn: () =>
      api.createProject({
        name: name.trim(),
        slug: slug.trim() || undefined,
        allowed_origins: parseOrigins(originsText),
      }),
    onSuccess: (res) => {
      setCreatedDsn(res.dsn)
      setName('')
      setSlug('')
      setSlugTouched(false)
      setOriginsText('')
      setFormError('')
      setOpen(false)
      void qc.invalidateQueries({ queryKey: ['projects'] })
    },
    onError: (e) => setFormError(String(e)),
  })

  const updateMutation = useMutation({
    mutationFn: () => {
      if (!editTarget) throw new Error('No project selected')
      return api.updateProject(editTarget.id, {
        name: editName.trim(),
        allowed_origins: parseOrigins(editOrigins),
      })
    },
    onSuccess: () => {
      setEditTarget(null)
      void qc.invalidateQueries({ queryKey: ['projects'] })
    },
    onError: (e) => setFormError(String(e)),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => api.deleteProject(id),
    onSuccess: () => {
      setDeleteTarget(null)
      void qc.invalidateQueries({ queryKey: ['projects'] })
    },
  })

  const rotateMutation = useMutation({
    mutationFn: (id: number) => api.rotateProjectKey(id),
    onSuccess: (res) => {
      setRotateTarget(null)
      setCreatedDsn(res.dsn)
      void qc.invalidateQueries({ queryKey: ['projects'] })
    },
  })

  function onCreate(e: FormEvent) {
    e.preventDefault()
    if (!name.trim()) return
    createMutation.mutate()
  }

  function onEdit(e: FormEvent) {
    e.preventDefault()
    if (!editName.trim()) return
    updateMutation.mutate()
  }

  function openEdit(project: Project) {
    setFormError('')
    setEditName(project.name)
    setEditOrigins((project.allowed_origins ?? []).join('\n'))
    setEditTarget(project)
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
            className="font-medium underline underline-offset-4"
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
          <span
            className="text-muted-foreground"
            title={formatTime(row.original.latest_activity_at)}
          >
            {formatRelativeTime(row.original.latest_activity_at)}
          </span>
        ),
      },
      {
        id: 'actions',
        enableSorting: false,
        enableHiding: false,
        header: () => <span className="sr-only">Actions</span>,
        cell: ({ row }) => (
          <ProjectRowActions
            project={row.original}
            onCopyDsn={async (id) => {
              const res = await api.project(id)
              setCreatedDsn(res.dsn)
              try {
                await navigator.clipboard.writeText(res.dsn)
              } catch {
                /* clipboard can fail in some browsers; DSN is still shown */
              }
            }}
            onEdit={openEdit}
            onRotate={setRotateTarget}
            onDelete={setDeleteTarget}
          />
        ),
      },
    ],
    []
  )

  const projects = projectsQuery.data ?? []
  const { table } = useDataTable({
    data: projects,
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

  const error = projectsQuery.error
    ? String(projectsQuery.error)
    : deleteMutation.error
      ? String(deleteMutation.error)
      : rotateMutation.error
        ? String(rotateMutation.error)
        : ''

  return (
    <section className="flex flex-col gap-4">
      <PageHeader
        title="Projects"
        description="Create projects, copy DSNs, and manage ingest keys."
        actions={
          <Dialog open={open} onOpenChange={setOpen}>
            <DialogTrigger render={<Button aria-label="New project" />}>
              <PlusIcon data-icon="inline-start" />
              <PageHeaderActionLabel>New project</PageHeaderActionLabel>
            </DialogTrigger>
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
                  <Field>
                    <FieldLabel htmlFor="project-origins">
                      Allowed origins
                    </FieldLabel>
                    <Textarea
                      id="project-origins"
                      value={originsText}
                      onChange={(e) => setOriginsText(e.target.value)}
                      placeholder={'http://localhost:3000\nhttps://app.example.com'}
                    />
                    <FieldDescription>
                      Browser SDK origins (one per line). Leave empty to allow
                      any Origin.
                    </FieldDescription>
                  </Field>
                </FieldGroup>
                {formError && open && (
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
        }
      />

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

      {error && (
        <Alert variant="destructive">
          <AlertCircleIcon />
          <AlertTitle>Error</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      <Dialog
        open={editTarget != null}
        onOpenChange={(next) => {
          if (!next) setEditTarget(null)
        }}
      >
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Edit project</DialogTitle>
            <DialogDescription>
              Update the display name and browser SDK origins.
            </DialogDescription>
          </DialogHeader>
          <form onSubmit={onEdit} className="flex flex-col gap-4">
            <FieldGroup className="grid gap-3">
              <Field>
                <FieldLabel htmlFor="edit-name">Name</FieldLabel>
                <Input
                  id="edit-name"
                  value={editName}
                  onChange={(e) => setEditName(e.target.value)}
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="edit-origins">Allowed origins</FieldLabel>
                <Textarea
                  id="edit-origins"
                  value={editOrigins}
                  onChange={(e) => setEditOrigins(e.target.value)}
                />
              </Field>
            </FieldGroup>
            {formError && editTarget && (
              <Alert variant="destructive">
                <AlertCircleIcon />
                <AlertTitle>Save failed</AlertTitle>
                <AlertDescription>{formError}</AlertDescription>
              </Alert>
            )}
            <DialogFooter>
              <Button type="submit" disabled={updateMutation.isPending}>
                Save
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={rotateTarget != null}
        onOpenChange={(next) => {
          if (!next) setRotateTarget(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Rotate ingest key?</AlertDialogTitle>
            <AlertDialogDescription>
              This invalidates the current DSN for{' '}
              <span className="font-medium text-foreground">
                {rotateTarget?.name}
              </span>
              . SDKs using the old key will stop ingesting until you update them.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={rotateMutation.isPending}>
              Cancel
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={rotateMutation.isPending || rotateTarget == null}
              onClick={() => {
                if (rotateTarget) rotateMutation.mutate(rotateTarget.id)
              }}
            >
              Rotate key
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={deleteTarget != null}
        onOpenChange={(next) => {
          if (!next) setDeleteTarget(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogMedia>
              <AlertCircleIcon />
            </AlertDialogMedia>
            <AlertDialogTitle>Delete project?</AlertDialogTitle>
            <AlertDialogDescription>
              This permanently deletes{' '}
              <span className="font-medium text-foreground">
                {deleteTarget?.name}
              </span>
              , its issues, events, and stored payloads. This cannot be undone.
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
              Delete project
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {projectsQuery.isLoading ? (
        <DataTableSkeleton columnCount={5} filterCount={2} rowCount={6} />
      ) : projects.length === 0 ? (
        <PageEmpty
          title="No projects yet"
          description="Create a project, copy its DSN, and point your Sentry SDK at this instance."
        />
      ) : (
        <DataTable table={table}>
          <ListDataTableFilters table={table} />
        </DataTable>
      )}
    </section>
  )
}

function ProjectRowActions({
  project,
  onCopyDsn,
  onEdit,
  onRotate,
  onDelete,
}: {
  project: Project
  onCopyDsn: (id: number) => Promise<void>
  onEdit: (project: Project) => void
  onRotate: (project: Project) => void
  onDelete: (project: Project) => void
}) {
  const [copying, setCopying] = useState(false)

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            aria-label={`Actions for ${project.name}`}
          />
        }
      >
        <MoreHorizontalIcon />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem
          disabled={copying}
          onClick={() => {
            setCopying(true)
            void onCopyDsn(project.id).finally(() => setCopying(false))
          }}
        >
          Copy DSN
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => onEdit(project)}>Edit</DropdownMenuItem>
        <DropdownMenuItem onClick={() => onRotate(project)}>
          Rotate key
        </DropdownMenuItem>
        <DropdownMenuItem variant="destructive" onClick={() => onDelete(project)}>
          Delete
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
