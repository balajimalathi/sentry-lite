import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { AlertCircleIcon } from 'lucide-react'
import { api, formatTime, type TraceDetail } from '@/api'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from '@/components/ui/breadcrumb'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

function fmtMs(n: number) {
  if (!Number.isFinite(n)) return '—'
  if (n < 1000) return `${n.toFixed(1)} ms`
  return `${(n / 1000).toFixed(2)} s`
}

export default function TracePage() {
  const { traceId = '' } = useParams()
  const [detail, setDetail] = useState<TraceDetail | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!traceId) return
    api
      .trace(traceId)
      .then(setDetail)
      .catch((e) => setError(String(e)))
  }, [traceId])

  if (error) {
    return (
      <Alert variant="destructive">
        <AlertCircleIcon />
        <AlertTitle>Failed to load trace</AlertTitle>
        <AlertDescription>{error}</AlertDescription>
      </Alert>
    )
  }

  return (
    <section className="flex flex-col gap-6">
      <Breadcrumb>
        <BreadcrumbList>
          <BreadcrumbItem>
            <BreadcrumbLink render={<Link to="/performance" />}>
              Performance
            </BreadcrumbLink>
          </BreadcrumbItem>
          <BreadcrumbSeparator />
          <BreadcrumbItem>
            <BreadcrumbPage>Trace</BreadcrumbPage>
          </BreadcrumbItem>
        </BreadcrumbList>
      </Breadcrumb>

      <div>
        <h1 className="font-heading text-2xl font-medium tracking-tight">
          Trace
        </h1>
        <p className="mt-1 font-mono text-sm text-muted-foreground">{traceId}</p>
      </div>

      {detail && detail.issues.length > 0 && (
        <div className="flex flex-col gap-2">
          <h2 className="text-lg font-medium">Related issues</h2>
          <ul className="list-inside list-disc text-sm">
            {detail.issues.map((i) => (
              <li key={i.issue_id}>
                <Link
                  to={`/issues/${i.issue_id}`}
                  className="underline-offset-4 hover:underline"
                >
                  #{i.issue_id} {i.title}
                </Link>
              </li>
            ))}
          </ul>
        </div>
      )}

      <div className="flex flex-col gap-4">
        <h2 className="text-lg font-medium">Transactions</h2>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Duration</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Time</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {(detail?.transactions ?? []).map((t) => (
              <TableRow key={t.event_id}>
                <TableCell>
                  <Link
                    to={`/performance/${encodeURIComponent(t.name)}?project_id=${t.project_id}`}
                    className="font-mono text-sm underline-offset-4 hover:underline"
                  >
                    {t.name}
                  </Link>
                </TableCell>
                <TableCell>{fmtMs(t.duration_ms)}</TableCell>
                <TableCell>{t.status || '—'}</TableCell>
                <TableCell className="text-muted-foreground">
                  {formatTime(t.timestamp)}
                </TableCell>
              </TableRow>
            ))}
            {(detail?.transactions?.length ?? 0) === 0 && (
              <TableRow>
                <TableCell colSpan={4} className="text-muted-foreground">
                  No transactions for this trace.
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>
    </section>
  )
}
