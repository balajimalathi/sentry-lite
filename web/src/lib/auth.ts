const STORAGE_KEY = 'sentry-lite-admin-session'
const LEGACY_STORAGE_KEY = 'sentry-lite-admin-token'
export const IDLE_MS = 60 * 60 * 1000

type Session = {
  token: string
  lastActiveAt: number
}

function readRaw(): Session | null {
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY)
    if (raw) {
      const parsed = JSON.parse(raw) as Session
      if (
        typeof parsed?.token === 'string' &&
        typeof parsed?.lastActiveAt === 'number'
      ) {
        return parsed
      }
    }
    // Migrate legacy plain-string token once.
    const legacy = sessionStorage.getItem(LEGACY_STORAGE_KEY)
    if (legacy) {
      const session: Session = { token: legacy, lastActiveAt: Date.now() }
      sessionStorage.setItem(STORAGE_KEY, JSON.stringify(session))
      sessionStorage.removeItem(LEGACY_STORAGE_KEY)
      return session
    }
  } catch {
    /* ignore */
  }
  return null
}

function writeSession(session: Session) {
  sessionStorage.setItem(STORAGE_KEY, JSON.stringify(session))
}

function isExpired(session: Session): boolean {
  return Date.now() - session.lastActiveAt > IDLE_MS
}

export function getAdminToken(): string | null {
  const session = readRaw()
  if (!session) return null
  if (isExpired(session)) {
    clearAdminToken()
    return null
  }
  return session.token
}

export function setAdminToken(token: string) {
  writeSession({ token, lastActiveAt: Date.now() })
}

export function clearAdminToken() {
  try {
    sessionStorage.removeItem(STORAGE_KEY)
    sessionStorage.removeItem(LEGACY_STORAGE_KEY)
  } catch {
    /* ignore */
  }
}

/** Bump last-active if a non-expired session exists. */
export function touchSession() {
  const session = readRaw()
  if (!session) return
  if (isExpired(session)) {
    clearAdminToken()
    return
  }
  writeSession({ ...session, lastActiveAt: Date.now() })
}

/** True when a session exists but has gone idle past IDLE_MS. */
export function isSessionExpired(): boolean {
  const session = readRaw()
  if (!session) return false
  return isExpired(session)
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
