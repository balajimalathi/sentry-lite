import { useEffect, useState, type FormEvent } from 'react'
import { Navigate, useNavigate } from 'react-router-dom'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  clearAdminToken,
  getAdminToken,
  setAdminToken,
  validateAdminToken,
} from '@/lib/auth'

export default function LoginPage() {
  const navigate = useNavigate()
  const [ready, setReady] = useState(false)
  const [authed, setAuthed] = useState(false)
  const [token, setToken] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [pending, setPending] = useState(false)

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      const existing = getAdminToken()
      if (!existing) {
        if (!cancelled) setReady(true)
        return
      }
      const ok = await validateAdminToken(existing)
      if (cancelled) return
      if (!ok) {
        clearAdminToken()
        setReady(true)
        return
      }
      setAuthed(true)
      setReady(true)
    })()
    return () => {
      cancelled = true
    }
  }, [])

  if (!ready) {
    return (
      <div className="flex min-h-svh items-center justify-center text-sm text-muted-foreground">
        Loading…
      </div>
    )
  }

  if (authed) {
    return <Navigate to="/" replace />
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    setPending(true)
    try {
      const trimmed = token.trim()
      if (!trimmed) {
        setError('Enter the gateway token')
        return
      }
      const ok = await validateAdminToken(trimmed)
      if (!ok) {
        setError('Invalid token')
        return
      }
      setAdminToken(trimmed)
      navigate('/', { replace: true })
    } catch {
      setError('Could not reach the API')
    } finally {
      setPending(false)
    }
  }

  return (
    <div className="mx-auto flex min-h-svh w-full max-w-sm flex-col justify-center gap-6 px-6">
      <div className="space-y-1">
        <h1 className="font-mono text-lg font-bold tracking-tight">sentry-lite</h1>
        <p className="text-sm text-muted-foreground">
          Enter the gateway token (
          <span className="font-mono">ADMIN_TOKEN</span>) to open the triage UI.
          Session expires after 1 hour of inactivity.
        </p>
      </div>
      <form onSubmit={onSubmit} className="flex flex-col gap-4">
        <div className="space-y-2">
          <Label htmlFor="token">Gateway token</Label>
          <Input
            id="token"
            type="password"
            autoComplete="current-password"
            value={token}
            onChange={(e) => setToken(e.target.value)}
            placeholder="ADMIN_TOKEN"
            autoFocus
          />
        </div>
        {error ? (
          <p className="text-sm text-destructive" role="alert">
            {error}
          </p>
        ) : null}
        <Button type="submit" disabled={pending}>
          {pending ? 'Checking…' : 'Sign in'}
        </Button>
      </form>
    </div>
  )
}
