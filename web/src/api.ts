import { authHeaders, clearAdminToken } from '@/lib/auth'

export type Project = {
  id: number
  slug: string
  name: string
  allowed_origins: string[]
  issue_count: number
  latest_activity_at: string | null
  created_at: string
}

export type CreatedProject = {
  project: Project
  public_key: string
  secret_key: string
  dsn: string
}

export type Facets = {
  environments: string[]
  releases: string[]
  tags: string[]
}

export type Issue = {
  id: number
  project_id: number
  fingerprint: string
  title: string
  culprit: string
  status: string
  level: string
  count: number
  first_seen: string
  last_seen: string
  first_release: string | null
  last_release: string | null
  regressed: boolean
  assignee?: string | null
  environments?: string[]
  /** Distinct key:value pairs from event_tags (excludes environment / release). */
  tags?: string[]
}

export type Frame = {
  filename: string
  function: string
  abs_path: string
  module: string
  lineno: number
  colno: number
  in_app: boolean
}

export type Event = {
  id: number
  event_id: string
  issue_id: number
  project_id: number
  timestamp: string
  environment: string | null
  release: string | null
  platform: string | null
  message: string | null
  exception_type: string | null
  culprit: string | null
  user_id: string | null
  user_email: string | null
  trace_id?: string | null
  raw_path: string
  payload_json?: string
  tags?: Record<string, string>
}

export type TransactionSummary = {
  name: string
  count: number
  p95_ms: number
  p99_ms: number
  project_id: number
}

export type Span = {
  span_id: string
  parent_span_id: string
  op: string
  description: string
  duration_ms: number
  status: string
}

export type TransactionSample = {
  id: number
  event_id: string
  project_id: number
  name: string
  op: string
  trace_id: string
  span_id: string
  duration_ms: number
  status: string
  environment: string | null
  release: string | null
  timestamp: string
  spans?: Span[]
}

export type TraceDetail = {
  trace_id: string
  transactions: TransactionSample[]
  issues: Array<{ issue_id: number; title: string; event_id: string }>
}

export type CronMonitor = {
  id: number
  project_id: number
  slug: string
  name: string
  schedule_sec: number
  grace_sec: number
  environment: string | null
  status: string
  last_checkin_at: string | null
  next_expected_at: string | null
  token: string
  created_at: string
}

export type Release = {
  id: number
  project_id: number
  version: string
  ref?: string | null
  url?: string | null
  date_released?: string | null
  created_at: string
  issue_count: number
  event_count: number
}

export type AlertRule = {
  id: number
  project_id: number
  name: string
  trigger: string
  channel: string
  target: string
  threshold: number
  window_sec: number
  enabled: boolean
  created_at: string
}

function handleUnauthorized(res: Response) {
  if (res.status !== 401) return
  clearAdminToken()
  if (typeof window !== 'undefined' && !window.location.pathname.startsWith('/login')) {
    window.location.assign('/login')
  }
}

async function apiFetch(path: string, init?: RequestInit): Promise<Response> {
  const headers = new Headers(init?.headers)
  const auth = authHeaders() as Record<string, string>
  if (auth.Authorization) {
    headers.set('Authorization', auth.Authorization)
  }
  const res = await fetch(path, { ...init, headers })
  handleUnauthorized(res)
  return res
}

async function get<T>(path: string): Promise<T> {
  const res = await apiFetch(path)
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
  return res.json()
}

async function post<T>(path: string, body: unknown): Promise<T> {
  const res = await apiFetch(path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `${res.status} ${res.statusText}`)
  }
  return res.json()
}

