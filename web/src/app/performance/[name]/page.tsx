import { useEffect, useState } from 'react'
import { Link, useParams, useSearchParams } from 'react-router-dom'
import { AlertCircleIcon } from 'lucide-react'
import {
  api,
  formatTime,
  type TransactionSample,
  type TransactionSummary,
} from '@/api'
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
  if (n < 1) return `${n.toFixed(2)} ms`
  if (n < 1000) return `${n.toFixed(1)} ms`
  return `${(n / 1000).toFixed(2)} s`
}

export default function TransactionDetailPage() {
  const { name: rawName = '' } = useParams()
  const name = decodeURIComponent(rawName)
  const [params] = useSearchParams()
  const projectId = params.get('project_id') || '1'
  const [summary, setSummary] = useState<TransactionSummary | null>(null)
  const [samples, setSamples] = useState<TransactionSample[]>([])
  const [error, setError] = useState('')

  useEffect(() => {
    if (!name) return
    api
      .transaction(name, projectId)
      .then((d) => {
        setSummary(d.summary)
        setSamples(d.samples)
      })
      .catch((e) => setError(String(e)))
  }, [name, projectId])

  if (error) {
    return (
      <Alert variant="destructive">
        <AlertCircleIcon />
        <AlertTitle>Failed to load transaction</AlertTitle>
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
            <BreadcrumbPage>{name}</BreadcrumbPage>
          </BreadcrumbItem>
        </BreadcrumbList>
      </Breadcrumb>

      <div>
        <h1 className="font-heading text-2xl font-medium tracking-tight font-mono">
          {name}
        </h1>
        {summary && (
          <p className="mt-1 text-sm text-muted-foreground">
            {summary.count} samples · p95 {fmtMs(summary.p95_ms)} · p99{' '}
            {fmtMs(summary.p99_ms)} (24h)
          </p>
        )}
      </div>

      <div className="flex flex-col gap-4">
        <h2 className="text-lg font-medium">Recent traces</h2>
        {samples.map((s) => (
          <div key={s.event_id} className="flex flex-col gap-2 border-b border-border pb-4">
            <div className="flex flex-wrap items-baseline justify-between gap-2">
              <div className="font-mono text-sm">
                {fmtMs(s.duration_ms)}
                {s.status ? (
                  <span className="ml-2 text-muted-foreground">{s.status}</span>
                ) : null}
              </div>
              <div className="text-xs text-muted-foreground">
                {formatTime(s.timestamp)}
                {s.trace_id ? (
                  <>
                    {' · '}
                    <Link
                      to={`/traces/${s.trace_id}`}
                      className="underline-offset-4 hover:underline"
                    >
                      trace {s.trace_id.slice(0, 8)}…
                    </Link>
                  </>
                ) : null}
              </div>
            </div>
            {(s.spans?.length ?? 0) > 0 && (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Op</TableHead>
                    <TableHead>Description</TableHead>
                    <TableHead>Duration</TableHead>
                    <TableHead>Status</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {s.spans!.map((sp, i) => (
                    <TableRow key={`${sp.span_id}-${i}`}>
                      <TableCell className="font-mono text-xs">{sp.op || '—'}</TableCell>
                      <TableCell className="max-w-md truncate text-sm">
                        {sp.description || '—'}
                      </TableCell>
                      <TableCell>{fmtMs(sp.duration_ms)}</TableCell>
                      <TableCell className="text-muted-foreground">
                        {sp.status || '—'}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </div>
        ))}
        {samples.length === 0 && (
          <p className="text-sm text-muted-foreground">No samples yet.</p>
        )}
      </div>
    </section>
  )
}
