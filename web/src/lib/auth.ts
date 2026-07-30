const STORAGE_KEY = 'sentry-lite-admin-token'

export function getAdminToken(): string | null {
  try {
    return sessionStorage.getItem(STORAGE_KEY)
  } catch {
    return null
  }
}

export function setAdminToken(token: string) {
  sessionStorage.setItem(STORAGE_KEY, token)
}

export function clearAdminToken() {
  try {
    sessionStorage.removeItem(STORAGE_KEY)
  } catch {
    /* ignore */
  }
}

export function authHeaders(): HeadersInit {
  const token = getAdminToken()
  if (!token) return {}
  return { Authorization: `Bearer ${token}` }
}

/** Probe whether management API requires a token. */
export async function probeAuthRequired(): Promise<boolean> {
  const res = await fetch('/api/internal/meta')
  if (res.status === 401) return true
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
  return false
}

export async function validateAdminToken(token: string): Promise<boolean> {
  const res = await fetch('/api/internal/meta', {
    headers: { Authorization: `Bearer ${token}` },
  })
  return res.ok
}
