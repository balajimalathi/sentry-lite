import { useEffect, useState, type FormEvent } from 'react'
import { Link, useParams } from 'react-router-dom'
import { AlertCircleIcon } from 'lucide-react'
import { api, formatTime, parsePayload, type Event, type Frame, type Issue } from '@/api'
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
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from '@/components/ui/empty'
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

  async function saveAssignee(e: FormEvent) {
    e.preventDefault()
    if (!issue) return
    const updated = await api.updateAssignee(issue.id, assigneeDraft.trim())
    setIssue(updated)
    setAssigneeDraft(updated.assignee ?? '')
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
    <section className="flex flex-col gap-6">
      <Breadcrumb>
        <BreadcrumbList>
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

      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="flex min-w-0 flex-1 flex-col gap-1">
          <h1 className="font-heading text-2xl font-medium tracking-tight">
            {issue.title}
          </h1>
          <p className="font-mono text-sm text-muted-foreground">
            {issue.culprit || 'No culprit'}
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Badge variant={statusVariant(issue.status)}>{issue.status}</Badge>
          {issue.regressed && <Badge variant="outline">regressed</Badge>}
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

      <form
        onSubmit={saveAssignee}
        className="flex flex-wrap items-end gap-2"
      >
        <div className="flex min-w-[12rem] flex-1 flex-col gap-1.5">
          <label htmlFor="assignee" className="text-sm text-muted-foreground">
            Owner
          </label>
          <Input
            id="assignee"
            value={assigneeDraft}
            onChange={(e) => setAssigneeDraft(e.target.value)}
            placeholder="email or handle"
          />
        </div>
        <Button type="submit" size="sm" variant="outline">
          Save owner
        </Button>
      </form>

      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6">
        {[
          { label: 'Events', value: String(issue.count) },
          { label: 'First seen', value: formatTime(issue.first_seen) },
          { label: 'Last seen', value: formatTime(issue.last_seen) },
          {
            label: 'Environments',
            value: (issue.environments ?? []).join(', ') || '—',
          },
          { label: 'First release', value: issue.first_release || '—' },
          { label: 'Last release', value: issue.last_release || '—' },
        ].map((item) => (
          <Card key={item.label} size="sm">
            <CardHeader>
              <CardDescription className="uppercase">
                {item.label}
              </CardDescription>
              <CardTitle className="text-sm">{item.value}</CardTitle>
            </CardHeader>
          </Card>
        ))}
      </div>

      {traceId && (
        <p className="text-sm">
          Trace:{' '}
          <Link
            to={`/traces/${traceId}`}
            className="font-mono underline-offset-4 hover:underline"
          >
            {traceId}
          </Link>
        </p>
      )}

      <div className="grid gap-4 lg:grid-cols-[2fr_1fr_1fr]">
        <Collapsible open={expanded} onOpenChange={setExpanded}>
          <Card>
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
              <CardContent>
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
                            ? 'font-medium text-foreground'
                            : 'text-muted-foreground'
                        }
                      >
                        <div className="font-mono text-sm">
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

        <Card>
          <CardHeader className="border-b">
            <CardTitle>Tags</CardTitle>
          </CardHeader>
          <CardContent>
            {Object.keys(tags).length === 0 ? (
              <p className="text-sm text-muted-foreground">No tags.</p>
            ) : (
              <dl className="flex flex-col">
                {Object.entries(tags).map(([k, v]) => (
                  <div
                    key={k}
                    className="grid grid-cols-[1fr_1.4fr] gap-2 border-b border-border py-1.5 last:border-b-0"
                  >
                    <dt className="text-sm text-muted-foreground">{k}</dt>
                    <dd className="text-sm break-words">{v}</dd>
                  </div>
                ))}
              </dl>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="border-b">
            <CardTitle>User</CardTitle>
          </CardHeader>
          <CardContent>
            <dl className="flex flex-col">
              <div className="grid grid-cols-[1fr_1.4fr] gap-2 border-b border-border py-1.5">
                <dt className="text-sm text-muted-foreground">id</dt>
                <dd className="text-sm break-words">{user.id || '—'}</dd>
              </div>
              <div className="grid grid-cols-[1fr_1.4fr] gap-2 py-1.5">
                <dt className="text-sm text-muted-foreground">email</dt>
                <dd className="text-sm break-words">{user.email || '—'}</dd>
              </div>
            </dl>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader className="border-b">
          <CardTitle>Breadcrumbs</CardTitle>
        </CardHeader>
        <CardContent className="px-0">
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
                    <TableCell className="max-w-md truncate text-sm">
                      {bc.message || '—'}
                    </TableCell>
                    <TableCell>{bc.level || '—'}</TableCell>
                    <TableCell className="pr-4 text-muted-foreground">
                      {bc.timestamp != null
                        ? typeof bc.timestamp === 'number'
                          ? formatTime(new Date(bc.timestamp * 1000).toISOString())
                          : formatTime(String(bc.timestamp))
                        : '—'}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="border-b">
          <CardTitle>Event timeline</CardTitle>
        </CardHeader>
        <CardContent className="px-0">
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
                      {formatTime(ev.timestamp)}
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
