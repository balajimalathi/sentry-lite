import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { AlertCircleIcon } from 'lucide-react'
import { api, type Project, type TransactionSummary } from '@/api'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
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

function fmtMs(n: number) {
  if (!Number.isFinite(n)) return '—'
  if (n < 1) return `${n.toFixed(2)} ms`
  if (n < 1000) return `${n.toFixed(1)} ms`
  return `${(n / 1000).toFixed(2)} s`
}

export default function PerformancePage() {
  const [projects, setProjects] = useState<Project[]>([])
  const [projectId, setProjectId] = useState('1')
  const [rows, setRows] = useState<TransactionSummary[]>([])
  const [error, setError] = useState('')

  useEffect(() => {
    api.projects().then((p) => {
      setProjects(p)
      if (p.length && !p.find((x) => String(x.id) === projectId)) {
        setProjectId(String(p[0].id))
      }
    })
  }, [])

  useEffect(() => {
    if (!projectId) return
    api
      .transactions(projectId)
      .then(setRows)
      .catch((e) => setError(String(e)))
  }, [projectId])

  const projectItems = projects.map((p) => ({
    label: p.name,
    value: String(p.id),
  }))

  return (
    <section className="flex flex-col gap-4">
      <h1 className="font-heading text-2xl font-medium tracking-tight">
        Performance
      </h1>
      <p className="text-sm text-muted-foreground">
        Transaction latency over the last 24 hours (p95 / p99).
      </p>
      {error && (
        <Alert variant="destructive">
          <AlertCircleIcon />
          <AlertTitle>Error</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      <FieldGroup className="grid gap-3 sm:grid-cols-[1fr_auto]">
        <Field>
          <FieldLabel>Project</FieldLabel>
          <Select
            items={projectItems}
            value={projectId}
            onValueChange={(v) => setProjectId(v == null ? '1' : String(v))}
          >
            <SelectTrigger className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
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
      </FieldGroup>

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Transaction</TableHead>
            <TableHead>Count</TableHead>
            <TableHead>p95</TableHead>
            <TableHead>p99</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((r) => (
            <TableRow key={r.name}>
              <TableCell>
                <Link
                  to={`/performance/${encodeURIComponent(r.name)}?project_id=${projectId}`}
                  className="font-mono text-sm underline-offset-4 hover:underline"
                >
                  {r.name}
                </Link>
              </TableCell>
              <TableCell>{r.count}</TableCell>
              <TableCell>{fmtMs(r.p95_ms)}</TableCell>
              <TableCell>{fmtMs(r.p99_ms)}</TableCell>
            </TableRow>
          ))}
          {rows.length === 0 && (
            <TableRow>
              <TableCell colSpan={4} className="text-muted-foreground">
                No transactions yet. Enable traces in your Sentry SDK
                (tracesSampleRate &gt; 0).
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>
    </section>
  )
}