export const api = {
  projects: () => get<Project[]>('/api/internal/projects'),
  createProject: (body: {
    name: string
    slug?: string
    allowed_origins?: string[]
  }) => post<CreatedProject>('/api/internal/projects', body),
  facets: (projectId?: string) =>
    get<Facets>(
      projectId
        ? `/api/internal/facets?project_id=${projectId}`
        : '/api/internal/facets'
    ),
  issues: (params: Record<string, string>) => {
    const q = new URLSearchParams()
    Object.entries(params).forEach(([k, v]) => {
      if (v) q.set(k, v)
    })
    return get<Issue[]>(`/api/internal/issues?${q}`)
  },
  issue: (id: string) =>
    get<{ issue: Issue; latest_event: Event | null }>(`/api/internal/issues/${id}`),
  events: (id: string) => get<Event[]>(`/api/internal/issues/${id}/events`),
  updateStatus: async (id: number, status: string) => {
    const res = await apiFetch(`/api/internal/issues/${id}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ status }),
    })
    if (!res.ok) throw new Error(`${res.status}`)
    return res.json() as Promise<Issue>
  },
  updateAssignee: async (id: number, assignee: string) => {
    const res = await apiFetch(`/api/internal/issues/${id}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ assignee }),
    })
    if (!res.ok) throw new Error(`${res.status}`)
    return res.json() as Promise<Issue>
  },
  releases: (projectId: string) =>
    get<Release[]>(`/api/internal/releases?project_id=${projectId}`),
  createRelease: (body: {
    project_id: number
    version: string
    ref?: string
    url?: string
  }) => post<Release>('/api/internal/releases', body),
  alerts: (projectId?: string) =>
    get<AlertRule[]>(
      projectId
        ? `/api/internal/alerts?project_id=${projectId}`
        : '/api/internal/alerts'
    ),
  createAlert: (body: Partial<AlertRule> & {
    project_id: number
    name: string
    trigger: string
    channel: string
    target: string
    secret?: string
  }) => post<AlertRule>('/api/internal/alerts', body),
  deleteAlert: async (id: number) => {
    const res = await apiFetch(`/api/internal/alerts/${id}`, { method: 'DELETE' })
    if (!res.ok) throw new Error(`${res.status}`)
  },
  transactions: (projectId: string) =>
    get<TransactionSummary[]>(
      `/api/internal/transactions?project_id=${projectId}`
    ),
  transaction: (name: string, projectId: string) =>
    get<{
      name: string
      summary: TransactionSummary | null
      samples: TransactionSample[]
    }>(
      `/api/internal/transaction?project_id=${projectId}&name=${encodeURIComponent(name)}`
    ),
  trace: (traceId: string) =>
    get<TraceDetail>(`/api/internal/traces/${encodeURIComponent(traceId)}`),
  crons: (projectId?: string) =>
    get<CronMonitor[]>(
      projectId
        ? `/api/internal/crons?project_id=${projectId}`
        : '/api/internal/crons'
    ),
  createCron: (body: {
    project_id: number
    name: string
    slug?: string
    schedule_sec: number
    grace_sec?: number
    environment?: string
  }) => post<CronMonitor>('/api/internal/crons', body),
  deleteCron: async (id: number) => {
    const res = await apiFetch(`/api/internal/crons/${id}`, { method: 'DELETE' })
    if (!res.ok) throw new Error(`${res.status}`)
  },
}

export function formatTime(iso: string | null | undefined) {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleString()
}

/** Compact relative age from an API ISO timestamp (Sentry-style: `5m`, `1h`, `35d`; future: `in 5m`). */
export function formatRelativeTime(iso: string | null | undefined) {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  const diffSec = Math.floor((Date.now() - d.getTime()) / 1000)
  const future = diffSec < 0
  const sec = Math.abs(diffSec)
  let label: string
  if (sec < 60) label = `${sec}s`
  else {
    const min = Math.floor(sec / 60)
    if (min < 60) label = `${min}m`
    else {
      const hr = Math.floor(min / 60)
      if (hr < 24) label = `${hr}h`
      else {
        const day = Math.floor(hr / 24)
        label = day < 365 ? `${day}d` : `${Math.floor(day / 365)}y`
      }
    }
  }
  return future ? `in ${label}` : label
}

export function parsePayload(ev: Event | null) {
  if (!ev?.payload_json) return null
  try {
    return JSON.parse(ev.payload_json) as {
      frames?: Frame[]
      tags?: Record<string, string>
      user?: { id?: string; email?: string; username?: string }
      breadcrumbs?: Array<{
        category?: string
        message?: string
        level?: string
        timestamp?: number | string
        type?: string
      }>
      exception_type?: string
      message?: string
      trace_id?: string
    }
  } catch {
    return null
  }
}
