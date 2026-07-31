"use client"

import { useState, useTransition } from "react"
import * as Sentry from "@sentry/nextjs"
import {
  BugIcon,
  ServerCrashIcon,
  CircleAlertIcon,
  MessageSquareIcon,
  FingerprintIcon,
  UserIcon,
  ActivityIcon,
  PackageIcon,
  TimerIcon,
  BellIcon,
} from "lucide-react"

import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import {
  captureDemoMessage,
  captureFingerprintedPair,
  captureReleaseEvents,
  captureServerException,
  captureWithContext,
  cronCheckIn,
  ensureDemoCron,
  seedDemoAlerts,
  triggerServerError,
} from "@/app/actions"

function TriageHint() {
  return (
    <Alert>
      <CircleAlertIcon />
      <AlertTitle>Where to look</AlertTitle>
      <AlertDescription>
        Triage UI:{" "}
        <a
          href="http://localhost:5173"
          className="underline underline-offset-3 hover:text-foreground"
        >
          http://localhost:5173
        </a>{" "}
        (or{" "}
        <a
          href="http://localhost:8080"
          className="underline underline-offset-3 hover:text-foreground"
        >
          :8080
        </a>
        ). Project 3 — Issues, Performance, Releases, Crons, Alerts.
      </AlertDescription>
    </Alert>
  )
}

function StatusAlert({ status }: { status: string | null }) {
  if (!status) return null
  return (
    <Alert>
      <CircleAlertIcon />
      <AlertTitle>Result</AlertTitle>
      <AlertDescription className="whitespace-pre-wrap font-mono text-xs">
        {status}
      </AlertDescription>
    </Alert>
  )
}

