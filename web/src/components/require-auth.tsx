import { useEffect, useState } from 'react'
import { Navigate, Outlet } from 'react-router-dom'
import { getAdminToken, probeAuthRequired } from '@/lib/auth'

type State =
  | { status: 'loading' }
  | { status: 'ok' }
  | { status: 'login' }
  | { status: 'error'; message: string }

/**
 * When ADMIN_TOKEN is set on the server, require a stored session token.
 * When unset (local/dev), allow through without login.
 */
export function RequireAuth() {
  const [state, setState] = useState<State>({ status: 'loading' })

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      try {
        if (getAdminToken()) {
          if (!cancelled) setState({ status: 'ok' })
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
