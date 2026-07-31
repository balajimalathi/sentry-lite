import { useEffect, useState, type FormEvent } from 'react'
import { Link, useParams } from 'react-router-dom'
import { AlertCircleIcon } from 'lucide-react'
import {
  api,
  formatRelativeTime,
  formatTime,
  parsePayload,
  type Event,
  type Frame,
  type Issue,
} from '@/api'
import { toTitleCase } from '@/lib/format'
import { statusVariant } from '@/lib/status'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from '@/components/ui/breadcrumb'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
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
  Empty,
  EmptyDescription,
  EmptyHeader,
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

export default function IssueDetailPage() {
  const { id = '' } = useParams()
  const [issue, setIssue] = useState<Issue | null>(null)
  const [latest, setLatest] = useState<Event | null>(null)
  const [events, setEvents] = useState<Event[]>([])
  const [expanded, setExpanded] = useState(true)
  const [error, setError] = useState('')
  const [assigneeDraft, setAssigneeDraft] = useState('')
  const [ownerOpen, setOwnerOpen] = useState(false)

  async function load() {
    const [detail, evs] = await Promise.all([api.issue(id), api.events(id)])
    setIssue(detail.issue)
    setAssigneeDraft(detail.issue.assignee ?? '')
    setLatest(detail.latest_event)
    setEvents(evs)
  }

  useEffect(() => {
    load().catch((e) => setError(String(e)))
  }, [id])

  async function setStatus(status: string) {
    if (!issue) return
    const updated = await api.updateStatus(issue.id, status)
    setIssue(updated)
  }

  function onOwnerOpenChange(next: boolean) {
    setOwnerOpen(next)
    if (next && issue) {
      setAssigneeDraft(issue.assignee ?? '')
    }
  }

  async function saveAssignee(e: FormEvent) {
    e.preventDefault()
    if (!issue) return
    const updated = await api.updateAssignee(issue.id, assigneeDraft.trim())
    setIssue(updated)
    setAssigneeDraft(updated.assignee ?? '')
    setOwnerOpen(false)
  }

  if (error) {
    return (
      <Alert variant="destructive">
        <AlertCircleIcon />
        <AlertTitle>Failed to load issue</AlertTitle>
        <AlertDescription>{error}</AlertDescription>
      </Alert>
    )
  }

  if (!issue) {
    return (
      <div className="flex flex-col gap-4">
        <Skeleton className="h-4 w-40" />
        <Skeleton className="h-8 w-2/3" />
        <div className="grid gap-3 sm:grid-cols-3">
          <Skeleton className="h-16" />
          <Skeleton className="h-16" />
          <Skeleton className="h-16" />
        </div>
        <Skeleton className="h-48 w-full" />
      </div>
    )
  }

  const payload = parsePayload(latest)
  const frames: Frame[] = payload?.frames ?? []
  const tags = latest?.tags ?? payload?.tags ?? {}
  const breadcrumbs = payload?.breadcrumbs ?? []
  const traceId = latest?.trace_id ?? payload?.trace_id ?? null
  const user = {
    id: latest?.user_id ?? payload?.user?.id,
    email: latest?.user_email ?? payload?.user?.email,
  }

  return (
    <section className="flex min-w-0 flex-col gap-6">
      <Breadcrumb className="min-w-0">
        <BreadcrumbList className="flex-wrap">
          <BreadcrumbItem>
            <BreadcrumbLink render={<Link to="/issues" />}>
              Issues
            </BreadcrumbLink>
          </BreadcrumbItem>
          <BreadcrumbSeparator />
          <BreadcrumbItem>
            <BreadcrumbPage>#{issue.id}</BreadcrumbPage>
          </BreadcrumbItem>
        </BreadcrumbList>
      </Breadcrumb>

      <div className="flex min-w-0 flex-col gap-3">
        <div className="flex min-w-0 items-start justify-between gap-3">
          <div className="flex min-w-0 flex-col gap-1">
            <div className="flex min-w-0 flex-wrap items-center gap-2">
              <h1 className="font-heading text-xl font-medium tracking-tight wrap-break-word sm:text-2xl">
                {issue.title}
              </h1>
              <Badge variant={statusVariant(issue.status)}>
                {toTitleCase(issue.status)}
              </Badge>
              {issue.regressed && <Badge variant="outline">Regressed</Badge>}
            </div>
            <p className="font-mono text-sm text-muted-foreground break-all">
              {issue.culprit || 'No culprit'}
            </p>
          </div>
          <Dialog open={ownerOpen} onOpenChange={onOwnerOpenChange}>
            <DialogTrigger
              render={
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  className="shrink-0"
                />
              }
            >
              {issue.assignee || 'Assign'}
            </DialogTrigger>
            <DialogContent className="sm:max-w-sm">
              <DialogHeader>
                <DialogTitle>Assign owner</DialogTitle>
                <DialogDescription>
                  Set an email or handle responsible for this issue.
                </DialogDescription>
              </DialogHeader>
              <form onSubmit={saveAssignee} className="flex flex-col gap-4">
                <FieldGroup>
                  <Field>
                    <FieldLabel htmlFor="assignee">Owner</FieldLabel>
                    <Input
                      id="assignee"
                      value={assigneeDraft}
                      onChange={(e) => setAssigneeDraft(e.target.value)}
                      placeholder="email or handle"
                      autoFocus
                    />
                  </Field>
                </FieldGroup>
                <DialogFooter>
                  <Button type="submit">Save owner</Button>
                </DialogFooter>
              </form>
            </DialogContent>
          </Dialog>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Button type="button" size="sm" onClick={() => setStatus('resolved')}>
            Resolve
          </Button>
          <Button
            type="button"
            size="sm"
            variant="secondary"
            onClick={() => setStatus('ignored')}
          >
            Ignore
          </Button>
          <Button
            type="button"
            size="sm"
            variant="outline"
            onClick={() => setStatus('open')}
          >
            Reopen
          </Button>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6">
        {(
          [
            { label: 'Events', value: String(issue.count) },
            {
              label: 'First seen',
              value: formatRelativeTime(issue.first_seen),
              title: formatTime(issue.first_seen),
            },
            {
              label: 'Last seen',
              value: formatRelativeTime(issue.last_seen),
              title: formatTime(issue.last_seen),
            },
            {
              label: 'Environments',
              value: (issue.environments ?? []).join(', ') || '—',
            },
            { label: 'First release', value: issue.first_release || '—' },
            { label: 'Last release', value: issue.last_release || '—' },
          ] as Array<{ label: string; value: string; title?: string }>
        ).map((item) => (
          <Card key={item.label} size="sm" className="min-w-0">
            <CardHeader className="min-w-0">
              <CardDescription className="uppercase">
                {item.label}
              </CardDescription>
              <CardTitle
                className="truncate text-sm"
                title={item.title ?? item.value}
              >
                {item.value}
              </CardTitle>
            </CardHeader>
          </Card>
        ))}
      </div>

      {traceId && (
        <p className="min-w-0 text-sm break-all">
          Trace:{' '}
          <Link
            to={`/traces/${traceId}`}
            className="font-mono underline-offset-4 hover:underline"
          >
            {traceId}
          </Link>
        </p>
      )}

      <div className="grid min-w-0 gap-4 lg:grid-cols-[2fr_1fr_1fr]">
        <Collapsible open={expanded} onOpenChange={setExpanded} className="min-w-0">
          <Card className="min-w-0">
            <CardHeader className="border-b">
              <CardTitle>Stack trace</CardTitle>
              <CardAction>
                <CollapsibleTrigger
                  render={<Button type="button" variant="outline" size="sm" />}
                >
                  {expanded ? 'Collapse' : 'Expand'}
                </CollapsibleTrigger>
              </CardAction>
            </CardHeader>
            <CollapsibleContent>
              <CardContent className="min-w-0 overflow-x-auto">
                {frames.length === 0 ? (
                  <p className="text-sm text-muted-foreground">
                    No stack frames on latest event.
                  </p>
                ) : (
                  <ol className="flex list-decimal flex-col gap-2 pl-4">
                    {[...frames].reverse().map((f, i) => (
                      <li
                        key={i}
                        className={
                          f.in_app
                            ? 'min-w-0 font-medium text-foreground'
                            : 'min-w-0 text-muted-foreground'
                        }
                      >
                        <div className="font-mono text-sm break-all">
                          {f.filename || f.abs_path || f.module || '?'}
                          {f.lineno ? `:${f.lineno}` : ''}
                          {f.function ? ` in ${f.function}` : ''}
                        </div>
                      </li>
                    ))}
                  </ol>
                )}
              </CardContent>
            </CollapsibleContent>
          </Card>
        </Collapsible>

        <Card className="min-w-0">
          <CardHeader className="border-b">
            <CardTitle>Tags</CardTitle>
          </CardHeader>
          <CardContent className="min-w-0">
            {Object.keys(tags).length === 0 ? (
              <p className="text-sm text-muted-foreground">No tags.</p>
            ) : (
              <dl className="flex flex-col">
                {Object.entries(tags).map(([k, v]) => (
                  <div
                    key={k}
                    className="grid grid-cols-1 gap-0.5 border-b border-border py-1.5 last:border-b-0 sm:grid-cols-[minmax(0,1fr)_minmax(0,1.4fr)] sm:gap-2"
                  >
                    <dt className="text-sm text-muted-foreground break-all">{k}</dt>
                    <dd className="text-sm break-all">{v}</dd>
                  </div>
                ))}
              </dl>
            )}
          </CardContent>
        </Card>

        <Card className="min-w-0">
          <CardHeader className="border-b">
            <CardTitle>User</CardTitle>
          </CardHeader>
          <CardContent className="min-w-0">
            <dl className="flex flex-col">
              <div className="grid grid-cols-1 gap-0.5 border-b border-border py-1.5 sm:grid-cols-[minmax(0,1fr)_minmax(0,1.4fr)] sm:gap-2">
                <dt className="text-sm text-muted-foreground">id</dt>
                <dd className="text-sm break-all">{user.id || '—'}</dd>
              </div>
              <div className="grid grid-cols-1 gap-0.5 py-1.5 sm:grid-cols-[minmax(0,1fr)_minmax(0,1.4fr)] sm:gap-2">
                <dt className="text-sm text-muted-foreground">email</dt>
                <dd className="text-sm break-all">{user.email || '—'}</dd>
              </div>
            </dl>
          </CardContent>
        </Card>
      </div>

      <Card className="min-w-0 overflow-hidden">
        <CardHeader className="border-b">
          <CardTitle>Breadcrumbs</CardTitle>
        </CardHeader>
        <CardContent className="min-w-0 px-0">
          {breadcrumbs.length === 0 ? (
            <p className="px-4 py-3 text-sm text-muted-foreground">
              No breadcrumbs on latest event.
            </p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="pl-4">Category</TableHead>
                  <TableHead>Message</TableHead>
                  <TableHead>Level</TableHead>
                  <TableHead className="pr-4">Time</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {breadcrumbs.map((bc, i) => (
                  <TableRow key={i}>
                    <TableCell className="pl-4 font-mono text-xs">
                      {bc.category || bc.type || '—'}
                    </TableCell>
                    <TableCell
                      className="max-w-48 truncate text-sm sm:max-w-md"
                      title={bc.message || undefined}
                    >
                      {bc.message || '—'}
                    </TableCell>
                    <TableCell>{bc.level || '—'}</TableCell>
                    <TableCell className="pr-4 text-muted-foreground">
                      {(() => {
                        if (bc.timestamp == null) return '—'
                        const iso =
                          typeof bc.timestamp === 'number'
                            ? new Date(bc.timestamp * 1000).toISOString()
                            : String(bc.timestamp)
                        return (
                          <span title={formatTime(iso)}>
                            {formatRelativeTime(iso)}
                          </span>
                        )
                      })()}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <Card className="min-w-0 overflow-hidden">
        <CardHeader className="border-b">
          <CardTitle>Event timeline</CardTitle>
        </CardHeader>
        <CardContent className="min-w-0 px-0">
          {events.length === 0 ? (
            <Empty className="py-8">
              <EmptyHeader>
                <EmptyTitle>No events</EmptyTitle>
                <EmptyDescription>
                  No events recorded for this issue yet.
                </EmptyDescription>
              </EmptyHeader>
            </Empty>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="pl-4">Event ID</TableHead>
                  <TableHead>Timestamp</TableHead>
                  <TableHead>Environment</TableHead>
                  <TableHead className="pr-4">Release</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {events.map((ev) => (
                  <TableRow key={ev.event_id}>
                    <TableCell className="pl-4 font-mono">
                      {ev.event_id.slice(0, 16)}
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      <span title={formatTime(ev.timestamp)}>
                        {formatRelativeTime(ev.timestamp)}
                      </span>
                    </TableCell>
                    <TableCell>{ev.environment || '—'}</TableCell>
                    <TableCell className="pr-4">{ev.release || '—'}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </section>
  )
}
