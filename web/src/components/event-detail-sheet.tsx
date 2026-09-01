import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { api, formatRelativeTime, formatTime, parsePayload } from '@/api'
import { StackTrace } from '@/components/stack-trace'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'

export function EventDetailSheet({
  eventId,
  onOpenChange,
}: {
  eventId: string | null
  onOpenChange: (open: boolean) => void
}) {
  const query = useQuery({
    queryKey: ['event', eventId],
    queryFn: () => api.event(eventId!),
    enabled: Boolean(eventId),
  })
  const ev = query.data
  const payload = ev ? parsePayload(ev) : null
  const frames = payload?.frames ?? []
  const tags = ev?.tags ?? payload?.tags ?? {}
  const request = payload?.request

  return (
    <Sheet open={Boolean(eventId)} onOpenChange={onOpenChange}>
      <SheetContent side="right" className="w-full sm:max-w-lg">
        <SheetHeader>
          <SheetTitle>Event</SheetTitle>
          <SheetDescription className="font-mono break-all">
            {eventId}
          </SheetDescription>
        </SheetHeader>
        <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-4 pb-4">
          {query.isLoading ? (
            <Skeleton className="h-40 w-full" />
          ) : ev ? (
            <>
              <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-sm">
                <dt className="text-muted-foreground">Time</dt>
                <dd title={formatTime(ev.timestamp)}>
                  {formatRelativeTime(ev.timestamp)}
                </dd>
                <dt className="text-muted-foreground">Environment</dt>
                <dd>{ev.environment || '—'}</dd>
                <dt className="text-muted-foreground">Release</dt>
                <dd className="font-mono text-xs">{ev.release || '—'}</dd>
                {ev.trace_id ? (
                  <>
                    <dt className="text-muted-foreground">Trace</dt>
                    <dd>
                      <Link
                        to={`/traces/${ev.trace_id}`}
                        className="font-mono text-xs underline-offset-4 hover:underline"
                      >
                        {ev.trace_id}
                      </Link>
                    </dd>
                  </>
                ) : null}
              </dl>
              <div>
                <h3 className="mb-2 text-sm font-medium">Stack trace</h3>
                <StackTrace frames={frames} />
              </div>
              {Object.keys(tags).length > 0 ? (
                <div className="flex flex-wrap gap-1">
                  {Object.entries(tags).map(([k, v]) => (
                    <Badge key={k} variant="outline" className="font-mono font-normal">
                      {k}:{v}
                    </Badge>
                  ))}
                </div>
              ) : null}
              {request ? (
                <pre className="max-h-40 overflow-auto rounded-md bg-muted p-3 font-mono text-xs">
                  {JSON.stringify(request, null, 2)}
                </pre>
              ) : null}
              {ev.payload_json ? (
                <details>
                  <summary className="cursor-pointer text-sm font-medium">
                    Raw payload
                  </summary>
                  <pre className="mt-2 max-h-64 overflow-auto rounded-md bg-muted p-3 font-mono text-xs">
                    {(() => {
                      try {
                        return JSON.stringify(JSON.parse(ev.payload_json), null, 2)
                      } catch {
                        return ev.payload_json
                      }
                    })()}
                  </pre>
                </details>
              ) : null}
            </>
          ) : (
            <p className="text-sm text-muted-foreground">Event not found.</p>
          )}
        </div>
      </SheetContent>
    </Sheet>
  )
}
