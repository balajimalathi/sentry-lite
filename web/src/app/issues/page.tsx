import { useEffect, useState, type FormEvent } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { AlertCircleIcon, SearchIcon } from 'lucide-react'
import { api, formatTime, type Issue, type Project } from '@/api'
import { statusVariant } from '@/lib/status'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
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
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

const ALL_PROJECTS = 'all'

export default function IssuesPage() {
  const [params, setParams] = useSearchParams()
  const [issues, setIssues] = useState<Issue[]>([])
  const [projects, setProjects] = useState<Project[]>([])
  const [error, setError] = useState('')

  const projectId = params.get('project_id') ?? ''
  const environment = params.get('environment') ?? ''
  const release = params.get('release') ?? ''
  const q = params.get('q') ?? ''
  const tag = params.get('tag') ?? ''
  const from = params.get('from') ?? ''
  const to = params.get('to') ?? ''

  const [draftProject, setDraftProject] = useState(
    projectId || ALL_PROJECTS
  )
  const [draftEnvironment, setDraftEnvironment] = useState(environment)
  const [draftRelease, setDraftRelease] = useState(release)
  const [draftQ, setDraftQ] = useState(q)
  const [draftTag, setDraftTag] = useState(tag)
  const [draftFrom, setDraftFrom] = useState(from)
  const [draftTo, setDraftTo] = useState(to)

  useEffect(() => {
    setDraftProject(projectId || ALL_PROJECTS)
    setDraftEnvironment(environment)
    setDraftRelease(release)
    setDraftQ(q)
    setDraftTag(tag)
    setDraftFrom(from)
    setDraftTo(to)
  }, [projectId, environment, release, q, tag, from, to])

  useEffect(() => {
    api.projects().then(setProjects).catch(() => {})
  }, [])

  useEffect(() => {
    api
      .issues({ project_id: projectId, environment, release, q, tag, from, to })
      .then(setIssues)
      .catch((e) => setError(String(e)))
  }, [projectId, environment, release, q, tag, from, to])

  function onFilter(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const next = new URLSearchParams()
    if (draftProject && draftProject !== ALL_PROJECTS) {
      next.set('project_id', draftProject)
    }
    if (draftEnvironment.trim()) next.set('environment', draftEnvironment.trim())
    if (draftRelease.trim()) next.set('release', draftRelease.trim())
    if (draftQ.trim()) next.set('q', draftQ.trim())
    if (draftTag.trim()) next.set('tag', draftTag.trim())
    if (draftFrom.trim()) next.set('from', draftFrom.trim())
    if (draftTo.trim()) next.set('to', draftTo.trim())
    setParams(next)
  }

  if (error) {
    return (
      <Alert variant="destructive">
        <AlertCircleIcon />
        <AlertTitle>Failed to load issues</AlertTitle>
        <AlertDescription>{error}</AlertDescription>
      </Alert>
    )
  }

  const projectItems = [
    { label: 'All', value: ALL_PROJECTS },
    ...projects.map((p) => ({ label: p.name, value: String(p.id) })),
  ]

  return (
    <section className="flex flex-col gap-4">
      <div className="flex flex-col gap-1">
        <h1 className="font-heading text-2xl font-medium tracking-tight">
          Issues
        </h1>
      </div>

      <form onSubmit={onFilter}>
        <FieldGroup className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          <Field>
            <FieldLabel>Project</FieldLabel>
            <Select
              items={projectItems}
              value={draftProject}
              onValueChange={(value) =>
                setDraftProject(value == null ? ALL_PROJECTS : String(value))
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
            <FieldLabel htmlFor="environment">Environment</FieldLabel>
            <Input
              id="environment"
              value={draftEnvironment}
              onChange={(e) => setDraftEnvironment(e.target.value)}
              placeholder="production"
            />
          </Field>
          <Field>
            <FieldLabel htmlFor="release">Release</FieldLabel>
            <Input
              id="release"
              value={draftRelease}
              onChange={(e) => setDraftRelease(e.target.value)}
              placeholder="1.0.0"
            />
          </Field>
          <Field>
            <FieldLabel htmlFor="tag">Tag (key:value)</FieldLabel>
            <Input
              id="tag"
              value={draftTag}
              onChange={(e) => setDraftTag(e.target.value)}
              placeholder="service:demo"
            />
          </Field>
          <Field>
            <FieldLabel htmlFor="q">Search</FieldLabel>
            <Input
              id="q"
              value={draftQ}
              onChange={(e) => setDraftQ(e.target.value)}
              placeholder="title, culprit, or message"
            />
          </Field>
          <Field>
            <FieldLabel htmlFor="from">From (RFC3339)</FieldLabel>
            <Input
              id="from"
              value={draftFrom}
              onChange={(e) => setDraftFrom(e.target.value)}
              placeholder="2026-01-01T00:00:00Z"
            />
          </Field>
          <Field>
            <FieldLabel htmlFor="to">To (RFC3339)</FieldLabel>
            <Input
              id="to"
              value={draftTo}
              onChange={(e) => setDraftTo(e.target.value)}
              placeholder="2026-12-31T23:59:59Z"
            />
          </Field>
          <Field className="justify-end">
            <FieldLabel className="sr-only">Apply</FieldLabel>
            <Button type="submit" className="w-full sm:w-auto">
              Filter
            </Button>
          </Field>
        </FieldGroup>
      </form>

      {issues.length === 0 ? (
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
              <TableRow>
                <TableHead>Title</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Owner</TableHead>
                <TableHead>Count</TableHead>
                <TableHead>First seen</TableHead>
                <TableHead>Last seen</TableHead>
                <TableHead>Culprit</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {issues.map((iss) => (
                <TableRow key={iss.id}>
                  <TableCell className="max-w-xs whitespace-normal">
                    <div className="flex flex-wrap items-center gap-2">
                      <Link
                        to={`/issues/${iss.id}`}
                        className="font-medium text-primary underline-offset-4 hover:underline"
                      >
                        {iss.title}
                      </Link>
                      {iss.regressed && (
                        <Badge variant="outline">regressed</Badge>
                      )}
                    </div>
                  </TableCell>
                  <TableCell>
                    <Badge variant={statusVariant(iss.status)}>
                      {iss.status}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {iss.assignee || '—'}
                  </TableCell>
                  <TableCell>{iss.count}</TableCell>
                  <TableCell className="text-muted-foreground">
                    {formatTime(iss.first_seen)}
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {formatTime(iss.last_seen)}
                  </TableCell>
                  <TableCell className="max-w-[12rem] truncate font-mono text-muted-foreground">
                    {iss.culprit || '—'}
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
