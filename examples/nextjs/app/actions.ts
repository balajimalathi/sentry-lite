"use server"

import * as Sentry from "@sentry/nextjs"

const PROJECT_ID = 3
const DEMO_CRON_SLUG = "sample-heartbeat"

function sentryLiteUrl() {
  return (process.env.SENTRY_LITE_URL ?? "http://localhost:8080").replace(
    /\/$/,
    ""
  )
}

type CronMonitor = {
  id: number
  project_id: number
  slug: string
  name: string
  token: string
  schedule_sec: number
  grace_sec: number
}

async function resolveCronToken(): Promise<{
  token: string
  source: "env" | "existing" | "created"
  monitor?: CronMonitor
}> {
  const base = sentryLiteUrl()
  const envToken = process.env.CRON_CHECKIN_TOKEN?.trim()
  if (envToken) {
    return { token: envToken, source: "env" }
  }

  const listRes = await fetch(
    `${base}/api/internal/crons?project_id=${PROJECT_ID}`,
    { cache: "no-store" }
  )
  if (!listRes.ok) {
    throw new Error(
      `list crons failed: ${listRes.status} ${await listRes.text()}`
    )
  }
  const monitors = (await listRes.json()) as CronMonitor[]
  const existing = monitors.find((m) => m.slug === DEMO_CRON_SLUG)
  if (existing?.token) {
    return { token: existing.token, source: "existing", monitor: existing }
  }

  const createRes = await fetch(`${base}/api/internal/crons`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      project_id: PROJECT_ID,
      name: "Sample Heartbeat",
      slug: DEMO_CRON_SLUG,
      schedule_sec: 60,
      grace_sec: 30,
      environment: "development",
    }),
  })
  if (!createRes.ok) {
    throw new Error(
      `create cron failed: ${createRes.status} ${await createRes.text()}`
    )
  }
  const created = (await createRes.json()) as CronMonitor
  return { token: created.token, source: "created", monitor: created }
}

function captureWithRelease(release: string, error: Error) {
  Sentry.withScope((scope) => {
    scope.setTag("surface", "release-demo")
    scope.addEventProcessor((event) => {
      event.release = release
      return event
    })
    Sentry.captureException(error)
  })
}

export async function triggerServerError() {
  return Sentry.withServerActionInstrumentation(
    "triggerServerError",
    { recordResponse: true },
    async () => {
      throw new Error("sample server action failure (sentry-lite)")
    }
  )
}

export async function captureServerException() {
  return Sentry.withServerActionInstrumentation(
    "captureServerException",
    { recordResponse: true },
    async () => {
      try {
        throw new Error("sample captured server exception (sentry-lite)")
      } catch (e) {
        Sentry.captureException(e, {
          tags: { service: "sample", surface: "server-action" },
        })
        await Sentry.flush(2000)
        return { ok: true as const }
      }
    }
  )
}

export async function captureDemoMessage() {
  return Sentry.withServerActionInstrumentation(
    "captureDemoMessage",
    { recordResponse: true },
    async () => {
      Sentry.captureMessage("sample captureMessage (sentry-lite)", {
        level: "info",
        tags: { service: "sample", surface: "server-action" },
      })
      await Sentry.flush(2000)
      return { ok: true as const }
    }
  )
}

export async function captureWithContext() {
  return Sentry.withServerActionInstrumentation(
    "captureWithContext",
    { recordResponse: true },
    async () => {
      Sentry.addBreadcrumb({
        category: "auth",
        message: "user opened demo context panel",
        level: "info",
      })
      Sentry.addBreadcrumb({
        category: "ui.click",
        message: "clicked Capture with context",
        level: "info",
      })
      Sentry.setUser({
        id: "demo-user-1",
        email: "demo@example.com",
        username: "demo",
      })

      try {
        throw new Error("sample exception with user + breadcrumbs (sentry-lite)")
      } catch (e) {
        Sentry.captureException(e, {
          tags: {
            service: "sample",
            surface: "server-action",
            feature: "context",
          },
        })
        await Sentry.flush(2000)
        return { ok: true as const }
      }
    }
  )
}

export async function captureFingerprintedPair() {
  return Sentry.withServerActionInstrumentation(
    "captureFingerprintedPair",
    { recordResponse: true },
    async () => {
      const fingerprint = ["demo-group"]
      Sentry.captureMessage("fingerprint demo message A (sentry-lite)", {
        fingerprint,
        tags: { service: "sample", surface: "fingerprint" },
      })
      Sentry.captureMessage("fingerprint demo message B (sentry-lite)", {
        fingerprint,
        tags: { service: "sample", surface: "fingerprint" },
      })
      await Sentry.flush(2000)
      return { ok: true as const, fingerprint }
    }
  )
}

export async function captureReleaseEvents() {
  return Sentry.withServerActionInstrumentation(
    "captureReleaseEvents",
    { recordResponse: true },
    async () => {
      captureWithRelease(
        "sample@0.1.0",
        new Error("sample release regression candidate @0.1.0 (sentry-lite)")
      )
      captureWithRelease(
        "sample@0.2.0",
        new Error("sample release regression candidate @0.2.0 (sentry-lite)")
      )
      await Sentry.flush(2000)
      return {
        ok: true as const,
        releases: ["sample@0.1.0", "sample@0.2.0"] as const,
      }
    }
  )
}

export async function ensureDemoCron() {
  return Sentry.withServerActionInstrumentation(
    "ensureDemoCron",
    { recordResponse: true },
    async () => {
      const base = sentryLiteUrl()
      const resolved = await resolveCronToken()
      return {
        ok: true as const,
        token: resolved.token,
        source: resolved.source,
        checkInUrl: `${base}/api/cron/check-in/${resolved.token}`,
        monitor: resolved.monitor,
      }
    }
  )
}

export async function cronCheckIn(status: "ok" | "error" = "ok") {
  return Sentry.withServerActionInstrumentation(
    "cronCheckIn",
    { recordResponse: true },
    async () => {
      const base = sentryLiteUrl()
      const resolved = await resolveCronToken()
      const duration_ms = 12 + Math.random() * 40
      const res = await fetch(`${base}/api/cron/check-in/${resolved.token}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ status, duration_ms }),
      })
      if (!res.ok) {
        throw new Error(`check-in failed: ${res.status} ${await res.text()}`)
      }
      const monitor = await res.json()
      return {
        ok: true as const,
        status,
        duration_ms,
        token: resolved.token,
        monitor,
      }
    }
  )
}

export async function seedDemoAlerts() {
  return Sentry.withServerActionInstrumentation(
    "seedDemoAlerts",
    { recordResponse: true },
    async () => {
      const base = sentryLiteUrl()
      const rules = [
        {
          project_id: PROJECT_ID,
          name: "Sample new issue",
          trigger: "new_issue",
          channel: "webhook",
          target: "https://example.com/hooks/sentry-lite-new-issue",
          threshold: 0,
          window_sec: 300,
        },
        {
          project_id: PROJECT_ID,
          name: "Sample error volume",
          trigger: "error_volume",
          channel: "webhook",
          target: "https://example.com/hooks/sentry-lite-error-volume",
          threshold: 3,
          window_sec: 300,
        },
      ] as const

      const created: unknown[] = []
      for (const rule of rules) {
        const res = await fetch(`${base}/api/internal/alerts`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(rule),
        })
        if (!res.ok) {
          const text = await res.text()
          throw new Error(
            `create alert "${rule.name}" failed: ${res.status} ${text}`
          )
        }
        created.push(await res.json())
      }

      return { ok: true as const, created }
    }
  )
}
