import { useMemo } from 'react'
import { Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { parseAsString, useQueryStates } from 'nuqs'
import {
  Area,
  AreaChart,
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  Pie,
  PieChart,
  XAxis,
  YAxis,
} from 'recharts'
import {
  api,
  formatRelativeTime,
  type CronMonitor,
  type Issue,
  type Project,
  type Release,
  type TransactionSummary,
} from '@/api'
import { PageHeader } from '@/components/page-header'
import { CreateProjectEmpty } from '@/components/page-empty'
import { Badge } from '@/components/ui/badge'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from '@/components/ui/chart'
import { Skeleton } from '@/components/ui/skeleton'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { toTitleCase } from '@/lib/format'
import { useProjectFilter } from '@/hooks/use-project-filter'

const RANGES = ['1h', '24h', '7d', '14d'] as const
type RangeKey = (typeof RANGES)[number]

const volumeConfig = {
  events: { label: 'Events', color: 'var(--chart-2)' },
} satisfies ChartConfig

const statusConfig = {
  open: { label: 'Open', color: 'var(--chart-1)' },
  resolved: { label: 'Resolved', color: 'var(--chart-2)' },
  ignored: { label: 'Ignored', color: 'var(--chart-3)' },
} satisfies ChartConfig

const topIssuesConfig = {
  count: { label: 'Events', color: 'var(--chart-2)' },
} satisfies ChartConfig

const STATUS_COLORS: Record<string, string> = {
  open: 'var(--chart-1)',
  resolved: 'var(--chart-2)',
  ignored: 'var(--chart-3)',
}

function rangeWindow(range: string): {
  from: string
  to: string
  interval: string
} {
  const to = new Date()
  const ms: Record<string, number> = {
    '1h': 60 * 60 * 1000,
    '24h': 24 * 60 * 60 * 1000,
    '7d': 7 * 24 * 60 * 60 * 1000,
    '14d': 14 * 24 * 60 * 60 * 1000,
  }
  const interval: Record<string, string> = {
    '1h': '5m',
    '24h': '1h',
    '7d': '6h',
    '14d': '1d',
  }
  const windowMs = ms[range] ?? ms['24h']
  const from = new Date(to.getTime() - windowMs)
  return {
    from: from.toISOString(),
    to: to.toISOString(),
    interval: interval[range] ?? '1h',
  }
}

function fmtMs(n: number) {
  if (!Number.isFinite(n)) return '—'
  if (n < 1) return `${n.toFixed(2)} ms`
  if (n < 1000) return `${n.toFixed(1)} ms`
  return `${(n / 1000).toFixed(2)} s`
}

function cronBadgeVariant(status: string) {
  switch (status) {
    case 'ok':
      return 'default' as const
    case 'late':
      return 'secondary' as const
    case 'missed':
      return 'destructive' as const
    default:
      return 'outline' as const
  }
}

function formatTick(iso: string, range: string) {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  if (range === '1h' || range === '24h') {
    return d.toLocaleTimeString(undefined, {
      hour: 'numeric',
      minute: '2-digit',
    })
  }
  return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
}

function formatCompactNumber(n: number): string {
  if (!Number.isFinite(n)) return '—'
  return new Intl.NumberFormat(undefined, {
    notation: 'compact',
    maximumFractionDigits: 1,
  }).format(n)
}

export default function DashboardPage() {
  const { projectId, setProjectId } = useProjectFilter()
  const [params, setParams] = useQueryStates({
    range: parseAsString.withDefault('24h'),
  })

  const range = (RANGES.includes(params.range as RangeKey)
    ? params.range
    : '24h') as RangeKey
  const window = useMemo(() => rangeWindow(range), [range])

  const projectsQuery = useQuery({
    queryKey: ['projects'],
    queryFn: () => api.projects(),
  })
  const projects = projectsQuery.data ?? []

  const statsQuery = useQuery({
    queryKey: ['stats', projectId || 'all', window.from, window.to, window.interval],
    queryFn: () =>
      api.stats({
        project_id: projectId || undefined,
        from: window.from,
        to: window.to,
        interval: window.interval,
      }),
  })

  const transactionsQuery = useQuery({
    queryKey: ['transactions', projectId],
    queryFn: () => api.transactions(projectId),
    enabled: !!projectId,
  })

  const cronsQuery = useQuery({
    queryKey: ['crons', projectId || 'all'],
    queryFn: () => api.crons(projectId || undefined),
  })

  const releasesQuery = useQuery({
    queryKey: ['releases', projectId],
    queryFn: () => api.releases(projectId),
    enabled: !!projectId,
  })

  const stats = statsQuery.data
  const loading =
    projectsQuery.isLoading ||
    statsQuery.isLoading ||
    cronsQuery.isLoading ||
    (projectId ? transactionsQuery.isLoading || releasesQuery.isLoading : false)

  const statusData = useMemo(() => {
    const by = stats?.by_status ?? {}
    return Object.entries(by).map(([name, value]) => ({
      name,
      value,
      fill: STATUS_COLORS[name] ?? 'var(--chart-4)',
    }))
  }, [stats?.by_status])

  const topIssuesData = useMemo(() => {
    const issues = stats?.top_issues ?? []
    return issues.map((issue) => ({
      id: issue.id,
      title:
        issue.title.length > 28 ? `${issue.title.slice(0, 28)}…` : issue.title,
      fullTitle: issue.title,
      count: issue.count,
    }))
  }, [stats?.top_issues])

  const slowestTx = useMemo(() => {
    const list = [...(transactionsQuery.data ?? [])]
    list.sort((a, b) => b.p95_ms - a.p95_ms)
    return list.slice(0, 5)
  }, [transactionsQuery.data])

  const worstP95 = slowestTx[0]?.p95_ms

  const crons = cronsQuery.data ?? []
  const releases = (releasesQuery.data ?? []).slice(0, 5)

  function setRange(next: string[]) {
    const value = next[0]
    if (value && RANGES.includes(value as RangeKey)) {
      void setParams({ range: value })
    }
  }

  if (!projectsQuery.isLoading && projects.length === 0) {
    return (
      <section className="flex flex-col gap-4">
        <PageHeader
          title="Dashboard"
          description="Overview of errors, performance, and monitors."
        />
        <CreateProjectEmpty />
      </section>
    )
  }

  return (
    <section className="flex flex-col gap-4">
      <PageHeader
        title="Dashboard"
        description="Overview of errors, performance, and monitors."
        actions={
          <ToggleGroup
            value={[range]}
            onValueChange={setRange}
            variant="outline"
            size="sm"
            spacing={0}
          >
            {RANGES.map((r) => (
              <ToggleGroupItem key={r} value={r} aria-label={r}>
                {r}
              </ToggleGroupItem>
            ))}
          </ToggleGroup>
        }
      />

      {loading ? (
        <DashboardSkeleton />
      ) : (
        <>
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">
            <KpiCard label="Unresolved" value={String(stats?.unresolved ?? 0)} />
            <KpiCard label="Events" value={String(stats?.events ?? 0)} />
            <KpiCard
              label="Regressions"
              value={String(stats?.regressions ?? 0)}
            />
            <KpiCard
              label="Crons unhealthy"
              value={String(stats?.crons_unhealthy ?? 0)}
            />
            {projectId ? (
              <KpiCard
                label="Worst p95"
                value={worstP95 != null ? fmtMs(worstP95) : '—'}
              />
            ) : (
              <KpiCard label="Projects" value={String(projects.length)} />
            )}
          </div>

          <div className="grid gap-4 lg:grid-cols-2">
            <Card>
              <CardHeader className="border-b">
                <CardTitle>Error volume</CardTitle>
                <CardDescription>Events over the selected range</CardDescription>
              </CardHeader>
              <CardContent className="pt-4">
                {(stats?.series?.length ?? 0) === 0 ? (
                  <MutedEmpty text="No events in this range." />
                ) : (
                  <ChartContainer config={volumeConfig} className="aspect-2/1">
                    <AreaChart data={stats?.series ?? []} margin={{ left: 8, right: 8 }}>
                      <CartesianGrid vertical={false} />
                      <XAxis
                        dataKey="t"
                        tickLine={false}
                        axisLine={false}
                        tickMargin={8}
                        minTickGap={24}
                        tickFormatter={(v) => formatTick(String(v), range)}
                      />
                      <YAxis
                        tickLine={false}
                        axisLine={false}
                        width={44}
                        tickMargin={4}
                        allowDecimals={false}
                        tickFormatter={(v) => formatCompactNumber(Number(v))}
                      />
                      <ChartTooltip
                        content={
                          <ChartTooltipContent
                            labelFormatter={(_, payload) => {
                              const t = payload?.[0]?.payload?.t
                              if (!t) return ''
                              const d = new Date(String(t))
                              if (Number.isNaN(d.getTime())) return String(t)
                              return d.toLocaleString()
                            }}
                          />
                        }
                      />
                      <Area
                        dataKey="events"
                        type="monotone"
                        fill="var(--color-events)"
                        fillOpacity={0.25}
                        stroke="var(--color-events)"
                        strokeWidth={2}
                      />
                    </AreaChart>
                  </ChartContainer>
                )}
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="border-b">
                <CardTitle>Issues by status</CardTitle>
                <CardDescription>Current issue status mix</CardDescription>
              </CardHeader>
              <CardContent className="pt-4">
                {statusData.length === 0 ? (
                  <MutedEmpty text="No issues yet." />
                ) : (
                  <ChartContainer config={statusConfig} className="aspect-2/1">
                    <PieChart>
                      <ChartTooltip
                        content={<ChartTooltipContent nameKey="name" hideLabel />}
                      />
                      <Pie
                        data={statusData}
                        dataKey="value"
                        nameKey="name"
                        innerRadius={48}
                        strokeWidth={2}
                      >
                        {statusData.map((entry) => (
                          <Cell key={entry.name} fill={entry.fill} />
                        ))}
                      </Pie>
                    </PieChart>
                  </ChartContainer>
                )}
                {statusData.length > 0 ? (
                  <div className="mt-2 flex flex-wrap justify-center gap-3">
                    {statusData.map((s) => (
                      <div
                        key={s.name}
                        className="flex items-center gap-1.5 text-xs text-muted-foreground"
                      >
                        <span
                          className="size-2 shrink-0 rounded-full"
                          style={{ background: s.fill }}
                        />
                        {toTitleCase(s.name)} ({s.value})
                      </div>
                    ))}
                  </div>
                ) : null}
              </CardContent>
            </Card>
          </div>

          <div className="grid gap-4 lg:grid-cols-2">
            <Card>
              <CardHeader className="border-b">
                <CardTitle>Top issues</CardTitle>
                <CardDescription>Open issues by event count</CardDescription>
              </CardHeader>
              <CardContent className="pt-4">
                {topIssuesData.length === 0 ? (
                  <MutedEmpty text="No open issues." />
                ) : (
                  <ChartContainer
                    config={topIssuesConfig}
                    className="aspect-auto h-64"
                  >
                    <BarChart
                      data={topIssuesData}
                      layout="vertical"
                      margin={{ left: 4, right: 8 }}
                    >
                      <CartesianGrid horizontal={false} />
                      <YAxis
                        dataKey="title"
                        type="category"
                        tickLine={false}
                        axisLine={false}
                        width={120}
                        tickMargin={4}
                      />
                      <XAxis
                        type="number"
                        tickLine={false}
                        axisLine={false}
                        allowDecimals={false}
                        tickFormatter={(v) => formatCompactNumber(Number(v))}
                      />
                      <ChartTooltip
                        content={
                          <ChartTooltipContent
                            labelFormatter={(_, payload) =>
                              String(payload?.[0]?.payload?.fullTitle ?? '')
                            }
                          />
                        }
                      />
                      <Bar
                        dataKey="count"
                        fill="var(--color-count)"
                        radius={4}
                      />
                    </BarChart>
                  </ChartContainer>
                )}
                {stats?.top_issues && stats.top_issues.length > 0 ? (
                  <ul className="mt-3 flex flex-col gap-1.5 border-t border-border pt-3">
                    {stats.top_issues.map((issue: Issue) => (
                      <li key={issue.id}>
                        <Link
                          to={`/issues/${issue.id}`}
                          className="flex items-center justify-between gap-2 text-sm hover:underline"
                        >
                          <span className="min-w-0 truncate">{issue.title}</span>
                          <span className="shrink-0 text-muted-foreground">
                            {issue.count}
                          </span>
                        </Link>
                      </li>
                    ))}
                  </ul>
                ) : null}
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="border-b">
                <CardTitle>Slowest transactions</CardTitle>
                <CardDescription>
                  {projectId
                    ? 'Highest p95 in the last 24h'
                    : 'Select a project to view latency'}
                </CardDescription>
              </CardHeader>
              <CardContent className="pt-4">
                {!projectId ? (
                  <MutedEmpty text="Select a project to view transactions." />
                ) : slowestTx.length === 0 ? (
                  <MutedEmpty text="No transactions yet." />
                ) : (
                  <ul className="flex flex-col gap-2">
                    {slowestTx.map((tx: TransactionSummary) => (
                      <li key={tx.name}>
                        <Link
                          to={`/performance/${encodeURIComponent(tx.name)}?project_id=${projectId}`}
                          className="flex items-center justify-between gap-3 rounded-md px-1 py-1.5 text-sm hover:bg-muted"
                        >
                          <span className="min-w-0 truncate font-medium">
                            {tx.name}
                          </span>
                          <span className="shrink-0 text-muted-foreground">
                            p95 {fmtMs(tx.p95_ms)} · {tx.count}
                          </span>
                        </Link>
                      </li>
                    ))}
                  </ul>
                )}
              </CardContent>
            </Card>
          </div>

          <div className="grid gap-4 lg:grid-cols-2">
            <Card>
              <CardHeader className="border-b">
                <CardTitle>Cron status</CardTitle>
                <CardDescription>Monitor health</CardDescription>
              </CardHeader>
              <CardContent className="pt-4">
                {crons.length === 0 ? (
                  <MutedEmpty text="No cron monitors." />
                ) : (
                  <ul className="flex flex-col gap-2">
                    {crons.slice(0, 8).map((c: CronMonitor) => (
                      <li
                        key={c.id}
                        className="flex items-center justify-between gap-3 text-sm"
                      >
                        <span className="min-w-0 truncate">{c.name}</span>
                        <div className="flex shrink-0 items-center gap-2">
                          <span className="text-xs text-muted-foreground">
                            {formatRelativeTime(c.last_checkin_at)}
                          </span>
                          <Badge variant={cronBadgeVariant(c.status)}>
                            {toTitleCase(c.status)}
                          </Badge>
                        </div>
                      </li>
                    ))}
                  </ul>
                )}
                {crons.length > 0 ? (
                  <Link
                    to={projectId ? `/crons?project_id=${projectId}` : '/crons'}
                    className="mt-3 inline-block text-xs text-muted-foreground hover:underline"
                  >
                    View all crons
                  </Link>
                ) : null}
              </CardContent>
            </Card>

            {projectId ? (
              <Card>
                <CardHeader className="border-b">
                  <CardTitle>Recent releases</CardTitle>
                  <CardDescription>Latest versions for this project</CardDescription>
                </CardHeader>
                <CardContent className="pt-4">
                  {releases.length === 0 ? (
                    <MutedEmpty text="No releases yet." />
                  ) : (
                    <ul className="flex flex-col gap-2">
                      {releases.map((r: Release) => (
                        <li
                          key={r.id}
                          className="flex items-center justify-between gap-3 text-sm"
                        >
                          <span className="min-w-0 truncate font-medium">
                            {r.version}
                          </span>
                          <span className="shrink-0 text-muted-foreground">
                            {r.issue_count} issues · {r.event_count} events
                          </span>
                        </li>
                      ))}
                    </ul>
                  )}
                  <Link
                    to={`/releases?project_id=${projectId}`}
                    className="mt-3 inline-block text-xs text-muted-foreground hover:underline"
                  >
                    View releases
                  </Link>
                </CardContent>
              </Card>
            ) : (
              <Card>
                <CardHeader className="border-b">
                  <CardTitle>Projects</CardTitle>
                  <CardDescription>Click a project to scope the dashboard</CardDescription>
                </CardHeader>
                <CardContent className="pt-4">
                  <ul className="flex flex-col gap-2">
                    {projects.map((p: Project) => (
                      <li key={p.id}>
                        <button
                          type="button"
                          className="flex w-full items-center justify-between gap-3 rounded-md px-1 py-1.5 text-left text-sm hover:bg-muted"
                          onClick={() => setProjectId(String(p.id))}
                        >
                          <span className="min-w-0 truncate font-medium">
                            {p.name}
                          </span>
                          <span className="shrink-0 text-muted-foreground">
                            {p.issue_count} issues ·{' '}
                            {formatRelativeTime(p.latest_activity_at)}
                          </span>
                        </button>
                      </li>
                    ))}
                  </ul>
                </CardContent>
              </Card>
            )}
          </div>
        </>
      )}
    </section>
  )
}

function KpiCard({ label, value }: { label: string; value: string }) {
  return (
    <Card size="sm" className="min-w-0">
      <CardHeader className="min-w-0">
        <CardDescription className="uppercase tracking-wide">
          {label}
        </CardDescription>
        <CardTitle className="truncate text-xl tabular-nums" title={value}>
          {value}
        </CardTitle>
      </CardHeader>
    </Card>
  )
}

function MutedEmpty({ text }: { text: string }) {
  return (
    <div className="flex min-h-32 items-center justify-center">
      <p className="text-sm text-muted-foreground">{text}</p>
    </div>
  )
}

function DashboardSkeleton() {
  return (
    <div className="flex flex-col gap-4">
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">
        {Array.from({ length: 5 }).map((_, i) => (
          <Skeleton key={i} className="h-20 rounded-xl" />
        ))}
      </div>
      <div className="grid gap-4 lg:grid-cols-2">
        <Skeleton className="h-64 rounded-xl" />
        <Skeleton className="h-64 rounded-xl" />
      </div>
      <div className="grid gap-4 lg:grid-cols-2">
        <Skeleton className="h-72 rounded-xl" />
        <Skeleton className="h-72 rounded-xl" />
      </div>
      <div className="grid gap-4 lg:grid-cols-2">
        <Skeleton className="h-48 rounded-xl" />
        <Skeleton className="h-48 rounded-xl" />
      </div>
    </div>
  )
}