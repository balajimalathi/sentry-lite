export type Project = {
  id: number
  slug: string
  name: string
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
  raw_path: string
  payload_json?: string
  tags?: Record<string, string>
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

async function get<T>(path: string): Promise<T> {
  const res = await fetch(path)
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
  return res.json()
}

async function post<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(path, {
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
  createProject: (body: { name: string; slug?: string }) =>
    post<CreatedProject>('/api/internal/projects', body),
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
    const res = await fetch(`/api/internal/issues/${id}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ status }),
    })
    if (!res.ok) throw new Error(`${res.status}`)
    return res.json() as Promise<Issue>
  },
  updateAssignee: async (id: number, assignee: string) => {
    const res = await fetch(`/api/internal/issues/${id}`, {
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
}

export function formatTime(iso: string | null | undefined) {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleString()
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
    }
  } catch {
    return null
  }
}
