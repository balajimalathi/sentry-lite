import type { Span, TransactionSample } from '@/api'

function fmtMs(n: number) {
  if (!Number.isFinite(n)) return '—'
  if (n < 1) return `${n.toFixed(2)} ms`
  if (n < 1000) return `${n.toFixed(1)} ms`
  return `${(n / 1000).toFixed(2)} s`
}

type Row = {
  key: string
  label: string
  offset: number
  duration: number
  status: string
}

function rowsFromTransactions(transactions: TransactionSample[]): Row[] {
  const rows: Row[] = []
  for (const tx of transactions) {
    rows.push({
      key: tx.event_id,
      label: tx.name || tx.op || 'transaction',
      offset: 0,
      duration: tx.duration_ms,
      status: tx.status,
    })
    for (const sp of tx.spans ?? []) {
      rows.push({
        key: `${tx.event_id}-${sp.span_id}`,
        label: sp.description || sp.op || 'span',
        offset: sp.start_offset_ms ?? 0,
        duration: sp.duration_ms,
        status: sp.status,
      })
    }
  }
  return rows
}

export function TraceWaterfall({
  transactions,
}: {
  transactions: TransactionSample[]
}) {
  const rows = rowsFromTransactions(transactions)
  const total = Math.max(
    1,
    ...rows.map((r) => r.offset + Math.max(r.duration, 0))
  )

  if (rows.length === 0) {
    return (
      <p className="text-sm text-muted-foreground">
        No spans recorded for this trace.
      </p>
    )
  }

  return (
    <div className="flex flex-col gap-2">
      {rows.map((row) => {
        const left = (row.offset / total) * 100
        const width = Math.max((row.duration / total) * 100, 0.8)
        return (
          <div key={row.key} className="grid grid-cols-[minmax(0,12rem)_1fr_auto] items-center gap-2">
            <div className="truncate font-mono text-xs" title={row.label}>
              {row.label}
            </div>
            <div className="relative h-5 overflow-hidden rounded-sm bg-muted">
              <div
                className="absolute inset-y-0 rounded-sm bg-primary/70"
                style={{ left: `${left}%`, width: `${width}%` }}
                title={`${fmtMs(row.duration)} · ${row.status || 'ok'}`}
              />
            </div>
            <div className="text-right font-mono text-xs text-muted-foreground">
              {fmtMs(row.duration)}
            </div>
          </div>
        )
      })}
    </div>
  )
}

export type { Span }
