import { useEffect, useState } from 'react'
import { Navigate, Outlet, useNavigate } from 'react-router-dom'
import {
  clearAdminToken,
  getAdminToken,
  isSessionExpired,
  probeAuthRequired,
  touchSession,
  validateAdminToken,
} from '@/lib/auth'

type State =
  | { status: 'loading' }
  | { status: 'ok' }
  | { status: 'login' }
  | { status: 'error'; message: string }

const TOUCH_THROTTLE_MS = 30_000
const IDLE_CHECK_MS = 60_000

/**
 * When ADMIN_TOKEN is set on the server, require a valid stored session token.
 * When unset (local/dev), allow through without login.
 * Tracks browser activity and expires after 1h of inactivity.
 */
export function RequireAuth() {
  const [state, setState] = useState<State>({ status: 'loading' })
  const navigate = useNavigate()

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      try {
        if (isSessionExpired()) {
          clearAdminToken()
          if (!cancelled) setState({ status: 'login' })
          return
        }

        const token = getAdminToken()
        if (token) {
          const ok = await validateAdminToken(token)
          if (cancelled) return
          if (!ok) {
            clearAdminToken()
            setState({ status: 'login' })
            return
          }
          touchSession()
          setState({ status: 'ok' })
          return
        }

        const required = await probeAuthRequired()
        if (cancelled) return
        setState(required ? { status: 'login' } : { status: 'ok' })
      } catch (err) {
        if (cancelled) return
        setState({
          status: 'error',
          message: err instanceof Error ? err.message : 'Failed to check auth',
        })
      }
    })()
    return () => {
      cancelled = true
    }
  }, [])

  useEffect(() => {
    if (state.status !== 'ok') return
    if (!getAdminToken()) return

    let lastTouch = 0
    const onActivity = () => {
      const now = Date.now()
      if (now - lastTouch < TOUCH_THROTTLE_MS) return
      lastTouch = now
      touchSession()
    }

    const onVisibility = () => {
      if (document.visibilityState === 'visible') onActivity()
    }

    const checkIdle = () => {
      if (isSessionExpired() || !getAdminToken()) {
        clearAdminToken()
        navigate('/login', { replace: true })
      }
    }

    window.addEventListener('pointerdown', onActivity)
    window.addEventListener('keydown', onActivity)
    document.addEventListener('visibilitychange', onVisibility)
    const timer = window.setInterval(checkIdle, IDLE_CHECK_MS)

    return () => {
      window.removeEventListener('pointerdown', onActivity)
      window.removeEventListener('keydown', onActivity)
      document.removeEventListener('visibilitychange', onVisibility)
      window.clearInterval(timer)
    }
  }, [state.status, navigate])

  if (state.status === 'loading') {
    return (
      <div className="flex min-h-svh items-center justify-center text-sm text-muted-foreground">
        Loading…
      </div>
    )
  }
  if (state.status === 'login') {
    return <Navigate to="/login" replace />
  }
  if (state.status === 'error') {
    return (
      <div className="flex min-h-svh items-center justify-center px-6 text-sm text-destructive">
        {state.message}
      </div>
    )
  }
  return <Outlet />
}
