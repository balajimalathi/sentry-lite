export type Project = {
  id: number
  slug: string
  name: string
  issue_count: number
  latest_activity_at: string | null
  created_at: string
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

async function get<T>(path: string): Promise<T> {
  const res = await fetch(path)
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
  return res.json()
}

export const api = {
  projects: () => get<Project[]>('/api/internal/projects'),
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
      breadcrumbs?: unknown[]
      exception_type?: string
      message?: string
    }
  } catch {
    return null
  }
}