export function FeaturePlayground() {
  const [pending, startTransition] = useTransition()
  const [status, setStatus] = useState<string | null>(null)

  function run(label: string, fn: () => Promise<unknown> | unknown) {
    setStatus(null)
    startTransition(async () => {
      try {
        const result = await fn()
        setStatus(
          typeof result === "string"
            ? result
            : `${label}: ${JSON.stringify(result, null, 2)}`
        )
      } catch (e) {
        setStatus(
          `${label} failed: ${e instanceof Error ? e.message : String(e)}`
        )
      }
    })
  }

  async function fetchMock(
    path: string,
    init?: RequestInit
  ): Promise<string> {
    const res = await fetch(path, init)
    const text = await res.text()
    let body: unknown = text
    try {
      body = JSON.parse(text)
    } catch {
      /* keep text */
    }
    return `${init?.method ?? "GET"} ${path} → ${res.status}\n${JSON.stringify(body, null, 2)}`
  }

  function onClientThrow() {
    setStatus(null)
    throw new Error("sample client throw (sentry-lite)")
  }

  function onClientCapture() {
    try {
      throw new Error("sample client captureException (sentry-lite)")
    } catch (e) {
      Sentry.captureException(e, {
        tags: { service: "sample", surface: "client" },
      })
      setStatus("Client exception sent. Check Issues for project 3.")
    }
  }

  return (
    <div className="mx-auto flex w-full max-w-5xl flex-col gap-6 p-6 pb-16">
      <header className="flex flex-col gap-2">
        <h1 className="font-mono text-2xl font-bold tracking-tight">
          sentry-lite · Next.js playground
        </h1>
        <p className="text-sm text-muted-foreground">
          Exercise errors, context, mock APIs / performance, releases, crons,
          and alerts via{" "}
          <code className="text-xs">@sentry/nextjs</code> against local
          sentry-lite (project 3).
        </p>
        <TriageHint />
        <StatusAlert status={status} />
      </header>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <BugIcon className="size-4" />
            Errors
          </CardTitle>
          <CardDescription>
            Client and server throws / captureException → Issues.
          </CardDescription>
        </CardHeader>
        <CardFooter className="flex flex-wrap gap-2">
          <Button type="button" variant="destructive" onClick={onClientThrow}>
            <BugIcon data-icon="inline-start" />
            Client throw
          </Button>
          <Button type="button" variant="outline" onClick={onClientCapture}>
            <BugIcon data-icon="inline-start" />
            Client capture
          </Button>
          <Button
            type="button"
            variant="destructive"
            disabled={pending}
            onClick={() =>
              run("Server throw", async () => {
                try {
                  await triggerServerError()
                } catch {
                  return "Server action threw. Check Issues for project 3."
                }
              })
            }
          >
            <ServerCrashIcon data-icon="inline-start" />
            Server throw
          </Button>
          <Button
            type="button"
            variant="outline"
            disabled={pending}
            onClick={() =>
              run("Server capture", async () => {
                const result = await captureServerException()
                return result?.ok
                  ? "Server exception captured. Check Issues."
                  : result
              })
            }
          >
            <ServerCrashIcon data-icon="inline-start" />
            Server capture
          </Button>
        </CardFooter>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <UserIcon className="size-4" />
            Context &amp; grouping
          </CardTitle>
          <CardDescription>
            Messages, breadcrumbs, user, tags, and shared fingerprints.
          </CardDescription>
        </CardHeader>
        <CardFooter className="flex flex-wrap gap-2">
          <Button
            type="button"
            variant="outline"
            disabled={pending}
            onClick={() =>
              run("captureMessage", async () => {
                const r = await captureDemoMessage()
                return r?.ok
                  ? "Message sent. Check Issues (message-only)."
                  : r
              })
            }
          >
            <MessageSquareIcon data-icon="inline-start" />
            Capture message
          </Button>
          <Button
            type="button"
            variant="outline"
            disabled={pending}
            onClick={() =>
              run("Context", async () => {
                const r = await captureWithContext()
                return r?.ok
                  ? "Exception with user + breadcrumbs sent."
                  : r
              })
            }
          >
            <UserIcon data-icon="inline-start" />
            Capture with context
          </Button>
          <Button
            type="button"
            variant="outline"
            disabled={pending}
            onClick={() =>
              run("Fingerprint", async () => {
                const r = await captureFingerprintedPair()
                return r?.ok
                  ? "Two messages, one fingerprint (demo-group). Check Issues."
                  : r
              })
            }
          >
            <FingerprintIcon data-icon="inline-start" />
            Fingerprint pair
          </Button>
        </CardFooter>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <ActivityIcon className="size-4" />
            Performance / mock APIs
          </CardTitle>
          <CardDescription>
            Instrumented Route Handlers with nested spans. Check Performance
            and Traces.
          </CardDescription>
        </CardHeader>
        <CardContent className="text-xs text-muted-foreground">
          Routes:{" "}
          <code>/api/mock/users</code>, <code>/users/[id]</code>,{" "}
          <code>/checkout</code>, <code>/slow</code>, <code>/boom</code>
        </CardContent>
        <CardFooter className="flex flex-wrap gap-2">
          <Button
            type="button"
            variant="outline"
            disabled={pending}
            onClick={() =>
              run("List users", () => fetchMock("/api/mock/users"))
            }
          >
            GET users
          </Button>
          <Button
            type="button"
            variant="outline"
            disabled={pending}
            onClick={() =>
              run("Get user", () => fetchMock("/api/mock/users/1"))
            }
          >
            GET user/1
          </Button>
          <Button
            type="button"
            variant="outline"
            disabled={pending}
            onClick={() =>
              run("Missing user", () => fetchMock("/api/mock/users/999"))
            }
          >
            GET user/999 (404)
          </Button>
          <Button
            type="button"
            variant="outline"
            disabled={pending}
            onClick={() =>
              run("Checkout", () =>
                fetchMock("/api/mock/checkout", {
                  method: "POST",
                  headers: { "Content-Type": "application/json" },
                  body: JSON.stringify({ userId: "1", amount: 42 }),
                })
              )
            }
          >
            POST checkout
          </Button>
          <Button
            type="button"
            variant="outline"
            disabled={pending}
            onClick={() =>
              run("Checkout flow", async () => {
                const users = await fetchMock("/api/mock/users")
                const checkout = await fetchMock("/api/mock/checkout", {
                  method: "POST",
                  headers: { "Content-Type": "application/json" },
                  body: JSON.stringify({ userId: "1", amount: 99 }),
                })
                return `${users}\n\n---\n\n${checkout}`
              })
            }
          >
            Full checkout flow
          </Button>
          <Button
            type="button"
            variant="outline"
            disabled={pending}
            onClick={() => run("Slow", () => fetchMock("/api/mock/slow"))}
          >
            GET slow (1.5s)
          </Button>
          <Button
            type="button"
            variant="destructive"
            disabled={pending}
            onClick={() =>
              run("Boom", async () => {
                try {
                  return await fetchMock("/api/mock/boom")
                } catch (e) {
                  return `boom fetch error: ${e instanceof Error ? e.message : String(e)}`
                }
              })
            }
          >
            GET boom
          </Button>
        </CardFooter>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <PackageIcon className="size-4" />
            Releases
          </CardTitle>
          <CardDescription>
            Sends exceptions tagged{" "}
            <code className="text-xs">sample@0.1.0</code> then{" "}
            <code className="text-xs">sample@0.2.0</code> (auto-upserted).
          </CardDescription>
        </CardHeader>
        <CardFooter className="flex flex-wrap gap-2">
          <Button
            type="button"
            variant="outline"
            disabled={pending}
            onClick={() =>
              run("Releases", async () => {
                const r = await captureReleaseEvents()
                return r?.ok
                  ? `Sent events for ${r.releases.join(" and ")}. Check Releases.`
                  : r
              })
            }
          >
            <PackageIcon data-icon="inline-start" />
            Send release events
          </Button>
        </CardFooter>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <TimerIcon className="size-4" />
            Crons
          </CardTitle>
          <CardDescription>
            Creates/reuses monitor{" "}
            <code className="text-xs">sample-heartbeat</code> on project 3, or
            uses <code className="text-xs">CRON_CHECKIN_TOKEN</code>.
          </CardDescription>
        </CardHeader>
        <CardFooter className="flex flex-wrap gap-2">
          <Button
            type="button"
            variant="outline"
            disabled={pending}
            onClick={() =>
              run("Ensure cron", async () => {
                const r = await ensureDemoCron()
                return r
              })
            }
          >
            Ensure monitor
          </Button>
          <Button
            type="button"
            variant="outline"
            disabled={pending}
            onClick={() =>
              run("Check-in OK", async () => {
                const r = await cronCheckIn("ok")
                return r
              })
            }
          >
            Check-in OK
          </Button>
          <Button
            type="button"
            variant="destructive"
            disabled={pending}
            onClick={() =>
              run("Check-in error", async () => {
                const r = await cronCheckIn("error")
                return r
              })
            }
          >
            Check-in error
          </Button>
        </CardFooter>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <BellIcon className="size-4" />
            Alerts
          </CardTitle>
          <CardDescription>
            Seeds webhook rules for <code className="text-xs">new_issue</code>{" "}
            and <code className="text-xs">error_volume</code>. Delivery needs a
            real Slack/email/Telegram target — see README.
          </CardDescription>
        </CardHeader>
        <CardFooter className="flex flex-wrap gap-2">
          <Button
            type="button"
            variant="outline"
            disabled={pending}
            onClick={() =>
              run("Seed alerts", async () => {
                const r = await seedDemoAlerts()
                return r?.ok
                  ? `Created ${r.created.length} alert rules. Check Alerts.`
                  : r
              })
            }
          >
            <BellIcon data-icon="inline-start" />
            Create demo alert rules
          </Button>
        </CardFooter>
      </Card>
    </div>
  )
}
