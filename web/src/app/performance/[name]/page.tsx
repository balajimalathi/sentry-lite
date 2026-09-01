import { Link, useParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { AlertCircleIcon } from 'lucide-react'
import { api, formatRelativeTime, formatTime } from '@/api'
import { SelectProjectEmpty } from '@/components/page-empty'
import { TraceWaterfall } from '@/components/trace-waterfall'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from '@/components/ui/breadcrumb'
import { Skeleton } from '@/components/ui/skeleton'
import { useProjectFilter } from '@/hooks/use-project-filter'

function fmtMs(n: number) {
  if (!Number.isFinite(n)) return '—'
  if (n < 1) return `${n.toFixed(2)} ms`
  if (n < 1000) return `${n.toFixed(1)} ms`
  return `${(n / 1000).toFixed(2)} s`
}

export default function TransactionDetailPage() {
  const { name: rawName = '' } = useParams()
  const name = decodeURIComponent(rawName)
  const { projectId } = useProjectFilter()

  const query = useQuery({
    queryKey: ['transaction', name, projectId],
    queryFn: () => api.transaction(name, projectId),
    enabled: Boolean(name && projectId),
  })

  if (!projectId) {
    return (
      <section className="flex flex-col gap-4">
        <h1 className="font-heading font-mono text-2xl font-medium tracking-tight">
          {name || 'Transaction'}
        </h1>
        <SelectProjectEmpty />
      </section>
    )
  }

  if (query.error) {
    return (
      <Alert variant="destructive">
        <AlertCircleIcon />
        <AlertTitle>Failed to load transaction</AlertTitle>
        <AlertDescription>{String(query.error)}</AlertDescription>
      </Alert>
    )
  }

  if (query.isLoading) {
    return (
      <div className="flex flex-col gap-4">
        <Skeleton className="h-4 w-40" />
        <Skeleton className="h-8 w-2/3" />
        <Skeleton className="h-48 w-full" />
      </div>
    )
  }

  const summary = query.data?.summary ?? null
  const samples = query.data?.samples ?? []

  return (
    <section className="flex flex-col gap-6">
      <Breadcrumb>
        <BreadcrumbList>
          <BreadcrumbItem>
            <BreadcrumbLink render={<Link to={`/performance?project_id=${projectId}`} />}>
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
        <h1 className="font-heading font-mono text-2xl font-medium tracking-tight">
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
                <span title={formatTime(s.timestamp)}>
                  {formatRelativeTime(s.timestamp)}
                </span>
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
            {(s.spans?.length ?? 0) > 0 ? (
              <TraceWaterfall transactions={[s]} />
            ) : null}
          </div>
        ))}
        {samples.length === 0 && (
          <p className="text-sm text-muted-foreground">No samples yet.</p>
        )}
      </div>
    </section>
  )
}
