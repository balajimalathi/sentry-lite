import { useState, type FormEvent } from 'react'
import { Link } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AlertCircleIcon, FolderIcon } from 'lucide-react'
import { api, formatTime } from '@/api'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

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
      void qc.invalidateQueries({ queryKey: ['projects'] })
    },
    onError: (e) => setFormError(String(e)),
  })

  function onCreate(e: FormEvent) {
    e.preventDefault()
    if (!name.trim()) return
    createMutation.mutate()
  }

  const projects = projectsQuery.data ?? []
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
      <div className="flex flex-col gap-1">
        <h1 className="font-heading text-2xl font-medium tracking-tight">
          Projects
        </h1>
        <p className="text-sm text-muted-foreground">
          Create a project, copy its DSN into your SDK. Environment, release, and
          tags come from the SDK on each event — not from this UI.
        </p>
      </div>

      <form onSubmit={onCreate}>
        <FieldGroup className="grid gap-3 sm:grid-cols-[1fr_1fr_auto]">
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
          <Field className="justify-end">
            <FieldLabel className="sr-only">Create</FieldLabel>
            <Button type="submit" disabled={createMutation.isPending}>
              New project
            </Button>
          </Field>
        </FieldGroup>
      </form>

      {formError && (
        <Alert variant="destructive">
          <AlertCircleIcon />
          <AlertTitle>Create failed</AlertTitle>
          <AlertDescription>{formError}</AlertDescription>
        </Alert>
      )}

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
      ) : projects.length === 0 ? (
        <Empty className="border border-dashed">
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <FolderIcon />
            </EmptyMedia>
            <EmptyTitle>No projects yet</EmptyTitle>
            <EmptyDescription>
              Create a project above, then point your SDK at the DSN.
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      ) : (
        <div className="overflow-hidden rounded-xl ring-1 ring-foreground/10">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Slug</TableHead>
                <TableHead>Issues</TableHead>
                <TableHead>Latest activity</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {projects.map((p) => (
                <TableRow key={p.id}>
                  <TableCell>
                    <Link
                      to={`/issues?project_id=${p.id}`}
                      className="font-medium text-primary underline-offset-4 hover:underline"
                    >
                      {p.name}
                    </Link>
                  </TableCell>
                  <TableCell className="font-mono text-muted-foreground">
                    {p.slug}
                  </TableCell>
                  <TableCell>{p.issue_count}</TableCell>
                  <TableCell className="text-muted-foreground">
                    {formatTime(p.latest_activity_at)}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </section>
  )
}
